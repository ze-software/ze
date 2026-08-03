// Design: docs/features/interfaces.md -- Router Advertisement sender for a LAN unit
//
// Package ifacera sends IPv6 Router Advertisements on a configured interface
// unit, so hosts on the link autoconfigure addresses, learn a default router,
// and learn resolvers. The interface component parses the configuration and
// decides which senders run; this package owns the socket, the timers, and the
// Router Solicitation answers.
//
// The timing helpers and the counters live here, with no build tag, so they run
// on every host. Only the socket work is Linux.
package ifacera

import (
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// Router constants of RFC 4861 Section 10.
const (
	// maxInitialAdvertInterval caps the wait between the first few
	// advertisements (MAX_INITIAL_RTR_ADVERT_INTERVAL).
	maxInitialAdvertInterval = 16 * time.Second
	// maxInitialAdvertisements is how many advertisements get that cap
	// (MAX_INITIAL_RTR_ADVERTISEMENTS).
	maxInitialAdvertisements = 3
	// maxFinalAdvertisements is how many zero-lifetime advertisements a
	// stopping sender may send (MAX_FINAL_RTR_ADVERTISEMENTS).
	maxFinalAdvertisements = 3
	// minDelayBetweenRAs is the floor between multicast advertisements
	// (MIN_DELAY_BETWEEN_RAS).
	minDelayBetweenRAs = 3 * time.Second
	// maxRADelayTime is the longest random wait before answering a Router
	// Solicitation (MAX_RA_DELAY_TIME).
	maxRADelayTime = 500 * time.Millisecond
)

// unsolicitedInterval returns the wait before the next unsolicited
// advertisement.
//
// RFC 4861 Section 6.2.4: the timer is reset to a value picked uniformly
// between MinRtrAdvInterval and MaxRtrAdvInterval, which keeps routers on one
// link from synchronizing. For the first MAX_INITIAL_RTR_ADVERTISEMENTS the
// wait is shortened to MAX_INITIAL_RTR_ADVERT_INTERVAL when the random value
// is longer, so a router that has just started is found quickly.
func unsolicitedInterval(minimum, maximum time.Duration, sent int, r *rand.Rand) time.Duration {
	interval := minimum
	if span := maximum - minimum; span > 0 {
		interval += time.Duration(r.Int64N(int64(span) + 1))
	}
	if sent < maxInitialAdvertisements && interval > maxInitialAdvertInterval {
		interval = maxInitialAdvertInterval
	}
	return interval
}

// solicitedDelay returns the random wait before answering a Router
// Solicitation, uniform in 0 to MAX_RA_DELAY_TIME (RFC 4861 Section 6.2.6).
func solicitedDelay(r *rand.Rand) time.Duration {
	return time.Duration(r.Int64N(int64(maxRADelayTime) + 1))
}

// solicitedSendTime returns when to send the advertisement that answers a
// Router Solicitation received at now, given when the last multicast
// advertisement went out and the random delay already drawn.
//
// RFC 4861 Section 6.2.6: consecutive multicast advertisements are rate limited
// to one every MIN_DELAY_BETWEEN_RAS. A solicitation that arrives inside that
// window is answered at MIN_DELAY_BETWEEN_RAS plus the random delay after the
// previous advertisement, so a flood of solicitations cannot become a flood of
// advertisements.
func solicitedSendTime(lastSent, now time.Time, delay time.Duration) time.Time {
	send := now.Add(delay)
	if lastSent.IsZero() {
		return send
	}
	if earliest := lastSent.Add(minDelayBetweenRAs + delay); earliest.After(send) {
		return earliest
	}
	return send
}

// raMetrics holds the counters this plugin publishes.
type raMetrics struct {
	sent      metrics.CounterVec
	solicited metrics.CounterVec
}

var metricsPtr atomic.Pointer[raMetrics]

// SetMetricsRegistry binds the counters to a registry. Called through the
// plugin registration's metrics hook; until then every counter is a no-op.
func SetMetricsRegistry(reg metrics.Registry) {
	metricsPtr.Store(&raMetrics{
		sent: reg.CounterVec("ze_iface_ra_sent_total",
			"Router Advertisements sent, periodic and solicited together, by interface.",
			[]string{"interface"}),
		solicited: reg.CounterVec("ze_iface_ra_solicited_total",
			"Router Advertisements sent in answer to a Router Solicitation, by interface.",
			[]string{"interface"}),
	})
}

// incSent counts one advertisement put on the wire.
func incSent(ifaceName string) {
	if m := metricsPtr.Load(); m != nil {
		m.sent.With(ifaceName).Inc()
	}
}

// incSolicited counts one advertisement that answered a Router Solicitation.
// The same advertisement is counted by incSent as well, so sent stays the total.
func incSolicited(ifaceName string) {
	if m := metricsPtr.Load(); m != nil {
		m.solicited.With(ifaceName).Inc()
	}
}
