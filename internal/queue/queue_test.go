package queue_test

import (
	"testing"

	"github.com/rdurica/robsong/internal/model"
	"github.com/rdurica/robsong/internal/queue"
)

func track(title string) model.Track {
	return model.Track{Path: title + ".mp3", Title: title}
}

func TestSetAndCurrent(t *testing.T) {
	q := queue.New()
	if _, ok := q.Current(); ok {
		t.Fatal("expected empty queue")
	}
	q.Set([]model.Track{track("a"), track("b")})
	cur, ok := q.Current()
	if !ok || cur.Title != "a" {
		t.Fatalf("current=%v ok=%v", cur, ok)
	}
}

func TestNextAndPrev(t *testing.T) {
	q := queue.New()
	q.Set([]model.Track{track("a"), track("b"), track("c")})

	next, ok := q.Next()
	if !ok || next.Title != "b" {
		t.Fatalf("next=%v ok=%v", next, ok)
	}
	next, ok = q.Next()
	if !ok || next.Title != "c" {
		t.Fatalf("next=%v ok=%v", next, ok)
	}
	if _, ok := q.Next(); ok {
		t.Fatal("expected end of queue")
	}

	prev, ok := q.Prev()
	if !ok || prev.Title != "b" {
		t.Fatalf("prev=%v ok=%v", prev, ok)
	}
	prev, ok = q.Prev()
	if !ok || prev.Title != "a" {
		t.Fatalf("prev=%v ok=%v", prev, ok)
	}
	if _, ok := q.Prev(); ok {
		t.Fatal("expected start of queue")
	}
}

func TestSetReplaces(t *testing.T) {
	q := queue.New()
	q.Set([]model.Track{track("a"), track("b")})
	_, _ = q.Next()
	q.Set([]model.Track{track("x"), track("y")})
	cur, ok := q.Current()
	if !ok || cur.Title != "x" {
		t.Fatalf("current=%v ok=%v", cur, ok)
	}
}

func TestSetEmpty(t *testing.T) {
	q := queue.New()
	q.Set([]model.Track{track("a")})
	q.Set(nil)
	if _, ok := q.Current(); ok {
		t.Fatal("expected empty queue")
	}
}
