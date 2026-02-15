// Package peer implements BGP session handling for the chaos testing tool.
// It builds and exchanges BGP messages (OPEN, KEEPALIVE, UPDATE, NOTIFICATION)
// using Ze's wire-encoding packages.
package peer

import (
	"encoding/binary"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// SessionConfig holds the parameters needed to establish a BGP session.
type SessionConfig struct {
	// ASN is the local autonomous system number.
	ASN uint32

	// RouterID is the BGP router identifier.
	RouterID netip.Addr

	// HoldTime is the proposed hold time in seconds.
	HoldTime uint16
}

// BuildOpen constructs a BGP OPEN message from the session config.
// It includes ASN4, multiprotocol (ipv4/unicast), and route-refresh capabilities.
func BuildOpen(cfg SessionConfig) *message.Open {
	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
		&capability.ASN4{ASN: cfg.ASN},
		&capability.RouteRefresh{},
	}

	optParams := buildOptionalParams(caps)

	myAS := uint16(cfg.ASN) //nolint:gosec // Truncation intended for AS_TRANS
	if cfg.ASN > 65535 {
		myAS = message.AS_TRANS
	}

	rid := cfg.RouterID.As4()

	return &message.Open{
		Version:        4,
		MyAS:           myAS,
		HoldTime:       cfg.HoldTime,
		BGPIdentifier:  binary.BigEndian.Uint32(rid[:]),
		ASN4:           cfg.ASN,
		OptionalParams: optParams,
	}
}

// buildOptionalParams builds optional parameters from capabilities.
// Each capability is wrapped in its own parameter (type 2) per RFC 5492.
func buildOptionalParams(caps []capability.Capability) []byte {
	if len(caps) == 0 {
		return nil
	}

	total := 0
	for _, c := range caps {
		total += 2 + c.Len() // param type (1) + param length (1) + cap TLV
	}

	buf := make([]byte, total)
	off := 0

	for _, c := range caps {
		capLen := c.Len()
		buf[off] = 2              // Parameter type: Capabilities (RFC 5492)
		buf[off+1] = byte(capLen) //nolint:gosec // Capability TLVs are always <256 bytes
		off += 2
		off += c.WriteTo(buf, off)
	}

	return buf
}

// BuildCeaseNotification creates a NOTIFICATION message for clean shutdown.
// Uses Cease (6) / Administrative Shutdown (2) per RFC 4271.
func BuildCeaseNotification() *message.Notification {
	return &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseAdminShutdown,
	}
}

// SerializeMessage serializes any BGP message to wire bytes.
func SerializeMessage(msg message.Message) []byte {
	size := msg.Len(nil)
	buf := make([]byte, size)
	msg.WriteTo(buf, 0, nil)

	return buf
}
