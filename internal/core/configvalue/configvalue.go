// Design: docs/architecture/config/syntax.md -- the shapes a delivered config value takes
//
// Package configvalue reads a YANG leaf-list or a YANG list out of the config
// map that Tree.ToMap produces (internal/component/config/tree.go).
//
// It exists because that map does not hold what the node type suggests, and
// four readers guessed wrong. A leaf-list is not always a slice: ToMap omits it
// at zero active members, collapses it to a bare string at exactly one, and
// emits a slice only at two or more. A list is never a slice at any count: it
// is a map of list key to entry, and the key is not repeated inside the entry.
//
// Delivery adds one more shape. An in-process component receives the map from
// ToMap unchanged, so a multi-member leaf-list reaches it as a []string. A
// plugin receives the same map marshaled to JSON, so the same leaf-list
// reaches it as a []any. Both are the producer's, and both are read here.
//
// The failure this removes is silent. A reader that asserts []any on such a
// value gets a failed assertion rather than an error, so the operator's setting
// is discarded with no message anywhere: the option works when they write two
// values and vanishes when they write one.
package configvalue

import "sort"

// LeafList coerces a YANG leaf-list value into its members, in the order the
// producer emitted them. It accepts every shape Tree.ToMap and the JSON
// delivery after it can produce: a bare string for one member, a []string for
// several in process, and a []any for several over JSON.
//
// It returns nil for an absent value, an empty value, and a value of any other
// type, because every caller reads nil as "the operator configured nothing". A
// lone empty string is that case too: one empty member and an unset leaf are
// indistinguishable once ToMap has collapsed the leaf-list to a scalar.
//
// Every member of a slice is kept, the empty string included. A reader that
// rejects an empty member MUST do it in its own validation, where it can name
// the leaf: dropping the member here would be the silent discard this package
// exists to end, and it would answer differently on the two delivery paths.
// The returned slice is never aliased to the caller's.
//
// Safe for concurrent use.
func LeafList(v any) []string {
	switch list := v.(type) {
	case string:
		if list == "" {
			return nil
		}
		return []string{list}
	case []string:
		if len(list) == 0 {
			return nil
		}
		members := make([]string, len(list))
		copy(members, list)
		return members
	case []any:
		members := make([]string, 0, len(list))
		for _, item := range list {
			s, ok := item.(string)
			if !ok {
				continue
			}
			members = append(members, s)
		}
		if len(members) == 0 {
			return nil
		}
		return members
	}
	return nil
}

// ListEntry is one entry of a YANG list. Key is the value of the list key leaf,
// which Tree.ToMap uses as the map key and does not repeat inside Fields, so
// looking the key leaf up in Fields finds nothing.
type ListEntry struct {
	// Key is the value of the list's key leaf, which is the name the operator
	// wrote beside the list keyword.
	Key string
	// Fields holds the entry's other leaves. It is the delivered map rather
	// than a copy, so a caller MUST NOT write to it.
	Fields map[string]any
}

// ListEntries coerces a YANG list value into its entries, sorted by key so that
// two reads of one config produce one order. Tree.ToMap emits a list as a map
// of key to entry at every count, one included, so this reads that map and
// nothing else.
//
// It returns nil for an absent value and for a value of any other type. An
// entry whose body is not a map is skipped rather than reported, because
// Tree.ToMap cannot produce one.
//
// A list declared `ordered-by user` MUST NOT be read here, because sorting by
// key substitutes the alphabet for the order the operator wrote and a
// first-match-wins list then evaluates in an order nobody chose. Read such a
// list with configorder.Entries (internal/core/configorder), which carries the
// delivered order and refuses a multi-entry list that arrives without one.
//
// Safe for concurrent use.
func ListEntries(v any) []ListEntry {
	byKey, ok := v.(map[string]any)
	if !ok || len(byKey) == 0 {
		return nil
	}
	entries := make([]ListEntry, 0, len(byKey))
	for key, body := range byKey {
		fields, ok := body.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, ListEntry{Key: key, Fields: fields})
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries
}
