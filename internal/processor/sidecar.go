package processor

import (
	"path/filepath"
	"slices"
	"strings"
)

// sidecarExtensions are companion files holding edits for the photo that
// shares their name: Lightroom/darktable settings and Apple edits.
var sidecarExtensions = map[string]struct{}{
	".xmp": {},
	".aae": {},
}

// Sidecar is a companion file that travels with a photo.
type Sidecar struct {
	Name string // file name in the photo's directory
	// Suffix follows the photo's stem in Name (".xmp", ".CR2.xmp", ".mov"),
	// so the sidecar can be renamed to match wherever the photo lands.
	Suffix string
}

// PairSidecars matches companion files in one directory to the photo they
// belong to, given the directory's file names. It recognizes IMG_1234.xmp or
// IMG_1234.aae next to IMG_1234.<photo ext>, darktable's IMG_1234.CR2.xmp
// next to IMG_1234.CR2, and a Live Photo's IMG_1234.mov next to
// IMG_1234.HEIC. Names compare case-insensitively; when several photos share
// a stem (RAW+JPEG), stem-named sidecars go with the first in name order.
// It returns photo name -> sidecars.
func PairSidecars(names []string) map[string][]Sidecar {
	names = slices.Sorted(slices.Values(names))
	byName := map[string]string{} // lower-cased photo name -> photo
	byStem := map[string]string{} // lower-cased stem -> first photo with it
	for _, n := range names {
		if !IsSupported(n) || IsVideo(n) {
			continue
		}
		byName[strings.ToLower(n)] = n
		stem := strings.ToLower(strings.TrimSuffix(n, filepath.Ext(n)))
		if _, ok := byStem[stem]; !ok {
			byStem[stem] = n
		}
	}

	pairs := map[string][]Sidecar{}
	for _, n := range names {
		ext := filepath.Ext(n)
		base := strings.TrimSuffix(n, ext)
		lowerExt := strings.ToLower(ext)
		_, edits := sidecarExtensions[lowerExt]
		if !edits && lowerExt != ".mov" {
			continue
		}
		if edits {
			if photo, ok := byName[strings.ToLower(base)]; ok {
				pairs[photo] = append(pairs[photo], Sidecar{Name: n, Suffix: filepath.Ext(base) + ext})
				continue
			}
		}
		if photo, ok := byStem[strings.ToLower(base)]; ok {
			pairs[photo] = append(pairs[photo], Sidecar{Name: n, Suffix: ext})
		}
	}
	return pairs
}
