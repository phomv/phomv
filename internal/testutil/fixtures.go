// Package testutil builds small in-memory image fixtures for tests.
package testutil

import (
	"bytes"
	"encoding/binary"
)

// TIFFWithDateTimeOriginal builds a little-endian TIFF stream whose Exif
// sub-IFD holds a single DateTimeOriginal tag ("YYYY:MM:DD HH:MM:SS").
func TIFFWithDateTimeOriginal(ts string) []byte {
	le := binary.LittleEndian
	var b bytes.Buffer
	b.WriteString("II")
	binary.Write(&b, le, uint16(42))
	binary.Write(&b, le, uint32(8)) // IFD0 offset

	// IFD0 at 8: one entry pointing at the Exif IFD (ends at 26).
	binary.Write(&b, le, uint16(1))
	ifdEntry(&b, 0x8769, 4, 1, 26)
	binary.Write(&b, le, uint32(0))

	// Exif IFD at 26: DateTimeOriginal, ASCII, value at 44.
	binary.Write(&b, le, uint16(1))
	ifdEntry(&b, 0x9003, 2, uint32(len(ts)+1), 44)
	binary.Write(&b, le, uint32(0))

	b.WriteString(ts)
	b.WriteByte(0)
	return b.Bytes()
}

// JPEGWithDateTimeOriginal wraps TIFFWithDateTimeOriginal in a minimal JPEG:
// SOI, an APP1 "Exif" segment, EOI. It carries no pixels, which is all EXIF
// parsing needs.
func JPEGWithDateTimeOriginal(ts string) []byte {
	payload := append([]byte("Exif\x00\x00"), TIFFWithDateTimeOriginal(ts)...)
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8, 0xFF, 0xE1})
	binary.Write(&b, binary.BigEndian, uint16(2+len(payload)))
	b.Write(payload)
	b.Write([]byte{0xFF, 0xD9})
	return b.Bytes()
}

// JPEGWithoutEXIF is a minimal JPEG (SOI, EOI) with no metadata segments.
func JPEGWithoutEXIF() []byte { return []byte{0xFF, 0xD8, 0xFF, 0xD9} }

func ifdEntry(b *bytes.Buffer, tag, typ uint16, count, value uint32) {
	le := binary.LittleEndian
	binary.Write(b, le, tag)
	binary.Write(b, le, typ)
	binary.Write(b, le, count)
	binary.Write(b, le, value)
}
