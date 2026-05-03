# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
make build       # build → ./bin/phomv (injects version/commit/date via ldflags)
make test        # go test ./...
make vet         # go vet ./...
make fmt         # gofmt -s -w .
make install     # go install to $GOBIN
make release     # cross-compile to ./dist for linux/darwin/windows

# Run a single package's tests
go test ./internal/processor/...
go test ./internal/filesystem/...
go test ./internal/worker/...
```

## Architecture

The CLI (`cmd/phomv/`) is a thin Cobra shell. All business logic lives in `internal/`:

```
internal/
  processor/   — timestamp extraction (EXIF > mtime fallback) + destination path formatting
  worker/      — concurrent pipeline: filepath.Walk → jobs channel → fixed worker pool → Results channel
  filesystem/  — file I/O primitives: copy (via temp+rename), move (rename with cross-device fallback),
                 SHA-256 idempotency check, collision suffix resolution, empty-dir cleanup
```

**Data flow:** `worker.Run(ctx, Config)` returns `(<-chan Result, *Stats)`. The CLI consumes the channel to print progress; a future GUI can consume the same channel without touching the engine.

**Key types:**
- `worker.Config` — source/dest paths, `filesystem.Operation` (copy/move), worker count, dry-run flag
- `worker.Result` — src, resolved dst, `Status` (OK/SkippedDuplicate/UnknownDate/Failed), `processor.TimeSource`, optional error
- `processor.PhotoTime` — resolved `time.Time` + `TimeSource` (EXIF or mtime)

**Collision resolution** (`filesystem.ResolveCollision`): identical files (SHA-256) are skipped; differing files with the same name get `_1`, `_2`, … suffixes before the extension.

**Unknown dates**: files where neither EXIF nor mtime is readable go to `<dest>/Unknown/<original-filename>` instead of failing the run.

**Module path:** `github.com/chinny/phomv`
