package web

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRenderWorkbench verifies that the workbench page template renders the
// shell regions defined by the spec: top bar, left navigation, workspace area,
// commit bar, and CLI bar. Each region must carry a stable id so the .wb
// runner and DOM-level tests can pin selection across HTMX swaps.
//
// VALIDATES: AC-1 (workbench shell renders all regions).
// PREVENTS: Removing or renaming a region without updating the contract that
// downstream tests rely on.
func TestRenderWorkbench(t *testing.T) {
	r, err := NewRenderer()
	assert.NoError(t, err)

	data := WorkbenchData{
		LayoutData: LayoutData{
			Title:      "Ze: /bgp/peer",
			Content:    template.HTML(`<div id="workspace-marker">workspace-content</div>`),
			HasSession: true,
			Username:   "alice",
			CLIPrompt:  "ze>",
		},
		Sections: WorkbenchSections([]string{"bgp", "peer"}),
	}

	rec := httptest.NewRecorder()
	assert.NoError(t, r.RenderWorkbench(rec, data))

	body := rec.Body.String()

	// Body class identifies the active UI mode for tests and CSS scoping.
	assert.Contains(t, body, `class="ui-workbench"`, "missing ui-workbench body class")

	// Workbench shell regions are required by the spec (Layout Regions table).
	assert.Contains(t, body, `id="workbench-shell"`, "missing workbench shell")
	assert.Contains(t, body, `data-ui-mode="workbench"`, "missing data-ui-mode marker")
	assert.Contains(t, body, `id="workbench-topbar"`, "missing workbench topbar")
	assert.Contains(t, body, `id="workbench-nav"`, "missing workbench left nav")
	assert.Contains(t, body, `id="workbench-workspace"`, "missing workbench workspace")
	assert.Contains(t, body, `id="commit-bar"`, "commit bar must be reused under workbench")

	// Workspace renders the supplied content.
	assert.Contains(t, body, `workspace-content`, "workspace must render Content")

	// Identity surfaces in the top bar.
	assert.Contains(t, body, `alice`, "top bar must show username")

	// Section navigation includes the spec sections.
	for _, want := range []string{"Dashboard", "Interfaces", "Routing", "Policy", "Firewall", "Services", "System", "Tools", "Logs"} {
		assert.Contains(t, body, want, "missing nav section "+want)
	}
}

// TestRenderWorkbench_NoFinderMarkers verifies that the workbench shell does
// NOT include Finder-only markers. Mixing Finder columns into the workbench
// would defeat the table-first design and confuse the rollback-mode test.
//
// VALIDATES: AC-1a (workbench markers are absent from Finder, and vice versa).
// PREVENTS: Accidental Finder column reuse leaking into the V2 chrome.
func TestRenderWorkbench_NoFinderMarkers(t *testing.T) {
	r, err := NewRenderer()
	assert.NoError(t, err)

	data := WorkbenchData{
		LayoutData: LayoutData{
			Title:   "Ze: /",
			Content: template.HTML(`<p>workspace</p>`),
		},
		Sections: WorkbenchSections(nil),
	}

	rec := httptest.NewRecorder()
	assert.NoError(t, r.RenderWorkbench(rec, data))

	body := rec.Body.String()

	// The Finder layout uses a top breadcrumb-bar (not the workbench topbar)
	// and does not have a workbench-shell wrapper. The workbench page uses a
	// workbench-breadcrumb inside its topbar, so the bare Finder marker
	// `class="breadcrumb-bar"` must not appear.
	assert.NotContains(t, body, `<nav class="breadcrumb-bar"`, "Finder breadcrumb-bar must not appear in workbench")
	assert.NotContains(t, body, `class="content-area"`, "Finder content-area must not appear in workbench")
	// Finder Sidebar/Finder columns belong in oob_response, not the workbench
	// shell. Their absence here is what the rollback test will rely on.
	assert.NotContains(t, body, `id="main-split"`, "Finder main-split must not appear in workbench")
}

// TestRenderLayout_NoWorkbenchMarkers verifies the inverse: the existing
// Finder layout does not pick up workbench markers. This is the rollback
// contract -- starting the hub with `ze.web.ui=finder` (or the Phases 1-3
// default) must produce a page free of V2 markers.
//
// VALIDATES: AC-1a (Finder pages carry no workbench markers).
// PREVENTS: A future shared template change accidentally bleeding workbench
// markers into the rollback path, which would silently break Phase 7 cleanup.
func TestRenderLayout_NoWorkbenchMarkers(t *testing.T) {
	r, err := NewRenderer()
	assert.NoError(t, err)

	data := LayoutData{
		Title:      "Ze: /",
		Content:    template.HTML("<p>finder-content</p>"),
		HasSession: true,
		CLIPrompt:  "ze>",
	}

	rec := httptest.NewRecorder()
	assert.NoError(t, r.RenderLayout(rec, data))

	body := rec.Body.String()

	assert.NotContains(t, body, `id="workbench-shell"`)
	assert.NotContains(t, body, `data-ui-mode="workbench"`)
	assert.NotContains(t, body, `id="workbench-nav"`)
	assert.NotContains(t, body, `id="workbench-topbar"`)
	assert.NotContains(t, body, `class="ui-workbench"`)
}

// TestWorkbenchSections_DefaultDashboard verifies the root path activates the
// Dashboard section, not Routing, Interfaces, or any other entry.
//
// VALIDATES: Section selection at /show/ root.
// PREVENTS: The dashboard losing its highlight, which would mislead the
// operator about which section they are in.
func TestWorkbenchSections_DefaultDashboard(t *testing.T) {
	got := WorkbenchSections(nil)
	assertOnlySelected(t, got, "dashboard")
}

// TestWorkbenchSections_BGPRouting verifies that paths under bgp activate the
// Routing section (the canonical Phase 4 acceptance workflow lives here).
//
// VALIDATES: Spec D12 -- BGP peer screen is the first complete workflow; the
// nav must put the operator in Routing when they are on a BGP path.
func TestWorkbenchSections_BGPRouting(t *testing.T) {
	got := WorkbenchSections([]string{"bgp", "peer", "thomas"})
	assertOnlySelected(t, got, "routing")
}

// TestWorkbenchSections_BGPPolicy verifies that paths under bgp/policy
// activate Policy, not Routing -- policy work is its own operator section.
//
// PREVENTS: Policy edits highlighting Routing and confusing the operator.
func TestWorkbenchSections_BGPPolicy(t *testing.T) {
	got := WorkbenchSections([]string{"bgp", "policy", "drop-bogons"})
	assertOnlySelected(t, got, "policy")
}

// TestWorkbenchSections_Interfaces verifies the Interfaces section lights up
// for paths under iface.
func TestWorkbenchSections_Interfaces(t *testing.T) {
	got := WorkbenchSections([]string{"iface", "eth0"})
	assertOnlySelected(t, got, "interfaces")
}

// TestWorkbenchSectionModel_BGP is the spec-listed test name covering the
// canonical Phase 1 expectation: bgp lives under Routing in the left nav.
//
// VALIDATES: Spec TDD entry "TestWorkbenchSectionModel_BGP" (Phase 5 row in
// spec, but the section model is built in Phase 1).
func TestWorkbenchSectionModel_BGP(t *testing.T) {
	got := WorkbenchSections([]string{"bgp"})
	var routing *WorkbenchSection
	for i := range got {
		if got[i].Key == "routing" {
			routing = &got[i]
			break
		}
	}
	assert.NotNil(t, routing, "Routing section must exist")
	assert.True(t, routing.Selected, "Routing section must be selected for /show/bgp/")
	assert.Equal(t, "Routing", routing.Label)
	assert.True(t, strings.HasPrefix(routing.URL, "/show/bgp"), "Routing URL must point at BGP")
}

// TestRenderWorkbenchNav_TwoLevel verifies the nav renders nested sub-lists.
func TestRenderWorkbenchNav_TwoLevel(t *testing.T) {
	r, err := NewRenderer()
	assert.NoError(t, err)

	data := WorkbenchData{
		LayoutData: LayoutData{
			Title:   "Ze: /",
			Content: template.HTML(`<p>workspace</p>`),
		},
		Sections: WorkbenchSections(nil),
	}

	rec := httptest.NewRecorder()
	assert.NoError(t, r.RenderWorkbench(rec, data))

	body := rec.Body.String()

	assert.Contains(t, body, `workbench-nav-sublist`, "nav must contain nested sub-lists")
	assert.Contains(t, body, `workbench-nav-sublink`, "nav must contain sub-page links")
	assert.Contains(t, body, `<details`, "nav must use details/summary for sections")
	assert.Contains(t, body, `<summary`, "nav must use details/summary for sections")
}

// TestRenderWorkbenchNav_ActiveHighlight verifies the active sub-page has the
// correct CSS class for highlighting.
func TestRenderWorkbenchNav_ActiveHighlight(t *testing.T) {
	r, err := NewRenderer()
	assert.NoError(t, err)

	data := WorkbenchData{
		LayoutData: LayoutData{
			Title:   "Ze: /bgp/peer",
			Content: template.HTML(`<p>peers</p>`),
		},
		Sections: WorkbenchSections([]string{"bgp", "peer"}),
	}

	rec := httptest.NewRecorder()
	assert.NoError(t, r.RenderWorkbench(rec, data))

	body := rec.Body.String()

	assert.Contains(t, body, `workbench-nav-subitem--active`, "active sub-page must have active class")
	assert.Contains(t, body, `aria-current="page"`, "active sub-page must have aria-current")
}

// TestRenderWorkbenchNav_ExpandedSection verifies that the active section's
// details element has the open attribute.
func TestRenderWorkbenchNav_ExpandedSection(t *testing.T) {
	r, err := NewRenderer()
	assert.NoError(t, err)

	data := WorkbenchData{
		LayoutData: LayoutData{
			Title:   "Ze: /bgp/peer",
			Content: template.HTML(`<p>peers</p>`),
		},
		Sections: WorkbenchSections([]string{"bgp", "peer"}),
	}

	rec := httptest.NewRecorder()
	assert.NoError(t, r.RenderWorkbench(rec, data))

	body := rec.Body.String()

	// The routing section should have open attribute; check that "open" appears
	// in a details element with data-section="routing".
	assert.Contains(t, body, `workbench-nav-section--active`, "active section must have active class")

	// Verify the active section has the open attribute. The template renders
	// the open attribute after data-section, so look for the closing ">" after
	// data-section="routing" and check for "open" in between.
	routingIdx := strings.Index(body, `data-section="routing"`)
	assert.Greater(t, routingIdx, 0, "routing section must exist in HTML")

	// Find the closing > of the details tag after data-section="routing".
	afterRouting := body[routingIdx:]
	closingBracket := strings.Index(afterRouting, ">")
	assert.Greater(t, closingBracket, 0, "details tag must close after data-section")
	detailsSuffix := afterRouting[:closingBracket]
	assert.Contains(t, detailsSuffix, "open", "active section details must have open attribute")
}

// assertOnlySelected verifies exactly one section in the list has Selected=true,
// and that section's Key matches wantKey.
func assertOnlySelected(t *testing.T, sections []WorkbenchSection, wantKey string) {
	t.Helper()
	selectedKeys := []string{}
	for _, s := range sections {
		if s.Selected {
			selectedKeys = append(selectedKeys, s.Key)
		}
	}
	assert.Equal(t, []string{wantKey}, selectedKeys, "expected exactly one selected section")
}
