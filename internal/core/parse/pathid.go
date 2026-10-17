// Design: docs/architecture/api/update-syntax.md — path-information modifier
// Related: origin.go — the other value parser config and the API share

package parse

import (
	"fmt"
	"net/netip"
	"strconv"
)

// PathID parses the value of a `path-information` keyword: the ADD-PATH Path
// Identifier.
//
// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending the
// Path Identifier field, which is of four octets."
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  octet 0 (a)  |  octet 1 (b)  |  octet 2 (c)  |  octet 3 (d)  |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// The four octets are written either as one decimal number, 0..4294967295, or
// in dotted form a.b.c.d, which names the octets in wire order: 0.0.0.1 is 1
// and 1.2.3.4 is 16909060. ExaBGP writes the dotted form, so the config and the
// `update text` command both read both forms through this one function.
// Anything else, including an IPv6 address, is refused by name.
func PathID(s string) (uint32, error) {
	if n, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint32(n), nil //nolint:gosec // G115: bounded by ParseUint bitSize 32
	}
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return 0, pathIDError(s)
	}
	if !ip.Is4() {
		return 0, pathIDError(s)
	}
	b := ip.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// pathIDError names the refused token and both forms PathID accepts.
func pathIDError(s string) error {
	return fmt.Errorf("invalid path-information %q: expected an integer 0..4294967295 "+
		"or a dotted a.b.c.d such as 0.0.0.1", s)
}
