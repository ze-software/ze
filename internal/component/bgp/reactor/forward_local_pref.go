// Design: docs/architecture/core-design.md -- egress attribute modification on the forward rails
// RFC: rfc/short/rfc4271.md -- LOCAL_PREF is internal-only (Section 5.1.5)
// Related: peer_forward_facts.go -- the sibling applyFacts* egress decisions
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server forward rail
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// localPrefAllowedTo answers RFC 4271 Section 5.1.5 for one destination: "A BGP
// speaker MUST NOT include this attribute in UPDATE messages it sends to
// external peers, except in the case of BGP Confederations [RFC3065]."
//
// The confederation exception has NO configuration surface in Ze. A session is
// internal when LocalAS == PeerAS (PeerSettings.IsEBGP) and external otherwise,
// and neither PeerSettings nor the YANG tree names a confederation member-AS, so
// the exception is constantly false and the prohibition covers every peer this
// daemon calls external. THIS is the single site the exception grows from if a
// member-AS ever becomes configurable: every egress rail asks here rather than
// re-deriving the answer, because the ones that did re-derive it disagreed --
// the announce rail stripped and the forward rail did not.
func localPrefAllowedTo(isIBGP bool) bool { return isIBGP }

// payloadHasLocalPref reports whether an UPDATE payload carries a LOCAL_PREF
// attribute. A malformed payload answers false: nothing to strip is the same
// answer as no attribute, and the RFC 7606 handling of a bad payload is not this
// function's decision.
//
// It reads the attribute SECTION rather than the payload bytes, so a prefix in
// the NLRI holding the byte 0x05 is not mistaken for the attribute.
// Allocation-free: ParseUpdateSections computes offsets and AttrFind walks the
// attribute headers in place.
func payloadHasLocalPref(payload []byte) bool {
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		return false
	}
	attrs := sections.Attrs(payload)
	if attrs == nil {
		return false
	}
	_, _, _, found := attribute.AttrFind(attrs, attribute.AttrLocalPref)
	return found
}

// modsTouchLocalPref reports whether an egress filter already recorded an
// operation on LOCAL_PREF for this destination. A filter that SETS the attribute
// on a route whose source carried none would otherwise put it on an external
// peer's wire, which the payload check alone cannot see.
func modsTouchLocalPref(mods *filterapi.ModAccumulator) bool {
	for _, op := range mods.Ops() {
		if op.Code == uint8(attribute.AttrLocalPref) {
			return true
		}
	}
	return false
}

// applyFactsLocalPref enforces RFC 4271 Section 5.1.5 on the forward rails: an
// UPDATE relayed to an EXTERNAL peer carries no LOCAL_PREF.
//
// RFC 4271 Section 5.1.5: "A BGP speaker MUST NOT include this attribute in
// UPDATE messages it sends to external peers, except in the case of BGP
// Confederations [RFC3065]."
//
// The announce rail (reactor_api_batch.go, buildAnnounceUpdate), the stored-RIB
// rail (peer_rib_routes.go) and the wire builder (reactor_wire.go) each already
// kept the attribute off an external session. The forward rail did not, so a
// route LEARNED from an internal peer and RELAYED to an external one kept the
// internal preference on the wire, while the same prefix originated locally did
// not.
//
// Called AFTER the egress filter pass on both rails, so this Suppress is the
// LAST operation on code 5 and filterapi.LastSetOrSuppress makes it win over a
// filter's Set (gr/gr_egress.go sets LOCAL_PREF=0 for RFC 9494 Section 4.6, and
// a policy chain can set any value). Winning is the conformant outcome: the
// prohibition is not a policy a filter may override.
//
// baseHasLocalPref is computed ONCE per UPDATE by the caller rather than once
// per destination. Recording the operation unconditionally would be simpler and
// would also force every route to every external peer onto the payload-rebuild
// path, which is the cost the route-server fast path exists to avoid.
func applyFactsLocalPref(f *peerForwardFacts, baseHasLocalPref bool, mods *filterapi.ModAccumulator) {
	if localPrefAllowedTo(!f.isEBGP) {
		return
	}
	if !baseHasLocalPref && !modsTouchLocalPref(mods) {
		return
	}
	mods.Op(uint8(attribute.AttrLocalPref), filterapi.AttrModSuppress, nil)
}
