package filterapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

// fakeCounter records Inc calls.
type fakeCounter struct{ n int }

func (c *fakeCounter) Inc()          { c.n++ }
func (c *fakeCounter) Add(_ float64) { c.n++ }

// fakeCounterVec hands out one fakeCounter per label-value tuple and remembers
// the label values it was asked for, so a test can assert WHICH series moved.
type fakeCounterVec struct {
	counters map[string]*fakeCounter
}

func (v *fakeCounterVec) With(labelValues ...string) metrics.Counter {
	key := ""
	for i, lv := range labelValues {
		if i > 0 {
			key += "|"
		}
		key += lv
	}
	if v.counters == nil {
		v.counters = map[string]*fakeCounter{}
	}
	c, ok := v.counters[key]
	if !ok {
		c = &fakeCounter{}
		v.counters[key] = c
	}
	return c
}

func (v *fakeCounterVec) Delete(_ ...string) bool { return false }

// fakeRegistry implements metrics.Registry, returning the one CounterVec this
// package creates and no-ops for everything else.
type fakeRegistry struct {
	vec   *fakeCounterVec
	names []string
}

func (r *fakeRegistry) Counter(_, _ string) metrics.Counter { return &fakeCounter{} }
func (r *fakeRegistry) Gauge(_, _ string) metrics.Gauge     { return metrics.NopRegistry{}.Gauge("", "") }
func (r *fakeRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	r.names = append(r.names, name)
	if r.vec == nil {
		r.vec = &fakeCounterVec{}
	}
	return r.vec
}

func (r *fakeRegistry) GaugeVec(_, _ string, _ []string) metrics.GaugeVec {
	return metrics.NopRegistry{}.GaugeVec("", "", nil)
}

func (r *fakeRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram {
	return metrics.NopRegistry{}.Histogram("", "", nil)
}

func (r *fakeRegistry) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return metrics.NopRegistry{}.HistogramVec("", "", nil, nil)
}

// resetAttrModMetrics puts the package back to its unwired state so tests do not
// leak a registry into each other.
func resetAttrModMetrics(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { attrModMetricsPtr.Store(nil) })
	attrModMetricsPtr.Store(nil)
}

// TestRecordRemoveBufferRefusedIsSilentWithoutARegistry verifies the recorder is
// safe before any registry is wired.
//
// VALIDATES: spec-fixit-rs-community-strip-arity R-2 -- the counter must not
// make a no-telemetry build worse than no counter at all.
// PREVENTS: a nil-pointer panic on the forwarding path in every build that does
// not enable metrics, which is the default.
func TestRecordRemoveBufferRefusedIsSilentWithoutARegistry(t *testing.T) {
	resetAttrModMetrics(t)

	assert.NotPanics(t, func() { RecordRemoveBufferRefused(8) })
}

// TestSetMetricsRegistryNilIsANoOp verifies a nil registry leaves the recorder
// disabled rather than storing a nil-bearing struct.
//
// VALIDATES: the reactor calls this unconditionally from its metrics block; a
// nil registry must degrade to "disabled", not to a panic on first use.
func TestSetMetricsRegistryNilIsANoOp(t *testing.T) {
	resetAttrModMetrics(t)

	SetMetricsRegistry(nil)

	assert.Nil(t, attrModMetricsPtr.Load())
	assert.NotPanics(t, func() { RecordRemoveBufferRefused(8) })
}

// TestRecordRemoveBufferRefusedCountsPerAttribute verifies each attribute code
// gets its own series, under a stable name.
//
// VALIDATES: spec-fixit-rs-community-strip-arity R-2 -- "the warning appears in
// soak logs" is the early signal, and this counter is what makes it measurable
// without already suspecting it.
// PREVENTS: a counter that aggregates every attribute into one series, which
// would say a contract violation happened but not which handler saw it.
func TestRecordRemoveBufferRefusedCountsPerAttribute(t *testing.T) {
	resetAttrModMetrics(t)
	reg := &fakeRegistry{}

	SetMetricsRegistry(reg)
	require.Contains(t, reg.names, "ze_bgp_attr_mod_remove_buffer_refused_total")

	RecordRemoveBufferRefused(8)
	RecordRemoveBufferRefused(8)
	RecordRemoveBufferRefused(32)

	require.NotNil(t, reg.vec)
	assert.Equal(t, 2, reg.vec.counters["community"].n)
	assert.Equal(t, 1, reg.vec.counters["large-community"].n)
	assert.NotContains(t, reg.vec.counters, "extended-community",
		"a series must not be created for an attribute that never refused")
}

// TestAttrLabelNamesTheListValuedAttributes pins the label vocabulary.
//
// VALIDATES: the three codes with a registered list-valued handler
// (filter_community registers 8, 16 and 32) each get a readable label, and
// anything else collapses to one series.
// PREVENTS: an unexpected code minting an unbounded set of time series on a path
// that is already an error.
func TestAttrLabelNamesTheListValuedAttributes(t *testing.T) {
	for _, tc := range []struct {
		code uint8
		want string
	}{
		{8, "community"},
		{16, "extended-community"},
		{32, "large-community"},
		{1, "other"},
		{255, "other"},
	} {
		assert.Equal(t, tc.want, attrLabel(tc.code), "code %d", tc.code)
	}
}

// TestSetMetricsRegistryWiresRegisteredFilterMetrics verifies a registered
// filter's ConfigureMetrics hook receives the registry SetMetricsRegistry is
// given, and that a nil registry stays a no-op that calls no hook.
//
// VALIDATES: route C: the loop filter's metrics arrive through filterapi, so
// the reactor names no filter package.
// PREVENTS: a filter's counters left dead because nothing hands it the
// registry. Not parallel: it writes the package filter registry.
func TestSetMetricsRegistryWiresRegisteredFilterMetrics(t *testing.T) {
	const name = "test-metrics-hook"
	var got metrics.Registry
	calls := 0
	require.NoError(t, Register(Filter{
		Name:    name,
		Ingress: func(PeerFilterInfo, []byte, map[string]any) (bool, []byte) { return true, nil },
		ConfigureMetrics: func(reg metrics.Registry) {
			calls++
			got = reg
		},
	}))
	t.Cleanup(func() {
		mu.Lock()
		delete(filters, name)
		mu.Unlock()
		attrModMetricsPtr.Store(nil)
	})

	SetMetricsRegistry(nil)
	assert.Equal(t, 0, calls, "a nil registry must call no hook")

	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)
	assert.Equal(t, 1, calls)
	assert.Same(t, reg, got)
}

// TestSetMetricsRegistryHookMayCallRegister proves a ConfigureMetrics hook runs
// outside the filter registry lock.
//
// VALIDATES: a hook that registers another filter returns, and the filter it
// registered is in the registry afterwards. Method: the call runs in a
// goroutine and the test waits a bounded time, so a deadlock fails the test
// rather than hanging it.
// PREVENTS: SetMetricsRegistry calling the hooks under mu.RLock, where a hook
// calling Register waits forever on the write lock. Not parallel: it writes
// the package filter registry.
func TestSetMetricsRegistryHookMayCallRegister(t *testing.T) {
	const name = "test-metrics-hook-registers"
	const inner = "test-metrics-hook-registered"
	accept := func(PeerFilterInfo, []byte, map[string]any) (bool, []byte) { return true, nil }
	var innerErr error
	require.NoError(t, Register(Filter{
		Name:    name,
		Ingress: accept,
		ConfigureMetrics: func(metrics.Registry) {
			innerErr = Register(Filter{Name: inner, Ingress: accept})
		},
	}))
	wedged := false
	t.Cleanup(func() {
		attrModMetricsPtr.Store(nil)
		if wedged {
			return // the stuck goroutine holds mu; taking it would hang the binary
		}
		mu.Lock()
		delete(filters, name)
		delete(filters, inner)
		mu.Unlock()
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		SetMetricsRegistry(metrics.NewPrometheusRegistry())
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		wedged = true
		t.Fatal("SetMetricsRegistry did not return: the hook's Register deadlocked on the registry lock")
	}

	require.NoError(t, innerErr)
	mu.RLock()
	_, registered := filters[inner]
	mu.RUnlock()
	assert.True(t, registered, "the filter the hook registered is missing")
}
