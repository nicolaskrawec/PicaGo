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

type imageFrameShadow struct {
	img    *ebiten.Image
	w      int
	h      int
	spread int
	radius int
	alpha  float32
}

var shadowCache imageFrameShadow

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

func DrawImage(screen *ebiten.Image, loaded *imagedata.LoadedImage, windowWidth, windowHeight int, view View, showShadow bool) {
	if loaded == nil || loaded.GPUTexture == nil {
		return
	}

	rect := ImageRect(windowWidth, windowHeight, loaded.Width, loaded.Height, view.Zoom, view.OffsetX, view.OffsetY)
	if showShadow {
		drawShadow(screen, loaded.Width, loaded.Height, windowWidth, windowHeight, rect, view.Alpha)
	}
	drawTexture(screen, loaded.GPUTexture, rect, view.FlipHorizontal, view.Alpha)
}

func DrawCompare(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, sliderPosition float64, orientation int, reverse bool, sliderOpacity float64, showShadow bool) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view, showShadow)
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
		if sliderOpacity > 0 {
			vector.FillRect(screen, float32(rectA.Min.X), float32(visibleTop)-1, float32(rectA.Max.X-rectA.Min.X), 2, color.NRGBA{230, 230, 230, uint8(math.Round(96 * clampAlpha(sliderOpacity)))}, true)
		}
	default:
		visibleLeft := clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.X, rectA.Max.X-1)
		if visibleLeft < rectA.Min.X {
			return
		}
		drawClippedCompareImage(screen, imageB.GPUTexture, rectB, visibleLeft, false, reverse, view.FlipHorizontal, view.Alpha)
		if sliderOpacity > 0 {
			vector.FillRect(screen, float32(visibleLeft)-1, float32(rectA.Min.Y), 2, float32(rectA.Max.Y-rectA.Min.Y), color.NRGBA{230, 230, 230, uint8(math.Round(96 * clampAlpha(sliderOpacity)))}, true)
		}
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

func drawShadow(screen *ebiten.Image, imageWidth, imageHeight, windowWidth, windowHeight int, rect stdimage.Rectangle, alpha float64) {
	shadowAlpha := clampAlpha(alpha)
	if shadowAlpha <= 0 {
		return
	}

	if imageWidth <= 0 || imageHeight <= 0 || windowWidth <= 0 || windowHeight <= 0 || rect.Empty() {
		return
	}

	fitZoom := FitZoom(windowWidth, windowHeight, imageWidth, imageHeight)
	baseRect := ImageRect(windowWidth, windowHeight, imageWidth, imageHeight, fitZoom, 0, 0)
	baseWidth := baseRect.Dx()
	baseHeight := baseRect.Dy()
	if baseWidth <= 0 || baseHeight <= 0 {
		return
	}

	spread := shadowSpread(baseWidth, baseHeight)
	radius := spread * 2
	shadowCache.update(baseWidth, baseHeight, spread, radius, float32(shadowAlpha*0.2))
	if shadowCache.img == nil {
		return
	}

	scaleX := float64(rect.Dx()) / float64(baseWidth)
	scaleY := float64(rect.Dy()) / float64(baseHeight)
	options := &ebiten.DrawImageOptions{}
	options.Filter = ebiten.FilterLinear
	options.GeoM.Scale(scaleX, scaleY)
	options.GeoM.Translate(
		float64(rect.Min.X)-float64(spread)*scaleX,
		float64(rect.Min.Y)-float64(spread)*scaleY,
	)
	screen.DrawImage(shadowCache.img, options)
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

func clampFloat32(value, minValue, maxValue float32) float32 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
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

func (s *imageFrameShadow) update(w, h, spread, radius int, alpha float32) {
	if s.img != nil && s.w == w && s.h == h && s.spread == spread && s.radius == radius && s.alpha == alpha {
		return
	}

	s.w = w
	s.h = h
	s.spread = spread
	s.radius = radius
	s.alpha = alpha
	s.img = buildShadowImage(w, h, spread, radius, alpha)
}

func buildShadowImage(w, h, spread, radius int, alpha float32) *ebiten.Image {
	if w <= 0 || h <= 0 || spread <= 0 || alpha <= 0 {
		return nil
	}

	outW := w + spread*2
	outH := h + spread*2
	rgba := stdimage.NewRGBA(stdimage.Rect(0, 0, outW, outH))

	rectX0 := float64(spread)
	rectY0 := float64(spread)
	rectX1 := float64(spread + w)
	rectY1 := float64(spread + h)
	blur := float64(spread)
	cornerRadius := float64(radius)

	for py := 0; py < outH; py++ {
		for px := 0; px < outW; px++ {
			x := float64(px) + 0.5
			y := float64(py) + 0.5

			distance := roundedRectSDF(x, y, rectX0, rectY0, rectX1, rectY1, cornerRadius)
			if distance < 0 {
				continue
			}

			t := 1.0 - clamp01(distance/blur)
			t *= t
			a := uint8(math.Round(float64(alpha) * t * 255))
			if a == 0 {
				continue
			}

			rgba.SetRGBA(px, py, color.RGBA{A: a})
		}
	}

	return ebiten.NewImageFromImage(rgba)
}

func roundedRectSDF(px, py, x0, y0, x1, y1, r float64) float64 {
	cx := (x0 + x1) * 0.5
	cy := (y0 + y1) * 0.5
	hx := (x1 - x0) * 0.5
	hy := (y1 - y0) * 0.5

	r = minFloat64(r, math.Min(hx, hy))
	qx := math.Abs(px-cx) - (hx - r)
	qy := math.Abs(py-cy) - (hy - r)
	ax := math.Max(qx, 0)
	ay := math.Max(qy, 0)

	return math.Hypot(ax, ay) + math.Min(math.Max(qx, qy), 0) - r
}

func shadowSpread(width, height int) int {
	visibleSize := math.Min(float64(width), float64(height))
	return int(math.Round(float64(clampFloat32(float32(visibleSize*0.08), 8, 500))))
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
