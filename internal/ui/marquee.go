package ui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	marqueeSpeedPx = float32(40) // pixels per second
	marqueePause   = 1500 * time.Millisecond
	marqueeTick    = 30 * time.Millisecond
	marqueeEndGap  = float32(48) // blank gap before looping
)

// MarqueeTitle scrolls long now-playing text inside a clipped viewport.
type MarqueeTitle struct {
	widget.BaseWidget

	text      *canvas.Text
	clip      *container.Clip
	mu        sync.Mutex
	offset    float32
	phase     marqueePhase
	holdUntil time.Time
	stop      chan struct{}
	started   bool
}

type marqueePhase int

const (
	marqueeHoldStart marqueePhase = iota
	marqueeScroll
	marqueeHoldEnd
)

// NewMarqueeTitle creates a bold scrolling title label.
func NewMarqueeTitle(initial string) *MarqueeTitle {
	m := &MarqueeTitle{
		text: canvas.NewText(initial, theme.Color(theme.ColorNameForeground)),
	}
	m.text.TextStyle = fyne.TextStyle{Bold: true}
	m.text.TextSize = theme.Size(theme.SizeNameText)
	m.clip = container.NewClip(m.text)
	m.ExtendBaseWidget(m)
	return m
}

// SetText updates the displayed title and resets scroll position.
func (m *MarqueeTitle) SetText(s string) {
	m.mu.Lock()
	m.offset = 0
	m.phase = marqueeHoldStart
	m.holdUntil = time.Now().Add(marqueePause)
	m.mu.Unlock()

	m.text.Text = s
	m.text.Color = theme.Color(theme.ColorNameForeground)
	m.text.TextSize = theme.Size(theme.SizeNameText)
	m.text.Refresh()
	m.layoutText()
	m.Refresh()
}

// Start begins the scroll loop (safe to call once).
func (m *MarqueeTitle) Start() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.stop = make(chan struct{})
	stop := m.stop
	m.holdUntil = time.Now().Add(marqueePause)
	m.mu.Unlock()

	go func() {
		t := time.NewTicker(marqueeTick)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				fyne.Do(m.tick)
			}
		}
	}()
}

// Stop ends the scroll loop.
func (m *MarqueeTitle) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return
	}
	close(m.stop)
	m.started = false
}

func (m *MarqueeTitle) CreateRenderer() fyne.WidgetRenderer {
	return &marqueeRenderer{m: m, objects: []fyne.CanvasObject{m.clip}}
}

func (m *MarqueeTitle) MinSize() fyne.Size {
	h := m.text.MinSize().Height
	if h < 1 {
		h = theme.Size(theme.SizeNameText) + theme.Size(theme.SizeNameInnerPadding)
	}
	return fyne.NewSize(40, h)
}

func (m *MarqueeTitle) tick() {
	m.text.Color = theme.Color(theme.ColorNameForeground)
	m.text.TextSize = theme.Size(theme.SizeNameText)

	viewW := m.Size().Width
	textW := m.text.MinSize().Width
	maxOff := textW - viewW + marqueeEndGap
	if viewW < 1 || textW <= viewW+1 {
		m.mu.Lock()
		m.offset = 0
		m.phase = marqueeHoldStart
		m.mu.Unlock()
		m.layoutText()
		return
	}

	m.mu.Lock()
	now := time.Now()
	switch m.phase {
	case marqueeHoldStart, marqueeHoldEnd:
		if now.Before(m.holdUntil) {
			m.mu.Unlock()
			m.layoutText()
			return
		}
		if m.phase == marqueeHoldEnd {
			m.offset = 0
			m.phase = marqueeHoldStart
			m.holdUntil = now.Add(marqueePause)
			m.mu.Unlock()
			m.layoutText()
			return
		}
		m.phase = marqueeScroll
	case marqueeScroll:
		step := marqueeSpeedPx * float32(marqueeTick) / float32(time.Second)
		m.offset += step
		if m.offset >= maxOff {
			m.offset = maxOff
			m.phase = marqueeHoldEnd
			m.holdUntil = now.Add(marqueePause)
		}
	}
	m.mu.Unlock()
	m.layoutText()
}

func (m *MarqueeTitle) layoutText() {
	textMin := m.text.MinSize()
	m.text.Resize(textMin)
	m.mu.Lock()
	off := m.offset
	m.mu.Unlock()
	// Keep Y at 0: Clip.Layout expands the text to the viewport height and
	// Fyne's text painter already centers glyphs vertically in a taller object.
	// Manual Y centering here would double-offset and push the title too low.
	// Negative X shifts text left so the title reads left→right through the overflow.
	m.text.Move(fyne.NewPos(-off, 0))
	m.clip.Refresh()
}

type marqueeRenderer struct {
	m       *MarqueeTitle
	objects []fyne.CanvasObject
}

func (r *marqueeRenderer) Destroy() {}

func (r *marqueeRenderer) Layout(size fyne.Size) {
	r.objects[0].Resize(size)
	r.m.layoutText()
}

func (r *marqueeRenderer) MinSize() fyne.Size {
	return r.m.MinSize()
}

func (r *marqueeRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *marqueeRenderer) Refresh() {
	r.m.text.Color = theme.Color(theme.ColorNameForeground)
	r.m.text.TextSize = theme.Size(theme.SizeNameText)
	r.m.text.Refresh()
	r.Layout(r.m.Size())
	canvas.Refresh(r.m)
}
