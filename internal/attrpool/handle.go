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
//	┌─────────┬─────────┬───────┬────────────────────────┐
//	│BufferBit│ PoolIdx │ Flags │        Slot            │
//	│ (1 bit) │ (5 bits)│(2 bit)│      (24 bits)         │
//	└─────────┴─────────┴───────┴────────────────────────┘
//	 31        30    26  25   24  23                    0
//
// BufferBit: 0 or 1, indicates which buffer contains the data.
// PoolIdx: 0-30 valid, 31 reserved for InvalidHandle.
// Flags:   Bit 0 = hasPathID (ADD-PATH present), Bit 1 = reserved.
// Slot:    0 to 16,777,215 (0xFFFFFF). Full 24-bit range usable.
//
// InvalidHandle (0xFFFFFFFF) has poolIdx=31, making IsValid() return false.
// Any handle with poolIdx < 31 is valid regardless of bufferBit/flags/slot values.
type Handle uint32

// InvalidHandle is the sentinel value indicating no valid handle.
// Uses bufferBit=1, poolIdx=31 (reserved), flags=3, slot=0xFFFFFF.
const InvalidHandle Handle = 0xFFFFFFFF

// NewHandle creates a handle with the given poolIdx, flags, and slot.
// poolIdx must be 0-30 (31 is reserved for InvalidHandle).
// flags must be 0-3 (2 bits).
// slot must be 0-0xFFFFFF (24 bits).
// BufferBit defaults to 0.
func NewHandle(poolIdx, flags uint8, slot uint32) Handle {
	return Handle(
		uint32(poolIdx&0x1F)<<26 |
			uint32(flags&0x3)<<24 |
			(slot & 0x00FFFFFF),
	)
}

// NewHandleWithBuffer creates a handle with all fields including bufferBit.
// bufferBit must be 0 or 1.
// poolIdx must be 0-30 (31 is reserved for InvalidHandle).
// flags must be 0-3 (2 bits).
// slot must be 0-0xFFFFFF (24 bits).
func NewHandleWithBuffer(bufferBit uint32, poolIdx, flags uint8, slot uint32) Handle {
	return Handle(
		(bufferBit&0x1)<<31 |
			uint32(poolIdx&0x1F)<<26 |
			uint32(flags&0x3)<<24 |
			(slot & 0x00FFFFFF),
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

// Flags returns the flags field (2 bits, 0-3).
func (h Handle) Flags() uint8 {
	return uint8((h >> 24) & 0x3) //nolint:gosec // G115: Result is 2 bits max (0-3), always fits in uint8.
}

// Slot returns the slot index (24 bits, 0-0xFFFFFF).
func (h Handle) Slot() uint32 {
	return uint32(h) & 0x00FFFFFF
}

// HasPathID returns true if the ADD-PATH path ID flag is set (bit 0 of flags).
// RFC 7911: When ADD-PATH is negotiated, NLRI includes a path identifier.
func (h Handle) HasPathID() bool {
	return h.Flags()&1 != 0
}

// WithFlags returns a new handle with the given flags, preserving bufferBit, poolIdx, and slot.
func (h Handle) WithFlags(flags uint8) Handle {
	// Mask: keep bufferBit (bit 31), poolIdx (bits 30-26), and slot (bits 23-0), clear flags (bits 25-24)
	return Handle((uint32(h) & 0xFCFFFFFF) | uint32(flags&0x3)<<24)
}

// WithBufferBit returns a new handle with the given bufferBit, preserving poolIdx, flags, and slot.
func (h Handle) WithBufferBit(bit uint32) Handle {
	// Mask: keep poolIdx (bits 30-26), flags (bits 25-24), and slot (bits 23-0), clear bufferBit (bit 31)
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
