// Package message provides BGP message building and parsing.
//
// This file contains UPDATE message builders for constructing route announcements.
// RFC 4271 Section 4.3 defines the UPDATE message format.
package message

import (
	"net/netip"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
)

// UpdateBuilder provides context for building UPDATE messages.
//
// RFC 4271 Section 4.3 - UPDATE messages are used to transfer routing
// information between BGP peers. The builder encapsulates peer-specific
// context needed for correct encoding.
type UpdateBuilder struct {
	// LocalAS is the local autonomous system number.
	// RFC 4271 Section 4.3b - Used for AS_PATH construction.
	LocalAS uint32

	// IsIBGP indicates whether this is an iBGP session.
	// RFC 4271 Section 5.1.5 - LOCAL_PREF is only for iBGP.
	// RFC 4271 Section 5.1.2 - AS_PATH handling differs for iBGP/eBGP.
	IsIBGP bool

	// ASN4 indicates 4-byte AS number capability is negotiated.
	// RFC 6793 - ASN4 determines AS number encoding.
	ASN4 bool

	// AddPath indicates ADD-PATH is negotiated for this family.
	// RFC 7911 - ADD-PATH requires path identifier in NLRI.
	AddPath bool
}

// NewUpdateBuilder creates a new UpdateBuilder with the given context.
func NewUpdateBuilder(localAS uint32, isIBGP bool, asn4, addPath bool) *UpdateBuilder {
	return &UpdateBuilder{
		LocalAS: localAS,
		IsIBGP:  isIBGP,
		ASN4:    asn4,
		AddPath: addPath,
	}
}

// UnicastParams contains parameters for building a unicast route UPDATE.
//
// All fields use primitive types to avoid reactor dependencies.
// This enables the builder to be used in message/ package independently.
type UnicastParams struct {
	// Prefix is the destination prefix.
	Prefix netip.Prefix

	// PathID is the ADD-PATH path identifier (RFC 7911).
	// Only used when ADD-PATH is negotiated.
	PathID uint32

	// NextHop is the next-hop address.
	// RFC 4271 Section 5.1.3 - NEXT_HOP attribute.
	NextHop netip.Addr

	// Origin is the origin type (IGP, EGP, INCOMPLETE).
	// RFC 4271 Section 5.1.1 - ORIGIN attribute.
	Origin attribute.Origin

	// ASPath is the configured AS path (optional).
	// RFC 4271 Section 5.1.2 - AS_PATH attribute.
	// For eBGP, local AS is prepended. For iBGP, used as-is.
	ASPath []uint32

	// MED is the multi-exit discriminator (optional, 0 = not set).
	// RFC 4271 Section 5.1.4 - MULTI_EXIT_DISC attribute.
	MED uint32

	// LocalPreference is the local preference (iBGP only, 0 = default 100).
	// RFC 4271 Section 5.1.5 - LOCAL_PREF attribute.
	LocalPreference uint32

	// Communities is the list of standard communities (RFC 1997).
	Communities []uint32

	// ExtCommunityBytes is the raw extended community bytes (RFC 4360).
	ExtCommunityBytes []byte

	// LargeCommunities is the list of large communities (RFC 8092).
	// Each entry is [3]uint32{GlobalAdmin, LocalData1, LocalData2}.
	LargeCommunities [][3]uint32

	// AtomicAggregate indicates ATOMIC_AGGREGATE is set (RFC 4271 Section 5.1.6).
	AtomicAggregate bool

	// HasAggregator indicates AGGREGATOR is set (RFC 4271 Section 5.1.7).
	HasAggregator bool
	AggregatorASN uint32
	AggregatorIP  [4]byte

	// LinkLocalNextHop is the IPv6 link-local next-hop address (RFC 2545 Section 3).
	// When set for IPv6 routes, MP_REACH_NLRI includes 32-byte next-hop (global + link-local).
	LinkLocalNextHop netip.Addr

	// UseExtendedNextHop enables RFC 8950 extended next-hop encoding.
	// When true and prefix is IPv4 with IPv6 next-hop, uses MP_REACH_NLRI.
	UseExtendedNextHop bool

	// RawAttributeBytes contains pre-packed raw attributes to append.
	// Each entry is a complete attribute (flags+code+length+value).
	// Used for pass-through of custom attributes from config.
	RawAttributeBytes [][]byte

	// ORIGINATOR_ID (RFC 4456) - 0 means not set.
	// Used for route reflector configurations.
	OriginatorID uint32

	// CLUSTER_LIST (RFC 4456) - cluster IDs traversed.
	// Used for route reflector configurations.
	ClusterList []uint32
}

// BuildUnicast builds an UPDATE message for a unicast route.
//
// RFC 4271 Section 4.3 - Constructs UPDATE with path attributes and NLRI.
// IPv4 unicast uses inline NLRI, IPv6 uses MP_REACH_NLRI (RFC 4760).
//
// RFC 4271 Appendix F.3 - Attributes are ordered by type code for
// consistent wire format and interoperability.
func (ub *UpdateBuilder) BuildUnicast(p UnicastParams) *Update {
	// Build attributes in a slice for sorting
	var attrs []attribute.Attribute

	// 1. ORIGIN (type 1) - RFC 4271 Section 5.1.1
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH (type 2) - RFC 4271 Section 5.1.2
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	// When ASN4=false, ASNs are 2-byte. When ASN4=true (default), ASNs are 4-byte.
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
	})

	// 3. NEXT_HOP (type 3) - RFC 4271 Section 5.1.3
	// Only for IPv4 unicast with IPv4 next-hop (not MP_REACH_NLRI, not extended next-hop)
	// RFC 8950: When extended next-hop is used, next-hop goes in MP_REACH_NLRI
	if p.Prefix.Addr().Is4() && p.NextHop.Is4() && !p.UseExtendedNextHop {
		attrs = append(attrs, &attribute.NextHop{Addr: p.NextHop})
	}
	// RFC 8950: For IPv6 unicast with IPv4 next-hop, include NEXT_HOP for compatibility
	if p.Prefix.Addr().Is6() && p.NextHop.Is4() && p.UseExtendedNextHop {
		attrs = append(attrs, &attribute.NextHop{Addr: p.NextHop})
	}

	// 4. MED (type 4) - RFC 4271 Section 5.1.4
	if p.MED > 0 {
		attrs = append(attrs, attribute.MED(p.MED))
	}

	// 5. LOCAL_PREF (type 5) - RFC 4271 Section 5.1.5
	// Only for iBGP sessions
	if ub.IsIBGP {
		lp := p.LocalPreference
		if lp == 0 {
			lp = 100 // Default LOCAL_PREF
		}
		attrs = append(attrs, attribute.LocalPref(lp))
	}

	// 6. ATOMIC_AGGREGATE (type 6) - RFC 4271 Section 5.1.6
	if p.AtomicAggregate {
		attrs = append(attrs, attribute.AtomicAggregate{})
	}

	// 7. AGGREGATOR (type 7) - RFC 4271 Section 5.1.7
	// RFC 6793: AGGREGATOR encoding depends on ASN4 capability.
	if p.HasAggregator {
		aggBytes := ub.packAggregator(p.AggregatorASN, p.AggregatorIP)
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrAggregator,
			data:  aggBytes,
		})
	}

	// 8. COMMUNITIES (type 8) - RFC 1997
	if len(p.Communities) > 0 {
		sorted := make([]uint32, len(p.Communities))
		copy(sorted, p.Communities)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		comms := make(attribute.Communities, len(sorted))
		for i, c := range sorted {
			comms[i] = attribute.Community(c)
		}
		attrs = append(attrs, comms)
	}

	// 9. ORIGINATOR_ID (type 9) - RFC 4456
	if p.OriginatorID != 0 {
		origIP := netip.AddrFrom4([4]byte{
			byte(p.OriginatorID >> 24), byte(p.OriginatorID >> 16),
			byte(p.OriginatorID >> 8), byte(p.OriginatorID),
		})
		attrs = append(attrs, attribute.OriginatorID(origIP))
	}

	// 10. CLUSTER_LIST (type 10) - RFC 4456
	if len(p.ClusterList) > 0 {
		cl := make(attribute.ClusterList, len(p.ClusterList))
		copy(cl, p.ClusterList)
		attrs = append(attrs, cl)
	}

	// 14. MP_REACH_NLRI (type 14) - RFC 4760
	// For IPv6 unicast and RFC 8950 extended next-hop, next-hop and NLRI go here
	var inlineNLRI []byte
	switch {
	case p.Prefix.Addr().Is6():
		// IPv6 unicast: use MP_REACH_NLRI
		mpReach := ub.buildMPReachUnicast(p)
		attrs = append(attrs, mpReach)
	case p.UseExtendedNextHop && p.Prefix.Addr().Is4() && p.NextHop.Is6():
		// RFC 8950: IPv4 unicast with IPv6 next-hop via MP_REACH_NLRI
		// buildMPReachUnicast handles this - AFI is set from prefix, not next-hop
		mpReach := ub.buildMPReachUnicast(p)
		attrs = append(attrs, mpReach)
	case p.Prefix.Addr().Is4():
		// IPv4 unicast: inline NLRI
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, p.Prefix, p.PathID)
		inlineNLRI = make([]byte, nlri.LenWithContext(inet, ub.AddPath))
		nlri.WriteNLRI(inet, inlineNLRI, 0, ub.AddPath)
	}

	// 16. EXTENDED_COMMUNITIES (type 16) - RFC 4360
	if len(p.ExtCommunityBytes) > 0 {
		// Pack as raw attribute bytes (already in wire format)
		// We need a wrapper that implements Attribute interface
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  p.ExtCommunityBytes,
		})
	}

	// 32. LARGE_COMMUNITIES (type 32) - RFC 8092
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

	// Sort attributes by type code per RFC 4271 Appendix F.3
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	// Pack sorted attributes
	attrSize := attribute.AttributesSize(attrs)
	// Calculate raw attributes size
	rawSize := 0
	for _, raw := range p.RawAttributeBytes {
		rawSize += len(raw)
	}
	attrBytes := make([]byte, attrSize+rawSize)
	off := attribute.WriteAttributesOrdered(attrs, attrBytes, 0)

	// Append raw attributes (already packed, pass-through from config)
	for _, raw := range p.RawAttributeBytes {
		off += copy(attrBytes[off:], raw)
	}

	return &Update{
		PathAttributes: attrBytes,
		NLRI:           inlineNLRI,
	}
}

// buildASPath constructs the AS_PATH attribute.
//
// RFC 4271 Section 5.1.2 - AS_PATH handling:
// - iBGP: Use configured path as-is (empty if none)
// - eBGP: Prepend local AS to configured path.
func (ub *UpdateBuilder) buildASPath(configuredPath []uint32) *attribute.ASPath {
	var segments []attribute.ASPathSegment

	switch {
	case len(configuredPath) > 0:
		// Use configured path, prepend local AS for eBGP only if not already first.
		// RFC 4271 Section 5.1.2: "prepend its own AS number as the last element"
		// (last in the sequence = first when reading left-to-right).
		asns := configuredPath
		if !ub.IsIBGP && configuredPath[0] != ub.LocalAS {
			asns = make([]uint32, 0, len(configuredPath)+1)
			asns = append(asns, ub.LocalAS)
			asns = append(asns, configuredPath...)
		}
		segments = []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: asns},
		}
	case ub.IsIBGP:
		// Empty AS_PATH for iBGP self-originated routes
		segments = nil
	default:
		// Prepend local AS for eBGP self-originated routes
		segments = []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{ub.LocalAS}},
		}
	}

	return &attribute.ASPath{Segments: segments}
}

// packAggregator encodes AGGREGATOR with context-dependent ASN size.
//
// RFC 6793 Section 4.2.3 - AGGREGATOR format:
//   - ASN4=true: 8 bytes (4-byte ASN + 4-byte IP).
//   - ASN4=false: 6 bytes (2-byte ASN + 4-byte IP).
//   - ASN4=false with ASN>65535: Uses AS_TRANS (23456).
func (ub *UpdateBuilder) packAggregator(asn uint32, ip [4]byte) []byte {
	asn4 := ub.ASN4

	if asn4 {
		// 8-byte format: 4-byte ASN + 4-byte IP
		buf := make([]byte, 8)
		buf[0] = byte(asn >> 24)
		buf[1] = byte(asn >> 16)
		buf[2] = byte(asn >> 8)
		buf[3] = byte(asn)
		copy(buf[4:8], ip[:])
		return buf
	}

	// 6-byte format: 2-byte ASN + 4-byte IP
	// RFC 6793: Large ASNs use AS_TRANS (23456)
	encodedASN := asn
	if asn > 65535 {
		encodedASN = 23456 // AS_TRANS
	}
	buf := make([]byte, 6)
	buf[0] = byte(encodedASN >> 8) //nolint:gosec // buf is 6 bytes, indices 0-5 are valid
	buf[1] = byte(encodedASN)      //nolint:gosec // buf is 6 bytes, indices 0-5 are valid
	copy(buf[2:6], ip[:])
	return buf
}

// buildMPReachUnicast constructs MP_REACH_NLRI for unicast routes.
//
// RFC 4760 Section 3 - MP_REACH_NLRI format for IPv6 unicast.
func (ub *UpdateBuilder) buildMPReachUnicast(p UnicastParams) *attribute.MPReachNLRI {
	var afi attribute.AFI
	var nlriAFI nlri.AFI

	if p.Prefix.Addr().Is6() {
		afi = attribute.AFIIPv6
		nlriAFI = nlri.AFIIPv6
	} else {
		afi = attribute.AFIIPv4
		nlriAFI = nlri.AFIIPv4
	}

	// Build NLRI
	inet := nlri.NewINET(nlri.Family{AFI: nlriAFI, SAFI: nlri.SAFIUnicast}, p.Prefix, p.PathID)
	nlriBytes := make([]byte, nlri.LenWithContext(inet, ub.AddPath))
	nlri.WriteNLRI(inet, nlriBytes, 0, ub.AddPath)

	// RFC 2545 Section 3: IPv6 MP_REACH_NLRI may include link-local as second next-hop.
	nextHops := []netip.Addr{p.NextHop}
	if p.LinkLocalNextHop.IsValid() && p.Prefix.Addr().Is6() {
		nextHops = append(nextHops, p.LinkLocalNextHop)
	}

	return &attribute.MPReachNLRI{
		AFI:      afi,
		SAFI:     attribute.SAFIUnicast,
		NextHops: nextHops,
		NLRI:     nlriBytes,
	}
}

// rawAttribute is a simple wrapper for pre-encoded attribute data.
type rawAttribute struct {
	flags attribute.AttributeFlags
	code  attribute.AttributeCode
	data  []byte
}

func (r *rawAttribute) Code() attribute.AttributeCode   { return r.code }
func (r *rawAttribute) Flags() attribute.AttributeFlags { return r.flags }
func (r *rawAttribute) Len() int                        { return len(r.data) }
func (r *rawAttribute) Pack() []byte                    { return r.data }

// PackWithContext returns Pack() - raw attributes are pre-encoded.
func (r *rawAttribute) PackWithContext(_, _ *bgpctx.EncodingContext) []byte { return r.data }

// WriteTo writes the pre-encoded data into buf at offset.
func (r *rawAttribute) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], r.data)
}

// WriteToWithContext writes pre-encoded data - context-independent.
func (r *rawAttribute) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return r.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (r *rawAttribute) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := r.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return r.WriteTo(buf, off), nil
}

// packAttributesOrdered packs attributes with MP_UNREACH first, regular attrs
// by type code, then MP_REACH last. This matches ExaBGP output.
//
// Uses WriteAttributesOrdered (zero-alloc write into pre-sized buffer).
func packAttributesOrdered(attrs []attribute.Attribute) []byte {
	if len(attrs) == 0 {
		return nil
	}
	totalSize := attribute.AttributesSize(attrs)
	result := make([]byte, totalSize)
	attribute.WriteAttributesOrdered(attrs, result, 0)
	return result
}

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
func (ub *UpdateBuilder) BuildVPN(p VPNParams) *Update {
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
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
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

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
func (ub *UpdateBuilder) buildMPReachVPN(p VPNParams) *rawAttribute {
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
		nhBytes = make([]byte, 12) // 8-byte RD + 4-byte IPv4
		copy(nhBytes[8:], p.NextHop.AsSlice())
	} else {
		nhBytes = make([]byte, 24) // 8-byte RD + 16-byte IPv6
		copy(nhBytes[8:], p.NextHop.AsSlice())
	}

	// Build VPN NLRI: Length + Label + RD + Prefix
	vpnNLRI := ub.buildVPNNLRIBytes(p)

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(vpnNLRI)
	value := make([]byte, valueLen)
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
func (ub *UpdateBuilder) buildVPNNLRIBytes(p VPNParams) []byte {
	// RFC 8277 Section 2: Encode label stack with BOS bit on last label
	labelBytes := nlri.EncodeLabelStack(p.Labels)

	// Prefix bytes
	prefixBits := p.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	prefixData := p.Prefix.Addr().AsSlice()[:prefixBytes]

	// Total bits: labels*24 + 64 (RD) + prefix bits
	// RFC 8277: Each label contributes 24 bits
	totalBits := len(p.Labels)*24 + 64 + prefixBits

	// Build: [path-id] + length + labels + RD + prefix
	var buf []byte
	if ub.AddPath && p.PathID != 0 {
		buf = make([]byte, 4+1+len(labelBytes)+8+prefixBytes)
		buf[0] = byte(p.PathID >> 24)
		buf[1] = byte(p.PathID >> 16)
		buf[2] = byte(p.PathID >> 8)
		buf[3] = byte(p.PathID)
		buf[4] = byte(totalBits)
		copy(buf[5:5+len(labelBytes)], labelBytes)
		copy(buf[5+len(labelBytes):5+len(labelBytes)+8], p.RDBytes[:])
		copy(buf[5+len(labelBytes)+8:], prefixData)
	} else {
		buf = make([]byte, 1+len(labelBytes)+8+prefixBytes)
		buf[0] = byte(totalBits)
		copy(buf[1:1+len(labelBytes)], labelBytes)
		copy(buf[1+len(labelBytes):1+len(labelBytes)+8], p.RDBytes[:])
		copy(buf[1+len(labelBytes)+8:], prefixData)
	}

	return buf
}

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
func (ub *UpdateBuilder) BuildLabeledUnicast(p LabeledUnicastParams) *Update {
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
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
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

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
func (ub *UpdateBuilder) buildMPReachLabeledUnicast(p LabeledUnicastParams) *rawAttribute {
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
	labeledNLRI := ub.buildLabeledUnicastNLRIBytes(p)

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(labeledNLRI)
	value := make([]byte, valueLen)
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

// buildLabeledUnicastNLRIBytes builds the labeled unicast NLRI bytes.
//
// RFC 8277 Section 2 - NLRI format:
// Length (1 octet, bits) + Labels (3 octets each) + Prefix (variable).
// RFC 8277 Section 2 - Multiple labels support.
func (ub *UpdateBuilder) buildLabeledUnicastNLRIBytes(p LabeledUnicastParams) []byte {
	// RFC 8277 Section 2: Encode label stack with BOS bit on last label
	labelBytes := nlri.EncodeLabelStack(p.Labels)

	// Prefix bytes
	prefixBits := p.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	prefixData := p.Prefix.Addr().AsSlice()[:prefixBytes]

	// Total bits: labels*24 + prefix bits
	// RFC 8277: Each label contributes 24 bits
	totalBits := len(p.Labels)*24 + prefixBits

	// Build: [path-id] + length + labels + prefix
	// RFC 7911: Path Identifier MUST be included when ADD-PATH is negotiated
	var buf []byte
	if ub.AddPath {
		// RFC 7911: Always include 4-byte path ID when ADD-PATH negotiated
		buf = make([]byte, 4+1+len(labelBytes)+prefixBytes)
		buf[0] = byte(p.PathID >> 24)
		buf[1] = byte(p.PathID >> 16)
		buf[2] = byte(p.PathID >> 8)
		buf[3] = byte(p.PathID)
		buf[4] = byte(totalBits)
		copy(buf[5:5+len(labelBytes)], labelBytes)
		copy(buf[5+len(labelBytes):], prefixData)
	} else {
		buf = make([]byte, 1+len(labelBytes)+prefixBytes)
		buf[0] = byte(totalBits)
		copy(buf[1:1+len(labelBytes)], labelBytes)
		copy(buf[1+len(labelBytes):], prefixData)
	}

	return buf
}

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
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
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

	attrBytes := make([]byte, attribute.AttributesSize(attrs))
	attribute.WriteAttributesOrdered(attrs, attrBytes, 0)

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
	for _, route := range routes {
		nlriData = append(nlriData, ub.buildMVPNNLRIBytes(route)...)
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
	value := make([]byte, valueLen)
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
	result := make([]byte, 2+len(data))
	result[0] = route.RouteType
	result[1] = byte(len(data))
	copy(result[2:], data)

	return result
}

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
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
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
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

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
	nlri := make([]byte, nlriLen)
	nlri[0] = 0
	nlri[1] = 17 // Length in octets (8+2+2+2+3=17)
	copy(nlri[2:10], p.RD[:])
	nlri[10] = byte(p.Endpoint >> 8)
	nlri[11] = byte(p.Endpoint)
	// RFC 4761: VE Block Offset (2 octets)
	nlri[12] = byte(p.Offset >> 8)
	nlri[13] = byte(p.Offset)
	// RFC 4761: VE Block Size (2 octets)
	nlri[14] = byte(p.Size >> 8)
	nlri[15] = byte(p.Size)
	// Label base: 20-bit label + 4-bit (TC=0, BOS=1)
	nlri[16] = byte(p.Base >> 12)
	nlri[17] = byte(p.Base >> 4)
	nlri[18] = byte(p.Base<<4) | 0x01

	// AFI=25 (L2VPN), SAFI=65 (VPLS)
	nhBytes := p.NextHop.AsSlice()
	nhLen := len(nhBytes)

	valueLen := 2 + 1 + 1 + nhLen + 1 + len(nlri)
	value := make([]byte, valueLen)
	value[0] = 0
	value[1] = 25 // AFI L2VPN
	value[2] = 65 // SAFI VPLS
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved
	copy(value[5+nhLen:], nlri)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

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
	var attrs []attribute.Attribute

	// 1. ORIGIN (IGP)
	attrs = append(attrs, attribute.OriginIGP)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	asPath := ub.buildASPath(nil)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
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

	attrBytes := make([]byte, attribute.AttributesSize(attrs))
	attribute.WriteAttributesOrdered(attrs, attrBytes, 0)

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
			nlriBytes = make([]byte, 1+payloadLen)
			nlriBytes[0] = byte(payloadLen)
			copy(nlriBytes[1:9], p.RD[:])
			copy(nlriBytes[9:], p.NLRI)
		} else {
			// Extended length (2 bytes): 0xfnnn format
			nlriBytes = make([]byte, 2+payloadLen)
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
	value := make([]byte, valueLen)
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

// EVPNParams contains parameters for building EVPN route UPDATEs.
//
// RFC 7432 - BGP MPLS-Based Ethernet VPN.
// Supports all 5 EVPN route types.
type EVPNParams struct {
	// RouteType is the EVPN route type (1-5).
	// RFC 7432 Section 7 defines route types.
	RouteType uint8

	// RD is the Route Distinguisher.
	RD nlri.RouteDistinguisher

	// NextHop is the next-hop address (PE address).
	NextHop netip.Addr

	// PathID is the ADD-PATH path identifier (RFC 7911).
	PathID uint32

	// ESI is the Ethernet Segment Identifier (Types 1, 2, 4, 5).
	// RFC 7432 Section 5 - 10-byte ESI.
	ESI [10]byte

	// EthernetTag is the Ethernet Tag ID (Types 1, 2, 3, 5).
	// RFC 7432 Section 7 - 4-byte ethernet tag.
	EthernetTag uint32

	// MAC is the MAC address (Type 2 only).
	// RFC 7432 Section 7.2 - 6-byte MAC.
	MAC [6]byte

	// IP is the IP address (Type 2 optional).
	// RFC 7432 Section 7.2 - IPv4 or IPv6.
	IP netip.Addr

	// OriginatorIP is the originating router's IP (Types 3, 4).
	// RFC 7432 Sections 7.3, 7.4.
	OriginatorIP netip.Addr

	// Prefix is the IP prefix (Type 5 only).
	// RFC 9136 Section 3.1.
	Prefix netip.Prefix

	// Gateway is the gateway IP (Type 5 only).
	// RFC 9136 Section 3.1 - 0.0.0.0 or :: if not set.
	Gateway netip.Addr

	// Labels is the MPLS label stack (Types 1, 2, 5).
	Labels []uint32

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
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
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
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

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
	// This matches ExaBGP output for compatibility testing.
	attrBytes := packAttributesOrdered(attrs)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachEVPN builds MP_REACH_NLRI for EVPN routes.
func (ub *UpdateBuilder) buildMPReachEVPN(p EVPNParams) *rawAttribute {
	// Build EVPN NLRI based on route type
	var evpnNLRI evpn.EVPN
	switch p.RouteType {
	case 1:
		evpnNLRI = evpn.NewEVPNType1(p.RD, p.ESI, p.EthernetTag, p.Labels)
	case 2:
		evpnNLRI = evpn.NewEVPNType2(p.RD, p.ESI, p.EthernetTag, p.MAC, p.IP, p.Labels)
	case 3:
		evpnNLRI = evpn.NewEVPNType3(p.RD, p.EthernetTag, p.OriginatorIP)
	case 4:
		evpnNLRI = evpn.NewEVPNType4(p.RD, p.ESI, p.OriginatorIP)
	case 5:
		evpnNLRI = evpn.NewEVPNType5(p.RD, p.ESI, p.EthernetTag, p.Prefix, p.Gateway, p.Labels)
	default:
		// Unknown type - return empty
		return &rawAttribute{
			flags: attribute.FlagOptional,
			code:  attribute.AttrMPReachNLRI,
			data:  nil,
		}
	}

	// Calculate NLRI size
	nlriLen := nlri.LenWithContext(evpnNLRI, ub.AddPath)

	// Build next-hop bytes
	var nhBytes []byte
	switch {
	case p.NextHop.Is4(), p.NextHop.Is6():
		nhBytes = p.NextHop.AsSlice()
	default:
		nhBytes = []byte{0, 0, 0, 0} // Default IPv4 0.0.0.0
	}
	nhLen := len(nhBytes)

	// MP_REACH_NLRI format:
	// AFI (2) + SAFI (1) + NH Len (1) + NH + Reserved (1) + NLRI
	value := make([]byte, 2+1+1+nhLen+1+nlriLen)
	value[0] = 0x00
	value[1] = byte(nlri.AFIL2VPN) // AFI 25
	value[2] = byte(nlri.SAFIEVPN) // SAFI 70
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved
	nlri.WriteNLRI(evpnNLRI, value, 5+nhLen, ub.AddPath)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

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
	var attrs []attribute.Attribute

	// 1. ORIGIN (IGP)
	attrs = append(attrs, attribute.OriginIGP)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	asPath := ub.buildASPath(nil)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
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
	value := make([]byte, valueLen)
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
	var attrs []attribute.Attribute

	// 1. ORIGIN (IGP)
	attrs = append(attrs, attribute.OriginIGP)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	asPath := ub.buildASPath(nil)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
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
	value := make([]byte, valueLen)
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

// BuildGroupedUnicastWithLimit builds multiple UPDATEs if needed to respect size limit.
//
// All routes MUST have identical attributes (caller's responsibility).
// Returns error if attributes exceed maxSize.
//
// Design: Build-and-tally approach - pack incrementally, flush when full.
// This avoids wasteful pre-calculation of sizes.
//
// RFC 4271 Section 4.3: Multiple UPDATEs may advertise same attributes.
// RFC 8654: Respects maxSize (4096 or 65535 based on Extended Message).
func (ub *UpdateBuilder) BuildGroupedUnicastWithLimit(routes []UnicastParams, maxSize int) ([]*Update, error) {
	if len(routes) == 0 {
		return nil, nil
	}

	// Build attributes once from first route (shared across all)
	attrBytes := ub.packGroupedAttributes(routes[0])

	// Calculate overhead and available NLRI space
	// Overhead = Header(19) + WithdrawnLen(2) + AttrLen(2) + Attrs
	overhead := HeaderLen + 4 + len(attrBytes)

	if overhead > maxSize {
		return nil, ErrAttributesTooLarge
	}

	nlriSpace := maxSize - overhead

	var updates []*Update
	var currentNLRI []byte

	for _, r := range routes {
		// Calculate NLRI size and pack
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, r.Prefix, r.PathID)
		nlriLen := nlri.LenWithContext(inet, ub.AddPath)
		nlriBytes := make([]byte, nlriLen)
		nlri.WriteNLRI(inet, nlriBytes, 0, ub.AddPath)

		// Check if single NLRI is too large
		if len(nlriBytes) > nlriSpace {
			return nil, ErrNLRITooLarge
		}

		// Would overflow? Flush current batch
		if len(currentNLRI)+len(nlriBytes) > nlriSpace && len(currentNLRI) > 0 {
			updates = append(updates, &Update{
				PathAttributes: attrBytes,
				NLRI:           currentNLRI,
			})
			currentNLRI = nil
		}

		currentNLRI = append(currentNLRI, nlriBytes...)
	}

	// Flush remainder
	if len(currentNLRI) > 0 {
		updates = append(updates, &Update{
			PathAttributes: attrBytes,
			NLRI:           currentNLRI,
		})
	}

	return updates, nil
}

// packGroupedAttributes packs attributes for grouped unicast routes.
// Uses first route's attributes; called once per batch.
func (ub *UpdateBuilder) packGroupedAttributes(first UnicastParams) []byte {
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, first.Origin)

	// 2. AS_PATH
	asPath := ub.buildASPath(first.ASPath)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
	})

	// 3. NEXT_HOP (IPv4 only)
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

	// 6. ATOMIC_AGGREGATE
	if first.AtomicAggregate {
		attrs = append(attrs, attribute.AtomicAggregate{})
	}

	// 7. AGGREGATOR
	if first.HasAggregator {
		aggBytes := ub.packAggregator(first.AggregatorASN, first.AggregatorIP)
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrAggregator,
			data:  aggBytes,
		})
	}

	// 8. COMMUNITIES
	if len(first.Communities) > 0 {
		sorted := make([]uint32, len(first.Communities))
		copy(sorted, first.Communities)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		comms := make(attribute.Communities, len(sorted))
		for i, c := range sorted {
			comms[i] = attribute.Community(c)
		}
		attrs = append(attrs, comms)
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

	// 16. EXTENDED_COMMUNITIES
	if len(first.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  first.ExtCommunityBytes,
		})
	}

	// 32. LARGE_COMMUNITIES
	if len(first.LargeCommunities) > 0 {
		lcs := make(attribute.LargeCommunities, len(first.LargeCommunities))
		for i, lc := range first.LargeCommunities {
			lcs[i] = attribute.LargeCommunity{
				GlobalAdmin: lc[0],
				LocalData1:  lc[1],
				LocalData2:  lc[2],
			}
		}
		attrs = append(attrs, lcs)
	}

	// Sort and pack attributes
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	// Calculate total size including raw attributes
	attrSize := attribute.AttributesSize(attrs)
	rawSize := 0
	for _, raw := range first.RawAttributeBytes {
		rawSize += len(raw)
	}
	attrBytes := make([]byte, attrSize+rawSize)
	off := attribute.WriteAttributesOrdered(attrs, attrBytes, 0)

	// Append raw attributes from first route (pass-through from config)
	for _, raw := range first.RawAttributeBytes {
		off += copy(attrBytes[off:], raw)
	}

	return attrBytes
}

// =============================================================================
// Size-Aware Builders (spec-api-bounds-safety.md)
// =============================================================================

// BuildFlowSpecWithMaxSize builds a FlowSpec UPDATE with size validation.
//
// RFC 5575 Section 4: Single FlowSpec rule is atomic - cannot be split.
// Returns ErrUpdateTooLarge if rule + attributes exceeds maxSize.
//
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
// Caller must provide maxSize based on negotiated capabilities.
func (ub *UpdateBuilder) BuildFlowSpecWithMaxSize(p FlowSpecParams, maxSize int) (*Update, error) {
	update := ub.BuildFlowSpec(p)

	// Calculate total UPDATE size: Header(19) + WithdrawnLen(2) + AttrLen(2) + Attrs
	updateSize := HeaderLen + 4 + len(update.PathAttributes)

	// RFC 5575 Section 4: FlowSpec NLRI max 4095 bytes.
	// Single FlowSpec rule is atomic - cannot be split across UPDATEs.
	// If rule + attributes > maxSize, MUST return error.
	if updateSize > maxSize {
		return nil, ErrUpdateTooLarge
	}

	return update, nil
}

// BuildUnicastWithMaxSize builds a unicast UPDATE with size validation.
//
// Returns ErrUpdateTooLarge if route + attributes exceeds maxSize.
// Single route is atomic - cannot be split.
//
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
func (ub *UpdateBuilder) BuildUnicastWithMaxSize(p UnicastParams, maxSize int) (*Update, error) {
	update := ub.BuildUnicast(p)

	// Calculate total UPDATE size: Header(19) + WithdrawnLen(2) + AttrLen(2) + Attrs + NLRI
	updateSize := HeaderLen + 4 + len(update.PathAttributes) + len(update.NLRI)

	if updateSize > maxSize {
		return nil, ErrUpdateTooLarge
	}

	return update, nil
}

// BuildMVPNWithLimit builds multiple UPDATEs if needed to respect size limit.
//
// MVPN routes share attributes, so they can be batched.
// Returns []*Update split across multiple messages if needed.
//
// Design: Build-and-tally approach - pack incrementally, flush when full.
//
// RFC 6514 - MVPN uses MP_REACH_NLRI with SAFI=5.
// RFC 4271 Section 4.3: Multiple UPDATEs may advertise same attributes.
// RFC 8654: Respects maxSize (4096 or 65535 based on Extended Message).
func (ub *UpdateBuilder) BuildMVPNWithLimit(routes []MVPNParams, maxSize int) ([]*Update, error) {
	if len(routes) == 0 {
		return nil, nil
	}

	// For MVPN, attributes are from first route
	// We need to calculate overhead to know how much space we have for NLRI

	// Calculate overhead: Header(19) + WithdrawnLen(2) + AttrLen(2) + base attrs
	// The MP_REACH_NLRI attribute contains AFI(2) + SAFI(1) + NH_Len(1) + NH + Reserved(1) + NLRI
	// We need to find non-NLRI attribute size

	// Build attributes without NLRI to measure base size
	first := routes[0]
	var attrs []attribute.Attribute

	// Same attribute building as BuildMVPN, but without MP_REACH
	attrs = append(attrs, first.Origin)

	asPath := ub.buildASPath(nil)
	asn4 := ub.ASN4
	asPathValue := asPath.PackWithASN4(asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathValue,
	})

	if first.NextHop.Is4() {
		attrs = append(attrs, &attribute.NextHop{Addr: first.NextHop})
	}

	if first.MED > 0 {
		attrs = append(attrs, attribute.MED(first.MED))
	}

	if ub.IsIBGP {
		lp := first.LocalPreference
		if lp == 0 {
			lp = 100
		}
		attrs = append(attrs, attribute.LocalPref(lp))
	}

	if first.OriginatorID != 0 {
		origIP := netip.AddrFrom4([4]byte{
			byte(first.OriginatorID >> 24), byte(first.OriginatorID >> 16),
			byte(first.OriginatorID >> 8), byte(first.OriginatorID),
		})
		attrs = append(attrs, attribute.OriginatorID(origIP))
	}

	if len(first.ClusterList) > 0 {
		cl := make(attribute.ClusterList, len(first.ClusterList))
		copy(cl, first.ClusterList)
		attrs = append(attrs, cl)
	}

	if len(first.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  first.ExtCommunityBytes,
		})
	}

	// Sort for consistent sizing
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	baseAttrSize := attribute.AttributesSize(attrs)

	// MP_REACH_NLRI header: flags(1) + code(1) + len(1-2) + AFI(2) + SAFI(1) + NH_Len(1) + NH + Reserved(1)
	nhLen := len(first.NextHop.AsSlice())
	mpReachOverhead := 3 + 2 + 1 + 1 + nhLen + 1 // ~13 bytes for IPv4 NH

	// Total overhead: Header + lengths + base attrs + MP_REACH header
	overhead := HeaderLen + 4 + baseAttrSize + mpReachOverhead

	if overhead > maxSize {
		return nil, ErrAttributesTooLarge
	}

	nlriSpace := maxSize - overhead

	var updates []*Update
	var currentBatch []MVPNParams
	currentSize := 0 // Track incrementally for O(n) instead of O(n²)

	for _, route := range routes {
		nlriBytes := ub.buildMVPNNLRIBytes(route)
		nlriLen := len(nlriBytes)

		// Check if single NLRI fits
		// RFC 6514: MVPN NLRI is typically small (<100 bytes), but validate anyway
		if nlriLen > nlriSpace {
			// Single NLRI too large - cannot fit in any UPDATE
			// Return error rather than creating oversized message
			return nil, ErrNLRITooLarge
		}

		// Would adding this route overflow?
		if currentSize+nlriLen > nlriSpace && len(currentBatch) > 0 {
			// Flush current batch
			updates = append(updates, ub.BuildMVPN(currentBatch))
			currentBatch = nil
			currentSize = 0
		}

		currentBatch = append(currentBatch, route)
		currentSize += nlriLen
	}

	// Flush remainder
	if len(currentBatch) > 0 {
		updates = append(updates, ub.BuildMVPN(currentBatch))
	}

	// Verify size limits (defense-in-depth for overhead calculation bugs)
	for _, u := range updates {
		size := HeaderLen + 4 + len(u.PathAttributes)
		if size > maxSize {
			// This indicates a bug in overhead calculation - return error, not partial results
			return nil, ErrUpdateTooLarge
		}
	}

	return updates, nil
}

// BuildVPNWithMaxSize builds a VPN UPDATE with size validation.
//
// Returns ErrUpdateTooLarge if route + attributes exceeds maxSize.
// Single VPN route is atomic - cannot be split.
//
// RFC 4364 - BGP/MPLS IP Virtual Private Networks.
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
func (ub *UpdateBuilder) BuildVPNWithMaxSize(p VPNParams, maxSize int) (*Update, error) {
	update := ub.BuildVPN(p)

	// VPN routes use MP_REACH_NLRI, no inline NLRI
	updateSize := HeaderLen + 4 + len(update.PathAttributes)

	if updateSize > maxSize {
		return nil, ErrUpdateTooLarge
	}

	return update, nil
}

// BuildLabeledUnicastWithMaxSize builds a labeled unicast UPDATE with size validation.
//
// Returns ErrUpdateTooLarge if route + attributes exceeds maxSize.
// Single labeled unicast route is atomic - cannot be split.
//
// RFC 8277 - Using BGP to Bind MPLS Labels to Address Prefixes.
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
func (ub *UpdateBuilder) BuildLabeledUnicastWithMaxSize(p LabeledUnicastParams, maxSize int) (*Update, error) {
	update := ub.BuildLabeledUnicast(p)

	// Labeled unicast uses MP_REACH_NLRI, no inline NLRI
	updateSize := HeaderLen + 4 + len(update.PathAttributes)

	if updateSize > maxSize {
		return nil, ErrUpdateTooLarge
	}

	return update, nil
}

// BuildVPLSWithMaxSize builds a VPLS UPDATE with size validation.
//
// Returns ErrUpdateTooLarge if route + attributes exceeds maxSize.
// Single VPLS route is atomic - cannot be split.
//
// RFC 4761 - Virtual Private LAN Service (VPLS) Using BGP.
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
func (ub *UpdateBuilder) BuildVPLSWithMaxSize(p VPLSParams, maxSize int) (*Update, error) {
	update := ub.BuildVPLS(p)

	// VPLS uses MP_REACH_NLRI, no inline NLRI
	updateSize := HeaderLen + 4 + len(update.PathAttributes)

	if updateSize > maxSize {
		return nil, ErrUpdateTooLarge
	}

	return update, nil
}

// BuildEVPNWithMaxSize builds an EVPN UPDATE with size validation.
//
// Returns ErrUpdateTooLarge if route + attributes exceeds maxSize.
// Single EVPN route is atomic - cannot be split.
//
// RFC 7432 - BGP MPLS-Based Ethernet VPN.
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
func (ub *UpdateBuilder) BuildEVPNWithMaxSize(p EVPNParams, maxSize int) (*Update, error) {
	update := ub.BuildEVPN(p)

	// EVPN uses MP_REACH_NLRI, no inline NLRI
	updateSize := HeaderLen + 4 + len(update.PathAttributes)

	if updateSize > maxSize {
		return nil, ErrUpdateTooLarge
	}

	return update, nil
}

// BuildMUPWithMaxSize builds a MUP UPDATE with size validation.
//
// Returns ErrUpdateTooLarge if route + attributes exceeds maxSize.
// Single MUP route is atomic - cannot be split.
//
// draft-mpmz-bess-mup-safi - Mobile User Plane Integration.
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
func (ub *UpdateBuilder) BuildMUPWithMaxSize(p MUPParams, maxSize int) (*Update, error) {
	update := ub.BuildMUP(p)

	// MUP uses MP_REACH_NLRI, no inline NLRI
	updateSize := HeaderLen + 4 + len(update.PathAttributes)

	if updateSize > maxSize {
		return nil, ErrUpdateTooLarge
	}

	return update, nil
}
