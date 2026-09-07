# Static Routes

The static plugin turns operator-declared routes into forwarding entries. A route
in the MAIN table becomes a path in the shared Loc-RIB, where the system RIB ranks
it against every other protocol offering the same prefix and the FIB plugin
programs the winner. A route in a NAMED table goes straight to the data plane, on
Linux through netlink and on VPP through GoVPP. Both support ECMP with weighted
next hops, BFD-tracked failover, blackhole and reject routes, interface-only next
hops, and redistribution into BGP.

<!-- source: internal/plugins/static/model.go -- route data model -->
<!-- source: internal/plugins/static/backend.go -- backend abstraction -->
<!-- source: internal/plugins/static/register.go -- registration and lifecycle -->

## A main-table route reaches the FIB through the Loc-RIB

`applyProgrammed` sends a main-table route to `insertPathLocked`, which builds one
`locrib.Path` and inserts it. `selectBest` then ranks that path against BGP, OSPF
and IS-IS on the administrative distance the path carries, and `rib { distance {
static N } }` is where that number comes from. The FIB plugin programs the winner
as `RTPROT_ZE` (250).

Direct FIB programming was the first design and it was reversed. Its two reasons
had both expired. The first was that the pipeline had no concept of an ECMP group
or a next-hop weight: `locrib.Path.ECMP` now carries a group, and each member is
a `nexthop.NextHop` holding an address, an outgoing device and a weight. The
second was that admin-distance arbitration added little, because static at 10
already beats eBGP at 20; that reads the leaf as a constant, and it is a
configurable leaf whose value decided nothing at all.

What the reversal removes is a SECOND WRITER. The Linux FIB keys an entry on
table, destination, tos and priority, and not on `rtm_protocol`, so a static route
written at `RTPROT_STATIC` and a Ze route written at `RTPROT_ZE` addressed the
same entry whenever their metrics agreed, which they do by default. Which one
forwarded was decided by write order. One writer per prefix is what makes the
declared distance mean anything.

A NAMED table keeps the direct write. The Loc-RIB is keyed by (family, prefix)
and carries no table, so a named-table route inserted there would collide with the
main-table route for the same prefix. A named table also has exactly one writer by
construction, so there is nothing for a distance to decide. The table dimension
belongs to `plan/immediate/spec-fib-depth.md`, which owns `BestChangeEntry.TableID`.

Redistribution is unchanged and does not pass through the Loc-RIB.
`redistribute { import static }` receives the routes over the redistribute event
bus, from `emitRouteChange`, which still emits for main-table forward routes only.

**A main-table route needs a FIB plugin.** The system RIB selects the winner and a
FIB plugin writes it, so a configuration with static routes in the main table and
no `fib { kernel { } }` or `fib { vpp { } }` block would select a route nobody
programs. The `static-fib-writer` doctor check refuses that configuration at error
severity rather than letting the route go quietly uninstalled. The static plugin
declares `rib` as a dependency, so the system RIB itself always runs.

<!-- source: internal/plugins/static/locrib.go -- staticPath, insertPathLocked, the main-table boundary -->
<!-- source: internal/plugins/static/inject.go -- route apply, BFD integration, redistribute emit -->
<!-- source: internal/plugins/static/events/events.go -- RouteChange event registration -->
<!-- source: internal/plugins/static/doctor.go -- the FIB-writer and route-skipped checks -->

## ECMP and BFD

ECMP is one multipath route, never one route per next hop. Linux uses
`Route.MultiPath` with `[]*NexthopInfo` and VPP uses several `FibPath` entries.
The weight mapping differs: the kernel takes `Hops = Weight - 1`, VPP takes the
weight directly.

BFD changes the membership of the ECMP group, not the presence of the route. On
BFD down the next hop is removed and the route is reprogrammed with the survivors:
a main-table route is re-inserted into the Loc-RIB with the surviving set, and a
named-table route is rewritten through the backend. The route is withdrawn only
when every next hop is down.

A main-table route's weights reach the data plane through `nexthop.NextHop.Weight`
on the Loc-RIB path and `ECMPPath.Weight` on the best-change event. Both are one
octet, which is what the kernel's `rtnh_hops` and VPP's `fib_path` weight each
carry, so `staticPath` caps a configured weight above 255 rather than letting the
netlink encoder wrap it.

A main-table route carries `RTPROT_ZE` (250), because fib-kernel writes it. A
named-table route carries `RTPROT_STATIC` (251), because the static plugin writes
it. The two never name the same entry, because they never name the same table.

The page said until 2026-09-07 that the two protocol numbers were shared and that
there was no collision, because fib-kernel owned sysrib-derived prefixes and
static owned config-driven ones. Both halves were wrong. Static stamped 251 rather
than 250, and nothing enforced the partition: the Linux FIB does not key on
`rtm_protocol`, so two writers at the default metric addressed one entry. The
partition is now true by construction rather than by convention.

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
returns early when the table is not 0. The same test decides where the route is
installed, so a named table is out of both the Loc-RIB and the redistribute bus.

An interface next hop is resolved TWICE for a main-table route, and the two
resolutions answer different questions. The static plugin resolves the name at
config time so an operator who names a device that cannot be resolved is refused
while the transaction can still fail. The FIB plugin resolves it again when it
programs the route, because the Loc-RIB carries the device NAME and an ifindex is
not stable across a device replacement. Both call `iface.ResolveIndex`, or
`iface.ResolveVPPIndex` on a VPP data plane, so they cannot disagree about what
resolves.

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

Which failures a route can still be skipped for depends on where it is installed.
A named-table route is skipped for anything its backend refuses, netlink errors
included. A main-table route is skipped for anything `staticPath` refuses -- an
unresolvable next-hop device, a next-hop naming neither an address nor a device, a
metric this build cannot program -- because the Loc-RIB accepts every path it is
given. A netlink error on a main-table route is reported by the FIB plugin as
`fib-sync-failure` instead, after the config transaction has already succeeded.

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
