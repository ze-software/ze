// Design: docs/architecture/api/commands.md — dashboard rendering.
// Overview: model.go — editor model and update loop.
// Related: model_dashboard.go — dashboard state and lifecycle.
// Related: model_dashboard_sort.go — sort column enum and sort logic.

package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	if snap == nil {
		return dashHeaderStyle.Render("BGP Dashboard") + "\n" +
			dashErrorStyle.Render("waiting for data...")
	}

	line1 := fmt.Sprintf("AS %d  rid %s  up %s  peers %d/%d",
		snap.LocalAS, snap.RouterID, snap.Uptime,
		snap.PeersEstablished, snap.PeersConfigured)
	if width > 0 && len(line1) > width {
		line1 = line1[:width]
	}

	var line2 string
	if pollError != "" {
		line2 = dashErrorStyle.Render(pollError)
	} else {
		line2 = dashConnStyle.Render("connected")
	}

	return dashHeaderStyle.Render(line1) + "\n" + line2
}

// renderDashboardFooter renders the 1-line footer with key hints and last update info.
func renderDashboardFooter(lastUpdate string, width int) string {
	left := "q Quit  s Sort  j/k Navigate  Enter Detail  Esc Back"
	right := ""
	if lastUpdate != "" {
		right = "Last update: " + lastUpdate
	}

	gap := max(1, width-len(left)-len(right))

	return dashFooterStyle.Render(left + strings.Repeat(" ", gap) + right)
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
		return header + "\n" + dashFooterStyle.Render("  no peers configured")
	}

	var sb strings.Builder
	sb.WriteString(renderTableHeader(cols, sortCol, sortAsc))
	sb.WriteString("\n")

	for i, p := range peers {
		if maxRows > 0 && i >= maxRows {
			break
		}
		row := renderPeerRow(p, cols, ds)
		if i == ds.resolveSelectedIndex(peers) {
			row = dashSelectedStyle.Render(row)
		}
		sb.WriteString(row)
		if i < len(peers)-1 && (maxRows <= 0 || i < maxRows-1) {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderTableHeader renders the column headers with sort indicator.
func renderTableHeader(cols []dashboardColumnDef, sortCol dashboardSortColumn, sortAsc bool) string {
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		name := c.col.String()
		if c.col == sortCol {
			if sortAsc {
				name += " ^"
			} else {
				name += " v"
			}
		}
		parts = append(parts, fmt.Sprintf("%-*s", c.width, name))
	}
	return dashHeaderStyle.Render(strings.Join(parts, "  "))
}

// renderPeerRow renders a single peer row.
func renderPeerRow(p dashboardPeer, cols []dashboardColumnDef, ds *dashboardState) string {
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		val := peerColumnValue(p, c.col, ds)
		padded := fmt.Sprintf("%-*s", c.width, val)
		parts = append(parts, padded)
	}
	return strings.Join(parts, "  ")
}

// peerColumnValue returns the display value for a peer in the given column.
func peerColumnValue(p dashboardPeer, col dashboardSortColumn, ds *dashboardState) string {
	switch col {
	case sortColumnAddress:
		return p.Address
	case sortColumnASN:
		return fmt.Sprintf("%d", p.RemoteAS)
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
	case "stopped", "idle":
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

	var sb strings.Builder
	sb.WriteString(dashHeaderStyle.Render(fmt.Sprintf("  Peer Detail: %s", peer.Address)))
	sb.WriteString("\n\n")

	rows := []struct{ label, value string }{
		{"Remote ASN", fmt.Sprintf("%d", peer.RemoteAS)},
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

	for _, r := range rows {
		fmt.Fprintf(&sb, "  %-16s %s\n", r.label, r.value)
	}

	sb.WriteString("\n")
	sb.WriteString(dashFooterStyle.Render("  Esc Back"))

	return sb.String()
}

// formatCounter formats an integer counter with thousands separators.
func formatCounter(n uint32) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
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
