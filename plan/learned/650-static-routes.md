# 650 -- Static Route Plugin with ECMP, BFD Failover, and Redistribute

## Context

Ze needed a static route plugin that programs routes directly to the kernel (netlink) and VPP (GoVPP), supporting ECMP with weighted next-hops, BFD-tracked failover, blackhole/reject, and redistribute integration. LocRIB was considered first but rejected because it selects a single best path per prefix and has no ECMP or weight concept; extending it would change the pipeline contract for all protocols.

## Decisions

- Direct FIB programming instead of LocRIB injection. Static routes are config-authoritative; the operator declares exactly what they want. Admin-distance arbitration adds little value (static distance 10 already beats eBGP 20 and iBGP 200).
- ECMP via multipath routes. Linux: `Route.MultiPath` with `[]*NexthopInfo`. VPP: multiple `FibPath` entries. Weight mapping: kernel `Hops = Weight - 1`, VPP `Weight` direct.
- BFD modifies ECMP group membership, not route presence. On BFD DOWN, that NH is removed and the multipath route is reprogrammed with remaining NHs. Only when ALL NHs are down is the route withdrawn entirely.
- Shared `RTPROT_ZE=250` with fib-kernel. No collision: fib-kernel manages sysrib-derived prefixes, static manages config-driven prefixes.
- VPP backend in a separate sub-package (`internal/plugins/static/vpp/`) to avoid forcing a govpp import on all builds. Defines its own `Path` type with `Weight uint8` (VPP limit); callers translating from the parent's `uint16` Weight must cap. For an interface-only path (no next-hop address) the path proto is derived from the ROUTE's family, not the (zero) next-hop address (`toFibPath(p, dst)`); see Gotchas (spec-fixit-static-interface-nexthops C-5).
- Redistribute events use a separate `events` package (`internal/plugins/static/events/`) following the l2tp pattern: package-level `var RouteChange = events.Register[*redistevents.RouteChangeBatch](...)`. The events package is imported by inject.go (named import), not by a blank import in register.go.
- Emit tracking uses a per-routeState `emitted` bool. On forward->non-forward route replacement, ActionRemove is emitted for the old route. On forward->forward replacement, no Remove is emitted (avoiding redistribute flap); the new route emits ActionAdd which is idempotent for the consumer.
- `static show` output is sorted by prefix string for deterministic API output. Lexicographic sort, not numeric (consistent with being JSON-consumed, not human-read).

## Consequences

- Static routes are visible to `redistribute { import static }` via the redistevents bus without going through LocRIB.
- The plugin auto-loads when config contains `static { }` (config-driven plugin loading via `ConfigRoots`). It also declares `OptionalDependencies: ["interface"]`, so when an `interface` stanza is present static is ordered into a LATER startup tier than the iface component and its next-hop resolves against an already-loaded backend (spec-fixit-static-interface-nexthops C-1/A-1c); with no `interface` stanza the optional dep is inert and static is unconstrained.
- VPP and kernel backends are independent for their FIB writes (kernel uses build tags `//go:build linux`, VPP is a separate package), BUT both now resolve an interface next-hop through the SAME shared `iface.Resolve`: the netlink backend reads `Binding.Ifindex` as a kernel ifindex, the VPP backend reads it as a VPP `sw_if_index` (the VPP iface backend publishes its index through `iface.InterfaceInfo.Index`). One resolver, two data planes (spec-fixit-static-interface-nexthops C-3/A-3a). The VPP backend has no plugin registration yet (just route programming); full plugin wiring requires either exporting parent types or duplicating the config parser.
- Functional tests require `CAP_NET_ADMIN` on Linux (same as firewall tests) for kernel route programming; the `static show` test works on any platform via the noop backend.

## Mistakes

- First spec draft proposed LocRIB injection. Abandoned because LocRIB's `selectBest` picks a single winner, fundamentally incompatible with ECMP groups.
- Initial redistribute emit implementation emitted ActionRemove unconditionally on route replacement in `applyRouteLocked`, causing a Remove+Add flap for forward->forward updates. Fixed by gating the Remove on `r.Action != actionForward`.
- `showRoutes()` initially returned a nil slice (producing `"null"` in JSON instead of `"[]"`). Fixed by initializing with `make([]showRoute, 0, len(rm.routes))`.

## Gotchas

- **Zero `netip.Addr` silently means IPv6 in `toFibPath`.** A zero `netip.Addr` has `Is4() == false`, so the pre-fix code encoded an interface-only (address-less) path as `PROTO_IP6` with an all-zero v6 next-hop even for an IPv4 route. `toFibPath` now takes the route prefix and, when the next-hop is unset, derives the proto from the route family (spec-fixit-static-interface-nexthops C-5/A-2a). Any new address-less path type must pass the route family, never infer proto from the next-hop.
- **The static data plane and the iface backend are chosen by two INDEPENDENT globals.** `vpp.GetActiveConnector()` selects the VPP static backend; `iface.LoadBackend`/`activeBackend` selects the iface backend. They can disagree: a VPP static backend resolving a name against a NETLINK iface backend would get a kernel ifindex and program it as a VPP `sw_if_index` -- a silently wrong path. The VPP static backend guards this by gating interface-only resolution on `iface.ActiveBackendName() == "vpp"` and rejecting a zero/invalid index (spec-fixit-static-interface-nexthops C-4/R-7). Any future VPP-aware consumer of `iface.Resolve` must apply the same gate.
- **Interface-only next-hop with no iface backend gives an actionable error.** `resolveNexthopIndex` distinguishes the no-backend case (`iface.GetBackend() == nil`) from a device-absent case and names the missing `interface { backend ... }` stanza (C-2). A doctor check (`doctor-static-interface-nexthop-no-backend`) surfaces the same dependency at config time. Neither is redundant: an interface next-hop may legitimately name an externally-created interface, so runtime resolution failure is always possible.

## Blast radius (deliberate, documented)

One unresolvable next-hop fails the WHOLE static section, not just its own route: per-route errors are joined (`inject.go` `errors.Join`) and a non-nil result becomes the `OnConfigure` error (`register.go`), which aborts daemon startup. This is a deliberate, documented choice (spec-fixit-static-interface-nexthops AC-3/D-3 = keep-and-document). Per-route isolation (log-and-skip the bad route, keep the rest) is the operator-friendlier behavior but changes the failure contract for ALL static routes and interacts with `OnConfigApply`'s journal/rollback, so it is deferred to its own spec (`plan/spec-fixit-static-per-route-isolation.md`). Any rework of `applyRoutes` MUST preserve the `routesEqual` diff short-circuit (`diff.go`), because `WantsConfig` includes `interface` and the diff is what keeps an unrelated interface edit from reprogramming every static route (R-10).

## Implementation Stats

- 2 commits, 22 new files, ~2700 lines added
- 35 unit tests passing (config parsing, diff, BFD integration, registration, CLI formatting)
- 4 functional tests (boot-apply, reload-add, reload-remove, show)
- 6 documentation files updated (features, configuration, command-reference, command-catalogue, plugins, comparison)
