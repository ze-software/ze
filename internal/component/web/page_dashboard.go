// Design: docs/architecture/web-workbench-pages.md -- Dashboard sub-page handlers
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

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/health"
)

// dashboardHealthData is the template payload for the component health table.
type dashboardHealthData struct {
	Title        string
	Columns      []WorkbenchTableColumn
	Rows         []WorkbenchTableRow
	EmptyMessage string
}

// healthRowCells is the cells one health row renders, one per header column.
//
// The header and the body used to range two different slices. The two agreed
// only by the habit of every producer. A short row ended before the last th,
// and a zero-cell row drew an empty tr. A long row put a td under no header at
// all. The table an operator reads is the header's shape, so a short row is
// padded and a long one is cut here.
//
// Either resize is a producer defect rather than a display choice, so both are
// logged. A short row leaves a blank cell under a heading that names data. An
// operator reads that blank as "no value", never as a missing column, so the
// short row is as quiet a failure as the long one.
func healthRowCells(v dashboardHealthData, row WorkbenchTableRow) []string {
	if len(row.Cells) != len(v.Columns) {
		serverLogger.Warn("health row does not carry one cell per table column",
			"row", row.Key, "cells", len(row.Cells), "columns", len(v.Columns))
	}

	cells := make([]string, len(v.Columns))
	copy(cells, row.Cells)

	return cells
}

// dashboardEventsData is the template payload for the recent events table.
type dashboardEventsData struct {
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
	Name       string
	ConfigKey  string // top-level config key to check for "configured" fallback
	HealthName string // matching name in the health registry (empty = none)
	AlwaysUp   bool   // true for components proven running by serving this page
}

// knownComponents lists the components shown in the health table. HealthName
// links a row to a live probe in health.DefaultRegistry; AlwaysUp marks the web
// server, which is necessarily running to serve this page.
var knownComponents = []componentDef{
	{Name: "BGP", ConfigKey: "bgp", HealthName: "bgp"},
	{Name: "Interfaces", ConfigKey: "iface", HealthName: "iface"},
	{Name: "L2TP", ConfigKey: "l2tp", HealthName: "l2tp"},
	{Name: labelDNS, ConfigKey: "dns"},
	{Name: "SSH", ConfigKey: "environment/ssh"},
	{Name: "Web", ConfigKey: "environment/web", AlwaysUp: true},
	{Name: "Telemetry", ConfigKey: "telemetry"},
	{Name: "MCP", ConfigKey: "environment/mcp"},
	{Name: "Looking Glass", ConfigKey: "environment/looking-glass"},
}

// handleDashboardHealthPage returns the rendered HTML for the component health
// table. Rows backed by a live probe in health.DefaultRegistry show real
// operational state; the web server is always shown running (it is serving this
// page); the remaining rows fall back to config-tree presence (F10).
func handleDashboardHealthPage(renderer *Renderer, viewTree *config.Tree, _ *http.Request, _ CommandDispatcher) template.HTML {
	data := dashboardHealthData{
		Title: "Component Health",
		Columns: []WorkbenchTableColumn{
			{Key: "component", Label: labelComponent},
			{Key: "status", Label: "Status"},
			{Key: "summary", Label: "Summary"},
		},
		EmptyMessage: "No component information available.",
	}

	// Index live probes by name for O(1) lookup.
	probes := make(map[string]health.ComponentHealth)
	for _, c := range health.Check().Components {
		probes[c.Name] = c
	}

	for _, comp := range knownComponents {
		status, flagClass, summary := componentHealthRow(comp, viewTree, probes)
		data.Rows = append(data.Rows, WorkbenchTableRow{
			Key:       strings.ToLower(comp.Name),
			FlagClass: flagClass,
			Cells:     []string{comp.Name, status, summary},
		})
	}

	return renderer.renderComponent("dashboard_health", dashboardHealth(data))
}

// componentHealthRow resolves one component's status, flag color, and summary:
// a live probe wins, then AlwaysUp (web is serving), then config presence.
func componentHealthRow(comp componentDef, viewTree *config.Tree, probes map[string]health.ComponentHealth) (status, flagClass, summary string) {
	if comp.HealthName != "" {
		if probe, ok := probes[comp.HealthName]; ok {
			switch probe.Status {
			case health.StatusHealthy:
				return "Running", flagClassGreen, healthSummary(probe.Reason, "Operational")
			case health.StatusDegraded:
				return "Degraded", flagClassYellow, healthSummary(probe.Reason, "Degraded")
			case health.StatusDown:
				return "Down", flagClassRed, healthSummary(probe.Reason, "Not running")
			}
		}
	}
	if comp.AlwaysUp {
		return "Running", flagClassGreen, "Serving this page"
	}
	if isComponentConfigured(viewTree, comp.ConfigKey) {
		return "Configured", flagClassGreen, "-"
	}
	return "Not configured", flagClassGrey, "-"
}

// healthSummary returns the probe reason, or a fallback when the probe gave none.
func healthSummary(reason, fallback string) string {
	if reason != "" {
		return reason
	}
	return fallback
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

// handleDashboardEventsPage returns the rendered HTML for the recent events table.
// Dispatches "show event recent" with optional namespace filter.
func handleDashboardEventsPage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	selectedNS := r.URL.Query().Get("namespace")

	data := dashboardEventsData{
		Title: "Recent Events",
		Columns: []WorkbenchTableColumn{
			{Key: "time", Label: "Time"},
			{Key: "namespace", Label: "Namespace"},
			{Key: "message", Label: labelMessage},
		},
		SelectedNS:   selectedNS,
		EmptyMessage: "No recent events.",
		EmptyHint:    "Events will appear here as system activity occurs.",
	}

	if dispatch != nil {
		username := GetUsernameFromRequest(r)

		// Fetch namespaces for the filter dropdown.
		nsRendered, nsErr := dispatch.JSON(r.Context(), plugin.CallerIdentity{Username: username, RemoteAddr: r.RemoteAddr}, "show event namespaces")
		defer nsRendered.TransportComplete()
		if nsErr == nil && nsRendered.Output != "" {
			data.Namespaces = parseNamespaces(nsRendered.Output)
		}

		// Fetch recent events with optional namespace filter.
		cmd := "show event recent"
		if selectedNS != "" {
			cmd += " namespace " + selectedNS
		}
		rendered, err := dispatch.JSON(r.Context(), plugin.CallerIdentity{Username: username, RemoteAddr: r.RemoteAddr}, cmd)
		defer rendered.TransportComplete()
		if err == nil && rendered.Output != "" {
			data.Rows = parseEventOutput(rendered.Output)
		}
	}

	return renderer.renderComponent("dashboard_events", dashboardEvents(data))
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
				Cells: []string{ev.Timestamp, ev.Namespace, ev.EventType},
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
			Cells: []string{"-", "-", line},
		})
	}
	return rows
}
