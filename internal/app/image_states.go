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

type persistedImageView struct {
	Alpha          float64
	FlipHorizontal bool
	FlipVertical   bool
	Rotation       int
	RotationAngle  float64
	MirrorScaleX   float64
	MirrorScaleY   float64
	Gamma          float64
	Exposure       float64
	Contrast       float64
}

func newPersistedImageView(view render.View) persistedImageView {
	return persistedImageView{
		Alpha:          view.Alpha,
		FlipHorizontal: view.FlipHorizontal,
		FlipVertical:   view.FlipVertical,
		Rotation:       view.Rotation,
		RotationAngle:  view.RotationAngle,
		MirrorScaleX:   view.MirrorScaleX,
		MirrorScaleY:   view.MirrorScaleY,
		Gamma:          view.Gamma,
		Exposure:       view.Exposure,
		Contrast:       view.Contrast,
	}
}

func (view persistedImageView) renderView() render.View {
	return render.View{
		Alpha:          view.Alpha,
		FlipHorizontal: view.FlipHorizontal,
		FlipVertical:   view.FlipVertical,
		Rotation:       view.Rotation,
		RotationAngle:  view.RotationAngle,
		MirrorScaleX:   view.MirrorScaleX,
		MirrorScaleY:   view.MirrorScaleY,
		Gamma:          view.Gamma,
		Exposure:       view.Exposure,
		Contrast:       view.Contrast,
	}
}

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
	var stored map[string]persistedImageView
	if json.Unmarshal(data, &stored) != nil {
		return make(map[string]render.View)
	}
	for key, state := range stored {
		if isImageViewStateKey(key) {
			states[key] = state.renderView()
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
	stored := make(map[string]persistedImageView, len(states))
	for key, state := range states {
		stored[key] = newPersistedImageView(state)
	}
	data, err := json.Marshal(stored)
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
