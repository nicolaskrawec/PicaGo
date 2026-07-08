package assets

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/png"
)

//go:embed icon.ico
var iconICO []byte

// WindowIcons returns the decoded icon candidates used by the desktop window.
func WindowIcons() []image.Image {
	icons, err := decodeICO(iconICO)
	if err != nil {
		return nil
	}
	return icons
}

func decodeICO(data []byte) ([]image.Image, error) {
	if len(data) < 6 {
		return nil, errInvalidICO
	}
	if binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, errInvalidICO
	}

	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count <= 0 {
		return nil, errInvalidICO
	}

	icons := make([]image.Image, 0, count)
	for i := 0; i < count; i++ {
		entryOffset := 6 + i*16
		if entryOffset+16 > len(data) {
			return nil, errInvalidICO
		}

		size := int(binary.LittleEndian.Uint32(data[entryOffset+8 : entryOffset+12]))
		offset := int(binary.LittleEndian.Uint32(data[entryOffset+12 : entryOffset+16]))
		if size <= 0 || offset < 0 || offset+size > len(data) {
			return nil, errInvalidICO
		}

		img, err := decodeICOImage(data[offset : offset+size])
		if err != nil {
			return nil, err
		}
		icons = append(icons, img)
	}

	return icons, nil
}

func decodeICOImage(data []byte) (image.Image, error) {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return img, nil
	}

	return decodeDIB32(data)
}

func decodeDIB32(data []byte) (image.Image, error) {
	if len(data) < 40 {
		return nil, errInvalidICO
	}
	if binary.LittleEndian.Uint32(data[:4]) != 40 {
		return nil, errUnsupportedICO
	}

	width := int(int32(binary.LittleEndian.Uint32(data[4:8])))
	fullHeight := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	planes := binary.LittleEndian.Uint16(data[12:14])
	bitCount := binary.LittleEndian.Uint16(data[14:16])
	compression := binary.LittleEndian.Uint32(data[16:20])

	if width <= 0 || fullHeight <= 0 || fullHeight%2 != 0 || planes != 1 || bitCount != 32 || compression != 0 {
		return nil, errUnsupportedICO
	}

	height := fullHeight / 2
	pixelsOffset := 40
	pixelsSize := width * height * 4
	if pixelsOffset+pixelsSize > len(data) {
		return nil, errInvalidICO
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	pixels := data[pixelsOffset : pixelsOffset+pixelsSize]
	for y := 0; y < height; y++ {
		srcY := height - 1 - y
		for x := 0; x < width; x++ {
			src := (srcY*width + x) * 4
			dst := y*img.Stride + x*4
			b := pixels[src]
			g := pixels[src+1]
			r := pixels[src+2]
			a := pixels[src+3]
			c := color.NRGBA{R: r, G: g, B: b, A: a}
			img.Pix[dst] = c.R
			img.Pix[dst+1] = c.G
			img.Pix[dst+2] = c.B
			img.Pix[dst+3] = c.A
		}
	}

	return img, nil
}

var (
	errInvalidICO     = icoError("invalid ico data")
	errUnsupportedICO = icoError("unsupported ico format")
)

type icoError string

func (e icoError) Error() string {
	return string(e)
}
