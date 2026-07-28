package filterapi

import (
	"testing"

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
