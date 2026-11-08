# Spec: home-cpe-1-logical-interface-name

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P1 |
| Release reason | Accepted configuration does the wrong thing: `pppoe-client pppoe0` validates, yet a static route, a firewall match or a masquerade rule that names `pppoe0` never matches the session device, which the kernel calls `pppN`. The guide's own example (`docs/guide/static-routes.md`, `interface pppoe0`) does not work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (RFC 5737 prefixes, `example.net`).

## Task

The `pppoe-client` list key (for example `pppoe0`) is a config-level name only.
The PPPoE dialer creates the PPP unit with PPPIOCNEWUNIT, so the kernel names
the device `ppp<unit>`, and the unit number can change on every reconnect. Every
consumer that names an interface (static routes, the FIB plugin, firewall and
NAT matches, DHCP server, TFTP, traffic shaping) either resolves the configured
name through `iface.Resolve`, which knows no mapping for a pppoe-client, or
takes it literally. None can name the session device by its configured name.

Goal: the configured name of a pppoe-client is the name of its kernel device,
on every session and every reconnect, so every consumer that names it reaches
the session; a static route through it survives the session's absence at boot
and across reconnects, in the main table and in a named table, alone or as one
member of a mixed ECMP group; and a commit that adds the client and its route
together is accepted.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - the shared resolver
  → Decision: "`os-name` applies to matched kinds only ... Created kinds such as veth, bridge and tunnel are made by ze under the logical name, so aliasing them would break creation." A pppoe-client is a created kind
  → Constraint: no Ze code resolves a configured name straight against the kernel (`./le arch iface-resolution`, `internal/le/arch/ifaceresolution/`)
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` - unit ownership: the unit stays open until `Cleanup` so `pppN` is not reused before the caller finishes
  → Constraint: a device named after the config key must be gone before the next session creates the same name
- [ ] `docs/architecture/static-routes.md` "An interface next hop is resolved TWICE", "One bad route does not drop the section", "ECMP and BFD", "Table selection and interface-only next hops"
  → Constraint: a main-table route is a Loc-RIB path that sysrib selects and the FIB plugin writes at `RTPROT_ZE` (250); a named-table route is written by static's own backend at `RTPROT_STATIC` (251). Each table has exactly one programming owner
  → Constraint: static resolves a route's table against the CANDIDATE routing-table section of the same delivery (`candidateTables`): "Every participant verifies before any applies, so a table added in the same commit as a route in it is not in the registry yet when static verifies the route". The declared interface set follows the same rule
  → Decision: "BFD changes the membership of the ECMP group, not the presence of the route": device presence joins BFD as a second input to a next-hop's activity
  → Constraint (page defect): the page says static refuses an unresolvable name "while the transaction can still fail". The producer does not: `staticPath` runs at apply, and per-route isolation skips the route and commits (see Current Behavior). This spec corrects the page
- [ ] `docs/architecture/config/transaction-protocol.md` section 2, "Phase 1: Verify", "Phase 2: Apply", "Phase 3: Rollback (conditional)"
  → Constraint: verify is non-destructive and sees only the delivered candidate sections; `WantsConfig` delivers another owner's root; "the engine does not track inter-plugin dependencies during apply", so static's apply MUST NOT depend on iface having applied first in the same transaction
- [ ] `docs/guide/firewall.md` "Interface Wildcard" (documents `internal/plugins/firewall/nft/lower_linux.go`)
- [ ] `docs/guide/static-routes.md` "Interface-only next-hops" and "Mixed ECMP" - `next { interface pppoe0 { } }` presented as working

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/pppoe_client.go` - `PPPoEClient.runSession` names the device `textbuf.StrInt("ppp", sess.UnitNum)` for `SetMTU`, `SetAdminUp`, `AddAddressP2P`, `AddRoute`; `PPPoEClient.status` reports the same `pppN` from `c.pppUnit`
- [ ] `internal/component/l2tp/pppoeclient/dialer.go` `Dial` - `ppp.DevPPPSetup(pppoxFD)` after PADS; the name is never passed down
- [ ] `internal/component/l2tp/pppoeclient/dialer.go` `Dial`, `internal/component/l2tp/pppoe/resolve.go` `resolveInterface`, `internal/component/l2tp/pppoe/kernel_linux.go` `pppoeCreate` - before 2026-11-07, `Dial` resolved `cfg.SourceInterface` through `iface.Resolve` for the discovery ifindex and MAC, then passed the CONFIGURED name to `pppoe.PPPoECreate`, whose `pppoeCreate` connects `AF_PPPOX` with `SockaddrPPPoE{Dev: ifname}`; `resolveInterface` dropped `Binding.OsName`. The access concentrator did the same (`InterfaceServer.createTransport` passed `ifName`, the configured `pppoe interface` name). With an `os-name` or `mac match` selector the session failed after PADS or bound another NIC. Fixed: `pppoe.InterfaceBinding` carries the resolved device, `PPPoECreate` takes the binding, and the AC keeps `device` beside `ifName`
- [ ] `internal/component/l2tp/ppp/devppp_linux.go` `DevPPPSetup` - PPPIOCGCHAN, opens /dev/ppp for the channel, PPPIOCATTCHAN, opens /dev/ppp for the unit, PPPIOCNEWUNIT with -1
- [ ] Linux `drivers/net/ppp/ppp_generic.c` (read from a local copy): `ppp_unit_register` names the device `ppp%i` only when `ifname_is_set` is false; `ppp_nl_newlink` (rtnetlink kind `ppp`, attribute `IFLA_PPP_DEV_FD` on an opened /dev/ppp file with no unit yet) sets `ifname_is_set` true and keeps the name the request carries; it returns EBUSY when `ppp_mutex` is contended (trylock to avoid lock inversion). `IFLA_PPP_DEV_FD` is in `vendor/golang.org/x/sys/unix/ztypes_linux.go`
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` `list pppoe-client` - description "The name is the list key and it never reaches the kernel"; `leaf name` is `type string` with no length or pattern
- [ ] `internal/component/iface/config.go` `parsePPPoEClientEntry` - validates `source-interface` with `ValidateIfaceName`, not the key; `ifaceConfig.osNameMap` and `ifaceConfig.permMACMap` cover ethernet only
- [ ] `internal/component/iface/resolve.go` - `resolver.osDeviceFor` maps a name through `osNames` (ethernet overrides) or uses it as is; `setResolverConfig` publishes `osNameMap` and `permMACMap`; cache invalidated by link events. `Subscribe` delivers `LinkAppeared`, `LinkUp` and `LinkDown` (RTM_DELLINK arrives as down, `iface.go` `LinkEventKind`) for a name that does not exist yet, keeping the newest event when a subscriber lags
- [ ] `internal/component/iface/config_apply.go` `applyAndPublish` - publishes the candidate's resolver mapping, then applies it; every apply path, the rollback re-apply included, goes through it, so the mapping always matches the config last applied. `applyConfig` creates dummy, veth, tunnel (`e.Spec`), wireguard, xfrm (`e.Spec`) and bridge devices under the configured name: those kinds do not have this defect
- [ ] `internal/plugins/static/register.go` - `Registration.WantsConfig` lists `static` and `interface`, so every transaction that reaches static delivers the candidate `interface` section beside the static one, but `verifyCandidateStatic` reads only the `static`, `routing-table` (`candidateTables`) and `bfd` (`checkBFDProfiles`) sections. Verify never resolves an interface. `staticTx.apply` and `staticTx.rollback` run `applyRouteSet`, whose journal undo re-applies the previous route set
- [ ] `internal/plugins/static/locrib.go` `staticPath` and `checkNextHopResolvable` - a main-table route is resolved at APPLY (`routeManager.insertPathLocked`), not at verify. `routeManager.applyRoutes` (`inject.go`) isolates a route whose next-hop does not resolve: it is logged, recorded in `routeManager.skipped` with its reason, shown by `static show` and `doctor-static-route-skipped`, and the commit SUCCEEDS. The skipped route is retried only on the next apply that reaches static. So a static route through a PPPoE device that is not up (boot, reconnect) is accepted and then silently waits for an unrelated reload; the comment above `checkNextHopResolvable` ("fails the transaction") is stale
- [ ] `internal/plugins/static/inject.go` `routeManager.programRouteLocked`, `activeNextHops`, `routeManager.watchBFD` - each next-hop carries an `active` flag that BFD drives; the route is reprogrammed with the active members, and withdrawn when none is active. A named-table route goes through `routeBackend.applyRoute` and `routeBackend.removeRoute`, a main-table route through the Loc-RIB (`applyProgrammed`, `withdrawProgrammed`)
- [ ] `internal/plugins/static/backend_linux.go` `resolveNexthopIndex` and `internal/plugins/fib/kernel/nexthop_linux.go` `buildRichRoute`, `buildNexthopInfo` - resolve through `iface.ResolveIndex` when programming; neither the static plugin nor fib-kernel subscribes to link events (grep for `iface.Subscribe` in both is empty), so a route the kernel removed with its device is not reinstalled when the device returns
- [ ] `internal/plugins/static/backend_linux.go` `netlinkStaticBackend.removeRoute` and `internal/plugins/fib/kernel/backend_linux.go` `netlinkBackend.delRoute`, `nexthop_linux.go` `netlinkBackend.delRichRoute` - return the `RouteDel` error as is; none treats ESRCH (route already gone, as after the kernel flushed it with its device) as done, unlike `mplscontext_linux.go`, which does
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerIfaceMatch`, used by `lowerMatch` for `MatchInputInterface` and `MatchOutputInterface` - emits a Meta IIFNAME or OIFNAME compare of the configured name, literal or wildcard; no resolver call anywhere in `internal/component/firewall` or `internal/plugins/firewall`
- [ ] BIRD `proto/static/static.c` (`/home/thomas/Code/github.com/CZ-NIC/bird`), `static_decide`: a next-hop is inactive when its neighbor has no interface (`r->neigh->scope < 0`), when `check link` is set and the link is down, or when its BFD session is not up; `static_announce_rte` announces only active next-hops and withdraws the route when none is left; `static_neigh_notify` re-decides on interface change. The neighbor is created `NEF_STICKY`, so a configured route whose interface is absent is kept and waits

**Behavior to preserve:**
- Every other interface kind's naming
- PPP unit ownership rules in `Dial` and `Cleanup`
- A static next-hop naming an interface that no candidate configuration declares as dynamic and that does not resolve is skipped at apply with its skip-reason, shown by `static show` and the doctor check, and the commit succeeds (per-route isolation)
- One programming owner per table: the FIB plugin for the main table, static's backend for a named table

**Behavior to change:**
- The pppoe-client device carries the configured name
- A static next-hop that names a pppoe-client declared in the same candidate is accepted while the device is absent, and is programmed whenever the device is up, with no reload needed
- Device presence of a declared dynamic interface is a next-hop activity input beside BFD, for main-table and named-table routes alike

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Shared logical-name resolver | `internal/component/iface/resolve.go` `Resolve`, `ResolveIndex`, `osDeviceFor`, `setMapping`; `docs/architecture/iface/logical-name-resolution.md` | maps a configured name to its kernel device through `os-name` and `mac match` overrides, or passes it through | does not apply to the mapping: the device carries the configured name, so no mapping is added. Its `Subscribe` is used (row below) |
| Candidate-section reading in static verify | `internal/plugins/static/register.go` `verifyCandidateStatic`, `candidateTables`, `checkBFDProfiles`; `Registration.WantsConfig` (`static`, `interface`) | static resolves a route's table and BFD profile against the sections of the same delivery, never against installed state, so a table and its route commit together | extends: the delivered `interface` section yields the candidate set of declared dynamic interface names, read the same way and at the same point |
| Declared-name parsing owned by iface | `internal/component/iface/config.go` `parsePPPoEClientEntry` and the section parser it belongs to | parses the `interface` root for the iface component | extends: a pure function in iface answers, from one `interface` section's data, the names of entries whose device iface creates on demand and may be absent (today the pppoe-client kind). Static calls it on its delivered section; static never names a kind |
| Static transaction journal | `internal/plugins/static/register.go` `pendingSection`, `staticTx`, `applyRouteSet` | holds the verified routes from verify to apply, and the undo that re-applies the previous route set on rollback | extends: the pending delivery and the undo carry the declared dynamic set with the routes, so apply and rollback use the set verified with them |
| Next-hop activity and reprogramming | `internal/plugins/static/inject.go` `nhState.active`, `programRouteLocked`, `activeNextHops`, `watchBFD` | BFD toggles a next-hop's activity; the route is reprogrammed with the active members, withdrawn when none is active | extends: device presence of a declared dynamic name is the second activity input; a next-hop is active only when BFD allows it AND its device is up |
| Created kinds named after the config key | `internal/component/iface/config_apply.go` (dummy, veth, tunnel, wireguard, xfrm, bridge creation) | creates the device under its configured name, so it needs no resolver mapping | extends: the pppoe-client joins the created kinds |
| Shared /dev/ppp setup | `internal/component/l2tp/ppp/devppp_linux.go` `DevPPPSetup` | PPPIOCNEWUNIT with -1, kernel names the unit `pppN` | replaces for the client only (named rtnetlink creation); the LNS and AC keep the unnamed path |
| PPPoE interface binding | `internal/component/l2tp/pppoe/resolve.go` `resolveInterface` over `iface.Resolve`; used by `pppoeclient.Dialer.Dial` and the AC's `newInterfaceServer` | resolved the logical name to ifindex, MAC and MTU and dropped the kernel device name, so `connect(AF_PPPOX)` took the configured name | extends: `InterfaceBinding` adds `Device` (`Binding.OsName`); one binding feeds discovery, PADT and the session socket, and `PPPoECreate` takes the binding so no configured name can reach it |
| Interface-name validation | `internal/component/iface/validate.go` `ValidateIfaceName` | validates kernel interface names, used for `source-interface` today | extends: applied to the pppoe-client key |
| Static next-hop resolvability check | `internal/plugins/static/locrib.go` `checkNextHopResolvable`, called from `staticPath` at apply | a route whose interface does not resolve is skipped at apply (main table); a named-table route is skipped on the backend's error | extends: a next-hop naming a declared dynamic interface is never a skip; it is an inactive member until its device is up |
| Link event subscription | `internal/component/iface/resolve.go` `Subscribe`, `onLinkEvent` | delivers per-name link events and invalidates the cache | extends: static subscribes once per declared dynamic name its committed routes use, and cancels when no committed route names it |
| Route removal by each owner | `internal/plugins/static/backend_linux.go` `netlinkStaticBackend.removeRoute`; `internal/plugins/fib/kernel/backend_linux.go` `netlinkBackend.delRoute`, `nexthop_linux.go` `netlinkBackend.delRichRoute`; precedent `mplscontext_linux.go` (ESRCH tolerated) | the error of `RouteDel` returns as is | extends: ESRCH on removal of a route through a device the kernel already deleted is "already done" for both owners, as the MPLS context removal does |
| fib-kernel external change watch | `internal/plugins/fib/kernel/monitor.go` `handleExternalChange` | reasserts routes Ze owns that another writer changed | does not apply: it reacts to route events, not to a device appearing; the Loc-RIB re-insert by static drives the main-table reinstall |
| nft interface match | `internal/plugins/firewall/nft/lower_linux.go` `lowerIfaceMatch` | emits a literal or wildcard IIFNAME/OIFNAME compare of the configured name | does not apply: unchanged, it becomes correct once the device carries the configured name |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `interface pppoe-client pppoe0 { ... }` plus `static ... next { interface pppoe0 }` and firewall `output-interface pppoe0`

### Transformation Path
1. Parse validates the key as a kernel interface name (`ValidateIfaceName`) and refuses a key equal to any other configured interface name
2. `reconcilePPPoEClients` passes `Name` in `PPPoEClientConfig` (already there)
3. `Dial` creates the unit through rtnetlink with that name and the unit descriptor, reads the unit number back for PPPIOCCONNECT; retries EBUSY, and EEXIST while the previous session's device is still being removed, bounded
4. `runSession` and `status` use the configured name for every backend call and the show output
5. Static verify (`verifyCandidateStatic`) reads the delivered candidate `interface` section beside `routing-table` and `bfd`, and asks iface's pure parser for the declared dynamic names in it. The set is the candidate's, never the resolver's or the running config's: the transaction that adds `pppoe-client pppoe0` and `route 0.0.0.0/0 next interface pppoe0` together verifies the route against a set that already holds `pppoe0`. A delivery with no `interface` section declares no dynamic name. Verify adds no refusal: an undeclared name keeps today's apply-time skip
6. The verified routes and their declared set travel together in `pendingSection` to `staticTx.apply`. Apply classifies each interface next-hop from that set and never from iface's published state, so the outcome does not depend on whether iface applied before static in the same transaction
7. Classification at apply, per next-hop: a name in the declared set is a waiting member, active only while its device is up (first state read through `iface.Resolve`, then kept by link events); a name outside the set follows today's path (resolves, or the route is skipped with its reason)
8. Static subscribes (`iface.Subscribe`) once per declared dynamic name that a committed route uses. `LinkUp` marks the member's device up; `LinkDown` (down or deleted) marks it absent. Each change runs `programRouteLocked`, which already combines BFD activity: the route is reprogrammed with the active members and withdrawn when none is active
9. Programming owner per table, unchanged: a main-table route is re-inserted into the Loc-RIB with its active members (or removed), sysrib selects, and the FIB plugin writes it at `RTPROT_ZE`, resolving the device again at write time, so a new ifindex after a reconnect is picked up. A named-table route is written by static's backend at `RTPROT_STATIC`. When the kernel already flushed the route with its device, the owner's removal answers ESRCH, which both owners treat as done
10. Deletion: a candidate that removes the pppoe-client but keeps a route through it no longer declares the name; at apply the route takes the undeclared path, exactly as any route through an undeclared device does today, and its subscription is cancelled. A candidate that removes both withdraws the route. The subscription of a name no committed route uses is cancelled at apply
11. Rollback: the static journal's undo re-applies the previous route set together with the previous declared set, and re-derives the subscriptions from them; iface's rollback re-applies its previous config through `applyAndPublish`. A transaction that added the client and its route and then failed elsewhere leaves neither a waiting route nor a subscription
12. nft rules keep their literal name; a string match on `pppoe0` matches every session of that client and nothing else

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | RTM_NEWLINK kind `ppp` with `IFLA_PPP_DEV_FD` and `IFLA_IFNAME` | No |
| Config transaction to static | the candidate `interface` section, delivered by `WantsConfig`, parsed by an iface pure function in static's own process | No |
| iface to static | link events through `iface.Subscribe`; process-local, so it holds while static runs internal (A-6) | No |
| static to FIB plugin (main table) | Loc-RIB path insert and remove; the FIB plugin resolves the device through `iface.ResolveIndex` | No |

### Integration Points
- `iface.Resolve`, `iface.ResolveIndex`, `iface.Subscribe`
- `internal/plugins/static/register.go` `verifyCandidateStatic`, `pendingSection`, `staticTx`, `applyRouteSet`
- `internal/plugins/static/locrib.go` `staticPath`, `checkNextHopResolvable`; `inject.go` `programRouteLocked`
- `internal/plugins/fib/kernel` removal path (ESRCH)
- Child 4 (`spec-home-cpe-4-ike-dynamic-wan.md`) binds IKE to the WAN by this name

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | consumers keep resolving through `iface.Resolve` |
| No unintended coupling | Yes | static learns "declared, may be absent" from an iface function over the delivered section, never by reading the pppoe-client list itself; static already imports `iface` for `ResolveIndex` |
| No duplicated functionality | Yes | no second mapping table; the name is the device; activity reuses the BFD membership path; candidate reading reuses the `candidateTables` pattern |
| Zero-copy preserved where applicable | N-A | |
| Registration over hardcoding, outbound | N-A | |
| Registration over hardcoding, inbound | Yes | which kinds are dynamic is answered by iface, the owner of the kinds; a later dynamic kind joins by iface's own parser with no static edit. Searched `osNameMap`, `permMACMap`, `setResolverConfig`, `checkNextHopResolvable`, `candidateTables`, `lowerIfaceMatch` |
| Works when the plugin runs external | Partly, as today | `iface.Subscribe` and `iface.ResolveIndex` are process-local; `warnIfExternal` already states interface next-hops need static internal. Candidate parsing is a pure function and works in either process |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's kernel accepts RTM_NEWLINK kind `ppp` from Ze's user (CAP_NET_ADMIN) and the created unit behaves as a PPPIOCNEWUNIT one for PPPIOCCONNECT | `ppp_nl_newlink` in `ppp_generic.c` | fall back is not allowed (no layering); design returns to the owner | `TestDevPPPSetupNamesUnit` (Linux, user namespace) | unvalidated |
| A-2 | The netlink library in vendor can send a link of kind `ppp` with `IFLA_PPP_DEV_FD`, or a raw RTM_NEWLINK is encoded by Ze | vendor holds the constant only | small encoder in `internal/component/l2tp/ppp` | Phase 1 read of the vendored link types | unvalidated |
| A-3 | The iface monitor reports `LinkUp` and `LinkDown` for a ppp device, and `LinkDown` on its deletion | `iface.go` `LinkDown` comment ("RTM_DELLINK arrives from the monitor as a down event"); `resolve.go` `onLinkEvent` | static never hears the device return or leave | `TestStaticInterfaceRouteFollowsDevice` (Linux) and the interop scenario | unvalidated |
| A-4 | Closing the unit descriptor in `Cleanup` removes the device before or soon after the next `Dial` | ppp unit lifetime ends with its file | EEXIST on fast reconnect | bounded retry, integration test reconnecting at once | unvalidated |
| A-5 | The iface apply prune step leaves a ppp device named after a pppoe-client alone | prune works from previous config kinds | prune deletes the live session device on reload | `TestApplyKeepsPPPoEClientDevice` | unvalidated |
| A-6 | Static runs internal whenever a route names an interface | `register.go` `warnIfExternal` documents that interface next-hops already need static internal | an external static gets no link events; the waiting route never installs | `warnIfExternal` text gains the waiting-route consequence; no new mechanism | known, documented |
| A-7 | When a device is deleted, the kernel flushes every route through it, in the main and named tables, and a later `RouteDel` answers ESRCH | Linux removes routes whose next-hop device unregisters; `mplscontext_linux.go` already tolerates ESRCH | the owner logs a false failure, or fib-kernel reports `fib-sync-failure` for a route that is correctly gone | `TestStaticInterfaceRouteFollowsDevice` deletes the device and asserts no removal error is logged | unvalidated |
| A-8 | A multipath route with one member on a deleted device is flushed whole by the kernel, not shrunk | kernel `fib_sync_down_dev` on unregister | if shrunk instead, the reprogram with survivors is still correct (replace), so the design holds either way; only the test's intermediate read differs | `TestStaticMixedECMPFollowsDevice` reads the kernel after deletion | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A key longer than 15 octets or invalid as an ifname was accepted before and now fails | parse refusal on upgrade | message names the limit and suggests a shorter name; pre-release, no migration owed |
| R-2 | A configuration that matched `ppp*` or `ppp0` in firewall or static stops matching | traffic not NATed | documented; VyOS mapping guide (child 10) drops the `ppp*` rewrite |
| R-3 | The configured name collides with an existing kernel device not in config | EEXIST every dial | error names the device and the fix; `ze doctor` not needed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the WAN session never comes up (creation fails), or routes and NAT miss the WAN |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | child 12 (same dialer), child 4 (IKE on the WAN name), child 9 and 21 (MSS on the WAN name), `spec-pppoe-virtual-interface-proof.md` (asserts `ppp<N>` exists on both sides: its AC changes to the configured name on the client), `spec-iface-route-arbitration.md` (learned default route) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pppoe-client pppoe0` config, real kernel | → | `Dial` named unit creation | `TestDevPPPSetupNamesUnit` (Linux, user namespace) |
| `pppoe-client pppoe0` with fake dialer | → | `runSession` backend calls use `pppoe0` | `TestPPPoEClientUsesConfiguredDeviceName` |
| `pppoe-client` `source-interface wan` with `ethernet wan { os-name zec0 }`, real kernel | → | `Dial` discovery and `connect(AF_PPPOX)` on `zec0` | `TestDialOSNameSourceInterface` (Linux, user namespace, `integration` tag) |
| static route `interface pppoe0` while the session is down, client and route at boot | → | `verifyCandidateStatic` reads the candidate `interface` section; route waits | `test/static/static-interface-nexthop-pppoe-absent.ci` |
| one reload adding the client and its route together | → | candidate-set verify and apply, independent of apply order | `test/static/static-interface-nexthop-pppoe-same-commit.ci` |
| device up, down, deleted, recreated (main and named table) | → | static link subscription drives `programRouteLocked` | `TestStaticInterfaceRouteFollowsDevice` (Linux, `resolve_integration_linux_test.go`) |
| full CPE: session, static default, masquerade, reconnect | → | everything above | interop `pppoe-named-device-reconnect-ze-client` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session up for `pppoe-client pppoe0` | kernel device `pppoe0` exists; no `pppN` created by the client; address, MTU and learned default route on `pppoe0` |
| AC-2 | Session lost and redialled | the new session's device is again `pppoe0`, whatever unit number the kernel gives it |
| AC-3 | main-table `route 0.0.0.0/0 next interface pppoe0` committed while no session is up | commit accepted; `static show` lists the route with the `pppoe0` next-hop inactive and the route NOT in the skipped set; the kernel main table holds it at `proto 250` within the bounded wait after `pppoe0` is up; after the session drops it is gone from the kernel with no removal error logged; after the reconnect it is back, through the new ifindex, with no reload |
| AC-4 | static next-hop naming an interface no candidate configuration declares and the kernel lacks | skipped at apply with its skip-reason, commit accepted, shown by `static show` and `doctor-static-route-skipped`: unchanged from today |
| AC-10 | one reload adds `pppoe-client pppoe0` and `route 0.0.0.0/0 next interface pppoe0` in the same transaction | verify accepts; the route is waiting (AC-3) and NOT skipped, whichever of iface and static applies first |
| AC-11 | a reload removes `pppoe-client pppoe0` and keeps the route | the name is no longer declared: the route follows AC-4; the link subscription for `pppoe0` is cancelled |
| AC-12 | a reload adds the client and its route, and another participant fails apply | rollback: static's route set and declared set are the previous ones, no waiting route and no subscription for `pppoe0` remain; iface's resolver mapping is the previous one |
| AC-13 | named-table route `table wan-out route 0.0.0.0/0 next interface pppoe0` | installed by static's backend at `proto 251` in that table when `pppoe0` is up; removed with the device without a removal error; reinstalled after reconnect; never in the main table or `show rib` |
| AC-14 | main-table mixed ECMP: `hop 192.0.2.1` plus `interface pppoe0` | while `pppoe0` is absent the route is programmed through `192.0.2.1` alone; once up, through both with their weights; after the session drops, through `192.0.2.1` alone again; the route is never withdrawn while one member is active |
| AC-15 | named-table mixed ECMP, same members as AC-14 | same membership sequence as AC-14, written by static's backend |
| AC-5 | firewall `output-interface pppoe0` masquerade | NATed traffic leaves through the session, counters increment, before and after a reconnect |
| AC-6 | pppoe-client key of 16 octets, or with a slash or space | refused by validation with the IFNAMSIZ limit in the message |
| AC-7 | pppoe-client key equal to an ethernet, bridge or other configured interface name | refused, message names both entries |
| AC-8 | `show interface` for the client | the OS name column shows `pppoe0` |
| AC-9 | iface reload while the session is up | `pppoe0` is not deleted |
| AC-16 | `pppoe-client` `source-interface` (or `pppoe interface` on the access concentrator) names an `ethernet` entry with an `os-name` or `mac match` selector | discovery and the session socket both use the kernel device the selector picks; the configured name never reaches `connect(AF_PPPOX)`; show, logs and events keep the configured name |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `next { interface pppoe0 }` as the guide shows, with `no-default-route` on the client | config to static to FIB on device appearance | interop `pppoe-named-device-reconnect-ze-client` |
| 2 | Masquerades the LAN out `pppoe0` | firewall literal name equals device | same scenario, nft counter |
| 3 | Unplugs the fibre and plugs it back | reconnect, same name, route back | same scenario, accel-ppp terminates the session |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDevPPPSetupNamesUnit` | `internal/component/l2tp/ppp/devppp_integration_linux_test.go` | A-1, A-4: named creation, unit number read back, recreate after close | |
| `TestPPPoEClientUsesConfiguredDeviceName` | `internal/component/iface/pppoe_client_test.go` | AC-1, AC-8 with a fake backend | |
| `TestPPPoEClientNameValidated` | `internal/component/iface/config_test.go` | AC-6, AC-7 | |
| `TestApplyKeepsPPPoEClientDevice` | `internal/component/iface/config_apply_test.go` | AC-9, A-5 | |
| `TestDialSessionSocketUsesResolvedDevice` | `internal/component/l2tp/pppoeclient/source_device_test.go` | AC-16, client: logical `wan` resolved to `eth7`; the session socket seam receives `eth7`, and discovery's ifindex comes from the same binding | |
| `TestInterfaceServerSessionSocketUsesResolvedDevice` | `internal/component/l2tp/pppoe/device_binding_test.go` | AC-16, access concentrator: `createTransport` passes the resolved device, `ifName` stays the configured name | |
| `TestDialOSNameSourceInterface` | `internal/component/l2tp/pppoeclient/source_device_integration_linux_test.go` | AC-16 against a real kernel: veth `zec0`, logical `wan` mapped by os-name, scripted AC through IPCP | |
| `TestDeclaredDynamicNames` | `internal/component/iface/config_test.go` | the pure parser answers pppoe-client keys from one `interface` section, nothing for other kinds, nothing for an absent section | |
| `TestStaticVerifyReadsCandidateInterfaceSection` | `internal/plugins/static/register_test.go` | AC-10: a delivery holding both the client and the route verifies, and the pending delivery carries `pppoe0` in its declared set; a delivery with no `interface` section declares nothing | |
| `TestStaticApplyIgnoresApplyOrder` | `internal/plugins/static/register_test.go` | AC-10: apply with the resolver still publishing the previous config classifies `pppoe0` as waiting, not skipped | |
| `TestStaticRollbackRestoresDeclaredSet` | `internal/plugins/static/register_test.go` | AC-12: undo restores routes, declared set and subscriptions | |
| `TestStaticDeclaredRemovedFallsBackToSkip` | `internal/plugins/static/register_test.go` | AC-4, AC-11 | |
| `TestStaticWaitingMemberNotSkipped` | `internal/plugins/static/locrib_test.go` | AC-3, AC-14 with a fake Loc-RIB sink: inactive member excluded from the path, route not in `skipped` | |
| `TestStaticNamedTableWaitingMember` | `internal/plugins/static/inject_test.go` | AC-13, AC-15 with a fake backend: applyRoute and removeRoute follow link events | |
| `TestStaticRemoveRouteToleratesESRCH` | `internal/plugins/static/backend_linux_test.go` | A-7, static's owner | |
| `TestFIBDeleteToleratesESRCH` | `internal/plugins/fib/kernel/integration_linux_test.go` | A-7, fib-kernel's owner: no `fib-sync-failure` for a route the kernel flushed | |
| `TestStaticInterfaceRouteFollowsDevice` | `internal/plugins/static/resolve_integration_linux_test.go` | AC-13, A-3, A-7 against a real kernel: a dummy device created, set up, deleted and recreated under a declared name; the named-table route read back after each step | |
| `TestStaticMixedECMPFollowsDevice` | `internal/plugins/static/resolve_integration_linux_test.go` | AC-15, A-8, named table | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pppoe-client name length | 1-15 | 15 | 0 | 16 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-client-name-too-long` | `test/parse/` | AC-6 | |
| `pppoe-client-name-collides` | `test/parse/` | AC-7 | |
| `static-interface-nexthop-pppoe-absent` | `test/static/` | AC-3 at commit: client and route at boot with no access concentrator; `static show` reports the next-hop inactive and the route not skipped | |
| `static-interface-nexthop-pppoe-same-commit` | `test/static/` | AC-10: boot without the client, one reload adds the client and the route; the reload succeeds and the route is waiting, not skipped | |
| `static-interface-nexthop-pppoe-follows-device` | `test/static/` (`option=needs-linux:caps=net-admin`) | AC-3, AC-14 main table through the real pipeline: a fixture creates a dummy device named `pppoe0` (no access concentrator answers, so the dialer never creates one), sets it up, deletes it and recreates it; after each step the kernel main table is read for the `proto 250` route and its members; `reject=stderr` on a removal or `fib-sync-failure` line | AC-3 and AC-14 proven on Linux: `test/draft/static/static-interface-nexthop-pppoe-follows-device.ci` passed on Ze's 7.3 kernel in QEMU, after the connected-routes fix (the iface monitor never announced addresses present before it started, and `connected` counted events instead of addresses). Promotion to `test/static/` owed |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-named-device-reconnect-ze-client` | `test/interop-pppoe/scenarios/` | accel-ppp | device named `pppoe0`, static default and masquerade through it, `accel-cmd` terminate, reconnect, route and NAT back. Run by `./le test deployment docker-pppoe-accel` | |

### Targeted runs (development evidence)
| What | Invocation |
|------|------------|
| Unit tests | `./le job run label home-cpe-1 command go test -race ./internal/component/iface/ ./internal/component/l2tp/ppp/ ./internal/plugins/static/ ./internal/plugins/fib/kernel/` |
| Functional | `./le test static` and `./le test functional parse` |
| Interop | `./le test deployment docker-pppoe-accel` |

## Files to Modify
- `internal/component/l2tp/ppp/devppp_linux.go`, `devppp_other.go` - named unit creation through rtnetlink; PPPIOCNEWUNIT path removed for the client (the LNS and AC keep theirs: unnamed subscriber units)
- `internal/component/l2tp/pppoeclient/dialer.go` - pass the name, bounded retry
- `internal/component/iface/pppoe_client.go` - device name from config in `runSession` and `status`
- `internal/component/iface/config.go` - key validation and cross-kind uniqueness; the pure declared-dynamic-names parser over one `interface` section
- `internal/component/iface/config_apply.go` - prune leaves the client device alone (A-5 decides whether a change is owed; `TestApplyKeepsPPPoEClientDevice` is written either way)
- `internal/plugins/static/register.go` - `verifyCandidateStatic` reads the candidate `interface` section; `pendingSection`, `staticTx` and `applyRouteSet` carry the declared set; `warnIfExternal` names the waiting-route consequence
- `internal/plugins/static/locrib.go` - `staticPath` builds the path from active members; `checkNextHopResolvable` comment corrected (it skips at apply, it does not fail the transaction)
- `internal/plugins/static/inject.go` - device presence in `nhState`; link subscriptions per declared name; `programRouteLocked` on each link event
- `internal/plugins/static/backend_linux.go` - `netlinkStaticBackend.removeRoute` treats ESRCH as done
- `internal/plugins/fib/kernel/backend_linux.go`, `nexthop_linux.go` - `delRoute` and `delRichRoute` treat ESRCH as done
- `docs/architecture/core-design.md` (declared by `internal/plugins/fib/kernel/backend_linux.go`) section 16 "FIB Pipeline" - checked: it describes add, update and withdraw entries and says nothing about a withdraw whose route the kernel already flushed; one sentence is added there stating that a withdraw answered ESRCH is complete, so the page stays true
- `internal/component/iface/yang/ze-iface-conf.yang` - list and `name` descriptions; `length "1..15"` and the ifname pattern
- Docs: `docs/guide/static-routes.md` "Interface-only next-hops" and "Mixed ECMP" (example now true; the route waits for the session and follows it), `docs/architecture/l2tp/cpe-1-pppoe-client.md`, `docs/architecture/iface/logical-name-resolution.md` (pppoe-client is a created kind), `docs/architecture/static-routes.md` ("An interface next hop is resolved TWICE" corrected to the apply-time skip; "ECMP and BFD" gains device presence; candidate `interface` section beside `candidateTables`), `docs/guide/firewall.md` "Interface Wildcard" (name the PPPoE client by its configured name)
- `docs/architecture/l2tp/bng-5-pppoe.md` (declared by `devppp_linux.go` and `devppp_other.go`) - "Patterns worth reusing", "Shared PPP integration ... kernel setup is shared" and the paragraph on the extracted identical ioctl sequences become wrong: the PPPoE client now creates its unit by name through rtnetlink while the LNS and AC keep PPPIOCNEWUNIT; say which transport takes which path
- `docs/features/interfaces.md` (declared by `config.go` and `config_apply.go`) - the Capability Table row "PPPoE | missing" and "Input validation (name, VLAN, MTU)" need the pppoe-client kind, its key limit (1 to 15 octets, ifname pattern, unique across kinds) and that its device carries the configured name; the logical-name section gains the pppoe-client as a created kind

## Files to Create
- `internal/component/l2tp/ppp/devppp_integration_linux_test.go`
- `test/parse/pppoe-client-name-too-long.ci`, `test/parse/pppoe-client-name-collides.ci`, `test/static/static-interface-nexthop-pppoe-absent.ci`, `test/static/static-interface-nexthop-pppoe-same-commit.ci`, `test/static/static-interface-nexthop-pppoe-follows-device.ci`
- `test/interop-pppoe/scenarios/pppoe-named-device-reconnect-ze-client/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | constraints on an existing key, descriptions |
| YANG validation constraints | Yes | `length`, ifname `pattern` |
| YANG custom validators | Yes | cross-kind uniqueness of interface names |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | rtnetlink ppp kind uses /dev/ppp, already checked |
| Prometheus counters/metrics | No | |
| BGP family surface | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/` PPPoE client and static routes entries |
| 2 | Config syntax changed? | Yes | key constraints; `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes | show output OS name; `docs/guide/command-reference.md` if it shows `pppN` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | static and fib behaviour; `docs/guide/plugins.md` if it describes resolution |
| 6 | Has a user guide page? | Yes | `docs/guide/static-routes.md`, `docs/guide/firewall.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/logical-name-resolution.md`, `docs/architecture/static-routes.md`, `docs/architecture/l2tp/cpe-1-pppoe-client.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin/event/command changed? | No | |
| 16 | Source anchors | Yes | derived at implementation |
| 17 | Existing examples for this area? | Yes | `docs/guide/static-routes.md` "Interface-only next-hops" and "Mixed ECMP" examples (`pppoe0`, `pppoe1`) |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `Dial` receives the name; `runSession` and `status` use it; named creation stubbed to fail
   - Tests: `TestPPPoEClientUsesConfiguredDeviceName`, `TestDevPPPSetupNamesUnit`
   - Files: `dialer.go`, `pppoe_client.go`, `devppp_linux.go`
   - Verify: fake-dialer test passes on names; kernel test fails on the stub
2. **Phase: named unit** -- rtnetlink creation, unit number read back, EBUSY and EEXIST retry; AC-1, AC-2
3. **Phase: validation** -- key length, pattern, uniqueness; AC-6, AC-7; prune check AC-9
4. **Phase: static candidate contract** -- iface's declared-names parser; static verify reads the candidate `interface` section; pending delivery and journal carry the declared set; AC-4, AC-10, AC-11, AC-12
5. **Phase: device presence** -- link subscriptions, presence as an activity input beside BFD, ESRCH tolerated by both removal owners; AC-3, AC-13, AC-14, AC-15
6. **Phase: interop and docs** -- scenario with reconnect and nft counters; AC-5; every page above

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | no consumer still builds `ppp` plus unit number for the client |
| Data flow | static learns "declared, may be absent" from iface's parser over the CANDIDATE section, never from the resolver's published state and never from the pppoe-client list |
| Apply order | no static test or code path assumes iface applied first in the same transaction |
| Rule: no-layering | PPPIOCNEWUNIT unnamed path not kept for the client beside the named one |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `ppp` plus unit name in the client | `grep -n 'StrInt("ppp"' internal/component/iface/pppoe_client.go` empty |
| Guide example true | interop scenario green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | key validated as an ifname before it reaches netlink |
| Isolation | a client never creates or deletes a device it did not create (EEXIST is reported, never resolved by deleting) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken | STOP: return to the owner with the kernel evidence |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Red Walks (the former Blocker, cleared 2026-11-08)

The first closure attempt stopped because no red phase was recorded for the
scenario or the `.ci` files (`ai/rules/interop-and-goal-validation.md`). The
main thread ran each break on 2026-11-08 through a Go build overlay
(`GOFLAGS=-overlay=<walk>/overlay.json`, driver
`tmp/session/2026-11-07-104251/scratch/rw1/run.sh`, logs beside it), rebuilding
the lab image or the guest binaries each time. The closure agent diffed each
overlay copy against the source its `overlay.json` names: each differs only by
the break below (the w5b YANG copy also lacks the RFC 8106 RDNSS sentence that
8d13a9dceb added to an unrelated leaf after the copy was taken).

| # | Test | Break (producer) | Observed red | Log |
|---|------|------------------|--------------|-----|
| 1 | interop `pppoe-named-device-reconnect-ze-client` | `pppoeclient.Dialer.Dial` passes `""` instead of `cfg.Name` to `ppp.DevPPPSetup` | `FAIL (1m16s): first session: wait for PPP device pppoe0 in ze timed out`; `0 passed, 1 failed` | `w1-dial-unnamed-lab.log` |
| 2 | the same scenario | `ifaceLinks.subscribe` (`internal/plugins/static/link.go`) forwards events until the first LinkDown after a LinkUp, then drops every later one | `FAIL (37s): session after accel-ppp terminated the first (ifindex 11): the static default route did not reach the kernel through pppoe0, got ""` | `w2b-linkwatch-no-return-lab.log` |
| 3 | `static-interface-nexthop-pppoe-absent.ci`, `static-interface-nexthop-pppoe-same-commit.ci` (QEMU guest, Ze 7.3) | `iface.DeclaredDynamicNames` answers no names (`if false && present`) | both FAIL: `ZE-OBSERVER-FAIL: boot, no session: 0.0.0.0/0 is skipped (interface "pppoe0" ... not found), want it waiting for pppoe0`, and the same skip `after one reload adding client and route` | `w3-declared-empty-guest.log` |
| 4 | `static-interface-nexthop-pppoe-follows-device.ci` (QEMU guest) | the link-watch break of row 2 | FAIL: `ZE-OBSERVER-FAIL: device recreated: 198.18.76.0/24 is ""` | `w2b-linkwatch-no-return-guest.log` |
| 5 | `pppoe-client-name-too-long.ci`, `pppoe-client-name-collides.ci` (QEMU guest) | YANG `length "1..15"` and `pattern` removed from `list pppoe-client` `leaf name`; the key's `ValidateIfaceName` call and `ifaceConfig.refuseDuplicateInterfaceNames` removed (`internal/component/iface/config.go`) | both FAIL: `configuration valid: .../stdin-toolong.conf` and `configuration valid: .../stdin-collide.conf` | `w5b-name-checks-all-guest.log` |

Restore: on the unbroken tree the scenario passed again
(`pppoe-named-device-reconnect-ze-client: pass (9s)`, `interop: 1 passed, 0
failed`, `restore-lab.log`). The `.ci` files passed on the unbroken tree at
fb9671a9f9, 6d194604f9 and 5ee1ebb500 (table below).
`docs/labs/pppoe-interop.md` now records the passing run and both scenario reds.

Verified at the first closure attempt, and standing as evidence for these ACs:

| AC | Producer | Evidence |
|----|----------|----------|
| AC-1, AC-2 | `ppp.createPPPLink` (`devppp_linux.go`: RTM_NEWLINK kind ppp, bounded EBUSY/EEXIST retry); `Dialer.Dial` passes `cfg.Name` | `TestDevPPPSetupNamesUnit` PASS on Ze 7.3 in QEMU (2026-11-07); interop pass |
| AC-3, AC-13, AC-14, AC-15 | `internal/plugins/static/link.go`, `inject.go` `programRouteLocked`; ESRCH as done in `netlinkStaticBackend.removeRoute` and fib-kernel `delOwnedRoute` | `TestStaticInterfaceRouteFollowsDevice`, `TestStaticMixedECMPFollowsDevice`, `TestStaticRemoveRouteToleratesESRCH`, `TestFIBDeleteToleratesESRCH` PASS on Ze 7.3; follows-device `.ci` passed (6d194604f9) |
| AC-4, AC-10, AC-11, AC-12 | `register.go` `verifyCandidateStatic`, `candidateDeclaredNames`, `classifyDeclared` over `iface.DeclaredDynamicNames` | `register_test.go` units; absent and same-commit `.ci` passed on Linux (fb9671a9f9) |
| AC-5 | nft keeps the literal name (`lowerIfaceMatch`), the device carries it | interop pass: masquerade counter rises before and after the reconnect |
| AC-6, AC-7, AC-9 | YANG `length "1..15"` and `pattern` on `list pppoe-client` `leaf name`; key validation in `config.go`; prune keeps the device | parse `.ci` passed on Linux (5ee1ebb500); `TestPPPoEClientNameValidated`, `TestApplyKeepsPPPoEClientDevice` |
| AC-8 | `PPPoEClient.status` reports `c.cfg.Name` | `TestPPPoEClientUsesConfiguredDeviceName` |
| AC-16 | `pppoe.InterfaceBinding.Device`; `PPPoECreate` takes the binding (289007ccf5) | `TestDialOSNameSourceInterface` PASS on Ze 7.3 in QEMU (2026-11-07) |

## Design Insights

The resolver exists to bind a logical name to hardware Ze did not create.
A PPPoE session device is created by Ze, like a bridge or a WireGuard device,
so it belongs on the other side of that line: created under its name, with no
mapping at all.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `identity-default-hides-a-mapping.md` | 2026-11-02 | `pppoe-client` configured name never reached the kernel device (fixed by this spec) |
| `queued-request-waits-for-an-unrelated-trigger.md` | 2026-11-02 | static route through a PPPoE device not reinstalled after a reconnect (fixed by this spec) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Create the PPP unit under the configured name (rtnetlink kind `ppp`, as pppd's `ifname` option does) | publish a logical-to-`pppN` mapping in the resolver and translate everywhere | nft `iifname`/`oifname` are strings compared per packet: a mapping means rewriting the ruleset on every reconnect, with a window where a rule names a `pppN` that is gone or, on a box that is also an LNS or AC, now belongs to a subscriber session. The firewall today translates nothing. Created kinds are already made under their logical name (`logical-name-resolution.md`), so this keeps one rule |
| A route through a declared, absent dynamic interface waits as an inactive member; static re-decides it on each link event | (a) today's skip, retried only on the next unrelated reload; (b) insert the path anyway and let fib-kernel retry on link events | (a) leaves the WAN default unrouted after every reconnect. (b) makes sysrib select a path no data plane can write, hiding a working alternative, and gives fib-kernel a second job for the main table while named tables would still need static. Waiting inside static matches BIRD (`static_decide`, `static_neigh_notify`) and reuses the BFD membership path, with one owner per table unchanged |
| Static's verify and apply use the CANDIDATE `interface` section | read the resolver's published declared set at apply | the resolver is published by iface's own apply, and "the engine does not track inter-plugin dependencies during apply": a route added with its client would pass or fail by apply order. `candidateTables` already settled this for tables |
| Declared names answered by an iface function over the section | static reads the pppoe-client list | static must not learn interface kinds (registration over enumeration) |
| No new verify refusal for an undeclared name | refuse at verify a name neither declared nor present | a device another program created is a valid next-hop and verify cannot know it will exist; the skip with a visible reason is the current, tested behavior (`static-per-route-isolation.ci`) |
| Keep IPCP-learned default route unchanged | move it to static | out of scope; `spec-iface-route-arbitration.md` owns route arbitration |

## Known Limitations
- Found while surveying, not in this child's scope as the umbrella states it ("any non-ethernet name"): an `ethernet` entry with `os-name` or `mac match` is named literally by firewall and NAT (`lowerIfaceMatch` takes the configured name), so `input-interface wan` with `ethernet wan { os-name eth2 }` matches nothing. Recorded as an open row in `plan/journal/identity-default-hides-a-mapping.md` (2026-11-02), owned by `spec-iface-resolution-across-the-plugin-boundary.md`.
- L2TP LNS and PPPoE AC subscriber devices stay `pppN`: they have no configured name.
- Device presence drives only a next-hop that NAMES the declared interface. A gateway next-hop (`hop 192.0.2.1`) whose address is reachable only through the session is not tied to the device, as today: its activity stays BFD's. BIRD ties a gateway to its neighbor's interface (`neigh_find` scope); whether Ze should is a separate defect class, not this spec's goal, and gets a journal row from the main thread if wanted.
- Other non-ethernet kinds checked: dummy, veth, tunnel, wireguard, xfrm, bridge are all created under their configured name (`config_apply.go`), so they need nothing.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass: the scoped package tests, tagged units and functional tests of the components this change reaches. No agent runs `./le verify`; the full verification is Thomas's release check, run by GitHub CI/CD on a pushed commit
- [ ] Every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it

---

## Implementation Summary

### What Was Implemented
- The PPPoE client creates its PPP unit under the configured name: `ppp.DevPPPSetup` takes a name and, when it is set, `createPPPLink` sends RTM_NEWLINK kind `ppp` with `IFLA_IFNAME` and `IFLA_PPP_DEV_FD` (NLM_F_EXCL, bounded EBUSY/EEXIST retry), then reads the unit back with PPPIOCGUNIT. The LNS and the AC keep PPPIOCNEWUNIT (a65e5c69c6, 1d7b88a626)
- `PPPoEClient.runSession` and `status` address and report the device by `cfg.Name` (1d7b88a626, a65e5c69c6)
- The pppoe-client key is validated as a kernel device name (YANG `length "1..15"`, `pattern`, `ValidateIfaceName`) and refused when another configured interface of any kind carries it (`ifaceConfig.refuseDuplicateInterfaceNames`) (a8932e4e13)
- `iface.DeclaredDynamicNames` answers the on-demand names of one delivered `interface` section; static's `verifyCandidateStatic` classifies each next-hop against the candidate section (`candidateDeclaredNames`, `classifyDeclared`, `nextHop.Dynamic`), so apply, rollback and the pending delivery carry the set verified with their routes (71fac007f6)
- Device presence is a second next-hop activity input beside BFD: `internal/plugins/static/link.go` keeps one `iface.Subscribe` watch per declared name a committed route uses, and `setLinkLocked`/`markDevice` reprogram on LinkUp, LinkDown and an ifindex move. Both removal owners treat ESRCH as done (`netlinkStaticBackend.removeRoute`, fib-kernel `delOwnedRoute`) (5982501268)
- PPPoE session sockets bind the resolved device (`pppoe.InterfaceBinding.Device`), on the client and the access concentrator (289007ccf5)
- Interop scenario `pppoe-named-device-reconnect-ze-client` and its checker `checkNamedDeviceReconnect` (69ecaa5a44)

### Bugs Found/Fixed
- Connected routes never saw addresses that predated the iface monitor, which kept the follows-device `.ci` red; fixed by 44d3dab2ff and proven by `static-interface-nexthop-pppoe-follows-device.ci`
- AC-16: the session socket took the configured name, not the resolved device; covered by `TestDialSessionSocketUsesResolvedDevice`, `TestInterfaceServerSessionSocketUsesResolvedDevice`, `TestDialOSNameSourceInterface`

### Documentation Updates
- Pages updated with the code: `docs/architecture/static-routes.md`, `docs/architecture/core-design.md` (ESRCH), `docs/guide/static-routes.md`, `docs/architecture/iface/logical-name-resolution.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/architecture/l2tp/cpe-1-pppoe-client.md`, `docs/features/interfaces.md`, `docs/guide/firewall.md`, `docs/guide/pppoe.md`, `docs/guide/configuration.md` (commits 71fac007f6, 5982501268, 69ecaa5a44, a8932e4e13, 289007ccf5)
- At closure: `docs/labs/pppoe-interop.md` "pppoe-named-device-reconnect-ze-client" records the passing run and the two scenario reds (anchors `dialer.go -- Dialer.Dial`, `link.go -- ifaceLinks.subscribe`); `features/static-routes.md` gains the on-demand interface clause, the three static `.ci` files as real-path tests, a `link.go` anchor and a 2026-11-08 Doc review
- `./le doc check all`: FAILED on surfaces this spec does not touch (the `../gh-pages` command-equivalent pages for `show interface pppoe-client`, and the command-help contract for `clear debug`); source anchors pass

### Deviations from Plan
- `TestPPPoEClientNameValidated` lives in `internal/component/iface/pppoe_client_test.go`, and `TestDeclaredDynamicNames` in `declared_dynamic_test.go`, not `config_test.go`
- `features/interfaces.md` real-path tests do not list the two parse `.ci` files: that declaration's Doc review is already stale from other sessions' changes to `docs/features/interfaces.md` on 2026-11-08, and re-reviewing it is outside this spec

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first closure attempt found no red phase recorded for the scenario or the five `.ci` files | Implementation promoted tests without a recorded break | `/ze-close` step 1, 2026-11-08 | Spec handed back blocked; the main thread ran the five walks; recorded under "Red Walks" |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The configured name of a pppoe-client is its kernel device, every session and reconnect | Done | `internal/component/l2tp/ppp/devppp_linux.go` `DevPPPSetup`, `createPPPLink`; `pppoeclient.Dialer.Dial` | red walk 1 |
| Every consumer naming it reaches the session | Done | nft keeps the literal name (`lowerIfaceMatch`); static resolves by name | interop masquerade counter |
| Static route survives absence at boot and across reconnects, main and named table, alone or in mixed ECMP | Done | `internal/plugins/static/link.go`, `inject.go` `programRouteLocked` | red walks 2-4 |
| Client and route added in one commit are accepted | Done | `register.go` `verifyCandidateStatic`, `candidateDeclaredNames` | red walk 3 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestDevPPPSetupNamesUnit`; interop first session | red walk 1 |
| AC-2 | Done | interop reconnect (new ifindex) | |
| AC-3 | Done | absent `.ci`, follows-device `.ci`, interop | red walks 2-4 |
| AC-4 | Done | `TestStaticDeclaredRemovedFallsBackToSkip`; `static-per-route-isolation.ci` unchanged | |
| AC-5 | Done | interop masquerade counter before and after reconnect | |
| AC-6 | Done | `pppoe-client-name-too-long.ci`, `TestPPPoEClientNameValidated` | red walk 5 |
| AC-7 | Done | `pppoe-client-name-collides.ci` | red walk 5 |
| AC-8 | Done | `TestPPPoEClientUsesConfiguredDeviceName` | |
| AC-9 | Done | `TestApplyKeepsPPPoEClientDevice` | |
| AC-10 | Done | same-commit `.ci`, `TestStaticApplyIgnoresApplyOrder` | red walk 3 |
| AC-11 | Done | `TestStaticDeclaredRemovedFallsBackToSkip` | |
| AC-12 | Done | `TestStaticRollbackRestoresDeclaredSet` | |
| AC-13 | Done | `TestStaticInterfaceRouteFollowsDevice` (Ze 7.3) | |
| AC-14 | Done | follows-device `.ci`, `TestStaticWaitingMemberNotSkipped` | red walk 4 |
| AC-15 | Done | `TestStaticMixedECMPFollowsDevice` (Ze 7.3) | |
| AC-16 | Done | `TestDialOSNameSourceInterface` (Ze 7.3), two unit tests | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| 18 unit and integration tests of the plan | Done | each found by `grep -rln "func <name>("` at closure | locations above and in Deviations |
| 5 functional `.ci` | Done | `test/parse/`, `test/static/` | red walks 3-5 |
| interop scenario | Done | `test/interop-pppoe/scenarios/pppoe-named-device-reconnect-ze-client/` | red walks 1-2 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify (code) | Done | every file listed changed in the seven child-1 commits |
| Files to Create | Done | see Files Exist below |
| Docs | Done | Documentation Updates above |

### Audit Summary
- **Total items:** 4 requirements, 16 ACs, 24 tests, plan files
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 test file locations (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The configured name is the kernel device on every session and reconnect | interop | `pppoe-named-device-reconnect-ze-client` pass (9s) on 2026-11-08; red with `Dial` passing `""` (walk 1) |
| Every consumer naming it reaches the session | interop | same scenario: static default through `pppoe0` at proto 250 and masqueraded ping with a rising nft counter, before and after `accel-cmd terminate` |
| A static route through it survives absence at boot and across reconnects, main and named table, alone or in mixed ECMP | functional + kernel integration | absent and follows-device `.ci` red under walks 3-4; `TestStaticInterfaceRouteFollowsDevice`, `TestStaticMixedECMPFollowsDevice` PASS on Ze 7.3 |
| A commit adding the client and its route together is accepted | functional | `static-interface-nexthop-pppoe-same-commit.ci`, red under walk 3 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by `./le spec review record` over the closure files; its path and the check result are in commit A's message |
| `./le spec review check` | run before commit A |
| Rounds | 1 |
| Reviewer lenses used | wiring and functional coverage; logic and concurrency of the link watch; security of the name validation and netlink create; removed-behavior and test-rewrite audit (`./le test weakened check base 1d7b88a626~1`: clean, 299 test files) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | no BLOCKER or ISSUE | - | - |

### Run 1 NOTEs
1. NOTE: `DevPPPSetup` blames a name collision when PPPIOCGUNIT fails after a successful named create; with NLM_F_EXCL that create cannot adopt another program's device, so the stated cause is unlikely (`internal/component/l2tp/ppp/devppp_linux.go` `DevPPPSetup`).
2. NOTE: a key equal to a device Ze does not list as a configured entry (`lo`, a VLAN unit such as `eth0.100`) passes parse and fails at dial after the bounded EEXIST retry, with a message naming the fix (R-3).
3. NOTE: `iface.BindResolverEventsForConsumerTest` is an exported test hook in a production file, matching about 59 existing `...ForTest` hooks.
4. NOTE: the per-route skip of a declared member whose device vanishes between `ensureLinkLocked` and `staticPath` waits for the next apply, the same as any skip.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/ppp/devppp_integration_linux_test.go` | Yes | ls 2026-11-08, 2.6K |
| `test/parse/pppoe-client-name-too-long.ci`, `pppoe-client-name-collides.ci` | Yes | ls 2026-11-08 |
| `test/static/static-interface-nexthop-pppoe-absent.ci`, `-same-commit.ci`, `-follows-device.ci` | Yes | ls 2026-11-08 |
| `test/interop-pppoe/scenarios/pppoe-named-device-reconnect-ze-client/` | Yes | accel-ppp.conf, chap-secrets, role, ze.conf |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | named create | `Dialer.Dial` passes `cfg.Name` to `DevPPPSetup`; `createPPPLink` retries only EBUSY/EEXIST; `grep 'StrInt("ppp"' pppoe_client.go` empty |
| AC-3, AC-13..15 | waiting member follows its device | red walks 2-4 observed 2026-11-08 |
| AC-4, AC-10..12 | candidate classification | `verifyCandidateStatic` calls `candidateDeclaredNames` then `classifyDeclared`; red walk 3 |
| AC-5 | NAT through the session | restore run pass (9s) |
| AC-6, AC-7 | name refusals | YANG `length "1..15"` and `pattern` on `list pppoe-client` `leaf name`; `parseIfaceConfig` calls `ValidateIfaceName` on the key and `refuseDuplicateInterfaceNames`; red walk 5 |
| AC-8, AC-9, AC-16 | status name, prune, resolved device | `PPPoEClient.status` reports `c.cfg.Name`; `pppoe.PPPoECreate` takes an `InterfaceBinding` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| client and route at boot, no session | `test/static/static-interface-nexthop-pppoe-absent.ci` | red under walk 3 proves it drives `DeclaredDynamicNames` |
| one reload adding both | `test/static/static-interface-nexthop-pppoe-same-commit.ci` | red under walk 3 |
| device up, down, deleted, recreated | `test/static/static-interface-nexthop-pppoe-follows-device.ci` | red under walk 4 proves it drives the link watch |
| key validation | `test/parse/pppoe-client-name-too-long.ci`, `-collides.ci` | red under walk 5 |
| full CPE | interop `pppoe-named-device-reconnect-ze-client` | red under walks 1 and 2 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestDevPPPSetupNamesUnit` PASS on Ze 7.3 in QEMU; interop first session |
| A-2 | confirmed | `sendPPPLinkCreate` encodes the request with the vendored `nl` package |
| A-3 | confirmed | `TestStaticInterfaceRouteFollowsDevice` and the interop reconnect |
| A-4 | confirmed | interop reconnect brings up a new `pppoe0`; `createPPPLink` bounded EEXIST retry |
| A-5 | confirmed | `TestApplyKeepsPPPoEClientDevice` |
| A-6 | confirmed | `warnIfExternal` names the waiting-route consequence (`register.go`) |
| A-7 | confirmed | `TestStaticRemoveRouteToleratesESRCH`, `TestFIBDeleteToleratesESRCH` PASS on Ze 7.3 |
| A-8 | confirmed | `TestStaticMixedECMPFollowsDevice` PASS on Ze 7.3 |
