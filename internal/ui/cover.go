package ui

import (
	"image"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const coverSize = 70

// coverLoader returns the embedded cover of the track at path. (nil, nil)
// means the track has no cover.
type coverLoader func(path string) (image.Image, error)

// coverArt shows the cover of the current track, or a muted music-note
// placeholder when there is none. Loading happens off the UI goroutine.
type coverArt struct {
	widget.BaseWidget

	load coverLoader

	// rmu serialises refreshes: the test driver runs fyne.Do on the
	// calling goroutine, so refreshes may otherwise overlap.
	rmu sync.Mutex
	mu  sync.Mutex
	gen uint64
	img image.Image
}

func newCoverArt(load coverLoader) *coverArt {
	c := &coverArt{load: load}
	c.ExtendBaseWidget(c)
	return c
}

// SetTrack starts loading the cover for path; "" means nothing is playing.
func (c *coverArt) SetTrack(path string) {
	c.mu.Lock()
	c.gen++
	gen := c.gen
	c.img = nil
	c.mu.Unlock()
	fyne.Do(c.Refresh)

	if path == "" {
		return
	}
	go func() {
		img, err := c.load(path)
		if err != nil {
			img = nil
		}
		fyne.Do(func() {
			c.mu.Lock()
			if c.gen != gen {
				c.mu.Unlock()
				return
			}
			c.img = img
			c.mu.Unlock()
			c.Refresh() // already on the UI goroutine
		})
	}()
}

// ShowsPlaceholder reports whether the placeholder icon is displayed.
func (c *coverArt) ShowsPlaceholder() bool {
	return c.Image() == nil
}

// Image returns the displayed cover, nil when the placeholder is shown.
func (c *coverArt) Image() image.Image {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.img
}

// MinSize is constant, so it never needs the renderer.
func (c *coverArt) MinSize() fyne.Size { return fyne.NewSize(coverSize, coverSize) }

// Refresh is serialised, see rmu.
func (c *coverArt) Refresh() {
	c.rmu.Lock()
	defer c.rmu.Unlock()
	c.BaseWidget.Refresh()
}

func (c *coverArt) CreateRenderer() fyne.WidgetRenderer {
	r := &coverRenderer{
		cover: c,
		art:   canvas.NewImageFromImage(nil),
		note:  canvas.NewImageFromResource(theme.NewDisabledResource(theme.MediaMusicIcon())),
	}
	r.art.FillMode = canvas.ImageFillContain
	r.note.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

type coverRenderer struct {
	cover *coverArt
	art   *canvas.Image
	note  *canvas.Image
}

// Layout keeps the cover square and centred, even when the player bar
// gives the widget more height than width (or vice versa).
func (r *coverRenderer) Layout(s fyne.Size) {
	side := fyne.Min(s.Width, s.Height)
	x, y := (s.Width-side)/2, (s.Height-side)/2
	r.art.Resize(fyne.NewSquareSize(side))
	r.art.Move(fyne.NewPos(x, y))
	pad := side * 0.2
	r.note.Resize(fyne.NewSquareSize(side - 2*pad))
	r.note.Move(fyne.NewPos(x+pad, y+pad))
}

func (r *coverRenderer) MinSize() fyne.Size { return fyne.NewSize(coverSize, coverSize) }

func (r *coverRenderer) Refresh() {
	img := r.cover.Image()
	if img == nil {
		r.art.Image = nil
		r.art.Hide()
		r.note.Show()
	} else {
		r.art.Image = img
		r.note.Hide()
		r.art.Show()
	}
	r.art.Refresh()
	r.note.Refresh()
}

func (r *coverRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.note, r.art}
}

func (r *coverRenderer) Destroy() {}
