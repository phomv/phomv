# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
