// Design: docs/architecture/core-design.md -- receive UPDATE coalescing.
// Related: session_validation.go -- enforcement of the shared classification.
// RFC 7606 Sections 3, 5.3 and 6 -- see rfc/short/rfc7606.md.
package reactor

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// rfc7606Validation belongs to one original body, or to a batch whose original
// NLRI fields all passed the same validator and whose clean attributes match.
// Offsets do not retain a read buffer. The zero value is unclassified and MUST
// NOT reach applyRFC7606; classifyRFC7606 and the coalescer supply a result.
// Only a clean, non-MP, non-rewriting result can be retained by the coalescer.
type rfc7606Validation struct {
	result            *message.RFC7606ValidationResult
	attrsOffset       int
	attrsLen          int
	hasNLRI           bool
	syntaxField       string
	firstASChecked    bool
	as4Prepared       bool
	as4Plan           wireu.AS4FamilyPlan
	diagnosticWireHex string
}

// classifyRFC7606 separates validation from side effects, so an earlier valid
// batch can be dispatched before the current message resets the session.
// RFC 7606 Section 3(j): "Finally, we observe that in order to use the approach
// of "treat-as-withdraw", the entire NLRI field and/or the MP_REACH_NLRI and
// MP_UNREACH_NLRI attributes need to be successfully parsed -- what this
// entails is discussed in more detail in Section 5."
// RFC 4271 Section 4.3 body: [withdrawn length:2][withdrawn:N]
// [attribute length:2][attributes:M][NLRI:remaining body octets].
func (s *Session) classifyRFC7606(body []byte) rfc7606Validation {
	if len(body) < 4 {
		return malformedRFC7606Envelope("RFC 7606 Section 3(b): UPDATE too short for section headers")
	}
	withdrawnLen := int(binary.BigEndian.Uint16(body[:2]))
	offset := 2 + withdrawnLen
	if offset+2 > len(body) {
		return malformedRFC7606Envelope("RFC 7606 Section 3(b): Withdrawn Routes Length exceeds UPDATE")
	}
	// RFC 7606 Sections 3(i), 5.3; retain the ordinary reader's error precedence.
	if withdrawnLen > 0 {
		if result := s.validateRFC7606IPv4(body[2:offset]); result != nil {
			return rfc7606Validation{result: result, syntaxField: "withdrawn"}
		}
	}
	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(body) {
		return malformedRFC7606Envelope("RFC 7606 Section 3(b): Total Attribute Length exceeds UPDATE")
	}
	// RFC 7606 Sections 3(j), 5.3: the field ends at THIS message's end.
	if offset+attrLen < len(body) {
		if result := s.validateRFC7606IPv4(body[offset+attrLen:]); result != nil {
			return rfc7606Validation{result: result, syntaxField: "nlri"}
		}
	}
	validation := rfc7606Validation{
		attrsOffset: offset, attrsLen: attrLen, hasNLRI: offset+attrLen < len(body),
	}
	// RFC 7606 Section 3: one attribute verdict shared with the coalesced reader.
	validation.result = s.validateRFC7606Attrs(body[offset:offset+attrLen], validation.hasNLRI)
	return validation
}

// batchable excludes verdicts whose enforcement rewrites attributes or whose MP
// NLRI belongs to an individual message. RFC 7606 Sections 3 and 6 require those
// messages to retain their own error action and diagnostic boundary.
func (v *rfc7606Validation) batchable() bool {
	r := v.result
	return r.Action == message.RFC7606ActionNone && len(r.DuplicateRanges) == 0 &&
		!r.MPReachNLRI.Present && !r.MPUnreachNLRI.Present && !r.TunnelEncapPresent
}

// prepareRFC7606FirstAS checks a potential batch before it can lose its original
// boundary. RFC 7606 Section 7.2: "then an UPDATE message with a mismatching first
// AS number in the AS_PATH attribute SHOULD be considered malformed" and
// "handled using the approach of 'treat-as-withdraw'".
func (s *Session) prepareRFC7606FirstAS(body []byte, v *rfc7606Validation) {
	if !v.hasNLRI || !v.batchable() {
		return
	}
	peerAS, check := s.firstASPeer()
	if !check {
		v.firstASChecked = true
		return
	}
	ctx := bgpctx.Registry.Get(s.recvCtxID)
	if ctx == nil {
		// Early interpretation needs the established receive width. Without
		// that context, preserve the ordinary post-enforcement path rather
		// than inventing a semantic verdict before route-ignore processing.
		return
	}
	asn4 := ctx.ASN4()
	r := v.result
	path := body[v.attrsOffset+r.ASPath.Start : v.attrsOffset+r.ASPath.End]
	if !ctx.ASN4() || r.AS4Present {
		// RFC 6793 Section 4.2.3 supplies the paired AGGREGATOR gate and
		// reconstructed path; Section 4.1 discards AS4 attributes from NEW
		// speakers. Preserve both decisions without borrowing this body.
		plan, err := wireu.PrepareAS4Family(body, ctx.ASN4())
		if err != nil {
			r.Action = message.RFC7606ActionTreatAsWithdraw
			r.AttrCode = uint8(attribute.AttrASPath)
			r.Description = "AS_PATH cannot be reconciled: " + err.Error()
			return
		}
		v.as4Plan, v.as4Prepared = plan, true
		path, asn4 = v.as4Plan.ASPath(body), true
	}
	v.firstASChecked = true
	if firstASPathMismatch(path, asn4, peerAS) {
		if v.as4Prepared {
			s.logAS4Discards(v.as4Plan.Discards())
		}
		r.Action = message.RFC7606ActionTreatAsWithdraw
		r.AttrCode = uint8(attribute.AttrASPath)
		r.Description = "AS_PATH first AS does not match the neighbor AS"
	}
}

// malformedRFC7606Envelope records an unlocatable UPDATE without closing it yet.
// RFC 7606 Section 3(j): "If this is not possible, the procedures of [RFC4271]
// and/or [RFC4760] continue to apply, meaning that the 'session reset' approach
// (or the 'AFI/SAFI disable' approach) MUST be followed."
func malformedRFC7606Envelope(description string) rfc7606Validation {
	return rfc7606Validation{result: &message.RFC7606ValidationResult{
		Action: message.RFC7606ActionSessionReset, Description: description,
	}}
}

// validateRFC7606IPv4 is the common legacy-field validator for both readers.
// RFC 7606 Section 3(i): "The Withdrawn Routes field MUST be checked for syntactic
// correctness in the same manner as the NLRI field."
// RFC 7911 Section 3: "The Path Identifier is a four-octet field".
func (s *Session) validateRFC7606IPv4(field []byte) *message.RFC7606ValidationResult {
	ctx := bgpctx.Registry.Get(s.recvCtxID)
	return message.ValidateNLRISyntaxAddPath(field, false, ctx.AddPathFor(family.IPv4Unicast))
}

// validateRFC7606Attrs uses the message validator and session receive policies.
// RFC 7606 Section 3(h): "Otherwise, the approach with the strongest action MUST be used."
func (s *Session) validateRFC7606Attrs(pathAttrs []byte, hasNLRI bool) *message.RFC7606ValidationResult {
	recvCtx := bgpctx.Registry.Get(s.recvCtxID)
	addPathFor := func(afi uint16, safi uint8) bool {
		return recvCtx.AddPathFor(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)})
	}
	// RFC 7705 Section 4.2: "the BGP speaker MUST treat UPDATEs sent and received
	// to this peer as if this was a natively configured iBGP session".
	isIBGP := s.settings.IsIBGP()
	asn4 := false
	if neg := s.Negotiated(); neg != nil {
		asn4 = neg.ASN4
	}
	// RFC 7606 Section 3.
	result := message.ValidateUpdateRFC7606AddPath(pathAttrs, hasNLRI, isIBGP, asn4, addPathFor)

	// RFC 7311 Section 3.3: "If an AIGP attribute is received on a BGP
	// session for which AIGP_SESSION is disabled, the attribute MUST be
	// treated exactly as if it were an unrecognized non-transitive attribute."
	if !s.settings.AIGPEnabled() {
		if _, _, _, present := attribute.AttrFind(pathAttrs, attribute.AttrAIGP); present {
			if result.Action < message.RFC7606ActionAttributeDiscard {
				result.Action = message.RFC7606ActionAttributeDiscard
				result.AttrCode = uint8(attribute.AttrAIGP)
				result.Description = "RFC 7311 Section 3.3: AIGP disabled on this session"
			}
			if result.Action == message.RFC7606ActionAttributeDiscard {
				recorded := false
				for _, entry := range result.DiscardEntries {
					if entry.Code == uint8(attribute.AttrAIGP) {
						recorded = true
						break
					}
				}
				if !recorded {
					result.DiscardEntries = append(result.DiscardEntries, message.DiscardEntry{
						Code: uint8(attribute.AttrAIGP), Reason: message.DiscardReasonEBGPInvalid,
					})
				}
			}
		}
	}

	// RFC 8669 Section 4: discard PrefixSID from EBGP unless configured to accept.
	// Presence comes from the same completed attribute walk. Raise its action
	// in place, preserving duplicate ranges and the MP metadata it already found.
	if !isIBGP && !s.settings.AcceptSRv6PrefixSID {
		if result.PrefixSIDPresent {
			entry := message.DiscardEntry{Code: uint8(attribute.AttrPrefixSID), Reason: message.DiscardReasonEBGPInvalid}
			if result.Action < message.RFC7606ActionAttributeDiscard {
				result.Action = message.RFC7606ActionAttributeDiscard
				result.AttrCode = uint8(attribute.AttrPrefixSID)
				result.Description = "RFC 8669 Section 4: PrefixSID from EBGP discarded (not configured to accept)"
			}
			if result.Action == message.RFC7606ActionAttributeDiscard {
				result.DiscardEntries = append(result.DiscardEntries, entry)
			}
		}
	}
	return result
}
