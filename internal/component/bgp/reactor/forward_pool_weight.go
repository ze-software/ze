// Design: docs/architecture/forward-congestion-pool.md -- two-tier pool sizing
// Overview: forward_pool.go -- forward pool dispatch
// Related: bufmux.go -- block-backed buffer multiplexer

package reactor

import (
	"math"
	"sort"
)

// Pool sizing constants. Intentionally easy to change for future configuration.
const (
	// nlriPerMessage is the conservative estimate of NLRIs packed per BGP
	// UPDATE message. Real packing is 50-200 for shared-attribute batches.
	// Used to convert NLRI counts to buffer (bufmux handle) counts.
	nlriPerMessage = 20
)

// totalPrefixMax sums the prefix maximum values across all families for a peer.
// A convergence event can touch multiple families simultaneously, so the pool
// must account for the combined demand. Uses uint64 internally to avoid
// overflow when multiple families have large values, then saturates at MaxUint32.
func totalPrefixMax(prefixMaximums map[string]uint32) uint32 {
	var total uint64
	for _, v := range prefixMaximums {
		total += uint64(v)
	}
	if total > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(total)
}

// burstFraction returns the fraction of prefix maximum that represents a
// realistic convergence burst for a peer of the given size. Smaller peers
// over-provision prefix maximum by 4x+, so burst fraction is higher.
// Larger peers set prefix maximum close to actual, so burst fraction is lower.
//
// See docs/architecture/forward-congestion-pool.md "Scaling Curve" for rationale.
func burstFraction(prefixMax uint32) float64 {
	switch {
	case prefixMax < 500:
		return 1.0
	case prefixMax < 10000:
		return 0.5
	case prefixMax < 100000:
		return 0.3
	case prefixMax < 500000:
		return 0.15
	}
	// 500K+ (full table): DFZ grows slowly, 10% burst fraction.
	return 0.1
}

// burstWeight returns the burst-adjusted prefix count for a peer.
// This is the expected number of NLRIs in a convergence burst, not the
// raw prefix maximum.
func burstWeight(prefixMax uint32) int {
	return int(float64(prefixMax) * burstFraction(prefixMax))
}

// buffersNeeded returns the number of bufmux handles needed to hold
// the given number of NLRIs, using the conservative packing ratio.
func buffersNeeded(nlris int) int {
	if nlris <= 0 {
		return 0
	}
	return (nlris + nlriPerMessage - 1) / nlriPerMessage
}

// peerBufferDemand returns the number of bufmux handles a peer needs
// based on its prefix maximum and session phase.
//
// Pre-EOR (initial table dump): the peer sends its entire routing table.
// Allocation covers the full prefix maximum so initial convergence is
// never throttled.
//
// Post-EOR (steady state): only incremental updates flow. Allocation
// covers the burst-adjusted weight only.
func peerBufferDemand(prefixMax uint32, preEOR bool) int {
	if preEOR {
		return buffersNeeded(int(prefixMax))
	}
	return buffersNeeded(burstWeight(prefixMax))
}

// overflowPeerCount returns K = max(1, floor(sqrt(N))), the number of
// simultaneously slow peers the overflow tier is sized for.
func overflowPeerCount(totalPeers int) int {
	k := int(math.Sqrt(float64(totalPeers)))
	if k < 1 {
		return 1
	}
	return k
}

// calculatePoolBudget computes the guaranteed and overflow buffer counts
// from a slice of per-peer buffer demands.
//
// Guaranteed = sum of all demands (pre-allocated, always available).
// Overflow = sum of K largest demands, where K = max(1, sqrt(N)).
//
// Returns (guaranteed, overflow) buffer counts. The total pool budget
// is guaranteed + overflow.
func calculatePoolBudget(demands []int) (guaranteed, overflow int) {
	if len(demands) == 0 {
		return 0, 0
	}

	for _, d := range demands {
		guaranteed += d
	}

	// Sort descending to pick top K.
	sorted := make([]int, len(demands))
	copy(sorted, demands)
	sort.Sort(sort.Reverse(sort.IntSlice(sorted)))

	k := overflowPeerCount(len(demands))
	for i := range k {
		if i < len(sorted) {
			overflow += sorted[i]
		}
	}

	return guaranteed, overflow
}
