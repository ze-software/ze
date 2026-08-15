// Design: docs/architecture/web-components.md -- HTMX fragment handlers
// Related: fragment.go -- FragmentData, the view model most fragments render
// Related: component_breadcrumb.templ -- the chrome chromeData serves

package web

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// chromeData is the page chrome's view model: who is signed in, where they
// are, and which shells they can switch to.
//
// It exists because three view models carry the same five values and the same
// chrome renders from all three. LayoutData drives the Finder and CLI pages,
// workbenchData drives the workbench shell, and FragmentData drives the
// out-of-band swap. A templ component takes one type, so the three converge
// here rather than the chrome being written three times.
type chromeData struct {
	Breadcrumbs []BreadcrumbSegment
	Username    string
	Insecure    bool
	ActiveUI    string
	Services    []PortalService
}

// chrome is the page chrome of a full page.
func (d LayoutData) chrome() chromeData {
	return chromeData{
		Breadcrumbs: d.Breadcrumbs,
		Username:    d.Username,
		Insecure:    d.Insecure,
		ActiveUI:    d.ActiveUI,
		Services:    d.Services,
	}
}

// chrome is the page chrome of an out-of-band fragment response.
func (d *FragmentData) chrome() chromeData {
	return chromeData{
		Breadcrumbs: d.Breadcrumbs,
		Username:    d.Username,
		Insecure:    d.Insecure,
		ActiveUI:    d.ActiveUI,
		Services:    d.Services,
	}
}

// saveOKData is what oobSaveOK renders. It carries the pending-change count
// after an edit, and whether the authorizer lets this session commit it.
//
// ReadOnly is the gate commitBar reads from LayoutData. Both components render
// the one element #commit-bar. The out-of-band one replaces what the page one
// drew, so a flag on one half is a flag the other half undoes.
//
// The route gate (RequireEditAuthz) authorizes "config commit". A set or a form
// post authorizes "config set". A session denied the first and allowed the
// second reaches this component with the buttons the page withheld.
type saveOKData struct {
	ChangeCount int
	ReadOnly    bool
}

// notificationErrorData is what notificationError renders.
type notificationErrorData struct {
	Message string
}

// terminalPageData is what terminalPage renders. See terminal.templ: no live
// path reaches it.
type terminalPageData struct {
	CLIPrompt string
}

// addFormField is one field of the add-entry overlay. Category is what the
// YANG list said about it: required, suggest or unique.
type addFormField struct {
	Path        string
	Placeholder string
	Category    string
	Inherited   string
}

// addFormData is what addFormOverlay renders.
type addFormData struct {
	AddURL          string
	ListName        string
	Heading         string
	KeyName         string
	KeyLabel        string
	KeyInputID      string
	Keyless         bool
	DisplayKey      string
	Workbench       bool
	IncludeKeyField bool
	Fields          []addFormField
}

// addFieldRequired is the Category of a field the YANG list marks ze:required.
// It is the one category the overlay marks with an asterisk and enforces in the
// browser.
const addFieldRequired = "required"

// addEntryContentID is the DOM id of the add control in the content area.
//
// A document holds one content area, and one component fills it. That is the
// list table of the Finder detail panel, or the workbench table of a workbench
// page. Both read this name. The shape a reader checks is "one content area per
// document" rather than "two components that happen to agree".
// TestRenderedPageCarriesNoDuplicateDOMID renders both compositions.
//
// The id is a stable hook the .wb suite clicks by name.
const addEntryContentID = "btn-add-entry"

// addEntryButtonID is the DOM id of an "add an entry" control OUTSIDE the
// content area.
//
// An id is unique per document, and two shells draw such a control beside the
// content area. The finder column sits next to the detail panel, and the
// sidebar flyout draws one per list section. The content-area control keeps
// addEntryContentID, so these two name their shell here. Each shell can draw
// one control per list on screen, so the list's own URL follows the shell name.
//
// The sidebar is legacy markup: sidebar (component_sidebar.templ) has no
// caller outside a golden capture, so of the two shells only the finder is
// live today.
func addEntryButtonID(shell, addURL string) string {
	var tb textbuf.Buffer

	return tb.Str("btn-add-entry-").Str(shell).Byte('-').Str(formFieldID(strings.Trim(addURL, "/"))).String()
}

// finderAddItemName is the display name of the synthetic "add an entry" item
// buildListColumn (fragment.go) puts at the head of a list column. The finder
// renders that one item as a button rather than as a link.
const finderAddItemName = "+ new"

// finderItemClass is one finder entry's class list.
func finderItemClass(it ColumnItem) string {
	var tb textbuf.Buffer

	tb.Str("finder-item")

	if it.Selected {
		tb.Str(" selected")
	}

	if it.HasChildren {
		tb.Str(" has-children")
	}

	return tb.String()
}

// sidebarEntryClass is one sidebar flyout entry's class list.
func sidebarEntryClass(selected bool) string {
	if selected {
		return "sidebar-flyout-entry selected"
	}

	return "sidebar-flyout-entry"
}

// relatedToolClass is one ze:related tool button's class list.
func relatedToolClass(class string) string {
	if class == "" {
		return "related-tool"
	}

	var tb textbuf.Buffer

	return tb.Str("related-tool related-tool--").Str(class).String()
}

// relatedToolHxVals is the hx-vals payload naming the tool a button runs and
// the config path it runs against. No command text crosses the wire.
func relatedToolHxVals(t RelatedToolButton) string {
	var tb textbuf.Buffer

	return tb.Str(`{"tool_id":`).Quoted(t.ToolID).
		Str(`,"context_path":`).Quoted(t.ContextPath).Byte('}').String()
}

// toolRerunHxVals is the hx-vals payload of an overlay's rerun button. It
// carries the confirmation the operator already gave.
func toolRerunHxVals(v toolOverlayData) string {
	var tb textbuf.Buffer

	return tb.Str(`{"tool_id":`).Quoted(v.ToolID).
		Str(`,"context_path":`).Quoted(v.ContextPath).
		Str(`,"confirm":"true"}`).String()
}

// toolOverlayClass is one overlay's class list, which carries which of the
// three shapes it is showing.
func toolOverlayClass(state ToolOverlayState) string {
	switch state {
	case ToolOverlayError:
		return "tool-overlay tool-overlay--error"
	case ToolOverlayConfirm:
		return "tool-overlay tool-overlay--confirm"
	case ToolOverlayResult:
		return "tool-overlay tool-overlay--result"
	}

	return "tool-overlay tool-overlay--result"
}

// listRowClass is one list-table row's class list. A pending row holds an edit
// the operator has not committed.
//
// The base class is always written. The markup this replaced emitted class=""
// for a row with no pending change, which names no class at all and leaves the
// row with no styling hook. wbTablePendingClass is the sibling that gets it
// right.
func listRowClass(pending bool) string {
	if pending {
		return "finder-table-row row--pending"
	}

	return "finder-table-row"
}

// listRowDetailURL is the fragment URL a list-table row navigates to. The
// workbench asks for its own shell, so the query carries which one is asking.
func listRowDetailURL(hxPath, activeUI string) string {
	var tb textbuf.Buffer

	tb.Str("/fragment/detail?path=").Str(hxPath)

	if activeUI == uiModeTokenWorkbench {
		tb.Str("&ui=workbench")
	}

	return tb.String()
}

// listRowSwapTarget is the element a list-table row swaps its detail into.
func listRowSwapTarget(activeUI string) string {
	if activeUI == uiModeTokenWorkbench {
		return "#workbench-workspace"
	}

	return "#content-area"
}

// addFormSwapTarget is the element the add-entry overlay swaps its answer into.
func addFormSwapTarget(workbench bool) string {
	if workbench {
		return "#workbench-workspace"
	}

	return "#detail"
}

// addFormSubmitLabel is the add-entry overlay's submit label. The workbench
// calls it a save because its overlay edits a form it already showed.
func addFormSubmitLabel(workbench bool) string {
	if workbench {
		return "Save"
	}

	return "Create"
}

// addButtonLabel is a workbench table's add button label, which falls back to
// the generic verb when the page names none.
func addButtonLabel(label string) string {
	if label == "" {
		return "Add"
	}

	return label
}

// wbTableColClass is one workbench table header's class list.
func wbTableColClass(sortable bool) string {
	if sortable {
		return "wb-table-col wb-table-col--sortable"
	}

	return "wb-table-col"
}

// wbTableRowClass is one log or event row's class list, which carries the
// severity the row reports.
func wbTableRowClass(flagClass string) string {
	if flagClass == "" {
		return "wb-table-row"
	}

	var tb textbuf.Buffer

	return tb.Str("wb-table-row wb-table-row--").Str(flagClass).String()
}

// wbTablePendingClass is one workbench table row's class list. A pending row
// holds an edit the operator has not committed.
func wbTablePendingClass(pending bool) string {
	if pending {
		return "wb-table-row wb-table-row--pending"
	}

	return "wb-table-row"
}

// wbTableEmptyColspan is how many columns the empty-state cell spans. The
// table draws a flag column before the data columns and an action column after
// them, so the span is the data-column count plus two.
//
// HTML gives colspan a minimum of 1, and the markup this replaced wrote 0. A
// browser reads 0 as "span the rest of the column group", which put the empty
// message under the flag column alone.
func wbTableEmptyColspan(v WorkbenchTableData) string {
	const flagAndActionColumns = 2

	return intText(len(v.Columns) + flagAndActionColumns)
}

// wbTableFlagClass is one workbench table flag cell's class list.
func wbTableFlagClass(flagClass string) string {
	if flagClass == "" {
		return "wb-table-flag"
	}

	var tb textbuf.Buffer

	return tb.Str("wb-table-flag wb-table-flag--").Str(flagClass).String()
}

// wbActionClass is one row action's class list.
func wbActionClass(class string) string {
	if class == "" {
		return "wb-action"
	}

	var tb textbuf.Buffer

	return tb.Str("wb-action wb-action--").Str(class).String()
}

// wbFormLabelClass is one workbench form label's class list.
func wbFormLabelClass(required bool) string {
	if required {
		return "wb-form-label wb-form-label--required"
	}

	return "wb-form-label"
}

// wbDetailTabClass is one detail panel tab button's class list.
func wbDetailTabClass(active bool) string {
	if active {
		return "wb-detail-tab wb-detail-tab--active"
	}

	return "wb-detail-tab"
}

// wbDetailContentClass is one detail panel tab body's class list.
func wbDetailContentClass(active bool) string {
	if active {
		return "wb-detail-content wb-detail-content--active"
	}

	return "wb-detail-content"
}

// wbDetailToolClass is one detail panel tool button's class list. Only the
// destructive class changes it, which is what the markup this replaced tested
// for by name.
func wbDetailToolClass(class string) string {
	if class == wbToolDanger {
		return "wb-detail-tool wb-detail-tool--danger"
	}

	return "wb-detail-tool"
}

// wbToolDanger marks a destructive related tool.
const wbToolDanger = "danger"
