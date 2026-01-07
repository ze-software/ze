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
	PrefixNLRI
	labels []uint32 // Label stack per RFC 3032 (BOS on last)
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
		PrefixNLRI: PrefixNLRI{
			family: Family{AFI: family.AFI, SAFI: SAFIMPLSLabel},
			prefix: prefix,
			pathID: pathID,
		},
		labels: labels,
	}
}

// Family, Prefix, PathID methods inherited from PrefixNLRI.

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
	prefixBytes := PrefixBytes(prefixBits)
	labelBytes := EncodeLabelStack(l.labels)

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
// Use LenWithContext() for ADD-PATH aware length calculation.
func (l *LabeledUnicast) Len() int {
	return 1 + len(l.labels)*3 + PrefixBytes(l.prefix.Bits())
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
	prefixBytes := PrefixBytes(prefixBits)

	// Total bits: 24 per label + prefix bits
	totalBits := len(l.labels)*24 + prefixBits

	pos := off

	// Length byte
	buf[pos] = byte(totalBits)
	pos++

	// Encode labels (zero-alloc)
	pos += WriteLabelStack(buf, pos, l.labels)

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
