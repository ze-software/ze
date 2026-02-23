package attrpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHandleValid verifies that Valid() correctly identifies valid handles.
//
// VALIDATES: Handle validity check works correctly with new 5-bit poolIdx.
//
// PREVENTS: Invalid handles being used in pool operations, causing
// out-of-bounds access or data corruption.
func TestHandleValid(t *testing.T) {
	tests := []struct {
		name  string
		h     Handle
		valid bool
	}{
		{"zero is valid", Handle(0), true},
		{"positive is valid", Handle(100), true},
		{"max valid handle (poolIdx=30)", NewHandle(30, 3, 0xFFFFFF), true},
		{"poolIdx=31 is invalid", NewHandleWithBuffer(0, 31, 0, 0), false},
		{"InvalidHandle is not valid", InvalidHandle, false},
		{"bufferBit=1 with valid poolIdx is valid", NewHandleWithBuffer(1, 5, 0, 100), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.h.IsValid())
		})
	}
}

// TestInvalidHandleConstant verifies InvalidHandle has expected value.
//
// VALIDATES: Sentinel value is correct.
//
// PREVENTS: Accidental collision with valid handle values.
func TestInvalidHandleConstant(t *testing.T) {
	assert.Equal(t, Handle(0xFFFFFFFF), InvalidHandle)
}

// TestHandleString verifies string representation for debugging.
//
// VALIDATES: Handle is printable for debugging.
//
// PREVENTS: Opaque values in logs making debugging difficult.
func TestHandleString(t *testing.T) {
	tests := []struct {
		h        Handle
		expected string
	}{
		{Handle(0), "Handle(0)"},
		{Handle(42), "Handle(42)"},
		{InvalidHandle, "InvalidHandle"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.h.String())
		})
	}
}

// TestHandleEncoding verifies bit-level encoding of bufferBit, poolIdx, flags, slot.
//
// VALIDATES: Handle correctly stores and retrieves all four fields.
// Layout: bufferBit(1 bit) | poolIdx(5 bits) | flags(2 bits) | slot(24 bits)
//
// PREVENTS: Bit masking errors causing field corruption or overlap.
func TestHandleEncoding(t *testing.T) {
	tests := []struct {
		name      string
		bufferBit uint32
		poolIdx   uint8
		flags     uint8
		slot      uint32
	}{
		{"zero values", 0, 0, 0, 0},
		{"max valid", 0, 30, 3, 0xFFFFFE},
		{"mid values", 0, 15, 1, 0x800000},
		{"slot boundary", 0, 0, 0, 0xFFFFFF},
		{"flags boundary", 0, 0, 3, 0},
		{"poolIdx boundary", 0, 30, 0, 0},
		{"bufferBit=1", 1, 5, 2, 1000},
		{"all fields set", 1, 30, 3, 0xFFFFFF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandleWithBuffer(tt.bufferBit, tt.poolIdx, tt.flags, tt.slot)
			assert.Equal(t, tt.bufferBit, h.BufferBit(), "bufferBit mismatch")
			assert.Equal(t, tt.poolIdx, h.PoolIdx(), "poolIdx mismatch")
			assert.Equal(t, tt.flags, h.Flags(), "flags mismatch")
			assert.Equal(t, tt.slot, h.Slot(), "slot mismatch")
		})
	}
}

// TestHandleInvalidHandleSentinel verifies InvalidHandle uses reserved poolIdx=31.
//
// VALIDATES: InvalidHandle sentinel uses reserved poolIdx.
//
// PREVENTS: Collision between valid handles and InvalidHandle.
func TestHandleInvalidHandleSentinel(t *testing.T) {
	// InvalidHandle must use reserved poolIdx=31
	assert.Equal(t, uint8(31), InvalidHandle.PoolIdx(), "InvalidHandle must use poolIdx=31")
	assert.Equal(t, uint32(1), InvalidHandle.BufferBit(), "InvalidHandle has bufferBit=1")
	assert.False(t, InvalidHandle.IsValid(), "InvalidHandle must be invalid")

	// Any handle with poolIdx < 31 is valid (regardless of bufferBit/flags/slot)
	h := NewHandle(0, 0, 0)
	assert.True(t, h.IsValid(), "poolIdx=0 should be valid")

	h = NewHandle(30, 3, 0xFFFFFE)
	assert.True(t, h.IsValid(), "poolIdx=30 should be valid")

	// bufferBit=1 doesn't affect validity
	h = NewHandleWithBuffer(1, 30, 3, 0xFFFFFE)
	assert.True(t, h.IsValid(), "poolIdx=30 with bufferBit=1 should be valid")
}

// TestHandleWithFlags verifies flag modification preserves other fields.
//
// VALIDATES: WithFlags only changes flags, preserves bufferBit, poolIdx and slot.
//
// PREVENTS: Flag modification corrupting other handle fields.
func TestHandleWithFlags(t *testing.T) {
	h := NewHandleWithBuffer(1, 5, 0, 1000)
	h2 := h.WithFlags(1)

	assert.Equal(t, uint32(1), h2.BufferBit(), "bufferBit should be preserved")
	assert.Equal(t, uint8(5), h2.PoolIdx(), "poolIdx should be preserved")
	assert.Equal(t, uint32(1000), h2.Slot(), "slot should be preserved")
	assert.Equal(t, uint8(1), h2.Flags(), "flags should be changed")

	// Test HasPathID (flag bit 0)
	assert.True(t, h2.HasPathID(), "flag bit 0 set should mean HasPathID")
	assert.False(t, h.HasPathID(), "original should not have path ID")
}

// TestHandleWithBufferBit verifies buffer bit modification preserves other fields.
//
// VALIDATES: WithBufferBit only changes bufferBit, preserves poolIdx, flags, and slot.
//
// PREVENTS: Buffer bit modification corrupting other handle fields.
func TestHandleWithBufferBit(t *testing.T) {
	h := NewHandleWithBuffer(0, 5, 2, 1000)
	h2 := h.WithBufferBit(1)

	assert.Equal(t, uint32(1), h2.BufferBit(), "bufferBit should be changed")
	assert.Equal(t, uint8(5), h2.PoolIdx(), "poolIdx should be preserved")
	assert.Equal(t, uint8(2), h2.Flags(), "flags should be preserved")
	assert.Equal(t, uint32(1000), h2.Slot(), "slot should be preserved")

	// Flip back
	h3 := h2.WithBufferBit(0)
	assert.Equal(t, uint32(0), h3.BufferBit(), "bufferBit should be changed back")
	assert.Equal(t, h.PoolIdx(), h3.PoolIdx(), "poolIdx should match original")
	assert.Equal(t, h.Flags(), h3.Flags(), "flags should match original")
	assert.Equal(t, h.Slot(), h3.Slot(), "slot should match original")
}

// TestHandleBoundary verifies boundary values for all fields.
//
// VALIDATES: Boundary values are correctly encoded/decoded.
// BOUNDARY: bufferBit 0-1, poolIdx 0-30, flags 0-3, slot 0-16777215.
//
// PREVENTS: Off-by-one errors at field boundaries.
func TestHandleBoundary(t *testing.T) {
	// bufferBit boundaries
	t.Run("bufferBit_0", func(t *testing.T) {
		h := NewHandleWithBuffer(0, 5, 0, 100)
		assert.Equal(t, uint32(0), h.BufferBit())
	})
	t.Run("bufferBit_1", func(t *testing.T) {
		h := NewHandleWithBuffer(1, 5, 0, 100)
		assert.Equal(t, uint32(1), h.BufferBit())
	})

	// poolIdx boundaries
	t.Run("poolIdx_last_valid_30", func(t *testing.T) {
		h := NewHandle(30, 0, 0)
		assert.True(t, h.IsValid())
		assert.Equal(t, uint8(30), h.PoolIdx())
	})
	t.Run("poolIdx_first_invalid_31", func(t *testing.T) {
		h := NewHandleWithBuffer(0, 31, 0, 0)
		assert.False(t, h.IsValid())
		assert.Equal(t, uint8(31), h.PoolIdx())
	})

	// flags boundaries
	t.Run("flags_last_valid_3", func(t *testing.T) {
		h := NewHandle(0, 3, 0)
		assert.Equal(t, uint8(3), h.Flags())
	})

	// slot boundaries
	t.Run("slot_max_0xFFFFFF_valid", func(t *testing.T) {
		// Full 24-bit range is usable for slot
		// Validity depends on poolIdx, not slot value
		h := NewHandle(0, 0, 0xFFFFFF)
		assert.Equal(t, uint32(0xFFFFFF), h.Slot())
		assert.True(t, h.IsValid()) // poolIdx=0 is valid
	})
}

// FuzzHandleRoundTrip verifies handle encoding/decoding is lossless.
//
// VALIDATES: All field values round-trip correctly through handle encoding.
// PREVENTS: Bit masking errors, field overlap, encoding corruption.
func FuzzHandleRoundTrip(f *testing.F) {
	// Seed corpus with boundary values
	f.Add(uint32(0), uint8(0), uint8(0), uint32(0))         // All zeros
	f.Add(uint32(1), uint8(30), uint8(3), uint32(0xFFFFFF)) // Max valid values
	f.Add(uint32(0), uint8(15), uint8(1), uint32(0x800000)) // Mid values
	f.Add(uint32(1), uint8(0), uint8(2), uint32(1000))      // Typical values

	f.Fuzz(func(t *testing.T, bufferBit uint32, poolIdx uint8, flags uint8, slot uint32) {
		// Normalize inputs to valid ranges
		bufferBit &= 1     // 0 or 1
		poolIdx %= 31      // 0-30 (31 reserved)
		flags &= 3         // 0-3
		slot &= 0x00FFFFFF // 24-bit

		h := NewHandleWithBuffer(bufferBit, poolIdx, flags, slot)

		// Verify round-trip
		if h.BufferBit() != bufferBit {
			t.Errorf("BufferBit mismatch: got %d, want %d", h.BufferBit(), bufferBit)
		}
		if h.PoolIdx() != poolIdx {
			t.Errorf("PoolIdx mismatch: got %d, want %d", h.PoolIdx(), poolIdx)
		}
		if h.Flags() != flags {
			t.Errorf("Flags mismatch: got %d, want %d", h.Flags(), flags)
		}
		if h.Slot() != slot {
			t.Errorf("Slot mismatch: got %d, want %d", h.Slot(), slot)
		}

		// Valid poolIdx (0-30) means valid handle
		if !h.IsValid() {
			t.Errorf("Handle should be valid with poolIdx=%d", poolIdx)
		}

		// WithFlags preserves other fields
		h2 := h.WithFlags((flags + 1) & 3)
		if h2.BufferBit() != bufferBit || h2.PoolIdx() != poolIdx || h2.Slot() != slot {
			t.Error("WithFlags corrupted other fields")
		}

		// WithBufferBit preserves other fields
		h3 := h.WithBufferBit(1 - bufferBit)
		if h3.PoolIdx() != poolIdx || h3.Flags() != flags || h3.Slot() != slot {
			t.Error("WithBufferBit corrupted other fields")
		}
	})
}

// FuzzInvalidHandle verifies poolIdx=31 always results in invalid handle.
//
// VALIDATES: Reserved poolIdx detection works regardless of other fields.
// PREVENTS: Invalid handles being treated as valid.
func FuzzInvalidHandle(f *testing.F) {
	f.Add(uint32(0), uint8(0), uint32(0))
	f.Add(uint32(1), uint8(3), uint32(0xFFFFFF))

	f.Fuzz(func(t *testing.T, bufferBit uint32, flags uint8, slot uint32) {
		bufferBit &= 1
		flags &= 3
		slot &= 0x00FFFFFF

		// poolIdx=31 should always be invalid
		h := NewHandleWithBuffer(bufferBit, 31, flags, slot)
		if h.IsValid() {
			t.Errorf("Handle with poolIdx=31 should be invalid: %v", h)
		}
	})
}
