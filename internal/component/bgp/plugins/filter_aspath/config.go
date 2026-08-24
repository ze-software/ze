// Design: docs/architecture/core-design.md -- AS-path filter config parsing
// Related: match.go -- AS-path regex matching algorithm
// Related: filter_aspath.go -- SDK entry point and handleFilterUpdate
//
// Config parsing for the bgp-filter-aspath plugin.
//
// Reads named as-path-list definitions out of the BGP config subtree:
//
//	bgp { policy { as-path-list NAME { entry REGEX { action A; } } } }
//
// Each list becomes an *aspathList with ordered entries whose regexes are
// compiled at config load time. Invalid or overly long regexes are rejected
// immediately. Go's regexp package uses RE2 semantics (linear time, no
// backtracking), providing inherent ReDoS protection.
package filter_aspath

import (
	"fmt"
	"regexp"

	"github.com/ze-software/ze/internal/core/configorder"
)

const (
	// maxRegexLen is the maximum allowed regex string length (defense in depth).
	maxRegexLen = 512
	// maxNameLen is the maximum allowed as-path-list name length.
	maxNameLen = 256
)

// parseAsPathLists walks bgp { policy { as-path-list ... } } and returns a
// map of name -> *aspathList ready for runtime evaluation.
func parseAsPathLists(bgpCfg map[string]any) (map[string]*aspathList, error) {
	result := make(map[string]*aspathList)

	policyBlock, ok := bgpCfg["policy"].(map[string]any)
	if !ok {
		return result, nil
	}
	aplBlock, ok := policyBlock["as-path-list"].(map[string]any)
	if !ok {
		return result, nil
	}

	for name, raw := range aplBlock {
		if len(name) > maxNameLen {
			return nil, fmt.Errorf("as-path-list name %q exceeds maximum length %d", name, maxNameLen)
		}
		listMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("as-path-list %q: not a map", name)
		}
		entries, err := parseAsPathEntries(name, listMap)
		if err != nil {
			return nil, err
		}
		result[name] = &aspathList{name: name, entries: entries}
	}
	return result, nil
}

// parseAsPathEntries reads the inner entry list for one as-path-list, in the
// order the operator wrote the entries.
//
// The `entry` list is `ordered-by user` (yang/ze-filter-aspath.yang) because evaluation is
// first-match-wins, so configorder.Entries is the reader. configvalue.ListEntries
// sorts by key, which would silently reorder the entries a filter decision
// depends on.
//
// A list of two or more entries delivered with no order is still refused, by
// configorder rather than here. That refusal is the guard that made this defect
// loud instead of silent, and it stays.
func parseAsPathEntries(listName string, listMap map[string]any) ([]aspathEntry, error) {
	entries, err := configorder.Entries(listMap, "entry", "regex")
	if err != nil {
		return nil, fmt.Errorf("as-path-list %q: %w", listName, err)
	}

	out := make([]aspathEntry, 0, len(entries))
	for _, entry := range entries {
		parsed, err := parseOneASPathEntry(listName, entry.Key, entry.Map)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

// parseOneASPathEntry reads a single entry's leaves and compiles the regex.
//
// regexStr is the entry's key. It arrives as an argument because the delivered
// map form keys the list by the regex and omits the leaf from the entry, so
// there is nothing in m to read it from (configorder.Entry).
func parseOneASPathEntry(listName, regexStr string, m map[string]any) (aspathEntry, error) {
	if regexStr == "" {
		return aspathEntry{}, fmt.Errorf("as-path-list %q: entry missing regex leaf", listName)
	}

	if len(regexStr) > maxRegexLen {
		return aspathEntry{}, fmt.Errorf("as-path-list %q: regex %q exceeds maximum length %d", listName, regexStr, maxRegexLen)
	}

	compiled, err := regexp.Compile(regexStr)
	if err != nil {
		return aspathEntry{}, fmt.Errorf("as-path-list %q: invalid regex %q: %w", listName, regexStr, err)
	}

	act, err := parseASPathAction(listName, regexStr, m["action"])
	if err != nil {
		return aspathEntry{}, err
	}

	return aspathEntry{
		regex:  compiled,
		action: act,
	}, nil
}

// parseASPathAction validates the action leaf, returning the YANG default
// (accept) when the leaf is absent and an error for any unknown value.
func parseASPathAction(listName, regexStr string, raw any) (action, error) {
	if raw == nil {
		return actionAccept, nil
	}
	s, ok := raw.(string)
	if !ok {
		return actionAccept, fmt.Errorf("as-path-list %q entry %q: action is not a string", listName, regexStr)
	}
	if s == "accept" {
		return actionAccept, nil
	}
	if s == "reject" {
		return actionReject, nil
	}
	return actionAccept, fmt.Errorf("as-path-list %q entry %q: invalid action %q", listName, regexStr, s)
}
