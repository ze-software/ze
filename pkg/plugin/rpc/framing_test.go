// Design: docs/architecture/api/process-protocol.md -- newline-delimited frame I/O
// Related: framing.go -- the reader and the writer under test

package rpc

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	item := answerItemOfLineSize(t, AnswerNoID, MaxMessageSize)

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

// TestCountedValuesCarryNewlinesAndCarriageReturns checks that a value holding
// the byte that ends a line, and one holding a carriage return, both reach a
// reader unchanged. The method: a record payload whose last byte is `\r`, one
// holding a raw `\n`, one holding `\r\n`, and a terminator message spelled over
// two lines are each written through the frame writer and read back through the
// frame reader, and compared byte for byte.
//
// VALIDATES: AC-19 -- both round-trip byte for byte. Neither is rewritten,
// neither is truncated, and no reader strips anything after the counted
// payload.
// PREVENTS: the byte pass over operator data coming back, and bufio.ScanLines
// taking a trailing `\r` a producer meant as data. This fails against
// bufio.ScanLines and against the rewriting pass alike, which is why it is
// written against the shipped frame layer rather than against the appenders.
func TestCountedValuesCarryNewlinesAndCarriageReturns(t *testing.T) {
	t.Parallel()

	payloads := []struct {
		name  string
		value string
	}{
		{name: "a payload whose last byte is a carriage return", value: `{"note":"trailing"}` + "\r"},
		{name: "a payload holding a raw newline", value: "{\n\t\"peer\": \"10.0.0.1\"\n}"},
		{name: "a payload holding a carriage return and a newline", value: "{\"note\":\"one\r\ntwo\"}"},
		{name: "a payload that is nothing but line breaks", value: "\r\n\r\n"},
	}

	for _, tt := range payloads {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var wire bytes.Buffer
			require.NoError(t, NewFrameWriter(&wire).Write(AppendAnswerItem(nil, 7, []byte(tt.value))))

			read, err := NewFrameReader(&wire).Read()
			require.NoError(t, err, "the line did not read back")

			_, kind, payload, err := ParseLine(read)
			require.NoError(t, err)
			tail, err := ParseAnswerTail(kind, payload)
			require.NoError(t, err)
			require.True(t, bytes.Equal([]byte(tt.value), tail.Item),
				"the payload came back as %q, want %q", tail.Item, tt.value)
		})
	}

	t.Run("a terminator message spelled over two lines", func(t *testing.T) {
		t.Parallel()

		const message = "peer 10.0.0.1 not configured\nrun `show bgp peer list`"

		var wire bytes.Buffer
		require.NoError(t, NewFrameWriter(&wire).Write(AppendAnswerTerminator(nil, 7, 0, 0, message)))

		read, err := NewFrameReader(&wire).Read()
		require.NoError(t, err, "the terminator did not read back")

		_, kind, payload, err := ParseLine(read)
		require.NoError(t, err)
		tail, err := ParseAnswerTail(kind, payload)
		require.NoError(t, err)
		require.Equal(t, message, tail.Message)
	})
}

// TestFrameRefusesWhatIsNotOneNewline checks that a line ends with exactly one
// `\n`, and that a count which disagrees with the bytes behind it is named
// rather than parsed. The method: a well-formed record line is terminated with
// `\r\n`, then with no newline at all, then given a count larger than its
// payload and one larger than the maximum message, and each is offered to the
// frame reader.
//
// VALIDATES: AC-20 -- a `\r\n` termination and a stated length that disagrees
// with what arrived are each refused with a named error rather than parsed.
// PREVENTS: a reader that strips a trailing `\r`, which corrupts a payload a
// conforming writer sent; and a count read off the wire being used to slice or
// to grow a buffer before it is bounded.
func TestFrameRefusesWhatIsNotOneNewline(t *testing.T) {
	t.Parallel()

	line := string(AppendAnswerItem(nil, 7, []byte(`{"peer":"10.0.0.1"}`)))

	tests := []struct {
		name    string
		wire    string
		refusal string
	}{
		{
			name:    "terminated with a carriage return and a newline",
			wire:    line + "\r\n",
			refusal: `terminated by "\r", want exactly one newline`,
		},
		{
			name:    "terminated by a byte that is not a newline",
			wire:    line + "X\n",
			refusal: `terminated by "X", want exactly one newline`,
		},
		{
			name:    "a count larger than the payload behind it",
			wire:    "#7 row 400:{\"peer\":\"10.0.0.1\"}\n",
			refusal: "arrived before the stream ended",
		},
		{
			name:    "a count past the maximum message",
			wire:    "#7 row 999999999:x\n",
			refusal: "past the 16777216-byte maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewFrameReader(strings.NewReader(tt.wire)).Read()
			require.Error(t, err, "the frame reader accepted %q", tt.wire)
			require.ErrorContains(t, err, tt.refusal,
				"%q was refused by another check than the one under test", tt.wire)
		})
	}
}

// TestAnswerLineWidthMatchesTheWriters checks that the field-shape table the
// frame reader computes a line's width from is the field order the appenders
// write. The method: one line of every kind is built by the shipped appenders,
// under an id and without one, and the width the table computes is compared
// with the length of the line itself.
//
// VALIDATES: the frame's own reading of the grammar cannot drift from the
// writers, which is what makes a count-driven frame safe to put under every
// line the protocol carries.
// PREVENTS: a kind whose fields move in the appender and not in the table,
// which would frame every line of that kind at the wrong byte.
func TestAnswerLineWidthMatchesTheWriters(t *testing.T) {
	t.Parallel()

	for _, id := range []uint64{AnswerNoID, 7, 1234567890, math.MaxUint64} {
		for kind, line := range answerKindLines(id) {
			width, state := answerLineWidth([]byte(line))
			require.Equal(t, answerLineStated, state,
				"id %d: the %s line does not state its own width: %q", id, kind, line)
			assert.Equal(t, uint64(len(line)), width,
				"id %d: the %s line is %d bytes and the table computes %d: %q", id, kind, len(line), width, line)
		}
	}

	// A request and a response state no width, so their newline frames them.
	for _, line := range []string{
		string(AppendRequest(nil, 7, "ze-bgp:peer-list", nil)),
		string(AppendOK(nil, 7)),
		string(AppendError(nil, 7, nil)),
		"error: no such command",
	} {
		_, state := answerLineWidth([]byte(line))
		assert.Equal(t, answerLineOther, state, "%q was read as an answer line", line)
	}
}
