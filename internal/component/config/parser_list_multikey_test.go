// VALIDATES: a multi-leaf-key list entry is stored and compared under its canonical address.
// PREVENTS: one peer on two interfaces refused as a duplicate list key, or two spellings of one address stored apart.

package config

import (
	"slices"
	"strings"
	"testing"
)

// multiKeySchema holds one list whose YANG key names three leaves, the shape
// of the BFD `single-hop-session` list: `key "peer vrf interface"`, with
// `vrf` defaulting to `default` and `interface` carrying no default.
func multiKeySchema() *Schema {
	schema := NewSchema()
	session := List(TypeString,
		Field("peer", Leaf(TypeString)),
		Field("vrf", LeafWithDefault(TypeString, "default")),
		Field("interface", Leaf(TypeString)),
		Field("profile", Leaf(TypeString)),
	)
	session.KeyName = "peer vrf interface"
	schema.Define("session", session)
	return schema
}

// sortedListKeys returns the map keys of multiKeySchema's `session` list, sorted.
func sortedListKeys(tree *Tree) []string {
	keys := make([]string, 0, len(tree.GetList("session")))
	for key := range tree.GetList("session") {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// TestMultiLeafKeyAbsentLeafComparesAsItsDefault proves the config file
// parser stores a multi-leaf-key entry under its canonical address and
// compares whole addresses for duplicates. Method: parse configs over a list
// keyed `peer vrf interface` and read the stored map keys or the refusal.
func TestMultiLeafKeyAbsentLeafComparesAsItsDefault(t *testing.T) {
	schema := multiKeySchema()

	t.Run("two interfaces are two entries", func(t *testing.T) {
		tree, err := NewParser(schema).Parse(`session fe80::b { interface zbl0; }
session fe80::b { interface zbl1; profile fast; }
`)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		want := []string{"fe80::b interface zbl0", "fe80::b interface zbl1"}
		if got := sortedListKeys(tree); !slices.Equal(got, want) {
			t.Fatalf("keys %q, want %q", got, want)
		}
		if v, _ := tree.GetList("session")["fe80::b interface zbl1"].Get("profile"); v != "fast" {
			t.Errorf("zbl1 entry profile %q, want fast", v)
		}
	})

	t.Run("vrf absent and vrf default are one entry", func(t *testing.T) {
		_, err := NewParser(schema).Parse(`session fe80::b { interface zbl0; }
session fe80::b { interface zbl0; vrf default; }
`)
		if err == nil {
			t.Fatal("vrf absent and vrf default accepted as two entries")
		}
		if !strings.Contains(err.Error(), "duplicate list key for session: fe80::b interface zbl0") {
			t.Errorf("error %q is not the duplicate refusal for the canonical key", err)
		}
	})

	t.Run("interface absent differs from interface zbl0", func(t *testing.T) {
		tree, err := NewParser(schema).Parse(`session fe80::b { profile p; }
session fe80::b { interface zbl0; }
`)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		want := []string{"fe80::b", "fe80::b interface zbl0"}
		if got := sortedListKeys(tree); !slices.Equal(got, want) {
			t.Fatalf("keys %q, want %q", got, want)
		}
	})

	t.Run("vrf other than default is in the address", func(t *testing.T) {
		tree, err := NewParser(schema).Parse("session fe80::b { vrf blue; interface zbl0; }\n")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		want := []string{"fe80::b vrf blue interface zbl0"}
		if got := sortedListKeys(tree); !slices.Equal(got, want) {
			t.Fatalf("keys %q, want %q (YANG key order)", got, want)
		}
	})

	refusals := []struct {
		name, text, leaf string
	}{
		{"whitespace in a key value", "session fe80::b { interface \"zbl 0\"; }\n", "interface"},
		{"empty vrf", "session fe80::b { interface zbl0; vrf \"\"; }\n", "vrf"},
		{"empty interface", "session fe80::b { interface \"\"; }\n", "interface"},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewParser(schema).Parse(tc.text)
			if err == nil {
				t.Fatal("accepted")
			}
			msg := err.Error()
			for _, want := range []string{"session", tc.leaf, "cannot be empty or hold a space", "remove the " + tc.leaf + " leaf or give it a value"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not say %q", msg, want)
				}
			}
		})
	}

	t.Run("brace form writes the first value and keeps explicit vrf default", func(t *testing.T) {
		tree, err := NewParser(schema).Parse(`session fe80::b { interface zbl0; vrf default; }
session fe80::b { interface zbl1; }
`)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		out := Serialize(tree, schema)
		if strings.Contains(out, `"`) {
			t.Errorf("brace form quotes an address into one key:\n%s", out)
		}
		if !strings.Contains(out, "vrf default") {
			t.Errorf("explicit vrf default not printed back:\n%s", out)
		}
		again, err := NewParser(schema).Parse(out)
		if err != nil {
			t.Fatalf("re-parse of own output: %v\n%s", err, out)
		}
		want := []string{"fe80::b interface zbl0", "fe80::b interface zbl1"}
		if got := sortedListKeys(again); !slices.Equal(got, want) {
			t.Fatalf("re-parsed keys %q, want %q", got, want)
		}
	})

	// The SetParser route: a set line addresses an entry, so the two
	// spellings of one canonical address land on one entry rather than
	// being refused as a duplicate, and the refusals are the file parser's.
	t.Run("set form: vrf absent and vrf default address one entry", func(t *testing.T) {
		tree, err := NewSetParser(schema).Parse(`set session fe80::b interface zbl0 profile slow
set session fe80::b vrf default interface zbl0 profile fast
`)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		want := []string{"fe80::b interface zbl0"}
		if got := sortedListKeys(tree); !slices.Equal(got, want) {
			t.Fatalf("keys %q, want %q", got, want)
		}
		if v, _ := tree.GetList("session")[want[0]].Get("profile"); v != "fast" {
			t.Errorf("profile %q, want the second line's fast on the one entry", v)
		}
	})

	t.Run("set form: interface absent differs from interface zbl0", func(t *testing.T) {
		tree, err := NewSetParser(schema).Parse(`set session fe80::b profile p
set session fe80::b interface zbl0 profile q
`)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		want := []string{"fe80::b", "fe80::b interface zbl0"}
		if got := sortedListKeys(tree); !slices.Equal(got, want) {
			t.Fatalf("keys %q, want %q", got, want)
		}
	})

	setRefusals := []struct {
		name, text, leaf string
	}{
		{"set form: whitespace in a key value", "set session fe80::b interface \"zbl 0\" profile p\n", "interface"},
		{"set form: empty vrf", "set session fe80::b interface zbl0 vrf \"\" profile p\n", "vrf"},
		{"set form: empty interface", "set session fe80::b interface \"\" profile p\n", "interface"},
	}
	for _, tc := range setRefusals {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSetParser(schema).Parse(tc.text)
			if err == nil {
				t.Fatal("accepted")
			}
			msg := err.Error()
			for _, want := range []string{"session", tc.leaf, "cannot be empty or hold a space", "remove the " + tc.leaf + " leaf or give it a value"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not say %q", msg, want)
				}
			}
		})
	}
}

// TestParseListAddressReadsLeftToRight proves the address rule over path
// tokens: a key leaf name is consumed with the token after it, a trailing
// key leaf name is a leaf read, and a key leaf named twice is refused.
func TestParseListAddressReadsLeftToRight(t *testing.T) {
	node, ok := multiKeySchema().Get("session").(*ListNode)
	if !ok {
		t.Fatal("session is not a list")
	}
	cases := []struct {
		tokens   []string
		key      string
		consumed int
	}{
		{[]string{"fe80::b"}, "fe80::b", 1},
		{[]string{"fe80::b", "interface"}, "fe80::b", 1},
		{[]string{"fe80::b", "interface", "zbl1"}, "fe80::b interface zbl1", 3},
		{[]string{"fe80::b", "interface", "zbl1", "interface"}, "fe80::b interface zbl1", 3},
		{[]string{"fe80::b", "interface", "zbl1", "profile", "fast"}, "fe80::b interface zbl1", 3},
		{[]string{"fe80::b", "vrf", "default", "interface", "zbl1"}, "fe80::b interface zbl1", 5},
		{[]string{"fe80::b", "interface", "interface"}, "fe80::b interface interface", 3},
	}
	for _, tc := range cases {
		addr, err := ParseListAddress("session", node, tc.tokens)
		if err != nil {
			t.Errorf("%q: %v", tc.tokens, err)
			continue
		}
		if addr.Key != tc.key || addr.Tokens != tc.consumed {
			t.Errorf("%q: key %q consumed %d, want %q consumed %d", tc.tokens, addr.Key, addr.Tokens, tc.key, tc.consumed)
		}
	}
	_, err := ParseListAddress("session", node, []string{"fe80::b", "interface", "zbl1", "interface", "zbl2"})
	if err == nil || !strings.Contains(err.Error(), "interface is given twice") {
		t.Errorf("key leaf named twice: got %v, want the given-twice refusal", err)
	}
}
