package processor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/phomv/phomv/internal/testutil"
)

func TestExtractTime(t *testing.T) {
	mtime := time.Date(2020, 2, 3, 12, 0, 0, 0, time.Local)
	cases := []struct {
		name       string
		data       []byte
		wantSource TimeSource
		want       time.Time
	}{
		{
			name:       "jpeg with DateTimeOriginal",
			data:       testutil.JPEGWithDateTimeOriginal("2019:08:17 14:30:05"),
			wantSource: SourceEXIF,
			want:       time.Date(2019, 8, 17, 14, 30, 5, 0, time.Local),
		},
		{
			name:       "jpeg without exif",
			data:       testutil.JPEGWithoutEXIF(),
			wantSource: SourceMTime,
			want:       mtime,
		},
		{
			name:       "corrupt file",
			data:       []byte("\xff\xd8\xff\xe1\x00\x10Exif\x00\x00garbage"),
			wantSource: SourceMTime,
			want:       mtime,
		},
		{
			name:       "empty file",
			data:       nil,
			wantSource: SourceMTime,
			want:       mtime,
		},
		{
			name:       "zero exif date",
			data:       testutil.JPEGWithDateTimeOriginal("0000:00:00 00:00:00"),
			wantSource: SourceMTime,
			want:       mtime,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "IMG_0001.jpg")
			if err := os.WriteFile(path, tc.data, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, mtime, mtime); err != nil {
				t.Fatal(err)
			}
			got, err := ExtractTime(path)
			if err != nil {
				t.Fatal(err)
			}
			if got.Source != tc.wantSource {
				t.Errorf("source = %v, want %v", got.Source, tc.wantSource)
			}
			if !got.When.Equal(tc.want) {
				t.Errorf("when = %v, want %v", got.When, tc.want)
			}
		})
	}
}

func TestExtractTimeMissingFile(t *testing.T) {
	_, err := ExtractTime(filepath.Join(t.TempDir(), "gone.jpg"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}

func TestTimeSourceString(t *testing.T) {
	for src, want := range map[TimeSource]string{
		SourceEXIF: "exif", SourceMTime: "mtime", SourceQuickTime: "quicktime", SourceUnknown: "unknown",
	} {
		if got := src.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", src, got, want)
		}
	}
}
