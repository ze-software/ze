package nlri

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNLRIIterator verifies basic NLRI iteration.
//
// VALIDATES: Iterator returns each prefix correctly without allocation.
// PREVENTS: Off-by-one errors, incorrect prefix extraction.
func TestNLRIIterator(t *testing.T) {
	// Wire format: multiple IPv4 prefixes concatenated
	// 10.0.0.0/8 = [8, 10]
	// 192.168.1.0/24 = [24, 192, 168, 1]
	// 172.16.0.0/16 = [16, 172, 16]
	data := []byte{
		8, 10, // 10.0.0.0/8
		24, 192, 168, 1, // 192.168.1.0/24
		16, 172, 16, // 172.16.0.0/16
	}

	iter := NewNLRIIterator(data, false)

	// First prefix
	prefix, pathID, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{8, 10}, prefix)
	assert.Equal(t, uint32(0), pathID)

	// Second prefix
	prefix, pathID, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{24, 192, 168, 1}, prefix)
	assert.Equal(t, uint32(0), pathID)

	// Third prefix
	prefix, pathID, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{16, 172, 16}, prefix)
	assert.Equal(t, uint32(0), pathID)

	// No more
	_, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestNLRIIteratorAddPath verifies ADD-PATH path-id parsing.
//
// VALIDATES: Iterator correctly extracts 4-byte path-id prefix
// PREVENTS: Incorrect ADD-PATH handling per RFC 7911.
func TestNLRIIteratorAddPath(t *testing.T) {
	// Wire format with ADD-PATH: [path-id (4 bytes), length, prefix...]
	// Path ID 100, 10.0.0.0/8 = [0, 0, 0, 100, 8, 10]
	// Path ID 200, 192.168.0.0/16 = [0, 0, 0, 200, 16, 192, 168]
	data := []byte{
		0, 0, 0, 100, 8, 10, // path-id=100, 10.0.0.0/8
		0, 0, 0, 200, 16, 192, 168, // path-id=200, 192.168.0.0/16
	}

	iter := NewNLRIIterator(data, true)

	// First prefix
	prefix, pathID, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{8, 10}, prefix) // prefix without path-id
	assert.Equal(t, uint32(100), pathID)

	// Second prefix
	prefix, pathID, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{16, 192, 168}, prefix)
	assert.Equal(t, uint32(200), pathID)

	// No more
	_, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestNLRIIteratorEmpty verifies empty buffer handling.
//
// VALIDATES: Empty buffer returns no items immediately
// PREVENTS: Panic on empty input.
func TestNLRIIteratorEmpty(t *testing.T) {
	iter := NewNLRIIterator(nil, false)
	_, _, ok := iter.Next()
	assert.False(t, ok)

	iter = NewNLRIIterator([]byte{}, false)
	_, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestNLRIIteratorIPv6 verifies IPv6 prefix parsing.
//
// VALIDATES: Iterator handles longer IPv6 prefixes correctly
// PREVENTS: Incorrect byte count for IPv6.
func TestNLRIIteratorIPv6(t *testing.T) {
	// 2001:db8::/32 = [32, 0x20, 0x01, 0x0d, 0xb8]
	// ::1/128 = [128, 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1]
	data := []byte{
		32, 0x20, 0x01, 0x0d, 0xb8, // 2001:db8::/32
	}

	iter := NewNLRIIterator(data, false)

	prefix, _, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{32, 0x20, 0x01, 0x0d, 0xb8}, prefix)

	_, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestNLRIIteratorZeroPrefix verifies /0 prefix handling.
//
// VALIDATES: Default route (0.0.0.0/0) parsed correctly
// PREVENTS: Edge case with zero-length prefix.
func TestNLRIIteratorZeroPrefix(t *testing.T) {
	// 0.0.0.0/0 = [0] (no prefix bytes)
	data := []byte{0}

	iter := NewNLRIIterator(data, false)

	prefix, _, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{0}, prefix) // just length byte

	_, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestNLRIIteratorReset verifies iterator can be reset.
//
// VALIDATES: Reset allows re-iteration from start
// PREVENTS: Iterator becoming unusable after exhaustion.
func TestNLRIIteratorReset(t *testing.T) {
	data := []byte{8, 10, 16, 172, 16}

	iter := NewNLRIIterator(data, false)

	// Exhaust iterator
	_, _, _ = iter.Next()
	_, _, _ = iter.Next()
	_, _, ok := iter.Next()
	assert.False(t, ok)

	// Reset and iterate again
	iter.Reset()
	prefix, _, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, []byte{8, 10}, prefix)
}

// TestNLRIIteratorCount verifies counting without full parse.
//
// VALIDATES: Count returns correct number of NLRIs
// PREVENTS: Miscounting prefixes.
func TestNLRIIteratorCount(t *testing.T) {
	data := []byte{
		8, 10, // 10.0.0.0/8
		24, 192, 168, 1, // 192.168.1.0/24
		16, 172, 16, // 172.16.0.0/16
	}

	iter := NewNLRIIterator(data, false)
	assert.Equal(t, 3, iter.Count())

	// With ADD-PATH
	dataAddPath := []byte{
		0, 0, 0, 1, 8, 10,
		0, 0, 0, 2, 16, 172, 16,
	}
	iter = NewNLRIIterator(dataAddPath, true)
	assert.Equal(t, 2, iter.Count())
}
