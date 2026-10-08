// Design: docs/architecture/bgp/structural-forwarding.md -- withheld routes are withdrawn
// Related: forward_aigp.go -- forwardUpdateSelected, the general rail's split
// Related: forward_rs.go -- reactorForwardRS, the route-server rail's split
// Related: forward_next_hop.go -- egressNextHopWithheld, the gates judged per field
//
// A source UPDATE may mix NLRI-bearing fields, and both forward rails split it
// into one section per field before any egress gate. These tests send mixed
// shapes through both rails and read every message each destination was asked
// to write, so they see the split, the per-field gate, and the RFC 7606 Section
// 5.1 sender rule at once.
package reactor

import (
	"encoding/binary"
	"maps"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// fwdAccumulator gives a pool callback the cumulative view of every item one
// destination was handed so far. A mixed UPDATE refused by a next-hop gate
// reaches a destination as one item per section (withdrawalBySection), which
// the pool may hand over in several batches, so a harness that reads one batch
// per destination would see half the answer. Each item's raw bodies are copied,
// because the pool reuses their buffers once the callback returns. Safe for
// concurrent use.
type fwdAccumulator struct {
	mu   sync.Mutex
	seen map[netip.Addr][]fwdItem
}

func newFwdAccumulator() *fwdAccumulator {
	return &fwdAccumulator{seen: make(map[netip.Addr][]fwdItem)}
}

// add records items for addr and returns every item addr was handed so far.
func (a *fwdAccumulator) add(addr netip.Addr, items []fwdItem) []fwdItem {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range items {
		item := fwdItem{updates: items[i].updates}
		for _, body := range items[i].rawBodies {
			item.rawBodies = append(item.rawBodies, append([]byte(nil), body...))
		}
		a.seen[addr] = append(a.seen[addr], item)
	}
	return append([]fwdItem(nil), a.seen[addr]...)
}

// fwdDrainGrace applies every delivery that arrives until the channel has been
// quiet for 100ms: the batches a destination receives after its first.
func fwdDrainGrace[D any](ch <-chan D, apply func(D)) {
	for {
		select {
		case d := <-ch:
			apply(d)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

// mixedMsg is one UPDATE a destination was asked to write, reduced to its four
// NLRI-bearing fields and the MP_REACH_NLRI next-hop field. Every slice is a
// copy, taken while the forward item still owned its buffer.
type mixedMsg struct {
	withdrawn []byte
	nlri      []byte
	reachNH   []byte
	reachNLRI []byte
	unreach   []byte // the whole MP_UNREACH_NLRI value: AFI, SAFI, NLRI
	hasReach  bool
	hasUnrch  bool
}

// fields counts the NLRI-bearing fields the message carries, the count RFC 7606
// Section 5.1 bounds at one.
func (m mixedMsg) fields() int {
	n := 0
	for _, present := range []bool{len(m.withdrawn) > 0, len(m.nlri) > 0, m.hasReach, m.hasUnrch} {
		if present {
			n++
		}
	}
	return n
}

func mixedMsgOf(t *testing.T, u *message.Update) mixedMsg {
	t.Helper()
	m := mixedMsg{
		withdrawn: append([]byte(nil), u.WithdrawnRoutes...),
		nlri:      append([]byte(nil), u.NLRI...),
	}
	if _, _, v, found := attribute.AttrFind(u.PathAttributes, attribute.AttrMPReachNLRI); found {
		require.GreaterOrEqual(t, len(v), 5)
		nhLen := int(v[3])
		require.GreaterOrEqual(t, len(v), 5+nhLen)
		m.hasReach = true
		m.reachNH = append([]byte(nil), v[4:4+nhLen]...)
		m.reachNLRI = append([]byte(nil), v[5+nhLen:]...)
	}
	if _, _, v, found := attribute.AttrFind(u.PathAttributes, attribute.AttrMPUnreachNLRI); found {
		m.hasUnrch = true
		m.unreach = append([]byte(nil), v...)
	}
	return m
}

// mixedRun describes one forward: the rail, the source, and an optional change
// to the reactor before the UPDATE is sent.
type mixedRun struct {
	rs        bool
	source    *Peer             // route-server rail: the advertiser, in the peer table
	info      forwardSourceInfo // general rail: what the caller resolved about the source
	configure func(*Reactor)
}

// mixedForward sends payload through the rail run names and returns every
// message each destination was asked to write, in order. It reads until the
// pool has been quiet for 300ms, because a split UPDATE reaches one destination
// as several items that may arrive in several batches.
func mixedForward(t *testing.T, run mixedRun, payload []byte, dests ...*Peer) map[netip.Addr][]mixedMsg {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)
	if run.source != nil {
		update.SourcePeerIP = run.source.Settings().Address
	} else {
		update.SourcePeerIP = netip.MustParseAddr(llnhAdvertiserAddr)
	}

	var mu sync.Mutex
	got := make(map[netip.Addr][]mixedMsg, len(dests))
	arrived := make(chan struct{}, 64)
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		mu.Lock()
		defer mu.Unlock()
		addr := k.peerAddr.Addr()
		for i := range items {
			for _, body := range items[i].rawBodies {
				u, unpackErr := message.UnpackUpdate(body)
				require.NoError(t, unpackErr)
				got[addr] = append(got[addr], mixedMsgOf(t, u))
			}
			for _, u := range items[i].updates {
				got[addr] = append(got[addr], mixedMsgOf(t, u))
			}
		}
		arrived <- struct{}{}
	}, fwdPoolConfig{chanSize: 64, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(dests)+1)
	for _, d := range dests {
		key := fwdKey{peerAddr: d.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = d
	}
	r := &Reactor{
		attrModHandlers:     attrModHandlersWithDefaults(),
		recentUpdates:       cache,
		peers:               peerMap,
		fwdPool:             pool,
		rsForwardingEnabled: run.rs,
	}
	if run.configure != nil {
		run.configure(r)
	}
	if run.rs {
		require.NotNil(t, run.source, "the route-server rail needs the advertiser")
		peerMap[run.source.Settings().PeerKey()] = run.source
		reactorForwardRS(r, update, id, run.source.Settings().Address, run.source)
	} else {
		adapter := &reactorAPIAdapter{r: r}
		_ = adapter.forwardUpdateCore(update, id, dests, run.info)
	}

	for {
		select {
		case <-arrived:
		case <-time.After(300 * time.Millisecond):
			mu.Lock()
			out := make(map[netip.Addr][]mixedMsg, len(got))
			maps.Copy(out, got)
			mu.Unlock()
			return out
		}
	}
}

// mixedRails is the pair of runs every test here repeats: the general rail with
// an external source, and the route-server rail with the same advertiser.
func mixedRails(t *testing.T, configure func(*Reactor)) []mixedRun {
	t.Helper()
	return []mixedRun{
		{info: llnhExternalSource, configure: configure},
		{rs: true, source: llnhExternalPeer(t, llnhAdvertiserAddr, llnhSegment, NextHopUnchanged), configure: configure},
	}
}

// mixedDest is an external peer that negotiated IPv4 and IPv6 unicast, the
// Link-Local Next Hop Capability, and VPN-IPv6. This speaker's one connected
// subnet is llnhSegment, so an address inside it is on the link and any other is
// more than one IP hop away.
func mixedDest(t *testing.T, addr string) *Peer {
	t.Helper()
	peer := llnhExternalPeer(t, addr, llnhSegment, NextHopUnchanged)
	negotiated := peer.negotiated.Load()
	negotiated.families[family.IPv4Unicast] = true
	negotiated.families[family.Family{AFI: family.AFIIPv6, SAFI: family.SAFI(128)}] = true
	negotiated.LinkLocalNextHop = true
	// buildForwardFacts, not refreshForwardFacts: a refresh settles the link
	// scope against the host's interface table and loses llnhSegment.
	peer.fwdFacts.Store(peer.buildForwardFacts())
	return peer
}

// mixedAttrs is ORIGIN igp and a four-octet AS_PATH of 65001, then extra.
func mixedAttrs(extra ...[]byte) []byte {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9}
	for _, e := range extra {
		attrs = append(attrs, e...)
	}
	return attrs
}

// mixedAttr frames one optional attribute, extended length when it needs it.
func mixedAttr(code byte, value []byte) []byte {
	if len(value) > 255 {
		out := []byte{0x90, code, 0, 0}
		binary.BigEndian.PutUint16(out[2:], uint16(len(value))) //nolint:gosec // fixture
		return append(out, value...)
	}
	return append([]byte{0x80, code, byte(len(value))}, value...)
}

// mixedReach is an IPv6 (AFI 2) MP_REACH_NLRI value of safi with next-hop field
// nh.
func mixedReach(safi byte, nh, nlri []byte) []byte {
	v := []byte{0x00, 0x02, safi, byte(len(nh))}
	v = append(v, nh...)
	v = append(v, 0) // Reserved.
	return mixedAttr(14, append(v, nlri...))
}

// mixedUnreach is an MP_UNREACH_NLRI value of afi/safi.
func mixedUnreachValue(afi uint16, safi byte, nlri []byte) []byte {
	return append([]byte{byte(afi >> 8), byte(afi), safi}, nlri...)
}

var (
	mixedV6Prefix  = []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x07, 0x00, 0x00} // 2001:db8:7::/64
	mixedV4MPPfx   = []byte{24, 10, 9, 0}                                       // 10.9.0.0/24
	mixedV4Legacy  = []byte{24, 10, 0, 0}                                       // 10.0.0.0/24
	mixedV4Withdrn = []byte{24, 198, 51, 100}                                   // 198.51.100.0/24
	mixedGlobalNH  = netip.MustParseAddr("2001:db8:1::1").AsSlice()
	mixedLLNH      = netip.MustParseAddr("fe80::1").AsSlice()
)

// mixedRefuse is an egress step that refuses one destination and passes the
// others, installed on both rails' filter surfaces.
func mixedRefuse(refused netip.Addr) func(*Reactor) {
	refuse := func(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
		return dest.Address != refused
	}
	return func(r *Reactor) {
		r.orderedEgressSteps = orderedEgressStepsFromFuncs(refuse)
		r.egressFilters = []filterapi.EgressFilterFunc{refuse}
	}
}

// requireSingleField asserts RFC 7606 Section 5.1 of every message written.
func requireSingleField(t *testing.T, rail string, msgs []mixedMsg) {
	t.Helper()
	for i, m := range msgs {
		assert.Equal(t, 1, m.fields(), "%s: message %d carries %d NLRI-bearing fields: %+v", rail, i, m.fields(), m)
	}
}

func railName(run mixedRun) string {
	if run.rs {
		return "rs"
	}
	return "general"
}

// TestMixedFamilyUpdateRefusedIsWithdrawnPerFamily forwards one UPDATE that
// announces 2001:db8:7::/64 in MP_REACH_NLRI and withdraws 10.9.0.0/24 in an
// IPv4 MP_UNREACH_NLRI, to a destination an egress step refuses and to one it
// passes, on both rails.
//
// VALIDATES: the refused destination is written the withdrawal of BOTH
// families, each in its own message; the passed one is written the announcement
// and the IPv4 withdrawal, each in its own message. RFC 7606 Section 5.1: "An
// UPDATE message MUST NOT contain more than one of the following: non-empty
// Withdrawn Routes field, non-empty Network Layer Reachability Information
// field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."
// PREVENTS: the refused destination being sent nothing, because one merged
// withdrawal of two families needs two MP_UNREACH_NLRI attributes, and keeping
// both the route it was refused and the route the source withdrew.
func TestMixedFamilyUpdateRefusedIsWithdrawnPerFamily(t *testing.T) {
	refusedAddr := netip.MustParseAddr("2001:db8:1::5")
	passedAddr := netip.MustParseAddr("2001:db8:1::6")
	payload := buildUpdatePayload(mixedAttrs(
		mixedAttr(15, mixedUnreachValue(1, 1, mixedV4MPPfx)),
		mixedReach(1, mixedGlobalNH, mixedV6Prefix),
	), nil)
	wantV4 := mixedUnreachValue(1, 1, mixedV4MPPfx)
	wantV6 := mixedUnreachValue(2, 1, mixedV6Prefix)

	for _, run := range mixedRails(t, mixedRefuse(refusedAddr)) {
		rail := railName(run)
		got := mixedForward(t, run, payload,
			mixedDest(t, refusedAddr.String()), mixedDest(t, passedAddr.String()))

		refused := got[refusedAddr]
		requireSingleField(t, rail, refused)
		var unreach [][]byte
		for _, m := range refused {
			assert.False(t, m.hasReach, "%s: nothing is announced to the refused destination", rail)
			if m.hasUnrch {
				unreach = append(unreach, m.unreach)
			}
		}
		assert.ElementsMatch(t, [][]byte{wantV4, wantV6}, unreach, "%s: both families are withdrawn", rail)

		passed := got[passedAddr]
		requireSingleField(t, rail, passed)
		var announced, withdrawn [][]byte
		for _, m := range passed {
			if m.hasReach {
				announced = append(announced, m.reachNLRI)
			}
			if m.hasUnrch {
				withdrawn = append(withdrawn, m.unreach)
			}
		}
		assert.Equal(t, [][]byte{mixedV6Prefix}, announced, "%s: the passed destination is announced the route", rail)
		assert.Equal(t, [][]byte{wantV4}, withdrawn, "%s: the passed destination is withdrawn the source's IPv4 route", rail)
	}
}

// TestWithdrawnRoutesAndUnreachRefusedAreSentApart forwards one UPDATE carrying
// a Withdrawn Routes field (198.51.100.0/24) and an IPv6 MP_UNREACH_NLRI
// (2001:db8:7::/64) to a destination an egress step refuses, on both rails.
//
// VALIDATES: both withdrawals reach it, each in its own message (RFC 7606
// Section 5.1, quoted above).
// PREVENTS: a converted withdrawal that carries two NLRI-bearing fields in one
// message.
func TestWithdrawnRoutesAndUnreachRefusedAreSentApart(t *testing.T) {
	refusedAddr := netip.MustParseAddr("2001:db8:1::5")
	payload := withdrawMergePayload(mixedV4Withdrn, mixedAttr(15, mixedUnreachValue(2, 1, mixedV6Prefix)), nil)

	for _, run := range mixedRails(t, mixedRefuse(refusedAddr)) {
		rail := railName(run)
		got := mixedForward(t, run, payload, mixedDest(t, refusedAddr.String()))

		msgs := got[refusedAddr]
		requireSingleField(t, rail, msgs)
		var withdrawn, unreach []byte
		for _, m := range msgs {
			withdrawn = append(withdrawn, m.withdrawn...)
			unreach = append(unreach, m.unreach...)
		}
		assert.Equal(t, mixedV4Withdrn, withdrawn, "%s: the IPv4 withdrawal reaches it", rail)
		assert.Equal(t, mixedUnreachValue(2, 1, mixedV6Prefix), unreach, "%s: the IPv6 withdrawal reaches it", rail)
	}
}

// TestMixedUpdateWithdrawsOnlyTheLinkLocalOnlyField forwards one UPDATE that
// announces 10.0.0.0/24 in the legacy NLRI (NEXT_HOP 192.0.2.1) and
// 2001:db8:7::/64 in MP_REACH_NLRI with the Link-Local-only next hop fe80::1, to
// a destination more than one IP hop away, under next hop unchanged, on both
// rails.
//
// VALIDATES: the destination is announced 10.0.0.0/24 and written the
// MP_UNREACH_NLRI that withdraws 2001:db8:7::/64. Draft Section 4: "If, after
// completing these procedures, there are no IPv6 next hop addresses included in
// the next hop, the BGP route MUST not be advertised to its peer. Instead,
// treat-as-withdraw (Section 2 of [RFC7606]) is used." The legacy route carries
// an IPv4 NEXT_HOP the gate never judged.
// PREVENTS: the IPv4 route being withdrawn because the IPv6 route in the same
// message had no usable next hop.
func TestMixedUpdateWithdrawsOnlyTheLinkLocalOnlyField(t *testing.T) {
	destAddr := netip.MustParseAddr(llnhOffSegmentAddr)
	payload := buildUpdatePayload(mixedAttrs(
		[]byte{0x40, 3, 4, 192, 0, 2, 1},
		mixedReach(1, mixedLLNH, mixedV6Prefix),
	), mixedV4Legacy)

	for _, run := range mixedRails(t, nil) {
		rail := railName(run)
		got := mixedForward(t, run, payload, mixedDest(t, destAddr.String()))

		msgs := got[destAddr]
		requireSingleField(t, rail, msgs)
		var nlri, withdrawn, unreach []byte
		for _, m := range msgs {
			assert.False(t, m.hasReach, "%s: no Link-Local-only next hop crosses: %x", rail, m.reachNH)
			nlri = append(nlri, m.nlri...)
			withdrawn = append(withdrawn, m.withdrawn...)
			unreach = append(unreach, m.unreach...)
		}
		assert.Equal(t, mixedV4Legacy, nlri, "%s: the IPv4 route is announced", rail)
		assert.Empty(t, withdrawn, "%s: the IPv4 route is not withdrawn", rail)
		assert.Equal(t, mixedUnreachValue(2, 1, mixedV6Prefix), unreach, "%s: the IPv6 route is withdrawn", rail)
	}
}

// TestUnspecifiedGlobalPairWithdrawnFromMultihopPeer relays the 32-octet next
// hop ":: then fe80::9" under next hop unchanged to a destination more than one
// IP hop away and to one on the link, on both rails.
//
// VALIDATES: both destinations are announced nothing and written the
// withdrawal of 2001:db8:7::/64. The unspecified global address is not a
// next hop, even when the destination is on-link. Capability 77 permits a
// single link-local address, not an unspecified global address in a pair.
// PREVENTS: the route reaching the multihop peer with the next hop ::.
func TestUnspecifiedGlobalPairWithdrawnFromMultihopPeer(t *testing.T) {
	offLink := netip.MustParseAddr(llnhOffSegmentAddr)
	onLink := netip.MustParseAddr(llnhOnSegmentAddr)
	pair := append(make([]byte, 16), netip.MustParseAddr("fe80::9").AsSlice()...)
	payload := buildUpdatePayload(mixedAttrs(mixedReach(1, pair, mixedV6Prefix)), nil)

	for _, run := range mixedRails(t, nil) {
		rail := railName(run)
		got := mixedForward(t, run, payload, mixedDest(t, offLink.String()), mixedDest(t, onLink.String()))

		var unreach []byte
		for _, m := range got[offLink] {
			assert.False(t, m.hasReach, "%s: the multihop destination is announced nothing: %x", rail, m.reachNH)
			unreach = append(unreach, m.unreach...)
		}
		assert.Equal(t, mixedUnreachValue(2, 1, mixedV6Prefix), unreach, "%s: the route is withdrawn", rail)

		var onLinkUnreach []byte
		for _, m := range got[onLink] {
			assert.False(t, m.hasReach, "%s: the on-link destination is announced nothing: %x", rail, m.reachNH)
			onLinkUnreach = append(onLinkUnreach, m.unreach...)
		}
		assert.Equal(t, mixedUnreachValue(2, 1, mixedV6Prefix), onLinkUnreach, "%s: the on-link route is withdrawn", rail)
	}
}

// TestVPNLinkLocalOnlyNextHopWithdrawnFromMultihopPeer relays a VPN-IPv6 route
// whose next-hop field is the 24-octet RD plus Link-Local fe80::1, under next
// hop unchanged, to a destination more than one IP hop away, on both rails.
//
// VALIDATES: it is announced nothing and written the VPN-IPv6 MP_UNREACH_NLRI
// that withdraws the route. Of a multihop external peer, draft Section 4 says:
// "If a Global IPv6 next hop is not included, the route MUST NOT be advertised
// to the external peer (treat-as-withdraw)."
// PREVENTS: the RD-prefixed form escaping the Link-Local-only gate that the
// 16-octet form meets.
func TestVPNLinkLocalOnlyNextHopWithdrawnFromMultihopPeer(t *testing.T) {
	destAddr := netip.MustParseAddr(llnhOffSegmentAddr)
	nh := append(make([]byte, 8), mixedLLNH...) // RD 0:0, then fe80::1
	// Label 16 with the bottom-of-stack bit, RD 0:0, then 2001:db8:7::/64.
	nlri := []byte{24 + 64 + 64, 0x00, 0x01, 0x01}
	nlri = append(nlri, make([]byte, 8)...)
	nlri = append(nlri, mixedV6Prefix[1:]...)
	payload := buildUpdatePayload(mixedAttrs(mixedReach(128, nh, nlri)), nil)

	for _, run := range mixedRails(t, nil) {
		rail := railName(run)
		got := mixedForward(t, run, payload, mixedDest(t, destAddr.String()))

		msgs := got[destAddr]
		require.NotEmpty(t, msgs, "%s: the destination is written the withdrawal", rail)
		var unreach []byte
		for _, m := range msgs {
			assert.False(t, m.hasReach, "%s: no Link-Local-only next hop crosses: %x", rail, m.reachNH)
			unreach = append(unreach, m.unreach...)
		}
		assert.Equal(t, mixedUnreachValue(2, 128, nlri), unreach, "%s: the VPN route is withdrawn", rail)
	}
}

// TestASPathResolveFailureCostsWithheldDestinationNothing relays, on both rails,
// a Link-Local-only route whose AS_PATH this speaker cannot re-encode to two
// external destinations: one more than one IP hop away, which a withhold gate
// refuses, and one on the link, which the gates pass.
//
// The AS_PATH is one AS_SET that declares five four-octet AS numbers and holds
// one. tryShift takes only a leading AS_SEQUENCE, so ASPathEdit.Record parses
// the path (recordPrepend) and fails, which is the resolve failure both rails
// answer by suppressing the route for that destination.
//
// VALIDATES: the multihop destination is still written the withdrawal of
// 2001:db8:7::/64 and announced nothing, and the on-link destination is sent
// nothing at all. A withdrawal carries no AS_PATH, so the path is never
// resolved for it.
// PREVENTS: a rail that resolves the AS_PATH before it asks whether the
// destination is withdrawn, which drops the withdrawal RFC 7606 Section 2
// treat-as-withdraw owes the refused destination and leaves its stale route.
func TestASPathResolveFailureCostsWithheldDestinationNothing(t *testing.T) {
	offLink := netip.MustParseAddr(llnhOffSegmentAddr)
	onLink := netip.MustParseAddr(llnhOnSegmentAddr)
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 1, 5, 0, 0, 0xfd, 0xe9}
	attrs = append(attrs, mixedReach(1, mixedLLNH, mixedV6Prefix)...)
	payload := buildUpdatePayload(attrs, nil)

	for _, run := range mixedRails(t, nil) {
		rail := railName(run)
		got := mixedForward(t, run, payload, mixedDest(t, offLink.String()), mixedDest(t, onLink.String()))

		msgs := got[offLink]
		require.NotEmpty(t, msgs, "%s: the multihop destination is written the withdrawal", rail)
		var unreach []byte
		for _, m := range msgs {
			assert.False(t, m.hasReach, "%s: the multihop destination is announced nothing: %x", rail, m.reachNH)
			unreach = append(unreach, m.unreach...)
		}
		assert.Equal(t, mixedUnreachValue(2, 1, mixedV6Prefix), unreach, "%s: the route is withdrawn", rail)
		assert.Empty(t, got[onLink], "%s: the unresolvable AS_PATH suppresses the route on the link", rail)
	}
}

// TestReceivedLinkLocalOnlyWithdrawnFromPeerWithoutCapability relays, on both
// rails under next hop unchanged, a route received with the Link-Local-only
// next hop fe80::1 to two destinations on the link: one that negotiated the
// Link-Local Next Hop Capability and one that did not.
//
// The next hop is the received one, never one this speaker chose, and the
// refusal still holds: RFC 2545 Section 3 makes the Global address mandatory in
// the next hop field, and draft-ietf-idr-linklocal-capability Section 3 says a
// Link-Local-only Next Hop "received without the Link-Local Next Hop Capability
// having been negotiated is not conformant with [RFC2545]". Section 4 then answers for the route:
// "treat-as-withdraw (Section 2 of [RFC7606]) is used."
//
// VALIDATES: the destination without the capability is announced nothing and
// written the withdrawal of 2001:db8:7::/64, while the one with it is announced
// fe80::1 unchanged.
// PREVENTS: a received Link-Local-only next hop crossing, unchanged, to a
// session that cannot carry it, which the written-next-hop gate never saw.
func TestReceivedLinkLocalOnlyWithdrawnFromPeerWithoutCapability(t *testing.T) {
	without := netip.MustParseAddr(llnhOnSegmentAddr)
	with := netip.MustParseAddr("2001:db8:1::5")
	payload := buildUpdatePayload(mixedAttrs(mixedReach(1, mixedLLNH, mixedV6Prefix)), nil)

	for _, run := range mixedRails(t, nil) {
		rail := railName(run)
		refused := mixedDest(t, without.String())
		refused.negotiated.Load().LinkLocalNextHop = false
		refused.fwdFacts.Store(refused.buildForwardFacts())
		got := mixedForward(t, run, payload, refused, mixedDest(t, with.String()))

		msgs := got[without]
		require.NotEmpty(t, msgs, "%s: the destination without the capability is written the withdrawal", rail)
		var unreach []byte
		for _, m := range msgs {
			assert.False(t, m.hasReach, "%s: no Link-Local-only next hop crosses: %x", rail, m.reachNH)
			unreach = append(unreach, m.unreach...)
		}
		assert.Equal(t, mixedUnreachValue(2, 1, mixedV6Prefix), unreach, "%s: the route is withdrawn", rail)

		require.Len(t, got[with], 1, "%s: the destination with the capability is owed the route", rail)
		assert.Equal(t, mixedLLNH, got[with][0].reachNH, "%s: fe80::1 crosses unchanged", rail)
	}
}

// llnhTwoSegments is an interface table that holds the advertiser's segment and
// the off-segment client's, so every client is directly attached to the speaker.
var llnhTwoSegments = []netip.Prefix{
	netip.MustParsePrefix("2001:db8:1::/64"),
	netip.MustParsePrefix("2001:db8:9::/64"),
}

// TestRouteServerReflectedLinkLocalOnlyWithdrawnFromClientOffTheSegment is the
// route-server twin of
// TestReflectedLinkLocalOnlyRouteWithdrawnFromClientOffTheSegment: an internal
// route-reflector client advertises a Link-Local-only next hop, and the
// route-server rail reflects it to a client off the advertiser's segment and to
// one on it.
//
// The speaker is attached to both segments (llnhTwoSegments), so the client off
// the advertiser's segment is still one hop from this speaker. The off-link gate
// (egressNextHopLinkLocalOnlyOffLink) then passes it, and only the reflection
// gate can withhold the route: a rail that stopped asking it would send fe80::1.
//
// VALIDATES: the client off the segment is announced nothing and written the
// withdrawal of 2001:db8:7::/64; the client on the segment is announced fe80::1
// unchanged. Draft Section 4: "A Route Reflector (RR) reflecting a route with a
// link-local-only next hop MUST NOT advertise that route to a client unless the
// client shares the same link-layer segment as the original advertiser."
// PREVENTS: the route-server rail losing the reflection gate the general rail
// asks.
func TestRouteServerReflectedLinkLocalOnlyWithdrawnFromClientOffTheSegment(t *testing.T) {
	payload := llnhReflectedPayload("fe80::1")
	offSegment := llnhClient(t, llnhOffSegmentAddr, llnhTwoSegments, false /*nextHopSelf*/)
	onSegment := llnhClient(t, llnhOnSegmentAddr, llnhTwoSegments, false /*nextHopSelf*/)
	advertiser := llnhClient(t, llnhAdvertiserAddr, llnhTwoSegments, false /*nextHopSelf*/)
	require.True(t, destOnLink(offSegment), "the client off the segment is directly attached")

	got := mixedForward(t, mixedRun{rs: true, source: advertiser}, payload, offSegment, onSegment)

	off := got[netip.MustParseAddr(llnhOffSegmentAddr)]
	require.NotEmpty(t, off, "the client off the segment is written the withdrawal")
	var unreach []byte
	for _, m := range off {
		assert.False(t, m.hasReach, "nothing is announced to it: %x", m.reachNH)
		unreach = append(unreach, m.unreach...)
	}
	assert.Equal(t, llnhReflectedWithdrawal(t, payload), unreach, "the route is withdrawn")

	on := got[netip.MustParseAddr(llnhOnSegmentAddr)]
	require.Len(t, on, 1, "the client on the segment is owed the route")
	assert.Equal(t, mixedLLNH, on[0].reachNH)
	assert.False(t, on[0].hasUnrch, "the client on the segment is withdrawn nothing")
}
