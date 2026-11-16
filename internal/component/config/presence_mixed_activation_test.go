// Design: docs/architecture/config/syntax.md -- presence value and block carriers keep their state.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPresenceMixedFormsKeepActivation checks every combination of independent
// scalar/block state without dropping either value or either author's record.
func TestPresenceMixedFormsKeepActivation(t *testing.T) {
	schema := NewSchema()
	node := Container(Field("child", Leaf(TypeString)))
	node.Presence = true
	schema.Define("mode", node)
	for _, scalarInactive := range []bool{false, true} {
		for _, blockInactive := range []bool{false, true} {
			name := "scalar-active/block-active"
			if scalarInactive {
				name = "scalar-inactive/block-active"
			}
			if blockInactive {
				name += "/block-inactive"
			}
			t.Run(name, func(t *testing.T) {
				tree := NewTree()
				tree.Set("mode", "true")
				tree.SetLeafInactive("mode", scalarInactive)
				child := tree.GetOrCreateContainer("mode")
				child.Set("child", "kept")
				child.SetInactive(blockInactive)
				meta := NewMetaTree()
				meta.SetEntry("mode", MetaEntry{User: "alice", Source: "web", Value: "true"})
				meta.GetOrCreateContainer("mode").SetEntry("child", MetaEntry{User: "bob", Source: "ssh", Value: "kept"})
				check := func(got *Tree) {
					t.Helper()
					value, found := got.Get("mode")
					assert.True(t, found)
					assert.Equal(t, "true", value)
					assert.Equal(t, scalarInactive, got.IsLeafInactive("mode"))
					block := got.GetContainer("mode")
					require.NotNil(t, block)
					assert.Equal(t, blockInactive, block.IsInactive())
					value, found = block.Get("child")
					assert.True(t, found)
					assert.Equal(t, "kept", value)
					applied := got.Clone()
					PruneInactive(applied, schema)
					_, found = applied.Get("mode")
					assert.Equal(t, !scalarInactive, found)
					assert.Equal(t, !blockInactive, applied.GetContainer("mode") != nil)
				}
				for range 2 {
					plain := SerializeSet(tree, schema)
					reread, err := NewSetParser(schema).Parse(plain)
					require.NoError(t, err, plain)
					check(reread)
					annotated := SerializeSetWithMeta(tree, meta, schema)
					reread, ownership, err := NewSetParser(schema).ParseWithMeta(annotated)
					require.NoError(t, err, annotated)
					check(reread)
					flag, found := ownership.GetEntry("mode")
					require.True(t, found)
					assert.Equal(t, "alice", flag.User)
					assert.Equal(t, "web", flag.Source)
					assert.Equal(t, "true", flag.Value)
					blockOwner := ownership.GetContainer("mode")
					require.NotNil(t, blockOwner)
					value, found := blockOwner.GetEntry("child")
					require.True(t, found)
					assert.Equal(t, "bob", value.User)
					assert.Equal(t, "ssh", value.Source)
					assert.Equal(t, "kept", value.Value)
					hierarchical := Serialize(tree, schema)
					reread, err = NewParser(schema).Parse(hierarchical)
					require.NoError(t, err, hierarchical)
					check(reread)
					tree, meta = reread, ownership
				}
			})
		}
	}
}
