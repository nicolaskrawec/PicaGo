package app

import (
	stdimage "image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"viewergo/internal/compare"
	"viewergo/internal/render"
)

func (v *Viewer) resetFit() {
	base := v.imageA
	if base == nil {
		return
	}
	v.fitMode = true
	v.markCurrentZoomChanged()

	fitWidth, fitHeight := base.Width, base.Height
	if v.view.Rotation%2 != 0 {
		fitWidth, fitHeight = fitHeight, fitWidth
	}
	targetView := render.View{
		Zoom:           render.FitZoom(v.windowWidth, v.windowHeight, fitWidth, fitHeight),
		OffsetX:        0,
		OffsetY:        0,
		Alpha:          1,
		FlipHorizontal: v.view.FlipHorizontal,
		FlipVertical:   v.view.FlipVertical,
		Rotation:       v.view.Rotation,
		RotationAngle:  v.view.RotationAngle,
		MirrorScaleX:   v.view.MirrorScaleX,
		MirrorScaleY:   v.view.MirrorScaleY,
		Gamma:          v.view.Gamma,
		Exposure:       v.view.Exposure,
		Contrast:       v.view.Contrast,
	}
	if v.skipNextFitAnimation {
		v.skipNextFitAnimation = false
		v.stopViewAnimation(true)
		v.view = targetView
		v.targetView = targetView
	} else if v.animateInitialFit {
		v.animateInitialFit = false
		v.stopViewAnimation(true)
		v.view = targetView
		v.targetView = targetView
		v.startOpeningAnimation()
	} else {
		v.startViewAnimation(v.view, targetView, viewChangeAnimationDuration, 0)
		v.targetView = targetView
	}
	v.ensureSliderPosition()
}

func (v *Viewer) toggleZoom100Fit(mouseX, mouseY float64) {
	if v.imageA == nil {
		return
	}
	v.markCurrentZoomChanged()

	if math.Abs(v.view.Zoom-1) < 0.001 {
		v.resetFit()
		v.showCenterInfo("Zoom %d%%", int(math.Round(v.targetView.Zoom*100)))
		return
	}

	targetView := v.view
	targetView.Zoom = 1
	targetView.Alpha = 1
	imageRect := v.currentImageRect()
	if !imageRect.Empty() {
		mouseX = clampFloat64(mouseX, float64(imageRect.Min.X), float64(imageRect.Max.X))
		mouseY = clampFloat64(mouseY, float64(imageRect.Min.Y), float64(imageRect.Max.Y))
	}
	centerX := float64(v.windowWidth) / 2
	centerY := float64(v.windowHeight) / 2
	imageCenterX := float64(v.imageA.Width) / 2
	imageCenterY := float64(v.imageA.Height) / 2
	if v.view.Zoom > 0 {
		worldX := (mouseX-centerX-v.view.OffsetX)/v.view.Zoom + imageCenterX
		worldY := (mouseY-centerY-v.view.OffsetY)/v.view.Zoom + imageCenterY
		targetView.OffsetX = mouseX - centerX - (worldX-imageCenterX)*targetView.Zoom
		targetView.OffsetY = mouseY - centerY - (worldY-imageCenterY)*targetView.Zoom
	} else {
		targetView.OffsetX = 0
		targetView.OffsetY = 0
	}
	v.fitMode = false
	v.startViewAnimation(v.view, targetView, viewChangeAnimationDuration, 0)
	v.targetView = targetView
	v.ensureSliderPosition()
	v.showCenterInfo("Zoom %d%%", int(math.Round(targetView.Zoom*100)))
}

func (v *Viewer) restoreWindow() {
	if !v.borderlessMaximized {
		return
	}
	v.invalidateSavedImageZooms()

	v.prepareSliderRestore()

	targetX, targetY := v.windowedPosX, v.windowedPosY
	targetWidth, targetHeight := v.windowedWidth, v.windowedHeight
	if v.imageA != nil && v.windowWidth > 0 && v.windowHeight > 0 && v.view.Zoom > 0 {
		imageRect := v.currentImageRect()
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
	v.invalidateSavedImageZooms()

	v.captureWindowedState()
	// Capture the desktop only once. Reusing the GPU texture avoids calling
	// the native GDI capture path again after a fullscreen/restore cycle.
	if v.desktopBackdrop == nil && v.desktopBackground {
		v.desktopBackdrop = captureDesktopBackdrop()
	}
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

func (v *Viewer) currentImageRect() stdimage.Rectangle {
	if v.imageA == nil {
		return stdimage.Rectangle{}
	}
	return render.ImageRectForView(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, v.view)
}

func (v *Viewer) toggleBorderlessMaximized() {
	if v.borderlessMaximized {
		v.restoreWindow()
		return
	}
	v.enterFromNativeMaximize = false
	v.pendingEnterFullscreen = true
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
	if v.fitMode {
		v.pendingResetFit = true
		v.pendingViewportRebase = false
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
