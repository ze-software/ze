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

// TestChangeFileRoundTripDeleteListOp verifies delete-list ops round-trip through serialize/parse.
//
// VALIDATES: formatDeleteListLine and parseDeleteListLine are symmetric.
// PREVENTS: delete-list ops lost or corrupted during change-file persistence.
func TestChangeFileRoundTripDeleteListOp(t *testing.T) {
	schema := testChangeFileSchema()
	stamp := time.Date(2026, 4, 21, 12, 34, 56, 0, time.UTC)

	ops := []StructuralOp{{
		Type:       StructuralOpDeleteList,
		User:       "thomas",
		Source:     "local",
		Time:       stamp,
		ParentPath: "bgp",
		ListName:   "peer",
	}}

	content := SerializeChangeFile(NewTree(), NewMetaTree(), ops, schema)
	assert.Contains(t, content, "#thomas @local %2026-04-21T12:34:56Z delete-list bgp peer")

	_, _, parsedOps, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	require.Len(t, parsedOps, 1)
	assert.Equal(t, ops[0], parsedOps[0])
}

// TestChangeFileDeleteListEmptyParent verifies delete-list at root level.
//
// VALIDATES: delete-list round-trips with empty parent path.
// PREVENTS: parseDeleteListLine mishandling root-level list deletes.
func TestChangeFileDeleteListEmptyParent(t *testing.T) {
	schema := testChangeFileSchema()
	stamp := time.Date(2026, 4, 21, 12, 34, 56, 0, time.UTC)

	ops := []StructuralOp{{
		Type:     StructuralOpDeleteList,
		User:     "thomas",
		Source:   "local",
		Time:     stamp,
		ListName: "peer",
	}}

	content := SerializeChangeFile(NewTree(), NewMetaTree(), ops, schema)

	_, _, parsedOps, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	require.Len(t, parsedOps, 1)
	assert.Equal(t, "", parsedOps[0].ParentPath)
	assert.Equal(t, "peer", parsedOps[0].ListName)
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

// TestPendingChangeSummaryPreservesSetStyleFallback exercises member deactivation
// and the valid zero kind. Both retain set-style display and masking.
func TestPendingChangeSummaryPreservesSetStyleFallback(t *testing.T) {
	op := StructuralOp{
		Type:       StructuralOpDeactivateMember,
		ParentPath: "system",
		ListName:   "name-server",
		NewKey:     "192.0.2.1",
	}
	change := op.PendingChange()
	if got := change.Summary(nil); got != "set system name-server " {
		t.Fatalf("deactivation summary = %q, want existing set-style display", got)
	}

	change.Kind = ""
	change.Value = "private-value"
	if got := change.Summary(nil); got != "set system name-server "+SecretDataPlaceholder {
		t.Fatalf("zero-kind summary = %q, want masked set-style display", got)
	}

	change.Kind = PendingChangeKind("future-change")
	require.PanicsWithValue(t, "BUG: invalid pending change kind", func() {
		change.Summary(nil)
	})
}

// TestChangeFileDeactivateOpsRoundTrip verifies that the structural ops the
// session editor records for copy and for leaf and path deactivation survive
// serialize and parse unchanged, and project to one pending change each.
func TestChangeFileDeactivateOpsRoundTrip(t *testing.T) {
	schema := testChangeFileSchema()
	stamp := time.Date(2026, 10, 10, 1, 2, 3, 0, time.UTC)
	base := StructuralOp{User: "thomas", Source: "ssh", Time: stamp}

	cases := []struct {
		name    string
		op      StructuralOp
		line    string
		kind    PendingChangeKind
		path    string
		summary string
	}{
		{"copy-entry", withOp(base, StructuralOpCopyEntry, "bgp", "peer", "london", "paris"),
			"copy-entry bgp peer london to paris", PendingChangeCopy, "bgp peer paris", "copy bgp peer london to bgp peer paris"},
		{"deactivate-leaf", withOp(base, StructuralOpDeactivateLeaf, "bgp peer london", "description", "", ""),
			"deactivate-leaf bgp peer london description", PendingChangeDeactivate, "bgp peer london description", "deactivate bgp peer london description"},
		{"activate-leaf", withOp(base, StructuralOpActivateLeaf, "bgp peer london", "description", "", ""),
			"activate-leaf bgp peer london description", PendingChangeActivate, "bgp peer london description", "activate bgp peer london description"},
		{"deactivate-path", withOp(base, StructuralOpDeactivatePath, "bgp peer", "london", "", ""),
			"deactivate-path bgp peer london", PendingChangeDeactivate, "bgp peer london", "deactivate bgp peer london"},
		{"activate-path root", withOp(base, StructuralOpActivatePath, "", "bgp", "", ""),
			"activate-path bgp", PendingChangeActivate, "bgp", "activate bgp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := SerializeChangeFile(NewTree(), NewMetaTree(), []StructuralOp{tc.op}, schema)
			assert.Contains(t, content, "#thomas @ssh %2026-10-10T01:02:03Z "+tc.line)

			_, _, parsed, err := ParseChangeFile(content, NewSetParser(schema))
			require.NoError(t, err)
			require.Len(t, parsed, 1)
			assert.Equal(t, tc.op, parsed[0])

			change := parsed[0].PendingChange()
			assert.Equal(t, tc.kind, change.Kind)
			assert.Equal(t, tc.path, change.Path)
			assert.Equal(t, tc.summary, change.Summary(schema))
		})
	}
}

// TestParseChangeFileRejectsTruncatedToggleOps verifies a toggle or copy line
// missing its operands is a parse error, never an op with an empty target.
func TestParseChangeFileRejectsTruncatedToggleOps(t *testing.T) {
	schema := testChangeFileSchema()
	for _, line := range []string{
		"#thomas @ssh %2026-10-10T01:02:03Z deactivate-leaf",
		"#thomas @ssh %2026-10-10T01:02:03Z activate-path",
		"#thomas @ssh %2026-10-10T01:02:03Z copy-entry bgp peer london paris",
		"deactivate-path bgp",
	} {
		_, _, _, err := ParseChangeFile(line+"\n", NewSetParser(schema))
		assert.Error(t, err, line)
	}
}

func withOp(base StructuralOp, opType StructuralOpType, parentPath, name, oldKey, newKey string) StructuralOp {
	base.Type = opType
	base.ParentPath = parentPath
	base.ListName = name
	base.OldKey = oldKey
	base.NewKey = newKey
	return base
}

// TestChangeFileLeafDeleteSurvivesRewrite verifies that a pending leaf delete
// inside a list entry survives a parse and a re-serialize of the change file.
// Every write-through after the first reads the file back and writes it again,
// and a delete line creates no tree node to hang its metadata on, so the
// serializer, which walks the tree, dropped the delete at the next edit and the
// commit never applied it.
func TestChangeFileLeafDeleteSurvivesRewrite(t *testing.T) {
	schema := testChangeFileSchema()
	content := "#thomas @ssh %2026-10-10T01:02:03Z ^old delete bgp peer london description\n"

	tree, meta, ops, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	rewritten := SerializeChangeFile(tree, meta, ops, schema)
	assert.Contains(t, rewritten, "#thomas @ssh %2026-10-10T01:02:03Z ^old delete bgp peer london description")

	_, again, _, err := ParseChangeFile(rewritten, NewSetParser(schema))
	require.NoError(t, err)
	entry, ok := again.GetContainer("bgp").GetContainer("peer").GetListEntry("london").GetEntry("description")
	require.True(t, ok, "the delete metadata survives a second round trip")
	assert.Equal(t, "old", entry.Previous)
}
