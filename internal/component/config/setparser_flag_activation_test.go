// Design: docs/architecture/config/syntax.md -- flag activation survives every printer.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlagActivationRoundTrips(t *testing.T) {
	for _, presence := range []bool{false, true} {
		kind := "empty-leaf"
		schema := NewSchema()
		if presence {
			kind = "presence-container"
			node := Container()
			node.Presence = true
			schema.Define("flag", node)
		} else {
			schema.Define("flag", Leaf(TypeEmpty))
		}
		for _, inactive := range []bool{false, true} {
			state := "active"
			command, hierarchical := "set flag\n", "flag\n"
			if inactive {
				state = "inactive"
				command, hierarchical = "nop flag\n", "inactive: flag\n"
			}
			t.Run(kind+"/"+state, func(t *testing.T) {
				tree := NewTree()
				tree.Set("flag", "true")
				tree.SetLeafInactive("flag", inactive)
				meta := NewMetaTree()
				meta.SetEntry("flag", MetaEntry{User: "alice", Source: "web", Value: "true"})
				for range 2 {
					plain := SerializeSet(tree, schema)
					assert.Equal(t, command, plain)
					reread, err := NewSetParser(schema).Parse(plain)
					require.NoError(t, err)
					assert.Equal(t, inactive, reread.IsLeafInactive("flag"))
					value, found := reread.Get("flag")
					assert.True(t, found)
					assert.Equal(t, "true", value)
					annotated := SerializeSetWithMeta(tree, meta, schema)
					assert.Equal(t, "#alice @web "+command, annotated)
					reread, ownership, err := NewSetParser(schema).ParseWithMeta(annotated)
					require.NoError(t, err)
					assert.Equal(t, inactive, reread.IsLeafInactive("flag"))
					entry, found := ownership.GetEntry("flag")
					require.True(t, found)
					assert.Equal(t, "true", entry.Value)
					text := Serialize(tree, schema)
					assert.Equal(t, hierarchical, text)
					reread, err = NewParser(schema).Parse(text)
					require.NoError(t, err)
					assert.Equal(t, inactive, reread.IsLeafInactive("flag"))
					tree, meta = reread, ownership
				}
			})
		}
	}
}
