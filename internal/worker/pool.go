// Package worker provides a concurrent file-processing pipeline.
//
// Discovery walks the source tree and emits Jobs; a fixed-size worker pool
// consumes them and emits Results. Progress events are exposed on a channel
// so a CLI or future GUI can render counters without coupling to the engine.
package worker

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/chinny/phomv/internal/filesystem"
	"github.com/chinny/phomv/internal/processor"
)

// Job is a single file slated for processing.
type Job struct {
	Path string
}

// Status describes the outcome for one job.
type Status int

const (
	StatusOK Status = iota
	StatusSkippedDuplicate
	StatusUnknownDate
	StatusFailed
)

// Result is the outcome of processing one Job.
type Result struct {
	Src     string
	Dst     string
	Status  Status
	Source  processor.TimeSource
	DryRun  bool
	Err     error
}

// Config controls a Run invocation.
type Config struct {
	Source      string
	Destination string
	Operation   filesystem.Operation
	Workers     int
	DryRun      bool
}

// Stats reports counters after Run completes.
type Stats struct {
	Discovered uint64
	Processed  uint64
	Skipped    uint64
	Unknown    uint64
	Failed     uint64
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

	go func() {
		defer close(jobs)
		_ = filepath.Walk(cfg.Source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if !processor.IsSupported(path) {
				return nil
			}
			atomic.AddUint64(&stats.Discovered, 1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- Job{Path: path}:
			}
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
				res := process(cfg, job)
				switch res.Status {
				case StatusOK:
					atomic.AddUint64(&stats.Processed, 1)
				case StatusSkippedDuplicate:
					atomic.AddUint64(&stats.Skipped, 1)
				case StatusUnknownDate:
					atomic.AddUint64(&stats.Unknown, 1)
					atomic.AddUint64(&stats.Processed, 1)
				case StatusFailed:
					atomic.AddUint64(&stats.Failed, 1)
				}
				results <- res
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

func process(cfg Config, job Job) Result {
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

	finalDst, skip, err := filesystem.ResolveCollision(job.Path, dst)
	if err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res
	}
	if skip {
		res.Dst = dst
		res.Status = StatusSkippedDuplicate
		if cfg.Operation == filesystem.OpMove && !cfg.DryRun {
			if rmErr := os.Remove(job.Path); rmErr != nil {
				res.Err = rmErr
			}
		}
		return res
	}

	res.Dst = finalDst
	if cfg.DryRun {
		if res.Status == 0 {
			res.Status = StatusOK
		}
		return res
	}

	if err := filesystem.Apply(cfg.Operation, job.Path, finalDst); err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res
	}
	if res.Status == 0 {
		res.Status = StatusOK
	}
	return res
}
