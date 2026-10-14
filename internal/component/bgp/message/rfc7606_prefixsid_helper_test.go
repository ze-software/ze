// Design: docs/architecture/wire/messages.md -- RFC 7606 UPDATE validation
// Related: rfc7606.go -- validatePrefixSIDValue, the receive-side Prefix-SID verdict

package message

import "github.com/ze-software/ze/internal/core/bgp/attribute"

// validatePrefixSIDAttr judges a Prefix-SID value as the receive walk does for an
// UPDATE with no MP_REACH_NLRI, whose routes are IPv4 unicast
// (attribute.PrefixSIDRouteFamily). It keeps the per-attribute signature the
// RFC 7606 unit tests were written against; the walk itself calls
// validatePrefixSIDForRoutes after it has read the family.
func validatePrefixSIDAttr(_ uint8, _ int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	return validatePrefixSIDValue(attrData, attribute.AFIIPv4, attribute.SAFIUnicast)
}
