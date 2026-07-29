package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	apptheme "github.com/rdurica/robsong/internal/theme"
)

// searchEntry is a single-line entry with transparent chrome and a custom caret.
// Fyne sizes both the focus ring and the built-in caret from InputBorder; we keep
// that size at 0 (no focus box) and draw our own caret instead.
type searchEntry struct {
	widget.Entry

	caret     *canvas.Rectangle
	clearBtn  *widget.Button
	clearBox  fyne.CanvasObject
	fieldBox  fyne.CanvasObject
	focused   bool
	onChanged func(string)
}

func newSearchEntry() *searchEntry {
	e := &searchEntry{
		caret: canvas.NewRectangle(color.NRGBA{R: 0x5b, G: 0x9b, B: 0xe6, A: 0xff}),
	}
	e.ExtendBaseWidget(e)
	e.SetPlaceHolder("Search…")
	e.Wrapping = fyne.TextWrapOff
	e.Scroll = container.ScrollNone
	e.caret.Hide()
	e.OnCursorChanged = e.syncCaret

	e.clearBtn = widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		e.SetText("")
		if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
			c.Focus(e)
		}
	})
	e.clearBtn.Importance = widget.LowImportance
	e.clearBox = container.NewCenter(container.New(layout.NewGridWrapLayout(fyne.NewSize(28, 28)), e.clearBtn))
	e.clearBox.Hide()

	e.OnChanged = func(s string) {
		e.syncClearBtn()
		if e.onChanged != nil {
			e.onChanged(s)
		}
	}
	return e
}

// SetOnChanged sets the text-change callback (after clear-button visibility sync).
func (e *searchEntry) SetOnChanged(f func(string)) {
	e.onChanged = f
}

func (e *searchEntry) FocusGained() {
	e.focused = true
	e.Entry.FocusGained()
	e.caret.Show()
	e.syncCaret()
}

func (e *searchEntry) FocusLost() {
	e.focused = false
	e.Entry.FocusLost()
	e.caret.Hide()
	e.caret.Refresh()
}

func (e *searchEntry) syncCaret() {
	if !e.focused {
		return
	}
	th := e.Theme()
	textSize := th.Size(theme.SizeNameText)
	lineH := fyne.MeasureText("Ag", textSize, e.TextStyle).Height
	e.caret.Resize(fyne.NewSize(2, lineH))
	e.caret.Move(e.CursorPosition())
	e.caret.Refresh()
}

func (e *searchEntry) syncClearBtn() {
	if e.Text == "" {
		e.clearBox.Hide()
	} else {
		e.clearBox.Show()
	}
	if e.fieldBox != nil {
		e.fieldBox.Refresh()
	} else {
		e.clearBox.Refresh()
	}
}

// field returns the themed entry with caret overlay and a clear button on the right.
func (e *searchEntry) field() fyne.CanvasObject {
	flat := container.NewThemeOverride(e, &apptheme.UnderlineInputTheme{})
	entryStack := container.New(&stackCaretLayout{}, flat, e.caret)
	e.fieldBox = container.NewBorder(nil, nil, nil, e.clearBox, entryStack)
	return e.fieldBox
}

type stackCaretLayout struct{}

func (l *stackCaretLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(size)
}

func (l *stackCaretLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objects[0].MinSize()
}
