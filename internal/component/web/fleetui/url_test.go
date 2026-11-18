package fleetui

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseTarget parses the text of an address as a request would carry it.
func parseTarget(t *testing.T, text string) *url.URL {
	t.Helper()

	target, err := url.Parse(text)
	require.NoError(t, err, text)

	return target
}

// TestFleetURLRoundTrip parses one address of each form the fleet interface
// has, with and without view parameters, and renders it back.
//
// VALIDATES: AC-4 -- an estate tab, a group node of one and of two levels, the
// node of the devices that lack a label, a group value that holds a slash and
// an equals sign, a device tab, the task list and an administration page each
// render back to the text they were parsed from, view parameters included.
// PREVENTS: a link that opens another view than the one it was built for.
func TestFleetURLRoundTrip(t *testing.T) {
	addresses := []string{
		"/fleet/",
		"/fleet/?range=week",
		"/fleet/estate/devices/",
		"/fleet/estate/devices/?q=state%3Aoffline&sort=-cpu",
		"/fleet/estate/devices/?id=edge-lon-01&page=3&q=site%3Alon+state%3A%21healthy&size=100&sort=state",
		"/fleet/estate/alarms/?size=25",
		"/fleet/estate/rollouts/",
		"/fleet/estate/tasks/",
		"/fleet/group/site=lon/dashboard/",
		"/fleet/group/site=lon/role=edge/devices/?view=role",
		"/fleet/group/site/dashboard/",
		"/fleet/group/site=a%2Fb/role=x%3Dy/alarms/",
		"/fleet/group/state=down/tasks/?view=state",
		"/fleet/device/edge-lon-04/dashboard/?range=hour",
		"/fleet/device/edge-lon-04/console/",
		"/fleet/device/edge%20lon/logs/?q=error",
		"/fleet/tasks/",
		"/fleet/tasks/?page=2",
		"/fleet/admin/audit/?q=actor%3Athomas",
		"/fleet/admin/tree-views/",
	}

	for _, text := range addresses {
		t.Run(text, func(t *testing.T) {
			address, err := ParseAddress(parseTarget(t, text))
			require.NoError(t, err)

			assert.Equal(t, text, address.String())
		})
	}
}

// TestFleetURLParsesTypedParts parses a group address and reads the typed
// values it carries.
//
// VALIDATES: the kind, the group path with each segment unescaped once, the
// tab and every view parameter arrive as typed values.
// PREVENTS: a handler reading a path segment or a query parameter itself.
func TestFleetURLParsesTypedParts(t *testing.T) {
	text := "/fleet/group/site=a%2Fb/role/devices/?q=has%3Aalarm&sort=-state&page=2&size=25&id=edge-lon-01&range=week&view=role"

	address, err := ParseAddress(parseTarget(t, text))
	require.NoError(t, err)

	assert.Equal(t, ObjectGroup, address.Kind)
	assert.Equal(t, []GroupLevel{{Key: "site", Value: "a/b"}, {Key: "role", Missing: true}}, address.Group)
	assert.Equal(t, "devices", address.Tab)
	assert.Equal(t, ViewParams{
		Filter:   "has:alarm",
		Sort:     SortSpec{Column: "state", Descending: true},
		Page:     2,
		Size:     25,
		Row:      "edge-lon-01",
		Range:    ChartRangeWeek,
		TreeView: "role",
	}, address.View)
}

// TestFleetURLEstateDashboardIsThePrefix parses the two spellings of the
// estate dashboard.
//
// VALIDATES: the fleet prefix and the spelled-out estate dashboard are one
// page, and it renders as the prefix.
// PREVENTS: two addresses for one view in the links the interface renders.
func TestFleetURLEstateDashboardIsThePrefix(t *testing.T) {
	short, err := ParseAddress(parseTarget(t, "/fleet/"))
	require.NoError(t, err)

	spelled, err := ParseAddress(parseTarget(t, "/fleet/estate/dashboard/"))
	require.NoError(t, err)

	assert.Equal(t, short, spelled)
	assert.Equal(t, ObjectEstate, short.Kind)
	assert.Equal(t, TabDashboard, short.Tab)
	assert.Equal(t, "/fleet/", spelled.String())
}

// TestFleetURLRefusesUnknownAddress parses addresses that name no page.
//
// VALIDATES: a path outside the prefix, a missing closing slash, an unknown
// object, a tab its kind does not register, a missing tab, a group with no
// level or an empty key, a group deeper than the limit, a trailing segment
// and a fragment address are each refused as an unknown address.
// PREVENTS: a mistyped link answered with the estate page under a 200.
func TestFleetURLRefusesUnknownAddress(t *testing.T) {
	tooDeep := "/fleet/group/" + strings.Repeat("k=v/", TreeLevelsMax+1) + "dashboard/"

	addresses := []string{
		"/fleet",
		"/other/",
		"/fleet/estate/devices",
		"/fleet//",
		"/fleet/nothing/",
		"/fleet/estate/",
		"/fleet/estate/console/",
		"/fleet/estate/devices/extra/",
		"/fleet/group/dashboard/",
		"/fleet/group/=lon/dashboard/",
		"/fleet/group/site=lon/rollouts/",
		tooDeep,
		"/fleet/device/edge-lon-04/",
		"/fleet/device/edge-lon-04/rollouts/",
		"/fleet/device//dashboard/",
		"/fleet/tasks/extra/",
		"/fleet/admin/",
		"/fleet/admin/nothing/",
		"/fleet/fragment/tree/estate/dashboard/",
		"/fleet/events",
		"/fleet/assets/fleet.js",
	}

	for _, text := range addresses {
		t.Run(text, func(t *testing.T) {
			_, err := ParseAddress(parseTarget(t, text))

			refusal := requireRefusal(t, err, ErrAddressUnknown)
			assert.Contains(t, refusal.Problem(), "No fleet page has the address")
		})
	}
}

// TestFleetURLGroupDepthLimit parses a group address of exactly the deepest
// tree.
//
// VALIDATES: a group of TreeLevelsMax levels is accepted.
// PREVENTS: the depth limit refusing the deepest tree a view may declare.
func TestFleetURLGroupDepthLimit(t *testing.T) {
	deepest := "/fleet/group/" + strings.Repeat("k=v/", TreeLevelsMax) + "dashboard/"

	address, err := ParseAddress(parseTarget(t, deepest))
	require.NoError(t, err)

	assert.Len(t, address.Group, TreeLevelsMax)
}

// TestFleetURLPageSizeBoundary parses the page size parameter at each offered
// size and outside them.
//
// VALIDATES: 25, 50 and 100 are kept, and any other value, an absent one
// included, is 50.
// PREVENTS: a table asked for a page of a size the interface does not offer.
func TestFleetURLPageSizeBoundary(t *testing.T) {
	cases := map[string]int{
		"size=25":  25,
		"size=50":  50,
		"size=100": 100,
		"":         50,
		"size=":    50,
		"size=0":   50,
		"size=24":  50,
		"size=26":  50,
		"size=101": 50,
		"size=-25": 50,
		"size=all": 50,
	}

	for query, want := range cases {
		values, err := url.ParseQuery(query)
		require.NoError(t, err)

		assert.Equal(t, want, ParseViewParams(values).Size, query)
	}
}

// TestFleetURLPageNumberBoundary parses the page number parameter at and
// below its first valid value.
//
// VALIDATES: a number of one or more is kept, and zero, a negative number, a
// word and an absent parameter are the first page.
// PREVENTS: a table asked for a page before its first.
func TestFleetURLPageNumberBoundary(t *testing.T) {
	cases := map[string]int{
		"page=1":   1,
		"page=2":   2,
		"page=999": 999,
		"":         1,
		"page=0":   1,
		"page=-3":  1,
		"page=two": 1,
	}

	for query, want := range cases {
		values, err := url.ParseQuery(query)
		require.NoError(t, err)

		assert.Equal(t, want, ParseViewParams(values).Page, query)
	}
}

// TestFleetURLChartRangeBoundary parses the chart range parameter.
//
// VALIDATES: hour, day and week are kept, and an unknown or absent value is
// day.
// PREVENTS: a chart with no period to draw.
func TestFleetURLChartRangeBoundary(t *testing.T) {
	cases := map[string]ChartRange{
		"range=hour":  ChartRangeHour,
		"range=day":   ChartRangeDay,
		"range=week":  ChartRangeWeek,
		"":            ChartRangeDay,
		"range=month": ChartRangeDay,
		"range=":      ChartRangeDay,
		"range=WEEK":  ChartRangeDay,
	}

	for query, want := range cases {
		values, err := url.ParseQuery(query)
		require.NoError(t, err)

		assert.Equal(t, want, ParseViewParams(values).Range, query)
	}
}

// TestChartRangeSpan asks each range for its period.
//
// VALIDATES: each range ends at the given time and starts an hour, a day or
// a week before it.
// PREVENTS: a chart whose period reads the wall clock.
func TestChartRangeSpan(t *testing.T) {
	hours := map[ChartRange]float64{ChartRangeHour: 1, ChartRangeDay: 24, ChartRangeWeek: 168}

	for chartRange, want := range hours {
		span := chartRange.Span(testNow)

		assert.Equal(t, testNow, span.To, chartRange.Key())
		assert.InDelta(t, want, span.To.Sub(span.From).Hours(), 0, chartRange.Key())
	}
}

// TestFleetURLDefaultViewHasNoQuery renders an address whose view holds only
// defaults.
//
// VALIDATES: the first page, the default size, the default range and the
// default tree view are left out of the query, spelled out or zero.
// PREVENTS: every link of the interface carrying parameters that say nothing.
func TestFleetURLDefaultViewHasNoQuery(t *testing.T) {
	spelled, err := ParseAddress(parseTarget(t, "/fleet/estate/devices/?page=1&size=50&range=day&view=site-role"))
	require.NoError(t, err)
	assert.Equal(t, "/fleet/estate/devices/", spelled.String())

	zero := Address{Kind: ObjectEstate, Tab: "devices"}
	assert.Equal(t, "/fleet/estate/devices/", zero.String())
}

// TestFleetFragmentRoundTrip parses fragment addresses and renders them back.
//
// VALIDATES: a fragment address carries its name, the page it belongs to
// with its view, and the item it shows.
// PREVENTS: a fragment fetched for another page or another view than the one
// that holds it.
func TestFleetFragmentRoundTrip(t *testing.T) {
	addresses := []string{
		"/fleet/fragment/card/estate/dashboard/?item=devices&range=week",
		"/fleet/fragment/tree/device/edge-lon-04/system/?view=role",
		"/fleet/fragment/table/group/site=lon/role/devices/?page=2&q=has%3Aalarm",
		"/fleet/fragment/panel/tasks/",
		"/fleet/fragment/table/admin/audit/",
	}

	for _, text := range addresses {
		t.Run(text, func(t *testing.T) {
			fragment, err := ParseFragment(parseTarget(t, text))
			require.NoError(t, err)

			assert.Equal(t, text, fragment.String())
		})
	}
}

// TestFleetFragmentRefusesWhatIsNotAFragment parses addresses that are not a
// fragment's.
//
// VALIDATES: a page address, a fragment with no page part and a fragment of
// an unknown page are refused as unknown addresses.
// PREVENTS: a fragment route answering for a page that does not exist.
func TestFleetFragmentRefusesWhatIsNotAFragment(t *testing.T) {
	addresses := []string{
		"/fleet/estate/devices/",
		"/fleet/fragment/",
		"/fleet/fragment/card/",
		"/fleet/fragment/card/estate/nothing/",
		"/fleet/fragment//estate/dashboard/",
	}

	for _, text := range addresses {
		t.Run(text, func(t *testing.T) {
			_, err := ParseFragment(parseTarget(t, text))

			requireRefusal(t, err, ErrAddressUnknown)
		})
	}
}

// TestTabsAreTheRegisteredSet lists the tabs of each kind of object.
//
// VALIDATES: each kind has the tabs the fleet interface fixes, in the order
// the tab row shows them, and the task list has none.
// PREVENTS: a tab an address accepts and the tab row does not show.
func TestTabsAreTheRegisteredSet(t *testing.T) {
	names := func(kind ObjectKind) []string {
		tabs := Tabs(kind)
		found := make([]string, 0, len(tabs))
		for _, tab := range tabs {
			found = append(found, tab.Name)
		}

		return found
	}

	assert.Equal(t, []string{"dashboard", "devices", "alarms", "rollouts", "tasks"}, names(ObjectEstate))
	assert.Equal(t, []string{"dashboard", "devices", "alarms", "tasks"}, names(ObjectGroup))
	assert.Equal(t, []string{
		"dashboard", "console", "system", "interfaces", "routing", "config", "logs", "tasks",
	}, names(ObjectDevice))
	assert.Equal(t, []string{"labels", "tree-views", "filters", "access", "audit", "settings"}, names(ObjectAdmin))
	assert.Empty(t, names(ObjectTasks))
}

// TestTabRegistryRefusesBadRegistrations registers into a registry of its
// own.
//
// VALIDATES: a kind that takes no tab, a tab with no name or no title and a
// duplicate are each refused, and a refused tab is not listed.
// PREVENTS: an address accepting a tab that nothing renders.
func TestTabRegistryRefusesBadRegistrations(t *testing.T) {
	var registry tabRegistry

	require.NoError(t, registry.register(ObjectDevice, Tab{Name: "bfd", Title: "BFD"}))

	require.ErrorIs(t, registry.register(ObjectTasks, Tab{Name: "bfd", Title: "BFD"}), errTabKind)
	require.ErrorIs(t, registry.register(ObjectUnspecified, Tab{Name: "bfd", Title: "BFD"}), errTabKind)
	require.ErrorIs(t, registry.register(ObjectDevice, Tab{Title: "BFD"}), errTabUnnamed)
	require.ErrorIs(t, registry.register(ObjectDevice, Tab{Name: "ospf"}), errTabUnnamed)
	require.ErrorIs(t, registry.register(ObjectDevice, Tab{Name: "bfd", Title: "Again"}), errTabDuplicate)

	assert.Equal(t, []Tab{{Name: "bfd", Title: "BFD"}}, registry.list(ObjectDevice))
	assert.True(t, registry.holds(ObjectDevice, "bfd"))
	assert.False(t, registry.holds(ObjectGroup, "bfd"))
}
