// Design: docs/architecture/fleet-interface.md -- the addresses of the fleet interface
// Related: tabs.go -- the registered tabs an address may name
// Related: table.go -- the page number and size an address carries
// Related: handler_shell.go -- fleetPath, the prefix of every address
//
// Every view of the fleet interface has an address that reproduces it. The
// path names an object and its tab; the query carries the view: the filter,
// the sort, the page, the row in the side panel, the chart range and the tree
// view. This file parses an address into typed values and renders them back,
// so a handler never reads a path segment and a template never builds a link
// from text.
//
//	/fleet/                                  the estate dashboard
//	/fleet/estate/<tab>/                     an estate tab
//	/fleet/group/<key>=<value>/.../<tab>/    a group node tab
//	/fleet/device/<name>/<tab>/              a device tab
//	/fleet/tasks/                            every task
//	/fleet/admin/<page>/                     an administration page
//	/fleet/fragment/<name>/<page path>       a fragment of one of the above
//
// A group segment is one tree level: its key, then "=", then its value, each
// escaped once. A segment with no "=" is the node of the devices that lack
// the key.

package fleetui

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The keys of the objects in an address that more than one place names.
const (
	objectKeyEstate = "estate"
	objectKeyGroup  = "group"
	objectKeyDevice = "device"
	objectKeyTasks  = "tasks"
)

// The fixed parts of the fleet addresses under fleetPath.
const (
	// EventsPath is the address of the live event stream.
	EventsPath = fleetPath + "events"
	// fragmentSegment is the first path segment of every fragment address.
	fragmentSegment = "fragment"
	// actionSegment is the first path segment of every prototype action
	// address.
	actionSegment = "action"
	// groupSeparator separates the key of a group segment from its value.
	groupSeparator = "="
)

// The names of the view parameters in a query.
const (
	paramFilter   = "q"
	paramSort     = "sort"
	paramPage     = "page"
	paramSize     = "size"
	paramRow      = "id"
	paramRange    = "range"
	paramTreeView = "view"
	paramItem     = "item"
	paramSub      = "sub"
)

// ErrAddressUnknown means an address names no page of the fleet interface. A
// handler answers 404 for it.
var ErrAddressUnknown = errors.New("no such fleet address")

// ObjectKind is the kind of thing a fleet page shows. The zero value is not a
// kind.
type ObjectKind uint8

const (
	// ObjectUnspecified is the invalid zero value.
	ObjectUnspecified ObjectKind = iota
	// ObjectEstate is every device the hub manages.
	ObjectEstate
	// ObjectGroup is the devices of one branch of the tree.
	ObjectGroup
	// ObjectDevice is one device.
	ObjectDevice
	// ObjectTasks is the list of every task. It has no tab.
	ObjectTasks
	// ObjectAdmin is the administration. Its tabs are its pages.
	ObjectAdmin
)

var objectKindSegments = []string{enumKeyUnspecified, objectKeyEstate, objectKeyGroup, objectKeyDevice, objectKeyTasks, "admin"}

// String answers the kind's first path segment.
func (kind ObjectKind) String() string {
	return enumText(objectKindSegments, kind)
}

// tabbed reports whether an address of this kind names a tab.
func (kind ObjectKind) tabbed() bool {
	switch kind {
	case ObjectEstate, ObjectGroup, ObjectDevice, ObjectAdmin:
		return true
	case ObjectUnspecified, ObjectTasks:
		return false
	}

	return false
}

// GroupLevel is one level of a group node's path: a tree level key and the
// value the node's devices share.
type GroupLevel struct {
	Key string
	// Value is the shared value. It means nothing while Missing is true.
	Value string
	// Missing reports that the node holds the devices that lack the key.
	Missing bool
}

// Address is one page of the fleet interface: an object, its tab and the
// view. ParseAddress builds a valid one, and a page builds the address of a
// link by changing a copy of its own.
type Address struct {
	Kind ObjectKind
	// Group is the path of a group node, one level per tree level. It is
	// set only for ObjectGroup.
	Group []GroupLevel
	// Device is the name of the device. It is set only for ObjectDevice.
	Device string
	// Tab is the tab, or the administration page. ObjectTasks has none.
	Tab  string
	View ViewParams
}

// ViewParams is the part of a view an address carries in its query. The zero
// value is the default view: no filter, the default sort, the first page of
// the default size, no row, the default chart range and the default tree
// view.
type ViewParams struct {
	// Filter is the filter text, as typed. ParseAddress does not judge it:
	// the filter parser of the table does.
	Filter string
	Sort   SortSpec
	// Page is the page number, from one. ParseViewParams never answers less
	// than one; Paginate clamps it to the last page.
	Page int
	// Size is the page size, one of PageSizes.
	Size int
	// Row names the row shown in the side panel. It is empty when the panel
	// shows none.
	Row   string
	Range ChartRange
	// TreeView names the tree view. ParseViewParams answers TreeViewDefault
	// for an address that names none.
	TreeView string
	// Sub names a part of the tab: a sub-tab, or a view the tab opens in
	// place of its default one. ParseViewParams does not judge it: the page
	// does, and an empty Sub is the tab's default.
	Sub string
}

// ChartRange is the period the charts of a dashboard show. The zero value is
// not a range.
type ChartRange uint8

const (
	// ChartRangeUnspecified is the invalid zero value.
	ChartRangeUnspecified ChartRange = iota
	// ChartRangeHour is the last hour.
	ChartRangeHour
	// ChartRangeDay is the last day. It is the default.
	ChartRangeDay
	// ChartRangeWeek is the last week.
	ChartRangeWeek
)

// ChartRangeDefault is the range of an address that names none, or one this
// interface does not know.
const ChartRangeDefault = ChartRangeDay

var (
	chartRangeKeys      = []string{enumKeyUnspecified, "hour", "day", "week"}
	chartRangeWords     = []string{enumWordUnspecified, "Last hour", "Last day", "Last week"}
	chartRangeDurations = []time.Duration{0, time.Hour, 24 * time.Hour, 7 * 24 * time.Hour}
)

// String answers the words a chart shows under its title.
func (chartRange ChartRange) String() string {
	return enumText(chartRangeWords, chartRange)
}

// Key answers the range's name in an address.
func (chartRange ChartRange) Key() string {
	return enumText(chartRangeKeys, chartRange)
}

// Span answers the period the range covers, ending at now. The invalid zero
// value covers the default range.
func (chartRange ChartRange) Span(now time.Time) TimeSpan {
	length := chartRangeDurations[ChartRangeDefault]
	if chartRange.named() {
		length = chartRangeDurations[chartRange]
	}

	return TimeSpan{From: now.Add(-length), To: now}
}

// named reports whether the range is one of the three named ranges.
func (chartRange ChartRange) named() bool {
	if chartRange == ChartRangeUnspecified {
		return false
	}

	return int(chartRange) < len(chartRangeDurations)
}

// ChartRanges answers every range, the shortest first, as a range selector
// lists them.
func ChartRanges() []ChartRange {
	return []ChartRange{ChartRangeHour, ChartRangeDay, ChartRangeWeek}
}

// ParseChartRange answers the range an address names. An absent or unknown
// name is the default range: a chart always has a period to draw.
func ParseChartRange(key string) ChartRange {
	chartRange, known := enumParse[ChartRange](chartRangeKeys, key)
	if known {
		return chartRange
	}

	return ChartRangeDefault
}

// ParseAddress reads the address of a fleet page. An address that names no
// page wraps ErrAddressUnknown: a path outside the fleet prefix, an unknown
// object, a tab its kind of object does not register, a segment that does not
// unescape, a group deeper than TreeLevelsMax. A fragment address is not a
// page: ParseFragment reads it.
func ParseAddress(target *url.URL) (Address, error) {
	escaped := target.EscapedPath()

	segments, held := pathSegments(escaped)
	if !held {
		return Address{}, addressUnknown(escaped)
	}

	address, err := parseObject(segments)
	if err != nil {
		return Address{}, addressUnknown(escaped)
	}
	address.View = ParseViewParams(target.Query())

	return address, nil
}

// pathSegments splits the escaped path of a fleet address into its escaped
// segments after the fleet prefix. It answers false for a path outside the
// prefix or without its closing slash.
func pathSegments(escaped string) ([]string, bool) {
	rest, prefixed := strings.CutPrefix(escaped, fleetPath)
	if !prefixed {
		return nil, false
	}
	if rest == "" {
		return []string{}, true
	}

	inner, closed := strings.CutSuffix(rest, "/")
	if !closed {
		return nil, false
	}

	return strings.Split(inner, "/"), true
}

// errSegments is the one reason parseObject refuses: the segments name no
// page. ParseAddress and ParseFragment turn it into the operator error, which
// names the whole address.
var errSegments = errors.New("the path segments name no page")

// parseObject reads the escaped segments after the fleet prefix into an
// address with no view.
func parseObject(segments []string) (Address, error) {
	if len(segments) == 0 {
		return Address{Kind: ObjectEstate, Tab: TabDashboard}, nil
	}

	kind, known := enumParse[ObjectKind](objectKindSegments, segments[0])
	if !known {
		return Address{}, errSegments
	}
	rest := segments[1:]

	switch kind {
	case ObjectEstate:
		return parseTabbed(Address{Kind: ObjectEstate}, rest)
	case ObjectGroup:
		return parseGroup(rest)
	case ObjectDevice:
		return parseDevice(rest)
	case ObjectTasks:
		if len(rest) != 0 {
			return Address{}, errSegments
		}
		return Address{Kind: ObjectTasks}, nil
	case ObjectAdmin:
		return parseTabbed(Address{Kind: ObjectAdmin}, rest)
	case ObjectUnspecified:
		return Address{}, errSegments
	}

	return Address{}, errSegments
}

// parseTabbed completes address with the tab that is the one segment left.
func parseTabbed(address Address, rest []string) (Address, error) {
	if len(rest) != 1 {
		return Address{}, errSegments
	}

	tab, err := url.PathUnescape(rest[0])
	if err != nil {
		return Address{}, errSegments
	}
	if !objectTabs.holds(address.Kind, tab) {
		return Address{}, errSegments
	}
	address.Tab = tab

	return address, nil
}

// parseGroup reads the segments of a group address: one per level, then the
// tab.
func parseGroup(rest []string) (Address, error) {
	levels := len(rest) - 1
	if levels < 1 {
		return Address{}, errSegments
	}
	if levels > TreeLevelsMax {
		return Address{}, errSegments
	}

	group := make([]GroupLevel, 0, levels)
	for _, segment := range rest[:levels] {
		level, err := parseGroupLevel(segment)
		if err != nil {
			return Address{}, err
		}
		group = append(group, level)
	}

	return parseTabbed(Address{Kind: ObjectGroup, Group: group}, rest[levels:])
}

// parseGroupLevel reads one escaped group segment. The key and the value are
// split before either is unescaped, so a key or a value that holds the
// separator survives.
func parseGroupLevel(segment string) (GroupLevel, error) {
	keyEscaped, valueEscaped, valued := strings.Cut(segment, groupSeparator)

	key, err := url.PathUnescape(keyEscaped)
	if err != nil {
		return GroupLevel{}, errSegments
	}
	if key == "" {
		return GroupLevel{}, errSegments
	}
	if !valued {
		return GroupLevel{Key: key, Missing: true}, nil
	}

	value, err := url.PathUnescape(valueEscaped)
	if err != nil {
		return GroupLevel{}, errSegments
	}

	return GroupLevel{Key: key, Value: value}, nil
}

// parseDevice reads the segments of a device address: the name, then the tab.
func parseDevice(rest []string) (Address, error) {
	if len(rest) != 2 {
		return Address{}, errSegments
	}

	name, err := url.PathUnescape(rest[0])
	if err != nil {
		return Address{}, errSegments
	}
	if name == "" {
		return Address{}, errSegments
	}

	return parseTabbed(Address{Kind: ObjectDevice, Device: name}, rest[1:])
}

// addressUnknown is the refusal for an address that names no fleet page.
func addressUnknown(escaped string) error {
	problem := textbuf.New().Str("No fleet page has the address ").Quoted(escaped).Str(".").Slice()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"The address names no estate tab, group, device, task list or administration page, so there is nothing to show.",
		"open "+fleetPath)
}

// bugAddressKind is the panic of an address rendered with no valid kind. Only
// code that builds an Address by hand and forgets its kind reaches it:
// ParseAddress never answers one.
const bugAddressKind = "BUG: fleetui: an address with no object kind was rendered"

// escapeSegment escapes text as one path segment, the separator of a group
// segment included, so a key or a value that holds it reads back whole.
func escapeSegment(text string) string {
	return strings.ReplaceAll(url.PathEscape(text), groupSeparator, "%3D")
}

// objectPath answers the path of the address after the fleet prefix, which is
// also the tail of the address of one of its fragments.
func (address Address) objectPath() string {
	path := textbuf.New()

	switch address.Kind {
	case ObjectEstate:
		path.Str("estate/").Str(escapeSegment(address.Tab)).Byte('/')
	case ObjectGroup:
		path.Str("group/")
		for _, level := range address.Group {
			path.Str(escapeSegment(level.Key))
			if !level.Missing {
				path.Str(groupSeparator).Str(escapeSegment(level.Value))
			}
			path.Byte('/')
		}
		path.Str(escapeSegment(address.Tab)).Byte('/')
	case ObjectDevice:
		path.Str("device/").Str(escapeSegment(address.Device)).Byte('/')
		path.Str(escapeSegment(address.Tab)).Byte('/')
	case ObjectTasks:
		path.Str("tasks/")
	case ObjectAdmin:
		path.Str("admin/").Str(escapeSegment(address.Tab)).Byte('/')
	case ObjectUnspecified:
		panic(bugAddressKind)
	default:
		panic(bugAddressKind)
	}

	return path.Slice()
}

// Path answers the escaped path of the address. The estate dashboard is the
// fleet prefix itself.
func (address Address) Path() string {
	if address.isEstateDashboard() {
		return fleetPath
	}

	return fleetPath + address.objectPath()
}

// isEstateDashboard reports whether the address is the estate's dashboard,
// the one page whose path names neither its object nor its tab.
func (address Address) isEstateDashboard() bool {
	if address.Kind != ObjectEstate {
		return false
	}

	return address.Tab == TabDashboard
}

// String answers the address as a link: its path, then its view as a query
// when the view is not the default one.
func (address Address) String() string {
	return withQuery(address.Path(), address.View.Query())
}

// withQuery joins a path and a query, and answers the path alone for an empty
// query.
func withQuery(path string, query url.Values) string {
	if len(query) == 0 {
		return path
	}

	return path + "?" + query.Encode()
}

// ParseViewParams reads the view an address carries in its query. It never
// refuses: a page number that is not a number or is below one is the first
// page, a page size that is not one of PageSizes is PageSizeDefault, and a
// chart range that is not known is ChartRangeDefault.
func ParseViewParams(query url.Values) ViewParams {
	return ViewParams{
		Filter:   query.Get(paramFilter),
		Sort:     ParseSort(query.Get(paramSort)),
		Page:     parsePageNumber(query.Get(paramPage)),
		Size:     parsePageSize(query.Get(paramSize)),
		Row:      query.Get(paramRow),
		Range:    ParseChartRange(query.Get(paramRange)),
		TreeView: treeViewParam(query),
		Sub:      query.Get(paramSub),
	}
}

// treeViewParam reads the tree view of an address. An address with no view
// parameter asks for the default view. A parameter that is present is
// answered as it is, an empty one included: ResolveTreeView judges it.
func treeViewParam(query url.Values) string {
	if query.Has(paramTreeView) {
		return query.Get(paramTreeView)
	}

	return TreeViewDefault
}

// Query answers the view as query parameters. A part that holds its default
// is left out, so the default view has an empty query and an address stays
// short. A zero part is its default, which is what an absent parameter parses
// to.
func (view ViewParams) Query() url.Values {
	query := url.Values{}

	if view.Filter != "" {
		query.Set(paramFilter, view.Filter)
	}
	if sort := view.Sort.String(); sort != "" {
		query.Set(paramSort, sort)
	}
	if view.Page > 1 {
		query.Set(paramPage, strconv.Itoa(view.Page))
	}
	if view.sizeChosen() {
		query.Set(paramSize, strconv.Itoa(view.Size))
	}
	if view.Row != "" {
		query.Set(paramRow, view.Row)
	}
	if view.rangeChosen() {
		query.Set(paramRange, view.Range.Key())
	}
	if view.treeViewChosen() {
		query.Set(paramTreeView, view.TreeView)
	}
	if view.Sub != "" {
		query.Set(paramSub, view.Sub)
	}

	return query
}

// treeViewChosen reports whether the view names a tree view other than the
// default one.
func (view ViewParams) treeViewChosen() bool {
	if view.TreeView == "" {
		return false
	}

	return view.TreeView != TreeViewDefault
}

// sizeChosen reports whether the view names a page size other than the
// default one.
func (view ViewParams) sizeChosen() bool {
	if view.Size == PageSizeDefault {
		return false
	}

	return validPageSize(view.Size)
}

// rangeChosen reports whether the view names a chart range other than the
// default one.
func (view ViewParams) rangeChosen() bool {
	if view.Range == ChartRangeDefault {
		return false
	}

	return view.Range.named()
}

// FragmentAddress is the address of one fragment of a page: a part htmx
// fetches on its own. It names the fragment, the page it belongs to, and for
// a fragment a page holds several of (a card, a chart, a sub-tab) which one.
type FragmentAddress struct {
	// Name is the registered name of the fragment.
	Name string
	// Page is the page the fragment belongs to, with its view.
	Page Address
	// Item names which one, for a fragment a page holds several of. It is
	// empty for the others.
	Item string
}

// ParseFragment reads the address of a fragment. An address that is not a
// fragment's, or whose page part names no page, wraps ErrAddressUnknown. It
// does not judge the name: the handler that owns the fragments does.
func ParseFragment(target *url.URL) (FragmentAddress, error) {
	return parseNamedPart(target, fragmentSegment)
}

// ParseAction reads the address of a prototype action, which has the shape
// of a fragment's under actionSegment: /fleet/action/<name>/<page path>. The
// name is the action's, and Item is unused.
func ParseAction(target *url.URL) (FragmentAddress, error) {
	return parseNamedPart(target, actionSegment)
}

// ActionAddress answers the address the form of the action registered under
// name posts to, for the page at address.
func ActionAddress(name string, address Address) string {
	path := fleetPath + actionSegment + "/" + escapeSegment(name) + "/" + address.objectPath()

	return withQuery(path, address.View.Query())
}

// parseNamedPart reads an address whose first segment is segment, whose
// second names a registered part, and whose rest is a page's path.
func parseNamedPart(target *url.URL, segment string) (FragmentAddress, error) {
	escaped := target.EscapedPath()

	segments, held := pathSegments(escaped)
	if !held {
		return FragmentAddress{}, addressUnknown(escaped)
	}
	// A fragment's page part is never empty: the estate dashboard's
	// fragments spell the estate and its tab out.
	if len(segments) < 3 {
		return FragmentAddress{}, addressUnknown(escaped)
	}
	if segments[0] != segment {
		return FragmentAddress{}, addressUnknown(escaped)
	}

	name, err := url.PathUnescape(segments[1])
	if err != nil {
		return FragmentAddress{}, addressUnknown(escaped)
	}
	if name == "" {
		return FragmentAddress{}, addressUnknown(escaped)
	}

	page, err := parseObject(segments[2:])
	if err != nil {
		return FragmentAddress{}, addressUnknown(escaped)
	}

	query := target.Query()
	page.View = ParseViewParams(query)

	return FragmentAddress{Name: name, Page: page, Item: query.Get(paramItem)}, nil
}

// String answers the fragment's address as a link.
func (fragment FragmentAddress) String() string {
	query := fragment.Page.View.Query()
	if fragment.Item != "" {
		query.Set(paramItem, fragment.Item)
	}

	path := fleetPath + fragmentSegment + "/" + escapeSegment(fragment.Name) + "/" + fragment.Page.objectPath()

	return withQuery(path, query)
}
