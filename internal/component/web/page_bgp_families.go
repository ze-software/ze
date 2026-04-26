// Design: plan/spec-web-5-bgp.md -- BGP Families page
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Peer page (sibling)

package web

import (
	"html/template"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// familyEntry holds one address family configuration row across a peer or group.
type familyEntry struct {
	Family           string
	PeerOrGroup      string
	Mode             string
	MaxPrefixes      string
	WarningPct       string
	TeardownOnLimit  string
	DefaultOriginate string
}

// collectFamilies walks the config tree and returns family config across all
// peers and groups.
func collectFamilies(viewTree *config.Tree) []familyEntry {
	if viewTree == nil {
		return nil
	}
	bgpTree := viewTree.GetContainer("bgp")
	if bgpTree == nil {
		return nil
	}

	var entries []familyEntry

	// Standalone peers
	for _, peerItem := range bgpTree.GetListOrdered("peer") {
		entries = append(entries, extractFamiliesFromPeer(peerItem.Key, peerItem.Value)...)
	}

	// Grouped peers
	for _, groupItem := range bgpTree.GetListOrdered("group") {
		groupTree := groupItem.Value
		if groupTree == nil {
			continue
		}
		// Group-level families
		entries = append(entries, extractFamiliesFromPeer(groupItem.Key+" (group)", groupTree)...)
		// Per-peer families within group
		for _, peerItem := range groupTree.GetListOrdered("peer") {
			entries = append(entries, extractFamiliesFromPeer(peerItem.Key, peerItem.Value)...)
		}
	}

	return entries
}

// extractFamiliesFromPeer reads session/family entries from a peer or group tree.
func extractFamiliesFromPeer(name string, peerTree *config.Tree) []familyEntry {
	if peerTree == nil {
		return nil
	}
	sess := peerTree.GetContainer("session")
	if sess == nil {
		return nil
	}

	families := sess.GetListOrdered("family")
	result := make([]familyEntry, 0, len(families))
	for _, f := range families {
		fe := familyEntry{
			Family:      f.Key,
			PeerOrGroup: name,
		}
		if f.Value != nil {
			if mode, ok := f.Value.Get("mode"); ok {
				fe.Mode = mode
			}
			if pfx := f.Value.GetContainer("prefix"); pfx != nil {
				if max, ok := pfx.Get("maximum"); ok {
					fe.MaxPrefixes = max
				}
				if warn, ok := pfx.Get("warning"); ok {
					fe.WarningPct = warn
				}
				if td, ok := pfx.Get("teardown"); ok {
					fe.TeardownOnLimit = td
				}
			}
			if do, ok := f.Value.Get("default-originate"); ok {
				fe.DefaultOriginate = do
			}
		}
		result = append(result, fe)
	}
	return result
}

// BuildBGPFamiliesTableData constructs a WorkbenchTableData for the families view.
func BuildBGPFamiliesTableData(entries []familyEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "family", Label: "Family", Sortable: true},
		{Key: "peer", Label: "Peer / Group", Sortable: true},
		{Key: "mode", Label: "Mode", Sortable: true},
		{Key: "max-prefixes", Label: "Max Prefixes"},
		{Key: "warning", Label: "Warning"},
		{Key: "teardown", Label: "Teardown on Limit"},
		{Key: "default-originate", Label: "Default Originate"},
	}

	rows := make([]WorkbenchTableRow, 0, len(entries))
	for _, fe := range entries {
		rows = append(rows, WorkbenchTableRow{
			Key: fe.Family + "/" + fe.PeerOrGroup,
			Cells: []string{
				fe.Family,
				fe.PeerOrGroup,
				valueOrDash(fe.Mode),
				valueOrDash(fe.MaxPrefixes),
				valueOrDash(fe.WarningPct),
				valueOrDash(fe.TeardownOnLimit),
				valueOrDash(fe.DefaultOriginate),
			},
		})
	}

	return WorkbenchTableData{
		Title:        "BGP Address Families",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No address families configured.",
		EmptyHint:    "Address families are configured per peer under session/family.",
	}
}

// HandleBGPFamiliesPage renders the BGP families table within the workbench.
func HandleBGPFamiliesPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	entries := collectFamilies(viewTree)
	tableData := BuildBGPFamiliesTableData(entries)
	return renderer.RenderFragment("workbench_table", tableData)
}
