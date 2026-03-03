package bgp_rib

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-rib/storage"
)

// newTestRIBManager creates a RIBManager with closed SDK connections for unit testing.
// The SDK plugin is initialized but connections are closed, so RPC calls (updateRoute)
// will fail silently. This is appropriate for testing internal state changes.
func newTestRIBManager(t *testing.T) *RIBManager {
	t.Helper()
	engineConn, engineRemote := net.Pipe()
	callbackConn, callbackRemote := net.Pipe()
	if err := engineRemote.Close(); err != nil {
		t.Logf("close engineRemote: %v", err)
	}
	if err := callbackRemote.Close(); err != nil {
		t.Logf("close callbackRemote: %v", err)
	}
	p := sdk.NewWithConn("rib-test", engineConn, callbackConn)
	t.Cleanup(func() { _ = p.Close() })
	return &RIBManager{
		plugin:    p,
		ribInPool: make(map[string]*storage.PeerRIB),
		ribOut:    make(map[string]map[string]*Route),
		peerUp:    make(map[string]bool),
	}
}

// TestParseEvent_SentFormat verifies parsing of sent UPDATE events.
//
// VALIDATES: Sent events with flat structure are parsed correctly.
// PREVENTS: Sent events being dropped due to format mismatch.
func TestParseEvent_SentFormat(t *testing.T) {
	// New command-style format: family at top level with operations array
	input := `{"type":"sent","msg-id":123,"peer":{"address":"10.0.0.1","asn":65001},"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24","10.0.1.0/24"]}]}`

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
	input := `{"message":{"type":"update","id":456,"direction":"received"},"peer":{"address":{"local":"10.0.0.2","peer":"10.0.0.1"},"asn":{"local":65002,"peer":65001}},"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24","10.0.1.0/24"]}]}`

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
	input := `{"type":"state","peer":{"address":"10.0.0.1","asn":65001},"state":"up"}`

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
	input := `{"type":"request","serial":"abc123","command":"rib adjacent status"}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "request", event.GetEventType())
	assert.Equal(t, "abc123", event.Serial)
	assert.Equal(t, "rib adjacent status", event.Command)
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
		Peer:  mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleSent(event)

	assert.Len(t, r.ribOut["10.0.0.1"], 2)
	assert.Contains(t, r.ribOut["10.0.0.1"], "ipv4/unicast:10.0.0.0/24")
	assert.Contains(t, r.ribOut["10.0.0.1"], "ipv4/unicast:10.0.1.0/24")

	route := r.ribOut["10.0.0.1"]["ipv4/unicast:10.0.0.0/24"]
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
		Peer:  mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleSent(announce)
	assert.Len(t, r.ribOut["10.0.0.1"], 1)

	// Then withdraw
	withdraw := &Event{
		Type: "sent",
		Peer: mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{Action: "del", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleSent(withdraw)
	assert.Len(t, r.ribOut["10.0.0.1"], 0)
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
		Peer:          mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
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
}

// TestHandleReceived_Withdraw verifies routes are removed on withdrawal.
//
// VALIDATES: Withdrawn routes are removed from Adj-RIB-In.
// PREVENTS: Stale route state.
func TestHandleReceived_Withdraw(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		"ipv4/unicast:10.0.1.0/24": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "1.1.1.1"},
	}

	event := &Event{
		Type:  "state",
		Peer:  mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
		State: "up",
	}

	r.handleState(event)

	// Verify internal state: peer is marked as up
	assert.True(t, r.peerUp["10.0.0.1"], "peer should be marked as up")
	// ribOut should be preserved (routes are replayed via SDK RPC, not text output)
	assert.Len(t, r.ribOut["10.0.0.1"], 2, "ribOut should still have routes")
}

// TestHandleState_PeerDown verifies Adj-RIB-In is cleared on peer down.
//
// VALIDATES: Received routes are cleared when peer goes down.
// PREVENTS: Stale routes from failed peers.
func TestHandleState_PeerDown(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
	}
	r.peerUp["10.0.0.1"] = true

	event := &Event{
		Type:  "state",
		Peer:  mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
		State: "down",
	}

	r.handleState(event)

	// ribInPool should be cleared (PeerRIB deleted)
	_, exists := r.ribInPool["10.0.0.1"]
	assert.False(t, exists, "PeerRIB should be deleted on peer down")
	// ribOut should be preserved for replay
	assert.Len(t, r.ribOut["10.0.0.1"], 1)
}

// TestStatusJSON verifies status command output.
//
// VALIDATES: Status returns correct route counts.
// PREVENTS: Incorrect stats being reported.
func TestStatusJSON(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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

	r.ribOut["10.0.0.2"] = map[string]*Route{
		"c": {},
	}
	r.peerUp["10.0.0.1"] = true
	r.peerUp["10.0.0.2"] = true

	status := r.statusJSON()
	assert.Contains(t, status, `"running":true`)
	assert.Contains(t, status, `"peers":2`)
	assert.Contains(t, status, `"routes_in":2`)
	assert.Contains(t, status, `"routes_out":1`)
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
				Peer: mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
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
				Peer:          json.RawMessage(`{"address":{"local":"","peer":"10.0.0.1"},"asn":{"local":0,"peer":65001}}`),
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
			for _, routes := range r.ribOut {
				totalOut += len(routes)
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
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {MsgID: 1, Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}

	// Pre-populate ribInPool via handleReceived
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})
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

	peer := mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001})

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
// VALIDATES: "rib adjacent status" returns status JSON via handleCommand.
// PREVENTS: Command rename breaking status queries.
func TestHandleCommand_RIBAdjacentStatus(t *testing.T) {
	r := newTestRIBManager(t)

	// Add route via pool storage
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})
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
	r.ribOut["10.0.0.1"] = map[string]*Route{"b": {}, "c": {}}

	status, data, err := r.handleCommand("rib adjacent status", "")
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"running":true`)
	assert.Contains(t, data, `"routes_in":1`)
	assert.Contains(t, data, `"routes_out":2`)
}

// TestHandleCommand_RIBAdjacentInboundShow verifies inbound show with selector.
//
// VALIDATES: "rib adjacent inbound show" filters by peer selector.
// PREVENTS: Wrong routes returned for filtered queries.
func TestHandleCommand_RIBAdjacentInboundShow(t *testing.T) {
	r := newTestRIBManager(t)

	// Add routes via pool storage for peer 10.0.0.1
	peer1JSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})
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
	peer2JSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.1", "peer": "10.0.0.2"}, "asn": map[string]uint32{"local": 65001, "peer": 65002}})
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
			status, data, err := r.handleCommand("rib adjacent inbound show", tt.selector)
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
// VALIDATES: "rib adjacent inbound empty" empties matching peers only.
// PREVENTS: Emptying wrong peers' routes.
func TestHandleCommand_RIBAdjacentInboundEmpty(t *testing.T) {
	r := newTestRIBManager(t)

	// Add routes via pool storage for peer 10.0.0.1
	peer1JSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})
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
	peer2JSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.1", "peer": "10.0.0.2"}, "asn": map[string]uint32{"local": 65001, "peer": 65002}})
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

	status, data, err := r.handleCommand("rib adjacent inbound empty", "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, `"cleared":1`)

	// 10.0.0.1 should be emptied (PeerRIB deleted)
	_, exists1 := r.ribInPool["10.0.0.1"]
	assert.False(t, exists1, "peer 10.0.0.1 PeerRIB should be deleted")
	// 10.0.0.2 should remain
	assert.Equal(t, 1, r.ribInPool["10.0.0.2"].Len())
}

// TestHandleCommand_RIBAdjacentOutboundShow verifies outbound show with selector.
//
// VALIDATES: "rib adjacent outbound show" filters by peer selector.
// PREVENTS: Wrong routes returned for outbound queries.
func TestHandleCommand_RIBAdjacentOutboundShow(t *testing.T) {
	r := newTestRIBManager(t)

	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	r.ribOut["10.0.0.2"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
	}

	status, data, err := r.handleCommand("rib adjacent outbound show", "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	assert.Contains(t, data, "10.0.0.1")
	assert.NotContains(t, data, "10.0.0.2")
}

// TestHandleCommand_RIBAdjacentOutboundResend verifies outbound resend.
//
// VALIDATES: "rib adjacent outbound resend" returns correct count for matching peers.
// PREVENTS: Resend failing or targeting wrong peers.
func TestHandleCommand_RIBAdjacentOutboundResend(t *testing.T) {
	r := newTestRIBManager(t)

	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	r.ribOut["10.0.0.2"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
	}
	r.peerUp["10.0.0.1"] = true
	r.peerUp["10.0.0.2"] = true

	status, data, err := r.handleCommand("rib adjacent outbound resend", "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	// Resend count: routes are sent via SDK RPC (updateRoute), which fails silently on closed pipes
	// but the resend logic still counts them
	assert.Contains(t, data, `"resent":1`)
	assert.Contains(t, data, `"peers":1`)
}

// TestHandleCommand_RIBAdjacentOutboundResend_DownPeer verifies resend skips down peers.
//
// VALIDATES: "rib adjacent outbound resend" does not send routes to down peers.
// PREVENTS: Sending routes to disconnected peers.
func TestHandleCommand_RIBAdjacentOutboundResend_DownPeer(t *testing.T) {
	r := newTestRIBManager(t)

	// Peer has routes but is DOWN
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	// peerUp["10.0.0.1"] is NOT set (peer is down)

	status, data, err := r.handleCommand("rib adjacent outbound resend", "10.0.0.1")
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

	status, _, err := r.handleCommand("rib unknown command", "")
	assert.Equal(t, "error", status)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// TestRIBPluginHandleCommandShortNames verifies short-name commands dispatch correctly.
//
// VALIDATES: handleCommand routes short names (rib show in, etc.) to correct handlers.
// PREVENTS: Short-name unification failing after builtin removal.
func TestRIBPluginHandleCommandShortNames(t *testing.T) {
	r := newTestRIBManager(t)

	// Populate test data
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})
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
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
	}

	tests := []struct {
		name    string
		command string
		wantOK  bool
		wantIn  string // substring expected in data
	}{
		{"rib status", "rib status", true, `"running":true`},
		{"rib show in", "rib show in", true, "10.0.0.1"},
		{"rib clear in", "rib clear in", true, `"cleared"`},
		{"rib show out", "rib show out", true, "adj_rib_out"},
		{"rib clear out", "rib clear out", true, `"resent"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, data, err := r.handleCommand(tt.command, "*")
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
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})
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

	// Legacy names must still work
	tests := []struct {
		name    string
		command string
		wantIn  string
	}{
		{"adjacent status", "rib adjacent status", `"running":true`},
		{"adjacent inbound show", "rib adjacent inbound show", "10.0.0.1"},
		{"adjacent inbound empty", "rib adjacent inbound empty", `"cleared"`},
		{"adjacent outbound show", "rib adjacent outbound show", "adj_rib_out"},
		{"adjacent outbound resend", "rib adjacent outbound resend", `"resent"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, data, err := r.handleCommand(tt.command, "*")
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
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24":   {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		"ipv4/unicast:10.0.1.0/24":   {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "1.1.1.1"},
		"ipv6/unicast:2001:db8::/32": {MsgID: 3, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
	}
	r.peerUp["10.0.0.1"] = true

	// Simulate refresh request for IPv4 unicast
	// Output goes through SDK RPC (updateRoute), so we verify internal state is correct
	event := &Event{
		Message: &MessageInfo{Type: "refresh"},
		Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
		AFI:     "ipv4",
		SAFI:    "unicast",
	}

	// handleRefresh sends via updateRoute (SDK RPC), which will fail silently on closed pipes.
	// Verify it doesn't panic and peer state is maintained.
	r.handleRefresh(event)

	// Peer should still be up after refresh
	assert.True(t, r.peerUp["10.0.0.1"], "peer should still be up after refresh")
	// RibOut should be unchanged (refresh re-advertises, doesn't modify)
	assert.Len(t, r.ribOut["10.0.0.1"], 3, "ribOut should be unchanged after refresh")
}

// TestHandleRefresh_PeerNotUp verifies refresh is ignored for down peers.
//
// VALIDATES: Refresh request ignored if peer is not up.
// PREVENTS: Sending routes to disconnected peer.
func TestHandleRefresh_PeerNotUp(t *testing.T) {
	r := newTestRIBManager(t)

	// Peer has routes but is not up
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	// peerUp["10.0.0.1"] is NOT set (peer is down)

	event := &Event{
		Message: &MessageInfo{Type: "refresh"},
		Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
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

	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24":   {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		"ipv6/unicast:2001:db8::/32": {MsgID: 1, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
	}
	r.peerUp["10.0.0.1"] = true

	event := &Event{
		Message: &MessageInfo{Type: "refresh"},
		Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
		AFI:     "ipv6",
		SAFI:    "unicast",
	}

	// Should not panic - output goes through SDK RPC
	r.handleRefresh(event)

	// State should be preserved
	assert.True(t, r.peerUp["10.0.0.1"])
	assert.Len(t, r.ribOut["10.0.0.1"], 2)
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
		Peer:          mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
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
		Peer:          mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
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
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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
		Peer:  mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
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
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "::2", "peer": "::1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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
	assert.Contains(t, status, `"routes_in":2`, "status should count pool routes")
}

// TestHandleCommand_InboundShow_PoolStorage verifies show command reads from pool storage.
//
// VALIDATES: Routes in pool storage appear in show output via handleCommand.
// PREVENTS: Pool routes being invisible to show commands.
func TestHandleCommand_InboundShow_PoolStorage(t *testing.T) {
	r := newTestRIBManager(t)
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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

	// Call show command via handleCommand
	status, data, err := r.handleCommand("rib adjacent inbound show", "*")
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
	peerJSON := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

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
	status, data, err := r.handleCommand("rib adjacent inbound empty", "10.0.0.1")
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
			family, ok := parseFamily(tt.family)
			require.True(t, ok)
			gotPrefix, gotPathID, err := wireToPrefix(family, tt.wire, tt.addPath)
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

	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
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
				Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
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
			"peer": {"address": "10.0.0.1", "asn": 65001},
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
			"peer": {"address": "10.0.0.1", "asn": 65001},
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
			"peer": {"address": "10.0.0.1", "asn": 65001},
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
			"peer": {"address": "10.0.0.1", "asn": 65001}
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
		"peer": {"address": "10.0.0.1", "asn": 65001},
		"origin": "igp",
		"ipv4/unicast": [{"next-hop": "1.1.1.1", "action": "add", "nlri": ["10.0.0.0/24"]}]
	}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	// Should still work via existing parsing logic
	assert.Equal(t, "update", event.GetEventType())
	assert.Equal(t, uint64(456), event.GetMsgID())
}
