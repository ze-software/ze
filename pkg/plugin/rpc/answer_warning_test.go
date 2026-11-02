package rpc

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnswerWarningTerminator checks the warning terminator from the bytes a
// writer appends to the verdict a reader derives. The method: append each
// terminator, parse it back through ParseAnswerLine, and derive its verdict;
// then check that a failure or a rejected row outranks the warning.
//
// VALIDATES: `wrn` carries the terminator's fields, parses as a terminator
// with Warning set, and derives VerdictWarning only where the counts say done.
// PREVENTS: a warning outcome with no wire form, read as done by every socket
// consumer (journal row 2026-10-20, silent-fall-through).
func TestAnswerWarningTerminator(t *testing.T) {
	t.Parallel()

	line := AppendAnswerWarning(nil, 9, 1, 0, "")
	assert.Equal(t, "#9 wrn 1 0 0:", string(line))

	kind, tail, err := ParseAnswerLine(line[len("#9 "):])
	require.NoError(t, err)
	assert.Equal(t, AnswerKindWarning, kind, "the token read off the line")
	assert.Equal(t, AnswerKindTerminator, tail.Kind, "a warning terminator still ends the answer")
	assert.True(t, tail.Warning)
	assert.Equal(t, uint64(1), tail.Count)
	assert.Equal(t, VerdictWarning, Verdict(&tail))

	ended := AnswerTail{Kind: AnswerKindTerminator, Count: 1}
	assert.Equal(t, VerdictDone, Verdict(&ended), "an `end` terminator states no warning")

	failed := AnswerTail{Kind: AnswerKindTerminator, Warning: true, Message: "peer is shutting down"}
	assert.Equal(t, VerdictError, Verdict(&failed), "a failure outranks the warning")

	partial := AnswerTail{Kind: AnswerKindTerminator, Warning: true, Count: 2, Faults: 1}
	assert.Equal(t, VerdictPartial, Verdict(&partial), "a rejected row outranks the warning")
}

// TestWriteDocumentAnswerWarning checks that the one document writer spells the
// terminator `wrn` when its producer reports a warning, and `end` otherwise.
//
// VALIDATES: the writer, not each caller, chooses the terminator kind.
// PREVENTS: a warning dropped by the writer every engine answer goes through.
func TestWriteDocumentAnswerWarning(t *testing.T) {
	t.Parallel()

	var wire bytes.Buffer
	require.NoError(t, WriteDocumentAnswer(&wire, 4, AnswerTail{Warning: true}, []byte(`{"message":"m"}`)))
	assert.Equal(t, "#4 top doc 0: 0:\n#4 row 15:{\"message\":\"m\"}\n#4 wrn 1 0 0:\n", wire.String())
}

// TestAnswerWarningTerminatorCarriesLines checks the warning lines a warning
// terminator carries, from the bytes a writer appends to the lines a reader
// returns. The method: write a document answer whose head states a warning and
// two lines, compare the wire, and parse the terminator back.
//
// VALIDATES: journal row 2026-11-02 (format-cannot-hold-a-state-the-tree-holds)
// -- the wrn message field carries the lines as a JSON array, read back as
// Warnings with no failure Message; a failure is spelled end; an unreadable
// field is refused.
// PREVENTS: a warning reaching a plugin with its text dropped.
func TestAnswerWarningTerminatorCarriesLines(t *testing.T) {
	t.Parallel()

	warnings := []string{`a "b"`, "c"}
	var wire bytes.Buffer
	require.NoError(t, WriteDocumentAnswer(&wire, 4, AnswerTail{Warning: true, Warnings: warnings}, []byte(`{}`)))
	assert.Equal(t, "#4 top doc 0: 0:\n#4 row 2:{}\n#4 wrn 1 0 15:[\"a \\\"b\\\"\",\"c\"]\n", wire.String())

	kind, tail, err := ParseAnswerLine([]byte(`wrn 1 0 15:["a \"b\"","c"]`))
	require.NoError(t, err)
	assert.Equal(t, AnswerKindWarning, kind)
	assert.Equal(t, warnings, tail.Warnings)
	assert.Empty(t, tail.Message, "the lines are no failure")
	assert.Equal(t, VerdictWarning, Verdict(&tail))

	var failed bytes.Buffer
	require.NoError(t, WriteDocumentAnswer(&failed, 5, AnswerTail{Warning: true, Warnings: warnings, Message: "boom"}, nil))
	assert.Contains(t, failed.String(), "#5 end 0 0 4:boom\n", "a failure outranks the warning and is never spelled wrn")

	_, _, err = ParseAnswerLine([]byte("wrn 1 0 4:boom"))
	require.Error(t, err, "a wrn field that holds no line array is refused, never read as a failure")
}
