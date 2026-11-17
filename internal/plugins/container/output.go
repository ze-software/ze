// Design: docs/architecture/container/runtime.md -- the output of a container
// Related: output_linux.go -- the two pipes and their readers that fill a buffer
// Related: supervise.go -- the supervisor, which keeps one buffer per configured container
// Related: doctor.go -- the stopped check, whose finding quotes the newest rows

package container

import (
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// outputStream names the standard stream a row of output came from. Zero is
// reserved and names no stream, so a row nobody filled never reads as one.
type outputStream uint8

const (
	outputStreamUnspecified outputStream = iota
	// outputStreamStdout: the entrypoint's standard output.
	outputStreamStdout
	// outputStreamStderr: the entrypoint's standard error.
	outputStreamStderr
)

// String answers the stream's name, as a row shows it.
func (s outputStream) String() string {
	switch s {
	case outputStreamStdout:
		return "stdout"
	case outputStreamStderr:
		return "stderr"
	case outputStreamUnspecified:
		return textUnspecified
	}
	return textUnspecified
}

// The bounds of a container's output (spec-container-1-runtime AC-16, and its
// security row on operator text). A buffer holds the newest outputBufferBytes
// of rows; each row costs its line and outputRowOverheadBytes, so a stream of
// empty lines is bounded too. A line longer than outputLineBytesMax is cut
// into rows of at most that length, at a character boundary, so no byte is
// lost to the cut and no row outgrows the bound.
const (
	outputBufferBytes      = 1 << 20
	outputRowOverheadBytes = 64
	outputLineBytesMax     = 4096
)

// The bounds of the output a finding quotes: the newest rows, each cut at a
// character boundary, so a finding stays one readable line.
const (
	endedOutputRows         = 5
	endedOutputLineBytesMax = 200
)

// outputRow is one line a container wrote: its sequence number, rising by one
// for each row of the container from 1, the time it was read, its stream and
// the line, without its line end and stripped of control characters.
type outputRow struct {
	sequence uint64
	time     time.Time
	stream   outputStream
	line     string
}

// outputRead answers a read after a sequence number: the rows held after it,
// oldest first, and how many rows after it the buffer no longer holds,
// because newer rows pushed them out. A reader that fell behind learns how
// much it missed, and the writer never waited for it.
type outputRead struct {
	rows    []outputRow
	dropped uint64
}

// outputBuffer holds the newest rows a container wrote, within
// outputBufferBytes. It lives in memory only: the output of a removed
// container or of a previous daemon run is gone. A write never waits for a
// reader: the oldest rows are dropped to make room. Safe for concurrent use.
type outputBuffer struct {
	mu sync.Mutex
	// rows holds the rows, oldest first; bytes is their cost against the
	// bound; next is the sequence number of the next row.
	rows  []outputRow
	bytes int
	next  uint64
}

// newOutputBuffer answers an empty buffer, whose first row takes sequence 1.
func newOutputBuffer() *outputBuffer {
	return &outputBuffer{next: 1}
}

// add appends a row of stream, read now, and drops the oldest rows past the
// bound. line MUST already be cut and stripped (outputText).
func (b *outputBuffer) add(stream outputStream, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rows = append(b.rows, outputRow{sequence: b.next, time: time.Now(), stream: stream, line: line})
	b.next++
	b.bytes += rowCost(line)
	for b.bytes > outputBufferBytes {
		b.bytes -= rowCost(b.rows[0].line)
		// The dropped row's line is released for the collector, which a
		// reslice alone would keep reachable.
		b.rows[0] = outputRow{}
		b.rows = b.rows[1:]
	}
}

// rowCost is what one row counts against the bound.
func rowCost(line string) int {
	return len(line) + outputRowOverheadBytes
}

// after answers the rows held whose sequence number is above sequence, and
// how many rows above it are no longer held. 0 reads from the first row.
func (b *outputBuffer) after(sequence uint64) outputRead {
	b.mu.Lock()
	defer b.mu.Unlock()
	oldest := b.next - uint64(len(b.rows))
	var read outputRead
	if sequence+1 < oldest {
		read.dropped = oldest - sequence - 1
	}
	for i := range b.rows {
		if b.rows[i].sequence > sequence {
			read.rows = append(read.rows, b.rows[i:]...)
			break
		}
	}
	return read
}

// newest answers the newest count rows held, oldest first.
func (b *outputBuffer) newest(count int) []outputRow {
	b.mu.Lock()
	defer b.mu.Unlock()
	start := max(len(b.rows)-count, 0)
	return append([]outputRow(nil), b.rows[start:]...)
}

// outputText answers a raw line as a row holds it: a byte sequence that is
// not UTF-8 becomes the replacement character, and every control character
// but the tab is stripped, the carriage return of a CRLF line end and an
// escape sequence's escape included, so a row cannot move a terminal's
// cursor or forge a line of Ze's log.
func outputText(raw []byte) string {
	var text strings.Builder
	text.Grow(len(raw))
	for len(raw) > 0 {
		char, size := utf8.DecodeRune(raw)
		raw = raw[size:]
		if char == '\t' {
			text.WriteRune(char)
			continue
		}
		if unicode.IsControl(char) {
			continue
		}
		text.WriteRune(char)
	}
	return text.String()
}

// runeCut answers where to cut a piece of a line that reached a length bound
// so it ends on a whole character: before a character the piece holds only
// the start of. The piece MUST be longer than utf8.UTFMax, so the cut is
// never 0. A piece that ends in bytes that start no character is cut whole:
// they are not text, and outputText replaces them.
func runeCut(piece []byte) int {
	for back := 1; back <= utf8.UTFMax; back++ {
		start := len(piece) - back
		if !utf8.RuneStart(piece[start]) {
			continue
		}
		if utf8.FullRune(piece[start:]) {
			return len(piece)
		}
		return start
	}
	return len(piece)
}

// outputSet holds the output buffer of each configured container, by name.
// A container keeps its buffer across its starts, so its output reads on
// over a restart, and loses it when it leaves the configuration. It is
// bounded by the configured containers. Safe for concurrent use.
type outputSet struct {
	mu      sync.Mutex
	buffers map[string]*outputBuffer
}

// of answers the buffer of the named container, made empty on first use.
func (o *outputSet) of(name string) *outputBuffer {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.buffers == nil {
		o.buffers = map[string]*outputBuffer{}
	}
	buffer, known := o.buffers[name]
	if !known {
		buffer = newOutputBuffer()
		o.buffers[name] = buffer
	}
	return buffer
}

// keepOnly drops the buffer of every container configured does not hold.
func (o *outputSet) keepOnly(configured map[string]instanceConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for name := range o.buffers {
		if _, kept := configured[name]; !kept {
			delete(o.buffers, name)
		}
	}
}

// endedCause answers the cause a stopped finding and its log line carry for
// a container that ended on its own with code: its exit status, then the
// newest rows it wrote, each with its stream, so the operator reads why it
// ended without another command.
func endedCause(code int, rows []outputRow) string {
	cause := "it exited with status " + strconv.Itoa(code)
	if len(rows) == 0 {
		return cause + " and wrote no output"
	}
	quoted := make([]string, 0, len(rows))
	for _, row := range rows {
		quoted = append(quoted, row.stream.String()+": "+quotedLine(row.line))
	}
	return cause + "; its last output: " + strings.Join(quoted, " | ")
}

// quotedLine answers line cut to endedOutputLineBytesMax at a character
// boundary, marked when it was cut.
func quotedLine(line string) string {
	if len(line) <= endedOutputLineBytesMax {
		return line
	}
	return line[:runeCut([]byte(line[:endedOutputLineBytesMax]))] + "..."
}
