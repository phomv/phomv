// Package filesystem wraps file IO with safety helpers: collision-free
// destination resolution, content-aware idempotency, and atomic-ish moves.
package filesystem

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Operation describes the action requested on a file.
type Operation int

const (
	OpCopy Operation = iota
	OpMove
)

func (o Operation) String() string {
	if o == OpMove {
		return "move"
	}
	return "copy"
}

// ResolveCollision returns a non-existent destination path. If desired exists,
// it appends incremental suffixes (_1, _2, ...) before the extension.
//
// If the existing file at desired is byte-identical to src, ResolveCollision
// returns ("", true, nil) to signal the caller can skip the operation.
func ResolveCollision(src, desired string) (string, bool, error) {
	if _, err := os.Stat(desired); errors.Is(err, os.ErrNotExist) {
		return desired, false, nil
	} else if err != nil {
		return "", false, err
	}

	same, err := sameContent(src, desired)
	if err != nil {
		return "", false, err
	}
	if same {
		return "", true, nil
	}

	ext := filepath.Ext(desired)
	stem := strings.TrimSuffix(desired, ext)
	for i := 1; i < 10000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", stem, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, false, nil
		} else if err != nil {
			return "", false, err
		}
	}
	return "", false, fmt.Errorf("could not resolve collision for %s", desired)
}

// Apply executes op (copy or move) from src to dst, creating parent dirs.
func Apply(op Operation, src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	switch op {
	case OpCopy:
		return copyFile(src, dst)
	case OpMove:
		return moveFile(src, dst)
	default:
		return fmt.Errorf("unknown operation %d", op)
	}
}

// CleanupEmptyDirs walks root bottom-up and removes directories that contain
// no files. The root itself is preserved.
func CleanupEmptyDirs(root string) error {
	var dirs []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		d := dirs[i]
		if d == root {
			continue
		}
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			_ = os.Remove(d)
		}
	}
	return nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Cross-device rename fails; fall back to copy + delete.
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".*.phomv-tmp")
	if err != nil {
		return err
	}
	tmp := out.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			os.Remove(tmp)
		}
	}()

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if info, err := os.Stat(src); err == nil {
		_ = os.Chmod(tmp, info.Mode())
		_ = os.Chtimes(tmp, info.ModTime(), info.ModTime())
	}
	removeTmp = false
	return os.Rename(tmp, dst)
}

func sameContent(a, b string) (bool, error) {
	infoA, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	infoB, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if infoA.Size() != infoB.Size() {
		return false, nil
	}
	hA, err := hashFile(a)
	if err != nil {
		return false, err
	}
	hB, err := hashFile(b)
	if err != nil {
		return false, err
	}
	return hA == hB, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
