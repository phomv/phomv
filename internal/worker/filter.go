package worker

import (
	"fmt"
	"path"
	"strings"
)

// systemDirs are OS, NAS and recycle-bin folders that hold thumbnails,
// deleted files or metadata rather than originals.
var systemDirs = map[string]struct{}{
	"@eaDir":                    {}, // Synology thumbnails/metadata
	"#recycle":                  {}, // Synology recycle bin
	"@Recycle":                  {}, // QNAP recycle bin
	"@Recently-Snapshot":        {}, // QNAP snapshots
	"$RECYCLE.BIN":              {}, // Windows
	"System Volume Information": {}, // Windows
	"lost+found":                {}, // fsck recovery; usually root-only
}

// filter decides which paths discovery skips.
type filter struct {
	includeHidden bool
	exclude       []string
}

// ValidateExcludes reports the first malformed --exclude glob.
func ValidateExcludes(patterns []string) error {
	for _, p := range patterns {
		if _, err := path.Match(p, ""); err != nil {
			return fmt.Errorf("bad --exclude pattern %q: %w", p, err)
		}
	}
	return nil
}

// skip reports whether the entry at rel (slash-separated, relative to the
// source root) should be left alone, and why. The root itself is never skipped.
func (f filter) skip(rel string, isDir bool) (bool, string) {
	if rel == "." {
		return false, ""
	}
	name := path.Base(rel)
	// AppleDouble files (._IMG_1234.jpg) are macOS metadata forks that
	// share the photo's extension; they are never images.
	if !isDir && strings.HasPrefix(name, "._") {
		return true, "appledouble"
	}
	if !f.includeHidden {
		if strings.HasPrefix(name, ".") {
			return true, "hidden"
		}
		if _, ok := systemDirs[name]; ok && isDir {
			return true, "system"
		}
	}
	for _, p := range f.exclude {
		// Patterns are validated up front, so Match can't fail here.
		if ok, _ := path.Match(p, name); ok {
			return true, "excluded"
		}
		if ok, _ := path.Match(p, rel); ok {
			return true, "excluded"
		}
	}
	return false, ""
}
