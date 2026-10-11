package ui

import (
	"errors"
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/model"
)

// maxRescanListRows is the number of missing tracks shown before the dialog list scrolls.
const maxRescanListRows = 10

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
	for {
		err := a.player.Play(t.Path)
		if err == nil {
			delete(a.missing, t.Path)
			a.updateNowPlaying()
			a.trackList.Refresh()
			return true
		}
		if !errors.Is(err, os.ErrNotExist) {
			a.setStatus("Play error: " + err.Error())
			return false
		}
		a.markMissing(t.Path)
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
	head := widget.NewRichText(&widget.TextSegment{
		Text:  "These tracks no longer exist on disk. Remove them from the playlist?",
		Style: msgStyle,
	})
	head.Wrapping = fyne.TextWrapWord
	rows := make([]fyne.CanvasObject, 0, len(missing))
	for _, t := range missing {
		line := t.DisplayTitle()
		if art := t.DisplayArtist(); art != "" {
			line += " — " + art
		}
		l := widget.NewLabel(line)
		l.Truncation = fyne.TextTruncateEllipsis
		rows = append(rows, l)
	}
	var list fyne.CanvasObject = container.NewVBox(rows...)
	if len(rows) > maxRescanListRows {
		sc := container.NewVScroll(list)
		sc.SetMinSize(fyne.NewSize(0, rows[0].MinSize().Height*maxRescanListRows))
		list = sc
	}
	d := dialog.NewCustomConfirm("", "Remove", "Cancel", container.NewBorder(head, nil, nil, nil, list), func(ok bool) {
		if ok {
			a.removeMissingTracks(missing)
		}
	}, a.win)
	d.SetConfirmImportance(widget.DangerImportance)
	d.Show()
	d.Resize(fyne.NewSize(480, d.MinSize().Height))
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
	if cur, ok := a.queue.Current(); ok && containsPath(paths, cur.Path) {
		rest := []model.Track{cur}
		seen := false
		for _, t := range prev {
			if seen && !containsPath(paths, t.Path) {
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

func containsPath(paths []string, p string) bool {
	for _, q := range paths {
		if q == p {
			return true
		}
	}
	return false
}
