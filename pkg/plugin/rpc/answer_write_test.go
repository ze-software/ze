package rpc

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

// rejectedRow is one entry of the errors collection a buffered consumer reads,
// as answerRecordTooLargeFault writes it.
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

	prefix := AnswerRecordLineSize(id, Record{})
	overLimit := bytes.Repeat([]byte{'x'}, MaxMessageSize+1-prefix)
	atLimit := overLimit[:len(overLimit)-1]

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
	fault := answerRecordTooLargeFault(math.MaxUint64, math.MaxInt)
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
	rows := func(yield func(Record) bool) { yield(Record{Item: []byte(`{"peer":"10.0.0.1"}`)}) }

	err := WriteRecordAnswer(&wire, 3, AnswerTail{Status: StatusDone, Key: AnswerErrorsKey}, rows)
	if err == nil {
		t.Fatal("an answer naming the reserved envelope was written, want it refused")
	}
	if wire.Len() != 0 {
		t.Errorf("the refusal wrote %d bytes, want none: %q", wire.Len(), wire.String())
	}
}
