package fleetui

import (
	"html"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The addresses the tests below ask for.
const (
	pathEstate       = "/fleet/"
	pathEstateTab    = "/fleet/estate/devices/"
	pathDevice       = "/fleet/device/" + edgeLon01 + "/dashboard/"
	pathGroup        = "/fleet/group/site=lon/role=edge/dashboard/"
	fragmentTreePath = "/fleet/fragment/tree/estate/dashboard/"
	// tabTestPage is the tab the registered pages of the tests fill.
	tabTestPage = "devices"
)

// testHandler answers the page handler over the test estate, with the page
// registry of the test, which starts empty.
func testHandler() (*fleetHandler, *pageRegistry) {
	registry := &pageRegistry{}

	return &fleetHandler{source: newTestSource(), pages: registry}, registry
}

// TestFleetEstatePageRendersShell asks for the estate dashboard.
//
// VALIDATES: AC-1 at the handler -- the estate page is the whole shell: the
// header with the hub's name and the account, the tree, the object header,
// the tab row from the registered tabs, and a content region that is never
// blank.
// PREVENTS: a shell that renders only when a page family has registered.
func TestFleetEstatePageRendersShell(t *testing.T) {
	handler, _ := testHandler()

	answer := getFleet(t, handler, pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)
	body := answer.Body.String()

	assert.Contains(t, body, `<header class="fleet-header">`)
	assert.Contains(t, body, `<span class="fleet-hub-name">`+testEstateName+`</span>`)
	assert.Contains(t, body, `role="tree"`, "the estate tree")
	assert.Contains(t, body, `data-fleet-lazy=`, "rows below the second level load on expansion")
	assert.Contains(t, body, `>`+testEstateName+`</h1>`, "the object header names the estate")
	assert.Contains(t, body, `href="/fleet/estate/devices/"`, "the tab row links the Devices tab")
	assert.Contains(t, body, `aria-current="page"`, "the active tab is marked")
	assert.Contains(t, body, "The Dashboard page of estate is not built yet.",
		"a tab with no registered page says so")
	assert.Contains(t, body, `data-fleet-events="`+EventsPath+`"`, "the page names its event stream")
	assert.Contains(t, body, `id="fleet-task-panel"`, "the task panel")
	assert.Contains(t, body, `id="fleet-toasts"`, "the toast area")
}

// TestFleetDevicePageRendersHeaderAndTabs asks for a device's dashboard.
//
// VALIDATES: a device address renders the device's name and state word in
// the object header, the device tabs, the breadcrumb through its groups, and
// the tree expanded to the device with its row selected.
// PREVENTS: a device page that shows the estate's header, and a tree that
// loses the selected row after a reload.
func TestFleetDevicePageRendersHeaderAndTabs(t *testing.T) {
	handler, _ := testHandler()

	answer := getFleet(t, handler, pathDevice)
	require.Equal(t, http.StatusOK, answer.Code)
	body := answer.Body.String()

	assert.Contains(t, body, `>`+edgeLon01+`</h1>`)
	assert.Contains(t, body, `<span class="fleet-state-word">Healthy</span>`, "the state is written as its word")
	assert.Contains(t, body, `href="/fleet/device/`+edgeLon01+`/console/"`, "the device tabs")
	assert.Contains(t, body, `href="/fleet/group/site=lon/role=edge/dashboard/"`, "the breadcrumb through the device's groups")
	assert.Contains(t, body, `fleet-tree-row--selected`, "the device's tree row is selected")
	assert.Contains(t, body, `data-fleet-key="device/`+edgeLon02+`"`, "the device's branch is drawn open, with its siblings")
	assert.NotContains(t, body, `data-fleet-key="device/`+coreAms01+`"`, "a branch off the selected path stays unloaded")
}

// TestFleetUnknownAddressIsNotFoundInsideShell asks for addresses that name
// nothing.
//
// VALIDATES: an unknown device, a group no device is in and an address that
// names no page each answer 404 with the not-found state inside the shell,
// in the data layer's own words, and name no event stream.
// PREVENTS: a bare error page that leaves the operator with no tree to
// leave by, and a 200 for an object that does not exist.
func TestFleetUnknownAddressIsNotFoundInsideShell(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		sentence string
	}{
		{"device", "/fleet/device/no-such-device/dashboard/", "The estate holds no device named"},
		{"group", "/fleet/group/site=mars/dashboard/", "No device of the estate is in the group"},
		{"page", "/fleet/nowhere/", "No fleet page has the address"},
		{"tab", "/fleet/estate/no-such-tab/", "No fleet page has the address"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			handler, _ := testHandler()

			answer := getFleet(t, handler, test.target)
			require.Equal(t, http.StatusNotFound, answer.Code)
			body := answer.Body.String()

			assert.Contains(t, body, `<header class="fleet-header">`, "the shell is still drawn")
			assert.Contains(t, body, `role="tree"`)
			assert.Contains(t, body, test.sentence)
			assert.NotContains(t, body, `fleet-tree-row--selected`, "no row is selected")
			assert.NotContains(t, body, `data-fleet-events=`, "no stream follows an address that names nothing")
		})
	}
}

// TestFleetUnknownTreeViewIsRefused asks for a page under a tree view the hub
// does not have.
//
// VALIDATES: the refusal lists the views, the answer is 400, and the tree is
// still drawn, under the default view.
// PREVENTS: a tree regrouped by a default in silence.
func TestFleetUnknownTreeViewIsRefused(t *testing.T) {
	handler, _ := testHandler()

	answer := getFleet(t, handler, pathEstate+"?view=by-color")
	require.Equal(t, http.StatusBadRequest, answer.Code)
	body := answer.Body.String()

	assert.Contains(t, body, "This hub has no tree view named")
	assert.Contains(t, body, `data-fleet-view="`+TreeViewDefault+`"`)
}

// TestFleetAddressReproducesView asks for a page with every view parameter
// set and reads the addresses the shell renders back.
//
// VALIDATES: AC-4 -- the handler carries the filter, the sort, the page, the
// size, the row, the chart range and the tree view of the address into the
// page: the entry of the tree view in use is the address asked for, whole,
// and a link to another tab keeps the tree view and drops what belongs to
// this tab's table.
// PREVENTS: a view parameter dropped between the address and the page, which
// makes a bookmarked view open as another.
func TestFleetAddressReproducesView(t *testing.T) {
	const target = pathEstateTab + "?id=core-lon-01&page=2&q=state%3Adown&range=week&size=100&sort=-name&view=role"
	require.Equal(t, target, testAddress(t, target).String(), "the address must be in its rendered form")

	handler, _ := testHandler()

	answer := getFleet(t, handler, target)
	require.Equal(t, http.StatusOK, answer.Code)
	body := answer.Body.String()

	assert.Contains(t, body, `value="`+html.EscapeString(target)+`" selected`,
		"the tree view in use is the address asked for, with every view parameter")
	assert.Contains(t, body, `data-fleet-view="role"`)
	assert.Contains(t, body, `href="/fleet/estate/alarms/?view=role"`,
		"a link to another tab keeps the tree view and nothing of this tab's table")
}

// TestFleetRegisteredPageFillsTheShell registers pages in a registry of the
// test and asks for them.
//
// VALIDATES: the shell renders the content a registered page builds, shows
// the state that matches a builder's failure with the status of that state,
// builds the side panel only while the address names a row, and leaves a
// static page's content out of the live regions.
// PREVENTS: a page whose failure renders blank, and a terminal swapped away
// by a live event.
func TestFleetRegisteredPageFillsTheShell(t *testing.T) {
	const (
		built   = "the devices page was built"
		details = "the details of the row were built"
	)

	offline := opererr.Wrap(ErrDeviceOffline, "edge-lon-01 has been offline since 14:02.",
		"The values below were reported then.", "open the device's tasks")
	unauthorized := opererr.Wrap(ErrUnauthorized, "Your profile does not allow this view.",
		"The profile read-only grants no fleet view.", "ask an administrator for the fleet profile")
	failed := opererr.New("The devices could not be read.", "The hub did not answer.", "ze show fleet devices")

	cases := []struct {
		name   string
		page   Page
		target string
		status int
		holds  []string
		lacks  []string
	}{
		{
			name:   "content",
			page:   Page{Kind: ObjectEstate, Tab: tabTestPage, Content: sentencePage(built)},
			target: pathEstateTab, status: http.StatusOK,
			holds: []string{built, `data-fleet-name="content"`, `data-fleet-panel="none"`},
		},
		{
			name:   "static content is no live region",
			page:   Page{Kind: ObjectEstate, Tab: tabTestPage, Content: sentencePage(built), Static: true},
			target: pathEstateTab, status: http.StatusOK,
			holds: []string{built},
			lacks: []string{`data-fleet-name="content"`},
		},
		{
			name:   "offline is a state of a page that rendered",
			page:   Page{Kind: ObjectEstate, Tab: tabTestPage, Content: failingPage(offline)},
			target: pathEstateTab, status: http.StatusOK,
			holds: []string{"edge-lon-01 has been offline since 14:02.", "fleet-hero--offline"},
		},
		{
			name:   "unauthorized",
			page:   Page{Kind: ObjectEstate, Tab: tabTestPage, Content: failingPage(unauthorized)},
			target: pathEstateTab, status: http.StatusForbidden,
			holds: []string{"Your profile does not allow this view.", "fleet-hero--unauthorized"},
		},
		{
			name:   "failed names the command",
			page:   Page{Kind: ObjectEstate, Tab: tabTestPage, Content: failingPage(failed)},
			target: pathEstateTab, status: http.StatusInternalServerError,
			holds: []string{"The devices could not be read.", "ze show fleet devices", "fleet-hero--failed"},
		},
		{
			name: "side panel with no row",
			page: Page{
				Kind: ObjectEstate, Tab: tabTestPage, Content: sentencePage(built), SidePanel: sentencePage(details),
			},
			target: pathEstateTab, status: http.StatusOK,
			holds: []string{`data-fleet-panel="empty"`, "Click the eye to see details"},
			lacks: []string{details},
		},
		{
			name: "side panel with a row",
			page: Page{
				Kind: ObjectEstate, Tab: tabTestPage, Content: sentencePage(built), SidePanel: sentencePage(details),
			},
			target: pathEstateTab + "?id=" + coreLon01, status: http.StatusOK,
			holds: []string{`data-fleet-panel="open"`, details, `data-fleet-name="panel"`},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			handler, registry := testHandler()
			registerTestPage(t, registry, test.page)

			answer := getFleet(t, handler, test.target)
			require.Equal(t, test.status, answer.Code)
			body := answer.Body.String()

			for _, text := range test.holds {
				assert.Contains(t, body, text)
			}
			for _, text := range test.lacks {
				assert.NotContains(t, body, text)
			}
		})
	}
}

// TestFleetNavigationAnswersTheRegions asks for a page the way a navigation
// link does, through htmx.
//
// VALIDATES: a partial htmx request is answered with the content region and
// the regions that follow the page, out of band, and no document; a request
// that restores history gets the whole shell.
// PREVENTS: a shell nested inside its own content region after a click.
func TestFleetNavigationAnswersTheRegions(t *testing.T) {
	handler, _ := testHandler()

	part := getFleet(t, handler, pathGroup, headerHTMXRequest, htmxTrue, headerHTMXType, htmxPartial)
	require.Equal(t, http.StatusOK, part.Code)
	body := part.Body.String()

	assert.NotContains(t, body, "<html")
	assert.NotContains(t, body, `<header class="fleet-header">`)
	assert.Contains(t, body, `id="fleet-content-body"`)
	assert.Contains(t, body, `id="fleet-object" hx-swap-oob="innerHTML"`)
	assert.Contains(t, body, `id="fleet-tree" hx-swap-oob="innerHTML"`)
	assert.Contains(t, body, `hx-swap-oob="outerHTML"`, "the side panel region is replaced")
	assert.Contains(t, body, `data-fleet-title="edge - Ze Fleet"`, "the title the script applies")

	restored := getFleet(t, handler, pathGroup,
		headerHTMXRequest, htmxTrue, headerHTMXType, htmxPartial, headerHTMXRestore, htmxTrue)
	require.Equal(t, http.StatusOK, restored.Code)
	assert.Contains(t, restored.Body.String(), "<html")
}

// TestFleetPagePathComesFromThePage asks for an estate tab, a group node and
// a device tab, each as the whole shell and as a navigation's answer.
//
// VALIDATES: AC-9 -- every answer that changes the page carries, on the
// content region, the path of the page, its address with the query, the fleet
// prefix and the fragment prefix, which the script reads in place of the
// browser's location.
// PREVENTS: a script that builds its fragment and stream addresses from the
// location, which a page replayed inside a recording's frame does not have.
func TestFleetPagePathComesFromThePage(t *testing.T) {
	addresses := []struct {
		name  string
		path  string
		query string
	}{
		{name: "estate tab", path: pathEstateTab, query: "?id=" + coreLon01},
		{name: "group node", path: pathGroup, query: "?view=site-role"},
		{name: "device tab", path: pathDevice},
	}
	answers := []struct {
		name   string
		header []string
	}{
		{name: "whole shell"},
		{name: "navigation", header: []string{headerHTMXRequest, htmxTrue, headerHTMXType, htmxPartial}},
	}

	for _, address := range addresses {
		for _, answer := range answers {
			t.Run(address.name+"/"+answer.name, func(t *testing.T) {
				handler, _ := testHandler()

				recorder := getFleet(t, handler, address.path+address.query, answer.header...)
				require.Equal(t, http.StatusOK, recorder.Code)
				region := contentRegionTag(t, recorder.Body.String())

				assert.Contains(t, region, `data-fleet-path="`+html.EscapeString(address.path)+`"`)
				assert.Contains(t, region, `data-fleet-address="`+html.EscapeString(address.path+address.query)+`"`)
				assert.Contains(t, region, `data-fleet-base="/fleet/"`)
				assert.Contains(t, region, `data-fleet-fragments="/fleet/fragment/"`)
				assert.Contains(t, region, `data-fleet-events="/fleet/events"`)
			})
		}
	}
}

// contentRegionTag answers the opening tag of the content region in page,
// the one element every answer that changes the page carries exactly once.
func contentRegionTag(t *testing.T, page string) string {
	t.Helper()

	const marker = `id="fleet-content-body"`
	require.Equal(t, 1, strings.Count(page, marker), "the answer carries one content region")

	before, _, _ := strings.Cut(page, marker)
	start := strings.LastIndex(before, "<")
	require.GreaterOrEqual(t, start, 0, "the content region sits inside a tag")
	end := strings.Index(page[start:], ">")
	require.Positive(t, end, "the content region's tag is closed")

	return page[start : start+end]
}

// TestFleetTreeFragmentNarrows asks for the tree fragment with and without a
// filter.
//
// VALIDATES: the filter keeps the devices whose name or a label value holds
// the text, with their ancestors, drawn expanded; a filter that keeps nothing
// says so and offers to clear it; the answer is the tree alone.
// PREVENTS: a filter that leaves its matches inside unloaded branches.
func TestFleetTreeFragmentNarrows(t *testing.T) {
	handler, _ := testHandler()
	fragments := http.HandlerFunc(handler.serveFragment)

	whole := getFleet(t, fragments, fragmentTreePath)
	require.Equal(t, http.StatusOK, whole.Code)
	assert.NotContains(t, whole.Body.String(), "<header", "a fragment is its region alone")
	assert.NotContains(t, whole.Body.String(), `data-fleet-key="device/`+coreAms01+`"`,
		"without a filter the devices are below the second level, and load on expansion")

	narrowed := getFleet(t, fragments, fragmentTreePath+"?item=ams")
	require.Equal(t, http.StatusOK, narrowed.Code)
	body := narrowed.Body.String()

	assert.Contains(t, body, `data-fleet-key="device/`+coreAms01+`"`)
	assert.Contains(t, body, `data-fleet-key="device/`+rsAms01+`"`)
	assert.NotContains(t, body, edgeLon01, "a device the filter drops is not drawn")
	assert.NotContains(t, body, `data-fleet-lazy=`, "a filter draws everything it keeps")

	empty := getFleet(t, fragments, fragmentTreePath+"?item=no-such-text")
	require.Equal(t, http.StatusOK, empty.Code)
	assert.Contains(t, empty.Body.String(), "No devices match this filter")
	assert.Contains(t, empty.Body.String(), `data-fleet-action="tree-filter-clear"`)
}

// TestFleetTreeBranchFragment asks for the rows under one tree row.
//
// VALIDATES: the branch fragment answers the devices of the node its item
// names, and refuses a key that names no node inside the region.
// PREVENTS: an expanded row that stays on its skeleton rows.
func TestFleetTreeBranchFragment(t *testing.T) {
	handler, _ := testHandler()
	fragments := http.HandlerFunc(handler.serveFragment)

	const branchPath = "/fleet/fragment/tree-branch/estate/dashboard/?item="

	branch := getFleet(t, fragments, branchPath+"group%2Fsite%3Dlon%2Frole%3Dedge%2Fdashboard%2F")
	require.Equal(t, http.StatusOK, branch.Code)
	body := branch.Body.String()

	assert.Contains(t, body, `data-fleet-key="device/`+edgeLon01+`"`)
	assert.Contains(t, body, `data-fleet-key="device/`+edgeLon02+`"`)
	assert.NotContains(t, body, coreLon01)

	unknown := getFleet(t, fragments, branchPath+"group%2Fsite%3Dmars%2Fdashboard%2F")
	require.Equal(t, http.StatusOK, unknown.Code)
	assert.Contains(t, unknown.Body.String(), "No device of the estate is in the group")
}

// TestFleetFragmentRefusals asks for fragments that cannot be answered.
//
// VALIDATES: a fragment nothing registered answers 404 with an operator
// error, and an item longer than a filter may be answers 400.
// PREVENTS: an unbounded text reaching the tree filter or the search.
func TestFleetFragmentRefusals(t *testing.T) {
	handler, _ := testHandler()
	fragments := http.HandlerFunc(handler.serveFragment)

	unknown := getFleet(t, fragments, "/fleet/fragment/no-such-fragment/estate/dashboard/")
	assert.Equal(t, http.StatusNotFound, unknown.Code)
	assert.Contains(t, unknown.Body.String(), "fix: ")

	long := make([]byte, fragmentItemMax+1)
	for index := range long {
		long[index] = 'a'
	}
	refused := getFleet(t, fragments, fragmentTreePath+"?item="+string(long))
	assert.Equal(t, http.StatusBadRequest, refused.Code)

	longest := getFleet(t, fragments, fragmentTreePath+"?item="+string(long[:fragmentItemMax]))
	assert.Equal(t, http.StatusOK, longest.Code, "the longest item allowed is answered")
}

// TestFleetSearchFindsDevicesAndPages asks the header search.
//
// VALIDATES: typed text finds devices by name and by label value, the
// devices matching by name first, then pages by title; at most ten; empty
// text finds nothing.
// PREVENTS: the enter key opening a label match ahead of the device whose
// name was typed.
func TestFleetSearchFindsDevicesAndPages(t *testing.T) {
	request := testRequest(t, pathEstate)

	byName := searchView(request, " CORE ")
	require.Len(t, byName.Results, 2)
	assert.Equal(t, coreAms01, byName.Results[0].Title)
	assert.Equal(t, coreLon01, byName.Results[1].Title)
	assert.Equal(t, "/fleet/device/"+coreAms01+"/dashboard/", byName.Results[0].Href)

	// "lon" is in three names and in the site label of the same devices.
	bySite := searchView(request, "lon")
	require.Len(t, bySite.Results, 3)

	pages := searchView(request, "alarm")
	require.Len(t, pages.Results, 1)
	assert.Equal(t, "/fleet/estate/alarms/", pages.Results[0].Href)

	assert.Empty(t, searchView(request, "  ").Results)
	assert.LessOrEqual(t, len(searchView(request, "e").Results), searchResultsMax)
}

// TestFleetHeaderCountsTasks reads the Tasks button of the header.
//
// VALIDATES: the count is the tasks that are not over, and the dot is set
// only while a task failed.
// PREVENTS: a count that includes finished tasks.
func TestFleetHeaderCountsTasks(t *testing.T) {
	request := testRequest(t, pathEstate)

	quiet := headerTasksView(request)
	assert.Equal(t, 1, quiet.Running)
	assert.False(t, quiet.Failed)

	request.Snapshot.Tasks = append(request.Snapshot.Tasks, Task{ID: "t3", Kind: "upgrade", State: TaskStatePartlyFailed})
	failed := headerTasksView(request)
	assert.Equal(t, 1, failed.Running)
	assert.True(t, failed.Failed)
	assert.Contains(t, failed.Label, "a task failed")
}

// navigationLeaving answers the answer to a navigation to target, asked the
// way htmx asks it from the page at current.
func navigationLeaving(t *testing.T, current, target string) string {
	t.Helper()

	handler, _ := testHandler()
	answer := getFleet(t, handler, target,
		headerHTMXRequest, htmxTrue, headerHTMXType, htmxPartial, headerHTMXCurrent, "http://hub.example"+current)
	require.Equal(t, http.StatusOK, answer.Code)

	return answer.Body.String()
}

// TestNavigationWithinAnObjectOmitsTheTree asks for another tab and another
// view of the object the page in hand shows.
//
// VALIDATES: a navigation that stays on its object under the same tree view
// answers the content, the object region, the tree's view choice and the side
// panel, and neither the tree rows nor the administration list, which cannot
// have changed.
// PREVENTS: every tab, sort, page, filter and range change resending the
// sidebar.
func TestNavigationWithinAnObjectOmitsTheTree(t *testing.T) {
	cases := []struct{ name, current, target string }{
		{"another tab", pathGroup, "/fleet/group/site=lon/role=edge/devices/"},
		{"another view", pathEstateTab, pathEstateTab + "?sort=-name"},
		{"another range", pathDevice, pathDevice + "?range=week"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := navigationLeaving(t, test.current, test.target)

			assert.Contains(t, body, `id="fleet-content-body"`)
			assert.Contains(t, body, `id="fleet-object" hx-swap-oob="innerHTML"`, "the tab row's active mark follows")
			assert.Contains(t, body, `id="fleet-tree-view" hx-swap-oob="innerHTML"`, "the view choice names the page in hand")
			assert.Contains(t, body, `id="fleet-side-panel"`, "the side panel follows the tab")
			assert.NotContains(t, body, `id="fleet-tree" hx-swap-oob`, "the tree rows cannot have changed")
			assert.NotContains(t, body, `id="fleet-admin-list"`, "the administration list cannot have changed")
		})
	}
}

// TestNavigationToAnotherObjectRefreshesTheTree asks for pages whose sidebar
// differs from the page in hand's.
//
// VALIDATES: a navigation to another object, under another tree view, between
// administration pages, or from a page htmx does not name, answers the tree
// rows and the administration list out of band.
// PREVENTS: a tree that keeps the previous object selected after the
// operator opened another one.
func TestNavigationToAnotherObjectRefreshesTheTree(t *testing.T) {
	cases := []struct{ name, current, target string }{
		{"another device", pathGroup, pathDevice},
		{"another group", "/fleet/group/site=lon/dashboard/", pathGroup},
		{"another tree view", pathGroup, pathGroup + "?view=role"},
		{"another administration page", "/fleet/admin/labels/", "/fleet/admin/filters/"},
		{"a page that does not parse", "/fleet/nowhere/", pathGroup},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := navigationLeaving(t, test.current, test.target)

			assert.Contains(t, body, `id="fleet-tree" hx-swap-oob="innerHTML"`)
			assert.Contains(t, body, `id="fleet-admin-list" hx-swap-oob="innerHTML"`)
		})
	}
}
