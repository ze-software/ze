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

// TestChangeFileRoundTripDeleteEntryOp verifies delete-entry ops round-trip through serialize/parse.
func TestChangeFileRoundTripDeleteEntryOp(t *testing.T) {
	schema := testChangeFileSchema()
	stamp := time.Date(2026, 4, 21, 12, 34, 56, 0, time.UTC)

	ops := []StructuralOp{{
		Type:       StructuralOpDeleteEntry,
		User:       "thomas",
		Source:     "local",
		Time:       stamp,
		ParentPath: "bgp",
		ListName:   "peer",
		OldKey:     "london",
	}}

	content := SerializeChangeFile(NewTree(), NewMetaTree(), ops, schema)
	assert.Contains(t, content, "#thomas @local %2026-04-21T12:34:56Z delete-entry bgp peer london")

	_, _, parsedOps, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	require.Len(t, parsedOps, 1)
	assert.Equal(t, ops[0], parsedOps[0])
}

// TestChangeFileRoundTripDeleteContainerOp verifies delete-container ops round-trip through serialize/parse.
func TestChangeFileRoundTripDeleteContainerOp(t *testing.T) {
	schema := testChangeFileSchema()
	stamp := time.Date(2026, 4, 21, 12, 34, 56, 0, time.UTC)

	ops := []StructuralOp{{
		Type:       StructuralOpDeleteContainer,
		User:       "thomas",
		Source:     "local",
		Time:       stamp,
		ParentPath: "bgp",
		ListName:   "peer",
	}}

	content := SerializeChangeFile(NewTree(), NewMetaTree(), ops, schema)
	assert.Contains(t, content, "#thomas @local %2026-04-21T12:34:56Z delete-container bgp peer")

	_, _, parsedOps, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	require.Len(t, parsedOps, 1)
	assert.Equal(t, ops[0], parsedOps[0])
}

// TestChangeFileDeleteEntryEmptyParent verifies delete-entry at root level (no parent path).
func TestChangeFileDeleteEntryEmptyParent(t *testing.T) {
	schema := testChangeFileSchema()
	stamp := time.Date(2026, 4, 21, 12, 34, 56, 0, time.UTC)

	ops := []StructuralOp{{
		Type:     StructuralOpDeleteEntry,
		User:     "thomas",
		Source:   "local",
		Time:     stamp,
		ListName: "peer",
		OldKey:   "london",
	}}

	content := SerializeChangeFile(NewTree(), NewMetaTree(), ops, schema)

	_, _, parsedOps, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	require.Len(t, parsedOps, 1)
	assert.Equal(t, "", parsedOps[0].ParentPath)
	assert.Equal(t, "peer", parsedOps[0].ListName)
	assert.Equal(t, "london", parsedOps[0].OldKey)
}

// TestChangeFileDeleteContainerEmptyParent verifies delete-container at root level.
func TestChangeFileDeleteContainerEmptyParent(t *testing.T) {
	schema := testChangeFileSchema()
	stamp := time.Date(2026, 4, 21, 12, 34, 56, 0, time.UTC)

	ops := []StructuralOp{{
		Type:     StructuralOpDeleteContainer,
		User:     "thomas",
		Source:   "local",
		Time:     stamp,
		ListName: "bgp",
	}}

	content := SerializeChangeFile(NewTree(), NewMetaTree(), ops, schema)

	_, _, parsedOps, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	require.Len(t, parsedOps, 1)
	assert.Equal(t, "", parsedOps[0].ParentPath)
	assert.Equal(t, "bgp", parsedOps[0].ListName)
}

// TestParseChangeFileRejectsMalformedDeleteEntry verifies missing metadata is rejected.
func TestParseChangeFileRejectsMalformedDeleteEntry(t *testing.T) {
	schema := testChangeFileSchema()
	content := "delete-entry bgp peer london\n"

	_, _, _, err := ParseChangeFile(content, NewSetParser(schema))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete-entry requires #user")
}

// TestParseChangeFileRejectsTruncatedDeleteEntry verifies too-few tokens are rejected.
func TestParseChangeFileRejectsTruncatedDeleteEntry(t *testing.T) {
	schema := testChangeFileSchema()
	content := "#thomas @local %2026-04-21T12:34:56Z delete-entry peer\n"

	_, _, _, err := ParseChangeFile(content, NewSetParser(schema))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete-entry requires")
}

// TestCoalesceRenameOpsSkipsDeleteOps verifies rename+delete are not merged.
func TestCoalesceRenameOpsSkipsDeleteOps(t *testing.T) {
	stamp := time.Date(2026, 4, 21, 12, 34, 56, 0, time.UTC)
	ops := []StructuralOp{
		{
			Type: StructuralOpRename, User: "thomas", Source: "local", Time: stamp,
			ParentPath: "bgp", ListName: "peer", OldKey: "a", NewKey: "b",
		},
		{
			Type: StructuralOpDeleteEntry, User: "thomas", Source: "local", Time: stamp,
			ParentPath: "bgp", ListName: "peer", OldKey: "b",
		},
	}

	result := CoalesceRenameOps(ops)
	require.Len(t, result, 2, "rename and delete must not be merged")
	assert.Equal(t, StructuralOpRename, result[0].Type)
	assert.Equal(t, "b", result[0].NewKey, "rename NewKey must not be overwritten")
	assert.Equal(t, StructuralOpDeleteEntry, result[1].Type)
}
