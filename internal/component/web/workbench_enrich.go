// Design: docs/architecture/web-interface.md -- workbench data enrichment
// Related: handler_workbench.go -- caller that runs enrichment per request
// Related: fragment.go -- shared data model the workbench extends
// Related: ../config/related.go -- ze:related descriptor source
//
// Spec: plan/spec-web-2-operator-workbench.md (Phase 4 -- BGP change-and-verify).
//
// The enrichment layer attaches workbench-specific data to a FragmentData built
// by the shared fragment pipeline: per-row related-tool buttons and
// pending-change markers. Finder leaves these fields zero-valued so the
// shared list_table template renders unchanged when V2 is disabled.

package web

import (
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	segBus            = "bus"
	segBusEndpoint    = "endpoint"
	segBusParticipant = "participant"
	colBusInternal    = "internal"
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
		// A row's path is its entry's address as segments, the same path its
		// URL carries: a multi-leaf key `fe80::b interface zbl0` is the three
		// segments fe80::b/interface/zbl0, never one segment holding spaces.
		key := data.ListTable.Rows[i].KeyValue
		rowPath := append([]string{}, path...)
		rowPath = append(rowPath, config.ListKeyTokens(listNode, key)...)
		rowPathJoined := encodeConfigPath(rowPath)

		data.ListTable.Rows[i].RowTools = buildRowTools(rowTools, rowPathJoined)
		data.ListTable.Rows[i].HasPendingChanges = entryHasPendingChange(pendingPaths, path, listNode, key)
	}

	appendImpliedWorkbenchRows(data.ListTable, tree, path)
}

// appendImpliedWorkbenchRows appends read-only rows for implied bus endpoints
// or participants that are not already present among the written rows of table.
// Uses Offline: false so workbench rendering never spawns external query
// processes.
func appendImpliedWorkbenchRows(table *ListTableView, tree *config.Tree, path []string) {
	if table == nil {
		return
	}
	if tree == nil {
		return
	}
	if len(path) != 2 {
		return
	}
	if path[0] != segBus {
		return
	}
	if path[1] != segBusEndpoint && path[1] != segBusParticipant {
		return
	}

	report, err := config.BuildImpliedReport(tree, config.ImpliedOptions{Offline: false})
	if err != nil {
		return
	}

	writtenKeys := make(map[string]bool, len(table.Rows))
	for i := range table.Rows {
		writtenKeys[table.Rows[i].KeyValue] = true
	}

	var nonKeyCols []ListTableColumn
	if len(table.Columns) > 1 {
		nonKeyCols = table.Columns[1:]
	}

	switch path[1] {
	case segBusEndpoint:
		for i := range report.Endpoints {
			ep := &report.Endpoints[i]
			if ep.Name == "" {
				continue
			}
			if writtenKeys[ep.Name] {
				continue
			}
			writtenKeys[ep.Name] = true

			cells := make([]ListTableCell, 0, len(nonKeyCols))
			for _, col := range nonKeyCols {
				val := ""
				if col.Name == colBusInternal {
					val = ep.CompiledPlugin
					if val == "" {
						val = ep.Name
					}
				}
				cells = append(cells, ListTableCell{Value: val})
			}

			table.Rows = append(table.Rows, ListTableRow{
				KeyValue:  ep.Name,
				Cells:     cells,
				Implied:   true,
				ImpliedBy: impliedEndpointReason(ep),
			})
		}
	case segBusParticipant:
		for i := range report.Participants {
			p := &report.Participants[i]
			if p.Path == "" {
				continue
			}
			if writtenKeys[p.Path] {
				continue
			}
			reason, hasImplied := impliedParticipantReason(p)
			if !hasImplied {
				continue
			}
			writtenKeys[p.Path] = true

			cells := make([]ListTableCell, len(nonKeyCols))
			table.Rows = append(table.Rows, ListTableRow{
				KeyValue:  p.Path,
				Cells:     cells,
				Implied:   true,
				ImpliedBy: reason,
			})
		}
	}
}

func impliedEndpointReason(ep *config.ImpliedEndpoint) string {
	if ep.ImpliedBy != "" {
		return ep.ImpliedBy
	}
	if ep.ReasonKind != "" {
		var tb textbuf.Buffer
		tb.Str(ep.ReasonKind)
		if ep.ReasonSource != "" {
			tb.Byte(' ').Str(ep.ReasonSource)
		}
		return tb.String()
	}
	return "auto-load of " + ep.Name
}

func impliedParticipantReason(p *config.ImpliedParticipant) (string, bool) {
	for i := range p.Receive {
		if reason, ok := impliedDirectionRuleReason(&p.Receive[i]); ok {
			return reason, true
		}
	}
	for i := range p.Send {
		if reason, ok := impliedDirectionRuleReason(&p.Send[i]); ok {
			return reason, true
		}
	}
	if impliedAllowActive(p.ReceiveAllowOrigin, p.ReceiveAllowNone, p.ReceiveAllow) {
		if p.ReceiveAllowSource != "" {
			return p.ReceiveAllowSource, true
		}
		return p.ReceiveAllowOrigin, true
	}
	if impliedAllowActive(p.SendAllowOrigin, p.SendAllowNone, p.SendAllow) {
		if p.SendAllowSource != "" {
			return p.SendAllowSource, true
		}
		return p.SendAllowOrigin, true
	}
	return "", false
}

func impliedDirectionRuleReason(rule *config.ImpliedDirectionRule) (string, bool) {
	if rule.Origin == config.ImpliedOriginWritten && len(rule.ReplacedDefaults) == 0 && !rule.Dormant {
		return "", false
	}
	if rule.Source != "" {
		return rule.Source, true
	}
	if rule.Dormant && rule.DormantReason != "" {
		return rule.DormantReason, true
	}
	return rule.Origin, true
}

func impliedAllowActive(origin string, allowNone bool, allow []string) bool {
	if origin == "" {
		return false
	}
	if origin == config.ImpliedOriginWritten {
		return false
	}
	if allowNone {
		return true
	}
	return len(allow) > 0
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
	baseURL := showPathPrefix + encodeConfigPath(path) + "/"
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
		// The list-table view selects only row and table placements.
		//exhaustive:ignore
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
	cp := encodeConfigPath(listPath)
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

// entryHasPendingChange reports whether any pending-change path names the
// list entry stored under key, or a node inside it. Pending paths use `/`
// separators. The segments after listPath are read as an entry address, so
// a path under `fe80::b/interface/zbl1` belongs to the entry
// `fe80::b interface zbl1` and never to the entry `fe80::b`, though one
// segment prefix covers both. A single-leaf key is one segment, so it
// matches the entry exactly or with a trailing `/` boundary.
func entryHasPendingChange(paths, listPath []string, listNode *config.ListNode, key string) bool {
	if key == "" || len(listPath) == 0 {
		return false
	}
	prefix := encodeConfigPath(listPath) + "/"
	listName := listPath[len(listPath)-1]
	for _, p := range paths {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok || rest == "" {
			continue
		}
		tokens, err := decodeConfigPath(rest)
		if err != nil {
			continue
		}
		addr, err := config.ParseListAddress(listName, listNode, tokens)
		if err == nil && addr.Key == key {
			return true
		}
	}
	return false
}
