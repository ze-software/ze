package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC7950LeafDefaultUsedWhenAbsent feeds ApplyDefaults a map that does not
// carry a defaulted leaf and reads the default back, at the top level and inside
// a list entry.
//
// RFC requirement: RFC7950-7.6.1-1 positive — a leaf absent from the data receives its schema default at the top level and inside a list entry, so the tree carries the value as if the operator had set it.
func TestRFC7950LeafDefaultUsedWhenAbsent(t *testing.T) {
	schema := NewSchema()
	schema.Define("hold-time", LeafWithDefault(TypeUint16, "90"))
	schema.Define("peer", List(TypeString,
		Field("port", LeafWithDefault(TypeUint16, "179")),
	))

	m := map[string]any{
		"peer": map[string]any{
			"192.0.2.1": map[string]any{},
		},
	}

	ApplyDefaults(m, schema.root)

	assert.Equal(t, "90", m["hold-time"], "absent top-level leaf must carry its default")
	peers, ok := m["peer"].(map[string]any)
	require.True(t, ok, "peer list must survive default application")
	entry, ok := peers["192.0.2.1"].(map[string]any)
	require.True(t, ok, "peer entry must survive default application")
	assert.Equal(t, "179", entry["port"], "absent leaf inside a list entry must carry its default")
}

// TestRFC7950LeafDefaultNotUsedWhenSet feeds ApplyDefaults a map that already
// carries the leaf and checks the operator's value survives, and that a leaf with
// no default is not invented.
//
// RFC requirement: RFC7950-7.6.1-1 negative — a leaf the operator set keeps its own value over the schema default, and a leaf with no default is not inserted.
func TestRFC7950LeafDefaultNotUsedWhenSet(t *testing.T) {
	schema := NewSchema()
	schema.Define("hold-time", LeafWithDefault(TypeUint16, "90"))
	schema.Define("description", Leaf(TypeString))

	m := map[string]any{
		"hold-time": "30",
	}

	ApplyDefaults(m, schema.root)

	assert.Equal(t, "30", m["hold-time"], "explicit value must not be replaced by the default")
	_, hasDescription := m["description"]
	assert.False(t, hasDescription, "leaf without a default must not be inserted")
	assert.Len(t, m, 1, "no other key may appear")
}

// TestRFC7950LeafDefaultFollowsItsAncestor checks the third branch of RFC 7950
// §7.6.1: a leaf whose closest ancestor that is not a non-presence container is a
// presence container or a list entry gets its default only when that ancestor
// exists in the data tree.
//
// Method: one schema holds a presence container and a list, each with a
// defaulted leaf. ApplyDefaults runs over a map where both ancestors exist and
// over an empty map, each on its own schema build and its own map.
//
// RFC requirement: RFC7950-7.6.1-3 positive — a presence container and a list entry that exist in the data each receive their child leaf's default.
// RFC requirement: RFC7950-7.6.1-3 negative — when the presence container and the list entry are absent, ApplyDefaults creates neither, so no default is in use.
func TestRFC7950LeafDefaultFollowsItsAncestor(t *testing.T) {
	build := func() *Schema {
		graceful := Container(Field("restart-time", LeafWithDefault(TypeUint16, "120")))
		graceful.Presence = true
		schema := NewSchema()
		schema.Define("graceful-restart", graceful)
		schema.Define("peer", List(TypeString,
			Field("port", LeafWithDefault(TypeUint16, "179")),
		))
		return schema
	}

	t.Run("ancestor exists", func(t *testing.T) {
		m := map[string]any{
			"graceful-restart": map[string]any{},
			"peer":             map[string]any{"192.0.2.1": map[string]any{}},
		}

		ApplyDefaults(m, build().root)

		graceful, ok := m["graceful-restart"].(map[string]any)
		require.True(t, ok, "presence container must survive default application")
		assert.Equal(t, "120", graceful["restart-time"], "existing presence container must carry its child's default")
		peers, ok := m["peer"].(map[string]any)
		require.True(t, ok, "peer list must survive default application")
		entry, ok := peers["192.0.2.1"].(map[string]any)
		require.True(t, ok, "peer entry must survive default application")
		assert.Equal(t, "179", entry["port"], "existing list entry must carry its child's default")
	})

	t.Run("ancestor absent", func(t *testing.T) {
		m := map[string]any{}

		ApplyDefaults(m, build().root)

		_, hasGraceful := m["graceful-restart"]
		assert.False(t, hasGraceful, "absent presence container must not be created to hold a default")
		_, hasPeer := m["peer"]
		assert.False(t, hasPeer, "absent list must not gain an entry to hold a default")
		assert.Empty(t, m, "no default may be in use when its ancestor is absent")
	})
}
