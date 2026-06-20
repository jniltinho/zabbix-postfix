// Package runner executes the mailq command as a subprocess.
package runner

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Run executes mailqPath with the given timeout and returns its combined output.
// Returns an error if mailq cannot be found, exits non-zero, or times out.
func Run(mailqPath string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, mailqPath).Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("mailq timed out after %s", timeout)
		}
		// mailq exits 1 on non-empty queue on some systems; treat output as valid.
		if len(out) > 0 {
			return string(out), nil
		}
		return "", fmt.Errorf("running %q: %w", mailqPath, err)
	}
	return string(out), nil
}
