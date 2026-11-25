package validation

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvergenceRecordAndResolve verifies that recording an announcement
// and then resolving it computes the correct latency.
//
// VALIDATES: Announce + Resolve produces non-zero latency.
// PREVENTS: Lost timing data or incorrect latency computation.
func TestConvergenceRecordAndResolve(t *testing.T) {
	c := NewConvergence(2, 5*time.Second)

	announceTime := time.Now()
	c.RecordAnnounce(0, p("10.0.0.0/24"), announceTime)

	// Simulate 100ms propagation delay.
	resolveTime := announceTime.Add(100 * time.Millisecond)
	c.RecordReceive(1, p("10.0.0.0/24"), resolveTime)

	stats := c.Stats()
	require.Equal(t, 1, stats.Resolved)
	assert.Equal(t, 0, stats.Pending)
	assert.InDelta(t, 100.0, float64(stats.Min/time.Millisecond), 1.0)
}

// TestConvergenceReceiveBeforeAnnounce checks that event order does not change latency.
//
// VALIDATES: A receipt observed before the announcement still resolves at its wire time.
// PREVENTS: A late delivered route appearing absent or timely.
func TestConvergenceReceiveBeforeAnnounce(t *testing.T) {
	c := NewConvergence(2, time.Second)
	at := time.Now()
	prefix := p("10.0.0.0/24")
	c.RecordReceive(1, prefix, at.Add(2*time.Second))
	c.RecordAnnounce(0, prefix, at, "ipv4/unicast")

	assert.Equal(t, []SlowRoute{
		{Source: 0, Peer: 1, Prefix: prefix, Age: 2 * time.Second, Received: true},
	}, c.CheckDeadline(at.Add(3*time.Second)))
	stats := c.StatsByFamily()["ipv4/unicast"]
	assert.Equal(t, 1, stats.Resolved)
	assert.Equal(t, 2*time.Second, stats.Min)
}

// TestConvergenceStaleReceive checks that only an arrival after a send clears it.
//
// VALIDATES: An earlier receive is rejected on either side of announcement processing.
// PREVENTS: A prior version of a route satisfying a new convergence deadline.
func TestConvergenceStaleReceive(t *testing.T) {
	c := NewConvergence(2, time.Second)
	at := time.Now()
	prefix := p("10.0.0.0/24")
	c.RecordReceive(1, prefix, at.Add(-2*time.Second))
	c.RecordAnnounce(0, prefix, at)
	c.RecordReceive(1, prefix, at.Add(-time.Second))

	assert.Equal(t, []SlowRoute{
		{Source: 0, Peer: 1, Prefix: prefix, Age: time.Second},
	}, c.CheckDeadline(at.Add(time.Second)))
}

// TestConvergenceEarlyReceiptInvalidated checks session and route boundaries.
//
// VALIDATES: Withdrawal and disconnect invalidate an unmatched receipt.
// PREVENTS: A receipt from a removed route or prior session resolving a new send.
func TestConvergenceEarlyReceiptInvalidated(t *testing.T) {
	prefix := p("10.0.0.0/24")
	cases := []struct {
		name       string
		invalidate func(*Convergence)
	}{
		{name: "withdrawal", invalidate: func(c *Convergence) { c.RecordWithdraw(0, prefix) }},
		{name: "disconnect", invalidate: func(c *Convergence) { c.Disconnect(1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewConvergence(2, time.Second)
			at := time.Now()
			c.RecordReceive(1, prefix, at.Add(time.Millisecond))
			tc.invalidate(c)

			c.RecordAnnounce(0, prefix, at)
			assert.Equal(t, []SlowRoute{
				{Source: 0, Peer: 1, Prefix: prefix, Age: time.Second},
			}, c.CheckDeadline(at.Add(time.Second)))
		})
	}
}

// TestConvergenceMultiPeerResolve verifies that a route announced by peer 0
// creates pending entries for peers 1 and 2, both resolved independently.
//
// VALIDATES: One announcement creates entries for all other peers.
// PREVENTS: Missing entries for non-source peers.
func TestConvergenceMultiPeerResolve(t *testing.T) {
	c := NewConvergence(3, 5*time.Second)

	announceTime := time.Now()
	c.RecordAnnounce(0, p("10.0.0.0/24"), announceTime)

	// Peer 1 receives after 50ms, peer 2 after 200ms.
	c.RecordReceive(1, p("10.0.0.0/24"), announceTime.Add(50*time.Millisecond))
	c.RecordReceive(2, p("10.0.0.0/24"), announceTime.Add(200*time.Millisecond))

	stats := c.Stats()
	assert.Equal(t, 2, stats.Resolved)
	assert.Equal(t, 0, stats.Pending)
	assert.InDelta(t, 50.0, float64(stats.Min/time.Millisecond), 1.0)
	assert.InDelta(t, 200.0, float64(stats.Max/time.Millisecond), 1.0)
	assert.InDelta(t, 125.0, float64(stats.Avg/time.Millisecond), 1.0)
}

// TestConvergenceDeadlineExceeded verifies that routes exceeding the
// convergence deadline are flagged.
//
// VALIDATES: Slow routes detected via CheckDeadline.
// PREVENTS: Silent convergence failures going undetected.
func TestConvergenceDeadlineExceeded(t *testing.T) {
	deadline := 100 * time.Millisecond
	c := NewConvergence(2, deadline)

	announceTime := time.Now()
	c.RecordAnnounce(0, p("10.0.0.0/24"), announceTime)

	// Simulate checking after deadline has passed, without resolution.
	slow := c.CheckDeadline(announceTime.Add(200 * time.Millisecond))
	assert.Equal(t, 1, len(slow))
}

// TestConvergenceDeadlineNotExceeded verifies that fast routes are not
// flagged by CheckDeadline.
//
// VALIDATES: Fast routes not falsely flagged.
// PREVENTS: False deadline violations on timely propagation.
func TestConvergenceDeadlineNotExceeded(t *testing.T) {
	deadline := 5 * time.Second
	c := NewConvergence(2, deadline)

	announceTime := time.Now()
	c.RecordAnnounce(0, p("10.0.0.0/24"), announceTime)

	// Resolve before deadline.
	c.RecordReceive(1, p("10.0.0.0/24"), announceTime.Add(100*time.Millisecond))

	slow := c.CheckDeadline(announceTime.Add(6 * time.Second))
	assert.Equal(t, 0, len(slow))
}

// TestConvergenceStatsP99 verifies p99 latency computation with enough
// data points.
//
// VALIDATES: P99 is computed from sorted latencies.
// PREVENTS: Incorrect percentile calculation.
func TestConvergenceStatsP99(t *testing.T) {
	c := NewConvergence(2, 5*time.Second)

	base := time.Now()
	// Generate 100 entries with latencies 1ms, 2ms, ..., 100ms.
	for i := range 100 {
		announceTime := base.Add(time.Duration(i) * time.Second)
		c.RecordAnnounce(0, p("10.0."+itoa(i)+".0/24"), announceTime)
		latency := time.Duration(i+1) * time.Millisecond
		c.RecordReceive(1, p("10.0."+itoa(i)+".0/24"), announceTime.Add(latency))
	}

	stats := c.Stats()
	assert.Equal(t, 100, stats.Resolved)
	// P99 of 1..100ms should be ~99ms.
	assert.InDelta(t, 99.0, float64(stats.P99/time.Millisecond), 2.0)
}

// TestConvergenceSourcePeerIgnored verifies that RecordReceive for the
// source peer (the one that announced) has no effect.
//
// VALIDATES: Source peer doesn't resolve its own announcement.
// PREVENTS: Self-receive counting as a valid propagation.
func TestConvergenceSourcePeerIgnored(t *testing.T) {
	c := NewConvergence(3, 5*time.Second)

	announceTime := time.Now()
	c.RecordAnnounce(0, p("10.0.0.0/24"), announceTime)

	// Source peer "receives" its own route — should be ignored.
	c.RecordReceive(0, p("10.0.0.0/24"), announceTime.Add(10*time.Millisecond))

	stats := c.Stats()
	// Still 2 pending (for peers 1 and 2), none resolved.
	assert.Equal(t, 0, stats.Resolved)
	assert.Equal(t, 2, stats.Pending)
}

// TestConvergenceConcurrent verifies thread-safety under concurrent
// announce and receive operations.
//
// VALIDATES: Concurrent operations don't corrupt state.
// PREVENTS: Data race on internal maps.
func TestConvergenceConcurrent(t *testing.T) {
	c := NewConvergence(4, 5*time.Second)
	base := time.Now()

	var wg sync.WaitGroup
	// 4 goroutines, each announcing 50 routes from their peer.
	for peer := range 4 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for i := range 50 {
				prefix := p("10." + itoa(idx) + "." + itoa(i) + ".0/24")
				announceTime := base.Add(time.Duration(i) * time.Millisecond)
				c.RecordAnnounce(idx, prefix, announceTime)
			}
		}(peer)
	}
	wg.Wait()

	// Now resolve all from peer 0's perspective.
	for peer := 1; peer < 4; peer++ {
		for i := range 50 {
			prefix := p("10.0." + itoa(i) + ".0/24")
			c.RecordReceive(peer, prefix, base.Add(time.Duration(i+100)*time.Millisecond))
		}
	}

	stats := c.Stats()
	// Peer 0 announced 50 routes, each expected at 3 other peers = 150 resolutions.
	assert.Equal(t, 150, stats.Resolved)
}

// TestConvergenceZeroDeadline verifies boundary: deadline of zero means
// everything is immediately overdue.
//
// VALIDATES: Zero deadline boundary.
// PREVENTS: Off-by-one in deadline comparison.
func TestConvergenceZeroDeadline(t *testing.T) {
	c := NewConvergence(2, 0)

	announceTime := time.Now()
	c.RecordAnnounce(0, p("10.0.0.0/24"), announceTime)

	// Even checking at the exact announce time should flag it.
	slow := c.CheckDeadline(announceTime)
	assert.Equal(t, 1, len(slow))
}

// TestConvergencePerFamily verifies that per-family latency stats are tracked
// alongside the aggregate when family is provided to RecordAnnounce.
func TestConvergencePerFamily(t *testing.T) {
	c := NewConvergence(2, 5*time.Second)
	base := time.Now()

	// Announce IPv4 routes with family tag.
	c.RecordAnnounce(0, p("10.0.0.0/24"), base, "ipv4/unicast")
	c.RecordAnnounce(0, p("10.0.1.0/24"), base, "ipv4/unicast")

	// Announce IPv6 route with family tag.
	c.RecordAnnounce(0, p("2001:db8::/32"), base, "ipv6/unicast")

	// Announce route without family (aggregate only).
	c.RecordAnnounce(0, p("10.0.2.0/24"), base)

	// Resolve all with different latencies.
	c.RecordReceive(1, p("10.0.0.0/24"), base.Add(50*time.Millisecond))
	c.RecordReceive(1, p("10.0.1.0/24"), base.Add(100*time.Millisecond))
	c.RecordReceive(1, p("2001:db8::/32"), base.Add(200*time.Millisecond))
	c.RecordReceive(1, p("10.0.2.0/24"), base.Add(75*time.Millisecond))

	// Aggregate stats include all 4.
	stats := c.Stats()
	assert.Equal(t, 4, stats.Resolved)

	// Per-family stats.
	byFamily := c.StatsByFamily()
	require.Contains(t, byFamily, "ipv4/unicast")
	require.Contains(t, byFamily, "ipv6/unicast")

	ipv4 := byFamily["ipv4/unicast"]
	assert.Equal(t, 2, ipv4.Resolved)
	assert.InDelta(t, 50.0, float64(ipv4.Min/time.Millisecond), 1.0)
	assert.InDelta(t, 100.0, float64(ipv4.Max/time.Millisecond), 1.0)
	assert.InDelta(t, 75.0, float64(ipv4.Avg/time.Millisecond), 1.0)

	ipv6 := byFamily["ipv6/unicast"]
	assert.Equal(t, 1, ipv6.Resolved)
	assert.InDelta(t, 200.0, float64(ipv6.Min/time.Millisecond), 1.0)

	// No-family route should not appear in per-family breakdown.
	_, hasEmpty := byFamily[""]
	assert.False(t, hasEmpty, "empty family should not appear in StatsByFamily")
}

// TestConvergenceReachability validates deadlines under a directional export policy.
//
// VALIDATES: Each pending route names an eligible destination other than its source.
// PREVENTS: Reporting an iBGP route missing at an eBGP destination.
func TestConvergenceReachability(t *testing.T) {
	c := NewConvergenceWithReachability(4, time.Second, mixedPeerReachability)
	at := time.Now()
	ibgpRoute := p("10.0.0.0/24")
	ebgpRoute := p("10.0.2.0/24")
	c.RecordAnnounce(0, ibgpRoute, at)
	c.RecordAnnounce(2, ebgpRoute, at)

	assert.ElementsMatch(t, []SlowRoute{
		{Source: 0, Peer: 1, Prefix: ibgpRoute, Age: time.Second},
		{Source: 2, Peer: 0, Prefix: ebgpRoute, Age: time.Second},
		{Source: 2, Peer: 1, Prefix: ebgpRoute, Age: time.Second},
		{Source: 2, Peer: 3, Prefix: ebgpRoute, Age: time.Second},
	}, c.CheckDeadline(at.Add(time.Second)))
}

// TestConvergenceFamilyReachability validates family-specific delivery expectations.
//
// VALIDATES: The family of an announcement limits its eligible destinations.
// PREVENTS: Reporting overdue delivery to a peer that does not support the family.
func TestConvergenceFamilyReachability(t *testing.T) {
	c := NewConvergenceWithReachability(3, time.Second, func(_, destination int, family string) bool {
		return destination != 2 || family == "ipv4/unicast"
	})
	at := time.Now()
	ipv4 := p("10.0.0.0/24")
	ipv6 := p("2001:db8::/32")
	c.RecordAnnounce(0, ipv4, at, "ipv4/unicast")
	c.RecordAnnounce(0, ipv6, at, "ipv6/unicast")

	assert.ElementsMatch(t, []SlowRoute{
		{Source: 0, Peer: 1, Prefix: ipv4, Age: time.Second},
		{Source: 0, Peer: 1, Prefix: ipv6, Age: time.Second},
		{Source: 0, Peer: 2, Prefix: ipv4, Age: time.Second},
	}, c.CheckDeadline(at.Add(time.Second)))
}

// TestConvergenceWithdrawal validates cancellation of the withdrawn route's delivery.
//
// VALIDATES: Other pending routes still have a deadline.
// PREVENTS: Treating a withdrawn route as missing or dropping unrelated obligations.
func TestConvergenceWithdrawal(t *testing.T) {
	c := NewConvergence(2, time.Second)
	at := time.Now()
	withdrawn := p("10.0.0.0/24")
	active := p("10.0.1.0/24")
	c.RecordAnnounce(0, withdrawn, at)
	c.RecordAnnounce(0, active, at)
	c.RecordWithdraw(0, withdrawn)

	assert.Equal(t, []SlowRoute{
		{Source: 0, Peer: 1, Prefix: active, Age: time.Second},
	}, c.CheckDeadline(at.Add(time.Second)))
}

// TestConvergenceDisconnect validates cancellation of pending routes for the ended session.
//
// VALIDATES: Both incoming and outgoing obligations for the disconnecting peer are removed.
// PREVENTS: Judging routes from or to an ended session against a deadline.
func TestConvergenceDisconnect(t *testing.T) {
	c := NewConvergence(3, time.Second)
	at := time.Now()
	first := p("10.0.0.0/24")
	second := p("10.0.1.0/24")
	c.RecordAnnounce(0, first, at)
	c.RecordAnnounce(1, second, at)
	c.Disconnect(1)

	assert.Equal(t, []SlowRoute{
		{Source: 0, Peer: 2, Prefix: first, Age: time.Second},
	}, c.CheckDeadline(at.Add(time.Second)))
}

// TestConvergenceLateReceive validates the deadline after a route eventually arrives.
//
// VALIDATES: A delivered route retains its observed latency as a deadline violation.
// PREVENTS: Clearing a convergence failure when the matching receipt is processed.
func TestConvergenceLateReceive(t *testing.T) {
	c := NewConvergence(2, time.Second)
	at := time.Now()
	route := p("10.0.0.0/24")
	c.RecordAnnounce(0, route, at)
	c.RecordReceive(1, route, at.Add(2*time.Second))

	assert.Equal(t, []SlowRoute{
		{Source: 0, Peer: 1, Prefix: route, Age: 2 * time.Second, Received: true},
	}, c.CheckDeadline(at.Add(3*time.Second)))
}
