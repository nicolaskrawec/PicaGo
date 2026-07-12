package app

import (
	stdimage "image"
	"io/fs"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"viewergo/internal/compare"
	imagedata "viewergo/internal/image"
	"viewergo/internal/input"
)

func (v *Viewer) Update() error {
	now := time.Now()
	v.rememberCurrentImageView()
	v.collectAsyncImageLoads()
	v.promotePendingHighResImages()
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
		mouseX, _ := ebiten.CursorPosition()
		dropSlot := v.imageDropSlot(mouseX)
		animateImageA := v.imageA == nil
		var droppedImages []string
		var firstDroppedImage string
		_ = fs.WalkDir(dropped, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if path == "." {
				return nil
			}
			if d.IsDir() {
				// Keep the old folder-drop fallback below, but do not mix files
				// from a dropped directory with two explicitly dropped files.
				return fs.SkipDir
			}
			if imagedata.IsSupportedFile(path) {
				if firstDroppedImage == "" {
					firstDroppedImage = path
				}
				droppedImages = append(droppedImages, path)
			}
			return nil
		})

		if len(droppedImages) >= 2 {
			v.startAsyncImageFSLoad(dropped, droppedImages[0], asyncImageSlotA, true, animateImageA)
			v.startAsyncImageFSLoad(dropped, droppedImages[1], asyncImageSlotB, false, false)
		} else if firstDroppedImage != "" {
			if dropSlot == asyncImageSlotA {
				v.startAsyncImageFSLoad(dropped, firstDroppedImage, asyncImageSlotA, true, animateImageA)
			} else {
				v.startAsyncImageFSLoad(dropped, firstDroppedImage, asyncImageSlotB, false, false)
			}
		} else {
			// A dropped directory is handled as before: use its first supported
			// image, found recursively.
			_ = fs.WalkDir(dropped, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				if imagedata.IsSupportedFile(path) {
					if dropSlot == asyncImageSlotA {
						v.startAsyncImageFSLoad(dropped, path, asyncImageSlotA, true, animateImageA)
					} else {
						v.startAsyncImageFSLoad(dropped, path, asyncImageSlotB, false, false)
					}
					return fs.SkipAll
				}
				return nil
			})
		}
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
	if !ebiten.IsKeyPressed(ebiten.KeyShift) && !ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		_ = v.loadAdjacentImage(-1)
	}
	if !ebiten.IsKeyPressed(ebiten.KeyShift) && !ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		_ = v.loadAdjacentImage(1)
	}
	if ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		v.rotateImage(-1)
	}
	if ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		v.rotateImage(1)
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
	if ebiten.IsKeyPressed(ebiten.KeyShift) && (inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyArrowRight)) {
		v.toggleScreenMirror(true)
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
	if !rightMousePressed {
		v.rightMouseDown = false
	}
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
		imageRect = v.currentImageRect()
	}

	if leftMousePressed && !v.leftMouseDown {
		if pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance) {
			v.rotateImage(-1)
			v.leftMouseDown = true
			return nil
		}
		if imageReady && pointInBottomBar(mouseX, mouseY, v.windowWidth, v.windowHeight) {
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
	if rightMousePressed && !v.rightMouseDown {
		if pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance) {
			v.rotateImage(1)
			v.rightMouseDown = true
			return nil
		}
		if imageReady && pointInBottomBar(mouseX, mouseY, v.windowWidth, v.windowHeight) {
			_ = v.loadAdjacentImage(1)
			v.rightMouseDown = true
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
			if dx != 0 || dy != 0 {
				v.fitMode = false
			}
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
		controlPressed := ebiten.IsKeyPressed(ebiten.KeyControl)
		if v.mode == displayModeCompare && v.imageB != nil && controlPressed {
			v.adjustCompareMaskAlpha(wheelDelta)
		} else if pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance) {
			if wheelDelta > 0 {
				v.rotateImage(-1)
			} else {
				v.rotateImage(1)
			}
		} else if imageReady && pointInBottomBar(mouseX, mouseY, v.windowWidth, v.windowHeight) {
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
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			v.toggleScreenMirror(false)
		} else {
			v.zoomAt(float64(v.windowWidth)/2, float64(v.windowHeight)/2, 1.15)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			v.toggleScreenMirror(false)
		} else {
			v.zoomAt(float64(v.windowWidth)/2, float64(v.windowHeight)/2, 1/1.15)
		}
	}

	if v.syncSliderWithImage && v.mode == displayModeCompare && v.compareMask == compareMaskSplit && v.imageA != nil && v.imageB != nil && imageReady && !v.draggingSlider && !v.pendingSliderRestore {
		syncRect := v.currentImageRect()
		v.applySliderSync(syncRect)
	}

	v.updateCompareGuideVisibility(now, mouseX, mouseY, imageRect, imageReady)

	v.updateSoloPreview(ebiten.IsKeyPressed(ebiten.Key1), ebiten.IsKeyPressed(ebiten.Key2))

	v.updateCursorShape(mouseX, mouseY, imageRect, imageReady)
	v.updateFramePacing(now, v.shouldStayActive(mouseMoved, leftMousePressed, rightMousePressed))

	return nil
}

// imageDropSlot divides the window vertically through its center. Until A is
// loaded, the whole window remains a drop target for A.
func (v *Viewer) imageDropSlot(mouseX int) asyncImageSlot {
	if v.imageA == nil || v.windowWidth <= 0 || mouseX < v.windowWidth/2 {
		return asyncImageSlotA
	}
	return asyncImageSlotB
}

func (v *Viewer) updateSoloPreview(showA, showB bool) {
	// If both keys are held, B wins consistently with the previous sequential
	// handling. A missing B must not start a preview that cannot be displayed.
	requestedMode := displayModeSingleA
	hasRequest := showA
	if showB && v.imageB != nil {
		requestedMode = displayModeSingleB
		hasRequest = true
	}
	if hasRequest {
		if !v.soloPreviewActive {
			v.modeBeforeSoloPreview = v.mode
			v.soloPreviewActive = true
		}
		v.mode = requestedMode
		return
	}
	if !v.soloPreviewActive {
		return
	}
	v.mode = v.modeBeforeSoloPreview
	if v.mode == displayModeSingleB && v.imageB == nil {
		v.mode = displayModeSingleA
	}
	if v.mode == displayModeCompare && (v.imageA == nil || v.imageB == nil) {
		v.mode = displayModeSingleA
	}
	v.soloPreviewActive = false
}
