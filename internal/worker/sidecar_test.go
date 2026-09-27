package worker

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/phomv/phomv/internal/filesystem"
)

// livePhotoTree writes IMG_1.HEIC with its Live Photo .MOV and .AAE edits,
// plus a raw with a darktable sidecar, all dated 2023-03-15.
func livePhotoTree(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	when := time.Date(2023, 3, 15, 12, 0, 0, 0, time.Local)
	for name, content := range map[string]string{
		"IMG_1.HEIC": "photo", "IMG_1.MOV": "motion", "IMG_1.AAE": "apple-edits",
		"raw/DSC_2.NEF": "raw", "raw/DSC_2.NEF.xmp": "darktable",
	} {
		p := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
	return src
}

const sidecarDay = "2023/2023_03/2023_03_15/"

func TestRunSidecarsFollowSuffixedPhoto(t *testing.T) {
	src := livePhotoTree(t)
	dst := t.TempDir()
	// Another photo already owns IMG_1.HEIC, so ours lands on IMG_1_1.
	day := filepath.Join(dst, filepath.FromSlash(sidecarDay))
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(day, "IMG_1.HEIC"), []byte("someone else"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{Source: src, Destination: dst, Operation: filesystem.OpMove, Workers: 2}
	results, stats := runAll(t, cfg)
	if stats.Discovered != 2 || stats.Processed != 2 || stats.Sidecars != 3 || stats.Failed != 0 {
		t.Fatalf("stats = %+v, want discovered=2 processed=2 sidecars=3", *stats)
	}
	for _, r := range results {
		if r.SidecarOf != "" && r.Source.String() != "mtime" {
			t.Errorf("%s: sidecar source = %v, want the photo's", r.Src, r.Source)
		}
	}
	want := map[string]string{
		sidecarDay + "IMG_1.HEIC":    "someone else",
		sidecarDay + "IMG_1_1.HEIC":  "photo",
		sidecarDay + "IMG_1_1.MOV":   "motion",
		sidecarDay + "IMG_1_1.AAE":   "apple-edits",
		sidecarDay + "DSC_2.NEF":     "raw",
		sidecarDay + "DSC_2.NEF.xmp": "darktable",
	}
	if got := readTree(t, dst); !reflect.DeepEqual(got, want) {
		t.Errorf("library = %v\nwant      %v", got, want)
	}
	if got := readTree(t, src); len(got) != 0 {
		t.Errorf("move left sources behind: %v", got)
	}
}

func TestRunSidecarsRerun(t *testing.T) {
	src := livePhotoTree(t)
	dst := t.TempDir()
	cfg := Config{Source: src, Destination: dst, Operation: filesystem.OpCopy, Workers: 2}
	runAll(t, cfg)

	t.Run("identical sidecars are skipped with their photo", func(t *testing.T) {
		_, stats := runAll(t, cfg)
		if stats.Skipped != 5 || stats.Processed != 0 || stats.Sidecars != 0 {
			t.Fatalf("stats = %+v, want 5 skipped", *stats)
		}
	})

	t.Run("a sidecar edited since is reported, not overwritten", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(src, "IMG_1.AAE"), []byte("newer edits"), 0o644); err != nil {
			t.Fatal(err)
		}
		results, stats := Run(t.Context(), cfg)
		var failed []string
		for r := range results {
			if r.Status == StatusFailed {
				failed = append(failed, filepath.Base(r.Src))
			}
		}
		if !reflect.DeepEqual(failed, []string{"IMG_1.AAE"}) || stats.Failed != 1 {
			t.Fatalf("failed = %v (stats %+v), want just IMG_1.AAE", failed, *stats)
		}
		b, _ := os.ReadFile(filepath.Join(dst, filepath.FromSlash(sidecarDay), "IMG_1.AAE"))
		if string(b) != "apple-edits" {
			t.Errorf("library sidecar was overwritten: %q", b)
		}
	})
}

func TestRunSidecarsDryRun(t *testing.T) {
	src := livePhotoTree(t)
	dst := t.TempDir()
	results, stats := runAll(t, Config{Source: src, Destination: dst, Operation: filesystem.OpMove, Workers: 2, DryRun: true})
	if stats.Sidecars != 3 {
		t.Errorf("stats = %+v, want 3 sidecars planned", *stats)
	}
	for _, r := range results {
		if r.SidecarOf == "" {
			continue
		}
		rel, _ := filepath.Rel(dst, r.Dst)
		if filepath.Dir(filepath.ToSlash(rel))+"/" != sidecarDay {
			t.Errorf("%s planned at %s, want in %s", r.Src, rel, sidecarDay)
		}
	}
	if got := readTree(t, dst); len(got) != 0 {
		t.Errorf("dry run wrote %v", got)
	}
	if got := readTree(t, src); len(got) != 5 {
		t.Errorf("dry run touched the source: %v", got)
	}
}

func TestRunSidecarFlags(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want []string // files in the day folder
	}{
		{
			name: "no-sidecars: .MOV is an ordinary video, edits stay behind",
			cfg:  Config{SkipSidecars: true},
			want: []string{"DSC_2.NEF", "IMG_1.HEIC", "IMG_1.MOV"},
		},
		{
			name: "no-videos still carries a Live Photo's motion",
			cfg:  Config{SkipVideos: true},
			want: []string{"DSC_2.NEF", "DSC_2.NEF.xmp", "IMG_1.AAE", "IMG_1.HEIC", "IMG_1.MOV"},
		},
		{
			name: "both",
			cfg:  Config{SkipSidecars: true, SkipVideos: true},
			want: []string{"DSC_2.NEF", "IMG_1.HEIC"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			cfg.Source, cfg.Destination = livePhotoTree(t), t.TempDir()
			cfg.Operation, cfg.Workers = filesystem.OpCopy, 2
			runAll(t, cfg)
			var got []string
			for p := range readTree(t, cfg.Destination) {
				got = append(got, filepath.Base(p))
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
