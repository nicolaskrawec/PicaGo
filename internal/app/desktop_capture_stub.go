//go:build !windows

package app

import "github.com/hajimehoshi/ebiten/v2"

func captureDesktopBackdrop() *ebiten.Image { return nil }
