package app

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestGraphicsLibraryFromConfig(t *testing.T) {
	tests := []struct {
		value string
		want  ebiten.GraphicsLibrary
	}{
		{value: "auto", want: ebiten.GraphicsLibraryAuto},
		{value: "", want: ebiten.GraphicsLibraryAuto},
		{value: "unknown", want: ebiten.GraphicsLibraryAuto},
		{value: " OpenGL ", want: ebiten.GraphicsLibraryOpenGL},
		{value: "DIRECTX", want: ebiten.GraphicsLibraryDirectX},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := graphicsLibraryFromConfig(test.value); got != test.want {
				t.Fatalf("graphicsLibraryFromConfig(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestDefaultConfigUsesAutomaticGraphicsLibrary(t *testing.T) {
	if got := defaultConfig().GraphicsLibrary; got != "auto" {
		t.Fatalf("default graphics library = %q, want auto", got)
	}
}

func TestCompactHelpShowsActiveRenderer(t *testing.T) {
	if text := (&Viewer{}).helpTextCompact(); !strings.Contains(text, "Renderer: ") {
		t.Fatalf("compact help does not show renderer: %q", text)
	}
}

func TestCompactHelpShowsFolderImageOrderShortcut(t *testing.T) {
	if text := (&Viewer{}).helpTextCompact(); !strings.Contains(text, "O: image order (current: name ascending)") {
		t.Fatalf("compact help does not show the image order shortcut: %q", text)
	}
}
