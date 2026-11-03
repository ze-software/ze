// Design: docs/architecture/api/process-protocol.md -- NLRI blocks in the filter text protocol
// Related: filter_delta_handlers.go -- the MP_REACH_NLRI / MP_UNREACH_NLRI handlers applying AttrModReplaceNLRI
// Related: forward_build.go -- buildModifiedPayload, which writes the legacy NLRI override
// RFC: rfc/short/rfc4760.md -- MP_REACH_NLRI and MP_UNREACH_NLRI

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// nlriRefusal names why a filter's modify answer cannot be applied to an NLRI
// section. The caller drops the route (fail-closed) and logs both fields, so an
// operator learns which family and which rule the answer broke.
type nlriRefusal struct {
	family string
	reason string
}

// Refusal reasons. Static strings: the refusal is logged, never built.
const (
	refuseNonCIDR   = "the engine rewrites only CIDR families (ipv4/ipv6 unicast, multicast, mpls-label); declare raw=true and return the rewritten UPDATE body, or reject the whole UPDATE"
	refuseNotSubset = "the answer names a prefix the section does not carry, or reorders it; a modify answer may only remove prefixes, keeping the order it was given"
	refuseNoSection = "the answer carries an mp block for a family the UPDATE has no MP_REACH_NLRI or MP_UNREACH_NLRI for; the engine cannot build one"
	refuseDuplicate = "the answer carries two blocks for one section; send each of nlri add, nlri del, mp <family> add and mp <family> del at most once"
	refuseSyntax    = "a block of the answer is neither \"nlri add|del <prefix>...\" nor \"mp <family> add|del <prefix>...\", or the answer carries more blocks than an UPDATE has sections"
	refuseMalformed = "the section the answer rewrites does not parse"
)

// nlriSection names the UPDATE section an NLRI block of the filter text was
// rendered from, and is rewritten into.
type nlriSection uint8

const (
	sectionAnnounce  nlriSection = iota // RFC 4271 Section 4.3 Network Layer Reachability Information: "nlri add"
	sectionWithdrawn                    // RFC 4271 Section 4.3 Withdrawn Routes: "nlri del"
	sectionMPReach                      // RFC 4760 Section 3 MP_REACH_NLRI: "mp <family> add"
	sectionMPUnreach                    // RFC 4760 Section 4 MP_UNREACH_NLRI: "mp <family> del"
	sectionCount
)

// multiprotocol reports whether the section is the NLRI of an MP attribute.
func (s nlriSection) multiprotocol() bool {
	return s == sectionMPReach || s == sectionMPUnreach
}

// nlriSource is one NLRI section of the UPDATE as it sits on the wire: its
// family, its bytes, and whether the sending session negotiated ADD-PATH for
// that family. present is false for an MP section the UPDATE does not carry;
// a legacy section is always present, empty or not.
type nlriSource struct {
	section nlriSection
	fam     family.Family
	raw     []byte
	addPath bool
	present bool
}

// familyName names the section's family for a refusal, empty for an MP
// section whose attribute did not parse far enough to name one.
func (s nlriSource) familyName() string {
	if s.fam == (family.Family{}) {
		return ""
	}
	return s.fam.String()
}

// nlriSectionOf reads one section from the wire. It is the single place the
// library chain's NLRI intents (libraryNLRIOps) find a section, so every
// intent rewrites the same four sections:
// a rewrite that skipped MP_REACH_NLRI leaked the IPv6 prefixes a filter
// dropped (plan/journal/guard-added-to-one-half-of-a-pair.md, 2026-10-17).
// A section that does not parse is the error, never an absent section: a
// caller that read a malformed MP attribute as absent let a strip or a dropped
// block pass with the attribute still in the UPDATE. The source returned with
// an MP error names no family (the zero family.Family), because the attribute
// did not parse far enough to name one. A nil MP attribute with no error is
// the absent section.
func nlriSectionOf(wireUpdate *wireu.WireUpdate, ctx *bgpctx.EncodingContext, section nlriSection) (nlriSource, error) {
	source := nlriSource{section: section}
	switch section {
	case sectionAnnounce:
		source.fam = family.IPv4Unicast
		// RFC 4271 Section 4.3: the legacy Network Layer Reachability Information.
		raw, err := wireUpdate.NLRI()
		if err != nil {
			return source, err
		}
		source.raw, source.present = raw, true
	case sectionWithdrawn:
		source.fam = family.IPv4Unicast
		// RFC 4271 Section 4.3: the legacy Withdrawn Routes field.
		raw, err := wireUpdate.Withdrawn()
		if err != nil {
			return source, err
		}
		source.raw, source.present = raw, true
	case sectionMPReach:
		mp, err := wireUpdate.MPReach()
		if err != nil {
			return source, err
		}
		if mp == nil {
			return source, nil
		}
		source.fam, source.raw, source.present = mp.Family(), mp.NLRIBytes(), true
	case sectionMPUnreach:
		mpu, err := wireUpdate.MPUnreach()
		if err != nil {
			return source, err
		}
		if mpu == nil {
			return source, nil
		}
		source.fam, source.raw, source.present = mpu.Family(), mpu.WithdrawnBytes(), true
	case sectionCount:
		return source, nil
	}
	source.addPath = ctx != nil && ctx.AddPath(source.fam)
	return source, nil
}

// nlriSubsetRefusal refuses a subset of a family whose NLRI is not a CIDR
// prefix: the engine cannot frame such a section entry by entry. name is the
// family as the answer names it, empty for a legacy section.
func nlriSubsetRefusal(source nlriSource, name string) (nlriRefusal, bool) {
	if !source.section.multiprotocol() {
		return nlriRefusal{}, true // the legacy sections are IPv4 unicast, a CIDR family
	}
	if !source.fam.PlainCIDR() {
		return nlriRefusal{family: name, reason: refuseNonCIDR}, false
	}
	return nlriRefusal{}, true
}

// recordNLRIRewrite records the kept NLRI of one section, the subset a filter
// left, where buildModifiedPayload reads it:
//   - legacy NLRI: *override, which replaces the source NLRI (an empty,
//     non-nil override drops them all);
//   - legacy Withdrawn Routes: a withdrawn rewrite on mods;
//   - MP_REACH_NLRI / MP_UNREACH_NLRI: an AttrModReplaceNLRI op on code 14 /
//     15, which keeps AFI, SAFI and the next hop, and drops the attribute when
//     kept is empty.
func recordNLRIRewrite(section nlriSection, kept []byte, override *[]byte, mods *filterapi.ModAccumulator) {
	switch section {
	case sectionAnnounce:
		*override = kept
		if *override == nil {
			*override = []byte{} // the source carried none, and the answer keeps none
		}
	case sectionWithdrawn:
		mods.SetWithdrawnRewrite(kept)
	case sectionMPReach:
		mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModReplaceNLRI, kept)
	case sectionMPUnreach:
		mods.Op(uint8(attribute.AttrMPUnreachNLRI), filterapi.AttrModReplaceNLRI, kept)
	case sectionCount:
	}
}

// dropNLRI returns the NLRI of the source section, in source order, less the
// entries that start at an offset in drops, which MUST be ascending and each
// once (policychain.Sets.NLRIDrops answers them so). Each kept NLRI is copied
// whole, path identifier and label stack included, carved from scratch. ok ==
// false when an offset starts no entry of the section, or the section does not
// parse. The walk is bounded by the section length and allocates nothing.
func dropNLRI(scratch *valueScratch, source nlriSource, drops []uint16) ([]byte, bool) {
	return wireu.DropCIDREntries(scratch.carveBytes(len(source.raw))[:0], source.raw, source.addPath, drops)
}
