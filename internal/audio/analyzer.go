package audio

import (
	"math"
	"sync"

	"github.com/gopxl/beep/v2"
)

// SpectrumBands is the number of visualizer columns (log-spaced frequency buckets).
const SpectrumBands = 52

const (
	fftSize   = 2048
	fftHalf   = fftSize / 2
	hopSize   = 512
	sampleHz  = 44100
	minFreqHz = 40.0
	maxFreqHz = 16000.0

	// Display mapping: typical music energy after FFT normalization sits
	// roughly in this dB window (0 dB ≈ full-scale tone in one bin).
	dbFloor = -55.0
	dbCeil  = -8.0
)

// analyzer taps PCM samples, runs a real FFT and exposes smoothed band levels.
type analyzer struct {
	streamer beep.Streamer

	mu     sync.Mutex
	buf    []float64
	write  int
	filled int
	hop    int
	window []float64
	re     []float64
	im     []float64
	mags   []float64
	raw    []float64
	bands  []float64
	edges  []int
	gain   float64 // slow auto-gain multiplier applied after dB map
}

func newAnalyzer(s beep.Streamer) *analyzer {
	a := &analyzer{
		streamer: s,
		buf:      make([]float64, fftSize),
		window:   make([]float64, fftSize),
		re:       make([]float64, fftSize),
		im:       make([]float64, fftSize),
		mags:     make([]float64, fftHalf),
		raw:      make([]float64, SpectrumBands),
		bands:    make([]float64, SpectrumBands),
		edges:    make([]int, SpectrumBands+1),
		gain:     1.0,
	}
	for i := 0; i < fftSize; i++ {
		a.window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(fftSize-1)))
	}
	for i := 0; i <= SpectrumBands; i++ {
		t := float64(i) / float64(SpectrumBands)
		freq := minFreqHz * math.Pow(maxFreqHz/minFreqHz, t)
		bin := int(math.Round(freq / sampleHz * float64(fftSize)))
		if bin < 1 {
			bin = 1
		}
		if bin > fftHalf {
			bin = fftHalf
		}
		a.edges[i] = bin
	}
	for i := 1; i <= SpectrumBands; i++ {
		if a.edges[i] <= a.edges[i-1] {
			a.edges[i] = a.edges[i-1] + 1
			if a.edges[i] > fftHalf {
				a.edges[i] = fftHalf
			}
		}
	}
	return a
}

func (a *analyzer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = a.streamer.Stream(samples)
	for i := 0; i < n; i++ {
		mono := 0.5 * (samples[i][0] + samples[i][1])
		a.buf[a.write] = mono
		a.write++
		if a.write >= fftSize {
			a.write = 0
		}
		if a.filled < fftSize {
			a.filled++
		}
		a.hop++
		if a.filled >= fftSize && a.hop >= hopSize {
			a.hop = 0
			a.compute()
		}
	}
	return n, ok
}

func (a *analyzer) Err() error { return nil }

func (a *analyzer) compute() {
	start := a.write
	for i := 0; i < fftSize; i++ {
		idx := start + i
		if idx >= fftSize {
			idx -= fftSize
		}
		a.re[i] = a.buf[idx] * a.window[i]
		a.im[i] = 0
	}
	fft(a.re, a.im)

	// Coherent gain for a Hann window is ~0.5; divide by N/2 so a
	// full-scale sine lands near magnitude 1.0 in its bin.
	norm := 2.0 / float64(fftSize)
	for i := 0; i < fftHalf; i++ {
		a.mags[i] = math.Hypot(a.re[i], a.im[i]) * norm
	}

	var framePeak float64
	for b := 0; b < SpectrumBands; b++ {
		lo := a.edges[b]
		hi := a.edges[b+1]
		if hi <= lo {
			hi = lo + 1
		}
		if hi > fftHalf {
			hi = fftHalf
		}
		// Peak bin in the band — keeps narrow tones from being diluted.
		var mx float64
		for i := lo; i < hi; i++ {
			if a.mags[i] > mx {
				mx = a.mags[i]
			}
		}
		a.raw[b] = mx
		if mx > framePeak {
			framePeak = mx
		}
	}

	// Absolute dB map first (stable baseline), then gentle auto-gain so
	// quiet tracks still move without pinning loud ones to the ceiling.
	span := dbCeil - dbFloor
	var mappedPeak float64
	for b := 0; b < SpectrumBands; b++ {
		db := 20 * math.Log10(a.raw[b]+1e-12)
		level := (db - dbFloor) / span
		if level < 0 {
			level = 0
		}
		if level > 1 {
			level = 1
		}
		a.raw[b] = level
		if level > mappedPeak {
			mappedPeak = level
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Target the loudest band around ~0.85 so there is headroom to jump into.
	const target = 0.85
	if mappedPeak > 0.05 {
		desired := target / mappedPeak
		if desired > 2.5 {
			desired = 2.5
		}
		if desired < 0.7 {
			desired = 0.7
		}
		a.gain += (desired - a.gain) * 0.08
	} else {
		a.gain += (1.0 - a.gain) * 0.05
	}

	for b := 0; b < SpectrumBands; b++ {
		level := a.raw[b] * a.gain
		if level > 1 {
			level = 1
		}
		// Mild lift of mid values for readable capsule rows.
		level = math.Sqrt(level)
		if level < 0.04 {
			level = 0
		}
		if level > a.bands[b] {
			a.bands[b] += (level - a.bands[b]) * 0.75
		} else {
			a.bands[b] += (level - a.bands[b]) * 0.35
		}
	}
}

// Levels copies current smoothed band levels (0..1) into dst (or allocates).
func (a *analyzer) Levels(dst []float64) []float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if dst == nil || len(dst) < SpectrumBands {
		dst = make([]float64, SpectrumBands)
	}
	copy(dst, a.bands)
	return dst[:SpectrumBands]
}

// Decay pulls bands toward zero (used when paused / idle).
func (a *analyzer) Decay(factor float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.bands {
		a.bands[i] *= factor
		if a.bands[i] < 0.01 {
			a.bands[i] = 0
		}
	}
}

func fft(re, im []float64) {
	n := len(re)
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		wlenRe := math.Cos(ang)
		wlenIm := math.Sin(ang)
		for i := 0; i < n; i += length {
			wRe, wIm := 1.0, 0.0
			half := length / 2
			for k := 0; k < half; k++ {
				uRe := re[i+k]
				uIm := im[i+k]
				vRe := re[i+k+half]*wRe - im[i+k+half]*wIm
				vIm := re[i+k+half]*wIm + im[i+k+half]*wRe
				re[i+k] = uRe + vRe
				im[i+k] = uIm + vIm
				re[i+k+half] = uRe - vRe
				im[i+k+half] = uIm - vIm
				nWRe := wRe*wlenRe - wIm*wlenIm
				wIm = wRe*wlenIm + wIm*wlenRe
				wRe = nWRe
			}
		}
	}
}
