// Design: docs/architecture/fleet-interface.md -- the sample history questions
// Overview: source.go -- the data source seam
// Related: summary.go -- Reduce, which fits a series to the width of a chart

package fleetui

import (
	"context"
	"time"
)

// The bounds of the history answers.
const (
	// SeriesPointsMax bounds the points of one series: a week of samples
	// taken every thirty seconds.
	SeriesPointsMax = 20160
	// HistoryDevicesMax bounds the devices the history usage answer names.
	HistoryDevicesMax = 10
)

// HistorySource answers from the hub's bounded history of what devices
// report.
type HistorySource interface {
	// DeviceSeries answers the samples of one metric of one device inside
	// span, in time order, at most SeriesPointsMax. A period the device did
	// not report in holds no point: the answer has a gap there.
	// Asked by: the charts of the device dashboard and the side panel of an
	// interface row. Answered by: fleet-8.
	DeviceSeries(ctx context.Context, device string, metric Metric, span TimeSpan) (Series, error)

	// EstateSeries answers one metric summed over the named devices inside
	// span, in time order, at most SeriesPointsMax. The hub sums, so the
	// answer does not grow with the number of devices, which is at most
	// DevicesMax.
	// Asked by: the charts of the estate and group dashboards.
	// Answered by: fleet-8.
	EstateSeries(ctx context.Context, devices []string, metric Metric, span TimeSpan) (Series, error)

	// HistoryUsage answers how much the history holds against its cap.
	// Asked by: the history administration page. Answered by: fleet-8.
	HistoryUsage(ctx context.Context) (HistoryUsage, error)
}

// Metric is one thing a device reports over time. The zero value is not a
// metric.
type Metric uint8

const (
	// MetricUnspecified is the invalid zero value.
	MetricUnspecified Metric = iota
	// MetricCPU is the processor use, in percent.
	MetricCPU
	// MetricMemory is the memory use, in percent.
	MetricMemory
	// MetricStorage is the storage use, in percent.
	MetricStorage
	// MetricLoad is the load average.
	MetricLoad
	// MetricTrafficIn is the traffic received, in bits per second.
	MetricTrafficIn
	// MetricTrafficOut is the traffic sent, in bits per second.
	MetricTrafficOut
	// MetricDevicesConnected is the number of connected devices. It is a
	// series of the estate, never of one device.
	MetricDevicesConnected
	// MetricPeersEstablished is the number of established BGP peers.
	MetricPeersEstablished
)

var (
	metricKeys = []string{
		enumKeyUnspecified, "cpu", "memory", "storage", "load",
		"traffic-in", "traffic-out", "devices-connected", "peers-established",
	}
	metricWords = []string{
		enumWordUnspecified, devicesWordCPU, "Memory", "Storage", "Load",
		"Traffic in", "Traffic out", "Devices connected", "Peers established",
	}
	metricUnits = []string{
		"", "%", "%", "%", "", metricUnitRate, metricUnitRate, "devices", "peers",
	}
)

// String answers the name a chart legend shows.
func (metric Metric) String() string {
	return enumText(metricWords, metric)
}

// Key answers the metric's name in an address.
func (metric Metric) Key() string {
	return enumText(metricKeys, metric)
}

// Unit answers the unit a chart names once on its vertical axis. A metric
// that is a plain number has an empty unit.
func (metric Metric) Unit() string {
	return enumText(metricUnits, metric)
}

// ParseMetric answers the metric an address names, or false.
func ParseMetric(key string) (Metric, bool) {
	return enumParse[Metric](metricKeys, key)
}

// TimeSpan is a period of time, From included and To excluded.
type TimeSpan struct {
	From time.Time
	To   time.Time
}

// Point is one sample: a value at a time.
type Point struct {
	At    time.Time
	Value float64
}

// Series is the samples of one metric over a period.
type Series struct {
	Metric Metric
	// Interval is the time between two samples when none is missing. Two
	// points further apart than that are separated by a gap.
	Interval time.Duration
	Points   []Point
}

// HistoryUsage is how much the hub's history holds.
type HistoryUsage struct {
	// Retention is how long a sample is kept.
	Retention time.Duration
	SizeBytes uint64
	CapBytes  uint64
	// Oldest is when the oldest sample held was taken.
	Oldest time.Time
	// Largest names the devices using most, at most HistoryDevicesMax,
	// largest first.
	Largest []HistoryDeviceUsage
}

// HistoryDeviceUsage is the share of the history one device uses.
type HistoryDeviceUsage struct {
	Device    string
	SizeBytes uint64
}
