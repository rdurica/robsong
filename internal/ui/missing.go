package ui

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/model"
)

const (
	rescanDialogWidth = 500
	rescanListHeight  = 260
)

// denseTheme shrinks paddings so list rows are compact.
type denseTheme struct{ fyne.Theme }

func (d denseTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 0
	case theme.SizeNameInnerPadding:
		return 3
	}
	return d.Theme.Size(n)
}

// rescanHeader returns the centred, counted heading of the rescan dialog.
func rescanHeader(n int) string {
	if n == 1 {
		return "1 track no longer exists on disk."
	}
	return fmt.Sprintf("%d tracks no longer exist on disk.", n)
}

// newRescanList builds the framed, fixed-height, scrollable list of missing tracks.
func newRescanList(missing []model.Track) fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, 2*len(missing))
	for i, t := range missing {
		line := t.DisplayTitle()
		if art := t.DisplayArtist(); art != "" {
			line += " — " + art
		}
		l := widget.NewLabel(line)
		l.Truncation = fyne.TextTruncateEllipsis
		if i > 0 {
			rows = append(rows, widget.NewSeparator())
		}
		rows = append(rows, l)
	}
	dense := container.NewThemeOverride(container.NewVBox(rows...), denseTheme{fyne.CurrentApp().Settings().Theme()})
	sc := container.NewVScroll(dense)
	sc.SetMinSize(fyne.NewSize(0, rescanListHeight))
	frame := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	frame.StrokeColor = theme.Color(theme.ColorNameSeparator)
	frame.StrokeWidth = 1
	frame.CornerRadius = theme.InputRadiusSize()
	return container.NewStack(frame, container.NewPadded(sc))
}

// isMissing reports whether a play attempt found the file gone.
func (a *App) isMissing(path string) bool {
	_, ok := a.missing[path]
	return ok
}

func (a *App) markMissing(path string) {
	if a.missing == nil {
		a.missing = map[string]struct{}{}
	}
	a.missing[path] = struct{}{}
}

// playSkippingMissing plays t; when its file is gone it marks the path missing
// and tries the track step returns next, until one plays or none are left.
// Returns true when a track started playing.
func (a *App) playSkippingMissing(t model.Track, step func() (model.Track, bool)) bool {
	var skipped []model.Track
	for {
		err := a.player.Play(t.Path)
		if err == nil {
			delete(a.missing, t.Path)
			a.updateNowPlaying()
			a.trackList.Refresh()
			switch len(skipped) {
			case 0:
			case 1:
				a.setStatus(`Track not found: "` + skipped[0].DisplayTitle() + `"`)
			default:
				a.setStatus(fmt.Sprintf("%d tracks not found, skipped", len(skipped)))
			}
			return true
		}
		if !errors.Is(err, os.ErrNotExist) {
			a.setStatus("Play error: " + err.Error())
			return false
		}
		a.markMissing(t.Path)
		skipped = append(skipped, t)
		a.trackList.Refresh()
		var ok bool
		if t, ok = step(); !ok {
			a.player.Stop()
			a.updateNowPlaying()
			a.setStatus("No playable tracks left")
			return false
		}
	}
}

// findMissingTracks returns the tracks whose file no longer exists, in order.
func findMissingTracks(tracks []model.Track) []model.Track {
	var missing []model.Track
	for _, t := range tracks {
		if _, err := os.Stat(t.Path); errors.Is(err, os.ErrNotExist) {
			missing = append(missing, t)
		}
	}
	return missing
}

// rescanSelectedPlaylist offers to remove tracks whose files are gone from the selected playlist.
func (a *App) rescanSelectedPlaylist() {
	missing := findMissingTracks(a.allTracks)
	if len(missing) == 0 {
		a.setStatus("Rescan: no missing tracks")
		return
	}
	msgStyle := widget.RichTextStyle{Inline: true, Alignment: fyne.TextAlignCenter}
	head := widget.NewRichText(
		&widget.TextSegment{Text: rescanHeader(len(missing)), Style: widget.RichTextStyle{Inline: true, Alignment: fyne.TextAlignCenter, TextStyle: fyne.TextStyle{Bold: true}}},
		&widget.TextSegment{Text: " Remove them from this playlist?", Style: msgStyle},
	)
	head.Wrapping = fyne.TextWrapWord
	list := newRescanList(missing)
	d := dialog.NewCustomConfirm("", "Remove", "Cancel", container.NewBorder(head, nil, nil, nil, container.NewPadded(list)), func(ok bool) {
		if ok {
			a.removeMissingTracks(missing)
		}
	}, a.win)
	d.SetConfirmImportance(widget.DangerImportance)
	d.Resize(fyne.NewSize(min(rescanDialogWidth, a.win.Canvas().Size().Width-32), d.MinSize().Height))
	d.Show()
	stripDialogTitle(a.win.Canvas())
	padTopModal(a.win.Canvas())
}

// removeMissingTracks removes the given tracks from the selected playlist only (never from disk).
func (a *App) removeMissingTracks(missing []model.Track) {
	paths := make([]string, 0, len(missing))
	for _, t := range missing {
		paths = append(paths, t.Path)
	}
	if err := a.store.RemoveTracksByPath(a.selectedPL, paths); err != nil {
		a.setStatus("Remove failed: " + err.Error())
		return
	}
	tracks, err := a.store.ListTracks(a.selectedPL)
	if err != nil {
		a.setStatus("Failed to load tracks: " + err.Error())
		return
	}
	prev := a.allTracks
	a.allTracks = tracks
	a.applyTrackFilter()

	// Same queue handling as removeTrackFromPlaylist: a playing track that was
	// removed stays current and Next continues with what followed it.
	if cur, ok := a.queue.Current(); ok && slices.Contains(paths, cur.Path) {
		rest := []model.Track{cur}
		seen := false
		for _, t := range prev {
			if seen && !slices.Contains(paths, t.Path) {
				rest = append(rest, t)
			}
			seen = seen || t.Path == cur.Path
		}
		a.queue.Set(rest)
	} else {
		a.resyncQueueFromPlaylist()
	}
	for _, p := range paths {
		delete(a.missing, p)
	}

	a.selectedTrack = -1
	a.trackList.UnselectAll()
	a.trackList.Refresh()
	a.setStatus(fmt.Sprintf("Rescan: removed %d missing tracks", len(paths)))
}
