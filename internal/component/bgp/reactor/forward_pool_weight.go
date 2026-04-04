// Design: docs/architecture/forward-congestion-pool.md -- two-tier pool sizing
// Overview: forward_pool.go -- forward pool dispatch
// Related: bufmux.go -- block-backed buffer multiplexer

package reactor

import (
	"math"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
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

// overflowFanOut returns the capped fan-out for the overflow sizing formula:
// min(N-1, floor(2*sqrt(N))), floor 1. This limits how many destination peers
// receive forwarded routes from the largest peer's restart burst.
// The 2*sqrt(N) cap prevents unreasonable pool sizes for large IXPs while
// still covering realistic convergence scenarios.
func overflowFanOut(totalPeers int) int {
	if totalPeers <= 1 {
		return 1
	}
	nMinus1 := totalPeers - 1
	sqrtCap := int(2 * math.Sqrt(float64(totalPeers)))
	f := min(nMinus1, sqrtCap)
	if f < 1 {
		return 1
	}
	return f
}

// overflowPeerInput holds the sizing inputs for one peer in the overflow
// budget calculation. Separates protocol facts (prefixMax, extMsg) from
// the pure sizing function.
type overflowPeerInput struct {
	prefixMax uint32 // total prefix maximum across all families
	extMsg    bool   // true if Extended Message (RFC 8654) negotiated
}

// overflowBudgetResult holds the output of the overflow pool sizing formula.
type overflowBudgetResult struct {
	slots int   // total overflow slot count
	bytes int64 // total byte budget (slots * per-slot byte size)
}

// overflowPoolBudget computes the shared overflow pool byte budget from
// peer prefix maximums and negotiated message sizes. Pure function.
//
// Formula:
//  1. largest = max peer's peerBufferDemand(prefixMax, preEOR=true)
//  2. fanOut = min(N-1, 2*sqrt(N)), floor 1
//  3. restartBurst = largest * fanOut
//  4. steadyContrib = sum of other peers' peerBufferDemand(prefixMax, false) * 0.1
//  5. totalSlots = restartBurst + steadyContrib, floor 64
//  6. Convert to bytes using per-peer negotiated sizes
func overflowPoolBudget(peers []overflowPeerInput) overflowBudgetResult {
	const minSlots = 64

	if len(peers) == 0 {
		return overflowBudgetResult{slots: minSlots, bytes: int64(minSlots) * int64(message.MaxMsgLen)}
	}

	// Find the largest peer by pre-EOR buffer demand.
	largestIdx := 0
	largestDemand := 0
	for i, p := range peers {
		d := peerBufferDemand(p.prefixMax, true)
		if d > largestDemand {
			largestDemand = d
			largestIdx = i
		}
	}

	fanOut := overflowFanOut(len(peers))
	restartBurst := largestDemand * fanOut

	// Byte size for restart burst slots: determined by the largest peer's
	// negotiated message size (the peer causing the burst).
	largestBufSize := int64(message.MaxMsgLen)
	if peers[largestIdx].extMsg {
		largestBufSize = int64(message.ExtMsgLen)
	}

	// Steady-state contributions from all other peers (post-EOR demand * 10%).
	var steadySlots int
	var steadyBytes int64
	for i, p := range peers {
		if i == largestIdx {
			continue
		}
		d := peerBufferDemand(p.prefixMax, false)
		contrib := int(float64(d) * 0.1)
		steadySlots += contrib
		bufSize := int64(message.MaxMsgLen)
		if p.extMsg {
			bufSize = int64(message.ExtMsgLen)
		}
		steadyBytes += int64(contrib) * bufSize
	}

	totalSlots := max(restartBurst+steadySlots, minSlots)

	totalBytes := int64(restartBurst)*largestBufSize + steadyBytes
	minBytes := int64(minSlots) * int64(message.MaxMsgLen)
	totalBytes = max(totalBytes, minBytes)

	return overflowBudgetResult{slots: totalSlots, bytes: totalBytes}
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
