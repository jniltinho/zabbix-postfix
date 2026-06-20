package mailq_test

import (
	"os"
	"path/filepath"
	"testing"

	"check_mailq/pkg/mailq"
)

func TestParseOutput_EmptyQueue(t *testing.T) {
	got, err := mailq.ParseOutput("Mail queue is empty\n")
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}
	if got != 0 {
		t.Errorf("empty queue: got %d, want 0", got)
	}
}

func TestParseOutput_EmptyString(t *testing.T) {
	got, err := mailq.ParseOutput("")
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}
	if got != 0 {
		t.Errorf("empty string: got %d, want 0", got)
	}
}

func TestParseOutput_NonEmptyQueue_CountsHexIDs(t *testing.T) {
	// Each queue entry line starts with a hex queue ID (uppercase hex + digits).
	input := `A1B2C3D4E5F6      2048 Tue Jun 18 10:00:00  sender@example.com
                                         recipient@example.com

B2C3D4E5F6G7      1024 Tue Jun 18 10:01:00  other@example.com
                                         dest@example.com

-- 2 Kbytes in 2 Requests.
`
	got, err := mailq.ParseOutput(input)
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}
	if got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

func TestParseOutput_MatchesGrepPipeline_EmptyFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("../../testdata", "mailq_output_empty.txt"))
	if err != nil {
		t.Skip("fixture not found")
	}
	got, err := mailq.ParseOutput(string(data))
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}
	if got != 0 {
		t.Errorf("empty fixture: got %d, want 0", got)
	}
}

func TestParseOutput_MatchesGrepPipeline_NonEmptyFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("../../testdata", "mailq_output_nonempty.txt"))
	if err != nil {
		t.Skip("fixture not found")
	}
	got, err := mailq.ParseOutput(string(data))
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}
	// The fixture has 3 queue entries (lines starting with hex ID).
	if got != 3 {
		t.Errorf("nonempty fixture: got %d, want 3", got)
	}
}

func TestParseOutput_LowercaseNotCounted(t *testing.T) {
	// Lines starting with lowercase or punctuation are not queue IDs.
	input := "-- some status line\n(some continuation)\n   indented\n"
	got, err := mailq.ParseOutput(input)
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}
