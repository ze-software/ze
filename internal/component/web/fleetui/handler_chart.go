// Design: docs/architecture/fleet-interface.md -- the chart cards of a dashboard and the JSON they draw
// Related: handler_dashboard.go -- the dashboards the chart cards are registered on
// Related: summary.go -- Reduce, which fits a series to the width of a chart
// Related: assets/fleet-chart.js -- the script that fetches the JSON and draws it; its head states the shape
//
// A chart card names the address of its data: the dashboard-chart fragment of
// the dashboard's page, its item the chart's key, with the dashboard's range.
// The fragment answers JSON, already reduced on the hub, so the browser never
// receives more points than a chart can draw. A period with no sample is a
// null, which the script draws as a gap. The answer never fails as markup: a
// source that cannot answer is an answer with no line and the refusal as its
// message, which the card shows in place of the plot.

package fleetui

import (
	"context"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// dashboardChartBudget is the most points of one series a chart is sent:
// about the width in pixels of the widest chart card.
const dashboardChartBudget = 300

// The keys of the charts, the item of their fragment address.
const (
	chartDevicesConnected = "devices-connected"
	chartTraffic          = "traffic"
	chartCPU              = "cpu"
	chartMemory           = "memory"
	chartLoad             = "load"
)

// chartColors is the chart colour of each series of a plot, in order: the
// second series is the third colour, so two lines differ by more than a
// legend.
var chartColors = []int{1, 3, 2, 4}

// dashboardChart is one chart a dashboard can draw: its key and the metrics
// it plots, at most chartSeriesMax.
type dashboardChart struct {
	Key     string
	Metrics []Metric
}

// dashboardCharts holds every chart, by key.
var dashboardCharts = []dashboardChart{
	{Key: chartDevicesConnected, Metrics: []Metric{MetricDevicesConnected}},
	{Key: chartTraffic, Metrics: []Metric{MetricTrafficIn, MetricTrafficOut}},
	{Key: chartCPU, Metrics: []Metric{MetricCPU}},
	{Key: chartMemory, Metrics: []Metric{MetricMemory}},
	{Key: chartLoad, Metrics: []Metric{MetricLoad}},
}

// lookupChart answers the chart of key, or false.
func lookupChart(key string) (dashboardChart, bool) {
	for _, chart := range dashboardCharts {
		if chart.Key == key {
			return chart, true
		}
	}

	return dashboardChart{}, false
}

// dashboardChartCard answers the builder of the card of the chart of key.
// It panics on a key no chart has, which only a registration can name.
func dashboardChartCard(key string) dashboardCardBuilder {
	chart, held := lookupChart(key)
	if !held {
		panic("BUG: fleetui: a chart card names the chart " + key + ", which does not exist")
	}

	return func(_ context.Context, request *PageRequest, _ *dashboardScope, frame CardView) (templ.Component, error) {
		return ChartCard(chartCardView(request, chart, frame)), nil
	}
}

// chartCardView answers the card of one chart: its title, the period of the
// dashboard's range under it, the address of its data and its legend.
func chartCardView(request *PageRequest, chart dashboardChart, frame CardView) ChartCardView {
	series := make([]ChartSeriesView, 0, len(chart.Metrics))
	for index, metric := range chart.Metrics {
		series = append(series, ChartSeriesView{Name: metric.String(), Color: chartColors[index%len(chartColors)]})
	}

	return ChartCardView{
		Card:   frame,
		Period: dashboardRange(request).String(),
		URL:    FragmentLink(fragmentDashboardChart, request.Address, chart.Key),
		Unit:   chart.Metrics[0].Unit(),
		Series: series,
	}
}

// chartData is the JSON a chart is drawn from, in the shape the head of
// assets/fleet-chart.js states.
type chartData struct {
	Unit       string            `json:"unit"`
	Timestamps []int64           `json:"timestamps"`
	Series     []chartDataSeries `json:"series"`
	Empty      bool              `json:"empty"`
	Message    string            `json:"message,omitempty"`
	Expected   string            `json:"expected,omitempty"`
	Refresh    int               `json:"refresh,omitempty"`
}

// chartDataSeries is one line of a chart. A nil value is a period with no
// sample: it is written null, and drawn as a gap.
type chartDataSeries struct {
	Name    string     `json:"name"`
	Average []*float64 `json:"average"`
	Maximum []*float64 `json:"maximum"`
}

// The message of a chart with nothing to draw.
const chartNoSamples = "No samples have been reported yet"

// dashboardChartFragment answers the JSON of the chart its item names, over
// the devices of the page's object and the page's range.
func dashboardChartFragment(ctx context.Context, request *PageRequest, item string) (templ.Component, error) {
	return fragmentJSON(chartAnswer(ctx, request, item)), nil
}

// chartAnswer answers the data of the chart key names. Every series of one
// chart is reduced with the same period and the widest interval among them,
// so their points share the timestamps.
func chartAnswer(ctx context.Context, request *PageRequest, key string) chartData {
	chart, held := lookupChart(key)
	if !held {
		return chartRefused(textbuf.New().Str("This dashboard has no chart named ").Quoted(key).Str(".").Slice())
	}

	scope, err := dashboardScopeOf(request)
	if err != nil {
		return chartFailed(err)
	}

	now := request.Snapshot.Now
	span := dashboardRange(request).Span(now)

	answers := make([]Series, 0, len(chart.Metrics))
	var interval time.Duration
	points := 0
	for _, metric := range chart.Metrics {
		series, seriesErr := chartSeries(ctx, request, &scope, metric, span)
		if seriesErr != nil {
			return chartFailed(seriesErr)
		}
		answers = append(answers, series)
		interval = max(interval, series.Interval)
		points += len(series.Points)
	}

	data := chartData{
		Unit:       chart.Metrics[0].Unit(),
		Timestamps: []int64{},
		Series:     []chartDataSeries{},
		Refresh:    int(interval / time.Second),
	}
	if points == 0 {
		data.Empty = true
		data.Message = chartNoSamples
		data.Expected = chartExpected(now, interval)

		return data
	}

	for index, series := range answers {
		buckets, reduceErr := Reduce(series.Points, span, interval, dashboardChartBudget)
		if reduceErr != nil {
			return chartFailed(reduceErr)
		}
		if index == 0 {
			for _, bucket := range buckets {
				data.Timestamps = append(data.Timestamps, bucket.At.Unix())
			}
		}
		data.Series = append(data.Series, chartLine(chart.Metrics[index], buckets))
	}

	return data
}

// chartSeries asks the history for one metric over the scope: the device's
// own series on a device's dashboard, the sum over the scope's devices
// otherwise.
func chartSeries(ctx context.Context, request *PageRequest, scope *dashboardScope, metric Metric, span TimeSpan) (Series, error) {
	if scope.device != nil {
		return request.Source.DeviceSeries(ctx, scope.device.Device.Name, metric, span)
	}

	return request.Source.EstateSeries(ctx, scope.names(), metric, span)
}

// chartLine answers one line of a chart from its reduced buckets: the
// average and the maximum of each period, null where the period is a gap.
func chartLine(metric Metric, buckets []Bucket) chartDataSeries {
	line := chartDataSeries{
		Name:    metric.String(),
		Average: make([]*float64, 0, len(buckets)),
		Maximum: make([]*float64, 0, len(buckets)),
	}
	for index := range buckets {
		bucket := &buckets[index]
		if bucket.Gap() {
			line.Average = append(line.Average, nil)
			line.Maximum = append(line.Maximum, nil)
			continue
		}
		average, maximum := bucket.Average, bucket.Maximum
		line.Average = append(line.Average, &average)
		line.Maximum = append(line.Maximum, &maximum)
	}

	return line
}

// chartExpected answers when the first sample is expected: one interval
// from now. A series that names no interval says nothing.
func chartExpected(now time.Time, interval time.Duration) string {
	if interval <= 0 {
		return ""
	}

	return "The first is expected at " + now.Add(interval).UTC().Format(chartClockLayout) + " UTC"
}

// chartClockLayout is the layout of the time a chart expects its first
// sample at.
const chartClockLayout = "15:04"

// chartFailed answers a chart whose data could not be read: no line, and the
// refusal's own words as the message.
func chartFailed(err error) chartData {
	refusal := refusalOf(err)

	return chartRefused(textbuf.New().Str(refusal.Problem()).Byte(' ').Str(refusal.Why()).Slice())
}

// chartRefused answers a chart with no line and message in its place.
func chartRefused(message string) chartData {
	return chartData{Timestamps: []int64{}, Series: []chartDataSeries{}, Empty: true, Message: message}
}
