package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"slices"
	"strings"
	"testing"
)

// answerItemOfLineSize returns the item whose record line under id is exactly
// size bytes.
//
// The payload's byte count is itself a field of the line, so the prefix widens
// with the payload and a fixed subtraction misses it by the width of that
// count. Correcting by the difference converges wherever the count's digit
// width is stable, which it is at every size this suite asks for.
func answerItemOfLineSize(t *testing.T, id uint64, size int) []byte {
	t.Helper()

	item := bytes.Repeat([]byte{'x'}, size-AnswerRecordLineSize(id, Record{}))
	for range 4 {
		delta := size - AnswerRecordLineSize(id, Record{Item: item})
		if delta == 0 {
			return item
		}
		item = bytes.Repeat([]byte{'x'}, len(item)+delta)
	}
	t.Fatalf("no item makes a record line of exactly %d bytes under id %d", size, id)
	return nil
}

// rejectedRow is one entry of the errors collection a buffered consumer reads,
// as appendAnswerRecordTooLargeFault writes it.
type rejectedRow struct {
	Message      string `json:"message"`
	Record       uint64 `json:"record"`
	EncodedBytes int64  `json:"encoded-bytes"`
	LimitBytes   int64  `json:"limit-bytes"`
}

// TestRecordSizeBoundaryIsTheEncodedLine checks the off-by-one of the refusal.
// The method: one item is built one byte over the limit and sliced to exactly
// the limit, and boundedRecord is asked about both. The at-limit record must
// come back untouched, because refusing a record the transport accepts loses a
// row for nothing.
//
// VALIDATES: AC-15 of spec-streaming-answer-protocol, boundary -- a record line
// of exactly MaxMessageSize is written, and one byte more is rejected.
// PREVENTS: a limit compared against the item rather than the line, which
// refuses every record within the last few bytes of the range.
func TestRecordSizeBoundaryIsTheEncodedLine(t *testing.T) {
	const id = 7

	atLimit := answerItemOfLineSize(t, id, MaxMessageSize)
	overLimit := answerItemOfLineSize(t, id, MaxMessageSize+1)

	kept := boundedRecord(id, 1, Record{Item: atLimit})
	if len(kept.Fault) > 0 {
		t.Errorf("a record line of exactly %d bytes was rejected: %s", MaxMessageSize, kept.Fault)
	}
	if len(kept.Item) != len(atLimit) {
		t.Errorf("the kept record carries %d bytes, want the %d it was given", len(kept.Item), len(atLimit))
	}

	rejected := boundedRecord(id, 1, Record{Item: overLimit})
	if len(rejected.Item) > 0 {
		t.Fatalf("a record line of %d bytes was written, want it rejected", MaxMessageSize+1)
	}
	var row rejectedRow
	if err := json.Unmarshal(rejected.Fault, &row); err != nil {
		t.Fatalf("read the rejected row %s: %v", rejected.Fault, err)
	}
	if row.EncodedBytes != MaxMessageSize+1 {
		t.Errorf("the rejected row states %d encoded bytes, want %d", row.EncodedBytes, MaxMessageSize+1)
	}
}

// TestTheRejectedRowFitsTheLineTheRecordDidNot checks the trap the refusal
// carries: a fault quoting the record it rejected would be rejected for the
// same reason, and the answer would then have no way to report anything. The
// method: the widest fault the builder can write, for the largest id, is
// measured against the limit and against its own capacity.
//
// VALIDATES: AC-15 of spec-streaming-answer-protocol -- the fault that replaces
// a wide record always reaches the wire.
// PREVENTS: a fault built from the record, which is the shape that turns one
// wide row into a failed answer.
func TestTheRejectedRowFitsTheLineTheRecordDidNot(t *testing.T) {
	fault := appendAnswerRecordTooLargeFault(nil, math.MaxUint64, math.MaxInt)
	if len(fault) > answerFaultCapacity {
		t.Errorf("the widest rejected row is %d bytes and the builder reserves %d, so it grows its slice", len(fault), answerFaultCapacity)
	}
	if size := AnswerRecordLineSize(math.MaxUint64, Record{Fault: fault}); size > MaxMessageSize {
		t.Errorf("the rejected row's line is %d bytes, over the %d limit it exists to report", size, MaxMessageSize)
	}
	var row rejectedRow
	if err := json.Unmarshal(fault, &row); err != nil {
		t.Fatalf("the rejected row %s is not readable: %v", fault, err)
	}
	if row.Record != math.MaxUint64 {
		t.Errorf("the rejected row names record %d, want %d", row.Record, uint64(math.MaxUint64))
	}
}

// TestWriteRecordAnswerRefusesTheReservedEnvelope checks that the ONE writer
// both ends of the connection use refuses an answer naming the envelope the
// rejected rows travel under, before it writes a line. The method: an answer
// naming that key is written to a buffer, and the buffer must stay empty.
//
// VALIDATES: AC-9 of spec-record-answers-1-sdk-path -- the reserved envelope is
// refused by the record producer as well as by the collapse, so a producer
// learns on its first answer whatever its row count turns out to be.
// PREVENTS: a streamed answer, which never reaches the collapse, carrying two
// collections under one key so that one overwrites the other.
func TestWriteRecordAnswerRefusesTheReservedEnvelope(t *testing.T) {
	t.Parallel()

	var wire bytes.Buffer
	row := RawRow(`{"peer":"10.0.0.1"}`)
	rows := func(yield func(RowRecord) bool) { yield(RowRecord{Item: &row}) }

	err := WriteRecordAnswer(&wire, 3, AnswerTail{Key: AnswerErrorsKey}, rows)
	if err == nil {
		t.Fatal("an answer naming the reserved envelope was written, want it refused")
	}
	if wire.Len() != 0 {
		t.Errorf("the refusal wrote %d bytes, want none: %q", wire.Len(), wire.String())
	}
}

// answerLineLimitWriter is the connection's own refusal, in a writer a test can
// hold. Conn.writeFrame (conn.go) rejects a frame wider than MaxMessageSize and
// the newline that ends it, so a line no producer bounded reaches no peer and
// the answer stops where it stands. A bytes.Buffer accepts such a line, which is
// what would let an unbounded document look written.
//
// It keeps what each line SAID rather than the line. One buffer is reused for
// every line of an answer, so a tail's slices point at another line's bytes by
// the time a test reads them, and a copy of a 16 MB record would double what
// this suite needs to say anything about it. items records each line's payload
// size, which is the only fact the tests read off a record.
type answerLineLimitWriter struct {
	tails []AnswerTail
	items []int
	err   error
}

// Write takes one framed answer line, refuses it exactly as the connection
// does, and keeps what it stated. The first line it cannot read back is kept as
// err, so a malformed line is reported by the test rather than by a panic here.
func (w *answerLineLimitWriter) Write(line []byte) (int, error) {
	if len(line) > MaxMessageSize+1 {
		return 0, fmt.Errorf("message exceeds maximum size %d", MaxMessageSize)
	}
	if w.err == nil {
		w.err = w.keep(line)
	}
	return len(line), nil
}

// keep reads one written line the way a peer reads it and records what it said.
func (w *answerLineLimitWriter) keep(line []byte) error {
	if len(line) == 0 || line[len(line)-1] != '\n' {
		return fmt.Errorf("an answer line of %d bytes ends with no newline", len(line))
	}
	_, kind, payload, err := ParseLine(line[:len(line)-1])
	if err != nil {
		return err
	}
	tail, err := ParseAnswerTail(kind, payload)
	if err != nil {
		return err
	}
	w.items = append(w.items, len(tail.Item))
	tail.Item = nil
	tail.Fault = slices.Clone(tail.Fault)
	w.tails = append(w.tails, tail)
	return nil
}

// kinds reports the kind of every line the answer wrote, in order.
func (w *answerLineLimitWriter) kinds() []string {
	kinds := make([]string, len(w.tails))
	for index := range w.tails {
		kinds[index] = w.tails[index].Kind
	}
	return kinds
}

// checkShape requires the answer to be the lines want names, under a head of
// item type wantType, and returns the terminator so the caller can read the
// outcome off it.
func (w *answerLineLimitWriter) checkShape(t *testing.T, wantType string, want []string) *AnswerTail {
	t.Helper()

	if w.err != nil {
		t.Fatalf("read back the answer that was written: %v", w.err)
	}
	if got := w.kinds(); !slices.Equal(got, want) {
		t.Fatalf("the answer wrote the lines %v, want %v", got, want)
	}
	if got := w.tails[0].Type; got != wantType {
		t.Errorf("the head states item type %q, want %q", got, wantType)
	}
	return &w.tails[len(w.tails)-1]
}

// checkRejectedDocument requires the answer's middle line to be the fault an
// over-wide record earns, and reads its four fields.
func (w *answerLineLimitWriter) checkRejectedDocument(t *testing.T) {
	t.Helper()

	var row rejectedRow
	if err := json.Unmarshal(w.tails[1].Fault, &row); err != nil {
		t.Fatalf("read the rejected row %s: %v", w.tails[1].Fault, err)
	}
	if row.Message != answerRecordTooLargeText {
		t.Errorf("the rejected document says %q, want %q", row.Message, answerRecordTooLargeText)
	}
	if row.Record != 1 {
		t.Errorf("the rejected document names record %d, want 1: a document is the answer's only record", row.Record)
	}
	if row.LimitBytes != MaxMessageSize {
		t.Errorf("the rejected document states a limit of %d, want %d", row.LimitBytes, MaxMessageSize)
	}
	if row.EncodedBytes <= MaxMessageSize {
		t.Errorf("the rejected document states %d encoded bytes, want more than the %d limit", row.EncodedBytes, MaxMessageSize)
	}
}

// collapse reads the answer back the way a buffered consumer does, and is what
// holds the head's item type to its meaning. CollapseAnswer refuses a doc answer
// that carries a rejected row by name, because a document has nowhere to put one
// beside itself, so an answer whose document has no line must not call itself a
// document answer.
//
// It reads the records this writer kept, which is why only an answer whose
// records are small is passed through it.
func (w *answerLineLimitWriter) collapse(t *testing.T) string {
	t.Helper()

	records := slices.Clone(w.tails[1 : len(w.tails)-1])
	rows := func(yield func(Record) bool) {
		for index := range records {
			if !yield(Record{Item: records[index].Item, Fault: records[index].Fault}) {
				return
			}
		}
	}
	document, err := CollapseAnswer(NewAnswer(w.tails[0], w.tails[len(w.tails)-1], rows))
	if err != nil {
		t.Fatalf("a buffered consumer could not read the answer back: %v", err)
	}
	return string(document)
}

// answerRecordTooLargeText is the text a rejected row states, written out here
// rather than read off the builder. An operator meets this sentence and
// test/plugin/plugin-command-partial-fault.ci asserts it, so it is a fixture
// held against drift rather than a value derived from the code under test.
const answerRecordTooLargeText = "answer record does not fit one wire message"

// answerRowsCollapsingPastTheLimit returns a walk short enough to be answered as
// one document and wide enough that the document has no line.
//
// Each row fits a line on its own, and the test refuses the fixture when one
// does not, so what the answer rejects is the DOCUMENT and never a row. That
// distinction is the whole fixture: the encoder bounded the rows and nothing
// bounded what they collapsed to.
func answerRowsCollapsingPastTheLimit(t *testing.T, id uint64) iter.Seq[RowRecord] {
	t.Helper()

	// Three JSON strings of half the maximum each collapse to about one and a
	// half lines, which leaves the fixture correct at either edge rather than
	// resting on the collapse's own few bytes of envelope.
	const rows = 3
	row := RawRow(`"` + strings.Repeat("x", MaxMessageSize/2) + `"`)
	if size := AnswerRecordLineSize(id, Record{Item: json.RawMessage(row)}); size > MaxMessageSize {
		t.Fatalf("fixture: one row's line is %d bytes, past the %d maximum, so the walk would reject a ROW", size, MaxMessageSize)
	}
	return func(yield func(RowRecord) bool) {
		for range rows {
			if !yield(RowRecord{Item: &row}) {
				return
			}
		}
	}
}

// TestWriteDocumentAnswerBounded checks that the one line a bounded answer
// occupies is measured before it is written, and that a document with no wire
// form earns the fault a record with no wire form earns rather than the write
// failing under it.
//
// The method: three answers are written to a writer that refuses what the
// connection refuses (answerLineLimitWriter), and each answer is read back with
// the shipped reader. A document whose line is exactly MaxMessageSize must be
// written whole. One byte more must reach the terminator as a rejected row. And
// a bounded WALK whose rows each fit but whose collapse does not must reach it
// the same way: that is the state
// plan/journal/gate-excludes-part-of-its-population.md recorded, where the
// encoder bounded the rows and nothing bounded the document they collapsed to.
//
// The head's item type is asserted with the kinds, and the two rejections are
// read back through CollapseAnswer as well. An answer whose document has no
// line is not a document answer, and the buffered consumer is what says so: it
// refuses a doc answer carrying a rejected row by name.
//
// VALIDATES: AC-9 -- a bounded answer's document past one wire message is
// refused with the same fault an over-wide record gets, rather than being
// written and read as truncated.
// PREVENTS: R-7 -- an answer the daemon produced whole reaching an operator as
// a truncated one, because its one document line failed the write and the walk
// returned before its terminator.
func TestWriteDocumentAnswerBounded(t *testing.T) {
	const id = 7

	t.Run("a document whose line is exactly the maximum is written whole", func(t *testing.T) {
		document := answerItemOfLineSize(t, id, MaxMessageSize)

		wire := &answerLineLimitWriter{}
		if err := WriteDocumentAnswer(wire, id, AnswerTail{}, document); err != nil {
			t.Fatalf("a document line of exactly %d bytes was refused: %v", MaxMessageSize, err)
		}

		terminator := wire.checkShape(t, AnswerTypeDocument, []string{AnswerKindHead, AnswerKindRecord, AnswerKindTerminator})
		if wire.items[1] != len(document) {
			t.Errorf("the record line carries %d bytes, want the %d the document holds", wire.items[1], len(document))
		}
		if terminator.Count != 1 || terminator.Faults != 0 {
			t.Errorf("the terminator states %d produced and %d rejected, want 1 and 0", terminator.Count, terminator.Faults)
		}
		if verdict := Verdict(terminator); verdict != VerdictDone {
			t.Errorf("the answer derives %q, want %q", verdict, VerdictDone)
		}
	})

	t.Run("a document one byte past the maximum is rejected", func(t *testing.T) {
		document := answerItemOfLineSize(t, id, MaxMessageSize+1)

		wire := &answerLineLimitWriter{}
		if err := WriteDocumentAnswer(wire, id, AnswerTail{}, document); err != nil {
			t.Fatalf("a document line of %d bytes was written to the connection instead of being rejected: %v", MaxMessageSize+1, err)
		}

		terminator := wire.checkShape(t, AnswerTypeMap, []string{AnswerKindHead, AnswerKindFault, AnswerKindTerminator})
		wire.checkRejectedDocument(t)
		if terminator.Count != 0 || terminator.Faults != 1 {
			t.Errorf("the terminator states %d produced and %d rejected, want 0 and 1", terminator.Count, terminator.Faults)
		}
		if verdict := Verdict(terminator); verdict != VerdictError {
			t.Errorf("the answer derives %q, want %q: no record reached the consumer", verdict, VerdictError)
		}
		if got := wire.collapse(t); !strings.Contains(got, answerRecordTooLargeText) {
			t.Errorf("a buffered consumer reads %q, want the rejected row's reason in it", got)
		}
	})

	t.Run("a bounded walk whose rows fit and whose document does not is rejected", func(t *testing.T) {
		wire := &answerLineLimitWriter{}
		rows := answerRowsCollapsingPastTheLimit(t, id)
		if err := WriteRecordAnswer(wire, id, AnswerTail{Key: "rows"}, rows); err != nil {
			t.Fatalf("a bounded walk lost its whole answer to the wire message maximum: %v", err)
		}

		terminator := wire.checkShape(t, AnswerTypeMap, []string{AnswerKindHead, AnswerKindFault, AnswerKindTerminator})
		wire.checkRejectedDocument(t)
		if terminator.Count != 0 || terminator.Faults != 1 {
			t.Errorf("the terminator states %d produced and %d rejected, want 0 and 1: no row reached the consumer", terminator.Count, terminator.Faults)
		}
		if verdict := Verdict(terminator); verdict != VerdictError {
			t.Errorf("the answer derives %q, want %q, which is what R-7 recorded as truncated", verdict, VerdictError)
		}
		if got := wire.collapse(t); !strings.Contains(got, answerRecordTooLargeText) {
			t.Errorf("a buffered consumer reads %q, want the rejected row's reason in it", got)
		}
	})
}
