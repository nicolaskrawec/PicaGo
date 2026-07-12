package app

import (
	"testing"

	"viewergo/internal/compare"
	imagedata "viewergo/internal/image"
	"viewergo/internal/render"
)

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
