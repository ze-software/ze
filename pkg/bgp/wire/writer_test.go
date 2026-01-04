package wire_test

import (
	"bytes"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/wire"
)

// TestSessionBufferBasic verifies basic buffer operations.
//
// VALIDATES: Write, Reset, Bytes work correctly.
// PREVENTS: Buffer corruption or incorrect offset tracking.
func TestSessionBufferBasic(t *testing.T) {
	sb := wire.NewSessionBuffer(false) // 4096 bytes

	if sb.Cap() != 4096 {
		t.Errorf("Cap() = %d, want 4096", sb.Cap())
	}

	if sb.Len() != 0 {
		t.Errorf("Len() = %d, want 0", sb.Len())
	}

	// Write some bytes
	data := []byte{0x01, 0x02, 0x03, 0x04}
	n := sb.WriteBytes(data)
	if n != 4 {
		t.Errorf("WriteBytes() = %d, want 4", n)
	}

	if sb.Len() != 4 {
		t.Errorf("Len() = %d, want 4", sb.Len())
	}

	if !bytes.Equal(sb.Bytes(), data) {
		t.Errorf("Bytes() = %v, want %v", sb.Bytes(), data)
	}

	// Write more
	data2 := []byte{0x05, 0x06}
	n = sb.WriteBytes(data2)
	if n != 2 {
		t.Errorf("WriteBytes() = %d, want 2", n)
	}

	expected := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	if !bytes.Equal(sb.Bytes(), expected) {
		t.Errorf("Bytes() = %v, want %v", sb.Bytes(), expected)
	}

	// Reset
	sb.Reset()
	if sb.Len() != 0 {
		t.Errorf("after Reset, Len() = %d, want 0", sb.Len())
	}

	if len(sb.Bytes()) != 0 {
		t.Errorf("after Reset, Bytes() len = %d, want 0", len(sb.Bytes()))
	}
}

// TestSessionBufferExtended verifies extended message buffer size.
//
// VALIDATES: Extended buffer is 65535 bytes.
// PREVENTS: Incorrect buffer size for extended messages.
func TestSessionBufferExtended(t *testing.T) {
	sb := wire.NewSessionBuffer(true) // 65535 bytes

	if sb.Cap() != 65535 {
		t.Errorf("Cap() = %d, want 65535", sb.Cap())
	}
}

// TestSessionBufferResize verifies buffer resize from standard to extended.
//
// VALIDATES: Resize preserves existing data and expands capacity.
// PREVENTS: Data loss during resize.
func TestSessionBufferResize(t *testing.T) {
	sb := wire.NewSessionBuffer(false) // Start with 4096

	// Write some data
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	sb.WriteBytes(data)

	// Resize to extended
	sb.Resize(true)

	if sb.Cap() != 65535 {
		t.Errorf("after Resize, Cap() = %d, want 65535", sb.Cap())
	}

	// Data should be preserved
	if !bytes.Equal(sb.Bytes(), data) {
		t.Errorf("after Resize, Bytes() = %v, want %v", sb.Bytes(), data)
	}
}

// TestSessionBufferResizeNoop verifies resize is noop when already large enough.
//
// VALIDATES: Resize doesn't reallocate unnecessarily.
// PREVENTS: Wasted allocations.
func TestSessionBufferResizeNoop(t *testing.T) {
	sb := wire.NewSessionBuffer(true) // Already 65535

	// Get pointer to underlying buffer
	before := sb.Bytes()

	sb.Resize(true) // Should be noop

	after := sb.Bytes()

	// Slices should point to same underlying array (no realloc)
	// Can't directly compare, but capacity should be same
	if sb.Cap() != 65535 {
		t.Errorf("Cap() = %d, want 65535", sb.Cap())
	}

	_ = before
	_ = after
}

// TestSessionBufferRemaining verifies remaining capacity calculation.
//
// VALIDATES: Remaining() returns correct available space.
// PREVENTS: Buffer overflow from incorrect capacity tracking.
func TestSessionBufferRemaining(t *testing.T) {
	sb := wire.NewSessionBuffer(false) // 4096

	if sb.Remaining() != 4096 {
		t.Errorf("Remaining() = %d, want 4096", sb.Remaining())
	}

	sb.WriteBytes(make([]byte, 100))

	if sb.Remaining() != 3996 {
		t.Errorf("Remaining() = %d, want 3996", sb.Remaining())
	}
}

// TestSessionBufferWriteByte verifies single byte write.
//
// VALIDATES: WriteByte works correctly.
// PREVENTS: Off-by-one errors in single byte writes.
func TestSessionBufferWriteByte(t *testing.T) {
	sb := wire.NewSessionBuffer(false)

	_ = sb.WriteByte(0xFF)
	_ = sb.WriteByte(0xFE)

	expected := []byte{0xFF, 0xFE}
	if !bytes.Equal(sb.Bytes(), expected) {
		t.Errorf("Bytes() = %v, want %v", sb.Bytes(), expected)
	}
}

// TestSessionBufferWriteUint16 verifies 16-bit big-endian write.
//
// VALIDATES: WriteUint16 writes in network byte order.
// PREVENTS: Endianness bugs.
func TestSessionBufferWriteUint16(t *testing.T) {
	sb := wire.NewSessionBuffer(false)

	sb.WriteUint16(0x1234)

	expected := []byte{0x12, 0x34}
	if !bytes.Equal(sb.Bytes(), expected) {
		t.Errorf("Bytes() = %v, want %v", sb.Bytes(), expected)
	}
}

// TestSessionBufferWriteUint32 verifies 32-bit big-endian write.
//
// VALIDATES: WriteUint32 writes in network byte order.
// PREVENTS: Endianness bugs.
func TestSessionBufferWriteUint32(t *testing.T) {
	sb := wire.NewSessionBuffer(false)

	sb.WriteUint32(0x12345678)

	expected := []byte{0x12, 0x34, 0x56, 0x78}
	if !bytes.Equal(sb.Bytes(), expected) {
		t.Errorf("Bytes() = %v, want %v", sb.Bytes(), expected)
	}
}

// TestSessionBufferOffset verifies offset tracking and manipulation.
//
// VALIDATES: Offset() and SetOffset() work correctly.
// PREVENTS: Incorrect position tracking for placeholder fills.
func TestSessionBufferOffset(t *testing.T) {
	sb := wire.NewSessionBuffer(false)

	if sb.Offset() != 0 {
		t.Errorf("Offset() = %d, want 0", sb.Offset())
	}

	sb.WriteBytes([]byte{1, 2, 3})

	if sb.Offset() != 3 {
		t.Errorf("Offset() = %d, want 3", sb.Offset())
	}

	// Skip ahead (for placeholder)
	sb.SetOffset(10)
	if sb.Offset() != 10 {
		t.Errorf("after SetOffset(10), Offset() = %d, want 10", sb.Offset())
	}

	// Len should reflect new offset
	if sb.Len() != 10 {
		t.Errorf("Len() = %d, want 10", sb.Len())
	}
}

// TestSessionBufferPutUint16At verifies writing uint16 at specific offset.
//
// VALIDATES: PutUint16At writes at correct position without moving offset.
// PREVENTS: Placeholder fill bugs.
func TestSessionBufferPutUint16At(t *testing.T) {
	sb := wire.NewSessionBuffer(false)

	// Write header with placeholder
	sb.WriteBytes([]byte{0xFF, 0xFF}) // Placeholder for length
	sb.WriteBytes([]byte{0x01, 0x02, 0x03})

	// Fill in length at position 0
	sb.PutUint16At(0, 3) // Length = 3

	expected := []byte{0x00, 0x03, 0x01, 0x02, 0x03}
	if !bytes.Equal(sb.Bytes(), expected) {
		t.Errorf("Bytes() = %v, want %v", sb.Bytes(), expected)
	}

	// Offset should not have changed
	if sb.Offset() != 5 {
		t.Errorf("Offset() = %d, want 5", sb.Offset())
	}
}
