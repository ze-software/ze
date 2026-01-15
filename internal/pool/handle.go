// Package pool provides zero-copy byte slice deduplication for BGP attributes and NLRI.
//
// The pool uses reference counting and periodic compaction to efficiently store
// deduplicated byte sequences. This is critical for memory efficiency when handling
// large RIBs where many routes share common attributes (e.g., AS_PATH, communities).
package pool

import "fmt"

// Handle is an opaque reference to data stored in a Pool.
// Handles are stable across compaction operations.
//
// Bit layout (32 bits total):
//
//	┌──────────┬───────┬────────────────────────┐
//	│PoolIdx   │ Flags │        Slot            │
//	│ (6 bits) │(2 bit)│      (24 bits)         │
//	└──────────┴───────┴────────────────────────┘
//	 31     26 25   24 23                      0
//
// PoolIdx: 0-62 valid, 63 reserved for InvalidHandle.
// Flags:   Bit 0 = hasPathID (ADD-PATH present), Bit 1 = reserved.
// Slot:    0 to 16,777,215 (0xFFFFFF).
type Handle uint32

// InvalidHandle is the sentinel value indicating no valid handle.
// Uses poolIdx=63 (reserved), flags=3, slot=0xFFFFFF.
const InvalidHandle Handle = 0xFFFFFFFF

// NewHandle creates a handle with the given poolIdx, flags, and slot.
// poolIdx must be 0-62 (63 is reserved for InvalidHandle).
// flags must be 0-3 (2 bits).
// slot must be 0-0xFFFFFF (24 bits).
func NewHandle(poolIdx uint8, flags uint8, slot uint32) Handle {
	return Handle(
		uint32(poolIdx&0x3F)<<26 |
			uint32(flags&0x3)<<24 |
			(slot & 0x00FFFFFF),
	)
}

// PoolIdx returns the pool index (6 bits, 0-62 valid, 63 reserved).
func (h Handle) PoolIdx() uint8 {
	return uint8(h >> 26) //nolint:gosec // G115: Result is 6 bits max (0-63), always fits in uint8.
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

// WithFlags returns a new handle with the given flags, preserving poolIdx and slot.
func (h Handle) WithFlags(flags uint8) Handle {
	// Mask: keep poolIdx (bits 31-26) and slot (bits 23-0), clear flags (bits 25-24)
	return Handle((uint32(h) & 0xFCFFFFFF) | uint32(flags&0x3)<<24)
}

// Valid returns true if the handle has a valid poolIdx (0-62).
// PoolIdx=63 is reserved for InvalidHandle.
func (h Handle) Valid() bool {
	return h.PoolIdx() < 63
}

// String returns a string representation of the handle for debugging.
func (h Handle) String() string {
	if h == InvalidHandle {
		return "InvalidHandle"
	}
	return fmt.Sprintf("Handle(%d)", h)
}
