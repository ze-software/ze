// Package attribute implements BGP path attributes (RFC 4271, RFC 4760).
package attribute

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Errors.
var (
	ErrShortData      = errors.New("attribute: short data")
	ErrInvalidLength  = errors.New("attribute: invalid length")
	ErrUnknownCode    = errors.New("attribute: unknown code")
	ErrMalformedValue = errors.New("attribute: malformed value")
)

// AttributeCode identifies a path attribute type (RFC 4271 Section 5).
type AttributeCode uint8

// Path attribute type codes.
const (
	AttrOrigin          AttributeCode = 1  // RFC 4271
	AttrASPath          AttributeCode = 2  // RFC 4271
	AttrNextHop         AttributeCode = 3  // RFC 4271
	AttrMED             AttributeCode = 4  // RFC 4271 (MULTI_EXIT_DISC)
	AttrLocalPref       AttributeCode = 5  // RFC 4271
	AttrAtomicAggregate AttributeCode = 6  // RFC 4271
	AttrAggregator      AttributeCode = 7  // RFC 4271
	AttrCommunity       AttributeCode = 8  // RFC 1997
	AttrOriginatorID    AttributeCode = 9  // RFC 4456
	AttrClusterList     AttributeCode = 10 // RFC 4456
	AttrMPReachNLRI     AttributeCode = 14 // RFC 4760
	AttrMPUnreachNLRI   AttributeCode = 15 // RFC 4760
	AttrExtCommunity    AttributeCode = 16 // RFC 4360
	AttrAS4Path         AttributeCode = 17 // RFC 6793
	AttrAS4Aggregator   AttributeCode = 18 // RFC 6793
	AttrPMSI            AttributeCode = 22 // RFC 6514
	AttrTunnelEncap     AttributeCode = 23 // RFC 5512
	AttrAIGP            AttributeCode = 26 // RFC 7311
	AttrBGPLS           AttributeCode = 29 // RFC 7752
	AttrLargeCommunity  AttributeCode = 32 // RFC 8092
	AttrPrefixSID       AttributeCode = 40 // RFC 8669
)

var attrCodeNames = map[AttributeCode]string{
	AttrOrigin:          "ORIGIN",
	AttrASPath:          "AS_PATH",
	AttrNextHop:         "NEXT_HOP",
	AttrMED:             "MULTI_EXIT_DISC",
	AttrLocalPref:       "LOCAL_PREF",
	AttrAtomicAggregate: "ATOMIC_AGGREGATE",
	AttrAggregator:      "AGGREGATOR",
	AttrCommunity:       "COMMUNITIES",
	AttrOriginatorID:    "ORIGINATOR_ID",
	AttrClusterList:     "CLUSTER_LIST",
	AttrMPReachNLRI:     "MP_REACH_NLRI",
	AttrMPUnreachNLRI:   "MP_UNREACH_NLRI",
	AttrExtCommunity:    "EXTENDED_COMMUNITIES",
	AttrAS4Path:         "AS4_PATH",
	AttrAS4Aggregator:   "AS4_AGGREGATOR",
	AttrPMSI:            "PMSI_TUNNEL",
	AttrTunnelEncap:     "TUNNEL_ENCAPSULATION",
	AttrAIGP:            "AIGP",
	AttrBGPLS:           "BGP_LS",
	AttrLargeCommunity:  "LARGE_COMMUNITIES",
	AttrPrefixSID:       "PREFIX_SID",
}

// String returns the attribute name.
func (c AttributeCode) String() string {
	if name, ok := attrCodeNames[c]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", c)
}

// AttributeFlags are the attribute flags (RFC 4271 Section 4.3).
type AttributeFlags uint8

// Attribute flag bits.
const (
	FlagOptional   AttributeFlags = 0x80 // Bit 0: Optional (1) vs Well-known (0)
	FlagTransitive AttributeFlags = 0x40 // Bit 1: Transitive (1) vs Non-transitive (0)
	FlagPartial    AttributeFlags = 0x20 // Bit 2: Partial (1) vs Complete (0)
	FlagExtLength  AttributeFlags = 0x10 // Bit 3: Extended Length (1) = 2 bytes
)

// IsOptional returns true if the optional flag is set.
func (f AttributeFlags) IsOptional() bool { return f&FlagOptional != 0 }

// IsTransitive returns true if the transitive flag is set.
func (f AttributeFlags) IsTransitive() bool { return f&FlagTransitive != 0 }

// IsPartial returns true if the partial flag is set.
func (f AttributeFlags) IsPartial() bool { return f&FlagPartial != 0 }

// IsExtLength returns true if extended length is used.
func (f AttributeFlags) IsExtLength() bool { return f&FlagExtLength != 0 }

// ParseHeader parses an attribute header and returns flags, code, length, header length.
// The header length is 3 for standard length, 4 for extended length.
func ParseHeader(data []byte) (flags AttributeFlags, code AttributeCode, length uint16, hdrLen int, err error) {
	if len(data) < 3 {
		return 0, 0, 0, 0, ErrShortData
	}

	flags = AttributeFlags(data[0])
	code = AttributeCode(data[1])

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

// PackHeader packs an attribute header.
// Automatically sets FlagExtLength if length > 255.
func PackHeader(flags AttributeFlags, code AttributeCode, length uint16) []byte {
	if length > 255 {
		flags |= FlagExtLength
	}

	if flags.IsExtLength() {
		return []byte{
			byte(flags),
			byte(code),
			byte(length >> 8),
			byte(length),
		}
	}

	return []byte{
		byte(flags),
		byte(code),
		byte(length),
	}
}

// Attribute is the interface for all BGP path attributes.
type Attribute interface {
	// Code returns the attribute type code.
	Code() AttributeCode

	// Flags returns the attribute flags.
	Flags() AttributeFlags

	// Len returns the attribute value length (excluding header).
	Len() int

	// Pack serializes the attribute value (excluding header).
	Pack() []byte
}
