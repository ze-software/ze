// VALIDATES: set form writes a multi-leaf-key entry under its whole address, and both SetParser paths read it back apart.
// PREVENTS: a set-form save and replay merging two entries for one peer on two interfaces into one.

package config

import (
	"slices"
	"strings"
	"testing"
)

// TestSetFormRoundTripKeepsTwoSessionsForOnePeer proves a set-form save of
// two entries for one peer on two interfaces replays into two entries.
// Method: parse brace form over a list keyed `peer vrf interface`, write set
// form, check every line names its entry's whole address and that no line
// sets `interface` on its own, then replay through Parse and ParseWithMeta.
func TestSetFormRoundTripKeepsTwoSessionsForOnePeer(t *testing.T) {
	schema := multiKeySchema()
	tree, err := NewParser(schema).Parse(`session fe80::b { interface zbl0; profile slow; }
session fe80::b { interface zbl1; profile fast; }
`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := SerializeSet(tree, schema)
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		zbl0 := strings.HasPrefix(line, "set session fe80::b interface zbl0")
		zbl1 := strings.HasPrefix(line, "set session fe80::b interface zbl1")
		if !zbl0 && !zbl1 {
			t.Errorf("line does not start with an entry's whole address: %q", line)
		}
		if strings.Count(line, " interface ") > 1 {
			t.Errorf("line sets interface on its own: %q", line)
		}
	}
	want := []string{"fe80::b interface zbl0", "fe80::b interface zbl1"}
	check := func(t *testing.T, replayed *Tree) {
		t.Helper()
		if got := sortedListKeys(replayed); !slices.Equal(got, want) {
			t.Fatalf("replayed keys %q, want %q\n%s", got, want, out)
		}
		entries := replayed.GetList("session")
		for key, profile := range map[string]string{want[0]: "slow", want[1]: "fast"} {
			if v, _ := entries[key].Get("profile"); v != profile {
				t.Errorf("%s: profile %q, want %q", key, v, profile)
			}
			iface := strings.Fields(key)[2]
			if v, _ := entries[key].Get("interface"); v != iface {
				t.Errorf("%s: interface leaf %q, want %q", key, v, iface)
			}
		}
	}

	t.Run("Parse", func(t *testing.T) {
		replayed, err := NewSetParser(schema).Parse(out)
		if err != nil {
			t.Fatalf("replay: %v\n%s", err, out)
		}
		check(t, replayed)
	})

	t.Run("ParseWithMeta", func(t *testing.T) {
		replayed, meta, err := NewSetParser(schema).ParseWithMeta(out)
		if err != nil {
			t.Fatalf("replay: %v\n%s", err, out)
		}
		check(t, replayed)
		metaList := meta.GetContainer("session")
		if metaList == nil {
			t.Fatal("meta tree holds no session list")
		}
		metaKeys := make([]string, 0, 2)
		for key := range metaList.Lists() {
			metaKeys = append(metaKeys, key)
		}
		slices.Sort(metaKeys)
		if !slices.Equal(metaKeys, want) {
			t.Errorf("meta keys %q, want %q", metaKeys, want)
		}
	})

	t.Run("explicit vrf default survives set form", func(t *testing.T) {
		explicit, err := NewParser(schema).Parse("session fe80::b { interface zbl0; vrf default; }\n")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		replayed, err := NewSetParser(schema).Parse(SerializeSet(explicit, schema))
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		entry := replayed.GetList("session")["fe80::b interface zbl0"]
		if entry == nil {
			t.Fatalf("replayed keys %q", sortedListKeys(replayed))
		}
		if v, ok := entry.Get("vrf"); !ok || v != "default" {
			t.Errorf("vrf %q present %v, want explicit default", v, ok)
		}
	})

	t.Run("materialized pending delete prints under the full address", func(t *testing.T) {
		content := "#alice @ssh %2026-10-10T01:02:03Z ^fast delete session fe80::b interface zbl1 profile\n"
		changeTree, changeMeta, _, err := ParseChangeFile(content, NewSetParser(schema))
		if err != nil {
			t.Fatalf("ParseChangeFile: %v", err)
		}
		entry := changeTree.GetList("session")["fe80::b interface zbl1"]
		if entry == nil {
			t.Fatalf("materialized keys %q, want the entry fe80::b interface zbl1", sortedListKeys(changeTree))
		}
		if value, _ := entry.Get("interface"); value != "zbl1" {
			t.Errorf("materialized entry interface = %q, want zbl1 (the creator writes the key leaves)", value)
		}
		printed := SerializeSetWithMeta(changeTree, changeMeta, schema)
		if !strings.Contains(printed, "delete session fe80::b interface zbl1 profile") {
			t.Errorf("pending delete not printed under the full address:\n%s", printed)
		}
		for line := range strings.SplitSeq(printed, "\n") {
			if strings.Contains(line, "session fe80::b") && !strings.Contains(line, "session fe80::b interface zbl1") {
				t.Errorf("line names the entry by its first value only: %q", line)
			}
		}
	})

	t.Run("delete of one entry by its address", func(t *testing.T) {
		replayed, err := NewSetParser(schema).Parse(out + "delete session fe80::b interface zbl0\n")
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		if got := sortedListKeys(replayed); !slices.Equal(got, want[1:]) {
			t.Errorf("keys after delete %q, want %q", got, want[1:])
		}
	})
}
