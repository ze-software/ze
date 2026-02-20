// Design: docs/architecture/wire/attributes.md — path attribute encoding
//
// Package attribute implements BGP path attributes.
//
// RFC 4271 Section 4.3 defines the path attribute format:
//   - Attribute Type: 2 octets (flags + type code)
//   - Attribute Length: 1 or 2 octets (based on Extended Length flag)
//   - Attribute Value: variable
//
// RFC 4271 Section 5 defines path attribute semantics and categories:
//   - Well-known mandatory
//   - Well-known discretionary
//   - Optional transitive
//   - Optional non-transitive
package attribute

import (
	"encoding/binary"
	"errors"
	"fmt"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
)

// Errors for attribute parsing and validation.
// RFC 4271 Section 6.3 specifies UPDATE message error handling.
var (
	ErrShortData      = errors.New("attribute: short data")
	ErrInvalidLength  = errors.New("attribute: invalid length")
	ErrUnknownCode    = errors.New("attribute: unknown code")
	ErrMalformedValue = errors.New("attribute: malformed value")
	// ErrMalformedASPath indicates a malformed AS_PATH attribute.
	// RFC 4271 Section 6.3: Error Subcode 11 - Malformed AS_PATH.
	// Returned when segment type is invalid (not 1-4) or path exceeds max length.
	ErrMalformedASPath = errors.New("attribute: malformed AS_PATH")
)

// AttributeCode identifies a path attribute type (RFC 4271 Section 5).
type AttributeCode uint8

// Path attribute type codes.
// RFC 4271 Section 4.3 defines type codes 1-7.
// RFC 4271 Section 5 defines attribute semantics.
const (
	AttrOrigin           AttributeCode = 1  // RFC 4271 Section 4.3a, 5.1.1 - well-known mandatory
	AttrASPath           AttributeCode = 2  // RFC 4271 Section 4.3b, 5.1.2 - well-known mandatory
	AttrNextHop          AttributeCode = 3  // RFC 4271 Section 4.3c, 5.1.3 - well-known mandatory
	AttrMED              AttributeCode = 4  // RFC 4271 Section 4.3d, 5.1.4 - optional non-transitive
	AttrLocalPref        AttributeCode = 5  // RFC 4271 Section 4.3e, 5.1.5 - well-known (IBGP only)
	AttrAtomicAggregate  AttributeCode = 6  // RFC 4271 Section 4.3f, 5.1.6 - well-known discretionary
	AttrAggregator       AttributeCode = 7  // RFC 4271 Section 4.3g, 5.1.7 - optional transitive
	AttrCommunity        AttributeCode = 8  // RFC 1997
	AttrOriginatorID     AttributeCode = 9  // RFC 4456
	AttrClusterList      AttributeCode = 10 // RFC 4456
	AttrMPReachNLRI      AttributeCode = 14 // RFC 4760
	AttrMPUnreachNLRI    AttributeCode = 15 // RFC 4760
	AttrExtCommunity     AttributeCode = 16 // RFC 4360
	AttrAS4Path          AttributeCode = 17 // RFC 6793
	AttrAS4Aggregator    AttributeCode = 18 // RFC 6793
	AttrPMSI             AttributeCode = 22 // RFC 6514
	AttrTunnelEncap      AttributeCode = 23 // RFC 5512
	AttrIPv6ExtCommunity AttributeCode = 25 // RFC 5701
	AttrAIGP             AttributeCode = 26 // RFC 7311
	AttrBGPLS            AttributeCode = 29 // RFC 7752
	AttrLargeCommunity   AttributeCode = 32 // RFC 8092
	AttrPrefixSID        AttributeCode = 40 // RFC 8669
)

var attrCodeNames = map[AttributeCode]string{
	AttrOrigin:           "ORIGIN",
	AttrASPath:           "AS_PATH",
	AttrNextHop:          "NEXT_HOP",
	AttrMED:              "MULTI_EXIT_DISC",
	AttrLocalPref:        "LOCAL_PREF",
	AttrAtomicAggregate:  "ATOMIC_AGGREGATE",
	AttrAggregator:       "AGGREGATOR",
	AttrCommunity:        "COMMUNITIES",
	AttrOriginatorID:     "ORIGINATOR_ID",
	AttrClusterList:      "CLUSTER_LIST",
	AttrMPReachNLRI:      "MP_REACH_NLRI",
	AttrMPUnreachNLRI:    "MP_UNREACH_NLRI",
	AttrExtCommunity:     "EXTENDED_COMMUNITIES",
	AttrAS4Path:          "AS4_PATH",
	AttrAS4Aggregator:    "AS4_AGGREGATOR",
	AttrPMSI:             "PMSI_TUNNEL",
	AttrTunnelEncap:      "TUNNEL_ENCAPSULATION",
	AttrIPv6ExtCommunity: "IPV6_EXTENDED_COMMUNITIES",
	AttrAIGP:             "AIGP",
	AttrBGPLS:            "BGP_LS",
	AttrLargeCommunity:   "LARGE_COMMUNITIES",
	AttrPrefixSID:        "PREFIX_SID",
}

// String returns the attribute name.
func (c AttributeCode) String() string {
	if name, ok := attrCodeNames[c]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", c)
}

// AttributeFlags are the attribute flags (RFC 4271 Section 4.3).
//
// RFC 4271 Section 4.3 defines the Attribute Flags octet:
//   - Bit 0 (0x80): Optional bit - 1=optional, 0=well-known
//   - Bit 1 (0x40): Transitive bit - 1=transitive, 0=non-transitive
//   - Bit 2 (0x20): Partial bit - 1=partial, 0=complete
//   - Bit 3 (0x10): Extended Length bit - 1=2-octet length, 0=1-octet length
//   - Bits 4-7: Unused, MUST be zero when sent, MUST be ignored when received
type AttributeFlags uint8

// Attribute flag bits per RFC 4271 Section 4.3.
const (
	// FlagOptional: RFC 4271 Section 4.3 - "defines whether the attribute is
	// optional (if set to 1) or well-known (if set to 0).".
	FlagOptional AttributeFlags = 0x80

	// FlagTransitive: RFC 4271 Section 4.3 - "defines whether an optional
	// attribute is transitive (if set to 1) or non-transitive (if set to 0).
	// For well-known attributes, the Transitive bit MUST be set to 1.".
	FlagTransitive AttributeFlags = 0x40

	// FlagPartial: RFC 4271 Section 4.3 - "defines whether the information
	// contained in the optional transitive attribute is partial (if set to 1)
	// or complete (if set to 0). For well-known attributes and for optional
	// non-transitive attributes, the Partial bit MUST be set to 0.".
	FlagPartial AttributeFlags = 0x20

	// FlagExtLength: RFC 4271 Section 4.3 - "defines whether the Attribute
	// Length is one octet (if set to 0) or two octets (if set to 1).".
	FlagExtLength AttributeFlags = 0x10
)

// IsOptional returns true if the optional flag is set (RFC 4271 Section 4.3 bit 0).
func (f AttributeFlags) IsOptional() bool { return f&FlagOptional != 0 }

// IsTransitive returns true if the transitive flag is set (RFC 4271 Section 4.3 bit 1).
func (f AttributeFlags) IsTransitive() bool { return f&FlagTransitive != 0 }

// IsPartial returns true if the partial flag is set (RFC 4271 Section 4.3 bit 2).
func (f AttributeFlags) IsPartial() bool { return f&FlagPartial != 0 }

// IsExtLength returns true if extended length is used (RFC 4271 Section 4.3 bit 3).
func (f AttributeFlags) IsExtLength() bool { return f&FlagExtLength != 0 }

// ParseHeader parses an attribute header and returns flags, code, length, header length.
//
// RFC 4271 Section 4.3 defines the path attribute header format:
//
//	 0                   1
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Attr. Flags  |Attr. Type Code|
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// If Extended Length bit is 0: third octet contains 1-octet length (header=3).
// If Extended Length bit is 1: third/fourth octets contain 2-octet length (header=4).
func ParseHeader(data []byte) (flags AttributeFlags, code AttributeCode, length uint16, hdrLen int, err error) {
	if len(data) < 3 {
		return 0, 0, 0, 0, ErrShortData
	}

	flags = AttributeFlags(data[0])
	code = AttributeCode(data[1])

	// RFC 4271 Section 4.3: Extended Length bit determines length field size
	if flags.IsExtLength() {
		if len(data) < 4 {
			return 0, 0, 0, 0, ErrShortData
		}
		length = binary.BigEndian.Uint16(data[2:4])
		hdrLen = 4
	} else {
		length = uint16(data[2])
		hdrLen = 3
	}

	return flags, code, length, hdrLen, nil
}

// Attribute is the interface for all BGP path attributes.
//
// RFC 4271 Section 4.3 defines path attributes as triples:
// <attribute type, attribute length, attribute value>
//
// RFC 4271 Section 5 defines attribute categories and processing rules.
type Attribute interface {
	// Code returns the attribute type code (RFC 4271 Section 4.3).
	Code() AttributeCode

	// Flags returns the attribute flags (RFC 4271 Section 4.3).
	Flags() AttributeFlags

	// Len returns the attribute value length in octets (excluding header).
	Len() int

	// WriteTo writes the attribute value (excluding header) into buf at offset.
	// Returns number of bytes written.
	// Caller guarantees buf has sufficient capacity.
	WriteTo(buf []byte, off int) int

	// WriteToWithContext writes the attribute value with context-dependent encoding.
	// Returns number of bytes written.
	WriteToWithContext(buf []byte, off int, srcCtx, dstCtx *bgpctx.EncodingContext) int
}

// WriteHeaderTo writes an attribute header into buf at offset.
// Returns number of bytes written (3 or 4).
// Automatically sets FlagExtLength if length > 255.
func WriteHeaderTo(buf []byte, off int, flags AttributeFlags, code AttributeCode, length uint16) int {
	if length > 255 {
		flags |= FlagExtLength
	}

	if flags.IsExtLength() {
		buf[off] = byte(flags)
		buf[off+1] = byte(code)
		buf[off+2] = byte(length >> 8)
		buf[off+3] = byte(length)
		return 4
	}

	buf[off] = byte(flags)
	buf[off+1] = byte(code)
	buf[off+2] = byte(length)
	return 3
}

// WriteAttrTo writes a complete attribute (header + value) into buf at offset.
// Returns total bytes written.
func WriteAttrTo(attr Attribute, buf []byte, off int) int {
	valueLen := attr.Len()
	hdrLen := WriteHeaderTo(buf, off, attr.Flags(), attr.Code(), uint16(valueLen)) //nolint:gosec // G115: valueLen bounded by BGP attr max (65535)
	n := attr.WriteTo(buf, off+hdrLen)
	return hdrLen + n
}

// WriteAttrToWithContext writes a complete attribute with context-dependent encoding.
// Returns total bytes written.
// Zero-allocation: calculates length without packing.
func WriteAttrToWithContext(attr Attribute, buf []byte, off int, srcCtx, dstCtx *bgpctx.EncodingContext) int {
	// Get context-dependent length
	// For AS_PATH, length depends on ASN4 context; for others, Len() is sufficient
	valueLen := attrLenWithContext(attr, dstCtx)

	hdrLen := WriteHeaderTo(buf, off, attr.Flags(), attr.Code(), uint16(valueLen)) //nolint:gosec // G115: valueLen bounded by BGP attr max (65535)
	n := attr.WriteToWithContext(buf, off+hdrLen, srcCtx, dstCtx)
	return hdrLen + n
}

// attrLenWithContext returns the attribute value length accounting for context.
//
// Context-dependent attributes (RFC 6793):
//   - AS_PATH: 2-byte vs 4-byte ASN encoding
//   - AGGREGATOR: 6-byte vs 8-byte format
//
// For other attributes: returns Len() (context-independent).
func attrLenWithContext(attr Attribute, dstCtx *bgpctx.EncodingContext) int {
	switch a := attr.(type) {
	case *ASPath:
		return a.LenWithContext(nil, dstCtx)
	case *Aggregator:
		return a.LenWithContext(nil, dstCtx)
	}
	// All other attributes have context-independent length
	return attr.Len()
}
