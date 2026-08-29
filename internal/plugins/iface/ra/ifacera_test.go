package ifacera

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

// countingRegistry records what each labeled counter reached, so a metrics test
// asserts a value rather than the absence of a panic.
type countingRegistry struct {
	metrics.NopRegistry
	vecs map[string]*countingVec
}

type countingVec struct {
	counts map[string]float64
}

type countingCounter struct {
	vec   *countingVec
	label string
}

func newCountingRegistry() *countingRegistry {
	return &countingRegistry{vecs: make(map[string]*countingVec)}
}

func (r *countingRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	v := &countingVec{counts: make(map[string]float64)}
	r.vecs[name] = v
	return v
}

func (r *countingRegistry) count(name, label string) float64 {
	v, ok := r.vecs[name]
	if !ok {
		return -1
	}
	return v.counts[label]
}

func (v *countingVec) With(labels ...string) metrics.Counter {
	return &countingCounter{vec: v, label: labels[0]}
}

func (v *countingVec) Delete(...string) bool { return false }

func (c *countingCounter) Inc()          { c.vec.counts[c.label]++ }
func (c *countingCounter) Add(f float64) { c.vec.counts[c.label] += f }

// VALIDATES: spec AC-12. Sending an advertisement increments
// ze_iface_ra_sent_total, and answering a Router Solicitation increments both
// that and ze_iface_ra_solicited_total, labeled by interface.
// PREVENTS: a counter that is declared, documented, and never incremented,
// which reads as a silent link rather than a broken counter.
func TestRASenderMetrics(t *testing.T) {
	reg := newCountingRegistry()
	SetMetricsRegistry(reg)
	t.Cleanup(func() { SetMetricsRegistry(metrics.NopRegistry{}) })

	incSent("eth0")
	incSent("eth0")
	incSent("eth1")
	incSolicited("eth0")

	assert.Equal(t, float64(2), reg.count("ze_iface_ra_sent_total", "eth0"))
	assert.Equal(t, float64(1), reg.count("ze_iface_ra_sent_total", "eth1"))
	assert.Equal(t, float64(1), reg.count("ze_iface_ra_solicited_total", "eth0"))
	assert.Equal(t, float64(0), reg.count("ze_iface_ra_solicited_total", "eth1"))
}

// VALIDATES: the counters do nothing and never panic before a registry is
// bound, which is the state during startup and in every unit test that does
// not ask for metrics.
// PREVENTS: a nil registry crashing the send loop.
func TestRAMetricsWithoutRegistry(t *testing.T) {
	SetMetricsRegistry(metrics.NopRegistry{})
	require.NotPanics(t, func() {
		incSent("eth0")
		incSolicited("eth0")
	})
}
