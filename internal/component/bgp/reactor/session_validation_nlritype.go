// Design: docs/architecture/core-design.md -- RFC 7606 UPDATE validation
// RFC: rfc/short/rfc7606.md -- Section 5.4 typed NLRI; rfc/short/rfc9552.md -- Sections 5.2 and 8.2.2
// Overview: session_validation.go -- enforceRFC7606, which applies this
// Related: ../message/rfc7606_bgpls_nlri.go -- RetainWellFormedNLRI, the Section 8.2.2 walk
//
// RFC 7606 Section 5.4: "A BGP speaker advertising support for such a typed
// address family MUST handle routes with unrecognized NLRI types within that
// address family by discarding them, unless the relevant specification for that
// address family specifies otherwise."
//
// Two things decide where this runs.
//
// The escape clause is per family, so the ruling is per family and lives in the
// nlritype registry, registered by the plugin that owns the family. Nothing here
// names a family. RFC 9552 Section 5.2 uses that clause for BGP-LS and requires
// unknown Link-State NLRI types to be preserved and propagated, so a blanket
// discard would violate a MUST rather than meet one.
//
// The discard belongs at ingress because the RIB is not the propagation gate.
// reactorForwardRS (forward_rs.go) fans the RECEIVED wire straight to every
// eligible peer off this same read goroutine, and buildFwdBody (forward_body.go)
// appends peerWire.Payload() verbatim on the same-context path. A discard at the
// RIB would stop installation and leave the relay untouched, which is the half
// Section 5.4 exists for on a speaker that is only ever a control-plane relay
// for these families.

package reactor

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

// mpNLRIEdit is one MP attribute's replacement NLRI section.
type mpNLRIEdit struct {
	code    uint8
	nlri    []byte // the surviving NLRIs; empty means drop the whole attribute
	dropped int
}

// typedNLRIOutcome is what the Section 5.4 pass decided about the UPDATE as a
// whole, as opposed to about the individual routes inside it.
type typedNLRIOutcome uint8

const (
	// typedNLRIKept: the returned UPDATE still conveys reachability and is
	// installed and relayed as usual. Covers every conforming UPDATE.
	typedNLRIKept typedNLRIOutcome = iota
	// typedNLRIEmptied: every route the UPDATE carried was discarded, so what is
	// left conveys nothing and must not be relayed. See applyTypedNLRIDiscard.
	typedNLRIEmptied
	// typedNLRIUnparseable: the MP NLRI framing overruns its attribute, which
	// RFC 7606 Section 5.3 makes incorrect and Section 3(j) routes to session reset.
	typedNLRIUnparseable
)

// applyTypedNLRIDiscard removes from the UPDATE's MP attributes every route its
// family's own rules discard one at a time: an unrecognized NLRI type under
// RFC 7606 Section 5.4, and a malformed Link-State NLRI under RFC 9552
// Section 8.2.2. It returns the UPDATE downstream consumers will see, that
// UPDATE's attribute section, and what the pass decided about the UPDATE as a
// whole.
//
// Returns wu and pathAttrs unchanged, sharing the received wire bytes, whenever
// nothing was discarded. That covers every family with no Section 5.4 ruling and
// every conforming UPDATE, so the zero-copy relay survives for every peer that
// sends types ze implements. The rewrite allocates only when a route is
// genuinely discarded.
//
// The typedNLRIEmptied outcome exists because an UPDATE that conveys nothing is
// not a harmless UPDATE. It takes two shapes and both must be dropped:
//
//   - MP_UNREACH was the only attribute, so nothing is left at all. RebuildUpdateBody
//     emits four zero octets, which IS the RFC 4724 Section 2 End-of-RIB marker.
//     Relaying it forges an EoR the peer never sent, ending a restarting peer's
//     RFC 4724 route deferral early.
//   - Other attributes survive (ORIGIN, AS_PATH) but every route is gone. RFC 7606
//     Section 5.2 calls that shape one where "we cannot be confident that the NLRI
//     have been successfully parsed", and it advertises and withdraws nothing.
//
// Neither has a wire encoding that means "an UPDATE that says nothing", so the only
// correct answer is not to relay it.
//
// pathAttrs must be the attribute section of wu.Payload() as it stands after the
// Section 3.g keep-first strip, and attrsOffset its start within that payload.
func (s *Session) applyTypedNLRIDiscard(
	wu *wireu.WireUpdate,
	pathAttrs []byte,
	attrsOffset int,
	mpReach, mpUnreach message.MPNLRILocation,
	addPathFor func(afi uint16, safi uint8) bool,
) (*wireu.WireUpdate, []byte, typedNLRIOutcome) {
	edits := make([]mpNLRIEdit, 0, 2)
	edits, ok := s.typedNLRIEdit(edits, uint8(attribute.AttrMPReachNLRI), mpReach, pathAttrs, addPathFor)
	if !ok {
		return wu, pathAttrs, typedNLRIUnparseable
	}
	edits, ok = s.typedNLRIEdit(edits, uint8(attribute.AttrMPUnreachNLRI), mpUnreach, pathAttrs, addPathFor)
	if !ok {
		return wu, pathAttrs, typedNLRIUnparseable
	}
	if len(edits) == 0 {
		return wu, pathAttrs, typedNLRIKept
	}

	newAttrs := rewriteMPNLRISections(pathAttrs, edits)
	oldCtxID := wu.SourceCtxID()
	oldSourceID := wu.SourceID()
	newBody := message.RebuildUpdateBody(wu.Payload(), newAttrs)
	rebuilt := wireu.NewWireUpdate(newBody, oldCtxID)
	rebuilt.SetSourceID(oldSourceID)

	outcome := typedNLRIKept
	if updateCarriesNoRoutes(newBody) {
		outcome = typedNLRIEmptied
	}
	// Keep pathAttrs a subslice of the body the caller now holds, so the
	// attribute-discard branch's in-place rewrite still shows through.
	return rebuilt, newBody[attrsOffset : attrsOffset+len(newAttrs)], outcome
}

// typedNLRIEdit appends the edit for one MP attribute, or nothing when that
// attribute is absent, its family has no ruling, or every route survives.
//
// Two rulings can remove a route here and a family may carry either or both.
// RFC 7606 Section 5.4 removes a route whose NLRI TYPE ze does not implement, in
// a family that has not overridden the rule. RFC 9552 Section 8.2.2 removes a
// Link-State NLRI whose SYNTAX is malformed in a way that lets ze skip it and
// keep processing the UPDATE. Both are per route and both keep the UPDATE, so one
// pass applies them in that order and one rewrite carries the result.
//
// Returns false when the attribute's NLRI framing could not be walked. RFC 7606
// Section 5.3 makes an MP attribute incorrect when "the length of the last NLRI
// found exceeds the amount of unconsumed data remaining in the attribute", and
// Section 3(j) says treat-as-withdraw needs the NLRI field parsed, so "if this is
// not possible ... the 'session reset' approach ... MUST be followed". RFC 9552
// Section 8.2.2 names the same class for BGP-LS and prescribes the same verdict.
// The caller applies it.
//
// Nothing upstream has already made this check for a typed family.
// message.validateMPNLRISyntax runs the Section 5.3 walk only for IPv4 and IPv6
// unicast and multicast, whose NLRI is a plain list of length-prefixed prefixes,
// and for BGP-LS, whose own RFC states a walk. It returns nil for every other
// typed family. Relaying the section unchanged on a framing error would therefore
// have handed a peer a one-byte way to bypass the Section 5.4 MUST: append one
// truncated NLRI and the split fails, so no route in the attribute is judged.
func (s *Session) typedNLRIEdit(
	edits []mpNLRIEdit,
	code uint8,
	loc message.MPNLRILocation,
	pathAttrs []byte,
	addPathFor func(afi uint16, safi uint8) bool,
) ([]mpNLRIEdit, bool) {
	if !loc.Present {
		return edits, true
	}
	fam := family.Family{AFI: family.AFI(loc.AFI), SAFI: family.SAFI(loc.SAFI)}
	// One registry read, not two: the answer is carried into Retain rather than looked up
	// again there. This is the gate every MP-bearing UPDATE passes, IPv6 unicast included.
	recognize := nlritype.Get(fam)
	// RFC 9552 Section 8.2.2 rules on the SYNTAX of an individual Link-State NLRI where
	// Section 5.4 rules on the TYPE of an individual typed NLRI. Both prescribe discarding
	// the NLRI and keeping the UPDATE, so one pass applies both and one rewrite carries
	// them. A family with neither ruling leaves before the attribute is located.
	afi := attribute.AFI(loc.AFI)
	safi := attribute.SAFI(loc.SAFI)
	if recognize == nil && !message.NLRISyntaxRuled(afi, safi) {
		return edits, true
	}

	_, _, value, found := attribute.AttrFind(pathAttrs, attribute.AttributeCode(code))
	if !found {
		return edits, true
	}
	start, ok := message.MPNLRIStart(code, value)
	if !ok {
		return edits, true
	}

	addPath := addPathFor != nil && addPathFor(loc.AFI, loc.SAFI)
	kept, dropped, err := nlritype.Retain(recognize, fam, value[start:], addPath)
	if err != nil {
		sessionLogger().Warn("RFC 7606 Section 5.3: MP NLRI framing overruns the attribute",
			"peer", s.settings.Address, "family", fam, "attr", code, "error", err)
		return edits, false
	}
	if dropped > 0 {
		// RFC 7606 Section 6: name what was dropped, so an operator can trace a route
		// that stopped arriving back to the type ze does not implement.
		//
		// Debug, not Info. A peer decides how often this fires: one line per UPDATE it
		// sends carrying a type ze does not implement, on the receive goroutine, with the
		// slog argument boxing that costs. Section 6 asks for a debugging facility, and
		// that is what this is; the record an operator is owed for a route that stopped
		// arriving comes from rfc7606Diagnostics, which enforceRFC7606 still calls on
		// every non-None action.
		//
		// The louder levels in this package are kept for outcomes a peer cannot repeat
		// cheaply: each Warn here fires once per session-resetting UPDATE, and the session
		// then goes down.
		sessionLogger().Debug("RFC 7606 Section 5.4: discarded routes with unrecognized NLRI types",
			"peer", s.settings.Address, "family", fam, "attr", code, "discarded", dropped)
	}

	// RFC 9552 Section 8.2.2: "A BGP-LS Speaker MUST perform the following syntactic
	// validation of the Link-State NLRI to determine if it is malformed", and a malformed
	// NLRI it can skip past "MUST" be handled "as 'NLRI discard'". Runs on the survivors of
	// the Section 5.4 filter, so the two rulings compose in one direction only.
	//
	// A framing failure here means the same thing it means above, and takes the same route:
	// the boundaries between NLRIs are unknowable, so no discard decision is possible.
	// Section 8.2.2 calls that the non-skipable class and prescribes a session reset for it,
	// which is the verdict the caller reaches on a false return.
	kept, malformed, framed := message.RetainWellFormedNLRI(afi, safi, kept, addPath)
	if !framed {
		sessionLogger().Warn("RFC 9552 Section 8.2.2: Link-State NLRI lengths do not sum to the MP attribute length",
			"peer", s.settings.Address, "family", fam, "attr", code)
		return edits, false
	}
	if malformed > 0 {
		// RFC 9552 Section 8.2.2: "An implementation SHOULD log a message for any errors
		// found during syntax validation for further analysis." Debug for the reason the
		// Section 5.4 line below gives: a peer decides how often it fires.
		sessionLogger().Debug("RFC 9552 Section 8.2.2: discarded malformed Link-State NLRIs",
			"peer", s.settings.Address, "family", fam, "attr", code, "discarded", malformed)
	}

	if dropped+malformed == 0 {
		return edits, true
	}
	return append(edits, mpNLRIEdit{code: code, nlri: kept, dropped: dropped + malformed}), true
}

// updateCarriesNoRoutes reports whether an UPDATE body conveys no reachability at
// all: no withdrawn routes, no IPv4 NLRI, and no MP_REACH or MP_UNREACH attribute.
//
// RFC 7606 Section 5.2 states the same shape from the other side: apart from the
// End-of-RIB marker, "an UPDATE message either carries only withdrawn routes ...
// or it advertises reachable routes". A body that does neither is either an EOR or
// nothing, and ze must not relay either one on a peer's behalf.
//
// A malformed body reads as carrying routes, and that branch IS reachable: the
// validator abandons its walk on an RFC 7606 Section 4 framing error and still
// reports the MP attributes it read first, so the body rebuilt here can carry an
// unparseable tail. Answering false there is deliberate. "This UPDATE conveys
// nothing" is a claim about bytes that parsed; on bytes that did not, the honest
// answer is to leave the UPDATE on the path it would have taken anyway rather
// than drop it on a guess. The framing error itself is judged upstream, where
// Sections 5.2, 5.3 and 3(j) decide between escalation and session reset.
func updateCarriesNoRoutes(body []byte) bool {
	if len(body) < 4 {
		return false
	}
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	withdrawnEnd := 2 + withdrawnLen
	if withdrawnEnd+2 > len(body) {
		return false
	}
	if withdrawnLen > 0 {
		return false
	}
	attrLen := int(binary.BigEndian.Uint16(body[withdrawnEnd : withdrawnEnd+2]))
	attrStart := withdrawnEnd + 2
	if attrStart+attrLen > len(body) {
		return false
	}
	if attrStart+attrLen < len(body) {
		return false // IPv4 NLRI follows the attributes
	}
	attrs := body[attrStart : attrStart+attrLen]
	if _, _, _, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); found {
		return false
	}
	if _, _, _, found := attribute.AttrFind(attrs, attribute.AttrMPUnreachNLRI); found {
		return false
	}
	return true
}

// rewriteMPNLRISections returns a new attribute section with each edited MP
// attribute's NLRI replaced, and an attribute whose NLRI is now empty removed.
//
// Attribute flags are preserved byte for byte, including the Extended Length
// bit. A replacement NLRI is never longer than the one it replaces, so the
// original length encoding always still fits and re-deciding it could only
// introduce a difference the peer did not send.
func rewriteMPNLRISections(pathAttrs []byte, edits []mpNLRIEdit) []byte {
	out := make([]byte, 0, len(pathAttrs))

	pos := 0
	for pos+2 <= len(pathAttrs) {
		attrStart := pos
		flags := pathAttrs[pos]
		code := pathAttrs[pos+1]
		pos += 2

		// Every abandonment below rewinds pos to attrStart first. The trailing copy at the
		// end of this function starts at pos, so leaving it past the flags and type code
		// would drop those two to four header bytes from the rebuilt attribute section --
		// silently corrupting the wire on exactly the malformed input the walk gave up on.
		lenWidth := 1
		var attrLen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				pos = attrStart
				break
			}
			lenWidth = 2
			attrLen = int(pathAttrs[pos])<<8 | int(pathAttrs[pos+1])
		} else {
			if pos+1 > len(pathAttrs) {
				pos = attrStart
				break
			}
			attrLen = int(pathAttrs[pos])
		}
		pos += lenWidth
		if pos+attrLen > len(pathAttrs) {
			pos = attrStart
			break
		}
		value := pathAttrs[pos : pos+attrLen]
		pos += attrLen

		edit, edited := findEdit(edits, code)
		if !edited {
			out = append(out, pathAttrs[attrStart:pos]...)
			continue
		}
		if len(edit.nlri) == 0 {
			// Every route in this attribute was discarded. An MP_REACH announcing
			// nothing, or an MP_UNREACH withdrawing nothing, is not something to relay.
			continue
		}

		start, ok := message.MPNLRIStart(code, value)
		if !ok {
			out = append(out, pathAttrs[attrStart:pos]...)
			continue
		}
		newLen := start + len(edit.nlri)
		out = append(out, flags, code)
		if lenWidth == 2 {
			out = append(out, byte(newLen>>8), byte(newLen))
		} else {
			out = append(out, byte(newLen))
		}
		out = append(out, value[:start]...)
		out = append(out, edit.nlri...)
	}

	// Any trailing bytes the walk could not frame are copied through untouched.
	//
	// This IS a live path, not a belt on a brace. ValidateUpdateRFC7606AddPath abandons its
	// own walk on an RFC 7606 Section 4 framing error and still reports the MP attributes it
	// read before it, so this function is reached with a section whose tail was never
	// validated. Copying the tail verbatim is what keeps the rebuild honest: the same bounds
	// conditions stop attribute.AttrFind, AttrIterator.Next and this walk at the same octet,
	// so every consumer of the rebuilt body sees the identical prefix and the identical
	// unparseable remainder the peer sent.
	if pos < len(pathAttrs) {
		out = append(out, pathAttrs[pos:]...)
	}
	return out
}

// findEdit returns the edit for an attribute code. At most two edits exist, so a
// linear scan beats a map allocation.
func findEdit(edits []mpNLRIEdit, code uint8) (mpNLRIEdit, bool) {
	for _, e := range edits {
		if e.code == code {
			return e, true
		}
	}
	return mpNLRIEdit{}, false
}
