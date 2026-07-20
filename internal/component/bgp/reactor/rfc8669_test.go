// RFC: rfc/short/rfc8669.md — BGP Prefix-SID attribute (code 40)
// Overview: session_validation.go — enforceRFC7606 applies the §4 EBGP boundary rule
//
// RFC 8669 §4 makes the SR domain boundary explicit: a Prefix-SID attribute arriving from
// an EBGP neighbor outside the domain MUST be discarded unless the speaker is configured
// to accept it from that neighbor. ze carries that configuration per peer as
// PeerSettings.AcceptSRv6PrefixSID, and enforces it on the real receive path, so these
// tests drive enforceRFC7606 rather than the message-level validator (which has no peer
// configuration to consult).

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
)

// rfc8669PrefixSIDAttrs returns the well-known mandatory attributes followed by a
// well-formed Prefix-SID attribute holding a Label-Index TLV (type 1, length 7).
func rfc8669PrefixSIDAttrs() []byte {
	value := []byte{1, 0, 7, 0, 0, 0, 0x00, 0x00, 0x03, 0x09} // Label Index 777
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}
	attrs = append(attrs, 0xC0, 40, byte(len(value))) // optional transitive, code 40
	return append(attrs, value...)
}

// rfc8669EBGPSession builds an EBGP session (local AS 65001, peer AS 65002) whose
// Prefix-SID acceptance is set by the caller.
func rfc8669EBGPSession(accept bool) *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	settings.AcceptSRv6PrefixSID = accept
	return NewSession(settings)
}

// TestRFC8669PrefixSIDFromEBGPAcceptedWhenConfigured proves the boundary rule does not
// over-fire: an operator who has declared the EBGP neighbor to be inside the SR domain
// keeps the attribute.
//
// VALIDATES: RFC 8669 §4 — "unless it is configured to accept the attribute from the EBGP
// neighbor". With AcceptSRv6PrefixSID set, the UPDATE passes with no action and the
// attribute survives on the wire for the RIB and for propagation.
// PREVENTS: an unconditional strip that would make intra-SR-domain EBGP sessions unable
// to carry Segment Routing information at all.
//
// RFC requirement: RFC8669-4-1 positive -- an EBGP peer configured to be inside the SR domain has its Prefix-SID attribute accepted and left on the wire.
func TestRFC8669PrefixSIDFromEBGPAcceptedWhenConfigured(t *testing.T) {
	s := rfc8669EBGPSession(true)
	body := makeUpdateBody(nil, rfc8669PrefixSIDAttrs(), []byte{24, 10, 0, 0})

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action,
		"a configured EBGP neighbor's Prefix-SID must not be discarded")

	_, _, _, found := attribute.AttrFind(rfc8669PathAttrs(t, wu.Payload()), attribute.AttrPrefixSID)
	assert.True(t, found, "the accepted Prefix-SID attribute must remain in the UPDATE")
}

// TestRFC8669PrefixSIDFromEBGPDiscardedByDefault drives the same UPDATE into a peer that
// has NOT been declared part of the SR domain.
//
// VALIDATES: RFC 8669 §4 — the attribute is discarded, and §6's attribute-discard action
// is the one used (the prefix itself is still installed; the session is not reset).
// PREVENTS: Segment Routing information leaking in from outside the SR domain, where the
// label indices it carries mean nothing and can collide with locally allocated ones.
//
// RFC requirement: RFC8669-4-1 negative -- an EBGP peer NOT configured to be inside the SR domain has its Prefix-SID attribute discarded and removed from the UPDATE.
func TestRFC8669PrefixSIDFromEBGPDiscardedByDefault(t *testing.T) {
	s := rfc8669EBGPSession(false)
	require.False(t, s.settings.AcceptSRv6PrefixSID,
		"acceptance must be opt-in: the default must be to discard")

	body := makeUpdateBody(nil, rfc8669PrefixSIDAttrs(), []byte{24, 10, 0, 0})

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "the boundary rule is an attribute discard, never a session reset")
	assert.Equal(t, message.RFC7606ActionAttributeDiscard, action)

	_, _, _, found := attribute.AttrFind(rfc8669PathAttrs(t, wu.Payload()), attribute.AttrPrefixSID)
	assert.False(t, found, "the discarded Prefix-SID attribute must be gone from the UPDATE")
}

// rfc8669PathAttrs slices the path-attributes section out of an UPDATE body.
func rfc8669PathAttrs(t *testing.T, body []byte) []byte {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4, "UPDATE body must hold both section headers")
	withdrawnLen := int(body[0])<<8 | int(body[1])
	off := 2 + withdrawnLen
	require.LessOrEqual(t, off+2, len(body))
	attrLen := int(body[off])<<8 | int(body[off+1])
	off += 2
	require.LessOrEqual(t, off+attrLen, len(body))
	return body[off : off+attrLen]
}
