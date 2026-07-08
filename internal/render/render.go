package render

import (
	stdimage "image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	imagedata "viewergo/internal/image"
)

var compareMaskShader = mustCompareMaskShader()

type View struct {
	Zoom           float64
	OffsetX        float64
	OffsetY        float64
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
	drawShadow(screen, rect)
	options := &ebiten.DrawImageOptions{}
	if view.FlipHorizontal {
		options.GeoM.Scale(-view.Zoom, view.Zoom)
		options.GeoM.Translate(float64(rect.Min.X)+float64(loaded.Width)*view.Zoom, float64(rect.Min.Y))
	} else {
		options.GeoM.Scale(view.Zoom, view.Zoom)
		options.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	}
	screen.DrawImage(loaded.GPUTexture, options)
}

func DrawCompare(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, sliderPosition float64, orientation int, reverse bool) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view)
	if imageA == nil || imageB == nil || imageB.GPUTexture == nil {
		return
	}

	rectA := ImageRect(windowWidth, windowHeight, imageA.Width, imageA.Height, view.Zoom, view.OffsetX, view.OffsetY)
	switch orientation {
	case 1:
		visibleTop := clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.Y, rectA.Max.Y-1)
		if visibleTop < rectA.Min.Y {
			return
		}
		drawMaskedCompareImage(screen, imageB, rectA, view, visibleTop, true, reverse)
		vector.FillRect(screen, float32(rectA.Min.X), float32(visibleTop)-1, float32(rectA.Max.X-rectA.Min.X), 2, color.NRGBA{230, 230, 230, 96}, true)
	default:
		visibleLeft := clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.X, rectA.Max.X-1)
		if visibleLeft < rectA.Min.X {
			return
		}
		drawMaskedCompareImage(screen, imageB, rectA, view, visibleLeft, false, reverse)
		vector.FillRect(screen, float32(visibleLeft)-1, float32(rectA.Min.Y), 2, float32(rectA.Max.Y-rectA.Min.Y), color.NRGBA{230, 230, 230, 96}, true)
	}
}

func drawMaskedCompareImage(screen *ebiten.Image, imageB *imagedata.LoadedImage, rectA stdimage.Rectangle, view View, boundary int, horizontal bool, reverse bool) {
	options := &ebiten.DrawRectShaderOptions{}
	scale := math.Min(float64(rectA.Dx())/float64(imageB.Width), float64(rectA.Dy())/float64(imageB.Height))
	drawWidth := float64(imageB.Width) * scale
	drawHeight := float64(imageB.Height) * scale
	left := float64(rectA.Min.X) + (float64(rectA.Dx())-drawWidth)/2
	top := float64(rectA.Min.Y) + (float64(rectA.Dy())-drawHeight)/2
	if view.FlipHorizontal {
		options.GeoM.Scale(-scale, scale)
		options.GeoM.Translate(left+drawWidth, top)
	} else {
		options.GeoM.Scale(scale, scale)
		options.GeoM.Translate(left, top)
	}
	options.Uniforms = map[string]any{
		"VisibleBoundary": float32(boundary),
		"Orientation":     float32(0),
		"Reverse":         float32(0),
	}
	if horizontal {
		options.Uniforms["Orientation"] = float32(1)
	}
	if reverse {
		options.Uniforms["Reverse"] = float32(1)
	}
	options.Images[0] = imageB.GPUTexture
	screen.DrawRectShader(imageB.Width, imageB.Height, compareMaskShader, options)
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

func drawShadow(screen *ebiten.Image, rect stdimage.Rectangle) {
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
			color.RGBA{0, 0, 0, layer.alpha},
			true,
		)
	}
}

func mustCompareMaskShader() *ebiten.Shader {
	shader, err := ebiten.NewShader([]byte(`//kage:unit pixels

package main

var VisibleBoundary float
var Orientation float
var Reverse float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	if Orientation < 0.5 {
		if Reverse < 0.5 {
			if dstPos.x < VisibleBoundary {
				return vec4(0.0)
			}
		} else {
			if dstPos.x > VisibleBoundary {
				return vec4(0.0)
			}
		}
	} else {
		if Reverse < 0.5 {
			if dstPos.y < VisibleBoundary {
				return vec4(0.0)
			}
		} else {
			if dstPos.y > VisibleBoundary {
				return vec4(0.0)
			}
		}
	}

	return imageSrc0UnsafeAt(srcPos)
}`))
	if err != nil {
		panic(err)
	}
	return shader
}
