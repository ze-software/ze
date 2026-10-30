// Design: docs/architecture/rib/unified-locrib.md -- the import gate and its route evaluators
// Related: match.go -- evaluatePrefix, the matching the BGP path shares
// Related: config.go -- parsePrefixListEntries, the entry parse the BGP path shares
//
// The prefix-list route evaluator for the Loc-RIB import gate. It reuses the
// BGP path's entry parse and first-match evaluation, so a prefix-list accepts
// and rejects the same prefixes at a Loc-RIB attachment as on a BGP session.

package filter_prefix

import (
	"errors"

	"github.com/ze-software/ze/internal/component/policy"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// compileRouteEvaluator compiles one prefix-list instance into a Loc-RIB
// route evaluator: the first matching entry decides, accept passes the route
// on and reject stops the chain; a route no entry matches is rejected (the
// implicit deny of evaluatePrefix).
func compileRouteEvaluator(name string, instance map[string]any) (policy.RouteEvaluator, error) {
	if instance == nil {
		return nil, errors.New("the prefix-list has no entries to judge a route with")
	}
	entries, err := parsePrefixListEntries(name, instance)
	if err != nil {
		return nil, err
	}
	return func(r ribpolicy.Route) policy.RouteResult {
		return policy.RouteResult{Reject: evaluatePrefix(entries, r.Prefix) == actionReject}
	}, nil
}
