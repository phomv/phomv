package main

import (
	"github.com/spf13/cobra"
)

var (
	flagSrc           string
	flagDest          string
	flagDryRun        bool
	flagWorkers       int
	flagVerbose       bool
	flagNoVideos      bool
	flagNoSidecars    bool
	flagIncludeHidden bool
	flagExclude       []string
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "phomv",
		Short: "Organize photo libraries by EXIF date",
		Long: `phomv organizes photos and videos into a YYYY/YYYY_MM/YYYY_MM_DD
hierarchy based on EXIF (photos) or QuickTime (videos) metadata, with file
mtime as a fallback. Operations run concurrently
and support a dry-run mode for safe previews.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&flagSrc, "src", "s", "", "Source directory")
	root.PersistentFlags().StringVarP(&flagDest, "dest", "d", "", "Destination directory")
	root.PersistentFlags().BoolVarP(&flagDryRun, "dry-run", "n", false, "Simulate execution without modifying disk")
	root.PersistentFlags().IntVarP(&flagWorkers, "workers", "w", 4, "Number of concurrent workers")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable debug logging")
	root.PersistentFlags().BoolVar(&flagNoVideos, "no-videos", false, "Leave video files (.mp4, .mov, ...) in place")
	root.PersistentFlags().BoolVar(&flagNoSidecars, "no-sidecars", false, "Don't carry .xmp/.aae/Live Photo .mov files along with their photo")
	root.PersistentFlags().BoolVar(&flagIncludeHidden, "include-hidden", false, "Also scan dot-files/-folders and system folders (@eaDir, $RECYCLE.BIN, ...)")
	root.PersistentFlags().StringArrayVarP(&flagExclude, "exclude", "x", nil, "Skip files/folders whose name or path under --src matches this glob (repeatable)")

	root.AddCommand(newMoveCmd())
	root.AddCommand(newCopyCmd())
	root.AddCommand(newVersionCmd())
	return root
}
