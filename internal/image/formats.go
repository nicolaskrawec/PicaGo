package image

import "path/filepath"

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
	_, ok := supportedExtensions[filepath.Ext(path)]
	return ok
}
