// Design: docs/architecture/update-building.md — MUP UPDATE builders
// RFC: rfc/short/draft-ietf-bess-mup-safi.md — MUP SAFI NLRI
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_grouped.go — grouped and size-aware UPDATE builders
package message

import (
	"net/netip"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// MUPParams contains parameters for building a MUP route UPDATE.
//
// draft-mpmz-bess-mup-safi - Mobile User Plane Integration.
type MUPParams struct {
	RouteType         uint8
	IsIPv6            bool
	NLRI              []byte
	NextHop           netip.Addr
	ExtCommunityBytes []byte
	PrefixSID         []byte

	// ORIGINATOR_ID (RFC 4456) - 0 means not set.
	OriginatorID uint32

	// CLUSTER_LIST (RFC 4456) - cluster IDs traversed.
	ClusterList []uint32
}

// BuildMUP builds an UPDATE message for a MUP route (SAFI 85).
func (ub *UpdateBuilder) BuildMUP(p MUPParams) *Update {
	ub.resetScratch()

	var attrs []attribute.Attribute

	// 1. ORIGIN (IGP)
	attrs = append(attrs, attribute.OriginIGP)

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

	// 3. NEXT_HOP - only for IPv4 MUP with IPv4 next-hop
	// For IPv6 MUP with IPv4 next-hop, use IPv4-mapped IPv6 in MP_REACH instead
	if !p.IsIPv6 && p.NextHop.Is4() {
		attrs = append(attrs, &attribute.NextHop{Addr: p.NextHop})
	}

	// 5. LOCAL_PREF
	if ub.IsIBGP {
		attrs = append(attrs, attribute.LocalPref(100))
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

	// 14. MP_REACH_NLRI
	mpReach := ub.buildMPReachMUP(p)
	attrs = append(attrs, mpReach)

	// 16. EXTENDED_COMMUNITIES
	if len(p.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  p.ExtCommunityBytes,
		})
	}

	// 40. PREFIX_SID
	if len(p.PrefixSID) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrPrefixSID,
			data:  p.PrefixSID,
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

// buildMPReachMUP constructs MP_REACH_NLRI for MUP routes.
func (ub *UpdateBuilder) buildMPReachMUP(p MUPParams) *rawAttribute {
	if len(p.NLRI) == 0 {
		return &rawAttribute{
			flags: attribute.FlagOptional,
			code:  attribute.AttrMPReachNLRI,
			data:  nil,
		}
	}

	var afi uint16 = 1
	if p.IsIPv6 {
		afi = 2
	}
	const safiMUP byte = 85

	// For IPv6 MUP with IPv4 next-hop, use IPv4-mapped IPv6 (::ffff:x.x.x.x)
	var nhBytes []byte
	if p.IsIPv6 && p.NextHop.Is4() {
		// Convert to IPv4-mapped IPv6: ::ffff:x.x.x.x
		mapped := p.NextHop.As16()
		nhBytes = mapped[:]
	} else {
		nhBytes = p.NextHop.AsSlice()
	}
	nhLen := len(nhBytes)

	valueLen := 2 + 1 + 1 + nhLen + 1 + len(p.NLRI)
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safiMUP
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

// BuildMUPWithdraw builds an UPDATE message to withdraw a MUP route (SAFI 85).
// Unlike simple withdrawals, MUP withdrawals include path attributes per the test expectations.
func (ub *UpdateBuilder) BuildMUPWithdraw(p MUPParams) *Update {
	ub.resetScratch()

	var attrs []attribute.Attribute

	// 1. ORIGIN (IGP)
	attrs = append(attrs, attribute.OriginIGP)

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

	// 5. LOCAL_PREF
	if ub.IsIBGP {
		attrs = append(attrs, attribute.LocalPref(100))
	}

	// 15. MP_UNREACH_NLRI
	mpUnreach := ub.buildMPUnreachMUP(p)
	attrs = append(attrs, mpUnreach)

	// 16. EXTENDED_COMMUNITIES
	if len(p.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  p.ExtCommunityBytes,
		})
	}

	// 40. PREFIX_SID
	if len(p.PrefixSID) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrPrefixSID,
			data:  p.PrefixSID,
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

// buildMPUnreachMUP constructs MP_UNREACH_NLRI for MUP route withdrawals.
func (ub *UpdateBuilder) buildMPUnreachMUP(p MUPParams) *rawAttribute {
	var afi uint16 = 1
	if p.IsIPv6 {
		afi = 2
	}
	const safiMUP byte = 85

	// Value: AFI(2) + SAFI(1) + Withdrawn NLRI
	valueLen := 2 + 1 + len(p.NLRI)
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safiMUP
	copy(value[3:], p.NLRI)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPUnreachNLRI,
		data:  value,
	}
}
