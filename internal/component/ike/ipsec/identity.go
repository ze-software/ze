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

// sortStrings is an insertion sort over the handful of type names. It avoids pulling the
// sort package into a package whose only need is a stable error message.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
