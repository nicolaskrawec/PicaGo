package app

import (
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

func TestThumbnailStripKeepsCenterAtDirectoryEdges(t *testing.T) {
	images := []string{"0", "1", "2", "3", "4", "5"}
	for _, currentIndex := range []int{0, len(images) - 1} {
		items := visibleThumbnailItems(images, currentIndex, 1280, 720)
		if len(items) != 5 {
			t.Fatalf("index %d: visible thumbnails = %d, want 5", currentIndex, len(items))
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
