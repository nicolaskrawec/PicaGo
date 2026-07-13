package render

import (
	stdimage "image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	imagedata "viewergo/internal/image"
)

// DrawComparisonImage draws B with the exact scale and center used by the
// comparison modes. It is used by the temporary solo-B preview so switching
// between compare and B does not introduce a visual zoom jump.
func DrawComparisonImage(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, showShadow bool) {
	if imageB == nil || imageB.GPUTexture == nil {
		return
	}
	if imageA == nil {
		DrawImage(screen, imageB, windowWidth, windowHeight, view, showShadow)
		return
	}
	rectA := ImageRectForView(windowWidth, windowHeight, imageA.Width, imageA.Height, view)
	rectB := compareRectForView(rectA, imageA.Width, imageA.Height, imageB.Width, imageB.Height, view)
	if showShadow && !imageB.HasTransparency {
		drawShadow(screen, imageB.Width, imageB.Height, windowWidth, windowHeight, rectB, view)
	}
	drawTexture(screen, imageB.GPUTexture, rectB, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, view.Alpha, view.Gamma, view.Exposure, view.Contrast)
}

func DrawCompare(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, sliderPosition float64, orientation int, feathered bool, reverse bool, maskAlpha, sliderOpacity float64, showShadow bool) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view, showShadow)
	if imageA == nil || imageB == nil || imageB.GPUTexture == nil {
		return
	}

	rectA := ImageRectForView(windowWidth, windowHeight, imageA.Width, imageA.Height, view)
	rectB := compareRectForView(rectA, imageA.Width, imageA.Height, imageB.Width, imageB.Height, view)
	maskView := view
	maskView.Alpha *= clampAlpha(maskAlpha)
	// Once rotated, clipping in B's local coordinates makes the mask boundary
	// depend on B's aspect ratio. Clip the already transformed B quad against
	// the exact screen-space slider line instead, so mask and line coincide.
	if math.Abs(view.RotationAngle) > 0.001 || view.MirrorScaleX != 0 || view.MirrorScaleY != 0 {
		if orientation == 1 {
			y := float32(clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.Y, rectA.Max.Y-1))
			keepY := float32(rectA.Max.Y)
			if reverse {
				keepY = float32(rectA.Min.Y)
			}
			drawTransformedImageClippedByLine(screen, imageB.GPUTexture, rectB, float32(rectA.Min.X), y, float32(rectA.Max.X), y, float32(rectA.Min.X), keepY, maskView)
			if sliderOpacity > 0 {
				vector.StrokeLine(screen, float32(rectA.Min.X), y, float32(rectA.Max.X), y, 2, color.NRGBA{230, 230, 230, uint8(math.Round(96 * clampAlpha(sliderOpacity)))}, true)
			}
		} else {
			x := float32(clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.X, rectA.Max.X-1))
			keepX := float32(rectA.Max.X)
			if reverse {
				keepX = float32(rectA.Min.X)
			}
			drawTransformedImageClippedByLine(screen, imageB.GPUTexture, rectB, x, float32(rectA.Min.Y), x, float32(rectA.Max.Y), keepX, float32(rectA.Min.Y), maskView)
			if sliderOpacity > 0 {
				vector.StrokeLine(screen, x, float32(rectA.Min.Y), x, float32(rectA.Max.Y), 2, color.NRGBA{230, 230, 230, uint8(math.Round(96 * clampAlpha(sliderOpacity)))}, true)
			}
		}
		return
	}
	switch orientation {
	case 1:
		visibleTop := clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.Y, rectA.Max.Y-1)
		if visibleTop < rectA.Min.Y {
			return
		}
		if feathered {
			drawFeatheredCompareImage(screen, imageB.GPUTexture, rectB, rectA, visibleTop, true, reverse, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, maskView.Alpha, view.Gamma, view.Exposure, view.Contrast)
		} else {
			drawClippedCompareImage(screen, imageB.GPUTexture, rectB, rectA, visibleTop, true, reverse, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, maskView.Alpha, view.Gamma, view.Exposure, view.Contrast)
		}
		if sliderOpacity > 0 {
			vector.FillRect(screen, float32(rectA.Min.X), float32(visibleTop)-1, float32(rectA.Max.X-rectA.Min.X), 2, color.NRGBA{230, 230, 230, uint8(math.Round(96 * clampAlpha(sliderOpacity)))}, true)
		}
	default:
		visibleLeft := clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.X, rectA.Max.X-1)
		if visibleLeft < rectA.Min.X {
			return
		}
		if feathered {
			drawFeatheredCompareImage(screen, imageB.GPUTexture, rectB, rectA, visibleLeft, false, reverse, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, maskView.Alpha, view.Gamma, view.Exposure, view.Contrast)
		} else {
			drawClippedCompareImage(screen, imageB.GPUTexture, rectB, rectA, visibleLeft, false, reverse, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, maskView.Alpha, view.Gamma, view.Exposure, view.Contrast)
		}
		if sliderOpacity > 0 {
			vector.FillRect(screen, float32(visibleLeft)-1, float32(rectA.Min.Y), 2, float32(rectA.Max.Y-rectA.Min.Y), color.NRGBA{230, 230, 230, uint8(math.Round(96 * clampAlpha(sliderOpacity)))}, true)
		}
	}
}

// DrawCompareRotating keeps the split in image-local coordinates. Both the
// clipped image and its separator therefore use the exact same continuously
// animated transform as the base image instead of snapping at quarter turns.
func DrawCompareRotating(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, localSide int, ratio, maskAlpha, sliderOpacity float64, showShadow bool) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view, showShadow)
	if imageA == nil || imageB == nil || imageB.GPUTexture == nil {
		return
	}
	ratio = clampFloat64(ratio, 0, 1)
	rectA := ImageRectForView(windowWidth, windowHeight, imageA.Width, imageA.Height, view)
	rectB := compareRectForView(rectA, imageA.Width, imageA.Height, imageB.Width, imageB.Height, view)
	u0, v0, u1, v1 := ratio, 0.0, ratio, 1.0
	keepU, keepV := .5, .5
	if localSide == 0 || localSide == 2 {
		u0, v0, u1, v1 = 0, ratio, 1, ratio
	}
	switch localSide {
	case 0:
		keepV = 0
	case 1:
		keepU = 1
	case 2:
		keepV = 1
	case 3:
		keepU = 0
	}
	x0, y0 := transformedLocalPoint(u0, v0, imageA.Width, imageA.Height, rectA, view)
	x1, y1 := transformedLocalPoint(u1, v1, imageA.Width, imageA.Height, rectA, view)
	keepX, keepY := transformedLocalPoint(keepU, keepV, imageA.Width, imageA.Height, rectA, view)
	maskView := view
	maskView.Alpha *= clampAlpha(maskAlpha)
	drawTransformedImageClippedByLine(screen, imageB.GPUTexture, rectB, x0, y0, x1, y1, keepX, keepY, maskView)
	if sliderOpacity <= 0 {
		return
	}
	alpha := uint8(math.Round(96 * clampAlpha(sliderOpacity)))
	vector.StrokeLine(screen, x0, y0, x1, y1, 2, color.NRGBA{230, 230, 230, alpha}, true)
}

type clippedVertex struct {
	x, y, u, v float32
}

func drawTransformedImageClippedByLine(screen, texture *ebiten.Image, destRect stdimage.Rectangle, x0, y0, x1, y1, keepX, keepY float32, view View) {
	if texture == nil || destRect.Empty() {
		return
	}
	b := texture.Bounds()
	point := func(u, v float64) clippedVertex {
		x, y := transformedLocalPoint(u, v, b.Dx(), b.Dy(), destRect, view)
		return clippedVertex{x, y, float32(b.Min.X) + float32(u)*float32(b.Dx()), float32(b.Min.Y) + float32(v)*float32(b.Dy())}
	}
	polygon := []clippedVertex{point(0, 0), point(1, 0), point(1, 1), point(0, 1)}
	side := func(x, y float32) float32 { return (x1-x0)*(y-y0) - (y1-y0)*(x-x0) }
	keepSign := side(keepX, keepY)
	inside := func(p clippedVertex) bool { return side(p.x, p.y)*keepSign >= -0.001 }
	result := make([]clippedVertex, 0, 6)
	for i, current := range polygon {
		previous := polygon[(i+len(polygon)-1)%len(polygon)]
		currentInside, previousInside := inside(current), inside(previous)
		if currentInside != previousInside {
			d0, d1 := side(previous.x, previous.y), side(current.x, current.y)
			t := d0 / (d0 - d1)
			result = append(result, clippedVertex{
				previous.x + t*(current.x-previous.x), previous.y + t*(current.y-previous.y),
				previous.u + t*(current.u-previous.u), previous.v + t*(current.v-previous.v),
			})
		}
		if currentInside {
			result = append(result, current)
		}
	}
	if len(result) < 3 {
		return
	}
	vertices := make([]ebiten.Vertex, len(result))
	for i, p := range result {
		vertices[i] = ebiten.Vertex{DstX: p.x, DstY: p.y, SrcX: p.u, SrcY: p.v, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: float32(clampAlpha(view.Alpha))}
	}
	indices := make([]uint16, 0, (len(result)-2)*3)
	for i := 1; i+1 < len(result); i++ {
		indices = append(indices, 0, uint16(i), uint16(i+1))
	}
	shader, uniforms := gammaDrawOptions(view.Gamma, view.Exposure, view.Contrast)
	if shader != nil {
		screen.DrawTrianglesShader(vertices, indices, shader, &ebiten.DrawTrianglesShaderOptions{Images: [4]*ebiten.Image{texture}, Uniforms: uniforms})
	} else {
		screen.DrawTriangles(vertices, indices, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	}
}

func transformedLocalPoint(u, v float64, width, height int, destRect stdimage.Rectangle, view View) (float32, float32) {
	angle := view.RotationAngle * math.Pi / 2
	cosine, sine := math.Cos(angle), math.Sin(angle)
	rotatedWidth := float64(width)*math.Abs(cosine) + float64(height)*math.Abs(sine)
	scale := float64(destRect.Dx()) / math.Max(1, rotatedWidth)
	mirrorX, mirrorY := 1.0, 1.0
	if view.FlipHorizontal {
		mirrorX = -1
	}
	if view.FlipVertical {
		mirrorY = -1
	}
	if view.MirrorScaleX != 0 {
		mirrorX = view.MirrorScaleX
	}
	if view.MirrorScaleY != 0 {
		mirrorY = view.MirrorScaleY
	}
	x := (u - .5) * float64(width) * mirrorX
	y := (v - .5) * float64(height) * mirrorY
	cx := float64(destRect.Min.X+destRect.Max.X) / 2
	cy := float64(destRect.Min.Y+destRect.Max.Y) / 2
	return float32(cx + scale*(cosine*x-sine*y)), float32(cy + scale*(sine*x+cosine*y))
}

func DrawCompareCircle(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, centerX, centerY int, diameterRatio float64, feathered bool, reverse bool, maskAlpha, sliderOpacity float64, showShadow bool) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view, showShadow)
	if imageA == nil || imageB == nil || imageA.GPUTexture == nil || imageB.GPUTexture == nil {
		return
	}

	rectA := ImageRectForView(windowWidth, windowHeight, imageA.Width, imageA.Height, view)
	rectB := compareRectForView(rectA, imageA.Width, imageA.Height, imageB.Width, imageB.Height, view)
	// Use the unrotated image dimensions as the reference. Using rectA here
	// would swap its width and height at 90 degrees and make the circle grow
	// or shrink on non-square images during rotation.
	baseRect := ImageRect(windowWidth, windowHeight, imageA.Width, imageA.Height, view.Zoom, view.OffsetX, view.OffsetY)
	radius := float64(minInt(baseRect.Dx(), baseRect.Dy())) * clampFloat64(diameterRatio, 0.02, 1) / 2
	if radius <= 0 {
		return
	}
	maskedAlpha := view.Alpha * clampAlpha(maskAlpha)

	if reverse {
		drawTexture(screen, imageB.GPUTexture, rectB, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, 0, 0, view.Alpha, view.Gamma, view.Exposure, view.Contrast)
		if feathered {
			drawCircularTextureFeathered(screen, imageA.GPUTexture, rectA, float64(centerX), float64(centerY), radius, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, maskedAlpha, view.Gamma, view.Exposure, view.Contrast)
		} else {
			drawCircularTexture(screen, imageA.GPUTexture, rectA, float64(centerX), float64(centerY), radius, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, maskedAlpha, view.Gamma, view.Exposure, view.Contrast)
		}
	} else {
		if feathered {
			drawCircularTextureFeathered(screen, imageB.GPUTexture, rectB, float64(centerX), float64(centerY), radius, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, maskedAlpha, view.Gamma, view.Exposure, view.Contrast)
		} else {
			drawCircularTexture(screen, imageB.GPUTexture, rectB, float64(centerX), float64(centerY), radius, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, view.MirrorScaleX, view.MirrorScaleY, maskedAlpha, view.Gamma, view.Exposure, view.Contrast)
		}
	}

	if sliderOpacity > 0 {
		borderAlpha := clampAlpha(sliderOpacity)
		drawCircleOutline(screen, float32(centerX), float32(centerY), float32(radius), color.NRGBA{230, 230, 230, uint8(math.Round(96 * borderAlpha))})
	}
}

// compareRectForView chooses B's fit scale before applying the rotation. If
// the fit were recomputed from the two animated bounding boxes, images with
// different aspect ratios would acquire a temporary zoom between quarter
// turns. Keeping this scale constant makes B rotate rigidly like A.
func compareRectForView(rectA stdimage.Rectangle, imageAWidth, imageAHeight, imageBWidth, imageBHeight int, view View) stdimage.Rectangle {
	if imageAWidth <= 0 || imageAHeight <= 0 || imageBWidth <= 0 || imageBHeight <= 0 {
		return stdimage.Rectangle{}
	}
	fitScale := view.Zoom * math.Min(float64(imageAWidth)/float64(imageBWidth), float64(imageAHeight)/float64(imageBHeight))
	angle := view.RotationAngle * math.Pi / 2
	rotatedWidth := (float64(imageBWidth)*math.Abs(math.Cos(angle)) + float64(imageBHeight)*math.Abs(math.Sin(angle))) * fitScale
	rotatedHeight := (float64(imageBWidth)*math.Abs(math.Sin(angle)) + float64(imageBHeight)*math.Abs(math.Cos(angle))) * fitScale
	cx := float64(rectA.Min.X+rectA.Max.X) / 2
	cy := float64(rectA.Min.Y+rectA.Max.Y) / 2
	return stdimage.Rect(
		int(math.Round(cx-rotatedWidth/2)),
		int(math.Round(cy-rotatedHeight/2)),
		int(math.Round(cx+rotatedWidth/2)),
		int(math.Round(cy+rotatedHeight/2)),
	)
}

func drawClippedCompareImage(screen *ebiten.Image, texture *ebiten.Image, destRect, maskRect stdimage.Rectangle, boundary int, horizontal bool, reverse bool, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast float64) {
	if math.Abs(rotation) > 0.001 || mirrorScaleX != 0 || mirrorScaleY != 0 {
		ratio := float64(boundary)
		if horizontal {
			ratio = (float64(boundary) - float64(maskRect.Min.Y)) / float64(maskRect.Dy())
		} else {
			ratio = (float64(boundary) - float64(maskRect.Min.X)) / float64(maskRect.Dx())
		}
		ratio = clampFloat64(ratio, 0, 1)
		screenSide := 1 // right
		if horizontal {
			if reverse {
				screenSide = 0 // top
			} else {
				screenSide = 2 // bottom
			}
		} else if reverse {
			screenSide = 3 // left
		}
		quarterTurns := ((int(math.Round(rotation)) % 4) + 4) % 4
		localSide := (screenSide - quarterTurns + 4) % 4
		// Convert the screen-space split coordinate back to the local image
		// coordinate. Counter-clockwise rotation reverses top/bottom, while
		// clockwise rotation reverses left/right; a half-turn reverses both.
		if quarterTurns == 2 ||
			(quarterTurns == 1 && (screenSide == 1 || screenSide == 3)) ||
			(quarterTurns == 3 && (screenSide == 0 || screenSide == 2)) {
			ratio = 1 - ratio
		}
		// The clipping side is expressed in the image's local coordinates,
		// before the mirror is applied. A horizontal mirror swaps left/right;
		// a vertical mirror swaps top/bottom. The split ratio must be mirrored
		// along with that side, otherwise the line moves correctly but B is
		// clipped from the opposite side.
		effectiveFlipHorizontal := flipHorizontal
		if math.Abs(mirrorScaleX) > 0.001 {
			effectiveFlipHorizontal = mirrorScaleX < 0
		}
		effectiveFlipVertical := flipVertical
		if math.Abs(mirrorScaleY) > 0.001 {
			effectiveFlipVertical = mirrorScaleY < 0
		}
		if effectiveFlipHorizontal && (localSide == 1 || localSide == 3) {
			localSide = 4 - localSide
			ratio = 1 - ratio
		}
		if effectiveFlipVertical && (localSide == 0 || localSide == 2) {
			localSide = 2 - localSide
			ratio = 1 - ratio
		}
		switch localSide {
		case 0: // top
			drawTransformedLocalImage(screen, texture, destRect, 0, 1, 0, ratio, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast)
		case 1: // right
			drawTransformedLocalImage(screen, texture, destRect, ratio, 1, 0, 1, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast)
		case 2: // bottom
			drawTransformedLocalImage(screen, texture, destRect, 0, 1, ratio, 1, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast)
		case 3: // left
			drawTransformedLocalImage(screen, texture, destRect, 0, ratio, 0, 1, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast)
		}
		return
	}
	visibleRect := destRect
	if horizontal {
		if reverse {
			visibleRect.Max.Y = minInt(visibleRect.Max.Y, boundary)
		} else {
			visibleRect.Min.Y = maxInt(visibleRect.Min.Y, boundary)
		}
	} else {
		if reverse {
			visibleRect.Max.X = minInt(visibleRect.Max.X, boundary)
		} else {
			visibleRect.Min.X = maxInt(visibleRect.Min.X, boundary)
		}
	}

	if visibleRect.Empty() {
		return
	}

	drawTextureClipped(screen, texture, destRect, visibleRect, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast)
}

func drawTransformedLocalImage(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, u0, u1, v0, v1 float64, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast float64) {
	if texture == nil || destRect.Empty() || u1 <= u0 || v1 <= v0 {
		return
	}
	texBounds := texture.Bounds()
	angle := rotation * math.Pi / 2
	cosine := math.Cos(angle)
	sine := math.Sin(angle)
	rotatedWidth := float64(texBounds.Dx())*math.Abs(cosine) + float64(texBounds.Dy())*math.Abs(sine)
	scale := float64(destRect.Dx()) / math.Max(1, rotatedWidth)
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
	point := func(u, v float64) (float32, float32, float32, float32) {
		x := (u - 0.5) * float64(texBounds.Dx()) * mirrorX
		y := (v - 0.5) * float64(texBounds.Dy()) * mirrorY
		dx := scale * (cosine*x - sine*y)
		dy := scale * (sine*x + cosine*y)
		return float32(float64(destRect.Min.X+destRect.Max.X)/2 + dx), float32(float64(destRect.Min.Y+destRect.Max.Y)/2 + dy), float32(float64(texBounds.Min.X) + u*float64(texBounds.Dx())), float32(float64(texBounds.Min.Y) + v*float64(texBounds.Dy()))
	}
	vertices := make([]ebiten.Vertex, 4)
	points := [4][4]float32{}
	points[0][0], points[0][1], points[0][2], points[0][3] = point(u0, v0)
	points[1][0], points[1][1], points[1][2], points[1][3] = point(u1, v0)
	points[2][0], points[2][1], points[2][2], points[2][3] = point(u0, v1)
	points[3][0], points[3][1], points[3][2], points[3][3] = point(u1, v1)
	for i := range vertices {
		vertices[i] = ebiten.Vertex{DstX: points[i][0], DstY: points[i][1], SrcX: points[i][2], SrcY: points[i][3], ColorR: 1, ColorG: 1, ColorB: 1, ColorA: float32(clampAlpha(alpha))}
	}
	shader, uniforms := gammaDrawOptions(gamma, exposure, contrast)
	if shader != nil {
		screen.DrawTrianglesShader(vertices, []uint16{0, 1, 2, 1, 2, 3}, shader, &ebiten.DrawTrianglesShaderOptions{Images: [4]*ebiten.Image{texture}, Uniforms: uniforms})
	} else {
		screen.DrawTriangles(vertices, []uint16{0, 1, 2, 1, 2, 3}, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	}
}

func drawFeatheredCompareImage(screen *ebiten.Image, texture *ebiten.Image, destRect, maskRect stdimage.Rectangle, boundary int, horizontal bool, reverse bool, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast float64) {
	const featherWidth = 10
	const featherSteps = 10
	if math.Abs(rotation) > 0.001 || mirrorScaleX != 0 || mirrorScaleY != 0 {
		drawClippedCompareImage(screen, texture, destRect, maskRect, boundary, horizontal, reverse, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast)
		return
	}

	// Draw the fully visible side first, then ten one-pixel bands around the
	// boundary. The bands are ordered so the transition is symmetric for both
	// horizontal and vertical splits, including the reversed direction.
	fullBoundary := boundary + featherWidth/2
	if reverse {
		fullBoundary = boundary - featherWidth/2
	}
	drawClippedCompareImage(screen, texture, destRect, maskRect, fullBoundary, horizontal, reverse, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast)

	for i := 0; i < featherSteps; i++ {
		bandStart := boundary - featherWidth/2 + i
		bandEnd := bandStart + 1
		bandAlpha := alpha * float64(i+1) / featherSteps
		if reverse {
			bandStart = boundary + featherWidth/2 - i - 1
			bandEnd = bandStart + 1
		}

		visibleRect := destRect
		if horizontal {
			visibleRect.Min.Y = maxInt(visibleRect.Min.Y, bandStart)
			visibleRect.Max.Y = minInt(visibleRect.Max.Y, bandEnd)
		} else {
			visibleRect.Min.X = maxInt(visibleRect.Min.X, bandStart)
			visibleRect.Max.X = minInt(visibleRect.Max.X, bandEnd)
		}
		if !visibleRect.Empty() {
			drawTextureClipped(screen, texture, destRect, visibleRect, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, bandAlpha, gamma, exposure, contrast)
		}
	}
}

func drawCircularTextureFeathered(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, centerX, centerY, radius float64, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast float64) {
	const featherWidth = 10.0
	const featherSteps = 10

	// Draw complete concentric circles from the outside in. The per-layer
	// alpha compensates for the previous layers, producing target opacities
	// of 10%, 20%, ..., 100% without directional artifacts.
	feather := math.Min(featherWidth, radius)
	outerRadius := radius + feather/2
	for i := 0; i < featherSteps; i++ {
		targetAlpha := clampAlpha(alpha) * float64(i+1) / featherSteps
		previousAlpha := clampAlpha(alpha) * float64(i) / featherSteps
		layerAlpha := (targetAlpha - previousAlpha) / (1 - previousAlpha)
		circleRadius := outerRadius - feather*float64(i)/float64(featherSteps-1)
		drawCircularTexture(screen, texture, destRect, centerX, centerY, circleRadius, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, layerAlpha, gamma, exposure, contrast)
	}
}

func drawCircularTexture(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, centerX, centerY, radius float64, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast float64) {
	if texture == nil || destRect.Empty() || radius <= 0 {
		return
	}

	left := maxInt(destRect.Min.X, int(math.Floor(centerX-radius)))
	right := minInt(destRect.Max.X, int(math.Ceil(centerX+radius)))
	if right <= left {
		return
	}

	width := right - left
	stripCount := minInt(width, 1024)
	for i := 0; i < stripCount; i++ {
		// Integer partitioning guarantees that adjacent strips neither overlap
		// nor leave gaps. Overlapping columns become visible as vertical lines
		// as soon as the mask is rendered with partial opacity.
		x0 := left + i*width/stripCount
		x1 := left + (i+1)*width/stripCount

		midX := (float64(x0+x1) / 2) - centerX
		if math.Abs(midX) > radius {
			continue
		}

		halfHeight := math.Sqrt(radius*radius - midX*midX)
		visibleRect := stdimage.Rect(
			x0,
			maxInt(destRect.Min.Y, int(math.Ceil(centerY-halfHeight))),
			x1,
			minInt(destRect.Max.Y, int(math.Floor(centerY+halfHeight))),
		)
		if !visibleRect.Empty() {
			drawTextureClipped(screen, texture, destRect, visibleRect, flipHorizontal, flipVertical, rotation, mirrorScaleX, mirrorScaleY, alpha, gamma, exposure, contrast)
		}
	}
}

func drawCircleOutline(screen *ebiten.Image, centerX, centerY, radius float32, stroke color.Color) {
	drawCircleStroke(screen, centerX, centerY, radius, stroke, 2)
}

func drawCircleStroke(screen *ebiten.Image, centerX, centerY, radius float32, stroke color.Color, width float32) {
	if radius <= 0 {
		return
	}

	path := &vector.Path{}
	path.MoveTo(centerX+radius, centerY)
	path.Arc(centerX, centerY, radius, 0, math.Pi*2, vector.Clockwise)
	path.Close()
	options := &vector.DrawPathOptions{AntiAlias: true}
	options.ColorScale.ScaleWithColor(stroke)
	vector.StrokePath(screen, path, &vector.StrokeOptions{Width: width}, options)
}
