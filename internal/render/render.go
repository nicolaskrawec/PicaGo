package render

import (
	stdimage "image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	imagedata "viewergo/internal/image"
)

type View struct {
	Zoom           float64
	OffsetX        float64
	OffsetY        float64
	Alpha          float64
	FlipHorizontal bool
	FlipVertical   bool
	Rotation       int // clockwise quarter turns
	RotationAngle  float64
	MirrorScaleX   float64
	MirrorScaleY   float64
}

type imageFrameShadow struct {
	img    *ebiten.Image
	w      int
	h      int
	spread int
	radius int
}

var shadowCache imageFrameShadow

func FitZoom(windowWidth, windowHeight, imageWidth, imageHeight int) float64 {
	if windowWidth <= 0 || windowHeight <= 0 || imageWidth <= 0 || imageHeight <= 0 {
		return 1
	}

	fitX := float64(windowWidth) / float64(imageWidth)
	fitY := float64(windowHeight) / float64(imageHeight)
	return math.Min(fitX, fitY)
}

func ImageRect(windowWidth, windowHeight, imageWidth, imageHeight int, zoom, offsetX, offsetY float64) stdimage.Rectangle {
	drawWidth := float64(imageWidth) * zoom
	drawHeight := float64(imageHeight) * zoom
	left := float64(windowWidth)/2 - drawWidth/2 + offsetX
	top := float64(windowHeight)/2 - drawHeight/2 + offsetY
	return stdimage.Rect(
		int(math.Round(left)),
		int(math.Round(top)),
		int(math.Round(left+drawWidth)),
		int(math.Round(top+drawHeight)),
	)
}

func ImageRectForView(windowWidth, windowHeight, imageWidth, imageHeight int, view View) stdimage.Rectangle {
	width, height := rotatedDimensions(imageWidth, imageHeight, view)
	return ImageRect(windowWidth, windowHeight, width, height, view.Zoom, view.OffsetX, view.OffsetY)
}

func rotatedDimensions(imageWidth, imageHeight int, view View) (int, int) {
	angle := view.RotationAngle
	radians := angle * math.Pi / 2
	cosine := math.Abs(math.Cos(radians))
	sine := math.Abs(math.Sin(radians))
	return maxInt(1, int(math.Round(float64(imageWidth)*cosine+float64(imageHeight)*sine))),
		maxInt(1, int(math.Round(float64(imageWidth)*sine+float64(imageHeight)*cosine)))
}

func DrawImage(screen *ebiten.Image, loaded *imagedata.LoadedImage, windowWidth, windowHeight int, view View, showShadow bool) {
	if loaded == nil || loaded.GPUTexture == nil {
		return
	}

	rect := ImageRectForView(windowWidth, windowHeight, loaded.Width, loaded.Height, view)
	if showShadow && !loaded.HasTransparency {
		drawShadow(screen, loaded.Width, loaded.Height, windowWidth, windowHeight, rect, view)
	}
	rotationAngle := view.RotationAngle
	drawTexture(screen, loaded.GPUTexture, rect, view.FlipHorizontal, view.FlipVertical, rotationAngle, view.MirrorScaleX, view.MirrorScaleY, view.Alpha)
}

func DrawCompare(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, sliderPosition float64, orientation int, feathered bool, reverse bool, sliderOpacity float64, showShadow bool) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view, showShadow)
	if imageA == nil || imageB == nil || imageB.GPUTexture == nil {
		return
	}

	rectA := ImageRectForView(windowWidth, windowHeight, imageA.Width, imageA.Height, view)
	imageBWidth, imageBHeight := rotatedDimensions(imageB.Width, imageB.Height, view)
	rectB := compareRect(rectA, imageBWidth, imageBHeight)
	switch orientation {
	case 1:
		visibleTop := clampBoundary(int(math.Ceil(sliderPosition)), rectA.Min.Y, rectA.Max.Y-1)
		if visibleTop < rectA.Min.Y {
			return
		}
		if feathered {
			drawFeatheredCompareImage(screen, imageB.GPUTexture, rectB, visibleTop, true, reverse, view.FlipHorizontal, view.FlipVertical, view.Rotation, view.Alpha)
		} else {
			drawClippedCompareImage(screen, imageB.GPUTexture, rectB, visibleTop, true, reverse, view.FlipHorizontal, view.FlipVertical, view.Rotation, view.Alpha)
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
			drawFeatheredCompareImage(screen, imageB.GPUTexture, rectB, visibleLeft, false, reverse, view.FlipHorizontal, view.FlipVertical, view.Rotation, view.Alpha)
		} else {
			drawClippedCompareImage(screen, imageB.GPUTexture, rectB, visibleLeft, false, reverse, view.FlipHorizontal, view.FlipVertical, view.Rotation, view.Alpha)
		}
		if sliderOpacity > 0 {
			vector.FillRect(screen, float32(visibleLeft)-1, float32(rectA.Min.Y), 2, float32(rectA.Max.Y-rectA.Min.Y), color.NRGBA{230, 230, 230, uint8(math.Round(96 * clampAlpha(sliderOpacity)))}, true)
		}
	}
}

func DrawCompareCircle(screen *ebiten.Image, imageA, imageB *imagedata.LoadedImage, windowWidth, windowHeight int, view View, centerX, centerY int, diameterRatio float64, feathered bool, reverse bool, sliderOpacity float64, showShadow bool) {
	DrawImage(screen, imageA, windowWidth, windowHeight, view, showShadow)
	if imageA == nil || imageB == nil || imageA.GPUTexture == nil || imageB.GPUTexture == nil {
		return
	}

	rectA := ImageRectForView(windowWidth, windowHeight, imageA.Width, imageA.Height, view)
	imageBWidth, imageBHeight := rotatedDimensions(imageB.Width, imageB.Height, view)
	rectB := compareRect(rectA, imageBWidth, imageBHeight)
	radius := float64(minInt(rectA.Dx(), rectA.Dy())) * clampFloat64(diameterRatio, 0.02, 1) / 2
	if radius <= 0 {
		return
	}

	if reverse {
		drawTexture(screen, imageB.GPUTexture, rectB, view.FlipHorizontal, view.FlipVertical, view.RotationAngle, 0, 0, view.Alpha)
		if feathered {
			drawCircularTextureFeathered(screen, imageA.GPUTexture, rectA, float64(centerX), float64(centerY), radius, view.FlipHorizontal, view.FlipVertical, view.Rotation, view.Alpha)
		} else {
			drawCircularTexture(screen, imageA.GPUTexture, rectA, float64(centerX), float64(centerY), radius, view.FlipHorizontal, view.FlipVertical, view.Rotation, view.Alpha)
		}
	} else {
		if feathered {
			drawCircularTextureFeathered(screen, imageB.GPUTexture, rectB, float64(centerX), float64(centerY), radius, view.FlipHorizontal, view.FlipVertical, view.Rotation, view.Alpha)
		} else {
			drawCircularTexture(screen, imageB.GPUTexture, rectB, float64(centerX), float64(centerY), radius, view.FlipHorizontal, view.FlipVertical, view.Rotation, view.Alpha)
		}
	}

	if sliderOpacity > 0 {
		borderAlpha := clampAlpha(sliderOpacity)
		drawCircleOutline(screen, float32(centerX), float32(centerY), float32(radius), color.NRGBA{230, 230, 230, uint8(math.Round(96 * borderAlpha))})
	}
}

func compareRect(baseRect stdimage.Rectangle, imageWidth, imageHeight int) stdimage.Rectangle {
	scale := math.Min(float64(baseRect.Dx())/float64(imageWidth), float64(baseRect.Dy())/float64(imageHeight))
	drawWidth := float64(imageWidth) * scale
	drawHeight := float64(imageHeight) * scale
	left := float64(baseRect.Min.X) + (float64(baseRect.Dx())-drawWidth)/2
	top := float64(baseRect.Min.Y) + (float64(baseRect.Dy())-drawHeight)/2
	return stdimage.Rect(
		int(math.Round(left)),
		int(math.Round(top)),
		int(math.Round(left+drawWidth)),
		int(math.Round(top+drawHeight)),
	)
}

func drawClippedCompareImage(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, boundary int, horizontal bool, reverse bool, flipHorizontal, flipVertical bool, rotation int, alpha float64) {
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

	drawTextureClipped(screen, texture, destRect, visibleRect, flipHorizontal, flipVertical, rotation, alpha)
}

func drawFeatheredCompareImage(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, boundary int, horizontal bool, reverse bool, flipHorizontal, flipVertical bool, rotation int, alpha float64) {
	const featherWidth = 10
	const featherSteps = 10

	// Draw the fully visible side first, then ten one-pixel bands around the
	// boundary. The bands are ordered so the transition is symmetric for both
	// horizontal and vertical splits, including the reversed direction.
	fullBoundary := boundary + featherWidth/2
	if reverse {
		fullBoundary = boundary - featherWidth/2
	}
	drawClippedCompareImage(screen, texture, destRect, fullBoundary, horizontal, reverse, flipHorizontal, flipVertical, rotation, alpha)

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
			drawTextureClipped(screen, texture, destRect, visibleRect, flipHorizontal, flipVertical, rotation, bandAlpha)
		}
	}
}

func drawCircularTextureFeathered(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, centerX, centerY, radius float64, flipHorizontal, flipVertical bool, rotation int, alpha float64) {
	const featherWidth = 10.0
	const featherSteps = 10

	// Draw complete concentric circles from the outside in. The per-layer
	// alpha compensates for the previous layers, producing target opacities
	// of 10%, 20%, ..., 100% without directional artifacts.
	feather := math.Min(featherWidth, radius)
	outerRadius := radius + feather/2
	for i := 0; i < featherSteps; i++ {
		targetAlpha := float64(i+1) / featherSteps
		previousAlpha := float64(i) / featherSteps
		layerAlpha := (targetAlpha - previousAlpha) / (1 - previousAlpha)
		circleRadius := outerRadius - feather*float64(i)/float64(featherSteps-1)
		drawCircularTexture(screen, texture, destRect, centerX, centerY, circleRadius, flipHorizontal, flipVertical, rotation, alpha*layerAlpha)
	}
}

func drawCircularTexture(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, centerX, centerY, radius float64, flipHorizontal, flipVertical bool, rotation int, alpha float64) {
	if texture == nil || destRect.Empty() || radius <= 0 {
		return
	}

	left := maxInt(destRect.Min.X, int(math.Floor(centerX-radius)))
	right := minInt(destRect.Max.X, int(math.Ceil(centerX+radius)))
	if right <= left {
		return
	}

	stripCount := int(math.Ceil(radius * 2))
	if stripCount < 24 {
		stripCount = 24
	}
	if stripCount > 1024 {
		stripCount = 1024
	}

	step := math.Max(1, float64(right-left)/float64(stripCount))
	for x := float64(left); x < float64(right); x += step {
		x0 := int(math.Floor(x))
		x1 := int(math.Ceil(math.Min(x+step, float64(right))))
		if x1 <= x0 {
			x1 = x0 + 1
		}

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
			drawTextureClipped(screen, texture, destRect, visibleRect, flipHorizontal, flipVertical, rotation, alpha)
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

func drawTexture(screen *ebiten.Image, texture *ebiten.Image, destRect stdimage.Rectangle, flipHorizontal, flipVertical bool, rotation, mirrorScaleX, mirrorScaleY, alpha float64) {
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

	screen.DrawImage(texture, options)
}

func drawTextureClipped(screen *ebiten.Image, texture *ebiten.Image, destRect, visibleRect stdimage.Rectangle, flipHorizontal, flipVertical bool, rotation int, alpha float64) {
	if texture == nil || destRect.Empty() || visibleRect.Empty() {
		return
	}

	texBounds := texture.Bounds()
	leftSrc, topSrc := sourcePoint(visibleRect.Min.X, visibleRect.Min.Y, destRect, texBounds, flipHorizontal, flipVertical, rotation)
	rightSrc, _ := sourcePoint(visibleRect.Max.X, visibleRect.Min.Y, destRect, texBounds, flipHorizontal, flipVertical, rotation)
	_, bottomSrc := sourcePoint(visibleRect.Min.X, visibleRect.Max.Y, destRect, texBounds, flipHorizontal, flipVertical, rotation)
	rightBottomSrcX, bottomSrc := sourcePoint(visibleRect.Max.X, visibleRect.Max.Y, destRect, texBounds, flipHorizontal, flipVertical, rotation)
	vertexAlpha := float32(clampAlpha(alpha))

	vertices := []ebiten.Vertex{
		{
			DstX:   float32(visibleRect.Min.X),
			DstY:   float32(visibleRect.Min.Y),
			SrcX:   float32(leftSrc),
			SrcY:   float32(topSrc),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Max.X),
			DstY:   float32(visibleRect.Min.Y),
			SrcX:   float32(rightSrc),
			SrcY:   float32(topSrc),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Min.X),
			DstY:   float32(visibleRect.Max.Y),
			SrcX:   float32(leftSrc),
			SrcY:   float32(bottomSrc),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
		{
			DstX:   float32(visibleRect.Max.X),
			DstY:   float32(visibleRect.Max.Y),
			SrcX:   float32(rightBottomSrcX),
			SrcY:   float32(bottomSrc),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: vertexAlpha,
		},
	}
	indices := []uint16{0, 1, 2, 1, 2, 3}
	options := &ebiten.DrawTrianglesOptions{
		Filter: ebiten.FilterLinear,
	}
	screen.DrawTriangles(vertices, indices, texture, options)
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

func drawShadow(screen *ebiten.Image, imageWidth, imageHeight, windowWidth, windowHeight int, rect stdimage.Rectangle, view View) {
	shadowAlpha := clampAlpha(view.Alpha)
	if shadowAlpha <= 0 {
		return
	}

	if imageWidth <= 0 || imageHeight <= 0 || windowWidth <= 0 || windowHeight <= 0 || rect.Empty() {
		return
	}

	fitZoom := FitZoom(windowWidth, windowHeight, imageWidth, imageHeight)
	baseRect := ImageRect(windowWidth, windowHeight, imageWidth, imageHeight, fitZoom, 0, 0)
	baseWidth := baseRect.Dx()
	baseHeight := baseRect.Dy()
	if baseWidth <= 0 || baseHeight <= 0 {
		return
	}

	spread := shadowSpread(baseWidth, baseHeight)
	radius := spread * 2
	shadowCache.update(baseWidth, baseHeight, spread, radius)
	if shadowCache.img == nil {
		return
	}

	imageScale := view.Zoom / fitZoom
	if imageScale <= 0 {
		return
	}
	options := &ebiten.DrawImageOptions{}
	options.Filter = ebiten.FilterLinear
	options.ColorScale.ScaleAlpha(float32(shadowAlpha))
	shadowWidth := float64(shadowCache.img.Bounds().Dx())
	shadowHeight := float64(shadowCache.img.Bounds().Dy())
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
	options.GeoM.Translate(-shadowWidth/2, -shadowHeight/2)
	options.GeoM.Scale(imageScale*mirrorX, imageScale*mirrorY)
	options.GeoM.Rotate(view.RotationAngle * math.Pi / 2)
	options.GeoM.Translate(float64(rect.Min.X+rect.Max.X)/2, float64(rect.Min.Y+rect.Max.Y)/2)
	screen.DrawImage(shadowCache.img, options)
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

func sourcePoint(x, y int, destRect, texRect stdimage.Rectangle, flipHorizontal, flipVertical bool, rotation int) (float64, float64) {
	u := float64(x-destRect.Min.X) / float64(destRect.Dx())
	v := float64(y-destRect.Min.Y) / float64(destRect.Dy())
	if flipHorizontal {
		u = 1 - u
	}
	if flipVertical {
		v = 1 - v
	}
	switch ((rotation % 4) + 4) % 4 {
	case 1: // clockwise
		return float64(texRect.Min.X) + v*float64(texRect.Dx()), float64(texRect.Min.Y) + (1-u)*float64(texRect.Dy())
	case 2:
		return float64(texRect.Min.X) + (1-u)*float64(texRect.Dx()), float64(texRect.Min.Y) + (1-v)*float64(texRect.Dy())
	case 3: // counter-clockwise
		return float64(texRect.Min.X) + (1-v)*float64(texRect.Dx()), float64(texRect.Min.Y) + u*float64(texRect.Dy())
	default:
		return float64(texRect.Min.X) + u*float64(texRect.Dx()), float64(texRect.Min.Y) + v*float64(texRect.Dy())
	}
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

func (s *imageFrameShadow) update(w, h, spread, radius int) {
	if s.img != nil && s.w == w && s.h == h && s.spread == spread && s.radius == radius {
		return
	}

	s.w = w
	s.h = h
	s.spread = spread
	s.radius = radius
	if s.img != nil {
		s.img.Deallocate()
	}
	s.img = buildShadowImage(w, h, spread, radius, 0.2)
}

func buildShadowImage(w, h, spread, radius int, alpha float32) *ebiten.Image {
	if w <= 0 || h <= 0 || spread <= 0 || alpha <= 0 {
		return nil
	}

	outW := w + spread*2
	outH := h + spread*2
	rgba := stdimage.NewRGBA(stdimage.Rect(0, 0, outW, outH))

	rectX0 := float64(spread)
	rectY0 := float64(spread)
	rectX1 := float64(spread + w)
	rectY1 := float64(spread + h)
	blur := float64(spread)
	cornerRadius := float64(radius)

	for py := 0; py < outH; py++ {
		for px := 0; px < outW; px++ {
			x := float64(px) + 0.5
			y := float64(py) + 0.5

			distance := roundedRectSDF(x, y, rectX0, rectY0, rectX1, rectY1, cornerRadius)
			if distance < 0 {
				continue
			}

			t := 1.0 - clamp01(distance/blur)
			t *= t
			a := uint8(math.Round(float64(alpha) * t * 255))
			if a == 0 {
				continue
			}

			rgba.SetRGBA(px, py, color.RGBA{A: a})
		}
	}

	return ebiten.NewImageFromImage(rgba)
}

func roundedRectSDF(px, py, x0, y0, x1, y1, r float64) float64 {
	cx := (x0 + x1) * 0.5
	cy := (y0 + y1) * 0.5
	hx := (x1 - x0) * 0.5
	hy := (y1 - y0) * 0.5

	r = minFloat64(r, math.Min(hx, hy))
	qx := math.Abs(px-cx) - (hx - r)
	qy := math.Abs(py-cy) - (hy - r)
	ax := math.Max(qx, 0)
	ay := math.Max(qy, 0)

	return math.Hypot(ax, ay) + math.Min(math.Max(qx, qy), 0) - r
}

func shadowSpread(width, height int) int {
	visibleSize := math.Min(float64(width), float64(height))
	return int(math.Round(float64(clampFloat32(float32(visibleSize*0.08), 8, 500))))
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
