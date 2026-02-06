package ipc

import (
	"encoding/json"
	"fmt"
	"sync"
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
func (d *RPCDispatcher) Dispatch(req *Request) any {
	// Validate method format
	if _, _, err := ParseMethod(req.Method); err != nil {
		return &RPCError{
			Error: normalizeErrorName(err),
			ID:    req.ID,
		}
	}

	// Find handler
	d.mu.RLock()
	handler, exists := d.handlers[req.Method]
	d.mu.RUnlock()

	if !exists {
		return &RPCError{
			Error: "unknown-method",
			ID:    req.ID,
		}
	}

	// Call handler
	result, err := handler(req.Method, req.Params)
	if err != nil {
		return &RPCError{
			Error: normalizeErrorName(err),
			ID:    req.ID,
		}
	}

	// Marshal result
	var resultJSON json.RawMessage
	if result != nil {
		var marshalErr error
		resultJSON, marshalErr = json.Marshal(result)
		if marshalErr != nil {
			return &RPCError{
				Error: "marshal-error",
				ID:    req.ID,
			}
		}
	}

	return &RPCResult{
		Result: resultJSON,
		ID:     req.ID,
	}
}
