package main

import (
	"path/filepath"

	"github.com/rdurica/robsong/internal/library"
	"github.com/rdurica/robsong/internal/model"
)

type launchMode int

const (
	launchFull launchMode = iota
	launchMini
	launchNoTracks
)

// chooseLaunch picks the startup window from the argument count and the
// number of supported tracks found.
func chooseLaunch(hadArgs bool, n int) launchMode {
	switch {
	case !hadArgs:
		return launchFull
	case n == 0:
		return launchNoTracks
	case n == 1:
		return launchMini
	default:
		return launchFull
	}
}

// collectTracks expands each argument (file or folder) in order, skipping
// unreadable or unsupported ones and de-duplicating by absolute path.
func collectTracks(args []string) []model.Track {
	var out []model.Track
	seen := map[string]struct{}{}
	for _, a := range args {
		ts, _ := library.ImportPaths([]string{a})
		for _, t := range ts {
			abs, err := filepath.Abs(t.Path)
			if err != nil {
				abs = t.Path
			}
			if _, ok := seen[abs]; ok {
				continue
			}
			seen[abs] = struct{}{}
			t.Path = abs
			out = append(out, t)
		}
	}
	return out
}
