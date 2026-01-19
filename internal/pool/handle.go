// Package pool provides memory-efficient byte slice deduplication.
//
// The pool system stores unique byte sequences and returns compact handles
// for reference. Multiple routes sharing identical attributes will share
// the same underlying storage, reducing memory usage by 80-90% for route
// reflector scenarios.
//
// See .claude/zebgp/POOL_ARCHITECTURE.md for design details.
package pool

// Handle is a compact reference to pooled data.
//
// Uses MSB (bit 31) as buffer bit, bits 0-30 as slot index.
// This creates two distinct number spaces for the double-buffer design:
//   - Lower half (0x00000000-0x7FFFFFFE): buffer 0
//   - Upper half (0x80000000-0xFFFFFFFF): buffer 1
//
// During compaction, both handles remain valid until the old buffer
// is fully drained.
type Handle uint32

const (
	// InvalidHandle represents an invalid or uninitialized handle.
	// Uses max slot index in buffer 0.
	InvalidHandle Handle = 0x7FFFFFFF

	// BufferBitMask extracts the buffer bit (bit 31).
	BufferBitMask Handle = 0x80000000

	// SlotIndexMask extracts the slot index (bits 0-30).
	SlotIndexMask Handle = 0x7FFFFFFF
)

// MakeHandle creates a handle from slot index and buffer bit.
//
// The bufferBit should be 0 or 1. The slotIdx should be < 0x7FFFFFFF
// (InvalidHandle's slot index is reserved).
func MakeHandle(slotIdx uint32, bufferBit uint32) Handle {
	return Handle(slotIdx&uint32(SlotIndexMask)) | Handle(bufferBit<<31)
}

// SlotIndex returns the slot index (bits 0-30).
func (h Handle) SlotIndex() uint32 {
	return uint32(h & SlotIndexMask)
}

// BufferBit returns the buffer bit (0 or 1).
func (h Handle) BufferBit() uint32 {
	return uint32(h >> 31)
}

// IsLowerHalf returns true if handle is in buffer 0 (lower half).
func (h Handle) IsLowerHalf() bool {
	return h < BufferBitMask
}

// IsUpperHalf returns true if handle is in buffer 1 (upper half).
func (h Handle) IsUpperHalf() bool {
	return h >= BufferBitMask
}

// IsValid returns true if this is not InvalidHandle.
func (h Handle) IsValid() bool {
	return h != InvalidHandle
}
