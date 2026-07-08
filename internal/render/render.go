package render

import (
	stdimage "image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	imagedata "viewergo/internal/image"
)

type View struct {
	Zoom           float64
	OffsetX        float64
	OffsetY        float64
	Alpha          float64
	FlipHorizontal bool
}

func FitZoom(windowWidth, windowHeight, imageWidth, imageHeight int) float64 {
	if windowWidth <= 0 || windowHeight <= 0 || imageWidth <= 0 || imageHeight <= 0 {
		return 1
	}

	fitX := float64(windowWidth) / float64(imageWidth)
	fitY := float64(windowHeight) / float64(imageHeight)
	return math.Min(fitX, fitY)
}

func ImageRect(windowWidth, windowHeight, imageWidth, imageHeight int, zoom, offsetX, offsetY float64) stdimage.Rectangle {
	drawWidth := float64(imageWidth) * zoom
	drawHeight := float64(imageHeight) * zoom
	left := float64(windowWidth)/2 - drawWidth/2 + offsetX
	top := float64(windowHeight)/2 - drawHeight/2 + offsetY
	return stdimage.Rect(
		int(math.Round(left)),
		int(math.Round(top)),
		int(math.Round(left+drawWidth)),
		int(math.Round(top+drawHeight)),
	)
}

func DrawImage(screen *ebiten.Image, loaded *imagedata.LoadedImage, windowWidth, windowHeight int, view View) {
	if loaded == nil || loaded.GPUTexture == nil {
		return
	}

	rect := ImageRect(windowWidth, windowHeight, loaded.Width, loaded.Height, view.Zoom, view.OffsetX, view.OffsetY)
	drawShadow(screen, rect, view.Alpha)
	drawTexture(screen, loaded.GPUTexture, rect, view.FlipHorizontal, view.Alpha)
}

func DrawCompare(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, sliderPosition float64, orientation int, reverse bool) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view)
	if imageA == nil || imageB == nil || imageB.GPUTexture == nil {
		return
	}

	rectA := ImageRect(windowWidth, windowHeight, imageA.Width, imageA.Height, view.Zoom, view.OffsetX, view.OffsetY)
	rectB := compareRect(rectA, imageB.Width, imageB.Height)
	switch orientation {
	case 1:
		visibleTop := clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.Y, rectA.Max.Y-1)
		if visibleTop < rectA.Min.Y {
			return
		}
		drawClippedCompareImage(screen, imageB.GPUTexture, rectB, visibleTop, true, reverse, view.FlipHorizontal, view.Alpha)
		vector.FillRect(screen, float32(rectA.Min.X), float32(visibleTop)-1, float32(rectA.Max.X-rectA.Min.X), 2, color.NRGBA{230, 230, 230, 96}, true)
	default:
		visibleLeft := clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.X, rectA.Max.X-1)
		if visibleLeft < rectA.Min.X {
			return
		}
		drawClippedCompareImage(screen, imageB.GPUTexture, rectB, visibleLeft, false, reverse, view.FlipHorizontal, view.Alpha)
		vector.FillRect(screen, float32(visibleLeft)-1, float32(rectA.Min.Y), 2, float32(rectA.Max.Y-rectA.Min.Y), color.NRGBA{230, 230, 230, 96}, true)
	}
}

func compareRect(baseRect stdimage.Rectangle, imageWidth, imageHeight int) stdimage.Rectangle {
	scale := math.Min(float64(baseRect.Dx())/float64(imageWidth), float64(baseRect.Dy())/float64(imageHeight))
	drawWidth := float64(imageWidth) * scale
	drawHeight := float64(imageHeight) * scale
	left := float64(baseRect.Min.X) + (float64(baseRect.Dx())-drawWidth)/2
	top := float64(baseRect.Min.Y) + (float64(baseRect.Dy())-drawHeight)/2
	return stdimage.Rect(
		int(math.Round(left)),
		int(math.Round(top)),
		int(math.Round(left+drawWidth)),
		int(math.Round(top+drawHeight)),
	)
}

func drawClippedCompareImage(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, boundary int, horizontal bool, reverse bool, flipHorizontal bool, alpha float64) {
	visibleRect := destRect
	if horizontal {
		if reverse {
			visibleRect.Max.Y = minInt(visibleRect.Max.Y, boundary)
		} else {
			visibleRect.Min.Y = maxInt(visibleRect.Min.Y, boundary)
		}
	} else {
		if reverse {
			visibleRect.Max.X = minInt(visibleRect.Max.X, boundary)
		} else {
			visibleRect.Min.X = maxInt(visibleRect.Min.X, boundary)
		}
	}

	if visibleRect.Empty() {
		return
	}

	drawTextureClipped(screen, texture, destRect, visibleRect, flipHorizontal, alpha)
}

func drawTexture(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, flipHorizontal bool, alpha float64) {
	if texture == nil || destRect.Empty() {
		return
	}

	options := &ebiten.DrawImageOptions{}
	options.Filter = ebiten.FilterLinear
	options.ColorScale.ScaleAlpha(float32(clampAlpha(alpha)))
	scaleX := float64(destRect.Dx()) / float64(texture.Bounds().Dx())
	scaleY := float64(destRect.Dy()) / float64(texture.Bounds().Dy())
	if flipHorizontal {
		options.GeoM.Scale(-scaleX, scaleY)
		options.GeoM.Translate(float64(destRect.Max.X), float64(destRect.Min.Y))
	} else {
		options.GeoM.Scale(scaleX, scaleY)
		options.GeoM.Translate(float64(destRect.Min.X), float64(destRect.Min.Y))
	}

	screen.DrawImage(texture, options)
}

func drawTextureClipped(screen *ebiten.Image, texture *ebiten.Image, destRect, visibleRect stdimage.Rectangle, flipHorizontal bool, alpha float64) {
	if texture == nil || destRect.Empty() || visibleRect.Empty() {
		return
	}

	texBounds := texture.Bounds()
	leftSrc := mapRange(visibleRect.Min.X, destRect.Min.X, destRect.Max.X, texBounds.Min.X, texBounds.Max.X, flipHorizontal)
	rightSrc := mapRange(visibleRect.Max.X, destRect.Min.X, destRect.Max.X, texBounds.Min.X, texBounds.Max.X, flipHorizontal)
	topSrc := mapRange(visibleRect.Min.Y, destRect.Min.Y, destRect.Max.Y, texBounds.Min.Y, texBounds.Max.Y, false)
	bottomSrc := mapRange(visibleRect.Max.Y, destRect.Min.Y, destRect.Max.Y, texBounds.Min.Y, texBounds.Max.Y, false)
	vertexAlpha := float32(clampAlpha(alpha))

	vertices := []ebiten.Vertex{
		{
			DstX:   float32(visibleRect.Min.X),
			DstY:   float32(visibleRect.Min.Y),
			SrcX:   float32(leftSrc),
			SrcY:   float32(topSrc),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Max.X),
			DstY:   float32(visibleRect.Min.Y),
			SrcX:   float32(rightSrc),
			SrcY:   float32(topSrc),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Min.X),
			DstY:   float32(visibleRect.Max.Y),
			SrcX:   float32(leftSrc),
			SrcY:   float32(bottomSrc),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Max.X),
			DstY:   float32(visibleRect.Max.Y),
			SrcX:   float32(rightSrc),
			SrcY:   float32(bottomSrc),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
	}
	indices := []uint16{0, 1, 2, 1, 2, 3}
	options := &ebiten.DrawTrianglesOptions{
		Filter: ebiten.FilterLinear,
	}
	screen.DrawTriangles(vertices, indices, texture, options)
}

func clampBoundary(value, min, max int) int {
	if value < min {
		return min
	}
	if max < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func drawShadow(screen *ebiten.Image, rect stdimage.Rectangle, alpha float64) {
	shadowAlpha := clampAlpha(alpha)
	if shadowAlpha <= 0 {
		return
	}

	shadowLayers := []struct {
		padding int
		alpha   uint8
	}{
		{padding: 14, alpha: 8},
		{padding: 10, alpha: 12},
		{padding: 6, alpha: 18},
		{padding: 3, alpha: 26},
	}

	for _, layer := range shadowLayers {
		layerRect := rect.Inset(-layer.padding)
		vector.FillRect(
			screen,
			float32(layerRect.Min.X),
			float32(layerRect.Min.Y),
			float32(layerRect.Max.X-layerRect.Min.X),
			float32(layerRect.Max.Y-layerRect.Min.Y),
			color.RGBA{0, 0, 0, uint8(float64(layer.alpha) * shadowAlpha)},
			true,
		)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mapRange(value, inMin, inMax, outMin, outMax int, reverse bool) float64 {
	if inMax == inMin {
		return float64(outMin)
	}

	t := float64(value-inMin) / float64(inMax-inMin)
	if reverse {
		t = 1 - t
	}
	return float64(outMin) + t*float64(outMax-outMin)
}

func clampAlpha(alpha float64) float64 {
	if alpha <= 0 {
		return 0
	}
	if alpha >= 1 {
		return 1
	}
	return alpha
}
