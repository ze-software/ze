// Design: docs/architecture/web-workbench-pages.md -- Interface table and detail pages
// Related: workbench_table.go -- Reusable table component
// Related: workbench_detail.go -- Reusable detail panel component
// Related: handler_workbench.go -- Workbench handler that dispatches to this page

package web

import (
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

	// The address family names the config tree and iface.AddrInfo both use.
	familyIPv4 = "ipv4"
	familyIPv6 = "ipv6"
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
	for _, family := range []string{familyIPv4, familyIPv6} {
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
		{Key: colName, Label: labelName, Sortable: true},
		{Key: colType, Label: labelType, Sortable: true},
		{Key: colState, Label: "Link State", Sortable: true},
		{Key: "mtu", Label: labelMTU, Sortable: true},
		{Key: "mac", Label: labelMAC},
		{Key: segAddresses, Label: "Addresses"},
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
				{Label: labelDetail, URL: detailURL},
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
	case segTunnel:
		return typ == segTunnel || typ == "wireguard" ||
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
func buildInterfaceDetailData(renderer *Renderer, info *iface.InterfaceInfo) WorkbenchDetailData {
	configHTML := buildDetailConfigHTML(renderer, info)
	statusHTML := buildDetailStatusHTML(renderer, info)
	countersHTML := buildDetailCountersHTML(renderer, info)

	tabs := []WorkbenchDetailTab{
		{Key: tabConfig, Label: "Configuration", Content: configHTML, Active: true},
		{Key: tabStatus, Label: labelStatus, Content: statusHTML},
		{Key: "counters", Label: "Traffic Counters", Content: countersHTML},
	}

	var tb2 textbuf.Buffer
	tools := []WorkbenchDetailTool{
		{Label: "Clear Counters", HxPost: tb2.Str("/admin/iface/clear-counters/").Str(info.Name).String(), Class: wbToolDanger, Confirm: tb2.Reset().Str("Clear counters for ").Str(info.Name).Byte('?').String()},
	}

	return WorkbenchDetailData{
		Title:    info.Name,
		Tabs:     tabs,
		CloseURL: ifacePathPrefix,
		Tools:    tools,
	}
}

// detailKV is one row of a wb-detail-kv table: a label and the value beside it.
// The interface, BGP peer and system panels all draw that table.
type detailKV struct {
	Key   string
	Value string
}

// ifaceAddressRow is one address of an interface, as the config panel lists it.
type ifaceAddressRow struct {
	Address string
	Prefix  int
	Family  string
}

// ifaceConfigData is what ifaceDetailConfig renders. The address list is drawn
// only when the interface holds one.
type ifaceConfigData struct {
	Rows      []detailKV
	Addresses []ifaceAddressRow
}

// ifaceCountersData is what ifaceDetailCounters renders. Name is the interface
// the panel polls, which is what advances the counters while the tab is open.
type ifaceCountersData struct {
	Name string
	Rows []detailKV
}

// ifaceCountersURL is the endpoint the counters panel re-reads.
func ifaceCountersURL(name string) string {
	var tb textbuf.Buffer

	return tb.Str("/show/iface/counters/").Str(name).String()
}

func buildDetailConfigHTML(renderer *Renderer, info *iface.InterfaceInfo) template.HTML {
	rows := []detailKV{
		{Key: labelName, Value: info.Name},
		{Key: "Type", Value: info.Type},
		{Key: labelMTU, Value: strconv.Itoa(info.MTU)},
	}
	if info.MAC != "" {
		rows = append(rows, detailKV{Key: labelMAC, Value: info.MAC})
	}

	rows = append(rows, detailKV{Key: "Admin State", Value: info.State})

	addresses := make([]ifaceAddressRow, 0, len(info.Addresses))
	for _, addr := range info.Addresses {
		addresses = append(addresses, ifaceAddressRow{
			Address: addr.Address,
			Prefix:  addr.PrefixLength,
			Family:  addr.Family,
		})
	}

	return renderer.renderComponent("iface_detail_config",
		ifaceDetailConfig(ifaceConfigData{Rows: rows, Addresses: addresses}))
}

func buildDetailStatusHTML(renderer *Renderer, info *iface.InterfaceInfo) template.HTML {
	rows := []detailKV{
		{Key: "Link State", Value: info.State},
		{Key: "Index", Value: strconv.Itoa(info.Index)},
		{Key: "MTU (actual)", Value: strconv.Itoa(info.MTU)},
	}
	if info.ParentIndex > 0 {
		rows = append(rows, detailKV{Key: "Parent Index", Value: strconv.Itoa(info.ParentIndex)})
	}

	if info.VlanID > 0 {
		rows = append(rows, detailKV{Key: "VLAN ID", Value: strconv.Itoa(info.VlanID)})
	}

	return renderer.renderComponent("iface_detail_status", detailKVSection(rows))
}

func buildDetailCountersHTML(renderer *Renderer, info *iface.InterfaceInfo) template.HTML {
	return renderer.renderComponent("iface_detail_counters",
		ifaceDetailCounters(ifaceCountersData{Name: info.Name, Rows: counterRows(info.Stats, info.Name)}))
}

// counterRows is the counter list one interface reports. A rate row appears
// only while the sampler holds a rate for that interface.
func counterRows(stats *iface.InterfaceStats, name string) []detailKV {
	var rows []detailKV

	if stats != nil {
		rows = []detailKV{
			{Key: "RX Bytes", Value: strconv.Itoa(int(stats.RxBytes))},
			{Key: "RX Packets", Value: strconv.Itoa(int(stats.RxPackets))},
			{Key: "RX Errors", Value: strconv.Itoa(int(stats.RxErrors))},
			{Key: "RX Dropped", Value: strconv.Itoa(int(stats.RxDropped))},
			{Key: "TX Bytes", Value: strconv.Itoa(int(stats.TxBytes))},
			{Key: "TX Packets", Value: strconv.Itoa(int(stats.TxPackets))},
			{Key: "TX Errors", Value: strconv.Itoa(int(stats.TxErrors))},
			{Key: "TX Dropped", Value: strconv.Itoa(int(stats.TxDropped))},
		}
	} else {
		rows = []detailKV{{Key: "Counters", Value: "not available"}}
	}

	if r, ok := iface.GetRate(name); ok {
		rows = append(rows,
			detailKV{Key: "RX bps", Value: formatRate(r.RxBps)},
			detailKV{Key: "TX bps", Value: formatRate(r.TxBps)},
			detailKV{Key: "RX pps", Value: formatRate(r.RxPps)},
			detailKV{Key: "TX pps", Value: formatRate(r.TxPps)},
		)
	}

	return rows
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
		return handleInterfaceCountersContent(renderer, path[1])
	}

	// Traffic sub-path: /show/iface/traffic/
	if len(path) >= 1 && path[0] == "traffic" {
		return buildTrafficPageContent(renderer)
	}

	infos, err := iface.ListInterfaces()
	if err != nil {
		// Backend not loaded: still show configured entries from the editor tree.
		tableData := buildInterfaceTableDataForView(nil, viewTree, filterType)
		return renderer.renderComponent("workbench_table", workbenchTable(tableData))
	}

	tableData := buildInterfaceTableDataForView(infos, viewTree, filterType)
	return renderer.renderComponent("workbench_table", workbenchTable(tableData))
}

func handleInterfaceDetailContent(renderer *Renderer, name string, viewTree *config.Tree) template.HTML {
	info, err := iface.GetInterface(name)
	if err != nil || info == nil {
		info = configuredInterfaceByName(viewTree, name)
	}
	if info == nil {
		return renderer.renderComponent("iface_detail_missing", ifaceDetailMissing())
	}

	detailData := buildInterfaceDetailData(renderer, info)
	return renderer.renderComponent("workbench_detail", workbenchDetail(detailData))
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

func handleInterfaceCountersContent(renderer *Renderer, name string) template.HTML {
	stats, err := iface.GetStats(name)
	if err != nil {
		stats = nil
	}

	return renderer.renderComponent("iface_counters_table", detailKVTable(counterRows(stats, name)))
}
