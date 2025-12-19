package nlri

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
)

// Errors for INET parsing.
var (
	ErrShortRead      = errors.New("nlri: short read")
	ErrInvalidPrefix  = errors.New("nlri: invalid prefix length")
	ErrInvalidAddress = errors.New("nlri: invalid address")
)

// INET represents an IPv4 or IPv6 unicast/multicast NLRI.
//
// This is the most common NLRI type, representing a simple IP prefix.
// Supports ADD-PATH (RFC 7911) with optional path ID.
type INET struct {
	family  Family
	prefix  netip.Prefix
	pathID  uint32
	hasPath bool
}

// NewINET creates a new INET NLRI.
// Use pathID=0 and the result will have HasPathID()=false.
// Use pathID>0 and the result will have HasPathID()=true.
func NewINET(family Family, prefix netip.Prefix, pathID uint32) *INET {
	return &INET{
		family:  family,
		prefix:  prefix,
		pathID:  pathID,
		hasPath: pathID != 0,
	}
}

// ParseINET parses an INET NLRI from wire format.
// Returns the parsed NLRI, remaining bytes, and any error.
func ParseINET(afi AFI, safi SAFI, data []byte, addpath bool) (NLRI, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrShortRead
	}

	offset := 0
	var pathID uint32

	// Parse optional path ID (ADD-PATH)
	if addpath {
		if len(data) < 4 {
			return nil, nil, ErrShortRead
		}
		pathID = binary.BigEndian.Uint32(data[:4])
		offset = 4
	}

	// Parse prefix length (in bits)
	if offset >= len(data) {
		return nil, nil, ErrShortRead
	}
	prefixLen := int(data[offset])
	offset++

	// Validate prefix length
	maxLen := 32
	if afi == AFIIPv6 {
		maxLen = 128
	}
	if prefixLen > maxLen {
		return nil, nil, ErrInvalidPrefix
	}

	// Calculate prefix bytes
	prefixBytes := (prefixLen + 7) / 8

	if offset+prefixBytes > len(data) {
		return nil, nil, ErrShortRead
	}

	// Build address from prefix bytes
	var addr netip.Addr
	if afi == AFIIPv4 {
		var ip [4]byte
		copy(ip[:], data[offset:offset+prefixBytes])
		addr = netip.AddrFrom4(ip)
	} else {
		var ip [16]byte
		copy(ip[:], data[offset:offset+prefixBytes])
		addr = netip.AddrFrom16(ip)
	}

	prefix, err := addr.Prefix(prefixLen)
	if err != nil {
		return nil, nil, ErrInvalidAddress
	}

	inet := &INET{
		family:  Family{AFI: afi, SAFI: safi},
		prefix:  prefix,
		pathID:  pathID,
		hasPath: addpath,
	}

	return inet, data[offset+prefixBytes:], nil
}

// Family returns the AFI/SAFI for this NLRI.
func (i *INET) Family() Family {
	return i.family
}

// Prefix returns the IP prefix.
func (i *INET) Prefix() netip.Prefix {
	return i.prefix
}

// PathID returns the ADD-PATH path identifier.
func (i *INET) PathID() uint32 {
	return i.pathID
}

// HasPathID returns true if this NLRI has an ADD-PATH path ID.
func (i *INET) HasPathID() bool {
	return i.hasPath
}

// Bytes returns the wire-format encoding.
func (i *INET) Bytes() []byte {
	prefixLen := i.prefix.Bits()
	prefixBytes := (prefixLen + 7) / 8

	var buf []byte
	if i.hasPath {
		buf = make([]byte, 4+1+prefixBytes)
		binary.BigEndian.PutUint32(buf[:4], i.pathID)
		buf[4] = byte(prefixLen)
		copy(buf[5:], i.prefix.Addr().AsSlice()[:prefixBytes])
	} else {
		buf = make([]byte, 1+prefixBytes)
		buf[0] = byte(prefixLen)
		copy(buf[1:], i.prefix.Addr().AsSlice()[:prefixBytes])
	}

	return buf
}

// Len returns the wire-format length in bytes.
func (i *INET) Len() int {
	prefixLen := i.prefix.Bits()
	prefixBytes := (prefixLen + 7) / 8
	if i.hasPath {
		return 4 + 1 + prefixBytes
	}
	return 1 + prefixBytes
}

// String returns a human-readable representation.
func (i *INET) String() string {
	if i.hasPath {
		return fmt.Sprintf("%s path-id=%d", i.prefix, i.pathID)
	}
	return i.prefix.String()
}
