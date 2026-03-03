// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4760.md — multiprotocol address families

package message

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// AFI represents Address Family Identifier (RFC 4760).
type AFI uint16

// Address Family Identifiers.
const (
	AFIIPv4  AFI = 1
	AFIIPv6  AFI = 2
	AFIL2VPN AFI = 25
	AFIBGPLS AFI = 16388
)

// SAFI represents Subsequent Address Family Identifier (RFC 4760).
type SAFI uint8

// Subsequent Address Family Identifiers.
const (
	SAFIUnicast   SAFI = 1
	SAFIMulticast SAFI = 2
	SAFIMPLSLabel SAFI = 4
	SAFIVPLS      SAFI = 65
	SAFIEVPN      SAFI = 70
	SAFIVPN       SAFI = 128
	SAFIFlowSpec  SAFI = 133
)

// Canonical family strings (used in output).
// Format: <afi>/<safi> (e.g., "ipv4/unicast").
const (
	FamilyIPv4Unicast   = "ipv4/unicast"
	FamilyIPv6Unicast   = "ipv6/unicast"
	FamilyIPv4Multicast = "ipv4/multicast"
	FamilyIPv6Multicast = "ipv6/multicast"
	FamilyIPv4MPLS      = "ipv4/mpls"
	FamilyIPv6MPLS      = "ipv6/mpls"
	FamilyIPv4MPLSVPN   = "ipv4/mpls-vpn"
	FamilyIPv6MPLSVPN   = "ipv6/mpls-vpn"
	FamilyIPv4FlowSpec  = "ipv4/flow"
	FamilyIPv6FlowSpec  = "ipv6/flow"
	FamilyL2VPNEVPN     = "l2vpn/evpn"
	FamilyL2VPNVPLS     = "l2vpn/vpls"
)

// FamilyConfigNames maps config names (slash-separated) to canonical family strings.
var FamilyConfigNames = map[string]string{
	"ipv4/unicast":   FamilyIPv4Unicast,
	"ipv6/unicast":   FamilyIPv6Unicast,
	"ipv4/multicast": FamilyIPv4Multicast,
	"ipv6/multicast": FamilyIPv6Multicast,
	"ipv4/mpls":      FamilyIPv4MPLS,
	"ipv6/mpls":      FamilyIPv6MPLS,
	"ipv4/mpls-vpn":  FamilyIPv4MPLSVPN,
	"ipv6/mpls-vpn":  FamilyIPv6MPLSVPN,
	"ipv4/flow":      FamilyIPv4FlowSpec,
	"ipv6/flow":      FamilyIPv6FlowSpec,
	"l2vpn/evpn":     FamilyL2VPNEVPN,
	"l2vpn/vpls":     FamilyL2VPNVPLS,
}

// ValidFamilyConfigNames returns a sorted list of valid config family names.
func ValidFamilyConfigNames() string {
	names := make([]string, 0, len(FamilyConfigNames))
	for name := range FamilyConfigNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// AFISAFIToFamily converts AFI/SAFI to canonical family string.
// Returns strings like "ipv4/unicast", "ipv6/flow", "l2vpn/evpn".
func AFISAFIToFamily(afi AFI, safi SAFI) string {
	return nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)}.String()
}
