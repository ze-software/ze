// Design: docs/architecture/core-design.md — RFC 7606 UPDATE validation
// Overview: session.go — BGP session struct and lifecycle
// RFC: rfc/short/rfc7606.md — revised UPDATE error handling

package reactor

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/wireu"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// enforceRFC7606 validates an UPDATE per RFC 7606 and enforces the resulting action.
//
// Returns the (potentially new) WireUpdate, the action taken, and an error if
// session-reset is required. When attribute-discard applies, ATTR_TOMBSTONE markers
// are written into the wire bytes per draft-mangin-idr-attr-tombstone-00.
//
// The marker is stamped here, at receive time, into the shared received wire. The
// Transitive bit derived here (Section 4.2) is what IBGP peers see. Section 5.3's
// EBGP-boundary clear is applied per destination on the egress wire path
// (wireu.rewriteASPathPrepend), where the destination session type is known.
// Called from processMessage() BEFORE callback dispatch so that malformed
// UPDATEs are never delivered to plugins as valid routes.
func (s *Session) enforceRFC7606(wu *wireu.WireUpdate) (*wireu.WireUpdate, message.RFC7606Action, error) {
	body := wu.Payload()

	// RFC 7911: NLRI in an ADD-PATH family carries a 4-byte Path Identifier before the
	// prefix. The Section 5.3 NLRI-syntax checks must skip it or they would misread a valid
	// ADD-PATH UPDATE as malformed and session-reset it. The receive context knows which
	// families negotiated ADD-PATH; AddPathFor is nil-safe.
	recvCtx := bgpctx.Registry.Get(s.recvCtxID)
	addPathFor := func(afi uint16, safi uint8) bool {
		return recvCtx.AddPathFor(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)})
	}
	ipv4AddPath := addPathFor(uint16(family.AFIIPv4), uint8(family.SAFIUnicast))
	// The IPv4 Withdrawn Routes and NLRI fields are always IPv4 unicast. The common case has
	// no ADD-PATH there, so use the plain validator; only reach for the add-path-aware one
	// when RFC 7911 is negotiated for the family.
	checkIPv4NLRI := func(field []byte) *message.RFC7606ValidationResult {
		if ipv4AddPath {
			return message.ValidateNLRISyntaxAddPath(field, false, true)
		}
		return message.ValidateNLRISyntax(field, false)
	}

	// RFC 7606 Section 3 (b): a structural length conflict means the section boundaries
	// cannot be trusted, so the NLRI field cannot be located at all. Section 3 (j) is
	// explicit that treat-as-withdraw requires the NLRI to be successfully parsed, and
	// "if this is not possible ... the 'session reset' approach ... MUST be followed".
	if len(body) < 4 {
		return s.rfc7606SessionReset(wu, "RFC 7606 Section 3(b): UPDATE too short for section headers")
	}

	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2 + withdrawnLen
	if offset+2 > len(body) {
		return s.rfc7606SessionReset(wu, "RFC 7606 Section 3(b): Withdrawn Routes Length exceeds UPDATE")
	}

	// RFC 7606 Section 3 (i)/5.3: the Withdrawn Routes field is checked for syntactic
	// correctness in the same manner as the NLRI field. Honor the action the validator
	// reports -- do not flatten every syntax error to treat-as-withdraw.
	if withdrawnLen > 0 {
		withdrawn := body[2 : 2+withdrawnLen]
		if result := checkIPv4NLRI(withdrawn); result != nil {
			return s.rfc7606NLRISyntaxAction(wu, result, "withdrawn")
		}
	}

	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(body) {
		return s.rfc7606SessionReset(wu, "RFC 7606 Section 3(b): Total Attribute Length exceeds UPDATE")
	}

	pathAttrs := body[offset : offset+attrLen]
	nlriLen := len(body) - (offset + attrLen)
	hasNLRI := nlriLen > 0

	// RFC 7606 Section 5.3: Validate IPv4 unicast body NLRI syntax, ADD-PATH-aware (RFC 7911).
	if nlriLen > 0 {
		nlri := body[offset+attrLen:]
		if result := checkIPv4NLRI(nlri); result != nil {
			return s.rfc7606NLRISyntaxAction(wu, result, "nlri")
		}
	}

	// Validate path attributes per RFC 7606
	isIBGP := s.settings.LocalAS == s.settings.PeerAS
	asn4 := false
	if neg := s.Negotiated(); neg != nil {
		asn4 = neg.ASN4
	}
	result := message.ValidateUpdateRFC7606AddPath(pathAttrs, hasNLRI, isIBGP, asn4, addPathFor)

	// RFC 8669 Section 4: discard PrefixSID from EBGP unless configured to accept.
	//
	// Presence comes from the walk above, not from a second walk of the same bytes.
	// PrefixSIDPresent is false whenever that walk abandoned the section early. Every
	// such abandonment carries treat-as-withdraw or session-reset, and the guards below
	// already decline to act on both. The two forms therefore agree on every input.
	if !isIBGP && !s.settings.AcceptSRv6PrefixSID {
		if result.PrefixSIDPresent {
			entry := message.DiscardEntry{Code: uint8(attribute.AttrPrefixSID), Reason: message.DiscardReasonEBGPInvalid}
			if result.Action < message.RFC7606ActionAttributeDiscard {
				// Raise the action ON the validator's own result. Building a fresh one
				// here dropped every field this branch does not own, DuplicateRanges
				// above all: without it the Section 3.g keep-first strip below silently
				// did nothing, so a duplicated attribute stayed on the wire. When the
				// duplicate was the Prefix-SID itself, the copy the discard did not
				// reach survived and Section 4's MUST was violated on the wire.
				//
				// The fields set here are exactly the ones the validator leaves zero on
				// this path (Action None means no strongest error was recorded), so the
				// verdict this produces is the one the fresh struct produced.
				result.Action = message.RFC7606ActionAttributeDiscard
				result.AttrCode = uint8(attribute.AttrPrefixSID)
				result.Description = "RFC 8669 Section 4: PrefixSID from EBGP discarded (not configured to accept)"
			}
			if result.Action == message.RFC7606ActionAttributeDiscard {
				result.DiscardEntries = append(result.DiscardEntries, entry)
			}
		}
	}

	// RFC 7606 Section 3.g keep-first: strip duplicate non-MP attributes recorded by the
	// validator so every downstream consumer (RIB, filters, cross-context re-encode) sees a
	// single occurrence of each code. The attribute index (attribute.AttributesWire) rejects
	// a duplicate code as a hard error, which silently drops MP routes at the RIB
	// (rib_structured.go MPReach/MPUnreach return nil on that error and skip the family).
	// MP_REACH/MP_UNREACH duplicates are session-reset by the validator and never recorded
	// here. Nothing to strip for session-reset (not processed) or treat-as-withdraw (the body
	// is re-synthesized into withdrawals downstream). Reuses the ATTR_DISCARD rebuild path and
	// allocates only on this malformed-input path.
	if len(result.DuplicateRanges) > 0 &&
		result.Action != message.RFC7606ActionSessionReset &&
		result.Action != message.RFC7606ActionTreatAsWithdraw {
		dedupedAttrs := message.StripAttrRanges(pathAttrs, result.DuplicateRanges)
		oldCtxID := wu.SourceCtxID()
		oldSourceID := wu.SourceID()
		newBody := message.RebuildUpdateBody(body, dedupedAttrs)
		wu = wireu.NewWireUpdate(newBody, oldCtxID)
		wu.SetSourceID(oldSourceID)
		// Keep body and pathAttrs consistent (pathAttrs a subslice of body) so the
		// ATTR_DISCARD branch's in-place ApplyAttrDiscard shows through. The withdrawn
		// section is unchanged by the rebuild, so the attrs still start at offset.
		body = newBody
		pathAttrs = body[offset : offset+len(dedupedAttrs)]
		sessionLogger().Debug("RFC 7606 Section 3.g: stripped duplicate attributes keep-first",
			"peer", s.settings.Address, "count", len(result.DuplicateRanges))
	}

	// RFC 7606 Section 5.4: discard routes whose NLRI type ze does not implement, in
	// families whose own specification has not overridden that rule. Runs on the bytes
	// the Section 3.g strip left, so the two rewrites compose in one direction only.
	//
	// Skipped for treat-as-withdraw and session-reset: the first turns every route in
	// this UPDATE into a withdrawal and the second drops the session, so there is no
	// route left for Section 5.4 to discard and the walk would only cost time.
	if result.Action != message.RFC7606ActionSessionReset && result.Action != message.RFC7606ActionTreatAsWithdraw {
		wu, pathAttrs = s.applyTypedNLRIDiscard(
			wu, pathAttrs, offset, result.MPReachNLRI, result.MPUnreachNLRI, addPathFor)
		body = wu.Payload()
	}

	switch result.Action {
	case message.RFC7606ActionNone:
		return s.publishBase(wu), message.RFC7606ActionNone, nil

	case message.RFC7606ActionAttributeDiscard:
		// RFC 7606 Section 2: "The attribute MUST be discarded ... and the UPDATE
		// message continues to be processed."
		// draft-mangin-idr-attr-tombstone-00 Section 5.1: Apply ATTR_TOMBSTONE markers.
		sessionLogger().Debug("RFC 7606 attribute-discard",
			"attr", result.AttrCode,
			"discard-entries", result.DiscardEntries,
			"description", result.Description)
		// RFC 7606 Section 6: the NLRI involved and the entire malformed UPDATE.
		s.rfc7606Diagnostics("attribute-discard", wu, result.AttrCode, result.Description)

		// draft-mangin-idr-attr-tombstone-00 Section 5.1: "Implementations SHOULD log
		// the upstream pairs separately before merging to preserve diagnostic
		// traceability."
		if upstream := message.ExtractUpstreamAttrDiscard(pathAttrs); len(upstream) > 0 {
			sessionLogger().Debug("RFC 7606 upstream ATTR_TOMBSTONE before merge",
				"upstream-entries", upstream,
				"local-entries", result.DiscardEntries)
		}

		newAttrs, rebuilt := message.ApplyAttrDiscard(pathAttrs, result.DiscardEntries)
		if rebuilt {
			// Path attributes section changed size — rebuild the full UPDATE body.
			// Save identifiers before replacing wu.
			oldCtxID := wu.SourceCtxID()
			oldSourceID := wu.SourceID()
			newBody := message.RebuildUpdateBody(body, newAttrs)
			wu = wireu.NewWireUpdate(newBody, oldCtxID)
			wu.SetSourceID(oldSourceID)
		}
		// If not rebuilt, pathAttrs (a slice of body) was modified in-place,
		// so wu.Payload() already reflects the change.

		return s.publishBase(wu), message.RFC7606ActionAttributeDiscard, nil

	case message.RFC7606ActionTreatAsWithdraw:
		// RFC 7606 Section 2: "MUST be handled as though all of the routes contained in an
		// UPDATE message ... had been withdrawn", "thus causing them to be removed from
		// the Adj-RIB-In".
		//
		// enforceRFC7606 only classifies and logs; processMessage synthesizes the
		// withdraw-only UPDATE(s) from this body and dispatches them, turning the announced
		// routes into withdrawals so the malformed UPDATE removes them instead of leaving a
		// previously-announced prefix installed and stale. The synthesis is deferred to the
		// caller because it is negotiation-aware (D-5: a non-negotiated MP family is skipped
		// rather than torn down) and may produce more than one UPDATE (D-8: RFC 7606 Section
		// 3.g allows only one MP_UNREACH per UPDATE, so two MP families ride two bodies),
		// neither of which fits this single-WireUpdate return.
		sessionLogger().Debug("RFC 7606 treat-as-withdraw",
			"attr", result.AttrCode,
			"description", result.Description)
		// RFC 7606 Section 6: logged on the UPDATE as the peer sent it -- the malformed one,
		// which is the whole point of the requirement.
		s.rfc7606Diagnostics("treat-as-withdraw", wu, result.AttrCode, result.Description)
		return wu, message.RFC7606ActionTreatAsWithdraw, nil

	case message.RFC7606ActionSessionReset:
		return s.rfc7606SessionReset(wu, result.Description)
	}

	return s.publishBase(wu), message.RFC7606ActionNone, nil
}

// publishBase builds the attribute span index over the bytes this UPDATE will be
// published with, on the receive goroutine, and returns the same WireUpdate.
//
// It is deliberately the LAST thing enforceRFC7606 does. Two branches above change the
// bytes after the RFC 7606 walk has read them, and an index built before either would
// describe an object nobody sees:
//
//   - the Section 3.g keep-first strip rebuilds the body and wraps it in a NEW WireUpdate,
//     shifting every attribute after the first stripped range;
//   - ApplyAttrDiscard's in-place branch overwrites the type-code byte with ATTR_TOMBSTONE
//     and builds no new WireUpdate at all, so the offsets survive but the code does not.
//
// wireu.WireUpdate.Attrs freezes the index on its first call, so this ordering is the whole
// guarantee. TestInPlaceDiscardPrecedesIndexBuild and TestStripRebuildIndexMatchesPublished
// pin it from the receive entry point.
//
// A build failure is NOT routed into an RFC 7606 action. The error is recorded on the base
// and returned by every accessor, which is exactly what the lazy builder did on first use,
// so no verdict changes. It is logged here because this is the one place that knows which
// peer sent the bytes.
func (s *Session) publishBase(wu *wireu.WireUpdate) *wireu.WireUpdate {
	attrs, err := wu.Attrs()
	if err != nil {
		sessionLogger().Debug("attribute index not built",
			"peer", s.settings.Address, "error", err)
		return wu
	}
	if attrs != nil && attrs.Spilled() && s.prefixMetrics != nil {
		s.prefixMetrics.attrSpanSpill.With(s.addrLabel).Inc()
	}
	return wu
}

// rfc7606Diagnostics logs the debugging facility RFC 7606 Section 6 requires.
//
// Section 6: "a BGP speaker must provide debugging facilities to permit issues caused by a
// malformed attribute to be diagnosed. At a minimum, such facilities must include logging
// an error listing the NLRI involved and containing the entire malformed UPDATE message
// when such an attribute is detected."
//
// Three deliberate choices:
//
//   - It is gated on the subsystem's Debug level and returns before building anything when
//     that level is off. slog evaluates its arguments eagerly, so an unguarded hex dump
//     would cost a full encode of every malformed UPDATE even with logging disabled --
//     which is exactly the amplification a hostile peer would aim for. "Debugging
//     facilities" is what the section asks for, and `ze.log.bgp.reactor.session=debug`
//     turns it on.
//   - The dump is the complete UPDATE body, untruncated, because the section says "the
//     entire malformed UPDATE message". The 19-octet header is omitted: it is a fixed
//     marker plus length and type, carries no diagnostic information, and wu.Payload() is
//     the body. The key name says body so the log does not overclaim.
//   - The IPv4 Withdrawn Routes and NLRI fields are decoded to prefixes as well as hexed,
//     since "listing the NLRI involved" is the point. MP-family NLRI lives inside the
//     attributes and is covered by the full body dump.
func (s *Session) rfc7606Diagnostics(event string, wu *wireu.WireUpdate, attrCode uint8, description string) {
	lg := sessionLogger()
	if !lg.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	body := wu.Payload()
	if len(body) < 4 {
		lg.Debug("RFC 7606 diagnostics",
			"event", event, "attr", attrCode, "description", description,
			"update-body-hex", textbuf.StringHex(body))
		return
	}

	recvCtx := bgpctx.Registry.Get(s.recvCtxID)
	addPath := recvCtx.AddPathFor(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast})

	var withdrawn, nlri []byte
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	if offset := 2 + withdrawnLen; offset+2 <= len(body) {
		withdrawn = body[2 : 2+withdrawnLen]
		attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
		if offset+2+attrLen <= len(body) {
			nlri = body[offset+2+attrLen:]
		}
	}

	lg.Debug("RFC 7606 diagnostics",
		"event", event,
		"attr", attrCode,
		"description", description,
		"withdrawn-prefixes", ipv4PrefixList(withdrawn, addPath),
		"nlri-prefixes", ipv4PrefixList(nlri, addPath),
		"update-body-hex", textbuf.StringHex(body))
}

// ipv4PrefixList renders an IPv4 unicast NLRI field as prefixes for the Section 6 log.
//
// Tolerant by design: this runs on input already known to be malformed, so a field that
// stops making sense is reported as far as it parsed rather than discarded. Returning
// nothing would defeat the point of the requirement.
func ipv4PrefixList(field []byte, addPath bool) []string {
	var out []string
	var tb textbuf.Buffer
	for pos := 0; pos < len(field); {
		if addPath {
			if pos+4 > len(field) {
				break
			}
			pos += 4 // RFC 7911 Path Identifier
		}
		if pos >= len(field) {
			break
		}
		bits := int(field[pos])
		pos++
		if bits > 32 {
			out = append(out, tb.Reset().Str("invalid-prefix-length/").Int(int64(bits)).String())
			break
		}
		octets := (bits + 7) / 8
		if pos+octets > len(field) {
			out = append(out, "truncated-prefix")
			break
		}
		var addr [4]byte
		copy(addr[:], field[pos:pos+octets])
		pos += octets
		out = append(out, textbuf.StringPrefix(netip.PrefixFrom(netip.AddrFrom4(addr), bits)))
	}
	return out
}

// rfc7606SessionReset performs the session-reset action: NOTIFICATION, FSM event, close.
//
// RFC 7606 Section 3 (a), which replaces RFC 4271 Section 6.3's first paragraph: "An
// error detected while processing the UPDATE message for which a session reset is
// specified MUST be indicated by sending the NOTIFICATION message with the Error Code
// UPDATE Message Error. The error subcode elaborates on the specific nature of the
// error."
//
// Every session-reset path routes through here, so the mandated NOTIFICATION cannot be
// skipped by a caller that returns the action directly.
func (s *Session) rfc7606SessionReset(wu *wireu.WireUpdate, description string) (*wireu.WireUpdate, message.RFC7606Action, error) {
	sessionLogger().Warn("RFC 7606 session-reset", "description", description)
	// RFC 7606 Section 6. A session reset is the most damaging outcome and the one an
	// operator most needs to diagnose, so it carries the same detail as the other two.
	s.rfc7606Diagnostics("session-reset", wu, 0, description)

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	s.logNotifyErr(conn,
		message.NotifyUpdateMessage,
		message.NotifyUpdateMalformedAttr,
		nil,
	)
	s.logFSMEvent(fsm.EventUpdateMsgErr)
	s.closeConn()

	return wu, message.RFC7606ActionSessionReset, fmt.Errorf("RFC 7606 session reset: %s", description)
}

// rfc7606NLRISyntaxAction enforces whatever action the NLRI syntax validator reported.
//
// RFC 7606 Section 5.3 makes a field "syntactically incorrect" when a prefix length
// exceeds the family maximum OR the last NLRI overruns the field. Section 3 (j) then
// requires session reset for either, because treat-as-withdraw is only available when
// "the entire NLRI field ... need[s] to be successfully parsed", and it cannot be.
//
// This used to flatten every syntax result to treat-as-withdraw regardless of the action
// the validator computed, silently downgrading a mandated session reset.
func (s *Session) rfc7606NLRISyntaxAction(
	wu *wireu.WireUpdate, result *message.RFC7606ValidationResult, field string,
) (*wireu.WireUpdate, message.RFC7606Action, error) {
	if result.Action == message.RFC7606ActionSessionReset {
		return s.rfc7606SessionReset(wu, result.Description)
	}
	sessionLogger().Debug("RFC 7606 NLRI syntax",
		"field", field,
		"action", result.Action,
		"description", result.Description)
	// RFC 7606 Section 6: the only enforcement outcome the facility would otherwise miss.
	// A Section 5.3 NLRI-syntax failure is not strictly "a malformed attribute", but it is
	// a malformed UPDATE the operator has to diagnose, and the NLRI is precisely what is
	// wrong with it.
	s.rfc7606Diagnostics("nlri-syntax", wu, result.AttrCode, result.Description)
	return wu, result.Action, nil
}

// mpFamilyDispatchable reports whether a synthesized withdrawal for an MP family may be
// dispatched to the RIB, i.e. whether the family would survive validateUpdateFamilies rather
// than trigger a strict-mode teardown.
//
// RFC 7606 treat-as-withdraw synthesis (message.SynthesizeWithdrawFamilies) uses it to skip
// a family the session never negotiated: that family has nothing in the RIB to withdraw, and
// re-deriving a teardown from a malformed UPDATE would be a new behavior the pre-synthesis
// drop never had (D-5). The accept condition mirrors validateUpdateFamilies exactly, so a
// family this admits also passes that check on the synthesized body.
func (s *Session) mpFamilyDispatchable(afi uint16, safi uint8) bool {
	neg := s.Negotiated()
	if neg == nil {
		return true
	}
	fam := capability.Family{AFI: capability.AFI(afi), SAFI: capability.SAFI(safi)}
	if neg.SupportsFamily(fam) {
		return true
	}
	return s.settings.IgnoreFamilyMismatch || s.shouldIgnoreFamily(fam)
}

// validateUpdateFamilies checks that AFI/SAFI in MP_REACH/MP_UNREACH were negotiated.
// RFC 4760 Section 6: "If a BGP speaker receives an UPDATE with MP_REACH_NLRI or
// MP_UNREACH_NLRI where the AFI/SAFI do not match those negotiated in OPEN,
// the speaker MAY treat this as an error.".
func (s *Session) validateUpdateFamilies(body []byte) error {
	// Need at least 4 bytes: withdrawn len (2) + attrs len (2)
	if len(body) < 4 {
		return nil // Let message parsing handle malformed
	}

	// Skip withdrawn routes
	withdrawnLen := binary.BigEndian.Uint16(body[0:2])
	offset := 2 + int(withdrawnLen)
	if offset+2 > len(body) {
		return nil
	}

	// Get path attributes
	attrLen := binary.BigEndian.Uint16(body[offset : offset+2])
	offset += 2
	if offset+int(attrLen) > len(body) {
		return nil
	}
	pathAttrs := body[offset : offset+int(attrLen)]

	// Parse path attributes looking for MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15)
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			break
		}

		flags := pathAttrs[pos]
		code := attribute.AttributeCode(pathAttrs[pos+1])
		pos += 2

		// Determine length (1 or 2 bytes based on extended length flag)
		var attrDataLen int
		if flags&0x10 != 0 { // Extended length
			if pos+2 > len(pathAttrs) {
				break
			}
			attrDataLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				break
			}
			attrDataLen = int(pathAttrs[pos])
			pos++
		}

		if pos+attrDataLen > len(pathAttrs) {
			break
		}

		attrData := pathAttrs[pos : pos+attrDataLen]
		pos += attrDataLen

		// Check MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15)
		if code == attribute.AttrMPReachNLRI || code == attribute.AttrMPUnreachNLRI {
			if len(attrData) < 3 {
				continue // Malformed, let other validation catch it
			}

			afi := capability.AFI(binary.BigEndian.Uint16(attrData[0:2]))
			safi := capability.SAFI(attrData[2])
			fam := capability.Family{AFI: afi, SAFI: safi}

			neg := s.Negotiated()
			if neg != nil && !neg.SupportsFamily(fam) {
				// Family not negotiated - check if we should ignore
				shouldIgnore := s.settings.IgnoreFamilyMismatch || s.shouldIgnoreFamily(fam)
				if shouldIgnore {
					// Lenient mode: log warning and skip
					sessionLogger().Debug("UPDATE family mismatch ignored", "afi", afi, "safi", safi)
				} else {
					// Strict mode: return error
					sessionLogger().Debug("UPDATE family mismatch rejected", "afi", afi, "safi", safi)
					return fmt.Errorf("%w: %s", ErrFamilyNotNegotiated, fam)
				}
			}
		}
	}

	return nil
}

// validateCapabilityModes checks required/refused capability codes against the negotiated result.
// Sends NOTIFICATION and tears down the session if any violation is found.
// RFC 5492 Section 3: Unsupported Capability subcode.
func (s *Session) validateCapabilityModes(conn net.Conn, neg *capability.Negotiated, required, refused []capability.Code) error {
	if len(required) > 0 && neg != nil {
		if missing := neg.CheckRequiredCodes(required); len(missing) > 0 {
			capData := buildUnsupportedCapabilityDataCodes(missing)
			s.logNotifyErr(conn,
				message.NotifyOpenMessage,
				message.NotifyOpenUnsupportedCapability,
				capData,
			)
			s.logFSMEvent(fsm.EventBGPOpenMsgErr)
			s.closeConn()
			return fmt.Errorf("%w: required capabilities not negotiated: %v", ErrInvalidState, missing)
		}
	}

	if len(refused) > 0 && neg != nil {
		if present := neg.CheckRefusedCodes(refused); len(present) > 0 {
			capData := buildUnsupportedCapabilityDataCodes(present)
			s.logNotifyErr(conn,
				message.NotifyOpenMessage,
				message.NotifyOpenUnsupportedCapability,
				capData,
			)
			s.logFSMEvent(fsm.EventBGPOpenMsgErr)
			s.closeConn()
			return fmt.Errorf("%w: refused capabilities present in peer OPEN: %v", ErrInvalidState, present)
		}
	}

	return nil
}

// validateAddPathFamilyModes checks per-family ADD-PATH required/refused against negotiation.
func (s *Session) validateAddPathFamilyModes(conn net.Conn, neg *capability.Negotiated, required, refused []capability.Family) error {
	if neg == nil {
		return nil
	}

	for _, f := range required {
		if neg.AddPathMode(f) != capability.AddPathNone {
			continue
		}
		capData := buildUnsupportedCapabilityData([]capability.Family{f})
		s.logNotifyErr(conn, message.NotifyOpenMessage, message.NotifyOpenUnsupportedCapability, capData)
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("%w: required ADD-PATH family not negotiated: %s", ErrInvalidState, f)
	}

	for _, f := range refused {
		if neg.AddPathMode(f) == capability.AddPathNone {
			continue
		}
		capData := buildUnsupportedCapabilityData([]capability.Family{f})
		s.logNotifyErr(conn, message.NotifyOpenMessage, message.NotifyOpenUnsupportedCapability, capData)
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("%w: refused ADD-PATH family present in peer OPEN: %s", ErrInvalidState, f)
	}

	return nil
}

// buildUnsupportedCapabilityData builds NOTIFICATION data for Unsupported Capability.
//
// RFC 5492 Section 3: The Data field contains one or more capability tuples.
// For Multiprotocol (code 1): AFI (2 bytes) + Reserved (1 byte) + SAFI (1 byte).
func buildUnsupportedCapabilityData(families []capability.Family) []byte {
	// Each Multiprotocol capability: code (1) + length (1) + AFI (2) + Reserved (1) + SAFI (1) = 6 bytes
	data := make([]byte, len(families)*6)
	offset := 0
	for _, f := range families {
		data[offset] = byte(capability.CodeMultiprotocol) // Capability code
		data[offset+1] = 4                                // Capability length
		binary.BigEndian.PutUint16(data[offset+2:], uint16(f.AFI))
		data[offset+4] = 0 // Reserved
		data[offset+5] = byte(f.SAFI)
		offset += 6
	}
	return data
}

// buildUnsupportedCapabilityDataCodes builds NOTIFICATION data for non-family capability codes.
//
// RFC 5492 Section 3: Each capability is encoded as code (1 byte) + length (1 byte).
// For refused/required non-Multiprotocol codes, length is 0 (no capability-specific data needed).
func buildUnsupportedCapabilityDataCodes(codes []capability.Code) []byte {
	if len(codes) == 0 {
		return nil
	}
	// Each code: capability code (1 byte) + length (1 byte) = 2 bytes
	data := make([]byte, len(codes)*2)
	for i, c := range codes {
		data[i*2] = byte(c)
		data[i*2+1] = 0 // length=0: no capability-specific value
	}
	return data
}
