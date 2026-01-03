package pool

import (
	"testing"
)

// TestHandleMakeAndExtract verifies handle creation and field extraction.
//
// VALIDATES: Handle correctly encodes buffer bit and slot index
// PREVENTS: Bit manipulation errors in handle encoding.
func TestHandleMakeAndExtract(t *testing.T) {
	tests := []struct {
		name      string
		slotIdx   uint32
		bufferBit uint32
		wantSlot  uint32
		wantBit   uint32
	}{
		{"slot 0 buffer 0", 0, 0, 0, 0},
		{"slot 0 buffer 1", 0, 1, 0, 1},
		{"slot 5 buffer 0", 5, 0, 5, 0},
		{"slot 5 buffer 1", 5, 1, 5, 1},
		{"max slot buffer 0", 0x7FFFFFFE, 0, 0x7FFFFFFE, 0},
		{"max slot buffer 1", 0x7FFFFFFE, 1, 0x7FFFFFFE, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := MakeHandle(tt.slotIdx, tt.bufferBit)

			if got := h.SlotIndex(); got != tt.wantSlot {
				t.Errorf("SlotIndex() = %d, want %d", got, tt.wantSlot)
			}
			if got := h.BufferBit(); got != tt.wantBit {
				t.Errorf("BufferBit() = %d, want %d", got, tt.wantBit)
			}
		})
	}
}

// TestHandleNumberSpace verifies the MSB design creates distinct number spaces.
//
// VALIDATES: Buffer 0 handles are in lower half, buffer 1 in upper half
// PREVENTS: Handle space overlap between buffers.
func TestHandleNumberSpace(t *testing.T) {
	// Buffer 0 handles should be in lower half (< 0x80000000)
	h0 := MakeHandle(100, 0)
	if h0 >= BufferBitMask {
		t.Errorf("Buffer 0 handle %#x should be < %#x", h0, BufferBitMask)
	}
	if !h0.IsLowerHalf() {
		t.Error("Buffer 0 handle should be in lower half")
	}

	// Buffer 1 handles should be in upper half (>= 0x80000000)
	h1 := MakeHandle(100, 1)
	if h1 < BufferBitMask {
		t.Errorf("Buffer 1 handle %#x should be >= %#x", h1, BufferBitMask)
	}
	if !h1.IsUpperHalf() {
		t.Error("Buffer 1 handle should be in upper half")
	}

	// Same slot index should be extractable from both
	if h0.SlotIndex() != h1.SlotIndex() {
		t.Errorf("Slot indices should match: %d vs %d", h0.SlotIndex(), h1.SlotIndex())
	}
}

// TestInvalidHandle verifies the invalid handle constant.
//
// VALIDATES: InvalidHandle is recognizable and in lower half
// PREVENTS: Using valid slot index as invalid marker.
func TestInvalidHandle(t *testing.T) {
	if InvalidHandle.SlotIndex() != 0x7FFFFFFF {
		t.Errorf("InvalidHandle slot should be max value, got %#x", InvalidHandle.SlotIndex())
	}
	if InvalidHandle.BufferBit() != 0 {
		t.Error("InvalidHandle should be in buffer 0 (lower half)")
	}
}
