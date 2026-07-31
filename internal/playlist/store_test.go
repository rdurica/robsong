package playlist_test

import (
	"path/filepath"
	"testing"

	"github.com/rdurica/robsong/internal/model"
	"github.com/rdurica/robsong/internal/playlist"
)

func TestLibrarySeedAndTracks(t *testing.T) {
	dir := t.TempDir()
	store, err := playlist.OpenPath(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	pls, err := store.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	if len(pls) != 1 || pls[0].Name != "Library" || !pls[0].System {
		t.Fatalf("unexpected playlists: %+v", pls)
	}

	libID, err := store.LibraryID()
	if err != nil {
		t.Fatal(err)
	}
	tracks := []model.Track{
		{Path: "/music/a.mp3", Title: "A", Artist: "Art", Album: "Alb", DurationMs: 1000},
		{Path: "/music/b.flac", Title: "B", Artist: "Art", DurationMs: 2000},
	}
	if err := store.AddTracks(libID, tracks); err != nil {
		t.Fatal(err)
	}
	// duplicate should be ignored
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

	p, err := store.CreatePlaylist("Favorites")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddTracks(p.ID, got[:1]); err != nil {
		t.Fatal(err)
	}
	fav, err := store.ListTracks(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fav) != 1 || fav[0].Title != "A" {
		t.Fatalf("fav=%+v", fav)
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

	if err := store.DeletePlaylist(libID); err == nil {
		t.Fatal("expected error deleting system playlist")
	}
	if err := store.DeletePlaylist(p.ID); err != nil {
		t.Fatal(err)
	}
}
