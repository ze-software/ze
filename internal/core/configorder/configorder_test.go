package configorder

import (
	"encoding/json"
	"strings"
	"testing"
)

// twoEntryList builds the shape the plugin-facing lowering delivers: a list
// keyed by its key leaf, and the operator's order beside it. The two keys are
// deliberately in the opposite order to the alphabet, so a reader that sorts
// them cannot pass.
func twoEntryList() map[string]any {
	return map[string]any{
		"entry": map[string]any{
			"10.0.0.0/8": map[string]any{"action": "reject"},
			"0.0.0.0/0":  map[string]any{"action": "accept"},
		},
		OrderKey("entry"): []string{"10.0.0.0/8", "0.0.0.0/0"},
	}
}

// throughJSON marshals a container and reads it back, which is what the plugin
// process does to a config section. It turns the []string order into a []any of
// strings and re-sorts the object keys, so a test that runs a fixture through it
// exercises the shape a plugin actually holds.
func throughJSON(t *testing.T, container map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(container)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func keysOf(entries []Entry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func assertKeys(t *testing.T, entries []Entry, want ...string) {
	t.Helper()
	got := keysOf(entries)
	if len(got) != len(want) {
		t.Fatalf("got %d entries %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d is %q, want %q (full order %v)", i, got[i], want[i], got)
		}
	}
}

// TestEntriesFollowTheDeliveredOrder reads a two-entry list whose keys are in
// the opposite order to the alphabet, in the same process and after a JSON
// round trip.
//
// VALIDATES: the reader returns the operator's order, and reads both forms the
// order value takes: a []string in process, a []any of strings after JSON.
// PREVENTS: the failover and first-match-wins defects this package exists for.
// A reader that sorted the keyed map would return 0.0.0.0/0 first here, which
// in a prefix-list means the specific reject entry never runs.
func TestEntriesFollowTheDeliveredOrder(t *testing.T) {
	t.Run("in process", func(t *testing.T) {
		entries, err := Entries(twoEntryList(), "entry", "prefix")
		if err != nil {
			t.Fatalf("Entries: %v", err)
		}
		assertKeys(t, entries, "10.0.0.0/8", "0.0.0.0/0")
		if entries[0].Map["action"] != "reject" {
			t.Errorf("first entry carries %v, want the reject entry's leaves", entries[0].Map)
		}
	})

	t.Run("after JSON", func(t *testing.T) {
		entries, err := Entries(throughJSON(t, twoEntryList()), "entry", "prefix")
		if err != nil {
			t.Fatalf("Entries: %v", err)
		}
		assertKeys(t, entries, "10.0.0.0/8", "0.0.0.0/0")
	})
}

// TestEntriesRefuseAMultiEntryListWithNoOrder removes the order key and reads
// the same list.
//
// VALIDATES: two or more entries with no delivered order is an error naming the
// list, never a sorted or arbitrary answer.
// PREVENTS: a fallback. Sorting the map is what the L2TP RADIUS reader did, and
// it silently replaced the operator's failover order with a lexical one.
func TestEntriesRefuseAMultiEntryListWithNoOrder(t *testing.T) {
	container := twoEntryList()
	delete(container, OrderKey("entry"))

	entries, err := Entries(container, "entry", "prefix")
	if err == nil {
		t.Fatalf("Entries accepted an unordered multi-entry list, returning %v", keysOf(entries))
	}
	if !strings.Contains(err.Error(), "entry") {
		t.Errorf("error does not name the list: %v", err)
	}
}

// TestEntriesAcceptASingleEntryWithNoOrder reads a one-entry list with no order
// key.
//
// VALIDATES: one entry needs no order, so the lowering can leave the reserved
// key off the common case and the reader still answers.
// PREVENTS: a refusal for every single-entry list in the tree, which is most of
// them.
func TestEntriesAcceptASingleEntryWithNoOrder(t *testing.T) {
	container := map[string]any{
		"entry": map[string]any{"10.0.0.0/8": map[string]any{"action": "reject"}},
	}

	entries, err := Entries(container, "entry", "prefix")
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	assertKeys(t, entries, "10.0.0.0/8")
}

// TestEntriesAcceptASliceOfEntries reads the RFC 7951 array shape, where each
// entry carries its own key leaf.
//
// VALIDATES: the array's own order is the answer, and Key comes from inside the
// entry rather than from a map key.
// PREVENTS: dropping the shape that several plugin tests and every hand-built
// config section use. Those fixtures were the only multi-entry coverage the BGP
// filter readers had.
func TestEntriesAcceptASliceOfEntries(t *testing.T) {
	container := map[string]any{
		"entry": []any{
			map[string]any{"prefix": "10.0.0.0/8", "action": "reject"},
			map[string]any{"prefix": "0.0.0.0/0", "action": "accept"},
		},
	}

	entries, err := Entries(container, "entry", "prefix")
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	assertKeys(t, entries, "10.0.0.0/8", "0.0.0.0/0")
	if entries[1].Map["action"] != "accept" {
		t.Errorf("second entry carries %v, want the accept entry's leaves", entries[1].Map)
	}
}

// TestEntriesAbsentListIsNotAnError reads a container that declares no such
// list.
//
// VALIDATES: an absent list yields no entries and no error, so a caller can ask
// for an optional list without a presence check of its own.
// PREVENTS: every reader growing the same "is the key there" branch.
func TestEntriesAbsentListIsNotAnError(t *testing.T) {
	entries, err := Entries(map[string]any{"name": "ORDERED"}, "entry", "prefix")
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries for an absent list", len(entries))
	}
}

// TestEntriesToleratesAMalformedOrder feeds an order that disagrees with the
// list it orders, in both directions, plus an order of the wrong type.
//
// VALIDATES: a disagreement is an error naming the offending value, never a
// panic and never a silently short answer.
// PREVENTS: an index into a map that has no such key, and an entry that no
// order names being dropped without a word.
func TestEntriesToleratesAMalformedOrder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		order any
		want  string
	}{
		{"names an entry the list does not hold", []string{"10.0.0.0/8", "192.0.2.0/24"}, "does not hold"},
		{"names fewer entries than the list holds", []string{"10.0.0.0/8"}, "the list holds"},
		{"is not a list of keys", "10.0.0.0/8", "want a list of entry keys"},
		{"holds something that is not a key", []any{"10.0.0.0/8", 42}, "want a string"},
		// The right LENGTH and the right MEMBERS, and still wrong: naming one
		// entry twice leaves another unnamed, so a reader that only counts
		// returns a duplicate and drops an entry without saying so.
		{"names one entry twice", []string{"10.0.0.0/8", "10.0.0.0/8"}, "twice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			container := twoEntryList()
			container[OrderKey("entry")] = tc.order

			entries, err := Entries(container, "entry", "prefix")
			if err == nil {
				t.Fatalf("Entries accepted a malformed order, returning %v", keysOf(entries))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v does not say %q", err, tc.want)
			}
		})
	}
}

// TestEntriesRefuseAMalformedList feeds a list whose value is neither an object
// nor an array, and an entry that is not an object.
//
// VALIDATES: a shape the lowering cannot produce is an error naming the type it
// found.
// PREVENTS: a type assertion that yields a nil map and reads as an entry with no
// leaves.
func TestEntriesRefuseAMalformedList(t *testing.T) {
	t.Run("list is a scalar", func(t *testing.T) {
		if _, err := Entries(map[string]any{"entry": "10.0.0.0/8"}, "entry", "prefix"); err == nil {
			t.Fatal("Entries accepted a scalar as a list")
		}
	})

	t.Run("entry is a scalar", func(t *testing.T) {
		container := map[string]any{
			"entry":           map[string]any{"10.0.0.0/8": "reject", "0.0.0.0/0": map[string]any{}},
			OrderKey("entry"): []string{"10.0.0.0/8", "0.0.0.0/0"},
		}
		if _, err := Entries(container, "entry", "prefix"); err == nil {
			t.Fatal("Entries accepted a scalar as an entry")
		}
	})
}

// TestOrderKeyCannotCollideWithAYANGNodeName states the property the reserved
// key rests on.
//
// VALIDATES: the order key starts with a character no YANG identifier can start
// with, so it can never shadow a sibling leaf, container or list.
// PREVENTS: someone changing the prefix to a letter, which would make the key
// collide with a node name and would not fail any other test here.
func TestOrderKeyCannotCollideWithAYANGNodeName(t *testing.T) {
	key := OrderKey("entry")
	if key != "@entry" {
		t.Fatalf("OrderKey(entry) is %q, want @entry", key)
	}
	first := key[0]
	if first == '_' || (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') {
		t.Fatalf("order key starts with %q, which a YANG identifier can also start with", first)
	}
}
