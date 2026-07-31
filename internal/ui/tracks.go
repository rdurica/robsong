package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/rdurica/robsong/internal/model"
)

func (a *App) trackFilterActive() bool {
	return a.trackSearch != nil && strings.TrimSpace(a.trackSearch.Text) != ""
}

// filterTracksByQuery returns tracks whose title or artist contains q
// (case-insensitive substring). Empty/whitespace q returns all tracks.
func filterTracksByQuery(tracks []model.Track, q string) []model.Track {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return tracks
	}
	filtered := make([]model.Track, 0, len(tracks))
	for _, t := range tracks {
		titleMatch := strings.Contains(strings.ToLower(t.DisplayTitle()), q)
		artistMatch := strings.Contains(strings.ToLower(t.Artist), q)
		if titleMatch || artistMatch {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// applyTrackFilter sets a.tracks from allTracks using the search entry
// (case-insensitive title or artist match).
func (a *App) applyTrackFilter() {
	q := ""
	if a.trackSearch != nil {
		q = a.trackSearch.Text
	}
	a.tracks = filterTracksByQuery(a.allTracks, q)
	a.trackListHead.SetText(fmt.Sprintf("%s (%d)", a.currentPlaylistName(), len(a.tracks)))
}

func (a *App) moveTrack(from, to int) {
	if a.trackFilterActive() {
		return
	}
	if from == to || from < 0 || to < 0 || from >= len(a.tracks) || to >= len(a.tracks) {
		return
	}
	tracks, err := a.store.MoveTrack(a.selectedPL, from, to)
	if err != nil {
		a.setStatus("Reorder failed: " + err.Error())
		return
	}
	a.allTracks = tracks
	a.applyTrackFilter()
	a.selectedTrack = to
	a.resyncQueueFromPlaylist()
	a.trackList.Refresh()
	a.trackList.Select(to)
}

// removeTrackFromPlaylist removes a track from the current playlist only (never from disk).
func (a *App) removeTrackFromPlaylist(index int) {
	if index < 0 || index >= len(a.tracks) {
		return
	}
	removed := a.tracks[index]
	storePos := -1
	for i, t := range a.allTracks {
		if t.Path == removed.Path {
			storePos = i
			break
		}
	}
	if storePos < 0 {
		return
	}
	playingRemoved := false
	if cur, ok := a.queue.Current(); ok && cur.Path == removed.Path {
		playingRemoved = true
	}

	if err := a.store.RemoveTrackAt(a.selectedPL, storePos); err != nil {
		a.setStatus("Remove failed: " + err.Error())
		return
	}
	tracks, err := a.store.ListTracks(a.selectedPL)
	if err != nil {
		a.setStatus("Failed to load tracks: " + err.Error())
		return
	}
	a.allTracks = tracks
	a.applyTrackFilter()

	if playingRemoved {
		// Keep the removed track as current so the playing file can finish;
		// Next() then continues with what followed it in the playlist.
		rest := append([]model.Track{removed}, a.allTracks[min(storePos, len(a.allTracks)):]...)
		a.queue.Set(rest)
	} else {
		a.resyncQueueFromPlaylist()
	}

	switch {
	case len(a.tracks) == 0:
		a.selectedTrack = -1
		a.trackList.UnselectAll()
	case index < len(a.tracks):
		a.selectedTrack = index
		a.trackList.Select(index)
	default:
		a.selectedTrack = len(a.tracks) - 1
		a.trackList.Select(a.selectedTrack)
	}
	a.trackList.Refresh()
}

// resyncQueueFromPlaylist realigns the playback queue with allTracks from the current track.
func (a *App) resyncQueueFromPlaylist() {
	cur, ok := a.queue.Current()
	if !ok {
		return
	}
	for i, t := range a.allTracks {
		if t.Path == cur.Path {
			a.queue.Set(a.allTracks[i:])
			return
		}
	}
}

// showTrackContextMenu shows right-click actions for a track.
func (a *App) showTrackContextMenu(index int, t model.Track, pos fyne.Position) {
	actions := []ctxMenuAction{
		{
			label:  "Play",
			icon:   theme.MediaPlayIcon(),
			action: func() { a.playFrom(index) },
		},
	}
	var addKids []ctxMenuAction
	for i := range a.playlists {
		pl := a.playlists[i]
		if pl.System || pl.ID == a.selectedPL {
			continue
		}
		addKids = append(addKids, ctxMenuAction{
			label:  pl.Name,
			action: func() { a.addTrackToPlaylist(t, pl) },
		})
	}
	if len(addKids) > 0 {
		actions = append(actions, ctxMenuAction{
			label:    "Add to playlist",
			children: addKids,
		})
	}
	actions = append(actions,
		ctxMenuAction{
			label:  "Properties",
			action: func() { a.showTrackProperties(t) },
		},
		ctxMenuAction{
			label:  "Delete",
			danger: true,
			action: func() { a.removeTrackFromPlaylist(index) },
		},
	)
	showCompactMenu(a.win.Canvas(), pos, actions)
}

func (a *App) addTrackToPlaylist(t model.Track, pl model.Playlist) {
	if err := a.store.AddTracks(pl.ID, []model.Track{t}); err != nil {
		a.setStatus(err.Error())
		return
	}
	if a.selectedPL == pl.ID {
		a.selectPlaylist(pl.ID)
	}
	a.setStatus("Added to " + pl.Name)
}
