package processor

import (
	"path/filepath"
	"time"
)

// UnknownDir is the bucket used when timestamp resolution fails.
const UnknownDir = "Unknown"

// DestinationFor builds the final destination path for a file given its
// timestamp. The structure is dest/YYYY/YYYY_MM/YYYY_MM_DD/filename.ext.
func DestinationFor(dest, srcPath string, when time.Time) string {
	base := filepath.Base(srcPath)
	year := when.Format("2006")
	yearMonth := when.Format("2006_01")
	yearMonthDay := when.Format("2006_01_02")
	return filepath.Join(dest, year, yearMonth, yearMonthDay, base)
}

// UnknownDestination builds a destination path when no timestamp is available.
func UnknownDestination(dest, srcPath string) string {
	base := filepath.Base(srcPath)
	return filepath.Join(dest, UnknownDir, base)
}
