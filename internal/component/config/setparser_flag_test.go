// Design: docs/architecture/config/syntax.md -- value-less flags survive set-format replay.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetPresenceFlagRoundTrip(t *testing.T) {
	schema := NewSchema()
	flag := Container()
	flag.Presence = true
	schema.Define("flag", flag)
	input := "set flag\n"
	for range 2 {
		tree, err := NewSetParser(schema).Parse(input)
		require.NoError(t, err)
		value, found := tree.Get("flag")
		assert.True(t, found)
		assert.Equal(t, "true", value)
		input = SerializeSet(tree, schema)
		assert.Equal(t, "set flag\n", input)
	}
}

func TestSetFlagMetadataRoundTrip(t *testing.T) {
	for _, presence := range []bool{false, true} {
		name := "empty-leaf"
		schema := NewSchema()
		if presence {
			name = "presence-container"
			flag := Container()
			flag.Presence = true
			schema.Define("flag", flag)
		} else {
			schema.Define("flag", Leaf(TypeEmpty))
		}
		t.Run(name, func(t *testing.T) {
			input := "#alice @web %2026-11-16T02:00:00Z set flag\n"
			for range 2 {
				tree, meta, err := NewSetParser(schema).ParseWithMeta(input)
				require.NoError(t, err)
				value, found := tree.Get("flag")
				assert.True(t, found)
				assert.Equal(t, "true", value)
				entry, found := meta.GetEntry("flag")
				require.True(t, found)
				assert.Equal(t, "true", entry.Value)
				assert.Equal(t, "alice", entry.User)
				assert.Equal(t, "web", entry.Source)
				input = SerializeSetWithMeta(tree, meta, schema)
				assert.Contains(t, input, "set flag\n")
			}
		})
	}
}

// TestSetContestedFlagsKeepBothOwners checks the metadata printer's multi-owner
// branch without turning the canonical presence value into a flag argument.
func TestSetContestedFlagsKeepBothOwners(t *testing.T) {
	for _, presence := range []bool{false, true} {
		name := "empty-leaf"
		schema := NewSchema()
		if presence {
			name = "presence-container"
			flag := Container()
			flag.Presence = true
			schema.Define("flag", flag)
		} else {
			schema.Define("flag", Leaf(TypeEmpty))
		}
		t.Run(name, func(t *testing.T) {
			input := "#alice @web set flag\n#bob @ssh set flag\n"
			for range 2 {
				tree, meta, err := NewSetParser(schema).ParseWithMeta(input)
				require.NoError(t, err)
				entries := meta.GetAllEntries("flag")
				require.Len(t, entries, 2)
				assert.Equal(t, "true", entries[0].Value)
				assert.Equal(t, "true", entries[1].Value)
				input = SerializeSetWithMeta(tree, meta, schema)
				assert.Contains(t, input, "#alice @web set flag\n")
				assert.Contains(t, input, "#bob @ssh set flag\n")
				assert.NotContains(t, input, "set flag true")
			}
		})
	}
}

func TestSetEmptyLeafFlagRoundTrip(t *testing.T) {
	schema := NewSchema()
	schema.Define("flag", Leaf(TypeEmpty))
	tree, err := NewSetParser(schema).Parse("set flag\n")
	require.NoError(t, err)
	text := SerializeSet(tree, schema)
	assert.Equal(t, "set flag\n", text)
	reread, err := NewSetParser(schema).Parse(text)
	require.NoError(t, err)
	value, found := reread.Get("flag")
	assert.True(t, found)
	assert.Equal(t, "true", value)
}
