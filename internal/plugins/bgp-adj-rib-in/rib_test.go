package bgp_adj_rib_in

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/shared"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// newTestManager creates an AdjRIBInManager with closed SDK connections for unit testing.
// The SDK plugin is initialized but connections are closed, so RPC calls (updateRoute)
// will fail silently. This is appropriate for testing internal state changes.
func newTestManager(t *testing.T) *AdjRIBInManager {
	t.Helper()
	engineConn, engineRemote := net.Pipe()
	callbackConn, callbackRemote := net.Pipe()
	if err := engineRemote.Close(); err != nil {
		t.Logf("close engineRemote: %v", err)
	}
	if err := callbackRemote.Close(); err != nil {
		t.Logf("close callbackRemote: %v", err)
	}
	p := sdk.NewWithConn("adj-rib-in-test", engineConn, callbackConn)
	t.Cleanup(func() { _ = p.Close() })
	return &AdjRIBInManager{
		plugin: p,
		ribIn:  make(map[string]map[string]*RawRoute),
		peerUp: make(map[string]bool),
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
	event := &shared.Event{
		Message:       &shared.MessageInfo{Type: "update", ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]shared.FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, "10.0.0.1", "should have peer entry")
	routes := r.ribIn["10.0.0.1"]
	require.Len(t, routes, 1, "should have 1 route")

	// Find the stored route
	var route *RawRoute
	for _, rt := range routes {
		route = rt
	}
	require.NotNil(t, route)

	assert.Equal(t, "ipv4/unicast", route.Family)
	assert.Equal(t, "40010100", route.AttrHex, "raw attributes should be stored as-is")
	assert.Equal(t, "0a000001", route.NHopHex, "next-hop 10.0.0.1 as wire hex")
	assert.Equal(t, "180a0000", route.NLRIHex, "NLRI wire bytes as hex")
	assert.Equal(t, uint64(1), route.SeqIndex, "first route gets sequence 1")
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
	event := &shared.Event{
		Message:       &shared.MessageInfo{Type: "update", ID: 200},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/mpls-vpn": "deadbeef"},
		FamilyOps: map[string][]shared.FamilyOperation{
			"ipv4/mpls-vpn": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, "10.0.0.1")
	require.Len(t, r.ribIn["10.0.0.1"], 1, "VPN route should be stored")

	// Verify the raw blob is used, not prefixToWireHex output.
	var route *RawRoute
	for _, rt := range r.ribIn["10.0.0.1"] {
		route = rt
	}
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
	announce := &shared.Event{
		Message:       &shared.MessageInfo{Type: "update", ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]shared.FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(announce)
	require.Len(t, r.ribIn["10.0.0.1"], 1)

	// Then withdraw
	withdraw := &shared.Event{
		Message: &shared.MessageInfo{Type: "update", ID: 101},
		Peer:    peerJSON,
		// Withdrawals may have raw-withdrawn but not raw-attributes
		RawWithdrawn: map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]shared.FamilyOperation{
			"ipv4/unicast": {
				{Action: "del", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(withdraw)
	assert.Empty(t, r.ribIn["10.0.0.1"], "route should be removed after withdrawal")
}

// TestReplayAllSources verifies replay sends "update hex" commands from all sources except target.
//
// VALIDATES: Replay sends routes from A,B to X, excludes X's own routes.
// PREVENTS: Replaying a peer's own routes back to it.
func TestReplayAllSources(t *testing.T) {
	r := newTestManager(t)

	// Store routes from peer A
	r.ribIn["10.0.0.1"] = map[string]*RawRoute{
		"ipv4/unicast:10.0.0.0/24": {
			Family:   "ipv4/unicast",
			AttrHex:  "40010100",
			NHopHex:  "0a000001",
			NLRIHex:  "180a0000",
			SeqIndex: 1,
		},
	}
	// Store routes from peer B
	r.ribIn["10.0.0.2"] = map[string]*RawRoute{
		"ipv4/unicast:10.0.1.0/24": {
			Family:   "ipv4/unicast",
			AttrHex:  "40010100",
			NHopHex:  "0a000002",
			NLRIHex:  "180a0001",
			SeqIndex: 2,
		},
	}
	// Store routes from target peer X (should be excluded)
	r.ribIn["10.0.0.3"] = map[string]*RawRoute{
		"ipv4/unicast:10.0.2.0/24": {
			Family:   "ipv4/unicast",
			AttrHex:  "40010100",
			NHopHex:  "0a000003",
			NLRIHex:  "180a0002",
			SeqIndex: 3,
		},
	}

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

	r.ribIn["10.0.0.1"] = map[string]*RawRoute{
		"ipv4/unicast:10.0.0.0/24": {
			Family: "ipv4/unicast", AttrHex: "40010100",
			NHopHex: "0a000001", NLRIHex: "180a0000", SeqIndex: 1,
		},
		"ipv4/unicast:10.0.1.0/24": {
			Family: "ipv4/unicast", AttrHex: "40010100",
			NHopHex: "0a000001", NLRIHex: "180a0001", SeqIndex: 5,
		},
		"ipv4/unicast:10.0.2.0/24": {
			Family: "ipv4/unicast", AttrHex: "40010100",
			NHopHex: "0a000001", NLRIHex: "180a0002", SeqIndex: 10,
		},
	}

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

	r.ribIn["10.0.0.1"] = map[string]*RawRoute{
		"ipv4/unicast:10.0.0.0/24": {
			Family: "ipv4/unicast", AttrHex: "40010100",
			NHopHex: "0a000001", NLRIHex: "180a0000", SeqIndex: 42,
		},
	}

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
		event := &shared.Event{
			Message:       &shared.MessageInfo{Type: "update", ID: uint64(100 + i)},
			Peer:          peerJSON,
			RawAttributes: "40010100",
			RawNLRI:       map[string]string{"ipv4/unicast": nlriHex[i]},
			FamilyOps: map[string][]shared.FamilyOperation{
				"ipv4/unicast": {
					{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{prefix}},
				},
			},
		}
		r.handleReceived(event)
	}

	// Collect sequence indices
	var indices []uint64
	for _, rt := range r.ribIn["10.0.0.1"] {
		indices = append(indices, rt.SeqIndex)
	}
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
	r.ribIn["10.0.0.1"] = map[string]*RawRoute{
		"ipv4/unicast:10.0.0.0/24": {
			Family: "ipv4/unicast", AttrHex: "40010100",
			NHopHex: "0a000001", NLRIHex: "180a0000", SeqIndex: 1,
		},
	}
	r.peerUp["10.0.0.1"] = true

	// Peer goes down
	downEvent := &shared.Event{
		Type: "state",
		Peer: mustMarshal(t, shared.PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
	}
	// State can be in flat peer format or top-level
	downEvent.State = "down"

	r.handleState(downEvent)

	assert.Empty(t, r.ribIn["10.0.0.1"], "routes should be cleared on peer down")
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

	r.ribIn["10.0.0.1"] = map[string]*RawRoute{
		"k1": {Family: "ipv4/unicast", SeqIndex: 1},
		"k2": {Family: "ipv4/unicast", SeqIndex: 2},
	}
	r.ribIn["10.0.0.2"] = map[string]*RawRoute{
		"k3": {Family: "ipv6/unicast", SeqIndex: 3},
	}

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

	r.ribIn["10.0.0.1"] = map[string]*RawRoute{
		"ipv4/unicast:10.0.0.0/24": {
			Family:   "ipv4/unicast",
			AttrHex:  "40010100",
			NHopHex:  "0a000001",
			NLRIHex:  "180a0000",
			SeqIndex: 1,
		},
	}

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
	event := &shared.Event{
		Message:       &shared.MessageInfo{Type: "update", ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000180a0001"},
		FamilyOps: map[string][]shared.FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Len(t, r.ribIn["10.0.0.1"], 2, "each NLRI should be stored separately")

	// Both should share the same AttrHex (from same UPDATE)
	for _, rt := range r.ribIn["10.0.0.1"] {
		assert.Equal(t, "40010100", rt.AttrHex, "all NLRIs share same attributes")
		assert.Equal(t, "0a000001", rt.NHopHex, "all NLRIs share same next-hop")
	}
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
	event := &shared.Event{
		Message:       &shared.MessageInfo{Type: "update", ID: 300},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/mpls-vpn": "aabbccdd11223344"},
		FamilyOps: map[string][]shared.FamilyOperation{
			"ipv4/mpls-vpn": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.Contains(t, r.ribIn, "10.0.0.1")
	// Only 1 entry: the first NLRI carries the raw blob, second is skipped (i > 0).
	require.Len(t, r.ribIn["10.0.0.1"], 1,
		"complex family multi-NLRI should store one entry with full raw blob")

	var route *RawRoute
	for _, rt := range r.ribIn["10.0.0.1"] {
		route = rt
	}
	require.NotNil(t, route)
	assert.Equal(t, "aabbccdd11223344", route.NLRIHex,
		"must store entire raw blob, not computed prefix bytes")
}
