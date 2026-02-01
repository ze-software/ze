package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponseJSONHasTypeWrapper verifies Response JSON includes type wrapper.
//
// VALIDATES: Response JSON has "type":"response" and "response":{...} structure.
// PREVENTS: Plugins expecting new ze-bgp JSON format failing to parse responses.
func TestResponseJSONHasTypeWrapper(t *testing.T) {
	resp := &Response{
		Serial: "123",
		Status: "done",
		Data:   map[string]any{"message": "ok"},
	}

	// Wrap the response
	wrapped := WrapResponse(resp)

	// Marshal to JSON
	data, err := json.Marshal(wrapped)
	require.NoError(t, err)

	// Parse to verify structure
	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Check top-level type field
	assert.Equal(t, "response", result["type"], "top-level type must be 'response'")

	// Check response object exists
	respObj, ok := result["response"].(map[string]any)
	require.True(t, ok, "response field must be object")

	// Check nested fields
	assert.Equal(t, "123", respObj["serial"])
	assert.Equal(t, "done", respObj["status"])
	assert.NotNil(t, respObj["data"])
}

// TestResponseMarshalFormat verifies full response wrapper JSON format.
//
// VALIDATES: Full response format matches IPC protocol spec.
// PREVENTS: Format deviations from docs/architecture/api/ipc_protocol.md.
func TestResponseMarshalFormat(t *testing.T) {
	tests := []struct {
		name     string
		resp     *Response
		wantType string
		wantKeys []string
	}{
		{
			name: "done_with_data",
			resp: &Response{
				Serial: "1",
				Status: "done",
				Data:   map[string]any{"count": 42},
			},
			wantType: "response",
			wantKeys: []string{"serial", "status", "data"},
		},
		{
			name: "error_response",
			resp: &Response{
				Serial: "2",
				Status: "error",
				Data:   "something went wrong",
			},
			wantType: "response",
			wantKeys: []string{"serial", "status", "data"},
		},
		{
			name: "partial_streaming",
			resp: &Response{
				Serial:  "3",
				Status:  "ack",
				Partial: true,
				Data:    map[string]any{"chunk": 1},
			},
			wantType: "response",
			wantKeys: []string{"serial", "status", "partial", "data"},
		},
		{
			name: "no_serial",
			resp: &Response{
				Status: "done",
			},
			wantType: "response",
			wantKeys: []string{"status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapResponse(tt.resp)
			data, err := json.Marshal(wrapped)
			require.NoError(t, err)

			var result map[string]any
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)

			// Top-level must have only "type" and "response" keys
			assert.Equal(t, tt.wantType, result["type"])
			assert.Contains(t, result, "response")
			assert.Len(t, result, 2, "top-level should only have 'type' and 'response'")

			// Check nested response has expected keys
			respObj := result["response"].(map[string]any) //nolint:forcetypeassert // test
			for _, key := range tt.wantKeys {
				assert.Contains(t, respObj, key, "response should contain %s", key)
			}
		})
	}
}

// TestCBOREncodingRemoved verifies CBOR encoding is no longer accepted.
//
// VALIDATES: ParseWireEncoding rejects "cbor" with error.
// PREVENTS: CBOR being accidentally re-added (incompatible with line-delimited protocol).
func TestCBOREncodingRemoved(t *testing.T) {
	_, err := ParseWireEncoding("cbor")
	require.Error(t, err, "CBOR encoding should not be accepted")
	assert.Contains(t, err.Error(), "invalid wire encoding")
}

// TestNewResponseHelpers verifies response helper functions.
//
// VALIDATES: NewResponse and NewErrorResponse create correct structures.
// PREVENTS: Helper functions not setting required fields.
func TestNewResponseHelpers(t *testing.T) {
	t.Run("NewResponse", func(t *testing.T) {
		resp := NewResponse("done", map[string]any{"ok": true})
		assert.Equal(t, "done", resp.Status)
		assert.Equal(t, map[string]any{"ok": true}, resp.Data)
	})

	t.Run("NewErrorResponse", func(t *testing.T) {
		resp := NewErrorResponse("failed to connect")
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, "failed to connect", resp.Data)
	})
}
