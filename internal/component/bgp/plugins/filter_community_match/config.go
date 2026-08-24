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

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/configorder"
)

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

// parseCommunityEntries reads the inner entry list for one community-match, in the
// order the operator wrote the entries.
//
// The `entry` list is `ordered-by user` (yang/ze-filter-community-match.yang) because evaluation is
// first-match-wins, so configorder.Entries is the reader. configvalue.ListEntries
// sorts by key, which would silently reorder the entries a filter decision
// depends on.
//
// A list of two or more entries delivered with no order is still refused, by
// configorder rather than here. That refusal is the guard that made this defect
// loud instead of silent, and it stays.
func parseCommunityEntries(listName string, listMap map[string]any) ([]communityEntry, error) {
	entries, err := configorder.Entries(listMap, "entry", "community")
	if err != nil {
		return nil, fmt.Errorf("community-match %q: %w", listName, err)
	}

	out := make([]communityEntry, 0, len(entries))
	for _, entry := range entries {
		parsed, err := parseOneCommunityEntry(listName, entry.Key, entry.Map)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

// parseOneCommunityEntry reads a single entry's leaves.
//
// communityStr is the entry's key. It arrives as an argument because the
// delivered map form keys the list by the community and omits the leaf from the
// entry, so there is nothing in m to read it from (configorder.Entry).
func parseOneCommunityEntry(listName, communityStr string, m map[string]any) (communityEntry, error) {
	if communityStr == "" {
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
