package app

import (
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	imagedata "viewergo/internal/image"
)

func (v *Viewer) loadAdjacentImage(step int) error {
	if v.imageA == nil || v.imageA.FilePath == "" || step == 0 {
		return nil
	}

	images, currentIndex, err := v.navigationImagePaths(v.imageA.FilePath)
	if err != nil {
		return err
	}

	nextIndex := currentIndex + step
	if currentIndex < 0 || nextIndex < 0 || nextIndex >= len(images) {
		return nil
	}

	nextPath := images[nextIndex]
	v.loadNavigationImage(nextPath)
	return nil
}

func (v *Viewer) loadNextSlideshowImage() error {
	images, currentIndex, err := v.navigationImagePathsForSlideshow()
	if err != nil || len(images) < 2 || currentIndex < 0 {
		return err
	}
	v.loadNavigationImage(images[(currentIndex+1)%len(images)])
	return nil
}

func (v *Viewer) navigationImagePathsForSlideshow() ([]string, int, error) {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return nil, -1, nil
	}
	return v.navigationImagePaths(v.imageA.FilePath)
}

func (v *Viewer) loadNavigationImage(nextPath string) {
	if loaded := v.takePrefetchedImage(nextPath); loaded != nil {
		// Invalidate an older asynchronous navigation result before applying
		// the cached image immediately.
		v.trackPendingImageLoad(asyncImageSlotA, 0)
		v.loadingImageName = ""
		v.applyLoadedImage(asyncImageSlotA, loaded, true, false, true, func() (*imagedata.DecodedImage, error) {
			return imagedata.DecodeFile(nextPath)
		})
		return
	}

	v.startAsyncImageFileLoad(nextPath, asyncImageSlotA, true, false)
}

func (v *Viewer) navigationImagePaths(filePath string) ([]string, int, error) {
	dir := filepath.Dir(filePath)
	currentName := filepath.Base(filePath)

	if v.navigationDirectory == "" || v.navigationDirectory != dir {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, -1, err
		}

		images := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !imagedata.IsSupportedFile(entry.Name()) {
				continue
			}
			images = append(images, filepath.Join(dir, entry.Name()))
		}
		v.navigationDirectory = dir
		v.navigationImages = images
	}

	currentIndex := -1
	for index, path := range v.navigationImages {
		if filepath.Base(path) == currentName {
			currentIndex = index
		}
	}
	return v.navigationImages, currentIndex, nil
}

func (v *Viewer) prefetchAdjacentImages() {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return
	}

	images, currentIndex, err := v.navigationImagePaths(v.imageA.FilePath)
	if err != nil || currentIndex < 0 {
		return
	}

	wanted := make(map[string]bool, prefetchedImageCacheLimit)
	for _, offset := range []int{-1, 1, 2} {
		index := currentIndex + offset
		if index >= 0 && index < len(images) {
			wanted[images[index]] = true
		}
	}

	// Drop entries that belonged to the previous position before inserting
	// new ones. This prevents FIFO eviction from removing the new previous or
	// next image while stale entries are still in the cache.
	for _, path := range append([]string(nil), v.prefetchOrder...) {
		if !wanted[path] {
			v.removePrefetchedImage(path)
		}
	}

	for path := range wanted {
		v.startPrefetch(path)
	}
}

func (v *Viewer) startPrefetch(path string) {
	if _, ok := v.prefetchedImages[path]; ok || v.prefetchInFlight[path] {
		return
	}
	v.prefetchInFlight[path] = true
	maxDimension := v.previewDimension()
	go func() {
		v.decodeSlots <- struct{}{}
		decoded, err := imagedata.DecodeFileForDisplay(path, maxDimension)
		<-v.decodeSlots
		v.prefetchResults <- prefetchedImageResult{path: path, decoded: decoded, err: err}
		ebiten.ScheduleFrame()
	}()
}

func (v *Viewer) cachePrefetchedImage(path string, loaded *imagedata.LoadedImage) {
	if loaded == nil {
		return
	}
	if !v.isWantedPrefetchPath(path) {
		loaded.Release()
		return
	}
	if _, exists := v.prefetchedImages[path]; exists {
		loaded.Release()
		return
	}
	if len(v.prefetchOrder) >= prefetchedImageCacheLimit {
		oldest := v.prefetchOrder[0]
		v.prefetchOrder = v.prefetchOrder[1:]
		v.prefetchedImages[oldest].Release()
		delete(v.prefetchedImages, oldest)
	}
	v.prefetchedImages[path] = loaded
	v.prefetchOrder = append(v.prefetchOrder, path)
}

func (v *Viewer) isWantedPrefetchPath(path string) bool {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return false
	}
	images, currentIndex, err := v.navigationImagePaths(v.imageA.FilePath)
	if err != nil || currentIndex < 0 {
		return false
	}
	for _, offset := range []int{-1, 1, 2} {
		index := currentIndex + offset
		if index >= 0 && index < len(images) && images[index] == path {
			return true
		}
	}
	return false
}

func (v *Viewer) removePrefetchedImage(path string) {
	loaded := v.prefetchedImages[path]
	if loaded != nil {
		loaded.Release()
		delete(v.prefetchedImages, path)
	}
	for i, cachedPath := range v.prefetchOrder {
		if cachedPath == path {
			v.prefetchOrder = append(v.prefetchOrder[:i], v.prefetchOrder[i+1:]...)
			break
		}
	}
}

func (v *Viewer) takePrefetchedImage(path string) *imagedata.LoadedImage {
	loaded := v.prefetchedImages[path]
	if loaded == nil {
		return nil
	}
	delete(v.prefetchedImages, path)
	for i, cachedPath := range v.prefetchOrder {
		if cachedPath == path {
			v.prefetchOrder = append(v.prefetchOrder[:i], v.prefetchOrder[i+1:]...)
			break
		}
	}
	return loaded
}
