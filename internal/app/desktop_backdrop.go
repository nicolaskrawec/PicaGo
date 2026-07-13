package app

import (
	"github.com/hajimehoshi/ebiten/v2"
)

func drawDesktopBackdrop(screen, backdrop *ebiten.Image) {
	if backdrop == nil {
		return
	}

	bounds := backdrop.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(screen.Bounds().Dx())/float64(bounds.Dx()), float64(screen.Bounds().Dy())/float64(bounds.Dy()))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(backdrop, op)
}
