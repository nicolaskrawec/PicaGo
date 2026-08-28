package input

import (
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
)

type DragState struct {
	Active bool
	X      int
	Y      int
}

func WheelDelta() float64 {
	_, dy := ebiten.Wheel()
	if runtime.GOOS == "js" {
		// Browsers report wheel movement in CSS pixels (typically +/-100 per
		// mouse-wheel notch), while Ebiten reports desktop wheel notches as
		// roughly +/-1. Keep the desktop behavior unchanged and convert only
		// the WebAssembly input to the same scale.
		dy /= 100
	}
	return dy
}
