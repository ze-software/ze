// Design: plan/learned/734-ipsec-3-data-model.md -- IPsec data model types
// RFC: rfc/short/rfc7296.md -- Identification payloads (Section 3.5), conformance set (Section 4)
// Related: types.go -- AuthConfig, which carries the parsed RemoteIDType
// Related: validate.go -- the commit-time identity checks that read these names

package ipsec

import (
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// remoteIDTypeNames maps the remote-id-type YANG enum onto the Identification Type
// numbers of RFC 7296 Section 3.5. The numbers come from the wire package rather than
// being restated here, so the config surface and the codec cannot drift apart
// (ai/rules/derive-not-hardcode.md).
//
// ID_DER_ASN1_GN is deliberately absent. RFC 7296 Section 3.5 assigns it, and RFC 7296
// Section 4 does not require accepting it, so ze offers no way to ask for a type it
// cannot compare.
var remoteIDTypeNames = map[string]uint8{
	"ipv4-address":   wire.IDTypeIPv4Addr,
	"ipv6-address":   wire.IDTypeIPv6Addr,
	"fqdn":           wire.IDTypeFQDN,
	"rfc822-address": wire.IDTypeRFC822Addr,
	"key-id":         wire.IDTypeKeyID,
	"der-asn1-dn":    wire.IDTypeDERASN1DN,
}

// RemoteIDTypeNames lists the accepted remote-id-type values, sorted, for an error
// message. Deriving the list from the map keeps a refusal honest when a type is added.
func RemoteIDTypeNames() []string {
	names := make([]string, 0, len(remoteIDTypeNames))
	for name := range remoteIDTypeNames {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

// ParseRemoteIDType maps a remote-id-type YANG enum value onto its RFC 7296 Section 3.5
// type number. The second result reports whether the name is known. The caller MUST
// REFUSE an unknown name rather than treat it as unset. A name read as unset would
// silently widen the peer's accepted identity types to every comparable one
// (ai/rules/fail-closed-guards.md).
func ParseRemoteIDType(s string) (uint8, bool) {
	t, ok := remoteIDTypeNames[s]
	return t, ok
}

// IDTerminator reports the first terminator octet in an IKE identity string, and
// whether the string holds one at all.
//
// RFC 7296 Section 3.5 MUST NOT, for ID_FQDN and repeated for ID_RFC822_ADDR: "The
// string MUST NOT contain any terminators (e.g., NULL, CR, etc.)."
//
// The section names two examples and closes with "etc.", so the set is read as every C0
// control octet (0x00 to 0x1F) plus DEL (0x7F). That covers NULL and CR, and it costs a
// legitimate value nothing: a domain name is letters, digits, hyphen and dot, and a mail
// address adds no control character either. Reading "etc." narrowly, as NULL and CR
// alone, would let LF through, and LF terminates a string in as many parsers as CR does
// (ai/rules/fail-closed-guards.md).
//
// The octets are examined one at a time rather than as runes. A terminator inside a
// multi-octet UTF-8 sequence is not reachable, because every continuation octet has its
// high bit set, so a byte walk cannot report a false position.
func IDTerminator(value string) (byte, bool) {
	for i := range len(value) {
		if c := value[i]; c < 0x20 || c == 0x7f {
			return c, true
		}
	}
	return 0, false
}

// sortStrings is an insertion sort over the handful of type names. It avoids pulling the
// sort package into a package whose only need is a stable error message.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
