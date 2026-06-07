package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	if err := os.MkdirAll(flagDest, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	configureLogging()

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
		Source:      flagSrc,
		Destination: flagDest,
		Operation:   op,
		Workers:     flagWorkers,
		DryRun:      flagDryRun,
	}

	log.Info().
		Str("op", op.String()).
		Str("src", cfg.Source).
		Str("dest", cfg.Destination).
		Int("workers", cfg.Workers).
		Bool("dry_run", cfg.DryRun).
		Msg("starting")

	results, stats := worker.Run(ctx, cfg)
	for r := range results {
		logResult(r)
	}

	log.Info().
		Uint64("discovered", stats.Discovered).
		Uint64("processed", stats.Processed).
		Uint64("skipped", stats.Skipped).
		Uint64("unknown", stats.Unknown).
		Uint64("failed", stats.Failed).
		Msg("done")

	if stats.Failed > 0 {
		return fmt.Errorf("%d files failed", stats.Failed)
	}
	return nil
}

func configureLogging() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	level := zerolog.InfoLevel
	if flagVerbose {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func logResult(r worker.Result) {
	evt := log.Debug()
	switch r.Status {
	case worker.StatusFailed:
		evt = log.Error().Err(r.Err)
	case worker.StatusSkippedDuplicate:
		evt = log.Info().Str("status", "skip-duplicate")
	case worker.StatusUnknownDate:
		evt = log.Warn().Str("status", "unknown-date")
	}
	evt.
		Str("src", r.Src).
		Str("dst", r.Dst).
		Str("source", r.Source.String()).
		Bool("dry_run", r.DryRun).
		Msg("file")
}
