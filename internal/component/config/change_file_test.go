package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testChangeFileSchema() *Schema {
	schema := NewSchema()
	schema.Define("bgp", Container(
		Field("peer", List(TypeString,
			Field("description", Leaf(TypeString)),
		)),
	))
	return schema
}

// TestChangeFileRoundTripRenameOp verifies rename ops round-trip alongside leaf metadata.
func TestChangeFileRoundTripRenameOp(t *testing.T) {
	schema := testChangeFileSchema()
	stamp := time.Date(2026, 4, 21, 12, 34, 56, 0, time.UTC)

	tree := NewTree()
	bgp := NewTree()
	entry := NewTree()
	entry.Set("description", "renamed peer")
	bgp.AddListEntry("peer", "paris", entry)
	tree.SetContainer("bgp", bgp)

	meta := NewMetaTree()
	target := meta.GetOrCreateContainer("bgp").GetOrCreateContainer("peer").GetOrCreateListEntry("paris")
	target.SetEntry("description", MetaEntry{
		User:     "thomas",
		Source:   "web",
		Time:     stamp,
		Previous: "old peer",
		Value:    "renamed peer",
	})

	ops := []StructuralOp{{
		Type:       StructuralOpRename,
		User:       "thomas",
		Source:     "web",
		Time:       stamp,
		ParentPath: "bgp",
		ListName:   "peer",
		OldKey:     "london",
		NewKey:     "paris",
	}}

	content := SerializeChangeFile(tree, meta, ops, schema)
	assert.Contains(t, content, "#thomas @web %2026-04-21T12:34:56Z rename bgp peer london to paris")

	parsedTree, parsedMeta, parsedOps, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	require.Len(t, parsedOps, 1)
	assert.Equal(t, ops[0], parsedOps[0])

	parsedBGP := parsedTree.GetContainer("bgp")
	require.NotNil(t, parsedBGP)
	parsedPeers := parsedBGP.GetList("peer")
	require.NotNil(t, parsedPeers)
	require.NotNil(t, parsedPeers["paris"])

	entries := parsedMeta.SessionEntries(ops[0].SessionKey())
	require.Len(t, entries, 1)
	assert.Equal(t, "bgp peer paris description", entries[0].Path)
}

// TestParseChangeFileRejectsMalformedRename verifies malformed rename directives are rejected.
func TestParseChangeFileRejectsMalformedRename(t *testing.T) {
	schema := testChangeFileSchema()
	content := "#thomas @web %2026-04-21T12:34:56Z rename bgp peer london paris\n"

	_, _, _, err := ParseChangeFile(content, NewSetParser(schema))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename")
}
