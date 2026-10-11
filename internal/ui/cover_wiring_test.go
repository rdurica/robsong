package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/rdurica/robsong/internal/model"
	"github.com/rdurica/robsong/internal/queue"
)

func TestUpdateNowPlayingDrivesCover(t *testing.T) {
	f := newFakeCovers()
	imgA := solidImg(color.RGBA{R: 255, A: 255})
	imgB := solidImg(color.RGBA{B: 255, A: 255})
	f.results["/a.mp3"] = imgA
	f.results["/b.mp3"] = imgB
	test.NewTempApp(t)
	a := &App{queue: queue.New(), nowTitle: NewMarqueeTitle(""), cover: newCoverArt(f.load)}

	a.updateNowPlaying()
	if !a.cover.ShowsPlaceholder() || f.callCount() != 0 {
		t.Fatalf("empty queue: placeholder=%v loads=%d", a.cover.ShowsPlaceholder(), f.callCount())
	}

	a.queue.Set([]model.Track{{Path: "/a.mp3", Title: "A"}, {Path: "/b.mp3", Title: "B"}})
	a.updateNowPlaying()
	eventually(t, "cover of first track", func() bool { return a.cover.Image() == imgA })
	f.mu.Lock()
	first := append([]string(nil), f.calls...)
	f.mu.Unlock()
	if len(first) != 1 || first[0] != "/a.mp3" {
		t.Fatalf("loader calls = %v, want [/a.mp3]", first)
	}

	if _, ok := a.queue.Next(); !ok {
		t.Fatal("queue.Next failed")
	}
	a.updateNowPlaying()
	eventually(t, "cover of next track", func() bool { return a.cover.Image() == imgB })
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 2 || f.calls[1] != "/b.mp3" {
		t.Fatalf("loader calls = %v, want [/a.mp3 /b.mp3]", f.calls)
	}
}

func assertSquareCentered(t *testing.T, state string, c *coverArt, size fyne.Size) {
	t.Helper()
	r := test.TempWidgetRenderer(t, c)
	c.Resize(size)
	r.Layout(size)
	side := size.Width
	if size.Height < side {
		side = size.Height
	}
	visible := 0
	for _, o := range r.Objects() {
		if !o.Visible() {
			continue
		}
		visible++
		s, p := o.Size(), o.Position()
		if s.Width != s.Height {
			t.Errorf("%s %v: object size %v not square", state, size, s)
		}
		if s.Width > side {
			t.Errorf("%s %v: object side %v exceeds widget side %v", state, size, s.Width, side)
		}
		wantX, wantY := (size.Width-s.Width)/2, (size.Height-s.Height)/2
		if p.X != wantX || p.Y != wantY {
			t.Errorf("%s %v: object at %v, want centered (%v,%v)", state, size, p, wantX, wantY)
		}
	}
	if visible == 0 {
		t.Errorf("%s %v: no visible object", state, size)
	}
}

func TestCoverLayoutIsSquareAndCentered(t *testing.T) {
	sizes := []fyne.Size{fyne.NewSize(70, 76), fyne.NewSize(90, 70), fyne.NewSize(70, 70)}
	for _, size := range sizes {
		f := newFakeCovers()
		c := newTestCover(t, f)
		assertSquareCentered(t, "placeholder", c, size)
	}
	for _, size := range sizes {
		f := newFakeCovers()
		img := solidImg(color.RGBA{G: 255, A: 255})
		f.results["/a.mp3"] = img
		c := newTestCover(t, f)
		c.SetTrack("/a.mp3")
		eventually(t, "image shown", func() bool { return c.Image() == img })
		// Wait until the async refresh finished rendering (it holds rmu).
		rr := test.TempWidgetRenderer(t, c).(*coverRenderer)
		eventually(t, "image rendered", func() bool {
			c.rmu.Lock()
			defer c.rmu.Unlock()
			return rr.art.Image == img
		})
		assertSquareCentered(t, "image", c, size)
	}
}
