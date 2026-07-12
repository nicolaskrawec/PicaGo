package render

import (
	stdimage "image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

type imageFrameShadow struct {
	img    *ebiten.Image
	w      int
	h      int
	spread int
	radius int
}

var shadowCache imageFrameShadow

func drawShadow(screen *ebiten.Image, imageWidth, imageHeight, windowWidth, windowHeight int, rect stdimage.Rectangle, view View) {
	shadowAlpha := clampAlpha(view.Alpha)
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
	shadowCache.update(baseWidth, baseHeight, spread, radius)
	if shadowCache.img == nil {
		return
	}

	imageScale := view.Zoom / fitZoom
	if imageScale <= 0 {
		return
	}
	options := &ebiten.DrawImageOptions{}
	options.Filter = ebiten.FilterLinear
	options.ColorScale.ScaleAlpha(float32(shadowAlpha))
	shadowWidth := float64(shadowCache.img.Bounds().Dx())
	shadowHeight := float64(shadowCache.img.Bounds().Dy())
	mirrorX, mirrorY := 1.0, 1.0
	if view.FlipHorizontal {
		mirrorX = -1
	}
	if view.FlipVertical {
		mirrorY = -1
	}
	if view.MirrorScaleX != 0 {
		mirrorX = view.MirrorScaleX
	}
	if view.MirrorScaleY != 0 {
		mirrorY = view.MirrorScaleY
	}
	options.GeoM.Translate(-shadowWidth/2, -shadowHeight/2)
	options.GeoM.Scale(imageScale*mirrorX, imageScale*mirrorY)
	options.GeoM.Rotate(view.RotationAngle * math.Pi / 2)
	options.GeoM.Translate(float64(rect.Min.X+rect.Max.X)/2, float64(rect.Min.Y+rect.Max.Y)/2)
	screen.DrawImage(shadowCache.img, options)
}

func (s *imageFrameShadow) update(w, h, spread, radius int) {
	if s.img != nil && s.w == w && s.h == h && s.spread == spread && s.radius == radius {
		return
	}

	s.w = w
	s.h = h
	s.spread = spread
	s.radius = radius
	if s.img != nil {
		s.img.Deallocate()
	}
	s.img = buildShadowImage(w, h, spread, radius, 0.2)
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
