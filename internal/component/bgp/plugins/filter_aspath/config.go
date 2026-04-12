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

// parseAsPathEntries reads the inner entry list for one as-path-list.
// Entries arrive as map[regex]map[leaves...] or []any (slice form).
// The slice form preserves order; the map form rejects >1 entries because
// first-match-wins requires deterministic order.
func parseAsPathEntries(listName string, listMap map[string]any) ([]aspathEntry, error) {
	rawEntries, ok := listMap["entry"]
	if !ok {
		return nil, nil
	}

	if entriesSlice, ok := rawEntries.([]any); ok {
		out := make([]aspathEntry, 0, len(entriesSlice))
		for _, item := range entriesSlice {
			entryMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("as-path-list %q: entry is not a map", listName)
			}
			e, err := parseOneASPathEntry(listName, entryMap)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	}

	if entriesMap, ok := rawEntries.(map[string]any); ok {
		if len(entriesMap) > 1 {
			return nil, fmt.Errorf("as-path-list %q: %d entries in map form would lose order (first-match-wins requires slice form)", listName, len(entriesMap))
		}
		out := make([]aspathEntry, 0, len(entriesMap))
		for keyRegex, item := range entriesMap {
			entryMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("as-path-list %q entry %q: not a map", listName, keyRegex)
			}
			if _, has := entryMap["regex"]; !has {
				entryMap["regex"] = keyRegex
			}
			e, err := parseOneASPathEntry(listName, entryMap)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	}

	return nil, fmt.Errorf("as-path-list %q: unexpected entry container type %T", listName, rawEntries)
}

// parseOneASPathEntry reads a single entry's leaves and compiles the regex.
func parseOneASPathEntry(listName string, m map[string]any) (aspathEntry, error) {
	regexStr, ok := m["regex"].(string)
	if !ok || regexStr == "" {
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
