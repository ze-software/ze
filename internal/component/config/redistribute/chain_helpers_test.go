package redistribute

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// This package cannot import the policy library that compiles a rule's chain
// (policy imports it), so its unit tests judge Accept with hand-built chains
// of the shapes the library compiles. The compiled path itself is proven in
// internal/plugins/redistribute (TestRuleChainCompilesFromPolicy).

// familyChain accepts a route of one of its families and rejects any other:
// the verdict of `match { family [ ... ]; }` followed by final accept.
type familyChain []family.Family

func (c familyChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict {
	if slices.Contains(c, r.Family) {
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}
	return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
}

// tagChain accepts a route carrying its tag and rejects any other: the verdict
// of `match { tag [ N ]; }` followed by final accept. A route whose tag is
// unset (HasTag false) is rejected; redistRoute gives an untagged route tag 0.
type tagChain uint32

func (c tagChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict {
	if r.HasTag && r.Tag == uint32(c) {
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}
	return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
}

// prefixSetChain accepts a route whose prefix is in the set and sets tag on
// it, and rejects any other: `filters [ P M ]; final accept;` where P rejects
// every prefix outside the set and M sets the tag. A filter's accept only
// passes the route on, so `final reject` would reject the set too.
type prefixSetChain struct {
	accept []netip.Prefix
	tag    uint32
}

func (c prefixSetChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict {
	if slices.Contains(c.accept, r.Prefix) {
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept, Sets: ribpolicy.Sets{Tag: c.tag, HasTag: true}}
	}
	return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
}

// acceptor is a rule or an evaluator: both judge one route for one importer.
type acceptor interface {
	Accept(route *RedistRoute, importingProtocol string) (ribpolicy.Sets, bool)
}

// accepts answers the verdict alone, for the tests that judge no sets.
func accepts(a acceptor, route RedistRoute, importingProtocol string) bool {
	_, ok := a.Accept(&route, importingProtocol)
	return ok
}

// evaluates is evaluate's verdict alone.
func evaluates(route RedistRoute, rules []ImportRule, importingProtocol string) bool {
	_, ok := evaluate(&route, rules, importingProtocol)
	return ok
}
