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
//
// The entry order is the operator's. It is delivered beside the list by
// Tree.ToPluginMap and read by internal/core/configorder, because a Go map has
// no order and first-match-wins depends on one.

package filter_prefix

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/configorder"
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

// parsePrefixListEntries reads the inner entry list for one prefix-list, in
// the order the operator wrote the entries.
//
// The `entry` list is `ordered-by user` (yang/ze-filter-prefix.yang) because
// evaluation is first-match-wins, so configorder.Entries is the reader.
// configvalue.ListEntries sorts by key, which would swap a reject entry and the
// catch-all below it without a word.
//
// A list of two or more entries delivered with no order is still refused, by
// configorder rather than here. That refusal is the guard that made this defect
// loud instead of silent, and it stays.
func parsePrefixListEntries(listName string, listMap map[string]any) ([]prefixEntry, error) {
	entries, err := configorder.Entries(listMap, "entry", "prefix")
	if err != nil {
		return nil, fmt.Errorf("prefix-list %q: %w", listName, err)
	}

	out := make([]prefixEntry, 0, len(entries))
	for _, entry := range entries {
		parsed, err := parseOneEntry(listName, entry.Key, entry.Map)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

// parseOneEntry reads a single entry's leaves into a prefixEntry, applying
// YANG defaults for missing ge/le/action.
//
// prefixStr is the entry's key. It arrives as an argument because the delivered
// map form keys the list by the prefix and omits the leaf from the entry, so
// there is nothing in m to read it from (configorder.Entry).
func parseOneEntry(listName, prefixStr string, m map[string]any) (prefixEntry, error) {
	if prefixStr == "" {
		return prefixEntry{}, fmt.Errorf("prefix-list %q: entry missing prefix leaf", listName)
	}
	pfx, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return prefixEntry{}, fmt.Errorf("prefix-list %q: invalid prefix %q: %w", listName, prefixStr, err)
	}

	// Per-family maximum prefix length: 32 for IPv4, 128 for IPv6.
	// ge/le must fall within this range; an IPv4 entry with ge 48 is
	// nonsensical and silently matches nothing at runtime, so reject it
	// at parse time.
	var familyMax uint8
	if pfx.Addr().Is4() {
		familyMax = 32
	} else {
		familyMax = 128
	}

	ge := uint8(pfx.Bits())
	if v, ok := readUint(m["ge"]); ok {
		if v > uint64(familyMax) {
			return prefixEntry{}, fmt.Errorf("prefix-list %q entry %s: ge %d exceeds family max %d", listName, prefixStr, v, familyMax)
		}
		ge = uint8(v)
	}

	le := familyMax
	if v, ok := readUint(m["le"]); ok {
		if v > uint64(familyMax) {
			return prefixEntry{}, fmt.Errorf("prefix-list %q entry %s: le %d exceeds family max %d", listName, prefixStr, v, familyMax)
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
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case uint64:
		return n, true
	case string:
		var x uint64
		if _, err := fmt.Sscanf(n, "%d", &x); err != nil {
			return 0, false
		}
		return x, true
	}
	return 0, false
}
