package processor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mvhd builds a movie header box whose creation and modification times are
// secs since 1904.
func mvhd(version byte, secs uint64) []byte {
	if version == 1 {
		return box("mvhd", fullBoxHeader(1), be64(secs), be64(secs), be32(1000), be64(0), make([]byte, 80))
	}
	return box("mvhd", fullBoxHeader(0), be32(uint32(secs)), be32(uint32(secs)), be32(1000), be32(0), make([]byte, 80))
}

func be64(v uint64) []byte { return append(be32(uint32(v>>32)), be32(uint32(v))...) }

func secsSince1904(t time.Time) uint64 {
	return uint64(t.Sub(time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)) / time.Second)
}

func TestExtractTimeVideo(t *testing.T) {
	created := time.Date(2022, 6, 30, 18, 45, 12, 0, time.UTC)
	mtime := time.Date(2020, 2, 3, 12, 0, 0, 0, time.Local)
	secs := secsSince1904(created)
	ftyp := box("ftyp", []byte("isom"), be32(0), []byte("isommp41"))
	mdat := box("mdat", []byte("frames"))
	trak := box("trak", box("tkhd", fullBoxHeader(0), make([]byte, 80)))

	cases := []struct {
		name       string
		file       string
		data       []byte
		wantSource TimeSource
		want       time.Time
	}{
		{"mp4 mvhd v0", "clip.mp4", bytes.Join([][]byte{ftyp, box("moov", mvhd(0, secs), trak), mdat}, nil), SourceQuickTime, created},
		{"mp4 mvhd v1", "clip.m4v", bytes.Join([][]byte{ftyp, box("moov", mvhd(1, secs)), mdat}, nil), SourceQuickTime, created},
		{"mdat before moov", "IMG_0001.MOV", bytes.Join([][]byte{ftyp, mdat, box("moov", trak, mvhd(0, secs))}, nil), SourceQuickTime, created},
		{"classic mov, no ftyp", "old.mov", bytes.Join([][]byte{box("wide"), mdat, box("moov", mvhd(0, secs))}, nil), SourceQuickTime, created},
		{"zero creation time", "clip.mp4", bytes.Join([][]byte{ftyp, box("moov", mvhd(0, 0)), mdat}, nil), SourceMTime, mtime},
		{"no moov", "clip.mp4", bytes.Join([][]byte{ftyp, mdat}, nil), SourceMTime, mtime},
		{"moov without mvhd", "clip.3gp", bytes.Join([][]byte{ftyp, box("moov", trak), box("mvhd", mvhd(0, secs)[8:])}, nil), SourceMTime, mtime},
		{"not a container", "clip.mp4", []byte("definitely not a movie"), SourceMTime, mtime},
		{"empty", "clip.mov", nil, SourceMTime, mtime},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
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
			if got.When.Location() != time.Local {
				t.Errorf("when is in %v, want local time", got.When.Location())
			}
		})
	}
}

func TestQuickTimeCreationTruncatedDoesNotPanic(t *testing.T) {
	full := bytes.Join([][]byte{
		box("ftyp", []byte("isom")),
		box("moov", mvhd(1, 3_000_000_000)),
	}, nil)
	for n := 0; n < len(full); n++ {
		quickTimeCreation(bytes.NewReader(full[:n]))
	}
}
