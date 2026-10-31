package redistribute

import (
	"slices"

	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// The orchestrator tests that judge dispatch, held routes and replay build
// their rules with hand-made chains of the shapes the policy library compiles.
// The compiled path is proven in config_test.go (TestRuleChainCompilesFromPolicy,
// TestRuleNarrowingWithdrawsHeldRoutes).

// familyChain accepts a route of one of its families and rejects any other:
// the verdict of `match { family [ ... ]; }` followed by final accept.
type familyChain []family.Family

func (c familyChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict {
	if slices.Contains(c, r.Family) {
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}
	return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
}

// tagChain accepts a route carrying its own tag and rejects any other, as a
// compiled `match tag [ N ]` does. redistRoute gives every route a tag, zero
// for a route with no tag, so tagChain(0) selects the untagged routes.
type tagChain uint32

func (c tagChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict {
	if r.HasTag && r.Tag == uint32(c) {
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}
	return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
}
