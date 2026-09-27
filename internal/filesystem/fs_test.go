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
	got, skip, err := ResolveCollisionReadOnly(src, dst, &Claims{})
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
	got, skip, err := ResolveCollisionReadOnly(src, dst, &Claims{})
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
	got, skip, err := ResolveCollisionReadOnly(src, dst, &Claims{})
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

func TestResolveCollisionReserve(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.jpg")
	writeFile(t, src, "new-content")
	dst := filepath.Join(dir, "out", "a.jpg")
	writeFile(t, dst, "existing-content")

	got, skip, err := ResolveCollision(src, dst, &Claims{})
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

	// Verify the file was actually created as empty
	info, err := os.Stat(want)
	if err != nil {
		t.Fatalf("expected reserved file to exist: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected reserved file to be empty, got size %d", info.Size())
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

func TestCopyFileWithExistingTemp(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.jpg")
	writeFile(t, src, "new-data")
	dst := filepath.Join(dir, "dst.jpg")

	// Pre-create what used to be the predictable temp file
	oldPredictableTemp := dst + ".phomv-tmp"
	writeFile(t, oldPredictableTemp, "old-garbage")

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new-data" {
		t.Fatalf("got %q, want %q", b, "new-data")
	}

	// The old predictable temp file should still exist (or at least not have been used/overwritten)
	// Actually, the new code uses os.CreateTemp which shouldn't touch this file.
	oldContent, err := os.ReadFile(oldPredictableTemp)
	if err != nil {
		t.Fatal(err)
	}
	if string(oldContent) != "old-garbage" {
		t.Fatal("old predictable temp file was overwritten")
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

func TestResolveCollisionReadOnlyHonorsClaims(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "x", "IMG_0001.jpg")
	b := filepath.Join(dir, "y", "IMG_0001.jpg")
	c := filepath.Join(dir, "z", "IMG_0001.jpg")
	writeFile(t, a, "first")
	writeFile(t, b, "second")
	writeFile(t, c, "first") // same bytes as a
	dst := filepath.Join(dir, "out", "IMG_0001.jpg")
	claims := &Claims{}

	got, skip, err := ResolveCollisionReadOnly(a, dst, claims)
	if err != nil || skip || got != dst {
		t.Fatalf("a: got %q skip=%v err=%v, want %q", got, skip, err, dst)
	}
	got, skip, err = ResolveCollisionReadOnly(b, dst, claims)
	if want := filepath.Join(dir, "out", "IMG_0001_1.jpg"); err != nil || skip || got != want {
		t.Fatalf("b: got %q skip=%v err=%v, want %q", got, skip, err, want)
	}
	if _, skip, err = ResolveCollisionReadOnly(c, dst, claims); err != nil || !skip {
		t.Fatalf("c: skip=%v err=%v, want skip as duplicate of a", skip, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote to disk: %v", err)
	}
}

func TestResolveCollisionSkipsDuplicateOfSuffixedVariant(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "IMG_001.jpg")
	dst := filepath.Join(dir, "out", "IMG_001.jpg")
	writeFile(t, src, "incoming")
	writeFile(t, dst, "different")
	writeFile(t, filepath.Join(dir, "out", "IMG_001_1.jpg"), "also-different")
	writeFile(t, filepath.Join(dir, "out", "IMG_001_2.jpg"), "incoming")

	for name, resolve := range map[string]func() (string, bool, error){
		"real":    func() (string, bool, error) { return ResolveCollision(src, dst, &Claims{}) },
		"dry-run": func() (string, bool, error) { return ResolveCollisionReadOnly(src, dst, &Claims{}) },
	} {
		got, skip, err := resolve()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !skip {
			t.Fatalf("%s: expected skip as duplicate of IMG_001_2.jpg; got %q", name, got)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "IMG_001_3.jpg")); !os.IsNotExist(err) {
		t.Fatal("no new variant should have been reserved")
	}
}

func TestResolveCollisionReadOnlyDuplicateOfClaimedVariant(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out", "IMG_0001.jpg")
	writeFile(t, dst, "on-disk")
	a := filepath.Join(dir, "x", "IMG_0001.jpg")
	b := filepath.Join(dir, "y", "IMG_0001.jpg")
	writeFile(t, a, "first")
	writeFile(t, b, "first") // same bytes as a, which claims _1
	claims := &Claims{}

	got, skip, err := ResolveCollisionReadOnly(a, dst, claims)
	if want := filepath.Join(dir, "out", "IMG_0001_1.jpg"); err != nil || skip || got != want {
		t.Fatalf("a: got %q skip=%v err=%v, want %q", got, skip, err, want)
	}
	if got, skip, err := ResolveCollisionReadOnly(b, dst, claims); err != nil || !skip {
		t.Fatalf("b: got %q skip=%v err=%v, want skip as duplicate of a", got, skip, err)
	}
}

func TestResolveCollisionSkipsDuplicateOfInFlightClaim(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "x", "IMG_0001.jpg")
	b := filepath.Join(dir, "y", "IMG_0001.jpg")
	writeFile(t, a, "same-bytes")
	writeFile(t, b, "same-bytes")
	dst := filepath.Join(dir, "out", "IMG_0001.jpg")
	claims := &Claims{}

	got, skip, err := ResolveCollision(a, dst, claims)
	if err != nil || skip || got != dst {
		t.Fatalf("a: got %q skip=%v err=%v, want %q", got, skip, err, dst)
	}
	// a's bytes haven't been written yet: dst is still an empty reservation.
	if got, skip, err := ResolveCollision(b, dst, claims); err != nil || !skip {
		t.Fatalf("b: got %q skip=%v err=%v, want skip as duplicate of in-flight a", got, skip, err)
	}
}

func TestResolveCollisionSkipReportsMatchedPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.jpg")
	dst := filepath.Join(dir, "out", "a.jpg")
	writeFile(t, src, "incoming")
	writeFile(t, dst, "different")
	writeFile(t, filepath.Join(dir, "out", "a_1.jpg"), "incoming")
	got, skip, err := ResolveCollision(src, dst, &Claims{})
	if want := filepath.Join(dir, "out", "a_1.jpg"); err != nil || !skip || got != want {
		t.Fatalf("got %q skip=%v err=%v, want skip at %q", got, skip, err, want)
	}
}

func TestReserveExact(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.xmp")
	writeFile(t, src, "edits")
	free := filepath.Join(dir, "out", "free.xmp")
	same := filepath.Join(dir, "out", "same.xmp")
	other := filepath.Join(dir, "out", "other.xmp")
	writeFile(t, same, "edits")
	writeFile(t, other, "older edits")

	for _, tc := range []struct {
		dst                 string
		reserved, duplicate bool
	}{
		{free, true, false},
		{same, false, true},
		{other, false, false},
	} {
		reserved, duplicate, err := ReserveExact(src, tc.dst, &Claims{})
		if err != nil || reserved != tc.reserved || duplicate != tc.duplicate {
			t.Errorf("%s: reserved=%v duplicate=%v err=%v, want %v/%v",
				filepath.Base(tc.dst), reserved, duplicate, err, tc.reserved, tc.duplicate)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "other_1.xmp")); !os.IsNotExist(err) {
		t.Error("ReserveExact must not fall back to a suffixed name")
	}
}

func TestSameContent(t *testing.T) {
	big := make([]byte, 3*compareChunk)
	for i := range big {
		big[i] = byte(i * 7)
	}
	flip := func(i int) []byte {
		b := append([]byte(nil), big...)
		b[i] ^= 0xFF
		return b
	}
	cases := []struct {
		name string
		a, b []byte
		want bool
	}{
		{"both empty", nil, nil, true},
		{"identical small", []byte("same"), []byte("same"), true},
		{"identical, exact chunk multiple", big, big, true},
		{"identical, partial last chunk", big[:2*compareChunk+5], big[:2*compareChunk+5], true},
		{"differ in first byte", big, flip(0), false},
		{"differ at chunk boundary", big, flip(compareChunk), false},
		{"differ in last byte", big, flip(len(big) - 1), false},
		{"different sizes", big, big[:len(big)-1], false},
	}
	dir := t.TempDir()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
			writeFile(t, a, string(tc.a))
			writeFile(t, b, string(tc.b))
			got, err := sameContent(a, b)
			if err != nil || got != tc.want {
				t.Fatalf("sameContent = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	if _, err := sameContent(filepath.Join(dir, "a"), filepath.Join(dir, "gone")); !os.IsNotExist(err) {
		t.Errorf("missing file: err = %v, want not-exist", err)
	}
}
