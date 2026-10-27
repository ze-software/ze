// Design: docs/architecture/dns/lan-resolver.md -- outstanding bound, per-client rate, coalescing
// Related: state.go -- the resolver and its sweeper
// Related: server.go -- the answer function that consults these limits

package dnsresolver

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

// clientsMax bounds the per-client rate table. A new source arriving while
// the table is full, after the stale windows are swept, is rate-limited:
// admitting it would let a flood of sources grow the table without bound.
const clientsMax = 65536

// rateExcessPassEvery lets one in this many queries over the rate through,
// so a client behind a shared address still gets some answers. Unbound's
// ip-ratelimit-factor default, 10.
const rateExcessPassEvery = 10

// clientKey is the source address a rate window counts.
type clientKey = netip.Addr

// clientWindow counts one source's queries within one second.
type clientWindow struct {
	second int64
	count  uint32
}

// limiter holds the outstanding-resolution count and the per-client rate
// windows. Safe for concurrent use.
type limiter struct {
	outstanding atomic.Int64

	mu      sync.Mutex
	clients map[clientKey]clientWindow
}

// acquire takes one outstanding-resolution slot under limit, and reports
// false when every slot is taken. A true answer MUST be paired with release.
func (l *limiter) acquire(limit uint32) bool {
	if l.outstanding.Add(1) > int64(limit) {
		l.outstanding.Add(-1)
		return false
	}
	return true
}

// release returns a slot taken by acquire. MUST follow a true acquire.
func (l *limiter) release() { l.outstanding.Add(-1) }

// admit counts one query from source against rate queries per second, and
// reports whether it is answered. Over the rate, one in
// rateExcessPassEvery of the excess passes.
func (l *limiter) admit(source netip.Addr, rate uint32, now time.Time) bool {
	// refuse-by-default: guards admit -- a rate of 0 is the YANG "off" value
	if rate == 0 {
		return true
	}
	second := now.Unix()
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.clients[source]
	if !ok {
		if len(l.clients) >= clientsMax {
			l.sweepLocked(second)
		}
		if len(l.clients) >= clientsMax {
			return false
		}
	}
	if w.second != second {
		w = clientWindow{second: second}
	}
	w.count++
	l.clients[source] = w
	if w.count <= rate {
		return true
	}
	return (w.count-rate)%rateExcessPassEvery == 0
}

// sweep drops every window older than the current second.
func (l *limiter) sweep(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(now.Unix())
}

func (l *limiter) sweepLocked(second int64) {
	for k, w := range l.clients {
		if w.second != second {
			delete(l.clients, k)
		}
	}
}

// flight is one resolution in progress. Waiters block on done; the leader
// writes res before closing it, and nobody writes res after.
type flight struct {
	done chan struct{}
	res  result
}

// flightTable coalesces identical questions: the first asker resolves, the
// others wait on its flight. Safe for concurrent use.
type flightTable struct {
	mu      sync.Mutex
	flights map[cacheKey]*flight
}

// join returns the flight for key and true when the caller is its leader,
// which MUST call finish. A false answer means another resolution of the
// same question is in progress: wait on its done channel.
func (t *flightTable) join(key cacheKey) (*flight, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if f, ok := t.flights[key]; ok {
		return f, false
	}
	f := &flight{done: make(chan struct{})}
	t.flights[key] = f
	return f, true
}

// finish publishes the leader's result to every waiter. MUST be called once
// by the leader join named.
func (t *flightTable) finish(key cacheKey, f *flight, res result) {
	t.mu.Lock()
	delete(t.flights, key)
	t.mu.Unlock()
	f.res = res
	close(f.done)
}

// budget bounds the work one client query may cause.
//
// RFC 4035 Section 5.4: "As with all DNS operations, however, the resolver
// MUST bound the work it puts into answering any particular query."
//
// Unbound's defaults are the bounds: max-sent-count 32 upstream queries and
// max-query-restarts 11, with the configured timeout as the deadline.
type budget struct {
	deadline time.Time
	queries  int
	restarts int
}

const (
	upstreamQueriesMax = 32
	restartsMax        = 11
)

// spendQuery takes one upstream query from the budget, and reports false
// when none is left.
func (b *budget) spendQuery() bool {
	if b.queries >= upstreamQueriesMax {
		return false
	}
	b.queries++
	return true
}

// spendRestart takes one CNAME restart, and reports false when none is left.
func (b *budget) spendRestart() bool {
	if b.restarts >= restartsMax {
		return false
	}
	b.restarts++
	return true
}
