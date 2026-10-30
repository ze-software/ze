// Design: docs/architecture/core-design.md — BGP policy final disposition.

package filterapi

import (
	"github.com/ze-software/ze/internal/component/policy"
)

// Policy is BGP's compiled, immutable policy snapshot: the protocol-neutral
// chain plus the BGP-only stage binding of each ref. Its zero value is an
// absent operator policy. Internal filters do not imply presence. The session
// decides its default (RFC 8212, AcceptsAtEnd).
//
// inProcess runs parallel to the chain's refs: entry i reports that ref i is
// owned by a registered ingress stage. It is nil when no ref is bound, so an
// unbound policy carries no second slice. Only BindInProcess proves a binding
// and only WithBoundFilters records one.
// Safe for concurrent use: callers MUST NOT mutate borrowed refs or bindings.
type Policy struct {
	chain           policy.Chain
	inProcess       []bool
	requiresIngress bool
}

// NewPolicy constructs a configured policy and refuses a missing or unknown final.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func NewPolicy(filters []policy.Ref, final policy.Final) (Policy, error) {
	chain, err := policy.NewChain(filters, final)
	if err != nil {
		return Policy{}, err
	}
	return Policy{chain: chain}, nil
}

// InternalPolicy carries automatic filters without configuring an operator policy.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func InternalPolicy(filters []policy.Ref) Policy {
	return (Policy{}).WithFilters(filters)
}

// CompilePolicy wraps a chain extracted from config with no stage binding.
func CompilePolicy(chain policy.Chain) Policy {
	return Policy{chain: chain}
}

// Filters returns borrowed references. The caller MUST NOT mutate them.
func (p Policy) Filters() []policy.Ref { return p.chain.Refs() }

// Bindings returns the borrowed stage bindings, parallel to Filters, or nil when
// no ref is bound. The caller MUST NOT mutate them.
func (p Policy) Bindings() []bool { return p.inProcess }

// Final returns the configured final, or zero for an absent operator policy.
func (p Policy) Final() policy.Final { return p.chain.Final() }

// Configured distinguishes an explicit empty policy from an absent policy.
func (p Policy) Configured() bool { return p.chain.Configured() }

// InProcess reports that ref i is owned by its registered ingress stage. Named
// internal refs are not supported on export, even when the owner has an egress
// hook. i MUST index Filters.
func (p Policy) InProcess(i int) bool {
	if p.inProcess == nil {
		return false
	}
	return p.inProcess[i]
}

// ActiveExternal reports whether the per-peer RPC chain must execute ref i.
// i MUST index Filters.
func (p Policy) ActiveExternal(i int) bool {
	return !p.chain.Refs()[i].Inactive && !p.InProcess(i)
}

// RequiresIngress reports whether an active reference belongs to a registered
// ingress stage. Consumers MUST refuse other directions before executing any
// external filter, since a terminal response can stop traversal before that ref.
func (p Policy) RequiresIngress() bool { return p.requiresIngress }

// WithFilters replaces references while preserving the policy's disposition.
// Every replacement ref is unbound: a caller that keeps stage ownership across
// the replacement uses WithBoundFilters.
// The caller MUST keep filters immutable for the lifetime of every derived policy.
func (p Policy) WithFilters(filters []policy.Ref) Policy {
	p.chain = p.chain.WithRefs(filters)
	p.inProcess = nil
	p.requiresIngress = false
	return p
}

// WithBoundFilters replaces references and their stage bindings together while
// preserving the policy's disposition. inProcess MUST be nil or as long as
// filters; a nil slice records no binding. The caller MUST keep both slices
// immutable for the lifetime of every derived policy.
func (p Policy) WithBoundFilters(filters []policy.Ref, inProcess []bool) Policy {
	p = p.WithFilters(filters)
	if inProcess == nil {
		return p
	}
	if len(inProcess) != len(filters) {
		panic("BUG: filterapi: stage bindings do not run parallel to the filter refs")
	}
	p.inProcess = inProcess
	for i, ref := range filters {
		if ref.Inactive {
			continue
		}
		if inProcess[i] {
			p.requiresIngress = true
			break
		}
	}
	return p
}

// Prepend puts ref, bound to its stage when inProcess is true, ahead of the
// policy's refs, keeping every existing binding and the disposition. The
// result owns new slices; the receiver's borrowed ones are never written.
func (p Policy) Prepend(ref policy.Ref, inProcess bool) Policy {
	refs := make([]policy.Ref, 0, len(p.Filters())+1)
	refs = append(refs, ref)
	refs = append(refs, p.Filters()...)
	if !inProcess {
		if p.inProcess == nil {
			return p.WithBoundFilters(refs, nil)
		}
	}
	bound := make([]bool, len(refs))
	bound[0] = inProcess
	copy(bound[1:], p.inProcess)
	return p.WithBoundFilters(refs, bound)
}

// AcceptsAtEnd applies the final disposition for a known session type.
// The caller MUST supply the external classification from the policy's snapshot.
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied."
// RFC 8212 Section 3: "Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied."
func (p Policy) AcceptsAtEnd(external bool) bool {
	if !p.Configured() {
		return !external
	}
	return p.chain.Final() == policy.FinalAccept
}
