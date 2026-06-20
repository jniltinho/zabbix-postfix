// Package offset reads and writes the pygtail offset file.
// The file format is two lines: inode number, then byte offset.
// This format is identical to pygtail.py v0.11.1, allowing zero-downtime
// migration from the Python tool.
package offset

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// File holds the inode and byte position saved between runs.
type File struct {
	Inode  uint64
	Offset int64
}

// Read parses an offset file written by pygtail or pygtail.py.
// Returns a zero File if path does not exist.
func Read(path string) (File, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return File{}, nil
	}
	if err != nil {
		return File{}, fmt.Errorf("reading offset file %q: %w", path, err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 2 {
		return File{}, fmt.Errorf("offset file %q: expected 2 lines, got %d", path, len(lines))
	}

	inode, err := strconv.ParseUint(strings.TrimSpace(lines[0]), 10, 64)
	if err != nil {
		return File{}, fmt.Errorf("offset file %q: parsing inode: %w", path, err)
	}

	off, err := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
	if err != nil {
		return File{}, fmt.Errorf("offset file %q: parsing offset: %w", path, err)
	}

	return File{Inode: inode, Offset: off}, nil
}

// Write saves inode and byte offset to path in pygtail-compatible format.
func Write(path string, inode uint64, offset int64) error {
	content := fmt.Sprintf("%d\n%d\n", inode, offset)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing offset file %q: %w", path, err)
	}
	return nil
}
