// Design: docs/architecture/config/change-file-structural-ops.md -- saved entry positions.

package config

import (
	"slices"
	"testing"
)

// TestRepositionListEntriesPreservesUnselectedEntries checks that restoring
// several copied entries follows candidate order, not operation order, while
// retaining every unrelated entry's relative position and exact subtree pointer.
func TestRepositionListEntriesPreservesUnselectedEntries(t *testing.T) {
	tree := NewTree()
	original := map[string]*Tree{}
	for _, key := range []string{"foreign-first", "zulu", "foreign-middle", "alpha", "mike", "yankee", "xray"} {
		entry := NewTree()
		entry.Set("description", key)
		original[key] = entry
		tree.AddListEntry("peer", key, entry)
	}
	source := NewTree()
	for _, key := range []string{"zulu", "xray", "yankee", "alpha", "mike"} {
		source.AddListEntry("peer", key, NewTree())
	}
	tree.RepositionListEntries("peer", []string{"yankee", "xray", "yankee"}, source)
	want := []string{"foreign-first", "zulu", "xray", "yankee", "foreign-middle", "alpha", "mike"}
	if !slices.Equal(tree.listOrder["peer"], want) {
		t.Fatalf("repositioned order = %v, want %v", tree.listOrder["peer"], want)
	}
	for key, entry := range original {
		if tree.GetList("peer")[key] != entry {
			t.Fatalf("payload pointer of %s changed", key)
		}
		if got, _ := entry.Get("description"); got != key {
			t.Fatalf("payload of %s changed to %q", key, got)
		}
	}
}

// TestRepositionListEntriesFirstAndAbsent keeps unselected keys and keys missing
// from either recorded order intact, including a legitimate empty string key.
func TestRepositionListEntriesFirstAndAbsent(t *testing.T) {
	tree := NewTree()
	for _, key := range []string{"foreign", "", "alpha", "last", "renamed"} {
		tree.AddListEntry("peer", key, NewTree())
	}
	source := NewTree()
	for _, key := range []string{"renamed", "missing", "", "alpha"} {
		source.AddListEntry("peer", key, NewTree())
	}
	tree.RepositionListEntries("peer", []string{"renamed", "last", "missing"}, source)
	want := []string{"renamed", "foreign", "", "alpha", "last"}
	if !slices.Equal(tree.listOrder["peer"], want) {
		t.Fatalf("repositioned order = %v, want %v", tree.listOrder["peer"], want)
	}
	tree.RepositionListEntries("peer", []string{"renamed"}, tree)
	if !slices.Equal(tree.listOrder["peer"], want) {
		t.Fatalf("self-positioning changed order: %v", tree.listOrder["peer"])
	}
}
