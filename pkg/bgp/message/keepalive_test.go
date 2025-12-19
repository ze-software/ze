package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKeepaliveType verifies KEEPALIVE message type.
//
// VALIDATES: Correct message type returned.
//
// PREVENTS: Wrong type causing message misrouting.
func TestKeepaliveType(t *testing.T) {
	k := &Keepalive{}
	assert.Equal(t, TypeKEEPALIVE, k.Type())
}

// TestKeepalivePack verifies KEEPALIVE packing.
//
// VALIDATES: KEEPALIVE has no body (header only).
//
// PREVENTS: Extra bytes causing peer to reject message.
func TestKeepalivePack(t *testing.T) {
	k := &Keepalive{}
	data, err := k.Pack(nil)
	require.NoError(t, err)

	// KEEPALIVE is just header (19 bytes), no body
	assert.Len(t, data, HeaderLen)

	// Verify it's a valid header
	h, err := ParseHeader(data)
	require.NoError(t, err)
	assert.Equal(t, TypeKEEPALIVE, h.Type)
	assert.Equal(t, uint16(HeaderLen), h.Length)
}

// TestKeepaliveUnpack verifies KEEPALIVE unpacking.
//
// VALIDATES: Empty body parsed correctly.
//
// PREVENTS: Rejecting valid KEEPALIVE messages.
func TestKeepaliveUnpack(t *testing.T) {
	// KEEPALIVE has no body
	msg, err := UnpackKeepalive([]byte{})
	require.NoError(t, err)
	assert.NotNil(t, msg)
	assert.Equal(t, TypeKEEPALIVE, msg.Type())
}

// TestKeepaliveRoundTrip verifies pack/unpack symmetry.
//
// VALIDATES: Serialization is reversible.
//
// PREVENTS: Data corruption in pack/unpack cycle.
func TestKeepaliveRoundTrip(t *testing.T) {
	original := &Keepalive{}
	data, err := original.Pack(nil)
	require.NoError(t, err)

	// Strip header for unpacking (body only)
	body := data[HeaderLen:]

	parsed, err := UnpackKeepalive(body)
	require.NoError(t, err)
	assert.Equal(t, TypeKEEPALIVE, parsed.Type())
}

// TestKeepaliveSingleton verifies singleton pattern.
//
// VALIDATES: Single instance reused.
//
// PREVENTS: Unnecessary allocations for stateless message.
func TestKeepaliveSingleton(t *testing.T) {
	k1 := NewKeepalive()
	k2 := NewKeepalive()
	assert.Same(t, k1, k2, "should return same instance")
}
