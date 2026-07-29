package ui

import "fyne.io/fyne/v2"

const playerTopGap = float32(12)

// playerTopLayout places title | controls | volume on one row.
// Controls stay horizontally centered; title fills all space up to them.
type playerTopLayout struct{}

var _ fyne.Layout = (*playerTopLayout)(nil)

func (l *playerTopLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 3 {
		return
	}
	title, controls, volume := objects[0], objects[1], objects[2]

	cMin := controls.MinSize()
	vMin := volume.MinSize()

	cx := (size.Width - cMin.Width) / 2
	cy := (size.Height - cMin.Height) / 2
	controls.Resize(cMin)
	controls.Move(fyne.NewPos(cx, cy))

	vw := vMin.Width
	if vw > size.Width {
		vw = size.Width
	}
	volume.Resize(fyne.NewSize(vw, size.Height))
	volume.Move(fyne.NewPos(size.Width-vw, 0))

	titleW := cx - playerTopGap
	if titleW < 40 {
		titleW = 40
	}
	// Don't run under the volume if the window is very narrow.
	maxTitle := size.Width - vw - playerTopGap
	if titleW > maxTitle {
		titleW = maxTitle
	}
	if titleW < 0 {
		titleW = 0
	}
	title.Resize(fyne.NewSize(titleW, size.Height))
	title.Move(fyne.NewPos(0, 0))
}

func (l *playerTopLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) != 3 {
		return fyne.NewSize(0, 0)
	}
	t := objects[0].MinSize()
	c := objects[1].MinSize()
	v := objects[2].MinSize()
	h := c.Height
	if t.Height > h {
		h = t.Height
	}
	if v.Height > h {
		h = v.Height
	}
	return fyne.NewSize(t.Width+c.Width+v.Width+playerTopGap*2, h)
}
