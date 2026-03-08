package gr

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleEventOpenCapture verifies that a received OPEN event with GR capability
// stores the parsed grPeerCap for later use during state events.
//
// VALIDATES: handleOpenEvent extracts cap 64 hex from OPEN JSON, decodes it, stores grPeerCap.
// PREVENTS: GR capability being lost when peer's OPEN arrives before session-down.
func TestHandleEventOpenCapture(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	// GR capability: restart-time=120 (0x0078), IPv4/unicast F-bit=1 (AFI=1 SAFI=1 flags=0x80)
	event := `{"type":"bgp","bgp":{"message":{"type":"open","direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"open":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[{"code":64,"name":"graceful-restart","value":"00780001018000020180"}]}}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	gp.mu.Lock()
	cap, ok := gp.peerCaps["10.0.0.1"]
	gp.mu.Unlock()

	require.True(t, ok, "GR capability should be stored for peer")
	assert.Equal(t, uint16(120), cap.RestartTime)
	require.Len(t, cap.Families, 2)
	assert.Equal(t, "ipv4/unicast", cap.Families[0].Family)
	assert.True(t, cap.Families[0].ForwardState)
	assert.Equal(t, "ipv6/unicast", cap.Families[1].Family)
	assert.True(t, cap.Families[1].ForwardState)
}

// TestHandleEventOpenNoGR verifies that OPEN without GR cap stores nothing.
//
// VALIDATES: handleOpenEvent ignores OPEN messages without capability code 64.
// PREVENTS: Spurious GR state for peers without GR capability.
func TestHandleEventOpenNoGR(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"open","direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"open":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[{"code":1,"name":"multiprotocol","value":"00010001"}]}}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	gp.mu.Lock()
	_, ok := gp.peerCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, ok, "no GR capability should be stored")
}

// TestHandleEventStateDown verifies that state-down with tcp-failure activates GR.
//
// VALIDATES: handleStateEvent on "down" with reason "tcp-failure" activates GR state manager.
// PREVENTS: GR not activating when the session drops due to TCP failure.
func TestHandleEventStateDown(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	// Pre-store GR capability (as if OPEN was received earlier)
	gp.peerCaps["10.0.0.1"] = &grPeerCap{
		RestartTime: 120,
		Families:    []grCapFamily{{Family: "ipv4/unicast", ForwardState: true}},
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"down","reason":"tcp-failure"}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	// GR state should be active (retain-routes would be dispatched via SDK, but we have no SDK here)
	assert.True(t, gp.state.peerActive("10.0.0.1"), "GR should be active after tcp-failure down")
}

// TestHandleEventStateDownNotification verifies that NOTIFICATION bypasses GR.
//
// VALIDATES: handleStateEvent on "down" with reason "notification" does NOT activate GR.
// PREVENTS: Route retention when session ended due to NOTIFICATION (RFC 4724).
func TestHandleEventStateDownNotification(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	gp.peerCaps["10.0.0.1"] = &grPeerCap{
		RestartTime: 120,
		Families:    []grCapFamily{{Family: "ipv4/unicast", ForwardState: true}},
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"down","reason":"notification"}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	assert.False(t, gp.state.peerActive("10.0.0.1"), "GR should NOT activate on NOTIFICATION")
}

// TestHandleEventStateDownNoGRCap verifies that non-GR peers don't activate GR.
//
// VALIDATES: handleStateEvent on "down" for peer without stored GR cap does nothing.
// PREVENTS: GR activation for peers that never advertised GR capability.
func TestHandleEventStateDownNoGRCap(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"down","reason":"tcp-failure"}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	assert.False(t, gp.state.peerActive("10.0.0.1"), "no GR for peer without capability")
}

// TestHandleEventEOR verifies that EOR event triggers stale route purge tracking.
//
// VALIDATES: handleEOREvent calls state manager's onEORReceived for the family.
// PREVENTS: EOR events being ignored, leaving stale routes indefinitely.
func TestHandleEventEOR(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	// Set up GR state: peer is in GR with IPv4 and IPv6 stale
	cap := &grPeerCap{
		RestartTime: 120,
		Families: []grCapFamily{
			{Family: "ipv4/unicast", ForwardState: true},
			{Family: "ipv6/unicast", ForwardState: true},
		},
	}
	gp.state.onSessionDown("10.0.0.1", cap, false)
	gp.state.onSessionReestablished("10.0.0.1", cap)

	// EOR for IPv4
	event := `{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"address":"10.0.0.1","asn":65001},"eor":{"family":"ipv4/unicast"}}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	// GR should still be active (IPv6 not yet received EOR)
	assert.True(t, gp.state.peerActive("10.0.0.1"))

	// EOR for IPv6 — completes GR
	event = `{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"address":"10.0.0.1","asn":65001},"eor":{"family":"ipv6/unicast"}}}`

	err = gp.handleEvent(event)
	require.NoError(t, err)

	assert.False(t, gp.state.peerActive("10.0.0.1"), "GR should complete after all EORs")
}

// TestHandleEventInvalidJSON verifies graceful handling of malformed events.
//
// VALIDATES: handleEvent returns nil (no error) for invalid JSON.
// PREVENTS: Plugin crash on malformed event delivery.
func TestHandleEventInvalidJSON(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	err := gp.handleEvent("not json at all")
	assert.NoError(t, err, "invalid JSON should not cause error")
}

// TestHandleEventUnknownType verifies that unknown event types are ignored.
//
// VALIDATES: handleEvent silently ignores events with unrecognized message types.
// PREVENTS: Unexpected behavior or errors from future event types.
func TestHandleEventUnknownType(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"keepalive"},"peer":{"address":"10.0.0.1","asn":65001}}}`

	err := gp.handleEvent(event)
	assert.NoError(t, err)
}

// TestHandleEventStateUp verifies that state-up triggers session reestablishment.
//
// VALIDATES: handleStateEvent on "up" calls onSessionReestablished with new GR cap.
// PREVENTS: Stale routes not being purged when peer reconnects without forwarding state.
func TestHandleEventStateUp(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	// Set up: peer had GR with IPv4 and IPv6
	oldCap := &grPeerCap{
		RestartTime: 120,
		Families: []grCapFamily{
			{Family: "ipv4/unicast", ForwardState: true},
			{Family: "ipv6/unicast", ForwardState: true},
		},
	}
	gp.state.onSessionDown("10.0.0.1", oldCap, false)

	// New OPEN only has IPv4 with F-bit=1 (IPv6 missing)
	gp.peerCaps["10.0.0.1"] = &grPeerCap{
		RestartTime: 120,
		Families:    []grCapFamily{{Family: "ipv4/unicast", ForwardState: true}},
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	// GR should still be active for IPv4 (waiting for EOR)
	assert.True(t, gp.state.peerActive("10.0.0.1"))
}

// TestHandleOpenEventCapHexDecode verifies round-trip from wire hex to stored cap.
//
// VALIDATES: The exact hex bytes from a real GR capability wire encoding are decoded correctly.
// PREVENTS: Off-by-one or endianness bugs in hex parsing pipeline.
func TestHandleOpenEventCapHexDecode(t *testing.T) {
	gp := &grPlugin{
		peerCaps: make(map[string]*grPeerCap),
		state:    newGRStateManager(nil),
	}

	// Build event with known hex: restart-time=300 (0x012C), IPv4/unicast F-bit=1
	// Wire: 0x012C (flags=0, time=300) + 0x0001 0x01 0x80 (AFI=1, SAFI=1, F=1)
	hexValue := "012c00010180"

	eventObj := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "open", "direction": "received"},
			"peer":    map[string]any{"address": "10.0.0.2", "asn": float64(65002)},
			"open": map[string]any{
				"asn":       float64(65002),
				"router-id": "2.2.2.2",
				"hold-time": float64(90),
				"capabilities": []any{
					map[string]any{"code": float64(64), "name": "graceful-restart", "value": hexValue},
				},
			},
		},
	}
	eventJSON, _ := json.Marshal(eventObj)

	err := gp.handleEvent(string(eventJSON))
	require.NoError(t, err)

	gp.mu.Lock()
	cap, ok := gp.peerCaps["10.0.0.2"]
	gp.mu.Unlock()

	require.True(t, ok)
	assert.Equal(t, uint16(300), cap.RestartTime)
	require.Len(t, cap.Families, 1)
	assert.Equal(t, "ipv4/unicast", cap.Families[0].Family)
	assert.True(t, cap.Families[0].ForwardState)
}
