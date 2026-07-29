package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	playBtnDiameter = float32(36)
	// Icon inset as a fraction of the circle diameter (smaller = larger glyph).
	playBtnIconPad = float32(0.16)
	// Play triangles look left-heavy; tiny right nudge for optical center.
	playBtnOpticalNudge = float32(0.03)
)

var (
	playBtnBg      = color.NRGBA{R: 0x1c, G: 0x22, B: 0x2c, A: 0xff}
	playBtnBgHover = color.NRGBA{R: 0x2a, G: 0x32, B: 0x3e, A: 0xff}
	playBtnBgPress = color.NRGBA{R: 0x11, G: 0x15, B: 0x1c, A: 0xff}
)

// PlayButton is a circular play/pause control with inverted (black) background.
type PlayButton struct {
	widget.BaseWidget

	icon     fyne.Resource
	onTapped func()
	hovered  bool
	pressed  bool
}

// NewPlayButton creates a circular black play button with a light icon.
func NewPlayButton(icon fyne.Resource, tapped func()) *PlayButton {
	b := &PlayButton{icon: icon, onTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

// SetIcon updates the play/pause glyph.
func (b *PlayButton) SetIcon(icon fyne.Resource) {
	b.icon = icon
	b.Refresh()
}

// MinSize is a fixed circle matching the control row.
func (b *PlayButton) MinSize() fyne.Size {
	return fyne.NewSquareSize(playBtnDiameter)
}

// Tapped handles clicks.
func (b *PlayButton) Tapped(*fyne.PointEvent) {
	if b.onTapped != nil {
		b.onTapped()
	}
}

// MouseIn marks hover for a slightly lighter fill.
func (b *PlayButton) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
}

// MouseOut clears hover.
func (b *PlayButton) MouseOut() {
	b.hovered = false
	b.Refresh()
}

// MouseMoved is required by desktop.Hoverable.
func (b *PlayButton) MouseMoved(*desktop.MouseEvent) {}

// MouseDown darkens the circle while pressed.
func (b *PlayButton) MouseDown(*desktop.MouseEvent) {
	b.pressed = true
	b.Refresh()
}

// MouseUp restores the normal/hover fill.
func (b *PlayButton) MouseUp(*desktop.MouseEvent) {
	b.pressed = false
	b.Refresh()
}

// Cursor shows a pointer over the control.
func (b *PlayButton) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (b *PlayButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewCircle(playBtnBg)
	img := canvas.NewImageFromResource(b.tintedIcon())
	img.FillMode = canvas.ImageFillContain
	r := &playButtonRenderer{btn: b, bg: bg, icon: img}
	r.objects = []fyne.CanvasObject{bg, img}
	return r
}

func (b *PlayButton) tintedIcon() fyne.Resource {
	if b.icon == nil {
		return nil
	}
	// Light icon on the black circle (inverted look).
	return theme.NewColoredResource(b.icon, theme.ColorNameForegroundOnPrimary)
}

func (b *PlayButton) fillColor() color.Color {
	switch {
	case b.pressed:
		return playBtnBgPress
	case b.hovered:
		return playBtnBgHover
	default:
		return playBtnBg
	}
}

func (b *PlayButton) isPlayGlyph() bool {
	if b.icon == nil {
		return false
	}
	name := strings.ToLower(b.icon.Name())
	return strings.Contains(name, "play") && !strings.Contains(name, "pause")
}

type playButtonRenderer struct {
	btn     *PlayButton
	bg      *canvas.Circle
	icon    *canvas.Image
	objects []fyne.CanvasObject
}

func (r *playButtonRenderer) Destroy() {}

func (r *playButtonRenderer) Layout(size fyne.Size) {
	// HBox may stretch height; keep a true circle centered in the allocated area.
	side := size.Width
	if size.Height < side {
		side = size.Height
	}
	x := (size.Width - side) / 2
	y := (size.Height - side) / 2
	r.bg.Resize(fyne.NewSquareSize(side))
	r.bg.Move(fyne.NewPos(x, y))

	pad := side * playBtnIconPad
	iconSide := side - pad*2
	nudgeX := float32(0)
	if r.btn.isPlayGlyph() {
		nudgeX = side * playBtnOpticalNudge
	}
	// Center the glyph box, then apply optical nudge (play only).
	ix := x + (side-iconSide)/2 + nudgeX
	iy := y + (side-iconSide)/2
	r.icon.Resize(fyne.NewSquareSize(iconSide))
	r.icon.Move(fyne.NewPos(ix, iy))
}

func (r *playButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSquareSize(playBtnDiameter)
}

func (r *playButtonRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *playButtonRenderer) Refresh() {
	r.bg.FillColor = r.btn.fillColor()
	r.bg.Refresh()
	r.icon.Resource = r.btn.tintedIcon()
	r.icon.Refresh()
	r.Layout(r.btn.Size())
	canvas.Refresh(r.btn)
}
