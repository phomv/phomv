package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set via -ldflags at build time; defaults are usable for go run.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("phomv %s (commit %s, built %s)\n", version, commit, date)
		},
	}
}
