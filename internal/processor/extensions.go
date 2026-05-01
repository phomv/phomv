package processor

import (
	"path/filepath"
	"strings"
)

// SupportedExtensions are the photo file extensions phomv recognizes.
var SupportedExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".cr2":  {},
	".heic": {},
	".png":  {},
	".tif":  {},
	".tiff": {},
	".nef":  {},
	".arw":  {},
	".dng":  {},
}

// IsSupported reports whether the file at path has a supported photo extension.
func IsSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := SupportedExtensions[ext]
	return ok
}
