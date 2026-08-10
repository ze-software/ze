// Design: docs/architecture/mrt.md — file writer with rotation

package mrt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const defaultBufSize = 1 << 20 // 1 MiB

// Writer writes MRT records to files with strftime-based filename rotation.
type Writer struct {
	pattern  string
	interval time.Duration
	bufSize  int

	mu       sync.Mutex
	sink     io.WriteCloser // *os.File for a real path; a nop-close stdout writer for "-"
	buf      *bufio.Writer
	curPath  string
	openedAt time.Time
}

// isStdout reports whether this Writer emits to stdout ("-") rather than a
// rotating file. stdout has no rotation and its Close does not close os.Stdout.
func (w *Writer) isStdout() bool { return cliio.IsStdin(w.pattern) }

// WriterOption configures a Writer.
type WriterOption func(*Writer)

// WithInterval sets the file rotation interval. Zero disables rotation.
func WithInterval(d time.Duration) WriterOption {
	return func(w *Writer) { w.interval = d }
}

// withBufSize sets the write buffer size in bytes.
func withBufSize(n int) WriterOption {
	return func(w *Writer) {
		if n > 0 {
			w.bufSize = n
		}
	}
}

// NewWriter creates a Writer that expands strftime codes in pattern for
// each file it opens. Records are buffered before flushing to disk.
func NewWriter(pattern string, opts ...WriterOption) *Writer {
	w := &Writer{
		pattern: pattern,
		bufSize: defaultBufSize,
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// Write writes a complete MRT record, rotating the file first if needed.
func (w *Writer) Write(record []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.needsRotation() {
		if err := w.rotateLocked(); err != nil {
			return err
		}
	}
	if err := w.ensureOpenLocked(); err != nil {
		return err
	}
	_, err := w.buf.Write(record)
	return err
}

// Flush flushes buffered data to the underlying file.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

// Close flushes and closes the current file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeLocked()
}

// Rotate forces a file rotation: flush, close, reopen with a new name.
func (w *Writer) Rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotateLocked()
}

func (w *Writer) flushLocked() error {
	if w.buf == nil {
		return nil
	}
	return w.buf.Flush()
}

func (w *Writer) closeLocked() error {
	err := w.flushLocked()
	if w.sink != nil {
		// A "-" sink is a nop-close over os.Stdout, so this leaves stdout open.
		if cerr := w.sink.Close(); err == nil {
			err = cerr
		}
		w.sink = nil
		w.buf = nil
		w.curPath = ""
	}
	return err
}

func (w *Writer) rotateLocked() error {
	if err := w.closeLocked(); err != nil {
		return err
	}
	return w.ensureOpenLocked()
}

func (w *Writer) needsRotation() bool {
	// stdout never rotates (no filename to re-expand); a strftime pattern with "-"
	// would be meaningless.
	if w.interval <= 0 || w.sink == nil || w.isStdout() {
		return false
	}
	return time.Since(w.openedAt) >= w.interval
}

func (w *Writer) ensureOpenLocked() error {
	if w.sink != nil {
		return nil
	}
	if w.isStdout() {
		// "-" writes all records to stdout, unrotated. cliio.Create yields a
		// nop-close writer, so Close never closes os.Stdout.
		wc, err := cliio.Create(w.pattern)
		if err != nil {
			return err
		}
		w.sink = wc
		w.buf = bufio.NewWriterSize(wc, w.bufSize)
		w.curPath = w.pattern
		w.openedAt = time.Now()
		return nil
	}
	path := expandPattern(w.pattern, time.Now())
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("mrt writer mkdir %s: %w", dir, err)
	}
	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("mrt writer open %s: %w", path, err)
	}
	w.sink = f
	w.buf = bufio.NewWriterSize(f, w.bufSize)
	w.curPath = path
	w.openedAt = time.Now()
	return nil
}

// expandPattern replaces strftime-style codes in pattern.
//
// Supported codes:
//
//	%Y  4-digit year
//	%m  2-digit month (01-12)
//	%d  2-digit day (01-31)
//	%H  2-digit hour (00-23)
//	%M  2-digit minute (00-59)
//	%S  2-digit second (00-59)
//	%s  unix timestamp
//	%N  literal %N (reserved for caller table-name substitution)
//	%%  literal %
func expandPattern(pattern string, t time.Time) string {
	var b textbuf.Buffer
	b.Grow(len(pattern) + 16)
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '%' || i+1 >= len(pattern) {
			b.Byte(pattern[i])
			continue
		}
		i++
		switch pattern[i] {
		case 'Y':
			writePadded(&b, t.Year(), 4)
		case 'm':
			writePadded(&b, int(t.Month()), 2)
		case 'd':
			writePadded(&b, t.Day(), 2)
		case 'H':
			writePadded(&b, t.Hour(), 2)
		case 'M':
			writePadded(&b, t.Minute(), 2)
		case 'S':
			writePadded(&b, t.Second(), 2)
		case 's':
			b.Int(t.Unix())
		case 'N':
			b.Str("%N")
		case '%':
			b.Byte('%')
		default:
			b.Byte('%')
			b.Byte(pattern[i])
		}
	}
	return b.String()
}

func writePadded(b *textbuf.Buffer, v, width int) {
	s := textbuf.StringInt(int64(v))
	for i := len(s); i < width; i++ {
		b.Byte('0')
	}
	b.Str(s)
}

// expandPatternTableName replaces %N in an already-expanded path with the
// given table name.
func expandPatternTableName(path, tableName string) string {
	return strings.ReplaceAll(path, "%N", tableName)
}
