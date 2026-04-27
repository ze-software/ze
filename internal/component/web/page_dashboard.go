// Design: plan/spec-web-8-tools-logs.md -- Dashboard sub-page handlers
// Related: workbench_dashboard.go -- BuildDashboardData for the overview panel
// Related: handler_admin.go -- CommandDispatcher type
// Related: workbench_pages.go -- Page dispatch (renderPageContent)
//
// Dashboard sub-pages extend the existing overview with dedicated Health and
// Events pages. Health shows per-component status indicators. Events shows
// recent events with namespace filtering. Both dispatch show commands through
// the standard CommandDispatcher.

package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// DashboardHealthData is the template payload for the component health table.
type DashboardHealthData struct {
	Title        string
	Columns      []WorkbenchTableColumn
	Rows         []WorkbenchTableRow
	EmptyMessage string
}

// DashboardEventsData is the template payload for the recent events table.
type DashboardEventsData struct {
	Title        string
	Columns      []WorkbenchTableColumn
	Rows         []WorkbenchTableRow
	Namespaces   []string
	SelectedNS   string
	EmptyMessage string
	EmptyHint    string
}

// --- Dashboard > Health ---

// componentDef describes one component for the health table.
type componentDef struct {
	Name      string
	ConfigKey string // top-level config key to check
}

// knownComponents lists the components shown in the health table.
var knownComponents = []componentDef{
	{Name: "BGP", ConfigKey: "bgp"},
	{Name: "Interfaces", ConfigKey: "iface"},
	{Name: "L2TP", ConfigKey: "l2tp"},
	{Name: "DNS", ConfigKey: "dns"},
	{Name: "SSH", ConfigKey: "environment/ssh"},
	{Name: "Web", ConfigKey: "environment/web"},
	{Name: "Telemetry", ConfigKey: "telemetry"},
	{Name: "MCP", ConfigKey: "environment/mcp"},
	{Name: "Looking Glass", ConfigKey: "environment/looking-glass"},
}

// HandleDashboardHealthPage returns the rendered HTML for the component health table.
// In v1, health is derived from config presence (configured = green, not configured = grey).
// Future versions dispatch "show health" for real operational state.
func HandleDashboardHealthPage(renderer *Renderer, viewTree *config.Tree, _ *http.Request, dispatch CommandDispatcher) template.HTML {
	data := DashboardHealthData{
		Title: "Component Health",
		Columns: []WorkbenchTableColumn{
			{Key: "component", Label: "Component"},
			{Key: "status", Label: "Status"},
			{Key: "summary", Label: "Summary"},
		},
		EmptyMessage: "No component information available.",
	}

	// Build rows from config tree presence. Future: dispatch "show health"
	// for real operational state when dispatch is available.
	for _, comp := range knownComponents {
		status := "Not configured"
		flagClass := flagClassGrey

		if isComponentConfigured(viewTree, comp.ConfigKey) {
			status = "Configured"
			flagClass = flagClassGreen
		}

		data.Rows = append(data.Rows, WorkbenchTableRow{
			Key:       strings.ToLower(comp.Name),
			FlagClass: flagClass,
			Cells:     []string{comp.Name, status, "-"},
		})
	}

	return renderer.RenderFragment("dashboard_health", data)
}

// isComponentConfigured checks if a component has config entries in the tree.
func isComponentConfigured(tree *config.Tree, configKey string) bool {
	if tree == nil {
		return false
	}

	parts := splitConfigPath(configKey)
	current := tree
	for _, part := range parts {
		child := current.GetContainer(part)
		if child == nil {
			return false
		}
		current = child
	}

	return true
}

// --- Dashboard > Events ---

// HandleDashboardEventsPage returns the rendered HTML for the recent events table.
// Dispatches "show event recent" with optional namespace filter.
func HandleDashboardEventsPage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	selectedNS := r.URL.Query().Get("namespace")

	data := DashboardEventsData{
		Title: "Recent Events",
		Columns: []WorkbenchTableColumn{
			{Key: "time", Label: "Time"},
			{Key: "namespace", Label: "Namespace"},
			{Key: "message", Label: "Message"},
		},
		SelectedNS:   selectedNS,
		EmptyMessage: "No recent events.",
		EmptyHint:    "Events will appear here as system activity occurs.",
	}

	if dispatch != nil {
		username := GetUsernameFromRequest(r)

		// Fetch namespaces for the filter dropdown.
		nsOutput, nsErr := dispatch("show event namespaces", username, r.RemoteAddr)
		if nsErr == nil && nsOutput != "" {
			data.Namespaces = parseNamespaces(nsOutput)
		}

		// Fetch recent events with optional namespace filter.
		cmd := "show event recent"
		if selectedNS != "" {
			cmd += " namespace " + selectedNS
		}
		output, err := dispatch(cmd, username, r.RemoteAddr)
		if err == nil && output != "" {
			data.Rows = parseEventOutput(output)
		}
	}

	return renderer.RenderFragment("dashboard_events", data)
}

// parseNamespaces parses show event namespaces JSON output into namespace names.
func parseNamespaces(output string) []string {
	var envelope struct {
		Namespaces []struct {
			Namespace string `json:"namespace"`
		} `json:"namespaces"`
	}
	if json.Unmarshal([]byte(output), &envelope) == nil && len(envelope.Namespaces) > 0 {
		ns := make([]string, 0, len(envelope.Namespaces))
		for _, entry := range envelope.Namespaces {
			if entry.Namespace != "" {
				ns = append(ns, entry.Namespace)
			}
		}
		return ns
	}

	// Fallback: line-per-namespace plain text.
	cleaned, _ := normalizeOutput(output)
	var namespaces []string
	for _, line := range splitLines(cleaned) {
		if s := strings.TrimSpace(line); s != "" {
			namespaces = append(namespaces, s)
		}
	}
	return namespaces
}

// parseEventOutput parses show event recent JSON output into table rows.
func parseEventOutput(output string) []WorkbenchTableRow {
	var envelope struct {
		Events []struct {
			Timestamp string `json:"timestamp"`
			Namespace string `json:"namespace"`
			EventType string `json:"event-type"`
		} `json:"events"`
	}
	if json.Unmarshal([]byte(output), &envelope) == nil && len(envelope.Events) > 0 {
		rows := make([]WorkbenchTableRow, 0, len(envelope.Events))
		for _, ev := range envelope.Events {
			rows = append(rows, WorkbenchTableRow{
				Cells: []string{
					template.HTMLEscapeString(ev.Timestamp),
					template.HTMLEscapeString(ev.Namespace),
					template.HTMLEscapeString(ev.EventType),
				},
			})
		}
		return rows
	}

	// Fallback: line-per-event plain text.
	cleaned, _ := normalizeOutput(output)
	if cleaned == "" {
		return nil
	}
	var rows []WorkbenchTableRow
	for _, line := range splitLines(cleaned) {
		if line == "" {
			continue
		}
		rows = append(rows, WorkbenchTableRow{
			Cells: []string{"-", "-", template.HTMLEscapeString(line)},
		})
	}
	return rows
}
