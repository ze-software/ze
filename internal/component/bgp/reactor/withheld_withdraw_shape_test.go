// Design: docs/architecture/update-building.md -- "A Withdrawal Names a Route This Connection Advertised"
// Related: reactor_api_batch.go -- withdrawBatchFromPeers
// Overview: group_updates_framing_test.go -- the peer, reactor and frame-reading harness these borrow
//
// A withdrawal this connection may not name a route for puts NOTHING on the
// wire, whatever its family. RFC 4271 Section 4.3 scopes a withdrawn route to
// the connection it was previously advertised on, and with the routes removed
// a withdrawal has nothing left to say. ExaBGP at the pinned 685f772e4 agrees:
// its qa/api/api-flow.ci records no attributes-only frame, and
// UpdateCollection._mp_family_messages yields nothing for a withdrawal while
// include_withdraw is False and nothing is announced.
//
// These tests read the BYTES the peer's connection received, because silence
// on the wire is the whole behavior.
package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ipv4FlowSpec is a family whose withdrawal carries path attributes of its own,
// so removing the routes would still leave a message to write.
var ipv4FlowSpec = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

// flowWithdrawBatch is one FlowSpec withdrawal, the shape api-flow's script
// writes before it has announced anything.
//
// The NLRI bytes are api-flow's own component encoding: destination 0.0.0.0/32,
// source 0.0.0.0/32, protocol =tcp, destination-port =3128. They travel as
// opaque wire, because what this test measures is whether they reach the peer
// at all.
func flowWithdrawBatch(t *testing.T) bgptypes.NLRIBatch {
	t.Helper()
	wire := []byte{
		0x13,
		0x01, 0x20, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x20, 0x00, 0x00, 0x00, 0x00,
		0x03, 0x81, 0x06,
		0x05, 0x91, 0x0c, 0x38,
	}
	flow, err := nlri.NewWireNLRI(ipv4FlowSpec, wire, false)
	require.NoError(t, err)
	return bgptypes.NLRIBatch{
		Family: ipv4FlowSpec,
		NLRIs:  []nlri.NLRI{flow},
	}
}

// flowSpecPeer is a peer that has negotiated ipv4/flow beside IPv4 unicast and
// has advertised nothing, so a withdrawal to it is withheld.
func flowSpecPeer(t *testing.T, addr string) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newGroupUpdatesPeer(t, addr, "absent")
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{
			family.IPv4Unicast: true,
			ipv4FlowSpec:       true,
		},
	})
	require.False(t, peer.hasAdvertised(), "the fixture must start with nothing advertised")
	return peer, conn
}

// updateSections cuts one UPDATE body into the three fields RFC 4271
// Section 4.3 gives it: withdrawn routes, path attributes, and NLRI.
func updateSections(t *testing.T, body []byte) (withdrawn, attributes, reachable []byte) {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4, "an UPDATE body carries both length fields")
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	require.LessOrEqual(t, 2+withdrawnLen+2, len(body), "withdrawn length must fit the body")
	attrLen := int(binary.BigEndian.Uint16(body[2+withdrawnLen : 4+withdrawnLen]))
	require.LessOrEqual(t, 4+withdrawnLen+attrLen, len(body), "attribute length must fit the body")
	return body[2 : 2+withdrawnLen],
		body[4+withdrawnLen : 4+withdrawnLen+attrLen],
		body[4+withdrawnLen+attrLen:]
}

// TestWithheldFlowWithdrawWritesNothing is api-flow's first withdrawal.
//
// VALIDATES: a FlowSpec withdrawal on a connection that has advertised nothing
// puts no UPDATE on the wire, though the family's withdrawal carries path
// attributes; the route is counted as withheld and the answer names the reason.
// PREVENTS: the attributes-only UPDATE ze used to send here (no RFC asks for
// it, and neither ExaBGP 5.0.13 nor the pinned ExaBGP 6.0 sends it), and the
// MP_UNREACH_NLRI that would name a route RFC 4271 Section 4.3 says this
// connection cannot name.
func TestWithheldFlowWithdrawWritesNothing(t *testing.T) {
	peer, conn := flowSpecPeer(t, "10.0.0.2")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	err := adapter.WithdrawNLRIBatch(t.Context(), selector.All(), flowWithdrawBatch(t), plugin.OperatorSender())
	require.Error(t, err, "the answer still carries the reason the routes were withheld")

	assert.Empty(t, updateBodies(t, conn.written()), "a withheld withdrawal writes nothing")
	assert.Equal(t, uint64(1), peer.adjOut.withheldCount(), "the route is still counted as withheld")
}

// TestWithheldFlowWithdrawArmsNothing keeps a withheld withdrawal from arming
// the guard that withheld it.
//
// RFC 4271 Section 4.3 scopes a withdrawn route to a connection that was
// previously advertised the route. If a withheld withdrawal armed the
// connection, the NEXT withdrawal would name a route the peer was never sent.
//
// VALIDATES: after the withheld withdrawal, the connection has still advertised
// nothing, so a second withdrawal is withheld exactly as the first was.
// PREVENTS: a withheld withdrawal counting as an advertisement.
func TestWithheldFlowWithdrawArmsNothing(t *testing.T) {
	peer, conn := flowSpecPeer(t, "10.0.0.2")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.Error(t, adapter.WithdrawNLRIBatch(t.Context(), selector.All(), flowWithdrawBatch(t), plugin.OperatorSender()))
	assert.False(t, peer.hasAdvertised(), "a withheld withdrawal makes no destination reachable")

	require.Error(t, adapter.WithdrawNLRIBatch(t.Context(), selector.All(), flowWithdrawBatch(t), plugin.OperatorSender()))

	assert.Empty(t, updateBodies(t, conn.written()), "the second withdrawal is withheld the same way")
	assert.Equal(t, uint64(2), peer.adjOut.withheldCount())
}

// TestWithheldUnicastWithdrawWritesNothing is the same rule for a family whose
// withdrawal carries no path attributes of its own: IPv4 unicast in the
// Withdrawn Routes field, IPv6 unicast as a bare MP_UNREACH_NLRI. Upstream sends
// nothing for those either, which `api-fast` records.
//
// VALIDATES: an IPv6 unicast withdrawal on an unadvertised connection puts no
// UPDATE on the wire.
// PREVENTS: an empty UPDATE, which carries neither routes nor attributes and is
// not the message RFC 4271 Section 6.3 makes valid.
func TestWithheldUnicastWithdrawWritesNothing(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{family.IPv6Unicast: true},
	})
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	batch := bgptypes.NLRIBatch{
		Family: family.IPv6Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/48"), 0)},
	}
	require.Error(t, adapter.WithdrawNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))

	assert.Empty(t, updateBodies(t, conn.written()),
		"a withdrawal with no attributes of its own leaves nothing to write")
	assert.Equal(t, uint64(1), peer.adjOut.withheldCount())
}
