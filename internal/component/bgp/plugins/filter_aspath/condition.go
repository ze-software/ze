// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: register.go -- registers the as-path condition at a BGP chain
// Related: match.go -- evaluateASPathText, the first-match walk the condition runs
// Related: config.go -- parseAsPathEntries, the entry parse the condition shares
//
// The as-path match condition: a match instance's `as-path NAME` passes an
// UPDATE whose AS path the named as-path-list accepts. It reads the list's
// own definition and walks the same regular expressions over the same text,
// so it accepts exactly what the list accepts as a chain member.

package filter_aspath

import (
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
)

// compileBGPCondition compiles a match instance's as-path value, the name of
// one as-path-list instance, into its check over a BGP UPDATE: the UPDATE
// passes when the list's first matching entry accepts its AS path, read as
// the flat decimal text (policyview.View.ASPathText); a path no entry
// matches fails (the implicit deny of evaluateASPathText). An UPDATE with no
// AS path is judged as the empty text, as the list's own filter judges it
// (handleFilterUpdate). A malformed AS path fails the member.
func compileBGPCondition(value any, resolve policy.InstanceResolver) (policy.ConditionCheck[policychain.MatchRoute], error) {
	entries, err := conditionEntries(value, resolve)
	if err != nil {
		return nil, err
	}
	return func(r *policychain.MatchRoute) (bool, error) {
		text, err := r.View.ASPathText()
		if err != nil {
			return false, err
		}
		return evaluateASPathText(entries, text) == actionAccept, nil
	}, nil
}

// conditionEntries reads the as-path-list the condition value names, from
// the instance config, and compiles its entries.
func conditionEntries(value any, resolve policy.InstanceResolver) ([]aspathEntry, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one as-path-list instance")
	}
	if name == "" {
		return nil, errors.New("must name one as-path-list instance")
	}
	if resolve == nil {
		return nil, errors.New("BUG: compiled with no instance resolver")
	}
	instance, err := resolve(name, filterTypeName)
	if err != nil {
		return nil, err
	}
	return parseAsPathEntries(name, instance)
}
