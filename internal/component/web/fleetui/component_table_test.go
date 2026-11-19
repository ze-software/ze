package fleetui

import (
	"strings"
	"testing"
	"time"
)

// TestPagerFigures checks the pagination row at the first, a middle and the
// last page, and for a table with no row.
//
// VALIDATES: the range text reads "first - last of total noun", a table
// with no row reads "0 noun", and the first and previous controls lead
// somewhere only after the first page, the next and last only before the
// last.
// PREVENTS: a control that links past either end, and "1 - 0 of 0".
func TestPagerFigures(t *testing.T) {
	cases := []struct {
		name     string
		page     PageWindow
		text     string
		previous bool
		next     bool
	}{
		{"first", Paginate(120, 1, 50), "1 - 50 of 120 objects", false, true},
		{"middle", Paginate(120, 2, 50), "51 - 100 of 120 objects", true, true},
		{"last", Paginate(120, 3, 50), "101 - 120 of 120 objects", true, false},
		{"only", Paginate(4, 1, 50), "1 - 4 of 4 objects", false, false},
		{"empty", Paginate(0, 1, 50), "0 objects", false, false},
	}

	for _, test := range cases {
		if got := PagerRange(test.page, "objects"); got != test.text {
			t.Errorf("%s: range %q, want %q", test.name, got, test.text)
		}

		if got := PagerHasPrevious(test.page); got != test.previous {
			t.Errorf("%s: previous page %v, want %v", test.name, got, test.previous)
		}

		if got := PagerHasNext(test.page); got != test.next {
			t.Errorf("%s: next page %v, want %v", test.name, got, test.next)
		}

		steps := pagerSteps(PagerView{Page: test.page})
		want := []bool{test.previous, test.previous, test.next, test.next}
		for index, step := range steps {
			if step.Enabled != want[index] {
				t.Errorf("%s: control %q enabled %v, want %v", test.name, step.Name, step.Enabled, want[index])
			}
		}
	}
}

// TestPagerMarkup renders the pagination row of a middle page and of an
// empty table.
//
// VALIDATES: an available control is a link carrying its address and its
// name; an unavailable one is no link; the select offers every page size
// with the current one selected.
// PREVENTS: a disabled control that still navigates.
func TestPagerMarkup(t *testing.T) {
	middle := componentMarkup(t, Pager(PagerView{
		Page: Paginate(120, 2, 50), Noun: "devices", Action: "/fleet/estate/devices/",
		FirstHref: "/fleet/estate/devices/", PrevHref: "/fleet/estate/devices/?page=1",
		NextHref: "/fleet/estate/devices/?page=3", LastHref: "/fleet/estate/devices/?page=3",
	}))

	for _, want := range []string{
		`aria-label="Previous page"`, `aria-label="Next page"`, `href="/fleet/estate/devices/?page=3"`,
		`hx-push-url="true"`, `<option value="50" selected`, `<option value="25"`, `<option value="100"`,
	} {
		if !strings.Contains(middle, want) {
			t.Errorf("the pager of a middle page holds no %s", want)
		}
	}

	empty := componentMarkup(t, Pager(PagerView{Page: Paginate(0, 1, 50), Noun: "devices", Action: "/fleet/estate/devices/"}))
	if strings.Contains(empty, `aria-label="Next page"`) {
		t.Error("the pager of an empty table links to a next page")
	}

	if got := strings.Count(empty, `aria-disabled="true"`); got != 4 {
		t.Errorf("the pager of an empty table has %d unavailable controls, want 4", got)
	}
}

// TestTableMarkup renders a table with selection and row controls.
//
// VALIDATES: header cells carry scope and the sort direction, a sortable
// header is a link, the first cell of a row is its header, a row's checkbox
// belongs to the bulk form and is named, an absent value is a dash, and the
// opened row is marked.
// PREVENTS: a table assistive technology cannot read, and a checkbox that
// posts nothing.
func TestTableMarkup(t *testing.T) {
	markup := componentMarkup(t, Table(TableView{
		Name: "Devices", Form: "bulk", Controls: true,
		Headers: []ColumnHeaderView{
			{Label: "Device", Kind: "kind-text", SortHref: "/fleet/estate/devices/?sort=-name", Sort: SortMarkAscending},
			{Label: "Address", Kind: "kind-address"},
		},
		Rows: []RowView{{
			ID: "edge-lon-04", Name: "edge-lon-04", Opened: true, EyeHref: "/fleet/estate/devices/?id=edge-lon-04",
			Cells: []CellView{LinkCell("edge-lon-04", "/fleet/device/edge-lon-04/"), MonoCell("")},
		}},
	}))

	for _, want := range []string{
		`<th scope="col"`, `aria-sort="ascending"`, `aria-sort="none"`, `href="/fleet/estate/devices/?sort=-name"`,
		`<th scope="row"`, `name="row" value="edge-lon-04" form="bulk"`, `aria-label="Select edge-lon-04"`,
		`fleet-table-row--opened`, `aria-label="Show the details of edge-lon-04"`, `class="fleet-absent"`,
		`data-fleet-select-page="true"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the table holds no %s", want)
		}
	}
}

// TestTagsCellCountsTheRest checks the labels a cell shows.
//
// VALIDATES: a cell shows at most three labels and counts the others.
// PREVENTS: a row as tall as its longest label list.
func TestTagsCellCountsTheRest(t *testing.T) {
	five := TagsCell(galleryTags())
	if len(five.Tags) != 3 || five.More != 2 {
		t.Errorf("five labels show %d and count %d more, want 3 and 2", len(five.Tags), five.More)
	}

	two := TagsCell(galleryTags()[:2])
	if len(two.Tags) != 2 || two.More != 0 {
		t.Errorf("two labels show %d and count %d more, want 2 and 0", len(two.Tags), two.More)
	}
}

// TestRelativeTime checks the wording of an age.
//
// VALIDATES: an age is written in its largest whole unit, singular for one,
// "now" below five seconds and for an age below zero.
// PREVENTS: "1 minutes ago", "60 seconds ago" and a negative age.
func TestRelativeTime(t *testing.T) {
	cases := []struct {
		age  time.Duration
		want string
	}{
		{-time.Minute, "now"},
		{0, "now"},
		{4 * time.Second, "now"},
		{5 * time.Second, "5 seconds ago"},
		{59 * time.Second, "59 seconds ago"},
		{time.Minute, "1 minute ago"},
		{59*time.Minute + 59*time.Second, "59 minutes ago"},
		{time.Hour, "1 hour ago"},
		{23 * time.Hour, "23 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{72 * time.Hour, "3 days ago"},
	}

	for _, test := range cases {
		if got := RelativeTime(test.age); got != test.want {
			t.Errorf("%s: %q, want %q", test.age, got, test.want)
		}
	}
}

// TestPercent checks the share a bar is drawn at.
//
// VALIDATES: a share is rounded to the nearest whole percent inside 0 to
// 100, and the share of nothing is not known.
// PREVENTS: a bar wider than its track, and 0% shown for a value that was
// never reported.
func TestPercent(t *testing.T) {
	cases := []struct {
		part, whole float64
		want        int
		known       bool
	}{
		{0, 8, 0, true}, {4, 8, 50, true}, {7.97, 8, 100, true}, {9, 8, 100, true},
		{-1, 8, 0, true}, {1, 3, 33, true}, {2, 3, 67, true}, {1, 0, 0, false}, {1, -2, 0, false},
	}

	for _, test := range cases {
		got, known := Percent(test.part, test.whole)
		if got != test.want || known != test.known {
			t.Errorf("%v of %v: %d, %v, want %d, %v", test.part, test.whole, got, known, test.want, test.known)
		}
	}
}

// TestConfirmAttributes renders actions with and without a confirmation.
//
// VALIDATES: a destructive action carries the data-confirm attributes with
// the text the operator types, and a table action is drawn disabled until a
// row is selected.
// PREVENTS: a destructive action that submits at one click.
func TestConfirmAttributes(t *testing.T) {
	destructive := componentMarkup(t, ActionButton(ActionView{
		Label: "Delete", Form: "bulk", Name: "action", Value: "delete", Bulk: true,
		Confirm: &ConfirmView{Action: "Delete 2 devices", Targets: "2 devices", Expect: "2", Destructive: true},
	}))

	for _, want := range []string{
		`type="submit"`, `form="bulk"`, `data-confirm-action="Delete 2 devices"`, `data-confirm-expect="2"`,
		`data-confirm-kind="destructive"`, `data-fleet-bulk-action="true"`, ` disabled`, `fleet-action--destructive`,
	} {
		if !strings.Contains(destructive, want) {
			t.Errorf("the destructive action holds no %s", want)
		}
	}

	plain := componentMarkup(t, ActionButton(ActionView{Label: "Open", Href: "/fleet/"}))
	if strings.Contains(plain, "data-confirm") {
		t.Error("an action with no confirmation carries a data-confirm attribute")
	}

	if !strings.Contains(plain, `href="/fleet/"`) {
		t.Error("an action with an address is not a link to it")
	}
}
