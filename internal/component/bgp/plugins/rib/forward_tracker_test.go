package rib

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// balanceHandle implements locrib.ForwardHandle + ForwardBytes and tracks an
// external AddRef/Release balance so tests can assert the buffer pool never
// leaks (every AddRef matched by exactly one Release).
type balanceHandle struct {
	balance *atomic.Int64
	adds    atomic.Int32
	data    []byte
}

func (h *balanceHandle) AddRef()       { h.balance.Add(1); h.adds.Add(1) }
func (h *balanceHandle) Release()      { h.balance.Add(-1) }
func (h *balanceHandle) Bytes() []byte { return h.data }

var famV4 = family.Family{AFI: 1, SAFI: 1}

// VALIDATES: rib-arch-6 AC-1 — the tracker AddRefs, reads the UPDATE bytes, and
// Releases; the recorded state reflects the forwarded prefix and byte length.
func TestForwardStateTracker_ReadsAndReleases(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	var balance atomic.Int64
	h := &balanceHandle{balance: &balance, data: []byte("update-wire-bytes")}
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	tr.onChange(locrib.Change{Family: famV4, Prefix: pfx, Kind: locrib.ChangeAdd, Best: trackerBGPPath(), Forward: h})

	require.Eventually(t, func() bool { return tr.snapshot().Forwarded == 1 },
		time.Second, time.Millisecond, "worker must process the forward change")

	s := tr.snapshot()
	assert.Equal(t, uint64(1), s.Forwarded)
	assert.Equal(t, uint64(len(h.data)), s.Bytes)
	assert.Equal(t, 1, s.Prefixes)
	assert.Equal(t, int32(1), h.adds.Load(), "exactly one AddRef")
	assert.Equal(t, int64(0), balance.Load(), "AddRef/Release must balance (no leak)")
}

// VALIDATES: rib-arch-6 — a disabled tracker is fully inert: it never AddRefs
// (no copy-out cost) and records no state.
func TestForwardStateTracker_DisabledIsInert(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	// Not enabled.

	var balance atomic.Int64
	h := &balanceHandle{balance: &balance, data: []byte("x")}
	tr.onChange(locrib.Change{Family: famV4, Prefix: netip.MustParsePrefix("10.0.0.0/24"), Kind: locrib.ChangeAdd, Best: trackerBGPPath(), Forward: h})

	// Give the worker a chance to (not) run.
	time.Sleep(20 * time.Millisecond)
	s := tr.snapshot()
	assert.False(t, s.Enabled)
	assert.Equal(t, uint64(0), s.Forwarded)
	assert.Equal(t, int32(0), h.adds.Load(), "disabled tracker must not AddRef")
	assert.Equal(t, int64(0), balance.Load())
}

// VALIDATES: rib-arch-6 — a ChangeRemove prunes the per-prefix state (no Forward
// handle involved) and counts the removal.
func TestForwardStateTracker_RemovePrunesState(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	var balance atomic.Int64
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	h := &balanceHandle{balance: &balance, data: []byte("bytes")}
	tr.onChange(locrib.Change{Family: famV4, Prefix: pfx, Kind: locrib.ChangeAdd, Best: trackerBGPPath(), Forward: h})
	require.Eventually(t, func() bool { return tr.snapshot().Prefixes == 1 }, time.Second, time.Millisecond)

	tr.onChange(locrib.Change{Family: famV4, Prefix: pfx, Kind: locrib.ChangeRemove})
	require.Eventually(t, func() bool { return tr.snapshot().Removes == 1 }, time.Second, time.Millisecond)

	assert.Equal(t, 0, tr.snapshot().Prefixes, "remove must prune the prefix")
	assert.Equal(t, int64(0), balance.Load())
}

// VALIDATES: rib-arch-6 AC-2 — under sustained concurrent churn the buffer-pool
// refcount stays balanced (no leak) and there is no data race. Run under -race.
func TestForwardStateTracker_NoLeakUnderChurn(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	tr.Enable()

	var balance atomic.Int64
	const goroutines, per = 8, 500

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range per {
				h := &balanceHandle{balance: &balance, data: []byte("update")}
				pfx := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(g), byte(i), 0}), 24)
				tr.onChange(locrib.Change{Family: famV4, Prefix: pfx, Kind: locrib.ChangeAdd, Best: trackerBGPPath(), Forward: h})
			}
		}(g)
	}
	// Concurrent reader to exercise the snapshot lock alongside the worker.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = tr.snapshot()
			}
		}
	}()
	wg.Wait()
	close(stop)

	// Stop drains the queue and joins the worker; every AddRef'd handle (whether
	// processed or dropped under backpressure) must be Released.
	tr.Stop()
	assert.Equal(t, int64(0), balance.Load(), "AddRef/Release must balance under churn (no buffer-pool leak)")
}

// trackerStaticID is a second protocol, so a test can make a non-BGP path the
// Loc-RIB best for a prefix.
var trackerStaticID = redistevents.RegisterProtocol("test-fastpath-static")

// trackerBGPPath is the path the rib plugin mirrors into the Loc-RIB for a
// BGP best route (rib_bestchange.go, bgpLocRIBInstance).
func trackerBGPPath() locrib.Path {
	return locrib.Path{Source: bgpProtocolID, Instance: bgpLocRIBInstance, NextHop: netip.MustParseAddr("192.0.2.1"), AdminDistance: 20}
}

// trackerStaticPath is a non-BGP path whose distance override beats BGP.
func trackerStaticPath() locrib.Path {
	return locrib.Path{Source: trackerStaticID, NextHop: netip.MustParseAddr("192.0.2.9"), HasDistanceOverride: true, DistanceOverride: 1}
}

// trackerPrefixes returns the prefixes the tracker records for famV4.
func trackerPrefixes(tr *forwardStateTracker) map[netip.Prefix]bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	out := make(map[netip.Prefix]bool, len(tr.state[famV4]))
	for p := range tr.state[famV4] {
		out[p] = true
	}
	return out
}

// locBGPBest returns the famV4 prefixes whose Loc-RIB best path is BGP.
func locBGPBest(loc *locrib.RIB) map[netip.Prefix]bool {
	out := make(map[netip.Prefix]bool)
	loc.Iterate(famV4, func(p netip.Prefix, g locrib.PathGroup) bool {
		if g.Best >= 0 && g.Best < len(g.Paths) && g.Paths[g.Best].Source == bgpProtocolID {
			out[p] = true
		}
		return true
	})
	return out
}

// VALIDATES: enable records every BGP best route the Loc-RIB already holds, in
// the enable call itself, and leaves a non-BGP best out.
// PREVENTS: a route installed before enable staying out of status (prefixes=0)
// until it changes (plan/journal/announced-state-never-replayed.md, the
// bgp-rs-fastpath.ci load red).
func TestForwardStateTracker_EnableReplaysInstalledBest(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()

	var balance atomic.Int64
	h := &balanceHandle{balance: &balance, data: []byte("update-wire-bytes")}
	bgpPrefix := netip.MustParsePrefix("10.20.30.0/24")
	staticPrefix := netip.MustParsePrefix("10.40.0.0/24")
	loc.InsertForward(famV4, bgpPrefix, trackerBGPPath(), h)
	loc.Insert(famV4, staticPrefix, trackerStaticPath())

	tr.Enable()

	s := tr.snapshot()
	assert.True(t, s.Enabled)
	assert.Equal(t, 1, s.Prefixes, "the BGP best installed before enable must be recorded by enable")
	assert.Equal(t, map[netip.Prefix]bool{bgpPrefix: true}, trackerPrefixes(tr))
	assert.Equal(t, uint64(1), s.Replayed, "status names the route enable found in the Loc-RIB")
	assert.Equal(t, uint64(0), s.Forwarded, "no UPDATE bytes exist for a route installed before enable")
	assert.Equal(t, int32(0), h.adds.Load(), "the disabled tracker took no reference")

	tr.Enable()
	assert.Equal(t, uint64(1), tr.snapshot().Replayed, "enabling an enabled tracker replays nothing")
}

// VALIDATES: an item queued in an earlier enable period is discarded once a
// re-enable has rebuilt the state, so it cannot overwrite the replay.
// PREVENTS: a change made before a disable, still queued at the re-enable,
// recording a prefix the Loc-RIB no longer holds.
func TestForwardStateTracker_DiscardsEarlierEnablePeriod(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	stale := forwardTrackItem{family: famV4, prefix: netip.MustParsePrefix("10.6.0.0/24"), epoch: tr.epoch.Load()}
	tr.Disable()
	tr.Enable()
	tr.process(stale)

	assert.Empty(t, trackerPrefixes(tr), "an item of an earlier enable period must not land")
}

// VALIDATES: a re-enable rebuilds the prefix set from the Loc-RIB, so a route
// removed while disabled is gone and a route added while disabled is present.
// PREVENTS: a disable/enable cycle resuming from state that missed every change
// made while the tracker was off.
func TestForwardStateTracker_ReenableRebuildsFromLocRIB(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	var balance atomic.Int64
	gone := netip.MustParsePrefix("10.1.0.0/24")
	kept := netip.MustParsePrefix("10.2.0.0/24")
	loc.InsertForward(famV4, gone, trackerBGPPath(), &balanceHandle{balance: &balance, data: []byte("bytes")})
	require.Eventually(t, func() bool { return tr.snapshot().Prefixes == 1 }, time.Second, time.Millisecond)

	tr.Disable()
	loc.Remove(famV4, gone, bgpProtocolID, bgpLocRIBInstance)
	loc.Insert(famV4, kept, trackerBGPPath())
	tr.Enable()

	assert.Equal(t, map[netip.Prefix]bool{kept: true}, trackerPrefixes(tr))
	require.Eventually(t, func() bool { return balance.Load() == 0 }, time.Second, time.Millisecond, "AddRef/Release must balance")
}

// VALIDATES: a BGP best that reaches the Loc-RIB with no source UPDATE (an
// AIGP re-election, a fallback after a removal) is recorded.
// PREVENTS: the tracker missing a BGP best only because Change.Forward is nil,
// which the forward-handle contract makes optional even for BGP.
func TestForwardStateTracker_RecordsBGPBestWithoutForward(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	loc.Insert(famV4, netip.MustParsePrefix("10.3.0.0/24"), trackerBGPPath())

	require.Eventually(t, func() bool { return tr.snapshot().Prefixes == 1 }, time.Second, time.Millisecond)
	assert.Equal(t, uint64(0), tr.snapshot().Forwarded, "no bytes were read")
}

// VALIDATES: a BGP best that replaces another BGP best with no handle records
// its length as unread, rather than keeping the previous path's length.
// PREVENTS: the state reporting the UPDATE length of a path the Loc-RIB no
// longer selects.
func TestForwardStateTracker_HandlelessBestReplacesReadLength(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	var balance atomic.Int64
	pfx := netip.MustParsePrefix("10.7.0.0/24")
	loc.InsertForward(famV4, pfx, trackerBGPPath(), &balanceHandle{balance: &balance, data: []byte("bytes")})
	length := func() (int, bool) {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		n, ok := tr.state[famV4][pfx]
		return n, ok
	}
	require.Eventually(t, func() bool { n, _ := length(); return n == len("bytes") }, time.Second, time.Millisecond)

	next := trackerBGPPath()
	next.NextHop = netip.MustParseAddr("192.0.2.2")
	loc.Insert(famV4, pfx, next)

	require.Eventually(t, func() bool { n, ok := length(); return ok && n == unreadUpdateLength }, time.Second, time.Millisecond,
		"the new best's bytes were never read")
	assert.Equal(t, uint64(1), tr.snapshot().Forwarded)
}

// VALIDATES: a prefix whose Loc-RIB best moves from BGP to another protocol
// leaves the tracker's state.
// PREVENTS: the state claiming a BGP best the Loc-RIB no longer selects.
func TestForwardStateTracker_NonBGPBestPrunes(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	var balance atomic.Int64
	pfx := netip.MustParsePrefix("10.4.0.0/24")
	loc.InsertForward(famV4, pfx, trackerBGPPath(), &balanceHandle{balance: &balance, data: []byte("bytes")})
	require.Eventually(t, func() bool { return tr.snapshot().Prefixes == 1 }, time.Second, time.Millisecond)

	loc.Insert(famV4, pfx, trackerStaticPath())

	require.Eventually(t, func() bool { return tr.snapshot().Prefixes == 0 }, time.Second, time.Millisecond,
		"a static best must prune the BGP entry")
}

// VALIDATES: enable racing Loc-RIB writers converges on exactly the BGP best
// set the Loc-RIB holds once the writers stop. Run under -race.
// PREVENTS: the replay and the live changes ordering a prefix's last state
// wrongly, leaving a removed prefix recorded or an installed one missing.
func TestForwardStateTracker_EnableDuringChurnConverges(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()

	// The writer makes 2*count changes, below forwardTrackQueue, so no change
	// is dropped and convergence is exact.
	const count = 200
	prefix := func(i int) netip.Prefix {
		return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 5, byte(i >> 8), byte(i)}), 32)
	}
	for i := range count {
		loc.Insert(famV4, prefix(i), trackerBGPPath())
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range count {
			if i%2 == 0 {
				loc.Remove(famV4, prefix(i), bgpProtocolID, bgpLocRIBInstance)
			}
			loc.Insert(famV4, prefix(count+i), trackerBGPPath())
		}
	})
	tr.Enable()
	wg.Wait()

	want := locBGPBest(loc)
	require.Eventually(t, func() bool { return assert.ObjectsAreEqual(want, trackerPrefixes(tr)) },
		time.Second, time.Millisecond, "state must converge on the Loc-RIB BGP best set")
	assert.Equal(t, uint64(0), tr.snapshot().Dropped)
}
