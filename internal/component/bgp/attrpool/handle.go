// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools
//
// Package pool provides zero-copy byte slice deduplication for BGP attributes and NLRI.
//
// The pool uses reference counting and periodic compaction to efficiently store
// deduplicated byte sequences. This is critical for memory efficiency when handling
// large RIBs where many routes share common attributes (e.g., AS_PATH, communities).
package attrpool

import "fmt"

// Handle is an opaque reference to data stored in a Pool.
// Handles are stable across compaction operations.
//
// Bit layout (32 bits total):
//
//	┌─────────┬─────────┬──────────────────────────────┐
//	│BufferBit│ PoolIdx │            Slot              │
//	│ (1 bit) │ (5 bits)│          (26 bits)           │
//	└─────────┴─────────┴──────────────────────────────┘
//	 31        30    26  25                            0
//
// BufferBit: 0 or 1, indicates which buffer contains the data.
// PoolIdx: 0-30 valid, 31 reserved for InvalidHandle.
// Slot:    0 to 67,108,863 (0x3FFFFFF). Full 26-bit range usable.
//
// InvalidHandle (0xFFFFFFFF) has poolIdx=31, making IsValid() return false.
// Any handle with poolIdx < 31 is valid regardless of bufferBit/slot values.
type Handle uint32

// InvalidHandle is the sentinel value indicating no valid handle.
// Uses bufferBit=1, poolIdx=31 (reserved), slot=0x3FFFFFF.
const InvalidHandle Handle = 0xFFFFFFFF

// NewHandle creates a handle with the given poolIdx and slot.
// poolIdx must be 0-30 (31 is reserved for InvalidHandle).
// slot must be 0-0x3FFFFFF (26 bits).
// BufferBit defaults to 0.
func NewHandle(poolIdx uint8, slot uint32) Handle {
	return Handle(
		uint32(poolIdx&0x1F)<<26 |
			(slot & 0x03FFFFFF),
	)
}

// NewHandleWithBuffer creates a handle with all fields including bufferBit.
// bufferBit must be 0 or 1.
// poolIdx must be 0-30 (31 is reserved for InvalidHandle).
// slot must be 0-0x3FFFFFF (26 bits).
func NewHandleWithBuffer(bufferBit uint32, poolIdx uint8, slot uint32) Handle {
	return Handle(
		(bufferBit&0x1)<<31 |
			uint32(poolIdx&0x1F)<<26 |
			(slot & 0x03FFFFFF),
	)
}

// BufferBit returns the buffer bit (0 or 1).
// Indicates which buffer contains the data during double-buffer compaction.
func (h Handle) BufferBit() uint32 {
	return uint32(h >> 31)
}

// PoolIdx returns the pool index (5 bits, 0-30 valid, 31 reserved).
func (h Handle) PoolIdx() uint8 {
	return uint8((h >> 26) & 0x1F) //nolint:gosec // G115: Result is 5 bits max (0-31), always fits in uint8.
}

// Slot returns the slot index (26 bits, 0-0x3FFFFFF).
func (h Handle) Slot() uint32 {
	return uint32(h) & 0x03FFFFFF
}

// WithBufferBit returns a new handle with the given bufferBit, preserving poolIdx and slot.
func (h Handle) WithBufferBit(bit uint32) Handle {
	// Mask: keep poolIdx (bits 30-26) and slot (bits 25-0), clear bufferBit (bit 31)
	return Handle((uint32(h) & 0x7FFFFFFF) | (bit&0x1)<<31)
}

// IsValid returns true if the handle has a valid poolIdx (0-30).
// PoolIdx=31 is reserved for InvalidHandle.
func (h Handle) IsValid() bool {
	return h.PoolIdx() < 31
}

// String returns a string representation of the handle for debugging.
func (h Handle) String() string {
	if h == InvalidHandle {
		return "InvalidHandle"
	}
	return fmt.Sprintf("Handle(%d)", h)
}
