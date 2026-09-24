package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckOverlap(t *testing.T) {
	root := t.TempDir()
	mkdir := func(p string) string {
		t.Helper()
		full := filepath.Join(root, p)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
		return full
	}
	pics := mkdir("Pictures")
	other := mkdir("Other")
	mkdir("Pictures2")

	cases := []struct {
		name    string
		src     string
		dest    string
		wantErr bool
	}{
		{"disjoint", pics, other, false},
		{"sibling with shared prefix", pics, filepath.Join(root, "Pictures2"), false},
		{"same dir", pics, pics, true},
		{"same dir, trailing dot segment", pics, filepath.Join(pics, "."), true},
		{"dest inside src", pics, filepath.Join(pics, "library"), true},
		{"dest inside src, not yet created", pics, filepath.Join(pics, "a", "b"), true},
		{"src inside dest", mkdir("Pictures/2023"), pics, true},
		{"relative dest via ..", pics, filepath.Join(other, "..", "Pictures", "lib"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckOverlap(tc.src, tc.dest)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CheckOverlap(%q, %q) = %v, wantErr %v", tc.src, tc.dest, err, tc.wantErr)
			}
		})
	}
}

func TestCheckOverlapResolvesSymlinks(t *testing.T) {
	root := t.TempDir()
	pics := filepath.Join(root, "Pictures")
	if err := os.MkdirAll(pics, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(pics, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := CheckOverlap(pics, filepath.Join(link, "library")); err == nil {
		t.Fatal("expected overlap through symlinked dest")
	}
	if err := CheckOverlap(link, pics); err == nil {
		t.Fatal("expected overlap through symlinked src")
	}
}

func TestCheckOverlapDoesNotCreateDest(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "src", "lib")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	CheckOverlap(filepath.Join(root, "src"), dest)
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("dest was created: %v", err)
	}
}
