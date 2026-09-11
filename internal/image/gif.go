package image

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
	"time"
)

const minimumGIFFrameDelay = 10 * time.Millisecond

func decodeGIF(reader io.Reader, fileName, filePath string, maxDimension int, exif []EXIFTag) (*DecodedImage, error) {
	decoded, err := gif.DecodeAll(reader)
	if err != nil {
		return nil, err
	}
	width, height := decoded.Config.Width, decoded.Config.Height
	if width <= 0 || height <= 0 || len(decoded.Image) == 0 {
		return nil, ErrTooManyPixels
	}
	framePixels := int64(width) * int64(height)
	if framePixels > maxImagePixels || int64(len(decoded.Image)) > maxImagePixels/framePixels {
		return nil, ErrTooManyPixels
	}

	frames := composeGIFFrames(decoded)
	delays := make([]time.Duration, len(frames))
	for i := range delays {
		delay := time.Duration(decoded.Delay[i]) * 10 * time.Millisecond
		if delay < minimumGIFFrameDelay {
			delay = minimumGIFFrameDelay
		}
		delays[i] = delay
	}

	result := &DecodedImage{
		FileName: fileName, FilePath: filePath, DecodeMethod: "image/gif",
		Width: width, Height: height, HasTransparency: true, EXIF: exif,
		Image: frames[0], AnimationFrames: frames,
		AnimationDelays: delays, AnimationLoopCount: decoded.LoopCount,
	}
	if maxDimension > 0 {
		result.AnimationPreviewFrames = make([]image.Image, len(frames))
		for i, frame := range frames {
			result.AnimationPreviewFrames[i] = makePreview(frame, maxDimension)
		}
		result.Preview = result.AnimationPreviewFrames[0]
	}
	return result, nil
}

func composeGIFFrames(source *gif.GIF) []image.Image {
	bounds := image.Rect(0, 0, source.Config.Width, source.Config.Height)
	canvas := image.NewRGBA(bounds)
	frames := make([]image.Image, len(source.Image))

	for i, frame := range source.Image {
		var previous *image.RGBA
		if gifDisposal(source, i) == gif.DisposalPrevious {
			previous = cloneRGBA(canvas)
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		frames[i] = cloneRGBA(canvas)

		switch gifDisposal(source, i) {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), &image.Uniform{C: color.Transparent}, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = previous
		}
	}
	return frames
}

func gifDisposal(source *gif.GIF, frame int) byte {
	if frame < len(source.Disposal) {
		return source.Disposal[frame]
	}
	return gif.DisposalNone
}

func cloneRGBA(source *image.RGBA) *image.RGBA {
	clone := image.NewRGBA(source.Bounds())
	copy(clone.Pix, source.Pix)
	return clone
}
