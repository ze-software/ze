// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS attribute TLV framework
// RFC: rfc/short/rfc9552.md — Section 8.2.2 fault management for BGP-LS
// Related: rfc7606.go — the RFC 7606 engine and the attrValidators registry
// Related: ../reactor/session_validation.go — enforceRFC7606, the receive path that acts on the verdict

package message

// RFC 9552 syntactic validation of the BGP-LS Attribute (code 29) on the session path.
//
// This lives in its own file for the reason rfc7606_optional_attrs.go gives: rfc7606.go is
// already past the 1000-line limit. The entry taken in attrValidators below is the same one
// every other attribute code takes.

// attrCodeBGPLS is the BGP-LS Attribute. RFC 9552 Section 5.3: "The BGP-LS Attribute
// (assigned value 29 by IANA) is an optional, non-transitive BGP Attribute".
const attrCodeBGPLS uint8 = 29

// bgplsTLVHeaderLen is the BGP-LS TLV header, RFC 9552 Section 5.1 (Figure 4). The format
// applies to the NLRI and the BGP-LS Attribute alike, and the TLV "is not padded to 4-octet
// alignment", so a TLV occupies exactly bgplsTLVHeaderLen + Length octets.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|              Type             |             Length            |   offsets 0..3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	//                        Value (variable)                     //   offset 4
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
const bgplsTLVHeaderLen = 4

func init() {
	attrValidators[attrCodeBGPLS] = validateBGPLSAttr
}

// validateBGPLSAttr performs the RFC 9552 Section 8.2.2 syntactic validation of the BGP-LS
// Attribute. The section states: "A BGP-LS Speaker MUST perform the following syntactic
// validation of the BGP-LS Attribute to determine if it is malformed."
//
//   - The sum of all TLV lengths found in the BGP-LS Attribute
//     corresponds to the BGP-LS Attribute length.
//
//   - The syntactic correctness of the Attributes (including the BGP-LS
//     Attribute) have been verified as per [RFC7606].
//
//   - The length of each TLV and, when the TLV is recognized then, the
//     length of its sub-TLVs in the BGP-LS Attribute are valid.
//
// The walk below is the first bullet and the first half of the third. Both errors it can
// report are one fact stated two ways: a TLV whose declared length runs past the attribute,
// and a tail too short to hold another TLV header, each mean the TLV lengths do not sum to
// the attribute length.
//
// The second bullet is the generic RFC 7606 Section 4 framing check that
// ValidateUpdateRFC7606AddPath already ran: this validator is called only after the
// attribute's flags, type and length have been read and its value proved to fit inside the
// Total Path Attribute Length.
//
// Sub-TLVs are not walked, because the third bullet gates that on "when the TLV is
// recognized", and ze recognizes no BGP-LS Attribute TLV on the session path: the BGP-LS
// TLV decoders (internal/component/bgp/plugins/nlri/ls) are reached from AttrTLVsToJSON
// alone, whose only non-test caller is the offline decoder
// (internal/component/bgp/cli/decode_update.go). Recognizing a TLV here would also mean
// judging its fixed layout's length, and Section 8.2.2 lists "the length of a fixed-length
// TLV is correct or the length of a variable length TLV is valid or permissible" among the
// semantic validations a BGP-LS Propagator does not perform.
//
// The verdict is 'Attribute Discard' because Section 8.2.2 prescribes it for exactly the
// case this walk sees: "When the error that is determined allows for the router to skip the
// malformed BGP-LS Attribute and continue the processing of the rest of the BGP UPDATE
// message (e.g., when the BGP-LS Attribute length and the total Path Attribute Length are
// correct but some TLV/sub-TLV length within the BGP-LS Attribute is invalid), then it MUST
// handle such malformed BGP-LS Attribute as 'Attribute Discard'." The stronger cases the
// same paragraph names -- an error that leaves the UPDATE unprocessable -- are the framing
// errors the outer walk catches before this validator is reached.
//
// Discarding the whole attribute is what Section 8.2.2 asks for: "the 'Attribute Discard'
// action results in the loss of all TLVs in the BGP-LS Attribute and not the removal of a
// specific malformed TLV", because removing one TLV "may give a wrong indication to a BGP-LS
// Consumer of that specific information being deleted or not available".
//
// No TLV type, and no byte of any TLV value, is read. Section 8.2.2 opens by forbidding it:
// "A BGP-LS Attribute MUST NOT be considered malformed or invalid based on the inclusion/
// exclusion of TLVs or contents of the TLV fields (i.e., semantic errors)".
func validateBGPLSAttr(code uint8, length int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	// The walk is bounded by the attribute length, which the caller read from the wire and
	// bounds-checked against the path-attributes section. Every iteration advances off by at
	// least bgplsTLVHeaderLen, so a peer cannot stall it.
	off := 0
	for off < length {
		if off+bgplsTLVHeaderLen > length {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionAttributeDiscard,
				AttrCode:    code,
				Reason:      DiscardReasonInvalidLength,
				Description: "RFC 9552 Section 8.2.2: BGP-LS Attribute ends inside a TLV header",
			}
		}
		tlvLen := int(attrData[off+2])<<8 | int(attrData[off+3])
		off += bgplsTLVHeaderLen
		if off+tlvLen > length {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionAttributeDiscard,
				AttrCode:    code,
				Reason:      DiscardReasonInvalidLength,
				Description: "RFC 9552 Section 8.2.2: BGP-LS Attribute TLV length exceeds the attribute length",
			}
		}
		off += tlvLen
	}
	return nil
}
