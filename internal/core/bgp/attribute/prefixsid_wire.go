// Design: docs/architecture/wire/attributes.md -- BGP Prefix-SID (attribute 40) wire decode and JSON
// RFC: rfc/full/rfc8669.txt Section 3 -- Label-Index TLV (type 1), Originator SRGB TLV (type 3)
// RFC: rfc/full/rfc9252.txt Sections 2, 3.1, 3.2.1 -- SRv6 L3/L2 Service TLV, SID Information
// Related: prefixsid.go -- EncodePrefixSID, the operator-text side of the same attribute
// Related: register.go -- where appendPrefixSIDJSON joins the JSON formatter registry
//
// Wire format. RFC 8669 Section 3 makes the attribute value a run of TLVs, and
// every nesting level below it repeats the same three-octet header, so one walk
// reads all three (RFC 9252 Sections 3 and 3.2):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Type      |            Length             |  Value       //
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	offset 0          1                               3
//
// Label-Index TLV value, type 1, length 7 (RFC 8669 Section 3.1):
//
//	+---------------+-------------------------------+
//	|   RESERVED    |             Flags             |
//	+---------------+-------------------------------+
//	offset 0          1
//	+---------------------------------------------------------------+
//	|                          Label Index                          |
//	+---------------------------------------------------------------+
//	offset 3
//
// Originator SRGB TLV value, type 3 (RFC 8669 Section 3.2): Flags(2) then one
// or more 6-octet ranges, each Base(3) followed by Range(3).
//
// SRv6 L3/L2 Service TLV value, types 5 and 6 (RFC 9252 Section 2): RESERVED(1)
// then Sub-TLVs. The SID Information Sub-TLV, type 1 (RFC 9252 Section 3.1):
//
//	+---------------+-----------------------------------------------+
//	|   RESERVED1   |    SRv6 SID Value (16 octets)                //|
//	+---------------+-----------------------------------------------+
//	offset 0          1
//	+---------------+-------------------------------+---------------+
//	| Svc SID Flags |   SRv6 Endpoint Behavior      |   RESERVED2   |
//	+---------------+-------------------------------+---------------+
//	offset 17         18                              20
//
// then Sub-Sub-TLVs. The SID Structure Sub-Sub-TLV, type 1, length 6 (RFC 9252
// Section 3.2.1): six 1-octet lengths, in bits, at offsets 0 through 5.

package attribute

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// Header and value sizes the two RFCs fix. Each one is quoted at the guard that
// enforces it.
const (
	prefixSIDHeaderLen = 3 // type(1) + length(2)

	prefixSIDLabelIndexLen    = 7 // RESERVED(1) + Flags(2) + Label Index(4)
	prefixSIDLabelIndexOffset = 3

	prefixSIDSRGBFlagsLen = 2 // Flags(2), then N ranges
	prefixSIDSRGBEntryLen = 6 // Base(3) + Range(3)
	prefixSIDSRGBBaseLen  = 3

	srv6ServiceReservedLen = 1 // RESERVED(1) ahead of the Sub-TLVs

	srv6SIDInfoLen        = 21 // RESERVED1(1) + SID(16) + Flags(1) + Behavior(2) + RESERVED2(1)
	srv6SIDValueOffset    = 1
	srv6SIDFlagsOffset    = 17
	srv6SIDBehaviorOffset = 18

	srv6SIDStructureLen = 6 // six 1-octet lengths in bits
)

// TLV type codes, from IANA's "BGP Prefix-SID TLV Types", "SRv6 Service Sub-TLV
// Types" and "SRv6 Service Data Sub-Sub-TLV Types" subregistries.
const (
	prefixSIDTLVLabelIndex     uint8 = 1
	prefixSIDTLVOriginatorSRGB uint8 = 3
	prefixSIDTLVSRv6L3Service  uint8 = 5
	prefixSIDTLVSRv6L2Service  uint8 = 6

	srv6SubTLVSIDInformation  uint8 = 1
	srv6SubSubTLVSIDStructure uint8 = 1
)

var errEmptyPrefixSIDAttribute = errors.New("prefix-sid: attribute carries no TLV")

// PrefixSID is the BGP Prefix-SID attribute (RFC 8669, code 40).
//
// It holds the attribute value exactly as the peer sent it, and decodes a field
// only where a reader asks for one. RFC 8669 Section 3 says "For future
// extensibility, unknown TLVs MUST be ignored and propagated unmodified", and
// RFC 9252 Section 2 says "all Reserved fields in the TLV, Sub-TLV, or
// Sub-Sub-TLV MUST be propagated unchanged". Rebuilding the value from decoded
// fields would satisfy neither, because a TLV ze does not know cannot be
// rebuilt at all, so the wire form stays the single copy of the fact.
//
// Every value of this type has passed validatePrefixSIDTLVs, so its framing is
// known good before any reader walks it.
type PrefixSID struct {
	tlvs []byte
}

var _ Attribute = (*PrefixSID)(nil)

// ParsePrefixSID parses the BGP Prefix-SID attribute value (RFC 8669, code 40).
//
// It fails closed: a broken frame or a TLV whose length contradicts its own RFC
// returns an error and no attribute, because a partly-read Prefix-SID cannot be
// told apart from one the peer meant to send (ai/rules/principles.md).
func ParsePrefixSID(data []byte) (*PrefixSID, error) {
	// RFC 8669 Section 3: "The BGP Prefix-SID attribute is defined here to be a
	// set of elements encoded as "Type/Length/Value" tuples (i.e., a set of
	// TLVs)." An empty set names no SID, so it is refused rather than answered
	// with an attribute that renders as nothing.
	if len(data) == 0 {
		return nil, errEmptyPrefixSIDAttribute
	}

	if err := validatePrefixSIDTLVs(data); err != nil {
		return nil, err
	}

	// The session read buffer is reused for the next message, so the attribute
	// owns its own copy of the bytes it will re-emit.
	tlvs := make([]byte, len(data))
	copy(tlvs, data)
	return &PrefixSID{tlvs: tlvs}, nil
}

func (p *PrefixSID) Code() AttributeCode { return AttrPrefixSID }

// Flags reports the categories RFC 8669 Section 3 assigns, where it says the
// BGP Prefix-SID attribute "is an optional, transitive BGP path attribute".
func (p *PrefixSID) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

func (p *PrefixSID) Len() int { return len(p.tlvs) }

// WriteTo re-emits the TLVs unchanged, which is what RFC 8669 Section 3 and RFC
// 9252 Section 2 require of a speaker that propagates the attribute.
func (p *PrefixSID) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], p.tlvs)
}

// WriteToWithContext writes the same bytes: no field in this attribute depends
// on the negotiated AS size or on any other session capability.
func (p *PrefixSID) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return p.WriteTo(buf, off)
}

// forEachPrefixSIDTLV walks a run of type(1) + length(2) + value TLVs and calls
// visit for each one. The level name goes into the error, so a truncation
// reports which nesting level broke.
//
// One function serves the TLVs of RFC 8669 Section 3, the SRv6 Service Sub-TLVs
// of RFC 9252 Section 3 and the Service Data Sub-Sub-TLVs of RFC 9252 Section
// 3.2, because all three carry that same header. The nesting is three levels
// deep and the grammar fixes it, so the walk is called from three explicit
// places rather than recursing to a depth the peer would choose.
func forEachPrefixSIDTLV(level string, data []byte, visit func(tlvType uint8, value []byte) error) error {
	for len(data) > 0 {
		if len(data) < prefixSIDHeaderLen {
			return fmt.Errorf("prefix-sid: %s header truncated (need %d, have %d)", level, prefixSIDHeaderLen, len(data))
		}
		tlvType := data[0]
		end := prefixSIDHeaderLen + int(binary.BigEndian.Uint16(data[1:3]))
		if len(data) < end {
			return fmt.Errorf("prefix-sid: %s type %d value truncated (need %d, have %d)",
				level, tlvType, end-prefixSIDHeaderLen, len(data)-prefixSIDHeaderLen)
		}
		if err := visit(tlvType, data[prefixSIDHeaderLen:end]); err != nil {
			return err
		}
		data = data[end:]
	}
	return nil
}

// validatePrefixSIDTLVs checks the framing of every level and the fixed lengths
// the RFCs state for the TLVs ze decodes.
func validatePrefixSIDTLVs(data []byte) error {
	return forEachPrefixSIDTLV("TLV", data, func(tlvType uint8, value []byte) error {
		switch tlvType {
		case prefixSIDTLVLabelIndex:
			// RFC 8669 Section 3.1: "Length: 7, the total length in octets of the
			// value portion of the TLV."
			if len(value) != prefixSIDLabelIndexLen {
				return fmt.Errorf("prefix-sid: label-index TLV length is %d, RFC 8669 Section 3.1 fixes it at %d",
					len(value), prefixSIDLabelIndexLen)
			}
			return nil
		case prefixSIDTLVOriginatorSRGB:
			// RFC 8669 Section 3.2: "Length: The total length in octets of the
			// value portion of the TLV: 2 + (non-zero multiple of 6)."
			if len(value) < prefixSIDSRGBFlagsLen+prefixSIDSRGBEntryLen || (len(value)-prefixSIDSRGBFlagsLen)%prefixSIDSRGBEntryLen != 0 {
				return fmt.Errorf("prefix-sid: originator-SRGB TLV length is %d, RFC 8669 Section 3.2 requires 2 + a non-zero multiple of 6",
					len(value))
			}
			return nil
		case prefixSIDTLVSRv6L3Service, prefixSIDTLVSRv6L2Service:
			return validateSRv6ServiceTLV(tlvType, value)
		default:
			// RFC 8669 Section 3: "For future extensibility, unknown TLVs MUST be
			// ignored and propagated unmodified." The framing above is all ze can
			// check, and the bytes stay whole for the reader and for the wire.
			return nil
		}
	})
}

// validateSRv6ServiceTLV checks an SRv6 L3 or L2 Service TLV (RFC 9252 Section 2).
func validateSRv6ServiceTLV(tlvType uint8, value []byte) error {
	// RFC 9252 Section 2: "RESERVED (1 octet): This field is reserved; it MUST
	// be set to 0 by the sender and ignored by the receiver." Ignored is what ze
	// does with its content, but the octet has to be there for the Sub-TLVs to
	// start where the format says they do.
	if len(value) < srv6ServiceReservedLen {
		return fmt.Errorf("prefix-sid: SRv6 service TLV type %d has no RESERVED octet", tlvType)
	}

	return forEachPrefixSIDTLV("SRv6 sub-TLV", value[srv6ServiceReservedLen:], func(subType uint8, subValue []byte) error {
		if subType != srv6SubTLVSIDInformation {
			return nil
		}
		return validateSRv6SIDInformation(subValue)
	})
}

// validateSRv6SIDInformation checks an SRv6 SID Information Sub-TLV (RFC 9252
// Section 3.1) and the Sub-Sub-TLVs it carries.
func validateSRv6SIDInformation(value []byte) error {
	// RFC 9252 Section 3.1 fixes the part ahead of the Sub-Sub-TLVs: RESERVED1
	// (1 octet), "SRv6 SID Value (16 octets)", "SRv6 Service SID Flags (1
	// octet)", "SRv6 Endpoint Behavior (2 octets)" and RESERVED2 (1 octet).
	if len(value) < srv6SIDInfoLen {
		return fmt.Errorf("prefix-sid: SRv6 SID information sub-TLV is %d octets, RFC 9252 Section 3.1 needs %d before its sub-sub-TLVs",
			len(value), srv6SIDInfoLen)
	}

	return forEachPrefixSIDTLV("SRv6 sub-sub-TLV", value[srv6SIDInfoLen:], func(subSubType uint8, subSubValue []byte) error {
		if subSubType != srv6SubSubTLVSIDStructure {
			return nil
		}
		// RFC 9252 Section 3.2.1: "SRv6 Service Data Sub-Sub-TLV Length (2
		// octets): This field contains a total length of 6 octets."
		if len(subSubValue) != srv6SIDStructureLen {
			return fmt.Errorf("prefix-sid: SRv6 SID structure sub-sub-TLV length is %d, RFC 9252 Section 3.2.1 fixes it at %d",
				len(subSubValue), srv6SIDStructureLen)
		}
		return nil
	})
}

// appendPrefixSIDJSON renders the attribute as the object ExaBGP publishes
// under "bgp-prefix-sid": one member for each TLV, named for the TLV.
//
// Without a formatter the generic arm names the attribute `attr-40` and prints
// its hex (appendAttributeJSON, component/bgp/format/text_json.go), which no
// reader can act on without decoding RFC 8669 by hand.
func appendPrefixSIDJSON(buf []byte, attr Attribute) []byte {
	// The POINTER, not the value type: knownAttrParsers stores what
	// ParsePrefixSID returns (wire.go). A formatter that asserts the other form
	// answers nil, and the reader silently gets attr-40 hex back with nothing
	// logged (plugins/rr/originator_json_test.go states the same trap).
	sid, ok := attr.(*PrefixSID)
	if !ok {
		return nil
	}

	buf = append(buf, '{')
	first := true
	err := forEachPrefixSIDTLV("TLV", sid.tlvs, func(tlvType uint8, value []byte) error {
		if !first {
			buf = append(buf, ',')
		}
		first = false

		switch tlvType {
		case prefixSIDTLVLabelIndex:
			buf = append(buf, `"sr-label-index":`...)
			buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint32(value[prefixSIDLabelIndexOffset:prefixSIDLabelIndexLen])), 10)
			return nil
		case prefixSIDTLVOriginatorSRGB:
			buf = appendPrefixSIDSRGBJSON(buf, value)
			return nil
		case prefixSIDTLVSRv6L3Service:
			return appendSRv6ServiceJSON(&buf, `"l3-service":[`, value)
		case prefixSIDTLVSRv6L2Service:
			return appendSRv6ServiceJSON(&buf, `"l2-service":[`, value)
		default:
			// The TLV is kept and named by its code, so an operator can read the
			// type ze does not decode and the hex that came with it. Dropping it
			// would leave the reader with no sign it was ever there.
			buf = append(buf, `"attribute-not-implemented-`...)
			buf = strconv.AppendUint(buf, uint64(tlvType), 10)
			buf = append(buf, `":"`...)
			buf = hex.AppendEncode(buf, value)
			buf = append(buf, '"')
			return nil
		}
	})
	if err != nil {
		return nil
	}
	return append(buf, '}')
}

// appendPrefixSIDSRGBJSON writes the Originator SRGB ranges as ExaBGP does,
// a list of [base, range] pairs (RFC 8669 Section 3.2).
func appendPrefixSIDSRGBJSON(buf, value []byte) []byte {
	buf = append(buf, `"sr-srgbs":[`...)
	for off := prefixSIDSRGBFlagsLen; off+prefixSIDSRGBEntryLen <= len(value); off += prefixSIDSRGBEntryLen {
		if off > prefixSIDSRGBFlagsLen {
			buf = append(buf, ',')
		}
		buf = append(buf, '[')
		buf = strconv.AppendUint(buf, uint64(uint24(value[off:])), 10)
		buf = append(buf, ',')
		buf = strconv.AppendUint(buf, uint64(uint24(value[off+prefixSIDSRGBBaseLen:])), 10)
		buf = append(buf, ']')
	}
	return append(buf, ']')
}

// uint24 reads the 3-octet big-endian form the SRGB base and range use.
func uint24(b []byte) uint32 {
	return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
}

// appendSRv6ServiceJSON writes one SRv6 L3 or L2 Service TLV as the array of
// its Sub-TLVs (RFC 9252 Section 2). The buffer is passed by pointer because
// the walk below appends from inside a closure.
func appendSRv6ServiceJSON(out *[]byte, key string, value []byte) error {
	*out = append(*out, key...)
	first := true
	err := forEachPrefixSIDTLV("SRv6 sub-TLV", value[srv6ServiceReservedLen:], func(subType uint8, subValue []byte) error {
		if !first {
			*out = append(*out, ',')
		}
		first = false

		if subType == srv6SubTLVSIDInformation {
			return appendSRv6SIDInformationJSON(out, subValue)
		}
		// ExaBGP's shape for a Sub-TLV it does not decode: the type code and the
		// bytes, so the reader sees what arrived.
		*out = append(*out, `{"type":`...)
		*out = strconv.AppendUint(*out, uint64(subType), 10)
		*out = append(*out, `,"raw":"`...)
		*out = hex.AppendEncode(*out, subValue)
		*out = append(*out, `"}`...)
		return nil
	})
	if err != nil {
		return err
	}
	*out = append(*out, ']')
	return nil
}

// appendSRv6SIDInformationJSON writes one SRv6 SID Information Sub-TLV (RFC
// 9252 Section 3.1).
//
// The flags member carries the octet that arrived. ExaBGP prints a literal 0
// here because it never reads the field; RFC 9252 Section 3.1 says the flags
// "MUST be set to 0 by the sender", so the two agree on every conformant
// sender and ze reports the peer that is not one.
func appendSRv6SIDInformationJSON(out *[]byte, value []byte) error {
	sid := netip.AddrFrom16([16]byte(value[srv6SIDValueOffset:srv6SIDFlagsOffset]))

	*out = append(*out, `{"sid":"`...)
	*out = sid.AppendTo(*out)
	*out = append(*out, `","flags":`...)
	*out = strconv.AppendUint(*out, uint64(value[srv6SIDFlagsOffset]), 10)
	*out = append(*out, `,"endpoint_behavior":`...)
	*out = strconv.AppendUint(*out, uint64(binary.BigEndian.Uint16(value[srv6SIDBehaviorOffset:srv6SIDBehaviorOffset+2])), 10)

	err := forEachPrefixSIDTLV("SRv6 sub-sub-TLV", value[srv6SIDInfoLen:], func(subSubType uint8, subSubValue []byte) error {
		*out = append(*out, ',')
		if subSubType == srv6SubSubTLVSIDStructure {
			*out = appendSRv6SIDStructureJSON(*out, subSubValue)
			return nil
		}
		// ExaBGP renders an unknown Sub-Sub-TLV as a bare object here, which is
		// not valid JSON in a member position, so ze names it by its code and
		// gives the hex. Nothing reads the ExaBGP form, because nothing can.
		*out = append(*out, `"sub-sub-tlv-`...)
		*out = strconv.AppendUint(*out, uint64(subSubType), 10)
		*out = append(*out, `":"`...)
		*out = hex.AppendEncode(*out, subSubValue)
		*out = append(*out, '"')
		return nil
	})
	if err != nil {
		return err
	}
	*out = append(*out, '}')
	return nil
}

// appendSRv6SIDStructureJSON writes the SID Structure Sub-Sub-TLV (RFC 9252
// Section 3.2.1). Every value is a length in bits.
func appendSRv6SIDStructureJSON(buf, value []byte) []byte {
	buf = append(buf, `"structure":{"locator-block-length":`...)
	buf = strconv.AppendUint(buf, uint64(value[0]), 10)
	buf = append(buf, `,"locator-node-length":`...)
	buf = strconv.AppendUint(buf, uint64(value[1]), 10)
	buf = append(buf, `,"function-length":`...)
	buf = strconv.AppendUint(buf, uint64(value[2]), 10)
	buf = append(buf, `,"argument-length":`...)
	buf = strconv.AppendUint(buf, uint64(value[3]), 10)
	buf = append(buf, `,"transposition-length":`...)
	buf = strconv.AppendUint(buf, uint64(value[4]), 10)
	buf = append(buf, `,"transposition-offset":`...)
	buf = strconv.AppendUint(buf, uint64(value[5]), 10)
	return append(buf, '}')
}
