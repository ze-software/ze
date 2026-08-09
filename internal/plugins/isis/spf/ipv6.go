// Design: docs/architecture/isis/isis-12-ipv6.md -- IPv6 leaf extraction + next-hop over the
// SHARED SPF tree. Single-topology (RFC 5308): IPv6 reachability rides the same
// per-level Dijkstra result the IPv4 slice computes (spf.go / graph.go); there is
// NO second SPF run and NO separate IPv6 topology graph. The only per-AF
// differences are the leaf TLV (236 vs 135), the next-hop source (neighbor
// link-local from TLV 232 vs IPv4 TLV 132), and the installed Loc-RIB family.
//
// RFC: rfc/short/rfc5308.md sec 2 -- "A prefix advertised with a metric larger
//   than MAX_V6_PATH_METRIC (0xFE000000) MUST NOT be considered during normal
//   SPF computation." This file applies that filter to TLV 236 leaves.
// RFC: rfc/short/rfc5308.md sec 5 -- the up/down-aware path preference order
//   (L1-up > L2-up > L2-down > L1-down, then metric) is identical to IPv4 and is
//   reused via the shared candidate.better / preferenceRank (route.go).

package spf

import (
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// MaxV6PathMetric is RFC 5308 sec 2 MAX_V6_PATH_METRIC. A TLV 236 prefix
// advertised with a metric STRICTLY GREATER than this value MUST NOT be
// considered during normal IPv6 SPF (it can be advertised for purposes other than
// building the normal routing table). It equals the RFC 5305 MAX_PATH_METRIC.
const MaxV6PathMetric uint64 = 0xFE000000

// NextHopResolverV6 maps a first-hop neighbor System ID to its IPv6 next-hop and
// outgoing interface for one level. The engine implements it from the per-circuit
// adjacency tables (Shared Contracts "Next-hop derivation for SPF"): the
// neighbor's learned IPv6 LINK-LOCAL address (from its IIH TLV 232) and the
// circuit it is adjacent on. A link-local next-hop is only meaningful with its
// interface, so the interface MUST be set for a link-local address (R-2). Returns
// ok=false when the neighbor has no usable IPv6 adjacency (the next-hop is then
// skipped, so a route is never installed pointing at an unresolvable next-hop).
type NextHopResolverV6 interface {
	ResolveNextHopV6(level Level, neighbor types.SystemID) (NextHop, bool)
}

// BuildRoutesV6 turns the per-level SPF results into the arbitrated set of IPv6
// routes to install. It is the IPv6 twin of BuildRoutes: it walks the SAME SPF
// results / graphs (the shared tree), but attaches each reachable node's TLV 236
// (IPv6) prefixes instead of TLV 135, resolves next-hops via the IPv6 resolver,
// and applies the RFC 5308 sec 2 MAX_V6_PATH_METRIC filter. The multi-level
// arbitration (preferenceRank / candidate.better) is shared with IPv4. The root's
// own connected prefixes (distance 0, no first-hop) are skipped, exactly as IPv4.
func BuildRoutesV6(results []*Result, graphs map[Level]*Graph, resolver NextHopResolverV6) []RouteEntry {
	best := make(map[netip.Prefix]candidate)

	for _, res := range results {
		if res == nil {
			continue
		}
		g := graphs[res.Level]
		if g == nil {
			continue
		}
		rootID := types.NewSourceID(res.Root, 0)
		for id, nr := range res.Nodes {
			node := g.Nodes[id]
			if node == nil {
				continue
			}
			if id == rootID {
				continue // the root's own prefixes are directly connected
			}
			if len(nr.FirstHops) == 0 {
				continue // unreachable as a forwarding destination
			}
			nhs := resolveHopsV6(resolver, res.Level, nr.FirstHops)
			if len(nhs) == 0 {
				continue // no first-hop resolved to a usable IPv6 next-hop
			}
			for _, p := range node.PrefixesV6 {
				// RFC 5308 sec 2: a TLV 236 prefix with metric > MAX_V6_PATH_METRIC
				// is excluded from normal SPF (decoded but not routed).
				if uint64(p.Metric) > MaxV6PathMetric {
					continue
				}
				total := clampMetric(nr.Metric, uint64(p.Metric))
				if total >= MaxPathMetric {
					continue // accumulated path cost at/above the ceiling: unreachable
				}
				cand := candidate{
					metric:   total,
					level:    res.Level,
					upDown:   p.UpDown,
					nextHops: nhs,
				}
				cur, ok := best[p.Prefix]
				if !ok || cand.better(cur) {
					best[p.Prefix] = cand
				}
			}
		}
	}

	out := make([]RouteEntry, 0, len(best))
	for pfx, c := range best {
		out = append(out, RouteEntry{
			Prefix:   pfx,
			Metric:   c.metric,
			Level:    c.level,
			UpDown:   c.upDown,
			NextHops: c.nextHops,
		})
	}
	// Sort for determinism via the zero-alloc netip.Prefix.Compare (a NUMERIC total
	// order), not the allocating Prefix.String() comparator. As in BuildRoutes
	// (route.go), Compare does NOT reproduce the lexicographic String() order, which
	// is harmless: the order is non-load-bearing (Apply re-derives the diff via
	// prefix-keyed maps; the CLI Snapshot re-sorts by String() for display).
	sort.Slice(out, func(i, j int) bool {
		return out[i].Prefix.Compare(out[j].Prefix) < 0
	})
	return out
}

// resolveHopsV6 resolves a node's first-hop System IDs to deduplicated IPv6
// next-hops via resolver, sorted by address. A first-hop with no usable IPv6
// adjacency is dropped; the result is the ECMP next-hop set for the destination.
// Mirrors resolveHops for IPv4.
func resolveHopsV6(resolver NextHopResolverV6, level Level, firstHops []types.SystemID) []NextHop {
	if resolver == nil {
		return nil
	}
	seen := make(map[netip.Addr]struct{}, len(firstHops))
	out := make([]NextHop, 0, len(firstHops))
	for _, sys := range firstHops {
		nh, ok := resolver.ResolveNextHopV6(level, sys)
		if !ok || !nh.Addr.IsValid() {
			continue
		}
		if _, dup := seen[nh.Addr]; dup {
			continue
		}
		seen[nh.Addr] = struct{}{}
		out = append(out, nh)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Addr.Compare(out[j].Addr) < 0
	})
	return out
}
