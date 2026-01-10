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
	input := `{"type":"sent","msg-id":123,"peer":{"address":"10.0.0.1","asn":65001},"announce":{"ipv4/unicast":{"1.1.1.1":["10.0.0.0/24","10.0.1.0/24"]}}}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "sent", event.GetEventType())
	assert.Equal(t, uint64(123), event.GetMsgID())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.NotNil(t, event.Announce)
	assert.Contains(t, event.Announce, "ipv4/unicast")
}

// TestParseEvent_ReceivedFormat verifies parsing of received UPDATE events.
//
// VALIDATES: Received events with message wrapper are parsed correctly.
// PREVENTS: Received events being dropped due to format mismatch.
func TestParseEvent_ReceivedFormat(t *testing.T) {
	// Actual format from plugin system: array of prefixes, same as sent events
	input := `{"message":{"type":"update","id":456},"direction":"received","peer":{"address":{"local":"10.0.0.2","peer":"10.0.0.1"},"asn":{"local":65002,"peer":65001}},"announce":{"ipv4/unicast":{"1.1.1.1":["10.0.0.0/24","10.0.1.0/24"]}}}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "update", event.GetEventType())
	assert.Equal(t, uint64(456), event.GetMsgID())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.NotNil(t, event.Announce)
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
	input := `{"type":"request","serial":"abc123","command":"rib status"}`

	event, err := parseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "request", event.GetEventType())
	assert.Equal(t, "abc123", event.Serial)
	assert.Equal(t, "rib status", event.Command)
}

// TestHandleSent_StoresRoutes verifies routes are stored in Adj-RIB-Out.
//
// VALIDATES: Sent routes are persisted for replay.
// PREVENTS: Routes being lost on peer reconnect.
func TestHandleSent_StoresRoutes(t *testing.T) {
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

	event := &Event{
		Type:  "sent",
		MsgID: 100,
		Peer:  mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
		Announce: map[string]map[string]any{
			"ipv4/unicast": {
				"1.1.1.1": []any{"10.0.0.0/24", "10.0.1.0/24"},
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
		Announce: map[string]map[string]any{
			"ipv4/unicast": {"1.1.1.1": []any{"10.0.0.0/24"}},
		},
	}
	r.handleSent(announce)
	assert.Len(t, r.ribOut["10.0.0.1"], 1)

	// Then withdraw
	withdraw := &Event{
		Type: "sent",
		Peer: mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
		Withdraw: map[string][]any{
			"ipv4/unicast": {"10.0.0.0/24"},
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

	// Actual format: array of prefixes (same as sent events)
	event := &Event{
		Message: &MessageInfo{Type: "update", ID: 200},
		Peer:    mustMarshal(t, map[string]any{"address": map[string]string{"local": "10.0.0.2", "peer": "10.0.0.1"}, "asn": map[string]uint32{"local": 65002, "peer": 65001}}),
		Announce: map[string]map[string]any{
			"ipv4/unicast": {
				"1.1.1.1": []any{"10.0.0.0/24", "10.0.1.0/24"},
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

	// First announce (array format)
	announce := &Event{
		Message: &MessageInfo{Type: "update", ID: 200},
		Peer:    nestedPeer,
		Announce: map[string]map[string]any{
			"ipv4/unicast": {
				"1.1.1.1": []any{"10.0.0.0/24"},
			},
		},
	}
	r.handleReceived(announce)
	assert.Len(t, r.ribIn["10.0.0.1"], 1)

	// Then withdraw
	withdraw := &Event{
		Message: &MessageInfo{Type: "update", ID: 201},
		Peer:    nestedPeer,
		Withdraw: map[string][]any{
			"ipv4/unicast": {"10.0.0.0/24"},
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
	assert.Contains(t, output, "peer 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1")
	assert.Contains(t, output, "peer 10.0.0.1 announce route 10.0.1.0/24 next-hop 1.1.1.1")
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

// TestRoutesJSON verifies routes command output.
//
// VALIDATES: Routes output includes both RIBs.
// PREVENTS: Missing route data in output.
func TestRoutesJSON(t *testing.T) {
	r := NewRIBManager(strings.NewReader(""), &bytes.Buffer{})

	r.ribIn["10.0.0.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
	}
	r.ribOut["10.0.0.2"] = map[string]*Route{
		"ipv4/unicast:10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "2.2.2.2"},
	}

	output := r.routesJSON()

	// Parse and verify structure
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Contains(t, result, "adj_rib_in")
	assert.Contains(t, result, "adj_rib_out")
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
				Type:     "sent",
				Peer:     mustMarshal(t, PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
				Announce: map[string]map[string]any{"ipv4/unicast": {"1.1.1.1": []any{"10.0.0.0/24"}}},
			},
			wantRibIn:  0,
			wantRibOut: 1,
		},
		{
			name: "update_to_ribIn",
			event: &Event{
				Message:  &MessageInfo{Type: "update"},
				Peer:     json.RawMessage(`{"address":{"local":"","peer":"10.0.0.1"},"asn":{"local":0,"peer":65001}}`),
				Announce: map[string]map[string]any{"ipv4/unicast": {"1.1.1.1": []any{"10.0.0.0/24"}}},
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

	// Route without path-id should NOT have path-id in command
	assert.Contains(t, output, "peer 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.1.1.1\n")

	// Routes with path-id MUST have path-id in command
	assert.Contains(t, output, "peer 10.0.0.1 announce route 10.0.0.0/24 path-id 1 next-hop 1.1.1.1")
	assert.Contains(t, output, "peer 10.0.0.1 announce route 10.0.0.0/24 path-id 2 next-hop 2.2.2.2")
}

// mustMarshal marshals v to json.RawMessage, failing test on error.
func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
