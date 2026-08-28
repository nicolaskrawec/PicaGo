package app

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"viewergo/internal/assets"
	"viewergo/internal/compare"
	imagedata "viewergo/internal/image"
	"viewergo/internal/render"
)

type displayMode int

const (
	displayModeSingleA displayMode = iota
	displayModeSingleB
	displayModeCompare
)

type backgroundMode int

const (
	backgroundDesktop backgroundMode = iota
	backgroundGray
	backgroundBlack
	backgroundWhite
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

type pendingHighResImage struct {
	slot    asyncImageSlot
	loaded  *imagedata.LoadedImage
	decoded *imagedata.DecodedImage
	readyAt time.Time
}

const idleFrameDelay = 500 * time.Millisecond
const targetTPS = 120
const compareBorderIdleDelay = 600 * time.Millisecond
const cursorIdleDelay = 2 * time.Second
const cornerCommandTolerance = 25
const cornerHintAlpha = 50
const defaultCircleMaskDiameterRatio = 0.1
const prefetchedImageCacheLimit = 3
const viewChangeAnimationDuration = 250 * time.Millisecond
const thumbnailMaxDimension = 128
const thumbnailCacheLimit = 16
const thumbnailWorkerLimit = 2
const thumbnailRevealDistance = 120

// Temporary diagnostic switch: keep the screen-sized texture only so we can
// verify whether full-resolution GPU uploads cause navigation stalls.
const fullResolutionUploadEnabled = true

var Version = "dev"

func windowTitle(imageName string) string {
	title := "PicaGo v" + Version
	if imageName == "" {
		return title
	}
	return title + " - " + imageName
}

type Viewer struct {
	imageA   *imagedata.LoadedImage
	imageB   *imagedata.LoadedImage
	decodedA *imagedata.DecodedImage
	decodedB *imagedata.DecodedImage

	mode displayMode
	// The 1/2 keys temporarily override mode while held. Keep the previous mode
	// so comparison (or the other single-image view) can be restored on release.
	modeBeforeSoloPreview displayMode
	soloPreviewActive     bool

	view       render.View
	targetView render.View
	fitMode    bool

	draggingImage           bool
	draggingSlider          bool
	sliderDragMinPosition   float64
	sliderDragMaxPosition   float64
	leftMouseDown           bool
	rightMouseDown          bool
	ignoreMouseUntilRelease bool
	lastMouseX              int
	lastMouseY              int

	windowWidth           int
	windowHeight          int
	slider                compare.Slider
	sliderInitialized     bool
	sliderOpacity         float64
	compareMaskAlpha      float64
	circleBorderOpacity   float64
	showShadow            bool
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

	borderlessMaximized        bool
	pendingInitialBorderless   bool
	pendingEnterFullscreen     bool
	enterFromNativeMaximize    bool
	fullscreenOriginX          int
	fullscreenOriginY          int
	fullscreenZoomRestore      render.View
	fullscreenZoomRestoreFit   bool
	fullscreenZoomRestoreValid bool
	pendingResetFit            bool
	skipNextFitAnimation       bool
	windowedPosX               int
	windowedPosY               int
	windowedWidth              int
	windowedHeight             int
	hasWindowedState           bool
	restoreClickPending        bool
	restoreClickStartX         int
	restoreClickStartY         int
	lastImageClickAt           time.Time
	lastImageClickX            int
	lastImageClickY            int
	lastActivityAt             time.Time
	slideshowPlaying           bool
	nextSlideshowAt            time.Time
	idleFPSMode                bool
	idleMouseTracked           bool
	idleMouseX                 int
	idleMouseY                 int
	lastCursorActivityAt       time.Time
	viewAnimationActive        bool
	viewAnimationStart         time.Time
	viewAnimationLength        time.Duration
	viewAnimationDelayUntil    time.Time
	viewAnimationFrom          render.View
	viewAnimationTo            render.View
	animateInitialFit          bool
	openingAnimationActive     bool
	openingAnimationStart      time.Time
	rotationAnimationActive    bool
	rotationAnimationStart     time.Time
	rotationAnimationFrom      float64
	rotationAnimationTo        float64
	rotationSplitLocalSide     int
	rotationSplitLocalRatio    float64
	pendingRotationTurns       int
	mirrorAnimationActive      bool
	mirrorAnimationStart       time.Time
	mirrorAnimationFrom        float64
	mirrorAnimationTo          float64
	mirrorAnimationHorizontal  bool
	nextImageLoadID            int
	pendingImageLoadAID        int
	pendingImageLoadBID        int
	imageLoadResults           chan asyncImageResult
	prefetchResults            chan prefetchedImageResult
	prefetchInFlight           map[string]bool
	prefetchSlots              chan struct{}
	prefetchedImages           map[string]*imagedata.DecodedImage
	prefetchOrder              []string
	thumbnailResults           chan thumbnailResult
	thumbnailSlots             chan struct{}
	thumbnailInFlight          map[string]bool
	thumbnailFailed            map[string]bool
	thumbnailCache             map[string]*thumbnailCacheEntry
	thumbnailUseCounter        uint64
	thumbnailDirectory         string
	thumbnailOpacity           float64
	pendingHighRes             [2]*pendingHighResImage
	initialPrefetchPending     bool
	navigationDirectory        string
	navigationImages           []string
	imageViewStates            map[string]render.View
	loadingImageName           string
	loadError                  string
	desktopBackdrop            *ebiten.Image
	desktopBackground          bool
	background                 backgroundMode
	preferences                config
	debugMode                  bool
	centerInfoText             string
	centerInfoUntil            time.Time
	centerInfoTexture          *ebiten.Image
}

func Run(args []string) error {
	cfg := loadConfig()
	game := &Viewer{
		mode:                displayModeSingleA,
		windowWidth:         640,
		windowHeight:        480,
		slider:              compare.Slider{Orientation: compare.OrientationVertical},
		circleMaskDiameter:  defaultCircleMaskDiameterRatio,
		compareMaskAlpha:    1,
		lastCompareBorderAt: time.Now(),
		lastActivityAt:      time.Now(),
		showShadow:          cfg.ShowShadow,
		showHelp:            cfg.ShowDebug,
		debugMode:           cfg.ShowDebug,
		background:          backgroundModeFromConfig(cfg.Background),
		preferences:         cfg,
		// A session started without an image can receive one later via drag and
		// drop. Do not capture the desktop in that case: the native capture path
		// is not safe during the subsequent fullscreen transition.
		desktopBackground:   cfg.DesktopBackground && len(args) > 0,
		syncSliderWithImage: true,
		animateInitialFit:   cfg.AnimateOnStart,
		imageLoadResults:    make(chan asyncImageResult, 4),
		prefetchResults:     make(chan prefetchedImageResult, 4),
		prefetchInFlight:    make(map[string]bool),
		prefetchSlots:       make(chan struct{}, 2),
		prefetchedImages:    make(map[string]*imagedata.DecodedImage),
		thumbnailResults:    make(chan thumbnailResult, thumbnailCacheLimit),
		thumbnailSlots:      make(chan struct{}, thumbnailWorkerLimit),
		thumbnailInFlight:   make(map[string]bool),
		thumbnailFailed:     make(map[string]bool),
		thumbnailCache:      make(map[string]*thumbnailCacheEntry),
		imageViewStates:     loadImageViewStates(),
	}

	ebiten.SetWindowResizable(true)
	ebiten.SetTPS(targetTPS)
	ebiten.SetWindowTitle(windowTitle(""))
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowIcon(assets.WindowIcons())

	if len(args) > 0 {
		game.windowWidth = 1280
		game.windowHeight = 720
		ebiten.SetWindowTitle(windowTitle(filepath.Base(args[0]) + " loading..."))
		ebiten.SetWindowSize(1280, 720)
		game.pendingInitialBorderless = true
		game.startAsyncImageFileLoad(args[0], asyncImageSlotA, true, cfg.AnimateOnStart)
	}

	err := ebiten.RunGame(game)
	game.rememberCurrentImageView()
	_ = saveImageViewStates(game.imageViewStates)
	game.savePreferences()
	if err != nil {
		return err
	}
	return nil
}

func (v *Viewer) Draw(screen *ebiten.Image) {
	background := color.RGBA{127, 127, 127, 255}
	switch v.background {
	case backgroundBlack:
		background = color.RGBA{0, 0, 0, 255}
	case backgroundWhite:
		background = color.RGBA{255, 255, 255, 255}
	}
	screen.Fill(background)
	if v.usesDesktopBackground() {
		drawDesktopBackdrop(screen, v.desktopBackdrop)
	}
	if v.usesDesktopBackground() {
		vector.FillRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.RGBA{0, 0, 0, 160}, false)
	}
	newWidth, newHeight := screen.Bounds().Dx(), screen.Bounds().Dy()
	// A native maximize/restore (the window button) can change the backbuffer
	// size before Update gets a chance to enter borderless fullscreen. Capture
	// the split position while the old viewport is still available so it can be
	// restored relative to the image after the resize.
	if newWidth != v.windowWidth || newHeight != v.windowHeight {
		v.prepareSliderRestore()
		if v.fitMode {
			// A resize should settle on the new fit immediately instead of
			// animating the zoom again. Do not affect the initial image fit,
			// which may intentionally animate when the first frame is displayed.
			if v.imageA != nil && v.windowWidth > 0 && v.windowHeight > 0 && !v.animateInitialFit {
				v.skipNextFitAnimation = true
			}
			v.pendingResetFit = true
			v.pendingViewportRebase = false
		}
	}
	// Consume a prepared viewport rebase even when the backbuffer size has
	// not changed yet; the fullscreen transition can be applied between Draw
	// calls.
	v.rebaseViewportForResize(newWidth, newHeight)
	v.windowWidth, v.windowHeight = newWidth, newHeight
	showShadow := v.showShadow

	if v.pendingResetFit {
		v.resetFit()
		v.pendingResetFit = false
	}
	drawView := v.openingDrawView(v.view)

	switch v.mode {
	case displayModeSingleA:
		render.DrawImage(screen, v.imageA, v.windowWidth, v.windowHeight, drawView, showShadow)
	case displayModeSingleB:
		render.DrawComparisonImage(screen, v.imageA, v.imageB, v.windowWidth, v.windowHeight, drawView, showShadow)
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
				drawView,
				mouseX,
				mouseY,
				v.circleMaskDiameter,
				v.reverseCompare,
				v.compareMaskAlpha,
				v.circleBorderOpacity,
				showShadow,
			)
		} else {
			if v.rotationAnimationActive || v.mirrorAnimationActive {
				render.DrawCompareRotating(
					screen, v.imageA, v.imageB, v.windowWidth, v.windowHeight, drawView,
					v.rotationSplitLocalSide, v.rotationSplitLocalRatio, v.compareMaskAlpha, v.sliderOpacity, showShadow,
				)
			} else {
				render.DrawCompare(
					screen,
					v.imageA,
					v.imageB,
					v.windowWidth,
					v.windowHeight,
					drawView,
					v.slider.Position,
					int(v.slider.Orientation),
					v.reverseCompare,
					v.compareMaskAlpha,
					v.sliderOpacity,
					showShadow,
				)
			}
		}
	}
	mouseX, mouseY := ebiten.CursorPosition()
	drawCornerHints(
		screen,
		v.windowWidth,
		pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance),
		pointInTopRightCorner(mouseX, mouseY, v.windowWidth, cornerCommandTolerance),
	)
	v.drawThumbnailStrip(screen)

	if v.showHelp {
		v.drawHelpOverlay(screen)
	}
	v.drawCenterInfo(screen)
	if v.loadingImageName != "" {
		ebitenutil.DebugPrintAt(screen, "Loading "+v.loadingImageName, 10, v.windowHeight-22)
	} else if v.loadError != "" {
		ebitenutil.DebugPrintAt(screen, "Load failed: "+v.loadError, 10, v.windowHeight-22)
	}
}

func (v *Viewer) showCenterInfo(format string, value any) {
	v.centerInfoText = fmt.Sprintf(format, value)
	v.centerInfoUntil = time.Now().Add(900 * time.Millisecond)
	if v.centerInfoTexture != nil {
		v.centerInfoTexture.Deallocate()
	}
	textWidth := maxInt(1, len(v.centerInfoText)*6)
	boxWidth := textWidth + 20
	boxHeight := 24
	v.centerInfoTexture = ebiten.NewImage(boxWidth, boxHeight)
	drawCenterInfoFrame(v.centerInfoTexture, float32(boxWidth), float32(boxHeight))
	ebitenutil.DebugPrintAt(v.centerInfoTexture, v.centerInfoText, 10, 4)
}

func (v *Viewer) drawCenterInfo(screen *ebiten.Image) {
	if v.centerInfoText == "" || v.centerInfoTexture == nil {
		return
	}
	remaining := time.Until(v.centerInfoUntil)
	if remaining <= 0 {
		return
	}

	const textHeight = 12
	const fadeDuration = 250 * time.Millisecond
	textWidth := len(v.centerInfoText) * 6
	boxWidth := float32(textWidth + 20)
	boxHeight := float32(textHeight + 12)
	left := float32(screen.Bounds().Dx()-int(boxWidth)) / 2
	top := float32(screen.Bounds().Dy()-int(boxHeight)) / 2
	fade := 1.0
	if remaining < fadeDuration {
		fade = float64(remaining) / float64(fadeDuration)
	}
	textOptions := &ebiten.DrawImageOptions{}
	textOptions.GeoM.Translate(float64(left), float64(top))
	textOptions.ColorScale.ScaleAlpha(float32(fade))
	screen.DrawImage(v.centerInfoTexture, textOptions)
}

func (v *Viewer) drawHelpOverlay(screen *ebiten.Image) {
	text := v.helpText()
	if !v.debugMode {
		text = v.helpTextCompact()
	}
	lines := strings.Split(text, "\n")
	maxLineWidth := 0
	for _, line := range lines {
		maxLineWidth = maxInt(maxLineWidth, len(line)*6)
	}

	const paddingX = 10
	const paddingY = 8
	const lineHeight = 16
	boxWidth := float32(maxLineWidth + paddingX*2)
	boxHeight := float32(len(lines)*lineHeight + paddingY*2)
	left, top := float32(6), float32(6)
	path := roundedRectPath(left, top, boxWidth, boxHeight, 8)

	fillOptions := &vector.DrawPathOptions{AntiAlias: true}
	fillOptions.ColorScale.ScaleWithColor(color.NRGBA{48, 48, 48, 190})
	vector.FillPath(screen, path, &vector.FillOptions{}, fillOptions)

	borderOptions := &vector.DrawPathOptions{AntiAlias: true}
	borderOptions.ColorScale.ScaleWithColor(color.NRGBA{220, 220, 220, 190})
	vector.StrokePath(screen, path, &vector.StrokeOptions{Width: 1.5}, borderOptions)
	ebitenutil.DebugPrintAt(screen, text, int(left)+paddingX, int(top)+paddingY)
}

func roundedRectPath(left, top, width, height, radius float32) *vector.Path {
	right := left + width
	bottom := top + height
	path := &vector.Path{}
	path.MoveTo(left+radius, top)
	path.LineTo(right-radius, top)
	path.Arc(right-radius, top+radius, radius, -math.Pi/2, 0, vector.Clockwise)
	path.LineTo(right, bottom-radius)
	path.Arc(right-radius, bottom-radius, radius, 0, math.Pi/2, vector.Clockwise)
	path.LineTo(left+radius, bottom)
	path.Arc(left+radius, bottom-radius, radius, math.Pi/2, math.Pi, vector.Clockwise)
	path.LineTo(left, top+radius)
	path.Arc(left+radius, top+radius, radius, math.Pi, math.Pi*1.5, vector.Clockwise)
	path.Close()
	return path
}

func drawCenterInfoFrame(dst *ebiten.Image, width, height float32) {
	path := roundedRectPath(0, 0, width, height, 6)
	options := &vector.DrawPathOptions{AntiAlias: true}
	options.ColorScale.ScaleWithColor(color.NRGBA{64, 64, 64, 255})
	vector.FillPath(dst, path, &vector.FillOptions{}, options)
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
		ebitenutil.DebugPrintAt(screen, "R", 8, 8)
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
