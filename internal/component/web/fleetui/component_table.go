// Design: docs/architecture/fleet-interface.md -- view data of the fleet table section
// Related: component_table.templ -- draws the section, the query row, the table and the pager
// Related: table.go -- PageWindow, Paginate and the sort the addresses here carry
// Related: url.go -- ViewParams.Query, which renders every address a table links to

package fleetui

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The table section is the same stack wherever a table appears: section
// title, query row, table actions row, selection row, pager, table, selection
// row, pager. Every address in these views is one the page handler rendered
// with ViewParams.Query, so a sort, a page and an opened row each have an
// address that reproduces the view.

// HiddenFieldView is one view parameter a form carries along unchanged, so
// submitting a filter keeps the sort and the tree view.
type HiddenFieldView struct {
	Name  string
	Value string
}

// QueryRowView is the query row of a table. Action is the page's path, which
// the form asks with the filter as q. Menus are the assisted filter menus:
// each item is a link whose address already holds the filter with its term
// written in (SetFilterTerm). Saved lists the saved filters. Matches is the
// count of rows the filter keeps ("4 of 44 devices"). Refusal is the reason a
// filter was refused, shown under the field.
type QueryRowView struct {
	ID          string
	Action      string
	Text        string
	Placeholder string
	Matches     string
	Refusal     string
	Hidden      []HiddenFieldView
	Menus       []MenuView
	Saved       *MenuView
	Save        *ActionView
}

// BulkView is the selection of a table and what can be done with it. Form is
// the id of the form the row checkboxes and the actions belong to, and Action
// the address it posts to. Filter is the table's filter, posted along so that
// "every match" names the same rows. Total counts the rows of every page.
type BulkView struct {
	Form    string
	Action  string
	Filter  string
	Noun    string
	Actions []ActionView
	More    *MenuView
	Total   int
}

// SortMark says how a column is sorted. The zero value is a column the table
// is not sorted by.
type SortMark uint8

// The sort marks.
const (
	// SortMarkUnspecified is a column the table is not sorted by.
	SortMarkUnspecified SortMark = iota
	// SortMarkAscending is the sort column, smallest first.
	SortMarkAscending
	// SortMarkDescending is the sort column, largest first.
	SortMarkDescending
)

var (
	sortMarkWords = []string{"none", "ascending", "descending"}
	sortMarkIcons = []string{"", "sort-asc", "sort-desc"}
)

// Aria answers the mark as the aria-sort value of its header cell.
func (mark SortMark) Aria() string {
	if int(mark) < len(sortMarkWords) {
		return sortMarkWords[mark]
	}

	return sortMarkWords[SortMarkUnspecified]
}

// Icon answers the icon of the mark, or the empty string for a column the
// table is not sorted by: the header then draws no mark.
func (mark SortMark) Icon() string {
	if int(mark) < len(sortMarkIcons) {
		return sortMarkIcons[mark]
	}

	return sortMarkIcons[SortMarkUnspecified]
}

// ColumnHeaderView is one header cell. Kind is the icon naming the kind of value
// ("kind-text", "kind-state", "kind-address", "kind-number", "kind-time",
// "kind-tag"). SortHref is the address that sorts by the column, the other
// way when it is already the sort column; a column that cannot be sorted has
// none. Numeric right aligns the column.
type ColumnHeaderView struct {
	Label    string
	Kind     string
	SortHref string
	Sort     SortMark
	Numeric  bool
}

// CellKind says what a cell holds. The zero value is not a kind.
type CellKind uint8

// The kinds of cell.
const (
	// CellUnspecified is the invalid zero value. It draws a dash.
	CellUnspecified CellKind = iota
	// CellText is plain text.
	CellText
	// CellObject is an object's icon and its name, linking to the object.
	CellObject
	// CellStatus is a state: its round mark and its word.
	CellStatus
	// CellTags is a list of labels, with a count of those not shown.
	CellTags
	// CellMono is an identifier, an address or a version.
	CellMono
	// CellUsage is a percentage with a thin usage bar.
	CellUsage
	// CellNumber is a figure, right aligned.
	CellNumber
	// CellTime is a relative time, with the time itself as its tooltip.
	CellTime
	// CellLink is a link.
	CellLink
)

// CellView is one cell. Kind decides which fields the cell reads; the
// constructors below fill exactly those. An absent text is drawn as a dash.
type CellView struct {
	Text   string
	Href   string
	Title  string
	Tags   []TagView
	Usage  UsageBarView
	Icon   ObjectIconView
	Status Status
	More   int
	Kind   CellKind
}

// TextCell answers a cell of plain text.
func TextCell(text string) CellView {
	return CellView{Kind: CellText, Text: text}
}

// MonoCell answers a cell holding an identifier, an address or a version.
func MonoCell(text string) CellView {
	return CellView{Kind: CellMono, Text: text}
}

// NumberCell answers a cell holding a figure.
func NumberCell(text string) CellView {
	return CellView{Kind: CellNumber, Text: text}
}

// LinkCell answers a cell holding a link.
func LinkCell(label, href string) CellView {
	return CellView{Kind: CellLink, Text: label, Href: href}
}

// TimeCell answers a cell holding a relative time ("3 minutes ago") with the
// time itself as its tooltip.
func TimeCell(relative, exact string) CellView {
	return CellView{Kind: CellTime, Text: relative, Title: exact}
}

// ObjectCell answers a cell holding an object's icon and its name as a link.
func ObjectCell(icon ObjectIconView, name, href string) CellView {
	return CellView{Kind: CellObject, Icon: icon, Text: name, Href: href}
}

// StatusCell answers a cell holding a state.
func StatusCell(status Status) CellView {
	return CellView{Kind: CellStatus, Status: status}
}

// tagsShownMax is the most labels a cell shows before it counts the rest.
const tagsShownMax = 3

// TagsCell answers a cell holding at most three of the labels and the count
// of the others.
func TagsCell(tags []TagView) CellView {
	shown := min(len(tags), tagsShownMax)

	return CellView{Kind: CellTags, Tags: tags[:shown], More: len(tags) - shown}
}

// UsageCell answers a cell holding a percentage with a thin usage bar. A
// figure the device has not reported is not known, and the cell shows a dash.
func UsageCell(name string, percent int, known bool) CellView {
	return CellView{Kind: CellUsage, Usage: UsageBarView{Name: name, Percent: percent, Known: known}}
}

// RowView is one row. ID is the value its checkbox posts and Name the name
// its controls are labeled with. EyeHref is the address that opens the row
// in the side panel. Opened marks the row whose details the side panel shows.
type RowView struct {
	ID      string
	Name    string
	EyeHref string
	Cells   []CellView
	Menu    *MenuView
	Opened  bool
}

// TableView is one table. Form is the id of the bulk form its checkboxes
// belong to; a table with no selection has none. Controls adds the fixed last
// column of the eye and the three-dot menu. Empty is the state shown under
// the headers when the table has no row.
type TableView struct {
	Name     string
	Form     string
	Headers  []ColumnHeaderView
	Rows     []RowView
	Empty    *StateHeroView
	Controls bool
}

// tableColumns answers how many columns the table draws, for the cell that
// spans them all.
func tableColumns(v TableView) string {
	columns := len(v.Headers)
	if v.Form != "" {
		columns++
	}

	if v.Controls {
		columns++
	}

	return strconv.Itoa(columns)
}

// PagerView is the pagination row. Page holds the figures (Paginate). The
// four addresses open the first, previous, next and last page; the page
// handler renders each. Action and Hidden are the page's path and its other
// view parameters, for the page size form, which asks the path with size.
type PagerView struct {
	Noun      string
	FirstHref string
	PrevHref  string
	NextHref  string
	LastHref  string
	Action    string
	Hidden    []HiddenFieldView
	Page      PageWindow
}

// PagerRange answers the range of rows a page shows: "1 - 24 of 49 objects".
// A table with no row answers "0 objects".
func PagerRange(page PageWindow, noun string) string {
	text := textbuf.New()
	if page.Total > 0 {
		text.Int(int64(page.First)).Str(" - ").Int(int64(page.Last)).Str(" of ")
	}

	return text.Int(int64(page.Total)).Byte(' ').Str(noun).String()
}

// PagerHasPrevious says whether a page before this one exists, so the first
// and previous controls lead somewhere.
func PagerHasPrevious(page PageWindow) bool {
	return page.Number > 1
}

// PagerHasNext says whether a page after this one exists, so the next and
// last controls lead somewhere.
func PagerHasNext(page PageWindow) bool {
	return page.Number < page.Pages
}

// pagerStepView is one of the four page controls.
type pagerStepView struct {
	Href    string
	Icon    string
	Name    string
	Enabled bool
}

// pagerSteps answers the first, previous, next and last controls of a page.
func pagerSteps(v PagerView) []pagerStepView {
	previous, next := PagerHasPrevious(v.Page), PagerHasNext(v.Page)

	return []pagerStepView{
		{Href: v.FirstHref, Icon: "first", Name: "First page", Enabled: previous},
		{Href: v.PrevHref, Icon: "prev", Name: "Previous page", Enabled: previous},
		{Href: v.NextHref, Icon: "next", Name: "Next page", Enabled: next},
		{Href: v.LastHref, Icon: "last", Name: "Last page", Enabled: next},
	}
}

// SelectionText answers the words after the count of selected rows:
// "selected of 44 objects".
func SelectionText(total int, noun string) string {
	return textbuf.New().Str("selected of ").Int(int64(total)).Byte(' ').Str(noun).String()
}

// TableSectionView is one section of a tab's content. Create is its creating
// action, or CreateMenu when there are several. Query, Bulk and Pager are
// each absent from a table that does not filter, select or page.
type TableSectionView struct {
	Title      string
	Create     *ActionView
	CreateMenu *MenuView
	Query      *QueryRowView
	Bulk       *BulkView
	Pager      *PagerView
	Table      TableView
}
