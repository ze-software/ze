package web

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderWorkbenchDetail renders a detail panel with tabs, close button,
// and related tools, then verifies all structural elements are present.
func TestRenderWorkbenchDetail(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchDetailData{
		Title:    "Peer: neighbor-1",
		CloseURL: "/show/bgp/peer/",
		Tabs: []WorkbenchDetailTab{
			{Key: "config", Label: "Config", Content: template.HTML("<p>config content</p>"), Active: true},
			{Key: "status", Label: "Status", Content: template.HTML("<p>status content</p>")},
			{Key: "actions", Label: "Actions", Content: template.HTML("<p>actions content</p>")},
		},
		Tools: []WorkbenchDetailTool{
			{Label: "Detail", HxPost: "/tools/related/run?tool=peer-detail", Class: "inspect"},
			{Label: "Teardown", HxPost: "/tools/related/run?tool=peer-teardown", Class: "danger", Confirm: "Tear down session?"},
		},
	}

	html := string(r.RenderFragment("workbench_detail", data))
	require.NotEmpty(t, html, "detail fragment must render")

	// Panel structure.
	assert.Contains(t, html, `wb-detail-panel`)
	assert.Contains(t, html, `Peer: neighbor-1`)
	assert.Contains(t, html, `wb-detail-close`)
	assert.Contains(t, html, `/show/bgp/peer/`, "close URL must be present")

	// Tabs.
	assert.Contains(t, html, `wb-detail-tab--active`)
	assert.Contains(t, html, `Config`)
	assert.Contains(t, html, `Status`)
	assert.Contains(t, html, `Actions`)

	// Tab content.
	assert.Contains(t, html, `config content`)
	assert.Contains(t, html, `status content`)
	assert.Contains(t, html, `actions content`)
	assert.Contains(t, html, `wb-detail-content--active`)

	// Tools.
	assert.Contains(t, html, `Detail`)
	assert.Contains(t, html, `Teardown`)
	assert.Contains(t, html, `wb-detail-tool--danger`)
	assert.Contains(t, html, `hx-confirm="Tear down session?"`)
}

// TestRenderWorkbenchDetail_TabSwitching verifies that only the active tab's
// content div gets the active class, while all tab content is present in HTML.
func TestRenderWorkbenchDetail_TabSwitching(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchDetailData{
		Title: "Test",
		Tabs: []WorkbenchDetailTab{
			{Key: "tab1", Label: "Tab 1", Content: template.HTML("<p>tab1</p>"), Active: false},
			{Key: "tab2", Label: "Tab 2", Content: template.HTML("<p>tab2</p>"), Active: true},
		},
	}

	html := string(r.RenderFragment("workbench_detail", data))
	require.NotEmpty(t, html, "detail fragment must render")

	// Both tab contents should be present in the HTML.
	assert.Contains(t, html, "tab1")
	assert.Contains(t, html, "tab2")

	// The tab2 content div should have the active class.
	assert.Contains(t, html, `data-tab-content="tab2"`)
}

// TestRenderWorkbenchDetail_NoTools verifies the panel renders without a tools
// section when no tools are provided.
func TestRenderWorkbenchDetail_NoTools(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchDetailData{
		Title: "Minimal",
		Tabs: []WorkbenchDetailTab{
			{Key: "info", Label: "Info", Content: template.HTML("<p>info</p>"), Active: true},
		},
	}

	html := string(r.RenderFragment("workbench_detail", data))
	require.NotEmpty(t, html, "detail fragment must render")

	assert.Contains(t, html, `Minimal`)
	assert.NotContains(t, html, `wb-detail-tools`, "tools section must be absent when no tools")
}
