package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/rdurica/robsong/internal/model"
)

func (ta *testApp) pnHasPlayNow() bool {
	for _, p := range ta.playlists {
		if p.ID == playNowID {
			return true
		}
	}
	return false
}

// pnSnapshot captures every DB playlist (names + track paths); also asserts nothing exists under playNowID.
func pnSnapshot(t *testing.T, ta *testApp) map[int64][]string {
	t.Helper()
	pls, err := ta.store.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	out := map[int64][]string{}
	for _, p := range pls {
		out[p.ID] = storePaths(t, ta.App, p.ID)
	}
	if got := storePaths(t, ta.App, playNowID); len(got) != 0 {
		t.Fatalf("rows stored under playNowID: %v", got)
	}
	return out
}

func pnAssertSnapshot(t *testing.T, ta *testApp, want map[int64][]string) {
	t.Helper()
	got := pnSnapshot(t, ta)
	if len(got) != len(want) {
		t.Fatalf("playlist count changed: got %d want %d", len(got), len(want))
	}
	for id, w := range want {
		if !equalStrings(got[id], w) {
			t.Fatalf("playlist %d changed: got %v want %v", id, got[id], w)
		}
	}
}

func pnAssertPlayNow(t *testing.T, ta *testApp, want ...model.Track) {
	t.Helper()
	if !equalStrings(pathsOf(ta.playNow), pathsOf(want)) {
		t.Fatalf("playNow = %v, want %v", pathsOf(ta.playNow), pathsOf(want))
	}
	if !equalStrings(pathsOf(ta.allTracks), pathsOf(want)) {
		t.Fatalf("allTracks = %v, want %v", pathsOf(ta.allTracks), pathsOf(want))
	}
}

// pnMenuLabels returns labels of all context-menu rows currently in overlays.
func pnMenuLabels(t *testing.T, ta *testApp) []string {
	t.Helper()
	var out []string
	for _, ov := range ta.win.Canvas().Overlays().List() {
		collectObjects(t, ov, func(o fyne.CanvasObject) {
			if r, ok := o.(*ctxMenuRow); ok {
				out = append(out, r.label)
			}
		})
	}
	return out
}

func pnMenuRow(t *testing.T, ta *testApp, label string) *ctxMenuRow {
	t.Helper()
	var found *ctxMenuRow
	for _, ov := range ta.win.Canvas().Overlays().List() {
		collectObjects(t, ov, func(o fyne.CanvasObject) {
			if r, ok := o.(*ctxMenuRow); ok && r.label == label && found == nil {
				found = r
			}
		})
	}
	return found
}

func pnContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestPlayNowAbsentWhenEmpty(t *testing.T) {
	ta := newTestApp(t)
	if ta.pnHasPlayNow() {
		t.Fatal("Play Now listed while empty")
	}
}

func TestSetPlayNowListsFirstSelectsAndPlaysFirst(t *testing.T) {
	ta := newTestApp(t)
	a, b := ta.existing(t, "a.mp3"), ta.existing(t, "b.mp3")
	ta.newPlaylist(t, "Mine")

	ta.setPlayNow([]model.Track{a, b})

	if len(ta.playlists) == 0 || ta.playlists[0].ID != playNowID || ta.playlists[0].Name != playNowName {
		t.Fatalf("playlists[0] = %+v, want Play Now first", ta.playlists)
	}
	if ta.selectedPL != playNowID {
		t.Fatalf("selectedPL = %d, want %d", ta.selectedPL, playNowID)
	}
	pnAssertPlayNow(t, ta, a, b)
	if got := ta.player.playedPaths(); len(got) == 0 || got[0] != a.Path {
		t.Fatalf("played = %v, want first %s", got, a.Path)
	}
}

func TestSetPlayNowReplacesContent(t *testing.T) {
	ta := newTestApp(t)
	a, b := ta.existing(t, "a.mp3"), ta.existing(t, "b.mp3")
	c, d := ta.existing(t, "c.mp3"), ta.existing(t, "d.mp3")
	ta.setPlayNow([]model.Track{a, b})
	ta.setPlayNow([]model.Track{c, d})
	pnAssertPlayNow(t, ta, c, d)
	n := 0
	for _, p := range ta.playlists {
		if p.ID == playNowID {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("Play Now listed %d times", n)
	}
}

func TestPlayNowRenameDeleteAreNoOps(t *testing.T) {
	ta := newTestApp(t)
	ta.newPlaylist(t, "Mine")
	a := ta.existing(t, "a.mp3")
	ta.setPlayNow([]model.Track{a})
	before := pnSnapshot(t, ta)

	ta.promptRenamePlaylist()
	if n := len(ta.win.Canvas().Overlays().List()); n != 0 {
		t.Fatalf("rename opened %d overlays for Play Now", n)
	}
	ta.deleteSelectedPlaylist()
	if n := len(ta.win.Canvas().Overlays().List()); n != 0 {
		t.Fatalf("delete opened %d overlays for Play Now", n)
	}
	if !ta.pnHasPlayNow() {
		t.Fatal("Play Now removed")
	}
	pnAssertPlayNow(t, ta, a)
	pnAssertSnapshot(t, ta, before)
}

func TestPlayNowMenuHasNoActions(t *testing.T) {
	ta := newTestApp(t)
	ta.setPlayNow([]model.Track{ta.existing(t, "a.mp3")})
	if got := ta.playlistMenuActions(model.Playlist{ID: playNowID, Name: playNowName}); len(got) != 0 {
		t.Fatalf("actions = %v, want none", labels(got))
	}
}

func TestPlayNowMoveAndRemoveTouchMemoryOnly(t *testing.T) {
	ta := newTestApp(t)
	mine := ta.newPlaylist(t, "Mine")
	ta.fill(t, ta.libraryID(t), ta.existing(t, "lib.mp3"))
	ta.fill(t, mine.ID, ta.existing(t, "m.mp3"))
	a, b, c := ta.existing(t, "a.mp3"), ta.existing(t, "b.mp3"), ta.existing(t, "c.mp3")
	ta.setPlayNow([]model.Track{a, b, c})
	before := pnSnapshot(t, ta)

	ta.moveTrack(0, 2)
	pnAssertPlayNow(t, ta, b, c, a)
	pnAssertSnapshot(t, ta, before)

	ta.removeTrackFromPlaylist(1) // removes c
	pnAssertPlayNow(t, ta, b, a)
	pnAssertSnapshot(t, ta, before)
}

func TestTrackContextMenuAddTargetsExcludePlayNow(t *testing.T) {
	ta := newTestApp(t)
	mine := ta.newPlaylist(t, "Mine")
	other := ta.newPlaylist(t, "Other")
	ta.fill(t, ta.libraryID(t), ta.existing(t, "lib.mp3"))
	pnTrack := ta.existing(t, "a.mp3")
	ta.setPlayNow([]model.Track{pnTrack})

	// From Play Now: targets are saved playlists, never Play Now.
	ta.showTrackContextMenu(0, pnTrack, fyne.NewPos(10, 10))
	row := pnMenuRow(t, ta, "Add to playlist")
	if row == nil {
		t.Fatalf("no Add to playlist row; rows = %v", pnMenuLabels(t, ta))
	}
	test.Tap(row)
	got := pnMenuLabels(t, ta)
	for _, want := range []string{mine.Name, other.Name} {
		if !pnContains(got, want) {
			t.Errorf("target %q missing from %v", want, got)
		}
	}
	if pnContains(got, playNowName) {
		t.Errorf("Play Now offered as add target: %v", got)
	}
	// Library (the system playlist) is a valid target from Play Now.
	pls, err := ta.store.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	libName := ""
	for _, p := range pls {
		if p.System {
			libName = p.Name
		}
	}
	if libName == "" {
		t.Fatal("no system playlist")
	}
	if !pnContains(got, libName) {
		t.Errorf("Library %q not offered from Play Now: %v", libName, got)
	}
	ta.win.Canvas().Overlays().Remove(ta.win.Canvas().Overlays().Top())

	// From a saved playlist while Play Now exists: still excluded.
	ta.selectPlaylist(mine.ID)
	tr := ta.existing(t, "m.mp3")
	ta.showTrackContextMenu(0, tr, fyne.NewPos(10, 10))
	row = pnMenuRow(t, ta, "Add to playlist")
	if row == nil {
		t.Fatalf("no Add to playlist row; rows = %v", pnMenuLabels(t, ta))
	}
	test.Tap(row)
	got = pnMenuLabels(t, ta)
	if pnContains(got, playNowName) {
		t.Errorf("Play Now offered as add target from saved playlist: %v", got)
	}
	if !pnContains(got, other.Name) {
		t.Errorf("target %q missing from %v", other.Name, got)
	}
	if pnContains(got, libName) {
		t.Errorf("Library offered from saved playlist: %v", got)
	}
}

func TestAddTrackFromPlayNowWritesToDB(t *testing.T) {
	ta := newTestApp(t)
	mine := ta.newPlaylist(t, "Mine")
	a := ta.existing(t, "a.mp3")
	ta.setPlayNow([]model.Track{a})
	libID := ta.libraryID(t)

	ta.addTrackToPlaylist(a, model.Playlist{ID: libID, Name: "Library", System: true})
	ta.addTrackToPlaylist(a, mine)

	if got := storePaths(t, ta.App, libID); !equalStrings(got, []string{a.Path}) {
		t.Errorf("library = %v", got)
	}
	if got := storePaths(t, ta.App, mine.ID); !equalStrings(got, []string{a.Path}) {
		t.Errorf("mine = %v", got)
	}
	if got := storePaths(t, ta.App, playNowID); len(got) != 0 {
		t.Errorf("rows under playNowID: %v", got)
	}
	pnAssertPlayNow(t, ta, a)
}

func TestSelectPlayNowKeepsLastPlaylistPref(t *testing.T) {
	ta := newTestApp(t)
	mine := ta.newPlaylist(t, "Mine")
	ta.selectPlaylist(mine.ID)
	ta.setPlayNow([]model.Track{ta.existing(t, "a.mp3")})
	ta.selectPlaylist(playNowID)
	if got := ta.fyneApp.Preferences().Int(prefLastPlaylistID); got != int(mine.ID) {
		t.Fatalf("last_playlist_id = %d, want %d", got, mine.ID)
	}
}

func TestImportWithPlayNowSelectedGoesToLibrary(t *testing.T) {
	ta := newTestApp(t)
	a := ta.existing(t, "a.mp3")
	ta.setPlayNow([]model.Track{a})
	imp := ta.existing(t, "imp.mp3")
	libID := ta.libraryID(t)
	before := pnSnapshot(t, ta)

	ta.importPaths([]string{imp.Path})

	got := storePaths(t, ta.App, libID)
	if !equalStrings(got, []string{imp.Path}) {
		t.Fatalf("library = %v, want [%s]", got, imp.Path)
	}
	for id, paths := range pnSnapshot(t, ta) { // also asserts nothing under -1
		if id != libID && !equalStrings(paths, before[id]) {
			t.Errorf("playlist %d changed: %v", id, paths)
		}
	}
	if !equalStrings(pathsOf(ta.playNow), []string{a.Path}) {
		t.Errorf("playNow = %v, want unchanged", pathsOf(ta.playNow))
	}
}

func TestPlayNowRemoveMissingTouchesMemoryOnly(t *testing.T) {
	ta := newTestApp(t)
	ta.fill(t, ta.libraryID(t), ta.existing(t, "lib.mp3"))
	e, g := ta.existing(t, "e.mp3"), ta.ghost("g.mp3")
	ta.setPlayNow([]model.Track{e, g})
	before := pnSnapshot(t, ta)

	ta.removeMissingTracks([]model.Track{g})

	pnAssertPlayNow(t, ta, e)
	pnAssertSnapshot(t, ta, before)
}

func TestPlayNowRescanDialogRemovesMemoryOnly(t *testing.T) {
	ta := newTestApp(t)
	ta.fill(t, ta.libraryID(t), ta.existing(t, "lib.mp3"))
	e, g := ta.existing(t, "e.mp3"), ta.ghost("g.mp3")
	ta.setPlayNow([]model.Track{e, g})
	before := pnSnapshot(t, ta)

	ta.rescanSelectedPlaylist()
	btn := overlayButton(t, ta.App, "Remove")
	if btn == nil {
		t.Fatal("rescan dialog has no Remove button")
	}
	test.Tap(btn)

	pnAssertPlayNow(t, ta, e)
	pnAssertSnapshot(t, ta, before)
}

func TestRemovingLastPlayNowTrackRemovesPlayNow(t *testing.T) {
	t.Run("removeTrackFromPlaylist", func(t *testing.T) {
		ta := newTestApp(t)
		ta.setPlayNow([]model.Track{ta.existing(t, "a.mp3")})
		ta.removeTrackFromPlaylist(0)
		if ta.pnHasPlayNow() {
			t.Fatal("Play Now still listed after removing its last track")
		}
	})
	t.Run("removeMissingTracks", func(t *testing.T) {
		ta := newTestApp(t)
		g := ta.ghost("g.mp3")
		ta.setPlayNow([]model.Track{g})
		ta.removeMissingTracks([]model.Track{g})
		if ta.pnHasPlayNow() {
			t.Fatal("Play Now still listed after removing its last track")
		}
	})
}

func TestDeletingSavedPlaylistWithPlayNowSelectsLibrary(t *testing.T) {
	ta := newTestApp(t)
	mine := ta.newPlaylist(t, "Mine")
	ta.setPlayNow([]model.Track{ta.existing(t, "a.mp3")})
	ta.selectPlaylist(mine.ID)

	ta.deleteSelectedPlaylist()
	btn := overlayButton(t, ta.App, "Delete")
	if btn == nil {
		t.Fatalf("no Delete button; texts = %v", overlayTexts(t, ta.App))
	}
	test.Tap(btn)

	if ta.selectedPL != ta.libraryID(t) {
		t.Fatalf("selectedPL = %d, want Library %d", ta.selectedPL, ta.libraryID(t))
	}
}
