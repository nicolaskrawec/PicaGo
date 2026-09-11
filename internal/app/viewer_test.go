package app

import (
	"bytes"
	"image"
	"math"
	"testing"
	"testing/fstest"
	"time"

	"viewergo/internal/compare"
	"viewergo/internal/i18n"
	imagedata "viewergo/internal/image"
	"viewergo/internal/render"
)

func TestRunPrintsVersionWithoutStartingViewer(t *testing.T) {
	previousVersion := Version
	Version = "1.3.0"
	t.Cleanup(func() { Version = previousVersion })

	for _, argument := range []string{"--version", "-version"} {
		var output bytes.Buffer
		if err := run([]string{argument}, &output); err != nil {
			t.Fatalf("run(%q): %v", argument, err)
		}
		if got, want := output.String(), "PicaGo 1.3.0\n"; got != want {
			t.Fatalf("run(%q) output = %q, want %q", argument, got, want)
		}
	}
}

func TestCursorBecomesIdleAfterDelay(t *testing.T) {
	now := time.Now()
	v := Viewer{}
	if v.cursorIdle(now) {
		t.Fatal("cursor with no recorded activity should remain visible")
	}

	v.lastCursorActivityAt = now
	if v.cursorIdle(now.Add(cursorIdleDelay - time.Millisecond)) {
		t.Fatal("cursor became idle before the delay elapsed")
	}
	if !v.cursorIdle(now.Add(cursorIdleDelay)) {
		t.Fatal("cursor did not become idle after the delay elapsed")
	}

	v.lastCursorActivityAt = now.Add(cursorIdleDelay)
	if v.cursorIdle(now.Add(cursorIdleDelay)) {
		t.Fatal("new mouse activity did not wake the cursor")
	}
}

func TestAdjustmentIndicatorFollowsCursorActivity(t *testing.T) {
	now := time.Now()
	if got := cursorActivityOpacity(now, time.Time{}); got != 0 {
		t.Fatalf("opacity without cursor activity = %v, want 0", got)
	}
	if got := cursorActivityOpacity(now, now.Add(-time.Second)); got != 1 {
		t.Fatalf("opacity while active = %v, want 1", got)
	}
	if got := cursorActivityOpacity(now, now.Add(-cursorIdleDelay+adjustmentIndicatorFadeDuration/2)); math.Abs(got-0.5) > 0.001 {
		t.Fatalf("opacity halfway through fade = %v, want 0.5", got)
	}
	if got := cursorActivityOpacity(now, now.Add(-cursorIdleDelay)); got != 0 {
		t.Fatalf("opacity once idle = %v, want 0", got)
	}
}

func TestAdjustmentIndicatorDetectsCorrections(t *testing.T) {
	if imageAdjustmentsActive(render.View{Gamma: defaultGamma, Contrast: 1}) {
		t.Fatal("neutral image reported as adjusted")
	}
	for _, view := range []render.View{
		{Gamma: 1.1, Contrast: 1},
		{Gamma: 1, Exposure: 0.1, Contrast: 1},
		{Gamma: 1, Contrast: 1.1},
	} {
		if !imageAdjustmentsActive(view) {
			t.Fatalf("correction not detected: %+v", view)
		}
	}
}

func TestAdjustmentIndicatorAvoidsCorners(t *testing.T) {
	for _, point := range [][2]int{{0, 0}, {59, 59}, {940, 0}, {999, 559}} {
		if !pointNearAnyCorner(point[0], point[1], 1000, 560, 60) {
			t.Fatalf("point %v should be inside a corner clearance", point)
		}
	}
	if pointNearAnyCorner(990, 280, 1000, 560, 60) {
		t.Fatal("middle of right edge should remain available")
	}
}

func TestAdjustmentIndicatorHitAreaIsCenteredOnLeftEdge(t *testing.T) {
	rect := adjustmentIndicatorRect(560)
	if !pointInRect(16, 280, rect) {
		t.Fatalf("indicator center is outside hit area: %v", rect)
	}
	if pointInRect(33, 280, rect) || pointInRect(16, 299, rect) {
		t.Fatalf("indicator hit area is too large: %v", rect)
	}
}

func TestImageAdjustmentsSummary(t *testing.T) {
	v := Viewer{localizer: i18n.New("fr")}
	view := render.View{Gamma: 1.2, Exposure: 0, Contrast: 1.4}
	if got, want := v.imageAdjustmentsSummary(view), "Image ajustée : Gamma 1.2   Contraste 1.4"; got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestTruncateOverlayLineUsesRuneLength(t *testing.T) {
	if got := truncateOverlayLine("éééé", 3); got != "éé…" {
		t.Fatalf("truncateOverlayLine = %q, want %q", got, "éé…")
	}
	if got := truncateOverlayLine("short", 10); got != "short" {
		t.Fatalf("short line changed to %q", got)
	}
}

func TestFullscreenDoubleClickToggles100AndPreviousZoom(t *testing.T) {
	previous := render.View{Zoom: 2, OffsetX: 45, OffsetY: -30, Gamma: 1.4, Contrast: 1.2}
	v := Viewer{
		imageA:              &imagedata.LoadedImage{Width: 100, Height: 50},
		borderlessMaximized: true,
		windowWidth:         1000,
		windowHeight:        800,
		view:                previous,
		targetView:          previous,
	}

	v.toggleFullscreen100Zoom(500, 400)
	if !v.fullscreenZoomRestoreValid {
		t.Fatal("100% zoom did not retain the previous view")
	}
	if v.targetView.Zoom != 1 {
		t.Fatalf("double-click target zoom = %v, want 1", v.targetView.Zoom)
	}

	v.toggleFullscreen100Zoom(500, 400)
	if v.fullscreenZoomRestoreValid {
		t.Fatal("restoring the previous zoom left the toggle active")
	}
	if v.targetView.Zoom != previous.Zoom || v.targetView.OffsetX != previous.OffsetX || v.targetView.OffsetY != previous.OffsetY {
		t.Fatalf("restored target = %+v, want zoom and offsets from %+v", v.targetView, previous)
	}
	if v.targetView.Gamma != previous.Gamma || v.targetView.Contrast != previous.Contrast {
		t.Fatalf("restoring zoom changed image adjustments: %+v", v.targetView)
	}
}

func TestSplitGuideFollowsCursorActivityRegardlessOfPosition(t *testing.T) {
	now := time.Now()
	v := Viewer{
		compareMask:         compareMaskSplit,
		lastCompareBorderAt: now,
	}
	imageRect := image.Rect(100, 100, 500, 500)

	if !v.shouldShowSlider(now, imageRect) {
		t.Fatal("split guide should be visible after mouse activity")
	}
	if v.shouldShowSlider(now.Add(cursorIdleDelay), imageRect) {
		t.Fatal("split guide should be hidden when the cursor becomes idle")
	}
}

func TestSplitCommandsWakeGuideWithoutWakingCursor(t *testing.T) {
	v := Viewer{
		compareMask:          compareMaskSplit,
		lastCompareBorderAt:  time.Now().Add(-cursorIdleDelay),
		lastCursorActivityAt: time.Now().Add(-cursorIdleDelay),
	}

	v.setCompareOrientation(compare.OrientationHorizontal)
	if v.splitGuideIdle(time.Now()) {
		t.Fatal("H/V command did not wake the split guide")
	}
	if !v.cursorIdle(time.Now()) {
		t.Fatal("H/V command should not wake the mouse cursor")
	}
}

func TestCircleCompareRotationPreservesImageSelection(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		v := Viewer{
			imageB:         &imagedata.LoadedImage{},
			mode:           displayModeCompare,
			compareMask:    compareMaskCircle,
			reverseCompare: reversed,
		}

		v.updateCompareForRotation(-1, 3, -1)
		if v.reverseCompare != reversed {
			t.Fatalf("-90 degree rotation changed reverseCompare from %v to %v", reversed, v.reverseCompare)
		}

		v.updateCompareForRotation(-2, 2, -2)
		if v.reverseCompare != reversed {
			t.Fatalf("-180 degree rotation changed reverseCompare from %v to %v", reversed, v.reverseCompare)
		}
	}
}

func TestSplitSliderMirrorsWithImage(t *testing.T) {
	tests := []struct {
		name        string
		orientation compare.Orientation
		position    float64
		horizontal  bool
		want        float64
		wantReverse bool
	}{
		{"vertical split follows horizontal mirror", compare.OrientationVertical, 25, true, 75, true},
		{"horizontal split follows vertical mirror", compare.OrientationHorizontal, 20, false, 80, true},
		{"vertical split ignores vertical mirror", compare.OrientationVertical, 25, false, 25, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := Viewer{
				imageA:       &imagedata.LoadedImage{Width: 100, Height: 100},
				imageB:       &imagedata.LoadedImage{Width: 100, Height: 100},
				mode:         displayModeCompare,
				compareMask:  compareMaskSplit,
				windowWidth:  100,
				windowHeight: 100,
				view:         render.View{Zoom: 1},
				targetView:   render.View{Zoom: 1},
				slider:       compare.Slider{Orientation: test.orientation, Position: test.position},
			}
			v.startMirrorAnimation(test.horizontal)
			if v.slider.Position != test.want {
				t.Fatalf("slider position = %v, want %v", v.slider.Position, test.want)
			}
			if v.reverseCompare != test.wantReverse {
				t.Fatalf("reverseCompare = %v, want %v", v.reverseCompare, test.wantReverse)
			}
		})
	}
}

func TestRotationAfterMirrorKeepsMaskInLocalPosition(t *testing.T) {
	v := Viewer{
		imageA:       &imagedata.LoadedImage{Width: 100, Height: 100},
		imageB:       &imagedata.LoadedImage{Width: 100, Height: 100},
		mode:         displayModeCompare,
		compareMask:  compareMaskSplit,
		windowWidth:  100,
		windowHeight: 100,
		view:         render.View{Zoom: 1},
		targetView:   render.View{Zoom: 1},
		slider:       compare.Slider{Orientation: compare.OrientationVertical, Position: 25},
	}

	v.startMirrorAnimation(true)
	// Complete the mirror before starting the rotation, as animateView would.
	v.view.MirrorScaleX = -1
	v.targetView.MirrorScaleX = -1
	v.mirrorAnimationActive = false
	v.updateCompareForRotation(1, 1, 1)

	if v.rotationSplitLocalSide != 1 || v.rotationSplitLocalRatio != 0.25 {
		t.Fatalf("animated mask local split = side %d ratio %v, want side 1 ratio 0.25", v.rotationSplitLocalSide, v.rotationSplitLocalRatio)
	}
}

func TestScreenMirrorSwapsAxesAfterQuarterTurn(t *testing.T) {
	tests := []struct {
		rotation          int
		requestHorizontal bool
		wantHorizontal    bool
	}{
		{0, true, true},
		{0, false, false},
		{1, true, false},
		{1, false, true},
		{2, true, true},
		{3, true, false},
	}
	for _, test := range tests {
		v := Viewer{view: render.View{Rotation: test.rotation}, targetView: render.View{Rotation: test.rotation}}
		v.toggleScreenMirror(test.requestHorizontal)
		if v.mirrorAnimationHorizontal != test.wantHorizontal {
			t.Fatalf("rotation %d, horizontal request %v mapped to horizontal=%v, want %v", test.rotation, test.requestHorizontal, v.mirrorAnimationHorizontal, test.wantHorizontal)
		}
	}
}

func TestSoloPreviewRestoresPreviousModeOnRelease(t *testing.T) {
	v := Viewer{
		imageA: &imagedata.LoadedImage{},
		imageB: &imagedata.LoadedImage{},
		mode:   displayModeCompare,
	}

	v.updateSoloPreview(true, false)
	if v.mode != displayModeSingleA || !v.soloPreviewActive {
		t.Fatalf("holding 1: mode=%v active=%v", v.mode, v.soloPreviewActive)
	}
	v.updateSoloPreview(false, false)
	if v.mode != displayModeCompare || v.soloPreviewActive {
		t.Fatalf("releasing 1: mode=%v active=%v, want compare and inactive", v.mode, v.soloPreviewActive)
	}

	v.updateSoloPreview(false, true)
	if v.mode != displayModeSingleB {
		t.Fatalf("holding 2: mode=%v, want single B", v.mode)
	}
	v.updateSoloPreview(false, false)
	if v.mode != displayModeCompare {
		t.Fatalf("releasing 2: mode=%v, want compare", v.mode)
	}
}

func TestSoloPreviewHandlesBothKeysAndMissingImageB(t *testing.T) {
	v := Viewer{imageA: &imagedata.LoadedImage{}, mode: displayModeCompare}
	v.updateSoloPreview(false, true)
	if v.soloPreviewActive || v.mode != displayModeCompare {
		t.Fatalf("missing B started preview: mode=%v active=%v", v.mode, v.soloPreviewActive)
	}

	v.imageB = &imagedata.LoadedImage{}
	v.updateSoloPreview(true, true)
	if v.mode != displayModeSingleB {
		t.Fatalf("both keys: mode=%v, want single B", v.mode)
	}
	v.updateSoloPreview(false, false)
	if v.mode != displayModeCompare {
		t.Fatalf("release after both keys: mode=%v, want compare", v.mode)
	}
}

func TestImageDropSlotUsesWindowCenter(t *testing.T) {
	v := Viewer{windowWidth: 1000}
	if got := v.imageDropSlot(999); got != asyncImageSlotA {
		t.Fatalf("drop without image A targets slot %v, want A", got)
	}

	v.imageA = &imagedata.LoadedImage{}
	tests := []struct {
		x    int
		want asyncImageSlot
	}{
		{0, asyncImageSlotA},
		{499, asyncImageSlotA},
		{500, asyncImageSlotB},
		{999, asyncImageSlotB},
	}
	for _, test := range tests {
		if got := v.imageDropSlot(test.x); got != test.want {
			t.Errorf("drop at x=%d targets slot %v, want %v", test.x, got, test.want)
		}
	}
}

func TestTwoImageDropPreparesCenteredVerticalComparison(t *testing.T) {
	v := Viewer{
		compareMask:       compareMaskCircle,
		reverseCompare:    true,
		sliderInitialized: true,
		slider: compare.Slider{
			Orientation: compare.OrientationHorizontal,
			Position:    123,
		},
	}

	v.prepareTwoImageDropComparison()

	if v.compareMask != compareMaskSplit {
		t.Fatalf("comparison mask = %v, want split", v.compareMask)
	}
	if v.slider.Orientation != compare.OrientationVertical {
		t.Fatalf("slider orientation = %v, want vertical", v.slider.Orientation)
	}
	if v.sliderInitialized || v.slider.Position != 0 {
		t.Fatalf("slider was not reset for centering: %+v", v.slider)
	}
	if v.reverseCompare {
		t.Fatal("grouped drop kept the reversed comparison")
	}
	if !v.centerComparisonOnNextImageA {
		t.Fatal("grouped drop did not schedule centering for the asynchronously loaded image A")
	}
}

func TestDroppedFolderNavigationIncludesOnlyDirectImages(t *testing.T) {
	dropped := fstest.MapFS{
		"album/z.png":         {},
		"album/a.jpg":         {},
		"album/notes.txt":     {},
		"album/nested/b.webp": {},
	}
	v := Viewer{folderSortModes: make(map[string]imageSortMode)}

	images := v.setDroppedFolderNavigation(dropped, "album")
	want := []string{"album/a.jpg", "album/z.png"}
	if len(images) != len(want) {
		t.Fatalf("folder images = %v, want %v", images, want)
	}
	for i := range want {
		if images[i] != want[i] {
			t.Fatalf("folder images = %v, want %v", images, want)
		}
	}
	paths, current, err := v.navigationImagePaths("album/z.png")
	if err != nil || current != 1 || len(paths) != 2 {
		t.Fatalf("dropped navigation paths = %v, current = %d, err = %v", paths, current, err)
	}
}

func TestLoadedImageResetPreservesMousePressLatch(t *testing.T) {
	v := Viewer{
		leftMouseDown:       true,
		draggingImage:       true,
		draggingSlider:      true,
		restoreClickPending: true,
	}
	v.resetInteractionForLoadedImage()
	if !v.leftMouseDown {
		t.Fatal("an asynchronous image load cleared the active mouse press")
	}
	if v.draggingImage || v.draggingSlider || v.restoreClickPending {
		t.Fatalf("loaded-image interaction state was not reset: %+v", v)
	}
}

func TestAdjustCompareMaskAlphaClamps(t *testing.T) {
	v := Viewer{compareMaskAlpha: 0.5}
	v.adjustCompareMaskAlpha(2)
	if v.compareMaskAlpha != 0.7 {
		t.Fatalf("alpha after wheel up = %v, want 0.7", v.compareMaskAlpha)
	}
	v.adjustCompareMaskAlpha(10)
	if v.compareMaskAlpha != 1 {
		t.Fatalf("alpha upper clamp = %v, want 1", v.compareMaskAlpha)
	}
	v.adjustCompareMaskAlpha(-20)
	if v.compareMaskAlpha != 0 {
		t.Fatalf("alpha lower clamp = %v, want 0", v.compareMaskAlpha)
	}
}

func TestPointInBottomLeftCorner(t *testing.T) {
	if !pointInBottomLeftCorner(0, 699, 700, cornerCommandTolerance) {
		t.Fatal("bottom-left pixel is not inside the horizontal mirror corner")
	}
	if pointInBottomLeftCorner(cornerCommandTolerance, 699, 700, cornerCommandTolerance) {
		t.Fatal("pixel to the right of the gamma corner is inside it")
	}
	if pointInBottomLeftCorner(0, 700-cornerCommandTolerance-1, 700, cornerCommandTolerance) {
		t.Fatal("pixel above the gamma corner is inside it")
	}
}

func TestPointInBottomRightCorner(t *testing.T) {
	if !pointInBottomRightCorner(999, 699, 1000, 700, cornerCommandTolerance) {
		t.Fatal("bottom-right pixel is not inside the gamma corner")
	}
	if pointInBottomRightCorner(1000-cornerCommandTolerance-1, 699, 1000, 700, cornerCommandTolerance) {
		t.Fatal("pixel to the left of the gamma corner is inside it")
	}
	if pointInBottomRightCorner(999, 700-cornerCommandTolerance-1, 1000, 700, cornerCommandTolerance) {
		t.Fatal("pixel above the gamma corner is inside it")
	}
}

func TestResetViewParametersRestoresDefaults(t *testing.T) {
	v := Viewer{
		imageA:       &imagedata.LoadedImage{Width: 100, Height: 50},
		windowWidth:  1000,
		windowHeight: 800,
		view: render.View{
			Zoom: 3, OffsetX: 42, OffsetY: -17, Alpha: 0.4,
			FlipHorizontal: true, FlipVertical: true, Rotation: 2,
			RotationAngle: 2, MirrorScaleX: -1, MirrorScaleY: -1,
			Gamma: 2, Exposure: 1, Contrast: 2,
		},
		targetView: render.View{Zoom: 3, Gamma: 2, Contrast: 2},
	}

	v.resetViewParameters()
	if v.targetView.Zoom != render.FitZoom(1000, 800, 100, 50) ||
		v.targetView.OffsetX != 0 || v.targetView.OffsetY != 0 ||
		v.targetView.Alpha != 1 || v.targetView.FlipHorizontal || v.targetView.FlipVertical ||
		v.targetView.Rotation != 0 || v.targetView.RotationAngle != 0 ||
		v.targetView.MirrorScaleX != 0 || v.targetView.MirrorScaleY != 0 ||
		v.targetView.Gamma != defaultGamma || v.targetView.Exposure != 0 || v.targetView.Contrast != 1 {
		t.Fatalf("reset target view = %+v", v.targetView)
	}
	if !viewAlmostEqual(v.view, v.targetView) || v.viewAnimationActive {
		t.Fatalf("reset should be instantaneous: view=%+v target=%+v animation=%v", v.view, v.targetView, v.viewAnimationActive)
	}
}

func TestSessionViewIsStoredInMemoryPerImage(t *testing.T) {
	const imagePath = `C:\images\one.png`
	v := Viewer{
		imageA:     &imagedata.LoadedImage{FilePath: imagePath},
		targetView: render.View{Zoom: 2.5, OffsetX: 42, OffsetY: -17},
		fitMode:    false,
	}

	v.rememberCurrentImageView()
	got := v.sessionImageViews[imageViewStateKey(imagePath)]
	if got.Zoom != 2.5 || got.OffsetX != 42 || got.OffsetY != -17 {
		t.Fatalf("stored session view = %+v, want zoom 2.5 and offsets 42/-17", got)
	}
	if _, exists := v.imageViewStates[imageViewStateKey(imagePath)]; exists {
		t.Fatal("session zoom should not be added to persisted image state")
	}

	v.fitMode = true
	v.targetView.Zoom = 1.0
	v.rememberCurrentImageView()
	if _, exists := v.sessionImageViews[imageViewStateKey(imagePath)]; exists {
		t.Fatal("fit mode should clear the session view")
	}
}
