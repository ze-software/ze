# IS-IS SPF and Route Install

The shortest-path computation and the route install that let an IS-IS-learned
prefix reach the kernel FIB. A per-level directed graph is built from the synced
database with system IDs and pseudo-nodes as vertices and Extended IS
Reachability (TLV 22) adjacencies as wide-metric edges; Dijkstra runs rooted at
self per level with ECMP; the overload bit is honored; RFC 2966 leaking runs
between levels.

| Concern | File |
|---------|------|
| Graph build from the database | `spf/graph.go` |
| Dijkstra | `spf/spf.go` |
| Prefix attach, preference, diff | `spf/route.go` |
| Level 1 and level 2 leaking | `spf/leak.go` |
| Loc-RIB insertion | `spf/install.go` |
| Debounce orchestration and metrics | `spf/computer.go` |
| IPv6 seam | `spf/ipv6.go` |
| SPF run log | `spf/spflog.go` |
| Engine glue | `spf_wiring.go` |

## Decision: install is Loc-RIB insertion, not a redistribute event

This is the load-bearing call. IS-IS becomes a Loc-RIB source exactly as BGP is:
the installer inserts a `locrib.Path` carrying the IS-IS protocol ID, an
instance, the next hop, admin distance 115 and the metric.

Redistribute events feed the redistribute orchestrator, which exports routes to
**other protocols**, and never install to the FIB. The protocol ID is registered
once and exposed by an accessor, so the redistribution layer reuses the same
identity.

<!-- source: internal/plugins/isis/spf/install.go -- ProtocolID, DefaultAdminDistance, Installer, RouteSink -->

After the inserted path, the Loc-RIB best-path to sysrib to FIB chain is the same
machinery that already installs static, connected and BGP routes. The novelty
here is SPF correctness, not the install path.

## Decision: one admin distance, level preference resolved inside SPF

`locrib.Path` has no protocol-type or level field, so per-level admin distance is
not modelable. IS-IS resolves the up/down-aware preference internally and
publishes exactly **one** path per prefix. The existing
`rib.admin-distance.isis` leaf is reused unchanged and no per-level leaves were
added.

Per-level admin distance against **other** protocols would need a level field on
the path and per-level YANG leaves. It is not implemented.

## Decision: RFC 5308 section 5 preference, not a flat level rule

`preferenceRank` orders level-1 up (0), level-2 up (1), level-2 down (2), level-1
down (3), with ties broken by metric. A leaked level-1 down prefix is the **least**
preferred and loses to any level-2 prefix.

A flat "level 1 beats level 2" rule is the classic trap here.

<!-- source: internal/plugins/isis/spf/route.go -- preferenceRank, candidate.better, BuildRoutes, DiffRoutes -->

## Decision: a 64-bit accumulator with a clamp

The TLV 22 IS-reachability metric is 24-bit; the TLV 135 and TLV 236 prefix
metric is the full 32-bit field, read in full and never capped at 24 bits. Path
cost accumulates in 64 bits and clamps at the maximum path metric, so a sum of
32-bit prefix and 24-bit edge metrics cannot wrap. A prefix at or above the
maximum is unreachable and skipped.

## Decision: ECMP needed a path-group expansion in shared code

ECMP emits one path per equal-cost next hop with a distinct instance, but sysrib
keys its routes by protocol **string** and the Loc-RIB change historically
carried only the single best path, so siblings collapsed to one next hop.

Equal-cost siblings now travel on the Loc-RIB change, and sysrib expands them
into the best-change entry's ECMP paths. The change is additive: a single-path
source leaves the ECMP list empty, so static, connected and BGP are unaffected.

<!-- source: internal/core/rib/locrib/change.go -- Change.ECMP -->
<!-- source: internal/core/rib/locrib/manager.go -- siblingNextHops -->
<!-- source: internal/component/sysrib/ecmp.go -- ecmpCollect -->
<!-- source: internal/component/sysrib/sysrib.go -- BestChangeEntry.ECMPPaths -->

## Decision: leaking is a one-pass fixpoint

Leaking skips any source prefix that already carries the down bit in both
directions, so the re-origination a leak triggers does not re-leak and the next
SPF run recomputes the same set. The loop terminates with no explicit iteration
cap.

<!-- source: internal/plugins/isis/spf/leak.go -- LeakPrefixes -->

## Trap: next-hop resolution must read the locked snapshot

The circuit goroutine is the single writer and mutates adjacency state and
addresses on every hello under the table lock. Reading the live adjacency
pointer's fields off that lock from the SPF goroutine races. Both the IPv4 and
the IPv6 resolver iterate a value snapshot taken under the lock.

<!-- source: internal/plugins/isis/spf_wiring.go -- ResolveNextHop, ResolveNextHopV6, triggerSPF -->

## Trap: the root's own connected prefixes are skipped

They sit at distance 0 with an empty first-hop set and belong to the connected
route source. If IS-IS installed them it would claim a directly connected prefix
with itself as the next hop.

## Trap: an unresolved next hop is dropped, never installed

A path whose next hop does not resolve is not installed pointing nowhere. A
malformed stored LSP is skipped as one bad node rather than failing the whole
run.

## File split

Orchestration and debounce (`computer.go`), Dijkstra alone (`spf.go`), prefix
attach and preference and diff (`route.go`), and leaking (`leak.go`) are separate
files by single responsibility. `ipv6.go` is the dual-stack seam; `spflog.go`
backs `show isis spf-log`.

## Owned metrics

`ze_isis_spf_runs_total{level}`, `ze_isis_spf_duration_seconds{level}` and
`ze_isis_spf_nodes{level}` on the computer;
`ze_isis_routes_installed{level,afi}` on the installer.

## Coverage boundary

The single-daemon functional test asserts that `show isis route` returns an empty
list with no adjacency: SPF is wired, there are no phantom routes, and a fresh
engine does not panic. The live multi-node install, a remote prefix in the kernel,
ECMP multipath and withdraw on neighbor loss, needs raw Layer-2 and is the interop
step.
