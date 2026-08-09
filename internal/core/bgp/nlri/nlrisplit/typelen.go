// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
// RFC: rfc/short/rfc7606.md -- Section 5.4, the typed families this framing serves
// Related: evpn.go, mvpn.go, mup.go -- the three families that share this walk

package nlrisplit

import "fmt"

// splitTypeLength carves an NLRI section whose every entry is framed as a fixed
// header holding a route type, one length octet inside that header, and a value
// of exactly that length.
//
// Three families use this shape, differing only in where the length octet sits:
//
//	EVPN  [route-type:1][length:1][value]                  hdrLen 2, lenOff 1
//	MVPN  [route-type:1][length:1][value]                  hdrLen 2, lenOff 1
//	MUP   [arch:1][route-type:2][length:1][value]          hdrLen 4, lenOff 3
//
// Under ADD-PATH (RFC 7911) each entry is prefixed with a 4-byte path identifier
// that is included in the returned slice, so a recognizer reading the route type
// skips it the same way for every family.
//
// The walk is route-type-agnostic: it uses only the length octet to find the
// boundaries. Deciding whether a type is one ze implements is RFC 7606 Section
// 5.4's business and lives in the owning plugin (nlritype.Recognizer).
//
// Slices alias data. A malformed entry returns the partially-parsed result plus a
// non-nil error; the caller decides whether to use it. That error is the RFC 7606
// Section 5.3 condition "the length of the last NLRI found exceeds the amount of
// unconsumed data remaining in the attribute", which Section 3(j) routes to
// session reset because such an NLRI field cannot be parsed at all.
func splitTypeLength(data []byte, addPath bool, hdrLen, lenOff int, label string) ([][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var result [][]byte
	offset := 0
	for offset < len(data) {
		start := offset
		head := 0
		if addPath {
			head = 4
		}
		// Need at least the path identifier (if any) plus the whole fixed header.
		if start+head+hdrLen > len(data) {
			return result, fmt.Errorf("nlrisplit: truncated %s header at offset %d", label, start)
		}
		length := int(data[start+head+lenOff])
		nlriLen := head + hdrLen + length
		if start+nlriLen > len(data) {
			return result, fmt.Errorf("nlrisplit: %s NLRI at offset %d extends past data (len=%d)", label, start, length)
		}
		result = append(result, data[start:start+nlriLen])
		offset = start + nlriLen
	}
	return result, nil
}
