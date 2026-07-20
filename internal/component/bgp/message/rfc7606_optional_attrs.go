// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc7606.md — revised error handling for UPDATE messages
// Related: rfc7606.go — the RFC 7606 engine and the attrValidators registry

package message

import (
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// RFC 7606 validation for three optional attributes that previously had no validator:
// Traffic Engineering (RFC 5543), IPv6 Address Specific Extended Community (RFC 5701) and
// ATTR_SET (RFC 6368). attrValidators is opt-in -- validateAttribute returns nil for an
// unregistered code -- so their absence meant ANY length was accepted, which is why
// Sections 7.13, 7.15 and 7.16 were disclosed as gaps rather than enforced.
//
// These live in their own file because rfc7606.go is already at the 1000-line limit.

// Attribute type codes validated here.
const (
	attrCodeTrafficEng  uint8 = 24  // RFC 5543 Traffic Engineering
	attrCodeIPv6ExtComm uint8 = 25  // RFC 5701 IPv6 Address Specific Extended Community
	attrCodeAttrSet     uint8 = 128 // RFC 6368 ATTR_SET
)

// attrFlagExtendedLn is RFC 4271 Section 4.3 bit 3: the Attribute Length is 2 octets.
// The outer walk in ValidateUpdateRFC7606 spells this 0x10 inline; ATTR_SET's inner
// attribute stream obeys the same encoding rules (RFC 6368 Section 5) and needs it too.
const attrFlagExtendedLn uint8 = 0x10

const (
	// RFC 5543 Section 3: one Traffic Engineering descriptor is Switching Cap(1) +
	// Encoding(1) + Reserved(2) + eight 4-octet Max LSP Bandwidth values.
	trafficEngDescriptorLen = 4 + 8*4

	// RFC 5701 Section 2: Type(1) Sub-Type(1) Global Administrator(16) Local Admin(2).
	ipv6ExtCommunityLen = 20

	// RFC 6368 Section 5: the ATTR_SET value opens with a 4-octet Origin AS.
	attrSetOriginASLen = 4

	// ATTR_SET may carry "any BGP attribute" (RFC 6368 Section 5), which includes another
	// ATTR_SET. A peer therefore controls the nesting depth, so the recursion is capped:
	// without a cap a crafted UPDATE could exhaust the stack. Four is far beyond any
	// legitimate PE/CE stacking, where one level is the design.
	attrSetMaxDepth = 4
)

func init() {
	attrValidators[attrCodeTrafficEng] = validateTrafficEngineeringAttr
	attrValidators[attrCodeIPv6ExtComm] = validateIPv6ExtCommunityAttr
	attrValidators[attrCodeAttrSet] = validateAttrSetAttr
}

// RFC 7606 Section 7.13: "an implementation that determines (for whatever reason) that an
// UPDATE message contains a malformed Traffic Engineering path attribute MUST handle it
// using the approach of 'treat-as-withdraw'."
//
// The check is deliberately minimal. RFC 7606 notes that RFC 5543 "does not detail what
// constitutes malformation", and binds only an implementation that has already decided the
// attribute is malformed -- it grants no license to invent criteria. RFC 5543 Section 3
// says the attribute "contains one or more" descriptors, each with 36 fixed octets before
// its variable switching-capability-specific information, whose length is defined per
// capability by RFC 4203/5307. Anything shorter than one descriptor cannot be well-formed
// under any switching capability; anything longer might be, so it is accepted. Rejecting
// more would blackhole valid routes, which is worse than under-validating an attribute ze
// does not act on.
func validateTrafficEngineeringAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length < trafficEngDescriptorLen {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:   RFC7606ActionTreatAsWithdraw,
			AttrCode: code,
			Description: b.Reset().Str("RFC 7606 Section 7.13: Traffic Engineering length ").
				Int(int64(length)).Str(" shorter than one RFC 5543 descriptor (").
				Int(trafficEngDescriptorLen).Str(")").String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.15: "The IPv6 Address Specific Extended Community attribute SHALL be
// considered malformed if its length is not a non-zero multiple of 20", and an UPDATE
// carrying a malformed one "SHALL be handled using the approach of 'treat-as-withdraw'".
//
// The attribute VALUE is deliberately ignored (the parameter is `_`). The same section
// says "a BGP speaker MUST NOT treat an unrecognized IPv6 Address Specific Extended
// Community Type or Sub-Type as an error", so reading Type or Sub-Type here could only
// ever be used to reject something the RFC forbids rejecting. Length is the whole test.
//
// Zero is malformed by the "non-zero" half: 0 % 20 == 0 would pass a multiple-of-20 test
// on its own.
func validateIPv6ExtCommunityAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length == 0 || length%ipv6ExtCommunityLen != 0 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:   RFC7606ActionTreatAsWithdraw,
			AttrCode: code,
			Description: b.Reset().Str("RFC 7606 Section 7.15: IPv6 Address Specific Extended Community length ").
				Int(int64(length)).Str(" not a non-zero multiple of ").
				Int(ipv6ExtCommunityLen).String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.16 replaces RFC 6368 Section 5's final paragraph: "An UPDATE message
// with a malformed ATTR_SET attribute SHALL be handled using the approach of 'treat as
// withdraw'." Only the ACTION changes; RFC 6368 Section 5 still defines malformed:
//
//	o  Its length is less than 4 octets.
//	o  The original path attributes carried in the variable-length attribute data
//	   include the MP_REACH or MP_UNREACH attribute.
//	o  The included attributes are malformed themselves.
//
// The outer session's isIBGP/asn4 are deliberately unused: see the inner-context note in
// validateAttrSetDepth. Taking them and ignoring them is the point -- the signature is
// fixed by attrValidatorFn.
func validateAttrSetAttr(code uint8, length int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	return validateAttrSetDepth(code, length, attrData, 0)
}

func validateAttrSetDepth(
	code uint8, length int, attrData []byte, depth int,
) *RFC7606ValidationResult {
	malformed := func(why string, n int64) *RFC7606ValidationResult {
		var b textbuf.Buffer
		b.Reset().Str("RFC 7606 Section 7.16: malformed ATTR_SET, ").Str(why)
		if n >= 0 {
			b.Str(" (").Int(n).Str(")")
		}
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.String(),
		}
	}

	if depth >= attrSetMaxDepth {
		return malformed("nested deeper than the supported limit of", attrSetMaxDepth)
	}
	if length < attrSetOriginASLen {
		return malformed("length shorter than the 4-octet Origin AS", int64(length))
	}
	// Defensive: the caller slices attrData to exactly `length`, but a short slice must
	// never be indexed past its end (this is the receive path for peer-controlled bytes).
	if len(attrData) < attrSetOriginASLen {
		return malformed("attribute data shorter than the 4-octet Origin AS", int64(len(attrData)))
	}

	// RFC 6368 Section 5: "a 4-octet 'Origin AS' value followed by a variable-length field
	// that conforms to the BGP UPDATE message path attribute encoding rules."
	pos := attrSetOriginASLen
	for pos < len(attrData) {
		if pos+2 > len(attrData) {
			return malformed("truncated inner attribute header", -1)
		}
		flags := attrData[pos]
		innerCode := attrData[pos+1]
		pos += 2

		var innerLen int
		if flags&attrFlagExtendedLn != 0 {
			if pos+2 > len(attrData) {
				return malformed("truncated inner extended length", -1)
			}
			innerLen = int(attrData[pos])<<8 | int(attrData[pos+1])
			pos += 2
		} else {
			if pos+1 > len(attrData) {
				return malformed("truncated inner length", -1)
			}
			innerLen = int(attrData[pos])
			pos++
		}
		if pos+innerLen > len(attrData) {
			return malformed("inner attribute length exceeds the remaining data", int64(innerLen))
		}
		innerData := attrData[pos : pos+innerLen]
		pos += innerLen

		// RFC 6368 Section 5: an ATTR_SET "can include any BGP attribute that can occur in
		// a BGP UPDATE message, except for the MP_REACH and MP_UNREACH attributes", and
		// their presence is itself a malformed condition. Note they are rejected on
		// PRESENCE, never validated as inner attributes: their validators assume the
		// top-level framing that locates an UPDATE's NLRI.
		if innerCode == attrCodeMPReachNLRI || innerCode == attrCodeMPUnreachNLRI {
			return malformed("contains an MP_REACH/MP_UNREACH attribute", int64(innerCode))
		}

		// "The included attributes are malformed themselves." Same definition, so the same
		// code: recurse for a nested ATTR_SET (depth-capped), otherwise reuse the ordinary
		// per-attribute validation.
		//
		// The inner stream is judged in ITS OWN context, never the outer session's:
		//
		//   - asn4 is forced true. RFC 6368 Section 5: "The AS_PATH and AGGREGATOR
		//     attributes contained within an ATTR_SET attribute MUST be encoded using
		//     4-octet AS numbers, regardless of the capabilities advertised by the BGP
		//     speaker to which the ATTR_SET attribute is transmitted." Passing the
		//     session's asn4 made validateASPathAttr read a conforming 4-octet inner
		//     AS_PATH with asSize 2 on a session that never negotiated RFC 6793, and
		//     withdraw the route for being malformed when it was not.
		//   - isIBGP is forced true. An ATTR_SET exists to carry the CUSTOMER's iBGP
		//     attributes across the provider (RFC 6368 is "Internal BGP as PE/CE
		//     Protocol"), so LOCAL_PREF, ORIGINATOR_ID and CLUSTER_LIST are legitimate
		//     inside it. Judging them with the outer session's eBGP context withdrew any
		//     route whose ATTR_SET carried the customer's LOCAL_PREF.
		if innerCode == attrCodeAttrSet {
			if r := validateAttrSetDepth(code, innerLen, innerData, depth+1); r != nil {
				return r
			}
			continue
		}
		// Only a MALFORMED inner attribute makes the ATTR_SET malformed. RFC 7606 assigns
		// "attribute discard" to AGGREGATOR (7.7), LOCAL_PREF from eBGP (7.5),
		// ORIGINATOR_ID (7.9) and CLUSTER_LIST (7.10) precisely so the route survives;
		// escalating those to a whole-UPDATE withdraw would invert that choice.
		if r := validateAttribute(innerCode, innerLen, innerData, true, true); r != nil &&
			r.Action >= RFC7606ActionTreatAsWithdraw {
			return malformed("an included attribute is itself malformed, code", int64(innerCode))
		}
	}
	return nil
}
