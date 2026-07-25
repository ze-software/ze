// Design: docs/architecture/update-building.md — grouped and size-aware UPDATE builders
// RFC: rfc/short/rfc4271.md — UPDATE message size constraints
// RFC: rfc/short/rfc8654.md — extended message size limits
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_vpn.go — VPN route building
// Related: update_build_labeled.go — labeled unicast route building
// Related: update_build_flowspec.go — FlowSpec route building
// Related: update_build_evpn.go — EVPN route building
package message

import (
	"net/netip"
	"slices"
	"sort"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// BuildGroupedUnicast builds one or more IPv4-unicast UPDATEs respecting maxSize
// and delivers each one to emit synchronously. All routes MUST share identical
// path attributes (caller's responsibility).
//
// The shared PathAttributes is packed ONCE at scratch[0:A) and reused across every
// emitted Update. Each chunk's NLRI is packed at scratch[A:) and ub.off is reset
// to A after emit returns, so the next chunk's NLRI overwrites the previous one
// but attrBytes is preserved. See the Update type doc and
// docs/architecture/update-building.md "Scratch Contract" for the full invariant.
//
// Each emitted Update MUST be consumed (WriteTo, copy out, or hand to SendUpdate
// which copies internally) before emit returns. If emit returns a non-nil error,
// no further Updates are built and the error is returned.
//
// RFC 4271 Section 4.3: Multiple UPDATEs may advertise same attributes.
// RFC 8654: Respects maxSize (4096 or 65535 based on Extended Message).
func (ub *UpdateBuilder) BuildGroupedUnicast(routes []UnicastParams, maxSize int, emit func(*Update) error) error {
	ub.resetScratch()

	if len(routes) == 0 {
		return nil
	}

	// Build attributes once from first route (shared across all emitted chunks).
	attrBytes := ub.packGroupedAttributes(&routes[0])

	// Calculate overhead and available NLRI space.
	// Overhead = Header(19) + WithdrawnLen(2) + AttrLen(2) + Attrs
	overhead := HeaderLen + 4 + len(attrBytes)
	if overhead > maxSize {
		return ErrAttributesTooLarge
	}
	nlriSpace := maxSize - overhead

	// nlriStart = end of attrBytes = start of the per-chunk NLRI region.
	// ub.off resets to this between emitted chunks so attrBytes stays valid.
	nlriStart := ub.off

	for i := range routes {
		r := &routes[i]
		inet := nlri.NewINET(family.IPv4Unicast, r.Prefix, r.PathID)
		nlriLen := nlri.LenWithContext(inet, ub.AddPath)

		// Single NLRI too large to fit any chunk.
		if nlriLen > nlriSpace {
			return ErrNLRITooLarge
		}

		// Would adding this NLRI overflow the current chunk? Flush first.
		currentLen := ub.off - nlriStart
		if currentLen+nlriLen > nlriSpace && currentLen > 0 {
			if err := emit(&Update{
				PathAttributes: attrBytes,
				NLRI:           ub.scratch[nlriStart:ub.off],
			}); err != nil {
				return err
			}
			ub.off = nlriStart
		}

		nlriBytes := ub.alloc(nlriLen)
		nlri.WriteNLRI(inet, nlriBytes, 0, ub.AddPath)
	}

	// Flush remainder.
	if ub.off > nlriStart {
		if err := emit(&Update{
			PathAttributes: attrBytes,
			NLRI:           ub.scratch[nlriStart:ub.off],
		}); err != nil {
			return err
		}
	}

	return nil
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

	return ub.packAttributesOrderedInto(attrs, first.RawAttributeBytes)
}

// =============================================================================
// Size-Aware Builders (spec-api-bounds-safety.md)
// =============================================================================

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
