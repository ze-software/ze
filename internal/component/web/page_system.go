// Design: plan/learned/691-web-7-system-services.md -- System section pages
// Related: workbench_form.go -- Form component
// Related: workbench_table.go -- Table component
// Related: page_ip_dns.go -- DNS form page (pattern reference)
// Related: workbench_dashboard.go -- Dashboard system panel (pattern reference)

package web

import (
	"fmt"
	"html/template"
	"runtime"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/version"
)

// --- System > Identity ---

// BuildSystemIdentityFormData reads system identity fields from the config tree
// and returns a WorkbenchFormData for the identity form. The hostname comes from
// system/host and the router-id from bgp/router-id, matching the YANG schemas.
func BuildSystemIdentityFormData(tree *config.Tree) WorkbenchFormData {
	hostname := ""
	domain := ""
	routerID := ""

	if tree != nil {
		if sys := tree.GetContainer("system"); sys != nil {
			if h, ok := sys.Get("host"); ok {
				hostname = h
			}
			if d, ok := sys.Get("domain"); ok {
				domain = d
			}
		}
		if bgp := tree.GetContainer("bgp"); bgp != nil {
			if rid, ok := bgp.Get("router-id"); ok {
				routerID = rid
			}
		}
	}

	return WorkbenchFormData{
		Title: "System Identity",
		Fields: []WorkbenchFormField{
			{
				Name:        "hostname",
				Path:        "system/host",
				Label:       "Hostname",
				Type:        "text",
				Value:       hostname,
				Description: "System hostname (supports $ENV_VAR expansion)",
			},
			{
				Name:        "domain",
				Path:        "system/domain",
				Label:       "Domain",
				Type:        "text",
				Value:       domain,
				Description: "System domain name",
			},
			{
				Name:        "router-id",
				Path:        "bgp/router-id",
				Label:       "Router ID",
				Type:        "ip",
				Value:       routerID,
				Description: "BGP router identifier (from bgp/router-id)",
			},
		},
		SaveURL:    "/config/form/",
		DiscardURL: "/show/system/identity/",
	}
}

// HandleSystemIdentityPage renders the System Identity form.
func HandleSystemIdentityPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := BuildSystemIdentityFormData(viewTree)
	return renderer.RenderFragment("workbench_form", formData)
}

// --- Router Identity Resolution ---

// ResolveRouterIdentity returns a display name for this router instance.
// Priority: system/host (configured hostname), then bgp/router-id, then "ze".
func ResolveRouterIdentity(tree *config.Tree) string {
	if tree != nil {
		if sys := tree.GetContainer("system"); sys != nil {
			if h, ok := sys.Get("host"); ok && h != "" {
				return h
			}
		}
		if bgp := tree.GetContainer("bgp"); bgp != nil {
			if rid, ok := bgp.Get("router-id"); ok && rid != "" {
				return rid
			}
		}
	}
	return "ze"
}

// CollectFleetPeers reads system/fleet/peer[] from the config tree and
// returns a list of fleet peer entries for the topbar selector. The
// current instance is marked Active.
func CollectFleetPeers(tree *config.Tree, currentIdentity string) []FleetPeer {
	if tree == nil {
		return nil
	}
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}
	fleet := sys.GetContainer("fleet")
	if fleet == nil {
		return nil
	}
	var peers []FleetPeer
	for _, entry := range fleet.GetListOrdered("peer") {
		url := ""
		if entry.Value != nil {
			if u, ok := entry.Value.Get("url"); ok {
				url = u
			}
		}
		peers = append(peers, FleetPeer{
			Name:   entry.Key,
			URL:    url,
			Active: entry.Key == currentIdentity,
		})
	}
	return peers
}

// --- System > Users ---

// userEntry holds extracted fields for one local user from the config tree.
type userEntry struct {
	Name     string
	Profiles []string
	KeyCount int
	System   bool
}

// collectUsers walks the config tree and returns all local users from
// system/authentication/user[].
func collectUsers(tree *config.Tree) []userEntry {
	if tree == nil {
		return nil
	}
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}
	auth := sys.GetContainer("authentication")
	if auth == nil {
		return nil
	}

	var users []userEntry
	for _, entry := range auth.GetListOrdered("user") {
		ue := userEntry{Name: entry.Key}
		if entry.Value != nil {
			// Profiles from leaf-list
			if profiles := entry.Value.GetList("profile"); len(profiles) > 0 {
				for name := range profiles {
					ue.Profiles = append(ue.Profiles, name)
				}
			}
			// Also try the leaf-list approach used by the YANG: profile is a leaf-list
			// which may be stored as ordered list entries
			profileEntries := entry.Value.GetListOrdered("profile")
			if len(profileEntries) > 0 {
				ue.Profiles = nil
				for _, pe := range profileEntries {
					ue.Profiles = append(ue.Profiles, pe.Key)
				}
			}
			// Count SSH public keys
			keys := entry.Value.GetList("public-keys")
			ue.KeyCount = len(keys)
		}
		users = append(users, ue)
	}
	return users
}

// BuildUsersTableData constructs a WorkbenchTableData for the users page.
func BuildUsersTableData(users []userEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Username", Sortable: true},
		{Key: "profiles", Label: "Profiles"},
		{Key: "keys", Label: "SSH Keys"},
	}

	rows := make([]WorkbenchTableRow, 0, len(users))
	var tb textbuf.Buffer
	for _, u := range users {
		profileStr := textbuf.Join(u.Profiles, ", ")
		if profileStr == "" {
			profileStr = "-"
		}
		nameDisplay := u.Name
		if u.System {
			nameDisplay = tb.Reset().Str(u.Name).Str(" (system)").String()
		}
		row := WorkbenchTableRow{
			Key:   u.Name,
			Cells: []string{nameDisplay, profileStr, strconv.Itoa(u.KeyCount)},
		}
		if !u.System {
			userURL := tb.Reset().Str("/show/system/authentication/user/").Str(u.Name).Byte('/').String()
			row.URL = userURL
			row.Actions = []WorkbenchRowAction{
				{Label: "Edit", URL: userURL},
			}
		}
		rows = append(rows, row)
	}

	return WorkbenchTableData{
		Title:        "Users",
		AddURL:       "/show/system/authentication/user/",
		AddLabel:     "Add User",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No users configured.",
		EmptyHint:    "Add a user to enable SSH and web authentication.",
	}
}

// HandleUsersPage renders the System Users table. Power user names (from the
// zefs database, created by ze init) are prepended as system users so the
// page never claims "No users configured" when authentication is active.
func HandleUsersPage(renderer *Renderer, viewTree *config.Tree, powerUsers []string) template.HTML {
	configUsers := collectUsers(viewTree)
	allUsers := make([]userEntry, 0, len(powerUsers)+len(configUsers))
	for _, name := range powerUsers {
		allUsers = append(allUsers, userEntry{Name: name, System: true})
	}
	allUsers = append(allUsers, configUsers...)
	tableData := BuildUsersTableData(allUsers)
	return renderer.RenderFragment("workbench_table", tableData)
}

// --- System > Resources ---

// ResourcesData holds runtime resource information.
type ResourcesData struct {
	Version     string
	Uptime      string
	CPUCount    int
	GOMAXPROCS  int
	Goroutines  int
	MemAlloc    string
	MemSys      string
	GCRuns      uint32
	CurrentTime string
}

// BuildResourcesData gathers runtime system resources.
func BuildResourcesData() ResourcesData {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return ResourcesData{
		Version:     version.Short(),
		Uptime:      formatUptime(time.Since(processStart)),
		CPUCount:    runtime.NumCPU(),
		GOMAXPROCS:  runtime.GOMAXPROCS(0),
		Goroutines:  runtime.NumGoroutine(),
		MemAlloc:    formatBytes(mem.Alloc),
		MemSys:      formatBytes(mem.Sys),
		GCRuns:      mem.NumGC,
		CurrentTime: time.Now().Format("2006-01-02 15:04:05 MST"),
	}
}

// buildResourcesHTML renders the resources property list as HTML.
func buildResourcesHTML(data ResourcesData) template.HTML {
	var b textbuf.Buffer
	b.Str(`<div class="wb-resources" hx-get="/show/system/resources/" hx-trigger="every 5s" hx-swap="innerHTML">`)
	b.Str(`<h2 class="wb-form-title">System Resources</h2>`)
	b.Str(`<table class="wb-detail-kv">`)
	writeKV(&b, "Version", data.Version)
	writeKV(&b, "Uptime", data.Uptime)
	writeKV(&b, "CPU Cores", strconv.Itoa(data.CPUCount))
	writeKV(&b, "GOMAXPROCS", strconv.Itoa(data.GOMAXPROCS))
	writeKV(&b, "Goroutines", strconv.Itoa(data.Goroutines))
	writeKV(&b, "Memory Allocated", data.MemAlloc)
	writeKV(&b, "Memory System", data.MemSys)
	writeKV(&b, "GC Runs", strconv.Itoa(int(data.GCRuns)))
	writeKV(&b, "Current Time", data.CurrentTime)
	b.Str(`</table>`)
	b.Str(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

// HandleResourcesPage renders the System Resources property list.
func HandleResourcesPage() template.HTML {
	data := BuildResourcesData()
	return buildResourcesHTML(data)
}

// --- System > Host Hardware ---

// HardwareSection represents one subsection of the host hardware inventory.
type HardwareSection struct {
	Title string
	Items []HardwareItem
}

// HardwareItem is one key-value pair in a hardware section.
type HardwareItem struct {
	Key      string
	Value    string
	CSSClass string // optional CSS class for visual indicators (e.g., "up", "down", "alarm")
}

// BuildHostHardwareData detects the host inventory and returns hardware
// sections for display. Detection errors are non-fatal: partial data is
// shown with error items appended to the relevant section.
func BuildHostHardwareData() []HardwareSection {
	inv, err := host.Detect()
	if err != nil {
		return []HardwareSection{
			{Title: "Detection Error", Items: []HardwareItem{
				{Key: "Error", Value: err.Error()},
			}},
		}
	}

	var sections []HardwareSection

	if inv.CPU != nil {
		items := []HardwareItem{
			{Key: "Model", Value: inv.CPU.ModelName},
			{Key: "Vendor", Value: inv.CPU.Vendor.String()},
			{Key: "Logical CPUs", Value: strconv.Itoa(inv.CPU.LogicalCPUs)},
			{Key: "Physical Cores", Value: strconv.Itoa(inv.CPU.PhysicalCores)},
		}
		if inv.CPU.BaseFreqMHz > 0 {
			var bBase textbuf.Buffer
			items = append(items, HardwareItem{Key: "Base Frequency", Value: bBase.Reset().Int(int64(inv.CPU.BaseFreqMHz)).Str(" MHz").String()})
		}
		if inv.CPU.MaxFreqMHz > 0 {
			var bMax textbuf.Buffer
			items = append(items, HardwareItem{Key: "Max Frequency", Value: bMax.Reset().Int(int64(inv.CPU.MaxFreqMHz)).Str(" MHz").String()})
		}
		for i := range inv.CPU.Cores {
			c := &inv.CPU.Cores[i]
			freq := ""
			if c.CurrentFreqMHz > 0 {
				var bFreq textbuf.Buffer
				freq = bFreq.Reset().Str(", ").Int(int64(c.CurrentFreqMHz)).Str(" MHz").String()
			}
			role := ""
			if c.Role != host.CoreRoleUniform && c.Role != host.CoreRoleUnknown {
				var bRole textbuf.Buffer
				role = bRole.Str(", ").Str(c.Role.String()).String()
			}
			var bKey textbuf.Buffer
			var bVal textbuf.Buffer
			items = append(items, HardwareItem{
				Key:   bKey.Reset().Str("Core ").Int(int64(c.CoreID)).Str(" (pkg ").Int(int64(c.PhysicalPackage)).Byte(')').String(),
				Value: bVal.Reset().Str("cpu").Int(int64(c.CPU)).Str(freq).Str(role).String(),
			})
		}
		sections = append(sections, HardwareSection{Title: "CPU", Items: items})
	}

	if len(inv.NICs) > 0 {
		var items []HardwareItem
		for i := range inv.NICs {
			nic := &inv.NICs[i]
			speed := "-"
			if nic.LinkSpeedMbps > 0 {
				var bSpd textbuf.Buffer
				speed = bSpd.Reset().Int(int64(nic.LinkSpeedMbps)).Str(" Mbps").String()
			}
			carrier := "down"
			cssClass := "down"
			if nic.Carrier {
				carrier = "up"
				cssClass = "up"
			}
			var bVal textbuf.Buffer
			items = append(items, HardwareItem{
				Key:      nic.Name,
				Value:    bVal.Str(nic.Driver).Str(", ").Str(nic.MAC).Str(", ").Str(speed).Str(", ").Str(carrier).String(),
				CSSClass: cssClass,
			})
		}
		sections = append(sections, HardwareSection{Title: "NIC", Items: items})
	}

	if inv.Memory != nil {
		sections = append(sections, HardwareSection{Title: "Memory", Items: []HardwareItem{
			{Key: "Total", Value: formatBytes(inv.Memory.TotalBytes)},
			{Key: "Available", Value: formatBytes(inv.Memory.AvailableBytes)},
			{Key: "Free", Value: formatBytes(inv.Memory.FreeBytes)},
			{Key: "Swap Total", Value: formatBytes(inv.Memory.SwapTotalBytes)},
			{Key: "Swap Free", Value: formatBytes(inv.Memory.SwapFreeBytes)},
		}})
	}

	if inv.Storage != nil && len(inv.Storage.Devices) > 0 {
		var items []HardwareItem
		for _, dev := range inv.Storage.Devices {
			var bDet textbuf.Buffer
			bDet.Str(formatBytes(dev.SizeBytes)).Str(", ").Str(dev.Model)
			if dev.Smart != nil && !dev.Smart.Unavailable {
				bDet.Str(", SMART: ").Str(smartHealthLabel(dev.Smart.Healthy)).Byte(' ').Int(int64(dev.Smart.TempCelsius)).Str("°C ").Int(int64(dev.Smart.PowerOnHours)).Str("h")
				if dev.Smart.ErrorCount > 0 {
					bDet.Str(" (").Int(int64(dev.Smart.ErrorCount)).Str(" errors)")
				}
			}
			detail := bDet.String()
			items = append(items, HardwareItem{
				Key:   dev.Name,
				Value: detail,
			})
		}
		sections = append(sections, HardwareSection{Title: "Storage", Items: items})
	}

	if inv.Thermal != nil && len(inv.Thermal.Sensors) > 0 {
		var items []HardwareItem
		for i := range inv.Thermal.Sensors {
			s := &inv.Thermal.Sensors[i]
			tempC := float64(s.TempMC) / 1000.0
			alarm := ""
			cssClass := ""
			if s.Alarm {
				alarm = " [ALARM]"
				cssClass = "alarm"
			}
			items = append(items, HardwareItem{
				Key:      s.Name,
				Value:    fmt.Sprintf("%.1f°C%s", tempC, alarm),
				CSSClass: cssClass,
			})
		}
		sections = append(sections, HardwareSection{Title: "Thermal", Items: items})
	}

	if inv.DMI != nil {
		var items []HardwareItem
		addIfSet := func(k, v string) {
			if v != "" {
				items = append(items, HardwareItem{Key: k, Value: v})
			}
		}
		addIfSet("System Vendor", inv.DMI.SystemVendor)
		addIfSet("System Product", inv.DMI.SystemProduct)
		addIfSet("System Version", inv.DMI.SystemVersion)
		addIfSet("Board Vendor", inv.DMI.BoardVendor)
		addIfSet("Board Product", inv.DMI.BoardProduct)
		addIfSet("BIOS Vendor", inv.DMI.BIOSVendor)
		addIfSet("BIOS Version", inv.DMI.BIOSVersion)
		addIfSet("BIOS Date", inv.DMI.BIOSDate)
		addIfSet("Chassis Type", inv.DMI.ChassisType)
		if len(items) > 0 {
			sections = append(sections, HardwareSection{Title: "DMI", Items: items})
		}
	}

	if inv.Kernel != nil {
		items := []HardwareItem{
			{Key: "Release", Value: inv.Kernel.Release},
		}
		if inv.Kernel.Architecture != "" {
			items = append(items, HardwareItem{Key: "Architecture", Value: inv.Kernel.Architecture})
		}
		if inv.Kernel.BootTime != "" {
			items = append(items, HardwareItem{Key: "Boot Time", Value: inv.Kernel.BootTime})
		}
		sections = append(sections, HardwareSection{Title: "Kernel", Items: items})
	}

	if len(sections) == 0 {
		sections = append(sections, HardwareSection{
			Title: "Info",
			Items: []HardwareItem{{Key: "Status", Value: "No hardware information detected"}},
		})
	}

	return sections
}

// buildHostHardwareHTML renders the hardware inventory as HTML.
func buildHostHardwareHTML(sections []HardwareSection) template.HTML {
	var b textbuf.Buffer
	b.Str(`<div class="wb-hardware" hx-get="/show/system/hardware/" hx-trigger="every 10s" hx-swap="innerHTML">`)
	b.Str(`<h2 class="wb-form-title">Host Hardware</h2>`)

	for _, sec := range sections {
		b.Str(`<div class="wb-hardware-section">`)
		fmt.Fprintf(&b, `<h3>%s</h3>`, template.HTMLEscapeString(sec.Title)) //nolint:errcheck // report output
		b.Str(`<table class="wb-detail-kv">`)
		for _, item := range sec.Items {
			writeHardwareKV(&b, item)
		}
		b.Str(`</table>`)
		b.Str(`</div>`)
	}

	if len(sections) == 0 {
		b.Str(`<p>No hardware information available.</p>`)
	}

	b.Str(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
}

// writeHardwareKV writes a key-value table row with optional CSS class for
// visual indicators (NIC carrier status, thermal alarms).
func writeHardwareKV(b *textbuf.Buffer, item HardwareItem) {
	if item.CSSClass != "" {
		b.Str(`<tr class="wb-hardware-`).Str(template.HTMLEscapeString(item.CSSClass)).Str(`">`)
	} else {
		b.Str(`<tr>`)
	}
	b.Str(`<td class="wb-detail-kv-key">`).Str(template.HTMLEscapeString(item.Key)).Str(`</td>`)
	b.Str(`<td class="wb-detail-kv-val">`).Str(template.HTMLEscapeString(item.Value)).Str(`</td>`)
	b.Str(`</tr>`)
}

// HandleHostHardwarePage renders the Host Hardware inventory.
func HandleHostHardwarePage() template.HTML {
	sections := BuildHostHardwareData()
	return buildHostHardwareHTML(sections)
}

// --- System > Sysctl Profiles ---

// sysctlProfileEntry holds extracted fields for one sysctl profile.
type sysctlProfileEntry struct {
	Name         string
	SettingCount int
}

// collectSysctlProfiles walks the config tree for sysctl/profile[].
func collectSysctlProfiles(tree *config.Tree) []sysctlProfileEntry {
	if tree == nil {
		return nil
	}
	sysctlTree := tree.GetContainer("sysctl")
	if sysctlTree == nil {
		return nil
	}

	var profiles []sysctlProfileEntry
	for _, entry := range sysctlTree.GetListOrdered("profile") {
		pe := sysctlProfileEntry{Name: entry.Key}
		if entry.Value != nil {
			settings := entry.Value.GetList("setting")
			pe.SettingCount = len(settings)
		}
		profiles = append(profiles, pe)
	}
	return profiles
}

// BuildSysctlProfilesTableData constructs a WorkbenchTableData for the sysctl profiles page.
func BuildSysctlProfilesTableData(profiles []sysctlProfileEntry) WorkbenchTableData {
	columns := []WorkbenchTableColumn{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "settings", Label: "Settings", Sortable: true},
	}

	rows := make([]WorkbenchTableRow, 0, len(profiles))
	var tb textbuf.Buffer
	for _, p := range profiles {
		profileURL := tb.Reset().Str("/show/sysctl/profile/").Str(p.Name).Byte('/').String()
		rows = append(rows, WorkbenchTableRow{
			Key: p.Name,
			URL: profileURL,
			Cells: []string{
				p.Name,
				strconv.Itoa(p.SettingCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View", URL: profileURL},
				{Label: "Edit", URL: profileURL},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "Sysctl Profiles",
		AddURL:       "/show/sysctl/profile/",
		AddLabel:     "Add Profile",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No sysctl profiles configured.",
		EmptyHint:    "Create a profile to group kernel tunables for interface units.",
	}
}

func smartHealthLabel(healthy bool) string {
	if healthy {
		return "OK"
	}
	return "FAILING"
}

// HandleSysctlProfilesPage renders the Sysctl Profiles table.
func HandleSysctlProfilesPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	profiles := collectSysctlProfiles(viewTree)
	tableData := BuildSysctlProfilesTableData(profiles)
	return renderer.RenderFragment("workbench_table", tableData)
}
