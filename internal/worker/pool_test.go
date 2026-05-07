package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chinny/phomv/internal/filesystem"
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
