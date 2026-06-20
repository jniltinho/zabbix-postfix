package offset_test

import (
	"os"
	"path/filepath"
	"testing"

	"pygtail/internal/offset"
)

func TestWriteRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.offset")

	if err := offset.Write(path, 12345, 67890); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := offset.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Inode != 12345 {
		t.Errorf("Inode = %d, want 12345", got.Inode)
	}
	if got.Offset != 67890 {
		t.Errorf("Offset = %d, want 67890", got.Offset)
	}
}

func TestRead_Missing_ReturnsZero(t *testing.T) {
	got, err := offset.Read("/nonexistent/path/test.offset")
	if err != nil {
		t.Fatalf("Read on missing file returned error: %v", err)
	}
	if got.Inode != 0 || got.Offset != 0 {
		t.Errorf("expected zero File, got %+v", got)
	}
}

// TestRead_PythonFormat verifies the Go reader can parse an offset file
// written by pygtail.py v0.11.1 (same two-line format).
func TestRead_PythonFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pygtail_compat.offset")

	// Write in exactly the format pygtail.py uses.
	if err := os.WriteFile(path, []byte("98765\n43210\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	got, err := offset.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Inode != 98765 {
		t.Errorf("Inode = %d, want 98765", got.Inode)
	}
	if got.Offset != 43210 {
		t.Errorf("Offset = %d, want 43210", got.Offset)
	}
}

func TestRead_Malformed_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.offset")

	if err := os.WriteFile(path, []byte("not-a-number\n0\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	if _, err := offset.Read(path); err == nil {
		t.Error("expected error for malformed offset file, got nil")
	}
}
