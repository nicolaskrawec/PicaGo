package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"viewergo/internal/render"
)

const imageStatesFileName = "image-views.json"

func loadImageViewStates() map[string]render.View {
	states := make(map[string]render.View)
	path, err := imageStatesPath()
	if err != nil {
		return states
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]render.View)
	}
	var stored map[string]render.View
	if json.Unmarshal(data, &stored) != nil {
		return make(map[string]render.View)
	}
	for key, state := range stored {
		// Deliberately ignore old formats. Only current hashed keys are
		// accepted, so an old file is not silently interpreted differently.
		if isImageViewStateKey(key) {
			states[key] = state
		}
	}
	return states
}

func isImageViewStateKey(key string) bool {
	if !strings.HasPrefix(key, "fnv1a64:") || len(key) != len("fnv1a64:")+16 {
		return false
	}
	_, err := strconv.ParseUint(key[len("fnv1a64:"):], 16, 64)
	return err == nil
}

func saveImageViewStates(states map[string]render.View) error {
	path, err := imageStatesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(states)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func imageStatesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appConfigDirectory, imageStatesFileName), nil
}
