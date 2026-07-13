package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type config struct {
	DesktopBackground bool `json:"desktopBackground"`
	ShowShadow        bool `json:"showShadow"`
	AnimateOnStart    bool `json:"animateOnStart"`
}

func defaultConfig() config {
	return config{
		DesktopBackground: true,
		ShowShadow:        true,
		AnimateOnStart:    true,
	}
}

func loadConfig() config {
	cfg := defaultConfig()

	executable, err := os.Executable()
	if err != nil {
		return cfg
	}
	path := filepath.Join(filepath.Dir(executable), "PicaGo.json")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if data, marshalErr := json.MarshalIndent(cfg, "", "  "); marshalErr == nil {
			_ = os.WriteFile(path, append(data, '\n'), 0644)
		}
		return cfg
	}
	if err != nil {
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig()
	}
	return cfg
}
