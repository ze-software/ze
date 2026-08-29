// Related: ra_schedule.go -- the RFC 4861 send schedule under test.
// RFC: rfc/full/rfc4861.txt -- Sections 6.2.1, 6.2.4, 6.2.6. RFC 4861 has no rfc/short/ summary and is not enrolled.

package ppp

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/test/sim"
)

// raTestStart is the instant every schedule test starts from. A fixed value
// keeps a failure reproducible.
var raTestStart = time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)

// newRATestSchedule returns a schedule on a fake clock and the clock that
// drives it. Every wait in these tests is fake time, so the whole file runs in
// microseconds while asserting on the RFC's real 3 second and 0.5 second
// bounds.
func newRATestSchedule(seed uint64) (*raSchedule, *sim.FakeClock) {
	clk := sim.NewFakeClock(raTestStart)
	return newRASchedule(clk, rand.New(rand.NewPCG(seed, seed+1))), clk
}

// VALIDATES: RFC 4861 Section 6.2.6, "Router Advertisements sent in response to
// a Router Solicitation MUST be delayed by a random time between 0 and
// MAX_RA_DELAY_TIME seconds". The delay is inside the range and it varies.
// PREVENTS: an answer sent the instant a solicitation arrives, which lets a
// subscriber set Ze's send rate, and every router on a link answering one
// solicitation at the same instant.
func TestRAScheduleDelaysASolicitedAdvertisement(t *testing.T) {
	seen := make(map[time.Duration]struct{})

	for seed := range uint64(200) {
		sched, clk := newRATestSchedule(seed)

		// One advertisement has gone out, and the rate-limit window has
		// closed, so the random delay is the only thing left to wait for.
		sched.advertised()
		clk.Add(ndp.MinDelayBetweenRAs + time.Second)

		sched.solicit()

		delay := sched.wait()
		if delay < 0 || delay > ndp.MaxRADelayTime {
			t.Fatalf("seed %d: solicited delay %v, want 0 to %v", seed, delay, ndp.MaxRADelayTime)
		}
		seen[delay] = struct{}{}
	}

	if len(seen) < 2 {
		t.Errorf("the solicited delay took %d distinct value(s) over 200 draws, want a random one", len(seen))
	}
}

// VALIDATES: RFC 4861 Section 6.2.6, "(If a single advertisement is sent in
// response to multiple solicitations, the delay is relative to the first
// solicitation.)" A burst leaves the schedule where the first solicitation put
// it.
// PREVENTS: a later solicitation in the same burst drawing its own delay, which
// moves the answer and makes the send time a function of the last solicitation
// rather than the first.
func TestRAScheduleTakesTheDelayFromTheFirstSolicitation(t *testing.T) {
	for seed := range uint64(50) {
		single, singleClk := newRATestSchedule(seed)
		single.advertised()
		singleClk.Add(ndp.MinDelayBetweenRAs + time.Second)
		single.solicit()

		burst, burstClk := newRATestSchedule(seed)
		burst.advertised()
		burstClk.Add(ndp.MinDelayBetweenRAs + time.Second)
		for range 5 {
			burst.solicit()
			burstClk.Add(time.Millisecond)
		}

		if !burst.nextSend.Equal(single.nextSend) {
			t.Fatalf("seed %d: a burst scheduled the answer at %v, one solicitation at %v; the delay must come from the first",
				seed, burst.nextSend.Sub(raTestStart), single.nextSend.Sub(raTestStart))
		}
	}
}

// VALIDATES: RFC 4861 Section 6.2.6, "consecutive Router Advertisements sent to
// the all-nodes multicast address MUST be rate limited to no more than one
// advertisement every MIN_DELAY_BETWEEN_RAS seconds". A solicitation arriving
// anywhere inside that window is answered when the window closes.
// PREVENTS: a subscriber sending one solicitation per millisecond setting Ze's
// advertisement rate.
func TestRAScheduleRateLimitsConsecutiveAdvertisements(t *testing.T) {
	for offsetMs := range 3000 {
		sched, clk := newRATestSchedule(uint64(offsetMs))

		sched.advertised()
		lastSent := sched.lastSent
		clk.Add(time.Duration(offsetMs) * time.Millisecond)

		sched.solicit()

		if gap := sched.nextSend.Sub(lastSent); gap < ndp.MinDelayBetweenRAs {
			t.Fatalf("a solicitation %d ms after an advertisement scheduled the next one %v later, want at least %v",
				offsetMs, gap, ndp.MinDelayBetweenRAs)
		}
	}
}

// VALIDATES: RFC 4861 Section 6.2.4. Every unsolicited interval is uniform
// between MinRtrAdvInterval and MaxRtrAdvInterval, and it varies.
// PREVENTS: the fixed ticker this schedule replaced, which synchronizes the
// advertisements of every session that came up together.
func TestRAScheduleRandomizesTheUnsolicitedInterval(t *testing.T) {
	sched, clk := newRATestSchedule(7)
	seen := make(map[time.Duration]struct{})

	for i := range 500 {
		previous := clk.Now()
		sched.advertised()
		interval := sched.nextSend.Sub(previous)

		// The first MaxInitialAdvertisements intervals carry their own
		// cap, which the next test covers.
		if i < ndp.MaxInitialAdvertisements {
			clk.Add(interval)
			continue
		}

		if interval < raMinRtrAdvInterval || interval > raMaxRtrAdvInterval {
			t.Fatalf("interval %d was %v, want %v to %v", i, interval, raMinRtrAdvInterval, raMaxRtrAdvInterval)
		}
		seen[interval] = struct{}{}
		clk.Add(interval)
	}

	if len(seen) < 2 {
		t.Errorf("the unsolicited interval took %d distinct value(s), want a random one (RFC 4861 Section 6.2.4)", len(seen))
	}
}

// VALIDATES: RFC 4861 Section 6.2.4. The first MAX_INITIAL_RTR_ADVERTISEMENTS
// advertisements are spaced by at most MAX_INITIAL_RTR_ADVERT_INTERVAL, and the
// configured interval applies from the one after them.
// PREVENTS: a subscriber waiting up to raMaxRtrAdvInterval for a second chance
// to find its router when the first advertisement is lost, and an initial burst
// that never ends and keeps advertising every 16 seconds for the session.
func TestRAScheduleCapsTheInitialAdvertisements(t *testing.T) {
	sched, clk := newRATestSchedule(11)
	start := clk.Now()

	// The first advertisement leaves as the interface becomes an
	// advertising interface, and each of the next ones waits at most the
	// cap, so all MAX_INITIAL_RTR_ADVERTISEMENTS land inside one cap fewer.
	limit := (ndp.MaxInitialAdvertisements - 1) * ndp.MaxInitialAdvertInterval
	for i := range ndp.MaxInitialAdvertisements {
		sched.advertised()
		if elapsed := clk.Now().Sub(start); elapsed > limit {
			t.Fatalf("initial advertisement %d left %v after the first, want the burst inside %v", i+1, elapsed, limit)
		}
		clk.Add(sched.wait())
	}

	previous := clk.Now()
	sched.advertised()
	if interval := sched.nextSend.Sub(previous); interval < raMinRtrAdvInterval {
		t.Errorf("the advertisement after the initial burst waited %v, want the configured %v at least",
			interval, raMinRtrAdvInterval)
	}
}

// VALIDATES: RFC 4861 Section 6.2.1. The two advertised intervals and the
// Router Lifetime satisfy the RFC's own bounds, and three unsolicited
// advertisements still fit inside one lifetime window.
// PREVENTS: the three numbers drifting apart, after which one lost
// advertisement near expiry silently drops the subscriber's default route.
func TestRAScheduleIntervalsFitTheRouterLifetime(t *testing.T) {
	lifetime := raRouterLifetime * time.Second

	if raMaxRtrAdvInterval*3 != lifetime {
		t.Errorf("3 * raMaxRtrAdvInterval = %v, want the router lifetime %v", raMaxRtrAdvInterval*3, lifetime)
	}
	if raMaxRtrAdvInterval < 4*time.Second || raMaxRtrAdvInterval > 1800*time.Second {
		t.Errorf("raMaxRtrAdvInterval = %v, want 4 s to 1800 s (RFC 4861 Section 6.2.1)", raMaxRtrAdvInterval)
	}
	if raMinRtrAdvInterval < 3*time.Second || raMinRtrAdvInterval > raMaxRtrAdvInterval*3/4 {
		t.Errorf("raMinRtrAdvInterval = %v, want 3 s to 0.75 * %v (RFC 4861 Section 6.2.1)",
			raMinRtrAdvInterval, raMaxRtrAdvInterval)
	}
	if lifetime < raMaxRtrAdvInterval || lifetime > 9000*time.Second {
		t.Errorf("router lifetime = %v, want between raMaxRtrAdvInterval and 9000 s (RFC 4861 Section 6.2.1)", lifetime)
	}
	// The rate limit must never hold back an unsolicited advertisement, or
	// the schedule and Section 6.2.6 would disagree about the next send.
	if raMinRtrAdvInterval < ndp.MinDelayBetweenRAs {
		t.Errorf("raMinRtrAdvInterval = %v, want at least MIN_DELAY_BETWEEN_RAS %v",
			raMinRtrAdvInterval, ndp.MinDelayBetweenRAs)
	}
}

// VALIDATES: the wait the final zero-lifetime advertisement owes the rate limit
// of RFC 4861 Section 6.2.6: the remainder of the window after the last
// advertisement, zero once the window has closed, and zero when nothing has
// been advertised yet.
// PREVENTS: a teardown that pauses for MIN_DELAY_BETWEEN_RAS every time, and a
// final advertisement that breaks the rate limit when a session is torn down
// straight after it advertised.
func TestRAScheduleCeaseWait(t *testing.T) {
	tests := []struct {
		name        string
		advertised  bool
		sinceLast   time.Duration
		wantCeaseAt time.Duration
	}{
		{name: "nothing advertised yet, nothing to wait for", advertised: false, wantCeaseAt: 0},
		{name: "torn down at once waits the whole window", advertised: true, sinceLast: 0, wantCeaseAt: ndp.MinDelayBetweenRAs},
		{name: "torn down inside the window waits the remainder", advertised: true, sinceLast: time.Second, wantCeaseAt: ndp.MinDelayBetweenRAs - time.Second},
		{name: "torn down at the edge of the window waits nothing", advertised: true, sinceLast: ndp.MinDelayBetweenRAs, wantCeaseAt: 0},
		{name: "torn down in steady state waits nothing", advertised: true, sinceLast: time.Hour, wantCeaseAt: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sched, clk := newRATestSchedule(3)
			if test.advertised {
				sched.advertised()
			}
			clk.Add(test.sinceLast)

			if got := sched.ceaseWait(); got != test.wantCeaseAt {
				t.Errorf("ceaseWait() = %v, want %v", got, test.wantCeaseAt)
			}
		})
	}
}

// VALIDATES: a schedule whose next advertisement is already overdue asks for no
// wait, rather than a negative one.
// PREVENTS: a negative duration reaching AfterFunc, where it fires at once and
// the sender spins.
func TestRAScheduleWaitIsNeverNegative(t *testing.T) {
	sched, clk := newRATestSchedule(5)
	sched.advertised()

	clk.Add(2 * raMaxRtrAdvInterval)

	if got := sched.wait(); got != 0 {
		t.Errorf("wait() = %v after the schedule passed, want 0", got)
	}
}
