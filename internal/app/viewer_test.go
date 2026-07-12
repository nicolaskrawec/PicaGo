package app

import (
	"testing"

	imagedata "viewergo/internal/image"
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
