// Design: docs/architecture/fleet-interface.md -- tests of what the pages look like: breadcrumb, table columns, side panel, chart series, style variables
// Related: assets/fleet-shell.css -- the variables every fleet stylesheet reads
// Related: summary.go -- Reduce, which fits a series to the width of a chart

package fleetui

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEstatePageHasNoBreadcrumb asks for the estate dashboard.
//
// VALIDATES: the estate page draws no breadcrumb, because its one step would
// repeat the name of the object header directly above it.
// PREVENTS: the hub's name written twice at the top of the estate page.
func TestEstatePageHasNoBreadcrumb(t *testing.T) {
	handler, _ := testHandler()

	answer := getFleet(t, handler, pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)

	assert.NotContains(t, answer.Body.String(), `class="fleet-breadcrumb"`)
}

// TestDevicePageHasBreadcrumb asks for a device's dashboard.
//
// VALIDATES: a device page keeps its breadcrumb, from the estate as a link to
// the device as the current step.
// PREVENTS: the estate's rule taking the way back away from every page.
func TestDevicePageHasBreadcrumb(t *testing.T) {
	handler, _ := testHandler()

	answer := getFleet(t, handler, pathDevice)
	require.Equal(t, http.StatusOK, answer.Code)
	body := answer.Body.String()

	assert.Contains(t, body, `class="fleet-breadcrumb"`)
	assert.Contains(t, body, `class="fleet-breadcrumb-current" aria-current="page">`+edgeLon01+`<`)
}

// TestDevicesTableDrawsEverySpecColumn renders the Devices tab.
//
// VALIDATES: the table holds the eleven columns of the spec: the selection
// checkbox, then Device, State, Labels, Version, Config, Uptime, Last seen,
// CPU, Memory and Alarms, and the fixed controls column.
// PREVENTS: a column dropped from the table while its width problem is fixed.
func TestDevicesTableDrawsEverySpecColumn(t *testing.T) {
	body := devicesGet(t, devicesTestPath)

	assert.Contains(t, body, `data-fleet-select-row="true"`, "the selection column")
	for _, word := range []string{
		devicesWordDevice, devicesWordState, devicesWordLabels, devicesWordVersion, devicesWordConfig,
		devicesWordUptime, devicesWordLastSeen, devicesWordCPU, devicesWordMemory, devicesWordAlarms,
	} {
		assert.Contains(t, body, `<span>`+word+`</span>`, "the %s column", word)
	}
	assert.Contains(t, body, `class="fleet-table-controls"`, "the fixed controls column")
}

// TestDevicesRowShowsTwoTagsWithoutModel fills the cells of one row whose
// device carries four labels, one of them its model.
//
// VALIDATES: the Labels cell shows at most two tags, never the model label,
// and counts the others, whose key=value pairs its title lists.
// PREVENTS: a Labels column so wide it pushes Uptime to Alarms out of view.
func TestDevicesRowShowsTwoTagsWithoutModel(t *testing.T) {
	request := testRequest(t, devicesTestPath)
	view := DeviceView{Device: testConnected(edgeLon01, "1.4.2", 10, 20,
		"model=ZX-9600", "site=lon", "role=edge", "rack=r1")}

	cells := devicesCells(request, &view)
	index := slices.IndexFunc(cells, func(cell CellView) bool { return cell.Kind == CellTags })
	require.GreaterOrEqual(t, index, 0, "the row has a Labels cell")
	tags := cells[index]

	require.LessOrEqual(t, len(tags.Tags), 2)
	for _, tag := range tags.Tags {
		assert.NotEqual(t, fieldModel, tag.Key, "the model is inventory, shown in the side panel")
	}
	assert.Equal(t, 1, tags.More, "three labels but the model, two shown")
	assert.Contains(t, tags.Title, "rack=r1", "the hidden label is named in the title")
	assert.NotContains(t, tags.Title, "model=", "the model is not a hidden label either")
}

// TestPageWithoutSidePanelBuilderHidesTheRegion renders a page registered
// with no side panel builder.
//
// VALIDATES: the side panel region is hidden, so the page takes the full
// width, and its placeholder sentence is not drawn.
// PREVENTS: "Click the eye to see details" pinned beside a page with no row
// to open.
func TestPageWithoutSidePanelBuilderHidesTheRegion(t *testing.T) {
	handler, registry := testHandler()
	registerTestPage(t, registry, Page{Kind: ObjectEstate, Tab: TabDashboard, Content: sentencePage("A page")})

	answer := getFleet(t, handler, pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)
	body := answer.Body.String()

	panel := regexp.MustCompile(`<aside class="fleet-side-panel"[^>]*>`).FindString(body)
	require.NotEmpty(t, panel, "the region is in the shell, for a navigation to replace")
	assert.Regexp(t, `\shidden[\s>=]`, panel, "the region is hidden")
	assert.NotContains(t, body, "Click the eye to see details")
}

// sampleTestSource is the test source marked as answering sample data.
type sampleTestSource struct {
	*testSource
}

// SampleData marks the source as a sample.
func (sampleTestSource) SampleData() {}

// TestHeaderTagsSampleSource renders the estate page over a sample source
// and over a real one.
//
// VALIDATES: the header carries the "Sample data" tag when the source
// implements SampleSource, and only then.
// PREVENTS: an invented estate presented as a real one, and a real hub
// labeled as a sample.
func TestHeaderTagsSampleSource(t *testing.T) {
	sample := &fleetHandler{source: sampleTestSource{testSource: newTestSource()}, pages: &pageRegistry{}}
	answer := getFleet(t, sample, pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)
	assert.Contains(t, answer.Body.String(), `class="fleet-sample-tag"`)

	realHandler, _ := testHandler()
	answer = getFleet(t, realHandler, pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)
	assert.NotContains(t, answer.Body.String(), `class="fleet-sample-tag"`)
}

// steppingTestSource is the test source with a script that can be stepped.
type steppingTestSource struct {
	*testSource
}

func (steppingTestSource) Step() (Event, error) {
	return Event{Kind: EventDeviceChanged, Device: edgeLon01}, nil
}

func (steppingTestSource) Hold() {}

// TestHeaderDrawsScriptControlsOnlyForAStepper renders the estate page over
// a source that can be stepped and over one that cannot.
//
// VALIDATES: the header carries Step and Hold, each a submit button of its
// own form posting to its prototype-only route with nothing swapped, when
// the source implements Stepper, and no script control otherwise.
// PREVENTS: a real hub's header drawing controls that post to routes it
// never mounts, and a review build with no way to step the estate from the
// page being judged.
func TestHeaderDrawsScriptControlsOnlyForAStepper(t *testing.T) {
	stepping := &fleetHandler{source: steppingTestSource{testSource: newTestSource()}, pages: &pageRegistry{}}
	answer := getFleet(t, stepping, pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)
	header := regexp.MustCompile(`(?s)<header class="fleet-header">.*?</header>`).FindString(answer.Body.String())
	require.NotEmpty(t, header, "the page draws its header")

	assert.Contains(t, header, `id="fleet-proto-step-form" method="post" action="/fleet/prototype/step" hx-post="/fleet/prototype/step" hx-target="this" hx-swap="none"`)
	assert.Contains(t, header, `id="fleet-proto-hold-form" method="post" action="/fleet/prototype/hold" hx-post="/fleet/prototype/hold" hx-target="this" hx-swap="none"`)
	assert.Regexp(t, `<button id="fleet-proto-step" type="submit"[^>]* form="fleet-proto-step-form"`, header)
	assert.Regexp(t, `<button id="fleet-proto-hold" type="submit"[^>]* form="fleet-proto-hold-form"`, header)

	realHandler, _ := testHandler()
	answer = getFleet(t, realHandler, pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)
	body := answer.Body.String()
	assert.NotContains(t, body, "fleet-proto-step")
	assert.NotContains(t, body, "fleet-proto-hold")
	assert.NotContains(t, body, "fleet-header-script")
}

// TestReduceWeekOfEvenSamplesHasNoGap reduces a week of samples taken every
// thirty minutes, as the history answers a week, with no outage.
//
// VALIDATES: the reduction at the interval of the series and the chart's
// budget answers no null between two points: the week draws as one line.
// PREVENTS: a week chart drawn as isolated points with empty space between.
func TestReduceWeekOfEvenSamplesHasNoGap(t *testing.T) {
	const interval = 30 * time.Minute
	// The hub's clock is not on the grid of the samples, as in a real answer.
	now := testNow.Add(7*time.Minute + 13*time.Second)
	span := ChartRangeWeek.Span(now)
	first := span.From.Truncate(interval).Add(interval)

	points := make([]Point, 0, 7*48)
	for at := first; at.Before(span.To); at = at.Add(interval) {
		points = append(points, Point{At: at, Value: float64(len(points) % 7)})
	}

	buckets, err := Reduce(points, span, interval, dashboardChartBudget)
	require.NoError(t, err)
	require.NotEmpty(t, buckets)

	for index := range buckets {
		assert.False(t, buckets[index].Gap(), "the period starting %s has no sample", buckets[index].At)
	}
}

// TestFleetStylesDeclareEveryVariable reads every stylesheet of the package.
//
// VALIDATES: each custom property a stylesheet under assets/ reads with
// var(--name) is declared in fleet-shell.css.
// PREVENTS: a rule that reads an undeclared variable and silently draws
// nothing, as the diff colors did.
func TestFleetStylesDeclareEveryVariable(t *testing.T) {
	shell, err := os.ReadFile(filepath.Join("assets", "fleet-shell.css"))
	require.NoError(t, err)

	declared := map[string]bool{}
	for _, match := range regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+)\s*:`).FindAllStringSubmatch(string(shell), -1) {
		declared[match[1]] = true
	}

	sheets, err := filepath.Glob(filepath.Join("assets", "*.css"))
	require.NoError(t, err)
	require.NotEmpty(t, sheets)

	used := regexp.MustCompile(`var\(\s*(--[a-z0-9-]+)`)
	for _, sheet := range sheets {
		text, readErr := os.ReadFile(sheet) //nolint:gosec // a path the glob above answered, inside the package
		require.NoError(t, readErr)
		for _, match := range used.FindAllStringSubmatch(string(text), -1) {
			assert.True(t, declared[match[1]], "%s reads %s, which fleet-shell.css does not declare",
				strings.TrimPrefix(sheet, "assets"+string(filepath.Separator)), match[1])
		}
	}
}
