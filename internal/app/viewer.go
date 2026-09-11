package app

import (
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"viewergo/internal/assets"
	"viewergo/internal/compare"
	"viewergo/internal/i18n"
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
	loadFull    imageDecodeFunc
}

type sessionImageView struct {
	Zoom    float64
	OffsetX float64
	OffsetY float64
}

type prefetchedImageResult struct {
	path    string
	decoded *imagedata.DecodedImage
	err     error
}

type pendingHighResImage struct {
	slot     asyncImageSlot
	loaded   *imagedata.LoadedImage
	loadFull imageDecodeFunc
	readyAt  time.Time
}

type imageDecodeFunc func() (*imagedata.DecodedImage, error)

type highResImageResult struct {
	slot    asyncImageSlot
	loaded  *imagedata.LoadedImage
	decoded *imagedata.DecodedImage
	err     error
}

const idleFrameDelay = 500 * time.Millisecond
const compareBorderIdleDelay = 600 * time.Millisecond
const cursorIdleDelay = 2 * time.Second
const adjustmentIndicatorFadeDuration = 250 * time.Millisecond
const adjustmentIndicatorCornerClearance = 60
const cornerCommandTolerance = 25
const cornerHintAlpha = 50
const defaultCircleMaskDiameterRatio = 0.1
const prefetchedImageCacheLimit = 3
const viewChangeAnimationDuration = 250 * time.Millisecond
const thumbnailMaxDimension = 128
const thumbnailCacheLimit = 64
const thumbnailWorkerLimit = 2
const thumbnailRevealDistance = 120
const thumbnailLoadFadeDuration = 220 * time.Millisecond
const thumbnailRevealSlideDistance = 32
const navigationKeyRepeatDelay = 350 * time.Millisecond
const navigationKeyRepeatInterval = 100 * time.Millisecond
const zoomKeyRepeatDelay = 180 * time.Millisecond
const zoomKeyRepeatInterval = 75 * time.Millisecond
const zoomKeyStepFactor = 1.10

var Version = "dev"

func windowTitle(imageName string) string {
	title := "PicaGo " + Version
	if imageName == "" {
		return title
	}
	return title + " - " + imageName
}

func graphicsLibraryFromConfig(value string) ebiten.GraphicsLibrary {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "opengl":
		return ebiten.GraphicsLibraryOpenGL
	case "directx":
		return ebiten.GraphicsLibraryDirectX
	default:
		return ebiten.GraphicsLibraryAuto
	}
}

type Viewer struct {
	imageA *imagedata.LoadedImage
	imageB *imagedata.LoadedImage

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
	navigationKeyDirection  int
	navigationKeyRepeatAt   time.Time
	zoomKeyDirection        int
	zoomKeyRepeatAt         time.Time
	zoomKeyStopAt100        bool

	windowWidth           int
	windowHeight          int
	slider                compare.Slider
	sliderInitialized     bool
	sliderOpacity         float64
	compareMaskAlpha      float64
	circleBorderOpacity   float64
	showShadow            bool
	showHelp              bool
	showEXIF              bool
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
	highResResults             chan highResImageResult
	prefetchResults            chan prefetchedImageResult
	prefetchInFlight           map[string]bool
	decodeSlots                chan struct{}
	prefetchedImages           map[string]*imagedata.LoadedImage
	prefetchOrder              []string
	thumbnailResults           chan thumbnailResult
	thumbnailInFlight          map[string]bool
	thumbnailFailed            map[string]bool
	thumbnailCache             map[string]*thumbnailCacheEntry
	thumbnailUseCounter        uint64
	thumbnailDirectory         string
	thumbnailOpacity           float64
	hoveredThumbnailPath       string
	thumbnailAnimationActive   bool
	thumbnailAnimationStart    time.Time
	thumbnailAnimationItems    []thumbnailTransitionItem
	pendingHighRes             [2]*pendingHighResImage
	prefetchAfterHighRes       *imagedata.LoadedImage
	navigationDirectory        string
	navigationImages           []string
	folderSortModes            map[string]imageSortMode
	imageViewStates            map[string]render.View
	// Zoom and position are intentionally kept separately from
	// imageViewStates: they follow the image during this process only and are
	// never written to disk.
	sessionImageViews map[string]sessionImageView
	loadingImageName  string
	loadError         string
	desktopBackdrop   *ebiten.Image
	desktopBackground bool
	background        backgroundMode
	preferences       config
	localizer         i18n.Localizer
	debugMode         bool
	centerInfoText    string
	centerInfoUntil   time.Time
	centerInfoTexture *ebiten.Image
}

func Run(args []string) error {
	return run(args, os.Stdout)
}

func run(args []string, output io.Writer) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		_, err := fmt.Fprintf(output, "PicaGo %s\n", Version)
		return err
	}

	cfg := loadConfig()
	initDebugLogging(cfg.ShowDebug)
	debugf("startup: args=%d desktopBackground=%v background=%q graphicsLibrary=%q", len(args), cfg.DesktopBackground, cfg.Background, cfg.GraphicsLibrary)
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
		localizer:           i18n.New(cfg.Language),
		// A session started without an image can receive one later via drag and
		// drop. Do not capture the desktop in that case: the native capture path
		// is not safe during the subsequent fullscreen transition.
		desktopBackground:   cfg.DesktopBackground && len(args) > 0,
		syncSliderWithImage: true,
		animateInitialFit:   cfg.AnimateOnStart,
		imageLoadResults:    make(chan asyncImageResult, 4),
		highResResults:      make(chan highResImageResult, 2),
		prefetchResults:     make(chan prefetchedImageResult, 4),
		prefetchInFlight:    make(map[string]bool),
		// Image decoders temporarily allocate the original pixel buffer even
		// when only a preview is retained. Serializing them bounds that peak to
		// one full CPU image at a time across display, prefetch and thumbnails.
		// Allow two independent image decodes to use the CPU concurrently.
		// This matches the thumbnail worker limit while keeping the temporary
		// full-resolution memory peak bounded.
		decodeSlots:       make(chan struct{}, 2),
		prefetchedImages:  make(map[string]*imagedata.LoadedImage),
		thumbnailResults:  make(chan thumbnailResult, thumbnailCacheLimit),
		thumbnailInFlight: make(map[string]bool),
		thumbnailFailed:   make(map[string]bool),
		thumbnailCache:    make(map[string]*thumbnailCacheEntry),
		folderSortModes:   loadFolderSortModes(),
		imageViewStates:   loadImageViewStates(),
		sessionImageViews: make(map[string]sessionImageView),
	}
	// Keep "auto" in the preferences so a later OS language change is picked
	// up. Explicit and invalid values are normalized before they are saved.
	configuredLanguage := strings.ToLower(strings.TrimSpace(cfg.Language))
	if configuredLanguage != "" && configuredLanguage != i18n.AutoLanguage {
		game.preferences.Language = game.localizer.Language()
	}

	ebiten.SetWindowResizable(true)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebiten.SetWindowTitle(windowTitle(""))
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowIcon(assets.WindowIcons())

	if len(args) > 0 {
		game.windowWidth = 1280
		game.windowHeight = 720
		ebiten.SetWindowTitle(windowTitle(game.localizer.Format("status.loading", filepath.Base(args[0]))))
		ebiten.SetWindowSize(1280, 720)
		game.pendingInitialBorderless = true
		game.startAsyncImageFileLoad(args[0], asyncImageSlotA, true, cfg.AnimateOnStart)
	}

	err := ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		GraphicsLibrary: graphicsLibraryFromConfig(cfg.GraphicsLibrary),
	})
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
		v.stopThumbnailAnimation()
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
	v.drawThumbnailStrip(screen)
	v.drawAdjustmentIndicator(screen, time.Now(), mouseX, mouseY)
	drawCornerHints(
		screen,
		v.windowWidth,
		v.windowHeight,
		pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance),
		pointInTopRightCorner(mouseX, mouseY, v.windowWidth, cornerCommandTolerance),
		pointInBottomLeftCorner(mouseX, mouseY, v.windowHeight, cornerCommandTolerance),
		pointInBottomRightCorner(mouseX, mouseY, v.windowWidth, v.windowHeight, cornerCommandTolerance),
	)

	if v.showHelp {
		v.drawHelpOverlay(screen)
	}
	if v.showEXIF {
		v.drawEXIFOverlay(screen)
	}
	v.drawCenterInfo(screen)
	if v.loadingImageName != "" {
		ebitenutil.DebugPrintAt(screen, v.message("status.loading", v.loadingImageName), 10, v.windowHeight-22)
	} else if v.loadError != "" {
		ebitenutil.DebugPrintAt(screen, v.message("error.load_failed", v.loadError), 10, v.windowHeight-22)
	}
}

func (v *Viewer) drawAdjustmentIndicator(screen *ebiten.Image, now time.Time, mouseX, mouseY int) {
	if v.imageA == nil || !imageAdjustmentsActive(v.view) || pointNearAnyCorner(
		mouseX, mouseY, v.windowWidth, v.windowHeight, adjustmentIndicatorCornerClearance,
	) {
		return
	}
	opacity := cursorActivityOpacity(now, v.lastCursorActivityAt)
	if opacity <= 0 {
		return
	}

	const (
		leftMargin  = float32(12)
		arm         = float32(4)
		gap         = float32(4)
		strokeWidth = float32(1.25)
	)
	x := leftMargin + arm
	y := float32(v.windowHeight) / 2
	ink := color.NRGBA{R: 255, G: 255, B: 255, A: uint8(math.Round(190 * opacity))}
	vector.StrokeLine(screen, x-arm, y-gap, x+arm, y-gap, strokeWidth, ink, true)
	vector.StrokeLine(screen, x, y-gap-arm, x, y-gap+arm, strokeWidth, ink, true)
	vector.StrokeLine(screen, x-arm, y+gap+arm, x+arm, y+gap+arm, strokeWidth, ink, true)
}

func cursorActivityOpacity(now, lastActivity time.Time) float64 {
	if lastActivity.IsZero() {
		return 0
	}
	remaining := cursorIdleDelay - now.Sub(lastActivity)
	if remaining <= 0 {
		return 0
	}
	if remaining >= adjustmentIndicatorFadeDuration {
		return 1
	}
	return float64(remaining) / float64(adjustmentIndicatorFadeDuration)
}

func (v *Viewer) text(key string) string {
	return v.localizer.Text(key)
}

func (v *Viewer) message(key string, args ...any) string {
	return v.localizer.Format(key, args...)
}

func (v *Viewer) showCenterInfo(key string, args ...any) {
	v.showCenterInfoText(v.message(key, args...))
}

func (v *Viewer) showCenterInfoText(text string) {
	v.centerInfoText = text
	v.centerInfoUntil = time.Now().Add(900 * time.Millisecond)
	if v.centerInfoTexture != nil {
		v.centerInfoTexture.Deallocate()
	}
	textWidth := maxInt(1, utf8.RuneCountInString(v.centerInfoText)*6)
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
	textWidth := utf8.RuneCountInString(v.centerInfoText) * 6
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
		maxLineWidth = maxInt(maxLineWidth, utf8.RuneCountInString(line)*6)
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

func (v *Viewer) drawEXIFOverlay(screen *ebiten.Image) {
	text := v.exifText()
	lines := strings.Split(text, "\n")
	maxVisibleLines := maxInt(1, (screen.Bounds().Dy()-28)/16)
	if len(lines) > maxVisibleLines {
		hidden := len(lines) - maxVisibleLines + 1
		lines = append(lines[:maxVisibleLines-1], v.message("exif.more", hidden))
	}
	maxLineRunes := maxInt(8, (screen.Bounds().Dx()-32)/6)
	for i, line := range lines {
		lines[i] = truncateOverlayLine(line, maxLineRunes)
	}
	text = strings.Join(lines, "\n")

	maxLineWidth := 0
	for _, line := range lines {
		maxLineWidth = maxInt(maxLineWidth, utf8.RuneCountInString(line)*6)
	}

	const paddingX = 10
	const paddingY = 8
	const lineHeight = 16
	boxWidth := float32(maxLineWidth + paddingX*2)
	boxHeight := float32(len(lines)*lineHeight + paddingY*2)
	left, top := float32(6), float32(6)
	path := roundedRectPath(left, top, boxWidth, boxHeight, 8)

	fillOptions := &vector.DrawPathOptions{AntiAlias: true}
	fillOptions.ColorScale.ScaleWithColor(color.NRGBA{48, 48, 48, 210})
	vector.FillPath(screen, path, &vector.FillOptions{}, fillOptions)

	borderOptions := &vector.DrawPathOptions{AntiAlias: true}
	borderOptions.ColorScale.ScaleWithColor(color.NRGBA{220, 220, 220, 190})
	vector.StrokePath(screen, path, &vector.StrokeOptions{Width: 1.5}, borderOptions)
	ebitenutil.DebugPrintAt(screen, text, int(left)+paddingX, int(top)+paddingY)
}

func truncateOverlayLine(line string, maxRunes int) string {
	if maxRunes < 1 || utf8.RuneCountInString(line) <= maxRunes {
		return line
	}
	if maxRunes == 1 {
		return "…"
	}
	runes := []rune(line)
	return string(runes[:maxRunes-1]) + "…"
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

func drawCornerHints(screen *ebiten.Image, windowWidth, windowHeight int, showTopLeft, showTopRight, showBottomLeft, showBottomRight bool) {
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

	if showBottomLeft {
		bottom := float32(windowHeight)
		bottomLeft := &vector.Path{}
		bottomLeft.MoveTo(0, bottom)
		bottomLeft.LineTo(0, bottom-size)
		bottomLeft.Arc(0, bottom, size, -math.Pi/2, 0, vector.Clockwise)
		bottomLeft.Close()
		options.ColorScale.Reset()
		options.ColorScale.ScaleWithColor(fill)
		vector.FillPath(screen, bottomLeft, &vector.FillOptions{}, options)
		ebitenutil.DebugPrintAt(screen, "H", 8, windowHeight-20)
	}

	if showBottomRight {
		right := float32(windowWidth)
		bottom := float32(windowHeight)
		bottomRight := &vector.Path{}
		bottomRight.MoveTo(right, bottom)
		bottomRight.LineTo(right-size, bottom)
		bottomRight.Arc(right, bottom, size, math.Pi, math.Pi*1.5, vector.Clockwise)
		bottomRight.Close()
		options.ColorScale.Reset()
		options.ColorScale.ScaleWithColor(fill)
		vector.FillPath(screen, bottomRight, &vector.FillOptions{}, options)
		ebitenutil.DebugPrintAt(screen, "G", windowWidth-16, windowHeight-20)
	}

}

func (v *Viewer) Layout(outsideWidth, outsideHeight int) (int, int) {
	if v.windowWidth == 0 || v.windowHeight == 0 {
		v.windowWidth = outsideWidth
		v.windowHeight = outsideHeight
	}
	return outsideWidth, outsideHeight
}
