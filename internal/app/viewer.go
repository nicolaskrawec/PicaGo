package app

import (
	stdimage "image"
	"image/color"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

type compareMaskMode int

const (
	compareMaskSplit compareMaskMode = iota
	compareMaskCircle
)

type asyncImageSlot int

const (
	asyncImageSlotA asyncImageSlot = iota
	asyncImageSlotB
)

type asyncImageResult struct {
	id          int
	slot        asyncImageSlot
	decoded     *imagedata.DecodedImage
	err         error
	resetView   bool
	animateFit  bool
	activateFit bool
}

type prefetchedImageResult struct {
	path    string
	decoded *imagedata.DecodedImage
	err     error
}

const idleFrameDelay = 500 * time.Millisecond
const compareBorderIdleDelay = 600 * time.Millisecond
const cornerCommandTolerance = 25
const cornerHintAlpha = 50
const defaultCircleMaskDiameterRatio = 0.1
const prefetchedImageCacheLimit = 6

var Version = "dev"

func windowTitle(imageName string) string {
	title := "PicaGo v" + Version
	if imageName == "" {
		return title
	}
	return title + " - " + imageName
}

type Viewer struct {
	imageA *imagedata.LoadedImage
	imageB *imagedata.LoadedImage

	mode displayMode

	view       render.View
	targetView render.View

	draggingImage           bool
	draggingSlider          bool
	sliderDragMinPosition   float64
	sliderDragMaxPosition   float64
	leftMouseDown           bool
	ignoreMouseUntilRelease bool
	lastMouseX              int
	lastMouseY              int

	windowWidth           int
	windowHeight          int
	slider                compare.Slider
	sliderInitialized     bool
	sliderOpacity         float64
	circleBorderOpacity   float64
	showShadow            bool
	showBlur              bool
	showHelp              bool
	compareMask           compareMaskMode
	circleMaskDiameter    float64
	lastCompareBorderAt   time.Time
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
	lastActivityAt           time.Time
	idleFPSMode              bool
	idleMouseTracked         bool
	idleMouseX               int
	idleMouseY               int
	viewAnimationActive      bool
	viewAnimationStart       time.Time
	viewAnimationLength      time.Duration
	viewAnimationDelayUntil  time.Time
	viewAnimationFrom        render.View
	viewAnimationTo          render.View
	animateInitialFit        bool
	nextImageLoadID          int
	pendingImageLoadAID      int
	pendingImageLoadBID      int
	imageLoadResults         chan asyncImageResult
	prefetchResults          chan prefetchedImageResult
	prefetchInFlight         map[string]bool
	prefetchedImages         map[string]*imagedata.DecodedImage
	prefetchOrder            []string
	loadingImageName         string
	loadError                string
}

func Run(args []string) error {
	game := &Viewer{
		mode:                displayModeSingleA,
		windowWidth:         640,
		windowHeight:        480,
		slider:              compare.Slider{Orientation: compare.OrientationVertical},
		circleMaskDiameter:  defaultCircleMaskDiameterRatio,
		lastCompareBorderAt: time.Now(),
		lastActivityAt:      time.Now(),
		showShadow:          true,
		syncSliderWithImage: true,
		animateInitialFit:   true,
		imageLoadResults:    make(chan asyncImageResult, 4),
		prefetchResults:     make(chan prefetchedImageResult, 4),
		prefetchInFlight:    make(map[string]bool),
		prefetchedImages:    make(map[string]*imagedata.DecodedImage),
	}

	ebiten.SetWindowResizable(true)
	ebiten.SetWindowTitle(windowTitle(""))
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowIcon(assets.WindowIcons())

	if len(args) > 0 {
		game.windowWidth = 1280
		game.windowHeight = 720
		ebiten.SetWindowTitle(windowTitle(filepath.Base(args[0]) + " loading..."))
		ebiten.SetWindowSize(1280, 720)
		game.pendingInitialBorderless = true
		game.startAsyncImageFileLoad(args[0], asyncImageSlotA, true, true)
	}

	if err := ebiten.RunGame(game); err != nil {
		return err
	}
	return nil
}

func (v *Viewer) startAsyncImageFileLoad(path string, slot asyncImageSlot, resetView, animateFit bool) {
	v.nextImageLoadID++
	id := v.nextImageLoadID
	v.trackPendingImageLoad(slot, id)
	v.loadingImageName = filepath.Base(path)
	v.loadError = ""
	ebiten.SetWindowTitle(windowTitle(v.loadingImageName + " loading..."))

	go func() {
		decoded, err := imagedata.DecodeFile(path)
		v.imageLoadResults <- asyncImageResult{
			id:          id,
			slot:        slot,
			decoded:     decoded,
			err:         err,
			resetView:   resetView,
			animateFit:  animateFit,
			activateFit: true,
		}
	}()
}

func (v *Viewer) startAsyncImageFSLoad(fsys fs.FS, path string, slot asyncImageSlot, resetView, animateFit bool) {
	v.nextImageLoadID++
	id := v.nextImageLoadID
	v.trackPendingImageLoad(slot, id)
	v.loadingImageName = filepath.Base(path)
	v.loadError = ""
	ebiten.SetWindowTitle(windowTitle(v.loadingImageName + " loading..."))

	go func() {
		decoded, err := imagedata.DecodeFS(fsys, path)
		v.imageLoadResults <- asyncImageResult{
			id:          id,
			slot:        slot,
			decoded:     decoded,
			err:         err,
			resetView:   resetView,
			animateFit:  animateFit,
			activateFit: true,
		}
	}()
}

func (v *Viewer) trackPendingImageLoad(slot asyncImageSlot, id int) {
	switch slot {
	case asyncImageSlotA:
		v.pendingImageLoadAID = id
	case asyncImageSlotB:
		v.pendingImageLoadBID = id
	}
}

func (v *Viewer) collectAsyncImageLoads() {
	for {
		select {
		case result := <-v.imageLoadResults:
			v.applyAsyncImageLoad(result)
		default:
			return
		}
	}
}

func (v *Viewer) collectPrefetchedImages() {
	for {
		select {
		case result := <-v.prefetchResults:
			delete(v.prefetchInFlight, result.path)
			if result.err == nil && result.decoded != nil {
				v.cachePrefetchedImage(result.path, result.decoded)
			}
		default:
			return
		}
	}
}

func (v *Viewer) applyAsyncImageLoad(result asyncImageResult) {
	if !v.isCurrentImageLoad(result.slot, result.id) {
		return
	}
	v.trackPendingImageLoad(result.slot, 0)
	v.loadingImageName = ""

	if result.err != nil {
		v.loadError = result.err.Error()
		ebiten.SetWindowTitle(windowTitle("load failed"))
		return
	}

	v.applyDecodedImage(result.slot, result.decoded, result.resetView, result.animateFit, result.activateFit)
}

func (v *Viewer) applyDecodedImage(slot asyncImageSlot, decoded *imagedata.DecodedImage, resetView, animateFit, activateFit bool) {
	loaded := imagedata.NewLoadedImage(decoded)
	if loaded == nil {
		v.loadError = "image load failed"
		ebiten.SetWindowTitle(windowTitle("load failed"))
		return
	}

	v.loadError = ""
	if resetView {
		v.view = render.View{}
		v.targetView = v.view
		v.stopViewAnimation(false)
	}

	switch slot {
	case asyncImageSlotA:
		v.imageA = loaded
		if v.imageB == nil {
			v.mode = displayModeSingleA
		}
		v.circleMaskDiameter = defaultCircleMaskDiameterRatio
		if activateFit {
			v.pendingResetFit = true
			v.animateInitialFit = animateFit
		}
	case asyncImageSlotB:
		v.imageB = loaded
		if v.imageA != nil {
			v.mode = displayModeCompare
		}
	}

	v.draggingImage = false
	v.draggingSlider = false
	v.leftMouseDown = false
	v.restoreClickPending = false
	ebiten.SetWindowTitle(windowTitle(loaded.FileName))
	v.prefetchAdjacentImages()
}

func (v *Viewer) isCurrentImageLoad(slot asyncImageSlot, id int) bool {
	switch slot {
	case asyncImageSlotA:
		return id != 0 && id == v.pendingImageLoadAID
	case asyncImageSlotB:
		return id != 0 && id == v.pendingImageLoadBID
	default:
		return false
	}
}

func (v *Viewer) Update() error {
	now := time.Now()
	v.collectAsyncImageLoads()
	v.collectPrefetchedImages()

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
			if imagedata.IsSupportedFile(path) {
				if v.imageA == nil {
					v.startAsyncImageFSLoad(dropped, path, asyncImageSlotA, true, true)
				} else {
					v.startAsyncImageFSLoad(dropped, path, asyncImageSlotB, false, false)
				}
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
	// On AZERTY, the physical key labelled Z can be reported as KeyW by
	// Ebiten/GLFW (the key constants follow the US layout).
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		v.toggleZoom100Fit()
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
	if inpututil.IsKeyJustPressed(ebiten.KeyC) && v.imageB != nil {
		v.setCircleCompare()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) || inpututil.IsKeyJustPressed(ebiten.KeySemicolon) {
		v.toggleFlipHorizontal()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		v.syncSliderWithImage = !v.syncSliderWithImage
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		v.showShadow = !v.showShadow
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		v.showBlur = !v.showBlur
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		v.toggleBorderlessMaximized()
	}

	v.animateView()

	leftMousePressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	rightMousePressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	mouseX, mouseY := ebiten.CursorPosition()
	mouseMoved := false
	if v.idleMouseTracked {
		mouseMoved = mouseX != v.idleMouseX || mouseY != v.idleMouseY
	} else {
		v.idleMouseTracked = true
	}
	if mouseMoved || leftMousePressed || rightMousePressed {
		v.markCompareBorderActivity(now)
	}
	v.idleMouseX = mouseX
	v.idleMouseY = mouseY
	mouseOutsideWindow := mouseX < 0 || mouseY < 0 || mouseX >= v.windowWidth || mouseY >= v.windowHeight
	if v.ignoreMouseUntilRelease {
		if leftMousePressed || rightMousePressed {
			v.updateFramePacing(now, true)
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
		if pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance) {
			_ = v.loadAdjacentImage(-1)
			v.leftMouseDown = true
			return nil
		}
		if pointInTopRightCorner(mouseX, mouseY, v.windowWidth, cornerCommandTolerance) {
			os.Exit(0)
		}

		if !imageReady {
			v.leftMouseDown = true
			return nil
		}

		if v.borderlessMaximized && !pointInRect(mouseX, mouseY, imageRect) && !v.canStartSliderDragFromOutside(mouseX, mouseY, imageRect) {
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
				v.sliderDragMinPosition = 0
				v.sliderDragMaxPosition = 0
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
			v.sliderDragMinPosition = 0
			v.sliderDragMaxPosition = 0
		}
	}
	if rightMousePressed {
		if pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance) {
			_ = v.loadAdjacentImage(1)
			return nil
		}
	}

	if leftMousePressed || (v.draggingSlider && mouseOutsideWindow) {
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
			if v.syncSliderWithImage && v.mode == displayModeCompare && v.compareMask == compareMaskSplit && v.imageA != nil && v.imageB != nil {
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
		v.sliderDragMinPosition = 0
		v.sliderDragMaxPosition = 0
	}

	wheelDelta := input.WheelDelta()
	if wheelDelta != 0 {
		v.markCompareBorderActivity(now)
		shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShift)
		if pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance) {
			if wheelDelta > 0 {
				_ = v.loadAdjacentImage(-1)
			} else {
				_ = v.loadAdjacentImage(1)
			}
		} else if v.mode == displayModeCompare && v.compareMask == compareMaskCircle && v.imageB != nil && imageReady && shiftPressed {
			v.adjustCircleMaskDiameter(wheelDelta)
		} else {
			v.zoomAt(float64(mouseX), float64(mouseY), math.Pow(1.3, wheelDelta))
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		v.zoomAt(float64(v.windowWidth)/2, float64(v.windowHeight)/2, 1.15)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		v.zoomAt(float64(v.windowWidth)/2, float64(v.windowHeight)/2, 1/1.15)
	}

	if v.syncSliderWithImage && v.mode == displayModeCompare && v.compareMask == compareMaskSplit && v.imageA != nil && v.imageB != nil && imageReady && !v.draggingSlider && !v.pendingSliderRestore {
		syncRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
		v.applySliderSync(syncRect)
	}

	v.updateCompareGuideVisibility(now, mouseX, mouseY, imageRect, imageReady)

	if ebiten.IsKeyPressed(ebiten.Key1) {
		v.mode = displayModeSingleA
	}
	if ebiten.IsKeyPressed(ebiten.Key2) && v.imageB != nil {
		v.mode = displayModeSingleB
	}

	v.updateCursorShape(mouseX, mouseY, imageRect, imageReady)
	v.updateFramePacing(now, v.shouldStayActive(mouseMoved, leftMousePressed, rightMousePressed))

	return nil
}

func (v *Viewer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{127, 127, 127, 255})
	newWidth, newHeight := screen.Bounds().Dx(), screen.Bounds().Dy()
	// A native maximize/restore (the window button) can change the backbuffer
	// size before Update gets a chance to enter borderless fullscreen. Capture
	// the split position while the old viewport is still available so it can be
	// restored relative to the image after the resize.
	if newWidth != v.windowWidth || newHeight != v.windowHeight {
		v.prepareSliderRestore()
	}
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
		render.DrawImage(screen, v.imageA, v.windowWidth, v.windowHeight, v.view, v.showShadow)
	case displayModeSingleB:
		render.DrawImage(screen, v.imageB, v.windowWidth, v.windowHeight, v.view, v.showShadow)
	case displayModeCompare:
		v.restoreSliderAfterResize()
		v.ensureSliderPosition()
		mouseX, mouseY := ebiten.CursorPosition()
		if v.compareMask == compareMaskCircle {
			render.DrawCompareCircle(
				screen,
				v.imageA,
				v.imageB,
				v.windowWidth,
				v.windowHeight,
				v.view,
				mouseX,
				mouseY,
				v.circleMaskDiameter,
				v.showBlur,
				v.reverseCompare,
				v.circleBorderOpacity,
				v.showShadow,
			)
		} else {
			render.DrawCompare(
				screen,
				v.imageA,
				v.imageB,
				v.windowWidth,
				v.windowHeight,
				v.view,
				v.slider.Position,
				int(v.slider.Orientation),
				v.showBlur,
				v.reverseCompare,
				v.sliderOpacity,
				v.showShadow,
			)
		}
	}
	mouseX, mouseY := ebiten.CursorPosition()
	drawCornerHints(
		screen,
		v.windowWidth,
		pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance),
		pointInTopRightCorner(mouseX, mouseY, v.windowWidth, cornerCommandTolerance),
	)

	if v.showHelp {
		ebitenutil.DebugPrintAt(screen, v.helpText(), 10, 10)
	}
	if v.loadingImageName != "" {
		ebitenutil.DebugPrintAt(screen, "Loading "+v.loadingImageName, 10, v.windowHeight-22)
	} else if v.loadError != "" {
		ebitenutil.DebugPrintAt(screen, "Load failed: "+v.loadError, 10, v.windowHeight-22)
	}
}

func drawCornerHints(screen *ebiten.Image, windowWidth int, showTopLeft, showTopRight bool) {
	const size = float32(40)
	fill := color.NRGBA{48, 48, 48, cornerHintAlpha}
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
		startView.Zoom = targetView.Zoom * 0.01
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

func (v *Viewer) toggleZoom100Fit() {
	if v.imageA == nil {
		return
	}

	if math.Abs(v.view.Zoom-1) < 0.001 {
		v.resetFit()
		return
	}

	v.stopViewAnimation(true)
	v.view.Zoom = 1
	v.view.OffsetX = 0
	v.view.OffsetY = 0
	v.view.Alpha = 1
	v.targetView = v.view
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
	v.sliderDragMinPosition = 0
	v.sliderDragMaxPosition = 0
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
	v.sliderDragMinPosition = 0
	v.sliderDragMaxPosition = 0
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
		if !v.sliderInitialized {
			v.slider.Position = float64(imageRect.Min.Y+imageRect.Max.Y) / 2
			v.sliderInitialized = true
		}
		v.slider.Position = clampSliderPosition(v.slider.Position, float64(imageRect.Min.Y), float64(imageRect.Max.Y-1))
		v.captureSliderSyncRatio(imageRect)
		return
	}

	if !v.sliderInitialized {
		v.slider.Position = float64(imageRect.Min.X+imageRect.Max.X) / 2
		v.sliderInitialized = true
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
	v.sliderInitialized = false
	v.ensureSliderPosition()
}

func (v *Viewer) setCompareOrientation(orientation compare.Orientation) {
	wasCircle := v.compareMask == compareMaskCircle
	v.compareMask = compareMaskSplit
	if v.slider.Orientation == orientation {
		if wasCircle {
			v.reverseCompare = false
		} else {
			v.reverseCompare = !v.reverseCompare
		}
		if v.mode == displayModeCompare || v.imageB != nil {
			v.mode = displayModeCompare
		}
		return
	}

	v.setSliderOrientation(orientation)
	v.reverseCompare = false
	if v.imageB != nil {
		v.mode = displayModeCompare
	}
}

func (v *Viewer) setCircleCompare() {
	v.mode = displayModeCompare
	now := time.Now()
	v.markCompareBorderActivity(now)
	if v.compareMask == compareMaskCircle {
		v.reverseCompare = !v.reverseCompare
		return
	}

	v.compareMask = compareMaskCircle
	v.reverseCompare = false
	if v.circleMaskDiameter <= 0 {
		v.circleMaskDiameter = defaultCircleMaskDiameterRatio
	}
}

func (v *Viewer) adjustCircleMaskDiameter(wheelDelta float64) {
	v.circleMaskDiameter = clampFloat64(v.circleMaskDiameter*math.Pow(1.15, wheelDelta), 0.02, 1)
}

func (v *Viewer) markCompareBorderActivity(now time.Time) {
	v.lastCompareBorderAt = now
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

	images, currentIndex, err := imagePathsInDirectory(v.imageA.FilePath)
	if err != nil {
		return err
	}

	nextIndex := currentIndex + step
	if currentIndex < 0 || nextIndex < 0 || nextIndex >= len(images) {
		return nil
	}

	nextPath := images[nextIndex]
	if decoded := v.takePrefetchedImage(nextPath); decoded != nil {
		// Invalidate an older asynchronous navigation result before applying
		// the cached image immediately.
		v.trackPendingImageLoad(asyncImageSlotA, 0)
		v.loadingImageName = ""
		v.applyDecodedImage(asyncImageSlotA, decoded, true, false, true)
		return nil
	}

	v.startAsyncImageFileLoad(nextPath, asyncImageSlotA, true, false)
	return nil
}

func imagePathsInDirectory(filePath string) ([]string, int, error) {
	dir := filepath.Dir(filePath)
	currentName := filepath.Base(filePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, -1, err
	}

	images := make([]string, 0, len(entries))
	currentIndex := -1
	for _, entry := range entries {
		if entry.IsDir() || !imagedata.IsSupportedFile(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		images = append(images, path)
		if entry.Name() == currentName {
			currentIndex = len(images) - 1
		}
	}
	return images, currentIndex, nil
}

func (v *Viewer) prefetchAdjacentImages() {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return
	}

	images, currentIndex, err := imagePathsInDirectory(v.imageA.FilePath)
	if err != nil || currentIndex < 0 {
		return
	}

	for _, offset := range []int{-1, 1, -2, 2} {
		index := currentIndex + offset
		if index >= 0 && index < len(images) {
			v.startPrefetch(images[index])
		}
	}
}

func (v *Viewer) startPrefetch(path string) {
	if _, ok := v.prefetchedImages[path]; ok || v.prefetchInFlight[path] {
		return
	}
	v.prefetchInFlight[path] = true
	go func() {
		decoded, err := imagedata.DecodeFile(path)
		v.prefetchResults <- prefetchedImageResult{path: path, decoded: decoded, err: err}
	}()
}

func (v *Viewer) cachePrefetchedImage(path string, decoded *imagedata.DecodedImage) {
	if _, exists := v.prefetchedImages[path]; exists {
		return
	}
	if len(v.prefetchOrder) >= prefetchedImageCacheLimit {
		oldest := v.prefetchOrder[0]
		v.prefetchOrder = v.prefetchOrder[1:]
		delete(v.prefetchedImages, oldest)
	}
	v.prefetchedImages[path] = decoded
	v.prefetchOrder = append(v.prefetchOrder, path)
}

func (v *Viewer) takePrefetchedImage(path string) *imagedata.DecodedImage {
	decoded := v.prefetchedImages[path]
	if decoded == nil {
		return nil
	}
	delete(v.prefetchedImages, path)
	for i, cachedPath := range v.prefetchOrder {
		if cachedPath == path {
			v.prefetchOrder = append(v.prefetchOrder[:i], v.prefetchOrder[i+1:]...)
			break
		}
	}
	return decoded
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

func clampFloat64(value, minValue, maxValue float64) float64 {
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

func (v *Viewer) setDragMode(mouseX, mouseY int, imageRect stdimage.Rectangle) {
	if v.compareMask == compareMaskCircle {
		v.draggingImage = true
		v.draggingSlider = false
		return
	}

	effectiveSliderPos := v.slider.Position
	if v.slider.Orientation == compare.OrientationHorizontal {
		effectiveSliderPos = clampSliderPosition(effectiveSliderPos, float64(imageRect.Min.Y), float64(imageRect.Max.Y))
		if math.Abs(float64(mouseY)-effectiveSliderPos) <= 15 {
			v.draggingSlider = true
			v.draggingImage = false
			v.sliderDragMinPosition = float64(imageRect.Min.Y)
			v.sliderDragMaxPosition = float64(imageRect.Max.Y - 1)
			return
		}
	} else {
		effectiveSliderPos = clampSliderPosition(effectiveSliderPos, float64(imageRect.Min.X), float64(imageRect.Max.X))
		if math.Abs(float64(mouseX)-effectiveSliderPos) <= 15 {
			v.draggingSlider = true
			v.draggingImage = false
			v.sliderDragMinPosition = float64(imageRect.Min.X)
			v.sliderDragMaxPosition = float64(imageRect.Max.X - 1)
			return
		}
	}

	v.draggingImage = true
	v.draggingSlider = false
}

func (v *Viewer) updateSliderPosition(mouseX, mouseY int) {
	minPosition, maxPosition := v.sliderDragBounds()
	if v.slider.Orientation == compare.OrientationHorizontal {
		v.slider.Position = clampSliderPosition(float64(mouseY), minPosition, maxPosition)
		imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
		v.captureSliderSyncRatio(imageRect)
		return
	}

	v.slider.Position = clampSliderPosition(float64(mouseX), minPosition, maxPosition)
	imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
	v.captureSliderSyncRatio(imageRect)
}

func (v *Viewer) sliderDragBounds() (float64, float64) {
	if v.sliderDragMaxPosition >= v.sliderDragMinPosition {
		return v.sliderDragMinPosition, v.sliderDragMaxPosition
	}

	imageRect := render.ImageRect(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view.Zoom, v.view.OffsetX, v.view.OffsetY)
	if v.slider.Orientation == compare.OrientationHorizontal {
		return float64(imageRect.Min.Y), float64(imageRect.Max.Y - 1)
	}

	return float64(imageRect.Min.X), float64(imageRect.Max.X - 1)
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

func (v *Viewer) shouldStayActive(mouseMoved, leftMousePressed, rightMousePressed bool) bool {
	if mouseMoved || leftMousePressed || rightMousePressed || v.draggingImage || v.draggingSlider || v.restoreClickPending {
		return true
	}

	if v.pendingInitialBorderless || v.pendingEnterFullscreen || v.pendingResetFit || v.pendingSliderRestore || v.pendingViewportRebase {
		return true
	}

	if v.pendingImageLoadAID != 0 || v.pendingImageLoadBID != 0 {
		return true
	}

	if len(v.prefetchInFlight) > 0 {
		return true
	}

	if v.viewAnimationActive {
		return true
	}

	if !viewAlmostEqual(v.view, v.targetView) {
		return true
	}

	if v.sliderOpacity > 0 && v.sliderOpacity < 1 {
		return true
	}

	if v.compareMask == compareMaskCircle && v.circleBorderOpacity > 0 {
		return true
	}

	return ebiten.IsKeyPressed(ebiten.KeyEscape) ||
		ebiten.IsKeyPressed(ebiten.KeyR) ||
		ebiten.IsKeyPressed(ebiten.KeyZ) ||
		ebiten.IsKeyPressed(ebiten.KeyW) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowRight) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowUp) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowDown) ||
		ebiten.IsKeyPressed(ebiten.KeyF1) ||
		ebiten.IsKeyPressed(ebiten.KeyH) ||
		ebiten.IsKeyPressed(ebiten.KeyV) ||
		ebiten.IsKeyPressed(ebiten.KeyM) ||
		ebiten.IsKeyPressed(ebiten.KeySemicolon) ||
		ebiten.IsKeyPressed(ebiten.KeyL) ||
		ebiten.IsKeyPressed(ebiten.KeyS) ||
		ebiten.IsKeyPressed(ebiten.KeyB) ||
		ebiten.IsKeyPressed(ebiten.KeyF11) ||
		ebiten.IsKeyPressed(ebiten.KeyC) ||
		ebiten.IsKeyPressed(ebiten.Key1) ||
		ebiten.IsKeyPressed(ebiten.Key2)
}

func (v *Viewer) updateFramePacing(now time.Time, active bool) {
	if active {
		v.lastActivityAt = now
		if v.idleFPSMode {
			ebiten.SetFPSMode(ebiten.FPSModeVsyncOn)
			v.idleFPSMode = false
		}
		return
	}

	if v.lastActivityAt.IsZero() {
		v.lastActivityAt = now
		return
	}

	if !v.idleFPSMode && now.Sub(v.lastActivityAt) >= idleFrameDelay {
		ebiten.SetFPSMode(ebiten.FPSModeVsyncOffMinimum)
		v.idleFPSMode = true
	}
}

func viewAlmostEqual(a, b render.View) bool {
	return math.Abs(a.Zoom-b.Zoom) < 0.001 &&
		math.Abs(a.OffsetX-b.OffsetX) < 0.001 &&
		math.Abs(a.OffsetY-b.OffsetY) < 0.001 &&
		math.Abs(a.Alpha-b.Alpha) < 0.001 &&
		a.FlipHorizontal == b.FlipHorizontal
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
	if v.compareMask == compareMaskCircle {
		if v.circleBorderOpacity <= 0 {
			ebiten.SetCursorMode(ebiten.CursorModeHidden)
		} else {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		}
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
		return
	}
	ebiten.SetCursorMode(ebiten.CursorModeVisible)

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
	if v.compareMask == compareMaskCircle {
		return false
	}

	if v.slider.Orientation == compare.OrientationHorizontal {
		effectiveSliderPos := clampSliderPosition(v.slider.Position, float64(imageRect.Min.Y), float64(imageRect.Max.Y))
		return math.Abs(float64(mouseY)-effectiveSliderPos) <= 15
	}

	effectiveSliderPos := clampSliderPosition(v.slider.Position, float64(imageRect.Min.X), float64(imageRect.Max.X))
	return math.Abs(float64(mouseX)-effectiveSliderPos) <= 15
}

func (v *Viewer) shouldShowSlider(now time.Time, mouseX, mouseY int, imageRect stdimage.Rectangle) bool {
	if imageRect.Empty() {
		return false
	}
	if v.compareMask == compareMaskCircle {
		return v.draggingImage || v.draggingSlider || v.lastCompareBorderAt.IsZero() || now.Sub(v.lastCompareBorderAt) < compareBorderIdleDelay
	}
	if v.compareMask == compareMaskSplit {
		if v.draggingSlider {
			return true
		}
		if v.sliderNearCursor(mouseX, mouseY, imageRect) {
			recentActivity := v.lastCompareBorderAt.IsZero() || now.Sub(v.lastCompareBorderAt) < compareBorderIdleDelay
			return recentActivity
		}
	}
	if v.draggingSlider || v.sliderNearCursor(mouseX, mouseY, imageRect) {
		return true
	}

	if v.slider.Orientation == compare.OrientationHorizontal {
		effectiveSliderPos := clampSliderPosition(v.slider.Position, float64(imageRect.Min.Y), float64(imageRect.Max.Y-1))
		return effectiveSliderPos <= float64(imageRect.Min.Y) || effectiveSliderPos >= float64(imageRect.Max.Y-1)
	}

	effectiveSliderPos := clampSliderPosition(v.slider.Position, float64(imageRect.Min.X), float64(imageRect.Max.X-1))
	return effectiveSliderPos <= float64(imageRect.Min.X) || effectiveSliderPos >= float64(imageRect.Max.X-1)
}

func (v *Viewer) canStartSliderDragFromOutside(mouseX, mouseY int, imageRect stdimage.Rectangle) bool {
	if v.compareMask == compareMaskCircle {
		return false
	}

	if imageRect.Empty() || !v.sliderAtImageEdge(imageRect) || !v.sliderNearCursor(mouseX, mouseY, imageRect) {
		return false
	}

	if v.slider.Orientation == compare.OrientationHorizontal {
		return mouseX >= imageRect.Min.X && mouseX < imageRect.Max.X
	}

	return mouseY >= imageRect.Min.Y && mouseY < imageRect.Max.Y
}

func (v *Viewer) sliderAtImageEdge(imageRect stdimage.Rectangle) bool {
	if imageRect.Empty() {
		return false
	}
	if v.compareMask == compareMaskCircle {
		return false
	}

	if v.slider.Orientation == compare.OrientationHorizontal {
		effectiveSliderPos := clampSliderPosition(v.slider.Position, float64(imageRect.Min.Y), float64(imageRect.Max.Y-1))
		return effectiveSliderPos <= float64(imageRect.Min.Y) || effectiveSliderPos >= float64(imageRect.Max.Y-1)
	}

	effectiveSliderPos := clampSliderPosition(v.slider.Position, float64(imageRect.Min.X), float64(imageRect.Max.X-1))
	return effectiveSliderPos <= float64(imageRect.Min.X) || effectiveSliderPos >= float64(imageRect.Max.X-1)
}

func (v *Viewer) updateCompareGuideVisibility(now time.Time, mouseX, mouseY int, imageRect stdimage.Rectangle, imageReady bool) {
	if v.compareMask == compareMaskCircle {
		targetOpacity := 0.0
		if v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady && v.shouldShowSlider(now, mouseX, mouseY, imageRect) {
			targetOpacity = 1
		}
		v.circleBorderOpacity = approachOpacity(v.circleBorderOpacity, targetOpacity)
		v.sliderOpacity = 0
		return
	}

	v.circleBorderOpacity = 0

	targetOpacity := 0.0
	if v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady && v.shouldShowSlider(now, mouseX, mouseY, imageRect) {
		targetOpacity = 1
	}

	v.sliderOpacity = approachOpacity(v.sliderOpacity, targetOpacity)
}

func approachOpacity(current, target float64) float64 {
	if target > current {
		current += (target - current) * 0.35
		if target-current < 0.01 {
			return target
		}
		return current
	}

	current += (target - current) * 0.18
	if current < 0.01 {
		return 0
	}
	return current
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
	if !v.syncSliderWithImage || v.mode != displayModeCompare || v.compareMask == compareMaskCircle || v.imageA == nil || v.imageB == nil || v.windowWidth <= 0 || v.windowHeight <= 0 || v.view.Zoom <= 0 {
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
	if v.syncSliderWithImage && v.mode == displayModeCompare && v.compareMask == compareMaskSplit && v.imageA != nil && v.imageB != nil {
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

	zoomPercent := int(math.Round(v.view.Zoom * 100))

	shadowMode := "off"
	if v.showShadow {
		shadowMode = "on"
	}

	blurMode := "off"
	if v.showBlur {
		blurMode = "on"
	}

	return "PicaGo " + Version + "\nF1 aide\nZoom image : " + strconv.Itoa(zoomPercent) + "%\nCache : " + v.prefetchStatusText() + "\nC : masque disque / inversion\nH : split horizontal\nV : split vertical\nM : miroir\nMolette : zoom image\nShift+molette : zoom masque cercle\nB : blur cercle " + blurMode + "\nZ : zoom 100% / maxi\nL : slide sync " + syncMode + "\nS : shadow " + shadowMode + "\nR : fit\n1 : " + fileA + "\n2 : " + fileB
}

func (v *Viewer) prefetchStatusText() string {
	if v.imageA != nil && v.imageA.FilePath != "" {
		if paths, _, err := imagePathsInDirectory(v.imageA.FilePath); err == nil {
			status := make([]string, 0, len(paths))
			for _, path := range paths {
				switch {
				case path == v.imageA.FilePath:
					status = append(status, "["+filepath.Base(path)+"]")
				case v.prefetchedImages[path] != nil:
					status = append(status, filepath.Base(path))
				}
			}
			if len(status) > 0 {
				return strings.Join(status, " ")
			}
		}
	}

	status := make([]string, 0, len(v.prefetchOrder)+1)
	if v.imageA != nil && v.imageA.FileName != "" {
		status = append(status, "["+v.imageA.FileName+"]")
	}
	for _, path := range v.prefetchOrder {
		if _, ok := v.prefetchedImages[path]; ok {
			status = append(status, filepath.Base(path))
		}
	}
	if len(status) == 0 {
		return "vide"
	}
	return strings.Join(status, " ")
}
