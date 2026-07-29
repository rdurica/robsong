package model

// Track represents a playable audio file with optional metadata.
type Track struct {
	ID         int64
	Path       string
	Title      string
	Artist     string
	Album      string
	DurationMs int64
}

// DisplayTitle returns Title or the basename fallback already stored in Title.
func (t Track) DisplayTitle() string {
	if t.Title != "" {
		return t.Title
	}
	return t.Path
}

// DisplayArtist returns Artist or a placeholder.
func (t Track) DisplayArtist() string {
	if t.Artist != "" {
		return t.Artist
	}
	return "Unknown artist"
}

// Playlist is a named collection of tracks persisted in SQLite.
type Playlist struct {
	ID        int64
	Name      string
	CreatedAt int64
	System    bool // true for the seeded Library playlist
}
