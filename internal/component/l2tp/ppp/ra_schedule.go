// Design: rfc/full/rfc4861.txt -- Sections 6.2.4 and 6.2.6 decide when the subscriber's Router Advertisements leave.
// Related: ra_send.go -- the loop that asks this schedule how long to wait.
//
// The arithmetic this file schedules with lives in internal/core/ndp, shared
// with the LAN Router Advertisement sender, so one reading of the RFC governs
// both. RFC 4861 has no rfc/short/ summary and is not enrolled.

package ppp

import (
	"math/rand/v2"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/ndp"
)

const (
	// raRouterLifetime is the Router Lifetime advertised to the subscriber,
	// in seconds. RFC 4861 Section 6.2.1 requires AdvDefaultLifetime to be
	// zero or between MaxRtrAdvInterval and 9000 seconds.
	raRouterLifetime = 1800

	// raMaxRtrAdvInterval is the longest wait between two unsolicited
	// advertisements, and raMinRtrAdvInterval the shortest. RFC 4861
	// Section 6.2.1 gives AdvDefaultLifetime a default of
	// 3 * MaxRtrAdvInterval and MinRtrAdvInterval a default of one third of
	// MaxRtrAdvInterval, so both derive from the lifetime and the
	// subscriber keeps its default route across two lost advertisements.
	raMaxRtrAdvInterval = raRouterLifetime * time.Second / 3
	raMinRtrAdvInterval = raMaxRtrAdvInterval / 3

	// raCeaseLifetime is the Router Lifetime of the final advertisement.
	// Zero tells the subscriber this router is no longer a default router.
	// RFC 4861 Section 4.2.
	raCeaseLifetime = 0
)

// raSchedule decides when the next multicast Router Advertisement leaves one
// subscriber interface. It sends nothing itself and answers four questions:
// how long to wait, what a Router Solicitation changes, what a completed send
// changes, and what the final advertisement of a teardown owes the rate limit.
//
// NOT safe for concurrent use. One goroutine owns it at a time: raSenderLoop
// until it closes senderDone, then stopRASender, which reads lastSent to rate
// limit the final advertisement.
type raSchedule struct {
	clk    clock.Clock
	random *rand.Rand

	// count is how many multicast advertisements this interface has sent,
	// held at ndp.MaxInitialAdvertisements once reached because the only reader
	// asks whether the initial burst is over.
	count     int
	lastSent  time.Time // when the last advertisement left; zero before the first
	nextSend  time.Time // when the next advertisement is scheduled
	solicited bool      // the scheduled advertisement answers a solicitation
}

// newRASchedule returns a schedule whose first advertisement is due at once,
// because an interface that becomes an advertising interface advertises
// immediately (RFC 4861 Section 6.2.4).
func newRASchedule(clk clock.Clock, random *rand.Rand) *raSchedule {
	return &raSchedule{clk: clk, random: random, nextSend: clk.Now()}
}

// wait returns how long the sender waits before the next advertisement. A
// schedule whose time has passed returns zero rather than a negative duration,
// so a late wake-up sends at once instead of arming a timer in the past.
func (s *raSchedule) wait() time.Duration {
	remaining := s.nextSend.Sub(s.clk.Now())
	if remaining < 0 {
		return 0
	}
	return remaining
}

// solicit schedules the advertisement that answers a Router Solicitation.
//
// RFC 4861 Section 6.2.6: "In all cases, Router Advertisements sent in response
// to a Router Solicitation MUST be delayed by a random time between 0 and
// MAX_RA_DELAY_TIME seconds. (If a single advertisement is sent in response to
// multiple solicitations, the delay is relative to the first solicitation.) In
// addition, consecutive Router Advertisements sent to the all-nodes multicast
// address MUST be rate limited to no more than one advertisement every
// MIN_DELAY_BETWEEN_RAS seconds."
//
// The section's own three-step algorithm is spelled out below, in its order.
func (s *raSchedule) solicit() {
	// The parenthetical: an answer is already scheduled, so this
	// solicitation is answered by that one. Drawing a second delay would
	// measure it from this solicitation rather than from the first.
	if s.solicited {
		return
	}

	// Steps two and three: a solicitation inside the rate-limit window is
	// answered at MIN_DELAY_BETWEEN_RAS plus the random delay after the
	// previous advertisement, and one outside it at the random delay.
	at := ndp.SolicitedSendTime(s.lastSent, s.clk.Now(), ndp.SolicitedDelay(s.random))

	// Step one: "If the computed value corresponds to a time later than the
	// time the next multicast Router Advertisement is scheduled to be sent,
	// ignore the random delay and send the advertisement at the
	// already-scheduled time." That advertisement answers the solicitation,
	// so nothing here changes and no extra message goes out.
	if !at.Before(s.nextSend) {
		return
	}

	s.solicited = true
	s.nextSend = at
}

// advertised records that an advertisement has left the interface and picks
// when the next one is due.
//
// RFC 4861 Section 6.2.6 sends a multicast answer with "the interface's
// interval timer ... reset to a new random value, as if an unsolicited
// advertisement had just been sent", so a solicited advertisement and an
// unsolicited one reschedule the same way.
func (s *raSchedule) advertised() {
	now := s.clk.Now()

	if s.count < ndp.MaxInitialAdvertisements {
		s.count++
	}
	s.lastSent = now
	s.solicited = false
	s.nextSend = now.Add(ndp.UnsolicitedInterval(raMinRtrAdvInterval, raMaxRtrAdvInterval, s.count, s.random))
}

// ceaseWait returns how long the final zero-lifetime advertisement waits for
// the rate limit. RFC 4861 Section 6.2.6 rate limits consecutive multicast
// advertisements to one every MIN_DELAY_BETWEEN_RAS, and the Section 6.2.5
// final advertisement is one of them, so a teardown that follows an
// advertisement inside that window waits out the remainder. The wait is bounded
// by MIN_DELAY_BETWEEN_RAS and is zero in steady state, where advertisements
// are at least raMinRtrAdvInterval apart.
func (s *raSchedule) ceaseWait() time.Duration {
	if s.lastSent.IsZero() {
		return 0
	}
	remaining := s.lastSent.Add(ndp.MinDelayBetweenRAs).Sub(s.clk.Now())
	if remaining < 0 {
		return 0
	}
	return remaining
}
