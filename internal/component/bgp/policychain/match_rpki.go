// Design: docs/guide/route-policy.md -- RPKI origin validation match condition on BGP chains
// RFC: rfc/short/rfc8893.md -- Sections 1 and 3: RPKI origin validation on ingress and egress
// Related: match.go -- MatchConditions, per-prefix NLRI walk
// Related: ../policyview/rpki_origin.go -- View.EffectiveOriginAS

package policychain

import (
	"slices"

	"github.com/ze-software/ze/internal/component/policy"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// compileBGPRPKIValidationState compiles a match instance's
// rpki-validation-state condition for BGP ingress and egress chains.
//
// RFC 8893 Section 3: "BGP implementations supporting RPKI-based origin
// validation MUST provide the same policy configuration primitives for
// decisions based on the validation state available for use in ingress,
// redistribution, and egress policies. When applied to egress policy,
// validation state MUST be determined using the effective origin AS of the
// route as it will (or would) be announced to the peer."
func compileBGPRPKIValidationState(value any, _ policy.InstanceResolver) (policy.ConditionCheck[MatchRoute], error) {
	states, err := policy.ParseRPKIValidationStates(value)
	if err != nil {
		return nil, err
	}
	return func(r *MatchRoute) (bool, error) {
		// Origin validation governs announcements, not withdrawals: never drop
		// or block a route withdrawal on RPKI validation state.
		if r.Withdraw {
			return r.AcceptAction, nil
		}
		if !r.Prefix.IsValid() {
			return false, nil
		}
		originAS, ok, err := r.View.EffectiveOriginAS()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		state := ribpolicy.ValidateOrigin(r.Prefix, originAS)
		if state == ribpolicy.ValidationNotValidated {
			return false, nil
		}
		return slices.Contains(states, state), nil
	}, nil
}
