package peer

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/message"
)

// TestOpenMessageBuild verifies that BuildOpen produces a valid OPEN
// message with correct ASN, capabilities, and router ID.
//
// VALIDATES: OPEN message construction with ASN4 and multiprotocol.
// PREVENTS: Malformed OPEN causing session establishment failure.
func TestOpenMessageBuild(t *testing.T) {
	cfg := SessionConfig{
		ASN:      65001,
		RouterID: netip.MustParseAddr("10.255.0.1"),
		HoldTime: 90,
	}

	open := BuildOpen(cfg)

	assert.Equal(t, uint8(4), open.Version)
	assert.Equal(t, uint16(65001), open.MyAS)
	assert.Equal(t, uint16(90), open.HoldTime)
	assert.Equal(t, uint32(65001), open.ASN4)
	// Router ID should be encoded as uint32.
	rid := netip.MustParseAddr("10.255.0.1").As4()
	expected := binary.BigEndian.Uint32(rid[:])
	assert.Equal(t, expected, open.BGPIdentifier)
	// Should have optional params (capabilities).
	assert.Greater(t, len(open.OptionalParams), 0, "should have capabilities")
}

// TestOpenMessageASN4Large verifies ASN4 > 65535 uses AS_TRANS.
//
// VALIDATES: RFC 6793 AS_TRANS handling for large ASNs.
// PREVENTS: Large ASN breaking 2-byte MyAS field.
func TestOpenMessageASN4Large(t *testing.T) {
	cfg := SessionConfig{
		ASN:      200000,
		RouterID: netip.MustParseAddr("10.255.0.1"),
		HoldTime: 90,
	}

	open := BuildOpen(cfg)

	assert.Equal(t, uint16(message.AS_TRANS), open.MyAS)
	assert.Equal(t, uint32(200000), open.ASN4)
}

// TestOpenMessageWriteTo verifies the OPEN can be serialized to wire bytes.
//
// VALIDATES: OPEN serialization produces valid wire format.
// PREVENTS: Buffer overflow or short write during serialization.
func TestOpenMessageWriteTo(t *testing.T) {
	cfg := SessionConfig{
		ASN:      65001,
		RouterID: netip.MustParseAddr("10.255.0.1"),
		HoldTime: 90,
	}

	open := BuildOpen(cfg)
	buf := make([]byte, open.Len(nil))
	n := open.WriteTo(buf, 0, nil)

	assert.Equal(t, len(buf), n)
	// Check marker (first 16 bytes all 0xFF).
	for i := range 16 {
		assert.Equal(t, byte(0xFF), buf[i], "marker byte %d", i)
	}
	// Message type should be OPEN (1).
	assert.Equal(t, byte(1), buf[18])
}

// TestKeepaliveWrite verifies KEEPALIVE serialization.
//
// VALIDATES: KEEPALIVE produces 19-byte header-only message.
// PREVENTS: Wrong KEEPALIVE format breaking session maintenance.
func TestKeepaliveWrite(t *testing.T) {
	ka := message.NewKeepalive()
	buf := make([]byte, ka.Len(nil))
	n := ka.WriteTo(buf, 0, nil)

	assert.Equal(t, 19, n)
	// Message type should be KEEPALIVE (4).
	assert.Equal(t, byte(4), buf[18])
}

// TestNotificationCease verifies building a NOTIFICATION Cease message.
//
// VALIDATES: Cease notification for clean shutdown.
// PREVENTS: Wrong error code in shutdown notification.
func TestNotificationCease(t *testing.T) {
	notif := BuildCeaseNotification()

	assert.Equal(t, message.NotifyCease, notif.ErrorCode)
	assert.Equal(t, message.NotifyCeaseAdminShutdown, notif.ErrorSubcode)

	buf := make([]byte, notif.Len(nil))
	n := notif.WriteTo(buf, 0, nil)

	require.Equal(t, len(buf), n)
	// Message type should be NOTIFICATION (3).
	assert.Equal(t, byte(3), buf[18])
}

// TestSerializeMessage verifies the generic message serialization helper.
//
// VALIDATES: SerializeMessage produces correct wire bytes for any message type.
// PREVENTS: Serialization buffer sizing errors.
func TestSerializeMessage(t *testing.T) {
	ka := message.NewKeepalive()
	data := SerializeMessage(ka)

	assert.Equal(t, 19, len(data))
	assert.Equal(t, byte(4), data[18]) // KEEPALIVE type
}

// TestOpenMultiFamily verifies that BuildOpen includes Multiprotocol
// capabilities for all specified families.
//
// VALIDATES: Multi-family OPEN includes one Multiprotocol cap per family.
// PREVENTS: Missing capabilities causing family negotiation failure.
func TestOpenMultiFamily(t *testing.T) {
	cfg := SessionConfig{
		ASN:      65001,
		RouterID: netip.MustParseAddr("10.255.0.1"),
		HoldTime: 90,
		Families: []string{"ipv4/unicast", "ipv6/unicast", "l2vpn/evpn"},
	}

	open := BuildOpen(cfg)

	// Serialized OPEN should be larger than one with just ipv4/unicast.
	singleCfg := SessionConfig{
		ASN:      65001,
		RouterID: netip.MustParseAddr("10.255.0.1"),
		HoldTime: 90,
	}
	singleOpen := BuildOpen(singleCfg)

	assert.Greater(t, len(open.OptionalParams), len(singleOpen.OptionalParams),
		"multi-family OPEN should have more capability bytes")

	// Should serialize without error.
	buf := make([]byte, open.Len(nil))
	n := open.WriteTo(buf, 0, nil)
	assert.Equal(t, len(buf), n)
}

// TestOpenDefaultFamily verifies that empty Families defaults to ipv4/unicast.
//
// VALIDATES: Backward compatibility when Families is nil.
// PREVENTS: Empty OPEN when no families specified.
func TestOpenDefaultFamily(t *testing.T) {
	withFamilies := BuildOpen(SessionConfig{
		ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), HoldTime: 90,
		Families: []string{"ipv4/unicast"},
	})
	withoutFamilies := BuildOpen(SessionConfig{
		ASN: 65001, RouterID: netip.MustParseAddr("10.255.0.1"), HoldTime: 90,
	})

	assert.Equal(t, len(withFamilies.OptionalParams), len(withoutFamilies.OptionalParams),
		"nil families should produce same OPEN as explicit ipv4/unicast")
}
