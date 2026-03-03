// Design: docs/architecture/update-building.md — VPLS UPDATE builders
// RFC: rfc/short/rfc4761.md — VPLS NLRI encoding
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
package message

import (
	"net/netip"
	"slices"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
)

// VPLSParams contains parameters for building a VPLS route UPDATE.
//
// RFC 4761 - Virtual Private LAN Service (VPLS) Using BGP.
type VPLSParams struct {
	RD                [8]byte
	Endpoint          uint16
	Base              uint32
	Offset            uint16
	Size              uint16
	NextHop           netip.Addr
	Origin            attribute.Origin
	LocalPreference   uint32
	MED               uint32
	ASPath            []uint32
	Communities       []uint32
	ExtCommunityBytes []byte
	OriginatorID      uint32
	ClusterList       []uint32
}

// BuildVPLS builds an UPDATE message for a VPLS route (AFI=25, SAFI=65).
//
// RFC 4761 - VPLS uses L2VPN AFI (25) and VPLS SAFI (65).
func (ub *UpdateBuilder) BuildVPLS(p VPLSParams) *Update {
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

	// 14. MP_REACH_NLRI for VPLS
	mpReach := ub.buildMPReachVPLS(p)
	attrs = append(attrs, mpReach)

	// 16. EXTENDED_COMMUNITIES
	if len(p.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  p.ExtCommunityBytes,
		})
	}

	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	attrBytes := make([]byte, attribute.AttributesSize(attrs))
	attribute.WriteAttributesOrdered(attrs, attrBytes, 0)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachVPLS constructs MP_REACH_NLRI for VPLS routes.
//
// RFC 4761 Section 3.2.2 - VPLS NLRI format:
// Length (2) + RD (8) + VE-ID (2) + VE Block Offset (2) + VE Block Size (2) + Label Base (3).
func (ub *UpdateBuilder) buildMPReachVPLS(p VPLSParams) *rawAttribute {
	nlriLen := 2 + 8 + 2 + 2 + 2 + 3
	nlriData := ub.alloc(nlriLen)
	nlriData[0] = 0
	nlriData[1] = 17 // Length in octets (8+2+2+2+3=17)
	copy(nlriData[2:10], p.RD[:])
	nlriData[10] = byte(p.Endpoint >> 8)
	nlriData[11] = byte(p.Endpoint)
	// RFC 4761: VE Block Offset (2 octets)
	nlriData[12] = byte(p.Offset >> 8)
	nlriData[13] = byte(p.Offset)
	// RFC 4761: VE Block Size (2 octets)
	nlriData[14] = byte(p.Size >> 8)
	nlriData[15] = byte(p.Size)
	// Label base: 20-bit label + 4-bit (TC=0, BOS=1)
	nlriData[16] = byte(p.Base >> 12)
	nlriData[17] = byte(p.Base >> 4)
	nlriData[18] = byte(p.Base<<4) | 0x01

	// AFI=25 (L2VPN), SAFI=65 (VPLS)
	nhBytes := p.NextHop.AsSlice()
	nhLen := len(nhBytes)

	valueLen := 2 + 1 + 1 + nhLen + 1 + len(nlriData)
	value := ub.alloc(valueLen)
	value[0] = 0
	value[1] = 25 // AFI L2VPN
	value[2] = 65 // SAFI VPLS
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
