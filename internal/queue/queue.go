package queue

import (
	"sync"

	"github.com/rdurica/robsong/internal/model"
)

// Queue is an in-memory playback queue. Index points at the current track.
type Queue struct {
	mu     sync.RWMutex
	tracks []model.Track
	index  int // -1 when empty / nothing current
}

// New returns an empty queue.
func New() *Queue {
	return &Queue{index: -1}
}

// Current returns the current track and whether one exists.
func (q *Queue) Current() (model.Track, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.index < 0 || q.index >= len(q.tracks) {
		return model.Track{}, false
	}
	return q.tracks[q.index], true
}

// Set replaces the entire queue and starts at index 0.
func (q *Queue) Set(tracks []model.Track) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = append([]model.Track{}, tracks...)
	if len(q.tracks) == 0 {
		q.index = -1
		return
	}
	q.index = 0
}

// Next advances to the next track. Returns the track and false if at end.
func (q *Queue) Next() (model.Track, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.index < 0 || q.index+1 >= len(q.tracks) {
		return model.Track{}, false
	}
	q.index++
	return q.tracks[q.index], true
}

// Prev moves to the previous track. Returns the track and false if at start.
func (q *Queue) Prev() (model.Track, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.index <= 0 {
		return model.Track{}, false
	}
	q.index--
	return q.tracks[q.index], true
}
