package main

import (
	"github.com/spf13/cobra"
)

var (
	flagSrc     string
	flagDest    string
	flagDryRun  bool
	flagWorkers int
	flagVerbose bool
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "phomv",
		Short: "Organize photo libraries by EXIF date",
		Long: `phomv organizes photos into a YYYY/MM/DD hierarchy based on EXIF
metadata, with file mtime as a fallback. Operations run concurrently and
support a dry-run mode for safe previews.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&flagSrc, "src", "s", "", "Source directory")
	root.PersistentFlags().StringVarP(&flagDest, "dest", "d", "", "Destination directory")
	root.PersistentFlags().BoolVarP(&flagDryRun, "dry-run", "n", false, "Simulate execution without modifying disk")
	root.PersistentFlags().IntVarP(&flagWorkers, "workers", "w", 4, "Number of concurrent workers")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable debug logging")

	root.AddCommand(newMoveCmd())
	root.AddCommand(newCopyCmd())
	root.AddCommand(newVersionCmd())
	return root
}
