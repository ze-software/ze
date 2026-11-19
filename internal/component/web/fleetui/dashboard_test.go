package fleetui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The addresses the dashboard tests ask for.
const (
	pathDashboardGroup   = "/fleet/group/site=lon/dashboard/"
	pathDashboardOffline = "/fleet/device/" + coreAms01 + "/dashboard/"
	pathChartEstate      = "/fleet/fragment/" + fragmentDashboardChart + "/estate/dashboard/"
)

// dashboardTestInterval is the time between two samples of the test history.
const dashboardTestInterval = 5 * time.Minute

// The hole in the test history: no device reported between these two ages.
const (
	dashboardGapFrom = 3 * time.Hour
	dashboardGapTo   = 2 * time.Hour
)

// dashboardTestSource is the test source with the answers a dashboard asks
// beyond the shell's: the rollouts, the audit trail, a divergence and the
// history. rolloutsErr makes the rollouts fail, as one card's source does.
type dashboardTestSource struct {
	*testSource
	rollouts    []Rollout
	rolloutsErr error
}

// newDashboardSource answers the dashboard test source: one paused rollout
// and one complete.
func newDashboardSource() *dashboardTestSource {
	return &dashboardTestSource{
		testSource: newTestSource(),
		rollouts: []Rollout{
			{Name: "edge-refresh", State: RolloutStatePaused, Started: testNow.Add(-time.Hour), BatchesTotal: 5, BatchesDone: 2},
			{Name: "core-timers", State: RolloutStateComplete, Started: testNow.Add(-48 * time.Hour), BatchesTotal: 2, BatchesDone: 2},
		},
	}
}

func (source *dashboardTestSource) Rollouts(context.Context) ([]Rollout, error) {
	if source.rolloutsErr != nil {
		return nil, source.rolloutsErr
	}

	return source.rollouts, nil
}

func (source *dashboardTestSource) AuditEntries(context.Context) ([]AuditEntry, error) {
	return []AuditEntry{
		{ID: "e1", At: testNow.Add(-10 * time.Minute), Actor: "thomas", Summary: "pushed the configuration", Devices: 4},
		{ID: "e2", At: testNow.Add(-time.Hour), Actor: "alice", Event: "label", Target: edgeLon01, Devices: 1},
	}, nil
}

func (source *dashboardTestSource) Divergence(_ context.Context, device string) (Divergence, error) {
	return Divergence{Device: device, Changes: 3, EditedAt: testNow.Add(-time.Hour)}, nil
}

func (source *dashboardTestSource) EstateSeries(_ context.Context, _ []string, metric Metric, span TimeSpan) (Series, error) {
	return Series{Metric: metric, Interval: dashboardTestInterval, Points: dashboardTestPoints(span, 10)}, nil
}

// DeviceSeries answers no sample for the device that never reported one.
func (source *dashboardTestSource) DeviceSeries(_ context.Context, device string, metric Metric, span TimeSpan) (Series, error) {
	if device == cpe0200 {
		return Series{Metric: metric, Interval: dashboardTestInterval, Points: []Point{}}, nil
	}

	return Series{Metric: metric, Interval: dashboardTestInterval, Points: dashboardTestPoints(span, 20)}, nil
}

// dashboardTestPoints answers one sample every interval over the last week,
// inside span, except in the hole between dashboardGapFrom and dashboardGapTo
// ago.
func dashboardTestPoints(span TimeSpan, value float64) []Point {
	points := make([]Point, 0, 7*24*12)
	holeFrom, holeTo := testNow.Add(-dashboardGapFrom), testNow.Add(-dashboardGapTo)
	for at := testNow.Add(-7 * 24 * time.Hour); at.Before(testNow); at = at.Add(dashboardTestInterval) {
		if !at.Before(holeFrom) && at.Before(holeTo) {
			continue
		}
		if at.Before(span.From) || !at.Before(span.To) {
			continue
		}
		points = append(points, Point{At: at, Value: value})
	}

	return points
}

// dashboardRequest builds the request of the page at target over source.
func dashboardRequest(t *testing.T, source Source, target string) *PageRequest {
	t.Helper()

	request, err := NewPageRequest(context.Background(), source, testAddress(t, target), testUser)
	require.NoError(t, err)

	return request
}

// dashboardScopeFor answers the scope of the dashboard at target.
func dashboardScopeFor(t *testing.T, request *PageRequest) *dashboardScope {
	t.Helper()

	scope, err := dashboardScopeOf(request)
	require.NoError(t, err)

	return &scope
}

// dashboardRowsBehind answers how many rows the devices table shows for the
// address href: the devices of the address's object, kept by the filter it
// carries, through the same functions the table uses.
func dashboardRowsBehind(t *testing.T, request *PageRequest, href string) int {
	t.Helper()

	address := testAddress(t, href)
	require.Equal(t, dashTabDevices, address.Tab, "the link %s must open the devices table", href)

	views := request.Snapshot.Views
	if address.Kind == ObjectGroup {
		grouped, err := GroupDevices(views, address.Group)
		require.NoError(t, err)
		views = grouped
	}

	filter, err := DeviceFilters().Parse(address.View.Filter, LabelKeys(request.Snapshot.Views))
	require.NoError(t, err, "the link %s must carry a filter the table accepts", href)

	return len(filter.Apply(views))
}

// requireSegmentsEqualTheirRows asserts that every segment of every donut of
// a card counts the rows its link opens.
func requireSegmentsEqualTheirRows(t *testing.T, request *PageRequest, card DonutCardView) {
	t.Helper()

	for _, donut := range card.Donuts {
		for _, segment := range donut.Segments {
			require.Equal(t, segment.Count, dashboardRowsBehind(t, request, segment.Href),
				"the %s segment %q of %s", card.Card.Title, segment.Word, segment.Href)
		}
	}
}

// dashboardHandler answers the fleet handler over source, with the pages
// this build registers, as the hub mounts it.
func dashboardHandler(source Source) *fleetHandler {
	return &fleetHandler{source: source, pages: &registeredPages}
}

// TestDashboardDonutCountsEqualTheirRows checks the first row of the estate.
//
// VALIDATES: AC-1 for the three donuts: each segment of Devices,
// Configuration and Software, the not-reported version included, counts
// exactly the rows of the devices table its link opens.
// PREVENTS: a count and a filtered table that disagree.
func TestDashboardDonutCountsEqualTheirRows(t *testing.T) {
	request := dashboardRequest(t, newDashboardSource(), pathEstate)
	scope := dashboardScopeFor(t, request)

	requireSegmentsEqualTheirRows(t, request, devicesCardView(scope, CardView{Title: titleDevices}))
	requireSegmentsEqualTheirRows(t, request, configCardView(scope, CardView{Title: titleConfiguration}))

	software := softwareCardView(scope, CardView{Title: titleSoftware})
	requireSegmentsEqualTheirRows(t, request, software)
	require.Equal(t, len(scope.views), DonutTotal(software.Donuts[0].Segments), "the Software card counts every device once")
}

// TestDashboardAttentionCountsEqualTheirRows checks Needs attention.
//
// VALIDATES: AC-1 for each line: a device line counts the rows of the
// devices table its filter keeps, and the paused rollouts line counts the
// paused rollouts and opens the rollouts table.
// PREVENTS: a line that promises more or fewer rows than it opens.
func TestDashboardAttentionCountsEqualTheirRows(t *testing.T) {
	source := newDashboardSource()
	request := dashboardRequest(t, source, pathEstate)
	scope := dashboardScopeFor(t, request)

	lines, err := NeedsAttention(scope.views, source.rollouts)
	require.NoError(t, err)
	view := attentionCardView(scope, lines, CardView{Title: titleAttention})
	require.NotEmpty(t, view.Lines)

	for index, line := range view.Lines {
		if lines[index].Kind == AttentionRolloutPaused {
			address := testAddress(t, line.Href)
			require.Equal(t, dashTabRollouts, address.Tab)
			require.Equal(t, 1, line.Count, "one test rollout is paused")

			continue
		}
		require.Equal(t, line.Count, dashboardRowsBehind(t, request, line.Href), "the line %q", line.Text)
	}
}

// TestDashboardEstateFiguresLinkTheirRows checks the Estate card.
//
// VALIDATES: the Devices figure counts the rows of the devices table it
// opens, and the open alarms figure counts the estate's open alarms and
// opens the alarms tab.
// PREVENTS: a figure whose link opens another set than it counts.
func TestDashboardEstateFiguresLinkTheirRows(t *testing.T) {
	request := dashboardRequest(t, newDashboardSource(), pathEstate)
	scope := dashboardScopeFor(t, request)

	figures := estateFigures(request, scope)
	require.Equal(t, "7", figures[0].Value)
	require.Equal(t, 7, dashboardRowsBehind(t, request, figures[0].Href))

	alarms := figures[len(figures)-1]
	require.Equal(t, "3", alarms.Value)
	require.Equal(t, dashTabAlarms, testAddress(t, alarms.Href).Tab)
}

// TestDashboardGroupCountsOnlyItsDevices checks the group node dashboard.
//
// VALIDATES: the Devices donut of the node site=lon totals its three
// devices, each segment opens the node's own devices table and counts its
// rows, and the Group card names the label that defines the node.
// PREVENTS: a group dashboard that counts the whole estate.
func TestDashboardGroupCountsOnlyItsDevices(t *testing.T) {
	request := dashboardRequest(t, newDashboardSource(), pathDashboardGroup)
	scope := dashboardScopeFor(t, request)

	devices := devicesCardView(scope, CardView{Title: titleDevices})
	require.Equal(t, 3, DonutTotal(devices.Donuts[0].Segments))
	for _, segment := range devices.Donuts[0].Segments {
		require.Equal(t, ObjectGroup, testAddress(t, segment.Href).Kind, "the segment %q opens the node's table", segment.Word)
	}
	requireSegmentsEqualTheirRows(t, request, devices)

	group := groupCardView(request, scope, CardView{Title: "Group"})
	require.Equal(t, []TagView{{Key: "site", Value: "lon"}}, group.Labels)
	require.Equal(t, "3", group.Figures[0].Value)
}

// TestDashboardEmptyAlarmsShowGoodNews checks the Alarms card of a node
// with no open alarm.
//
// VALIDATES: the card lists no row, counts zero beside its title and draws
// the good-news state.
// PREVENTS: an empty card that reads as a failure.
func TestDashboardEmptyAlarmsShowGoodNews(t *testing.T) {
	request := dashboardRequest(t, newDashboardSource(), "/fleet/group/site=lon/role=edge/dashboard/")
	scope := dashboardScopeFor(t, request)

	view := alarmsCardView(request, scope, CardView{Title: titleAlarms})
	require.Empty(t, view.List.Rows)
	require.Zero(t, view.Card.Count)

	var page bytes.Buffer
	require.NoError(t, dashboardAlarms(view).Render(context.Background(), &page))
	require.Contains(t, page.String(), "No open alarms.")
}

// TestDashboardEstatePageRendersEveryCard checks the estate dashboard at its
// real entry point.
//
// VALIDATES: /fleet/ answers 200 with a slot for every registered card of
// the estate, in registration order, and the range selector.
// PREVENTS: a card registered but never drawn.
func TestDashboardEstatePageRendersEveryCard(t *testing.T) {
	answer := getFleet(t, dashboardHandler(newDashboardSource()), pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)

	body := answer.Body.String()
	require.Contains(t, body, `aria-label="Chart range"`)

	last := -1
	for _, card := range dashboardCards.of(ObjectEstate) {
		at := strings.Index(body, `id="`+dashboardSlotPrefix+card.Name+`"`)
		require.Greater(t, at, last, "the card %s is drawn after the one before it", card.Name)
		last = at
	}
}

// TestDashboardFailingCardKeepsThePage checks one card's failure.
//
// VALIDATES: AC-5: when the rollouts cannot be read, the estate dashboard
// still answers 200, the Rollouts card shows the failed state with a retry
// of that card alone, and the Devices card renders its donut.
// PREVENTS: one failing source blanking the whole dashboard.
func TestDashboardFailingCardKeepsThePage(t *testing.T) {
	source := newDashboardSource()
	source.rolloutsErr = opererr.New("The rollouts could not be read.", "The rollout store did not answer.", "ze show fleet rollouts")

	answer := getFleet(t, dashboardHandler(source), pathEstate)
	require.Equal(t, http.StatusOK, answer.Code)

	body := answer.Body.String()
	_, rollouts, found := strings.Cut(body, `id="`+dashboardSlotPrefix+cardRollouts+`"`)
	require.True(t, found, "the page carries the rollouts card")
	require.Contains(t, rollouts, "The rollouts could not be read.")
	require.Contains(t, rollouts, "fleet-hero--failed")
	require.Contains(t, rollouts, "Retry this card")
	require.Contains(t, rollouts, `hx-get="/fleet/fragment/`+fragmentDashboardCard+`/estate/dashboard/?item=`+cardRollouts+`"`)

	_, devices, found := strings.Cut(body, `id="`+dashboardSlotPrefix+cardDevices+`"`)
	require.True(t, found, "the page carries the devices card")
	devicesCard, _, found := strings.Cut(devices, `id="`+dashboardSlotPrefix+cardConfiguration+`"`)
	require.True(t, found, "the configuration card follows the devices card")
	require.Contains(t, devicesCard, "fleet-legend")
}

// TestDashboardCardFragmentAnswersOneCard checks a card fetched alone.
//
// VALIDATES: the card fragment answers the card in its slot, and a card the
// dashboard does not have answers the not-found state under 200.
// PREVENTS: a retry that fetches nothing.
func TestDashboardCardFragmentAnswersOneCard(t *testing.T) {
	fragments := http.HandlerFunc(dashboardHandler(newDashboardSource()).serveFragment)
	base := "/fleet/fragment/" + fragmentDashboardCard + "/estate/dashboard/?item="

	answer := getFleet(t, fragments, base+cardDevices)
	require.Equal(t, http.StatusOK, answer.Code)
	require.Contains(t, answer.Body.String(), `id="`+dashboardSlotPrefix+cardDevices+`"`)

	missing := getFleet(t, fragments, base+"no-such-card")
	require.Equal(t, http.StatusOK, missing.Code)
	require.Contains(t, missing.Body.String(), "no-such-card")
}

// TestDashboardOfflineDeviceShowsTheOfflineState checks AC-6.
//
// VALIDATES: the dashboard of the offline core-ams-01 shows Quick info, the
// offline state naming when it was last seen, and the four charts, and
// none of its live cards.
// PREVENTS: an offline device's last values shown as current.
func TestDashboardOfflineDeviceShowsTheOfflineState(t *testing.T) {
	answer := getFleet(t, dashboardHandler(newDashboardSource()), pathDashboardOffline)
	require.Equal(t, http.StatusOK, answer.Code)

	body := answer.Body.String()
	require.Contains(t, body, `id="`+dashboardSlotPrefix+cardQuickInfo+`"`)
	require.Contains(t, body, `id="`+dashboardSlotPrefix+dashboardOfflineName+`"`)
	require.Contains(t, body, "fleet-hero--offline")
	require.Contains(t, body, coreAms01+" is offline.")
	require.Contains(t, body, "6 days ago")

	for _, card := range dashboardCards.of(ObjectDevice) {
		slot := `id="` + dashboardSlotPrefix + card.Name + `"`
		if card.Live {
			require.NotContains(t, body, slot, "the live card %s is left out", card.Name)
			continue
		}
		require.Contains(t, body, slot, "the card %s stays", card.Name)
	}
}

// TestDashboardConnectedDeviceShowsItsLiveCards checks the device dashboard
// of a diverged, connected device.
//
// VALIDATES: every card of the device dashboard is drawn, the Configuration
// card carries the divergence callout with the count of changes, and no
// offline state is drawn.
// PREVENTS: the offline rule hiding the cards of a connected device.
func TestDashboardConnectedDeviceShowsItsLiveCards(t *testing.T) {
	answer := getFleet(t, dashboardHandler(newDashboardSource()), "/fleet/device/"+rsAms01+"/dashboard/")
	require.Equal(t, http.StatusOK, answer.Code)

	body := answer.Body.String()
	for _, card := range dashboardCards.of(ObjectDevice) {
		require.Contains(t, body, `id="`+dashboardSlotPrefix+card.Name+`"`)
	}
	require.NotContains(t, body, `id="`+dashboardSlotPrefix+dashboardOfflineName+`"`)
	require.Contains(t, body, "3 changes")
}

// dashboardChartAnswer asks the chart fragment for target and decodes it,
// once as the documented keys and once as the typed answer.
func dashboardChartAnswer(t *testing.T, target string) (map[string]json.RawMessage, chartData) {
	t.Helper()

	fragments := http.HandlerFunc(dashboardHandler(newDashboardSource()).serveFragment)
	answer := getFleet(t, fragments, target)
	require.Equal(t, http.StatusOK, answer.Code)

	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(answer.Body.Bytes(), &keys), "the chart answers JSON: %s", answer.Body.String())

	var data chartData
	require.NoError(t, json.Unmarshal(answer.Body.Bytes(), &data))

	return keys, data
}

// TestDashboardChartAnswersTheDocumentedShape checks the traffic chart of
// the estate over the default day.
//
// VALIDATES: the answer holds the keys the head of fleet-chart.js documents,
// two series named as the legend names them, one value per timestamp, a
// null where the history has its hole and a value either side of it.
// PREVENTS: AC-7 broken: a period with no sample joined by a line.
func TestDashboardChartAnswersTheDocumentedShape(t *testing.T) {
	keys, data := dashboardChartAnswer(t, pathChartEstate+"?item="+chartTraffic)

	for _, key := range []string{"unit", "timestamps", "series", "empty", "refresh"} {
		require.Contains(t, keys, key)
	}
	require.False(t, data.Empty)
	require.Equal(t, metricUnitRate, data.Unit)
	require.LessOrEqual(t, len(data.Timestamps), dashboardChartBudget)
	require.Len(t, data.Series, 2)
	require.Equal(t, MetricTrafficIn.String(), data.Series[0].Name)
	require.Equal(t, MetricTrafficOut.String(), data.Series[1].Name)

	inHole := dashboardTimestampIndex(t, data, testNow.Add(-(dashboardGapFrom+dashboardGapTo)/2))
	beforeHole := dashboardTimestampIndex(t, data, testNow.Add(-4*time.Hour))
	for _, series := range data.Series {
		require.Len(t, series.Average, len(data.Timestamps))
		require.Nil(t, series.Average[inHole], "the hole is a null")
		require.NotNil(t, series.Average[beforeHole], "a sampled period holds a value")
	}
	require.Contains(t, string(keys["series"]), "null")
}

// dashboardTimestampIndex answers the index of the period that holds at.
func dashboardTimestampIndex(t *testing.T, data chartData, at time.Time) int {
	t.Helper()

	found := -1
	for index, stamp := range data.Timestamps {
		if stamp <= at.Unix() {
			found = index
		}
	}
	require.GreaterOrEqual(t, found, 0, "a period holds %s", at)

	return found
}

// TestDashboardChartHonoursTheRange checks the range parameter.
//
// VALIDATES: AC-4 at the data: the chart's first period starts one hour,
// one day or one week before the hub's clock as the address's range says,
// and the default is the day.
// PREVENTS: a range selector whose charts keep the old period.
func TestDashboardChartHonoursTheRange(t *testing.T) {
	cases := []struct {
		query string
		span  time.Duration
	}{
		{"&range=hour", time.Hour},
		{"", 24 * time.Hour},
		{"&range=week", 7 * 24 * time.Hour},
	}
	for _, each := range cases {
		_, data := dashboardChartAnswer(t, pathChartEstate+"?item="+chartDevicesConnected+each.query)
		require.NotEmpty(t, data.Timestamps)
		require.Equal(t, testNow.Add(-each.span).Unix(), data.Timestamps[0], "the range %q", each.query)
		require.LessOrEqual(t, len(data.Timestamps), dashboardChartBudget)
	}
}

// TestDashboardChartWithoutSamples checks a device that never reported.
//
// VALIDATES: the chart answers empty, the "No samples" message and when the
// first sample is expected, with no line.
// PREVENTS: an empty history drawn as a flat line at zero.
func TestDashboardChartWithoutSamples(t *testing.T) {
	_, data := dashboardChartAnswer(t, "/fleet/fragment/"+fragmentDashboardChart+"/device/"+cpe0200+"/dashboard/?item="+chartCPU)

	require.True(t, data.Empty)
	require.Equal(t, chartNoSamples, data.Message)
	require.Equal(t, "The first is expected at 12:05 UTC", data.Expected)
	require.Empty(t, data.Series)
}

// TestDashboardRangeSelectorMarksTheCurrentRange checks the range links.
//
// VALIDATES: the selector lists hour, day and week, marks the address's
// range, and every chart card's data address and subtitle carry it.
// PREVENTS: charts beside each other showing different periods.
func TestDashboardRangeSelectorMarksTheCurrentRange(t *testing.T) {
	request := dashboardRequest(t, newDashboardSource(), pathEstate+"?range=week")

	view := dashboardView(request)
	require.Len(t, view.Ranges, 3)
	for index, chartRange := range ChartRanges() {
		require.Equal(t, chartRange == ChartRangeWeek, view.Ranges[index].Current)
		require.Equal(t, chartRange, testAddress(t, view.Ranges[index].Href).View.Range)
	}

	chart, held := lookupChart(chartTraffic)
	require.True(t, held)
	card := chartCardView(request, chart, CardView{Title: "Traffic"})
	require.Equal(t, "Last week", card.Period)
	require.Contains(t, card.URL, "range=week")
	require.Equal(t, []ChartSeriesView{{Name: "Traffic in", Color: 1}, {Name: "Traffic out", Color: 3}}, card.Series)
}

// TestDashboardBusiestRanksByTheAddressChoice checks Busiest devices.
//
// VALIDATES: the sort parameter chooses the metric, the busiest device of
// that metric comes first and its choice is marked; an unknown metric is
// refused with the ranking error.
// PREVENTS: a ranking choice that the address does not reproduce.
func TestDashboardBusiestRanksByTheAddressChoice(t *testing.T) {
	request := dashboardRequest(t, newDashboardSource(), pathEstate+"?sort=memory")
	scope := dashboardScopeFor(t, request)

	view, err := busiestCardView(request, scope, CardView{Title: titleBusiest})
	require.NoError(t, err)
	require.Equal(t, edgeLon02, view.Rows[0].Device)
	require.Equal(t, 91, view.Rows[0].Bar.Percent)
	for _, choice := range view.Choices {
		require.Equal(t, choice.Label == MetricMemory.String(), choice.Current)
	}

	refused := dashboardRequest(t, newDashboardSource(), pathEstate+"?sort=bogus")
	_, err = busiestCardView(refused, dashboardScopeFor(t, refused), CardView{Title: titleBusiest})
	require.ErrorIs(t, err, ErrRankRefused)
}

// TestDashboardChartRefusalsNameTheirFix asks for a chart no chart
// registered.
//
// VALIDATES: the chart's message holds the problem, why and fix.
// PREVENTS: a chart message that names the problem and not what to do.
func TestDashboardChartRefusalsNameTheirFix(t *testing.T) {
	unknown := chartAnswer(t.Context(), nil, "nosuchchart")
	assert.True(t, unknown.Empty)
	assert.Contains(t, unknown.Message, `"nosuchchart"`)
	assert.Contains(t, unknown.Message, "Fix: reload the page")

	failed := chartFailed(ErrDeadline)
	assert.Contains(t, failed.Message, "Fix: ze show doctor")
}
