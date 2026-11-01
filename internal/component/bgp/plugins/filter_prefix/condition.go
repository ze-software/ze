// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: register.go -- registers the prefix-list condition
// Related: match.go -- evaluatePrefix, the first-match walk the condition shares
// Related: config.go -- parsePrefixListEntries, the entry parse the condition shares
//
// The prefix-list match condition: a match instance's `prefix-list NAME`
// passes a route whose prefix the named list accepts. It is the only
// prefix-list check a match runs, and it reads the list's own definition, so
// it accepts exactly what the list accepts on a BGP session.

package filter_prefix

import (
	"errors"

	"github.com/ze-software/ze/internal/component/policy"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// compileRouteCondition compiles a match instance's prefix-list value, the
// name of one prefix-list instance, into its check over a Loc-RIB route. The
// route passes when the list's first matching entry accepts its prefix; a
// prefix no entry matches fails (the implicit deny of evaluatePrefix).
func compileRouteCondition(value any, resolve policy.InstanceResolver) (policy.RouteCheck, error) {
	entries, err := conditionEntries(value, resolve)
	if err != nil {
		return nil, err
	}
	return func(r *ribpolicy.Route) (bool, error) {
		return evaluatePrefix(entries, r.Prefix) == actionAccept, nil
	}, nil
}

// conditionEntries reads the prefix-list the condition value names and
// parses its entries.
func conditionEntries(value any, resolve policy.InstanceResolver) ([]prefixEntry, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one prefix-list instance")
	}
	if name == "" {
		return nil, errors.New("must name one prefix-list instance")
	}
	if resolve == nil {
		return nil, errors.New("BUG: compiled with no instance resolver")
	}
	instance, err := resolve(name, filterTypeName)
	if err != nil {
		return nil, err
	}
	return parsePrefixListEntries(name, instance)
}
