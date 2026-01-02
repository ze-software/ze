package reactor

import (
	"sort"

	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// NegotiatedCapabilities tracks what was negotiated (not how to encode).
// Encoding parameters live in EncodingContext (recvCtx/sendCtx).
//
// This struct answers "what families are enabled?" while EncodingContext
// answers "how do we encode for this peer?".
type NegotiatedCapabilities struct {
	families        map[nlri.Family]bool // private for O(1) lookup
	ExtendedMessage bool                 // RFC 8654: Extended message support
}

// NewNegotiatedCapabilities creates from capability negotiation result.
func NewNegotiatedCapabilities(neg *capability.Negotiated) *NegotiatedCapabilities {
	if neg == nil {
		return nil
	}

	nc := &NegotiatedCapabilities{
		families:        make(map[nlri.Family]bool),
		ExtendedMessage: neg.ExtendedMessage,
	}

	for _, f := range neg.Families() {
		// f is capability.Family which is now nlri.Family (type alias)
		nc.families[f] = true
	}

	return nc
}

// Has returns whether the family was negotiated.
func (nc *NegotiatedCapabilities) Has(f nlri.Family) bool {
	if nc == nil || nc.families == nil {
		return false
	}
	return nc.families[f]
}

// Families returns all negotiated families in deterministic order.
// Used for EOR sending where order should be reproducible for testing.
// Orders by AFI first, then SAFI.
func (nc *NegotiatedCapabilities) Families() []nlri.Family {
	if nc == nil || nc.families == nil {
		return nil
	}

	result := make([]nlri.Family, 0, len(nc.families))
	for f := range nc.families {
		result = append(result, f)
	}

	sort.Slice(result, func(i, j int) bool {
		return nlri.FamilyLess(result[i], result[j])
	})

	return result
}
