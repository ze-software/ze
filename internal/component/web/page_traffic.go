// Design: docs/architecture/web-workbench-pages.md -- Traffic monitoring page
// Related: page_interfaces.go -- Interface table page (sibling)
// Related: workbench_table.go -- Reusable table component

package web

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/component/iface"
)

// trafficRow holds one row of the traffic monitoring table.
type trafficRow struct {
	Interface string
	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	RxDropped uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
	TxDropped uint64
	TotalRate uint64 // RxBytes + TxBytes, used for sorting
}

// buildTrafficTableData constructs a WorkbenchTableData for the traffic page.
// Rows are sorted by total traffic (RX+TX bytes) descending.
func buildTrafficTableData(infos []iface.InterfaceInfo) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "interface", Label: "Interface", Sortable: true},
		{Key: "rx-bytes", Label: "RX Bytes", Sortable: true},
		{Key: "rx-packets", Label: "RX Packets", Sortable: true},
		{Key: "rx-errors", Label: "RX Errors", Sortable: true},
		{Key: "rx-dropped", Label: "RX Drops", Sortable: true},
		{Key: "tx-bytes", Label: "TX Bytes", Sortable: true},
		{Key: "tx-packets", Label: "TX Packets", Sortable: true},
		{Key: "tx-errors", Label: "TX Errors", Sortable: true},
		{Key: "tx-dropped", Label: "TX Drops", Sortable: true},
	}

	rows := buildTrafficRows(infos)

	return WorkbenchTableData{
		Title:        "Traffic",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No interfaces to monitor.",
		EmptyHint:    "Interfaces with traffic counters will appear here.",
	}
}

func buildTrafficRows(infos []iface.InterfaceInfo) []WorkbenchTableRow {
	// Build sortable traffic rows.
	tRows := make([]trafficRow, 0, len(infos))
	for i := range infos {
		info := &infos[i]
		tr := trafficRow{Interface: info.Name}
		if info.Stats != nil {
			tr.RxBytes = info.Stats.RxBytes
			tr.RxPackets = info.Stats.RxPackets
			tr.RxErrors = info.Stats.RxErrors
			tr.RxDropped = info.Stats.RxDropped
			tr.TxBytes = info.Stats.TxBytes
			tr.TxPackets = info.Stats.TxPackets
			tr.TxErrors = info.Stats.TxErrors
			tr.TxDropped = info.Stats.TxDropped
			tr.TotalRate = info.Stats.RxBytes + info.Stats.TxBytes
		}
		tRows = append(tRows, tr)
	}

	// Sort by total rate descending.
	sort.Slice(tRows, func(i, j int) bool {
		return tRows[i].TotalRate > tRows[j].TotalRate
	})

	rows := make([]WorkbenchTableRow, 0, len(tRows))
	for _, tr := range tRows {
		rows = append(rows, WorkbenchTableRow{
			Key:   tr.Interface,
			URL:   "/show/iface/detail/" + tr.Interface,
			Cells: trafficCells(tr),
		})
	}
	return rows
}

func trafficCells(tr trafficRow) []string {
	return []string{
		tr.Interface,
		strconv.Itoa(int(tr.RxBytes)),
		strconv.Itoa(int(tr.RxPackets)),
		strconv.Itoa(int(tr.RxErrors)),
		strconv.Itoa(int(tr.RxDropped)),
		strconv.Itoa(int(tr.TxBytes)),
		strconv.Itoa(int(tr.TxPackets)),
		strconv.Itoa(int(tr.TxErrors)),
		strconv.Itoa(int(tr.TxDropped)),
	}
}

// buildTrafficPageContent renders the traffic page content as HTML.
// The outer div uses hx-get with a 5-second polling interval so traffic
// counters refresh automatically without a full page reload.
func buildTrafficPageContent(renderer *Renderer) template.HTML {
	infos, _ := iface.ListInterfaces()
	tableData := buildTrafficTableData(infos)

	return renderer.renderComponent("traffic_panel",
		pollPanel("/show/iface/traffic/", workbenchTable(tableData)))
}

// HandleTrafficPage renders the traffic monitoring page for direct HTTP responses.
func HandleTrafficPage(renderer *Renderer, w http.ResponseWriter, _ *http.Request) {
	content := buildTrafficPageContent(renderer)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(content)); err != nil {
		http.Error(w, "write error", http.StatusInternalServerError)
	}
}
