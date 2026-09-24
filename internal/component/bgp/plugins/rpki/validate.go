// Design: docs/architecture/plugin/rib-storage-design.md — RFC 6811 origin validation
// RFC: rfc/short/rfc6811.md -- Section 2: the validation state of a route comes from the lookup
// of its prefix and origin AS in the VRP set
// Overview: rpki.go — plugin using validation for route decisions
// Related: roa_cache.go — ROA cache providing VRP lookups
package rpki

import "net"

// Validation state constants (must match adj_rib_in values).
const (
	ValidationNotValidated uint8 = 0
	ValidationValid        uint8 = 1
	ValidationNotFound     uint8 = 2
	ValidationInvalid      uint8 = 3
)

// OriginNone is a sentinel for AS_SET or an unreadable origin.
const OriginNone uint32 = 0xFFFFFFFF

// Validate performs RFC 6811 origin validation for a prefix and origin AS.
// Returns ValidationValid, ValidationInvalid, or ValidationNotFound.
//
// A prefix ze cannot parse gets ValidationInvalid and a warning, never
// ValidationNotFound: NotFound states that the VRP set was consulted and covers
// nothing, and the default not-found action accepts the route on that reading.
// An unreadable prefix was never validated, so it fails closed instead.
func (c *ROACache) Validate(prefix string, originAS uint32) uint8 {
	// Parse first. The parse decides whether the query is answerable at all, so
	// it must run before the cache lookup rather than after it.
	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		logger().Warn("roa: prefix is not a CIDR, refusing to validate", "prefix", prefix)
		return ValidationInvalid
	}

	covering := c.findCovering(ipnet)
	if len(covering) == 0 {
		return ValidationNotFound
	}

	prefixLen, _ := ipnet.Mask.Size()

	// If origin is NONE (AS_SET), it can never match any VRP.
	if originAS == OriginNone {
		return ValidationInvalid
	}

	// RFC 6811: Valid if ANY covering VRP matches (ASN + maxLength).
	for _, entry := range covering {
		if uint8(prefixLen) <= entry.MaxLength && entry.ASN == originAS && entry.ASN != 0 {
			return ValidationValid
		}
	}

	// Covering VRPs exist but none matched.
	return ValidationInvalid
}
