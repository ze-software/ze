// Design: docs/architecture/wire/attributes.md -- BGP Prefix-SID (attribute 40) receive actions per family
// RFC: rfc/full/rfc8669.txt Sections 3.1, 3.2, 4.1, 6 -- TLV lengths, invalid attribute, error handling
// RFC: rfc/full/rfc9252.txt Section 7 -- malformed SRv6 Service TLV, treat-as-withdraw
// Related: prefixsid_wire.go -- ParsePrefixSID, the family-independent framing rules this file reuses
//
// The receive path judges a Prefix-SID value once, with the family of the
// routes it is attached to. The family matters because RFC 8669 defines the
// Label-Index TLV (type 1) and the Originator SRGB TLV (type 3) only for IPv4
// and IPv6 labeled unicast, and its Section 6 error handling binds only there.
// Everywhere else those two types are unrecognized TLVs, propagated unmodified
// whatever their length. The SRv6 Service TLVs (types 5 and 6) are judged the
// same way on every family.
//
//	TLV:  +------+-----------+---------------+
//	      | Type | Length(2) | Value (Length)|
//	      +------+-----------+---------------+
//	      offset 0  1           3

package attribute

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// PrefixSIDVerdict is the receive-side judgement of one Prefix-SID value. The
// zero value is Unspecified, which no classification returns.
type PrefixSIDVerdict uint8

const (
	// PrefixSIDVerdictUnspecified is the zero value, never an answer.
	PrefixSIDVerdictUnspecified PrefixSIDVerdict = iota
	// PrefixSIDWellFormed: keep the attribute.
	PrefixSIDWellFormed
	// PrefixSIDMalformedAttribute: RFC 8669 Section 6 malformed attribute,
	// handled by attribute-discard.
	PrefixSIDMalformedAttribute
	// PrefixSIDMalformedServiceTLV: at least one malformed SRv6 Service TLV,
	// handled by treat-as-withdraw (RFC 9252 Section 7).
	PrefixSIDMalformedServiceTLV
	// PrefixSIDInvalidAttribute: RFC 8669 Section 4.1 invalid attribute (no
	// Label-Index TLV on labeled unicast), handled by attribute-discard.
	PrefixSIDInvalidAttribute
)

// The two framing verdicts of RFC 8669 Section 6 keep the wording the receive
// walk has always logged, so an operator's log search keeps matching.
var (
	errPrefixSIDTLVOverrun     = errors.New("RFC 8669 Section 6: Prefix-SID TLV length exceeds attribute bounds")
	errPrefixSIDTrailingOctets = errors.New("RFC 8669 Section 6: trailing bytes in Prefix-SID attribute")
	errPrefixSIDNoLabelIndex   = errors.New("RFC 8669 Section 4.1: Prefix-SID attached to labeled unicast carries no Label-Index TLV")
)

// PrefixSIDLabeledUnicast reports whether afi/safi is IPv4 or IPv6 Labeled
// Unicast (RFC 8277), the only families RFC 8669 Section 3.1 and Section 3.2
// define the Label-Index and Originator SRGB TLVs for.
func PrefixSIDLabeledUnicast(afi AFI, safi SAFI) bool {
	if safi != SAFIMPLSLabel {
		return false
	}
	return afi == AFIIPv4 || afi == AFIIPv6
}

// PrefixSIDRouteFamily returns the family of the routes the UPDATE whose
// attribute section is attrs carries a Prefix-SID for: the MP_REACH_NLRI family
// (RFC 4760 Section 3), or IPv4 unicast for an UPDATE without one, whose routes
// travel in the RFC 4271 NLRI field. It returns false for an MP_REACH_NLRI
// shorter than its AFI and SAFI, and a caller MUST NOT read a family then.
//
// It is the one declaration of that reading: the RFC 7606 receive step, the
// repeated-TLV discard and the egress Label-Index clear all call it.
//
// An UPDATE carrying both is read by its MP_REACH_NLRI family. The UPDATE has
// one Prefix-SID for both sets of routes, so one reading must win, and the
// labeled-unicast routes are the ones RFC 8669 Section 3.1 requires the
// Label-Index TLV for.
func PrefixSIDRouteFamily(attrs []byte) (AFI, SAFI, bool) {
	_, _, reach, found := AttrFind(attrs, AttrMPReachNLRI)
	if !found {
		return AFIIPv4, SAFIUnicast, true
	}
	if len(reach) < 3 {
		return 0, 0, false
	}
	return AFI(binary.BigEndian.Uint16(reach)), SAFI(reach[2]), true
}

// ClassifyPrefixSID judges a received Prefix-SID value attached to routes of
// family afi/safi. The error describes every verdict other than
// PrefixSIDWellFormed and is nil for it. The accept path allocates nothing.
//
// RFC 8669 Section 6: "a malformed BGP Prefix-SID attribute is one that cannot
// be parsed due to not meeting the minimum attribute length requirement,
// containing a TLV length that doesn't conform to the length constraints for
// the TLV, or containing a TLV length that would extend beyond the end of the
// attribute."
//
// RFC 9252 Section 7: "The treat-as-withdraw action [RFC7606] MUST be performed
// when at least one malformed SRv6 Service TLV is present in the BGP Prefix-SID
// attribute." A Service TLV is malformed when "The TLV Length is inconsistent
// with the length of the BGP Prefix-SID attribute", so the type octet decides
// the action before the length does. Off labeled unicast, a framing error met
// after a Service TLV was read is that Service TLV's length disagreeing with the
// attribute: the attribute holds the Service SID needed for installation, and
// RFC 7606 Section 2 forbids attribute-discard of an attribute that has an
// effect on installation.
//
// A length error in a type 1 or type 3 TLV leaves the framing intact, so the
// walk goes on: a malformed Service TLV after it is the stronger action (RFC
// 7606 Section 3.h).
func ClassifyPrefixSID(value []byte, afi AFI, safi SAFI) (PrefixSIDVerdict, error) {
	// RFC 8669 Section 3: the attribute is "a set of TLVs"; an empty one fails
	// the minimum attribute length on every family (it names no TLV, and
	// ParsePrefixSID refuses it, which would break every reader).
	if len(value) == 0 {
		return PrefixSIDMalformedAttribute, errEmptyPrefixSIDAttribute
	}

	labeled := PrefixSIDLabeledUnicast(afi, safi)
	escalate := false // off labeled unicast, a Service TLV has been read
	sawLabelIndex := false
	var discard error

	data := value
	for len(data) > 0 {
		if len(data) < prefixSIDHeaderLen {
			if escalate {
				return PrefixSIDMalformedServiceTLV, errServiceTLVLengthInconsistent(0)
			}
			return PrefixSIDMalformedAttribute, errPrefixSIDTrailingOctets
		}
		tlvType := data[0]
		end := prefixSIDHeaderLen + int(binary.BigEndian.Uint16(data[1:3]))
		service := tlvType == PrefixSIDTLVSRv6L3Service || tlvType == PrefixSIDTLVSRv6L2Service
		if len(data) < end {
			if service {
				return PrefixSIDMalformedServiceTLV, errServiceTLVLengthInconsistent(tlvType)
			}
			if escalate {
				return PrefixSIDMalformedServiceTLV, errServiceTLVLengthInconsistent(0)
			}
			return PrefixSIDMalformedAttribute, errPrefixSIDTLVOverrun
		}
		tlvValue := data[prefixSIDHeaderLen:end]
		data = data[end:]

		if service {
			if err := validateSRv6ServiceTLV(tlvType, tlvValue); err != nil {
				return PrefixSIDMalformedServiceTLV, fmt.Errorf("RFC 9252 Section 7: malformed SRv6 Service TLV: %w", err)
			}
			escalate = !labeled
			continue
		}
		if !labeled {
			// RFC 8669 Section 3.1: the Label-Index TLV "MUST be ignored when
			// received for other BGP AFI/SAFI combinations", and Section 3.2 says
			// the same of the Originator SRGB TLV. Unrecognized here, so its
			// length is not judged (RFC 9252 Section 7: a TLV "is not considered
			// malformed because its Type is unrecognized").
			continue
		}
		switch tlvType {
		case prefixSIDTLVLabelIndex:
			sawLabelIndex = true
			// RFC 8669 Section 3.1: "Length: 7, the total length in octets of the
			// value portion of the TLV."
			if len(tlvValue) != prefixSIDLabelIndexLen && discard == nil {
				discard = fmt.Errorf("RFC 8669 Section 6: label-index TLV length is %d, RFC 8669 Section 3.1 fixes it at %d",
					len(tlvValue), prefixSIDLabelIndexLen)
			}
		case prefixSIDTLVOriginatorSRGB:
			// RFC 8669 Section 3.2: "Length: The total length in octets of the
			// value portion of the TLV: 2 + (non-zero multiple of 6)."
			if !prefixSIDSRGBLengthValid(len(tlvValue)) && discard == nil {
				discard = fmt.Errorf("RFC 8669 Section 6: originator-SRGB TLV length is %d, RFC 8669 Section 3.2 requires 2 + a non-zero multiple of 6",
					len(tlvValue))
			}
		}
	}

	if discard != nil {
		return PrefixSIDMalformedAttribute, discard
	}
	// RFC 8669 Section 4.1: "A BGP Prefix-SID attribute received without a
	// Label-Index TLV MUST be considered to be "invalid" by the receiving
	// speaker." Section 6 handles an invalid attribute as a malformed one on
	// labeled unicast.
	if labeled && !sawLabelIndex {
		return PrefixSIDInvalidAttribute, errPrefixSIDNoLabelIndex
	}
	return PrefixSIDWellFormed, nil
}

// errServiceTLVLengthInconsistent describes an SRv6 Service TLV whose length
// disagrees with the attribute. tlvType 0 names the framing error that follows
// an earlier Service TLV rather than the Service TLV that overruns.
func errServiceTLVLengthInconsistent(tlvType uint8) error {
	if tlvType == 0 {
		return errors.New("RFC 9252 Section 7: SRv6 Service TLV length is inconsistent with the length of the BGP Prefix-SID attribute (octets left after it do not frame a TLV)")
	}
	return fmt.Errorf("RFC 9252 Section 7: SRv6 Service TLV type %d length is inconsistent with the length of the BGP Prefix-SID attribute", tlvType)
}

// prefixSIDSRGBLengthValid reports whether n octets is an Originator SRGB TLV
// value length RFC 8669 Section 3.2 allows: 2 + a non-zero multiple of 6.
func prefixSIDSRGBLengthValid(n int) bool {
	if n < prefixSIDSRGBFlagsLen+prefixSIDSRGBEntryLen {
		return false
	}
	return (n-prefixSIDSRGBFlagsLen)%prefixSIDSRGBEntryLen == 0
}
