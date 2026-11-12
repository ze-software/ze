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

The config parser selects each internal route action before the route reaches
the backend or Loc-RIB. Reload, rollback and BFD preserve that action.
An unnamed internal action is a BUG, not a skipped-route error; interface,
metric and backend failures retain their operational error paths.
The VPP translator also selects a named backend action before insertion.
Deletion supplies no action and bypasses path construction, so its zero value
remains valid.

<!-- source: internal/plugins/static/config.go -- parseRoute -->
<!-- source: internal/plugins/static/inject.go -- programRouteLocked -->
<!-- source: internal/plugins/static/model.go -- actionType.String -->
<!-- source: internal/plugins/static/locrib.go -- staticPath -->
<!-- source: internal/plugins/static/backend_linux.go -- buildRoute -->
<!-- source: internal/plugins/static/backend_vpp_linux.go -- toVPPRoute -->
<!-- source: internal/plugins/static/vpp/backend.go -- ApplyRoute, RemoveRoute, routeAddDel, buildFibPaths -->

## A main-table route reaches the FIB through the Loc-RIB

`applyProgrammed` sends a main-table route to `insertPathLocked`, which builds one
`locrib.Path` and inserts it. The path carries no protocol-wide distance: the
Loc-RIB ranks it against BGP, OSPF and IS-IS at the distance
`rib { distance { static N } }` declares, looked up when it ranks, and re-ranks
it when a reload changes that number. A route's own `distance` leaf travels on
the path as `DistanceOverride` and wins for that route alone. The FIB plugin
programs the winner as `RTPROT_ZE` (250).
<!-- source: internal/plugins/static/locrib.go -- staticPath -->
<!-- source: internal/core/rib/locrib/distance.go -- resolvedDistance, Reselect -->

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

The kernel side is read back by five functional tests in `test/static/`.
`static-kernel-distance-static-wins.ci` and `static-kernel-distance-bgp-wins.ci`
declare opposite distances for one contested prefix and each expect exactly one
kernel entry, `proto 250`, via the winner's next-hop.
`static-kernel-distance-reload.ci` starts at `static 5`, reloads to `static 250`
with no other change, and expects the kernel entry to move to the BGP next-hop
with one entry left, because the Loc-RIB re-ranks the installed route.
`static-kernel-distance-route-override.ci` gives one route its own `distance 3`
and a witness route none, under `static 5`, and reloads to `static 250`: the
witness moves to the BGP next-hop, and the route with its own distance keeps its
one kernel entry via the static next-hop, because the override wins over the
declared static distance before and after the reload.
`static-kernel-weighted-multipath.ci` expects the configured weights on the
kernel's multipath hops, and `static-kernel-interface-nexthop.ci` expects an
interface-only route to leave by that device with no gateway. A reload that
fails is undone by the journal `applyRouteSet` returns, which re-applies the
previous set to the Loc-RIB (`TestStaticRollbackRestoresThePreviousPathSet`).

A NAMED table keeps the direct write. The Loc-RIB is keyed by (family, prefix)
and carries no table, so a named-table route inserted there would collide with the
main-table route for the same prefix. A named table also has exactly one writer by
construction, so there is nothing for a distance to decide. The table dimension
belongs to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md`, which owns the
`BestChangeEntry.TableID` production (split out of `spec-fib-depth` on 2026-10-08).
`test/static/static-named-table-unchanged.ci` holds the boundary: the route lands
in its table as protocol 251, and neither `show rib` nor the main table lists it.

Redistribution is unchanged and does not pass through the Loc-RIB.
`redistribute { import static }` receives the routes over the redistribute event
bus, from `emitRouteChange`, which emits every main-table route: forward,
blackhole and reject. FRR (`zebra_redistribute_check` tests no next-hop type)
and BIRD (`static_announce_rte` announces every destination type) redistribute
a discard route the same way, which is how an operator originates an aggregate.

**A main-table route needs a FIB plugin, and the operator does not have to ask
for one.** The system RIB selects the winner and a FIB plugin writes it. Static
declares `Registration.NeedsDataPlane`, so when a config has static routes and no
`fib { ... }` block the engine loads the plugin whose `Registration.DataPlane`
matches the backend `interface { backend }` selects: `fib-kernel` for `netlink`,
`fib-vpp` for `vpp`. An explicit `fib { ... }` block always wins, because the
block is a config path and its own plugin is already in the startup set.

No package holds a list of FIB plugins. The writer declares the data plane it
programs and the registry answers, so adding one needs no edit to static and none
to the engine. `fib-p4` declares that it programs the FIB and names no data
plane, so `fib { p4 { } }` is still the operator's choice and the plugin is never
the ANSWER to a producer that needs one: its P4Runtime client is not written, so
it would answer with silence.

If no plugin programs the selected data plane, auto-loading refuses the whole
selection before starting a route producer. Boot and reload both propagate the
refusal. A configuration that needs no data-plane writer remains valid, and an
explicit FIB plugin remains the operator's choice. `ze doctor` reports the same
refusal before ze starts: the `plugin-dataplane-writer` check
(`doctor-plugin-no-dataplane-writer`) asks `registry.DataPlaneWriter`, the
judgement `appendDataPlaneWriter` makes, for every `NeedsDataPlane` producer the
config selects, static and `bgp-epe` alike. It keys on the registration, so
static holds no check of its own for it.

The static plugin declares `rib` as a dependency, so the system RIB itself always
runs.

<!-- source: internal/plugins/static/locrib.go -- staticPath, insertPathLocked, the main-table boundary -->
<!-- source: internal/plugins/static/inject.go -- route apply, BFD integration, redistribute emit -->
<!-- source: internal/plugins/static/events/events.go -- RouteChange event registration -->
<!-- source: internal/plugins/static/doctor.go -- the route-skipped check -->
<!-- source: internal/component/plugin/server/startup_autoload.go -- appendDataPlaneWriter, the writer resolution -->
<!-- source: internal/component/plugin/registry/dataplane_writer.go -- DataPlaneWriter, the refusal boot, reload and doctor share -->
<!-- source: internal/component/plugin/doctor/check_dataplane_writer.go -- checkDataPlaneWriter, dataPlaneWriterRows -->

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

A next hop's activity has two inputs, and neither owns it. BFD is the first: it
allows the next hop while its session is up, or while it is not BFD-tracked.
Device presence is the second: a next hop naming a declared on-demand interface
("Table selection and interface-only next hops") is allowed only while its
device is up. A next hop is active only while both allow it
(`nhState.refresh`), so a BFD session coming up never brings a member whose
device is absent into the group, and a device coming up never brings in a member
whose BFD session is down. A change to either input reprograms the route through
the same path: re-inserted into the Loc-RIB with the active set, or rewritten
through the backend, and withdrawn when no member is active. A withdrawal
removes what was last written (`routeState.programmed`), never the configured
route, so a route whose members were never active withdraws nothing, and a
removal never has to resolve a device that was not installed.

Static asks for the BFD service once, when it starts, and its registration
starts it after the bfd plugin (`StartAfter: ["bfd"]`) so the service exists by
then. When the BFD engine stops, every session's subscription closes: the next
hop forgets its session and is active again, untracked, as it is when static
runs without BFD (`routeManager.endVanishedBFDSession`).

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
<!-- source: internal/core/routingtable/config.go -- routing-table config parsing -->
<!-- source: internal/plugins/routingtable/register.go -- plugin registration -->
<!-- source: internal/plugins/static/register.go -- candidateTables -->
<!-- source: internal/plugins/static/register.go -- candidateDeclaredNames, classifyDeclared -->
<!-- source: internal/component/iface/declared_dynamic.go -- DeclaredDynamicNames, dynamicKinds -->
<!-- source: internal/plugins/static/inject.go -- newNHState, nhState.refresh, programRouteLocked -->

The registry is a package-level `atomic.Pointer[Registry]`. A plugin registers
itself in `init()`, which rules out constructor injection, and `bfdapi.GetService()`
had already set this pattern. The routing-table plugin publishes it at boot and
again at the apply of each reload that changes the tables, and a rollback
publishes back the tables the apply replaced.

Static does not resolve a route's table against that registry. It reads the
`routing-table` root (`ConfigReads`), so every commit that reaches static
delivers the candidate routing-table section beside the static one, and static
resolves against that section (`candidateTables`). Every participant verifies
before any applies, so a table added in the same commit as a route in it is not
in the registry yet when static verifies the route; resolving against the
candidate lets the two land together, and refuses a route whose table the
commit deletes.

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
APPLY, in `staticPath`, not at verify, so a name that cannot be resolved does not
fail the transaction: the route is skipped with its reason, the rest of the
section commits, and the route is retried on the next apply ("One bad route does
not drop the section"). The FIB plugin resolves it again when it programs the
route, because the Loc-RIB carries the device NAME and an ifindex is not stable
across a device replacement. Both call `iface.ResolveIndex`, or
`iface.ResolveVPPIndex` on a VPP data plane, so they cannot disagree about what
resolves.

Some interfaces have no device for part of their configured life: iface creates
a pppoe-client's device only while its session is up. Static learns which names
are of that kind from the CANDIDATE `interface` section, read beside
`candidateTables` and for the same reason. `WantsConfig` delivers that section
with the static one, and verify asks iface's pure function
`iface.DeclaredDynamicNames` which of its names are dynamic, so static never
names an interface kind and a kind iface later creates the same way needs no
static edit. A delivery with no `interface` section declares no name. Verify adds
no refusal.

Verify records the answer on each next hop (`nextHop.Dynamic`), so the routes the
pending delivery holds, the apply and the journal's undo each carry the declared
set they were verified with. Apply classifies from that flag and never from what
iface has published, so a pppoe-client and a route through it committed together
do not depend on which participant applies first, and a rollback restores the
previous declared set with the previous routes. A route whose classification
changes is a changed route for the diff (`routesEqual`), so a commit that removes
the client and keeps the route reapplies it.

A next hop naming a declared name is a WAITING member: it is never resolved and
never skipped, and it stays out of the ECMP group (`nhState.waiting`,
`nhState.refresh`). Its route is held in the route map like any other: a mixed
group is programmed through its other active members, and a route with no active
member is withdrawn, or never written, and `static show` lists it with its
members inactive and not skipped. A next hop naming an undeclared name keeps the
resolve-or-skip path above.

The route manager watches each declared name a held route uses, once per name
however many routes and members use it (`linkWatch`, `link.go`). It subscribes
through `iface.Subscribe`, then reads the device's first state through
`iface.Resolve`, so a change between the two still arrives as an event.
`LinkUp` makes the members naming the device active, and `LinkDown`, which the
iface monitor also sends when the device is deleted, makes them wait again. Each
change reprograms the routes whose membership moved, under the lock BFD's
watchers take. A main-table route is re-inserted into the Loc-RIB, and the FIB
plugin resolves the device again when it writes, so a session that comes back
under a new ifindex is followed with no reload. A named-table route is rewritten
by static's backend, which resolves the device at that write. A device that is
up again under a new ifindex, with the down event lost to a full subscriber
channel, still reprograms its routes.

When the device is deleted, the kernel flushes every route through it, so the
withdrawal that follows finds nothing. Both removal owners answer that ESRCH as
done (`core-design.md`, section 16). Static's named-table removal names the
route by table, prefix, metric and protocol and no next hop, so it never has to
resolve a device that is gone.

Watches follow the committed set. Every apply, and the rollback that re-applies
the previous set, ends the watch of a name no held route uses any more, and
`shutdown` ends every watch and waits for its goroutine. Like the BFD service,
the link events are process-local, so a forked static never hears a device come
up and a route through it never installs (`warnIfExternal`).

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

A VPP path via a next-hop address alone names no interface, and carries
`staticvpp.NoInterface` (~0), never 0. VPP reads `sw_if_index` 0 as `local0`:
`fib_path_create` makes any path whose `sw_if_index` is not ~0 an attached
next hop on that interface, so a 0 would never resolve the next hop.

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
given. A next hop the candidate declares dynamic is never one of those: it waits
and is not resolved. A netlink error on a main-table route is reported by the FIB plugin as
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
`OptionalDependencies: ["interface"]`, so static is ordered into a later
startup tier than the iface component and its next hops resolve against a
loaded backend. The edge declares no config need, so the resolver pulls the
interface plugin whenever static loads, with or without an `interface` stanza;
it is optional only in that a build without the interface plugin still runs
static.

Emit tracking uses a per-route `emitted` flag. A replacement, forward or
blackhole or reject on either side, emits no remove, which avoids a
redistribute flap; the new route emits an add, which is idempotent for the
consumer. A forward route whose next hops are all down is withdrawn.

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
