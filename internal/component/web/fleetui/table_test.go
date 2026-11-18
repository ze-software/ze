package fleetui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sortedNames sorts the test estate by the sort parameter of an address and
// answers the names in order.
func sortedNames(t *testing.T, sort string) []string {
	t.Helper()

	sorted, err := DeviceColumns().Sort(testViews(), ParseSort(sort))
	require.NoError(t, err, sort)

	return namesOf(sorted)
}

// TestDeviceSortKeys sorts the test estate by each column of the devices
// table.
//
// VALIDATES: the default sort is the name ascending, the state column sorts
// by severity, the version column by version, the figures by value with a
// device that reported none first, and a leading "-" reverses the order.
// PREVENTS: a state column sorted by its word, and a device with no sample
// sorted as if it used no processor.
func TestDeviceSortKeys(t *testing.T) {
	cases := []struct {
		sort string
		want []string
	}{
		{"", []string{coreAms01, coreLon01, cpe0113, cpe0200, edgeLon01, edgeLon02, rsAms01}},
		{"name", []string{coreAms01, coreLon01, cpe0113, cpe0200, edgeLon01, edgeLon02, rsAms01}},
		{"-name", []string{rsAms01, edgeLon02, edgeLon01, cpe0200, cpe0113, coreLon01, coreAms01}},
		{"-state", []string{coreLon01, coreAms01, rsAms01, cpe0113, cpe0200, edgeLon01, edgeLon02}},
		{"cpu", []string{coreAms01, cpe0200, rsAms01, edgeLon02, cpe0113, edgeLon01, coreLon01}},
		{"-memory", []string{edgeLon02, coreLon01, edgeLon01, cpe0113, rsAms01, coreAms01, cpe0200}},
		{"-alarms", []string{coreLon01, coreAms01, edgeLon01, edgeLon02, rsAms01, cpe0113, cpe0200}},
		{"-version", []string{edgeLon01, edgeLon02, cpe0113, coreLon01, coreAms01, rsAms01, cpe0200}},
		{"-last-seen", []string{coreAms01, edgeLon01, edgeLon02, coreLon01, rsAms01, cpe0113, cpe0200}},
		{"-uptime", []string{edgeLon01, edgeLon02, coreLon01, rsAms01, cpe0113, coreAms01, cpe0200}},
		{"config", []string{edgeLon01, coreLon01, coreAms01, edgeLon02, cpe0113, rsAms01, cpe0200}},
	}

	for index := range cases {
		tc := &cases[index]
		t.Run("sort="+tc.sort, func(t *testing.T) {
			assert.Equal(t, tc.want, sortedNames(t, tc.sort))
		})
	}
}

// TestSortIsStable sorts by a column that does not tell two devices apart.
//
// VALIDATES: rows the column holds equal keep the order they came in, in
// both directions.
// PREVENTS: rows that change place on every reload of the same view.
func TestSortIsStable(t *testing.T) {
	ascending := sortedNames(t, "state")
	assert.Equal(t, []string{edgeLon01, edgeLon02, cpe0200, cpe0113, rsAms01, coreAms01, coreLon01}, ascending)

	descending := sortedNames(t, "-state")
	assert.Equal(t, []string{edgeLon01, edgeLon02}, descending[5:], "the two healthy devices keep their order")
}

// TestSortLeavesItsInputAlone sorts a slice and reads the original.
//
// VALIDATES: Sort answers a new slice and does not reorder the one given.
// PREVENTS: a sorted table reordering the views the tree and the cards of
// the same request still read.
func TestSortLeavesItsInputAlone(t *testing.T) {
	views := testViews()
	before := namesOf(views)

	_, err := DeviceColumns().Sort(views, ParseSort("-cpu"))
	require.NoError(t, err)

	assert.Equal(t, before, namesOf(views))
}

// TestSortRefusesUnknownColumn sorts by a column the devices table does not
// have.
//
// VALIDATES: the refusal is an operator error that names the column and
// lists the sortable ones.
// PREVENTS: an unknown sort key silently sorted by the default column.
func TestSortRefusesUnknownColumn(t *testing.T) {
	_, err := DeviceColumns().Sort(testViews(), ParseSort("-serial"))

	refusal := requireRefusal(t, err, ErrSortRefused)
	assert.Contains(t, refusal.Problem(), `"serial"`)
	assert.Contains(t, refusal.Fix(), "name, state, version, config, uptime, last-seen, cpu, memory, alarms")
}

// TestSortSpecRoundTrip parses and renders the sort parameter.
//
// VALIDATES: a column, a descending column and the default sort each render
// back to the text they were parsed from.
// PREVENTS: a sort link that reverses the direction it shows.
func TestSortSpecRoundTrip(t *testing.T) {
	assert.Equal(t, SortSpec{Column: "cpu"}, ParseSort("cpu"))
	assert.Equal(t, SortSpec{Column: "cpu", Descending: true}, ParseSort("-cpu"))
	assert.Equal(t, SortSpec{}, ParseSort(""))

	for _, text := range []string{"", "cpu", "-cpu", "last-seen", "-last-seen"} {
		assert.Equal(t, text, ParseSort(text).String())
	}
}

// TestPaginateFigures pages tables of several sizes.
//
// VALIDATES: the figures "first to last of total" and the page count for a
// first page, a last partial page, an exact fit and an empty table.
// PREVENTS: a pagination row that names rows the page does not show.
func TestPaginateFigures(t *testing.T) {
	cases := []struct {
		name                string
		total, number, size int
		want                Page
	}{
		{"first page", 49, 1, 25, Page{Number: 1, Pages: 2, Size: 25, Total: 49, First: 1, Last: 25}},
		{"last partial page", 49, 2, 25, Page{Number: 2, Pages: 2, Size: 25, Total: 49, First: 26, Last: 49}},
		{"exact fit", 100, 1, 100, Page{Number: 1, Pages: 1, Size: 100, Total: 100, First: 1, Last: 100}},
		{"one row over", 101, 2, 100, Page{Number: 2, Pages: 2, Size: 100, Total: 101, First: 101, Last: 101}},
		{"fewer rows than a page", 4, 1, 50, Page{Number: 1, Pages: 1, Size: 50, Total: 4, First: 1, Last: 4}},
		{"no row", 0, 1, 50, Page{Number: 1, Pages: 1, Size: 50, Total: 0, First: 0, Last: 0}},
	}

	for index := range cases {
		tc := &cases[index]
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Paginate(tc.total, tc.number, tc.size))
		})
	}
}

// TestPaginatePageNumberBoundary asks for pages before the first and beyond
// the last.
//
// VALIDATES: page 0 and a negative page show page 1, the last page is kept,
// and a page beyond the last shows the last page.
// PREVENTS: an address with a stale page number answered with an empty table.
func TestPaginatePageNumberBoundary(t *testing.T) {
	assert.Equal(t, 1, Paginate(120, 0, 50).Number)
	assert.Equal(t, 1, Paginate(120, -4, 50).Number)
	assert.Equal(t, 3, Paginate(120, 3, 50).Number)
	assert.Equal(t, 3, Paginate(120, 4, 50).Number)
	assert.Equal(t, 3, Paginate(120, 999, 50).Number)
	assert.Equal(t, 1, Paginate(0, 7, 50).Number)
}

// TestPaginatePageSizeBoundary asks for page sizes the table offers and does
// not offer.
//
// VALIDATES: 25, 50 and 100 are kept and any other size is 50.
// PREVENTS: a page of a size no address parameter can express.
func TestPaginatePageSizeBoundary(t *testing.T) {
	assert.Equal(t, []int{25, 50, 100}, PageSizes())

	for size, want := range map[int]int{25: 25, 50: 50, 100: 100, 0: 50, 24: 50, 26: 50, 99: 50, 101: 50, -25: 50, 1000: 50} {
		assert.Equal(t, want, Paginate(500, 1, size).Size, "size %d", size)
	}
}

// TestPageRows cuts the rows of one page out of a table.
//
// VALIDATES: a page holds exactly the rows its figures name, and an empty
// table has an empty page.
// PREVENTS: a page that shows rows of its neighbor.
func TestPageRows(t *testing.T) {
	rows := make([]int, 0, 60)
	for index := range 60 {
		rows = append(rows, index+1)
	}

	first := PageRows(rows, Paginate(len(rows), 1, 25))
	assert.Len(t, first, 25)
	assert.Equal(t, 1, first[0])
	assert.Equal(t, 25, first[24])

	last := PageRows(rows, Paginate(len(rows), 3, 25))
	assert.Equal(t, []int{51, 52, 53, 54, 55, 56, 57, 58, 59, 60}, last)

	assert.Empty(t, PageRows([]int{}, Paginate(0, 1, 25)))
}

// TestQueryDevices asks for one page of the devices table: filtered, sorted
// and paged.
//
// VALIDATES: the answer holds the rows that match the filter, in the sort
// order, with the figures of the pagination row counting the matches.
// PREVENTS: a count of matches that counts the whole estate.
func TestQueryDevices(t *testing.T) {
	answer, err := QueryDevices(testViews(), DeviceQuery{
		Filter: "site:lon", Sort: ParseSort("-cpu"), Page: 1, Size: 25,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{coreLon01, edgeLon01, edgeLon02}, namesOf(answer.Rows))
	assert.Equal(t, Page{Number: 1, Pages: 1, Size: 25, Total: 3, First: 1, Last: 3}, answer.Page)
}

// TestQueryDevicesPagesTheMatches asks for the second page of a query over a
// larger estate.
//
// VALIDATES: the second page starts after the first, and a page beyond the
// last is the last page.
// PREVENTS: a fragment that carries more rows than it shows.
func TestQueryDevicesPagesTheMatches(t *testing.T) {
	devices := make([]Device, 0, 60)
	for index := range 60 {
		name := "edge-" + string(rune('a'+index/26)) + string(rune('a'+index%26))
		devices = append(devices, testConnected(name, "1.4.2", 10, 10, "site=lon"))
	}
	views := DeriveDevices(devices, nil, nil, testNow)

	second, err := QueryDevices(views, DeviceQuery{Page: 2, Size: 25})
	require.NoError(t, err)
	assert.Len(t, second.Rows, 25)
	assert.Equal(t, "edge-az", second.Rows[0].Device.Name)
	assert.Equal(t, Page{Number: 2, Pages: 3, Size: 25, Total: 60, First: 26, Last: 50}, second.Page)

	beyond, err := QueryDevices(views, DeviceQuery{Page: 9, Size: 25})
	require.NoError(t, err)
	assert.Len(t, beyond.Rows, 10)
	assert.Equal(t, 3, beyond.Page.Number)
}

// TestQueryDevicesRefusals asks for a page with a filter and with a sort the
// table does not accept.
//
// VALIDATES: each is refused with its own cause, so the handler keeps the
// previous rows and shows the message.
// PREVENTS: a refused filter answered as an empty page.
func TestQueryDevicesRefusals(t *testing.T) {
	_, err := QueryDevices(testViews(), DeviceQuery{Filter: "stat:offline", Page: 1, Size: 50})
	requireRefusal(t, err, ErrFilterRefused)

	_, err = QueryDevices(testViews(), DeviceQuery{Sort: ParseSort("serial"), Page: 1, Size: 50})
	requireRefusal(t, err, ErrSortRefused)
}
