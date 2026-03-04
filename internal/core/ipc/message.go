// Design: docs/architecture/api/ipc_protocol.md — response mapping and error normalization

package ipc

import (
	"encoding/json"
	"fmt"
	"strings"

	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// MapResponse converts the current plugin Response fields to an IPC wire message.
// Returns either *rpc.RPCResult (success) or *rpc.RPCError (error).
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
		resp := &rpc.RPCError{
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

	resp := &rpc.RPCResult{
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
