package render

import (
	stdimage "image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	imagedata "viewergo/internal/image"
)

type View struct {
	Zoom           float64
	OffsetX        float64
	OffsetY        float64
	Alpha          float64
	FlipHorizontal bool
	FlipVertical   bool
	Rotation       int // clockwise quarter turns
	RotationAngle  float64
	MirrorScaleX   float64
	MirrorScaleY   float64
	Gamma          float64
	Exposure       float64
	Contrast       float64
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

func ImageRectForView(windowWidth, windowHeight, imageWidth, imageHeight int, view View) stdimage.Rectangle {
	width, height := rotatedDimensions(imageWidth, imageHeight, view)
	return ImageRect(windowWidth, windowHeight, width, height, view.Zoom, view.OffsetX, view.OffsetY)
}

func rotatedDimensions(imageWidth, imageHeight int, view View) (int, int) {
	angle := view.RotationAngle
	radians := angle * math.Pi / 2
	cosine := math.Abs(math.Cos(radians))
	sine := math.Abs(math.Sin(radians))
	return maxInt(1, int(math.Round(float64(imageWidth)*cosine+float64(imageHeight)*sine))),
		maxInt(1, int(math.Round(float64(imageWidth)*sine+float64(imageHeight)*cosine)))
}

func DrawImage(screen *ebiten.Image, loaded *imagedata.LoadedImage, windowWidth, windowHeight int, view View, showShadow bool) {
	if loaded == nil || loaded.GPUTexture == nil {
		return
	}

	rect := ImageRectForView(windowWidth, windowHeight, loaded.Width, loaded.Height, view)
	if showShadow && !loaded.HasTransparency {
		drawShadow(screen, loaded.Width, loaded.Height, windowWidth, windowHeight, rect, view)
	}
	rotationAngle := view.RotationAngle
	drawTexture(screen, loaded.GPUTexture, rect, view.FlipHorizontal, view.FlipVertical, rotationAngle, view.MirrorScaleX, view.MirrorScaleY, view.Alpha, view.Gamma, view.Exposure, view.Contrast)
}
