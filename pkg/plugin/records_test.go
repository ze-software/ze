package plugin

import (
	"bytes"
	"encoding/json"
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// jsonRow is a row a test handler already holds as JSON, appended into whatever
// buffer the SDK offers it. It is the shape a real producer takes: the row
// writes into the caller's buffer and allocates nothing of its own.
type jsonRow string

// AppendTo appends the row's JSON to buf and returns the extended slice.
func (r jsonRow) AppendTo(buf []byte) []byte { return append(buf, r...) }

// rowsFrom is the walk over items, one Record for each, in order.
func rowsFrom(items ...string) iter.Seq[Record] {
	return func(yield func(Record) bool) {
		for _, item := range items {
			if !yield(Record{Item: jsonRow(item)}) {
				return
			}
		}
	}
}

// answerTails decodes every line of one written answer into the tail it
// carries, so a test reads the wire rather than the writer that produced it.
// Each line of an answer on the plugin connection is `#<id> ok` and a key=value
// tail, which is what rpc.ParseLine cuts apart.
func answerTails(t *testing.T, id uint64, wire []byte) []rpc.AnswerTail {
	t.Helper()
	var tails []rpc.AnswerTail
	for line := range bytes.SplitSeq(bytes.TrimSuffix(wire, []byte{'\n'}), []byte{'\n'}) {
		lineID, verb, payload, err := rpc.ParseLine(line)
		require.NoError(t, err, "answer line %q", line)
		require.Equal(t, id, lineID, "answer line %q", line)
		require.Equal(t, rpc.AnswerVerbOK, verb, "answer line %q", line)
		tail, tailErr := rpc.ParseAnswerTail(payload)
		require.NoError(t, tailErr, "answer line %q", line)
		tails = append(tails, tail)
	}
	return tails
}

// TestRecordsWriteAnswerFaultsARowNoLineCanCarry checks that one row too wide
// for a wire message costs the operator that row and nothing else. The method:
// a walk yields a small row, a row wider than rpc.MaxMessageSize, and a second
// small row; the answer is written to a buffer and read back off the wire.
//
// The wide row has no wire form at all, so the only choice is which row is
// lost. Ending the walk there would lose every later row AND the terminator,
// and a consumer reads a missing terminator as a lost connection.
//
// VALIDATES: AC-6 of spec-record-answers-1-sdk-path -- a row wider than one
// wire message is reported as a fault, the walk continues, and the terminator
// counts it under faults.
// PREVENTS: a single wide row turning a plugin's whole answer into a truncated
// one, so an operator loses the rows around it and is told nothing about why.
func TestRecordsWriteAnswerFaultsARowNoLineCanCarry(t *testing.T) {
	t.Parallel()

	const id = 4
	wide := `"` + string(bytes.Repeat([]byte{'x'}, rpc.MaxMessageSize)) + `"`

	produced := 0
	rows := func(yield func(Record) bool) {
		for _, item := range []string{`{"peer":"10.0.0.1"}`, wide, `{"peer":"10.0.0.2"}`} {
			produced++
			if !yield(Record{Item: jsonRow(item)}) {
				return
			}
		}
	}

	var wire bytes.Buffer
	records := Records{Key: "peers", Rows: rows}
	require.NoError(t, records.WriteAnswer(&wire, id, rpc.StatusDone))

	assert.Equal(t, 3, produced, "the walk must continue past the row no line can carry")

	tails := answerTails(t, id, wire.Bytes())
	require.Len(t, tails, 3, "a walk under the threshold is a head, one document and a terminator")

	terminator := tails[2]
	require.True(t, terminator.IsTerminator(), "the last line must carry count=")
	assert.Equal(t, uint64(2), terminator.Count, "the two rows that fit are what the walk produced")
	assert.Equal(t, uint64(1), terminator.Faults, "the row no line can carry is counted under faults")

	// The operator reads the rows that were kept beside the row that was
	// rejected, which is what makes the rejection actionable.
	var document struct {
		Peers  []json.RawMessage `json:"peers"`
		Errors []struct {
			Message      string `json:"message"`
			Record       uint64 `json:"record"`
			EncodedBytes int64  `json:"encoded-bytes"`
			LimitBytes   int64  `json:"limit-bytes"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(tails[1].Item, &document))
	assert.Equal(t, []json.RawMessage{
		json.RawMessage(`{"peer":"10.0.0.1"}`),
		json.RawMessage(`{"peer":"10.0.0.2"}`),
	}, document.Peers)
	require.Len(t, document.Errors, 1)
	assert.Equal(t, uint64(2), document.Errors[0].Record, "the rejected row names its position in the walk")
	assert.Greater(t, document.Errors[0].EncodedBytes, int64(rpc.MaxMessageSize))
	assert.Equal(t, int64(rpc.MaxMessageSize), document.Errors[0].LimitBytes)
}

// TestRecordsMarshalJSONIsTheDocumentTheWireCollapsesTo checks that the two
// readings of one walk agree. The method: the same rows are written as an
// answer and marshaled as a value, and the document each produces is compared.
//
// The two readings exist because not every transport carries a line for each
// record: the direct bridge answers one marshaled value. They MUST be the same
// document, or an operator reads one answer in process and another over the
// socket for the same command.
//
// VALIDATES: AC-4 of spec-record-answers-1-sdk-path -- a handler answers with a
// walk and the rows reach the operator, whichever transport carried them.
// PREVENTS: a second collapse in the SDK, which is how the two transports come
// to disagree about the answer to one command.
func TestRecordsMarshalJSONIsTheDocumentTheWireCollapsesTo(t *testing.T) {
	t.Parallel()

	const id = 6
	items := []string{`{"peer":"10.0.0.1"}`, `{"peer":"10.0.0.2"}`}

	var wire bytes.Buffer
	require.NoError(t, Records{Key: "peers", Rows: rowsFrom(items...)}.WriteAnswer(&wire, id, rpc.StatusDone))
	tails := answerTails(t, id, wire.Bytes())
	require.Len(t, tails, 3)

	value, err := json.Marshal(Records{Key: "peers", Rows: rowsFrom(items...)})
	require.NoError(t, err)

	assert.Equal(t, string(tails[1].Item), string(value),
		"the document a bridge caller reads is the document the wire collapsed to")
	assert.Equal(t, `{"peers":[{"peer":"10.0.0.1"},{"peer":"10.0.0.2"}]}`, string(value))
}

// TestRecordsWithNoGeneratorIsAnEmptyCollection checks that a handler naming an
// envelope and producing nothing answers with an empty collection rather than
// panicking on a nil walk. The method: a Records carrying no Rows is written and
// marshaled.
//
// VALIDATES: the boundary of the row count, zero, on both readings of a walk.
// PREVENTS: a nil iter.Seq reaching a range, which ends the plugin process
// instead of answering the command.
func TestRecordsWithNoGeneratorIsAnEmptyCollection(t *testing.T) {
	t.Parallel()

	const id = 2

	var wire bytes.Buffer
	require.NoError(t, Records{Key: "peers"}.WriteAnswer(&wire, id, rpc.StatusDone))

	tails := answerTails(t, id, wire.Bytes())
	require.Len(t, tails, 3)
	assert.Equal(t, `{"peers":[]}`, string(tails[1].Item))
	assert.Equal(t, uint64(0), tails[2].Count)

	value, err := json.Marshal(Records{Key: "peers"})
	require.NoError(t, err)
	assert.Equal(t, `{"peers":[]}`, string(value))
}
