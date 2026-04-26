package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderWorkbenchTable renders a table with columns, rows, and actions,
// then verifies toolbar, headers, flag column, cells, and action buttons.
func TestRenderWorkbenchTable(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchTableData{
		Title:  "BGP Peers",
		AddURL: "/admin/bgp/peer/add",
		Columns: []WorkbenchTableColumn{
			{Key: "name", Label: "Name", Sortable: true},
			{Key: "state", Label: "State", Sortable: false},
			{Key: "uptime", Label: "Uptime", Sortable: true},
		},
		Rows: []WorkbenchTableRow{
			{
				Key:       "peer-1",
				Flags:     "E",
				FlagClass: "green",
				Cells:     []string{"peer-1.example.com", "Established", "3d 12h"},
				Actions: []WorkbenchRowAction{
					{Label: "View", URL: "/show/bgp/peer/peer-1"},
					{Label: "Disable", HxPost: "/admin/bgp/peer/peer-1/disable", Class: "danger", Confirm: "Disable peer-1?"},
				},
			},
			{
				Key:       "peer-2",
				Flags:     "D",
				FlagClass: "grey",
				Cells:     []string{"peer-2.example.com", "Idle", "-"},
				Pending:   true,
				Actions: []WorkbenchRowAction{
					{Label: "View", URL: "/show/bgp/peer/peer-2"},
				},
			},
		},
	}

	html := string(r.RenderFragment("workbench_table", data))
	require.NotEmpty(t, html, "table fragment must render")

	// Toolbar
	assert.Contains(t, html, `wb-table-toolbar`, "toolbar must be present")
	assert.Contains(t, html, `wb-table-add`, "add button must be present")
	assert.Contains(t, html, `wb-table-search`, "search input must be present")

	// Headers
	assert.Contains(t, html, `wb-table-flag-header`, "flag header must be present")
	assert.Contains(t, html, `Name`, "Name column header must be present")
	assert.Contains(t, html, `State`, "State column header must be present")
	assert.Contains(t, html, `Uptime`, "Uptime column header must be present")
	assert.Contains(t, html, `wb-table-col--sortable`, "sortable column must have class")
	assert.Contains(t, html, `wb-table-actions-header`, "actions header must be present")

	// Rows
	assert.Contains(t, html, `data-key="peer-1"`, "row key must be present")
	assert.Contains(t, html, `data-key="peer-2"`, "second row key must be present")
	assert.Contains(t, html, `wb-table-row--pending`, "pending row must have class")

	// Flags
	assert.Contains(t, html, `wb-table-flag--green`, "green flag class must be present")
	assert.Contains(t, html, `wb-table-flag--grey`, "grey flag class must be present")

	// Cells
	assert.Contains(t, html, `peer-1.example.com`, "cell content must be present")
	assert.Contains(t, html, `Established`, "cell content must be present")

	// Actions
	assert.Contains(t, html, `hx-post="/admin/bgp/peer/peer-1/disable"`, "HTMX post action must be present")
	assert.Contains(t, html, `hx-confirm="Disable peer-1?"`, "confirm attribute must be present")
	assert.Contains(t, html, `wb-action--danger`, "danger action class must be present")
	assert.Contains(t, html, `href="/show/bgp/peer/peer-1"`, "link action must be present")
}

// TestRenderWorkbenchTable_EmptyState renders a table with zero rows and
// verifies the empty message and add button appear.
func TestRenderWorkbenchTable_EmptyState(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchTableData{
		Title:        "BGP Peers",
		AddURL:       "/admin/bgp/peer/add",
		AddLabel:     "Add Peer",
		EmptyMessage: "No peers configured",
		EmptyHint:    "Add a BGP peer to get started.",
		Columns: []WorkbenchTableColumn{
			{Key: "name", Label: "Name"},
		},
	}

	html := string(r.RenderFragment("workbench_table", data))
	require.NotEmpty(t, html, "table fragment must render")

	assert.Contains(t, html, `wb-table-empty-row`, "empty row must be present")
	assert.Contains(t, html, `No peers configured`, "empty message must appear")
	assert.Contains(t, html, `Add a BGP peer to get started.`, "empty hint must appear")
	assert.Contains(t, html, `wb-table-empty-add`, "add button must appear in empty state")
	assert.Contains(t, html, `Add Peer`, "custom add label must appear")

	// Must not contain data rows.
	assert.NotContains(t, html, `wb-table-row"`, "no data rows in empty state")
}

// TestRenderWorkbenchTable_FlagColors renders rows with different FlagClass
// values and verifies the correct CSS classes are applied.
func TestRenderWorkbenchTable_FlagColors(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchTableData{
		Columns: []WorkbenchTableColumn{
			{Key: "name", Label: "Name"},
		},
		Rows: []WorkbenchTableRow{
			{Key: "r1", Flags: "E", FlagClass: "green", Cells: []string{"row1"}},
			{Key: "r2", Flags: "D", FlagClass: "grey", Cells: []string{"row2"}},
			{Key: "r3", Flags: "X", FlagClass: "red", Cells: []string{"row3"}},
			{Key: "r4", Flags: "", FlagClass: "", Cells: []string{"row4"}},
		},
	}

	html := string(r.RenderFragment("workbench_table", data))
	require.NotEmpty(t, html, "table fragment must render")

	assert.Contains(t, html, `wb-table-flag--green`, "green flag must be present")
	assert.Contains(t, html, `wb-table-flag--grey`, "grey flag must be present")
	assert.Contains(t, html, `wb-table-flag--red`, "red flag must be present")

	// Row with empty FlagClass should have wb-table-flag but no color modifier.
	r4Idx := strings.Index(html, `data-key="r4"`)
	require.Greater(t, r4Idx, 0, "r4 row must exist")
	// The flag cell for r4 should not have a color modifier class.
	r4Section := html[r4Idx : r4Idx+200]
	assert.Contains(t, r4Section, `class="wb-table-flag"`, "empty flag class should have no modifier")
}
