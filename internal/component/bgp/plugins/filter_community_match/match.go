// Design: docs/architecture/core-design.md -- community match filter matching
// Related: config.go -- community-list config parsing
// Related: filter_community_match.go -- SDK entry point and handleFilterUpdate
//
// The matching algorithm walks an ordered list of community match entries.
// For each entry, the UPDATE's community attribute (standard, large, or
// extended) is extracted from the filter text format, parsed into individual
// values, and checked for presence of the entry's match value. First match
// wins; no match yields actionReject (implicit deny).
//
// Like AS-path filtering, community matching is attribute-level: the result
// is always accept or reject for the whole UPDATE (never modify).
package filter_community_match

import (
	"github.com/ze-software/ze/internal/component/bgp/filtertext"
)

// action is the per-entry decision applied when a community matches.
type action int

const (
	actionAccept action = iota
	actionReject
)

func (a action) String() string {
	if a == actionAccept {
		return "accept"
	}
	return "reject"
}

// communityType identifies which community attribute to check. It is an alias
// on the shared definition, not a second one: filter_modify decides on the same
// attribute of the same text, and one reading serves both.
type communityType = filtertext.CommunityKind

const (
	communityStandard = filtertext.CommunityStandard
	communityLarge    = filtertext.CommunityLarge
	communityExtended = filtertext.CommunityExtended
)

// communityEntry is one ordered match entry inside a community-list.
type communityEntry struct {
	community string        // value to match (as it appears in text format)
	ctype     communityType // which attribute to check
	action    action
}

// communityList is a named ordered list of match entries.
type communityList struct {
	name    string
	entries []communityEntry
}

// evaluateCommunities walks the entries in order and returns the action of
// the first entry whose community value is found in the UPDATE's community
// attributes. Returns actionReject if no entry matches (implicit deny).
func evaluateCommunities(entries []communityEntry, updateText string) action {
	for i := range entries {
		e := &entries[i]
		if filtertext.HasCommunity(updateText, e.ctype, e.community) {
			return e.action
		}
	}
	return actionReject
}
