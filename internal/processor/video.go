package processor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// MP4/QuickTime files are ISO-BMFF containers like HEIF. The movie header
// box (moov/mvhd) records the creation time as seconds since 1904-01-01 UTC.

var quickTimeEpoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

func readQuickTimeTime(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()
	return quickTimeCreation(f)
}

// quickTimeCreation returns the mvhd creation time, in local time so that
// videos bucket by day the same way photos do.
func quickTimeCreation(r io.ReadSeeker) (time.Time, error) {
	moov, body, err := findBox(r, 0, -1, "moov")
	if err != nil {
		return time.Time{}, err
	}
	end := int64(-1)
	if moov.size != -1 {
		end = body - moov.headerSize + moov.size
	}
	if _, _, err := findBox(r, body, end, "mvhd"); err != nil {
		return time.Time{}, err
	}

	// FullBox version + flags, then creation_time (32-bit in v0, 64-bit in v1).
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:4]); err != nil {
		return time.Time{}, err
	}
	var secs uint64
	switch hdr[0] {
	case 0:
		if _, err := io.ReadFull(r, hdr[4:8]); err != nil {
			return time.Time{}, err
		}
		secs = uint64(binary.BigEndian.Uint32(hdr[4:8]))
	case 1:
		if _, err := io.ReadFull(r, hdr[4:12]); err != nil {
			return time.Time{}, err
		}
		secs = binary.BigEndian.Uint64(hdr[4:12])
	default:
		return time.Time{}, fmt.Errorf("mvhd: unknown version %d", hdr[0])
	}
	// Many encoders leave the field zero; a value past year ~2^33 is garbage.
	if secs == 0 || secs > 1<<33 {
		return time.Time{}, errors.New("mvhd: no creation time")
	}
	return quickTimeEpoch.Add(time.Duration(secs) * time.Second).Local(), nil
}

// findBox scans sibling boxes in [pos, end) (end < 0: to EOF) for typ and
// returns its header and body offset, leaving r positioned at the body.
func findBox(r io.ReadSeeker, pos, end int64, typ string) (boxHeader, int64, error) {
	for end < 0 || pos < end {
		if _, err := r.Seek(pos, io.SeekStart); err != nil {
			return boxHeader{}, 0, err
		}
		h, err := readBoxHeader(r)
		if err != nil {
			return boxHeader{}, 0, fmt.Errorf("no %s box: %w", typ, err)
		}
		if h.typ == typ {
			return h, pos + h.headerSize, nil
		}
		if h.size == -1 {
			break
		}
		pos += h.size
	}
	return boxHeader{}, 0, fmt.Errorf("no %s box", typ)
}
