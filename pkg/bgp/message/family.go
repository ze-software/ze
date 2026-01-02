package message

import (
	"fmt"
	"sort"
	"strings"
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
const (
	FamilyIPv4Unicast   = "ipv4 unicast"
	FamilyIPv6Unicast   = "ipv6 unicast"
	FamilyIPv4Multicast = "ipv4 multicast"
	FamilyIPv6Multicast = "ipv6 multicast"
	FamilyIPv4MPLS      = "ipv4 mpls"
	FamilyIPv6MPLS      = "ipv6 mpls"
	FamilyIPv4MPLSVPN   = "ipv4 mpls-vpn"
	FamilyIPv6MPLSVPN   = "ipv6 mpls-vpn"
	FamilyIPv4FlowSpec  = "ipv4 flowspec"
	FamilyIPv6FlowSpec  = "ipv6 flowspec"
	FamilyL2VPNEVPN     = "l2vpn evpn"
	FamilyL2VPNVPLS     = "l2vpn vpls"
)

// FamilyConfigNames maps config names (hyphenated) to canonical family strings (spaced).
var FamilyConfigNames = map[string]string{
	"ipv4-unicast":   FamilyIPv4Unicast,
	"ipv6-unicast":   FamilyIPv6Unicast,
	"ipv4-multicast": FamilyIPv4Multicast,
	"ipv6-multicast": FamilyIPv6Multicast,
	"ipv4-mpls":      FamilyIPv4MPLS,
	"ipv6-mpls":      FamilyIPv6MPLS,
	"ipv4-mpls-vpn":  FamilyIPv4MPLSVPN,
	"ipv6-mpls-vpn":  FamilyIPv6MPLSVPN,
	"ipv4-flowspec":  FamilyIPv4FlowSpec,
	"ipv6-flowspec":  FamilyIPv6FlowSpec,
	"l2vpn-evpn":     FamilyL2VPNEVPN,
	"l2vpn-vpls":     FamilyL2VPNVPLS,
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
// Returns strings like "ipv4 unicast", "ipv6 flowspec", "l2vpn evpn".
func AFISAFIToFamily(afi AFI, safi SAFI) string {
	switch {
	case afi == AFIIPv4 && safi == SAFIUnicast:
		return FamilyIPv4Unicast
	case afi == AFIIPv6 && safi == SAFIUnicast:
		return FamilyIPv6Unicast
	case afi == AFIIPv4 && safi == SAFIMulticast:
		return FamilyIPv4Multicast
	case afi == AFIIPv6 && safi == SAFIMulticast:
		return FamilyIPv6Multicast
	case afi == AFIIPv4 && safi == SAFIMPLSLabel:
		return FamilyIPv4MPLS
	case afi == AFIIPv6 && safi == SAFIMPLSLabel:
		return FamilyIPv6MPLS
	case afi == AFIIPv4 && safi == SAFIVPN:
		return FamilyIPv4MPLSVPN
	case afi == AFIIPv6 && safi == SAFIVPN:
		return FamilyIPv6MPLSVPN
	case afi == AFIIPv4 && safi == SAFIFlowSpec:
		return FamilyIPv4FlowSpec
	case afi == AFIIPv6 && safi == SAFIFlowSpec:
		return FamilyIPv6FlowSpec
	case afi == AFIL2VPN && safi == SAFIEVPN:
		return FamilyL2VPNEVPN
	case afi == AFIL2VPN && safi == SAFIVPLS:
		return FamilyL2VPNVPLS
	default:
		return fmt.Sprintf("afi-%d safi-%d", afi, safi)
	}
}
