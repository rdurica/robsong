package instance_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rdurica/robsong/internal/instance"
)

func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "rs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

func useRuntimeDir(t *testing.T) string {
	t.Helper()
	d := shortDir(t)
	t.Setenv("XDG_RUNTIME_DIR", d)
	return d
}

func TestSocketPathUsesRuntimeDir(t *testing.T) {
	d := useRuntimeDir(t)
	got, err := instance.SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(d, "robsong.sock"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSocketPathFallsBackToXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	cfg := shortDir(t)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	got, err := instance.SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cfg, "robsong", "robsong.sock"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSocketPathFallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	home := shortDir(t)
	t.Setenv("HOME", home)
	got, err := instance.SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", "robsong", "robsong.sock"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSendWithoutListenerReturnsFalse(t *testing.T) {
	useRuntimeDir(t)
	ok, err := instance.Send([]string{"/a.mp3"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected false with no listener")
	}
}

func listen(t *testing.T) (chan []string, func()) {
	t.Helper()
	ch := make(chan []string, 4)
	c, err := instance.Listen(func(p []string) { ch <- p })
	if err != nil {
		t.Fatal(err)
	}
	var closed bool
	closeFn := func() {
		if !closed {
			closed = true
			_ = c.Close()
		}
	}
	t.Cleanup(closeFn)
	return ch, closeFn
}

func recv(t *testing.T, ch chan []string) []string {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for onOpen")
		return nil
	}
}

func TestListenSendDeliversPathsInOrder(t *testing.T) {
	useRuntimeDir(t)
	ch, _ := listen(t)
	want := []string{"/music/b.mp3", "/music/a.flac", "/music/c.wav"}
	ok, err := instance.Send(want)
	if err != nil || !ok {
		t.Fatalf("Send = %v, %v", ok, err)
	}
	if got := recv(t, ch); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestEmptySliceDeliveredAsNonNilEmpty(t *testing.T) {
	useRuntimeDir(t)
	ch, _ := listen(t)
	ok, err := instance.Send([]string{})
	if err != nil || !ok {
		t.Fatalf("Send = %v, %v", ok, err)
	}
	got := recv(t, ch)
	if got == nil || len(got) != 0 {
		t.Fatalf("want non-nil empty slice, got %#v", got)
	}
}

func TestNilSliceDeliveredAsNonNilEmpty(t *testing.T) {
	useRuntimeDir(t)
	ch, _ := listen(t)
	ok, err := instance.Send(nil)
	if err != nil || !ok {
		t.Fatalf("Send = %v, %v", ok, err)
	}
	if got := recv(t, ch); got == nil || len(got) != 0 {
		t.Fatalf("want non-nil empty slice, got %#v", got)
	}
}

func TestListenTakesOverStalePlainFile(t *testing.T) {
	d := useRuntimeDir(t)
	if err := os.WriteFile(filepath.Join(d, "robsong.sock"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	ch, _ := listen(t)
	ok, err := instance.Send([]string{"/x.mp3"})
	if err != nil || !ok {
		t.Fatalf("Send = %v, %v", ok, err)
	}
	if got := recv(t, ch); !reflect.DeepEqual(got, []string{"/x.mp3"}) {
		t.Fatalf("got %v", got)
	}
}

func TestListenTakesOverDeadSocket(t *testing.T) {
	useRuntimeDir(t)
	c, err := instance.Listen(func([]string) {})
	if err != nil {
		t.Fatal(err)
	}
	path, _ := instance.SocketPath()
	// Simulate a crash: keep the socket file after the listener is gone.
	_ = c.Close()
	// Recreate a dead socket file by listening natively and closing without unlink.
	dead := deadSocket(t, path)
	if _, err := os.Stat(dead); err != nil {
		t.Fatal(err)
	}
	ch, _ := listen(t)
	ok, err := instance.Send([]string{"/y.mp3"})
	if err != nil || !ok {
		t.Fatalf("Send = %v, %v", ok, err)
	}
	if got := recv(t, ch); !reflect.DeepEqual(got, []string{"/y.mp3"}) {
		t.Fatalf("got %v", got)
	}
}

func TestCloseRemovesSocketFile(t *testing.T) {
	useRuntimeDir(t)
	c, err := instance.Listen(func([]string) {})
	if err != nil {
		t.Fatal(err)
	}
	path, _ := instance.SocketPath()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("socket should exist while listening: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("socket file should be removed, stat err = %v", err)
	}
}

func TestSendAfterCloseReturnsFalse(t *testing.T) {
	useRuntimeDir(t)
	_, closeFn := listen(t)
	closeFn()
	ok, err := instance.Send([]string{"/a.mp3"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected false after Close")
	}
}
