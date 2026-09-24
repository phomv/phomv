package worker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/phomv/phomv/internal/filesystem"
)

func TestRunCopyWithMTimeFallback(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Two photos with no EXIF, dated via mtime.
	a := filepath.Join(src, "nested", "a.jpg")
	b := filepath.Join(src, "b.jpeg")
	if err := os.MkdirAll(filepath.Dir(a), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("fake-jpeg"+p), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	when := time.Date(2023, 3, 15, 10, 0, 0, 0, time.UTC)
	for _, p := range []string{a, b} {
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}

	results, stats := Run(context.Background(), Config{
		Source: src, Destination: dst,
		Operation: filesystem.OpCopy,
		Workers:   2,
	})
	for r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error for %s: %v", r.Src, r.Err)
		}
	}

	if stats.Discovered != 2 || stats.Processed != 2 {
		t.Fatalf("stats = %+v, want Discovered=2 Processed=2", stats)
	}
	for _, name := range []string{"a.jpg", "b.jpeg"} {
		want := filepath.Join(dst, "2023", "2023_03", "2023_03_15", name)
		if _, err := os.Stat(want); err != nil {
			t.Errorf("expected file at %s: %v", want, err)
		}
	}
}

func TestRunIdempotentSkipsIdentical(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	p := filepath.Join(src, "a.jpg")
	if err := os.WriteFile(p, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p, when, when); err != nil {
		t.Fatal(err)
	}

	cfg := Config{Source: src, Destination: dst, Operation: filesystem.OpCopy, Workers: 1}
	for r := range firstChan(Run(context.Background(), cfg)) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	results, stats := Run(context.Background(), cfg)
	for r := range results {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	if stats.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %+v", stats)
	}
}

func TestRunDryRunDoesNotWrite(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	p := filepath.Join(src, "a.jpg")
	if err := os.WriteFile(p, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	results, _ := Run(context.Background(), Config{
		Source: src, Destination: dst,
		Operation: filesystem.OpCopy, Workers: 1, DryRun: true,
	})
	for range results {
	}
	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry-run wrote files: %+v", entries)
	}
}

func firstChan(c <-chan Result, _ *Stats) <-chan Result { return c }

func TestRunReportsUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	src := t.TempDir()
	dst := t.TempDir()

	ok := filepath.Join(src, "ok.jpg")
	locked := filepath.Join(src, "locked")
	if err := os.WriteFile(ok, []byte("fake-jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "hidden.jpg"), []byte("fake-jpeg-2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	results, stats := Run(context.Background(), Config{
		Source: src, Destination: dst,
		Operation: filesystem.OpCopy,
		Workers:   2,
	})
	var walkErrs []Result
	for r := range results {
		if r.Status == StatusWalkError {
			walkErrs = append(walkErrs, r)
		}
	}
	if len(walkErrs) != 1 || walkErrs[0].Src != locked || walkErrs[0].Err == nil {
		t.Fatalf("walk error results = %+v, want one for %s", walkErrs, locked)
	}
	if stats.WalkErrors != 1 {
		t.Fatalf("WalkErrors = %d, want 1", stats.WalkErrors)
	}
	if stats.Processed != 1 {
		t.Fatalf("Processed = %d, want 1 (readable file still handled)", stats.Processed)
	}
}

func TestRunReportsMissingSource(t *testing.T) {
	results, stats := Run(context.Background(), Config{
		Source:      filepath.Join(t.TempDir(), "gone"),
		Destination: t.TempDir(),
		Operation:   filesystem.OpCopy,
	})
	for range results {
	}
	if stats.WalkErrors != 1 {
		t.Fatalf("WalkErrors = %d, want 1", stats.WalkErrors)
	}
}

// seedDuplicate copies a single photo from a fresh src into dst so that a
// following move run sees it as a duplicate.
func seedDuplicate(t *testing.T) (src, dst, photo string) {
	t.Helper()
	src, dst = t.TempDir(), t.TempDir()
	photo = filepath.Join(src, "a.jpg")
	if err := os.WriteFile(photo, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(photo, when, when); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Source: src, Destination: dst, Operation: filesystem.OpCopy, Workers: 1}
	for r := range firstChan(Run(context.Background(), cfg)) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	return src, dst, photo
}

func TestRunMoveDuplicateRemovesSource(t *testing.T) {
	src, dst, photo := seedDuplicate(t)
	results, stats := Run(context.Background(), Config{Source: src, Destination: dst, Operation: filesystem.OpMove, Workers: 1})
	for r := range results {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	if stats.Skipped != 1 || stats.Failed != 0 {
		t.Fatalf("stats = %+v, want 1 skipped, 0 failed", stats)
	}
	if _, err := os.Stat(photo); !os.IsNotExist(err) {
		t.Fatalf("source still present: %v", err)
	}
}

func TestRunMoveDuplicateRemoveFailureIsFailed(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	src, dst, photo := seedDuplicate(t)
	// A read-only directory blocks unlinking its entries.
	if err := os.Chmod(src, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(src, 0o755) })

	results, stats := Run(context.Background(), Config{Source: src, Destination: dst, Operation: filesystem.OpMove, Workers: 1})
	var got []Result
	for r := range results {
		got = append(got, r)
	}
	if len(got) != 1 || got[0].Status != StatusFailed || got[0].Err == nil {
		t.Fatalf("results = %+v, want one failed result with an error", got)
	}
	if stats.Failed != 1 || stats.Skipped != 0 {
		t.Fatalf("stats = %+v, want 1 failed, 0 skipped", stats)
	}
	if _, err := os.Stat(photo); err != nil {
		t.Fatalf("source should remain after failed removal: %v", err)
	}
}

// TestRunDryRunMatchesRealRun checks that a dry run predicts the same
// destinations as a real run when several sources share a filename. Contents
// are distinct: duplicate detection only compares the base path (#17), so
// with a repeated body the outcome would depend on worker ordering.
func TestRunDryRunMatchesRealRun(t *testing.T) {
	src := t.TempDir()
	when := time.Date(2022, 6, 1, 12, 0, 0, 0, time.UTC)
	for dir, body := range map[string]string{"a": "one", "b": "two", "c": "three"} {
		p := filepath.Join(src, dir, "IMG_0001.jpg")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}

	outcome := func(dryRun bool) (map[string]bool, Stats) {
		dst := t.TempDir()
		results, stats := Run(context.Background(), Config{
			Source: src, Destination: dst, Operation: filesystem.OpCopy, Workers: 4, DryRun: dryRun,
		})
		dsts := map[string]bool{}
		for r := range results {
			if r.Err != nil {
				t.Fatal(r.Err)
			}
			if r.Status == StatusOK {
				rel, _ := filepath.Rel(dst, r.Dst)
				dsts[rel] = true
			}
		}
		return dsts, *stats
	}

	dryDsts, dry := outcome(true)
	realDsts, real := outcome(false)
	if len(dryDsts) != 3 || len(realDsts) != 3 {
		t.Fatalf("want 3 distinct destinations; dry=%v real=%v", dryDsts, realDsts)
	}
	for d := range realDsts {
		if !dryDsts[d] {
			t.Fatalf("dry run missed %s; dry=%v real=%v", d, dryDsts, realDsts)
		}
	}
	if dry.Processed != real.Processed || dry.Skipped != real.Skipped {
		t.Fatalf("stats differ: dry=%+v real=%+v", dry, real)
	}
}
