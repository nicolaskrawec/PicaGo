package app

const (
	defaultGamma = 1.0
	gammaStep    = 0.1
	minGamma     = 0.1
	maxGamma     = 4.0
)

func (v *Viewer) adjustGamma(wheelDelta float64) {
	gamma := v.view.Gamma
	if gamma <= 0 {
		gamma = defaultGamma
	}
	gamma = clampFloat64(gamma+wheelDelta*gammaStep, minGamma, maxGamma)
	v.view.Gamma = gamma
	v.targetView.Gamma = gamma
}
