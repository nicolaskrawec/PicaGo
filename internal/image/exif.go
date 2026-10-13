package image

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bep/imagemeta"
)

const maxEXIFValueRunes = 160

// EXIFTag is a display-ready EXIF field attached to a decoded image.
type EXIFTag struct {
	Name  string
	Value string
}

func readEXIF(reader io.ReadSeeker, fileName string) []EXIFTag {
	format, ok := exifImageFormat(fileName)
	if !ok {
		return nil
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil
	}

	tags := make([]EXIFTag, 0, 32)
	_, err := imagemeta.Decode(imagemeta.Options{
		R:           reader,
		ImageFormat: format,
		Sources:     imagemeta.EXIF,
		ShouldHandleTag: func(tag imagemeta.TagInfo) bool {
			// Ignore embedded thumbnails; their tags duplicate the main image's
			// metadata and can contain large binary values.
			return !strings.HasPrefix(tag.Namespace, "IFD1")
		},
		HandleTag: func(tag imagemeta.TagInfo) error {
			tags = append(tags, EXIFTag{Name: tag.Tag, Value: exifValueText(tag.Value)})
			return nil
		},
		Timeout:      250 * time.Millisecond,
		LimitNumTags: 512,
		LimitTagSize: 4096,
	})
	_, _ = reader.Seek(0, io.SeekStart)
	if err != nil {
		return nil
	}

	sort.SliceStable(tags, func(i, j int) bool {
		return strings.ToLower(tags[i].Name) < strings.ToLower(tags[j].Name)
	})
	return tags
}

func exifImageFormat(fileName string) (imagemeta.ImageFormat, bool) {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".jpg", ".jpeg":
		return imagemeta.JPEG, true
	case ".png":
		return imagemeta.PNG, true
	case ".webp":
		return imagemeta.WebP, true
	case ".tif", ".tiff":
		return imagemeta.TIFF, true
	default:
		return imagemeta.ImageFormatAuto, false
	}
}

func exifValueText(value any) string {
	text := strings.Join(strings.Fields(fmt.Sprint(value)), " ")
	if utf8.RuneCountInString(text) <= maxEXIFValueRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxEXIFValueRunes-1]) + "…"
}

// ReadCaptureTime reads the original EXIF shooting date without decoding pixels.
// Missing, invalid, or unsupported metadata returns the zero time.
func ReadCaptureTime(reader io.Reader, fileName string) time.Time {
	format, ok := exifImageFormat(fileName)
	if !ok {
		return time.Time{}
	}
	seeker, ok := reader.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(io.LimitReader(reader, 16<<20))
		if err != nil {
			return time.Time{}
		}
		seeker = bytes.NewReader(data)
	}
	var captured time.Time
	_, err := imagemeta.Decode(imagemeta.Options{
		R:           seeker,
		ImageFormat: format,
		Sources:     imagemeta.EXIF,
		ShouldHandleTag: func(tag imagemeta.TagInfo) bool {
			return !strings.HasPrefix(tag.Namespace, "IFD1") && tag.Tag == "DateTimeOriginal"
		},
		HandleTag: func(tag imagemeta.TagInfo) error {
			captured, _ = time.Parse("2006:01:02 15:04:05", strings.TrimRight(fmt.Sprint(tag.Value), "\x00 "))
			return nil
		},
		Timeout:      250 * time.Millisecond,
		LimitNumTags: 512,
		LimitTagSize: 4096,
	})
	if err != nil {
		return time.Time{}
	}
	return captured
}
