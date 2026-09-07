package app

import (
	"fmt"
	stdimage "image"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	imagedata "viewergo/internal/image"
)

func TestCurrentThumbnailRemainsCentered(t *testing.T) {
	images := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"}
	items := visibleThumbnailItems(images, 4, 1280, 720)
	if len(items) != 9 {
		t.Fatalf("visible thumbnails = %d, want 9", len(items))
	}
	for _, item := range items {
		if !item.current {
			continue
		}
		if center := (item.rect.Min.X + item.rect.Max.X) / 2; center != 640 {
			t.Fatalf("current thumbnail center = %d, want 640", center)
		}
		return
	}
	t.Fatal("current thumbnail is missing")
}

func TestThumbnailStripDisplaysEightImagesOnEachSide(t *testing.T) {
	images := make([]string, 17)
	for index := range images {
		images[index] = fmt.Sprintf("%02d", index)
	}
	items := visibleThumbnailItems(images, 8, 1280, 720)
	if len(items) != 17 {
		t.Fatalf("visible thumbnails = %d, want 17", len(items))
	}
	if !items[8].current {
		t.Fatal("middle thumbnail is not the current image")
	}
}

func TestThumbnailStripKeepsCenterAtDirectoryEdges(t *testing.T) {
	images := []string{"0", "1", "2", "3", "4", "5"}
	for _, currentIndex := range []int{0, len(images) - 1} {
		items := visibleThumbnailItems(images, currentIndex, 1280, 720)
		if len(items) != len(images) {
			t.Fatalf("index %d: visible thumbnails = %d, want %d", currentIndex, len(items), len(images))
		}
		for _, item := range items {
			if item.current {
				if center := (item.rect.Min.X + item.rect.Max.X) / 2; center != 640 {
					t.Fatalf("index %d: current center = %d, want 640", currentIndex, center)
				}
			}
		}
	}
}

func TestSmallWindowShowsOnlyCurrentThumbnail(t *testing.T) {
	items := visibleThumbnailItems([]string{"0", "1", "2"}, 1, 200, 120)
	if len(items) != 1 || !items[0].current || items[0].path != "1" {
		t.Fatalf("small-window thumbnails = %+v, want current image only", items)
	}
}

func TestThumbnailSizeUsesHorizontalDistanceFromCenter(t *testing.T) {
	tests := []struct {
		x    float64
		want int
	}{
		{x: 0, want: 80},
		{x: 50, want: 65},
		{x: 100, want: 56},
		{x: 250, want: 51},
	}
	for _, test := range tests {
		if got := thumbnailSizeAtX(test.x); got != test.want {
			t.Errorf("x = %v: thumbnail size = %d, want %d", test.x, got, test.want)
		}
		if got := thumbnailSizeAtX(-test.x); got != test.want {
			t.Errorf("x = %v: thumbnail size = %d, want symmetric size %d", -test.x, got, test.want)
		}
	}
}

func TestThumbnailRevealOffsetMovesUpAsOpacityIncreases(t *testing.T) {
	if got := thumbnailRevealOffset(0); got != thumbnailRevealSlideDistance {
		t.Fatalf("hidden thumbnail offset = %d, want %d", got, thumbnailRevealSlideDistance)
	}
	if got := thumbnailRevealOffset(0.5); got != thumbnailRevealSlideDistance/2 {
		t.Fatalf("half-visible thumbnail offset = %d, want %d", got, thumbnailRevealSlideDistance/2)
	}
	if got := thumbnailRevealOffset(1); got != 0 {
		t.Fatalf("visible thumbnail offset = %d, want 0", got)
	}
}

func TestThumbnailRectsUseFormulaAndRemainSymmetric(t *testing.T) {
	current := thumbnailRect(1000, 700, 0)
	previousRight := current
	for distance := 1; distance <= thumbnailMaxRadius; distance++ {
		right := thumbnailRect(1000, 700, distance)
		left := thumbnailRect(1000, 700, -distance)
		x := float64(right.Min.X+right.Max.X)/2 - 500
		if got, want := right.Dx(), thumbnailSizeAtX(x); got != want {
			t.Fatalf("distance %d: width = %d, formula gives %d", distance, got, want)
		}
		if right.Dx() != right.Dy() {
			t.Fatalf("distance %d: thumbnail is %dx%d, want square", distance, right.Dx(), right.Dy())
		}
		if left.Dx() != right.Dx() || left.Min.X != 1000-right.Max.X || left.Max.X != 1000-right.Min.X {
			t.Fatalf("distance %d: left %v and right %v are not symmetric", distance, left, right)
		}
		if gap := right.Min.X - previousRight.Max.X; gap != thumbnailGap {
			t.Fatalf("distance %d: gap = %d, want %d", distance, gap, thumbnailGap)
		}
		previousRight = right
	}
}

func TestThumbnailTransitionMovesAndResizesSelectedImage(t *testing.T) {
	images := make([]string, 25)
	for index := range images {
		images[index] = fmt.Sprintf("%02d", index)
	}
	fromIndex, toIndex := 12, 13
	fromItems := visibleThumbnailItems(images, fromIndex, 1000, 700)
	transition := buildThumbnailTransition(fromItems, images, fromIndex, toIndex, 1000, 700)
	startedAt := time.Now()
	v := Viewer{
		thumbnailAnimationActive: true,
		thumbnailAnimationStart:  startedAt,
		thumbnailAnimationItems:  transition,
		windowWidth:              1000,
	}

	findItem := func(items []thumbnailItem, path string) thumbnailItem {
		t.Helper()
		for _, item := range items {
			if item.path == path {
				return item
			}
		}
		t.Fatalf("thumbnail %q is missing", path)
		return thumbnailItem{}
	}

	selectedPath := images[toIndex]
	start := findItem(v.currentThumbnailItemsAt(startedAt), selectedPath)
	middle := findItem(v.currentThumbnailItemsAt(startedAt.Add(thumbnailMoveDuration/2)), selectedPath)
	end := findItem(v.currentThumbnailItemsAt(startedAt.Add(thumbnailMoveDuration)), selectedPath)
	if start.rect != thumbnailRect(1000, 700, 1) {
		t.Fatalf("selected thumbnail starts at %v, want old position %v", start.rect, thumbnailRect(1000, 700, 1))
	}
	if end.rect != thumbnailRect(1000, 700, 0) || !end.current {
		t.Fatalf("selected thumbnail ends at %v (current %v), want centered current thumbnail", end.rect, end.current)
	}
	if middle.rect.Min.X >= start.rect.Min.X || middle.rect.Min.X <= end.rect.Min.X {
		t.Fatalf("selected thumbnail middle position %v is not between %v and %v", middle.rect, start.rect, end.rect)
	}
	if middle.rect.Dx() <= start.rect.Dx() || middle.rect.Dx() >= end.rect.Dx() {
		t.Fatalf("selected thumbnail middle width %d is not between %d and %d", middle.rect.Dx(), start.rect.Dx(), end.rect.Dx())
	}
}

func TestThumbnailTransitionFadesStripEdges(t *testing.T) {
	images := make([]string, 25)
	for index := range images {
		images[index] = fmt.Sprintf("%02d", index)
	}
	fromIndex, toIndex := 12, 13
	fromItems := visibleThumbnailItems(images, fromIndex, 600, 700)
	transition := buildThumbnailTransition(fromItems, images, fromIndex, toIndex, 600, 700)

	var entering, leaving thumbnailTransitionItem
	for _, item := range transition {
		switch item.path {
		case images[17]:
			entering = item
		case images[8]:
			leaving = item
		}
	}
	if entering.fromOpacity != 0 || entering.toOpacity <= 0 {
		t.Fatalf("entering thumbnail opacity = %v -> %v, want fade in", entering.fromOpacity, entering.toOpacity)
	}
	if leaving.fromOpacity <= 0 || leaving.toOpacity != 0 {
		t.Fatalf("leaving thumbnail opacity = %v -> %v, want fade out", leaving.fromOpacity, leaving.toOpacity)
	}
}

func TestInterruptedThumbnailTransitionStartsFromCurrentGeometry(t *testing.T) {
	images := make([]string, 25)
	for index := range images {
		images[index] = fmt.Sprintf("%02d", index)
	}
	first := buildThumbnailTransition(
		visibleThumbnailItems(images, 12, 1000, 700), images, 12, 13, 1000, 700,
	)
	startedAt := time.Now()
	v := Viewer{
		thumbnailAnimationActive: true,
		thumbnailAnimationStart:  startedAt,
		thumbnailAnimationItems:  first,
		windowWidth:              1000,
	}
	snapshot := v.currentThumbnailItemsAt(startedAt.Add(thumbnailMoveDuration / 3))
	second := buildThumbnailTransition(snapshot, images, 13, 14, 1000, 700)

	fromByPath := make(map[string]stdimage.Rectangle, len(second))
	for _, item := range second {
		fromByPath[item.path] = item.fromRect
	}
	for _, item := range snapshot {
		if got := fromByPath[item.path]; got != item.rect {
			t.Fatalf("thumbnail %q restarted at %v, want current geometry %v", item.path, got, item.rect)
		}
	}
}

func TestThumbnailAnimationStopsAfterDuration(t *testing.T) {
	startedAt := time.Now()
	v := Viewer{
		thumbnailAnimationActive: true,
		thumbnailAnimationStart:  startedAt,
		thumbnailAnimationItems:  []thumbnailTransitionItem{{path: "image"}},
	}
	v.updateThumbnailAnimation(startedAt.Add(thumbnailMoveDuration - time.Millisecond))
	if !v.thumbnailAnimationActive {
		t.Fatal("thumbnail animation stopped too early")
	}
	v.updateThumbnailAnimation(startedAt.Add(thumbnailMoveDuration))
	if v.thumbnailAnimationActive || v.thumbnailAnimationItems != nil {
		t.Fatal("thumbnail animation did not stop after its duration")
	}
}

func TestThumbnailsShowOnlyNearBottomWhileCursorIsActive(t *testing.T) {
	now := time.Now()
	v := Viewer{
		imageA:               &imagedata.LoadedImage{},
		windowWidth:          1000,
		windowHeight:         700,
		lastCursorActivityAt: now,
	}
	if !v.shouldShowThumbnails(now, 500, 699) {
		t.Fatal("active cursor near bottom did not reveal thumbnails")
	}
	if v.shouldShowThumbnails(now, 500, 500) {
		t.Fatal("cursor away from bottom revealed thumbnails")
	}
	if v.shouldShowThumbnails(now.Add(cursorIdleDelay), 500, 699) {
		t.Fatal("idle cursor kept thumbnails visible")
	}
}

func TestThumbnailVisibilityFadesInAndOut(t *testing.T) {
	now := time.Now()
	v := Viewer{
		imageA:               &imagedata.LoadedImage{},
		windowWidth:          1000,
		windowHeight:         700,
		lastCursorActivityAt: now,
	}
	v.updateThumbnailVisibility(now, 500, 699)
	visibleOpacity := v.thumbnailOpacity
	if visibleOpacity <= 0 {
		t.Fatal("thumbnail fade-in did not start")
	}
	v.updateThumbnailVisibility(now, 500, 500)
	if v.thumbnailOpacity >= visibleOpacity {
		t.Fatal("thumbnail fade-out did not start after leaving bottom edge")
	}
}

func TestThumbnailOpacityDecreasesWithDistance(t *testing.T) {
	wants := []float64{1, 0.9, 0.8, 0.7, 0.6}
	for distance, want := range wants {
		if got := thumbnailItemOpacity(distance); math.Abs(got-want) > 0.0001 {
			t.Errorf("distance %d opacity = %v, want %v", distance, got, want)
		}
	}
}

func TestHoveredThumbnailUsesFullOpacity(t *testing.T) {
	if got := thumbnailItemDisplayOpacity(4, true); got != 1 {
		t.Fatalf("hovered opacity = %v, want 1", got)
	}
	if got, want := thumbnailItemDisplayOpacity(4, false), thumbnailItemOpacity(4); got != want {
		t.Fatalf("opacity after hover = %v, want %v", got, want)
	}
}

func TestThumbnailLoadFadeProgress(t *testing.T) {
	startedAt := time.Now()
	entry := &thumbnailCacheEntry{fadeStartedAt: startedAt}
	if got := thumbnailLoadOpacity(entry, startedAt); got != 0 {
		t.Fatalf("opacity at fade start = %v, want 0", got)
	}
	if got := thumbnailLoadOpacity(entry, startedAt.Add(thumbnailLoadFadeDuration/2)); math.Abs(got-0.5) > 0.0001 {
		t.Fatalf("opacity halfway through fade = %v, want 0.5", got)
	}
	if got := thumbnailLoadOpacity(entry, startedAt.Add(thumbnailLoadFadeDuration)); got != 1 {
		t.Fatalf("opacity after fade = %v, want 1", got)
	}
}

func TestThumbnailLoadFadesAreIndependent(t *testing.T) {
	now := time.Now()
	older := &thumbnailCacheEntry{fadeStartedAt: now.Add(-thumbnailLoadFadeDuration * 3 / 4)}
	newer := &thumbnailCacheEntry{fadeStartedAt: now.Add(-thumbnailLoadFadeDuration / 4)}
	olderOpacity := thumbnailLoadOpacity(older, now)
	newerOpacity := thumbnailLoadOpacity(newer, now)
	if olderOpacity <= newerOpacity {
		t.Fatalf("older opacity %v should exceed newer opacity %v", olderOpacity, newerOpacity)
	}
}

func TestThumbnailsUseTwoPixelSpacing(t *testing.T) {
	current := thumbnailRect(1000, 700, 0)
	next := thumbnailRect(1000, 700, 1)
	if gap := next.Min.X - current.Max.X; gap != 2 {
		t.Fatalf("current thumbnail gap = %d, want 2", gap)
	}
	nextNext := thumbnailRect(1000, 700, 2)
	if gap := nextNext.Min.X - next.Max.X; gap != 2 {
		t.Fatalf("neighbor thumbnail gap = %d, want 2", gap)
	}
}

func TestThumbnailIsClickableDuringFadeInAndGapsAreReserved(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "a.png"),
		filepath.Join(dir, "b.png"),
		filepath.Join(dir, "c.png"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	v := Viewer{
		imageA:           &imagedata.LoadedImage{FilePath: paths[1]},
		windowWidth:      1000,
		windowHeight:     700,
		thumbnailOpacity: 0.35,
	}
	items := v.currentThumbnailItems()
	if len(items) != 3 {
		t.Fatalf("visible thumbnails = %d, want 3", len(items))
	}
	var current, next thumbnailItem
	for _, item := range items {
		if item.current {
			current = item
		} else if item.path == paths[2] {
			next = item
		}
	}
	x, y := (next.rect.Min.X+next.rect.Max.X)/2, (next.rect.Min.Y+next.rect.Max.Y)/2
	if path, ok := v.thumbnailPathAt(x, y); !ok || path != paths[2] {
		t.Fatalf("fade-in click selected %q, %v; want %q, true", path, ok, paths[2])
	}
	gapX := current.rect.Max.X
	if !v.pointInThumbnailStrip(gapX, y) {
		t.Fatal("gap between thumbnails was not reserved by the strip")
	}

	v.thumbnailOpacity = 0
	if _, ok := v.thumbnailPathAt(x, y); ok {
		t.Fatal("fully hidden thumbnail remained clickable")
	}
}

func TestThumbnailPreloadsSixteenImagesOnEachSide(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 41)
	for index := range paths {
		paths[index] = filepath.Join(dir, fmt.Sprintf("%02d.png", index))
		if err := os.WriteFile(paths[index], nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	v := Viewer{imageA: &imagedata.LoadedImage{FilePath: paths[20]}}
	wanted := v.wantedThumbnailPaths()
	wantCount := thumbnailPreloadRadius*2 + 1
	if len(wanted) != wantCount {
		t.Fatalf("preloaded thumbnail paths = %d, want %d", len(wanted), wantCount)
	}
	if wanted[0] != paths[20] {
		t.Fatalf("first preloaded path = %q, want current %q", wanted[0], paths[20])
	}
	wantStart := []string{paths[20], paths[19], paths[21], paths[18], paths[22], paths[17], paths[23]}
	for index, want := range wantStart {
		if wanted[index] != want {
			t.Fatalf("preload priority %d = %q, want %q", index, wanted[index], want)
		}
	}
}
