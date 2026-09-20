// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin
// RFC: rfc/short/rfc8277.md
//
// Package bgp_labeled implements Labeled Unicast NLRI (RFC 8277, SAFI 4).
package labeled

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Type aliases for shared nlri types.
type (
	Family = family.Family
	AFI    = family.AFI
	SAFI   = family.SAFI
	NLRI   = nlri.NLRI
)

// Re-export constants.
const (
	AFIIPv4       = family.AFIIPv4
	AFIIPv6       = family.AFIIPv6
	SAFIMPLSLabel = family.SAFIMPLSLabel
)

// Local family vars for this plugin (registered at init).
var (
	IPv4LabeledUnicast = family.MustRegister(AFIIPv4, SAFIMPLSLabel, "ipv4", "mpls-label")
	IPv6LabeledUnicast = family.MustRegister(AFIIPv6, SAFIMPLSLabel, "ipv6", "mpls-label")
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
	family  Family
	prefix  netip.Prefix
	pathID  uint32   // RFC 7911: the Path Identifier, which has no absent value
	hasPath bool     // RFC 7911: true when this NLRI carries a Path Identifier
	labels  []uint32 // RFC 8277 Section 2.1 stack ENTRIES: label + traffic class + S
}

// NewLabeledUnicast creates a new labeled unicast NLRI.
//
// RFC 8277: Labels are encoded per RFC 3032: 20-bit label + 3-bit TC + 1-bit S.
// The last label has S=1 (Bottom of Stack).
//
// RFC 7911 Section 3 gives the Path Identifier four octets and reserves no
// value, so zero is an identifier like any other and cannot say that none is
// carried. hasPath is that fact, and the caller states it: a route built with
// hasPath false has no Path Identifier whatever pathID holds.
//
// Use WriteNLRI() with addPath=true to encode the identifier.
// The family's SAFI is overridden to SAFIMPLSLabel (4) regardless of input.
func NewLabeledUnicast(fam Family, prefix netip.Prefix, labels []uint32, pathID uint32, hasPath bool) *LabeledUnicast {
	return &LabeledUnicast{
		family:  Family{AFI: fam.AFI, SAFI: SAFIMPLSLabel},
		prefix:  prefix,
		pathID:  pathID,
		hasPath: hasPath,
		labels:  nlri.LabelEntriesFor(labels),
	}
}

// Family returns the AFI/SAFI for this NLRI.
func (l *LabeledUnicast) Family() Family { return l.family }

// Prefix returns the IP prefix.
func (l *LabeledUnicast) Prefix() netip.Prefix { return l.prefix }

// PathID returns the ADD-PATH path identifier. Zero is an identifier like any
// other, so a caller that needs to know whether one is carried asks HasPathID
// rather than comparing this value against zero.
func (l *LabeledUnicast) PathID() uint32 { return l.pathID }

// HasPathID reports whether this NLRI carries an RFC 7911 Path Identifier.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
// The field has no reserved or absent value, so an identifier of zero is a real
// identifier and PathID alone cannot say whether one is there.
func (l *LabeledUnicast) HasPathID() bool { return l.hasPath }

// SupportsAddPath returns true - labeled unicast supports ADD-PATH per RFC 7911.
func (l *LabeledUnicast) SupportsAddPath() bool { return true }

// Labels returns the 20-bit MPLS labels of the stack.
func (l *LabeledUnicast) Labels() []uint32 {
	return nlri.LabelValues(l.labels)
}

// LabelEntries returns the stack as the wire carries it: one 3-octet entry per
// label, each holding the label, its traffic class and the bottom-of-stack bit.
func (l *LabeledUnicast) LabelEntries() []uint32 { return l.labels }

// Bytes returns the wire-format encoding (payload only, no path ID).
//
// RFC 8277 Section 2.2 - NLRI Encoding:
// [Length (1 byte)][Labels (3*N bytes)][Prefix (variable)]
//
// Note: Path ID is NOT included. Use WriteNLRI() for ADD-PATH encoding.
//
// Bytes allocates a standalone slice and delegates to WriteTo; hot-path
// senders should call WriteTo directly with a pool buffer.
func (l *LabeledUnicast) Bytes() []byte {
	buf := make([]byte, l.Len())
	l.WriteTo(buf, 0)
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
	var sb textbuf.Buffer
	sb.Str("prefix ").Str(l.prefix.String())
	if len(l.labels) > 0 {
		sb.Str(" label ")
		fmt.Fprintf(&sb, "%d", nlri.LabelValue(l.labels[0])) //nolint:errcheck // buffer output
		for _, entry := range l.labels[1:] {
			sb.Byte(',')
			fmt.Fprintf(&sb, "%d", nlri.LabelValue(entry)) //nolint:errcheck // buffer output
		}
	}
	if l.hasPath {
		sb.Str(" path-id ")
		fmt.Fprintf(&sb, "%d", l.pathID) //nolint:errcheck // buffer output
	}
	return sb.String()
}
