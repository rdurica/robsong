package library

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/dhowden/tag"
)

// ReadCover returns the embedded cover art of the audio file at path.
// A file without an embedded picture yields (nil, nil); unreadable tags or
// undecodable picture data yield (nil, err).
func ReadCover(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		return nil, fmt.Errorf("read tags %s: %w", path, err)
	}
	pic := meta.Picture()
	if pic == nil {
		return nil, nil
	}
	img, _, err := image.Decode(bytes.NewReader(pic.Data))
	if err != nil {
		return nil, fmt.Errorf("decode cover %s: %w", path, err)
	}
	return img, nil
}
