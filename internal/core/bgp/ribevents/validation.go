// Design: docs/architecture/plugin/rib-storage-design.md -- validation eligibility between receive stores
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 retained ineligible routes
package ribevents

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
)

// ValidationRoute identifies a received path without its attribute bytes.
type ValidationRoute struct {
	Peer   netip.Addr
	Family family.Family
	Prefix netip.Prefix
	PathID uint32
	// NLRI is the full native wire encoding for a non-CIDR route, without
	// ADD-PATH. FlowSpec uses FlowSpecKey's shortest length encoding.
	// A string keeps the key comparable and owns its bytes.
	NLRI string
}

// ValidationLookup reads the authoritative Adj-RIB-In eligibility. A zero msgID
// asks about the current route; a nonzero msgID MUST match its received UPDATE.
// Implementations MUST be safe for concurrent calls and MUST NOT emit events.
type ValidationLookup func(route ValidationRoute, msgID uint64) bool

type validationProvider struct {
	eligible ValidationLookup
	present  func(ValidationRoute) bool
}

var validationLookup atomic.Pointer[validationProvider]

// RegisterValidationLookup installs the receive gate before peer events start.
// The owner MUST unregister with nil after stopping event delivery.
func RegisterValidationLookup(fn ValidationLookup, present func(ValidationRoute) bool) {
	if fn == nil {
		validationLookup.Store(nil)
		return
	}
	validationLookup.Store(&validationProvider{eligible: fn, present: present})
}

// ValidationEnabled reports whether a receive validator has enabled its gate.
// The owner registers its lookup when enabling validation and unregisters at stop.
func ValidationEnabled() bool {
	return validationLookup.Load() != nil
}

// RouteEligible combines independent validators. FlowSpec fails closed until
// the selecting RIB has authorized the exact received generation.
func RouteEligible(route ValidationRoute, msgID uint64) bool {
	if IsFlowSpec(route.Family) {
		provider := flowSpecLookup.Load()
		if provider == nil || !provider.eligible(route, msgID) {
			return false
		}
	}
	fn := validationLookup.Load()
	if fn == nil {
		return true
	}
	return fn.eligible(route, msgID)
}

// RoutePresent reports whether the receive store still owns a pending or
// installed path, including an ineligible one. Retained paths must keep their
// regenerated ADD-PATH identity across temporary validation withdrawals.
// Without a receive gate there is no retained validation ownership.
func RoutePresent(route ValidationRoute) bool {
	if IsFlowSpec(route.Family) {
		provider := flowSpecLookup.Load()
		return provider != nil && provider.present != nil && provider.present(route)
	}
	provider := validationLookup.Load()
	return provider != nil && provider.present != nil && provider.present(route)
}

// EventValidationChange requests selection after the receive gate changes.
const EventValidationChange = "validation-change"

// ValidationChange names paths whose eligibility may have changed. Producers
// MUST emit after releasing the receive-store lock. Consumers MUST re-read the
// gate rather than trusting event order; independent plugin delivery can race.
var ValidationChange = events.Register[[]ValidationRoute](Namespace, EventValidationChange)
