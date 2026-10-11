package ui

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/model"
	"github.com/rdurica/robsong/internal/playlist"
)

// shellEnv wires a Shell to a fake player, a lazily opened real store
// (counting opens) and a fake path resolver.
type shellEnv struct {
	*Shell
	t       *testing.T
	fp      *fakePlayer
	fa      fyne.App
	dir     string
	opens   int
	openErr error
	store   *playlist.Store
	known   map[string]model.Track
}

func newShellEnv(t *testing.T) *shellEnv {
	t.Helper()
	e := &shellEnv{
		t:     t,
		fp:    newFakePlayer(),
		fa:    test.NewTempApp(t),
		dir:   t.TempDir(),
		known: map[string]model.Track{},
	}
	openStore := func() (*playlist.Store, error) {
		e.opens++
		if e.openErr != nil {
			return nil, e.openErr
		}
		if e.store == nil {
			s, err := playlist.OpenPath(filepath.Join(e.dir, "library.db"))
			if err != nil {
				return nil, err
			}
			e.store = s
			t.Cleanup(func() { _ = s.Close() })
		}
		return e.store, nil
	}
	readTracks := func(paths []string) []model.Track {
		var out []model.Track
		for _, p := range paths {
			if tr, ok := e.known[p]; ok {
				out = append(out, tr)
			}
		}
		return out
	}
	e.Shell = newShell(e.fa, e.fp, openStore, readTracks)
	return e
}

// track creates a real file, registers it with readTracks and returns it.
func (e *shellEnv) track(name string) model.Track {
	e.t.Helper()
	p := filepath.Join(e.dir, name+".mp3")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		e.t.Fatal(err)
	}
	tr := model.Track{Path: p, Title: "Title " + name, Artist: "Art " + name}
	e.known[p] = tr
	return tr
}

func (e *shellEnv) libraryPaths() []string {
	e.t.Helper()
	if e.store == nil {
		return nil
	}
	id, err := e.store.LibraryID()
	if err != nil {
		e.t.Fatal(err)
	}
	tracks, err := e.store.ListTracks(id)
	if err != nil {
		e.t.Fatal(err)
	}
	return pathsOf(tracks)
}

func winButtons(t *testing.T, win fyne.Window) []*widget.Button {
	t.Helper()
	var out []*widget.Button
	collectObjects(t, win.Content(), func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok {
			out = append(out, b)
		}
	})
	return out
}

func winButton(t *testing.T, win fyne.Window, text string) *widget.Button {
	t.Helper()
	for _, b := range winButtons(t, win) {
		if b.Text == text {
			return b
		}
	}
	return nil
}

// winSliderByStep returns the slider with the given step (volume 0.01, progress 0.001).
func winSliderByStep(t *testing.T, win fyne.Window, step float64) *widget.Slider {
	t.Helper()
	var found *widget.Slider
	collectObjects(t, win.Content(), func(o fyne.CanvasObject) {
		if s, ok := o.(*widget.Slider); ok && s.Step == step && found == nil {
			found = s
		}
	})
	return found
}

func toastShown(t *testing.T, win fyne.Window, sub string) bool {
	t.Helper()
	found := false
	for _, ov := range win.Canvas().Overlays().List() {
		collectObjects(t, ov, func(o fyne.CanvasObject) {
			var s string
			switch w := o.(type) {
			case *widget.Label:
				s = w.Text
			case *widget.Button:
				s = w.Text
			case *canvas.Text:
				s = w.Text
			case *widget.RichText:
				s = w.String()
			}
			if strings.Contains(s, sub) {
				found = true
			}
		})
	}
	return found
}

func shellEventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func requireMini(t *testing.T, e *shellEnv) {
	t.Helper()
	if e.mini == nil || e.full != nil {
		t.Fatalf("want mini mode, got mini=%v full=%v", e.mini != nil, e.full != nil)
	}
}

func requireFull(t *testing.T, e *shellEnv) {
	t.Helper()
	if e.full == nil || e.mini != nil {
		t.Fatalf("want full mode, got mini=%v full=%v", e.mini != nil, e.full != nil)
	}
}

func requirePlayNow(t *testing.T, e *shellEnv, want ...model.Track) {
	t.Helper()
	requireFull(t, e)
	if got := pathsOf(e.full.playNow); !equalStrings(got, pathsOf(want)) {
		t.Fatalf("Play Now = %v, want %v", got, pathsOf(want))
	}
	if e.full.selectedPL != playNowID {
		t.Fatalf("selectedPL = %d, want Play Now (%d)", e.full.selectedPL, playNowID)
	}
	if len(e.full.playlists) == 0 || e.full.playlists[0].ID != playNowID {
		t.Fatalf("Play Now is not the first playlist: %+v", e.full.playlists)
	}
}

func requirePlayed(t *testing.T, e *shellEnv, want ...string) {
	t.Helper()
	if got := e.fp.playedPaths(); !equalStrings(got, want) {
		t.Fatalf("played = %v, want %v", got, want)
	}
}

// --- showMini ---

func TestShowMiniPlaysTrackWithoutOpeningStore(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")

	e.showMini(a)

	requireMini(t, e)
	requirePlayed(t, e, a.Path)
	if e.opens != 0 {
		t.Fatalf("openStore called %d times at mini start, want 0", e.opens)
	}
	if got := e.mini.win.Title(); got != a.Title {
		t.Fatalf("window title = %q, want %q", got, a.Title)
	}
}

func TestShowMiniBarHasLibraryAndExpandButNoPrevNext(t *testing.T) {
	e := newShellEnv(t)
	e.showMini(e.track("a"))

	if winButton(t, e.mini.win, "+ Library") == nil {
		t.Error("mini bar has no '+ Library' button")
	}
	if winButton(t, e.mini.win, "Expand") == nil {
		t.Error("mini bar has no 'Expand' button")
	}
	if e.mini.addBtn == nil || e.mini.expandBtn == nil {
		t.Error("addBtn/expandBtn not set")
	}
	for _, b := range winButtons(t, e.mini.win) {
		if b.Icon == nil {
			continue
		}
		n := b.Icon.Name()
		if n == theme.MediaSkipPreviousIcon().Name() || n == theme.MediaSkipNextIcon().Name() {
			t.Errorf("mini bar contains prev/next button (icon %s)", n)
		}
	}
}

// --- handleOpen in mini ---

func TestHandleOpenInMiniOnePathReplacesTrack(t *testing.T) {
	e := newShellEnv(t)
	a, b := e.track("a"), e.track("b")
	e.showMini(a)

	e.handleOpen([]string{b.Path})

	requireMini(t, e)
	if e.mini.track.Path != b.Path {
		t.Fatalf("mini track = %q, want %q", e.mini.track.Path, b.Path)
	}
	requirePlayed(t, e, a.Path, b.Path)
	if got := e.mini.win.Title(); got != b.Title {
		t.Fatalf("title = %q, want %q", got, b.Title)
	}
	if e.opens != 0 {
		t.Fatalf("openStore called %d times, want 0", e.opens)
	}
}

func TestHandleOpenInMiniEmptyPathsExpands(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.showMini(a)

	e.handleOpen([]string{})

	requirePlayNow(t, e, a)
	requirePlayed(t, e, a.Path)
}

func TestHandleOpenInMiniManyPathsExpandsWithNewPlayNow(t *testing.T) {
	e := newShellEnv(t)
	a, b, c := e.track("a"), e.track("b"), e.track("c")
	e.showMini(a)

	e.handleOpen([]string{b.Path, c.Path})

	requirePlayNow(t, e, b, c)
	requirePlayed(t, e, a.Path, b.Path)
}

func TestExpandButtonOpensFullWithCurrentTrackWithoutRestart(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.showMini(a)

	test.Tap(e.mini.expandBtn)

	requirePlayNow(t, e, a)
	requirePlayed(t, e, a.Path)
}

// --- showFull / handleOpen in full ---

func TestShowFullWithTwoTracksFillsPlayNowAndPlaysFirst(t *testing.T) {
	e := newShellEnv(t)
	a, b := e.track("a"), e.track("b")

	e.showFull([]model.Track{a, b})

	requirePlayNow(t, e, a, b)
	requirePlayed(t, e, a.Path)
}

func TestShowFullWithoutTracksHasNoPlayNow(t *testing.T) {
	e := newShellEnv(t)

	e.showFull(nil)

	requireFull(t, e)
	if len(e.full.playNow) != 0 {
		t.Fatalf("Play Now = %v, want empty", pathsOf(e.full.playNow))
	}
	for _, p := range e.full.playlists {
		if p.ID == playNowID {
			t.Fatal("Play Now listed without tracks")
		}
	}
	requirePlayed(t, e)
}

func TestHandleOpenInFullOnePathReplacesPlayNow(t *testing.T) {
	e := newShellEnv(t)
	a, b, c := e.track("a"), e.track("b"), e.track("c")
	e.showFull([]model.Track{a, b})

	e.handleOpen([]string{c.Path})

	requirePlayNow(t, e, c)
	requirePlayed(t, e, a.Path, c.Path)
}

func TestHandleOpenInFullEmptyKeepsPlayNow(t *testing.T) {
	e := newShellEnv(t)
	a, b := e.track("a"), e.track("b")
	e.showFull([]model.Track{a, b})

	e.handleOpen([]string{})

	requirePlayNow(t, e, a, b)
	requirePlayed(t, e, a.Path)
}

func TestHandleOpenInFullReplacesPlayNowWithNewSet(t *testing.T) {
	e := newShellEnv(t)
	a, b, c, d := e.track("a"), e.track("b"), e.track("c"), e.track("d")
	e.showFull([]model.Track{a, b})

	e.handleOpen([]string{c.Path, d.Path})

	requirePlayNow(t, e, c, d)
	requirePlayed(t, e, a.Path, c.Path)
}

// --- unsupported input ---

func TestHandleOpenUnsupportedPathsToastsInMini(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.showMini(a)

	e.handleOpen([]string{filepath.Join(e.dir, "notes.txt")})

	requireMini(t, e)
	if !toastShown(t, e.mini.win, "No supported audio file") {
		t.Fatal("no 'No supported audio file' toast in mini window")
	}
	if e.mini.track.Path != a.Path {
		t.Fatal("mini track changed on unsupported input")
	}
	requirePlayed(t, e, a.Path)
}

func TestHandleOpenUnsupportedPathsToastsInFull(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.showFull([]model.Track{a, e.track("b")})

	e.handleOpen([]string{filepath.Join(e.dir, "notes.txt")})

	requireFull(t, e)
	if !toastShown(t, e.full.win, "No supported audio file") {
		t.Fatal("no 'No supported audio file' toast in full window")
	}
	if len(e.full.playNow) != 2 {
		t.Fatalf("Play Now changed: %v", pathsOf(e.full.playNow))
	}
}

// --- + Library ---

func TestMiniAddToLibraryAddsAndDisablesButton(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.showMini(a)

	test.Tap(e.mini.addBtn)

	if e.opens != 1 {
		t.Fatalf("openStore calls = %d, want 1", e.opens)
	}
	if got := e.libraryPaths(); !equalStrings(got, []string{a.Path}) {
		t.Fatalf("Library = %v, want [%s]", got, a.Path)
	}
	if !toastShown(t, e.mini.win, "Added to library") {
		t.Error("no 'Added to library' toast")
	}
	if !e.mini.addBtn.Disabled() {
		t.Error("+ Library button should be disabled after adding")
	}
}

func TestMiniAddToLibraryWhenAlreadyInLibraryWritesNothing(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	st, err := e.openStoreForSetup()
	if err != nil {
		t.Fatal(err)
	}
	libID, _ := st.LibraryID()
	if err := st.AddTracks(libID, []model.Track{a}); err != nil {
		t.Fatal(err)
	}
	e.opens = 0
	e.showMini(a)

	test.Tap(e.mini.addBtn)

	if got := e.libraryPaths(); !equalStrings(got, []string{a.Path}) {
		t.Fatalf("Library = %v, want a single entry %s", got, a.Path)
	}
	if !e.mini.addBtn.Disabled() {
		t.Error("+ Library button should be disabled when already in Library")
	}
}

// openStoreForSetup opens the temp store directly (not through the Shell) to
// pre-seed data; the Shell's openStore reuses the same instance.
func (e *shellEnv) openStoreForSetup() (*playlist.Store, error) {
	if e.store == nil {
		s, err := playlist.OpenPath(filepath.Join(e.dir, "library.db"))
		if err != nil {
			return nil, err
		}
		e.store = s
		e.t.Cleanup(func() { _ = s.Close() })
	}
	return e.store, nil
}

func TestMiniAddToLibraryStoreErrorShowsToastAndKeepsPlaying(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.openErr = errors.New("db is locked")
	e.showMini(a)

	test.Tap(e.mini.addBtn)

	if !toastShown(t, e.mini.win, "db is locked") {
		t.Error("error text not shown in toast")
	}
	if e.mini.addBtn.Disabled() {
		t.Error("+ Library button must stay enabled after an error")
	}
	if n := e.fp.stopCount(); n != 0 {
		t.Errorf("player stopped %d times, want 0", n)
	}
	requireMini(t, e)
}

// --- track end, volume, store reuse ---

func TestMiniTrackEndStopsPlayerAndKeepsWindow(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.showMini(a)
	if e.fp.onEnded == nil {
		t.Fatal("shell did not register an onEnded callback")
	}

	e.fp.onEnded()

	shellEventually(t, "player stop after track end", func() bool { return e.fp.stopCount() >= 1 })
	requireMini(t, e)
	requirePlayed(t, e, a.Path)
}

func TestMiniVolumePersistsAndSurvivesExpand(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.showMini(a)
	vol := winSliderByStep(t, e.mini.win, 0.01)
	if vol == nil {
		t.Fatal("mini bar has no volume slider")
	}

	vol.SetValue(0.35)

	if got := e.fa.Preferences().Float("volume"); math.Abs(got-0.35) > 1e-9 {
		t.Fatalf("pref volume = %v, want 0.35", got)
	}
	e.expand()
	requireFull(t, e)
	if got := e.fa.Preferences().Float("volume"); math.Abs(got-0.35) > 1e-9 {
		t.Fatalf("pref volume after expand = %v, want 0.35", got)
	}
	if got := e.fp.Volume(); math.Abs(got-0.35) > 1e-9 {
		t.Fatalf("player volume after expand = %v, want 0.35", got)
	}
}

func TestExpandReusesStoreOpenedByAddToLibrary(t *testing.T) {
	e := newShellEnv(t)
	a := e.track("a")
	e.showMini(a)
	test.Tap(e.mini.addBtn)

	test.Tap(e.mini.expandBtn)

	requireFull(t, e)
	if e.opens != 1 {
		t.Fatalf("openStore calls = %d, want 1 (store reused by expand)", e.opens)
	}
}

func TestExpandOpensStoreOnceWhenNotYetOpened(t *testing.T) {
	e := newShellEnv(t)
	e.showMini(e.track("a"))

	e.expand()

	requireFull(t, e)
	if e.opens != 1 {
		t.Fatalf("openStore calls = %d, want 1", e.opens)
	}
}

// --- store open failure on full start ---

func TestShowFullReturnsErrorWhenStoreCannotOpen(t *testing.T) {
	e := newShellEnv(t)
	e.openErr = errors.New("db is locked")

	err := e.showFull([]model.Track{e.track("a")})

	if err == nil || !strings.Contains(err.Error(), "db is locked") {
		t.Fatalf("showFull error = %v, want one containing 'db is locked'", err)
	}
	if e.full != nil || e.mini != nil {
		t.Fatal("a window was created although the store failed to open")
	}
}

// --- closing the mini window ---

func TestClosingMiniClosesPlayerAndOpenedStore(t *testing.T) {
	e := newShellEnv(t)
	e.showMini(e.track("a"))
	test.Tap(e.mini.addBtn) // opens the store lazily
	if e.Shell.store == nil {
		t.Fatal("store not opened by + Library")
	}
	st := e.Shell.store

	e.mini.close() // the window's close-intercept handler

	if n := e.fp.closeCount(); n != 1 {
		t.Fatalf("player Close called %d times, want 1", n)
	}
	if _, err := st.LibraryID(); err == nil {
		t.Fatal("store still usable after closing the mini window")
	}
}

func TestClosingMiniWithoutStoreClosesPlayer(t *testing.T) {
	e := newShellEnv(t)
	e.showMini(e.track("a"))

	e.mini.close() // the window's close-intercept handler

	if n := e.fp.closeCount(); n != 1 {
		t.Fatalf("player Close called %d times, want 1", n)
	}
	if e.opens != 0 {
		t.Fatalf("store opened %d times on close", e.opens)
	}
}

// --- handleTracks seam (Open expands paths off the UI thread) ---

func TestHandleTracksNonEmptyPathsWithoutTracksToasts(t *testing.T) {
	e := newShellEnv(t)
	e.showMini(e.track("a"))

	e.handleTracks(true, nil)

	if !toastShown(t, e.mini.win, "No supported audio file") {
		t.Fatal("no 'No supported audio file' toast")
	}
}

func TestHandleTracksNoPathsInFullKeepsPlayNow(t *testing.T) {
	e := newShellEnv(t)
	e.showFull([]model.Track{e.track("a"), e.track("b")})

	e.handleTracks(false, nil)

	requireFull(t, e)
	if len(e.full.playNow) != 2 {
		t.Fatalf("Play Now changed: %v", pathsOf(e.full.playNow))
	}
}
