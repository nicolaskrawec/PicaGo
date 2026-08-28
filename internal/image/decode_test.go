package image

import (
	stdimage "image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeFileThumbnailBoundsMemoryImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	source := stdimage.NewRGBA(stdimage.Rect(0, 0, 400, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 400; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	thumbnail, err := DecodeFileThumbnail(path, 128)
	if err != nil {
		t.Fatal(err)
	}
	if got := thumbnail.Bounds().Size(); got.X != 128 || got.Y != 64 {
		t.Fatalf("thumbnail size = %v, want (128,64)", got)
	}
}
