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

// SlowRoute describes a route that exceeded the convergence deadline.
type SlowRoute struct {
	Source int
	Peer   int
	Prefix netip.Prefix
	Age    time.Duration
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
// Thread-safe for concurrent RecordAnnounce and RecordReceive calls.
type Convergence struct {
	mu        sync.Mutex
	peerCount int
	deadline  time.Duration

	// pending maps (peer, prefix) → announcement info for unresolved entries.
	pending map[pendingKey]pendingEntry

	// latencies stores all resolved propagation durations.
	latencies []time.Duration

	// familyLatencies stores per-family resolved latencies alongside aggregate.
	familyLatencies map[string][]time.Duration
}

// NewConvergence creates a convergence tracker for n peers with the given
// propagation deadline.
func NewConvergence(n int, deadline time.Duration) *Convergence {
	return &Convergence{
		peerCount:       n,
		deadline:        deadline,
		pending:         make(map[pendingKey]pendingEntry),
		familyLatencies: make(map[string][]time.Duration),
	}
}

// RecordAnnounce records that a peer announced a route at the given time.
// Creates pending entries for all other peers (they should receive it).
// If family is provided, per-family latency is tracked when resolved.
func (c *Convergence) RecordAnnounce(source int, prefix netip.Prefix, at time.Time, family ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fam := ""
	if len(family) > 0 {
		fam = family[0]
	}

	for peer := range c.peerCount {
		if peer == source {
			continue
		}
		key := pendingKey{peer: peer, prefix: prefix}
		c.pending[key] = pendingEntry{source: source, announceTime: at, family: fam}
	}
}

// RecordReceive records that a peer received a route at the given time.
// If a pending entry exists for this (peer, prefix), it is resolved and
// the latency recorded (both aggregate and per-family when family was set).
func (c *Convergence) RecordReceive(peer int, prefix netip.Prefix, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := pendingKey{peer: peer, prefix: prefix}
	entry, ok := c.pending[key]
	if !ok {
		return
	}

	latency := at.Sub(entry.announceTime)
	c.latencies = append(c.latencies, latency)
	if entry.family != "" {
		c.familyLatencies[entry.family] = append(c.familyLatencies[entry.family], latency)
	}
	delete(c.pending, key)
}

// CheckDeadline returns all pending entries that have exceeded the
// convergence deadline as of the given time.
func (c *Convergence) CheckDeadline(now time.Time) []SlowRoute {
	c.mu.Lock()
	defer c.mu.Unlock()

	var slow []SlowRoute
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
