// Design: docs/architecture/isis/isis-9-spf-rib.md step 5 -- L1<->L2 inter-level route leaking
// (the ORIGINATION side). After per-level Dijkstra (spf.go) yields the reachable
// node set, an L1L2 router re-originates the OTHER level's reachable IS-IS
// prefixes into this level's own LSP, marking them with the RFC 2966 up/down bit
// so they never loop back up. This file computes WHICH prefixes leak and with
// which up/down state; the engine (lsdb_wiring.go) feeds the result back into
// origination (levelState merges them into TLV 135 / 236), and the receiving-side
// preference (route.go preferenceRank) ranks them L1-up > L2-up > L2-down > L1-down.
//
// RFC: rfc/short/rfc2966.md -- "When an L2 router ... advertises ... an L1
//   router's reachability into L1, it MUST set the up/down bit. ... a router
//   MUST NOT propagate a prefix with the up/down bit set from L1 back into L2."
//   The up/down bit is the loop-prevention marker: a down-bit prefix is NEVER
//   re-leaked upward, which makes the leak a one-pass fixpoint (the leaked-down
//   prefixes a re-origination adds are skipped on the next leak pass).
// RFC: rfc/short/rfc5305.md sec 4.1 -- the up/down bit lives in the TLV 135
//   control octet, not the metric; the leaked metric carries the full path cost.
// RFC: rfc/short/rfc5308.md sec 5 -- the same up/down leaking applies to IPv6
//   (TLV 236), so the IPv6 leak mirrors the IPv4 one over the SHARED SPF tree.

package spf

import (
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// LeakedPrefix is one prefix an L1L2 router re-originates into a level after
// leaking it from the other level. The engine turns it into a TLV 135 (IPv4) or
// TLV 236 (IPv6) entry in this level's own LSP, with UpDown reflecting the RFC
// 2966 down bit (set for an L2->L1 down leak, clear for an L1->L2 up leak).
type LeakedPrefix struct {
	// Prefix is the leaked destination prefix (IPv4 for TLV 135, IPv6 for TLV 236).
	Prefix netip.Prefix
	// Metric is the full path cost to the prefix in the SOURCE level (node distance
	// + prefix metric), clamped at MaxPathMetric so a wide-metric sum never wraps.
	// It fits in 32 bits because MaxPathMetric (0xFE000000) is below 2^32.
	Metric uint32
	// UpDown is the RFC 2966 up/down bit to stamp on the re-originated entry: set
	// for an L2->L1 (down) leak, clear for an L1->L2 (up) leak.
	UpDown bool
}

// LeakResult is the per-target-level set of prefixes to re-originate. A nil/empty
// slice for a level means nothing leaks into it. Only an L1L2 router (both levels
// present in the SPF result) produces a non-empty result; a single-level node
// leaks nothing (there is no other level to leak from).
type LeakResult struct {
	// IntoL1 are the prefixes leaked DOWN from L2 into L1 (RFC 2966: up/down bit
	// set). IPv4 (TLV 135).
	IntoL1 []LeakedPrefix
	// IntoL2 are the prefixes leaked UP from L1 into L2 (up/down bit clear). IPv4.
	IntoL2 []LeakedPrefix
	// IntoL1V6 / IntoL2V6 are the IPv6 (TLV 236) equivalents over the SHARED SPF
	// tree (RFC 5308 sec 5): identical leak rules, IPv6 prefixes.
	IntoL1V6 []LeakedPrefix
	// IntoL2V6 are the IPv6 prefixes leaked UP from L1 into L2.
	IntoL2V6 []LeakedPrefix
}

// Empty reports whether nothing leaks in any direction or family.
func (r LeakResult) Empty() bool {
	return len(r.IntoL1) == 0 && len(r.IntoL2) == 0 &&
		len(r.IntoL1V6) == 0 && len(r.IntoL2V6) == 0
}

// LeakPrefixes computes the RFC 2966 inter-level leak for an L1L2 router from the
// per-level SPF results and graphs. For each target level it collects the OTHER
// level's reachable IS-IS prefixes (a node reachable as a forwarding destination
// in that level), skipping:
//
//   - the root's own prefixes (already advertised at both levels by the connected
//     / redistribution origination, never "leaked");
//   - any source prefix that ALREADY carries the up/down (down) bit -- the
//     loop-prevention rule (RFC 2966): a down-bit prefix MUST NOT be leaked back
//     up, and re-leaking a down-bit prefix downward is pointless churn. Skipping
//     it in BOTH directions makes the leak a one-pass fixpoint, so the
//     re-origination it triggers does not loop.
//
// The leaked entry's up/down bit is set for the L2->L1 (down) direction and clear
// for the L1->L2 (up) direction. The leaked metric is the full source-level path
// cost (node distance + prefix metric), clamped at MaxPathMetric.
//
// results must contain both Level1 and Level2 for any leak to occur; a result set
// with a single level returns an empty LeakResult (a single-level node has no
// other level to leak from). The function is pure (no engine state) so it is
// fully unit-testable on a hand-built topology.
func LeakPrefixes(results []*Result, graphs map[Level]*Graph) LeakResult {
	byLevel := make(map[Level]*Result, len(results))
	for _, res := range results {
		if res != nil {
			byLevel[res.Level] = res
		}
	}
	l1, l2 := byLevel[Level1], byLevel[Level2]
	// Leaking is an L1L2-router behavior: both levels must have run.
	if l1 == nil || l2 == nil {
		return LeakResult{}
	}

	var out LeakResult
	// L2 -> L1 (down): set the up/down bit on the re-originated entry.
	out.IntoL1 = leakInto(l2, graphs[Level2], true, false)
	out.IntoL1V6 = leakInto(l2, graphs[Level2], true, true)
	// L1 -> L2 (up): leave the up/down bit clear.
	out.IntoL2 = leakInto(l1, graphs[Level1], false, false)
	out.IntoL2V6 = leakInto(l1, graphs[Level1], false, true)
	return out
}

// leakInto collects the prefixes to leak FROM the source level's SPF result into
// the other level. setDownBit is the RFC 2966 up/down bit to stamp on each
// re-originated entry (true for L2->L1 down, false for L1->L2 up). v6 selects the
// IPv6 (TLV 236) prefix set instead of the IPv4 (TLV 135) one. The returned slice
// is deduplicated by prefix (keeping the minimum total metric) and sorted for a
// stable, alloc-free origination input.
func leakInto(src *Result, g *Graph, setDownBit, v6 bool) []LeakedPrefix {
	if src == nil || g == nil {
		return nil
	}
	rootID := types.NewSourceID(src.Root, 0)
	best := make(map[netip.Prefix]uint32)
	for id, nr := range src.Nodes {
		// The root's own prefixes are its connected/redistributed advertisement,
		// already present at both levels; they are not "leaked" from another level.
		if id == rootID {
			continue
		}
		// Only a node reachable as a forwarding destination contributes leakable
		// prefixes (a node with no first-hop is not actually reachable here).
		if len(nr.FirstHops) == 0 {
			continue
		}
		node := g.Nodes[id]
		if node == nil {
			continue
		}
		prefixes := node.Prefixes
		if v6 {
			prefixes = node.PrefixesV6
		}
		for _, p := range prefixes {
			if !p.Prefix.IsValid() {
				continue
			}
			// RFC 2966 loop prevention: a prefix that already carries the up/down
			// (down) bit MUST NOT be re-leaked back up, and re-leaking it down is
			// pointless churn -- skip it in both directions so the leak is a
			// one-pass fixpoint.
			if p.UpDown {
				continue
			}
			total := clampMetric(nr.Metric, uint64(p.Metric))
			if total >= MaxPathMetric {
				continue // RFC 5305 sec 4 / RFC 5308 sec 2: unreachable, do not leak
			}
			m := uint32(total) // total < MaxPathMetric (0xFE000000) < 2^32
			if cur, ok := best[p.Prefix.Masked()]; !ok || m < cur {
				best[p.Prefix.Masked()] = m
			}
		}
	}
	if len(best) == 0 {
		return nil
	}
	out := make([]LeakedPrefix, 0, len(best))
	for pfx, m := range best {
		out = append(out, LeakedPrefix{Prefix: pfx, Metric: m, UpDown: setDownBit})
	}
	// Deterministic order so the re-originated LSP bytes are stable across runs
	// (an unstable order would re-flood an identical LSP). netip.Prefix.Compare is
	// a zero-alloc total order, matching the SPF install path comparators.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Prefix.Compare(out[j].Prefix) < 0
	})
	return out
}
