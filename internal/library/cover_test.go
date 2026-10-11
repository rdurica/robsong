package library

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhowden/tag"
)

const (
	coverTestW = 7
	coverTestH = 5
)

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, coverTestW, coverTestH))
	for y := 0; y < coverTestH; y++ {
		for x := 0; x < coverTestW; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 40), B: 128, A: 255})
		}
	}
	return img
}

func encodeJPEG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, testImage(), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, testImage()); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func id3Frame(id string, body []byte) []byte {
	out := []byte(id)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(body)))
	out = append(out, size...)
	out = append(out, 0, 0) // flags
	return append(out, body...)
}

func syncsafe(n int) []byte {
	return []byte{byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)}
}

// buildMP3 builds a minimal ID3v2.3 tag (TIT2 + optional APIC) followed by fake audio bytes.
func buildMP3(mime string, pic []byte) []byte {
	frames := id3Frame("TIT2", append([]byte{0}, []byte("Test Title")...))
	if pic != nil {
		body := []byte{0}
		body = append(body, []byte(mime)...)
		body = append(body, 0, 3, 0) // mime terminator, picture type (front cover), empty description
		body = append(body, pic...)
		frames = append(frames, id3Frame("APIC", body)...)
	}
	out := []byte{'I', 'D', '3', 3, 0, 0}
	out = append(out, syncsafe(len(frames))...)
	out = append(out, frames...)
	// fake MPEG frame sync + padding
	return append(out, 0xFF, 0xFB, 0x90, 0x00, 0, 0, 0, 0, 0, 0, 0, 0)
}

// writeFixture writes the file and sanity-checks that dhowden/tag parses it as intended.
func writeFixture(t *testing.T, name, mime string, pic []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, buildMP3(mime, pic), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		t.Fatalf("fixture invalid: tag.ReadFrom: %v", err)
	}
	if m.Title() != "Test Title" {
		t.Fatalf("fixture invalid: title = %q", m.Title())
	}
	p := m.Picture()
	if pic == nil && p != nil {
		t.Fatal("fixture invalid: unexpected picture")
	}
	if pic != nil && (p == nil || !bytes.Equal(p.Data, pic)) {
		t.Fatal("fixture invalid: picture not parsed from tag")
	}
	return path
}

func TestReadCoverJPEG(t *testing.T) {
	path := writeFixture(t, "jpeg.mp3", "image/jpeg", encodeJPEG(t))

	img, err := ReadCover(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img == nil {
		t.Fatal("expected image, got nil")
	}
	b := img.Bounds()
	if b.Dx() != coverTestW || b.Dy() != coverTestH {
		t.Fatalf("bounds = %dx%d, want %dx%d", b.Dx(), b.Dy(), coverTestW, coverTestH)
	}
}

func TestReadCoverPNG(t *testing.T) {
	path := writeFixture(t, "png.mp3", "image/png", encodePNG(t))

	img, err := ReadCover(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img == nil {
		t.Fatal("expected image, got nil")
	}
	b := img.Bounds()
	if b.Dx() != coverTestW || b.Dy() != coverTestH {
		t.Fatalf("bounds = %dx%d, want %dx%d", b.Dx(), b.Dy(), coverTestW, coverTestH)
	}
}

func TestReadCoverNoPicture(t *testing.T) {
	path := writeFixture(t, "nopic.mp3", "", nil)

	img, err := ReadCover(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img != nil {
		t.Fatalf("expected nil image, got %v", img.Bounds())
	}
}

func TestReadCoverCorruptPicture(t *testing.T) {
	path := writeFixture(t, "corrupt.mp3", "image/jpeg", []byte("this is not an image at all"))

	img, err := ReadCover(path)
	if err == nil {
		t.Fatal("expected error for corrupt picture data")
	}
	if img != nil {
		t.Fatal("expected nil image for corrupt picture data")
	}
}

func TestReadCoverMissingFile(t *testing.T) {
	img, err := ReadCover(filepath.Join(t.TempDir(), "does-not-exist.mp3"))
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if img != nil {
		t.Fatal("expected nil image for nonexistent path")
	}
}
