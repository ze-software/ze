// Design: docs/architecture/api/process-protocol.md -- newline-delimited frame I/O
// Related: framing.go -- the reader and the writer under test

package rpc

import (
	"bufio"
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

// TestCountedTextPastTheMaximumDoesNotWrapItsWidth checks that a byte count a
// peer chose cannot become a SMALL width. The method: a record line states a
// count near the range of a uint64, which is what makes the header plus the
// count wrap, and the width the frame reader computes is required to stay past
// MaxMessageSize so the reader refuses the line.
//
// The wrap is the whole point. A counted text is the digits, the colon, and the
// bytes: adding that 21-byte header to a count of 2^64-21 or more rolls the sum
// back to a number under 21, which is under the maximum and would frame the
// line INSIDE its own count field.
//
// VALIDATES: the MaxMessageSize bound holds for every count a peer can spell,
// not only the ones that leave the sum in range.
// PREVENTS: an untrusted count defeating the one bound that stands between it
// and the slice the frame reader takes.
func TestCountedTextPastTheMaximumDoesNotWrapItsWidth(t *testing.T) {
	t.Parallel()

	// Each count wraps `header + size` to a different small number: 2^64-21
	// wraps it to 0, and each step up moves the wrapped width one byte on.
	for _, count := range []string{
		"18446744073709551595", // 2^64 - 21, the header's own width
		"18446744073709551599",
		"18446744073709551615", // the widest a uint64 spells
	} {
		line := "#7 " + AnswerKindRecord + " " + count + ":x"

		width, state := answerLineWidth([]byte(line))
		require.Equal(t, answerLineStated, state, "%q did not state a width", line)
		assert.Greater(t, width, uint64(MaxMessageSize),
			"%q states a width of %d, which is inside the %d-byte maximum", line, width, MaxMessageSize)

		_, err := NewFrameReader(strings.NewReader(line + "\n")).Read()
		require.Error(t, err, "the frame reader accepted %q", line)
		assert.ErrorContains(t, err, "past the 16777216-byte maximum",
			"%q was refused by another check than the one under test", line)
	}
}

// TestPlainTextIsFramedByItsNewlineAlone checks that the split function a
// rendering stream uses frames on the newline and measures nothing. The method:
// lines of an operator's text that OPEN with an answer kind word and fields
// that parse are read through both split functions, and only the plain-text one
// delivers them.
//
// The pairing is the assertion. ScanAnswerLines is right for a stream of answer
// lines and wrong for text, because text states no width: it measures the line
// by fields the renderer never wrote and then refuses the stream over the byte
// it lands on, delivering nothing at all -- the lines already read included.
//
// VALIDATES: a carriage return in an operator's data still reaches the caller,
// which is what bufio.ScanLines got wrong, without the width arithmetic that
// belongs to a frame.
// PREVENTS: the answer framer being put back on a plain-text stream, where one
// rendered line that reads like an answer costs an operator the whole output.
func TestPlainTextIsFramedByItsNewlineAlone(t *testing.T) {
	t.Parallel()

	// Rendering lines that open with a kind word and carry fields its shape
	// accepts. Nothing here is an answer line: no producer of this protocol
	// wrote one of them.
	const text = "end 1 0 0: rows written\nrow 3:abc and more\ntop map 5:peers 0: trailing\nend of file\r\n"
	want := []string{
		"end 1 0 0: rows written",
		"row 3:abc and more",
		"top map 5:peers 0: trailing",
		"end of file\r",
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Split(ScanLinesKeepingReturns)
	var got []string
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	require.NoError(t, scanner.Err(), "the rendering stream was refused")
	assert.Equal(t, want, got, "the rendering did not reach the caller line for line")

	// The same text through the answer framer, which is what this stream MUST
	// NOT use: it measures the first line and refuses the stream on the byte
	// the arithmetic lands on.
	scanner = bufio.NewScanner(strings.NewReader(text))
	scanner.Split(ScanAnswerLines)
	require.False(t, scanner.Scan(), "the answer framer delivered a line of plain text")
	require.Error(t, scanner.Err(), "the answer framer read plain text without measuring it")
}
