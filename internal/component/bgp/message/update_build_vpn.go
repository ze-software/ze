// Design: docs/architecture/update-building.md — VPN UPDATE builders
// RFC: rfc/short/rfc4364.md — BGP/MPLS IP VPN NLRI
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_grouped.go — grouped and size-aware UPDATE builders
package message

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// VPNParams contains parameters for building a VPN route UPDATE.
//
// RFC 4364 - BGP/MPLS IP Virtual Private Networks (VPNs).
// RFC 4659 - BGP-MPLS IP VPN Extension for IPv6 VPN.
type VPNParams struct {
	// Prefix is the destination prefix (customer route).
	Prefix netip.Prefix

	// PathID is the ADD-PATH path identifier (RFC 7911).
	PathID uint32

	// NextHop is the next-hop address (PE address).
	NextHop netip.Addr

	// LinkLocalNextHop is the advertising speaker's IPv6 link-local address, to
	// follow NextHop as a second VPN-IPv6 address (RFC 4659 Section 3.2.1.1).
	// The caller decides inclusion (Peer.linkLocalNextHopFor, reactor/link_scope.go);
	// the zero Addr means the Next Hop field carries the global address alone.
	LinkLocalNextHop netip.Addr

	// Labels is the MPLS label stack (20-bit values per RFC 8277 Section 2).
	// RFC 4364 Section 4.3 - Labels for VPN route.
	// RFC 8277 Section 2 - Multiple labels support.
	Labels []uint32

	// RDBytes is the Route Distinguisher in wire format (8 bytes).
	// RFC 4364 Section 4.2 - RD makes VPN routes unique.
	RDBytes [8]byte

	// Origin is the origin type.
	Origin attribute.Origin

	// ASPath is the configured AS path.
	ASPath []uint32

	// MED is the multi-exit discriminator, emitted only when HasMED is set.
	MED uint32
	// HasMED says the route carries a MED; 0 is a legal metric.
	HasMED bool

	// LocalPreference is the local preference (iBGP only). Without
	// HasLocalPreference the builder sends the default 100.
	LocalPreference uint32
	// HasLocalPreference says the route carries a LOCAL_PREF; 0 is legal.
	HasLocalPreference bool

	// Communities is the list of standard communities.
	Communities []uint32

	// ExtCommunityBytes is the raw extended community bytes.
	// RFC 4360 - Used for route targets in VPN.
	ExtCommunityBytes []byte

	// LargeCommunities is the list of large communities.
	LargeCommunities [][3]uint32

	// AtomicAggregate and Aggregator.
	AtomicAggregate bool
	HasAggregator   bool
	AggregatorASN   uint32
	AggregatorIP    [4]byte

	// ORIGINATOR_ID and CLUSTER_LIST (RFC 4456)
	OriginatorID uint32
	ClusterList  []uint32

	// PrefixSID is the BGP Prefix-SID attribute bytes (RFC 8669, RFC 9252).
	// Used for Segment Routing (SR-MPLS or SRv6) in VPN routes.
	PrefixSID []byte
}

// BuildVPN builds an UPDATE message for a VPN route (SAFI 128).
//
// RFC 4364 Section 4 - VPN routes use MP_REACH_NLRI with SAFI=128.
// NLRI format: Label(3) + RD(8) + Prefix.
func (ub *UpdateBuilder) BuildVPN(p *VPNParams) *Update {
	ub.resetScratch()

	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	attrs = ub.appendASPath(attrs, p.ASPath)

	// No NEXT_HOP: the next hop rides in MP_REACH_NLRI alone.
	// RFC 4760 Section 3: "An UPDATE message that carries no NLRI, other than the
	// one encoded in the MP_REACH_NLRI attribute, SHOULD NOT carry the NEXT_HOP
	// attribute." The API rail agrees through family.Family.LegacyNextHop.

	// 4. MED
	attrs = appendMED(attrs, p.MED, p.HasMED)

	// 5. LOCAL_PREF
	attrs = ub.appendLocalPref(attrs, p.LocalPreference, p.HasLocalPreference)

	// 6. ATOMIC_AGGREGATE
	if p.AtomicAggregate {
		attrs = append(attrs, attribute.AtomicAggregate{})
	}

	// 7. AGGREGATOR
	// RFC 6793: AGGREGATOR encoding depends on ASN4 capability.
	if p.HasAggregator {
		attrs = ub.appendAggregator(attrs, p.AggregatorASN, p.AggregatorIP)
	}

	// 8. COMMUNITIES
	if len(p.Communities) > 0 {
		sorted := make([]uint32, len(p.Communities))
		copy(sorted, p.Communities)
		slices.Sort(sorted)

		comms := make(attribute.Communities, len(sorted))
		for i, c := range sorted {
			comms[i] = attribute.Community(c)
		}
		attrs = append(attrs, comms)
	}

	// 9. ORIGINATOR_ID (RFC 4456)
	if p.OriginatorID != 0 {
		origIP := netip.AddrFrom4([4]byte{
			byte(p.OriginatorID >> 24), byte(p.OriginatorID >> 16),
			byte(p.OriginatorID >> 8), byte(p.OriginatorID),
		})
		attrs = append(attrs, attribute.OriginatorID(origIP))
	}

	// 10. CLUSTER_LIST (RFC 4456)
	if len(p.ClusterList) > 0 {
		cl := make(attribute.ClusterList, len(p.ClusterList))
		copy(cl, p.ClusterList)
		attrs = append(attrs, cl)
	}

	// 14. MP_REACH_NLRI for VPN
	// RFC 4271 Appendix F.3: attributes SHOULD be ordered by type code
	mpReach := ub.buildMPReachVPN(p)
	attrs = append(attrs, mpReach)

	// 16. EXTENDED_COMMUNITIES (route targets)
	if len(p.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  p.ExtCommunityBytes,
		})
	}

	// 32. LARGE_COMMUNITIES
	if len(p.LargeCommunities) > 0 {
		lcs := make(attribute.LargeCommunities, len(p.LargeCommunities))
		for i, lc := range p.LargeCommunities {
			lcs[i] = attribute.LargeCommunity{
				GlobalAdmin: lc[0],
				LocalData1:  lc[1],
				LocalData2:  lc[2],
			}
		}
		attrs = append(attrs, lcs)
	}

	// 40. BGP_PREFIX_SID (RFC 8669, RFC 9252)
	if len(p.PrefixSID) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attrPrefixSID,
			data:  p.PrefixSID,
		})
	}

	// Order attributes as attribute.OrderAttributes does: the MP attribute
	// first (RFC 7606 Section 5.1), then every other attribute by code.
	// Matches the wire-byte order used by ExaBGP fixture round-trip tests.
	attrBytes := ub.packAttributesOrderedInto(attrs, nil)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachVPN constructs MP_REACH_NLRI raw bytes for VPN routes.
//
// RFC 4364 Section 4.3.4 - VPN NLRI format:
// Length(1) + Label(3) + RD(8) + Prefix(variable).
// Next-hop: RD(8, all zeros) + IP address, optionally followed by a second
// RD(8, all zeros) + IPv6 link-local address.
//
// RFC 4659 Section 3.2.1.1: "The value of the Length of the Next Hop Network
// Address field in the MP_REACH_NLRI attribute shall be set to 24 when only a
// global address is present, and to 48 if a link-local address is also
// included in the Next Hop field."
//
//	+0      +8                    +24     +32                   +48
//	| RD=0  | Global IPv6 (16 B)  | RD=0  | Link-Local IPv6 (16 B) |
//
// The global-only form ends at +24. An IPv4 next hop is RD(8) + IPv4(4), 12
// octets, for a VPN-IPv4 route, and RD(8) + ::ffff:IPv4(16), 24 octets, for a
// VPN-IPv6 route (RFC 4659 Section 3.2.1.2).
//
// Returns a rawAttribute because VPN next-hop format (RD+IP) differs from
// standard MPReachNLRI (which only supports plain IP addresses).
func (ub *UpdateBuilder) buildMPReachVPN(p *VPNParams) *rawAttribute {
	var afi uint16
	if p.Prefix.Addr().Is6() {
		afi = 2 // AFI IPv6
	} else {
		afi = 1 // AFI IPv4
	}
	safi := byte(128) // SAFI MPLS VPN

	nhLen := vpnNextHopOctets(p)

	// Build VPN NLRI: Length + Label + RD + Prefix
	vpnNLRI := ub.buildVPNNLRIBytes(p)

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI.
	// Writing directly into value (rather than via an intermediate nhBytes slice from
	// ub.alloc) keeps the RD slots zeroed without relying on scratch-buffer state.
	valueLen := 2 + 1 + 1 + nhLen + 1 + len(vpnNLRI)
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(nhLen)
	// RFC 4659 Section 3.2.1.1: each VPN-IPv6 address of the Next Hop field
	// has its "8-octet RD is set to zero".
	clear(value[4 : 4+nhLen])
	if nhLen == vpnIPv4NextHopOctets {
		copy(value[4+8:], p.NextHop.AsSlice()) // IPv4 address
	} else {
		// RFC 4659 Section 3.2.1.2: an IPv4 next hop of a VPN-IPv6 route is
		// "encoded as an IPv4-mapped IPv6 address"; As16 writes ::ffff:a.b.c.d.
		global := p.NextHop.As16()
		copy(value[4+8:4+24], global[:]) // Global (or IPv4-mapped) address
	}
	if nhLen == vpnNextHopPairOctets {
		ll := p.LinkLocalNextHop.As16()
		copy(value[4+32:4+48], ll[:]) // Link-Local address after the second zero RD
	}
	value[4+nhLen] = 0 // Reserved
	copy(value[4+nhLen+1:], vpnNLRI)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

// vpnNextHopPairOctets is the RFC 4659 Section 3.2.1.1 global-plus-link-local
// Next Hop field: two VPN-IPv6 addresses of 24 octets each.
const vpnNextHopPairOctets = 48

// vpnIPv4NextHopOctets is the VPN-IPv4 Next Hop field of an IPv4 next hop:
// 8-byte zero RD + 4-byte IPv4 (RFC 4364 Section 4.3.4).
const vpnIPv4NextHopOctets = 12

// vpnNextHopOctets returns the Next Hop field length buildMPReachVPN writes.
//
// The link-local second address is written only after an IPv6 next hop and only
// when it is IPv6 link-local unicast: RFC 4659 Section 3.2.1.1 names it "the
// link-local IPv6 address of the advertising BGP speaker", so any other value in
// that slot would break the sentence the 48-octet form exists to satisfy.
func vpnNextHopOctets(p *VPNParams) int {
	if !p.NextHop.Is6() {
		if p.Prefix.Addr().Is6() {
			// RFC 4659 Section 3.2.1.2: zero RD + IPv4-mapped IPv6 address.
			return 24
		}
		return vpnIPv4NextHopOctets
	}
	if !p.LinkLocalNextHop.Is6() {
		return 24 // 8-byte RD + 16-byte IPv6
	}
	if !p.LinkLocalNextHop.IsLinkLocalUnicast() {
		return 24
	}
	return vpnNextHopPairOctets
}

// buildVPNNLRIBytes builds the VPN NLRI bytes.
//
// RFC 4364 Section 4.3.4 - VPN-IPv4 NLRI:
// Length (1 octet) + Labels (3 octets each) + RD (8 octets) + Prefix (variable).
// RFC 8277 Section 2 - Multiple labels support.
// RFC 7911 Section 3 - with ADD-PATH negotiated, a 4-octet Path Identifier
// precedes the Length octet.
func (ub *UpdateBuilder) buildVPNNLRIBytes(p *VPNParams) []byte {
	// Prefix bytes
	prefixBits := p.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	prefixData := p.Prefix.Addr().AsSlice()[:prefixBytes]

	// RFC 8277: Each label contributes 24 bits (3 bytes)
	labelLen := len(p.Labels) * 3
	// Total bits: labels*24 + 64 (RD) + prefix bits
	totalBits := len(p.Labels)*24 + 64 + prefixBits

	// Build: [path-id] + length + labels + RD + prefix
	// Write labels directly via WriteLabelValues (zero-alloc).
	var buf []byte
	// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
	// message, the NLRI encoding MUST be extended by prepending the Path
	// Identifier field, which is of four octets."
	// The negotiated flag alone decides: the field reserves no value, so a Path
	// Identifier of 0 is written like any other.
	if ub.AddPath {
		buf = ub.alloc(4 + 1 + labelLen + 8 + prefixBytes)
		buf[0] = byte(p.PathID >> 24)
		buf[1] = byte(p.PathID >> 16)
		buf[2] = byte(p.PathID >> 8)
		buf[3] = byte(p.PathID)
		buf[4] = byte(totalBits)
		nlri.WriteLabelValues(buf, 5, p.Labels)
		copy(buf[5+labelLen:5+labelLen+8], p.RDBytes[:])
		copy(buf[5+labelLen+8:], prefixData)
	} else {
		buf = ub.alloc(1 + labelLen + 8 + prefixBytes)
		buf[0] = byte(totalBits)
		nlri.WriteLabelValues(buf, 1, p.Labels)
		copy(buf[1+labelLen:1+labelLen+8], p.RDBytes[:])
		copy(buf[1+labelLen+8:], prefixData)
	}

	return buf
}
