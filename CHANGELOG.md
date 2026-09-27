# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Discovery skips hidden files/folders, NAS and OS clutter (`@eaDir`,
  `#recycle`, `@Recycle`, `@Recently-Snapshot`, `$RECYCLE.BIN`,
  `System Volume Information`, `lost+found`) and macOS `._*` files by
  default, so thumbnails no longer end up in the library.
  `--include-hidden` opts back in; `-x/--exclude <glob>` (repeatable) adds
  your own skips. Skipped folders are logged at debug level and counted as
  `excluded_dirs` (#22).
- Shell completions (bash, zsh, fish, PowerShell) and man pages ship in the
  release archives and are installed by Homebrew; `make docs` generates
  them locally (#24).
- Progress at the default log level: a live `N done / M found` counter on a
  terminal, or a `progress` log line every 10 s when stderr isn't one (#18).
- Videos (`.mp4`, `.mov`, `.m4v`, `.3gp`) are organized alongside photos,
  dated from the QuickTime/MP4 `mvhd` creation time with mtime fallback. Logs
  report their time source as `quicktime`. `--no-videos` leaves them in
  place (#19).
- Sidecar files travel with their photo: `IMG_1234.xmp` / `IMG_1234.CR2.xmp`,
  Apple `IMG_1234.AAE`, and a Live Photo's `IMG_1234.mov` land next to the
  photo and take its collision suffix (`IMG_1234_1.HEIC` → `IMG_1234_1.mov`).
  A different file already at a sidecar's target is reported as a failure,
  never overwritten. `--no-sidecars` turns pairing off (#20).

### Changed
- Faster duplicate checks: same-size files are compared byte-for-byte and
  stop at the first difference instead of being SHA-256 hashed in full
  (~5× faster for identical files, far faster when they differ early), and
  discovery uses `filepath.WalkDir`, saving a `stat` per file (#23).

### Fixed
- Re-running an import no longer piles up identical copies: a file that
  already exists at a `_1`, `_2`, … variant of its destination is now skipped
  as a duplicate, not written again under the next free suffix (#17).
- Byte-identical files processed at the same time by different workers were
  all written (`IMG.jpg`, `IMG_1.jpg`, …) instead of the extras being skipped
  as duplicates (#34).

## [0.1.2] - 2026-09-23

### Fixed
- HEIC/HEIF photos are now dated from their EXIF `DateTimeOriginal` instead of
  silently falling back to file mtime (#12).
- `copy`/`move` now refuse to run when `--src` and `--dest` are the same
  directory or one is nested inside the other (symlinks resolved); previously
  the walker could re-process files it had just written (#13).
- Unreadable directories (permissions, I/O errors) were silently skipped during
  discovery. Each is now logged as a warning, counted as `walk_errors` in the
  summary, and makes the run exit nonzero (#14).
- `move`: when a duplicate's source file could not be deleted, the error was
  dropped and the file reported as a skipped duplicate. It is now reported as
  failed, and the run exits nonzero (#15).
- `--dry-run` reported the same destination for every file sharing a name
  (e.g. `IMG_0001.jpg` from different folders); it now predicts the `_1`, `_2`,
  … suffixes and duplicate skips a real run would produce (#16).
- `make build`/`make install` used the old `github.com/chinny/phomv` module path
  and failed.

## [0.1.1] - 2026-06-06

### Changed
- Module path moved from `github.com/chinny/phomv` to `github.com/phomv/phomv`
  (repo transferred to the dedicated `phomv` org). Re-`go install` from the new path.

### Added
- Homebrew tap publishing via GoReleaser → `phomv/homebrew-tap`
  (`brew install phomv/tap/phomv`).
- winget manifest publishing via GoReleaser → PRs against `microsoft/winget-pkgs`
  through the `phomv/winget-pkgs` fork (`winget install phomv.phomv`).

## [0.1.0] - 2026-06-05

### Added
- Concurrent worker pool pipeline (`copy` and `move` subcommands)
- EXIF `DateTimeOriginal` extraction with file-mtime fallback
- SHA-256 idempotency check to skip duplicate files
- Collision-safe suffix resolution (`IMG_001_1.jpg`, `IMG_001_2.jpg`, …)
- `Unknown/` bucket for files with unreadable timestamps
- Dry-run mode (`--dry-run`)
- Structured logging via zerolog (`--verbose`)
- Cross-platform builds for Linux, macOS, Windows via `make release`
