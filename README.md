<p align="center">
  <img src="docs/logo.png" alt="phomv logo" width="480">
</p>

# phomv

`phomv` (photo move) is a high-performance CLI utility written in Go that
organizes photo directories into a `YYYY/YYYY_MM/YYYY_MM_DD` hierarchy based on EXIF
metadata. It is designed to be fast, safe, and decoupled enough that the same
core engine can later back a Wails or Fyne GUI.

## Features

- Recursive scan of common photo formats (`.jpg`, `.jpeg`, `.png`, `.heic`,
  `.cr2`, `.nef`, `.arw`, `.dng`, `.tif`, `.tiff`).
- EXIF `DateTimeOriginal` extraction with file-mtime fallback.
- Concurrent worker pool (configurable, default 4 workers).
- Atomic-ish copy via temp files + rename; cross-device move fallback.
- Idempotent: identical files are skipped via SHA-256 content compare.
- Collision-safe naming (`IMG_001_1.jpg`, `IMG_001_2.jpg`, ...).
- Dry-run mode that logs every planned action without touching disk.
- Files with unreadable timestamps go to an `Unknown/` bucket instead of
  crashing the run.
- Structured logging via zerolog.

## Install

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
