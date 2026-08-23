package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToMapLeafListMemberCountShapes parses real config text through the schema
// and asserts the Go type ToMap emits for a leaf-list at each member count.
//
// VALIDATES: a leaf-list is absent from the map at zero active members, a bare
// string at exactly one, and a []string at two or more.
// PREVENTS: the assumption every leaf-list reader is written against going
// stale without a test noticing. Four readers asserted []any on such a value,
// which succeeds at two members and fails at one, so the operator's option
// worked when they wrote two values and vanished when they wrote one. The one
// member row is the row that discriminates: internal/core/configvalue is
// written against exactly these three shapes.
func TestToMapLeafListMemberCountShapes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  any
	}{
		{"one member is a bare string", "name-server 9.9.9.9;\n", "9.9.9.9"},
		{"two members are a []string", "name-server 9.9.9.9;\nname-server 8.8.8.8;\n", []string{"9.9.9.9", "8.8.8.8"}},
		{"three members are a []string", "name-server [ 9.9.9.9 8.8.8.8 1.1.1.1 ];\n", []string{"9.9.9.9", "8.8.8.8", "1.1.1.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := NewParser(testSchema()).Parse(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, tree.ToMap()["name-server"])
		})
	}

	t.Run("no member at all omits the key", func(t *testing.T) {
		tree, err := NewParser(testSchema()).Parse("router-id 10.0.0.1;\n")
		require.NoError(t, err)
		assert.NotContains(t, tree.ToMap(), "name-server")
	})
}

// TestToMapListIsAlwaysAKeyedMap parses a YANG list at one and at two entries
// and asserts ToMap emits the same shape for both.
//
// VALIDATES: a list is a map of list key to entry at every count, never a
// slice, and the key leaf is the map key rather than a field inside the entry.
// PREVENTS: two defects at once. A reader that asserts []any on a list finds
// nothing at EVERY count, so the option never applies: the l2tp named pools
// were configured, accepted, and then every subscriber whose RADIUS profile
// named one was refused. A reader that then looks for the key leaf inside the
// entry finds nothing either, so every entry would come back nameless.
// keyedListSchema declares a list whose KEY LEAF is also a child of the entry,
// the shape `list named-pool { key "name"; leaf name {...} }` has in
// internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang. The schema
// must declare that leaf for the "key is not a field" assertion below to mean
// anything: a list with no key leaf cannot emit one whatever ToMap does.
func keyedListSchema(t *testing.T) *Schema {
	t.Helper()
	schema := NewSchema()
	pools := List(TypeString,
		Field("name", Leaf(TypeString)),
		Field("gateway", Leaf(TypeIPv4)),
	)
	pools.KeyName = "name"
	pools.KeyLeaf = Leaf(TypeString)
	require.True(t, pools.Has("name"),
		"the schema MUST declare the key leaf, or the assertion this schema exists for is vacuous")
	schema.Define("named-pool", pools)
	return schema
}

// TestToMapKeyedListOmitsTheKeyLeafFromTheEntry is the assertion the pool fix
// rests on: the operator writes `named-pool gold { ... }`, and the name reaches
// the reader as the MAP KEY, never as a `name` field inside the entry, even
// though the schema declares that leaf.
//
// VALIDATES: a keyed list entry carries the leaves written inside its braces
// and not its own key leaf.
// PREVENTS: a reader taking the entry name from `entry["name"]`, which is what
// the pool parser did while it also asserted []any. Both halves had to change
// together, and a schema without a key leaf would let a wrong reader pass.
func TestToMapKeyedListOmitsTheKeyLeafFromTheEntry(t *testing.T) {
	tree, err := NewParser(keyedListSchema(t)).Parse("named-pool gold {\n gateway 10.0.0.1;\n}\n")
	require.NoError(t, err)

	byKey, ok := tree.ToMap()["named-pool"].(map[string]any)
	require.True(t, ok, "a list is a map[string]any, never a slice")
	require.Len(t, byKey, 1)

	entry, ok := byKey["gold"].(map[string]any)
	require.True(t, ok, "the entry is keyed by the name the operator wrote")
	assert.Equal(t, "10.0.0.1", entry["gateway"], "a leaf written inside the braces is a field")
	assert.NotContains(t, entry, "name", "the key leaf is the map key, never a field inside the entry")
}

func TestToMapListIsAlwaysAKeyedMap(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		wantKeys []string
	}{
		{
			name:     "one entry",
			input:    "neighbor 10.0.0.1 {\n peer-as 65001;\n}\n",
			wantKeys: []string{"10.0.0.1"},
		},
		{
			name:     "two entries",
			input:    "neighbor 10.0.0.1 {\n peer-as 65001;\n}\nneighbor 10.0.0.2 {\n peer-as 65002;\n}\n",
			wantKeys: []string{"10.0.0.1", "10.0.0.2"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := NewParser(testSchema()).Parse(tc.input)
			require.NoError(t, err)

			byKey, ok := tree.ToMap()["neighbor"].(map[string]any)
			require.True(t, ok, "a list is a map[string]any, never a slice")
			require.Len(t, byKey, len(tc.wantKeys))

			for _, key := range tc.wantKeys {
				_, ok := byKey[key].(map[string]any)
				require.True(t, ok, "entry %q is a map", key)
			}
		})
	}
}
