package audio

import (
	"math"
	"testing"

	"github.com/gopxl/beep/v2"
)

// toneStreamer emits a stereo sine (and optional second tone) forever.
type toneStreamer struct {
	freq1, freq2, amp float64
	phase1, phase2    float64
}

func (t *toneStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	for i := range samples {
		v := t.amp * math.Sin(t.phase1)
		if t.freq2 > 0 {
			v += t.amp * 0.6 * math.Sin(t.phase2)
		}
		samples[i][0], samples[i][1] = v, v
		t.phase1 += 2 * math.Pi * t.freq1 / sampleHz
		if t.freq2 > 0 {
			t.phase2 += 2 * math.Pi * t.freq2 / sampleHz
		}
	}
	return len(samples), true
}

func (t *toneStreamer) Err() error { return nil }

func feed(a *analyzer, frames int) {
	buf := make([][2]float64, 512)
	for i := 0; i < frames; i++ {
		a.Stream(buf)
	}
}

func TestAnalyzerSinePeaksInExpectedBand(t *testing.T) {
	src := &toneStreamer{freq1: 440, amp: 0.5}
	a := newAnalyzer(beep.Streamer(src))
	feed(a, 20) // ~10k samples → several FFT windows

	levels := a.Levels(nil)
	var peakIdx int
	var peak float64
	var sum float64
	for i, v := range levels {
		sum += v
		if v > peak {
			peak = v
			peakIdx = i
		}
	}
	t.Logf("peak=%.3f at band %d, mean=%.3f, levels=%v", peak, peakIdx, sum/float64(len(levels)), fmtBands(levels))

	if peak < 0.35 {
		t.Fatalf("peak level too low: %.3f (expected >= 0.35)", peak)
	}
	if peak > 0.98 {
		t.Fatalf("peak level pinned to ceiling: %.3f", peak)
	}
	// 440Hz should land in a low-mid band, not the last few (highs).
	if peakIdx > SpectrumBands*2/3 {
		t.Fatalf("440Hz peaked at band %d, expected lower third/mid", peakIdx)
	}
	// Dynamic range: not every band lit.
	lit := 0
	for _, v := range levels {
		if v > 0.2 {
			lit++
		}
	}
	if lit > SpectrumBands/2 {
		t.Fatalf("too many bands lit (%d) — poor contrast", lit)
	}
}

func TestAnalyzerSilenceNearZero(t *testing.T) {
	src := &toneStreamer{freq1: 440, amp: 0}
	a := newAnalyzer(beep.Streamer(src))
	feed(a, 20)
	levels := a.Levels(nil)
	for i, v := range levels {
		if v > 0.05 {
			t.Fatalf("silence band %d = %.3f, want ~0", i, v)
		}
	}
}

func fmtBands(levels []float64) string {
	out := make([]byte, 0, len(levels)*4)
	for _, v := range levels {
		n := int(v*10 + 0.5)
		if n > 9 {
			n = 9
		}
		out = append(out, byte('0'+n))
	}
	return string(out)
}

func TestAnalyzerBroadbandHasMidLevels(t *testing.T) {
	// Mix of tones across spectrum at moderate amplitude (music-like).
	src := &multiTone{amps: []float64{0.25, 0.2, 0.15, 0.12, 0.1}, freqs: []float64{80, 220, 440, 2000, 6000}}
	a := newAnalyzer(beep.Streamer(src))
	feed(a, 30)
	levels := a.Levels(nil)
	var peak, sum float64
	lit := 0
	for _, v := range levels {
		sum += v
		if v > peak {
			peak = v
		}
		if v > 0.15 {
			lit++
		}
	}
	mean := sum / float64(len(levels))
	t.Logf("peak=%.3f mean=%.3f lit=%d levels=%s", peak, mean, lit, fmtBands(levels))
	if peak < 0.4 {
		t.Fatalf("peak too low: %.3f", peak)
	}
	if peak > 0.98 {
		t.Fatalf("peak pinned: %.3f", peak)
	}
	if mean < 0.08 || mean > 0.55 {
		t.Fatalf("mean out of expected range: %.3f", mean)
	}
	if lit < 3 {
		t.Fatalf("too few bands lit: %d", lit)
	}
}

type multiTone struct {
	amps, freqs []float64
	phases      []float64
}

func (m *multiTone) Stream(samples [][2]float64) (n int, ok bool) {
	if m.phases == nil {
		m.phases = make([]float64, len(m.freqs))
	}
	for i := range samples {
		var v float64
		for j := range m.freqs {
			v += m.amps[j] * math.Sin(m.phases[j])
			m.phases[j] += 2 * math.Pi * m.freqs[j] / sampleHz
		}
		samples[i][0], samples[i][1] = v, v
	}
	return len(samples), true
}

func (m *multiTone) Err() error { return nil }

func TestAnalyzerQuietMusicStillVisible(t *testing.T) {
	src := &multiTone{amps: []float64{0.04, 0.03, 0.025, 0.02, 0.015}, freqs: []float64{80, 220, 440, 2000, 6000}}
	a := newAnalyzer(beep.Streamer(src))
	feed(a, 40)
	levels := a.Levels(nil)
	var peak float64
	for _, v := range levels {
		if v > peak {
			peak = v
		}
	}
	t.Logf("quiet peak=%.3f levels=%s", peak, fmtBands(levels))
	if peak < 0.35 {
		t.Fatalf("quiet music peak too low: %.3f", peak)
	}
}
