package attribute

import (
	"encoding/binary"
	"fmt"
)

// Community represents a standard BGP community (RFC 1997).
type Community uint32

// Well-known communities.
const (
	CommunityNoExport          Community = 0xFFFFFF01 // NO_EXPORT
	CommunityNoAdvertise       Community = 0xFFFFFF02 // NO_ADVERTISE
	CommunityNoExportSubconfed Community = 0xFFFFFF03 // NO_EXPORT_SUBCONFED
	CommunityNoPeer            Community = 0xFFFFFF04 // NOPEER (RFC 3765)
)

// String returns the community in ASN:value format.
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

// Communities represents the COMMUNITIES attribute (RFC 1997).
type Communities []Community

func (c Communities) Code() AttributeCode   { return AttrCommunity }
func (c Communities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }
func (c Communities) Len() int              { return len(c) * 4 }
func (c Communities) Pack() []byte {
	buf := make([]byte, len(c)*4)
	for i, comm := range c {
		binary.BigEndian.PutUint32(buf[i*4:], uint32(comm))
	}
	return buf
}

// ParseCommunities parses a COMMUNITIES attribute.
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

// ExtendedCommunity represents an extended community (RFC 4360).
// 8 bytes: type(2) + value(6)
type ExtendedCommunity [8]byte

// ExtendedCommunities represents the EXTENDED_COMMUNITIES attribute.
type ExtendedCommunities []ExtendedCommunity

func (e ExtendedCommunities) Code() AttributeCode   { return AttrExtCommunity }
func (e ExtendedCommunities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }
func (e ExtendedCommunities) Len() int              { return len(e) * 8 }
func (e ExtendedCommunities) Pack() []byte {
	buf := make([]byte, len(e)*8)
	for i, ec := range e {
		copy(buf[i*8:], ec[:])
	}
	return buf
}

// ParseExtendedCommunities parses an EXTENDED_COMMUNITIES attribute.
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

// LargeCommunity represents a large community (RFC 8092).
// 12 bytes: global(4) + local1(4) + local2(4)
type LargeCommunity struct {
	GlobalAdmin uint32
	LocalData1  uint32
	LocalData2  uint32
}

// String returns the large community in GA:LD1:LD2 format.
func (l LargeCommunity) String() string {
	return fmt.Sprintf("%d:%d:%d", l.GlobalAdmin, l.LocalData1, l.LocalData2)
}

// LargeCommunities represents the LARGE_COMMUNITIES attribute (RFC 8092).
type LargeCommunities []LargeCommunity

func (l LargeCommunities) Code() AttributeCode   { return AttrLargeCommunity }
func (l LargeCommunities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }
func (l LargeCommunities) Len() int              { return len(l) * 12 }
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
