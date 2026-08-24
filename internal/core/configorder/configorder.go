// Design: docs/architecture/config/syntax.md -- the entry order of a YANG list on the plugin config path
//
// Package configorder reads a YANG list declared `ordered-by user`, in the
// order the operator wrote its entries.
//
// A list reaches a plugin as a JSON object keyed by the list key, and a JSON
// object has no order once it is unmarshalled into a Go map. The config Tree
// does hold the order, so the plugin-facing lowering emits it beside the list
// under a reserved key, and this package is the only reader of that key.
//
// Use this package for a list whose evaluation depends on the order of its
// entries: a first-match-wins filter, a firewall chain, a failover server list.
// Use configvalue.ListEntries for every other list. Sorting a keyed list to
// make it deterministic is not a substitute. It is deterministic and wrong,
// because it replaces the operator's order with the alphabet.
package configorder

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// KeyPrefix marks the reserved key that carries a list's entry order. The order
// of a list named "entry" travels beside it under "@entry".
//
// No YANG node name can start with it, because a YANG identifier starts with a
// letter or an underscore (RFC 7950 Section 6.2). So the reserved key can never
// shadow a leaf, a container or a list declared in the same place.
// TestOrderKeyCannotCollideWithAYANGNodeName holds that property.
const KeyPrefix = "@"

// OrderKey returns the key that carries listName's entry order.
func OrderKey(listName string) string {
	var key textbuf.Buffer
	return key.Str(KeyPrefix).Str(listName).String()
}

// Entry is one entry of a YANG list: the value of its key leaf, and the leaves
// written inside its braces.
//
// Key is always set. The delivered map form keys the list by it and omits it
// from the entry, and the slice form carries it inside the entry, so a caller
// reads Key rather than reaching into Map for the key leaf.
type Entry struct {
	// Key is the value of the list's key leaf: a prefix, an AS-path regex, a
	// term name, a server name.
	Key string
	// Map holds the entry's other leaves. It is the delivered map rather than a
	// copy, so a caller MUST NOT write to it.
	Map map[string]any
}

// Entries returns the entries of container's listName list, in the order the
// operator wrote them. keyLeaf names the list's YANG key leaf, which is how the
// slice form spells the value the map form uses as its key.
//
// It returns no entries and no error when the list is absent or empty.
//
// It returns an error when the list holds two or more entries and no order was
// delivered with it. A first-match-wins list evaluated in an arbitrary order is
// a silent defect, so a missing order is refused rather than filled in.
func Entries(container map[string]any, listName, keyLeaf string) ([]Entry, error) {
	raw, ok := container[listName]
	if !ok || raw == nil {
		return nil, nil
	}

	if slice, ok := raw.([]any); ok {
		return entriesFromSlice(slice, listName, keyLeaf)
	}

	byKey, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: list is %T, want a map keyed by %s or a list of entries", listName, raw, keyLeaf)
	}
	return entriesFromMap(container, byKey, listName)
}

// entriesFromSlice reads the RFC 7951 shape: a list of entry objects, each
// carrying its own key leaf. No Ze lowering produces this shape. It is accepted
// because hand-built config sections and several plugin tests use it, and
// because its own order is already the operator's order.
func entriesFromSlice(slice []any, listName, keyLeaf string) ([]Entry, error) {
	entries := make([]Entry, 0, len(slice))
	for i, item := range slice {
		entryMap, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: entry %d is %T, want an object", listName, i, item)
		}
		key, ok := entryMap[keyLeaf].(string)
		if !ok || key == "" {
			return nil, fmt.Errorf("%s: entry %d has no %s", listName, i, keyLeaf)
		}
		entries = append(entries, Entry{Key: key, Map: entryMap})
	}
	return entries, nil
}

// entriesFromMap reads the delivered shape: an object keyed by the list key,
// with the order beside it under the reserved key.
func entriesFromMap(container, byKey map[string]any, listName string) ([]Entry, error) {
	if len(byKey) == 0 {
		return nil, nil
	}

	order, err := orderFor(container, byKey, listName)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(order))
	for _, key := range order {
		entryMap, ok := byKey[key].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: entry %q is %T, want an object", listName, key, byKey[key])
		}
		entries = append(entries, Entry{Key: key, Map: entryMap})
	}
	return entries, nil
}

// orderFor returns the delivered order of byKey's entries, after checking that
// it names every entry exactly once.
//
// One entry needs no order, so a single-entry list is answered whether or not
// the reserved key was delivered. That is what keeps the key out of the payload
// for the common case.
//
// A delivered order that disagrees with the list it orders can only come from a
// defect in the lowering, and either direction of the disagreement loses an
// entry: a name the list does not hold indexes nothing, and an entry the order
// does not name is never evaluated. Both are refused.
func orderFor(container, byKey map[string]any, listName string) ([]string, error) {
	rawOrder, delivered := container[OrderKey(listName)]
	if !delivered {
		if len(byKey) == 1 {
			for key := range byKey {
				return []string{key}, nil
			}
		}
		return nil, fmt.Errorf("%s: %d entries delivered with no order; a list evaluated in order must be lowered for a plugin with Tree.ToPluginMap", listName, len(byKey))
	}

	order, err := stringSlice(rawOrder, listName)
	if err != nil {
		return nil, err
	}
	if len(order) != len(byKey) {
		return nil, fmt.Errorf("%s: order names %d entries, the list holds %d", listName, len(order), len(byKey))
	}
	// The count alone does not prove the two agree: an order that names one
	// entry twice has the right length and leaves another entry unnamed, so it
	// would return a duplicate and drop an entry in silence.
	seen := make(map[string]bool, len(order))
	for _, key := range order {
		if _, ok := byKey[key]; !ok {
			return nil, fmt.Errorf("%s: order names %q, which the list does not hold", listName, key)
		}
		if seen[key] {
			return nil, fmt.Errorf("%s: order names %q twice", listName, key)
		}
		seen[key] = true
	}
	return order, nil
}

// stringSlice reads the two forms the order value takes. It is a []string in
// the same process, and a []any of strings once it has been through JSON.
func stringSlice(raw any, listName string) ([]string, error) {
	switch value := raw.(type) {
	case []string:
		return value, nil
	case []any:
		order := make([]string, 0, len(value))
		for i, item := range value {
			key, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s: order entry %d is %T, want a string", listName, i, item)
			}
			order = append(order, key)
		}
		return order, nil
	}
	return nil, fmt.Errorf("%s: order is %T, want a list of entry keys", listName, raw)
}
