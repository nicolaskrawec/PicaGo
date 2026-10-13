package app

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"testing"
	"testing/fstest"
	"time"

	imagedata "github.com/nicolaskrawec/PicaGo/internal/image"
)

func TestSupersededImageDecodeLeavesQueue(t *testing.T) {
	v := Viewer{decodeSlots: make(chan struct{}, 1)}
	v.decodeSlots <- struct{}{}
	v.trackPendingImageLoad(asyncImageSlotA, 1)
	cancel := v.imageLoadCancel[asyncImageSlotA]
	finished := make(chan bool, 1)
	go func() { finished <- v.acquireImageDecode(cancel) }()
	v.trackPendingImageLoad(asyncImageSlotA, 2)
	select {
	case acquired := <-finished:
		if acquired {
			t.Fatal("superseded request acquired a decoder")
		}
	case <-time.After(time.Second):
		t.Fatal("superseded request still waiting for a decoder")
	}
	<-v.decodeSlots
	if v.acquireImageDecode(cancel) {
		t.Fatal("cancelled request acquired an available decoder")
	}
	if len(v.decodeSlots) != 0 {
		t.Fatal("cancelled request leaked a decoder slot")
	}
}

func TestBackgroundDecodeYieldsToForeground(t *testing.T) {
	v := Viewer{decodeSlots: make(chan struct{}, 1)}
	for _, slot := range []asyncImageSlot{asyncImageSlotA, asyncImageSlotB} {
		v.trackPendingImageLoad(slot, 1)
		if v.tryBackgroundDecode() {
			t.Fatal("background decode started while foreground was pending")
		}
		v.trackPendingImageLoad(slot, 0)
	}
	if !v.tryBackgroundDecode() {
		t.Fatal("idle decoder was not available")
	}
	if v.tryBackgroundDecode() {
		t.Fatal("busy decoder accepted background work")
	}
	<-v.decodeSlots
}

func TestHighResolutionWaitsWithoutQueuingStaleDecode(t *testing.T) {
	loaded := &imagedata.LoadedImage{}
	v := Viewer{imageA: loaded, decodeSlots: make(chan struct{}, 1)}
	v.decodeSlots <- struct{}{}
	called := make(chan struct{}, 1)
	pending := &pendingHighResImage{
		slot: asyncImageSlotA, loaded: loaded,
		loadFull: func() (*imagedata.DecodedImage, error) {
			called <- struct{}{}
			return nil, nil
		},
	}
	v.pendingHighRes[asyncImageSlotA] = pending
	v.promotePendingHighResImages()
	if v.pendingHighRes[asyncImageSlotA] != pending {
		t.Fatal("busy decoder consumed the pending full-resolution request")
	}
	v.imageA = &imagedata.LoadedImage{FilePath: "next.jpg"}
	<-v.decodeSlots
	v.promotePendingHighResImages()
	if v.pendingHighRes[asyncImageSlotA] != nil || len(v.decodeSlots) != 0 {
		t.Fatal("stale full-resolution request was not discarded")
	}
	select {
	case <-called:
		t.Fatal("stale full-resolution image was decoded")
	default:
	}
}

func TestPrefetchStartsDuringPreviewAndReusesRunningDecode(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 256, 128)), nil); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{
		"a.jpg": &fstest.MapFile{Data: encoded.Bytes()},
		"b.jpg": &fstest.MapFile{Data: encoded.Bytes()},
	}
	loaded := &imagedata.LoadedImage{FilePath: "a.jpg"}
	v := Viewer{
		imageA: loaded, windowWidth: 64, windowHeight: 32,
		navigationFS: fsys, navigationDirectory: ".", navigationImages: []string{"a.jpg", "b.jpg"},
		decodeSlots: make(chan struct{}, 2), prefetchResults: make(chan prefetchedImageResult, 2),
		prefetchInFlight: make(map[string]bool),
	}
	v.pendingHighRes[asyncImageSlotA] = &pendingHighResImage{loaded: loaded, readyAt: time.Now().Add(time.Hour)}
	v.prefetchAdjacentImages()
	if !v.prefetchInFlight["b.jpg"] {
		t.Fatal("neighbour was not prefetched while full resolution was pending")
	}
	v.loadNavigationImage("b.jpg")
	if v.pendingPrefetchPath != "b.jpg" || v.pendingImageLoadAID == 0 {
		t.Fatal("navigation did not reuse the running prefetch")
	}
	if v.pendingHighRes[asyncImageSlotA] != nil {
		t.Fatal("navigation retained the previous full-resolution request")
	}
	select {
	case result := <-v.prefetchResults:
		if result.err != nil {
			t.Fatal(result.err)
		}
		defer result.decoded.Release()
		if result.decoded.Image != nil || result.decoded.Preview.Bounds().Size() != image.Pt(64, 32) {
			t.Fatal("prefetch did not retain only the screen-sized preview")
		}
		if result.decoded.Width != 256 || result.decoded.Height != 128 {
			t.Fatal("prefetch lost original dimensions")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("prefetch did not finish")
	}
}

func TestPrefetchFailureDoesNotRetryEveryFrame(t *testing.T) {
	v := Viewer{
		imageA:       &imagedata.LoadedImage{FilePath: "a.jpg"},
		navigationFS: fstest.MapFS{}, navigationDirectory: ".", navigationImages: []string{"a.jpg", "b.jpg"},
		decodeSlots: make(chan struct{}, 2), prefetchResults: make(chan prefetchedImageResult, 1),
		prefetchInFlight: map[string]bool{"b.jpg": true}, prefetchFailed: make(map[string]bool),
	}
	v.prefetchResults <- prefetchedImageResult{path: "b.jpg", err: errors.New("invalid JPEG")}
	v.collectPrefetchedImages()
	v.prefetchAdjacentImages()
	if !v.prefetchFailed["b.jpg"] || v.prefetchInFlight["b.jpg"] || len(v.decodeSlots) != 0 {
		t.Fatal("failed prefetch was restarted")
	}
}

func TestStalePrefetchReleasesPixelsBeforeGPUUpload(t *testing.T) {
	decoded := &imagedata.DecodedImage{Preview: image.NewRGBA(image.Rect(0, 0, 8, 8))}
	v := Viewer{prefetchResults: make(chan prefetchedImageResult, 1)}
	v.prefetchResults <- prefetchedImageResult{path: "old.jpg", decoded: decoded}
	v.collectPrefetchedImages()
	if decoded.Preview != nil {
		t.Fatal("stale prefetch retained CPU pixels")
	}
}
