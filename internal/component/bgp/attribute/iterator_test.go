package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttrIterator verifies basic attribute iteration.
//
// VALIDATES: Iterator returns each attribute correctly
// PREVENTS: Off-by-one errors, incorrect attribute extraction.
func TestAttrIterator(t *testing.T) {
	t.Parallel()
	// Build wire format:
	// ORIGIN (type 1, transitive, length 1): value = IGP (0)
	// MED (type 4, optional transitive, length 4): value = 100
	data := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN: flags=0x40, code=1, len=1, value=0
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED: flags=0x80, code=4, len=4, value=100
	}

	iter := NewAttrIterator(data)

	// First attribute: ORIGIN
	typeCode, flags, value, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, AttrOrigin, typeCode)
	assert.Equal(t, AttributeFlags(0x40), flags)
	assert.Equal(t, []byte{0x00}, value)

	// Second attribute: MED
	typeCode, flags, value, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, AttrMED, typeCode)
	assert.Equal(t, AttributeFlags(0x80), flags)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x64}, value)

	// No more
	_, _, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestAttrIteratorExtendedLength verifies extended length handling.
//
// VALIDATES: Iterator handles 2-byte length for large attributes
// PREVENTS: Incorrect parsing of attributes > 255 bytes.
func TestAttrIteratorExtendedLength(t *testing.T) {
	t.Parallel()
	// Extended length attribute (flags bit 0x10 set)
	// AS_PATH with 300 bytes of data (hypothetical)
	data := make([]byte, 4+256) // flags + code + 2-byte len + 256 bytes
	data[0] = 0x50              // Transitive + Extended Length
	data[1] = 0x02              // AS_PATH
	data[2] = 0x01              // Length high byte (256)
	data[3] = 0x00              // Length low byte
	// Fill value with dummy data
	for i := 4; i < len(data); i++ {
		data[i] = byte(i)
	}

	iter := NewAttrIterator(data)

	typeCode, flags, value, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, AttrASPath, typeCode)
	assert.Equal(t, AttributeFlags(0x50), flags)
	assert.Equal(t, 256, len(value))

	_, _, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestAttrIteratorEmpty verifies empty buffer handling.
//
// VALIDATES: Empty buffer returns no items immediately
// PREVENTS: Panic on empty input.
func TestAttrIteratorEmpty(t *testing.T) {
	t.Parallel()
	iter := NewAttrIterator(nil)
	_, _, _, ok := iter.Next()
	assert.False(t, ok)

	iter = NewAttrIterator([]byte{})
	_, _, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestAttrIteratorFind verifies finding specific attribute.
//
// VALIDATES: Find locates attribute by type code
// PREVENTS: Missing attributes, incorrect search.
func TestAttrIteratorFind(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0xC8, // LOCAL_PREF = 200
	}

	iter := NewAttrIterator(data)

	// Find MED
	value, ok := iter.Find(AttrMED)
	require.True(t, ok)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x64}, value)

	// Reset and find LOCAL_PREF
	iter.Reset()
	value, ok = iter.Find(AttrLocalPref)
	require.True(t, ok)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0xC8}, value)

	// Find non-existent
	iter.Reset()
	_, ok = iter.Find(AttrCommunity)
	assert.False(t, ok)
}

// TestAttrIteratorReset verifies iterator reset.
//
// VALIDATES: Reset allows re-iteration from start
// PREVENTS: Iterator becoming unusable after exhaustion.
func TestAttrIteratorReset(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED
	}

	iter := NewAttrIterator(data)

	// Exhaust iterator
	_, _, _, _ = iter.Next()
	_, _, _, _ = iter.Next()
	_, _, _, ok := iter.Next()
	assert.False(t, ok)

	// Reset and iterate again
	iter.Reset()
	typeCode, _, _, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, AttrOrigin, typeCode)
}

// TestAttrIteratorCount verifies counting attributes.
//
// VALIDATES: Count returns correct number of attributes
// PREVENTS: Miscounting attributes.
func TestAttrIteratorCount(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0xC8, // LOCAL_PREF
	}

	iter := NewAttrIterator(data)
	assert.Equal(t, 3, iter.Count())
}

// TestAttrIteratorValueSlices verifies returned slices are correct.
//
// VALIDATES: Returned slices point to correct buffer regions
// PREVENTS: Buffer overrun from invalid slicing.
func TestAttrIteratorValueSlices(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN: value at offset 3, len 1
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED: value at offset 7, len 4
	}

	iter := NewAttrIterator(data)

	_, _, value, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{0x00}, value)
	assert.Equal(t, 1, len(value))

	_, _, value, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x64}, value)
	assert.Equal(t, 4, len(value))
}
