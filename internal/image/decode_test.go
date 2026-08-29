package image

import (
	stdimage "image"
	"image/color"
	"image/jpeg"
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

	preview, err := DecodeFileForDisplay(path, 200)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Image != nil {
		t.Fatal("display preview retained the full-resolution CPU image")
	}
	if preview.Preview == nil {
		t.Fatal("display preview has no retained screen-sized image")
	}
	if got := preview.Preview.Bounds().Size(); got.X != 200 || got.Y != 100 {
		t.Fatalf("display preview size = %v, want (200,100)", got)
	}
}

func TestDecodeJPEGForDisplayUsesOriginalMetadataAndPreviewBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.JPG")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	source := stdimage.NewRGBA(stdimage.Rect(0, 0, 4000, 2000))
	for y := 0; y < source.Bounds().Dy(); y += 20 {
		for x := 0; x < source.Bounds().Dx(); x += 20 {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x / 20), G: uint8(y / 20), B: 100, A: 255})
		}
	}
	if err := jpeg.Encode(file, source, &jpeg.Options{Quality: 80}); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeFileForDisplay(path, 128)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Width != 4000 || decoded.Height != 2000 {
		t.Fatalf("metadata dimensions = %dx%d, want 4000x2000", decoded.Width, decoded.Height)
	}
	if decoded.Image != nil {
		t.Fatal("display preview retained the full-resolution CPU image")
	}
	if got := decoded.Preview.Bounds().Size(); got.X != 128 || got.Y != 64 {
		t.Fatalf("JPEG preview size = %v, want (128,64)", got)
	}
}

func TestJPEGScaleDenom(t *testing.T) {
	tests := []struct {
		longest, maxDimension, want int
	}{
		{1200, 128, 8},
		{500, 128, 2},
		{200, 128, 1},
	}
	for _, test := range tests {
		if got := jpegScaleDenom(test.longest, test.maxDimension); got != test.want {
			t.Errorf("jpegScaleDenom(%d, %d) = %d, want %d", test.longest, test.maxDimension, got, test.want)
		}
	}
}
