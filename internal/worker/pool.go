// Package worker provides a concurrent file-processing pipeline.
//
// Discovery walks the source tree and emits Jobs; a fixed-size worker pool
// consumes them and emits Results. Progress events are exposed on a channel
// so a CLI or future GUI can render counters without coupling to the engine.
package worker

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/phomv/phomv/internal/filesystem"
	"github.com/phomv/phomv/internal/processor"
)

// Job is a single file slated for processing.
type Job struct {
	Path     string
	Sidecars []processor.Sidecar // companions in the same directory
}

// Status describes the outcome for one job.
type Status int

const (
	StatusOK Status = iota
	StatusSkippedDuplicate
	StatusUnknownDate
	StatusFailed
	// StatusWalkError marks a path discovery could not read (permissions,
	// I/O error); its subtree, if any, was not scanned.
	StatusWalkError
	// StatusExcludedDir marks a directory discovery skipped (hidden, system,
	// or matched by --exclude); Reason says why.
	StatusExcludedDir
)

// Result is the outcome of processing one Job.
type Result struct {
	Src    string
	Dst    string
	Status Status
	Source processor.TimeSource
	DryRun bool
	Err    error
	// SidecarOf is the photo this file travels with, if it is a sidecar.
	SidecarOf string
	// Reason explains a StatusExcludedDir: "hidden", "system" or "excluded".
	Reason string
}

// Config controls a Run invocation.
type Config struct {
	Source      string
	Destination string
	Operation   filesystem.Operation
	Workers     int
	DryRun      bool
	SkipVideos  bool // leave video files in place
	// IncludeHidden scans dot-files, dot-directories and system/NAS
	// folders (@eaDir, $RECYCLE.BIN, ...), which are skipped by default.
	IncludeHidden bool
	// Exclude holds globs matched against each name and its path relative
	// to Source; matches are skipped. Check them with ValidateExcludes.
	Exclude []string
	// SkipSidecars stops pairing .xmp/.aae/Live Photo .mov files with their
	// photo; paired .mov files are then treated as ordinary videos.
	SkipSidecars bool
}

// Stats reports counters after Run completes.
type Stats struct {
	Discovered uint64
	Processed  uint64
	Skipped    uint64
	Unknown    uint64
	Sidecars   uint64
	Failed     uint64
	WalkErrors uint64
	// ExcludedDirs counts directories skipped without being scanned.
	ExcludedDirs uint64
}

// Run executes the pipeline. Results are emitted on the returned channel and
// it is closed once all workers exit. The returned Stats pointer is updated
// as work progresses; callers should only read it after the channel closes.
func Run(ctx context.Context, cfg Config) (<-chan Result, *Stats) {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	results := make(chan Result, cfg.Workers*2)
	stats := &Stats{}

	jobs := make(chan Job, cfg.Workers*2)
	claims := &filesystem.Claims{}

	go func() {
		defer close(jobs)
		sidecarsOf := map[string][]processor.Sidecar{} // photo path -> sidecars
		isSidecar := map[string]bool{}
		filt := filter{includeHidden: cfg.IncludeHidden, exclude: cfg.Exclude}
		_ = filepath.WalkDir(cfg.Source, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				atomic.AddUint64(&stats.WalkErrors, 1)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case results <- Result{Src: path, Status: StatusWalkError, DryRun: cfg.DryRun, Err: err}:
				}
				return nil
			}
			rel, _ := filepath.Rel(cfg.Source, path)
			rel = filepath.ToSlash(rel)
			if skip, why := filt.skip(rel, d.IsDir()); skip {
				if !d.IsDir() {
					return nil
				}
				atomic.AddUint64(&stats.ExcludedDirs, 1)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case results <- Result{Src: path, Status: StatusExcludedDir, DryRun: cfg.DryRun, Reason: why}:
				}
				return filepath.SkipDir
			}
			if d.IsDir() {
				if !cfg.SkipSidecars {
					indexSidecars(path, rel, filt, sidecarsOf, isSidecar)
				}
				return nil
			}
			if isSidecar[path] {
				delete(isSidecar, path)
				return nil
			}
			if !processor.IsSupported(path) || (cfg.SkipVideos && processor.IsVideo(path)) {
				return nil
			}
			atomic.AddUint64(&stats.Discovered, 1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- Job{Path: path, Sidecars: sidecarsOf[path]}:
			}
			delete(sidecarsOf, path)
			return nil
		})
	}()

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				for _, res := range process(cfg, claims, job) {
					switch {
					case res.Status == StatusFailed:
						atomic.AddUint64(&stats.Failed, 1)
					case res.Status == StatusSkippedDuplicate:
						atomic.AddUint64(&stats.Skipped, 1)
					case res.SidecarOf != "":
						atomic.AddUint64(&stats.Sidecars, 1)
					case res.Status == StatusUnknownDate:
						atomic.AddUint64(&stats.Unknown, 1)
						atomic.AddUint64(&stats.Processed, 1)
					case res.Status == StatusOK:
						atomic.AddUint64(&stats.Processed, 1)
					}
					results <- res
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		if cfg.Operation == filesystem.OpMove && !cfg.DryRun {
			_ = filesystem.CleanupEmptyDirs(cfg.Source)
		}
		close(results)
	}()

	return results, stats
}

// indexSidecars records which files in dir (at rel under the source) are
// sidecars of which photos, ignoring files the filter skips. Errors are left
// for the walk itself to report.
func indexSidecars(dir, rel string, filt filter, sidecarsOf map[string][]processor.Sidecar, isSidecar map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if skip, _ := filt.skip(path.Join(rel, e.Name()), false); !e.IsDir() && !skip {
			names = append(names, e.Name())
		}
	}
	for photo, sidecars := range processor.PairSidecars(names) {
		sidecarsOf[filepath.Join(dir, photo)] = sidecars
		for _, sc := range sidecars {
			isSidecar[filepath.Join(dir, sc.Name)] = true
		}
	}
}

// process handles a job's file, then its sidecars once the file is in place.
func process(cfg Config, claims *filesystem.Claims, job Job) []Result {
	res := processFile(cfg, claims, job)
	out := []Result{res}
	if res.Status == StatusFailed {
		return out
	}
	return append(out, placeSidecars(cfg, claims, job, res)...)
}

// placeSidecars puts each sidecar next to where the photo landed (or, for a
// skipped duplicate, the copy already there), renamed to match it:
// IMG_1.HEIC -> IMG_1_2.HEIC carries IMG_1.mov -> IMG_1_2.mov. A different
// file already at a sidecar's target is reported, never overwritten.
func placeSidecars(cfg Config, claims *filesystem.Claims, job Job, photo Result) []Result {
	var out []Result
	stem := strings.TrimSuffix(photo.Dst, filepath.Ext(photo.Dst))
	for _, sc := range job.Sidecars {
		src := filepath.Join(filepath.Dir(job.Path), sc.Name)
		res := Result{Src: src, Dst: stem + sc.Suffix, Status: StatusOK, Source: photo.Source, DryRun: cfg.DryRun, SidecarOf: job.Path}
		reserve := filesystem.ReserveExact
		if cfg.DryRun {
			reserve = filesystem.ReserveExactReadOnly
		}
		reserved, duplicate, err := reserve(src, res.Dst, claims)
		switch {
		case err != nil:
			res.Status, res.Err = StatusFailed, err
		case duplicate:
			res.Status = StatusSkippedDuplicate
			if cfg.Operation == filesystem.OpMove && !cfg.DryRun {
				if rmErr := os.Remove(src); rmErr != nil {
					res.Status, res.Err = StatusFailed, fmt.Errorf("remove duplicate source: %w", rmErr)
				}
			}
		case !reserved:
			res.Status, res.Err = StatusFailed, fmt.Errorf("a different file already exists at %s", res.Dst)
		case !cfg.DryRun:
			if err := filesystem.Apply(cfg.Operation, src, res.Dst); err != nil {
				os.Remove(res.Dst)
				res.Status, res.Err = StatusFailed, err
			}
		}
		out = append(out, res)
	}
	return out
}

func processFile(cfg Config, claims *filesystem.Claims, job Job) Result {
	res := Result{Src: job.Path, DryRun: cfg.DryRun}

	pt, err := processor.ExtractTime(job.Path)
	var dst string
	if err != nil {
		dst = processor.UnknownDestination(cfg.Destination, job.Path)
		res.Source = processor.SourceUnknown
		res.Status = StatusUnknownDate
	} else {
		dst = processor.DestinationFor(cfg.Destination, job.Path, pt.When)
		res.Source = pt.Source
	}

	if cfg.DryRun {
		finalDst, skip, err := filesystem.ResolveCollisionReadOnly(job.Path, dst, claims)
		if err != nil {
			res.Status = StatusFailed
			res.Err = err
			return res
		}
		if skip {
			res.Dst = finalDst
			res.Status = StatusSkippedDuplicate
			return res
		}
		res.Dst = finalDst
		if res.Status == 0 {
			res.Status = StatusOK
		}
		return res
	}

	finalDst, skip, err := filesystem.ResolveCollision(job.Path, dst, claims)
	if err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res
	}
	if skip {
		res.Dst = finalDst
		res.Status = StatusSkippedDuplicate
		if cfg.Operation == filesystem.OpMove {
			// The move isn't complete until the source is gone.
			if rmErr := os.Remove(job.Path); rmErr != nil {
				res.Status = StatusFailed
				res.Err = fmt.Errorf("remove duplicate source: %w", rmErr)
			}
		}
		return res
	}

	res.Dst = finalDst

	if err := filesystem.Apply(cfg.Operation, job.Path, finalDst); err != nil {
		os.Remove(finalDst)
		res.Status = StatusFailed
		res.Err = err
		return res
	}
	if res.Status == 0 {
		res.Status = StatusOK
	}
	return res
}
