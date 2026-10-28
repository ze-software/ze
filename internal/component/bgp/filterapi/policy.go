// Design: docs/architecture/core-design.md — BGP policy final disposition.

package filterapi

import "fmt"

// FinalDisposition is the operator's decision after every filter passes.
// Zero denotes absence, never a configured policy.
type FinalDisposition uint8

const (
	FinalAccept FinalDisposition = iota + 1
	FinalReject
)

// Policy is an immutable policy snapshot. Its zero value is an absent operator
// policy. Internal filters do not imply presence. The session decides its default.
// Safe for concurrent use: callers MUST NOT mutate borrowed filter references.
type Policy struct {
	filters         []FilterRef
	final           FinalDisposition
	requiresIngress bool
}

// NewPolicy constructs a configured policy and refuses a missing or unknown final.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func NewPolicy(filters []FilterRef, final FinalDisposition) (Policy, error) {
	switch final {
	case FinalAccept, FinalReject:
		return (Policy{final: final}).WithFilters(filters), nil
	default: // Open input: callers can convert arbitrary numeric values.
		return Policy{}, fmt.Errorf("invalid policy final disposition %d: set final accept or final reject", final)
	}
}

// InternalPolicy carries automatic filters without configuring an operator policy.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func InternalPolicy(filters []FilterRef) Policy {
	return (Policy{}).WithFilters(filters)
}

// Filters returns borrowed references. The caller MUST NOT mutate them.
func (p Policy) Filters() []FilterRef { return p.filters }

// Final returns the configured final, or zero for an absent operator policy.
func (p Policy) Final() FinalDisposition { return p.final }

// Configured distinguishes an explicit empty policy from an absent policy.
func (p Policy) Configured() bool { return p.final != 0 }

// RequiresIngress reports whether an active reference belongs to a registered
// ingress stage. Consumers MUST refuse other directions before executing any
// external filter, since a terminal response can stop traversal before that ref.
func (p Policy) RequiresIngress() bool { return p.requiresIngress }

// WithFilters replaces references while preserving the policy's disposition.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func (p Policy) WithFilters(filters []FilterRef) Policy {
	p.filters = filters
	p.requiresIngress = false
	for _, ref := range filters {
		if ref.Inactive {
			continue
		}
		if ref.InProcess() {
			p.requiresIngress = true
			break
		}
	}
	return p
}

// AcceptsAtEnd applies the final disposition for a known session type.
// The caller MUST supply the external classification from the policy's snapshot.
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied."
// RFC 8212 Section 3: "Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied."
func (p Policy) AcceptsAtEnd(external bool) bool {
	if !p.Configured() {
		return !external
	}
	return p.final == FinalAccept
}
