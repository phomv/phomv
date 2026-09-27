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
	"sync"
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
// It atomically creates an empty file at the destination path to prevent
// Time-of-Check to Time-of-Use (TOCTOU) race conditions.
//
// If the existing file at desired, or at any suffixed variant tried before a
// free one is found, is byte-identical to src, ResolveCollision returns that
// path and true to signal the caller can skip the operation.
//
// Pass the same claims to every call in a run so that a file racing an
// identical one still in flight is recognized as its duplicate; nil disables
// that tracking.
func ResolveCollision(src, desired string, claims *Claims) (string, bool, error) {
	return resolveCollision(src, desired, claims, false)
}

// Claims records which source each destination path was handed out to during
// a run. A real run reserves paths on disk and records them here; a dry run
// records them here instead of reserving. Safe for concurrent use; the zero
// value is ready.
type Claims struct {
	mu     sync.Mutex
	byPath map[string]string // destination -> src that claimed it
}

func (c *Claims) key(p string) string {
	if caseInsensitive {
		return strings.ToLower(p)
	}
	return p
}

// claim marks p as taken by src unless it is already claimed or exists on disk.
func (c *Claims) claim(src, p string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, taken := c.byPath[c.key(p)]; taken {
		return false, nil
	}
	if _, err := os.Stat(p); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	c.setLocked(src, p)
	return true, nil
}

// reserve creates p on disk for src and records the claim under one lock, so
// no caller can find the empty reservation without also finding its claimant.
func (c *Claims) reserve(src, p string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ok, err := reserveOnDisk(p)
	if ok {
		c.setLocked(src, p)
	}
	return ok, err
}

func (c *Claims) setLocked(src, p string) {
	if c.byPath == nil {
		c.byPath = map[string]string{}
	}
	c.byPath[c.key(p)] = src
}

// claimant returns the src that claimed p in this run, if any.
func (c *Claims) claimant(p string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	src, ok := c.byPath[c.key(p)]
	return src, ok
}

// ResolveCollisionReadOnly is the dry-run counterpart of ResolveCollision: it
// writes nothing, but records its picks in claims so that later calls in the
// same run see them as taken, mirroring the real run's on-disk reservations.
func ResolveCollisionReadOnly(src, desired string, claims *Claims) (string, bool, error) {
	return resolveCollision(src, desired, claims, true)
}

// ReserveExact is ResolveCollision without suffixing: it reserves exactly dst
// and reports true, or, if dst is taken, reports whether it already holds
// src's bytes.
func ReserveExact(src, dst string, claims *Claims) (reserved, duplicate bool, err error) {
	return tryPath(src, dst, claims, false)
}

// ReserveExactReadOnly is the dry-run counterpart of ReserveExact.
func ReserveExactReadOnly(src, dst string, claims *Claims) (reserved, duplicate bool, err error) {
	return tryPath(src, dst, claims, true)
}

func resolveCollision(src, desired string, claims *Claims, dryRun bool) (string, bool, error) {
	ext := filepath.Ext(desired)
	stem := strings.TrimSuffix(desired, ext)
	for i := 0; i < 10000; i++ {
		candidate := desired
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d%s", stem, i, ext)
		}
		reserved, duplicate, err := tryPath(src, candidate, claims, dryRun)
		if err != nil {
			return "", false, err
		}
		if reserved || duplicate {
			return candidate, duplicate, nil
		}
	}
	return "", false, fmt.Errorf("could not resolve collision for %s", desired)
}

// tryPath reserves p for src (in claims for a dry run, otherwise on disk,
// recording the reservation in claims when non-nil). If p is taken, it
// reports whether p already holds src's bytes.
func tryPath(src, p string, claims *Claims, dryRun bool) (reserved, duplicate bool, err error) {
	switch {
	case dryRun:
		reserved, err = claims.claim(src, p)
	case claims != nil:
		reserved, err = claims.reserve(src, p)
	default:
		reserved, err = reserveOnDisk(p)
	}
	if reserved || err != nil {
		return reserved, false, err
	}
	duplicate, err = isDuplicate(src, p, claims)
	return false, duplicate, err
}

// isDuplicate reports whether the file at the taken path p holds src's bytes,
// so re-running an import skips files that landed on a suffixed name last
// time. A path claimed earlier in this run may still be an empty reservation
// (or, in a dry run, never written), so compare against its claimant instead.
func isDuplicate(src, p string, claims *Claims) (bool, error) {
	if claims != nil {
		if claimant, ok := claims.claimant(p); ok {
			same, err := sameContent(src, claimant)
			if !errors.Is(err, os.ErrNotExist) {
				return same, err
			}
			// A move consumed the claimant; p now holds its bytes.
		}
	}
	same, err := sameContent(src, p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil // removed since we checked; nothing to compare
	}
	return same, err
}

// reserveOnDisk atomically creates an empty file at p (and its parent dirs),
// reporting false if p already exists.
func reserveOnDisk(p string) (bool, error) {
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o666)
	if err != nil {
		if os.IsExist(err) || errors.Is(err, os.ErrExist) {
			return false, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return false, fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err)
			}
			f, err = os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o666)
			if err != nil {
				if os.IsExist(err) || errors.Is(err, os.ErrExist) {
					return false, nil
				}
				return false, err
			}
		} else {
			return false, err
		}
	}
	f.Close()
	return true, nil
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
