// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation
//
// Package peer implements BGP session handling for the chaos testing tool.
// It builds and exchanges BGP messages (OPEN, KEEPALIVE, UPDATE, NOTIFICATION)
// using Ze's wire-encoding packages.
package peer

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
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

// BuildOpen constructs a BGP OPEN message from the session config.
// It includes ASN4, multiprotocol capabilities for each family, and route-refresh.
//
// Family names are resolved via family.LookupFamily; unregistered names are
// silently skipped (matching the previous behavior when the hardcoded lookup
// table did not cover the entry).
//
// Every capability rides in one Capabilities parameter through
// capability.PackOptionalParams, so the parameters use the RFC 9072 framing
// whenever the envelope is extended; its size error is returned.
func BuildOpen(cfg SessionConfig) (*message.Open, error) {
	families := cfg.Families
	if len(families) == 0 {
		families = []string{"ipv4/unicast"}
	}

	var caps []capability.Capability
	for _, f := range families {
		fam, ok := family.LookupFamily(f)
		if !ok {
			continue
		}
		caps = append(caps, &capability.Multiprotocol{
			AFI:  fam.AFI,
			SAFI: fam.SAFI,
		})
	}
	caps = append(caps, &capability.ASN4{ASN: cfg.ASN}, &capability.RouteRefresh{})

	// One Capabilities parameter for every capability, framed by the one packer.
	// RFC 9072 Section 2
	optParams, extended, err := capability.PackOptionalParams([]capability.OptionalParam{capability.CapabilitiesParam(caps)})
	if err != nil {
		return nil, fmt.Errorf("chaos OPEN: %w", err)
	}

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
		ExtendedParams: extended,
	}, nil
}

// BuildCeaseNotification creates a NOTIFICATION message for clean shutdown.
// Uses Cease (6) / Administrative Shutdown (2) per RFC 4271.
func BuildCeaseNotification() *message.Notification {
	return &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseAdminShutdown,
	}
}

// serializeMessage serializes any BGP message to wire bytes.
func serializeMessage(msg message.Message) []byte {
	size := msg.Len(nil)
	buf := make([]byte, size)
	msg.WriteTo(buf, 0, nil)

	return buf
}
