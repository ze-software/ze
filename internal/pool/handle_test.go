package pool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHandleValid verifies that Valid() correctly identifies valid handles.
//
// VALIDATES: Handle validity check works correctly.
//
// PREVENTS: Invalid handles being used in pool operations, causing
// out-of-bounds access or data corruption.
func TestHandleValid(t *testing.T) {
	// Max valid handle: poolIdx=62, flags=3, slot=0xFFFFFF = 0xFBFFFFFF
	tests := []struct {
		name  string
		h     Handle
		valid bool
	}{
		{"zero is valid", Handle(0), true},
		{"positive is valid", Handle(100), true},
		{"max valid handle (poolIdx=62)", NewHandle(62, 3, 0xFFFFFF), true},
		{"poolIdx=63 is invalid", NewHandle(63, 0, 0), false},
		{"InvalidHandle is not valid", InvalidHandle, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.h.Valid())
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

// TestHandleEncoding verifies bit-level encoding of poolIdx, flags, slot.
//
// VALIDATES: Handle correctly stores and retrieves all three fields.
// Layout: poolIdx(6 bits) | flags(2 bits) | slot(24 bits)
//
// PREVENTS: Bit masking errors causing field corruption or overlap.
func TestHandleEncoding(t *testing.T) {
	tests := []struct {
		name    string
		poolIdx uint8
		flags   uint8
		slot    uint32
	}{
		{"zero values", 0, 0, 0},
		{"max valid", 62, 3, 0xFFFFFE},
		{"mid values", 31, 1, 0x800000},
		{"slot boundary", 0, 0, 0xFFFFFE},
		{"flags boundary", 0, 3, 0},
		{"poolIdx boundary", 62, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandle(tt.poolIdx, tt.flags, tt.slot)
			assert.Equal(t, tt.poolIdx, h.PoolIdx(), "poolIdx mismatch")
			assert.Equal(t, tt.flags, h.Flags(), "flags mismatch")
			assert.Equal(t, tt.slot, h.Slot(), "slot mismatch")
		})
	}
}

// TestHandleInvalidHandleSentinel verifies InvalidHandle uses reserved poolIdx=63.
//
// VALIDATES: InvalidHandle sentinel uses reserved poolIdx.
//
// PREVENTS: Collision between valid handles and InvalidHandle.
func TestHandleInvalidHandleSentinel(t *testing.T) {
	// InvalidHandle must use reserved poolIdx=63
	assert.Equal(t, uint8(63), InvalidHandle.PoolIdx(), "InvalidHandle must use poolIdx=63")
	assert.False(t, InvalidHandle.Valid(), "InvalidHandle must be invalid")

	// Any handle with poolIdx < 63 is valid (regardless of flags/slot)
	h := NewHandle(0, 0, 0)
	assert.True(t, h.Valid(), "poolIdx=0 should be valid")

	h = NewHandle(62, 3, 0xFFFFFE)
	assert.True(t, h.Valid(), "poolIdx=62 should be valid")
}

// TestHandleWithFlags verifies flag modification preserves other fields.
//
// VALIDATES: WithFlags only changes flags, preserves poolIdx and slot.
//
// PREVENTS: Flag modification corrupting other handle fields.
func TestHandleWithFlags(t *testing.T) {
	h := NewHandle(5, 0, 1000)
	h2 := h.WithFlags(1)

	assert.Equal(t, uint8(5), h2.PoolIdx(), "poolIdx should be preserved")
	assert.Equal(t, uint32(1000), h2.Slot(), "slot should be preserved")
	assert.Equal(t, uint8(1), h2.Flags(), "flags should be changed")

	// Test HasPathID (flag bit 0)
	assert.True(t, h2.HasPathID(), "flag bit 0 set should mean HasPathID")
	assert.False(t, h.HasPathID(), "original should not have path ID")
}

// TestHandleBoundary verifies boundary values for all fields.
//
// VALIDATES: Boundary values are correctly encoded/decoded.
// BOUNDARY: poolIdx 0-62, flags 0-3, slot 0-16777214.
//
// PREVENTS: Off-by-one errors at field boundaries.
func TestHandleBoundary(t *testing.T) {
	// poolIdx boundaries
	t.Run("poolIdx_last_valid_62", func(t *testing.T) {
		h := NewHandle(62, 0, 0)
		assert.True(t, h.Valid())
		assert.Equal(t, uint8(62), h.PoolIdx())
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
		assert.True(t, h.Valid()) // poolIdx=0 is valid
	})
}
