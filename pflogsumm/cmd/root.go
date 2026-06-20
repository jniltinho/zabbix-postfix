// Package cmd implements the pflogsumm CLI.
package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"pflogsumm/internal/formatter"
	"pflogsumm/pkg/parser"
)

var (
	formatFlag  = "human"
	zabbixFlag  bool
	mailqFlag   bool
	lastFlag    string
	version     = "0.1.0"
)

// NewRootCmd builds and returns the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "pflogsumm [flags] [file1 [filen]]",
		Short:   "Summarise Postfix mail.log and output metric counts",
		Version: version,
		Args:    cobra.ArbitraryArgs,
		RunE:    run,
	}

	// Normalise flag names: underscores and dashes are interchangeable, matching
	// how Perl GetOptions treats them (e.g. --no_bounce_detail == --no-bounce-detail).
	root.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		return pflag.NormalizedName(strings.ReplaceAll(name, "_", "-"))
	})

	root.Flags().StringVar(&formatFlag, "format", "human", "output format: human, keyvalue, json, summary")
	root.Flags().BoolVar(&zabbixFlag, "zabbix", false, "output key=value metrics (for Zabbix monitoring)")
	root.Flags().BoolVar(&mailqFlag, "mailq", false, "append current mail queue after report")
	root.Flags().StringVarP(&lastFlag, "last", "l", "", "only count log lines from the last duration (e.g. 5m, 1h, 30s)")

	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		fmt.Print(`Summarise Postfix mail.log and output metric counts

Usage:
  pflogsumm [flags] [file1 [filen]]

usage: pflogsumm -[eq] [-d <today|yesterday>] [--detail <cnt>]
	[--bounce-detail <cnt>] [--deferral-detail <cnt>]
	[-h <cnt>] [-i|--ignore-case] [--iso-date-time] [--mailq]
	[-m|--uucp-mung] [--no-no-msg-size] [--problems-first]
	[--rej-add-from] [--reject-detail <cnt>] [--smtp-detail <cnt>]
	[--smtpd-stats] [--smtpd-warning-detail <cnt>]
	[--syslog-name=string] [-u <cnt>] [--verbose-msg-detail]
	[--verp-mung[=<n>]] [--zero-fill] [file1 [filen]]

       pflogsumm --[version|help]

Go-specific flags:
  --format string   output format: human, keyvalue, json, summary (default "human")
  --zabbix          output key=value metrics for Zabbix (overrides --format)
  --mailq           append current mail queue after report
  --last duration   only count lines from the last N minutes/hours (e.g. 5m, 1h)
`)
	})

	// -d today|yesterday: filter log lines to a specific day (implemented).
	root.Flags().StringP("day", "d", "", "limit report to today or yesterday")
	_ = root.Flags().MarkHidden("day")

	// Integer flags with shorthands.
	// -h conflicts with cobra's built-in --help; register long-only --h so
	// "--h 0" works but "-h 0" cannot be supported without patching cobra.
	root.Flags().IntP("no-top-users", "u", 0, "compatibility flag (ignored)")
	_ = root.Flags().MarkHidden("no-top-users")
	root.Flags().Int("h", 0, "compatibility flag (ignored)")
	_ = root.Flags().MarkHidden("h")

	// Boolean flags with shorthands.
	for _, spec := range []struct{ long, short string }{
		{"extended", "e"},
		{"quiet", "q"},
		{"ignore-case", "i"},
		{"uucp-mung", "m"},
	} {
		root.Flags().BoolP(spec.long, spec.short, false, "compatibility flag (ignored)")
		_ = root.Flags().MarkHidden(spec.long)
	}

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
		"no-bounce-detail", "no-deferral-detail", "no-reject-detail",
		"no-smtpd-warnings", "no-no-msg-size",
		"problems-first", "rej-add-from", "verbose-msg-detail",
		"zero-fill", "iso-date-time", "smtpd-stats",
	} {
		root.Flags().Bool(name, false, "compatibility flag (ignored)")
		_ = root.Flags().MarkHidden(name)
	}

	// --verp-mung accepts an optional integer: --verp-mung or --verp-mung=2.
	// NoOptDefVal makes the value optional, matching Perl's ":i" behaviour.
	root.Flags().String("verp-mung", "", "compatibility flag (ignored)")
	_ = root.Flags().MarkHidden("verp-mung")
	root.Flags().Lookup("verp-mung").NoOptDefVal = "1"

	// String-valued compat flags.
	root.Flags().String("syslog-name", "", "compatibility flag (ignored)")
	_ = root.Flags().MarkHidden("syslog-name")

	return root
}

func run(cmd *cobra.Command, args []string) error {
	day, _ := cmd.Flags().GetString("day")

	var r io.Reader
	if len(args) == 0 {
		r = os.Stdin
	} else {
		var readers []io.Reader
		var closers []io.Closer
		for _, path := range args {
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("opening %q: %w", path, err)
			}
			readers = append(readers, f)
			closers = append(closers, f)
		}
		defer func() {
			for _, c := range closers {
				c.Close()
			}
		}()
		r = io.MultiReader(readers...)
	}

	var metrics parser.Metrics
	var err error
	if lastFlag != "" {
		d, err := time.ParseDuration(lastFlag)
		if err != nil {
			return fmt.Errorf("--last: invalid duration %q (use e.g. 5m, 1h, 30s)", lastFlag)
		}
		metrics, err = parser.ParseLastN(r, d)
	} else {
		metrics, err = parser.ParseFiltered(r, day)
	}
	if err != nil {
		return fmt.Errorf("parsing log: %w", err)
	}

	// --zabbix overrides --format to key=value output.
	if zabbixFlag {
		fmt.Print(formatter.Format(metrics, "keyvalue"))
		return nil
	}

	// Human format: derive the day label for the header.
	if formatFlag == "human" {
		dayLabel := ""
		if day != "" {
			t := time.Now()
			if day == "yesterday" {
				t = t.AddDate(0, 0, -1)
			}
			dayLabel = fmt.Sprintf("%s %d", t.Format("Jan"), t.Day())
		}
		report := formatter.FormatHuman(metrics, dayLabel)
		fmt.Print(report)

		if mailqFlag {
			appendMailQ()
		}
		return nil
	}

	fmt.Print(formatter.Format(metrics, formatFlag))

	if mailqFlag && formatFlag == "human" {
		appendMailQ()
	}
	return nil
}

// appendMailQ runs `mailq` and prints the output with a section header.
func appendMailQ() {
	out, err := exec.Command("mailq").Output()
	if err != nil {
		// mailq may not be available; print a note but don't fail.
		fmt.Println("\nCurrent Mail Queue")
		fmt.Println("------------------")
		fmt.Printf("(mailq not available: %v)\n", err)
		return
	}
	fmt.Println("\nCurrent Mail Queue")
	fmt.Println("------------------")
	fmt.Print(string(out))
}
