// Design: docs/architecture/core-design.md — RFC 7606 UPDATE validation
// Overview: session.go — BGP session struct and lifecycle

package reactor

import (
	"encoding/binary"
	"fmt"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// enforceRFC7606 validates an UPDATE per RFC 7606 and enforces the resulting action.
//
// Returns the (potentially new) WireUpdate, the action taken, and an error if
// session-reset is required. When attribute-discard applies, ATTR_DISCARD markers
// are written into the wire bytes per draft-mangin-idr-attr-discard-00.
// Called from processMessage() BEFORE callback dispatch so that malformed
// UPDATEs are never delivered to plugins as valid routes.
func (s *Session) enforceRFC7606(wu *wireu.WireUpdate) (*wireu.WireUpdate, message.RFC7606Action, error) {
	body := wu.Payload()

	// Parse UPDATE structure — truncated sections trigger treat-as-withdraw
	// to prevent malformed wire reaching plugins via callback dispatch.
	if len(body) < 4 {
		sessionLogger().Debug("RFC 7606 treat-as-withdraw (UPDATE too short for section headers)")
		return wu, message.RFC7606ActionTreatAsWithdraw, nil
	}

	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2 + withdrawnLen
	if offset+2 > len(body) {
		sessionLogger().Debug("RFC 7606 treat-as-withdraw (withdrawn length exceeds UPDATE)")
		return wu, message.RFC7606ActionTreatAsWithdraw, nil
	}

	// RFC 7606 Section 5.3: Validate withdrawn routes NLRI syntax (IPv4)
	if withdrawnLen > 0 {
		withdrawn := body[2 : 2+withdrawnLen]
		if result := message.ValidateNLRISyntax(withdrawn, false); result != nil {
			sessionLogger().Debug("RFC 7606 treat-as-withdraw (withdrawn NLRI syntax)", "description", result.Description)
			return wu, message.RFC7606ActionTreatAsWithdraw, nil
		}
	}

	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(body) {
		sessionLogger().Debug("RFC 7606 treat-as-withdraw (attribute length exceeds UPDATE)")
		return wu, message.RFC7606ActionTreatAsWithdraw, nil
	}

	pathAttrs := body[offset : offset+attrLen]
	nlriLen := len(body) - (offset + attrLen)
	hasNLRI := nlriLen > 0

	// RFC 7606 Section 5.3: Validate NLRI syntax (IPv4)
	if nlriLen > 0 {
		nlri := body[offset+attrLen:]
		if result := message.ValidateNLRISyntax(nlri, false); result != nil {
			sessionLogger().Debug("RFC 7606 treat-as-withdraw (NLRI syntax)", "description", result.Description)
			return wu, message.RFC7606ActionTreatAsWithdraw, nil
		}
	}

	// Validate path attributes per RFC 7606
	isIBGP := s.settings.LocalAS == s.settings.PeerAS
	asn4 := false
	if neg := s.Negotiated(); neg != nil {
		asn4 = neg.ASN4
	}
	result := message.ValidateUpdateRFC7606(pathAttrs, hasNLRI, isIBGP, asn4)

	switch result.Action {
	case message.RFC7606ActionNone:
		return wu, message.RFC7606ActionNone, nil

	case message.RFC7606ActionAttributeDiscard:
		// RFC 7606 Section 2: "The attribute MUST be discarded ... and the UPDATE
		// message continues to be processed."
		// draft-mangin-idr-attr-discard-00: Apply ATTR_DISCARD markers.
		sessionLogger().Debug("RFC 7606 attribute-discard",
			"attr", result.AttrCode,
			"discard-entries", result.DiscardEntries,
			"description", result.Description)

		// draft-mangin-idr-attr-discard-00 Section 5.1: log upstream pairs before merge.
		if upstream := message.ExtractUpstreamAttrDiscard(pathAttrs); len(upstream) > 0 {
			sessionLogger().Debug("RFC 7606 upstream ATTR_DISCARD before merge",
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
		// RFC 7606 Section 2: "MUST be handled as though all of the routes
		// contained in an UPDATE message ... had been withdrawn"
		sessionLogger().Debug("RFC 7606 treat-as-withdraw",
			"attr", result.AttrCode,
			"description", result.Description)
		return wu, message.RFC7606ActionTreatAsWithdraw, nil

	case message.RFC7606ActionSessionReset:
		// RFC 7606: Session reset — send NOTIFICATION with UPDATE Message Error.
		sessionLogger().Warn("RFC 7606 session-reset", "description", result.Description)

		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		// RFC 4271 Section 6.3: "If any ... error ... is detected ... a NOTIFICATION
		// message MUST be sent with the Error Code UPDATE Message Error."
		s.logNotifyErr(conn,
			message.NotifyUpdateMessage,
			message.NotifyUpdateMalformedAttr,
			nil,
		)
		s.logFSMEvent(fsm.EventUpdateMsgErr)
		s.closeConn()

		return wu, message.RFC7606ActionSessionReset, fmt.Errorf("RFC 7606 session reset: %s", result.Description)
	}

	return wu, message.RFC7606ActionNone, nil
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
