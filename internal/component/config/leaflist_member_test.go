package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLeafListSchema returns the full YANG schema (system name-server is the
// reference plain leaf-list: ValueOrArrayNode, ip-address items).
func testLeafListSchema(t *testing.T) *Schema {
	t.Helper()
	schema, err := YANGSchema()
	require.NoError(t, err)
	return schema
}

// TestSetParserLeafListRoundTrip: set-format parse must store leaf-list
// members in the multi-value map (the map every serializer reads), so a
// parse → serialize round-trip preserves the lines. Before the fix the set
// parser stored a scalar and the serializer silently dropped the leaf-list.
//
// VALIDATES: set-format leaf-list lines survive parse → serialize.
// PREVENTS: committed config.conf losing leaf-list lines on every rewrite.
func TestSetParserLeafListRoundTrip(t *testing.T) {
	schema := testLeafListSchema(t)

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"single member", "set system name-server 8.8.8.8\n", []string{"8.8.8.8"}},
		{"bracket form", "set system name-server [ 8.8.8.8 1.1.1.1 ]\n", []string{"8.8.8.8", "1.1.1.1"}},
		{"per-member lines accumulate", "set system name-server 8.8.8.8\nset system name-server 1.1.1.1\n", []string{"8.8.8.8", "1.1.1.1"}},
		{"duplicate member line is idempotent", "set system name-server 8.8.8.8\nset system name-server 8.8.8.8\n", []string{"8.8.8.8"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := NewSetParser(schema).Parse(tt.input)
			require.NoError(t, err)
			system := tree.GetContainer("system")
			require.NotNil(t, system, "system container should exist")
			assert.Equal(t, tt.want, system.GetSlice("name-server"),
				"members must land in the multi-value store")

			out := SerializeSet(tree, schema)
			for _, member := range tt.want {
				assert.Contains(t, out, member,
					"serialized set format must preserve member %s", member)
			}
		})
	}
}

// TestSetParserLeafListMemberDelete: `delete <path> <member>` removes one
// member; `delete <path>` removes the whole leaf-list including the
// multi-value store.
//
// VALIDATES: member-delete line semantics and whole-leaf delete cleanup.
// PREVENTS: whole-leaf delete leaving stale members in multiValues.
func TestSetParserLeafListMemberDelete(t *testing.T) {
	schema := testLeafListSchema(t)

	tree, err := NewSetParser(schema).Parse(
		"set system name-server [ 9.9.9.9 8.8.8.8 ]\ndelete system name-server 8.8.8.8\n")
	require.NoError(t, err)
	system := tree.GetContainer("system")
	require.NotNil(t, system)
	assert.Equal(t, []string{"9.9.9.9"}, system.GetSlice("name-server"),
		"member delete must remove exactly one member")

	tree, err = NewSetParser(schema).Parse(
		"set system name-server [ 9.9.9.9 8.8.8.8 ]\ndelete system name-server\n")
	require.NoError(t, err)
	system = tree.GetContainer("system")
	require.NotNil(t, system)
	assert.Empty(t, system.GetSlice("name-server"),
		"whole-leaf delete must clear the multi-value store")
	out := SerializeSet(tree, schema)
	assert.NotContains(t, out, "name-server", "deleted leaf-list must not serialize")
}

// TestMetaTreeMemberEntries: SetEntry keyed by (session, member) — two
// members from the same session coexist; re-setting the same member
// replaces its entry; scalar entries (Member=="") keep replace-by-session.
//
// VALIDATES: per-(path,member) session metadata representation.
// PREVENTS: the second member's metadata replacing the first (single-value
// SessionEntry assumption).
func TestMetaTreeMemberEntries(t *testing.T) {
	mt := NewMetaTree()
	when := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	base := MetaEntry{User: "thomas", Source: "ssh", Time: when}

	a := base
	a.Member = "8.8.8.8"
	a.Value = "8.8.8.8"
	b := base
	b.Member = "1.1.1.1"
	b.Value = "1.1.1.1"

	mt.SetEntry("name-server", a)
	mt.SetEntry("name-server", b)
	assert.Len(t, mt.GetAllEntries("name-server"), 2,
		"two members from one session must coexist")

	// Same member again: replaced, not duplicated.
	mt.SetEntry("name-server", a)
	assert.Len(t, mt.GetAllEntries("name-server"), 2,
		"re-setting the same member must replace its entry")

	// Scalar behavior unchanged: same session replaces.
	s1 := base
	s1.Value = "1.2.3.4"
	s2 := base
	s2.Value = "5.6.7.8"
	mt.SetEntry("router-id", s1)
	mt.SetEntry("router-id", s2)
	assert.Len(t, mt.GetAllEntries("router-id"), 1,
		"scalar entries keep replace-by-session semantics")
}

// TestParseWithMetaLeafListMembers: metadata-annotated per-member lines
// produce one MetaEntry per member with Member set; a member-delete line
// produces a delete-intent entry (Value empty, Member set) and the change
// round-trips through SerializeChangeFile.
//
// VALIDATES: change-file round-trip for leaf-list member set + delete.
// PREVENTS: member metadata lost or merged on change-file re-read.
func TestParseWithMetaLeafListMembers(t *testing.T) {
	schema := testLeafListSchema(t)
	input := "#thomas @ssh %2026-06-10T12:00:00Z set system name-server 8.8.8.8\n" +
		"#thomas @ssh %2026-06-10T12:00:00Z set system name-server 1.1.1.1\n" +
		"#thomas @ssh %2026-06-10T12:00:00Z delete system name-server 9.9.9.9\n"

	tree, meta, err := NewSetParser(schema).ParseWithMeta(input)
	require.NoError(t, err)

	system := tree.GetContainer("system")
	require.NotNil(t, system)
	assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, system.GetSlice("name-server"))

	sysMeta := meta.GetContainer("system")
	require.NotNil(t, sysMeta, "system metadata container should exist")
	entries := sysMeta.GetAllEntries("name-server")
	require.Len(t, entries, 3, "one entry per member operation")

	byMember := map[string]MetaEntry{}
	for _, e := range entries {
		byMember[e.Member] = e
	}
	require.Contains(t, byMember, "8.8.8.8")
	require.Contains(t, byMember, "1.1.1.1")
	require.Contains(t, byMember, "9.9.9.9")
	assert.Equal(t, "8.8.8.8", byMember["8.8.8.8"].Value)
	assert.Equal(t, "1.1.1.1", byMember["1.1.1.1"].Value)
	assert.Empty(t, byMember["9.9.9.9"].Value, "member delete intent has empty Value")

	// Round-trip: serialize the change file and re-parse.
	out := SerializeChangeFile(tree, meta, nil, schema)
	assert.Contains(t, out, "set system name-server 8.8.8.8")
	assert.Contains(t, out, "set system name-server 1.1.1.1")
	assert.Contains(t, out, "delete system name-server 9.9.9.9")

	tree2, meta2, err := NewSetParser(schema).ParseWithMeta(out)
	require.NoError(t, err)
	system2 := tree2.GetContainer("system")
	require.NotNil(t, system2)
	assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, system2.GetSlice("name-server"),
		"members must survive the round-trip")
	sysMeta2 := meta2.GetContainer("system")
	require.NotNil(t, sysMeta2)
	assert.Len(t, sysMeta2.GetAllEntries("name-server"), 3,
		"member metadata must survive the round-trip")
}

// TestSetFormatDeactivatedMemberRoundTrip: deactivated members serialize as
// an `inactive <path> <member>` line with the bare member in the set line —
// never as an "inactive:"-prefixed item, which would fail item validation on
// reparse for typed leaf-lists (ip-address).
//
// VALIDATES: set-format round-trip of deactivated leaf-list members.
// PREVENTS: a committed config with a deactivated name-server member that
// the daemon can no longer parse.
func TestSetFormatDeactivatedMemberRoundTrip(t *testing.T) {
	schema := testLeafListSchema(t)
	input := "set system name-server [ 9.9.9.9 8.8.8.8 ]\ninactive system name-server 8.8.8.8\n"

	tree, err := NewSetParser(schema).Parse(input)
	require.NoError(t, err)
	system := tree.GetContainer("system")
	require.NotNil(t, system)
	require.Equal(t, []string{"9.9.9.9", "inactive:8.8.8.8"}, system.GetSlice("name-server"))

	for _, out := range []string{
		SerializeSet(tree, schema),
		SerializeSetWithMeta(tree, NewMetaTree(), schema),
	} {
		assert.NotContains(t, out, "inactive:8.8.8.8",
			"raw inactive: prefix must not be serialized (fails reparse validation)")
		assert.Contains(t, out, "inactive system name-server 8.8.8.8",
			"deactivation must serialize as an inactive line")

		tree2, parseErr := NewSetParser(schema).Parse(out)
		require.NoError(t, parseErr, "serialized form must reparse")
		system2 := tree2.GetContainer("system")
		require.NotNil(t, system2)
		assert.Equal(t, []string{"9.9.9.9", "inactive:8.8.8.8"}, system2.GetSlice("name-server"),
			"deactivated member must survive the round-trip")
	}
}

// TestChangeFileMemberOpsRoundTrip: the leaf-list member structural ops
// (insert-member with position, deactivate-member, activate-member) survive
// the change-file serialize → parse round-trip.
//
// VALIDATES: per-user change files preserve ordered member operations.
// PREVENTS: a session's insert position or deactivation lost between edits.
func TestChangeFileMemberOpsRoundTrip(t *testing.T) {
	schema := testLeafListSchema(t)
	when := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	ops := []StructuralOp{
		{Type: StructuralOpInsertMember, User: "thomas", Source: "ssh", Time: when,
			ParentPath: "system", ListName: "name-server", NewKey: "1.1.1.1", Position: InsertFirst},
		{Type: StructuralOpInsertMember, User: "thomas", Source: "ssh", Time: when,
			ParentPath: "system", ListName: "name-server", NewKey: "4.4.4.4", Position: InsertBefore, OldKey: "9.9.9.9"},
		{Type: StructuralOpDeactivateMember, User: "thomas", Source: "ssh", Time: when,
			ParentPath: "system", ListName: "name-server", NewKey: "8.8.8.8"},
		{Type: StructuralOpActivateMember, User: "thomas", Source: "ssh", Time: when,
			ParentPath: "system", ListName: "name-server", NewKey: "8.8.8.8"},
	}

	out := SerializeChangeFile(NewTree(), NewMetaTree(), ops, schema)
	_, _, parsed, err := ParseChangeFile(out, NewSetParser(schema))
	require.NoError(t, err)
	require.Len(t, parsed, len(ops))
	for i := range ops {
		assert.Equal(t, ops[i].Type, parsed[i].Type, "op %d type", i)
		assert.Equal(t, ops[i].ParentPath, parsed[i].ParentPath, "op %d parent", i)
		assert.Equal(t, ops[i].ListName, parsed[i].ListName, "op %d leaf", i)
		assert.Equal(t, ops[i].NewKey, parsed[i].NewKey, "op %d member", i)
		assert.Equal(t, ops[i].OldKey, parsed[i].OldKey, "op %d ref", i)
		assert.Equal(t, ops[i].Position, parsed[i].Position, "op %d position", i)
	}
}

// TestPendingChangeMemberSummary: member operations carry the member in the
// unified pending-change view so diffs and conflict messages show it.
//
// VALIDATES: PendingChange propagates Member; delete summary names the member.
// PREVENTS: 'delete system name-server' shown without saying which member.
func TestPendingChangeMemberSummary(t *testing.T) {
	se := SessionEntry{
		Path: "system name-server",
		Entry: MetaEntry{
			User: "thomas", Source: "ssh",
			Time:   time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
			Member: "8.8.8.8",
		},
	}
	pc := PendingChangeFromSessionEntry(se)
	assert.Equal(t, PendingChangeDelete, pc.Kind)
	assert.Equal(t, "8.8.8.8", pc.Member)
	assert.True(t, strings.Contains(pc.Summary(), "8.8.8.8"),
		"delete summary must name the member, got %q", pc.Summary())
}
