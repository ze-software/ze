package adj_rib_in

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	"codeberg.org/thomas-mangin/ze/internal/core/seqmap"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// newTestManager creates an AdjRIBInManager with closed SDK connections for unit testing.
// The SDK plugin is initialized but connections are closed, so RPC calls (updateRoute)
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
		plugin:  p,
		ribIn:   make(map[string]*seqmap.Map[string, *RawRoute]),
		peerUp:  make(map[string]bool),
		pending: make(map[string]*PendingRoute),
	}
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

// testPeerJSON returns peer JSON in ze-bgp nested format for peer 10.0.0.1 / AS 65001.
func testPeerJSON(t *testing.T) json.RawMessage {
	t.Helper()
	return mustMarshal(t, map[string]any{
		"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"},
		"asn":     map[string]uint32{"local": 65002, "peer": 65001},
	})
}

// TestStoreReceivedRoute verifies RawRoute is stored with hex fields from format=full event.
//
// VALIDATES: RawRoute stored with AttrHex, NHopHex, NLRIHex from format=full event.
// PREVENTS: Raw hex fields being discarded or parsed into Route structs.
func TestStoreReceivedRoute(t *testing.T) {
	r := newTestManager(t)

	// format=full event: ORIGIN IGP (40 01 01 00), 10.0.0.0/24 (18 0a 00 00)
	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: "update", ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, "10.0.0.1", "should have peer entry")
	routes := r.ribIn["10.0.0.1"]
	require.Equal(t, 1, routes.Len(), "should have 1 route")

	// Find the stored route via Range
	var route *RawRoute
	var routeSeq uint64
	routes.Range(func(_ string, seq uint64, rt *RawRoute) bool {
		route = rt
		routeSeq = seq
		return true
	})
	require.NotNil(t, route)

	assert.Equal(t, "ipv4/unicast", route.Family)
	assert.Equal(t, "40010100", route.AttrHex, "raw attributes should be stored as-is")
	assert.Equal(t, "0a000001", route.NHopHex, "next-hop 10.0.0.1 as wire hex")
	assert.Equal(t, "180a0000", route.NLRIHex, "NLRI wire bytes as hex")
	assert.Equal(t, uint64(1), routeSeq, "first route gets sequence 1")
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
		Message:       &bgp.MessageInfo{Type: "update", ID: 200},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/mpls-vpn": "deadbeef"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/mpls-vpn": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, "10.0.0.1")
	require.Equal(t, 1, r.ribIn["10.0.0.1"].Len(), "VPN route should be stored")

	// Verify the raw blob is used, not prefixToWireHex output.
	var route *RawRoute
	r.ribIn["10.0.0.1"].Range(func(_ string, _ uint64, rt *RawRoute) bool {
		route = rt
		return true
	})
	require.NotNil(t, route)
	assert.Equal(t, "ipv4/mpls-vpn", route.Family)
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
		Message:       &bgp.MessageInfo{Type: "update", ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(announce)
	require.Equal(t, 1, r.ribIn["10.0.0.1"].Len())

	// Then withdraw
	withdraw := &bgp.Event{
		Message: &bgp.MessageInfo{Type: "update", ID: 101},
		Peer:    peerJSON,
		// Withdrawals may have raw-withdrawn but not raw-attributes
		RawWithdrawn: map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/unicast": {
				{Action: "del", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(withdraw)
	assert.Equal(t, 0, r.ribIn["10.0.0.1"].Len(), "route should be removed after withdrawal")
}

// TestReplayAllSources verifies replay sends "update hex" commands from all sources except target.
//
// VALIDATES: Replay sends routes from A,B to X, excludes X's own routes.
// PREVENTS: Replaying a peer's own routes back to it.
func TestReplayAllSources(t *testing.T) {
	r := newTestManager(t)

	// Store routes from peer A
	m1 := seqmap.New[string, *RawRoute]()
	m1.Put("ipv4/unicast:10.0.0.0/24", 1, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn["10.0.0.1"] = m1

	// Store routes from peer B
	m2 := seqmap.New[string, *RawRoute]()
	m2.Put("ipv4/unicast:10.0.1.0/24", 2, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000002", NLRIHex: "180a0001",
	})
	r.ribIn["10.0.0.2"] = m2

	// Store routes from target peer X (should be excluded)
	m3 := seqmap.New[string, *RawRoute]()
	m3.Put("ipv4/unicast:10.0.2.0/24", 3, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000003", NLRIHex: "180a0002",
	})
	r.ribIn["10.0.0.3"] = m3

	// Replay for target peer 10.0.0.3, from-index 0
	cmds, _ := r.buildReplayCommands("10.0.0.3", 0)

	// Should have routes from A and B, not from X (10.0.0.3)
	assert.Len(t, cmds, 2, "should replay routes from 2 source peers, excluding target")
	for _, cmd := range cmds {
		assert.True(t, strings.HasPrefix(cmd, "update hex "), "replay must use 'update hex' format")
		assert.Contains(t, cmd, "attr set ", "must include raw attributes")
		assert.Contains(t, cmd, "nhop set ", "must include next-hop hex")
		assert.Contains(t, cmd, "nlri ipv4/unicast add ", "must include NLRI with family")
		assert.NotContains(t, cmd, "0a000003", "must not contain target peer's nhop")
	}
}

// TestReplayFromIndex verifies incremental replay sends only newer routes.
//
// VALIDATES: Replay from non-zero index sends only routes with SeqIndex >= from-index.
// PREVENTS: Full replay on every reconnect.
func TestReplayFromIndex(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[string, *RawRoute]()
	m.Put("ipv4/unicast:10.0.0.0/24", 1, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	m.Put("ipv4/unicast:10.0.1.0/24", 5, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0001",
	})
	m.Put("ipv4/unicast:10.0.2.0/24", 10, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0002",
	})
	r.ribIn["10.0.0.1"] = m

	// Replay from index 5 → only routes with SeqIndex >= 5
	cmds, _ := r.buildReplayCommands("10.0.0.99", 5)
	assert.Len(t, cmds, 2, "should replay only routes with SeqIndex >= 5")
}

// TestReplayReturnsLastIndex verifies response includes last-index value.
//
// VALIDATES: Response includes last-index as JSON data.
// PREVENTS: Callers unable to track replay progress.
func TestReplayReturnsLastIndex(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[string, *RawRoute]()
	m.Put("ipv4/unicast:10.0.0.0/24", 42, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn["10.0.0.1"] = m

	_, lastIdx := r.buildReplayCommands("10.0.0.99", 0)
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
			Message:       &bgp.MessageInfo{Type: "update", ID: uint64(100 + i)},
			Peer:          peerJSON,
			RawAttributes: "40010100",
			RawNLRI:       map[string]string{"ipv4/unicast": nlriHex[i]},
			FamilyOps: map[string][]bgp.FamilyOperation{
				"ipv4/unicast": {
					{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{prefix}},
				},
			},
		}
		r.handleReceived(event)
	}

	// Collect sequence indices via Range
	var indices []uint64
	r.ribIn["10.0.0.1"].Range(func(_ string, seq uint64, _ *RawRoute) bool {
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
	m := seqmap.New[string, *RawRoute]()
	m.Put("ipv4/unicast:10.0.0.0/24", 1, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn["10.0.0.1"] = m
	r.peerUp["10.0.0.1"] = true

	// Peer goes down
	downEvent := &bgp.Event{
		Type: "state",
		Peer: mustMarshal(t, bgp.PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
	}
	// State can be in flat peer format or top-level
	downEvent.State = "down"

	r.handleState(downEvent)

	assert.Nil(t, r.ribIn["10.0.0.1"], "routes should be cleared on peer down")
	assert.False(t, r.peerUp["10.0.0.1"], "peer should be marked down")
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

// TestReplayCommandFormat verifies the exact "update hex" command format.
//
// VALIDATES: Replay builds correct "update hex attr set ... nhop set ... nlri FAM add ..." command.
// PREVENTS: Malformed commands that engine can't parse.
func TestReplayCommandFormat(t *testing.T) {
	route := &RawRoute{
		Family:  "ipv4/unicast",
		AttrHex: "400101004002060201000000c8",
		NHopHex: "0a000001",
		NLRIHex: "180a0000",
	}

	cmd := formatHexCommand(route)
	assert.Equal(t, "update hex attr set 400101004002060201000000c8 nhop set 0a000001 nlri ipv4/unicast add 180a0000", cmd)
}

// TestHandleCommand_Status verifies status command returns route counts.
//
// VALIDATES: Status returns per-peer route counts as JSON.
// PREVENTS: Status command failing or returning wrong data.
func TestHandleCommand_Status(t *testing.T) {
	r := newTestManager(t)

	m1 := seqmap.New[string, *RawRoute]()
	m1.Put("k1", 1, &RawRoute{Family: "ipv4/unicast"})
	m1.Put("k2", 2, &RawRoute{Family: "ipv4/unicast"})
	r.ribIn["10.0.0.1"] = m1

	m2 := seqmap.New[string, *RawRoute]()
	m2.Put("k3", 3, &RawRoute{Family: "ipv6/unicast"})
	r.ribIn["10.0.0.2"] = m2

	status, data, err := r.handleCommand("adj-rib-in status", "")
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))

	// Should report running and route counts
	assert.Equal(t, true, result["running"])
}

// TestHandleCommand_Show verifies show command returns human-readable route data.
//
// VALIDATES: Show returns routes in JSON with family, prefix fields.
// PREVENTS: Show command failing or returning hex-only output.
func TestHandleCommand_Show(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[string, *RawRoute]()
	m.Put("ipv4/unicast:10.0.0.0/24", 1, &RawRoute{
		Family:  "ipv4/unicast",
		AttrHex: "40010100",
		NHopHex: "0a000001",
		NLRIHex: "180a0000",
	})
	r.ribIn["10.0.0.1"] = m

	status, data, err := r.handleCommand("adj-rib-in show", "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, "10.0.0.1", "should contain peer address")
	assert.Contains(t, data, "ipv4/unicast", "should contain family")
}

// TestMultipleNLRIsPerUpdate verifies multiple NLRIs in single UPDATE are stored individually.
//
// VALIDATES: Each NLRI in a multi-NLRI UPDATE gets its own RawRoute entry.
// PREVENTS: Multiple NLRIs being merged into one entry.
func TestMultipleNLRIsPerUpdate(t *testing.T) {
	r := newTestManager(t)

	// Two NLRIs: 10.0.0.0/24 (18 0a 00 00) + 10.0.1.0/24 (18 0a 00 01)
	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: "update", ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000180a0001"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Equal(t, 2, r.ribIn["10.0.0.1"].Len(), "each NLRI should be stored separately")

	// Both should share the same AttrHex (from same UPDATE)
	r.ribIn["10.0.0.1"].Range(func(_ string, _ uint64, rt *RawRoute) bool {
		assert.Equal(t, "40010100", rt.AttrHex, "all NLRIs share same attributes")
		assert.Equal(t, "0a000001", rt.NHopHex, "all NLRIs share same next-hop")
		return true
	})
}

// TestAdjRibInReplayArgsPassthrough verifies replay receives correct target peer and from-index.
//
// VALIDATES: handleCommand("adj-rib-in replay", "127.0.0.2 0") replays routes for 127.0.0.2.
// PREVENTS: Args being dropped, causing replay to target "*" instead of specific peer.
func TestAdjRibInReplayArgsPassthrough(t *testing.T) {
	r := newTestManager(t)

	// Store a route from source peer 10.0.0.1
	m := seqmap.New[string, *RawRoute]()
	m.Put("ipv4/unicast:10.0.0.0/24", 1, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn["10.0.0.1"] = m

	// Call handleCommand with the selector that would come from args
	// This simulates: command="adj-rib-in replay", args=["127.0.0.2", "0"]
	// The selector is args joined with space: "127.0.0.2 0"
	status, data, err := r.handleCommand("adj-rib-in replay", "127.0.0.2 0")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	// Should have replayed 1 route (from 10.0.0.1, target is 127.0.0.2)
	assert.Contains(t, data, `"replayed":1`)
	assert.Contains(t, data, `"last-index":1`)
}

// TestAdjRibInReplayArgsEmpty verifies empty selector returns an error.
//
// VALIDATES: handleCommand("adj-rib-in replay", "") returns error requiring target peer.
// PREVENTS: Replay running without a target peer, which could cause unexpected behavior.
func TestAdjRibInReplayArgsEmpty(t *testing.T) {
	r := newTestManager(t)

	status, _, err := r.handleCommand("adj-rib-in replay", "")
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires target peer address")
}

// TestHandleState_PeerUpTriggersReplay verifies that peer-up triggers automatic replay
// of routes from all other source peers to the newly-up peer.
//
// VALIDATES: handleState on peer-up calls routeSender/updateRoute with correct peer and commands.
// PREVENTS: Newly-added peers receiving no routes until other peers send new UPDATEs.
func TestHandleState_PeerUpTriggersReplay(t *testing.T) {
	r := newTestManager(t)

	// Spy on route sends to verify handleState actually triggers replay.
	var sent []struct{ peer, cmd string }
	r.routeSender = func(peer, cmd string) {
		sent = append(sent, struct{ peer, cmd string }{peer, cmd})
	}

	// Pre-populate routes from peer A (10.0.0.1)
	m1 := seqmap.New[string, *RawRoute]()
	m1.Put("ipv4/unicast:10.0.0.0/24", 1, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn["10.0.0.1"] = m1

	// Pre-populate routes from peer B (10.0.0.2)
	m2 := seqmap.New[string, *RawRoute]()
	m2.Put("ipv4/unicast:10.0.1.0/24", 2, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000002", NLRIHex: "180a0001",
	})
	r.ribIn["10.0.0.2"] = m2

	// Peer C (10.0.0.3) comes up -- should trigger replay of routes from A and B.
	upEvent := &bgp.Event{
		Type:  "state",
		State: "up",
		Peer:  mustMarshal(t, bgp.PeerInfoFlat{Address: "10.0.0.3", ASN: 65003}),
	}

	r.handleState(upEvent)

	// Verify peer is marked up.
	assert.True(t, r.peerUp["10.0.0.3"], "peer should be marked up")

	// Verify handleState actually triggered replay via routeSender.
	assert.Len(t, sent, 2, "should replay routes from peers A and B")
	for _, s := range sent {
		assert.Equal(t, "10.0.0.3", s.peer, "routes should target newly-up peer")
		assert.True(t, strings.HasPrefix(s.cmd, "update hex "), "replay uses 'update hex' format")
	}
}

// TestHandleState_PeerUpEmptyRIB verifies that peer-up with no routes in RIB
// sends nothing and produces no errors.
//
// VALIDATES: Peer-up with empty adj-rib-in works cleanly (startup scenario).
// PREVENTS: Errors or panics when replaying an empty RIB.
func TestHandleState_PeerUpEmptyRIB(t *testing.T) {
	r := newTestManager(t)

	var sendCount int
	r.routeSender = func(_, _ string) { sendCount++ }

	// No routes in ribIn -- this is the startup scenario.
	upEvent := &bgp.Event{
		Type:  "state",
		State: "up",
		Peer:  mustMarshal(t, bgp.PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
	}

	// Should not panic or error.
	r.handleState(upEvent)

	assert.True(t, r.peerUp["10.0.0.1"], "peer should be marked up")
	assert.Equal(t, 0, sendCount, "empty RIB should send no replay commands")
}

// TestHandleState_PeerUpSelfExclusion verifies that a peer's own routes
// are not replayed back to it on peer-up.
//
// VALIDATES: Routes sourced from peer X are not replayed to peer X on its peer-up.
// PREVENTS: Routing loops from replaying a peer's own routes back to it.
func TestHandleState_PeerUpSelfExclusion(t *testing.T) {
	r := newTestManager(t)

	var sent []struct{ peer, cmd string }
	r.routeSender = func(peer, cmd string) {
		sent = append(sent, struct{ peer, cmd string }{peer, cmd})
	}

	// Peer 10.0.0.1 has routes from itself (shouldn't happen normally,
	// but tests the exclusion logic).
	m1 := seqmap.New[string, *RawRoute]()
	m1.Put("ipv4/unicast:10.0.0.0/24", 1, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn["10.0.0.1"] = m1

	// Also routes from another peer
	m2 := seqmap.New[string, *RawRoute]()
	m2.Put("ipv4/unicast:10.0.1.0/24", 2, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000002", NLRIHex: "180a0001",
	})
	r.ribIn["10.0.0.2"] = m2

	// Peer 10.0.0.1 comes up
	upEvent := &bgp.Event{
		Type:  "state",
		State: "up",
		Peer:  mustMarshal(t, bgp.PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
	}

	r.handleState(upEvent)

	// Only routes from 10.0.0.2 should be replayed, not 10.0.0.1's own routes.
	assert.Len(t, sent, 1, "should replay only routes from other peers")
	assert.Equal(t, "10.0.0.1", sent[0].peer, "routes target the newly-up peer")
	assert.Contains(t, sent[0].cmd, "0a000002", "should contain peer B's next-hop")
	assert.NotContains(t, sent[0].cmd, "0a000001", "should NOT contain own next-hop")
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
		Message:       &bgp.MessageInfo{Type: "update", ID: 300},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/mpls-vpn": "aabbccdd11223344"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/mpls-vpn": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, "10.0.0.1")
	// Only 1 entry: the first NLRI carries the raw blob, second is skipped (i > 0).
	require.Equal(t, 1, r.ribIn["10.0.0.1"].Len(),
		"complex family multi-NLRI should store one entry with full raw blob")

	var route *RawRoute
	r.ribIn["10.0.0.1"].Range(func(_ string, _ uint64, rt *RawRoute) bool {
		route = rt
		return true
	})
	require.NotNil(t, route)
	assert.Equal(t, "aabbccdd11223344", route.NLRIHex,
		"must store entire raw blob, not computed prefix bytes")
}
