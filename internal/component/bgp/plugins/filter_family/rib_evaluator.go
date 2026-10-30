// Design: docs/architecture/rib/unified-locrib.md -- the import gate and its route evaluators
// Related: config.go -- parseFamilyFilter, the instance parse the BGP path shares
//
// The family-filter route evaluator for the Loc-RIB import gate.

package filter_family

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/policy"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// compileRouteEvaluator compiles one family-filter instance into a Loc-RIB
// route evaluator: `action remove` rejects a route of the instance's family
// and passes every other route on, as it removes that family's NLRI from a
// BGP UPDATE. `action tear-down` closes a BGP session, which a Loc-RIB route
// has none of, so that instance is refused here.
func compileRouteEvaluator(name string, instance map[string]any, _ policy.InstanceResolver) (policy.RouteEvaluator, error) {
	if instance == nil {
		return nil, errors.New("the family-filter has no family to judge a route with")
	}
	parsed, err := parseFamilyFilter(name, instance)
	if err != nil {
		return nil, err
	}
	if parsed.action == actionTearDown {
		return nil, fmt.Errorf("family-filter %q has action %s, which closes a BGP session; "+
			"a Loc-RIB route has no session to close, so set action %s or use another filter here",
			name, actionTearDown, actionRemove)
	}
	fam := parsed.family
	return func(r ribpolicy.Route) policy.RouteResult {
		return policy.RouteResult{Reject: r.Family == fam}
	}, nil
}
