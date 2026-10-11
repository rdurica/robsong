package ui

import (
	"errors"
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// Seam the implementation must provide (production code, internal/ui):
//
//	type coverLoader func(path string) (image.Image, error)
//	func newCoverArt(load coverLoader) *coverArt     // *coverArt is a fyne.Widget, fixed square MinSize (~70px)
//	func (c *coverArt) SetTrack(path string)         // "" = nothing playing
//	func (c *coverArt) ShowsPlaceholder() bool
//	func (c *coverArt) Image() image.Image           // currently displayed cover, nil when placeholder

type fakeCovers struct {
	mu      sync.Mutex
	calls   []string
	gates   map[string]chan struct{} // path -> release gate (nil = no blocking)
	results map[string]image.Image
	errs    map[string]error
}

func newFakeCovers() *fakeCovers {
	return &fakeCovers{
		gates:   map[string]chan struct{}{},
		results: map[string]image.Image{},
		errs:    map[string]error{},
	}
}

func (f *fakeCovers) load(path string) (image.Image, error) {
	f.mu.Lock()
	f.calls = append(f.calls, path)
	gate := f.gates[path]
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.results[path], f.errs[path]
}

func (f *fakeCovers) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func solidImg(c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", what)
}

func newTestCover(t *testing.T, f *fakeCovers) *coverArt {
	t.Helper()
	test.NewTempApp(t)
	return newCoverArt(f.load)
}

func TestCoverShowsEmbeddedImage(t *testing.T) {
	f := newFakeCovers()
	img := solidImg(color.RGBA{R: 255, A: 255})
	f.results["/a.mp3"] = img
	c := newTestCover(t, f)

	c.SetTrack("/a.mp3")

	eventually(t, "cover image shown", func() bool { return c.Image() == img })
	if c.ShowsPlaceholder() {
		t.Fatal("placeholder shown although cover loaded")
	}
}

func TestCoverPlaceholderWhenNoImageOrError(t *testing.T) {
	f := newFakeCovers()
	img := solidImg(color.RGBA{G: 255, A: 255})
	f.results["/with.mp3"] = img
	f.errs["/err.mp3"] = errors.New("bad cover")
	c := newTestCover(t, f)

	c.SetTrack("/with.mp3")
	eventually(t, "cover shown", func() bool { return c.Image() == img })
	withSize := c.MinSize()

	for _, p := range []string{"/none.mp3", "/err.mp3"} {
		c.SetTrack(p)
		eventually(t, "load finished for "+p, func() bool { return f.callCount() >= 2 })
		eventually(t, "placeholder for "+p, func() bool { return c.ShowsPlaceholder() && c.Image() == nil })
		if got := c.MinSize(); got != withSize {
			t.Fatalf("MinSize for %s = %v, want %v (layout must not jump)", p, got, withSize)
		}
	}
	if withSize.Width <= 0 || withSize.Width != withSize.Height {
		t.Fatalf("cover must be square, got %v", withSize)
	}
}

func TestCoverNothingPlayingIsImmediatePlaceholderWithoutLoading(t *testing.T) {
	f := newFakeCovers()
	img := solidImg(color.RGBA{B: 255, A: 255})
	f.results["/a.mp3"] = img
	c := newTestCover(t, f)

	if !c.ShowsPlaceholder() {
		t.Fatal("fresh cover must show placeholder")
	}
	c.SetTrack("/a.mp3")
	eventually(t, "cover shown", func() bool { return c.Image() == img })
	before := f.callCount()

	c.SetTrack("")

	if !c.ShowsPlaceholder() || c.Image() != nil {
		t.Fatal("stop must show placeholder immediately")
	}
	time.Sleep(30 * time.Millisecond)
	if f.callCount() != before {
		t.Fatal("loader must not be called when nothing plays")
	}
}

func TestCoverNothingPlayingDropsInFlightLoad(t *testing.T) {
	f := newFakeCovers()
	gate := make(chan struct{})
	f.gates["/a.mp3"] = gate
	f.results["/a.mp3"] = solidImg(color.RGBA{R: 1, A: 255})
	c := newTestCover(t, f)

	c.SetTrack("/a.mp3")
	eventually(t, "loader started", func() bool { return f.callCount() == 1 })
	c.SetTrack("")
	close(gate)
	time.Sleep(50 * time.Millisecond)

	if !c.ShowsPlaceholder() {
		t.Fatal("stale result applied after stop")
	}
}

func TestCoverSwitchShowsNewTrackCoverOrPlaceholder(t *testing.T) {
	f := newFakeCovers()
	a := solidImg(color.RGBA{R: 255, A: 255})
	b := solidImg(color.RGBA{B: 255, A: 255})
	f.results["/a.mp3"] = a
	f.results["/b.mp3"] = b
	c := newTestCover(t, f)

	c.SetTrack("/a.mp3")
	eventually(t, "A shown", func() bool { return c.Image() == a })
	c.SetTrack("/b.mp3")
	eventually(t, "B shown", func() bool { return c.Image() == b })
	c.SetTrack("/nocover.mp3")
	eventually(t, "placeholder after switch to coverless", func() bool { return c.ShowsPlaceholder() })
}

func TestCoverStaleLoadDoesNotOverrideNewerTrack(t *testing.T) {
	f := newFakeCovers()
	a := solidImg(color.RGBA{R: 255, A: 255})
	b := solidImg(color.RGBA{B: 255, A: 255})
	gateA := make(chan struct{})
	f.gates["/a.mp3"] = gateA
	f.results["/a.mp3"] = a
	f.results["/b.mp3"] = b
	c := newTestCover(t, f)

	c.SetTrack("/a.mp3")
	eventually(t, "A load started", func() bool { return f.callCount() == 1 })
	c.SetTrack("/b.mp3")
	eventually(t, "B shown", func() bool { return c.Image() == b })

	close(gateA) // A resolves after B
	time.Sleep(50 * time.Millisecond)

	if c.Image() != b {
		t.Fatal("stale cover of A replaced B")
	}
}

func TestCoverStaleLoadDoesNotOverrideNewerPlaceholder(t *testing.T) {
	f := newFakeCovers()
	gateA := make(chan struct{})
	f.gates["/a.mp3"] = gateA
	f.results["/a.mp3"] = solidImg(color.RGBA{R: 255, A: 255})
	c := newTestCover(t, f)

	c.SetTrack("/a.mp3")
	eventually(t, "A load started", func() bool { return f.callCount() == 1 })
	c.SetTrack("/b.mp3") // no cover
	eventually(t, "B loaded", func() bool { return f.callCount() == 2 })
	close(gateA)
	time.Sleep(50 * time.Millisecond)

	if !c.ShowsPlaceholder() {
		t.Fatal("stale cover of A shown for coverless B")
	}
}

func TestCoverSetTrackDoesNotBlockOnLoader(t *testing.T) {
	f := newFakeCovers()
	gate := make(chan struct{})
	defer close(gate)
	f.gates["/slow.mp3"] = gate
	c := newTestCover(t, f)

	done := make(chan struct{})
	go func() {
		c.SetTrack("/slow.mp3")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SetTrack blocked while loader was blocked")
	}
}
