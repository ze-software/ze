// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: match.go -- the condition registry
// Related: match_route.go -- the route checks registered here

package policy

import ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"

// RPKIValidationStateCondition is the match condition for RPKI origin
// validation state (RFC 8893 Section 3), shared by RouteConditions (redistribution)
// and policychain.MatchConditions (BGP ingress and egress).
var RPKIValidationStateCondition MatchCondition

func init() {
	// The generic match filter: always built, so a Loc-RIB chain has a
	// filter type in a build without BGP. Its type declares no field of its
	// own: each condition adds what it reads (MustRegisterMatchCondition),
	// and each instance is checked against only the conditions it names.
	MustRegisterFilterType(MatchFilterType, nil, nil)
	MustRegisterInstanceFields(MatchFilterType, matchInstanceFields)
	MustRegisterRouteEvaluator(MatchFilterType, compileMatchEvaluator)

	// The built-in conditions over the route view's own fields, each keyed
	// by the field it reads. A source condition is one modify carries too.
	for _, c := range []struct {
		field   Field
		source  bool
		compile ConditionCompiler[ribpolicy.Route]
	}{
		{FieldFamily, false, compileRouteFamily},
		{FieldProtocol, true, compileRouteProtocol},
		{FieldNextHop, false, compileRouteNextHop},
		{FieldInterface, false, compileRouteInterface},
		{FieldMetric, false, compileRouteMetric},
		{FieldTag, true, compileRouteTag},
		{FieldLevel, true, compileRouteLevel},
		{FieldRouteType, true, compileRouteRouteType},
	} {
		cond := MustRegisterMatchCondition(c.field.name, []Field{c.field}, c.source)
		RouteConditions.MustAdd(cond, c.compile)
	}

	// RFC 8893 Section 3: "BGP implementations supporting RPKI-based origin
	// validation MUST provide the same policy configuration primitives for
	// decisions based on the validation state available for use in ingress,
	// redistribution, and egress policies."
	// Declares FieldPrefix alongside FieldRPKIValidationState so BGP chains
	// evaluate validation state per NLRI prefix.
	RPKIValidationStateCondition = MustRegisterMatchCondition(
		FieldRPKIValidationState.name,
		[]Field{FieldPrefix, FieldRPKIValidationState},
		false,
	)
	RouteConditions.MustAdd(RPKIValidationStateCondition, compileRouteRPKIValidationState)
}
