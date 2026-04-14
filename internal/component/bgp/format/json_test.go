package format

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	evpn "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/evpn"
	flowspec "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/flowspec"
	labeled "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/labeled"
	vpn "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/vpn"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// getBGPPayload extracts the bgp payload from ze-bgp JSON format.
// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"<event>"},"peer":{...},...}}.
func getBGPPayload(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	assert.Equal(t, "bgp", result["type"], "top-level type must be 'bgp'")
	payload, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "bgp payload must exist")
	return payload
}

// getEventType extracts the event type from bgp.message.type in ze-bgp JSON format.
func getEventType(t *testing.T, result map[string]any) string {
	t.Helper()
	bgpPayload := getBGPPayload(t, result)
	msgObj, ok := bgpPayload["message"].(map[string]any)
	require.True(t, ok, "bgp.message must be object")
	eventType, ok := msgObj["type"].(string)
	require.True(t, ok, "bgp.message.type must be string")
	return eventType
}

// getEventPayload extracts the event-specific payload from ze-bgp JSON format.
// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"<event>"},"peer":{...},"<event>":{...}}}.
// For state events: data is at bgp level (state is a simple string, not container).
// For UPDATE events: data is nested under "update" key (update container).
// For other events: returns the inner payload nested under the event type key.
func getEventPayload(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	bgpPayload := getBGPPayload(t, result)
	eventType := getEventType(t, result)

	// State events have data at bgp level (state is a simple string, not container)
	if eventType == "state" {
		return bgpPayload
	}

	// All other events (including update) have data nested under the event type key
	eventPayload, ok := bgpPayload[eventType].(map[string]any)
	require.True(t, ok, "bgp.%s must be object", eventType)
	return eventPayload
}

// getNLRI extracts the nlri object from event payload.
func getNLRI(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	nlri, ok := payload["nlri"].(map[string]any)
	require.True(t, ok, "nlri object must exist in payload")
	return nlri
}

// TestJSONEncoderStateUp verifies JSON output for peer state "up".
//
// VALIDATES: ze-bgp JSON format for state messages.
// PREVENTS: JSON format incompatibility with consumers expecting ze-bgp JSON.
func TestJSONEncoderStateUp(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	msg := enc.StateUp(&peer)

	// Parse JSON to verify structure
	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err, "JSON must be valid")

	// ze-bgp JSON: get bgp payload and verify type in message
	eventType := getEventType(t, result)
	assert.Equal(t, "state", eventType)

	// Get event-specific payload
	payload := getEventPayload(t, result)

	// Check peer structure
	peerMap, ok := payload["peer"].(map[string]any)
	require.True(t, ok, "peer must be object")
	assert.Equal(t, "192.168.1.2", peerMap["address"])
	remoteMap, ok := peerMap["remote"].(map[string]any)
	require.True(t, ok, "peer.remote must be object")
	assert.Equal(t, float64(65002), remoteMap["as"])

	// State in payload
	assert.Equal(t, "up", payload["state"])
}

// TestJSONEncoderStateDown verifies JSON output for peer state "down".
//
// VALIDATES: ze-bgp JSON format with reason field in event payload.
// PREVENTS: Missing reason in down notifications, making debugging harder.
func TestJSONEncoderStateDown(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	msg := enc.StateDown(&peer, "hold timer expired")

	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err)

	// ze-bgp JSON: get event payload
	payload := getEventPayload(t, result)

	// State and reason in payload
	assert.Equal(t, "down", payload["state"])
	assert.Equal(t, "hold timer expired", payload["reason"])
}

// TestJSONEncoderStateConnected verifies JSON output for "connected" state.
//
// VALIDATES: ze-bgp JSON format with connected state in event payload.
// PREVENTS: Missing TCP connection events in event stream.
func TestJSONEncoderStateConnected(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65000,
		PeerAS:       65001,
	}

	msg := enc.StateConnected(&peer)

	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err)

	// ze-bgp JSON: get event payload
	payload := getEventPayload(t, result)

	// State in payload
	assert.Equal(t, "connected", payload["state"])
}

// TestJSONEncoderValidJSON verifies output is always valid JSON.
//
// VALIDATES: All encoder methods produce valid JSON.
//
// PREVENTS: Parse errors in consumers due to malformed JSON.
func TestJSONEncoderValidJSON(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	messages := []string{
		enc.StateUp(&peer),
		enc.StateDown(&peer, "test reason"),
		enc.StateConnected(&peer),
		enc.StateDown(&peer, `reason with "quotes" and \backslashes`),
	}

	for i, msg := range messages {
		var result map[string]any
		err := json.Unmarshal([]byte(msg), &result)
		assert.NoError(t, err, "message %d must be valid JSON: %s", i, msg)
	}
}

// TestJSONEncoderSpecialCharacters verifies proper escaping.
//
// VALIDATES: Special characters in strings are properly escaped.
// PREVENTS: JSON injection or parse errors from special characters.
func TestJSONEncoderSpecialCharacters(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// Reason with special characters
	reason := `connection reset: "peer closed" with \n newline`
	msg := enc.StateDown(&peer, reason)

	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err, "JSON with special chars must be valid")

	// ze-bgp JSON: reason in event payload
	payload := getEventPayload(t, result)
	assert.Contains(t, payload["reason"], "peer closed")
}

// TestJSONEncoderIPv6 verifies IPv6 address formatting.
//
// VALIDATES: IPv6 addresses are formatted correctly in ze-bgp JSON format.
// PREVENTS: IPv6 address parsing errors in consumers.
func TestJSONEncoderIPv6(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("2001:db8::1"),
		LocalAddress: netip.MustParseAddr("2001:db8::2"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	msg := enc.StateUp(&peer)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg), &result))

	// ze-bgp JSON: get event payload
	payload := getEventPayload(t, result)

	peerMap, ok := payload["peer"].(map[string]any)
	require.True(t, ok)

	// peer.address is string
	peerAddr, ok := peerMap["address"].(string)
	require.True(t, ok, "peer.address must be string")
	assert.Contains(t, peerAddr, "2001:db8")
}

// TestAPIOutputIncludesMsgID verifies API JSON has message.id field in ze-bgp JSON format.
//
// VALIDATES: API output contains id in message wrapper for received UPDATEs.
// PREVENTS: Controller can't reference updates for forwarding.
func TestAPIOutputIncludesMsgID(t *testing.T) {
	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// UPDATE with msg-id
	msg := bgptypes.RawMessage{
		Type: message.TypeUPDATE,
		RawBytes: []byte{
			// Minimal UPDATE with NLRI: 10.0.0.0/24
			0x00, 0x00, // withdrawn length
			0x00, 0x00, // attrs length
			0x18, 0x0a, 0x00, 0x00, // NLRI: 10.0.0.0/24
		},
		MessageID: 12345,
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	// Parse JSON
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: message is at bgp.message level
	bgpPayload := getBGPPayload(t, result)
	msgWrapper, ok := bgpPayload["message"].(map[string]any)
	require.True(t, ok, "message wrapper must be present in bgp payload")
	msgID, ok := msgWrapper["id"]
	require.True(t, ok, "message.id must be present")
	assert.Equal(t, float64(12345), msgID)
}

// TestJSONEncoderNotification verifies JSON output for NOTIFICATION message.
//
// VALIDATES: ze-bgp JSON NOTIFICATION with code, subcode, data in bgp payload.
// PREVENTS: Plugin can't parse notification events or missing error context.
func TestJSONEncoderNotification(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	notify := DecodedNotification{
		ErrorCode:        6, // Cease
		ErrorSubcode:     2, // Administrative Shutdown
		ErrorCodeName:    "Cease",
		ErrorSubcodeName: "Administrative Shutdown",
		ShutdownMessage:  "maintenance window",
		Data:             []byte{0x12, 0x6d, 0x61, 0x69, 0x6e, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x20, 0x77, 0x69, 0x6e, 0x64, 0x6f, 0x77},
	}

	msg := enc.Notification(&peer, notify, "received", 42)

	// Parse JSON to verify structure
	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err, "JSON must be valid: %s", msg)

	// ze-bgp JSON: get event payload
	payload := getEventPayload(t, result)

	// Check event type in bgp.message
	eventType := getEventType(t, result)
	assert.Equal(t, "notification", eventType)

	// Check message metadata (in bgp.message)
	bgpPayload := getBGPPayload(t, result)
	msgMeta, ok := bgpPayload["message"].(map[string]any)
	require.True(t, ok, "message metadata must exist")
	assert.Equal(t, float64(42), msgMeta["id"])
	assert.Equal(t, "received", msgMeta["direction"])

	// Check peer structure (at bgp level, not inside notification)
	peerMap, ok := bgpPayload["peer"].(map[string]any)
	require.True(t, ok, "peer must be object at bgp level")
	assert.Equal(t, "192.168.1.2", peerMap["address"])
	remoteMap, ok := peerMap["remote"].(map[string]any)
	require.True(t, ok, "peer.remote must be object")
	assert.Equal(t, float64(65002), remoteMap["as"])

	// Notification fields in payload
	assert.Equal(t, float64(6), payload["code"])
	assert.Equal(t, float64(2), payload["subcode"])
	assert.NotEmpty(t, payload["data"], "data field must be present")

	// Human-readable names (hyphenated per json-format.md)
	assert.Equal(t, "Cease", payload["code-name"])
	assert.Equal(t, "Administrative Shutdown", payload["subcode-name"])
}

// TestJSONEncoderNotificationMinimal verifies minimal NOTIFICATION JSON.
//
// VALIDATES: ze-bgp JSON NOTIFICATION without shutdown message still has required fields.
// PREVENTS: Crash or missing fields on minimal notifications.
func TestJSONEncoderNotificationMinimal(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	notify := DecodedNotification{
		ErrorCode:        4, // Hold Timer Expired
		ErrorSubcode:     0,
		ErrorCodeName:    "Hold Timer Expired",
		ErrorSubcodeName: "Unspecific",
		Data:             nil, // No data
	}

	msg := enc.Notification(&peer, notify, "received", 0)

	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err)

	// ze-bgp JSON: get event payload
	payload := getEventPayload(t, result)

	// Required fields in payload
	assert.Equal(t, float64(4), payload["code"])
	assert.Equal(t, float64(0), payload["subcode"])
	assert.Equal(t, "", payload["data"]) // Empty string, not nil

	// Extensions present (hyphenated)
	assert.Equal(t, "Hold Timer Expired", payload["code-name"])

	// message metadata should exist (has type and direction)
	bgpPayload := getBGPPayload(t, result)
	msgMeta, ok := bgpPayload["message"].(map[string]any)
	require.True(t, ok, "message metadata should exist")
	assert.Equal(t, "notification", msgMeta["type"])
	_, hasID := msgMeta["id"]
	assert.False(t, hasID, "message.id should not be present when zero")
}

// TestJSONEncoderNotificationSent verifies NOTIFICATION with "sent" direction.
//
// VALIDATES: ze-bgp JSON sent notifications include direction in message metadata.
// PREVENTS: Direction field missing or incorrect for outbound notifications.
func TestJSONEncoderNotificationSent(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	notify := DecodedNotification{
		ErrorCode:        6,
		ErrorSubcode:     3, // Peer De-configured
		ErrorCodeName:    "Cease",
		ErrorSubcodeName: "Peer De-configured",
	}

	msg := enc.Notification(&peer, notify, "sent", 100)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg), &result))

	// ze-bgp JSON: get event payload
	payload := getEventPayload(t, result)

	// Notification fields in payload
	assert.Equal(t, float64(6), payload["code"])
	assert.Equal(t, float64(3), payload["subcode"])

	// message metadata is at bgp.message level
	bgpPayload := getBGPPayload(t, result)
	msgMeta, ok := bgpPayload["message"].(map[string]any)
	require.True(t, ok, "message metadata must exist")
	assert.Equal(t, float64(100), msgMeta["id"])
	assert.Equal(t, "sent", msgMeta["direction"])
}

// TestFormatMessageNotificationText verifies FormatMessage returns text for NOTIFICATION.
//
// NOTE: FormatMessage only returns TEXT for non-UPDATE messages. For JSON format,
// Server.formatMessage() should be used (see TestServerFormatMessageNotificationJSON).
//
// VALIDATES: FormatMessage returns parseable text for NOTIFICATION.
// PREVENTS: Crashes or malformed output from FormatMessage with NOTIFICATION.
func TestFormatMessageNotificationText_Parsed(t *testing.T) {
	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// NOTIFICATION: Cease/Administrative Shutdown with message "goodbye"
	rawBytes := []byte{
		0x06,                              // Error code: Cease (6)
		0x02,                              // Subcode: Administrative Shutdown (2)
		0x07,                              // Message length: 7
		'g', 'o', 'o', 'd', 'b', 'y', 'e', // "goodbye"
	}

	msg := bgptypes.RawMessage{
		Type:      message.TypeNOTIFICATION,
		RawBytes:  rawBytes,
		MessageID: 42,
		Direction: "received",
	}

	// Even with plugin.EncodingJSON, FormatMessage returns text for non-UPDATE
	// This is by design - JSON encoding for non-UPDATE uses Server.formatMessage()
	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingText,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	// Verify text format
	assert.Contains(t, output, "peer 10.0.0.1")
	assert.Contains(t, output, "received")
	assert.Contains(t, output, "notification")
	assert.Contains(t, output, "42")     // msg-id
	assert.Contains(t, output, "code 6") // error code
	assert.Contains(t, output, "subcode 2")
	assert.Contains(t, output, "Cease")
}

// TestFormatMessageIgnoresEncodingForParsedNonUpdate documents that FormatMessage
// ignores Encoding for parsed non-UPDATE messages.
//
// This is by design: Server.formatMessage() handles JSON encoding for non-UPDATE
// messages using the shared JSONEncoder with proper counter semantics.
//
// VALIDATES: FormatMessage with plugin.EncodingJSON + plugin.FormatParsed + NOTIFICATION returns TEXT.
// PREVENTS: Confusion about why JSON encoding is ignored.
func TestFormatMessageIgnoresEncodingForParsedNonUpdate(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65002,
	}

	msg := bgptypes.RawMessage{
		Type:      message.TypeNOTIFICATION,
		RawBytes:  []byte{0x04, 0x00}, // Hold Timer Expired
		MessageID: 1,
		Direction: "received",
	}

	// Request JSON encoding with parsed format
	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON, // Requested JSON...
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	// ...but we get TEXT because FormatMessage ignores Encoding for parsed non-UPDATE
	assert.True(t, strings.HasPrefix(output, "peer "),
		"Expected text format starting with 'peer ', got: %s", output)
	assert.False(t, strings.HasPrefix(output, "{"),
		"Should NOT be JSON for parsed non-UPDATE")
}

// TestAPIOutputNoMsgIDWhenZero verifies message.id omitted when zero in ze-bgp JSON format.
//
// VALIDATES: Zero message.id is not included in output.
// PREVENTS: Cluttering output with meaningless id:0.
func TestAPIOutputNoMsgIDWhenZero(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65002,
	}

	msg := bgptypes.RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  []byte{0x00, 0x00, 0x00, 0x00}, // Empty UPDATE
		MessageID: 0,                              // No msg-id
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	// Parse JSON
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// ze-bgp JSON: payload under "bgp"
	payload := getBGPPayload(t, result)

	// message wrapper may or may not exist when id is zero
	// If it exists, id should NOT be present
	if msgWrapper, ok := payload["message"].(map[string]any); ok {
		_, hasID := msgWrapper["id"]
		assert.False(t, hasID, "message.id should not be present when zero")
	}
}

// =============================================================================
// New NLRI Format Tests (Command-Style)
// =============================================================================
// These tests validate the ze-bgp JSON JSON format for UPDATE NLRI:
//   - Top-level "type":"bgp" with payload under "bgp" key
//   - Family under "nlri" object (not at top level)
//   - Attributes under "attr" object
//   - Operations array with "action": "add" or "action": "del"
//   - "next-hop" per operation group (for add actions)
//   - "nlri" array (strings for simple prefixes)
//
// RFC 4271 Section 5.1.3: NEXT_HOP defines the IP address of the router
// that SHOULD be used as next hop to the destinations.
// RFC 4760 Section 3: Each MP_REACH_NLRI has its own next-hop field.
// =============================================================================

// TestJSONEncoderIPv4UnicastNewFormat verifies ze-bgp JSON command-style JSON format.
//
// VALIDATES: IPv4 unicast uses bgp.family.family → operations list with action/next-hop/nlri.
// PREVENTS: Old format without type wrapper being emitted.
func TestJSONEncoderIPv4UnicastNewFormat(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with IPv4 NLRI: 192.168.1.0/24
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	// Parse JSON
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: get event payload and nlri object
	payload := getEventPayload(t, result)
	nlriObj := getNLRI(t, payload)

	// Family should be under nlri
	fam, ok := nlriObj["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array under nlri: %s", output)
	require.Len(t, fam, 1, "should have one operation group")

	// Check operation structure
	op, ok := fam[0].(map[string]any)
	require.True(t, ok, "operation should be map")
	assert.Equal(t, "add", op["action"], "action should be 'add'")
	assert.Equal(t, "10.0.0.1", op["next-hop"], "next-hop should be present")

	// Check nlri array
	nlri, ok := op["nlri"].([]any)
	require.True(t, ok, "nlri should be array")
	require.Len(t, nlri, 1)
	assert.Equal(t, "192.168.1.0/24", nlri[0])
}

// TestJSONEncoderWithdrawNewFormat verifies withdrawal uses action: del.
//
// VALIDATES: ze-bgp JSON withdrawals use action: del with no next-hop.
// PREVENTS: Withdraw still using old nested format.
func TestJSONEncoderWithdrawNewFormat(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with withdrawn routes only
	body := buildTestUpdateBodyWithWithdrawn(
		netip.MustParsePrefix("172.16.0.0/16"),
	)

	wireUpdate := wireu.NewWireUpdate(body, testEncodingContext())
	attrsWire, _ := wireUpdate.Attrs()

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: get event payload and nlri object
	payload := getEventPayload(t, result)
	nlriObj := getNLRI(t, payload)

	// Family under nlri with action: del
	fam, ok := nlriObj["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array: %s", output)
	require.Len(t, fam, 1)

	op, ok := fam[0].(map[string]any)
	require.True(t, ok, "operation should be map")
	assert.Equal(t, "del", op["action"])
	_, hasNextHop := op["next-hop"]
	assert.False(t, hasNextHop, "del operations should NOT have next-hop")

	nlri, ok := op["nlri"].([]any)
	require.True(t, ok)
	assert.Equal(t, "172.16.0.0/16", nlri[0])
}

// TestJSONEncoderMultiFamilyNewFormat verifies multiple families in one UPDATE.
//
// VALIDATES: ze-bgp JSON each family appears under nlri with own operations.
// PREVENTS: Multi-family UPDATEs breaking JSON structure.
func TestJSONEncoderMultiFamilyNewFormat(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with both IPv4 and IPv6 NLRI
	body := buildTestUpdateBodyWithBothFamilies(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParseAddr("2001:db8::1"),
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: get event payload and nlri object
	payload := getEventPayload(t, result)
	nlriObj := getNLRI(t, payload)

	// Both families should be under nlri
	ipv4Fam, ok := nlriObj["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be present: %s", output)
	require.Len(t, ipv4Fam, 1)

	ipv6Fam, ok := nlriObj["ipv6/unicast"].([]any)
	require.True(t, ok, "ipv6/unicast should be present: %s", output)
	require.Len(t, ipv6Fam, 1)

	// Check IPv4 operation
	ipv4Op, ok := ipv4Fam[0].(map[string]any)
	require.True(t, ok, "ipv4 operation should be map")
	assert.Equal(t, "add", ipv4Op["action"])
	assert.Equal(t, "10.0.0.1", ipv4Op["next-hop"])

	// Check IPv6 operation
	ipv6Op, ok := ipv6Fam[0].(map[string]any)
	require.True(t, ok, "ipv6 operation should be map")
	assert.Equal(t, "add", ipv6Op["action"])
	assert.Equal(t, "2001:db8::1", ipv6Op["next-hop"])
}

// TestJSONEncoderAnnounceAndWithdrawSameFamily verifies add + del in same family.
//
// VALIDATES: ze-bgp JSON same family can have both add and del operations.
// PREVENTS: Mixed announce/withdraw breaking JSON.
func TestJSONEncoderAnnounceAndWithdrawSameFamily(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with both NLRI and withdrawn
	body := buildTestUpdateBodyWithNLRIAndWithdrawn(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParsePrefix("172.16.0.0/16"),
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, _ := wireUpdate.Attrs()

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: get event payload and nlri object
	payload := getEventPayload(t, result)
	nlriObj := getNLRI(t, payload)

	// Family should have both add and del operations
	fam, ok := nlriObj["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array: %s", output)
	require.Len(t, fam, 2, "should have 2 operation groups (add + del)")

	// Find add and del operations
	var hasAdd, hasDel bool
	for _, item := range fam {
		op, ok := item.(map[string]any)
		require.True(t, ok, "operation should be map")
		switch op["action"] {
		case "add":
			hasAdd = true
			assert.Equal(t, "10.0.0.1", op["next-hop"])
		case "del":
			hasDel = true
			_, hasNH := op["next-hop"]
			assert.False(t, hasNH, "del should not have next-hop")
		}
	}
	assert.True(t, hasAdd, "should have add operation")
	assert.True(t, hasDel, "should have del operation")
}

// TestJSONEncoderADDPATHNewFormat verifies path-id in NLRI objects.
//
// VALIDATES: ze-bgp JSON ADD-PATH path-id appears in nlri objects.
// PREVENTS: Path-id being lost with new format.
func TestJSONEncoderADDPATHNewFormat(t *testing.T) {
	// Create encoding context with ADD-PATH for IPv4 unicast
	ctxID := testEncodingContextWithAddPath()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with path-id in NLRI
	body := buildTestUpdateBodyWithPathID(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		42, // path-id
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: get event payload and nlri object
	payload := getEventPayload(t, result)
	nlriObj := getNLRI(t, payload)

	// Family should be under nlri
	fam, ok := nlriObj["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array: %s", output)
	require.Len(t, fam, 1)

	op, ok := fam[0].(map[string]any)
	require.True(t, ok, "operation should be map")
	nlri, ok := op["nlri"].([]any)
	require.True(t, ok)
	require.Len(t, nlri, 1)

	// With ADD-PATH, nlri items are objects with prefix and path-id
	nlriItem, ok := nlri[0].(map[string]any)
	require.True(t, ok, "with ADD-PATH, nlri should be object: %v", nlri[0])
	assert.Equal(t, "192.168.1.0/24", nlriItem["prefix"])
	assert.Equal(t, float64(42), nlriItem["path-id"])
}

// buildTestUpdateBodyWithWithdrawn builds UPDATE with withdrawn routes only.
func buildTestUpdateBodyWithWithdrawn(prefix netip.Prefix) []byte {
	// Build withdrawn section
	var withdrawn []byte
	if prefix.Addr().Is4() {
		bits := prefix.Bits()
		withdrawn = append(withdrawn, byte(bits))
		prefixBytes := (bits + 7) / 8
		addr := prefix.Addr().As4()
		withdrawn = append(withdrawn, addr[:prefixBytes]...)
	}

	// Build body: withdrawn_len(2) + withdrawn + attr_len(2) + attrs (empty) + nlri (empty)
	body := make([]byte, 4+len(withdrawn))
	binary.BigEndian.PutUint16(body[0:2], uint16(len(withdrawn))) //nolint:gosec // test data
	copy(body[2:], withdrawn)
	binary.BigEndian.PutUint16(body[2+len(withdrawn):], 0) // attr len = 0

	return body
}

// buildTestUpdateBodyWithNLRIAndWithdrawn builds UPDATE with both NLRI and withdrawn.
func buildTestUpdateBodyWithNLRIAndWithdrawn(announcePrefix netip.Prefix, nextHop netip.Addr, withdrawPrefix netip.Prefix) []byte {
	// Build withdrawn section
	var withdrawn []byte
	if withdrawPrefix.Addr().Is4() {
		bits := withdrawPrefix.Bits()
		withdrawn = append(withdrawn, byte(bits))
		prefixBytes := (bits + 7) / 8
		addr := withdrawPrefix.Addr().As4()
		withdrawn = append(withdrawn, addr[:prefixBytes]...)
	}

	// Build attributes: ORIGIN (igp) + AS_PATH (empty)
	var attrs []byte
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00)

	// NEXT_HOP
	if nextHop.Is4() {
		b := nextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// Build NLRI
	var nlri []byte
	if announcePrefix.Addr().Is4() {
		bits := announcePrefix.Bits()
		prefixBytes := (bits + 7) / 8
		addr := announcePrefix.Addr().As4()
		nlri = append(nlri, byte(bits))
		nlri = append(nlri, addr[:prefixBytes]...)
	}

	// Build body
	body := make([]byte, 4+len(withdrawn)+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], uint16(len(withdrawn))) //nolint:gosec // test data
	copy(body[2:], withdrawn)
	binary.BigEndian.PutUint16(body[2+len(withdrawn):], uint16(len(attrs))) //nolint:gosec // test data
	copy(body[4+len(withdrawn):], attrs)
	copy(body[4+len(withdrawn)+len(attrs):], nlri)

	return body
}

// buildTestUpdateBodyWithPathID builds UPDATE with path-id in NLRI (ADD-PATH).
func buildTestUpdateBodyWithPathID(prefix netip.Prefix, nextHop netip.Addr, pathID uint32) []byte {
	// ORIGIN (igp) + AS_PATH (empty)
	var attrs []byte
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00)

	// NEXT_HOP
	if nextHop.Is4() {
		b := nextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// Build NLRI with path-id (ADD-PATH format: path-id(4) + prefix)
	var nlri []byte
	pathIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(pathIDBytes, pathID)
	nlri = append(nlri, pathIDBytes...)

	if prefix.Addr().Is4() {
		bits := prefix.Bits()
		nlri = append(nlri, byte(bits))
		prefixBytes := (bits + 7) / 8
		addr := prefix.Addr().As4()
		nlri = append(nlri, addr[:prefixBytes]...)
	}

	// Build body
	body := make([]byte, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs))) //nolint:gosec // test data
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlri)

	return body
}

// testEncodingContextWithAddPath creates an encoding context with ADD-PATH enabled.
func testEncodingContextWithAddPath() bgpctx.ContextID {
	addPath := map[family.Family]bool{
		{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true,
	}
	ctx := bgpctx.EncodingContextWithAddPath(true, addPath)
	return bgpctx.Registry.Register(ctx)
}

// TestJSONEncoderIPv4DualNextHop verifies handling of IPv4 with two next-hops.
//
// RFC 4760: One UPDATE can have:
//   - NEXT_HOP attribute (for IPv4 unicast NLRI section)
//   - MP_REACH_NLRI (AFI=1, SAFI=1) with different next-hop
//
// Both should appear as separate operation groups in the output.
//
// VALIDATES: ze-bgp JSON two next-hops for same family produce two operations.
// PREVENTS: Merging different next-hops, losing route information.
func TestJSONEncoderIPv4DualNextHop(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with:
	// - Legacy IPv4 NLRI (192.168.1.0/24) with NEXT_HOP attribute (10.0.0.1)
	// - MP_REACH_NLRI for IPv4/unicast (192.168.2.0/24) with next-hop (10.0.0.2)
	body := buildTestUpdateBodyWithDualIPv4NextHop(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"), // Legacy next-hop
		netip.MustParsePrefix("192.168.2.0/24"),
		netip.MustParseAddr("10.0.0.2"), // MP next-hop
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: get event payload and nlri object
	payload := getEventPayload(t, result)
	nlriObj := getNLRI(t, payload)

	// ipv4/unicast should have TWO operation groups (different next-hops)
	fam, ok := nlriObj["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array: %s", output)
	require.Len(t, fam, 2, "should have 2 operation groups (two next-hops)")

	// Collect next-hops and nlris
	nextHops := make(map[string][]string)
	for _, item := range fam {
		op, ok := item.(map[string]any)
		require.True(t, ok, "operation should be map")
		assert.Equal(t, "add", op["action"])

		nh, ok := op["next-hop"].(string)
		require.True(t, ok, "next-hop should be string")

		nlriArr, ok := op["nlri"].([]any)
		require.True(t, ok, "nlri should be array")

		for _, n := range nlriArr {
			if s, ok := n.(string); ok {
				nextHops[nh] = append(nextHops[nh], s)
			}
		}
	}

	// Verify both next-hops are present with correct NLRIs
	assert.Contains(t, nextHops, "10.0.0.1", "legacy next-hop should be present")
	assert.Contains(t, nextHops, "10.0.0.2", "MP next-hop should be present")
	assert.Contains(t, nextHops["10.0.0.1"], "192.168.1.0/24", "legacy NLRI")
	assert.Contains(t, nextHops["10.0.0.2"], "192.168.2.0/24", "MP NLRI")
}

// TestJSONEncoderRDTypes verifies RD format with type prefix in JSON output.
//
// RFC 4364 Section 4.2 defines three RD types:
//   - Type 0: 2-byte ASN + 4-byte assigned → "0:ASN:assigned"
//   - Type 1: 4-byte IP + 2-byte assigned → "1:IP:assigned"
//   - Type 2: 4-byte ASN + 2-byte assigned → "2:ASN:assigned"
//
// VALIDATES: RD includes type prefix for unambiguous parsing.
// PREVENTS: Type 0 and Type 2 being indistinguishable.
func TestJSONEncoderRDTypes(t *testing.T) {
	tests := []struct {
		name    string
		rdType  nlri.RDType
		rdValue [6]byte
		wantRD  string
	}{
		{
			name:    "type_0_2byte_asn",
			rdType:  nlri.RDType0,
			rdValue: [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}, // ASN=65000, assigned=100
			wantRD:  "0:65000:100",
		},
		{
			name:    "type_1_ip_address",
			rdType:  nlri.RDType1,
			rdValue: [6]byte{0xC0, 0x00, 0x02, 0x01, 0x00, 0x64}, // IP=192.0.2.1, assigned=100
			wantRD:  "1:192.0.2.1:100",
		},
		{
			name:    "type_2_4byte_asn",
			rdType:  nlri.RDType2,
			rdValue: [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x64}, // ASN=65536, assigned=100
			wantRD:  "2:65536:100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := nlri.RouteDistinguisher{Type: tt.rdType, Value: tt.rdValue}

			// Verify RD String() output
			assert.Equal(t, tt.wantRD, rd.String(), "RD should include type prefix")

			// Create IPVPN NLRI and verify JSON output
			vpnNLRI := vpn.NewVPN(
				family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
				rd,
				[]uint32{100}, // label
				netip.MustParsePrefix("10.0.0.0/24"),
				0, // no path-id
			)

			// Format using formatNLRIJSONValue (registry-based decode)
			var sb strings.Builder
			formatNLRIJSONValue(&sb, vpnNLRI, vpnNLRI.Family().String())
			output := sb.String()

			// Verify RD in JSON output
			assert.Contains(t, output, fmt.Sprintf(`"rd":%q`, tt.wantRD),
				"JSON should contain RD with type prefix")
		})
	}
}

// TestJSONEncoderEVPN verifies EVPN Type 2 (MAC/IP) NLRI JSON format.
//
// RFC 7432: EVPN NLRI includes route-type, RD, ESI, ethernet-tag, MAC, IP, labels.
// JSON format: {"route-type":"mac-ip-advertisement","rd":"0:65000:1","esi":"00:...","mac":"..."}
//
// VALIDATES: EVPN fields appear in structured JSON format.
// PREVENTS: ESI, MAC, or route-type being lost.
func TestJSONEncoderEVPN(t *testing.T) {
	// Create EVPNType2 (MAC/IP Advertisement) by parsing wire format
	// RFC 7432 Section 7.2 wire format:
	// RD (8) + ESI (10) + EthTag (4) + MACLen (1) + MAC (6) + IPLen (1) + IP (4) + Labels (3)

	// RD: Type 0, ASN 65000, assigned 100
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	// ESI: all zeros (single-homed)
	esi := make([]byte, 10)
	// Ethernet Tag: 0
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	// MAC: 00:11:22:33:44:55
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	// IP: 10.0.0.1
	ip := []byte{0x0A, 0x00, 0x00, 0x01}
	// Label: 100 (with BOS)
	label := []byte{0x00, 0x06, 0x41} // label 100, BOS=1

	// Create EVPN Type 2 directly for testing JSON formatter
	evpnNLRI := createTestEVPNType2(rd, esi, ethTag, mac, ip, label)

	// Format using formatNLRIJSONValue (registry-based decode)
	var sb strings.Builder
	formatNLRIJSONValue(&sb, evpnNLRI, evpnNLRI.Family().String())
	output := sb.String()

	// Verify all fields (plugin decode format: array with code/name/parsed/raw fields)
	assert.Contains(t, output, `"name":"MAC/IP advertisement"`, "route type name")
	assert.Contains(t, output, `"rd":"0:65000:100"`, "RD with type prefix")
	assert.Contains(t, output, `"ethernet-tag":0`, "ethernet-tag")
	assert.Contains(t, output, `"mac":"00:11:22:33:44:55"`, "MAC")
	assert.Contains(t, output, `"ip":"10.0.0.1"`, "IP")
	assert.Contains(t, output, `"label":[[100]]`, "labels (nested array)")

	// Verify valid JSON (plugin returns array)
	var parsed []any
	err := json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err, "Output should be valid JSON: %s", output)
	require.Len(t, parsed, 1, "should contain one EVPN route")
}

// createTestEVPNType2 creates an EVPNType2 for testing by parsing wire format.
func createTestEVPNType2(rdBytes, esiBytes, ethTagBytes, macBytes, ipBytes, labelBytes []byte) *evpn.EVPNType2 {
	// Build the full wire data that parseEVPNType2 expects
	// Preallocate: RD + ESI + EthTag + MACLen(1) + MAC + IPLen(1) + IP + Labels
	dataLen := len(rdBytes) + len(esiBytes) + len(ethTagBytes) + 1 + len(macBytes) + 1 + len(ipBytes) + len(labelBytes)
	data := make([]byte, 0, dataLen)
	data = append(data, rdBytes...)
	data = append(data, esiBytes...)
	data = append(data, ethTagBytes...)
	data = append(data, byte(48)) // MAC length in bits
	data = append(data, macBytes...)
	data = append(data, byte(32)) // IP length in bits
	data = append(data, ipBytes...)
	data = append(data, labelBytes...)

	// Build full EVPN NLRI with type and length header (preallocate: 2 + data)
	evpnData := make([]byte, 0, 2+len(data))
	evpnData = append(evpnData, byte(evpn.EVPNRouteType2), byte(len(data)))
	evpnData = append(evpnData, data...)

	// Parse using the public ParseEVPN function
	parsed, _, err := evpn.ParseEVPN(evpnData, false)
	if err != nil {
		panic(fmt.Sprintf("failed to parse test EVPN: %v", err))
	}

	evpn, ok := parsed.(*evpn.EVPNType2)
	if !ok {
		panic(fmt.Sprintf("expected *EVPNType2, got %T", parsed))
	}

	return evpn
}

// TestJSONEncoderLabeledUnicast verifies labeled unicast NLRI JSON format.
//
// RFC 8277: Labeled Unicast NLRI includes MPLS labels.
// JSON format: {"prefix":"10.0.0.0/24", "labels":[100]}
//
// VALIDATES: Labels appear in structured JSON format.
// PREVENTS: Labels being lost or incorrectly formatted.
func TestJSONEncoderLabeledUnicast(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		labels     []uint32
		pathID     uint32
		wantLabels string
		wantPathID bool
	}{
		{
			name:       "single_label",
			prefix:     "10.0.0.0/24",
			labels:     []uint32{100},
			pathID:     0,
			wantLabels: `"labels":[100]`,
			wantPathID: false,
		},
		{
			name:       "label_stack",
			prefix:     "192.168.1.0/24",
			labels:     []uint32{100, 200},
			pathID:     0,
			wantLabels: `"labels":[100,200]`,
			wantPathID: false,
		},
		{
			name:       "with_path_id",
			prefix:     "172.16.0.0/16",
			labels:     []uint32{300},
			pathID:     42,
			wantLabels: `"labels":[300]`,
			wantPathID: false, // path-id is transport-level (ADD-PATH), not in decoder output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create LabeledUnicast NLRI
			lu := labeled.NewLabeledUnicast(
				family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
				netip.MustParsePrefix(tt.prefix),
				tt.labels,
				tt.pathID,
			)

			// Format using formatNLRIJSONValue (routes through registry decoder)
			var sb strings.Builder
			formatNLRIJSONValue(&sb, lu, lu.Family().String())
			output := sb.String()

			// Verify prefix
			assert.Contains(t, output, fmt.Sprintf(`"prefix":%q`, tt.prefix),
				"JSON should contain prefix")

			// Verify labels
			assert.Contains(t, output, tt.wantLabels,
				"JSON should contain labels array")

			// Verify path-id
			if tt.wantPathID {
				assert.Contains(t, output, fmt.Sprintf(`"path-id":%d`, tt.pathID),
					"JSON should contain path-id")
			} else {
				assert.NotContains(t, output, `"path-id"`,
					"JSON should not contain path-id when zero")
			}

			// Verify valid JSON
			var parsed map[string]any
			err := json.Unmarshal([]byte(output), &parsed)
			require.NoError(t, err, "Output should be valid JSON: %s", output)
		})
	}
}

// buildTestUpdateBodyWithDualIPv4NextHop builds UPDATE with both legacy and MP IPv4 unicast.
//
// RFC 4760: One UPDATE can have:
//   - Legacy IPv4 NLRI with NEXT_HOP attribute
//   - MP_REACH_NLRI for IPv4/unicast with different next-hop
//
//nolint:dupl // Test helper, similar structure to buildTestUpdateBodyWithBothFamilies is intentional
func buildTestUpdateBodyWithDualIPv4NextHop(
	legacyPrefix netip.Prefix, legacyNextHop netip.Addr,
	mpPrefix netip.Prefix, mpNextHop netip.Addr,
) []byte {
	// ORIGIN (igp) + AS_PATH (empty)
	var attrs []byte
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00)

	// NEXT_HOP for legacy IPv4 NLRI
	if legacyNextHop.Is4() {
		b := legacyNextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// MP_REACH_NLRI for IPv4/unicast with different next-hop
	// AFI=1 (IPv4), SAFI=1 (unicast), NH len=4, next-hop, reserved=0, NLRI
	mpReach := make([]byte, 0, 16)
	mpReach = append(mpReach, 0x00, 0x01, 0x01, 0x04) // AFI IPv4, SAFI unicast, NH len = 4
	nhBytes := mpNextHop.As4()
	mpReach = append(mpReach, nhBytes[:]...)

	// Reserved + IPv4 NLRI in MP_REACH
	bits := mpPrefix.Bits()
	prefixBytes := (bits + 7) / 8
	addr := mpPrefix.Addr().As4()
	mpReach = append(mpReach, 0x00, byte(bits))
	mpReach = append(mpReach, addr[:prefixBytes]...)

	// MP_REACH_NLRI attribute (optional, transitive)
	attrs = append(attrs, 0x90, 0x0e, byte(len(mpReach)>>8), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)

	// Legacy IPv4 NLRI (in body, not attributes)
	legacyNLRI := make([]byte, 0, 5)
	bits = legacyPrefix.Bits()
	legacyNLRI = append(legacyNLRI, byte(bits))
	prefixBytes = (bits + 7) / 8
	addr = legacyPrefix.Addr().As4()
	legacyNLRI = append(legacyNLRI, addr[:prefixBytes]...)

	// Build body
	body := make([]byte, 4+len(attrs)+len(legacyNLRI))
	binary.BigEndian.PutUint16(body[0:2], 0)                  // withdrawn len
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs))) //nolint:gosec // test data
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], legacyNLRI)

	return body
}

// TestJSONEncoderMPLSVPN verifies MPLS-VPN NLRI JSON output format.
//
// RFC 4364: IPVPN NLRI includes RD and labels in structured format.
// Output: {"prefix":"10.0.0.0/24", "rd":"0:65000:1", "labels":[100]}.
//
// VALIDATES: IPVPN NLRI includes all required fields.
// PREVENTS: Missing RD or labels in VPN route output.
func TestJSONEncoderMPLSVPN(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		rdType     nlri.RDType
		rdValue    [6]byte
		labels     []uint32
		pathID     uint32
		wantRD     string
		wantLabels string
		wantPathID bool
	}{
		{
			name:       "vpnv4_type0_rd",
			prefix:     "10.0.0.0/24",
			rdType:     nlri.RDType0,
			rdValue:    [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}, // 65000:100
			labels:     []uint32{100},
			wantRD:     "0:65000:100",
			wantLabels: `"labels":[[100]]`, // Plugin uses nested arrays for MPLS label stacks
		},
		{
			name:       "vpnv4_type1_rd",
			prefix:     "192.168.1.0/24",
			rdType:     nlri.RDType1,
			rdValue:    [6]byte{0x0A, 0x00, 0x00, 0x01, 0x00, 0x64}, // 10.0.0.1:100
			labels:     []uint32{200, 300},
			wantRD:     "1:10.0.0.1:100",
			wantLabels: `"labels":[[200],[300]]`, // Plugin uses nested arrays for MPLS label stacks
		},
		{
			name:       "vpnv4_type2_rd",
			prefix:     "172.16.0.0/16",
			rdType:     nlri.RDType2,
			rdValue:    [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x64}, // 65536:100
			labels:     []uint32{400},
			pathID:     42,
			wantRD:     "2:65536:100",
			wantLabels: `"labels":[[400]]`, // Plugin uses nested arrays for MPLS label stacks
			wantPathID: false,              // path-id is transport-level, not in plugin decode output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create IPVPN NLRI
			rd := nlri.RouteDistinguisher{Type: tt.rdType, Value: tt.rdValue}
			vpnNLRI := vpn.NewVPN(
				family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
				rd,
				tt.labels,
				netip.MustParsePrefix(tt.prefix),
				tt.pathID,
			)

			// Format using formatNLRIJSONValue (registry-based decode)
			var sb strings.Builder
			formatNLRIJSONValue(&sb, vpnNLRI, vpnNLRI.Family().String())
			output := sb.String()

			// Verify prefix
			assert.Contains(t, output, fmt.Sprintf(`"prefix":%q`, tt.prefix),
				"JSON should contain prefix")

			// Verify RD with type prefix
			assert.Contains(t, output, fmt.Sprintf(`"rd":%q`, tt.wantRD),
				"JSON should contain RD with type prefix")

			// Verify labels
			assert.Contains(t, output, tt.wantLabels,
				"JSON should contain labels array")

			// Verify path-id
			if tt.wantPathID {
				assert.Contains(t, output, fmt.Sprintf(`"path-id":%d`, tt.pathID),
					"JSON should contain path-id")
			} else {
				assert.NotContains(t, output, `"path-id"`,
					"JSON should not contain path-id when zero")
			}

			// Verify valid JSON
			var parsed map[string]any
			err := json.Unmarshal([]byte(output), &parsed)
			require.NoError(t, err, "Output should be valid JSON: %s", output)
		})
	}
}

// TestJSONEncoderFlowSpec verifies FlowSpec NLRI JSON output format.
//
// RFC 8955: FlowSpec components include operators for matching criteria.
// The current implementation outputs FlowSpec as a "spec" string field.
//
// VALIDATES: FlowSpec NLRI produces valid JSON with spec field.
// PREVENTS: FlowSpec components being unparseable in JSON output.
func TestJSONEncoderFlowSpec(t *testing.T) {
	tests := []struct {
		name       string
		rdType     nlri.RDType
		rdValue    [6]byte
		components []flowspec.FlowComponent
		wantRD     string
		wantKey    string // Expected top-level key in structured output
	}{
		{
			name:    "flowspec_vpn_dest_prefix",
			rdType:  nlri.RDType0,
			rdValue: [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}, // 65000:100
			components: []flowspec.FlowComponent{
				flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")),
			},
			wantRD:  "0:65000:100",
			wantKey: "destination",
		},
		{
			name:    "flowspec_vpn_protocol",
			rdType:  nlri.RDType2,
			rdValue: [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x64}, // 65536:100
			components: []flowspec.FlowComponent{
				flowspec.NewFlowIPProtocolComponent(6), // TCP
			},
			wantRD:  "2:65536:100",
			wantKey: "protocol",
		},
		{
			name:    "flowspec_vpn_port_range",
			rdType:  nlri.RDType1,
			rdValue: [6]byte{0x0A, 0x00, 0x00, 0x01, 0x00, 0x64}, // 10.0.0.1:100
			components: []flowspec.FlowComponent{
				flowspec.NewFlowDestPortComponent(80, 443),
			},
			wantRD:  "1:10.0.0.1:100",
			wantKey: "destination-port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create FlowSpecVPN
			rd := nlri.RouteDistinguisher{Type: tt.rdType, Value: tt.rdValue}
			fsv := flowspec.NewFlowSpecVPN(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, rd)
			for _, comp := range tt.components {
				fsv.AddComponent(comp)
			}

			// Format using formatNLRIJSONValue (registry-based decode)
			var sb strings.Builder
			formatNLRIJSONValue(&sb, fsv, fsv.Family().String())
			output := sb.String()

			// Verify RD with type prefix
			assert.Contains(t, output, fmt.Sprintf(`"rd":%q`, tt.wantRD),
				"JSON should contain RD with type prefix")

			// Verify structured component key exists (plugin uses structured JSON, not spec string)
			assert.Contains(t, output, fmt.Sprintf("%q", tt.wantKey),
				"JSON should contain component key")

			// Verify valid JSON
			var parsed map[string]any
			err := json.Unmarshal([]byte(output), &parsed)
			require.NoError(t, err, "Output should be valid JSON: %s", output)

			// Verify required fields exist
			assert.Contains(t, parsed, "rd", "JSON should have rd field")
			assert.Contains(t, parsed, tt.wantKey, "JSON should have component key")
		})
	}
}

// TestJSONEncoderNegotiated verifies negotiated capabilities JSON output.
//
// VALIDATES: Negotiated message contains capability fields in ze-bgp JSON format.
// PREVENTS: Plugins missing capability info after OPEN exchange.
func TestJSONEncoderNegotiated(t *testing.T) {
	enc := NewJSONEncoder("1.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		PeerAS:       65001,
		LocalAS:      65000,
	}

	neg := DecodedNegotiated{
		MessageSize:    4096,
		HoldTime:       90,
		ASN4:           true,
		RouteRefresh:   "enhanced",
		Families:       []string{"ipv4/unicast", "ipv6/unicast"},
		AddPathSend:    []string{"ipv4/unicast"},
		AddPathReceive: []string{"ipv4/unicast", "ipv6/unicast"},
		ExtendedNextHop: map[string]string{
			"ipv4/unicast": "ipv6",
		},
	}

	output := enc.Negotiated(&peer, neg)

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: check type in bgp.message
	eventType := getEventType(t, result)
	assert.Equal(t, "negotiated", eventType)

	// Get event-specific payload
	payload := getEventPayload(t, result)

	// Fields in event payload (hyphenated names, hold-time nested under timer)
	timer, ok := payload["timer"].(map[string]any)
	require.True(t, ok, "timer must be a map")
	assert.Equal(t, float64(90), timer["hold-time"])
	assert.Equal(t, true, payload["asn4"])

	// Check families in payload
	families, ok := payload["families"].([]any)
	require.True(t, ok, "families must be array")
	assert.Len(t, families, 2)
	assert.Contains(t, families, "ipv4/unicast")
	assert.Contains(t, families, "ipv6/unicast")

	// Check add-path (hyphenated)
	addPath, ok := payload["add-path"].(map[string]any)
	require.True(t, ok, "add-path must exist")

	sendFams, ok := addPath["send"].([]any)
	require.True(t, ok, "send must be array")
	assert.Contains(t, sendFams, "ipv4/unicast")

	recvFams, ok := addPath["receive"].([]any)
	require.True(t, ok, "receive must be array")
	assert.Len(t, recvFams, 2)
}

// TestJSONEncoderNegotiatedMinimal verifies negotiated with minimal fields.
//
// VALIDATES: Handles negotiated with only required fields in ze-bgp JSON format.
// PREVENTS: Nil pointer panics when optional fields missing.
func TestJSONEncoderNegotiatedMinimal(t *testing.T) {
	enc := NewJSONEncoder("1.0")
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("10.0.0.1")}

	neg := DecodedNegotiated{
		MessageSize:  4096,
		HoldTime:     180,
		ASN4:         false,
		RouteRefresh: "absent",
		Families:     []string{"ipv4/unicast"},
	}

	output := enc.Negotiated(&peer, neg)

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ze-bgp JSON: get event payload
	payload := getEventPayload(t, result)

	// Fields in event payload (hyphenated names, hold-time nested under timer)
	timer, ok := payload["timer"].(map[string]any)
	require.True(t, ok, "timer must be a map")
	assert.Equal(t, float64(180), timer["hold-time"])
	assert.Equal(t, false, payload["asn4"])

	// Families in payload
	families, ok := payload["families"].([]any)
	require.True(t, ok, "families must be array")
	assert.Len(t, families, 1)

	// add-path should not exist when empty
	_, hasAddPath := payload["add-path"]
	assert.False(t, hasAddPath, "add-path should be absent when empty")
}

// =============================================================================
// ze-bgp JSON Event Format Tests
// =============================================================================
// These tests validate the new JSON format per docs/architecture/api/ipc_protocol.md:
//   - Top-level "type" field indicates payload key ("bgp" or "rib")
//   - Event content nested under "bgp" or "rib" key
//   - Event type moved to payload (e.g., bgp.type = "update")
//   - Attributes nested under "attr"
//   - NLRIs nested under "nlri"
// =============================================================================

// TestEventJSONHasTopLevelType verifies BGP event has wrapper structure.
//
// VALIDATES: BGP event has "type":"bgp" and "bgp":{...} at top level.
// PREVENTS: Plugins expecting ze-bgp JSON format failing to parse.
func TestEventJSONHasTopLevelType(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
		MessageID:  123,
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Check top-level type field
	assert.Equal(t, "bgp", result["type"], "top-level type must be 'bgp'")

	// Check bgp payload object exists
	bgpPayload, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "bgp field must be object: %s", output)

	// BGP payload should have message.type field
	msgObj, ok := bgpPayload["message"].(map[string]any)
	require.True(t, ok, "bgp.message must be object: %s", output)
	assert.Equal(t, "update", msgObj["type"], "bgp.message.type must be 'update'")
}

// TestEventJSONBgpTypeField verifies event type is inside bgp payload.
//
// VALIDATES: Event type is in bgp.type (update, state, notification, etc).
// PREVENTS: Event type being at wrong location in JSON structure.
func TestEventJSONBgpTypeField(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	tests := []struct {
		name     string
		output   string
		wantType string
	}{
		{
			name:     "state_up",
			output:   enc.StateUp(&peer),
			wantType: "state",
		},
		{
			name:     "state_down",
			output:   enc.StateDown(&peer, "test"),
			wantType: "state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]any
			err := json.Unmarshal([]byte(tt.output), &result)
			require.NoError(t, err)

			// Top-level type indicates payload key
			assert.Equal(t, "bgp", result["type"])

			// Event type is in bgp.message.type
			bgpPayload := result["bgp"].(map[string]any)     //nolint:forcetypeassert,errcheck // test
			msgObj := bgpPayload["message"].(map[string]any) //nolint:forcetypeassert,errcheck // test
			assert.Equal(t, tt.wantType, msgObj["type"])
		})
	}
}

// TestEventJSONMessageMetadata verifies message object structure.
//
// VALIDATES: message object has type, id and direction.
// PREVENTS: Missing type or metadata in message object.
func TestEventJSONMessageMetadata(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, _ := wireUpdate.Attrs()

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
		MessageID:  456,
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Get bgp payload and message object
	bgpPayload := getBGPPayload(t, result)
	msgObj, ok := bgpPayload["message"].(map[string]any)
	require.True(t, ok, "bgp.message must be object: %s", output)

	// message should have type, id and direction
	assert.Equal(t, "update", msgObj["type"])
	assert.Equal(t, float64(456), msgObj["id"])
	assert.Equal(t, "received", msgObj["direction"])
}

// TestEventJSONPeerObject verifies peer object structure.
//
// VALIDATES: peer is object with address and asn fields.
// PREVENTS: Peer format changes breaking plugin parsing.
func TestEventJSONPeerObject(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	output := enc.StateUp(&peer)

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Use getEventPayload to get the nested state object
	statePayload := getEventPayload(t, result)

	peerObj, ok := statePayload["peer"].(map[string]any)
	require.True(t, ok, "bgp.state.peer must be object")

	assert.Equal(t, "192.168.1.2", peerObj["address"])
	remoteObj, ok := peerObj["remote"].(map[string]any)
	require.True(t, ok, "peer.remote must be object")
	assert.Equal(t, float64(65002), remoteObj["as"])
}

// TestEventJSONPeerNameGroup verifies the peerMap helper includes name and group in JSON output.
//
// VALIDATES: JSONEncoder methods include "name" and "group" in peer object when set.
// PREVENTS: peerMap omitting name/group fields.
func TestEventJSONPeerNameGroup(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	tests := []struct {
		name      string
		peer      plugin.PeerInfo
		wantGroup bool
	}{
		{
			name: "name and group",
			peer: plugin.PeerInfo{
				Address:   netip.MustParseAddr("10.0.0.1"),
				PeerAS:    65001,
				Name:      "upstream1",
				GroupName: "transit",
			},
			wantGroup: true,
		},
		{
			name: "name only",
			peer: plugin.PeerInfo{
				Address: netip.MustParseAddr("10.0.0.2"),
				PeerAS:  65002,
				Name:    "peer_east",
			},
			wantGroup: false,
		},
		{
			name: "group only",
			peer: plugin.PeerInfo{
				Address:   netip.MustParseAddr("10.0.0.3"),
				PeerAS:    65003,
				GroupName: "edge",
			},
			wantGroup: true,
		},
		{
			name: "no name no group",
			peer: plugin.PeerInfo{
				Address: netip.MustParseAddr("10.0.0.4"),
				PeerAS:  65004,
			},
			wantGroup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := enc.StateUp(&tt.peer)

			var result map[string]any
			err := json.Unmarshal([]byte(msg), &result)
			require.NoError(t, err, "JSON must be valid")

			payload := getEventPayload(t, result)
			peerObj, ok := payload["peer"].(map[string]any)
			require.True(t, ok, "peer must be object")

			// name is always present (YANG key)
			assert.Equal(t, tt.peer.Name, peerObj["name"])

			if tt.wantGroup {
				assert.Equal(t, tt.peer.GroupName, peerObj["group"])
			} else {
				_, hasGroup := peerObj["group"]
				assert.False(t, hasGroup, "group should be absent")
			}
		})
	}
}

// TestEventJSONNestedStructure verifies ze-bgp JSON nested structure for attributes and NLRIs.
//
// VALIDATES: Attributes under "attr", NLRIs under "nlri" objects.
// PREVENTS: Fields being at top level of bgp payload instead of nested.
func TestEventJSONNestedStructure(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, _ := wireUpdate.Attrs()

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Use getEventPayload to get the nested update object
	updatePayload := getEventPayload(t, result)

	t.Run("attr_nested", func(t *testing.T) {
		// Attributes should be nested under "attr"
		attrs, ok := updatePayload["attr"].(map[string]any)
		require.True(t, ok, "bgp.update.attr must be object: %s", output)

		// Should have origin attribute
		assert.Contains(t, attrs, "origin")

		// origin should NOT be at update level directly
		_, hasOriginAtTop := updatePayload["origin"]
		assert.False(t, hasOriginAtTop, "origin should NOT be at update level")
	})

	t.Run("nlri_nested", func(t *testing.T) {
		// NLRIs should be nested under "nlri"
		nlri, ok := updatePayload["nlri"].(map[string]any)
		require.True(t, ok, "bgp.update.nlri must be object: %s", output)

		// Should have ipv4/unicast family
		assert.Contains(t, nlri, "ipv4/unicast")

		// ipv4/unicast should NOT be at update level directly
		_, hasFamilyAtTop := updatePayload["ipv4/unicast"]
		assert.False(t, hasFamilyAtTop, "ipv4/unicast should NOT be at update level")
	})
}

// TestEventJSONRawSection verifies raw bytes in format=full.
//
// VALIDATES: Wire bytes are in "raw" object when format=full.
// PREVENTS: Raw bytes missing or at wrong location.
func TestEventJSONRawSection(t *testing.T) {
	ctxID := testEncodingContext()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)

	wireUpdate := wireu.NewWireUpdate(body, ctxID)
	attrsWire, _ := wireUpdate.Attrs()

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatFull, // Request raw bytes
	}

	output := FormatMessage(&peer, msg, content, "")

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Raw is at bgp level (alongside update), not inside update
	bgpPayload := getBGPPayload(t, result)

	// Raw should exist at bgp level
	raw, ok := bgpPayload["raw"].(map[string]any)
	require.True(t, ok, "bgp.raw must be object for format=full: %s", output)

	// Raw should have attributes and nlri fields
	assert.Contains(t, raw, "attributes", "raw should have attributes")
	assert.Contains(t, raw, "nlri", "raw should have nlri")
}
