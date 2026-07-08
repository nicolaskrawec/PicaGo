package app

import (
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
	"github.com/hajimehoshi/ebiten/v2/vector"

	"viewergo/internal/assets"
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

	draggingImage           bool
	draggingSlider          bool
	leftMouseDown           bool
	ignoreMouseUntilRelease bool
	lastMouseX              int
	lastMouseY              int

	windowWidth           int
	windowHeight          int
	slider                compare.Slider
	showHelp              bool
	reverseCompare        bool
	syncSliderWithImage   bool
	sliderSyncRatio       float64
	pendingSliderRestore  bool
	sliderBaseWidth       int
	sliderBaseHeight      int
	pendingViewportRebase bool
	viewportBaseWidth     int
	viewportBaseHeight    int

	borderlessMaximized      bool
	pendingInitialBorderless bool
	pendingEnterFullscreen   bool
	enterFromNativeMaximize  bool
	fullscreenOriginX        int
	fullscreenOriginY        int
	pendingResetFit          bool
	windowedPosX             int
	windowedPosY             int
	windowedWidth            int
	windowedHeight           int
	hasWindowedState         bool
	restoreClickPending      bool
	restoreClickStartX       int
	restoreClickStartY       int
	lastImageClickAt         time.Time
	lastImageClickX          int
	lastImageClickY          int
	viewAnimationActive      bool
	viewAnimationStart       time.Time
	viewAnimationLength      time.Duration
	viewAnimationDelayUntil  time.Time
	viewAnimationFrom        render.View
	viewAnimationTo          render.View
	animateInitialFit        bool
}

func Run(args []string) error {
	game := &Viewer{
		mode:                displayModeSingleA,
		windowWidth:         640,
		windowHeight:        480,
		slider:              compare.Slider{Orientation: compare.OrientationVertical},
		syncSliderWithImage: true,
		animateInitialFit:   true,
	}

	ebiten.SetWindowResizable(true)
	ebiten.SetWindowTitle("PicaGo")
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowIcon(assets.WindowIcons())

	if len(args) > 0 {
		loaded, err := imagedata.LoadFile(args[0])
		if err != nil {
			return err
		}

		game.imageA = loaded
		game.windowWidth = 1280
		game.windowHeight = 720
		ebiten.SetWindowTitle(loaded.FileName)
		ebiten.SetWindowSize(1280, 720)
		game.pendingResetFit = true
		game.pendingInitialBorderless = true
	}

	if err := ebiten.RunGame(game); err != nil {
		return err
	}
	return nil
}

func (v *Viewer) Update() error {
	if v.pendingInitialBorderless {
		v.enterFromNativeMaximize = false
		v.pendingEnterFullscreen = true
		v.pendingInitialBorderless = false
	}
	if v.pendingEnterFullscreen && !v.borderlessMaximized {
		v.enterBorderlessMaximized()
		v.pendingEnterFullscreen = false
	}
	if !v.borderlessMaximized && ebiten.IsWindowMaximized() {
		v.enterFromNativeMaximize = true
		v.pendingEnterFullscreen = true
	}

	if dropped := ebiten.DroppedFiles(); dropped != nil {
		_ = fs.WalkDir(dropped, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if loaded, err := imagedata.LoadFS(dropped, path); err == nil {
				switch {
				case v.imageA == nil:
					v.imageA = loaded
					v.mode = displayModeSingleA
					v.pendingResetFit = true
					v.animateInitialFit = true
				case v.imageB == nil:
					v.imageB = loaded
					v.mode = displayModeCompare
				default:
					v.imageB = loaded
				}
				v.draggingImage = false
				v.draggingSlider = false
				v.leftMouseDown = false
				v.restoreClickPending = false
				ebiten.SetWindowTitle(loaded.FileName)
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
	rightMousePressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	mouseX, mouseY := ebiten.CursorPosition()
	if v.ignoreMouseUntilRelease {
		if leftMousePressed || rightMousePressed {
			return nil
		}
		v.ignoreMouseUntilRelease = false
	}
	imageReady := v.imageA != nil && !v.pendingResetFit && v.view.Zoom > 0
	var imageRect stdimage.Rectangle
	if imageReady {
		imageRect = render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
	}

	if leftMousePressed && !v.leftMouseDown {
		if v.borderlessMaximized && pointInTopLeftCorner(mouseX, mouseY, 10) {
			_ = v.loadAdjacentImage(-1)
			v.leftMouseDown = true
			return nil
		}
		if v.borderlessMaximized && pointInTopRightCorner(mouseX, mouseY, v.windowWidth, 10) {
			os.Exit(0)
		}

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
				if v.borderlessMaximized {
					v.restoreWindow()
				} else {
					v.pendingEnterFullscreen = true
				}
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
		v.stopViewAnimation(false)
		v.lastMouseX = mouseX
		v.lastMouseY = mouseY
		if v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady {
			v.setDragMode(mouseX, mouseY, imageRect)
		} else {
			v.draggingImage = true
			v.draggingSlider = false
		}
	}
	if rightMousePressed {
		if v.borderlessMaximized && pointInTopLeftCorner(mouseX, mouseY, 10) {
			_ = v.loadAdjacentImage(1)
			return nil
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
		if v.borderlessMaximized && pointInTopLeftCorner(mouseX, mouseY, 10) {
			if dy > 0 {
				_ = v.loadAdjacentImage(-1)
			} else {
				_ = v.loadAdjacentImage(1)
			}
		} else {
			v.zoomAt(float64(mouseX), float64(mouseY), math.Pow(1.3, dy))
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		v.zoomAt(float64(v.windowWidth)/2, float64(v.windowHeight)/2, 1.3)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		v.zoomAt(float64(v.windowWidth)/2, float64(v.windowHeight)/2, 1/1.3)
	}

	if v.syncSliderWithImage && v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady && !v.draggingSlider && !v.pendingSliderRestore {
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
	newWidth, newHeight := screen.Bounds().Dx(), screen.Bounds().Dy()
	v.rebaseViewportForResize(newWidth, newHeight)
	v.windowWidth, v.windowHeight = newWidth, newHeight

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
		v.restoreSliderAfterResize()
		v.ensureSliderPosition()
		render.DrawCompare(screen, v.imageA, v.imageB, v.windowWidth, v.windowHeight, v.view, v.slider.Position, int(v.slider.Orientation), v.reverseCompare)
	}
	if v.borderlessMaximized {
		mouseX, mouseY := ebiten.CursorPosition()
		drawCornerHints(
			screen,
			v.windowWidth,
			pointInTopLeftCorner(mouseX, mouseY, 10),
			pointInTopRightCorner(mouseX, mouseY, v.windowWidth, 10),
		)
	}

	if v.showHelp {
		ebitenutil.DebugPrintAt(screen, v.helpText(), 10, 10)
	}
}

func drawCornerHints(screen *ebiten.Image, windowWidth int, showTopLeft, showTopRight bool) {
	const size = float32(40)
	fill := color.NRGBA{48, 48, 48, 180}
	options := &vector.DrawPathOptions{AntiAlias: true}

	if showTopLeft {
		topLeft := &vector.Path{}
		topLeft.MoveTo(0, 0)
		topLeft.LineTo(size, 0)
		topLeft.Arc(0, 0, size, 0, math.Pi/2, vector.Clockwise)
		topLeft.Close()
		options.ColorScale.Reset()
		options.ColorScale.ScaleWithColor(fill)
		vector.FillPath(screen, topLeft, &vector.FillOptions{}, options)
		ebitenutil.DebugPrintAt(screen, "<>", 8, 8)
	}

	if showTopRight {
		topRight := &vector.Path{}
		right := float32(windowWidth)
		topRight.MoveTo(right, 0)
		topRight.LineTo(right, size)
		topRight.Arc(right, 0, size, math.Pi/2, math.Pi, vector.Clockwise)
		topRight.Close()
		options.ColorScale.Reset()
		options.ColorScale.ScaleWithColor(fill)
		vector.FillPath(screen, topRight, &vector.FillOptions{}, options)
		ebitenutil.DebugPrintAt(screen, "X", windowWidth-16, 8)
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

	targetView := render.View{
		Zoom:           render.FitZoom(v.windowWidth, v.windowHeight, base.Width, base.Height),
		OffsetX:        0,
		OffsetY:        0,
		Alpha:          1,
		FlipHorizontal: v.view.FlipHorizontal,
	}
	if v.animateInitialFit {
		startView := targetView
		startView.Zoom = targetView.Zoom * 0.1
		startView.Alpha = 0
		v.startViewAnimation(startView, targetView, 500*time.Millisecond, 120*time.Millisecond)
		v.animateInitialFit = false
	} else {
		v.stopViewAnimation(true)
		v.view = targetView
		v.targetView = targetView
	}
	v.ensureSliderPosition()
}

func (v *Viewer) restoreWindow() {
	if !v.borderlessMaximized {
		return
	}

	v.prepareSliderRestore()

	targetX, targetY := v.windowedPosX, v.windowedPosY
	targetWidth, targetHeight := v.windowedWidth, v.windowedHeight
	if v.imageA != nil && v.windowWidth > 0 && v.windowHeight > 0 && v.view.Zoom > 0 {
		imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
		if imageRect.Dx() > 0 && imageRect.Dy() > 0 {
			targetWidth = imageRect.Dx()
			targetHeight = imageRect.Dy()
			targetX = v.fullscreenOriginX + imageRect.Min.X
			targetY = v.fullscreenOriginY + imageRect.Min.Y
		}
	}

	v.view.OffsetX = 0
	v.view.OffsetY = 0
	v.targetView.OffsetX = 0
	v.targetView.OffsetY = 0

	if ebiten.IsFullscreen() {
		ebiten.SetFullscreen(false)
	}
	ebiten.SetWindowDecorated(true)
	if ebiten.IsWindowMaximized() {
		ebiten.RestoreWindow()
	}
	if targetWidth > 0 && targetHeight > 0 {
		ebiten.SetWindowSize(targetWidth, targetHeight)
	}
	if v.hasWindowedState {
		if monitor := ebiten.Monitor(); monitor != nil {
			monitorWidth, monitorHeight := monitor.Size()
			if monitorWidth > 0 {
				targetX = clampInt(targetX, 0, maxInt(0, monitorWidth-targetWidth))
			}
			if monitorHeight > 0 {
				targetY = clampInt(targetY, 0, maxInt(0, monitorHeight-targetHeight))
			}
		}
		ebiten.SetWindowPosition(targetX, targetY)
	}

	v.borderlessMaximized = false
	v.enterFromNativeMaximize = false
	v.leftMouseDown = false
	v.ignoreMouseUntilRelease = true
	v.draggingImage = false
	v.draggingSlider = false
}

func (v *Viewer) enterBorderlessMaximized() {
	if v.borderlessMaximized {
		return
	}

	v.captureWindowedState()
	v.prepareSliderRestore()
	v.prepareViewportRebase()

	ebiten.SetWindowDecorated(true)
	ebiten.SetFullscreen(true)
	if v.enterFromNativeMaximize {
		v.fullscreenOriginX, v.fullscreenOriginY = ebiten.WindowPosition()
	} else {
		v.fullscreenOriginX, v.fullscreenOriginY = 0, 0
	}

	v.borderlessMaximized = true
	v.leftMouseDown = false
	v.ignoreMouseUntilRelease = true
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
	v.enterFromNativeMaximize = false
	v.pendingEnterFullscreen = true
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
	v.stopViewAnimation(false)
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

func pointInTopRightCorner(x, y, width, tolerance int) bool {
	if width <= 0 || tolerance <= 0 {
		return false
	}
	return x >= width-tolerance && y >= 0 && y < tolerance
}

func pointInTopLeftCorner(x, y, tolerance int) bool {
	if tolerance <= 0 {
		return false
	}
	return x >= 0 && x < tolerance && y >= 0 && y < tolerance
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func clampInt(value, minValue, maxValue int) int {
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func lerpFloat(a, b, t float64) float64 {
	return a + (b-a)*t
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
	v.stopViewAnimation(false)

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
	v.targetView.Alpha = 1
}

func (v *Viewer) animateView() {
	if v.viewAnimationActive {
		now := time.Now()
		if now.Before(v.viewAnimationDelayUntil) {
			v.view = v.viewAnimationFrom
			v.targetView = v.viewAnimationTo
			return
		}

		elapsed := now.Sub(v.viewAnimationStart)
		if elapsed >= v.viewAnimationLength {
			v.view = v.viewAnimationTo
			v.targetView = v.viewAnimationTo
			v.viewAnimationActive = false
			return
		}

		t := float64(elapsed) / float64(v.viewAnimationLength)
		eased := 1 - math.Pow(1-t, 3)
		v.view.Zoom = lerpFloat(v.viewAnimationFrom.Zoom, v.viewAnimationTo.Zoom, eased)
		v.view.OffsetX = lerpFloat(v.viewAnimationFrom.OffsetX, v.viewAnimationTo.OffsetX, eased)
		v.view.OffsetY = lerpFloat(v.viewAnimationFrom.OffsetY, v.viewAnimationTo.OffsetY, eased)
		v.view.Alpha = lerpFloat(v.viewAnimationFrom.Alpha, v.viewAnimationTo.Alpha, eased)
		v.view.FlipHorizontal = v.viewAnimationTo.FlipHorizontal
		v.targetView = v.viewAnimationTo
		return
	}

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

	if math.Abs(v.view.Alpha-v.targetView.Alpha) < 0.001 {
		v.view.Alpha = v.targetView.Alpha
	} else {
		v.view.Alpha += (v.targetView.Alpha - v.view.Alpha) * smoothing
	}
}

func (v *Viewer) startViewAnimation(from, to render.View, duration, delay time.Duration) {
	now := time.Now()
	v.viewAnimationActive = true
	v.viewAnimationStart = now.Add(delay)
	v.viewAnimationLength = duration
	v.viewAnimationDelayUntil = v.viewAnimationStart
	v.viewAnimationFrom = from
	v.viewAnimationTo = to
	v.view = from
	v.targetView = to
}

func (v *Viewer) stopViewAnimation(finish bool) {
	if !v.viewAnimationActive {
		return
	}
	if finish {
		v.view = v.viewAnimationTo
		v.targetView = v.viewAnimationTo
	} else {
		v.targetView = v.view
	}
	v.viewAnimationActive = false
	v.viewAnimationDelayUntil = time.Time{}
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
	if v.pendingSliderRestore {
		return
	}

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

func (v *Viewer) prepareSliderRestore() {
	if !v.syncSliderWithImage || v.mode != displayModeCompare || v.imageA == nil || v.imageB == nil || v.windowWidth <= 0 || v.windowHeight <= 0 || v.view.Zoom <= 0 {
		return
	}

	imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
	if imageRect.Empty() {
		return
	}

	v.captureSliderSyncRatio(imageRect)
	v.sliderBaseWidth = v.windowWidth
	v.sliderBaseHeight = v.windowHeight
	v.pendingSliderRestore = true
}

func (v *Viewer) restoreSliderAfterResize() {
	if !v.pendingSliderRestore || v.windowWidth <= 0 || v.windowHeight <= 0 || v.imageA == nil || v.view.Zoom <= 0 {
		return
	}
	if v.windowWidth == v.sliderBaseWidth && v.windowHeight == v.sliderBaseHeight {
		return
	}

	imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
	if imageRect.Empty() {
		return
	}

	v.applySliderSync(imageRect)
	v.pendingSliderRestore = false
}

func (v *Viewer) prepareViewportRebase() {
	if v.windowWidth <= 0 || v.windowHeight <= 0 {
		return
	}

	v.viewportBaseWidth = v.windowWidth
	v.viewportBaseHeight = v.windowHeight
	v.pendingViewportRebase = true
}

func (v *Viewer) rebaseViewportForResize(newWidth, newHeight int) {
	if !v.pendingViewportRebase || newWidth <= 0 || newHeight <= 0 || v.viewportBaseWidth <= 0 || v.viewportBaseHeight <= 0 {
		return
	}

	dx := float64(v.viewportBaseWidth-newWidth) / 2
	dy := float64(v.viewportBaseHeight-newHeight) / 2
	v.view.OffsetX += dx
	v.view.OffsetY += dy
	v.targetView.OffsetX += dx
	v.targetView.OffsetY += dy
	if v.syncSliderWithImage && v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil {
		if v.slider.Orientation == compare.OrientationHorizontal {
			v.slider.Position -= dy
		} else {
			v.slider.Position -= dx
		}
	}
	v.pendingViewportRebase = false
}

func (v *Viewer) helpText() string {
	fileA := "A"
	if v.imageA != nil && v.imageA.FileName != "" {
		fileA = v.imageA.FileName
	}

	fileB := "aucune"
	if v.imageB != nil && v.imageB.FileName != "" {
		fileB = v.imageB.FileName
	}

	syncMode := "off"
	if v.syncSliderWithImage {
		syncMode = "on"
	}

	return "PicaGo " + Version + "\nF1 aide\nH : split horizontal\nV : split vertical\nM : miroir\nL : slide sync " + syncMode + "\nR : fit\n1 : " + fileA + "\n2 : " + fileB
}
