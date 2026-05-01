package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveCollisionNoConflict(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.jpg")
	writeFile(t, src, "hello")
	dst := filepath.Join(dir, "out", "a.jpg")
	got, skip, err := ResolveCollision(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("did not expect skip")
	}
	if got != dst {
		t.Fatalf("got %q, want %q", got, dst)
	}
}

func TestResolveCollisionIdenticalSkips(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.jpg")
	dst := filepath.Join(dir, "out", "a.jpg")
	writeFile(t, src, "same-bytes")
	writeFile(t, dst, "same-bytes")
	got, skip, err := ResolveCollision(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Fatalf("expected skip; got %q", got)
	}
}

func TestResolveCollisionDifferentSuffixes(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.jpg")
	dst := filepath.Join(dir, "out", "a.jpg")
	writeFile(t, src, "new-content")
	writeFile(t, dst, "existing-content")
	got, skip, err := ResolveCollision(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("did not expect skip")
	}
	want := filepath.Join(dir, "out", "a_1.jpg")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyCopy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.jpg")
	writeFile(t, src, "data")
	dst := filepath.Join(dir, "out", "a.jpg")
	if err := Apply(OpCopy, src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("source should still exist after copy")
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "data" {
		t.Fatalf("got %q, want %q", b, "data")
	}
}

func TestApplyMove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.jpg")
	writeFile(t, src, "data")
	dst := filepath.Join(dir, "out", "a.jpg")
	if err := Apply(OpMove, src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be gone after move")
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "data" {
		t.Fatalf("got %q, want %q", b, "data")
	}
}

func TestCleanupEmptyDirs(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "empty", "deeper")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(root, "keep")
	writeFile(t, filepath.Join(keep, "f.txt"), "x")

	if err := CleanupEmptyDirs(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Fatal("empty dir should have been removed")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("non-empty dir should be preserved")
	}
}
