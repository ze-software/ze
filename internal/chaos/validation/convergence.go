// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"net/netip"
	"slices"
	"sync"
	"time"
)

// pendingKey identifies a specific route-to-peer expectation.
type pendingKey struct {
	peer   int
	prefix netip.Prefix
}

// pendingEntry records when a route was announced and which peer sourced it.
type pendingEntry struct {
	source       int
	announceTime time.Time
	family       string
}

// earlyReceiptLimit bounds unmatched receipts. A simulator can publish only
// one announcement at a time, so the limit leaves room for every peer pair in
// the largest CLI scenario as well as unrelated receipts.
const earlyReceiptLimit = 4096

// SlowRoute describes a route that exceeded the convergence deadline.
type SlowRoute struct {
	Source   int
	Peer     int
	Prefix   netip.Prefix
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

	// pending maps (peer, prefix) → announcement info for unresolved entries.
	pending map[pendingKey]pendingEntry
	// earlyReceipts holds arrivals seen before their announcement was published.
	earlyReceipts map[pendingKey]time.Time

	// latencies stores all resolved propagation durations.
	latencies []time.Duration

	// late retains completed deliveries that missed their deadline.
	late []SlowRoute

	// familyLatencies stores per-family resolved latencies alongside aggregate.
	familyLatencies map[string][]time.Duration
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
		familyLatencies: make(map[string][]time.Duration),
	}
}

// RecordAnnounce records that a peer announced a route at the given time.
// Creates pending entries for peers eligible to receive it.
// If family is provided, per-family latency is tracked when resolved.
func (c *Convergence) RecordAnnounce(source int, prefix netip.Prefix, at time.Time, family ...string) {
	if source < 0 {
		return
	}
	if source >= c.peerCount {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	fam := ""
	if len(family) > 0 {
		fam = family[0]
	}

	for peer := range c.peerCount {
		if !c.reachable.allows(source, peer, fam) {
			continue
		}
		key := pendingKey{peer: peer, prefix: prefix}
		entry := pendingEntry{source: source, announceTime: at, family: fam}
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
	if peer < 0 {
		return
	}
	if peer >= c.peerCount {
		return
	}
	if !prefix.IsValid() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key := pendingKey{peer: peer, prefix: prefix}
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
	if len(c.earlyReceipts) >= earlyReceiptLimit {
		clear(c.earlyReceipts)
	}
	c.earlyReceipts[key] = at
}

func (c *Convergence) recordDelivery(key pendingKey, entry pendingEntry, at time.Time) {
	latency := at.Sub(entry.announceTime)
	c.latencies = append(c.latencies, latency)
	if latency >= c.deadline {
		c.late = append(c.late, SlowRoute{
			Source:   entry.source,
			Peer:     key.peer,
			Prefix:   key.prefix,
			Age:      latency,
			Received: true,
		})
	}
	if entry.family != "" {
		c.familyLatencies[entry.family] = append(c.familyLatencies[entry.family], latency)
	}
	delete(c.pending, key)
}

// RecordWithdraw cancels pending delivery of a route withdrawn by its source.
func (c *Convergence) RecordWithdraw(source int, prefix netip.Prefix) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for destination := range c.peerCount {
		key := pendingKey{peer: destination, prefix: prefix}
		delete(c.earlyReceipts, key)
		if entry, ok := c.pending[key]; ok && entry.source == source {
			delete(c.pending, key)
		}
	}
}

// Disconnect cancels pending delivery to and from the ended session.
func (c *Convergence) Disconnect(peer int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.pending {
		if key.peer == peer || entry.source == peer {
			delete(c.pending, key)
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
			slow = append(slow, SlowRoute{
				Source: entry.source,
				Peer:   key.peer,
				Prefix: key.prefix,
				Age:    age,
			})
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
		result[fam] = computeStats(lats, 0)
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
