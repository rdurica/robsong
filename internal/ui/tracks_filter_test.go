package ui

import (
	"testing"

	"github.com/rdurica/robsong/internal/model"
)

func TestFilterTracksByQuery(t *testing.T) {
	tracks := []model.Track{
		{Title: "Bohemian Rhapsody", Artist: "Queen"},
		{Title: "Stairway to Heaven", Artist: "Led Zeppelin"},
		{Title: "Blackbird", Artist: "The Beatles"},
		{Title: "No Artist Song", Artist: ""},
		{Path: "/music/untitled.mp3", Title: "", Artist: "Solo Act"},
	}

	t.Run("empty query returns all", func(t *testing.T) {
		got := filterTracksByQuery(tracks, "  ")
		if len(got) != len(tracks) {
			t.Fatalf("len=%d want %d", len(got), len(tracks))
		}
	})

	t.Run("matches title case-insensitive", func(t *testing.T) {
		got := filterTracksByQuery(tracks, "bohemian")
		if len(got) != 1 || got[0].Title != "Bohemian Rhapsody" {
			t.Fatalf("got=%v", got)
		}
	})

	t.Run("matches artist case-insensitive", func(t *testing.T) {
		got := filterTracksByQuery(tracks, "zeppelin")
		if len(got) != 1 || got[0].Artist != "Led Zeppelin" {
			t.Fatalf("got=%v", got)
		}
	})

	t.Run("matches title or artist", func(t *testing.T) {
		got := filterTracksByQuery(tracks, "black")
		if len(got) != 1 || got[0].Title != "Blackbird" {
			t.Fatalf("got=%v", got)
		}
		got = filterTracksByQuery(tracks, "beat")
		if len(got) != 1 || got[0].Artist != "The Beatles" {
			t.Fatalf("got=%v", got)
		}
	})

	t.Run("does not match Unknown artist placeholder", func(t *testing.T) {
		got := filterTracksByQuery(tracks, "unknown")
		if len(got) != 0 {
			t.Fatalf("got=%v want empty", got)
		}
	})

	t.Run("falls back to path for empty title", func(t *testing.T) {
		got := filterTracksByQuery(tracks, "untitled")
		if len(got) != 1 || got[0].Artist != "Solo Act" {
			t.Fatalf("got=%v", got)
		}
	})
}
