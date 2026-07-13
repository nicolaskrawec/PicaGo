package app

import (
	stdimage "image"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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

func pointInBottomBar(x, y, windowWidth, windowHeight int) bool {
	if windowWidth <= 0 || windowHeight <= 0 {
		return false
	}
	return x >= bottomCommandCornerWidth &&
		x < windowWidth-bottomCommandCornerWidth &&
		y >= windowHeight-bottomCommandHeight &&
		y < windowHeight
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

	fileA := "A"
	if v.imageA != nil && v.imageA.FileName != "" {
		fileA = v.imageA.FileName
	}

	fileB := "aucune"
	if v.imageB != nil && v.imageB.FileName != "" {
		fileB = v.imageB.FileName
	}

	syncMode := "off"
	if v.syncSliderWithImage {
		syncMode = "on"
	}

	zoomPercent := int(math.Round(v.view.Zoom * 100))

	shadowMode := "off"
	if v.showShadow {
		shadowMode = "on"
	}

	blurMode := "off"
	if v.showBlur {
		blurMode = "on"
	}

	rotation := v.view.Rotation * 90
	return "PicaGo " + Version + " - NkSoft\nF1 aide\nZoom image : " + strconv.Itoa(zoomPercent) + "%\nRotation : " + strconv.Itoa(rotation) + " deg\nMÃ©moire : " + v.memoryStatusText() + "\nCache : " + v.prefetchStatusText() + "\nC : masque disque / inversion\nH : split horizontal\nV : split vertical\nShift+gauche/droite : miroir horizontal\nShift+haut/bas : miroir vertical\nCtrl+gauche : rotation anti-horaire\nCtrl+droite : rotation horaire\nMolette : zoom image\nShift+molette : zoom masque cercle\nCtrl+molette : opacité masque\nB : blur cercle " + blurMode + "\nZ : zoom 100% / maxi\nL : slide sync " + syncMode + "\nS : shadow " + shadowMode + "\nR : fit\n1 : " + fileA + "\n2 : " + fileB
}

func (v *Viewer) helpTextCompact() string {
	return v.helpTextCategorized(false)

	fileA := "A"
	if v.imageA != nil && v.imageA.FileName != "" {
		fileA = v.imageA.FileName
	}
	fileB := "aucune"
	if v.imageB != nil && v.imageB.FileName != "" {
		fileB = v.imageB.FileName
	}
	syncMode := "off"
	if v.syncSliderWithImage {
		syncMode = "on"
	}
	shadowMode := "off"
	if v.showShadow {
		shadowMode = "on"
	}
	blurMode := "off"
	if v.showBlur {
		blurMode = "on"
	}
	return "PicaGo " + Version + " - NkSoft\nF1 aide\nC : masque disque / inversion\nH : split horizontal\nV : split vertical\nShift+gauche/droite : miroir horizontal\nShift+haut/bas : miroir vertical\nCtrl+gauche/droite : rotation\nMolette : zoom image\nShift+molette : gamma image\nAlt+molette : exposition\nCtrl+Alt+molette : contraste\nCtrl+molette : opacité masque\nCtrl+Shift+molette : taille masque cercle\nB : blur cercle " + blurMode + "\nL : slide sync " + syncMode + "\nS : shadow " + shadowMode + "\nR : fit\n1 : " + fileA + "\n2 : " + fileB
}

func (v *Viewer) helpTextCategorized(debug bool) string {
	fileA := "A"
	if v.imageA != nil && v.imageA.FileName != "" {
		fileA = v.imageA.FileName
	}
	fileB := "aucune"
	if v.imageB != nil && v.imageB.FileName != "" {
		fileB = v.imageB.FileName
	}
	syncMode := "off"
	if v.syncSliderWithImage {
		syncMode = "on"
	}
	shadowMode := "off"
	if v.showShadow {
		shadowMode = "on"
	}
	blurMode := "off"
	if v.showBlur {
		blurMode = "on"
	}

	lines := []string{"PicaGo " + Version + " - NkSoft"}
	if debug {
		lines = append(lines,
			"Zoom actuel : "+strconv.Itoa(int(math.Round(v.view.Zoom*100)))+"%",
			"Rotation actuelle : "+strconv.Itoa(v.view.Rotation*90)+" deg",
			"Mémoire : "+v.memoryStatusText(),
			"Cache : "+v.prefetchStatusText(),
		)
	}
	lines = append(lines,
		"",
		"GEOMETRIE",
		"Molette : zoom image",
		"Haut / bas : zoom image",
		"Z / W : zoom 100% / maxi",
		"R : ajuster à la fenêtre",
		"Ctrl+gauche/droite : rotation",
		"Shift+gauche/droite : miroir horizontal",
		"Shift+haut/bas : miroir vertical",
		"",
		"AJUSTEMENTS",
		"Shift+molette : gamma",
		"Alt+molette : exposition / luminosité",
		"Ctrl+Alt+molette : contraste",
		"",
		"COMPARAISON",
		"C : masque circulaire / inversion",
		"H / V : split horizontal / vertical",
		"Ctrl+molette : opacité du masque",
		"Ctrl+Shift+molette : taille du masque circulaire",
		"B : flou du cercle "+blurMode,
		"L : synchronisation du slider "+syncMode,
		"",
		"DIVERS",
		"S : ombre "+shadowMode,
		"F11 : plein écran",
		"Esc : quitter",
		"",
		"NAVIGATION",
		"gauche / droite : image précédente / suivante",
		"1 : "+fileA,
		"2 : "+fileB,
	)
	return strings.Join(lines, "\n")
}

func (v *Viewer) memoryStatusText() string {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	toMiB := func(value uint64) string {
		return strconv.FormatUint(value/(1024*1024), 10) + " MiB"
	}
	return "alloc " + toMiB(stats.Alloc) + " / heap " + toMiB(stats.HeapInuse) + " / sys " + toMiB(stats.Sys) +
		" / cache " + strconv.Itoa(len(v.prefetchedImages)) + " / actifs " + strconv.Itoa(len(v.prefetchInFlight))
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
		return "vide"
	}
	return strings.Join(status, " ")
}
