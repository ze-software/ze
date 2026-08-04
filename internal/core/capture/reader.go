// Design: plan/spec-improve-3-event-replay.md -- capture decoder for the replay harness
// Overview: capture.go -- the format this decoder reads

package capture

import (
	"bufio"
	"encoding/json"
	"io"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// MaxLineLen bounds one capture line. The largest legitimate line is a 65535-byte
// extended BGP message: base64 grows it to 87380 bytes, and the surrounding JSON
// adds under 200. 256 KiB leaves generous headroom while still refusing a file
// that would otherwise be read into unbounded memory.
const MaxLineLen = 256 * 1024

// Reader decodes a capture file. It validates the header before returning, so a
// caller that gets a Reader knows the schema version is one it can read.
//
// Every error names the line it came from, because the first thing an operator
// needs from a damaged capture is where it went wrong.
type Reader struct {
	sc      *bufio.Scanner
	line    int
	lastSeq uint64
	done    bool
}

// NewReader validates the header line and returns a Reader positioned at the
// first event.
func NewReader(r io.Reader) (*Reader, Header, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineLen)

	var hdr Header
	var b textbuf.Buffer
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, hdr, lineErr(1, ErrBadHeader, b.Err(err).String())
		}
		return nil, hdr, lineErr(1, ErrNoHeader, "the file is empty")
	}
	if err := json.Unmarshal(sc.Bytes(), &hdr); err != nil {
		return nil, hdr, lineErr(1, ErrBadHeader, b.Err(err).String())
	}
	if hdr.Format != Format {
		detail := b.Str("format is ").Quoted(hdr.Format).Str(", want ").Quoted(Format).String()
		return nil, hdr, lineErr(1, ErrBadHeader, detail)
	}
	if hdr.Version != Version {
		detail := b.Str("file is version ").Int(int64(hdr.Version)).
			Str(", this build reads version ").Int(int64(Version)).String()
		return nil, hdr, lineErr(1, ErrUnsupportedVersion, detail)
	}
	return &Reader{sc: sc, line: 1}, hdr, nil
}

// Next returns the next event, or ErrEndOfStream when the file is exhausted.
//
// A blank line is skipped: a capture truncated at a rotation boundary can end in
// one, and that is not a corruption worth refusing. Anything else that does not
// decode into a complete event of a known type is ErrBadEvent naming the line.
func (r *Reader) Next() (*Event, error) {
	for {
		if r.done {
			return nil, lineErr(r.line, ErrEndOfStream, "no further events")
		}
		if !r.sc.Scan() {
			r.done = true
			if err := r.sc.Err(); err != nil {
				var b textbuf.Buffer
				return nil, lineErr(r.line+1, ErrBadEvent, b.Err(err).String())
			}
			return nil, lineErr(r.line, ErrEndOfStream, "no further events")
		}
		r.line++
		raw := r.sc.Bytes()
		if len(raw) == 0 {
			continue
		}

		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			var b textbuf.Buffer
			return nil, lineErr(r.line, ErrBadEvent, b.Err(err).String())
		}
		if err := r.validate(&ev); err != nil {
			return nil, err
		}
		r.lastSeq = ev.Seq
		return &ev, nil
	}
}

// validate enforces the invariants a replay depends on: the sequence advances,
// the type is one this version knows, and a message event actually carries the
// bytes it claims. A stream that fails any of these is not replayable, and
// saying so is the whole point of the check.
func (r *Reader) validate(ev *Event) error {
	var b textbuf.Buffer
	if ev.Seq <= r.lastSeq {
		detail := b.Str("sequence went from ").Uint(r.lastSeq).Str(" to ").Uint(ev.Seq).String()
		return lineErr(r.line, ErrSequence, detail)
	}
	switch ev.Type {
	case EventMessage:
		if len(ev.Data) == 0 {
			return lineErr(r.line, ErrBadEvent, "message event carries no data")
		}
		if int(ev.Len) != len(ev.Data) {
			detail := b.Str("message len is ").Uint16(ev.Len).
				Str(" but data is ").Int(int64(len(ev.Data))).Str(" bytes").String()
			return lineErr(r.line, ErrBadEvent, detail)
		}
	case EventConfig:
		if ev.Op == "" {
			return lineErr(r.line, ErrBadEvent, "config event names no operation")
		}
	case EventSession:
		if ev.Event == "" {
			return lineErr(r.line, ErrBadEvent, "session event names no event")
		}
	default:
		return lineErr(r.line, ErrBadEvent, b.Str("unknown event type ").Quoted(ev.Type).String())
	}
	return nil
}

// lineErr wraps a sentinel with the line number and a specific reason.
func lineErr(line int, kind error, detail string) error {
	return &lineError{line: line, kind: kind, detail: detail}
}

// lineError carries both the sentinel a caller branches on and the position an
// operator needs. errors.Is reaches the sentinel through Unwrap.
type lineError struct {
	kind   error
	detail string
	line   int
}

func (e *lineError) Error() string {
	var b textbuf.Buffer
	return b.Str("capture: line ").Int(int64(e.line)).Str(": ").
		Err(e.kind).Str(": ").Str(e.detail).String()
}

func (e *lineError) Unwrap() error { return e.kind }
