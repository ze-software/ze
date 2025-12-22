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
// TODO: Add asn4 parameter when ValidateUpdateRFC7606 signature is updated (Phase 3).
func validateAttribute(code uint8, length int, attrData []byte) *RFC7606ValidationResult {
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
		// RFC 7606 Section 7.1: ORIGIN value must be 0 (IGP), 1 (EGP), or 2 (INCOMPLETE)
		if len(attrData) > 0 && attrData[0] > 2 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 7.1: ORIGIN undefined value %d", attrData[0]),
			}
		}

	case attrCodeASPath:
		// RFC 7606 Section 7.2: Validate AS_PATH segment structure
		// Note: Using 2-byte ASN size. Will be parameterized in Phase 3.
		if result := validateASPath(attrData, false); result != nil {
			return result
		}

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

	case attrCodeMPReachNLRI:
		// RFC 7606 Section 5.3: MP_REACH_NLRI must be at least 5 bytes
		// (AFI=2 + SAFI=1 + NH_LEN=1 + Reserved=1 minimum)
		if length < 5 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionSessionReset,
				AttrCode:    code,
				Description: fmt.Sprintf("RFC 7606 Section 5.3: MP_REACH_NLRI length %d < 5", length),
			}
		}
		// RFC 7606 Section 7.11: Validate next-hop length per AFI/SAFI
		if result := validateMPReachNextHop(attrData); result != nil {
			return result
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

		// RFC 7606 Section 5.3: Check for overrun
		if pos+prefixBytes > len(nlri) {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				Description: fmt.Sprintf("RFC 7606 Section 5.3: NLRI overrun (need %d bytes, have %d)", prefixBytes, len(nlri)-pos),
			}
		}

		pos += prefixBytes
	}

	return nil
}
