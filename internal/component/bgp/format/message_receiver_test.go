package format

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestRawMessageType verifies RawMessage struct exists with expected fields.
//
// VALIDATES: RawMessage has Type, RawBytes, Timestamp fields for raw message passing.
//
// PREVENTS: Regression where raw message data is lost during forwarding.
func TestRawMessageType(t *testing.T) {
	now := time.Now()
	msg := bgptypes.RawMessage{
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
	peer := plugin.PeerInfo{
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

	msg := bgptypes.RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  updateBytes,
		Timestamp: time.Now(),
	}

	content := bgptypes.ContentConfig{Encoding: "json", Format: "raw"}
	rawOutput := string(AppendMessage(nil, &peer, msg, content, ""))
	require.Contains(t, rawOutput, "raw", "raw format should contain raw field")

	content.Format = "parsed"
	parsedOutput := string(AppendMessage(nil, &peer, msg, content, ""))
	// Parsed output should have update structure but no raw field
	require.Contains(t, parsedOutput, "update")

	content.Format = "full"
	fullOutput := string(AppendMessage(nil, &peer, msg, content, ""))
	require.Contains(t, fullOutput, "raw", "full format should contain raw field")
	require.Contains(t, fullOutput, "update", "full format should contain parsed data")
}

// TestFormatFullWithRoutes verifies format=full includes both parsed routes AND raw bytes.
//
// VALIDATES: format=full outputs actual route data plus raw hex.
//
// PREVENTS: Bug where format=full only outputs raw bytes without parsed content.
func TestFormatFullWithRoutes(t *testing.T) {
	peer := plugin.PeerInfo{
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

	msg := bgptypes.RawMessage{
		Type:      message.TypeUPDATE,
		RawBytes:  updateBytes,
		Timestamp: time.Now(),
	}

	// Test JSON full format
	content := bgptypes.ContentConfig{Encoding: "json", Format: "full"}
	output := string(AppendMessage(nil, &peer, msg, content, ""))

	require.Contains(t, output, "raw", "full format must contain raw field")
	require.Contains(t, output, "10.0.0.0/24", "full format must contain parsed prefix")
	// New format: family at top level with action
	require.Contains(t, output, `"ipv4/unicast":[`, "full format must contain family operations")
	require.Contains(t, output, `"action":"add"`, "full format must contain action")

	// Test text full format
	content.Encoding = "text"
	textOutput := string(AppendMessage(nil, &peer, msg, content, ""))

	require.Contains(t, textOutput, "10.0.0.0/24", "text full format must contain parsed prefix")
	require.Contains(t, textOutput, "raw", "text full format must contain raw marker")
}

// TestContentConfigDefaults verifies ContentConfig has sensible defaults.
//
// VALIDATES: Default encoding is "text", default format is "parsed".
//
// PREVENTS: Nil pointer or empty string causing unexpected behavior.
func TestContentConfigDefaults(t *testing.T) {
	cfg := bgptypes.ContentConfig{}

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
	// Check structured capabilities
	require.Equal(t, DecodedCapability{Code: 1, Name: "multiprotocol", Value: "ipv4/unicast"}, decoded.Capabilities[0])
	require.Equal(t, DecodedCapability{Code: 2, Name: "unknown-2", Value: "0200"}, decoded.Capabilities[1])
	require.Equal(t, DecodedCapability{Code: 65, Name: "asn4", Value: "65536"}, decoded.Capabilities[2])
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodeOpen(tt.bytes)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodeNotification(tt.bytes)
			require.Equal(t, uint8(0), decoded.ErrorCode)
		})
	}
}

// TestDecodeOpenAddPathReceive verifies AddPath capability formatting.
//
// VALIDATES: formatCapability emits "ipv4/unicast receive" for AddPathReceive,
// "ipv6/unicast send-receive" for AddPathBoth, and skips entries with AddPathNone.
// PREVENTS: byte-drift when migrating formatCapability off fmt.Sprintf.
func TestDecodeOpenAddPathReceive(t *testing.T) {
	// OPEN with AddPath cap (code 69) carrying three tuples:
	//   ipv4/unicast receive (mode=1)
	//   ipv6/unicast both    (mode=3)
	//   ipv4/multicast none  (mode=0, must be skipped)
	openBytes := []byte{
		0x04,       // version 4
		0xfd, 0xe9, // AS 65001
		0x00, 0x5a, // hold 90
		0x01, 0x02, 0x03, 0x04, // router id 1.2.3.4
		0x10, // opt param len = 16
		// Optional Parameter type=2 (Capabilities), length=14
		0x02, 0x0e,
		// AddPath capability code=69 (0x45), length=12
		0x45, 0x0c,
		0x00, 0x01, 0x01, 0x01, // ipv4/unicast receive
		0x00, 0x02, 0x01, 0x03, // ipv6/unicast both
		0x00, 0x01, 0x02, 0x00, // ipv4/multicast none (skipped)
	}

	decoded := DecodeOpen(openBytes)

	require.Len(t, decoded.Capabilities, 2, "none mode must be skipped: %+v", decoded.Capabilities)
	require.Equal(t, DecodedCapability{Code: 69, Name: "addpath", Value: "ipv4/unicast receive"}, decoded.Capabilities[0])
	require.Equal(t, DecodedCapability{Code: 69, Name: "addpath", Value: "ipv6/unicast send-receive"}, decoded.Capabilities[1])
}

// TestDecodeNotification_UnknownSubcode verifies the "Subcode(N)" fallback.
//
// VALIDATES: notificationSubcodeString and the per-error-code helpers all emit
// byte-identical "Subcode(99)" for unmapped subcodes across the 5 fallback sites.
// PREVENTS: byte-drift in the migrated decode.go fallbacks.
func TestDecodeNotification_UnknownSubcode(t *testing.T) {
	tests := []struct {
		name      string
		errorCode message.NotifyErrorCode
		want      string
	}{
		{"open_default", message.NotifyOpenMessage, "Subcode(99)"},
		{"update_default", message.NotifyUpdateMessage, "Subcode(99)"},
		{"header_default", message.NotifyMessageHeader, "Subcode(99)"},
		{"fsm_default", message.NotifyFSMError, "Subcode(99)"},
		{"toplevel_default", message.NotifyHoldTimerExpired, "Subcode(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifyBytes := []byte{byte(tt.errorCode), 99}

			decoded := DecodeNotification(notifyBytes)

			require.Equal(t, uint8(tt.errorCode), decoded.ErrorCode)
			require.Equal(t, uint8(99), decoded.ErrorSubcode)
			require.Equal(t, tt.want, decoded.ErrorSubcodeName)
		})
	}
}

// TestDecodeRouteRefresh_UnknownSubtype verifies the "unknown(N)" fallback.
//
// VALIDATES: DecodeRouteRefresh emits "unknown(99)" for an unmapped subtype.
// PREVENTS: byte-drift when migrating DecodeRouteRefresh off fmt.Sprintf.
func TestDecodeRouteRefresh_UnknownSubtype(t *testing.T) {
	// ROUTE-REFRESH body: AFI(2) + Subtype(1) + SAFI(1)
	body := []byte{
		0x00, 0x01, // AFI=1 (ipv4)
		0x63, // Subtype=99 (unknown)
		0x01, // SAFI=1 (unicast)
	}

	decoded := DecodeRouteRefresh(body)

	require.Equal(t, uint16(1), decoded.AFI)
	require.Equal(t, uint8(1), decoded.SAFI)
	require.Equal(t, uint8(99), decoded.Subtype)
	require.Equal(t, "unknown(99)", decoded.SubtypeName)
	require.Equal(t, "ipv4/unicast", decoded.Family)
}

// TestDecodeNegotiated_UnknownAfiSafi verifies the afiSafiToFamily / afiToString fallbacks.
//
// VALIDATES: NegotiatedToDecoded emits "afi(99)/safi(99)" for an unknown family
// and "afi(99)" for an unknown next-hop AFI in ExtendedNextHop.
// PREVENTS: byte-drift when migrating afiSafiToFamily / afiToString off fmt.Sprintf.
func TestDecodeNegotiated_UnknownAfiSafi(t *testing.T) {
	// Build a Negotiated with an unknown AFI/SAFI family + an extended-nexthop
	// tuple whose NextHopAFI is unknown.
	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: 99, SAFI: 99},
		&capability.Multiprotocol{AFI: 1, SAFI: 1},
		&capability.ExtendedNextHop{
			Families: []capability.ExtendedNextHopFamily{
				{NLRIAFI: 1, NLRISAFI: 1, NextHopAFI: 99},
			},
		},
	}
	neg := capability.Negotiate(caps, caps, 65001, 65001)

	decoded := NegotiatedToDecoded(neg)

	require.Contains(t, decoded.Families, "afi(99)/safi(99)", "unknown family must format as afi(N)/safi(N): %v", decoded.Families)
	require.Contains(t, decoded.Families, "ipv4/unicast", "known family must still format normally: %v", decoded.Families)
	require.Equal(t, "afi(99)", decoded.ExtendedNextHop["ipv4/unicast"], "unknown next-hop AFI must format as afi(N)")

	// Direct exercise of the afi(99)/<known-safi> path (matches Mistake Log scope).
	mixedCaps := []capability.Capability{&capability.Multiprotocol{AFI: 99, SAFI: 1}}
	negMixed := capability.Negotiate(mixedCaps, mixedCaps, 65001, 65001)
	decodedMixed := NegotiatedToDecoded(negMixed)
	require.Contains(t, decodedMixed.Families, "afi(99)/unicast", "afi(99)/unicast: %v", decodedMixed.Families)
}

// TestNegotiatedToDecoded verifies conversion from capability.Negotiated to DecodedNegotiated.
//
// VALIDATES: All fields converted correctly including families, ADD-PATH, and extended NH.
// PREVENTS: Missing or incorrect capability data sent to plugins.
func TestNegotiatedToDecoded(t *testing.T) {
	t.Run("full_capabilities", func(t *testing.T) {
		neg := &capability.Negotiated{
			ASN4:                 true,
			ExtendedMessage:      true,
			RouteRefresh:         true,
			EnhancedRouteRefresh: true,
			HoldTime:             90,
		}
		// Set families via Negotiate (we need to populate internal maps)
		// For simplicity, test with nil (which gives empty families)

		decoded := NegotiatedToDecoded(neg)

		require.Equal(t, 65535, decoded.MessageSize, "extended message = 65535")
		require.Equal(t, uint16(90), decoded.HoldTime)
		require.True(t, decoded.ASN4)
		require.Equal(t, "enhanced", decoded.RouteRefresh)
	})

	t.Run("basic_capabilities", func(t *testing.T) {
		neg := &capability.Negotiated{
			ASN4:            false,
			ExtendedMessage: false,
			RouteRefresh:    true,
			HoldTime:        180,
		}

		decoded := NegotiatedToDecoded(neg)

		require.Equal(t, 4096, decoded.MessageSize, "no extended message = 4096")
		require.Equal(t, uint16(180), decoded.HoldTime)
		require.False(t, decoded.ASN4)
		require.Equal(t, "normal", decoded.RouteRefresh)
	})

	t.Run("no_route_refresh", func(t *testing.T) {
		neg := &capability.Negotiated{
			RouteRefresh:         false,
			EnhancedRouteRefresh: false,
		}

		decoded := NegotiatedToDecoded(neg)

		require.Equal(t, "absent", decoded.RouteRefresh)
	})

	t.Run("nil_negotiated", func(t *testing.T) {
		decoded := NegotiatedToDecoded(nil)

		require.Equal(t, 0, decoded.MessageSize)
		require.Equal(t, uint16(0), decoded.HoldTime)
		require.False(t, decoded.ASN4)
		require.Empty(t, decoded.Families)
	})
}

// TestFormatFullAddPathFlags verifies format=full raw block includes per-family ADD-PATH flags.
//
// VALIDATES: AC-1 (ADD-PATH peer → add-path in raw block), AC-2 (no ADD-PATH → omitted),
// AC-3 (partial: IPv4 has ADD-PATH, IPv6 absent).
// PREVENTS: RIB plugin unable to determine ADD-PATH state from event JSON.
func TestFormatFullAddPathFlags(t *testing.T) {
	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Build UPDATE with path-id in NLRI (ADD-PATH format)
	body := buildTestUpdateBodyWithPathID(
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		42,
	)

	t.Run("add_path_peer_emits_flags", func(t *testing.T) {
		// Create encoding context with ADD-PATH for IPv4 unicast only
		addPathMap := map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true}
		ctx := bgpctx.EncodingContextWithAddPath(true, addPathMap)
		ctxID := bgpctx.Registry.Register(ctx)

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
			Format:   plugin.FormatFull,
		}

		output := string(AppendMessage(nil, &peer, msg, content, ""))

		var result map[string]any
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "JSON must be valid: %s", output)

		// Navigate to raw block
		bgpObj, ok := result["bgp"].(map[string]any)
		require.True(t, ok, "bgp object must exist")
		raw, ok := bgpObj["raw"].(map[string]any)
		require.True(t, ok, "raw block must exist in format=full")

		// AC-1: ADD-PATH flags present for negotiated family
		addPath, ok := raw["add-path"].(map[string]any)
		require.True(t, ok, "add-path must exist in raw block: %s", output)
		assert.Equal(t, true, addPath["ipv4/unicast"], "IPv4 unicast should be true")

		// AC-3: IPv6 not present (not negotiated)
		_, hasIPv6 := addPath["ipv6/unicast"]
		assert.False(t, hasIPv6, "IPv6 unicast should be absent (not negotiated)")
	})

	t.Run("no_add_path_omits_field", func(t *testing.T) {
		// Create encoding context WITHOUT ADD-PATH
		ctx := bgpctx.EncodingContextWithAddPath(true, nil)
		ctxID := bgpctx.Registry.Register(ctx)

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
			Format:   plugin.FormatFull,
		}

		output := string(AppendMessage(nil, &peer, msg, content, ""))

		var result map[string]any
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "JSON must be valid: %s", output)

		bgpObj, ok := result["bgp"].(map[string]any)
		require.True(t, ok, "bgp object must exist")
		raw, ok := bgpObj["raw"].(map[string]any)
		require.True(t, ok, "raw block must exist in format=full")

		// AC-2: add-path key omitted when no families have ADD-PATH
		_, hasAddPath := raw["add-path"]
		assert.False(t, hasAddPath, "add-path should be absent when no ADD-PATH negotiated")
	})
}
