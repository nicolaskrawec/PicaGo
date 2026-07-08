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

	rectWidth := float64(rect.Dx())
	rectHeight := float64(rect.Dy())
	if rectWidth <= 0 || rectHeight <= 0 {
		return
	}

	visibleSize := math.Min(rectWidth, rectHeight)
	spread := clampFloat32(float32(visibleSize*0.3), 8, 500)
	edgePadding := clampFloat32(spread*0.015625, 1, 2)
	const layers = 64
	const maxOpacity = 0.3

	prevOpacity := 0.0
	for i := 0; i < layers; i++ {
		t := float64(i) / float64(layers-1)
		padding := lerpFloat32(spread, edgePadding, float32(t))
		targetOpacity := t * maxOpacity
		layerAlpha := alphaStep(prevOpacity, targetOpacity) * shadowAlpha
		prevOpacity = targetOpacity
		drawRoundedShadowLayer(screen, rect, padding, 0, spread*0.9, layerAlpha)
	}
}

func drawRoundedShadowLayer(screen *ebiten.Image, rect stdimage.Rectangle, padding, offsetY, radius float32, alpha float64) {
	if alpha <= 0 {
		return
	}

	x := float32(rect.Min.X) - padding
	y := float32(rect.Min.Y) - padding + offsetY
	w := float32(rect.Dx()) + padding*2
	h := float32(rect.Dy()) + padding*2
	if w <= 0 || h <= 0 {
		return
	}

	path := roundedRectPath(x, y, w, h, radius)
	options := &vector.DrawPathOptions{
		AntiAlias: true,
	}
	options.ColorScale.Scale(0, 0, 0, float32(clampAlpha(alpha)))
	vector.FillPath(screen, path, &vector.FillOptions{}, options)
}

func roundedRectPath(x, y, width, height, radius float32) *vector.Path {
	path := &vector.Path{}
	if width <= 0 || height <= 0 {
		return path
	}

	r := minFloat32(radius, minFloat32(width/2, height/2))
	if r <= 0 {
		path.MoveTo(x, y)
		path.LineTo(x+width, y)
		path.LineTo(x+width, y+height)
		path.LineTo(x, y+height)
		path.Close()
		return path
	}

	path.MoveTo(x+r, y)
	path.LineTo(x+width-r, y)
	path.ArcTo(x+width, y, x+width, y+r, r)
	path.LineTo(x+width, y+height-r)
	path.ArcTo(x+width, y+height, x+width-r, y+height, r)
	path.LineTo(x+r, y+height)
	path.ArcTo(x, y+height, x, y+height-r, r)
	path.LineTo(x, y+r)
	path.ArcTo(x, y, x+r, y, r)
	path.Close()
	return path
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

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func clampFloat32(value, minValue, maxValue float32) float32 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func lerpFloat32(a, b, t float32) float32 {
	return a + (b-a)*t
}

func alphaStep(previous, target float64) float64 {
	if target <= previous {
		return 0
	}
	if previous >= 1 {
		return 0
	}
	return (target - previous) / (1 - previous)
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
