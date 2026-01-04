// Package nlri implements BGP Network Layer Reachability Information encoding.
//
// RFC 4271 Section 4.3 - UPDATE Message Format:
// NLRI is encoded as one or more 2-tuples of the form <length, prefix>:
//
//	+---------------------------+
//	|   Length (1 octet)        |
//	+---------------------------+
//	|   Prefix (variable)       |
//	+---------------------------+
//
// RFC 4760 Section 5 - NLRI Encoding:
// Extends RFC 4271 NLRI encoding for multiprotocol (IPv6, VPN, etc.).
// Same <length, prefix> format applies to all address families.
//
// RFC 7911 Section 3 - Extended NLRI Encodings:
// ADD-PATH extends NLRI by prepending a 4-octet Path Identifier.
// Path ID handling is done by WriteNLRI(), not by the NLRI type itself.
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
// RFC 4271 Section 4.3 - Network Layer Reachability Information:
// Each prefix is encoded as <length, prefix> where length is in bits
// and prefix contains the minimum octets needed for the prefix.
//
// RFC 7911 Section 3 - Extended NLRI Encodings:
// Path ID is stored but NOT included in Len()/Bytes()/WriteTo().
// Use WriteNLRI() for ADD-PATH aware encoding.
type INET struct {
	family Family
	prefix netip.Prefix
	pathID uint32 // RFC 7911: 0 means no path ID
}

// NewINET creates a new INET NLRI.
// pathID=0 means no path identifier; pathID>0 stores the path ID.
// Use WriteNLRI() with PackContext.AddPath=true to encode with path ID.
func NewINET(family Family, prefix netip.Prefix, pathID uint32) *INET {
	return &INET{
		family: family,
		prefix: prefix,
		pathID: pathID,
	}
}

// ParseINET parses an INET NLRI from wire format.
//
// RFC 4271 Section 4.3 - UPDATE Message Format:
// Parses the <length, prefix> encoding where:
//   - Length (1 octet): prefix length in bits (0-32 for IPv4, 0-128 for IPv6)
//   - Prefix (variable): minimum octets to contain the prefix bits
//
// RFC 4760 Section 5 - NLRI Encoding:
// Same encoding applies for IPv6 (AFI=2) with 128-bit maximum prefix length.
//
// RFC 7911 Section 3 - Extended NLRI Encodings:
// When addpath=true, a 4-octet Path Identifier precedes the <length, prefix>.
//
// Returns the parsed NLRI, remaining bytes, and any error.
func ParseINET(afi AFI, safi SAFI, data []byte, addpath bool) (NLRI, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrShortRead
	}

	offset := 0
	var pathID uint32

	// RFC 7911 Section 3: Parse optional 4-octet Path Identifier (ADD-PATH)
	if addpath {
		if len(data) < 4 {
			return nil, nil, ErrShortRead
		}
		pathID = binary.BigEndian.Uint32(data[:4])
		offset = 4
	}

	// RFC 4271 Section 4.3: Parse prefix length (1 octet, value in bits)
	// "The Length field indicates the length in bits of the IP address prefix."
	if offset >= len(data) {
		return nil, nil, ErrShortRead
	}
	prefixLen := int(data[offset])
	offset++

	// RFC 4271 Section 4.3: IPv4 prefix length is 0-32 bits
	// RFC 4760 Section 5: IPv6 prefix length is 0-128 bits
	maxLen := 32
	if afi == AFIIPv6 {
		maxLen = 128
	}
	if prefixLen > maxLen {
		return nil, nil, ErrInvalidPrefix
	}

	// RFC 4271 Section 4.3: Calculate minimum octets for prefix
	// "The Prefix field contains an IP address prefix, followed by enough
	// trailing bits to make the end of the field fall on an octet boundary."
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
		family: Family{AFI: afi, SAFI: safi},
		prefix: prefix,
		pathID: pathID,
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

// PathID returns the ADD-PATH path identifier (0 if none).
func (i *INET) PathID() uint32 {
	return i.pathID
}

// Bytes returns the wire-format encoding (payload only, no path ID).
//
// RFC 4271 Section 4.3 - UPDATE Message Format:
// Encodes as <length, prefix> where length is prefix bits and prefix is
// the minimum octets needed (trailing bits are zero-padded to octet boundary).
//
// Note: Path ID is NOT included. Use WriteNLRI() for ADD-PATH encoding.
func (i *INET) Bytes() []byte {
	prefixLen := i.prefix.Bits()
	prefixBytes := (prefixLen + 7) / 8

	buf := make([]byte, 1+prefixBytes)
	buf[0] = byte(prefixLen)
	copy(buf[1:], i.prefix.Addr().AsSlice()[:prefixBytes])

	return buf
}

// Len returns the wire-format length in bytes (payload only, no path ID).
// Use LenWithContext() for ADD-PATH aware length calculation.
func (i *INET) Len() int {
	prefixLen := i.prefix.Bits()
	prefixBytes := (prefixLen + 7) / 8
	return 1 + prefixBytes
}

// BaseLen returns the payload length WITHOUT path ID.
// After Phase 3, this is identical to Len(). Kept for Phase 4 cleanup.
func (i *INET) BaseLen() int {
	return i.Len()
}

// WritePayloadTo writes the NLRI payload (without path ID) into buf at offset.
// Returns number of bytes written.
// After Phase 3, this is identical to WriteTo(buf, off, nil). Kept for Phase 4 cleanup.
func (i *INET) WritePayloadTo(buf []byte, off int) int {
	prefixLen := i.prefix.Bits()
	prefixBytes := (prefixLen + 7) / 8

	pos := off

	// Write prefix length
	buf[pos] = byte(prefixLen)
	pos++

	// Write prefix bytes
	copy(buf[pos:], i.prefix.Addr().AsSlice()[:prefixBytes])
	pos += prefixBytes

	return pos - off
}

// String returns a human-readable representation.
func (i *INET) String() string {
	if i.pathID != 0 {
		return fmt.Sprintf("%s path-id=%d", i.prefix, i.pathID)
	}
	return i.prefix.String()
}

// WriteTo writes the NLRI payload (without path ID) into buf at offset.
// Returns number of bytes written.
//
// Note: Path ID is NOT written. Use WriteNLRI() for ADD-PATH encoding.
// The ctx parameter is ignored (kept for interface compatibility).
func (i *INET) WriteTo(buf []byte, off int, _ *PackContext) int {
	return i.WritePayloadTo(buf, off)
}

// Pack returns wire-format bytes adapted for negotiated capabilities.
//
// Deprecated: Use WriteNLRI() for zero-allocation encoding.
// This method allocates a new slice; prefer WriteNLRI() with pre-allocated buffer.
func (i *INET) Pack(ctx *PackContext) []byte {
	size := LenWithContext(i, ctx)
	buf := make([]byte, size)
	WriteNLRI(i, buf, 0, ctx)
	return buf
}
