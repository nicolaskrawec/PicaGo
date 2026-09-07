package app

import (
	stdimage "image"
	"io/fs"
	"math"
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
	v.collectHighResolutionImages()
	v.collectPrefetchedImages()
	v.collectThumbnails()
	v.ensureVisibleThumbnails()
	v.updateThumbnailAnimation(now)

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
		return ebiten.Termination
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		v.resetViewParameters()
		v.showCenterInfo("%s", "Vue réinitialisée")
	}
	// On AZERTY, the physical key labelled Z can be reported as KeyW by
	// Ebiten/GLFW (the key constants follow the US layout).
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		zoomMouseX, zoomMouseY := ebiten.CursorPosition()
		v.toggleZoom100Fit(float64(zoomMouseX), float64(zoomMouseY))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		v.toggleSlideshow(now)
	}
	v.updateNavigationKeyRepeat(now)
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
		v.savePreferences()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		v.toggleBackground()
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
		v.lastCursorActivityAt = now
	}
	if mouseMoved || leftMousePressed || rightMousePressed {
		v.markCompareBorderActivity(now)
		v.lastCursorActivityAt = now
	}
	v.idleMouseX = mouseX
	v.idleMouseY = mouseY
	v.updateThumbnailVisibility(now, mouseX, mouseY)
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
		// The bottom of the split can overlap the thumbnail strip. Give a
		// click near the split line priority so the line remains draggable.
		splitSliderClick := false
		if v.mode == displayModeCompare && v.compareMask == compareMaskSplit &&
			v.imageA != nil && v.imageB != nil && !v.pendingResetFit && v.view.Zoom > 0 {
			splitSliderClick = v.isSplitSliderClick(mouseX, mouseY, v.currentImageRect())
		}
		if !splitSliderClick {
			if path, ok := v.thumbnailPathAt(mouseX, mouseY); ok {
				if v.imageA == nil || path != v.imageA.FilePath {
					v.loadNavigationImage(path)
				}
				v.leftMouseDown = true
				return nil
			}
			// The gaps belong to the thumbnail strip too. Consume the click so it
			// cannot fall through to the legacy bottom-bar navigation underneath.
			if v.pointInThumbnailStrip(mouseX, mouseY) {
				v.leftMouseDown = true
				return nil
			}
		}
		if pointInTopRightCorner(mouseX, mouseY, v.windowWidth, cornerCommandTolerance) {
			return ebiten.Termination
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
					v.toggleFullscreen100Zoom(float64(mouseX), float64(mouseY))
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
		v.lastCursorActivityAt = now
		shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShift)
		controlPressed := ebiten.IsKeyPressed(ebiten.KeyControl)
		altPressed := ebiten.IsKeyPressed(ebiten.KeyAlt)
		if controlPressed && altPressed {
			v.adjustContrast(wheelDelta)
		} else if v.mode == displayModeCompare && v.imageB != nil && controlPressed && !shiftPressed {
			v.adjustCompareMaskAlpha(wheelDelta)
		} else if pointInTopLeftCorner(mouseX, mouseY, cornerCommandTolerance) {
			if wheelDelta > 0 {
				v.rotateImage(-1)
			} else {
				v.rotateImage(1)
			}
		} else if imageReady && v.pointInThumbnailStrip(mouseX, mouseY) {
			if wheelDelta > 0 {
				_ = v.loadAdjacentImage(-1)
			} else {
				_ = v.loadAdjacentImage(1)
			}
		} else if v.mode == displayModeCompare && v.compareMask == compareMaskCircle && v.imageB != nil && imageReady && shiftPressed && controlPressed {
			v.adjustCircleMaskDiameter(wheelDelta)
		} else if shiftPressed {
			v.adjustGamma(wheelDelta)
		} else if altPressed {
			v.adjustExposure(wheelDelta)
		} else {
			v.zoomAt(float64(mouseX), float64(mouseY), math.Pow(1.15, wheelDelta))
		}
	}
	v.updateZoomKeyRepeat(now)

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			v.toggleScreenMirror(false)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			v.toggleScreenMirror(false)
		}
	}

	if v.syncSliderWithImage && v.mode == displayModeCompare && v.compareMask == compareMaskSplit && v.imageA != nil && v.imageB != nil && imageReady && !v.draggingSlider && !v.pendingSliderRestore {
		syncRect := v.currentImageRect()
		v.applySliderSync(syncRect)
	}

	v.updateCompareGuideVisibility(now, imageRect, imageReady)

	v.updateSoloPreview(ebiten.IsKeyPressed(ebiten.Key1), ebiten.IsKeyPressed(ebiten.Key2))

	v.updateCursorShape(now, mouseX, mouseY, imageRect, imageReady)
	v.updateSlideshow(now)
	v.updateFramePacing(now, v.shouldStayActive(now, mouseMoved, leftMousePressed, rightMousePressed))

	return nil
}

// updateNavigationKeyRepeat handles the OS-independent repeat behaviour for
// the left/right navigation keys. Ebiten's IsKeyPressed reports the physical
// state, so this also works when the platform's key-repeat settings differ.
func (v *Viewer) updateNavigationKeyRepeat(now time.Time) {
	leftPressed := ebiten.IsKeyPressed(ebiten.KeyArrowLeft)
	rightPressed := ebiten.IsKeyPressed(ebiten.KeyArrowRight)
	plainNavigation := !ebiten.IsKeyPressed(ebiten.KeyShift) && !ebiten.IsKeyPressed(ebiten.KeyControl)

	direction := 0
	if plainNavigation {
		switch {
		case leftPressed && !rightPressed:
			direction = -1
		case rightPressed && !leftPressed:
			direction = 1
		}
	}

	if direction == 0 {
		v.navigationKeyDirection = 0
		v.navigationKeyRepeatAt = time.Time{}
		return
	}

	if direction != v.navigationKeyDirection {
		v.navigationKeyDirection = direction
		v.navigationKeyRepeatAt = now.Add(navigationKeyRepeatDelay)
		_ = v.loadAdjacentImage(direction)
		return
	}

	if now.Before(v.navigationKeyRepeatAt) {
		return
	}

	// Do not issue another request while the previous preview is still being
	// decoded. The next repeat is scheduled from the current frame so a slow
	// decode cannot cause a burst of stale requests.
	if v.pendingImageLoadAID != 0 {
		v.navigationKeyRepeatAt = now.Add(navigationKeyRepeatInterval)
		return
	}

	_ = v.loadAdjacentImage(direction)
	v.navigationKeyRepeatAt = now.Add(navigationKeyRepeatInterval)
}

func (v *Viewer) updateZoomKeyRepeat(now time.Time) {
	upPressed := ebiten.IsKeyPressed(ebiten.KeyArrowUp)
	downPressed := ebiten.IsKeyPressed(ebiten.KeyArrowDown)
	direction := 0
	if !ebiten.IsKeyPressed(ebiten.KeyShift) {
		switch {
		case upPressed && !downPressed:
			direction = 1
		case downPressed && !upPressed:
			direction = -1
		}
	}

	if direction == 0 {
		v.zoomKeyDirection = 0
		v.zoomKeyRepeatAt = time.Time{}
		v.zoomKeyStopAt100 = false
		return
	}

	if direction != v.zoomKeyDirection {
		v.zoomKeyDirection = direction
		v.zoomKeyRepeatAt = now.Add(zoomKeyRepeatDelay)
		v.zoomKeyStopAt100 = (direction > 0 && v.targetView.Zoom < 1) || (direction < 0 && v.targetView.Zoom > 1)
		v.zoomAtKeyboard(direction)
		return
	}

	if now.Before(v.zoomKeyRepeatAt) {
		return
	}

	if v.zoomKeyStopAt100 && v.targetView.Zoom == 1 {
		v.zoomKeyRepeatAt = now.Add(zoomKeyRepeatInterval)
		return
	}

	v.zoomAtKeyboard(direction)
	v.zoomKeyRepeatAt = now.Add(zoomKeyRepeatInterval)
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
