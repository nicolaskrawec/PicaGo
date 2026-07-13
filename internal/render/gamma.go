package render

import (
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

const gammaShaderSource = `
//kage:unit pixels

package main

var Gamma float
var Exposure float
var Contrast float

func sampleLinear(position vec2) vec4 {
	origin := imageSrc0Origin()
	size := imageSrc0Size()
	last := origin + size - vec2(1)
	position = position - vec2(0.5)
	base := clamp(floor(position), origin, last)
	next := min(base+vec2(1), last)
	weight := fract(position)
	top := mix(imageSrc0At(base), imageSrc0At(vec2(next.x, base.y)), weight.x)
	bottom := mix(imageSrc0At(vec2(base.x, next.y)), imageSrc0At(next), weight.x)
	return mix(top, bottom, weight.y)
}

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	fragment := sampleLinear(texCoord)
	fragment.rgb *= exp2(vec3(Exposure))
	fragment.rgb = (fragment.rgb - vec3(0.5)) * Contrast + vec3(0.5)
	correction := vec3(1.0 / Gamma)
	fragment.rgb = pow(fragment.rgb, correction)
	return fragment * color
}
`

var gammaShaderOnce sync.Once
var gammaShader *ebiten.Shader

func gammaDrawOptions(gamma, exposure, contrast float64) (*ebiten.Shader, map[string]any) {
	if gamma <= 0 {
		gamma = 1
	}
	if math.Abs(gamma-1) < 0.0001 && math.Abs(exposure) < 0.0001 && math.Abs(contrast-1) < 0.0001 {
		return nil, nil
	}

	gammaShaderOnce.Do(func() {
		gammaShader, _ = ebiten.NewShader([]byte(gammaShaderSource))
	})
	if gammaShader == nil {
		return nil, nil
	}
	return gammaShader, map[string]any{
		"Gamma":    float32(clampGamma(gamma)),
		"Exposure": float32(exposure),
		"Contrast": float32(clampContrast(contrast)),
	}
}

func clampContrast(contrast float64) float64 {
	if contrast <= 0 {
		return 1
	}
	if contrast > 4 {
		return 4
	}
	return contrast
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
