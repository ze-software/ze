// Design: docs/architecture/core-design.md -- community match filter matching
// Related: config.go -- community-list config parsing
// Related: condition.go -- the community condition of a match instance
//
// A community-match list is an ordered list of match entries. The community
// condition (condition.go) checks, for each entry in order, whether the
// UPDATE's community attribute (standard, large, or extended) carries the
// entry's value. First match wins; no match yields actionReject (implicit
// deny).
//
// Like AS-path filtering, community matching is attribute-level: the result
// is always accept or reject for the whole UPDATE (never modify).
package filter_community_match

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
type communityType = attribute.CommunityKind

const (
	communityStandard = attribute.CommunityStandard
	communityLarge    = attribute.CommunityLarge
	communityExtended = attribute.CommunityExtended
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
