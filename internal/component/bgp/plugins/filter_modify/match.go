// Design: docs/architecture/core-design.md -- route attribute modifier
// Related: config.go -- parseMatchBlock, which builds the condition
// Related: filter_modify.go -- evaluateModify, which gates on it
//
// The condition under which a modify definition applies.
//
// A filter chain is a pipe in which a reject DROPS the route, so a match filter
// placed earlier can only express "modify these and discard everything else".
// The route that must keep flowing untouched has nowhere to go. A condition
// carried by the modifier itself expresses it: the operations apply to a route
// that meets the condition, and every other route leaves the filter unchanged.
package filter_modify

import (
	"bytes"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// matchCommunity is one community value the route may carry, in one of the
// three community attributes, held as the bytes the route's fields carry it
// in: key is the field's JSON key with its colon (`"community":`), and
// needle the value as a quoted JSON string (`"65000:100"`), the canonical
// text the community formatter writes. Both are built once at config time,
// so a match allocates nothing per route.
type matchCommunity struct {
	key    []byte
	needle []byte
}

// newMatchCommunity builds the match of canonical, a value already in the
// formatter's canonical text, in the field of kind.
func newMatchCommunity(kind attribute.CommunityKind, canonical string) matchCommunity {
	key := append(append([]byte{'"'}, kind.FieldName()...), '"', ':')
	needle := append(append([]byte{'"'}, canonical...), '"')
	return matchCommunity{key: key, needle: needle}
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

// matches reports whether fields, the route's fields object as the engine
// writes it (compact JSON, each community field a list of strings), satisfies
// the condition. An empty condition matches every route.
func (c matchCond) matches(fields []byte) bool {
	if c.empty() {
		return true
	}
	for i := range c.communities {
		if c.communities[i].in(fields) {
			return true
		}
	}
	return false
}

// in reports whether the field m names lists m's value. The value is looked
// for between the field's opening bracket and the first closing one, which a
// community string cannot hold, and as a quoted string, so "65000:1" never
// matches "65000:10". A key cannot match inside a longer key either: it opens
// with its quote, which a key like "large-community" holds only before
// "large".
func (m *matchCommunity) in(fields []byte) bool {
	at := bytes.Index(fields, m.key)
	if at < 0 {
		return false
	}
	list := fields[at+len(m.key):]
	if !bytes.HasPrefix(list, []byte{'['}) {
		return false
	}
	list, _, found := bytes.Cut(list, []byte{']'})
	if !found {
		return false
	}
	return bytes.Contains(list, m.needle)
}
