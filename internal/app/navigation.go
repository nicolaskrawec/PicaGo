package app

import (
	"os"
	"path/filepath"

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
	if decoded := v.takePrefetchedImage(nextPath); decoded != nil {
		// Invalidate an older asynchronous navigation result before applying
		// the cached image immediately.
		v.trackPendingImageLoad(asyncImageSlotA, 0)
		v.loadingImageName = ""
		v.applyDecodedImage(asyncImageSlotA, decoded, true, false, true)
		return nil
	}

	v.startAsyncImageFileLoad(nextPath, asyncImageSlotA, true, false)
	return nil
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
	go func() {
		v.prefetchSlots <- struct{}{}
		decoded, err := imagedata.DecodeFileForDisplay(path, 1920)
		<-v.prefetchSlots
		v.prefetchResults <- prefetchedImageResult{path: path, decoded: decoded, err: err}
	}()
}

func (v *Viewer) cachePrefetchedImage(path string, decoded *imagedata.DecodedImage) {
	if !v.isWantedPrefetchPath(path) {
		decoded.Release()
		return
	}
	if _, exists := v.prefetchedImages[path]; exists {
		decoded.Release()
		return
	}
	if len(v.prefetchOrder) >= prefetchedImageCacheLimit {
		oldest := v.prefetchOrder[0]
		v.prefetchOrder = v.prefetchOrder[1:]
		v.prefetchedImages[oldest].Release()
		delete(v.prefetchedImages, oldest)
	}
	v.prefetchedImages[path] = decoded
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
	decoded := v.prefetchedImages[path]
	if decoded != nil {
		decoded.Release()
		delete(v.prefetchedImages, path)
	}
	for i, cachedPath := range v.prefetchOrder {
		if cachedPath == path {
			v.prefetchOrder = append(v.prefetchOrder[:i], v.prefetchOrder[i+1:]...)
			break
		}
	}
}

func (v *Viewer) takePrefetchedImage(path string) *imagedata.DecodedImage {
	decoded := v.prefetchedImages[path]
	if decoded == nil {
		return nil
	}
	delete(v.prefetchedImages, path)
	for i, cachedPath := range v.prefetchOrder {
		if cachedPath == path {
			v.prefetchOrder = append(v.prefetchOrder[:i], v.prefetchOrder[i+1:]...)
			break
		}
	}
	return decoded
}
