package app

import (
	stdimage "image"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	imagedata "github.com/nicolaskrawec/PicaGo/internal/image"
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

func pointInBottomLeftCorner(x, y, height, tolerance int) bool {
	if height <= 0 || tolerance <= 0 {
		return false
	}
	return x >= 0 && x < tolerance && y >= height-tolerance && y < height
}

func pointInBottomRightCorner(x, y, width, height, tolerance int) bool {
	if width <= 0 || height <= 0 || tolerance <= 0 {
		return false
	}
	return x >= width-tolerance && x < width && y >= height-tolerance && y < height
}

func pointNearAnyCorner(x, y, width, height, clearance int) bool {
	if width <= 0 || height <= 0 || clearance <= 0 || x < 0 || y < 0 || x >= width || y >= height {
		return false
	}
	nearHorizontalEdge := x < clearance || x >= width-clearance
	nearVerticalEdge := y < clearance || y >= height-clearance
	return nearHorizontalEdge && nearVerticalEdge
}

func adjustmentIndicatorRect(windowHeight int) stdimage.Rectangle {
	const (
		width  = 32
		height = 36
	)
	centerY := windowHeight / 2
	return stdimage.Rect(0, centerY-height/2, width, centerY+height/2)
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
	fileB := v.text("state.none")
	if v.imageB != nil && v.imageB.FileName != "" {
		fileB = imageInfoLabel(v.imageB)
	}
	syncMode := v.text("state.off")
	if v.syncSliderWithImage {
		syncMode = v.text("state.on")
	}
	shadowMode := v.text("state.off")
	if v.showShadow {
		shadowMode = v.text("state.on")
	}
	sortMode := imageSortByNameAscending
	if v.imageA != nil && v.imageA.FilePath != "" {
		sortMode = v.currentImageSortMode(filepath.Dir(v.imageA.FilePath))
	}

	var graphicsInfo ebiten.DebugInfo
	ebiten.ReadDebugInfo(&graphicsInfo)
	lines := []string{
		"PicaGo " + Version + " - NkSoft",
		v.message("debug.renderer", graphicsInfo.GraphicsLibrary.String()),
	}
	if debug {
		lines = append(lines,
			v.message("debug.current_zoom", int(math.Round(v.view.Zoom*100))),
			v.message("debug.current_rotation", v.view.Rotation*90),
			v.message("debug.memory", v.memoryStatusText()),
			v.message("debug.cache", v.prefetchStatusText()),
		)
	}
	lines = append(lines,
		"",
		v.text("help.geometry"),
		v.text("help.mouse_wheel_zoom"),
		v.text("help.up_down_zoom"),
		v.text("help.zoom_100_fit"),
		v.text("help.fit_window"),
		v.text("help.rotate"),
		v.text("help.horizontal_mirror"),
		v.text("help.vertical_mirror"),
		"",
		v.text("help.adjustments"),
		v.text("help.corner_horizontal_mirror"),
		v.text("help.corner_gamma"),
		v.text("help.shift_gamma"),
		v.text("help.alt_exposure"),
		v.text("help.ctrl_alt_contrast"),
		"",
		v.text("help.comparison"),
		v.text("help.circular_mask"),
		v.text("help.split"),
		v.text("help.mask_opacity"),
		v.text("help.mask_size"),
		v.message("help.slider_sync", syncMode),
		"",
		v.text("help.other"),
		v.text("help.background"),
		v.text("help.slideshow"),
		v.message("help.image_order", v.text(sortMode.translationKey())),
		v.message("help.shadow", shadowMode),
		v.text("help.exif"),
		v.text("help.fullscreen"),
		v.text("help.quit"),
		"",
		v.text("help.navigation"),
		v.text("help.previous_next"),
		v.message("help.image_a", fileA),
		v.message("help.image_b", fileB),
	)
	return strings.Join(lines, "\n")
}

func (v *Viewer) exifText() string {
	if v.imageA == nil {
		return v.text("exif.title") + "\n\n" + v.text("exif.no_image")
	}

	lines := []string{
		v.text("exif.title") + " - " + v.imageA.FileName,
		v.message("exif.dimensions", v.imageA.Width, v.imageA.Height),
	}
	if len(v.imageA.EXIF) == 0 {
		return strings.Join(append(lines, "", v.text("exif.none")), "\n")
	}

	lines = append(lines, "")
	for _, tag := range v.imageA.EXIF {
		lines = append(lines, tag.Name+": "+tag.Value)
	}
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
	return v.message(
		"debug.memory_values",
		toMiB(stats.Alloc),
		toMiB(stats.HeapInuse),
		toMiB(stats.Sys),
		len(v.prefetchedImages),
		len(v.prefetchInFlight),
		len(v.thumbnailCache),
		len(v.thumbnailInFlight),
	)
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
		return v.text("state.empty")
	}
	return strings.Join(status, " ")
}
