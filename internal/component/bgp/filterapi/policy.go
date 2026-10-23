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
// policy, which accepts at exhaustion. Internal filters do not imply presence.
// Safe for concurrent use: callers MUST NOT mutate borrowed filter references.
type Policy struct {
	filters []FilterRef
	final   FinalDisposition
}

// NewPolicy constructs a configured policy and refuses a missing or unknown final.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func NewPolicy(filters []FilterRef, final FinalDisposition) (Policy, error) {
	switch final {
	case FinalAccept, FinalReject:
		return Policy{filters: filters, final: final}, nil
	default: // Open input: callers can convert arbitrary numeric values.
		return Policy{}, fmt.Errorf("invalid policy final disposition %d: set final accept or final reject", final)
	}
}

// InternalPolicy carries automatic filters without configuring an operator policy.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func InternalPolicy(filters []FilterRef) Policy {
	return Policy{filters: filters}
}

// Filters returns borrowed references. The caller MUST NOT mutate them.
func (p Policy) Filters() []FilterRef { return p.filters }

// Final returns the configured final, or zero for an absent operator policy.
func (p Policy) Final() FinalDisposition { return p.final }

// Configured distinguishes an explicit empty policy from an absent policy.
func (p Policy) Configured() bool { return p.final != 0 }

// WithFilters replaces references while preserving the policy's disposition.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func (p Policy) WithFilters(filters []FilterRef) Policy {
	p.filters = filters
	return p
}

// AcceptsAtEnd applies the final disposition, including absent/internal acceptance.
func (p Policy) AcceptsAtEnd() bool { return p.final != FinalReject }
