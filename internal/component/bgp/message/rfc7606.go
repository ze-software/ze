// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc7606.md — revised error handling for UPDATE messages

package message

import (
	"encoding/binary"
	"fmt"
)

// RFC7606Action represents the error handling action per RFC 7606.
type RFC7606Action int

// RFC 7606 Section 2: action strength ordering (strongest to weakest):
// session-reset > treat-as-withdraw > attribute-discard > none
// Iota values match this ordering so numeric comparison gives strongest action.
const (
	// RFC7606ActionNone - No error detected.
	RFC7606ActionNone RFC7606Action = iota
	// RFC7606ActionAttributeDiscard - Discard malformed attribute, continue (RFC 7606 Section 2).
	RFC7606ActionAttributeDiscard
	// RFC7606ActionTreatAsWithdraw - Treat UPDATE as withdrawal (RFC 7606 Section 2).
	RFC7606ActionTreatAsWithdraw
	// RFC7606ActionSessionReset - Reset session (RFC 4271 behavior).
	RFC7606ActionSessionReset
)

func (a RFC7606Action) String() string {
	switch a {
	case RFC7606ActionNone:
		return "none"
	case RFC7606ActionTreatAsWithdraw:
		return "treat-as-withdraw"
	case RFC7606ActionAttributeDiscard:
		return "attribute-discard"
	case RFC7606ActionSessionReset:
		return "session-reset"
	default:
		return "unknown"
	}
}

// RFC7606ValidationResult contains the result of UPDATE validation.
type RFC7606ValidationResult struct {
	Action         RFC7606Action
	AttrCode       uint8          // Attribute code that caused the strongest error (0 if N/A)
	Reason         uint8          // Discard reason code (draft-mangin-idr-attr-discard-00 Section 4.4)
	Description    string         // Human-readable error description for the strongest error
	DiscardEntries []DiscardEntry // Attributes to discard with reason codes when Action is AttributeDiscard
}

// Attribute type codes per RFC 4271.
const (
	attrCodeOrigin         uint8 = 1
	attrCodeASPath         uint8 = 2
	attrCodeNextHop        uint8 = 3
	attrCodeMED            uint8 = 4
	attrCodeLocalPref      uint8 = 5
	attrCodeAtomicAgg      uint8 = 6
	attrCodeAggregator     uint8 = 7
	attrCodeCommunity      uint8 = 8
	attrCodeOriginatorID   uint8 = 9
	attrCodeClusterList    uint8 = 10
	attrCodeMPReachNLRI    uint8 = 14
	attrCodeMPUnreachNLRI  uint8 = 15
	attrCodeExtCommunity   uint8 = 16
	attrCodeLargeCommunity uint8 = 32
)

// Attribute flags bits (RFC 4271 Section 4.3).
const (
	attrFlagOptional   uint8 = 0x80 // Bit 0: Optional (1) vs Well-known (0)
	attrFlagTransitive uint8 = 0x40 // Bit 1: Transitive (1) vs Non-transitive (0)
)

// wellKnownAttrs lists attributes that are well-known (not optional).
// Well-known attributes must NOT have the Optional bit set.
// Well-known attributes MUST have the Transitive bit set.
var wellKnownAttrs = map[uint8]bool{
	attrCodeOrigin:    true,
	attrCodeASPath:    true,
	attrCodeNextHop:   true,
	attrCodeLocalPref: true, // Well-known for IBGP
	attrCodeAtomicAgg: true,
}

// validateAttributeFlags validates attribute flags per RFC 7606 Section 3.c.
//
// Well-known attributes must:
// - NOT have the Optional bit set (they are mandatory)
// - MUST have the Transitive bit set
//
// Returns nil if valid, or RFC7606ValidationResult with treat-as-withdraw action.
func validateAttributeFlags(code, flags uint8) *RFC7606ValidationResult {
	if !wellKnownAttrs[code] {
		// Optional attribute - flags not restricted
		return nil
	}

	// Well-known attribute: must NOT be optional
	if flags&attrFlagOptional != 0 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 3.c: well-known attribute %d marked as optional", code),
		}
	}

	// Well-known attribute: must be transitive
	if flags&attrFlagTransitive == 0 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 3.c: well-known attribute %d not transitive", code),
		}
	}

	return nil
}

// ValidateUpdateRFC7606 validates an UPDATE message per RFC 7606.
//
// RFC 7606 revises error handling for UPDATE messages to minimize session resets.
// This function checks ALL path attributes for malformations and returns the
// strongest error handling action per RFC 7606 Section 3.h.
//
// Parameters:
//   - pathAttrs: Raw path attributes bytes from UPDATE message
//   - hasNLRI: Whether the UPDATE has traditional IPv4 NLRI field
//   - isIBGP: Whether this is an IBGP session (affects LOCAL_PREF, ORIGINATOR_ID, CLUSTER_LIST)
//   - asn4: Whether 4-octet AS capability is negotiated (affects AS_PATH, AGGREGATOR length)
//
// Returns:
//   - ValidationResult with strongest action, error details, and attribute codes to discard
func ValidateUpdateRFC7606(pathAttrs []byte, hasNLRI, isIBGP, asn4 bool) *RFC7606ValidationResult {
	if len(pathAttrs) == 0 {
		// Empty path attributes with NLRI = missing mandatory attributes
		if hasNLRI {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				Description: "RFC 7606 Section 3.d: missing well-known mandatory attributes",
			}
		}
		return &RFC7606ValidationResult{Action: RFC7606ActionNone}
	}

	// Track which mandatory attributes are present
	var hasOrigin, hasASPath, hasNextHop bool
	var mpReachCount, mpUnreachCount int

	// RFC 7606 Section 3.g: Track seen attribute codes to detect duplicates
	seenCodes := make(map[uint8]bool)

	// RFC 7606 Section 3.h: "If multiple errors are found, use the strongest action."
	// Collect all errors to determine the strongest action.
	strongest := RFC7606ActionNone
	var strongestCode uint8
	var strongestDesc string
	var discardEntries []DiscardEntry

	// recordError updates the strongest action and tracks discard entries.
	recordError := func(r *RFC7606ValidationResult) {
		if r.Action == RFC7606ActionAttributeDiscard {
			discardEntries = append(discardEntries, DiscardEntry{Code: r.AttrCode, Reason: r.Reason})
		}
		if r.Action > strongest {
			strongest = r.Action
			strongestCode = r.AttrCode
			strongestDesc = r.Description
		}
	}

	// Parse attributes
	pos := 0
	for pos < len(pathAttrs) {
		// Need at least flags + type code
		if pos+2 > len(pathAttrs) {
			// RFC 7606 Section 4: "If the remaining number of octets ... is less than three
			// (or less than four if the Attribute Flags field has the Extended Length bit set)"
			// MUST use treat-as-withdraw — structural error, can't continue parsing.
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    strongestCode,
				Description: "RFC 7606 Section 4: insufficient data for attribute header",
			}
		}

		flags := pathAttrs[pos]
		attrCode := pathAttrs[pos+1]
		pos += 2

		// Determine attribute length
		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if pos+2 > len(pathAttrs) {
				return &RFC7606ValidationResult{
					Action:      RFC7606ActionTreatAsWithdraw,
					Description: "RFC 7606 Section 4: insufficient data for extended length",
				}
			}
			attrLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				return &RFC7606ValidationResult{
					Action:      RFC7606ActionTreatAsWithdraw,
					Description: "RFC 7606 Section 4: insufficient data for length",
				}
			}
			attrLen = int(pathAttrs[pos])
			pos++
		}

		// Check attribute data bounds
		if pos+attrLen > len(pathAttrs) {
			// RFC 7606 Section 4: "attribute length ... exceeds the amount of data"
			// Structural error — can't continue parsing remaining attributes.
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCode,
				Description: fmt.Sprintf("RFC 7606 Section 4: attribute %d length %d exceeds remaining data", attrCode, attrLen),
			}
		}

		attrData := pathAttrs[pos : pos+attrLen]
		pos += attrLen

		// RFC 7606 Section 3.c: Validate attribute flags
		if flagsResult := validateAttributeFlags(attrCode, flags); flagsResult != nil {
			recordError(flagsResult)
			continue // Collect more errors
		}

		// RFC 7606 Section 3.g: Handle duplicate attributes
		// MP_REACH/MP_UNREACH duplicates are handled below with session-reset
		// Other duplicates: discard all but first (skip validation/tracking)
		if seenCodes[attrCode] && attrCode != attrCodeMPReachNLRI && attrCode != attrCodeMPUnreachNLRI {
			continue
		}
		seenCodes[attrCode] = true

		// Validate specific attributes per RFC 7606 Section 7
		result := validateAttribute(attrCode, attrLen, attrData, isIBGP, asn4)
		if result != nil && result.Action != RFC7606ActionNone {
			// RFC 7606 Section 3.h: session-reset is immediate — no point collecting more
			if result.Action == RFC7606ActionSessionReset {
				return result
			}
			recordError(result)
			// Don't return — continue to collect all errors
		}

		// Track mandatory attributes and MP attribute counts
		switch attrCode {
		case attrCodeOrigin:
			hasOrigin = true
		case attrCodeASPath:
			hasASPath = true
		case attrCodeNextHop:
			hasNextHop = true
		case attrCodeMPReachNLRI:
			mpReachCount++
			hasNextHop = true // MP_REACH provides next-hop
		case attrCodeMPUnreachNLRI:
			mpUnreachCount++
		}
	}

	// RFC 7606 Section 3.g: "If the MP_REACH_NLRI attribute or the MP_UNREACH_NLRI
	// attribute appears more than once in the UPDATE message, then a NOTIFICATION
	// message MUST be sent with the Error Subcode 'Malformed Attribute List'."
	if mpReachCount > 1 || mpUnreachCount > 1 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			Description: "RFC 7606 Section 3.g: multiple MP_REACH_NLRI or MP_UNREACH_NLRI attributes",
		}
	}

	// RFC 7606 Section 3.d: Missing well-known mandatory attributes
	// For UPDATE with NLRI: ORIGIN, AS_PATH, NEXT_HOP are mandatory
	// (NEXT_HOP can be in MP_REACH_NLRI instead of explicit attribute)
	if hasNLRI && mpReachCount == 0 {
		if !hasOrigin {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeOrigin,
				Description: "RFC 7606 Section 3.d: missing ORIGIN attribute",
			})
		}
		if !hasASPath {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 3.d: missing AS_PATH attribute",
			})
		}
		if !hasNextHop {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeNextHop,
				Description: "RFC 7606 Section 3.d: missing NEXT_HOP attribute",
			})
		}
	}

	// For MP_REACH_NLRI UPDATE: ORIGIN and AS_PATH are mandatory
	if mpReachCount > 0 {
		if !hasOrigin {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeOrigin,
				Description: "RFC 7606 Section 3.d: missing ORIGIN attribute",
			})
		}
		if !hasASPath {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 3.d: missing AS_PATH attribute",
			})
		}
	}

	// RFC 7606 Section 5.2: "An UPDATE message with only path attributes and no associated
	// NLRI ... if any path attribute fails the checks ... and the error action is not
	// 'attribute discard' ... the session-reset action MUST be used."
	// No reachable NLRI means: no traditional NLRI AND no MP_REACH_NLRI.
	if !hasNLRI && mpReachCount == 0 && len(pathAttrs) > 0 && strongest > RFC7606ActionAttributeDiscard {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    strongestCode,
			Description: fmt.Sprintf("RFC 7606 Section 5.2: %s (escalated — attrs with no NLRI)", strongestDesc),
		}
	}

	if strongest == RFC7606ActionNone {
		return &RFC7606ValidationResult{Action: RFC7606ActionNone}
	}

	return &RFC7606ValidationResult{
		Action:         strongest,
		AttrCode:       strongestCode,
		Description:    strongestDesc,
		DiscardEntries: discardEntries,
	}
}

// attrValidatorFn checks a single attribute and returns a validation result, or nil if valid.
type attrValidatorFn func(code uint8, length int, data []byte, isIBGP, asn4 bool) *RFC7606ValidationResult

// attrValidators maps attribute type codes to per-attribute RFC 7606 validators.
// nil entries mean no specific validation for that code.
var attrValidators [256]attrValidatorFn

func init() {
	attrValidators[attrCodeOrigin] = validateOriginAttr
	attrValidators[attrCodeASPath] = validateASPathAttr
	attrValidators[attrCodeNextHop] = validateNextHopAttr
	attrValidators[attrCodeMED] = validateMEDAttr
	attrValidators[attrCodeLocalPref] = validateLocalPrefAttr
	attrValidators[attrCodeAtomicAgg] = validateAtomicAggAttr
	attrValidators[attrCodeAggregator] = validateAggregatorAttr
	attrValidators[attrCodeCommunity] = validateCommunityAttr
	attrValidators[attrCodeOriginatorID] = validateOriginatorIDAttr
	attrValidators[attrCodeClusterList] = validateClusterListAttr
	attrValidators[attrCodeExtCommunity] = validateExtCommunityAttr
	attrValidators[attrCodeLargeCommunity] = validateLargeCommunityAttr
	attrValidators[attrCodeMPReachNLRI] = validateMPReachAttr
	attrValidators[attrCodeMPUnreachNLRI] = validateMPUnreachAttr
}

// validateAttribute checks a single attribute per RFC 7606 Section 7.
func validateAttribute(code uint8, length int, attrData []byte, isIBGP, asn4 bool) *RFC7606ValidationResult {
	if fn := attrValidators[code]; fn != nil {
		return fn(code, length, attrData, isIBGP, asn4)
	}
	return nil
}

// RFC 7606 Section 7.1: ORIGIN must be length 1, value 0-2.
func validateOriginAttr(code uint8, length int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	if length != 1 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.1: ORIGIN length %d != 1", length),
		}
	}
	if len(attrData) > 0 && attrData[0] > 2 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.1: ORIGIN undefined value %d", attrData[0]),
		}
	}
	return nil
}

// RFC 7606 Section 7.2: Validate AS_PATH segment structure.
func validateASPathAttr(_ uint8, _ int, attrData []byte, _, asn4 bool) *RFC7606ValidationResult {
	return validateASPath(attrData, asn4)
}

// RFC 7606 Section 7.3: NEXT_HOP must be length 4.
func validateNextHopAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length != 4 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.3: NEXT_HOP length %d != 4", length),
		}
	}
	return nil
}

// RFC 7606 Section 7.4: MED must be length 4.
func validateMEDAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length != 4 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.4: MED length %d != 4", length),
		}
	}
	return nil
}

// RFC 7606 Section 7.5: LOCAL_PREF from EBGP discarded; must be length 4.
func validateLocalPrefAttr(code uint8, length int, _ []byte, isIBGP, _ bool) *RFC7606ValidationResult {
	if !isIBGP {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonEBGPInvalid,
			Description: "RFC 7606 Section 7.5: LOCAL_PREF from external neighbor must be discarded",
		}
	}
	if length != 4 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.5: LOCAL_PREF length %d != 4", length),
		}
	}
	return nil
}

// RFC 7606 Section 7.6: ATOMIC_AGGREGATE must be length 0 (attribute-discard).
func validateAtomicAggAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length != 0 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonInvalidLength,
			Description: fmt.Sprintf("RFC 7606 Section 7.6: ATOMIC_AGGREGATE length %d != 0", length),
		}
	}
	return nil
}

// RFC 7606 Section 7.7: AGGREGATOR length depends on 4-octet AS capability (attribute-discard).
func validateAggregatorAttr(code uint8, length int, _ []byte, _, asn4 bool) *RFC7606ValidationResult {
	expectedLen := 6
	if asn4 {
		expectedLen = 8
	}
	if length != expectedLen {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonInvalidLength,
			Description: fmt.Sprintf("RFC 7606 Section 7.7: AGGREGATOR length %d, expected %d (asn4=%t)", length, expectedLen, asn4),
		}
	}
	return nil
}

// RFC 7606 Section 7.8: Community must be non-zero multiple of 4.
func validateCommunityAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length == 0 || length%4 != 0 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.8: Community length %d not multiple of 4", length),
		}
	}
	return nil
}

// RFC 7606 Section 7.9: ORIGINATOR_ID from EBGP discarded; must be length 4.
func validateOriginatorIDAttr(code uint8, length int, _ []byte, isIBGP, _ bool) *RFC7606ValidationResult {
	if !isIBGP {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonEBGPInvalid,
			Description: "RFC 7606 Section 7.9: ORIGINATOR_ID from external neighbor must be discarded",
		}
	}
	if length != 4 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.9: ORIGINATOR_ID length %d != 4", length),
		}
	}
	return nil
}

// RFC 7606 Section 7.10: CLUSTER_LIST from EBGP discarded; must be non-zero multiple of 4.
func validateClusterListAttr(code uint8, length int, _ []byte, isIBGP, _ bool) *RFC7606ValidationResult {
	if !isIBGP {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonEBGPInvalid,
			Description: "RFC 7606 Section 7.10: CLUSTER_LIST from external neighbor must be discarded",
		}
	}
	if length == 0 || length%4 != 0 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.10: CLUSTER_LIST length %d not multiple of 4", length),
		}
	}
	return nil
}

// RFC 7606 Section 7.14: Extended Community must be non-zero multiple of 8.
func validateExtCommunityAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length == 0 || length%8 != 0 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 7.14: Extended Community length %d not multiple of 8", length),
		}
	}
	return nil
}

// RFC 8092 Section 5: Large Community must be non-zero multiple of 12.
func validateLargeCommunityAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length == 0 || length%12 != 0 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 8092 Section 5: Large Community length %d not multiple of 12", length),
		}
	}
	return nil
}

// RFC 7606 Section 5.3/7.11: MP_REACH_NLRI minimum length 5, next-hop validation.
func validateMPReachAttr(code uint8, length int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	if length < 5 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 5.3: MP_REACH_NLRI length %d < 5", length),
		}
	}
	return validateMPReachNextHop(attrData)
}

// RFC 7606 Section 5.3: MP_UNREACH_NLRI minimum length 3.
func validateMPUnreachAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length < 3 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    code,
			Description: fmt.Sprintf("RFC 7606 Section 5.3: MP_UNREACH_NLRI length %d < 3", length),
		}
	}
	return nil
}

// AS_PATH segment type constants (RFC 4271 Section 4.3).
const (
	asPathTypeASSet      = 1 // AS_SET: unordered set of ASes
	asPathTypeASSequence = 2 // AS_SEQUENCE: ordered set of ASes
	asPathTypeConfedSeq  = 3 // AS_CONFED_SEQUENCE (RFC 5065)
	asPathTypeConfedSet  = 4 // AS_CONFED_SET (RFC 5065)
)

// validateASPath validates AS_PATH segment structure per RFC 7606 Section 7.2.
//
// An AS_PATH is considered malformed if:
// - Unrecognized segment type is encountered
// - Segment overrun: segment length exceeds remaining data
// - Segment underrun: only 1 byte remaining after last segment
// - Zero segment length
//
// Parameters:
//   - data: Raw AS_PATH attribute value bytes
//   - asn4: True if ASNs are 4 bytes, false for 2 bytes
//
// Returns nil if valid, or RFC7606ValidationResult with treat-as-withdraw action.
func validateASPath(data []byte, asn4 bool) *RFC7606ValidationResult {
	// Empty AS_PATH is valid per RFC 7606 Section 5 (AS_PATH may have length zero)
	if len(data) == 0 {
		return nil
	}

	asSize := 2
	if asn4 {
		asSize = 4
	}

	pos := 0
	for pos < len(data) {
		// Need at least 2 bytes for segment header (type + length)
		if pos+2 > len(data) {
			// RFC 7606 Section 7.2: underrun - not enough for segment header
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 7.2: AS_PATH segment underrun (incomplete header)",
			}
		}

		segType := data[pos]
		segLen := int(data[pos+1])
		pos += 2

		// RFC 7606 Section 7.2: Validate segment type (1-4 are valid)
		if segType < asPathTypeASSet || segType > asPathTypeConfedSet {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: fmt.Sprintf("RFC 7606 Section 7.2: unrecognized AS_PATH segment type %d", segType),
			}
		}

		// RFC 7606 Section 7.2: Zero segment length is malformed
		if segLen == 0 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 7.2: AS_PATH segment with zero length",
			}
		}

		// RFC 7606 Section 7.2: Check for overrun
		segDataLen := segLen * asSize
		if pos+segDataLen > len(data) {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: fmt.Sprintf("RFC 7606 Section 7.2: AS_PATH segment overrun (need %d bytes, have %d)", segDataLen, len(data)-pos),
			}
		}
		pos += segDataLen
	}

	// RFC 7606 Section 7.2: Check for underrun (trailing partial data)
	// This is already handled above - if we exit the loop with pos < len(data),
	// the next iteration would catch it. But if pos == len(data) exactly, we're good.

	return nil
}

// AFI constants (RFC 4760).
const (
	afiIPv4 uint16 = 1
	afiIPv6 uint16 = 2
)

// SAFI constants (RFC 4760).
const (
	safiUnicast   uint8 = 1
	safiMulticast uint8 = 2
	safiMPLS      uint8 = 4   // RFC 3107
	safiVPN       uint8 = 128 // RFC 4364
)

// validateMPReachNextHop validates MP_REACH_NLRI next-hop length per RFC 7606 Section 7.11.
//
// The next-hop length must be consistent with the AFI/SAFI. Invalid lengths make it
// impossible to reliably locate the NLRI, so session-reset is required.
//
// Expected lengths:
//   - IPv4 unicast/multicast: 4 bytes
//   - IPv4 unicast/multicast with RFC 5549: 16 bytes (IPv6 next-hop)
//   - IPv6 unicast/multicast: 16 bytes (global) or 32 bytes (global + link-local)
//   - VPNv4: 12 bytes (8-byte RD + 4-byte IPv4)
//   - VPNv6: 24 bytes (8-byte RD + 16-byte IPv6)
func validateMPReachNextHop(data []byte) *RFC7606ValidationResult {
	// Need at least AFI (2) + SAFI (1) + NH_LEN (1) = 4 bytes
	if len(data) < 4 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    attrCodeMPReachNLRI,
			Description: "RFC 7606 Section 7.11: MP_REACH_NLRI too short to parse next-hop",
		}
	}

	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]
	nhLen := int(data[3])

	// Validate next-hop length based on AFI/SAFI
	var valid bool
	switch afi {
	case afiIPv4:
		switch safi {
		case safiUnicast, safiMulticast:
			// RFC 4760: 4 bytes for IPv4
			// RFC 5549: 16 bytes for IPv6 next-hop over IPv4 NLRI
			valid = nhLen == 4 || nhLen == 16
		case safiMPLS:
			// RFC 3107: 4 bytes for IPv4
			valid = nhLen == 4
		case safiVPN:
			// RFC 4364: 12 bytes (8-byte RD + 4-byte IPv4)
			// Also allow 24 bytes for RFC 5549 (8-byte RD + 16-byte IPv6)
			valid = nhLen == 12 || nhLen == 24
		default:
			// Unknown SAFI - accept any length to be permissive
			valid = true
		}
	case afiIPv6:
		switch safi {
		case safiUnicast, safiMulticast:
			// RFC 4760: 16 bytes (global) or 32 bytes (global + link-local)
			valid = nhLen == 16 || nhLen == 32
		case safiMPLS:
			// RFC 3107: 16 bytes (global) or 32 bytes (global + link-local)
			valid = nhLen == 16 || nhLen == 32
		case safiVPN:
			// RFC 4659: 24 bytes (8-byte RD + 16-byte IPv6)
			// Also 48 bytes for dual next-hop
			valid = nhLen == 24 || nhLen == 48
		default:
			// Unknown SAFI - accept any length to be permissive
			valid = true
		}
	default:
		// Unknown AFI - accept any length to be permissive
		valid = true
	}

	if !valid {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    attrCodeMPReachNLRI,
			Description: fmt.Sprintf("RFC 7606 Section 7.11: invalid next-hop length %d for AFI=%d SAFI=%d", nhLen, afi, safi),
		}
	}

	return nil
}

// ValidateNLRISyntax validates NLRI field structure per RFC 7606 Section 5.3.
//
// An NLRI field is considered syntactically incorrect if:
// - Any prefix length exceeds the maximum for the address family (32 for IPv4, 128 for IPv6)
// - Any prefix's byte count exceeds the remaining data in the field (overrun)
//
// Parameters:
//   - nlri: Raw NLRI bytes (prefix-length + prefix-bytes for each prefix)
//   - isIPv6: True for IPv6 NLRI (max prefix length 128), false for IPv4 (max 32)
//
// Returns nil if valid, or RFC7606ValidationResult with treat-as-withdraw action.
func ValidateNLRISyntax(nlri []byte, isIPv6 bool) *RFC7606ValidationResult {
	if len(nlri) == 0 {
		return nil
	}

	maxLen := 32
	if isIPv6 {
		maxLen = 128
	}

	pos := 0
	for pos < len(nlri) {
		// Each NLRI starts with 1-byte prefix length
		prefixLen := int(nlri[pos]) //nolint:gosec // pos < len(nlri) guaranteed by loop
		pos++

		// RFC 7606 Section 5.3: Prefix length must not exceed max for address family
		if prefixLen > maxLen {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				Description: fmt.Sprintf("RFC 7606 Section 5.3: prefix length %d > %d", prefixLen, maxLen),
			}
		}

		// Calculate bytes needed for this prefix: ceiling(prefixLen / 8)
		prefixBytes := (prefixLen + 7) / 8

		// RFC 7606 Section 3(j): "in order to use the approach of 'treat-as-withdraw',
		// the entire NLRI field ... need to be successfully parsed ... If this is not
		// possible ... the 'session reset' approach ... MUST be followed."
		// Overrun means the field cannot be fully parsed — session-reset required.
		if pos+prefixBytes > len(nlri) {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionSessionReset,
				Description: fmt.Sprintf("RFC 7606 Section 5.3/3(j): NLRI overrun (need %d bytes, have %d)", prefixBytes, len(nlri)-pos),
			}
		}

		pos += prefixBytes
	}

	return nil
}
