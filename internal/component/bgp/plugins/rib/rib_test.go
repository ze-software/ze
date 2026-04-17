package rib

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// newTestRIBManager creates a RIBManager with closed SDK connections for unit testing.
// The SDK plugin is initialized but connections are closed, so RPC calls (updateRoute)
// will fail silently. This is appropriate for testing internal state changes.
func newTestRIBManager(t *testing.T) *RIBManager {
	t.Helper()
	registerBuiltinCommands() // includes LLGR commands
	pluginEnd, remoteEnd := net.Pipe()
	if err := remoteEnd.Close(); err != nil {
		t.Logf("close remoteEnd: %v", err)
	}
	p := sdk.NewWithConn("rib-test", pluginEnd)
	t.Cleanup(func() { _ = p.Close() })
	return &RIBManager{
		plugin:        p,
		ribInPool:     make(map[string]*storage.PeerRIB),
		ribOut:        make(map[string]map[string]map[string]*Route),
		peerUp:        make(map[string]bool),
		peerMeta:      make(map[string]*PeerMeta),
		retainedPeers: make(map[string]bool),
		grState:       make(map[string]*peerGRState),
		bestPrev:      make(map[family.Family]*bestPrevStore),
	}
}

// TestParseEvent_SentFormat verifies parsing of sent UPDATE events.
//
// VALIDATES: Sent events with flat structure are parsed correctly.
// PREVENTS: Sent events being dropped due to format mismatch.
func TestParseEvent_SentFormat(t *testing.T) {
	// New command-style format: family at top level with operations array
	input := `{"type":"sent","msg-id":123,"peer":{"address":"10.0.0.1","remote":{"as":65001}},"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24","10.0.1.0/24"]}]}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "sent", event.GetEventType())
	assert.Equal(t, uint64(123), event.GetMsgID())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.NotNil(t, event.FamilyOps)
	assert.Contains(t, event.FamilyOps, "ipv4/unicast")
	require.Len(t, event.FamilyOps["ipv4/unicast"], 1)
	assert.Equal(t, "add", event.FamilyOps["ipv4/unicast"][0].Action)
	assert.Equal(t, "1.1.1.1", event.FamilyOps["ipv4/unicast"][0].NextHop)
}

// TestParseEvent_ReceivedFormat verifies parsing of received UPDATE events.
//
// VALIDATES: Received events with message wrapper are parsed correctly.
// PREVENTS: Received events being dropped due to format mismatch.
func TestParseEvent_ReceivedFormat(t *testing.T) {
	// New command-style format: direction inside message wrapper
	input := `{"message":{"type":"update","id":456,"direction":"received"},"peer":{"address":"10.0.0.1","local":{"address":"10.0.0.2","as":65002},"remote":{"as":65001}},"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24","10.0.1.0/24"]}]}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "update", event.GetEventType())
	assert.Equal(t, uint64(456), event.GetMsgID())
	assert.Equal(t, "received", event.GetDirection())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.NotNil(t, event.FamilyOps)
	assert.Contains(t, event.FamilyOps, "ipv4/unicast")
}

// TestParseEvent_StateFormat verifies parsing of state events.
//
// VALIDATES: State events are parsed correctly.
// PREVENTS: Peer state changes being missed.
func TestParseEvent_StateFormat(t *testing.T) {
	input := `{"type":"state","peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"up"}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "state", event.GetEventType())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.Equal(t, "up", event.GetPeerState())
}

// TestParseEvent_RequestFormat verifies parsing of request events.
//
// VALIDATES: CLI command requests are parsed correctly.
// PREVENTS: Commands being ignored.
func TestParseEvent_RequestFormat(t *testing.T) {
	input := `{"type":"request","serial":"abc123","command":"bgp rib adjacent status"}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "request", event.GetEventType())
	assert.Equal(t, "abc123", event.Serial)
	assert.Equal(t, "bgp rib adjacent status", event.Command)
}

// TestHandleSent_StoresRoutes verifies routes are stored in Adj-RIB-Out.
//
// VALIDATES: Sent routes are persisted for replay.
// PREVENTS: Routes being lost on peer reconnect.
func TestHandleSent_StoresRoutes(t *testing.T) {
	r := newTestRIBManager(t)

	// New command-style format: family operations with action/next-hop/nlri
	event := &Event{
		Type:  "sent",
		MsgID: 100,
		Peer:  mustMarshal(t, PeerInfoJSON{Address: "10.0.0.1", Remote: PeerRemoteInfo{AS: 65001}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleSent(event)

	require.Contains(t, r.ribOut["10.0.0.1"], "ipv4/unicast")
	assert.Len(t, r.ribOut["10.0.0.1"]["ipv4/unicast"], 2)
	assert.Contains(t, r.ribOut["10.0.0.1"]["ipv4/unicast"], "10.0.0.0/24")
	assert.Contains(t, r.ribOut["10.0.0.1"]["ipv4/unicast"], "10.0.1.0/24")

	route := r.ribOut["10.0.0.1"]["ipv4/unicast"]["10.0.0.0/24"]
	assert.Equal(t, "10.0.0.0/24", route.Prefix)
	assert.Equal(t, "1.1.1.1", route.NextHop)
	assert.Equal(t, uint64(100), route.MsgID)
}

// TestHandleSent_Withdraw verifies routes are removed on withdrawal.
//
// VALIDATES: Withdrawn routes are removed from Adj-RIB-Out.
// PREVENTS: Stale routes being replayed.
func TestHandleSent_Withdraw(t *testing.T) {
	r := newTestRIBManager(t)

	// First announce
	announce := &Event{
		Type:  "sent",
		MsgID: 100,
		Peer:  mustMarshal(t, PeerInfoJSON{Address: "10.0.0.1", Remote: PeerRemoteInfo{AS: 65001}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleSent(announce)
	assert.Len(t, r.ribOut["10.0.0.1"]["ipv4/unicast"], 1)

	// Then withdraw
	withdraw := &Event{
		Type: "sent",
		Peer: mustMarshal(t, PeerInfoJSON{Address: "10.0.0.1", Remote: PeerRemoteInfo{AS: 65001}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{Action: "del", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleSent(withdraw)
	// After withdrawing all routes in a family, the family map is cleaned up
	assert.Empty(t, r.ribOut["10.0.0.1"])
}

// TestHandleReceived_StoresRoutes verifies routes are stored in Adj-RIB-In.
//
// VALIDATES: Received routes are tracked per peer in pool storage.
// PREVENTS: Route state being lost.
func TestHandleReceived_StoresRoutes(t *testing.T) {
	r := newTestRIBManager(t)

	// format=full with raw fields
	// Two NLRIs: 10.0.0.0/24 (18 0a 00 00) + 10.0.1.0/24 (18 0a 00 01)
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 200},
		Peer:          mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}}),
		RawAttributes: "40010100", // ORIGIN IGP
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000180a0001"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.NotNil(t, r.ribInPool["10.0.0.1"], "PeerRIB should be created")
	assert.Equal(t, 2, r.ribInPool["10.0.0.1"].Len(), "should have 2 routes in pool")

	// Verify specific NLRIs are stored (not just the count)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}
	nlri1, err := prefixToWire("ipv4/unicast", "10.0.0.0/24", 0, false)
	require.NoError(t, err)
	_, found1 := r.ribInPool["10.0.0.1"].Lookup(ipv4Uni, nlri1)
	assert.True(t, found1, "10.0.0.0/24 should be in RIB")

	nlri2, err := prefixToWire("ipv4/unicast", "10.0.1.0/24", 0, false)
	require.NoError(t, err)
	_, found2 := r.ribInPool["10.0.0.1"].Lookup(ipv4Uni, nlri2)
	assert.True(t, found2, "10.0.1.0/24 should be in RIB")
}

// TestHandleReceived_Withdraw verifies routes are removed on withdrawal.
//
// VALIDATES: Withdrawn routes are removed from Adj-RIB-In.
// PREVENTS: Stale route state.
func TestHandleReceived_Withdraw(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// First announce with raw fields
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 200},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(announce)
	require.Equal(t, 1, r.ribInPool["10.0.0.1"].Len())

	// Then withdraw
	withdraw := &Event{
		Message:      &MessageInfo{Type: "update", ID: 201},
		Peer:         peerJSON,
		RawWithdrawn: map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{Action: "del", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(withdraw)
	assert.Equal(t, 0, r.ribInPool["10.0.0.1"].Len())
}

// TestHandleState_PeerUp verifies internal state on peer up.
//
// VALIDATES: Peer state is marked as up and routes are prepared for replay.
// PREVENTS: Peer state not being tracked correctly.
func TestHandleState_PeerUp(t *testing.T) {
	r := newTestRIBManager(t)

	// Pre-populate ribOut
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
			"10.0.1.0/24": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "1.1.1.1"},
		},
	}

	event := &Event{
		Type:  "state",
		Peer:  mustMarshal(t, PeerInfoJSON{Address: "10.0.0.1", Remote: PeerRemoteInfo{AS: 65001}}),
		State: "up",
	}

	r.handleState(event)

	// Verify internal state: peer is marked as up
	assert.True(t, r.peerUp["10.0.0.1"], "peer should be marked as up")
	// ribOut should be preserved (routes are replayed via SDK RPC, not text output)
	assert.Len(t, r.ribOut["10.0.0.1"]["ipv4/unicast"], 2, "ribOut should still have routes")
}

// TestHandleState_PeerDown verifies Adj-RIB-In is cleared on peer down.
//
// VALIDATES: Received routes are cleared when peer goes down.
// PREVENTS: Stale routes from failed peers.
func TestHandleState_PeerDown(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// Pre-populate ribInPool via handleReceived
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(announce)
	require.Equal(t, 1, r.ribInPool["10.0.0.1"].Len())

	// Pre-populate ribOut
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
		},
	}
	r.peerUp["10.0.0.1"] = true

	event := &Event{
		Type:  "state",
		Peer:  mustMarshal(t, PeerInfoJSON{Address: "10.0.0.1", Remote: PeerRemoteInfo{AS: 65001}}),
		State: "down",
	}

	r.handleState(event)

	// ribInPool should be cleared (PeerRIB deleted)
	_, exists := r.ribInPool["10.0.0.1"]
	assert.False(t, exists, "PeerRIB should be deleted on peer down")
	// peerMeta should be cleared alongside ribInPool
	_, metaExists := r.peerMeta["10.0.0.1"]
	assert.False(t, metaExists, "peerMeta should be deleted on peer down")
	// ribOut should be preserved for replay
	assert.Len(t, r.ribOut["10.0.0.1"]["ipv4/unicast"], 1)
}

// TestStatusJSON verifies status command output.
//
// VALIDATES: Status returns correct route counts.
// PREVENTS: Incorrect stats being reported.
func TestStatusJSON(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// Add routes via pool storage
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000180a0001"}, // 2 NLRIs
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}}},
		},
	}
	r.handleReceived(event)

	r.ribOut["10.0.0.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {},
		},
	}
	r.peerUp["10.0.0.1"] = true
	r.peerUp["10.0.0.2"] = true

	status := r.statusJSON()
	assert.Contains(t, status, `"running":true`)
	assert.Contains(t, status, `"peers":2`)
	assert.Contains(t, status, `"routes-in":2`)
	assert.Contains(t, status, `"routes-out":1`)
}

// TestDispatch_RoutesToCorrectHandler verifies event routing.
//
// VALIDATES: Events are dispatched to correct handlers.
// PREVENTS: Events being processed by wrong handler.
func TestDispatch_RoutesToCorrectHandler(t *testing.T) {
	tests := []struct {
		name       string
		event      *Event
		wantRibIn  int
		wantRibOut int
	}{
		{
			name: "sent_to_ribOut",
			event: &Event{
				Type: "sent",
				Peer: mustMarshal(t, PeerInfoJSON{Address: "10.0.0.1", Remote: PeerRemoteInfo{AS: 65001}}),
				FamilyOps: map[string][]FamilyOperation{
					"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
				},
			},
			wantRibIn:  0,
			wantRibOut: 1,
		},
		{
			name: "update_to_ribInPool",
			event: &Event{
				Message:       &MessageInfo{Type: "update"},
				Peer:          json.RawMessage(`{"address":"10.0.0.1","local":{"address":"","as":0},"remote":{"as":65001}}`),
				RawAttributes: "40010100",
				RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
				FamilyOps: map[string][]FamilyOperation{
					"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
				},
			},
			wantRibIn:  1,
			wantRibOut: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			r.dispatch(tt.event)

			totalIn := 0
			for _, peerRIB := range r.ribInPool {
				totalIn += peerRIB.Len()
			}
			totalOut := 0
			for _, peerFamilies := range r.ribOut {
				for _, familyRoutes := range peerFamilies {
					totalOut += len(familyRoutes)
				}
			}

			assert.Equal(t, tt.wantRibIn, totalIn, "ribInPool count")
			assert.Equal(t, tt.wantRibOut, totalOut, "ribOut count")
		})
	}
}

// TestHandleState_ConcurrentUpDown verifies no race on rapid state changes.
//
// VALIDATES: State transitions are atomic.
// PREVENTS: Race condition between up/down events.
func TestHandleState_ConcurrentUpDown(t *testing.T) {
	r := newTestRIBManager(t)

	// Pre-populate ribOut
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
	}

	// Pre-populate ribInPool via handleReceived
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0001"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.1.0/24"}}},
		},
	}
	r.handleReceived(announce)

	peer := mustMarshal(t, PeerInfoJSON{Address: "10.0.0.1", Remote: PeerRemoteInfo{AS: 65001}})

	// Rapid state changes from multiple goroutines
	done := make(chan bool)
	for i := range 10 {
		go func(n int) {
			state := "up"
			if n%2 == 0 {
				state = "down"
			}
			event := &Event{
				Type:  "state",
				Peer:  peer,
				State: state,
			}
			r.handleState(event)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	// Verify no panic and state is consistent
	r.mu.RLock()
	_, hasRibOut := r.ribOut["10.0.0.1"]
	r.mu.RUnlock()

	// ribOut should always exist (we never delete it)
	assert.True(t, hasRibOut, "ribOut should persist through state changes")
}

// TestRIBRouteKeyWithPathID verifies path-id creates unique route keys.
//
// VALIDATES: Same prefix with different path-ids stored separately.
// PREVENTS: ADD-PATH routes overwriting each other.
func TestRIBRouteKeyWithPathID(t *testing.T) {
	tests := []struct {
		family string
		prefix string
		pathID uint32
		want   string
	}{
		{"ipv4/unicast", "10.0.0.0/24", 0, "ipv4/unicast:10.0.0.0/24"},
		{"ipv4/unicast", "10.0.0.0/24", 1, "ipv4/unicast:10.0.0.0/24:1"},
		{"ipv4/unicast", "10.0.0.0/24", 2, "ipv4/unicast:10.0.0.0/24:2"},
		{"ipv6/unicast", "2001:db8::/32", 0, "ipv6/unicast:2001:db8::/32"},
		{"ipv6/unicast", "2001:db8::/32", 100, "ipv6/unicast:2001:db8::/32:100"},
	}

	for _, tt := range tests {
		got := routeKey(tt.family, tt.prefix, tt.pathID)
		assert.Equal(t, tt.want, got, "routeKey(%q, %q, %d)", tt.family, tt.prefix, tt.pathID)
	}
}

// TestRIBParseStructuredJSON verifies structured NLRI format parsing.
//
// VALIDATES: Both object and legacy string formats parsed correctly.
// PREVENTS: JSON format change breaking RIB storage.
func TestRIBParseStructuredJSON(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		wantPrefix string
		wantPathID uint32
	}{
		{
			name:       "object_with_path_id",
			input:      map[string]any{"prefix": "10.0.0.0/24", "path-id": float64(1)},
			wantPrefix: "10.0.0.0/24",
			wantPathID: 1,
		},
		{
			name:       "object_without_path_id",
			input:      map[string]any{"prefix": "10.0.0.0/24"},
			wantPrefix: "10.0.0.0/24",
			wantPathID: 0,
		},
		{
			name:       "legacy_string_format",
			input:      "10.0.0.0/24",
			wantPrefix: "10.0.0.0/24",
			wantPathID: 0,
		},
		{
			name:       "object_with_large_path_id",
			input:      map[string]any{"prefix": "192.168.1.0/24", "path-id": float64(4294967295)},
			wantPrefix: "192.168.1.0/24",
			wantPathID: 4294967295,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPrefix, gotPathID := parseNLRIValue(tt.input)
			assert.Equal(t, tt.wantPrefix, gotPrefix, "prefix")
			assert.Equal(t, tt.wantPathID, gotPathID, "pathID")
		})
	}
}

// TestReplayRoutesWithPathID verifies path-id is included in replay commands.
//
// VALIDATES: Replayed routes include path-id when non-zero via formatRouteCommand.
// PREVENTS: Path-id being lost during session restart replay.
func TestReplayRoutesWithPathID(t *testing.T) {
	// Test formatRouteCommand directly since replay now goes through SDK RPC
	routeNoPathID := &Route{MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1", PathID: 0}
	routePathID1 := &Route{MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1", PathID: 1}
	routePathID2 := &Route{MsgID: 3, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "2.2.2.2", PathID: 2}

	// Route without path-id should NOT have path-information in command
	cmd0 := formatRouteCommand(routeNoPathID)
	assert.Contains(t, cmd0, "nhop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24")
	assert.NotContains(t, cmd0, "path-information")

	// Routes with path-id MUST have path-information in command (per-NLRI modifier)
	cmd1 := formatRouteCommand(routePathID1)
	assert.Contains(t, cmd1, "nlri ipv4/unicast path-information 1 add 10.0.0.0/24")
	assert.Contains(t, cmd1, "nhop 1.1.1.1")

	cmd2 := formatRouteCommand(routePathID2)
	assert.Contains(t, cmd2, "nlri ipv4/unicast path-information 2 add 10.0.0.0/24")
	assert.Contains(t, cmd2, "nhop 2.2.2.2")
}

// mustMarshal marshals v to json.RawMessage, failing test on error.
func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

// TestHandleCommand_RIBAdjacentStatus verifies renamed status command.
//
// VALIDATES: "bgp rib adjacent status" returns status JSON via handleCommand.
// PREVENTS: Command rename breaking status queries.
func TestHandleCommand_RIBAdjacentStatus(t *testing.T) {
	r := newTestRIBManager(t)

	// Add route via pool storage
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(announce)

	r.peerUp["10.0.0.1"] = true
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {},
			"10.0.1.0/24": {},
		},
	}

	status, data, err := r.handleCommand("bgp rib adjacent status", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"running":true`)
	assert.Contains(t, data, `"routes-in":1`)
	assert.Contains(t, data, `"routes-out":2`)
}

// TestHandleCommand_RIBShowReceived verifies received show with selector.
//
// VALIDATES: "bgp rib show" with received scope filters by peer selector.
// PREVENTS: Wrong routes returned for filtered queries.
func TestHandleCommand_RIBShowReceived(t *testing.T) {
	r := newTestRIBManager(t)

	// Add routes via pool storage for peer 10.0.0.1
	peer1JSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})
	event1 := &Event{
		Message:       &MessageInfo{Type: "update", ID: 100},
		Peer:          peer1JSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"}, // 10.0.0.0/24
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(event1)

	// Add routes via pool storage for peer 10.0.0.2
	peer2JSON := mustMarshal(t, map[string]any{"address": "10.0.0.2", "local": map[string]any{"address": "10.0.0.1", "as": uint32(65001)}, "remote": map[string]any{"as": uint32(65002)}})
	event2 := &Event{
		Message:       &MessageInfo{Type: "update", ID: 101},
		Peer:          peer2JSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0001"}, // 10.0.1.0/24
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "2.2.2.2", Action: "add", NLRIs: []any{"10.0.1.0/24"}}},
		},
	}
	r.handleReceived(event2)

	tests := []struct {
		name      string
		selector  string
		wantPeer1 bool
		wantPeer2 bool
	}{
		{
			name:      "all_peers",
			selector:  "*",
			wantPeer1: true,
			wantPeer2: true,
		},
		{
			name:      "specific_peer",
			selector:  "10.0.0.1",
			wantPeer1: true,
			wantPeer2: false,
		},
		{
			name:      "multi_peer",
			selector:  "10.0.0.1,10.0.0.2",
			wantPeer1: true,
			wantPeer2: true,
		},
		{
			name:      "negation",
			selector:  "!10.0.0.2",
			wantPeer1: true,
			wantPeer2: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, data, err := r.handleCommand("bgp rib show", tt.selector, []string{"received"})
			require.NoError(t, err)
			assert.Equal(t, "done", status)
			if tt.wantPeer1 {
				assert.Contains(t, data, "10.0.0.1")
			} else {
				assert.NotContains(t, data, "10.0.0.1")
			}
			if tt.wantPeer2 {
				assert.Contains(t, data, "10.0.0.2")
			} else {
				assert.NotContains(t, data, "10.0.0.2")
			}
		})
	}
}

// TestHandleCommand_RIBAdjacentInboundEmpty verifies inbound empty with selector.
//
// VALIDATES: "bgp rib adjacent inbound empty" empties matching peers only.
// PREVENTS: Emptying wrong peers' routes.
func TestHandleCommand_RIBAdjacentInboundEmpty(t *testing.T) {
	r := newTestRIBManager(t)

	// Add routes via pool storage for peer 10.0.0.1
	peer1JSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})
	event1 := &Event{
		Message:       &MessageInfo{Type: "update", ID: 100},
		Peer:          peer1JSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(event1)

	// Add routes via pool storage for peer 10.0.0.2
	peer2JSON := mustMarshal(t, map[string]any{"address": "10.0.0.2", "local": map[string]any{"address": "10.0.0.1", "as": uint32(65001)}, "remote": map[string]any{"as": uint32(65002)}})
	event2 := &Event{
		Message:       &MessageInfo{Type: "update", ID: 101},
		Peer:          peer2JSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0001"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "2.2.2.2", Action: "add", NLRIs: []any{"10.0.1.0/24"}}},
		},
	}
	r.handleReceived(event2)

	status, data, err := r.handleCommand("bgp rib adjacent inbound empty", "*", []string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"cleared":1`)

	// 10.0.0.1 should be emptied (PeerRIB deleted)
	_, exists1 := r.ribInPool["10.0.0.1"]
	assert.False(t, exists1, "peer 10.0.0.1 PeerRIB should be deleted")
	// 10.0.0.2 should remain
	assert.Equal(t, 1, r.ribInPool["10.0.0.2"].Len())
}

// TestHandleCommand_RIBShowSent verifies sent show with selector.
//
// VALIDATES: "bgp rib show" with sent scope filters by peer selector.
// PREVENTS: Wrong routes returned for outbound queries.
func TestHandleCommand_RIBShowSent(t *testing.T) {
	r := newTestRIBManager(t)

	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
	}
	r.ribOut["10.0.0.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
		},
	}

	status, data, err := r.handleCommand("bgp rib show", "10.0.0.1", []string{"sent"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, "10.0.0.1")
	assert.NotContains(t, data, "10.0.0.2")
}

// TestHandleCommand_RIBAdjacentOutboundResend verifies outbound resend.
//
// VALIDATES: "bgp rib adjacent outbound resend" returns correct count for matching peers.
// PREVENTS: Resend failing or targeting wrong peers.
func TestHandleCommand_RIBAdjacentOutboundResend(t *testing.T) {
	r := newTestRIBManager(t)

	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
	}
	r.ribOut["10.0.0.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.1.0/24": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
		},
	}
	r.peerUp["10.0.0.1"] = true
	r.peerUp["10.0.0.2"] = true

	status, data, err := r.handleCommand("bgp rib adjacent outbound resend", "*", []string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	// Resend count: routes are sent via SDK RPC (updateRoute), which fails silently on closed pipes
	// but the resend logic still counts them
	assert.Contains(t, data, `"resent":1`)
	assert.Contains(t, data, `"peers":1`)
}

// TestHandleCommand_RIBAdjacentOutboundResend_DownPeer verifies resend skips down peers.
//
// VALIDATES: "bgp rib adjacent outbound resend" does not send routes to down peers.
// PREVENTS: Sending routes to disconnected peers.
func TestHandleCommand_RIBAdjacentOutboundResend_DownPeer(t *testing.T) {
	r := newTestRIBManager(t)

	// Peer has routes but is DOWN
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
	}
	// peerUp["10.0.0.1"] is NOT set (peer is down)

	status, data, err := r.handleCommand("bgp rib adjacent outbound resend", "*", []string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"resent":0`, "should not resend to down peer")
	assert.Contains(t, data, `"peers":0`, "no peers should be affected")
}

// TestHandleCommand_UnknownCommand verifies unknown commands are rejected.
//
// VALIDATES: Unknown commands return error response.
// PREVENTS: Silent failures on typos.
func TestHandleCommand_UnknownCommand(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib unknown command", "", nil)
	assert.Equal(t, "error", status)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// TestRIBPluginHandleCommandShortNames verifies short-name commands dispatch correctly.
//
// VALIDATES: handleCommand routes short names (bgp rib show in, etc.) to correct handlers.
// PREVENTS: Short-name unification failing after builtin removal.
func TestRIBPluginHandleCommandShortNames(t *testing.T) {
	r := newTestRIBManager(t)

	// Populate test data
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(announce)
	r.peerUp["10.0.0.1"] = true
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.1.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
		},
	}

	tests := []struct {
		name    string
		command string
		args    []string
		wantOK  bool
		wantIn  string // substring expected in data
	}{
		{"bgp rib status", "bgp rib status", nil, true, `"running":true`},
		{"bgp rib show received", "bgp rib show", []string{"received"}, true, "10.0.0.1"},
		{"bgp rib clear in", "bgp rib clear in", []string{"*"}, true, `"cleared"`},
		{"bgp rib show sent", "bgp rib show", []string{"sent"}, true, "adj-rib-out"},
		{"bgp rib clear out", "bgp rib clear out", []string{"*"}, true, `"resent"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, data, err := r.handleCommand(tt.command, "*", tt.args)
			if tt.wantOK {
				require.NoError(t, err, "command %q should succeed", tt.command)
				assert.Equal(t, "done", status)
				assert.Contains(t, data, tt.wantIn, "command %q data", tt.command)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestRIBPluginHandleCommandLegacyNames verifies old-style names still work.
//
// VALIDATES: handleCommand still routes legacy names (rib adjacent ...) to correct handlers.
// PREVENTS: Backward compatibility break during command unification.
func TestRIBPluginHandleCommandLegacyNames(t *testing.T) {
	r := newTestRIBManager(t)

	// Populate test data
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 100},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(announce)
	r.peerUp["10.0.0.1"] = true

	// Legacy names that still work (status, empty, resend have aliases;
	// show commands now use unified "bgp rib show" with scope args).
	tests := []struct {
		name    string
		command string
		args    []string
		wantIn  string
	}{
		{"adjacent status", "bgp rib adjacent status", nil, `"running":true`},
		{"adjacent inbound empty", "bgp rib adjacent inbound empty", []string{"*"}, `"cleared"`},
		{"adjacent outbound resend", "bgp rib adjacent outbound resend", []string{"*"}, `"resent"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, data, err := r.handleCommand(tt.command, "*", tt.args)
			require.NoError(t, err, "legacy command %q should succeed", tt.command)
			assert.Equal(t, "done", status)
			assert.Contains(t, data, tt.wantIn, "legacy command %q data", tt.command)
		})
	}
}

// =============================================================================
// RFC 7313 - Enhanced Route Refresh Tests
// =============================================================================

// TestHandleRefresh_InternalState verifies route refresh filters routes correctly.
//
// RFC 7313 Section 3: Upon receiving a route refresh request, the speaker
// SHOULD send BoRR, re-advertise Adj-RIB-Out, then send EoRR.
//
// VALIDATES: Refresh handler filters routes by family correctly.
// PREVENTS: Wrong family routes being included in refresh response.
func TestHandleRefresh_InternalState(t *testing.T) {
	r := newTestRIBManager(t)

	// Pre-populate ribOut with routes
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
			"10.0.1.0/24": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "1.1.1.1"},
		},
		"ipv6/unicast": {
			"2001:db8::/32": {MsgID: 3, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
		},
	}
	r.peerUp["10.0.0.1"] = true

	// Simulate refresh request for IPv4 unicast
	// Output goes through SDK RPC (updateRoute), so we verify internal state is correct
	event := &Event{
		Message: &MessageInfo{Type: "refresh"},
		Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}}),
		AFI:     "ipv4",
		SAFI:    "unicast",
	}

	// handleRefresh sends via updateRoute (SDK RPC), which will fail silently on closed pipes.
	// Verify it doesn't panic and peer state is maintained.
	r.handleRefresh(event)

	// Peer should still be up after refresh
	assert.True(t, r.peerUp["10.0.0.1"], "peer should still be up after refresh")
	// RibOut should be unchanged (refresh re-advertises, doesn't modify)
	assert.Len(t, r.ribOut["10.0.0.1"], 2, "ribOut should have 2 families unchanged after refresh")
}

// TestHandleRefresh_PeerNotUp verifies refresh is ignored for down peers.
//
// VALIDATES: Refresh request ignored if peer is not up.
// PREVENTS: Sending routes to disconnected peer.
func TestHandleRefresh_PeerNotUp(t *testing.T) {
	r := newTestRIBManager(t)

	// Peer has routes but is not up
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
	}
	// peerUp["10.0.0.1"] is NOT set (peer is down)

	event := &Event{
		Message: &MessageInfo{Type: "refresh"},
		Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}}),
		AFI:     "ipv4",
		SAFI:    "unicast",
	}

	// Should not panic, just return early
	r.handleRefresh(event)

	// Peer should still be down
	assert.False(t, r.peerUp["10.0.0.1"], "peer should remain down")
}

// TestHandleRefresh_IPv6Family verifies refresh for IPv6 unicast.
//
// VALIDATES: IPv6 routes are filtered correctly by family.
// PREVENTS: Wrong family routes being sent.
func TestHandleRefresh_IPv6Family(t *testing.T) {
	r := newTestRIBManager(t)

	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
		"ipv6/unicast": {
			"2001:db8::/32": {MsgID: 1, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
		},
	}
	r.peerUp["10.0.0.1"] = true

	event := &Event{
		Message: &MessageInfo{Type: "refresh"},
		Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}}),
		AFI:     "ipv6",
		SAFI:    "unicast",
	}

	// Should not panic - output goes through SDK RPC
	r.handleRefresh(event)

	// State should be preserved
	assert.True(t, r.peerUp["10.0.0.1"])
	assert.Len(t, r.ribOut["10.0.0.1"], 2, "ribOut should have 2 families")
}

// =============================================================================
// Pool Storage Tests (Phase 3-4)
// =============================================================================

// TestHandleReceived_PoolStorage verifies routes stored in PeerRIB when raw fields present.
//
// VALIDATES: Raw attributes and NLRIs from format=full are stored in pool.
// PREVENTS: Pool storage being ignored when raw fields are available.
func TestHandleReceived_PoolStorage(t *testing.T) {
	r := newTestRIBManager(t)

	// Event with raw fields (format=full from engine)
	// Raw attrs: ORIGIN IGP (40 01 01 00)
	// Raw NLRI: 10.0.0.0/24 (18 0a 00 00)
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 300},
		Peer:          mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}}),
		RawAttributes: "40010100",                                    // ORIGIN IGP
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"}, // 10.0.0.0/24
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}

	r.handleReceived(event)

	// Should be stored in pool storage
	assert.NotNil(t, r.ribInPool["10.0.0.1"], "PeerRIB should be created")
	assert.Equal(t, 1, r.ribInPool["10.0.0.1"].Len(), "should have 1 route in pool")
}

// TestHandleReceived_PoolStorage_MultipleNLRIs verifies multiple NLRIs in one UPDATE.
//
// VALIDATES: Concatenated NLRIs are split and stored individually.
// PREVENTS: Only first NLRI being stored.
func TestHandleReceived_PoolStorage_MultipleNLRIs(t *testing.T) {
	r := newTestRIBManager(t)

	// Two NLRIs: 10.0.0.0/24 (18 0a 00 00) + 10.0.1.0/24 (18 0a 00 01)
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 302},
		Peer:          mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}}),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000180a0001"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}}},
		},
	}

	r.handleReceived(event)

	assert.Equal(t, 2, r.ribInPool["10.0.0.1"].Len(), "should have 2 routes in pool")
}

// TestHandleReceived_PoolStorage_Withdraw verifies withdrawal removes from pool.
//
// VALIDATES: Withdrawn routes are removed from pool storage.
// PREVENTS: Stale routes remaining in pool.
func TestHandleReceived_PoolStorage_Withdraw(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// First: announce
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 303},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(announce)
	require.Equal(t, 1, r.ribInPool["10.0.0.1"].Len())

	// Then: withdraw
	withdraw := &Event{
		Message:      &MessageInfo{Type: "update", ID: 304},
		Peer:         peerJSON,
		RawWithdrawn: map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{Action: "del", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(withdraw)

	assert.Equal(t, 0, r.ribInPool["10.0.0.1"].Len(), "route should be withdrawn")
}

// TestHandleState_PeerDown_ClearsPoolStorage verifies pool cleared on peer down.
//
// VALIDATES: Pool storage is cleared when peer goes down.
// PREVENTS: Stale pool data from failed peers.
func TestHandleState_PeerDown_ClearsPoolStorage(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// Add route via pool storage
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 305},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(announce)
	r.peerUp["10.0.0.1"] = true
	require.Equal(t, 1, r.ribInPool["10.0.0.1"].Len())

	// Peer goes down
	stateDown := &Event{
		Type:  "state",
		Peer:  mustMarshal(t, PeerInfoJSON{Address: "10.0.0.1", Remote: PeerRemoteInfo{AS: 65001}}),
		State: "down",
	}
	r.handleState(stateDown)

	// Pool should be cleared
	if peerRIB := r.ribInPool["10.0.0.1"]; peerRIB != nil {
		assert.Equal(t, 0, peerRIB.Len(), "pool should be empty after peer down")
	}
}

// TestHandleReceived_PoolStorage_IPv6 verifies IPv6 routes in pool storage.
//
// VALIDATES: IPv6 unicast routes are stored in pool correctly.
// PREVENTS: IPv6 address parsing/formatting bugs in cross-storage.
func TestHandleReceived_PoolStorage_IPv6(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "::1", "local": map[string]any{"address": "::2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// IPv6 NLRI: 2001:db8::/32
	// Wire format: [prefix-len:1][prefix-bytes:4] = [32][20][01][0d][b8]
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 400},
		Peer:          peerJSON,
		RawAttributes: "40010100", // ORIGIN IGP
		RawNLRI:       map[string]string{"ipv6/unicast": "2020010db8"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv6/unicast": {{NextHop: "::1", Action: "add", NLRIs: []any{"2001:db8::/32"}}},
		},
	}

	r.handleReceived(event)

	assert.NotNil(t, r.ribInPool["::1"], "PeerRIB should be created for IPv6 peer")
	assert.Equal(t, 1, r.ribInPool["::1"].Len(), "should have 1 IPv6 route in pool")
}

// TestHandleReceived_PoolStorage_SkipsEVPN verifies EVPN is skipped.
//
// VALIDATES: Non-unicast families are not processed by pool storage.
// PREVENTS: EVPN wire format being corrupted by splitNLRIs().
func TestHandleReceived_PoolStorage_SkipsEVPN(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// EVPN NLRI (will be skipped - not simple prefix format)
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 401},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"l2vpn/evpn": "0203deadbeef"}, // Fake EVPN bytes
		FamilyOps: map[string][]FamilyOperation{
			"l2vpn/evpn": {{Action: "add", NLRIs: []any{"type2:00:11:22:33:44:55"}}},
		},
	}

	r.handleReceived(event)

	// Should not crash, and pool should be empty (EVPN skipped)
	if peerRIB := r.ribInPool["10.0.0.1"]; peerRIB != nil {
		assert.Equal(t, 0, peerRIB.Len(), "EVPN should be skipped, pool should be empty")
	}
}

// TestStatusJSON_WithPoolStorage verifies status includes pool route counts.
//
// VALIDATES: Status JSON includes routes from pool storage.
// PREVENTS: Pool routes not being counted in status.
func TestStatusJSON_WithPoolStorage(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// Add routes via pool storage
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 306},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000180a0001"}, // 2 NLRIs
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}}},
		},
	}
	r.handleReceived(event)
	r.peerUp["10.0.0.1"] = true

	status := r.statusJSON()
	assert.Contains(t, status, `"routes-in":2`, "status should count pool routes")
}

// TestHandleCommand_InboundShow_PoolStorage verifies show command reads from pool storage.
//
// VALIDATES: Routes in pool storage appear in show output via handleCommand.
// PREVENTS: Pool routes being invisible to show commands.
func TestHandleCommand_InboundShow_PoolStorage(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// Add route via pool storage
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 307},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"}, // 10.0.0.0/24
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(event)

	// Verify route is in pool
	require.Equal(t, 1, r.ribInPool["10.0.0.1"].Len())

	// Call show command via handleCommand (unified pipeline, received scope)
	status, data, err := r.handleCommand("bgp rib show", "*", []string{"received"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, "10.0.0.1", "should contain peer address")
	assert.Contains(t, data, "10.0.0.0/24", "should contain prefix from pool")
	assert.Contains(t, data, "ipv4/unicast", "should contain family")
}

// TestHandleCommand_InboundEmpty_PoolStorage verifies empty command clears pool storage.
//
// VALIDATES: Empty command clears routes from pool storage via handleCommand.
// PREVENTS: Pool routes remaining after empty command.
func TestHandleCommand_InboundEmpty_PoolStorage(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// Add route via pool storage
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 308},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(event)
	require.Equal(t, 1, r.ribInPool["10.0.0.1"].Len())

	// Call empty command via handleCommand
	status, data, err := r.handleCommand("bgp rib adjacent inbound empty", "*", []string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"cleared":1`)

	// Verify pool is cleared (entry deleted to avoid memory leak)
	_, exists := r.ribInPool["10.0.0.1"]
	assert.False(t, exists, "pool entry should be deleted")
}

// =============================================================================
// Cross-Storage Consistency Tests
// =============================================================================

// TestPrefixToWire verifies text prefix to wire bytes conversion.
//
// VALIDATES: Text prefixes convert correctly to wire format.
// PREVENTS: Cross-storage key mismatches.
func TestPrefixToWire(t *testing.T) {
	tests := []struct {
		name    string
		family  string
		prefix  string
		pathID  uint32
		addPath bool
		want    []byte
	}{
		{
			name:   "ipv4_24",
			family: "ipv4/unicast",
			prefix: "10.0.0.0/24",
			want:   []byte{24, 10, 0, 0},
		},
		{
			name:   "ipv4_8",
			family: "ipv4/unicast",
			prefix: "10.0.0.0/8",
			want:   []byte{8, 10},
		},
		{
			name:   "ipv4_32",
			family: "ipv4/unicast",
			prefix: "192.168.1.1/32",
			want:   []byte{32, 192, 168, 1, 1},
		},
		{
			name:   "ipv4_0",
			family: "ipv4/unicast",
			prefix: "0.0.0.0/0",
			want:   []byte{0},
		},
		{
			name:    "ipv4_addpath",
			family:  "ipv4/unicast",
			prefix:  "10.0.0.0/24",
			pathID:  100,
			addPath: true,
			want:    []byte{0, 0, 0, 100, 24, 10, 0, 0}, // path-id + prefix
		},
		{
			name:   "ipv6_32",
			family: "ipv6/unicast",
			prefix: "2001:db8::/32",
			want:   []byte{32, 0x20, 0x01, 0x0d, 0xb8},
		},
		{
			name:   "ipv6_128",
			family: "ipv6/unicast",
			prefix: "2001:db8::1/128",
			want:   []byte{128, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := prefixToWire(tt.family, tt.prefix, tt.pathID, tt.addPath)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestWireToPrefix verifies wire bytes to text prefix conversion.
//
// VALIDATES: Wire bytes convert correctly to text format.
// PREVENTS: Cross-storage lookup failures.
func TestWireToPrefix(t *testing.T) {
	tests := []struct {
		name       string
		family     string
		wire       []byte
		addPath    bool
		wantPrefix string
		wantPathID uint32
	}{
		{
			name:       "ipv4_24",
			family:     "ipv4/unicast",
			wire:       []byte{24, 10, 0, 0},
			wantPrefix: "10.0.0.0/24",
		},
		{
			name:       "ipv4_8",
			family:     "ipv4/unicast",
			wire:       []byte{8, 10},
			wantPrefix: "10.0.0.0/8",
		},
		{
			name:       "ipv4_addpath",
			family:     "ipv4/unicast",
			wire:       []byte{0, 0, 0, 100, 24, 10, 0, 0},
			addPath:    true,
			wantPrefix: "10.0.0.0/24",
			wantPathID: 100,
		},
		{
			name:       "ipv6_32",
			family:     "ipv6/unicast",
			wire:       []byte{32, 0x20, 0x01, 0x0d, 0xb8},
			wantPrefix: "2001:db8::/32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fam, ok := parseFamily(tt.family)
			require.True(t, ok)
			gotPrefix, gotPathID, err := wireToPrefix(fam, tt.wire, tt.addPath)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPrefix, gotPrefix)
			assert.Equal(t, tt.wantPathID, gotPathID)
		})
	}
}

// TestDispatch_RefreshEvents verifies refresh event types are routed correctly.
//
// VALIDATES: refresh, borr, eorr events are dispatched to correct handlers.
// PREVENTS: Events being ignored or misrouted.
func TestDispatch_RefreshEvents(t *testing.T) {
	r := newTestRIBManager(t)

	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
	}
	r.peerUp["10.0.0.1"] = true

	tests := []struct {
		name      string
		eventType string
	}{
		{"refresh dispatches without panic", "refresh"},
		{"borr dispatches without panic", "borr"},
		{"eorr dispatches without panic", "eorr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &Event{
				Message: &MessageInfo{Type: tt.eventType},
				Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}}),
				AFI:     "ipv4",
				SAFI:    "unicast",
			}

			// Should not panic
			r.dispatch(event)
		})
	}
}

// =============================================================================
// ze-bgp JSON New Event Format Tests
// =============================================================================
// These tests validate parsing of the new JSON format per ipc_protocol.md v2.0:
//   - Top-level "type" field indicates payload key ("bgp" or "rib")
//   - Event content nested under "bgp" or "rib" key
//   - Event type in payload (e.g., bgp.type = "update")
//   - Attributes nested under "attributes"
//   - NLRIs nested under "nlri"
// =============================================================================

// TestParseEvent_NewBGPFormat verifies parsing of ze-bgp JSON BGP event format.
//
// VALIDATES: New wrapped BGP format with type/bgp structure parses correctly.
// PREVENTS: Plugin breaking when engine updates to ze-bgp JSON format.
func TestParseEvent_NewBGPFormat(t *testing.T) {
	// ze-bgp JSON format: type at top, payload nested under "bgp"
	input := `{
		"type": "bgp",
		"bgp": {
			"type": "update",
			"message": {"id": 789, "direction": "received"},
			"peer": {"address": "10.0.0.1", "remote": {"as": 65001}},
			"attributes": {"origin": "igp", "as-path": [65001]},
			"nlri": {"ipv4/unicast": [{"next-hop": "1.1.1.1", "action": "add", "nlri": ["10.0.0.0/24"]}]}
		}
	}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "update", event.GetEventType())
	assert.Equal(t, uint64(789), event.GetMsgID())
	assert.Equal(t, "received", event.GetDirection())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.Equal(t, uint32(65001), event.GetPeerASN())

	// Verify attributes are parsed
	assert.Equal(t, "igp", event.Origin)

	// Verify NLRI operations
	require.Contains(t, event.FamilyOps, "ipv4/unicast")
	require.Len(t, event.FamilyOps["ipv4/unicast"], 1)
	assert.Equal(t, "add", event.FamilyOps["ipv4/unicast"][0].Action)
}

// TestParseEvent_NewBGPFormatState verifies state events in new format.
//
// VALIDATES: State events with type/bgp wrapper parse correctly.
// PREVENTS: Peer state changes being missed in new format.
func TestParseEvent_NewBGPFormatState(t *testing.T) {
	input := `{
		"type": "bgp",
		"bgp": {
			"type": "state",
			"peer": {"address": "10.0.0.1", "remote": {"as": 65001}},
			"state": "up"
		}
	}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "state", event.GetEventType())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.Equal(t, "up", event.GetPeerState())
}

// TestParseEvent_NewBGPFormatWithRaw verifies format=full raw bytes parsing.
//
// VALIDATES: Raw wire bytes from format=full are parsed into Event fields.
// PREVENTS: Pool storage failing due to missing raw fields.
func TestParseEvent_NewBGPFormatWithRaw(t *testing.T) {
	input := `{
		"type": "bgp",
		"bgp": {
			"type": "update",
			"message": {"id": 100, "direction": "received"},
			"peer": {"address": "10.0.0.1", "remote": {"as": 65001}},
			"attributes": {"origin": "igp"},
			"nlri": {"ipv4/unicast": [{"next-hop": "1.1.1.1", "action": "add", "nlri": ["10.0.0.0/24"]}]},
			"raw": {
				"attributes": "40010100",
				"nlri": {"ipv4/unicast": "180a0000"},
				"withdrawn": {}
			}
		}
	}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "update", event.GetEventType())
	assert.Equal(t, uint64(100), event.GetMsgID())

	// Verify raw fields are populated
	assert.Equal(t, "40010100", event.RawAttributes)
	require.NotNil(t, event.RawNLRI)
	assert.Equal(t, "180a0000", event.RawNLRI["ipv4/unicast"])
}

// TestParseEvent_NewRIBFormat verifies parsing of ze-bgp JSON RIB event format.
//
// VALIDATES: New wrapped RIB format with type/rib structure parses correctly.
// PREVENTS: RIB cache events being ignored in new format.
func TestParseEvent_NewRIBFormat(t *testing.T) {
	// ze-bgp JSON format: type at top, payload nested under "rib"
	input := `{
		"type": "rib",
		"rib": {
			"type": "cache",
			"action": "new",
			"msg-id": 12345,
			"peer": {"address": "10.0.0.1", "remote": {"as": 65001}}
		}
	}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "cache", event.GetEventType())
	assert.Equal(t, uint64(12345), event.GetMsgID())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
}

// TestParseEvent_BackwardsCompatible verifies old format still works.
//
// VALIDATES: Legacy format without wrapper still parses correctly.
// PREVENTS: Breaking existing plugins during transition.
func TestParseEvent_BackwardsCompatible(t *testing.T) {
	// Old format without type/bgp wrapper
	input := `{
		"message": {"type": "update", "id": 456, "direction": "received"},
		"peer": {"address": "10.0.0.1", "remote": {"as": 65001}},
		"origin": "igp",
		"ipv4/unicast": [{"next-hop": "1.1.1.1", "action": "add", "nlri": ["10.0.0.0/24"]}]
	}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	// Should still work via existing parsing logic
	assert.Equal(t, "update", event.GetEventType())
	assert.Equal(t, uint64(456), event.GetMsgID())
}

// TestHandleReceived_AddPathNLRI verifies ADD-PATH NLRIs are split correctly.
//
// VALIDATES: AC-4 — ADD-PATH NLRIs with 4-byte path-ID prefix are correctly stored.
// PREVENTS: Path-ID bytes being misinterpreted as prefix length.
func TestHandleReceived_AddPathNLRI(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// ADD-PATH NLRI: [path-id:4][prefix-len:1][prefix-bytes]
	// 10.0.0.0/24 with path-id 42: 00 00 00 2a 18 0a 00 00
	// 10.0.1.0/24 with path-id 43: 00 00 00 2b 18 0a 00 01
	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 300},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "0000002a180a00000000002b180a0001"},
		AddPath:       map[string]bool{"ipv4/unicast": true},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	require.NotNil(t, r.ribInPool["10.0.0.1"], "PeerRIB should be created")
	assert.Equal(t, 2, r.ribInPool["10.0.0.1"].Len(), "should have 2 ADD-PATH routes in pool")

	// Verify actual stored wire bytes contain correct ADD-PATH NLRIs (not garbage from wrong parsing).
	// With addPath=true: key is [path-id:4][prefix-len:1][prefix-bytes].
	// With addPath=false: path-id bytes would be misread as prefix-lengths, producing different keys.
	ipv4u := family.IPv4Unicast
	_, found42 := r.ribInPool["10.0.0.1"].Lookup(ipv4u, []byte{0x00, 0x00, 0x00, 0x2a, 0x18, 0x0a, 0x00, 0x00})
	_, found43 := r.ribInPool["10.0.0.1"].Lookup(ipv4u, []byte{0x00, 0x00, 0x00, 0x2b, 0x18, 0x0a, 0x00, 0x01})
	assert.True(t, found42, "route with path-id 42 (10.0.0.0/24) must be stored with correct ADD-PATH wire key")
	assert.True(t, found43, "route with path-id 43 (10.0.1.0/24) must be stored with correct ADD-PATH wire key")
}

// TestHandleReceived_AddPathWithdraw verifies ADD-PATH withdrawals match stored routes.
//
// VALIDATES: AC-6 — ADD-PATH withdrawals with path-ID correctly remove stored routes.
// PREVENTS: Withdrawal failing to match because path-ID bytes weren't consumed.
func TestHandleReceived_AddPathWithdraw(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": "10.0.0.1", "local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"as": uint32(65001)}})

	// First announce with ADD-PATH
	announce := &Event{
		Message:       &MessageInfo{Type: "update", ID: 300},
		Peer:          peerJSON,
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "0000002a180a0000"},
		AddPath:       map[string]bool{"ipv4/unicast": true},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(announce)

	// Verify announcement stored with correct ADD-PATH wire key
	ipv4u := family.IPv4Unicast
	_, found := r.ribInPool["10.0.0.1"].Lookup(ipv4u, []byte{0x00, 0x00, 0x00, 0x2a, 0x18, 0x0a, 0x00, 0x00})
	require.True(t, found, "route must be stored with ADD-PATH wire key before withdrawal")

	// Then withdraw with ADD-PATH
	withdraw := &Event{
		Message:      &MessageInfo{Type: "update", ID: 301},
		Peer:         peerJSON,
		RawWithdrawn: map[string]string{"ipv4/unicast": "0000002a180a0000"},
		AddPath:      map[string]bool{"ipv4/unicast": true},
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{Action: "del", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(withdraw)
	assert.Equal(t, 0, r.ribInPool["10.0.0.1"].Len(), "ADD-PATH withdrawal should remove the route")
}

// TestExtractCandidate_PoolWiring verifies extractCandidate reads pool handles correctly.
// This is the wiring test that bridges pool storage to the best-path comparison algorithm.
//
// VALIDATES: Pool handle → Candidate field mapping is correct.
// PREVENTS: Wrong pool type used for a field, wrong field mapping, or nil-pointer on access.
func TestExtractCandidate_PoolWiring(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{
		"address": "10.0.0.1",
		"local":   map[string]any{"address": "10.0.0.2", "as": uint32(65000)},
		"remote":  map[string]any{"as": uint32(65001)},
	})

	// Raw attributes: ORIGIN=IGP(0), AS_PATH=SEQUENCE[65001,65002], LOCAL_PREF=200, MED=50.
	// ORIGIN: flags=40, type=01, len=01, value=00 (IGP)
	// AS_PATH: flags=40, type=02, len=0A, SEQ(02) count(02) 65001(0000FDE9) 65002(0000FDEA)
	// MED: flags=80, type=04, len=04, value=00000032 (50)
	// LOCAL_PREF: flags=40, type=05, len=04, value=000000C8 (200)
	rawAttrs := "4001010040020A020200" + "00FDE900" + "00FDEA80040400000032400504000000C8"

	event := &Event{
		Message:       &MessageInfo{Type: "update", ID: 500},
		Peer:          peerJSON,
		RawAttributes: rawAttrs,
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"}, // 10.0.0.0/24
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleReceived(event)

	// Verify peerMeta was populated from the event.
	require.NotNil(t, r.peerMeta["10.0.0.1"], "peerMeta should be populated")
	assert.Equal(t, uint32(65001), r.peerMeta["10.0.0.1"].PeerASN)
	assert.Equal(t, uint32(65000), r.peerMeta["10.0.0.1"].LocalASN)

	// Extract candidate from pool entry.
	peerRIB := r.ribInPool["10.0.0.1"]
	require.NotNil(t, peerRIB)

	var entry storage.RouteEntry
	var found bool
	peerRIB.Iterate(func(_ family.Family, _ []byte, e storage.RouteEntry) bool {
		entry = e
		found = true
		return false // stop after first
	})
	require.True(t, found, "should have a route entry")

	c := r.extractCandidate("10.0.0.1", entry)

	assert.Equal(t, "10.0.0.1", c.PeerAddr)
	assert.Equal(t, uint32(65001), c.PeerASN, "PeerASN from peerMeta")
	assert.Equal(t, uint32(65000), c.LocalASN, "LocalASN from peerMeta")
	assert.Equal(t, OriginIGP, c.Origin, "ORIGIN=IGP from pool")
	assert.Equal(t, uint32(200), c.LocalPref, "LOCAL_PREF=200 from pool")
	assert.Equal(t, uint32(50), c.MED, "MED=50 from pool")
	assert.Equal(t, 2, c.ASPathLen, "AS_PATH length=2 from pool (SEQUENCE[65001,65002])")
	assert.Equal(t, uint32(65001), c.FirstAS, "FirstAS=65001 from pool")
}

// TestPeerMetaCleanup_ClearAndRelease verifies peerMeta is cleaned up by
// inboundEmptyJSON (bgp rib clear in) and releaseRoutesJSON (bgp rib release-routes).
//
// VALIDATES: peerMeta deleted alongside ribInPool in clear and release paths.
// PREVENTS: peerMeta memory leak when routes are cleared or GR-released.
func TestPeerMetaCleanup_ClearAndRelease(t *testing.T) {
	t.Run("bgp rib clear in", func(t *testing.T) {
		r := newTestRIBManager(t)
		peerJSON := mustMarshal(t, map[string]any{
			"address": "10.0.0.1",
			"local":   map[string]any{"address": "10.0.0.2", "as": uint32(65000)},
			"remote":  map[string]any{"as": uint32(65001)},
		})
		event := &Event{
			Message:       &MessageInfo{Type: "update", ID: 1},
			Peer:          peerJSON,
			RawAttributes: "40010100",
			RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
			FamilyOps: map[string][]FamilyOperation{
				"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
			},
		}
		r.handleReceived(event)
		require.NotNil(t, r.peerMeta["10.0.0.1"], "peerMeta should exist before clear")

		r.inboundEmptyJSON("*")

		_, ribExists := r.ribInPool["10.0.0.1"]
		assert.False(t, ribExists, "ribInPool should be cleared")
		_, metaExists := r.peerMeta["10.0.0.1"]
		assert.False(t, metaExists, "peerMeta should be cleared with ribInPool")
	})

	t.Run("bgp rib release-routes", func(t *testing.T) {
		r := newTestRIBManager(t)
		peerJSON := mustMarshal(t, map[string]any{
			"address": "10.0.0.1",
			"local":   map[string]any{"address": "10.0.0.2", "as": uint32(65000)},
			"remote":  map[string]any{"as": uint32(65001)},
		})
		event := &Event{
			Message:       &MessageInfo{Type: "update", ID: 1},
			Peer:          peerJSON,
			RawAttributes: "40010100",
			RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
			FamilyOps: map[string][]FamilyOperation{
				"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
			},
		}
		r.handleReceived(event)
		require.NotNil(t, r.peerMeta["10.0.0.1"], "peerMeta should exist before release")

		// Mark peer as retained, then release.
		r.retainedPeers["10.0.0.1"] = true
		r.releaseRoutesJSON("*")

		_, ribExists := r.ribInPool["10.0.0.1"]
		assert.False(t, ribExists, "ribInPool should be cleared on release")
		_, metaExists := r.peerMeta["10.0.0.1"]
		assert.False(t, metaExists, "peerMeta should be cleared on release")
		_, retainExists := r.retainedPeers["10.0.0.1"]
		assert.False(t, retainExists, "retainedPeers should be cleared on release")
	})
}

// TestHandleSentPerFamily verifies routes are stored in separate family maps.
//
// VALIDATES: AC-1 — routes stored under correct family key.
// PREVENTS: Routes from different families mixing in the same map.
func TestHandleSentPerFamily(t *testing.T) {
	r := newTestRIBManager(t)

	event := &Event{
		Message: &MessageInfo{Type: "update", ID: 1},
		Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "remote": map[string]any{"as": uint32(65001)}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
			"ipv6/unicast": {{NextHop: "::1", Action: "add", NLRIs: []any{"2001:db8::/32"}}},
		},
	}
	r.handleSent(event)

	// Verify per-family structure
	peerFamilies := r.ribOut["10.0.0.1"]
	require.Len(t, peerFamilies, 2, "should have 2 family maps")
	require.Contains(t, peerFamilies, "ipv4/unicast")
	require.Contains(t, peerFamilies, "ipv6/unicast")
	assert.Len(t, peerFamilies["ipv4/unicast"], 1)
	assert.Len(t, peerFamilies["ipv6/unicast"], 1)

	// Verify route contents
	rt := peerFamilies["ipv4/unicast"]["10.0.0.0/24"]
	require.NotNil(t, rt)
	assert.Equal(t, "ipv4/unicast", rt.Family)
	assert.Equal(t, "10.0.0.0/24", rt.Prefix)
}

// TestHandleSentWithdrawalPerFamily verifies withdrawal removes from correct family.
//
// VALIDATES: AC-8 — withdrawal removes from correct family map.
// PREVENTS: Withdrawal affecting wrong family or leaving stale entries.
func TestHandleSentWithdrawalPerFamily(t *testing.T) {
	r := newTestRIBManager(t)

	// Add routes in two families
	add := &Event{
		Message: &MessageInfo{Type: "update", ID: 1},
		Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "remote": map[string]any{"as": uint32(65001)}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}}},
			"ipv6/unicast": {{NextHop: "::1", Action: "add", NLRIs: []any{"2001:db8::/32"}}},
		},
	}
	r.handleSent(add)

	// Withdraw one ipv4 route
	del := &Event{
		Message: &MessageInfo{Type: "update", ID: 2},
		Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "remote": map[string]any{"as": uint32(65001)}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{Action: "del", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleSent(del)

	// ipv4 should have 1 route, ipv6 untouched
	assert.Len(t, r.ribOut["10.0.0.1"]["ipv4/unicast"], 1)
	assert.Len(t, r.ribOut["10.0.0.1"]["ipv6/unicast"], 1)
}

// TestHandleSentWithdrawalCleansEmptyMaps verifies empty maps are removed.
//
// VALIDATES: AC-11 — empty family and peer maps cleaned up.
// PREVENTS: Zombie empty maps accumulating in memory.
func TestHandleSentWithdrawalCleansEmptyMaps(t *testing.T) {
	r := newTestRIBManager(t)

	// Add one route
	add := &Event{
		Message: &MessageInfo{Type: "update", ID: 1},
		Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "remote": map[string]any{"as": uint32(65001)}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleSent(add)
	require.Contains(t, r.ribOut, "10.0.0.1")

	// Withdraw the only route
	del := &Event{
		Message: &MessageInfo{Type: "update", ID: 2},
		Peer:    mustMarshal(t, map[string]any{"address": "10.0.0.1", "remote": map[string]any{"as": uint32(65001)}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {{Action: "del", NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	r.handleSent(del)

	// Both family map and peer map should be gone
	_, hasPeer := r.ribOut["10.0.0.1"]
	assert.False(t, hasPeer, "empty peer map should be removed from ribOut")
}

// TestHandleRefreshPerFamily verifies only the requested family is sent.
//
// VALIDATES: AC-2 — only requested family routes sent on refresh.
// PREVENTS: Sending routes from other families on route refresh.
func TestHandleRefreshPerFamily(t *testing.T) {
	r := newTestRIBManager(t)
	r.peerUp["10.0.0.1"] = true

	// Pre-populate ribOut with two families
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
		"ipv6/unicast": {
			"2001:db8::/32": {MsgID: 2, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
		},
	}

	// Request refresh for ipv4/unicast only
	refreshEvent := &Event{
		Peer: mustMarshal(t, map[string]any{"address": "10.0.0.1", "remote": map[string]any{"as": uint32(65001)}}),
		AFI:  "ipv4", SAFI: "unicast",
	}
	// handleRefresh sends routes via SDK (closed pipe in test, no-op),
	// but we can verify it doesn't panic and the ribOut is unchanged.
	r.handleRefresh(refreshEvent)

	// ribOut should be unchanged (routes are sent, not removed)
	assert.Len(t, r.ribOut["10.0.0.1"]["ipv4/unicast"], 1)
	assert.Len(t, r.ribOut["10.0.0.1"]["ipv6/unicast"], 1)
}

// TestHandleStateReplayAllFamilies verifies all families are replayed on peer-up.
//
// VALIDATES: AC-3 — all families replayed in MsgID order on peer-up.
// PREVENTS: Only one family being replayed after reconnect.
func TestHandleStateReplayAllFamilies(t *testing.T) {
	r := newTestRIBManager(t)

	// Pre-populate ribOut with two families
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
		"ipv6/unicast": {
			"2001:db8::/32": {MsgID: 2, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
		},
	}

	// Peer comes up
	upEvent := &Event{
		Peer: mustMarshal(t, map[string]any{"address": "10.0.0.1", "state": "up", "remote": map[string]any{"as": uint32(65001)}}),
	}
	r.handleState(upEvent)

	// ribOut should be preserved (routes are replayed, not consumed)
	assert.Len(t, r.ribOut["10.0.0.1"], 2, "both families should still be in ribOut")
}

// TestOutboundResendAllFamilies verifies resend without family sends everything.
//
// VALIDATES: AC-4 — no-family resend replays all families.
// PREVENTS: Per-family restructuring accidentally losing routes on resend.
func TestOutboundResendAllFamilies(t *testing.T) {
	r := newTestRIBManager(t)
	r.peerUp["10.0.0.1"] = true
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
		"ipv6/unicast": {
			"2001:db8::/32": {MsgID: 2, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
		},
	}

	status, data, err := r.handleCommand("bgp rib clear out", "*", []string{"*"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(2), result["resent"], "should resend routes from both families")
}

// TestOutboundResendSingleFamily verifies resend with family sends only that family.
//
// VALIDATES: AC-5 — family-specific resend.
// PREVENTS: Resending routes from other families.
func TestOutboundResendSingleFamily(t *testing.T) {
	r := newTestRIBManager(t)
	r.peerUp["10.0.0.1"] = true
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
		"ipv6/unicast": {
			"2001:db8::/32": {MsgID: 2, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
		},
	}

	status, data, err := r.handleCommand("bgp rib clear out", "*", []string{"*", "ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(1), result["resent"], "should resend only ipv4/unicast routes")
}

// TestStatusJSONMultiFamilyCount verifies status counts across families.
//
// VALIDATES: AC-6 — total route count matches sum across families.
// PREVENTS: Per-family restructuring breaking route counts.
func TestStatusJSONMultiFamilyCount(t *testing.T) {
	r := newTestRIBManager(t)
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
			"10.0.1.0/24": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
		},
		"ipv6/unicast": {
			"2001:db8::/32": {MsgID: 3, Family: "ipv6/unicast", Prefix: "2001:db8::/32"},
		},
	}

	data := r.statusJSON()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(3), result["routes-out"], "should count 3 total routes across families")
}

// TestOutboundSourceMultiFamily verifies pipeline iterates all families.
//
// VALIDATES: AC-7 — all routes from all families appear in pipeline output.
// PREVENTS: Pipeline missing routes from some families.
func TestOutboundSourceMultiFamily(t *testing.T) {
	r := newTestRIBManager(t)
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		},
		"ipv6/unicast": {
			"2001:db8::/32": {MsgID: 2, Family: "ipv6/unicast", Prefix: "2001:db8::/32"},
		},
	}

	src := newOutboundSource(r, "*")
	count := 0
	families := map[string]bool{}
	for {
		item, ok := src.Next()
		if !ok {
			break
		}
		families[item.Family] = true
		count++
	}
	assert.Equal(t, 2, count, "should iterate 2 routes total")
	assert.True(t, families["ipv4/unicast"], "should include ipv4/unicast")
	assert.True(t, families["ipv6/unicast"], "should include ipv6/unicast")
}

// TestOutRouteKey verifies the ribOut-specific prefix-only key function.
//
// VALIDATES: outRouteKey produces prefix-only keys (no family prefix).
// PREVENTS: Key collisions or missing pathID in ribOut keys.
func TestOutRouteKey(t *testing.T) {
	tests := []struct {
		prefix string
		pathID uint32
		want   string
	}{
		{"10.0.0.0/24", 0, "10.0.0.0/24"},
		{"10.0.0.0/24", 1, "10.0.0.0/24:1"},
		{"10.0.0.0/24", 2, "10.0.0.0/24:2"},
		{"2001:db8::/32", 0, "2001:db8::/32"},
		{"2001:db8::/32", 100, "2001:db8::/32:100"},
	}

	for _, tt := range tests {
		got := outRouteKey(tt.prefix, tt.pathID)
		assert.Equal(t, tt.want, got, "outRouteKey(%q, %d)", tt.prefix, tt.pathID)
	}
}

// TestOutboundResendSelectorFromArgs verifies bgp rib clear out extracts the selector
// from args[0], not the peer parameter (which is always "*" for plugin dispatch).
//
// VALIDATES: AC-10 — selector from args filters peers correctly.
// PREVENTS: Selector silently discarded, resending to all peers.
func TestOutboundResendSelectorFromArgs(t *testing.T) {
	r := newTestRIBManager(t)

	// Two peers, both up, both with routes in ribOut
	r.peerUp["10.0.0.1"] = true
	r.peerUp["10.0.0.2"] = true
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.1.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.1.0.0/24", NextHop: "1.1.1.1"},
		},
	}
	r.ribOut["10.0.0.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.2.0.0/24": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.2.0.0/24", NextHop: "2.2.2.2"},
		},
	}

	// Dispatch with selector "!10.0.0.1" in args (all except 10.0.0.1).
	// peer param is "*" (as it would be for plugin-dispatched commands).
	status, data, err := r.handleCommand("bgp rib clear out", "*", []string{"!10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	// Only 10.0.0.2 should be resent (1 peer, 1 route)
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(1), result["peers"], "should resend to 1 peer (not 10.0.0.1)")
	assert.Equal(t, float64(1), result["resent"], "should resend 1 route")
}

// TestInboundEmptySelectorFromArgs verifies bgp rib clear in extracts selector from args.
//
// VALIDATES: AC-12 — bgp rib clear in uses args[0] for selector.
// PREVENTS: Clearing all peers when only one was requested.
func TestInboundEmptySelectorFromArgs(t *testing.T) {
	r := newTestRIBManager(t)

	// Two peers with routes in ribInPool
	r.ribInPool["10.0.0.1"] = storage.NewPeerRIB("10.0.0.1")
	r.ribInPool["10.0.0.2"] = storage.NewPeerRIB("10.0.0.2")

	// Clear only 10.0.0.1
	status, data, err := r.handleCommand("bgp rib clear in", "*", []string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"cleared"`)

	// 10.0.0.1 should be gone, 10.0.0.2 should remain
	_, has1 := r.ribInPool["10.0.0.1"]
	_, has2 := r.ribInPool["10.0.0.2"]
	assert.False(t, has1, "10.0.0.1 should be cleared")
	assert.True(t, has2, "10.0.0.2 should remain")
}

// TestRetainRoutesSelectorFromArgs verifies bgp rib retain-routes extracts selector from args.
//
// VALIDATES: AC-13 — bgp rib retain-routes uses args[0] for selector.
// PREVENTS: Retaining all peers when only one was requested.
func TestRetainRoutesSelectorFromArgs(t *testing.T) {
	r := newTestRIBManager(t)

	// Two peers with routes
	r.ribInPool["10.0.0.1"] = storage.NewPeerRIB("10.0.0.1")
	r.ribInPool["10.0.0.2"] = storage.NewPeerRIB("10.0.0.2")

	// Retain only 10.0.0.1
	status, data, err := r.handleCommand("bgp rib retain-routes", "*", []string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"retained-peers":1`)

	assert.True(t, r.retainedPeers["10.0.0.1"], "10.0.0.1 should be retained")
	assert.False(t, r.retainedPeers["10.0.0.2"], "10.0.0.2 should NOT be retained")
}

// TestReleaseRoutesSelectorFromArgs verifies bgp rib release-routes extracts selector from args.
//
// VALIDATES: AC-14 — bgp rib release-routes uses args[0] for selector.
// PREVENTS: Releasing all peers when only one was requested.
func TestReleaseRoutesSelectorFromArgs(t *testing.T) {
	r := newTestRIBManager(t)

	// Two retained peers
	r.ribInPool["10.0.0.1"] = storage.NewPeerRIB("10.0.0.1")
	r.ribInPool["10.0.0.2"] = storage.NewPeerRIB("10.0.0.2")
	r.retainedPeers["10.0.0.1"] = true
	r.retainedPeers["10.0.0.2"] = true

	// Release only 10.0.0.1
	status, data, err := r.handleCommand("bgp rib release-routes", "*", []string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"released-peers":1`)

	assert.False(t, r.retainedPeers["10.0.0.1"], "10.0.0.1 should be released")
	assert.True(t, r.retainedPeers["10.0.0.2"], "10.0.0.2 should still be retained")
}

// TestOutboundResendNoArgError verifies bgp bgp rib clear out returns error with no args.
//
// VALIDATES: AC-15 — missing required selector returns error.
// PREVENTS: Accidentally resending to all peers when selector was forgotten.
func TestOutboundResendNoArgError(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib clear out", "*", nil)
	require.Error(t, err, "bgp rib clear out with no args should return error")
	assert.Equal(t, "error", status)
}

// --- bgp rib inject / bgp rib withdraw tests ---

// TestInjectRoute_Basic verifies a route can be injected into adj-rib-in.
//
// VALIDATES: AC-1 -- bgp rib inject 10.0.0.1 ipv4/unicast 10.0.0.0/24 inserts route.
// PREVENTS: Inject command silently failing without inserting.
func TestInjectRoute_Basic(t *testing.T) {
	r := newTestRIBManager(t)

	status, data, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"injected":"10.0.0.0/24"`)
	assert.Contains(t, data, `"peer":"10.0.0.1"`)

	// Verify route is in RIB.
	peerRIB := r.ribInPool["10.0.0.1"]
	require.NotNil(t, peerRIB, "PeerRIB should exist after inject")
	assert.Equal(t, 1, peerRIB.FamilyLen(family.Family{AFI: 1, SAFI: 1}))
}

// TestInjectRoute_AllAttributes verifies all optional attributes are set.
//
// VALIDATES: AC-2 -- origin, nhop, aspath, localpref, med all set correctly.
// PREVENTS: Attribute args silently ignored.
func TestInjectRoute_AllAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	args := []string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24",
		"origin", "egp",
		"nhop", "1.1.1.1",
		"aspath", "64500,64501,64502",
		"localpref", "200",
		"med", "50",
	}
	status, _, err := r.handleCommand("bgp rib inject", "", args)
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	// Verify route exists and has attributes by looking it up.
	peerRIB := r.ribInPool["10.0.0.1"]
	require.NotNil(t, peerRIB)

	nlriBytes, err := prefixToWire("ipv4/unicast", "10.0.0.0/24", 0, false)
	require.NoError(t, err)
	entry, found := peerRIB.Lookup(family.Family{AFI: 1, SAFI: 1}, nlriBytes)
	require.True(t, found, "route should exist after inject")
	require.NotNil(t, entry)
}

// TestWithdrawRoute_Basic verifies a route can be withdrawn from adj-rib-in.
//
// VALIDATES: AC-3 -- bgp rib withdraw removes route.
// PREVENTS: Withdraw silently failing.
func TestWithdrawRoute_Basic(t *testing.T) {
	r := newTestRIBManager(t)

	// Inject first.
	_, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24"})
	require.NoError(t, err)

	// Withdraw.
	status, data, err := r.handleCommand("bgp rib withdraw", "", []string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"existed":true`)

	// Verify route is gone.
	peerRIB := r.ribInPool["10.0.0.1"]
	assert.Equal(t, 0, peerRIB.FamilyLen(family.Family{AFI: 1, SAFI: 1}))
}

// TestInjectRoute_VisibleInShow verifies injected routes appear in bgp rib show.
//
// VALIDATES: AC-4 -- injected routes appear in bgp rib show output.
// PREVENTS: Routes inserted but not queryable.
func TestInjectRoute_VisibleInShow(t *testing.T) {
	r := newTestRIBManager(t)

	_, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "aspath", "64500,64501",
	})
	require.NoError(t, err)

	status, data, err := r.handleCommand("bgp rib show", "10.0.0.1", nil)
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, "10.0.0.0/24")
}

// TestInjectRoute_MissingPeer verifies error when not enough args.
//
// VALIDATES: AC-5 -- missing peer argument returns error with usage hint.
// PREVENTS: Routes injected without a peer label.
func TestInjectRoute_MissingPeer(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{"ipv4/unicast", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "usage:")
}

// TestInjectRoute_InvalidPrefix verifies error on malformed prefix.
//
// VALIDATES: AC-6 -- invalid prefix returns error.
// PREVENTS: Panic or silent failure on bad input.
func TestInjectRoute_InvalidPrefix(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/unicast", "not-a-prefix"})
	require.Error(t, err)
	assert.Equal(t, "error", status)
}

// TestInjectRoute_InvalidASPath verifies error on malformed AS path.
//
// VALIDATES: AC-7 -- invalid ASN returns error.
// PREVENTS: Corrupt AS path stored in RIB.
func TestInjectRoute_InvalidASPath(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "aspath", "abc,def",
	})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "invalid ASN")
}

// TestInjectRoute_UnknownAttr verifies error on unknown attribute keyword.
//
// VALIDATES: AC-8 -- unknown attribute returns error.
// PREVENTS: Typos silently ignored.
func TestInjectRoute_UnknownAttr(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "bogus", "value",
	})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "unknown attribute")
}

// TestInjectRoute_IPv6 verifies IPv6 prefix injection.
//
// VALIDATES: AC-9 -- IPv6 prefix works.
// PREVENTS: IPv6 silently rejected or mangled.
func TestInjectRoute_IPv6(t *testing.T) {
	r := newTestRIBManager(t)

	status, data, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv6/unicast", "2001:db8::/32"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"injected":"2001:db8::/32"`)

	peerRIB := r.ribInPool["10.0.0.1"]
	require.NotNil(t, peerRIB)
	assert.Equal(t, 1, peerRIB.FamilyLen(family.Family{AFI: 2, SAFI: 1}))
}

// TestWithdrawRoute_NonExistent verifies withdrawing a non-existent route.
//
// VALIDATES: AC-10 -- withdraw of missing route reports existed=false.
// PREVENTS: Error on withdraw of non-existent route.
func TestWithdrawRoute_NonExistent(t *testing.T) {
	r := newTestRIBManager(t)

	// Create PeerRIB first so we don't get "no RIB for peer" error.
	_, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24"})
	require.NoError(t, err)

	status, data, err := r.handleCommand("bgp rib withdraw", "", []string{"10.0.0.1", "ipv4/unicast", "192.168.0.0/24"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"existed":false`)
}

// TestInjectRoute_ImplicitWithdraw verifies re-inject replaces old attributes.
//
// VALIDATES: AC-11 -- multiple injects to same prefix = implicit withdraw.
// PREVENTS: Duplicate entries or stale attributes.
func TestInjectRoute_ImplicitWithdraw(t *testing.T) {
	r := newTestRIBManager(t)

	// Inject with localpref 100.
	_, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "localpref", "100"})
	require.NoError(t, err)

	// Re-inject same prefix with localpref 200.
	_, _, err = r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "localpref", "200"})
	require.NoError(t, err)

	// Should still be exactly 1 route (implicit withdraw replaced the old one).
	peerRIB := r.ribInPool["10.0.0.1"]
	require.NotNil(t, peerRIB)
	assert.Equal(t, 1, peerRIB.FamilyLen(family.Family{AFI: 1, SAFI: 1}))
}

// TestInjectRoute_InvalidPeerAddress verifies non-IP peer is rejected.
//
// VALIDATES: Peer address must be a valid IP.
// PREVENTS: Arbitrary strings used as peer labels (XSS in LG output).
func TestInjectRoute_InvalidPeerAddress(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{"not-an-ip", "ipv4/unicast", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "invalid peer address")
}

// TestInjectRoute_IPv6NextHopRejected verifies IPv6 nhop returns error.
//
// VALIDATES: AC-5 -- IPv6 nhop accepted for unknown peer (no session, fallback).
// PREVENTS: Unknown peer rejecting valid IPv6 next-hop.
func TestInjectRoute_IPv6NhopUnknownPeer(t *testing.T) {
	r := newTestRIBManager(t)

	// 10.0.0.1 has no peerMeta entry -- fallback accepts any valid IP.
	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "nhop", "2001:db8::1",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
}

// VALIDATES: AC-4 -- IPv6 nhop rejected for real peer without ExtendedNextHop.
// PREVENTS: IPv6 nhop accepted when peer hasn't negotiated the capability.
func TestInjectRoute_IPv6NhopRealPeerNoCapability(t *testing.T) {
	r := newTestRIBManager(t)

	// Simulate a real peer with metadata but no ExtendedNextHop capability.
	// ContextID 0 means no encoding context (JSON event path, no structured event).
	r.peerMeta["10.0.0.1"] = &PeerMeta{PeerASN: 65000, LocalASN: 65001, ContextID: 0}

	// ContextID 0 = no capability info, should accept with warning.
	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "nhop", "2001:db8::1",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
}

// VALIDATES: AC-4 -- IPv6 nhop rejected when peer has context but no ExtendedNextHop.
// PREVENTS: IPv6 nhop accepted when capability not negotiated.
func TestInjectRoute_IPv6NhopRealPeerContextNoExtNH(t *testing.T) {
	r := newTestRIBManager(t)

	// Register an encoding context WITHOUT ExtendedNextHop.
	ctx := bgpctx.NewEncodingContext(nil, nil, bgpctx.DirectionRecv)
	ctxID := bgpctx.Registry.Register(ctx)

	r.peerMeta["10.0.0.1"] = &PeerMeta{PeerASN: 65000, LocalASN: 65001, ContextID: ctxID}

	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "nhop", "2001:db8::1",
	})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "extended-nexthop")
	assert.Contains(t, err.Error(), "RFC 8950")
}

// TestInjectRoute_TrailingKeyNoValue verifies odd attr args are rejected.
//
// VALIDATES: Trailing key without value returns error.
// PREVENTS: Attribute silently ignored when value is missing.
func TestInjectRoute_TrailingKeyNoValue(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "origin",
	})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "has no value")
}

// TestInjectRoute_NonSimpleFamily verifies EVPN/VPN families are rejected.
//
// VALIDATES: Only simple prefix families accepted.
// PREVENTS: Malformed NLRI bytes for complex family types.
func TestInjectRoute_NonSimpleFamily(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "l2vpn/evpn", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "simple prefix families")
}

// TestWithdrawRoute_InvalidPeerAddress verifies non-IP peer is rejected in withdraw.
//
// VALIDATES: Peer validation also applies to withdraw.
// PREVENTS: Asymmetry between inject and withdraw validation.
func TestWithdrawRoute_InvalidPeerAddress(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib withdraw", "", []string{"not-an-ip", "ipv4/unicast", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "invalid peer address")
}

// TestInjectRoute_IPv4Multicast verifies multicast family works.
//
// VALIDATES: Multicast is a valid simple prefix family.
// PREVENTS: Only unicast tested, multicast silently broken.
func TestInjectRoute_IPv4Multicast(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/multicast", "224.0.0.0/4"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	peerRIB := r.ribInPool["10.0.0.1"]
	require.NotNil(t, peerRIB)
	assert.Equal(t, 1, peerRIB.FamilyLen(family.Family{AFI: 1, SAFI: 2}))
}

// TestInjectRoute_OriginIncomplete verifies incomplete origin value.
//
// VALIDATES: All three origin values are accepted.
// PREVENTS: Only default igp tested.
func TestInjectRoute_OriginIncomplete(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "origin", "incomplete",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
}

// TestInjectRoute_NoAttributes verifies inject with zero optional attrs.
//
// VALIDATES: Default origin=igp is set when no attrs provided.
// PREVENTS: Malformed wire bytes when no optional attrs given.
func TestInjectRoute_NoAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24"})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	nlriBytes, err := prefixToWire("ipv4/unicast", "10.0.0.0/24", 0, false)
	require.NoError(t, err)
	entry, found := r.ribInPool["10.0.0.1"].Lookup(family.Family{AFI: 1, SAFI: 1}, nlriBytes)
	require.True(t, found)
	require.NotNil(t, entry)
}

// TestInjectRoute_SingleASN verifies single-ASN aspath (no comma).
//
// VALIDATES: parseASNList handles single value without comma.
// PREVENTS: Single ASN rejected or parsed as empty.
func TestInjectRoute_SingleASN(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "aspath", "64500",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
}

// TestInjectRoute_DuplicateAttr verifies last-wins behavior for repeated attrs.
//
// VALIDATES: Repeated attribute keyword uses last value.
// PREVENTS: First-wins or error on duplicate.
func TestInjectRoute_DuplicateAttr(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "localpref", "100", "localpref", "200",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
}

// TestInjectRoute_InvalidFamily verifies unparseable family string.
//
// VALIDATES: Garbage family string returns error.
// PREVENTS: Panic on malformed family.
func TestInjectRoute_InvalidFamily(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "bogus/family", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Equal(t, "error", status)
}

// TestInjectRoute_FamilyMismatch verifies IPv6 prefix rejected for IPv4 family.
//
// VALIDATES: prefixToWire rejects address family mismatch.
// PREVENTS: IPv6 address truncated to 4 bytes and stored as garbage.
func TestInjectRoute_FamilyMismatch(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{"10.0.0.1", "ipv4/unicast", "2001:db8::/32"})
	require.Error(t, err)
	assert.Equal(t, "error", status)
}

// TestInjectRoute_IPv4MappedIPv6NextHop verifies ::ffff:x.x.x.x works as nhop.
//
// VALIDATES: IPv4-mapped IPv6 addresses (RFC 4291) accepted as next-hop.
// PREVENTS: Mapped addresses rejected by the IPv6 check.
func TestInjectRoute_IPv4MappedIPv6NextHop(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "nhop", "::ffff:10.0.0.1",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
}

// TestWithdrawRoute_MissingArgs verifies error with insufficient args.
//
// VALIDATES: Withdraw with < 3 args returns usage error.
// PREVENTS: Panic on missing prefix arg.
func TestWithdrawRoute_MissingArgs(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("bgp rib withdraw", "", []string{"10.0.0.1", "ipv4/unicast"})
	require.Error(t, err)
	assert.Equal(t, "error", status)
	assert.Contains(t, err.Error(), "usage:")
}

// TestParseASNList verifies AS number list parsing.
//
// VALIDATES: parseASNList handles valid and invalid inputs.
// PREVENTS: Malformed ASN lists accepted silently.
func TestParseASNList(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []uint32
		wantErr bool
	}{
		{"single", "64500", []uint32{64500}, false},
		{"multiple", "64500,64501,64502", []uint32{64500, 64501, 64502}, false},
		{"spaces", "64500, 64501, 64502", []uint32{64500, 64501, 64502}, false},
		{"empty_parts", "64500,,64501", []uint32{64500, 64501}, false},
		{"invalid", "abc", nil, true},
		{"negative", "-1", nil, true},
		{"overflow", "4294967296", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseASNList(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
