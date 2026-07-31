package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// SupportedExtensions lists playable file extensions (with leading dot).
var SupportedExtensions = []string{".mp3", ".wav", ".flac", ".ogg"}

// Decode opens path and returns a seekable streamer for a supported format.
func Decode(path string) (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	var (
		streamer beep.StreamSeekCloser
		format   beep.Format
	)
	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(f)
	case ".wav":
		streamer, format, err = wav.Decode(f)
	case ".flac":
		streamer, format, err = flac.Decode(f)
	case ".ogg":
		streamer, format, err = vorbis.Decode(f)
	default:
		_ = f.Close()
		return nil, beep.Format{}, fmt.Errorf("unsupported format: %s", ext)
	}
	if err != nil {
		_ = f.Close()
		return nil, beep.Format{}, err
	}
	return streamer, format, nil
}

// SupportedExt reports whether the extension is playable.
func SupportedExt(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range SupportedExtensions {
		if ext == e {
			return true
		}
	}
	return false
}
