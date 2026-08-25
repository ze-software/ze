// Design: docs/architecture/api/commands.md — dashboard rendering.
// Overview: model.go — editor model and update loop.
// Related: model_dashboard.go — dashboard state and lifecycle.
// Related: model_dashboard_sort.go — sort column enum and sort logic.

package cli

import (
	"image/color"
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Dashboard styles.
var (
	dashHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")) // white
	dashFooterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))           // dim gray
	dashSelectedStyle = lipgloss.NewStyle().Bold(true)                                  // selection, no color of its own
	dashErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))             // red for errors
	dashConnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))             // green for connected
)

// Peer-state colors. These are colors rather than styles because the peer table
// merges the state color into the row's own style, and a style can only be
// merged from the parts it is built out of.
var (
	dashStateGreen  = lipgloss.Color("2") // established
	dashStateYellow = lipgloss.Color("3") // negotiating
	dashStateRed    = lipgloss.Color("1") // down
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

// renderDashboardHeader renders the 1-line header bar.
//
// It states BGP facts and nothing else.
//
// It used to carry a second line reading `connected`, the CLI's own poll
// status. Under `peers 3/3` that read as a statement about the sessions, and
// it said nothing the peer rows did not say. A poll fault now goes to the
// error zone the Model renders for every view
// (docs/architecture/cli/error-surface.md).
func renderDashboardHeader(snap *dashboardSnapshot, width int) string {
	var tb textbuf.Buffer
	if snap == nil {
		return dashHeaderStyle.Render("BGP Dashboard  waiting for data...")
	}

	line1 := tb.Str("AS ").Uint32(snap.LocalAS).Str("  rid ").Str(snap.RouterID).Str("  up ").Str(snap.Uptime).Str("  peers ").Int(int64(snap.PeersEstablished)).Byte('/').Int(int64(snap.PeersConfigured)).String()
	if width > 0 && len(line1) > width {
		line1 = line1[:width]
	}

	return dashHeaderStyle.Render(line1)
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

	// The selection marker's gutter is part of what a row occupies, so it is
	// counted before any column is judged to fit. Leaving it out let the table
	// run selectionMarkerWidth past the terminal, and the wrapped tail then
	// overwrote the line under it.
	total := selectionMarkerWidth
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
		selected := i == ds.resolveSelectedIndex(peers)
		row := renderPeerRow(p, cols, ds, selected)
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
	// selectionMarkerWidth of leading blanks keeps the headers over the cells
	// the marker shifts right.
	return dashHeaderStyle.Render(tb.Reset().Repeat(" ", selectionMarkerWidth).Str(textbuf.Join(parts, "  ")).String())
}

// renderPeerRow renders a single peer row.
//
// Each cell is padded BEFORE it is styled. peerColumnValue answers plain text.
// The style is applied to the padded cell, so textbuf.PadRight counts content
// rather than escapes.
//
// Padding a cell that already carried an ANSI escape counted the escape as
// content. The count exceeded the column, so PadRight added nothing. Every
// column after State then sat left of its header
// for `established` and eight left for `idle`.
func renderPeerRow(p dashboardPeer, cols []dashboardColumnDef, ds *dashboardState, selected bool) string {
	var tb textbuf.Buffer
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		val := peerColumnValue(p, c.col, ds)
		tb.Reset().PadRight(val, c.width)
		cell := tb.String()

		// The selection is a marker and weight, and paints no background.
		//
		// A background must be chosen against a palette the terminal owns and
		// the website re-declares. Every choice put a cell's own color on top
		// of it. The state green landed on the selection cyan at 1.29:1.
		// Pinning a foreground won that back and cost the state its color.
		//
		// Weight reads whatever the palette says, so every cell keeps the
		// color it would have had.
		style := lipgloss.NewStyle()
		if selected {
			style = dashSelectedStyle
		}
		if fg, ok := stateColor(p.State); ok && c.col == sortColumnState {
			style = style.Foreground(fg)
		}
		parts = append(parts, style.Render(cell))
	}
	marker := "  "
	if selected {
		marker = dashSelectedStyle.Render("> ")
	}
	return tb.Reset().Str(marker).Str(textbuf.Join(parts, "  ")).String()
}

// selectionMarkerWidth is the gutter renderPeerRow writes the selection marker
// into, and the blanks renderTableHeader writes so the headers stay over their
// cells.
const selectionMarkerWidth = 2

// peerColumnValue returns the display value for a peer in the given column.
func peerColumnValue(p dashboardPeer, col dashboardSortColumn, ds *dashboardState) string {
	switch col {
	case sortColumnAddress:
		return p.Address
	case sortColumnASN:
		return strconv.Itoa(int(p.RemoteAS))
	case sortColumnState:
		return p.State
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

// stateColor returns the color a peer state is shown in, and whether it has one.
//
// It answers a COLOR rather than a rendered string on purpose. Rendering here
// emitted a full reset after the text, and on the selected row that reset landed
// inside the selection background and ended it, so every column after State fell
// back to the terminal default. The peer table merges this color into the row's
// own style, so one escape carries both and nothing resets mid-row.
func stateColor(state string) (color.Color, bool) {
	switch state {
	case "established":
		return dashStateGreen, true
	case "connecting", "active", "opensent", "openconfirm":
		return dashStateYellow, true
	case "stopped", "idle", "idle-hold":
		return dashStateRed, true
	}
	return nil, false
}

// stateStyled returns the state string with its color applied. The detail view
// uses it: that surface renders one field at a time and has no row style for the
// color to be merged into.
func stateStyled(state string) string {
	fg, ok := stateColor(state)
	if !ok {
		return state
	}
	return lipgloss.NewStyle().Foreground(fg).Render(state)
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
	// Indented to three, which is where every row under it starts.
	//
	// It is NOT where the peer table's header starts. That header opens with
	// the selection marker's two-space gutter and then "Peer". A two-space
	// indent here made the first seven characters of the two lines identical,
	// and a differential repaint then left them from the previous frame.
	sb.Str(dashHeaderStyle.Render(tb.Str("   Peer Detail: ").Str(peer.Address).String()))
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
