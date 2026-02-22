// Design: docs/architecture/update-building.md — grouped and size-aware UPDATE builders
// Related: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_vpn.go — VPN route building
// Related: update_build_labeled.go — labeled unicast route building
// Related: update_build_mvpn.go — MVPN route building
// Related: update_build_vpls.go — VPLS route building
// Related: update_build_flowspec.go — FlowSpec route building
// Related: update_build_evpn.go — EVPN route building
// Related: update_build_mup.go — MUP route building
package message

import (
	"net/netip"
	"slices"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

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
	ub.resetScratch()

	if len(routes) == 0 {
		return nil, nil
	}

	// Build attributes once from first route (shared across all)
	attrBytes := ub.packGroupedAttributes(&routes[0])

	// Calculate overhead and available NLRI space
	// Overhead = Header(19) + WithdrawnLen(2) + AttrLen(2) + Attrs
	overhead := HeaderLen + 4 + len(attrBytes)

	if overhead > maxSize {
		return nil, ErrAttributesTooLarge
	}

	nlriSpace := maxSize - overhead

	var updates []*Update
	var currentNLRI []byte

	for i := range routes {
		r := &routes[i]
		// Calculate NLRI size and pack
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, r.Prefix, r.PathID)
		nlriLen := nlri.LenWithContext(inet, ub.AddPath)
		nlriBytes := ub.alloc(nlriLen)
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
func (ub *UpdateBuilder) packGroupedAttributes(first *UnicastParams) []byte {
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, first.Origin)

	// 2. AS_PATH
	asPath := ub.buildASPath(first.ASPath)
	asn4 := ub.ASN4
	asPathBuf := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathBuf, 0, asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBuf,
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
		slices.Sort(sorted)
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
func (ub *UpdateBuilder) BuildUnicastWithMaxSize(p *UnicastParams, maxSize int) (*Update, error) {
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
	ub.resetScratch()

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
	asPathBuf := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathBuf, 0, asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBuf,
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

	for i := range routes {
		nlriBytes := ub.buildMVPNNLRIBytes(routes[i])
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

		currentBatch = append(currentBatch, routes[i])
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
func (ub *UpdateBuilder) BuildVPNWithMaxSize(p *VPNParams, maxSize int) (*Update, error) {
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
func (ub *UpdateBuilder) BuildLabeledUnicastWithMaxSize(p *LabeledUnicastParams, maxSize int) (*Update, error) {
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
