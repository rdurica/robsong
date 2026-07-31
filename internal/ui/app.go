package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/assets"
	"github.com/rdurica/robsong/internal/audio"
	"github.com/rdurica/robsong/internal/model"
	"github.com/rdurica/robsong/internal/playlist"
	"github.com/rdurica/robsong/internal/queue"
)

const prefLastPlaylistID = "last_playlist_id"

// App wires UI, store, queue and player.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	store   *playlist.Store
	queue   *queue.Queue
	player  *audio.Player

	playlists            []model.Playlist
	allTracks            []model.Track
	tracks               []model.Track
	selectedPL           int64
	selectedTrack        int
	seeking              bool
	progressUpdating     bool
	toast                *widget.PopUp
	toastToken           uint64
	playlistList         *widget.List
	trackList            *widget.List
	nowTitle             *MarqueeTitle
	posLabel             *widget.Label
	durLabel             *widget.Label
	progress             *widget.Slider
	volume               *widget.Slider
	muteBtn              *widget.Button
	muted                bool
	volumeBeforeMute     float64
	playBtn              *PlayButton
	trackListHead        *widget.Label
	trackSearch          *searchEntry
	spectrum             *SpectrumViz
	playlistPanel        fyne.CanvasObject
	playlistsBtn         *widget.Button
	playlistPanelVisible bool
}

// NewApp constructs the application UI.
func NewApp(fa fyne.App, store *playlist.Store, player *audio.Player) *App {
	win := fa.NewWindow("Robsong")
	win.SetIcon(assets.Logo)
	a := &App{
		fyneApp:       fa,
		win:           win,
		store:         store,
		queue:         queue.New(),
		player:        player,
		selectedTrack: -1,
	}
	player.SetOnEnded(func() {
		fyne.Do(func() { a.onTrackEnded() })
	})
	a.build()
	a.reloadPlaylists()
	if len(a.playlists) > 0 {
		want := int64(a.fyneApp.Preferences().IntWithFallback(prefLastPlaylistID, int(a.playlists[0].ID)))
		idx := 0
		for i, p := range a.playlists {
			if p.ID == want {
				idx = i
				break
			}
		}
		a.selectPlaylist(a.playlists[idx].ID)
		a.playlistList.Select(idx)
	}
	a.spectrum.Start()
	a.nowTitle.Start()
	a.startProgressTicker()
	return a
}

// ShowAndRun shows the window and runs the event loop.
func (a *App) ShowAndRun() {
	a.win.Resize(fyne.NewSize(1024, 700))
	a.win.CenterOnScreen()
	a.win.ShowAndRun()
}

func (a *App) build() {
	a.trackListHead = widget.NewLabel("Tracks")
	a.trackListHead.TextStyle = fyne.TextStyle{Bold: true}

	a.trackSearch = newSearchEntry()
	a.trackSearch.SetOnChanged(func(string) {
		a.selectedTrack = -1
		a.applyTrackFilter()
		a.trackList.Refresh()
		a.trackList.UnselectAll()
	})
	searchIcon := widget.NewIcon(theme.SearchIcon())
	searchIconBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(22, 22)), searchIcon)
	underline := canvas.NewRectangle(color.NRGBA{R: 0xd0, G: 0xd5, B: 0xdc, A: 0xff})
	underline.SetMinSize(fyne.NewSize(0, 1))
	searchField := container.NewBorder(nil, underline, nil, nil, a.trackSearch.field())
	// ~50% larger than 160×28; height must stay above Entry MinSize or text clips.
	searchSized := container.New(layout.NewGridWrapLayout(fyne.NewSize(240, 42)), searchField)
	searchRow := container.NewHBox(container.NewCenter(searchIconBox), searchSized)
	trackHead := container.NewBorder(nil, nil, a.trackListHead, searchRow)

	a.playlistList = widget.NewList(
		func() int { return len(a.playlists) },
		func() fyne.CanvasObject { return newPlaylistRow(a) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row := obj.(*playlistRow)
			if id < 0 || id >= len(a.playlists) {
				return
			}
			row.Update(int(id), a.playlists[id].Name)
		},
	)
	a.playlistList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(a.playlists) {
			a.selectPlaylist(a.playlists[id].ID)
		}
	}

	a.trackList = widget.NewList(
		func() int { return len(a.tracks) },
		func() fyne.CanvasObject {
			return newTrackRow(a)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row := obj.(*trackRow)
			if id < 0 || id >= len(a.tracks) {
				return
			}
			t := a.tracks[id]
			playing := false
			if cur, ok := a.queue.Current(); ok && cur.Path == t.Path {
				playing = true
			}
			row.Update(int(id), t, playing)
		},
	)
	a.trackList.OnSelected = func(id widget.ListItemID) {
		a.selectedTrack = int(id)
	}

	// Narrow icon rail (web/Cursor-style activity bar).
	a.playlistsBtn = iconBtn(theme.ListIcon(), a.togglePlaylists)
	importFiles := iconBtn(theme.FileIcon(), a.importFiles)
	importFolder := iconBtn(theme.FolderOpenIcon(), a.importFolder)
	rail := container.NewVBox(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(44, 40)), a.playlistsBtn),
		container.New(layout.NewGridWrapLayout(fyne.NewSize(44, 40)), importFiles),
		container.New(layout.NewGridWrapLayout(fyne.NewSize(44, 40)), importFolder),
	)
	railBox := container.NewBorder(nil, nil, nil, widget.NewSeparator(), rail)

	newPL := iconBtn(theme.ContentAddIcon(), a.promptNewPlaylist)
	renamePL := iconBtn(theme.DocumentCreateIcon(), a.promptRenamePlaylist)
	delPL := iconBtn(theme.DeleteIcon(), a.deleteSelectedPlaylist)
	panelHead := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("Playlists", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(newPL, renamePL, delPL),
	)
	a.playlistPanel = container.NewBorder(nil, nil, nil, widget.NewSeparator(),
		container.NewBorder(panelHead, nil, nil, nil, a.playlistList),
	)
	a.playlistPanelVisible = false
	a.playlistPanel.Hide()
	a.playlistsBtn.Importance = widget.LowImportance

	center := container.NewBorder(trackHead, nil, nil, nil, a.trackList)
	left := container.NewHBox(railBox, a.playlistPanel)
	body := container.NewBorder(nil, nil, left, nil, center)

	a.win.SetContent(container.NewBorder(
		nil,
		a.buildPlayerBar(),
		nil, nil, body,
	))
	a.win.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyDelete {
			a.removeTrackFromPlaylist(a.selectedTrack)
		}
	})
	a.win.SetCloseIntercept(func() {
		a.spectrum.Stop()
		a.nowTitle.Stop()
		a.player.Close()
		_ = a.store.Close()
		a.win.Close()
	})
}

func (a *App) togglePlaylists() {
	a.playlistPanelVisible = !a.playlistPanelVisible
	if a.playlistPanelVisible {
		a.playlistPanel.Show()
		a.playlistsBtn.Importance = widget.HighImportance
	} else {
		a.playlistPanel.Hide()
		a.playlistsBtn.Importance = widget.LowImportance
	}
	a.playlistsBtn.Refresh()
	a.playlistPanel.Refresh()
	a.win.Content().Refresh()
}

func iconBtn(icon fyne.Resource, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon("", icon, tapped)
	b.Importance = widget.LowImportance
	return b
}
