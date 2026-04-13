// Design: docs/architecture/core-design.md -- redistribution evaluator
// Related: route.go -- RedistRoute, ImportRule, Evaluate
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
func (e *Evaluator) Accept(route RedistRoute, importingProtocol string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return Evaluate(route, e.rules, importingProtocol)
}

// Rules returns a deep copy of the current import rules (for diagnostics/CLI).
func (e *Evaluator) Rules() []ImportRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]ImportRule, len(e.rules))
	for i, r := range e.rules {
		out[i] = ImportRule{
			Source:   r.Source,
			Families: slices.Clone(r.Families),
		}
	}
	return out
}
