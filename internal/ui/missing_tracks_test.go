package ui

import (
	"errors"
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/rdurica/robsong/internal/model"
)

func labels(actions []ctxMenuAction) []string {
	var out []string
	for _, a := range actions {
		out = append(out, a.label)
	}
	return out
}

func TestPlaylistMenuActions(t *testing.T) {
	ta := newTestApp(t)
	user := ta.newPlaylist(t, "Mine")

	t.Run("library has only Rescan", func(t *testing.T) {
		pl := model.Playlist{ID: ta.libraryID(t), Name: "Library", System: true}
		got := labels(ta.playlistMenuActions(pl))
		if !equalStrings(got, []string{"Rescan"}) {
			t.Fatalf("labels = %v, want [Rescan]", got)
		}
	})
	t.Run("user playlist has Rescan Rename Delete(danger)", func(t *testing.T) {
		actions := ta.playlistMenuActions(user)
		if !equalStrings(labels(actions), []string{"Rescan", "Rename", "Delete"}) {
			t.Fatalf("labels = %v", labels(actions))
		}
		if !actions[2].danger {
			t.Error("Delete must be danger")
		}
		if actions[0].danger || actions[1].danger {
			t.Error("Rescan/Rename must not be danger")
		}
	})
}

func TestFindMissingTracks(t *testing.T) {
	ta := newTestApp(t)
	e1, e2 := ta.existing(t, "e1.mp3"), ta.existing(t, "e2.mp3")
	g1, g2 := ta.ghost("g1.mp3"), ta.ghost("g2.mp3")
	got := findMissingTracks([]model.Track{g1, e1, g2, e2})
	if !equalStrings(pathsOf(got), []string{g1.Path, g2.Path}) {
		t.Fatalf("missing = %v, want [g1 g2]", pathsOf(got))
	}
	if len(findMissingTracks([]model.Track{e1, e2})) != 0 {
		t.Error("existing tracks reported missing")
	}
}

func TestRescanNoMissing(t *testing.T) {
	ta := newTestApp(t)
	e1, e2 := ta.existing(t, "e1.mp3"), ta.existing(t, "e2.mp3")
	lib := ta.libraryID(t)
	ta.fill(t, lib, e1, e2)

	ta.rescanSelectedPlaylist()

	if ta.status != "Rescan: no missing tracks" {
		t.Errorf("status = %q", ta.status)
	}
	if overlayButton(t, ta.App, "Remove") != nil {
		t.Error("confirm dialog shown although nothing is missing")
	}
	if got := storePaths(t, ta.App, lib); !equalStrings(got, []string{e1.Path, e2.Path}) {
		t.Errorf("store changed: %v", got)
	}
}

// rescanFixture: Library = [e1, g1, e2, g2], selected.
func rescanFixture(t *testing.T) (*testApp, int64, model.Track, model.Track, model.Track, model.Track) {
	t.Helper()
	ta := newTestApp(t)
	e1, e2 := ta.existing(t, "e1.mp3"), ta.existing(t, "e2.mp3")
	g1, g2 := ta.ghost("g1.mp3"), ta.ghost("g2.mp3")
	lib := ta.libraryID(t)
	ta.fill(t, lib, e1, g1, e2, g2)
	return ta, lib, e1, g1, e2, g2
}

func TestRescanDialogListsMissingAndConfirmRemoves(t *testing.T) {
	ta, lib, e1, g1, e2, g2 := rescanFixture(t)

	ta.rescanSelectedPlaylist()

	btn := overlayButton(t, ta.App, "Remove")
	if btn == nil {
		t.Fatalf("no Remove button in overlays; texts = %v", overlayTexts(t, ta.App))
	}
	if btn.Importance != widget.DangerImportance {
		t.Errorf("Remove importance = %v, want Danger", btn.Importance)
	}
	if overlayButton(t, ta.App, "Cancel") == nil {
		t.Error("no Cancel button")
	}
	for _, g := range []model.Track{g1, g2} {
		if !overlayContains(t, ta.App, g.DisplayTitle()+" — "+g.DisplayArtist()) {
			t.Errorf("dialog does not list %q; texts = %v", g.Title, overlayTexts(t, ta.App))
		}
	}
	for _, e := range []model.Track{e1, e2} {
		if overlayContains(t, ta.App, e.Title) {
			t.Errorf("dialog lists existing track %q", e.Title)
		}
	}
	if got := storePaths(t, ta.App, lib); len(got) != 4 {
		t.Fatalf("store changed before confirm: %v", got)
	}

	test.Tap(btn)

	want := []string{e1.Path, e2.Path}
	if got := storePaths(t, ta.App, lib); !equalStrings(got, want) {
		t.Errorf("store = %v, want %v", got, want)
	}
	if got := pathsOf(ta.allTracks); !equalStrings(got, want) {
		t.Errorf("allTracks = %v, want %v", got, want)
	}
	if ta.status != "Rescan: removed 2 missing tracks" {
		t.Errorf("status = %q", ta.status)
	}
}

func TestRescanCancelKeepsEverything(t *testing.T) {
	ta, lib, _, _, _, _ := rescanFixture(t)
	ta.rescanSelectedPlaylist()
	cancel := overlayButton(t, ta.App, "Cancel")
	if cancel == nil {
		t.Fatalf("no Cancel button; texts = %v", overlayTexts(t, ta.App))
	}
	test.Tap(cancel)
	if got := storePaths(t, ta.App, lib); len(got) != 4 {
		t.Errorf("store changed after cancel: %v", got)
	}
	if len(ta.allTracks) != 4 {
		t.Errorf("allTracks changed after cancel: %d", len(ta.allTracks))
	}
	if ta.status == "Rescan: removed 2 missing tracks" {
		t.Error("status reports removal after cancel")
	}
}

func TestRescanIgnoresSearchFilter(t *testing.T) {
	ta, lib, e1, _, e2, _ := rescanFixture(t)
	ta.trackSearch.SetText("e1")
	ta.applyTrackFilter()
	if len(ta.tracks) != 1 {
		t.Fatalf("filter precondition: tracks = %d, want 1", len(ta.tracks))
	}

	ta.rescanSelectedPlaylist()
	btn := overlayButton(t, ta.App, "Remove")
	if btn == nil {
		t.Fatalf("no Remove button; texts = %v", overlayTexts(t, ta.App))
	}
	test.Tap(btn)

	if got := storePaths(t, ta.App, lib); !equalStrings(got, []string{e1.Path, e2.Path}) {
		t.Errorf("store = %v", got)
	}
}

func TestRescanLeavesDiskAndOtherPlaylistsAndClearsMissing(t *testing.T) {
	ta := newTestApp(t)
	e1 := ta.existing(t, "e1.mp3")
	g1, g2 := ta.ghost("g1.mp3"), ta.ghost("g2.mp3")
	lib := ta.libraryID(t)
	user := ta.newPlaylist(t, "Mine")
	if err := ta.store.AddTracks(lib, []model.Track{e1, g1, g2}); err != nil {
		t.Fatal(err)
	}
	if err := ta.store.AddTracks(user.ID, []model.Track{g1, e1, g2}); err != nil {
		t.Fatal(err)
	}
	ta.selectPlaylist(user.ID)
	// Mark g1 missing via a failed play attempt on the user playlist.
	ta.player.errs[g1.Path] = notExist(g1.Path)
	ta.playFrom(0)
	if !ta.isMissing(g1.Path) {
		t.Fatalf("precondition: g1 should be marked missing")
	}

	ta.rescanSelectedPlaylist()
	btn := overlayButton(t, ta.App, "Remove")
	if btn == nil {
		t.Fatalf("no Remove button; texts = %v", overlayTexts(t, ta.App))
	}
	test.Tap(btn)

	if got := storePaths(t, ta.App, user.ID); !equalStrings(got, []string{e1.Path}) {
		t.Errorf("user playlist = %v, want [e1]", got)
	}
	if got := storePaths(t, ta.App, lib); !equalStrings(got, []string{e1.Path, g1.Path, g2.Path}) {
		t.Errorf("library changed: %v", got)
	}
	if fileExists(e1.Path) == false {
		t.Error("existing file was deleted from disk")
	}
	if ta.isMissing(g1.Path) || ta.isMissing(g2.Path) {
		t.Error("removed paths still marked missing")
	}
}

func TestRescanRemovingPlayingTrackKeepsItCurrent(t *testing.T) {
	ta := newTestApp(t)
	e1, e2 := ta.existing(t, "e1.mp3"), ta.existing(t, "e2.mp3")
	g1 := ta.ghost("g1.mp3")
	lib := ta.libraryID(t)
	ta.fill(t, lib, e1, g1, e2)
	ta.playFrom(1) // fake player accepts the ghost path
	if cur, _ := ta.queue.Current(); cur.Path != g1.Path {
		t.Fatalf("precondition: current = %q", cur.Path)
	}

	ta.removeMissingTracks([]model.Track{g1})

	if cur, ok := ta.queue.Current(); !ok || cur.Path != g1.Path {
		t.Fatalf("current = %v/%v, want removed track kept current", cur.Path, ok)
	}
	ta.next()
	if got := ta.player.playedPaths(); got[len(got)-1] != e2.Path {
		t.Errorf("next played %v, want e2", got)
	}
}

func TestPlayFromMissingSkipsForward(t *testing.T) {
	ta := newTestApp(t)
	g, b := ta.ghost("g.mp3"), ta.existing(t, "b.mp3")
	ta.fill(t, ta.libraryID(t), g, b)
	ta.player.errs[g.Path] = notExist(g.Path)

	ta.playFrom(0)

	if !ta.isMissing(g.Path) {
		t.Error("g not marked missing")
	}
	if got := ta.player.playedPaths(); !equalStrings(got, []string{g.Path, b.Path}) {
		t.Errorf("played = %v", got)
	}
	if cur, _ := ta.queue.Current(); cur.Path != b.Path {
		t.Errorf("current = %q, want b", cur.Path)
	}
	if ta.isMissing(b.Path) {
		t.Error("b marked missing")
	}
}

func TestNextSkipsMissing(t *testing.T) {
	ta := newTestApp(t)
	a, g, c := ta.existing(t, "a.mp3"), ta.ghost("g.mp3"), ta.existing(t, "c.mp3")
	ta.fill(t, ta.libraryID(t), a, g, c)
	ta.player.errs[g.Path] = notExist(g.Path)
	ta.playFrom(0)

	ta.next()

	if !ta.isMissing(g.Path) {
		t.Error("g not marked missing")
	}
	if cur, _ := ta.queue.Current(); cur.Path != c.Path {
		t.Errorf("current = %q, want c", cur.Path)
	}
	got := ta.player.playedPaths()
	if got[len(got)-1] != c.Path {
		t.Errorf("last played = %q, want c", got[len(got)-1])
	}
}

func TestPrevSkipsMissingBackward(t *testing.T) {
	ta := newTestApp(t)
	a, g, c := ta.existing(t, "a.mp3"), ta.ghost("g.mp3"), ta.existing(t, "c.mp3")
	ta.fill(t, ta.libraryID(t), a, g, c)
	ta.player.errs[g.Path] = notExist(g.Path)
	ta.playFrom(0)
	ta.next() // skips g forward onto c

	ta.prev()

	if !ta.isMissing(g.Path) {
		t.Error("g not marked missing")
	}
	if cur, _ := ta.queue.Current(); cur.Path != a.Path {
		t.Errorf("current = %q, want a", cur.Path)
	}
	got := ta.player.playedPaths()
	if got[len(got)-1] != a.Path {
		t.Errorf("last played = %q, want a", got[len(got)-1])
	}
}

func TestTrackEndedSkipsMissing(t *testing.T) {
	ta := newTestApp(t)
	a, g, c := ta.existing(t, "a.mp3"), ta.ghost("g.mp3"), ta.existing(t, "c.mp3")
	ta.fill(t, ta.libraryID(t), a, g, c)
	ta.player.errs[g.Path] = notExist(g.Path)
	ta.playFrom(0)

	ta.onTrackEnded()

	if cur, _ := ta.queue.Current(); cur.Path != c.Path {
		t.Errorf("current = %q, want c", cur.Path)
	}
	if !ta.isMissing(g.Path) {
		t.Error("g not marked missing")
	}
}

func TestAllRemainingMissingStops(t *testing.T) {
	ta := newTestApp(t)
	a, g1, g2 := ta.existing(t, "a.mp3"), ta.ghost("g1.mp3"), ta.ghost("g2.mp3")
	ta.fill(t, ta.libraryID(t), a, g1, g2)
	ta.player.errs[g1.Path] = notExist(g1.Path)
	ta.player.errs[g2.Path] = notExist(g2.Path)
	ta.playFrom(0)
	stopsBefore := ta.player.stopCount()

	ta.next()

	if !ta.isMissing(g1.Path) || !ta.isMissing(g2.Path) {
		t.Error("both ghosts should be marked missing")
	}
	if ta.player.stopCount() <= stopsBefore {
		t.Error("player.Stop not called")
	}
	if ta.status != "No playable tracks left" {
		t.Errorf("status = %q", ta.status)
	}
}

func TestMissingMarkIsPerPathAcrossPlaylists(t *testing.T) {
	ta := newTestApp(t)
	g, b := ta.ghost("g.mp3"), ta.existing(t, "b.mp3")
	lib := ta.libraryID(t)
	other := ta.newPlaylist(t, "Other")
	if err := ta.store.AddTracks(other.ID, []model.Track{b, g}); err != nil {
		t.Fatal(err)
	}
	ta.fill(t, lib, g, b)
	ta.player.errs[g.Path] = notExist(g.Path)
	ta.playFrom(0)
	if !ta.isMissing(g.Path) {
		t.Fatal("g not marked missing")
	}

	ta.selectPlaylist(other.ID)

	if !ta.isMissing(g.Path) {
		t.Error("mark lost after switching playlist")
	}
	if ta.isMissing(b.Path) {
		t.Error("b wrongly missing")
	}
}

func TestTrackRowUpdateMissingStyling(t *testing.T) {
	ta := newTestApp(t)
	row := newTrackRow(ta.App)
	tr := model.Track{Path: "/x.mp3", Title: "T", Artist: "A", DurationMs: 65000}

	row.Update(0, tr, false, true)
	if row.line.Importance != widget.LowImportance || row.duration.Importance != widget.LowImportance {
		t.Errorf("missing: line=%v duration=%v, want both Low", row.line.Importance, row.duration.Importance)
	}
	if row.duration.Text == "" {
		t.Error("duration text must still be shown for missing track")
	}
	row.Update(0, tr, false, false)
	if row.line.Importance != widget.MediumImportance {
		t.Errorf("not missing: line importance = %v, want Medium", row.line.Importance)
	}
}

func TestReplayMissingTrackWhoseFileReappeared(t *testing.T) {
	ta := newTestApp(t)
	g, b := ta.ghost("g.mp3"), ta.existing(t, "b.mp3")
	ta.fill(t, ta.libraryID(t), g, b)
	ta.player.errs[g.Path] = notExist(g.Path)
	ta.playFrom(0)
	if !ta.isMissing(g.Path) {
		t.Fatal("precondition: g missing")
	}

	delete(ta.player.errs, g.Path) // file is back
	ta.playFrom(0)

	if ta.isMissing(g.Path) {
		t.Error("g still marked missing after successful play")
	}
	if cur, _ := ta.queue.Current(); cur.Path != g.Path {
		t.Errorf("current = %q, want g", cur.Path)
	}
}

func TestOtherPlayErrorDoesNotMarkOrSkip(t *testing.T) {
	ta := newTestApp(t)
	a, b := ta.existing(t, "a.mp3"), ta.existing(t, "b.mp3")
	ta.fill(t, ta.libraryID(t), a, b)
	ta.player.errs[a.Path] = errors.New("unsupported format")

	ta.playFrom(0)

	if ta.status != "Play error: unsupported format" {
		t.Errorf("status = %q", ta.status)
	}
	if ta.isMissing(a.Path) {
		t.Error("a marked missing for non-ENOENT error")
	}
	if got := ta.player.playedPaths(); !equalStrings(got, []string{a.Path}) {
		t.Errorf("played = %v, want only a", got)
	}
}

func TestRemoveTrackFromPlaylistClearsMissing(t *testing.T) {
	ta := newTestApp(t)
	g, b := ta.ghost("g.mp3"), ta.existing(t, "b.mp3")
	lib := ta.libraryID(t)
	ta.fill(t, lib, g, b)
	ta.player.errs[g.Path] = notExist(g.Path)
	ta.playFrom(0)
	if !ta.isMissing(g.Path) {
		t.Fatal("precondition: g missing")
	}

	ta.removeTrackFromPlaylist(0)

	if got := storePaths(t, ta.App, lib); !equalStrings(got, []string{b.Path}) {
		t.Errorf("store = %v", got)
	}
	if got := pathsOf(ta.allTracks); !equalStrings(got, []string{b.Path}) {
		t.Errorf("allTracks = %v", got)
	}
	if ta.isMissing(g.Path) {
		t.Error("removed path still marked missing")
	}
}

func TestFreshAppHasNothingMissing(t *testing.T) {
	ta := newTestApp(t)
	for _, p := range []string{"", "/a.mp3", "/does/not/exist.mp3"} {
		if ta.isMissing(p) {
			t.Errorf("isMissing(%q) = true on fresh app", p)
		}
	}
}

// titledGhost returns a missing track with an explicit Title.
func (ta *testApp) titledGhost(file, title string) model.Track {
	g := ta.ghost(file)
	g.Title = title
	return g
}

func TestPlayFromSkipSetsSingleTrackStatus(t *testing.T) {
	ta := newTestApp(t)
	g, b := ta.titledGhost("g.mp3", "Ghost"), ta.existing(t, "b.mp3")
	ta.fill(t, ta.libraryID(t), g, b)
	ta.player.errs[g.Path] = notExist(g.Path)

	ta.playFrom(0)

	if want := `Track not found: "Ghost"`; ta.status != want {
		t.Errorf("status = %q, want %q", ta.status, want)
	}
}

func TestNextSkippingTwoSetsPluralStatus(t *testing.T) {
	ta := newTestApp(t)
	a, c := ta.existing(t, "a.mp3"), ta.existing(t, "c.mp3")
	g1, g2 := ta.titledGhost("g1.mp3", "Ghost1"), ta.titledGhost("g2.mp3", "Ghost2")
	ta.fill(t, ta.libraryID(t), a, g1, g2, c)
	ta.player.errs[g1.Path] = notExist(g1.Path)
	ta.player.errs[g2.Path] = notExist(g2.Path)
	ta.playFrom(0)

	ta.next()

	if cur, _ := ta.queue.Current(); cur.Path != c.Path {
		t.Fatalf("current = %q, want c", cur.Path)
	}
	if want := "2 tracks not found, skipped"; ta.status != want {
		t.Errorf("status = %q, want %q", ta.status, want)
	}
}

func TestPrevSkipSetsSingleTrackStatus(t *testing.T) {
	ta := newTestApp(t)
	a, c := ta.existing(t, "a.mp3"), ta.existing(t, "c.mp3")
	g := ta.titledGhost("g.mp3", "Ghost")
	ta.fill(t, ta.libraryID(t), a, g, c)
	ta.player.errs[g.Path] = notExist(g.Path)
	ta.playFrom(0)
	ta.next() // skips g onto c
	ta.status = ""

	ta.prev()

	if cur, _ := ta.queue.Current(); cur.Path != a.Path {
		t.Fatalf("current = %q, want a", cur.Path)
	}
	if want := `Track not found: "Ghost"`; ta.status != want {
		t.Errorf("status = %q, want %q", ta.status, want)
	}
}

func TestTrackEndedSkipSetsSingleTrackStatus(t *testing.T) {
	ta := newTestApp(t)
	a, c := ta.existing(t, "a.mp3"), ta.existing(t, "c.mp3")
	g := ta.titledGhost("g.mp3", "Ghost")
	ta.fill(t, ta.libraryID(t), a, g, c)
	ta.player.errs[g.Path] = notExist(g.Path)
	ta.playFrom(0)

	ta.onTrackEnded()

	if cur, _ := ta.queue.Current(); cur.Path != c.Path {
		t.Fatalf("current = %q, want c", cur.Path)
	}
	if want := `Track not found: "Ghost"`; ta.status != want {
		t.Errorf("status = %q, want %q", ta.status, want)
	}
}

func TestPlayFromWithoutSkipLeavesStatusEmpty(t *testing.T) {
	ta := newTestApp(t)
	a, b := ta.existing(t, "a.mp3"), ta.existing(t, "b.mp3")
	ta.fill(t, ta.libraryID(t), a, b)
	ta.status = "stale"

	ta.playFrom(0)

	if ta.status != "" {
		t.Errorf("status = %q, want empty", ta.status)
	}
}

// rescanWithGhosts fills the Library with n missing tracks and opens the rescan dialog.
func rescanWithGhosts(t *testing.T, n int) *testApp {
	t.Helper()
	ta := newTestApp(t)
	ta.win.Resize(fyne.NewSize(1024, 700))
	ghosts := make([]model.Track, 0, n)
	for i := 0; i < n; i++ {
		ghosts = append(ghosts, ta.ghost(fmt.Sprintf("g%03d.mp3", i)))
	}
	ta.fill(t, ta.libraryID(t), ghosts...)
	ta.rescanSelectedPlaylist()
	return ta
}

// overlayVScroll returns the first vertical scroll container in the dialog overlay.
func overlayVScroll(t *testing.T, a *App) *container.Scroll {
	t.Helper()
	var found *container.Scroll
	for _, ov := range a.win.Canvas().Overlays().List() {
		collectObjects(t, ov, func(o fyne.CanvasObject) {
			if sc, ok := o.(*container.Scroll); ok && found == nil && sc.Direction != container.ScrollHorizontalOnly {
				found = sc
			}
		})
	}
	return found
}

func overlayMinHeight(a *App) float32 {
	var h float32
	for _, ov := range a.win.Canvas().Overlays().List() {
		if m := ov.MinSize().Height; m > h {
			h = m
		}
	}
	return h
}

func TestRescanDialogListAlwaysInScroll(t *testing.T) {
	for _, n := range []int{2, 50} {
		t.Run(fmt.Sprintf("%d missing", n), func(t *testing.T) {
			ta := rescanWithGhosts(t, n)
			if overlayVScroll(t, ta.App) == nil {
				t.Fatalf("no vertical Scroll in dialog overlay; texts = %v", overlayTexts(t, ta.App))
			}
		})
	}
}

func TestRescanDialogHeightBoundedWithManyTracks(t *testing.T) {
	ref := rescanWithGhosts(t, 12)
	refH := overlayMinHeight(ref.App)

	ta := rescanWithGhosts(t, 50)
	sc := overlayVScroll(t, ta.App)
	if sc == nil {
		t.Fatalf("no vertical Scroll in dialog overlay")
	}
	if h := sc.MinSize().Height; h > 320 {
		t.Errorf("scroll MinSize height = %v, want <= 320", h)
	}
	if h := overlayMinHeight(ta.App); h > refH {
		t.Errorf("overlay MinSize height with 50 tracks = %v, exceeds 12-track height %v", h, refH)
	}
}

func TestRescanDialogHeaderShowsCount(t *testing.T) {
	cases := map[int]string{
		1:  "1 track no longer exists on disk.",
		2:  "2 tracks no longer exist on disk.",
		50: "50 tracks no longer exist on disk.",
	}
	for n, want := range cases {
		t.Run(fmt.Sprintf("%d missing", n), func(t *testing.T) {
			ta := rescanWithGhosts(t, n)
			if !overlayContains(t, ta.App, want) {
				t.Errorf("header %q not found; texts = %v", want, overlayTexts(t, ta.App))
			}
		})
	}
}
