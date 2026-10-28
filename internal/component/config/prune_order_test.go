package config

import (
	"slices"
	"strings"
	"testing"
)

// TestPruneUnchangedListOrder compares both directions of nested keyed lists,
// proving plain order is presentation while ordered-by-user order is behavior.
func TestPruneUnchangedListOrder(t *testing.T) {
	for _, kind := range []string{"list", "positional", "flex", "inline", "ordered"} {
		t.Run(kind, func(t *testing.T) {
			field := Field("value", Leaf(TypeString))
			var node Node
			switch kind {
			case "list", "positional", "ordered":
				list := List(TypeString, field)
				list.UserOrdered = kind == "ordered"
				if kind == "list" {
					list.KeyName = "name"
				}
				node = list
			case "flex":
				node = Flex(field)
			case "inline":
				node = InlineList(TypeString, field)
			}
			schema := NewSchema()
			schema.Define("outer", Container(Field("scope", List(TypeString, Field("entries", node)))))
			makeTree := func(keys ...string) *Tree {
				root, outer, scope := NewTree(), NewTree(), NewTree()
				root.SetContainer("outer", outer)
				outer.AddListEntry("scope", "one", scope)
				for _, key := range keys {
					entry := NewTree()
					entry.Set("value", key)
					scope.AddListEntry("entries", key, entry)
				}
				return root
			}
			before, after := makeTree("zulu", "alpha"), makeTree("alpha", "zulu")
			for _, pair := range [][2]*Tree{{before, after}, {after, before}} {
				pruned := pair[0].Clone()
				PruneUnchanged(pruned, pair[1], schema)
				out := Serialize(pruned, schema)
				if kind == "ordered" {
					if !strings.Contains(out, "zulu") || !strings.Contains(out, "alpha") {
						t.Fatalf("ordered list lost its order context: %q", out)
					}
				} else if strings.TrimSpace(out) != "" {
					t.Fatalf("plain reorder left configuration or an empty ancestor: %q", out)
				}
			}
			keys := before.GetContainer("outer").GetList("scope")["one"].GetListOrdered("entries")
			if len(keys) != 2 || keys[0].Key != "zulu" || keys[1].Key != "alpha" {
				t.Fatalf("comparison mutated the original order: %v", keys)
			}
		})
	}
}

// TestPruneUnchangedStructuralChanges keeps intentional empty containers and
// inactive transitions, but drops ancestors emptied only by pruning deletions.
func TestPruneUnchangedStructuralChanges(t *testing.T) {
	presence := Container(Field("value", Leaf(TypeString)))
	presence.Presence = true
	schema := NewSchema()
	schema.Define("outer", Container(Field("presence", presence), Field("value", Leaf(TypeString))))
	for _, tc := range []struct {
		name, before, after string
		want                []string
	}{
		{"new empty", "outer {}", "outer { presence {} }", []string{"outer", "presence"}},
		{"intentionally emptied", "outer { presence { value old; } }", "outer { presence {} }", []string{"presence"}},
		{"inactive container", "outer { value same; }", "inactive: outer { value same; }", []string{"inactive:", "outer"}},
		{"inactive presence", "outer { presence { value same; } }", "outer { inactive: presence { value same; } }", []string{"inactive:", "presence"}},
		{"deletion only", "outer { presence { value old; } value same; }", "outer { value same; }", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := parseTree(t, schema, tc.before)
			after := parseTree(t, schema, tc.after)
			PruneUnchanged(after, before, schema)
			out := Serialize(after, schema)
			if tc.want == nil && strings.TrimSpace(out) != "" {
				t.Fatalf("pruning left empty ancestors: %q", out)
			}
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("lost %q in %q", want, out)
				}
			}
		})
	}
}

// TestPruneUnchangedOrderedInsertion keeps a new entry without misclassifying
// the unchanged common-entry sequence as a reorder.
func TestPruneUnchangedOrderedInsertion(t *testing.T) {
	list := List(TypeString, Field("value", Leaf(TypeString)))
	list.UserOrdered = true
	schema := NewSchema()
	schema.Define("entry", list)
	before := parseTree(t, schema, "entry a { value a; } entry c { value c; }")
	after := parseTree(t, schema, "entry a { value a; } entry b { value b; } entry c { value c; }")
	PruneUnchanged(after, before, schema)
	if keys := listKeys(after, "entry"); !slices.Equal(keys, []string{"b"}) {
		t.Fatalf("insertion retained unchanged entries: %v", keys)
	}
}

// TestPruneUnchangedListState keeps inactive entry transitions and real nested
// edits while pruning an unrelated plain-list reorder in the same entry.
func TestPruneUnchangedListState(t *testing.T) {
	nested := List(TypeString, Field("value", Leaf(TypeString)))
	entry := List(TypeString, Field("value", Leaf(TypeString)), Field("nested", nested))
	schema := NewSchema()
	schema.Define("entry", entry)
	before := parseTree(t, schema, `entry one {
		value old;
		nested z { value same; }
		nested a { value same; }
	}`)
	for _, tc := range []struct {
		name, text, want string
	}{
		{"inactive", `inactive: entry one {
			value old;
			nested a { value same; }
			nested z { value same; }
		}`, "inactive: entry one"},
		{"changed leaf", `entry one {
			value new;
			nested a { value same; }
			nested z { value same; }
		}`, "new"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			after := parseTree(t, schema, tc.text)
			PruneUnchanged(after, before, schema)
			out := Serialize(after, schema)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("lost real change %q: %q", tc.want, out)
			}
			if strings.Contains(out, "nested") {
				t.Fatalf("kept nested plain reorder: %q", out)
			}
		})
	}
}

// TestPruneUnchangedReplayRepresentations compares hierarchy with both saved
// forms, so internal scalar/member storage cannot invent a configuration edit.
func TestPruneUnchangedReplayRepresentations(t *testing.T) {
	schema := NewSchema()
	schema.Define("scope", Container(
		Field("action", Flex()),
		Field("members", &BracketLeafListNode{Type: TypeString}),
		Field("array", &ValueOrArrayNode{Type: TypeString}),
	))
	for _, input := range []string{
		"scope { action single; }",
		`scope { action "two words"; }`,
		"scope { action first; action second; action first; }",
		"scope { members [ a b ]; inactive: members b; array [ c d ]; }",
	} {
		original := parseTree(t, schema, input)
		for _, reader := range setFormParsers {
			t.Run(reader.name+"/"+input, func(t *testing.T) {
				replayed := reader.parse(t, schema, SerializeSet(original, schema))
				for _, pair := range [][2]*Tree{{original, replayed}, {replayed, original}} {
					pruned := pair[0].Clone()
					PruneUnchanged(pruned, pair[1], schema)
					if got := strings.TrimSpace(Serialize(pruned, schema)); got != "" {
						t.Fatalf("equivalent replay left config: %s", got)
					}
				}
			})
		}
	}
}

// TestPruneUnchangedPrinterSemantics preserves canonical scalar spelling and
// ephemeral omission while keeping changes to hidden persisted configuration.
func TestPruneUnchangedPrinterSemantics(t *testing.T) {
	ephemeral := Leaf(TypeString)
	ephemeral.Ephemeral = true
	hidden := Leaf(TypeString)
	hidden.Hidden = true
	schema := NewSchema()
	schema.Define("scope", Container(
		Field("flag", Leaf(TypeBool)), Field("temporary", ephemeral), Field("hidden", hidden),
	))
	for _, tc := range []struct {
		name, field, old, new string
		changed               bool
	}{
		{"canonical boolean", "flag", "true", "enable", false},
		{"ephemeral", "temporary", "before", "after", false},
		{"hidden persists", "hidden", "before", "after", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, after := NewTree(), NewTree()
			oldScope, newScope := NewTree(), NewTree()
			oldScope.Set(tc.field, tc.old)
			newScope.Set(tc.field, tc.new)
			before.SetContainer("scope", oldScope)
			after.SetContainer("scope", newScope)
			if equal := SubtreesEqual(oldScope, newScope, schema.Get("scope")); equal == tc.changed {
				t.Errorf("semantic equality = %v, changed = %v", equal, tc.changed)
			}
			PruneUnchanged(after, before, schema)
			got := strings.TrimSpace(Serialize(after, schema))
			if (got != "") != tc.changed {
				t.Fatalf("pruned output %q, changed = %v", got, tc.changed)
			}
		})
	}
}

// TestSubtreesEqualMemberStores ignores the optional scalar mirror when both
// trees carry the same authoritative members, while preserving inactive edits.
func TestSubtreesEqualMemberStores(t *testing.T) {
	schema := NewSchema()
	schema.Define("members", &BracketLeafListNode{Type: TypeString})
	before := parseTree(t, schema, "members [ a b ];")
	after := NewTree()
	after.AppendValue("members", "a")
	after.AppendValue("members", "b")
	if !SubtreesEqual(before, after, schema.root) {
		t.Fatal("a missing derived scalar mirror changed semantic equality")
	}
	pruned := after.Clone()
	PruneUnchanged(pruned, before, schema)
	if got := Serialize(pruned, schema); strings.TrimSpace(got) != "" {
		t.Fatalf("mirror-only difference survived pruning: %q", got)
	}
	if err := after.DeactivateMultiValue("members", "b"); err != nil {
		t.Fatal(err)
	}
	if SubtreesEqual(before, after, schema.root) {
		t.Fatal("member deactivation was mistaken for a mirror-only difference")
	}
}

// TestSubtreesEqualFreeformSpelling preserves the freeform writer's existing
// bracket normalization without equating a flag with a literal true member.
func TestSubtreesEqualFreeformSpelling(t *testing.T) {
	for _, tc := range []struct {
		old, new string
		equal    bool
	}{
		{"a b", "[ a b ]", true},
		{"true", "[ true ]", false},
		{"", "[  ]", true},
		{"[ ]", "[ [ ] ]", false},
		{"[", "[ [ ]", true},
	} {
		before, after := NewTree(), NewTree()
		before.Set("entry", tc.old)
		after.Set("entry", tc.new)
		if got := SubtreesEqual(before, after, Freeform()); got != tc.equal {
			t.Fatalf("%q vs %q: equal=%v, want %v", tc.old, tc.new, got, tc.equal)
		}
	}
}
