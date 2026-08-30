// Design: docs/architecture/web-workbench-pages.md -- BGP Groups table page
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Peer page (sibling)

package web

import (
	"html/template"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
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
		ge.Families = textbuf.Join(names, ", ")
	}

	return ge
}

// buildBGPGroupsTableData constructs a WorkbenchTableData for the groups table.
func buildBGPGroupsTableData(groups []groupEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: colName, Label: labelName, Sortable: true},
		{Key: "peer-count", Label: "Peer Count", Sortable: true},
		{Key: colRemoteAS, Label: labelRemoteAS, Sortable: true},
		{Key: segFamilies, Label: labelFamilies},
	}

	rows := make([]WorkbenchTableRow, 0, len(groups))
	var tb textbuf.Buffer
	for _, ge := range groups {
		groupURL := tb.Reset().Str("/show/bgp/group/").Str(ge.Name).Byte('/').String()
		rows = append(rows, WorkbenchTableRow{
			Key: ge.Name,
			URL: groupURL,
			Cells: []string{
				ge.Name,
				strconv.Itoa(ge.PeerCount),
				valueOrDash(ge.RemoteAS),
				valueOrDash(ge.Families),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View Peers", URL: tb.Reset().Str("/show/bgp/group/").Str(ge.Name).Str("/peer/").String()},
				{Label: labelEdit, URL: groupURL},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "BGP Groups",
		AddURL:       "/show/bgp/group/",
		AddLabel:     "Add Group",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No peer groups configured.",
		EmptyHint:    "Groups let you share settings across multiple peers.",
	}
}

// handleBGPGroupsPage renders the BGP groups table within the workbench.
func handleBGPGroupsPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	groups := collectGroups(viewTree)
	tableData := buildBGPGroupsTableData(groups)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}
