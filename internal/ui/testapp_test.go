package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/audio"
	"github.com/rdurica/robsong/internal/model"
	"github.com/rdurica/robsong/internal/playlist"
	"github.com/rdurica/robsong/internal/queue"
)

// fakePlayer is a playerBackend that records Play calls and fails per path.
type fakePlayer struct {
	mu      sync.Mutex
	played  []string
	errs    map[string]error
	stops   int
	closes  int
	onEnded func()
	vol     float64
	volSet  bool
}

func newFakePlayer() *fakePlayer { return &fakePlayer{errs: map[string]error{}} }

func (p *fakePlayer) Play(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.played = append(p.played, path)
	return p.errs[path]
}
func (p *fakePlayer) Stop()             { p.mu.Lock(); p.stops++; p.mu.Unlock() }
func (p *fakePlayer) TogglePause() bool { return false }
func (p *fakePlayer) IsPlaying() bool   { return false }
func (p *fakePlayer) SetVolume(v float64) {
	p.mu.Lock()
	p.vol, p.volSet = v, true
	p.mu.Unlock()
}
func (p *fakePlayer) Volume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.volSet {
		return p.vol
	}
	return 1
}
func (p *fakePlayer) Position() time.Duration          { return 0 }
func (p *fakePlayer) Duration() time.Duration          { return 0 }
func (p *fakePlayer) Seek(time.Duration) error         { return nil }
func (p *fakePlayer) Snapshot() audio.Snapshot         { return audio.Snapshot{} }
func (p *fakePlayer) Spectrum(dst []float64) []float64 { return dst }
func (p *fakePlayer) Close()                           { p.mu.Lock(); p.closes++; p.mu.Unlock() }
func (p *fakePlayer) SetOnEnded(fn func())             { p.onEnded = fn }
func (p *fakePlayer) playedPaths() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.played...)
}
func (p *fakePlayer) stopCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stops
}

// notExist is the error a real player returns for a vanished file.
func notExist(path string) error {
	return &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
}

type testApp struct {
	*App
	player *fakePlayer
	dir    string
}

// newTestApp builds a minimal App (no NewApp) with a fake player and a real temp store.
func newTestApp(t *testing.T) *testApp {
	t.Helper()
	fa := test.NewTempApp(t)
	dir := t.TempDir()
	store, err := playlist.OpenPath(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatalf("OpenPath: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	fp := newFakePlayer()
	a := &App{
		fyneApp:       fa,
		win:           test.NewTempWindow(t, widget.NewLabel("x")),
		store:         store,
		queue:         queue.New(),
		player:        fp,
		selectedTrack: -1,
		cover:         newCoverArt(newFakeCovers().load),
		nowTitle:      NewMarqueeTitle(""),
		spectrum:      NewSpectrumViz(),
		trackSearch:   newSearchEntry(),
		trackListHead: widget.NewLabel("Tracks"),
	}
	a.playlistList = widget.NewList(
		func() int { return len(a.playlists) },
		func() fyne.CanvasObject { return newPlaylistRow(a) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(a.playlists) {
				return
			}
			obj.(*playlistRow).Update(int(id), a.playlists[id].Name)
		},
	)
	a.trackList = widget.NewList(
		func() int { return len(a.tracks) },
		func() fyne.CanvasObject { return newTrackRow(a) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(a.tracks) {
				return
			}
			tr := a.tracks[id]
			playing := false
			if cur, ok := a.queue.Current(); ok && cur.Path == tr.Path {
				playing = true
			}
			obj.(*trackRow).Update(int(id), tr, playing, a.isMissing(tr.Path))
		},
	)
	a.reloadPlaylists()
	return &testApp{App: a, player: fp, dir: dir}
}

// existing creates a real file and returns a track for it.
func (ta *testApp) existing(t *testing.T, name string) model.Track {
	t.Helper()
	p := filepath.Join(ta.dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return model.Track{Path: p, Title: name, Artist: "Art " + name}
}

// ghost returns a track whose file does not exist.
func (ta *testApp) ghost(name string) model.Track {
	return model.Track{Path: filepath.Join(ta.dir, "gone", name), Title: name, Artist: "Art " + name}
}

func (ta *testApp) libraryID(t *testing.T) int64 {
	t.Helper()
	id, err := ta.store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// fill adds the tracks to playlist id and selects it.
func (ta *testApp) fill(t *testing.T, id int64, tracks ...model.Track) {
	t.Helper()
	if err := ta.store.AddTracks(id, tracks); err != nil {
		t.Fatalf("AddTracks: %v", err)
	}
	ta.selectPlaylist(id)
}

func (ta *testApp) newPlaylist(t *testing.T, name string) model.Playlist {
	t.Helper()
	pl, err := ta.store.CreatePlaylist(name)
	if err != nil {
		t.Fatal(err)
	}
	ta.reloadPlaylists()
	return pl
}

func storePaths(t *testing.T, a *App, id int64) []string {
	t.Helper()
	tracks, err := a.store.ListTracks(id)
	if err != nil {
		t.Fatal(err)
	}
	return pathsOf(tracks)
}

func pathsOf(tracks []model.Track) []string {
	out := make([]string, 0, len(tracks))
	for _, tr := range tracks {
		out = append(out, tr.Path)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// collectObjects walks obj and all descendants.
func collectObjects(t *testing.T, obj fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	t.Helper()
	visit(obj)
	switch o := obj.(type) {
	case *fyne.Container:
		for _, c := range o.Objects {
			collectObjects(t, c, visit)
		}
	case fyne.Widget:
		for _, c := range test.TempWidgetRenderer(t, o).Objects() {
			collectObjects(t, c, visit)
		}
	}
}

// overlayTexts returns all label/button/text strings shown in the window overlays.
func overlayTexts(t *testing.T, a *App) []string {
	t.Helper()
	var out []string
	for _, ov := range a.win.Canvas().Overlays().List() {
		collectObjects(t, ov, func(o fyne.CanvasObject) {
			switch w := o.(type) {
			case *widget.Label:
				out = append(out, w.Text)
			case *widget.Button:
				out = append(out, w.Text)
			case *canvas.Text:
				out = append(out, w.Text)
			case *widget.RichText:
				out = append(out, w.String())
			}
		})
	}
	return out
}

func overlayContains(t *testing.T, a *App, sub string) bool {
	t.Helper()
	for _, s := range overlayTexts(t, a) {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// overlayButton finds a button with the given text in any overlay.
func overlayButton(t *testing.T, a *App, text string) *widget.Button {
	t.Helper()
	var found *widget.Button
	for _, ov := range a.win.Canvas().Overlays().List() {
		collectObjects(t, ov, func(o fyne.CanvasObject) {
			if b, ok := o.(*widget.Button); ok && b.Text == text && found == nil {
				found = b
			}
		})
	}
	return found
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (p *fakePlayer) closeCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closes
}
