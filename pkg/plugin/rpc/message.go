// Design: docs/architecture/api/ipc_protocol.md — RPC wire message types
// Related: conn.go — Conn uses Request/RPCResult/RPCError for RPC framing
// Related: framing.go — NUL-delimited frame reader/writer
// Related: types.go — domain-specific RPC input/output types

package rpc

import "encoding/json"

// NewError creates an RPCError with an explicit short code and human-readable detail.
// The code is a short kebab-case identifier (e.g., "unknown-command", "unauthorized").
// The message is preserved in Params.message for human display.
func NewError(id json.RawMessage, code, message string) *RPCError {
	params, _ := json.Marshal(map[string]string{"message": message})

	return &RPCError{
		Error:  code,
		Params: params,
		ID:     id,
	}
}

// CodedError is a Go error that carries a short machine-readable code.
// Used to pass structured error information through the dispatch chain
// so that Dispatch can construct an RPCError with a proper code.
type CodedError struct {
	Code    string // Short kebab-case identifier (e.g., "unknown-command")
	message string
}

// NewCodedError creates an error with a code and human-readable message.
func NewCodedError(code, message string) *CodedError {
	return &CodedError{Code: code, message: message}
}

func (e *CodedError) Error() string { return e.message }

// ExtractMessage extracts the human-readable message from RPCError params JSON.
// Returns the message if present, or empty string.
func ExtractMessage(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var detail struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(params, &detail) == nil {
		return detail.Message
	}
	return ""
}

// Request represents an IPC request on the wire.
// Method uses "module:rpc-name" format (e.g., "ze-bgp:peer-list").
type Request struct {
	Method string          `json:"method"`           // module:rpc-name
	Params json.RawMessage `json:"params,omitempty"` // Input parameters
	ID     json.RawMessage `json:"id,omitempty"`     // Correlation ID (string or number)
	More   bool            `json:"more,omitempty"`   // Request streaming responses
}

// RPCResult represents a successful IPC response on the wire.
type RPCResult struct {
	Result    json.RawMessage `json:"result"`              // Output data
	ID        json.RawMessage `json:"id,omitempty"`        // Echoed from request
	Continues bool            `json:"continues,omitempty"` // More responses follow
}

// RPCError represents an IPC error response on the wire.
type RPCError struct {
	Error  string          `json:"error"`            // Error identity name
	Params json.RawMessage `json:"params,omitempty"` // Error parameters
	ID     json.RawMessage `json:"id,omitempty"`     // Echoed from request
}
