package image

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDecodeJPEGAppliesEXIFOrientation(t *testing.T) {
	source := stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 3))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	data := addEXIFOrientation(encoded.Bytes(), 6) // Rotate 90° clockwise.

	decoded, err := DecodeBytes(data, "oriented.jpg", "oriented.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.Image.Bounds().Size(); got.X != 3 || got.Y != 2 {
		t.Fatalf("oriented image size = %v, want (3,2)", got)
	}
	if decoded.Width != 3 || decoded.Height != 2 {
		t.Fatalf("oriented metadata dimensions = %dx%d, want 3x2", decoded.Width, decoded.Height)
	}
	if len(decoded.EXIF) != 1 || decoded.EXIF[0].Name != "Orientation" || decoded.EXIF[0].Value != "6" {
		t.Fatalf("EXIF = %#v, want Orientation: 6", decoded.EXIF)
	}

	preview, err := decodePreviewFromReader(bytes.NewReader(data), "oriented.jpg", "oriented.jpg", 100)
	if err != nil {
		t.Fatal(err)
	}
	if got := preview.Preview.Bounds().Size(); got.X != 3 || got.Y != 2 {
		t.Fatalf("oriented preview size = %v, want (3,2)", got)
	}
}

func TestEXIFImageFormat(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"photo.JPG", true},
		{"photo.jpeg", true},
		{"photo.png", true},
		{"photo.webp", true},
		{"photo.tiff", true},
		{"photo.gif", false},
		{"photo.bmp", false},
	}
	for _, test := range tests {
		if _, got := exifImageFormat(test.name); got != test.ok {
			t.Errorf("exifImageFormat(%q) supported = %v, want %v", test.name, got, test.ok)
		}
	}
}

func TestEXIFValueTextNormalizesAndBoundsValues(t *testing.T) {
	if got := exifValueText("line 1\n\tline 2"); got != "line 1 line 2" {
		t.Fatalf("normalized value = %q", got)
	}
	long := strings.Repeat("é", maxEXIFValueRunes+10)
	got := exifValueText(long)
	if utf8.RuneCountInString(got) != maxEXIFValueRunes || !strings.HasSuffix(got, "…") {
		t.Fatalf("bounded value has %d runes", utf8.RuneCountInString(got))
	}
}

func addEXIFOrientation(jpegData []byte, orientation byte) []byte {
	// Little-endian TIFF/EXIF block containing one SHORT Orientation tag.
	exif := []byte{
		'E', 'x', 'i', 'f', 0, 0,
		'I', 'I', 42, 0, 8, 0, 0, 0,
		1, 0,
		0x12, 0x01, 3, 0, 1, 0, 0, 0,
		orientation, 0, 0, 0,
		0, 0, 0, 0,
	}
	segmentLength := len(exif) + 2
	segment := []byte{0xff, 0xe1, byte(segmentLength >> 8), byte(segmentLength)}
	segment = append(segment, exif...)
	return append(append(append([]byte{}, jpegData[:2]...), segment...), jpegData[2:]...)
}

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
