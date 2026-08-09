# Static Routes

The static plugin programs operator-declared routes straight into the FIB, on
Linux through netlink and on VPP through GoVPP. It supports ECMP with weighted
next hops, BFD-tracked failover, blackhole and reject routes, policy routing
tables, interface-only next hops, and redistribution into BGP.

<!-- source: internal/plugins/static/model.go -- route data model -->
<!-- source: internal/plugins/static/backend.go -- backend abstraction -->
<!-- source: internal/plugins/static/register.go -- registration and lifecycle -->

## Direct FIB programming, not Loc-RIB injection

Loc-RIB was the first design and was abandoned. `selectBest` picks a single
winner per prefix and the pipeline has no concept of an ECMP group or a next-hop
weight, so supporting static routes there would change the pipeline contract for
every protocol.

Static routes are config-authoritative: the operator declares exactly what to
install. Admin-distance arbitration adds little, because the static distance of
10 already beats eBGP at 20 and iBGP at 200.

Redistribution still reaches BGP. `redistribute { import static }` receives the
routes over the redistribute event bus without passing through Loc-RIB.

<!-- source: internal/plugins/static/inject.go -- route apply, BFD integration, redistribute emit -->
<!-- source: internal/plugins/static/events/events.go -- RouteChange event registration -->

## ECMP and BFD

ECMP is one multipath route, never one route per next hop. Linux uses
`Route.MultiPath` with `[]*NexthopInfo` and VPP uses several `FibPath` entries.
The weight mapping differs: the kernel takes `Hops = Weight - 1`, VPP takes the
weight directly.

BFD changes the membership of the ECMP group, not the presence of the route. On
BFD down the next hop is removed and the multipath route is reprogrammed with
the survivors. The route is withdrawn only when every next hop is down.

`RTPROT_ZE` (250) is shared with fib-kernel. There is no collision: fib-kernel
owns sysrib-derived prefixes and static owns config-driven prefixes.

## Table selection and interface-only next hops

Config is `static { table <name> { route ... } }`. The table name to table ID
mapping lives in a separate `routingtable` plugin, not inside static, because
policy routing needs the same mapping and a later VRF feature will absorb it.

<!-- source: internal/plugins/routingtable/registry.go -- table name to ID registry -->
<!-- source: internal/plugins/routingtable/config.go -- routing-table config parsing -->
<!-- source: internal/plugins/routingtable/register.go -- plugin registration -->

The registry is a package-level `atomic.Pointer[Registry]`. A plugin registers
itself in `init()`, which rules out constructor injection, and `bfdapi.GetService()`
had already set this pattern.

An interface-only next hop reuses the `nextHop` struct with a zero `Address`
rather than a separate type. The kernel model treats `Gw` and `LinkIndex` as
independent fields, so a separate Go type would split what the kernel treats as
one thing. In YANG, `container next` inside the `forward` case holds
`list hop` and `list interface` as siblings, so a mixed ECMP group is valid at
the schema level.

Routes in a non-default table are not redistributed into BGP: the emit path
returns early when the table is not 0.

## Two data planes, one resolver

The VPP backend is a separate sub-package so that a build without VPP does not
import govpp. It defines its own `Path` type with a `uint8` weight, which is the
VPP limit, so a caller translating from the parent's `uint16` weight caps it.

<!-- source: internal/plugins/static/backend_linux.go -- netlink multipath programming -->
<!-- source: internal/plugins/static/backend_vpp_linux.go -- VPP backend selection -->
<!-- source: internal/plugins/static/vpp/backend.go -- VPP route programming, toFibPath -->
<!-- source: internal/plugins/static/backend_other.go -- rejecting backend for non-Linux -->

Both backends resolve an interface next hop through the same `iface.Resolve`.
The netlink backend reads `Binding.Ifindex` as a kernel ifindex and the VPP
backend reads it as a VPP `sw_if_index`.

**The data plane and the iface backend are selected by two independent
globals.** `vpp.GetActiveConnector()` selects the VPP static backend and
`iface.LoadBackend` selects the iface backend. They can disagree, and a VPP
static backend resolving against a netlink iface backend would program a kernel
ifindex as a VPP `sw_if_index`: a silently wrong path. The VPP backend gates
interface-only resolution on `iface.ActiveBackendName() == "vpp"` and rejects a
zero or invalid index. Any future VPP-aware consumer of `iface.Resolve` needs
the same gate.

**A zero `netip.Addr` reports `Is4() == false`.** An address-less path was
therefore encoded as `PROTO_IP6` with an all-zero IPv6 next hop, even for an
IPv4 route. `toFibPath` takes the route prefix and derives the family from the
ROUTE when the next hop is unset. Any new address-less path type passes the
route family and never infers the family from the next hop.

## One bad route does not drop the section

A per-route failure was once joined into one error that failed the whole static
section. Now `applyRouteLocked` logs the failure, tears down the half-built
state, drops the route from the route map and records it in a skipped map.
`applyRoutes` returns nil, so `OnConfigure` proceeds and the good routes stay
programmed.

<!-- source: internal/plugins/static/diff.go -- diff engine and routesEqual short circuit -->
<!-- source: internal/plugins/static/doctor.go -- interface next-hop readiness check -->

A skipped route is kept out of the diff baseline, so the next apply retries it
and it clears once its device or backend appears. The skip is visible, never
silent: `static show` reports `skipped` with a `skip-reason`, and the
`doctor-static-route-skipped` check reports it too.

Replacing a live route with a now-unresolvable next hop skips the new route and
withdraws the old one, so that prefix is consistently unrouted across the FIB,
the announcement and `static show`. It is not a blackhole and it is retried on
the next apply.

## Startup ordering and emit rules

The plugin loads when the config carries a `static { }` root. It declares
`OptionalDependencies: ["interface"]`, so with an `interface` stanza present
static is ordered into a later startup tier than the iface component and its
next hops resolve against a loaded backend. With no `interface` stanza the
optional dependency is inert.

Emit tracking uses a per-route `emitted` flag. A forward to non-forward
replacement emits a remove for the old route. A forward to forward replacement
emits no remove, which avoids a redistribute flap; the new route emits an add,
which is idempotent for the consumer.

## Config format traps

- `Tree.ToMap()` serializes a YANG list as `map[key]value`, not as an array of
  objects with the key as a field. A parser written against the array form
  never matches the daemon's runtime format, and unit tests fed array-shaped
  JSON will still pass. Every plugin config parser uses the map form.
- `netlink.Route.Table` 0 (`RT_TABLE_UNSPEC`) and 254 (`RT_TABLE_MAIN`) both
  mean the main table. Routes written with 0 are read back as 254, so the list
  path normalizes 254 to 0.
- `static show` output is sorted by prefix string. The sort is lexicographic,
  not numeric, because the output is consumed as JSON rather than read by a
  human.

<!-- source: internal/plugins/static/config.go -- map-keyed tree traversal, table resolution -->
<!-- source: internal/plugins/static/eventbus.go -- event bus integration -->
<!-- source: internal/plugins/static/logger.go -- plugin logger -->
<!-- source: internal/plugins/routingtable/logger.go -- routing-table plugin logger -->
