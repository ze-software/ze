package ifacera

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

// newTestRand gives every timing test the same sequence, so a failure is
// reproducible rather than a once-in-a-run event.
func newTestRand() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

// VALIDATES: RFC 4861 Section 6.2.4. The wait before the next unsolicited
// advertisement is uniform between MinRtrAdvInterval and MaxRtrAdvInterval, and
// the first MAX_INITIAL_RTR_ADVERTISEMENTS waits are capped at
// MAX_INITIAL_RTR_ADVERT_INTERVAL so a new router is found quickly.
// PREVENTS: a fixed interval, which synchronizes with other routers on the
// link, and a slow first burst, which leaves hosts without a router for
// minutes after Ze starts.
func TestRAIntervalBounds(t *testing.T) {
	t.Run("every interval falls inside the configured range", func(t *testing.T) {
		r := newTestRand()
		minimum := 200 * time.Second
		maximum := 600 * time.Second

		for i := range 1000 {
			// Past the initial burst, so the cap does not apply.
			got := unsolicitedInterval(minimum, maximum, maxInitialAdvertisements+i, r)
			assert.GreaterOrEqual(t, got, minimum, "interval below minimum-interval")
			assert.LessOrEqual(t, got, maximum, "interval above maximum-interval")
		}
	})

	t.Run("the interval varies", func(t *testing.T) {
		r := newTestRand()
		seen := make(map[time.Duration]struct{})
		for range 100 {
			seen[unsolicitedInterval(200*time.Second, 600*time.Second, 10, r)] = struct{}{}
		}
		assert.Greater(t, len(seen), 1,
			"a constant interval synchronizes advertisements between routers (RFC 4861 Section 6.2.4)")
	})

	t.Run("the initial advertisements are capped", func(t *testing.T) {
		r := newTestRand()
		for sent := range maxInitialAdvertisements {
			for range 200 {
				got := unsolicitedInterval(200*time.Second, 600*time.Second, sent, r)
				assert.LessOrEqual(t, got, maxInitialAdvertInterval,
					"advertisement %d of the initial burst waited longer than MAX_INITIAL_RTR_ADVERT_INTERVAL", sent)
			}
		}
	})

	t.Run("the cap does not raise a shorter configured interval", func(t *testing.T) {
		r := newTestRand()
		minimum := 3 * time.Second
		maximum := 4 * time.Second
		for sent := range maxInitialAdvertisements {
			for range 200 {
				got := unsolicitedInterval(minimum, maximum, sent, r)
				assert.GreaterOrEqual(t, got, minimum)
				assert.LessOrEqual(t, got, maximum, "the cap must shorten a wait, never lengthen it")
			}
		}
	})

	t.Run("the cap stops applying after the initial burst", func(t *testing.T) {
		r := newTestRand()
		aboveCap := false
		for range 1000 {
			if unsolicitedInterval(200*time.Second, 600*time.Second, maxInitialAdvertisements, r) > maxInitialAdvertInterval {
				aboveCap = true
				break
			}
		}
		assert.True(t, aboveCap,
			"once the initial burst is sent the configured interval applies, not the 16 second cap")
	})

	t.Run("a minimum equal to the maximum is a fixed interval", func(t *testing.T) {
		r := newTestRand()
		for range 50 {
			assert.Equal(t, 600*time.Second, unsolicitedInterval(600*time.Second, 600*time.Second, 10, r))
		}
	})
}

// VALIDATES: RFC 4861 Section 6.2.6. A solicited advertisement waits a random
// time between 0 and MAX_RA_DELAY_TIME.
// PREVENTS: every router on a link answering one Router Solicitation at the
// same instant.
func TestRASolicitedDelayBounds(t *testing.T) {
	r := newTestRand()
	seen := make(map[time.Duration]struct{})
	for range 1000 {
		got := solicitedDelay(r)
		assert.GreaterOrEqual(t, got, time.Duration(0))
		assert.LessOrEqual(t, got, maxRADelayTime)
		seen[got] = struct{}{}
	}
	assert.Greater(t, len(seen), 1, "the delay must be random, not constant")
}

// VALIDATES: RFC 4861 Section 6.2.6. Multicast advertisements are rate limited
// to one every MIN_DELAY_BETWEEN_RAS, and a solicitation arriving inside that
// window is answered when the window ends rather than dropped.
// PREVENTS: a Router Solicitation flood turning into an advertisement flood.
func TestRARateLimit(t *testing.T) {
	start := time.Unix(1000, 0)

	t.Run("a solicitation long after the last advertisement waits only its delay", func(t *testing.T) {
		last := start
		now := start.Add(10 * time.Second)
		got := solicitedSendTime(last, now, 100*time.Millisecond)
		assert.Equal(t, now.Add(100*time.Millisecond), got)
	})

	t.Run("a solicitation inside the rate-limit window is deferred", func(t *testing.T) {
		last := start
		now := start.Add(500 * time.Millisecond)
		got := solicitedSendTime(last, now, 100*time.Millisecond)
		// RFC 4861 Section 6.2.6: schedule at MIN_DELAY_BETWEEN_RAS plus the
		// random value, measured from the previous advertisement.
		assert.Equal(t, last.Add(minDelayBetweenRAs+100*time.Millisecond), got)
		assert.False(t, got.Before(now), "a deferred advertisement is never scheduled in the past")
	})

	t.Run("a solicitation at the edge of the window", func(t *testing.T) {
		last := start
		now := start.Add(minDelayBetweenRAs)
		got := solicitedSendTime(last, now, 0)
		assert.Equal(t, now, got)
	})

	t.Run("the very first solicitation is not delayed by a zero last-sent time", func(t *testing.T) {
		got := solicitedSendTime(time.Time{}, start, 200*time.Millisecond)
		assert.Equal(t, start.Add(200*time.Millisecond), got)
	})
}

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
