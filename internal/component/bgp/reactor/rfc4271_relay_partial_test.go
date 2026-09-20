// Design: docs/architecture/api/process-protocol.md — the stored-route relay rail
// RFC: rfc/short/rfc4271.md — Partial bit semantics (Sections 4.3 and 5)
// Related: session_validation.go — publishBase, the one place that owns these bytes
// Related: relay_payload.go — writeRelayPayload, which copies the flags octet verbatim

package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC4271PartialClearedOnTheRelayedWire drives both halves of the route-server rail
// with one UPDATE: the receive path publishes it, its published attribute bytes become the
// stored route an adj-rib-in plugin holds, and RelayStoredRoute replays that route to a
// second peer. The assertions read the flags octets of the body the destination peer
// receives.
//
// The destination is an RFC 7947 route-server client, which is the rail the defect lived
// on: the egress steps rewrite nothing for such a peer, so every attribute reaches it as
// the reconstruction wrote it, and the reconstruction copies each surviving attribute out
// of the stored block byte for byte.
//
// The received UPDATE carries a Partial bit on three attributes: a well-known ORIGIN
// (0x60), an optional non-transitive MED (0xA0), and a recognized optional transitive
// COMMUNITIES (0xE0). RFC 7606 Section 3(c) checks only the Optional and Transitive bits,
// so none of the three is malformed and ze accepts all of them.
//
// VALIDATES: the ORIGIN and the MED leave ze with the Partial bit cleared, and the
// COMMUNITIES leaves ze with the bit a previous AS set still on it.
//
// PREVENTS: the zero-copy relay putting a flags octet back on the wire that RFC 4271
// Section 4.3 forbids. writeRelayPayload copies each surviving attribute with
// `copy(buf[off:], attrs[s.start:s.end])`, so whatever octet the source peer sent is what
// the destination peer reads unless the receive path has already repaired it. A unit test
// over the walk alone would not see this: the defect was that nothing called it.
//
// RFC requirement: RFC4271-4.3-2 positive -- an UPDATE whose ORIGIN arrives with flags 0x60
// and whose MULTI_EXIT_DISC arrives with flags 0xA0 is relayed to a route-server client with
// those two attributes carrying 0x40 and 0x80, while a COMMUNITIES attribute that arrived
// with 0xE0 is relayed unchanged (internal/component/bgp/reactor/session_validation.go,
// publishBase -> attribute.ClearPartialOnWellKnownAndNonTransitive).
func TestRFC4271PartialClearedOnTheRelayedWire(t *testing.T) {
	// The attribute block of test/plugin/remove-private-as-replace-peer.ci, with the
	// Partial bit set on ORIGIN and on a MED this fixture adds, and with a COMMUNITIES
	// whose Partial bit a previous AS set. The community is 1:1 rather than a well-known
	// one: RFC 1997's egress gate suppresses a NO_EXPORT route toward another AS, and this
	// test needs the route to reach the destination peer.
	attrs := []byte{
		0x60, 0x01, 0x01, 0x00, // ORIGIN, well-known, Partial set on the wire
		0x40, 0x02, 0x0e, 0x02, 0x03, 0x00, 0x00, 0xfb, 0xf0, 0x00, 0x00, 0xfc, 0x00, 0x00, 0x00, 0xfb, 0xf1, // AS_PATH
		0x40, 0x03, 0x04, 0x01, 0x01, 0x01, 0x01, // NEXT_HOP 1.1.1.1
		0xa0, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a, // MED, optional non-transitive, Partial set
		0xe0, 0x08, 0x04, 0x00, 0x01, 0x00, 0x01, // COMMUNITIES 1:1, optional transitive, Partial set by a previous AS
	}
	nlri := []byte{24, 10, 0, 0}

	s := newValidateSession()
	// The AS_PATH above carries four-octet ASNs, and RFC 6793 makes that the wire form only
	// once AS4 is negotiated. enforceRFC7606 reads the width from the session, and the relay
	// fixture below labels its reconstruction with a four-octet context, so both halves of
	// this test have to agree on it.
	s.negotiated = &capability.Negotiated{ASN4: true}

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, attrs, nlri), 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionNone, action,
		"RFC 7606 Section 3(c) judges the Optional and Transitive bits only, so a Partial bit "+
			"the RFC forbids is not a malformed attribute and the UPDATE is published")

	// What an adj-rib-in plugin stores: the PUBLISHED attribute section, hex-encoded.
	// rib_structured.go takes the same bytes from RawMessage.AttrsWire.Packed().
	published, err := wu.Attrs()
	require.NoError(t, err, "the published UPDATE must index")
	route := rpc.StoredRoute{
		SourcePeer: "10.0.0.1",
		Family:     "ipv4/unicast",
		AttrHex:    hex.EncodeToString(published.Packed()),
		NextHopHex: "01010101",
		NLRIHex:    "180a0000",
	}

	api, _, dispatched, mu, done := relayFixture(t)
	// The RS-client leaf is the only thing that selects the transparent path: with it the
	// AS_PATH is not prepended and RFC 4271 Section 5.1.4 lets the MED through, so what the
	// destination reads is the reconstruction itself rather than a rebuild of it.
	dst := api.r.peers[netip.MustParseAddrPort("10.0.0.2:179")]
	require.NotNil(t, dst, "fixture must expose the destination peer")
	dst.settings.RSClient = true
	dst.refreshForwardFacts()

	relayed := relayDispatchedBodyFor(t, api, dispatched, mu, done, route)

	assert.Equal(t, byte(0x40), rfc4271PublishedFlags(t, relayed, byte(attribute.AttrOrigin)),
		"RFC 4271 Section 4.3: the Partial bit must be 0 for a well-known attribute, and the "+
			"relay is what puts this octet on the wire")
	assert.Equal(t, byte(0x80), rfc4271PublishedFlags(t, relayed, byte(attribute.AttrMED)),
		"RFC 4271 Section 4.3: the Partial bit must be 0 for an optional non-transitive attribute")
	assert.Equal(t, byte(0xe0), rfc4271PublishedFlags(t, relayed, byte(attribute.AttrCommunity)),
		"RFC 4271 Section 5: a Partial bit set by a previous AS on an optional transitive "+
			"attribute must not be set back to 0 by the current AS")
}
