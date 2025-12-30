// Package message provides BGP message building and parsing.
//
// This file contains UPDATE message builders for constructing route announcements.
// RFC 4271 Section 4.3 defines the UPDATE message format.
package message

import (
	"net/netip"
	"sort"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
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

	// Ctx contains negotiated capability context (ADD-PATH, ASN4).
	// RFC 7911 - ADD-PATH requires path identifier in NLRI.
	// RFC 6793 - ASN4 determines AS number encoding.
	Ctx *nlri.PackContext
}

// NewUpdateBuilder creates a new UpdateBuilder with the given context.
func NewUpdateBuilder(localAS uint32, isIBGP bool, ctx *nlri.PackContext) *UpdateBuilder {
	return &UpdateBuilder{
		LocalAS: localAS,
		IsIBGP:  isIBGP,
		Ctx:     ctx,
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

	// UseExtendedNextHop enables RFC 8950 extended next-hop encoding.
	// When true and prefix is IPv4 with IPv6 next-hop, uses MP_REACH_NLRI.
	UseExtendedNextHop bool

	// RawAttributeBytes contains pre-packed raw attributes to append.
	// Each entry is a complete attribute (flags+code+length+value).
	// Used for pass-through of custom attributes from config.
	RawAttributeBytes [][]byte
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
	asPath := ub.buildASPath(p.ASPath)
	attrs = append(attrs, asPath)

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
	if p.HasAggregator {
		attrs = append(attrs, &attribute.Aggregator{
			ASN:     p.AggregatorASN,
			Address: netip.AddrFrom4(p.AggregatorIP),
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
		ctx := ub.Ctx
		if ctx == nil {
			ctx = &nlri.PackContext{}
		}
		inlineNLRI = inet.Pack(ctx)
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
	attrBytes := attribute.PackAttributesOrdered(attrs)

	// Append raw attributes (already packed, pass-through from config)
	for _, raw := range p.RawAttributeBytes {
		attrBytes = append(attrBytes, raw...)
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
		// Use configured path, prepend local AS for eBGP
		asns := make([]uint32, 0, len(configuredPath)+1)
		if !ub.IsIBGP {
			asns = append(asns, ub.LocalAS)
		}
		asns = append(asns, configuredPath...)
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
	ctx := ub.Ctx
	if ctx == nil {
		ctx = &nlri.PackContext{}
	}
	nlriBytes := inet.Pack(ctx)

	return &attribute.MPReachNLRI{
		AFI:      afi,
		SAFI:     attribute.SAFIUnicast,
		NextHops: []netip.Addr{p.NextHop},
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

// packAttributesNoSort packs attributes in the order provided (no sorting).
// Used when ExaBGP ordering differs from RFC 4271 Appendix F.3.
func packAttributesNoSort(attrs []attribute.Attribute) []byte {
	var result []byte
	for _, attr := range attrs {
		result = append(result, attribute.PackAttribute(attr)...)
	}
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

	// Label is the MPLS label (20-bit value).
	// RFC 4364 Section 4.3 - Label for VPN route.
	Label uint32

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
	asPath := ub.buildASPath(p.ASPath)
	attrs = append(attrs, asPath)

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
	if p.HasAggregator {
		attrs = append(attrs, &attribute.Aggregator{
			ASN:     p.AggregatorASN,
			Address: netip.AddrFrom4(p.AggregatorIP),
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

	// 16. EXTENDED_COMMUNITIES (route targets)
	// NOTE: ExaBGP places EXT_COM before MP_REACH. This violates RFC 4271
	// Appendix F.3 ordering (should be type-code order: 14 before 16) but
	// we match ExaBGP for compatibility.
	if len(p.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  p.ExtCommunityBytes,
		})
	}

	// 14. MP_REACH_NLRI for VPN (after EXT_COM for ExaBGP compatibility)
	mpReach := ub.buildMPReachVPN(p)
	attrs = append(attrs, mpReach)

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

	// NOTE: We do NOT sort VPN attributes by type code.
	// ExaBGP orders them as: ORIGIN, AS_PATH, NEXT_HOP, LOCAL_PREF, EXT_COM, MP_REACH.
	// This differs from RFC 4271 Appendix F.3 but matches ExaBGP for compatibility.
	attrBytes := packAttributesNoSort(attrs)

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
// Length (1 octet) + Label (3 octets) + RD (8 octets) + Prefix (variable).
func (ub *UpdateBuilder) buildVPNNLRIBytes(p VPNParams) []byte {
	// Label encoding: 20-bit label, 3-bit TC=0, 1-bit BOS=1
	label := p.Label
	labelBytes := []byte{
		byte(label >> 12),
		byte(label >> 4),
		byte(label<<4) | 0x01, // BOS = 1
	}

	// Prefix bytes
	prefixBits := p.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	prefixData := p.Prefix.Addr().AsSlice()[:prefixBytes]

	// Total bits: 24 (label) + 64 (RD) + prefix bits
	totalBits := 24 + 64 + prefixBits

	// Build: [path-id] + length + label + RD + prefix
	ctx := ub.Ctx
	hasAddPath := ctx != nil && ctx.AddPath

	var buf []byte
	if hasAddPath && p.PathID != 0 {
		buf = make([]byte, 4+1+3+8+prefixBytes)
		buf[0] = byte(p.PathID >> 24)
		buf[1] = byte(p.PathID >> 16)
		buf[2] = byte(p.PathID >> 8)
		buf[3] = byte(p.PathID)
		buf[4] = byte(totalBits)
		copy(buf[5:8], labelBytes)
		copy(buf[8:16], p.RDBytes[:])
		copy(buf[16:], prefixData)
	} else {
		buf = make([]byte, 1+3+8+prefixBytes)
		buf[0] = byte(totalBits)
		copy(buf[1:4], labelBytes)
		copy(buf[4:12], p.RDBytes[:])
		copy(buf[12:], prefixData)
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
}

// BuildMVPN builds an UPDATE message for MVPN routes (SAFI 5).
//
// RFC 6514 - MVPN routes use MP_REACH_NLRI with SAFI=5.
// Multiple routes can be included in a single UPDATE.
func (ub *UpdateBuilder) BuildMVPN(routes []MVPNParams) *Update {
	if len(routes) == 0 {
		return &Update{}
	}

	first := routes[0]
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, first.Origin)

	// 2. AS_PATH
	asPath := ub.buildASPath(nil)
	attrs = append(attrs, asPath)

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

	attrBytes := attribute.PackAttributesOrdered(attrs)

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
	asPath := ub.buildASPath(p.ASPath)
	attrs = append(attrs, asPath)

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

	attrBytes := attribute.PackAttributesOrdered(attrs)

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
type FlowSpecParams struct {
	IsIPv6                bool
	RD                    [8]byte // For FlowSpec VPN (SAFI 134)
	NLRI                  []byte  // Pre-built FlowSpec NLRI
	NextHop               netip.Addr
	CommunityBytes        []byte
	ExtCommunityBytes     []byte
	IPv6ExtCommunityBytes []byte // RFC 5701
}

// BuildFlowSpec builds an UPDATE message for a FlowSpec route (SAFI 133/134).
//
// RFC 5575 - FlowSpec uses SAFI 133 (plain) or 134 (VPN).
func (ub *UpdateBuilder) BuildFlowSpec(p FlowSpecParams) *Update {
	var attrs []attribute.Attribute

	// 1. ORIGIN (IGP)
	attrs = append(attrs, attribute.OriginIGP)

	// 2. AS_PATH
	asPath := ub.buildASPath(nil)
	attrs = append(attrs, asPath)

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

	attrBytes := attribute.PackAttributesOrdered(attrs)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachFlowSpec constructs MP_REACH_NLRI for FlowSpec routes.
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

	valueLen := 2 + 1 + 1 + nhLen + 1 + len(p.NLRI)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
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
}

// BuildMUP builds an UPDATE message for a MUP route (SAFI 85).
func (ub *UpdateBuilder) BuildMUP(p MUPParams) *Update {
	var attrs []attribute.Attribute

	// 1. ORIGIN (IGP)
	attrs = append(attrs, attribute.OriginIGP)

	// 2. AS_PATH
	asPath := ub.buildASPath(nil)
	attrs = append(attrs, asPath)

	// 5. LOCAL_PREF
	if ub.IsIBGP {
		attrs = append(attrs, attribute.LocalPref(100))
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

	attrBytes := attribute.PackAttributesOrdered(attrs)

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

	nhBytes := p.NextHop.AsSlice()
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
