// Design: plan/post-release/engineering/spec-filter-wire-0-umbrella.md -- D-12, a filter plugin answers over the route view
// Related: external.go -- judgeExternal, which calls such a filter's plugin
// Related: ../config/library_chains.go -- compileLibraryChain, the reader

package policychain

import "sync"

// viewFilters holds the filter types whose plugin answers over the route
// view (D-12): it declares the fields it reads, answers sets and named
// intents, and never reads or writes the text update. Only the library chain
// applies sets, so a BGP chain naming one of these types compiles to the
// library chain even when no member evaluates in process; its plugin is then
// an external member of that chain (judgeExternal).
var viewFilters = struct {
	mu    sync.RWMutex
	types map[string]struct{}
}{types: map[string]struct{}{}}

// MustRegisterViewFilter registers filterType as answered by its plugin over
// the route view. The plugin that owns the type calls it from init(), beside
// policy.MustRegisterFilterType; its filter MUST declare the fields it reads,
// since judgeExternal fails a member that declares none. A second
// registration, or one beside an in-process evaluator, is a Ze defect and
// panics.
func MustRegisterViewFilter(filterType string) {
	if HasEvaluator(filterType) {
		panic("BUG: policychain: " + filterType + " has an in-process evaluator and a plugin filter")
	}
	viewFilters.mu.Lock()
	defer viewFilters.mu.Unlock()
	if _, dup := viewFilters.types[filterType]; dup {
		panic("BUG: policychain: second view filter registration for " + filterType)
	}
	viewFilters.types[filterType] = struct{}{}
}

// AnswersOverView reports whether filterType's plugin answers over the route
// view (MustRegisterViewFilter).
func AnswersOverView(filterType string) bool {
	viewFilters.mu.RLock()
	defer viewFilters.mu.RUnlock()
	_, ok := viewFilters.types[filterType]
	return ok
}
