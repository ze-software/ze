// Design: docs/architecture/api/ipc_protocol.md — IPC framing and dispatch

package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// RPCHandler processes an RPC method call.
// Receives the full wire method name and raw JSON params.
// Returns result data (marshaled into DispatchResult) or an error (converted to DispatchError).
type RPCHandler func(method string, params json.RawMessage) (any, error)

// DispatchResult holds a successful dispatch result for wire serialization.
type DispatchResult struct {
	Result json.RawMessage // JSON-encoded result data
	ID     uint64          // Correlation ID
}

// DispatchError holds a dispatch error for wire serialization.
type DispatchError struct {
	Error  string          // Short error code (e.g., "unknown-method", "handler-error")
	Params json.RawMessage // Structured error detail ({"code":"...","message":"..."})
	ID     uint64          // Correlation ID
}

// RPCDispatcher routes wire method calls to registered handlers.
// Methods use "module:rpc-name" format (e.g., "ze-bgp:peer-list").
type RPCDispatcher struct {
	handlers map[string]RPCHandler
	mu       sync.RWMutex
}

// NewRPCDispatcher creates a new RPC dispatcher.
func NewRPCDispatcher() *RPCDispatcher {
	return &RPCDispatcher{
		handlers: make(map[string]RPCHandler),
	}
}

// Register adds a handler for a wire method.
// The method must be valid "module:rpc-name" format.
func (d *RPCDispatcher) Register(method string, handler RPCHandler) error {
	if _, _, err := ParseMethod(method); err != nil {
		return fmt.Errorf("invalid method: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.handlers[method]; exists {
		return fmt.Errorf("method already registered: %s", method)
	}

	d.handlers[method] = handler
	return nil
}

// HasMethod returns true if a handler is registered for the method.
func (d *RPCDispatcher) HasMethod(method string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, exists := d.handlers[method]
	return exists
}

// newDispatchError creates a DispatchError with structured detail payload.
func newDispatchError(id uint64, code, message string) *DispatchError {
	return &DispatchError{
		Error:  code,
		Params: rpc.NewErrorPayload(code, message),
		ID:     id,
	}
}

// Dispatch routes a Request to the registered handler and returns a DispatchResult or DispatchError.
func (d *RPCDispatcher) Dispatch(req *rpc.Request) any {
	// Validate method format
	if _, _, err := ParseMethod(req.Method); err != nil {
		return newDispatchError(req.ID, "invalid-method", err.Error())
	}

	// Find handler
	d.mu.RLock()
	handler, exists := d.handlers[req.Method]
	d.mu.RUnlock()

	if !exists {
		return newDispatchError(req.ID, "unknown-method", "unknown method")
	}

	// Call handler
	result, err := handler(req.Method, req.Params)
	if err != nil {
		// If the handler returned a CodedError, use its explicit code.
		// Otherwise fall back to a generic "handler-error" code.
		var coded *rpc.CodedError
		if errors.As(err, &coded) {
			return newDispatchError(req.ID, coded.Code, coded.Error())
		}
		return newDispatchError(req.ID, "handler-error", err.Error())
	}

	// Marshal result
	var resultJSON json.RawMessage
	if result != nil {
		var marshalErr error
		resultJSON, marshalErr = json.Marshal(result)
		if marshalErr != nil {
			return newDispatchError(req.ID, "marshal-error", marshalErr.Error())
		}
	}

	return &DispatchResult{
		Result: resultJSON,
		ID:     req.ID,
	}
}
