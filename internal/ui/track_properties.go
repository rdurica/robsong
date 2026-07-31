package ui

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/model"
)

func (a *App) showTrackProperties(t model.Track) {
	sizeText := "Unavailable"
	if info, err := os.Stat(t.Path); err == nil {
		sizeText = formatBytes(info.Size())
	}

	var playlistsText string
	if pls, err := a.store.PlaylistsContaining(t.Path); err == nil {
		var names []string
		for _, p := range pls {
			if p.System {
				continue
			}
			names = append(names, p.Name)
		}
		if len(names) > 0 {
			playlistsText = strings.Join(names, ", ")
		}
	}

	valueLabel := func(text string) *widget.Label {
		l := widget.NewLabel(text)
		// Truncate instead of wrap — wrap inflates Form MinSize and leaves a tall empty dialog.
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	copyable := func(text, copiedMsg string) fyne.CanvasObject {
		icon := newCopyIcon(func() {
			a.win.Clipboard().SetContent(text)
			a.setStatus(copiedMsg)
		})
		return container.NewBorder(nil, nil, nil, icon, valueLabel(text))
	}

	folder := filepath.Dir(t.Path)
	filename := filepath.Base(t.Path)
	items := []*widget.FormItem{
		widget.NewFormItem("Title", valueLabel(t.DisplayTitle())),
		widget.NewFormItem("Artist", valueLabel(t.DisplayArtist())),
		widget.NewFormItem("Duration", valueLabel(formatMs(t.DurationMs))),
		widget.NewFormItem("Folder", copyable(folder, "Folder copied")),
		widget.NewFormItem("Filename", copyable(filename, "Filename copied")),
		widget.NewFormItem("Size", valueLabel(sizeText)),
	}
	if playlistsText != "" {
		items = append(items, widget.NewFormItem("Playlists", valueLabel(playlistsText)))
	}
	form := widget.NewForm(items...)
	d := dialog.NewCustom("", "Close", form, a.win)
	d.Show()
	stripDialogTitle(a.win.Canvas())
	padTopModal(a.win.Canvas())
	// Wider dialog; height clamps to MinSize (no stretch gap under Size).
	d.Resize(fyne.NewSize(680, 1))
}

const copyIconHit = float32(28)

// copyIcon is a compact tappable clipboard icon with a larger hit target.
type copyIcon struct {
	widget.BaseWidget
	img   *canvas.Image
	onTap func()
}

func newCopyIcon(onTap func()) *copyIcon {
	img := canvas.NewImageFromResource(theme.ContentCopyIcon())
	img.SetMinSize(fyne.NewSquareSize(theme.IconInlineSize()))
	img.FillMode = canvas.ImageFillContain
	c := &copyIcon{img: img, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *copyIcon) MinSize() fyne.Size {
	return fyne.NewSquareSize(copyIconHit)
}

func (c *copyIcon) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *copyIcon) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (c *copyIcon) CreateRenderer() fyne.WidgetRenderer {
	return &copyIconRenderer{img: c.img}
}

type copyIconRenderer struct {
	img *canvas.Image
}

func (r *copyIconRenderer) Layout(size fyne.Size) {
	side := theme.IconInlineSize()
	r.img.Resize(fyne.NewSquareSize(side))
	r.img.Move(fyne.NewPos((size.Width-side)/2, (size.Height-side)/2))
}

func (r *copyIconRenderer) MinSize() fyne.Size {
	return fyne.NewSquareSize(copyIconHit)
}

func (r *copyIconRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.img} }
func (r *copyIconRenderer) Refresh()                     { r.img.Refresh() }
func (r *copyIconRenderer) Destroy()                     {}
