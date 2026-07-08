package image

import (
	"path/filepath"
	"strings"
)

var supportedExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".gif":  {},
	".webp": {},
	".bmp":  {},
	".tif":  {},
	".tiff": {},
}

func IsSupportedFile(path string) bool {
	_, ok := supportedExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}
