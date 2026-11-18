// Design: docs/architecture/config/syntax.md -- whole-leaf and member inactivity.
// Related: serialize_set.go -- plain and annotated set writers.

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlainWholeLeafInactiveSetRoundTrip starts with the hierarchical whole-leaf
// spelling, then saves and rereads set format. The whole marker and each
// member's independent activation intent must survive, including one member.
func TestPlainWholeLeafInactiveSetRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"one", `c { inactive: plain [ first ]; keep survivor; }`},
		{"many", `c { inactive: plain [ first second third ]; keep survivor; }`},
		{"quoted", `c { inactive: plain [ "first value" "bracket [ value ]" ]; keep survivor; }`},
		{"mixed member intent", `c { inactive: plain [ "first value" second third ]; inactive: plain second; keep survivor; }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema, original := plainWholeInactiveInput(t, tc.input)
			text := SerializeSet(original, schema)
			reread, err := NewSetParser(schema).Parse(text)
			require.NoError(t, err, "saved:\n%s", text)
			assertPlainWholeInactiveIntent(t, original, reread, schema, true, text)
		})
	}
}

// TestPlainWholeLeafInactiveOrderedSetRoundTrip keeps repeated members of an
// ordered plain leaf-list. A whole-list nop must not be replayed as individual
// member deactivations, which would collapse the repeated values.
func TestPlainWholeLeafInactiveOrderedSetRoundTrip(t *testing.T) {
	for _, annotated := range []bool{false, true} {
		name := "set"
		if annotated {
			name = "set-meta"
		}
		t.Run(name, func(t *testing.T) {
			schema := NewSchema()
			plain := valueOrArray(TypeString)
			plain.Ordered = true
			schema.Define("c", Container(Field("plain", plain), Field("keep", Leaf(TypeString))))
			original, err := NewParser(schema).Parse(`c { inactive: plain [ first first second ]; keep survivor; }`)
			require.NoError(t, err)
			require.Equal(t, []string{"first", "first", "second"}, original.GetContainer("c").GetSlice("plain"))
			text := SerializeSet(original, schema)
			if annotated {
				text = SerializeSetWithMeta(original, NewMetaTree(), schema)
			}
			reread, _, err := NewSetParser(schema).ParseWithMeta(text)
			require.NoError(t, err, "saved:\n%s", text)
			assertPlainWholeInactiveIntent(t, original, reread, schema, true, text)
		})
	}
}

// TestPlainWholeLeafNopRetainsSetAndInactiveIntent reads a hand-written bracket
// nop after an independently inactive member. An unordered plain list keeps set
// deduplication; an ordered list keeps repeats. Both retain the older marker.
func TestPlainWholeLeafNopRetainsSetAndInactiveIntent(t *testing.T) {
	for _, ordered := range []bool{false, true} {
		for _, annotated := range []bool{false, true} {
			name := "unordered/set"
			if ordered {
				name = "ordered/set"
			}
			if annotated {
				name += "-meta"
			}
			t.Run(name, func(t *testing.T) {
				plain := valueOrArray(TypeString)
				plain.Ordered = ordered
				schema := NewSchema()
				schema.Define("c", Container(Field("plain", plain)))
				input := "nop c plain older\nnop c plain [ first first second ]\n"
				var tree *Tree
				var err error
				if annotated {
					tree, _, err = NewSetParser(schema).ParseWithMeta(input)
				} else {
					tree, err = NewSetParser(schema).Parse(input)
				}
				require.NoError(t, err)
				child := tree.GetContainer("c")
				require.NotNil(t, child)
				assert.True(t, child.IsLeafInactive("plain"))
				want := []MemberState{{Value: "older", Inactive: true}, {Value: "first"}, {Value: "second"}}
				active := []string{"first", "second"}
				if ordered {
					want = []MemberState{{Value: "first"}, {Value: "first"}, {Value: "second"}, {Value: "older", Inactive: true}}
					active = []string{"first", "first", "second"}
				}
				assert.Equal(t, want, child.GetMultiValuesState("plain"))
				child.ClearLeafInactive("plain")
				assert.Equal(t, active, child.GetSlice("plain"))
			})
		}
	}
}

// TestPlainWholeLeafInactiveSetMetaRoundTrip covers every metadata writer route:
// ordinary annotations, session whole assignments and individual member entries.
// Metadata must survive without turning whole inactivity into member inactivity.
func TestPlainWholeLeafInactiveSetMetaRoundTrip(t *testing.T) {
	for _, mode := range []string{"empty", "committed", "session assignment", "member entries"} {
		t.Run(mode, func(t *testing.T) {
			schema, original := plainWholeInactiveInput(t,
				`c { inactive: plain [ "first value" second "bracket [ value ]" ]; inactive: plain second; keep survivor; }`)
			meta := NewMetaTree()
			entry := MetaEntry{
				User:     "thomas",
				Time:     time.Date(2026, 11, 17, 18, 0, 0, 0, time.UTC),
				Previous: "older value",
			}
			switch mode {
			case "empty":
				// Empty metadata must use the same state model as ordinary set output.
			case "committed":
				meta.GetOrCreateContainer("c").SetEntry("plain", entry)
			case "session assignment":
				entry.Source = "ssh"
				entry.Value = JoinLeafListMembers([]string{"first value", "second", "bracket [ value ]"})
				meta.GetOrCreateContainer("c").SetEntry("plain", entry)
			case "member entries":
				entry.Source = "ssh"
				for _, member := range original.GetContainer("c").GetMultiValuesState("plain") {
					owned := entry
					owned.Member = member.Value
					owned.Value = member.Value
					meta.GetOrCreateContainer("c").SetEntry("plain", owned)
				}
			}
			text := SerializeSetWithMeta(original, meta, schema)
			reread, restoredMeta, err := NewSetParser(schema).ParseWithMeta(text)
			require.NoError(t, err, "saved:\n%s", text)
			assertPlainWholeInactiveIntent(t, original, reread, schema, true, text)
			if mode == "empty" {
				return
			}
			containerMeta := restoredMeta.GetContainer("c")
			require.NotNil(t, containerMeta, "metadata disappeared:\n%s", text)
			entries := containerMeta.GetAllEntries("plain")
			require.NotEmpty(t, entries, "leaf metadata disappeared:\n%s", text)
			for _, restored := range entries {
				assert.Equal(t, entry.User, restored.User)
				assert.Equal(t, entry.Source, restored.Source)
				assert.Equal(t, entry.Time, restored.Time)
				assert.Equal(t, entry.Previous, restored.Previous)
			}
		})
	}
}

// TestPlainMemberOnlyInactiveSetRoundTrip is the permitted survivor: one or all
// members may be inactive while their parent leaf stays active. Quoted member
// boundaries and clearing only the leaf marker must preserve that distinction.
func TestPlainMemberOnlyInactiveSetRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"one inactive member", `c { plain [ first second ]; inactive: plain second; keep survivor; }`},
		{"all members inactive", `c { plain [ first second ]; inactive: plain first; inactive: plain second; keep survivor; }`},
		{"single quoted member", `c { inactive: plain "only member"; keep survivor; }`},
	} {
		for _, annotated := range []bool{false, true} {
			name := tc.name + "/set"
			if annotated {
				name += "-meta"
			}
			t.Run(name, func(t *testing.T) {
				schema, original := plainWholeInactiveInput(t, tc.input)
				text := SerializeSet(original, schema)
				if annotated {
					text = SerializeSetWithMeta(original, NewMetaTree(), schema)
				}
				reread, _, err := NewSetParser(schema).ParseWithMeta(text)
				require.NoError(t, err, "saved:\n%s", text)
				assertPlainWholeInactiveIntent(t, original, reread, schema, false, text)
			})
		}
	}
}

func plainWholeInactiveInput(t *testing.T, input string) (*Schema, *Tree) {
	t.Helper()
	schema := NewSchema()
	schema.Define("c", Container(
		Field("plain", valueOrArray(TypeString)),
		Field("keep", Leaf(TypeString)),
	))
	tree, err := NewParser(schema).Parse(input)
	require.NoError(t, err)
	return schema, tree
}

// assertPlainWholeInactiveIntent judges parsed state, then clears only whole-leaf
// inactivity and prunes. Members with no independent inactive marker must become
// effective again; independently deactivated members must remain ineffective.
func assertPlainWholeInactiveIntent(t *testing.T, original, reread *Tree, schema *Schema, whole bool, text string) {
	t.Helper()
	before := original.GetContainer("c")
	after := reread.GetContainer("c")
	require.NotNil(t, before)
	require.NotNil(t, after, "saved:\n%s", text)
	require.Equal(t, whole, before.IsLeafInactive("plain"), "input did not establish the intended control")
	assert.Equal(t, whole, after.IsLeafInactive("plain"), "whole-leaf state changed:\n%s", text)
	assert.Equal(t, before.GetMultiValuesState("plain"), after.GetMultiValuesState("plain"),
		"whole-leaf and member intent are distinct:\n%s", text)
	kept, ok := after.Get("keep")
	require.True(t, ok)
	assert.Equal(t, "survivor", kept)

	active := reread.Clone()
	active.GetContainer("c").ClearLeafInactive("plain")
	PruneInactive(active, schema)
	assert.Equal(t, before.GetSlice("plain"), active.GetContainer("c").GetSlice("plain"),
		"reactivating the leaf changed independent member intent:\n%s", text)
}
