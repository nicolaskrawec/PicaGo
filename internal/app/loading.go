package app

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	imagedata "viewergo/internal/image"
	"viewergo/internal/render"
)

func (v *Viewer) startAsyncImageFileLoad(path string, slot asyncImageSlot, resetView, animateFit bool) {
	v.nextImageLoadID++
	id := v.nextImageLoadID
	// A new request supersedes a preview whose full-resolution upload has not
	// started yet. The old decode may still finish in its goroutine, but its
	// result is already invalidated by the new request ID.
	v.pendingHighRes[slot] = nil
	v.trackPendingImageLoad(slot, id)
	v.loadingImageName = filepath.Base(path)
	v.loadError = ""
	ebiten.SetWindowTitle(windowTitle(v.loadingImageName + " loading..."))

	go func() {
		decoded, err := imagedata.DecodeFileForDisplay(path, v.previewDimension())
		v.imageLoadResults <- asyncImageResult{
			id:          id,
			slot:        slot,
			decoded:     decoded,
			err:         err,
			resetView:   resetView,
			animateFit:  animateFit,
			activateFit: true,
		}
	}()
}

func (v *Viewer) startAsyncImageFSLoad(fsys fs.FS, path string, slot asyncImageSlot, resetView, animateFit bool) {
	v.nextImageLoadID++
	id := v.nextImageLoadID
	v.pendingHighRes[slot] = nil
	v.trackPendingImageLoad(slot, id)
	v.loadingImageName = filepath.Base(path)
	v.loadError = ""
	ebiten.SetWindowTitle(windowTitle(v.loadingImageName + " loading..."))

	go func() {
		decoded, err := imagedata.DecodeFSForDisplay(fsys, path, v.previewDimension())
		v.imageLoadResults <- asyncImageResult{
			id:          id,
			slot:        slot,
			decoded:     decoded,
			err:         err,
			resetView:   resetView,
			animateFit:  animateFit,
			activateFit: true,
		}
	}()
}

func (v *Viewer) trackPendingImageLoad(slot asyncImageSlot, id int) {
	switch slot {
	case asyncImageSlotA:
		v.pendingImageLoadAID = id
	case asyncImageSlotB:
		v.pendingImageLoadBID = id
	}
}

func (v *Viewer) collectAsyncImageLoads() {
	for {
		select {
		case result := <-v.imageLoadResults:
			v.applyAsyncImageLoad(result)
		default:
			return
		}
	}
}

func (v *Viewer) promotePendingHighResImages() {
	now := time.Now()
	for i, pending := range v.pendingHighRes {
		if pending == nil || now.Before(pending.readyAt) {
			continue
		}
		// Keep the initial full-resolution upload out of the animated fit
		// period. The preview is sufficient while the image zooms in.
		if pending.slot == asyncImageSlotA && v.initialPrefetchPending && (v.pendingResetFit || v.viewAnimationActive || v.openingAnimationActive) {
			continue
		}
		current := v.imageA
		if pending.slot == asyncImageSlotB {
			current = v.imageB
		}
		if current == pending.loaded {
			imagedata.UpgradeLoadedImage(pending.loaded, pending.decoded)
		}
		if pending.slot == asyncImageSlotA && v.initialPrefetchPending {
			v.initialPrefetchPending = false
			v.prefetchAdjacentImages()
		}
		v.pendingHighRes[i] = nil
	}
}

func (v *Viewer) previewDimension() int {
	dimension := v.windowWidth
	if v.windowHeight > dimension {
		dimension = v.windowHeight
	}
	if dimension < 1 {
		return 1920
	}
	return dimension
}

func (v *Viewer) collectPrefetchedImages() {
	for {
		select {
		case result := <-v.prefetchResults:
			delete(v.prefetchInFlight, result.path)
			if result.err == nil && result.decoded != nil {
				v.cachePrefetchedImage(result.path, result.decoded)
			} else {
				result.decoded.Release()
			}
		default:
			return
		}
	}
}

func (v *Viewer) applyAsyncImageLoad(result asyncImageResult) {
	if !v.isCurrentImageLoad(result.slot, result.id) {
		result.decoded.Release()
		return
	}
	v.trackPendingImageLoad(result.slot, 0)
	v.loadingImageName = ""

	if result.err != nil {
		result.decoded.Release()
		v.loadError = result.err.Error()
		ebiten.SetWindowTitle(windowTitle("load failed"))
		return
	}

	v.applyDecodedImage(result.slot, result.decoded, result.resetView, result.animateFit, result.activateFit)
}

func (v *Viewer) applyDecodedImage(slot asyncImageSlot, decoded *imagedata.DecodedImage, resetView, animateFit, activateFit bool) {
	loaded := imagedata.NewLoadedPreviewImage(decoded)
	if loaded == nil {
		decoded.Release()
		v.loadError = "image load failed"
		ebiten.SetWindowTitle(windowTitle("load failed"))
		return
	}

	v.loadError = ""
	v.pendingHighRes[slot] = nil
	v.initialPrefetchPending = false
	if v.imageA != nil && v.imageA.FilePath != "" {
		v.imageViewStates[imageViewStateKey(v.imageA.FilePath)] = v.targetView
	}
	savedView, hasSavedView := v.imageViewStates[imageViewStateKey(decoded.FilePath)]
	hasSavedZoom := hasSavedView && savedView.Zoom > 0
	var oldLoaded *imagedata.LoadedImage
	var oldDecoded *imagedata.DecodedImage
	if resetView {
		if hasSavedView {
			v.stopViewAnimation(true)
			v.view = savedView
			v.targetView = savedView
		} else {
			v.view = render.View{}
			v.targetView = v.view
			v.stopViewAnimation(false)
		}
	}

	switch slot {
	case asyncImageSlotA:
		oldLoaded = v.imageA
		oldDecoded = v.decodedA
		v.imageA = loaded
		v.decodedA = decoded
		if v.imageB == nil {
			v.mode = displayModeSingleA
		}
		v.circleMaskDiameter = defaultCircleMaskDiameterRatio
		if activateFit && !hasSavedZoom {
			v.fitMode = true
			v.pendingResetFit = true
			v.animateInitialFit = animateFit
			v.skipNextFitAnimation = !animateFit
		} else if hasSavedZoom {
			v.fitMode = false
			v.pendingResetFit = false
			v.animateInitialFit = false
			v.skipNextFitAnimation = false
		}
	case asyncImageSlotB:
		oldLoaded = v.imageB
		oldDecoded = v.decodedB
		v.imageB = loaded
		v.decodedB = decoded
		if v.imageA != nil {
			v.mode = displayModeCompare
		}
	}
	oldLoaded.Release()
	if slot == asyncImageSlotA && oldDecoded != nil && oldLoaded != nil {
		// The image that was displayed becomes the new -1 entry.
		v.cachePrefetchedImage(oldLoaded.FilePath, oldDecoded)
	} else if oldDecoded != nil {
		oldDecoded.Release()
	}
	if fullResolutionUploadEnabled {
		// Give the preview a few frames to reach the screen before starting the
		// potentially expensive full-resolution GPU upload.
		v.pendingHighRes[slot] = &pendingHighResImage{
			slot: slot, loaded: loaded, decoded: decoded, readyAt: time.Now().Add(250 * time.Millisecond),
		}
	}
	if slot == asyncImageSlotA && animateFit {
		v.initialPrefetchPending = true
	}

	v.draggingImage = false
	v.draggingSlider = false
	v.leftMouseDown = false
	v.restoreClickPending = false
	ebiten.SetWindowTitle(windowTitle(loaded.FileName))
	if !(slot == asyncImageSlotA && animateFit) {
		v.prefetchAdjacentImages()
	}
}

func (v *Viewer) isCurrentImageLoad(slot asyncImageSlot, id int) bool {
	switch slot {
	case asyncImageSlotA:
		return id != 0 && id == v.pendingImageLoadAID
	case asyncImageSlotB:
		return id != 0 && id == v.pendingImageLoadBID
	default:
		return false
	}
}

func (v *Viewer) rememberCurrentImageView() {
	if v.imageA == nil || v.imageA.FilePath == "" || v.imageViewStates == nil {
		return
	}
	// targetView contains the final user-requested transform while an
	// animation is in progress, which is the state to restore later.
	key := imageViewStateKey(v.imageA.FilePath)
	state := v.targetView
	if v.invalidatedZoomStates[key] {
		state.Zoom = 0
	}
	v.imageViewStates[key] = state
}

func (v *Viewer) invalidateSavedImageZooms() {
	for key, state := range v.imageViewStates {
		state.Zoom = 0
		v.imageViewStates[key] = state
		v.invalidatedZoomStates[key] = true
	}
}

func (v *Viewer) markCurrentZoomChanged() {
	if v.imageA == nil || v.imageA.FilePath == "" {
		return
	}
	delete(v.invalidatedZoomStates, imageViewStateKey(v.imageA.FilePath))
}

func imageViewStateKey(path string) string {
	if absolutePath, err := filepath.Abs(path); err == nil {
		path = absolutePath
	}
	return strings.ToLower(filepath.Clean(path))
}
