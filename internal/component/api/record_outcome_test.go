// Design: docs/architecture/api/architecture.md -- buffered command outcomes.
package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// REST and gRPC take the command's envelope after Execute returns. Its record
// walk must already have published its actual outcome, rather than changing
// Status while either transport encodes the Data field.
func TestEngineRecordOutcomeIsFinalBeforeReturn(t *testing.T) {
	for _, test := range []struct {
		name     string
		metadata rpc.ExecuteCommandRecordAnswer
		status   string
	}{
		{name: "healthy", metadata: rpc.ExecuteCommandRecordAnswer{Count: 1}, status: rpc.VerdictDone},
		{name: "partial", metadata: rpc.ExecuteCommandRecordAnswer{Count: 1, Faults: 1}, status: rpc.VerdictPartial},
		{name: "incomplete", metadata: rpc.ExecuteCommandRecordAnswer{Count: 1, Message: "evaluation stopped before acknowledgement"}, status: rpc.VerdictAborted},
		{name: "warning", metadata: rpc.ExecuteCommandRecordAnswer{Count: 1, Warning: true, Warnings: []string{"retry the retained cleanup"}}, status: rpc.VerdictWarning},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := plugin.NewResponse(plugin.StatusDone, nil)
			completed := false
			response.Data = plugin.Records{Key: "cleanup", Rows: func(yield func(rpc.RowRecord) bool) {
				item := rpc.RawRow(`{"phase":"complete-native"}`)
				if !yield(rpc.RowRecord{Item: &item}) {
					return
				}
				if test.metadata.Faults != 0 {
					fault := rpc.RawRow(`{"cause":"missing-native-ack"}`)
					if !yield(rpc.RowRecord{Fault: &fault}) {
						return
					}
				}
				require.NoError(t, response.CompleteRecordAnswer(&test.metadata))
				completed = true
			}}
			engine := NewAPIEngine(func(context.Context, CallerIdentity, string) (*plugin.Response, error) {
				return response, nil
			}, fakeCommands(), allowAllAuth(), nil)
			result, err := engine.Execute(t.Context(), &ExecuteRequest{Command: "bgp summary"})
			require.NoError(t, err)
			assert.Same(t, response, result, "the original response owns transport completion")
			assert.True(t, completed, "API returned before its record outcome arrived")
			assert.Equal(t, test.status, result.Status)
			assert.Equal(t, test.metadata.Message, result.Error)
			assert.Equal(t, test.metadata.Warnings, result.Warnings)
			_, generated := plugin.RecordRows(result)
			assert.False(t, generated, "a buffered API handed a one-shot walk to its transport")
		})
	}
}

// Serializing a buffered API result twice must not walk its producer twice.
// The original completion callback still belongs to the response transport.
func TestEngineRecordOutcomeIsMaterializedOnce(t *testing.T) {
	response := plugin.NewResponse(plugin.StatusDone, nil)
	walks, completed := 0, 0
	response.OnTransportComplete(func() { completed++ })
	response.Data = plugin.Records{Key: "cleanup", Rows: func(yield func(rpc.RowRecord) bool) {
		walks++
		item := rpc.RawRow(`{"phase":"complete-native"}`)
		if !yield(rpc.RowRecord{Item: &item}) {
			return
		}
		require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 1}))
	}}
	engine := NewAPIEngine(func(context.Context, CallerIdentity, string) (*plugin.Response, error) {
		return response, nil
	}, fakeCommands(), allowAllAuth(), nil)
	result, err := engine.Execute(t.Context(), &ExecuteRequest{Command: "bgp summary"})
	require.NoError(t, err)
	assert.Zero(t, completed, "buffering completed an unwritten transport")
	first, err := json.Marshal(result)
	require.NoError(t, err)
	second, err := json.Marshal(result)
	require.NoError(t, err)
	assert.JSONEq(t, string(first), string(second))
	assert.Contains(t, string(first), "complete-native")
	assert.Equal(t, 1, walks)
	result.TransportComplete()
	assert.Equal(t, 1, completed)
}

// A malformed record schema is refused before the transport can retry its
// one-shot producer. The original delivery action belongs to the refusal now.
func TestEngineRecordOutcomeRejectsMalformedBeforeTransport(t *testing.T) {
	walks, completed := 0, 0
	response := plugin.NewResponse(plugin.StatusDone, plugin.Records{
		Key:  rpc.AnswerErrorsKey,
		Rows: func(func(rpc.RowRecord) bool) { walks++ },
	})
	response.OnTransportComplete(func() { completed++ })
	engine := NewAPIEngine(func(context.Context, CallerIdentity, string) (*plugin.Response, error) {
		return response, nil
	}, fakeCommands(), allowAllAuth(), nil)
	result, err := engine.Execute(t.Context(), &ExecuteRequest{Command: "bgp summary"})
	require.ErrorIs(t, err, rpc.ErrReservedEnvelopeKey)
	assert.Equal(t, plugin.StatusError, result.Status)
	assert.Contains(t, result.Error, "Cannot return the result of bgp summary")
	assert.Contains(t, result.Error, "ze show errors")
	assert.Nil(t, result.Data)
	assert.Zero(t, walks)
	assert.Zero(t, completed)
	response.TransportComplete()
	assert.Zero(t, completed, "original response retained transferred completion")
	result.TransportComplete()
	assert.Equal(t, 1, completed)
}
