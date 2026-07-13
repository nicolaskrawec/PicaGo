package app

import (
	"math"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"viewergo/internal/render"
)

const openingAnimationDuration = 500 * time.Millisecond

func (v *Viewer) startOpeningAnimation() {
	v.openingAnimationStart = time.Now()
	v.openingAnimationActive = true
}

// openingDrawView is a presentation-only transform. Unlike resetFit and the
// Z/R animations it never mutates the real view or its target.
func (v *Viewer) openingDrawView(base render.View) render.View {
	if !v.openingAnimationActive {
		return base
	}
	elapsed := time.Since(v.openingAnimationStart)
	if elapsed >= openingAnimationDuration {
		v.openingAnimationActive = false
		return base
	}
	t := float64(elapsed) / float64(openingAnimationDuration)
	eased := 1 - math.Pow(1-t, 3)
	zoomScale := lerpFloat(0.1, 1, eased)
	base.Zoom *= zoomScale
	visibility := math.Max(0, math.Min(1, (zoomScale-0.5)/0.5))
	base.Alpha *= visibility
	return base
}

func (v *Viewer) toggleFlipHorizontal() {
	v.startMirrorAnimation(true)
}

func (v *Viewer) toggleFlipVertical() {
	v.startMirrorAnimation(false)
}

// toggleScreenMirror maps the requested visible axis back to the image's
// local axes. A quarter turn swaps horizontal and vertical on screen.
func (v *Viewer) toggleScreenMirror(horizontal bool) {
	rotation := ((v.view.Rotation % 4) + 4) % 4
	if rotation%2 != 0 {
		horizontal = !horizontal
	}
	if horizontal {
		v.toggleFlipHorizontal()
	} else {
		v.toggleFlipVertical()
	}
}

func (v *Viewer) startMirrorAnimation(horizontal bool) {
	v.rememberCurrentImageView()
	localSide, localRatio, hasLocalSplit := v.captureCompareLocalSplit()
	if v.mirrorAnimationActive && v.mirrorAnimationHorizontal == horizontal {
		v.mirrorAnimationFrom = v.mirrorAnimationTo
		v.mirrorAnimationTo = -v.mirrorAnimationTo
	} else {
		from := 1.0
		if (horizontal && v.view.FlipHorizontal) || (!horizontal && v.view.FlipVertical) {
			from = -1
		}
		v.mirrorAnimationFrom = from
		v.mirrorAnimationTo = -from
	}
	v.mirrorAnimationHorizontal = horizontal
	v.mirrorAnimationStart = time.Now()
	v.mirrorAnimationActive = true
	if horizontal {
		v.view.FlipHorizontal = v.mirrorAnimationTo < 0
		v.targetView.FlipHorizontal = v.view.FlipHorizontal
		v.view.MirrorScaleX = v.mirrorAnimationFrom
		v.targetView.MirrorScaleX = v.mirrorAnimationTo
	} else {
		v.view.FlipVertical = v.mirrorAnimationTo < 0
		v.targetView.FlipVertical = v.view.FlipVertical
		v.view.MirrorScaleY = v.mirrorAnimationFrom
		v.targetView.MirrorScaleY = v.mirrorAnimationTo
	}
	if hasLocalSplit {
		v.rotationSplitLocalSide = localSide
		v.rotationSplitLocalRatio = localRatio
		v.applyCompareLocalSplit(localSide, localRatio, v.targetView)
	}
}

func (v *Viewer) rotateImage(quarterTurns int) {
	v.rememberCurrentImageView()
	if v.rotationAnimationActive {
		v.pendingRotationTurns += quarterTurns
		return
	}
	v.startRotationAnimation(quarterTurns)
}

func (v *Viewer) startRotationAnimation(quarterTurns int) {
	from := v.view.RotationAngle
	target := ((v.view.Rotation+quarterTurns)%4 + 4) % 4
	to := from + float64(quarterTurns)
	v.updateCompareForRotation(quarterTurns, target, to)
	v.view.Rotation = target
	v.targetView.Rotation = target
	v.view.RotationAngle = from
	v.targetView.RotationAngle = to
	v.rotationAnimationFrom = from
	v.rotationAnimationTo = to
	v.rotationAnimationStart = time.Now()
	v.rotationAnimationActive = true
}

func (v *Viewer) zoomAt(mouseX, mouseY, factor float64) {
	base := v.imageA
	if base == nil {
		return
	}
	v.fitMode = false
	v.markCurrentZoomChanged()
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
	v.showCenterInfo("Zoom %d%%", int(math.Round(newZoom*100)))
}

func (v *Viewer) animateView() {
	if v.mirrorAnimationActive {
		elapsed := time.Since(v.mirrorAnimationStart)
		const mirrorAnimationDuration = 250 * time.Millisecond
		if elapsed >= mirrorAnimationDuration {
			if v.mirrorAnimationHorizontal {
				v.view.MirrorScaleX = v.mirrorAnimationTo
			} else {
				v.view.MirrorScaleY = v.mirrorAnimationTo
			}
			v.mirrorAnimationActive = false
		} else {
			t := float64(elapsed) / float64(mirrorAnimationDuration)
			eased := 1 - math.Pow(1-t, 3)
			scale := lerpFloat(v.mirrorAnimationFrom, v.mirrorAnimationTo, eased)
			if v.mirrorAnimationHorizontal {
				v.view.MirrorScaleX = scale
			} else {
				v.view.MirrorScaleY = scale
			}
		}
	}
	if v.rotationAnimationActive {
		elapsed := time.Since(v.rotationAnimationStart)
		const rotationAnimationDuration = 250 * time.Millisecond
		if elapsed >= rotationAnimationDuration {
			v.view.RotationAngle = v.rotationAnimationTo
			v.rotationAnimationActive = false
			if v.pendingRotationTurns != 0 {
				step := 1
				if v.pendingRotationTurns < 0 {
					step = -1
				}
				v.pendingRotationTurns -= step
				v.startRotationAnimation(step)
			}
		} else {
			t := float64(elapsed) / float64(rotationAnimationDuration)
			eased := 1 - math.Pow(1-t, 3)
			v.view.RotationAngle = lerpFloat(v.rotationAnimationFrom, v.rotationAnimationTo, eased)
		}
	}
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
		v.view.FlipVertical = v.viewAnimationTo.FlipVertical
		v.view.Rotation = v.viewAnimationTo.Rotation
		v.view.RotationAngle = v.viewAnimationTo.RotationAngle
		v.view.Gamma = lerpFloat(v.viewAnimationFrom.Gamma, v.viewAnimationTo.Gamma, eased)
		v.view.Exposure = lerpFloat(v.viewAnimationFrom.Exposure, v.viewAnimationTo.Exposure, eased)
		v.view.Contrast = lerpFloat(v.viewAnimationFrom.Contrast, v.viewAnimationTo.Contrast, eased)
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

	if math.Abs(v.view.Gamma-v.targetView.Gamma) < 0.001 {
		v.view.Gamma = v.targetView.Gamma
	} else {
		v.view.Gamma += (v.targetView.Gamma - v.view.Gamma) * smoothing
	}
	if math.Abs(v.view.Exposure-v.targetView.Exposure) < 0.001 {
		v.view.Exposure = v.targetView.Exposure
	} else {
		v.view.Exposure += (v.targetView.Exposure - v.view.Exposure) * smoothing
	}
	if math.Abs(v.view.Contrast-v.targetView.Contrast) < 0.001 {
		v.view.Contrast = v.targetView.Contrast
	} else {
		v.view.Contrast += (v.targetView.Contrast - v.view.Contrast) * smoothing
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
	if v.centerInfoText != "" && time.Now().Before(v.centerInfoUntil) {
		return true
	}
	if v.openingAnimationActive {
		return true
	}

	if v.rotationAnimationActive {
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
		ebiten.IsKeyPressed(ebiten.KeyL) ||
		ebiten.IsKeyPressed(ebiten.KeyS) ||
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
		// Image decodes and cache evictions may have released large CPU-side
		// buffers. Run one collection when the viewer becomes idle, rather than
		// adding GC pauses to active navigation.
		runtime.GC()
	}
}

func viewAlmostEqual(a, b render.View) bool {
	return math.Abs(a.Zoom-b.Zoom) < 0.001 &&
		math.Abs(a.OffsetX-b.OffsetX) < 0.001 &&
		math.Abs(a.OffsetY-b.OffsetY) < 0.001 &&
		math.Abs(a.Alpha-b.Alpha) < 0.001 &&
		a.FlipHorizontal == b.FlipHorizontal &&
		a.FlipVertical == b.FlipVertical &&
		a.Rotation == b.Rotation &&
		math.Abs(a.RotationAngle-b.RotationAngle) < 0.001 &&
		math.Abs(a.MirrorScaleX-b.MirrorScaleX) < 0.001 &&
		math.Abs(a.MirrorScaleY-b.MirrorScaleY) < 0.001 &&
		math.Abs(a.Gamma-b.Gamma) < 0.001 &&
		math.Abs(a.Exposure-b.Exposure) < 0.001 &&
		math.Abs(a.Contrast-b.Contrast) < 0.001
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
