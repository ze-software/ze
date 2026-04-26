// Design: plan/spec-web-5-bgp.md -- BGP Summary page
// Related: workbench_table.go -- Reusable table component
// Related: page_bgp_peers.go -- Peer page (sibling)

package web

import (
	"html/template"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// BuildBGPSummaryTableData constructs a read-only summary table from the
// config tree's peer listing. For v1, operational data (state, uptime,
// prefix counts) is not available; the table shows config-derived fields
// with placeholder columns. A future spec will populate from the BGP
// reactor's live session data.
func BuildBGPSummaryTableData(viewTree *config.Tree) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Peer", Sortable: true},
		{Key: "remote-ip", Label: "Remote IP", Sortable: true},
		{Key: "remote-as", Label: "Remote AS", Sortable: true},
		{Key: "state", Label: "State", Sortable: true},
		{Key: "uptime", Label: "Uptime"},
		{Key: "prefixes", Label: "Prefixes"},
		{Key: "messages-in", Label: "Msg In"},
		{Key: "messages-out", Label: "Msg Out"},
		{Key: "last-error", Label: "Last Error"},
	}

	peers := collectPeers(viewTree)
	rows := make([]WorkbenchTableRow, 0, len(peers))
	for _, pe := range peers {
		// Operational state placeholder
		state := peerStateConfigured
		if pe.Disabled {
			state = peerStateDisabled
		}

		flags, flagClass := peerFlag(pe)

		rows = append(rows, WorkbenchTableRow{
			Key:       pe.Name,
			URL:       pe.EditURL,
			Flags:     flags,
			FlagClass: flagClass,
			Cells: []string{
				pe.Name,
				valueOrDash(pe.RemoteIP),
				valueOrDash(pe.RemoteAS),
				state,
				"--", // Uptime placeholder
				"--", // Prefixes placeholder
				"--", // Messages In placeholder
				"--", // Messages Out placeholder
				"--", // Last Error placeholder
			},
		})
	}

	return WorkbenchTableData{
		Title:        "BGP Summary",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No BGP peers configured.",
		EmptyHint:    "Add peers to see the BGP summary.",
	}
}

// HandleBGPSummaryPage renders the BGP summary table within the workbench.
func HandleBGPSummaryPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	tableData := BuildBGPSummaryTableData(viewTree)
	return renderer.RenderFragment("workbench_table", tableData)
}
