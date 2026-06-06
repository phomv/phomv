package processor

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDestinationFor(t *testing.T) {
	when := time.Date(2024, 7, 4, 12, 0, 0, 0, time.UTC)
	got := DestinationFor("/dest", "/src/IMG_1234.jpg", when)
	want := filepath.Join("/dest", "2024", "2024_07", "2024_07_04", "IMG_1234.jpg")
	if got != want {
		t.Fatalf("DestinationFor = %q, want %q", got, want)
	}
}

func TestUnknownDestination(t *testing.T) {
	got := UnknownDestination("/dest", "/src/foo.heic")
	want := filepath.Join("/dest", "Unknown", "foo.heic")
	if got != want {
		t.Fatalf("UnknownDestination = %q, want %q", got, want)
	}
}

func TestIsSupported(t *testing.T) {
	cases := map[string]bool{
		"a.jpg":        true,
		"a.JPG":        true,
		"b.jpeg":       true,
		"c.HEIC":       true,
		"d.cr2":        true,
		"notes.txt":    false,
		"video.mp4":    false,
		"no-extension": false,
	}
	for name, want := range cases {
		if got := IsSupported(name); got != want {
			t.Errorf("IsSupported(%q) = %v, want %v", name, got, want)
		}
	}
}
