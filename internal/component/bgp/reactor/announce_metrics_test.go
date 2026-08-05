package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
)

// announceFakeCounter records Inc calls.
type announceFakeCounter struct{ n int }

func (c *announceFakeCounter) Inc()          { c.n++ }
func (c *announceFakeCounter) Add(_ float64) { c.n++ }

// announceFakeCounterVec hands out one counter per label-value tuple and keeps
// the tuple, so a test can assert WHICH series moved rather than only that some
// counter did.
type announceFakeCounterVec struct {
	counters map[string]*announceFakeCounter
}

func (v *announceFakeCounterVec) With(labelValues ...string) metrics.Counter {
	key := ""
	for i, lv := range labelValues {
		if i > 0 {
			key += "|"
		}
		key += lv
	}
	if v.counters == nil {
		v.counters = map[string]*announceFakeCounter{}
	}
	c, ok := v.counters[key]
	if !ok {
		c = &announceFakeCounter{}
		v.counters[key] = c
	}
	return c
}

func (v *announceFakeCounterVec) Delete(_ ...string) bool { return false }

// announceFakeRegistry implements metrics.Registry, returning the one CounterVec
// this file's subject creates and no-ops for everything else.
type announceFakeRegistry struct {
	vec   *announceFakeCounterVec
	names []string
}

func (r *announceFakeRegistry) Counter(_, _ string) metrics.Counter { return &announceFakeCounter{} }
func (r *announceFakeRegistry) Gauge(_, _ string) metrics.Gauge {
	return metrics.NopRegistry{}.Gauge("", "")
}

func (r *announceFakeRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	r.names = append(r.names, name)
	if r.vec == nil {
		r.vec = &announceFakeCounterVec{}
	}
	return r.vec
}

func (r *announceFakeRegistry) GaugeVec(_, _ string, _ []string) metrics.GaugeVec {
	return metrics.NopRegistry{}.GaugeVec("", "", nil)
}

func (r *announceFakeRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram {
	return metrics.NopRegistry{}.Histogram("", "", nil)
}

func (r *announceFakeRegistry) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return metrics.NopRegistry{}.HistogramVec("", "", nil, nil)
}

// resetAnnounceMetrics puts the package back to its unwired state so tests do
// not leak a registry into each other.
func resetAnnounceMetrics(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { announceMetricsPtr.Store(nil) })
	announceMetricsPtr.Store(nil)
}

// TestRecordAnnounceDroppedOversizeIsSilentWithoutARegistry verifies the
// recorder is safe before any registry is wired.
//
// VALIDATES: a build with metrics disabled, which is the default, keeps working.
// PREVENTS: a nil-pointer panic on the announce path in every such build.
func TestRecordAnnounceDroppedOversizeIsSilentWithoutARegistry(t *testing.T) {
	resetAnnounceMetrics(t)

	assert.NotPanics(t, func() {
		recordAnnounceDroppedOversize(announceRailBatch, announceStageNLRI)
	}, "the recorder must be a no-op until a registry is wired")

	setAnnounceMetricsRegistry(nil)
	assert.NotPanics(t, func() {
		recordAnnounceDroppedOversize(announceRailQueued, announceStageAttributes)
	}, "a nil registry must leave the recorder disabled, not half-wired")
}

// TestRecordAnnounceDroppedOversizeCountsPerRailAndStage pins the label
// vocabulary and proves the series are distinct.
//
// VALIDATES: an operator can tell WHICH writer refused and at WHICH region,
// which is the difference between an alert and a grep.
// PREVENTS: both rails collapsing onto one series, which would make the counter
// unable to answer the first question asked of it.
func TestRecordAnnounceDroppedOversizeCountsPerRailAndStage(t *testing.T) {
	resetAnnounceMetrics(t)

	reg := &announceFakeRegistry{}
	setAnnounceMetricsRegistry(reg)

	require.Contains(t, reg.names, "ze_bgp_announce_dropped_oversize_total",
		"the counter must register under its ze_-prefixed name")

	recordAnnounceDroppedOversize(announceRailBatch, announceStageNLRI)
	recordAnnounceDroppedOversize(announceRailBatch, announceStageAttributes)
	recordAnnounceDroppedOversize(announceRailBatch, announceStageAttributes)
	recordAnnounceDroppedOversize(announceRailQueued, announceStageAttributes)

	require.NotNil(t, reg.vec)
	assert.Equal(t, 1, reg.vec.counters["batch|nlri"].n)
	assert.Equal(t, 2, reg.vec.counters["batch|attributes"].n)
	assert.Equal(t, 1, reg.vec.counters["queued|attributes"].n)
	assert.Len(t, reg.vec.counters, 3, "no series beyond the three driven above")
}

// TestAnnounceDropLoggersRecordTheCounter is the wiring test: the counter must
// move when the PRODUCING log function runs, not only when the recorder is
// called directly.
//
// VALIDATES: both fail-closed drop sites are counted, so the metric cannot be
// registered-but-dead.
// PREVENTS: the exact failure this spec exists to fix. AC-5's drop was
// implemented and logged, and a counter that no drop site calls would leave the
// operator with the same log-only visibility while looking solved.
func TestAnnounceDropLoggersRecordTheCounter(t *testing.T) {
	resetAnnounceMetrics(t)

	reg := &announceFakeRegistry{}
	setAnnounceMetricsRegistry(reg)

	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	logAnnounceTooLarge(bgptypes.NLRIBatch{Family: family.IPv4Unicast}, 256, announceStageNLRI)
	logRIBRouteTooLarge(wn, 4096, announceStageAttributes)

	require.NotNil(t, reg.vec, "a drop must reach the counter through its log function")
	assert.Equal(t, 1, reg.vec.counters["batch|nlri"].n,
		"logAnnounceTooLarge must count the batch rail")
	assert.Equal(t, 1, reg.vec.counters["queued|attributes"].n,
		"logRIBRouteTooLarge must count the queued rail")
}
