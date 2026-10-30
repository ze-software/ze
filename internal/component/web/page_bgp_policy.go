// Design: docs/architecture/web-workbench-pages.md -- Policy filters page
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Peer page (sibling)

package web

import (
	"html/template"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// policyPathPrefix is the workbench URL of the policy root. Filters live at
// the top-level `policy` root, beside policy routing's `policy route`, so a
// filter's editor URL is the policy root plus its type and name.
const policyPathPrefix = "/show/" + segPolicy + "/"

// policyEntry holds one filter/policy definition from the config tree.
type policyEntry struct {
	Name      string
	Type      string
	RuleCount int
}

// collectPolicies walks the top-level policy root and returns every filter
// instance. The filter types come from the merged YANG schema, so a type a
// plugin provides appears and vanishes with its plugin.
func collectPolicies(viewTree *config.Tree, schema *config.Schema) []policyEntry {
	if viewTree == nil || schema == nil {
		return nil
	}
	policyTree := viewTree.GetContainer(segPolicy)
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

// policyFilterListNames names the filter types under the policy root: the
// lists the schema marks ze:filter. The root also holds lists that are not
// filter types, such as policy routing's `route`, and the marker is what keeps
// them off this page (internal/component/policy BuildRegistry reads the same
// marker).
func policyFilterListNames(schema *config.Schema) []string {
	node, err := walkSchema(schema, []string{segPolicy})
	if err != nil {
		return nil
	}
	lister, ok := node.(childLister)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(lister.Children()))
	for _, name := range lister.Children() {
		list, isList := lister.Get(name).(*config.ListNode)
		if !isList {
			continue
		}
		if list.Filter {
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

// buildBGPPolicyTableData constructs a WorkbenchTableData for the filters page.
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
			URL: policyPathPrefix + pe.Type + "/" + pe.Name + "/",
			Cells: []string{
				pe.Name,
				pe.Type,
				strconv.Itoa(pe.RuleCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: labelEdit, URL: policyPathPrefix + pe.Type + "/" + pe.Name + "/"},
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
			URL:   policyPathPrefix + name + "/",
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

// handleBGPPolicyPage renders the filters table of the policy root within the
// workbench.
func handleBGPPolicyPage(renderer *Renderer, viewTree *config.Tree, schema *config.Schema) template.HTML {
	entries := collectPolicies(viewTree, schema)
	tableData := buildBGPPolicyTableData(entries, policyFilterAddActions(schema)...)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}
