// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation
//
// Package peer implements BGP session handling for the chaos testing tool.
// It builds and exchanges BGP messages (OPEN, KEEPALIVE, UPDATE, NOTIFICATION)
// using Ze's wire-encoding packages.
package peer

import (
	"encoding/binary"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// SessionConfig holds the parameters needed to establish a BGP session.
type SessionConfig struct {
	// ASN is the local autonomous system number.
	ASN uint32

	// RouterID is the BGP router identifier.
	RouterID netip.Addr

	// HoldTime is the proposed hold time in seconds.
	HoldTime uint16

	// Families is the list of address families to negotiate.
	// Empty defaults to ["ipv4/unicast"].
	Families []string
}

// familyAFISAFI holds an AFI/SAFI pair for Multiprotocol capability construction.
type familyAFISAFI struct {
	afi  family.AFI
	safi family.SAFI
}

// familyToAFISAFI maps family strings to (AFI, SAFI) pairs for Multiprotocol capabilities.
// SYNC: Must stay in sync with familyToNLRI in sender.go — both maps
// must cover the same set of family strings.
var familyToAFISAFI = map[string]familyAFISAFI{
	"ipv4/unicast":   {family.AFIIPv4, family.SAFIUnicast},
	"ipv6/unicast":   {family.AFIIPv6, family.SAFIUnicast},
	"ipv4/multicast": {family.AFIIPv4, family.SAFIMulticast},
	"ipv6/multicast": {family.AFIIPv6, family.SAFIMulticast},
	"ipv4/mpls-vpn":  {family.AFIIPv4, family.SAFIVPN},
	"ipv6/mpls-vpn":  {family.AFIIPv6, family.SAFIVPN},
	"l2vpn/evpn":     {family.AFIL2VPN, family.SAFIEVPN},
	"ipv4/flow":      {family.AFIIPv4, family.SAFIFlowSpec},
	"ipv6/flow":      {family.AFIIPv6, family.SAFIFlowSpec},
}

// BuildOpen constructs a BGP OPEN message from the session config.
// It includes ASN4, multiprotocol capabilities for each family, and route-refresh.
func BuildOpen(cfg SessionConfig) *message.Open {
	families := cfg.Families
	if len(families) == 0 {
		families = []string{"ipv4/unicast"}
	}

	var caps []capability.Capability
	for _, f := range families {
		pair, ok := familyToAFISAFI[f]
		if !ok {
			continue
		}
		caps = append(caps, &capability.Multiprotocol{
			AFI:  pair.afi,
			SAFI: pair.safi,
		})
	}
	caps = append(caps, &capability.ASN4{ASN: cfg.ASN}, &capability.RouteRefresh{})

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
