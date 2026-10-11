package instance_test

import (
	"net"
	"testing"
)

// deadSocket leaves a unix socket file at path that nobody accepts on.
func deadSocket(t *testing.T, path string) string {
	t.Helper()
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ul := l.(*net.UnixListener)
	ul.SetUnlinkOnClose(false)
	_ = ul.Close()
	return path
}
