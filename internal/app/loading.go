package app

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"path/filepath"
	"runtime"
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
	ebiten.SetWindowTitle(windowTitle(v.message("status.loading", v.loadingImageName)))
	maxDimension := v.previewDimension()

	go func() {
		v.decodeSlots <- struct{}{}
		decoded, err := imagedata.DecodeFileForDisplay(path, maxDimension)
		<-v.decodeSlots
		v.imageLoadResults <- asyncImageResult{
			id:          id,
			slot:        slot,
			decoded:     decoded,
			err:         err,
			resetView:   resetView,
			animateFit:  animateFit,
			activateFit: true,
			loadFull: func() (*imagedata.DecodedImage, error) {
				return imagedata.DecodeFile(path)
			},
		}
		ebiten.ScheduleFrame()
	}()
}

func (v *Viewer) startAsyncImageFSLoad(fsys fs.FS, path string, slot asyncImageSlot, resetView, animateFit bool) {
	v.nextImageLoadID++
	id := v.nextImageLoadID
	v.pendingHighRes[slot] = nil
	v.trackPendingImageLoad(slot, id)
	v.loadingImageName = filepath.Base(path)
	v.loadError = ""
	ebiten.SetWindowTitle(windowTitle(v.message("status.loading", v.loadingImageName)))
	maxDimension := v.previewDimension()

	go func() {
		v.decodeSlots <- struct{}{}
		decoded, err := imagedata.DecodeFSForDisplay(fsys, path, maxDimension)
		<-v.decodeSlots
		v.imageLoadResults <- asyncImageResult{
			id:          id,
			slot:        slot,
			decoded:     decoded,
			err:         err,
			resetView:   resetView,
			animateFit:  animateFit,
			activateFit: true,
			loadFull: func() (*imagedata.DecodedImage, error) {
				return imagedata.DecodeFS(fsys, path)
			},
		}
		ebiten.ScheduleFrame()
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
		// Keep the full-resolution decode out of the animated fit period. The
		// screen-sized preview is sufficient while the image zooms in.
		if pending.slot == asyncImageSlotA && v.prefetchAfterHighRes == pending.loaded && (v.pendingResetFit || v.viewAnimationActive || v.openingAnimationActive) {
			continue
		}
		current := v.imageA
		if pending.slot == asyncImageSlotB {
			current = v.imageB
		}
		v.pendingHighRes[i] = nil
		if current != pending.loaded || pending.loadFull == nil {
			continue
		}
		go func(pending *pendingHighResImage) {
			v.decodeSlots <- struct{}{}
			decoded, err := pending.loadFull()
			<-v.decodeSlots
			v.highResResults <- highResImageResult{
				slot: pending.slot, loaded: pending.loaded, decoded: decoded, err: err,
			}
			ebiten.ScheduleFrame()
		}(pending)
	}
}

func (v *Viewer) collectHighResolutionImages() {
	for {
		select {
		case result := <-v.highResResults:
			current := v.imageA
			if result.slot == asyncImageSlotB {
				current = v.imageB
			}
			if result.err == nil && result.decoded != nil && current == result.loaded {
				imagedata.UpgradeLoadedImage(result.loaded, result.decoded)
			}
			// Uploading copies pixels to the GPU. Never retain the full CPU
			// decode after this point, including stale and failed requests.
			result.decoded.Release()
			if result.slot == asyncImageSlotA && current == result.loaded && v.prefetchAfterHighRes == result.loaded {
				v.prefetchAfterHighRes = nil
				v.prefetchAdjacentImages()
			}
		default:
			return
		}
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
				loaded := imagedata.NewLoadedPreviewImage(result.decoded)
				result.decoded.Release()
				v.cachePrefetchedImage(result.path, loaded)
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
		v.loadError = v.localizedImageError(result.err)
		ebiten.SetWindowTitle(windowTitle(v.text("status.load_failed")))
		return
	}

	v.applyDecodedImage(result.slot, result.decoded, result.resetView, result.animateFit, result.activateFit, result.loadFull)
}

func (v *Viewer) applyDecodedImage(slot asyncImageSlot, decoded *imagedata.DecodedImage, resetView, animateFit, activateFit bool, loadFull imageDecodeFunc) {
	loaded := imagedata.NewLoadedPreviewImage(decoded)
	if loaded == nil {
		decoded.Release()
		v.loadError = v.text("error.image_load_failed")
		ebiten.SetWindowTitle(windowTitle(v.text("status.load_failed")))
		return
	}
	decoded.Release()
	v.applyLoadedImage(slot, loaded, resetView, animateFit, activateFit, loadFull)
}

func (v *Viewer) localizedImageError(err error) string {
	switch {
	case errors.Is(err, imagedata.ErrFileTooLarge):
		return v.text("error.image_file_too_large")
	case errors.Is(err, imagedata.ErrTooManyPixels):
		return v.text("error.image_too_many_pixels")
	default:
		// Decoder and operating-system errors are dynamic and not suitable as
		// translation keys. Keep the detail in the debug log and show a stable,
		// localizable message in the UI.
		debugf("image load failed: %v", err)
		return v.text("error.image_load_failed")
	}
}

func (v *Viewer) applyLoadedImage(slot asyncImageSlot, loaded *imagedata.LoadedImage, resetView, animateFit, activateFit bool, loadFull imageDecodeFunc) {

	v.loadError = ""
	v.pendingHighRes[slot] = nil
	if slot == asyncImageSlotA {
		fromPath := ""
		if v.imageA != nil {
			fromPath = v.imageA.FilePath
		}
		v.startThumbnailAnimation(fromPath, loaded.FilePath, time.Now())
	}
	v.rememberCurrentImageView()
	// Persist the previous image's state when a new image is applied. Changes
	// made while viewing the current image remain in memory until this point
	// or until the application closes.
	_ = saveImageViewStates(v.imageViewStates)
	savedView, hasSavedView := v.imageViewStates[imageViewStateKey(loaded.FilePath)]
	sessionView, hasSessionView := v.sessionImageViews[imageViewStateKey(loaded.FilePath)]
	restoreSessionView := slot == asyncImageSlotA && resetView && hasSessionView && sessionView.Zoom > 0
	var oldLoaded *imagedata.LoadedImage
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
		if restoreSessionView {
			v.view.Zoom = sessionView.Zoom
			v.view.OffsetX = sessionView.OffsetX
			v.view.OffsetY = sessionView.OffsetY
			v.targetView.Zoom = sessionView.Zoom
			v.targetView.OffsetX = sessionView.OffsetX
			v.targetView.OffsetY = sessionView.OffsetY
			// Persisted view states omit fields whose default value is zero.
			// Fit normally fills these fields in, but a session view skips the
			// fit pass, so restore their rendering defaults explicitly.
			if v.view.Alpha <= 0 {
				v.view.Alpha = 1
				v.targetView.Alpha = 1
			}
			if v.view.Gamma <= 0 {
				v.view.Gamma = defaultGamma
				v.targetView.Gamma = defaultGamma
			}
			if v.view.Contrast <= 0 {
				v.view.Contrast = 1
				v.targetView.Contrast = 1
			}
		}
	}

	switch slot {
	case asyncImageSlotA:
		oldLoaded = v.imageA
		v.imageA = loaded
		v.fullscreenZoomRestoreValid = false
		if v.imageB == nil {
			v.mode = displayModeSingleA
		}
		v.circleMaskDiameter = defaultCircleMaskDiameterRatio
		if activateFit && !restoreSessionView {
			v.fitMode = true
			v.pendingResetFit = true
			v.animateInitialFit = animateFit
			v.skipNextFitAnimation = !animateFit
		} else if restoreSessionView {
			// Returning to an image with a session zoom must not trigger the
			// normal fit pass scheduled for newly opened images.
			v.fitMode = false
			v.pendingResetFit = false
			v.animateInitialFit = false
			v.skipNextFitAnimation = false
		}
	case asyncImageSlotB:
		oldLoaded = v.imageB
		v.imageB = loaded
		if v.imageA != nil {
			v.mode = displayModeCompare
		}
	}
	if slot == asyncImageSlotA && oldLoaded != nil {
		// Keep the previous image instantly navigable, but only as a
		// screen-sized GPU texture. The old full texture is released below.
		preview := imagedata.NewLoadedPreviewCopy(oldLoaded, v.previewDimension())
		v.cachePrefetchedImage(oldLoaded.FilePath, preview)
	}
	oldLoaded.Release()
	needsHighResolution := loadFull != nil && !loaded.IsFullResolution()
	if needsHighResolution {
		// Delay the decode itself rather than retaining full CPU pixels during
		// the preview animation.
		v.pendingHighRes[slot] = &pendingHighResImage{
			slot: slot, loaded: loaded, loadFull: loadFull, readyAt: time.Now().Add(250 * time.Millisecond),
		}
	}

	v.resetInteractionForLoadedImage()
	ebiten.SetWindowTitle(windowTitle(loaded.FileName))
	if slot == asyncImageSlotA {
		if needsHighResolution {
			v.prefetchAfterHighRes = loaded
		} else {
			v.prefetchAfterHighRes = nil
			v.prefetchAdjacentImages()
		}
	}
}

func (v *Viewer) resetInteractionForLoadedImage() {
	v.draggingImage = false
	v.draggingSlider = false
	v.restoreClickPending = false
	// Keep leftMouseDown latched until Update observes the physical button
	// release. Clearing it here can turn one long click into a second click if
	// an asynchronous load completes while the button is still held and the
	// thumbnail strip has already recentered around the new image.
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
	if v.imageA == nil || v.imageA.FilePath == "" {
		return
	}
	// targetView contains the final user-requested transform while an
	// animation is in progress, which is the state to restore later.
	key := imageViewStateKey(v.imageA.FilePath)
	state := v.targetView
	if v.sessionImageViews == nil {
		v.sessionImageViews = make(map[string]sessionImageView)
	}
	if v.fitMode || state.Zoom <= 0 {
		delete(v.sessionImageViews, key)
	} else {
		v.sessionImageViews[key] = sessionImageView{
			Zoom: state.Zoom, OffsetX: state.OffsetX, OffsetY: state.OffsetY,
		}
	}

	if v.imageViewStates == nil {
		return
	}
	state.Zoom = 0
	state.OffsetX = 0
	state.OffsetY = 0
	if imageViewStateIsDefault(state) {
		if _, exists := v.imageViewStates[key]; exists {
			delete(v.imageViewStates, key)
		}
		return
	}
	if previous, exists := v.imageViewStates[key]; exists && viewAlmostEqual(previous, state) {
		return
	}
	v.imageViewStates[key] = state
}

func imageViewStateIsDefault(state render.View) bool {
	alphaDefault := state.Alpha == 0 || state.Alpha == 1
	gammaDefault := state.Gamma <= 0 || state.Gamma == defaultGamma
	contrastDefault := state.Contrast <= 0 || state.Contrast == 1
	return alphaDefault &&
		!state.FlipHorizontal && !state.FlipVertical && state.Rotation == 0 &&
		state.RotationAngle == 0 && state.MirrorScaleX == 0 && state.MirrorScaleY == 0 &&
		gammaDefault && state.Exposure == 0 && contrastDefault
}

func imageViewStateKey(path string) string {
	absolutePath, err := filepath.Abs(path)
	if err == nil {
		path = absolutePath
	}
	path = filepath.Clean(path)
	// Windows paths are case-insensitive. On Unix-like systems, changing the
	// case would make distinct files such as Photo.jpg and photo.jpg share the
	// same persisted view state.
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	path = strings.ReplaceAll(path, "\\", "/")
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(path))
	return fmt.Sprintf("fnv1a64:%016x", hash.Sum64())
}
