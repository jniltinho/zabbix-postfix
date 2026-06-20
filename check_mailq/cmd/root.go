// Package cmd implements the check_mailq CLI.
package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"check_mailq/internal/runner"
	"check_mailq/pkg/mailq"
)

const (
	stateOK       = 0
	stateWarning  = 1
	stateCritical = 2
	stateUnknown  = 3
)

// nagiosExit is returned by run() to signal a specific Nagios exit code without
// printing an error message (SilenceErrors suppresses cobra's output; main.go
// reads ExitCode() and calls os.Exit).
type nagiosExit struct{ code int }

func (e *nagiosExit) Error() string { return "" }
func (e *nagiosExit) ExitCode() int { return e.code }

var (
	mailqPath    string
	timeoutSecs  int
	warningFlag  int
	criticalFlag int
	warningDom   int
	criticalDom  int
	zabbixFlag   bool
	sudoFlag     bool
	verboseFlag  bool
	mtaFlag      string
	version      = "0.1.0"
)

// NewRootCmd builds and returns the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "check_mailq",
		Short:         "Check the number of messages in the Postfix mail queue",
		Version:       version,
		Args:          cobra.NoArgs,
		RunE:          run,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.Flags().StringVar(&mailqPath, "mailq-path", "mailq", "path to the mailq binary")
	root.Flags().IntVarP(&timeoutSecs, "timeout", "t", 15, "plugin timeout in seconds")
	root.Flags().IntVarP(&warningFlag, "warning", "w", 0, "min messages in queue to generate warning")
	root.Flags().IntVarP(&criticalFlag, "critical", "c", 0, "min messages in queue to generate critical alert (w < c)")
	root.Flags().IntVarP(&warningDom, "Warning", "W", 0, "min messages for same domain to generate warning (sendmail/qmail only)")
	root.Flags().IntVarP(&criticalDom, "Critical", "C", 0, "min messages for same domain to generate critical alert (sendmail/qmail only)")
	root.Flags().StringVarP(&mtaFlag, "mailserver", "M", "", "MTA type: postfix|sendmail|qmail|exim|nullmailer (default: autodetect)")
	root.Flags().BoolVarP(&sudoFlag, "sudo", "s", false, "use sudo to call the mailq command")
	root.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "debugging output")
	root.Flags().BoolVar(&zabbixFlag, "zabbix", false, "output raw count for Zabbix UserParameter (no thresholds, always exits 0)")

	return root
}

func run(cmd *cobra.Command, _ []string) error {
	timeout := time.Duration(timeoutSecs) * time.Second

	// -M / --mailserver: we only support postfix; warn and exit UNKNOWN for others.
	if mtaFlag != "" && mtaFlag != "postfix" {
		fmt.Printf("UNKNOWN: only 'postfix' is supported by this build (got %q)\n", mtaFlag)
		return &nagiosExit{stateUnknown}
	}

	if verboseFlag {
		fmt.Printf("Running: %s (sudo=%v, timeout=%s)\n", mailqPath, sudoFlag, timeout)
	}

	output, err := runner.Run(mailqPath, timeout, sudoFlag)
	if err != nil {
		if zabbixFlag {
			return err
		}
		fmt.Printf("UNKNOWN: %v\n", err)
		return &nagiosExit{stateUnknown}
	}

	if verboseFlag {
		fmt.Printf("mailq output:\n%s\n", output)
	}

	count, err := mailq.ParseOutput(output)
	if err != nil {
		if zabbixFlag {
			return err
		}
		fmt.Printf("UNKNOWN: parsing mailq output: %v\n", err)
		return &nagiosExit{stateUnknown}
	}

	if verboseFlag {
		fmt.Printf("msg_q = %d\n", count)
	}

	// --zabbix: raw count only, no thresholds.
	if zabbixFlag {
		fmt.Println(count)
		return nil
	}

	// Nagios mode: -w and -c are required.
	if !cmd.Flags().Changed("warning") || !cmd.Flags().Changed("critical") {
		fmt.Println("UNKNOWN: -w and -c are required")
		fmt.Printf("Usage: %s -w <warn> -c <crit> [-W <warn>] [-C <crit>] [-M <MTA>] [-t <timeout>] [-s] [-v]\n", cmd.Use)
		return &nagiosExit{stateUnknown}
	}
	if warningFlag >= criticalFlag {
		fmt.Println("UNKNOWN: warning (-w) must be less than critical (-c)")
		return &nagiosExit{stateUnknown}
	}

	// -W / -C are for sendmail/qmail domain-based thresholds; log if set under postfix.
	if verboseFlag && (cmd.Flags().Changed("Warning") || cmd.Flags().Changed("Critical")) {
		fmt.Println("Note: -W/-C domain thresholds are not applicable for postfix (ignored)")
	}

	state, msg := evaluate(count, warningFlag, criticalFlag)

	if verboseFlag {
		fmt.Printf("msg_q = %d warn=%d crit=%d\n", count, warningFlag, criticalFlag)
	}

	fmt.Printf("%s|unsent=%d;%d;%d;0\n", msg, count, warningFlag, criticalFlag)

	if state == stateOK {
		return nil
	}
	return &nagiosExit{state}
}

func evaluate(count, warn, crit int) (int, string) {
	switch {
	case count == 0:
		return stateOK, "OK: postfix mailq is empty"
	case count < warn:
		return stateOK, fmt.Sprintf("OK: postfix mailq (%d) is below threshold (%d/%d)", count, warn, crit)
	case count < crit:
		return stateWarning, fmt.Sprintf("WARNING: postfix mailq is %d (threshold w = %d)", count, warn)
	default:
		return stateCritical, fmt.Sprintf("CRITICAL: postfix mailq is %d (threshold c = %d)", count, crit)
	}
}
