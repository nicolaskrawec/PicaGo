package image

import (
	stdimage "image"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type loadedAnimation struct {
	frames    []*ebiten.Image
	delays    []time.Duration
	duration  time.Duration
	loopCount int
	startedAt time.Time
	index     int
	active    bool
}

func (loaded *LoadedImage) installAnimation(frames []stdimage.Image, delays []time.Duration, loopCount int) {
	if loaded == nil || len(frames) <= 1 || len(frames) != len(delays) {
		return
	}
	textures := make([]*ebiten.Image, len(frames))
	for i, frame := range frames {
		textures[i] = ebiten.NewImageFromImage(frame)
	}
	animation := &loadedAnimation{frames: textures, delays: append([]time.Duration(nil), delays...), loopCount: loopCount, startedAt: time.Now(), active: true}
	for _, delay := range animation.delays {
		animation.duration += delay
	}
	if loaded.GPUTexture != nil {
		loaded.GPUTexture.Deallocate()
	}
	loaded.animation = animation
	loaded.GPUTexture = textures[0]
}

func (loaded *LoadedImage) releaseAnimation() {
	if loaded == nil || loaded.animation == nil {
		return
	}
	for _, frame := range loaded.animation.frames {
		frame.Deallocate()
	}
	loaded.animation = nil
	loaded.GPUTexture = nil
}

// UpdateAnimation selects the frame for now and reports whether more animated
// frames remain. A GIF without a loop extension is played exactly once.
func (loaded *LoadedImage) UpdateAnimation(now time.Time) bool {
	if loaded == nil || loaded.animation == nil || loaded.animation.duration <= 0 {
		return false
	}
	a := loaded.animation
	elapsed := now.Sub(a.startedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	plays := 0 // LoopCount == 0 means forever.
	if a.loopCount < 0 {
		plays = 1
	} else if a.loopCount > 0 {
		plays = a.loopCount + 1
	}
	if plays > 0 && elapsed >= time.Duration(plays)*a.duration {
		a.index = len(a.frames) - 1
		a.active = false
		loaded.GPUTexture = a.frames[a.index]
		return false
	}
	position := elapsed % a.duration
	index := 0
	for index < len(a.delays)-1 && position >= a.delays[index] {
		position -= a.delays[index]
		index++
	}
	a.index = index
	loaded.GPUTexture = a.frames[index]
	return true
}

func (loaded *LoadedImage) IsAnimated() bool {
	return loaded != nil && loaded.animation != nil && loaded.animation.active
}

// RestartAnimation starts an animation when an image becomes visible. This is
// important for prefetched images, which may have spent time in the cache.
func (loaded *LoadedImage) RestartAnimation(now time.Time) {
	if loaded == nil || loaded.animation == nil {
		return
	}
	loaded.animation.startedAt = now
	loaded.animation.index = 0
	loaded.animation.active = true
	loaded.GPUTexture = loaded.animation.frames[0]
}
