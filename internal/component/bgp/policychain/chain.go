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
// A member may also be a filter a plugin runs (external.go): it is sent the
// fields it declared over filter-update, read through the same view, and its
// sets fold beside the in-process members' sets.

package policychain

import (
	"encoding/binary"
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
	// For an external member it holds what the attachment allows, and its
	// reply's sets are checked against its declaration as well.
	settable setMask
	// external is set for a filter a plugin runs; evaluate is nil then.
	external *externalMember
}

// Chain is one compiled in-process BGP chain. Immutable after Compile, so safe
// for concurrent use; each run brings its own view and RunSets.
type Chain struct {
	attachment string
	members    []member
	final      policy.Final
	// bases and the two readers serve Apply's increment and decrement.
	bases                Bases
	localPreference, med policyview.Reader
	// community serves the community overlay (overlayCommunity).
	community policyview.Reader
	// prefix serves the prefix overlay (overlayPrefix).
	prefix policyview.Reader
	// refused counts the routes a member failed under its on-error.
	refused atomic.Uint64
}

// Compile checks the chain's members at attachment through the route-policy
// library and compiles each instance member with its type's registered
// evaluator. An external member (Member.Plugin set) was judged against its
// plugin's declaration when the plugin declared (Stage 1), so Compile
// records it with what the attachment lets a filter set.
// A name the registry does not hold, a type with no evaluator, a field the
// attachment does not carry or allow, and a missing final are each refused by
// name. bases is what an increment or decrement of an absent attribute starts
// from (bgp defaults attribute).
func Compile(attachment string, reg *policy.Registry, members []Member, final policy.Final, bases Bases) (*Chain, error) {
	if final != policy.FinalAccept && final != policy.FinalReject {
		return nil, fmt.Errorf("%s: missing or invalid final; write filters [ NAME ]; final accept; or final reject", attachment)
	}
	names := make([]string, 0, len(members))
	for _, m := range members {
		if m.Plugin == "" {
			names = append(names, m.Name)
		}
	}
	if err := policy.CheckChain(attachment, reg, names); err != nil {
		return nil, err
	}
	chain := &Chain{attachment: attachment, members: make([]member, 0, len(members)), final: final, bases: bases}
	var err error
	if chain.localPreference, err = policyview.ReaderFor(policy.FieldLocalPreference); err != nil {
		return nil, fmt.Errorf("BUG: policychain: %w", err)
	}
	if chain.med, err = policyview.ReaderFor(policy.FieldMED); err != nil {
		return nil, fmt.Errorf("BUG: policychain: %w", err)
	}
	if chain.prefix, err = policyview.ReaderFor(policy.FieldPrefix); err != nil {
		return nil, fmt.Errorf("BUG: policychain: %w", err)
	}
	if chain.community, err = policyview.ReaderFor(policy.FieldCommunity); err != nil {
		return nil, fmt.Errorf("BUG: policychain: %w", err)
	}
	for _, m := range members {
		if m.Plugin != "" {
			reference := m.Plugin + ":" + m.Name
			chain.members = append(chain.members, member{
				name: reference, filterType: m.Plugin, settable: externalSettable(attachment, reference),
				external: &externalMember{plugin: m.Plugin, filter: m.Name},
			})
			continue
		}
		compiled, err := compileMember(attachment, reg, m.Name)
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
//
// Teardown is set when an external member asked the session be closed on an
// import route (NotifyCode and NotifySubcode as it answered them); the route
// is rejected. It is never set on export.
type Outcome struct {
	Action                    OutcomeAction
	Failed                    bool
	Member                    string
	Teardown                  bool
	NotifyCode, NotifySubcode uint8
}

// RunSets is the per-run scratch a caller owns beside its view: one member's
// sets and the chain's folded sets. One per goroutine, reused across runs.
type RunSets struct {
	member Sets
	folded Sets
	// communityWire holds the COMMUNITY value the overlay answers a later
	// member. Two buffers alternate, because each overlay is built from the
	// one before it. Each is bounded by the attribute plus the 16 values one
	// member adds, and keeps its storage across runs.
	communityWire [2][]byte
	communityNext uint8
	// prefix holds the NLRI the prefix overlay answers a later member.
	prefix prefixOverlay
	// route is what an external member is sent beside the view; fields the
	// fields object it is sent, kept across runs.
	route  ExternalRoute
	fields []byte
}

// SetRoute records what external members are sent for the next runs. The
// caller MUST clear it (SetRoute(ExternalRoute{})) before the RunSets is
// reused for another route's caller or returned to a pool, because Raw is
// borrowed.
func (r *RunSets) SetRoute(route ExternalRoute) {
	r.route = route
	if r.fields == nil {
		r.fields = make([]byte, 0, fieldsOctets)
	}
}

// Folded answers the sets the chain asked, valid until the next Run.
func (r *RunSets) Folded() *Sets { return &r.folded }

// Run judges the route view holds. view MUST have been Reset for this route;
// the caller Clears it after it has applied the folded sets. Run allocates
// nothing on the pass and reject paths.
func (c *Chain) Run(view *policyview.View, sets *RunSets) Outcome {
	sets.folded.Reset()
	sets.prefix.receivedSet = false
	for i := range c.members {
		m := &c.members[i]
		sets.member.Reset()
		onError := m.onError
		var decision Decision
		var err error
		if m.external != nil {
			var verdict externalVerdict
			verdict, onError, err = judgeExternal(m, view, sets)
			decision = verdict.decision
			if err == nil && verdict.teardown && sets.route.Direction == directionImport {
				return Outcome{Action: OutcomeReject, Member: m.name, Teardown: true,
					NotifyCode: verdict.notifyCode, NotifySubcode: verdict.notifySub}
			}
		} else {
			decision, err = m.evaluate(view, &sets.member)
		}
		if err == nil {
			err = m.refuseUndeclared(&sets.member)
		}
		if err == nil {
			err = decisionError(decision)
		}
		if err != nil {
			if c.fail(m, onError, err) {
				return Outcome{Action: OutcomeReject, Failed: true, Member: m.name}
			}
			continue // fail-open: the member passes, and none of its sets apply
		}
		if decision == DecisionReject {
			return Outcome{Action: OutcomeReject, Member: m.name}
		}
		err = c.overlayNumeric(view, &sets.member)
		if err == nil {
			err = c.overlayCommunity(view, sets)
		}
		if err == nil {
			err = c.overlayPrefix(view, sets)
		}
		if err != nil {
			if c.fail(m, onError, err) {
				return Outcome{Action: OutcomeReject, Failed: true, Member: m.name}
			}
			continue // fail-open: none of the member's sets apply
		}
		if err := sets.folded.merge(&sets.member); err != nil {
			if c.fail(m, onError, err) {
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

// overlayNumeric resolves the member's numeric sets against what the view
// answers now (the route's value, or an earlier member's set), records each
// as a plain set, and overlays the view so a later member reads it
// (read-after-set). An increment or decrement of an absent attribute starts
// from the chain's base.
func (c *Chain) overlayNumeric(view *policyview.View, member *Sets) error {
	if member.written&setKindLocalPreference.bit() != 0 {
		value, err := resolveOnView(view, c.localPreference, member.localPreference, c.bases.LocalPreference)
		if err != nil {
			return fmt.Errorf("local-preference: %w", err)
		}
		member.localPreference = numericSet{replace: true, base: value}
		if err := view.SetNumber(c.localPreference, uint64(value)); err != nil {
			return fmt.Errorf("local-preference: %w", err)
		}
	}
	if member.written&setKindMED.bit() != 0 {
		value, err := resolveOnView(view, c.med, member.med, c.bases.MED)
		if err != nil {
			return fmt.Errorf("med: %w", err)
		}
		member.med = numericSet{replace: true, base: value}
		if err := view.SetNumber(c.med, uint64(value)); err != nil {
			return fmt.Errorf("med: %w", err)
		}
	}
	return nil
}

// overlayCommunity overlays the view's COMMUNITY with what the member's adds
// and removes make of it, so a later member reads the set value
// (read-after-set). The value is what the COMMUNITY handler writes for the
// same operations: the current values no remove names, in order, then the
// additions. It runs only for a member that changed communities, so a chain
// whose members set none reads the wire bytes and allocates nothing.
func (c *Chain) overlayCommunity(view *policyview.View, sets *RunSets) error {
	member := &sets.member
	if member.written&setKindCommunity.bit() == 0 {
		return nil
	}
	current, err := view.Read(c.community)
	if err != nil {
		return fmt.Errorf("community: %w", err)
	}
	buf := &sets.communityWire[sets.communityNext]
	sets.communityNext ^= 1
	out := (*buf)[:0]
	wire := current.Wire()
	// RFC 1997: "The attribute consists of a set of four octet values, each of which specify a community."
	for i := 0; i+4 <= len(wire); i += 4 {
		if !member.removesCommunity(binary.BigEndian.Uint32(wire[i:])) {
			out = append(out, wire[i:i+4]...)
		}
	}
	for _, community := range member.communityAdd[:member.communityAddN] {
		out = binary.BigEndian.AppendUint32(out, community)
	}
	*buf = out
	if err := view.SetWire(c.community, out); err != nil {
		return fmt.Errorf("community: %w", err)
	}
	return nil
}

// resolveOnView answers the value set gives the field reader reads.
func resolveOnView(view *policyview.View, reader policyview.Reader, set numericSet, base uint32) (uint32, error) {
	current, present, err := numberOf(view, reader)
	if err != nil {
		return 0, err
	}
	return set.resolve(current, present, base, true)
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
func (c *Chain) fail(m *member, onError rpc.OnErrorPolicy, err error) bool {
	c.refused.Add(1)
	chainLogger().Warn("policy filter failed, its on-error decides the route",
		"attachment", c.attachment, "filter", m.name, "type", m.filterType, "on-error", onError.String(), "error", err)
	return onError != rpc.OnErrorAccept
}

// directionImport is the direction an ExternalRoute names for a received
// route, the only one on which a teardown is honoured.
const directionImport = "import"
