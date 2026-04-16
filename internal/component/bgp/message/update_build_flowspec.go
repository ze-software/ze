// Design: docs/architecture/update-building.md — FlowSpec UPDATE builders
// RFC: rfc/short/rfc8955.md — FlowSpec NLRI encoding
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_grouped.go — grouped and size-aware UPDATE builders
package message

import (
	"net/netip"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// FlowSpecParams contains parameters for building a FlowSpec route UPDATE.
//
// RFC 5575 - Dissemination of Flow Specification Rules.
//
// Design note: CommunityBytes uses []byte (not []uint32 like UnicastParams)
// because FlowSpec routes are config-originated and low-volume. The config
// loader pre-packs communities once at load time, avoiding repacking at
// each BuildFlowSpec() call. For received route forwarding (high-volume),
// the Route.wireBytes cache provides zero-copy - that's separate from this.
type FlowSpecParams struct {
	IsIPv6                bool
	RD                    [8]byte // For FlowSpec VPN (SAFI 134)
	NLRI                  []byte  // Pre-built FlowSpec NLRI
	NextHop               netip.Addr
	CommunityBytes        []byte // Pre-packed by config loader (RFC 1997)
	ExtCommunityBytes     []byte // Pre-packed extended communities (RFC 4360)
	IPv6ExtCommunityBytes []byte // RFC 5701

	// ORIGINATOR_ID (RFC 4456) - 0 means not set.
	OriginatorID uint32

	// CLUSTER_LIST (RFC 4456) - cluster IDs traversed.
	ClusterList []uint32
}

// BuildFlowSpec builds an UPDATE message for a FlowSpec route (SAFI 133/134).
//
// RFC 5575 - FlowSpec uses SAFI 133 (plain) or 134 (VPN).
func (ub *UpdateBuilder) BuildFlowSpec(p FlowSpecParams) *Update {
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

	// 8. COMMUNITY
	if len(p.CommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrCommunity,
			data:  p.CommunityBytes,
		})
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
	mpReach := ub.buildMPReachFlowSpec(p)
	attrs = append(attrs, mpReach)

	// 16. EXTENDED_COMMUNITIES
	if len(p.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  p.ExtCommunityBytes,
		})
	}

	// 25. IPv6 EXTENDED_COMMUNITIES (RFC 5701)
	if len(p.IPv6ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrIPv6ExtCommunity,
			data:  p.IPv6ExtCommunityBytes,
		})
	}

	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	attrBytes := ub.packAttributesOrderedInto(attrs, nil)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachFlowSpec constructs MP_REACH_NLRI for FlowSpec routes.
// RFC 8955 Section 4 defines FlowSpec NLRI format.
// RFC 8955 Section 8 defines FlowSpec VPN NLRI format with Route Distinguisher.
func (ub *UpdateBuilder) buildMPReachFlowSpec(p FlowSpecParams) *rawAttribute {
	if len(p.NLRI) == 0 {
		return &rawAttribute{
			flags: attribute.FlagOptional,
			code:  attribute.AttrMPReachNLRI,
			data:  nil,
		}
	}

	var afi uint16 = 1
	var safi byte = 133 // FlowSpec
	if p.IsIPv6 {
		afi = 2
	}
	isVPN := p.RD != [8]byte{}
	if isVPN {
		safi = 134 // FlowSpec VPN
	}

	nhBytes := p.NextHop.AsSlice()
	nhLen := len(nhBytes)

	// Build NLRI bytes - for VPN, wrap with length prefix and RD per RFC 8955 Section 8
	var nlriBytes []byte
	if isVPN {
		// RFC 8955 Section 8: FlowSpec VPN NLRI = Length + RD (8) + FlowSpec components
		// p.NLRI contains just the component bytes for VPN routes
		payloadLen := 8 + len(p.NLRI) // RD (8) + components
		if payloadLen < 240 {
			// Single byte length
			nlriBytes = ub.alloc(1 + payloadLen)
			nlriBytes[0] = byte(payloadLen)
			copy(nlriBytes[1:9], p.RD[:])
			copy(nlriBytes[9:], p.NLRI)
		} else {
			// Extended length (2 bytes): 0xfnnn format
			nlriBytes = ub.alloc(2 + payloadLen)
			nlriBytes[0] = 0xF0 | byte(payloadLen>>8)
			nlriBytes[1] = byte(payloadLen)
			copy(nlriBytes[2:10], p.RD[:])
			copy(nlriBytes[10:], p.NLRI)
		}
	} else {
		// Non-VPN: p.NLRI already contains length prefix + components
		nlriBytes = p.NLRI
	}

	valueLen := 2 + 1 + 1 + nhLen + 1 + len(nlriBytes)
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved
	copy(value[5+nhLen:], nlriBytes)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}
