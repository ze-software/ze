// Design: docs/architecture/core-design.md — negotiated capability tracking
// Related: session_negotiate.go — capability negotiation produces NegotiatedCapabilities

package reactor

import (
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// NegotiatedCapabilities tracks what was negotiated (not how to encode).
// Encoding parameters live in EncodingContext (recvCtx/sendCtx).
//
// This struct answers "what families are enabled?" while EncodingContext
// answers "how do we encode for this peer?".
type NegotiatedCapabilities struct {
	families             map[family.Family]bool // private for O(1) lookup
	ExtendedMessage      bool                   // RFC 8654: Extended message support
	RouteRefresh         bool                   // RFC 2918: Route refresh
	EnhancedRouteRefresh bool                   // RFC 7313: Enhanced route refresh
	ASN4                 bool                   // RFC 6793: 4-byte ASN support
}

// NewNegotiatedCapabilities creates from capability negotiation result.
func NewNegotiatedCapabilities(neg *capability.Negotiated) *NegotiatedCapabilities {
	if neg == nil {
		return nil
	}

	nc := &NegotiatedCapabilities{
		families:             make(map[family.Family]bool),
		ExtendedMessage:      neg.ExtendedMessage,
		RouteRefresh:         neg.RouteRefresh,
		EnhancedRouteRefresh: neg.EnhancedRouteRefresh,
		ASN4:                 neg.ASN4,
	}

	for _, f := range neg.Families() {
		// f is capability.Family which is now family.Family (type alias)
		nc.families[f] = true
	}

	return nc
}

// Has returns whether the family was negotiated.
func (nc *NegotiatedCapabilities) Has(f family.Family) bool {
	if nc == nil || nc.families == nil {
		return false
	}
	return nc.families[f]
}

// Families returns all negotiated families in deterministic order.
// Used for EOR sending where order should be reproducible for testing.
// Orders by AFI first, then SAFI.
func (nc *NegotiatedCapabilities) Families() []family.Family {
	if nc == nil || nc.families == nil {
		return nil
	}

	result := make([]family.Family, 0, len(nc.families))
	for f := range nc.families {
		result = append(result, f)
	}

	sort.Slice(result, func(i, j int) bool {
		return family.FamilyLess(result[i], result[j])
	})

	return result
}
