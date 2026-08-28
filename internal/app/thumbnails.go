package app

import (
	stdimage "image"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	imagedata "viewergo/internal/image"
)

const (
	thumbnailBoxWidth      = 104
	thumbnailBoxHeight     = 68
	thumbnailCurrentWidth  = 116
	thumbnailCurrentHeight = 78
	thumbnailGap           = 8
	thumbnailBottomMargin  = 10
	thumbnailMaxRadius     = 4
)

type thumbnailResult struct {
	path  string
	image stdimage.Image
	err   error
}

type thumbnailCacheEntry struct {
	texture  *ebiten.Image
	lastUsed uint64
}

type thumbnailItem struct {
	path    string
	rect    stdimage.Rectangle
	current bool
}

func thumbnailRadius(windowWidth int) int {
	available := windowWidth/2 - thumbnailBoxWidth/2 - thumbnailBottomMargin
	if available <= 0 {
		return 0
	}
	radius := available / (thumbnailBoxWidth + thumbnailGap)
	if radius > thumbnailMaxRadius {
		return thumbnailMaxRadius
	}
	return radius
}

func thumbnailRect(windowWidth, windowHeight, offset int) stdimage.Rectangle {
	width, height := thumbnailBoxWidth, thumbnailBoxHeight
	if offset == 0 {
		width, height = thumbnailCurrentWidth, thumbnailCurrentHeight
	}
	centerX := windowWidth/2 + offset*(thumbnailBoxWidth+thumbnailGap)
	bottom := maxInt(0, windowHeight-thumbnailBottomMargin)
	return stdimage.Rect(centerX-width/2, bottom-height, centerX+(width-width/2), bottom)
}

func visibleThumbnailItems(images []string, currentIndex, windowWidth, windowHeight int) []thumbnailItem {
	if currentIndex < 0 || currentIndex >= len(images) || windowWidth <= 0 || windowHeight <= 0 {
		return nil
	}
	radius := thumbnailRadius(windowWidth)
	items := make([]thumbnailItem, 0, radius*2+1)
	for offset := -radius; offset <= radius; offset++ {
		index := currentIndex + offset
		if index < 0 || index >= len(images) {
			continue
		}
		items = append(items, thumbnailItem{
			path: images[index], rect: thumbnailRect(windowWidth, windowHeight, offset), current: offset == 0,
		})
	}
	return items
}

func (v *Viewer) currentThumbnailItems() []thumbnailItem {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return nil
	}
	images, currentIndex, err := v.navigationImagePaths(v.imageA.FilePath)
	if err != nil {
		return nil
	}
	return visibleThumbnailItems(images, currentIndex, v.windowWidth, v.windowHeight)
}

func (v *Viewer) wantedThumbnailPaths() []string {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return nil
	}
	images, currentIndex, err := v.navigationImagePaths(v.imageA.FilePath)
	if err != nil || currentIndex < 0 {
		return nil
	}
	radius := thumbnailRadius(v.windowWidth)
	paths := make([]string, 0, radius*2+1)
	paths = append(paths, images[currentIndex])
	for distance := 1; distance <= radius; distance++ {
		if index := currentIndex - distance; index >= 0 {
			paths = append(paths, images[index])
		}
		if index := currentIndex + distance; index < len(images) {
			paths = append(paths, images[index])
		}
	}
	return paths
}

func (v *Viewer) ensureVisibleThumbnails() {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return
	}
	paths := v.wantedThumbnailPaths()
	dir := v.navigationDirectory
	if dir != "" && dir != v.thumbnailDirectory {
		v.clearThumbnailCache()
		v.thumbnailDirectory = dir
	}
	for _, path := range paths {
		if entry := v.thumbnailCache[path]; entry != nil {
			v.touchThumbnail(entry)
			continue
		}
		if v.thumbnailInFlight[path] || v.thumbnailFailed[path] {
			continue
		}
		v.thumbnailInFlight[path] = true
		go func(path string) {
			v.thumbnailSlots <- struct{}{}
			thumbnail, err := imagedata.DecodeFileThumbnail(path, thumbnailMaxDimension)
			<-v.thumbnailSlots
			v.thumbnailResults <- thumbnailResult{path: path, image: thumbnail, err: err}
			ebiten.ScheduleFrame()
		}(path)
	}
}

func (v *Viewer) collectThumbnails() {
	for {
		select {
		case result := <-v.thumbnailResults:
			delete(v.thumbnailInFlight, result.path)
			if result.err != nil || result.image == nil {
				v.thumbnailFailed[result.path] = true
				continue
			}
			if !v.isWantedThumbnail(result.path) {
				continue
			}
			if existing := v.thumbnailCache[result.path]; existing != nil {
				v.touchThumbnail(existing)
				continue
			}
			v.evictThumbnailIfNeeded()
			entry := &thumbnailCacheEntry{texture: ebiten.NewImageFromImage(result.image)}
			v.touchThumbnail(entry)
			v.thumbnailCache[result.path] = entry
		default:
			return
		}
	}
}

func (v *Viewer) isWantedThumbnail(path string) bool {
	for _, wantedPath := range v.wantedThumbnailPaths() {
		if wantedPath == path {
			return true
		}
	}
	return false
}

func (v *Viewer) touchThumbnail(entry *thumbnailCacheEntry) {
	if entry == nil {
		return
	}
	v.thumbnailUseCounter++
	entry.lastUsed = v.thumbnailUseCounter
}

func (v *Viewer) evictThumbnailIfNeeded() {
	if len(v.thumbnailCache) < thumbnailCacheLimit {
		return
	}
	var oldestPath string
	oldestUse := ^uint64(0)
	for path, entry := range v.thumbnailCache {
		if entry.lastUsed < oldestUse {
			oldestPath = path
			oldestUse = entry.lastUsed
		}
	}
	if entry := v.thumbnailCache[oldestPath]; entry != nil {
		entry.texture.Deallocate()
		delete(v.thumbnailCache, oldestPath)
	}
}

func (v *Viewer) clearThumbnailCache() {
	for path, entry := range v.thumbnailCache {
		if entry != nil && entry.texture != nil {
			entry.texture.Deallocate()
		}
		delete(v.thumbnailCache, path)
	}
	clear(v.thumbnailFailed)
}

func (v *Viewer) drawThumbnailStrip(screen *ebiten.Image) {
	if v.thumbnailOpacity <= 0.01 {
		return
	}
	items := v.currentThumbnailItems()
	if len(items) == 0 {
		return
	}
	stripRect := items[0].rect
	for _, item := range items[1:] {
		stripRect = stripRect.Union(item.rect)
	}
	stripRect = stripRect.Inset(-6)
	drawThumbnailFrame(screen, stripRect, color.NRGBA{20, 20, 20, 155}, color.NRGBA{255, 255, 255, 30}, 8, 1, v.thumbnailOpacity)

	for _, item := range items {
		frameColor := color.NRGBA{180, 180, 180, 190}
		frameWidth := float32(1)
		if item.current {
			frameColor = color.NRGBA{255, 255, 255, 245}
			frameWidth = 3
		}
		drawThumbnailFrame(screen, item.rect, color.NRGBA{42, 42, 42, 225}, frameColor, 5, frameWidth, v.thumbnailOpacity)

		texture := (*ebiten.Image)(nil)
		if entry := v.thumbnailCache[item.path]; entry != nil {
			texture = entry.texture
		} else if item.current && v.imageA != nil && item.path == v.imageA.FilePath {
			texture = v.imageA.GPUTexture
		}
		drawThumbnailTexture(screen, texture, item.rect.Inset(5), v.thumbnailOpacity)
	}
}

func drawThumbnailFrame(screen *ebiten.Image, rect stdimage.Rectangle, fill, stroke color.NRGBA, radius, strokeWidth float32, opacity float64) {
	path := roundedRectPath(float32(rect.Min.X), float32(rect.Min.Y), float32(rect.Dx()), float32(rect.Dy()), radius)
	fillOptions := &vector.DrawPathOptions{AntiAlias: true}
	fillOptions.ColorScale.ScaleWithColor(withOpacity(fill, opacity))
	vector.FillPath(screen, path, &vector.FillOptions{}, fillOptions)
	if strokeWidth <= 0 {
		return
	}
	strokeOptions := &vector.DrawPathOptions{AntiAlias: true}
	strokeOptions.ColorScale.ScaleWithColor(withOpacity(stroke, opacity))
	vector.StrokePath(screen, path, &vector.StrokeOptions{Width: strokeWidth}, strokeOptions)
}

func drawThumbnailTexture(screen, texture *ebiten.Image, bounds stdimage.Rectangle, opacity float64) {
	if texture == nil || bounds.Empty() {
		return
	}
	textureBounds := texture.Bounds()
	scale := minFloat64(
		float64(bounds.Dx())/float64(maxInt(1, textureBounds.Dx())),
		float64(bounds.Dy())/float64(maxInt(1, textureBounds.Dy())),
	)
	width := float64(textureBounds.Dx()) * scale
	height := float64(textureBounds.Dy()) * scale
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.ColorScale.ScaleAlpha(float32(opacity))
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(
		float64(bounds.Min.X)+(float64(bounds.Dx())-width)/2,
		float64(bounds.Min.Y)+(float64(bounds.Dy())-height)/2,
	)
	screen.DrawImage(texture, options)
}

func withOpacity(value color.NRGBA, opacity float64) color.NRGBA {
	value.A = uint8(float64(value.A) * clampFloat64(opacity, 0, 1))
	return value
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (v *Viewer) thumbnailPathAt(x, y int) (string, bool) {
	if v.thumbnailOpacity < 0.5 {
		return "", false
	}
	for _, item := range v.currentThumbnailItems() {
		if pointInRect(x, y, item.rect) {
			return item.path, true
		}
	}
	return "", false
}

func (v *Viewer) pointInThumbnailStrip(x, y int) bool {
	if v.thumbnailOpacity < 0.5 {
		return false
	}
	items := v.currentThumbnailItems()
	if len(items) == 0 {
		return false
	}
	rect := items[0].rect
	for _, item := range items[1:] {
		rect = rect.Union(item.rect)
	}
	return pointInRect(x, y, rect.Inset(-6))
}

func (v *Viewer) updateThumbnailVisibility(now time.Time, mouseX, mouseY int) {
	targetOpacity := 0.0
	if v.shouldShowThumbnails(now, mouseX, mouseY) {
		targetOpacity = 1
	}
	v.thumbnailOpacity = approachOpacity(v.thumbnailOpacity, targetOpacity)
}

func (v *Viewer) shouldShowThumbnails(now time.Time, mouseX, mouseY int) bool {
	return v.imageA != nil &&
		mouseX >= 0 && mouseX < v.windowWidth &&
		mouseY >= maxInt(0, v.windowHeight-thumbnailRevealDistance) && mouseY < v.windowHeight &&
		!v.cursorIdle(now)
}
