// Design: docs/architecture/fleet-interface.md -- tests of the Devices tab, its side panel and the actions of its selection

package fleetui

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The test estate of this file: the seven devices of fixture_test.go and
// devicesTestExtra connected healthy devices at site par, role cpe, so each
// page size fills a page of its own and the last page is shorter.
const (
	devicesTestExtra = 113
	devicesTestTotal = 120
	devicesTestPath  = "/fleet/estate/devices/"
)

// devicesTestSource is the test source over the larger estate.
type devicesTestSource struct {
	*testSource
}

func newDevicesTestSource() *devicesTestSource {
	return &devicesTestSource{testSource: newTestSource()}
}

func (source *devicesTestSource) Devices(context.Context) ([]Device, error) {
	devices := testDevices()
	for index := range devicesTestExtra {
		name := "cpe-" + strconv.Itoa(1000+index)
		devices = append(devices, testConnected(name, "1.4.2", 10, 20, "site=par", "role=cpe"))
	}

	return devices, nil
}

// devicesTestViews answers the views of the larger estate as of testNow.
func devicesTestViews(t *testing.T) []DeviceView {
	t.Helper()

	devices, err := newDevicesTestSource().Devices(context.Background())
	require.NoError(t, err)

	return DeriveDevices(devices, testAlarms(), testTasks(), testNow)
}

// devicesTestHandler answers the page handler over the larger estate, with a
// registry holding the two Devices pages as register_devices_pages.go
// registers them, and no other page.
func devicesTestHandler(t *testing.T) *fleetHandler {
	t.Helper()

	registry := &pageRegistry{}
	for _, kind := range []ObjectKind{ObjectEstate, ObjectGroup} {
		page, held := registeredPages.lookup(kind, devicesTab)
		require.True(t, held, "the Devices page of the %s must be registered", kind)
		registerTestPage(t, registry, page)
	}

	return &fleetHandler{source: newDevicesTestSource(), pages: registry}
}

// devicesGet answers the body of the page at target, which MUST answer 200.
func devicesGet(t *testing.T, target string) string {
	t.Helper()

	answer := getFleet(t, devicesTestHandler(t), target)
	require.Equal(t, http.StatusOK, answer.Code, "the page %s", target)

	return answer.Body.String()
}

// devicesRowName finds the device name each row's checkbox posts.
var devicesRowName = regexp.MustCompile(`name="row" value="([^"]+)"`)

// devicesRows answers the names of the rows of the table, in their order.
func devicesRows(body string) []string {
	matches := devicesRowName.FindAllStringSubmatch(body, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1])
	}

	return names
}

// TestDevicesTablePagesBySize checks that each page size shows that many
// rows and says which, and that the last page holds the rest. Method: ask
// the estate's Devices tab at each size, and at the last page of the
// default size, through the page handler.
func TestDevicesTablePagesBySize(t *testing.T) {
	for _, size := range PageSizes() {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			body := devicesGet(t, devicesTestPath+"?size="+strconv.Itoa(size))

			assert.Len(t, devicesRows(body), size)
			assert.Contains(t, body, "1 - "+strconv.Itoa(size)+" of 120 devices")
		})
	}

	last := devicesGet(t, devicesTestPath+"?page=3")
	assert.Len(t, devicesRows(last), devicesTestTotal-2*PageSizeDefault)
	assert.Contains(t, last, "101 - 120 of 120 devices")
}

// TestDevicesFilterNarrowsRowsAndCount checks that a filter keeps exactly
// the matching devices, in name order, and that the count names them.
func TestDevicesFilterNarrowsRowsAndCount(t *testing.T) {
	body := devicesGet(t, devicesTestPath+"?q="+url.QueryEscape("site:lon"))

	assert.Equal(t, []string{coreLon01, edgeLon01, edgeLon02}, devicesRows(body))
	assert.Contains(t, body, "3 of 120 devices", "the query row counts the matches out of the table's devices")
	assert.Contains(t, body, "1 - 3 of 3 devices", "the pager ranges over the matches")
}

// TestDevicesSortByStateOrdersBySeverity checks that the State column sorts
// by severity, the worst first when descending, and marks the header.
func TestDevicesSortByStateOrdersBySeverity(t *testing.T) {
	severity := map[string]int{}
	worst := 0
	for _, view := range devicesTestViews(t) {
		severity[view.Device.Name] = view.State.Severity()
		worst = max(worst, view.State.Severity())
	}

	body := devicesGet(t, devicesTestPath+"?sort=-state")
	names := devicesRows(body)
	require.NotEmpty(t, names)

	assert.Equal(t, worst, severity[names[0]], "the worst state comes first")
	for index := 1; index < len(names); index++ {
		assert.GreaterOrEqual(t, severity[names[index-1]], severity[names[index]],
			"%s sorts before %s", names[index-1], names[index])
	}
	assert.Contains(t, body, `aria-sort="descending"`)
}

// TestDevicesGroupTableHoldsItsDevices checks that a group node's table
// holds the node's devices and no other, at the first level and below it.
func TestDevicesGroupTableHoldsItsDevices(t *testing.T) {
	site := devicesGet(t, "/fleet/group/site=lon/devices/")
	assert.Equal(t, []string{coreLon01, edgeLon01, edgeLon02}, devicesRows(site))
	assert.Contains(t, site, "3 of 3 devices")

	role := devicesGet(t, "/fleet/group/site=par/role=cpe/devices/?size=100")
	assert.Len(t, devicesRows(role), 100)
	assert.Contains(t, role, "1 - 100 of 113 devices")
	assert.NotContains(t, devicesRows(role), coreLon01)
}

// TestDevicesSidePanelShowsTheRow checks that a row named in the address is
// shown in the side panel with every section, that its row is marked, and
// that the table's links keep it in the address.
func TestDevicesSidePanelShowsTheRow(t *testing.T) {
	body := devicesGet(t, devicesTestPath+"?id="+coreLon01)

	assert.Contains(t, body, `<h2 class="fleet-panel-title">`+coreLon01+`</h2>`)
	for _, section := range []string{"Identity", "Labels", "Connection", "Health", "Now", "Open alarms"} {
		assert.Contains(t, body, `<section class="fleet-panel-card" aria-label="`+section+`"`, "the %s section", section)
	}
	assert.Contains(t, body, "fleet-table-row--opened", "the row shown in the panel is marked")
	assert.Contains(t, body, `?id=`+coreLon01+`&amp;sort=-name"`, "a sort link keeps the open row")
}

// TestDevicesSidePanelUnknownRow checks that a row naming no device of the
// table shows the not-found state in the panel, and the table still shows.
func TestDevicesSidePanelUnknownRow(t *testing.T) {
	body := devicesGet(t, devicesTestPath+"?id=no-such-device")

	assert.Contains(t, body, `fleet-hero fleet-hero--`+HeroNoData.Key())
	assert.Contains(t, body, "no-such-device")
	assert.Len(t, devicesRows(body), PageSizeDefault)
}

// devicesRefusal finds the refusal paragraph of the query row.
var devicesRefusal = regexp.MustCompile(`(?s)<p class="fleet-query-refusal"[^>]*>(.*?)</p>`)

// TestDevicesMalformedFilterIsRefused checks that a filter the grammar
// refuses names the bad term and every valid field, says the table is
// unfiltered, and that the table then shows every device.
func TestDevicesMalformedFilterIsRefused(t *testing.T) {
	body := devicesGet(t, devicesTestPath+"?q="+url.QueryEscape("colour:red"))

	found := devicesRefusal.FindStringSubmatch(body)
	require.Len(t, found, 2, "the query row shows the refusal")
	refusal := found[1]

	assert.Contains(t, refusal, "colour")
	for _, field := range DeviceFilters().Fields() {
		assert.Contains(t, refusal, field, "the refusal lists the field %s", field)
	}
	assert.Contains(t, refusal, "unfiltered")

	assert.Len(t, devicesRows(body), PageSizeDefault)
	assert.Contains(t, body, "120 of 120 devices")
}

// postDevicesAction posts form to the action route of the table at page,
// and answers the recorded response.
func postDevicesAction(t *testing.T, page string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, actionPath+devicesActionName+"/"+page, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	newActionHandler(newDevicesTestSource()).ServeHTTP(recorder, request)

	return recorder
}

// TestDevicesBulkPostAnswersPrototypeToast checks that a bulk action answers
// the out-of-band toast naming the action and the devices, and changes
// nothing.
func TestDevicesBulkPostAnswersPrototypeToast(t *testing.T) {
	answer := postDevicesAction(t, "estate/devices/", url.Values{
		devicesFieldAction: {"estate-remove"},
		devicesFieldRow:    {edgeLon01, coreLon01},
	})
	require.Equal(t, http.StatusOK, answer.Code)

	body := answer.Body.String()
	assert.Contains(t, body, `hx-swap-oob="beforeend"`)
	assert.Contains(t, body, devicesPrototypeWord)
	assert.Contains(t, body, "Remove from the estate, 2 devices: "+edgeLon01+", "+coreLon01+".")
}

// TestDevicesBulkEveryMatch checks that "every match" acts on every device
// of the table the posted filter matches, not only the page's rows.
func TestDevicesBulkEveryMatch(t *testing.T) {
	answer := postDevicesAction(t, "group/site=par/devices/", url.Values{
		devicesFieldAction: {"config-push"},
		devicesFieldAll:    {devicesAllMatches},
		paramFilter:        {"role:cpe"},
	})
	require.Equal(t, http.StatusOK, answer.Code)

	assert.Contains(t, answer.Body.String(), "Push config, 113 devices: ")
	assert.Contains(t, answer.Body.String(), ", and 108 more.")
}

// TestDevicesBulkSkipsAGoneRow checks that a checked row the table no
// longer holds is skipped and reported, and the rest is acted on.
func TestDevicesBulkSkipsAGoneRow(t *testing.T) {
	answer := postDevicesAction(t, "estate/devices/", url.Values{
		devicesFieldAction: {"label-add"},
		devicesFieldRow:    {edgeLon01, "gone-device"},
	})
	require.Equal(t, http.StatusOK, answer.Code)

	body := answer.Body.String()
	assert.Contains(t, body, "Add label, 1 device: "+edgeLon01+".")
	assert.Contains(t, body, "Skipped, because no longer in this table: gone-device.")
}

// TestDevicesBulkRefusesUnknownAction checks that an action the table does
// not offer answers an error toast naming it.
func TestDevicesBulkRefusesUnknownAction(t *testing.T) {
	answer := postDevicesAction(t, "estate/devices/", url.Values{
		devicesFieldAction: {"format-disk"},
		devicesFieldRow:    {edgeLon01},
	})
	require.Equal(t, http.StatusOK, answer.Code)

	body := answer.Body.String()
	assert.Contains(t, body, "fleet-toast--"+ToastError.Key())
	assert.Contains(t, body, "format-disk")
	assert.NotContains(t, body, devicesPrototypeWord)
}

// TestDevicesActionRouteRefusesOtherPages checks that the action route
// answers only for a Devices tab.
func TestDevicesActionRouteRefusesOtherPages(t *testing.T) {
	for _, page := range []string{"device/" + edgeLon01 + "/dashboard/", "estate/alarms/", "nowhere/"} {
		answer := postDevicesAction(t, page, url.Values{devicesFieldAction: {"label-add"}, devicesFieldRow: {edgeLon01}})
		assert.Equal(t, http.StatusNotFound, answer.Code, "the page %s has no devices table", page)
	}
}

// TestDevicesDuration checks the uptime's two largest units at each
// boundary.
func TestDevicesDuration(t *testing.T) {
	for _, tc := range []struct {
		age  time.Duration
		want string
	}{
		{0, "0m"},
		{59 * time.Second, "0m"},
		{time.Minute, "1m"},
		{time.Hour - time.Minute, "59m"},
		{time.Hour, "1h 0m"},
		{24*time.Hour - time.Minute, "23h 59m"},
		{24 * time.Hour, "1d 0h"},
		{41*24*time.Hour + 3*time.Hour + 5*time.Minute, "41d 3h"},
	} {
		assert.Equal(t, tc.want, devicesDuration(tc.age), "%s", tc.age)
	}
}

// devicesEye finds the eye control of one row: its address, the address of
// the side panel fragment it asks for, the region it replaces and the
// address it pushes.
func devicesEye(t *testing.T, body, row string) (href, panel, pushed string) {
	t.Helper()

	eye := regexp.MustCompile(`<a class="fleet-icon-button" href="([^"]+)" hx-get="([^"]+)" hx-target="#fleet-side-panel" ` +
		`hx-swap="outerHTML" hx-push-url="([^"]+)" data-fleet-eye="` + regexp.QuoteMeta(row) + `"`)
	match := eye.FindStringSubmatch(body)
	require.NotNil(t, match, "the row %s has an eye that asks for the side panel alone", row)

	return html.UnescapeString(match[1]), html.UnescapeString(match[2]), html.UnescapeString(match[3])
}

// TestOpeningAPanelAnswersOnlyThePanel opens a row's side panel by its eye,
// and closes it by the panel's close control, as the browser asks them.
//
// VALIDATES: the eye of a row asks for the side panel fragment of the page
// with the row, replaces the side panel region with it and pushes the page's
// address with the row; the fragment answers the panel open on the row, with
// the address it was drawn for, and none of the table; its close control asks
// for the panel without the row, which answers the empty panel alone.
// PREVENTS: an eye that re-renders the whole table and the regions around it
// to show one row's details.
func TestOpeningAPanelAnswersOnlyThePanel(t *testing.T) {
	handler := devicesTestHandler(t)
	fragments := http.HandlerFunc(handler.serveFragment)
	opened := devicesTestPath + "?id=" + coreLon01

	href, panel, pushed := devicesEye(t, devicesGet(t, devicesTestPath), coreLon01)
	assert.Equal(t, opened, href, "without the script the eye opens the page with the row")
	assert.Equal(t, opened, pushed, "the address bar takes the page with the row")
	assert.Equal(t, fragmentBase+SidePanelFragment+"/estate/devices/?id="+coreLon01, panel)

	answer := getFleet(t, fragments, panel, headerHTMXRequest, htmxTrue, headerHTMXType, htmxPartial)
	require.Equal(t, http.StatusOK, answer.Code)
	body := answer.Body.String()

	assert.Contains(t, body, `<aside class="fleet-side-panel" id="fleet-side-panel"`, "the answer is the panel region")
	assert.Contains(t, body, `data-fleet-panel="open"`)
	assert.Contains(t, body, `data-fleet-address="`+html.EscapeString(opened)+`"`, "the region names the page with the row")
	assert.Contains(t, body, `data-fleet-row="`+coreLon01+`"`, "the region names the row the table marks")
	assert.Contains(t, body, `<h2 class="fleet-panel-title">`+coreLon01+`</h2>`)
	assert.NotContains(t, body, "<table", "the table is not sent again")
	assert.NotContains(t, body, `data-fleet-select-row`, "no row of the table is sent again")
	assert.NotContains(t, body, `id="fleet-content-body"`, "the content region is not sent again")
	assert.NotContains(t, body, `hx-swap-oob`, "no region around the panel is sent")

	closer := regexp.MustCompile(`href="` + regexp.QuoteMeta(devicesTestPath) + `" hx-get="([^"]+)" hx-target="#fleet-side-panel" ` +
		`hx-swap="outerHTML" hx-push-url="` + regexp.QuoteMeta(devicesTestPath) + `"`)
	match := closer.FindStringSubmatch(body)
	require.NotNil(t, match, "the panel's close control asks for the panel without the row")
	assert.Equal(t, fragmentBase+SidePanelFragment+"/estate/devices/", html.UnescapeString(match[1]))

	closed := getFleet(t, fragments, html.UnescapeString(match[1]), headerHTMXRequest, htmxTrue, headerHTMXType, htmxPartial)
	require.Equal(t, http.StatusOK, closed.Code)
	assert.Contains(t, closed.Body.String(), `data-fleet-panel="empty"`)
	assert.Contains(t, closed.Body.String(), "Click the eye to see details")
	assert.NotContains(t, closed.Body.String(), "<table")
}

// TestPanelAddressStillReproducesTheView asks for the address the eye pushes
// as the browser does on a reload or a shared link.
//
// VALIDATES: AC-4 -- the page with a row in its address is the whole shell
// with the panel open on that row, the row marked with the selected band and
// its eye marked current, and the panel region names the page's address.
// PREVENTS: a panel that only the eye's fragment can open, so a reload or a
// bookmark loses it.
func TestPanelAddressStillReproducesTheView(t *testing.T) {
	opened := devicesTestPath + "?id=" + coreLon01
	body := devicesGet(t, opened)

	assert.Contains(t, body, "<html", "a reload is the whole page")
	assert.Contains(t, body, `data-fleet-panel="open"`)
	assert.Contains(t, body, `data-fleet-address="`+html.EscapeString(opened)+`" data-fleet-row="`+coreLon01+`"`)
	assert.Contains(t, body, `<h2 class="fleet-panel-title">`+coreLon01+`</h2>`)
	assert.Contains(t, body, `class="fleet-table-row fleet-table-row--opened"`, "the row shown in the panel is marked")
	assert.Contains(t, body, `data-fleet-eye="`+coreLon01+`" aria-current="page"`, "its eye is marked current")
	assert.Len(t, devicesRows(body), PageSizeDefault, "the table is drawn under the panel")
}
