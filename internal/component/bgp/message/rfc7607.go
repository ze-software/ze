// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc7607.md — Codification of AS 0 Processing
// Related: rfc7606.go — the attribute walk this registers into, and the AS_PATH and
// AGGREGATOR validators that carry the other half of RFC 7607 Section 2
//
// RFC 7607 proscribes AS 0 in four attributes. AS_PATH and AGGREGATOR already have a
// validator in rfc7606.go and the AS 0 test was added there, beside the structural
// tests of the same attribute. AS4_PATH and AS4_AGGREGATOR had no validator at all,
// so this file registers theirs.

package message

import "github.com/ze-software/ze/internal/core/textbuf"

// Attribute type codes RFC 6793 Section 9 assigns to the four-octet AS attributes.
const (
	attrCodeAS4Path       uint8 = 17
	attrCodeAS4Aggregator uint8 = 18
)

// as4AggregatorASOctets is the width of the AS field that leads AS4_AGGREGATOR. The
// attribute exists to carry a four-octet AS, so the width is fixed rather than
// negotiated (RFC 6793 Section 3).
const as4AggregatorASOctets = 4

func init() {
	attrValidators[attrCodeAS4Path] = validateAS4PathAttr
	attrValidators[attrCodeAS4Aggregator] = validateAS4AggregatorAttr
}

// validateAS4PathAttr rejects AS 0 in AS4_PATH.
//
// RFC 7607 Section 2: "An UPDATE message that contains the AS number of zero in the
// AS4_PATH or AS4_AGGREGATOR attribute MUST be considered as malformed and be handled
// by the procedures specified in [RFC6793]."
//
// RFC 6793 Section 6 names that procedure: "the 'attribute discard' approach is chosen
// to handle a malformed AS4_PATH attribute."
//
// This answers the AS 0 question only. A structurally inconsistent AS4_PATH is a
// different malformation, owned by RFC 6793 Section 6's own length and segment
// conditions, and reported nowhere on this rail today. Widening this validator to
// report it would close that gap silently, under an RFC that does not ask for it.
func validateAS4PathAttr(code uint8, _ int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	if !asPathHoldsASZero(attrData, as4AggregatorASOctets) {
		return nil
	}
	var b textbuf.Buffer
	return &RFC7606ValidationResult{
		Action:      RFC7606ActionAttributeDiscard,
		AttrCode:    code,
		Reason:      DiscardReasonMalformedValue,
		Description: b.Reset().Str("RFC 7607 Section 2: AS 0 in AS4_PATH").String(),
	}
}

// validateAS4AggregatorAttr rejects AS 0 in AS4_AGGREGATOR.
//
// RFC 7607 Section 2 makes the attribute malformed, and RFC 6793 Section 6 names the
// procedure: "the 'attribute discard' approach is chosen to handle a malformed
// AS4_AGGREGATOR attribute."
//
// The attribute length is not policed here. RFC 6793 Section 6 owns that condition
// ("malformed if the attribute length is not 8") and no receive path enforces it, which
// rfc/short/rfc6793.md records under RFC6793-6-5. The bound below is what this function
// needs to read the AS field, not a length verdict.
func validateAS4AggregatorAttr(code uint8, _ int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	if len(attrData) < as4AggregatorASOctets {
		return nil
	}
	if !asnIsZero(attrData[:as4AggregatorASOctets]) {
		return nil
	}
	var b textbuf.Buffer
	return &RFC7606ValidationResult{
		Action:      RFC7606ActionAttributeDiscard,
		AttrCode:    code,
		Reason:      DiscardReasonMalformedValue,
		Description: b.Reset().Str("RFC 7607 Section 2: AS 0 in AS4_AGGREGATOR").String(),
	}
}

// asPathHoldsASZero reports whether any AS number in AS_PATH-shaped segment data is
// zero. asnOctets is 2 for a two-octet encoding and 4 for a four-octet one.
//
// Segment data is <type><count> followed by count AS numbers, repeated. The walk is
// bounded by len(data), which the attribute length bounds, and pos rises by at least
// two on every iteration, so the loop cannot run forever on hostile input.
//
// It stops at the first structural inconsistency and reports no zero. A structural
// fault is a different malformation with a different owner: RFC 7606 Section 7.2 for
// AS_PATH, RFC 6793 Section 6 for AS4_PATH. This function answers one question.
func asPathHoldsASZero(data []byte, asnOctets int) bool {
	pos := 0
	for pos+2 <= len(data) {
		count := int(data[pos+1])
		pos += 2

		end := pos + count*asnOctets
		if end > len(data) {
			return false
		}
		for ; pos < end; pos += asnOctets {
			if asnIsZero(data[pos : pos+asnOctets]) {
				return true
			}
		}
	}
	return false
}

// asnIsZero reports whether an AS number, in its two-octet or four-octet wire form, is
// zero. Comparing the octets avoids branching on the width to pick a decoder.
func asnIsZero(asn []byte) bool {
	for _, octet := range asn {
		if octet != 0 {
			return false
		}
	}
	return true
}
