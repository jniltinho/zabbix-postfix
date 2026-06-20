package mailq_test

import (
	"os"
	"testing"
	"time"

	"pflogsumm/internal/mailq"
)

func TestRun_MissingBinary_ReturnsError(t *testing.T) {
	_, err := mailq.Run("/nonexistent/mailq-binary", 5*time.Second, false)
	if err == nil {
		t.Error("expected error for missing binary, got nil")
	}
}

func TestRun_Timeout_ReturnsError(t *testing.T) {
	if _, err := os.Stat("/bin/sleep"); err != nil {
		t.Skip("/bin/sleep not available")
	}
	_, err := mailq.Run("/bin/sleep 10", 100*time.Millisecond, false)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestRun_EchoOutput_ReturnsStdout(t *testing.T) {
	if _, err := os.Stat("/bin/echo"); err != nil {
		t.Skip("/bin/echo not available")
	}
	out, err := mailq.Run("/bin/echo", 5*time.Second, false)
	if err != nil {
		t.Fatalf("Run with echo: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output from echo")
	}
}
