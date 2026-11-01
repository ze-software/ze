// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: match_route.go -- the built-in conditions judged over a ribpolicy.Route
// Related: register_match.go -- the match filter type and the built-in condition registrations
// Related: rib.go -- the route evaluator a match instance compiles into
// Related: fields.go -- the per-instance field declaration the conditions feed
//
// The match framework. A condition is a registration: the config key that
// names it in a match instance, the route fields it reads, and whether modify
// carries it too. Its check is registered separately for each route view it
// can judge (Conditions), so one condition serves a Loc-RIB route here and a
// BGP UPDATE on the BGP side without this package learning a BGP type. The
// framework names no condition: the field check, the compiled matcher and
// the source-condition answer all read the registry.

package policy

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// MatchFilterType is the ze:filter list of the generic match filter.
const MatchFilterType = "match"

// MatchCondition is one registered match condition, as
// MustRegisterMatchCondition returns it. Only a registration yields one, so a
// check cannot be added for a condition nobody declared.
type MatchCondition struct{ key string }

// Key answers the config key that names the condition in a match instance.
func (c MatchCondition) Key() string { return c.key }

// conditionDecl is what a condition declares once, whatever view judges it.
type conditionDecl struct {
	condition MatchCondition
	reads     []Field
	source    bool // a source condition: modify carries it too
}

// conditionDecls is the condition registry: every declared condition, sorted
// by key so the field answer and the compile order are stable. The
// registrations run from init(), the reads at config load; the mutex makes it
// safe for concurrent use.
var conditionDecls = struct {
	mu    sync.RWMutex
	byKey []conditionDecl
}{}

// MustRegisterMatchCondition declares a match condition: key is the leaf or
// container that names it in a match instance (the condition's YANG, which
// its package adds to the match list), reads the route fields its check
// reads, and source whether modify carries it beside its own matches. Every
// read field joins the match filter type's declaration, so a chain is checked
// against exactly what its instances name. A second registration of one key,
// an empty key, or a condition that reads nothing is a Ze defect and panics.
func MustRegisterMatchCondition(key string, reads []Field, source bool) MatchCondition {
	if key == "" {
		panic("BUG: policy: a match condition registered with no key")
	}
	if key == matchKeyAction {
		panic("BUG: policy: match condition " + key + " collides with the match action leaf")
	}
	if len(reads) == 0 {
		panic("BUG: policy: match condition " + key + " reads no route field")
	}
	conditionDecls.mu.Lock()
	defer conditionDecls.mu.Unlock()
	at, found := slices.BinarySearchFunc(conditionDecls.byKey, key, func(d conditionDecl, k string) int {
		return strings.Compare(d.condition.key, k)
	})
	if found {
		panic("BUG: policy: match condition " + key + " registered twice")
	}
	vocab.mustExtendFilterTypeReads(MatchFilterType, reads)
	cond := MatchCondition{key: key}
	conditionDecls.byKey = slices.Insert(conditionDecls.byKey, at, conditionDecl{condition: cond, reads: slices.Clone(reads), source: source})
	return cond
}

// declaredConditions answers the registered conditions, sorted by key. The
// caller MUST NOT change the slice.
func declaredConditions() []conditionDecl {
	conditionDecls.mu.RLock()
	defer conditionDecls.mu.RUnlock()
	return conditionDecls.byKey
}

// MatchFields answers the route fields the conditions cfg names read, in key
// order. sourceOnly restricts it to the source conditions (modify's).
func MatchFields(cfg map[string]any, sourceOnly bool) []Field {
	var reads []Field
	for _, d := range declaredConditions() {
		if sourceOnly && !d.source {
			continue
		}
		if _, named := cfg[d.condition.key]; !named {
			continue
		}
		for _, f := range d.reads {
			if !slices.Contains(reads, f) {
				reads = append(reads, f)
			}
		}
	}
	return reads
}

// IsSourceConditionKey reports whether key names a registered source
// condition, the conditions modify carries beside its community matches.
func IsSourceConditionKey(key string) bool {
	for _, d := range declaredConditions() {
		if d.condition.key == key {
			return d.source
		}
	}
	return false
}

// ConditionCheck judges one view against one compiled condition. It MUST be
// a pure function of view and the config it compiled from: no state of any
// plugin process, no I/O, no allocation, safe for concurrent use. A condition
// therefore runs in process only and follows no plugin transport. An error
// (a malformed field the view could not read) fails the route under the
// member's on-error; it is never read as a match or a miss.
type ConditionCheck[V any] func(view *V) (bool, error)

// ConditionCompiler compiles a condition's config value (the leaf, leaf-list
// or container the match instance holds under the condition's key) into its
// check. It MUST NOT change any state: a refused verify runs it. resolve
// answers the config of another instance the value names (a prefix-list).
type ConditionCompiler[V any] func(value any, resolve InstanceResolver) (ConditionCheck[V], error)

// Conditions holds the checks of the conditions one route view can judge.
// The view's owner creates it (RouteConditions for a ribpolicy.Route here; the
// BGP view's set on the BGP side) and each condition's package adds its check
// from init(). Safe for concurrent use.
type Conditions[V any] struct {
	view      string // how an error names a route of this view
	mu        sync.RWMutex
	compilers map[MatchCondition]ConditionCompiler[V]
}

// NewConditions creates the empty condition set of one route view. view
// names its routes in an error ("a Loc-RIB route").
func NewConditions[V any](view string) *Conditions[V] {
	return &Conditions[V]{view: view, compilers: make(map[MatchCondition]ConditionCompiler[V])}
}

// MustAdd registers how cond is judged on this view. A nil compiler, a
// condition nobody registered, or a second check for one condition is a Ze
// defect and panics.
func (c *Conditions[V]) MustAdd(cond MatchCondition, compile ConditionCompiler[V]) {
	if compile == nil {
		panic("BUG: policy: match condition " + cond.key + " added with no compiler")
	}
	if !conditionDeclared(cond) {
		panic("BUG: policy: match condition " + cond.key + " added before it was registered")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, dup := c.compilers[cond]; dup {
		panic("BUG: policy: match condition " + cond.key + " added twice for " + c.view)
	}
	c.compilers[cond] = compile
}

func conditionDeclared(cond MatchCondition) bool {
	for _, d := range declaredConditions() {
		if d.condition == cond {
			return true
		}
	}
	return false
}

func (c *Conditions[V]) compiler(cond MatchCondition) (ConditionCompiler[V], bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	compile, ok := c.compilers[cond]
	return compile, ok
}

// Match is one compiled match instance over view V: the checks of the
// conditions it names and its action. A route matches when every check
// holds; within one condition the values are alternatives. A route that does
// not carry a field a condition reads fails that condition: absence is never
// a match. Immutable after CompileMatch, so safe for concurrent use.
type Match[V any] struct {
	checks []ConditionCheck[V]
	// rejectMatching is the action leaf: true rejects a route meeting every
	// condition and passes the rest on, false (accept) does the opposite.
	rejectMatching bool
}

// CompileMatch compiles the conditions cfg names, over the checks set holds,
// and, for a match instance, its action leaf. sourceOnly compiles only the
// source conditions, for a caller (modify) whose other keys are its own and
// whose match block has no action. A value that does not parse, a reference
// resolve refuses, and a condition this view cannot judge are each an error
// naming the condition.
func CompileMatch[V any](set *Conditions[V], cfg map[string]any, sourceOnly bool, resolve InstanceResolver) (*Match[V], error) {
	m := &Match[V]{}
	for _, d := range declaredConditions() {
		if sourceOnly && !d.source {
			continue
		}
		value, named := cfg[d.condition.key]
		if !named {
			continue
		}
		compile, ok := set.compiler(d.condition)
		if !ok {
			return nil, fmt.Errorf("match %s: the condition cannot judge %s; use it on a chain whose routes it can judge",
				d.condition.key, set.view)
		}
		check, err := compile(value, resolve)
		if err != nil {
			return nil, fmt.Errorf("match %s: %w", d.condition.key, err)
		}
		m.checks = append(m.checks, check)
	}
	if sourceOnly {
		return m, nil
	}
	rejectMatching, err := parseMatchAction(cfg)
	if err != nil {
		return nil, err
	}
	m.rejectMatching = rejectMatching
	return m, nil
}

// Named reports whether the match names any condition.
func (m *Match[V]) Named() bool { return len(m.checks) > 0 }

// Matches reports whether view meets every condition. It allocates nothing.
func (m *Match[V]) Matches(view *V) (bool, error) {
	for _, check := range m.checks {
		holds, err := check(view)
		if err != nil {
			return false, err
		}
		if !holds {
			return false, nil
		}
	}
	return true, nil
}

// Rejects answers the match instance's verdict on view: with action accept,
// view is rejected unless it meets every condition; with action reject, it
// is rejected when it meets them. It allocates nothing.
func (m *Match[V]) Rejects(view *V) (bool, error) {
	matches, err := m.Matches(view)
	if err != nil {
		return false, err
	}
	return matches == m.rejectMatching, nil
}

// matchKeyAction is the match instance's action leaf. It is not a condition:
// it reads no route field, so no condition may take its key.
const matchKeyAction = "action"

// The values of the action leaf, as the YANG enumeration spells them.
const (
	matchActionAccept = "accept"
	matchActionReject = "reject"
)

// parseMatchAction answers whether a route meeting the conditions is rejected.
// An instance that leaves the leaf out accepts, the YANG default; any value
// other than the two the enumeration names is refused, never read as accept.
func parseMatchAction(instance map[string]any) (rejectMatching bool, err error) {
	value, named := instance[matchKeyAction]
	if !named {
		return false, nil
	}
	switch value {
	case matchActionAccept:
		return false, nil
	case matchActionReject:
		return true, nil
	}
	return false, fmt.Errorf("match action %v: want %s or %s", value, matchActionAccept, matchActionReject)
}

// matchInstanceFields is the match filter type's per-instance declaration.
func matchInstanceFields(instance map[string]any) (reads, sets []Field) {
	return MatchFields(instance, false), nil
}
