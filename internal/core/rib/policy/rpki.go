// Design: docs/architecture/rib/unified-locrib.md -- RPKI origin validation seam for route policy
// RFC: rfc/short/rfc8893.md -- Sections 1 and 3: uniform origin validation across ingress, redistribution, and egress
// Related: policy.go -- Route.ValidationState

package policy

import (
	"net/netip"
	"sync/atomic"
)

// ValidationState is the RFC 6811 Section 2 RPKI Route Origin Validation state
// of a route.
type ValidationState uint8

const (
	// ValidationNotValidated means no validation state is available (no active
	// RPKI validator or the route carries no origin AS).
	ValidationNotValidated ValidationState = 0
	// ValidationValid means at least one covering VRP authorizes the prefix
	// length and origin AS (RFC 6811 Section 2).
	ValidationValid ValidationState = 1
	// ValidationNotFound means no covering VRP exists for the prefix (RFC 6811
	// Section 2).
	ValidationNotFound ValidationState = 2
	// ValidationInvalid means at least one covering VRP exists, and none
	// authorizes the prefix length and origin AS (RFC 6811 Section 2).
	ValidationInvalid ValidationState = 3
)

// OriginNone is the RFC 6811 Section 2 distinguished value "NONE" for an
// AS_PATH whose final segment is an AS_SET or another non-AS_SEQUENCE /
// non-confederation segment type.
const OriginNone uint32 = 0xFFFFFFFF

// OriginValidator evaluates the RFC 6811 Section 2 origin validation state for
// one (prefix, originAS) pair against the active ROA/VRP cache. Implementations
// MUST be safe for concurrent use and MUST NOT block.
type OriginValidator func(prefix netip.Prefix, originAS uint32) ValidationState

var (
	originValidator atomic.Pointer[OriginValidator]
	localOriginAS   atomic.Uint32
)

// SetLocalOriginAS publishes the speaker's global BGP local AS so redistribution
// policy can validate locally originated non-BGP routes (static, connected,
// OSPF, IS-IS) that carry no BGP AS_PATH (RFC 8893 Section 3). Passing 0 clears
// it so routes without an explicit origin AS evaluate as ValidationNotValidated.
func SetLocalOriginAS(asn uint32) {
	localOriginAS.Store(asn)
}

// LocalOriginAS returns the published global BGP local AS, or 0 when unset.
func LocalOriginAS() uint32 {
	return localOriginAS.Load()
}

// SetOriginValidator publishes v as the process-wide RPKI origin validator read
// by ingress, redistribution, and egress policy evaluation (RFC 8893 Section 3).
// Passing nil clears the validator so ValidateOrigin answers ValidationNotValidated.
func SetOriginValidator(v OriginValidator) {
	if v == nil {
		originValidator.Store(nil)
		return
	}
	fn := v
	originValidator.Store(&fn)
}

// ValidateOrigin evaluates RFC 6811 Section 2 origin validation for (prefix,
// originAS) through the active origin validator, returning
// ValidationNotValidated when no validator is active or prefix is invalid.
//
// RFC 8893 Section 1: "A validating BGP speaker MUST apply Route Origin
// Validation policy semantics (see Section 2 of [RFC6811] and Section 4 of
// [RFC8481]) after applying any egress configuration and policy."
func ValidateOrigin(prefix netip.Prefix, originAS uint32) ValidationState {
	if !prefix.IsValid() {
		return ValidationNotValidated
	}
	v := originValidator.Load()
	if v == nil {
		return ValidationNotValidated
	}
	return (*v)(prefix, originAS)
}

// ValidationState returns the route's RPKI origin validation state (RFC 6811
// Section 2, RFC 8893 Section 3), or ValidationNotValidated when the route
// carries no origin AS or no validator is active.
func (r Route) ValidationState() ValidationState {
	if !r.HasOriginAS {
		return ValidationNotValidated
	}
	return ValidateOrigin(r.Prefix, r.OriginAS)
}
