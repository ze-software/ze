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
	"slices"
	"strings"
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

// communityType identifies which community attribute to check.
type communityType int

const (
	communityStandard communityType = iota
	communityLarge
	communityExtended
)

func (ct communityType) String() string {
	switch ct {
	case communityStandard:
		return "standard"
	case communityLarge:
		return "large"
	case communityExtended:
		return "extended"
	}
	return "unknown"
}

// textFieldName returns the attribute keyword in the filter text format.
func (ct communityType) textFieldName() string {
	switch ct {
	case communityStandard:
		return "community"
	case communityLarge:
		return "large-community"
	case communityExtended:
		return "extended-community"
	}
	return ""
}

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
		values := extractCommunityField(updateText, e.ctype)
		if containsCommunity(values, e.community) {
			return e.action
		}
	}
	return actionReject
}

// extractCommunityField extracts the community values from the filter text
// for the specified type. Returns individual value strings.
//
// Text format examples:
//   - "community 65001:100" -> ["65001:100"]
//   - "community [65001:100 no-export]" -> ["65001:100", "no-export"]
//   - "large-community 65000:1:2" -> ["65000:1:2"]
//   - "extended-community 000200010000000a" -> ["000200010000000a"]
//
// For standard community, uses cutOnWordBoundary to avoid false-matching
// "community " inside "extended-community " or "large-community ".
// Large and extended names are unique prefixes with no collision risk.
func extractCommunityField(updateText string, ctype communityType) []string {
	fieldName := ctype.textFieldName()
	if fieldName == "" {
		return nil
	}

	rest, ok := cutOnWordBoundary(updateText, fieldName)
	if !ok {
		return nil
	}

	value := extractValueUntilNextAttr(rest)
	if value == "" {
		return nil
	}

	// Strip brackets and split: "[val1 val2]" -> ["val1", "val2"]
	if len(value) >= 2 && value[0] == '[' && value[len(value)-1] == ']' {
		return strings.Fields(value[1 : len(value)-1])
	}
	return []string{value}
}

// cutOnWordBoundary finds "keyword " in text where keyword is preceded by
// start-of-string or a space (word boundary). Returns the text after
// "keyword " and true, or ("", false) if not found. This prevents
// "community " from matching inside "extended-community " or
// "large-community ".
func cutOnWordBoundary(text, keyword string) (string, bool) {
	needle := keyword + " "

	// Check start of string.
	if strings.HasPrefix(text, needle) {
		return text[len(needle):], true
	}

	// Check after a space: " keyword ".
	_, after, ok := strings.Cut(text, " "+needle)
	if ok {
		return after, true
	}

	return "", false
}

// containsCommunity checks if target is present in the values slice.
func containsCommunity(values []string, target string) bool {
	return slices.Contains(values, target)
}

// extractValueUntilNextAttr returns the portion of s up to the first token
// that is a known attribute keyword. Handles both bracketed [val1 val2] and
// single-value forms.
func extractValueUntilNextAttr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end >= 0 {
			return s[:end+1]
		}
		return s
	}
	before, _, found := strings.Cut(s, " ")
	if !found {
		return s
	}
	return before
}
