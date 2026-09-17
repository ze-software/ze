// Design: docs/architecture/api/process-protocol.md -- daemon-owned plugin state.
package rpc

import (
	"fmt"
	"io/fs"
	"strings"
)

const (
	MethodStateGet       = "ze-plugin-engine:state-get"
	MethodStatePut       = "ze-plugin-engine:state-put"
	MethodStateRemove    = "ze-plugin-engine:state-remove"
	MethodStateList      = "ze-plugin-engine:state-list"
	MethodStateIncrement = "ze-plugin-engine:state-increment"
	StateKeyMax          = 512
	StateDataMax         = 1024 * 1024
	StateListMax         = 4096
	StateMessageMax      = 4096
)

// StateStatus is the wire outcome. Empty is invalid and never means success.
type StateStatus string

const (
	StateOK            StateStatus = "ok"
	StateAbsent        StateStatus = "absent"
	StateUnavailable   StateStatus = "unavailable"
	StateCorrupt       StateStatus = "corrupt"
	StateReadFailed    StateStatus = "read-failed"
	StatePersistFailed StateStatus = "persist-failed"
)

// StateInput carries one raw registered state key, or a prefix for state-list.
// Data is base64 in JSON. A put stores zero bytes when data is empty.
type StateInput struct {
	Key  string `json:"key"`
	Data []byte `json:"data,omitempty"`
}

// StateOutput explicitly separates absence from a failed read or mutation.
type StateOutput struct {
	Status  StateStatus `json:"status"`
	Data    []byte      `json:"data,omitempty"`
	Keys    []string    `json:"keys,omitempty"`
	Value   uint32      `json:"value,omitempty"`
	Message string      `json:"message,omitempty"`
}

// StateError carries a failed state outcome, identically on both transports.
type StateError struct {
	Status  StateStatus
	Message string
}

func (e *StateError) Error() string {
	return fmt.Sprintf("runtime state %s: %s", e.Status, e.Message)
}

// ValidateStateInput enforces the YANG bounds on both sides of the transport.
func ValidateStateInput(in StateInput) error {
	if len(in.Key) > StateKeyMax {
		return fmt.Errorf("state key exceeds %d bytes", StateKeyMax)
	}
	if !strings.HasPrefix(in.Key, "meta/") {
		return fmt.Errorf("state key %q is outside metadata", in.Key)
	}
	if !fs.ValidPath(strings.TrimSuffix(in.Key, "/")) {
		return fmt.Errorf("invalid state key %q", in.Key)
	}
	for _, char := range in.Key {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case strings.ContainsRune("_.:/-", char):
		default:
			return fmt.Errorf("invalid character in state key %q", in.Key)
		}
	}
	if len(in.Data) > StateDataMax {
		return fmt.Errorf("state data exceeds %d bytes", StateDataMax)
	}
	return nil
}
