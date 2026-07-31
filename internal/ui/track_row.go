package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/model"
)

// trackRow is a dense list row with double-click play and drag-to-reorder.
type trackRow struct {
	widget.BaseWidget
	app       *App
	index     int
	line      *widget.Label
	duration  *widget.Label
	dragAccum float32
}

func newTrackRow(a *App) *trackRow {
	r := &trackRow{
		app:      a,
		line:     widget.NewLabel("title"),
		duration: widget.NewLabel("0:00"),
	}
	r.line.Truncation = fyne.TextTruncateEllipsis
	r.duration.Importance = widget.LowImportance
	r.ExtendBaseWidget(r)
	return r
}

func (r *trackRow) Update(index int, t model.Track, playing bool) {
	r.index = index
	artist := t.DisplayArtist()
	title := t.DisplayTitle()
	if artist != "" {
		r.line.SetText(title + "  —  " + artist)
	} else {
		r.line.SetText(title)
	}
	r.line.TextStyle = fyne.TextStyle{Bold: playing}
	r.line.Refresh()
	if t.DurationMs > 0 {
		r.duration.SetText(formatMs(t.DurationMs))
	} else {
		r.duration.SetText("")
	}
}

func (r *trackRow) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewBorder(nil, nil, nil, r.duration, r.line)
	// Spacers vertically center the labels within the list row height.
	return widget.NewSimpleRenderer(container.NewVBox(
		layout.NewSpacer(), content, layout.NewSpacer(),
	))
}

func (r *trackRow) MinSize() fyne.Size {
	return fyne.NewSize(120, 28)
}

func (r *trackRow) Tapped(*fyne.PointEvent) {
	r.app.selectedTrack = r.index
	r.app.trackList.Select(r.index)
}

func (r *trackRow) DoubleTapped(*fyne.PointEvent) {
	r.app.playFrom(r.index)
}

func (r *trackRow) TappedSecondary(e *fyne.PointEvent) {
	r.app.selectedTrack = r.index
	r.app.trackList.Select(r.index)
	if r.index < 0 || r.index >= len(r.app.tracks) {
		return
	}
	r.app.showTrackContextMenu(r.index, r.app.tracks[r.index], e.AbsolutePosition)
}

func (r *trackRow) Dragged(e *fyne.DragEvent) {
	r.dragAccum += e.Dragged.DY
	rowH := r.Size().Height
	if rowH < 1 {
		rowH = 24
	}
	for r.dragAccum >= rowH {
		if r.index+1 >= len(r.app.tracks) {
			r.dragAccum = 0
			break
		}
		from := r.index
		r.app.moveTrack(from, from+1)
		r.index = from + 1
		r.dragAccum -= rowH
	}
	for r.dragAccum <= -rowH {
		if r.index-1 < 0 {
			r.dragAccum = 0
			break
		}
		from := r.index
		r.app.moveTrack(from, from-1)
		r.index = from - 1
		r.dragAccum += rowH
	}
}

func (r *trackRow) DragEnd() {
	r.dragAccum = 0
}

var _ fyne.Tappable = (*trackRow)(nil)
var _ fyne.DoubleTappable = (*trackRow)(nil)
var _ fyne.SecondaryTappable = (*trackRow)(nil)
var _ fyne.Draggable = (*trackRow)(nil)
