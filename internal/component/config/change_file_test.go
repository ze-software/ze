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
		ParentPath: EditTokens{"bgp"},
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

	entries := parsedMeta.SessionEntries(schema, ops[0].SessionKey())
	require.Len(t, entries, 1)
	assert.Equal(t, "bgp peer paris description", entries[0].Path.String())
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
		ParentPath: EditTokens{"bgp"},
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
		ParentPath: EditTokens{"bgp"},
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
	assert.Equal(t, "", parsedOps[0].ParentPath.String())
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
	assert.Equal(t, "", parsedOps[0].ParentPath.String())
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
		ParentPath: EditTokens{"bgp"},
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
	assert.Equal(t, "", parsedOps[0].ParentPath.String())
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
			ParentPath: EditTokens{"bgp"}, ListName: "peer", OldKey: "a", NewKey: "b",
		},
		{
			Type: StructuralOpDeleteEntry, User: "thomas", Source: "local", Time: stamp,
			ParentPath: EditTokens{"bgp"}, ListName: "peer", OldKey: "b",
		},
	}

	result := CoalesceRenameOps(ops)
	require.Len(t, result, 2, "rename and delete must not be merged")
	assert.Equal(t, StructuralOpRename, result[0].Type)
	assert.Equal(t, "b", result[0].NewKey, "rename NewKey must not be overwritten")
	assert.Equal(t, StructuralOpDeleteEntry, result[1].Type)
}

// TestPendingChangeSummaryPreservesSetStyleFallback exercises the member
// toggles and the valid zero kind. A member toggle names its verb and its
// member (round-5 ISSUE 3: it printed "set <path> " with an empty value), and
// the zero kind keeps the set-style display. Both mask the value when no
// schema can tell a credential from an address.
func TestPendingChangeSummaryPreservesSetStyleFallback(t *testing.T) {
	op := StructuralOp{
		Type:       StructuralOpDeactivateMember,
		ParentPath: EditTokens{"system"},
		ListName:   "name-server",
		NewKey:     "192.0.2.1",
	}
	change := op.PendingChange(nil)
	if got := change.Summary(nil); got != "deactivate system name-server "+SecretDataPlaceholder {
		t.Fatalf("deactivation summary = %q, want the verb and the masked member", got)
	}
	op.Type = StructuralOpActivateMember
	if got := op.PendingChange(nil).Summary(nil); got != "activate system name-server "+SecretDataPlaceholder {
		t.Fatalf("activation summary = %q, want the verb and the masked member", got)
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
// session editor records for creation, copy and leaf/path deactivation survive
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
		{"create-entry", withOp(base, StructuralOpCreateEntry, EditTokens{"bgp", "peer"}, "london", "", ""),
			"create-entry bgp peer london", PendingChangeSet, "bgp peer london", "set bgp peer london"},
		{"create-entry quoted", withOp(base, StructuralOpCreateEntry, EditTokens{"bgp", "peer"}, "edge west", "", ""),
			`create-entry bgp peer "edge west"`, PendingChangeSet, `bgp peer "edge west"`, `set bgp peer "edge west"`},
		{"copy-entry", withOp(base, StructuralOpCopyEntry, EditTokens{"bgp"}, "peer", "london", "paris"),
			"copy-entry bgp peer london to paris", PendingChangeCopy, "bgp peer paris", "copy bgp peer london to bgp peer paris"},
		{"deactivate-leaf", withOp(base, StructuralOpDeactivateLeaf, EditTokens{"bgp", "peer", "london"}, "description", "", ""),
			"deactivate-leaf bgp peer london description", PendingChangeDeactivate, "bgp peer london description", "deactivate bgp peer london description"},
		{"activate-leaf", withOp(base, StructuralOpActivateLeaf, EditTokens{"bgp", "peer", "london"}, "description", "", ""),
			"activate-leaf bgp peer london description", PendingChangeActivate, "bgp peer london description", "activate bgp peer london description"},
		{"deactivate-path", withOp(base, StructuralOpDeactivatePath, EditTokens{"bgp", "peer"}, "london", "", ""),
			"deactivate-path bgp peer london", PendingChangeDeactivate, "bgp peer london", "deactivate bgp peer london"},
		{"activate-path root", withOp(base, StructuralOpActivatePath, EditTokens{}, "bgp", "", ""),
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

			change := parsed[0].PendingChange(schema)
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
		"#thomas @ssh %2026-10-10T01:02:03Z create-entry",
		"create-entry bgp peer london",
		"#thomas @ssh %2026-10-10T01:02:03Z deactivate-leaf",
		"#thomas @ssh %2026-10-10T01:02:03Z activate-path",
		"#thomas @ssh %2026-10-10T01:02:03Z copy-entry bgp peer london paris",
		"deactivate-path bgp",
	} {
		_, _, _, err := ParseChangeFile(line+"\n", NewSetParser(schema))
		assert.Error(t, err, line)
	}
}

func withOp(base StructuralOp, opType StructuralOpType, parentPath EditTokens, name, oldKey, newKey string) StructuralOp {
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

// TestKeyMoveRecordRoundTripsQuotedKeys proves the rename and copy records
// of a change file read back the keys they were written with.
//
// GOAL: a session-mode rename or copy of an entry whose key holds a space
// (a multi-leaf address), a quote, a backslash or the word `to` reloads
// with the same keys. METHOD: format each op with formatKeyMoveLine, read it
// back through extractMeta and parseKeyMoveLine, and compare; then load old
// raw records written before keys were quoted.
//
// VALIDATES: splitQuotedTokens is the inverse of QuoteIfNeeded; `to` is read
// at its fixed place; a ParentPath token starting with `"` round-trips; an
// old raw record loads as it did; an unterminated `'` is refused naming the
// line.
// PREVENTS: strings.Fields splitting `fe80::b interface zbl0` into three
// tokens, and the first `to` being taken for the separator.
func TestKeyMoveRecordRoundTripsQuotedKeys(t *testing.T) {
	keys := []string{
		"fe80::b interface zbl0",
		`a"b`,
		`a\b`,
		"it's",
		`all "of\ 'them`,
		"to",
	}
	for _, key := range keys {
		assert.Equal(t, []string{key}, mustSplit(t, QuoteIfNeeded(key)), "splitQuotedTokens(QuoteIfNeeded(%q))", key)
	}

	for _, opType := range []StructuralOpType{StructuralOpRename, StructuralOpCopyEntry} {
		for _, key := range keys {
			op := StructuralOp{
				Type: opType, User: "alice", Source: "local", Time: time.Unix(1700000000, 0).UTC(),
				ParentPath: EditTokens{"bfd", `"odd`}, ListName: "single-hop-session", OldKey: key, NewKey: "fe80::c " + key,
			}
			line := formatKeyMoveLine(op)
			entry, cmdLine := extractMeta(line)
			got, err := parseKeyMoveLine(1, entry, cmdLine, opType)
			require.NoError(t, err, line)
			assert.Equal(t, op.OldKey, got.OldKey, line)
			assert.Equal(t, op.NewKey, got.NewKey, line)
			assert.Equal(t, op.ParentPath, got.ParentPath, line)
			assert.Equal(t, op.ListName, got.ListName, line)
		}
	}

	t.Run("old raw record loads", func(t *testing.T) {
		entry, cmdLine := extractMeta("#alice @local rename bfd single-hop-session fe80::b#2 to fe80::c")
		got, err := parseKeyMoveLine(1, entry, cmdLine, StructuralOpRename)
		require.NoError(t, err)
		assert.Equal(t, "fe80::b#2", got.OldKey)
		assert.Equal(t, "fe80::c", got.NewKey)
		assert.Equal(t, "bfd", got.ParentPath.String())
	})

	t.Run("unterminated quote is refused naming the line", func(t *testing.T) {
		entry, cmdLine := extractMeta("#alice @local rename bgp peer 'peer1 to peer2")
		_, err := parseKeyMoveLine(7, entry, cmdLine, StructuralOpRename)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "line 7")
		assert.Contains(t, err.Error(), "unterminated")
	})
}

// mustSplit is splitQuotedTokens for a line the test knows is well formed.
func mustSplit(t *testing.T, line string) []string {
	t.Helper()
	tokens, err := splitQuotedTokens(line)
	require.NoError(t, err, line)
	return tokens
}
