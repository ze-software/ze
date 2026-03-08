// Design: docs/architecture/update-building.md — UPDATE builder common infrastructure
// RFC: rfc/short/rfc4271.md — UPDATE message construction (Section 4.3)
// Overview: update.go — UPDATE message wire representation
// Detail: update_build_vpn.go — VPN route building
// Detail: update_build_labeled.go — labeled unicast route building
// Detail: update_build_mvpn.go — MVPN route building
// Detail: update_build_vpls.go — VPLS route building
// Detail: update_build_flowspec.go — FlowSpec route building
// Detail: update_build_evpn.go — EVPN route building
// Detail: update_build_mup.go — MUP route building
// Detail: update_build_grouped.go — grouped and size-aware builders
// Related: update_split.go — UPDATE splitting and chunking
// Related: common_attrs.go — shared attribute extraction helpers
package message

import (
	"net/netip"
	"slices"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wire"
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

	// scratch is a reusable buffer for intermediate encoding during Build* calls.
	// Allocated once on first use, reused across calls. Sub-slices are handed out
	// via alloc() and remain valid until the next resetScratch() call.
	scratch []byte
	off     int
}

// resetScratch prepares the scratch buffer for a new Build* call.
// Called at the start of each Build* method.
func (ub *UpdateBuilder) resetScratch() {
	if ub.scratch == nil {
		ub.scratch = make([]byte, wire.StandardMaxSize)
	}
	ub.off = 0
}

// alloc returns a sub-slice of length n from the scratch buffer.
// The returned slice is valid until the next resetScratch() call.
// Grows the scratch buffer if needed (rare — only for extended messages).
func (ub *UpdateBuilder) alloc(n int) []byte {
	end := ub.off + n
	if end > len(ub.scratch) {
		newSize := max(len(ub.scratch)*2, end)
		newBuf := make([]byte, newSize)
		copy(newBuf, ub.scratch[:ub.off])
		ub.scratch = newBuf
	}
	s := ub.scratch[ub.off:end:end]
	ub.off = end
	return s
}

// NewUpdateBuilder creates a new UpdateBuilder with the given context.
func NewUpdateBuilder(localAS uint32, isIBGP, asn4, addPath bool) *UpdateBuilder {
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

	// SAFI overrides the default SAFIUnicast when set.
	// Use attribute.SAFIMulticast for multicast routes.
	// Zero value defaults to SAFIUnicast.
	SAFI attribute.SAFI
}

// BuildUnicast builds an UPDATE message for a unicast route.
//
// RFC 4271 Section 4.3 - Constructs UPDATE with path attributes and NLRI.
// IPv4 unicast uses inline NLRI, IPv6 uses MP_REACH_NLRI (RFC 4760).
//
// RFC 4271 Appendix F.3 - Attributes are ordered by type code for
// consistent wire format and interoperability.
func (ub *UpdateBuilder) BuildUnicast(p *UnicastParams) *Update {
	ub.resetScratch()

	// Build attributes in a fixed-size buffer for sorting (max 12 attribute types).
	var attrBuf [12]attribute.Attribute
	attrs := attrBuf[:0]

	// 1. ORIGIN (type 1) - RFC 4271 Section 5.1.1
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH (type 2) - RFC 4271 Section 5.1.2
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	// When ASN4=false, ASNs are 2-byte. When ASN4=true (default), ASNs are 4-byte.
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathBuf := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathBuf, 0, asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBuf,
	})

	// 3. NEXT_HOP (type 3) - RFC 4271 Section 5.1.3
	// Only for IPv4 unicast with IPv4 next-hop (not MP_REACH_NLRI, not extended next-hop)
	// RFC 8950: When extended next-hop is used, next-hop goes in MP_REACH_NLRI
	isUnicast := p.SAFI == 0 || p.SAFI == attribute.SAFIUnicast
	if isUnicast && p.Prefix.Addr().Is4() && p.NextHop.Is4() && !p.UseExtendedNextHop {
		attrs = append(attrs, &attribute.NextHop{Addr: p.NextHop})
	}
	// RFC 8950: For IPv6 unicast with IPv4 next-hop, include NEXT_HOP for compatibility
	if isUnicast && p.Prefix.Addr().Is6() && p.NextHop.Is4() && p.UseExtendedNextHop {
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
		slices.Sort(sorted)

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
	// Non-unicast SAFIs and IPv6 always use MP_REACH_NLRI.
	// IPv4 unicast uses inline NLRI unless extended next-hop is negotiated.
	var inlineNLRI []byte
	switch {
	case !isUnicast, p.Prefix.Addr().Is6():
		// Non-unicast SAFI or IPv6: use MP_REACH_NLRI
		mpReach := ub.buildMPReach(p)
		attrs = append(attrs, mpReach)
	case p.UseExtendedNextHop && p.Prefix.Addr().Is4() && p.NextHop.Is6():
		// RFC 8950: IPv4 unicast with IPv6 next-hop via MP_REACH_NLRI
		// buildMPReach handles this - AFI is set from prefix, not next-hop
		mpReach := ub.buildMPReach(p)
		attrs = append(attrs, mpReach)
	case p.Prefix.Addr().Is4():
		// IPv4 unicast: inline NLRI
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, p.Prefix, p.PathID)
		inlineNLRI = make([]byte, nlri.LenWithContext(inet, ub.AddPath))
		nlri.WriteNLRI(inet, inlineNLRI, 0, ub.AddPath)
	}

	// 16. EXTENDED_COMMUNITIES (type 16) - RFC 4360
	if len(p.ExtCommunityBytes) > 0 {
		// Write as raw attribute bytes (already in wire format)
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

	// Write sorted attributes
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
		buf := ub.alloc(8)
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
	buf := ub.alloc(6)
	buf[0] = byte(encodedASN >> 8) //nolint:gosec // buf is 6 bytes, indices 0-5 are valid
	buf[1] = byte(encodedASN)      //nolint:gosec // buf is 6 bytes, indices 0-5 are valid
	copy(buf[2:6], ip[:])
	return buf
}

// buildMPReach constructs MP_REACH_NLRI for the given params.
//
// RFC 4760 Section 3 - MP_REACH_NLRI format.
// SAFI is determined by p.SAFI (defaults to SAFIUnicast when zero).
func (ub *UpdateBuilder) buildMPReach(p *UnicastParams) *attribute.MPReachNLRI {
	var afi attribute.AFI
	var nlriAFI nlri.AFI

	if p.Prefix.Addr().Is6() {
		afi = attribute.AFIIPv6
		nlriAFI = nlri.AFIIPv6
	} else {
		afi = attribute.AFIIPv4
		nlriAFI = nlri.AFIIPv4
	}

	safi := p.SAFI
	if safi == 0 {
		safi = attribute.SAFIUnicast
	}
	nlriSAFI := nlri.SAFI(safi)

	inet := nlri.NewINET(nlri.Family{AFI: nlriAFI, SAFI: nlriSAFI}, p.Prefix, p.PathID)
	nlriBytes := ub.alloc(nlri.LenWithContext(inet, ub.AddPath))
	nlri.WriteNLRI(inet, nlriBytes, 0, ub.AddPath)

	// RFC 2545 Section 3: IPv6 MP_REACH_NLRI may include link-local as second next-hop.
	nhCount := 1
	if p.LinkLocalNextHop.IsValid() && p.Prefix.Addr().Is6() {
		nhCount = 2
	}
	nextHops := make([]netip.Addr, nhCount)
	nextHops[0] = p.NextHop
	if nhCount == 2 {
		nextHops[1] = p.LinkLocalNextHop
	}

	return &attribute.MPReachNLRI{
		AFI:      afi,
		SAFI:     safi,
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
