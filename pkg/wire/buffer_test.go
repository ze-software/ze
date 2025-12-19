package wire

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBufferReadByte verifies single byte reading.
//
// VALIDATES: Basic byte extraction works.
//
// PREVENTS: Off-by-one errors in position tracking.
func TestBufferReadByte(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03})

	v, err := b.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), v)

	v, err = b.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x02), v)

	v, err = b.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x03), v)

	// EOF
	_, err = b.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

// TestBufferReadUint16 verifies big-endian uint16 reading.
//
// VALIDATES: Network byte order (big-endian) parsing.
//
// PREVENTS: Endianness bugs in protocol parsing.
func TestBufferReadUint16(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03, 0x04})

	v, err := b.ReadUint16()
	require.NoError(t, err)
	assert.Equal(t, uint16(0x0102), v)

	v, err = b.ReadUint16()
	require.NoError(t, err)
	assert.Equal(t, uint16(0x0304), v)

	// EOF
	_, err = b.ReadUint16()
	assert.ErrorIs(t, err, io.EOF)
}

// TestBufferReadUint32 verifies big-endian uint32 reading.
//
// VALIDATES: 4-byte network order parsing.
//
// PREVENTS: Endianness bugs in AS numbers, router IDs.
func TestBufferReadUint32(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	v, err := b.ReadUint32()
	require.NoError(t, err)
	assert.Equal(t, uint32(0x01020304), v)

	// Not enough bytes
	_, err = b.ReadUint32()
	assert.ErrorIs(t, err, io.EOF)
}

// TestBufferReadBytes verifies multi-byte reading (zero-copy).
//
// VALIDATES: Zero-copy slice extraction.
//
// PREVENTS: Unnecessary allocations during parsing.
func TestBufferReadBytes(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	b := NewBuffer(data)

	v, err := b.ReadBytes(3)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, v)

	// Verify zero-copy (same underlying array)
	v[0] = 0xFF
	assert.Equal(t, byte(0xFF), data[0], "should be zero-copy")

	// Read remaining
	v, err = b.ReadBytes(2)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x04, 0x05}, v)

	// EOF
	_, err = b.ReadBytes(1)
	assert.ErrorIs(t, err, io.EOF)
}

// TestBufferRemaining verifies remaining bytes access.
//
// VALIDATES: Access to unread portion of buffer.
//
// PREVENTS: Incorrect remaining length calculations.
func TestBufferRemaining(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	assert.Equal(t, 5, b.Remaining())

	b.ReadBytes(2)
	assert.Equal(t, 3, b.Remaining())

	b.ReadBytes(3)
	assert.Equal(t, 0, b.Remaining())
}

// TestBufferRemainingBytes verifies remaining bytes slice.
//
// VALIDATES: Zero-copy access to remaining data.
//
// PREVENTS: Extra copies when passing remaining data to sub-parsers.
func TestBufferRemainingBytes(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	b.ReadBytes(2)
	rem := b.RemainingBytes()
	assert.Equal(t, []byte{0x03, 0x04, 0x05}, rem)
}

// TestBufferSkip verifies byte skipping.
//
// VALIDATES: Efficient skipping without reading.
//
// PREVENTS: Unnecessary processing of ignored fields.
func TestBufferSkip(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	err := b.Skip(2)
	require.NoError(t, err)

	v, err := b.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x03), v)

	// Skip past end
	err = b.Skip(10)
	assert.ErrorIs(t, err, io.EOF)
}

// TestBufferEmpty verifies empty buffer handling.
//
// VALIDATES: Edge case - empty input.
//
// PREVENTS: Panic on empty buffer.
func TestBufferEmpty(t *testing.T) {
	b := NewBuffer([]byte{})

	assert.Equal(t, 0, b.Remaining())

	_, err := b.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

// TestBufferNil verifies nil buffer handling.
//
// VALIDATES: Edge case - nil input treated as empty.
//
// PREVENTS: Panic on nil input.
func TestBufferNil(t *testing.T) {
	b := NewBuffer(nil)

	assert.Equal(t, 0, b.Remaining())

	_, err := b.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

// TestBufferPeek verifies peeking without advancing.
//
// VALIDATES: Look-ahead without consuming.
//
// PREVENTS: Incorrect parsing when type byte determines structure.
func TestBufferPeek(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03})

	v, err := b.Peek()
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), v)

	// Position unchanged
	v, err = b.Peek()
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), v)

	// Still can read
	v, err = b.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), v)
}

// TestBufferOffset verifies position tracking.
//
// VALIDATES: Current position is accessible.
//
// PREVENTS: Lost track of parse position for error messages.
func TestBufferOffset(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	assert.Equal(t, 0, b.Offset())

	b.ReadBytes(3)
	assert.Equal(t, 3, b.Offset())
}

// TestBufferReadUint64 verifies big-endian uint64 reading.
//
// VALIDATES: 8-byte network order parsing.
//
// PREVENTS: Errors parsing extended community values.
func TestBufferReadUint64(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})

	v, err := b.ReadUint64()
	require.NoError(t, err)
	assert.Equal(t, uint64(0x0102030405060708), v)
}

// TestBufferSlice verifies creating sub-buffer.
//
// VALIDATES: Sub-buffer for nested parsing.
//
// PREVENTS: Position corruption when parsing nested structures.
func TestBufferSlice(t *testing.T) {
	b := NewBuffer([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	// Read length prefix
	b.ReadByte()

	// Create sub-buffer for next 3 bytes
	sub, err := b.Slice(3)
	require.NoError(t, err)

	// Sub-buffer is independent
	v, err := sub.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x02), v)
	assert.Equal(t, 2, sub.Remaining())

	// Original buffer advanced past sub-buffer
	assert.Equal(t, 1, b.Remaining())
}
