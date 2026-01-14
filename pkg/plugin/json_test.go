package plugin

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONEncoderStateUp verifies JSON output for peer state "up".
//
// VALIDATES: ExaBGP v6 JSON format for state messages.
//
// PREVENTS: JSON format incompatibility with ExaBGP clients expecting
// specific field names and structure.
func TestJSONEncoderStateUp(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")
	enc.SetHostname("testhost")
	enc.SetPID(12345, 1)

	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	msg := enc.StateUp(peer)

	// Parse JSON to verify structure
	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err, "JSON must be valid")

	// Check required fields
	assert.Equal(t, "6.0.0", result["zebgp"])
	assert.Equal(t, "testhost", result["host"])
	assert.Equal(t, float64(12345), result["pid"])
	assert.Equal(t, float64(1), result["ppid"])

	// Check message wrapper
	msgWrapper, ok := result["message"].(map[string]any)
	require.True(t, ok, "message wrapper must exist")
	assert.Equal(t, "state", msgWrapper["type"])

	// Check neighbor structure
	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok, "neighbor must be object")

	address, ok := peerMap["address"].(map[string]any)
	require.True(t, ok, "address must be object")
	assert.Equal(t, "192.168.1.1", address["local"])
	assert.Equal(t, "192.168.1.2", address["peer"])

	asn, ok := peerMap["asn"].(map[string]any)
	require.True(t, ok, "asn must be object")
	assert.Equal(t, float64(65001), asn["local"])
	assert.Equal(t, float64(65002), asn["peer"])

	assert.Equal(t, "up", peerMap["state"])
}

// TestJSONEncoderStateDown verifies JSON output for peer state "down".
//
// VALIDATES: Down state includes reason field.
//
// PREVENTS: Missing reason in down notifications, making debugging harder.
func TestJSONEncoderStateDown(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	msg := enc.StateDown(peer, "hold timer expired")

	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err)

	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "down", peerMap["state"])
	assert.Equal(t, "hold timer expired", peerMap["reason"])
}

// TestJSONEncoderStateConnected verifies JSON output for "connected" state.
//
// VALIDATES: Connected state message format.
//
// PREVENTS: Missing TCP connection events in event stream.
func TestJSONEncoderStateConnected(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65000,
		PeerAS:       65001,
	}

	msg := enc.StateConnected(peer)

	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err)

	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "connected", peerMap["state"])
}

// TestJSONEncoderCounter verifies per-neighbor message counter.
//
// VALIDATES: Counter increments for each message to same peer.
//
// PREVENTS: Incorrect message ordering detection by consumers.
func TestJSONEncoderCounter(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// First message
	msg1 := enc.StateUp(peer)
	var result1 map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg1), &result1))
	assert.Equal(t, float64(1), result1["counter"])

	// Second message to same peer
	msg2 := enc.StateDown(peer, "test")
	var result2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg2), &result2))
	assert.Equal(t, float64(2), result2["counter"])

	// Third message
	msg3 := enc.StateConnected(peer)
	var result3 map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg3), &result3))
	assert.Equal(t, float64(3), result3["counter"])
}

// TestJSONEncoderCounterPerPeer verifies counters are per-peer.
//
// VALIDATES: Different peers have independent counters.
//
// PREVENTS: Counter confusion when multiple peers are active.
func TestJSONEncoderCounterPerPeer(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer1 := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}
	peer2 := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.3"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65003,
	}

	// Messages to peer1
	msg1 := enc.StateUp(peer1)
	var result1 map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg1), &result1))
	assert.Equal(t, float64(1), result1["counter"])

	// First message to peer2 should have counter=1
	msg2 := enc.StateUp(peer2)
	var result2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg2), &result2))
	assert.Equal(t, float64(1), result2["counter"])

	// Second message to peer1 should have counter=2
	msg3 := enc.StateDown(peer1, "test")
	var result3 map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg3), &result3))
	assert.Equal(t, float64(2), result3["counter"])
}

// TestJSONEncoderTimestamp verifies timestamp format.
//
// VALIDATES: Unix timestamp with fractional seconds in message wrapper.
//
// PREVENTS: Time parsing errors in clients expecting float timestamp.
func TestJSONEncoderTimestamp(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	// Use fixed time for test
	fixedTime := time.Date(2024, 12, 19, 12, 0, 0, 123456789, time.UTC)
	enc.SetTimeFunc(func() time.Time { return fixedTime })

	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	msg := enc.StateUp(peer)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg), &result))

	// Time should be in message wrapper
	msgWrapper, ok := result["message"].(map[string]any)
	require.True(t, ok, "message wrapper must exist")
	timestamp, ok := msgWrapper["time"].(float64)
	require.True(t, ok, "message.time must be float64")

	// Verify it's in the right range (Unix timestamp)
	assert.Greater(t, timestamp, float64(1700000000), "timestamp should be recent Unix time")
	assert.Less(t, timestamp, float64(2000000000), "timestamp should be reasonable")
}

// TestJSONEncoderValidJSON verifies output is always valid JSON.
//
// VALIDATES: All encoder methods produce valid JSON.
//
// PREVENTS: Parse errors in consumers due to malformed JSON.
func TestJSONEncoderValidJSON(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	messages := []string{
		enc.StateUp(peer),
		enc.StateDown(peer, "test reason"),
		enc.StateConnected(peer),
		enc.StateDown(peer, `reason with "quotes" and \backslashes`),
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
//
// PREVENTS: JSON injection or parse errors from special characters.
func TestJSONEncoderSpecialCharacters(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// Reason with special characters
	reason := `connection reset: "peer closed" with \n newline`
	msg := enc.StateDown(peer, reason)

	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err, "JSON with special chars must be valid")

	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok)
	// The reason should be properly escaped in JSON but decoded back
	assert.Contains(t, peerMap["reason"], "peer closed")
}

// TestJSONEncoderIPv6 verifies IPv6 address formatting.
//
// VALIDATES: IPv6 addresses are formatted correctly in JSON.
//
// PREVENTS: IPv6 address parsing errors in consumers.
func TestJSONEncoderIPv6(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := PeerInfo{
		Address:      netip.MustParseAddr("2001:db8::1"),
		LocalAddress: netip.MustParseAddr("2001:db8::2"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	msg := enc.StateUp(peer)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg), &result))

	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok)
	address, ok := peerMap["address"].(map[string]any)
	require.True(t, ok)

	// IPv6 addresses should be in standard format
	local, ok := address["local"].(string)
	require.True(t, ok)
	peerAddr, ok := address["peer"].(string)
	require.True(t, ok)
	assert.True(t, strings.Contains(local, "2001:db8"))
	assert.True(t, strings.Contains(peerAddr, "2001:db8"))
}

// TestAPIOutputIncludesMsgID verifies API JSON has message.id field.
//
// VALIDATES: API output contains id in message wrapper for received UPDATEs.
// PREVENTS: Controller can't reference updates for forwarding.
func TestAPIOutputIncludesMsgID(t *testing.T) {
	peer := PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// UPDATE with msg-id
	msg := RawMessage{
		Type: message.TypeUPDATE,
		RawBytes: []byte{
			// Minimal UPDATE with NLRI: 10.0.0.0/24
			0x00, 0x00, // withdrawn length
			0x00, 0x00, // attrs length
			0x18, 0x0a, 0x00, 0x00, // NLRI: 10.0.0.0/24
		},
		MessageID: 12345,
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	// Parse JSON
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Check message.id present
	msgWrapper, ok := result["message"].(map[string]any)
	require.True(t, ok, "message wrapper must be present")
	msgID, ok := msgWrapper["id"]
	require.True(t, ok, "message.id must be present")
	assert.Equal(t, float64(12345), msgID)
}

// TestJSONEncoderNotification verifies JSON output for NOTIFICATION message.
//
// VALIDATES: NOTIFICATION JSON contains code, subcode, data, and ZeBGP extensions.
// PREVENTS: Plugin can't parse notification events or missing error context.
func TestJSONEncoderNotification(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")
	enc.SetHostname("testhost")
	enc.SetPID(12345, 1)

	peer := PeerInfo{
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

	msg := enc.Notification(peer, notify, "received", 42)

	// Parse JSON to verify structure
	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err, "JSON must be valid: %s", msg)

	// Check standard fields
	assert.Equal(t, "6.0.0", result["zebgp"])
	assert.Equal(t, "testhost", result["host"])
	assert.Equal(t, "received", result["direction"])

	// Check message wrapper
	msgWrapper, ok := result["message"].(map[string]any)
	require.True(t, ok, "message wrapper must exist")
	assert.Equal(t, "notification", msgWrapper["type"])
	assert.Equal(t, float64(42), msgWrapper["id"])

	// Check peer structure
	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok, "peer must be object")

	// Check notification object
	notifObj, ok := peerMap["notification"].(map[string]any)
	require.True(t, ok, "notification must be object")

	// ExaBGP required fields
	assert.Equal(t, float64(6), notifObj["code"])
	assert.Equal(t, float64(2), notifObj["subcode"])
	assert.NotEmpty(t, notifObj["data"], "data field must be present")

	// ZeBGP extensions
	assert.Equal(t, "Cease", notifObj["code_name"])
	assert.Equal(t, "Administrative Shutdown", notifObj["subcode_name"])
	assert.Equal(t, "maintenance window", notifObj["message"])
}

// TestJSONEncoderNotificationMinimal verifies minimal NOTIFICATION JSON.
//
// VALIDATES: NOTIFICATION without shutdown message still has required fields.
// PREVENTS: Crash or missing fields on minimal notifications.
func TestJSONEncoderNotificationMinimal(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := PeerInfo{
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

	msg := enc.Notification(peer, notify, "received", 0)

	var result map[string]any
	err := json.Unmarshal([]byte(msg), &result)
	require.NoError(t, err)

	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok, "peer must be object")
	notifObj, ok := peerMap["notification"].(map[string]any)
	require.True(t, ok, "notification must be object")

	// Required fields still present
	assert.Equal(t, float64(4), notifObj["code"])
	assert.Equal(t, float64(0), notifObj["subcode"])
	assert.Equal(t, "", notifObj["data"]) // Empty string, not nil

	// Extensions present
	assert.Equal(t, "Hold Timer Expired", notifObj["code_name"])

	// message field should NOT be present when empty
	_, hasMessage := notifObj["message"]
	assert.False(t, hasMessage, "message should not be present when empty")

	// message.id should NOT be present when zero
	msgWrapper, ok := result["message"].(map[string]any)
	require.True(t, ok, "message wrapper must exist")
	_, hasID := msgWrapper["id"]
	assert.False(t, hasID, "message.id should not be present when zero")
}

// TestJSONEncoderNotificationSent verifies NOTIFICATION with "sent" direction.
//
// VALIDATES: Sent notifications include direction field correctly.
// PREVENTS: Direction field missing or incorrect for outbound notifications.
func TestJSONEncoderNotificationSent(t *testing.T) {
	enc := NewJSONEncoder("6.0.0")

	peer := PeerInfo{
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

	msg := enc.Notification(peer, notify, "sent", 100)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(msg), &result))

	assert.Equal(t, "sent", result["direction"])

	msgWrapper, ok := result["message"].(map[string]any)
	require.True(t, ok, "message wrapper must exist")
	assert.Equal(t, float64(100), msgWrapper["id"])
}

// TestFormatMessageNotificationText verifies FormatMessage returns text for NOTIFICATION.
//
// NOTE: FormatMessage only returns TEXT for non-UPDATE messages. For JSON format,
// Server.formatMessage() should be used (see TestServerFormatMessageNotificationJSON).
//
// VALIDATES: FormatMessage returns parseable text for NOTIFICATION.
// PREVENTS: Crashes or malformed output from FormatMessage with NOTIFICATION.
func TestFormatMessageNotificationText_Parsed(t *testing.T) {
	peer := PeerInfo{
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

	msg := RawMessage{
		Type:      message.TypeNOTIFICATION,
		RawBytes:  rawBytes,
		MessageID: 42,
		Direction: "received",
	}

	// Even with EncodingJSON, FormatMessage returns text for non-UPDATE
	// This is by design - JSON encoding for non-UPDATE uses Server.formatMessage()
	content := ContentConfig{
		Encoding: EncodingText,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

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
// VALIDATES: FormatMessage with EncodingJSON + FormatParsed + NOTIFICATION returns TEXT.
// PREVENTS: Confusion about why JSON encoding is ignored.
func TestFormatMessageIgnoresEncodingForParsedNonUpdate(t *testing.T) {
	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65002,
	}

	msg := RawMessage{
		Type:      message.TypeNOTIFICATION,
		RawBytes:  []byte{0x04, 0x00}, // Hold Timer Expired
		MessageID: 1,
		Direction: "received",
	}

	// Request JSON encoding with parsed format
	content := ContentConfig{
		Encoding: EncodingJSON, // Requested JSON...
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	// ...but we get TEXT because FormatMessage ignores Encoding for parsed non-UPDATE
	assert.True(t, strings.HasPrefix(output, "peer "),
		"Expected text format starting with 'peer ', got: %s", output)
	assert.False(t, strings.HasPrefix(output, "{"),
		"Should NOT be JSON for parsed non-UPDATE")
}

// TestAPIOutputNoMsgIDWhenZero verifies message.id omitted when zero.
//
// VALIDATES: Zero message.id is not included in output.
// PREVENTS: Cluttering output with meaningless id:0.
func TestAPIOutputNoMsgIDWhenZero(t *testing.T) {
	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65002,
	}

	msg := RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  []byte{0x00, 0x00, 0x00, 0x00}, // Empty UPDATE
		MessageID: 0,                              // No msg-id
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	// Parse JSON
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// message wrapper should exist
	msgWrapper, ok := result["message"].(map[string]any)
	require.True(t, ok, "message wrapper must exist")

	// id should NOT be present in message wrapper when zero
	_, ok = msgWrapper["id"]
	assert.False(t, ok, "message.id should not be present when zero")
}

// =============================================================================
// New NLRI Format Tests (Command-Style)
// =============================================================================
// These tests validate the new JSON format for UPDATE NLRI:
//   - Family at top level (not nested under "announce" or "withdraw")
//   - Operations array with "action": "add" or "action": "del"
//   - "next-hop" per operation group (for add actions)
//   - "nlri" array (strings for simple prefixes)
//
// RFC 4271 Section 5.1.3: NEXT_HOP defines the IP address of the router
// that SHOULD be used as next hop to the destinations.
// RFC 4760 Section 3: Each MP_REACH_NLRI has its own next-hop field.
// =============================================================================

// TestJSONEncoderIPv4UnicastNewFormat verifies new command-style JSON format.
//
// VALIDATES: IPv4 unicast uses family → operations list with action/next-hop/nlri.
// PREVENTS: Old announce/withdraw format being emitted.
func TestJSONEncoderIPv4UnicastNewFormat(t *testing.T) {
	ctxID := testEncodingContext()

	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with IPv4 NLRI: 192.168.1.0/24
	body := buildTestUpdateBodyWithAttrs(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		0, 0, nil,
	)

	wireUpdate := NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	// Parse JSON
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// New format: "ipv4/unicast" at top level (not under "announce")
	_, hasAnnounce := result["announce"]
	assert.False(t, hasAnnounce, "new format should NOT have 'announce' key")

	// Family should be at top level
	family, ok := result["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array at top level: %s", output)
	require.Len(t, family, 1, "should have one operation group")

	// Check operation structure
	op, ok := family[0].(map[string]any)
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
// VALIDATES: Withdrawals use action: del with no next-hop.
// PREVENTS: Withdraw still using old nested format.
func TestJSONEncoderWithdrawNewFormat(t *testing.T) {
	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with withdrawn routes only
	body := buildTestUpdateBodyWithWithdrawn(
		netip.MustParsePrefix("172.16.0.0/16"),
	)

	wireUpdate := NewWireUpdate(body, testEncodingContext())
	attrsWire, _ := wireUpdate.Attrs()

	msg := RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// New format: no "withdraw" key
	_, hasWithdraw := result["withdraw"]
	assert.False(t, hasWithdraw, "new format should NOT have 'withdraw' key")

	// Family at top level with action: del
	family, ok := result["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array: %s", output)
	require.Len(t, family, 1)

	op, ok := family[0].(map[string]any)
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
// VALIDATES: Each family appears at top level with own operations.
// PREVENTS: Multi-family UPDATEs breaking JSON structure.
func TestJSONEncoderMultiFamilyNewFormat(t *testing.T) {
	ctxID := testEncodingContext()

	peer := PeerInfo{
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

	wireUpdate := NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Both families should be at top level
	ipv4Fam, ok := result["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be present: %s", output)
	require.Len(t, ipv4Fam, 1)

	ipv6Fam, ok := result["ipv6/unicast"].([]any)
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
// VALIDATES: Same family can have both add and del operations.
// PREVENTS: Mixed announce/withdraw breaking JSON.
func TestJSONEncoderAnnounceAndWithdrawSameFamily(t *testing.T) {
	ctxID := testEncodingContext()

	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with both NLRI and withdrawn
	body := buildTestUpdateBodyWithNLRIAndWithdrawn(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParsePrefix("172.16.0.0/16"),
	)

	wireUpdate := NewWireUpdate(body, ctxID)
	attrsWire, _ := wireUpdate.Attrs()

	msg := RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Family should have both add and del operations
	family, ok := result["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array: %s", output)
	require.Len(t, family, 2, "should have 2 operation groups (add + del)")

	// Find add and del operations
	var hasAdd, hasDel bool
	for _, item := range family {
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
// VALIDATES: ADD-PATH path-id appears in nlri objects.
// PREVENTS: Path-id being lost with new format.
func TestJSONEncoderADDPATHNewFormat(t *testing.T) {
	// Create encoding context with ADD-PATH for IPv4 unicast
	ctxID := testEncodingContextWithAddPath()

	peer := PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with path-id in NLRI
	body := buildTestUpdateBodyWithPathID(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		42, // path-id
	)

	wireUpdate := NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Family should be at top level
	family, ok := result["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array: %s", output)
	require.Len(t, family, 1)

	op, ok := family[0].(map[string]any)
	require.True(t, ok, "operation should be map")
	nlri, ok := op["nlri"].([]any)
	require.True(t, ok)
	require.Len(t, nlri, 1)

	// With ADD-PATH, nlri items are objects with prefix and path-id
	nlriObj, ok := nlri[0].(map[string]any)
	require.True(t, ok, "with ADD-PATH, nlri should be object: %v", nlri[0])
	assert.Equal(t, "192.168.1.0/24", nlriObj["prefix"])
	assert.Equal(t, float64(42), nlriObj["path-id"])
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

	// Build attributes
	var attrs []byte

	// ORIGIN
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00) // igp

	// AS_PATH (empty)
	attrs = append(attrs, 0x40, 0x02, 0x00)

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
		nlri = append(nlri, byte(bits))
		prefixBytes := (bits + 7) / 8
		addr := announcePrefix.Addr().As4()
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
	var attrs []byte

	// ORIGIN
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00) // igp

	// AS_PATH (empty)
	attrs = append(attrs, 0x40, 0x02, 0x00)

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
	addPath := map[nlri.Family]bool{
		nlri.IPv4Unicast: true,
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
// VALIDATES: Two next-hops for same family produce two operations.
// PREVENTS: Merging different next-hops, losing route information.
func TestJSONEncoderIPv4DualNextHop(t *testing.T) {
	ctxID := testEncodingContext()

	peer := PeerInfo{
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

	wireUpdate := NewWireUpdate(body, ctxID)
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err)

	msg := RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
		Direction:  "received",
	}

	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatParsed,
	}

	output := FormatMessage(peer, msg, content, "")

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// ipv4/unicast should have TWO operation groups (different next-hops)
	family, ok := result["ipv4/unicast"].([]any)
	require.True(t, ok, "ipv4/unicast should be array: %s", output)
	require.Len(t, family, 2, "should have 2 operation groups (two next-hops)")

	// Collect next-hops and nlris
	nextHops := make(map[string][]string)
	for _, item := range family {
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rd := nlri.RouteDistinguisher{Type: tc.rdType, Value: tc.rdValue}

			// Verify RD String() output
			assert.Equal(t, tc.wantRD, rd.String(), "RD should include type prefix")

			// Create IPVPN NLRI and verify JSON output
			vpn := nlri.NewIPVPN(
				nlri.IPv4VPN,
				rd,
				[]uint32{100}, // label
				netip.MustParsePrefix("10.0.0.0/24"),
				0, // no path-id
			)

			// Format using formatIPVPNJSON
			var sb strings.Builder
			formatIPVPNJSON(&sb, vpn)
			output := sb.String()

			// Verify RD in JSON output
			assert.Contains(t, output, fmt.Sprintf(`"rd":"%s"`, tc.wantRD),
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
	evpn := createTestEVPNType2(rd, esi, ethTag, mac, ip, label)

	// Format using formatEVPNType2JSON
	var sb strings.Builder
	formatEVPNType2JSON(&sb, evpn)
	output := sb.String()

	// Verify all fields
	assert.Contains(t, output, `"route-type":"mac-ip-advertisement"`, "route-type")
	assert.Contains(t, output, `"rd":"0:65000:100"`, "RD with type prefix")
	assert.Contains(t, output, `"esi":"00:00:00:00:00:00:00:00:00:00"`, "ESI")
	assert.Contains(t, output, `"ethernet-tag":0`, "ethernet-tag")
	assert.Contains(t, output, `"mac":"00:11:22:33:44:55"`, "MAC")
	assert.Contains(t, output, `"ip":"10.0.0.1"`, "IP")
	assert.Contains(t, output, `"labels":[100]`, "labels")

	// Verify valid JSON
	var parsed map[string]any
	err := json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err, "Output should be valid JSON: %s", output)
}

// createTestEVPNType2 creates an EVPNType2 for testing by parsing wire format.
func createTestEVPNType2(rdBytes, esiBytes, ethTagBytes, macBytes, ipBytes, labelBytes []byte) *nlri.EVPNType2 {
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
	evpnData = append(evpnData, byte(nlri.EVPNRouteType2), byte(len(data)))
	evpnData = append(evpnData, data...)

	// Parse using the public ParseEVPN function
	parsed, _, err := nlri.ParseEVPN(evpnData, false)
	if err != nil {
		panic(fmt.Sprintf("failed to parse test EVPN: %v", err))
	}

	evpn, ok := parsed.(*nlri.EVPNType2)
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
			wantPathID: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create LabeledUnicast NLRI
			lu := nlri.NewLabeledUnicast(
				nlri.IPv4LabeledUnicast,
				netip.MustParsePrefix(tc.prefix),
				tc.labels,
				tc.pathID,
			)

			// Format using formatLabeledUnicastJSON
			var sb strings.Builder
			formatLabeledUnicastJSON(&sb, lu)
			output := sb.String()

			// Verify prefix
			assert.Contains(t, output, fmt.Sprintf(`"prefix":"%s"`, tc.prefix),
				"JSON should contain prefix")

			// Verify labels
			assert.Contains(t, output, tc.wantLabels,
				"JSON should contain labels array")

			// Verify path-id
			if tc.wantPathID {
				assert.Contains(t, output, fmt.Sprintf(`"path-id":%d`, tc.pathID),
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
	var attrs []byte

	// ORIGIN
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00) // igp

	// AS_PATH (empty)
	attrs = append(attrs, 0x40, 0x02, 0x00)

	// NEXT_HOP for legacy IPv4 NLRI
	if legacyNextHop.Is4() {
		b := legacyNextHop.As4()
		attrs = append(attrs, 0x40, 0x03, 0x04)
		attrs = append(attrs, b[:]...)
	}

	// MP_REACH_NLRI for IPv4/unicast with different next-hop
	// AFI=1 (IPv4), SAFI=1 (unicast), NH len=4, next-hop, reserved=0, NLRI
	mpReach := make([]byte, 0, 16)
	mpReach = append(mpReach, 0x00, 0x01) // AFI IPv4
	mpReach = append(mpReach, 0x01)       // SAFI unicast
	mpReach = append(mpReach, 0x04)       // NH len = 4
	nhBytes := mpNextHop.As4()
	mpReach = append(mpReach, nhBytes[:]...)
	mpReach = append(mpReach, 0x00) // reserved

	// IPv4 NLRI in MP_REACH
	bits := mpPrefix.Bits()
	mpReach = append(mpReach, byte(bits))
	prefixBytes := (bits + 7) / 8
	addr := mpPrefix.Addr().As4()
	mpReach = append(mpReach, addr[:prefixBytes]...)

	// MP_REACH_NLRI attribute (optional, transitive)
	attrs = append(attrs, 0x90, 0x0e) // flags=0x90, type=14
	attrs = append(attrs, byte(len(mpReach)>>8), byte(len(mpReach)))
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
			wantLabels: `"labels":[100]`,
		},
		{
			name:       "vpnv4_type1_rd",
			prefix:     "192.168.1.0/24",
			rdType:     nlri.RDType1,
			rdValue:    [6]byte{0x0A, 0x00, 0x00, 0x01, 0x00, 0x64}, // 10.0.0.1:100
			labels:     []uint32{200, 300},
			wantRD:     "1:10.0.0.1:100",
			wantLabels: `"labels":[200,300]`,
		},
		{
			name:       "vpnv4_type2_rd",
			prefix:     "172.16.0.0/16",
			rdType:     nlri.RDType2,
			rdValue:    [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x64}, // 65536:100
			labels:     []uint32{400},
			pathID:     42,
			wantRD:     "2:65536:100",
			wantLabels: `"labels":[400]`,
			wantPathID: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create IPVPN NLRI
			rd := nlri.RouteDistinguisher{Type: tc.rdType, Value: tc.rdValue}
			vpn := nlri.NewIPVPN(
				nlri.IPv4VPN,
				rd,
				tc.labels,
				netip.MustParsePrefix(tc.prefix),
				tc.pathID,
			)

			// Format using formatIPVPNJSON
			var sb strings.Builder
			formatIPVPNJSON(&sb, vpn)
			output := sb.String()

			// Verify prefix
			assert.Contains(t, output, fmt.Sprintf(`"prefix":"%s"`, tc.prefix),
				"JSON should contain prefix")

			// Verify RD with type prefix
			assert.Contains(t, output, fmt.Sprintf(`"rd":"%s"`, tc.wantRD),
				"JSON should contain RD with type prefix")

			// Verify labels
			assert.Contains(t, output, tc.wantLabels,
				"JSON should contain labels array")

			// Verify path-id
			if tc.wantPathID {
				assert.Contains(t, output, fmt.Sprintf(`"path-id":%d`, tc.pathID),
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
		components []nlri.FlowComponent
		wantRD     string
		wantSpec   string // Expected substring in spec field
	}{
		{
			name:    "flowspec_vpn_dest_prefix",
			rdType:  nlri.RDType0,
			rdValue: [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}, // 65000:100
			components: []nlri.FlowComponent{
				nlri.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")),
			},
			wantRD:   "0:65000:100",
			wantSpec: "dest-prefix=10.0.0.0/24",
		},
		{
			name:    "flowspec_vpn_protocol",
			rdType:  nlri.RDType2,
			rdValue: [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x64}, // 65536:100
			components: []nlri.FlowComponent{
				nlri.NewFlowIPProtocolComponent(6), // TCP
			},
			wantRD:   "2:65536:100",
			wantSpec: "protocol",
		},
		{
			name:    "flowspec_vpn_port_range",
			rdType:  nlri.RDType1,
			rdValue: [6]byte{0x0A, 0x00, 0x00, 0x01, 0x00, 0x64}, // 10.0.0.1:100
			components: []nlri.FlowComponent{
				nlri.NewFlowDestPortComponent(80, 443),
			},
			wantRD:   "1:10.0.0.1:100",
			wantSpec: "dest-port",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create FlowSpecVPN
			rd := nlri.RouteDistinguisher{Type: tc.rdType, Value: tc.rdValue}
			fsv := nlri.NewFlowSpecVPN(nlri.IPv4FlowSpec, rd)
			for _, comp := range tc.components {
				fsv.AddComponent(comp)
			}

			// Format using formatFlowSpecVPNJSON
			var sb strings.Builder
			formatFlowSpecVPNJSON(&sb, fsv)
			output := sb.String()

			// Verify RD with type prefix
			assert.Contains(t, output, fmt.Sprintf(`"rd":"%s"`, tc.wantRD),
				"JSON should contain RD with type prefix")

			// Verify spec field contains expected component info
			assert.Contains(t, output, tc.wantSpec,
				"JSON spec field should contain component info")

			// Verify valid JSON
			var parsed map[string]any
			err := json.Unmarshal([]byte(output), &parsed)
			require.NoError(t, err, "Output should be valid JSON: %s", output)

			// Verify required fields exist
			assert.Contains(t, parsed, "rd", "JSON should have rd field")
			assert.Contains(t, parsed, "spec", "JSON should have spec field")
		})
	}
}
