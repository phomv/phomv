package processor

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// ErrNoTimestamp signals that neither EXIF data nor mtime yielded a usable timestamp.
var ErrNoTimestamp = errors.New("no timestamp available")

// TimeSource describes where a file's organizing timestamp came from.
type TimeSource int

const (
	SourceUnknown TimeSource = iota
	SourceEXIF
	SourceMTime
)

func (s TimeSource) String() string {
	switch s {
	case SourceEXIF:
		return "exif"
	case SourceMTime:
		return "mtime"
	default:
		return "unknown"
	}
}

// PhotoTime holds the resolved timestamp and where it came from.
type PhotoTime struct {
	When   time.Time
	Source TimeSource
}

// ExtractTime returns the best available timestamp for a file.
// Priority: EXIF DateTimeOriginal -> file mtime.
func ExtractTime(path string) (PhotoTime, error) {
	if t, err := readEXIFTime(path); err == nil {
		return PhotoTime{When: t, Source: SourceEXIF}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return PhotoTime{}, fmt.Errorf("stat %s: %w", path, err)
	}
	mt := info.ModTime()
	if mt.IsZero() {
		return PhotoTime{}, ErrNoTimestamp
	}
	return PhotoTime{When: mt, Source: SourceMTime}, nil
}

func readEXIFTime(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}, err
	}
	t, err := x.DateTime()
	if err != nil {
		return time.Time{}, err
	}
	if t.IsZero() {
		return time.Time{}, errors.New("zero exif time")
	}
	return t, nil
}
