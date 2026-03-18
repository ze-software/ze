// Design: docs/architecture/api/ipc_protocol.md — response mapping and error normalization

package ipc

import (
	"encoding/json"
	"fmt"
	"strconv"

	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// MapResponse converts the current plugin Response fields to an IPC wire message.
// Returns either *DispatchResult (success) or *DispatchError (error).
func MapResponse(status, serial string, partial bool, data any) any {
	var id uint64
	if serial != "" {
		// Parse serial as uint64. If invalid, use 0.
		parsed, parseErr := strconv.ParseUint(serial, 10, 64)
		if parseErr == nil {
			id = parsed
		}
	}

	if status == "error" {
		msg := fmt.Sprintf("%v", data)
		if s, ok := data.(string); ok {
			msg = s
		}
		if e, ok := data.(error); ok {
			msg = e.Error()
		}
		return &DispatchError{
			Error:  "error",
			Params: rpc.NewErrorPayload("error", msg),
			ID:     id,
		}
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

	resp := &DispatchResult{
		Result: result,
		ID:     id,
	}
	_ = partial // partial/streaming no longer used in new wire format
	return resp
}
