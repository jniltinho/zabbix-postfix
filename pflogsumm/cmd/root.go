// Package cmd implements the pflogsumm CLI.
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"pflogsumm/internal/formatter"
	"pflogsumm/pkg/parser"
)

var (
	formatFlag = "keyvalue"
	version    = "0.1.0"
)

// NewRootCmd builds and returns the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "pflogsumm [flags] [logfile]",
		Short:   "Summarise Postfix mail.log and output metric counts",
		Version: version,
		Args:    cobra.MaximumNArgs(1),
		RunE:    run,
	}

	root.Flags().StringVar(&formatFlag, "format", "keyvalue", "output format: keyvalue, json, summary")

	// Compatibility flags from the Perl pflogsumm; silently ignored.
	// -h cannot be registered (cobra reserves it for --help); the shell script
	// must drop "-h 0" when switching to this binary.
	// -u IS registered so "-u 0" works without error.
	root.Flags().IntP("no-top-users", "u", 0, "compatibility flag (ignored)")
	_ = root.Flags().MarkHidden("no-top-users")

	// Integer compat flags (accept a numeric argument).
	for _, name := range []string{
		"detail", "bounce-detail", "deferral-detail", "reject-detail",
		"smtp-detail", "smtpd-warning-detail",
	} {
		root.Flags().Int(name, 0, "compatibility flag (ignored)")
		_ = root.Flags().MarkHidden(name)
	}

	// Boolean compat flags (no argument).
	for _, name := range []string{
		"no_bounce_detail", "no_deferral_detail", "no_reject_detail",
		"no_smtpd_warnings", "no_no_msg_size",
		"problems-first", "rej-add-from", "verbose-msg-detail",
		"zero-fill", "iso-date-time", "smtpd-stats", "mailq",
	} {
		root.Flags().Bool(name, false, "compatibility flag (ignored)")
		_ = root.Flags().MarkHidden(name)
	}

	// String-valued compat flags.
	for _, name := range []string{"syslog-name", "d"} {
		root.Flags().String(name, "", "compatibility flag (ignored)")
		_ = root.Flags().MarkHidden(name)
	}

	return root
}

func run(cmd *cobra.Command, args []string) error {
	var r io.Reader

	if len(args) == 1 {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening %q: %w", args[0], err)
		}
		defer f.Close()
		r = f
	} else {
		r = os.Stdin
	}

	metrics, err := parser.Parse(r)
	if err != nil {
		return fmt.Errorf("parsing log: %w", err)
	}

	fmt.Print(formatter.Format(metrics, formatFlag))
	return nil
}
