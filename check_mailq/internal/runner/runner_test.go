package runner_test

import (
	"os"
	"testing"
	"time"

	"check_mailq/internal/runner"
)

func TestRun_MissingBinary_ReturnsError(t *testing.T) {
	_, err := runner.Run("/nonexistent/mailq-binary", 5*time.Second, false)
	if err == nil {
		t.Error("expected error for missing binary, got nil")
	}
}

func TestRun_Timeout_ReturnsError(t *testing.T) {
	// Use `sleep 10` to simulate a hanging mailq.
	sleepBin, err := os.Stat("/bin/sleep")
	if err != nil || sleepBin == nil {
		t.Skip("/bin/sleep not available")
	}
	_, err = runner.Run("/bin/sleep 10", 100*time.Millisecond, false)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestRun_EchoOutput_ReturnsStdout(t *testing.T) {
	// Use `/bin/echo` to simulate a mailq that returns known output.
	echoBin := "/bin/echo"
	if _, err := os.Stat(echoBin); err != nil {
		t.Skip("/bin/echo not available")
	}
	// We can't easily test the full pipeline without a real mailq,
	// but we verify that runner.Run returns stdout from a subprocess.
	out, err := runner.Run(echoBin, 5*time.Second, false)
	if err != nil {
		t.Fatalf("Run with echo: %v", err)
	}
	// echo with no args outputs a newline
	if len(out) == 0 {
		t.Error("expected non-empty output from echo")
	}
}
