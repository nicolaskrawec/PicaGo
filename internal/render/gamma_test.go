package render

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestGammaShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(gammaShaderSource)); err != nil {
		t.Fatalf("gamma shader failed to compile: %v", err)
	}
}

// Run the shader on the graphics backend: compilation alone cannot catch
// undefined results from fractional powers of negative color channels.
func TestMain(m *testing.M) {
	game := &gammaTestGame{m: m}
	if err := ebiten.RunGame(game); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(game.code)
}

type gammaTestGame struct {
	m    *testing.M
	code int
}

func (g *gammaTestGame) Update() error {
	g.code = g.m.Run()
	return ebiten.Termination
}

func TestGammaShaderDarkRamp(t *testing.T) {
	// With contrast 2 and gamma 2, values below 0.25 must become black.
	// Mixed channels also verify that clipping preserves the other channels.
	samples := []color.RGBA{
		{0, 0, 0, 255}, {16, 16, 16, 255}, {32, 32, 32, 255},
		{63, 63, 63, 255}, {64, 64, 64, 255}, {96, 96, 96, 255},
		{32, 128, 64, 255},
	}
	want := []color.RGBA{
		{0, 0, 0, 255}, {0, 0, 0, 255}, {0, 0, 0, 255},
		{0, 0, 0, 255}, {11, 11, 11, 255}, {128, 128, 128, 255},
		{0, 181, 11, 255},
	}
	ramp := image.NewRGBA(image.Rect(0, 0, len(samples), 1))
	for x, c := range samples {
		ramp.SetRGBA(x, 0, c)
	}
	source := ebiten.NewImageFromImage(ramp)
	defer source.Deallocate()
	destination := ebiten.NewImage(len(samples), 1)
	defer destination.Deallocate()
	shader, uniforms := gammaDrawOptions(2, 0, 2)
	options := &ebiten.DrawRectShaderOptions{Uniforms: uniforms}
	options.Images[0] = source
	destination.DrawRectShader(len(samples), 1, shader, options)
	pixels := make([]byte, 4*len(samples))
	destination.ReadPixels(pixels)
	for x, expected := range want {
		for channel, value := range []byte{expected.R, expected.G, expected.B, expected.A} {
			got := pixels[4*x+channel]
			if delta := int(got) - int(value); delta < -1 || delta > 1 {
				t.Errorf("sample %d channel %d: got %d, want %d (tolerance 1)", x, channel, got, value)
			}
		}
	}
}

func (g *gammaTestGame) Draw(*ebiten.Image) {}

func (g *gammaTestGame) Layout(int, int) (int, int) { return 16, 16 }
