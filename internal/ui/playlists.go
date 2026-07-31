package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/model"
)

func (a *App) reloadPlaylists() {
	pls, err := a.store.ListPlaylists()
	if err != nil {
		a.setStatus("Failed to load playlists: " + err.Error())
		return
	}
	a.playlists = pls
	a.playlistList.Refresh()
}

func (a *App) selectPlaylist(id int64) {
	a.selectedPL = id
	a.selectedTrack = -1
	tracks, err := a.store.ListTracks(id)
	if err != nil {
		a.setStatus("Failed to load tracks: " + err.Error())
		return
	}
	a.allTracks = tracks
	a.applyTrackFilter()
	a.trackList.Refresh()
	a.trackList.UnselectAll()
	a.fyneApp.Preferences().SetInt(prefLastPlaylistID, int(id))
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
	if pl == nil {
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
	if pl == nil {
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
		if len(a.playlists) > 0 {
			a.selectPlaylist(a.playlists[0].ID)
			a.playlistList.Select(0)
		}
	}, a.win)
	d.SetConfirmImportance(widget.DangerImportance)
	d.Show()
	stripDialogTitle(a.win.Canvas())
	padTopModal(a.win.Canvas())
}

// showPlaylistContextMenu shows right-click Rename/Delete for a user playlist.
func (a *App) showPlaylistContextMenu(index int, pos fyne.Position) {
	if index < 0 || index >= len(a.playlists) {
		return
	}
	pl := a.playlists[index]
	if pl.System {
		return
	}
	showCompactMenu(a.win.Canvas(), pos, []ctxMenuAction{
		{label: "Rename", action: a.promptRenamePlaylist},
		{label: "Delete", action: a.deleteSelectedPlaylist, danger: true},
	})
}
