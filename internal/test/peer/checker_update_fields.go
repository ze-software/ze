// Design: docs/architecture/testing/ci-format.md — scoped UPDATE assertions
// Related: checker.go, reject.go — positive and negative wire assertions

package peer

import (
	"fmt"
	"strconv"
	"strings"
)

type updateField uint8

const (
	updateFieldUnspecified updateField = iota
	updateFieldAnnounced
	updateFieldWithdrawn
)

func scopedUpdateRule(rule string) (updateField, string, bool) {
	if needle, ok := strings.CutPrefix(rule, "announced:"); ok {
		return updateFieldAnnounced, needle, true
	}
	if needle, ok := strings.CutPrefix(rule, "withdrawn:"); ok {
		return updateFieldWithdrawn, needle, true
	}
	return updateFieldUnspecified, "", false
}

// parseUpdateAttributeRule reads the fixture's decimal type code and hex value
// needle. A named attribute is a same-message condition, not another frame.
func parseUpdateAttributeRule(rule string) (uint8, string, error) {
	code, value, present := strings.Cut(rule, ",")
	if !present {
		return 0, "", fmt.Errorf("want decimal-code,hex-value, got %q", rule)
	}
	n, err := strconv.ParseUint(code, 10, 8)
	if err != nil {
		return 0, "", fmt.Errorf("invalid attribute code %q: %w", code, err)
	}
	if n == 0 {
		return 0, "", fmt.Errorf("attribute code must be 1..255")
	}
	if value == "" {
		return 0, "", fmt.Errorf("attribute value needle is empty")
	}
	if len(value)%2 != 0 {
		return 0, "", fmt.Errorf("attribute value needle has odd hex length")
	}
	value = strings.ToUpper(value)
	if strings.Trim(value, "0123456789ABCDEF") != "" {
		return 0, "", fmt.Errorf("attribute value needle is not hexadecimal")
	}
	return uint8(n), value, nil
}

// rejectedUpdateField keeps unscoped byte matching unchanged. A malformed UPDATE
// fails a scoped rejection closed: unreadable fields cannot prove an absence.
func rejectedUpdateField(rule, stream string) bool {
	field, needle, scoped := scopedUpdateRule(rule)
	if !scoped {
		return indexByteAligned(stream, rule, 0) >= 0
	}
	// RFC 4271 Section 4.3; RFC 4760 Sections 3 and 4.
	matched, valid := matchUpdateField(stream, needle, field)
	return !valid || matched
}

// matchUpdateField matches bytes only inside the requested NLRI fields. It does
// not decode NLRI: a needle may include an ADD-PATH identifier or opaque family
// bytes, and no match spans separate fields.
// An optional ":code,value" suffix also requires bytes inside that attribute's
// value in the same UPDATE, independent of short or extended attribute lengths.
//
// RFC 4271 Section 4.3: "An UPDATE message MAY simultaneously advertise a
// feasible route and withdraw multiple unfeasible routes from service."
// RFC 4760 Section 3: "A variable length field that lists NLRI for the feasible
// routes that are being advertised in this attribute."
// RFC 4760 Section 4: "A variable-length field that lists NLRI for the routes
// that are being withdrawn from service."
//
// Byte offsets (the matcher works on hex, so each offset is doubled):
//
// UPDATE: 19 | withdrawn-length(2) | withdrawn(W) | attributes-length(2)
//
//	| attributes(A) | announced(remainder)
//
// Attribute: flags(1) | code(1) | length(1 or 2) | value(length)
// MP_REACH:  AFI(2) | SAFI(1) | next-hop-length(1) | next-hop(N) | reserved(1) | NLRI
// MP_UNREACH: AFI(2) | SAFI(1) | NLRI
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets." Those bytes stay in their NLRI field.
func matchUpdateField(stream, needle string, field updateField) (matched, valid bool) {
	needle, attribute, hasAttribute := strings.Cut(needle, ":")
	var attributeCode uint8
	var attributeValue string
	if hasAttribute {
		var err error
		attributeCode, attributeValue, err = parseUpdateAttributeRule(attribute)
		if err != nil {
			return false, false
		}
	}
	attributeMatched := !hasAttribute
	if len(stream) < 2*HeaderLen {
		return false, false
	}
	if len(stream)%2 != 0 {
		return false, false
	}
	if !strings.EqualFold(stream[36:38], "02") {
		return false, true
	}
	length, err := strconv.ParseUint(stream[32:36], 16, 16)
	if err != nil {
		return false, false
	}
	if int(length)*2 != len(stream) {
		return false, false
	}
	body := stream[2*HeaderLen:]
	if len(body) < 8 {
		return false, false
	}
	withdrawn, err := strconv.ParseUint(body[:4], 16, 16)
	if err != nil {
		return false, false
	}
	attrOffset := 4 + 2*int(withdrawn)
	if attrOffset+4 > len(body) {
		return false, false
	}
	attrLength, err := strconv.ParseUint(body[attrOffset:attrOffset+4], 16, 16)
	if err != nil {
		return false, false
	}
	attrStart := attrOffset + 4
	attrEnd := attrStart + 2*int(attrLength)
	if attrEnd > len(body) {
		return false, false
	}
	switch field {
	case updateFieldUnspecified:
		return false, false
	case updateFieldAnnounced:
		matched = indexByteAligned(body[attrEnd:], needle, 0) >= 0
	case updateFieldWithdrawn:
		matched = indexByteAligned(body[4:attrOffset], needle, 0) >= 0
	default:
		panic("BUG: unknown scoped UPDATE field")
	}
	// Each iteration consumes one bounded attribute header and its value.
	for offset := attrStart; offset < attrEnd; {
		if attrEnd-offset < 6 {
			return false, false
		}
		flags, err := strconv.ParseUint(body[offset:offset+2], 16, 8)
		if err != nil {
			return false, false
		}
		code, err := strconv.ParseUint(body[offset+2:offset+4], 16, 8)
		if err != nil {
			return false, false
		}
		header := 6
		if flags&0x10 != 0 {
			header = 8
		}
		if offset+header > attrEnd {
			return false, false
		}
		size, err := strconv.ParseUint(body[offset+4:offset+header], 16, 16)
		if err != nil {
			return false, false
		}
		end := offset + header + 2*int(size)
		if end > attrEnd {
			return false, false
		}
		value := body[offset+header : end]
		if hasAttribute && code == uint64(attributeCode) {
			attributeMatched = attributeMatched || indexByteAligned(value, attributeValue, 0) >= 0
		}
		if code == 14 {
			if len(value) < 10 {
				return false, false
			}
			nextHop, err := strconv.ParseUint(value[6:8], 16, 8)
			if err != nil {
				return false, false
			}
			nlriOffset := 10 + 2*int(nextHop)
			if nlriOffset > len(value) {
				return false, false
			}
			if field == updateFieldAnnounced {
				matched = matched || indexByteAligned(value[nlriOffset:], needle, 0) >= 0
			}
		}
		if code == 15 {
			if len(value) < 6 {
				return false, false
			}
			if field == updateFieldWithdrawn {
				matched = matched || indexByteAligned(value[6:], needle, 0) >= 0
			}
		}
		offset = end
	}
	return matched && attributeMatched, true
}
