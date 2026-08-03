package role

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// --- recording metrics registry (mirrors redistribute_egress/metrics_test.go,
// extended to record CounterVec children per label value) ---

type recordingCounter struct{ value atomic.Int64 }

func (r *recordingCounter) Inc()          { r.value.Add(1) }
func (r *recordingCounter) Add(v float64) { r.value.Add(int64(v)) }
func (r *recordingCounter) Get() int64    { return r.value.Load() }

type recordingCounterVec struct {
	reg  *recordingRegistry
	name string
}

func (v recordingCounterVec) With(labelValues ...string) metrics.Counter {
	return v.reg.child(v.name, strings.Join(labelValues, ","))
}
func (recordingCounterVec) Delete(...string) bool { return false }

type recordingRegistry struct {
	mu       sync.Mutex
	counters map[string]*recordingCounter
}

func newRecordingRegistry() *recordingRegistry {
	return &recordingRegistry{counters: map[string]*recordingCounter{}}
}

// child returns the counter for name{labels}, creating it on first use.
func (r *recordingRegistry) child(name, labels string) *recordingCounter {
	key := name + "{" + labels + "}"
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[key]; ok {
		return c
	}
	c := &recordingCounter{}
	r.counters[key] = c
	return c
}

// value reads name{labels}; a never-touched child reads 0.
func (r *recordingRegistry) value(name, labels string) int64 {
	return r.child(name, labels).Get()
}

func (r *recordingRegistry) Counter(name, _ string) metrics.Counter { return r.child(name, "") }
func (r *recordingRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return recordingCounterVec{reg: r, name: name}
}
func (r *recordingRegistry) Gauge(string, string) metrics.Gauge { return nopGauge{} }
func (r *recordingRegistry) GaugeVec(string, string, []string) metrics.GaugeVec {
	return nopGaugeVec{}
}
func (r *recordingRegistry) Histogram(string, string, []float64) metrics.Histogram {
	return nopHistogram{}
}
func (r *recordingRegistry) HistogramVec(string, string, []float64, []string) metrics.HistogramVec {
	return nopHistogramVec{}
}

type nopGauge struct{}

func (nopGauge) Set(float64) {}
func (nopGauge) Inc()        {}
func (nopGauge) Dec()        {}
func (nopGauge) Add(float64) {}

type nopGaugeVec struct{}

func (nopGaugeVec) With(...string) metrics.Gauge { return nopGauge{} }
func (nopGaugeVec) Delete(...string) bool        { return false }

type nopHistogram struct{}

func (nopHistogram) Observe(float64) {}

type nopHistogramVec struct{}

func (nopHistogramVec) With(...string) metrics.Histogram { return nopHistogram{} }
func (nopHistogramVec) Delete(...string) bool            { return false }

// installRecordingMetrics binds a fresh recording registry to the role plugin
// and resets the first-drop warn latches, so each test starts from zero.
func installRecordingMetrics(t *testing.T) *recordingRegistry {
	t.Helper()
	rec := newRecordingRegistry()
	setMetricsRegistry(rec)
	resetDropWarnedForTest()
	t.Cleanup(func() {
		roleMetricsPtr.Store(nil)
		resetDropWarnedForTest()
	})
	return rec
}

// clearFilterState restores the package filter state after a test.
func clearFilterState(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		setFilterState(nil, nil)
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	})
}

// TestRoleDropsAreCounted drives every route-dropping path of the role plugin
// from its real filter entry point and asserts the drop is counted.
//
// The plugin's suppressions and rejects were logged at Debug and nothing else,
// so at the default log level a peer's advertisements could be withdrawn with
// no operator-visible signal at all. Each subtest exercises one drop reason.
//
// VALIDATES: every path where OTCIngressFilter/OTCEgressFilter refuses a route
// increments its reason-labeled counter, so the drop is observable.
// PREVENTS: role-based route suppression being silently invisible to operators
// (ai/rules/evidence.md: a guard must deny AND say something).
func TestRoleDropsAreCounted(t *testing.T) {
	noOTC := buildTestPayload(buildTestAttrs(0), nil)
	withOTC := buildTestPayload(buildTestAttrs(65001), nil)
	// OTC attribute with length 3 instead of 4: malformed per RFC 9234 Section 5.
	malformed := buildTestPayload([]byte{0x40, 0x01, 0x01, 0x00, 0xC0, 35, 3, 0x00, 0x01, 0x02}, nil)

	srcAddr := netip.MustParseAddr("10.0.0.1")
	destAddr := netip.MustParseAddr("10.0.0.5")

	tests := []struct {
		name   string
		metric string
		reason string
		// run performs the drop through the plugin's public filter entry point
		// and returns whether the route was accepted.
		run func(t *testing.T) bool
	}{
		{
			name:   "ingress_leak_from_customer",
			metric: metricRouteRejects,
			reason: reasonLabelLeak,
			run: func(t *testing.T) bool {
				setFilterState(map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}, nil)
				setFilterRemoteRole("10.0.0.1", roleCustomer)
				src := filterapi.PeerFilterInfo{Address: srcAddr, PeerAS: 65001}
				accept, _ := OTCIngressFilter(src, withOTC, map[string]any{})
				return accept
			},
		},
		{
			name:   "ingress_malformed_otc",
			metric: metricRouteRejects,
			reason: reasonLabelMalformedOTC,
			run: func(t *testing.T) bool {
				setFilterState(map[string]*peerRoleConfig{"10.0.0.1": {role: roleCustomer}}, nil)
				setFilterRemoteRole("10.0.0.1", roleProvider)
				src := filterapi.PeerFilterInfo{Address: srcAddr, PeerAS: 65001}
				accept, _ := OTCIngressFilter(src, malformed, map[string]any{})
				return accept
			},
		},
		{
			name:   "egress_wire_bytes_otc_to_provider",
			metric: metricRouteSuppressions,
			reason: reasonLabelOTCPresent,
			run: func(t *testing.T) bool {
				setFilterState(map[string]*peerRoleConfig{
					"10.0.0.1": {role: roleProvider},
					"10.0.0.5": {role: roleCustomer},
				}, nil)
				setFilterRemoteRole("10.0.0.5", roleProvider)
				src := filterapi.PeerFilterInfo{Address: srcAddr, PeerAS: 65001}
				dest := filterapi.PeerFilterInfo{Address: destAddr, PeerAS: 65005, LocalAS: 65000}
				var mods filterapi.ModAccumulator
				return OTCEgressFilter(src, dest, withOTC, map[string]any{}, &mods)
			},
		},
		{
			name:   "egress_source_role_gao_rexford",
			metric: metricRouteSuppressions,
			reason: reasonLabelSourceRole,
			run: func(t *testing.T) bool {
				setFilterState(map[string]*peerRoleConfig{
					"10.0.0.1": {role: roleCustomer},
					"10.0.0.5": {role: roleCustomer},
				}, nil)
				setFilterRemoteRole("10.0.0.5", roleProvider)
				src := filterapi.PeerFilterInfo{Address: srcAddr, PeerAS: 65001}
				dest := filterapi.PeerFilterInfo{Address: destAddr, PeerAS: 65005, LocalAS: 65000}
				var mods filterapi.ModAccumulator
				// src-role customer => the source peer IS a Provider; dest IS a
				// Provider; a route with no OTC must not transit provider->provider.
				meta := map[string]any{"src-role": roleCustomer}
				return OTCEgressFilter(src, dest, noOTC, meta, &mods)
			},
		},
		{
			name:   "egress_export_set_excludes_dest",
			metric: metricRouteSuppressions,
			reason: reasonLabelExportSet,
			run: func(t *testing.T) bool {
				setFilterState(map[string]*peerRoleConfig{
					// export set allows only providers; the destination is a customer.
					"10.0.0.1": {role: roleProvider, export: []string{roleProvider},
						resolvedExport: resolveExport(roleProvider, []string{roleProvider})},
					"10.0.0.5": {role: roleProvider},
				}, nil)
				setFilterRemoteRole("10.0.0.5", roleCustomer)
				src := filterapi.PeerFilterInfo{Address: srcAddr, PeerAS: 65001}
				dest := filterapi.PeerFilterInfo{Address: destAddr, PeerAS: 65005, LocalAS: 65000}
				var mods filterapi.ModAccumulator
				meta := map[string]any{"src-role": roleProvider}
				return OTCEgressFilter(src, dest, noOTC, meta, &mods)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := installRecordingMetrics(t)
			clearFilterState(t)

			accept := tt.run(t)
			assert.False(t, accept, "the route must be dropped for this subtest to prove anything")
			assert.Equal(t, int64(1), rec.value(tt.metric, tt.reason),
				"drop must increment %s{reason=%q}", tt.metric, tt.reason)
		})
	}
}

// TestRoleAcceptedRouteIsNotCounted pins the counters to actual drops: a route
// the plugin accepts must leave every drop counter at zero. Without this, a
// counter wired to the wrong branch would still satisfy the table above.
//
// VALIDATES: the drop counters count drops only, never accepted routes.
// PREVENTS: a miswired counter that increments on the accept path, which would
// make the operator signal meaningless.
func TestRoleAcceptedRouteIsNotCounted(t *testing.T) {
	rec := installRecordingMetrics(t)
	clearFilterState(t)

	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider},
		"10.0.0.5": {role: roleProvider},
	}, nil)
	setFilterRemoteRole("10.0.0.1", roleCustomer)
	setFilterRemoteRole("10.0.0.5", roleCustomer)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.5"), PeerAS: 65005, LocalAS: 65000}
	noOTC := buildTestPayload(buildTestAttrs(0), nil)

	// Ingress from a Customer without OTC: accepted, not stamped, not rejected.
	acceptIn, _ := OTCIngressFilter(src, noOTC, map[string]any{})
	require.True(t, acceptIn)

	// Egress to a Customer: accepted and stamped.
	var mods filterapi.ModAccumulator
	acceptOut := OTCEgressFilter(src, dest, noOTC, map[string]any{"src-role": roleProvider}, &mods)
	require.True(t, acceptOut)

	for _, reason := range []string{
		reasonLabelLeak, reasonLabelMalformedOTC,
		reasonLabelOTCPresent, reasonLabelSourceRole, reasonLabelExportSet,
	} {
		assert.Zero(t, rec.value(metricRouteRejects, reason),
			"accepted route must not increment rejects{reason=%q}", reason)
		assert.Zero(t, rec.value(metricRouteSuppressions, reason),
			"accepted route must not increment suppressions{reason=%q}", reason)
	}
}

// TestRoleFirstDropEmitsWarn proves the second half of the operator signal: a
// counter only helps an operator who already scrapes and alerts on it. The
// first drop of each reason also emits a WARN naming the peer, so "my routes
// vanished" is answerable from the log alone at the default level.
//
// The latch is per reason and per process, so this cannot flood a hot path: the
// per-route detail stays at Debug.
//
// VALIDATES: the first role-driven suppression emits a WARN naming the peer and
// the reason; subsequent identical drops do not repeat it.
// PREVENTS: a per-UPDATE Info/Warn log on the forward hot path, and equally the
// silent-drop failure where nothing is emitted at the default log level.
func TestRoleFirstDropEmitsWarn(t *testing.T) {
	installRecordingMetrics(t)
	clearFilterState(t)

	var buf bytes.Buffer
	prev := logger()
	ConfigureLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { ConfigureLogger(prev); loggerPtr.Store(slogutil.DiscardLogger()) })

	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider},
		"10.0.0.5": {role: roleCustomer},
	}, nil)
	setFilterRemoteRole("10.0.0.5", roleProvider)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.5"), PeerAS: 65005, LocalAS: 65000}
	withOTC := buildTestPayload(buildTestAttrs(65001), nil)

	var mods filterapi.ModAccumulator
	accept := OTCEgressFilter(src, dest, withOTC, map[string]any{}, &mods)
	require.False(t, accept)

	first := buf.String()
	assert.Contains(t, first, "role dropped a route",
		"the first suppression must be visible at the default (WARN) level")
	assert.Contains(t, first, "10.0.0.5", "the WARN must name the destination peer")
	assert.Contains(t, first, reasonLabelOTCPresent, "the WARN must name the reason")

	// A second identical drop must not repeat the WARN: the latch is what keeps
	// this off the hot path.
	buf.Reset()
	accept = OTCEgressFilter(src, dest, withOTC, map[string]any{}, &mods)
	require.False(t, accept)
	assert.Empty(t, buf.String(), "the per-reason WARN latch must fire once, not per route")
}

// TestRoleMetricsSafeBeforeConfigure proves the counters are usable before
// ConfigureMetrics runs. The filters are registered from init() and the reactor
// can call them whether or not a metrics registry was ever bound (telemetry
// disabled, or the plugin loaded without a registry).
//
// VALIDATES: recording a drop with no registry bound is a no-op, not a panic.
// PREVENTS: a nil-pointer dereference on the forward path when telemetry is off.
func TestRoleMetricsSafeBeforeConfigure(t *testing.T) {
	roleMetricsPtr.Store(nil)
	resetDropWarnedForTest()
	t.Cleanup(func() { roleMetricsPtr.Store(nil); resetDropWarnedForTest() })
	clearFilterState(t)

	setFilterState(map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}, nil)
	setFilterRemoteRole("10.0.0.1", roleCustomer)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001}
	withOTC := buildTestPayload(buildTestAttrs(65001), nil)

	assert.NotPanics(t, func() {
		accept, _ := OTCIngressFilter(src, withOTC, map[string]any{})
		assert.False(t, accept)
	})
}
