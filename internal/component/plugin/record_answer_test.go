// Design: docs/architecture/api/process-protocol.md -- command outcomes.
package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A transport receives an owned metadata snapshot, so mutating its copy cannot
// rewrite the outcome another transport later reads from the same response.
func TestRecordAnswerMetadataOwnsReturnedSnapshot(t *testing.T) {
	response := NewResponse(StatusDone, RawJSON(`{"cleanup":[]}`))
	require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
		Warning: true, Warnings: []string{"retained warning"},
	}))
	first, present := response.RecordAnswerMetadata()
	require.True(t, present)
	first.Count = 123
	first.Warning = false
	first.Warnings[0] = "replacement warning"
	second, present := response.RecordAnswerMetadata()
	require.True(t, present)
	assert.Zero(t, second.Count)
	assert.True(t, second.Warning)
	assert.Equal(t, []string{"retained warning"}, second.Warnings)
	assert.Equal(t, []string{"retained warning"}, response.Warnings)
}

// Absence means the transport has no completed original outcome, not that it
// can claim a successful zero-row command from a head or ordinary built data.
func TestRecordAnswerMetadataReportsNoOriginalOutcome(t *testing.T) {
	for _, response := range []*Response{nil, NewResponse(StatusDone, Map{"ready": true})} {
		metadata, present := response.RecordAnswerMetadata()
		assert.False(t, present)
		assert.Nil(t, metadata)
	}
}

// An already completed record answer may still carry structured progress for
// a socket surface to write. Its failure must not prevent that surface from
// receiving the typed response; the writer states its actual partial outcome.
func TestCommandDispatcherAnswerKeepsCompletedRecordProgress(t *testing.T) {
	response := NewResponse(StatusDone, Records{Key: "cleanup", Rows: func(yield func(rpc.RowRecord) bool) {
		item := rpc.RawRow(`{"id":1}`)
		if !yield(rpc.RowRecord{Item: &item}) {
			return
		}
		fault := rpc.RawRow(`{"id":2,"cause":"retained cleanup"}`)
		yield(rpc.RowRecord{Fault: &fault})
	}})
	require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 1, Faults: 1}))
	dispatcher := CommandDispatcher(func(context.Context, CallerIdentity, string) (*Response, error) {
		return response, nil
	})
	rendered, err := dispatcher.Answer(t.Context(), CallerIdentity{}, "show cleanup")
	require.NoError(t, err, "completed progress was discarded before the socket writer could consume it")
	require.Same(t, response, rendered.Response)
	_, generated := RecordRows(rendered.Response)
	require.True(t, generated)
	answer, err := AnswerFor(rendered.Response)
	require.NoError(t, err)
	document, err := rpc.CollapseAnswer(answer)
	require.NoError(t, err)
	assert.JSONEq(t, `{"cleanup":[{"id":1}],"errors":[{"id":2,"cause":"retained cleanup"}]}`, string(document))
	assert.Equal(t, rpc.VerdictPartial, answer.Verdict())
}
