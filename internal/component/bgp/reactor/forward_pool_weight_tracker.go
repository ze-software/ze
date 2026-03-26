// Design: docs/architecture/forward-congestion-pool.md -- two-tier pool sizing
// Overview: forward_pool.go -- forward pool dispatch
// Related: forward_pool_weight.go -- weight calculation functions

package reactor

import "sync"

// weightTracker tracks per-peer buffer demand and recalculates pool budget
// when the peer set changes. It is the bridge between prefix maximums
// (from config) and bufmux capacity (runtime allocation).
//
// Thread-safe. All methods may be called from any goroutine.
// The onBudgetChanged callback is always called outside the lock.
type weightTracker struct {
	mu    sync.Mutex
	peers map[string]*peerWeight // key: peer address

	// onBudgetChanged is called after every recalculation with the new
	// guaranteed and overflow buffer counts. Called outside wt.mu to
	// avoid holding the lock during external operations. May be nil.
	onBudgetChanged func(guaranteed, overflow int)
}

// peerWeight holds the sizing inputs for one peer.
type peerWeight struct {
	prefixMax    uint32
	preEOR       bool // true until all family EORs received
	familyCount  int  // number of families (from PrefixMaximum map)
	eorsReceived int  // EORs received so far
}

// newWeightTracker creates a weight tracker. The callback fires on every
// peer set change with the new (guaranteed, overflow) buffer counts.
// Callback may be nil.
func newWeightTracker(onBudgetChanged func(guaranteed, overflow int)) *weightTracker {
	return &weightTracker{
		peers:           make(map[string]*peerWeight),
		onBudgetChanged: onBudgetChanged,
	}
}

// AddPeer registers a peer with its prefix maximum and family count.
// The peer starts in pre-EOR state (full-table allocation).
// familyCount is the number of address families configured (one EOR
// expected per family). Recalculates pool budget.
//
// If the peer already exists, its prefix maximum is updated and EOR
// state is reset to pre-EOR.
func (wt *weightTracker) AddPeer(peerAddr string, prefixMax uint32, familyCount int) {
	wt.mu.Lock()
	wt.peers[peerAddr] = &peerWeight{
		prefixMax:   prefixMax,
		preEOR:      true,
		familyCount: familyCount,
	}
	g, o := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o)
}

// RemovePeer removes a peer from tracking. Recalculates pool budget.
// No-op if the peer is not tracked.
func (wt *weightTracker) RemovePeer(peerAddr string) {
	wt.mu.Lock()
	if _, ok := wt.peers[peerAddr]; !ok {
		wt.mu.Unlock()
		return
	}
	delete(wt.peers, peerAddr)
	g, o := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o)
}

// PeerEORReceived records one EOR received for a peer. When the count
// reaches familyCount, the peer transitions from pre-EOR to post-EOR
// demand (shrinks allocation). Recalculates pool budget on transition.
// No-op if peer is unknown or already post-EOR.
func (wt *weightTracker) PeerEORReceived(peerAddr string) {
	wt.mu.Lock()
	pw, ok := wt.peers[peerAddr]
	if !ok || !pw.preEOR {
		wt.mu.Unlock()
		return
	}
	pw.eorsReceived++
	if pw.familyCount > 0 && pw.eorsReceived < pw.familyCount {
		wt.mu.Unlock()
		return
	}
	// All family EORs received (or familyCount==0): transition to post-EOR.
	pw.preEOR = false
	g, o := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o)
}

// PeerEORComplete transitions a peer from pre-EOR to post-EOR demand
// immediately, regardless of EOR count. Used when all EORs are known
// to be complete. Recalculates pool budget. No-op if peer is unknown
// or already post-EOR.
func (wt *weightTracker) PeerEORComplete(peerAddr string) {
	wt.mu.Lock()
	pw, ok := wt.peers[peerAddr]
	if !ok || !pw.preEOR {
		wt.mu.Unlock()
		return
	}
	pw.preEOR = false
	g, o := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o)
}

// UpdateFamilyCount updates the expected EOR count for a peer based on
// the actual negotiated family count (available after OPEN exchange).
// This corrects the initial estimate from config-declared families.
// No-op if peer is unknown or already post-EOR.
func (wt *weightTracker) UpdateFamilyCount(peerAddr string, negotiatedFamilies int) {
	wt.mu.Lock()
	pw, ok := wt.peers[peerAddr]
	if !ok || !pw.preEOR {
		wt.mu.Unlock()
		return
	}
	pw.familyCount = negotiatedFamilies
	// If EORs already received >= new count, transition now.
	if negotiatedFamilies > 0 && pw.eorsReceived >= negotiatedFamilies {
		pw.preEOR = false
		g, o := wt.budgetLocked()
		wt.mu.Unlock()
		wt.fireCallback(g, o)
		return
	}
	wt.mu.Unlock()
}

// PeerCount returns the number of tracked peers.
func (wt *weightTracker) PeerCount() int {
	wt.mu.Lock()
	n := len(wt.peers)
	wt.mu.Unlock()
	return n
}

// PeerDemand returns the current buffer demand for a peer (pre-EOR or
// post-EOR based on state). Returns 0 for unknown peers.
func (wt *weightTracker) PeerDemand(peerAddr string) int {
	wt.mu.Lock()
	pw, ok := wt.peers[peerAddr]
	if !ok {
		wt.mu.Unlock()
		return 0
	}
	d := peerBufferDemand(pw.prefixMax, pw.preEOR)
	wt.mu.Unlock()
	return d
}

// TotalBudget returns the current (guaranteed, overflow) buffer counts.
func (wt *weightTracker) TotalBudget() (guaranteed, overflow int) {
	wt.mu.Lock()
	demands := wt.demandsLocked()
	wt.mu.Unlock()
	return calculatePoolBudget(demands)
}

// UsageToWeightRatios returns the overflow usage-to-weight ratio for each
// peer that has both a tracked weight and overflow items. The ratio is
// overflowItems / peerDemand (0.0 = no overflow pressure, >1.0 = over budget).
// Peers with zero demand or no overflow items are omitted.
// Used by Phase 5 buffer denial to identify the backpressure target (AC-19).
func (wt *weightTracker) UsageToWeightRatios(overflowDepths map[string]int) map[string]float64 {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	result := make(map[string]float64)
	for addr, depth := range overflowDepths {
		if depth <= 0 {
			continue
		}
		pw, ok := wt.peers[addr]
		if !ok {
			continue
		}
		demand := peerBufferDemand(pw.prefixMax, pw.preEOR)
		if demand <= 0 {
			continue
		}
		result[addr] = float64(depth) / float64(demand)
	}
	return result
}

// demandsLocked returns the current buffer demand for each peer.
// Caller must hold wt.mu.
func (wt *weightTracker) demandsLocked() []int {
	demands := make([]int, 0, len(wt.peers))
	for _, pw := range wt.peers {
		demands = append(demands, peerBufferDemand(pw.prefixMax, pw.preEOR))
	}
	return demands
}

// budgetLocked computes the current pool budget under the lock.
// Caller must hold wt.mu. Returns (-1, -1) if no callback is set.
func (wt *weightTracker) budgetLocked() (guaranteed, overflow int) {
	if wt.onBudgetChanged == nil {
		return -1, -1
	}
	demands := wt.demandsLocked()
	return calculatePoolBudget(demands)
}

// fireCallback calls onBudgetChanged if set. Must be called outside wt.mu.
func (wt *weightTracker) fireCallback(guaranteed, overflow int) {
	if wt.onBudgetChanged != nil && guaranteed >= 0 {
		wt.onBudgetChanged(guaranteed, overflow)
	}
}
