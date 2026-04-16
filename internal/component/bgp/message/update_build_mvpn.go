// Design: docs/architecture/update-building.md — MVPN UPDATE builders
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_grouped.go — grouped and size-aware UPDATE builders
package message

import (
	"net/netip"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// MVPNParams contains parameters for building an MVPN route UPDATE.
//
// RFC 6514 - BGP Encodings and Procedures for Multicast VPNs.
type MVPNParams struct {
	// RouteType is the MVPN route type (5=Source-AD, 6=Shared-Join, 7=Source-Join).
	RouteType uint8

	// IsIPv6 indicates IPv6 MVPN (AFI=2) vs IPv4 MVPN (AFI=1).
	IsIPv6 bool

	// RD is the Route Distinguisher.
	RD [8]byte

	// SourceAS is the source AS for join routes (types 6, 7).
	SourceAS uint32

	// Source is the source IP or RP address.
	Source netip.Addr

	// Group is the multicast group address.
	Group netip.Addr

	// NextHop is the next-hop address.
	NextHop netip.Addr

	// Origin is the origin type.
	Origin attribute.Origin

	// LocalPreference for iBGP.
	LocalPreference uint32

	// MED.
	MED uint32

	// ExtCommunityBytes for route targets.
	ExtCommunityBytes []byte

	// ORIGINATOR_ID (RFC 4456) - 0 means not set.
	OriginatorID uint32

	// CLUSTER_LIST (RFC 4456) - cluster IDs traversed.
	ClusterList []uint32
}

// BuildMVPN builds an UPDATE message for MVPN routes (SAFI 5).
//
// RFC 6514 - MVPN routes use MP_REACH_NLRI with SAFI=5.
// Multiple routes can be included in a single UPDATE.
//
// LIMITATION: Shared attributes (Origin, LocalPreference, MED, ExtCommunityBytes,
// OriginatorID, ClusterList) are taken from routes[0] only. If routes in the
// slice have differing values for these attributes, only the first route's
// values are used.
func (ub *UpdateBuilder) BuildMVPN(routes []MVPNParams) *Update {
	ub.resetScratch()

	if len(routes) == 0 {
		return &Update{}
	}

	first := routes[0]
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, first.Origin)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	asPath := ub.buildASPath(nil)
	asn4 := ub.ASN4
	asPathBuf := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathBuf, 0, asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBuf,
	})

	// 3. NEXT_HOP (for IPv4 test compatibility)
	if first.NextHop.Is4() {
		attrs = append(attrs, &attribute.NextHop{Addr: first.NextHop})
	}

	// 4. MED
	if first.MED > 0 {
		attrs = append(attrs, attribute.MED(first.MED))
	}

	// 5. LOCAL_PREF
	if ub.IsIBGP {
		lp := first.LocalPreference
		if lp == 0 {
			lp = 100
		}
		attrs = append(attrs, attribute.LocalPref(lp))
	}

	// 9. ORIGINATOR_ID (RFC 4456)
	if first.OriginatorID != 0 {
		origIP := netip.AddrFrom4([4]byte{
			byte(first.OriginatorID >> 24), byte(first.OriginatorID >> 16),
			byte(first.OriginatorID >> 8), byte(first.OriginatorID),
		})
		attrs = append(attrs, attribute.OriginatorID(origIP))
	}

	// 10. CLUSTER_LIST (RFC 4456)
	if len(first.ClusterList) > 0 {
		cl := make(attribute.ClusterList, len(first.ClusterList))
		copy(cl, first.ClusterList)
		attrs = append(attrs, cl)
	}

	// 14. MP_REACH_NLRI for MVPN
	mpReach := ub.buildMPReachMVPN(routes)
	attrs = append(attrs, mpReach)

	// 16. EXTENDED_COMMUNITIES
	if len(first.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  first.ExtCommunityBytes,
		})
	}

	// Sort by type code
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	attrBytes := ub.packAttributesOrderedInto(attrs, nil)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachMVPN constructs MP_REACH_NLRI for MVPN routes.
//
// RFC 6514 Section 4 - MVPN NLRI format.
func (ub *UpdateBuilder) buildMPReachMVPN(routes []MVPNParams) *rawAttribute {
	if len(routes) == 0 {
		return nil
	}

	first := routes[0]

	// Build NLRI data for all routes
	var nlriData []byte
	for i := range routes {
		nlriData = append(nlriData, ub.buildMVPNNLRIBytes(routes[i])...) //nolint:gocritic // existing append pattern for NLRI aggregation
	}

	// AFI/SAFI
	var afi uint16 = 1 // IPv4
	if first.IsIPv6 {
		afi = 2 // IPv6
	}
	const safiMVPN uint8 = 5

	// Next-hop
	nhBytes := first.NextHop.AsSlice()
	nhLen := len(nhBytes)

	// Build MP_REACH_NLRI value
	valueLen := 2 + 1 + 1 + nhLen + 1 + len(nlriData)
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safiMVPN
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved
	copy(value[5+nhLen:], nlriData)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

// buildMVPNNLRIBytes builds a single MVPN NLRI.
//
// RFC 6514 Section 4 - MVPN NLRI format:
// Route Type (1) + Length (1) + Route Type Specific Data.
func (ub *UpdateBuilder) buildMVPNNLRIBytes(route MVPNParams) []byte {
	var data []byte

	switch route.RouteType {
	case 5: // Source Active A-D
		// RD (8) + Source (len + IP) + Group (len + IP)
		data = append(data, route.RD[:]...)
		if route.Source.Is4() {
			data = append(data, 32) // prefix len
			src4 := route.Source.As4()
			data = append(data, src4[:]...)
		} else {
			data = append(data, 128)
			src16 := route.Source.As16()
			data = append(data, src16[:]...)
		}
		if route.Group.Is4() {
			data = append(data, 32)
			grp4 := route.Group.As4()
			data = append(data, grp4[:]...)
		} else {
			data = append(data, 128)
			grp16 := route.Group.As16()
			data = append(data, grp16[:]...)
		}

	case 6, 7: // Shared Tree Join (6) or Source Tree Join (7)
		// RD (8) + Source-AS (4) + Source/RP (len + IP) + Group (len + IP)
		data = append(data, route.RD[:]...)
		data = append(data, byte(route.SourceAS>>24), byte(route.SourceAS>>16),
			byte(route.SourceAS>>8), byte(route.SourceAS))
		if route.Source.Is4() {
			data = append(data, 32)
			src4 := route.Source.As4()
			data = append(data, src4[:]...)
		} else {
			data = append(data, 128)
			src16 := route.Source.As16()
			data = append(data, src16[:]...)
		}
		if route.Group.Is4() {
			data = append(data, 32)
			grp4 := route.Group.As4()
			data = append(data, grp4[:]...)
		} else {
			data = append(data, 128)
			grp16 := route.Group.As16()
			data = append(data, grp16[:]...)
		}
	}

	// MVPN NLRI: Type (1) + Length (1) + Data
	result := ub.alloc(2 + len(data))
	result[0] = route.RouteType
	result[1] = byte(len(data))
	copy(result[2:], data)

	return result
}
