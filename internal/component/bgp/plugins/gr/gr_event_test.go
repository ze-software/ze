package gr

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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
	event := `{"type":"bgp","bgp":{"message":{"type":"open","direction":"received"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"open":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[{"code":64,"name":"graceful-restart","value":"00780001018000020180"}]}}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	gp.mu.Lock()
	cap, ok := gp.peerCaps["10.0.0.1"]
	gp.mu.Unlock()

	require.True(t, ok, "GR capability should be stored for peer")
	assert.Equal(t, uint16(120), cap.RestartTime)
	require.Len(t, cap.Families, 2)
	assert.Equal(t, family.IPv4Unicast, cap.Families[0].Family)
	assert.True(t, cap.Families[0].ForwardState)
	assert.Equal(t, family.IPv6Unicast, cap.Families[1].Family)
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

	event := `{"type":"bgp","bgp":{"message":{"type":"open","direction":"received"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"open":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[{"code":1,"name":"multiprotocol","value":"00010001"}]}}}`

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
		Families:    []grCapFamily{{Family: family.IPv4Unicast, ForwardState: true}},
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"down","reason":"tcp-failure"}}`

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
		Families:    []grCapFamily{{Family: family.IPv4Unicast, ForwardState: true}},
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"down","reason":"notification"}}`

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

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"down","reason":"tcp-failure"}}`

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
			{Family: family.IPv4Unicast, ForwardState: true},
			{Family: family.IPv6Unicast, ForwardState: true},
		},
	}
	gp.state.onSessionDown("10.0.0.1", cap, nil, false)
	gp.state.onSessionReestablished("10.0.0.1", cap, nil)

	// EOR for IPv4
	event := `{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"eor":{"family":"ipv4/unicast"}}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	// GR should still be active (IPv6 not yet received EOR)
	assert.True(t, gp.state.peerActive("10.0.0.1"))

	// EOR for IPv6 -- completes GR
	event = `{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"eor":{"family":"ipv6/unicast"}}}`

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

	event := `{"type":"bgp","bgp":{"message":{"type":"keepalive"},"peer":{"address":"10.0.0.1","remote":{"as":65001}}}}`

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
			{Family: family.IPv4Unicast, ForwardState: true},
			{Family: family.IPv6Unicast, ForwardState: true},
		},
	}
	gp.state.onSessionDown("10.0.0.1", oldCap, nil, false)

	// New OPEN only has IPv4 with F-bit=1 (IPv6 missing)
	gp.peerCaps["10.0.0.1"] = &grPeerCap{
		RestartTime: 120,
		Families:    []grCapFamily{{Family: family.IPv4Unicast, ForwardState: true}},
	}

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"up"}}`

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
			"peer":    map[string]any{"address": "10.0.0.2", "remote": map[string]any{"as": float64(65002)}},
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
	assert.Equal(t, family.IPv4Unicast, cap.Families[0].Family)
	assert.True(t, cap.Families[0].ForwardState)
}

// TestHandleEventOpenLLGR verifies that OPEN with both GR and LLGR capabilities
// stores both grPeerCap and llgrPeerCap.
//
// VALIDATES: handleOpenEvent parses cap 71 alongside cap 64 from OPEN JSON.
// PREVENTS: LLGR capability ignored when peer advertises both GR and LLGR.
func TestHandleEventOpenLLGR(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// GR: restart-time=120, ipv4/unicast F=1
	// LLGR: ipv4/unicast F=1, LLST=3600 (0x000E10)
	event := `{"type":"bgp","bgp":{"message":{"type":"open","direction":"received"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"open":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[{"code":64,"name":"graceful-restart","value":"007800010180"},{"code":71,"name":"long-lived-graceful-restart","value":"00010180000e10"}]}}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	// Verify GR capability stored
	gp.mu.Lock()
	grCap, grOK := gp.peerCaps["10.0.0.1"]
	llgrCap, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	require.True(t, grOK, "GR capability should be stored")
	assert.Equal(t, uint16(120), grCap.RestartTime)

	require.True(t, llgrOK, "LLGR capability should be stored")
	require.Len(t, llgrCap.Families, 1)
	assert.Equal(t, family.IPv4Unicast, llgrCap.Families[0].Family)
	assert.True(t, llgrCap.Families[0].ForwardState)
	assert.Equal(t, uint32(3600), llgrCap.Families[0].LLST)
}

// TestHandleEventOpenLLGR_NoGR verifies that LLGR is ignored without GR capability.
//
// VALIDATES: RFC 9494: LLGR MUST be ignored if GR capability is not present.
// PREVENTS: LLGR state created for peers that only advertise cap 71 without cap 64.
func TestHandleEventOpenLLGR_NoGR(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// Only LLGR capability, no GR
	event := `{"type":"bgp","bgp":{"message":{"type":"open","direction":"received"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"open":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[{"code":71,"name":"long-lived-graceful-restart","value":"00010180000e10"}]}}}`

	err := gp.handleEvent(event)
	require.NoError(t, err)

	gp.mu.Lock()
	_, grOK := gp.peerCaps["10.0.0.1"]
	_, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, grOK, "no GR capability should be stored")
	assert.False(t, llgrOK, "LLGR should be discarded when GR is absent (RFC 9494)")
}

// ---------- Structured OPEN handling tests ----------
//
// These tests exercise the handleStructuredEvent / handleStructuredOpen / extractGRCaps
// path that processes raw OPEN wire bytes delivered via DirectBridge (StructuredEvent)
// rather than JSON text events.

// buildOpenBody constructs an OPEN message body (without the 19-byte BGP header).
// Format: Version(1) + MyAS(2) + HoldTime(2) + BGPIdentifier(4) + OptParamLen(1) + OptionalParams.
// myAS is always 65001 in tests.
func buildOpenBody(optParams []byte) []byte {
	// Fixed test values: myAS=65001, holdTime=90, bgpID=1.1.1.1
	body := make([]byte, 10+len(optParams))
	body[0] = 4    // Version 4
	body[1] = 0xFD // myAS high byte (65001 = 0xFDE9)
	body[2] = 0xE9 // myAS low byte
	body[3] = 0x00 // holdTime high byte (90 = 0x005A)
	body[4] = 0x5A // holdTime low byte
	body[5] = 0x01 // bgpID (1.1.1.1 = 0x01010101)
	body[6] = 0x01
	body[7] = 0x01
	body[8] = 0x01
	body[9] = byte(len(optParams))
	copy(body[10:], optParams)
	return body
}

// buildCapabilityParam wraps capability TLVs in an optional parameter type 2 (capabilities).
func buildCapabilityParam(caps ...[]byte) []byte {
	var payload []byte
	for _, c := range caps {
		payload = append(payload, c...)
	}
	// Optional parameter: type(1) + length(1) + capabilities
	param := make([]byte, 2+len(payload))
	param[0] = 2 // type 2 = capabilities
	param[1] = byte(len(payload))
	copy(param[2:], payload)
	return param
}

// buildGRCapTLV builds a GR capability TLV (code 64).
// flags: 4 high bits of first byte; restartTime: 12-bit value.
// families: each entry is {AFI-high, AFI-low, SAFI, flags}.
func buildGRCapTLV(flags byte, restartTime uint16, families [][4]byte) []byte {
	valueLen := 2 + 4*len(families)
	tlv := make([]byte, 2+valueLen)
	tlv[0] = 64 // code
	tlv[1] = byte(valueLen)
	tlv[2] = (flags << 4) | byte((restartTime>>8)&0x0F)
	tlv[3] = byte(restartTime)
	off := 4
	for _, f := range families {
		copy(tlv[off:off+4], f[:])
		off += 4
	}
	return tlv
}

// buildLLGRCapTLV builds an LLGR capability TLV (code 71).
// Each family tuple is 7 bytes: AFI(2) + SAFI(1) + flags(1) + LLST(3).
func buildLLGRCapTLV(families [][7]byte) []byte {
	valueLen := 7 * len(families)
	tlv := make([]byte, 2+valueLen)
	tlv[0] = 71 // code
	tlv[1] = byte(valueLen)
	off := 2
	for _, f := range families {
		copy(tlv[off:off+7], f[:])
		off += 7
	}
	return tlv
}

// TestHandleStructuredEventDispatchOpen verifies that handleStructuredEvent with
// EventType="open" dispatches to handleStructuredOpen.
//
// VALIDATES: handleStructuredEvent routes "open" events to the structured OPEN handler.
// PREVENTS: Structured OPEN events being silently dropped by the dispatcher.
func TestHandleStructuredEventDispatchOpen(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// GR cap: restart-time=120, IPv4/unicast F-bit=1
	grCap := buildGRCapTLV(0, 120, [][4]byte{{0x00, 0x01, 0x01, 0x80}})
	optParams := buildCapabilityParam(grCap)
	openBody := buildOpenBody(optParams)

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   "open",
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{RawBytes: openBody},
	}

	gp.handleStructuredEvent(se)

	gp.mu.Lock()
	cap, ok := gp.peerCaps["10.0.0.1"]
	gp.mu.Unlock()

	require.True(t, ok, "GR capability should be stored after structured open dispatch")
	assert.Equal(t, uint16(120), cap.RestartTime)
	require.Len(t, cap.Families, 1)
	assert.Equal(t, family.IPv4Unicast, cap.Families[0].Family)
	assert.True(t, cap.Families[0].ForwardState)
}

// TestHandleStructuredOpenNilRawBytes verifies that handleStructuredOpen with
// nil RawBytes does not crash and does not change state.
//
// VALIDATES: handleStructuredOpen returns early when RawBytes is nil.
// PREVENTS: Nil pointer dereference on structured events without wire data.
func TestHandleStructuredOpenNilRawBytes(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	msg := &bgptypes.RawMessage{RawBytes: nil}
	gp.handleStructuredOpen("10.0.0.1", msg)

	gp.mu.Lock()
	_, grOK := gp.peerCaps["10.0.0.1"]
	_, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, grOK, "no GR capability should be stored for nil RawBytes")
	assert.False(t, llgrOK, "no LLGR capability should be stored for nil RawBytes")
}

// TestHandleStructuredOpenMalformedBytes verifies graceful handling of malformed
// OPEN wire bytes (too short for UnpackOpen to parse).
//
// VALIDATES: handleStructuredOpen does not crash on truncated/malformed wire data.
// PREVENTS: Panic or error propagation from invalid wire data in structured events.
func TestHandleStructuredOpenMalformedBytes(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	tests := []struct {
		name     string
		rawBytes []byte
	}{
		{"empty bytes", []byte{}},
		{"too short for OPEN body", []byte{0x04, 0xFD, 0xE9}},
		{"exactly 9 bytes (one short)", []byte{0x04, 0xFD, 0xE9, 0x00, 0x5A, 0x01, 0x01, 0x01, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &bgptypes.RawMessage{RawBytes: tt.rawBytes}
			// Must not panic
			gp.handleStructuredOpen("10.0.0.1", msg)

			gp.mu.Lock()
			_, grOK := gp.peerCaps["10.0.0.1"]
			_, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
			gp.mu.Unlock()

			assert.False(t, grOK, "no GR cap stored for malformed bytes")
			assert.False(t, llgrOK, "no LLGR cap stored for malformed bytes")
		})
	}
}

// TestHandleStructuredOpenValidGR verifies that a valid OPEN with GR capability
// (code 64) populates peerCaps with correct restart-time and family data.
//
// VALIDATES: handleStructuredOpen extracts GR capability from raw OPEN wire bytes.
// PREVENTS: GR capability being lost when delivered via structured (DirectBridge) path.
func TestHandleStructuredOpenValidGR(t *testing.T) {
	tests := []struct {
		name        string
		flags       byte
		restartTime uint16
		families    [][4]byte // {AFI-high, AFI-low, SAFI, flags}
		wantTime    uint16
		wantFams    []struct {
			family family.Family
			fwdBit bool
		}
	}{
		{
			name:        "IPv4 unicast F-bit=1, restart=120",
			flags:       0,
			restartTime: 120,
			families:    [][4]byte{{0x00, 0x01, 0x01, 0x80}},
			wantTime:    120,
			wantFams: []struct {
				family family.Family
				fwdBit bool
			}{
				{family.IPv4Unicast, true},
			},
		},
		{
			name:        "IPv4+IPv6 unicast, restart=300",
			flags:       0,
			restartTime: 300,
			families: [][4]byte{
				{0x00, 0x01, 0x01, 0x80}, // ipv4/unicast F=1
				{0x00, 0x02, 0x01, 0x00}, // ipv6/unicast F=0
			},
			wantTime: 300,
			wantFams: []struct {
				family family.Family
				fwdBit bool
			}{
				{family.IPv4Unicast, true},
				{family.IPv6Unicast, false},
			},
		},
		{
			name:        "max restart-time (4095)",
			flags:       0,
			restartTime: 4095,
			families:    [][4]byte{{0x00, 0x01, 0x01, 0x80}},
			wantTime:    4095,
			wantFams: []struct {
				family family.Family
				fwdBit bool
			}{
				{family.IPv4Unicast, true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gp := &grPlugin{
				peerCaps:     make(map[string]*grPeerCap),
				peerLLGRCaps: make(map[string]*llgrPeerCap),
				state:        newGRStateManager(nil),
			}

			grCap := buildGRCapTLV(tt.flags, tt.restartTime, tt.families)
			optParams := buildCapabilityParam(grCap)
			openBody := buildOpenBody(optParams)

			msg := &bgptypes.RawMessage{RawBytes: openBody}
			gp.handleStructuredOpen("10.0.0.1", msg)

			gp.mu.Lock()
			cap, ok := gp.peerCaps["10.0.0.1"]
			gp.mu.Unlock()

			require.True(t, ok, "GR capability should be stored")
			assert.Equal(t, tt.wantTime, cap.RestartTime)
			require.Len(t, cap.Families, len(tt.wantFams))
			for i, wf := range tt.wantFams {
				assert.Equal(t, wf.family, cap.Families[i].Family)
				assert.Equal(t, wf.fwdBit, cap.Families[i].ForwardState)
			}
		})
	}
}

// TestHandleStructuredOpenValidLLGR verifies that a valid OPEN with LLGR capability
// (code 71) populates peerLLGRCaps with correct family and LLST data.
//
// VALIDATES: handleStructuredOpen extracts LLGR capability from raw OPEN wire bytes.
// PREVENTS: LLGR capability being lost when delivered via structured (DirectBridge) path.
func TestHandleStructuredOpenValidLLGR(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// GR: restart-time=120, ipv4/unicast F-bit=1
	grCap := buildGRCapTLV(0, 120, [][4]byte{{0x00, 0x01, 0x01, 0x80}})
	// LLGR: ipv4/unicast F-bit=1, LLST=3600 (0x000E10)
	llgrCap := buildLLGRCapTLV([][7]byte{{0x00, 0x01, 0x01, 0x80, 0x00, 0x0E, 0x10}})
	optParams := buildCapabilityParam(grCap, llgrCap)
	openBody := buildOpenBody(optParams)

	msg := &bgptypes.RawMessage{RawBytes: openBody}
	gp.handleStructuredOpen("10.0.0.1", msg)

	gp.mu.Lock()
	grOK := gp.peerCaps["10.0.0.1"]
	llgrCapResult, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	require.NotNil(t, grOK, "GR capability should be stored")
	require.True(t, llgrOK, "LLGR capability should be stored")
	require.Len(t, llgrCapResult.Families, 1)
	assert.Equal(t, family.IPv4Unicast, llgrCapResult.Families[0].Family)
	assert.True(t, llgrCapResult.Families[0].ForwardState)
	assert.Equal(t, uint32(3600), llgrCapResult.Families[0].LLST)
}

// TestHandleStructuredOpenLLGRNoGR verifies that LLGR is deleted when GR capability
// is not present in the OPEN, per RFC 9494.
//
// VALIDATES: RFC 9494: LLGR MUST be ignored if GR capability is not present.
// PREVENTS: LLGR state created for peers that only advertise cap 71 without cap 64.
func TestHandleStructuredOpenLLGRNoGR(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// Only LLGR capability (code 71), no GR (code 64)
	llgrCap := buildLLGRCapTLV([][7]byte{{0x00, 0x01, 0x01, 0x80, 0x00, 0x0E, 0x10}})
	optParams := buildCapabilityParam(llgrCap)
	openBody := buildOpenBody(optParams)

	msg := &bgptypes.RawMessage{RawBytes: openBody}
	gp.handleStructuredOpen("10.0.0.1", msg)

	gp.mu.Lock()
	_, grOK := gp.peerCaps["10.0.0.1"]
	_, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, grOK, "no GR capability should be stored")
	assert.False(t, llgrOK, "LLGR should be discarded when GR is absent (RFC 9494)")
}

// TestExtractGRCapsTruncatedData verifies that extractGRCaps handles truncated
// capability data gracefully without crashing.
//
// VALIDATES: extractGRCaps stops walking when data is too short for a complete TLV.
// PREVENTS: Out-of-bounds read on malformed capability bytes inside optional params.
func TestExtractGRCapsTruncatedData(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	tests := []struct {
		name   string
		data   []byte
		wantGR bool
	}{
		{
			name:   "empty data",
			data:   []byte{},
			wantGR: false,
		},
		{
			name:   "single byte (no length)",
			data:   []byte{64},
			wantGR: false,
		},
		{
			name:   "code+length but truncated value",
			data:   []byte{64, 6, 0x00, 0x78}, // claims 6 bytes of value but only 2 present
			wantGR: false,
		},
		{
			name:   "GR code with value too short for decodeGR",
			data:   []byte{64, 1, 0x00}, // 1 byte value, decodeGR needs >= 2
			wantGR: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Must not panic
			result := gp.extractGRCaps("10.0.0.1", tt.data, false)
			assert.Equal(t, tt.wantGR, result, "foundGR mismatch")

			gp.mu.Lock()
			_, ok := gp.peerCaps["10.0.0.1"]
			gp.mu.Unlock()
			assert.False(t, ok, "no GR cap should be stored for truncated data")
		})
	}
}

// TestHandleStructuredOpenGRPlusLLGR verifies that an OPEN with both GR and LLGR
// capabilities correctly populates both peerCaps and peerLLGRCaps.
//
// VALIDATES: handleStructuredOpen stores both GR and LLGR caps from a single OPEN.
// PREVENTS: Second capability overwriting or preventing storage of the first.
func TestHandleStructuredOpenGRPlusLLGR(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// GR: restart-time=240, IPv4/unicast F=1, IPv6/unicast F=1
	grCap := buildGRCapTLV(0, 240, [][4]byte{
		{0x00, 0x01, 0x01, 0x80}, // ipv4/unicast F=1
		{0x00, 0x02, 0x01, 0x80}, // ipv6/unicast F=1
	})
	// LLGR: IPv4/unicast LLST=7200, IPv6/unicast LLST=3600
	llgrCap := buildLLGRCapTLV([][7]byte{
		{0x00, 0x01, 0x01, 0x80, 0x00, 0x1C, 0x20}, // ipv4/unicast F=1, LLST=7200
		{0x00, 0x02, 0x01, 0x80, 0x00, 0x0E, 0x10}, // ipv6/unicast F=1, LLST=3600
	})
	optParams := buildCapabilityParam(grCap, llgrCap)
	openBody := buildOpenBody(optParams)

	msg := &bgptypes.RawMessage{RawBytes: openBody}
	gp.handleStructuredOpen("10.0.0.1", msg)

	gp.mu.Lock()
	grResult, grOK := gp.peerCaps["10.0.0.1"]
	llgrResult, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	// Verify GR
	require.True(t, grOK, "GR capability should be stored")
	assert.Equal(t, uint16(240), grResult.RestartTime)
	require.Len(t, grResult.Families, 2)
	assert.Equal(t, family.IPv4Unicast, grResult.Families[0].Family)
	assert.True(t, grResult.Families[0].ForwardState)
	assert.Equal(t, family.IPv6Unicast, grResult.Families[1].Family)
	assert.True(t, grResult.Families[1].ForwardState)

	// Verify LLGR
	require.True(t, llgrOK, "LLGR capability should be stored")
	require.Len(t, llgrResult.Families, 2)
	assert.Equal(t, family.IPv4Unicast, llgrResult.Families[0].Family)
	assert.True(t, llgrResult.Families[0].ForwardState)
	assert.Equal(t, uint32(7200), llgrResult.Families[0].LLST)
	assert.Equal(t, family.IPv6Unicast, llgrResult.Families[1].Family)
	assert.True(t, llgrResult.Families[1].ForwardState)
	assert.Equal(t, uint32(3600), llgrResult.Families[1].LLST)
}

// TestHandleStructuredEventNonOpenType verifies that handleStructuredEvent ignores
// event types other than "state" and "open" (EOR is handled via text OnEvent path).
//
// VALIDATES: handleStructuredEvent does not dispatch unknown event types.
// PREVENTS: Unexpected side effects from unrecognized structured event types.
func TestHandleStructuredEventNonOpenType(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   "keepalive",
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{RawBytes: []byte{0x04}},
	}

	// Must not panic or change state
	gp.handleStructuredEvent(se)

	gp.mu.Lock()
	_, grOK := gp.peerCaps["10.0.0.1"]
	_, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, grOK, "keepalive event should not store GR cap")
	assert.False(t, llgrOK, "keepalive event should not store LLGR cap")
}

// TestHandleStructuredEventOpenWrongRawMessageType verifies that handleStructuredEvent
// handles a non-*bgptypes.RawMessage RawMessage gracefully.
//
// VALIDATES: handleStructuredEvent type-asserts RawMessage safely.
// PREVENTS: Panic when RawMessage is set but is not *bgptypes.RawMessage.
func TestHandleStructuredEventOpenWrongRawMessageType(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   "open",
		Direction:   rpc.DirectionReceived,
		RawMessage:  "not a RawMessage", // wrong type
	}

	// Must not panic
	gp.handleStructuredEvent(se)

	gp.mu.Lock()
	_, grOK := gp.peerCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, grOK, "wrong RawMessage type should not store GR cap")
}

// TestHandleStructuredOpenNoCapabilities verifies that an OPEN with no optional
// parameters produces no GR/LLGR state.
//
// VALIDATES: handleStructuredOpen handles OPEN messages without any capabilities.
// PREVENTS: False positives from OPEN messages without GR support.
func TestHandleStructuredOpenNoCapabilities(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// OPEN with zero-length optional parameters
	openBody := buildOpenBody(nil)

	msg := &bgptypes.RawMessage{RawBytes: openBody}
	gp.handleStructuredOpen("10.0.0.1", msg)

	gp.mu.Lock()
	_, grOK := gp.peerCaps["10.0.0.1"]
	_, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, grOK, "no GR cap should be stored without capabilities")
	assert.False(t, llgrOK, "no LLGR cap should be stored without capabilities")
}

// TestHandleStructuredOpenNonCapabilityParam verifies that optional parameters
// that are NOT type 2 (capabilities) are skipped without error.
//
// VALIDATES: handleStructuredOpen skips non-capability optional parameters.
// PREVENTS: Non-capability parameters being misinterpreted as capability data.
func TestHandleStructuredOpenNonCapabilityParam(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// Optional parameter with type 1 (Authentication, deprecated), not type 2
	authParam := []byte{0x01, 0x02, 0xAA, 0xBB} // type=1, len=2, data=AABB
	openBody := buildOpenBody(authParam)

	msg := &bgptypes.RawMessage{RawBytes: openBody}
	gp.handleStructuredOpen("10.0.0.1", msg)

	gp.mu.Lock()
	_, grOK := gp.peerCaps["10.0.0.1"]
	_, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, grOK, "non-capability param should not produce GR cap")
	assert.False(t, llgrOK, "non-capability param should not produce LLGR cap")
}

// TestHandleStructuredOpenLLGRDeletedWhenPreExisting verifies that pre-existing
// LLGR state is cleaned up when the new OPEN lacks GR capability (RFC 9494).
//
// VALIDATES: handleStructuredOpen deletes pre-existing peerLLGRCaps when GR absent.
// PREVENTS: Stale LLGR state surviving across OPEN messages without GR.
func TestHandleStructuredOpenLLGRDeletedWhenPreExisting(t *testing.T) {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		state:        newGRStateManager(nil),
	}

	// Pre-populate LLGR state (as if a previous OPEN had both GR+LLGR)
	gp.peerLLGRCaps["10.0.0.1"] = &llgrPeerCap{
		Families: []llgrCapFamily{{Family: family.IPv4Unicast, ForwardState: true, LLST: 3600}},
	}

	// New OPEN has only a multiprotocol capability (code 1), no GR or LLGR
	mpCap := []byte{0x01, 0x04, 0x00, 0x01, 0x00, 0x01} // code=1, len=4, IPv4/unicast
	optParams := buildCapabilityParam(mpCap)
	openBody := buildOpenBody(optParams)

	msg := &bgptypes.RawMessage{RawBytes: openBody}
	gp.handleStructuredOpen("10.0.0.1", msg)

	gp.mu.Lock()
	_, llgrOK := gp.peerLLGRCaps["10.0.0.1"]
	gp.mu.Unlock()

	assert.False(t, llgrOK, "pre-existing LLGR should be deleted when new OPEN has no GR")
}
