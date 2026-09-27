package processor

import (
	"reflect"
	"testing"
)

func TestPairSidecars(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  map[string][]Sidecar
	}{
		{
			name:  "live photo with apple edits",
			files: []string{"IMG_1234.HEIC", "IMG_1234.MOV", "IMG_1234.AAE"},
			want: map[string][]Sidecar{
				"IMG_1234.HEIC": {{"IMG_1234.AAE", ".AAE"}, {"IMG_1234.MOV", ".MOV"}},
			},
		},
		{
			name:  "lightroom and darktable xmp",
			files: []string{"a.CR2", "a.xmp", "b.nef", "b.nef.xmp"},
			want: map[string][]Sidecar{
				"a.CR2": {{"a.xmp", ".xmp"}},
				"b.nef": {{"b.nef.xmp", ".nef.xmp"}},
			},
		},
		{
			name:  "case-insensitive match",
			files: []string{"img_1.jpg", "IMG_1.XMP"},
			want:  map[string][]Sidecar{"img_1.jpg": {{"IMG_1.XMP", ".XMP"}}},
		},
		{
			name:  "raw+jpeg: stem sidecar goes to first by name, full-name to its photo",
			files: []string{"DSC_1.NEF", "DSC_1.JPG", "DSC_1.xmp", "DSC_1.JPG.xmp"},
			want: map[string][]Sidecar{
				"DSC_1.JPG": {{"DSC_1.JPG.xmp", ".JPG.xmp"}, {"DSC_1.xmp", ".xmp"}},
			},
		},
		{
			name:  "orphans and non-sidecars are left alone",
			files: []string{"clip.mov", "notes.xmp", "a.jpg", "a.txt", "a.mp4", "b.mov.xmp"},
			want:  map[string][]Sidecar{},
		},
		{
			name:  "video is never an owner",
			files: []string{"clip.mp4", "clip.xmp", "clip.mov"},
			want:  map[string][]Sidecar{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PairSidecars(tc.files); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
