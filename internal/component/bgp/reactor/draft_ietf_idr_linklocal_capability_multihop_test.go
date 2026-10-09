// Design: docs/architecture/bgp/structural-forwarding.md -- the next hop a relayed route carries
// RFC: rfc/short/draft-ietf-idr-linklocal-capability.md
// Related: forward_next_hop.go -- the egress next-hop decisions of the forward rails
// Related: rfc_draft_linklocal_reflect_test.go -- llnhForward, the forward rail harness
// Overview: rfc_draft_linklocal_test.go -- the same Section 4 sentence under next hop self
//
// draft-ietf-idr-linklocal-capability Section 4: "When sending a message to an
// external peer X, and the peer is multiple IP hops away from the speaker (aka
// "multihop EBGP"): * Link-Local IPv6 next hops MUST NOT be included."
// These tests relay a route whose received MP_REACH_NLRI Next Hop field is the
// 32-octet Global plus Link-Local pair through forwardUpdateCore, under the next
// hop modes that leave the received next hop in place, and read the field each
// external destination was asked to write.
package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// llnhMultihopAddr is an external peer no connected subnet holds: more than one
// IP hop away from the speaker.
const llnhMultihopAddr = "2001:db8:ff::1"

// llnhReceivedPairPayload is an announcement of 2001:db8:7::/64 whose MP_REACH
// Next Hop field is the 32-octet RFC 2545 Section 3 pair the advertiser sent:
// its Global 2001:db8:1::1 followed by its Link-Local fe80::9.
func llnhReceivedPairPayload() []byte {
	global := netip.MustParseAddr(llnhAdvertiserAddr).As16()
	linkLocal := netip.MustParseAddr("fe80::9").As16()
	// AFI 2 (IPv6), SAFI 1 (unicast), Length of Next Hop Network Address 32.
	value := []byte{0x00, 0x02, 0x01, 0x20}
	value = append(value, global[:]...)
	value = append(value, linkLocal[:]...)
	// The Reserved octet, then 2001:db8:7::/64 as one NLRI.
	value = append(value, 0x00, 0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x07, 0x00, 0x00)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x04, 0x02, 0x01, 0xfd, 0xe9, // AS_PATH 65001
		0x80, 0x0e, byte(len(value)), // MP_REACH_NLRI header
	}
	attrs = append(attrs, value...)
	return buildUpdatePayload(attrs, nil)
}

// llnhExternalPeer builds an established external peer (AS 65002) at addr whose
// configured next-hop mode is mode. connected is the interface table its link
// scope is settled against. It negotiated IPv6 unicast and the Link-Local Next
// Hop Capability, so the draft's procedures apply to its session (Section 2).
func llnhExternalPeer(t *testing.T, addr string, connected []netip.Prefix, mode uint8) *Peer {
	t.Helper()
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       netip.MustParseAddr(addr),
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65002,
		RouterID:      0x0a000001,
		NextHopMode:   mode,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:         map[family.Family]bool{family.IPv6Unicast: true},
		LinkLocalNextHop: true,
	})
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.llScope.Store(newLinkScopeFrom(connected, peer.settings.Address))
	peer.fwdFacts.Store(peer.buildForwardFacts())
	return peer
}

// llnhExternalSource is a route learned from the external advertiser.
var llnhExternalSource = forwardSourceInfo{resolved: true, globalLocalAS: 65000}

// TestLinkLocalReceivedPairStrippedForMultihopExternalPeer relays the pair to a
// multihop external peer under both modes that keep the received next hop.
//
// VALIDATES: the field written to the multihop external peer is the 16-octet
// Global 2001:db8:1::1 alone, under next hop unchanged and under the default
// next hop auto.
// PREVENTS: the received Link-Local fe80::9 crossing to a peer that cannot
// reach it, which the next-hop-self units never exercised.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 positive -- a route received with the 32-octet Global 2001:db8:1::1 plus Link-Local fe80::9 next hop and relayed through forwardUpdateCore to an external peer no connected subnet holds (multihop EBGP), under next hop unchanged and under next hop auto, is written with the Global alone: a 16-octet Next Hop field holding no Link-Local.
func TestLinkLocalReceivedPairStrippedForMultihopExternalPeer(t *testing.T) {
	global := netip.MustParseAddr(llnhAdvertiserAddr).As16()
	for _, mode := range []uint8{NextHopUnchanged, NextHopAuto} {
		multihop := llnhExternalPeer(t, llnhMultihopAddr, llnhSegment, mode)

		got := llnhForward(t, llnhReceivedPairPayload(), llnhExternalSource, multihop)

		field, sent := got[netip.MustParseAddr(llnhMultihopAddr)]
		require.True(t, sent, "mode %d: the route reached the multihop peer", mode)
		assert.Equal(t, global[:], field, "mode %d: the Global alone, no Link-Local", mode)
	}
}

// TestLinkLocalReceivedPairKeptForDirectlyAttachedExternalPeer is the other side:
// the same relay to an external peer on the advertiser's subnet.
//
// VALIDATES: a directly attached external peer still receives the 32-octet pair
// with the received Link-Local fe80::9 (Section 4: "the speaker can use the
// received Link-Local IPv6 address, provided that peer X is directly
// attached"), so the removal is keyed on the hop count.
// PREVENTS: a fix that strips every received Link-Local from every external peer.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 negative -- the same relayed route sent to an external peer one IP hop away (2001:db8:1::3, on the connected 2001:db8:1::/64) keeps the 32-octet field: the Global 2001:db8:1::1 followed by the received Link-Local fe80::9.
func TestLinkLocalReceivedPairKeptForDirectlyAttachedExternalPeer(t *testing.T) {
	attached := llnhExternalPeer(t, llnhOnSegmentAddr, llnhSegment, NextHopUnchanged)

	got := llnhForward(t, llnhReceivedPairPayload(), llnhExternalSource, attached)

	field, sent := got[netip.MustParseAddr(llnhOnSegmentAddr)]
	require.True(t, sent, "the route reached the directly attached peer")
	global := netip.MustParseAddr(llnhAdvertiserAddr).As16()
	linkLocal := netip.MustParseAddr("fe80::9").As16()
	assert.Equal(t, append(global[:], linkLocal[:]...), field, "the Global then the received Link-Local")
}

// llnhForwardRS relays payload, received from the external advertiser at
// 2001:db8:1::1, through the ROUTE-SERVER rail (reactorForwardRS) to clients,
// and returns the MP_REACH Next Hop field each client was asked to write.
func llnhForwardRS(t *testing.T, payload []byte, clients ...*Peer) map[netip.Addr][]byte {
	t.Helper()
	return llnhForwardRSRead(t, payload, llnhItemNextHopField, clients...)
}

// llnhForwardRSWritten is llnhForwardRS reading both the announced next hop and
// the withdrawal each client was written (llnhItemWritten).
func llnhForwardRSWritten(t *testing.T, payload []byte, clients ...*Peer) map[netip.Addr]llnhWritten {
	t.Helper()
	return llnhForwardRSRead(t, payload, llnhItemWritten, clients...)
}

// llnhForwardRSRead is the harness both read through.
func llnhForwardRSRead[T any](t *testing.T, payload []byte, read func(*testing.T, []fwdItem) T, clients ...*Peer) map[netip.Addr]T {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)

	type delivery struct {
		addr  netip.Addr
		field T
	}
	delivered := make(chan delivery, 8)
	acc := newFwdAccumulator()
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		delivered <- delivery{addr: k.peerAddr.Addr(), field: read(t, acc.add(k.peerAddr.Addr(), items))}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(clients)+1)
	for _, c := range clients {
		key := fwdKey{peerAddr: c.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = c
	}
	source := llnhExternalPeer(t, llnhAdvertiserAddr, llnhSegment, NextHopUnchanged)
	peerMap[source.Settings().PeerKey()] = source

	r := &Reactor{
		attrModHandlers:     attrModHandlersWithDefaults(),
		recentUpdates:       cache,
		peers:               peerMap,
		fwdPool:             pool,
		rsForwardingEnabled: true,
	}
	reactorForwardRS(r, update, id, source.Settings().Address, source)

	got := make(map[netip.Addr]T, len(clients))
	for range clients {
		select {
		case d := <-delivered:
			got[d.addr] = d.field
		case <-time.After(500 * time.Millisecond):
			return got
		}
	}
	fwdDrainGrace(delivered, func(d delivery) { got[d.addr] = d.field })
	return got
}

// TestLinkLocalRouteServerStripsReceivedPairForMultihopClient relays the pair
// through the route-server rail to a client no connected subnet holds.
//
// VALIDATES: reactorForwardRS asks the multihop client to write the 16-octet
// Global 2001:db8:1::1 alone, under next hop unchanged and under next hop auto.
// PREVENTS: the route-server rail losing the cut the general rail makes: the
// forwardUpdateCore units above never reach reactorForwardRS.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 positive -- a route received with the 32-octet Global 2001:db8:1::1 plus Link-Local fe80::9 next hop and relayed through the route-server rail (reactorForwardRS) to an external client no connected subnet holds (multihop EBGP), under next hop unchanged and under next hop auto, is written with the Global alone: a 16-octet Next Hop field holding no Link-Local.
func TestLinkLocalRouteServerStripsReceivedPairForMultihopClient(t *testing.T) {
	global := netip.MustParseAddr(llnhAdvertiserAddr).As16()
	for _, mode := range []uint8{NextHopUnchanged, NextHopAuto} {
		multihop := llnhExternalPeer(t, llnhMultihopAddr, llnhSegment, mode)

		got := llnhForwardRS(t, llnhReceivedPairPayload(), multihop)

		field, sent := got[netip.MustParseAddr(llnhMultihopAddr)]
		require.True(t, sent, "mode %d: the route reached the multihop client", mode)
		assert.Equal(t, global[:], field, "mode %d: the Global alone, no Link-Local", mode)
	}
}

// TestLinkLocalRouteServerKeepsReceivedPairForAttachedClient is the other side
// on the route-server rail: a client on the advertiser's subnet.
//
// VALIDATES: reactorForwardRS still hands a directly attached client the
// 32-octet pair with the received Link-Local fe80::9, so the rail's cut is
// keyed on the hop count.
// PREVENTS: a route-server fix that strips every received Link-Local.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 negative -- the same route relayed through the route-server rail (reactorForwardRS) to an external client one IP hop away (2001:db8:1::3, on the connected 2001:db8:1::/64) keeps the 32-octet field: the Global 2001:db8:1::1 followed by the received Link-Local fe80::9.
func TestLinkLocalRouteServerKeepsReceivedPairForAttachedClient(t *testing.T) {
	attached := llnhExternalPeer(t, llnhOnSegmentAddr, llnhSegment, NextHopUnchanged)

	got := llnhForwardRS(t, llnhReceivedPairPayload(), attached)

	field, sent := got[netip.MustParseAddr(llnhOnSegmentAddr)]
	require.True(t, sent, "the route reached the directly attached client")
	global := netip.MustParseAddr(llnhAdvertiserAddr).As16()
	linkLocal := netip.MustParseAddr("fe80::9").As16()
	assert.Equal(t, append(global[:], linkLocal[:]...), field, "the Global then the received Link-Local")
}

// TestEgressNextHopGlobalHalf checks pair trimming through the normalizer used
// by both forwarding rails.
//
// VALIDATES: an on-link destination keeps a pair whose global entity is on-link;
// an off-link destination, or one with no link scope, gets the first half of a
// 32-octet pair or a 48-octet VPN-IPv6 pair, nothing for a 16-octet field, and the
// last MP_REACH Set in mods is asked in place of the payload's field.
// PREVENTS: the VPN form or a filter-written pair escaping the removal.
func TestEgressNextHopGlobalHalf(t *testing.T) {
	pair := make([]byte, 32)
	copy(pair, netip.MustParseAddr("2001:db8:1::1").AsSlice())
	copy(pair[16:], netip.MustParseAddr("fe80::9").AsSlice())
	vpnPair := make([]byte, 48)
	copy(vpnPair[8:], pair[:16])
	copy(vpnPair[32:], pair[16:])
	normalize := func(dest *Peer, field, rewrite []byte, fam family.Family) []byte {
		t.Helper()
		src := buildMPReachSource(uint16(fam.AFI), byte(fam.SAFI), field, []byte{0x30, 0xfc, 0x00, 0x00})
		var mods filterapi.ModAccumulator
		if rewrite != nil {
			mods.Op(14, filterapi.AttrModSet, rewrite)
		}
		applyEgressNextHopScope(dest, dest.forwardFacts(), &mods, buildUpdatePayload(src, nil), fam)
		out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, mods.Ops())
		require.True(t, ok, "normalization emits the MP_REACH attribute")
		_, _, value, found := attribute.AttrFind(out, attribute.AttrMPReachNLRI)
		require.True(t, found)
		return value[4 : 4+int(value[3])]
	}
	onLink := llnhExternalPeer(t, llnhOnSegmentAddr, llnhSegment, NextHopUnchanged)
	offLink := llnhExternalPeer(t, llnhMultihopAddr, llnhSegment, NextHopUnchanged)
	unscoped := llnhExternalPeer(t, llnhMultihopAddr, llnhSegment, NextHopUnchanged)
	unscoped.llScope.Store(nil)

	assert.Equal(t, pair, normalize(onLink, pair, nil, family.IPv6Unicast), "an on-link destination keeps the pair")
	assert.Equal(t, pair[:16], normalize(offLink, pair, nil, family.IPv6Unicast), "an off-link destination loses the Link-Local")
	assert.Equal(t, pair[:16], normalize(unscoped, pair, nil, family.IPv6Unicast), "no link scope proves no shared subnet")
	assert.Equal(t, vpnPair[:24], normalize(offLink, vpnPair, nil, family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}), "RD and Global kept")
	assert.Equal(t, pair[:16], normalize(offLink, pair[:16], nil, family.IPv6Unicast), "an off-link Global alone needs no change")
	assert.Equal(t, pair[:16], normalize(offLink, pair[:16], pair, family.IPv6Unicast), "a pair written by a rewrite is asked, not the payload's field")

	offSubnetPair := append([]byte(nil), pair...)
	copy(offSubnetPair, netip.MustParseAddr("2001:db8:ff::9").AsSlice())
	assert.Equal(t, offSubnetPair[:16], normalize(onLink, pair, offSubnetPair, family.IPv6Unicast), "the rewritten global entity, not the received one, decides subnet membership")
	assert.Equal(t, pair, normalize(onLink, offSubnetPair, pair, family.IPv6Unicast), "an on-link rewrite restores the common-subnet condition")
}

// TestMPReachNextHopHandler_Rewrite48To24Bytes applies the cut
// applyEgressNextHopScope makes to a VPN-IPv6 pair.
//
// VALIDATES: a 24-octet Set (RD + Global) replaces a 48-octet next hop, the
// length octet becomes 24, and the Reserved octet and NLRI follow unchanged.
// PREVENTS: the handler refusing the 24-octet form and leaving the Link-Local.
func TestMPReachNextHopHandler_Rewrite48To24Bytes(t *testing.T) {
	oldNH := make([]byte, 48)
	copy(oldNH[8:], netip.MustParseAddr("2001:db8:1::1").AsSlice())
	copy(oldNH[32:], netip.MustParseAddr("fe80::9").AsSlice())
	nlri := []byte{0x30, 0xfc, 0x00, 0x00}
	src := buildMPReachSource(2, 128, oldNH, nlri)
	ops := []filterapi.AttrOp{{Code: 14, Action: filterapi.AttrModSet, Buf: oldNH[:24]}}

	out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, ops)

	require.True(t, ok, "handler planned an emitted attribute")
	require.Equal(t, len(src)-24, len(out), "length shrank by 24 octets")
	val := out[3:]
	assert.Equal(t, byte(24), val[3], "NH length updated to 24")
	assert.Equal(t, oldNH[:24], val[4:28], "RD and Global written")
	assert.Equal(t, byte(0), val[28], "reserved byte preserved")
	assert.Equal(t, nlri, val[29:], "NLRI preserved")
}
