package app

import (
	stdimage "image"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	imagedata "viewergo/internal/image"
)

const (
	thumbnailBoxWidth      = 70
	thumbnailBoxHeight     = 70
	thumbnailCurrentWidth  = 90
	thumbnailCurrentHeight = 90
	thumbnailGap           = 2
	thumbnailBottomMargin  = 10
	thumbnailMaxRadius     = 8
	thumbnailPreloadRadius = 16
)

type thumbnailResult struct {
	path  string
	image stdimage.Image
	err   error
}

type thumbnailCacheEntry struct {
	texture       *ebiten.Image
	lastUsed      uint64
	fadeStartedAt time.Time
}

type thumbnailItem struct {
	path     string
	rect     stdimage.Rectangle
	current  bool
	distance int
}

func thumbnailRadius(windowWidth int) int {
	available := windowWidth/2 - thumbnailCurrentWidth/2 - thumbnailBottomMargin
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
	centerX := windowWidth / 2
	if offset == 0 {
		width, height = thumbnailCurrentWidth, thumbnailCurrentHeight
	} else {
		distance := absInt(offset)
		centerOffset := thumbnailCurrentWidth/2 + thumbnailGap + thumbnailBoxWidth/2 +
			(distance-1)*(thumbnailBoxWidth+thumbnailGap)
		if offset < 0 {
			centerOffset = -centerOffset
		}
		centerX += centerOffset
	}
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
			distance: absInt(offset),
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
	radius := thumbnailPreloadRadius
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
	if len(paths) == 0 {
		return
	}

	// The active thumbnail is a hard priority: do not start its neighbors
	// until it has completed (or failed). This keeps a slow portrait/landscape
	// decode from appearing after less important previews.
	currentPath := paths[0]
	if entry := v.thumbnailCache[currentPath]; entry != nil {
		v.touchThumbnail(entry)
	} else if !v.thumbnailFailed[currentPath] {
		if !v.thumbnailInFlight[currentPath] {
			v.startThumbnailLoad(currentPath)
		}
		return
	}

	// wantedThumbnailPaths is ordered current, left 1, right 1, left 2,
	// right 2, ... . Only start work while a decoder is genuinely available;
	// no waiting goroutine can race ahead of that priority order.
	for _, path := range paths[1:] {
		if entry := v.thumbnailCache[path]; entry != nil {
			v.touchThumbnail(entry)
			continue
		}
		if v.thumbnailInFlight[path] || v.thumbnailFailed[path] {
			continue
		}
		if len(v.thumbnailInFlight) >= thumbnailWorkerLimit {
			continue
		}
		v.startThumbnailLoad(path)
	}
}

func (v *Viewer) startThumbnailLoad(path string) {
	if path == "" || v.thumbnailInFlight[path] || len(v.thumbnailInFlight) >= thumbnailWorkerLimit {
		return
	}
	v.thumbnailInFlight[path] = true
	go func() {
		thumbnail, err := imagedata.DecodeFileThumbnail(path, thumbnailMaxDimension)
		v.thumbnailResults <- thumbnailResult{path: path, image: thumbnail, err: err}
		ebiten.ScheduleFrame()
	}()
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
			entry := &thumbnailCacheEntry{
				texture:       ebiten.NewImageFromImage(result.image),
				fadeStartedAt: time.Now(),
			}
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

	now := time.Now()
	for _, item := range items {
		if !item.current {
			v.drawThumbnailItem(screen, item, now)
		}
	}
	for _, item := range items {
		if item.current {
			v.drawThumbnailItem(screen, item, now)
			break
		}
	}
}

func (v *Viewer) drawThumbnailItem(screen *ebiten.Image, item thumbnailItem, now time.Time) {
	entry := v.thumbnailCache[item.path]
	if entry == nil || entry.texture == nil {
		return
	}
	hovered := item.path == v.hoveredThumbnailPath
	itemOpacity := thumbnailItemDisplayOpacity(item.distance, hovered) * v.thumbnailOpacity * thumbnailLoadOpacity(entry, now)
	drawThumbnailTexture(screen, entry.texture, item.rect, itemOpacity)
}

func thumbnailLoadOpacity(entry *thumbnailCacheEntry, now time.Time) float64 {
	if entry == nil {
		return 0
	}
	if entry.fadeStartedAt.IsZero() || !now.Before(entry.fadeStartedAt.Add(thumbnailLoadFadeDuration)) {
		return 1
	}
	progress := clampFloat64(float64(now.Sub(entry.fadeStartedAt))/float64(thumbnailLoadFadeDuration), 0, 1)
	// Smoothstep keeps both ends of the fade soft while remaining deterministic.
	return progress * progress * (3 - 2*progress)
}

func (v *Viewer) thumbnailLoadFadeActive(now time.Time) bool {
	if v.thumbnailOpacity <= 0 {
		return false
	}
	for _, entry := range v.thumbnailCache {
		if entry != nil && thumbnailLoadOpacity(entry, now) < 1 {
			return true
		}
	}
	return false
}

func thumbnailItemOpacity(distance int) float64 {
	return clampFloat64(0.8-float64(distance)*0.1, 0, 0.8)
}

func thumbnailItemDisplayOpacity(distance int, hovered bool) float64 {
	if hovered {
		return 1
	}
	return thumbnailItemOpacity(distance)
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

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (v *Viewer) thumbnailPathAt(x, y int) (string, bool) {
	if v.thumbnailOpacity <= 0 {
		return "", false
	}
	items := v.currentThumbnailItems()
	for _, item := range items {
		if item.current && pointInRect(x, y, item.rect) {
			return item.path, true
		}
	}
	for index := len(items) - 1; index >= 0; index-- {
		item := items[index]
		if pointInRect(x, y, item.rect) {
			return item.path, true
		}
	}
	return "", false
}

func (v *Viewer) pointInThumbnailStrip(x, y int) bool {
	if v.thumbnailOpacity <= 0 {
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
	v.hoveredThumbnailPath = ""
	if path, ok := v.thumbnailPathAt(mouseX, mouseY); ok {
		v.hoveredThumbnailPath = path
	}
}

func (v *Viewer) shouldShowThumbnails(now time.Time, mouseX, mouseY int) bool {
	return v.imageA != nil &&
		mouseX >= 0 && mouseX < v.windowWidth &&
		mouseY >= maxInt(0, v.windowHeight-thumbnailRevealDistance) && mouseY < v.windowHeight &&
		!v.cursorIdle(now)
}
