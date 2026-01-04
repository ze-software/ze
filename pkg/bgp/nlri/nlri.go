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

import (
	"encoding/binary"
	"fmt"
)

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
		return fmt.Sprintf("afi-%d", a)
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
		return fmt.Sprintf("safi-%d", s)
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
// Format: <afi>/<safi> (e.g., "ipv4/unicast", "l2vpn/evpn").
func (f Family) String() string {
	// Handle well-known combinations
	switch {
	case f.AFI == AFIL2VPN && f.SAFI == SAFIEVPN:
		return "l2vpn/evpn"
	case f.AFI == AFIL2VPN && f.SAFI == SAFIVPLS:
		return "l2vpn/vpls"
	case f.AFI == AFIBGPLS && f.SAFI == SAFIBGPLinkState:
		return "bgp-ls/bgp-ls"
	default:
		return fmt.Sprintf("%s/%s", f.AFI.String(), f.SAFI.String())
	}
}

// familyStrings maps string representations to Family values.
// Format: <afi>/<safi> (e.g., "ipv4/unicast").
// Includes aliases for config compatibility.
var familyStrings = map[string]Family{
	// Primary names
	"ipv4/unicast":      IPv4Unicast,
	"ipv6/unicast":      IPv6Unicast,
	"ipv4/multicast":    IPv4Multicast,
	"ipv6/multicast":    IPv6Multicast,
	"ipv4/mpls-label":   IPv4LabeledUnicast,
	"ipv6/mpls-label":   IPv6LabeledUnicast,
	"ipv4/vpn":          IPv4VPN,
	"ipv6/vpn":          IPv6VPN,
	"l2vpn/evpn":        L2VPNEVPN,
	"ipv4/flowspec":     IPv4FlowSpec,
	"ipv6/flowspec":     IPv6FlowSpec,
	"ipv4/flowspec-vpn": IPv4FlowSpecVPN,
	"ipv6/flowspec-vpn": IPv6FlowSpecVPN,
	"ipv4/mvpn":         IPv4MVPN,
	"ipv6/mvpn":         IPv6MVPN,
	"l2vpn/vpls":        L2VPNVPLS,
	"ipv4/rtc":          IPv4RTC,
	"ipv4/mup":          IPv4MUP,
	"ipv6/mup":          IPv6MUP,
	// Config aliases
	"ipv4/mpls-vpn":  IPv4VPN,
	"ipv6/mpls-vpn":  IPv6VPN,
	"ipv4/nlri-mpls": IPv4LabeledUnicast,
	"ipv6/nlri-mpls": IPv6LabeledUnicast,
	"ipv4/flow":      IPv4FlowSpec,
	"ipv6/flow":      IPv6FlowSpec,
	"ipv4/flow-vpn":  IPv4FlowSpecVPN,
	"ipv6/flow-vpn":  IPv6FlowSpecVPN,
	"ipv4/mcast-vpn": IPv4MVPN,
	"ipv6/mcast-vpn": IPv6MVPN,
	// BGP-LS
	"bgp-ls/bgp-ls": {AFI: AFIBGPLS, SAFI: 71}, // SAFIBGPLinkState
}

// ParseFamily parses a family string like "ipv4/unicast".
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
//
// Phase 3 simplification: Len()/Bytes()/WriteTo() return payload only (no path ID).
// Use WriteNLRI() for ADD-PATH aware encoding.
type NLRI interface {
	// Family returns the AFI/SAFI for this NLRI.
	// RFC 4760 Section 3: <AFI, SAFI> identifies NLRI semantics.
	Family() Family

	// Bytes returns the wire-format encoding of this NLRI (payload only).
	// RFC 4271 Section 4.3: Encoded as <length, prefix> tuples.
	// The returned slice may be shared; do not modify.
	//
	// Note: Path ID is NOT included. Use WriteNLRI() for ADD-PATH encoding.
	Bytes() []byte

	// Pack returns wire-format bytes adapted for negotiated capabilities.
	//
	// Deprecated: Use WriteNLRI() for zero-allocation encoding.
	// This method allocates a new slice; prefer WriteNLRI() with pre-allocated buffer.
	Pack(ctx *PackContext) []byte

	// Len returns the payload length in bytes (no path ID).
	// Use LenWithContext() for ADD-PATH aware length calculation.
	Len() int

	// String returns a human-readable representation.
	String() string

	// PathID returns the ADD-PATH path identifier (0 if not present).
	// RFC 7911 Section 3: Path Identifier is a 4-octet field.
	PathID() uint32

	// WriteTo writes the NLRI payload (without path ID) into buf at offset.
	// Returns number of bytes written.
	//
	// Note: Path ID is NOT written. Use WriteNLRI() for ADD-PATH encoding.
	// The ctx parameter is kept for interface compatibility but is ignored.
	WriteTo(buf []byte, off int, ctx *PackContext) int
}

// LenWithContext returns the wire-format length adjusted for context.
//
// Phase 3: Len() now returns payload length (no path ID). LenWithContext adds
// 4 bytes when ctx.AddPath=true for types that support ADD-PATH.
//
// RFC 7911: ADD-PATH prepends 4-byte path identifier when negotiated:
//   - If ctx is nil or ctx.AddPath=false: returns Len() (payload only)
//   - If ctx.AddPath=true: returns Len() + 4 (for types supporting ADD-PATH)
//
// Note: Some NLRI types (FlowSpec, BGPLS, etc.) don't support ADD-PATH
// and always return Len() regardless of context.
func LenWithContext(n NLRI, ctx *PackContext) int {
	baseLen := n.Len()

	// Types that don't support ADD-PATH
	if !supportsAddPath(n) {
		return baseLen
	}

	// ADD-PATH: add 4 bytes for path identifier
	if ctx != nil && ctx.AddPath {
		return baseLen + 4
	}

	return baseLen
}

// supportsAddPath returns true if the NLRI type supports ADD-PATH encoding.
// Types that don't support ADD-PATH have Pack() that ignores the context.
func supportsAddPath(n NLRI) bool {
	switch n.(type) {
	case *FlowSpec, *FlowSpecVPN:
		return false
	case *BGPLSNode, *BGPLSLink, *BGPLSPrefix, *BGPLSSRv6SID:
		return false
	case *MVPN, *VPLS, *RTC, *MUP:
		return false
	default:
		return true
	}
}

// PayloadWriter is implemented by NLRI types that support payload-only writing.
// This interface enables zero-allocation ADD-PATH encoding by separating
// path ID handling from payload encoding.
type PayloadWriter interface {
	// BaseLen returns payload length WITHOUT path ID.
	BaseLen() int
	// WritePayloadTo writes payload only (no path ID) into buf at offset.
	WritePayloadTo(buf []byte, off int) int
	// PathID returns the stored path identifier (0 if unset).
	PathID() uint32
}

// WriteNLRI writes NLRI with ADD-PATH handling into buf at offset.
//
// RFC 7911 Section 3: ADD-PATH prepends 4-byte path identifier:
//   - If ctx.AddPath=true: writes path ID + payload
//   - If ctx.AddPath=false or ctx=nil: writes payload only
//
// For NLRI types implementing PayloadWriter, uses zero-allocation path.
// For others, falls back to WriteTo.
func WriteNLRI(n NLRI, buf []byte, off int, ctx *PackContext) int {
	// Try optimized path for types with PayloadWriter
	if pw, ok := n.(PayloadWriter); ok {
		return writeNLRIOptimized(pw, buf, off, ctx)
	}
	// Fallback for types without PayloadWriter
	return n.WriteTo(buf, off, ctx)
}

// writeNLRIOptimized handles ADD-PATH encoding for PayloadWriter types.
//
// Phase 3: Path ID is only written when ctx.AddPath=true.
// When ctx=nil or ctx.AddPath=false, only payload is written.
func writeNLRIOptimized(pw PayloadWriter, buf []byte, off int, ctx *PackContext) int {
	pos := off

	// Handle ADD-PATH path identifier
	// RFC 7911: Path ID only included when ctx.AddPath=true
	if ctx != nil && ctx.AddPath {
		binary.BigEndian.PutUint32(buf[pos:], pw.PathID())
		pos += 4
	}

	// Write payload
	pos += pw.WritePayloadTo(buf, pos)

	return pos - off
}
