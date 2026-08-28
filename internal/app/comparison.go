package app

import (
	stdimage "image"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"viewergo/internal/compare"
	"viewergo/internal/render"
)

func (v *Viewer) ensureSliderPosition() {
	if v.imageA == nil {
		return
	}

	imageRect := v.currentImageRect()
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
	v.markCompareBorderActivity(time.Now())
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

func (v *Viewer) adjustCompareMaskAlpha(wheelDelta float64) {
	v.compareMaskAlpha = clampFloat64(v.compareMaskAlpha+wheelDelta*0.1, 0, 1)
}

func (v *Viewer) markCompareBorderActivity(now time.Time) {
	v.lastCompareBorderAt = now
}

func (v *Viewer) captureCompareLocalSplit() (int, float64, bool) {
	if v.imageA == nil || v.imageB == nil || v.mode != displayModeCompare || v.compareMask != compareMaskSplit {
		return 0, 0, false
	}
	rect := v.currentImageRect()
	if rect.Empty() {
		return 0, 0, false
	}
	screenSide := 1
	ratio := (v.slider.Position - float64(rect.Min.X)) / float64(rect.Dx())
	if v.slider.Orientation == compare.OrientationHorizontal {
		ratio = (v.slider.Position - float64(rect.Min.Y)) / float64(rect.Dy())
		if v.reverseCompare {
			screenSide = 0
		} else {
			screenSide = 2
		}
	} else if v.reverseCompare {
		screenSide = 3
	}
	ratio = clampFloat64(ratio, 0, 1)
	rotation := ((v.view.Rotation % 4) + 4) % 4
	if splitRatioReversedByRotation(rotation, screenSide) {
		ratio = 1 - ratio
	}
	localSide := (screenSide - rotation + 4) % 4
	flipH, flipV := effectiveViewFlips(v.view)
	if flipH && (localSide == 1 || localSide == 3) {
		localSide = 4 - localSide
		ratio = 1 - ratio
	}
	if flipV && (localSide == 0 || localSide == 2) {
		localSide = 2 - localSide
		ratio = 1 - ratio
	}
	return localSide, ratio, true
}

func (v *Viewer) applyCompareLocalSplit(localSide int, ratio float64, view render.View) {
	flipH, flipV := effectiveViewFlips(view)
	if flipH && (localSide == 1 || localSide == 3) {
		localSide = 4 - localSide
		ratio = 1 - ratio
	}
	if flipV && (localSide == 0 || localSide == 2) {
		localSide = 2 - localSide
		ratio = 1 - ratio
	}
	rotation := ((view.Rotation % 4) + 4) % 4
	screenSide := (localSide + rotation) % 4
	if splitRatioReversedByRotation(rotation, screenSide) {
		ratio = 1 - ratio
	}
	ratio = clampFloat64(ratio, 0, 1)
	v.reverseCompare = screenSide == 0 || screenSide == 3
	if screenSide == 0 || screenSide == 2 {
		v.slider.Orientation = compare.OrientationHorizontal
	} else {
		v.slider.Orientation = compare.OrientationVertical
	}
	rect := render.ImageRectForView(v.windowWidth, v.windowHeight, v.imageA.Width, v.imageA.Height, view)
	if v.slider.Orientation == compare.OrientationHorizontal {
		v.slider.Position = float64(rect.Min.Y) + ratio*float64(rect.Dy())
	} else {
		v.slider.Position = float64(rect.Min.X) + ratio*float64(rect.Dx())
	}
	v.sliderSyncRatio = ratio
}

func effectiveViewFlips(view render.View) (bool, bool) {
	flipH, flipV := view.FlipHorizontal, view.FlipVertical
	if math.Abs(view.MirrorScaleX) > 0.001 {
		flipH = view.MirrorScaleX < 0
	}
	if math.Abs(view.MirrorScaleY) > 0.001 {
		flipV = view.MirrorScaleY < 0
	}
	return flipH, flipV
}

func splitRatioReversedByRotation(rotation, screenSide int) bool {
	return rotation == 2 ||
		(rotation == 1 && (screenSide == 1 || screenSide == 3)) ||
		(rotation == 3 && (screenSide == 0 || screenSide == 2))
}

func (v *Viewer) updateCompareForRotation(_ int, targetRotation int, targetAngle float64) {
	if v.imageB == nil || v.mode != displayModeCompare {
		return
	}
	// A circular mask has no directional side to rotate. Its A/B selection is
	// an explicit user choice and must not inherit the split slider's
	// top/right/bottom/left remapping.
	if v.compareMask == compareMaskCircle {
		return
	}
	localSide, localRatio, ok := v.captureCompareLocalSplit()
	if !ok {
		return
	}
	v.rotationSplitLocalSide = localSide
	v.rotationSplitLocalRatio = localRatio
	futureView := v.view
	futureView.Rotation = targetRotation
	futureView.RotationAngle = targetAngle
	v.applyCompareLocalSplit(localSide, localRatio, futureView)
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
		imageRect := v.currentImageRect()
		v.captureSliderSyncRatio(imageRect)
		return
	}

	v.slider.Position = clampSliderPosition(float64(mouseX), minPosition, maxPosition)
	imageRect := v.currentImageRect()
	v.captureSliderSyncRatio(imageRect)
}

func (v *Viewer) sliderDragBounds() (float64, float64) {
	if v.sliderDragMaxPosition >= v.sliderDragMinPosition {
		return v.sliderDragMinPosition, v.sliderDragMaxPosition
	}

	imageRect := v.currentImageRect()
	if v.slider.Orientation == compare.OrientationHorizontal {
		return float64(imageRect.Min.Y), float64(imageRect.Max.Y - 1)
	}

	return float64(imageRect.Min.X), float64(imageRect.Max.X - 1)
}

func (v *Viewer) cursorIdle(now time.Time) bool {
	return !v.lastCursorActivityAt.IsZero() && now.Sub(v.lastCursorActivityAt) >= cursorIdleDelay
}

func (v *Viewer) updateCursorShape(now time.Time, mouseX, mouseY int, imageRect stdimage.Rectangle, imageReady bool) {
	if v.cursorIdle(now) {
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
		return
	}

	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	if v.draggingImage {
		ebiten.SetCursorShape(ebiten.CursorShapeMove)
		return
	}
	if v.compareMask == compareMaskCircle {
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
		return
	}

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

func (v *Viewer) shouldShowSlider(now time.Time, imageRect stdimage.Rectangle) bool {
	if imageRect.Empty() {
		return false
	}
	if v.compareMask == compareMaskCircle {
		return v.draggingImage || v.draggingSlider || v.lastCompareBorderAt.IsZero() || now.Sub(v.lastCompareBorderAt) < compareBorderIdleDelay
	}
	return v.draggingSlider || !v.splitGuideIdle(now)
}

func (v *Viewer) splitGuideIdle(now time.Time) bool {
	return !v.lastCompareBorderAt.IsZero() && now.Sub(v.lastCompareBorderAt) >= cursorIdleDelay
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

func (v *Viewer) updateCompareGuideVisibility(now time.Time, imageRect stdimage.Rectangle, imageReady bool) {
	if v.compareMask == compareMaskCircle {
		targetOpacity := 0.0
		if v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady && v.shouldShowSlider(now, imageRect) {
			targetOpacity = 1
		}
		v.circleBorderOpacity = approachOpacity(v.circleBorderOpacity, targetOpacity)
		v.sliderOpacity = 0
		return
	}

	v.circleBorderOpacity = 0

	targetOpacity := 0.0
	if v.mode == displayModeCompare && v.imageA != nil && v.imageB != nil && imageReady && v.shouldShowSlider(now, imageRect) {
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

	imageRect := v.currentImageRect()
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

	imageRect := v.currentImageRect()
	if imageRect.Empty() {
		return
	}

	v.applySliderSync(imageRect)
	v.pendingSliderRestore = false
}
