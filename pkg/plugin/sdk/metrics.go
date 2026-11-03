// Design: docs/plugin-development/metrics.md -- metrics of a forked plugin
// Related: ../rpc/metrics.go -- the metrics-publish wire types and bounds
// Related: sdk.go -- Run starts and stops the publish worker

package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The plugin's publish cadence: the default, and the bounds SetMetricsInterval
// accepts.
const (
	MetricsIntervalDefault = 5 * time.Second
	MetricsIntervalMin     = 100 * time.Millisecond
	MetricsIntervalMax     = time.Hour
	metricsByeFlushMax     = 2 * time.Second
)

// MetricsPublisher is the metrics.Registry a plugin that runs in its own
// process registers its metrics with. Each metric is created in the engine's
// registry under the same name and shape it would have in-process, and the
// plugin decides how often the engine learns of changes:
//
//   - a declaration is published at once, so the name exists in the engine as
//     soon as the plugin is running;
//   - updates accumulate here and are published every interval
//     (SetMetricsInterval, MetricsIntervalDefault otherwise), when the plugin
//     calls Plugin.FlushMetrics, and when the engine removes the plugin (bye);
//   - a publish the engine did not take (an error, or no registry yet) keeps
//     its changes for the next one, so no increment is lost.
//
// What is held is bounded: rpc.MetricsDeclaredMax names and
// rpc.MetricsSeriesMax labeled series. A counter accumulates one float, a
// gauge holds its value, and a histogram holds one count and one sum per
// bucket, whatever the observation rate.
//
// Lifecycle: NewMetricsPublisher, then Plugin.Run starts the publish worker
// after the startup handshake and stops it before it returns. Safe for
// concurrent use.
type MetricsPublisher struct {
	mu        sync.Mutex
	families  []*metricFamily // declared order; refused names are not here
	byName    map[string]*metricFamily
	series    int // labeled series held across the vectors
	interval  time.Duration
	declared  chan struct{} // a new name waits to be published
	full      chan struct{} // a histogram bucket nears rpc.MetricBucketCountMax
	newTicker func(time.Duration) (<-chan time.Time, func())

	// publishMu serializes publishes, so a gauge value never overtakes a
	// later one on its way to the engine.
	publishMu sync.Mutex
}

type metricFamily struct {
	decl     rpc.MetricDeclaration
	declared bool   // the engine applied the declaration
	refused  string // why the name is not forwarded; empty while it is
	warned   bool   // the one WARN line for a value the wire cannot carry
	series   map[string]*metricSeries
	deletes  [][]string
}

type metricSeries struct {
	values  []string
	dirty   bool
	deleted bool      // removed by Delete: a held handle updates nothing
	value   float64   // counter: added since the last publish; gauge: current value
	counts  []uint64  // histogram: observations per bucket since the last publish
	sums    []float64 // histogram: their sum per bucket
}

// NewMetricsPublisher returns an empty publisher with the default interval.
func NewMetricsPublisher() *MetricsPublisher {
	return &MetricsPublisher{
		byName:    make(map[string]*metricFamily),
		interval:  MetricsIntervalDefault,
		declared:  make(chan struct{}, 1),
		full:      make(chan struct{}, 1),
		newTicker: realTicker,
	}
}

func realTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// SetInterval sets how often accumulated updates are published. It is read
// when the publish worker starts, so it MUST be called before Plugin.Run.
func (m *MetricsPublisher) SetInterval(d time.Duration) error {
	if d < MetricsIntervalMin {
		return fmt.Errorf("metrics interval %s is below the minimum of %s", d, MetricsIntervalMin)
	}
	if d > MetricsIntervalMax {
		return fmt.Errorf("metrics interval %s is above the maximum of %s", d, MetricsIntervalMax)
	}
	m.mu.Lock()
	m.interval = d
	m.mu.Unlock()
	return nil
}

// metricsConn carries a publisher to NewWithConn, as rpc.BridgedConn carries
// a DirectBridge.
type metricsConn struct {
	net.Conn
	publisher *MetricsPublisher
}

// ConnWithMetrics returns conn carrying publisher: the plugin NewWithConn
// builds on it publishes publisher's metrics. The runner of a forked plugin
// uses it to hand the registry it gave ConfigureMetrics to the SDK.
func ConnWithMetrics(conn net.Conn, publisher *MetricsPublisher) net.Conn {
	return metricsConn{Conn: conn, publisher: publisher}
}

// MetricsRegistry returns the registry whose metrics this plugin publishes to
// the engine, creating it on first call. Create the metrics before Run.
func (p *Plugin) MetricsRegistry() metrics.Registry {
	return p.metricsPublisher()
}

// SetMetricsInterval sets how often the engine learns of metric updates. It
// MUST be called before Run.
func (p *Plugin) SetMetricsInterval(d time.Duration) error {
	return p.metricsPublisher().SetInterval(d)
}

// FlushMetrics publishes every accumulated update now.
func (p *Plugin) FlushMetrics(ctx context.Context) error {
	return p.metricsPublisher().publish(ctx, p.callEngineWithResult, true)
}

func (p *Plugin) metricsPublisher() *MetricsPublisher {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.metrics == nil {
		p.metrics = NewMetricsPublisher()
	}
	return p.metrics
}

// startMetrics starts the publish worker when the plugin has a publisher. The
// returned stop MUST be called before Run returns; it waits for the worker.
func (p *Plugin) startMetrics(ctx context.Context) (stop func()) {
	p.mu.Lock()
	publisher := p.metrics
	p.mu.Unlock()
	if publisher == nil {
		return func() {}
	}
	quit := make(chan struct{})
	done := make(chan struct{})
	go publisher.runWorker(ctx, quit, done, p.callEngineWithResult)
	return func() {
		close(quit)
		<-done
	}
}

// flushMetricsAtBye publishes the last updates before the plugin acknowledges
// its removal; after the acknowledgement the engine closes the connection.
func (p *Plugin) flushMetricsAtBye(ctx context.Context) {
	p.mu.Lock()
	publisher := p.metrics
	p.mu.Unlock()
	if publisher == nil {
		return
	}
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metricsByeFlushMax)
	defer cancel()
	if err := publisher.publish(flushCtx, p.callEngineWithResult, true); err != nil {
		slog.Warn("plugin metrics: the last updates before removal did not reach the engine", "plugin", p.name, "error", err)
	}
}

type engineCall func(ctx context.Context, method string, params any) (json.RawMessage, error)

// runWorker publishes until quit closes or ctx ends, then closes done. It is
// one long-lived goroutine per plugin, started by startMetrics.
func (m *MetricsPublisher) runWorker(ctx context.Context, quit <-chan struct{}, done chan<- struct{}, call engineCall) {
	defer close(done)

	m.mu.Lock()
	interval := m.interval
	m.mu.Unlock()
	tick, stopTick := m.newTicker(interval)
	defer stopTick()

	// Names declared before the handshake ended are published at once.
	m.publishLogged(ctx, call, false)
	for {
		select {
		case <-quit:
			return
		case <-ctx.Done():
			return
		case <-m.declared:
			m.publishLogged(ctx, call, false)
		case <-m.full:
			m.publishLogged(ctx, call, true)
		case <-tick:
			m.publishLogged(ctx, call, true)
		}
	}
}

func (m *MetricsPublisher) publishLogged(ctx context.Context, call engineCall, withUpdates bool) {
	if err := m.publish(ctx, call, withUpdates); err != nil {
		slog.Warn("plugin metrics: publish to the engine failed; the updates are kept for the next one", "error", err)
	}
}

// sentItem remembers what one publish took from a series, so a publish the
// engine did not take can give it back.
type sentItem struct {
	family *metricFamily
	series *metricSeries
	update rpc.MetricUpdate
}

type sentPublish struct {
	declared []*metricFamily
	deletes  map[*metricFamily][][]string
	updates  []sentItem
}

// publish sends the pending declarations, deletions and, when withUpdates,
// the accumulated updates, in one metrics-publish.
func (m *MetricsPublisher) publish(ctx context.Context, call engineCall, withUpdates bool) error {
	m.publishMu.Lock()
	defer m.publishMu.Unlock()

	in, sent := m.collect(withUpdates)
	if len(in.Declare)+len(in.Delete)+len(in.Update) == 0 {
		return nil
	}
	raw, err := call(ctx, rpc.MethodMetricsPublish, in)
	if err != nil {
		m.restore(sent)
		return err
	}
	var out rpc.MetricsPublishOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		m.restore(sent)
		return fmt.Errorf("decode metrics-publish answer: %w", err)
	}
	if out.Unavailable {
		m.restore(sent)
		return nil
	}
	m.settle(sent, out.Refused)
	return nil
}

func (m *MetricsPublisher) collect(withUpdates bool) (*rpc.MetricsPublishInput, *sentPublish) {
	m.mu.Lock()
	defer m.mu.Unlock()

	in := &rpc.MetricsPublishInput{}
	sent := &sentPublish{deletes: make(map[*metricFamily][][]string)}
	for _, f := range m.families {
		if f.refused != "" {
			continue
		}
		if !f.declared {
			in.Declare = append(in.Declare, f.decl)
			sent.declared = append(sent.declared, f)
		}
		if !withUpdates {
			continue
		}
		for _, values := range f.deletes {
			in.Delete = append(in.Delete, rpc.MetricSeries{Name: f.decl.Name, Values: values})
		}
		if len(f.deletes) > 0 {
			sent.deletes[f] = f.deletes
			f.deletes = nil
		}
		for _, s := range f.series {
			if !s.dirty {
				continue
			}
			u := takeUpdate(f, s)
			in.Update = append(in.Update, u)
			sent.updates = append(sent.updates, sentItem{family: f, series: s, update: u})
		}
	}
	return in, sent
}

// takeUpdate moves a series' pending change into an update. A bucket above
// rpc.MetricBucketCountMax sends that many and keeps the rest pending, at
// the same mean, so the series stays dirty.
func takeUpdate(f *metricFamily, s *metricSeries) rpc.MetricUpdate {
	u := rpc.MetricUpdate{Name: f.decl.Name, Values: s.values}
	s.dirty = false
	switch f.decl.Kind {
	case rpc.MetricCounter, rpc.MetricCounterVec:
		u.Value = s.value
		s.value = 0
	case rpc.MetricGauge, rpc.MetricGaugeVec:
		u.Value = s.value
	case rpc.MetricHistogram, rpc.MetricHistogramVec:
		u.Counts = make([]uint64, len(s.counts))
		u.Sums = make([]float64, len(s.sums))
		for i, count := range s.counts {
			take := min(count, rpc.MetricBucketCountMax)
			sum := s.sums[i]
			if take < count {
				sum = s.sums[i] / float64(count) * float64(take)
				s.dirty = true
			}
			u.Counts[i], u.Sums[i] = take, sum
			s.counts[i] -= take
			s.sums[i] -= sum
		}
	}
	return u
}

// restore gives back what a publish took, for the next one.
func (m *MetricsPublisher) restore(sent *sentPublish) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for f, deletes := range sent.deletes {
		f.deletes = append(deletes, f.deletes...)
	}
	for _, item := range sent.updates {
		s := item.series
		if s.deleted {
			continue
		}
		s.dirty = true
		switch item.family.decl.Kind {
		case rpc.MetricCounter, rpc.MetricCounterVec:
			s.value += item.update.Value
		case rpc.MetricGauge, rpc.MetricGaugeVec:
			// The held value is already the newest.
		case rpc.MetricHistogram, rpc.MetricHistogramVec:
			for i := range s.counts {
				s.counts[i] += item.update.Counts[i]
				s.sums[i] += item.update.Sums[i]
			}
		}
	}
}

// settle records what the engine applied and logs what it refused. A refused
// declaration stops the name from being forwarded again.
func (m *MetricsPublisher) settle(sent *sentPublish, refused []rpc.MetricRefusal) {
	m.mu.Lock()
	defer m.mu.Unlock()

	refusedNames := make(map[string]string, len(refused))
	for _, r := range refused {
		slog.Warn("plugin metrics: the engine refused a metric", "item", r.Item, "metric", r.Name, "values", r.Values, "reason", r.Reason)
		if r.Item == "declare" {
			refusedNames[r.Name] = r.Reason
		}
	}
	for _, f := range sent.declared {
		if reason, ok := refusedNames[f.decl.Name]; ok {
			f.refused = reason
			continue
		}
		f.declared = true
	}
}

// wakeMetrics wakes the publish worker; a wake already pending is enough.
func wakeMetrics(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// family returns the named metric, declaring it on first use. A name already
// declared with the same kind is shared, as PrometheusRegistry shares it; a
// different kind is the panic Prometheus raises for a second collector.
func (m *MetricsPublisher) family(d rpc.MetricDeclaration) *metricFamily {
	m.mu.Lock()
	defer m.mu.Unlock()

	if f, ok := m.byName[d.Name]; ok {
		if f.decl.Kind != d.Kind {
			panic("BUG: metric " + d.Name + " declared as " + string(d.Kind) + " after " + string(f.decl.Kind))
		}
		return f
	}
	f := &metricFamily{decl: d, series: make(map[string]*metricSeries)}
	m.byName[d.Name] = f
	if err := rpc.ValidateMetricDeclaration(&d); err != nil {
		f.refused = err.Error()
	}
	if f.refused == "" && len(m.families) >= rpc.MetricsDeclaredMax {
		f.refused = "the plugin declares more than " + strconv.Itoa(rpc.MetricsDeclaredMax) + " metrics"
	}
	if f.refused != "" {
		slog.Warn("plugin metrics: metric not forwarded to the engine", "metric", d.Name, "reason", f.refused)
		return f
	}
	m.families = append(m.families, f)
	wakeMetrics(m.declared)
	return f
}

// seriesFor returns the series of values, creating it. A vector holds at most
// rpc.MetricsSeriesMax series across the plugin; past it the series is
// detached, updates nothing, and one WARN line names the metric.
func (m *MetricsPublisher) seriesFor(f *metricFamily, values []string) *metricSeries {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(values) != len(f.decl.Labels) {
		panic("BUG: metric " + f.decl.Name + " given " + strconv.Itoa(len(values)) + " label values for " + strconv.Itoa(len(f.decl.Labels)) + " labels")
	}
	key := strings.Join(values, "\xff")
	if s, ok := f.series[key]; ok {
		return s
	}
	s := &metricSeries{values: slices.Clone(values)}
	if f.decl.Kind.IsHistogram() {
		s.counts = make([]uint64, len(f.decl.Buckets)+1)
		s.sums = make([]float64, len(f.decl.Buckets)+1)
	}
	if f.refused != "" {
		s.deleted = true
		return s
	}
	if len(values) > 0 {
		if m.series >= rpc.MetricsSeriesMax {
			m.warnOnce(f, "the plugin holds "+strconv.Itoa(rpc.MetricsSeriesMax)+" labeled series, the limit")
			s.deleted = true
			return s
		}
		m.series++
	}
	// A series exists from its creation, at zero, as With creates it in-process.
	s.dirty = true
	f.series[key] = s
	return s
}

func (m *MetricsPublisher) deleteSeries(f *metricFamily, values []string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := strings.Join(values, "\xff")
	s, ok := f.series[key]
	if !ok {
		return false
	}
	s.deleted = true
	delete(f.series, key)
	m.series--
	f.deletes = append(f.deletes, slices.Clone(values))
	return true
}

// warnOnce MUST be called with m.mu held.
func (m *MetricsPublisher) warnOnce(f *metricFamily, reason string) {
	if f.warned {
		return
	}
	f.warned = true
	slog.Warn("plugin metrics: update not forwarded to the engine", "metric", f.decl.Name, "reason", reason)
}

// update applies fn to s under the lock, unless s was deleted or the value
// cannot travel: JSON carries no NaN or infinity.
func (m *MetricsPublisher) update(f *metricFamily, s *metricSeries, value float64, fn func(*metricSeries)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s.deleted {
		return
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		m.warnOnce(f, "a value that is not finite cannot be published")
		return
	}
	fn(s)
	s.dirty = true
}

// Counter implements metrics.Registry.
func (m *MetricsPublisher) Counter(name, help string) metrics.Counter {
	f := m.family(rpc.MetricDeclaration{Name: name, Help: help, Kind: rpc.MetricCounter})
	return &publishedCounter{m: m, f: f, s: m.seriesFor(f, nil)}
}

// Gauge implements metrics.Registry.
func (m *MetricsPublisher) Gauge(name, help string) metrics.Gauge {
	f := m.family(rpc.MetricDeclaration{Name: name, Help: help, Kind: rpc.MetricGauge})
	return &publishedGauge{m: m, f: f, s: m.seriesFor(f, nil)}
}

// Histogram implements metrics.Registry.
func (m *MetricsPublisher) Histogram(name, help string, buckets []float64) metrics.Histogram {
	f := m.family(rpc.MetricDeclaration{Name: name, Help: help, Kind: rpc.MetricHistogram, Buckets: metrics.ResolveBuckets(buckets)})
	return &publishedHistogram{m: m, f: f, s: m.seriesFor(f, nil)}
}

// CounterVec implements metrics.Registry.
func (m *MetricsPublisher) CounterVec(name, help string, labelNames []string) metrics.CounterVec {
	f := m.family(rpc.MetricDeclaration{Name: name, Help: help, Kind: rpc.MetricCounterVec, Labels: slices.Clone(labelNames)})
	return &publishedVec{m: m, f: f}
}

// GaugeVec implements metrics.Registry.
func (m *MetricsPublisher) GaugeVec(name, help string, labelNames []string) metrics.GaugeVec {
	f := m.family(rpc.MetricDeclaration{Name: name, Help: help, Kind: rpc.MetricGaugeVec, Labels: slices.Clone(labelNames)})
	return &publishedGaugeVec{publishedVec{m: m, f: f}}
}

// HistogramVec implements metrics.Registry.
func (m *MetricsPublisher) HistogramVec(name, help string, buckets []float64, labelNames []string) metrics.HistogramVec {
	f := m.family(rpc.MetricDeclaration{Name: name, Help: help, Kind: rpc.MetricHistogramVec, Labels: slices.Clone(labelNames), Buckets: metrics.ResolveBuckets(buckets)})
	return &publishedHistogramVec{publishedVec{m: m, f: f}}
}

var _ metrics.Registry = (*MetricsPublisher)(nil)

type publishedCounter struct {
	m *MetricsPublisher
	f *metricFamily
	s *metricSeries
}

func (c *publishedCounter) Inc() { c.Add(1) }

func (c *publishedCounter) Add(v float64) {
	if v < 0 {
		panic("BUG: counter " + c.f.decl.Name + " cannot decrease")
	}
	c.m.update(c.f, c.s, v, func(s *metricSeries) { s.value += v })
}

type publishedGauge struct {
	m *MetricsPublisher
	f *metricFamily
	s *metricSeries
}

func (g *publishedGauge) Set(v float64) {
	g.m.update(g.f, g.s, v, func(s *metricSeries) { s.value = v })
}
func (g *publishedGauge) Inc() { g.Add(1) }
func (g *publishedGauge) Dec() { g.Add(-1) }
func (g *publishedGauge) Add(v float64) {
	g.m.update(g.f, g.s, v, func(s *metricSeries) { s.value += v })
}

type publishedHistogram struct {
	m *MetricsPublisher
	f *metricFamily
	s *metricSeries
}

// Observe counts v in the first bucket whose upper bound is at least v, as
// Prometheus does, and wakes an early publish when that bucket nears the
// most one update may carry.
func (h *publishedHistogram) Observe(v float64) {
	i := sort.SearchFloat64s(h.f.decl.Buckets, v)
	near := false
	h.m.update(h.f, h.s, v, func(s *metricSeries) {
		s.counts[i]++
		s.sums[i] += v
		near = s.counts[i] >= rpc.MetricBucketCountMax/2
	})
	if near {
		wakeMetrics(h.m.full)
	}
}

type publishedVec struct {
	m *MetricsPublisher
	f *metricFamily
}

func (v *publishedVec) With(labelValues ...string) metrics.Counter {
	return &publishedCounter{m: v.m, f: v.f, s: v.m.seriesFor(v.f, labelValues)}
}

func (v *publishedVec) Delete(labelValues ...string) bool { return v.m.deleteSeries(v.f, labelValues) }

type publishedGaugeVec struct{ publishedVec }

func (v *publishedGaugeVec) With(labelValues ...string) metrics.Gauge {
	return &publishedGauge{m: v.m, f: v.f, s: v.m.seriesFor(v.f, labelValues)}
}

type publishedHistogramVec struct{ publishedVec }

func (v *publishedHistogramVec) With(labelValues ...string) metrics.Histogram {
	return &publishedHistogram{m: v.m, f: v.f, s: v.m.seriesFor(v.f, labelValues)}
}
