// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// RFC: rfc/short/rfc8955.md -- flow specification NLRI length encoding (Section 4)
// Related: register.go -- binds SplitFlowSpec to the flowspec and flow-vpn families
// Related: nlrisplit.go -- Splitter type and registry this splitter plugs into

package nlrisplit

import "fmt"

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
func SplitFlowSpec(data []byte, addPath bool, fn func(nlri []byte)) (int, error) {
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

		// A zero-length flow specification matches nothing and would leave the
		// walk on the same octet forever, so it is malformed rather than empty.
		if length == 0 {
			return count, fmt.Errorf("nlrisplit: zero-length flowspec NLRI at offset %d", start)
		}

		nlriLen := head + lengthOctets + length
		if start+nlriLen > len(data) {
			return count, fmt.Errorf("nlrisplit: flowspec NLRI at offset %d extends past data (len=%d)", start, length)
		}

		if fn != nil {
			fn(data[start : start+nlriLen])
		}
		count++
		offset = start + nlriLen
	}

	return count, nil
}
