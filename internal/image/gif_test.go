package image

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/gif"
	"testing"
	"time"
)

func TestComposeGIFFramesHonorsDisposal(t *testing.T) {
	palette := color.Palette{color.Transparent, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}, color.RGBA{G: 255, A: 255}}
	fullRed := imageWithIndexes(stdimage.Rect(0, 0, 2, 1), palette, []uint8{1, 1})
	leftBlue := imageWithIndexes(stdimage.Rect(0, 0, 1, 1), palette, []uint8{2})
	rightGreen := imageWithIndexes(stdimage.Rect(1, 0, 2, 1), palette, []uint8{3})

	source := &gif.GIF{
		Image:    []*stdimage.Paletted{fullRed, leftBlue, rightGreen},
		Disposal: []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalNone},
		Config:   stdimage.Config{Width: 2, Height: 1},
	}
	frames := composeGIFFrames(source)
	assertRGBA(t, frames[1].At(0, 0), color.RGBA{B: 255, A: 255})
	assertRGBA(t, frames[1].At(1, 0), color.RGBA{R: 255, A: 255})
	assertRGBA(t, frames[2].At(0, 0), color.RGBA{R: 255, A: 255})
	assertRGBA(t, frames[2].At(1, 0), color.RGBA{G: 255, A: 255})

	source.Image = []*stdimage.Paletted{fullRed, rightGreen}
	source.Disposal = []byte{gif.DisposalBackground, gif.DisposalNone}
	frames = composeGIFFrames(source)
	assertRGBA(t, frames[1].At(0, 0), color.RGBA{})
}

func TestDecodeAnimatedGIFRetainsFramesAndTiming(t *testing.T) {
	palette := color.Palette{color.Black, color.White}
	animation := &gif.GIF{
		Image: []*stdimage.Paletted{
			imageWithIndexes(stdimage.Rect(0, 0, 1, 1), palette, []uint8{0}),
			imageWithIndexes(stdimage.Rect(0, 0, 1, 1), palette, []uint8{1}),
		},
		Delay:     []int{0, 7},
		LoopCount: 2,
		Config:    stdimage.Config{Width: 1, Height: 1, ColorModel: palette},
	}
	var data bytes.Buffer
	if err := gif.EncodeAll(&data, animation); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeBytes(data.Bytes(), "animated.gif", "animated.gif")
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.AnimationFrames) != 2 || decoded.AnimationLoopCount != 2 {
		t.Fatalf("unexpected animation metadata: frames=%d loops=%d", len(decoded.AnimationFrames), decoded.AnimationLoopCount)
	}
	if got := decoded.AnimationDelays; got[0] != minimumGIFFrameDelay || got[1] != 70*time.Millisecond {
		t.Fatalf("unexpected delays: %v", got)
	}
}

func imageWithIndexes(bounds stdimage.Rectangle, palette color.Palette, indexes []uint8) *stdimage.Paletted {
	result := stdimage.NewPaletted(bounds, palette)
	copy(result.Pix, indexes)
	return result
}

func assertRGBA(t *testing.T, got color.Color, want color.RGBA) {
	t.Helper()
	r, g, b, a := got.RGBA()
	actual := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	if actual != want {
		t.Fatalf("color = %#v, want %#v", actual, want)
	}
}
