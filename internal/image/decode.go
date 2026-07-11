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
	xdraw "golang.org/x/image/draw"
)

type LoadedImage struct {
	FileName        string
	FilePath        string
	Width           int
	Height          int
	HasTransparency bool
	GPUTexture      *ebiten.Image
}

type DecodedImage struct {
	FileName        string
	FilePath        string
	Width           int
	Height          int
	HasTransparency bool
	Image           stddraw.Image
	// Preview is deliberately kept separate from Image: it can be uploaded to
	// the GPU quickly while the full-resolution texture is deferred.
	Preview stddraw.Image
}

// Release drops the CPU-side image references immediately. The memory is
// then reclaimed by Go's garbage collector instead of waiting for the next
// collection cycle while the prefetch cache keeps changing.
func (decoded *DecodedImage) Release() {
	if decoded == nil {
		return
	}
	decoded.Image = nil
	decoded.Preview = nil
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

	return decodeFromReader(file, filepath.Base(path), path, 0)
}

func DecodeFileForDisplay(path string, maxDimension int) (*DecodedImage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodeFromReader(file, filepath.Base(path), path, maxDimension)
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

	return decodeFromReader(file, filepath.Base(path), path, 0)
}

func DecodeFSForDisplay(fsys fs.FS, path string, maxDimension int) (*DecodedImage, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodeFromReader(file, filepath.Base(path), path, maxDimension)
}

func LoadBytes(data []byte, fileName, filePath string) (*LoadedImage, error) {
	decoded, err := DecodeBytes(data, fileName, filePath)
	if err != nil {
		return nil, err
	}
	return NewLoadedImage(decoded), nil
}

func DecodeBytes(data []byte, fileName, filePath string) (*DecodedImage, error) {
	return decodeFromReader(bytes.NewReader(data), fileName, filePath, 0)
}

func decodeFromReader(reader io.Reader, fileName, filePath string, maxDimension int) (*DecodedImage, error) {
	decoded, _, err := stddraw.Decode(reader)
	if err != nil {
		return nil, err
	}

	bounds := decoded.Bounds()
	result := &DecodedImage{
		FileName:        fileName,
		FilePath:        filePath,
		Width:           bounds.Dx(),
		Height:          bounds.Dy(),
		HasTransparency: hasTransparency(decoded),
		Image:           decoded,
	}
	if maxDimension > 0 {
		result.Preview = makePreview(decoded, maxDimension)
	}
	return result, nil
}

func makePreview(source stddraw.Image, maxDimension int) stddraw.Image {
	bounds := source.Bounds()
	longest := bounds.Dx()
	if bounds.Dy() > longest {
		longest = bounds.Dy()
	}
	if longest <= maxDimension {
		return source
	}
	scale := float64(maxDimension) / float64(longest)
	width := max(1, int(float64(bounds.Dx())*scale))
	height := max(1, int(float64(bounds.Dy())*scale))
	preview := stddraw.NewRGBA(stddraw.Rect(0, 0, width, height))
	xdraw.ApproxBiLinear.Scale(preview, preview.Bounds(), source, bounds, xdraw.Src, nil)
	return preview
}

func hasTransparency(source stddraw.Image) bool {
	switch source.(type) {
	case *stddraw.Gray, *stddraw.Gray16, *stddraw.YCbCr:
		return false
	}
	bounds := source.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := source.At(x, y).RGBA()
			if alpha < 0xffff {
				return true
			}
		}
	}
	return false
}

func NewLoadedImage(decoded *DecodedImage) *LoadedImage {
	if decoded == nil {
		return nil
	}

	return &LoadedImage{
		FileName:        decoded.FileName,
		FilePath:        decoded.FilePath,
		Width:           decoded.Width,
		Height:          decoded.Height,
		HasTransparency: decoded.HasTransparency,
		GPUTexture:      ebiten.NewImageFromImage(decoded.Image),
	}
}

func NewLoadedPreviewImage(decoded *DecodedImage) *LoadedImage {
	if decoded == nil {
		return nil
	}
	textureSource := decoded.Preview
	if textureSource == nil {
		textureSource = decoded.Image
	}
	return &LoadedImage{
		FileName: decoded.FileName, FilePath: decoded.FilePath,
		Width: decoded.Width, Height: decoded.Height,
		HasTransparency: decoded.HasTransparency,
		GPUTexture:      ebiten.NewImageFromImage(textureSource),
	}
}

func (loaded *LoadedImage) Release() {
	if loaded == nil || loaded.GPUTexture == nil {
		return
	}
	loaded.GPUTexture.Deallocate()
	loaded.GPUTexture = nil
}

func UpgradeLoadedImage(loaded *LoadedImage, decoded *DecodedImage) {
	if loaded == nil || decoded == nil || decoded.Image == nil {
		return
	}
	texture := ebiten.NewImageFromImage(decoded.Image)
	if loaded.GPUTexture != nil {
		loaded.GPUTexture.Deallocate()
	}
	loaded.GPUTexture = texture
}
