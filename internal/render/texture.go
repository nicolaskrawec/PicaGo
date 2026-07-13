package render

import (
	stdimage "image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

func drawTexture(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma float64) {
	if texture == nil || destRect.Empty() {
		return
	}

	options := &ebiten.DrawImageOptions{}
	options.Filter = ebiten.FilterLinear
	options.ColorScale.ScaleAlpha(float32(clampAlpha(alpha)))
	radians := rotation * math.Pi / 2
	rotatedWidth := float64(texture.Bounds().Dx())*math.Abs(math.Cos(radians)) + float64(texture.Bounds().Dy())*math.Abs(math.Sin(radians))
	scale := float64(destRect.Dx()) / math.Max(1, rotatedWidth)
	scaleX, scaleY := scale, scale
	scaleXSign, scaleYSign := 1.0, 1.0
	translateX, translateY := float64(destRect.Min.X), float64(destRect.Min.Y)
	if flipHorizontal {
		scaleXSign = -1
		translateX = float64(destRect.Max.X)
	}
	if flipVertical {
		scaleYSign = -1
		translateY = float64(destRect.Max.Y)
	}
	if mirrorScaleX != 0 {
		scaleXSign = mirrorScaleX
		translateX = float64(destRect.Min.X+destRect.Max.X)/2 - float64(destRect.Dx())*mirrorScaleX/2
	}
	if mirrorScaleY != 0 {
		scaleYSign = mirrorScaleY
		translateY = float64(destRect.Min.Y+destRect.Max.Y)/2 - float64(destRect.Dy())*mirrorScaleY/2
	}
	if math.Abs(rotation) > 0.001 {
		// Rotate around the center of the destination rectangle. For odd
		// quarter turns destRect already has swapped dimensions.
		options.GeoM.Translate(-float64(texture.Bounds().Dx())/2, -float64(texture.Bounds().Dy())/2)
		options.GeoM.Scale(scaleX*scaleXSign, scaleY*scaleYSign)
		options.GeoM.Rotate(rotation * math.Pi / 2)
		options.GeoM.Translate(float64(destRect.Min.X+destRect.Max.X)/2, float64(destRect.Min.Y+destRect.Max.Y)/2)
	} else {
		options.GeoM.Scale(scaleX*scaleXSign, scaleY*scaleYSign)
		options.GeoM.Translate(translateX, translateY)
	}

	shader, uniforms := gammaDrawOptions(gamma)
	if shader != nil {
		drawTexturedQuadShader(screen, texture, destRect, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, shader, uniforms)
		return
	}
	screen.DrawImage(texture, options)
}

func drawTexturedQuadShader(screen, texture *ebiten.Image, destRect stdimage.Rectangle, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha float64, shader *ebiten.Shader, uniforms map[string]any) {
	points := [][2]int{{destRect.Min.X, destRect.Min.Y}, {destRect.Max.X, destRect.Min.Y}, {destRect.Min.X, destRect.Max.Y}, {destRect.Max.X, destRect.Max.Y}}
	vertices := make([]ebiten.Vertex, 4)
	for i, point := range points {
		sourceX, sourceY := sourcePoint(point[0], point[1], destRect, texture.Bounds(), flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY)
		vertices[i] = ebiten.Vertex{DstX: float32(point[0]), DstY: float32(point[1]), SrcX: float32(sourceX), SrcY: float32(sourceY), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: float32(clampAlpha(alpha))}
	}
	screen.DrawTrianglesShader(vertices, []uint16{0, 1, 2, 1, 2, 3}, shader, &ebiten.DrawTrianglesShaderOptions{Images: [4]*ebiten.Image{texture}, Uniforms: uniforms})
}

func drawTextureClipped(screen *ebiten.Image, texture *ebiten.Image, destRect, visibleRect stdimage.Rectangle, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma float64) {
	if texture == nil || destRect.Empty() || visibleRect.Empty() {
		return
	}

	texBounds := texture.Bounds()
	leftTopSrcX, leftTopSrcY := sourcePoint(visibleRect.Min.X, visibleRect.Min.Y, destRect, texBounds, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY)
	rightTopSrcX, rightTopSrcY := sourcePoint(visibleRect.Max.X, visibleRect.Min.Y, destRect, texBounds, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY)
	leftBottomSrcX, leftBottomSrcY := sourcePoint(visibleRect.Min.X, visibleRect.Max.Y, destRect, texBounds, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY)
	rightBottomSrcX, rightBottomSrcY := sourcePoint(visibleRect.Max.X, visibleRect.Max.Y, destRect, texBounds, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY)
	vertexAlpha := float32(clampAlpha(alpha))

	vertices := []ebiten.Vertex{
		{
			DstX:   float32(visibleRect.Min.X),
			DstY:   float32(visibleRect.Min.Y),
			SrcX:   float32(leftTopSrcX),
			SrcY:   float32(leftTopSrcY),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Max.X),
			DstY:   float32(visibleRect.Min.Y),
			SrcX:   float32(rightTopSrcX),
			SrcY:   float32(rightTopSrcY),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Min.X),
			DstY:   float32(visibleRect.Max.Y),
			SrcX:   float32(leftBottomSrcX),
			SrcY:   float32(leftBottomSrcY),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Max.X),
			DstY:   float32(visibleRect.Max.Y),
			SrcX:   float32(rightBottomSrcX),
			SrcY:   float32(rightBottomSrcY),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
	}
	indices := []uint16{0, 1, 2, 1, 2, 3}
	options := &ebiten.DrawTrianglesOptions{
		Filter: ebiten.FilterLinear,
	}
	shader, uniforms := gammaDrawOptions(gamma)
	if shader != nil {
		screen.DrawTrianglesShader(vertices, indices, shader, &ebiten.DrawTrianglesShaderOptions{Images: [4]*ebiten.Image{texture}, Uniforms: uniforms})
	} else {
		screen.DrawTriangles(vertices, indices, texture, options)
	}
}

func clampBoundary(value, min, max int) int {
	if value < min {
		return min
	}
	if max < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampFloat32(value, minValue, maxValue float32) float32 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func mapRange(value, inMin, inMax, outMin, outMax int, reverse bool) float64 {
	if inMax == inMin {
		return float64(outMin)
	}

	t := float64(value-inMin) / float64(inMax-inMin)
	if reverse {
		t = 1 - t
	}
	return float64(outMin) + t*float64(outMax-outMin)
}

func sourcePoint(x, y int, destRect, texRect stdimage.Rectangle, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY float64) (float64, float64) {
	angle := rotation * math.Pi / 2
	cosine := math.Cos(angle)
	sine := math.Sin(angle)
	rotatedWidth := float64(texRect.Dx())*math.Abs(cosine) + float64(texRect.Dy())*math.Abs(sine)
	scale := float64(destRect.Dx()) / math.Max(1, rotatedWidth)
	deltaX := (float64(x) - float64(destRect.Min.X+destRect.Max.X)/2) / scale
	deltaY := (float64(y) - float64(destRect.Min.Y+destRect.Max.Y)/2) / scale
	sourceX := cosine*deltaX + sine*deltaY
	sourceY := -sine*deltaX + cosine*deltaY

	mirrorX, mirrorY := 1.0, 1.0
	if flipHorizontal {
		mirrorX = -1
	}
	if flipVertical {
		mirrorY = -1
	}
	if mirrorScaleX != 0 {
		mirrorX = mirrorScaleX
	}
	if mirrorScaleY != 0 {
		mirrorY = mirrorScaleY
	}
	if math.Abs(mirrorX) > 0.001 {
		sourceX /= mirrorX
	}
	if math.Abs(mirrorY) > 0.001 {
		sourceY /= mirrorY
	}
	return float64(texRect.Min.X) + sourceX + float64(texRect.Dx())/2,
		float64(texRect.Min.Y) + sourceY + float64(texRect.Dy())/2
}

func clampAlpha(alpha float64) float64 {
	if alpha <= 0 {
		return 0
	}
	if alpha >= 1 {
		return 1
	}
	return alpha
}

func clampFloat64(value, minValue, maxValue float64) float64 {
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
