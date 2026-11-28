// Design: docs/architecture/wire/attributes.md -- RFC 8097 Origin Validation State boundary handling
// RFC: rfc/short/rfc8097.md -- Section 2 (EBGP default drop/strip, overrides, Reserved ignored, greatest-value selection, >2 discard + log)
// Related: session_read.go -- processValidatedMessage receive pipeline
// Related: session_write.go -- writeUpdateBody egress pipeline
// Related: session_egress_normalize.go -- normalizeTransmittedUpdateWith in-place egress strip
// Related: session_problem_diagnostic.go -- codeDoctorBGPOriginValidationStateBad

package reactor

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// sanitizeOriginValidationState enforces RFC 8097 Section 2 on received UPDATE
// messages before publication to plugins, RIB, and forwarders:
//   - On EBGP peers without AcceptOriginValidationState configured, drops every
//     0x43/0x00 community without processing it further.
//   - On IBGP peers (or EBGP peers explicitly configured to accept), ignores
//     the 5-octet Reserved field, discards and logs any instance with state > 2
//     (RFC 7606 attribute-discard strategy), and keeps only the single canonical
//     instance with the numerically greatest valid state (2 > 1 > 0).
func (s *Session) sanitizeOriginValidationState(wu *wireu.WireUpdate) *wireu.WireUpdate {
	if wu == nil {
		return nil
	}
	sections, err := wire.ParseUpdateSections(wu.Payload())
	if err != nil {
		return wu
	}
	attrs := sections.Attrs(wu.Payload())
	hdr, flags, values, found := attribute.AttrFind(attrs, attribute.AttrExtCommunity)
	if !found {
		return wu
	}

	isEBGP := s.sessionIsEBGP()
	sanitized, changed, _, _, discarded := attribute.SanitizeOriginValidationStateAttr(
		values, isEBGP, s.settings.AcceptOriginValidationState)

	// RFC 8097 Section 2: "If the value received is greater than the largest
	// specified value (2), the implementation MUST apply a strategy similar to
	// attribute discard [RFC7606] by discarding the erroneous community and
	// logging the error for further analysis."
	for _, badVal := range discarded {
		sessionLogger().Error("discarding RFC 8097 origin validation state extended community with invalid state value",
			"code", codeDoctorBGPOriginValidationStateBad,
			"peer", s.settings.Address,
			"validation-state", badVal,
		)
	}
	if !changed {
		return wu
	}

	headerLen := 3
	if flags&attribute.FlagExtLength != 0 {
		headerLen = 4
	}
	rewritten := make([]byte, 0, len(attrs)-len(values)+len(sanitized))
	rewritten = append(rewritten, attrs[:hdr]...)
	if len(sanitized) > 0 {
		rewritten = append(rewritten, attrs[hdr:hdr+headerLen]...)
		if headerLen == 4 {
			binary.BigEndian.PutUint16(rewritten[hdr+2:], uint16(len(sanitized)))
		} else {
			rewritten[hdr+2] = byte(len(sanitized))
		}
		rewritten = append(rewritten, sanitized...)
	}
	rewritten = append(rewritten, attrs[hdr+headerLen+len(values):]...)

	result := wireu.NewWireUpdate(message.RebuildUpdateBody(wu.Payload(), rewritten), wu.SourceCtxID())
	result.SetSourceID(wu.SourceID())
	result.SetMessageID(wu.MessageID())
	result.SetSourceReceipt(wu.SourceReceipt())
	result.SetSourceOwner(wu.SourceOwner())
	return result
}

// originValidationStateSendAllowedTo reports whether RFC 8097 Section 2 permits
// sending the BGP Prefix Origin Validation State extended community (0x43/0x00)
// to a peer.
//
// RFC 8097 Section 2: "Similarly, by default, an implementation SHOULD NOT send
// the community to EBGP peers."
// RFC 8097 Section 2: "However, it SHOULD be possible to configure an
// implementation to send or accept the community when warranted."
func originValidationStateSendAllowedTo(isEBGP bool, settings *PeerSettings) bool {
	if !isEBGP {
		return true
	}
	// refuse-by-default: guards Session.originValidationStateSendAllowedHeld -- absent peer settings cannot authorize sending 0x43/0x00 across an EBGP boundary.
	if settings == nil {
		return false
	}
	return settings.SendOriginValidationState
}

// sessionIsEBGP reports whether this session is an External BGP session.
// Safe for concurrent use when the caller does not hold writeMu.
func (s *Session) sessionIsEBGP() bool {
	s.mu.RLock()
	neg := s.negotiated
	s.mu.RUnlock()
	return s.sessionIsEBGPFrom(neg)
}

// sessionIsEBGPHeld reports whether this session is an External BGP session.
// Caller MUST hold writeMu (which publishNegotiation holds when publishing
// s.negotiated; writers MUST NOT take s.mu in reverse lock order).
func (s *Session) sessionIsEBGPHeld() bool {
	return s.sessionIsEBGPFrom(s.negotiated)
}

// sessionIsEBGPFrom derives the EBGP verdict from a negotiated snapshot,
// falling back to PeerSettings.IsEBGP() (RFC 7705 Section 4.2) before
// negotiation completes.
func (s *Session) sessionIsEBGPFrom(neg *capability.Negotiated) bool {
	if neg != nil {
		if neg.Identity != nil {
			return !neg.Identity.IsIBGP()
		}
	}
	if s.settings == nil {
		return true
	}
	return s.settings.IsEBGP()
}

// originValidationStateSendAllowedHeld reports whether RFC 8097 Section 2
// permits transmitting 0x43/0x00 on this session.
// Caller MUST hold writeMu.
func (s *Session) originValidationStateSendAllowedHeld() bool {
	return originValidationStateSendAllowedTo(s.sessionIsEBGPHeld(), s.settings)
}
