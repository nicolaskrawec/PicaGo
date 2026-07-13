package app

import "time"

func (v *Viewer) slideshowInterval() time.Duration {
	seconds := v.preferences.SlideshowIntervalSeconds
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

func (v *Viewer) toggleSlideshow(now time.Time) {
	if v.slideshowPlaying {
		v.slideshowPlaying = false
		v.nextSlideshowAt = time.Time{}
		v.showCenterInfo("%s", "Diaporama : pause")
		return
	}

	images, _, err := v.navigationImagePathsForSlideshow()
	if err != nil || len(images) < 2 {
		v.showCenterInfo("%s", "Diaporama indisponible")
		return
	}
	v.slideshowPlaying = true
	v.nextSlideshowAt = now.Add(v.slideshowInterval())
	v.showCenterInfo("Diaporama : lecture (%d s)", int(v.slideshowInterval()/time.Second))
}

func (v *Viewer) updateSlideshow(now time.Time) {
	if !v.slideshowPlaying || now.Before(v.nextSlideshowAt) {
		return
	}
	v.nextSlideshowAt = now.Add(v.slideshowInterval())
	if v.pendingImageLoadAID != 0 || v.pendingImageLoadBID != 0 {
		return
	}
	_ = v.loadNextSlideshowImage()
}
