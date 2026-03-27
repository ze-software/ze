package reactor

import (
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/test/sim"
)

// --- Weight tracker: WorstPeerRatio ---

// TestWeightTracker_WorstPeerRatio verifies that the peer with the highest
// overflow-to-demand ratio is identified as the worst offender.
//
// VALIDATES: AC-4 teardown target identification.
// PREVENTS: Teardown of the wrong peer.
func TestWeightTracker_WorstPeerRatio(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1:179", 100000, 1)
	wt.AddPeer("10.0.0.2:179", 50000, 1)
	wt.AddPeer("10.0.0.3:179", 200000, 1)

	depths := map[string]int{
		"10.0.0.1:179": 500,  // moderate
		"10.0.0.2:179": 1000, // high relative to weight
		"10.0.0.3:179": 100,  // low
	}

	addr, ratio := wt.WorstPeerRatio(depths)
	assert.Equal(t, "10.0.0.2:179", addr)
	assert.Greater(t, ratio, 0.0)

	// Peer 2 has the highest ratio because 1000 items vs smaller demand.
	demand2 := wt.PeerDemand("10.0.0.2:179")
	require.Greater(t, demand2, 0)
	expectedRatio := float64(1000) / float64(demand2)
	assert.InDelta(t, expectedRatio, ratio, 0.001)
}

// TestWeightTracker_WorstPeerRatioEmpty returns zero values with no overflow.
//
// VALIDATES: WorstPeerRatio edge case.
// PREVENTS: Panic on empty input.
func TestWeightTracker_WorstPeerRatioEmpty(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1:179", 100000, 1)

	addr, ratio := wt.WorstPeerRatio(map[string]int{})
	assert.Equal(t, "", addr)
	assert.Equal(t, 0.0, ratio)
}

// --- Congestion controller: ShouldDeny ---

func makeCongestionController(poolRatio float64, depths map[string]int, wt *weightTracker) *congestionController {
	return newCongestionController(congestionConfig{
		gracePeriod:    5 * time.Second,
		poolUsedRatio:  func() float64 { return poolRatio },
		overflowDepths: func() map[string]int { return depths },
		weights:        wt,
		clock:          clock.RealClock{},
	})
}

// TestCongestion_ShouldDenyHighRatio verifies buffer denial for the worst peer
// when pool usage exceeds the denial threshold.
//
// VALIDATES: AC-2 buffer denial for highest usage-to-weight ratio peer.
// PREVENTS: Unbounded buffer consumption by one peer.
func TestCongestion_ShouldDenyHighRatio(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1:179", 10000, 1)
	wt.AddPeer("10.0.0.2:179", 10000, 1)

	depths := map[string]int{
		"10.0.0.1:179": 500, // high
		"10.0.0.2:179": 10,  // low
	}

	var denied atomic.Int64
	cc := newCongestionController(congestionConfig{
		gracePeriod:    5 * time.Second,
		poolUsedRatio:  func() float64 { return 0.85 }, // above 80% threshold
		overflowDepths: func() map[string]int { return depths },
		weights:        wt,
		clock:          clock.RealClock{},
		onDenied:       func() { denied.Add(1) },
	})

	// Worst peer should be denied.
	assert.True(t, cc.ShouldDeny("10.0.0.1:179"))
	assert.Equal(t, int64(1), denied.Load())

	// Non-worst peer should NOT be denied.
	assert.False(t, cc.ShouldDeny("10.0.0.2:179"))
	assert.Equal(t, int64(1), denied.Load()) // no additional denial
}

// TestCongestion_ShouldDenyBelowThreshold verifies no denial when pool is healthy.
//
// VALIDATES: AC-12 fast peers unaffected during normal operation.
// PREVENTS: False denial when pool has headroom.
func TestCongestion_ShouldDenyBelowThreshold(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1:179", 10000, 1)

	depths := map[string]int{"10.0.0.1:179": 500}

	cc := makeCongestionController(0.50, depths, wt) // well below 80%

	assert.False(t, cc.ShouldDeny("10.0.0.1:179"))
}

// TestCongestion_ShouldDenyNilController verifies nil controller never denies.
//
// VALIDATES: Safe nil handling.
// PREVENTS: Nil pointer panic.
func TestCongestion_ShouldDenyNilController(t *testing.T) {
	t.Parallel()

	var cc *congestionController
	assert.False(t, cc.ShouldDeny("10.0.0.1:179"))
}

// TestCongestion_FastPeerUnaffected verifies a peer with low overflow ratio
// is never denied, even when the pool is under heavy pressure.
//
// VALIDATES: AC-12 fast destination peers unaffected during congestion.
// PREVENTS: Innocent peers penalized for another peer's congestion.
func TestCongestion_FastPeerUnaffected(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("slow:179", 10000, 1)
	wt.AddPeer("fast:179", 10000, 1)

	depths := map[string]int{
		"slow:179": 1000, // congested
		"fast:179": 0,    // healthy
	}

	cc := makeCongestionController(0.99, depths, wt) // critical pool level

	assert.True(t, cc.ShouldDeny("slow:179"))
	assert.False(t, cc.ShouldDeny("fast:179"))
}

// --- Congestion controller: CheckTeardown ---

// TestCongestion_ForcedTeardownFires verifies teardown fires after grace period
// when all conditions are met.
//
// VALIDATES: AC-4 forced teardown on pool exhaustion.
// PREVENTS: One peer freezing the entire system.
func TestCongestion_ForcedTeardownFires(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1:179", 1000, 1) // small peer, low demand

	depths := map[string]int{"10.0.0.1:179": 500} // way over 2x weight

	fc := sim.NewFakeClock(time.Now())
	var tornDown atomic.Int64
	var lastGR atomic.Bool

	cc := newCongestionController(congestionConfig{
		gracePeriod:    5 * time.Second,
		poolUsedRatio:  func() float64 { return 0.97 }, // above 95%
		overflowDepths: func() map[string]int { return depths },
		weights:        wt,
		clock:          fc,
		peerGRCapable:  func(string) bool { return false },
		onTeardown: func(_ netip.AddrPort, gr bool) {
			tornDown.Add(1)
			lastGR.Store(gr)
		},
		onTeardownFired: func() {},
	})

	addr := netip.MustParseAddrPort("10.0.0.1:179")

	// First check: starts grace timer.
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(0), tornDown.Load())

	// Advance 3 seconds: still within grace.
	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(0), tornDown.Load())

	// Advance past grace period.
	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(1), tornDown.Load())
	assert.False(t, lastGR.Load())
}

// TestCongestion_TeardownGracePeriodResets verifies grace period resets when
// conditions clear.
//
// VALIDATES: AC-4 grace period prevents false teardowns.
// PREVENTS: Stale grace timer firing after conditions improve.
func TestCongestion_TeardownGracePeriodResets(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1:179", 1000, 1)

	overflowing := true
	depths := func() map[string]int {
		if overflowing {
			return map[string]int{"10.0.0.1:179": 500}
		}
		return map[string]int{}
	}

	poolRatio := 0.97
	fc := sim.NewFakeClock(time.Now())
	var tornDown atomic.Int64

	cc := newCongestionController(congestionConfig{
		gracePeriod:    5 * time.Second,
		poolUsedRatio:  func() float64 { return poolRatio },
		overflowDepths: depths,
		weights:        wt,
		clock:          fc,
		onTeardown:     func(_ netip.AddrPort, _ bool) { tornDown.Add(1) },
	})

	addr := netip.MustParseAddrPort("10.0.0.1:179")

	// Start grace.
	cc.CheckTeardown(addr)
	fc.Add(3 * time.Second)

	// Conditions clear (pool drops below threshold).
	poolRatio = 0.50
	cc.CheckTeardown(addr)

	// Conditions return.
	poolRatio = 0.97
	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr) // This restarts grace from now.

	// 3s more is not enough (need 5 from the restart).
	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(0), tornDown.Load())

	// 2 more seconds: now past the restarted grace.
	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(1), tornDown.Load())
}

// TestCongestion_TeardownGRCapable verifies GR-capable peers are reported
// correctly to the teardown callback.
//
// VALIDATES: AC-5 GR-capable teardown uses TCP close without NOTIFICATION.
// PREVENTS: Sending NOTIFICATION to GR peers (which would delete routes).
func TestCongestion_TeardownGRCapable(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1:179", 1000, 1)

	depths := map[string]int{"10.0.0.1:179": 500}
	fc := sim.NewFakeClock(time.Now())
	var lastGR atomic.Bool

	cc := newCongestionController(congestionConfig{
		gracePeriod:    1 * time.Millisecond, // short for test
		poolUsedRatio:  func() float64 { return 0.97 },
		overflowDepths: func() map[string]int { return depths },
		weights:        wt,
		clock:          fc,
		peerGRCapable:  func(string) bool { return true },
		onTeardown: func(_ netip.AddrPort, gr bool) {
			lastGR.Store(gr)
		},
	})

	addr := netip.MustParseAddrPort("10.0.0.1:179")
	cc.CheckTeardown(addr) // start grace
	fc.Add(time.Second)
	cc.CheckTeardown(addr) // fire

	assert.True(t, lastGR.Load())
}

// TestCongestion_TeardownNotWorstPeer verifies that teardown does not fire
// when the failing peer is not the worst offender.
//
// VALIDATES: AC-4 teardown targets worst peer only.
// PREVENTS: Teardown of a healthy peer due to another peer's congestion.
func TestCongestion_TeardownNotWorstPeer(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1:179", 1000, 1) // worst
	wt.AddPeer("10.0.0.2:179", 1000, 1)

	depths := map[string]int{
		"10.0.0.1:179": 500,
		"10.0.0.2:179": 10,
	}

	fc := sim.NewFakeClock(time.Now())
	var tornDown atomic.Int64

	cc := newCongestionController(congestionConfig{
		gracePeriod:    1 * time.Millisecond,
		poolUsedRatio:  func() float64 { return 0.97 },
		overflowDepths: func() map[string]int { return depths },
		weights:        wt,
		clock:          fc,
		onTeardown:     func(_ netip.AddrPort, _ bool) { tornDown.Add(1) },
	})

	// Peer 2 fails but is NOT the worst -- no teardown.
	notWorst := netip.MustParseAddrPort("10.0.0.2:179")
	cc.CheckTeardown(notWorst)
	fc.Add(time.Second)
	cc.CheckTeardown(notWorst)
	assert.Equal(t, int64(0), tornDown.Load())
}

// TestCongestion_NilCheckTeardown verifies nil controller is safe.
//
// VALIDATES: Safe nil handling for CheckTeardown.
// PREVENTS: Nil pointer panic in worker loop.
func TestCongestion_NilCheckTeardown(t *testing.T) {
	t.Parallel()

	var cc *congestionController
	cc.CheckTeardown(netip.MustParseAddrPort("10.0.0.1:179")) // must not panic
}
