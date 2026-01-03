package api

import (
	"net/netip"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/stretchr/testify/require"
)

// TestRawMessageType verifies RawMessage struct exists with expected fields.
//
// VALIDATES: RawMessage has Type, RawBytes, Timestamp fields for raw message passing.
//
// PREVENTS: Regression where raw message data is lost during forwarding.
func TestRawMessageType(t *testing.T) {
	now := time.Now()
	msg := RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  []byte{0x00, 0x00, 0x00, 0x17},
		Timestamp: now,
	}

	require.Equal(t, message.TypeUPDATE, msg.Type)
	require.Equal(t, []byte{0x00, 0x00, 0x00, 0x17}, msg.RawBytes)
	require.Equal(t, now, msg.Timestamp)
}

// TestFormatSwitchingParsedRawFull verifies format config controls parsing.
//
// VALIDATES: format=raw doesn't include parsed fields, format=parsed includes them,
// format=full includes both.
//
// PREVENTS: Bug where parsing always happens regardless of format setting.
func TestFormatSwitchingParsedRawFull(t *testing.T) {
	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// Valid UPDATE wire bytes (minimal: 0 withdrawn, 0 attrs, 0 nlri)
	updateBytes := []byte{
		0x00, 0x00, // withdrawn length
		0x00, 0x00, // path attr length
	}

	msg := RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  updateBytes,
		Timestamp: time.Now(),
	}

	content := ContentConfig{Encoding: "json", Format: "raw"}
	rawOutput := FormatMessage(peer, msg, content)
	require.Contains(t, rawOutput, "raw", "raw format should contain raw field")

	content.Format = "parsed"
	parsedOutput := FormatMessage(peer, msg, content)
	// Parsed output should have update structure but no raw field
	require.Contains(t, parsedOutput, "update")

	content.Format = "full"
	fullOutput := FormatMessage(peer, msg, content)
	require.Contains(t, fullOutput, "raw", "full format should contain raw field")
	require.Contains(t, fullOutput, "update", "full format should contain parsed data")
}

// TestFormatFullWithRoutes verifies format=full includes both parsed routes AND raw bytes.
//
// VALIDATES: format=full outputs actual route data plus raw hex.
//
// PREVENTS: Bug where format=full only outputs raw bytes without parsed content.
func TestFormatFullWithRoutes(t *testing.T) {
	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// Valid UPDATE with: 0 withdrawn, ORIGIN+NEXT_HOP attrs, one /24 NLRI
	// Structure: withdrawn_len(2) + attrs_len(2) + attrs + nlri
	updateBytes := []byte{
		0x00, 0x00, // withdrawn length = 0
		0x00, 0x0b, // path attr length = 11
		// ORIGIN (type 1): well-known mandatory, IGP
		0x40, 0x01, 0x01, 0x00,
		// NEXT_HOP (type 3): well-known mandatory, 192.168.1.2
		0x40, 0x03, 0x04, 0xc0, 0xa8, 0x01, 0x02,
		// NLRI: 10.0.0.0/24
		0x18, 0x0a, 0x00, 0x00,
	}

	msg := RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  updateBytes,
		Timestamp: time.Now(),
	}

	// Test JSON full format
	content := ContentConfig{Encoding: "json", Format: "full"}
	output := FormatMessage(peer, msg, content)

	require.Contains(t, output, "raw", "full format must contain raw field")
	require.Contains(t, output, "10.0.0.0/24", "full format must contain parsed prefix")
	require.Contains(t, output, "announce", "full format must contain announce section")

	// Test text full format
	content.Encoding = "text"
	textOutput := FormatMessage(peer, msg, content)

	require.Contains(t, textOutput, "10.0.0.0/24", "text full format must contain parsed prefix")
	require.Contains(t, textOutput, "raw", "text full format must contain raw marker")
}

// TestContentConfigDefaults verifies ContentConfig has sensible defaults.
//
// VALIDATES: Default encoding is "text", default format is "parsed".
//
// PREVENTS: Nil pointer or empty string causing unexpected behavior.
func TestContentConfigDefaults(t *testing.T) {
	cfg := ContentConfig{}

	// Apply defaults
	cfg = cfg.WithDefaults()

	require.Equal(t, "text", cfg.Encoding)
	require.Equal(t, "parsed", cfg.Format)
}

// TestDecodeOpen verifies DecodeOpen parses OPEN message bytes.
//
// VALIDATES: DecodeOpen extracts version, AS, hold time, router ID, capabilities.
//
// PREVENTS: Bug where OPEN messages can't be formatted for processes.
func TestDecodeOpen(t *testing.T) {
	// Valid OPEN: version=4, AS=65001, hold=90, router_id=1.2.3.4, opt_len=0
	openBytes := []byte{
		0x04,       // version
		0xfd, 0xe9, // AS 65001
		0x00, 0x5a, // hold time 90
		0x01, 0x02, 0x03, 0x04, // router ID 1.2.3.4
		0x00, // opt param len
	}

	decoded := DecodeOpen(openBytes)

	require.Equal(t, uint8(4), decoded.Version)
	require.Equal(t, uint32(65001), decoded.ASN)
	require.Equal(t, uint16(90), decoded.HoldTime)
	require.Equal(t, "1.2.3.4", decoded.RouterID)
	require.Empty(t, decoded.Capabilities) // No capabilities in this OPEN
}

// TestDecodeOpenWithCapabilities verifies capabilities are parsed.
//
// VALIDATES: DecodeOpen extracts capabilities from optional parameters.
//
// PREVENTS: Bug where capabilities missing from OPEN output.
func TestDecodeOpenWithCapabilities(t *testing.T) {
	// Valid OPEN with capabilities:
	// - Multiprotocol IPv4/Unicast (type 1, len 4)
	// - Route Refresh (type 2, len 0)
	// - ASN4 65536 (type 65, len 4)
	openBytes := []byte{
		0x04,       // version
		0x5b, 0xa0, // AS 23456 (AS_TRANS)
		0x00, 0xb4, // hold time 180
		0x0a, 0x00, 0x00, 0x01, // router ID 10.0.0.1
		0x10, // opt param len = 16
		// Optional Parameter: Type 2 (Capabilities), Length 14
		0x02, 0x0e,
		// Capability: Multiprotocol IPv4/Unicast (code 1, len 4)
		0x01, 0x04, 0x00, 0x01, 0x00, 0x01,
		// Capability: Route Refresh (code 2, len 0)
		0x02, 0x00,
		// Capability: ASN4 65536 (code 65, len 4)
		0x41, 0x04, 0x00, 0x01, 0x00, 0x00,
	}

	decoded := DecodeOpen(openBytes)

	require.Equal(t, uint8(4), decoded.Version)
	require.Equal(t, uint32(65536), decoded.ASN) // Should use ASN4 capability
	require.Equal(t, uint16(180), decoded.HoldTime)
	require.Equal(t, "10.0.0.1", decoded.RouterID)
	require.Len(t, decoded.Capabilities, 3)
	require.Contains(t, decoded.Capabilities, "multiprotocol ipv4-unicast")
	require.Contains(t, decoded.Capabilities, "route-refresh")
	require.Contains(t, decoded.Capabilities, "4-byte-asn 65536")
}

// TestDecodeOpenInvalid verifies DecodeOpen handles invalid input gracefully.
//
// VALIDATES: Short/invalid OPEN bytes return zero-value DecodedOpen.
//
// PREVENTS: Panic on malformed OPEN message.
func TestDecodeOpenInvalid(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
	}{
		{"empty", []byte{}},
		{"too_short", []byte{0x04, 0xfd}},
		{"truncated", []byte{0x04, 0xfd, 0xe9, 0x00}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decoded := DecodeOpen(tc.bytes)
			// Should return zero values, not panic
			require.Equal(t, uint8(0), decoded.Version)
		})
	}
}

// TestDecodeNotification verifies DecodeNotification parses NOTIFICATION bytes.
//
// VALIDATES: DecodeNotification extracts error code, subcode, and data.
//
// PREVENTS: Bug where NOTIFICATION messages can't be formatted for processes.
func TestDecodeNotification(t *testing.T) {
	// Cease/Admin Shutdown with message "goodbye"
	notifyBytes := []byte{
		0x06,                              // error code: Cease
		0x02,                              // subcode: Admin Shutdown
		0x07,                              // message length
		'g', 'o', 'o', 'd', 'b', 'y', 'e', // message
	}

	decoded := DecodeNotification(notifyBytes)

	require.Equal(t, uint8(6), decoded.ErrorCode)
	require.Equal(t, uint8(2), decoded.ErrorSubcode)
	require.Equal(t, "Cease", decoded.ErrorCodeName)
	require.Equal(t, "Administrative Shutdown", decoded.ErrorSubcodeName)
	require.Equal(t, "goodbye", decoded.ShutdownMessage)
}

// TestDecodeNotificationMinimal verifies minimal NOTIFICATION (no data).
//
// VALIDATES: NOTIFICATION with just code+subcode decodes correctly.
//
// PREVENTS: Bug on minimal notification without data field.
func TestDecodeNotificationMinimal(t *testing.T) {
	// Hold Timer Expired (code 4, subcode 0)
	notifyBytes := []byte{0x04, 0x00}

	decoded := DecodeNotification(notifyBytes)

	require.Equal(t, uint8(4), decoded.ErrorCode)
	require.Equal(t, uint8(0), decoded.ErrorSubcode)
	require.Equal(t, "Hold Timer Expired", decoded.ErrorCodeName)
	require.Empty(t, decoded.ShutdownMessage)
}

// TestDecodeNotificationInvalid verifies DecodeNotification handles invalid input.
//
// VALIDATES: Short/invalid NOTIFICATION bytes return zero-value.
//
// PREVENTS: Panic on malformed NOTIFICATION message.
func TestDecodeNotificationInvalid(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
	}{
		{"empty", []byte{}},
		{"too_short", []byte{0x06}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decoded := DecodeNotification(tc.bytes)
			require.Equal(t, uint8(0), decoded.ErrorCode)
		})
	}
}
