// The RFC 7311 Section 3.2 transitive-bit rule, driven through the receive path a peer's
// UPDATE actually takes. See rfc/short/rfc7311.md.
//
// Overview: session_validation.go -- enforceRFC7606 applies the verdict on the receive path
// Related: ../message/rfc7606.go -- validateAttributeFlags reaches the verdict
// Related: ../message/rfc7606_aigp_test.go -- the same rule isolated at the validator
//
// The message-package tests hold the validator. These hold the whole path, because the
// verdict only matters if enforceRFC7606 acts on it: the walk can answer attribute discard
// and the session can still hand the malformed AIGP onward.

package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rfc7311AIGPAttrs returns the well-known mandatory attributes followed by one AIGP
// attribute carrying the given flags byte and the type-1 metric TLV for 1234.
func rfc7311AIGPAttrs(aigpFlags byte) []byte {
	return []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
		aigpFlags, uint8(attribute.AttrAIGP), 0x0b, // AIGP, length 11
		0x01, 0x00, 0x0b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0xd2, // TLV type 1, metric 1234
	}
}

// rfc7311EBGPSession builds an EBGP session (local AS 65001, peer AS 65002). Section 3.2
// is a receive rule with no session-type condition, so the same verdict is owed on an IBGP
// session; EBGP is chosen because it is the side an untrusted peer sits on.
func rfc7311EBGPSession() *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	return NewSession(settings)
}

// TestRFC7311AIGPTransitiveDiscardedOnReceive is the end-to-end proof that a malformed
// AIGP does not survive the receive path.
//
// VALIDATES: RFC 7311 Section 3.2 -- an UPDATE whose AIGP carries the transitive bit comes
// out of enforceRFC7606 with no AIGP codepoint left, one ATTR_TOMBSTONE recording the
// discard, the route's own attributes intact, and no error, so the session stands.
// PREVENTS: the leak this test was written against -- ze accepting the malformed form and
// re-advertising it, which is what it did until the check existed. It also prevents the
// opposite over-correction, a treat-as-withdraw or a session reset that would cost the
// peer the routes the UPDATE carries.
//
// RFC requirement: RFC7311-3.2-4 negative -- an UPDATE received with an AIGP attribute
// whose transitive bit is set leaves enforceRFC7606 with the AIGP codepoint gone, one
// ATTR_TOMBSTONE in its place, ORIGIN, AS_PATH and NEXT_HOP still present, the
// attribute-discard action, and no error.
func TestRFC7311AIGPTransitiveDiscardedOnReceive(t *testing.T) {
	s := rfc7311EBGPSession()

	body := makeUpdateBody(nil, rfc7311AIGPAttrs(0xC0), []byte{24, 10, 0, 0})
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "RFC 7311 Section 3.2 asks for a discard, never a session reset")
	assert.Equal(t, message.RFC7606ActionAttributeDiscard, action)

	attrs := rfc8669PathAttrs(t, wu.Payload())

	surviving, _ := countAttrCode(attrs, uint8(attribute.AttrAIGP))
	assert.Equal(t, 0, surviving,
		"RFC 7311 Section 3.2: a malformed AIGP must not be passed along to other BGP peers")

	markers, marker := countAttrCode(attrs, uint8(attribute.AttrTombstone))
	require.Equal(t, 1, markers, "one ATTR_TOMBSTONE records the discard")
	require.GreaterOrEqual(t, len(marker), 2)
	assert.Equal(t, uint8(attribute.AttrAIGP), marker[0], "the marker must name the AIGP codepoint")
	assert.Equal(t, message.DiscardReasonMalformedValue, marker[1],
		"the marker must carry why, so a discarded AIGP is not read as an absent one")

	// RFC 7606 Section 2: the UPDATE continues to be processed, so the route still rides.
	for _, code := range []attribute.AttributeCode{
		attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrNextHop,
	} {
		count, _ := countAttrCode(attrs, uint8(code))
		assert.Equal(t, 1, count, "attribute %d must survive the AIGP discard", code)
	}
}

// TestRFC7311AIGPNonTransitiveKeptOnReceive is the same UPDATE with the transitive bit
// clear, which is the form RFC 7311 Section 3 defines.
//
// VALIDATES: a conformant AIGP reaches the end of the receive path untouched -- same flags,
// same metric TLV, no discard action and no tombstone.
// PREVENTS: the check firing on the codepoint rather than on the bit, which would strip
// every AIGP ze receives and silently take the attribute out of service for the operators
// who depend on it.
//
// RFC requirement: RFC7311-3.2-4 positive -- an UPDATE received with an AIGP attribute
// whose transitive bit is clear leaves enforceRFC7606 with no error action, the AIGP still
// present with flags 0x80 and its metric TLV byte-identical, and no ATTR_TOMBSTONE.
func TestRFC7311AIGPNonTransitiveKeptOnReceive(t *testing.T) {
	s := rfc7311EBGPSession()

	body := makeUpdateBody(nil, rfc7311AIGPAttrs(0x80), []byte{24, 10, 0, 0})
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action,
		"a non-transitive AIGP is well-formed and must reach the RIB")

	attrs := rfc8669PathAttrs(t, wu.Payload())

	count, value := countAttrCode(attrs, uint8(attribute.AttrAIGP))
	require.Equal(t, 1, count, "a conformant AIGP must survive the receive path")
	assert.Equal(t, "01000b00000000000004d2", hex.EncodeToString(value),
		"the metric TLV must be forwarded unchanged")

	_, flags, _, found := attribute.AttrFind(attrs, attribute.AttrAIGP)
	require.True(t, found)
	assert.Equal(t, attribute.FlagOptional, flags,
		"the flags byte must be forwarded unchanged, transitive bit still clear")

	markers, _ := countAttrCode(attrs, uint8(attribute.AttrTombstone))
	assert.Equal(t, 0, markers, "nothing was discarded, so no marker may be written")
}
