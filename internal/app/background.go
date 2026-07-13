package app

func (v *Viewer) usesDesktopBackground() bool {
	return v.borderlessMaximized && v.background == backgroundDesktop && v.desktopBackdrop != nil
}

func (v *Viewer) canSelectDesktopBackground() bool {
	return v.borderlessMaximized && v.desktopBackground && v.desktopBackdrop != nil
}

func (v *Viewer) toggleBackground() {
	if v.canSelectDesktopBackground() {
		switch v.background {
		case backgroundDesktop:
			v.background = backgroundGray
		case backgroundGray:
			v.background = backgroundBlack
		case backgroundBlack:
			v.background = backgroundWhite
		default:
			v.background = backgroundDesktop
		}
	} else {
		switch v.background {
		case backgroundGray, backgroundDesktop:
			v.background = backgroundBlack
		case backgroundBlack:
			v.background = backgroundWhite
		default:
			v.background = backgroundGray
		}
	}
	v.showCenterInfo("Fond : %s", v.backgroundName())
}

func (v *Viewer) backgroundName() string {
	if v.usesDesktopBackground() {
		return "bureau"
	}
	switch v.background {
	case backgroundBlack:
		return "noir"
	case backgroundWhite:
		return "blanc"
	default:
		return "gris"
	}
}
