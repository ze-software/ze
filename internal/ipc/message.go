package ipc

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

// MapResponse converts the current plugin Response fields to an IPC wire message.
// Returns either *RPCResult (success) or *RPCError (error).
func MapResponse(status, serial string, partial bool, data any) any {
	var id json.RawMessage
	if serial != "" {
		// Validate serial is a valid JSON number before using as raw JSON
		if json.Valid([]byte(serial)) {
			id = json.RawMessage(serial)
		} else {
			// Quote invalid values as JSON strings to prevent corrupt output
			quoted, _ := json.Marshal(serial)
			id = json.RawMessage(quoted)
		}
	}

	if status == "error" {
		errMsg := normalizeErrorName(data)
		resp := &RPCError{
			Error: errMsg,
			ID:    id,
		}
		return resp
	}

	// Marshal data to JSON for the result field
	var result json.RawMessage
	if data != nil {
		var err error
		result, err = json.Marshal(data)
		if err != nil {
			errObj := map[string]string{"marshal-error": err.Error()}
			result, _ = json.Marshal(errObj)
		}
	}

	resp := &RPCResult{
		Result:    result,
		ID:        id,
		Continues: partial,
	}
	return resp
}

// normalizeErrorName converts an error data value to a kebab-case error identity.
func normalizeErrorName(data any) string {
	msg := fmt.Sprintf("%v", data)
	if s, ok := data.(string); ok {
		msg = s
	}
	if e, ok := data.(error); ok {
		msg = e.Error()
	}
	// Convert spaces to hyphens for kebab-case
	msg = strings.ToLower(msg)
	msg = strings.ReplaceAll(msg, " ", "-")
	return msg
}
