// Design: docs/architecture/api/commands.md — dashboard rendering.
// Overview: model.go — editor model and update loop.
// Related: model_dashboard.go — dashboard state and lifecycle.
// Related: model_dashboard_sort.go — sort column enum and sort logic.

package cli

import (
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Dashboard styles.
var (
	dashHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")) // white
	dashFooterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))           // dim gray
	dashSelectedStyle = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("6"))  // cyan bg
	dashGreenStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))             // green
	dashYellowStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))             // yellow
	dashRedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))             // red
	dashErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))             // red for errors
	dashConnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))             // green for connected
)

// dashboardColumnDef defines a table column with its sort identity, width, and priority.
// Lower priority number = dropped last (never drop Peer).
type dashboardColumnDef struct {
	col      dashboardSortColumn
	width    int
	priority int // 1=never drop, 7=first to drop
}

// dashboardColumns defines the table columns in display order.
var dashboardColumns = []dashboardColumnDef{
	{col: sortColumnAddress, width: 16, priority: 1},
	{col: sortColumnASN, width: 8, priority: 2},
	{col: sortColumnState, width: 12, priority: 3},
	{col: sortColumnUptime, width: 10, priority: 4},
	{col: sortColumnRx, width: 8, priority: 5},
	{col: sortColumnTx, width: 8, priority: 6},
	{col: sortColumnRate, width: 8, priority: 7},
}

// renderDashboardHeader renders the 2-line header bar.
func renderDashboardHeader(snap *dashboardSnapshot, pollError string, width int) string {
	var tb textbuf.Buffer
	if snap == nil {
		return tb.Str(dashHeaderStyle.Render("BGP Dashboard")).Byte('\n').Str(dashErrorStyle.Render("waiting for data...")).String()
	}

	line1 := tb.Str("AS ").Uint32(snap.LocalAS).Str("  rid ").Str(snap.RouterID).Str("  up ").Str(snap.Uptime).Str("  peers ").Int(int64(snap.PeersEstablished)).Byte('/').Int(int64(snap.PeersConfigured)).String()
	if width > 0 && len(line1) > width {
		line1 = line1[:width]
	}

	var line2 string
	if pollError != "" {
		line2 = dashErrorStyle.Render(pollError)
	} else {
		line2 = dashConnStyle.Render("connected")
	}

	return tb.Reset().Str(dashHeaderStyle.Render(line1)).Byte('\n').Str(line2).String()
}

// renderDashboardFooter renders the 1-line footer with key hints and last update info.
func renderDashboardFooter(lastUpdate string, width int) string {
	left := "q Quit  s Sort  j/k Navigate  Enter Detail  Esc Back"
	var tb textbuf.Buffer
	right := ""
	if lastUpdate != "" {
		right = tb.Str("Last update: ").Str(lastUpdate).String()
	}

	gap := max(1, width-len(left)-len(right))

	return dashFooterStyle.Render(tb.Reset().Str(left).Repeat(" ", gap).Str(right).String())
}

// visibleColumns returns the columns that fit within the terminal width.
func visibleColumns(width int) []dashboardColumnDef {
	cols := make([]dashboardColumnDef, len(dashboardColumns))
	copy(cols, dashboardColumns)

	// Calculate total width needed.
	total := 0
	for _, c := range cols {
		total += c.width + 2 // 2 for spacing
	}

	// Drop columns from highest priority number (least important) first.
	for total > width && len(cols) > 1 {
		maxPri := 0
		maxIdx := 0
		for i, c := range cols {
			if c.priority > maxPri {
				maxPri = c.priority
				maxIdx = i
			}
		}
		total -= cols[maxIdx].width + 2
		cols = append(cols[:maxIdx], cols[maxIdx+1:]...)
	}

	return cols
}

// renderDashboardPeerTable renders the peer table with headers and rows.
func renderDashboardPeerTable(peers []dashboardPeer, ds *dashboardState, sortCol dashboardSortColumn, sortAsc bool, width, maxRows int) string {
	cols := visibleColumns(width)
	if len(peers) == 0 {
		header := renderTableHeader(cols, sortCol, sortAsc)
		var tb textbuf.Buffer
		return tb.Str(header).Byte('\n').Str(dashFooterStyle.Render("  no peers configured")).String()
	}

	var sb textbuf.Buffer
	sb.Str(renderTableHeader(cols, sortCol, sortAsc)).Byte('\n')

	for i, p := range peers {
		if maxRows > 0 && i >= maxRows {
			break
		}
		row := renderPeerRow(p, cols, ds)
		if i == ds.resolveSelectedIndex(peers) {
			row = dashSelectedStyle.Render(row)
		}
		sb.Str(row)
		if i < len(peers)-1 && (maxRows <= 0 || i < maxRows-1) {
			sb.Byte('\n')
		}
	}

	return sb.String()
}

// renderTableHeader renders the column headers with sort indicator.
func renderTableHeader(cols []dashboardColumnDef, sortCol dashboardSortColumn, sortAsc bool) string {
	var tb textbuf.Buffer
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		tb.Reset().Str(c.col.String())
		if c.col == sortCol {
			if sortAsc {
				tb.Str(" ^")
			} else {
				tb.Str(" v")
			}
		}
		header := tb.String()
		parts = append(parts, tb.Reset().PadRight(header, c.width).String())
	}
	return dashHeaderStyle.Render(textbuf.Join(parts, "  "))
}

// renderPeerRow renders a single peer row.
func renderPeerRow(p dashboardPeer, cols []dashboardColumnDef, ds *dashboardState) string {
	var tb textbuf.Buffer
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		val := peerColumnValue(p, c.col, ds)
		parts = append(parts, tb.Reset().PadRight(val, c.width).String())
	}
	return textbuf.Join(parts, "  ")
}

// peerColumnValue returns the display value for a peer in the given column.
func peerColumnValue(p dashboardPeer, col dashboardSortColumn, ds *dashboardState) string {
	switch col {
	case sortColumnAddress:
		return p.Address
	case sortColumnASN:
		return strconv.Itoa(int(p.RemoteAS))
	case sortColumnState:
		return stateStyled(p.State)
	case sortColumnUptime:
		return p.Uptime
	case sortColumnRx:
		return formatCounter(p.UpdatesReceived)
	case sortColumnTx:
		return formatCounter(p.UpdatesSent)
	case sortColumnRate:
		return ds.peerRate(p.Address)
	case numSortColumns:
		return ""
	}
	return ""
}

// stateStyled returns the state string with color applied.
func stateStyled(state string) string {
	switch state {
	case "established":
		return dashGreenStyle.Render(state)
	case "connecting", "active", "opensent", "openconfirm":
		return dashYellowStyle.Render(state)
	case "stopped", "idle", "idle-hold":
		return dashRedStyle.Render(state)
	}
	return state
}

// renderDashboardDetail renders the detail view for a single peer.
func renderDashboardDetail(ds *dashboardState) string {
	if ds.snapshot == nil {
		return dashFooterStyle.Render("  no data")
	}

	var peer *dashboardPeer
	for i := range ds.snapshot.Peers {
		if ds.snapshot.Peers[i].Address == ds.detailAddr {
			peer = &ds.snapshot.Peers[i]
			break
		}
	}
	if peer == nil {
		return dashFooterStyle.Render("  peer not found")
	}

	rate := ds.peerRate(peer.Address)

	var sb textbuf.Buffer
	var tb textbuf.Buffer
	sb.Str(dashHeaderStyle.Render(tb.Str("  Peer Detail: ").Str(peer.Address).String()))
	sb.Str("\n\n")

	rows := []struct{ label, value string }{
		{"Remote ASN", strconv.Itoa(int(peer.RemoteAS))},
		{"State", stateStyled(peer.State)},
		{"Uptime", peer.Uptime},
		{"Updates Rx", formatCounter(peer.UpdatesReceived)},
		{"Updates Tx", formatCounter(peer.UpdatesSent)},
		{"Keepalives Rx", formatCounter(peer.KeepalivesReceived)},
		{"Keepalives Tx", formatCounter(peer.KeepalivesSent)},
		{"EOR Rx", formatCounter(peer.EORReceived)},
		{"EOR Tx", formatCounter(peer.EORSent)},
		{"Update Rate", rate},
	}

	// Append extended fields from peer-detail RPC if available.
	if d := ds.detailData; d != nil {
		if rid, ok := d["router-id"].(string); ok {
			rows = append(rows, struct{ label, value string }{"Router ID", rid})
		}
		if las, ok := d["local-as"].(float64); ok {
			rows = append(rows, struct{ label, value string }{"Local ASN", strconv.Itoa(int(las))})
		}
		if timer, ok := d["timer"].(map[string]any); ok {
			if rht, ok := timer["receive-hold-time"].(float64); ok {
				rows = append(rows, struct{ label, value string }{"Recv Hold Time", textbuf.IntStr(int64(rht), "s")})
			}
			if sht, ok := timer["send-hold-time"].(float64); ok {
				rows = append(rows, struct{ label, value string }{"Send Hold Time", textbuf.IntStr(int64(sht), "s")})
			}
			if cr, ok := timer["connect-retry"].(float64); ok {
				rows = append(rows, struct{ label, value string }{"Connect Retry", textbuf.IntStr(int64(cr), "s")})
			}
		}
		if conn, ok := d["connect"].(bool); ok {
			rows = append(rows, struct{ label, value string }{"Connect", strconv.FormatBool(conn)})
		}
		if acc, ok := d["accept"].(bool); ok {
			rows = append(rows, struct{ label, value string }{"Accept", strconv.FormatBool(acc)})
		}
		if lip, ok := d["local-ip"].(string); ok {
			rows = append(rows, struct{ label, value string }{"Local IP", lip})
		}
		if name, ok := d["name"].(string); ok {
			rows = append(rows, struct{ label, value string }{"Name", name})
		}
		if group, ok := d["group"].(string); ok {
			rows = append(rows, struct{ label, value string }{"Group", group})
		}
	}

	for _, r := range rows {
		sb.Str("  ").PadRight(r.label, 16).Byte(' ').Str(r.value).Byte('\n')
	}

	sb.Byte('\n')
	sb.Str(dashFooterStyle.Render("  Esc Back  ? Help"))

	return sb.String()
}

// formatCounter formats an integer counter with thousands separators.
func formatCounter(n uint32) string {
	if n < 1000 {
		return strconv.Itoa(int(n))
	}
	s := strconv.Itoa(int(n))
	// Insert commas from the right.
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// renderDashboardHelp renders the help overlay.
func renderDashboardHelp() string {
	var sb textbuf.Buffer
	sb.Str(dashHeaderStyle.Render("  Dashboard Help"))
	sb.Str("\n\n")

	keys := []struct{ key, action string }{
		{"j / Down", "Select next peer"},
		{"k / Up", "Select previous peer"},
		{"s", "Cycle sort column"},
		{"S", "Reverse sort direction"},
		{"Enter", "Show peer detail"},
		{"Esc", "Back (detail -> table -> exit)"},
		{"q / Ctrl-C", "Quit dashboard"},
		{"?", "Toggle this help"},
	}

	for _, k := range keys {
		sb.Str("  ").PadRight(k.key, 14).Byte(' ').Str(k.action).Byte('\n')
	}

	sb.Byte('\n')
	sb.Str(dashFooterStyle.Render("  Press any key to dismiss"))
	return sb.String()
}
