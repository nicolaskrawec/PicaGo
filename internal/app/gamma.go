package app

import (
	"fmt"
	"math"
	"strings"

	"github.com/nicolaskrawec/PicaGo/internal/render"
)

const (
	defaultGamma = 1.0
	gammaStep    = 0.1
	minGamma     = 0.1
	maxGamma     = 4.0
	minExposure  = -4.0
	maxExposure  = 4.0
	minContrast  = 0.0
	maxContrast  = 4.0
)

func imageAdjustmentsActive(view render.View) bool {
	gamma := view.Gamma
	if gamma <= 0 {
		gamma = defaultGamma
	}
	contrast := view.Contrast
	if contrast <= 0 {
		contrast = 1
	}
	return math.Abs(gamma-defaultGamma) >= 0.001 ||
		math.Abs(view.Exposure) >= 0.001 ||
		math.Abs(contrast-1) >= 0.001
}

func (v *Viewer) imageAdjustmentsSummary(view render.View) string {
	gamma := view.Gamma
	if gamma <= 0 {
		gamma = defaultGamma
	}
	contrast := view.Contrast
	if contrast <= 0 {
		contrast = 1
	}
	adjustments := make([]string, 0, 3)
	if math.Abs(gamma-defaultGamma) >= 0.001 {
		adjustments = append(adjustments, fmt.Sprintf("%s %.1f", v.text("adjustment.gamma"), gamma))
	}
	if math.Abs(view.Exposure) >= 0.001 {
		adjustments = append(adjustments, fmt.Sprintf("%s %+.1f", v.text("adjustment.exposure"), view.Exposure))
	}
	if math.Abs(contrast-1) >= 0.001 {
		adjustments = append(adjustments, fmt.Sprintf("%s %.1f", v.text("adjustment.contrast"), contrast))
	}
	return v.message("notification.image_adjusted", strings.Join(adjustments, "   "))
}

func (v *Viewer) adjustGamma(wheelDelta float64) {
	gamma := v.view.Gamma
	if gamma <= 0 {
		gamma = defaultGamma
	}
	gamma = clampFloat64(gamma+wheelDelta*gammaStep, minGamma, maxGamma)
	v.view.Gamma = gamma
	v.targetView.Gamma = gamma
	v.rememberCurrentImageView()
	v.showCenterInfo("notification.gamma", gamma)
}

func (v *Viewer) adjustExposure(wheelDelta float64) {
	exposure := clampFloat64(v.view.Exposure+wheelDelta*0.1, minExposure, maxExposure)
	v.view.Exposure = exposure
	v.targetView.Exposure = exposure
	v.rememberCurrentImageView()
	v.showCenterInfo("notification.exposure", exposure)
}

func (v *Viewer) adjustContrast(wheelDelta float64) {
	contrast := v.view.Contrast
	if contrast <= 0 {
		contrast = 1
	}
	contrast = clampFloat64(contrast+wheelDelta*0.1, minContrast, maxContrast)
	v.view.Contrast = contrast
	v.targetView.Contrast = contrast
	v.rememberCurrentImageView()
	v.showCenterInfo("notification.contrast", contrast)
}
