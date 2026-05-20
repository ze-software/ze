// Design: plan/spec-web-4-interfaces.md -- Interface table and detail pages
// Related: workbench_table.go -- Reusable table component
// Related: workbench_detail.go -- Reusable detail panel component
// Related: handler_workbench.go -- Workbench handler that dispatches to this page

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// InterfaceTypes returns the list of interface types available for creation.
// Derived from the iface package's SupportedTypes, excluding loopback
// (singleton, not user-created).
func InterfaceTypes() []string {
	all := iface.SupportedTypes()
	result := make([]string, 0, len(all))
	for _, t := range all {
		if t == "loopback" {
			continue
		}
		result = append(result, t)
	}
	return result
}

// ifaceFlag computes the flag string for an interface row.
// R = link state "up".
func ifaceFlag(info iface.InterfaceInfo) (string, string) {
	if info.State == "up" {
		return "R", flagClassGreen
	}
	return ".", flagClassRed
}

// BuildInterfaceTableData constructs a WorkbenchTableData from a list of
// InterfaceInfo. filterType, when non-empty, restricts the table to
// interfaces matching that type.
func BuildInterfaceTableData(infos []iface.InterfaceInfo, filterType string) WorkbenchTableData {
	rates := iface.ListRates()

	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "state", Label: "Link State", Sortable: true},
		{Key: "mtu", Label: "MTU", Sortable: true},
		{Key: "mac", Label: "MAC"},
		{Key: "addresses", Label: "Addresses"},
		{Key: "rx-bps", Label: "RX bps", Sortable: true},
		{Key: "tx-bps", Label: "TX bps", Sortable: true},
		{Key: "rx-pps", Label: "RX pps", Sortable: true},
		{Key: "tx-pps", Label: "TX pps", Sortable: true},
	}

	var rows []WorkbenchTableRow
	for _, info := range infos {
		if filterType != "" && !matchesTypeFilter(info, filterType) {
			continue
		}

		flags, flagClass := ifaceFlag(info)

		addrs := make([]string, 0, len(info.Addresses))
		for _, a := range info.Addresses {
			var bAddr textbuf.Buffer
			addrs = append(addrs, bAddr.Reset().Str(a.Address).Byte('/').Int(int64(a.PrefixLength)).String())
		}
		addrStr := strings.Join(addrs, ", ")
		if addrStr == "" {
			addrStr = "-"
		}

		mac := info.MAC
		if mac == "" {
			mac = "-"
		}

		rxBps, txBps, rxPps, txPps := "-", "-", "-", "-"
		if rates != nil {
			if r, ok := rates[info.Name]; ok {
				rxBps = formatRate(r.RxBps)
				txBps = formatRate(r.TxBps)
				rxPps = formatRate(r.RxPps)
				txPps = formatRate(r.TxPps)
			}
		}

		rows = append(rows, WorkbenchTableRow{
			Key:       info.Name,
			URL:       "/show/iface/detail/" + info.Name,
			Flags:     flags,
			FlagClass: flagClass,
			Cells:     []string{info.Name, info.Type, info.State, strconv.Itoa(info.MTU), mac, addrStr, rxBps, txBps, rxPps, txPps},
			Actions: []WorkbenchRowAction{
				{Label: "Detail", URL: "/show/iface/detail/" + info.Name},
			},
		})
	}

	emptyMsg := "No interfaces found."
	if filterType != "" {
		emptyMsg = "No " + filterType + " interfaces found."
	}

	title := "All Interfaces"
	if filterType != "" {
		title = capitalizeFirst(filterType) + " Interfaces"
	}

	return WorkbenchTableData{
		Title:        title,
		AddURL:       "/show/iface/create",
		AddLabel:     "Add Interface",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: emptyMsg,
		EmptyHint:    "Add an interface to begin.",
	}
}

// matchesTypeFilter checks if an interface matches the given type filter.
// "vlan" matches interfaces with a VlanID > 0.
// "tunnel" matches both tunnel and wireguard types.
func matchesTypeFilter(info iface.InterfaceInfo, filterType string) bool {
	switch filterType {
	case "vlan":
		return info.VlanID > 0
	case "tunnel":
		return info.Type == "tunnel" || info.Type == "wireguard" ||
			info.Type == "gre" || info.Type == "gretap" ||
			info.Type == "ip6gre" || info.Type == "ip6gretap" ||
			info.Type == "ipip" || info.Type == "sit" ||
			info.Type == "ip6tnl"
	default:
		return info.Type == filterType
	}
}

// BuildInterfaceDetailData constructs a WorkbenchDetailData for a single
// interface, showing config, status, and traffic counter tabs.
func BuildInterfaceDetailData(info *iface.InterfaceInfo) WorkbenchDetailData {
	configHTML := buildDetailConfigHTML(info)
	statusHTML := buildDetailStatusHTML(info)
	countersHTML := buildDetailCountersHTML(info)

	tabs := []WorkbenchDetailTab{
		{Key: "config", Label: "Configuration", Content: configHTML, Active: true},
		{Key: "status", Label: "Status", Content: statusHTML},
		{Key: "counters", Label: "Traffic Counters", Content: countersHTML},
	}

	tools := []WorkbenchDetailTool{
		{Label: "Clear Counters", HxPost: "/admin/iface/clear-counters/" + info.Name, Class: "danger", Confirm: "Clear counters for " + info.Name + "?"},
	}

	return WorkbenchDetailData{
		Title:    info.Name,
		Tabs:     tabs,
		CloseURL: "/show/iface/",
		Tools:    tools,
	}
}

func buildDetailConfigHTML(info *iface.InterfaceInfo) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="wb-detail-section">`)
	b.WriteString(`<table class="wb-detail-kv">`)
	writeKV(&b, "Name", info.Name)
	writeKV(&b, "Type", info.Type)
	writeKV(&b, "MTU", strconv.Itoa(info.MTU))
	if info.MAC != "" {
		writeKV(&b, "MAC", info.MAC)
	}
	writeKV(&b, "Admin State", info.State)
	b.WriteString(`</table>`)

	if len(info.Addresses) > 0 {
		b.WriteString(`<h4>Addresses</h4><ul>`)
		for _, addr := range info.Addresses {
			fmt.Fprintf(&b, `<li>%s/%d (%s)</li>`,
				template.HTMLEscapeString(addr.Address),
				addr.PrefixLength,
				template.HTMLEscapeString(addr.Family))
		}
		b.WriteString(`</ul>`)
	}

	b.WriteString(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

func buildDetailStatusHTML(info *iface.InterfaceInfo) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="wb-detail-section">`)
	b.WriteString(`<table class="wb-detail-kv">`)
	writeKV(&b, "Link State", info.State)
	writeKV(&b, "Index", strconv.Itoa(info.Index))
	writeKV(&b, "MTU (actual)", strconv.Itoa(info.MTU))
	if info.ParentIndex > 0 {
		writeKV(&b, "Parent Index", strconv.Itoa(info.ParentIndex))
	}
	if info.VlanID > 0 {
		writeKV(&b, "VLAN ID", strconv.Itoa(info.VlanID))
	}
	b.WriteString(`</table>`)
	b.WriteString(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

func buildDetailCountersHTML(info *iface.InterfaceInfo) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="wb-detail-section"`)
	fmt.Fprintf(&b, ` hx-get="/show/iface/counters/%s" hx-trigger="every 3s" hx-swap="innerHTML"`,
		template.HTMLEscapeString(info.Name))
	b.WriteString(`>`)
	b.WriteString(formatCountersTable(info.Stats, info.Name))
	b.WriteString(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

func formatCountersTable(stats *iface.InterfaceStats, name string) string {
	var b strings.Builder
	b.WriteString(`<table class="wb-detail-kv">`)
	if stats != nil {
		writeKV(&b, "RX Bytes", strconv.Itoa(int(stats.RxBytes)))
		writeKV(&b, "RX Packets", strconv.Itoa(int(stats.RxPackets)))
		writeKV(&b, "RX Errors", strconv.Itoa(int(stats.RxErrors)))
		writeKV(&b, "RX Dropped", strconv.Itoa(int(stats.RxDropped)))
		writeKV(&b, "TX Bytes", strconv.Itoa(int(stats.TxBytes)))
		writeKV(&b, "TX Packets", strconv.Itoa(int(stats.TxPackets)))
		writeKV(&b, "TX Errors", strconv.Itoa(int(stats.TxErrors)))
		writeKV(&b, "TX Dropped", strconv.Itoa(int(stats.TxDropped)))
	} else {
		writeKV(&b, "Counters", "not available")
	}
	if r, ok := iface.GetRate(name); ok {
		writeKV(&b, "RX bps", formatRate(r.RxBps))
		writeKV(&b, "TX bps", formatRate(r.TxBps))
		writeKV(&b, "RX pps", formatRate(r.RxPps))
		writeKV(&b, "TX pps", formatRate(r.TxPps))
	}
	b.WriteString(`</table>`)
	return b.String()
}

// capitalizeFirst returns the string with its first rune uppercased.
// Used instead of the deprecated strings.Title for single-word labels.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func formatRate(v float64) string {
	if v < 0.1 {
		return "0"
	}
	if v < 10 {
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'f', 0, 64)
}

func writeKV(b *strings.Builder, key, value string) {
	b.WriteString(`<tr><td class="wb-detail-kv-key">`)
	b.WriteString(template.HTMLEscapeString(key))
	b.WriteString(`</td><td class="wb-detail-kv-val">`)
	b.WriteString(template.HTMLEscapeString(value))
	b.WriteString(`</td></tr>`)
}

// HandleInterfacesPage renders the interface list table within the workbench.
// It is called by the workbench handler when the path starts with "iface/".
// Returns the rendered HTML content for embedding in the workbench shell.
func HandleInterfacesPage(renderer *Renderer, r *http.Request, path []string) template.HTML {
	filterType := r.URL.Query().Get("type")

	// Detail sub-path: /show/iface/detail/<name>
	if len(path) >= 2 && path[0] == "detail" {
		return handleInterfaceDetailContent(renderer, path[1])
	}

	// Counters sub-path: /show/iface/counters/<name> (HTMX partial for auto-refresh)
	if len(path) >= 2 && path[0] == "counters" {
		return handleInterfaceCountersContent(path[1])
	}

	// Traffic sub-path: /show/iface/traffic/
	if len(path) >= 1 && path[0] == "traffic" {
		return BuildTrafficPageContent(renderer)
	}

	infos, err := iface.ListInterfaces()
	if err != nil {
		// Backend not loaded: show empty table gracefully.
		tableData := BuildInterfaceTableData(nil, filterType)
		return renderer.RenderFragment("workbench_table", tableData)
	}

	tableData := BuildInterfaceTableData(infos, filterType)
	return renderer.RenderFragment("workbench_table", tableData)
}

func handleInterfaceDetailContent(renderer *Renderer, name string) template.HTML {
	info, err := iface.GetInterface(name)
	if err != nil || info == nil {
		return template.HTML(`<div class="wb-detail-panel"><p>Interface not found.</p></div>`) //nolint:gosec // static HTML
	}

	detailData := BuildInterfaceDetailData(info)
	return renderer.RenderFragment("workbench_detail", detailData)
}

func handleInterfaceCountersContent(name string) template.HTML {
	stats, err := iface.GetStats(name)
	if err != nil {
		return template.HTML(formatCountersTable(nil, name)) //nolint:gosec // trusted builder output
	}
	return template.HTML(formatCountersTable(stats, name)) //nolint:gosec // trusted builder output
}
