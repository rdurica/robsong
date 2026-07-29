package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	ctxMenuPadX   = float32(12)
	ctxMenuPadY   = float32(6)
	ctxMenuRowH   = float32(28)
	ctxMenuRadius = float32(8)
	ctxMenuMinW   = float32(160)
	ctxMenuArrowW = float32(14)
)

type ctxMenuAction struct {
	label    string
	action   func()
	children []ctxMenuAction
}

// showCompactMenu shows a tight right-click menu as one overlay (single outside-click dismisses all).
func showCompactMenu(c fyne.Canvas, pos fyne.Position, actions []ctxMenuAction) {
	if len(actions) == 0 {
		return
	}

	layer := &ctxMenuLayer{c: c}
	layer.ExtendBaseWidget(layer)
	layer.dismiss = func() {
		c.Overlays().Remove(layer)
	}

	mainRows := make([]fyne.CanvasObject, 0, len(actions))
	for _, a := range actions {
		act := a
		hasKids := len(act.children) > 0
		row := newCtxMenuRow(act.label, hasKids, func() {
			if hasKids {
				return
			}
			layer.dismiss()
			if act.action != nil {
				act.action()
			}
		})
		if hasKids {
			kids := act.children
			row.onHover = func() { layer.showSub(row, kids) }
		} else {
			row.onHover = layer.hideSub
		}
		mainRows = append(mainRows, row)
	}
	layer.main = buildCtxPanelFromRows(mainRows)
	layer.sub = buildCtxPanelFromRows(nil) // placeholder; replaced on open
	layer.sub.Hide()

	c.Overlays().Add(layer)
	// Overlay stack may offset to InteractiveArea — store positions relative to layer.
	origin := layer.Position()
	layer.mainAt = fyne.NewPos(pos.X-origin.X, pos.Y-origin.Y)
	layer.Refresh()
}

type ctxMenuLayer struct {
	widget.BaseWidget
	c       fyne.Canvas
	main    fyne.CanvasObject
	sub     fyne.CanvasObject
	mainAt  fyne.Position
	subAt   fyne.Position
	hasSub  bool
	dismiss func()
}

func (l *ctxMenuLayer) showSub(anchor *ctxMenuRow, children []ctxMenuAction) {
	if len(children) == 0 {
		l.hideSub()
		return
	}
	rows := make([]fyne.CanvasObject, 0, len(children))
	for _, a := range children {
		act := a
		rows = append(rows, newCtxMenuRow(act.label, false, func() {
			l.dismiss()
			if act.action != nil {
				act.action()
			}
		}))
	}
	l.sub = buildCtxPanelFromRows(rows)
	abs := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	sz := anchor.Size()
	origin := l.Position()
	l.subAt = fyne.NewPos(abs.X+sz.Width+2-origin.X, abs.Y-ctxMenuPadY-origin.Y)
	l.hasSub = true
	l.Refresh()
}

func (l *ctxMenuLayer) hideSub() {
	if !l.hasSub {
		return
	}
	l.hasSub = false
	if l.sub != nil {
		l.sub.Hide()
	}
	l.Refresh()
}

func (l *ctxMenuLayer) Tapped(*fyne.PointEvent) {
	if l.dismiss != nil {
		l.dismiss()
	}
}

func (l *ctxMenuLayer) TappedSecondary(*fyne.PointEvent) {
	if l.dismiss != nil {
		l.dismiss()
	}
}

func (l *ctxMenuLayer) CreateRenderer() fyne.WidgetRenderer {
	return &ctxMenuLayerRenderer{layer: l}
}

type ctxMenuLayerRenderer struct {
	layer *ctxMenuLayer
}

func (r *ctxMenuLayerRenderer) Destroy() {}

func (r *ctxMenuLayerRenderer) Layout(size fyne.Size) {
	if r.layer.main != nil {
		ms := r.layer.main.MinSize()
		r.layer.main.Move(r.layer.mainAt)
		r.layer.main.Resize(ms)
	}
	if r.layer.sub != nil {
		if r.layer.hasSub {
			ss := r.layer.sub.MinSize()
			r.layer.sub.Move(r.layer.subAt)
			r.layer.sub.Resize(ss)
			r.layer.sub.Show()
		} else {
			r.layer.sub.Hide()
		}
	}
	_ = size
}

func (r *ctxMenuLayerRenderer) MinSize() fyne.Size {
	if r.layer.c != nil {
		return r.layer.c.Size()
	}
	return fyne.NewSize(0, 0)
}

func (r *ctxMenuLayerRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, 2)
	if r.layer.main != nil {
		objs = append(objs, r.layer.main)
	}
	if r.layer.sub != nil {
		objs = append(objs, r.layer.sub)
	}
	return objs
}

func (r *ctxMenuLayerRenderer) Refresh() {
	if r.layer.main != nil {
		r.layer.main.Refresh()
	}
	if r.layer.sub != nil {
		r.layer.sub.Refresh()
	}
	canvas.Refresh(r.layer)
}

func buildCtxPanelFromRows(rows []fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameMenuBackground))
	bg.CornerRadius = ctxMenuRadius
	bg.StrokeColor = theme.Color(theme.ColorNameSeparator)
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.New(&ctxMenuLayout{}, rows...))
}

// ctxMenuLayout packs rows tightly — no theme Padding gaps between items.
type ctxMenuLayout struct{}

func (l *ctxMenuLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := ctxMenuPadY
	for _, o := range objects {
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h
	}
}

func (l *ctxMenuLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		ms := o.MinSize()
		if ms.Width > w {
			w = ms.Width
		}
		h += ms.Height
	}
	if w < ctxMenuMinW {
		w = ctxMenuMinW
	}
	return fyne.NewSize(w, h+2*ctxMenuPadY)
}

type ctxMenuRow struct {
	widget.BaseWidget
	label   string
	expand  bool
	onTap   func()
	onHover func()
	hovered bool
}

func newCtxMenuRow(label string, expand bool, onTap func()) *ctxMenuRow {
	r := &ctxMenuRow{label: label, expand: expand, onTap: onTap}
	r.ExtendBaseWidget(r)
	return r
}

func (r *ctxMenuRow) MinSize() fyne.Size {
	w := fyne.MeasureText(r.label, theme.Size(theme.SizeNameText), fyne.TextStyle{}).Width + 2*ctxMenuPadX
	if r.expand {
		w += ctxMenuArrowW + ctxMenuPadX/2
	}
	if w < ctxMenuMinW {
		w = ctxMenuMinW
	}
	return fyne.NewSize(w, ctxMenuRowH)
}

func (r *ctxMenuRow) Tapped(*fyne.PointEvent) {
	if r.expand && r.onHover != nil {
		r.onHover()
		return
	}
	if r.onTap != nil {
		r.onTap()
	}
}

func (r *ctxMenuRow) MouseIn(*desktop.MouseEvent) {
	r.hovered = true
	r.Refresh()
	if r.onHover != nil {
		r.onHover()
	}
}

func (r *ctxMenuRow) MouseOut() {
	r.hovered = false
	r.Refresh()
}

func (r *ctxMenuRow) MouseMoved(*desktop.MouseEvent) {}

func (r *ctxMenuRow) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 4
	text := canvas.NewText(r.label, theme.Color(theme.ColorNameForeground))
	text.TextSize = theme.Size(theme.SizeNameText)
	arrow := canvas.NewText("›", theme.Color(theme.ColorNamePlaceHolder))
	arrow.TextSize = theme.Size(theme.SizeNameText) + 2
	arrow.Alignment = fyne.TextAlignCenter
	if !r.expand {
		arrow.Hide()
	}
	return &ctxMenuRowRenderer{
		row:     r,
		bg:      bg,
		text:    text,
		arrow:   arrow,
		objects: []fyne.CanvasObject{bg, text, arrow},
	}
}

type ctxMenuRowRenderer struct {
	row     *ctxMenuRow
	bg      *canvas.Rectangle
	text    *canvas.Text
	arrow   *canvas.Text
	objects []fyne.CanvasObject
}

func (r *ctxMenuRowRenderer) Destroy() {}

func (r *ctxMenuRowRenderer) Layout(size fyne.Size) {
	r.bg.Resize(fyne.NewSize(size.Width-4, size.Height-2))
	r.bg.Move(fyne.NewPos(2, 1))
	ts := r.text.MinSize()
	right := size.Width - ctxMenuPadX
	if r.row.expand {
		as := r.arrow.MinSize()
		r.arrow.Resize(fyne.NewSize(ctxMenuArrowW, as.Height))
		r.arrow.Move(fyne.NewPos(size.Width-ctxMenuPadX-ctxMenuArrowW, (size.Height-as.Height)/2))
		right = size.Width - ctxMenuPadX - ctxMenuArrowW - 4
	}
	r.text.Move(fyne.NewPos(ctxMenuPadX, (size.Height-ts.Height)/2))
	r.text.Resize(fyne.NewSize(right-ctxMenuPadX, ts.Height))
}

func (r *ctxMenuRowRenderer) MinSize() fyne.Size {
	return r.row.MinSize()
}

func (r *ctxMenuRowRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *ctxMenuRowRenderer) Refresh() {
	r.text.Text = r.row.label
	r.text.Color = theme.Color(theme.ColorNameForeground)
	r.text.TextSize = theme.Size(theme.SizeNameText)
	r.arrow.Color = theme.Color(theme.ColorNamePlaceHolder)
	r.arrow.TextSize = theme.Size(theme.SizeNameText) + 2
	if r.row.expand {
		r.arrow.Show()
	} else {
		r.arrow.Hide()
	}
	if r.row.hovered {
		r.bg.FillColor = theme.Color(theme.ColorNameHover)
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.Refresh()
	r.text.Refresh()
	r.arrow.Refresh()
	canvas.Refresh(r.row)
}
