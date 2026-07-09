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
	GPUTexture *ebiten.Image
}

type DecodedImage struct {
	FileName string
	FilePath string
	Width    int
	Height   int
	Image    stddraw.Image
}

func LoadFile(path string) (*LoadedImage, error) {
	decoded, err := DecodeFile(path)
	if err != nil {
		return nil, err
	}
	return NewLoadedImage(decoded), nil
}

func DecodeFile(path string) (*DecodedImage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return decodeFromReader(file, filepath.Base(path), path)
}

func LoadFS(fsys fs.FS, path string) (*LoadedImage, error) {
	decoded, err := DecodeFS(fsys, path)
	if err != nil {
		return nil, err
	}
	return NewLoadedImage(decoded), nil
}

func DecodeFS(fsys fs.FS, path string) (*DecodedImage, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return decodeFromReader(file, filepath.Base(path), path)
}

func LoadBytes(data []byte, fileName, filePath string) (*LoadedImage, error) {
	decoded, err := DecodeBytes(data, fileName, filePath)
	if err != nil {
		return nil, err
	}
	return NewLoadedImage(decoded), nil
}

func DecodeBytes(data []byte, fileName, filePath string) (*DecodedImage, error) {
	return decodeFromReader(bytes.NewReader(data), fileName, filePath)
}

func decodeFromReader(reader io.Reader, fileName, filePath string) (*DecodedImage, error) {
	decoded, _, err := stddraw.Decode(reader)
	if err != nil {
		return nil, err
	}

	bounds := decoded.Bounds()
	return &DecodedImage{
		FileName: fileName,
		FilePath: filePath,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Image:    decoded,
	}, nil
}

func NewLoadedImage(decoded *DecodedImage) *LoadedImage {
	if decoded == nil {
		return nil
	}

	return &LoadedImage{
		FileName:   decoded.FileName,
		FilePath:   decoded.FilePath,
		Width:      decoded.Width,
		Height:     decoded.Height,
		GPUTexture: ebiten.NewImageFromImage(decoded.Image),
	}
}
