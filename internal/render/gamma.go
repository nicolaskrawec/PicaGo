package render

import (
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

const gammaShaderSource = `
package main

var Gamma float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	fragment := imageSrc0At(texCoord)
	correction := vec3(1.0 / Gamma)
	fragment.rgb = pow(fragment.rgb, correction)
	return fragment * color
}
`

var gammaShaderOnce sync.Once
var gammaShader *ebiten.Shader

func gammaDrawOptions(gamma float64) (*ebiten.Shader, map[string]any) {
	if gamma <= 0 {
		gamma = 1
	}
	if math.Abs(gamma-1) < 0.0001 {
		return nil, nil
	}

	gammaShaderOnce.Do(func() {
		gammaShader, _ = ebiten.NewShader([]byte(gammaShaderSource))
	})
	if gammaShader == nil {
		return nil, nil
	}
	return gammaShader, map[string]any{"Gamma": float32(clampGamma(gamma))}
}

func clampGamma(gamma float64) float64 {
	if gamma < 0.1 {
		return 0.1
	}
	if gamma > 4 {
		return 4
	}
	return gamma
}
