// Design: docs/architecture/web-interface.md -- V2 workbench data enrichment
// Related: handler_workbench.go -- caller that runs enrichment per request
// Related: fragment.go -- shared data model the workbench extends
// Related: ../config/related.go -- ze:related descriptor source
//
// Spec: plan/spec-web-2-operator-workbench.md (Phase 4 -- BGP change-and-verify).
//
// The enrichment layer attaches V2-specific data to a FragmentData built
// by the shared fragment pipeline: per-row related-tool buttons and
// pending-change markers. Finder leaves these fields zero-valued so the
// shared list_table template renders unchanged when V2 is disabled.

package web

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// enrichWorkbenchTable populates the workbench-only fields on data.ListTable:
// it promotes named lists without `unique` constraints into a table (Phase
// 5 generalization), then attaches row tool buttons (placement=row), table
// tool buttons (placement=table), and the per-row HasPendingChanges flag
// derived from the editor session.
//
// path is the YANG path the request landed on; for a list view it ends at
// the list name (no entry key). tree is the user's working tree (used to
// build a fresh table when Finder did not). pendingPaths is the
// slash-joined paths of the operator's uncommitted edits; pass nil when
// no session exists.
func enrichWorkbenchTable(data *FragmentData, schema *config.Schema, tree *config.Tree, path, pendingPaths []string) {
	if data == nil {
		return
	}

	listNode := lookupListNode(schema, path)
	if listNode == nil {
		return
	}

	// Phase 5: build a list table for any named list, not just lists with
	// unique constraints. Finder's narrower gate keeps its existing behavior
	// because the workbench handler is the only caller that runs this
	// promotion step.
	if data.ListTable == nil {
		data.ListTable = buildWorkbenchListTable(tree, schema, path, listNode)
	}
	if data.ListTable == nil {
		return
	}

	rowTools, tableTools := splitRelatedByPlacement(listNode.Related)
	data.ListTable.TableTools = buildTableTools(tableTools, path)

	for i := range data.ListTable.Rows {
		rowPath := append([]string{}, path...)
		rowPath = append(rowPath, data.ListTable.Rows[i].KeyValue)
		rowPathJoined := strings.Join(rowPath, "/")

		data.ListTable.Rows[i].RowTools = buildRowTools(rowTools, rowPathJoined)
		data.ListTable.Rows[i].HasPendingChanges = anyPathUnder(pendingPaths, rowPathJoined)
	}
}

// buildWorkbenchListTable constructs a ListTableView for any named list,
// using the broader default-columns rule from the spec (key plus required,
// unique, suggested, decorated leaves). Finder builds tables only for
// lists with `unique` constraints; the workbench promotes every named
// list to a table to keep the table-first contract.
func buildWorkbenchListTable(tree *config.Tree, schema *config.Schema, path []string, listNode *config.ListNode) *ListTableView {
	cols := defaultWorkbenchColumns(listNode)
	if len(cols) == 0 {
		// Nothing to put in the rows besides the key. Render a key-only
		// table so the operator still sees the named list as a list.
		cols = nil
	}
	keys := collectListKeys(tree, schema, path)
	baseURL := "/show/" + strings.Join(path, "/") + "/"
	return buildListTable(tree, schema, path, listNode, keys, cols, baseURL)
}

// defaultWorkbenchColumns merges unique, required, and suggested leaf
// paths into a deduplicated ordered slice. Order: unique first (most
// distinctive), then required (mandatory operator inputs), then suggest
// (optional helpful fields). Duplicates from `unique` overlapping with
// `required` are dropped.
func defaultWorkbenchColumns(listNode *config.ListNode) []string {
	var cols []string
	seen := make(map[string]bool)
	add := func(field string) {
		if field == "" || seen[field] {
			return
		}
		seen[field] = true
		cols = append(cols, field)
	}
	for _, c := range collectUniqueFields(listNode) {
		add(c)
	}
	for _, c := range collectRequiredFields(listNode) {
		add(c)
	}
	for _, c := range collectSuggestFields(listNode) {
		add(c)
	}
	return cols
}

// lookupListNode returns the list node at the supplied YANG path, or nil
// if the path does not end at a list. The workbench only enriches list
// views; other contexts return without changes.
func lookupListNode(schema *config.Schema, path []string) *config.ListNode {
	if len(path) == 0 {
		return nil
	}
	node, err := walkSchema(schema, path)
	if err != nil || node == nil {
		return nil
	}
	listNode, ok := node.(*config.ListNode)
	if !ok {
		return nil
	}
	// Ensure the path ends at the list itself (collection view), not an entry.
	if isListEntryPath(schema, path) {
		return nil
	}
	return listNode
}

// splitRelatedByPlacement partitions a Related slice into row-placement and
// table-placement subsets. Tools with other placements (global/detail/field)
// are not surfaced by the table view; they reach the workbench through
// other render paths in later phases.
func splitRelatedByPlacement(tools []*config.RelatedTool) (row, table []*config.RelatedTool) {
	for _, t := range tools {
		switch t.Placement {
		case config.RelatedPlacementRow:
			row = append(row, t)
		case config.RelatedPlacementTable:
			table = append(table, t)
		case config.RelatedPlacementDetail,
			config.RelatedPlacementGlobal,
			config.RelatedPlacementField:
			// Surfaced through other render paths (detail drawer, top bar,
			// field-level affordance). The list-table view ignores them.
		}
	}
	return row, table
}

// buildRowTools converts a per-row slice of descriptors into the render
// data the template iterates over.
func buildRowTools(tools []*config.RelatedTool, contextPath string) []RelatedToolButton {
	if len(tools) == 0 {
		return nil
	}
	out := make([]RelatedToolButton, 0, len(tools))
	for _, t := range tools {
		out = append(out, RelatedToolButton{
			ToolID:      t.ID,
			Label:       t.Label,
			ContextPath: contextPath,
			Class:       t.Class.String(),
			Confirm:     t.Confirm,
		})
	}
	return out
}

// buildTableTools is the table-level analog of buildRowTools; the
// context path is the list node itself so commands resolve against the
// list (not a specific row).
func buildTableTools(tools []*config.RelatedTool, listPath []string) []RelatedToolButton {
	if len(tools) == 0 {
		return nil
	}
	cp := strings.Join(listPath, "/")
	out := make([]RelatedToolButton, 0, len(tools))
	for _, t := range tools {
		out = append(out, RelatedToolButton{
			ToolID:      t.ID,
			Label:       t.Label,
			ContextPath: cp,
			Class:       t.Class.String(),
			Confirm:     t.Confirm,
		})
	}
	return out
}

// anyPathUnder reports whether any pending-change path is a descendant of
// (or equal to) the row's YANG path. Pending paths use `/` separators and
// match the row prefix exactly or with a trailing `/` boundary.
func anyPathUnder(paths []string, prefix string) bool {
	if prefix == "" {
		return false
	}
	for _, p := range paths {
		if p == prefix {
			return true
		}
		if strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}
