// Package nlri implements BGP Network Layer Reachability Information types.
//
// RFC 4271 Section 4.3 defines the base NLRI encoding for IPv4 prefixes as
// a 2-tuple of <length, prefix> where length is the prefix length in bits
// and prefix contains the minimum number of octets to represent the prefix.
//
// RFC 4760 extends this to support multiple address families via the
// MP_REACH_NLRI (Type Code 14) and MP_UNREACH_NLRI (Type Code 15) path
// attributes. Section 5 of RFC 4760 defines the same <length, prefix>
// encoding for multiprotocol NLRI.
//
// Supports all major NLRI families:
//   - INET (IPv4/IPv6 unicast/multicast) - RFC 4271, RFC 4760
//   - IPVPN (VPNv4/VPNv6) - RFC 4364, RFC 4659
//   - EVPN (all 5 route types) - RFC 7432
//   - FlowSpec - RFC 8955
//   - BGP-LS - RFC 9552
//   - And more
package nlri

import "fmt"

// String constants for family names.
const familyBGPLS = "bgp-ls"

// AFI represents Address Family Identifier.
// RFC 4760 Section 3: AFI is a 2-octet field in MP_REACH_NLRI/MP_UNREACH_NLRI.
// Values are assigned by IANA Address Family Numbers registry.
type AFI uint16

// Address Family Identifiers.
// RFC 4760 Section 3: "Presently defined values for the Address Family
// Identifier field are specified in the IANA's Address Family Numbers registry"
// See: https://www.iana.org/assignments/address-family-numbers/
const (
	AFIIPv4  AFI = 1     // IPv4 - RFC 4760
	AFIIPv6  AFI = 2     // IPv6 - RFC 4760
	AFIL2VPN AFI = 25    // L2VPN - RFC 4761, RFC 7432
	AFIBGPLS AFI = 16388 // BGP-LS - RFC 9552
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

// SAFI represents Subsequent Address Family Identifier.
// RFC 4760 Section 3: SAFI is a 1-octet field in MP_REACH_NLRI/MP_UNREACH_NLRI.
// RFC 4760 Section 6 defines values 1 (unicast) and 2 (multicast).
// Additional values are assigned by IANA SAFI registry.
type SAFI uint8

// Subsequent Address Family Identifiers.
// RFC 4760 Section 6 defines base values. Additional values from IANA registry.
// See: https://www.iana.org/assignments/safi-namespace/
const (
	SAFIUnicast   SAFI = 1   // RFC 4760 Section 6
	SAFIMulticast SAFI = 2   // RFC 4760 Section 6
	SAFIMPLSLabel SAFI = 4   // RFC 8277
	SAFIEVPN      SAFI = 70  // RFC 7432
	SAFIVPN       SAFI = 128 // RFC 4364 (VPNv4), RFC 4659 (VPNv6)
	SAFIFlowSpec  SAFI = 133 // RFC 8955
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
// RFC 4760 Section 3: The combination of <AFI, SAFI> identifies the semantics
// of the Network Layer Reachability Information that follows.
type Family struct {
	AFI  AFI
	SAFI SAFI
}

// Common address families.
// These are the most commonly used AFI/SAFI combinations.
var (
	IPv4Unicast        = Family{AFI: AFIIPv4, SAFI: SAFIUnicast}   // RFC 4271
	IPv6Unicast        = Family{AFI: AFIIPv6, SAFI: SAFIUnicast}   // RFC 4760
	IPv4Multicast      = Family{AFI: AFIIPv4, SAFI: SAFIMulticast} // RFC 4760
	IPv6Multicast      = Family{AFI: AFIIPv6, SAFI: SAFIMulticast} // RFC 4760
	IPv4LabeledUnicast = Family{AFI: AFIIPv4, SAFI: SAFIMPLSLabel} // RFC 8277
	IPv6LabeledUnicast = Family{AFI: AFIIPv6, SAFI: SAFIMPLSLabel} // RFC 8277
	IPv4VPN            = Family{AFI: AFIIPv4, SAFI: SAFIVPN}       // RFC 4364
	IPv6VPN            = Family{AFI: AFIIPv6, SAFI: SAFIVPN}       // RFC 4659
	L2VPNEVPN          = Family{AFI: AFIL2VPN, SAFI: SAFIEVPN}     // RFC 7432
	IPv4FlowSpec       = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}  // RFC 8955
	IPv6FlowSpec       = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec}  // RFC 8955
)

// FamilyLess provides deterministic ordering for sorted iteration.
// Orders by AFI first, then SAFI. Used for consistent EOR ordering in tests.
func FamilyLess(a, b Family) bool {
	if a.AFI != b.AFI {
		return a.AFI < b.AFI
	}
	return a.SAFI < b.SAFI
}

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
	"ipv4-mpls-label":   IPv4LabeledUnicast,
	"ipv6-mpls-label":   IPv6LabeledUnicast,
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
// RFC 4271 Section 4.3 defines NLRI for IPv4 unicast as a variable-length
// field containing one or more 2-tuples of the form <length, prefix>:
//
//	+---------------------------+
//	|   Length (1 octet)        |  <- prefix length in bits
//	+---------------------------+
//	|   Prefix (variable)       |  <- minimum octets to contain prefix
//	+---------------------------+
//
// RFC 4760 Section 5 extends this encoding to all address families.
//
// RFC 7911 Section 3 extends the encoding with an optional Path Identifier
// for ADD-PATH support:
//
//	+--------------------------------+
//	| Path Identifier (4 octets)     |  <- only when ADD-PATH negotiated
//	+--------------------------------+
//	| Length (1 octet)               |
//	+--------------------------------+
//	| Prefix (variable)              |
//	+--------------------------------+
//
// This is the core interface for all NLRI types (prefixes, VPN routes,
// EVPN routes, FlowSpec rules, etc.).
type NLRI interface {
	// Family returns the AFI/SAFI for this NLRI.
	// RFC 4760 Section 3: <AFI, SAFI> identifies NLRI semantics.
	Family() Family

	// Bytes returns the wire-format encoding of this NLRI.
	// RFC 4271 Section 4.3: Encoded as <length, prefix> tuples.
	// The returned slice may be shared; do not modify.
	//
	// Note: For capability-aware encoding (ADD-PATH, etc.), use Pack() instead.
	Bytes() []byte

	// Pack returns wire-format bytes adapted for negotiated capabilities.
	//
	// RFC 7911 Section 3: Handles ADD-PATH path identifier based on ctx.AddPath:
	//   - If ctx is nil: behaves like Bytes()
	//   - If ctx.AddPath=true and HasPathID()=true: returns with path ID
	//   - If ctx.AddPath=true and HasPathID()=false: prepends NOPATH (4 zeros)
	//   - If ctx.AddPath=false: returns without path ID (strips if present)
	Pack(ctx *PackContext) []byte

	// Len returns the length in bytes of the wire encoding.
	Len() int

	// String returns a human-readable representation.
	String() string

	// PathID returns the ADD-PATH path identifier (0 if not present).
	// RFC 7911 Section 3: Path Identifier is a 4-octet field.
	PathID() uint32

	// HasPathID returns true if this NLRI has an ADD-PATH path ID.
	// RFC 7911: Path ID is present when ADD-PATH capability is negotiated.
	HasPathID() bool
}
