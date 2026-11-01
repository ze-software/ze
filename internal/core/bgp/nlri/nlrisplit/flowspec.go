// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// RFC: rfc/short/rfc8955.md -- flow specification NLRI length encoding (Section 4)
// Related: register.go -- binds SplitFlowSpec to the flowspec and flow-vpn families
// Related: nlrisplit.go -- Splitter type and registry this splitter plugs into

package nlrisplit

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
)

// flowSpecExtendedLength is the first value RFC 8955 Section 4 encodes in two
// octets. Below it the length is one octet; at or above it the length is two
// octets whose most significant nibble is 0xf, which caps the encoding at 4095.
const flowSpecExtendedLength = 0xF0

// SplitFlowSpec is the Splitter for the flow specification families, SAFI 133
// and SAFI 134 (RFC 8955 Section 4, RFC 8956 for IPv6). Each NLRI is framed as:
//
//	[length:1 or 2][components]
//
// The length octet is read, not assumed: "If the NLRI length is smaller than 240
// (0xf0 hex) octets, the length field can be encoded as a single octet.
// Otherwise, it is encoded as an extended-length 2-octet value in which the most
// significant nibble has the hex value 0xf." Reading a two-octet length as one
// octet frames every following NLRI in the section at the wrong offset, so the
// distinction decides the whole walk rather than one entry.
//
// For SAFI 134 an 8-octet Route Distinguisher precedes the components and is
// counted inside the length, so the same walk frames both families.
//
// Under ADD-PATH (RFC 7911) each NLRI is prefixed with a 4-byte path identifier
// that is included in the visited slice.
//
// Slices alias data. A malformed entry returns the count of the entries visited
// before it plus a non-nil error; the caller decides whether to use the partial
// result.
//
// The walk is bounded by len(data): a zero length is rejected below, so every
// entry advances the offset.
func SplitFlowSpec(data []byte, addPath bool, fn func(nlri []byte) bool) (int, error) {
	count := 0
	offset := 0
	for offset < len(data) {
		start := offset
		head := 0
		if addPath {
			head = 4
		}

		if start+head+1 > len(data) {
			return count, fmt.Errorf("nlrisplit: truncated flowspec length at offset %d", start)
		}

		lengthOctets := 1
		length := int(data[start+head])
		if length >= flowSpecExtendedLength {
			if start+head+2 > len(data) {
				return count, fmt.Errorf("nlrisplit: truncated flowspec extended length at offset %d", start)
			}
			lengthOctets = 2
			length = int(data[start+head]&0x0F)<<8 | int(data[start+head+1])
		}

		// The length octets alone advance the walk, so a zero length cannot
		// stall it. It is refused because the value it announces holds no
		// component, and RFC 8955 requires at least one.
		// RFC 8955 Section 4.2: "Encoding: <[component]+>"
		// RFC 8955 Section 4.2: "An NLRI value not encoded as specified here,
		// including an NLRI that contains an unknown component type, is
		// considered malformed and error handling according to Section 10 is
		// performed."
		if length == 0 {
			return count, fmt.Errorf("nlrisplit: zero-length flowspec NLRI at offset %d", start)
		}

		nlriLen := head + lengthOctets + length
		if start+nlriLen > len(data) {
			return count, fmt.Errorf("nlrisplit: flowspec NLRI at offset %d extends past data (len=%d)", start, length)
		}

		count++
		if fn != nil {
			if !fn(data[start : start+nlriLen]) {
				return count, nil
			}
		}
		offset = start + nlriLen
	}

	return count, nil
}

// IsFlowSpec reports whether fam is IPv4 or IPv6 FlowSpec or FlowSpec VPN
// (SAFI 133 or 134).
func IsFlowSpec(fam family.Family) bool {
	return (fam.AFI == family.AFIIPv4 || fam.AFI == family.AFIIPv6) &&
		(fam.SAFI == family.SAFIFlowSpec || fam.SAFI == family.SAFIFlowSpecVPN)
}

// FlowSpecDestination reads only the first, destination-prefix component of
// one FlowSpec NLRI, the projection a prefix policy judges. An absent or
// nonzero-offset destination has no CIDR equivalent: it answers an invalid
// prefix, which a caller denies.
//
// RFC 8955 Section 4.2.2.1: "Defines the destination prefix to match.  The length and prefix
// fields are encoded as in BGP UPDATE messages [RFC4271]."
// Receive validation already checks component ordering and the remaining rule;
// filter text is a projection, not a second FlowSpec parser or a unicast rewrite.
// one MUST be a complete native NLRI returned by SplitFlowSpec.
func FlowSpecDestination(fam family.Family, one []byte) netip.Prefix {
	off := 1
	if one[0] >= 240 {
		off = 2
	}
	if fam.SAFI == family.SAFIFlowSpecVPN {
		off += 8
	}
	if off+2 > len(one) {
		return netip.Prefix{}
	}
	if one[off] != 1 {
		return netip.Prefix{}
	}
	bits := int(one[off+1])
	off += 2
	width := 4
	if fam.AFI == family.AFIIPv6 {
		width = 16
		if off == len(one) {
			return netip.Prefix{}
		}
		if one[off] != 0 {
			return netip.Prefix{}
		}
		off++
	}
	if bits > width*8 {
		return netip.Prefix{}
	}
	n := (bits + 7) / 8
	if off+n > len(one) {
		return netip.Prefix{}
	}
	var addr [16]byte
	copy(addr[:], one[off:off+n])
	ip := netip.AddrFrom16(addr)
	if width == 4 {
		ip = netip.AddrFrom4([4]byte(addr[:4]))
	}
	return netip.PrefixFrom(ip, bits).Masked()
}
