package ui

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/model"
)

const (
	// playNowID is the sentinel ID of the in-memory Play Now playlist; nothing
	// is ever stored under it.
	playNowID   int64 = -1
	playNowName       = "Play Now"
)

func (a *App) reloadPlaylists() {
	pls, err := a.store.ListPlaylists()
	if err != nil {
		a.setStatus("Failed to load playlists: " + err.Error())
		return
	}
	if len(a.playNow) > 0 {
		pls = append([]model.Playlist{{ID: playNowID, Name: playNowName}}, pls...)
	}
	a.playlists = pls
	a.playlistList.Refresh()
}

// setPlayNow replaces the Play Now content, selects it and plays from the first track.
func (a *App) setPlayNow(tracks []model.Track) {
	if len(tracks) == 0 {
		return
	}
	a.playNow = slices.Clone(tracks)
	a.reloadPlaylists()
	a.selectPlaylist(playNowID)
	a.playlistList.Select(0)
	a.playFrom(0)
}

// selectDefaultPlaylist selects the Library, or else the first playlist that
// is not Play Now.
func (a *App) selectDefaultPlaylist() {
	idx := -1
	if libID, err := a.store.LibraryID(); err == nil {
		for i, p := range a.playlists {
			if p.ID == libID {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		for i, p := range a.playlists {
			if p.ID != playNowID {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		return
	}
	a.selectPlaylist(a.playlists[idx].ID)
	a.playlistList.Select(idx)
}

// dropEmptyPlayNow removes the Play Now entry once its last track is gone and
// moves the selection to the default playlist.
func (a *App) dropEmptyPlayNow() {
	if a.selectedPL != playNowID || len(a.playNow) > 0 {
		return
	}
	a.reloadPlaylists()
	a.selectDefaultPlaylist()
}

// storeTracks lists the tracks of playlist id; Play Now comes from memory.
func (a *App) storeTracks(id int64) ([]model.Track, error) {
	if id == playNowID {
		return slices.Clone(a.playNow), nil
	}
	return a.store.ListTracks(id)
}

func (a *App) selectPlaylist(id int64) {
	a.selectedPL = id
	a.selectedTrack = -1
	tracks, err := a.storeTracks(id)
	if err != nil {
		a.setStatus("Failed to load tracks: " + err.Error())
		return
	}
	a.allTracks = tracks
	a.applyTrackFilter()
	a.trackList.Refresh()
	a.trackList.UnselectAll()
	if id != playNowID {
		a.fyneApp.Preferences().SetInt(prefLastPlaylistID, int(id))
	}
}

func (a *App) currentPlaylistName() string {
	for _, p := range a.playlists {
		if p.ID == a.selectedPL {
			return p.Name
		}
	}
	return "Tracks"
}

func (a *App) promptNewPlaylist() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Playlist name")
	d := dialog.NewForm("New playlist", "Create", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", entry),
	}, func(ok bool) {
		if !ok || entry.Text == "" {
			return
		}
		p, err := a.store.CreatePlaylist(entry.Text)
		if err != nil {
			a.setStatus("Create failed: " + err.Error())
			return
		}
		a.reloadPlaylists()
		libID, err := a.store.LibraryID()
		if err != nil {
			a.setStatus("Created playlist: " + p.Name)
			return
		}
		a.selectPlaylist(libID)
		for i := range a.playlists {
			if a.playlists[i].ID == libID {
				a.playlistList.Select(i)
				break
			}
		}
		if !a.playlistPanelVisible {
			a.togglePlaylists()
		}
		a.setStatus("Created playlist: " + p.Name)
	}, a.win)
	d.Show()
	d.Resize(fyne.NewSize(480, 220))
	padTopModal(a.win.Canvas())
}

func (a *App) promptRenamePlaylist() {
	var pl *model.Playlist
	for i := range a.playlists {
		if a.playlists[i].ID == a.selectedPL {
			pl = &a.playlists[i]
			break
		}
	}
	if pl == nil || pl.ID == playNowID {
		return
	}
	if pl.System {
		a.setStatus("Cannot rename the Library playlist")
		return
	}
	entry := widget.NewEntry()
	entry.SetText(pl.Name)
	entry.SetPlaceHolder("Playlist name")
	id := pl.ID
	d := dialog.NewForm("Rename playlist", "Rename", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", entry),
	}, func(ok bool) {
		if !ok || entry.Text == "" {
			return
		}
		if err := a.store.RenamePlaylist(id, entry.Text); err != nil {
			a.setStatus("Rename failed: " + err.Error())
			return
		}
		a.reloadPlaylists()
		a.selectPlaylist(id)
		for i := range a.playlists {
			if a.playlists[i].ID == id {
				a.playlistList.Select(i)
				break
			}
		}
		a.setStatus("Renamed playlist: " + entry.Text)
	}, a.win)
	d.Show()
	d.Resize(fyne.NewSize(480, 220))
	padTopModal(a.win.Canvas())
}

func (a *App) deleteSelectedPlaylist() {
	var pl *model.Playlist
	for i := range a.playlists {
		if a.playlists[i].ID == a.selectedPL {
			pl = &a.playlists[i]
			break
		}
	}
	if pl == nil || pl.ID == playNowID {
		return
	}
	if pl.System {
		a.setStatus("Cannot delete the Library playlist")
		return
	}
	msgStyle := widget.RichTextStyle{Inline: true, Alignment: fyne.TextAlignCenter}
	nameStyle := widget.RichTextStyle{
		Inline: true, Alignment: fyne.TextAlignCenter,
		TextStyle: fyne.TextStyle{Bold: true},
	}
	msg := widget.NewRichText(
		&widget.TextSegment{Text: "Are you sure you want to delete playlist ", Style: msgStyle},
		&widget.TextSegment{Text: "\"" + pl.Name + "\"", Style: nameStyle},
		&widget.TextSegment{Text: "?", Style: msgStyle},
	)
	msg.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustomConfirm("", "Delete", "Cancel", msg, func(ok bool) {
		if !ok {
			return
		}
		if err := a.store.DeletePlaylist(pl.ID); err != nil {
			a.setStatus(err.Error())
			return
		}
		a.reloadPlaylists()
		a.selectDefaultPlaylist()
	}, a.win)
	d.SetConfirmImportance(widget.DangerImportance)
	d.Show()
	stripDialogTitle(a.win.Canvas())
	padTopModal(a.win.Canvas())
}

// playlistMenuActions returns the right-click actions for a playlist.
func (a *App) playlistMenuActions(pl model.Playlist) []ctxMenuAction {
	if pl.ID == playNowID {
		return nil
	}
	actions := []ctxMenuAction{{label: "Rescan", action: a.rescanSelectedPlaylist}}
	if pl.System {
		return actions
	}
	return append(actions,
		ctxMenuAction{label: "Rename", action: a.promptRenamePlaylist},
		ctxMenuAction{label: "Delete", action: a.deleteSelectedPlaylist, danger: true},
	)
}

// showPlaylistContextMenu shows right-click actions for a playlist.
func (a *App) showPlaylistContextMenu(index int, pos fyne.Position) {
	if index < 0 || index >= len(a.playlists) {
		return
	}
	showCompactMenu(a.win.Canvas(), pos, a.playlistMenuActions(a.playlists[index]))
}
