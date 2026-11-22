// Design: docs/architecture/wire/attributes.md -- RFC 8097 Origin Validation State Extended Community
// RFC: rfc/short/rfc8097.md -- Section 2 encoding, decoding, EBGP drop, multi-instance resolution, and >2 discard
// Related: community.go -- ExtendedCommunity, ExtendedCommunities.WriteTo
// Related: extcomm_hex.go -- ParseExtendedCommunityHex
// Related: extcomm_keyword.go -- ParseExtendedCommunityKeyword
// Related: extcomm_decoded.go -- AppendDecoded

package attribute

import (
	"fmt"
	"strconv"
	"strings"
)

// RFC 8097 Section 2 and Section 4:
// "The value of the high-order octet of the extended Type field is 0x43,
// which indicates it is non-transitive. The value of the low-order octet of
// the extended Type field as assigned by IANA is 0x00."
const (
	ExtCommTypeNonTransOpaque      uint8  = 0x43
	ExtCommSubTypeOriginValidation uint8  = 0x00
	extCommOriginValidationState   uint16 = 0x4300
)

// OriginValidationState is the RFC 6811 / RFC 8097 Section 2 prefix origin
// validation state encoded in the last octet of the Origin Validation State
// Extended Community.
type OriginValidationState uint8

const (
	// OriginValidationValid indicates lookup result "valid" (0).
	OriginValidationValid OriginValidationState = 0
	// OriginValidationNotFound indicates lookup result "not found" (1).
	OriginValidationNotFound OriginValidationState = 1
	// OriginValidationInvalid indicates lookup result "invalid" (2).
	OriginValidationInvalid OriginValidationState = 2
)

// String returns the canonical text form of a valid origin validation state.
func (s OriginValidationState) String() string {
	switch s {
	case OriginValidationValid:
		return "valid"
	case OriginValidationNotFound:
		return "not-found"
	case OriginValidationInvalid:
		return "invalid"
	default:
		return "unknown"
	}
}

// IsOriginValidationStateExtCommunity reports whether ec carries the RFC 8097
// type/subtype pair (0x43, 0x00).
func IsOriginValidationStateExtCommunity(ec ExtendedCommunity) bool {
	if ec[0] != ExtCommTypeNonTransOpaque {
		return false
	}
	return ec[1] == ExtCommSubTypeOriginValidation
}

// originValidationReservedNonZero reports whether any of the 5 Reserved octets
// (bytes 2..6) of ec is non-zero.
func originValidationReservedNonZero(ec ExtendedCommunity) bool {
	for i := 2; i < 7; i++ {
		if ec[i] != 0 {
			return true
		}
	}
	return false
}

// validateOriginValidationStateExtCommunityForSend enforces RFC 8097 Section 2
// on any locally configured or parsed 0x43/0x00 extended community before it
// can enter Ze's send/policy paths:
//
// RFC 8097 Section 2: "The Reserved field MUST be set to 0 and ignored upon
// the receipt of this community. The last octet of the extended community is an
// unsigned integer that gives the route's validation state [RFC6811]."
func validateOriginValidationStateExtCommunityForSend(ec ExtendedCommunity) error {
	if !IsOriginValidationStateExtCommunity(ec) {
		return nil
	}
	if originValidationReservedNonZero(ec) {
		return fmt.Errorf("RFC 8097 origin validation state extended community requires Reserved octets [2..6] to be zero")
	}
	if OriginValidationState(ec[7]) > OriginValidationInvalid {
		return fmt.Errorf("RFC 8097 origin validation state %d exceeds maximum specified value 2", ec[7])
	}
	return nil
}

// HasOriginValidationStateNonZeroReserved reports whether raw (a packed
// EXTENDED_COMMUNITIES attribute value of 8-byte entries) contains any
// 0x43/0x00 entry with a non-zero octet in its 5-byte Reserved field [2..6].
func HasOriginValidationStateNonZeroReserved(raw []byte) bool {
	if len(raw)%8 != 0 {
		return false
	}
	for off := 0; off+8 <= len(raw); off += 8 {
		if raw[off] != ExtCommTypeNonTransOpaque {
			continue
		}
		if raw[off+1] != ExtCommSubTypeOriginValidation {
			continue
		}
		for i := 2; i < 7; i++ {
			if raw[off+i] != 0 {
				return true
			}
		}
	}
	return false
}

// ZeroOriginValidationStateReservedBytes zeroes the 5-octet Reserved field
// (bytes 2..6) of every RFC 8097 Origin Validation State extended community
// (0x43/0x00) in raw in place, leaving all other extended communities untouched.
//
// RFC 8097 Section 2: "The Reserved field MUST be set to 0 and ignored upon
// the receipt of this community."
func ZeroOriginValidationStateReservedBytes(raw []byte) {
	if len(raw)%8 != 0 {
		return
	}
	for off := 0; off+8 <= len(raw); off += 8 {
		if raw[off] != ExtCommTypeNonTransOpaque {
			continue
		}
		if raw[off+1] != ExtCommSubTypeOriginValidation {
			continue
		}
		clear(raw[off+2 : off+7])
	}
}

// zeroOriginValidationStateReservedInWireAttr zeroes Reserved bytes [2..6] of
// any 0x43/0x00 entry inside a complete wire-encoded EXTENDED_COMMUNITIES
// attribute (flags + code + 1- or 2-byte length + value).
func zeroOriginValidationStateReservedInWireAttr(wireAttr []byte) {
	if len(wireAttr) < 3 {
		return
	}
	hdrLen := 3
	if AttributeFlags(wireAttr[0])&FlagExtLength != 0 {
		hdrLen = 4
	}
	if len(wireAttr) < hdrLen {
		return
	}
	ZeroOriginValidationStateReservedBytes(wireAttr[hdrLen:])
}

// NewOriginValidationStateExtCommunity constructs an RFC 8097 Section 2 Origin
// Validation State Extended Community.
//
// RFC 8097 Section 2: "The value of the high-order octet of the extended Type
// field is 0x43, which indicates it is non-transitive. The value of the
// low-order octet of the extended Type field as assigned by IANA is 0x00. The
// Reserved field MUST be set to 0 and ignored upon the receipt of this
// community. The last octet of the extended community is an unsigned integer
// that gives the route's validation state [RFC6811]."
func NewOriginValidationStateExtCommunity(state OriginValidationState) (ExtendedCommunity, error) {
	if state > OriginValidationInvalid {
		return ExtendedCommunity{}, fmt.Errorf(
			"invalid RFC 8097 origin validation state %d: maximum specified value is %d (0=valid, 1=not-found, 2=invalid)",
			state, OriginValidationInvalid)
	}
	return ExtendedCommunity{
		ExtCommTypeNonTransOpaque,
		ExtCommSubTypeOriginValidation,
		0, 0, 0, 0, 0,
		uint8(state),
	}, nil
}

// DecodeOriginValidationStateExtCommunity decodes one RFC 8097 Origin
// Validation State Extended Community.
//
// RFC 8097 Section 2: "The Reserved field MUST be set to 0 and ignored upon
// the receipt of this community."
//
// Returns (state, true) when ec has type 0x43/0x00 and state <= 2 (regardless
// of any non-zero bytes in the 5-octet Reserved field). Returns (rawState, false)
// when ec has type 0x43/0x00 and rawState > 2, or (0, false) for other types.
func DecodeOriginValidationStateExtCommunity(ec ExtendedCommunity) (OriginValidationState, bool) {
	if !IsOriginValidationStateExtCommunity(ec) {
		return 0, false
	}
	state := OriginValidationState(ec[7])
	if state > OriginValidationInvalid {
		return state, false
	}
	return state, true
}

// ResolveOriginValidationState extracts the effective RFC 8097 origin
// validation state from a list of received extended communities.
//
// RFC 8097 Section 2:
//   - "The Reserved field MUST be set to 0 and ignored upon the receipt of this community."
//   - "However, if more than one instance is received, an implementation MUST disregard
//     all instances other than the one with the numerically greatest validation state value."
//   - "If the value received is greater than the largest specified value (2), the
//     implementation MUST apply a strategy similar to attribute discard [RFC7606] by
//     discarding the erroneous community and logging the error for further analysis."
func ResolveOriginValidationState(comms ExtendedCommunities) (state OriginValidationState, found bool, discardedValues []uint8) {
	for _, ec := range comms {
		if !IsOriginValidationStateExtCommunity(ec) {
			continue
		}
		val, ok := DecodeOriginValidationStateExtCommunity(ec)
		if !ok {
			discardedValues = append(discardedValues, uint8(val))
			continue
		}
		if !found {
			state = val
			found = true
			continue
		}
		if val > state {
			state = val
		}
	}
	return state, found, discardedValues
}

// SanitizeOriginValidationStateAttr enforces all RFC 8097 Section 2 receive
// rules over the raw value octets of an EXTENDED_COMMUNITIES attribute (whose
// length is a multiple of 8 bytes):
//
//   - RFC 8097 Section 2 (EBGP default drop): "By default, implementations MUST
//     drop the origin validation state extended community if received from an
//     External BGP (EBGP) peer, without processing it further."
//   - RFC 8097 Section 2 (Reserved ignored): "The Reserved field MUST be set to
//     0 and ignored upon the receipt of this community."
//   - RFC 8097 Section 2 (multiple instances): "However, if more than one
//     instance is received, an implementation MUST disregard all instances other
//     than the one with the numerically greatest validation state value."
//   - RFC 8097 Section 2 (value > 2 discard): "If the value received is greater
//     than the largest specified value (2), the implementation MUST apply a
//     strategy similar to attribute discard [RFC7606] by discarding the
//     erroneous community and logging the error for further analysis."
//
// When no 0x43/0x00 community appears in attrValue, returns (attrValue, false, ...)
// without allocating. Otherwise returns the sanitized extended-communities bytes
// (empty slice when all entries were dropped/discarded), the resolved winning
// state (if any), and every erroneous state value > 2 discarded for logging.
func SanitizeOriginValidationStateAttr(attrValue []byte, isEBGP, acceptFromEBGP bool) (rewritten []byte, changed bool, state OriginValidationState, hasState bool, discardedValues []uint8) {
	if len(attrValue)%8 != 0 {
		return attrValue, false, 0, false, nil
	}
	ovCount := 0
	for off := 0; off+8 <= len(attrValue); off += 8 {
		if attrValue[off] == ExtCommTypeNonTransOpaque {
			if attrValue[off+1] == ExtCommSubTypeOriginValidation {
				ovCount++
			}
		}
	}
	if ovCount == 0 {
		return attrValue, false, 0, false, nil
	}

	// RFC 8097 Section 2: drop from EBGP without processing further unless configured.
	if isEBGP {
		if !acceptFromEBGP {
			out := make([]byte, 0, len(attrValue)-ovCount*8)
			for off := 0; off+8 <= len(attrValue); off += 8 {
				if attrValue[off] == ExtCommTypeNonTransOpaque {
					if attrValue[off+1] == ExtCommSubTypeOriginValidation {
						continue
					}
				}
				out = append(out, attrValue[off:off+8]...)
			}
			return out, true, 0, false, nil
		}
	}

	// iBGP (or explicitly configured EBGP): ignore Reserved octets [2..6],
	// discard erroneous values > 2, and keep only the numerically greatest valid state.
	for off := 0; off+8 <= len(attrValue); off += 8 {
		if attrValue[off] != ExtCommTypeNonTransOpaque {
			continue
		}
		if attrValue[off+1] != ExtCommSubTypeOriginValidation {
			continue
		}
		rawVal := attrValue[off+7]
		if rawVal > uint8(OriginValidationInvalid) {
			discardedValues = append(discardedValues, rawVal)
			continue
		}
		val := OriginValidationState(rawVal)
		if !hasState {
			state = val
			hasState = true
			continue
		}
		if val > state {
			state = val
		}
	}

	// Fast path: single valid 0x43/0x00 instance with already-zero Reserved octets needs no rewrite.
	if ovCount == 1 {
		if hasState {
			if len(discardedValues) == 0 {
				reservedDirty := false
				for off := 0; off+8 <= len(attrValue); off += 8 {
					if attrValue[off] == ExtCommTypeNonTransOpaque {
						if attrValue[off+1] == ExtCommSubTypeOriginValidation {
							var ec ExtendedCommunity
							copy(ec[:], attrValue[off:off+8])
							reservedDirty = originValidationReservedNonZero(ec)
							break
						}
					}
				}
				if !reservedDirty {
					return attrValue, false, state, true, nil
				}
			}
		}
	}

	outCap := len(attrValue) - ovCount*8
	if hasState {
		outCap += 8
	}
	out := make([]byte, 0, outCap)
	emittedWinner := false
	for off := 0; off+8 <= len(attrValue); off += 8 {
		if attrValue[off] == ExtCommTypeNonTransOpaque {
			if attrValue[off+1] == ExtCommSubTypeOriginValidation {
				if hasState {
					if !emittedWinner {
						canonical, _ := NewOriginValidationStateExtCommunity(state)
						out = append(out, canonical[:]...)
						emittedWinner = true
					}
				}
				continue
			}
		}
		out = append(out, attrValue[off:off+8]...)
	}
	return out, true, state, hasState, discardedValues
}

func parseOriginValidationState(value string) (ExtendedCommunity, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "valid", "0":
		return NewOriginValidationStateExtCommunity(OriginValidationValid)
	case "not-found", "not_found", "notfound", "1":
		return NewOriginValidationStateExtCommunity(OriginValidationNotFound)
	case "invalid", "2":
		return NewOriginValidationStateExtCommunity(OriginValidationInvalid)
	default:
		if n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 8); err == nil {
			return NewOriginValidationStateExtCommunity(OriginValidationState(n))
		}
		return ExtendedCommunity{}, fmt.Errorf(
			"invalid origin-validation-state %q: want valid (0), not-found (1), or invalid (2)", value)
	}
}

func appendExtCommOriginValidationState(buf []byte, e ExtendedCommunity) []byte {
	state, ok := DecodeOriginValidationStateExtCommunity(e)
	if !ok {
		buf = append(buf, "origin-validation-state:"...)
		return strconv.AppendUint(buf, uint64(e[7]), 10)
	}
	buf = append(buf, "origin-validation-state:"...)
	return append(buf, state.String()...)
}
