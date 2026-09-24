// Design: docs/architecture/core-design.md -- session ingress validation
// RFC: rfc/short/rfc4271.md
// RFC: rfc/short/rfc7611.md

package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgpfilter "github.com/ze-software/ze/internal/component/bgp/reactor/filter"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// VALIDATES: an UPDATE from an external peer whose leftmost AS is not the
// peer's AS is handled as a Malformed AS_PATH, which RFC 7606 Section 7.2
// turns into treat-as-withdraw, and the session stays Established.
// PREVENTS: installing a route whose leftmost AS fails the neighbor check.
func TestRFC4271LeftmostASMismatchIsMalformedASPath(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s, client := firstASSession(t, settings, []capability.Capability{&capability.ASN4{ASN: 65002}}, nil)
	prefix := []byte{24, 203, 0, 113}
	// RFC requirement: RFC4271-6.3-13 positive -- an external UPDATE whose leftmost AS 65003 is not the peer AS 65002 is delivered as a withdrawal of its prefix, not an announcement.
	wu := firstASReceive(t, s, client, makeUpdateBody(nil, firstASAttrs(4, 65003, 65002), prefix))
	require.Equal(t, makeUpdateBody(prefix, nil, nil), wu.Payload())
	// RFC requirement: RFC4271-6.3-13 negative -- an external UPDATE whose leftmost AS is the peer AS is announced unchanged.
	wu = firstASReceive(t, s, client, makeUpdateBody(nil, firstASAttrs(4, 65002, 65003), prefix))
	nlri, err := wu.NLRI()
	require.NoError(t, err)
	require.True(t, bytes.Equal(prefix, nlri), "matching leftmost AS announced %x", nlri)
}

// VALIDATES: when two connections to one peer collide in OpenConfirm, the
// collision decision closes exactly one of them, whichever side has the
// higher BGP Identifier.
// PREVENTS: keeping both colliding connections, or closing both.
func TestRFC4271CollisionClosesExactlyOneConnection(t *testing.T) {
	// RFC requirement: RFC4271-6.8-1 positive -- in OpenConfirm, a collision closes exactly one connection: the incoming one when the local identifier is higher, the existing one when the remote identifier is higher.
	for _, tc := range []struct {
		name             string
		localID, remote  uint32
		wantCloseCurrent bool
	}{
		{"local-wins", 0x0a000002, 0x0a000001, false},
		{"remote-wins", 0x0a000001, 0x0a000002, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session, client, server := setupOpenConfirmSession(t, tc.localID)
			defer func() { _ = client.Close() }()
			defer func() { _ = server.Close() }()
			accept, closeExisting := session.detectCollision(tc.remote)
			closeIncoming := !accept
			require.NotEqual(t, closeIncoming, closeExisting, "exactly one connection closes")
			require.Equal(t, tc.wantCloseCurrent, closeExisting)
		})
	}
}

// VALIDATES: a route this router itself originated (its own ORIGINATOR_ID)
// is refused at ingress even when it carries ACCEPT_OWN, so it never
// re-enters the routing context it came from; a route from another
// originator carrying the same community is accepted.
// PREVENTS: ACCEPT_OWN reinjecting a route into its own source context.
func TestRFC7611OwnRouteNeverReacceptedIntoSource(t *testing.T) {
	for _, own := range []bool{true, false} {
		id := uint32(0x01020305)
		if own {
			id = 0x01020304
		}
		attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, byte(attribute.AttrOriginatorID), 4}
		attrs = binary.BigEndian.AppendUint32(attrs, id)
		attrs = append(attrs, 0xc0, byte(attribute.AttrCommunity), 4)
		attrs = binary.BigEndian.AppendUint32(attrs, uint32(attribute.CommunityAcceptOwn))
		accepted, _ := bgpfilter.LoopIngress(filterapi.PeerFilterInfo{LocalAS: 65001, PeerAS: 65001, RouterID: 0x01020304}, makeUpdateBody(nil, attrs, []byte{24, 10, 20, 0}), nil)
		if own {
			// RFC requirement: RFC7611-2.1-1 negative -- a route carrying ACCEPT_OWN whose ORIGINATOR_ID is this router is refused at ingress.
			require.False(t, accepted)
			continue
		}
		// RFC requirement: RFC7611-2.1-1 positive -- a route carrying ACCEPT_OWN from another originator is accepted at ingress.
		require.True(t, accepted)
	}
}
