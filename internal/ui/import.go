package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	"github.com/rdurica/robsong/internal/audio"
	"github.com/rdurica/robsong/internal/library"
)

func (a *App) importFiles() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		uri := reader.URI()
		_ = reader.Close()
		a.importPaths([]string{uri.Path()})
	}, a.win)
	fd.SetFilter(storage.NewExtensionFileFilter(audio.SupportedExtensions))
	fd.Show()
	fd.Resize(fyne.NewSize(800, 600))
	padTopModal(a.win.Canvas())
}

func (a *App) importFolder() {
	fd := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		a.importPaths([]string{uri.Path()})
	}, a.win)
	fd.Show()
	fd.Resize(fyne.NewSize(800, 600))
	padTopModal(a.win.Canvas())
}

func (a *App) importPaths(paths []string) {
	tracks, err := library.ImportPaths(paths)
	if err != nil {
		a.setStatus("Import error: " + err.Error())
		return
	}
	if len(tracks) == 0 {
		a.setStatus("No supported audio files found (mp3, flac, wav, ogg/vorbis)")
		return
	}
	libID, err := a.store.LibraryID()
	if err != nil {
		a.setStatus(err.Error())
		return
	}
	target := a.selectedPL
	if target == 0 {
		target = libID
	}
	if err := a.store.AddTracks(libID, tracks); err != nil {
		a.setStatus(err.Error())
		return
	}
	if target != libID {
		_ = a.store.AddTracks(target, tracks)
	}
	a.selectPlaylist(a.selectedPL)
	a.setStatus(fmt.Sprintf("Imported %d track(s)", len(tracks)))
}
