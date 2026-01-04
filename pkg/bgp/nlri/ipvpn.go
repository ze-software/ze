// Package nlri implements BGP Network Layer Reachability Information types.
package nlri

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
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
func (rd RouteDistinguisher) Bytes() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint16(buf[:2], uint16(rd.Type))
	copy(buf[2:], rd.Value[:])
	return buf
}

// String returns a human-readable representation.
//
// Per RFC 4364 Section 4.2, the string format depends on RD type:
//   - Type 0: "ASN:assigned" (e.g., "65000:100")
//   - Type 1: "IP:assigned" (e.g., "192.0.2.1:100")
//   - Type 2: "ASN:assigned" (e.g., "65000:100", 4-byte ASN)
func (rd RouteDistinguisher) String() string {
	switch rd.Type {
	case RDType0:
		// RFC 4364 Section 4.2 Type 0: 2-byte ASN : 4-byte assigned
		asn := binary.BigEndian.Uint16(rd.Value[:2])
		assigned := binary.BigEndian.Uint32(rd.Value[2:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	case RDType1:
		// RFC 4364 Section 4.2 Type 1: 4-byte IP : 2-byte assigned
		ip := netip.AddrFrom4([4]byte(rd.Value[:4]))
		assigned := binary.BigEndian.Uint16(rd.Value[4:6])
		return fmt.Sprintf("%s:%d", ip, assigned)
	case RDType2:
		// RFC 4364 Section 4.2 Type 2: 4-byte ASN : 2-byte assigned
		asn := binary.BigEndian.Uint32(rd.Value[:4])
		assigned := binary.BigEndian.Uint16(rd.Value[4:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	default:
		return fmt.Sprintf("rd-type%d:%x", rd.Type, rd.Value)
	}
}

// ParseRDString parses a Route Distinguisher from string format.
//
// RFC 4364 Section 4.2 defines RD types:
//   - Type 0: "ASN:value" (2-byte ASN, 4-byte value) e.g., "65000:100"
//   - Type 1: "IP:value" (4-byte IP, 2-byte value) e.g., "192.0.2.1:100"
//   - Type 2: "ASN:value" (4-byte ASN, 2-byte value) e.g., "4200000001:100"
//
// Detection:
//   - If first part contains "." → Type 1 (IP:value)
//   - If ASN > 65535 → Type 2 (4-byte ASN, 2-byte value)
//   - Otherwise → Type 0 (2-byte ASN, 4-byte value)
func ParseRDString(s string) (RouteDistinguisher, error) {
	var rd RouteDistinguisher
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return rd, fmt.Errorf("invalid RD format: %s (expected ASN:value or IP:value)", s)
	}

	// Check if first part is an IP address (Type 1)
	if strings.Contains(parts[0], ".") {
		ip, err := netip.ParseAddr(parts[0])
		if err != nil || !ip.Is4() {
			return rd, fmt.Errorf("invalid IP in RD: %s", parts[0])
		}
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

	// Parse ASN to determine Type 0 vs Type 2
	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return rd, fmt.Errorf("invalid ASN in RD: %s", parts[0])
	}

	if asn > 65535 {
		// Type 2: 4-byte ASN : 2-byte value
		val, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return rd, fmt.Errorf("invalid RD value (must be 0-65535 for 4-byte ASN): %s", parts[1])
		}
		rd.Type = RDType2
		rd.Value[0] = byte(asn >> 24)
		rd.Value[1] = byte(asn >> 16)
		rd.Value[2] = byte(asn >> 8)
		rd.Value[3] = byte(asn)
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
	rd.Value[0] = byte(asn >> 8)
	rd.Value[1] = byte(asn)
	rd.Value[2] = byte(val >> 24)
	rd.Value[3] = byte(val >> 16)
	rd.Value[4] = byte(val >> 8)
	rd.Value[5] = byte(val)
	return rd, nil
}

// ParseLabelStack parses MPLS labels from wire format.
//
// RFC 3107 (MPLS-BGP) specifies label encoding for BGP NLRI:
//
//	Each label is 3 bytes: 20-bit label value, 3-bit EXP/TC, 1-bit S (BOS)
//
// RFC 4364 Section 4.3.2 states PE routers distribute labeled VPN-IPv4 routes.
// RFC 4659 Section 3.2 extends this for labeled VPN-IPv6 routes.
//
// Returns the label values and remaining bytes.
func ParseLabelStack(data []byte) ([]uint32, []byte, error) {
	var labels []uint32

	for {
		if len(data) < 3 {
			return nil, nil, ErrShortRead
		}

		// RFC 3107: Label is in upper 20 bits of 3 bytes
		// Byte 0: label[19:12]
		// Byte 1: label[11:4]
		// Byte 2: label[3:0], EXP[2:0], S (bottom-of-stack)
		labelVal := uint32(data[0])<<12 | uint32(data[1])<<4 | uint32(data[2]>>4)
		bos := data[2]&0x01 != 0

		labels = append(labels, labelVal)
		data = data[3:]

		if bos {
			break
		}
	}

	return labels, data, nil
}

// EncodeLabelStack encodes labels to wire format per RFC 3107.
//
// RFC 3107 label encoding (3 bytes per label):
//
//	Byte 0: label[19:12]
//	Byte 1: label[11:4]
//	Byte 2: label[3:0] | EXP[2:0] | S
//
// The S (bottom-of-stack) bit is set on the last label only.
func EncodeLabelStack(labels []uint32) []byte {
	buf := make([]byte, len(labels)*3)
	for i, label := range labels {
		off := i * 3
		buf[off] = byte(label >> 12)
		buf[off+1] = byte(label >> 4)
		buf[off+2] = byte(label<<4) & 0xF0
		if i == len(labels)-1 {
			buf[off+2] |= 0x01 // RFC 3107: S (bottom-of-stack) bit
		}
	}
	return buf
}

// IPVPN represents a VPNv4 or VPNv6 NLRI.
//
// RFC 4364 Section 4.3.4 defines VPNv4 NLRI encoding:
//   - AFI=1 (IPv4), SAFI=128 (MPLS-labeled VPN)
//   - Prefix = MPLS label(s) + 8-byte RD + IPv4 prefix
//
// RFC 4659 Section 3.2 defines VPNv6 NLRI encoding:
//   - AFI=2 (IPv6), SAFI=128 (MPLS-labeled VPN)
//   - Prefix = MPLS label(s) + 8-byte RD + IPv6 prefix
//
// RFC 7911 Section 3 - Extended NLRI Encodings:
// Path ID is stored but NOT included in Len()/Bytes()/WriteTo().
// Use WriteNLRI() for ADD-PATH aware encoding.
type IPVPN struct {
	family Family             // RFC 4364/4659: AFI + SAFI
	rd     RouteDistinguisher // RFC 4364 Section 4.1: 8-byte RD
	labels []uint32           // RFC 3107: MPLS label stack
	prefix netip.Prefix       // IPv4 (RFC 4364) or IPv6 (RFC 4659) prefix
	pathID uint32             // RFC 7911: 0 means no path ID
}

// NewIPVPN creates a new IPVPN NLRI.
// pathID=0 means no path identifier; pathID>0 stores the path ID.
// Use WriteNLRI() with PackContext.AddPath=true to encode with path ID.
func NewIPVPN(family Family, rd RouteDistinguisher, labels []uint32, prefix netip.Prefix, pathID uint32) *IPVPN {
	return &IPVPN{
		family: family,
		rd:     rd,
		labels: labels,
		prefix: prefix,
		pathID: pathID,
	}
}

// ParseIPVPN parses a VPN NLRI from wire format.
//
// RFC 4364 Section 4.3.4 and RFC 4659 Section 3.2 define the NLRI encoding.
// Per RFC 3107, the labeled VPN NLRI format is:
//
//	+---------------------------+
//	|   Length (1 octet)        |  Total bits: labels + RD + prefix
//	+---------------------------+
//	|   MPLS Label (3+ octets)  |  One or more 3-byte labels
//	+---------------------------+
//	|   Route Distinguisher     |  8 octets (RFC 4364 Section 4.2)
//	|   (8 octets)              |
//	+---------------------------+
//	|   IP Prefix               |  Variable length
//	|   (variable)              |
//	+---------------------------+
//
// For VPNv4 (RFC 4364): AFI=1, SAFI=128.
// For VPNv6 (RFC 4659): AFI=2, SAFI=128.
func ParseIPVPN(afi AFI, safi SAFI, data []byte, addpath bool) (NLRI, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrShortRead
	}

	offset := 0
	var pathID uint32

	// RFC 7911: Parse optional ADD-PATH path identifier
	if addpath {
		if len(data) < 4 {
			return nil, nil, ErrShortRead
		}
		pathID = binary.BigEndian.Uint32(data[:4])
		offset = 4
	}

	// RFC 3107: Parse prefix length (in bits, includes labels + RD + prefix)
	if offset >= len(data) {
		return nil, nil, ErrShortRead
	}
	totalBits := int(data[offset])
	offset++

	// Calculate bytes needed
	totalBytes := (totalBits + 7) / 8
	if offset+totalBytes > len(data) {
		return nil, nil, ErrShortRead
	}

	nlriData := data[offset : offset+totalBytes]

	// RFC 3107: Parse MPLS label stack (minimum 3 bytes per label)
	if len(nlriData) < 3 {
		return nil, nil, ErrShortRead
	}
	labels, nlriData, err := ParseLabelStack(nlriData)
	if err != nil {
		return nil, nil, err
	}
	labelBits := len(labels) * 24

	// RFC 4364 Section 4.1/4.2: Parse RD (8 bytes = 64 bits)
	if len(nlriData) < 8 {
		return nil, nil, ErrShortRead
	}
	rd, err := ParseRouteDistinguisher(nlriData[:8])
	if err != nil {
		return nil, nil, err
	}
	nlriData = nlriData[8:]
	rdBits := 64

	// Remaining bits are IP prefix (IPv4 per RFC 4364, IPv6 per RFC 4659)
	prefixBits := totalBits - labelBits - rdBits
	if prefixBits < 0 {
		return nil, nil, ErrInvalidPrefix
	}
	prefixBytes := (prefixBits + 7) / 8

	if len(nlriData) < prefixBytes {
		return nil, nil, ErrShortRead
	}

	// Build address based on AFI
	var addr netip.Addr
	if afi == AFIIPv4 {
		// RFC 4364: VPN-IPv4 (12-byte: 8-byte RD + 4-byte IPv4)
		var ip [4]byte
		copy(ip[:], nlriData[:prefixBytes])
		addr = netip.AddrFrom4(ip)
	} else {
		// RFC 4659: VPN-IPv6 (24-byte: 8-byte RD + 16-byte IPv6)
		var ip [16]byte
		copy(ip[:], nlriData[:prefixBytes])
		addr = netip.AddrFrom16(ip)
	}

	prefix, err := addr.Prefix(prefixBits)
	if err != nil {
		return nil, nil, ErrInvalidAddress
	}

	vpn := &IPVPN{
		family: Family{AFI: afi, SAFI: safi},
		rd:     rd,
		labels: labels,
		prefix: prefix,
		pathID: pathID,
	}

	return vpn, data[offset+totalBytes:], nil
}

// Family returns the AFI/SAFI.
// RFC 4364: AFI=1, SAFI=128 for VPNv4.
// RFC 4659: AFI=2, SAFI=128 for VPNv6.
func (v *IPVPN) Family() Family { return v.family }

// RD returns the Route Distinguisher per RFC 4364 Section 4.1.
func (v *IPVPN) RD() RouteDistinguisher { return v.rd }

// Labels returns the MPLS label stack per RFC 3107.
func (v *IPVPN) Labels() []uint32 { return v.labels }

// Prefix returns the IP prefix (IPv4 per RFC 4364, IPv6 per RFC 4659).
func (v *IPVPN) Prefix() netip.Prefix { return v.prefix }

// PathID returns the ADD-PATH path identifier (0 if none).
func (v *IPVPN) PathID() uint32 { return v.pathID }

// Bytes returns the wire format (payload only, no path ID).
//
// Wire format:
//
//	[Length (1 byte)]              Total bits of labels + RD + prefix
//	[MPLS Labels (3+ bytes)]       RFC 3107 label stack
//	[Route Distinguisher (8 bytes)] RFC 4364 Section 4.2
//	[IP Prefix (variable)]         IPv4 (RFC 4364) or IPv6 (RFC 4659)
//
// Note: Path ID is NOT included. Use WriteNLRI() for ADD-PATH encoding.
func (v *IPVPN) Bytes() []byte {
	labelBytes := EncodeLabelStack(v.labels)
	rdBytes := v.rd.Bytes()

	prefixBits := v.prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8

	// RFC 3107: Length field = label bits + RD bits (64) + prefix bits
	totalBits := len(labelBytes)*8 + 64 + prefixBits

	buf := make([]byte, 1+len(labelBytes)+8+prefixBytes)
	buf[0] = byte(totalBits)
	copy(buf[1:], labelBytes)
	copy(buf[1+len(labelBytes):], rdBytes)
	copy(buf[1+len(labelBytes)+8:], v.prefix.Addr().AsSlice()[:prefixBytes])

	return buf
}

// Len returns the wire format length (payload only, no path ID).
// Use LenWithContext() for ADD-PATH aware length calculation.
func (v *IPVPN) Len() int {
	prefixBytes := (v.prefix.Bits() + 7) / 8
	return 1 + len(v.labels)*3 + 8 + prefixBytes
}

// String returns a human-readable representation.
func (v *IPVPN) String() string {
	s := fmt.Sprintf("RD:%s %s", v.rd, v.prefix)
	if len(v.labels) > 0 {
		s = fmt.Sprintf("%s labels=%v", s, v.labels)
	}
	if v.pathID != 0 {
		s = fmt.Sprintf("%s path-id=%d", s, v.pathID)
	}
	return s
}

// WriteTo writes the NLRI payload (without path ID) into buf at offset.
// Returns number of bytes written.
//
// RFC 4364 Section 4.3.4 / RFC 4659 Section 3.2 - VPN NLRI Format:
// Encodes as [length][labels][RD][prefix] where length is total bits.
//
// RFC 7911 Section 3: Path ID is NOT written by this method.
// Use WriteNLRI() for ADD-PATH encoding with path identifier.
func (v *IPVPN) WriteTo(buf []byte, off int, _ *PackContext) int {
	prefixBits := v.prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	labelCount := len(v.labels)

	// RFC 3107: Length field = label bits + RD bits (64) + prefix bits
	totalBits := labelCount*24 + 64 + prefixBits

	pos := off

	// Write length field
	buf[pos] = byte(totalBits)
	pos++

	// Write MPLS labels
	pos += writeLabelStack(buf, pos, v.labels)

	// Write Route Distinguisher (8 bytes)
	binary.BigEndian.PutUint16(buf[pos:], uint16(v.rd.Type))
	copy(buf[pos+2:], v.rd.Value[:])
	pos += 8

	// Write IP prefix
	copy(buf[pos:], v.prefix.Addr().AsSlice()[:prefixBytes])
	pos += prefixBytes

	return pos - off
}

// Pack returns wire-format bytes adapted for negotiated capabilities.
//
// Deprecated: Use WriteNLRI() for zero-allocation encoding.
// This method allocates a new slice; prefer WriteNLRI() with pre-allocated buffer.
func (v *IPVPN) Pack(ctx *PackContext) []byte {
	size := LenWithContext(v, ctx)
	buf := make([]byte, size)
	WriteNLRI(v, buf, 0, ctx)
	return buf
}

// writeLabelStack writes MPLS labels to buf at offset.
// Returns number of bytes written.
func writeLabelStack(buf []byte, off int, labels []uint32) int {
	for i, label := range labels {
		pos := off + i*3
		buf[pos] = byte(label >> 12)
		buf[pos+1] = byte(label >> 4)
		buf[pos+2] = byte(label<<4) & 0xF0
		if i == len(labels)-1 {
			buf[pos+2] |= 0x01 // RFC 3107: S (bottom-of-stack) bit
		}
	}
	return len(labels) * 3
}
