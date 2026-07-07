package input

import "github.com/hajimehoshi/ebiten/v2"

type DragState struct {
	Active bool
	X      int
	Y      int
}

func WheelDelta() float64 {
	_, dy := ebiten.Wheel()
	return dy
}
