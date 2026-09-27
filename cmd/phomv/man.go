package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newManCmd generates man pages for release packaging; it's hidden from help.
func newManCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			header := &doc.GenManHeader{Title: "PHOMV", Section: "1", Source: "phomv " + version}
			return doc.GenManTree(cmd.Root(), header, dir)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "manpages", "Output directory")
	return cmd
}
