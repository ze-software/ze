// Design: docs/architecture/web-components.md -- Workbench detail panel
// Related: workbench_table.go -- Table component (sibling)
// Related: render.go -- Fragment rendering
//
// Spec: plan/spec-web-3-foundation.md (Detail panel, Phase 4).
//
// WorkbenchDetailData drives the workbench_detail.html template, rendering
// a tabbed detail panel with a close button and related tool actions.

package web

import "html/template"

// WorkbenchDetailData holds the data for a detail panel component.
type WorkbenchDetailData struct {
	Title    string
	Tabs     []WorkbenchDetailTab
	CloseURL string
	Tools    []WorkbenchDetailTool
}

// WorkbenchDetailTab describes one tab in the detail panel.
type WorkbenchDetailTab struct {
	Key     string
	Label   string
	Content template.HTML
	Active  bool
}

// WorkbenchDetailTool describes a related-tool button in the panel footer.
type WorkbenchDetailTool struct {
	Label   string
	HxPost  string
	Class   string // "inspect", "diagnose", "refresh", "danger"
	Confirm string
}
