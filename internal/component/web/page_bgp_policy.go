// Design: docs/architecture/web-workbench-pages.md -- BGP Policy/Filters page
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Peer page (sibling)

package web

import (
	"html/template"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// policyEntry holds one filter/policy definition from the config tree.
type policyEntry struct {
	Name      string
	Type      string
	RuleCount int
}

// collectPolicies walks the bgp/policy container and returns all filter
// definitions. Filter list names come from the merged YANG schema so
// plugin-provided filters appear and vanish with their plugin.
func collectPolicies(viewTree *config.Tree, schema *config.Schema) []policyEntry {
	if viewTree == nil || schema == nil {
		return nil
	}
	bgpTree := viewTree.GetContainer("bgp")
	if bgpTree == nil {
		return nil
	}
	policyTree := bgpTree.GetContainer("policy")
	if policyTree == nil {
		return nil
	}

	var entries []policyEntry
	for _, listName := range policyFilterListNames(schema) {
		for _, item := range policyTree.GetListOrdered(listName) {
			ruleCount := countRules(item.Value)
			entries = append(entries, policyEntry{
				Name:      item.Key,
				Type:      listName,
				RuleCount: ruleCount,
			})
		}
	}
	return entries
}

func policyFilterListNames(schema *config.Schema) []string {
	node, err := walkSchema(schema, []string{segBGP, segPolicy})
	if err != nil {
		return nil
	}
	lister, ok := node.(childLister)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(lister.Children()))
	for _, name := range lister.Children() {
		if _, ok := lister.Get(name).(*config.ListNode); ok {
			names = append(names, name)
		}
	}
	return names
}

// countRules counts the number of rule entries in a filter definition.
// Rules are typically stored as list entries under "rule" or "entry".
func countRules(filterTree *config.Tree) int {
	if filterTree == nil {
		return 0
	}
	// Try common rule list names
	for _, name := range []string{"rule", "entry", "term", "sequence"} {
		if l := filterTree.GetList(name); len(l) > 0 {
			return len(l)
		}
	}
	return 0
}

// buildBGPPolicyTableData constructs a WorkbenchTableData for the policy page.
func buildBGPPolicyTableData(entries []policyEntry, addActions ...WorkbenchTableAddAction) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: colName, Label: labelName, Sortable: true},
		{Key: colType, Label: labelType, Sortable: true},
		{Key: segRules, Label: "Rule Count", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	for _, pe := range entries {
		rows = append(rows, WorkbenchTableRow{
			Key: pe.Name,
			URL: "/show/bgp/policy/" + pe.Name + "/",
			Cells: []string{
				pe.Name,
				pe.Type,
				strconv.Itoa(pe.RuleCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: labelEdit, URL: "/show/bgp/policy/" + pe.Name + "/"},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "BGP Filters",
		AddActions:   addActions,
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No filters configured.",
		EmptyHint:    "Filters control which routes are accepted or advertised.",
	}
}

func policyFilterAddActions(schema *config.Schema) []WorkbenchTableAddAction {
	names := policyFilterListNames(schema)
	actions := make([]WorkbenchTableAddAction, 0, len(names))
	for _, name := range names {
		actions = append(actions, WorkbenchTableAddAction{
			Label: "Add " + titleFromSchemaName(name),
			URL:   "/show/bgp/policy/" + name + "/",
		})
	}
	return actions
}

func titleFromSchemaName(name string) string {
	var out textbuf.Buffer
	upperNext := true
	for _, r := range name {
		if r == '-' || r == '_' {
			out.Byte(' ')
			upperNext = true
			continue
		}
		if upperNext && r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		out.WriteRune(r)
		upperNext = false
	}
	return out.String()
}

// handleBGPPolicyPage renders the BGP policy/filters table within the workbench.
func handleBGPPolicyPage(renderer *Renderer, viewTree *config.Tree, schema *config.Schema) template.HTML {
	entries := collectPolicies(viewTree, schema)
	tableData := buildBGPPolicyTableData(entries, policyFilterAddActions(schema)...)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}
