// Package cmd implements the pygtail CLI.
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"pygtail/internal/reader"
)

var (
	offsetFile    string
	noCopyTruncate bool
	version       = "0.1.0"
)

// NewRootCmd builds and returns the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "pygtail [flags] <logfile>",
		Short:   "Print log file lines that have not been read since the last run",
		Version: version,
		Args:    cobra.ExactArgs(1),
		RunE:    run,
	}

	root.Flags().StringVarP(&offsetFile, "offset-file", "o", "", "offset file path (default: <logfile>.offset)")
	root.Flags().BoolVar(&noCopyTruncate, "no-copytruncate", false, "disable copytruncate support (emit warning on shrink instead)")

	return root
}

func run(cmd *cobra.Command, args []string) error {
	logfile := args[0]

	opts := reader.DefaultOptions()
	opts.OffsetFile = offsetFile
	opts.CopyTruncate = !noCopyTruncate

	rc, currentInode, err := reader.Read(logfile, opts)
	if err != nil {
		return fmt.Errorf("reading %q: %w", logfile, err)
	}
	defer rc.Close()

	if _, err := io.Copy(os.Stdout, rc); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	of := offsetFile
	if of == "" {
		of = logfile + ".offset"
	}
	return reader.SaveOffset(logfile, of, currentInode)
}
