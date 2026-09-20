// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding
// RFC: rfc/short/rfc4364.md — route distinguisher types
//
// Package nlri implements BGP Network Layer Reachability Information types.
package nlri

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// RDType represents Route Distinguisher type.
//
// RFC 4364 Section 4.2 defines the Route Distinguisher encoding:
//   - Type Field: 2 bytes
//   - Value Field: 6 bytes
//
// The interpretation of the Value field depends on the type field value.
type RDType uint16

// Route Distinguisher types per RFC 4364 Section 4.2.
//
// RFC 4364 Section 4.2 specifies three RD type values:
//   - Type 0: Administrator=2-byte ASN, Assigned Number=4 bytes
//   - Type 1: Administrator=4-byte IP, Assigned Number=2 bytes
//   - Type 2: Administrator=4-byte ASN, Assigned Number=2 bytes
const (
	RDType0 RDType = 0 // RFC 4364 Section 4.2: 2-byte ASN : 4-byte assigned number
	RDType1 RDType = 1 // RFC 4364 Section 4.2: 4-byte IP address : 2-byte assigned number
	RDType2 RDType = 2 // RFC 4364 Section 4.2: 4-byte ASN : 2-byte assigned number
)

// RouteDistinguisher uniquely identifies a VPN route.
//
// RFC 4364 Section 4.1 defines VPN-IPv4 addresses as 12-byte quantities:
//   - 8-byte Route Distinguisher (RD)
//   - 4-byte IPv4 address
//
// RFC 4659 Section 2 extends this for VPN-IPv6 as 24-byte quantities:
//   - 8-byte Route Distinguisher (RD)
//   - 16-byte IPv6 address
//
// The RD itself is 8 bytes: 2-byte type field + 6-byte value field.
// Per RFC 4364 Section 4.1, the RD's purpose is solely to allow creation
// of distinct routes to a common IP address prefix across different VPNs.
type RouteDistinguisher struct {
	Type  RDType  // RFC 4364 Section 4.2: Type field (2 bytes)
	Value [6]byte // RFC 4364 Section 4.2: Value field (6 bytes)
}

// ParseRouteDistinguisher parses an RD from 8 bytes.
//
// RFC 4364 Section 4.2 encoding:
//
//	Bytes 0-1: Type field (big-endian)
//	Bytes 2-7: Value field (interpretation depends on Type)
func ParseRouteDistinguisher(data []byte) (RouteDistinguisher, error) {
	if len(data) < 8 {
		return RouteDistinguisher{}, ErrShortRead
	}

	rd := RouteDistinguisher{
		Type: RDType(binary.BigEndian.Uint16(data[:2])),
	}
	copy(rd.Value[:], data[2:8])
	return rd, nil
}

// Bytes returns the wire format per RFC 4364 Section 4.2.
// Hot-path callers use WriteTo(buf, off) into a pool buffer; this method
// is for JSON/test/format callers that need a standalone 8-byte slice.
func (rd RouteDistinguisher) Bytes() []byte {
	buf := make([]byte, 8) // pool-fallback: result owned by caller
	binary.BigEndian.PutUint16(buf[:2], uint16(rd.Type))
	copy(buf[2:], rd.Value[:])
	return buf
}

// Len returns the RD length in bytes (always 8).
// RFC 4364 Section 4.2: RD is 8 octets (2-byte type + 6-byte value).
func (rd RouteDistinguisher) Len() int { return 8 }

// WriteTo writes the RD directly to buf at offset (zero-alloc).
// Returns bytes written (always 8).
func (rd RouteDistinguisher) WriteTo(buf []byte, off int) int {
	binary.BigEndian.PutUint16(buf[off:], uint16(rd.Type))
	copy(buf[off+2:], rd.Value[:])
	return 8
}

// CheckedWriteTo validates capacity before writing.
func (rd RouteDistinguisher) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := rd.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return rd.WriteTo(buf, off), nil
}

// String returns a human-readable representation with type prefix.
//
// RFC 4364 Section 4.2 defines three RD types. The format includes the type
// prefix to disambiguate between Type 0 and Type 2 (both use ASN:assigned):
//   - Type 0: "0:ASN:assigned" (e.g., "0:65000:100") - 2-byte ASN
//   - Type 1: "1:IP:assigned" (e.g., "1:192.0.2.1:100") - 4-byte IP
//   - Type 2: "2:ASN:assigned" (e.g., "2:65536:100") - 4-byte ASN
//
// The type prefix is required for unambiguous parsing since Type 0 and Type 2
// would otherwise be indistinguishable for ASNs <= 65535.
func (rd RouteDistinguisher) String() string {
	b := textbuf.Get()
	defer b.Release()
	switch rd.Type {
	case RDType0:
		administrator := binary.BigEndian.Uint16(rd.Value[:2])
		assigned := binary.BigEndian.Uint32(rd.Value[2:6])
		return b.Str("0:").Uint16(administrator).Byte(':').Uint32(assigned).String()
	case RDType1:
		ip := netip.AddrFrom4([4]byte(rd.Value[:4]))
		assigned := binary.BigEndian.Uint16(rd.Value[4:6])
		return b.Str("1:").Addr(ip).Byte(':').Uint16(assigned).String()
	case RDType2:
		administrator := binary.BigEndian.Uint32(rd.Value[:4])
		assigned := binary.BigEndian.Uint16(rd.Value[4:6])
		return b.Str("2:").Uint32(administrator).Byte(':').Uint16(assigned).String()
	default:
		return b.Str("rd-type").Uint16(uint16(rd.Type)).Byte(':').Hex(rd.Value[:]).String()
	}
}

// ParseRDString parses a Route Distinguisher from string format.
//
// RFC 4364 Section 4.2 defines RD types:
//   - Type 0: "ASN:value" (2-byte ASN, 4-byte value) e.g., "65000:100"
//   - Type 1: "IP:value" (4-byte IP, 2-byte value) e.g., "192.0.2.1:100"
//   - Type 2: "ASN:value" (4-byte ASN, 2-byte value) e.g., "4200000001:100"
//
// The administrator decides the type, and netip decides the administrator:
//   - netip reads it as an IPv4 address -> Type 1.
//   - the AS number needs more than two octets -> Type 2.
//   - otherwise -> Type 0.
//
// A DECLARED type wins over that last rule. `String()` writes the type prefix
// because Type 0 and Type 2 are indistinguishable for an AS number of 65535 or
// less. So `2:65000:100` MUST come back as Type 2, or the round trip through
// String() loses what the prefix carries. A declared type the administrator
// cannot hold is refused rather than corrected.
//
// The AS number is read in every RFC 5396 spelling. A dot does not say the
// field is an address, because Section 2 of that RFC writes AS 65546 as `1.10`.
// The probe above is netip for that reason, rather than a search for a dot.
func ParseRDString(s string) (RouteDistinguisher, error) {
	var rd RouteDistinguisher
	declaredType := false
	parts := strings.Split(s, ":")

	// Handle typed format: type:ASN:value or type:IP:value (3 parts)
	// This format is produced by RouteDistinguisher.String()
	// Type 0: "0:ASN:assigned" (e.g., "0:65000:100")
	// Type 1: "1:IP:assigned" (e.g., "1:1.2.3.4:100")
	// Type 2: "2:ASN:assigned" (e.g., "2:65000:100")
	if len(parts) == 3 {
		rdType, err := strconv.ParseUint(parts[0], 10, 8)
		if err != nil || rdType > 2 {
			return rd, fmt.Errorf("invalid RD type: %s", parts[0])
		}
		// The declared type and the administrator must be the same KIND.
		//
		// The question asked is "is this an address", answered by netip. It is
		// not "does it contain a dot". A dot is not the tell, because RFC 5396
		// Section 2 writes AS 65546 as `1.10`. So `2:1.10:5` declares a
		// four-byte AS number and MUST be accepted. `0:1.2.3.4:5` declares an
		// AS number, carries an address, and MUST NOT be.
		_, addrErr := netip.ParseAddr(parts[1])
		if rdType == 1 && addrErr != nil {
			return rd, fmt.Errorf("invalid RD format: type 1 requires IP address, got %s", parts[1])
		}
		if rdType != 1 && addrErr == nil {
			return rd, fmt.Errorf("invalid RD format: type %d requires ASN, got IP %s", rdType, parts[1])
		}
		// Reconstruct as 2-part format for parsing below
		rd.Type = RDType(rdType)
		declaredType = true
		parts = parts[1:] // Now parts is [ASN/IP, value]
	} else if len(parts) != 2 {
		return rd, fmt.Errorf("invalid RD format: %s (expected ASN:value or IP:value)", s)
	}

	// Check if first part is an IP address (Type 1).
	//
	// The probe is netip rather than a search for a dot. A dot is not the
	// tell. RFC 5396 Section 2 writes AS 65546 as `1.10`, so a dotted
	// administrator is an AS number as often as it is an address. netip
	// refuses `1.10`, which leaves it to the AS branch below.
	if ip, err := netip.ParseAddr(parts[0]); err == nil && ip.Is4() {
		val, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return rd, fmt.Errorf("invalid RD value (must be 0-65535): %s", parts[1])
		}
		rd.Type = RDType1
		ip4 := ip.As4()
		copy(rd.Value[:4], ip4[:])
		rd.Value[4] = byte(val >> 8)
		rd.Value[5] = byte(val)
		return rd, nil
	}

	// Parse ASN to determine Type 0 vs Type 2. asn.Parse reads all three RFC
	// 5396 spellings, so `rd 1.10:5` names the route distinguisher `65546:5`
	// names, on the command path as well as in the configuration.
	number, err := asn.Parse(parts[0])
	if err != nil {
		return rd, fmt.Errorf("invalid ASN in RD: %s", parts[0])
	}
	// The magnitude picks the type, and a type the caller DECLARED overrides
	// it: see the note on the round trip above.
	fourByte := number > 65535
	if declaredType {
		if rd.Type == RDType0 && fourByte {
			return RouteDistinguisher{}, fmt.Errorf(
				"invalid RD %q: type 0 holds a 2-byte AS number, and %s needs four", s, parts[0])
		}
		fourByte = rd.Type == RDType2
	}

	if fourByte {
		// Type 2: 4-byte ASN : 2-byte value
		val, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return rd, fmt.Errorf("invalid RD value (must be 0-65535 for 4-byte ASN): %s", parts[1])
		}
		rd.Type = RDType2
		rd.Value[0] = byte(number >> 24)
		rd.Value[1] = byte(number >> 16)
		rd.Value[2] = byte(number >> 8)
		rd.Value[3] = byte(number)
		rd.Value[4] = byte(val >> 8)
		rd.Value[5] = byte(val)
		return rd, nil
	}

	// Type 0: 2-byte ASN : 4-byte value
	val, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return rd, fmt.Errorf("invalid RD value: %s", parts[1])
	}
	rd.Type = RDType0
	rd.Value[0] = byte(number >> 8)
	rd.Value[1] = byte(number)
	rd.Value[2] = byte(val >> 24)
	rd.Value[3] = byte(val >> 16)
	rd.Value[4] = byte(val >> 8)
	rd.Value[5] = byte(val)
	return rd, nil
}

// ParseLabelStack parses an MPLS label stack from wire format and returns the
// ENTRIES, the remaining bytes, and any error.
//
// RFC 8277 Section 2.1 carries "one or more 3-octet entries in the form
// described in RFC 3032 Section 2.1", and each entry is three facts, not one:
//
//	Byte 0: label[19:12]
//	Byte 1: label[11:4]
//	Byte 2: label[3:0], TC[2:0], S (bottom of stack)
//
// This returns each entry whole, so the traffic class and the bottom-of-stack
// bit survive. Returning the 20-bit label alone discarded both, and a decoder
// that re-encoded a route then published a traffic class of zero for one the
// peer had set. LabelValue answers the label a caller wants to display or
// match on.
//
// Parsing stops after the entry whose S bit is set, which is what terminates
// the stack.
func ParseLabelStack(data []byte) ([]uint32, []byte, error) {
	var entries []uint32

	for {
		if len(data) < 3 {
			return nil, nil, ErrShortRead
		}

		entry := uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
		entries = append(entries, entry)
		data = data[3:]

		if entry&labelBottomOfStack != 0 {
			break
		}
	}

	return entries, data, nil
}

// labelBottomOfStack is the S bit of an RFC 3032 Section 2.1 label stack entry:
// set on the last entry of a stack and clear on every other.
const labelBottomOfStack = 0x000001

// LabelValue answers the 20-bit label of one stack entry, which is what a
// display, a match or a forwarding decision reads.
func LabelValue(entry uint32) uint32 { return entry >> 4 }

// LabelEntryFor builds a stack entry from a label value, with a zero traffic
// class and the bottom-of-stack bit set when bottom is true.
//
// For a caller that HAS a label and no entry: an operator's config, a CLI
// argument, a route this speaker originates. A caller relaying an entry it
// parsed passes that entry through instead, so the peer's traffic class is not
// replaced by this function's zero.
func LabelEntryFor(label uint32, bottom bool) uint32 {
	entry := label << 4
	if bottom {
		entry |= labelBottomOfStack
	}
	return entry
}

// LabelEntriesFor builds a whole stack from label values, setting the
// bottom-of-stack bit on the last.
func LabelEntriesFor(labels []uint32) []uint32 {
	if len(labels) == 0 {
		return nil
	}
	entries := make([]uint32, len(labels))
	for index, label := range labels {
		entries[index] = LabelEntryFor(label, index == len(labels)-1)
	}
	return entries
}

// LabelValues answers the label of every entry in a stack.
func LabelValues(entries []uint32) []uint32 {
	if len(entries) == 0 {
		return nil
	}
	labels := make([]uint32, len(entries))
	for index, entry := range entries {
		labels[index] = LabelValue(entry)
	}
	return labels
}

// EncodeLabelStack encodes label stack ENTRIES to wire format, three octets
// each, and returns a standalone slice.
//
// RFC 3032 Section 2.1 gives each entry its layout, which RFC 8277 Section 2.1
// carries in an NLRI without the data plane's TTL octet:
//
//	Byte 0: label[19:12]
//	Byte 1: label[11:4]
//	Byte 2: label[3:0] | TC[2:0] | S
//
// The entry is written whole, so a traffic class a peer set survives. The S bit
// is the one field WriteLabelStack owns rather than copies, and it sets it on
// the last entry and clears it on the rest.
//
// A caller holding bare 20-bit LABELS rather than entries wants
// LabelEntriesFor first, or WriteLabelValues, which writes them directly.
//
// Retained as a convenience wrapper for JSON and test callers that need a
// standalone slice. Hot-path encoders call WriteLabelStack (helpers.go)
// directly with a pool buffer to skip the make.
func EncodeLabelStack(entries []uint32) []byte {
	buf := make([]byte, len(entries)*3) // pool-fallback: result owned by caller
	WriteLabelStack(buf, 0, entries)
	return buf
}
