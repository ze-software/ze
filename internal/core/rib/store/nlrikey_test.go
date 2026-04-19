package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNLRIKey_IPv4 validates AC-2: IPv4 prefix round-trips through NLRIKey.
//
// VALIDATES: NLRIKey correctly encodes IPv4 prefix-len + prefix bytes.
// PREVENTS: Trailing zeros corrupting NLRI bytes on round-trip.
func TestNLRIKey_IPv4(t *testing.T) {
	// /24 prefix: [prefix-len=24][10][0][0]
	nlri := []byte{24, 10, 0, 0}
	key := NewNLRIKey(nlri)

	assert.Equal(t, 4, key.Len())
	assert.Equal(t, nlri, key.Bytes())
}

// TestNLRIKey_IPv6 validates AC-3: IPv6 prefix round-trips through NLRIKey.
//
// VALIDATES: NLRIKey correctly encodes IPv6 prefix bytes.
// PREVENTS: Longer NLRI bytes being truncated or padded incorrectly.
func TestNLRIKey_IPv6(t *testing.T) {
	// /48 prefix: [prefix-len=48][2001:0db8:0001] = 7 bytes
	nlri := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	key := NewNLRIKey(nlri)

	assert.Equal(t, 7, key.Len())
	assert.Equal(t, nlri, key.Bytes())
}

// TestNLRIKey_AddPath validates AC-4: ADD-PATH prefix with path-id round-trips.
//
// VALIDATES: NLRIKey includes 4-byte path-id prefix.
// PREVENTS: Path-id being lost or corrupted in fixed-size key.
func TestNLRIKey_AddPath(t *testing.T) {
	// ADD-PATH: [path-id:4][prefix-len=24][10][0][0]
	nlri := []byte{0, 0, 0, 42, 24, 10, 0, 0}
	key := NewNLRIKey(nlri)

	assert.Equal(t, 8, key.Len())
	assert.Equal(t, nlri, key.Bytes())
}

// TestNLRIKey_MaxLength validates AC-4: max NLRI length (21 bytes) fits.
//
// VALIDATES: ADD-PATH IPv6 /128 (21 bytes) fits in [24]byte.
// PREVENTS: Max-length NLRI being truncated.
func TestNLRIKey_MaxLength(t *testing.T) {
	// ADD-PATH IPv6 /128: [path-id:4][prefix-len=128][16 bytes addr] = 21 bytes
	nlri := make([]byte, 21)
	nlri[3] = 1    // path-id = 1
	nlri[4] = 128  // prefix-len
	nlri[5] = 0x20 // 2001:...
	nlri[6] = 0x01
	key := NewNLRIKey(nlri)

	assert.Equal(t, 21, key.Len())
	assert.Equal(t, nlri, key.Bytes())
}

// TestNLRIKey_Equality validates AC-2: same input produces equal keys.
//
// VALIDATES: NLRIKey is deterministic and comparable.
// PREVENTS: Map lookups failing due to non-deterministic key encoding.
func TestNLRIKey_Equality(t *testing.T) {
	nlri := []byte{24, 10, 0, 0}
	k1 := NewNLRIKey(nlri)
	k2 := NewNLRIKey(nlri)

	assert.Equal(t, k1, k2, "same input must produce equal keys")

	different := []byte{24, 10, 0, 1}
	k3 := NewNLRIKey(different)
	assert.NotEqual(t, k1, k3, "different input must produce unequal keys")
}

// TestNLRIKey_Empty validates edge case: zero-length NLRI.
//
// VALIDATES: Empty NLRI produces a valid key with Len()==0.
// PREVENTS: Panic on empty input.
func TestNLRIKey_Empty(t *testing.T) {
	key := NewNLRIKey(nil)
	assert.Equal(t, 0, key.Len())
	assert.Equal(t, []byte{}, key.Bytes())

	key2 := NewNLRIKey([]byte{})
	assert.Equal(t, 0, key2.Len())
	assert.Equal(t, key, key2)
}

// TestNLRIKey_Oversized validates safety: input > 24 bytes is truncated.
//
// VALIDATES: No panic on oversized input.
// PREVENTS: Index out of bounds.
func TestNLRIKey_Oversized(t *testing.T) {
	nlri := make([]byte, 30)
	for i := range nlri {
		nlri[i] = byte(i)
	}
	key := NewNLRIKey(nlri)

	assert.Equal(t, 24, key.Len())
	assert.Equal(t, nlri[:24], key.Bytes())
}
