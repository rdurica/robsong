package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestChooseLaunch(t *testing.T) {
	cases := []struct {
		name    string
		hadArgs bool
		n       int
		want    launchMode
	}{
		{"no args", false, 0, launchFull},
		{"no args ignores count", false, 3, launchFull},
		{"args none supported", true, 0, launchNoTracks},
		{"one track", true, 1, launchMini},
		{"two tracks", true, 2, launchFull},
		{"many tracks", true, 10, launchFull},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := chooseLaunch(c.hadArgs, c.n); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func touch(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not really audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func trackPaths(t *testing.T, args []string) []string {
	t.Helper()
	got := []string{}
	for _, tr := range collectTracks(args) {
		got = append(got, tr.Path)
	}
	return got
}

func TestCollectTracksSingleFile(t *testing.T) {
	d := t.TempDir()
	a := touch(t, filepath.Join(d, "a.mp3"))
	got := trackPaths(t, []string{a})
	if !reflect.DeepEqual(got, []string{a}) {
		t.Fatalf("got %v", got)
	}
}

func TestCollectTracksFolderWithThreeFiles(t *testing.T) {
	d := t.TempDir()
	touch(t, filepath.Join(d, "1.mp3"))
	touch(t, filepath.Join(d, "2.flac"))
	touch(t, filepath.Join(d, "3.wav"))
	touch(t, filepath.Join(d, "readme.txt"))
	if got := trackPaths(t, []string{d}); len(got) != 3 {
		t.Fatalf("want 3 tracks, got %v", got)
	}
}

func TestCollectTracksFolderWithOneFile(t *testing.T) {
	d := t.TempDir()
	touch(t, filepath.Join(d, "only.ogg"))
	touch(t, filepath.Join(d, "cover.jpg"))
	if got := trackPaths(t, []string{d}); len(got) != 1 {
		t.Fatalf("want 1 track, got %v", got)
	}
}

func TestCollectTracksDropsUnsupported(t *testing.T) {
	d := t.TempDir()
	a := touch(t, filepath.Join(d, "a.mp3"))
	n := touch(t, filepath.Join(d, "notes.txt"))
	got := trackPaths(t, []string{a, n})
	if !reflect.DeepEqual(got, []string{a}) {
		t.Fatalf("got %v", got)
	}
}

func TestCollectTracksSkipsMissingKeepsOthers(t *testing.T) {
	d := t.TempDir()
	a := touch(t, filepath.Join(d, "a.mp3"))
	b := touch(t, filepath.Join(d, "b.flac"))
	got := trackPaths(t, []string{a, filepath.Join(d, "missing.mp3"), b})
	if !reflect.DeepEqual(got, []string{a, b}) {
		t.Fatalf("got %v", got)
	}
}

func TestCollectTracksPreservesArgOrder(t *testing.T) {
	d := t.TempDir()
	a := touch(t, filepath.Join(d, "a.mp3"))
	b := touch(t, filepath.Join(d, "b.mp3"))
	c := touch(t, filepath.Join(d, "c.mp3"))
	got := trackPaths(t, []string{c, a, b})
	if !reflect.DeepEqual(got, []string{c, a, b}) {
		t.Fatalf("got %v", got)
	}
}

func TestCollectTracksDeduplicates(t *testing.T) {
	d := t.TempDir()
	a := touch(t, filepath.Join(d, "a.mp3"))
	got := trackPaths(t, []string{a, a, d})
	if !reflect.DeepEqual(got, []string{a}) {
		t.Fatalf("got %v", got)
	}
}

func TestCollectTracksReturnsAbsolutePaths(t *testing.T) {
	d := t.TempDir()
	touch(t, filepath.Join(d, "a.mp3"))
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	got := trackPaths(t, []string{"a.mp3"})
	if len(got) != 1 || !filepath.IsAbs(got[0]) {
		t.Fatalf("want one absolute path, got %v", got)
	}
}
