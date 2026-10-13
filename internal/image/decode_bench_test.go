package image

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"testing"
)

// PICAGO_BENCH_JPEG optionally selects a real photograph for local measurements.
func BenchmarkJPEGDisplay(b *testing.B) {
	var data []byte
	if path := os.Getenv("PICAGO_BENCH_JPEG"); path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
	} else {
		source := image.NewRGBA(image.Rect(0, 0, 6000, 4000))
		for y := 0; y < 4000; y++ {
			for x := 0; x < 6000; x++ {
				i := y*source.Stride + x*4
				source.Pix[i], source.Pix[i+1], source.Pix[i+2], source.Pix[i+3] = byte(x/8), byte(y/8), byte((x+y)/16), 255
			}
		}
		var encoded bytes.Buffer
		if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 90}); err != nil {
			b.Fatal(err)
		}
		data = encoded.Bytes()
	}
	for _, size := range []struct {
		name      string
		dimension int
	}{{"thumbnail", 128}, {"screen", 1920}, {"full", 0}} {
		b.Run(size.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				decoded, err := decodeFromReader(bytes.NewReader(data), "photo.jpg", "photo.jpg", size.dimension)
				if err != nil {
					b.Fatal(err)
				}
				decoded.Release()
			}
		})
	}
}
