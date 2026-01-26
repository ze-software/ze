package reader

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReaderReadByte verifies single byte reading.
//
// VALIDATES: Basic byte extraction works.
//
// PREVENTS: Off-by-one errors in position tracking.
func TestReaderReadByte(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03})

	v, err := r.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), v)

	v, err = r.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x02), v)

	v, err = r.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x03), v)

	// EOF
	_, err = r.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

// TestReaderReadUint16 verifies big-endian uint16 reading.
//
// VALIDATES: Network byte order (big-endian) parsing.
//
// PREVENTS: Endianness bugs in protocol parsing.
func TestReaderReadUint16(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03, 0x04})

	v, err := r.ReadUint16()
	require.NoError(t, err)
	assert.Equal(t, uint16(0x0102), v)

	v, err = r.ReadUint16()
	require.NoError(t, err)
	assert.Equal(t, uint16(0x0304), v)

	// EOF
	_, err = r.ReadUint16()
	assert.ErrorIs(t, err, io.EOF)
}

// TestReaderReadUint32 verifies big-endian uint32 reading.
//
// VALIDATES: 4-byte network order parsing.
//
// PREVENTS: Endianness bugs in AS numbers, router IDs.
func TestReaderReadUint32(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	v, err := r.ReadUint32()
	require.NoError(t, err)
	assert.Equal(t, uint32(0x01020304), v)

	// Not enough bytes
	_, err = r.ReadUint32()
	assert.ErrorIs(t, err, io.EOF)
}

// TestReaderReadBytes verifies multi-byte reading (zero-copy).
//
// VALIDATES: Zero-copy slice extraction.
//
// PREVENTS: Unnecessary allocations during parsing.
func TestReaderReadBytes(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	r := NewReader(data)

	v, err := r.ReadBytes(3)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, v)

	// Verify zero-copy (same underlying array)
	v[0] = 0xFF
	assert.Equal(t, byte(0xFF), data[0], "should be zero-copy")

	// Read remaining
	v, err = r.ReadBytes(2)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x04, 0x05}, v)

	// EOF
	_, err = r.ReadBytes(1)
	assert.ErrorIs(t, err, io.EOF)
}

// TestReaderRemaining verifies remaining bytes access.
//
// VALIDATES: Access to unread portion of buffer.
//
// PREVENTS: Incorrect remaining length calculations.
func TestReaderRemaining(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	assert.Equal(t, 5, r.Remaining())

	_, _ = r.ReadBytes(2)
	assert.Equal(t, 3, r.Remaining())

	_, _ = r.ReadBytes(3)
	assert.Equal(t, 0, r.Remaining())
}

// TestReaderRemainingBytes verifies remaining bytes slice.
//
// VALIDATES: Zero-copy access to remaining data.
//
// PREVENTS: Extra copies when passing remaining data to sub-parsers.
func TestReaderRemainingBytes(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	_, _ = r.ReadBytes(2)
	rem := r.RemainingBytes()
	assert.Equal(t, []byte{0x03, 0x04, 0x05}, rem)
}

// TestReaderSkip verifies byte skipping.
//
// VALIDATES: Efficient skipping without reading.
//
// PREVENTS: Unnecessary processing of ignored fields.
func TestReaderSkip(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	err := r.Skip(2)
	require.NoError(t, err)

	v, err := r.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x03), v)

	// Skip past end
	err = r.Skip(10)
	assert.ErrorIs(t, err, io.EOF)
}

// TestReaderEmpty verifies empty buffer handling.
//
// VALIDATES: Edge case - empty input.
//
// PREVENTS: Panic on empty buffer.
func TestReaderEmpty(t *testing.T) {
	r := NewReader([]byte{})

	assert.Equal(t, 0, r.Remaining())

	_, err := r.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

// TestReaderNil verifies nil buffer handling.
//
// VALIDATES: Edge case - nil input treated as empty.
//
// PREVENTS: Panic on nil input.
func TestReaderNil(t *testing.T) {
	r := NewReader(nil)

	assert.Equal(t, 0, r.Remaining())

	_, err := r.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

// TestReaderPeek verifies peeking without advancing.
//
// VALIDATES: Look-ahead without consuming.
//
// PREVENTS: Incorrect parsing when type byte determines structure.
func TestReaderPeek(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03})

	v, err := r.Peek()
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), v)

	// Position unchanged
	v, err = r.Peek()
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), v)

	// Still can read
	v, err = r.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), v)
}

// TestReaderOffset verifies position tracking.
//
// VALIDATES: Current position is accessible.
//
// PREVENTS: Lost track of parse position for error messages.
func TestReaderOffset(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	assert.Equal(t, 0, r.Offset())

	_, _ = r.ReadBytes(3)
	assert.Equal(t, 3, r.Offset())
}

// TestReaderReadUint64 verifies big-endian uint64 reading.
//
// VALIDATES: 8-byte network order parsing.
//
// PREVENTS: Errors parsing extended community values.
func TestReaderReadUint64(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})

	v, err := r.ReadUint64()
	require.NoError(t, err)
	assert.Equal(t, uint64(0x0102030405060708), v)
}

// TestReaderSlice verifies creating sub-reader.
//
// VALIDATES: Sub-reader for nested parsing.
//
// PREVENTS: Position corruption when parsing nested structures.
func TestReaderSlice(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	// Read length prefix
	_, _ = r.ReadByte()

	// Create sub-reader for next 3 bytes
	sub, err := r.Slice(3)
	require.NoError(t, err)

	// Sub-reader is independent
	v, err := sub.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(0x02), v)
	assert.Equal(t, 2, sub.Remaining())

	// Original reader advanced past sub-reader
	assert.Equal(t, 1, r.Remaining())
}
