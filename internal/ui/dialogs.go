package ui

import (
	"image/color"
	"reflect"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// modalPopUpContent returns the PopUp content of the topmost canvas overlay, if any.
func modalPopUpContent(c fyne.Canvas) *widget.PopUp {
	top := c.Overlays().Top()
	if top == nil {
		return nil
	}
	rv := reflect.ValueOf(top)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	field := rv.FieldByName("Content")
	if !field.IsValid() || !field.CanInterface() {
		return nil
	}
	pop, _ := field.Interface().(*widget.PopUp)
	return pop
}

// stripDialogTitle removes the Fyne dialog title row so body text isn't pushed down by an empty header.
func stripDialogTitle(c fyne.Canvas) {
	pop := modalPopUpContent(c)
	if pop == nil {
		return
	}
	box, ok := pop.Content.(*fyne.Container)
	if !ok || len(box.Objects) < 5 {
		return
	}
	// dialogLayout order: icon, background, content, buttons, title
	box.Objects[4] = layout.NewSpacer()
	pop.Refresh()
}

// padTopModal insets the topmost modal popup so footer buttons aren't flush to the edges.
func padTopModal(c fyne.Canvas) {
	const edge = float32(16)
	pop := modalPopUpContent(c)
	if pop == nil || pop.Content == nil {
		return
	}
	pop.Content = container.New(
		layout.NewCustomPaddedLayout(edge, edge, edge, edge),
		pop.Content,
	)
	pop.Refresh()
}

func (a *App) setStatus(msg string) {
	a.hideToast()
	if msg == "" {
		return
	}

	label := canvas.NewText(msg, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	label.TextSize = theme.TextSize()
	icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.InfoIcon(), theme.ColorNameForegroundOnPrimary))
	icon.SetMinSize(fyne.NewSquareSize(theme.IconInlineSize()))
	icon.FillMode = canvas.ImageFillContain
	row := container.NewHBox(container.NewCenter(icon), container.NewCenter(label))
	bg := canvas.NewRectangle(color.NRGBA{R: 0x1c, G: 0x22, B: 0x2c, A: 0xf2})
	bg.CornerRadius = 6
	content := container.NewStack(bg, container.NewPadded(row))

	pop := widget.NewPopUp(content, a.win.Canvas())
	a.toast = pop
	a.toastToken++
	token := a.toastToken

	size := content.MinSize()
	winSize := a.win.Canvas().Size()
	const pad float32 = 16
	x := winSize.Width - size.Width - pad
	if x < pad {
		x = pad
	}
	pop.ShowAtPosition(fyne.NewPos(x, pad))

	time.AfterFunc(3*time.Second, func() {
		fyne.Do(func() {
			if a.toastToken != token {
				return
			}
			a.hideToast()
		})
	})
}

func (a *App) hideToast() {
	a.toastToken++
	if a.toast != nil {
		a.toast.Hide()
		a.toast = nil
	}
}
