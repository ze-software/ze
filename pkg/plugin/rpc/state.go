// Design: docs/architecture/api/process-protocol.md -- daemon-owned plugin state.
package rpc

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/ze-software/ze/pkg/zefs"
)

const (
	MethodStateGet       = "ze-plugin-engine:state-get"
	MethodStatePut       = "ze-plugin-engine:state-put"
	MethodStateRemove    = "ze-plugin-engine:state-remove"
	MethodStateList      = "ze-plugin-engine:state-list"
	MethodStateIncrement = "ze-plugin-engine:state-increment"
	StateKeyMax          = 512
	// StateDataMax is the value bound of a key whose registration declares
	// none (zefs.KeyEntry.ValueMax zero); StateDataMaxFor gives one key's bound.
	StateDataMax    = 1024 * 1024
	StateListMax    = 4096
	StateMessageMax = 4096
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
	if limit := StateDataMaxFor(in.Key); len(in.Data) > limit {
		return fmt.Errorf("state data for %s is %d bytes, over its %d-byte bound", in.Key, len(in.Data), limit)
	}
	return nil
}

// StateDataMaxFor returns the value bound of one concrete key: the ValueMax its
// zefs registration declares, or StateDataMax when it declares none or no
// registration matches. The SDK and the daemon both bound a value with it, so
// a key's bound is declared once, on its registration.
func StateDataMaxFor(key string) int {
	entry, found := zefs.Lookup(key)
	if !found {
		return StateDataMax
	}
	if entry.ValueMax == 0 {
		return StateDataMax
	}
	return entry.ValueMax
}

// StateDataLargest returns the largest value bound any registered key carries,
// never less than StateDataMax. The daemon sizes its guard on an undecoded
// request with it, and ze-plugin-engine.yang models it as the state-data
// length (TestStateDataYANGLengthIsTheLargestBound compares the two).
func StateDataLargest() int {
	return max(StateDataMax, zefs.ValueMaxLargest())
}
