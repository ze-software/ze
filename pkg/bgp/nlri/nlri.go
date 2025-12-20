// Package nlri implements BGP Network Layer Reachability Information types.
//
// Supports all major NLRI families:
//   - INET (IPv4/IPv6 unicast/multicast)
//   - IPVPN (VPNv4/VPNv6)
//   - EVPN (all 5 route types)
//   - FlowSpec
//   - BGP-LS
//   - And more
package nlri

import "fmt"

// String constants for family names.
const familyBGPLS = "bgp-ls"

// AFI represents Address Family Identifier (RFC 4760).
type AFI uint16

// Address Family Identifiers (IANA registry).
const (
	AFIIPv4  AFI = 1
	AFIIPv6  AFI = 2
	AFIL2VPN AFI = 25
	AFIBGPLS AFI = 16388
)

// String returns a human-readable AFI name.
func (a AFI) String() string {
	switch a {
	case AFIIPv4:
		return "ipv4"
	case AFIIPv6:
		return "ipv6"
	case AFIL2VPN:
		return "l2vpn"
	case AFIBGPLS:
		return familyBGPLS
	default:
		return fmt.Sprintf("afi(%d)", a)
	}
}

// SAFI represents Subsequent Address Family Identifier (RFC 4760).
type SAFI uint8

// Subsequent Address Family Identifiers (IANA registry).
const (
	SAFIUnicast   SAFI = 1
	SAFIMulticast SAFI = 2
	SAFIMPLSLabel SAFI = 4
	SAFIEVPN      SAFI = 70
	SAFIVPN       SAFI = 128
	SAFIFlowSpec  SAFI = 133
)

// String returns a human-readable SAFI name.
func (s SAFI) String() string {
	switch s { //nolint:exhaustive // Unknown SAFIs formatted in default
	case SAFIUnicast:
		return "unicast"
	case SAFIMulticast:
		return "multicast"
	case SAFIMPLSLabel:
		return "mpls-label"
	case SAFIMVPN:
		return "mvpn"
	case SAFIEVPN:
		return "evpn"
	case SAFIVPLS:
		return "vpls"
	case SAFIMUP:
		return "mup"
	case SAFIVPN:
		return "vpn"
	case SAFIRTC:
		return "rtc"
	case SAFIFlowSpec:
		return "flowspec"
	case SAFIFlowSpecVPN:
		return "flowspec-vpn"
	case SAFIBGPLinkState:
		return familyBGPLS
	default:
		return fmt.Sprintf("safi(%d)", s)
	}
}

// Family combines AFI and SAFI to identify an address family.
type Family struct {
	AFI  AFI
	SAFI SAFI
}

// Common address families.
var (
	IPv4Unicast   = Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
	IPv6Unicast   = Family{AFI: AFIIPv6, SAFI: SAFIUnicast}
	IPv4Multicast = Family{AFI: AFIIPv4, SAFI: SAFIMulticast}
	IPv6Multicast = Family{AFI: AFIIPv6, SAFI: SAFIMulticast}
	IPv4VPN       = Family{AFI: AFIIPv4, SAFI: SAFIVPN}
	IPv6VPN       = Family{AFI: AFIIPv6, SAFI: SAFIVPN}
	L2VPNEVPN     = Family{AFI: AFIL2VPN, SAFI: SAFIEVPN}
	IPv4FlowSpec  = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}
	IPv6FlowSpec  = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec}
)

// String returns a human-readable family name.
func (f Family) String() string {
	// Handle well-known combinations
	switch {
	case f.AFI == AFIL2VPN && f.SAFI == SAFIEVPN:
		return "l2vpn-evpn"
	case f.AFI == AFIL2VPN && f.SAFI == SAFIVPLS:
		return "l2vpn-vpls"
	case f.AFI == AFIBGPLS && f.SAFI == SAFIBGPLinkState:
		return familyBGPLS
	default:
		return fmt.Sprintf("%s-%s", f.AFI.String(), f.SAFI.String())
	}
}

// familyStrings maps string representations to Family values.
var familyStrings = map[string]Family{
	"ipv4-unicast":      IPv4Unicast,
	"ipv6-unicast":      IPv6Unicast,
	"ipv4-multicast":    IPv4Multicast,
	"ipv6-multicast":    IPv6Multicast,
	"ipv4-vpn":          IPv4VPN,
	"ipv6-vpn":          IPv6VPN,
	"l2vpn-evpn":        L2VPNEVPN,
	"ipv4-flowspec":     IPv4FlowSpec,
	"ipv6-flowspec":     IPv6FlowSpec,
	"ipv4-flowspec-vpn": IPv4FlowSpecVPN,
	"ipv6-flowspec-vpn": IPv6FlowSpecVPN,
	"ipv4-mvpn":         IPv4MVPN,
	"ipv6-mvpn":         IPv6MVPN,
	"l2vpn-vpls":        L2VPNVPLS,
	"ipv4-rtc":          IPv4RTC,
	"ipv4-mup":          IPv4MUP,
	"ipv6-mup":          IPv6MUP,
}

// ParseFamily parses a family string like "ipv4-unicast".
// Returns the family and true if valid, or zero value and false if not.
func ParseFamily(s string) (Family, bool) {
	f, ok := familyStrings[s]
	return f, ok
}

// NLRI represents Network Layer Reachability Information.
//
// This is the core interface for all NLRI types (prefixes, VPN routes,
// EVPN routes, FlowSpec rules, etc.).
type NLRI interface {
	// Family returns the AFI/SAFI for this NLRI.
	Family() Family

	// Bytes returns the wire-format encoding of this NLRI.
	// The returned slice may be shared; do not modify.
	Bytes() []byte

	// Len returns the length in bytes of the wire encoding.
	Len() int

	// String returns a human-readable representation.
	String() string

	// PathID returns the ADD-PATH path identifier (0 if not present).
	PathID() uint32

	// HasPathID returns true if this NLRI has an ADD-PATH path ID.
	HasPathID() bool
}
