// Design: plan/spec-web-5-bgp.md -- BGP Groups table page
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Peer page (sibling)

package web

import (
	"fmt"
	"html/template"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// groupEntry holds extracted fields for one BGP peer group.
type groupEntry struct {
	Name      string
	PeerCount int
	RemoteAS  string
	Families  string
}

// collectGroups walks the config tree and returns all BGP groups.
func collectGroups(viewTree *config.Tree) []groupEntry {
	if viewTree == nil {
		return nil
	}
	bgpTree := viewTree.GetContainer("bgp")
	if bgpTree == nil {
		return nil
	}

	var groups []groupEntry
	for _, entry := range bgpTree.GetListOrdered("group") {
		ge := extractGroupEntry(entry.Key, entry.Value)
		groups = append(groups, ge)
	}
	return groups
}

// extractGroupEntry reads relevant fields from one group's config sub-tree.
func extractGroupEntry(name string, groupTree *config.Tree) groupEntry {
	ge := groupEntry{Name: name}
	if groupTree == nil {
		return ge
	}

	// Count peers in this group
	ge.PeerCount = len(groupTree.GetList("peer"))

	// Group-level session defaults
	if sess := groupTree.GetContainer("session"); sess != nil {
		if asn := sess.GetContainer("asn"); asn != nil {
			if remote, ok := asn.Get("remote"); ok {
				ge.RemoteAS = remote
			}
		}

		families := sess.GetListOrdered("family")
		names := make([]string, 0, len(families))
		for _, f := range families {
			names = append(names, f.Key)
		}
		ge.Families = strings.Join(names, ", ")
	}

	return ge
}

// BuildBGPGroupsTableData constructs a WorkbenchTableData for the groups table.
func BuildBGPGroupsTableData(groups []groupEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "peer-count", Label: "Peer Count", Sortable: true},
		{Key: "remote-as", Label: "Remote AS", Sortable: true},
		{Key: "families", Label: "Families"},
	}

	rows := make([]WorkbenchTableRow, 0, len(groups))
	for _, ge := range groups {
		rows = append(rows, WorkbenchTableRow{
			Key: ge.Name,
			URL: fmt.Sprintf("/show/bgp/group/%s/", ge.Name),
			Cells: []string{
				ge.Name,
				fmt.Sprintf("%d", ge.PeerCount),
				valueOrDash(ge.RemoteAS),
				valueOrDash(ge.Families),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Peers", URL: fmt.Sprintf("/show/bgp/peer/?group=%s", ge.Name)},
				{Label: "Edit", URL: fmt.Sprintf("/show/bgp/group/%s/", ge.Name)},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "BGP Groups",
		AddURL:       "/show/bgp/group/add",
		AddLabel:     "Add Group",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No peer groups configured.",
		EmptyHint:    "Groups let you share settings across multiple peers.",
	}
}

// HandleBGPGroupsPage renders the BGP groups table within the workbench.
func HandleBGPGroupsPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	groups := collectGroups(viewTree)
	tableData := BuildBGPGroupsTableData(groups)
	return renderer.RenderFragment("workbench_table", tableData)
}
