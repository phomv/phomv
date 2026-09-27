package worker

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/phomv/phomv/internal/filesystem"
	"github.com/phomv/phomv/internal/testutil"
)

// readTree maps every file under root (relative, slash-separated) to its content.
func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		tree[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func runAll(t *testing.T, cfg Config) ([]Result, *Stats) {
	t.Helper()
	ch, stats := Run(context.Background(), cfg)
	var out []Result
	for r := range ch {
		if r.Err != nil {
			t.Errorf("%s: %v", r.Src, r.Err)
		}
		out = append(out, r)
	}
	return out, stats
}

func TestEndToEnd(t *testing.T) {
	src := t.TempDir()
	mtime := time.Date(2023, 3, 15, 12, 0, 0, 0, time.Local)
	exifJPEG := string(testutil.JPEGWithDateTimeOriginal("2021:07:04 10:00:00"))
	sourceFiles := map[string]string{
		"card1/IMG_0001.jpg":     exifJPEG, // EXIF wins over mtime
		"card1/sub/IMG_0002.jpg": "two-a",  // no EXIF: dated by mtime
		"card2/IMG_0002.jpg":     "two-b",  // same name + day, different bytes: _1
		"card2/IMG_0001.jpg":     exifJPEG, // byte-identical to card1's: skipped
		"card2/notes.txt":        "not a photo",
	}
	for rel, content := range sourceFiles {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	const day0001 = "2021/2021_07/2021_07_04/"
	const day0002 = "2023/2023_03/2023_03_15/"
	wantPaths := []string{day0001 + "IMG_0001.jpg", day0002 + "IMG_0002.jpg", day0002 + "IMG_0002_1.jpg"}
	checkLibrary := func(t *testing.T, dst string) {
		t.Helper()
		lib := readTree(t, dst)
		var got []string
		for p := range lib {
			got = append(got, p)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, wantPaths) {
			t.Fatalf("library = %v, want %v", got, wantPaths)
		}
		if lib[day0001+"IMG_0001.jpg"] != exifJPEG {
			t.Error("IMG_0001.jpg content mismatch")
		}
		// Worker scheduling decides which IMG_0002 gets the suffix.
		pair := []string{lib[day0002+"IMG_0002.jpg"], lib[day0002+"IMG_0002_1.jpg"]}
		sort.Strings(pair)
		if !reflect.DeepEqual(pair, []string{"two-a", "two-b"}) {
			t.Errorf("IMG_0002 variants = %q, want two-a and two-b", pair)
		}
	}
	wantStats := func(t *testing.T, got *Stats, discovered, processed, skipped uint64) {
		t.Helper()
		if got.Discovered != discovered || got.Processed != processed || got.Skipped != skipped ||
			got.Failed != 0 || got.Unknown != 0 || got.WalkErrors != 0 {
			t.Fatalf("stats = %+v, want discovered=%d processed=%d skipped=%d",
				*got, discovered, processed, skipped)
		}
	}

	dst := t.TempDir()
	copyCfg := Config{Source: src, Destination: dst, Operation: filesystem.OpCopy, Workers: 4}

	t.Run("dry run predicts the copy", func(t *testing.T) {
		cfg := copyCfg
		cfg.DryRun = true
		results, stats := runAll(t, cfg)
		wantStats(t, stats, 4, 3, 1)
		var planned []string
		for _, r := range results {
			if r.Status == StatusOK {
				rel, _ := filepath.Rel(dst, r.Dst)
				planned = append(planned, filepath.ToSlash(rel))
			}
		}
		sort.Strings(planned)
		if !reflect.DeepEqual(planned, wantPaths) {
			t.Errorf("planned = %v, want %v", planned, wantPaths)
		}
		if lib := readTree(t, dst); len(lib) != 0 {
			t.Errorf("dry run wrote %v", lib)
		}
	})

	t.Run("copy", func(t *testing.T) {
		_, stats := runAll(t, copyCfg)
		wantStats(t, stats, 4, 3, 1)
		checkLibrary(t, dst)
		if got := readTree(t, src); !reflect.DeepEqual(got, sourceFiles) {
			t.Errorf("copy changed the source: %v", got)
		}
	})

	t.Run("copy again is a no-op", func(t *testing.T) {
		_, stats := runAll(t, copyCfg)
		wantStats(t, stats, 4, 0, 4)
		checkLibrary(t, dst)
	})

	t.Run("move cleans up the source", func(t *testing.T) {
		moveDst := t.TempDir()
		_, stats := runAll(t, Config{Source: src, Destination: moveDst, Operation: filesystem.OpMove, Workers: 4})
		wantStats(t, stats, 4, 3, 1)
		checkLibrary(t, moveDst)
		if got := readTree(t, src); !reflect.DeepEqual(got, map[string]string{"card2/notes.txt": "not a photo"}) {
			t.Errorf("source after move = %v, want only card2/notes.txt", got)
		}
		if _, err := os.Stat(filepath.Join(src, "card1")); !os.IsNotExist(err) {
			t.Errorf("emptied card1/ was not removed: %v", err)
		}
	})
}
