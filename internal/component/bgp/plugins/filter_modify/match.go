// Design: docs/architecture/core-design.md -- route attribute modifier
// Related: config.go -- parseMatchBlock, which builds the condition
// Related: filter_modify.go -- handleFilterUpdate, which gates on it
//
// The condition under which a modify definition applies.
//
// A filter chain is a pipe in which a reject DROPS the route, so a match filter
// placed earlier can only express "modify these and discard everything else".
// The route that must keep flowing untouched has nowhere to go. A condition
// carried by the modifier itself expresses it: the operations apply to a route
// that meets the condition, and every other route leaves the filter unchanged.
package filter_modify

import "github.com/ze-software/ze/internal/component/bgp/filtertext"

// matchCommunity is one community value the route may carry, in one of the
// three community attributes.
type matchCommunity struct {
	kind  filtertext.CommunityKind
	value string
}

// matchCond is the condition a route meets before the operations apply.
//
// A zero matchCond states nothing and applies to every route, which is what
// every definition written before this container did.
type matchCond struct {
	// communities is satisfied by ANY listed value being present. The list is a
	// set of alternatives, not a conjunction: an operator who blackholes on two
	// communities states both and either one fires.
	communities []matchCommunity
}

// empty reports whether the condition states nothing.
func (c matchCond) empty() bool {
	return len(c.communities) == 0
}

// matches reports whether updateText satisfies the condition. An empty
// condition matches every route.
func (c matchCond) matches(updateText string) bool {
	if c.empty() {
		return true
	}
	for i := range c.communities {
		m := &c.communities[i]
		if filtertext.HasCommunity(updateText, m.kind, m.value) {
			return true
		}
	}
	return false
}
