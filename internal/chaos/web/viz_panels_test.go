package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestWritePanelGrid verifies writePanelGrid produces CSS Grid with 4 panel slots.
//
// VALIDATES: AC-1, AC-10 — panel grid has 4 slots with unique IDs.
// PREVENTS: Missing panel slots or duplicate IDs.
func TestWritePanelGrid(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	var buf strings.Builder
	writePanelGrid(&buf, d, defaultPanelSelections)
	html := buf.String()

	if !strings.Contains(html, `class="panel-grid"`) {
		t.Error("missing panel-grid container class")
	}
	for i := range maxPanels {
		id := "viz-panel-" + itoa(i)
		if !strings.Contains(html, id) {
			t.Errorf("missing panel slot %s", id)
		}
		contentID := "viz-panel-content-" + itoa(i)
		if !strings.Contains(html, contentID) {
			t.Errorf("missing panel content div %s", contentID)
		}
	}
}

// TestWritePanelGridDefaultSelections verifies default panels are Families, Convergence, Timeline, Events.
//
// VALIDATES: AC-8 — default panel mode load shows expected 4 panels.
// PREVENTS: Wrong default panel configuration.
func TestWritePanelGridDefaultSelections(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	var buf strings.Builder
	writePanelGrid(&buf, d, defaultPanelSelections)
	html := buf.String()

	// Each default viz should have its option selected in the dropdown.
	for _, name := range defaultPanelSelections {
		needle := `value="` + name + `" selected`
		if !strings.Contains(html, needle) {
			t.Errorf("default selection %q not marked as selected", name)
		}
	}
}

// TestWritePanelSlot verifies individual panel slot structure.
//
// VALIDATES: AC-2 — panel slot has dropdown and content area with HTMX polling.
// PREVENTS: Panel slot missing dropdown or content div.
func TestWritePanelSlot(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	var buf strings.Builder
	selections := defaultPanelSelections
	writePanelSlot(&buf, d, 2, selections)
	html := buf.String()

	checks := []struct {
		needle string
		label  string
	}{
		{`id="viz-panel-2"`, "panel slot ID"},
		{`id="viz-panel-content-2"`, "panel content ID"},
		{`<select name="viz"`, "dropdown select"},
		{`hx-get="/viz/panel-content?panel=2"`, "dropdown hx-get"},
		{`hx-target="#viz-panel-content-2"`, "dropdown hx-target"},
		{`hx-trigger="every`, "content polling"},
	}
	for _, c := range checks {
		if !strings.Contains(html, c.needle) {
			t.Errorf("missing %s: %q", c.label, c.needle)
		}
	}
}

// TestPanelDropdownOptions verifies dropdown lists all viz tab names.
//
// VALIDATES: AC-2 — panel dropdown contains all available visualizations.
// PREVENTS: Missing viz option in dropdown.
func TestPanelDropdownOptions(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	var buf strings.Builder
	writePanelSlot(&buf, d, 0, defaultPanelSelections)
	html := buf.String()

	for _, tab := range vizTabs {
		needle := `value="` + tab.Name + `"`
		if !strings.Contains(html, needle) {
			t.Errorf("dropdown missing option for %q", tab.Name)
		}
	}
}

// TestStripOuterVizAttrs verifies HTMX polling attributes are removed from viz content.
//
// VALIDATES: AC-3 — panel content doesn't conflict with panel slot polling.
// PREVENTS: Double polling when viz content is rendered inside a panel.
func TestStripOuterVizAttrs(t *testing.T) {
	t.Parallel()

	input := `<div class="viz-panel" hx-get="/viz/events" hx-trigger="every 500ms [!window._frozen]" hx-target="#viz-content" hx-swap="innerHTML">
<h3>Events</h3></div>`

	result := stripOuterVizAttrs(input)

	if !strings.Contains(result, `<div class="viz-panel">`) {
		t.Errorf("expected plain viz-panel div, got: %s", result[:min(80, len(result))])
	}
	if strings.Contains(result, `hx-get=`) {
		t.Error("hx-get not stripped from outer div")
	}
	if strings.Contains(result, `hx-trigger=`) {
		t.Error("hx-trigger not stripped from outer div")
	}
	if !strings.Contains(result, `<h3>Events</h3>`) {
		t.Error("inner content should be preserved")
	}
}

// TestStripOuterVizAttrsWithID verifies stripping works for divs with id attribute.
//
// VALIDATES: AC-3 — convergence panel (has id="viz-convergence") is properly stripped.
// PREVENTS: Panels with extra attributes failing to strip.
func TestStripOuterVizAttrsWithID(t *testing.T) {
	t.Parallel()

	input := `<div class="viz-panel" id="viz-convergence" sse-swap="convergence" hx-swap="outerHTML">
<h3>Convergence</h3></div>`

	result := stripOuterVizAttrs(input)

	if !strings.Contains(result, `<div class="viz-panel">`) {
		t.Errorf("expected plain viz-panel div, got: %s", result[:min(80, len(result))])
	}
	if strings.Contains(result, `sse-swap=`) {
		t.Error("sse-swap not stripped from outer div")
	}
}

// TestStripOuterVizAttrsNoMatch verifies passthrough when no viz-panel div found.
//
// VALIDATES: Defensive — non-viz HTML is returned unchanged.
// PREVENTS: Crash or corruption on unexpected input.
func TestStripOuterVizAttrsNoMatch(t *testing.T) {
	t.Parallel()

	input := `<div class="something-else"><h3>Title</h3></div>`
	result := stripOuterVizAttrs(input)
	if result != input {
		t.Error("non-viz HTML should pass through unchanged")
	}
}

// TestHandleVizPanels verifies the handler returns a panel grid.
//
// VALIDATES: AC-1, AC-10 — GET /viz/panels returns grid with 4 panel slots.
// PREVENTS: Handler not rendering panel grid.
func TestHandleVizPanels(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()
	d.state.StartTime = time.Now()

	req := httptest.NewRequest("GET", "/viz/panels", http.NoBody)
	rec := httptest.NewRecorder()
	d.handleVizPanels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `class="panel-grid"`) {
		t.Error("response missing panel-grid class")
	}
	for i := range maxPanels {
		if !strings.Contains(body, "viz-panel-"+itoa(i)) {
			t.Errorf("response missing panel slot %d", i)
		}
	}
}

// TestHandleVizPanelContent verifies individual panel content endpoint.
//
// VALIDATES: AC-2, AC-3 — GET /viz/panel-content returns viz without outer HTMX attrs.
// PREVENTS: Panel content with conflicting polling attributes.
func TestHandleVizPanelContent(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()
	d.state.StartTime = time.Now()

	req := httptest.NewRequest("GET", "/viz/panel-content?panel=1&viz=families", http.NoBody)
	rec := httptest.NewRecorder()
	d.handleVizPanelContent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `class="viz-panel"`) {
		t.Error("response missing viz-panel class")
	}
	// Outer HTMX attributes should be stripped.
	if strings.Contains(body, `hx-target="#viz-content"`) {
		t.Error("outer hx-target not stripped from panel content")
	}
}

// TestHandleVizPanelContentInvalidPanel verifies boundary: panel >= maxPanels is rejected.
//
// VALIDATES: Boundary — panel param must be 0-3.
// PREVENTS: Out-of-range panel index causing unexpected behavior.
func TestHandleVizPanelContentInvalidPanel(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	t.Cleanup(func() { d.broker.Close() })

	tests := []struct {
		name  string
		query string
	}{
		{"too high", "/viz/panel-content?panel=4&viz=families"},
		{"negative", "/viz/panel-content?panel=-1&viz=families"},
		{"not a number", "/viz/panel-content?panel=abc&viz=families"},
		{"missing", "/viz/panel-content?viz=families"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tt.query, http.NoBody)
			rec := httptest.NewRecorder()
			d.handleVizPanelContent(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tt.name, rec.Code)
			}
		})
	}
}

// TestHandleVizPanelContentInvalidViz verifies boundary: unknown viz name is rejected.
//
// VALIDATES: Boundary — viz param must be a known visualization name.
// PREVENTS: Invalid viz name causing empty content or panic.
func TestHandleVizPanelContentInvalidViz(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	req := httptest.NewRequest("GET", "/viz/panel-content?panel=0&viz=nonexistent", http.NoBody)
	rec := httptest.NewRecorder()
	d.handleVizPanelContent(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// TestIsValidVizName verifies all known viz names are accepted and unknown ones rejected.
//
// VALIDATES: Defensive — viz name validation.
// PREVENTS: Accepting arbitrary strings as viz names.
func TestIsValidVizName(t *testing.T) {
	t.Parallel()

	for _, tab := range vizTabs {
		if !isValidVizName(tab.Name) {
			t.Errorf("expected %q to be valid", tab.Name)
		}
	}
	if isValidVizName("nonexistent") {
		t.Error("expected 'nonexistent' to be invalid")
	}
	if isValidVizName("") {
		t.Error("expected empty string to be invalid")
	}
}

// TestPanelSelectionsFromRequest verifies query params override defaults.
//
// VALIDATES: AC-2 — panel selections can be changed via URL params.
// PREVENTS: Dropdown selections not being passed to panel grid.
func TestPanelSelectionsFromRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/viz/panels?p0=events&p2=route-matrix", http.NoBody)
	selections := panelSelectionsFromRequest(req)

	if selections[0] != "events" {
		t.Errorf("p0: expected events, got %s", selections[0])
	}
	if selections[1] != defaultPanelSelections[1] {
		t.Errorf("p1: expected default %s, got %s", defaultPanelSelections[1], selections[1])
	}
	if selections[2] != "route-matrix" {
		t.Errorf("p2: expected route-matrix, got %s", selections[2])
	}
	if selections[3] != defaultPanelSelections[3] {
		t.Errorf("p3: expected default %s, got %s", defaultPanelSelections[3], selections[3])
	}
}
