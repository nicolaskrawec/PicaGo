package image

import (
	"bytes"
	stddraw "image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/hajimehoshi/ebiten/v2"
)

type LoadedImage struct {
	FileName   string
	FilePath   string
	Width      int
	Height     int
	Decoded    stddraw.Image
	GPUTexture *ebiten.Image
}

func LoadFile(path string) (*LoadedImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadBytes(data, filepath.Base(path), path)
}

func LoadFS(fsys fs.FS, path string) (*LoadedImage, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return LoadBytes(data, filepath.Base(path), path)
}

func LoadBytes(data []byte, fileName, filePath string) (*LoadedImage, error) {
	decoded, _, err := stddraw.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := decoded.Bounds()
	texture := ebiten.NewImageFromImage(decoded)

	return &LoadedImage{
		FileName:   fileName,
		FilePath:   filePath,
		Width:      bounds.Dx(),
		Height:     bounds.Dy(),
		Decoded:    decoded,
		GPUTexture: texture,
	}, nil
}
