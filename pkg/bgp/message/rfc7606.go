package message

import (
	"encoding/binary"
	"fmt"
)

// RFC7606Action represents the error handling action per RFC 7606.
type RFC7606Action int

const (
	// RFC7606ActionNone - No error detected.
	RFC7606ActionNone RFC7606Action = iota
	// RFC7606ActionTreatAsWithdraw - Treat UPDATE as withdrawal (RFC 7606 Section 2).
	RFC7606ActionTreatAsWithdraw
	// RFC7606ActionAttributeDiscard - Discard malformed attribute, continue (RFC 7606 Section 2).
	RFC7606ActionAttributeDiscard
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
	Action      RFC7606Action
	AttrCode    uint8  // Attribute code that caused the error (0 if N/A)
	Description string // Human-readable error description
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

// ValidateUpdateRFC7606 validates an UPDATE message per RFC 7606.
//
// RFC 7606 revises error handling for UPDATE messages to minimize session resets.
// This function checks path attributes for common malformations and returns the
// appropriate error handling action.
//
// Parameters:
//   - pathAttrs: Raw path attributes bytes from UPDATE message
//   - hasNLRI: Whether the UPDATE has NLRI (for mandatory attribute checking)
//
// Returns:
//   - ValidationResult with action to take and error details
func ValidateUpdateRFC7606(pathAttrs []byte, hasNLRI bool) *RFC7606ValidationResult {
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

	// Parse attributes
	pos := 0
	for pos < len(pathAttrs) {
		// Need at least flags + type code
		if pos+2 > len(pathAttrs) {
			// RFC 7606 Section 4: treat-as-withdraw for attribute parsing errors
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
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
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCode,
				Description: fmt.Sprintf("RFC 7606 Section 4: attribute %d length %d exceeds remaining data", attrCode, attrLen),
			}
		}

		attrData := pathAttrs[pos : pos+attrLen]
		pos += attrLen

		// Validate specific attributes per RFC 7606 Section 7
		result := validateAttribute(attrCode, attrLen, attrData)
		if result != nil && result.Action != RFC7606ActionNone {
			return result
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

	// RFC 7606 Section 3.g: Multiple MP_REACH or MP_UNREACH is session reset
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
		// Traditional IPv4 UPDATE needs explicit NEXT_HOP
		if !hasOrigin {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeOrigin,
				Description: "RFC 7606 Section 3.d: missing ORIGIN attribute",
			}
		}
		if !hasASPath {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 3.d: missing AS_PATH attribute",
			}
		}
		if !hasNextHop {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeNextHop,
				Description: "RFC 7606 Section 3.d: missing NEXT_HOP attribute",
			}
		}
	}

	// For MP_REACH_NLRI UPDATE: ORIGIN and AS_PATH are mandatory
	if mpReachCount > 0 {
		if !hasOrigin {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeOrigin,
				Description: "RFC 7606 Section 3.d: missing ORIGIN attribute",
			}
		}
		if !hasASPath {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 3.d: missing AS_PATH attribute",
			}
		}
	}

	return &RFC7606ValidationResult{Action: RFC7606ActionNone}
}

// validateAttribute checks a single attribute per RFC 7606 Section 7.
func validateAttribute(code uint8, length int, _ []byte) *RFC7606ValidationResult {
	switch code {
	case attrCodeOrigin:
		// RFC 7606 Section 7.1: ORIGIN must be length 1
		if length != 1 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.1: ORIGIN length %d != 1", length),
			}
		}
		// Value validation (0, 1, 2) is handled by attribute parser

	case attrCodeNextHop:
		// RFC 7606 Section 7.3: NEXT_HOP must be length 4
		if length != 4 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.3: NEXT_HOP length %d != 4", length),
			}
		}

	case attrCodeMED:
		// RFC 7606 Section 7.4: MED must be length 4
		if length != 4 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.4: MED length %d != 4", length),
			}
		}

	case attrCodeLocalPref:
		// RFC 7606 Section 7.5: LOCAL_PREF must be length 4
		if length != 4 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.5: LOCAL_PREF length %d != 4", length),
			}
		}

	case attrCodeAtomicAgg:
		// RFC 7606 Section 7.6: ATOMIC_AGGREGATE must be length 0
		// Action is attribute-discard, not treat-as-withdraw
		if length != 0 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionAttributeDiscard,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.6: ATOMIC_AGGREGATE length %d != 0", length),
			}
		}

	case attrCodeAggregator:
		// RFC 7606 Section 7.7: AGGREGATOR must be 6 (2-byte AS) or 8 (4-byte AS)
		// Action is attribute-discard
		if length != 6 && length != 8 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionAttributeDiscard,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.7: AGGREGATOR length %d not 6 or 8", length),
			}
		}

	case attrCodeCommunity:
		// RFC 7606 Section 7.8: Community must be non-zero multiple of 4
		if length == 0 || length%4 != 0 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.8: Community length %d not multiple of 4", length),
			}
		}

	case attrCodeOriginatorID:
		// RFC 7606 Section 7.9: ORIGINATOR_ID must be length 4
		if length != 4 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.9: ORIGINATOR_ID length %d != 4", length),
			}
		}

	case attrCodeClusterList:
		// RFC 7606 Section 7.10: CLUSTER_LIST must be non-zero multiple of 4
		if length == 0 || length%4 != 0 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.10: CLUSTER_LIST length %d not multiple of 4", length),
			}
		}

	case attrCodeExtCommunity:
		// RFC 7606 Section 7.14: Extended Community must be non-zero multiple of 8
		if length == 0 || length%8 != 0 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.14: Extended Community length %d not multiple of 8", length),
			}
		}

	case attrCodeLargeCommunity:
		// RFC 8092 Section 5: Large Community must be non-zero multiple of 12
		if length == 0 || length%12 != 0 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 8092 Section 5: Large Community length %d not multiple of 12", length),
			}
		}
	}

	return nil
}
