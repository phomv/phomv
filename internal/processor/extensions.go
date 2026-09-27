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

// VideoExtensions are the video file extensions phomv recognizes; all are
// ISO-BMFF (MP4/QuickTime) containers.
var VideoExtensions = map[string]struct{}{
	".mp4": {},
	".mov": {},
	".m4v": {},
	".3gp": {},
}

// IsSupported reports whether the file at path has a supported photo or
// video extension.
func IsSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := SupportedExtensions[ext]
	return ok || IsVideo(path)
}

// IsVideo reports whether the file at path has a supported video extension.
func IsVideo(path string) bool {
	_, ok := VideoExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}
