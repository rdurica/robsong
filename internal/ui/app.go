package ui

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/assets"
	"github.com/rdurica/robsong/internal/audio"
	"github.com/rdurica/robsong/internal/library"
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

	playlists          []model.Playlist
	allTracks          []model.Track
	tracks             []model.Track
	selectedPL         int64
	selectedTrack      int
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
		func() fyne.CanvasObject { return widget.NewLabel("playlist") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(a.playlists) {
				return
			}
			obj.(*widget.Label).SetText(a.playlists[id].Name)
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
	a.volume.OnChanged = func(v float64) { a.player.SetVolume(v) }

	prev := iconBtn(theme.MediaSkipPreviousIcon(), a.prev)
	a.playBtn = NewPlayButton(theme.MediaPlayIcon(), a.togglePlay)
	next := iconBtn(theme.MediaSkipNextIcon(), a.next)

	controls := container.NewHBox(prev, a.playBtn, next)

	timeW := float32(44)
	posBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(timeW, 24)), a.posLabel)
	durBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(timeW, 24)), a.durLabel)
	seek := container.NewBorder(nil, nil, posBox, durBox, a.progress)

	volBox := container.NewCenter(container.NewHBox(
		widget.NewIcon(theme.VolumeUpIcon()),
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, playBtnDiameter)), a.volume),
	))
	top := container.New(&playerTopLayout{}, a.nowTitle, controls, volBox)

	return container.NewVBox(
		a.spectrum,
		top,
		seek,
	)
}

func iconBtn(icon fyne.Resource, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon("", icon, tapped)
	b.Importance = widget.LowImportance
	return b
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

func (a *App) trackFilterActive() bool {
	return a.trackSearch != nil && strings.TrimSpace(a.trackSearch.Text) != ""
}

// applyTrackFilter sets a.tracks from allTracks using the search entry (case-insensitive title match).
func (a *App) applyTrackFilter() {
	q := ""
	if a.trackSearch != nil {
		q = strings.ToLower(strings.TrimSpace(a.trackSearch.Text))
	}
	if q == "" {
		a.tracks = a.allTracks
	} else {
		filtered := make([]model.Track, 0, len(a.allTracks))
		for _, t := range a.allTracks {
			if strings.Contains(strings.ToLower(t.DisplayTitle()), q) {
				filtered = append(filtered, t)
			}
		}
		a.tracks = filtered
	}
	a.trackListHead.SetText(fmt.Sprintf("%s (%d)", a.currentPlaylistName(), len(a.tracks)))
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

	// Keep playback queue aligned with new list order if we are playing this playlist.
	if cur, ok := a.queue.Current(); ok {
		for i, t := range a.allTracks {
			if t.Path == cur.Path {
				a.queue.Set(a.allTracks[i:])
				break
			}
		}
	}
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
	} else if cur, ok := a.queue.Current(); ok {
		for i, t := range a.allTracks {
			if t.Path == cur.Path {
				a.queue.Set(a.allTracks[i:])
				break
			}
		}
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
		a.selectPlaylist(p.ID)
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
	d := dialog.NewConfirm("Delete playlist", "Delete \""+pl.Name+"\"?", func(ok bool) {
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
	d.Show()
	padTopModal(a.win.Canvas())
}

// showTrackContextMenu shows right-click actions for a track.
func (a *App) showTrackContextMenu(index int, t model.Track, pos fyne.Position) {
	actions := []ctxMenuAction{
		{label: "Play", action: func() { a.playFrom(index) }},
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
	actions = append(actions, ctxMenuAction{
		label:  "Delete",
		action: func() { a.removeTrackFromPlaylist(index) },
	})
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

func (a *App) importFiles() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		uri := reader.URI()
		_ = reader.Close()
		a.importPaths([]string{uri.Path()})
	}, a.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".mp3", ".flac", ".wav", ".ogg"}))
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

// padTopModal insets the topmost modal popup so footer buttons aren't flush to the edges.
func padTopModal(c fyne.Canvas) {
	const edge = float32(16)
	top := c.Overlays().Top()
	if top == nil {
		return
	}
	rv := reflect.ValueOf(top)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	field := rv.FieldByName("Content")
	if !field.IsValid() || !field.CanInterface() {
		return
	}
	pop, ok := field.Interface().(*widget.PopUp)
	if !ok || pop.Content == nil {
		return
	}
	pop.Content = container.New(
		layout.NewCustomPaddedLayout(edge, edge, edge, edge),
		pop.Content,
	)
	pop.Refresh()
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

func (a *App) setStatus(msg string) {
	a.hideToast()
	if msg == "" {
		return
	}

	label := canvas.NewText(msg, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	label.TextSize = theme.TextSize()
	icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.InfoIcon(), theme.ColorNameForegroundOnPrimary))
	icon.SetMinSize(fyne.NewSquareSize(theme.IconInlineSize()))
	icon.FillMode = canvas.ImageFillContain
	row := container.NewHBox(container.NewCenter(icon), container.NewCenter(label))
	bg := canvas.NewRectangle(color.NRGBA{R: 0x1c, G: 0x22, B: 0x2c, A: 0xf2})
	bg.CornerRadius = 6
	content := container.NewStack(bg, container.NewPadded(row))

	pop := widget.NewPopUp(content, a.win.Canvas())
	a.toast = pop
	a.toastToken++
	token := a.toastToken

	size := content.MinSize()
	winSize := a.win.Canvas().Size()
	const pad float32 = 16
	x := winSize.Width - size.Width - pad
	if x < pad {
		x = pad
	}
	pop.ShowAtPosition(fyne.NewPos(x, pad))

	time.AfterFunc(3*time.Second, func() {
		fyne.Do(func() {
			if a.toastToken != token {
				return
			}
			a.hideToast()
		})
	})
}

func (a *App) hideToast() {
	a.toastToken++
	if a.toast != nil {
		a.toast.Hide()
		a.toast = nil
	}
}

func formatMs(ms int64) string {
	if ms <= 0 {
		return "0:00"
	}
	return formatDur(time.Duration(ms) * time.Millisecond)
}

func formatDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

// trackRow is a dense list row with double-click play and drag-to-reorder.
type trackRow struct {
	widget.BaseWidget
	app       *App
	index     int
	line      *widget.Label
	duration  *widget.Label
	dragAccum float32
}

func newTrackRow(a *App) *trackRow {
	r := &trackRow{
		app:      a,
		line:     widget.NewLabel("title"),
		duration: widget.NewLabel("0:00"),
	}
	r.line.Truncation = fyne.TextTruncateEllipsis
	r.duration.Importance = widget.LowImportance
	r.ExtendBaseWidget(r)
	return r
}

func (r *trackRow) Update(index int, t model.Track, playing bool) {
	r.index = index
	artist := t.DisplayArtist()
	title := t.DisplayTitle()
	if artist != "" {
		r.line.SetText(title + "  —  " + artist)
	} else {
		r.line.SetText(title)
	}
	r.line.TextStyle = fyne.TextStyle{Bold: playing}
	r.line.Refresh()
	if t.DurationMs > 0 {
		r.duration.SetText(formatMs(t.DurationMs))
	} else {
		r.duration.SetText("")
	}
}

func (r *trackRow) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewBorder(nil, nil, nil, r.duration, r.line)
	// Spacers vertically center the labels within the list row height.
	return widget.NewSimpleRenderer(container.NewVBox(
		layout.NewSpacer(), content, layout.NewSpacer(),
	))
}

func (r *trackRow) MinSize() fyne.Size {
	return fyne.NewSize(120, 28)
}

func (r *trackRow) Tapped(*fyne.PointEvent) {
	r.app.selectedTrack = r.index
	r.app.trackList.Select(r.index)
}

func (r *trackRow) DoubleTapped(*fyne.PointEvent) {
	r.app.playFrom(r.index)
}

func (r *trackRow) TappedSecondary(e *fyne.PointEvent) {
	r.app.selectedTrack = r.index
	r.app.trackList.Select(r.index)
	if r.index < 0 || r.index >= len(r.app.tracks) {
		return
	}
	r.app.showTrackContextMenu(r.index, r.app.tracks[r.index], e.AbsolutePosition)
}

func (r *trackRow) Dragged(e *fyne.DragEvent) {
	r.dragAccum += e.Dragged.DY
	rowH := r.Size().Height
	if rowH < 1 {
		rowH = 24
	}
	for r.dragAccum >= rowH {
		if r.index+1 >= len(r.app.tracks) {
			r.dragAccum = 0
			break
		}
		from := r.index
		r.app.moveTrack(from, from+1)
		r.index = from + 1
		r.dragAccum -= rowH
	}
	for r.dragAccum <= -rowH {
		if r.index-1 < 0 {
			r.dragAccum = 0
			break
		}
		from := r.index
		r.app.moveTrack(from, from-1)
		r.index = from - 1
		r.dragAccum += rowH
	}
}

func (r *trackRow) DragEnd() {
	r.dragAccum = 0
}

var _ fyne.Tappable = (*trackRow)(nil)
var _ fyne.DoubleTappable = (*trackRow)(nil)
var _ fyne.SecondaryTappable = (*trackRow)(nil)
var _ fyne.Draggable = (*trackRow)(nil)
