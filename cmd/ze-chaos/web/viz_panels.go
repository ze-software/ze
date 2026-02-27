// Design: docs/architecture/chaos-web-dashboard.md — multi-panel viz layout
// Related: viz.go — viz write functions reused for panel content
// Related: render.go — writeLayout tab bar includes panel mode toggle

package web

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// vizTab describes a visualization that can appear in a panel slot.
type vizTab struct {
	Name     string // URL path segment (e.g. "families")
	Label    string // Display label (e.g. "Families")
	Interval string // HTMX polling interval (e.g. "2s")
}

// vizTabs lists all available visualizations for panel selection.
var vizTabs = []vizTab{
	{Name: "families", Label: "Families", Interval: "2s"},
	{Name: "convergence", Label: "Convergence", Interval: "2s"},
	{Name: "peer-timeline", Label: "Timeline", Interval: "2s"},
	{Name: "events", Label: "Events", Interval: "1s"},
	{Name: "all-peers", Label: "All Peers", Interval: "2s"},
	{Name: "route-matrix", Label: "Route Matrix", Interval: "2s"},
	{Name: "chaos-timeline", Label: "Chaos Timeline", Interval: "2s"},
	{Name: "chaos-events", Label: "Chaos Events", Interval: "2s"},
}

// defaultPanelSelections holds the initial viz for each of the 4 panel slots.
var defaultPanelSelections = [maxPanels]string{"families", "convergence", "peer-timeline", "events"}

const maxPanels = 4

// handleVizPanels serves the multi-panel grid layout.
func (d *Dashboard) handleVizPanels(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	selections := panelSelectionsFromRequest(r)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writePanelGrid(w, d, selections)
}

// handleVizPanelContent serves individual panel content for HTMX polling.
// Query params: panel (0-3), viz (visualization name).
func (d *Dashboard) handleVizPanelContent(w http.ResponseWriter, r *http.Request) {
	panelStr := r.URL.Query().Get("panel")
	panel, panelErr := strconv.Atoi(panelStr)
	if panelErr != nil || panel < 0 || panel >= maxPanels {
		http.Error(w, "invalid panel", http.StatusBadRequest)
		return
	}

	vizName := r.URL.Query().Get("viz")
	if !isValidVizName(vizName) {
		http.Error(w, "invalid viz", http.StatusBadRequest)
		return
	}

	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf bytes.Buffer
	renderVizToBuffer(&buf, d, vizName)
	content := stripOuterVizAttrs(buf.String())
	h := &htmlWriter{w: w}
	h.write(content)
}

// panelSelectionsFromRequest reads panel selections from query params,
// falling back to defaults for missing values.
func panelSelectionsFromRequest(r *http.Request) [maxPanels]string {
	selections := defaultPanelSelections
	for i := range maxPanels {
		if v := r.URL.Query().Get("p" + strconv.Itoa(i)); isValidVizName(v) {
			selections[i] = v
		}
	}
	return selections
}

// isValidVizName returns true if the name matches a known viz tab.
func isValidVizName(name string) bool {
	for _, tab := range vizTabs {
		if tab.Name == name {
			return true
		}
	}
	return false
}

// vizInterval returns the polling interval for a named viz.
func vizInterval(name string) string {
	for _, tab := range vizTabs {
		if tab.Name == name {
			return tab.Interval
		}
	}
	return "2s"
}

// writePanelGrid renders the CSS Grid container holding all panel slots.
func writePanelGrid(w io.Writer, d *Dashboard, selections [maxPanels]string) {
	h := &htmlWriter{w: w}
	h.write(`<div class="panel-grid">`)
	for i := range maxPanels {
		writePanelSlot(w, d, i, selections)
	}
	h.write(`</div>`)
}

// writePanelSlot renders one panel: dropdown selector + viz content area with independent polling.
func writePanelSlot(w io.Writer, d *Dashboard, slot int, selections [maxPanels]string) {
	h := &htmlWriter{w: w}
	selected := selections[slot]
	interval := vizInterval(selected)
	slotStr := strconv.Itoa(slot)

	h.writef(`<div class="panel-slot" id="viz-panel-%s">`, slotStr)

	// Panel header with viz selector dropdown.
	h.writef(`<div class="panel-header"><select name="viz" hx-get="/viz/panel-content?panel=%s" hx-target="#viz-panel-content-%s" hx-swap="innerHTML" hx-include="this">`,
		slotStr, slotStr)

	for _, tab := range vizTabs {
		sel := ""
		if tab.Name == selected {
			sel = ` selected`
		}
		h.writef(`<option value="%s"%s>%s</option>`, tab.Name, sel, tab.Label)
	}
	h.write(`</select></div>`)

	// Content area with independent HTMX polling.
	h.writef(`<div class="panel-content" id="viz-panel-content-%s" hx-get="/viz/panel-content?panel=%s&viz=%s" hx-trigger="every %s [!window._frozen]" hx-swap="innerHTML">`,
		slotStr, slotStr, selected, interval)

	// Render initial content inline.
	var buf bytes.Buffer
	renderVizToBuffer(&buf, d, selected)
	h.write(stripOuterVizAttrs(buf.String()))
	h.write(`</div></div>`)
}

// renderVizToBuffer dispatches to the appropriate viz write function,
// capturing the output in the provided writer.
func renderVizToBuffer(w io.Writer, d *Dashboard, vizName string) {
	switch vizName {
	case "families":
		writeFamilyMatrix(w, d.state)
	case "convergence":
		writeConvergenceHistogram(w, d.state.Convergence, d.state.ConvergenceDeadline)
	case "peer-timeline":
		writePeerTimeline(w, d.state, 1, "all")
	case "events":
		writeEventStream(w, d.state, "", "")
	case "all-peers":
		writeAllPeers(w, d.state, "id", "asc")
	case "route-matrix":
		writeRouteMatrix(w, d.state.RouteMatrix, routeMatrixOpts{})
	case "chaos-timeline":
		writeChaosTimeline(w, d.state, d.state.WarmupDuration)
	case "chaos-events":
		writeChaosEvents(w, d.state)
	}
}

// stripOuterVizAttrs removes the HTMX polling/swap attributes from the outermost
// viz-panel div so that panel slot containers manage polling independently.
// The viz-panel class is preserved for styling.
func stripOuterVizAttrs(html string) string {
	// Find the first <div that has class="viz-panel".
	idx := strings.Index(html, `<div class="viz-panel"`)
	if idx < 0 {
		// Some panels use id= before class=; try alternate.
		idx = strings.Index(html, `<div class="viz-panel" `)
		if idx < 0 {
			return html
		}
	}

	// Find the closing > of this opening tag.
	end := strings.Index(html[idx:], ">")
	if end < 0 {
		return html
	}

	// Replace the opening tag with a plain div.viz-panel.
	return html[:idx] + `<div class="viz-panel">` + html[idx+end+1:]
}
