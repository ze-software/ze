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

// TestKeepaliveRejectsPayload verifies KEEPALIVE with payload is rejected.
//
// RFC 4271 Section 4.4: "A KEEPALIVE message consists of only the message
// header and has a length of 19 octets."
//
// RFC 4271 Section 6.1: "if the Length field of a KEEPALIVE message is not
// equal to 19, then the Error Subcode MUST be set to Bad Message Length"
//
// VALIDATES: Non-empty payload returns NOTIFICATION error (Code 1, Subcode 2).
//
// PREVENTS: Accepting malformed KEEPALIVE that violates RFC 4271.
func TestKeepaliveRejectsPayload(t *testing.T) {
	// KEEPALIVE with unexpected payload data
	payload := []byte{0x01, 0x02, 0x03}

	msg, err := UnpackKeepalive(payload)
	require.Error(t, err, "KEEPALIVE with payload must be rejected")
	assert.Nil(t, msg)

	// Error must be a *Notification for proper BGP error handling
	var notif *Notification
	require.ErrorAs(t, err, &notif, "error must be *Notification")

	// RFC 4271 Section 6.1: Message Header Error (1), Bad Message Length (2)
	assert.Equal(t, NotifyMessageHeader, notif.ErrorCode, "error code must be Message Header Error")
	assert.Equal(t, NotifyHeaderBadLength, notif.ErrorSubcode, "subcode must be Bad Message Length")
}
