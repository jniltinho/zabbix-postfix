// Package cmd implements the check_mailq CLI.
package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"check_mailq/internal/runner"
	"check_mailq/pkg/mailq"
)

var (
	mailqPath string
	timeout   time.Duration
	version   = "0.1.0"
)

// NewRootCmd builds and returns the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "check_mailq",
		Short:   "Count the number of messages in the Postfix mail queue",
		Version: version,
		Args:    cobra.NoArgs,
		RunE:    run,
	}

	root.Flags().StringVar(&mailqPath, "mailq-path", "mailq", "path to the mailq binary")
	root.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "subprocess timeout")

	return root
}

func run(_ *cobra.Command, _ []string) error {
	output, err := runner.Run(mailqPath, timeout)
	if err != nil {
		return err
	}

	count, err := mailq.ParseOutput(output)
	if err != nil {
		return fmt.Errorf("parsing mailq output: %w", err)
	}

	fmt.Println(count)
	return nil
}
