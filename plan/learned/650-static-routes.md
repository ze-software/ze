# 650 -- Static Route Plugin with ECMP, BFD Failover, and Redistribute

## Context

Ze needed a static route plugin that programs routes directly to the kernel (netlink) and VPP (GoVPP), supporting ECMP with weighted next-hops, BFD-tracked failover, blackhole/reject, and redistribute integration. LocRIB was considered first but rejected because it selects a single best path per prefix and has no ECMP or weight concept; extending it would change the pipeline contract for all protocols.

## Decisions

- Direct FIB programming instead of LocRIB injection. Static routes are config-authoritative; the operator declares exactly what they want. Admin-distance arbitration adds little value (static distance 10 already beats eBGP 20 and iBGP 200).
- ECMP via multipath routes. Linux: `Route.MultiPath` with `[]*NexthopInfo`. VPP: multiple `FibPath` entries. Weight mapping: kernel `Hops = Weight - 1`, VPP `Weight` direct.
- BFD modifies ECMP group membership, not route presence. On BFD DOWN, that NH is removed and the multipath route is reprogrammed with remaining NHs. Only when ALL NHs are down is the route withdrawn entirely.
- Shared `RTPROT_ZE=250` with fib-kernel. No collision: fib-kernel manages sysrib-derived prefixes, static manages config-driven prefixes.
- VPP backend in a separate sub-package (`internal/plugins/static/vpp/`) to avoid forcing a govpp import on all builds. Defines its own `Path` type with `Weight uint8` (VPP limit); callers translating from the parent's `uint16` Weight must cap.
- Redistribute events use a separate `events` package (`internal/plugins/static/events/`) following the l2tp pattern: package-level `var RouteChange = events.Register[*redistevents.RouteChangeBatch](...)`. The events package is imported by inject.go (named import), not by a blank import in register.go.
- Emit tracking uses a per-routeState `emitted` bool. On forward->non-forward route replacement, ActionRemove is emitted for the old route. On forward->forward replacement, no Remove is emitted (avoiding redistribute flap); the new route emits ActionAdd which is idempotent for the consumer.
- `static show` output is sorted by prefix string for deterministic API output. Lexicographic sort, not numeric (consistent with being JSON-consumed, not human-read).

## Consequences

- Static routes are visible to `redistribute { import static }` via the redistevents bus without going through LocRIB.
- The plugin auto-loads when config contains `static { }` (config-driven plugin loading via `ConfigRoots`).
- VPP and kernel backends are independent: kernel uses build tags (`//go:build linux`), VPP is a separate package. The VPP backend has no plugin registration yet (just route programming); full plugin wiring requires either exporting parent types or duplicating the config parser.
- Functional tests require `CAP_NET_ADMIN` on Linux (same as firewall tests) for kernel route programming; the `static show` test works on any platform via the noop backend.

## Mistakes

- First spec draft proposed LocRIB injection. Abandoned because LocRIB's `selectBest` picks a single winner, fundamentally incompatible with ECMP groups.
- Initial redistribute emit implementation emitted ActionRemove unconditionally on route replacement in `applyRouteLocked`, causing a Remove+Add flap for forward->forward updates. Fixed by gating the Remove on `r.Action != actionForward`.
- `showRoutes()` initially returned a nil slice (producing `"null"` in JSON instead of `"[]"`). Fixed by initializing with `make([]showRoute, 0, len(rm.routes))`.

## Implementation Stats

- 2 commits, 22 new files, ~2700 lines added
- 35 unit tests passing (config parsing, diff, BFD integration, registration, CLI formatting)
- 4 functional tests (boot-apply, reload-add, reload-remove, show)
- 6 documentation files updated (features, configuration, command-reference, command-catalogue, plugins, comparison)
