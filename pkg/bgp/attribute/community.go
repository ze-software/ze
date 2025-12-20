package attribute

import (
	"encoding/binary"
	"fmt"
)

// Community represents a standard BGP community.
//
// RFC 1997 - BGP Communities Attribute:
// Communities are treated as 32 bit values. The community attribute values
// ranging from 0x00000000 through 0x0000FFFF and 0xFFFF0000 through 0xFFFFFFFF
// are reserved. Other values encode an AS number in the first two octets.
//
// RFC 1997 Section "COMMUNITIES attribute":
// The COMMUNITIES path attribute is an optional transitive attribute of
// variable length. The attribute consists of a set of four octet values,
// each of which specify a community.
type Community uint32

// Well-known communities.
//
// RFC 1997 Section "Well-known Communities":
// The following communities have global significance and their operations
// shall be implemented in any community-attribute-aware BGP speaker.
const (
	// CommunityNoExport - RFC 1997: All routes received carrying a communities
	// attribute containing this value MUST NOT be advertised outside a BGP
	// confederation boundary.
	CommunityNoExport Community = 0xFFFFFF01 // NO_EXPORT

	// CommunityNoAdvertise - RFC 1997: All routes received carrying a communities
	// attribute containing this value MUST NOT be advertised to other BGP peers.
	CommunityNoAdvertise Community = 0xFFFFFF02 // NO_ADVERTISE

	// CommunityNoExportSubconfed - RFC 1997: All routes received carrying a
	// communities attribute containing this value MUST NOT be advertised to
	// external BGP peers (this includes peers in other member autonomous systems
	// inside a BGP confederation).
	CommunityNoExportSubconfed Community = 0xFFFFFF03 // NO_EXPORT_SUBCONFED

	// CommunityNoPeer - RFC 3765: Routes with this community should not be
	// advertised to peers.
	CommunityNoPeer Community = 0xFFFFFF04 // NOPEER (RFC 3765)
)

// String returns the community in ASN:value format.
//
// RFC 1997: Community values encode an AS number in the first two octets
// and a locally defined value in the last two octets, displayed as ASN:value.
func (c Community) String() string {
	switch c {
	case CommunityNoExport:
		return "NO_EXPORT"
	case CommunityNoAdvertise:
		return "NO_ADVERTISE"
	case CommunityNoExportSubconfed:
		return "NO_EXPORT_SUBCONFED"
	case CommunityNoPeer:
		return "NOPEER"
	}
	return fmt.Sprintf("%d:%d", c>>16, c&0xFFFF)
}

// Communities represents the COMMUNITIES attribute.
//
// RFC 1997 Section "COMMUNITIES attribute":
// The COMMUNITIES path attribute has Type Code 8. It is an optional
// transitive attribute of variable length consisting of a set of
// four octet values.
type Communities []Community

// Code returns the attribute type code (8).
// RFC 1997: "The COMMUNITIES attribute has Type Code 8."
func (c Communities) Code() AttributeCode { return AttrCommunity }

// Flags returns the attribute flags.
// RFC 1997: "The COMMUNITIES path attribute is an optional transitive attribute."
func (c Communities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the length in bytes (4 bytes per community).
func (c Communities) Len() int { return len(c) * 4 }

// Pack encodes the communities attribute.
// RFC 1997: Each community is encoded as a 4-octet value.
func (c Communities) Pack() []byte {
	buf := make([]byte, len(c)*4)
	for i, comm := range c {
		binary.BigEndian.PutUint32(buf[i*4:], uint32(comm))
	}
	return buf
}

// ParseCommunities parses a COMMUNITIES attribute.
//
// RFC 1997: The attribute consists of a set of four octet values.
// Length must be a multiple of 4 bytes.
func ParseCommunities(data []byte) (Communities, error) {
	if len(data)%4 != 0 {
		return nil, ErrInvalidLength
	}
	comms := make(Communities, len(data)/4)
	for i := range comms {
		comms[i] = Community(binary.BigEndian.Uint32(data[i*4:]))
	}
	return comms, nil
}

// Contains returns true if the community list contains the given community.
func (c Communities) Contains(comm Community) bool {
	for _, v := range c {
		if v == comm {
			return true
		}
	}
	return false
}

// ExtendedCommunity represents an extended community.
//
// RFC 4360 Section 2 - BGP Extended Communities Attribute:
// Each Extended Community is encoded as an 8-octet quantity:
//   - Type Field:  1 or 2 octets
//   - Value Field: Remaining octets
//
// RFC 4360 Section 2:
// The high-order octet of the Type Field contains:
//   - Bit 0 (I): IANA authority bit
//   - Bit 1 (T): Transitive bit (0=transitive, 1=non-transitive across AS)
//   - Bits 2-7: Structure of the community
//
// Two extended communities are declared equal only when all 8 octets are equal.
type ExtendedCommunity [8]byte

// ExtendedCommunities represents the EXTENDED_COMMUNITIES attribute.
//
// RFC 4360 Section 2:
// The Extended Communities Attribute is a transitive optional BGP attribute,
// with the Type Code 16. The attribute consists of a set of "extended communities".
type ExtendedCommunities []ExtendedCommunity

// Code returns the attribute type code (16).
// RFC 4360 Section 2: "The Extended Communities Attribute... with the Type Code 16."
func (e ExtendedCommunities) Code() AttributeCode { return AttrExtCommunity }

// Flags returns the attribute flags.
// RFC 4360 Section 2: "The Extended Communities Attribute is a transitive optional BGP attribute."
func (e ExtendedCommunities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the length in bytes (8 bytes per extended community).
func (e ExtendedCommunities) Len() int { return len(e) * 8 }

// Pack encodes the extended communities attribute.
// RFC 4360 Section 2: Each Extended Community is encoded as an 8-octet quantity.
func (e ExtendedCommunities) Pack() []byte {
	buf := make([]byte, len(e)*8)
	for i, ec := range e {
		copy(buf[i*8:], ec[:])
	}
	return buf
}

// ParseExtendedCommunities parses an EXTENDED_COMMUNITIES attribute.
//
// RFC 4360 Section 2: Each Extended Community is encoded as an 8-octet quantity.
// Length must be a multiple of 8 bytes.
func ParseExtendedCommunities(data []byte) (ExtendedCommunities, error) {
	if len(data)%8 != 0 {
		return nil, ErrInvalidLength
	}
	comms := make(ExtendedCommunities, len(data)/8)
	for i := range comms {
		copy(comms[i][:], data[i*8:])
	}
	return comms, nil
}

// LargeCommunity represents a large community.
//
// RFC 8092 Section 3 - BGP Large Communities Attribute:
// Each BGP Large Community value is encoded as a 12-octet quantity:
//
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                      Global Administrator                     |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                       Local Data Part 1                       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                       Local Data Part 2                       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// RFC 8092 Section 3:
// Global Administrator: A four-octet namespace identifier (SHOULD be an ASN).
// Local Data Part 1:    A four-octet operator-defined value.
// Local Data Part 2:    A four-octet operator-defined value.
type LargeCommunity struct {
	GlobalAdmin uint32 // RFC 8092: Global Administrator (typically ASN)
	LocalData1  uint32 // RFC 8092: Local Data Part 1
	LocalData2  uint32 // RFC 8092: Local Data Part 2
}

// String returns the large community in canonical representation.
//
// RFC 8092 Section 5 - Canonical Representation:
// The canonical representation is three separate unsigned integers in decimal
// notation: Global Administrator, Local Data 1, Local Data 2. Each number is
// separated from the next by a single colon. Numbers MUST NOT contain leading
// zeros; a zero value MUST be represented with a single zero.
func (l LargeCommunity) String() string {
	return fmt.Sprintf("%d:%d:%d", l.GlobalAdmin, l.LocalData1, l.LocalData2)
}

// LargeCommunities represents the LARGE_COMMUNITIES attribute.
//
// RFC 8092 Section 3:
// The BGP Large Communities attribute is an optional transitive path attribute
// of variable length. All routes with this attribute belong to the communities
// specified in the attribute.
//
// RFC 8092 Section 8 - IANA Considerations:
// IANA has assigned the value 32 (LARGE_COMMUNITY) in the "BGP Path Attributes"
// subregistry.
type LargeCommunities []LargeCommunity

// Code returns the attribute type code (32).
// RFC 8092 Section 8: "IANA has assigned the value 32 (LARGE_COMMUNITY)."
func (l LargeCommunities) Code() AttributeCode { return AttrLargeCommunity }

// Flags returns the attribute flags.
// RFC 8092 Section 3: "optional transitive path attribute"
func (l LargeCommunities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the length in bytes (12 bytes per large community).
func (l LargeCommunities) Len() int { return len(l) * 12 }

// Pack encodes the large communities attribute.
// RFC 8092 Section 3: Each value is encoded as a 12-octet quantity.
func (l LargeCommunities) Pack() []byte {
	buf := make([]byte, len(l)*12)
	for i, lc := range l {
		offset := i * 12
		binary.BigEndian.PutUint32(buf[offset:], lc.GlobalAdmin)
		binary.BigEndian.PutUint32(buf[offset+4:], lc.LocalData1)
		binary.BigEndian.PutUint32(buf[offset+8:], lc.LocalData2)
	}
	return buf
}

// ParseLargeCommunities parses a LARGE_COMMUNITIES attribute.
//
// RFC 8092 Section 6 - Error Handling:
// A BGP Large Communities attribute SHALL be considered malformed if the
// length, expressed in octets, is not a non-zero multiple of 12.
//
// A BGP UPDATE message with a malformed BGP Large Communities attribute
// SHALL be handled using the approach of "treat-as-withdraw" (RFC 7606).
func ParseLargeCommunities(data []byte) (LargeCommunities, error) {
	if len(data)%12 != 0 {
		return nil, ErrInvalidLength
	}
	comms := make(LargeCommunities, len(data)/12)
	for i := range comms {
		offset := i * 12
		comms[i] = LargeCommunity{
			GlobalAdmin: binary.BigEndian.Uint32(data[offset:]),
			LocalData1:  binary.BigEndian.Uint32(data[offset+4:]),
			LocalData2:  binary.BigEndian.Uint32(data[offset+8:]),
		}
	}
	return comms, nil
}
