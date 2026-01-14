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
// VALIDATES: Received routes are tracked per peer.
// PREVENTS: Route state being lost.
func TestHandleReceived_StoresRoutes(t *testing.T) {
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

	// New command-style format: family operations with action/next-hop/nlri
	event := &Event{
		Message: &MessageInfo{Type: "update", ID: 200},
		Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24", "10.0.1.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	assert.Len(t, r.ribIn["10.0.0.1"], 2)
	route := r.ribIn["10.0.0.1"]["ipv4/unicast:10.0.0.0/24"]
	require.NotNil(t, route)
	assert.Equal(t, "10.0.0.0/24", route.Prefix)
	assert.Equal(t, "1.1.1.1", route.NextHop)
	assert.Equal(t, uint64(200), route.MsgID)
}

// TestHandleReceived_Withdraw verifies routes are removed on withdrawal.
//
// VALIDATES: Withdrawn routes are removed from Adj-RIB-In.
// PREVENTS: Stale route state.
func TestHandleReceived_Withdraw(t *testing.T) {
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})
	nestedPeer := mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}})

	// First announce
	announce := &Event{
		Message: &MessageInfo{Type: "update", ID: 200},
		Peer:    nestedPeer,
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "1.1.1.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(announce)
	assert.Len(t, r.ribIn["10.0.0.1"], 1)

	// Then withdraw
	withdraw := &Event{
		Message: &MessageInfo{Type: "update", ID: 201},
		Peer:    nestedPeer,
		FamilyOps: map[string][]FamilyOperation{
			"ipv4/unicast": {
				{Action: "del", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(withdraw)
	assert.Len(t, r.ribIn["10.0.0.1"], 0)
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

	// Pre-populate ribIn and ribOut
	r.ribIn["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
	}
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

	// ribIn should be cleared
	assert.Len(t, r.ribIn["10.0.0.1"], 0)
	// ribOut should be preserved for replay
	assert.Len(t, r.ribOut["10.0.0.1"], 1)
}

// TestStatusJSON verifies status command output.
//
// VALIDATES: Status returns correct route counts.
// PREVENTS: Incorrect stats being reported.
func TestStatusJSON(t *testing.T) {
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

	r.ribIn["10.0.0.1"] = map[string]*Route{
		"a": {}, "b": {},
	}
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
			name: "update_to_ribIn",
			event: &Event{
				Message: &MessageInfo{Type: "update"},
				Peer:    json.RawMessage(`{"address":{"local":"","peer":"10.0.0.1"},"asn":{"local":0,"peer":65001}}`),
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
			for _, routes := range r.ribIn {
				totalIn += len(routes)
			}
			totalOut := 0
			for _, routes := range r.ribOut {
				totalOut += len(routes)
			}

			assert.Equal(t, tt.wantRibIn, totalIn, "ribIn count")
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

	// Pre-populate ribOut and ribIn
	r.ribOut["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {MsgID: 1, Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	r.ribIn["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {Prefix: "10.0.1.0/24"},
	}

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

	r.peerUp["10.0.0.1"] = true
	r.ribIn["10.0.0.1"] = map[string]*Route{"a": {}}
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

	r.ribIn["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	r.ribIn["10.0.0.2"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
	}

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

	r.ribIn["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
	}
	r.ribIn["10.0.0.2"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
	}

	peerJSON, _ := json.Marshal("10.0.0.1")
	event := &Event{
		Type:    "request",
		Serial:  "empty1",
		Command: "rib adjacent inbound empty",
		Peer:    peerJSON,
	}
	r.handleRequest(event)

	// 10.0.0.1 should be emptied
	assert.Len(t, r.ribIn["10.0.0.1"], 0)
	// 10.0.0.2 should remain
	assert.Len(t, r.ribIn["10.0.0.2"], 1)
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
