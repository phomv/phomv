package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/phomv/phomv/internal/filesystem"
	"github.com/phomv/phomv/internal/worker"
)

func newMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move",
		Short: "Organize photos by moving them",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOp(filesystem.OpMove)
		},
	}
}

func newCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "copy",
		Short: "Organize photos by copying them (safer)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOp(filesystem.OpCopy)
		},
	}
}

func runOp(op filesystem.Operation) error {
	if flagSrc == "" || flagDest == "" {
		return errors.New("--src and --dest are required")
	}
	if info, err := os.Stat(flagSrc); err != nil || !info.IsDir() {
		return fmt.Errorf("source %q is not a directory", flagSrc)
	}
	if err := filesystem.CheckOverlap(flagSrc, flagDest); err != nil {
		return err
	}
	if err := worker.ValidateExcludes(flagExclude); err != nil {
		return err
	}
	if err := os.MkdirAll(flagDest, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	// On a terminal, a live counter shares stderr with the log.
	var prog *progress
	var logOut io.Writer = os.Stderr
	if isTerminal(os.Stderr) {
		prog = &progress{out: os.Stderr}
		logOut = prog
	}
	configureLogging(logOut)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Warn().Msg("interrupt received, shutting down")
		cancel()
	}()

	cfg := worker.Config{
		Source:        flagSrc,
		Destination:   flagDest,
		Operation:     op,
		Workers:       flagWorkers,
		DryRun:        flagDryRun,
		SkipVideos:    flagNoVideos,
		SkipSidecars:  flagNoSidecars,
		IncludeHidden: flagIncludeHidden,
		Exclude:       flagExclude,
	}

	log.Info().
		Str("op", op.String()).
		Str("src", cfg.Source).
		Str("dest", cfg.Destination).
		Int("workers", cfg.Workers).
		Bool("dry_run", cfg.DryRun).
		Bool("videos", !cfg.SkipVideos).
		Bool("sidecars", !cfg.SkipSidecars).
		Bool("include_hidden", cfg.IncludeHidden).
		Strs("exclude", cfg.Exclude).
		Msg("starting")

	results, stats := worker.Run(ctx, cfg)
	var done atomic.Uint64
	stopProgress := reportProgress(prog, &done, stats)
	for r := range results {
		if countsAsDone(r) {
			done.Add(1)
		}
		logResult(r)
	}
	stopProgress()

	log.Info().
		Uint64("discovered", stats.Discovered).
		Uint64("processed", stats.Processed).
		Uint64("skipped", stats.Skipped).
		Uint64("unknown", stats.Unknown).
		Uint64("sidecars", stats.Sidecars).
		Uint64("failed", stats.Failed).
		Uint64("walk_errors", stats.WalkErrors).
		Uint64("excluded_dirs", stats.ExcludedDirs).
		Msg("done")

	var errs []error
	if stats.Failed > 0 {
		errs = append(errs, fmt.Errorf("%d files failed", stats.Failed))
	}
	if stats.WalkErrors > 0 {
		errs = append(errs, fmt.Errorf("%d paths could not be read and were not scanned", stats.WalkErrors))
	}
	return errors.Join(errs...)
}

// reportProgress shows done/found totals until the returned func is called:
// redrawn often on a terminal (prog non-nil), otherwise logged every 10s so
// long unattended runs still show signs of life.
func reportProgress(prog *progress, done *atomic.Uint64, stats *worker.Stats) (stop func()) {
	interval := 10 * time.Second
	if prog != nil {
		interval = 200 * time.Millisecond
	}
	quit, exited := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(exited)
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-quit:
				return
			case <-tick.C:
				d, found := done.Load(), atomic.LoadUint64(&stats.Discovered)
				if prog != nil {
					prog.update(d, found)
				} else {
					log.Info().Uint64("done", d).Uint64("found", found).Msg("progress")
				}
			}
		}
	}()
	return func() {
		close(quit)
		<-exited
		if prog != nil {
			prog.finish()
		}
	}
}

// countsAsDone reports whether r finishes one of the files counted in
// Stats.Discovered (sidecars, skipped folders and walk errors aren't).
func countsAsDone(r worker.Result) bool {
	switch r.Status {
	case worker.StatusWalkError, worker.StatusExcludedDir:
		return false
	}
	return r.SidecarOf == ""
}

func configureLogging(out io.Writer) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	level := zerolog.InfoLevel
	if flagVerbose {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: out})
}

func logResult(r worker.Result) {
	if r.Status == worker.StatusExcludedDir {
		log.Debug().Str("path", r.Src).Str("reason", r.Reason).Msg("skipped directory")
		return
	}
	if r.Status == worker.StatusWalkError {
		log.Warn().Err(r.Err).Str("path", r.Src).Msg("unreadable, not scanned")
		return
	}
	evt := log.Debug()
	switch r.Status {
	case worker.StatusFailed:
		evt = log.Error().Err(r.Err)
	case worker.StatusSkippedDuplicate:
		evt = log.Info().Str("status", "skip-duplicate")
	case worker.StatusUnknownDate:
		evt = log.Warn().Str("status", "unknown-date")
	}
	if r.SidecarOf != "" {
		evt = evt.Str("sidecar_of", r.SidecarOf)
	}
	evt.
		Str("src", r.Src).
		Str("dst", r.Dst).
		Str("source", r.Source.String()).
		Bool("dry_run", r.DryRun).
		Msg("file")
}
