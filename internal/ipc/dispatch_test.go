package ipc

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRPCDispatchSimple verifies basic method routing to handler.
//
// VALIDATES: Registered handler receives dispatch and result is wrapped in RPCResult.
// PREVENTS: Dispatch table not routing methods to handlers.
func TestRPCDispatchSimple(t *testing.T) {
	d := NewRPCDispatcher()

	err := d.Register("ze-bgp:peer-list", func(_ string, _ json.RawMessage) (any, error) {
		return map[string]string{"status": "ok"}, nil
	})
	require.NoError(t, err)

	req := &Request{
		Method: "ze-bgp:peer-list",
		ID:     json.RawMessage(`1`),
	}

	resp := d.Dispatch(req)
	result, ok := resp.(*RPCResult)
	require.True(t, ok, "expected RPCResult, got %T", resp)
	assert.NotNil(t, result.Result)
	assert.Equal(t, json.RawMessage(`1`), result.ID)
}

// TestRPCDispatchWithParams verifies parameters are passed to handler.
//
// VALIDATES: Handler receives the request params from JSON.
// PREVENTS: Parameter loss during dispatch.
func TestRPCDispatchWithParams(t *testing.T) {
	d := NewRPCDispatcher()

	var receivedMethod string
	var receivedParams json.RawMessage
	err := d.Register("ze-bgp:peer-teardown", func(method string, params json.RawMessage) (any, error) {
		receivedMethod = method
		receivedParams = params
		return map[string]string{"status": "ok"}, nil
	})
	require.NoError(t, err)

	req := &Request{
		Method: "ze-bgp:peer-teardown",
		Params: json.RawMessage(`{"selector":"10.0.0.1","subcode":2}`),
		ID:     json.RawMessage(`2`),
	}

	resp := d.Dispatch(req)
	_, ok := resp.(*RPCResult)
	require.True(t, ok)
	assert.Equal(t, "ze-bgp:peer-teardown", receivedMethod)
	assert.JSONEq(t, `{"selector":"10.0.0.1","subcode":2}`, string(receivedParams))
}

// TestRPCDispatchUnknownMethod verifies error for unregistered method.
//
// VALIDATES: Unknown method returns RPCError with descriptive message.
// PREVENTS: Silent failures or panics on unknown methods.
func TestRPCDispatchUnknownMethod(t *testing.T) {
	d := NewRPCDispatcher()

	req := &Request{
		Method: "ze-bgp:nonexistent",
		ID:     json.RawMessage(`3`),
	}

	resp := d.Dispatch(req)
	errResp, ok := resp.(*RPCError)
	require.True(t, ok, "expected RPCError, got %T", resp)
	assert.Contains(t, errResp.Error, "unknown")
	assert.Equal(t, json.RawMessage(`3`), errResp.ID)
}

// TestRPCDispatchHandlerError verifies error from handler is formatted as RPCError.
//
// VALIDATES: Handler errors are converted to kebab-case RPCError.
// PREVENTS: Raw Go errors leaking to wire protocol.
func TestRPCDispatchHandlerError(t *testing.T) {
	d := NewRPCDispatcher()

	err := d.Register("ze-bgp:peer-teardown", func(_ string, _ json.RawMessage) (any, error) {
		return nil, errors.New("peer not found")
	})
	require.NoError(t, err)

	req := &Request{
		Method: "ze-bgp:peer-teardown",
		ID:     json.RawMessage(`4`),
	}

	resp := d.Dispatch(req)
	errResp, ok := resp.(*RPCError)
	require.True(t, ok, "expected RPCError, got %T", resp)
	assert.Equal(t, "peer-not-found", errResp.Error)
	assert.Equal(t, json.RawMessage(`4`), errResp.ID)
}

// TestRPCDispatchInvalidMethod verifies error for malformed method names.
//
// VALIDATES: Methods without colon separator or empty components are rejected.
// PREVENTS: Panics from bad method parsing.
func TestRPCDispatchInvalidMethod(t *testing.T) {
	d := NewRPCDispatcher()

	tests := []struct {
		name   string
		method string
	}{
		{"empty", ""},
		{"no-colon", "no-colon-separator"},
		{"empty-module", ":peer-list"},
		{"empty-rpc", "ze-bgp:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{
				Method: tt.method,
				ID:     json.RawMessage(`99`),
			}
			resp := d.Dispatch(req)
			errResp, ok := resp.(*RPCError)
			require.True(t, ok, "expected RPCError for method %q, got %T", tt.method, resp)
			assert.NotEmpty(t, errResp.Error)
		})
	}
}

// TestRPCDispatchDuplicateRegister verifies duplicate method registration is rejected.
//
// VALIDATES: Re-registering the same method returns error.
// PREVENTS: Silent handler shadowing.
func TestRPCDispatchDuplicateRegister(t *testing.T) {
	d := NewRPCDispatcher()

	handler := func(_ string, _ json.RawMessage) (any, error) { return "ok", nil }

	err := d.Register("ze-bgp:peer-list", handler)
	require.NoError(t, err)

	err = d.Register("ze-bgp:peer-list", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

// TestRPCDispatchHasMethod verifies method existence check.
//
// VALIDATES: HasMethod returns true for registered methods, false otherwise.
// PREVENTS: Dispatch attempts to unregistered methods.
func TestRPCDispatchHasMethod(t *testing.T) {
	d := NewRPCDispatcher()

	err := d.Register("ze-bgp:peer-list", func(_ string, _ json.RawMessage) (any, error) {
		return "ok", nil
	})
	require.NoError(t, err)

	assert.True(t, d.HasMethod("ze-bgp:peer-list"))
	assert.False(t, d.HasMethod("ze-bgp:nonexistent"))
}

// TestRPCDispatchNilResult verifies nil result from handler produces valid response.
//
// VALIDATES: Handler returning nil result produces RPCResult with null.
// PREVENTS: Nil pointer in result marshaling.
func TestRPCDispatchNilResult(t *testing.T) {
	d := NewRPCDispatcher()

	err := d.Register("ze-system:daemon-shutdown", func(_ string, _ json.RawMessage) (any, error) {
		return map[string]string{"status": "done"}, nil
	})
	require.NoError(t, err)

	req := &Request{
		Method: "ze-system:daemon-shutdown",
		ID:     json.RawMessage(`7`),
	}

	resp := d.Dispatch(req)
	result, ok := resp.(*RPCResult)
	require.True(t, ok)
	assert.Equal(t, json.RawMessage(`7`), result.ID)
}
