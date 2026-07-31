package library

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	"github.com/rdurica/robsong/internal/audio"
	"github.com/rdurica/robsong/internal/model"
)

// ImportPaths imports files and directories, returning discovered tracks.
func ImportPaths(paths []string) ([]model.Track, error) {
	var tracks []model.Track
	seen := map[string]struct{}{}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return tracks, fmt.Errorf("stat %s: %w", p, err)
		}
		if info.IsDir() {
			_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if !audio.SupportedExt(filepath.Ext(path)) {
					return nil
				}
				abs, err := filepath.Abs(path)
				if err != nil {
					return nil
				}
				if _, ok := seen[abs]; ok {
					return nil
				}
				seen[abs] = struct{}{}
				t, err := ReadTrack(abs)
				if err != nil {
					return nil
				}
				tracks = append(tracks, t)
				return nil
			})
			continue
		}
		if !audio.SupportedExt(filepath.Ext(p)) {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		t, err := ReadTrack(abs)
		if err != nil {
			continue
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// ReadTrack reads metadata and duration for a single audio file.
func ReadTrack(path string) (model.Track, error) {
	t := model.Track{
		Path:  path,
		Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
	}

	f, err := os.Open(path)
	if err != nil {
		return t, err
	}
	meta, err := tag.ReadFrom(f)
	_ = f.Close()
	if err == nil {
		if meta.Title() != "" {
			t.Title = meta.Title()
		}
		if meta.Artist() != "" {
			t.Artist = meta.Artist()
		}
		if meta.Album() != "" {
			t.Album = meta.Album()
		}
	}

	if ms, err := DurationMs(path); err == nil {
		t.DurationMs = ms
	}
	return t, nil
}

// DurationMs probes the audio length in milliseconds.
func DurationMs(path string) (int64, error) {
	streamer, format, err := audio.Decode(path)
	if err != nil {
		return 0, err
	}
	defer streamer.Close()
	return format.SampleRate.D(streamer.Len()).Milliseconds(), nil
}
