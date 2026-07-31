package playlist_test

import (
	"path/filepath"
	"testing"

	"github.com/rdurica/robsong/internal/model"
	"github.com/rdurica/robsong/internal/playlist"
)

func openTestStore(t *testing.T) *playlist.Store {
	t.Helper()
	store, err := playlist.OpenPath(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func sampleTracks() []model.Track {
	return []model.Track{
		{Path: "/music/a.mp3", Title: "A", Artist: "Art", Album: "Alb", DurationMs: 1000},
		{Path: "/music/b.flac", Title: "B", Artist: "Art", DurationMs: 2000},
		{Path: "/music/c.wav", Title: "C", Artist: "Art", DurationMs: 3000},
	}
}

func TestLibrarySeed(t *testing.T) {
	store := openTestStore(t)

	pls, err := store.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	if len(pls) != 1 || pls[0].Name != "Library" || !pls[0].System {
		t.Fatalf("unexpected playlists: %+v", pls)
	}
	if _, err := store.LibraryID(); err != nil {
		t.Fatal(err)
	}
}

func TestAddTracksDedupes(t *testing.T) {
	store := openTestStore(t)
	libID, err := store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	tracks := sampleTracks()[:2]
	if err := store.AddTracks(libID, tracks); err != nil {
		t.Fatal(err)
	}
	if err := store.AddTracks(libID, tracks[:1]); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListTracks(libID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d tracks", len(got))
	}
}

func TestPlaylistsContaining(t *testing.T) {
	store := openTestStore(t)
	libID, err := store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	tracks := sampleTracks()[:2]
	if err := store.AddTracks(libID, tracks); err != nil {
		t.Fatal(err)
	}
	p, err := store.CreatePlaylist("Favorites")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddTracks(p.ID, tracks[:1]); err != nil {
		t.Fatal(err)
	}

	containing, err := store.PlaylistsContaining("/music/a.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if len(containing) != 2 {
		t.Fatalf("PlaylistsContaining got %d playlists: %+v", len(containing), containing)
	}
	if containing[0].Name != "Library" || !containing[0].System {
		t.Fatalf("expected Library first: %+v", containing[0])
	}
	if containing[1].Name != "Favorites" {
		t.Fatalf("expected Favorites second: %+v", containing[1])
	}

	none, err := store.PlaylistsContaining("/music/missing.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no playlists, got %+v", none)
	}
}

func TestRenamePlaylist(t *testing.T) {
	store := openTestStore(t)
	libID, err := store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RenamePlaylist(libID, "Nope"); err == nil {
		t.Fatal("expected error renaming system playlist")
	}

	p, err := store.CreatePlaylist("Old")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RenamePlaylist(p.ID, "New"); err != nil {
		t.Fatal(err)
	}
	pls, err := store.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pl := range pls {
		if pl.ID == p.ID {
			found = true
			if pl.Name != "New" {
				t.Fatalf("name=%q", pl.Name)
			}
		}
	}
	if !found {
		t.Fatal("renamed playlist missing")
	}
}

func TestDeletePlaylist(t *testing.T) {
	store := openTestStore(t)
	libID, err := store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePlaylist(libID); err == nil {
		t.Fatal("expected error deleting system playlist")
	}

	p, err := store.CreatePlaylist("Temp")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePlaylist(p.ID); err != nil {
		t.Fatal(err)
	}
	pls, err := store.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	if len(pls) != 1 || pls[0].ID != libID {
		t.Fatalf("unexpected playlists after delete: %+v", pls)
	}
}

func TestRemoveTrackAt(t *testing.T) {
	store := openTestStore(t)
	libID, err := store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddTracks(libID, sampleTracks()); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveTrackAt(libID, 1); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListTracks(libID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d tracks", len(got))
	}
	if got[0].Title != "A" || got[1].Title != "C" {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestMoveTrack(t *testing.T) {
	store := openTestStore(t)
	libID, err := store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddTracks(libID, sampleTracks()); err != nil {
		t.Fatal(err)
	}

	moved, err := store.MoveTrack(libID, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 3 || moved[0].Title != "B" || moved[1].Title != "C" || moved[2].Title != "A" {
		t.Fatalf("unexpected move result: %+v", moved)
	}

	got, err := store.ListTracks(libID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Title != "B" || got[1].Title != "C" || got[2].Title != "A" {
		t.Fatalf("unexpected persisted order: %+v", got)
	}

	same, err := store.MoveTrack(libID, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(same) != 3 || same[0].Title != "B" {
		t.Fatalf("noop move changed order: %+v", same)
	}
}

func TestReplaceOrder(t *testing.T) {
	store := openTestStore(t)
	libID, err := store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	tracks := sampleTracks()
	if err := store.AddTracks(libID, tracks); err != nil {
		t.Fatal(err)
	}
	reversed := []model.Track{tracks[2], tracks[1], tracks[0]}
	if err := store.ReplaceOrder(libID, reversed); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListTracks(libID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Title != "C" || got[1].Title != "B" || got[2].Title != "A" {
		t.Fatalf("unexpected order: %+v", got)
	}
}
