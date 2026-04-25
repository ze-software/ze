package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestEnrichWorkbenchTable_RowToolsFromBGPPeer verifies that the enrichment
// pass attaches every Day-One BGP row tool to each peer row, preserving
// the spec's tool ordering and embedding the row's YANG path so the
// browser POSTs the correct context.
//
// VALIDATES: AC-3 (rows show related tool actions), Spec D12 (BGP peer
// is the first complete workflow).
// PREVENTS: A regression that drops tool buttons from rows or sends a
// stale context path.
func TestEnrichWorkbenchTable_RowToolsFromBGPPeer(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	data := buildFragmentData(schema, tree, []string{"bgp", "peer"})
	require.NotNil(t, data.ListTable, "bgp/peer must render as a list table")
	enrichWorkbenchTable(data, schema, tree, []string{"bgp", "peer"}, nil)

	require.Len(t, data.ListTable.Rows, 1, "the seeded tree contains exactly one peer")
	row := data.ListTable.Rows[0]
	assert.Equal(t, "thomas", row.KeyValue)

	gotIDs := make([]string, 0, len(row.RowTools))
	for _, b := range row.RowTools {
		gotIDs = append(gotIDs, b.ToolID)
		assert.Equal(t, "bgp/peer/thomas", b.ContextPath, "context path must point at the row")
	}
	for _, want := range []string{"peer-detail", "peer-capabilities", "peer-statistics", "peer-flush", "peer-teardown"} {
		assert.Contains(t, gotIDs, want, "row missing expected tool %q", want)
	}
}

// TestEnrichWorkbenchTable_ConfirmFlagOnTeardown verifies that mutating
// tools (peer-teardown) propagate their Confirm prompt onto the rendered
// row button so the template can flag the explicit-confirmation path.
//
// VALIDATES: D8 (mutating tools require confirmation), spec wires this
// from YANG -> RelatedTool.Confirm -> RelatedToolButton.Confirm.
func TestEnrichWorkbenchTable_ConfirmFlagOnTeardown(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	data := buildFragmentData(schema, tree, []string{"bgp", "peer"})
	require.NotNil(t, data.ListTable)
	enrichWorkbenchTable(data, schema, tree, []string{"bgp", "peer"}, nil)

	row := data.ListTable.Rows[0]
	var teardown *RelatedToolButton
	for i := range row.RowTools {
		if row.RowTools[i].ToolID == "peer-teardown" {
			teardown = &row.RowTools[i]
			break
		}
	}
	require.NotNil(t, teardown, "peer-teardown must render on bgp/peer rows")
	assert.NotEmpty(t, teardown.Confirm, "peer-teardown must carry the Confirm prompt")
	assert.Equal(t, "danger", teardown.Class, "peer-teardown styling class must be danger")
}

// TestEnrichWorkbenchTable_PendingMarker verifies that a row whose subtree
// has uncommitted edits gets HasPendingChanges=true. The marker drives
// the visible row indicator the operator uses to track unsaved work.
//
// VALIDATES: AC-17 (pending change marker visible after edit).
// PREVENTS: Operators losing track of which row has unsaved edits when
// the table contains many peers.
func TestEnrichWorkbenchTable_PendingMarker(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	data := buildFragmentData(schema, tree, []string{"bgp", "peer"})
	require.NotNil(t, data.ListTable)

	// Pending paths from the editor session: one path under the row, one
	// unrelated to verify the prefix gate.
	pending := []string{"bgp/peer/thomas/connection/remote/ip", "iface/eth0/admin"}
	enrichWorkbenchTable(data, schema, tree, []string{"bgp", "peer"}, pending)

	require.Len(t, data.ListTable.Rows, 1)
	assert.True(t, data.ListTable.Rows[0].HasPendingChanges, "row with subtree edit must be flagged")
}

// TestEnrichWorkbenchTable_PendingMarkerIsolated verifies that with
// multiple peers in the table, only the rows whose YANG subtree contains
// a pending change get HasPendingChanges=true. A naive implementation
// that flagged "the table has any pending change" would mark every row.
//
// VALIDATES: row-level prefix match (anyPathUnder) is per-row.
// PREVENTS: A future change that swaps anyPathUnder for a "are there any
// pending changes at all" check, which would silently flag every row.
func TestEnrichWorkbenchTable_PendingMarkerIsolated(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "thomas", config.NewTree())
	bgp.AddListEntry("peer", "alex", config.NewTree())
	bgp.AddListEntry("peer", "marie", config.NewTree())

	data := buildFragmentData(schema, tree, []string{"bgp", "peer"})
	require.NotNil(t, data.ListTable)

	// Only thomas has a pending edit.
	pending := []string{"bgp/peer/thomas/connection/remote/ip"}
	enrichWorkbenchTable(data, schema, tree, []string{"bgp", "peer"}, pending)

	require.Len(t, data.ListTable.Rows, 3)
	flagged := map[string]bool{}
	for _, row := range data.ListTable.Rows {
		flagged[row.KeyValue] = row.HasPendingChanges
	}
	assert.True(t, flagged["thomas"], "thomas must be flagged")
	assert.False(t, flagged["alex"], "alex must not be flagged")
	assert.False(t, flagged["marie"], "marie must not be flagged")
}

// TestEnrichWorkbenchTable_NoPendingChanges verifies that with no editor
// session changes, no row carries the pending flag.
func TestEnrichWorkbenchTable_NoPendingChanges(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	data := buildFragmentData(schema, tree, []string{"bgp", "peer"})
	require.NotNil(t, data.ListTable)
	enrichWorkbenchTable(data, schema, tree, []string{"bgp", "peer"}, nil)

	for _, row := range data.ListTable.Rows {
		assert.False(t, row.HasPendingChanges, "no edits in session: row %q must not be pending", row.KeyValue)
	}
}

// TestEnrichWorkbenchTable_PromotesListWithoutUniqueToTable verifies that
// the workbench enrichment builds a list table even for lists that lack a
// `unique` YANG constraint. Finder only renders the table view when
// uniqueFields is non-empty; the workbench's table-first contract requires
// every named list to render as a table.
//
// VALIDATES: Phase 5 generalization (named list without unique renders
// table); BGP peer table (which DOES have unique) continues to work
// because TestEnrichWorkbenchTable_RowToolsFromBGPPeer covers it.
// PREVENTS: A list without unique falling back to the Finder column
// columns, which would break the table-first navigation goal.
func TestEnrichWorkbenchTable_PromotesListWithoutUniqueToTable(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	// bgp/group has key "name" but no `unique` constraint, so Finder leaves
	// data.ListTable nil. The workbench must still produce a table view.
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("group", "g1", config.NewTree())

	data := buildFragmentData(schema, tree, []string{"bgp", "group"})
	assert.Nil(t, data.ListTable, "Finder builder must skip lists without unique")

	enrichWorkbenchTable(data, schema, tree, []string{"bgp", "group"}, nil)
	require.NotNil(t, data.ListTable, "workbench must promote bgp/group to a table")
	require.Len(t, data.ListTable.Rows, 1)
	assert.Equal(t, "g1", data.ListTable.Rows[0].KeyValue)
	// At minimum the key column must be present.
	require.NotEmpty(t, data.ListTable.Columns)
	assert.True(t, data.ListTable.Columns[0].Key, "first column must be the list key")
}

// TestDefaultWorkbenchColumns_OrdersUniqueRequiredSuggest verifies the
// column merge order: unique fields first, then required, then suggest,
// with duplicates dropped.
//
// VALIDATES: Spec Default columns row ("Key plus required, unique,
// suggested, and decorated fields when available").
// PREVENTS: Column drift across releases changing the operator's mental
// model of the table layout.
func TestDefaultWorkbenchColumns_OrdersUniqueRequiredSuggest(t *testing.T) {
	listNode := &config.ListNode{
		Unique:   [][]string{{"a/b"}},
		Required: [][]string{{"a", "b"}, {"c"}},
		Suggest:  [][]string{{"d"}},
	}
	got := defaultWorkbenchColumns(listNode)
	assert.Equal(t, []string{"a/b", "c", "d"}, got, "unique > required > suggest, dedup applied")
}

// TestEnrichWorkbenchTable_NoOpOnNonListPath verifies that the enrichment
// pass is idempotent on contexts that do not produce a list table view
// (e.g. a leaf or container path).
func TestEnrichWorkbenchTable_NoOpOnNonListPath(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	data := buildFragmentData(schema, tree, []string{"bgp", "peer", "thomas"})
	enrichWorkbenchTable(data, schema, tree, []string{"bgp", "peer", "thomas"}, nil)

	// No list table at the row context; the call must just return cleanly.
	if data.ListTable != nil {
		for _, row := range data.ListTable.Rows {
			assert.Empty(t, row.RowTools)
		}
	}
}

// TestAnyPathUnder verifies the prefix-matching helper correctly identifies
// paths that fall under a row's subtree, including the row path itself
// (rename) and excludes sibling rows that share a prefix-of-prefix.
func TestAnyPathUnder(t *testing.T) {
	cases := []struct {
		name   string
		paths  []string
		prefix string
		want   bool
	}{
		{name: "exact match", paths: []string{"bgp/peer/thomas"}, prefix: "bgp/peer/thomas", want: true},
		{name: "child path", paths: []string{"bgp/peer/thomas/connection/remote/ip"}, prefix: "bgp/peer/thomas", want: true},
		{name: "sibling", paths: []string{"bgp/peer/alex/connection/remote/ip"}, prefix: "bgp/peer/thomas", want: false},
		{name: "shared prefix only", paths: []string{"bgp/peer/thomas-2"}, prefix: "bgp/peer/thomas", want: false},
		{name: "empty paths", paths: nil, prefix: "bgp/peer/thomas", want: false},
		{name: "empty prefix", paths: []string{"bgp/peer/thomas"}, prefix: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, anyPathUnder(tc.paths, tc.prefix))
		})
	}
}
