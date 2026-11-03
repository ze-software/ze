// Design: docs/architecture/api/process-protocol.md -- metrics a plugin publishes
// Related: dispatch_registry.go -- the engineOps entry for metrics-publish
// Related: ../../../../pkg/plugin/sdk/metrics.go -- the plugin side that accumulates and publishes

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// metricsPublishEncodedMax bounds one encoded publish before it is decoded.
const metricsPublishEncodedMax = 16 << 20

// Refusal items, as rpc.MetricRefusal.Item names them.
const (
	metricItemDeclare = "declare"
	metricItemDelete  = "delete"
	metricItemUpdate  = "update"
)

// pluginMetricSet holds the metrics plugins publish over the protocol, each
// created in the engine's registry under the name and shape the plugin
// declared, which is the metric the plugin would create in-process.
//
// It knows no plugin and no metric: names, shapes and owners all arrive in
// declarations. A plugin may update only names it declared itself. A name
// declared again with the same shape, by a plugin that restarted or by
// another plugin, shares the metric, as PrometheusRegistry shares it; a
// different shape is refused, because Prometheus panics on one. The metrics
// outlive the process that declared them, as an in-process plugin's do.
//
// Bounded per plugin: rpc.MetricsDeclaredMax names and rpc.MetricsSeriesMax
// labeled series. Safe for concurrent use; the zero value is ready.
type pluginMetricSet struct {
	mu     sync.Mutex
	reg    metrics.Registry
	byName map[string]*forwardedMetric
	names  map[string]map[string]bool // plugin -> names it declared
	series map[string]map[string]bool // plugin -> labeled series it holds
}

// forwardedMetric is one published metric and its handle in the current
// registry (nil until created there).
type forwardedMetric struct {
	decl         rpc.MetricDeclaration
	counter      metrics.Counter
	gauge        metrics.Gauge
	histogram    metrics.Histogram
	counterVec   metrics.CounterVec
	gaugeVec     metrics.GaugeVec
	histogramVec metrics.HistogramVec
}

func (m *forwardedMetric) created() bool {
	return m.counter != nil || m.gauge != nil || m.histogram != nil ||
		m.counterVec != nil || m.gaugeVec != nil || m.histogramVec != nil
}

func (m *forwardedMetric) forget() {
	*m = forwardedMetric{decl: m.decl}
}

// setRegistry points the set at the engine's registry. A new registry drops
// the handles of the old one; each metric is created again on its next use.
func (set *pluginMetricSet) setRegistry(reg metrics.Registry) {
	set.mu.Lock()
	defer set.mu.Unlock()

	if set.reg == reg {
		return
	}
	set.reg = reg
	for _, m := range set.byName {
		m.forget()
	}
}

func (s *Server) opMetricsPublish(_ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	if len(params) > metricsPublishEncodedMax {
		return nil, fmt.Errorf("metrics publish is %d octets, above the limit of %d; publish more often", len(params), metricsPublishEncodedMax)
	}
	var in rpc.MetricsPublishInput
	if err := json.Unmarshal(params, &in); err != nil {
		return nil, fmt.Errorf("decode metrics publish: %w", err)
	}
	if err := rpc.ValidateMetricsPublish(&in); err != nil {
		return nil, err
	}
	if proc == nil {
		return nil, errors.New("metrics publish has no plugin owner")
	}
	out, apply := s.pluginMetrics.publish(proc.Name(), &in)
	// Applied outside the lock: a histogram update replays its observations.
	for _, fn := range apply {
		fn()
	}
	return out, nil
}

// publish judges every item of in for owner and returns the answer and the
// updates to apply, in order: declarations, then deletions, then updates.
func (set *pluginMetricSet) publish(owner string, in *rpc.MetricsPublishInput) (rpc.MetricsPublishOutput, []func()) {
	set.mu.Lock()
	defer set.mu.Unlock()

	var out rpc.MetricsPublishOutput
	if set.reg == nil {
		out.Unavailable = true
		return out, nil
	}
	if set.byName == nil {
		set.byName = make(map[string]*forwardedMetric)
		set.names = make(map[string]map[string]bool)
		set.series = make(map[string]map[string]bool)
	}
	refuse := func(item, name string, values []string, reason string) {
		out.Refused = append(out.Refused, rpc.MetricRefusal{Item: item, Name: name, Values: values, Reason: reason})
	}
	for i := range in.Declare {
		if reason := set.declare(owner, &in.Declare[i]); reason != "" {
			refuse(metricItemDeclare, in.Declare[i].Name, nil, reason)
		}
	}
	var apply []func()
	for _, d := range in.Delete {
		fn, reason := set.deletion(owner, d)
		if reason != "" {
			refuse(metricItemDelete, d.Name, d.Values, reason)
			continue
		}
		apply = append(apply, fn)
	}
	for i := range in.Update {
		fn, reason := set.update(owner, &in.Update[i])
		if reason != "" {
			refuse(metricItemUpdate, in.Update[i].Name, in.Update[i].Values, reason)
			continue
		}
		apply = append(apply, fn)
	}
	return out, apply
}

// declare MUST be called with set.mu held. It answers the refusal reason, or
// "" once the metric exists for owner.
func (set *pluginMetricSet) declare(owner string, d *rpc.MetricDeclaration) string {
	owned := set.names[owner]
	if owned == nil {
		owned = make(map[string]bool)
		set.names[owner] = owned
	}
	if !owned[d.Name] && len(owned) >= rpc.MetricsDeclaredMax {
		return "plugin " + owner + " already declares " + fmt.Sprint(rpc.MetricsDeclaredMax) + " metrics, the limit"
	}
	m := set.byName[d.Name]
	if m == nil {
		m = &forwardedMetric{decl: *d}
	}
	if !shapeOf(&m.decl).Equal(shapeOf(d)) {
		return "declared before as " + describeShape(shapeOf(&m.decl)) + ", not " + describeShape(shapeOf(d))
	}
	if reason := set.create(m); reason != "" {
		return reason
	}
	set.byName[d.Name] = m
	owned[d.Name] = true
	return ""
}

// create MUST be called with set.mu held. It makes m's handle in the current
// registry, refusing a name the registry holds with another shape.
func (set *pluginMetricSet) create(m *forwardedMetric) string {
	if m.created() {
		return ""
	}
	reader, ok := set.reg.(metrics.ShapeReader)
	if !ok {
		return "the engine's metrics registry cannot report what a name already means, so no plugin metric is created in it"
	}
	want := shapeOf(&m.decl)
	if have, exists := reader.Shape(m.decl.Name); exists && !have.Equal(want) {
		return "the engine already has this name as " + describeShape(have) + ", not " + describeShape(want)
	}
	d := &m.decl
	switch d.Kind {
	case rpc.MetricCounter:
		m.counter = set.reg.Counter(d.Name, d.Help)
	case rpc.MetricGauge:
		m.gauge = set.reg.Gauge(d.Name, d.Help)
	case rpc.MetricHistogram:
		m.histogram = set.reg.Histogram(d.Name, d.Help, d.Buckets)
	case rpc.MetricCounterVec:
		m.counterVec = set.reg.CounterVec(d.Name, d.Help, d.Labels)
	case rpc.MetricGaugeVec:
		m.gaugeVec = set.reg.GaugeVec(d.Name, d.Help, d.Labels)
	case rpc.MetricHistogramVec:
		m.histogramVec = set.reg.HistogramVec(d.Name, d.Help, d.Buckets, d.Labels)
	default: // open set: the kind arrived over the wire
		return "unknown kind " + string(d.Kind)
	}
	return ""
}

// owned MUST be called with set.mu held. It returns owner's metric of name,
// created in the current registry, with values matching its labels.
func (set *pluginMetricSet) owned(owner, name string, values []string) (*forwardedMetric, string) {
	if !set.names[owner][name] {
		return nil, "plugin " + owner + " has not declared this metric"
	}
	m := set.byName[name]
	if reason := set.create(m); reason != "" {
		return nil, reason
	}
	if len(values) != len(m.decl.Labels) {
		return nil, "carries " + fmt.Sprint(len(values)) + " label values for " + fmt.Sprint(len(m.decl.Labels)) + " labels"
	}
	return m, ""
}

func seriesKey(name string, values []string) string {
	return name + "\xff" + strings.Join(values, "\xff")
}

// deletion MUST be called with set.mu held.
func (set *pluginMetricSet) deletion(owner string, d rpc.MetricSeries) (func(), string) {
	m, reason := set.owned(owner, d.Name, d.Values)
	if reason != "" {
		return nil, reason
	}
	if !m.decl.Kind.IsVector() {
		return nil, "a " + string(m.decl.Kind) + " has no series to delete"
	}
	delete(set.series[owner], seriesKey(d.Name, d.Values))
	values := d.Values
	switch m.decl.Kind {
	case rpc.MetricCounterVec:
		return func() { m.counterVec.Delete(values...) }, ""
	case rpc.MetricGaugeVec:
		return func() { m.gaugeVec.Delete(values...) }, ""
	case rpc.MetricHistogramVec:
		return func() { m.histogramVec.Delete(values...) }, ""
	case rpc.MetricCounter, rpc.MetricGauge, rpc.MetricHistogram:
	}
	return nil, "a " + string(m.decl.Kind) + " has no series to delete"
}

// update MUST be called with set.mu held.
func (set *pluginMetricSet) update(owner string, u *rpc.MetricUpdate) (func(), string) {
	m, reason := set.owned(owner, u.Name, u.Values)
	if reason != "" {
		return nil, reason
	}
	if m.decl.Kind.IsVector() {
		held := set.series[owner]
		if held == nil {
			held = make(map[string]bool)
			set.series[owner] = held
		}
		key := seriesKey(u.Name, u.Values)
		if !held[key] && len(held) >= rpc.MetricsSeriesMax {
			return nil, "plugin " + owner + " already holds " + fmt.Sprint(rpc.MetricsSeriesMax) + " labeled series, the limit"
		}
		held[key] = true
	}
	if m.decl.Kind.IsHistogram() {
		if len(u.Counts) != len(m.decl.Buckets)+1 {
			return nil, "carries " + fmt.Sprint(len(u.Counts)) + " bucket counts for " + fmt.Sprint(len(m.decl.Buckets)+1) + " buckets"
		}
	}
	values, value := u.Values, u.Value
	counts, sums, buckets := u.Counts, u.Sums, m.decl.Buckets
	switch m.decl.Kind {
	case rpc.MetricCounter, rpc.MetricCounterVec:
		if value < 0 {
			return nil, "a counter cannot decrease"
		}
		if m.counter != nil {
			return func() { m.counter.Add(value) }, ""
		}
		return func() { m.counterVec.With(values...).Add(value) }, ""
	case rpc.MetricGauge:
		return func() { m.gauge.Set(value) }, ""
	case rpc.MetricGaugeVec:
		return func() { m.gaugeVec.With(values...).Set(value) }, ""
	case rpc.MetricHistogram:
		return func() { replayBuckets(m.histogram, buckets, counts, sums) }, ""
	case rpc.MetricHistogramVec:
		return func() { replayBuckets(m.histogramVec.With(values...), buckets, counts, sums) }, ""
	}
	return nil, "unknown kind " + string(m.decl.Kind)
}

// replayBuckets adds a plugin's per-bucket counts and sums to h. Each bucket's
// observations are replayed at their mean, which lies inside the bucket, so
// the engine's bucket counts, count and sum equal the plugin's (the sum to
// floating-point rounding).
func replayBuckets(h metrics.Histogram, buckets []float64, counts []uint64, sums []float64) {
	for i, count := range counts {
		if count == 0 {
			continue
		}
		mean := sums[i] / float64(count)
		if i < len(buckets) && mean > buckets[i] {
			mean = buckets[i]
		}
		if i > 0 && mean <= buckets[i-1] {
			mean = math.Nextafter(buckets[i-1], math.Inf(1))
		}
		for range count {
			h.Observe(mean)
		}
	}
}

func shapeOf(d *rpc.MetricDeclaration) metrics.Shape {
	shape := metrics.Shape{LabelNames: d.Labels, Buckets: d.Buckets}
	switch d.Kind {
	case rpc.MetricCounter:
		shape.Kind = metrics.KindCounter
	case rpc.MetricGauge:
		shape.Kind = metrics.KindGauge
	case rpc.MetricHistogram:
		shape.Kind = metrics.KindHistogram
	case rpc.MetricCounterVec:
		shape.Kind = metrics.KindCounterVec
	case rpc.MetricGaugeVec:
		shape.Kind = metrics.KindGaugeVec
	case rpc.MetricHistogramVec:
		shape.Kind = metrics.KindHistogramVec
	default: // open set: KindUnspecified equals no created shape
		shape.Kind = metrics.KindUnspecified
	}
	return shape
}

func describeShape(shape metrics.Shape) string {
	return shape.Kind.String() + " labels [" + strings.Join(shape.LabelNames, " ") + "] buckets " + fmt.Sprint(shape.Buckets)
}
