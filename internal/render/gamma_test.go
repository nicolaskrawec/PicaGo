package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestGammaShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(gammaShaderSource)); err != nil {
		t.Fatalf("gamma shader failed to compile: %v", err)
	}
}
