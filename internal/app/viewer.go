package app

import (
	"errors"
	stdimage "image"
	"image/color"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"

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

var Version = "dev"

type Viewer struct {
	imageA *imagedata.LoadedImage
	imageB *imagedata.LoadedImage

	mode displayMode

	view       render.View
	targetView render.View

	draggingImage  bool
	draggingSlider bool
	leftMouseDown  bool
	lastMouseX     int
	lastMouseY     int

	windowWidth  int
	windowHeight int
	slider         compare.Slider
	showHelp       bool
	reverseCompare bool
	syncSliderWithImage bool
	sliderSyncRatio     float64

	borderlessMaximized bool
	pendingInitialBorderless bool
	pendingResetFit     bool
	windowedPosX        int
	windowedPosY        int
	windowedWidth       int
	windowedHeight      int
	hasWindowedState    bool
	restoreClickPending bool
	restoreClickStartX  int
	restoreClickStartY  int
	lastImageClickAt    time.Time
	lastImageClickX     int
	lastImageClickY     int
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
		imageA:               loaded,
		mode:                 displayModeSingleA,
		windowWidth:          1280,
		windowHeight:         720,
		slider:               compare.Slider{Orientation: compare.OrientationVertical},
		syncSliderWithImage:   true,
	}

	ebiten.SetWindowResizable(true)
	ebiten.SetWindowTitle(loaded.FileName)
	ebiten.SetWindowSize(1280, 720)
	game.pendingResetFit = true
	game.pendingInitialBorderless = true

	if err := ebiten.RunGame(game); err != nil {
		return err
	}
	return nil
}

func (v *Viewer) Update() error {
	if v.pendingInitialBorderless {
		v.captureWindowedState()
		v.enterBorderlessMaximized()
		v.pendingInitialBorderless = false
	}

	if dropped := ebiten.DroppedFiles(); dropped != nil {
		_ = fs.WalkDir(dropped, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if loaded, err := imagedata.LoadFS(dropped, path); err == nil {
				firstComparisonImage := v.imageB == nil
				v.imageB = loaded
				if firstComparisonImage {
					v.mode = displayModeCompare
				}
				v.draggingImage = false
				v.draggingSlider = false
				v.leftMouseDown = false
				v.restoreClickPending = false
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
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		_ = v.loadAdjacentImage(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		_ = v.loadAdjacentImage(1)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		v.showHelp = !v.showHelp
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyH) {
		v.setCompareOrientation(compare.OrientationHorizontal)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyV) {
		v.setCompareOrientation(compare.OrientationVertical)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) || inpututil.IsKeyJustPressed(ebiten.KeySemicolon) {
		v.toggleFlipHorizontal()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		v.syncSliderWithImage = !v.syncSliderWithImage
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		v.toggleBorderlessMaximized()
	}

	v.animateView()

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

		if pointInRect(mouseX, mouseY, imageRect) && !v.sliderNearCursor(mouseX, mouseY, imageRect) {
			now := time.Now()
			if !v.lastImageClickAt.IsZero() &&
				now.Sub(v.lastImageClickAt) <= 350*time.Millisecond &&
				absInt(mouseX-v.lastImageClickX) <= 4 &&
				absInt(mouseY-v.lastImageClickY) <= 4 {
				v.toggleBorderlessMaximized()
				v.lastImageClickAt = time.Time{}
				v.leftMouseDown = true
				v.draggingImage = false
				v.draggingSlider = false
				return nil
			}
			v.lastImageClickAt = now
			v.lastImageClickX = mouseX
			v.lastImageClickY = mouseY
		}

		v.leftMouseDown = true
		v.lastMouseX = mouseX
		v.lastMouseY = mouseY
		if v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady {
			v.setDragMode(mouseX, mouseY, imageRect)
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
			v.updateSliderPosition(mouseX, mouseY)
		} else if v.draggingImage {
			dx := mouseX - v.lastMouseX
			dy := mouseY - v.lastMouseY
			v.view.OffsetX += float64(dx)
			v.view.OffsetY += float64(dy)
			v.targetView.OffsetX += float64(dx)
			v.targetView.OffsetY += float64(dy)
			if v.syncSliderWithImage && v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil {
				if v.slider.Orientation == compare.OrientationHorizontal {
					v.slider.Position += float64(dy)
				} else {
					v.slider.Position += float64(dx)
				}
			}
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
		v.zoomAt(float64(mouseX), float64(mouseY), math.Pow(1.3, dy))
	}

	if v.syncSliderWithImage && v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady && !v.draggingSlider {
		syncRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
		v.applySliderSync(syncRect)
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

	v.updateCursorShape(mouseX, mouseY, imageRect, imageReady)

	return nil
}

func (v *Viewer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{127, 127, 127, 255})
	v.windowWidth, v.windowHeight = screen.Bounds().Dx(), screen.Bounds().Dy()

	if v.pendingResetFit {
		if v.borderlessMaximized && v.hasWindowedState && v.windowWidth == v.windowedWidth && v.windowHeight == v.windowedHeight {
			return
		}
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
		render.DrawCompare(screen, v.imageA, v.imageB, v.windowWidth, v.windowHeight, v.view, v.slider.Position, int(v.slider.Orientation), v.reverseCompare)
	}

	if v.showHelp {
		ebitenutil.DebugPrintAt(screen, v.helpText(), 10, 10)
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
	v.targetView = v.view
	v.ensureSliderPosition()
}

func (v *Viewer) restoreWindow() {
	if !v.borderlessMaximized {
		return
	}

	targetX, targetY := v.windowedPosX, v.windowedPosY
	targetWidth, targetHeight := v.windowedWidth, v.windowedHeight
	if v.imageA != nil && v.windowWidth > 0 && v.windowHeight > 0 && v.view.Zoom > 0 {
		imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
		if imageRect.Dx() > 0 && imageRect.Dy() > 0 {
			targetWidth = imageRect.Dx()
			targetHeight = imageRect.Dy()
		}
	}

	v.view.OffsetX = 0
	v.view.OffsetY = 0
	v.targetView.OffsetX = 0
	v.targetView.OffsetY = 0

	ebiten.SetWindowDecorated(true)
	if ebiten.IsWindowMaximized() {
		ebiten.RestoreWindow()
	}
	if targetWidth > 0 && targetHeight > 0 {
		ebiten.SetWindowSize(targetWidth, targetHeight)
	}
	if v.hasWindowedState {
		ebiten.SetWindowPosition(targetX, targetY)
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

	v.captureWindowedState()

	ebiten.SetWindowDecorated(false)
	ebiten.MaximizeWindow()

	v.borderlessMaximized = true
	v.leftMouseDown = false
	v.draggingImage = false
	v.draggingSlider = false
}

func (v *Viewer) captureWindowedState() {
	if v.borderlessMaximized {
		return
	}

	x, y := ebiten.WindowPosition()
	w, h := ebiten.WindowSize()
	if w > 0 && h > 0 {
		v.windowedPosX = x
		v.windowedPosY = y
		v.windowedWidth = w
		v.windowedHeight = h
		v.hasWindowedState = true
	}
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
		v.slider.Position = clampSliderPosition(v.slider.Position, float64(imageRect.Min.Y), float64(imageRect.Max.Y-1))
		v.captureSliderSyncRatio(imageRect)
		return
	}

	if v.slider.Position == 0 {
		v.slider.Position = float64(imageRect.Min.X+imageRect.Max.X) / 2
	}
	v.slider.Position = clampSliderPosition(v.slider.Position, float64(imageRect.Min.X), float64(imageRect.Max.X-1))
	v.captureSliderSyncRatio(imageRect)
}

func (v *Viewer) setSliderOrientation(orientation compare.Orientation) {
	if v.slider.Orientation == orientation {
		return
	}
	v.slider.Orientation = orientation
	v.slider.Position = 0
	v.ensureSliderPosition()
}

func (v *Viewer) setCompareOrientation(orientation compare.Orientation) {
	if v.slider.Orientation == orientation {
		v.reverseCompare = !v.reverseCompare
		return
	}

	v.setSliderOrientation(orientation)
	v.reverseCompare = false
}

func (v *Viewer) toggleFlipHorizontal() {
	v.view.FlipHorizontal = !v.view.FlipHorizontal
	v.targetView.FlipHorizontal = v.view.FlipHorizontal
}

func (v *Viewer) toggleBorderlessMaximized() {
	if v.borderlessMaximized {
		v.restoreWindow()
		return
	}
	v.enterBorderlessMaximized()
}

func (v *Viewer) loadAdjacentImage(step int) error {
	if v.imageA == nil || v.imageA.FilePath == "" || step == 0 {
		return nil
	}

	dir := filepath.Dir(v.imageA.FilePath)
	currentName := filepath.Base(v.imageA.FilePath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	images := make([]string, 0, len(entries))
	currentIndex := -1
	for _, entry := range entries {
		if entry.IsDir() || !imagedata.IsSupportedFile(entry.Name()) {
			continue
		}
		images = append(images, entry.Name())
		if entry.Name() == currentName {
			currentIndex = len(images) - 1
		}
	}

	nextIndex := currentIndex + step
	if currentIndex < 0 || nextIndex < 0 || nextIndex >= len(images) {
		return nil
	}

	loaded, err := imagedata.LoadFile(filepath.Join(dir, images[nextIndex]))
	if err != nil {
		return err
	}

	v.imageA = loaded
	v.view = render.View{}
	v.targetView = v.view
	v.draggingImage = false
	v.draggingSlider = false
	v.leftMouseDown = false
	v.restoreClickPending = false
	v.pendingResetFit = true
	ebiten.SetWindowTitle(loaded.FileName)
	return nil
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

func clampSliderPosition(position, minPosition, maxPosition float64) float64 {
	if maxPosition < minPosition {
		maxPosition = minPosition
	}
	if position < minPosition {
		return minPosition
	}
	if position > maxPosition {
		return maxPosition
	}
	return position
}

func (v *Viewer) setDragMode(mouseX, mouseY int, imageRect stdimage.Rectangle) {
	effectiveSliderPos := v.slider.Position
	if v.slider.Orientation == compare.OrientationHorizontal {
		effectiveSliderPos = clampSliderPosition(effectiveSliderPos, float64(imageRect.Min.Y), float64(imageRect.Max.Y))
		if math.Abs(float64(mouseY)-effectiveSliderPos) <= 15 {
			v.draggingSlider = true
			v.draggingImage = false
			return
		}
	} else {
		effectiveSliderPos = clampSliderPosition(effectiveSliderPos, float64(imageRect.Min.X), float64(imageRect.Max.X))
		if math.Abs(float64(mouseX)-effectiveSliderPos) <= 15 {
			v.draggingSlider = true
			v.draggingImage = false
			return
		}
	}

	v.draggingImage = true
	v.draggingSlider = false
}

func (v *Viewer) updateSliderPosition(mouseX, mouseY int) {
	imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
	if v.slider.Orientation == compare.OrientationHorizontal {
		v.slider.Position = clampSliderPosition(float64(mouseY), float64(imageRect.Min.Y), float64(imageRect.Max.Y-1))
		v.captureSliderSyncRatio(imageRect)
		return
	}

	v.slider.Position = clampSliderPosition(float64(mouseX), float64(imageRect.Min.X), float64(imageRect.Max.X-1))
	v.captureSliderSyncRatio(imageRect)
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

	v.targetView.OffsetX = mouseX - centerX - (worldX-imageCenterX)*newZoom
	v.targetView.OffsetY = mouseY - centerY - (worldY-imageCenterY)*newZoom
	v.targetView.Zoom = newZoom
}

func (v *Viewer) animateView() {
	const smoothing = 0.2

	if math.Abs(v.view.Zoom-v.targetView.Zoom) < 0.001 {
		v.view.Zoom = v.targetView.Zoom
	} else {
		v.view.Zoom += (v.targetView.Zoom - v.view.Zoom) * smoothing
	}

	if math.Abs(v.view.OffsetX-v.targetView.OffsetX) < 0.001 {
		v.view.OffsetX = v.targetView.OffsetX
	} else {
		v.view.OffsetX += (v.targetView.OffsetX - v.view.OffsetX) * smoothing
	}

	if math.Abs(v.view.OffsetY-v.targetView.OffsetY) < 0.001 {
		v.view.OffsetY = v.targetView.OffsetY
	} else {
		v.view.OffsetY += (v.targetView.OffsetY - v.view.OffsetY) * smoothing
	}
}

func (v *Viewer) updateCursorShape(mouseX, mouseY int, imageRect stdimage.Rectangle, imageReady bool) {
	if v.mode != displayModeCompare || v.imageA == nil || v.imageB == nil || !imageReady {
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
		return
	}

	if v.draggingSlider || v.sliderNearCursor(mouseX, mouseY, imageRect) {
		if v.slider.Orientation == compare.OrientationHorizontal {
			ebiten.SetCursorShape(ebiten.CursorShapeNSResize)
		} else {
			ebiten.SetCursorShape(ebiten.CursorShapeEWResize)
		}
		return
	}

	ebiten.SetCursorShape(ebiten.CursorShapeDefault)
}

func (v *Viewer) sliderNearCursor(mouseX, mouseY int, imageRect stdimage.Rectangle) bool {
	if v.slider.Orientation == compare.OrientationHorizontal {
		effectiveSliderPos := clampSliderPosition(v.slider.Position, float64(imageRect.Min.Y), float64(imageRect.Max.Y))
		return math.Abs(float64(mouseY)-effectiveSliderPos) <= 15
	}

	effectiveSliderPos := clampSliderPosition(v.slider.Position, float64(imageRect.Min.X), float64(imageRect.Max.X))
	return math.Abs(float64(mouseX)-effectiveSliderPos) <= 15
}

func (v *Viewer) captureSliderSyncRatio(imageRect stdimage.Rectangle) {
	if imageRect.Empty() {
		return
	}

	if v.slider.Orientation == compare.OrientationHorizontal {
		span := float64(imageRect.Max.Y - imageRect.Min.Y - 1)
		if span <= 0 {
			v.sliderSyncRatio = 0.5
			return
		}
		v.sliderSyncRatio = (clampSliderPosition(v.slider.Position, float64(imageRect.Min.Y), float64(imageRect.Max.Y-1)) - float64(imageRect.Min.Y)) / span
		return
	}

	span := float64(imageRect.Max.X - imageRect.Min.X - 1)
	if span <= 0 {
		v.sliderSyncRatio = 0.5
		return
	}
	v.sliderSyncRatio = (clampSliderPosition(v.slider.Position, float64(imageRect.Min.X), float64(imageRect.Max.X-1)) - float64(imageRect.Min.X)) / span
}

func (v *Viewer) applySliderSync(imageRect stdimage.Rectangle) {
	if imageRect.Empty() {
		return
	}

	if v.slider.Orientation == compare.OrientationHorizontal {
		span := float64(imageRect.Max.Y - imageRect.Min.Y - 1)
		if span <= 0 {
			return
		}
		v.slider.Position = float64(imageRect.Min.Y) + clampSliderPosition(v.sliderSyncRatio, 0, 1)*span
		return
	}

	span := float64(imageRect.Max.X - imageRect.Min.X - 1)
	if span <= 0 {
		return
	}
	v.slider.Position = float64(imageRect.Min.X) + clampSliderPosition(v.sliderSyncRatio, 0, 1)*span
}

func (v *Viewer) helpText() string {
	fileA := "A"
	if v.imageA != nil && v.imageA.FileName != "" {
		fileA = v.imageA.FileName
	}

	fileB := "B"
	if v.imageB != nil && v.imageB.FileName != "" {
		fileB = v.imageB.FileName
	}

	syncMode := "off"
	if v.syncSliderWithImage {
		syncMode = "on"
	}

	return "PicaGo " + Version + "\nF1 aide\nH horizontal\nV vertical\nreappui: inverse\nM mirror\nL slide sync: " + syncMode + "\nR fit\nC compare\n1 " + fileA + "\n2 " + fileB + "\nS slide\nclick fond: retour"
}
