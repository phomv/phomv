<p align="center">
  <img src="docs/phomv-lockup.png" alt="phomv — organize photos, fast" width="480">
</p>

# phomv

[![CI](https://github.com/phomv/phomv/actions/workflows/ci.yml/badge.svg)](https://github.com/phomv/phomv/actions/workflows/ci.yml)

`phomv` (photo move) is a high-performance CLI utility written in Go that
organizes photo directories into a `YYYY/YYYY_MM/YYYY_MM_DD` hierarchy based on EXIF
metadata. It is designed to be fast, safe, and decoupled enough that the same
core engine can later back a Wails or Fyne GUI.

## Features

- Recursive scan of common photo formats (`.jpg`, `.jpeg`, `.png`, `.heic`,
  `.cr2`, `.nef`, `.arw`, `.dng`, `.tif`, `.tiff`) and videos (`.mp4`, `.mov`,
  `.m4v`, `.3gp`).
- EXIF `DateTimeOriginal` (photos) or QuickTime `mvhd` creation time (videos),
  with file-mtime fallback.
- Concurrent worker pool (configurable, default 4 workers).
- Atomic-ish copy via temp files + rename; cross-device move fallback.
- Idempotent: identical files are skipped via a byte-for-byte content compare.
- Collision-safe naming (`IMG_001_1.jpg`, `IMG_001_2.jpg`, ...).
- Sidecars travel with their photo and take its new name: `.xmp`
  (Lightroom, darktable's `IMG_1.CR2.xmp`), Apple `.aae` edits, and a Live
  Photo's `.mov`.
- Dry-run mode that logs every planned action without touching disk.
- Skips thumbnail/system clutter by default: dot-folders and dot-files,
  NAS and OS folders (`@eaDir`, `#recycle`, `@Recycle`, `$RECYCLE.BIN`,
  `System Volume Information`, `lost+found`), and macOS `._*` metadata
  files. Add your own skips with `--exclude`.
- Files with unreadable timestamps go to an `Unknown/` bucket instead of
  crashing the run.
- Structured logging via zerolog.

## Install

### Homebrew (macOS / Linux)

```sh
brew install phomv/tap/phomv
```

### Windows (winget)

```sh
winget install phomv.phomv
```

### Pre-built binaries

Grab the latest release for your platform from the [releases page](https://github.com/phomv/phomv/releases).

### From source

```sh
go install github.com/phomv/phomv/cmd/phomv@latest
```

Or build locally:

```sh
make build       # produces ./bin/phomv
make install     # installs into $GOBIN
make release     # cross-compiles to ./dist for linux/macOS/windows
```

Requires Go 1.24+.

## Usage

```sh
# Preview what a copy would do
phomv copy --src ~/Pictures/import --dest ~/Pictures/library --dry-run

# Actually copy
phomv copy -s ~/Pictures/import -d ~/Pictures/library -w 8

# Move and clean up empty source dirs
phomv move -s /mnt/sdcard -d ~/Pictures/library

# Skip screenshots and one folder of raws
phomv copy -s /mnt/sdcard -d ~/Pictures/library -x Screenshots -x '2019/raw'

# Version
phomv version
```

### Flags

| Flag              | Default | Description                                    |
| ----------------- | ------- | ---------------------------------------------- |
| `-s, --src`       | -       | Source directory (required)                    |
| `-d, --dest`      | -       | Destination directory (required)               |
| `-n, --dry-run`   | `false` | Simulate execution without touching disk       |
| `-w, --workers`   | `4`     | Number of concurrent workers                   |
| `-v, --verbose`   | `false` | Enable debug logging                           |
| `--no-videos`     | `false` | Leave video files in place (Live Photo `.mov`s still follow their photo) |
| `-x, --exclude`   | -       | Skip files/folders whose name, or path under `--src`, matches this glob (repeatable) |
| `--include-hidden`| `false` | Also scan dot-files/folders and system/NAS folders (`._*` files are always skipped) |
| `--no-sidecars`   | `false` | Don't carry `.xmp`/`.aae`/Live Photo `.mov` with their photo |

## Project layout

```
phomv/
├── cmd/phomv/          # Cobra CLI entry point
├── internal/
│   ├── processor/      # EXIF extraction + destination path formatting
│   ├── worker/         # Discovery + worker pool pipeline
│   └── filesystem/     # Copy/move/collision/idempotency primitives
├── Makefile            # Build, test, cross-compile
└── go.mod
```

The CLI is a thin shell around `internal/worker.Run`, which streams `Result`
values on a channel. A future GUI can consume the same channel to render
progress without touching the engine.

## Testing

```sh
make test
```
