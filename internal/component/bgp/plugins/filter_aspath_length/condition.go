// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: register.go -- registers the as-path-length condition at a BGP chain
// Related: aspath_length.go -- evaluateASPathLength and the hop count the condition shares
// Related: config.go -- parseAsPathLengthDef, the bounds parse the condition shares
//
// The as-path-length match condition: a match instance's `as-path-length
// NAME` passes an UPDATE whose AS path hop count lies within the named
// instance's bounds. It reads the instance's own definition and counts the
// hops the filter counts, so it accepts exactly what the filter accepts as a
// chain member.

package filter_aspath_length

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
)

// compileBGPCondition compiles a match instance's as-path-length value, the
// name of one as-path-length instance, into its check over a BGP UPDATE: the
// UPDATE passes when its AS path hop count (pathHops) lies within the
// instance's min and max. An UPDATE with no AS path counts no hops, as the
// filter counts it (handleFilterUpdate). A malformed AS path fails the
// member.
func compileBGPCondition(value any, resolve policy.InstanceResolver) (policy.ConditionCheck[policychain.MatchRoute], error) {
	def, err := conditionDef(value, resolve)
	if err != nil {
		return nil, err
	}
	reader, err := policyview.ReaderFor(policy.FieldASPath)
	if err != nil {
		return nil, fmt.Errorf("BUG: as-path-length: %w", err)
	}
	return func(r *policychain.MatchRoute) (bool, error) {
		path, err := r.View.Read(reader)
		if err != nil {
			return false, err
		}
		return evaluateASPathLength(pathHops(path.ASPath()), def), nil
	}, nil
}

// conditionDef reads the as-path-length instance the condition value names,
// from the instance config, and parses its bounds.
func conditionDef(value any, resolve policy.InstanceResolver) (*asPathLengthDef, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one as-path-length instance")
	}
	if name == "" {
		return nil, errors.New("must name one as-path-length instance")
	}
	if resolve == nil {
		return nil, errors.New("BUG: compiled with no instance resolver")
	}
	instance, err := resolve(name, filterTypeName)
	if err != nil {
		return nil, err
	}
	return parseAsPathLengthDef(name, instance)
}
