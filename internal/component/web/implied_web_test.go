// Design: docs/architecture/web-components.md -- graphical inspection show | implied and read-only workbench implied rows.
// Related: graphical_inspection.go -- graphicalInspectionWords.
// Related: workbench_enrich.go -- enrichWorkbenchTable and appendImpliedWorkbenchRows.
// Related: component_list_table.templ -- listTable implied row rendering.
// Related: component_workbench_table.templ -- workbenchTable implied row rendering.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestGraphicalInspectionWordsImplied verifies that graphicalInspectionWords
// emits `show [confirmed|saved] | implied` and
// `show [confirmed|saved] | implied | format config` for the inspect form,
// supports `view=implied` under inspect-metadata (candidate-only), and refuses
// invalid implied flags.
//
// VALIDATES: AC-23 (web graphical inspection offers show | implied).
// PREVENTS: graphical inspection dropping the implied pipe or permitting
// unvalidated form parameters.
func TestGraphicalInspectionWordsImplied(t *testing.T) {
	cases := []struct {
		name      string
		form      url.Values
		wantWords []string
		wantLine  string
	}{
		{
			name:      "candidate tree implied",
			form:      url.Values{"action": {"inspect"}, "source": {"candidate"}, "format": {"tree"}, "implied": {"true"}},
			wantWords: []string{"show", "|", "implied"},
			wantLine:  "show | implied",
		},
		{
			name:      "candidate config implied",
			form:      url.Values{"action": {"inspect"}, "source": {"candidate"}, "format": {"config"}, "implied": {"true"}},
			wantWords: []string{"show", "|", "implied", "|", "format", "config"},
			wantLine:  "show | implied | format config",
		},
		{
			name:      "confirmed tree implied",
			form:      url.Values{"action": {"inspect"}, "source": {"confirmed"}, "format": {"tree"}, "implied": {"true"}},
			wantWords: []string{"show", "confirmed", "|", "implied"},
			wantLine:  "show confirmed | implied",
		},
		{
			name:      "confirmed config implied",
			form:      url.Values{"action": {"inspect"}, "source": {"confirmed"}, "format": {"config"}, "implied": {"true"}},
			wantWords: []string{"show", "confirmed", "|", "implied", "|", "format", "config"},
			wantLine:  "show confirmed | implied | format config",
		},
		{
			name:      "saved tree implied",
			form:      url.Values{"action": {"inspect"}, "source": {"saved"}, "format": {"tree"}, "implied": {"true"}},
			wantWords: []string{"show", "saved", "|", "implied"},
			wantLine:  "show saved | implied",
		},
		{
			name:      "saved config implied",
			form:      url.Values{"action": {"inspect"}, "source": {"saved"}, "format": {"config"}, "implied": {"true"}},
			wantWords: []string{"show", "saved", "|", "implied", "|", "format", "config"},
			wantLine:  "show saved | implied | format config",
		},
		{
			name:      "metadata view implied",
			form:      url.Values{"action": {"inspect-metadata"}, "view": {"implied"}},
			wantWords: []string{"show", "|", "implied"},
			wantLine:  "show | implied",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/config/action", strings.NewReader(tc.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			require.NoError(t, req.ParseForm())

			words, handled, err := graphicalInspectionWords(req)
			require.NoError(t, err)
			assert.True(t, handled)
			assert.Equal(t, tc.wantWords, words)
			assert.Equal(t, tc.wantLine, graphicalCommandLine(words))
		})
	}

	// Invalid implied value is refused.
	badInspect := httptest.NewRequest(http.MethodPost, "/config/action",
		strings.NewReader(url.Values{"action": {"inspect"}, "source": {"candidate"}, "format": {"tree"}, "implied": {"bogus"}}.Encode()))
	badInspect.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	require.NoError(t, badInspect.ParseForm())
	_, handled, err := graphicalInspectionWords(badInspect)
	assert.True(t, handled)
	require.Error(t, err)

	// Metadata implied refuses non-candidate source.
	badMeta := httptest.NewRequest(http.MethodPost, "/config/action",
		strings.NewReader(url.Values{"action": {"inspect-metadata"}, "view": {"implied"}, "source": {"saved"}}.Encode()))
	badMeta.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	require.NoError(t, badMeta.ParseForm())
	_, handled, err = graphicalInspectionWords(badMeta)
	assert.True(t, handled)
	require.Error(t, err)

	// Rendered inspection panel exposes both implied controls.
	renderer, err := NewRenderer()
	require.NoError(t, err)
	panelHTML := string(renderer.renderComponent("graphical_inspection", graphicalInspectionPanel(&GraphicalActions{Path: "bgp"}, true)))
	assert.Contains(t, panelHTML, `id="graphical-inspect-implied"`)
	assert.Contains(t, panelHTML, `value="implied"`)
}

// TestEnrichWorkbenchTableImpliedBusRows verifies that enrichWorkbenchTable
// appends read-only Implied rows for auto-loaded bus endpoints and implied bus
// participants without duplicating already-written rows.
//
// VALIDATES: AC-23 (workbench tree pages show implied rows read-only with origin,
// and written endpoints appear once as written).
// PREVENTS: duplicating written endpoints as implied or omitting auto-loaded rows.
func TestEnrichWorkbenchTableImpliedBusRows(t *testing.T) {
	restore := config.ResetImpliedProvidersForTest()
	t.Cleanup(restore)

	config.RegisterImpliedBusProvider(func(_ *config.Tree, _ config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
		return []config.ImpliedEndpoint{
				{
					Name:           "written-ep",
					CompiledPlugin: "written-ep",
					ReasonKind:     "config-path",
					ReasonSource:   "bgp",
					ImpliedBy:      "config section bgp",
				},
				{
					Name:           "auto-ep",
					CompiledPlugin: "auto-ep",
					ReasonKind:     "config-path",
					ReasonSource:   "bgp",
					ImpliedBy:      "config section bgp",
				},
			},
			[]config.ImpliedParticipant{
				{
					Path: "/bus/endpoint/written-part",
					Receive: []config.ImpliedDirectionRule{
						{
							Message: "demo:msg",
							Allowed: true,
							Origin:  config.ImpliedOriginRegistered,
							Source:  "registration of written-part",
						},
					},
				},
				{
					Path: "/bus/endpoint/auto-ep",
					Receive: []config.ImpliedDirectionRule{
						{
							Message: "demo:msg",
							Allowed: true,
							Origin:  config.ImpliedOriginRegistered,
							Source:  "registration of auto-ep",
						},
					},
				},
				{
					Path: "/bus/endpoint/explicit-only",
					Receive: []config.ImpliedDirectionRule{
						{
							Message: "demo:msg",
							Allowed: true,
							Origin:  config.ImpliedOriginWritten,
							Source:  "written",
						},
					},
				},
			},
			nil
	})

	schema, err := config.YANGSchema()
	require.NoError(t, err)

	tree := config.NewTree()
	busTree := tree.GetOrCreateContainer("bus")
	writtenEP := config.NewTree()
	writtenEP.Set("internal", "written-ep")
	busTree.AddListEntry("endpoint", "written-ep", writtenEP)
	busTree.AddListEntry("participant", "/bus/endpoint/written-part", config.NewTree())

	// 1. bus > endpoint table: written-ep appears once (not implied), auto-ep appears as Implied.
	epData := buildFragmentData(schema, tree, []string{"bus", "endpoint"})
	enrichWorkbenchTable(epData, schema, tree, []string{"bus", "endpoint"}, nil)
	require.NotNil(t, epData.ListTable)
	require.Len(t, epData.ListTable.Rows, 2, "must have 1 written row and 1 implied row without duplicating written-ep")

	assert.Equal(t, "written-ep", epData.ListTable.Rows[0].KeyValue)
	assert.False(t, epData.ListTable.Rows[0].Implied)

	assert.Equal(t, "auto-ep", epData.ListTable.Rows[1].KeyValue)
	assert.True(t, epData.ListTable.Rows[1].Implied)
	assert.Equal(t, "config section bgp", epData.ListTable.Rows[1].ImpliedBy)
	assert.Empty(t, epData.ListTable.Rows[1].RowTools)
	assert.False(t, epData.ListTable.Rows[1].HasPendingChanges)

	// 2. bus > participant table: written-part not duplicated, explicit-only skipped, auto-ep appended as Implied.
	partData := buildFragmentData(schema, tree, []string{"bus", "participant"})
	enrichWorkbenchTable(partData, schema, tree, []string{"bus", "participant"}, nil)
	require.NotNil(t, partData.ListTable)
	require.Len(t, partData.ListTable.Rows, 2)

	assert.Equal(t, "/bus/endpoint/written-part", partData.ListTable.Rows[0].KeyValue)
	assert.False(t, partData.ListTable.Rows[0].Implied)

	assert.Equal(t, "/bus/endpoint/auto-ep", partData.ListTable.Rows[1].KeyValue)
	assert.True(t, partData.ListTable.Rows[1].Implied)
	assert.Equal(t, "registration of auto-ep", partData.ListTable.Rows[1].ImpliedBy)
}

// TestRenderImpliedRowsReadOnlyWithBadge verifies that both listTable and
// workbenchTable render an Implied row read-only with the `implied` badge and
// ImpliedBy reason, emitting zero rename, delete, or inline edit controls for
// that row even when the page itself is editable.
//
// VALIDATES: AC-23 (workbench tree pages show implied rows read-only with a badge).
// PREVENTS: rendering edit/delete/rename controls on implied rows that do not
// exist as written configuration entries.
func TestRenderImpliedRowsReadOnlyWithBadge(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	// 1. listTable with an editable written row and an implied row.
	frag := &FragmentData{
		ReadOnly: false,
		ActiveUI: uiModeTokenWorkbench,
		ListTable: &ListTableView{
			Name:    "endpoint",
			FormURL: "/config/add-form/bus/endpoint/",
			Columns: []ListTableColumn{
				{Name: "name", Key: true},
				{Name: "internal"},
			},
			Rows: []ListTableRow{
				{
					KeyValue: "written-ep",
					URL:      "/show/bus/endpoint/written-ep/",
					HxPath:   "bus/endpoint/written-ep",
					Cells: []ListTableCell{
						{Value: "written-ep", Leaf: "internal", Path: "bus/endpoint/written-ep"},
					},
				},
				{
					KeyValue:  "auto-ep",
					Cells:     []ListTableCell{{Value: "auto-ep"}},
					Implied:   true,
					ImpliedBy: "config section bgp",
				},
			},
		},
	}

	listHTML := string(renderer.renderComponent("list_table", listTable(frag)))
	impliedRowStart := strings.Index(listHTML, `data-row-key="auto-ep"`)
	require.Greater(t, impliedRowStart, 0, "implied row must appear in listTable")
	impliedRowEnd := strings.Index(listHTML[impliedRowStart:], "</tr>")
	require.Greater(t, impliedRowEnd, 0)
	impliedListTR := listHTML[impliedRowStart : impliedRowStart+impliedRowEnd]

	assert.Contains(t, impliedListTR, `data-implied`)
	assert.Contains(t, impliedListTR, `class="badge badge-implied"`)
	assert.Contains(t, impliedListTR, `title="config section bgp"`)
	assert.Contains(t, impliedListTR, `>implied</span>`)
	assert.Contains(t, impliedListTR, `config section bgp`)
	assert.NotContains(t, impliedListTR, `finder-rename-btn`, "implied row must emit no rename button")
	assert.NotContains(t, impliedListTR, `finder-delete-btn`, "implied row must emit no delete button")
	assert.NotContains(t, impliedListTR, `<input`, "implied row must emit no editable input")
	assert.NotContains(t, impliedListTR, `hx-post=`, "implied row must emit no mutating HTMX action")

	// Written sibling row stays editable.
	writtenRowStart := strings.Index(listHTML, `data-row-key="written-ep"`)
	require.Greater(t, writtenRowStart, 0)
	writtenRowEnd := strings.Index(listHTML[writtenRowStart:], "</tr>")
	require.Greater(t, writtenRowEnd, 0)
	writtenListTR := listHTML[writtenRowStart : writtenRowStart+writtenRowEnd]
	assert.Contains(t, writtenListTR, `finder-rename-btn`)
	assert.Contains(t, writtenListTR, `finder-delete-btn`)
	assert.Contains(t, writtenListTR, `<input`)

	// 2. workbenchTable with a written row and an implied row (even if Actions is populated).
	wbData := WorkbenchTableData{
		Title: "Bus Endpoints",
		Columns: []WorkbenchTableColumn{
			{Key: "name", Label: "Name"},
			{Key: "kind", Label: "Kind"},
		},
		Rows: []WorkbenchTableRow{
			{
				Key:   "written-ep",
				Flags: "W",
				Cells: []string{"written-ep", "internal"},
				Actions: []WorkbenchRowAction{
					{Label: "Delete", HxPost: "/config/delete/bus/endpoint/written-ep", Class: "danger"},
				},
			},
			{
				Key:       "auto-ep",
				Cells:     []string{"auto-ep", "internal"},
				Implied:   true,
				ImpliedBy: "config section bgp",
				Actions: []WorkbenchRowAction{
					{Label: "Delete", HxPost: "/config/delete/bus/endpoint/auto-ep", Class: "danger"},
					{Label: "Edit", URL: "/show/bus/endpoint/auto-ep/"},
				},
			},
		},
	}

	wbHTML := string(renderer.renderComponent("workbench_table", workbenchTable(wbData)))
	wbImpliedStart := strings.Index(wbHTML, `data-key="auto-ep"`)
	require.Greater(t, wbImpliedStart, 0, "implied row must appear in workbenchTable")
	wbImpliedEnd := strings.Index(wbHTML[wbImpliedStart:], "</tr>")
	require.Greater(t, wbImpliedEnd, 0)
	wbImpliedTR := wbHTML[wbImpliedStart : wbImpliedStart+wbImpliedEnd]

	assert.Contains(t, wbImpliedTR, `data-implied`)
	assert.Contains(t, wbImpliedTR, `class="badge badge-implied"`)
	assert.Contains(t, wbImpliedTR, `title="config section bgp"`)
	assert.Contains(t, wbImpliedTR, `>implied</span>`)
	assert.Contains(t, wbImpliedTR, `config section bgp`)
	assert.NotContains(t, wbImpliedTR, `<button`, "implied workbench row must emit no action buttons")
	assert.NotContains(t, wbImpliedTR, `<a `, "implied workbench row must emit no edit links")
	assert.NotContains(t, wbImpliedTR, `hx-post=`, "implied workbench row must emit no mutating HTMX action")
}
