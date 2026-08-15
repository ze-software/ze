// Design: docs/architecture/bgp/egress-attribute-rules.md -- egress attribute modification on the forward rails
// RFC: rfc/short/rfc4271.md -- a received MULTI_EXIT_DISC is not relayed to another neighboring AS (Section 5.1.4)
// RFC: rfc/short/rfc7947.md -- a route server preserves MULTI_EXIT_DISC across the fabric (Section 2.2.3)
// Related: forward_local_pref.go -- the Section 5.1.5 sibling, which shares this mechanism and reverses its precedence
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server forward rail
package reactor

import (
	"bytes"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// medValue names the MULTI_EXIT_DISC one UPDATE payload carries: the attribute
// VALUE bytes where they already sit, and whether the attribute is there at all.
//
// The bytes are compared rather than decoded, so a value of any length answers
// the one question this file asks -- is the value about to be written the value
// that arrived? A four-octet decode would have to invent an answer for a length
// RFC 4271 Section 5.1.4 does not allow, and inventing one is how a zero becomes
// indistinguishable from an absence.
type medValue struct {
	raw     []byte // aliases the payload; never retained past the destination it was read for
	present bool
}

// sameAs reports whether two payloads carry the same MULTI_EXIT_DISC.
func (m medValue) sameAs(other medValue) bool {
	return m.present == other.present && bytes.Equal(m.raw, other.raw)
}

// payloadMED reports the MULTI_EXIT_DISC an UPDATE payload carries. A malformed
// payload answers absent: nothing to suppress is the same answer as no
// attribute, and the RFC 7606 handling of a bad payload is not this function's
// decision (validateMEDAttr, message/rfc7606.go, owns the length).
//
// It reads the attribute SECTION rather than the payload bytes, so a prefix in
// the NLRI holding the byte 0x04 is not mistaken for the attribute.
// Allocation-free: ParseUpdateSections computes offsets and AttrFind walks the
// attribute headers in place.
func payloadMED(payload []byte) medValue {
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		return medValue{}
	}
	attrs := sections.Attrs(payload)
	if attrs == nil {
		return medValue{}
	}
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMED)
	if !found {
		return medValue{}
	}
	return medValue{raw: value, present: true}
}

// medPropagationAllowedTo answers, for ONE destination, whether a MULTI_EXIT_DISC
// received from a neighboring AS may be carried on to it.
//
// RFC 4271 Section 5.1.4: "If received over EBGP, the MULTI_EXIT_DISC attribute
// MAY be propagated over IBGP to other BGP speakers within the same AS (see also
// 9.1.2.2). The MULTI_EXIT_DISC attribute received from a neighboring AS MUST
// NOT be propagated to other neighboring ASes."
//
// An internal destination is inside the same AS, so the received metric is owed
// to it. An external destination is another neighboring AS and must not see it.
//
// A ROUTE-SERVER CLIENT IS THE EXEMPTION, and it is gated on the exact condition
// the exempting RFC names. RFC 7947 Section 2.2.3: "Contrary to Section 5.1.4 of
// [RFC4271], if applied to an NLRI UPDATE sent to a route server, this attribute
// SHOULD be propagated to other route server clients, and the route server
// SHOULD NOT modify its value." An IXP fabric exists to let clients discriminate
// among each other's entry points, so a route server that stripped the metric
// would break the service it provides. RFC7947-x-3 carries that sentence in Ze's
// ledger, at the SHOULD level RFC 7947 gives it, so it gates nothing. The predicate
// below meets it anyway: the automatic Section 5.1.4 strip never fires toward a route
// server client, and TestReactorForwardRSTransparent is its proof. An operator's own
// med-remove policy still removes the metric, upstream of this predicate, which is the
// RFC 4271 Section 5.1.4 mechanism rather than an exception to this one.
//
// The source AS is NOT an input. RFC 4271 permits the metric toward the AS it
// came from ("other neighboring ASes"), and MULTI_EXIT_DISC is optional, so not
// sending it there is conformant too. Threading a source AS to egress to widen
// this answer would buy a permission nobody asked for, and every other
// implementation strips on the same condition (FRR: from != bgp->peer_self).
func medPropagationAllowedTo(isEBGP, rsClient bool) bool { return !isEBGP || rsClient }

// modsSetMED reports whether an egress filter already recorded a Set on
// MULTI_EXIT_DISC for this destination. That Set is the operator ORIGINATING a
// metric toward this peer, which Section 5.1.4 permits, so it must not be
// undone: the suppression below is skipped and filterapi.LastSetOrSuppress
// leaves the filter's value as the last word.
//
// THIS IS WHERE MED PARTS COMPANY WITH ITS SIBLING. modsTouchLocalPref
// (forward_local_pref.go) makes a filter's operation FORCE the strip, because
// Section 5.1.5 prohibits the attribute outright and a policy may not grant what
// the RFC refuses. Section 5.1.4 prohibits only relaying somebody else's value,
// and a metric Ze sets toward a peer is what the attribute is for.
func modsSetMED(mods *filterapi.ModAccumulator) bool {
	for _, op := range mods.Ops() {
		if op.Code == uint8(attribute.AttrMED) && op.Action == filterapi.AttrModSet {
			return true
		}
	}
	return false
}

// applyFactsMED enforces RFC 4271 Section 5.1.4 on the forward rails: a
// MULTI_EXIT_DISC that ARRIVED from a neighboring AS is not relayed to another
// one.
//
// RFC 4271 Section 5.1.4: "The MULTI_EXIT_DISC attribute received from a
// neighboring AS MUST NOT be propagated to other neighboring ASes."
//
// PROVENANCE IS THE TEST, NOT THE DESTINATION ALONE. Both forward rails relay an
// UPDATE another speaker sent, so every byte of src came off that speaker's
// wire and a MED in it is by definition received. A metric Ze originates reaches
// a peer by two other routes, and both survive here:
//
//   - the announce rails, which never call this function. writeAnnounceUpdate
//     (reactor_wire.go) and buildRIBRouteUpdate (peer_rib_routes.go) encode a
//     RouteSpec and a rib.Route that Ze itself produced.
//   - an egress filter, either as an operation on this destination's accumulator
//     (modsSetMED) or as a policy chain's wire override, which arrives here as a
//     base whose metric differs from the one received.
//
// A base equal to the source is the received value carried on unchanged, and
// that is the propagation the section forbids. A policy that rewrites the metric
// to the value it already held is indistinguishable on the wire from that
// propagation, so the MUST NOT decides the tie and the attribute is suppressed.
//
// Recorded AFTER the egress filter pass on both rails, like its sibling, so the
// base this reads is the payload the rebuild runs over rather than the source.
// Unlike its sibling the Suppress does not exist to WIN there: it is skipped
// whenever a filter originated the metric.
//
// src is computed ONCE per UPDATE by the caller rather than once per
// destination. Recording the operation unconditionally would be simpler and
// would also force every route to every external peer onto the payload-rebuild
// path, which is the cost the route-server fast path exists to avoid.
func applyFactsMED(f *peerForwardFacts, src, base medValue, mods *filterapi.ModAccumulator) {
	if medPropagationAllowedTo(f.isEBGP, f.rsClient) {
		return
	}
	// Nothing on this destination's wire to remove. Recording anyway would cost
	// the rebuild and change no byte.
	if !base.present {
		return
	}
	// Nothing was received, so whatever the base carries was originated here.
	if !src.present {
		return
	}
	// The base is an egress filter's wire override carrying a different metric:
	// origination, not propagation.
	if !base.sameAs(src) {
		return
	}
	if modsSetMED(mods) {
		return
	}
	mods.Op(uint8(attribute.AttrMED), filterapi.AttrModSuppress, nil)
}
