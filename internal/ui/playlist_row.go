package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// playlistRow is a sidebar playlist item with right-click Rename/Delete.
type playlistRow struct {
	widget.BaseWidget
	app   *App
	index int
	label *widget.Label
}

func newPlaylistRow(a *App) *playlistRow {
	r := &playlistRow{
		app:   a,
		label: widget.NewLabel("playlist"),
	}
	r.label.Truncation = fyne.TextTruncateEllipsis
	r.ExtendBaseWidget(r)
	return r
}

func (r *playlistRow) Update(index int, name string) {
	r.index = index
	r.label.SetText(name)
}

func (r *playlistRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewVBox(
		layout.NewSpacer(), r.label, layout.NewSpacer(),
	))
}

func (r *playlistRow) MinSize() fyne.Size {
	return fyne.NewSize(80, 28)
}

func (r *playlistRow) Tapped(*fyne.PointEvent) {
	if r.index >= 0 {
		r.app.playlistList.Select(r.index)
	}
}

func (r *playlistRow) TappedSecondary(e *fyne.PointEvent) {
	if r.index < 0 || r.index >= len(r.app.playlists) {
		return
	}
	r.app.playlistList.Select(r.index)
	r.app.showPlaylistContextMenu(r.index, e.AbsolutePosition)
}
