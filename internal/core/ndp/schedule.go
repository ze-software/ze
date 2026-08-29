// Design: rfc/full/rfc4861.txt -- Sections 6.2.4 and 6.2.6, the send schedule of an advertising interface.
// Detail: ra.go -- the wire encoding these timers decide when to transmit.
// Overview: every Router Advertisement sender in Ze shares this arithmetic, so
// one reading of the RFC governs the LAN sender and the PPP subscriber sender.
// RFC 4861 has no rfc/short/ summary and is not enrolled.

package ndp

import (
	"math/rand/v2"
	"time"
)

// Router constants of RFC 4861 Section 10.
const (
	// MaxInitialAdvertInterval caps the wait between the first few
	// advertisements (MAX_INITIAL_RTR_ADVERT_INTERVAL).
	MaxInitialAdvertInterval = 16 * time.Second
	// MaxInitialAdvertisements is how many advertisements get that cap
	// (MAX_INITIAL_RTR_ADVERTISEMENTS).
	MaxInitialAdvertisements = 3
	// MinDelayBetweenRAs is the floor between multicast advertisements
	// (MIN_DELAY_BETWEEN_RAS).
	MinDelayBetweenRAs = 3 * time.Second
	// MaxRADelayTime is the longest random wait before answering a Router
	// Solicitation (MAX_RA_DELAY_TIME).
	MaxRADelayTime = 500 * time.Millisecond
)

// UnsolicitedInterval returns the wait before the next unsolicited
// advertisement. sent is how many multicast advertisements the interface has
// already sent.
//
// RFC 4861 Section 6.2.4: the timer is reset to a value picked uniformly
// between MinRtrAdvInterval and MaxRtrAdvInterval, which keeps routers on one
// link from synchronizing. For the first MAX_INITIAL_RTR_ADVERTISEMENTS the
// wait is shortened to MAX_INITIAL_RTR_ADVERT_INTERVAL when the random value
// is longer, so a router that has just started is found quickly.
func UnsolicitedInterval(minimum, maximum time.Duration, sent int, r *rand.Rand) time.Duration {
	interval := minimum
	if span := maximum - minimum; span > 0 {
		interval += time.Duration(r.Int64N(int64(span) + 1))
	}
	if sent < MaxInitialAdvertisements && interval > MaxInitialAdvertInterval {
		interval = MaxInitialAdvertInterval
	}
	return interval
}

// SolicitedDelay returns the random wait before answering a Router
// Solicitation, uniform in 0 to MAX_RA_DELAY_TIME (RFC 4861 Section 6.2.6).
func SolicitedDelay(r *rand.Rand) time.Duration {
	return time.Duration(r.Int64N(int64(MaxRADelayTime) + 1))
}

// SolicitedSendTime returns when to send the advertisement that answers a
// Router Solicitation received at now, given when the last multicast
// advertisement went out and the random delay already drawn.
//
// RFC 4861 Section 6.2.6: consecutive multicast advertisements are rate limited
// to one every MIN_DELAY_BETWEEN_RAS. A solicitation that arrives inside that
// window is answered at MIN_DELAY_BETWEEN_RAS plus the random delay after the
// previous advertisement, so a flood of solicitations cannot become a flood of
// advertisements.
func SolicitedSendTime(lastSent, now time.Time, delay time.Duration) time.Time {
	send := now.Add(delay)
	if lastSent.IsZero() {
		return send
	}
	if earliest := lastSent.Add(MinDelayBetweenRAs + delay); earliest.After(send) {
		return earliest
	}
	return send
}
