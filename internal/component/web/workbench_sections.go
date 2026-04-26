// Design: docs/architecture/web-components.md -- Workbench left navigation
// Related: ui_mode.go -- Workbench/Finder selector
//
// Spec: plan/spec-web-3-foundation.md (Two-level navigation, Phases 1-3).
//
// Per the spec, v1 keeps the section taxonomy in one Go file. The schema-driven
// `ze:nav-section` extension is deferred to a follow-up; if entries spread to
// more than one Go file, the boundary calls for switching to metadata first.

package web

import "strings"

// Section-key and path-segment constants used in selection logic.
const (
	segBGP       = "bgp"
	segFirewall  = "firewall"
	segPolicy    = "policy"
	segRouting   = "routing"
	segSystem    = "system"
	segL2TP      = "l2tp"
	segSSH       = "ssh"
	segWeb       = "web"
	segTelemetry = "telemetry"
	segTACACS    = "tacacs"
	segMCP       = "mcp"
	segLG        = "lg"
	segAPI       = "api"
	segTools     = "tools"
	segLogs      = "logs"
	segHealth    = "health"
	segEvents    = "events"
)

// WorkbenchSubPage is a child entry within a workbench navigation section.
type WorkbenchSubPage struct {
	// Key is a stable identifier used for selection highlighting and tests.
	Key string
	// Label is the user-facing sub-page name.
	Label string
	// URL is the destination path within the web UI.
	URL string
	// Selected is true when the current view matches this sub-page.
	Selected bool
}

// WorkbenchSection is one entry in the workbench left navigation.
type WorkbenchSection struct {
	// Key is a stable identifier used for selection highlighting and tests.
	Key string
	// Label is the user-facing section name.
	Label string
	// URL is the destination path within the web UI (matches the first child).
	URL string
	// Selected is true when the current view falls under this section.
	Selected bool
	// Expanded is true when this section's children should be visible.
	Expanded bool
	// Children are the sub-pages within this section.
	Children []WorkbenchSubPage
}

// sectionDef is a build-time definition for one nav section with its children.
type sectionDef struct {
	key      string
	label    string
	children []WorkbenchSubPage
}

// sections returns the canonical two-level navigation structure. Each section's
// URL is set to the first child's URL so clicking the section header navigates
// to the default sub-page.
func sections() []sectionDef {
	return []sectionDef{
		{key: "dashboard", label: "Dashboard", children: []WorkbenchSubPage{
			{Key: "overview", Label: "Overview", URL: "/show/"},
			{Key: "health", Label: "Health", URL: "/show/health/"},
			{Key: "events", Label: "Recent Events", URL: "/show/events/"},
		}},
		{key: "interfaces", label: "Interfaces", children: []WorkbenchSubPage{
			{Key: "all", Label: "All Interfaces", URL: "/show/iface/"},
			{Key: "ethernet", Label: "Ethernet", URL: "/show/iface/?type=ethernet"},
			{Key: "bridge", Label: "Bridge", URL: "/show/iface/?type=bridge"},
			{Key: "vlan", Label: "VLAN", URL: "/show/iface/?type=vlan"},
			{Key: "tunnel", Label: "Tunnel", URL: "/show/iface/?type=tunnel"},
			{Key: "traffic", Label: "Traffic", URL: "/show/iface/traffic/"},
		}},
		{key: "ip", label: "IP", children: []WorkbenchSubPage{
			{Key: "addresses", Label: "Addresses", URL: "/show/ip/addresses/"},
			{Key: "routes", Label: "Routes", URL: "/show/ip/routes/"},
			{Key: "dns", Label: "DNS", URL: "/show/ip/dns/"},
		}},
		{key: segRouting, label: "Routing", children: []WorkbenchSubPage{
			{Key: "peers", Label: "Peers", URL: "/show/bgp/peer/"},
			{Key: "groups", Label: "Groups", URL: "/show/bgp/group/"},
			{Key: "families", Label: "Families", URL: "/show/bgp/family/"},
			{Key: "summary", Label: "Summary", URL: "/show/bgp/summary/"},
			{Key: "redistribute", Label: "Redistribute", URL: "/show/redistribute/"},
		}},
		{key: segPolicy, label: "Policy", children: []WorkbenchSubPage{
			{Key: "filters", Label: "Filters", URL: "/show/bgp/policy/"},
			{Key: "communities", Label: "Communities", URL: "/show/bgp/community/"},
			{Key: "prefix-lists", Label: "Prefix Lists", URL: "/show/bgp/prefix-list/"},
		}},
		{key: segFirewall, label: "Firewall", children: []WorkbenchSubPage{
			{Key: "tables", Label: "Tables", URL: "/show/firewall/"},
			{Key: "chains", Label: "Chains", URL: "/show/firewall/chain/"},
			{Key: "rules", Label: "Rules", URL: "/show/firewall/rule/"},
			{Key: "sets", Label: "Sets", URL: "/show/firewall/set/"},
			{Key: "connections", Label: "Connections", URL: "/show/firewall/connections/"},
		}},
		{key: "l2tp", label: "L2TP", children: []WorkbenchSubPage{
			{Key: "sessions", Label: "Sessions", URL: "/show/l2tp/sessions/"},
			{Key: "configuration", Label: "Configuration", URL: "/show/l2tp/"},
			{Key: "health", Label: "Health", URL: "/show/l2tp/health/"},
		}},
		{key: "services", label: "Services", children: []WorkbenchSubPage{
			{Key: "ssh", Label: "SSH", URL: "/show/ssh/"},
			{Key: "web", Label: "Web", URL: "/show/web/"},
			{Key: "telemetry", Label: "Telemetry", URL: "/show/telemetry/"},
			{Key: "tacacs", Label: "TACACS", URL: "/show/tacacs/"},
			{Key: "mcp", Label: "MCP", URL: "/show/mcp/"},
			{Key: "lg", Label: "Looking Glass", URL: "/show/lg/"},
			{Key: "api", Label: "API", URL: "/show/api/"},
		}},
		{key: "system", label: "System", children: []WorkbenchSubPage{
			{Key: "identity", Label: "Identity", URL: "/show/system/identity/"},
			{Key: "users", Label: "Users", URL: "/show/users/"},
			{Key: "resources", Label: "Resources", URL: "/show/system/resources/"},
			{Key: "hardware", Label: "Host Hardware", URL: "/show/system/hardware/"},
			{Key: "sysctl", Label: "Sysctl Profiles", URL: "/show/system/sysctl/"},
		}},
		{key: segTools, label: "Tools", children: []WorkbenchSubPage{
			{Key: "ping", Label: "Ping", URL: "/show/tools/ping/"},
			{Key: "decode", Label: "BGP Decode", URL: "/show/tools/bgp-decode/"},
			{Key: "metrics", Label: "Metrics Query", URL: "/show/tools/metrics/"},
			{Key: "capture", Label: "Capture", URL: "/show/tools/capture/"},
		}},
		{key: segLogs, label: "Logs", children: []WorkbenchSubPage{
			{Key: "live", Label: "Live Log", URL: "/show/logs/live/"},
			{Key: "warnings", Label: "Warnings", URL: "/show/logs/warnings/"},
			{Key: "errors", Label: "Errors", URL: "/show/logs/errors/"},
		}},
	}
}

// WorkbenchSections returns the ordered list of left-nav sections that the V2
// shell renders. Selection is driven by URL path matching so the active section
// follows /show navigation without per-page configuration.
//
// The order is intentional: Dashboard first, then operator-facing object
// classes, then service control, then diagnostic tools.
func WorkbenchSections(currentPath []string) []WorkbenchSection {
	defs := sections()
	result := make([]WorkbenchSection, len(defs))

	for i, def := range defs {
		children := make([]WorkbenchSubPage, len(def.children))
		copy(children, def.children)

		sectionURL := ""
		if len(children) > 0 {
			sectionURL = children[0].URL
		}

		section := WorkbenchSection{
			Key:      def.key,
			Label:    def.label,
			URL:      sectionURL,
			Children: children,
		}

		// Try to select a child by matching the current path.
		selectChild(&section, currentPath)

		result[i] = section
	}

	// If no section was selected, default to Dashboard + its first child.
	anySelected := false
	for _, s := range result {
		if s.Selected {
			anySelected = true
			break
		}
	}

	if !anySelected && len(result) > 0 {
		result[0].Selected = true
		result[0].Expanded = true
		if len(result[0].Children) > 0 {
			result[0].Children[0].Selected = true
		}
	}

	return result
}

// selectChild determines which child (if any) matches the current path and
// marks the section and child as selected/expanded.
func selectChild(section *WorkbenchSection, currentPath []string) {
	if len(currentPath) == 0 {
		return
	}

	first := currentPath[0]

	// Map leading path segments to section keys. Multiple leading segments can
	// map to the same section (e.g., segBGP maps to routing, policy, or both
	// depending on the second segment).
	switch section.Key {
	case "dashboard":
		// Dashboard matches the empty path (handled by the fallback default).
		return
	case "interfaces":
		if first != "iface" {
			return
		}
	case "ip":
		if first != "ip" {
			return
		}
	case segRouting:
		if first != segBGP && first != "redistribute" {
			return
		}
		// bgp/policy and bgp/community and bgp/prefix-list belong to Policy.
		if first == segBGP && len(currentPath) > 1 {
			switch currentPath[1] {
			case segPolicy, "community", "prefix-list":
				return
			}
		}
	case segPolicy:
		if first != segBGP {
			return
		}
		if len(currentPath) < 2 {
			return
		}
		switch currentPath[1] {
		case segPolicy, "community", "prefix-list":
			// matches
		default:
			return
		}
	case segFirewall:
		if first != segFirewall {
			return
		}
	case "l2tp":
		if first != "l2tp" {
			return
		}
	case "services":
		switch first {
		case "ssh", "web", "telemetry", "tacacs", "mcp", "lg", "api":
			// matches
		case "environment":
			// legacy compat
		default:
			return
		}
	case "system":
		switch first {
		case "system", "users":
			// matches
		default:
			return
		}
	case segTools:
		if first != segTools {
			return
		}
	case segLogs:
		if first != segLogs {
			return
		}
	default:
		return
	}

	section.Selected = true
	section.Expanded = true

	// Try to find the best matching child.
	selectBestChild(section, currentPath)
}

// selectBestChild marks the best-matching child as selected.
func selectBestChild(section *WorkbenchSection, currentPath []string) {
	// Build a path for matching: /segment1/segment2/...
	showPath := "/show/" + strings.Join(currentPath, "/")
	adminPath := "/admin/" + strings.Join(currentPath, "/")

	bestIdx := -1
	bestLen := 0

	for i, child := range section.Children {
		childPath := child.URL
		if idx := strings.IndexByte(childPath, '?'); idx >= 0 {
			childPath = childPath[:idx]
		}

		// Normalize: ensure trailing slash for prefix matching.
		if !strings.HasSuffix(childPath, "/") {
			childPath += "/"
		}

		match := false
		if strings.HasPrefix(showPath+"/", childPath) || strings.HasPrefix(showPath, childPath) {
			match = true
		}
		if strings.HasPrefix(adminPath+"/", childPath) || strings.HasPrefix(adminPath, childPath) {
			match = true
		}

		if match && len(childPath) > bestLen {
			bestLen = len(childPath)
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		section.Children[bestIdx].Selected = true
	} else if len(section.Children) > 0 {
		// Default to first child when section matches but no specific child does.
		section.Children[0].Selected = true
	}
}
