// Design: docs/architecture/update-building.md — VPN UPDATE builders
// Related: update_build.go — core UpdateBuilder struct and unicast builders
package message

import (
	"net/netip"
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
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

	// MED is the multi-exit discriminator.
	MED uint32

	// LocalPreference is the local preference (iBGP only).
	LocalPreference uint32

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
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathBuf := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathBuf, 0, asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBuf,
	})

	// 3. NEXT_HOP - RFC 4271 Section 5.1.3
	// For ExaBGP compatibility, include NEXT_HOP even for MP_REACH_NLRI routes.
	// RFC 4760 says NEXT_HOP is optional when using MP_REACH_NLRI, but ExaBGP
	// includes it for VPN routes with IPv4 next-hop.
	if p.NextHop.Is4() {
		attrs = append(attrs, &attribute.NextHop{Addr: p.NextHop})
	}

	// 4. MED
	if p.MED > 0 {
		attrs = append(attrs, attribute.MED(p.MED))
	}

	// 5. LOCAL_PREF
	if ub.IsIBGP {
		lp := p.LocalPreference
		if lp == 0 {
			lp = 100
		}
		attrs = append(attrs, attribute.LocalPref(lp))
	}

	// 6. ATOMIC_AGGREGATE
	if p.AtomicAggregate {
		attrs = append(attrs, attribute.AtomicAggregate{})
	}

	// 7. AGGREGATOR
	// RFC 6793: AGGREGATOR encoding depends on ASN4 capability.
	if p.HasAggregator {
		aggBytes := ub.packAggregator(p.AggregatorASN, p.AggregatorIP)
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrAggregator,
			data:  aggBytes,
		})
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
			code:  attribute.AttrPrefixSID,
			data:  p.PrefixSID,
		})
	}

	// Order attributes: MP_UNREACH first, regular attrs by code, MP_REACH last.
	// This matches ExaBGP output for compatibility testing.
	attrBytes := packAttributesOrdered(attrs)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachVPN constructs MP_REACH_NLRI raw bytes for VPN routes.
//
// RFC 4364 Section 4.3.4 - VPN NLRI format:
// Length(1) + Label(3) + RD(8) + Prefix(variable).
// Next-hop: RD(8, all zeros) + IP address.
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

	// Build next-hop: RD (8 bytes, all zeros) + IP address
	// RFC 4364: Next-hop for VPN is RD + IP
	var nhBytes []byte
	if p.NextHop.Is4() {
		nhBytes = ub.alloc(12) // 8-byte RD + 4-byte IPv4
		copy(nhBytes[8:], p.NextHop.AsSlice())
	} else {
		nhBytes = ub.alloc(24) // 8-byte RD + 16-byte IPv6
		copy(nhBytes[8:], p.NextHop.AsSlice())
	}

	// Build VPN NLRI: Length + Label + RD + Prefix
	vpnNLRI := ub.buildVPNNLRIBytes(p)

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(vpnNLRI)
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(len(nhBytes))
	copy(value[4:], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[4+len(nhBytes)+1:], vpnNLRI)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

// buildVPNNLRIBytes builds the VPN NLRI bytes.
//
// RFC 4364 Section 4.3.4 - VPN-IPv4 NLRI:
// Length (1 octet) + Labels (3 octets each) + RD (8 octets) + Prefix (variable).
// RFC 8277 Section 2 - Multiple labels support.
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
	// Write labels directly via WriteLabelStack (zero-alloc).
	var buf []byte
	if ub.AddPath && p.PathID != 0 {
		buf = ub.alloc(4 + 1 + labelLen + 8 + prefixBytes)
		buf[0] = byte(p.PathID >> 24)
		buf[1] = byte(p.PathID >> 16)
		buf[2] = byte(p.PathID >> 8)
		buf[3] = byte(p.PathID)
		buf[4] = byte(totalBits)
		nlri.WriteLabelStack(buf, 5, p.Labels)
		copy(buf[5+labelLen:5+labelLen+8], p.RDBytes[:])
		copy(buf[5+labelLen+8:], prefixData)
	} else {
		buf = ub.alloc(1 + labelLen + 8 + prefixBytes)
		buf[0] = byte(totalBits)
		nlri.WriteLabelStack(buf, 1, p.Labels)
		copy(buf[1+labelLen:1+labelLen+8], p.RDBytes[:])
		copy(buf[1+labelLen+8:], prefixData)
	}

	return buf
}
