// Design: docs/architecture/api/process-protocol.md -- command outcomes.
package sdk

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: valid syntax cannot make contradictory metadata a successful result.
// Method: project dispatch values with invalid count/status/warning relations.
func TestDispatchRecordValueRefusesContradictoryMetadata(t *testing.T) {
	for _, metadata := range []string{
		`{"count":1,"faults":1}`,
		`{"count":18446744073709551615,"faults":1}`,
		`{"count":0,"faults":0,"warnings":["unmarked warning"]}`,
		`{"count":1,"faults":0,"document-absent":true}`,
	} {
		t.Run(metadata, func(t *testing.T) {
			var output rpc.DispatchCommandOutput
			require.NoError(t, json.Unmarshal([]byte(`{"status":"done","data":{"kept":true},"record-answer":`+metadata+`}`), &output))
			result, err := dispatchCommandResult(&output, nil)
			require.ErrorContains(t, err, "record")
			require.Equal(t, DispatchResult{}, result, "invalid metadata must not publish a command result")
		})
	}
}

// Goal: a returned error cannot discard a concurrently returned value.
// Method: call the existing direct projection with both and retain the cause.
func TestDispatchRecordValueKeepsOutputBesideError(t *testing.T) {
	cause := errors.New("original dispatch failure")
	output := &rpc.DispatchCommandOutput{Status: rpc.VerdictAborted, Data: json.RawMessage(`{"kept":true}`), Warnings: []string{"original warning"}}
	result, err := dispatchCommandResult(output, cause)
	require.ErrorIs(t, err, cause)
	require.Equal(t, output.Status, result.Status)
	require.JSONEq(t, string(output.Data), string(result.Data))
	require.Equal(t, output.Warnings, result.Warnings)
}

// Goal: a missing value is an explicit failure, rather than a nil dereference.
// Method: project nil output with and without an original dispatch error.
func TestDispatchRecordValueRefusesMissingOutput(t *testing.T) {
	t.Run("missing-value", func(t *testing.T) {
		var err error
		require.NotPanics(t, func() {
			_, err = dispatchCommandResult(nil, nil)
		})
		require.Error(t, err)
	})
	t.Run("original-error", func(t *testing.T) {
		cause := errors.New("original dispatch refusal")
		_, err := dispatchCommandResult(nil, cause)
		require.ErrorIs(t, err, cause)
	})
}

// Goal: an explicit failed built value cannot become a successful SDK result.
// Method: retain its data and require the shared unstated-failure message;
// successful and warning built values remain successful controls.
func TestDispatchRecordValueRefusesUnstatedError(t *testing.T) {
	for _, status := range []string{rpc.StatusError, rpc.StatusDone, rpc.StatusWarning} {
		t.Run(status, func(t *testing.T) {
			output := &rpc.DispatchCommandOutput{Status: status, Data: json.RawMessage(`{"retained":true}`)}
			result, err := dispatchCommandResult(output, nil)
			if status == rpc.StatusError {
				require.EqualError(t, err, rpc.AnswerFailureUnstated)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, status, result.Status)
			require.JSONEq(t, string(output.Data), string(result.Data))
		})
	}
}

// Goal: original-outcome metadata cannot describe a different value envelope.
// Method: refuse inconsistent document presence, message and warning fields.
func TestDispatchRecordValueRefusesContradictoryEnvelope(t *testing.T) {
	for _, wire := range []string{
		`{"status":"done","record-answer":{"count":1,"faults":0}}`,
		`{"status":"done","data":{},"record-answer":{"count":0,"faults":0,"document-absent":true}}`,
		`{"status":"aborted","data":{},"error":"changed","record-answer":{"count":1,"faults":0,"message":"original"}}`,
		`{"status":"warning","data":{},"warnings":["changed"],"record-answer":{"count":1,"faults":0,"warning":true,"warnings":["original"]}}`,
	} {
		t.Run(wire, func(t *testing.T) {
			var output rpc.DispatchCommandOutput
			require.NoError(t, json.Unmarshal([]byte(wire), &output))
			_, err := dispatchCommandResult(&output, nil)
			require.ErrorContains(t, err, "record")
		})
	}
}
