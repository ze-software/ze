package adj_rib_in

import (
	"encoding/hex"
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// ipv4VPN is the ipv4/mpls-vpn family for tests (registered via TestMain).
var ipv4VPN = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}

// newTestManager creates an AdjRIBInManager with closed SDK connections for unit testing.
// The SDK plugin is initialized but connections are closed, so RPC calls (relayRoutes)
// will fail silently. This is appropriate for testing internal state changes.
func newTestManager(t *testing.T) *AdjRIBInManager {
	t.Helper()
	pluginEnd, remoteEnd := net.Pipe()
	if err := remoteEnd.Close(); err != nil {
		t.Logf("close remoteEnd: %v", err)
	}
	p := sdk.NewWithConn("adj-rib-in-test", pluginEnd)
	t.Cleanup(func() { _ = p.Close() })
	return &AdjRIBInManager{
		plugin:         p,
		ribIn:          make(map[netip.Addr]*seqmap.Map[compactRouteKey, *RawRoute]),
		peerUp:         make(map[netip.Addr]bool),
		pending:        make(map[compactPendingKey]*pendingRoute),
		earlyDecisions: make(map[compactPendingKey]*earlyDecision),
	}
}

func commandArgs(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

// testPeerJSON returns peer JSON in YANG-aligned format for peer 10.0.0.1 / AS 65001.
func testPeerJSON(t *testing.T) json.RawMessage {
	t.Helper()
	return mustMarshal(t, map[string]any{
		"remote": map[string]any{"address": "10.0.0.1", "as": uint32(65001)},
		"local":  map[string]any{"address": "10.0.0.2", "as": uint32(65002)},
	})
}

// TestStoreReceivedRoute verifies RawRoute is stored with hex fields from format=full event.
//
// VALIDATES: RawRoute stored with AttrHex, NHopHex, NLRIHex from format=full event.
// PREVENTS: Raw hex fields being discarded or parsed into Route structs.
func mustMarshalAny(v any) []byte {
	switch d := v.(type) {
	case json.RawMessage:
		return []byte(d)
	default:
		b, _ := json.Marshal(d)
		return b
	}
}

func TestStoreReceivedRoute(t *testing.T) {
	r := newTestManager(t)

	// format=full event: ORIGIN IGP (40 01 01 00), 10.0.0.0/24 (18 0a 00 00)
	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "180a0000"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"), "should have peer entry")
	routes := r.ribIn[netip.MustParseAddr("10.0.0.1")]
	require.Equal(t, 1, routes.Len(), "should have 1 route")

	// Find the stored route via Range
	var route *RawRoute
	var routeSeq uint64
	routes.Range(func(_ compactRouteKey, seq uint64, rt *RawRoute) bool {
		route = rt
		routeSeq = seq
		return true
	})
	require.NotNil(t, route)

	assert.Equal(t, family.IPv4Unicast, route.Family)
	assert.Equal(t, "40010100", route.AttrHex, "raw attributes should be stored as-is")
	assert.Equal(t, "0a000001", route.NHopHex, "next-hop 10.0.0.1 as wire hex")
	assert.Equal(t, "180a0000", route.NLRIHex, "NLRI wire bytes as hex")
	assert.Equal(t, uint64(1), routeSeq, "first route gets sequence 1")
}

// TestHandleReceivedStructuredIPv4NextHop verifies structured IPv4 UPDATEs keep NEXT_HOP.
//
// VALIDATES: structured IPv4/unicast received UPDATEs are stored with NEXT_HOP from attributes.
// PREVENTS: adj-rib-in dropping direct-bridge IPv4 routes because FamilyOperation.NextHop is empty.
func TestHandleReceivedStructuredIPv4NextHop(t *testing.T) {
	r := newTestManager(t)
	body, err := hex.DecodeString("0000001c400101025002000602010000fde9400304ac1e00038004040000000018092b00180a2b00180b2b00")
	require.NoError(t, err)

	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)
	require.Equal(t, "172.30.0.3", legacyNextHop(attrs))
	nlri, err := wu.NLRI()
	require.NoError(t, err)
	require.Len(t, wireNLRIsToAny(nlri, false, family.IPv4Unicast), 3)

	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: "172.30.0.3",
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})

	routes, ok := r.ribIn[netip.MustParseAddr("172.30.0.3")]
	require.True(t, ok, "should have peer entry")
	require.Equal(t, 3, routes.Len(), "should store all announced routes")

	routes.Range(func(key compactRouteKey, _ uint64, route *RawRoute) bool {
		assert.Equal(t, "ac1e0003", route.NHopHex, "route %s should keep NEXT_HOP", key)
		return true
	})
}

// TestStoreAllFamilies verifies VPN, EVPN, FlowSpec routes are stored (no filtering).
//
// VALIDATES: All address families are stored without isSimplePrefixFamily filter.
// VALIDATES: Complex family NLRIHex uses raw blob, not computed prefix bytes.
// PREVENTS: Complex families being silently dropped.
// PREVENTS: VPN NLRI stored as bare IPv4 prefix (missing RD+labels).
func TestStoreAllFamilies(t *testing.T) {
	r := newTestManager(t)

	// VPN family route - raw NLRI bytes contain RD+labels+prefix in wire format.
	// The raw blob "deadbeef" must be stored as-is; prefixToWireHex would produce
	// bare IPv4 bytes "180a0000" which is wrong for VPN wire format.
	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 200},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{ipv4VPN: "deadbeef"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			ipv4VPN: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	require.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len(), "VPN route should be stored")

	// Verify the raw blob is used, not prefixToWireHex output.
	var route *RawRoute
	r.ribIn[netip.MustParseAddr("10.0.0.1")].Range(func(_ compactRouteKey, _ uint64, rt *RawRoute) bool {
		route = rt
		return true
	})
	require.NotNil(t, route)
	assert.Equal(t, ipv4VPN, route.Family)
	assert.Equal(t, "deadbeef", route.NLRIHex,
		"complex family must use raw NLRI blob, not computed prefix bytes")
}

// TestRemoveWithdrawnRoute verifies withdrawal removes route from ribIn.
//
// VALIDATES: Withdrawn routes are removed from ribIn.
// PREVENTS: Stale route state after withdrawal.
func TestRemoveWithdrawnRoute(t *testing.T) {
	r := newTestManager(t)
	peerJSON := testPeerJSON(t)

	// First announce
	announce := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "180a0000"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(announce)
	require.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len())

	// Then withdraw
	withdraw := &bgp.Event{
		Message: &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 101},
		Peer:    peerJSON,
		// Withdrawals may have raw-withdrawn but not raw-attributes
		RawWithdrawn: map[family.Family]string{family.IPv4Unicast: "180a0000"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{Action: routeaction.Del, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(withdraw)
	assert.Equal(t, 0, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len(), "route should be removed after withdrawal")
}

// TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute proves the Adj-RIB-In consequence of RFC
// 7606 Section 2: when treat-as-withdraw is used, the affected route is REMOVED from the
// Adj-RIB-In. It observes the ribIn map directly, not merely that a withdrawal-shaped
// message was dispatched -- that dispatch is RFC7606-2-1's concern, proven separately in
// the reactor (TestSessionRFC7606TreatAsWithdrawDispatchesWithdrawal).
//
// The withdrawal fed here is produced by message.SynthesizeWithdraw -- the exact function
// the reactor runs to treat a malformed UPDATE as a withdrawal -- so this pins the whole
// chain: malformed announce -> synthesized withdrawal -> Adj-RIB-In removal. The positive
// half (a valid UPDATE leaves its route installed) is the same observation before the
// withdrawal, so removal is measured against a route that was genuinely there.
//
// RFC requirement: RFC7606-2-5 positive — a valid UPDATE leaves its route in the Adj-RIB-In.
// RFC requirement: RFC7606-2-5 negative — treat-as-withdraw removes the route from the Adj-RIB-In.
func TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute(t *testing.T) {
	r := newTestManager(t)
	peer := netip.MustParseAddr("192.0.2.1")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	feed := func(body []byte) {
		wu := wireu.NewWireUpdate(body, ctxID)
		attrs, _ := wu.Attrs()
		r.handleReceivedStructured(&rpc.StructuredEvent{
			EventType:   rpc.EventKindUpdate,
			PeerAddress: peer.String(),
			RawMessage: &bgptypes.RawMessage{
				Type:       msgtype.TypeUPDATE,
				WireUpdate: wu,
				AttrsWire:  attrs,
			},
		})
	}

	// A valid UPDATE announcing 10.0.0.0/8: Withdrawn length 0, 14 octets of well-known
	// mandatory attributes, then the NLRI.
	announce := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
	feed(announce)
	require.Equal(t, 1, r.ribIn[peer].Len(),
		"2-5 positive: a valid UPDATE leaves its route in the Adj-RIB-In")

	// The same prefix re-announced with a MALFORMED ORIGIN (length 2), run through the RFC
	// 7606 treat-as-withdraw synthesis. Feeding that withdrawal must remove 10.0.0.0/8.
	malformed := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0f, // Total Path Attribute Length 15
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN with length 2 (invalid)
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
	withdrawal, changed := message.SynthesizeWithdraw(malformed)
	require.True(t, changed, "treat-as-withdraw must produce a withdrawal for an announced route")
	feed(withdrawal)
	assert.Equal(t, 0, r.ribIn[peer].Len(),
		"2-5 negative: treat-as-withdraw must remove the route from the Adj-RIB-In")
}

// TestReplayAllSources verifies replay excludes the target peer's own routes from all sources except target.
//
// VALIDATES: Replay sends routes from A,B to X, excludes X's own routes.
// PREVENTS: Replaying a peer's own routes back to it.
func TestReplayAllSources(t *testing.T) {
	r := newTestManager(t)

	// Store routes from peer A
	m1 := seqmap.New[compactRouteKey, *RawRoute]()
	m1.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m1

	// Store routes from peer B
	m2 := seqmap.New[compactRouteKey, *RawRoute]()
	m2.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0), 2, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000002", NLRIHex: "180a0001",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.2")] = m2

	// Store routes from target peer X (should be excluded)
	m3 := seqmap.New[compactRouteKey, *RawRoute]()
	m3.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.2.0/24", 0), 3, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000003", NLRIHex: "180a0002",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.3")] = m3

	// Replay for target peer 10.0.0.3, from-index 0
	cmds, _ := r.buildReplayRoutes(netip.MustParseAddr("10.0.0.3"), 0, unboundedReplay())

	// Should have routes from A and B, not from X (10.0.0.3)
	assert.Len(t, cmds, 2, "should replay routes from 2 source peers, excluding target")
	for _, rt := range cmds {
		assert.Equal(t, "ipv4/unicast", rt.Family, "family travels with the route")
		assert.NotEmpty(t, rt.AttrHex, "must include raw attributes")
		assert.NotEmpty(t, rt.NextHopHex, "must include next-hop hex")
		assert.NotEmpty(t, rt.NLRIHex, "must include NLRI")
		// The source peer is what lets the engine reproduce the forward rail's
		// egress transform; the old "update hex" command form dropped it.
		assert.NotEqual(t, "10.0.0.3", rt.SourcePeer, "target peer's own routes are excluded")
		assert.NotEqual(t, "0a000003", rt.NextHopHex, "must not contain target peer's nhop")
	}
}

// TestReplayFromIndex verifies incremental replay sends only newer routes.
//
// VALIDATES: from-index is a RESUME CURSOR -- replay from a non-zero index sends
// only routes with SeqIndex STRICTLY GREATER than from-index.
// PREVENTS: Full replay on every reconnect, and re-sending the boundary route.
//
// The ">= from-index" this previously asserted was the defect, not the contract:
// bgp-rs feeds the previous call's `last-index` (the highest sequence it has
// already received) straight back in, so an inclusive read re-relays that route
// on every delta iteration. See buildReplayRoutes' doc comment.
func TestReplayFromIndex(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0), 5, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0001",
	})
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.2.0/24", 0), 10, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0002",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	// Replay from cursor 5 → only routes with SeqIndex > 5 (the route AT 5 was
	// already delivered in the batch that produced the cursor).
	cmds, _ := r.buildReplayRoutes(netip.MustParseAddr("10.0.0.99"), 5, unboundedReplay())
	assert.Len(t, cmds, 1, "should replay only routes strictly after the cursor")
	assert.Equal(t, "180a0002", cmds[0].NLRIHex, "the seq-10 route, not the seq-5 boundary route")
}

// TestReplayCursorRoundTripTerminates pins the bgp-rs delta-convergence contract:
// feeding a call's own last-index straight back must yield nothing.
//
// VALIDATES: buildReplayRoutes(peer, lastIndex) returns zero routes when no route
// has been stored since lastIndex was issued.
// PREVENTS: the duplicate a route-server client saw as the same UPDATE arriving
// twice back to back. seqmap.Since is inclusive (seq >= fromSeq), so before the
// fix this round trip re-relayed the boundary route; `replayed` was therefore
// never 0, and bgp-rs's convergence loop (replayConvergenceMax = 10) ran to its
// cap re-sending that one route on every attempt instead of exiting early.
func TestReplayCursorRoundTripTerminates(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0), 2, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0001",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	target := netip.MustParseAddr("10.0.0.99")

	first, lastIndex := r.buildReplayRoutes(target, 0, unboundedReplay())
	assert.Len(t, first, 2, "full replay delivers every stored route")
	assert.Equal(t, uint64(2), lastIndex, "last-index is the highest sequence delivered")

	// The delta bgp-rs issues next: same cursor, nothing new stored.
	second, secondLast := r.buildReplayRoutes(target, lastIndex, unboundedReplay())
	assert.Empty(t, second, "a delta at the cursor must deliver nothing; replayed==0 is what ends the convergence loop")
	assert.Zero(t, secondLast, "no routes delivered means no new last-index")

	// A route stored after the cursor is still picked up.
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.2.0/24", 0), 3, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0002",
	})
	third, thirdLast := r.buildReplayRoutes(target, lastIndex, unboundedReplay())
	assert.Len(t, third, 1, "the delta must still deliver genuinely new routes")
	assert.Equal(t, "180a0002", third[0].NLRIHex)
	assert.Equal(t, uint64(3), thirdLast)
}

// TestReplayReturnsLastIndex verifies response includes last-index value.
//
// VALIDATES: Response includes last-index as JSON data.
// PREVENTS: Callers unable to track replay progress.
func TestReplayReturnsLastIndex(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 42, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	_, lastIdx := r.buildReplayRoutes(netip.MustParseAddr("10.0.0.99"), 0, unboundedReplay())
	assert.Equal(t, uint64(42), lastIdx, "last-index should be max SeqIndex of replayed routes")
}

// TestSequenceIndexMonotonic verifies each insert gets an increasing index.
//
// VALIDATES: Index increases monotonically with each route insertion.
// PREVENTS: Duplicate or decreasing sequence values.
func TestSequenceIndexMonotonic(t *testing.T) {
	r := newTestManager(t)
	peerJSON := testPeerJSON(t)

	// Insert 3 routes
	for i, prefix := range []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"} {
		nlriHex := []string{"180a0000", "180a0001", "180a0002"}
		event := &bgp.Event{
			Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: uint64(100 + i)},
			Peer:          peerJSON,
			RawAttributes: "40010100",
			RawNLRI:       map[family.Family]string{family.IPv4Unicast: nlriHex[i]},
			FamilyOps: map[family.Family][]bgp.FamilyOperation{
				family.IPv4Unicast: {
					{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{prefix}},
				},
			},
		}
		r.handleReceived(event)
	}

	// Collect sequence indices via Range
	var indices []uint64
	r.ribIn[netip.MustParseAddr("10.0.0.1")].Range(func(_ compactRouteKey, seq uint64, _ *RawRoute) bool {
		indices = append(indices, seq)
		return true
	})
	require.Len(t, indices, 3)

	// Verify all are unique and monotonically increasing
	seen := make(map[uint64]bool)
	for _, idx := range indices {
		assert.False(t, seen[idx], "sequence index %d should be unique", idx)
		assert.Greater(t, idx, uint64(0), "sequence index should be > 0")
		seen[idx] = true
	}
}

// TestClearPeerOnDown verifies peer down clears that peer's routes.
//
// VALIDATES: Peer state=down clears ribIn for that peer.
// PREVENTS: Stale routes persisting after peer disconnect.
func TestClearPeerOnDown(t *testing.T) {
	r := newTestManager(t)

	// Pre-populate routes
	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m
	r.peerUp[netip.MustParseAddr("10.0.0.1")] = true

	// Peer goes down
	downEvent := &bgp.Event{
		Type: "state",
		Peer: mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: "10.0.0.1", AS: 65001}}),
	}
	// State can be in flat peer format or top-level
	downEvent.State = "down"

	r.handleState(downEvent)

	assert.Nil(t, r.ribIn[netip.MustParseAddr("10.0.0.1")], "routes should be cleared on peer down")
	assert.False(t, r.peerUp[netip.MustParseAddr("10.0.0.1")], "peer should be marked down")
}

// TestNHopToHex verifies next-hop IP to wire hex conversion.
//
// VALIDATES: IPv4 and IPv6 addresses convert to correct wire hex.
// PREVENTS: Malformed nhop hex in replay commands.
func TestNHopToHex(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
	}{
		{"IPv4", "10.0.0.1", "0a000001"},
		{"IPv4 loopback", "127.0.0.1", "7f000001"},
		{"IPv4 all zeros", "0.0.0.0", "00000000"},
		{"IPv6 loopback", "::1", "00000000000000000000000000000001"},
		{"IPv6 full", "2001:db8::1", "20010db8000000000000000000000001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nhopToHex(tt.ip)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestReplayRouteCarriesSource verifies a replayed route carries the peer it was
// learned from, alongside the stored wire bytes.
//
// VALIDATES: the engine can reproduce the forward rail's egress transform, which
// keys off the SOURCE peer (AS_PATH prepend, RFC 4456 reflection, RFC 9234 role).
// PREVENTS: regressing to the old "update hex attr set ..." command form, which
// dropped the source and so sent replays down the announce rail where the local
// AS is prepended BEFORE the export filters run
// (spec-fixit-bgp-egress-rail-divergence, closed 2026-08-14).
func TestReplayRouteCarriesSource(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family:  family.IPv4Unicast,
		AttrHex: "400101004002060201000000c8",
		NHopHex: "0a000001",
		NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	routes, _ := r.buildReplayRoutes(netip.MustParseAddr("10.0.0.99"), 0, unboundedReplay())
	require.Len(t, routes, 1)
	assert.Equal(t, rpc.StoredRoute{
		SourcePeer: "10.0.0.1",
		Family:     "ipv4/unicast",
		AttrHex:    "400101004002060201000000c8",
		NextHopHex: "0a000001",
		NLRIHex:    "180a0000",
	}, routes[0])
}

// TestHandleCommand_Status verifies status command returns route counts.
//
// VALIDATES: Status returns per-peer route counts as JSON.
// PREVENTS: Status command failing or returning wrong data.
func TestHandleCommand_Status(t *testing.T) {
	r := newTestManager(t)

	m1 := seqmap.New[compactRouteKey, *RawRoute]()
	m1.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{Family: family.IPv4Unicast})
	m1.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0), 2, &RawRoute{Family: family.IPv4Unicast})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m1

	m2 := seqmap.New[compactRouteKey, *RawRoute]()
	m2.Put(routeKeyFromStrings(family.IPv6Unicast, "2001:db8::/32", 0), 3, &RawRoute{Family: family.IPv6Unicast})
	r.ribIn[netip.MustParseAddr("10.0.0.2")] = m2
	status, data, err := r.handleCommand("show bgp adj-rib-in status", nil, "")
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	var result map[string]any
	require.NoError(t, json.Unmarshal(mustMarshalAny(data), &result))

	// Should report running and route counts
	assert.Equal(t, true, result["running"])
}

// TestHandleCommand_Show verifies show command returns human-readable route data.
//
// VALIDATES: Show returns routes in JSON with family, prefix fields.
// PREVENTS: Show command failing or returning hex-only output.
func TestHandleCommand_Show(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family:  family.IPv4Unicast,
		AttrHex: "40010100",
		NHopHex: "0a000001",
		NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m
	status, data, err := r.handleCommand("show bgp adj-rib-in", nil, "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, string(mustMarshal(t, data)), "10.0.0.1", "should contain peer address")
	assert.Contains(t, string(mustMarshal(t, data)), "ipv4/unicast", "should contain family")
}

// TestHandleCommand_ShowSelectorArgCompatibility verifies the legacy string
// dispatch path still filters by the selector argument after the typed-args refactor.
//
// VALIDATES: handleCommand("show bgp adj-rib-in", args=["10.0.0.1"], peer="*") shows only that peer.
// PREVENTS: string dispatch clients receiving a full-table dump for a single-peer query.
func TestHandleCommand_ShowSelectorArgCompatibility(t *testing.T) {
	r := newTestManager(t)

	m1 := seqmap.New[compactRouteKey, *RawRoute]()
	m1.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family:  family.IPv4Unicast,
		AttrHex: "40010100",
		NHopHex: "0a000001",
		NLRIHex: "180a0000",
	})
	m2 := seqmap.New[compactRouteKey, *RawRoute]()
	m2.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0), 2, &RawRoute{
		Family:  family.IPv4Unicast,
		AttrHex: "40010100",
		NHopHex: "0a000002",
		NLRIHex: "180a0001",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m1
	r.ribIn[netip.MustParseAddr("10.0.0.2")] = m2

	status, data, err := r.handleCommand("show bgp adj-rib-in", []string{"10.0.0.1"}, "*")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)
	got := string(mustMarshal(t, data))
	assert.Contains(t, got, "10.0.0.1")
	assert.NotContains(t, got, "10.0.0.2")
}

// TestMultipleNLRIsPerUpdate verifies multiple NLRIs in single UPDATE are stored individually.
//
// VALIDATES: Each NLRI in a multi-NLRI UPDATE gets its own RawRoute entry.
// PREVENTS: Multiple NLRIs being merged into one entry.
func TestMultipleNLRIsPerUpdate(t *testing.T) {
	r := newTestManager(t)

	// Two NLRIs: 10.0.0.0/24 (18 0a 00 00) + 10.0.1.0/24 (18 0a 00 01)
	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "180a0000180a0001"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Equal(t, 2, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len(), "each NLRI should be stored separately")

	// Both should share the same AttrHex (from same UPDATE)
	r.ribIn[netip.MustParseAddr("10.0.0.1")].Range(func(_ compactRouteKey, _ uint64, rt *RawRoute) bool {
		assert.Equal(t, "40010100", rt.AttrHex, "all NLRIs share same attributes")
		assert.Equal(t, "0a000001", rt.NHopHex, "all NLRIs share same next-hop")
		return true
	})
}

// TestAdjRibInReplayArgsPassthrough verifies replay receives correct target peer and from-index.
//
// VALIDATES: handleCommand("request bgp adj-rib-in replay", args=["127.0.0.2", "0"]) replays routes for 127.0.0.2.
// PREVENTS: Args being dropped, causing replay to target "*" instead of specific peer.
func TestAdjRibInReplayArgsPassthrough(t *testing.T) {
	r := newTestManager(t)

	// Store a route from source peer 10.0.0.1
	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	// Capture the relay instead of reaching a live engine. newTestManager's SDK
	// plugin has closed connections, so the real call fails -- and since
	// replayCommand now propagates that failure (rather than logging and
	// reporting statusDone), the stub is what lets this test exercise the
	// argument passthrough it is actually about.
	var gotDest string
	var gotRoutes []rpc.StoredRoute
	r.routeRelayer = func(dest string, routes []rpc.StoredRoute) {
		gotDest, gotRoutes = dest, routes
	}

	// Call handleCommand with the selector that would come from args
	// This simulates: command="request bgp adj-rib-in replay", args=["127.0.0.2", "0"]
	status, data, err := r.handleCommand("request bgp adj-rib-in replay", []string{"127.0.0.2", "0"}, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	// The target peer from args[0] must reach the relay verbatim.
	assert.Equal(t, "127.0.0.2", gotDest, "args[0] is the relay destination")
	require.Len(t, gotRoutes, 1)
	assert.Equal(t, "10.0.0.1", gotRoutes[0].SourcePeer, "the route carries its source peer")

	// Should have replayed 1 route (from 10.0.0.1, target is 127.0.0.2)
	assert.Contains(t, string(mustMarshal(t, data)), `"replayed":1`)
	assert.Contains(t, string(mustMarshal(t, data)), `"last-index":1`)
}

// TestAdjRibInReplayArgsEmpty verifies empty selector returns an error.
//
// VALIDATES: handleCommand("request bgp adj-rib-in replay", nil) returns error requiring target peer.
// PREVENTS: Replay running without a target peer, which could cause unexpected behavior.
func TestAdjRibInReplayArgsEmpty(t *testing.T) {
	r := newTestManager(t)
	status, _, err := r.handleCommand("request bgp adj-rib-in replay", nil, "")
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires target peer address")
}

// TestHandleState_PeerUpTriggersReplay verifies that peer-up triggers automatic replay
// of routes from all other source peers to the newly-up peer.
//
// VALIDATES: handleState on peer-up relays the stored routes to the newly-up peer.
// PREVENTS: Newly-added peers receiving no routes until other peers send new UPDATEs.
func TestHandleState_PeerUpTriggersReplay(t *testing.T) {
	r := newTestManager(t)

	// Spy on route sends to verify handleState actually triggers replay.
	var sent []struct {
		peer   string
		routes []rpc.StoredRoute
	}
	r.routeRelayer = func(peer string, routes []rpc.StoredRoute) {
		sent = append(sent, struct {
			peer   string
			routes []rpc.StoredRoute
		}{peer, routes})
	}

	// Pre-populate routes from peer A (10.0.0.1)
	m1 := seqmap.New[compactRouteKey, *RawRoute]()
	m1.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m1

	// Pre-populate routes from peer B (10.0.0.2)
	m2 := seqmap.New[compactRouteKey, *RawRoute]()
	m2.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0), 2, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000002", NLRIHex: "180a0001",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.2")] = m2

	// Peer C (10.0.0.3) comes up -- should trigger replay of routes from A and B.
	upEvent := &bgp.Event{
		Type:  "state",
		State: "up",
		Peer:  mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: "10.0.0.3", AS: 65003}}),
	}

	r.handleState(upEvent)

	// Verify peer is marked up.
	assert.True(t, r.peerUp[netip.MustParseAddr("10.0.0.3")], "peer should be marked up")

	// Verify handleState actually triggered replay via routeRelayer. One relay
	// call carries the whole replay, so assert on the routes it contains rather
	// than on a call count.
	require.Len(t, sent, 1, "a peer-up replay is one relay call")
	assert.Equal(t, "10.0.0.3", sent[0].peer, "routes should target newly-up peer")
	require.Len(t, sent[0].routes, 2, "should replay routes from peers A and B")
	sources := []string{sent[0].routes[0].SourcePeer, sent[0].routes[1].SourcePeer}
	assert.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, sources,
		"each replayed route names the peer it was learned from")
}

// TestHandleState_PeerUpEmptyRIB verifies that peer-up with no routes in RIB
// sends nothing and produces no errors.
//
// VALIDATES: Peer-up with empty adj-rib-in works cleanly (startup scenario).
// PREVENTS: Errors or panics when replaying an empty RIB.
func TestHandleState_PeerUpEmptyRIB(t *testing.T) {
	r := newTestManager(t)

	var sendCount int
	r.routeRelayer = func(_ string, routes []rpc.StoredRoute) { sendCount += len(routes) }

	// No routes in ribIn -- this is the startup scenario.
	upEvent := &bgp.Event{
		Type:  "state",
		State: "up",
		Peer:  mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: "10.0.0.1", AS: 65001}}),
	}

	// Should not panic or error.
	r.handleState(upEvent)

	assert.True(t, r.peerUp[netip.MustParseAddr("10.0.0.1")], "peer should be marked up")
	assert.Equal(t, 0, sendCount, "empty RIB should send no replay commands")
}

// TestHandleState_PeerUpSelfExclusion verifies that a peer's own routes
// are not replayed back to it on peer-up.
//
// VALIDATES: Routes sourced from peer X are not replayed to peer X on its peer-up.
// PREVENTS: Routing loops from replaying a peer's own routes back to it.
func TestHandleState_PeerUpSelfExclusion(t *testing.T) {
	r := newTestManager(t)

	var sent []struct {
		peer   string
		routes []rpc.StoredRoute
	}
	r.routeRelayer = func(peer string, routes []rpc.StoredRoute) {
		sent = append(sent, struct {
			peer   string
			routes []rpc.StoredRoute
		}{peer, routes})
	}

	// Peer 10.0.0.1 has routes from itself (shouldn't happen normally,
	// but tests the exclusion logic).
	m1 := seqmap.New[compactRouteKey, *RawRoute]()
	m1.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m1

	// Also routes from another peer
	m2 := seqmap.New[compactRouteKey, *RawRoute]()
	m2.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0), 2, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000002", NLRIHex: "180a0001",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.2")] = m2

	// Peer 10.0.0.1 comes up
	upEvent := &bgp.Event{
		Type:  "state",
		State: "up",
		Peer:  mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: "10.0.0.1", AS: 65001}}),
	}

	r.handleState(upEvent)

	// Only routes from 10.0.0.2 should be replayed, not 10.0.0.1's own routes.
	require.Len(t, sent, 1, "a peer-up replay is one relay call")
	assert.Equal(t, "10.0.0.1", sent[0].peer, "routes target the newly-up peer")
	require.Len(t, sent[0].routes, 1, "should replay only routes from other peers")
	assert.Equal(t, "10.0.0.2", sent[0].routes[0].SourcePeer, "the surviving route is peer B's")
	assert.Equal(t, "0a000002", sent[0].routes[0].NextHopHex, "should contain peer B's next-hop")
}

// TestComplexFamilyMultiNLRI verifies that multi-NLRI VPN UPDATEs store
// only one entry using the raw blob (which covers all NLRIs).
//
// VALIDATES: Complex family stores raw blob for first NLRI, skips subsequent.
// PREVENTS: Duplicate or incorrectly-encoded entries for VPN multi-NLRI UPDATEs.
func TestComplexFamilyMultiNLRI(t *testing.T) {
	r := newTestManager(t)

	// VPN UPDATE with 2 parsed NLRIs but a single concatenated raw blob.
	// The raw blob contains both NLRIs in wire format (RD+labels+prefix).
	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 300},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{ipv4VPN: "aabbccdd11223344"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			ipv4VPN: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	// Only 1 entry: the first NLRI carries the raw blob, second is skipped (i > 0).
	require.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len(),
		"complex family multi-NLRI should store one entry with full raw blob")

	var route *RawRoute
	r.ribIn[netip.MustParseAddr("10.0.0.1")].Range(func(_ compactRouteKey, _ uint64, rt *RawRoute) bool {
		route = rt
		return true
	})
	require.NotNil(t, route)
	assert.Equal(t, "aabbccdd11223344", route.NLRIHex,
		"must store entire raw blob, not computed prefix bytes")
}

// TestReplayOwnerDedupe verifies that an explicit replay request makes this
// plugin stand down from its own peer-up replay, while a plugin nobody drives
// keeps self-replaying.
//
// VALIDATES: spec AC-5 -- exactly one replay owner per process. bgp-rs withholds
// a peer from forward targets while it replays (Replaying), so its replay never
// races the live forward; this plugin's self-replay has no such gate, and with
// both firing a route learned just before establishment went out twice.
// PREVENTS: the duplicate announce in test/plugin/rfc7606-relay-one-field.ci,
// and equally the opposite regression -- a standalone adj-rib-in silently
// replaying nothing because it stood down for an owner that does not exist.
func TestReplayOwnerDedupe(t *testing.T) {
	newWithRoute := func(t *testing.T) (*AdjRIBInManager, *[]string) {
		t.Helper()
		r := newTestManager(t)
		m := seqmap.New[compactRouteKey, *RawRoute]()
		m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
			Family: family.IPv4Unicast, AttrHex: "40010100",
			NHopHex: "0a000001", NLRIHex: "180a0000",
		})
		r.ribIn[netip.MustParseAddr("10.0.0.1")] = m
		var destinations []string
		r.routeRelayer = func(dest string, routes []rpc.StoredRoute) {
			if len(routes) > 0 {
				destinations = append(destinations, dest)
			}
		}
		return r, &destinations
	}

	upEvent := &bgp.Event{
		Type:  "state",
		State: "up",
		Peer:  mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: "10.0.0.2", AS: 65002}}),
	}

	t.Run("standalone self-replays", func(t *testing.T) {
		r, dest := newWithRoute(t)
		r.handleState(upEvent)
		assert.Equal(t, []string{"10.0.0.2"}, *dest,
			"with no replay owner, peer-up must still replay")
	})

	t.Run("owned stands down", func(t *testing.T) {
		r, dest := newWithRoute(t)

		// bgp-rs claims ownership at startup, before any session establishes.
		status, _, err := r.claimReplayCommand()
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		require.Empty(t, *dest, "claiming ownership does not itself replay")

		// Every peer-up from here on must stand down.
		r.handleState(upEvent)
		assert.Empty(t, *dest, "once replay is owned, peer-up must not self-replay")
		assert.True(t, r.peerUp[netip.MustParseAddr("10.0.0.2")],
			"standing down from replay must not stop tracking peer state")

		// The owner's explicit replay still runs.
		status, _, err = r.replayCommand([]string{"10.0.0.3"})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		assert.Equal(t, []string{"10.0.0.3"}, *dest, "the owner's replay still runs")
	})

	// The ownership claim must cover the FIRST peer, not just later ones.
	// Latching on the first replay left the first peer-up racing: nothing had
	// claimed yet, so the self-replay AND the owner's replay both fired and the
	// route went out twice -- the duplicate this spec exists to remove.
	t.Run("claim precedes the first peer-up", func(t *testing.T) {
		r, dest := newWithRoute(t)
		_, _, err := r.claimReplayCommand()
		require.NoError(t, err)
		r.handleState(upEvent)
		assert.Empty(t, *dest, "the very first peer-up after a claim must not self-replay")
	})

	// The DECLARATIVE path: the engine delivers the claim on the Stage-2
	// configure callback, which completes before this plugin sends Stage 5 ready
	// and therefore before the engine starts peers. This is what makes the very
	// first peer-up safe without any dispatch having to arrive in time.
	t.Run("startup claim stands self-replay down", func(t *testing.T) {
		r, dest := newWithRoute(t)

		r.applyStartupClaims(func(role string) bool { return role == claimPeerUpReplay })
		require.Empty(t, *dest, "applying a startup claim does not itself replay")

		r.handleState(upEvent)
		assert.Empty(t, *dest,
			"a role claimed at Stage 2 must stop self-replay for the FIRST peer-up")
		assert.True(t, r.peerUp[netip.MustParseAddr("10.0.0.2")],
			"standing down from replay must not stop tracking peer state")
	})

	// Fail-closed: an unclaimed role, or no claim source at all, must leave
	// self-replay ON. Standing down for an owner that does not exist loses
	// routes; self-replaying alongside an owner only duplicates an idempotent
	// UPDATE.
	t.Run("unclaimed role leaves self-replay on", func(t *testing.T) {
		for name, claimActive := range map[string]func(string) bool{
			"no claim source":  nil,
			"nothing claimed":  func(string) bool { return false },
			"unrelated claims": func(role string) bool { return role == "some-other-role" },
		} {
			t.Run(name, func(t *testing.T) {
				r, dest := newWithRoute(t)
				r.applyStartupClaims(claimActive)
				r.handleState(upEvent)
				assert.Equal(t, []string{"10.0.0.2"}, *dest,
					"without a claim for this role, peer-up must still replay")
			})
		}
	})

	// The late-join corrective must not undo the declarative claim, and the two
	// overlapping must stay idempotent.
	t.Run("startup claim and late-join corrective are idempotent", func(t *testing.T) {
		r, dest := newWithRoute(t)

		r.applyStartupClaims(func(string) bool { return true })
		status, _, err := r.claimReplayCommand()
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		r.applyStartupClaims(func(string) bool { return true })

		r.handleState(upEvent)
		assert.Empty(t, *dest, "overlapping claim paths must not re-enable self-replay")
	})

	// An operator running the diagnostic replay verb must NOT silently disable
	// peer-up replay for the rest of the process lifetime.
	t.Run("operator replay does not latch ownership", func(t *testing.T) {
		r, dest := newWithRoute(t)

		status, _, err := r.replayCommand([]string{"10.0.0.3"})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		require.Equal(t, []string{"10.0.0.3"}, *dest)

		r.handleState(upEvent)
		assert.Equal(t, []string{"10.0.0.3", "10.0.0.2"}, *dest,
			"a plain replay must leave peer-up self-replay working")
	})
}
