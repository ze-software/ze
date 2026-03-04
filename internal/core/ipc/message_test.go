package ipc

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestWireFormatRequest verifies Request JSON parsing and serialization.
//
// VALIDATES: Request fields match spec wire format (method, params, id, more).
// PREVENTS: Request deserialization failures breaking IPC dispatch.
func TestWireFormatRequest(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    rpc.Request
		wantErr bool
	}{
		{
			name: "simple_request",
			json: `{"method":"ze-bgp:peer-list"}`,
			want: rpc.Request{Method: "ze-bgp:peer-list"},
		},
		{
			name: "with_params",
			json: `{"method":"ze-bgp:peer-teardown","params":{"selector":"10.0.0.1","subcode":2}}`,
			want: rpc.Request{
				Method: "ze-bgp:peer-teardown",
				Params: json.RawMessage(`{"selector":"10.0.0.1","subcode":2}`),
			},
		},
		{
			name: "with_id_string",
			json: `{"method":"ze-bgp:peer-list","id":"abc"}`,
			want: rpc.Request{
				Method: "ze-bgp:peer-list",
				ID:     json.RawMessage(`"abc"`),
			},
		},
		{
			name: "with_id_number",
			json: `{"method":"ze-bgp:peer-list","id":42}`,
			want: rpc.Request{
				Method: "ze-bgp:peer-list",
				ID:     json.RawMessage(`42`),
			},
		},
		{
			name: "streaming_request",
			json: `{"method":"ze-bgp:subscribe","params":{"events":["update"]},"id":1,"more":true}`,
			want: rpc.Request{
				Method: "ze-bgp:subscribe",
				Params: json.RawMessage(`{"events":["update"]}`),
				ID:     json.RawMessage(`1`),
				More:   true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got rpc.Request
			err := json.Unmarshal([]byte(tc.json), &got)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want.Method, got.Method)
			assert.Equal(t, tc.want.More, got.More)

			// Compare RawMessage fields as JSON
			if tc.want.Params != nil {
				assert.JSONEq(t, string(tc.want.Params), string(got.Params))
			}
			if tc.want.ID != nil {
				assert.Equal(t, string(tc.want.ID), string(got.ID))
			}
		})
	}
}

// TestWireFormatResponse verifies RPCResult JSON formatting.
//
// VALIDATES: RPCResult fields match spec wire format (result, id, continues).
// PREVENTS: RPCResult serialization producing invalid JSON for clients.
func TestWireFormatResponse(t *testing.T) {
	tests := []struct {
		name string
		resp rpc.RPCResult
		want string
	}{
		{
			name: "simple_result",
			resp: rpc.RPCResult{
				Result: json.RawMessage(`{"peers":[]}`),
			},
			want: `{"result":{"peers":[]}}`,
		},
		{
			name: "with_id",
			resp: rpc.RPCResult{
				Result: json.RawMessage(`{"version":"0.1.0"}`),
				ID:     json.RawMessage(`1`),
			},
			want: `{"result":{"version":"0.1.0"},"id":1}`,
		},
		{
			name: "streaming_response",
			resp: rpc.RPCResult{
				Result:    json.RawMessage(`{"peer":"10.0.0.1"}`),
				ID:        json.RawMessage(`1`),
				Continues: true,
			},
			want: `{"result":{"peer":"10.0.0.1"},"id":1,"continues":true}`,
		},
		{
			name: "final_streaming_response",
			resp: rpc.RPCResult{
				Result: json.RawMessage(`{"peer":"10.0.0.2"}`),
				ID:     json.RawMessage(`1`),
			},
			want: `{"result":{"peer":"10.0.0.2"},"id":1}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.resp)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(data))
		})
	}
}

// TestWireFormatError verifies Error response JSON formatting.
//
// VALIDATES: Error fields match spec wire format (error, params, id).
// PREVENTS: Error responses not matching the expected wire format.
func TestWireFormatError(t *testing.T) {
	tests := []struct {
		name string
		resp rpc.RPCError
		want string
	}{
		{
			name: "simple_error",
			resp: rpc.RPCError{
				Error: "peer-not-found",
			},
			want: `{"error":"peer-not-found"}`,
		},
		{
			name: "error_with_params",
			resp: rpc.RPCError{
				Error:  "peer-not-found",
				Params: json.RawMessage(`{"address":"10.0.0.99"}`),
			},
			want: `{"error":"peer-not-found","params":{"address":"10.0.0.99"}}`,
		},
		{
			name: "error_with_id",
			resp: rpc.RPCError{
				Error: "invalid-parameter",
				ID:    json.RawMessage(`42`),
			},
			want: `{"error":"invalid-parameter","id":42}`,
		},
		{
			name: "error_with_all_fields",
			resp: rpc.RPCError{
				Error:  "invalid-family",
				Params: json.RawMessage(`{"family":"ipv99/unicast"}`),
				ID:     json.RawMessage(`"req-1"`),
			},
			want: `{"error":"invalid-family","params":{"family":"ipv99/unicast"},"id":"req-1"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.resp)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(data))
		})
	}
}

// TestWireFormatStreaming verifies more/continues flag semantics.
//
// VALIDATES: Streaming request (more:true) and response (continues:true) flags.
// PREVENTS: Streaming protocol breaking due to incorrect flag handling.
func TestWireFormatStreaming(t *testing.T) {
	// Client sends streaming request
	reqJSON := `{"method":"ze-bgp:subscribe","params":{"events":["update"]},"id":1,"more":true}`
	var req rpc.Request
	err := json.Unmarshal([]byte(reqJSON), &req)
	require.NoError(t, err)
	assert.True(t, req.More, "streaming request should have more=true")
	assert.Equal(t, "ze-bgp:subscribe", req.Method)

	// Server sends streaming response (continues=true)
	streamResp := rpc.RPCResult{
		Result:    json.RawMessage(`{"event":"update","peer":"10.0.0.1"}`),
		ID:        json.RawMessage(`1`),
		Continues: true,
	}
	data, err := json.Marshal(streamResp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"continues":true`)

	// Server sends final response (no continues)
	finalResp := rpc.RPCResult{
		Result: json.RawMessage(`{"event":"update","peer":"10.0.0.2"}`),
		ID:     json.RawMessage(`1`),
	}
	data, err = json.Marshal(finalResp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "continues")
}

// TestResponseMapping verifies mapping from current plugin.Response to IPC RPCResult/RPCError.
//
// VALIDATES: Current Response struct fields map correctly to JSON wire format.
// PREVENTS: Existing handler responses being incorrectly translated to IPC.
func TestResponseMapping(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		serial     string
		partial    bool
		data       any
		wantResp   bool // true = RPCResult, false = RPCError
		wantID     string
		wantCont   bool
		wantResult string
		wantError  string
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
			wantID:     "42",
			wantResult: `{"count":5}`,
		},
		{
			name:      "error_response",
			status:    "error",
			serial:    "7",
			data:      "peer not found",
			wantResp:  false,
			wantID:    "7",
			wantError: "peer-not-found",
		},
		{
			name:       "partial_streaming",
			status:     "done",
			serial:     "1",
			partial:    true,
			data:       map[string]any{"peer": "10.0.0.1"},
			wantResp:   true,
			wantID:     "1",
			wantCont:   true,
			wantResult: `{"peer":"10.0.0.1"}`,
		},
		{
			name:     "nil_data",
			status:   "done",
			data:     nil,
			wantResp: true,
		},
		{
			name:      "error_type_data",
			status:    "error",
			data:      fmt.Errorf("connection refused"),
			wantResp:  false,
			wantError: "connection-refused",
		},
		{
			name:      "error_numeric_data",
			status:    "error",
			data:      42,
			wantResp:  false,
			wantError: "42",
		},
		{
			name:       "non_numeric_serial",
			status:     "done",
			serial:     "hello",
			data:       map[string]any{"ok": true},
			wantResp:   true,
			wantID:     `"hello"`,
			wantResult: `{"ok":true}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := MapResponse(tc.status, tc.serial, tc.partial, tc.data)
			require.NotNil(t, result)

			if tc.wantResp {
				resp, ok := result.(*rpc.RPCResult)
				require.True(t, ok, "expected *RPCResult")
				assert.Equal(t, tc.wantCont, resp.Continues)
				if tc.wantID != "" {
					assert.Equal(t, tc.wantID, string(resp.ID))
				}
				if tc.wantResult != "" {
					assert.JSONEq(t, tc.wantResult, string(resp.Result))
				}
			} else {
				errResp, ok := result.(*rpc.RPCError)
				require.True(t, ok, "expected *RPCError type")
				assert.NotEmpty(t, errResp.Error)
				if tc.wantError != "" {
					assert.Equal(t, tc.wantError, errResp.Error)
				}
				if tc.wantID != "" {
					assert.Equal(t, tc.wantID, string(errResp.ID))
				}
			}
		})
	}
}

// TestRequestRoundTrip verifies Request JSON round-trip.
//
// VALIDATES: Request marshals and unmarshals to identical values.
// PREVENTS: Data loss during JSON serialization.
func TestRequestRoundTrip(t *testing.T) {
	original := rpc.Request{
		Method: "ze-bgp:peer-teardown",
		Params: json.RawMessage(`{"selector":"10.0.0.1","subcode":2}`),
		ID:     json.RawMessage(`"req-42"`),
		More:   true,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded rpc.Request
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Method, decoded.Method)
	assert.Equal(t, original.More, decoded.More)
	assert.JSONEq(t, string(original.Params), string(decoded.Params))
	assert.Equal(t, string(original.ID), string(decoded.ID))
}
