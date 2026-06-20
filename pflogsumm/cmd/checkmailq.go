package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"pflogsumm/internal/mailq"
)

const (
	stateOK       = 0
	stateWarning  = 1
	stateCritical = 2
	stateUnknown  = 3
)

// nagiosExit signals a Nagios exit code without printing a cobra error.
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
	mailqZabbix  bool
	sudoFlag     bool
	verboseFlag  bool
	mtaFlag      string
)

// NewCheckMailqCmd returns the check-mailq subcommand.
func NewCheckMailqCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "check-mailq",
		Short:         "Check the number of messages in the Postfix mail queue",
		Args:          cobra.NoArgs,
		RunE:          runCheckMailq,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.Flags().StringVar(&mailqPath, "mailq-path", "mailq", "path to the mailq binary")
	cmd.Flags().IntVarP(&timeoutSecs, "timeout", "t", 15, "plugin timeout in seconds")
	cmd.Flags().IntVarP(&warningFlag, "warning", "w", 0, "min messages in queue to generate warning")
	cmd.Flags().IntVarP(&criticalFlag, "critical", "c", 0, "min messages in queue to generate critical alert (w < c)")
	cmd.Flags().IntVarP(&warningDom, "Warning", "W", 0, "min messages for same domain to generate warning")
	cmd.Flags().IntVarP(&criticalDom, "Critical", "C", 0, "min messages for same domain to generate critical alert")
	cmd.Flags().StringVarP(&mtaFlag, "mailserver", "M", "", "MTA type: postfix|sendmail|qmail|exim (default: autodetect)")
	cmd.Flags().BoolVarP(&sudoFlag, "sudo", "s", false, "use sudo to call the mailq command")
	cmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "debugging output")
	cmd.Flags().BoolVar(&mailqZabbix, "zabbix", false, "output raw count for Zabbix UserParameter (always exits 0)")

	return cmd
}

func runCheckMailq(cmd *cobra.Command, _ []string) error {
	timeout := time.Duration(timeoutSecs) * time.Second

	if mtaFlag != "" && mtaFlag != "postfix" {
		fmt.Printf("UNKNOWN: only 'postfix' is supported by this build (got %q)\n", mtaFlag)
		return &nagiosExit{stateUnknown}
	}

	if verboseFlag {
		fmt.Printf("Running: %s (sudo=%v, timeout=%s)\n", mailqPath, sudoFlag, timeout)
	}

	output, err := mailq.Run(mailqPath, timeout, sudoFlag)
	if err != nil {
		if mailqZabbix {
			return err
		}
		fmt.Printf("UNKNOWN: %v\n", err)
		return &nagiosExit{stateUnknown}
	}

	count, err := mailq.ParseOutput(output)
	if err != nil {
		if mailqZabbix {
			return err
		}
		fmt.Printf("UNKNOWN: parsing mailq output: %v\n", err)
		return &nagiosExit{stateUnknown}
	}

	if verboseFlag {
		fmt.Printf("msg_q = %d\n", count)
	}

	if mailqZabbix {
		fmt.Println(count)
		return nil
	}

	if !cmd.Flags().Changed("warning") || !cmd.Flags().Changed("critical") {
		fmt.Println("UNKNOWN: -w and -c are required")
		fmt.Printf("Usage: pflogsumm check-mailq -w <warn> -c <crit> [-t <timeout>] [-s] [-v]\n")
		return &nagiosExit{stateUnknown}
	}
	if warningFlag >= criticalFlag {
		fmt.Println("UNKNOWN: warning (-w) must be less than critical (-c)")
		return &nagiosExit{stateUnknown}
	}

	state, msg := evaluateMailq(count, warningFlag, criticalFlag)
	fmt.Printf("%s|unsent=%d;%d;%d;0\n", msg, count, warningFlag, criticalFlag)
	if state == stateOK {
		return nil
	}
	return &nagiosExit{state}
}

func evaluateMailq(count, warn, crit int) (int, string) {
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
