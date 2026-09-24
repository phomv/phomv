package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CheckOverlap returns an error if src and dest are the same directory or one
// contains the other. Discovery walks src while workers write into dest, so
// any nesting lets the walker pick up freshly written files and process them
// again. Symlinks are resolved; dest need not exist yet.
func CheckOverlap(src, dest string) error {
	s, err := resolvePath(src)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	d, err := resolvePath(dest)
	if err != nil {
		return fmt.Errorf("resolve destination: %w", err)
	}
	switch {
	case samePath(s, d):
		return fmt.Errorf("source and destination are the same directory (%s)", s)
	case within(d, s):
		return fmt.Errorf("destination %s is inside source %s", d, s)
	case within(s, d):
		return fmt.Errorf("source %s is inside destination %s", s, d)
	}
	return nil
}

// resolvePath returns the absolute, symlink-free form of p. If p does not
// exist, its nearest existing ancestor is resolved and the rest re-joined.
func resolvePath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	var missing []string
	for cur := abs; ; {
		real, err := filepath.EvalSymlinks(cur)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			return real, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		missing = append(missing, filepath.Base(cur))
		cur = parent
	}
}

// Default filesystems on macOS and Windows are case-insensitive.
var caseInsensitive = runtime.GOOS == "darwin" || runtime.GOOS == "windows"

func samePath(a, b string) bool {
	if caseInsensitive {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// within reports whether child is strictly inside parent.
func within(child, parent string) bool {
	if caseInsensitive {
		child, parent = strings.ToLower(child), strings.ToLower(parent)
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
