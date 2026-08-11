# Spec: fixit-route-removal-protocol-blind

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | - |
| Updated | 2026-08-11 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`netlinkBackend.RemoveRoute`
(`internal/plugins/iface/netlink/manage_linux.go`) builds a `netlink.Route` from
destination, gateway, link index and metric, then calls `RouteDel`. It sets no
`Protocol` field. The match is protocol-blind, so any kernel route that shares
that four-tuple is deleted, whoever installed it.

The routes it exists to remove are themselves untagged. The DHCPv4 client
(`internal/plugins/iface/dhcp`) and the PPPoE client
(`internal/component/iface/pppoe_client.go`) install their default route through
`netlinkBackend.AddRoute`, which also sets no `Protocol`, so those routes land in
the kernel as `RTPROT_UNSPEC`.

Symptom: a static default route configured through `internal/plugins/static`,
which correctly stamps `Protocol: rtprotStatic`, is deleted when a DHCP lease
expires or a link bounces, provided it shares gateway, interface and metric with
the DHCP-learned default. `RemoveRoute` cannot tell the two apart. The static
plugin's own delta never touches foreign routes, and `fibkernel` sweeps only
`rtprotZE`, so the hazard is entirely on the iface side.

Goal: stamp DHCP-installed and PPPoE-installed routes with their own protocol
value in `internal/core/rtproto`, and make `RemoveRoute` match on that protocol
so it can only remove routes it installed.

Provenance: VyOS T9054 (`frrender: keep PPPoE/DHCP default route when "protocols
static" is deleted`) is the mirror image of this failure. Ze is structurally
immune to the VyOS direction. This spec covers the direction Ze is exposed
to.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/features/interfaces.md` - the feature table this backend answers to
  -> Decision: DHCP installs its default route by direct netlink, so the iface
     layer, not the FIB plugin, owns those routes and their protocol id
  -> Constraint: link-down deprioritization is remove-then-add at metric + 1024,
     so one link bounce runs the delete lever twice

### RFC Summaries (Scope: protocol)
Not applicable. rtm_protocol is a Linux kernel field, not a protocol Ze speaks
on a wire. No RFC governs the value.

**Key insights:** (minimal context to resume after compaction)
- A zero `Protocol` in RTM_DELROUTE is a kernel WILDCARD. The delete is the lever.
- Two routes that share destination, gateway, link and metric cannot coexist:
  the kernel answers EEXIST, and `RouteReplace` overwrites the protocol.

## Current Behavior (MANDATORY)

**Source files read:** (research pass 2026-08-10, every claim at its producer)
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - `(*netlinkBackend).AddRoute` and `RemoveRoute` both build `netlink.Route{LinkIndex, Dst, Gw, Priority}` and set no `Protocol`. `RemoveRoute` swallows `ESRCH`.
  -> Constraint: **a zero `Protocol` in the DELETE is a WILDCARD, so stamping the ADD alone changes nothing.** `RouteDel` uses `nl.NewRtDelMsg()`, whose Protocol is 0, and `prepareRouteReq` leaves it. Verified against a running kernel in an ephemeral netns: `ip route del` with no `proto` deleted a `proto static` route; the same delete with `proto 250` returned `ESRCH` and left it. **The DELETE side is the fix lever.**
  -> Decision: the spec's Task was wrong on one point. Routes land as `RTPROT_BOOT` (3), not `RTPROT_UNSPEC`: `RouteReplace` builds with `nl.NewRtMsg()`, which sets `Protocol: unix.RTPROT_BOOT`, and `prepareRouteReq` overrides only `if route.Protocol > 0`.
- [ ] `internal/core/rtproto/rtproto.go` - exports `FIBKernel = 250`, `Static = 251`, `PolicyRoute = 252`, each with a `zeNames` entry driving `IsZe` and `Name`. 253 is free.
  -> Decision: the Task named `rtprotStatic` and `rtprotZE` as living here. They do not. They are package aliases in `internal/plugins/static/backend_linux.go` and `internal/plugins/fib/kernel/backend_linux.go`.
- [ ] `internal/component/iface/register.go` - `cleanupStaleIPv6DefaultRoutes` removes routes the KERNEL installed from RAs, not Ze's.
  -> Constraint: **that caller MUST stay protocol-blind.** A `RemoveRoute` that unconditionally stamps breaks it silently, because `ESRCH` is swallowed. Any design making the protocol implicit rather than a parameter is wrong here.
- [ ] `internal/core/routewatch/routewatch.go` - `(*Watcher).deliver` SUPPRESSES events for any protocol `IsZe` reports true for.
  -> Constraint: giving iface routes an `rtproto` constant listed in `zeNames` would silence DHCP, RA and PPPoE route events for every `routewatch` handler. AC-6 exists for this.
- [ ] `internal/plugins/fib/kernel/backend_linux.go` - `listZeRoutes` skips any route whose `Protocol != rtprotZE`, and `delRoute` stamps the delete.
  -> Decision: `fibkernel` is NOT exposed. Neither is `internal/plugins/static`, whose `removeRoute` deletes through the same stamping builder. The hazard is entirely on the iface side, as the Task says.

**Behavior to preserve:**
- `cleanupStaleIPv6DefaultRoutes` removing kernel-installed RA routes (AC-3).
- Every `routewatch` subscriber's event stream (AC-6).
- `RemoveRoute` removing routes Ze installed (AC-4).

**Behavior to change:**
- `RemoveRoute` matches on protocol, so it can only remove what this backend installed, EXCEPT where a caller explicitly asks for a blind match.
- DHCP, RA, PPPoE and L2TP/PPP installed routes carry a protocol value.

## Open decisions (settled 2026-08-10, before code)

| # | Decision | Settled as |
|---|----------|-----------|
| D-1 | Where the protocol travels | An explicit typed parameter on `AddRoute` and `RemoveRoute`, `rtproto.Proto`, with a named `rtproto.Any` for the blind match. A caller reaches a wildcard delete only by typing its name |
| D-2 | Whether the new value joins the `IsZe` set | It does not. `IsZe` is now a switch over the three producer-owned protocols and `Name` reads a separate `names` map that also holds `Iface`, so the value renders without silencing `routewatch` |
| D-3 | A legacy `RTPROT_BOOT` route after an upgrade | The delete keeps returning success and `logRemoveRouteMiss` reports the orphan at WARN with its destination, gateway and interface |

The reasoning each decision replaced:

| # | Decision | Why it was not obvious |
|---|----------|----------------------|
| D-1 | Where the protocol travels | There is no field for it. `RouteInfo` carries Destination, Gateway and Metric only, and `AddRoute`/`RemoveRoute` are declared in three places (`internal/component/iface/backend.go`, `dispatch.go`, and a private `IfaceBackend` redeclaration in `internal/component/l2tp/ppp/session.go`) with about six implementations plus test fakes. Adding a parameter touches them all; the review will ask why an untyped value that every future caller can default to zero is not the same trap this spec is fixing |
| D-2 | Whether the new constant joins `zeNames` | If it does, `IsZe` returns true and `routewatch` goes silent for these routes (AC-6). If it does not, `rtproto.Name` will not render it. Both have a cost |
| D-3 | What `RemoveRoute` does about a legacy `RTPROT_BOOT` route after an upgrade | A stamped delete misses it and `ESRCH` becomes `nil`, so the orphan is invisible even in the logs. Fixing one invisible failure with another is not acceptable (AC-5) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A DHCPv4 lease expiring, a link carrier event, a router-lost event, or a PPP
  session tearing down. Each reaches a route removal.
- Format at entry: an interface name, a destination CIDR, a gateway IP and a
  metric. Nothing in it said who owned the route.

### Transformation Path
1. The producer calls `iface.RemoveRoute` (`internal/component/iface/dispatch.go`)
   and now names the owning protocol as `rtproto.Proto`.
2. Dispatch forwards to the active backend's `RemoveRoute`.
3. `(*netlinkBackend).RemoveRoute` (`internal/plugins/iface/netlink/manage_linux.go`)
   puts that value in `netlink.Route.Protocol` and calls `RouteDel`.
4. The kernel matches destination, gateway, link, metric AND rtm_protocol. A
   value of 0 (`rtproto.Any`) leaves the protocol out of the match.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Component -> backend | `iface.Backend.AddRoute` / `RemoveRoute`, one added typed parameter | Yes |
| Backend -> kernel | rtnetlink RTM_NEWROUTE / RTM_DELROUTE, `rtm_protocol` field | Yes -- `TestRouteIntegration_*` read the kernel back |
| Kernel -> routewatch | RTM_NEWROUTE / RTM_DELROUTE events carry protocol 253, `IsZe` reports false, so `(*Watcher).deliver` still delivers | Yes -- `TestIfaceProtocolIsNamedButNotZeOwned` |

### Integration Points
- `internal/core/rtproto` - holds the new `Proto` type and its `Any` and `Iface` values
- `internal/plugins/iface/netlink/route_linux.go`, `protocolName` - renders 253 as `ze-iface` through `rtproto.Name`
- `internal/plugins/kernel` - redistributes kernel routes and filters only RTPROT_KERNEL, RTPROT_REDIRECT and `rtproto.IsZe`, so iface routes stay redistributable

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every caller goes through `iface.AddRoute`/`RemoveRoute`; no caller builds a `netlink.Route` |
| No unintended coupling (components stay isolated) | Yes | The shared vocabulary is one leaf package, `internal/core/rtproto`, which every tier may import |
| No duplicated functionality (extends existing, does not recreate) | Yes | `rtproto` already held the producer ids; this adds one value and one type |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No buffer or wire encoding on this path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No registry involved. `rtproto` gains one constant beside its three siblings, which is the existing shape, not a new per-feature branch |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A zero `Protocol` in RTM_DELROUTE matches any protocol | research pass against a running kernel | the defect does not exist | `ip route del` with and without `proto` in an ephemeral netns, and `TestRouteIntegration_StampedRemoveLeavesStaticRoute` failing when the stamp is removed | confirmed |
| A-2 | Routes this backend installed before the change carry RTPROT_BOOT, not RTPROT_UNSPEC | `nl.NewRtMsg()` sets `Protocol: unix.RTPROT_BOOT`, and `prepareRouteReq` overrides only when `route.Protocol > 0` | AC-5's legacy case names the wrong value | reverting the add stamp made `TestRouteIntegration_AddStampsAndRemoveDeletesOwnRoute` report `protocol = 3` | confirmed |
| A-3 | 253 is free: no other producer in Ze or in the common daemon range uses it | `rtproto.go` (250, 251, 252 taken) and `rtProtoNames` in `route_linux.go` (11, 16, 42, 186-193) | route output renders the wrong producer | `TestRouteProtocolsAreDistinct`, and the `rtProtoNames` map has no 253 | confirmed |
| A-4 | Making iface routes visible to `routewatch` keeps the kernel redistribution working | `internal/plugins/kernel/kernel.go` filters only RTPROT_KERNEL and RTPROT_REDIRECT; `routewatch` filters `IsZe` | DHCP, RA and PPPoE route churn stops reaching subscribers | `TestIfaceProtocolIsNamedButNotZeOwned` plus `TestWatcherFilterZeOwned` still passing | confirmed |
| A-5 | A static route and an iface route with the same destination, gateway, link and metric can coexist in the kernel | assumed while writing AC-1 | AC-1 must be read as "one route object, currently owned by the static plugin" | measured: `ip route add` answers EEXIST and `ip route replace` overwrites the protocol | broken -- see Mistake Log |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A route installed by an earlier Ze version (RTPROT_BOOT) is never removed after an upgrade | the WARN from `logRemoveRouteMiss`, naming destination, gateway and interface | the operator removes it once with `ip route del`; a later spec can sweep RTPROT_BOOT defaults at start |
| R-2 | A caller added later reaches a wildcard delete by leaving the argument out | none at runtime | closed by construction: the parameter is required and `rtproto.Any` has to be typed out |
| R-3 | The add side still takes over a foreign route that shares destination, metric and table | an operator sees `proto 253` on a route they configured as static | closed for the metric that matters (2026-08-11). A learned default now lands at 254, not 0, so it no longer shares the key an operator's static default occupies. `RouteReplace` still asks no ownership question, so a route an operator puts AT 254 is still taken over. See Known Limitations and `spec-admin-distance-reaches-the-kernel` |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Default routes. A delete that is too narrow leaks a route on lease expiry; a delete that is too wide removes an operator's route. Both are loss of connectivity |
| How is it reverted? | Single commit revert. Routes installed while it ran carry protocol 253, and a reverted (blind) delete still removes them |
| Who else touches this path? | `internal/plugins/static` and `internal/plugins/fib/kernel` install routes with their own protocol and are unaffected. `internal/core/routewatch` and `internal/plugins/kernel` read the protocol of every route event |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| DHCPv4 lease expiry (`(*DHCPClient).removeV4Addr`) | → | `iface.RemoveRoute` with `rtproto.Iface` → `(*netlinkBackend).RemoveRoute` → RTM_DELROUTE | `TestRouteIntegration_StampedRemoveLeavesStaticRoute` |
| Link carrier down / up (`handleLinkDown`, `handleLinkUp`) | → | `RemoveRoute` + `AddRoute` with `rtproto.Iface` | `TestHandleLinkDownWithRoutePriority`, `TestHandleLinkUpWithRoutePriority`, `TestRouteIntegration_LinkBounceKeepsStaticRoute` |
| accept_ra_defrtr suppression (`cleanupStaleIPv6DefaultRoutes`) | → | `RemoveRoute` with `rtproto.Any` | `TestStaleKernelRouteCleanup`, `TestRouteIntegration_BlindRemoveDeletesKernelRoute` |
| Router lost (`handleRouterLost`) | → | `RemoveRoute` with `rtproto.Iface` | `TestNeighRouterRemoved` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A static default route and a DHCP-learned default share gateway, interface and metric. The DHCP lease expires | The static route SURVIVES. Only the DHCP-installed route is removed |
| AC-2 | The same pair, and the link bounces | The static route survives both the down and the up handling |
| AC-3 | `cleanupStaleIPv6DefaultRoutes` runs against kernel-installed RA routes | It still removes them. That caller asks for a protocol-blind match and MUST keep getting one |
| AC-4 | A route this backend installed is removed | It is removed. The stamp does not make Ze unable to delete its own routes |
| AC-5 | A route installed by a PREVIOUS Ze version, carrying `RTPROT_BOOT`, reaches `RemoveRoute` after an upgrade | The outcome is DELIBERATE and stated. Silent `ESRCH` swallowed into success is not acceptable: an orphan must be observable |
| AC-6 | A `routewatch` subscriber during DHCP, RA or PPPoE route churn | It receives the same events it receives today. The new protocol value MUST NOT silence them |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a static default route and enables DHCP on the same interface, then the lease expires | dhcp client -> `iface.RemoveRoute(rtproto.Iface)` -> `RouteDel` with rtm_protocol 253 -> kernel keeps the `proto static` route | `TestRouteIntegration_StampedRemoveLeavesStaticRoute` |
| 2 | unplugs and replugs the cable on that interface | link worker -> `handleLinkDown` / `handleLinkUp` -> stamped remove plus add at metric + 1024 and back | `TestRouteIntegration_LinkBounceKeepsStaticRoute`, `TestHandleLinkDownWithRoutePriority` |
| 3 | sets `route-priority` so Ze takes over IPv6 default routes from RAs | `suppressAcceptRaDefrtr` -> `cleanupStaleIPv6DefaultRoutes` -> `iface.RemoveRoute(rtproto.Any)` -> the kernel's `proto ra` route goes | `TestRouteIntegration_BlindRemoveDeletesKernelRoute`, `TestStaleKernelRouteCleanup` |
| 4 | upgrades from a version that stamped nothing, then the lease expires | dhcp client -> stamped delete -> ESRCH -> WARN naming the route the kernel still holds | `TestRouteIntegration_LegacyBootRouteIsReported` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIfaceProtocolIsNamedButNotZeOwned` | `internal/core/rtproto/rtproto_test.go` | AC-6: `IsZe(Iface)` is false and `Name(Iface)` is not empty | pass |
| `TestAnyIsTheUnsetProtocol` | `internal/core/rtproto/rtproto_test.go` | D-1: `Any` is the named wildcard and names no producer | pass |
| `TestRouteProtocolsAreDistinct` | `internal/core/rtproto/rtproto_test.go` | 253 collides with no existing producer id | pass |
| `TestAddRouteRejectsTheBlindProtocol` | `internal/plugins/iface/netlink/manage_linux_test.go` | an install with `rtproto.Any` is refused, so no unowned route reaches the kernel | pass |
| `TestRemoveRouteMissWarnsThroughTheProductionLogger` | `internal/plugins/iface/netlink/manage_linux_test.go` | AC-5: the orphan WARN reaches slogutil's global log ring, which only a logger built by `slogutil.Logger` feeds, so the record an operator sees is proven and not one a test installed | pass |
| `TestProtocolNameNamesEveryProducer` | `internal/plugins/iface/netlink/route_linux_test.go` | `protocolName` renders 253 as `ze-iface` for `show route` and the web IP Routes page, and still renders `boot`, `kernel`, `bgp`, `ra` and an unknown decimal as before | pass |
| `TestHandleLinkDownWithRoutePriority`, `TestHandleLinkUpWithRoutePriority`, `TestLinkDownIPv6`, `TestLinkUpIPv6`, `TestNeighRouterDetected`, `TestNeighRouterRemoved`, `TestReloadMetricChange`, `TestAcceptRaDefrtrRestore`, `TestMultipleRoutersOnSameLink` | `internal/component/iface/config_test.go` | every iface route call names `rtproto.Iface` (the `routeCall` fixture carries the protocol) | pass |
| `TestStaleKernelRouteCleanup` | `internal/component/iface/config_test.go` | AC-3 at the caller: the RA cleanup names `rtproto.Any` | pass |
| `TestSLAACUnitWithRoutePriorityKeepsAnIPv6DefaultRoute` | `internal/component/iface/route_metric_test.go` | round 4: a unit with `route-priority` and no DHCP of either family is suppressed AND installed for, so it keeps an IPv6 default route | pass |
| `TestWrittenRoutePrioritiesMultiUnit`, `TestWrittenRoutePrioritiesNoMatch` | `internal/component/iface/config_test.go` | round 4: `writtenRoutePriorities` is the one derivation both the suppression and the install read | pass |
| `TestRemoveRouteMissNamesTheProtocolThatHoldsTheRoute` | `internal/plugins/iface/netlink/manage_linux_test.go` | round 4: the ESRCH WARN names the protocol the kernel holds the route under, and states no cause the code did not establish | pass |
| `TestRemoveRouteMissReportsWhatTheTableReadEstablished` | same file | round 5: each of the three outcomes of the readback is reported as itself -- a FAILED read at WARN, a survivor at WARN naming it, an empty read at DEBUG | pass |
| `TestRepeatedLinkEventsMoveTheRouteOnce`, `TestRepeatedLinkEventsMoveTheIPv6RouteOnce` | `internal/component/iface/route_metric_test.go` | round 5: a link event repeating a state ze already acted on reaches no route call, so the readback is off the per-event path; and a router-lost remove names the metric the route is installed at | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `rtproto.Proto` on the wire | 0..255 (`rtm_protocol` is one octet) | 253 (`Iface`) | N/A -- 0 is `Any`, a valid wildcard | N/A -- no value above 253 is used, and 254/255 stay free |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestRouteIntegration_StampedRemoveLeavesStaticRoute` | `internal/plugins/iface/netlink/route_integration_linux_test.go` | AC-1: the DHCP lease expires and the operator's static default route is still there | pass |
| `TestRouteIntegration_LinkBounceKeepsStaticRoute` | same file | AC-2: the link bounces and the static default route is still there | pass |
| `TestRouteIntegration_BlindRemoveDeletesKernelRoute` | same file | AC-3: Ze still clears the kernel's own RA default route | pass |
| `TestRouteIntegration_AddStampsAndRemoveDeletesOwnRoute` | same file | AC-4: Ze still removes the route it installed | pass |
| `TestRouteIntegration_LegacyBootRouteIsReported` | same file | AC-5: a route from an earlier version survives the delete and is named in a WARN | pass -- the `held-by=boot` assertion added in round 4 ran on 2026-08-11 under `make ze-qemu-integration-test` |
| `TestRouteIntegration_RepeatedRemoveDoesNotWarn` | same file | round 4: removing a route that is already gone is silent, so the link monitor's repeated down events do not fill the log with orphan WARNs | pass -- ran on 2026-08-11 under `make ze-qemu-integration-test`, as root in the VM |
| `TestLearnedDefaultRouteReachesTheDHCPClientAtItsOwnMetric` | `internal/component/iface/route_metric_test.go` | a DHCPv4 lease installs its default route at 254, an explicit `route-priority 0` restores metric 0, a written priority still wins | pass |
| `TestLearnedMetricSurvivesTheLinkBounce` | same file | the link-down and link-up shuffle stays on 254 / 254+1024 | pass |
| `TestRAOwnershipStillNeedsAWrittenRoutePriority` | same file | the non-zero default did not make ze suppress `accept_ra_defrtr` or install `::/0` on a unit that never wrote the leaf | pass |
| `TestPPPoEDefaultRouteTakesTheLearnedMetric` | same file | a PPPoE session installs its default route at 254, and `route-priority` reaches the call that passed a literal 0 | pass |
| `TestRoutePriorityYANGDefaultMatchesTheConstant` | same file | both `route-priority` leaves declare the value the parser applies | pass |
| `test/plugin/iface-learned-route-metric.ci` | `test/plugin/` | a static route and a learned route to one destination coexist, the static one keeps its gateway and its `proto static`, and it forwards. The learned metric is read from `ze schema show ze-iface-conf`, so a build that went back to 0 fails here | pass -- 3.7s on a real kernel, first execution |
| `test/plugin/iface-route-protocol-name.ci` | `test/plugin/` | the operator runs `show route` and reads `ze-iface` in the protocol column of a route stamped 253, and `boot` on a route that still carries 3 | pass -- 2.9s on a real kernel |

The integration suite runs under `make ze-qemu-integration-test`, which is the
running proof `ai/rules/platform-linux.md` requires.

**Correction (2026-08-11).** This spec claimed "no CLI surface reports it" and
skipped the `.ci` on that ground. The claim was wrong. `protocolName`
(`internal/plugins/iface/netlink/route_linux.go`) fills
`iface.KernelRoute.Protocol` from `rtproto.Name`, and two surfaces print that
field: `show route` (`internal/component/iface/cmd/show_route.go`, the
`ze-show:route` RPC) and the web IP Routes page
(`internal/component/web/page_ip_routes.go`). So an interface-layer route that
read `boot` before this spec reads `ze-iface` after it, which is a user-visible
change and owes a functional test (`ai/rules/testing.md`, "The Rule"). The
`.ci` row above is that test. `routeFlag` in the web page switches on the same
string and both values fall to its default arm, so the flag column is
unchanged.

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Nothing crosses a wire. `rtm_protocol` is a local kernel field, and the peer of this exchange is the Linux kernel itself, which the integration tests run against | N-A |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/core/rtproto/rtproto.go` - `Proto` type, `Any` and `Iface` values, `IsZe` separated from `Name`
- `internal/core/rtproto/rtproto_test.go` - AC-6 and the `Any` contract
- `internal/component/iface/backend.go` - the `Backend` interface takes the protocol
- `internal/component/iface/dispatch.go` - the package-level `AddRoute` / `RemoveRoute`
- `internal/component/iface/register.go` - the metric shuffle and RA handlers stamp; the RA cleanup names `rtproto.Any`
- `internal/component/iface/pppoe_client.go` - the PPPoE default route stamps
- `internal/component/iface/config_test.go`, `internal/component/iface/migrate_linux_test.go` - fakes carry the protocol and every assertion states it
- `internal/component/l2tp/ppp/session.go` - the private `IfaceBackend` redeclaration
- `internal/component/l2tp/ppp/ncp.go`, `ipv6_service.go`, `helpers_test.go` - PPP route calls and the fake
- `internal/plugins/iface/netlink/manage_linux.go` - stamps the add, matches the delete, reports the ESRCH orphan
- `internal/plugins/iface/netlink/backend_other.go`, `internal/plugins/iface/vpp/ifacevpp.go` - the other two implementations
- `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - the DHCPv4 default route, add and remove
- `docs/features/interfaces.md` - the protocol a route from the interface layer carries

## Files to Create
- `internal/plugins/iface/netlink/route_integration_linux_test.go` - the kernel proof for AC-1 to AC-5
- `internal/plugins/iface/netlink/manage_linux_test.go` - the install-side guard, which needs no kernel

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Nothing is configurable: the protocol id is a property of the producer, not an operator choice |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | No command added or changed |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | Yes | No RPC is added, but `ze-show:route` reports a changed value: `test/plugin/iface-route-protocol-name.ci` asserts the protocol column reads `ze-iface`. The kernel field itself stays proven by the integration suite |
| Pipe completeness | N-A | No new output |
| Env var registration | No | No env var added |
| Doctor check for runtime dependencies | No | No new dependency. The netlink socket and the route table were already used by this backend |
| Prometheus counters/metrics | No | The orphan case is reported through a WARN, which is the observable AC-5 asks for. A counter would need a name and a dashboard nobody asked for |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/interfaces.md` -- a route from the interface layer now renders as `proto 253` / `ze-iface` |
| 2 | Config syntax changed? | No | No leaf added |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | No registration changed |
| 6 | Has a user guide page? | No | `docs/guide/` describes the config, which is unchanged |
| 7 | Wire format changed? | No | `rtm_protocol` is a kernel field, not a Ze wire format |
| 8 | Plugin SDK/protocol changed? | No | `iface.Backend` is internal; `pkg/plugin` and `pkg/ze` are untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC governs rtm_protocol |
| 10 | Test infrastructure changed? | No | The new file is picked up by the existing `ZE_QEMU_INTEGRATION_PKGS` glob in `mk/test-integration.mk` |
| 11 | Affects daemon comparison? | No | Nothing in `docs/comparison.md` names route ownership |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` describes the `rtproto.IsZe` filter in routewatch, which is unchanged by construction (AC-6) |
| 13 | Route metadata keys added/changed? | No | No metadata key |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features.md` names `rtproto 250-252` as the Ze-owned filter, which stays exact because `Iface` is outside `IsZe`. `docs/architecture/core-design.md` names `rtproto.IsZe()`, unchanged |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example shows a route protocol |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- the vocabulary and the entry points
   - Tests: `TestIfaceProtocolIsNamedButNotZeOwned`, `TestAnyIsTheUnsetProtocol`
   - Files: `internal/core/rtproto/rtproto.go`, `internal/component/iface/backend.go`, `dispatch.go`, and every implementation and fake
   - Verify: the entry points carry the protocol end to end and the tree compiles. The kernel behavior is still the old one, so the integration tests fail
2. **Phase: The kernel lever** -- stamp the add, match the delete, report the orphan
   - Tests: the five `TestRouteIntegration_*` in `route_integration_linux_test.go`
   - Files: `internal/plugins/iface/netlink/manage_linux.go`
   - Verify: the five tests pass, and each goes red when the stamp it depends on is removed
3. **Phase: The callers** -- every producer names its protocol
   - Tests: the `internal/component/iface` unit tests, whose `routeCall` fixture now carries the protocol
   - Files: `register.go`, `pppoe_client.go`, `dhcp_v4_linux.go`, `ppp/ncp.go`, `ppp/ipv6_service.go`
   - Verify: `cleanupStaleIPv6DefaultRoutes` names `rtproto.Any` and every other caller names `rtproto.Iface`

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named test |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Exactly one caller names `rtproto.Any`, and it is `cleanupStaleIPv6DefaultRoutes`. Every other `AddRoute` / `RemoveRoute` names `rtproto.Iface` |
| Correctness | The ESRCH warning fires for a stamped delete and stays silent for a blind one, so a double-remove on the teardown path does not shout |
| Naming | `Any` reads as a match rule and `Iface` reads as a producer. Neither is reachable by leaving an argument out |
| Data flow | `IsZe` and `Name` no longer share one map, and `IsZe`'s membership is unchanged: 250, 251, 252 |
| Rule: `ai/rules/evidence.md` (guards) | The guard fails closed on both sides: an install refuses `rtproto.Any`, a delete cannot be unstamped by accident, and a miss is reported rather than swallowed |
| Rule: `ai/rules/platform-linux.md` | The proof runs on a real kernel under `make ze-qemu-integration-test`, and every test skips rather than fails without CAP_NET_ADMIN |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each test was re-run with the stamp reverted and the ones that claim the fix went red |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `rtproto.Proto` with `Any` and `Iface` | `make ze-test-pkg PKG=./internal/core/rtproto` |
| A stamped delete in the netlink backend | `grep -c "netlink.RouteProtocol(proto)" internal/plugins/iface/netlink/manage_linux.go` returns 2 (add and remove) |
| An install can never be unowned | `TestAddRouteRejectsTheBlindProtocol` |
| Exactly one blind caller | `grep -rn "rtproto.Any" internal/component internal/plugins --include=*.go` names `cleanupStaleIPv6DefaultRoutes` only, tests aside |
| The kernel proof | `make ze-qemu-integration-test` runs `TestRouteIntegration_*` |
| The orphan is observable | `TestRouteIntegration_LegacyBootRouteIsReported` asserts a WARN naming the interface, destination and gateway |
| The WARN reaches an operator, not a discard handler | `TestRemoveRouteMissWarnsThroughTheProductionLogger` -- the record must arrive in `slogutil.GlobalLogRing()` under component `iface.netlink` |
| The operator reads the owner in `show route` | `make ze-test-pkg PKG=./internal/plugins/iface/netlink RUN=TestProtocolNameNamesEveryProducer`, and `test/plugin/iface-route-protocol-name.ci` under `make ze-qemu-needs-linux-test` |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The protocol is not operator input. It comes from a typed constant in Ze's own code and reaches the kernel as one octet, which cannot overflow the field |
| Privilege | No new privilege. The backend already held CAP_NET_ADMIN for route installs |
| Fail-open | The change makes the delete NARROWER, so the failure mode moves from deleting a foreign route to leaving one of Ze's own. AC-5's WARN is what stops that failure being silent |
| Denial of service | The WARN fires once per delete that misses, which is bounded by the caller's own event rate: a lease expiry, or a link event |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- The kernel refuses a second route with the same destination, gateway, link and
  metric: `ip route add` answers EEXIST. So "a static route and a DHCP route that
  share the four-tuple" is never two kernel objects. It is ONE object, owned by
  whoever wrote it last, and the delete decides whether that owner keeps it.
- A wildcard spelled 0 is a wildcard every caller reaches by accident. Giving it
  a name (`rtproto.Any`) is what turns "I forgot" into "I asked".
- **A report that asks the kernel a question is a per-event cost, and it must be
  paid where the events are not routine.** `netlink.RouteList` dumps every route
  of the family and filters by interface in userspace, so the ESRCH readback
  costs a whole FIB on a box that redistributes one. The routine miss it was
  paying for came from a caller that removed a route it had already moved:
  `handleLinkUpdate` re-emits up or down on every RTM_NEWLINK, and the link
  handlers had no memory of which metric their route sat at. The fix belongs at
  the caller, in the state it was missing, not in a cheaper question.
- `IsZe` and `Name` looked like one question and were two: membership of a set
  whose members `routewatch` MUTES, and a display name. One map served both, so
  naming a protocol silenced it. Splitting them costs three lines.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A typed `rtproto.Proto` parameter on both methods | a field on `RouteInfo`; a per-backend default; an untyped `int` | `RouteInfo` is a RESULT type, returned by `ListRoutes`, and never travels into a delete. A per-backend default cannot express the one caller that must stay blind. An untyped int makes 0 reachable by omission, which is the trap this spec exists to close |
| `Any` named, and equal to the kernel's own RTPROT_UNSPEC | a separate boolean `blind` parameter | Two parameters that can disagree is a state nobody wants to reason about. One value, one meaning, and it matches what the kernel already calls unspecified |
| `IsZe` as an explicit switch, `Name` reading a wider map | adding `Iface` to `zeNames` and giving `routewatch` an exception list | An exception list in `routewatch` puts one producer's spelling in a core package (`ai/rules/plugins.md`), and it grows with every producer |
| WARN on a stamped ESRCH, success still returned | returning the error; a metric; silence | The caller's intent IS satisfied, so an error would make every teardown path handle a condition it cannot act on. Silence is what AC-5 refuses |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- **The add side asks no ownership question; it is now kept off the contested
  metric instead.** `RouteReplace` matches on destination, metric and table and
  takes no protocol, so an interface-layer add at a key an operator's route
  occupies overwrites it and re-stamps it 253. Since 2026-08-11 a learned default
  route is installed at metric 254 (`defaultLearnedRouteMetric`,
  `internal/component/iface/config.go`), which is not where a static default
  lands, so the two are two kernel routes and the operator's forwards. Two things
  remain: an operator who writes `route-priority 0` opts back into the collision,
  and a route an operator installs AT 254 is still taken over. The ownership
  question itself needs the administrative distance to reach the kernel, which is
  `spec-admin-distance-reaches-the-kernel`.
- **Legacy routes are reported, not swept.** A route from a version that stamped
  nothing carries RTPROT_BOOT and survives every stamped delete. AC-5 asks for
  the orphan to be observable, which the WARN gives. A start-time sweep of
  RTPROT_BOOT default routes on Ze-managed interfaces is separate work.
- **A failed RA restore leaves a window with no IPv6 default route, and the
  retry is config-driven.** `restoreAcceptRaDefrtr`
  (`internal/component/iface/register.go`) emits `accept_ra_defrtr=1` first and
  returns on an Emit error, keeping the interface `suppressed` and carrying ze's
  `::/0`, which forwards. If a router-lost arrives in that state,
  `handleRouterLost` removes that `::/0`, and `handleRouterDiscovered` installs
  no replacement because it reads `priorities[name]`, which no longer publishes
  the interface. The kernel is still suppressed, so neither side holds a default
  route. The retry rides on `suppressRAForConfig`, called from exactly two
  places, both config-event driven (`OnConfigApply` and the reload path), so on
  a box whose config never changes again the window never closes. Closing it
  needs a periodic reconcile, which is separate work.
- **The VPP interface backend still refuses route programming.** Its `AddRoute`
  and `RemoveRoute` return "not supported" as before. Only the signature moved.
- **The ESRCH readback still costs one route dump of the family, and it is paid
  on a delete the kernel refused.** `netlink.RouteList` asks for RTM_GETROUTE with
  NLM_F_DUMP and filters by output interface in userspace, so on a box with a
  redistributed FIB the read is the whole table. Nothing in the vendored library
  asks a narrower question without a second handle carrying
  `NETLINK_GET_STRICT_CHK`, which is a kernel-version branch this defect does not
  need: the routine miss no longer reaches the read. What reaches it is a kernel
  that disagrees with ze about where a route is, and that is worth one dump.

## Mistake Log

### Wrong Assumptions
| ID | Assumption | What was actually true | Where it showed |
|----|-----------|------------------------|-----------------|
| A-5 | A static route and an interface-layer route sharing destination, gateway, link and metric coexist in the kernel, and the blind delete picks the wrong one | The kernel holds ONE route for that key. `ip route add` answers EEXIST and `ip route replace` overwrites the protocol in place. The blind delete removes that single object whoever owns it, which is the same symptom by a different mechanism | measured in an ephemeral netns while designing the AC-1 test. The AC and its test read the same way afterwards, so no design changed. A-5 STAYS BROKEN after the 2026-08-11 metric work: `ip route add` still answers EEXIST for that key, and the 254 default only stops the DEFAULT path from reaching the condition |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Journal rows written under `plan/journal/<class>.md`. No learned summary:
      the 2026-08-10 owner directive bans saving a lesson beside the commit, and
      `plan/learned/` holds no `NNN-*.md` files any more
- [ ] **Commit A:** code + tests + docs + spec + journal rows
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## State at 2026-08-10 (session paused, NOT closed)

Implemented and proven on a real kernel. Not reviewed, so not closed.

**What landed.** `rtproto.Proto` with two named values, `Any = 0` (the kernel's
own wildcard) and `Iface = 253`. Both `AddRoute` and `RemoveRoute` take a trailing
typed `proto`, and `cleanupStaleIPv6DefaultRoutes` is the ONLY production caller
naming `rtproto.Any` — the blind match is now something a caller must ask for by
name rather than reach by omission. `zeNames` is gone: `IsZe` is an explicit
switch that excludes `Iface`, so `routewatch` keeps delivering DHCP, RA and PPPoE
route events, while `Name` renders 253 from a separate map. A stamped delete that
returns `ESRCH` now WARNs with interface, destination, gateway, metric and
protocol, so an upgrade orphan is visible instead of swallowed.

**One guard added beyond the brief, and it was right.** `AddRoute` refuses
`rtproto.Any` before any netlink call. Installing under a match-anything rule
would put an unowned route in the kernel, which is this defect returning by the
other door.

**Evidence.** `route_integration_linux_test.go`, five cases plus the install
guard, all PASS under QEMU on a real kernel as root. Discrimination is exact:
without the delete stamp AC-1, AC-2 and AC-5 fail; without the add stamp AC-4 and
AC-2 fail with `installed route protocol = 3`, which also confirms routes really
did land as `RTPROT_BOOT` rather than `RTPROT_UNSPEC` as the Task claimed.

**What blocked closure, and how each was cleared (2026-08-11)**

| # | Owed | Cleared by |
|---|------|------------|
| 1 | No independent Review Gate pass, no artifact | Six rounds, clean at round 6. See `## Review Gate` |
| 2 | No `## Implementation Audit`, `## Goal Validation` or `## Review Gate` section | All three appended, plus `## Implementation Summary`, `## Deferrals Resolved` and `## Pre-Commit Verification` |
| 3 | The owner decision below | Settled by Thomas on 2026-08-11 and implemented |

**Settled by Thomas on 2026-08-11, and implemented.** Learned default routes
take their metric from a documented default instead of 0: `route-priority`
defaults to 254 on a unit and on a `pppoe-client`, which is where Cisco's DHCP
client ranks the same route and which mirrors the order `rib admin-distance`
uses. `PPPoEClientConfig` gained the field that made the setting unreachable for
PPPoE. Writing the leaf is still what hands ze the IPv6 RA default routes, so the
non-zero default did not change who owns them. Evidence:
`internal/component/iface/route_metric_test.go` (four tests, executed) and
`test/plugin/iface-learned-route-metric.ci` (needs-linux, and executed on a real
kernel on 2026-08-11: PASS 3.7s).

**The superseded record: the ADD side still takes ownership.**
`RouteReplace` matches on destination, metric and table and takes no protocol, so
an iface add at a key an operator's static route occupies overwrites it and
re-stamps it 253. Measured in a netns: `ip route replace default via 192.0.2.2
metric 5 proto 253` turned a `proto static` route into `proto 253`. Refusing to
install means no DHCP default route at all, so this is a policy call, not a bug
fix. A-5 is recorded BROKEN for the same reason: a static and an iface route
sharing the four-tuple cannot coexist — `ip route add` answers EEXIST, so it is
one kernel object owned by the last writer.

## Implementation Summary

### What Was Implemented
- `internal/core/rtproto`: a typed `Proto` with `Any = 0` (the kernel's own
  RTPROT_UNSPEC) and `Iface = 253`. `IsZe` became an explicit switch over 250,
  251 and 252; `Name` reads a separate `names` map, so naming a protocol no
  longer mutes it in `routewatch`.
- `AddRoute` and `RemoveRoute` take a trailing `rtproto.Proto` on the `Backend`
  interface and on all three implementations. `(*netlinkBackend).AddRoute`
  stamps `netlink.Route.Protocol` and refuses `rtproto.Any`;
  `(*netlinkBackend).RemoveRoute` matches the delete on it.
  `cleanupStaleIPv6DefaultRoutes` is the only production caller that names
  `Any`, and it has to type it out.
- A stamped delete the kernel answers ESRCH is reported by
  `reportRemoveRouteMiss`: a route surviving under another protocol is a WARN
  naming that protocol, a failed table read is a WARN saying the read failed,
  and nothing there at all is DEBUG.
- Learned default routes now land at `defaultLearnedRouteMetric` (254), from a
  `route-priority` default on a unit and on a `pppoe-client`, so a learned
  default no longer shares the kernel key an operator's static default occupies.
- The link handlers carry `routeMetricState`, so a repeated link event reaches
  no netlink call, and a failed `AddRoute` after a landed `RemoveRoute` writes
  `routeMetricUnknown` back rather than leaving the map claiming a metric the
  kernel does not hold.

### Bugs Found/Fixed
- `loggerPtr` in `internal/plugins/iface/netlink` was fixed at `DiscardLogger`,
  so all 14 log sites in the package went nowhere, AC-5's WARN among them. Fixed
  by the `slogutil.LazyLogger` seam. `plan/journal/unwired-feature.md`.
- A SLAAC-only unit that wrote `route-priority` was suppressed and got no `::/0`
  at all: the suppression gate read the config and the installer read the
  running DHCP clients. One derivation, `writtenRoutePriorities`, now serves
  both. `TestSLAACUnitWithRoutePriorityKeepsAnIPv6DefaultRoute`.
- The metric-state dedupe guard swallowed the recovery event after a failed
  `AddRoute`. `TestAFailedMetricMoveLeavesTheRouteRecoverable`.

### Documentation Updates
- `docs/features/interfaces.md`: the interface layer's routes carry `proto 253`
  (`ze-iface`), anchored at `internal/plugins/iface/netlink/manage_linux.go`.
- `docs/guide/configuration.md`: the `route-priority` default of 254, what the
  metric decides about ownership, and the IPv6 suppression rule. Anchored at
  `internal/component/iface/yang/ze-iface-conf.yang`,
  `internal/component/iface/config.go` and `register.go`.
- `make ze-doc-test` was NOT run in this closure session: the main thread
  scoped this phase to no test suite, and the doc edits are prose inside
  paragraphs whose source anchors already exist and still name their producers.

### Deviations from Plan
- 17 files beyond the plan, each listed in "Files from Plan". The two largest
  groups are the 2026-08-11 metric decision (which the plan predates) and the
  `loggerPtr` seam (without which AC-5 was unobservable).
- The plan said no CLI surface reports the protocol. It was wrong: `show route`
  and the web IP Routes page both print it, so two `.ci` files were added.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Stamp DHCP-installed and PPPoE-installed routes with their own protocol value in `internal/core/rtproto` | Done | `rtproto.Iface` (`internal/core/rtproto/rtproto.go`); `(*netlinkBackend).AddRoute` (`internal/plugins/iface/netlink/manage_linux.go`) puts it in `netlink.Route.Protocol` | Every interface-layer producer names it: `dhcp_v4_linux.go`, `pppoe_client.go`, `register.go`, `ppp/ncp.go`, `ppp/ipv6_service.go` |
| Make `RemoveRoute` match on that protocol so it can only remove routes it installed | Done | `(*netlinkBackend).RemoveRoute` (same file) stamps the delete; `rtproto.Any` is the one way to a wildcard and has to be typed out | `cleanupStaleIPv6DefaultRoutes` (`internal/component/iface/register.go`) is the only production caller naming `Any` |
| The blind match stays available where the kernel, not Ze, installed the route | Done | `cleanupStaleIPv6DefaultRoutes`, `rtproto.Any` | AC-3 |
| The new value must not silence `routewatch` | Done | `IsZe` is an explicit switch over 250, 251, 252 (`rtproto.go`); `Name` reads a separate map | AC-6 |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `(*netlinkBackend).RemoveRoute` (`internal/plugins/iface/netlink/manage_linux.go`) -- `TestRouteIntegration_StampedRemoveLeavesStaticRoute` | Ran on a real kernel. Reverting the delete stamp turns it red |
| AC-2 | Done | `handleLinkDown` / `handleLinkUp` (`internal/component/iface/register.go`) -- `TestRouteIntegration_LinkBounceKeepsStaticRoute`, `TestHandleLinkDownWithRoutePriority`, `TestHandleLinkUpWithRoutePriority`, `TestLearnedMetricSurvivesTheLinkBounce` | The restore step re-stamps a route that shares the key: the add side, recorded in Known Limitations |
| AC-3 | Done | `cleanupStaleIPv6DefaultRoutes` (`internal/component/iface/register.go`) -- `TestStaleKernelRouteCleanup`, `TestRouteIntegration_BlindRemoveDeletesKernelRoute` | The blind match survived the change and is now asked for by name |
| AC-4 | Done | `(*netlinkBackend).AddRoute` and `RemoveRoute` -- `TestRouteIntegration_AddStampsAndRemoveDeletesOwnRoute`, `TestAddRouteRejectsTheBlindProtocol` | The add refuses `rtproto.Any`, so no unowned route reaches the kernel |
| AC-5 | Done | `reportRemoveRouteMiss` and `logRemoveRouteMiss` (`internal/plugins/iface/netlink/manage_linux.go`) -- `TestRouteIntegration_LegacyBootRouteIsReported`, `TestRemoveRouteMissWarnsThroughTheProductionLogger`, `TestRemoveRouteMissNamesTheProtocolThatHoldsTheRoute`, `TestRemoveRouteMissReportsWhatTheTableReadEstablished` | Round 4 changed the outcome from "assumed orphan" to "orphan the kernel confirmed": the reporter lists the table back and names the protocol holding the route. A miss with nothing there is DEBUG. Round 5 made the third outcome its own report: a table read that FAILED is a WARN saying so, never an absence, so the orphan report does not disappear when the dump does |
| AC-6 | Done | `IsZe` (`internal/core/rtproto/rtproto.go`), read by `(*Watcher).deliver` (`internal/core/routewatch/routewatch.go`) -- `TestIfaceProtocolIsNamedButNotZeOwned` | `Iface` is outside the muted set, so DHCP, RA and PPPoE route events still reach subscribers |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestIfaceProtocolIsNamedButNotZeOwned`, `TestAnyIsTheUnsetProtocol`, `TestRouteProtocolsAreDistinct` | PASS | `internal/core/rtproto/rtproto_test.go` | |
| `TestAddRouteRejectsTheBlindProtocol`, `TestRemoveRouteMissWarnsThroughTheProductionLogger` | PASS | `internal/plugins/iface/netlink/manage_linux_test.go` | |
| `TestRemoveRouteMissNamesTheProtocolThatHoldsTheRoute` | PASS | same file | Added in round 4 for ISSUE 2 |
| `TestProtocolNameNamesEveryProducer` | PASS | `internal/plugins/iface/netlink/route_linux_test.go` | Round 3 NOTE 7: the name overclaims, the cases it runs are correct |
| The nine `internal/component/iface` route-call tests, and `TestStaleKernelRouteCleanup` | PASS | `internal/component/iface/config_test.go` | |
| `TestWrittenRoutePrioritiesMultiUnit`, `TestWrittenRoutePrioritiesNoMatch` | PASS | same file | Round 4: they replace `TestRoutePriorityForInterface*`, whose producer no longer exists |
| `TestSLAACUnitWithRoutePriorityKeepsAnIPv6DefaultRoute` | PASS | `internal/component/iface/route_metric_test.go` | Round 4, the BLOCKER's test |
| The five metric tests | PASS | same file | |
| `TestRouteIntegration_StampedRemoveLeavesStaticRoute`, `_LinkBounceKeepsStaticRoute`, `_BlindRemoveDeletesKernelRoute`, `_AddStampsAndRemoveDeletesOwnRoute`, `_LegacyBootRouteIsReported` | PASS | `internal/plugins/iface/netlink/route_integration_linux_test.go` | On a real kernel as root. The `held-by=boot` assertion round 4 added to `_LegacyBootRouteIsReported` ran in the 2026-08-11 integration pass |
| `TestRouteIntegration_RepeatedRemoveDoesNotWarn` | PASS | same file | Written in round 4, first executed in the 2026-08-11 integration pass |
| `test/plugin/iface-route-protocol-name.ci` | PASS, 2.9s | `test/plugin/` | On a real kernel |
| `test/plugin/iface-learned-route-metric.ci` | PASS, 3.7s | `test/plugin/` | On a real kernel, first execution of this file |

**What the QEMU evidence is, exactly.** One command produced both `.ci` records:
`make ze-qemu-needs-linux-test` on 2026-08-11, run with `ZE_QEMU_LINUX_ONLY=1`
through `scripts/evidence/qemu-run.py`, log
`tmp/session/2026-08-10-640fa955-f03a-45e8-a58f-4b367f5859e6/scratch/qemu-iface-ci.log`.
Its plugin phase reported `2.9s PASS 255 iface-route-protocol-name` and
`3.7s PASS 252 iface-learned-route-metric` on the VM's real kernel (lines 348 and
349). That same run did NOT finish: it was cut short later, in the in-VM unit
phase that follows the `.ci` suites (line 3550). So the two `.ci` results are
genuine kernel runs and the TARGET is still unproven end to end. The two
statements describe one run, at two different points in it. **No
`make ze-qemu-needs-linux-test` run has completed end to end on this machine.**

**The integration evidence is a second, separate run.**
`make ze-qemu-integration-test` on 2026-08-11, log
`tmp/session/2026-08-10-640fa955-f03a-45e8-a58f-4b367f5859e6/scratch/qemu-integration2.log`,
reported `ok github.com/ze-software/ze/internal/plugins/iface/netlink 3.226s`
(line 313). That verdict covers every test in the package, so
`TestRouteIntegration_RepeatedRemoveDoesNotWarn` and the `held-by=boot`
assertion both executed. The log is not verbose, so the record is the package
verdict rather than a per-test line; 3.226s is also what tells the netns cases
apart from a run that skipped them, which costs milliseconds. The same run
carries failures in other packages (`internal/component/ike/dataplane`,
`internal/plugins/traffic/netlink`, and a `cmd/ze/hub` cut off by the target's
own `-timeout 120s`). None is in this spec's diff.

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Every file in "Files to Modify" | Modified | 17 of 17, `docs/features/interfaces.md` included |
| `internal/plugins/iface/netlink/route_integration_linux_test.go`, `manage_linux_test.go` | Created | Both from "Files to Create" |
| `internal/plugins/iface/netlink/route_linux_test.go` | Created, not in the plan | `protocolName` is named in Integration Points but the file list never carried its test |
| `internal/component/iface/route_metric_test.go` | Created, not in the plan | The 2026-08-11 metric decision, which the plan predates |
| `test/plugin/iface-route-protocol-name.ci`, `test/plugin/iface-learned-route-metric.ci` | Created, not in the plan | Added by the 2026-08-11 correction: `show route` prints the value, so it owes a functional test |
| `internal/component/iface/config.go`, `iface.go`, `yang/ze-iface-conf.yang`, `docs/guide/configuration.md` | Modified, not in the plan | `defaultLearnedRouteMetric` and the two `route-priority` leaves: the owner decision of 2026-08-11 |
| `internal/plugins/iface/netlink/addr_primary.go`, `doctor_linux.go`, `ifacenetlink.go`, `macvlan_linux.go`, `monitor_linux.go`, `show_linux.go`, `tunnel_linux.go`, `wireguard_linux.go` | Modified, not in the plan | The `loggerPtr` to `slogutil.LazyLogger` seam. `loggerPtr` was fixed at `DiscardLogger`, so all 14 log sites in the package went nowhere, AC-5's WARN among them. Recorded in `plan/journal/unwired-feature.md` |
| `internal/plugins/iface/netlink/route_linux.go` | Modified, named in Integration Points but not in the file list | `protocolName` renders 253 |

### Audit Summary

- **Total items:** 6 acceptance criteria, 4 Task requirements, every test in the
  TDD Test Plan above (3 of them added in round 5), 36 files (19 the plan named:
  17 to modify plus 2 to create; 17 beyond it)
- **Done:** every AC, every Task requirement
- **Partial:** none
- **Skipped:** none
- **Changed:** 17 files beyond the plan (each one listed in the table above, and
  the same 36 the review artifact hashes); AC-5's mechanism changed in round 4
  from an assumed cause to one read from the kernel, and round 5 made the read's
  FAILURE a report of its own
- **Not executed:** nothing in this spec's test set.
  `TestRouteIntegration_RepeatedRemoveDoesNotWarn` and the assertion round 4
  added to `TestRouteIntegration_LegacyBootRouteIsReported` both ran in the
  2026-08-11 `make ze-qemu-integration-test` pass. What is still unproven is a
  TARGET, not a test: no `make ze-qemu-needs-linux-test` run has completed end
  to end on this machine, though both of this spec's `.ci` files passed on a
  real kernel inside the run that was cut short

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Stamp DHCP-installed and PPPoE-installed routes with their own protocol value | functional, on a real kernel | `TestRouteIntegration_AddStampsAndRemoveDeletesOwnRoute` reads the route back and asserts `protocol == 253`. With the add stamp reverted it reports `installed route protocol = 3`, which is also how A-2 was confirmed: the routes landed as `RTPROT_BOOT`, not `RTPROT_UNSPEC` as the Task said |
| `RemoveRoute` matches on that protocol, so it can only remove routes it installed | functional, on a real kernel | `TestRouteIntegration_StampedRemoveLeavesStaticRoute`: a `proto static` default sharing gateway, link and metric survives an interface-layer remove. Reverted stamp, red: `static default route was deleted by an interface-layer remove` |
| An operator can see who owns a route | functional `.ci` | `test/plugin/iface-route-protocol-name.ci` PASS 2.9s: `show route` prints `ze-iface` for a route stamped 253 and `boot` for one still carrying 3 |
| A learned default route does not take over an operator's static default | functional `.ci` | `test/plugin/iface-learned-route-metric.ci` PASS 3.7s, first-ever execution: the static route and the learned route coexist, the static one keeps its gateway and its `proto static`, and it forwards |
| Writing `route-priority` hands Ze the IPv6 default route rather than losing it | unit, and the producer is one map | `TestSLAACUnitWithRoutePriorityKeepsAnIPv6DefaultRoute`: a SLAAC-only unit with `route-priority 100` is suppressed AND gets `::/0` at 100. With the two derivations put back, red at `ze suppressed the kernel's RA default route, so it MUST install one of its own` |
| An upgrade orphan is observable | unit, plus functional on a real kernel | `TestRouteIntegration_LegacyBootRouteIsReported` (WARN naming interface, destination and gateway) and `TestRemoveRouteMissWarnsThroughTheProductionLogger` (the record reaches `slogutil.GlobalLogRing()` under `iface.netlink`, which no handler a test installs can satisfy) |
| The whole integration set runs against a real kernel, not a fake backend | integration, in QEMU | `make ze-qemu-integration-test` on 2026-08-11 reported `ok github.com/ze-software/ze/internal/plugins/iface/netlink 3.226s` (log `tmp/session/2026-08-10-640fa955-f03a-45e8-a58f-4b367f5859e6/scratch/qemu-integration2.log`, line 313). The package verdict covers all six `TestRouteIntegration_*` cases, including `_RepeatedRemoveDoesNotWarn` and the `held-by=boot` assertion, both of which had never executed before. The run's other failures are foreign to this diff |
| Interop | N-A | Nothing crosses a wire. `rtm_protocol` is a local kernel field and the peer of this exchange is the Linux kernel, which the integration tests run against |

## Deferrals Resolved

The metadata table names no deferral shard, and
`plan/deferrals/fixit-route-removal-protocol-blind.md` does not exist, so there
is no row to account for and nothing to `git rm`.

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none -- no shard was opened for this spec | n/a | `ls plan/deferrals/fixit-route-removal-protocol-blind.md` reports no such file. The two findings left unfixed in the Review Gate are homed elsewhere: ISSUE 4 is discoverability in `mk/test-integration.mk`, ISSUE 5 is recorded in `plan/journal/unwired-feature.md`, and the add-side ownership question is `spec-admin-distance-reaches-the-kernel` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-route-removal-protocol-blind-640fa955-f03a-45e8-a58f-4b367f5859e6.md` |
| `review_gate.py check` | OK, exit 0: `verdict=clean`, hashes match the 44 files the artifact lists |
| Rounds | 6 recorded, clean at round 6. Round 1 and 2 found the wrong `rtProtoNames` keys, shared kernel state in the `.ci`, and a swallowed `parseUnits` error; round 3 found the BLOCKER below plus 4 ISSUE and 4 NOTE; round 4 found 0 BLOCKER and the 2 ISSUE that round 5 fixed, both in round 4's own readback; round 5 found a PRODUCT defect in its own fix (the four link handlers did not write `metricState` back on a failed `AddRoute`, so the dedupe guard made the next opposite event a no-op and the route stayed gone); round 6 verified that fix, the fifth site in `handleDHCPLeaseEvent` and the `restoreAcceptRaDefrtr` ordering, and reported 0 BLOCKER, 0 ISSUE, 6 NOTE |
| Reviewer lenses used | protocol-map correctness against the vendored constants and `/usr/include/linux/rtnetlink.h`; kernel-state isolation in the `.ci`; guard symmetry between add and delete; claim-versus-producer on every comment the diff added |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The comment claimed the suppression gate agrees with `routePriorityForInterface`, and it did not: one read the config, the other read the running DHCP clients. A SLAAC-only unit with `route-priority` lost its IPv6 default route entirely, and `docs/guide/configuration.md` repeated the claim to operators | `internal/component/iface/register.go`, `docs/guide/configuration.md` | One derivation, `writtenRoutePriorities`, published by `suppressRAForConfig` and read by `handleRouterDiscovered`. `routePriorityForInterface` is deleted. The doc now states that DHCP is no part of the test. `TestSLAACUnitWithRoutePriorityKeepsAnIPv6DefaultRoute` |
| 2 | ISSUE | The ESRCH message blamed an upgrade orphan on every stamped miss, contradicting the comment three lines above it, and the benign case is routine: `(*monitor).handleLinkUpdate` re-emits a down event on every RTM_NEWLINK | `internal/plugins/iface/netlink/manage_linux.go` | `reportRemoveRouteMiss` asks the kernel which cause it is. A surviving route under another protocol is a WARN naming that protocol (`held-by`); nothing there at all is DEBUG. `TestRemoveRouteMissNamesTheProtocolThatHoldsTheRoute`, `TestRouteIntegration_RepeatedRemoveDoesNotWarn` |
| 3 | ISSUE | `routeNetNSName` kept the last 8 characters, so two tests derived `zert_ticRoute`, and a leftover `/var/run/netns` entry made `withRouteNetNS` skip, dropping AC-1 or AC-2 with no failure | `internal/plugins/iface/netlink/route_integration_linux_test.go` | The name carries a hash of the full test name. `netns.NewNamed` failing for anything other than EPERM or EACCES is now `t.Fatalf`, so an EEXIST from a leftover namespace fails the run instead of skipping it |
| 4 | ISSUE | `ze-integration-iface-test` does not name `./internal/plugins/iface/netlink/...` | `mk/test-integration.mk` | Not a defect: `ZE_QEMU_INTEGRATION_PKGS` globs the build tag, which is why the tests ran, and the 2026-08-11 pass reported the package `ok`. The row is discoverability of the package name in one variable |
| 5 | ISSUE | `teardownNCPResources` passes `rtproto.Iface` to a `RemoveRoute` whose gateway is `""`, which the backend rejects before netlink, so the PPP peer route is never removed | `internal/component/l2tp/ppp/ncp.go` | Homed as a journal row: `plan/journal/unwired-feature.md` carries it from 2026-08-10 with the same diagnosis. It does not block this spec's goal, so it takes the route `ai/rules/completion.md` gives a defect met while doing something else |
| 6-9 | NOTE | `int(rp)` on a 32-bit build, `TestProtocolNameNamesEveryProducer` overclaiming, the `.ci` headers naming the wrong parallelism variable, and no test pinning the absent-leaf invariant at the config-tree boundary | various | Not fixed. NOTEs do not block (`ai/rules/planning.md`) |
| 10 | ISSUE (round 4 review) | `routeUnderAnotherProtocol` returned `(0, false)` for a survivor that is absent AND for a listing that FAILED, so AC-5's orphan WARN disappeared exactly when the dump failed: ENOBUFS on a large FIB, or `ErrDumpInterrupted`, which makes `RouteListFiltered` drop the routes it read | `internal/plugins/iface/netlink/manage_linux.go` | The readback returns three states. A failed read is a WARN that says the read failed; the DEBUG names the key it checked instead of declaring the route gone. `TestRemoveRouteMissReportsWhatTheTableReadEstablished` |
| 11 | ISSUE (round 4 review) | The readback cost a full route dump per link event, under `dhcpMu`. `handleLinkUpdate` re-emits up or down on every RTM_NEWLINK, and the link handlers removed a route they had already moved, so a routine miss reached the dump on every link attribute change | `internal/component/iface/register.go` | The handlers now carry `routeMetricState`: which metric ze last put that route at. A repeated event reaches no netlink call at all, so the readback is off the per-event path. `TestRepeatedLinkEventsMoveTheRouteOnce`, `TestRepeatedLinkEventsMoveTheIPv6RouteOnce` |

| 12 | BLOCKER (round 5 review) | The `metricState` dedupe guard was written before the `AddRoute` error path: after a `RemoveRoute` had landed, a failed add returned without writing the entry back, so the map still claimed the old metric and the next opposite event deduped itself away. The route stayed gone | `internal/component/iface/register.go`, `handleLinkDown` / `handleLinkUp` / `handleLinkDownIPv6` / `handleLinkUpIPv6` | All four handlers set `entry.metricState = routeMetricUnknown` and write the entry back before returning on the add error. `handleDHCPLeaseEvent` records `routeMetricUnknown` for the same reason. `TestAFailedMetricMoveLeavesTheRouteRecoverable`, `TestAFailedIPv6MetricMoveLeavesTheRouteRecoverable` |

**Round 6 is clean.** 0 BLOCKER, 0 ISSUE, 6 NOTE, over the files that changed
since round 5 plus the QEMU integration evidence and closure readiness. NOTE 2
and NOTE 3 are answered in this closure: the residual RA gap is now a Known
Limitation, and `docs/guide/configuration.md` names the administrative distance
the code comment names. The other four NOTEs are recorded and do not block
(`ai/rules/planning.md`).

## Pre-Commit Verification

Re-checked on 2026-08-11 in the closure session. The three package runs below
are `make ze-test-pkg` with `-race`, on the working tree that this commit
carries.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/iface/netlink/route_integration_linux_test.go` | Yes | `ls -l` reports 15781 bytes |
| `internal/plugins/iface/netlink/manage_linux_test.go` | Yes | `ls -l` reports 7668 bytes |
| `internal/plugins/iface/netlink/route_linux_test.go` | Yes | `ls -l` reports 2335 bytes (created beyond the plan) |
| `internal/component/iface/route_metric_test.go` | Yes | `ls -l` reports 30938 bytes (created beyond the plan) |
| `test/plugin/iface-route-protocol-name.ci` | Yes | `ls -l` reports 8215 bytes |
| `test/plugin/iface-learned-route-metric.ci` | Yes | `ls -l` reports 10789 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A stamped remove leaves an operator's route alone | `TestRouteIntegration_StampedRemoveLeavesStaticRoute` at `route_integration_linux_test.go:215`, in the package the 2026-08-11 integration run reported `ok ... 3.226s` |
| AC-2 | A link bounce keeps the static default route | `make ze-test-pkg PKG=./internal/component/iface RUN='...TestHandleLinkDownWithRoutePriority\|TestHandleLinkUpWithRoutePriority...'` -> `ok github.com/ze-software/ze/internal/component/iface 1.336s`; the kernel half is `TestRouteIntegration_LinkBounceKeepsStaticRoute` at `:361` |
| AC-3 | The blind match still clears the kernel's own RA default route | same iface run covered `TestStaleKernelRouteCleanup`; `TestRouteIntegration_BlindRemoveDeletesKernelRoute` at `:276` |
| AC-4 | Ze still removes the route it installed, and refuses to install blind | `make ze-test-pkg PKG=./internal/plugins/iface/netlink RUN='TestAddRouteRejectsTheBlindProtocol\|...'` -> `ok ... 1.112s`; `TestRouteIntegration_AddStampsAndRemoveDeletesOwnRoute` at `:240` |
| AC-5 | An upgrade orphan is observable | same netlink run covered `TestRemoveRouteMissWarnsThroughTheProductionLogger`, `TestRemoveRouteMissNamesTheProtocolThatHoldsTheRoute` and `TestRemoveRouteMissReportsWhatTheTableReadEstablished`; `TestRouteIntegration_LegacyBootRouteIsReported` at `:297`, `held-by=boot` included, ran in the integration pass |
| AC-6 | Naming 253 does not mute `routewatch` | `make ze-test-pkg PKG=./internal/core/rtproto RUN='TestIfaceProtocolIsNamedButNotZeOwned\|TestAnyIsTheUnsetProtocol\|TestRouteProtocolsAreDistinct'` -> `ok ... 1.027s` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| DHCPv4 lease expiry -> stamped RTM_DELROUTE | `test/plugin/iface-route-protocol-name.ci` | Read: it installs a route stamped 253, asserts `show route` reports `protocol == 'ze-iface'`, and keeps a `proto boot` control route that must still read `boot`. The kernel half is `TestRouteIntegration_StampedRemoveLeavesStaticRoute` |
| Link carrier down / up | (Go tests) | `TestHandleLinkDownWithRoutePriority` (`config_test.go:1808`) and `TestHandleLinkUpWithRoutePriority` (`:1840`) drive the handlers, both green in the run above |
| accept_ra_defrtr suppression -> `rtproto.Any` | (Go test) | `TestStaleKernelRouteCleanup` (`config_test.go:2714`), green in the run above |
| Router lost -> stamped remove | (Go test) | `TestNeighRouterRemoved` (`config_test.go:2468`), green in the run above |
| Learned metric reaches the operator | `test/plugin/iface-learned-route-metric.ci` | Read: it takes 254 from `ze schema show ze-iface-conf` rather than hardcoding it, asserts the static and the learned route coexist as two rows, and ends on `expect=stderr:contains=coexist, static forwards` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `ip route del` with and without `proto` in an ephemeral netns; `TestRouteIntegration_BlindRemoveDeletesKernelRoute` deletes an RTPROT_RA `::/0` with `rtproto.Any` on a real kernel |
| A-2 | confirmed | Reverting the add stamp made `TestRouteIntegration_AddStampsAndRemoveDeletesOwnRoute` report `protocol = 3`. Producer chain: `nl.NewRtMsg` sets `Protocol: unix.RTPROT_BOOT`, `prepareRouteReq` overrides only `if route.Protocol > 0` |
| A-3 | confirmed | `TestRouteProtocolsAreDistinct`, green in the rtproto run above; `rtProtoNames` in `route_linux.go` has no 253 |
| A-4 | confirmed | `TestIfaceProtocolIsNamedButNotZeOwned`, green in the rtproto run above. `IsZe` is an explicit switch over 250, 251, 252 |
| A-5 | broken | Measured: `ip route add` answers EEXIST for the shared key and `ip route replace` overwrites the protocol in place. Mistake Log row A-5, and the Deviations note that the 254 default only keeps the DEFAULT path away from the condition |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/interfaces.md`: "Routes carry `proto 253` (`ze-iface`), so a teardown removes only its own" | anchor at `internal/plugins/iface/netlink/manage_linux.go` -- `AddRoute` stamps `rtm_protocol`, `RemoveRoute` matches on it | Yes, re-read at both ends |
| `docs/guide/configuration.md`: the `route-priority` default is 254 | anchor at `internal/component/iface/config.go` -- `defaultLearnedRouteMetric = 254`, and the YANG leaves declare the same value (`TestRoutePriorityYANGDefaultMatchesTheConstant`) | Yes |
| `docs/guide/configuration.md`: 254 is the administrative distance a Cisco IOS DHCP client gives a learned default route | the doc now repeats what the comment above `defaultLearnedRouteMetric` states, and adds that the distance is not a Linux metric and ze does not read it (round 6 NOTE 3) | Yes, corrected in this closure |
| Every other category answered No | `grep -rn "source: internal/core/rtproto\|source: internal/plugins/iface/netlink\|source: internal/component/iface" docs/` returns 157 anchors across 32 files; the ones naming a file this diff changed are the two above, plus `docs/features.md` and `docs/architecture/core-design.md`, which name `rtproto.IsZe` and the 250-252 filter. Both stay exact because `Iface` is outside `IsZe` (AC-6) | Yes |
