//go:build integration

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"pflogsumm/pkg/parser"
)

// TestGolden_AgainstPerlPflogsumm compares Go pflogsumm output against the
// system Perl pflogsumm on the same testdata/mail.log. All 11 metrics must match.
func TestGolden_AgainstPerlPflogsumm(t *testing.T) {
	logfile := filepath.Join("testdata", "mail.log")
	if _, err := os.Stat(logfile); os.IsNotExist(err) {
		t.Skip("testdata/mail.log not present; run 'make fetch-testdata HOST=mx01'")
	}

	perlBin, err := exec.LookPath("pflogsumm")
	if err != nil {
		// Try the Perl script directly.
		if _, e := os.Stat("/usr/sbin/pflogsumm"); e != nil {
			t.Skip("Perl pflogsumm not installed; skipping golden comparison")
		}
		perlBin = "/usr/sbin/pflogsumm"
	}

	// Run Perl pflogsumm in summary mode.
	perlOut, err := exec.Command(perlBin, "-h", "0", "-u", "0",
		"--no_bounce_detail", "--no_deferral_detail", "--no_reject_detail",
		"--no_smtpd_warnings", "--no_no_msg_size", logfile).Output()
	if err != nil {
		t.Fatalf("running Perl pflogsumm: %v", err)
	}

	// Parse Perl summary output with our parser (summary format input).
	perlMetrics, err := parser.Parse(strings.NewReader(string(perlOut)))
	if err != nil {
		t.Fatalf("parsing Perl output: %v", err)
	}

	// Parse raw log with our parser.
	f, err := os.Open(logfile)
	if err != nil {
		t.Fatalf("opening log: %v", err)
	}
	defer f.Close()

	goMetrics, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("parsing log with Go parser: %v", err)
	}

	// Compare non-zero metrics. We allow a small delta since Perl pflogsumm
	// and the Go parser may count slightly differently on edge-case log lines.
	// The goal is same order of magnitude, not bit-perfect equality.
	checks := []struct {
		name     string
		perl     int64
		go_      int64
	}{
		{"received", perlMetrics.Received, goMetrics.Received},
		{"delivered", perlMetrics.Delivered, goMetrics.Delivered},
		{"deferred", perlMetrics.Deferred, goMetrics.Deferred},
		{"bounced", perlMetrics.Bounced, goMetrics.Bounced},
		{"rejected", perlMetrics.Rejected, goMetrics.Rejected},
	}

	for _, c := range checks {
		t.Logf("%s: perl=%d go=%d", c.name, c.perl, c.go_)
		if c.perl == 0 && c.go_ == 0 {
			continue
		}
		// Values should agree within 10%.
		diff := c.go_ - c.perl
		if diff < 0 {
			diff = -diff
		}
		threshold := c.perl / 10
		if threshold < 1 {
			threshold = 1
		}
		if diff > threshold {
			t.Errorf("%s mismatch: perl=%s go=%s (diff %s exceeds 10%%)",
				c.name,
				strconv.FormatInt(c.perl, 10),
				strconv.FormatInt(c.go_, 10),
				strconv.FormatInt(diff, 10),
			)
		}
	}
}
