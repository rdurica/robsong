package audio

import (
	"fmt"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/speaker"
)

const (
	sampleRate      = beep.SampleRate(44100)
	bufferSize      = time.Second / 20 // ~50ms — keeps volume changes responsive while dragging
	resampleQuality = 16               // higher quality for music (1–64)
)

// Snapshot is a lock-light view of playback state for UI updates.
type Snapshot struct {
	Position time.Duration
	Duration time.Duration
	Progress float64
	Playing  bool
	Paused   bool
}

// Player wraps beep playback with pause, seek, volume and end callback.
type Player struct {
	mu sync.Mutex

	ctrl     *beep.Ctrl
	volume   *effects.Volume
	analyzer *analyzer
	streamer beep.StreamSeekCloser
	format   beep.Format
	playing  bool
	paused   bool
	vol      float64 // 0..1 linear
	onEnded  func()
	gen      uint64

	// Wall-clock position tracking avoids speaker.Lock() from the UI ticker
	// (frequent locks starve the audio callback → audible ticks/crackles).
	clockStart  time.Time
	clockOffset time.Duration
	duration    time.Duration
}

// New creates a Player. Call Close when done.
func New() (*Player, error) {
	if err := speaker.Init(sampleRate, sampleRate.N(bufferSize)); err != nil {
		return nil, fmt.Errorf("speaker init: %w", err)
	}
	return &Player{vol: 0.8}, nil
}

// SetOnEnded registers a callback invoked when the current track finishes naturally.
func (p *Player) SetOnEnded(fn func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onEnded = fn
}

// Play loads and plays path. Stops any current playback first.
func (p *Player) Play(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopLocked()

	streamer, format, err := Decode(path)
	if err != nil {
		return err
	}

	var s beep.Streamer = streamer
	if format.SampleRate != sampleRate {
		s = beep.Resample(resampleQuality, format.SampleRate, sampleRate, streamer)
	}

	an := newAnalyzer(s)
	ctrl := &beep.Ctrl{Streamer: an, Paused: false}
	vol := &effects.Volume{
		Streamer: ctrl,
		Base:     2,
		Volume:   linearToBeep(p.vol),
		Silent:   p.vol <= 0.001,
	}

	p.streamer = streamer
	p.format = format
	p.analyzer = an
	p.ctrl = ctrl
	p.volume = vol
	p.playing = true
	p.paused = false
	p.duration = format.SampleRate.D(streamer.Len())
	p.clockOffset = 0
	p.clockStart = time.Now()
	p.gen++
	gen := p.gen
	ended := p.onEnded

	speaker.Play(beep.Seq(vol, beep.Callback(func() {
		p.mu.Lock()
		if gen != p.gen {
			p.mu.Unlock()
			return
		}
		p.playing = false
		p.paused = false
		p.analyzer = nil
		cb := ended
		p.mu.Unlock()
		if cb != nil {
			cb()
		}
	})))
	return nil
}

// TogglePause toggles pause state. Returns whether now paused.
func (p *Player) TogglePause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctrl == nil {
		return false
	}
	if p.ctrl.Paused {
		p.resumeLocked()
		return false
	}
	p.pauseLocked()
	return true
}

func (p *Player) pauseLocked() {
	if p.ctrl == nil || p.ctrl.Paused {
		return
	}
	p.clockOffset = p.positionApproxLocked()
	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
	p.paused = true
}

func (p *Player) resumeLocked() {
	if p.ctrl == nil || !p.ctrl.Paused {
		return
	}
	speaker.Lock()
	p.ctrl.Paused = false
	speaker.Unlock()
	p.paused = false
	p.clockStart = time.Now()
}

// Stop stops playback.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func (p *Player) stopLocked() {
	p.gen++
	speaker.Clear()
	if p.streamer != nil {
		_ = p.streamer.Close()
		p.streamer = nil
	}
	p.ctrl = nil
	p.volume = nil
	p.analyzer = nil
	p.playing = false
	p.paused = false
	p.duration = 0
	p.clockOffset = 0
}

// Spectrum copies current FFT band levels (0..1) for the visualizer.
// When paused or stopped, bands gently decay.
func (p *Player) Spectrum(dst []float64) []float64 {
	p.mu.Lock()
	an := p.analyzer
	paused := p.paused
	playing := p.playing
	p.mu.Unlock()
	if an == nil || !playing {
		if dst == nil || len(dst) < SpectrumBands {
			dst = make([]float64, SpectrumBands)
		} else {
			for i := range dst[:SpectrumBands] {
				dst[i] = 0
			}
			dst = dst[:SpectrumBands]
		}
		return dst
	}
	if paused {
		an.Decay(0.88)
	}
	return an.Levels(dst)
}

// IsPlaying reports active (not stopped) playback, including paused.
func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

// SetVolume sets linear volume 0..1.
// Updates the live gain without speaker.Lock so dragging the slider stays smooth
// and audible immediately (locking here stalls both UI drag and the audio callback).
func (p *Player) SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.mu.Lock()
	p.vol = v
	vol := p.volume
	p.mu.Unlock()
	if vol != nil {
		vol.Volume = linearToBeep(v)
		vol.Silent = v <= 0.001
	}
}

// Volume returns current linear volume 0..1.
func (p *Player) Volume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.vol
}

// Position returns estimated playback position without locking the speaker.
func (p *Player) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.positionApproxLocked()
}

func (p *Player) positionApproxLocked() time.Duration {
	if !p.playing {
		return 0
	}
	pos := p.clockOffset
	if !p.paused {
		pos += time.Since(p.clockStart)
	}
	if p.duration > 0 && pos > p.duration {
		pos = p.duration
	}
	if pos < 0 {
		pos = 0
	}
	return pos
}

// Duration returns total track duration.
func (p *Player) Duration() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.duration
}

// Seek seeks to a position within the track.
func (p *Player) Seek(pos time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.streamer == nil {
		return nil
	}
	if pos < 0 {
		pos = 0
	}
	if p.duration > 0 && pos > p.duration {
		pos = p.duration
	}
	samples := p.format.SampleRate.N(pos)
	if samples < 0 {
		samples = 0
	}
	if samples > p.streamer.Len() {
		samples = p.streamer.Len()
	}
	speaker.Lock()
	err := p.streamer.Seek(samples)
	speaker.Unlock()
	if err != nil {
		return err
	}
	p.clockOffset = p.format.SampleRate.D(samples)
	p.clockStart = time.Now()
	return nil
}

// Snapshot returns UI state in one lock (no speaker lock).
func (p *Player) Snapshot() Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	pos := p.positionApproxLocked()
	var prog float64
	if p.duration > 0 {
		prog = float64(pos) / float64(p.duration)
	}
	return Snapshot{
		Position: pos,
		Duration: p.duration,
		Progress: prog,
		Playing:  p.playing,
		Paused:   p.paused,
	}
}

// Close releases speaker resources.
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func linearToBeep(v float64) float64 {
	if v <= 0 {
		return -8
	}
	// Map 0..1 → -5..0 (0 = unity gain for effects.Volume).
	return (v - 1) * 5
}
