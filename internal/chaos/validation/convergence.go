// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/family"
)

// pendingKey identifies a specific route-to-peer expectation.
type pendingKey struct {
	peer  int
	route routeKey
}

// pendingEntry records when a route was announced and which peer sourced it.
type pendingEntry struct {
	source       int
	announceTime time.Time
	reportFamily bool
}

// SlowRoute describes a route that exceeded the convergence deadline.
type SlowRoute struct {
	Source   int
	Peer     int
	Prefix   netip.Prefix
	Family   family.Family // Zero denotes the default unicast family for the prefix.
	Age      time.Duration
	Received bool
}

// ConvergenceStats holds aggregate latency statistics.
type ConvergenceStats struct {
	Resolved int
	Pending  int
	Min      time.Duration
	Max      time.Duration
	Avg      time.Duration
	P99      time.Duration
}

// Convergence tracks announcement-to-receipt latency for route propagation.
// Safe for concurrent use.
type Convergence struct {
	mu        sync.Mutex
	peerCount int
	deadline  time.Duration
	reachable Reachability

	// pending maps (peer, family, prefix) to unresolved announcements.
	pending map[pendingKey]pendingEntry
	// earlyReceipts holds one timestamp per unmatched destination and route.
	// A batch can arrive at all destinations before the sends are published.
	earlyReceipts map[pendingKey]time.Time

	// latencies stores all resolved propagation durations.
	latencies []time.Duration

	// late retains missed deadlines when a pending obligation ends.
	late []SlowRoute

	// familyLatencies stores per-family resolved latencies alongside aggregate.
	familyLatencies map[family.Family][]time.Duration
}

// NewConvergence creates a tracker with full reachability between distinct peers.
func NewConvergence(n int, deadline time.Duration) *Convergence {
	return NewConvergenceWithReachability(n, deadline, nil)
}

// NewConvergenceWithReachability creates a tracker for the target's export policy.
func NewConvergenceWithReachability(n int, deadline time.Duration, reachable Reachability) *Convergence {
	return &Convergence{
		peerCount:       n,
		deadline:        deadline,
		reachable:       reachable,
		pending:         make(map[pendingKey]pendingEntry),
		earlyReceipts:   make(map[pendingKey]time.Time),
		familyLatencies: make(map[family.Family][]time.Duration),
	}
}

// RecordAnnounce records that a peer announced a route at the given time.
// Creates pending entries for peers eligible to receive it.
// If family is provided, per-family latency is tracked when resolved.
func (c *Convergence) RecordAnnounce(source int, prefix netip.Prefix, at time.Time, names ...string) {
	name := ""
	if len(names) > 0 {
		name = names[0]
	}
	c.recordAnnounce(source, makeRouteKey(prefix, name), at, name)
}

func (c *Convergence) recordAnnounce(source int, route routeKey, at time.Time, name string) {
	if source < 0 {
		return
	}
	if source >= c.peerCount {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for peer := range c.peerCount {
		if !c.reachable.allows(source, peer, name) {
			continue
		}
		key := pendingKey{peer: peer, route: route}
		entry := pendingEntry{source: source, announceTime: at, reportFamily: name != ""}
		if receivedAt, found := c.earlyReceipts[key]; found {
			delete(c.earlyReceipts, key)
			if !receivedAt.Before(at) {
				c.recordDelivery(key, entry, receivedAt)
				continue
			}
		}
		c.pending[key] = entry
	}
}

// RecordReceive records that a peer received a route at the given time.
// The simulator can publish the source's announcement after the destination
// receives it. Keep an unmatched arrival until its announcement arrives.
func (c *Convergence) RecordReceive(peer int, prefix netip.Prefix, at time.Time) {
	c.recordReceive(peer, makeRouteKey(prefix, ""), at)
}

func (c *Convergence) recordReceive(peer int, route routeKey, at time.Time) {
	if peer < 0 {
		return
	}
	if peer >= c.peerCount {
		return
	}
	if !route.prefix.IsValid() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key := pendingKey{peer: peer, route: route}
	entry, ok := c.pending[key]
	if ok {
		if at.Before(entry.announceTime) {
			return
		}
		c.recordDelivery(key, entry, at)
		return
	}
	if earlier, exists := c.earlyReceipts[key]; exists {
		if at.After(earlier) {
			c.earlyReceipts[key] = at
		}
		return
	}
	c.earlyReceipts[key] = at
}

func (c *Convergence) recordDelivery(key pendingKey, entry pendingEntry, at time.Time) {
	latency := at.Sub(entry.announceTime)
	c.latencies = append(c.latencies, latency)
	if latency >= c.deadline {
		c.late = append(c.late, slowRoute(key, entry, latency, true))
	}
	if entry.reportFamily {
		fam := key.route.family
		c.familyLatencies[fam] = append(c.familyLatencies[fam], latency)
	}
	delete(c.pending, key)
}

func slowRoute(key pendingKey, entry pendingEntry, age time.Duration, received bool) SlowRoute {
	result := SlowRoute{Source: entry.source, Peer: key.peer, Prefix: key.route.prefix, Age: age, Received: received}
	if key.route.family != unicastFamily(key.route.prefix) {
		result.Family = key.route.family
	}
	return result
}

func (c *Convergence) cancelPending(key pendingKey, entry pendingEntry, at time.Time) {
	if !at.IsZero() {
		age := at.Sub(entry.announceTime)
		if age >= c.deadline {
			c.late = append(c.late, slowRoute(key, entry, age, false))
		}
	}
	delete(c.pending, key)
}

// RecordWithdraw cancels pending delivery of a route withdrawn by its source.
func (c *Convergence) RecordWithdraw(source int, prefix netip.Prefix) {
	c.recordWithdraw(source, makeRouteKey(prefix, ""), time.Time{})
}

func (c *Convergence) recordWithdraw(source int, route routeKey, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for destination := range c.peerCount {
		key := pendingKey{peer: destination, route: route}
		delete(c.earlyReceipts, key)
		entry, ok := c.pending[key]
		if !ok {
			continue
		}
		if entry.source == source {
			c.cancelPending(key, entry, at)
		}
	}
}

// Disconnect cancels pending delivery to and from the ended session.
func (c *Convergence) Disconnect(peer int) {
	c.disconnect(peer, time.Time{})
}

func (c *Convergence) disconnect(peer int, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.pending {
		if key.peer == peer || entry.source == peer {
			c.cancelPending(key, entry, at)
		}
	}
	for key := range c.earlyReceipts {
		if key.peer == peer {
			delete(c.earlyReceipts, key)
		}
	}
}

// CheckDeadline returns late deliveries and pending routes past the deadline.
func (c *Convergence) CheckDeadline(now time.Time) []SlowRoute {
	c.mu.Lock()
	defer c.mu.Unlock()

	slow := slices.Clone(c.late)
	for key, entry := range c.pending {
		age := now.Sub(entry.announceTime)
		if age >= c.deadline {
			slow = append(slow, slowRoute(key, entry, age, false))
		}
	}
	return slow
}

// Stats returns aggregate convergence statistics for all resolved entries.
func (c *Convergence) Stats() ConvergenceStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	return computeStats(c.latencies, len(c.pending))
}

// StatsByFamily returns per-family convergence statistics.
func (c *Convergence) StatsByFamily() map[string]ConvergenceStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make(map[string]ConvergenceStats, len(c.familyLatencies))
	for fam, lats := range c.familyLatencies {
		result[routeFamilyName(fam)] = computeStats(lats, 0)
	}
	return result
}

func computeStats(latencies []time.Duration, pending int) ConvergenceStats {
	stats := ConvergenceStats{
		Resolved: len(latencies),
		Pending:  pending,
	}

	if len(latencies) == 0 {
		return stats
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	slices.Sort(sorted)

	stats.Min = sorted[0]
	stats.Max = sorted[len(sorted)-1]

	var total time.Duration
	for _, d := range sorted {
		total += d
	}
	stats.Avg = total / time.Duration(len(sorted))

	p99Idx := (len(sorted) * 99 / 100)
	if p99Idx >= len(sorted) {
		p99Idx = len(sorted) - 1
	}
	stats.P99 = sorted[p99Idx]

	return stats
}
