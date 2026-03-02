// Design: docs/architecture/update-building.md — labeled unicast UPDATE builders
// RFC: rfc/short/rfc8277.md — labeled unicast NLRI encoding
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
package message

import (
	"net/netip"
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// LabeledUnicastParams contains parameters for building a labeled unicast route UPDATE.
//
// RFC 8277 - Using BGP to Bind MPLS Labels to Address Prefixes (SAFI 4).
type LabeledUnicastParams struct {
	// Prefix is the destination prefix.
	Prefix netip.Prefix

	// PathID is the ADD-PATH path identifier (RFC 7911).
	PathID uint32

	// NextHop is the next-hop address.
	NextHop netip.Addr

	// Labels is the MPLS label stack (20-bit values per RFC 8277 Section 2).
	// RFC 8277 Section 2 - Multiple labels support.
	Labels []uint32

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

	// ExtCommunityBytes is the extended community attribute bytes.
	ExtCommunityBytes []byte

	// LargeCommunities is the list of large communities.
	LargeCommunities [][3]uint32

	// AtomicAggregate indicates ATOMIC_AGGREGATE is set.
	AtomicAggregate bool

	// Aggregator fields.
	HasAggregator bool
	AggregatorASN uint32
	AggregatorIP  [4]byte

	// ORIGINATOR_ID and CLUSTER_LIST (RFC 4456)
	OriginatorID uint32
	ClusterList  []uint32

	// PrefixSID is the BGP Prefix-SID attribute bytes (RFC 8669).
	PrefixSID []byte

	// RawAttributeBytes contains pre-packed raw attributes.
	RawAttributeBytes [][]byte
}

// BuildLabeledUnicast builds an UPDATE message for a labeled unicast route (SAFI 4).
//
// RFC 8277 - NLRI format: Label(3) + Prefix.
func (ub *UpdateBuilder) BuildLabeledUnicast(p *LabeledUnicastParams) *Update {
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

	// 14. MP_REACH_NLRI for labeled unicast
	mpReach := ub.buildMPReachLabeledUnicast(p)
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

	// 40. PREFIX_SID (RFC 8669)
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

	// Append raw attributes (already packed, pass-through from config)
	for _, raw := range p.RawAttributeBytes {
		attrBytes = append(attrBytes, raw...)
	}

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachLabeledUnicast constructs MP_REACH_NLRI raw bytes for labeled unicast routes.
//
// RFC 8277 Section 2 - Labeled Unicast NLRI format:
// Length (bits) + Label (3 octets) + Prefix (variable).
// Next-hop: Plain IP address (4 or 16 bytes).
func (ub *UpdateBuilder) buildMPReachLabeledUnicast(p *LabeledUnicastParams) *rawAttribute {
	var afi uint16
	if p.Prefix.Addr().Is6() {
		afi = 2 // AFI IPv6
	} else {
		afi = 1 // AFI IPv4
	}
	safi := byte(4) // SAFI Labeled Unicast

	// Build next-hop: Plain IP address
	nhBytes := p.NextHop.AsSlice()

	// Build Labeled Unicast NLRI: Length + Label + Prefix
	labeledNLRI := ub.BuildLabeledUnicastNLRIBytes(p)

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(labeledNLRI)
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(len(nhBytes))
	copy(value[4:4+len(nhBytes)], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[5+len(nhBytes):], labeledNLRI)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

// BuildLabeledUnicastNLRIBytes builds the labeled unicast NLRI bytes.
//
// RFC 8277 Section 2 - NLRI format:
// Length (1 octet, bits) + Labels (3 octets each) + Prefix (variable).
// RFC 8277 Section 2 - Multiple labels support.
func (ub *UpdateBuilder) BuildLabeledUnicastNLRIBytes(p *LabeledUnicastParams) []byte {
	// Prefix bytes
	prefixBits := p.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	prefixData := p.Prefix.Addr().AsSlice()[:prefixBytes]

	// RFC 8277: Each label contributes 24 bits (3 bytes)
	labelLen := len(p.Labels) * 3
	// Total bits: labels*24 + prefix bits
	totalBits := len(p.Labels)*24 + prefixBits

	// Build: [path-id] + length + labels + prefix
	// Write labels directly via WriteLabelStack (zero-alloc).
	// RFC 7911: Path Identifier MUST be included when ADD-PATH is negotiated
	var buf []byte
	if ub.AddPath {
		// RFC 7911: Always include 4-byte path ID when ADD-PATH negotiated
		buf = ub.alloc(4 + 1 + labelLen + prefixBytes)
		buf[0] = byte(p.PathID >> 24)
		buf[1] = byte(p.PathID >> 16)
		buf[2] = byte(p.PathID >> 8)
		buf[3] = byte(p.PathID)
		buf[4] = byte(totalBits)
		nlri.WriteLabelStack(buf, 5, p.Labels)
		copy(buf[5+labelLen:], prefixData)
	} else {
		buf = ub.alloc(1 + labelLen + prefixBytes)
		buf[0] = byte(totalBits)
		nlri.WriteLabelStack(buf, 1, p.Labels)
		copy(buf[1+labelLen:], prefixData)
	}

	return buf
}
