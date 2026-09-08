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
	v.savePreferences()
	v.showCenterInfo("notification.background", v.backgroundName())
}

func backgroundModeFromConfig(value string) backgroundMode {
	switch value {
	case "gray":
		return backgroundGray
	case "black":
		return backgroundBlack
	case "white":
		return backgroundWhite
	default:
		return backgroundDesktop
	}
}

func (v *Viewer) savePreferences() {
	v.preferences.ShowShadow = v.showShadow
	switch v.background {
	case backgroundGray:
		v.preferences.Background = "gray"
	case backgroundBlack:
		v.preferences.Background = "black"
	case backgroundWhite:
		v.preferences.Background = "white"
	default:
		v.preferences.Background = "desktop"
	}
	_ = saveConfig(v.preferences)
}

func (v *Viewer) backgroundName() string {
	if v.usesDesktopBackground() {
		return v.text("background.desktop")
	}
	switch v.background {
	case backgroundBlack:
		return v.text("background.black")
	case backgroundWhite:
		return v.text("background.white")
	default:
		return v.text("background.gray")
	}
}
