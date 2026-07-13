package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const appConfigDirectory = "PicaGo"

type config struct {
	DesktopBackground bool   `json:"desktopBackground"`
	Background        string `json:"background"`
	ShowShadow        bool   `json:"showShadow"`
	AnimateOnStart    bool   `json:"animateOnStart"`
	ShowDebug         bool   `json:"showDebug"`
}

func defaultConfig() config {
	return config{
		DesktopBackground: true,
		Background:        "desktop",
		ShowShadow:        true,
		AnimateOnStart:    true,
		ShowDebug:         false,
	}
}

func loadConfig() config {
	defaults := defaultConfig()
	path, err := userConfigPath()
	if err == nil {
		if cfg, found := readConfig(path, defaults); found {
			return cfg
		}
	}

	// Keep existing installations working: use the old configuration beside
	// the executable once, then migrate it to the per-user configuration.
	if legacyPath, legacyErr := legacyConfigPath(); legacyErr == nil {
		if cfg, found := readConfig(legacyPath, defaults); found {
			_ = saveConfig(cfg)
			return cfg
		}
	}

	_ = saveConfig(defaults)
	return defaults
}

func readConfig(path string, defaults config) (config, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return defaults, false
	}
	cfg := defaults
	if json.Unmarshal(data, &cfg) != nil {
		return defaults, false
	}
	return cfg, true
}

func saveConfig(cfg config) error {
	path, err := userConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func userConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appConfigDirectory, "config.json"), nil
}

func legacyConfigPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(executable), "PicaGo.json"), nil
}
