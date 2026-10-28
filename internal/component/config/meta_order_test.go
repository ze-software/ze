// Design: docs/architecture/config/syntax.md -- metadata replay preserves tree order.

package config

import (
	"slices"
	"testing"
)

// TestSessionEntriesOrderedPreservesMetadata separates metadata insertion from
// tree order, including every list carrier, an unrecorded map entry, duplicate
// and stale order keys, a metadata-only deletion and another session's value.
func TestSessionEntriesOrderedPreservesMetadata(t *testing.T) {
	for _, list := range []string{"peer", "server", "vpls", "route", "term"} {
		t.Run(list, func(t *testing.T) {
			tree := NewTree()
			for _, key := range []string{"zulu", "alpha", "mike"} {
				tree.AddListEntry(list, key, NewTree())
			}
			tree.GetList(list)["bravo"] = NewTree()
			tree.listOrder[list] = append(tree.listOrder[list], "zulu", "removed")
			meta := NewMetaTree()
			for _, key := range []string{"mike", "bravo", "deleted", "alpha", "zulu"} {
				entry := MetaEntry{User: "alice", Value: key, Previous: "old"}
				if key == "deleted" {
					entry.Value = ""
				}
				child := meta.GetOrCreateContainer(list).GetOrCreateListEntry(key)
				child.SetEntry("value", entry)
				child.SetEntry("value", MetaEntry{User: "bob", Value: "other"})
			}
			entries := meta.SessionEntriesOrdered(listOrderSchema(), "alice", tree)
			want := []string{"zulu", "alpha", "mike", "bravo", "deleted"}
			if len(entries) != len(want) {
				t.Fatalf("entries = %v, want %d records", entries, len(want))
			}
			for i, entry := range entries {
				if !slices.Equal(entry.Path, []string{list, want[i], "value"}) {
					t.Fatalf("path[%d] = %v, want %s %s value", i, entry.Path, list, want[i])
				}
				value := want[i]
				if value == "deleted" {
					value = ""
				}
				if entry.Entry != (MetaEntry{User: "alice", Value: value, Previous: "old"}) {
					t.Fatalf("metadata[%d] = %+v, ownership or deletion changed", i, entry.Entry)
				}
			}
			if got := meta.SessionEntriesOrdered(listOrderSchema(), "bob", tree); len(got) != len(want) {
				t.Fatalf("other session lost records: %v", got)
			}
		})
	}
}

// TestSessionEntriesOrderedNestedMultiKey preserves both nesting and the codec's
// multi-leaf key boundaries, while retaining an absent container's tombstone.
func TestSessionEntriesOrderedNestedMultiKey(t *testing.T) {
	schema := NewSchema()
	session := List(TypeString,
		Field("peer", Leaf(TypeString)),
		Field("interface", Leaf(TypeString)),
		Field("profile", Leaf(TypeString)),
	)
	session.KeyName = "peer interface"
	schema.Define("routing", Container(Field("session", session)))
	input := "#alice set routing session zulu interface eth9 profile first\n" +
		"#alice set routing session alpha interface eth1 profile second\n" +
		"#alice set routing session zulu interface eth0 profile third\n"
	tree, meta, err := NewSetParser(schema).ParseWithMeta(input)
	if err != nil {
		t.Fatal(err)
	}
	entries := meta.SessionEntriesOrdered(schema, "alice", tree)
	want := [][]string{
		{"routing", "session", "zulu", "interface", "eth9", "profile"},
		{"routing", "session", "alpha", "interface", "eth1", "profile"},
		{"routing", "session", "zulu", "interface", "eth0", "profile"},
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %v, want %d records", entries, len(want))
	}
	for i := range want {
		if !slices.Equal(entries[i].Path, want[i]) {
			t.Fatalf("path[%d] = %v, want %v", i, entries[i].Path, want[i])
		}
	}
	meta.GetOrCreateContainer("absent").GetOrCreateContainer("list").GetOrCreateListEntry("gone").SetEntry("leaf", MetaEntry{User: "alice", Previous: "removed"})
	entries = meta.SessionEntriesOrdered(schema, "alice", tree)
	if len(entries) != len(want)+1 {
		t.Fatalf("metadata-only container lost: %v", entries)
	}
	found := false
	for _, entry := range entries {
		if slices.Equal(entry.Path, []string{"absent", "list", "gone", "leaf"}) {
			found = entry.Entry.Value == "" && entry.Entry.Previous == "removed"
		}
	}
	if !found {
		t.Fatal("metadata-only deletion was not preserved")
	}
}
