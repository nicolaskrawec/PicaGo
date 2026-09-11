package app

import (
	stdimage "image"
	"image/color"
	"math"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	imagedata "viewergo/internal/image"
)

const (
	thumbnailMinSize        = 50.0
	thumbnailSizeAmplitude  = 30.0
	thumbnailSizeFalloff    = 50.0
	thumbnailGap            = 2
	thumbnailBottomMargin   = 10
	thumbnailMaxRadius      = 8
	thumbnailPreloadRadius  = 16
	thumbnailMoveDuration   = 280 * time.Millisecond
	thumbnailFrameAlpha     = 50
	thumbnailLeftExclusion  = 20
	thumbnailRightExclusion = 20
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
	opacity  float64
}

type thumbnailTransitionItem struct {
	path        string
	fromRect    stdimage.Rectangle
	toRect      stdimage.Rectangle
	fromOpacity float64
	toOpacity   float64
	current     bool
	distance    int
}

func thumbnailRadius(windowWidth int) int {
	if windowWidth <= 0 {
		return 0
	}
	for radius := 1; radius <= thumbnailMaxRadius; radius++ {
		left := thumbnailRect(windowWidth, 1, -radius)
		right := thumbnailRect(windowWidth, 1, radius)
		if left.Min.X < thumbnailBottomMargin || right.Max.X > windowWidth-thumbnailBottomMargin {
			return radius - 1
		}
	}
	return thumbnailMaxRadius
}

func thumbnailSizeAtX(x float64) int {
	scaledX := x / thumbnailSizeFalloff
	return int(math.Round(thumbnailMinSize + thumbnailSizeAmplitude/(1+scaledX*scaledX)))
}

func nextThumbnailSize(centerOffset, previousSize float64) int {
	size := thumbnailSizeAtX(centerOffset + previousSize/2 + thumbnailGap + thumbnailMinSize/2)
	for range 4 {
		nextCenterOffset := centerOffset + previousSize/2 + thumbnailGap + float64(size)/2
		nextSize := thumbnailSizeAtX(nextCenterOffset)
		if nextSize == size {
			break
		}
		size = nextSize
	}
	return size
}

func thumbnailRect(windowWidth, windowHeight, offset int) stdimage.Rectangle {
	centerX := windowWidth / 2
	bottom := maxInt(0, windowHeight-thumbnailBottomMargin)
	currentSize := thumbnailSizeAtX(0)
	current := stdimage.Rect(
		centerX-currentSize/2,
		bottom-currentSize,
		centerX+(currentSize-currentSize/2),
		bottom,
	)
	if offset == 0 {
		return current
	}

	// Lay out the right side first. Each thumbnail's center determines its
	// size, while the previous rectangle anchors it so integer rounding never
	// changes the requested gap. The left side is its exact mirror.
	centerOffset := 0.0
	previousSize := float64(currentSize)
	right := current
	for distance := 1; distance <= absInt(offset); distance++ {
		size := nextThumbnailSize(centerOffset, previousSize)
		centerOffset += previousSize/2 + thumbnailGap + float64(size)/2
		right = stdimage.Rect(right.Max.X+thumbnailGap, bottom-size, right.Max.X+thumbnailGap+size, bottom)
		previousSize = float64(size)
	}
	if offset > 0 {
		return right
	}
	return stdimage.Rect(2*centerX-right.Max.X, right.Min.Y, 2*centerX-right.Min.X, right.Max.Y)
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
			distance: absInt(offset), opacity: thumbnailItemOpacity(absInt(offset)),
		})
	}
	return items
}

func (v *Viewer) targetThumbnailItems() []thumbnailItem {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return nil
	}
	images, currentIndex, err := v.navigationImagePaths(v.imageA.FilePath)
	if err != nil {
		return nil
	}
	return visibleThumbnailItems(images, currentIndex, v.windowWidth, v.windowHeight)
}

func (v *Viewer) currentThumbnailItems() []thumbnailItem {
	return v.currentThumbnailItemsAt(time.Now())
}

func (v *Viewer) currentThumbnailItemsAt(now time.Time) []thumbnailItem {
	if !v.thumbnailAnimationActive || len(v.thumbnailAnimationItems) == 0 {
		return v.targetThumbnailItems()
	}
	progress := thumbnailAnimationProgress(v.thumbnailAnimationStart, now)
	items := make([]thumbnailItem, 0, len(v.thumbnailAnimationItems))
	for _, transition := range v.thumbnailAnimationItems {
		items = append(items, thumbnailItem{
			path:     transition.path,
			rect:     interpolateThumbnailRect(transition.fromRect, transition.toRect, progress),
			current:  transition.current,
			distance: transition.distance,
			opacity:  interpolateFloat64(transition.fromOpacity, transition.toOpacity, progress),
		})
	}
	return items
}

func thumbnailAnimationProgress(start, now time.Time) float64 {
	progress := clampFloat64(float64(now.Sub(start))/float64(thumbnailMoveDuration), 0, 1)
	return progress * progress * (3 - 2*progress)
}

func interpolateThumbnailRect(from, to stdimage.Rectangle, progress float64) stdimage.Rectangle {
	return stdimage.Rect(
		interpolateInt(from.Min.X, to.Min.X, progress),
		interpolateInt(from.Min.Y, to.Min.Y, progress),
		interpolateInt(from.Max.X, to.Max.X, progress),
		interpolateInt(from.Max.Y, to.Max.Y, progress),
	)
}

func interpolateInt(from, to int, progress float64) int {
	return int(math.Round(float64(from) + float64(to-from)*progress))
}

func interpolateFloat64(from, to, progress float64) float64 {
	return from + (to-from)*progress
}

func buildThumbnailTransition(
	currentItems []thumbnailItem,
	images []string,
	fromIndex, toIndex, windowWidth, windowHeight int,
) []thumbnailTransitionItem {
	targetItems := visibleThumbnailItems(images, toIndex, windowWidth, windowHeight)
	fromByPath := make(map[string]thumbnailItem, len(currentItems))
	targetByPath := make(map[string]thumbnailItem, len(targetItems))
	selectedPaths := make(map[string]bool, len(currentItems)+len(targetItems))
	for _, item := range currentItems {
		fromByPath[item.path] = item
		selectedPaths[item.path] = true
	}
	for _, item := range targetItems {
		targetByPath[item.path] = item
		selectedPaths[item.path] = true
	}

	transition := make([]thumbnailTransitionItem, 0, len(selectedPaths))
	for index, path := range images {
		if !selectedPaths[path] {
			continue
		}
		fromItem, hasFrom := fromByPath[path]
		if !hasFrom {
			fromItem = thumbnailItem{
				path: path,
				rect: thumbnailRect(windowWidth, windowHeight, index-fromIndex),
			}
		}
		toItem, hasTarget := targetByPath[path]
		if !hasTarget {
			toItem = thumbnailItem{
				path:     path,
				rect:     thumbnailRect(windowWidth, windowHeight, index-toIndex),
				distance: absInt(index - toIndex),
			}
		}
		transition = append(transition, thumbnailTransitionItem{
			path:        path,
			fromRect:    fromItem.rect,
			toRect:      toItem.rect,
			fromOpacity: fromItem.opacity,
			toOpacity:   toItem.opacity,
			current:     hasTarget && toItem.current,
			distance:    toItem.distance,
		})
	}
	return transition
}

func (v *Viewer) startThumbnailAnimation(fromPath, toPath string, now time.Time) {
	currentItems := v.currentThumbnailItemsAt(now)
	v.thumbnailAnimationActive = false
	if fromPath == "" || toPath == "" || fromPath == toPath ||
		filepath.Clean(filepath.Dir(fromPath)) != filepath.Clean(filepath.Dir(toPath)) {
		v.thumbnailAnimationItems = nil
		return
	}
	images, fromIndex, err := v.navigationImagePaths(fromPath)
	if err != nil || fromIndex < 0 {
		v.thumbnailAnimationItems = nil
		return
	}
	toIndex := -1
	for index, path := range images {
		if filepath.Base(path) == filepath.Base(toPath) {
			toIndex = index
			break
		}
	}
	if toIndex < 0 || toIndex == fromIndex {
		v.thumbnailAnimationItems = nil
		return
	}

	v.thumbnailAnimationItems = buildThumbnailTransition(
		currentItems, images, fromIndex, toIndex, v.windowWidth, v.windowHeight,
	)
	v.thumbnailAnimationStart = now
	v.thumbnailAnimationActive = len(v.thumbnailAnimationItems) > 0
}

func (v *Viewer) updateThumbnailAnimation(now time.Time) {
	if !v.thumbnailAnimationActive || now.Before(v.thumbnailAnimationStart.Add(thumbnailMoveDuration)) {
		return
	}
	v.stopThumbnailAnimation()
}

func (v *Viewer) stopThumbnailAnimation() {
	v.thumbnailAnimationActive = false
	v.thumbnailAnimationItems = nil
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
	v.pruneThumbnailCache(paths)

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

func (v *Viewer) pruneThumbnailCache(paths []string) {
	wanted := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		wanted[path] = struct{}{}
	}
	for path, entry := range v.thumbnailCache {
		if _, ok := wanted[path]; ok {
			continue
		}
		if entry != nil && entry.texture != nil {
			entry.texture.Deallocate()
		}
		delete(v.thumbnailCache, path)
	}
	for path := range v.thumbnailFailed {
		if _, ok := wanted[path]; !ok {
			delete(v.thumbnailFailed, path)
		}
	}
}

func (v *Viewer) startThumbnailLoad(path string) {
	if path == "" || v.thumbnailInFlight[path] || len(v.thumbnailInFlight) >= thumbnailWorkerLimit {
		return
	}
	v.thumbnailInFlight[path] = true
	droppedFS := v.droppedFolderFSForPath(path)
	go func() {
		v.decodeSlots <- struct{}{}
		var thumbnail stdimage.Image
		var err error
		if droppedFS != nil {
			thumbnail, err = imagedata.DecodeFSThumbnail(droppedFS, path, thumbnailMaxDimension)
		} else {
			thumbnail, err = imagedata.DecodeFileThumbnail(path, thumbnailMaxDimension)
		}
		<-v.decodeSlots
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
	// The split line is dragged through the bottom reveal area as well. Keep
	// the strip hidden for the whole slider interaction, including the frame
	// in which the drag starts.
	if v.thumbnailsDisabled() || v.thumbnailOpacity <= 0.01 {
		return
	}
	now := time.Now()
	items := v.currentThumbnailItemsAt(now)
	if len(items) == 0 {
		return
	}
	v.drawThumbnailFrame(screen)

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

func (v *Viewer) drawThumbnailFrame(screen *ebiten.Image) {
	bounds := thumbnailFrameRect(v.windowWidth, v.windowHeight, v.thumbnailOpacity)
	if bounds.Empty() {
		return
	}
	alpha := uint8(math.Round(thumbnailFrameAlpha * clampFloat64(v.thumbnailOpacity, 0, 1)))
	vector.FillRect(
		screen,
		float32(bounds.Min.X),
		float32(bounds.Min.Y),
		float32(bounds.Dx()),
		float32(bounds.Dy()),
		color.NRGBA{0, 0, 0, alpha},
		false,
	)
}

func thumbnailFrameRect(windowWidth, windowHeight int, opacity float64) stdimage.Rectangle {
	if windowWidth <= 0 || windowHeight <= 0 {
		return stdimage.Rectangle{}
	}
	frameHeight := thumbnailSizeAtX(0) + 2*thumbnailBottomMargin
	revealOffset := thumbnailRevealOffset(opacity)
	return stdimage.Rect(0, windowHeight-frameHeight+revealOffset, windowWidth, windowHeight+revealOffset)
}

func (v *Viewer) drawThumbnailItem(screen *ebiten.Image, item thumbnailItem, now time.Time) {
	entry := v.thumbnailCache[item.path]
	if entry == nil || entry.texture == nil {
		return
	}
	hovered := item.path == v.hoveredThumbnailPath
	itemOpacity := item.opacity
	if hovered {
		itemOpacity = 1
	}
	itemOpacity *= v.thumbnailOpacity * thumbnailLoadOpacity(entry, now)
	revealOffset := thumbnailRevealOffset(v.thumbnailOpacity)
	drawRect := item.rect
	drawRect.Min.Y += revealOffset
	drawRect.Max.Y += revealOffset
	drawThumbnailTexture(screen, entry.texture, drawRect, itemOpacity)
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

func thumbnailRevealOffset(opacity float64) int {
	// thumbnailOpacity already follows an exponential ease-out curve. Reusing
	// it for the position keeps the fade and upward movement synchronized.
	progress := clampFloat64(opacity, 0, 1)
	return int(math.Round(float64(thumbnailRevealSlideDistance) * (1 - progress)))
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
	return clampFloat64(1-float64(distance)*0.1, 0, 1)
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
	if v.thumbnailOpacity <= 0 || x < thumbnailLeftExclusion || x >= v.windowWidth-thumbnailRightExclusion ||
		pointInBottomLeftCorner(x, y, v.windowHeight, cornerCommandTolerance) ||
		pointInBottomRightCorner(x, y, v.windowWidth, v.windowHeight, cornerCommandTolerance) {
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
	if v.thumbnailOpacity <= 0 || x < thumbnailLeftExclusion || x >= v.windowWidth-thumbnailRightExclusion ||
		pointInBottomLeftCorner(x, y, v.windowHeight, cornerCommandTolerance) ||
		pointInBottomRightCorner(x, y, v.windowWidth, v.windowHeight, cornerCommandTolerance) {
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
		!v.thumbnailsDisabled() &&
		mouseX >= thumbnailLeftExclusion && mouseX < v.windowWidth-thumbnailRightExclusion &&
		mouseY >= maxInt(0, v.windowHeight-thumbnailRevealDistance) && mouseY < v.windowHeight &&
		!pointInBottomLeftCorner(mouseX, mouseY, v.windowHeight, cornerCommandTolerance) &&
		!pointInBottomRightCorner(mouseX, mouseY, v.windowWidth, v.windowHeight, cornerCommandTolerance) &&
		!v.cursorIdle(now)
}

func (v *Viewer) thumbnailsDisabled() bool {
	return v.draggingSlider || (v.mode == displayModeCompare && v.compareMask == compareMaskCircle)
}
