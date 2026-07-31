package playlist

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rdurica/robsong/internal/model"

	_ "modernc.org/sqlite"
)

const libraryName = "Library"

// Store persists playlists and their tracks in SQLite.
type Store struct {
	db     *sql.DB
	closed sync.Once
}

// Open creates or opens the database at the default config path
// (~/.config/robsong/library.db) and seeds the Library playlist.
func Open() (*Store, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	return OpenPath(filepath.Join(dir, "library.db"))
}

// OpenPath opens a store at an explicit database path (useful for tests).
func OpenPath(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.seedLibrary(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "robsong"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "robsong"), nil
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS playlists (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	system INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS playlist_tracks (
	playlist_id INTEGER NOT NULL,
	position INTEGER NOT NULL,
	path TEXT NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	artist TEXT NOT NULL DEFAULT '',
	album TEXT NOT NULL DEFAULT '',
	duration_ms INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (playlist_id, position),
	FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (s *Store) seedLibrary() error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM playlists WHERE system = 1 AND name = ?`, libraryName).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = s.db.Exec(
		`INSERT INTO playlists (name, created_at, system) VALUES (?, ?, 1)`,
		libraryName, time.Now().Unix(),
	)
	return err
}

// Close closes the underlying database. Safe to call more than once.
func (s *Store) Close() error {
	var err error
	s.closed.Do(func() {
		err = s.db.Close()
	})
	return err
}

// ListPlaylists returns all playlists ordered by system first, then name.
func (s *Store) ListPlaylists() ([]model.Playlist, error) {
	rows, err := s.db.Query(`
		SELECT id, name, created_at, system
		FROM playlists
		ORDER BY system DESC, name COLLATE NOCASE ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlaylists(rows)
}

// PlaylistsContaining returns playlists that include a track with the given path.
func (s *Store) PlaylistsContaining(path string) ([]model.Playlist, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.created_at, p.system
		FROM playlists p
		JOIN playlist_tracks pt ON pt.playlist_id = p.id
		WHERE pt.path = ?
		ORDER BY p.system DESC, p.name COLLATE NOCASE ASC
	`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlaylists(rows)
}

func scanPlaylists(rows *sql.Rows) ([]model.Playlist, error) {
	var out []model.Playlist
	for rows.Next() {
		var p model.Playlist
		var system int
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &system); err != nil {
			return nil, err
		}
		p.System = system == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// LibraryID returns the id of the system Library playlist.
func (s *Store) LibraryID() (int64, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM playlists WHERE system = 1 AND name = ?`, libraryName).Scan(&id)
	return id, err
}

// CreatePlaylist creates a user playlist and returns it.
func (s *Store) CreatePlaylist(name string) (model.Playlist, error) {
	now := time.Now().Unix()
	res, err := s.db.Exec(
		`INSERT INTO playlists (name, created_at, system) VALUES (?, ?, 0)`,
		name, now,
	)
	if err != nil {
		return model.Playlist{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Playlist{}, err
	}
	return model.Playlist{ID: id, Name: name, CreatedAt: now, System: false}, nil
}

// RenamePlaylist renames a non-system playlist.
func (s *Store) RenamePlaylist(id int64, name string) error {
	res, err := s.db.Exec(`UPDATE playlists SET name = ? WHERE id = ? AND system = 0`, name, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("playlist not found or is system")
	}
	return nil
}

// DeletePlaylist deletes a non-system playlist and its tracks.
func (s *Store) DeletePlaylist(id int64) error {
	res, err := s.db.Exec(`DELETE FROM playlists WHERE id = ? AND system = 0`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("playlist not found or is system")
	}
	return nil
}

// ListTracks returns tracks for a playlist ordered by position.
func (s *Store) ListTracks(playlistID int64) ([]model.Track, error) {
	rows, err := s.db.Query(`
		SELECT rowid, path, title, artist, album, duration_ms
		FROM playlist_tracks
		WHERE playlist_id = ?
		ORDER BY position ASC
	`, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Track
	for rows.Next() {
		var t model.Track
		if err := rows.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.DurationMs); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AddTracks appends tracks to a playlist (and de-duplicates paths within that playlist).
func (s *Store) AddTracks(playlistID int64, tracks []model.Track) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var maxPos sql.NullInt64
	if err := tx.QueryRow(
		`SELECT MAX(position) FROM playlist_tracks WHERE playlist_id = ?`, playlistID,
	).Scan(&maxPos); err != nil {
		return err
	}
	pos := 0
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}

	existing := map[string]struct{}{}
	rows, err := tx.Query(`SELECT path FROM playlist_tracks WHERE playlist_id = ?`, playlistID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		existing[p] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO playlist_tracks (playlist_id, position, path, title, artist, album, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range tracks {
		if _, ok := existing[t.Path]; ok {
			continue
		}
		if _, err := stmt.Exec(playlistID, pos, t.Path, t.Title, t.Artist, t.Album, t.DurationMs); err != nil {
			return err
		}
		existing[t.Path] = struct{}{}
		pos++
	}
	return tx.Commit()
}

// RemoveTrackAt removes the track at the given position and reindexes.
func (s *Store) RemoveTrackAt(playlistID int64, position int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`DELETE FROM playlist_tracks WHERE playlist_id = ? AND position = ?`,
		playlistID, position,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE playlist_tracks
		SET position = position - 1
		WHERE playlist_id = ? AND position > ?
	`, playlistID, position); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceOrder rewrites playlist track positions to match the given order.
func (s *Store) ReplaceOrder(playlistID int64, tracks []model.Track) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ?`, playlistID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO playlist_tracks (playlist_id, position, path, title, artist, album, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, t := range tracks {
		if _, err := stmt.Exec(playlistID, i, t.Path, t.Title, t.Artist, t.Album, t.DurationMs); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MoveTrack swaps track at from with to within a playlist and returns the new order.
func (s *Store) MoveTrack(playlistID int64, from, to int) ([]model.Track, error) {
	tracks, err := s.ListTracks(playlistID)
	if err != nil {
		return nil, err
	}
	if from < 0 || to < 0 || from >= len(tracks) || to >= len(tracks) || from == to {
		return tracks, nil
	}
	item := tracks[from]
	if from < to {
		copy(tracks[from:to], tracks[from+1:to+1])
		tracks[to] = item
	} else {
		copy(tracks[to+1:from+1], tracks[to:from])
		tracks[to] = item
	}
	if err := s.ReplaceOrder(playlistID, tracks); err != nil {
		return nil, err
	}
	return tracks, nil
}
