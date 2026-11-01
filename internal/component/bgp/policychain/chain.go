// Design: plan/post-release/engineering/spec-filter-wire-0-umbrella.md -- D-5, D-7, D-9, the in-process chain over the BGP view
// Detail: sets.go -- the set fields a member answers
// Related: ../policyview/view.go -- the lazy view a member reads
//
// A BGP chain at `bgp import` or `bgp export` compiles its library members
// (filter types whose evaluator is registered here) into a Chain checked by
// the route-policy library (policy.CheckChain) and run in process over the
// lazy BGP view. A member answers pass or reject and the set fields it
// changes; the run folds the sets in chain order, applies a member's on-error
// when its read fails (a malformed attribute, AC-28) or when it sets a field
// it did not declare (D-9), and decides the rest with the chain's final.
//
// Plugin filter types are not members of this chain: they still run over the
// filter-update path until the typed bridge slot and the batched protocol
// replace it (spec phase 3b).

package policychain

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var chainLogger = slogutil.LazyLogger("bgp.policy.chain")

// Decision is one member's verdict on one route.
type Decision uint8

const (
	DecisionUnspecified Decision = iota
	// DecisionPass passes the route to the next member with the member's sets.
	DecisionPass
	// DecisionReject stops the chain: the route is rejected.
	DecisionReject
)

// Evaluator judges one route over the view, writing the fields it changes
// into sets. It MUST NOT keep view, a Value it read, or sets past its return,
// MUST NOT block, and MUST be safe for concurrent use: several goroutines run
// one compiled chain, each with its own view. An error fails the route under
// the member's on-error, and a read wrapping policyview.ErrMalformed is the
// usual one.
type Evaluator func(view *policyview.View, sets *Sets) (Decision, error)

// EvaluatorCompiler compiles one filter instance's config subtree into its
// Evaluator. It MUST NOT change any state: a refused verify runs it.
type EvaluatorCompiler func(name string, instance map[string]any) (Evaluator, error)

type evaluatorRegistration struct {
	compile EvaluatorCompiler
	onError rpc.OnErrorPolicy
}

var evaluators = struct {
	mu    sync.RWMutex
	types map[string]evaluatorRegistration
}{types: map[string]evaluatorRegistration{}}

// MustRegisterEvaluator registers the in-process BGP evaluator of a filter
// type, with the on-error policy its members take when a run fails. filterType
// MUST already declare its fields (policy.MustRegisterFilterType). A second
// registration, or an unspecified on-error, is a Ze defect and panics.
func MustRegisterEvaluator(filterType string, onError rpc.OnErrorPolicy, compile EvaluatorCompiler) {
	if compile == nil {
		panic("BUG: policychain: nil evaluator compiler for " + filterType)
	}
	if onError != rpc.OnErrorReject && onError != rpc.OnErrorAccept {
		panic("BUG: policychain: evaluator for " + filterType + " registered without an on-error policy")
	}
	evaluators.mu.Lock()
	defer evaluators.mu.Unlock()
	if _, dup := evaluators.types[filterType]; dup {
		panic("BUG: policychain: second evaluator for " + filterType)
	}
	evaluators.types[filterType] = evaluatorRegistration{compile: compile, onError: onError}
}

// HasEvaluator reports whether filterType runs in process over the BGP view.
func HasEvaluator(filterType string) bool {
	evaluators.mu.RLock()
	defer evaluators.mu.RUnlock()
	_, ok := evaluators.types[filterType]
	return ok
}

func evaluatorTypes() string {
	evaluators.mu.RLock()
	defer evaluators.mu.RUnlock()
	if len(evaluators.types) == 0 {
		return "none is registered"
	}
	return strings.Join(slices.Sorted(maps.Keys(evaluators.types)), ", ")
}

// member is one compiled chain member.
type member struct {
	name       string
	filterType string
	evaluate   Evaluator
	onError    rpc.OnErrorPolicy
	// settable holds the set kinds the member declared (policy.InstanceSets),
	// which policy.CheckChain already bounded by what the attachment allows.
	settable setMask
}

// Chain is one compiled in-process BGP chain. Immutable after Compile, so safe
// for concurrent use; each run brings its own view and RunSets.
type Chain struct {
	attachment string
	members    []member
	final      policy.Final
	// refused counts the routes a member failed under its on-error.
	refused atomic.Uint64
}

// Compile checks the chain names at attachment through the route-policy
// library and compiles each member with its type's registered evaluator.
// A name the registry does not hold, a type with no evaluator, a field the
// attachment does not carry or allow, and a missing final are each refused by
// name.
func Compile(attachment string, reg *policy.Registry, names []string, final policy.Final) (*Chain, error) {
	if final != policy.FinalAccept && final != policy.FinalReject {
		return nil, fmt.Errorf("%s: missing or invalid final; write filters [ NAME ]; final accept; or final reject", attachment)
	}
	if err := policy.CheckChain(attachment, reg, names); err != nil {
		return nil, err
	}
	chain := &Chain{attachment: attachment, members: make([]member, 0, len(names)), final: final}
	for _, name := range names {
		compiled, err := compileMember(attachment, reg, name)
		if err != nil {
			return nil, err
		}
		chain.members = append(chain.members, compiled)
	}
	return chain, nil
}

func compileMember(attachment string, reg *policy.Registry, name string) (member, error) {
	inst, ok := reg.Lookup(name)
	if !ok {
		return member{}, fmt.Errorf("%s: %q is not a filter defined under the %s root; define it, or name an existing one",
			attachment, name, policy.RootName)
	}
	evaluators.mu.RLock()
	registration, ok := evaluators.types[inst.Type]
	evaluators.mu.RUnlock()
	if !ok {
		return member{}, fmt.Errorf("%s filter %q (%s): the type has no in-process BGP evaluator (types that have one: %s)",
			attachment, name, inst.Type, evaluatorTypes())
	}
	sets, err := policy.InstanceSets(inst)
	if err != nil {
		return member{}, fmt.Errorf("%s: %w", attachment, err)
	}
	evaluate, err := registration.compile(name, inst.Config)
	if err != nil {
		return member{}, fmt.Errorf("%s filter %q (%s): %w", attachment, name, inst.Type, err)
	}
	return member{name: name, filterType: inst.Type, evaluate: evaluate, onError: registration.onError, settable: maskOf(sets)}, nil
}

// OutcomeAction is the chain's verdict on one route.
type OutcomeAction uint8

const (
	OutcomeUnspecified OutcomeAction = iota
	OutcomeAccept
	OutcomeReject
)

// Outcome is one run's answer. Failed marks a reject that no filter decided:
// a member failed under a fail-closed on-error, so the caller drops the route
// rather than treating it as a policy reject (withdrawals stay under a final
// reject only when Failed is false). Member names the member that rejected or
// failed, empty when the final decided.
type Outcome struct {
	Action OutcomeAction
	Failed bool
	Member string
}

// RunSets is the per-run scratch a caller owns beside its view: one member's
// sets and the chain's folded sets. One per goroutine, reused across runs.
type RunSets struct {
	member Sets
	folded Sets
}

// Folded answers the sets the chain asked, valid until the next Run.
func (r *RunSets) Folded() *Sets { return &r.folded }

// Run judges the route view holds. view MUST have been Reset for this route;
// the caller Clears it after it has applied the folded sets. Run allocates
// nothing on the pass and reject paths.
func (c *Chain) Run(view *policyview.View, sets *RunSets) Outcome {
	sets.folded.Reset()
	for i := range c.members {
		m := &c.members[i]
		sets.member.Reset()
		decision, err := m.evaluate(view, &sets.member)
		if err == nil {
			err = m.refuseUndeclared(&sets.member)
		}
		if err == nil {
			err = decisionError(decision)
		}
		if err != nil {
			if c.fail(m, err) {
				return Outcome{Action: OutcomeReject, Failed: true, Member: m.name}
			}
			continue // fail-open: the member passes, and none of its sets apply
		}
		if decision == DecisionReject {
			return Outcome{Action: OutcomeReject, Member: m.name}
		}
		if err := sets.folded.merge(&sets.member); err != nil {
			if c.fail(m, err) {
				return Outcome{Action: OutcomeReject, Failed: true, Member: m.name}
			}
		}
	}
	switch c.final {
	case policy.FinalAccept:
		return Outcome{Action: OutcomeAccept}
	case policy.FinalReject:
		sets.folded.Reset()
		return Outcome{Action: OutcomeReject}
	}
	// Compile accepts only a verified final; anything else is a Ze defect,
	// and the run fails closed on it.
	return Outcome{Action: OutcomeReject, Failed: true}
}

// Refused answers how many routes a member of this chain failed.
func (c *Chain) Refused() uint64 { return c.refused.Load() }

// errUndeclaredSet is wrapped when a member writes a field it did not declare.
var errUndeclaredSet = errors.New("sets a field the filter did not declare")

// refuseUndeclared fails a member whose writes leave its declared sets (D-9):
// none of that route's sets apply.
func (m *member) refuseUndeclared(sets *Sets) error {
	extra := sets.written &^ m.settable
	if extra == 0 {
		return nil
	}
	for kind := setKindLocalPreference; kind < setKindCount; kind++ {
		if extra&kind.bit() != 0 {
			return fmt.Errorf("%w: %s", errUndeclaredSet, setKindFields[kind])
		}
	}
	return errUndeclaredSet
}

func decisionError(decision Decision) error {
	switch decision {
	case DecisionPass, DecisionReject:
		return nil
	case DecisionUnspecified:
	}
	return fmt.Errorf("BUG: evaluator answered decision %d, neither pass nor reject", decision)
}

// fail logs and counts a member failure and answers whether the route is
// rejected (fail-closed) rather than passed on (fail-open).
func (c *Chain) fail(m *member, err error) bool {
	c.refused.Add(1)
	chainLogger().Warn("policy filter failed, its on-error decides the route",
		"attachment", c.attachment, "filter", m.name, "type", m.filterType, "on-error", m.onError.String(), "error", err)
	return m.onError != rpc.OnErrorAccept
}
