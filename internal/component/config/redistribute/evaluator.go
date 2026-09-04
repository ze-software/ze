// Design: docs/architecture/core-design.md -- redistribution evaluator
// Related: route.go -- RedistRoute, ImportRule, evaluate
// Related: registry.go -- source registry for protocol lookup

package redistribute

import (
	"slices"
	"sync"
	"sync/atomic"
)

// global is the singleton evaluator, set by SetGlobal during startup.
// Protocols call Global() to get the evaluator for route acceptance checks.
var global atomic.Pointer[Evaluator]

// SetGlobal installs the evaluator as the global singleton.
// Called from config loading after parsing redistribute rules.
func SetGlobal(ev *Evaluator) {
	global.Store(ev)
}

// Global returns the global evaluator, or nil if redistribution is not configured.
func Global() *Evaluator {
	return global.Load()
}

// Evaluator holds redistribution import rules and evaluates routes against them.
// Thread-safe: rules are swapped atomically on config reload.
type Evaluator struct {
	mu    sync.RWMutex
	rules []ImportRule
}

// NewEvaluator creates a redistribution evaluator with the given import rules.
func NewEvaluator(rules []ImportRule) *Evaluator {
	return &Evaluator{rules: rules}
}

// Reload replaces the import rules (called on config reload).
func (e *Evaluator) Reload(rules []ImportRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = rules
}

// Accept checks whether a route should be imported into the given protocol.
// Returns true if any import rule accepts the route without creating a loop.
//
// importingProtocol MUST name a protocol. An empty name panics.
func (e *Evaluator) Accept(route RedistRoute, importingProtocol string) bool {
	// Guarded here as well as in ImportRule.Accept. An empty rule list never
	// runs that loop, so the callee's guard alone would let a caller with no
	// importing protocol read back a silent false.
	mustNameImportingProtocol(importingProtocol)
	e.mu.RLock()
	defer e.mu.RUnlock()
	return evaluate(route, e.rules, importingProtocol)
}

// Rules returns a deep copy of the current import rules (for diagnostics/CLI).
func (e *Evaluator) Rules() []ImportRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]ImportRule, len(e.rules))
	for i, r := range e.rules {
		out[i] = ImportRule{
			Source:      r.Source,
			Destination: r.Destination,
			Families:    slices.Clone(r.Families),
		}
	}
	return out
}

// HasDestination reports whether any rule feeds the given destination protocol.
// A rule with an empty Destination is destination-agnostic and matches every
// destination. The redistribute orchestrator calls this on a BGP peer-up to
// skip firing a replay request when no import feeds BGP.
//
// dest MUST name a protocol. An empty name panics: a destination IS the
// importing protocol, so an unnamed one carries the same ambiguity Accept
// refuses, and a false here silently cancels a replay.
func (e *Evaluator) HasDestination(dest string) bool {
	mustNameImportingProtocol(dest)
	e.mu.RLock()
	defer e.mu.RUnlock()
	for i := range e.rules {
		if e.rules[i].Destination == "" || e.rules[i].Destination == dest {
			return true
		}
	}
	return false
}
