// Design: plan/spec-web-7-system-services.md -- System section pages
// Related: workbench_form.go -- Form component
// Related: workbench_table.go -- Table component
// Related: page_ip_dns.go -- DNS form page (pattern reference)
// Related: workbench_dashboard.go -- Dashboard system panel (pattern reference)

package web

import (
	"fmt"
	"html/template"
	"runtime"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/version"
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
				Label:       "Hostname",
				Type:        "text",
				Value:       hostname,
				Description: "System hostname (supports $ENV_VAR expansion)",
			},
			{
				Name:        "domain",
				Label:       "Domain",
				Type:        "text",
				Value:       domain,
				Description: "System domain name",
			},
			{
				Name:        "router-id",
				Label:       "Router ID",
				Type:        "ip",
				Value:       routerID,
				Description: "BGP router identifier (from bgp/router-id)",
			},
		},
		SaveURL:    "/admin/system/identity/save",
		DiscardURL: "/show/system/identity/",
	}
}

// HandleSystemIdentityPage renders the System Identity form.
func HandleSystemIdentityPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	formData := BuildSystemIdentityFormData(viewTree)
	return renderer.RenderFragment("workbench_form", formData)
}

// --- System > Users ---

// userEntry holds extracted fields for one local user from the config tree.
type userEntry struct {
	Name     string
	Profiles []string
	KeyCount int
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
	for _, u := range users {
		profileStr := strings.Join(u.Profiles, ", ")
		if profileStr == "" {
			profileStr = "-"
		}
		rows = append(rows, WorkbenchTableRow{
			Key:   u.Name,
			URL:   fmt.Sprintf("/show/system/authentication/user/%s/", u.Name),
			Cells: []string{u.Name, profileStr, fmt.Sprintf("%d", u.KeyCount)},
			Actions: []WorkbenchRowAction{
				{Label: "Edit", URL: fmt.Sprintf("/show/system/authentication/user/%s/", u.Name)},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "Users",
		AddURL:       "/show/system/authentication/user/add",
		AddLabel:     "Add User",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No users configured.",
		EmptyHint:    "Add a user to enable SSH and web authentication.",
	}
}

// HandleUsersPage renders the System Users table.
func HandleUsersPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	users := collectUsers(viewTree)
	tableData := BuildUsersTableData(users)
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
		Uptime:      "-", // Requires operational data from show RPC (future)
		CPUCount:    runtime.NumCPU(),
		GOMAXPROCS:  runtime.GOMAXPROCS(0),
		Goroutines:  runtime.NumGoroutine(),
		MemAlloc:    formatBytes(mem.Alloc),
		MemSys:      formatBytes(mem.Sys),
		GCRuns:      mem.NumGC,
		CurrentTime: "-", // Requires operational data from show RPC (future)
	}
}

// buildResourcesHTML renders the resources property list as HTML.
func buildResourcesHTML(data ResourcesData) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="wb-resources" hx-get="/show/system/resources/" hx-trigger="every 5s" hx-swap="innerHTML">`)
	b.WriteString(`<h2 class="wb-form-title">System Resources</h2>`)
	b.WriteString(`<table class="wb-detail-kv">`)
	writeKV(&b, "Version", data.Version)
	writeKV(&b, "Uptime", data.Uptime)
	writeKV(&b, "CPU Cores", fmt.Sprintf("%d", data.CPUCount))
	writeKV(&b, "GOMAXPROCS", fmt.Sprintf("%d", data.GOMAXPROCS))
	writeKV(&b, "Goroutines", fmt.Sprintf("%d", data.Goroutines))
	writeKV(&b, "Memory Allocated", data.MemAlloc)
	writeKV(&b, "Memory System", data.MemSys)
	writeKV(&b, "GC Runs", fmt.Sprintf("%d", data.GCRuns))
	writeKV(&b, "Current Time", data.CurrentTime)
	b.WriteString(`</table>`)
	b.WriteString(`</div>`)
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
	Key   string
	Value string
}

// BuildHostHardwareData returns placeholder hardware sections. In v1, real
// data requires dispatching show host/* RPCs. The sections mirror the
// host.Inventory struct: CPU, NIC, Storage, Memory, Thermal, DMI.
func BuildHostHardwareData() []HardwareSection {
	return []HardwareSection{
		{Title: "CPU", Items: []HardwareItem{
			{Key: "Status", Value: "Requires show host/cpu command dispatch"},
		}},
		{Title: "NIC", Items: []HardwareItem{
			{Key: "Status", Value: "Requires show host/nic command dispatch"},
		}},
		{Title: "Storage", Items: []HardwareItem{
			{Key: "Status", Value: "Requires show host/storage command dispatch"},
		}},
		{Title: "Memory", Items: []HardwareItem{
			{Key: "Status", Value: "Requires show host/memory command dispatch"},
		}},
		{Title: "Thermal", Items: []HardwareItem{
			{Key: "Status", Value: "Requires show host/thermal command dispatch"},
		}},
		{Title: "DMI", Items: []HardwareItem{
			{Key: "Status", Value: "Requires show host/dmi command dispatch"},
		}},
	}
}

// buildHostHardwareHTML renders the hardware inventory as HTML.
func buildHostHardwareHTML(sections []HardwareSection) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="wb-hardware">`)
	b.WriteString(`<h2 class="wb-form-title">Host Hardware</h2>`)

	for _, sec := range sections {
		b.WriteString(`<div class="wb-hardware-section">`)
		fmt.Fprintf(&b, `<h3>%s</h3>`, template.HTMLEscapeString(sec.Title))
		b.WriteString(`<table class="wb-detail-kv">`)
		for _, item := range sec.Items {
			writeKV(&b, item.Key, item.Value)
		}
		b.WriteString(`</table>`)
		b.WriteString(`</div>`)
	}

	if len(sections) == 0 {
		b.WriteString(`<p>No hardware information available.</p>`)
	}

	b.WriteString(`</div>`)
	return template.HTML(b.String()) //nolint:gosec // trusted builder output
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
	for _, p := range profiles {
		rows = append(rows, WorkbenchTableRow{
			Key: p.Name,
			URL: fmt.Sprintf("/show/sysctl/profile/%s/", p.Name),
			Cells: []string{
				p.Name,
				fmt.Sprintf("%d", p.SettingCount),
			},
			Actions: []WorkbenchRowAction{
				{Label: "View", URL: fmt.Sprintf("/show/sysctl/profile/%s/", p.Name)},
				{Label: "Edit", URL: fmt.Sprintf("/show/sysctl/profile/%s/", p.Name)},
			},
		})
	}

	return WorkbenchTableData{
		Title:        "Sysctl Profiles",
		AddURL:       "/show/sysctl/profile/add",
		AddLabel:     "Add Profile",
		Columns:      columns,
		Rows:         rows,
		EmptyMessage: "No sysctl profiles configured.",
		EmptyHint:    "Create a profile to group kernel tunables for interface units.",
	}
}

// HandleSysctlProfilesPage renders the Sysctl Profiles table.
func HandleSysctlProfilesPage(renderer *Renderer, viewTree *config.Tree) template.HTML {
	profiles := collectSysctlProfiles(viewTree)
	tableData := BuildSysctlProfilesTableData(profiles)
	return renderer.RenderFragment("workbench_table", tableData)
}
