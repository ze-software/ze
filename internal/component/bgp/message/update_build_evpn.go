// Design: docs/architecture/update-building.md — EVPN UPDATE builders
// RFC: rfc/short/rfc7432.md — EVPN NLRI route types
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_grouped.go — grouped and size-aware UPDATE builders
package message

import (
	"net/netip"
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// EVPNParams contains parameters for building EVPN route UPDATEs.
//
// RFC 7432 - BGP MPLS-Based Ethernet VPN.
// NLRI bytes must be pre-built by the caller (same pattern as FlowSpecParams/MUPParams).
type EVPNParams struct {
	// NLRI is the pre-built EVPN NLRI bytes.
	// Caller constructs these using evpn.NewEVPNType1-5() then Bytes().
	NLRI []byte

	// NextHop is the next-hop address (PE address).
	NextHop netip.Addr

	// Path attributes
	Origin            attribute.Origin
	ASPath            []uint32
	MED               uint32
	LocalPreference   uint32
	Communities       []uint32
	LargeCommunities  [][3]uint32
	ExtCommunityBytes []byte // Pre-packed (RT, etc.)

	// ORIGINATOR_ID (RFC 4456) - 0 means not set.
	OriginatorID uint32

	// CLUSTER_LIST (RFC 4456).
	ClusterList []uint32
}

// BuildEVPN builds an UPDATE message for EVPN routes (AFI=25, SAFI=70).
//
// RFC 7432 - BGP MPLS-Based Ethernet VPN.
func (ub *UpdateBuilder) BuildEVPN(p EVPNParams) *Update {
	ub.resetScratch()

	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathBuf := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathBuf, 0, asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBuf,
	})

	// 3. NEXT_HOP - for IPv4 next-hop compatibility
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

	// 9. ORIGINATOR_ID
	if p.OriginatorID != 0 {
		origIP := netip.AddrFrom4([4]byte{
			byte(p.OriginatorID >> 24), byte(p.OriginatorID >> 16),
			byte(p.OriginatorID >> 8), byte(p.OriginatorID),
		})
		attrs = append(attrs, attribute.OriginatorID(origIP))
	}

	// 10. CLUSTER_LIST
	if len(p.ClusterList) > 0 {
		cl := make(attribute.ClusterList, len(p.ClusterList))
		copy(cl, p.ClusterList)
		attrs = append(attrs, cl)
	}

	// 14. MP_REACH_NLRI for EVPN
	mpReach := ub.buildMPReachEVPN(p)
	attrs = append(attrs, mpReach)

	// 16. EXTENDED_COMMUNITIES
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

	// Order attributes: MP_UNREACH first, regular attrs by code, MP_REACH last.
	// Matches the wire-byte order used by ExaBGP fixture round-trip tests.
	attrBytes := ub.packAttributesOrderedInto(attrs, nil)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachEVPN builds MP_REACH_NLRI for EVPN routes.
// NLRI bytes must be pre-built by the caller in p.NLRI.
func (ub *UpdateBuilder) buildMPReachEVPN(p EVPNParams) *rawAttribute {
	if len(p.NLRI) == 0 {
		return &rawAttribute{
			flags: attribute.FlagOptional,
			code:  attribute.AttrMPReachNLRI,
			data:  nil,
		}
	}

	// Build next-hop bytes
	var nhBytes []byte
	if p.NextHop.Is4() || p.NextHop.Is6() {
		nhBytes = p.NextHop.AsSlice()
	} else {
		nhBytes = []byte{0, 0, 0, 0} // No next-hop: IPv4 0.0.0.0
	}
	nhLen := len(nhBytes)

	// MP_REACH_NLRI format:
	// AFI (2) + SAFI (1) + NH Len (1) + NH + Reserved (1) + NLRI
	value := ub.alloc(2 + 1 + 1 + nhLen + 1 + len(p.NLRI))
	value[0] = 0x00
	value[1] = byte(family.AFIL2VPN) // AFI 25
	value[2] = byte(family.SAFIEVPN) // SAFI 70
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved
	copy(value[5+nhLen:], p.NLRI)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}
