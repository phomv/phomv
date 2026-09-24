package processor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// HEIC/HEIF files are ISO-BMFF containers. EXIF lives in an item of type
// "Exif" whose bytes are located via the meta box's iinf (item info) and
// iloc (item location) boxes. heifEXIF extracts that item and returns a
// TIFF stream that goexif can decode directly.

const (
	maxMetaBoxSize = 16 << 20 // meta holds only item tables; real files are a few KB
	maxEXIFSize    = 4 << 20  // EXIF is capped at 64 KB in JPEG; be generous
)

var errNotHEIF = errors.New("not a HEIF container")

type boxHeader struct {
	typ        string
	headerSize int64
	size       int64 // total box size including header; -1 means "to end of file"
}

func readBoxHeader(r io.Reader) (boxHeader, error) {
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return boxHeader{}, err
	}
	h := boxHeader{typ: string(b[4:8]), headerSize: 8, size: int64(binary.BigEndian.Uint32(b[:4]))}
	switch h.size {
	case 0:
		h.size = -1
	case 1:
		var ext [8]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return boxHeader{}, err
		}
		h.headerSize = 16
		h.size = int64(binary.BigEndian.Uint64(ext[:]))
	}
	if h.size != -1 && h.size < h.headerSize {
		return boxHeader{}, fmt.Errorf("heif: invalid %q box size %d", h.typ, h.size)
	}
	return h, nil
}

// heifEXIF returns the TIFF-formatted EXIF payload of a HEIF file.
func heifEXIF(r io.ReadSeeker) ([]byte, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	first, err := readBoxHeader(r)
	if err != nil || first.typ != "ftyp" {
		return nil, errNotHEIF
	}

	// Walk top-level boxes to find meta.
	h, pos := first, int64(0)
	for h.typ != "meta" {
		if h.size == -1 {
			return nil, errors.New("heif: no meta box")
		}
		pos += h.size
		if _, err := r.Seek(pos, io.SeekStart); err != nil {
			return nil, err
		}
		if h, err = readBoxHeader(r); err != nil {
			return nil, fmt.Errorf("heif: no meta box: %w", err)
		}
	}
	if h.size == -1 || h.size-h.headerSize > maxMetaBoxSize {
		return nil, errors.New("heif: meta box too large")
	}
	meta := make([]byte, h.size-h.headerSize)
	if _, err := io.ReadFull(r, meta); err != nil {
		return nil, err
	}

	extents, err := findEXIFItem(meta)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for _, e := range extents {
		if e.length > maxEXIFSize || int64(buf.Len())+e.length > maxEXIFSize {
			return nil, errors.New("heif: exif item too large")
		}
		if _, err := r.Seek(e.offset, io.SeekStart); err != nil {
			return nil, err
		}
		if _, err := io.CopyN(&buf, r, e.length); err != nil {
			return nil, err
		}
	}

	// Exif item payload: 4-byte offset to the TIFF header, then the data.
	data := buf.Bytes()
	if len(data) < 4 {
		return nil, errors.New("heif: exif item truncated")
	}
	off := int64(binary.BigEndian.Uint32(data[:4])) + 4
	if off > int64(len(data)) {
		return nil, errors.New("heif: bad exif tiff header offset")
	}
	return data[off:], nil
}

type extent struct{ offset, length int64 }

// findEXIFItem parses the body of a meta box (a FullBox) and returns the
// file extents of its Exif item.
func findEXIFItem(meta []byte) ([]extent, error) {
	if len(meta) < 4 {
		return nil, errors.New("heif: meta box truncated")
	}
	var iinf, iloc []byte
	rest := meta[4:] // skip version + flags
	for len(rest) >= 8 {
		size := int64(binary.BigEndian.Uint32(rest[:4]))
		typ := string(rest[4:8])
		hdr := int64(8)
		if size == 1 {
			if len(rest) < 16 {
				break
			}
			size, hdr = int64(binary.BigEndian.Uint64(rest[8:16])), 16
		} else if size == 0 {
			size = int64(len(rest))
		}
		if size < hdr || size > int64(len(rest)) {
			return nil, fmt.Errorf("heif: invalid %q box size", typ)
		}
		switch typ {
		case "iinf":
			iinf = rest[hdr:size]
		case "iloc":
			iloc = rest[hdr:size]
		}
		rest = rest[size:]
	}
	if iinf == nil || iloc == nil {
		return nil, errors.New("heif: missing iinf or iloc")
	}

	id, err := exifItemID(iinf)
	if err != nil {
		return nil, err
	}
	return itemExtents(iloc, id)
}

// cursor is a bounds-checked big-endian reader over a byte slice.
type cursor struct {
	b   []byte
	err error
}

func (c *cursor) take(n int) []byte {
	if c.err != nil {
		return nil
	}
	if n > len(c.b) {
		c.err = errors.New("heif: box truncated")
		return nil
	}
	v := c.b[:n]
	c.b = c.b[n:]
	return v
}

// uint reads an n-byte (0, 2, 4 or 8) big-endian unsigned integer.
func (c *cursor) uint(n int) uint64 {
	v := c.take(n)
	if v == nil {
		return 0
	}
	switch n {
	case 0:
		return 0
	case 2:
		return uint64(binary.BigEndian.Uint16(v))
	case 4:
		return uint64(binary.BigEndian.Uint32(v))
	case 8:
		return binary.BigEndian.Uint64(v)
	}
	c.err = fmt.Errorf("heif: unsupported field size %d", n)
	return 0
}

func exifItemID(iinf []byte) (uint32, error) {
	c := &cursor{b: iinf}
	version := c.uint(4) >> 24
	if version == 0 {
		c.uint(2) // entry_count
	} else {
		c.uint(4)
	}
	for c.err == nil && len(c.b) >= 8 {
		size := int(binary.BigEndian.Uint32(c.b[:4]))
		typ := string(c.b[4:8])
		if size < 8 || size > len(c.b) {
			return 0, errors.New("heif: invalid infe box size")
		}
		box := c.take(size)
		if typ != "infe" {
			continue
		}
		e := &cursor{b: box[8:]}
		v := e.uint(4) >> 24
		if v < 2 {
			continue // v0/v1 entries carry no item_type
		}
		var id uint32
		if v == 2 {
			id = uint32(e.uint(2))
		} else {
			id = uint32(e.uint(4))
		}
		e.uint(2) // item_protection_index
		if t := e.take(4); e.err == nil && string(t) == "Exif" {
			return id, nil
		}
	}
	if c.err != nil {
		return 0, c.err
	}
	return 0, errors.New("heif: no Exif item")
}

func itemExtents(iloc []byte, want uint32) ([]extent, error) {
	c := &cursor{b: iloc}
	version := c.uint(4) >> 24
	sizes := c.take(2)
	if c.err != nil {
		return nil, c.err
	}
	offsetSize, lengthSize := int(sizes[0]>>4), int(sizes[0]&0xf)
	baseOffsetSize, indexSize := int(sizes[1]>>4), int(sizes[1]&0xf)
	if version == 0 {
		indexSize = 0 // reserved in v0
	}

	var count uint64
	if version < 2 {
		count = c.uint(2)
	} else {
		count = c.uint(4)
	}
	for i := uint64(0); i < count && c.err == nil; i++ {
		var id uint32
		if version < 2 {
			id = uint32(c.uint(2))
		} else {
			id = uint32(c.uint(4))
		}
		method := uint64(0)
		if version == 1 || version == 2 {
			method = c.uint(2) & 0xf
		}
		c.uint(2) // data_reference_index
		base := c.uint(baseOffsetSize)
		n := c.uint(2)
		extents := make([]extent, 0, n)
		for j := uint64(0); j < n && c.err == nil; j++ {
			c.uint(indexSize)
			off := c.uint(offsetSize)
			length := c.uint(lengthSize)
			extents = append(extents, extent{offset: int64(base + off), length: int64(length)})
		}
		if c.err != nil {
			return nil, c.err
		}
		if id != want {
			continue
		}
		if method != 0 {
			return nil, fmt.Errorf("heif: unsupported exif construction method %d", method)
		}
		if len(extents) == 0 {
			return nil, errors.New("heif: exif item has no extents")
		}
		return extents, nil
	}
	if c.err != nil {
		return nil, c.err
	}
	return nil, errors.New("heif: exif item not in iloc")
}
