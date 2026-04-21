# 626 -- rs-fastpath-2-adjrib

## Context

Second child of the `rs-fastpath` umbrella. The umbrella's premise for this phase was that `bgp-adj-rib-in` subscribed synchronously to UPDATE events and its BART insert sat in bgp-rs's forwarding latency. Two linked changes were scoped: (1) make adj-rib-in async, (2) relax `bgp-rs -> bgp-adj-rib-in` from a hard dependency to optional so operators can run pure route-server forwarding without replay. Phase 1 profiling (see `plan/learned/625-rs-fastpath-1-profile.md`) captured the actual cost centres at 100k routes and reshaped the scope before any code landed.

## Decisions

- **Reframed scope to soft-dep only** over the literal spec. Profile evidence showed `bgp-adj-rib-in` contributed <1 % of allocation pressure and is not visible in the top-25 CPU nodes -- per-plugin `deliveryLoop` goroutines already isolate it from bgp-rs's forwarding path. The real 25.7 % CPU cost is `bgp-rib`, which is auto-loaded via `ConfigRoots: ["bgp"]` and out of this umbrella's reach. Pursuing an async-storage refactor of adj-rib-in would have been expensive architecturally with no measurable win.
- **Added `OptionalDependencies []string` to `registry.Registration`** over a sentinel value on the existing `Dependencies` field or a separate `SoftDeps` concept. Reasons: preserves backwards behaviour for hard deps (no migration); reuses the existing resolver, cycle-detection, and topological-tier code paths (each walks the union of both edge kinds); reads cleanly at call sites (`Dependencies: [...]` vs `OptionalDependencies: [...]`).
- **Owner handles run-time absence** over the registry doing a silent log. Placing the fallback in the owner keeps registry semantics narrow (dependency resolution, not user-facing messaging) and lets each owner craft the right message and behaviour for its feature. bgp-rs uses `sync.Once` + string-match on `"unknown command"` to detect and downgrade the error.
- **Send EOR on every replay-failure path** (not just the missing-dep path) over the original asymmetric pattern. The first review pass flagged it as pre-existing asymmetry; "resolve all" extended the fix to every branch. Replay returning an IPC timeout, an engine error, or a missing-dep fallback all terminate in a single unified `sendEOR(peerAddr, gen)` call. Without this, any non-success replay left the peer waiting indefinitely for end-of-RIB markers.
- **Kept Go unit test over a `.ci` functional test** for AC-5/AC-6. In the standard ze build, adj-rib-in is always registered (via `plugin/all` blank imports), so the "dep absent at run time" scenario only emerges in a custom build. `TestRSSoftDepSkipsReplay` exercises the identical code path deterministically via `dispatchCommandHook`.
- **Deferred async-storage refactor** to child-3 verification rather than abandoning it outright. If the rs -> engine text-RPC removal (child 3) exposes adj-rib-in or bgp-rib as residual bottlenecks, their subscription mode can be revisited then with fresh evidence.

## Consequences

- **New mechanism available:** any plugin can now declare soft relationships with `OptionalDependencies`. Cycle detection + tier ordering already work when both endpoints are in the resolved set. This unblocks future "feature-gating via plugin presence" patterns.
- **No behaviour change for the default build.** `adj-rib-in` is still registered via `plugin/all`, so `ResolveDependencies` still pulls it in when bgp-rs is configured -- existing `.ci` tests (`adj-rib-in-replay-on-peerup`, `rs-ipv4-withdrawal`, `rs-backpressure`) all pass unchanged.
- **Behaviour change in error paths:** `replayForPeer` now always terminates with `sendEOR(peerAddr, gen)` regardless of how the replay ended (success, missing-dep, IPC timeout, engine error). Pre-refactor, transient replay failures left the peer without EOR -- the peer would be excluded from forward targets (Replaying=true was cleared, but no EOR was sent), leaving peers that gate on EOR stuck in initial-sync. This is an improvement but WAS a silent change in observable wire behaviour; peers that previously never received EOR on a replay error now do. `sendEOR` internally checks `p.ReplayGen == gen` + `len(p.Families) > 0`, so stale-generation and new-peer races are handled.
- **Config-reload teardown now handles optional deps.** `stopOrphanedDependencies` walks both `Dependencies` and `OptionalDependencies` via the new `collectOrphanCandidates` + `pluginDependsOn` helpers. Without this, a minimal deployment (bgp-rs as the only user of adj-rib-in) would leak adj-rib-in across config-reload teardowns of bgp-rs.
- **Forward-path performance unchanged in practice.** The expected perf win was small to begin with (<1 % allocation share) and absent unless the operator also excludes `adj-rib-in` from the registered set. Recorded honestly in the spec's audit and in the umbrella's AC-1 rationale.
- **Constraint for future plugin authors:** when declaring an `OptionalDependencies` entry, the owner MUST provide a run-time fallback for absence. bgp-rs's one-shot WARN + skip pattern is documented in `ai/rules/plugin-design.md` as the reference template.
- **Bigger performance win remains deferred:** `bgp-rib`'s 25.7 % CPU cost is still on the table; child 3's zero-copy pass-through targets the more structural `sdk.UpdateRoute` RPC round-trip, not the subscriber side. A future spec may re-examine `bgp-rib`'s ConfigRoots-based auto-load if profile evidence warrants.

## Gotchas

- **Phase 1 profile explicitly contradicted the umbrella's premise.** The umbrella cited `RIBManager.dispatchStructured` at 25.72 % CPU as the "adj-rib-in hot-path cost" -- but that package is `rib`, not `adj_rib_in`. Easy mistake because both plugins have similar names and both store routes. Keep function-name-to-package mappings explicit when profile-reading.
- **`sync.Once` is package-level and cannot be reset for tests.** The one-shot WARN log cannot be asserted from a test that may run after other tests in the same binary. Tests instead assert behaviour (skip + clear + EOR) rather than log presence. Acceptable trade-off.
- **`ErrUnknownCommand` crosses the plugin IPC boundary as a plain string.** The rs plugin cannot `errors.Is(err, server.ErrUnknownCommand)` because it lives in a different import tree. `strings.Contains(err.Error(), "unknown command")` is the pragmatic match; if the engine ever changes that error string, the soft-dep fallback silently reverts to logging ERROR (tests catch this because the same string is used).
- **Updating `TestBgpRSDependsOnAdjRibIn` rather than deleting it.** The test's intent ("bgp-rs declares its relationship with adj-rib-in") survived the refactor; the assertion changed from checking `Dependencies` to checking `OptionalDependencies` + absence from `Dependencies`. Intent-preserving updates are fine; weakening assertions is not.
- **Cycle detection with mixed edges is non-obvious.** A cycle `A --(optional)--> B --(hard)--> A` must still be rejected at registration time, otherwise `ResolveDependencies([...])` can enter a loop when B is registered. Explicit test (`TestResolveDependenciesOptionalCycle`) guards this.

## Files

- `internal/component/plugin/registry/registry.go` -- `OptionalDependencies` field; resolver + cycle + tier walks; registration validation.
- `internal/component/plugin/registry/registry_test.go` -- 7 new tests covering optional-dep semantics (present / absent / mixed / tier / self / empty / cycle).
- `internal/component/plugin/server/startup_autoload.go` -- `collectOrphanCandidates` + `pluginDependsOn` pure helpers; `stopOrphanedDependencies` walks both hard and optional deps.
- `internal/component/plugin/server/startup_autoload_test.go` -- `TestCollectOrphanCandidates_HardAndOptional` (6 sub-tests) + `TestPluginDependsOn` (9 sub-tests).
- `internal/component/bgp/plugins/rs/register.go` -- moved adj-rib-in from `Dependencies` to `OptionalDependencies`.
- `internal/component/bgp/plugins/rs/server.go` -- `adjRibInMissingOnce sync.Once` as a RouteServer struct field (instance-scoped, test-friendly) instead of package-level.
- `internal/component/bgp/plugins/rs/server_handlers.go` -- `errUnknownCommandMarker` const + `isDispatchUnknownCommand` helper + unified `sendEOR`-on-every-error-path fallback in `replayForPeer`.
- `internal/component/bgp/plugins/rs/server_test.go` -- `TestRSSoftDepSkipsReplay` using channel-based sync (not polling) + spot-check on the EOR command text.
- `internal/component/plugin/all/all_test.go` -- `TestBgpRSDependsOnAdjRibIn` updated to assert new field placement.
- `ai/rules/plugin-design.md` -- `OptionalDependencies` table row + "Optional Dependencies" section (graceful-fallback pattern).
- `docs/guide/plugins.md` -- Dependencies section rewritten with hard/optional split + bgp-rs example.
- `plan/deferrals.md` -- three rows: async-storage deferral, `.ci` functional test deferral, `bgp-rib`'s ConfigRoots auto-load as a separate open question.
