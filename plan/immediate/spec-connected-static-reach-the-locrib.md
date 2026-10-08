# Spec: connected and static reach the Loc-RIB

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 6/7 |
| Handoff | - |
| Updated | 2026-10-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The recorded implementation is at phase 6/7. Final surfaces and closure
evidence remain. Named-table and VRF support stays with
`plan/immediate/spec-fib-nexthop-objects-vpp-metric.md` (the TableID producer,
split out of `plan/immediate/spec-fib-depth.md` on 2026-10-08); it is outside
this spec's main-table scope and does not block that scope.

-> Decision (owner, 2026-10-08): interface-layer route arbitration (DHCP, RA and PPP routes) gets its own spec, `plan/immediate/spec-iface-route-arbitration.md`. It does not block this spec.
-> Decision (owner, 2026-10-08): the RIB owns administrative distance. Protocols hand routes to the Loc-RIB without a protocol-wide distance; the Loc-RIB looks up the configured distance per protocol when ranking and re-runs selection when that configuration changes, so a reload that changes a distance re-ranks installed routes. A static route may still carry its own per-route distance, which overrides the configured static distance for that route.

### Original defect

`rib { distance { } }` (`internal/component/sysrib/yang/ze-rib-conf.yang`)
declares six protocols. Commit `de739c8b2` made four of them decide something,
through the seam `internal/core/rib/distance`: `ebgp` and `ibgp` reach
`locrib.Path.AdminDistance` from the BGP RIB plugin's stamp
(`rib_bestchange.go`), `ospf` and `isis` from their SPF installers, where
`Installer.insert` writes `ribdistance.OrDefault("ospf", in.distance)` into the
Path it inserts, and the IS-IS twin does the same with `"isis"`.

Two leaves decide nothing. `connected` (default 0) and `static` (default 10)
are settable, validated, completed by the editor and printed in the config
reference, and no route-install decision anywhere reads them. An operator can
write `rib { distance { static 250 } }`, reload cleanly, and the kernel will
still prefer the static route over eBGP. Nothing logs it. A configurable value
that is silently inert is the defect `ai/rules/principles.md` names first, and
it is worse than the duplication the distance spec removed, because a duplicate
had two live readers and this has none.

Goal: every leaf in `rib { distance { } }` decides a route-install outcome, by
the same mechanism the other four already use. `connected` and `static`
prefixes become `locrib.Path` values in the shared Loc-RIB, `selectBest` ranks
them against BGP, OSPF and IS-IS on the declared distance, and one writer
programs the kernel.

The two halves are different problems.

| Half | What it is | What changes |
|------|-----------|--------------|
| static | a RELOCATION | the install exists and works; it moves from the static plugin's own netlink handle to the Loc-RIB, so Ze's declared distance arbitrates instead of the Linux FIB resolving the collision by its own rules |
| connected | a REPRESENTATION | there is no install to move and Ze must not start one; the kernel creates a connected route when an address is assigned. The prefix becomes VISIBLE in the Loc-RIB so a path at distance 0 can WIN, without Ze programming a route the kernel already holds |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/static-routes.md` - declared by every static file this
      spec changes, and it carries the DECISION this spec reverses
  → Decision: its "Direct FIB programming, not Loc-RIB injection" section says
    "Loc-RIB was the first design and was abandoned". Its two reasons are
    re-answered here rather than ignored. Reason one, "the pipeline has no
    concept of an ECMP group or a next-hop weight", is now half stale:
    `locrib.Path.ECMP`, `sysribevents.ECMPPath` and
    `buildRichRoute`/`buildMultiPath` carry an ECMP group today, and `Weight`
    exists on `ECMPPath`. What is still missing is a weight ON `locrib.Path`
    and an interface next-hop anywhere. Reason two, "admin-distance arbitration
    adds little because static at 10 already beats eBGP at 20", assumes the
    leaf is a constant; it is a configurable leaf, which is the whole defect.
  → Constraint: the page's claim `RTPROT_ZE (250) is shared with fib-kernel.
    There is no collision` is WRONG in both halves and is repaired by the same
    work. Static's `netlinkStaticBackend.buildRoute` stamps `rtproto.Static`,
    which is 251, not 250; and the partition the page asserts ("fib-kernel owns
    sysrib-derived prefixes, static owns config-driven prefixes") is enforced
    by nothing. The page becomes true by construction once one writer owns the
    main table.
- [ ] `docs/architecture/rib/unified-locrib.md` - declared by
      `locrib/candidate.go` and `locrib/entry.go`, the Path model and
      `selectBest` this spec extends
  → Constraint: `Path` is value-typed and self-contained so copies cross
    component boundaries without pointer aliasing. A next-hop list added for
    static keeps that property: no pointer field, and a slice the producer
    builds once and never mutates, the contract `Labels` already states.
  → Decision: arbitration is `AdminDistance` then `Metric` then first-seen, and
    this spec does not change it. Everything added is carry-through metadata,
    excluded from `key()` and from `selectBest`.
- [ ] `docs/architecture/core-design.md` - declared by `sysrib.go`,
      `fibkernel.go`, `redistevents/registry.go`, `connected/connected.go` and
      `distance/distance.go`
  → Decision: the page describes the cross-protocol RIB and is SILENT on which
    protocols may insert into it. It gains the rule this spec settles: every
    protocol that competes for a main-table prefix inserts a `locrib.Path`, and
    a protocol whose FIB entry the OS creates says so at registration.
- [ ] `docs/architecture/forked-route-install.md` - declared by
      `routeinstall/sink.go` and `plugin/server/dispatch_route.go`
  → Constraint: `locrib.Default` returns nil in a forked subprocess, gated on
    `ze.plugin.hub.token`, so a forked producer holds a `routeinstall.Sink`
    instead. connected and static are both `RunEngine` plugins and can be
    forked, so both need the wiring shape `(*engine).initSPF`
    (`internal/plugins/ospf/spf_wiring.go`) already uses: build the installer
    with `locrib.Default`, and call `SetRemoteSink(routeinstall.New(...))` when
    it came back nil.
  → Constraint: `rpc.RouteInstallEntry` carries no `RouteType` and no ECMP set,
    so a forked producer's blackhole route and its multipath are dropped at the
    boundary today. Static needs both.
- [ ] `docs/architecture/api/process-protocol.md` - declared by
      `sysrib/events/events.go`, the `BestChangeEntry` contract the FIB plugins
      decode
  → Constraint: `BestChangeEntry` is an external JSON contract for a forked FIB
    plugin. A field added here is a wire change, and JSON keys are kebab-case.
- [ ] `docs/architecture/api/ipc_protocol.md` - declared by
      `pkg/plugin/rpc/types.go`, which carries `RouteInstallEntry`
  → Constraint: the same kebab-case JSON contract applies, and the plugin SDK
    is pre-release so a field addition needs no shim.
- [ ] `plan/immediate/spec-fib-depth.md` - declared by `sysrib/ecmp.go`,
      `sysrib/nhresolver.go`, `fib/kernel/richroute.go` and
      `fib/kernel/nexthop_linux.go`; status `in-progress`
  → Decision: it OWNS the table dimension. Its design item "VRF table wired
    through `BestChangeEntry.TableID`" and its AC-9 are unimplemented, and it
    `Depends` on `spec-vrf-0-umbrella`. `TableID` therefore stays unpopulated
    by sysrib in this spec, and static's NAMED-table routes stay on the direct
    netlink path. That is the boundary, not a deferral: see Key Design
    Decisions.
  → Constraint: it also owns `ECMPPath.Weight`, which LANDED on the event but
    is hardcoded to 1 by `ecmpCollect`. Populating it from a producer is this
    spec's work and must not be a second design of the same field.
- [ ] `docs/architecture/rib/forward-handle.md` - declared by
      `locrib/manager.go`, whose `InsertForward` both new producers call
  → Constraint: `InsertForward` takes a `ForwardHandle` that BGP uses to thread
    the forwarding context. connected and static pass nil, as OSPF's
    `(*Installer).insertPath` and its IS-IS twin already do.

### RFC Summaries (Scope: plugin)
- [ ] N-A. Administrative distance is not an RFC concept; `ze-rib-conf.yang`
      says so in the container's `ze:help`: "RFC 4271 mandates no values; these
      follow the Cisco and Juniper convention."
  → Constraint: no RFC text constrains the ordering, so the only obligation is
    that the number an operator writes is the number that decides.

**Key insights:**
- The seam exists and is the mechanism. `ribdistance.OrDefault(protocol,
  fallback)` is the one call a producer makes, and an unset seam reports that
  it did not answer rather than returning 0, because 0 is the BEST distance and
  the one `connected` holds.
- `connectedevents.ProtocolID` and `staticevents.ProtocolID` already exist
  (`RegisterProtocol("connected")`, `RegisterProtocol("static")`), so both
  halves already have a Loc-RIB `Source` identity. Nothing new is registered
  for identity.
- `(*nhResolver).Resolve` terminates on a path whose `NextHop` is invalid, with
  the comment "Connected route: current is the directly-reachable NH". The
  resolver was written expecting connected routes in the Loc-RIB and has never
  seen one.
- The declared distance never reaches a FORKED producer: `distance.Set` has
  exactly one caller, `publishDistances` in `internal/component/sysrib/register.go`,
  which runs in the engine process. This already silently affects OSPF and IS-IS.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/connected/connected.go` - `routeObserver` counts
      prefixes from `interface`/`addr-added` and `addr-removed`, and
      `emit`/`emitID` publish a `redistevents.RouteChangeBatch` tagged
      `connectedevents.ProtocolID`. No `locrib`, `InsertForward` or
      `AdminDistance` reference exists in any non-test file of the package.
- [ ] `internal/plugins/connected/events/events.go` - `ProtocolID =
      RegisterProtocol("connected")` plus `RegisterProducer`.
- [ ] `internal/plugins/static/inject.go` - `(*routeManager).applyRoutes` diffs
      the config set; `applyRouteLocked` and `programRouteLocked` call
      `rm.backend.applyRoute`/`removeRoute` SYNCHRONOUSLY and use the returned
      error for per-route isolation (`rm.skipped`). `emitRouteChange` publishes
      redistevents for advertisement only.
- [ ] `internal/plugins/static/backend_linux.go` - `applyRoute` is netlink
      `RouteReplace`, `removeRoute` is `RouteDel`, both on a route built by
      `buildRoute` with `Protocol` set to `rtproto.Static`, `Priority` set to
      the route's metric and `Table` to the resolved table id. Multipath is a
      `[]*netlink.NexthopInfo` whose `Hops` is the configured weight minus one;
      an interface-only next-hop resolves through `resolveNexthopIndex` to a
      `LinkIndex`.
- [ ] `internal/plugins/static/register.go` - `OnConfigure` and `OnConfigApply`
      call `rm.applyRoutes` and return its error; `OnConfigApply` wraps it in
      an `sdk.Journal` whose `Rollback` re-applies the old set. `OnConfigVerify`
      parses and stages into `pendingSection`.
- [ ] `internal/plugins/static/doctor.go` - `static-route-skipped` reads
      `routeManager.skipped`, which is populated only by a synchronous backend
      error.
- [ ] `internal/plugins/static/yang/ze-static-conf.yang` - `metric` defaults to
      0, so a default static route and a BGP route land at the same kernel
      priority.
- [ ] `internal/component/sysrib/sysrib.go` - `parseAdminDistanceConfig`
      returns every protocol the schema declares; `effectivePriority` applies
      it and warns once per protocol otherwise; `processEvent` stores one
      `protocolRoute` per protocol per prefix; `recomputeBest` selects the
      lowest `priority`, tiebreaks on protocol name, and emits
      Add/Update/Withdraw; `changeToBatch` translates a `locrib.Change`.
      `TableID` is never assigned anywhere in the package.
- [ ] `internal/component/sysrib/ecmp.go` - `ecmpCollect` writes `Weight: 1` on
      every path it builds, inter-protocol and intra-protocol alike.
- [ ] `internal/component/sysrib/nhresolver.go` - `Resolve` walks the Loc-RIB
      by LPM up to `maxRecursionDepth` and returns `Resolved: true` when it
      reaches a path with an invalid `NextHop`.
- [ ] `internal/component/sysrib/register.go` - `publishDistances` installs the
      seam closure; `verifySysRIBConfig` refuses an unparsable `rib` section.
- [ ] `internal/core/rib/locrib/candidate.go` - `Path` fields: `Source`,
      `Instance`, `NextHop`, `AdminDistance`, `Metric`, `Labels`, `IsEBGP`,
      `BackupNextHop`, `BackupRepairLabels`, `ECMP []netip.Addr`, `RouteType`.
      No table, no interface, no per-next-hop weight.
- [ ] `internal/core/rib/locrib/entry.go` - `selectBest`: lower
      `AdminDistance`, then lower `Metric`, then first seen.
- [ ] `internal/core/rib/distance/distance.go` - `Of` returns `(value, ok)`;
      `OrDefault(protocol, fallback)` is the producer form.
- [ ] `internal/core/rib/routeinstall/sink.go` - `(*Sink).InsertForward` builds
      an `rpc.RouteInstallEntry` from a Path; it copies neither `RouteType` nor
      `ECMP`, because the RPC type has no field for either.
- [ ] `internal/component/plugin/server/dispatch_route.go` -
      `applyRouteInstall` re-resolves the protocol NAME to the engine's own
      `ProtocolID` and builds a `locrib.Path`, copying `AdminDistance` straight
      from the wire.
- [ ] `internal/plugins/fib/kernel/backend_linux.go` - `addRoute` is netlink
      `RouteAdd` with `Protocol` set to `rtproto.FIBKernel` and no `Priority`;
      `replaceRoute` is `RouteReplace` with the same shape.
- [ ] `internal/plugins/fib/kernel/nexthop_linux.go` - `buildRichRoute` sets
      `Priority` from the change's metric, `Table` when non-zero, the RTN type,
      and a multipath from `ECMPPaths`. `ECMPPath` has no interface field, so
      the rich path cannot program an interface-only next-hop.
- [ ] `internal/plugins/fib/kernel/fibkernel.go` - `hasRichFields` routes a
      change with a metric, table, ECMP, labels, SRv6 SID, backup or route type
      through the rich backend; failures raise `fib-sync-failure` and a
      `fib-programming-lag` warning after the pending window.
- [ ] `internal/core/redistevents/registry.go` - `RegisterProtocol(name)`
      allocates an ID, `RegisterProducer(id)` marks it a redistribution
      producer and panics on an unknown ID, `ProtocolName` and `ProtocolIDOf`
      map between the two.
- [ ] `internal/core/rtproto/rtproto.go` - `FIBKernel` 250, `Static` 251,
      `PolicyRoute` 252, `Iface` 253; `IsZe` is true for the first three, which
      makes `routewatch` suppress their events for every subscriber.

**What arbitrates TODAY when static and BGP hold one prefix.** Nothing in Ze.
Two independent writers reach the same kernel table with the same destination:

| Writer | Call | rtm_protocol | Priority | Table |
|--------|------|--------------|----------|-------|
| static | `RouteReplace` | 251 | the route's `metric`, default 0 | the configured table, 0 = main |
| fib-kernel, plain | `RouteAdd` | 250 | unset (0) | main |
| fib-kernel, rich | `RouteAdd` / `RouteReplace` | 250 | the change's `Metric` | `TableID`, never set by sysrib, so main |

The Linux FIB keys an entry on table, destination, tos and priority, and not on
rtm_protocol. With both at the default metric the two writers address the same
entry, so the outcome is decided by write ORDER and by which netlink verb each
side used, not by any number Ze computed. Ze's declared distance participates
nowhere. `A-4` records what must be measured before the fix, and `AC-11`
requires it to be measured RED.

**Behavior to preserve:**
- `redistevents` stays the redistribution bus and never installs. Both plugins
  keep emitting on it, unchanged, so `redistribute { import static }` and
  `import connected` are untouched.
- Static's config-transaction contract: an unparsable or unresolvable route
  still fails `OnConfigVerify`/`OnConfigure` synchronously, and `OnConfigApply`
  still rolls the previous set back through `sdk.Journal`.
- Static's named-table routes keep their current path, protocol stamp and
  behavior in every respect.
- `selectBest` ordering: distance, then metric, then first seen.
- Connected routes are never programmed by Ze. The OS owns them.

**Behavior to change:**
- `rib { distance { connected } }` and `{ static }` decide which route the
  kernel forwards on.
- A main-table static route is stamped `RTPROT_ZE` (250) by fib-kernel instead
  of `RTPROT_STATIC` (251) by the static plugin.
- A next-hop covered only by a connected interface prefix becomes resolvable
  through `nhResolver`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Connected: an `interface` / `addr-added` or `addr-removed` EventBus message
  carrying the JSON `addrPayload` (name, address, prefix-length, family).
- Static: a `static` config section delivered to `OnConfigVerify` /
  `OnConfigure` / `OnConfigApply` as `sdk.ConfigSection` JSON.
- Both: `rib { distance { } }` delivered to sysrib's configure callback, which
  publishes the resolved table through `distance.Set`.

### Transformation Path
1. The producer resolves the prefix (connected: `toNetworkPrefix`; static:
   `parseStaticConfig` plus `routingtable.Registry.Resolve`).
2. The producer stamps `AdminDistance` with `ribdistance.OrDefault(name,
   bootstrap)` and calls `InsertForward` on the RIB from `locrib.Default`, or
   buffers into a `routeinstall.Sink` when forked.
3. Forked only: the engine's `applyRouteInstall` rebuilds the `locrib.Path`,
   re-resolving the protocol name to its own `ProtocolID` and RE-STAMPING
   `AdminDistance` from the declaration, because the seam is engine-only.
4. `(*PathGroup).upsert` runs `selectBest` across every source for the prefix
   and the shard dispatches a `Change`.
5. `changeToBatch` maps the Change to sysrib's batch; `processEvent` stores one
   `protocolRoute`, replacing the whole per-prefix entry because
   `FromLocRIB` is set; `recomputeBest` resolves the next-hop and emits.
6. A winner whose protocol declared that the OS installs its routes produces a
   WITHDRAW of Ze's own FIB entry rather than an install.
7. `publishChanges` emits `(system-rib, best-change)`; `(*fibKernel).processEvent`
   programs netlink as `RTPROT_ZE`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| connected/static plugin ↔ Loc-RIB, in-process | `(*RIB).InsertForward` / `Remove`, value-typed `locrib.Path` | No |
| connected/static plugin ↔ engine, forked | `ze-plugin-engine:route-install` / `:route-remove` RPC, `rpc.RouteInstallEntry` JSON, kebab-case | No |
| Loc-RIB ↔ sysrib | `locrib.Change` through the bounded channel drained by `(*sysRIB).run`'s worker | No |
| sysrib ↔ FIB plugins | `(system-rib, best-change)`, `sysribevents.BestChangeEntry` JSON | No |
| sysrib ↔ redistevents registry | `ProtocolIDOf` plus the new OS-installed property | No |
| fib-kernel ↔ kernel | netlink `RouteAdd`/`RouteReplace`/`RouteDel`, `RTPROT_ZE` | No |

### Integration Points
- `internal/core/rib/distance.OrDefault` - the one call each producer makes to
  read its declared distance.
- `internal/core/rib/locrib.(*RIB).InsertForward` / `Remove` - the insertion
  point OSPF and IS-IS already use; connected and static join it unchanged.
- `internal/core/rib/routeinstall.New` - the forked sink, wired exactly as
  `(*engine).initSPF` in `internal/plugins/ospf/spf_wiring.go` wires it.
- `internal/core/redistevents.RegisterProducer` - the existing precedent for
  declaring a property of an already-registered protocol; the OS-installed
  declaration sits beside it.
- `internal/plugins/fib/kernel` `richRouteBackend` - already programs metric,
  route type, ECMP and table, so static's blackhole, reject and multipath need
  no new backend.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | this is the point of the spec: static's direct netlink write for a main-table route is the bypassed layer being removed |
| No unintended coupling (components stay isolated) | No | no generic package names "connected" or "static"; the OS-installed property is REGISTERED by the plugin and READ by sysrib through the ID |
| No duplicated functionality (extends existing, does not recreate) | No | the second kernel writer is deleted, not duplicated; `ecmpCollect`'s hardcoded weight is replaced, not paralleled |
| Zero-copy preserved where applicable (refs, not copies) | No | the next-hop slice on `Path` follows the `Labels` contract: built once per change, shared, never mutated |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | `connectedevents` declares the OS-installed property in its own `init()`; sysrib asks the registry and never spells a plugin name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `connectedevents.ProtocolID` and `staticevents.ProtocolID` give both halves a usable Loc-RIB `Source` with no new identity registration | `connected/events/events.go`, `static/events/events.go`, both call `redistevents.RegisterProtocol` | a second identity would have to be registered and `WouldLoop` re-checked for redistribution | `TestConnectedPathCarriesTheRegisteredSource`, `TestStaticPathCarriesTheRegisteredSource` | unvalidated |
| A-2 | A connected `Path` with an invalid `NextHop` is what `(*nhResolver).Resolve` treats as its terminator | the `!path.NextHop.IsValid()` branch in `Resolve` and its "Connected route" comment | connected paths would resolve to nothing and BGP routes over them would report unreachable | `TestNextHopResolvesThroughAConnectedPrefix` | unvalidated |
| A-3 | Everything sysrib publishes today lands in the kernel's MAIN table, so the main-table boundary in Key Design Decisions costs no existing behavior | grep for `TableID` across `internal/component/sysrib`: the only hit is the field declaration on `BestChangeEntry` | a protocol already programming a named table would lose its table | `TestSysRIBEmitsNoTableID`, plus the QEMU test asserting `ip route show table main` | unvalidated |
| A-4 | Today a default-metric static route and a BGP route for one prefix collide on a single kernel FIB entry, so which one forwards depends on write order rather than on any Ze decision | `buildRoute` sets `Priority` from a `metric` that defaults to 0, and fib-kernel's `addRoute` sets no Priority; the Linux FIB does not key on rtm_protocol | the defect is narrower than stated and the risk of the relocation changes shape | a QEMU test on TODAY's code that configures both and reads `ip route get`, recorded before any change (AC-11) | unvalidated |
| A-5 | The distance an operator writes never reaches a FORKED producer today, because `distance.Set` has one caller and it runs in the engine | `grep distance.Set` returns only `publishDistances` in `internal/component/sysrib/register.go`; `locrib.Default` gates on `ze.plugin.hub.token` | the engine-side re-stamp is unnecessary and adds a layer | `TestForkedProducerDistanceIsRestampedByTheEngine`, driven through `applyRouteInstall` | unvalidated |
| A-6 | Static's synchronous failures that matter to an operator are RESOLUTION failures (interface, table, bounds), which stay inside the plugin, and not netlink write failures | `applyRouteLocked` records `rm.skipped` from any backend error; `resolveNexthopIndex` and `validateRouteMetric` are the resolution failures inside `buildRoute` | the `static-route-skipped` doctor check loses cases and an operator loses a diagnosis | `TestStaticRefusesAnUnresolvableNextHopBeforeInsert`, plus the doctor check's own test | unvalidated |
| A-7 | `sdk.Journal` rollback stays meaningful when the apply is an insert rather than a program: re-applying the old route set re-inserts the old Paths | `OnConfigApply` records apply and rollback closures over `rm.applyRoutes` | a failed transaction leaves the Loc-RIB holding the new set | `TestStaticRollbackRestoresThePreviousPathSet` | unvalidated |
| A-8 | `RouteInstallEntry` gaining `RouteType` and an ECMP list breaks nothing, because the plugin SDK is pre-release and no out-of-tree consumer exists | `CLAUDE.md`, "Ze is PRE-RELEASE"; the type is JSON with omitempty siblings | an SDK consumer would need a migration | `./le verify worktree`, plus the existing forked OSPF/IS-IS route-install tests | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Connected prefixes entering the Loc-RIB change RECURSIVE NEXT-HOP RESOLUTION for every protocol. `srv6SIDResolvable` returns false today for a SID covered only by a connected prefix, and `recomputeBest` WITHDRAWS on that; after the change it resolves and the route installs | an existing sysrib or SRv6 test flips, or a prefix appears in the kernel that did not before | this is a correction, not a regression, and it is stated as AC-4; the change must carry a test that pins the NEW behavior and a QEMU test that reads the kernel |
| R-2 | A connected prefix at distance 0 wins against every protocol for the same prefix, which is correct and also means a Ze-programmed route DISAPPEARS from the kernel the moment an operator assigns an overlapping interface address | a prefix vanishes from `ip route` after an address is configured | AC-5 makes the withdraw explicit and logged once per prefix; the alternative, leaving a stale Ze route beside the kernel's own, is the defect this half exists to remove |
| R-3 | Moving static's install off the synchronous path loses the transaction's ability to fail on a netlink error. A route that resolves but that netlink refuses now surfaces as a `fib-sync-failure` report, and the config commit succeeds | a functional test that expects a commit to fail on a bad route passes instead | A-6 splits the failure classes and keeps every resolution failure synchronous; the doctor check's text narrows to what it can still see, in the same change |
| R-4 | Two writers exist during implementation. A half-landed state has static inserting Paths AND writing netlink, so one prefix gets two kernel entries at two protocols | duplicated prefixes in `ip route` with proto 250 and 251 | `ai/rules/no-layering.md`: the direct write for main-table routes is DELETED in the same phase that adds the insert, never left as a fallback; the phase order in Implementation Steps enforces it |
| R-5 | The Loc-RIB is keyed by (family, prefix) with no table dimension, so a named-table static route inserted by mistake would collide with a main-table route for the same prefix and one would be lost | a policy-routing test loses a route | the main-table boundary is a guard with a test, not a convention: `TestNamedTableStaticRouteNeverReachesTheLocRIB` |
| R-6 | Static's BFD-driven next-hop subsetting reprograms on every BFD transition. Through the Loc-RIB that becomes an insert per transition, adding a shard write and a sysrib hop to a failure-detection path | BFD failover latency regresses in the QEMU test | the insert is one shard write under an existing lock and no allocation beyond the next-hop slice; the QEMU BFD test asserts the surviving next-hop is programmed, and a latency regression is a finding, not an accepted cost |
| R-7 | `ecmpCollect` currently writes `Weight: 1` for INTER-protocol equal-cost paths too. Populating weight from the producer must not silently change what an equal-cost BGP or IS-IS group programs | a multipath kernel route changes shape for a protocol this spec does not touch | producers that state no weight keep 1, and the test that pins it names the protocols: `TestEqualCostGroupKeepsWeightOneForProducersThatStateNone` |
| R-8 | The engine-side re-stamp of `AdminDistance` overwrites a distance a forked producer chose deliberately per route | a forked producer's per-route distance stops taking effect | no producer sets a per-route distance today: all four stamp one protocol-wide value through `OrDefault`. The wire value becomes the FALLBACK, so a producer whose protocol the declaration does not name keeps what it sent |
| R-9 | `routewatch` suppresses events for every protocol `rtproto.IsZe` reports true for, which includes both 250 and 251, so neither writer's own churn is visible to the FIB monitor. Moving static to 250 changes which sweeper owns its routes: `sweepStale` will now delete a main-table static route sysrib did not refresh | a static route disappears after the sweep delay on a slow boot | this is a GAIN (crash recovery now covers static) and a hazard; the sweep-delay path is tested by `TestStartupSweepKeepsARefreshedStaticRoute` |
| R-10 | The package is large. Two producers, a Loc-RIB contract extension, an RPC extension, a sysrib publish decision and a deleted kernel writer | the implementing session reaches its budget mid-phase | the halves are independent and phase-ordered: connected (phases 1-3) delivers a working leaf on its own, static (phases 4-6) delivers the second. If the budget forces a cut, the CUT IS AT A PHASE BOUNDARY reported to the main thread, never inside an AC |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The kernel forwarding table. A wrong winner blackholes a prefix, and a wrong withdraw removes a route an operator configured. Connected's half additionally changes recursive next-hop resolution for BGP, OSPF, IS-IS and SRv6 |
| How is it reverted? | Single commit revert per half, with no config migration: the YANG leaves already exist and keep their values. A reverted static half restores `RTPROT_STATIC` on the next config apply, and a reverted connected half stops inserting; neither leaves state behind |
| Who else touches this path? | `plan/immediate/spec-fib-nexthop-objects-vpp-metric.md` owns `BestChangeEntry.TableID` and `plan/immediate/spec-fib-depth.md` (in-progress) owns `ECMPPath.Weight`; `spec-fixit-bgp-distance-declaration` closed on 2026-09-05 and left the distance seam this spec consumes at `internal/core/rib/distance` (`Of`, `OrDefault`), published by `publishDistances` (`internal/component/sysrib/register.go`); read the seam rather than the closed spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interface`/`addr-added` EventBus message | → | `(*routeObserver).handleAddrAdded` → `insertPath` → `(*RIB).InsertForward` | `TestAddrAddedInsertsAConnectedPath` |
| `interface`/`addr-removed` EventBus message | → | `(*routeObserver).handleAddrRemoved` → `removePath` → `(*RIB).Remove` | `TestAddrRemovedWithdrawsTheConnectedPath` |
| `rib { distance { connected 0 } }` config | → | `ribdistance.OrDefault("connected", 0)` at the connected stamp site | `TestConnectedStampsTheDeclaredDistance` |
| `rib { distance { static 250 } }` config | → | `ribdistance.OrDefault("static", 10)` at the static stamp site, then `selectBest` | `test/static/static-distance-loses-to-ebgp.ci` |
| `static { route ... }` config section | → | `(*routeManager).applyRouteLocked` → `insertPath` → `(*RIB).InsertForward` | `TestStaticApplyInsertsAPathPerRoute` |
| A connected prefix winning `selectBest` | → | `(*sysRIB).recomputeBest` → withdraw branch for an OS-installed winner | `TestOSInstalledWinnerWithdrawsTheZeRoute` (`internal/component/sysrib/sysrib_osinstalled_test.go`) |
| A forked connected plugin's insert | → | `(*Sink).InsertForward` → `applyRouteInstall` → the engine's `locrib.RIB` | `TestSinkInsertForwardMarshalsEntry` (`internal/core/rib/routeinstall/sink_test.go`) and `TestForkedProducerDistanceIsRestampedByTheEngine` (`applyRouteInstall` then a Lookup in the engine Loc-RIB) |
| An operator's `ip addr add` on a booted appliance | → | the whole chain, ending at netlink | `test/plugin/connected-distance-arbitration.ci` (reads the kernel table: "won by connected, ze-programmed=no"); the QEMU-guest run of it is OWED |
| An operator's `static` config on a booted appliance | → | the whole chain, ending at netlink `RTPROT_ZE` | `test/static/static-distance-loses-to-ebgp.ci` reads the system RIB winner only; a test reading the kernel entry (`proto 250`, the winner's next-hop) is OWED |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An interface address is configured for 10.0.0.0/24 | A Loc-RIB path exists for 10.0.0.0/24 with source `connected`, an invalid next-hop and the declared connected distance |
| AC-2 | The interface address is removed | The Loc-RIB path for that prefix is gone, and any lower-preference path for the same prefix becomes the winner |
| AC-3 | `rib { distance { connected 250 } }` and a BGP path for the same prefix | The BGP path wins `selectBest` and the kernel forwards on it |
| AC-4 | A BGP route whose next-hop is covered only by a connected interface prefix | `(*nhResolver).Resolve` reports resolved, and the resolved direct next-hop is the BGP next-hop itself |
| AC-5 | A connected path wins a prefix Ze had programmed | sysrib emits a withdraw, fib-kernel deletes the `RTPROT_ZE` route, the prefix stays in the system RIB as connected-won, and one log line names the prefix and the reason |
| AC-6 | A connected path wins a prefix Ze had NOT programmed | No FIB event is emitted for that prefix and no kernel route is created by Ze |
| AC-7 | `static { route 10.0.0.0/8 { forward { next { 192.0.2.1 } } } }` with no table | One Loc-RIB path exists with source `static` and the declared static distance, and the kernel route for it carries `proto 250` |
| AC-8 | `rib { distance { static 250 } }`, plus a static and an eBGP route for one prefix | The eBGP path wins, the kernel forwards on the BGP next-hop, and no second entry for the prefix exists |
| AC-9 | `rib { distance { static 5 } }`, same two routes | The static path wins and the kernel forwards on the static next-hop |
| AC-10 | `static { table blue { route ... } }` | The route is programmed directly into table blue, no Loc-RIB path is created for it, and no main-table entry appears |
| AC-11 | The tree BEFORE this change, with a default-metric static route and a BGP route for one prefix | The QEMU test that asserts distance-driven selection FAILS, and the failure is recorded (`ai/rules/interop-and-goal-validation.md`) |
| AC-12 | A static route with two weighted next-hops in the main table | The kernel multipath route carries both, with the configured weights, through `ECMPPath.Weight` |
| AC-13 | A static route whose only next-hop is an interface name | The kernel route carries that interface's index, reached through the Loc-RIB path rather than through the static backend |
| AC-14 | A static blackhole route in the main table | The kernel route is `RTN_BLACKHOLE` with `proto 250` |
| AC-15 | A static route whose next-hop interface cannot be resolved | The config apply fails synchronously with the existing message, no Loc-RIB path is inserted, and the `static-route-skipped` doctor check reports it |
| AC-16 | A config transaction that applies a new static set and then aborts | The Loc-RIB holds the previous set, and the kernel matches it |
| AC-17 | A FORKED connected or static plugin, with `rib { distance { static 5 } }` written | The path the engine's Loc-RIB holds ranks at distance 5; the forked plugin sends no protocol-wide distance |
| AC-18 | BFD marks one of a static route's two next-hops down | The Loc-RIB path is re-inserted with the surviving next-hop and the kernel route is reprogrammed |
| AC-19 | Any protocol neither the declaration nor the bootstrap table names inserts a path with no override | The Loc-RIB ranks it at `locrib.UndeclaredDistance` (255), never at zero; no silent zero exists |
| AC-20 | Static route and eBGP route installed under `static 5`, then a reload to `static 250` with no other change | The Loc-RIB re-runs selection (`(*RIB).Reselect`, called by sysrib after it publishes), BGP wins, and the kernel moves to the BGP next-hop with one entry left (owner decision 2026-10-08) |
| AC-21 | A static route with its own `distance N` leaf | The route ranks at N whatever `rib { distance { static } }` says, and a reload of the declared static distance leaves it at N |
| AC-22 | A reload that changes `ebgp`, `ibgp`, `ospf` or `isis` | Installed paths of that protocol are re-ranked against an installed static route, both ways |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Assigns 10.0.0.0/24 to an interface while a peer announces 10.0.0.0/24 | iface event -> connected -> Loc-RIB -> selectBest -> sysrib withdraw -> fib-kernel delete | `test/plugin/connected-distance-arbitration.ci` |
| 2 | Writes `rib { distance { static 250 } }` and expects BGP to win a prefix both offer | config -> sysrib -> distance seam -> static stamp -> selectBest -> fib-kernel | `test/static/static-distance-loses-to-ebgp.ci` (system RIB winner) and `test/static/static-kernel-distance-bgp-wins.ci` (one kernel entry, proto 250, via the BGP next-hop) |
| 3 | Configures a weighted two-next-hop static route and reads `ip route` | config -> static -> Loc-RIB path with weights -> sysrib -> rich route -> netlink multipath | `test/static/static-kernel-weighted-multipath.ci` (kernel hops carry weight 3 and weight 1, proto 250); unit `TestStaticPathCarriesWeightedNextHops`, `TestBestChangeCarriesTheDeclaredWeights` |
| 4 | Configures a static route in table blue for policy routing | config -> static -> direct backend, unchanged | `test/static/static-named-table-unchanged.ci` (table 171, proto 251, absent from main and `show rib`) and `test/static/static-table-interface.ci` |
| 5 | Runs `show rib` after an interface address is configured | connected path -> Loc-RIB -> sysrib `(*sysRIB).showRIB` | `TestOSInstalledWinnerStaysInTheSystemRIB` (calls `showRIB`) |
| 6 | Runs the doctor after configuring a static route with an unresolvable interface | static resolution failure -> `rm.skipped` -> `static-route-skipped` | `test/static/static-per-route-isolation.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAddrAddedInsertsAConnectedPath` | `internal/plugins/connected/locrib_test.go` | AC-1: source, invalid next-hop, distance | exists |
| `TestAddrRemovedWithdrawsTheConnectedPath` | `internal/plugins/connected/locrib_test.go` | AC-2 | exists |
| `TestConnectedRanksAtTheDeclaredDistance` with `TestConnectedPathCarriesNoDistance` | `internal/plugins/connected/locrib_test.go` | the producer stamps no distance and the Loc-RIB ranks connected at the declared value | exists (planned as `TestConnectedStampsTheDeclaredDistance`; renamed when the RIB took ownership of distance) |
| `TestConnectedRefcountInsertsOnce` | `internal/plugins/connected/locrib_test.go` | two addresses in one prefix produce one path and one withdraw | exists |
| `TestConnectedPathCarriesTheRegisteredSource` | `internal/plugins/connected/locrib_test.go` | A-1 | exists |
| `TestOSInstalledIsDeclaredNotDerived` | `internal/core/redistevents/registry_test.go` | the property is registered, and an unregistered ID is reported as unknown rather than false | exists |
| `TestOSInstalledWinnerWithdrawsTheZeRoute` | `internal/component/sysrib/sysrib_osinstalled_test.go` | AC-5 | exists (planned as `TestConnectedWinnerWithdrawsTheZeRoute`) |
| `TestOSInstalledWinnerEmitsNothingWhenNothingWasProgrammed` | `internal/component/sysrib/sysrib_osinstalled_test.go` | AC-6 | exists (planned as `TestConnectedWinnerEmitsNothingWhenNothingWasProgrammed`) |
| `TestOSInstalledLoserLeavesTheZeRouteProgrammed` | `internal/component/sysrib/sysrib_osinstalled_test.go` | AC-3 | exists (planned as `TestConnectedLoserLeavesTheZeRouteProgrammed`) |
| `TestConnectedPrefixIsWhatTheResolverTerminatesOn` with `TestRecursiveNHResolve_DirectlyConnected` | `internal/plugins/connected/locrib_test.go`, `internal/component/sysrib/nhresolver_test.go` | AC-4, A-2 | exists (planned as `TestNextHopResolvesThroughAConnectedPrefix`) |
| `TestResolvableSRv6SIDIsProgrammed` | `internal/component/sysrib/sysrib_srv6_test.go` | R-1: a SID whose locator is covered by a connected path resolves and is programmed | exists (planned as `TestSRv6SIDResolvesThroughAConnectedPrefix`) |
| `TestSysRIBEmitsNoTableID` | `internal/component/sysrib/sysrib_nexthop_detail_test.go` | A-3, the main-table boundary | exists |
| `TestStaticApplyInsertsAPathPerRoute` | `internal/plugins/static/locrib_test.go` | AC-7 | exists |
| `TestStaticPathCarriesTheRegisteredSource` | `internal/plugins/static/locrib_test.go` | A-1 | exists |
| `TestStaticPathCarriesNoProtocolDistance`, `TestStaticLosesToEBGPAtARaisedDistance`, `TestStaticBeatsEBGPAtALoweredDistance` | `internal/plugins/static/locrib_test.go` | AC-8, AC-9: static stamps no protocol distance and the Loc-RIB ranks it at the declared value | exists (planned as `TestStaticStampsTheDeclaredDistance`) |
| `TestStaticPathCarriesTheRouteOwnDistance` | `internal/plugins/static/locrib_test.go` | AC-21 at the producer: the per-route `distance` leaf reaches the Path as an override, 0 included | exists |
| `TestReloadReranksStaticAgainstBGP` | `internal/core/rib/locrib/reselect_test.go` | AC-20: `static 5` to `static 250` hands the installed prefix to BGP through `Reselect` and tells the FIB | exists; red with `Reselect` stubbed |
| `TestReselectIsQuietWhenNothingMoved` | `internal/core/rib/locrib/reselect_test.go` | `Reselect` dispatches nothing for a prefix whose ranking did not change | exists |
| `TestStaticOwnDistanceOverridesTheDeclared` | `internal/core/rib/locrib/reselect_test.go` | AC-21: the per-route override wins over the declared static distance and survives its reload | exists |
| `TestEachProtocolDistanceChangeReranks` | `internal/core/rib/locrib/reselect_test.go` | AC-22: `ebgp`, `ibgp`, `ospf`, `isis` each re-rank against a static route, both ways | exists; red with `Reselect` stubbed |
| `TestNamedTableStaticRouteNeverReachesTheLocRIB` | `internal/plugins/static/locrib_test.go` | AC-10, R-5 | exists |
| `TestStaticRefusesAnUnresolvableNextHopBeforeInsert` | `internal/plugins/static/locrib_test.go` | AC-15, A-6 | exists |
| `TestStaticRollbackRestoresThePreviousPathSet` | `internal/plugins/static/register_test.go` | AC-16, A-7 | exists. Drives `applyRouteSet` (the OnConfigApply body, extracted so the journal is testable) over a real `locrib.RIB`, then the journal's undo as OnConfigRollback runs it; asserts the Loc-RIB holds the previous set prefix by prefix. Red: undo re-applying the new set gives "after rollback the Loc-RIB holds map[10.0.0.0/8:192.0.2.9 198.51.100.0/24:invalid IP], want the previous set map[10.0.0.0/8:192.0.2.1 172.16.0.0/12:192.0.2.1]"; restored green |
| `TestStaticBFDDownReinsertsTheSurvivingNextHop` | `internal/plugins/static/locrib_test.go` | AC-18 | exists |
| `TestStaticBlackholeCarriesTheRouteType` | `internal/plugins/static/locrib_test.go` | AC-14 | exists |
| `TestOnChangeCarriesBestPathECMP` with `TestBestChangeCarriesTheDeclaredWeights` | `internal/core/rib/locrib/locrib_test.go`, `internal/component/sysrib/sysrib_nexthop_detail_test.go` | the next-hop list is carry-through, and the weights reach the best-change | exists (planned as `TestPathCarriesWeightedNextHops`; no test asserts the weighted list is excluded from `key()`) |
| `TestEqualCostGroupKeepsWeightOneForProducersThatStateNone` | `internal/component/sysrib/sysrib_nexthop_detail_test.go` | R-7 | exists |
| `TestECMPPathCarriesTheInterface` | `internal/component/sysrib/sysrib_nexthop_detail_test.go` | AC-13 through the event contract | exists |
| `TestForkedProducerDistanceIsRestampedByTheEngine` | `internal/component/plugin/server/dispatch_route_restamp_test.go` | AC-17, A-5 | removed with the engine restamp: the Loc-RIB resolves the distance for every producer, forked or not, so no restamp is left to test |
| `TestRouteInstallEntryCarriesRouteTypeAndECMP` | `internal/component/plugin/server/dispatch_route_restamp_test.go` | A-8, the forked static blackhole and multipath | exists |
| `TestSinkInsertForwardMarshalsEntry` with the `applyRouteInstall` helper of `dispatch_route_restamp_test.go` | `internal/core/rib/routeinstall/sink_test.go` | the sink wiring | exists (planned as `TestForkedConnectedInsertReachesTheEngineRIB`; neither uses the connected source) |
| `TestOSInstalledWinnerStaysInTheSystemRIB` | `internal/component/sysrib/sysrib_osinstalled_test.go` | user story 5 (`showRIB` lists the connected winner) | exists (planned as `TestShowRIBListsAConnectedWinner`) |
| `TestFIBKernelSweepStale` | `internal/plugins/fib/kernel/fibkernel_test.go` | R-9: a refreshed proto-250 route survives the sweep, whichever producer it came from | exists, protocol-agnostic (planned as `TestStartupSweepKeepsARefreshedStaticRoute`) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `rib distance connected` | 0-255 | 255 | N/A (uint8, 0 is valid and is the classical value) | N/A |
| `rib distance static` | 1-255 | 255 | 0, refused by the YANG `range` | N/A |
| static route `metric` | 0-4294967295 | 4294967295 | N/A | N/A, bounded by `validateRouteMetric` against `maxNetlinkInt` |
| static next-hop `weight` | 1-255 | 255 | 0 | N/A, `ECMPPath.Weight` is uint8 |
| ECMP group size | 1-128 | 128 | 0 | 129, truncated at `MaxECMPPaths` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `static-distance-loses-to-ebgp` | `test/static/static-distance-loses-to-ebgp.ci` | operator raises the static distance above eBGP and BGP takes the prefix | |
| `static-distance-beats-ebgp` | `test/static/static-distance-beats-ebgp.ci` | the default static distance keeps the prefix | |
| `static-named-table-unchanged` | `test/static/static-named-table-unchanged.ci` | a policy-routing route still lands in its table | |
| `connected-distance-arbitration` | `test/plugin/connected-distance-arbitration.ci` | a connected prefix outranks a BGP path and Ze programs nothing for it | |
| `connected-distance-raised-loses` | `test/plugin/connected-distance-raised-loses.ci` | `rib { distance { connected 250 } }` hands the prefix back to BGP | |
| `static-per-route-isolation` | `test/static/static-per-route-isolation.ci` | existing test, updated for the narrowed skip classes (AC-15) | updated in `1e3328ef53`: asserts fib-kernel's refused add for the kernel-unreachable gateway, rejects static's "route skipped" |
| `static-kernel-distance-static-wins` | `test/static/static-kernel-distance-static-wins.ci` | `static 5`: the kernel holds exactly one entry for 10.0.0.0/8, proto 250, via the static next-hop (AC-9, AC-11) | green 2026-10-08; red recorded below |
| `static-kernel-distance-bgp-wins` | `test/static/static-kernel-distance-bgp-wins.ci` | `static 250`: exactly one kernel entry in any table, proto 250, via the BGP next-hop (AC-8, AC-11) | green 2026-10-08; red recorded below |
| `static-kernel-weighted-multipath` | `test/static/static-kernel-weighted-multipath.ci` | the kernel multipath entry carries `weight 3` and `weight 1` on its two hops, proto 250 (AC-12) | green 2026-10-08; red recorded below |
| `static-kernel-interface-nexthop` | `test/static/static-kernel-interface-nexthop.ci` | an interface-only route leaves by `dev zentk`, proto 250, with no gateway (AC-13) | green 2026-10-08; red recorded below |
| `static-kernel-distance-reload` | `test/static/static-kernel-distance-reload.ci` | a reload from `static 5` to `static 250` with no other change moves the kernel entry from the static to the BGP next-hop, one entry left, and `show rib` names bgp (AC-20) | green 2026-10-08 under `unshare -rn`; red recorded in Goal Validation |
| `static-kernel-distance-route-override` | `test/static/static-kernel-distance-route-override.ci` | a route with its own `distance 3` keeps exactly one kernel entry, proto 250, via the static next-hop against eBGP 20 across a reload from `static 5` to `static 250`, while a witness route without one moves to the BGP next-hop (AC-21) | green 2026-10-08 under `unshare -rn`; red recorded in Goal Validation |

### QEMU Integration Tests (Linux-only paths, `ai/rules/platform-linux.md`)

None of the five Go tests planned here exists under its planned name (checked
2026-10-08: no `func TestQEMU` in the static, connected, sysrib or fib-kernel
packages). Every planned behavior is covered by a `.ci` test that reads the
installed kernel state under CAP_NET_ADMIN, which runs in the QEMU guest through
`./le test qemu` and was run here inside an unprivileged user network namespace
(`unshare -rn`, which grants CAP_NET_ADMIN over a private stack, no root).

| Planned test | Covered by | What it reads from the kernel | Status |
|------|----------|-------------------------------|--------|
| `TestQEMUStaticDistanceDecidesAgainstBGP` | `test/static/static-kernel-distance-{static,bgp}-wins.ci` | exactly one entry for the prefix across all tables, `proto 250`, via the next-hop of the winner the distance selects | covered |
| `TestQEMUConnectedBeatsBGPForTheSamePrefix` | `test/plugin/connected-distance-arbitration.ci` | the prefix is won by connected and Ze programmed nothing for it (`ze-programmed=no`) | covered |
| `TestQEMUStaticWeightedMultipath` | `test/static/static-kernel-weighted-multipath.ci` | `nexthop via 192.0.2.1 dev zentk weight 3` and `nexthop via 192.0.2.3 dev zentk weight 1`, proto 250 | covered |
| `TestQEMUStaticInterfaceNextHop` | `test/static/static-kernel-interface-nexthop.ci` | `198.18.74.0/24 dev zentk proto 250`, no `via` | covered |
| `TestQEMUNamedTableStaticUnchanged` | `test/static/static-named-table-unchanged.ci` | table 171 holds the route as proto 251 via the gateway; the main table and `show rib` do not | covered |

Discrimination, each run 2026-10-08 under `unshare -rn` with a pristine copy saved
first and restored after (no `MUTATION-APPLIED` left in the tree):

| Test | Break applied to the producer | Red observed | Restored |
|------|-------------------------------|--------------|----------|
| `static-kernel-distance-bgp-wins` (AC-11) | `staticRoute.inMainTable` returns false, so main-table routes take the static plugin's own netlink write: the behavior before `125979ea99` | `10.0.0.0/8 won by bgp: want exactly one kernel entry, proto 250 via 198.51.100.1; ip route show table all: ["10.0.0.0/8 via 192.0.2.1 dev zentk proto 251"]`: BGP won the system RIB and the kernel forwarded on the static route anyway | green |
| `static-kernel-distance-static-wins` (AC-11) | same break | `10.0.0.0/8: system RIB winner is "bgp", want "static"` | green |
| `static-kernel-weighted-multipath` (AC-12) | `staticPath` stamps weight 1 on every hop | `ip route: "198.18.73.0/24 proto 250 nexthop via 192.0.2.1 dev zentk weight 1 nexthop via 192.0.2.3 dev zentk weight 1"` | green |
| `static-kernel-interface-nexthop` (AC-13) | `staticPath` drops `Interface` from the path | `198.18.74.0/24: want a proto 250 entry out of dev zentk; ip route: ""` | green |
| `static-kernel-distance-reload` (AC-20) | `(*RIB).Reselect` returns at entry, so a reload publishes the new declaration and re-ranks nothing already installed | `after the reload to static 250: want exactly one kernel entry, proto 250 via 198.51.100.1; ip route show table all: ["10.0.0.0/8 via 192.0.2.1 dev zentk proto 250 "]`: the reload logged `static:250` and the kernel kept forwarding on the static next-hop | green (PASS 5.6s) |

AC-11's literal wording asks for a run on the tree before `125979ea99`. That tree
was not checked out; the break above reproduces its producer behavior (the
direct RTPROT_STATIC write for main-table routes) on the current tree, which is
the condition the commit's fix removed.

Load note: `static-kernel-weighted-multipath` reached the 30s `.ci` timeout twice
(no observer output, the daemon had not logged "static routes loaded") at load
average 10-16, and passed in every other run (3.6s to 14.5s), including when run
directly after `static-kernel-interface-nexthop`. Same intermittent startup-under-
load pattern recorded for the other static tests; a stress-repro on a quiet host
is owed with the static gate.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire-visible behavior changes: no packet Ze sends or accepts differs. The external system this change speaks to is the Linux FIB, and the QEMU table above is the equivalent proof, per `ai/rules/interop-and-goal-validation.md`'s "config-only feature with no protocol impact" and `ai/rules/platform-linux.md` | |

## Files to Modify

- `internal/core/redistevents/registry.go` - the OS-installed declaration and
  its accessor, beside `RegisterProducer`
- `internal/core/rib/locrib/candidate.go` - `Path` gains a next-hop list
  carrying address, interface and weight; `Equal` compares it, `key()` does not
- `internal/core/rib/routeinstall/sink.go` - the entry gains route type and the
  next-hop list
- `internal/core/rib/distance/distance.go` - doc only: name connected and
  static as consumers
- `internal/component/plugin/server/dispatch_route.go` - rebuild the new Path
  fields, and RE-STAMP `AdminDistance` from the declaration with the wire value
  as fallback
- `internal/component/sysrib/sysrib.go` - resolve the OS-installed property at
  ingest, and make an OS-installed winner produce a withdraw of Ze's entry
  rather than an install
- `internal/component/sysrib/ecmp.go` - populate `Weight` and the interface
  from the producer instead of hardcoding 1
- `internal/component/sysrib/events/events.go` - `ECMPPath` gains an interface
- `internal/plugins/connected/connected.go` - insert and remove Loc-RIB paths
- `internal/plugins/connected/register.go` - forked sink wiring
- `internal/plugins/connected/events/events.go` - declare that the OS installs
  connected routes
- `internal/plugins/static/inject.go` - insert and remove Loc-RIB paths for
  main-table routes; keep the direct backend for named tables
- `internal/plugins/static/register.go` - forked sink wiring
- `internal/plugins/static/backend_linux.go` - the main-table write path is
  deleted, not kept as a fallback
- `internal/plugins/static/doctor.go` - narrow `static-route-skipped` to the
  failures it can still see
- `internal/plugins/fib/kernel/nexthop_linux.go` - resolve an interface
  next-hop to an ifindex in the rich route
- `pkg/plugin/rpc/types.go` - `RouteInstallEntry` gains route type and the
  next-hop list
- `docs/architecture/static-routes.md` - the abandoned-design section is
  rewritten, and the false `RTPROT_ZE` / no-collision paragraph is corrected
- `docs/architecture/rib/unified-locrib.md` - the Path model gains the
  next-hop list, and the page names connected and static as sources
- `docs/architecture/core-design.md` - one sentence: which protocols insert,
  and that a protocol whose FIB entry the OS creates declares it
- `docs/architecture/forked-route-install.md` - the entry's new fields and the
  engine-side re-stamp
- `docs/architecture/api/process-protocol.md` - `ECMPPath` gains an interface
- `docs/architecture/api/ipc_protocol.md` - the RPC type change
- `docs/architecture/rib/forward-handle.md` - name connected and static among
  the callers that pass a nil handle
- `plan/immediate/spec-fib-depth.md` - its `ECMPPath.Weight` item is satisfied here; a
  note records that, and `TableID` stays its own
- `docs/guide/static-routes.md` - the distance interaction an operator can now
  rely on
- `docs/features.md` - the leaf that now decides something
- `docs/config-reference.md` - regenerated if the generator's output moves

## Files to Create

- `internal/plugins/connected/locrib.go` - the connected insert/remove path and
  its sink wiring, kept out of `connected.go` so the observer stays one concern
- `test/static/static-distance-loses-to-ebgp.ci`
- `test/static/static-distance-beats-ebgp.ci`
- `test/static/static-named-table-unchanged.ci`
- `test/plugin/connected-distance-arbitration.ci`
- `test/plugin/connected-distance-raised-loses.ci`
- `test/static/static-kernel-distance-route-override.ci` (AC-21), driven by
  `staticKernelDistanceRouteOverride` in `internal/test/fixture/static_kernel_fixture.go`
- the retired deferral shard "connected-static-reach-the-locrib"

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Both leaves already exist in `internal/component/sysrib/yang/ze-rib-conf.yang`. This spec adds no config node: the defect is that two existing nodes decide nothing |
| YANG validation constraints | N-A | `connected` is uint8 0-255 and `static` is uint8 1-255 already; neither range changes |
| YANG custom validators | N-A | No cross-node constraint is introduced |
| CLI commands/flags | No | No new command. `show rib` output gains connected rows through the existing `(*sysRIB).showRIB` path |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | Automatic for the existing leaves |
| Functional test for new RPC/API | Yes | `test/plugin/connected-distance-arbitration.ci` and the four other `.ci` files above cover the operator-visible behavior |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No `environment/` leaf added |
| Doctor check for runtime dependencies | Yes | No NEW runtime dependency is added; the existing `static-route-skipped` check (`internal/plugins/static/doctor.go`, code `doctorCodeRouteSkipped`) CHANGES meaning and its text, unit test and `.ci` coverage change with it (AC-15) |
| Prometheus counters/metrics | Yes | `ze_sysrib_routes_best` already counts the system best set and will now include connected winners. No new series; the existing gauge's meaning is documented in `docs/architecture/core-design.md` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: the distance leaves now decide |
| 2 | Config syntax changed? | No | No syntax change; `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` describe the same grammar |
| 3 | CLI command added/changed? | No | No command added or changed |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` is unaffected, but `docs/architecture/api/ipc_protocol.md` and `docs/architecture/api/process-protocol.md` carry the two changed payloads |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: static no longer programs main-table routes itself |
| 6 | Has a user guide page? | Yes | `docs/guide/static-routes.md` |
| 7 | Wire format changed? | No | No BGP or IGP wire format changes; the netlink change is a route protocol stamp, documented on the static page |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` and `ai/rules/plugins.md` if the route-install contract is named there |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Administrative distance is not an RFC concept; `ze-rib-conf.yang` says so |
| 10 | Test infrastructure changed? | No | Existing `.ci` and QEMU harnesses are used unchanged |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: Ze arbitrates static and connected against dynamic protocols, which is what FRR and BIRD do |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/architecture/rib/unified-locrib.md`, `docs/architecture/static-routes.md`, `docs/architecture/forked-route-install.md`, `docs/architecture/rib/forward-handle.md` |
| 13 | Route metadata keys added/changed? | No | No metadata key added |
| 14 | Prometheus counters added/changed? | No | No new series; the meaning note lands in the architecture page instead |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` and `docs/features/plugins.md`: the OS-installed declaration is a new registered property |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-connected-static-reach-the-locrib.md` at the start of implementation. The design owners known at spec time are `docs/architecture/core-design.md`, `docs/architecture/rib/unified-locrib.md`, `docs/architecture/rib/forward-handle.md`, `docs/architecture/static-routes.md`, `docs/architecture/forked-route-install.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/ipc_protocol.md` and `plan/immediate/spec-fib-depth.md`, all named above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/static-routes.md` and `docs/config-reference.md` show the `rib { distance { } }` and `static { }` blocks; both must match the behavior after the change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry points reach the
   Loc-RIB before anything decides
   - Tests: `TestAddrAddedInsertsAConnectedPath`,
     `TestStaticApplyInsertsAPathPerRoute`,
     `TestForkedConnectedInsertReachesTheEngineRIB`
   - Files: `internal/plugins/connected/locrib.go`,
     `internal/plugins/connected/register.go`,
     `internal/plugins/static/inject.go`, `internal/plugins/static/register.go`
   - Verify: the wiring tests FAIL because both producers are stubs. Nothing is
     removed from the static backend yet
2. **Phase: Record today's arbitration (AC-11)** -- measure before changing
   - Tests: `TestQEMUStaticDistanceDecidesAgainstBGP` on the UNCHANGED tree
   - Files: `internal/plugins/fib/kernel/integration_linux_test.go`
   - Verify: the test is RED and the failure output is recorded in the spec.
     A test that has never been red proves nothing
     (`ai/rules/interop-and-goal-validation.md`)
3. **Phase: connected representation** -- the OS-installed declaration, the
   insert, and the withdraw decision
   - Tests: `TestOSInstalledIsDeclaredNotDerived`,
     `TestConnectedWinnerWithdrawsTheZeRoute`,
     `TestConnectedWinnerEmitsNothingWhenNothingWasProgrammed`,
     `TestConnectedLoserLeavesTheZeRouteProgrammed`,
     `TestNextHopResolvesThroughAConnectedPrefix`,
     `TestSRv6SIDResolvesThroughAConnectedPrefix`,
     `test/plugin/connected-distance-arbitration.ci`,
     `test/plugin/connected-distance-raised-loses.ci`,
     `TestQEMUConnectedBeatsBGPForTheSamePrefix`
   - Files: `internal/core/redistevents/registry.go`,
     `internal/plugins/connected/events/events.go`,
     `internal/plugins/connected/locrib.go`,
     `internal/component/sysrib/sysrib.go`, plus
     `docs/architecture/core-design.md` and
     `docs/architecture/rib/unified-locrib.md` in the same phase
   - Verify: AC-1 through AC-6 hold. The connected half stands on its own and
     could be committed here
4. **Phase: the forked distance hole** -- the engine re-stamp
   - Tests: `TestForkedProducerDistanceIsRestampedByTheEngine`,
     `TestRouteInstallEntryCarriesRouteTypeAndECMP`
   - Files: `internal/component/plugin/server/dispatch_route.go`,
     `internal/core/rib/routeinstall/sink.go`, `pkg/plugin/rpc/types.go`,
     `docs/architecture/forked-route-install.md`,
     `docs/architecture/api/ipc_protocol.md`
   - Verify: AC-17 holds, and the same fix closes the hole for forked OSPF and
     IS-IS with no change to either
5. **Phase: the Path next-hop list** -- weight and interface reach the FIB
   - Tests: `TestPathCarriesWeightedNextHops`,
     `TestEqualCostGroupKeepsWeightOneForProducersThatStateNone`,
     `TestECMPPathCarriesTheInterface`, `TestQEMUStaticWeightedMultipath`,
     `TestQEMUStaticInterfaceNextHop`
   - Files: `internal/core/rib/locrib/candidate.go`,
     `internal/component/sysrib/ecmp.go`,
     `internal/component/sysrib/events/events.go`,
     `internal/plugins/fib/kernel/nexthop_linux.go`,
     `docs/architecture/api/process-protocol.md`
   - Verify: AC-12 and AC-13 hold with no change to what BGP, OSPF or IS-IS
     program (R-7)
6. **Phase: static relocation** -- the insert lands and the second writer is
   DELETED in the same phase (R-4, `ai/rules/no-layering.md`)
   - Tests: `TestStaticStampsTheDeclaredDistance`,
     `TestNamedTableStaticRouteNeverReachesTheLocRIB`,
     `TestStaticRefusesAnUnresolvableNextHopBeforeInsert`,
     `TestStaticRollbackRestoresThePreviousPathSet`,
     `TestStaticBFDDownReinsertsTheSurvivingNextHop`,
     `TestStaticBlackholeCarriesTheRouteType`,
     `TestStartupSweepKeepsARefreshedStaticRoute`, the three static `.ci` files,
     `TestQEMUNamedTableStaticUnchanged`, and phase 2's QEMU test now GREEN
   - Files: `internal/plugins/static/inject.go`,
     `internal/plugins/static/backend_linux.go`,
     `internal/plugins/static/doctor.go`,
     `internal/plugins/fib/kernel/fibkernel.go`,
     `docs/architecture/static-routes.md`, `docs/guide/static-routes.md`
   - Verify: AC-7 through AC-10 and AC-14 through AC-18 hold, and the
     `RTPROT_STATIC` write for a main-table route no longer exists anywhere
7. **Phase: the remaining surfaces** -- docs, comparison, plugin inventory
   - Tests: `./le verify worktree`
   - Files: `docs/features.md`, `docs/comparison.md`,
     `docs/plugin-overview.md`, `docs/features/plugins.md`,
     `docs/config-reference.md`, `plan/immediate/spec-fib-depth.md`
   - Verify: `./le spec citation anchors` reports no unnamed design owner

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-11's RED output is pasted rather than described |
| Feature completeness | Both halves reach the kernel through the same single writer; no prefix has two entries |
| Correctness: one writer | `grep -rn "rtproto.Static" internal/plugins/static` finds only the named-table path, and no main-table route is written twice |
| Correctness: the withdraw | An OS-installed winner produces a withdraw of Ze's OWN entry and never silence, because silence leaves a stale `RTPROT_ZE` route beside the kernel's connected route (R-2) |
| Correctness: no accidental zero | `ribdistance.OrDefault("connected", 0)` passes 0 DELIBERATELY, because 0 is connected's classical distance. It must carry a comment saying so, since it is the exact shape `distance.go` warns against |
| Correctness: the guard is named | The OS-installed property is asked as a question with an answer, not inferred from a zero, a nil next-hop, or a protocol name comparison (`ai/rules/principles.md`) |
| Naming | JSON keys kebab-case on both changed payloads; the next-hop list type names what it holds, not its Go shape |
| Data flow | connected and static reach the FIB only through Loc-RIB -> sysrib -> fib-kernel for main-table routes; no plugin package names another plugin |
| Registration | `redistevents` gains a property a plugin DECLARES; sysrib reads it by ID and spells no plugin name |
| Rule: `ai/rules/no-layering.md` | The static main-table netlink write is deleted in the phase that adds the insert, and no commit in between leaves both live |
| Rule: `ai/rules/platform-linux.md` | Every netlink-visible claim has a QEMU test that reads the kernel, not a unit test over the builder |
| Rule: `ai/rules/documentation.md` | `docs/architecture/static-routes.md` is corrected in the phase that changes the behavior, including its false `RTPROT_ZE` paragraph |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| connected inserts a Loc-RIB path | `grep -rn "InsertForward" internal/plugins/connected` returns a non-test hit |
| static inserts a Loc-RIB path | `grep -rn "InsertForward" internal/plugins/static` returns a non-test hit |
| the second kernel writer is gone for main-table routes | `grep -rn "rtprotStatic" internal/plugins/static` shows only the named-table path |
| the OS-installed property is registered, not hardcoded | `grep -rn '"connected"' internal/component/sysrib internal/core/rib` returns nothing |
| both distance leaves decide | `go test ./internal/component/sysrib ./internal/plugins/static ./internal/plugins/connected -run Distance -count=1` |
| the operator can reach it | `./le verify current mode full` covering the five new `.ci` files |
| the kernel agrees | the five QEMU tests, run on Linux |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | An `addr-added` payload is external input from the iface layer: the prefix length is already checked by `toNetworkPrefix`, and a malformed payload must not reach `InsertForward` |
| Resource exhaustion | One Loc-RIB path per connected prefix and one per static route, both bounded by configuration rather than by a peer. The ECMP group stays bounded by `MaxECMPPaths` |
| Privilege boundary | The netlink write moves from the static plugin's process to fib-kernel's. A forked static plugin then needs no `CAP_NET_ADMIN` for main-table routes, which is a reduction; the named-table path still needs it and that must be stated in the doctor check's message |
| Fail-closed | An unknown protocol asked for the OS-installed property must report unknown, and sysrib must log once and program normally, never treat unknown as "do not program", which would silently blackhole every route from a protocol that forgot to register |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| QEMU test cannot run on the dev machine | It is not optional. Run it on the Linux target; a Linux-only claim with no QEMU evidence is unproven (`ai/rules/platform-linux.md`) |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The Loc-RIB was designed for connected routes and has never held one.
  `(*nhResolver).Resolve` terminates on a path with an invalid next-hop and
  calls that a connected route in its own comment. Connected's half fills a
  hole the resolver already had a shape for.
- `docs/architecture/static-routes.md` rejected Loc-RIB injection for reasons
  that have half expired. ECMP arrived; weight and interface did not. Reading
  the page's reason and checking each half against today's code is what turned
  an apparently large redesign into three named fields.
- The declared distance never reaching a forked producer is a defect in the
  distance seam that landed hours before this spec was written. It is in scope
  here because connected and static are forkable plugins, and the fix closes it
  for OSPF and IS-IS at the same line.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A main-table static route AUTO-LOADS the FIB writer instead of refusing a config with no `fib { }` block (owner decision, 2026-09-06) | (a) the `static-fib-writer` doctor check at error severity, which is what commit `125979ea99` shipped; (b) a hard `Dependencies: ["fib-kernel"]` on static, which is what OSPF and IS-IS already do | The owner's words are "the FIB block should be auto-loaded when we use static", so (a) is withdrawn: a config an operator could commit before the relocation must still start after it. (b) spells one plugin name in a producer, so a VPP deployment loads the kernel writer. The writer is DECLARED instead: `Registration.DataPlane` names the data plane a plugin programs, `Registration.NeedsDataPlane` says a producer's routes reach forwarding only through one, and the engine resolves the pair against the `interface { backend }` the operator already chose. No package holds a list of FIB plugins, and `fib-p4` declares no data plane, so a backend that programs nothing is never the answer |
| "The OS installs this protocol's routes" is a property the PROTOCOL declares at registration, read by sysrib through the ID | (a) a new `routetype.Type` value; (b) a boolean field on `locrib.Path`; (c) sysrib comparing the winner's protocol name to "connected" | (a) is wrong on the type's own terms: `routetype` values ARE the Linux RTN_ constants and describe the forwarding ACTION, and a connected route IS unicast. The question is ownership, not action. (b) touches `Path`, `Equal`, the RPC entry, `protocolRoute`, `BestChangeEntry` and every producer, to carry a value that is constant per protocol. (c) spells a plugin name in a component, which `ai/rules/plugins.md` forbids. The registry already carries exactly this shape in `RegisterProducer` |
| An OS-installed winner produces a WITHDRAW of Ze's own FIB entry, not silence | emit nothing and let the previous entry stand | Silence leaves a stale `RTPROT_ZE` route competing with the kernel's own connected route for the same prefix, which is a second writer by another name. The withdraw is the whole behavior: a connected route wins by REMOVING Ze's |
| Only MAIN-table static routes join the Loc-RIB; a named-table route keeps the direct path | (a) add a table dimension to the Loc-RIB key; (b) carry `TableID` on `Path` and let sysrib populate `BestChangeEntry.TableID` | The Loc-RIB is keyed by (family, prefix) with no table, so (a) is a storage-shape change for every protocol and every shard, to serve one producer. (b) is already owned by `plan/immediate/spec-fib-nexthop-objects-vpp-metric.md` (split out of `spec-fib-depth` on 2026-10-08), which depends on `spec-vrf-0-umbrella`. A named table has exactly one writer by construction and nothing to arbitrate, so distance decides nothing there: this is a boundary, not a scope cut. It is guarded by a test rather than left as a convention |
| The forked path re-stamps `AdminDistance` in the ENGINE, taking the wire value as the fallback | (a) publish the distance table to every forked plugin over RPC; (b) leave the forked producer stamping its bootstrap default | (a) is a second distribution channel for a value the engine already holds, and it has to be replayed on every reload to every plugin. (b) is the current defect: `rib { distance { ospf 5 } }` is inert for a forked OSPF at the Loc-RIB arbitration point. The engine is where the declaration lives and where the Path is rebuilt anyway |
| Static keeps its synchronous refusal of routes it cannot RESOLVE, and loses only the synchronous netlink error | keep a synchronous confirmation by making the insert acknowledge kernel programming | An acknowledgement path from fib-kernel back to static is a new cross-plugin round trip on the config apply, for a failure class fib-kernel already reports as `fib-sync-failure`. The split is by failure class, and the doctor check's text narrows to match what it can still see |
| `Path` gains one next-hop list carrying address, interface and weight | separate parallel slices, or a second field beside `ECMP` | Three facts about one next-hop belong in one value. The list follows the `Labels` contract: built once, shared, never mutated, excluded from `key()`, compared by `Equal` so the FIB observes a change |

## Known Limitations

- Interface-layer routes (DHCPv4, IPv6 RA, PPPoE, PPP NCPs, `rtproto.Iface` =
  253) still program the kernel directly and are not arbitrated. They are a
  THIRD inert consumer of the same idea, recorded in
  `plan/journal/guard-added-to-one-half-of-a-pair.md` (2026-08-10), which names
  a destination spec, `spec-admin-distance-reaches-the-kernel`, that does not
  exist on disk. This spec cannot be that destination: its Task and ACs cover
  the connected/static main-table paths, and `rib { distance { } }` declares
  no iface leaf. The owner decided on 2026-10-08 that the interface-layer
  remainder gets its own spec, `plan/immediate/spec-iface-route-arbitration.md`,
  and that it does not block this spec.
- `BestChangeEntry.TableID` stays unpopulated by sysrib. It belongs to
  `plan/immediate/spec-fib-nexthop-objects-vpp-metric.md` and the VRF umbrella.
- The BGP RIB's `IsEBGP` remains the way sysrib classifies eBGP from iBGP. This
  spec does not revisit it.

## RFC Documentation (Scope: plugin)

N-A. No RFC governs administrative distance, and `ze-rib-conf.yang` says so in
the container's own help text. No protocol behavior changes: no packet Ze sends
or accepts differs after this change.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-19 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- connected and static insert their main-table routes into the Loc-RIB (`internal/plugins/connected/locrib.go`, `internal/plugins/static/locrib.go`); fib-kernel is the one writer of main-table static routes (`proto 250`); named tables keep static's direct write (`proto 251`).
- The Loc-RIB owns administrative distance (owner decision 2026-10-08): `resolvedDistance` and `DistanceProtocol` (`internal/core/rib/locrib/distance.go`) rank every path at its protocol's declared distance; no producer stamps one. A static route's own `distance` leaf travels as `Path.DistanceOverride`.
- A reload re-ranks installed routes: every table sysrib takes on goes through `(*sysRIB).declareDistances` (`internal/component/sysrib/register.go`), which publishes it and calls `(*RIB).Reselect`.
- sysrib's Loc-RIB feed (`internal/component/sysrib/locrib_feed.go`) loses no change under a Reselect burst: an overflowed change records its prefix, and the worker re-reads it from the Loc-RIB once the queue is drained.

### Bugs Found/Fixed
- Round 1 BLOCKER (a distance-only reload did not re-rank installed routes): fixed by the RIB-owns-distance change; `TestReloadReranksStaticAgainstBGP`, `TestEachProtocolDistanceChangeReranks`, `test/static/static-kernel-distance-reload.ci`.
- Round 2 ISSUE 1 (stage-2 configure published a table without re-ranking): `TestDeclareDistancesReranksInstalledPaths`.
- Round 2 ISSUE 2 (sysrib dropped Loc-RIB changes on a full channel, which a Reselect burst reaches): `TestFeedOverflowIsReReadNotDropped`, `TestFeedOverflowWaitsForTheQueue`.
- Round 3 ISSUE 3 (an overflow recorded after the worker drained the queue waited for an unrelated change): `TestFeedOverflowWakesTheWorker`.

### Documentation Updates
- `docs/architecture/core-design.md` System RIB section: `declareDistances` as the one publish path, and the feed's overflow re-read, anchored on `internal/component/sysrib/locrib_feed.go -- locRIBFeed, takeOverflow, resyncOverflow`.
- Earlier commits: `8bc1a22350` (core-design, IS-IS/OSPF pages, `ze-rib-conf.yang`), `5ede86fd04` (unified-locrib, static-routes arch and guide, forked-route-install, process-protocol), `5828b35d10` (comparison, static-routes).

### Deviations from Plan
- The five QEMU tests the plan named were replaced by `.ci` tests that read the real kernel table under `unshare -rn` with `CAP_NET_ADMIN` (`static-kernel-*`, `connected-distance-*`). They drive the netlink path the appliance uses; no QEMU guest run was made.
- Producer-side distance stamping, and its engine re-stamp (A-5, R-8), was removed by the owner decision of 2026-10-08; the tests that pinned it were replaced (`plan/verification-debt/b1a5c0de.md`).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Distance was stamped by each producer when the Path was built | A distance-only reload then re-ranked nothing until the route itself changed | Review Gate round 1 | Owner decision: the Loc-RIB resolves distance when it ranks and re-runs selection on reload |
| approach | `Reselect` was wired to the apply and rollback publishes only | Stage-2 configure publishes too, and connected starts with no dependency on `rib` | Review Gate round 2 | Every publish goes through `declareDistances` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| connected and static reach the Loc-RIB, main table | Done | `internal/plugins/connected/locrib.go`, `internal/plugins/static/locrib.go` | |
| the RIB owns distance, re-ranks on reload, static per-route override | Done | `internal/core/rib/locrib/distance.go`, `internal/component/sysrib/register.go` `declareDistances` | owner decision 2026-10-08 |
| named tables stay out of the Loc-RIB | Done | `internal/plugins/static/locrib.go` `inMainTable` | TableID owner is spec-fib-nexthop-objects-vpp-metric |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAddrAddedInsertsAConnectedPath` | |
| AC-2 | Done | `TestAddrRemovedWithdrawsTheConnectedPath` | |
| AC-3 | Done | `TestOSInstalledLoserLeavesTheZeRouteProgrammed`, `test/plugin/connected-distance-raised-loses.ci` | |
| AC-4 | Done | `TestConnectedPrefixIsWhatTheResolverTerminatesOn`, `TestRecursiveNHResolve_DirectlyConnected` | |
| AC-5 | Done | `TestOSInstalledWinnerWithdrawsTheZeRoute` | |
| AC-6 | Done | `TestOSInstalledWinnerEmitsNothingWhenNothingWasProgrammed`, `test/plugin/connected-distance-arbitration.ci` | |
| AC-7 | Done | `TestStaticApplyInsertsAPathPerRoute`, `static-kernel-distance-static-wins.ci` (proto 250) | |
| AC-8 | Done | `static-distance-loses-to-ebgp.ci`, `static-kernel-distance-bgp-wins.ci` | |
| AC-9 | Done | `static-distance-beats-ebgp.ci`, `static-kernel-distance-static-wins.ci` | |
| AC-10 | Done | `TestNamedTableStaticRouteNeverReachesTheLocRIB`, `static-named-table-unchanged.ci` | |
| AC-11 | Done | red recorded in Goal Validation (pre-`125979ea99` producer) | kernel `.ci`, not QEMU |
| AC-12 | Done | `static-kernel-weighted-multipath.ci` | |
| AC-13 | Done | `static-kernel-interface-nexthop.ci`, `TestECMPPathCarriesTheInterface` | |
| AC-14 | Done | `TestStaticBlackholeCarriesTheRouteType` | |
| AC-15 | Done | `TestStaticRefusesAnUnresolvableNextHopBeforeInsert`, `static-per-route-isolation.ci` | |
| AC-16 | Done | `TestStaticRollbackRestoresThePreviousPathSet` | kernel half rests on fib-kernel being the single writer |
| AC-17 | Done | `TestForkedRouteRanksAtTheDeclaredDistance` | |
| AC-18 | Done | `TestStaticBFDDownReinsertsTheSurvivingNextHop` | |
| AC-19 | Done | `UndeclaredDistance` in `resolvedDistance`; `reselect_test.go` | |
| AC-20 | Done | `TestReloadReranksStaticAgainstBGP`, `TestDeclareDistancesReranksInstalledPaths`, `static-kernel-distance-reload.ci` | |
| AC-21 | Done | `TestStaticOwnDistanceOverridesTheDeclared`, `TestForkedRouteKeepsItsOwnOverride`, `static-kernel-distance-route-override.ci` | |
| AC-22 | Done | `TestEachProtocolDistanceChangeReranks` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| unit tests of the TDD table | Done | as named in the TDD table (reconciled names) | `go test -race` green over sysrib, core/rib, static, connected, routeinstall, fib/kernel, ospf/isis spf and the route-install tests of plugin/server, 2026-10-08 |
| `.ci` of the Functional table | Done | `test/static/`, `test/plugin/` | static suite 18/18 under `unshare -rn` |
| five QEMU tests | Changed | kernel-reading `.ci` | Deviations |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Create | Done | Pre-Commit Verification, Files Exist |

### Audit Summary
- **Total items:** 22 ACs, 3 requirements
- **Done:** 25
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (QEMU tests to kernel-reading `.ci`, recorded in Deviations)

## Goal Validation (BLOCKING)

Written 2026-10-08. Goals from the Task section; evidence named per row, and
what is missing is said in the row rather than left to a later pass.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `static` decides a route-install outcome: one number changes and the prefix changes hands | functional | `test/static/static-distance-beats-ebgp.ci` and `static-distance-loses-to-ebgp.ci` differ only in the declared distance and expect opposite winners in `show rib`; both green under `unshare -rn` on 2026-10-08. Kernel half: `test/static/static-kernel-distance-static-wins.ci` and `static-kernel-distance-bgp-wins.ci` read exactly one kernel entry, proto 250, via the winner's next-hop. AC-11 red: with main-table routes sent back through static's direct write (the pre-`125979ea99` producer), bgp-wins reads `10.0.0.0/8 via 192.0.2.1 dev zentk proto 251` while BGP holds the system RIB, and static-wins reads winner "bgp"; restored green |
| `connected` decides a route-install outcome: a connected prefix at distance 0 wins and Ze programs nothing the kernel already holds | functional + unit | `test/plugin/connected-distance-arbitration.ci` ("won by connected, ze-programmed=no", read from the kernel), `test/plugin/connected-distance-raised-loses.ci` (distance 250 hands it back to BGP); `TestOSInstalledWinnerWithdrawsTheZeRoute`, `TestOSInstalledWinnerEmitsNothingWhenNothingWasProgrammed`, `TestOSInstalledLoserLeavesTheZeRouteProgrammed` |
| One writer programs the kernel for main-table static routes | functional | `test/static/static-named-table-unchanged.ci` waits for the main-table static route as `proto 250` (fib-kernel's stamp, not static's 251) before its assertions; `test/static/static-per-route-isolation.ci` (`1e3328ef53`) shows a kernel-refused main-table route surfacing as fib-kernel's "add route failed", not as static's skip |
| Named tables keep the direct write and stay out of the Loc-RIB (main-table scope boundary) | functional, red proven | `test/static/static-named-table-unchanged.ci` (`5828b35d10`): table 171 holds the route as proto 251, main table and `show rib` do not. Red 1: `inMainTable()` forced true gives "never reached table 171". Red 2: named route inserted into the Loc-RIB gives "leaked into the main table ... proto 250" |
| Weighted multipath and interface-only next-hops survive the relocation (AC-12, AC-13) | functional, red proven + unit | `test/static/static-kernel-weighted-multipath.ci` (kernel hops `weight 3` and `weight 1`; red with weights stamped 1: both hops `weight 1`), `test/static/static-kernel-interface-nexthop.ci` (`dev zentk proto 250`, no gateway; red with the interface dropped: no kernel entry); unit `TestStaticPathCarriesWeightedNextHops`, `TestBestChangeCarriesTheDeclaredWeights`, `TestECMPPathCarriesTheInterface` |
| A failed transaction leaves the Loc-RIB on the previous set (AC-16, A-7) | unit, red proven | `TestStaticRollbackRestoresThePreviousPathSet` (`internal/plugins/static/register_test.go`) over a real `locrib.RIB`: after the journal's undo the Loc-RIB holds the previous set prefix by prefix. Red with the undo re-applying the new set. The kernel half of AC-16 is not read by a rollback-specific test: it rests on the FIB plugin being the single writer of Loc-RIB winners, which the kernel tests above prove |
| A distance-only reload re-ranks the installed static route and the kernel follows (AC-20, Round 1 BLOCKER) | unit, red proven + functional, red proven | `TestReloadReranksStaticAgainstBGP` (`internal/core/rib/locrib/reselect_test.go`): `static 5` to `static 250` hands the prefix to BGP by `Reselect` alone and dispatches the change to the FIB. Red with `Reselect` stubbed to `return`: the test fails, restored green. Kernel half: `test/static/static-kernel-distance-reload.ci` green under `unshare -rn` on 2026-10-08 (PASS 6.7s): after SIGHUP to `static 250` exactly one kernel entry, proto 250, via 198.51.100.1. Red run 2026-10-08 under `unshare -rn` with `Reselect` stubbed to return at entry: the fixture failed with `after the reload to static 250: want exactly one kernel entry, proto 250 via 198.51.100.1; ip route show table all: ["10.0.0.0/8 via 192.0.2.1 dev zentk proto 250 "]`; restored green (PASS 5.6s). The `.ci` was recreated after an unclean reboot lost the original, and its budget is 45s: a first red at the siblings' 30s ended on the harness timeout before the fixture's own message |
| A static route's own `distance` leaf overrides the declared static distance (AC-21) | unit, red proven + functional, red proven | `TestStaticOwnDistanceOverridesTheDeclared` (`reselect_test.go`): the route ranks at its own N whatever `static` declares, and a reload of `static` leaves it at N; red with `Reselect` stubbed. Producer half: `TestStaticPathCarriesTheRouteOwnDistance` (`internal/plugins/static/locrib_test.go`), the override reaches the Path with 0 kept as a value. End to end: `test/static/static-kernel-distance-route-override.ci` green under `unshare -rn` on 2026-10-08 (PASS 11.0s as a draft, 7.6s promoted): under `static 5`, 10.0.0.0/8 with its own `distance 3` and the witness 10.1.0.0/16 without one both forward via 192.0.2.1 against eBGP 20; after SIGHUP to `static 250` the witness moves to 198.51.100.1, proving the reload re-ranked, while 10.0.0.0/8 keeps exactly one kernel entry, proto 250, via 192.0.2.1, and `show rib` names static. Red run 2026-10-08 under `unshare -rn` with `resolvedDistance` (`internal/core/rib/locrib/distance.go`) made to skip `HasDistanceOverride`: `after the reload to static 250, the route with its own distance 3: 10.0.0.0/8: want exactly one kernel entry, proto 250 via 192.0.2.1; ip route show table all: ["10.0.0.0/8 via 198.51.100.1 dev zentk proto 250 "]`; pristine file restored, green again |
| A reload of `ebgp`, `ibgp`, `ospf` or `isis` re-ranks installed paths both ways (AC-22) | unit, red proven | `TestEachProtocolDistanceChangeReranks` (`reselect_test.go`), one subtest per protocol: raising it above static hands the prefix to static, lowering it hands it back, by `Reselect` alone. All four subtests red with `Reselect` stubbed (`scratch/reselect-red.log`), restored green |

Ready for independent closure: YES for the owed tests (2026-10-08). Gates still
owed by the main thread: `./le test static -a` under net-admin on a quiet host or
the QEMU guest (`./le test qemu`), and a stress-repro of
`static-kernel-weighted-multipath`, which timed out twice at load average 10-16.

Closure run 2026-10-08 (independent reviewer, under `unshare -rn`):
`./le test static -a` passed 16/16 in 73.7s at load average 3-9.
`static-kernel-weighted-multipath` passed in each of 6 completed runs, 1.8s to
8.9s, at load average 16 to 48 (one further attempt did not build: another
session's in-progress edit to `internal/component/plugin/server/server.go`). At
load 48 it finished in 8.9s against the 30s budget, so the fixture's startup
budget is not wrong; the two earlier 30s timeouts are attributed to host
contention at that moment, not to the test.

The owner decided Round 1's BLOCKER on 2026-10-08 (Decision at the top), and
the closure resumed. Closure run 2026-10-08, second independent reviewer:
`./le test static -a` under `unshare -rn` (loopback up) passed 18/18 in 79.1s,
over the tree carrying the Round 2 and 3 fixes.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Named-table and VRF routes through the Loc-RIB (TableID) | outside this spec's main-table scope (Task) | `plan/immediate/spec-fib-nexthop-objects-vpp-metric.md` |
| DHCP, RA and PPP route arbitration at the interface layer | owner decision 2026-10-08 | `plan/immediate/spec-iface-route-arbitration.md` |

## Review Gate

<!-- Filled at implementation time by /ze-review (BLOCKING before closure).
     Never delete this section. -->

### Round 1
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| `125979ea99`, `48b08b2532`, `74fdd86513`, `5828b35d10`, `1e3328ef53`, `02d9aca2ce`, emphasis on `applyRouteSet` undo and reload concurrency | logic, transaction/rollback, docs | 1 | 1 | 1 |

| # | Severity | Finding | Location | Status |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | A commit that changes only `rib { distance { static N } }` (or `connected`) does not re-rank a route already installed. The distance is read only when a Path is built (`staticPath`, connected `insertPath`); static is not delivered the `rib` root (`ConfigRoots` static, `ConfigReads` bfd, `internal/plugins/static/register.go`), and even a delivered static section skips an unchanged route (`routesEqual` in `(*routeManager).applyRoutes`, `inject.go`). The Original defect this spec exists to remove is "write `static 250`, reload cleanly, and the kernel still prefers the static route": that remains true until the route itself changes or the daemon restarts. BGP, OSPF and IS-IS share the gap through the same seam (`internal/core/rib/distance`), so the fix is a design choice: producers re-stamp on a distance change, or `locrib` ranks on the declaration and re-runs selection when it changes | `internal/plugins/static/locrib.go` `staticPath`; `internal/plugins/connected/locrib.go`; `internal/core/rib/distance` | FIXED, re-reviewed in Round 2 (two ISSUEs in the trigger and the delivery, below): the Loc-RIB resolves every path's distance (`resolvedDistance`, `internal/core/rib/locrib/distance.go`) and sysrib calls `(*RIB).Reselect` after each publish (`reselectLocRIB`, `internal/component/sysrib/register.go`); no producer stamps a distance; AC-20 to AC-22 |
| 2 | ISSUE | `applyRouteSet` comment said "A failed apply is undone before returning", and called `j.Rollback()` on apply error. `sdk.Journal.Record` stores the undo only after apply succeeds, so that Rollback ran nothing; and `applyRoutes` returns nil by construction (per-route isolation into `rm.skipped`) | `internal/plugins/static/register.go` `applyRouteSet` | FIXED: dead call removed, comment states the real contract. No behavior change, so no regression test is possible; `go test -race ./internal/plugins/static/` green |
| 3 | NOTE | Rollback journals are never discarded on commit for section-apply plugins, and `config-rollback` fans out to every participant, so a transaction that fails before static's apply replays the undo of the last committed one. Cross-plugin protocol gap, not specific to this spec | `(*configTxBridge).subscribeRollback`; static and fib-kernel `OnConfigRollback` | journal row in `plan/journal/rollback-forgets-partial-apply.md` |

Answers to the closure brief: a failed apply cannot leave the Loc-RIB and kernel
disagreeing through static, because a refused route is never inserted (and a
refused replacement withdraws the old Path, `applyRouteLocked`), and the FIB
plugin programs only Loc-RIB winners. A netlink refusal after insert is the
accepted R-3 split (`fib-sync-failure`). Concurrency with reload: `mu` guards
`currentRoutes`, `rm.mu` guards the route map, and the SDK delivers apply and
rollback serially; the defect is finding 3, not a race.

### Round 2
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| `5ede86fd04`, `1ad4b3af7c`, `9cbf577e83`, `3a4449847e`, `8bc1a22350`, `ade3d8c460` (RIB owns distance) | Reselect correctness and locking, producer stamping (grep), override scope, reload trigger paths, go-style, docs vs code | 0 | 2 | 2 |

Whole-tree pre-checks (`./le repo check`, `./le commit audit`) were not run by
this reviewer: the brief excluded whole-tree gates, and other sessions' work
fills the tree. Style pass over every changed Go file: no peer-reachable
`panic`, no discarded error, locking contracts stated; no finding.

Producer stamping: `grep -rn 'AdminDistance\s*[:=]\|AdminDistance:'` over
non-test Go outside `internal/core/rib/locrib` returns nothing, and no non-test
caller of `distance.Of`, `OrDefault` or `DefaultAdminDistance` remains.
Reselect: every shard is held for its whole pass, as an insert holds it, and
subscribers run under it as they do for an insert; a prefix with no valid best
is skipped because ranking cannot change validity.

### Round 3 (re-review of the Round 2 fixes)
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| `internal/component/sysrib/locrib_feed.go`, `register.go` `declareDistances` | concurrency interleavings of offer, drain and resync | 0 | 1 | 0 |

### Round 4
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| the final feed and `declareDistances` | ordering (no queued change older than a resync read), liveness (every overflow is woken or drained), shutdown | 0 | 0 | 0 |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| R1-1 | BLOCKER | A distance-only reload did not re-rank installed routes | `staticPath`, connected `insertPath`, the seam | `5ede86fd04` and the tests in Goal Validation |
| R1-2 | ISSUE | `applyRouteSet` ran a dead `j.Rollback()` under a false comment | `internal/plugins/static/register.go` | `aba5035c85` |
| R2-1 | ISSUE | Stage-2 `OnConfigure` published the declared table without `Reselect`. connected registers no dependency on `rib`, and a forked producer starts on its own, so a path inserted before sysrib's stage 2 stayed ranked at the schema default until it was re-sent. Root cause: `publishDistances` had four callers, and the re-rank sat beside two of them | `internal/component/sysrib/register.go` OnConfigure | `declareDistances`, the one path for the seed, configure, apply and rollback. `TestDeclareDistancesReranksInstalledPaths`, red with the `reselectLocRIB` call removed |
| R2-2 | ISSUE | sysrib's Loc-RIB OnChange handler dropped a change when its 4096-slot channel was full. `Reselect` dispatches one change per re-ranked prefix from a tight loop while it holds the shard lock that the worker's next-hop resolution waits on, so a reload of `ebgp` over a table larger than the channel dropped winners the kernel then never followed. [workaround] a larger channel moves the threshold. [source] a dropped change records its prefix, and the worker re-reads it from the Loc-RIB | `(*sysRIB).run` OnChange handler | `locRIBFeed` (`locrib_feed.go`). `TestFeedOverflowIsReReadNotDropped` and `TestFeedOverflowWaitsForTheQueue`, red with the overflow record removed |
| R3-1 | ISSUE | An overflow recorded after the worker drained the queue was not resynced until an unrelated later change arrived | `locRIBFeed.offer` | the one-slot `wake` channel. `TestFeedOverflowWakesTheWorker`, red with the wake send removed |

### NOTEs
- The route-install `distance` field is accepted from any forked protocol, not only static. Only static produces it today (grep above), and the docs say so (`docs/architecture/forked-route-install.md`, `process-protocol.md`). Refusing it for other protocols would name `static` in the engine.
- `go test -race ./internal/component/plugin/server/` is red in event-monitor, RPC-registration and RFC 8907 command tests that this change does not touch (another session's in-flight RPC naming and BGP persist work); the route-install tests there (`-run 'Route|Forked|Distance|Restamp'`) are green.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/connected-static-reach-the-locrib-450bc92b-6ac1-4190-bd40-b427ecba17bf.md` |
| `./le spec review check` | clean |
| Rounds | 4 |
| Reviewer lenses used | logic+locking, wiring of every reload path, scope of the override, docs vs code, go-style |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/connected/locrib.go` | yes | `ls` 2026-10-08 |
| `test/static/static-distance-loses-to-ebgp.ci`, `static-distance-beats-ebgp.ci`, `static-named-table-unchanged.ci`, `static-kernel-distance-reload.ci`, `static-kernel-distance-route-override.ci` | yes | `ls` 2026-10-08 |
| `test/plugin/connected-distance-arbitration.ci`, `connected-distance-raised-loses.ci` | yes | `ls` 2026-10-08 |
| `internal/component/sysrib/locrib_feed.go` | yes | written in Round 2 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-19 | unit and `.ci` named in the Audit | `go test -race -count=1` green over sysrib, core/rib/..., static, connected, routeinstall, fib/kernel, ospf/spf, isis/spf, 2026-10-08; `./le test static -a` 18/18 |
| AC-20 | reload re-ranks | `TestReloadReranksStaticAgainstBGP`, `TestDeclareDistancesReranksInstalledPaths` green; `static-kernel-distance-reload` PASS in the static suite |
| AC-21 | own distance wins | `static-kernel-distance-route-override` PASS in the static suite |
| AC-22 | each protocol re-ranks | `TestEachProtocolDistanceChangeReranks` green |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `rib { distance { static N } }` reload (SIGHUP) | `test/static/static-kernel-distance-reload.ci` | read: drives SIGHUP and reads `ip route show table all` |
| static `distance` leaf | `test/static/static-kernel-distance-route-override.ci` | read: asserts one kernel entry via the static next-hop across the reload |
| `static { route }` with no table | `test/static/static-kernel-distance-static-wins.ci` | PASS, proto 250 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestConnectedPathCarriesTheRegisteredSource`, `TestStaticPathCarriesTheRegisteredSource` |
| A-2 | confirmed | `TestConnectedPrefixIsWhatTheResolverTerminatesOn`, `TestRecursiveNHResolve_DirectlyConnected` |
| A-3 | confirmed | `TestSysRIBEmitsNoTableID` |
| A-4 | confirmed | AC-11 red recorded in Goal Validation |
| A-5 | broken, superseded | the owner decision moved distance into the Loc-RIB, so no restamp exists; `TestForkedRouteRanksAtTheDeclaredDistance` |
| A-6 | confirmed | `TestStaticRefusesAnUnresolvableNextHopBeforeInsert`, `static-per-route-isolation.ci` |
| A-7 | confirmed | `TestStaticRollbackRestoresThePreviousPathSet` |
| A-8 | confirmed | `TestRouteInstallEntryCarriesRouteTypeAndECMP`, forked OSPF/IS-IS spf tests green |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| core-design: every table goes through `declareDistances` | `register.go`: the seed, OnConfigure, apply and rollback each call `s.declareDistances` | yes |
| core-design: the feed re-reads overflowed prefixes | `locrib_feed.go` `offer`, `takeOverflow`, `resyncOverflow` | yes |
| forked-route-install, process-protocol: the `distance` field is a route's own distance | `dispatch_route.go` `applyRouteInstall` sets `HasDistanceOverride` only from `e.DistanceOverride` | yes |
