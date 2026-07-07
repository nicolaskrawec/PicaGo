package app

import (
	"errors"
	stdimage "image"
	"image/color"
	"io/fs"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"viewergo/internal/compare"
	imagedata "viewergo/internal/image"
	"viewergo/internal/input"
	"viewergo/internal/render"
)

type displayMode int

const (
	displayModeSingleA displayMode = iota
	displayModeSingleB
	displayModeCompare
)

type Viewer struct {
	imageA *imagedata.LoadedImage
	imageB *imagedata.LoadedImage

	mode displayMode

	view render.View

	draggingImage  bool
	draggingSlider bool
	leftMouseDown  bool
	lastMouseX     int
	lastMouseY     int

	windowWidth  int
	windowHeight int
	slider       compare.Slider
	showHelp     bool

	borderlessMaximized bool
	pendingResetFit     bool
	restoreClickPending bool
	restoreClickStartX  int
	restoreClickStartY  int
}

func Run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: imageviewer <image-path>")
	}

	loaded, err := imagedata.LoadFile(args[0])
	if err != nil {
		return err
	}

	game := &Viewer{
		imageA:       loaded,
		mode:         displayModeSingleA,
		windowWidth:  1280,
		windowHeight: 720,
		slider:       compare.Slider{Orientation: compare.OrientationVertical},
	}

	ebiten.SetWindowResizable(true)
	ebiten.SetWindowTitle(loaded.FileName)
	ebiten.SetWindowSize(1280, 720)
	game.pendingResetFit = true
	game.enterBorderlessMaximized()

	if err := ebiten.RunGame(game); err != nil {
		return err
	}
	return nil
}

func (v *Viewer) Update() error {
	if dropped := ebiten.DroppedFiles(); dropped != nil && v.imageB == nil {
		_ = fs.WalkDir(dropped, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if loaded, err := imagedata.LoadFS(dropped, path); err == nil {
				v.imageB = loaded
				v.mode = displayModeCompare
				return fs.SkipAll
			}
			return nil
		})
	}

	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		os.Exit(0)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		v.resetFit()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyH) {
		v.showHelp = !v.showHelp
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyV) {
		v.toggleSliderOrientation()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		v.toggleBorderlessMaximized()
	}

	leftMousePressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	mouseX, mouseY := ebiten.CursorPosition()
	imageReady := !v.pendingResetFit && v.view.Zoom > 0
	var imageRect stdimage.Rectangle
	if imageReady {
		imageRect = render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
	}

	if leftMousePressed && !v.leftMouseDown {
		if !imageReady {
			v.leftMouseDown = true
			return nil
		}

		if v.borderlessMaximized && !pointInRect(mouseX, mouseY, imageRect) {
			v.restoreClickPending = true
			v.restoreClickStartX = mouseX
			v.restoreClickStartY = mouseY
			v.leftMouseDown = true
			return nil
		}

		v.leftMouseDown = true
		v.lastMouseX = mouseX
		v.lastMouseY = mouseY
		if v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady {
			effectiveSliderPos := v.slider.Position
			if v.slider.Orientation == compare.OrientationHorizontal {
				if effectiveSliderPos < float64(imageRect.Min.Y) {
					effectiveSliderPos = float64(imageRect.Min.Y)
				}
				if effectiveSliderPos > float64(imageRect.Max.Y) {
					effectiveSliderPos = float64(imageRect.Max.Y)
				}
				if math.Abs(float64(mouseY)-effectiveSliderPos) <= 10 {
					v.draggingSlider = true
					v.draggingImage = false
				} else {
					v.draggingImage = true
					v.draggingSlider = false
				}
			} else {
				if effectiveSliderPos < float64(imageRect.Min.X) {
					effectiveSliderPos = float64(imageRect.Min.X)
				}
				if effectiveSliderPos > float64(imageRect.Max.X) {
					effectiveSliderPos = float64(imageRect.Max.X)
				}
				if math.Abs(float64(mouseX)-effectiveSliderPos) <= 10 {
					v.draggingSlider = true
					v.draggingImage = false
				} else {
					v.draggingImage = true
					v.draggingSlider = false
				}
			}
		} else {
			v.draggingImage = true
			v.draggingSlider = false
		}
	}

	if leftMousePressed {
		if v.restoreClickPending {
			if absInt(mouseX-v.restoreClickStartX) > 4 || absInt(mouseY-v.restoreClickStartY) > 4 {
				v.restoreClickPending = false
			}
		}

		if v.draggingSlider {
			imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
			if v.slider.Orientation == compare.OrientationHorizontal {
				clampedY := mouseY
				if clampedY < imageRect.Min.Y {
					clampedY = imageRect.Min.Y
				}
				if clampedY > imageRect.Max.Y {
					clampedY = imageRect.Max.Y
				}
				v.slider.Position = float64(clampedY)
			} else {
				clampedX := mouseX
				if clampedX < imageRect.Min.X {
					clampedX = imageRect.Min.X
				}
				if clampedX > imageRect.Max.X {
					clampedX = imageRect.Max.X
				}
				v.slider.Position = float64(clampedX)
			}
		} else if v.draggingImage {
			dx := mouseX - v.lastMouseX
			dy := mouseY - v.lastMouseY
			v.view.OffsetX += float64(dx)
			v.view.OffsetY += float64(dy)
		}
		v.lastMouseX = mouseX
		v.lastMouseY = mouseY
	} else {
		if v.restoreClickPending {
			if v.borderlessMaximized && !pointInRect(mouseX, mouseY, imageRect) {
				v.restoreWindow()
			}
			v.restoreClickPending = false
		}
		v.leftMouseDown = false
		v.draggingImage = false
		v.draggingSlider = false
	}

	if dy := input.WheelDelta(); dy != 0 {
		v.zoomAt(float64(mouseX), float64(mouseY), math.Pow(1.1, dy))
	}

	if ebiten.IsKeyPressed(ebiten.KeyC) && v.imageB != nil {
		v.mode = displayModeCompare
	}
	if ebiten.IsKeyPressed(ebiten.Key1) {
		v.mode = displayModeSingleA
	}
	if ebiten.IsKeyPressed(ebiten.Key2) && v.imageB != nil {
		v.mode = displayModeSingleB
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) && v.imageB != nil {
		v.mode = displayModeCompare
	}

	return nil
}

func (v *Viewer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{24, 24, 28, 255})
	v.windowWidth, v.windowHeight = screen.Bounds().Dx(), screen.Bounds().Dy()

	if v.pendingResetFit {
		v.resetFit()
		v.pendingResetFit = false
	}

	switch v.mode {
	case displayModeSingleA:
		render.DrawImage(screen, v.imageA, v.windowWidth, v.windowHeight, v.view)
	case displayModeSingleB:
		render.DrawImage(screen, v.imageB, v.windowWidth, v.windowHeight, v.view)
	case displayModeCompare:
		v.ensureSliderPosition()
		render.DrawCompare(screen, v.imageA, v.imageB, v.windowWidth, v.windowHeight, v.view, v.slider.Position, int(v.slider.Orientation))
	}

	if v.showHelp {
		ebitenutil.DebugPrintAt(screen, "H aide  |  Shift+H horizontal  |  V vertical  |  R reset  |  C compare  |  1 A  |  2 B  |  S slide  |  click fond: retour", 10, 10)
	}
}

func (v *Viewer) Layout(outsideWidth, outsideHeight int) (int, int) {
	if v.windowWidth == 0 || v.windowHeight == 0 {
		v.windowWidth = outsideWidth
		v.windowHeight = outsideHeight
	}
	return outsideWidth, outsideHeight
}

func (v *Viewer) resetFit() {
	base := v.imageA
	if base == nil {
		return
	}

	v.view.Zoom = render.FitZoom(v.windowWidth, v.windowHeight, base.Width, base.Height)
	v.view.OffsetX = 0
	v.view.OffsetY = 0
	v.ensureSliderPosition()
}

func (v *Viewer) restoreWindow() {
	if !v.borderlessMaximized {
		return
	}

	ebiten.SetWindowDecorated(true)
	if ebiten.IsWindowMaximized() {
		ebiten.RestoreWindow()
	}

	v.borderlessMaximized = false
	v.leftMouseDown = false
	v.draggingImage = false
	v.draggingSlider = false
}

func (v *Viewer) enterBorderlessMaximized() {
	if v.borderlessMaximized {
		return
	}

	ebiten.SetWindowDecorated(false)
	ebiten.MaximizeWindow()

	v.borderlessMaximized = true
	v.leftMouseDown = false
	v.draggingImage = false
	v.draggingSlider = false
}

func (v *Viewer) ensureSliderPosition() {
	if v.imageA == nil {
		return
	}

	imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
	if v.slider.Orientation == compare.OrientationHorizontal {
		if v.slider.Position == 0 {
			v.slider.Position = float64(imageRect.Min.Y+imageRect.Max.Y) / 2
		}
		if v.slider.Position < float64(imageRect.Min.Y) {
			v.slider.Position = float64(imageRect.Min.Y)
		}
		if v.slider.Position > float64(imageRect.Max.Y) {
			v.slider.Position = float64(imageRect.Max.Y)
		}
		return
	}

	if v.slider.Position == 0 {
		v.slider.Position = float64(imageRect.Min.X+imageRect.Max.X) / 2
	}
	if v.slider.Position < float64(imageRect.Min.X) {
		v.slider.Position = float64(imageRect.Min.X)
	}
	if v.slider.Position > float64(imageRect.Max.X) {
		v.slider.Position = float64(imageRect.Max.X)
	}
}

func (v *Viewer) setSliderOrientation(orientation compare.Orientation) {
	if v.slider.Orientation == orientation {
		return
	}
	v.slider.Orientation = orientation
	v.slider.Position = 0
	v.ensureSliderPosition()
}

func (v *Viewer) toggleSliderOrientation() {
	if v.slider.Orientation == compare.OrientationVertical {
		v.setSliderOrientation(compare.OrientationHorizontal)
		return
	}
	v.setSliderOrientation(compare.OrientationVertical)
}

func (v *Viewer) toggleBorderlessMaximized() {
	if v.borderlessMaximized {
		v.restoreWindow()
		return
	}
	v.enterBorderlessMaximized()
}

func pointInRect(x, y int, rect stdimage.Rectangle) bool {
	return x >= rect.Min.X && x < rect.Max.X && y >= rect.Min.Y && y < rect.Max.Y
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (v *Viewer) zoomAt(mouseX, mouseY, factor float64) {
	base := v.imageA
	if base == nil {
		return
	}

	oldZoom := v.view.Zoom
	newZoom := math.Max(oldZoom*factor, 0.05)
	if newZoom > 32 {
		newZoom = 32
	}

	centerX := float64(v.windowWidth) / 2
	centerY := float64(v.windowHeight) / 2
	imageCenterX := float64(base.Width) / 2
	imageCenterY := float64(base.Height) / 2

	worldX := (mouseX-centerX-v.view.OffsetX)/oldZoom + imageCenterX
	worldY := (mouseY-centerY-v.view.OffsetY)/oldZoom + imageCenterY

	v.view.OffsetX = mouseX - centerX - (worldX-imageCenterX)*newZoom
	v.view.OffsetY = mouseY - centerY - (worldY-imageCenterY)*newZoom
	v.view.Zoom = newZoom
}
