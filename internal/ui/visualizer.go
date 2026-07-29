package ui

import (
	"image"
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/audio"
)

const (
	vizRows   = 7
	vizHeight = float32(36)
)

// SpectrumViz is an animated capsule-grid equalizer driven by live FFT bands.
type SpectrumViz struct {
	widget.BaseWidget

	raster   *canvas.Raster
	mu       sync.Mutex
	levels   []float64
	source   func([]float64) []float64
	active   bool
	stopAnim chan struct{}
	started  bool
}

// NewSpectrumViz creates a spectrum visualizer (call Start after window is up).
func NewSpectrumViz() *SpectrumViz {
	v := &SpectrumViz{
		levels: make([]float64, audio.SpectrumBands),
	}
	v.ExtendBaseWidget(v)
	v.raster = canvas.NewRaster(v.draw)
	v.raster.SetMinSize(fyne.NewSize(240, vizHeight))
	return v
}

func (v *SpectrumViz) CreateRenderer() fyne.WidgetRenderer {
	return &spectrumRenderer{viz: v, raster: v.raster}
}

func (v *SpectrumViz) MinSize() fyne.Size {
	return fyne.NewSize(240, vizHeight)
}

// SetSource registers a callback that fills band levels (0..1).
func (v *SpectrumViz) SetSource(fn func([]float64) []float64) {
	v.mu.Lock()
	v.source = fn
	v.mu.Unlock()
}

// SetActive marks whether music is currently audible (playing, not paused).
func (v *SpectrumViz) SetActive(active bool) {
	v.mu.Lock()
	v.active = active
	v.mu.Unlock()
}

// Start begins the animation loop.
func (v *SpectrumViz) Start() {
	v.mu.Lock()
	if v.started {
		v.mu.Unlock()
		return
	}
	v.started = true
	v.stopAnim = make(chan struct{})
	stop := v.stopAnim
	v.mu.Unlock()

	go func() {
		t := time.NewTicker(33 * time.Millisecond)
		defer t.Stop()
		buf := make([]float64, audio.SpectrumBands)
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				v.tick(buf)
				fyne.Do(func() { v.raster.Refresh() })
			}
		}
	}()
}

// Stop ends the animation loop.
func (v *SpectrumViz) Stop() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.started {
		return
	}
	close(v.stopAnim)
	v.started = false
	v.stopAnim = nil
}

func (v *SpectrumViz) tick(buf []float64) {
	v.mu.Lock()
	src := v.source
	active := v.active
	v.mu.Unlock()

	if src == nil {
		v.mu.Lock()
		for i := range v.levels {
			v.levels[i] *= 0.9
		}
		v.mu.Unlock()
		return
	}

	bands := src(buf)
	v.mu.Lock()
	copy(v.levels, bands)
	if !active {
		for i := range v.levels {
			v.levels[i] *= 0.82
		}
	}
	v.mu.Unlock()
}

func (v *SpectrumViz) draw(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w < 4 || h < 4 {
		return img
	}

	v.mu.Lock()
	levels := append([]float64(nil), v.levels...)
	v.mu.Unlock()

	cols := len(levels)
	if cols == 0 {
		return img
	}

	bg := color.NRGBA{R: 0xfa, G: 0xfb, B: 0xfd, A: 0xff}
	fillRect(img, 0, 0, w, h, bg)

	gapX := 2
	gapY := 2
	padX := 6
	padY := 4
	innerW := w - padX*2
	innerH := h - padY*2
	if innerW < cols*2 || innerH < vizRows*2 {
		return img
	}

	cellW := (innerW - gapX*(cols-1)) / cols
	cellH := (innerH - gapY*(vizRows-1)) / vizRows
	if cellW < 2 {
		cellW = 2
	}
	if cellH < 2 {
		cellH = 2
	}

	usedW := cols*cellW + (cols-1)*gapX
	usedH := vizRows*cellH + (vizRows-1)*gapY
	ox := padX + (innerW-usedW)/2
	oy := padY + (innerH-usedH)/2

	inactive := color.NRGBA{R: 0xec, G: 0xf0, B: 0xf5, A: 0xff}

	for col := 0; col < cols; col++ {
		lit := int(math.Ceil(float64(levels[col]) * float64(vizRows)))
		if levels[col] < 0.02 {
			lit = 0
		}
		if lit > vizRows {
			lit = vizRows
		}
		for row := 0; row < vizRows; row++ {
			fromBottom := vizRows - 1 - row
			x0 := ox + col*(cellW+gapX)
			y0 := oy + row*(cellH+gapY)
			if fromBottom < lit {
				// Soft ice-blue gradient: deeper at base, airier toward the tip.
				t := 0.0
				if lit > 1 {
					t = float64(fromBottom) / float64(lit-1)
				}
				fillCapsuleGloss(img, x0, y0, cellW, cellH, softBlue(t))
			} else {
				fillCapsule(img, x0, y0, cellW, cellH, inactive)
			}
		}
	}
	return img
}

// softBlue returns a modern ice/sky blue. t=0 base, t=1 tip (lighter).
func softBlue(t float64) color.NRGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	// base #6BA3E8 → tip #C8E0F8
	return color.NRGBA{
		R: uint8(0x6b + t*float64(0xc8-0x6b)),
		G: uint8(0xa3 + t*float64(0xe0-0xa3)),
		B: uint8(0xe8 + t*float64(0xf8-0xe8)),
		A: 0xff,
	}
}

type spectrumRenderer struct {
	viz    *SpectrumViz
	raster *canvas.Raster
}

func (r *spectrumRenderer) Layout(s fyne.Size) { r.raster.Resize(s) }
func (r *spectrumRenderer) MinSize() fyne.Size {
	return fyne.NewSize(240, vizHeight)
}
func (r *spectrumRenderer) Refresh() { r.raster.Refresh() }
func (r *spectrumRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.raster}
}
func (r *spectrumRenderer) Destroy() {}

func fillRect(img *image.NRGBA, x0, y0, w, h int, c color.NRGBA) {
	bounds := img.Bounds()
	for y := y0; y < y0+h && y < bounds.Max.Y; y++ {
		if y < bounds.Min.Y {
			continue
		}
		for x := x0; x < x0+w && x < bounds.Max.X; x++ {
			if x < bounds.Min.X {
				continue
			}
			img.SetNRGBA(x, y, c)
		}
	}
}

func fillCapsule(img *image.NRGBA, x0, y0, w, h int, c color.NRGBA) {
	fillCapsuleShade(img, x0, y0, w, h, c, false)
}

func fillCapsuleGloss(img *image.NRGBA, x0, y0, w, h int, c color.NRGBA) {
	fillCapsuleShade(img, x0, y0, w, h, c, true)
}

func fillCapsuleShade(img *image.NRGBA, x0, y0, w, h int, c color.NRGBA, gloss bool) {
	if w <= 0 || h <= 0 {
		return
	}
	r := float64(h) / 2
	if r < 1 {
		r = 1
	}
	centerY := float64(h-1) / 2
	left := r
	right := float64(w-1) - r
	if right < left {
		right = left
	}
	r2 := (r + 0.35) * (r + 0.35)

	bounds := img.Bounds()
	for y := 0; y < h; y++ {
		py := y0 + y
		if py < bounds.Min.Y || py >= bounds.Max.Y {
			continue
		}
		cy := float64(y) - centerY
		cy2 := cy * cy
		// Vertical shade: slightly deeper bottom, soft highlight on top third.
		shade := 1.0
		if gloss {
			yn := float64(y) / math.Max(float64(h-1), 1)
			if yn < 0.42 {
				// Specular band near the top edge of the pill.
				shine := 1.0 - yn/0.42
				shade = 1.0 + 0.38*shine
			} else {
				shade = 1.0 - 0.12*((yn-0.42)/0.58)
			}
		}
		for x := 0; x < w; x++ {
			px := x0 + x
			if px < bounds.Min.X || px >= bounds.Max.X {
				continue
			}
			cx := float64(x)
			var d2 float64
			if cx < left {
				dx := cx - left
				d2 = dx*dx + cy2
			} else if cx > right {
				dx := cx - right
				d2 = dx*dx + cy2
			} else {
				d2 = cy2
			}
			if d2 > r2 {
				continue
			}
			pc := c
			if gloss {
				pc = shadeColor(c, shade)
				// Thin bright rim on the upper inner edge.
				edge := math.Sqrt(d2) / math.Sqrt(r2)
				if edge > 0.72 && float64(y) < centerY {
					pc = mixWhite(pc, 0.22*(edge-0.72)/0.28)
				}
			}
			img.SetNRGBA(px, py, pc)
		}
	}
}

func shadeColor(c color.NRGBA, mul float64) color.NRGBA {
	clamp := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	return color.NRGBA{
		R: clamp(float64(c.R) * mul),
		G: clamp(float64(c.G) * mul),
		B: clamp(float64(c.B) * mul),
		A: c.A,
	}
}

func mixWhite(c color.NRGBA, t float64) color.NRGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return color.NRGBA{
		R: uint8(float64(c.R)*(1-t) + 255*t),
		G: uint8(float64(c.G)*(1-t) + 255*t),
		B: uint8(float64(c.B)*(1-t) + 255*t),
		A: c.A,
	}
}
