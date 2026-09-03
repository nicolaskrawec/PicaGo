package app

import (
	stdimage "image"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	imagedata "viewergo/internal/image"
)

func pointInRect(x, y int, rect stdimage.Rectangle) bool {
	return x >= rect.Min.X && x < rect.Max.X && y >= rect.Min.Y && y < rect.Max.Y
}

func pointInTopRightCorner(x, y, width, tolerance int) bool {
	if width <= 0 || tolerance <= 0 {
		return false
	}
	return x >= width-tolerance && y >= 0 && y < tolerance
}

func pointInTopLeftCorner(x, y, tolerance int) bool {
	if tolerance <= 0 {
		return false
	}
	return x >= 0 && x < tolerance && y >= 0 && y < tolerance
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func clampInt(value, minValue, maxValue int) int {
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func lerpFloat(a, b, t float64) float64 {
	return a + (b-a)*t
}

func clampSliderPosition(position, minPosition, maxPosition float64) float64 {
	if maxPosition < minPosition {
		maxPosition = minPosition
	}
	if position < minPosition {
		return minPosition
	}
	if position > maxPosition {
		return maxPosition
	}
	return position
}

func clampFloat64(value, minValue, maxValue float64) float64 {
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (v *Viewer) helpText() string {
	return v.helpTextCategorized(true)
}

func (v *Viewer) helpTextCompact() string {
	return v.helpTextCategorized(false)
}

func (v *Viewer) helpTextCategorized(debug bool) string {
	fileA := "A"
	if v.imageA != nil && v.imageA.FileName != "" {
		fileA = imageInfoLabel(v.imageA)
	}
	fileB := "none"
	if v.imageB != nil && v.imageB.FileName != "" {
		fileB = imageInfoLabel(v.imageB)
	}
	syncMode := "off"
	if v.syncSliderWithImage {
		syncMode = "on"
	}
	shadowMode := "off"
	if v.showShadow {
		shadowMode = "on"
	}

	var graphicsInfo ebiten.DebugInfo
	ebiten.ReadDebugInfo(&graphicsInfo)
	lines := []string{
		"PicaGo " + Version + " - NkSoft",
		"Renderer: " + graphicsInfo.GraphicsLibrary.String(),
	}
	if debug {
		lines = append(lines,
			"Current zoom: "+strconv.Itoa(int(math.Round(v.view.Zoom*100)))+"%",
			"Current rotation: "+strconv.Itoa(v.view.Rotation*90)+"°",
			"Memory: "+v.memoryStatusText(),
			"Cache: "+v.prefetchStatusText(),
		)
	}
	lines = append(lines,
		"",
		"GEOMETRY",
		"Mouse wheel: zoom",
		"Up / Down: zoom",
		"Z / W: 100% / fit zoom",
		"R: fit to window",
		"Ctrl+Left/Right: rotate",
		"Shift+Left/Right: horizontal mirror",
		"Shift+Up/Down: vertical mirror",
		"",
		"ADJUSTMENTS",
		"Shift+mouse wheel: gamma",
		"Alt+mouse wheel: exposure / brightness",
		"Ctrl+Alt+mouse wheel: contrast",
		"",
		"COMPARISON",
		"C: circular mask / invert",
		"H / V: horizontal / vertical split",
		"Ctrl+mouse wheel: mask opacity",
		"Ctrl+Shift+mouse wheel: circular mask size",
		"L: slider sync "+syncMode,
		"",
		"OTHER",
		"B: background (desktop fullscreen / gray / black / white)",
		"P: slideshow play / pause",
		"S: shadow "+shadowMode,
		"F11: fullscreen",
		"Esc: quit",
		"",
		"NAVIGATION",
		"Left / Right: previous / next image",
		"1: "+fileA,
		"2: "+fileB,
	)
	return strings.Join(lines, "\n")
}

func imageInfoLabel(image *imagedata.LoadedImage) string {
	if image == nil {
		return ""
	}
	label := image.FileName
	if image.DecodeMethod != "" {
		label += " [" + image.DecodeMethod + "]"
	}
	return label
}

func (v *Viewer) memoryStatusText() string {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	toMiB := func(value uint64) string {
		return strconv.FormatUint(value/(1024*1024), 10) + " MiB"
	}
	return "alloc " + toMiB(stats.Alloc) + " / heap " + toMiB(stats.HeapInuse) + " / sys " + toMiB(stats.Sys) +
		" / cache " + strconv.Itoa(len(v.prefetchedImages)) + " / active " + strconv.Itoa(len(v.prefetchInFlight)) +
		" / thumbs " + strconv.Itoa(len(v.thumbnailCache)) + "/" + strconv.Itoa(len(v.thumbnailInFlight))
}

func (v *Viewer) prefetchStatusText() string {
	if v.imageA != nil && v.imageA.FilePath != "" {
		if paths, _, err := v.navigationImagePaths(v.imageA.FilePath); err == nil {
			status := make([]string, 0, len(paths))
			for _, path := range paths {
				switch {
				case path == v.imageA.FilePath:
					status = append(status, "["+filepath.Base(path)+"]")
				case v.prefetchedImages[path] != nil:
					status = append(status, filepath.Base(path))
				}
			}
			if len(status) > 0 {
				return strings.Join(status, " ")
			}
		}
	}

	status := make([]string, 0, len(v.prefetchOrder)+1)
	if v.imageA != nil && v.imageA.FileName != "" {
		status = append(status, "["+v.imageA.FileName+"]")
	}
	for _, path := range v.prefetchOrder {
		if _, ok := v.prefetchedImages[path]; ok {
			status = append(status, filepath.Base(path))
		}
	}
	if len(status) == 0 {
		return "empty"
	}
	return strings.Join(status, " ")
}
