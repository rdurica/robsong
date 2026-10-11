package ui

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/assets"
	"github.com/rdurica/robsong/internal/audio"
	"github.com/rdurica/robsong/internal/library"
	"github.com/rdurica/robsong/internal/model"
	"github.com/rdurica/robsong/internal/playlist"
	"github.com/rdurica/robsong/internal/queue"
)

const miniWidth = 640

// Shell owns the single fyne.App, one player, a lazily opened store and the
// current window mode (mini player or full window).
type Shell struct {
	fa         fyne.App
	player     playerBackend
	openStore  func() (*playlist.Store, error)
	readTracks func(paths []string) []model.Track
	store      *playlist.Store
	animate    bool // start bar animations and tickers (off in tests: the test driver runs fyne.Do on the caller)

	mini *miniWindow // non-nil only in mini mode
	full *App        // non-nil only in full mode
}

// NewShell creates a Shell. openStore is called at most once successfully,
// the first time a store is needed.
func NewShell(fa fyne.App, player *audio.Player, openStore func() (*playlist.Store, error)) *Shell {
	s := newShell(fa, player, openStore, readAudioTracks)
	s.animate = true
	return s
}

func newShell(fa fyne.App, player playerBackend, openStore func() (*playlist.Store, error), readTracks func(paths []string) []model.Track) *Shell {
	s := &Shell{fa: fa, player: player, openStore: openStore, readTracks: readTracks}
	player.SetOnEnded(func() { fyne.Do(s.onTrackEnded) })
	return s
}

// readAudioTracks expands files and folders into supported tracks, skipping
// unreadable arguments and de-duplicating by absolute path.
func readAudioTracks(paths []string) []model.Track {
	var out []model.Track
	seen := map[string]struct{}{}
	for _, p := range paths {
		ts, _ := library.ImportPaths([]string{p})
		for _, t := range ts {
			if abs, err := filepath.Abs(t.Path); err == nil {
				t.Path = abs
			}
			if _, dup := seen[t.Path]; dup {
				continue
			}
			seen[t.Path] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}

// RunMini shows the mini player for t and runs the event loop.
func (s *Shell) RunMini(t model.Track) {
	s.showMini(t)
	s.fa.Run()
}

// RunFull shows the full window and runs the event loop; more than one track
// fills Play Now and starts playing.
func (s *Shell) RunFull(tracks []model.Track) {
	s.showFull(tracks)
	s.fa.Run()
}

// Open handles a launch request from another process. Safe from any goroutine.
func (s *Shell) Open(paths []string) {
	fyne.Do(func() { s.handleOpen(paths) })
}

func (s *Shell) getStore() (*playlist.Store, error) {
	if s.store != nil {
		return s.store, nil
	}
	st, err := s.openStore()
	if err != nil {
		return nil, err
	}
	s.store = st
	return st, nil
}

func (s *Shell) onTrackEnded() {
	switch {
	case s.mini != nil:
		s.player.Stop()
	case s.full != nil:
		s.full.onTrackEnded()
	}
}

func (s *Shell) showMini(t model.Track) {
	s.mini = newMiniWindow(s)
	s.mini.setTrack(t)
	s.mini.win.Show()
}

func (s *Shell) showFull(tracks []model.Track) {
	st, err := s.getStore()
	if err != nil {
		// Without a store the full window cannot work; keep whatever is shown.
		s.toast("Library unavailable: " + err.Error())
		return
	}
	a := newApp(s.fa, st, s.player)
	if s.animate {
		a.startBar()
	}
	s.full = a
	a.win.Resize(fyne.NewSize(1024, 700))
	a.win.CenterOnScreen()
	a.win.Show()
	if len(tracks) > 0 {
		a.setPlayNow(tracks)
	}
}

// toast shows msg in whichever window is current.
func (s *Shell) toast(msg string) {
	switch {
	case s.mini != nil:
		s.mini.host.setStatus(msg)
	case s.full != nil:
		s.full.setStatus(msg)
	}
}

func (s *Shell) handleOpen(paths []string) {
	tracks := s.readTracks(paths)
	if len(paths) > 0 && len(tracks) == 0 {
		s.toast("No supported audio file")
		return
	}
	switch {
	case s.mini != nil:
		switch len(tracks) {
		case 0:
			s.expandTo(nil)
		case 1:
			s.mini.setTrack(tracks[0])
			s.mini.win.RequestFocus()
		default:
			s.expandTo(tracks)
		}
	case s.full != nil:
		if len(tracks) > 0 {
			s.full.setPlayNow(tracks)
		}
		s.full.win.RequestFocus()
	}
}

func (s *Shell) expand() { s.expandTo(nil) }

// expandTo switches from mini to full around the same player. Without
// tracks, Play Now holds just the current track and playback continues
// untouched; with tracks, they replace it and start playing.
func (s *Shell) expandTo(tracks []model.Track) {
	m := s.mini
	if m == nil {
		return
	}
	st, err := s.getStore()
	if err != nil {
		m.host.setStatus("Cannot open library: " + err.Error())
		return
	}
	a := newApp(s.fa, st, s.player)
	if s.animate {
		a.startBar()
	}
	a.playNow = []model.Track{m.track}
	a.reloadPlaylists()
	a.selectPlaylist(playNowID)
	a.playlistList.Select(0)
	a.queue.Set(a.tracks)
	a.updateNowPlaying()
	a.trackList.Refresh()
	a.win.Resize(fyne.NewSize(1024, 700))
	a.win.CenterOnScreen()
	a.win.Show()

	s.mini, s.full = nil, a
	m.host.stopBar()
	m.win.Close()
	if len(tracks) > 0 {
		a.setPlayNow(tracks)
	}
}

// miniWindow is the compact single-track player. Its bar is the shared player
// bar built on a headless App (no lists, store or playlist queue).
type miniWindow struct {
	shell     *Shell
	host      *App
	win       fyne.Window
	track     model.Track
	addBtn    *widget.Button
	expandBtn *widget.Button
}

func newMiniWindow(s *Shell) *miniWindow {
	win := s.fa.NewWindow("Robsong")
	win.SetIcon(assets.Logo)
	host := &App{
		fyneApp:       s.fa,
		win:           win,
		queue:         queue.New(),
		player:        s.player,
		selectedTrack: -1,
		cover:         newCoverArt(library.ReadCover),
	}
	m := &miniWindow{shell: s, host: host, win: win}
	host.loadVolumePrefs()

	m.addBtn = widget.NewButton("+ Library", m.addToLibrary)
	m.expandBtn = widget.NewButton("Expand", s.expand)
	footer := container.NewHBox(layout.NewSpacer(), m.addBtn, m.expandBtn)
	bar := host.buildBar(barOptions{toggle: m.togglePlay, footer: footer})
	win.SetContent(bar)
	win.Resize(fyne.NewSize(miniWidth, bar.MinSize().Height))
	win.SetFixedSize(true)
	win.SetCloseIntercept(m.close)

	if s.animate {
		host.startBar()
	}
	return m
}

// setTrack shows t and starts playing it.
func (m *miniWindow) setTrack(t model.Track) {
	m.track = t
	title := t.Title
	if title == "" {
		title = filepath.Base(t.Path)
	}
	m.win.SetTitle(title)
	m.addBtn.Enable()
	m.host.queue.Set([]model.Track{t})
	m.host.updateNowPlaying()
	m.play()
}

func (m *miniWindow) play() {
	if err := m.shell.player.Play(m.track.Path); err != nil {
		m.host.setStatus("Play error: " + err.Error())
		return
	}
	m.host.spectrum.SetActive(true)
}

func (m *miniWindow) togglePlay() {
	if m.shell.player.IsPlaying() {
		m.shell.player.TogglePause()
		return
	}
	m.play()
}

func (m *miniWindow) addToLibrary() {
	st, err := m.shell.getStore()
	if err != nil {
		m.host.setStatus("Add to library failed: " + err.Error())
		return
	}
	libID, err := st.LibraryID()
	if err != nil {
		m.host.setStatus("Add to library failed: " + err.Error())
		return
	}
	containing, err := st.PlaylistsContaining(m.track.Path)
	if err != nil {
		m.host.setStatus("Add to library failed: " + err.Error())
		return
	}
	for _, p := range containing {
		if p.ID == libID {
			m.addBtn.Disable()
			return
		}
	}
	if err := st.AddTracks(libID, []model.Track{m.track}); err != nil {
		m.host.setStatus("Add to library failed: " + err.Error())
		return
	}
	m.addBtn.Disable()
	m.host.setStatus("Added to library")
}

// close ends the process: stops the player, releases the store and closes the window.
func (m *miniWindow) close() {
	m.host.stopBar()
	m.shell.player.Close()
	if m.shell.store != nil {
		_ = m.shell.store.Close()
	}
	m.win.Close()
}
