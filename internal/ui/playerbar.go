package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) buildPlayerBar() fyne.CanvasObject {
	a.spectrum = NewSpectrumViz()
	a.spectrum.SetSource(a.player.Spectrum)

	a.nowTitle = NewMarqueeTitle("Nothing playing")

	a.posLabel = widget.NewLabel("0:00")
	a.posLabel.Importance = widget.LowImportance
	a.durLabel = widget.NewLabel("0:00")
	a.durLabel.Importance = widget.LowImportance
	a.durLabel.Alignment = fyne.TextAlignTrailing

	a.progress = widget.NewSlider(0, 1)
	a.progress.Step = 0.001
	a.progress.OnChanged = func(float64) {
		if a.progressUpdating {
			return
		}
		a.seeking = true
	}
	a.progress.OnChangeEnded = func(v float64) {
		if a.progressUpdating {
			return
		}
		dur := a.player.Duration()
		if dur > 0 {
			_ = a.player.Seek(time.Duration(float64(dur) * v))
		}
		a.seeking = false
	}

	a.volume = widget.NewSlider(0, 1)
	a.volume.Step = 0.01
	a.volume.SetValue(a.player.Volume())
	a.volume.OnChanged = a.onVolumeChanged

	prev := iconBtn(theme.MediaSkipPreviousIcon(), a.prev)
	a.playBtn = NewPlayButton(theme.MediaPlayIcon(), a.togglePlay)
	next := iconBtn(theme.MediaSkipNextIcon(), a.next)

	controls := container.NewHBox(prev, a.playBtn, next)

	timeW := float32(44)
	posBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(timeW, 24)), a.posLabel)
	durBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(timeW, 24)), a.durLabel)
	seek := container.NewBorder(nil, nil, posBox, durBox, a.progress)

	a.muteBtn = iconBtn(theme.VolumeUpIcon(), a.toggleMute)
	volBox := container.NewCenter(container.NewHBox(
		a.muteBtn,
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, playBtnDiameter)), a.volume),
	))
	top := container.New(&playerTopLayout{}, a.nowTitle, controls, volBox)

	return container.NewVBox(
		a.spectrum,
		top,
		seek,
	)
}

func (a *App) toggleMute() {
	if a.muted {
		v := a.volumeBeforeMute
		if v <= 0 {
			v = 0.8
		}
		a.muted = false
		a.player.SetVolume(v)
		a.volume.SetValue(v)
		a.syncMuteIcon()
		return
	}
	if a.volume.Value > 0 {
		a.volumeBeforeMute = a.volume.Value
	}
	a.muted = true
	a.player.SetVolume(0)
	a.volume.SetValue(0)
	a.syncMuteIcon()
}

func (a *App) onVolumeChanged(v float64) {
	a.player.SetVolume(v)
	if v > 0.05 {
		a.volumeBeforeMute = v
	}
	if a.muted && v > 0 {
		a.muted = false
		a.syncMuteIcon()
		return
	}
	if !a.muted && v <= 0.001 {
		if a.volumeBeforeMute <= 0 {
			a.volumeBeforeMute = 0.8
		}
		a.muted = true
		a.syncMuteIcon()
	}
}

func (a *App) syncMuteIcon() {
	if a.muteBtn == nil {
		return
	}
	if a.muted {
		a.muteBtn.SetIcon(theme.NewColoredResource(theme.VolumeMuteIcon(), theme.ColorNameError))
		return
	}
	a.muteBtn.SetIcon(theme.VolumeUpIcon())
}

func (a *App) startProgressTicker() {
	go func() {
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			if a.seeking {
				continue
			}
			snap := a.player.Snapshot()
			fyne.Do(func() {
				a.progressUpdating = true
				a.progress.Value = snap.Progress
				a.progress.Refresh()
				a.progressUpdating = false
				a.posLabel.SetText(formatDur(snap.Position))
				a.durLabel.SetText(formatDur(snap.Duration))
				playing := snap.Playing && !snap.Paused
				a.spectrum.SetActive(playing)
				if playing {
					a.playBtn.SetIcon(theme.MediaPauseIcon())
				} else {
					a.playBtn.SetIcon(theme.MediaPlayIcon())
				}
			})
		}
	}()
}

// playFrom starts playback at index and continues through the rest of the list.
func (a *App) playFrom(index int) {
	if index < 0 || index >= len(a.tracks) {
		return
	}
	a.selectedTrack = index
	a.queue.Set(a.tracks[index:])
	a.playCurrent()
	a.trackList.Refresh()
}

func (a *App) playCurrent() {
	t, ok := a.queue.Current()
	if !ok {
		return
	}
	if err := a.player.Play(t.Path); err != nil {
		a.setStatus("Play error: " + err.Error())
		return
	}
	a.spectrum.SetActive(true)
	a.updateNowPlaying()
	a.trackList.Refresh()
	a.setStatus("")
}

func (a *App) togglePlay() {
	if !a.player.IsPlaying() {
		if _, ok := a.queue.Current(); ok {
			a.playCurrent()
			return
		}
		if len(a.tracks) > 0 {
			start := a.selectedTrack
			if start < 0 {
				start = 0
			}
			a.playFrom(start)
			return
		}
		a.setStatus("No tracks to play")
		return
	}
	a.player.TogglePause()
}

func (a *App) next() {
	if t, ok := a.queue.Next(); ok {
		if err := a.player.Play(t.Path); err != nil {
			a.setStatus("Play error: " + err.Error())
			return
		}
		a.updateNowPlaying()
		a.trackList.Refresh()
		return
	}
	a.player.Stop()
	a.updateNowPlaying()
	a.trackList.Refresh()
}

func (a *App) prev() {
	if a.player.IsPlaying() && a.player.Position() > 3*time.Second {
		_ = a.player.Seek(0)
		return
	}
	if t, ok := a.queue.Prev(); ok {
		if err := a.player.Play(t.Path); err != nil {
			a.setStatus("Play error: " + err.Error())
			return
		}
		a.updateNowPlaying()
		a.trackList.Refresh()
		return
	}
	_ = a.player.Seek(0)
}

func (a *App) onTrackEnded() {
	if t, ok := a.queue.Next(); ok {
		if err := a.player.Play(t.Path); err != nil {
			a.setStatus("Play error: " + err.Error())
			return
		}
		a.updateNowPlaying()
		a.trackList.Refresh()
		return
	}
	a.player.Stop()
	a.updateNowPlaying()
	a.trackList.Refresh()
}

func (a *App) updateNowPlaying() {
	t, ok := a.queue.Current()
	if !ok {
		a.nowTitle.SetText("Nothing playing")
		return
	}
	line := t.DisplayTitle()
	if art := t.DisplayArtist(); art != "" {
		line += "  —  " + art
	}
	a.nowTitle.SetText(line)
}
