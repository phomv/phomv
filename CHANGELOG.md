# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
