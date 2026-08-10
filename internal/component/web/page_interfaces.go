// Design: docs/architecture/web-workbench-pages.md -- Interface table and detail pages
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

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// interfaceTypes returns the list of interface types available for creation.
// Derived from the iface package's SupportedTypes, excluding loopback
// (singleton, not user-created).
func interfaceTypes() []string {
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

const (
	interfaceStateConfigured = "configured"
	interfaceTypeVLAN        = "vlan"
	defaultInterfaceMTU      = 1500
)

func buildInterfaceTableDataForView(runtime []iface.InterfaceInfo, viewTree *config.Tree, filterType string) WorkbenchTableData {
	infos := mergeInterfaceInfos(runtime, collectConfiguredInterfaces(viewTree))
	return BuildInterfaceTableData(infos, filterType)
}

func collectConfiguredInterfaces(viewTree *config.Tree) []iface.InterfaceInfo {
	if viewTree == nil {
		return nil
	}
	ifaceTree := viewTree.GetContainer("interface")
	if ifaceTree == nil {
		return nil
	}

	var infos []iface.InterfaceInfo
	for _, typ := range iface.SupportedTypes() {
		if typ == "loopback" {
			if loop := ifaceTree.GetContainer("loopback"); loop != nil {
				infos = append(infos, configuredInterfaceInfo("loopback", typ, loop))
			}
			continue
		}
		for _, entry := range ifaceTree.GetListOrdered(typ) {
			info := configuredInterfaceInfo(entry.Key, typ, entry.Value)
			infos = append(infos, info)
			infos = append(infos, configuredVLANInfos(info, entry.Value)...)
		}
	}
	return infos
}

func configuredInterfaceInfo(name, typ string, entry *config.Tree) iface.InterfaceInfo {
	info := iface.InterfaceInfo{
		Name:  name,
		Type:  typ,
		State: interfaceStateConfigured,
		MTU:   defaultInterfaceMTU,
	}
	if entry == nil {
		return info
	}
	if mtu, ok := entry.Get("mtu"); ok {
		if v, err := strconv.Atoi(mtu); err == nil && v > 0 {
			info.MTU = v
		}
	}
	if mac := entry.GetContainer("mac"); mac != nil {
		if address, ok := mac.Get("address"); ok {
			info.MAC = address
		}
	}
	info.Addresses = configuredInterfaceAddresses(entry)
	return info
}

func configuredVLANInfos(parent iface.InterfaceInfo, entry *config.Tree) []iface.InterfaceInfo {
	if entry == nil {
		return nil
	}
	var infos []iface.InterfaceInfo
	for _, unit := range entry.GetListOrdered("unit") {
		vlanID := configuredUnitVLANID(unit.Value)
		if vlanID <= 0 {
			continue
		}
		var tb textbuf.Buffer
		name := tb.Str(parent.Name).Byte('.').Int(int64(vlanID)).String()
		infos = append(infos, iface.InterfaceInfo{
			Name:      name,
			Type:      interfaceTypeVLAN,
			State:     interfaceStateConfigured,
			MTU:       parent.MTU,
			MAC:       parent.MAC,
			Addresses: configuredUnitAddresses(unit.Value),
			VlanID:    vlanID,
		})
	}
	return infos
}

func configuredInterfaceAddresses(entry *config.Tree) []iface.AddrInfo {
	var addrs []iface.AddrInfo
	for _, unit := range entry.GetListOrdered("unit") {
		if configuredUnitVLANID(unit.Value) > 0 {
			continue
		}
		addrs = append(addrs, configuredUnitAddresses(unit.Value)...)
	}
	return addrs
}

func configuredUnitVLANID(unit *config.Tree) int {
	if unit == nil {
		return 0
	}
	raw, ok := unit.Get("vlan-id")
	if !ok {
		return 0
	}
	vlanID, err := strconv.Atoi(raw)
	if err != nil || vlanID <= 0 {
		return 0
	}
	return vlanID
}

func configuredUnitAddresses(unit *config.Tree) []iface.AddrInfo {
	if unit == nil {
		return nil
	}
	var addrs []iface.AddrInfo
	for _, family := range []string{"ipv4", "ipv6"} {
		familyTree := unit.GetContainer(family)
		if familyTree == nil {
			continue
		}
		for _, cidr := range familyTree.GetSlice("address") {
			addrs = append(addrs, configuredAddrInfo(cidr, family))
		}
	}
	return addrs
}

func configuredAddrInfo(cidr, family string) iface.AddrInfo {
	address, prefixRaw, ok := strings.Cut(cidr, "/")
	if !ok {
		return iface.AddrInfo{Address: cidr, Family: family}
	}
	prefix, err := strconv.Atoi(prefixRaw)
	if err != nil {
		return iface.AddrInfo{Address: address, Family: family}
	}
	return iface.AddrInfo{Address: address, PrefixLength: prefix, Family: family}
}

func mergeInterfaceInfos(runtime, configured []iface.InterfaceInfo) []iface.InterfaceInfo {
	if len(configured) == 0 {
		return runtime
	}
	byName := make(map[string]int, len(runtime))
	byMAC := make(map[string]int, len(runtime))
	for i := range runtime {
		byName[runtime[i].Name] = i
		if runtime[i].MAC != "" {
			byMAC[strings.ToLower(runtime[i].MAC)] = i
		}
	}

	usedRuntime := make([]bool, len(runtime))
	infos := make([]iface.InterfaceInfo, 0, len(configured)+len(runtime))
	for i := range configured {
		cfg := &configured[i]
		info := *cfg
		if idx, ok := byName[cfg.Name]; ok {
			info = mergeConfiguredRuntime(*cfg, runtime[idx])
			usedRuntime[idx] = true
		} else if cfg.MAC != "" {
			if idx, ok := byMAC[strings.ToLower(cfg.MAC)]; ok {
				info = mergeConfiguredRuntime(*cfg, runtime[idx])
				info.Name = cfg.Name
				usedRuntime[idx] = true
			}
		}
		infos = append(infos, info)
	}
	for i := range runtime {
		if !usedRuntime[i] {
			infos = append(infos, runtime[i])
		}
	}
	return infos
}

func mergeConfiguredRuntime(cfg, runtime iface.InterfaceInfo) iface.InterfaceInfo {
	info := runtime
	info.Name = cfg.Name
	if cfg.Type != "" {
		info.Type = cfg.Type
	}
	if info.State == "" {
		info.State = cfg.State
	}
	if info.MTU == 0 {
		info.MTU = cfg.MTU
	}
	if info.MAC == "" {
		info.MAC = cfg.MAC
	}
	if len(info.Addresses) == 0 {
		info.Addresses = cfg.Addresses
	}
	if info.VlanID == 0 {
		info.VlanID = cfg.VlanID
	}
	return info
}

func normalizeInterfaceInfo(info iface.InterfaceInfo) iface.InterfaceInfo {
	if info.VlanID > 0 || info.Type == interfaceTypeVLAN {
		return info
	}
	if typ := iface.CanonicalInterfaceType(&info); typ != "" {
		info.Type = typ
	}
	return info
}

// ifaceFlag computes the flag string for an interface row.
// R = live link up, C = configured but not present in live state.
func ifaceFlag(info iface.InterfaceInfo) (string, string) {
	switch info.State {
	case "up":
		return "R", flagClassGreen
	case interfaceStateConfigured:
		return "C", flagClassYellow
	default:
		return ".", flagClassRed
	}
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
	for i := range infos {
		info := normalizeInterfaceInfo(infos[i])
		if filterType != "" && !matchesTypeFilter(info, filterType) {
			continue
		}

		flags, flagClass := ifaceFlag(info)

		addrs := make([]string, 0, len(info.Addresses))
		for _, a := range info.Addresses {
			var bAddr textbuf.Buffer
			bAddr.Str(a.Address)
			if a.PrefixLength > 0 {
				bAddr.Byte('/').Int(int64(a.PrefixLength))
			}
			addrs = append(addrs, bAddr.String())
		}
		addrStr := textbuf.Join(addrs, ", ")
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

		var tb textbuf.Buffer
		detailURL := tb.Str("/show/iface/detail/").Str(info.Name).String()
		rows = append(rows, WorkbenchTableRow{
			Key:       info.Name,
			URL:       detailURL,
			Flags:     flags,
			FlagClass: flagClass,
			Cells:     []string{info.Name, info.Type, info.State, strconv.Itoa(info.MTU), mac, addrStr, rxBps, txBps, rxPps, txPps},
			Actions: []WorkbenchRowAction{
				{Label: "Detail", URL: detailURL},
			},
		})
	}

	var tb textbuf.Buffer
	emptyMsg := "No interfaces found."
	title := "All Interfaces"
	if filterType != "" {
		emptyMsg = tb.Str("No ").Str(filterType).Str(" interfaces found.").String()
		title = tb.Reset().Str(capitalizeFirst(filterType)).Str(" Interfaces").String()
	}

	return WorkbenchTableData{
		Title:        title,
		AddURL:       "/show/interface/ethernet/",
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
	typ := interfaceDisplayType(info)
	switch filterType {
	case interfaceTypeVLAN:
		return typ == interfaceTypeVLAN || info.VlanID > 0
	case "tunnel":
		return typ == "tunnel" || typ == "wireguard" ||
			info.Type == "gre" || info.Type == "gretap" ||
			info.Type == "ip6gre" || info.Type == "ip6gretap" ||
			info.Type == "ipip" || info.Type == "sit" ||
			info.Type == "ip6tnl"
	default:
		return typ == filterType
	}
}

func interfaceDisplayType(info iface.InterfaceInfo) string {
	return normalizeInterfaceInfo(info).Type
}

// buildInterfaceDetailData constructs a WorkbenchDetailData for a single
// interface, showing config, status, and traffic counter tabs.
func buildInterfaceDetailData(info *iface.InterfaceInfo) WorkbenchDetailData {
	configHTML := buildDetailConfigHTML(info)
	statusHTML := buildDetailStatusHTML(info)
	countersHTML := buildDetailCountersHTML(info)

	tabs := []WorkbenchDetailTab{
		{Key: "config", Label: "Configuration", Content: configHTML, Active: true},
		{Key: "status", Label: "Status", Content: statusHTML},
		{Key: "counters", Label: "Traffic Counters", Content: countersHTML},
	}

	var tb2 textbuf.Buffer
	tools := []WorkbenchDetailTool{
		{Label: "Clear Counters", HxPost: tb2.Str("/admin/iface/clear-counters/").Str(info.Name).String(), Class: "danger", Confirm: tb2.Reset().Str("Clear counters for ").Str(info.Name).Byte('?').String()},
	}

	return WorkbenchDetailData{
		Title:    info.Name,
		Tabs:     tabs,
		CloseURL: "/show/iface/",
		Tools:    tools,
	}
}

func buildDetailConfigHTML(info *iface.InterfaceInfo) template.HTML {
	var b textbuf.Buffer
	b.Str(`<div class="wb-detail-section">`)
	b.Str(`<table class="wb-detail-kv">`)
	writeKV(&b, "Name", info.Name)
	writeKV(&b, "Type", info.Type)
	writeKV(&b, "MTU", strconv.Itoa(info.MTU))
	if info.MAC != "" {
		writeKV(&b, "MAC", info.MAC)
	}
	writeKV(&b, "Admin State", info.State)
	b.Str(`</table>`)

	if len(info.Addresses) > 0 {
		b.Str(`<h4>Addresses</h4><ul>`)
		for _, addr := range info.Addresses {
			fmt.Fprintf(&b, `<li>%s/%d (%s)</li>`, //nolint:errcheck // report output
				template.HTMLEscapeString(addr.Address),
				addr.PrefixLength,
				template.HTMLEscapeString(addr.Family))
		}
		b.Str(`</ul>`)
	}

	b.Str(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

func buildDetailStatusHTML(info *iface.InterfaceInfo) template.HTML {
	var b textbuf.Buffer
	b.Str(`<div class="wb-detail-section">`)
	b.Str(`<table class="wb-detail-kv">`)
	writeKV(&b, "Link State", info.State)
	writeKV(&b, "Index", strconv.Itoa(info.Index))
	writeKV(&b, "MTU (actual)", strconv.Itoa(info.MTU))
	if info.ParentIndex > 0 {
		writeKV(&b, "Parent Index", strconv.Itoa(info.ParentIndex))
	}
	if info.VlanID > 0 {
		writeKV(&b, "VLAN ID", strconv.Itoa(info.VlanID))
	}
	b.Str(`</table>`)
	b.Str(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

func buildDetailCountersHTML(info *iface.InterfaceInfo) template.HTML {
	var b textbuf.Buffer
	b.Str(`<div class="wb-detail-section"`)
	fmt.Fprintf(&b, ` hx-get="/show/iface/counters/%s" hx-trigger="every 3s" hx-swap="innerHTML"`, //nolint:errcheck // report output
		template.HTMLEscapeString(info.Name))
	b.Str(`>`)
	b.Str(formatCountersTable(info.Stats, info.Name))
	b.Str(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

func formatCountersTable(stats *iface.InterfaceStats, name string) string {
	var b textbuf.Buffer
	b.Str(`<table class="wb-detail-kv">`)
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
	b.Str(`</table>`)
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

func writeKV(b *textbuf.Buffer, key, value string) {
	b.Str(`<tr><td class="wb-detail-kv-key">`)
	b.Str(template.HTMLEscapeString(key))
	b.Str(`</td><td class="wb-detail-kv-val">`)
	b.Str(template.HTMLEscapeString(value))
	b.Str(`</td></tr>`)
}

// handleInterfacesPage renders the interface list table within the workbench.
// It is called by the workbench handler when the path starts with "iface/".
// Returns the rendered HTML content for embedding in the workbench shell.
func handleInterfacesPage(renderer *Renderer, r *http.Request, path []string, viewTree *config.Tree) template.HTML {
	filterType := r.URL.Query().Get("type")

	// Detail sub-path: /show/iface/detail/<name>
	if len(path) >= 2 && path[0] == "detail" {
		return handleInterfaceDetailContent(renderer, path[1], viewTree)
	}

	// Counters sub-path: /show/iface/counters/<name> (HTMX partial for auto-refresh)
	if len(path) >= 2 && path[0] == "counters" {
		return handleInterfaceCountersContent(path[1])
	}

	// Traffic sub-path: /show/iface/traffic/
	if len(path) >= 1 && path[0] == "traffic" {
		return buildTrafficPageContent(renderer)
	}

	infos, err := iface.ListInterfaces()
	if err != nil {
		// Backend not loaded: still show configured entries from the editor tree.
		tableData := buildInterfaceTableDataForView(nil, viewTree, filterType)
		return renderer.RenderFragment("workbench_table", tableData)
	}

	tableData := buildInterfaceTableDataForView(infos, viewTree, filterType)
	return renderer.RenderFragment("workbench_table", tableData)
}

func handleInterfaceDetailContent(renderer *Renderer, name string, viewTree *config.Tree) template.HTML {
	info, err := iface.GetInterface(name)
	if err != nil || info == nil {
		info = configuredInterfaceByName(viewTree, name)
	}
	if info == nil {
		return template.HTML(`<div class="wb-detail-panel"><p>Interface not found.</p></div>`) //nolint:gosec // static HTML
	}

	detailData := buildInterfaceDetailData(info)
	return renderer.RenderFragment("workbench_detail", detailData)
}

func configuredInterfaceByName(viewTree *config.Tree, name string) *iface.InterfaceInfo {
	infos := collectConfiguredInterfaces(viewTree)
	for i := range infos {
		if infos[i].Name == name {
			found := infos[i]
			return &found
		}
	}
	return nil
}

func handleInterfaceCountersContent(name string) template.HTML {
	stats, err := iface.GetStats(name)
	if err != nil {
		return template.HTML(formatCountersTable(nil, name)) //nolint:gosec // trusted builder output
	}
	return template.HTML(formatCountersTable(stats, name)) //nolint:gosec // trusted builder output
}
