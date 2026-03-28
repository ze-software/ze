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
// Uses IP-only keys matching production format (peerAddrLabel + OverflowDepths).
//
// VALIDATES: AC-4 teardown target identification.
// PREVENTS: Teardown of the wrong peer.
func TestWeightTracker_WorstPeerRatio(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1", 100000, 1)
	wt.AddPeer("10.0.0.2", 50000, 1)
	wt.AddPeer("10.0.0.3", 200000, 1)

	depths := map[string]int{
		"10.0.0.1": 500,  // moderate
		"10.0.0.2": 1000, // high relative to weight
		"10.0.0.3": 100,  // low
	}

	addr, ratio := wt.WorstPeerRatio(depths)
	assert.Equal(t, "10.0.0.2", addr)
	assert.Greater(t, ratio, 0.0)

	demand2 := wt.PeerDemand("10.0.0.2")
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
	wt.AddPeer("10.0.0.1", 100000, 1)

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
	wt.AddPeer("10.0.0.1", 10000, 1)
	wt.AddPeer("10.0.0.2", 10000, 1)

	depths := map[string]int{
		"10.0.0.1": 500, // high
		"10.0.0.2": 10,  // low
	}

	var denied atomic.Int64
	cc := newCongestionController(congestionConfig{
		gracePeriod:    5 * time.Second,
		poolUsedRatio:  func() float64 { return 0.85 },
		overflowDepths: func() map[string]int { return depths },
		weights:        wt,
		clock:          clock.RealClock{},
		onDenied:       func() { denied.Add(1) },
	})

	assert.True(t, cc.ShouldDeny("10.0.0.1"))
	assert.Equal(t, int64(1), denied.Load())

	assert.False(t, cc.ShouldDeny("10.0.0.2"))
	assert.Equal(t, int64(1), denied.Load())
}

// TestCongestion_ShouldDenyBelowThreshold verifies no denial when pool is healthy.
//
// VALIDATES: AC-12 fast peers unaffected during normal operation.
// PREVENTS: False denial when pool has headroom.
func TestCongestion_ShouldDenyBelowThreshold(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1", 10000, 1)

	depths := map[string]int{"10.0.0.1": 500}
	cc := makeCongestionController(0.50, depths, wt)

	assert.False(t, cc.ShouldDeny("10.0.0.1"))
}

// TestCongestion_ShouldDenyNilController verifies nil controller never denies.
//
// VALIDATES: Safe nil handling.
// PREVENTS: Nil pointer panic.
func TestCongestion_ShouldDenyNilController(t *testing.T) {
	t.Parallel()

	var cc *congestionController
	assert.False(t, cc.ShouldDeny("10.0.0.1"))
}

// TestCongestion_FastPeerUnaffected verifies a peer with low overflow ratio
// is never denied, even when the pool is under heavy pressure.
//
// VALIDATES: AC-12 fast destination peers unaffected during congestion.
// PREVENTS: Innocent peers penalized for another peer's congestion.
func TestCongestion_FastPeerUnaffected(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1", 10000, 1)
	wt.AddPeer("10.0.0.2", 10000, 1)

	depths := map[string]int{
		"10.0.0.1": 1000, // congested
		"10.0.0.2": 0,    // healthy
	}

	cc := makeCongestionController(0.99, depths, wt)

	assert.True(t, cc.ShouldDeny("10.0.0.1"))
	assert.False(t, cc.ShouldDeny("10.0.0.2"))
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
	wt.AddPeer("10.0.0.1", 1000, 1)

	depths := map[string]int{"10.0.0.1": 500}

	fc := sim.NewFakeClock(time.Now())
	var tornDown atomic.Int64
	var lastGR atomic.Bool

	cc := newCongestionController(congestionConfig{
		gracePeriod:    5 * time.Second,
		poolUsedRatio:  func() float64 { return 0.97 },
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

	cc.CheckTeardown(addr)
	assert.Equal(t, int64(0), tornDown.Load())

	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(0), tornDown.Load())

	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(1), tornDown.Load())
	assert.False(t, lastGR.Load())
}

// TestCongestion_TeardownGracePeriodResets verifies grace period resets when
// conditions clear (pool drops below threshold).
//
// VALIDATES: AC-4 grace period prevents false teardowns.
// PREVENTS: Stale grace timer firing after conditions improve.
func TestCongestion_TeardownGracePeriodResets(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1", 1000, 1)

	overflowing := true
	depths := func() map[string]int {
		if overflowing {
			return map[string]int{"10.0.0.1": 500}
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

	cc.CheckTeardown(addr)
	fc.Add(3 * time.Second)

	// Pool drops below threshold -- grace clears.
	poolRatio = 0.50
	cc.CheckTeardown(addr)

	// Conditions return.
	poolRatio = 0.97
	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr)

	fc.Add(3 * time.Second)
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(0), tornDown.Load())

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
	wt.AddPeer("10.0.0.1", 1000, 1)

	depths := map[string]int{"10.0.0.1": 500}
	fc := sim.NewFakeClock(time.Now())
	var lastGR atomic.Bool

	cc := newCongestionController(congestionConfig{
		gracePeriod:    1 * time.Second, // must be >= 1s (constructor clamps to 5s default below 1s)
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
	cc.CheckTeardown(addr)
	fc.Add(2 * time.Second) // exceed 1s grace period
	cc.CheckTeardown(addr)

	assert.True(t, lastGR.Load())
}

// TestCongestion_TeardownNotWorstPeer verifies that teardown does not fire
// when the failing peer is not the worst offender, and does NOT reset
// the grace timer for the actual worst peer (finding 5).
//
// VALIDATES: AC-4 teardown targets worst peer only.
// PREVENTS: Teardown of a healthy peer; non-worst workers resetting grace.
func TestCongestion_TeardownNotWorstPeer(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1", 1000, 1) // worst
	wt.AddPeer("10.0.0.2", 1000, 1)

	depths := map[string]int{
		"10.0.0.1": 500,
		"10.0.0.2": 10,
	}

	fc := sim.NewFakeClock(time.Now())
	var tornDown atomic.Int64

	cc := newCongestionController(congestionConfig{
		gracePeriod:    5 * time.Second,
		poolUsedRatio:  func() float64 { return 0.97 },
		overflowDepths: func() map[string]int { return depths },
		weights:        wt,
		clock:          fc,
		onTeardown:     func(_ netip.AddrPort, _ bool) { tornDown.Add(1) },
	})

	worst := netip.MustParseAddrPort("10.0.0.1:179")
	notWorst := netip.MustParseAddrPort("10.0.0.2:179")

	// Start grace for worst peer.
	cc.CheckTeardown(worst)

	// Non-worst peer calls CheckTeardown -- must NOT reset grace.
	fc.Add(3 * time.Second)
	cc.CheckTeardown(notWorst)

	// Worst peer calls again after total 6s -- should fire (grace was NOT reset).
	fc.Add(3 * time.Second)
	cc.CheckTeardown(worst)
	assert.Equal(t, int64(1), tornDown.Load())
}

// TestCongestion_TeardownRatioBelowThreshold verifies no teardown when peer
// is worst but ratio < 2x (finding 7).
//
// VALIDATES: AC-4 ratio threshold enforcement.
// PREVENTS: Premature teardown of peer with moderate overflow.
func TestCongestion_TeardownRatioBelowThreshold(t *testing.T) {
	t.Parallel()

	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1", 100000, 1) // large peer, high demand

	// Overflow is 10 items against demand of ~750 (burstWeight(100000)/20).
	// Ratio = 10/750 = 0.013, well below 2.0.
	depths := map[string]int{"10.0.0.1": 10}

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

	addr := netip.MustParseAddrPort("10.0.0.1:179")
	cc.CheckTeardown(addr)
	fc.Add(time.Second)
	cc.CheckTeardown(addr)
	assert.Equal(t, int64(0), tornDown.Load(), "ratio < 2x should not trigger teardown")
}

// TestCongestion_NilCheckTeardown verifies nil controller is safe.
//
// VALIDATES: Safe nil handling for CheckTeardown.
// PREVENTS: Nil pointer panic in worker loop.
func TestCongestion_NilCheckTeardown(t *testing.T) {
	t.Parallel()

	var cc *congestionController
	cc.CheckTeardown(netip.MustParseAddrPort("10.0.0.1:179"))
}
