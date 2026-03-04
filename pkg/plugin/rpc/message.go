// Design: docs/architecture/api/ipc_protocol.md — RPC wire message types
// Related: conn.go — Conn uses Request/RPCResult/RPCError for RPC framing
// Related: framing.go — NUL-delimited frame reader/writer
// Related: types.go — domain-specific RPC input/output types

package rpc

import "encoding/json"

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
