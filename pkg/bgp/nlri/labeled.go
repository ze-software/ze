// Package nlri implements BGP Network Layer Reachability Information encoding.
//
// This file implements Labeled Unicast NLRI (SAFI 4) per RFC 8277.

package nlri

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"
)

// LabeledUnicast represents a labeled unicast NLRI (SAFI 4).
//
// RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes.
// RFC 8277 Section 2.2 - NLRI format (without Multiple Labels Capability):
//
//	+---------------------------+
//	|   Length (1 octet)        |  = 24*N + prefix_bits (N = number of labels)
//	+---------------------------+
//	|   Label (3 octets)        |  20-bit label + 3-bit TC + 1-bit S
//	+---------------------------+
//	|   Prefix (variable)       |
//	+---------------------------+
//
// RFC 7911 Section 3 - Extended NLRI with ADD-PATH:
//
//	+---------------------------+
//	|   Path ID (4 octets)      |  Only when ADD-PATH negotiated
//	+---------------------------+
//	|   Length (1 octet)        |
//	+---------------------------+
//	|   Label(s) (3*N octets)   |
//	+---------------------------+
//	|   Prefix (variable)       |
//	+---------------------------+
type LabeledUnicast struct {
	family  Family
	prefix  netip.Prefix
	labels  []uint32 // Label stack per RFC 3032 (BOS on last)
	pathID  uint32
	hasPath bool
}

// NewLabeledUnicast creates a new labeled unicast NLRI.
//
// RFC 8277: Labels are encoded per RFC 3032: 20-bit label + 3-bit TC + 1-bit S.
// The last label has S=1 (Bottom of Stack).
//
// The family's SAFI is overridden to SAFIMPLSLabel (4) regardless of input.
func NewLabeledUnicast(family Family, prefix netip.Prefix, labels []uint32, pathID uint32) *LabeledUnicast {
	return &LabeledUnicast{
		family:  Family{AFI: family.AFI, SAFI: SAFIMPLSLabel},
		prefix:  prefix,
		labels:  labels,
		pathID:  pathID,
		hasPath: pathID != 0,
	}
}

// Family returns the AFI/SAFI for this NLRI.
// SAFI is always SAFIMPLSLabel (4) per RFC 8277.
func (l *LabeledUnicast) Family() Family {
	return l.family
}

// Prefix returns the IP prefix.
func (l *LabeledUnicast) Prefix() netip.Prefix {
	return l.prefix
}

// Labels returns the MPLS label stack.
func (l *LabeledUnicast) Labels() []uint32 {
	return l.labels
}

// PathID returns the ADD-PATH path identifier.
func (l *LabeledUnicast) PathID() uint32 {
	return l.pathID
}

// HasPathID returns true if this NLRI has an ADD-PATH path ID.
func (l *LabeledUnicast) HasPathID() bool {
	return l.hasPath
}

// encodeLabel encodes a single MPLS label to 3 bytes per RFC 3032.
//
// RFC 3032 - MPLS Label Stack Encoding:
//
//	 0                   1                   2
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|          Label Value (20 bits)        |TC |S|
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Label Value: 20 bits (0-1048575)
// TC: 3 bits (Traffic Class, set to 0)
// S: 1 bit (Stack bit: 0=more labels, 1=bottom of stack)
//
// NOTE: RFC 3032 data plane uses 4 bytes (includes TTL).
// BGP uses 3 bytes (no TTL) per RFC 8277.
func encodeLabel(label uint32, bos bool) []byte {
	// TC = 0, S = bos ? 1 : 0
	s := byte(0)
	if bos {
		s = 1
	}
	return []byte{
		byte(label >> 12),
		byte(label >> 4),
		byte(label<<4) | s,
	}
}

// Bytes returns the wire-format encoding.
//
// RFC 8277 Section 2.2 - NLRI Encoding:
// [PathID (4 bytes, optional)][Length (1 byte)][Labels (3*N bytes)][Prefix (variable)]
//
// RFC 7911 Section 3: When HasPathID() is true, prepends 4-octet Path Identifier.
func (l *LabeledUnicast) Bytes() []byte {
	prefixBits := l.prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	labelBytes := len(l.labels) * 3

	// Total bits: 24 per label + prefix bits
	totalBits := len(l.labels)*24 + prefixBits

	// Calculate buffer size
	size := 1 + labelBytes + prefixBytes // length + labels + prefix
	if l.hasPath {
		size += 4 // path ID
	}

	buf := make([]byte, size)
	offset := 0

	// Path ID (optional)
	if l.hasPath {
		binary.BigEndian.PutUint32(buf[:4], l.pathID)
		offset = 4
	}

	// Length byte
	buf[offset] = byte(totalBits)
	offset++

	// Encode labels
	for i, label := range l.labels {
		bos := i == len(l.labels)-1 // Last label has BOS=1
		labelEncoded := encodeLabel(label, bos)
		copy(buf[offset:offset+3], labelEncoded)
		offset += 3
	}

	// Prefix bytes
	if prefixBytes > 0 {
		copy(buf[offset:], l.prefix.Addr().AsSlice()[:prefixBytes])
	}

	return buf
}

// Pack returns wire-format bytes adapted for negotiated capabilities.
//
// RFC 7911 Section 3 - Extended NLRI Encodings:
// When ADD-PATH is negotiated, NLRI is encoded with Path Identifier.
//
// Behavior:
//   - If ctx is nil: returns Bytes() (no capability adaptation)
//   - If ctx.AddPath=true and HasPathID()=true: returns with path ID
//   - If ctx.AddPath=true and HasPathID()=false: prepends NOPATH (4 zeros)
//   - If ctx.AddPath=false: returns without path ID (strips if present)
func (l *LabeledUnicast) Pack(ctx *PackContext) []byte {
	if ctx == nil {
		return l.Bytes()
	}

	if ctx.AddPath {
		if l.hasPath {
			return l.Bytes() // Already has path ID
		}
		// Prepend NOPATH (4 zero bytes) - RFC 7911 requires path ID when negotiated
		return append([]byte{0, 0, 0, 0}, l.Bytes()...)
	}

	// No ADD-PATH: strip path ID if present
	if l.hasPath {
		return l.Bytes()[4:] // Skip 4-byte path ID
	}
	return l.Bytes()
}

// Len returns the wire-format length in bytes.
func (l *LabeledUnicast) Len() int {
	prefixBytes := (l.prefix.Bits() + 7) / 8
	labelBytes := len(l.labels) * 3
	size := 1 + labelBytes + prefixBytes
	if l.hasPath {
		size += 4
	}
	return size
}

// String returns a human-readable representation.
func (l *LabeledUnicast) String() string {
	var sb strings.Builder
	sb.WriteString(l.prefix.String())

	if len(l.labels) == 1 {
		fmt.Fprintf(&sb, " label=%d", l.labels[0])
	} else if len(l.labels) > 1 {
		labels := make([]string, len(l.labels))
		for i, lbl := range l.labels {
			labels[i] = fmt.Sprintf("%d", lbl)
		}
		fmt.Fprintf(&sb, " labels=[%s]", strings.Join(labels, ","))
	}

	if l.hasPath {
		fmt.Fprintf(&sb, " path-id=%d", l.pathID)
	}

	return sb.String()
}
