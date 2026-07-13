package app

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

func (v *Viewer) adjustGamma(wheelDelta float64) {
	gamma := v.view.Gamma
	if gamma <= 0 {
		gamma = defaultGamma
	}
	gamma = clampFloat64(gamma+wheelDelta*gammaStep, minGamma, maxGamma)
	v.view.Gamma = gamma
	v.targetView.Gamma = gamma
	v.rememberCurrentImageView()
	v.showCenterInfo("Gamma %.2f", gamma)
}

func (v *Viewer) adjustExposure(wheelDelta float64) {
	exposure := clampFloat64(v.view.Exposure+wheelDelta*0.1, minExposure, maxExposure)
	v.view.Exposure = exposure
	v.targetView.Exposure = exposure
	v.rememberCurrentImageView()
	v.showCenterInfo("Exposure %+.1f", exposure)
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
	v.showCenterInfo("Contrast %.1f", contrast)
}
