package app

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	imagedata "github.com/nicolaskrawec/PicaGo/internal/image"
)

type imageSortMode int

const (
	imageSortByNameAscending imageSortMode = iota
	imageSortByNameDescending
	imageSortByModificationDateAscending
	imageSortByModificationDateDescending
	imageSortByCreationDateAscending
	imageSortByCreationDateDescending
	imageSortModeCount
)

type navigationImage struct {
	path         string
	name         string
	modifiedTime time.Time
	createdTime  time.Time
}

func (mode imageSortMode) configValue() string {
	switch mode {
	case imageSortByNameDescending:
		return "nameDescending"
	case imageSortByModificationDateAscending:
		return "modificationDateAscending"
	case imageSortByModificationDateDescending:
		return "modificationDateDescending"
	case imageSortByCreationDateAscending:
		return "creationDateAscending"
	case imageSortByCreationDateDescending:
		return "creationDateDescending"
	default:
		return "nameAscending"
	}
}

func imageSortModeFromConfig(value string) imageSortMode {
	switch value {
	case "nameDescending":
		return imageSortByNameDescending
	case "modificationDateAscending":
		return imageSortByModificationDateAscending
	case "modificationDateDescending":
		return imageSortByModificationDateDescending
	case "creationDateAscending":
		return imageSortByCreationDateAscending
	case "creationDateDescending":
		return imageSortByCreationDateDescending
	default:
		return imageSortByNameAscending
	}
}

func (mode imageSortMode) translationKey() string {
	switch mode {
	case imageSortByNameDescending:
		return "sort.name_descending"
	case imageSortByModificationDateAscending:
		return "sort.modification_date_ascending"
	case imageSortByModificationDateDescending:
		return "sort.modification_date_descending"
	case imageSortByCreationDateAscending:
		return "sort.creation_date_ascending"
	case imageSortByCreationDateDescending:
		return "sort.creation_date_descending"
	default:
		return "sort.name_ascending"
	}
}

func sortNavigationImages(images []navigationImage, mode imageSortMode) {
	sort.SliceStable(images, func(i, j int) bool {
		left, right := images[i], images[j]
		switch mode {
		case imageSortByNameDescending:
			return left.name > right.name
		case imageSortByModificationDateAscending:
			if !left.modifiedTime.Equal(right.modifiedTime) {
				return left.modifiedTime.Before(right.modifiedTime)
			}
		case imageSortByModificationDateDescending:
			if !left.modifiedTime.Equal(right.modifiedTime) {
				return left.modifiedTime.After(right.modifiedTime)
			}
		case imageSortByCreationDateAscending:
			if !left.createdTime.Equal(right.createdTime) {
				return left.createdTime.Before(right.createdTime)
			}
		case imageSortByCreationDateDescending:
			if !left.createdTime.Equal(right.createdTime) {
				return left.createdTime.After(right.createdTime)
			}
		}
		return left.name < right.name
	})
}

func (v *Viewer) currentImageSortMode(dir string) imageSortMode {
	return v.folderSortModes[folderSettingsKey(dir)]
}

func (v *Viewer) cycleImageSort() {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return
	}
	dir := filepath.Dir(v.imageA.FilePath)
	mode := (v.currentImageSortMode(dir) + 1) % imageSortModeCount
	if v.folderSortModes == nil {
		v.folderSortModes = make(map[string]imageSortMode)
	}
	if mode == imageSortByNameAscending {
		delete(v.folderSortModes, folderSettingsKey(dir))
	} else {
		v.folderSortModes[folderSettingsKey(dir)] = mode
	}
	_ = saveFolderSortModes(v.folderSortModes)

	// Rebuild the current directory immediately. The current image is found
	// again in the reordered list, so changing the order never changes it.
	if v.navigationFS != nil && v.isDroppedFolderPath(v.imageA.FilePath) {
		v.setDroppedFolderNavigation(v.navigationFS, v.navigationDirectory)
	} else {
		v.navigationDirectory = ""
		v.navigationImages = nil
	}
	v.stopThumbnailAnimation()
	v.prefetchAdjacentImages()
	v.showCenterInfo("notification.sort", v.text(mode.translationKey()))
}

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
	droppedFS := v.droppedFolderFSForPath(nextPath)
	if loaded := v.takePrefetchedImage(nextPath); loaded != nil {
		// Invalidate an older asynchronous navigation result before applying
		// the cached image immediately.
		v.trackPendingImageLoad(asyncImageSlotA, 0)
		v.loadingImageName = ""
		v.applyLoadedImage(asyncImageSlotA, loaded, true, false, true, func() (*imagedata.DecodedImage, error) {
			if droppedFS != nil {
				return imagedata.DecodeFS(droppedFS, nextPath)
			}
			return imagedata.DecodeFile(nextPath)
		})
		return
	}

	if droppedFS != nil {
		v.startAsyncImageFSLoad(droppedFS, nextPath, asyncImageSlotA, true, false)
	} else {
		v.startAsyncImageFileLoad(nextPath, asyncImageSlotA, true, false)
	}
}

func (v *Viewer) setDroppedFolderNavigation(fsys fs.FS, dir string) []string {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		v.clearDroppedFolderNavigation()
		return nil
	}
	items := make([]navigationImage, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !imagedata.IsSupportedFile(entry.Name()) {
			continue
		}
		item := navigationImage{path: path.Join(dir, entry.Name()), name: entry.Name()}
		if info, infoErr := entry.Info(); infoErr == nil {
			item.modifiedTime = info.ModTime()
		}
		items = append(items, item)
	}
	sortNavigationImages(items, v.currentImageSortMode(dir))
	v.navigationDirectory = dir
	v.navigationImages = make([]string, len(items))
	for i, item := range items {
		v.navigationImages[i] = item.path
	}
	v.navigationFS = fsys
	return v.navigationImages
}

func (v *Viewer) clearDroppedFolderNavigation() {
	v.navigationFS = nil
	v.navigationDirectory = ""
	v.navigationImages = nil
}

func (v *Viewer) isDroppedFolderPath(path string) bool {
	if v.navigationFS == nil {
		return false
	}
	for _, candidate := range v.navigationImages {
		if candidate == path {
			return true
		}
	}
	return false
}

func (v *Viewer) droppedFolderFSForPath(path string) fs.FS {
	if v.isDroppedFolderPath(path) {
		return v.navigationFS
	}
	return nil
}

func (v *Viewer) navigationImagePaths(filePath string) ([]string, int, error) {
	if v.isDroppedFolderPath(filePath) {
		for index, path := range v.navigationImages {
			if path == filePath {
				return v.navigationImages, index, nil
			}
		}
	}
	dir := filepath.Dir(filePath)
	currentName := filepath.Base(filePath)

	if v.navigationDirectory == "" || v.navigationDirectory != dir {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, -1, err
		}

		mode := v.currentImageSortMode(dir)
		items := make([]navigationImage, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !imagedata.IsSupportedFile(entry.Name()) {
				continue
			}
			item := navigationImage{
				path: filepath.Join(dir, entry.Name()),
				name: entry.Name(),
			}
			if mode != imageSortByNameAscending && mode != imageSortByNameDescending {
				if info, infoErr := entry.Info(); infoErr == nil {
					item.modifiedTime = info.ModTime()
					item.createdTime = fileCreationTime(item.path, info)
				}
			}
			items = append(items, item)
		}
		sortNavigationImages(items, mode)
		images := make([]string, len(items))
		for index, item := range items {
			images[index] = item.path
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
	droppedFS := v.droppedFolderFSForPath(path)
	go func() {
		v.decodeSlots <- struct{}{}
		var decoded *imagedata.DecodedImage
		var err error
		if droppedFS != nil {
			decoded, err = imagedata.DecodeFSForDisplay(droppedFS, path, maxDimension)
		} else {
			decoded, err = imagedata.DecodeFileForDisplay(path, maxDimension)
		}
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
