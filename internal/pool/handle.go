// Package pool provides zero-copy byte slice deduplication for BGP attributes and NLRI.
//
// The pool uses reference counting and periodic compaction to efficiently store
// deduplicated byte sequences. This is critical for memory efficiency when handling
// large RIBs where many routes share common attributes (e.g., AS_PATH, communities).
package pool

import "fmt"

// Handle is an opaque reference to data stored in a Pool.
// Handles are stable across compaction operations.
type Handle uint32

// InvalidHandle is the sentinel value indicating no valid handle.
// It uses the maximum uint32 value to avoid collision with valid slot indices.
const InvalidHandle Handle = 0xFFFFFFFF

// Valid returns true if the handle is not InvalidHandle.
func (h Handle) Valid() bool {
	return h != InvalidHandle
}

// String returns a string representation of the handle for debugging.
func (h Handle) String() string {
	if h == InvalidHandle {
		return "InvalidHandle"
	}
	return fmt.Sprintf("Handle(%d)", h)
}
