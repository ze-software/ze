// Design: docs/architecture/core-design.md — RFC 7606 UPDATE validation
// Overview: session.go — BGP session struct and lifecycle
// RFC: rfc/short/rfc7606.md — revised UPDATE error handling
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md — first-AS receive validation

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
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
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
// Transitive bit derived here (Section 4.2) is what every peer sees. Section
// 5.3's EBGP-boundary clear is NOT performed: Thomas removed that support on
// 2026-09-09, along with wireu.rewriteASPathPrepend, the unreached path that had
// been its only implementation (docs/architecture/wire/attributes.md).
// Direct validation callers use this entry point. Both readers share its
// classification and applyRFC7606 before dispatching UPDATEs to plugins.
func (s *Session) enforceRFC7606(wu *wireu.WireUpdate) (*wireu.WireUpdate, message.RFC7606Action, error) {
	// RFC 7606 Sections 3 and 5.3: direct callers classify once before enforcement.
	validation := s.classifyRFC7606(wu.Payload())
	out, action, err := s.applyRFC7606(wu, &validation)
	if err == nil && action < message.RFC7606ActionTreatAsWithdraw {
		out = s.publishBase(out)
	}
	return out, action, err
}

// applyRFC7606 consumes the shared receive verdict without walking it again.
// RFC 7606 Section 3(h): "Otherwise, the approach with the strongest action MUST be used."
// The caller MUST supply a classification of these bytes, or a clean batch of
// identical attributes whose individual NLRI fields have already been validated.
func (s *Session) applyRFC7606(wu *wireu.WireUpdate, validation *rfc7606Validation) (*wireu.WireUpdate, message.RFC7606Action, error) {
	result := validation.result
	if validation.syntaxField != "" {
		// RFC 7606 Sections 3(j) and 5.3.
		return s.rfc7606NLRISyntaxAction(wu, result, validation.syntaxField)
	}
	if result.Action == message.RFC7606ActionSessionReset {
		// RFC 7606 Section 3(a).
		if result.Notification != nil {
			return s.rfc7606ResetNotification(wu, result.Description, result.Notification, "")
		}
		return s.rfc7606SessionReset(wu, result.Description, "")
	}
	body := wu.Payload()

	// RFC 7606 Section 6 asks for "an error listing the NLRI involved and containing the
	// entire malformed UPDATE message". That is the message the PEER sent, and two branches
	// below replace wu with a rebuilt body before the diagnostics run: the Section 3.g
	// keep-first strip and the Section 5.4 typed-NLRI discard. Hold the received one so the
	// log reports what arrived rather than what ze made of it.
	receivedWU := wu

	offset := validation.attrsOffset
	pathAttrs := body[offset : offset+validation.attrsLen]
	hasNLRI := validation.hasNLRI

	recvCtx := bgpctx.Registry.Get(s.recvCtxID)
	addPathFor := func(afi uint16, safi uint8) bool {
		return recvCtx.AddPathFor(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)})
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

	// RFC 9012 Section 13; RFC 9830 Sections 2.2 and 2.3. Attribute ranges
	// MUST be deduplicated first, and both returned values MUST be used so
	// later discards edit the published body rather than the received bytes.
	wu, pathAttrs = applyTunnelEncap(wu, pathAttrs, hasNLRI, result)
	body = wu.Payload()

	// RFC 7606 Section 5.4: discard routes whose NLRI type ze does not implement, in
	// families whose own specification has not overridden that rule. RFC 9552 Section 8.2.2:
	// discard a Link-State NLRI whose syntax is malformed in a way ze can skip past. Runs on
	// the bytes the Section 3.g strip left, so the rewrites compose in one direction only.
	//
	// Skipped only for session-reset, where the UPDATE is not processed at all.
	//
	// It is NOT skipped for treat-as-withdraw, though that reads like a case where no route
	// survives to discard. Treat-as-withdraw keeps the NLRI: SynthesizeWithdrawFamilies
	// (message/rfc7606_withdraw.go) turns this UPDATE's MP_REACH into an MP_UNREACH carrying
	// the SAME NLRI bytes, and processMessage dispatches it to every eligible peer. Skipping
	// the filter here therefore relayed the unrecognized types inside a withdrawal, which is
	// the wire-visible half of Section 5.4 the filter exists to close, and it made the
	// symmetry the design claims for withdrawals untrue on this one path.
	if result.Action != message.RFC7606ActionSessionReset {
		var outcome typedNLRIOutcome
		wu, pathAttrs, outcome = s.applyTypedNLRIDiscard(
			wu, pathAttrs, offset, result.MPReachNLRI, result.MPUnreachNLRI, addPathFor)
		body = wu.Payload()
		switch outcome {
		case typedNLRIUnparseable:
			// RFC 7606 Section 5.3 makes an MP attribute incorrect when the last NLRI in it
			// overruns the attribute, and Section 3(j) requires session reset when the NLRI
			// field cannot be parsed, because treat-as-withdraw needs it parsed. Same verdict
			// the validator already reaches for an IPv4 or IPv6 unicast MP NLRI; typed
			// families reach it here because their framing walk is the family's splitter.
			return s.rfc7606SessionReset(receivedWU,
				"RFC 7606 Section 5.3: MP NLRI overruns the attribute; Section 3(j) requires session reset", "")
		case typedNLRIEmptied:
			// Section 5.4 discarded every route the UPDATE carried. What is left encodes no
			// reachability, in either of the two shapes applyTypedNLRIDiscard names: a body
			// that IS the RFC 4724 Section 2 End-of-RIB marker, or attributes with no NLRI at
			// all. Neither may be relayed on the peer's behalf.
			//
			// Treat-as-withdraw is the rail that drops it: SynthesizeWithdrawFamilies finds no
			// withdrawn routes, no NLRI and no MP_UNREACH family in this body and returns no
			// bodies, which processMessage already treats as "consume the UPDATE, restart the
			// HoldTimer per RFC 4271 Section 8.2.2 Event 27, dispatch nothing". Nothing is
			// installed, nothing is forwarded, and no EOR is counted for a peer that sent none.
			// Debug for the same reason as the sibling line in typedNLRIEdit: a peer decides
			// how often it fires, one line per UPDATE, on the receive goroutine.
			sessionLogger().Debug("RFC 7606 Section 5.4: every route discarded, UPDATE not relayed",
				"peer", s.settings.Address)
			// This return jumps the action switch, so the Section 6 log that switch would
			// have emitted has to happen here. The UPDATE can be malformed AND emptied at
			// once, and the operator whose route stopped arriving needs the attribute and
			// the description, which the line above carries neither of.
			if result.Action != message.RFC7606ActionNone {
				s.rfc7606Diagnostics(result.Action.String(), receivedWU, result.AttrCode, result.Description)
			}
			return wu, message.RFC7606ActionTreatAsWithdraw, nil
		case typedNLRIKept:
		default:
			panic("BUG: invalid typed NLRI outcome")
		}
	}

	switch result.Action {
	case message.RFC7606ActionNone:
		return wu, message.RFC7606ActionNone, nil

	case message.RFC7606ActionAttributeDiscard:
		// RFC 7606 Section 2: "The attribute MUST be discarded ... and the UPDATE
		// message continues to be processed."
		// draft-mangin-idr-attr-tombstone-00 Section 5.1: Apply ATTR_TOMBSTONE markers.
		sessionLogger().Debug("RFC 7606 attribute-discard",
			"attr", result.AttrCode,
			"discard-entries", result.DiscardEntries,
			"description", result.Description)
		// RFC 7606 Section 6: the NLRI involved and the entire malformed UPDATE, as the peer
		// sent it. Not wu: the Section 3.g and Section 5.4 rewrites above have already
		// replaced that with bytes ze built.
		validation.diagnosticWireHex = s.rfc7606Diagnostics("attribute-discard", receivedWU, result.AttrCode, result.Description)

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

		return wu, message.RFC7606ActionAttributeDiscard, nil

	case message.RFC7606ActionTreatAsWithdraw:
		// RFC 7606 Section 2: "MUST be handled as though all of the routes contained in an
		// UPDATE message ... had been withdrawn", "thus causing them to be removed from
		// the Adj-RIB-In".
		//
		// applyRFC7606 applies the verdict and logs; processValidatedMessage synthesizes the
		// withdraw-only UPDATE(s) from this body and dispatches them, turning the announced
		// routes into withdrawals so the malformed UPDATE removes them instead of leaving a
		// previously-announced prefix installed and stale. The synthesis is deferred to the
		// caller because it is negotiation-aware (D-5: a non-negotiated MP family is skipped
		// rather than torn down) and may produce more than one UPDATE (D-8: RFC 7606 Section
		// 3.g allows only one MP_UNREACH per UPDATE, so two MP families ride two bodies),
		// neither of which fits this single-WireUpdate return.
		//
		// The body handed on is the one the Section 5.4 filter above left, so the synthesized
		// withdrawal names only route types ze implements.
		sessionLogger().Debug("RFC 7606 treat-as-withdraw",
			"attr", result.AttrCode,
			"description", result.Description)
		// RFC 7606 Section 6: logged on the UPDATE as the peer sent it -- the malformed one,
		// which is the whole point of the requirement. Not wu, which the rewrites above may
		// have replaced with bytes ze built.
		s.rfc7606Diagnostics("treat-as-withdraw", receivedWU, result.AttrCode, result.Description)
		return wu, message.RFC7606ActionTreatAsWithdraw, nil

	case message.RFC7606ActionSessionReset:
		// Carrier-aware validation may raise this action after keep-first
		// rebuilding. RFC 7606 Section 6 still requires the received message.
		if result.Notification != nil {
			return s.rfc7606ResetNotification(receivedWU, result.Description, result.Notification, "")
		}
		return s.rfc7606SessionReset(receivedWU, result.Description, "")
	default:
		panic("BUG: invalid RFC 7606 action")
	}
}

// firstASMismatch checks a validated, reconstructed path, not the peer's AS_TRANS
// placeholders. The caller MUST finish RFC 7606 validation and AS4 reconstruction
// first and MUST synthesize withdrawals on a mismatch.
//
// Draft-ietf-sidrops-aspa-verification-28 Section 5.1 describes the neighbor-AS
// prerequisite and refers its error handling to RFC 4271 and RFC 7606.
// Section 5.5 exempts the receiving RS-client; the route server itself still
// checks paths received from its clients.
func (s *Session) firstASMismatch(wu *wireu.WireUpdate) bool {
	peerAS, check := s.firstASPeer()
	if !check {
		return false
	}

	attrs, err := wu.Attrs()
	if err != nil {
		return true
	}
	if attrs == nil {
		return false // A withdrawal or End-of-RIB carries no AS_PATH to check.
	}
	nlri, err := wu.NLRI()
	if err != nil {
		return true
	}
	if len(nlri) == 0 {
		mpReach, err := attrs.GetRaw(attribute.AttrMPReachNLRI)
		if err != nil {
			return true
		}
		if mpReach == nil {
			return false
		}
	}
	path, err := attrs.GetRaw(attribute.AttrASPath)
	if err != nil {
		return true
	}

	// RFC 4271 Section 4.3 / RFC 6793 Section 3:
	// AS_PATH value: type[0], count[1], first ASN[2:4] or [2:6], ...
	// An empty path or a leading set supplies no most-recent AS_SEQUENCE hop.
	asn4 := false
	if ctx := bgpctx.Registry.Get(wu.SourceCtxID()); ctx != nil {
		asn4 = ctx.ASN4()
	} else if neg := s.Negotiated(); neg != nil {
		asn4 = neg.ASN4
	}
	return firstASPathMismatch(path, asn4, peerAS)
}

// firstASPeer preserves the configured neighbor check and RS-client exemption.
// RFC 7606 Section 7.2 applies only "if the local system is configured to do so".
func (s *Session) firstASPeer() (uint32, bool) {
	peerAS := s.settings.PeerAS
	if peerAS == 0 {
		if neg := s.Negotiated(); neg != nil {
			peerAS = neg.PeerASN
		}
	}
	return peerAS, !s.settings.isIBGPWith(peerAS) && !s.localRSClient
}

// firstASPathMismatch compares the existing most-recent AS_SEQUENCE rule.
// RFC 7606 Section 7.2 requires the leftmost path AS to equal the sending peer.
func firstASPathMismatch(path []byte, asn4 bool, peerAS uint32) bool {
	if len(path) < 4 || path[0] != byte(attribute.ASSequence) || path[1] == 0 {
		return true
	}
	if asn4 {
		if len(path) < 6 {
			return true
		}
		return binary.BigEndian.Uint32(path[2:6]) != peerAS
	}
	return uint32(binary.BigEndian.Uint16(path[2:4])) != peerAS
}

// publishBase stamps RFC 4271 Section 9's Partial bit and builds the attribute span index
// over the bytes this UPDATE will be published with, on the receive goroutine, and returns
// the same WireUpdate.
//
// It runs after RFC 7606 enforcement and first-AS checks, before publication.
// Direct enforcement callers publish when returning. Two enforcement branches
// change bytes after the validation walk, and an index built before either
// would describe an object nobody sees:
//
//   - the Section 3.g keep-first strip rebuilds the body and wraps it in a NEW WireUpdate,
//     shifting every attribute after the first stripped range;
//   - ApplyAttrDiscard's in-place branch overwrites the type-code byte with ATTR_TOMBSTONE
//     and builds no new WireUpdate at all, so the offsets survive but the code does not.
//
// wireu.WireUpdate.Attrs freezes the index on its first call, so these code/offset
// rewrites must precede semantic access. Partial changes only flags, which spans
// read from the payload. TestInPlaceDiscardPrecedesIndexBuild and
// TestStripRebuildIndexMatchesPublished pin the enforcement ordering.
//
// Four branches return without calling it, and each returns an UPDATE nobody publishes: the
// two session resets, the Section 5.4 branch whose UPDATE conveys nothing and is dropped, and
// treat-as-withdraw, whose body processMessage re-synthesizes into withdrawals rather than
// dispatching. An index over bytes no consumer reads would cost a walk and describe nothing.
//
// A build failure is NOT routed into an RFC 7606 action. The error is recorded on the base
// and returned by every accessor, which is exactly what the lazy builder did on first use,
// so no verdict changes. It is logged here because this is the one place that knows which
// peer sent the bytes.
func (s *Session) publishBase(wu *wireu.WireUpdate) *wireu.WireUpdate {
	// RFC 4271 Section 9: "If an optional transitive attribute is unrecognized, the
	// Partial bit (the third high-order bit) in the attribute flags octet is set to 1,
	// and the attribute is retained for propagation to other BGP speakers."
	// Ordinary sessions normalize before publication so stored routes and every
	// forward rail share the result. Route-server client sessions instead retain
	// the optional attributes for RFC 7947 Section 2.2 transparency.
	//
	// It runs only on the paths that PUBLISH, which is why it lives here rather than
	// beside the RFC 7606 walk: an UPDATE ze session-resets or turns into withdrawals
	// propagates nothing, and RFC 7606 Section 6 requires its diagnostics to dump the
	// message the PEER sent.
	//
	// The section comes from the payload rather than from wu.Attrs(), because that call
	// FREEZES the index and the index must describe the bytes ze publishes. A payload
	// that does not parse into sections stamps nothing and is reported by the wu.Attrs()
	// call below, which fails on the same input.
	if sections, secErr := wire.ParseUpdateSections(wu.Payload()); secErr == nil {
		attrs := sections.Attrs(wu.Payload())
		// RFC 7947 Section 2.2: "Optional recognized and unrecognized BGP
		// attributes, whether transitive or non-transitive, SHOULD NOT be
		// updated by the route server (unless enforced by local IXP operator
		// configuration) and SHOULD be passed on to other route server clients."
		if !s.settings.RSClient {
			attribute.SetPartialOnUnrecognizedTransitive(attrs)
		}

		// RFC 4271 Section 4.3, the requirement the stamp above does not answer:
		// "For well-known attributes and for optional non-transitive attributes,
		// the Partial bit MUST be set to 0." RFC 7606 Section 3(c) is why such an
		// octet reaches ze at all -- it narrows the receive check to "the value of
		// either the Optional or Transitive bits", so an ORIGIN arriving as 0x60 is
		// not malformed and is not withdrawn. Without this clear, the route-server
		// and stored-route relays copy that octet onward byte for byte
		// (relay_payload.go, writeRelayPayload) and ze puts it back on the wire.
		//
		// It belongs here for the same reason the stamp does: these bytes are the
		// bytes the RIB retains, the bytes a route server relays zero-copy, and the
		// bytes every rebuild copies the untouched attributes out of.
		//
		// The clear and the stamp act on disjoint classes -- an attribute that is
		// both Optional and Transitive is the stamp's alone -- so no ordering of
		// the two can undo the other, and the Section 5 bit a previous AS set on an
		// optional transitive attribute survives both.
		attribute.ClearPartialOnWellKnownAndNonTransitive(attrs)

		// RFC 4271 Section 5, the other half of the same sentence: "Unrecognized
		// non-transitive optional attributes MUST be quietly ignored and not passed
		// along to other BGP peers." Section 9 repeats it. Without this ze relays a
		// peer's private non-transitive attributes to third parties, which is the one
		// thing the Transitive bit exists to prevent.
		//
		// Stamping runs first and the two never touch the same attribute: one acts on
		// optional TRANSITIVE codes, this one on optional non-transitive codes.
		//
		// Removing an attribute shortens the section, so unlike the stamp this cannot
		// be done in place. It takes the same route the Section 3.g duplicate strip
		// above takes, and allocates only when a peer actually sent one.
		if !s.settings.RSClient {
			if ranges := message.UnrecognizedNonTransitiveRanges(attrs); len(ranges) > 0 {
				stripped := message.StripAttrRanges(attrs, ranges)
				rebuilt := wireu.NewWireUpdate(message.RebuildUpdateBody(wu.Payload(), stripped), wu.SourceCtxID())
				rebuilt.SetSourceID(wu.SourceID())
				wu = rebuilt
				sessionLogger().Debug("RFC 4271 Section 5: dropped unrecognized non-transitive attributes",
					"peer", s.settings.Address, "count", len(ranges))
			}
		}
	}

	// RFC 8092 Section 3, applied at this site for the reason the strips above are:
	// the deduplicated attribute is what the RIB, the relays and every rebuild see.
	wu = removeRedundantLargeCommunities(wu)

	// RFC 8669 Section 6, RFC 9252 Section 7, applied here for the same reason:
	// the RIB, the relays and the JSON encoder all read the first TLV alone.
	// Route-server clients included, as with the RFC 8092 removal above and the
	// RFC 7606 Section 3.g strip: the discard is the RFC's own receive rule, not
	// an attribute update RFC 7947 Section 2.2 asks a route server to avoid.
	wu = discardRepeatedPrefixSIDTLVs(wu, s.settings.Address)

	// RFC 8277 Section 2.2, applied at this site for the same reason: the label
	// fields the RIB keeps and the relays copy carry a zero Rsrv.
	wu = clearLabelRsrv(wu, bgpctx.Registry.Get(s.recvCtxID).AddPathFor)

	wu = discardNonVPNAcceptOwn(wu)

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
// Debug-disabled calls return before building any text. Readers validate the BGP
// header before this facility runs, so its marker, length and UPDATE type can be
// reproduced exactly from the original body without retaining another buffer.
// Coalescing MUST classify original messages before merging them: an erroneous
// message is diagnosed separately, never as a synthetic batch.
func (s *Session) rfc7606Diagnostics(event string, wu *wireu.WireUpdate, attrCode uint8, description string) string {
	return s.rfc7606DiagnosticsWithWire(event, wu, attrCode, description, "")
}

// rfc7606DiagnosticsWithWire reuses the original hex already logged before an
// in-place attribute discard. A later first-AS error must not dump tombstones
// as though the peer had sent them. RFC 7606 Section 6 requires the entire
// malformed UPDATE, not its rewritten representation.
func (s *Session) rfc7606DiagnosticsWithWire(event string, wu *wireu.WireUpdate, attrCode uint8, description, wireHex string) string {
	lg := sessionLogger()
	if !lg.Enabled(context.Background(), slog.LevelDebug) {
		return ""
	}
	body := wu.Payload()
	if wireHex == "" {
		var header [message.HeaderLen]byte
		hdr := message.Header{Length: uint16(message.HeaderLen + len(body)), Type: msgtype.TypeUPDATE}
		hdr.WriteTo(header[:], 0)
		var dump textbuf.Buffer
		wireHex = dump.Hex(header[:]).Hex(body).String()
	}

	recvCtx := bgpctx.Registry.Get(s.recvCtxID)
	addPath := recvCtx.AddPathFor(family.IPv4Unicast)
	var withdrawn, nlri, attrs []byte
	if len(body) >= 4 {
		withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
		if offset := 2 + withdrawnLen; offset+2 <= len(body) {
			withdrawn = body[2 : 2+withdrawnLen]
			attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
			if offset+2+attrLen <= len(body) {
				attrs = body[offset+2 : offset+2+attrLen]
				nlri = body[offset+2+attrLen:]
			}
		}
	}

	var mpReach, mpUnreach []string
	iter := attribute.NewAttrIterator(attrs)
	for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
		//exhaustive:ignore // Only MP attributes contain NLRI.
		switch code {
		case attribute.AttrMPReachNLRI:
			mpReach = append(mpReach, mpDiagnosticNLRI(code, value, recvCtx)...)
		case attribute.AttrMPUnreachNLRI:
			mpUnreach = append(mpUnreach, mpDiagnosticNLRI(code, value, recvCtx)...)
		}
	}
	wireKey, representation := "update-wire-hex", "original"
	if event == "invalid-next-hop" {
		// RFC 4271 Section 6.3 local/subnet policy is not RFC 7606
		// Section 7.3's malformed-length case. It may run on a clean batch.
		wireKey, representation = "update-processed-hex", "processed"
	}
	lg.Debug("RFC 7606 diagnostics",
		"event", event,
		"attr", attrCode,
		"description", description,
		"representation", representation,
		"withdrawn-prefixes", ipv4PrefixList(withdrawn, addPath),
		"nlri-prefixes", ipv4PrefixList(nlri, addPath),
		"mp-reach-prefixes", mpReach,
		"mp-unreach-prefixes", mpUnreach,
		wireKey, wireHex)
	return wireHex
}

// mpDiagnosticNLRI lists every framed MP NLRI, including unknown typed routes.
// RFC 7606 Section 6: "At a minimum, such facilities must include logging
// an error listing the NLRI involved and containing the entire
// malformed UPDATE message when such an attribute is detected."
func mpDiagnosticNLRI(code attribute.AttributeCode, value []byte, ctx *bgpctx.EncodingContext) []string {
	start, ok := message.MPNLRIStart(uint8(code), value)
	if !ok {
		var tb textbuf.Buffer
		return []string{tb.Str("unreadable-mp-header/").Hex(value).String()}
	}
	fam := family.Family{AFI: family.AFI(binary.BigEndian.Uint16(value[:2])), SAFI: family.SAFI(value[2])}
	addPath := ctx.AddPathFor(fam)
	field := value[start:]
	if fam.SAFI == family.SAFIUnicast || fam.SAFI == family.SAFIMulticast {
		if fam.AFI == family.AFIIPv4 {
			return ipDiagnosticPrefixList(field, addPath, false)
		}
		if fam.AFI == family.AFIIPv6 {
			return ipDiagnosticPrefixList(field, addPath, true)
		}
	}
	split := nlrisplit.Get(fam)
	if code == attribute.AttrMPUnreachNLRI {
		split = nlrisplit.GetWithdraw(fam)
	}
	var out []string
	var tb textbuf.Buffer
	consumed := 0
	if split != nil {
		_, err := split(field, addPath, func(one []byte) bool {
			out = append(out, tb.Reset().Str("afi=").Int(int64(fam.AFI)).
				Str("/safi=").Int(int64(fam.SAFI)).Str("/nlri=").Hex(one).String())
			consumed += len(one)
			return true
		})
		if err == nil {
			return out
		}
	}
	out = append(out, tb.Reset().Str("afi=").Int(int64(fam.AFI)).
		Str("/safi=").Int(int64(fam.SAFI)).Str("/unreadable-nlri=").Hex(field[consumed:]).String())
	return out
}

// ipv4PrefixList renders an IPv4 unicast NLRI field as prefixes for the Section 6 log.
//
// Tolerant by design: this runs on input already known to be malformed, so a field that
// stops making sense is reported as far as it parsed rather than discarded. Returning
// nothing would defeat the point of the requirement.
func ipv4PrefixList(field []byte, addPath bool) []string {
	return ipDiagnosticPrefixList(field, addPath, false)
}

// ipDiagnosticPrefixList formats prefix-framed NLRI without hiding a malformed tail.
// RFC 7606 Section 6: "logging an error listing the NLRI involved".
func ipDiagnosticPrefixList(field []byte, addPath, ipv6 bool) []string {
	var out []string
	var tb textbuf.Buffer
	maxBits := 32
	if ipv6 {
		maxBits = 128
	}
	for pos := 0; pos < len(field); {
		if addPath {
			if pos+4 > len(field) {
				out = append(out, "truncated-path-identifier")
				break
			}
			pos += 4 // RFC 7911 Path Identifier
		}
		if pos >= len(field) {
			out = append(out, "missing-prefix")
			break
		}
		bits := int(field[pos])
		pos++
		if bits > maxBits {
			out = append(out, tb.Reset().Str("invalid-prefix-length/").Int(int64(bits)).String())
			break
		}
		octets := (bits + 7) / 8
		if pos+octets > len(field) {
			out = append(out, "truncated-prefix")
			break
		}
		if ipv6 {
			var addr [16]byte
			copy(addr[:], field[pos:pos+octets])
			out = append(out, textbuf.StringPrefix(netip.PrefixFrom(netip.AddrFrom16(addr), bits)))
		} else {
			var addr [4]byte
			copy(addr[:], field[pos:pos+octets])
			out = append(out, textbuf.StringPrefix(netip.PrefixFrom(netip.AddrFrom4(addr), bits)))
		}
		pos += octets
	}
	return out
}

// rfc7606LateAttributeError applies the common error boundary after semantic
// checks of non-MP_UNREACH attributes. It uses the original validator's fact,
// not the NLRI remaining after typed-route or attribute rewrites.
//
// RFC 7606 Section 5.2: if the UPDATE "doesn't encode any reachable NLRI" and
// "any encountered error specifies an error-handling approach other than
// 'attribute discard', then the 'session reset' approach MUST be used."
// The caller supplies a genuine late error, never Section 5.4's synthetic
// treat-as-withdraw action used solely to drop an emptied typed UPDATE.
func (s *Session) rfc7606LateAttributeError(wu *wireu.WireUpdate, validation *rfc7606Validation, attrCode uint8, description string) (message.RFC7606Action, error) {
	if !validation.result.HasReachableNLRI {
		_, action, err := s.rfc7606SessionReset(wu,
			"RFC 7606 Section 5.2: "+description+" (escalated -- attrs with no NLRI)",
			validation.diagnosticWireHex)
		return action, err
	}
	s.rfc7606DiagnosticsWithWire("treat-as-withdraw", wu, attrCode, description, validation.diagnosticWireHex)
	return message.RFC7606ActionTreatAsWithdraw, nil
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
func (s *Session) rfc7606SessionReset(wu *wireu.WireUpdate, description, wireHex string) (*wireu.WireUpdate, message.RFC7606Action, error) {
	return s.rfc7606ResetNotification(wu, description, &message.Notification{
		ErrorCode:    message.NotifyUpdateMessage,
		ErrorSubcode: message.NotifyUpdateMalformedAttr,
	}, wireHex)
}

func (s *Session) rfc7606ResetNotification(wu *wireu.WireUpdate, description string, notification *message.Notification, wireHex string) (*wireu.WireUpdate, message.RFC7606Action, error) {
	sessionLogger().Warn("RFC 7606 session-reset", "description", description)
	// RFC 7606 Section 6. A session reset is the most damaging outcome and the one an
	// operator most needs to diagnose, so it carries the same detail as the other two.
	s.rfc7606DiagnosticsWithWire("session-reset", wu, 0, description, wireHex)

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	s.logNotifyErr(conn, notification.ErrorCode, notification.ErrorSubcode, notification.Data)
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
		// RFC 4271 Section 6.3: "If the field is syntactically incorrect,
		// then the Error Subcode MUST be set to Invalid Network Field."
		return s.rfc7606ResetNotification(wu, result.Description, &message.Notification{
			ErrorCode:    message.NotifyUpdateMessage,
			ErrorSubcode: message.NotifyUpdateInvalidNetwork,
		}, "")
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
//
// RFC 4760 Section 6: "An implementation MAY support all, some, or none of the
// Subsequent Address Family Identifier values defined in this document." So an UPDATE
// can name an <AFI, SAFI> this session never negotiated, and Section 7 says what the
// receiver owes for an attribute carrying one: "if the speaker determines that the
// attribute is incorrect, the speaker MUST delete all the BGP routes received from that
// neighbor whose AFI/SAFI is the same as the one carried in the incorrect MP_REACH_NLRI
// or MP_UNREACH_NLRI attribute."
//
// Strict mode returns the error that drives that refusal. IgnoreFamilyMismatch and
// IgnoreFamilies ask for the lenient reading of "determines that the attribute is
// incorrect" -- the family is unsupported rather than the encoding malformed -- and take
// Section 7's other two remedies instead of the session: the UPDATE is dropped, so no
// route of that AFI/SAFI is taken, and the session survives.
//
// Dropping is what the option NAMES and what the RFC's MUST asks for. Until 2026-09-03
// the lenient branch logged and let the whole UPDATE through, so an operator who wrote
// `family <afi/safi> { mode ignore; }` got the unnegotiated NLRI installed in the RIB and
// forwarded, which is the one outcome Section 7 forbids. The comment here, the doc on
// PeerSettings.IgnoreFamilies and the YANG description of `mode ignore` all said the NLRI
// was skipped; only the code disagreed.
//
// drop is per-MESSAGE and not per-attribute, because a rebuild on the receive path to
// strip one attribute is not worth what it costs. An UPDATE that carries an unnegotiated
// MP attribute alongside a negotiated IPv4 NLRI field loses the IPv4 half too. No
// conformant peer sends one (RFC 4760 Section 8 obliges the sender to advertise the
// family first), and the operator asked to ignore the family rather than to salvage what
// travels with it.
func (s *Session) validateUpdateFamilies(body []byte) (drop bool, err error) {
	// Need at least 4 bytes: withdrawn len (2) + attrs len (2)
	if len(body) < 4 {
		return false, nil // Let message parsing handle malformed
	}

	// Skip withdrawn routes
	withdrawnLen := binary.BigEndian.Uint16(body[0:2])
	offset := 2 + int(withdrawnLen)
	if offset+2 > len(body) {
		return false, nil
	}

	// Get path attributes
	attrLen := binary.BigEndian.Uint16(body[offset : offset+2])
	offset += 2
	if offset+int(attrLen) > len(body) {
		return false, nil
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
		if flags&0x10 == 0 {
			if pos+1 > len(pathAttrs) {
				break
			}
			attrDataLen = int(pathAttrs[pos])
			pos++
		} else { // Extended length
			if pos+2 > len(pathAttrs) {
				break
			}
			attrDataLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
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
					// Lenient mode: drop the message, keep the session. Returning
					// here rather than continuing the attribute loop is what makes
					// the drop reach the caller.
					sessionLogger().Debug("UPDATE dropped, family not negotiated", "afi", afi, "safi", safi)
					return true, nil
				}
				// Strict mode: return error
				sessionLogger().Debug("UPDATE family mismatch rejected", "afi", afi, "safi", safi)
				return false, fmt.Errorf("%w: %s", ErrFamilyNotNegotiated, fam)
			}
		}
	}

	return false, nil
}

// validateCapabilityModes checks required/refused capability codes against the negotiated result.
// Sends NOTIFICATION and tears down the session if any violation is found.
// RFC 5492 Section 3: Unsupported Capability subcode.
func (s *Session) validateCapabilityModes(conn net.Conn, neg *capability.Negotiated, required, refused []capability.Code, localCaps, peerCaps []capability.Capability) error {
	if len(required) > 0 && neg != nil {
		if missing := neg.CheckRequiredCodes(required); len(missing) > 0 {
			capData := buildUnsupportedCapabilityDataCodes(missing, localCaps)
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
			capData := buildUnsupportedCapabilityDataCodes(present, peerCaps)
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
func (s *Session) validateAddPathFamilyModes(conn net.Conn, neg *capability.Negotiated, required, refused []capability.Family, localCaps, peerCaps []capability.Capability) error {
	if neg == nil {
		return nil
	}

	for _, f := range required {
		if neg.AddPathMode(f) != capability.AddPathNone {
			continue
		}
		// RFC 5492 Section 5: encode the required local ADD-PATH value.
		capData := buildUnsupportedAddPathData(f, localCaps)
		s.logNotifyErr(conn, message.NotifyOpenMessage, message.NotifyOpenUnsupportedCapability, capData)
		s.logFSMEvent(fsm.EventBGPOpenMsgErr)
		s.closeConn()
		return fmt.Errorf("%w: required ADD-PATH family not negotiated: %s", ErrInvalidState, f)
	}

	for _, f := range refused {
		if neg.AddPathMode(f) == capability.AddPathNone {
			continue
		}
		// RFC 5492 Section 5: encode the refused peer ADD-PATH value.
		capData := buildUnsupportedAddPathData(f, peerCaps)
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

// buildUnsupportedCapabilityDataCodes selects the causing capabilities from the
// local OPEN for missing requirements, or the peer OPEN for refused capabilities.
// It retains every instance and its value, using the same encoder as OPEN.
//
// RFC 5492 Section 5: "The Data field in the NOTIFICATION message MUST list the
// set of capabilities that causes the speaker to send the message."
// "Each such capability is encoded in the same way as it would be encoded in
// the OPEN message."
//
// Each tuple: offset 0 Code (1 octet), offset 1 Length (1 octet),
// offset 2 Value (Length octets). There is no Optional Parameter wrapper.
func buildUnsupportedCapabilityDataCodes(codes []capability.Code, caps []capability.Capability) []byte {
	size := 0
	for _, code := range codes {
		for _, cap := range caps {
			if cap.Code() == code {
				size += cap.Len()
			}
		}
	}
	if size == 0 {
		return nil
	}
	data := make([]byte, size)
	offset := 0
	for _, code := range codes {
		for _, cap := range caps {
			if cap.Code() == code {
				offset += cap.WriteTo(data, offset)
			}
		}
	}
	return data
}

// buildUnsupportedAddPathData selects the causing family's original advertised
// direction, retaining the original capability-instance grouping. Splitting each
// family entry into its own capability would inflate a legal OPEN's values past
// the NOTIFICATION size limit.
//
// RFC 5492 Section 5: "Each such capability is encoded in the same way as it
// would be encoded in the OPEN message."
//
// Each capability: Code 69 (offset 0), Length (offset 1), then four-octet entries
// from offset 2: AFI (2 octets), SAFI (1 octet), Send/Receive (1 octet).
func buildUnsupportedAddPathData(family capability.Family, caps []capability.Capability) []byte {
	// Both callers supply parsed OPEN capabilities. The one-octet capability
	// length bounds each ADD-PATH instance to 63 four-octet family entries.
	var families [255 / 4]capability.AddPathFamily
	selected := capability.AddPath{}
	size := 0
	for _, cap := range caps {
		addPath, ok := cap.(*capability.AddPath)
		if !ok {
			continue
		}
		count := 0
		for _, entry := range addPath.Families {
			if entry.AFI == family.AFI && entry.SAFI == family.SAFI {
				count++
			}
		}
		if count > 0 {
			selected.Families = families[:count]
			size += selected.Len()
		}
	}
	if size == 0 {
		return nil
	}
	data := make([]byte, size)
	offset := 0
	for _, cap := range caps {
		addPath, ok := cap.(*capability.AddPath)
		if !ok {
			continue
		}
		count := 0
		for _, entry := range addPath.Families {
			if entry.AFI == family.AFI && entry.SAFI == family.SAFI {
				families[count] = entry
				count++
			}
		}
		if count > 0 {
			selected.Families = families[:count]
			offset += selected.WriteTo(data, offset)
		}
	}
	return data
}
