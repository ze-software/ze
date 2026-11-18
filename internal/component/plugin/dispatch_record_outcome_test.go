// Design: docs/architecture/api/process-protocol.md -- command outcomes.
// Related: record_answer.go -- completed original record metadata.
// Related: dispatch.go -- in-process and wire response projections.

package plugin

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A completed answer with no document must preserve its warning and zero
// counts without manufacturing a public null document on either projection.
func TestDispatchRecordOutcomeAbsentDocumentStaysAbsent(t *testing.T) {
	lines := []string{"cleanup remains owned until its acknowledgement"}
	response := &Response{Status: StatusDone}
	require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
		Warning: true, Warnings: lines, DocumentAbsent: true,
	}))

	answer, err := AnswerFor(response)
	require.NoError(t, err)
	document, err := rpc.CollapseAnswer(answer)
	require.NoError(t, err)
	assert.Empty(t, document, "absent progress became a public document")
	assert.Equal(t, rpc.VerdictWarning, answer.Verdict())
	assert.Equal(t, lines, answer.Warnings())
	metadata, err := answer.RecordAnswerMetadata()
	require.NoError(t, err)
	assert.Zero(t, metadata.Count)
	assert.Zero(t, metadata.Faults)

	var wire bytes.Buffer
	require.NoError(t, WriteAnswer(&wire, 7, response))
	written := readAnswer(t, wire.Bytes())
	require.Len(t, written, 2, "absent document gained an item line")
	assert.Equal(t, rpc.AnswerTypeDocument, written[0].tail.Type)
	tail := written[1].tail
	assert.Equal(t, rpc.VerdictWarning, rpc.Verdict(&tail))
	assert.Zero(t, tail.Count)
	assert.Zero(t, tail.Faults)
	assert.Equal(t, lines, tail.Warnings)
	text, err := ResponseJSON(response, nil)
	require.NoError(t, err)
	assert.Empty(t, text, "text projection manufactured a null document")
}

// A real empty collection still has a document, while its completed record
// walk produced zero rows. Counting the document once loses that distinction.
func TestDispatchRecordOutcomeEmptyCollectionKeepsZeroCount(t *testing.T) {
	const empty = `{"cleanup":[]}`
	response := NewResponse(StatusDone, RawJSON(empty))
	require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{}))

	answer, err := AnswerFor(response)
	require.NoError(t, err)
	document, err := rpc.CollapseAnswer(answer)
	require.NoError(t, err)
	assert.JSONEq(t, empty, string(document))
	metadata, err := answer.RecordAnswerMetadata()
	require.NoError(t, err)
	assert.Zero(t, metadata.Count, "one empty document is not one produced row")
	assert.Zero(t, metadata.Faults)
	assert.Equal(t, rpc.VerdictDone, answer.Verdict())

	var wire bytes.Buffer
	require.NoError(t, WriteAnswer(&wire, 7, response))
	written := readAnswer(t, wire.Bytes())
	require.Len(t, written, 3, "real empty collection lost its document line")
	assert.JSONEq(t, empty, string(written[1].tail.Item))
	tail := written[2].tail
	assert.Zero(t, tail.Count)
	assert.Zero(t, tail.Faults)
	assert.Equal(t, rpc.VerdictDone, rpc.Verdict(&tail))
	text, err := ResponseJSON(response, nil)
	require.NoError(t, err)
	assert.JSONEq(t, empty, text)
}

// Refused rows outrank a warning, but the original warning kind and lines
// still belong beside that bounded document when the daemon writes it again.
func TestDispatchRecordOutcomeWarningSurvivesRefusedRows(t *testing.T) {
	const progress = `{"cleanup":[{"id":1},{"id":2}],"errors":[{"id":3,"cause":"acknowledgement missing"}]}`
	lines := []string{"retry the cleanup retained by its original owner"}
	response := NewResponse(StatusDone, RawJSON(progress))
	require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
		Count: 2, Faults: 1, Warning: true, Warnings: lines,
	}))

	answer, err := AnswerFor(response)
	require.NoError(t, err)
	document, err := rpc.CollapseAnswer(answer)
	require.NoError(t, err)
	assert.JSONEq(t, progress, string(document))
	assert.Equal(t, rpc.VerdictPartial, answer.Verdict())
	assert.Equal(t, lines, answer.Warnings())
	metadata, err := answer.RecordAnswerMetadata()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), metadata.Count)
	assert.Equal(t, uint64(1), metadata.Faults)
	assert.True(t, metadata.Warning)
	assert.Empty(t, metadata.Message, "a synthetic failure replaced the original empty message")

	var wire bytes.Buffer
	require.NoError(t, WriteAnswer(&wire, 7, response))
	written := readAnswer(t, wire.Bytes())
	require.Len(t, written, 3)
	assert.JSONEq(t, progress, string(written[1].tail.Item))
	tail := written[2].tail
	assert.Equal(t, uint64(2), tail.Count)
	assert.Equal(t, uint64(1), tail.Faults)
	assert.Equal(t, rpc.VerdictPartial, rpc.Verdict(&tail))
	assert.True(t, tail.Warning)
	assert.Equal(t, lines, tail.Warnings)
	assert.Empty(t, tail.Message)
}

// An original counted document that cannot fit the wire is replaced by one
// real rejection. The old counts describe inaccessible rows and must not be
// repeated over that substitute. The in-process sibling has no line limit.
func TestDispatchRecordOutcomeOversizedDocumentCountsActualWireRefusal(t *testing.T) {
	document := RawJSON(`{"blob":"` + strings.Repeat("x", rpc.MaxMessageSize) + `"}`)
	response := NewResponse(StatusDone, document)
	require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
		Count: 97, Faults: 3,
	}))

	answer, err := AnswerFor(response)
	require.NoError(t, err)
	var received int
	for record := range answer.Records {
		received++
		assert.Len(t, record.Item, len(document))
		assert.Empty(t, record.Fault)
	}
	assert.Equal(t, 1, received)
	metadata, err := answer.RecordAnswerMetadata()
	require.NoError(t, err)
	assert.Equal(t, uint64(97), metadata.Count)
	assert.Equal(t, uint64(3), metadata.Faults)
	assert.Equal(t, rpc.VerdictPartial, answer.Verdict())

	var wire bytes.Buffer
	require.NoError(t, WriteAnswer(&wire, 7, response))
	written := readAnswer(t, wire.Bytes())
	require.Len(t, written, 3)
	assert.Equal(t, rpc.AnswerTypeMap, written[0].tail.Type)
	assert.Empty(t, written[1].tail.Item)
	require.NotEmpty(t, written[1].tail.Fault)
	var refusal struct {
		Message string `json:"message"`
		Record  uint64 `json:"record"`
		Encoded int    `json:"encoded-bytes"`
		Limit   int    `json:"limit-bytes"`
	}
	require.NoError(t, json.Unmarshal(written[1].tail.Fault, &refusal))
	assert.Equal(t, "answer record does not fit one wire message", refusal.Message)
	assert.Equal(t, uint64(1), refusal.Record)
	assert.Greater(t, refusal.Encoded, rpc.MaxMessageSize)
	assert.Equal(t, rpc.MaxMessageSize, refusal.Limit)
	tail := written[2].tail
	assert.Zero(t, tail.Count, "discarded original rows were counted as delivered")
	assert.Equal(t, uint64(1), tail.Faults, "one substitute fault became original refused rows")
	assert.Equal(t, rpc.VerdictError, rpc.Verdict(&tail))
}

// Publishing a completed outcome takes ownership of its values. A caller
// retaining the raw DTO cannot rewrite counts, failure text or warning lines.
func TestDispatchRecordOutcomeOwnsItsCompletedMetadata(t *testing.T) {
	const progress = `{"cleanup":[{"id":1}]}`
	tests := []struct {
		name     string
		message  string
		warning  bool
		warnings []string
		verdict  string
	}{
		{name: "failure", message: "evaluation stopped before the final acknowledgement", verdict: rpc.VerdictAborted},
		{name: "warning", warning: true, warnings: []string{"retry the retained cleanup"}, verdict: rpc.VerdictWarning},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := &rpc.ExecuteCommandRecordAnswer{
				Count: 7, Message: test.message, Warning: test.warning,
				Warnings: append([]string(nil), test.warnings...),
			}
			response := NewResponse(StatusDone, RawJSON(progress))
			require.NoError(t, response.CompleteRecordAnswer(metadata))
			metadata.Count = 99
			metadata.Message = "replacement failure"
			metadata.Warning = !test.warning
			if len(metadata.Warnings) != 0 {
				metadata.Warnings[0] = "replacement warning"
			}
			assert.Equal(t, test.message, response.Error)
			assert.Equal(t, test.warnings, response.Warnings)

			answer, err := AnswerFor(response)
			require.NoError(t, err)
			document, err := rpc.CollapseAnswer(answer)
			require.NoError(t, err)
			assert.JSONEq(t, progress, string(document))
			assert.Equal(t, test.verdict, answer.Verdict())
			assert.Equal(t, test.message, answer.Message())
			assert.Equal(t, test.warnings, answer.Warnings())
			retained, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Equal(t, uint64(7), retained.Count)
			assert.Zero(t, retained.Faults)
			assert.Equal(t, test.warning, retained.Warning)

			var wire bytes.Buffer
			require.NoError(t, WriteAnswer(&wire, 7, response))
			written := readAnswer(t, wire.Bytes())
			require.Len(t, written, 3)
			tail := written[2].tail
			assert.Equal(t, uint64(7), tail.Count)
			assert.Equal(t, test.message, tail.Message)
			assert.Equal(t, test.warning, tail.Warning)
			assert.Equal(t, test.warnings, tail.Warnings)
		})
	}
}

// Refused completion metadata must not partially replace an already valid
// response. Every invalid shape is checked through the public publishing API.
func TestDispatchRecordOutcomeRefusedMetadataLeavesResponseUnchanged(t *testing.T) {
	tests := []struct {
		name     string
		absent   bool
		metadata *rpc.ExecuteCommandRecordAnswer
	}{
		{name: "nil metadata"},
		{name: "overflow", metadata: &rpc.ExecuteCommandRecordAnswer{Count: ^uint64(0), Faults: 1}},
		{name: "absent with original rows", metadata: &rpc.ExecuteCommandRecordAnswer{DocumentAbsent: true, Count: 1}},
		{name: "warning lines without warning", metadata: &rpc.ExecuteCommandRecordAnswer{Warnings: []string{"unowned warning"}}},
		{name: "absence over an actual document", metadata: &rpc.ExecuteCommandRecordAnswer{DocumentAbsent: true}},
		{name: "document over actual absence", absent: true, metadata: &rpc.ExecuteCommandRecordAnswer{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const progress = `{"cleanup":[{"id":1}]}`
			lines := []string{"the original warning"}
			response := NewResponse(StatusDone, RawJSON(progress))
			original := &rpc.ExecuteCommandRecordAnswer{Count: 7, Warning: true, Warnings: lines}
			if test.absent {
				response.Data = nil
				original.Count = 0
				original.DocumentAbsent = true
			}
			require.NoError(t, response.CompleteRecordAnswer(original))
			require.Error(t, response.CompleteRecordAnswer(test.metadata))
			assert.Equal(t, StatusWarning, response.Status)
			assert.Empty(t, response.Error)
			assert.Equal(t, lines, response.Warnings)
			answer, err := AnswerFor(response)
			require.NoError(t, err)
			document, err := rpc.CollapseAnswer(answer)
			require.NoError(t, err)
			if test.absent {
				assert.Nil(t, response.Data)
				assert.Empty(t, document)
			} else {
				assert.Equal(t, RawJSON(progress), response.Data)
				assert.JSONEq(t, progress, string(document))
			}
			retained, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Equal(t, original.Count, retained.Count)
			assert.Zero(t, retained.Faults)
			assert.Equal(t, rpc.VerdictWarning, answer.Verdict())
			assert.Equal(t, lines, answer.Warnings())
		})
	}
}

// A missing response is an invalid completion target, even when the supplied
// metadata itself describes a valid absent document. It must refuse, not panic.
func TestDispatchRecordOutcomeNilResponseRefusesCompletion(t *testing.T) {
	var response *Response
	require.Error(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{DocumentAbsent: true}))
}
