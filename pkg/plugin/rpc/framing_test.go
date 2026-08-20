// Design: docs/architecture/api/process-protocol.md -- newline-delimited frame I/O
// Related: framing.go -- the reader and the writer under test

package rpc

import (
	"bytes"
	"testing"
)

// TestAnswerRecordLineAtTheSizeLimitCrossesTheFrame checks which side of
// MaxMessageSize a record line must stay on. The method: a record line of
// exactly MaxMessageSize bytes is written and read back, and one byte more is
// offered to the same writer.
//
// It is not parallel: each case holds 16 MB, and the point of the test is that
// the size is the real one rather than a scaled model of it.
//
// VALIDATES: AC-15 of the streaming answer protocol -- the size a producer
// refuses a record by is the size the transport refuses, to the byte.
// PREVENTS: a producer that rejects a record the frame would have carried, or
// builds one the frame refuses, which the newline in each direction makes easy
// to get wrong by one.
func TestAnswerRecordLineAtTheSizeLimitCrossesTheFrame(t *testing.T) {
	prefix := AnswerRecordLineSize(AnswerNoID, Record{})
	item := bytes.Repeat([]byte{'x'}, MaxMessageSize-prefix)

	line := AppendAnswerItem(nil, AnswerNoID, item)
	if len(line) != MaxMessageSize {
		t.Fatalf("the line is %d bytes, want exactly the %d limit", len(line), MaxMessageSize)
	}

	var wire bytes.Buffer
	if err := NewFrameWriter(&wire).Write(line); err != nil {
		t.Fatalf("a line of exactly %d bytes was refused: %v", MaxMessageSize, err)
	}
	read, err := NewFrameReader(&wire).Read()
	if err != nil {
		t.Fatalf("read the line back: %v", err)
	}
	if len(read) != len(line) {
		t.Errorf("the line read back is %d bytes, want the %d written", len(read), len(line))
	}

	line = append(line, 'x')
	if err = NewFrameWriter(&bytes.Buffer{}).Write(line); err == nil {
		t.Errorf("a line of %d bytes was accepted, want it refused over the %d limit", len(line), MaxMessageSize)
	}
}
