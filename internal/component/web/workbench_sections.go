// Design: docs/architecture/web-components.md -- Workbench left navigation
// Related: ui_mode.go -- Workbench/Finder selector
//
// Spec: plan/spec-web-2-operator-workbench.md (Left Navigation Sections, D9).
//
// Per the spec, v1 keeps the section taxonomy in one Go file. The schema-driven
// `ze:nav-section` extension is deferred to a follow-up; if entries spread to
// more than one Go file, the boundary calls for switching to metadata first.

package web

// WorkbenchSection is one entry in the workbench left navigation.
type WorkbenchSection struct {
	// Key is a stable identifier used for selection highlighting and tests.
	Key string
	// Label is the user-facing section name.
	Label string
	// URL is the destination path within the web UI.
	URL string
	// Selected is true when the current view falls under this section.
	Selected bool
}

// WorkbenchSections returns the ordered list of left-nav sections that the V2
// shell renders. Selection is driven by the leading YANG path segment so the
// active section follows /show navigation without per-page configuration.
//
// The order is intentional: Dashboard first, then operator-facing object
// classes, then service control, then diagnostic tools.
func WorkbenchSections(currentPath []string) []WorkbenchSection {
	first := ""
	if len(currentPath) > 0 {
		first = currentPath[0]
	}

	sections := []WorkbenchSection{
		{Key: "dashboard", Label: "Dashboard", URL: "/show/"},
		{Key: "interfaces", Label: "Interfaces", URL: "/show/iface/"},
		{Key: "routing", Label: "Routing", URL: "/show/bgp/"},
		{Key: "policy", Label: "Policy", URL: "/show/bgp/policy/"},
		{Key: "firewall", Label: "Firewall", URL: "/show/firewall/"},
		{Key: "services", Label: "Services", URL: "/show/environment/"},
		{Key: "system", Label: "System", URL: "/show/users/"},
		{Key: "tools", Label: "Tools", URL: "/admin/"},
		{Key: "logs", Label: "Logs", URL: "/admin/show/log/"},
	}

	// Selection map: which leading YANG segment lights up which section.
	switch first {
	case "":
		sections[0].Selected = true
	case "iface":
		sections[1].Selected = true
	case "bgp":
		// Policy takes precedence inside bgp/policy.
		if len(currentPath) > 1 && currentPath[1] == "policy" {
			sections[3].Selected = true
		} else {
			sections[2].Selected = true
		}
	case "firewall":
		sections[4].Selected = true
	case "environment":
		sections[5].Selected = true
	case "users":
		sections[6].Selected = true
	}

	return sections
}
