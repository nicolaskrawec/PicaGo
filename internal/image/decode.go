package image

import (
	"bytes"
	"fmt"
	stddraw "image"
	_ "image/gif"
	stdjpeg "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/gen2brain/jpegn"
	"github.com/hajimehoshi/ebiten/v2"
	xdraw "golang.org/x/image/draw"
)

const (
	maxImageFileBytes = 100 * 1024 * 1024
	maxImagePixels    = int64(100_000_000)
)

type LoadedImage struct {
	FileName        string
	FilePath        string
	DecodeMethod    string
	Width           int
	Height          int
	HasTransparency bool
	GPUTexture      *ebiten.Image
}

type DecodedImage struct {
	FileName        string
	FilePath        string
	DecodeMethod    string
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
	if err := validateImageFile(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return decodeFromReader(file, filepath.Base(path), path, 0)
}

func DecodeFileForDisplay(path string, maxDimension int) (*DecodedImage, error) {
	if err := validateImageFile(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodePreviewFromReader(file, filepath.Base(path), path, maxDimension)
}

// DecodeFileThumbnail decodes a file and retains only a small, display-ready
// image. The full-size decode is released before returning so thumbnail caches
// never keep the original pixels alive.
func DecodeFileThumbnail(path string, maxDimension int) (stddraw.Image, error) {
	if err := validateImageFile(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoded, err := decodeFromReader(file, filepath.Base(path), path, maxDimension)
	if err != nil {
		return nil, err
	}
	thumbnail := decoded.Preview
	if thumbnail == nil {
		thumbnail = decoded.Image
	}
	decoded.Image = nil
	decoded.Preview = nil
	return thumbnail, nil
}

func LoadFS(fsys fs.FS, path string) (*LoadedImage, error) {
	decoded, err := DecodeFS(fsys, path)
	if err != nil {
		return nil, err
	}
	return NewLoadedImage(decoded), nil
}

func DecodeFS(fsys fs.FS, path string) (*DecodedImage, error) {
	if err := validateImageFS(fsys, path); err != nil {
		return nil, err
	}
	file, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return decodeFromReader(file, filepath.Base(path), path, 0)
}

func DecodeFSForDisplay(fsys fs.FS, path string, maxDimension int) (*DecodedImage, error) {
	if err := validateImageFS(fsys, path); err != nil {
		return nil, err
	}
	file, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodePreviewFromReader(file, filepath.Base(path), path, maxDimension)
}

func LoadBytes(data []byte, fileName, filePath string) (*LoadedImage, error) {
	decoded, err := DecodeBytes(data, fileName, filePath)
	if err != nil {
		return nil, err
	}
	return NewLoadedImage(decoded), nil
}

func DecodeBytes(data []byte, fileName, filePath string) (*DecodedImage, error) {
	if len(data) > maxImageFileBytes {
		return nil, fmt.Errorf("image file too large")
	}
	if err := validateImageConfig(bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return decodeFromReader(bytes.NewReader(data), fileName, filePath, 0)
}

func validateImageFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > maxImageFileBytes {
		return fmt.Errorf("image file too large")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return validateImageConfig(file)
}

func validateImageFS(fsys fs.FS, path string) error {
	info, err := fs.Stat(fsys, path)
	if err != nil {
		return err
	}
	if info.Size() > maxImageFileBytes {
		return fmt.Errorf("image file too large")
	}
	file, err := fsys.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return validateImageConfig(file)
}

func validateImageConfig(reader io.Reader) error {
	config, _, err := stddraw.DecodeConfig(reader)
	if err != nil {
		return err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > maxImagePixels/int64(config.Height) {
		return fmt.Errorf("image has too many pixels")
	}
	return nil
}

func decodeFromReader(reader io.Reader, fileName, filePath string, maxDimension int) (*DecodedImage, error) {
	if isJPEGFile(fileName) {
		return decodeJPEG(reader, fileName, filePath, maxDimension)
	}

	decoded, _, err := stddraw.Decode(reader)
	if err != nil {
		return nil, err
	}

	bounds := decoded.Bounds()
	result := &DecodedImage{
		FileName:     fileName,
		FilePath:     filePath,
		DecodeMethod: "image.Decode",
		Width:        bounds.Dx(),
		Height:       bounds.Dy(),
		// Alpha detection requires a full pixel-by-pixel pass. Keep it disabled
		// so decoding large images does not trigger a second full image scan.
		HasTransparency: false,
		Image:           decoded,
	}
	if maxDimension > 0 {
		result.Preview = makePreview(decoded, maxDimension)
	}
	return result, nil
}

func isJPEGFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return ext == ".jpg" || ext == ".jpeg"
}

// decodeJPEG uses JPEG's native IDCT scaling for display-sized images. This
// avoids allocating a full-resolution pixel buffer before resizing it. jpegn
// is pure Go (and uses SIMD where available); the standard decoder remains a
// compatibility fallback for JPEG variants it does not support.
func decodeJPEG(reader io.Reader, fileName, filePath string, maxDimension int) (*DecodedImage, error) {
	// A full-resolution load does not need a preliminary header pass. Decode
	// the source once and obtain dimensions from the resulting image.
	if maxDimension <= 0 {
		decoded, err := jpegn.Decode(reader, &jpegn.Options{AutoRotate: true})
		decodeMethod := "jpegn"
		if err != nil {
			// Retry only when the source can be rewound; this preserves the
			// compatibility fallback without buffering the compressed file.
			if seeker, ok := reader.(io.Seeker); ok {
				if _, seekErr := seeker.Seek(0, io.SeekStart); seekErr == nil {
					decoded, err = stdjpeg.Decode(reader)
					decodeMethod = "image/jpeg"
				}
			}
			if err != nil {
				return nil, err
			}
		}
		result := decodedImageResult(decoded, fileName, filePath)
		result.DecodeMethod = decodeMethod
		return result, nil
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	config, err := jpegn.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	orientation := jpegEXIFOrientation(data)
	decodeOptions := &jpegn.Options{AutoRotate: true}
	if maxDimension > 0 {
		longest := max(config.Width, config.Height)
		denom := jpegScaleDenom(longest, maxDimension)
		if denom > 1 {
			decodeOptions.ScaleDenom = denom
		}
	}

	decoded, err := jpegn.Decode(bytes.NewReader(data), decodeOptions)
	decodeMethod := "jpegn"
	if err != nil {
		// Keep compatibility with the standard library for uncommon JPEG
		// variants rejected by jpegn (for example arithmetic-coded files).
		decoded, err = stdjpeg.Decode(bytes.NewReader(data))
		decodeMethod = "image/jpeg"
		if err != nil {
			return nil, err
		}
	}

	metadataWidth, metadataHeight := orientedJPEGDimensions(config.Width, config.Height, orientation)
	result := &DecodedImage{
		FileName: fileName, FilePath: filePath,
		DecodeMethod: decodeMethod,
		// AutoRotate can swap the dimensions for EXIF orientations 5-8.
		Width: metadataWidth, Height: metadataHeight,
		HasTransparency: false, Image: decoded,
	}
	result.Preview = makePreview(decoded, maxDimension)
	return result, nil
}

func jpegEXIFOrientation(data []byte) int {
	exif, err := jpegn.DecodeExif(bytes.NewReader(data))
	if err != nil || exif.Orientation < 1 || exif.Orientation > 8 {
		return 1
	}
	return exif.Orientation
}

func orientedJPEGDimensions(width, height, orientation int) (int, int) {
	if orientation >= 5 && orientation <= 8 {
		return height, width
	}
	return width, height
}

func decodedImageResult(decoded stddraw.Image, fileName, filePath string) *DecodedImage {
	bounds := decoded.Bounds()
	return &DecodedImage{
		FileName: fileName, FilePath: filePath,
		Width: bounds.Dx(), Height: bounds.Dy(),
		HasTransparency: false, Image: decoded,
	}
}

func jpegScaleDenom(longest, maxDimension int) int {
	denom := 1
	for _, candidate := range []int{2, 4, 8} {
		if longest/candidate >= maxDimension {
			denom = candidate
		}
	}
	return denom
}

// decodePreviewFromReader keeps only the screen-sized result. Decoding most
// formats still requires a temporary full-resolution image, but that buffer is
// no longer retained by navigation caches or while a preview is displayed.
func decodePreviewFromReader(reader io.Reader, fileName, filePath string, maxDimension int) (*DecodedImage, error) {
	decoded, err := decodeFromReader(reader, fileName, filePath, maxDimension)
	if err != nil {
		return nil, err
	}
	if decoded.Preview == nil {
		decoded.Preview = decoded.Image
	}
	decoded.Image = nil
	return decoded, nil
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

func NewLoadedImage(decoded *DecodedImage) *LoadedImage {
	if decoded == nil {
		return nil
	}

	return &LoadedImage{
		FileName:        decoded.FileName,
		FilePath:        decoded.FilePath,
		DecodeMethod:    decoded.DecodeMethod,
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
		DecodeMethod: decoded.DecodeMethod,
		Width:        decoded.Width, Height: decoded.Height,
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

func (loaded *LoadedImage) IsFullResolution() bool {
	if loaded == nil || loaded.GPUTexture == nil {
		return false
	}
	bounds := loaded.GPUTexture.Bounds()
	return bounds.Dx() == loaded.Width && bounds.Dy() == loaded.Height
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

// NewLoadedPreviewCopy creates a GPU-only, screen-sized copy of an already
// loaded texture. It is used when the current full-resolution image becomes a
// navigation neighbour, avoiding a second CPU decode just to keep the previous
// image immediately available.
func NewLoadedPreviewCopy(source *LoadedImage, maxDimension int) *LoadedImage {
	if source == nil || source.GPUTexture == nil || maxDimension <= 0 {
		return nil
	}
	bounds := source.GPUTexture.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	longest := max(width, height)
	if longest <= 0 {
		return nil
	}
	scale := min(1, float64(maxDimension)/float64(longest))
	previewWidth := max(1, int(float64(width)*scale))
	previewHeight := max(1, int(float64(height)*scale))
	texture := ebiten.NewImage(previewWidth, previewHeight)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Scale(float64(previewWidth)/float64(width), float64(previewHeight)/float64(height))
	texture.DrawImage(source.GPUTexture, options)
	return &LoadedImage{
		FileName:        source.FileName,
		FilePath:        source.FilePath,
		DecodeMethod:    source.DecodeMethod,
		Width:           source.Width,
		Height:          source.Height,
		HasTransparency: source.HasTransparency,
		GPUTexture:      texture,
	}
}
