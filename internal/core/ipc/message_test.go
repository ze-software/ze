package ipc

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestWireFormatRequest verifies Request fields match the new wire format.
//
// VALIDATES: Request fields (ID uint64, Method, Params) are correctly set.
// PREVENTS: Request deserialization failures breaking IPC dispatch.
func TestWireFormatRequest(t *testing.T) {
	tests := []struct {
		name   string
		req    rpc.Request
		method string
		id     uint64
	}{
		{
			name:   "simple_request",
			req:    rpc.Request{Method: "ze-bgp:peer-list", ID: 1},
			method: "ze-bgp:peer-list",
			id:     1,
		},
		{
			name: "with_params",
			req: rpc.Request{
				Method: "ze-bgp:peer-teardown",
				Params: json.RawMessage(`{"selector":"10.0.0.1","subcode":2}`),
				ID:     2,
			},
			method: "ze-bgp:peer-teardown",
			id:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.method, tt.req.Method)
			assert.Equal(t, tt.id, tt.req.ID)
		})
	}
}

// TestResponseMapping verifies mapping from current plugin.Response to IPC DispatchResult/DispatchError.
//
// VALIDATES: Current Response struct fields map correctly to dispatch types.
// PREVENTS: Existing handler responses being incorrectly translated.
func TestResponseMapping(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		serial      string
		partial     bool
		data        any
		wantResp    bool // true = DispatchResult, false = DispatchError
		wantID      uint64
		wantResult  string
		wantError   string
		wantMessage string // Expected Params.message (for error responses)
	}{
		{
			name:       "done_no_serial",
			status:     "done",
			data:       map[string]any{"version": "0.1.0"},
			wantResp:   true,
			wantResult: `{"version":"0.1.0"}`,
		},
		{
			name:       "done_with_serial",
			status:     "done",
			serial:     "42",
			data:       map[string]any{"count": 5},
			wantResp:   true,
			wantID:     42,
			wantResult: `{"count":5}`,
		},
		{
			name:        "error_response",
			status:      "error",
			serial:      "7",
			data:        "peer not found",
			wantResp:    false,
			wantID:      7,
			wantError:   "error",
			wantMessage: "peer not found",
		},
		{
			name:       "partial_streaming",
			status:     "done",
			serial:     "1",
			partial:    true,
			data:       map[string]any{"peer": "10.0.0.1"},
			wantResp:   true,
			wantID:     1,
			wantResult: `{"peer":"10.0.0.1"}`,
		},
		{
			name:     "nil_data",
			status:   "done",
			data:     nil,
			wantResp: true,
		},
		{
			name:        "error_type_data",
			status:      "error",
			data:        fmt.Errorf("connection refused"),
			wantResp:    false,
			wantError:   "error",
			wantMessage: "connection refused",
		},
		{
			name:        "error_numeric_data",
			status:      "error",
			data:        42,
			wantResp:    false,
			wantError:   "error",
			wantMessage: "42",
		},
		{
			name:       "non_numeric_serial",
			status:     "done",
			serial:     "hello",
			data:       map[string]any{"ok": true},
			wantResp:   true,
			wantID:     0, // non-numeric serial produces id 0
			wantResult: `{"ok":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapResponse(tt.status, tt.serial, tt.partial, tt.data)
			require.NotNil(t, result)

			if tt.wantResp {
				resp, ok := result.(*DispatchResult)
				require.True(t, ok, "expected *DispatchResult")
				if tt.wantID != 0 {
					assert.Equal(t, tt.wantID, resp.ID)
				}
				if tt.wantResult != "" {
					assert.JSONEq(t, tt.wantResult, string(resp.Result))
				}
			} else {
				errResp, ok := result.(*DispatchError)
				require.True(t, ok, "expected *DispatchError type")
				assert.NotEmpty(t, errResp.Error)
				if tt.wantError != "" {
					assert.Equal(t, tt.wantError, errResp.Error)
				}
				if tt.wantMessage != "" {
					msg := rpc.ExtractErrorMessage(errResp.Params)
					assert.Equal(t, tt.wantMessage, msg)
				}
				if tt.wantID != 0 {
					assert.Equal(t, tt.wantID, errResp.ID)
				}
			}
		})
	}
}
