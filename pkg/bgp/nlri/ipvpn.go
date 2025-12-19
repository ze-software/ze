package nlri

import (
	"encoding/binary"
	"fmt"
	"net/netip"
)

// RDType represents Route Distinguisher type.
type RDType uint16

// Route Distinguisher types (RFC 4364).
const (
	RDType0 RDType = 0 // 2-byte ASN : 4-byte assigned
	RDType1 RDType = 1 // 4-byte IP : 2-byte assigned
	RDType2 RDType = 2 // 4-byte ASN : 2-byte assigned
)

// RouteDistinguisher uniquely identifies a VPN route.
// 8 bytes: 2-byte type + 6-byte value.
type RouteDistinguisher struct {
	Type  RDType
	Value [6]byte
}

// ParseRouteDistinguisher parses an RD from 8 bytes.
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

// Bytes returns the wire format.
func (rd RouteDistinguisher) Bytes() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint16(buf[:2], uint16(rd.Type))
	copy(buf[2:], rd.Value[:])
	return buf
}

// String returns a human-readable representation.
func (rd RouteDistinguisher) String() string {
	switch rd.Type {
	case RDType0:
		// 2-byte ASN : 4-byte assigned
		asn := binary.BigEndian.Uint16(rd.Value[:2])
		assigned := binary.BigEndian.Uint32(rd.Value[2:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	case RDType1:
		// 4-byte IP : 2-byte assigned
		ip := netip.AddrFrom4([4]byte(rd.Value[:4]))
		assigned := binary.BigEndian.Uint16(rd.Value[4:6])
		return fmt.Sprintf("%s:%d", ip, assigned)
	case RDType2:
		// 4-byte ASN : 2-byte assigned
		asn := binary.BigEndian.Uint32(rd.Value[:4])
		assigned := binary.BigEndian.Uint16(rd.Value[4:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	default:
		return fmt.Sprintf("rd-type%d:%x", rd.Type, rd.Value)
	}
}

// ParseLabelStack parses MPLS labels from wire format.
// Each label is 3 bytes: 20-bit label, 3-bit TC, 1-bit BOS, 8-bit TTL.
// Returns the label values and remaining bytes.
func ParseLabelStack(data []byte) ([]uint32, []byte, error) {
	var labels []uint32

	for {
		if len(data) < 3 {
			return nil, nil, ErrShortRead
		}

		// Label is in upper 20 bits of 3 bytes
		// Byte 0: label[19:12]
		// Byte 1: label[11:4]
		// Byte 2: label[3:0], TC[2:0], BOS
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

// EncodeLabelStack encodes labels to wire format.
func EncodeLabelStack(labels []uint32) []byte {
	buf := make([]byte, len(labels)*3)
	for i, label := range labels {
		off := i * 3
		buf[off] = byte(label >> 12)
		buf[off+1] = byte(label >> 4)
		buf[off+2] = byte(label<<4) & 0xF0
		if i == len(labels)-1 {
			buf[off+2] |= 0x01 // BOS
		}
	}
	return buf
}

// IPVPN represents a VPNv4 or VPNv6 NLRI.
type IPVPN struct {
	family  Family
	rd      RouteDistinguisher
	labels  []uint32
	prefix  netip.Prefix
	pathID  uint32
	hasPath bool
}

// NewIPVPN creates a new IPVPN NLRI.
func NewIPVPN(family Family, rd RouteDistinguisher, labels []uint32, prefix netip.Prefix, pathID uint32) *IPVPN {
	return &IPVPN{
		family:  family,
		rd:      rd,
		labels:  labels,
		prefix:  prefix,
		pathID:  pathID,
		hasPath: pathID != 0,
	}
}

// ParseIPVPN parses a VPN NLRI from wire format.
func ParseIPVPN(afi AFI, safi SAFI, data []byte, addpath bool) (NLRI, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrShortRead
	}

	offset := 0
	var pathID uint32

	// Parse optional path ID
	if addpath {
		if len(data) < 4 {
			return nil, nil, ErrShortRead
		}
		pathID = binary.BigEndian.Uint32(data[:4])
		offset = 4
	}

	// Parse prefix length (in bits, includes labels + RD + prefix)
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

	// Parse label stack (minimum 3 bytes)
	if len(nlriData) < 3 {
		return nil, nil, ErrShortRead
	}
	labels, nlriData, err := ParseLabelStack(nlriData)
	if err != nil {
		return nil, nil, err
	}
	labelBits := len(labels) * 24

	// Parse RD (8 bytes = 64 bits)
	if len(nlriData) < 8 {
		return nil, nil, ErrShortRead
	}
	rd, err := ParseRouteDistinguisher(nlriData[:8])
	if err != nil {
		return nil, nil, err
	}
	nlriData = nlriData[8:]
	rdBits := 64

	// Remaining bits are prefix
	prefixBits := totalBits - labelBits - rdBits
	if prefixBits < 0 {
		return nil, nil, ErrInvalidPrefix
	}
	prefixBytes := (prefixBits + 7) / 8

	if len(nlriData) < prefixBytes {
		return nil, nil, ErrShortRead
	}

	// Build address
	var addr netip.Addr
	if afi == AFIIPv4 {
		var ip [4]byte
		copy(ip[:], nlriData[:prefixBytes])
		addr = netip.AddrFrom4(ip)
	} else {
		var ip [16]byte
		copy(ip[:], nlriData[:prefixBytes])
		addr = netip.AddrFrom16(ip)
	}

	prefix, err := addr.Prefix(prefixBits)
	if err != nil {
		return nil, nil, ErrInvalidAddress
	}

	vpn := &IPVPN{
		family:  Family{AFI: afi, SAFI: safi},
		rd:      rd,
		labels:  labels,
		prefix:  prefix,
		pathID:  pathID,
		hasPath: addpath,
	}

	return vpn, data[offset+totalBytes:], nil
}

// Family returns the AFI/SAFI.
func (v *IPVPN) Family() Family { return v.family }

// RD returns the Route Distinguisher.
func (v *IPVPN) RD() RouteDistinguisher { return v.rd }

// Labels returns the MPLS label stack.
func (v *IPVPN) Labels() []uint32 { return v.labels }

// Prefix returns the IP prefix.
func (v *IPVPN) Prefix() netip.Prefix { return v.prefix }

// PathID returns the ADD-PATH path identifier.
func (v *IPVPN) PathID() uint32 { return v.pathID }

// HasPathID returns true if this NLRI has a path ID.
func (v *IPVPN) HasPathID() bool { return v.hasPath }

// Bytes returns the wire format.
func (v *IPVPN) Bytes() []byte {
	labelBytes := EncodeLabelStack(v.labels)
	rdBytes := v.rd.Bytes()

	prefixBits := v.prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8

	totalBits := len(labelBytes)*8 + 64 + prefixBits

	var buf []byte
	if v.hasPath {
		buf = make([]byte, 4+1+len(labelBytes)+8+prefixBytes)
		binary.BigEndian.PutUint32(buf[:4], v.pathID)
		buf[4] = byte(totalBits)
		copy(buf[5:], labelBytes)
		copy(buf[5+len(labelBytes):], rdBytes)
		copy(buf[5+len(labelBytes)+8:], v.prefix.Addr().AsSlice()[:prefixBytes])
	} else {
		buf = make([]byte, 1+len(labelBytes)+8+prefixBytes)
		buf[0] = byte(totalBits)
		copy(buf[1:], labelBytes)
		copy(buf[1+len(labelBytes):], rdBytes)
		copy(buf[1+len(labelBytes)+8:], v.prefix.Addr().AsSlice()[:prefixBytes])
	}

	return buf
}

// Len returns the wire format length.
func (v *IPVPN) Len() int {
	prefixBytes := (v.prefix.Bits() + 7) / 8
	n := 1 + len(v.labels)*3 + 8 + prefixBytes
	if v.hasPath {
		n += 4
	}
	return n
}

// String returns a human-readable representation.
func (v *IPVPN) String() string {
	s := fmt.Sprintf("RD:%s %s", v.rd, v.prefix)
	if len(v.labels) > 0 {
		s = fmt.Sprintf("%s labels=%v", s, v.labels)
	}
	if v.hasPath {
		s = fmt.Sprintf("%s path-id=%d", s, v.pathID)
	}
	return s
}
