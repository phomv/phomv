package worker

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/phomv/phomv/internal/filesystem"
)

func TestFilterSkip(t *testing.T) {
	def := filter{}
	hidden := filter{includeHidden: true}
	excl := filter{exclude: []string{"Screenshots", "*.png", "2019/raw"}}
	cases := []struct {
		f     filter
		rel   string
		isDir bool
		want  string // skip reason, "" = keep
	}{
		{def, ".", true, ""},
		{def, "DCIM/IMG_1.jpg", false, ""},
		{def, ".thumbnails", true, "hidden"},
		{def, "DCIM/.thumbnails", true, "hidden"},
		{def, ".hidden.jpg", false, "hidden"},
		{def, "DCIM/._IMG_1.jpg", false, "appledouble"},
		{def, "@eaDir", true, "system"},
		{def, "photos/@eaDir", true, "system"},
		{def, "$RECYCLE.BIN", true, "system"},
		{def, "System Volume Information", true, "system"},
		{def, "@eaDir", false, ""}, // a file with that name isn't a folder
		{hidden, ".thumbnails", true, ""},
		{hidden, "@eaDir", true, ""},
		{hidden, "._IMG_1.jpg", false, "appledouble"}, // never an image
		{excl, "phone/Screenshots", true, "excluded"},
		{excl, "a/b/shot.png", false, "excluded"},
		{excl, "2019/raw", true, "excluded"},
		{excl, "2020/raw", true, ""},
		{excl, "a/2019/raw", true, ""}, // path patterns anchor at --src
	}
	for _, tc := range cases {
		skip, why := tc.f.skip(tc.rel, tc.isDir)
		if skip != (tc.want != "") || why != tc.want {
			t.Errorf("skip(%q, dir=%v) = %v %q, want %q", tc.rel, tc.isDir, skip, why, tc.want)
		}
	}
}

func TestValidateExcludes(t *testing.T) {
	if err := ValidateExcludes([]string{"*.png", "a/b", "IMG_[0-9]*"}); err != nil {
		t.Errorf("valid patterns rejected: %v", err)
	}
	if err := ValidateExcludes([]string{"ok", "[abc"}); err == nil {
		t.Error("malformed pattern accepted")
	}
}

func TestRunSkipsHiddenSystemAndExcluded(t *testing.T) {
	src := t.TempDir()
	when := time.Date(2023, 3, 15, 12, 0, 0, 0, time.Local)
	for _, rel := range []string{
		"DCIM/IMG_1.jpg",
		"DCIM/._IMG_1.jpg",
		"DCIM/.thumbnails/IMG_1.jpg",
		"@eaDir/IMG_1.jpg/SYNOPHOTO_THUMB_XL.jpg",
		"$RECYCLE.BIN/old.jpg",
		"Screenshots/shot.png",
		"DCIM/IMG_2.png",
	} {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name         string
		cfg          Config
		want         []string
		excludedDirs uint64
	}{
		{"defaults", Config{}, []string{"IMG_1.jpg", "IMG_2.png", "shot.png"}, 3},
		{"exclude", Config{Exclude: []string{"Screenshots", "*.png"}}, []string{"IMG_1.jpg"}, 4},
		{"include hidden", Config{IncludeHidden: true}, []string{
			"IMG_1.jpg", "IMG_1_1.jpg", "IMG_2.png", "SYNOPHOTO_THUMB_XL.jpg", "old.jpg", "shot.png",
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			cfg.Source, cfg.Destination = src, t.TempDir()
			cfg.Operation, cfg.Workers = filesystem.OpCopy, 2
			results, stats := runAll(t, cfg)
			var got []string
			for p := range readTree(t, cfg.Destination) {
				got = append(got, filepath.Base(p))
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("library = %v, want %v", got, tc.want)
			}
			var excluded uint64
			for _, r := range results {
				if r.Status == StatusExcludedDir {
					excluded++
				}
			}
			if stats.ExcludedDirs != tc.excludedDirs || excluded != tc.excludedDirs {
				t.Errorf("excluded dirs = %d (results %d), want %d", stats.ExcludedDirs, excluded, tc.excludedDirs)
			}
		})
	}
}

func TestRunHiddenSourceRootIsScanned(t *testing.T) {
	src := filepath.Join(t.TempDir(), ".import")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stats := runAll(t, Config{Source: src, Destination: t.TempDir(), Operation: filesystem.OpCopy, DryRun: true})
	if stats.Discovered != 1 {
		t.Fatalf("discovered %d in a dot-named --src, want 1", stats.Discovered)
	}
}

func TestRunSkipsUnreadableLostAndFound(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	src := t.TempDir()
	lf := filepath.Join(src, "lost+found")
	if err := os.Mkdir(lf, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(lf, 0o755) })
	_, stats := runAll(t, Config{Source: src, Destination: t.TempDir(), Operation: filesystem.OpCopy, DryRun: true})
	if stats.WalkErrors != 0 || stats.ExcludedDirs != 1 {
		t.Fatalf("stats = %+v, want lost+found skipped without a walk error", *stats)
	}
}
