// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin
// RFC: rfc/short/rfc8277.md
//
// Package bgp_labeled implements Labeled Unicast NLRI (RFC 8277, SAFI 4).
package bgp_nlri_labeled

import (
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// Type aliases for shared nlri types.
type (
	Family = nlri.Family
	AFI    = nlri.AFI
	SAFI   = nlri.SAFI
	NLRI   = nlri.NLRI
)

// Re-export constants.
const (
	AFIIPv4       = nlri.AFIIPv4
	AFIIPv6       = nlri.AFIIPv6
	SAFIMPLSLabel = nlri.SAFIMPLSLabel
)

// LabeledUnicast represents a labeled unicast NLRI (SAFI 4).
//
// RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes.
// RFC 8277 Section 2.2 - NLRI format:
//
//	+---------------------------+
//	|   Length (1 octet)        |  = 24*N + prefix_bits (N = number of labels)
//	+---------------------------+
//	|   Label (3 octets)        |  20-bit label + 3-bit TC + 1-bit S
//	+---------------------------+
//	|   Prefix (variable)       |
//	+---------------------------+
//
// RFC 7911 Section 3 - Extended NLRI Encodings:
// Path ID is stored but NOT included in Len()/Bytes()/WriteTo().
// Use WriteNLRI() for ADD-PATH aware encoding.
type LabeledUnicast struct {
	family Family
	prefix netip.Prefix
	pathID uint32   // RFC 7911: 0 means no path ID
	labels []uint32 // Label stack per RFC 3032 (BOS on last)
}

// NewLabeledUnicast creates a new labeled unicast NLRI.
//
// RFC 8277: Labels are encoded per RFC 3032: 20-bit label + 3-bit TC + 1-bit S.
// The last label has S=1 (Bottom of Stack).
//
// pathID=0 means no path identifier; pathID>0 stores the path ID.
// Use WriteNLRI() with addPath=true to encode with path ID.
// The family's SAFI is overridden to SAFIMPLSLabel (4) regardless of input.
func NewLabeledUnicast(family Family, prefix netip.Prefix, labels []uint32, pathID uint32) *LabeledUnicast {
	return &LabeledUnicast{
		family: Family{AFI: family.AFI, SAFI: SAFIMPLSLabel},
		prefix: prefix,
		pathID: pathID,
		labels: labels,
	}
}

// Family returns the AFI/SAFI for this NLRI.
func (l *LabeledUnicast) Family() Family { return l.family }

// Prefix returns the IP prefix.
func (l *LabeledUnicast) Prefix() netip.Prefix { return l.prefix }

// PathID returns the ADD-PATH path identifier (0 if none).
func (l *LabeledUnicast) PathID() uint32 { return l.pathID }

// HasPathID returns true if a path ID is set.
func (l *LabeledUnicast) HasPathID() bool { return l.pathID != 0 }

// SupportsAddPath returns true - labeled unicast supports ADD-PATH per RFC 7911.
func (l *LabeledUnicast) SupportsAddPath() bool { return true }

// Labels returns the MPLS label stack.
func (l *LabeledUnicast) Labels() []uint32 {
	return l.labels
}

// Bytes returns the wire-format encoding (payload only, no path ID).
//
// RFC 8277 Section 2.2 - NLRI Encoding:
// [Length (1 byte)][Labels (3*N bytes)][Prefix (variable)]
//
// Note: Path ID is NOT included. Use WriteNLRI() for ADD-PATH encoding.
func (l *LabeledUnicast) Bytes() []byte {
	prefixBits := l.prefix.Bits()
	prefixBytes := nlri.PrefixBytes(prefixBits)
	labelBytes := nlri.EncodeLabelStack(l.labels)

	// Total bits: 24 per label + prefix bits
	totalBits := len(l.labels)*24 + prefixBits

	// Calculate buffer size: length (1) + labels + prefix
	buf := make([]byte, 1+len(labelBytes)+prefixBytes)

	// Length byte
	buf[0] = byte(totalBits)

	// Copy encoded labels
	copy(buf[1:], labelBytes)

	// Prefix bytes
	if prefixBytes > 0 {
		copy(buf[1+len(labelBytes):], l.prefix.Addr().AsSlice()[:prefixBytes])
	}

	return buf
}

// Len returns the wire-format length in bytes (payload only, no path ID).
func (l *LabeledUnicast) Len() int {
	return 1 + len(l.labels)*3 + nlri.PrefixBytes(l.prefix.Bits())
}

// WriteTo writes the NLRI payload (without path ID) into buf at offset.
// Returns number of bytes written.
//
// RFC 8277 Section 2.2 - Labeled Unicast NLRI Format:
// Encodes as [length][labels][prefix] where length is total bits.
//
// RFC 7911 Section 3: Path ID is NOT written by this method.
// Use WriteNLRI() for ADD-PATH encoding with path identifier.
func (l *LabeledUnicast) WriteTo(buf []byte, off int) int {
	prefixBits := l.prefix.Bits()
	prefixBytes := nlri.PrefixBytes(prefixBits)

	// Total bits: 24 per label + prefix bits
	totalBits := len(l.labels)*24 + prefixBits

	pos := off

	// Length byte
	buf[pos] = byte(totalBits)
	pos++

	// Encode labels (zero-alloc)
	pos += nlri.WriteLabelStack(buf, pos, l.labels)

	// Prefix bytes
	if prefixBytes > 0 {
		copy(buf[pos:], l.prefix.Addr().AsSlice()[:prefixBytes])
		pos += prefixBytes
	}

	return pos - off
}

// String returns command-style format for API round-trip compatibility.
// Format: prefix <prefix> [label <labels>] [path-id <id>].
func (l *LabeledUnicast) String() string {
	var sb strings.Builder
	sb.WriteString("prefix ")
	sb.WriteString(l.prefix.String())
	if len(l.labels) > 0 {
		sb.WriteString(" label ")
		fmt.Fprintf(&sb, "%d", l.labels[0])
		for _, lbl := range l.labels[1:] {
			sb.WriteString(",")
			fmt.Fprintf(&sb, "%d", lbl)
		}
	}
	if l.pathID != 0 {
		sb.WriteString(" path-id ")
		fmt.Fprintf(&sb, "%d", l.pathID)
	}
	return sb.String()
}
