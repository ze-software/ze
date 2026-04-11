// Design: docs/architecture/core-design.md -- prefix-list filter config parsing
// Related: match.go -- prefix matching algorithm
//
// Config parsing for the bgp-filter-prefix plugin.
//
// Reads named prefix-list definitions out of the BGP config subtree:
//
//   bgp { policy { prefix-list NAME { entry P { ge G; le L; action A; } } } }
//
// Each list becomes a *prefixList with ordered entries. ge defaults to the
// prefix length of the entry; le defaults to 32 (IPv4) or 128 (IPv6); action
// defaults to accept (matches the YANG default).

package filter_prefix

import (
	"fmt"
	"net/netip"
)

// parsePrefixLists walks bgp { policy { prefix-list ... } } and returns a
// map of name -> *prefixList ready for runtime evaluation.
func parsePrefixLists(bgpCfg map[string]any) (map[string]*prefixList, error) {
	result := make(map[string]*prefixList)

	policyBlock, ok := bgpCfg["policy"].(map[string]any)
	if !ok {
		return result, nil
	}
	pflBlock, ok := policyBlock["prefix-list"].(map[string]any)
	if !ok {
		return result, nil
	}

	for name, raw := range pflBlock {
		listMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("prefix-list %q: not a map", name)
		}
		entries, err := parsePrefixListEntries(name, listMap)
		if err != nil {
			return nil, err
		}
		result[name] = &prefixList{name: name, entries: entries}
	}
	return result, nil
}

// parsePrefixListEntries reads the inner entry list for one prefix-list.
// Entries arrive as map[prefix]map[leaves...], which loses ordering. Order
// is recovered by walking the entries in the slice form when the config
// tree presents them as a list. The configjson layer presents YANG lists as
// either map (when keyed-by-key) or []any (when explicit list); both are
// handled here.
func parsePrefixListEntries(listName string, listMap map[string]any) ([]prefixEntry, error) {
	rawEntries, ok := listMap["entry"]
	if !ok {
		return nil, nil
	}

	if entriesSlice, ok := rawEntries.([]any); ok {
		out := make([]prefixEntry, 0, len(entriesSlice))
		for _, item := range entriesSlice {
			entryMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("prefix-list %q: entry is not a map", listName)
			}
			e, err := parseOneEntry(listName, entryMap)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	}

	if entriesMap, ok := rawEntries.(map[string]any); ok {
		// Keyed-by-prefix map. Order from a Go map is non-deterministic, so
		// callers that depend on order MUST present the list form. We still
		// support the map form for tests and round-trip configs.
		out := make([]prefixEntry, 0, len(entriesMap))
		for keyPrefix, item := range entriesMap {
			entryMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("prefix-list %q entry %q: not a map", listName, keyPrefix)
			}
			// If the entry map omits the prefix leaf, recover it from the key.
			if _, has := entryMap["prefix"]; !has {
				entryMap["prefix"] = keyPrefix
			}
			e, err := parseOneEntry(listName, entryMap)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	}

	return nil, fmt.Errorf("prefix-list %q: unexpected entry container type %T", listName, rawEntries)
}

// parseOneEntry reads a single entry's leaves into a prefixEntry, applying
// YANG defaults for missing ge/le/action.
func parseOneEntry(listName string, m map[string]any) (prefixEntry, error) {
	prefixStr, ok := m["prefix"].(string)
	if !ok || prefixStr == "" {
		return prefixEntry{}, fmt.Errorf("prefix-list %q: entry missing prefix leaf", listName)
	}
	pfx, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return prefixEntry{}, fmt.Errorf("prefix-list %q: invalid prefix %q: %w", listName, prefixStr, err)
	}

	ge := uint8(pfx.Bits())
	if v, ok := readUint(m["ge"]); ok {
		if v > 128 {
			return prefixEntry{}, fmt.Errorf("prefix-list %q entry %s: ge %d out of range", listName, prefixStr, v)
		}
		ge = uint8(v)
	}

	var le uint8
	if pfx.Addr().Is4() {
		le = 32
	} else {
		le = 128
	}
	if v, ok := readUint(m["le"]); ok {
		if v > 128 {
			return prefixEntry{}, fmt.Errorf("prefix-list %q entry %s: le %d out of range", listName, prefixStr, v)
		}
		le = uint8(v)
	}

	if ge > le {
		return prefixEntry{}, fmt.Errorf("prefix-list %q entry %s: ge %d > le %d", listName, prefixStr, ge, le)
	}

	act, err := parseAction(listName, prefixStr, m["action"])
	if err != nil {
		return prefixEntry{}, err
	}

	return prefixEntry{
		prefix: pfx,
		ge:     ge,
		le:     le,
		action: act,
	}, nil
}

// parseAction validates the action leaf, returning the YANG default (accept)
// when the leaf is absent and an error for any unknown value.
func parseAction(listName, prefixStr string, raw any) (action, error) {
	if raw == nil {
		return actionAccept, nil
	}
	s, ok := raw.(string)
	if !ok {
		return actionAccept, fmt.Errorf("prefix-list %q entry %s: action is not a string", listName, prefixStr)
	}
	if s == "accept" {
		return actionAccept, nil
	}
	if s == "reject" {
		return actionReject, nil
	}
	return actionAccept, fmt.Errorf("prefix-list %q entry %s: invalid action %q", listName, prefixStr, s)
}

// readUint coerces config values that may arrive as float64 (JSON), int,
// or string into a uint64. Returns ok=false if the value is missing or not
// a recognized numeric form.
func readUint(v any) (uint64, bool) {
	if n, ok := v.(float64); ok {
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	}
	if n, ok := v.(int); ok {
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	}
	if n, ok := v.(int64); ok {
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	}
	if n, ok := v.(uint64); ok {
		return n, true
	}
	if s, ok := v.(string); ok {
		var x uint64
		_, err := fmt.Sscanf(s, "%d", &x)
		if err != nil {
			return 0, false
		}
		return x, true
	}
	return 0, false
}
