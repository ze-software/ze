// Related: schedule.go -- the send schedule of RFC 4861 Sections 6.2.4 and 6.2.6.

package ndp

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
			got := UnsolicitedInterval(minimum, maximum, MaxInitialAdvertisements+i, r)
			assert.GreaterOrEqual(t, got, minimum, "interval below minimum-interval")
			assert.LessOrEqual(t, got, maximum, "interval above maximum-interval")
		}
	})

	t.Run("the interval varies", func(t *testing.T) {
		r := newTestRand()
		seen := make(map[time.Duration]struct{})
		for range 100 {
			seen[UnsolicitedInterval(200*time.Second, 600*time.Second, 10, r)] = struct{}{}
		}
		assert.Greater(t, len(seen), 1,
			"a constant interval synchronizes advertisements between routers (RFC 4861 Section 6.2.4)")
	})

	t.Run("the initial advertisements are capped", func(t *testing.T) {
		r := newTestRand()
		for sent := range MaxInitialAdvertisements {
			for range 200 {
				got := UnsolicitedInterval(200*time.Second, 600*time.Second, sent, r)
				assert.LessOrEqual(t, got, MaxInitialAdvertInterval,
					"advertisement %d of the initial burst waited longer than MAX_INITIAL_RTR_ADVERT_INTERVAL", sent)
			}
		}
	})

	t.Run("the cap does not raise a shorter configured interval", func(t *testing.T) {
		r := newTestRand()
		minimum := 3 * time.Second
		maximum := 4 * time.Second
		for sent := range MaxInitialAdvertisements {
			for range 200 {
				got := UnsolicitedInterval(minimum, maximum, sent, r)
				assert.GreaterOrEqual(t, got, minimum)
				assert.LessOrEqual(t, got, maximum, "the cap must shorten a wait, never lengthen it")
			}
		}
	})

	t.Run("the cap stops applying after the initial burst", func(t *testing.T) {
		r := newTestRand()
		aboveCap := false
		for range 1000 {
			if UnsolicitedInterval(200*time.Second, 600*time.Second, MaxInitialAdvertisements, r) > MaxInitialAdvertInterval {
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
			assert.Equal(t, 600*time.Second, UnsolicitedInterval(600*time.Second, 600*time.Second, 10, r))
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
		got := SolicitedDelay(r)
		assert.GreaterOrEqual(t, got, time.Duration(0))
		assert.LessOrEqual(t, got, MaxRADelayTime)
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
		got := SolicitedSendTime(last, now, 100*time.Millisecond)
		assert.Equal(t, now.Add(100*time.Millisecond), got)
	})

	t.Run("a solicitation inside the rate-limit window is deferred", func(t *testing.T) {
		last := start
		now := start.Add(500 * time.Millisecond)
		got := SolicitedSendTime(last, now, 100*time.Millisecond)
		// RFC 4861 Section 6.2.6: schedule at MIN_DELAY_BETWEEN_RAS plus the
		// random value, measured from the previous advertisement.
		assert.Equal(t, last.Add(MinDelayBetweenRAs+100*time.Millisecond), got)
		assert.False(t, got.Before(now), "a deferred advertisement is never scheduled in the past")
	})

	t.Run("a solicitation at the edge of the window", func(t *testing.T) {
		last := start
		now := start.Add(MinDelayBetweenRAs)
		got := SolicitedSendTime(last, now, 0)
		assert.Equal(t, now, got)
	})

	t.Run("the very first solicitation is not delayed by a zero last-sent time", func(t *testing.T) {
		got := SolicitedSendTime(time.Time{}, start, 200*time.Millisecond)
		assert.Equal(t, start.Add(200*time.Millisecond), got)
	})
}

// VALIDATES: RFC 4861 Section 6.2.6 as it reaches the Section 6.2.5 final
// advertisement. A teardown inside the rate-limit window waits out the
// remainder, and a teardown outside it waits not at all.
// PREVENTS: the two Ze senders answering that sentence differently, which they
// did until this arithmetic became one function. The LAN sender sent three
// advertisements in one scheduler tick while the PPP sender waited.
func TestRACeaseWait(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		lastSent time.Time
		want     time.Duration
	}{
		{"never advertised", time.Time{}, 0},
		{"advertised now", now, MinDelayBetweenRAs},
		{"one second into the window", now.Add(-1 * time.Second), MinDelayBetweenRAs - time.Second},
		{"the window has just closed", now.Add(-MinDelayBetweenRAs), 0},
		{"long past the window", now.Add(-time.Hour), 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, CeaseWait(test.lastSent, now))
		})
	}
}
