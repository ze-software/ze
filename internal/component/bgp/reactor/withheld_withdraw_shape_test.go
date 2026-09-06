// Design: docs/architecture/update-building.md -- "A Withdrawal Names a Route This Connection Advertised"
// Related: reactor_api_batch.go -- withdrawBatchFromPeers, buildWithheldWithdrawUpdate
// Overview: group_updates_framing_test.go -- the peer, reactor and frame-reading harness these borrow
//
// A withdrawal this connection may not name a route for still puts the
// withdrawal's path attributes on the wire, with no route in them. RFC 4271
// Section 6.3: "An UPDATE message that contains correct path attributes, but no
// NLRI, SHALL be treated as a valid UPDATE message." Nothing in an RFC asks a
// speaker to SEND that message; it is the ExaBGP compatibility contract, and
// test/exabgp-compat/api/api-flow.ci is where upstream recorded it.
//
// These tests read the BYTES the peer's connection received, because the shape
// is the whole behavior: a test that asked the builder for a message would pass
// against a rail that built one and wrote nothing.
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

// ipv4FlowSpec is the family whose withdrawal carries path attributes of its
// own, which is what makes an attributes-only message possible for it.
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

// attributeCodes reads the type code of every attribute in a block.
func attributeCodes(t *testing.T, block []byte) []uint8 {
	t.Helper()
	var codes []uint8
	for off := 0; off < len(block); {
		require.GreaterOrEqual(t, len(block)-off, 3, "an attribute carries flags, code and a length")
		flags := block[off]
		length := int(block[off+2])
		header := 3
		// RFC 4271 Section 4.3: the Extended Length bit makes the length two octets.
		if flags&0x10 != 0 {
			require.GreaterOrEqual(t, len(block)-off, 4, "an extended-length attribute carries two length octets")
			length = int(binary.BigEndian.Uint16(block[off+2 : off+4]))
			header = 4
		}
		codes = append(codes, block[off+1])
		require.LessOrEqual(t, off+header+length, len(block), "the attribute must fit its block")
		off += header + length
	}
	return codes
}

// TestWithheldFlowWithdrawWritesItsAttributesWithNoRoute is api-flow's second
// frame.
//
// VALIDATES: a FlowSpec withdrawal on a connection that has advertised nothing
// puts ONE UPDATE on the wire, carrying the attributes the withdrawal would
// have carried, no MP_UNREACH_NLRI, no withdrawn routes and no NLRI. The
// harness peer is external, so RFC 4271 Section 5.1.5 leaves LOCAL_PREF out and
// ORIGIN and AS_PATH are what the message owes; api-flow's own peer is internal
// and its fixture pins the third attribute.
// PREVENTS: the silence that left api-flow waiting for that frame until its
// read deadline, and the MP_UNREACH_NLRI that would name a route RFC 4271
// Section 4.3 says this connection cannot name.
func TestWithheldFlowWithdrawWritesItsAttributesWithNoRoute(t *testing.T) {
	peer, conn := flowSpecPeer(t, "10.0.0.2")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	err := adapter.WithdrawNLRIBatch(selector.All(), flowWithdrawBatch(t), plugin.OperatorSender())
	require.Error(t, err, "the answer still carries the reason the routes were withheld")

	bodies := updateBodies(t, conn.written())
	require.Len(t, bodies, 1, "one UPDATE reaches the peer")

	withdrawn, attributes, reachable := updateSections(t, bodies[0])
	assert.Empty(t, withdrawn, "no withdrawn routes")
	assert.Empty(t, reachable, "no NLRI")
	assert.Equal(t, []uint8{1, 2}, attributeCodes(t, attributes),
		"ORIGIN and AS_PATH, and no MP_UNREACH_NLRI (code 15)")
	assert.Equal(t, uint64(1), peer.adjOut.withheldCount(), "the route is still counted as withheld")
}

// TestWithheldFlowWithdrawArmsNothing keeps the message from arming the guard it
// was produced by.
//
// RFC 4271 Section 4.3 scopes a withdrawn route to a connection that was
// previously advertised the route, and a message with no NLRI makes no
// destination reachable. If it armed the connection, the NEXT withdrawal would
// name a route the peer was never sent.
//
// VALIDATES: after the withheld message, the connection has still advertised
// nothing, so a second withdrawal is withheld exactly as the first was.
// PREVENTS: an attributes-only UPDATE counting as an advertisement.
func TestWithheldFlowWithdrawArmsNothing(t *testing.T) {
	peer, conn := flowSpecPeer(t, "10.0.0.2")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.Error(t, adapter.WithdrawNLRIBatch(selector.All(), flowWithdrawBatch(t), plugin.OperatorSender()))
	assert.False(t, peer.hasAdvertised(), "a message with no NLRI makes no destination reachable")

	require.Error(t, adapter.WithdrawNLRIBatch(selector.All(), flowWithdrawBatch(t), plugin.OperatorSender()))

	bodies := updateBodies(t, conn.written())
	require.Len(t, bodies, 2, "the second withdrawal is withheld the same way")
	_, _, reachable := updateSections(t, bodies[1])
	assert.Empty(t, reachable, "and it names no route either")
	assert.Equal(t, uint64(2), peer.adjOut.withheldCount())
}

// TestWithheldUnicastWithdrawWritesNothing is the other half of the rule, and it
// is what keeps the compatibility contract from reaching a family it was never
// recorded for.
//
// A withdrawal whose family carries no path attributes of its own -- IPv4
// unicast in the Withdrawn Routes field, IPv6 unicast as a bare MP_UNREACH_NLRI
// -- has nothing left once the routes are removed, so nothing is written.
// Upstream sends nothing for those too, which `api-fast` records.
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
	require.Error(t, adapter.WithdrawNLRIBatch(selector.All(), batch, plugin.OperatorSender()))

	assert.Empty(t, updateBodies(t, conn.written()),
		"a withdrawal with no attributes of its own leaves nothing to write")
	assert.Equal(t, uint64(1), peer.adjOut.withheldCount())
}
