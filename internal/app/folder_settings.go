package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const folderSettingsFileName = "folder-settings.json"

func loadFolderSortModes() map[string]imageSortMode {
	modes := make(map[string]imageSortMode)
	path, err := folderSettingsPath()
	if err != nil {
		return modes
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return modes
	}
	var stored map[string]string
	if json.Unmarshal(data, &stored) != nil {
		return modes
	}
	for key, value := range stored {
		mode := imageSortModeFromConfig(value)
		if isFolderSettingsKey(key) && mode != imageSortByNameAscending {
			modes[key] = mode
		}
	}
	return modes
}

func saveFolderSortModes(modes map[string]imageSortMode) error {
	path, err := folderSettingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	stored := make(map[string]string, len(modes))
	for key, mode := range modes {
		if isFolderSettingsKey(key) && mode != imageSortByNameAscending {
			stored[key] = mode.configValue()
		}
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func folderSettingsKey(dir string) string {
	normalized, err := filepath.Abs(dir)
	if err != nil {
		normalized = filepath.Clean(dir)
	}
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	normalized = filepath.ToSlash(normalized)
	sum := sha256.Sum256([]byte(normalized))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func isFolderSettingsKey(key string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(key, prefix) || len(key) != len(prefix)+sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(key[len(prefix):])
	return err == nil
}

func folderSettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appConfigDirectory, folderSettingsFileName), nil
}
