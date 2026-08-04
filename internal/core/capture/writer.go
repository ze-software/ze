// Design: plan/spec-improve-3-event-replay.md -- bounded JSONL capture writer
// Overview: capture.go -- the format this encoder writes

package capture

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"
	"time"
)

// Writer encodes capture events as JSONL into an io.Writer, never past a byte
// bound.
//
// # The bound
//
// limit is a HARD cap on the bytes this Writer will hand to the underlying
// io.Writer, header included. Every line is encoded into a reusable scratch
// buffer first and its exact length is checked against the remaining budget, so
// a line that would cross the bound is refused WHOLE, with ErrLimitReached and
// nothing written. A capture file therefore never exceeds limit by one byte, and
// never ends in a half-written line a reader would report as corrupt.
//
// limit <= 0 means unbounded, which is for tests and for an in-memory sink. A
// file-backed capture always sets one.
//
// # Allocation
//
// The scratch buffer is reused across events and grows to the largest line seen,
// so steady-state encoding allocates nothing. This matters because an enabled
// capture encodes one line per received message.
//
// A Writer is NOT safe for concurrent use. The reactor's capture owns one per
// file and drives it from a single writer goroutine.
type Writer struct {
	w     io.Writer
	buf   []byte
	seq   uint64
	n     int64
	limit int64
}

// NewWriter returns a Writer that appends to w and never writes more than limit
// bytes in total. limit <= 0 is unbounded.
func NewWriter(w io.Writer, limit int64) *Writer {
	return &Writer{w: w, buf: make([]byte, 0, 1024), limit: limit}
}

// SetLimit changes the byte bound. It exists for one caller: a capture that held
// back a tail reserve for its closing lines and now needs the full budget to
// write them. Lowering the limit below what is already written does not truncate
// anything; it only refuses every later write.
func (w *Writer) SetLimit(limit int64) { w.limit = limit }

// WriteHeader writes the first line of the file. It must be called once, before
// any event.
func (w *Writer) WriteHeader(h Header) error {
	h.Format = Format
	h.Version = Version
	// HTML escaping off: the event lines below are hand-encoded and do not
	// escape, so leaving it on would make one line of the file follow a
	// different escaping rule from every other line.
	var line bytes.Buffer
	enc := json.NewEncoder(&line)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(h); err != nil {
		return err
	}
	// Encode already appended the newline that ends the JSONL line.
	w.buf = append(w.buf[:0], line.Bytes()...)
	return w.flush()
}

// WriteMessage records one complete wire message. data must be the FULL message
// including its header; the reader restores exactly these bytes.
func (w *Writer) WriteMessage(ts time.Time, direction string, msgType uint8, data []byte, sourceID uint32, ctxID uint16) error {
	w.begin(ts, EventMessage)
	w.field("direction")
	w.quoted(direction)
	w.field("msg-type")
	w.buf = strconv.AppendUint(w.buf, uint64(msgType), 10)
	w.field("len")
	w.buf = strconv.AppendUint(w.buf, uint64(len(data)), 10)
	w.field("data")
	w.buf = append(w.buf, '"')
	w.buf = base64.StdEncoding.AppendEncode(w.buf, data)
	w.buf = append(w.buf, '"')
	if sourceID != 0 {
		w.field("source-id")
		w.buf = strconv.AppendUint(w.buf, uint64(sourceID), 10)
	}
	if ctxID != 0 {
		w.field("ctx-id")
		w.buf = strconv.AppendUint(w.buf, uint64(ctxID), 10)
	}
	return w.end()
}

// WriteConfig records one config operation the reactor applied. payload must
// already be valid JSON and already redacted (RedactPayload); a nil payload
// omits the field.
//
// A payload that would push the line past MaxLineLen is replaced by a marker
// naming the byte count. This is the one place the format drops content on
// purpose, and it exists because a config payload is the only unbounded field:
// a reconcile carries the WHOLE config tree, which grows with the peer count,
// while a message is capped at 65535 bytes by the wire. Writing the long line
// anyway would produce a file this package's own Reader refuses, and it refuses
// at the FIRST long line, so one oversized reconcile would cost the operator
// every event after it.
func (w *Writer) WriteConfig(ts time.Time, op, txID string, payload []byte) error {
	w.begin(ts, EventConfig)
	w.field("op")
	w.quoted(op)
	if txID != "" {
		w.field("tx-id")
		w.quoted(txID)
	}
	if len(payload) > 0 {
		w.field("payload")
		// 2 for the closing brace and the newline end() appends.
		if len(w.buf)+len(payload)+2 > MaxLineLen {
			w.buf = append(w.buf, `{"capture-omitted-payload-bytes":`...)
			w.buf = strconv.AppendInt(w.buf, int64(len(payload)), 10)
			w.buf = append(w.buf, '}')
		} else {
			w.buf = append(w.buf, payload...)
		}
	}
	return w.end()
}

// WriteSession records a session-lifecycle or writer-lifecycle event. drops is
// written only for SessionDrops, where it is the cumulative count of events the
// writer shed, so a reader knows the stream has a gap.
func (w *Writer) WriteSession(ts time.Time, event string, drops uint64) error {
	w.begin(ts, EventSession)
	w.field("event")
	w.quoted(event)
	if event == SessionDrops || drops != 0 {
		w.field("drops")
		w.buf = strconv.AppendUint(w.buf, drops, 10)
	}
	return w.end()
}

// begin starts an event line in the scratch buffer with the common fields. The
// sequence counter advances only once the line is accepted (see end), so a
// refused write leaves no gap in the numbering.
func (w *Writer) begin(ts time.Time, eventType string) {
	w.buf = append(w.buf[:0], `{"seq":`...)
	w.buf = strconv.AppendUint(w.buf, w.seq+1, 10)
	w.buf = append(w.buf, `,"ts":"`...)
	w.buf = ts.UTC().AppendFormat(w.buf, TimeFormat)
	w.buf = append(w.buf, `","type":"`...)
	w.buf = append(w.buf, eventType...)
	w.buf = append(w.buf, '"')
}

// end closes the line and flushes it, advancing the sequence only on success.
func (w *Writer) end() error {
	w.buf = append(w.buf, '}', '\n')
	if err := w.flush(); err != nil {
		return err
	}
	w.seq++
	return nil
}

// field appends a comma, a quoted key, and a colon.
func (w *Writer) field(name string) {
	w.buf = append(w.buf, ',', '"')
	w.buf = append(w.buf, name...)
	w.buf = append(w.buf, '"', ':')
}

// maxFieldLen bounds one quoted field value. Every such value is a direction, an
// operation name, an event name or a transaction id, and none is legitimately
// longer. The bound exists because WriteConfig bounds only the PAYLOAD, so
// without it a caller passing an unbounded operation name through
// captureOperationPhase's default branch could still emit a line the package's
// own Reader refuses.
const maxFieldLen = 256

// quoted appends a JSON string, truncated to maxFieldLen bytes. Every value
// written this way comes from a package constant or a config-supplied
// identifier, so the escape set is the minimum JSON requires plus the control
// range.
func (w *Writer) quoted(s string) {
	if len(s) > maxFieldLen {
		s = s[:maxFieldLen]
	}
	w.buf = append(w.buf, '"')
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			w.buf = append(w.buf, '\\', c)
		case c == '\n':
			w.buf = append(w.buf, '\\', 'n')
		case c == '\r':
			w.buf = append(w.buf, '\\', 'r')
		case c == '\t':
			w.buf = append(w.buf, '\\', 't')
		case c < 0x20:
			w.buf = append(w.buf, '\\', 'u', '0', '0',
				hexDigit[c>>4], hexDigit[c&0x0f])
		default:
			w.buf = append(w.buf, c)
		}
	}
	w.buf = append(w.buf, '"')
}

const hexDigit = "0123456789abcdef"

// flush writes the scratch buffer, refusing the whole line when it would cross
// the bound. Nothing partial ever reaches the file.
func (w *Writer) flush() error {
	if w.limit > 0 && w.n+int64(len(w.buf)) > w.limit {
		return ErrLimitReached
	}
	n, err := w.w.Write(w.buf)
	w.n += int64(n)
	return err
}
