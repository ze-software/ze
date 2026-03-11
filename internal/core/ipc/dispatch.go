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
// Returns result data (marshaled into RPCResult) or an error (converted to RPCError).
type RPCHandler func(method string, params json.RawMessage) (any, error)

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

// Dispatch routes a Request to the registered handler and returns an RPCResult or RPCError.
func (d *RPCDispatcher) Dispatch(req *rpc.Request) any {
	// Validate method format
	if _, _, err := ParseMethod(req.Method); err != nil {
		return rpc.NewError(req.ID, "invalid-method", err.Error())
	}

	// Find handler
	d.mu.RLock()
	handler, exists := d.handlers[req.Method]
	d.mu.RUnlock()

	if !exists {
		return rpc.NewError(req.ID, "unknown-method", "unknown method")
	}

	// Call handler
	result, err := handler(req.Method, req.Params)
	if err != nil {
		// If the handler returned a CodedError, use its explicit code.
		// Otherwise fall back to a generic "handler-error" code.
		var coded *rpc.CodedError
		if errors.As(err, &coded) {
			return rpc.NewError(req.ID, coded.Code, coded.Error())
		}
		return rpc.NewError(req.ID, "handler-error", err.Error())
	}

	// Marshal result
	var resultJSON json.RawMessage
	if result != nil {
		var marshalErr error
		resultJSON, marshalErr = json.Marshal(result)
		if marshalErr != nil {
			return rpc.NewError(req.ID, "marshal-error", marshalErr.Error())
		}
	}

	return &rpc.RPCResult{
		Result: resultJSON,
		ID:     req.ID,
	}
}
