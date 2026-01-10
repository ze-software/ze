package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestASPathIterator verifies basic AS-PATH segment iteration.
//
// VALIDATES: Iterator returns each segment correctly
// PREVENTS: Off-by-one errors, incorrect segment extraction.
func TestASPathIterator(t *testing.T) {
	// AS_PATH with two segments (4-byte ASN):
	// AS_SEQUENCE [65001, 65002]
	// AS_SET [65003]
	data := []byte{
		0x02, 0x02, // AS_SEQUENCE, 2 ASNs
		0x00, 0x00, 0xFD, 0xE9, // 65001
		0x00, 0x00, 0xFD, 0xEA, // 65002
		0x01, 0x01, // AS_SET, 1 ASN
		0x00, 0x00, 0xFD, 0xEB, // 65003
	}

	iter := NewASPathIterator(data, true) // asn4=true

	// First segment: AS_SEQUENCE
	segType, asns, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, ASSequence, segType)
	assert.Equal(t, []byte{0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA}, asns)

	// Second segment: AS_SET
	segType, asns, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, ASSet, segType)
	assert.Equal(t, []byte{0x00, 0x00, 0xFD, 0xEB}, asns)

	// No more
	_, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestASPathIteratorASN2 verifies 2-byte ASN parsing.
//
// VALIDATES: Iterator correctly handles 2-byte ASN encoding
// PREVENTS: Incorrect length calculation for legacy ASN format.
func TestASPathIteratorASN2(t *testing.T) {
	// AS_PATH with 2-byte ASNs:
	// AS_SEQUENCE [65001, 65002, 65003]
	data := []byte{
		0x02, 0x03, // AS_SEQUENCE, 3 ASNs
		0xFD, 0xE9, // 65001
		0xFD, 0xEA, // 65002
		0xFD, 0xEB, // 65003
	}

	iter := NewASPathIterator(data, false) // asn4=false

	segType, asns, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, ASSequence, segType)
	assert.Equal(t, []byte{0xFD, 0xE9, 0xFD, 0xEA, 0xFD, 0xEB}, asns)

	_, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestASPathIteratorEmpty verifies empty buffer handling.
//
// VALIDATES: Empty buffer returns no items immediately
// PREVENTS: Panic on empty input.
func TestASPathIteratorEmpty(t *testing.T) {
	iter := NewASPathIterator(nil, true)
	_, _, ok := iter.Next()
	assert.False(t, ok)

	iter = NewASPathIterator([]byte{}, true)
	_, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestASPathIteratorReset verifies iterator reset.
//
// VALIDATES: Reset allows re-iteration from start
// PREVENTS: Iterator becoming unusable after exhaustion.
func TestASPathIteratorReset(t *testing.T) {
	data := []byte{
		0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9, // AS_SEQUENCE [65001]
	}

	iter := NewASPathIterator(data, true)

	// Exhaust iterator
	_, _, _ = iter.Next()
	_, _, ok := iter.Next()
	assert.False(t, ok)

	// Reset and iterate again
	iter.Reset()
	segType, _, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, ASSequence, segType)
}

// TestASPathIteratorCount verifies counting segments.
//
// VALIDATES: Count returns correct number of segments
// PREVENTS: Miscounting segments.
func TestASPathIteratorCount(t *testing.T) {
	data := []byte{
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA, // AS_SEQUENCE [65001, 65002]
		0x01, 0x01, 0x00, 0x00, 0xFD, 0xEB, // AS_SET [65003]
	}

	iter := NewASPathIterator(data, true)
	assert.Equal(t, 2, iter.Count())
}

// TestASNIterator verifies ASN iteration within a segment.
//
// VALIDATES: ASNIterator returns each ASN correctly
// PREVENTS: Incorrect ASN extraction from segment bytes.
func TestASNIterator(t *testing.T) {
	// 4-byte ASNs: 65001, 65002, 65003
	asns := []byte{
		0x00, 0x00, 0xFD, 0xE9,
		0x00, 0x00, 0xFD, 0xEA,
		0x00, 0x00, 0xFD, 0xEB,
	}

	iter := NewASNIterator(asns, true) // asn4=true

	asn, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, uint32(65001), asn)

	asn, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, uint32(65002), asn)

	asn, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, uint32(65003), asn)

	_, ok = iter.Next()
	assert.False(t, ok)
}

// TestASNIteratorASN2 verifies 2-byte ASN iteration.
//
// VALIDATES: ASNIterator handles 2-byte ASN encoding
// PREVENTS: Incorrect ASN values from 2-byte format.
func TestASNIteratorASN2(t *testing.T) {
	// 2-byte ASNs: 65001, 65002
	asns := []byte{0xFD, 0xE9, 0xFD, 0xEA}

	iter := NewASNIterator(asns, false) // asn4=false

	asn, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, uint32(65001), asn)

	asn, ok = iter.Next()
	require.True(t, ok)
	assert.Equal(t, uint32(65002), asn)

	_, ok = iter.Next()
	assert.False(t, ok)
}

// TestASNIteratorEmpty verifies empty segment handling.
//
// VALIDATES: Empty segment returns no ASNs
// PREVENTS: Panic on empty segment.
func TestASNIteratorEmpty(t *testing.T) {
	iter := NewASNIterator(nil, true)
	_, ok := iter.Next()
	assert.False(t, ok)

	iter = NewASNIterator([]byte{}, true)
	_, ok = iter.Next()
	assert.False(t, ok)
}

// TestASNIteratorCount verifies ASN counting.
//
// VALIDATES: Count returns correct number of ASNs
// PREVENTS: Miscounting ASNs.
func TestASNIteratorCount(t *testing.T) {
	asns4 := []byte{
		0x00, 0x00, 0xFD, 0xE9,
		0x00, 0x00, 0xFD, 0xEA,
		0x00, 0x00, 0xFD, 0xEB,
	}
	iter := NewASNIterator(asns4, true)
	assert.Equal(t, 3, iter.Count())

	asns2 := []byte{0xFD, 0xE9, 0xFD, 0xEA}
	iter = NewASNIterator(asns2, false)
	assert.Equal(t, 2, iter.Count())
}
