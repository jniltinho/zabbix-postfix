//go:build integration

package main_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"pygtail/internal/reader"
)

// TestIntegration_RealMailLog reads from testdata/mail.log (populated via
// `make fetch-testdata HOST=mx01`) and verifies non-empty output.
func TestIntegration_RealMailLog(t *testing.T) {
	logfile := filepath.Join("testdata", "mail.log")
	if _, err := os.Stat(logfile); os.IsNotExist(err) {
		t.Skip("testdata/mail.log not present; run 'make fetch-testdata HOST=mx01'")
	}

	dir := t.TempDir()
	offFile := filepath.Join(dir, "test.offset")

	opts := reader.DefaultOptions()
	opts.OffsetFile = offFile

	rc, inode, err := reader.Read(logfile, opts)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty output from mail.log")
	}

	if err := reader.SaveOffset(logfile, offFile, inode); err != nil {
		t.Errorf("SaveOffset: %v", err)
	}

	t.Logf("Read %d bytes from %s", len(data), logfile)
}
