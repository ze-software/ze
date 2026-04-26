// Design: docs/architecture/web-components.md -- Workbench table component
// Related: workbench_sections.go -- Navigation model
//
// Spec: plan/spec-web-3-foundation.md (Table component, Phase 3).
//
// WorkbenchTableData is the reusable table component data model. It drives
// the workbench_table.html template, rendering sortable columns, flag
// indicators, row actions (links and HTMX buttons), and an empty state.

package web

// Flag color class constants used in WorkbenchTableRow.FlagClass.
const (
	flagClassGreen  = "green"
	flagClassRed    = "red"
	flagClassGrey   = "grey"
	flagClassYellow = "yellow"
)

// WorkbenchTableData holds the data for a workbench table component.
type WorkbenchTableData struct {
	Title        string
	AddURL       string
	AddLabel     string
	Columns      []WorkbenchTableColumn
	Rows         []WorkbenchTableRow
	EmptyMessage string
	EmptyHint    string
}

// WorkbenchTableColumn describes one column header in the table.
type WorkbenchTableColumn struct {
	Key      string
	Label    string
	Sortable bool
}

// WorkbenchTableRow describes one data row in the table.
type WorkbenchTableRow struct {
	Key       string
	URL       string
	Flags     string
	FlagClass string // "red", "grey", "green", or ""
	Cells     []string
	Actions   []WorkbenchRowAction
	Pending   bool
}

// WorkbenchRowAction describes a single action button or link on a row.
type WorkbenchRowAction struct {
	Label   string
	URL     string
	HxPost  string
	Class   string // "danger" for destructive actions
	Confirm string
}
