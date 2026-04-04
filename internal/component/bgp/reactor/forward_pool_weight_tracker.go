// Design: docs/architecture/forward-congestion-pool.md -- two-tier pool sizing
// Overview: forward_pool.go -- forward pool dispatch
// Related: forward_pool_weight.go -- weight calculation functions
// Related: forward_pool_congestion.go -- two-threshold congestion enforcement

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
	// guaranteed and overflow buffer counts plus the overflow byte budget
	// from overflowPoolBudget(). Called outside wt.mu to avoid holding
	// the lock during external operations. May be nil.
	onBudgetChanged func(guaranteed, overflow int, ob overflowBudgetResult)
}

// peerWeight holds the sizing inputs for one peer.
type peerWeight struct {
	prefixMax    uint32
	preEOR       bool // true until all family EORs received
	familyCount  int  // number of families (from PrefixMaximum map)
	eorsReceived int  // EORs received so far
	extMsg       bool // true if Extended Message (RFC 8654) negotiated
}

// newWeightTracker creates a weight tracker. The callback fires on every
// peer set change with the new (guaranteed, overflow) buffer counts and
// overflow byte budget. Callback may be nil.
func newWeightTracker(onBudgetChanged func(guaranteed, overflow int, ob overflowBudgetResult)) *weightTracker {
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
	g, o, ob := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o, ob)
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
	g, o, ob := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o, ob)
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
	g, o, ob := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o, ob)
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
	g, o, ob := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o, ob)
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
		g, o, ob := wt.budgetLocked()
		wt.mu.Unlock()
		wt.fireCallback(g, o, ob)
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

// WorstPeerRatio returns the peer address with the highest usage-to-weight
// ratio and that ratio value. Returns ("", 0) if no peer has overflow items.
// Used by the congestion controller to identify the teardown candidate (AC-4).
func (wt *weightTracker) WorstPeerRatio(overflowDepths map[string]int) (worstAddr string, worstRatio float64) {
	wt.mu.Lock()
	defer wt.mu.Unlock()

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
		ratio := float64(depth) / float64(demand)
		if ratio > worstRatio {
			worstRatio = ratio
			worstAddr = addr
		}
	}
	return worstAddr, worstRatio
}

// UpdateExtMsg updates the Extended Message flag for a peer after session
// negotiation. When Extended Message (RFC 8654) is agreed, the peer's
// overflow budget uses 64K buffers instead of 4K. Recalculates pool budget.
// No-op if peer is unknown.
func (wt *weightTracker) UpdateExtMsg(peerAddr string, extMsg bool) {
	wt.mu.Lock()
	pw, ok := wt.peers[peerAddr]
	if !ok {
		wt.mu.Unlock()
		return
	}
	if pw.extMsg == extMsg {
		wt.mu.Unlock()
		return // no change
	}
	pw.extMsg = extMsg
	g, o, ob := wt.budgetLocked()
	wt.mu.Unlock()
	wt.fireCallback(g, o, ob)
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

// overflowInputsLocked returns the overflow sizing inputs for all peers.
// Caller must hold wt.mu.
func (wt *weightTracker) overflowInputsLocked() []overflowPeerInput {
	inputs := make([]overflowPeerInput, 0, len(wt.peers))
	for _, pw := range wt.peers {
		inputs = append(inputs, overflowPeerInput{
			prefixMax: pw.prefixMax,
			extMsg:    pw.extMsg,
		})
	}
	return inputs
}

// budgetLocked computes the current pool budget under the lock.
// Returns guaranteed/overflow buffer counts and the overflow byte budget.
// Caller must hold wt.mu. Returns (-1, -1, zero) if no callback is set.
func (wt *weightTracker) budgetLocked() (guaranteed, overflow int, ob overflowBudgetResult) {
	if wt.onBudgetChanged == nil {
		return -1, -1, overflowBudgetResult{}
	}
	demands := wt.demandsLocked()
	guaranteed, overflow = calculatePoolBudget(demands)
	inputs := wt.overflowInputsLocked()
	ob = overflowPoolBudget(inputs)
	return guaranteed, overflow, ob
}

// fireCallback calls onBudgetChanged if set. Must be called outside wt.mu.
func (wt *weightTracker) fireCallback(guaranteed, overflow int, ob overflowBudgetResult) {
	if wt.onBudgetChanged != nil && guaranteed >= 0 {
		wt.onBudgetChanged(guaranteed, overflow, ob)
	}
}
