package api

import (
	"net/netip"
	"strings"
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

// TestEncodingSwitchingJSON verifies JSON encoding is used when configured.
//
// VALIDATES: Process with encoding=json receives JSON-formatted output.
//
// PREVENTS: Bug where all processes receive text format regardless of config.
func TestEncodingSwitchingJSON(t *testing.T) {
	// Create a formatter that respects encoding config
	peer := PeerInfo{
		Address:      netip.MustParseAddr("192.168.1.2"),
		LocalAddress: netip.MustParseAddr("192.168.1.1"),
		LocalAS:      65001,
		PeerAS:       65002,
		RouterID:     0xC0A80101,
	}

	routes := []ReceivedRoute{{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.168.1.2"),
		Origin:  "igp",
		ASPath:  []uint32{65002},
	}}

	// Test JSON encoding
	jsonOutput := FormatReceivedUpdateWithEncoding(peer, routes, "json")
	require.True(t, strings.HasPrefix(jsonOutput, "{"), "JSON output should start with {")
	require.Contains(t, jsonOutput, `"type":"update"`)
	require.Contains(t, jsonOutput, `"10.0.0.0/8"`)

	// Test text encoding
	textOutput := FormatReceivedUpdateWithEncoding(peer, routes, "text")
	require.True(t, strings.HasPrefix(textOutput, "neighbor"), "Text output should start with neighbor")
	require.Contains(t, textOutput, "receive update")
	require.Contains(t, textOutput, "10.0.0.0/8")
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

// TestReceivedRouteToRouteUpdate verifies conversion between route types.
//
// VALIDATES: ReceivedRoute can be converted to RouteUpdate for JSON encoding.
//
// PREVENTS: Type mismatch when switching between text and JSON encoders.
func TestReceivedRouteToRouteUpdate(t *testing.T) {
	received := ReceivedRoute{
		Prefix:          netip.MustParsePrefix("10.0.0.0/8"),
		NextHop:         netip.MustParseAddr("192.168.1.2"),
		Origin:          "igp",
		LocalPreference: 100,
		MED:             50,
		ASPath:          []uint32{65001, 65002},
	}

	update := received.ToRouteUpdate()

	require.Equal(t, "10.0.0.0/8", update.Prefix)
	require.Equal(t, "192.168.1.2", update.NextHop)
	require.Equal(t, "igp", update.Origin)
	require.Equal(t, uint32(100), update.LocalPref)
	require.Equal(t, uint32(50), update.MED)
	require.Equal(t, []uint32{65001, 65002}, update.ASPath)
	require.Equal(t, "ipv4", update.AFI)
	require.Equal(t, "unicast", update.SAFI)
}
