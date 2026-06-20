// Package reader implements incremental log file reading with offset tracking
// and log rotation detection, mirroring the behaviour of pygtail.py v0.11.1.
package reader

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"pygtail/internal/offset"
)

// Options controls reader behaviour.
type Options struct {
	// OffsetFile is the path where the read position is persisted.
	// Defaults to "<logfile>.offset".
	OffsetFile string
	// CopyTruncate enables detection of copytruncate-style log rotation
	// (same inode, file shrank). Enabled by default.
	CopyTruncate bool
}

// DefaultOptions returns Options with copytruncate enabled.
func DefaultOptions() Options {
	return Options{CopyTruncate: true}
}

// Read opens logfile, seeks to the last saved offset, and returns a reader
// over only the new content. The caller MUST close the returned io.ReadCloser.
// After reading, call SaveOffset to persist the new position.
func Read(logfile string, opts Options) (io.ReadCloser, uint64, error) {
	if opts.OffsetFile == "" {
		opts.OffsetFile = logfile + ".offset"
	}

	saved, err := offset.Read(opts.OffsetFile)
	if err != nil {
		return nil, 0, err
	}

	info, err := os.Stat(logfile)
	if err != nil {
		return nil, 0, fmt.Errorf("stat %q: %w", logfile, err)
	}
	currentInode := inode(info)
	currentSize := info.Size()

	var readers []io.Reader
	var closers []io.Closer

	addFile := func(path string, startOffset int64) error {
		f, err := openFile(path, startOffset)
		if err != nil {
			return err
		}
		readers = append(readers, f)
		closers = append(closers, f)
		return nil
	}

	switch {
	case saved.Inode == 0:
		// First run — read from the beginning.
		if err := addFile(logfile, 0); err != nil {
			return nil, 0, err
		}

	case saved.Inode != currentInode:
		// Inode changed: file was rotated. Find and drain the rotated file first.
		rotated := findRotated(logfile, saved.Inode)
		if rotated != "" {
			if err := addFile(rotated, saved.Offset); err != nil {
				return nil, 0, err
			}
		} else {
			fmt.Fprintf(os.Stderr, "pygtail: rotated file for inode %d not found; reading from start of %s\n", saved.Inode, logfile)
		}
		// Then read from the start of the new file.
		if err := addFile(logfile, 0); err != nil {
			return nil, 0, err
		}

	case opts.CopyTruncate && currentSize < saved.Offset:
		// Same inode but file shrank: copytruncate rotation.
		if err := addFile(logfile, 0); err != nil {
			return nil, 0, err
		}

	case !opts.CopyTruncate && currentSize < saved.Offset:
		fmt.Fprintf(os.Stderr, "pygtail: file %s shrank (expected >= %d bytes, got %d); copytruncate disabled\n",
			logfile, saved.Offset, currentSize)
		if err := addFile(logfile, 0); err != nil {
			return nil, 0, err
		}

	default:
		// Normal case: continue from saved offset.
		if err := addFile(logfile, saved.Offset); err != nil {
			return nil, 0, err
		}
	}

	multi := io.MultiReader(readers...)
	return &multiCloser{Reader: multi, closers: closers}, currentInode, nil
}

// SaveOffset writes the current position of logfile to the offset file.
func SaveOffset(logfile, offsetFile string, inode uint64) error {
	if offsetFile == "" {
		offsetFile = logfile + ".offset"
	}
	info, err := os.Stat(logfile)
	if err != nil {
		return fmt.Errorf("stat %q for offset: %w", logfile, err)
	}
	return offset.Write(offsetFile, inode, info.Size())
}

// openFile opens path (plain or .gz) and seeks to startOffset.
func openFile(path string, startOffset int64) (io.ReadCloser, error) {
	if strings.HasSuffix(path, ".gz") {
		return openGzip(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	if startOffset > 0 {
		if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
			f.Close()
			return nil, fmt.Errorf("seek %q to %d: %w", path, startOffset, err)
		}
	}
	return f, nil
}

// openGzip opens a gzip-compressed file and returns a reader from its start.
// Seeking inside gzip is not supported; callers always read from the beginning
// of compressed rotated files.
func openGzip(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open gz %q: %w", path, err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("gzip reader %q: %w", path, err)
	}
	return &gzipCloser{gz: gz, f: f}, nil
}

// findRotated searches for the rotated log file that matches savedInode.
// Search order mirrors pygtail.py exactly.
func findRotated(logfile string, savedInode uint64) string {
	dir := filepath.Dir(logfile)
	base := filepath.Base(logfile)

	candidates := []string{
		logfile + ".0",
		logfile + ".1",
		logfile + ".1.gz",
	}

	// dateext patterns
	datePatterns := []string{
		base + "-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]",
		base + "-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9].gz",
		base + "-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]",
		base + "-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9].gz",
		// TimedRotatingFileHandler
		base + ".[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]",
	}
	for _, pat := range datePatterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pat))
		candidates = append(candidates, matches...)
	}

	for _, c := range candidates {
		info, err := os.Stat(c)
		if err != nil {
			continue
		}
		if inode(info) == savedInode {
			return c
		}
	}
	return ""
}

// inode extracts the inode number from FileInfo on Linux/macOS.
func inode(info os.FileInfo) uint64 {
	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		return sys.Ino
	}
	return 0
}

// multiCloser combines a Reader with a list of Closers.
type multiCloser struct {
	io.Reader
	closers []io.Closer
}

func (m *multiCloser) Close() error {
	var last error
	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			last = err
		}
	}
	return last
}

// gzipCloser closes both the gzip.Reader and the underlying file.
type gzipCloser struct {
	gz *gzip.Reader
	f  *os.File
}

func (g *gzipCloser) Read(p []byte) (int, error) { return g.gz.Read(p) }

func (g *gzipCloser) Close() error {
	err := g.gz.Close()
	g.f.Close()
	return err
}
