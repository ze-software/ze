// Design: docs/architecture/wire/nlri.md — VPN NLRI plugin
// RFC: rfc/short/rfc4364.md
//
// Package vpn implements VPN NLRI types for the vpn plugin.
// RFC 4364: BGP/MPLS IP VPNs (VPNv4)
// RFC 4659: BGP-MPLS IP VPN Extension for IPv6 VPN (VPNv6)
package bgp_nlri_vpn

import (
	"encoding/binary"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// Type aliases for nlri types used by VPN.
type (
	Family             = nlri.Family
	AFI                = nlri.AFI
	SAFI               = nlri.SAFI
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants from nlri for local use.
const (
	AFIIPv4 = nlri.AFIIPv4
	AFIIPv6 = nlri.AFIIPv6
	SAFIVPN = nlri.SAFIVPN
)

// Re-export family constants from nlri.
var (
	IPv4VPN = nlri.IPv4VPN
	IPv6VPN = nlri.IPv6VPN
)

// Re-export parsing functions from nlri (shared types).
var (
	ParseRouteDistinguisher = nlri.ParseRouteDistinguisher
	ParseLabelStack         = nlri.ParseLabelStack
	EncodeLabelStack        = nlri.EncodeLabelStack
	ParseRDString           = nlri.ParseRDString
	PrefixBytes             = nlri.PrefixBytes
)

// VPN errors - re-export from nlri since they apply to VPN parsing.
var (
	ErrShortRead      = nlri.ErrShortRead
	ErrInvalidPrefix  = nlri.ErrInvalidPrefix
	ErrInvalidAddress = nlri.ErrInvalidAddress
)

// WriteLabelStack writes MPLS labels to wire format at offset.
// Returns bytes written.
var WriteLabelStack = nlri.WriteLabelStack

// VPN represents a VPNv4 or VPNv6 NLRI.
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
type VPN struct {
	family Family             // RFC 4364/4659: AFI + SAFI
	rd     RouteDistinguisher // RFC 4364 Section 4.1: 8-byte RD
	labels []uint32           // RFC 3107: MPLS label stack
	prefix netip.Prefix       // IPv4 (RFC 4364) or IPv6 (RFC 4659) prefix
	pathID uint32             // RFC 7911: 0 means no path ID
}

// NewVPN creates a new VPN NLRI.
// pathID=0 means no path identifier; pathID>0 stores the path ID.
// Use WriteNLRI() with addPath=true to encode with path ID.
func NewVPN(family Family, rd RouteDistinguisher, labels []uint32, prefix netip.Prefix, pathID uint32) *VPN {
	return &VPN{
		family: family,
		rd:     rd,
		labels: labels,
		prefix: prefix,
		pathID: pathID,
	}
}

// ParseVPN parses a VPN NLRI from wire format.
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
func ParseVPN(afi AFI, safi SAFI, data []byte, addpath bool) (*VPN, []byte, error) {
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
	prefixBytes := PrefixBytes(prefixBits)

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

	v := &VPN{
		family: Family{AFI: afi, SAFI: safi},
		rd:     rd,
		labels: labels,
		prefix: prefix,
		pathID: pathID,
	}

	return v, data[offset+totalBytes:], nil
}

// Family returns the AFI/SAFI.
// RFC 4364: AFI=1, SAFI=128 for VPNv4.
// RFC 4659: AFI=2, SAFI=128 for VPNv6.
func (v *VPN) Family() Family { return v.family }

// RD returns the Route Distinguisher per RFC 4364 Section 4.1.
func (v *VPN) RD() RouteDistinguisher { return v.rd }

// Labels returns the MPLS label stack per RFC 3107.
func (v *VPN) Labels() []uint32 { return v.labels }

// Prefix returns the IP prefix (IPv4 per RFC 4364, IPv6 per RFC 4659).
func (v *VPN) Prefix() netip.Prefix { return v.prefix }

// PathID returns the ADD-PATH path identifier (0 if none).
func (v *VPN) PathID() uint32 { return v.pathID }

// HasPathID returns true if path ID is set.
func (v *VPN) HasPathID() bool { return v.pathID != 0 }

// SupportsAddPath returns true - VPN NLRIs support ADD-PATH per RFC 7911.
func (v *VPN) SupportsAddPath() bool { return true }

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
func (v *VPN) Bytes() []byte {
	labelBytes := EncodeLabelStack(v.labels)
	rdBytes := v.rd.Bytes()

	prefixBits := v.prefix.Bits()
	prefixBytes := PrefixBytes(prefixBits)

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
func (v *VPN) Len() int {
	return 1 + len(v.labels)*3 + 8 + PrefixBytes(v.prefix.Bits())
}

// String returns command-style format for API round-trip compatibility.
// Format: rd <rd> prefix <prefix> [label <labels>] [path-id <id>].
func (v *VPN) String() string {
	var sb strings.Builder
	sb.WriteString("rd ")
	sb.WriteString(v.rd.String())
	sb.WriteString(" prefix ")
	sb.WriteString(v.prefix.String())
	if len(v.labels) > 0 {
		sb.WriteString(" label ")
		sb.WriteString(strconv.FormatUint(uint64(v.labels[0]), 10))
		for _, l := range v.labels[1:] {
			sb.WriteString(",")
			sb.WriteString(strconv.FormatUint(uint64(l), 10))
		}
	}
	if v.pathID != 0 {
		sb.WriteString(" path-id ")
		sb.WriteString(strconv.FormatUint(uint64(v.pathID), 10))
	}
	return sb.String()
}

// WriteTo writes the NLRI payload (without path ID) into buf at offset.
// Returns number of bytes written.
//
// RFC 4364 Section 4.3.4 / RFC 4659 Section 3.2 - VPN NLRI Format:
// Encodes as [length][labels][RD][prefix] where length is total bits.
//
// RFC 7911 Section 3: Path ID is NOT written by this method.
// Use WriteNLRI() for ADD-PATH encoding with path identifier.
func (v *VPN) WriteTo(buf []byte, off int) int {
	prefixBits := v.prefix.Bits()
	prefixBytes := PrefixBytes(prefixBits)
	labelCount := len(v.labels)

	// RFC 3107: Length field = label bits + RD bits (64) + prefix bits
	totalBits := labelCount*24 + 64 + prefixBits

	pos := off

	// Write length field
	buf[pos] = byte(totalBits)
	pos++

	// Write MPLS labels
	pos += WriteLabelStack(buf, pos, v.labels)

	// Write Route Distinguisher (8 bytes)
	binary.BigEndian.PutUint16(buf[pos:], uint16(v.rd.Type))
	copy(buf[pos+2:], v.rd.Value[:])
	pos += 8

	// Write IP prefix
	copy(buf[pos:], v.prefix.Addr().AsSlice()[:prefixBytes])
	pos += prefixBytes

	return pos - off
}

// VPNFamilies returns the address families this plugin can decode.
func VPNFamilies() []string {
	return []string{"ipv4/vpn", "ipv6/vpn"}
}
