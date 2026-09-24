# IS-IS SPF and Route Install

The shortest-path computation and the route install that let an IS-IS-learned
prefix reach the kernel FIB. A per-level directed graph is built from the synced
database with system IDs and pseudo-nodes as vertices. Narrow IS reachability
(TLV 2) and wide IS reachability (TLV 22) supply edges; IPv4 TLVs 128, 130 and
135 supply prefix leaves. Dijkstra runs per level with ECMP and the overload
bit, followed by RFC 2966 inter-level leaking.

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
`rib.distance.isis` leaf is reused unchanged and no per-level leaves were
added.

Per-level admin distance against **other** protocols would need a level field on
the path and per-level YANG leaves. It is not implemented.

## Decision: metric type precedes cost

IPv4 selection follows RFC 2966 section 3.2: L1 internal-metric routes, L2
internal-metric routes, L1 down-leaked internal-metric routes, then the same
three classes with external metrics. TLV 130 identifies external reachability;
its default metric can still be internal. Such a route competes with TLV 128
on total cost. An external metric competes on its own value first and uses
distance to the advertising router only to break an external-metric tie.

The narrow L2 up/down bit is ignored as recommended by RFC 2966 section 3.3.
Wide IPv4 and IPv6 retain RFC 5302 / RFC 5308's L1-up, L2-up, L2-down,
L1-down order. IPv6 has no narrow external-metric class.

<!-- source: internal/plugins/isis/spf/route.go -- preferenceRank, candidate.better, BuildRoutes, DiffRoutes -->

## Decision: a 64-bit accumulator with a clamp

The TLV 22 IS-reachability metric is 24-bit; the TLV 135 and TLV 236 prefix
metric is the full 32-bit field, read in full and never capped at 24 bits. Path
cost accumulates in 64 bits and clamps at the maximum path metric, so a sum of
32-bit prefix and 24-bit edge metrics cannot wrap. A prefix at or above the
maximum is unreachable and skipped.

Narrow IPv4 leaks retain TLV 128 or TLV 130 and their metric type. Internal
metrics add the distance to the advertising router and saturate at 63 on
re-origination. External metrics retain their advertised value.

## Decision: the maximum LINK metric excludes the link, not the path

RFC 5305 section 3 states two bounds, and they are different requirements. The
path bound above clamps an accumulated cost at MAX_PATH_METRIC (0xFE000000). The
link bound says a link advertised at the maximum LINK metric (2^24-1 =
16777215) MUST NOT be considered during the normal SPF computation, so an
operator can advertise a link for traffic engineering and keep it out of
hop-by-hop routing.

The exclusion is applied where an edge is considered, in `relax`, and not where
the graph is built. The RFC keeps such a link advertised on purpose, so the
graph still carries it for every reader that is not the normal shortest path
tree. One guard covers both levels and both address families, because Ze runs a
single per-level SPF tree and IPv6 rides it (`spf/ipv6.go`).

<!-- source: internal/plugins/isis/spf/spf.go -- relax -->

## Decision: ECMP needed a path-group expansion in shared code

ECMP emits one path per equal-cost next hop with a distinct instance, but sysrib
keys its routes by protocol **string** and the Loc-RIB change historically
carried only the single best path, so siblings collapsed to one next hop.

Equal-cost siblings now travel on the Loc-RIB change, and sysrib expands them
into the best-change entry's ECMP paths. The change is additive: a single-path
source leaves the ECMP list empty, so static, connected and BGP are unaffected.

The group sysrib WRITES is filtered once more. `ecmpCollect` drops a member
whose protocol `rib { fib-withhold }` names, and answers with no group at all
when the winner's own protocol is named, so `fib-withhold [ isis ]` keeps an
IS-IS multipath out of the FIB. Selection is untouched: `show rib` and `show
ecmp-groups` read `ecmpRIBGroup`, which filters nothing, so an operator still
reads the equal-cost paths that competed for the prefix.

<!-- source: internal/core/rib/locrib/change.go -- Change.ECMP -->
<!-- source: internal/core/rib/locrib/manager.go -- siblingNextHops -->
A member of that group carries its own forwarding path, not the winner's. The
collectors return a `forwardingPath`, which holds the label stack and the SRv6
SID beside the device and the share, so an equal-cost member promoted to take
the prefix is programmed with the labels IS-IS gave IT. Until 2026-09-20 the
promotion overrode the device and the share alone and the winner's label stack
rode onto the member, which is an MPLS misforward wherever the two protocols
impose different labels.

<!-- source: internal/component/sysrib/ecmp.go -- forwardingPath, ecmpCollect, ecmpRIBGroup -->
<!-- source: internal/component/sysrib/sysrib.go -- BestChangeEntry.ECMPPaths, fibChange -->

## Decision: leaking is a one-pass fixpoint

Leaking excludes an L1 prefix whose down bit is set, which prevents it returning
to L2. Wide L2 prefixes with the bit set are also excluded from another down
leak; narrow L2 entries ignore that bit under RFC 2966 section 3.3.

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

## Next-hop capability and terminal rejection

A missing adjacency cannot supply a next hop. An Up adjacency whose advertised
protocol set excludes the packet's address family instead supplies a terminal
unreachable route when no capable ECMP member remains. Dropping that route
would let a less-specific default forward traffic to an unsupported router.
Equal-cost originators in the winning preference class contribute one merged,
deduplicated next-hop set. A capable member excludes terminal-rejection members
from that set; a higher-cost or less-preferred route cannot bypass rejection.

The adjacency's logical interface and on-link flag travel with the gateway
through the Loc-RIB, route-install RPC, sysrib and kernel FIB. The gateway need
not share the interface's IP subnet: physical IS-IS adjacency establishes that
it is on the outgoing link.

SPF reads one locked raw LSDB snapshot per level. A source's fragment zero must
be live before its other standard fragments contribute routes. Every run
rebuilds the graph; periodic own-LSP refresh triggers a full run even without
a received topology change.
Runs are serialized through installation and completion callbacks. If the
configured root or level set changes during computation, that run is discarded
and rescheduled; it cannot install old-root routes or publish old reachability.

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
