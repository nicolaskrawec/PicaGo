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
	Zoom    float64
	OffsetX float64
	OffsetY float64
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
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(view.Zoom, view.Zoom)
	options.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	screen.DrawImage(loaded.GPUTexture, options)
}

func DrawCompare(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, sliderPosition float64, orientation int) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view)
	if imageA == nil || imageB == nil || imageB.GPUTexture == nil {
		return
	}

	rectA := ImageRect(windowWidth, windowHeight, imageA.Width, imageA.Height, view.Zoom, view.OffsetX, view.OffsetY)
	rectB := ImageRect(windowWidth, windowHeight, imageB.Width, imageB.Height, view.Zoom, view.OffsetX, view.OffsetY)

	switch orientation {
	case 1:
		visibleTop := int(math.Ceil(sliderPosition))
		if visibleTop < rectA.Min.Y {
			visibleTop = rectA.Min.Y
		}
		if visibleTop > rectA.Max.Y {
			visibleTop = rectA.Max.Y
		}
		if visibleTop >= rectA.Max.Y {
			return
		}

		sourceTop := int(math.Round(float64(visibleTop-rectB.Min.Y) / view.Zoom))
		if sourceTop >= imageB.Height {
			return
		}

		sourceRect := stdimage.Rect(0, sourceTop, imageB.Width, imageB.Height)
		subImage, ok := imageB.GPUTexture.SubImage(sourceRect).(*ebiten.Image)
		if !ok {
			return
		}

		options := &ebiten.DrawImageOptions{}
		options.GeoM.Scale(view.Zoom, view.Zoom)
		options.GeoM.Translate(float64(rectB.Min.X), float64(visibleTop))
		screen.DrawImage(subImage, options)

		vector.FillRect(screen, float32(rectA.Min.X), float32(visibleTop)-1, float32(rectA.Max.X-rectA.Min.X), 2, color.RGBA{255, 255, 255, 255}, true)
	default:
		visibleLeft := int(math.Ceil(sliderPosition))
		if visibleLeft < rectA.Min.X {
			visibleLeft = rectA.Min.X
		}
		if visibleLeft > rectA.Max.X {
			visibleLeft = rectA.Max.X
		}

		if visibleLeft >= rectA.Max.X {
			return
		}

		sourceLeft := int(math.Round(float64(visibleLeft-rectB.Min.X) / view.Zoom))
		if sourceLeft >= imageB.Width {
			return
		}

		sourceRect := stdimage.Rect(sourceLeft, 0, imageB.Width, imageB.Height)
		subImage, ok := imageB.GPUTexture.SubImage(sourceRect).(*ebiten.Image)
		if !ok {
			return
		}

		options := &ebiten.DrawImageOptions{}
		options.GeoM.Scale(view.Zoom, view.Zoom)
		options.GeoM.Translate(float64(visibleLeft), float64(rectB.Min.Y))
		screen.DrawImage(subImage, options)

		vector.FillRect(screen, float32(visibleLeft)-1, float32(rectA.Min.Y), 2, float32(rectA.Max.Y-rectA.Min.Y), color.RGBA{255, 255, 255, 255}, true)
	}
}
