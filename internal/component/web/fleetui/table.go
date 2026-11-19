// Design: docs/architecture/fleet-interface.md -- sorting and paging a table, and the devices query
// Related: filter.go -- the filter a table applies before it sorts
// Related: url.go -- the view parameters these values travel in
// Related: register_devices.go -- the columns of the devices table
//
// A table shows one page of the rows that match its filter, in the order of
// one column. The three steps are pure functions over the rows a handler
// holds. QueryDevices runs them in order for the devices table, and it is the
// one function a handler calls: a hub that filters, sorts and pages on its
// side for scale replaces that function and nothing else.

package fleetui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// PageSizeDefault is the page size of an address that names none, or one
// that is not offered.
const PageSizeDefault = 50

// bugSortColumn is what a refused registration of this file names.
const bugSortColumn = "the sort column"

// sortDescendingMark opens the sort parameter of a descending sort.
const sortDescendingMark = "-"

// pageSizes holds the page sizes a table offers.
var pageSizes = []int{25, 50, 100}

// PageSizes answers the page sizes a table offers, the smallest first.
func PageSizes() []int {
	return slices.Clone(pageSizes)
}

// validPageSize reports whether size is one a table offers.
func validPageSize(size int) bool {
	return slices.Contains(pageSizes, size)
}

// parsePageSize reads the page size of an address. Anything but an offered
// size is the default size.
func parsePageSize(text string) int {
	size, err := strconv.Atoi(text)
	if err != nil {
		return PageSizeDefault
	}
	if validPageSize(size) {
		return size
	}

	return PageSizeDefault
}

// parsePageNumber reads the page number of an address. Anything but a number
// of one or more is the first page.
func parsePageNumber(text string) int {
	number, err := strconv.Atoi(text)
	if err != nil {
		return 1
	}

	return max(number, 1)
}

// SortSpec names the column a table is sorted by and the direction. The zero
// value is the table's default sort.
type SortSpec struct {
	// Column is the key of the column. It is empty for the default column.
	Column     string
	Descending bool
}

// ParseSort reads the sort parameter of an address: a column key, with a
// leading "-" for a descending sort. It does not judge the key: the column
// set of the table does, when it sorts.
func ParseSort(text string) SortSpec {
	column, descending := strings.CutPrefix(text, sortDescendingMark)

	return SortSpec{Column: column, Descending: descending}
}

// String answers the sort parameter of an address. The default sort is the
// empty text.
func (spec SortSpec) String() string {
	if spec.Descending {
		return sortDescendingMark + spec.Column
	}

	return spec.Column
}

// ErrSortRefused means an address names a sort column its table does not
// have.
var ErrSortRefused = errors.New("the sort column is not one of the table's")

// Column is one sortable column of a table over rows of type T.
type Column[T any] struct {
	// Key is the column's name in an address.
	Key string
	// Compare orders two rows by the column: negative when a sorts first.
	Compare func(a, b *T) int
}

// ColumnSet is the sortable columns of one table, a registered set: the
// table's own register file adds the columns, and a later feature adds one
// without an edit to the table. The first column registered is the default
// sort. Build one with NewColumnSet. Safe for concurrent use.
type ColumnSet[T any] struct {
	mu      sync.RWMutex
	subject string
	columns []Column[T]
}

// NewColumnSet builds the empty column set of a table. subject names the
// table's rows in a refusal, such as "devices".
func NewColumnSet[T any](subject string) *ColumnSet[T] {
	return &ColumnSet[T]{subject: subject}
}

// MustRegister adds a sortable column. Call it from init() in a register
// file. It panics on a column with no key or no comparison and on a key the
// set already holds: each is a defect of the build's composition.
func (set *ColumnSet[T]) MustRegister(column Column[T]) {
	if column.Key == "" {
		registrationBug(bugSortColumn, column.Key, set.subject, bugUnnamed)
	}
	if column.Compare == nil {
		registrationBug(bugSortColumn, column.Key, set.subject, "has no comparison")
	}

	set.mu.Lock()
	defer set.mu.Unlock()

	if slices.ContainsFunc(set.columns, func(held Column[T]) bool { return held.Key == column.Key }) {
		registrationBug(bugSortColumn, column.Key, set.subject, bugTwice)
	}
	set.columns = append(set.columns, column)
}

// Keys answers the keys of the sortable columns, in registration order.
func (set *ColumnSet[T]) Keys() []string {
	set.mu.RLock()
	defer set.mu.RUnlock()

	keys := make([]string, 0, len(set.columns))
	for _, column := range set.columns {
		keys = append(keys, column.Key)
	}

	return keys
}

// column answers the column spec names: the first one for the default sort.
func (set *ColumnSet[T]) column(spec SortSpec) (Column[T], error) {
	set.mu.RLock()
	defer set.mu.RUnlock()

	if len(set.columns) == 0 {
		panic("BUG: fleetui: the table of " + set.subject + " was sorted before any column registered")
	}
	if spec.Column == "" {
		return set.columns[0], nil
	}

	for _, column := range set.columns {
		if column.Key == spec.Column {
			return column, nil
		}
	}

	keys := make([]string, 0, len(set.columns))
	for _, column := range set.columns {
		keys = append(keys, column.Key)
	}
	problem := textbuf.New().Str("The table of ").Str(set.subject).Str(" has no sort column named ").
		Quoted(spec.Column).Str(".").Slice()
	fix := textbuf.New().Str("sort by one of: ").Join(keys, ", ").Slice()

	return Column[T]{}, opererr.Wrap(ErrSortRefused, problem,
		"The rows were not sorted again, so the table still shows its previous order.", fix)
}

// Sort answers a new slice holding rows in the order spec names. The sort is
// stable: rows the column does not tell apart keep the order they came in,
// in either direction. It refuses a column the set does not hold with an
// error wrapping ErrSortRefused that lists the columns.
func (set *ColumnSet[T]) Sort(rows []T, spec SortSpec) ([]T, error) {
	column, err := set.column(spec)
	if err != nil {
		return nil, err
	}

	// The sort moves pointers, never the rows, which can be large.
	order := make([]*T, len(rows))
	for index := range rows {
		order[index] = &rows[index]
	}
	slices.SortStableFunc(order, func(a, b *T) int {
		if spec.Descending {
			return column.Compare(b, a)
		}

		return column.Compare(a, b)
	})

	sorted := make([]T, len(rows))
	for index, row := range order {
		sorted[index] = *row
	}

	return sorted, nil
}

// PageWindow is one page of a table and the figures its pagination row shows:
// "First to Last of Total".
type PageWindow struct {
	// Number is the page shown, from one, and Pages how many there are. A
	// table with no row has one empty page.
	Number int
	Pages  int
	// Size is the page size.
	Size int
	// Total counts the rows of every page.
	Total int
	// First and Last are the positions, from one, of the first and last row
	// shown. Both are zero for a table with no row.
	First int
	Last  int
}

// Paginate answers the page of a table of total rows. A size that is not
// offered is the default size, a number below one is the first page, and a
// number beyond the last page is the last page.
func Paginate(total, number, size int) PageWindow {
	if !validPageSize(size) {
		size = PageSizeDefault
	}
	total = max(total, 0)

	pages := max((total+size-1)/size, 1)
	number = min(max(number, 1), pages)

	page := PageWindow{Number: number, Pages: pages, Size: size, Total: total}
	if total > 0 {
		page.First = (number-1)*size + 1
		page.Last = min(number*size, total)
	}

	return page
}

// PageRows answers the rows of rows that page shows. page MUST come from
// Paginate over len(rows).
func PageRows[T any](rows []T, page PageWindow) []T {
	if page.Total != len(rows) {
		panic("BUG: fleetui.PageRows: the page was not computed over these rows")
	}
	if page.Total == 0 {
		return rows[:0]
	}

	return rows[page.First-1 : page.Last]
}

// DeviceQuery is what the devices table asks for: the rows that match a
// filter, in an order, one page of them.
type DeviceQuery struct {
	Filter string
	Sort   SortSpec
	// Page and Size are the page number and size of the address, as
	// ParseViewParams answers them.
	Page int
	Size int
}

// DeviceAnswer is one page of the devices table.
type DeviceAnswer struct {
	// Rows holds the devices of the page.
	Rows []DeviceView
	// Page holds the figures of the pagination row. Its Total is the number
	// of devices that match the filter.
	Page PageWindow
}

// QueryDevices answers one page of the devices table over views, which are
// the devices of the object the table belongs to: the estate's or a group
// node's. It filters, then sorts, then pages.
//
// It refuses a filter the grammar does not accept with an error wrapping
// ErrFilterRefused, and a sort column the table does not have with one
// wrapping ErrSortRefused. The table then keeps its previous rows.
//
// This is the one function a handler calls for the devices table, so a hub
// that does the three steps on its side replaces it alone.
func QueryDevices(views []DeviceView, query DeviceQuery) (DeviceAnswer, error) {
	filter, err := DeviceFilters().Parse(query.Filter, LabelKeys(views))
	if err != nil {
		return DeviceAnswer{}, err
	}

	sorted, err := DeviceColumns().Sort(filter.Apply(views), query.Sort)
	if err != nil {
		return DeviceAnswer{}, err
	}

	page := Paginate(len(sorted), query.Page, query.Size)

	return DeviceAnswer{Rows: PageRows(sorted, page), Page: page}, nil
}
