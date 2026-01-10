package plugin

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
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

	output := FormatMessage(peer, msg, content)

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

	output := FormatMessage(peer, msg, content)

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
