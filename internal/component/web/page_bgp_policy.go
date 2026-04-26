// Design: plan/spec-web-5-bgp.md -- BGP Policy/Filters page
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Peer page (sibling)

package web

import (
	"fmt"
	"html/template"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// policyEntry holds one filter/policy definition from the config tree.
type policyEntry struct {
	Name      string
	Type      string
	RuleCount int
}

// collectPolicies walks the bgp/policy container and returns all filter
// definitions. Each filter type is a list augmented by its plugin; the
// list entries are the named filter instances.
func collectPolicies(viewTree *config.Tree) []policyEntry {
	if viewTree == nil {
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

	// Each container name under policy is a filter type (augmented by plugins).
	for _, typeName := range policyTree.ContainerNames() {
		typeTree := policyTree.GetContainer(typeName)
		if typeTree == nil {
			continue
		}
		// Each filter type may contain named filter lists. Walk all lists.
		for _, listName := range listNamesFromTree(typeTree) {
			for _, item := range typeTree.GetListOrdered(listName) {
				ruleCount := countRules(item.Value)
				entries = append(entries, policyEntry{
					Name:      item.Key,
					Type:      typeName,
					RuleCount: ruleCount,
				})
			}
		}
	}

	// Also check for direct lists under policy (non-container filter types).
	for _, listName := range listNamesFromTree(policyTree) {
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

// listNamesFromTree returns the names of all lists in a tree by checking
// which list keys exist.
func listNamesFromTree(t *config.Tree) []string {
	if t == nil {
		return nil
	}
	// Use ContainerNames to enumerate, then check for lists.
	// The Tree type does not expose a ListNames() method, so we check
	// known policy list patterns. For a fully generic approach, we would
	// need the schema. For v1, we return container names and check for
	// lists by trying GetListOrdered on the tree's own level.
	// Actually, we need to find list keys at this level.
	// GetList returns nil for non-existent lists, so we check all
	// known YANG list names under policy.
	var names []string
	for _, candidate := range []string{"filter", "community", "prefix-list", "as-path", "route-map"} {
		if l := t.GetList(candidate); len(l) > 0 {
			names = append(names, candidate)
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

// BuildBGPPolicyTableData constructs a WorkbenchTableData for the policy page.
func BuildBGPPolicyTableData(entries []policyEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "rules", Label: "Rule Count", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	for _, pe := range entries {
		rows = append(rows, WorkbenchTableRow{
			Key: pe.Name,
			URL: fmt.Sprintf("/show/bgp/policy/%s/", pe.Name),
			Cells: []string{
				pe.Name,
				pe.Type,
				fmt.Sprintf("%d", pe.RuleCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "Edit", URL: fmt.Sprintf("/show/bgp/policy/%s/", pe.Name)},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "BGP Filters",
		AddURL:       "/show/bgp/policy/add",
		AddLabel:     "Add Filter",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No filters configured.",
		EmptyHint:    "Filters control which routes are accepted or advertised.",
	}
}

// HandleBGPPolicyPage renders the BGP policy/filters table within the workbench.
func HandleBGPPolicyPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	entries := collectPolicies(viewTree)
	tableData := BuildBGPPolicyTableData(entries)
	return renderer.RenderFragment("workbench_table", tableData)
}
