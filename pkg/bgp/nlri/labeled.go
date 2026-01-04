// Package nlri implements BGP Network Layer Reachability Information encoding.
//
// This file implements Labeled Unicast NLRI (SAFI 4) per RFC 8277.

package nlri

import (
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
// RFC 7911 Section 3 - Extended NLRI Encodings:
// Path ID is stored but NOT included in Len()/Bytes()/WriteTo().
// Use WriteNLRI() for ADD-PATH aware encoding.
type LabeledUnicast struct {
	family Family
	prefix netip.Prefix
	labels []uint32 // Label stack per RFC 3032 (BOS on last)
	pathID uint32   // RFC 7911: 0 means no path ID
}

// NewLabeledUnicast creates a new labeled unicast NLRI.
//
// RFC 8277: Labels are encoded per RFC 3032: 20-bit label + 3-bit TC + 1-bit S.
// The last label has S=1 (Bottom of Stack).
//
// pathID=0 means no path identifier; pathID>0 stores the path ID.
// Use WriteNLRI() with PackContext.AddPath=true to encode with path ID.
// The family's SAFI is overridden to SAFIMPLSLabel (4) regardless of input.
func NewLabeledUnicast(family Family, prefix netip.Prefix, labels []uint32, pathID uint32) *LabeledUnicast {
	return &LabeledUnicast{
		family: Family{AFI: family.AFI, SAFI: SAFIMPLSLabel},
		prefix: prefix,
		labels: labels,
		pathID: pathID,
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

// PathID returns the ADD-PATH path identifier (0 if none).
func (l *LabeledUnicast) PathID() uint32 {
	return l.pathID
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

// Bytes returns the wire-format encoding (payload only, no path ID).
//
// RFC 8277 Section 2.2 - NLRI Encoding:
// [Length (1 byte)][Labels (3*N bytes)][Prefix (variable)]
//
// Note: Path ID is NOT included. Use WriteNLRI() for ADD-PATH encoding.
func (l *LabeledUnicast) Bytes() []byte {
	prefixBits := l.prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	labelBytes := len(l.labels) * 3

	// Total bits: 24 per label + prefix bits
	totalBits := len(l.labels)*24 + prefixBits

	// Calculate buffer size
	size := 1 + labelBytes + prefixBytes // length + labels + prefix

	buf := make([]byte, size)
	offset := 0

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

// Len returns the wire-format length in bytes (payload only, no path ID).
// Use LenWithContext() for ADD-PATH aware length calculation.
func (l *LabeledUnicast) Len() int {
	prefixBytes := (l.prefix.Bits() + 7) / 8
	labelBytes := len(l.labels) * 3
	return 1 + labelBytes + prefixBytes
}

// WriteTo writes the NLRI payload (without path ID) into buf at offset.
// Returns number of bytes written.
//
// RFC 8277 Section 2.2 - Labeled Unicast NLRI Format:
// Encodes as [length][labels][prefix] where length is total bits.
//
// RFC 7911 Section 3: Path ID is NOT written by this method.
// Use WriteNLRI() for ADD-PATH encoding with path identifier.
func (l *LabeledUnicast) WriteTo(buf []byte, off int, _ *PackContext) int {
	prefixBits := l.prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	labelCount := len(l.labels)

	// Total bits: 24 per label + prefix bits
	totalBits := labelCount*24 + prefixBits

	pos := off

	// Length byte
	buf[pos] = byte(totalBits)
	pos++

	// Encode labels
	for i, label := range l.labels {
		bos := i == labelCount-1 // Last label has BOS=1
		buf[pos] = byte(label >> 12)
		buf[pos+1] = byte(label >> 4)
		buf[pos+2] = byte(label<<4) & 0xF0
		if bos {
			buf[pos+2] |= 0x01
		}
		pos += 3
	}

	// Prefix bytes
	if prefixBytes > 0 {
		copy(buf[pos:], l.prefix.Addr().AsSlice()[:prefixBytes])
		pos += prefixBytes
	}

	return pos - off
}

// Pack returns wire-format bytes adapted for negotiated capabilities.
//
// Deprecated: Use WriteNLRI() for zero-allocation encoding.
// This method allocates a new slice; prefer WriteNLRI() with pre-allocated buffer.
func (l *LabeledUnicast) Pack(ctx *PackContext) []byte {
	size := LenWithContext(l, ctx)
	buf := make([]byte, size)
	WriteNLRI(l, buf, 0, ctx)
	return buf
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

	if l.pathID != 0 {
		fmt.Fprintf(&sb, " path-id=%d", l.pathID)
	}

	return sb.String()
}
