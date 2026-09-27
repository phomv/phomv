package processor

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/phomv/phomv/internal/testutil"
)

func box(typ string, body ...[]byte) []byte {
	payload := bytes.Join(body, nil)
	out := make([]byte, 8, 8+len(payload))
	binary.BigEndian.PutUint32(out, uint32(8+len(payload)))
	copy(out[4:], typ)
	return append(out, payload...)
}

func be16(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

// fullBoxHeader is the version + flags word of an ISO-BMFF FullBox.
func fullBoxHeader(version byte) []byte { return []byte{version, 0, 0, 0} }

// buildHEIC assembles ftyp + meta(hdlr, iinf, iloc) + mdat, with the Exif
// item stored in mdat. ilocVersion selects the iloc field widths.
func buildHEIC(t *testing.T, exifItem []byte, ilocVersion byte) []byte {
	t.Helper()
	ftyp := box("ftyp", []byte("heic"), be32(0), []byte("mif1heic"))
	hdlr := box("hdlr", fullBoxHeader(0), be32(0), []byte("pict"), make([]byte, 12), []byte{0})
	// A v0 infe (no item_type) precedes the Exif entry, as a v2 hvc1 entry would.
	infe0 := box("infe", fullBoxHeader(0), be16(7), be16(0), []byte("x\x00"))
	infe := box("infe", fullBoxHeader(2), be16(1), be16(0), []byte("Exif"), []byte{0})
	iinf := box("iinf", fullBoxHeader(0), be16(2), infe0, infe)

	iloc := func(offset uint32) []byte {
		body := [][]byte{fullBoxHeader(ilocVersion), {0x44, 0x00}} // offset/length 4 bytes, no base/index
		if ilocVersion < 2 {
			body = append(body, be16(1), be16(1))
		} else {
			body = append(body, be32(1), be32(1))
		}
		if ilocVersion == 1 || ilocVersion == 2 {
			body = append(body, be16(0)) // construction_method 0 (file offset)
		}
		body = append(body, be16(0), be16(1), be32(offset), be32(uint32(len(exifItem))))
		return box("iloc", body...)
	}

	// The iloc size doesn't depend on the offset value, so size it once.
	metaLen := len(box("meta", fullBoxHeader(0), hdlr, iinf, iloc(0)))
	offset := uint32(len(ftyp) + metaLen + 8)
	meta := box("meta", fullBoxHeader(0), hdlr, iinf, iloc(offset))
	return bytes.Join([][]byte{ftyp, meta, box("mdat", exifItem)}, nil)
}

func TestExtractTimeHEIC(t *testing.T) {
	const ts = "2023:04:05 06:07:08"
	tiff := testutil.TIFFWithDateTimeOriginal(ts)
	want := time.Date(2023, 4, 5, 6, 7, 8, 0, time.Local)

	cases := []struct {
		name        string
		item        []byte
		ilocVersion byte
	}{
		{"iloc v0", append(be32(0), tiff...), 0},
		{"iloc v1", append(be32(0), tiff...), 1},
		{"iloc v2", append(be32(0), tiff...), 2},
		// Apple writes an "Exif\0\0" prefix and points the offset past it.
		{"exif prefix", append(append(be32(6), "Exif\x00\x00"...), tiff...), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "IMG_0001.HEIC")
			if err := os.WriteFile(path, buildHEIC(t, tc.item, tc.ilocVersion), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := ExtractTime(path)
			if err != nil {
				t.Fatal(err)
			}
			if got.Source != SourceEXIF {
				t.Fatalf("source = %v, want exif", got.Source)
			}
			if !got.When.Equal(want) {
				t.Fatalf("when = %v, want %v", got.When, want)
			}
		})
	}
}

func TestExtractTimeHEICWithoutEXIFFallsBackToMTime(t *testing.T) {
	ftyp := box("ftyp", []byte("heic"), be32(0), []byte("mif1heic"))
	path := filepath.Join(t.TempDir(), "IMG_0002.HEIC")
	if err := os.WriteFile(path, append(ftyp, box("mdat", []byte("pixels"))...), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ExtractTime(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != SourceMTime {
		t.Fatalf("source = %v, want mtime", got.Source)
	}
}

func TestHeifEXIFRejectsNonHEIF(t *testing.T) {
	if _, err := heifEXIF(bytes.NewReader([]byte("\xff\xd8\xff\xe1not a heif file"))); err != errNotHEIF {
		t.Fatalf("err = %v, want errNotHEIF", err)
	}
}

func TestHeifEXIFTruncatedMetaDoesNotPanic(t *testing.T) {
	full := buildHEIC(t, append(be32(0), testutil.TIFFWithDateTimeOriginal("2023:04:05 06:07:08")...), 1)
	for n := 0; n < len(full); n++ {
		heifEXIF(bytes.NewReader(full[:n]))
	}
}
