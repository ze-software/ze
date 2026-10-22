// Design: docs/architecture/core-design.md -- RFC 7606 UPDATE validation
// RFC: rfc/short/rfc7606.md -- Section 5.4 typed NLRI; rfc/short/rfc9552.md -- Sections 5.2 and 8.2.2
// Overview: session_validation.go -- enforceRFC7606, which applies this
// Related: ../../../core/bgp/nlri/nlrisyntax/nlrisyntax.go -- the per-family NLRI syntax rules
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
	"encoding/hex"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

// mpNLRIEdit is one MP attribute's replacement NLRI section.
type mpNLRIEdit struct {
	code    uint8
	afi     uint16
	safi    uint8
	nlri    []byte // the surviving NLRIs; empty means drop the whole attribute
	dropped int
	// withdrawn holds the announced NLRIs the family's recognizer answered
	// treat-as-withdraw for, path identifiers included, relayed with their own
	// bytes. Only an MP_REACH edit has any.
	withdrawn []byte
	// replaced holds the held generations that stand in for malformed NLRIs of
	// this attribute (typedNLRIWithdrawable). Either edit can carry them.
	replaced []byte
}

// Label values of ze_bgp_update_nlri_malformed_total.
const (
	typedNLRIActionWithdrawn = "withdrawn"
	typedNLRIActionDiscarded = "discarded"
)

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
// Section 8.2.2. An announced route its family's recognizer answers
// treat-as-withdraw for (draft-ietf-bess-mup-safi Section 3.1.3.1) leaves the
// MP_REACH_NLRI and is withdrawn: in an MP_UNREACH_NLRI added to the UPDATE, or,
// when the UPDATE already carries one, in the returned withdrawBody, a
// withdraw-only UPDATE the caller dispatches. It returns the UPDATE downstream
// consumers will see, that UPDATE's attribute section, what the pass decided
// about the UPDATE as a whole, and that withdraw body (nil when none).
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
// attrs is the view a family's UPDATE-level check reads (nlritype.UpdateCheck):
// pathAttrs with the codes attribute-discard removes after this pass marked absent.
// withdrawAll says the whole UPDATE is treated as withdrawn (RFC 7606 Section 2),
// so no route it announces is held (typedNLRIHold).
//
// A withdrawal in a family whose recognizer judges the body is relayed by the
// generation of the route this session held, never by malformed bytes, and not at
// all when no generation is held (session_validation_nlritype_held.go). That
// applies to the routes moved out of the MP_REACH_NLRI and to a received
// MP_UNREACH_NLRI alike.
func (s *Session) applyTypedNLRIDiscard(
	wu *wireu.WireUpdate,
	pathAttrs []byte,
	attrsOffset int,
	attrs *nlritype.UpdateAttrs,
	mpReach, mpUnreach message.MPNLRILocation,
	addPathFor func(afi uint16, safi uint8) bool,
	withdrawAll bool,
) (out *wireu.WireUpdate, outAttrs []byte, outcome typedNLRIOutcome, withdrawBodies [][]byte) {
	// Stamps the routes this UPDATE files in the hold (typedNLRIRelayEdit).
	s.typedNLRIUpdate++
	edits := make([]mpNLRIEdit, 0, 2)
	edits, ok := s.typedNLRIEdit(edits, uint8(attribute.AttrMPReachNLRI), mpReach, pathAttrs, attrs, addPathFor, withdrawAll)
	if !ok {
		return wu, pathAttrs, typedNLRIUnparseable, nil
	}
	edits, ok = s.typedNLRIEdit(edits, uint8(attribute.AttrMPUnreachNLRI), mpUnreach, pathAttrs, attrs, addPathFor, withdrawAll)
	if !ok {
		return wu, pathAttrs, typedNLRIUnparseable, nil
	}
	if len(edits) == 0 {
		return wu, pathAttrs, typedNLRIKept, nil
	}

	newAttrs := rewriteMPNLRISections(pathAttrs, edits)

	// RFC 7606 Section 2 treat-as-withdraw, per route: the NLRIs the family's own
	// specification answers that way (draft-ietf-bess-mup-safi Section 3.1.3.1) left
	// the MP_REACH above and become a withdrawal here, so an earlier generation of the
	// same route leaves the Adj-RIB-In and is withdrawn wherever it was relayed. A
	// malformed NLRI is withdrawn by that earlier generation's bytes (typedNLRIEdit).
	// RFC 7606 Section 3(g) refuses a second MP_UNREACH_NLRI in one UPDATE, so a
	// withdrawal joins this UPDATE only while it carries none and the result fits
	// the message limit; otherwise it rides withdraw-only UPDATEs the caller
	// dispatches, sized to that limit (a held generation can be longer than the
	// malformed NLRI it replaces).
	var err error
	if reach, found := findEdit(edits, uint8(attribute.AttrMPReachNLRI)); found {
		newAttrs, withdrawBodies, err = s.typedNLRIPlaceWithdrawals(wu.Payload(), pathAttrs, newAttrs, withdrawBodies,
			reach.afi, reach.safi, append(reach.withdrawn, reach.replaced...), addPathFor)
		if err != nil {
			return wu, pathAttrs, typedNLRIUnparseable, nil
		}
	}
	if unreach, found := findEdit(edits, uint8(attribute.AttrMPUnreachNLRI)); found {
		newAttrs, withdrawBodies, err = s.typedNLRIPlaceWithdrawals(wu.Payload(), pathAttrs, newAttrs, withdrawBodies,
			unreach.afi, unreach.safi, unreach.replaced, addPathFor)
		if err != nil {
			return wu, pathAttrs, typedNLRIUnparseable, nil
		}
	}
	oldCtxID := wu.SourceCtxID()
	oldSourceID := wu.SourceID()
	newBody := message.RebuildUpdateBody(wu.Payload(), newAttrs)
	rebuilt := wireu.NewWireUpdate(newBody, oldCtxID)
	rebuilt.SetSourceID(oldSourceID)

	outcome = typedNLRIKept
	if updateCarriesNoRoutes(newBody) {
		outcome = typedNLRIEmptied
	}
	// Keep pathAttrs a subslice of the body the caller now holds, so the
	// attribute-discard branch's in-place rewrite still shows through.
	return rebuilt, newBody[attrsOffset : attrsOffset+len(newAttrs)], outcome, withdrawBodies
}

// typedNLRIPlaceWithdrawals carries nlri, withdrawals of one family, either in
// newAttrs as an MP_UNREACH_NLRI, when the rewritten UPDATE carries none and the
// result fits the session's UPDATE limit, or in withdraw-only bodies appended to
// bodies. body is the UPDATE before the rewrite and pathAttrs its attribute
// section, so the rewritten size is known without building it.
func (s *Session) typedNLRIPlaceWithdrawals(
	body, pathAttrs, newAttrs []byte, bodies [][]byte, afi uint16, safi uint8, nlri []byte,
	addPathFor func(afi uint16, safi uint8) bool,
) ([]byte, [][]byte, error) {
	if len(nlri) == 0 {
		return newAttrs, bodies, nil
	}
	addPath := addPathFor != nil && addPathFor(afi, safi)
	if _, _, _, hasUnreach := attribute.AttrFind(newAttrs, attribute.AttrMPUnreachNLRI); !hasUnreach {
		// An Extended Length MP_UNREACH_NLRI header (4) and its AFI and SAFI (3).
		size := len(body) - len(pathAttrs) + len(newAttrs) + 4 + 3 + len(nlri)
		if size <= int(message.MaxMessageLength(msgtype.TypeUPDATE, s.extendedMessage))-message.HeaderLen {
			return message.AppendMPUnreachNLRI(newAttrs, afi, safi, nlri), bodies, nil
		}
	}
	bodies, err := s.typedNLRIWithdrawBodies(bodies, afi, safi, nlri, addPath)
	return newAttrs, bodies, err
}

// typedNLRIUpdateAttrs is the attribute view a family's UPDATE-level check reads.
//
// Attribute-discard rewrites the attribute section only after the typed NLRI pass,
// so the codes it will remove are still in pathAttrs. They are marked absent here,
// because the route is installed and relayed without them: a DSD route whose
// Prefix-SID the RFC 8669 Section 4 EBGP rule discards arrives with no Prefix-SID,
// and draft-ietf-bess-mup-safi Section 3.3.6 withdraws it.
func typedNLRIUpdateAttrs(pathAttrs []byte, result *message.RFC7606ValidationResult) nlritype.UpdateAttrs {
	attrs := nlritype.NewUpdateAttrs(pathAttrs)
	if result.Action != message.RFC7606ActionAttributeDiscard {
		return attrs
	}
	for _, entry := range result.DiscardEntries {
		attrs.Discarding(attribute.AttributeCode(entry.Code))
	}
	return attrs
}

// typedNLRIEdit appends the edit for one MP attribute, or nothing when that
// attribute is absent, its family has no ruling, or every route survives.
//
// Three rulings can remove a route here and a family may carry any of them.
// RFC 7606 Section 5.4 removes a route whose NLRI TYPE ze does not implement, in
// a family that has not overridden the rule; the same recognizer moves an
// announced route with a malformed body to the edit's withdrawn NLRIs when its
// family prescribes treat-as-withdraw, or discards it when its route key cannot
// form. The family's UPDATE-level check does the same for an announced route its
// specification withdraws because of the UPDATE carrying it (draft-ietf-bess-mup-safi
// Section 3.3.6: a DSD route with no Prefix-SID). RFC 9552 Section 8.2.2 removes a
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
// Nothing upstream has made this check for every typed family.
// message.validateMPNLRISyntax runs the Section 5.3 walk only for IPv4 and IPv6
// unicast and multicast, whose NLRI is a plain list of length-prefixed prefixes,
// and the registered Judge of a family whose own specification states a syntax
// rule (nlrisyntax). It returns nil for every other typed family. Relaying the
// section unchanged on a framing error would therefore
// have handed a peer a one-byte way to bypass the Section 5.4 MUST: append one
// truncated NLRI and the split fails, so no route in the attribute is judged.
func (s *Session) typedNLRIEdit(
	edits []mpNLRIEdit,
	code uint8,
	loc message.MPNLRILocation,
	pathAttrs []byte,
	attrs *nlritype.UpdateAttrs,
	addPathFor func(afi uint16, safi uint8) bool,
	withdrawAll bool,
) ([]mpNLRIEdit, bool) {
	if !loc.Present {
		return edits, true
	}
	fam := family.Family{AFI: family.AFI(loc.AFI), SAFI: family.SAFI(loc.SAFI)}
	// One registry read, not two: the answer is carried into Retain rather than looked up
	// again there. This is the gate every MP-bearing UPDATE passes, IPv6 unicast included.
	recognize, check := nlritype.Rulings(fam)
	// A family's registered syntax rule (nlrisyntax) rules on the SYNTAX of an individual
	// NLRI where Section 5.4 rules on its TYPE. Both remove the NLRI and keep the UPDATE,
	// so one pass applies both and one rewrite carries them. A family with neither ruling
	// leaves before the attribute is located.
	syntax, syntaxRuled := nlrisyntax.Get(fam)
	if syntax.Retain == nil {
		syntaxRuled = false
	}
	if recognize == nil && check == nil && !syntaxRuled {
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
	announce := code == uint8(attribute.AttrMPReachNLRI)
	retained, err := nlritype.RetainUpdate(recognize, check, attrs, fam, value[start:], addPath, announce, s.typedNLRIKeyScratch[:])
	kept, dropped := retained.Kept, retained.Discarded
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

	// The family's registered syntax rule runs on the survivors of the Section 5.4 filter,
	// so the two rulings compose in one direction only. RFC 9552 Section 8.2.2 (BGP-LS) and
	// RFC 9830 Section 5 (SR Policy) remove a malformed NLRI the walk can skip past; RFC 7432
	// Section 7.1 (EVPN) removes none and reports a wrong Ethernet A-D length unwalkable.
	//
	// A framing failure here means the same thing it means above, and takes the same route:
	// the boundaries between NLRIs are unknowable, so no removal decision is possible, and
	// the caller reaches the session reset on a false return.
	malformed := 0
	if syntaxRuled {
		var framed bool
		kept, malformed, framed = syntax.Retain(kept, addPath)
		if !framed {
			sessionLogger().Warn("NLRI syntax rule: the MP NLRI section cannot be walked",
				"peer", s.settings.Address, "family", fam, "attr", code, "rule", syntax.Citation)
			return edits, false
		}
	}
	if malformed > 0 {
		// RFC 9552 Section 8.2.2: "An implementation SHOULD log a message for any errors
		// found during syntax validation for further analysis." Debug for the reason the
		// Section 5.4 line above gives: a peer decides how often it fires.
		sessionLogger().Debug(syntax.RemovedMessage,
			"peer", s.settings.Address, "family", fam, "attr", code, "discarded", malformed)
	}

	if retained.WithdrawnCount+retained.UnkeyableCount > 0 {
		s.logTypedNLRIMalformed(fam, code, retained)
	}

	// A family whose recognizer judges the body holds each route's last well-formed
	// generation, and every withdrawal it relays names a held route by those bytes.
	withdrawn := retained.Withdrawn
	var replaced []byte
	relayDropped, relayChanged := 0, false
	if recognize != nil && nlritype.JudgesBody(fam) {
		relay, holdErr := s.typedNLRIRelayEdit(fam, recognize, kept, withdrawn, addPath, announce, withdrawAll)
		if holdErr != nil {
			sessionLogger().Warn("RFC 7606 Section 5.3: typed NLRI section cannot be walked",
				"peer", s.settings.Address, "family", fam, "attr", code, "error", holdErr)
			return edits, false
		}
		relayDropped, relayChanged, replaced = relay.dropped, relay.changed, relay.replaced
		if announce {
			withdrawn = relay.nlri
		} else {
			kept = relay.nlri
		}
		if relayDropped > 0 {
			// Debug, for the reason the Section 5.4 line above gives.
			sessionLogger().Debug("RFC 7606 Section 2: withdrawn NLRIs naming no held route not relayed",
				"peer", s.settings.Address, "family", fam, "attr", code, "dropped", relayDropped)
		}
	}

	// A section whose routes all arrived in the family's current encoding, and lost
	// none, is relayed as received. One that held a route in an earlier encoding
	// carries it in the current one (nlritype.RegisterCanonical,
	// draft-ietf-bess-mup-safi Section 3.1.3.1), so it is rewritten too.
	if dropped+malformed+retained.WithdrawnCount+retained.Canonicalized == 0 && !relayChanged {
		return edits, true
	}
	return append(edits, mpNLRIEdit{
		code: code, afi: loc.AFI, safi: loc.SAFI,
		nlri: kept, dropped: dropped + malformed + relayDropped, withdrawn: withdrawn, replaced: replaced,
	}), true
}

// typedNLRIRelayEdit applies the held-generation rule to one MP attribute of a
// body-judged family. For an MP_REACH_NLRI it files the kept routes
// (typedNLRIHold) and returns the withdrawable form of the routes moved out of it;
// for an MP_UNREACH_NLRI it returns the withdrawable form of the section itself.
// The announcements are filed first, stamped with this UPDATE, so a withdrawal in
// the same UPDATE of a key it announces is left out (RFC 4271 Section 4.3); the
// MP_REACH_NLRI is edited before the MP_UNREACH_NLRI (applyTypedNLRIDiscard), so
// the stamp covers both.
func (s *Session) typedNLRIRelayEdit(
	fam family.Family, recognize nlritype.Recognizer, kept, withdrawn []byte, addPath, announce, withdrawAll bool,
) (typedNLRIRelay, error) {
	if !announce {
		return s.typedNLRIWithdrawable(fam, recognize, kept, addPath, true)
	}
	if err := s.typedNLRIHold(fam, kept, addPath, withdrawAll); err != nil {
		return typedNLRIRelay{}, err
	}
	return s.typedNLRIWithdrawable(fam, recognize, withdrawn, addPath, false)
}

// logTypedNLRIMalformed records the NLRIs a family's recognizer answered
// treat-as-withdraw for: logged with their bytes, as RFC 7606 Section 6 asks for
// "an error listing the NLRI involved", and counted per peer.
//
// Debug, for the reason the Section 5.4 line in typedNLRIEdit gives: a peer decides
// how often it fires. The counter is what an operator watches.
func (s *Session) logTypedNLRIMalformed(fam family.Family, code uint8, retained nlritype.Retained) {
	if retained.WithdrawnCount > 0 {
		sessionLogger().Debug("RFC 7606 Section 2: malformed NLRIs treated as withdrawn",
			"peer", s.settings.Address, "family", fam, "attr", code,
			"withdrawn", retained.WithdrawnCount, "nlri", hex.EncodeToString(retained.Withdrawn))
	}
	if len(retained.Unkeyable) > 0 {
		sessionLogger().Debug("RFC 7606 Section 2: malformed NLRIs discarded, no route key forms to withdraw them by",
			"peer", s.settings.Address, "family", fam, "attr", code, "nlri", hex.EncodeToString(retained.Unkeyable))
	}
	if s.prefixMetrics == nil {
		return
	}
	if retained.WithdrawnCount > 0 {
		s.prefixMetrics.typedNLRIMalformed.With(s.addrLabel, typedNLRIActionWithdrawn).Add(float64(retained.WithdrawnCount))
	}
	if retained.UnkeyableCount > 0 {
		s.prefixMetrics.typedNLRIMalformed.With(s.addrLabel, typedNLRIActionDiscarded).Add(float64(retained.UnkeyableCount))
	}
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
// bit, while the new length still fits the received length encoding, so the
// rewrite introduces no difference the peer did not send. A replacement NLRI is
// shorter than the one it replaces (a held generation that stands in for a
// malformed NLRI travels apart, in typedNLRIPlaceWithdrawals) unless it carries a
// route in its family's canonical form, which can be longer
// (nlritype.RegisterCanonical). An attribute that then passes 255 octets gains the
// Extended Length bit: RFC 4271 Section 4.3, "If the Extended Length bit of the
// Attribute Flags octet is set to 1, the third and fourth octets of the path
// attribute contain the length of the attribute data in octets."
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
		if flags&0x10 == 0 {
			if pos+1 > len(pathAttrs) {
				pos = attrStart
				break
			}
			attrLen = int(pathAttrs[pos])
		} else {
			if pos+2 > len(pathAttrs) {
				pos = attrStart
				break
			}
			lenWidth = 2
			attrLen = int(pathAttrs[pos])<<8 | int(pathAttrs[pos+1])
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
		if lenWidth == 1 && newLen > 0xff {
			flags |= 0x10
			lenWidth = 2
		}
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
