// Design: docs/architecture/core-design.md -- community match filter config parsing
// Related: match.go -- community matching algorithm
// Related: filter_community_match.go -- SDK entry point and handleFilterUpdate
//
// Config parsing for the bgp-filter-community-match plugin.
//
// Reads named community-match definitions out of the BGP config subtree:
//
//	bgp { policy { community-match NAME { entry COMMUNITY { type T; action A; } } } }
//
// Each list becomes a *communityList with ordered entries. Community values
// are stored as strings and matched against the text format output at runtime.
// Values are checked for non-empty and length limit but not parsed, because
// the match is a string comparison against what filter_format.go emits.
package filter_community_match

import "fmt"

const (
	// maxNameLen is the maximum allowed community-list name length.
	maxNameLen = 256
	// maxCommunityLen is the maximum allowed community value string length.
	maxCommunityLen = 256
)

// parseCommunityLists walks bgp { policy { community-match ... } } and returns
// a map of name -> *communityList ready for runtime evaluation.
func parseCommunityLists(bgpCfg map[string]any) (map[string]*communityList, error) {
	result := make(map[string]*communityList)

	policyBlock, ok := bgpCfg["policy"].(map[string]any)
	if !ok {
		return result, nil
	}
	cmBlock, ok := policyBlock["community-match"].(map[string]any)
	if !ok {
		return result, nil
	}

	for name, raw := range cmBlock {
		if len(name) > maxNameLen {
			return nil, fmt.Errorf("community-match name %q exceeds maximum length %d", name, maxNameLen)
		}
		listMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("community-match %q: not a map", name)
		}
		entries, err := parseCommunityEntries(name, listMap)
		if err != nil {
			return nil, err
		}
		result[name] = &communityList{name: name, entries: entries}
	}
	return result, nil
}

// parseCommunityEntries reads the inner entry list for one community-match.
func parseCommunityEntries(listName string, listMap map[string]any) ([]communityEntry, error) {
	rawEntries, ok := listMap["entry"]
	if !ok {
		return nil, nil
	}

	if entriesSlice, ok := rawEntries.([]any); ok {
		out := make([]communityEntry, 0, len(entriesSlice))
		for _, item := range entriesSlice {
			entryMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("community-match %q: entry is not a map", listName)
			}
			e, err := parseOneCommunityEntry(listName, entryMap)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	}

	if entriesMap, ok := rawEntries.(map[string]any); ok {
		if len(entriesMap) > 1 {
			return nil, fmt.Errorf("community-match %q: %d entries in map form would lose order (first-match-wins requires slice form)", listName, len(entriesMap))
		}
		out := make([]communityEntry, 0, len(entriesMap))
		for keyCommunity, item := range entriesMap {
			entryMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("community-match %q entry %q: not a map", listName, keyCommunity)
			}
			if _, has := entryMap["community"]; !has {
				entryMap["community"] = keyCommunity
			}
			e, err := parseOneCommunityEntry(listName, entryMap)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	}

	return nil, fmt.Errorf("community-match %q: unexpected entry container type %T", listName, rawEntries)
}

// parseOneCommunityEntry reads a single entry's leaves.
func parseOneCommunityEntry(listName string, m map[string]any) (communityEntry, error) {
	communityStr, ok := m["community"].(string)
	if !ok || communityStr == "" {
		return communityEntry{}, fmt.Errorf("community-match %q: entry missing community leaf", listName)
	}
	if len(communityStr) > maxCommunityLen {
		return communityEntry{}, fmt.Errorf("community-match %q: community %q exceeds maximum length %d", listName, communityStr, maxCommunityLen)
	}

	ctype, err := parseCommunityType(listName, communityStr, m["type"])
	if err != nil {
		return communityEntry{}, err
	}

	act, err := parseCommunityAction(listName, communityStr, m["action"])
	if err != nil {
		return communityEntry{}, err
	}

	return communityEntry{
		community: communityStr,
		ctype:     ctype,
		action:    act,
	}, nil
}

// parseCommunityType validates the type leaf. Defaults to standard.
func parseCommunityType(listName, communityStr string, raw any) (communityType, error) {
	if raw == nil {
		return communityStandard, nil
	}
	s, ok := raw.(string)
	if !ok {
		return communityStandard, fmt.Errorf("community-match %q entry %q: type is not a string", listName, communityStr)
	}
	if s == "standard" {
		return communityStandard, nil
	}
	if s == "large" {
		return communityLarge, nil
	}
	if s == "extended" {
		return communityExtended, nil
	}
	return communityStandard, fmt.Errorf("community-match %q entry %q: invalid type %q", listName, communityStr, s)
}

// parseCommunityAction validates the action leaf. Defaults to accept.
func parseCommunityAction(listName, communityStr string, raw any) (action, error) {
	if raw == nil {
		return actionAccept, nil
	}
	s, ok := raw.(string)
	if !ok {
		return actionAccept, fmt.Errorf("community-match %q entry %q: action is not a string", listName, communityStr)
	}
	if s == "accept" {
		return actionAccept, nil
	}
	if s == "reject" {
		return actionReject, nil
	}
	return actionAccept, fmt.Errorf("community-match %q entry %q: invalid action %q", listName, communityStr, s)
}
