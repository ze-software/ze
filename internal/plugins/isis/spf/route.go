// Design: docs/architecture/isis/isis-9-spf-rib.md -- prefix attach, L1/L2 leaking, route diff.
// After Dijkstra (spf.go) yields per-node distances and first-hops, this file
// attaches each node's advertised IP prefixes (TLV 135) to a route at the node's
// distance plus the prefix metric, choosing the minimum total metric and the
// equal-cost next-hop set (ECMP), then arbitrates a prefix reachable at more
// than one level / up-down state into a single winning route.
//
// RFC: rfc/short/rfc2966.md -- the up/down bit (TLV 135 control octet) is the
//   loop-prevention marker for L1<->L2 leaking: a prefix leaked DOWN (L2 -> L1)
//   carries up/down set and is NOT re-advertised UP into L2.
// RFC: rfc/short/rfc5305.md sec 4.1 -- the up/down bit lives in the control
//   octet, not the metric; the prefix metric is the full 32-bit field.
// RFC: rfc/short/rfc5308.md sec 5 / RFC 5302 -- the up/down-aware preference
//   order when a prefix is reachable at more than one level/up-down state:
//   best to worst is L1-up > L2-up > L2-down > L1-down, ties broken by metric.
//   An L1 DOWN (leaked) prefix is LESS preferred than an L2 prefix, so a flat
//   "L1 always beats L2" rule is wrong.

package spf

import (
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// NextHop is one resolved equal-cost next-hop for an installed prefix: the IP
// address to forward to and the outgoing interface. SPF resolves the first-hop
// System ID to this via the NextHopResolver (the local adjacency table).
type NextHop struct {
	// Addr is the next-hop IP address (the adjacent neighbor's interface address,
	// TLV 132). Invalid means the next-hop could not be resolved and the entry is
	// dropped.
	Addr netip.Addr
	// Interface is the outgoing interface name the adjacency is on. Empty when
	// unknown (the kernel then resolves the egress from the route table).
	Interface string
}

// RouteEntry is one prefix SPF resolved to install: the destination, the total
// IS-IS metric, the level it was selected from, the up/down state, and the
// equal-cost next-hops. Exactly one RouteEntry is published per prefix after the
// multi-level arbitration (the L1-over-L2 / up-down order is resolved here, NOT
// in sysrib).
type RouteEntry struct {
	// Prefix is the destination IPv4 prefix (masked to its length).
	Prefix netip.Prefix
	// Metric is the total IS-IS path cost (node distance + prefix metric),
	// clamped at MaxPathMetric.
	Metric uint64
	// Level is the level the winning route came from (1 or 2).
	Level Level
	// UpDown is the up/down bit of the winning advertisement (RFC 2966): set when
	// the prefix was leaked down a level. Carried for the snapshot / diagnostics;
	// it does not change the installed next-hop.
	UpDown bool
	// NextHops are the resolved equal-cost next-hops (ECMP). At least one for a
	// route that is published; sorted by address for a stable diff.
	NextHops []NextHop
}

// preferenceRank returns the RFC 5308 sec 5 / RFC 5302 preference class of a
// (level, up/down) advertisement: LOWER is MORE preferred. The order is
// L1-up (0) > L2-up (1) > L2-down (2) > L1-down (3); ties within a class are
// broken by metric (lower wins) at the call site. This is the up/down-aware
// order: an L1-down (leaked) prefix (rank 3) is LESS preferred than any L2
// prefix (rank 1 or 2), so it is NOT a flat "L1 beats L2".
func preferenceRank(level Level, upDown bool) int {
	switch {
	case level == Level1 && !upDown:
		return 0 // L1 up: intra-area, most preferred
	case level == Level2 && !upDown:
		return 1 // L2 up
	case level == Level2 && upDown:
		return 2 // L2 down (leaked into L2 from below; rare)
	default:
		return 3 // L1 down: leaked from L2 into L1, least preferred
	}
}

// candidate is one (prefix, level, up/down) route option before arbitration. The
// route builder collects every candidate from every level's SPF result, then
// selects the single winner per prefix.
type candidate struct {
	metric   uint64
	level    Level
	upDown   bool
	nextHops []NextHop
}

// better reports whether candidate c is strictly preferred over o per the RFC
// 5308 sec 5 order: first the preference class (rank), then the metric. Used to
// pick the single winning route for a prefix across levels.
func (c candidate) better(o candidate) bool {
	rc, ro := preferenceRank(c.level, c.upDown), preferenceRank(o.level, o.upDown)
	if rc != ro {
		return rc < ro
	}
	return c.metric < o.metric
}

// NextHopResolver maps a first-hop neighbor System ID (a directly-adjacent
// router) to its next-hop address and outgoing interface for one level. The
// engine implements it from the per-circuit adjacency tables (Shared Contracts
// "Next-hop derivation for SPF"): the neighbor's learned IPv4 interface address
// (TLV 132) and the circuit it is adjacent on. Returns ok=false when the
// neighbor has no usable adjacency (the next-hop is then skipped).
type NextHopResolver interface {
	ResolveNextHop(level Level, neighbor types.SystemID) (NextHop, bool)
}

// BuildRoutes turns the per-level SPF results into the arbitrated set of routes
// to install. For each level it attaches every reachable node's TLV 135 prefixes
// at (node distance + prefix metric), resolving the node's first-hops to
// next-hops via res; a prefix advertised with a metric at or above MaxPathMetric
// is unreachable and skipped (RFC 5305 sec 4 / RFC 5308 sec 2). When the same
// prefix is reachable from more than one level / up-down state, the single
// winner is chosen by the RFC 5308 sec 5 preference order (L1-up > L2-up >
// L2-down > L1-down, then metric). The root's own connected prefixes (distance
// 0, empty first-hop set) are skipped here: they are installed by the connected
// route source, not IS-IS (avoids IS-IS claiming a directly-connected prefix).
//
// results may hold one or both levels; resolver is the live next-hop source.
func BuildRoutes(results []*Result, graphs map[Level]*Graph, resolver NextHopResolver) []RouteEntry {
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
			// The root's own prefixes (distance 0, no first-hop) are directly
			// connected; the connected source owns them. Skip so IS-IS does not
			// install a /len pointing at no next-hop.
			if id == rootID {
				continue
			}
			if len(nr.FirstHops) == 0 {
				continue // unreachable as a forwarding destination (no next-hop)
			}
			nhs := resolveHops(resolver, res.Level, nr.FirstHops)
			if len(nhs) == 0 {
				continue // no first-hop resolved to a usable next-hop
			}
			for _, p := range node.Prefixes {
				total := clampMetric(nr.Metric, uint64(p.Metric))
				if total >= MaxPathMetric {
					continue // RFC 5305 sec 4: at/above MAX_PATH_METRIC is unreachable
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
	// Sort for determinism. netip.Prefix.Compare is a zero-alloc NUMERIC total
	// order (validity, family, masked address, prefix length, then unmasked
	// address); the previous Prefix.String() comparator allocated two strings per
	// comparison on the SPF install path. NOTE: Compare does NOT reproduce the
	// lexicographic String() order (e.g. it orders "10.2.0.0/16" before
	// "10.10.0.0/16", which a string sort reverses). That is harmless here: this
	// order is non-load-bearing -- Apply re-derives the diff via prefix-keyed maps
	// (DiffRoutes/IndexByPrefix), and the CLI Snapshot re-sorts by String() for
	// display. We only need SOME stable total order, and Compare is the cheaper one.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Prefix.Compare(out[j].Prefix) < 0
	})
	return out
}

// resolveHops resolves a node's first-hop System IDs to deduplicated next-hops
// via resolver, sorted by address. A first-hop with no usable adjacency is
// dropped; the result is the ECMP next-hop set for the destination.
func resolveHops(resolver NextHopResolver, level Level, firstHops []types.SystemID) []NextHop {
	if resolver == nil {
		return nil
	}
	seen := make(map[netip.Addr]struct{}, len(firstHops))
	out := make([]NextHop, 0, len(firstHops))
	for _, sys := range firstHops {
		nh, ok := resolver.ResolveNextHop(level, sys)
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

// ---- Route diff (add / change / remove between SPF runs) ----

// RouteDelta is the difference between two successive route sets: the prefixes to
// add or change (the new RouteEntry) and the prefixes to remove (gone since the
// last run). The install layer (install.go) applies Added/Changed via
// InsertForward and Removed via a forward-remove.
type RouteDelta struct {
	// Added are prefixes new since the previous run.
	Added []RouteEntry
	// Changed are prefixes present in both runs whose metric / next-hop set /
	// level differs.
	Changed []RouteEntry
	// Removed are prefixes present in the previous run but gone now.
	Removed []netip.Prefix
}

// Empty reports whether the delta carries no changes.
func (d RouteDelta) Empty() bool {
	return len(d.Added) == 0 && len(d.Changed) == 0 && len(d.Removed) == 0
}

// DiffRoutes computes the add/change/remove delta from the previously installed
// set prev to the newly computed set cur. Both are indexed by prefix. A prefix
// in cur but not prev is Added; in both with a differing route is Changed; in
// prev but not cur is Removed. Equality compares metric, level, up/down, and the
// (sorted) next-hop set, so a pure next-hop change is a Change (R-4: stale routes
// are removed when a neighbor is lost).
func DiffRoutes(prev, cur map[netip.Prefix]RouteEntry) RouteDelta {
	var d RouteDelta
	for pfx, c := range cur {
		p, ok := prev[pfx]
		if !ok {
			d.Added = append(d.Added, c)
			continue
		}
		if !routeEqual(p, c) {
			d.Changed = append(d.Changed, c)
		}
	}
	for pfx := range prev {
		if _, ok := cur[pfx]; !ok {
			d.Removed = append(d.Removed, pfx)
		}
	}
	// Sort each list for deterministic application order. netip.Compare is a
	// zero-alloc NUMERIC total order (no per-comparison Prefix.String() allocation);
	// it is NOT the lexicographic String() order, which is fine because the order is
	// non-load-bearing here (Apply consumes Added/Changed/Removed independently of
	// their slice order). We only need a stable total order, not the old string one.
	sort.Slice(d.Added, func(i, j int) bool { return d.Added[i].Prefix.Compare(d.Added[j].Prefix) < 0 })
	sort.Slice(d.Changed, func(i, j int) bool { return d.Changed[i].Prefix.Compare(d.Changed[j].Prefix) < 0 })
	sort.Slice(d.Removed, func(i, j int) bool { return d.Removed[i].Compare(d.Removed[j]) < 0 })
	return d
}

// routeEqual reports whether two RouteEntries are identical for change
// detection: same metric, level, up/down, and next-hop set (order-independent;
// both sides are address-sorted by BuildRoutes so a positional compare suffices).
func routeEqual(a, b RouteEntry) bool {
	if a.Metric != b.Metric || a.Level != b.Level || a.UpDown != b.UpDown {
		return false
	}
	if len(a.NextHops) != len(b.NextHops) {
		return false
	}
	for i := range a.NextHops {
		if a.NextHops[i].Addr != b.NextHops[i].Addr || a.NextHops[i].Interface != b.NextHops[i].Interface {
			return false
		}
	}
	return true
}

// IndexByPrefix turns a route slice into a prefix-keyed map for diffing and for
// the installed-set snapshot the install layer retains between runs.
func IndexByPrefix(routes []RouteEntry) map[netip.Prefix]RouteEntry {
	m := make(map[netip.Prefix]RouteEntry, len(routes))
	for _, r := range routes {
		m[r.Prefix] = r
	}
	return m
}

// ---- Snapshot (`show isis route`, rendered by isis-13) ----

// RouteSnapshotEntry is one row of the `show isis route` view: the prefix,
// metric, level, up/down flag, and the resolved next-hops (address + interface).
// Flat value with no pointers so it crosses the CLI/RPC boundary cleanly.
type RouteSnapshotEntry struct {
	Prefix   string             `json:"prefix"`
	Metric   uint64             `json:"metric"`
	Level    string             `json:"level"`
	UpDown   bool               `json:"up-down,omitempty"`
	NextHops []RouteSnapshotHop `json:"next-hops"`
}

// RouteSnapshotHop is one next-hop in a snapshot row.
type RouteSnapshotHop struct {
	NextHop   string `json:"next-hop"`
	Interface string `json:"interface,omitempty"`
}

// Snapshot renders a route set as the `show isis route` view, sorted by prefix.
// It is a pure projection of the installed RouteEntry set (the engine holds the
// set and calls this for the CLI).
func Snapshot(routes []RouteEntry) []RouteSnapshotEntry {
	out := make([]RouteSnapshotEntry, 0, len(routes))
	for _, r := range routes {
		hops := make([]RouteSnapshotHop, 0, len(r.NextHops))
		for _, nh := range r.NextHops {
			hops = append(hops, RouteSnapshotHop{
				NextHop:   nh.Addr.String(),
				Interface: nh.Interface,
			})
		}
		out = append(out, RouteSnapshotEntry{
			Prefix:   r.Prefix.String(),
			Metric:   r.Metric,
			Level:    r.Level.String(),
			UpDown:   r.UpDown,
			NextHops: hops,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix < out[j].Prefix })
	return out
}
