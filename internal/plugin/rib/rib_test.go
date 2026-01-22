package rib

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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

// TestHandleState_PeerUp verifies replay on peer up.
//
// VALIDATES: Stored routes are replayed when peer comes up.
// PREVENTS: Routes being lost after session restart.
func TestHandleState_PeerUp(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

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

	output := out.String()
	assert.Contains(t, output, "peer 10.0.0.1 update text nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24")
	assert.Contains(t, output, "peer 10.0.0.1 update text nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.1.0/24")
	assert.Contains(t, output, "session api ready")
}

// TestHandleState_PeerDown verifies Adj-RIB-In is cleared on peer down.
//
// VALIDATES: Received routes are cleared when peer goes down.
// PREVENTS: Stale routes from failed peers.
func TestHandleState_PeerDown(t *testing.T) {
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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
			r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

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
	for i := 0; i < 10; i++ {
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
	for i := 0; i < 10; i++ {
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
// VALIDATES: Replayed routes include path-id when non-zero.
// PREVENTS: Path-id being lost during session restart replay.
func TestReplayRoutesWithPathID(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

	// Pre-populate ribOut with routes, some with path-id
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24":   {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1", PathID: 0},
		"ipv4/unicast:10.0.0.0/24:1": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1", PathID: 1},
		"ipv4/unicast:10.0.0.0/24:2": {MsgID: 3, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "2.2.2.2", PathID: 2},
	}

	event := &Event{
		Type:  "state",
		Peer:  mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
		State: "up",
	}

	r.handleState(event)

	output := out.String()

	// Route without path-id should NOT have path-information in command
	assert.Contains(t, output, "peer 10.0.0.1 update text nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24\n")

	// Routes with path-id MUST have path-information in command
	assert.Contains(t, output, "peer 10.0.0.1 update text path-information set 1 nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24")
	assert.Contains(t, output, "peer 10.0.0.1 update text path-information set 2 nhop set 2.2.2.2 nlri ipv4/unicast add 10.0.0.0/24")
}

// mustMarshal marshals v to json.RawMessage, failing test on error.
func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

// TestHandleRequest_RIBAdjacentStatus verifies renamed status command.
//
// VALIDATES: "rib adjacent status" returns status JSON.
// PREVENTS: Command rename breaking status queries.
func TestHandleRequest_RIBAdjacentStatus(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

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

	event := &Event{
		Type:    "request",
		Serial:  "test123",
		Command: "rib adjacent status",
	}
	r.handleRequest(event)

	output := out.String()
	assert.Contains(t, output, "@test123 done")
	assert.Contains(t, output, `"running":true`)
	assert.Contains(t, output, `"routes_in":1`)
	assert.Contains(t, output, `"routes_out":2`)
}

// TestHandleRequest_RIBAdjacentInboundShow verifies inbound show with selector.
//
// VALIDATES: "rib adjacent inbound show" filters by peer selector.
// PREVENTS: Wrong routes returned for filtered queries.
func TestHandleRequest_RIBAdjacentInboundShow(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

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
		name        string
		selector    string // peer selector (JSON string format)
		wantPeer1   bool
		wantPeer2   bool
		wantSuccess bool
	}{
		{
			name:        "all_peers",
			selector:    "*",
			wantPeer1:   true,
			wantPeer2:   true,
			wantSuccess: true,
		},
		{
			name:        "specific_peer",
			selector:    "10.0.0.1",
			wantPeer1:   true,
			wantPeer2:   false,
			wantSuccess: true,
		},
		{
			name:        "multi_peer",
			selector:    "10.0.0.1,10.0.0.2",
			wantPeer1:   true,
			wantPeer2:   true,
			wantSuccess: true,
		},
		{
			name:        "negation",
			selector:    "!10.0.0.2",
			wantPeer1:   true,
			wantPeer2:   false,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out.Reset()
			// Peer selector is sent as JSON string in "peer" field
			peerJSON, _ := json.Marshal(tt.selector)
			event := &Event{
				Type:    "request",
				Serial:  "inshow",
				Command: "rib adjacent inbound show",
				Peer:    peerJSON,
			}
			r.handleRequest(event)

			output := out.String()
			if tt.wantSuccess {
				assert.Contains(t, output, "@inshow done")
			}
			if tt.wantPeer1 {
				assert.Contains(t, output, "10.0.0.1")
			} else {
				assert.NotContains(t, output, "10.0.0.1")
			}
			if tt.wantPeer2 {
				assert.Contains(t, output, "10.0.0.2")
			} else {
				assert.NotContains(t, output, "10.0.0.2")
			}
		})
	}
}

// TestHandleRequest_RIBAdjacentInboundEmpty verifies inbound empty with selector.
//
// VALIDATES: "rib adjacent inbound empty" empties matching peers only.
// PREVENTS: Emptying wrong peers' routes.
func TestHandleRequest_RIBAdjacentInboundEmpty(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

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

	peerSelector, _ := json.Marshal("10.0.0.1")
	event := &Event{
		Type:    "request",
		Serial:  "empty1",
		Command: "rib adjacent inbound empty",
		Peer:    peerSelector,
	}
	r.handleRequest(event)

	// 10.0.0.1 should be emptied (PeerRIB deleted)
	_, exists1 := r.ribInPool["10.0.0.1"]
	assert.False(t, exists1, "peer 10.0.0.1 PeerRIB should be deleted")
	// 10.0.0.2 should remain
	assert.Equal(t, 1, r.ribInPool["10.0.0.2"].Len())
	assert.Contains(t, out.String(), "@empty1 done")
}

// TestHandleRequest_RIBAdjacentOutboundShow verifies outbound show with selector.
//
// VALIDATES: "rib adjacent outbound show" filters by peer selector.
// PREVENTS: Wrong routes returned for outbound queries.
func TestHandleRequest_RIBAdjacentOutboundShow(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	r.ribOut["10.0.0.2"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
	}

	peerJSON, _ := json.Marshal("10.0.0.1")
	event := &Event{
		Type:    "request",
		Serial:  "outshow",
		Command: "rib adjacent outbound show",
		Peer:    peerJSON,
	}
	r.handleRequest(event)

	output := out.String()
	assert.Contains(t, output, "@outshow done")
	assert.Contains(t, output, "10.0.0.1")
	assert.NotContains(t, output, "10.0.0.2")
}

// TestHandleRequest_RIBAdjacentOutboundResend verifies outbound resend.
//
// VALIDATES: "rib adjacent outbound resend" replays routes to matching peers.
// PREVENTS: Resend failing or targeting wrong peers.
func TestHandleRequest_RIBAdjacentOutboundResend(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	r.ribOut["10.0.0.2"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
	}
	r.peerUp["10.0.0.1"] = true
	r.peerUp["10.0.0.2"] = true

	peerJSON, _ := json.Marshal("10.0.0.1")
	event := &Event{
		Type:    "request",
		Serial:  "resend1",
		Command: "rib adjacent outbound resend",
		Peer:    peerJSON,
	}
	r.handleRequest(event)

	output := out.String()
	assert.Contains(t, output, "@resend1 done")
	// Should have replayed routes for 10.0.0.1
	assert.Contains(t, output, "peer 10.0.0.1 update text")
	assert.Contains(t, output, "nlri ipv4/unicast add 10.0.0.0/24")
	// Should NOT have replayed routes for 10.0.0.2
	assert.NotContains(t, output, "peer 10.0.0.2 update text")
	// Should NOT send "session api ready" - that's only for reconnect
	assert.NotContains(t, output, "session api ready")
}

// TestHandleRequest_RIBAdjacentOutboundResend_DownPeer verifies resend skips down peers.
//
// VALIDATES: "rib adjacent outbound resend" does not send routes to down peers.
// PREVENTS: Sending routes to disconnected peers.
func TestHandleRequest_RIBAdjacentOutboundResend_DownPeer(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

	// Peer has routes but is DOWN
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	// peerUp["10.0.0.1"] is NOT set (peer is down)

	peerJSON, _ := json.Marshal("10.0.0.1")
	event := &Event{
		Type:    "request",
		Serial:  "resend_down",
		Command: "rib adjacent outbound resend",
		Peer:    peerJSON,
	}
	r.handleRequest(event)

	output := out.String()
	assert.Contains(t, output, "@resend_down done")
	assert.Contains(t, output, `"resent":0`, "should not resend to down peer")
	assert.Contains(t, output, `"peers":0`, "no peers should be affected")
	// Should NOT have any route updates
	assert.NotContains(t, output, "peer 10.0.0.1 update text")
}

// TestHandleRequest_UnknownCommand verifies unknown commands are rejected.
//
// VALIDATES: Unknown commands return error response.
// PREVENTS: Silent failures on typos.
func TestHandleRequest_UnknownCommand(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

	event := &Event{
		Type:    "request",
		Serial:  "unknown",
		Command: "rib unknown command",
	}
	r.handleRequest(event)

	output := out.String()
	assert.Contains(t, output, "@unknown error")
}

// =============================================================================
// RFC 7313 - Enhanced Route Refresh Tests
// =============================================================================

// TestHandleRefresh_SendsMarkersAndRoutes verifies route refresh response.
//
// RFC 7313 Section 3: Upon receiving a route refresh request, the speaker
// SHOULD send BoRR, re-advertise Adj-RIB-Out, then send EoRR.
//
// VALIDATES: Refresh triggers BoRR, routes, EoRR sequence.
// PREVENTS: Missing markers or routes in refresh response.
func TestHandleRefresh_SendsMarkersAndRoutes(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

	// Pre-populate ribOut with routes
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24":   {MsgID: 1, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		"ipv4/unicast:10.0.1.0/24":   {MsgID: 2, Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "1.1.1.1"},
		"ipv6/unicast:2001:db8::/32": {MsgID: 3, Family: "ipv6/unicast", Prefix: "2001:db8::/32", NextHop: "::1"},
	}
	r.peerUp["10.0.0.1"] = true

	// Simulate refresh request for IPv4 unicast
	event := &Event{
		Message: &MessageInfo{Type: "refresh"},
		Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
		AFI:     "ipv4",
		SAFI:    "unicast",
	}

	r.handleRefresh(event)

	output := out.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Verify sequence: BoRR, routes (in order), EoRR
	require.GreaterOrEqual(t, len(lines), 4, "expected at least 4 output lines")

	// First line should be BoRR
	assert.Contains(t, lines[0], "borr ipv4/unicast", "first line should be BoRR")

	// Last line should be EoRR
	assert.Contains(t, lines[len(lines)-1], "eorr ipv4/unicast", "last line should be EoRR")

	// Middle lines should contain IPv4 routes only
	assert.Contains(t, output, "nlri ipv4/unicast add 10.0.0.0/24")
	assert.Contains(t, output, "nlri ipv4/unicast add 10.0.1.0/24")

	// IPv6 route should NOT be included (wrong family)
	assert.NotContains(t, output, "2001:db8::/32")
}

// TestHandleRefresh_EmptyRibOut verifies refresh with no routes.
//
// RFC 7313: BoRR and EoRR should still be sent even if no routes to advertise.
//
// VALIDATES: Empty Adj-RIB-Out still sends BoRR and EoRR markers.
// PREVENTS: Missing markers when no routes exist.
func TestHandleRefresh_EmptyRibOut(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

	// Peer is up but has no routes in ribOut
	r.peerUp["10.0.0.1"] = true

	event := &Event{
		Message: &MessageInfo{Type: "refresh"},
		Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
		AFI:     "ipv4",
		SAFI:    "unicast",
	}

	r.handleRefresh(event)

	output := out.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have exactly 2 lines: BoRR and EoRR
	require.Len(t, lines, 2, "expected 2 output lines for empty RIB")
	assert.Contains(t, lines[0], "borr ipv4/unicast")
	assert.Contains(t, lines[1], "eorr ipv4/unicast")
}

// TestHandleRefresh_PeerNotUp verifies refresh is ignored for down peers.
//
// VALIDATES: Refresh request ignored if peer is not up.
// PREVENTS: Sending routes to disconnected peer.
func TestHandleRefresh_PeerNotUp(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

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

	r.handleRefresh(event)

	// Should produce no output (peer is down)
	assert.Empty(t, out.String(), "should not send anything to down peer")
}

// TestHandleRefresh_IPv6Family verifies refresh for IPv6 unicast.
//
// VALIDATES: IPv6 routes are filtered correctly by family.
// PREVENTS: Wrong family routes being sent.
func TestHandleRefresh_IPv6Family(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

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

	r.handleRefresh(event)

	output := out.String()
	assert.Contains(t, output, "borr ipv6/unicast")
	assert.Contains(t, output, "eorr ipv6/unicast")
	assert.Contains(t, output, "2001:db8::/32")
	// IPv4 route should NOT be included
	assert.NotContains(t, output, "10.0.0.0/24")
}

// =============================================================================
// Pool Storage Tests (Phase 3-4)
// =============================================================================

// TestHandleReceived_PoolStorage verifies routes stored in PeerRIB when raw fields present.
//
// VALIDATES: Raw attributes and NLRIs from format=full are stored in pool.
// PREVENTS: Pool storage being ignored when raw fields are available.
func TestHandleReceived_PoolStorage(t *testing.T) {
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

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

	// Legacy storage should be empty (or not used)
	// Note: we may still populate it for show commands compatibility
}

// TestHandleReceived_PoolStorage_MultipleNLRIs verifies multiple NLRIs in one UPDATE.
//
// VALIDATES: Concatenated NLRIs are split and stored individually.
// PREVENTS: Only first NLRI being stored.
func TestHandleReceived_PoolStorage_MultipleNLRIs(t *testing.T) {
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
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

// TestHandleInboundShow_PoolStorage verifies show command reads from pool storage.
//
// VALIDATES: Routes in pool storage appear in show output.
// PREVENTS: Pool routes being invisible to show commands.
func TestHandleInboundShow_PoolStorage(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)
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

	// Now call show command
	out.Reset()
	showEvent := &Event{
		Type:    "request",
		Serial:  "show1",
		Command: "rib adjacent inbound show",
		Peer:    json.RawMessage(`"*"`),
	}
	r.handleRequest(showEvent)

	output := out.String()
	assert.Contains(t, output, "@show1 done", "should respond done")
	assert.Contains(t, output, "10.0.0.1", "should contain peer address")
	assert.Contains(t, output, "10.0.0.0/24", "should contain prefix from pool")
	assert.Contains(t, output, "ipv4/unicast", "should contain family")
}

// TestHandleInboundEmpty_PoolStorage verifies empty command clears pool storage.
//
// VALIDATES: Empty command clears routes from pool storage.
// PREVENTS: Pool routes remaining after empty command.
func TestHandleInboundEmpty_PoolStorage(t *testing.T) {
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)
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

	// Call empty command
	out.Reset()
	emptyEvent := &Event{
		Type:    "request",
		Serial:  "empty1",
		Command: "rib adjacent inbound empty",
		Peer:    json.RawMessage(`"10.0.0.1"`),
	}
	r.handleRequest(emptyEvent)

	// Verify pool is cleared (entry deleted to avoid memory leak)
	_, exists := r.ribInPool["10.0.0.1"]
	assert.False(t, exists, "pool entry should be deleted")
	assert.Contains(t, out.String(), "@empty1 done")
	assert.Contains(t, out.String(), `"cleared":1`)
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
	var out bytes.Buffer
	r := NewRIBManager(strings.NewReader(""), &out)

	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	r.peerUp["10.0.0.1"] = true

	tests := []struct {
		name      string
		eventType string
		wantOut   bool // Whether we expect output
	}{
		{"refresh triggers response", "refresh", true},
		{"borr is logged only", "borr", false},
		{"eorr is logged only", "eorr", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out.Reset()
			event := &Event{
				Message: &MessageInfo{Type: tt.eventType},
				Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
				AFI:     "ipv4",
				SAFI:    "unicast",
			}

			r.dispatch(event)

			if tt.wantOut {
				assert.NotEmpty(t, out.String(), "expected output for %s", tt.eventType)
			} else {
				assert.Empty(t, out.String(), "expected no output for %s", tt.eventType)
			}
		})
	}
}

// =============================================================================
// IPC Protocol 2.0 New Event Format Tests
// =============================================================================
// These tests validate parsing of the new JSON format per ipc_protocol.md v2.0:
//   - Top-level "type" field indicates payload key ("bgp" or "rib")
//   - Event content nested under "bgp" or "rib" key
//   - Event type in payload (e.g., bgp.type = "update")
//   - Attributes nested under "attributes"
//   - NLRIs nested under "nlri"
// =============================================================================

// TestParseEvent_NewBGPFormat verifies parsing of IPC 2.0 BGP event format.
//
// VALIDATES: New wrapped BGP format with type/bgp structure parses correctly.
// PREVENTS: Plugin breaking when engine updates to IPC 2.0 format.
func TestParseEvent_NewBGPFormat(t *testing.T) {
	// IPC 2.0 format: type at top, payload nested under "bgp"
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

// TestParseEvent_NewRIBFormat verifies parsing of IPC 2.0 RIB event format.
//
// VALIDATES: New wrapped RIB format with type/rib structure parses correctly.
// PREVENTS: RIB cache events being ignored in new format.
func TestParseEvent_NewRIBFormat(t *testing.T) {
	// IPC 2.0 format: type at top, payload nested under "rib"
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
