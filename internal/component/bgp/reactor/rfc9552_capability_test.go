// Design: docs/architecture/wire/nlri-bgpls.md -- BGP-LS family negotiation
// RFC: rfc/short/rfc9552.md -- Section 5.2, Link-State NLRI only between capable speakers
// Overview: reactor_api_batch.go -- AnnounceNLRIBatch, where a batch meets each peer's families
// Related: ../../../core/bgp/capability/rfc7752_bgpls_test.go -- the capability encoding and intersection

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// rfc9552AnnounceToPeer announces one Link-State Node NLRI, next hop 192.0.2.1, to an
// established peer 10.0.0.2 whose negotiated families are the ones given. It returns the
// UPDATE bodies written to the peer's connection and the batch error.
func rfc9552AnnounceToPeer(t *testing.T, families map[family.Family]bool) ([][]byte, error) {
	t.Helper()
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	peer.negotiated.Store(&NegotiatedCapabilities{families: families})
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	wn, err := nlri.NewWireNLRI(lsFam, lsNodeNLRI(65001), false)
	require.NoError(t, err)
	attrs := attribute.NewBuilder()
	attrs.SetOrigin(uint8(attribute.OriginIGP))
	batch := bgptypes.NLRIBatch{
		Family:  lsFam,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
		Attrs:   attrs,
	}
	announceErr := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
	return updateBodies(t, conn.written()), announceErr
}

// TestRFC9552LinkStateSentToCapablePeer announces a Link-State NLRI to a peer that
// negotiated the BGP-LS family.
//
// VALIDATES: RFC 9552 Section 5.2 -- once the Multiprotocol capability for (16388, 71) is
// negotiated, the Link-State NLRI reaches the peer in an MP_REACH_NLRI for that family.
// PREVENTS: a negative unit below that passes because nothing is ever sent at all.
//
// RFC requirement: RFC9552-5.2-7 positive -- to an established peer that negotiated AFI 16388 / SAFI 71, an announced Link-State Node NLRI leaves in one UPDATE whose MP_REACH_NLRI names AFI 16388, SAFI 71.
// RFC requirement: RFC7752-3.2-1 positive -- Link-State NLRI is exchanged once the capability is negotiated: to an established peer that negotiated AFI 16388 / SAFI 71 through the Multiprotocol capability, an announced Link-State Node NLRI leaves in one UPDATE whose MP_REACH_NLRI names AFI 16388, SAFI 71.
func TestRFC9552LinkStateSentToCapablePeer(t *testing.T) {
	bodies, err := rfc9552AnnounceToPeer(t, map[family.Family]bool{family.IPv4Unicast: true, lsFam: true})
	require.NoError(t, err)
	require.Len(t, bodies, 1, "one UPDATE reaches the capable peer")

	_, attributes, _ := updateSections(t, bodies[0])
	_, value, ok := findPathAttr(attributes, byte(attribute.AttrMPReachNLRI))
	require.True(t, ok, "the Link-State NLRI travels in MP_REACH_NLRI")
	require.GreaterOrEqual(t, len(value), 3)
	assert.Equal(t, []byte{0x40, 0x04, 0x47}, value[:3], "MP_REACH_NLRI names AFI 16388, SAFI 71")
}

// TestRFC9552LinkStateNeverSentToIncapablePeer announces the same NLRI to a peer whose
// negotiated families hold IPv4 unicast only.
//
// VALIDATES: RFC 9552 Section 5.2 -- "For two BGP Speakers to exchange Link-State NLRI,
// they MUST use BGP Capabilities Advertisement to ensure that they are both capable of
// properly processing such NLRI." Without the negotiated family nothing is written to the
// peer, and the caller is told no peer accepted the family.
// PREVENTS: Link-State NLRI sent to a speaker that never said it can process it.
//
// RFC requirement: RFC9552-5.2-7 negative -- to an established peer that negotiated IPv4 unicast only, the same announcement writes no UPDATE to the connection and returns ErrNoPeersAcceptedFamily.
// RFC requirement: RFC7752-3.2-1 negative -- without the capability no Link-State NLRI is exchanged: to an established peer that negotiated IPv4 unicast only, the same announcement writes no UPDATE to the connection and returns ErrNoPeersAcceptedFamily.
func TestRFC9552LinkStateNeverSentToIncapablePeer(t *testing.T) {
	bodies, err := rfc9552AnnounceToPeer(t, map[family.Family]bool{family.IPv4Unicast: true})
	require.ErrorIs(t, err, route.ErrNoPeersAcceptedFamily)
	assert.Empty(t, bodies, "nothing reaches a peer that did not negotiate BGP-LS")
}
