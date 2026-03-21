# 305 — Plugin Startup Ordering

## Objective

Fix two bugs causing "rpc error: unknown command" in ze-chaos: (1) plugins with dependencies start simultaneously, so a dependent plugin can dispatch commands before its dependency has registered them; (2) `adj-rib-in` `OnExecuteCommand` drops the `args` parameter, breaking delta replay entirely.

## Decisions

- Tier-ordered handshake with a single ProcessManager per phase (Approach A) chosen over: multiple `runPluginPhase` calls per tier (Approach B — overwrites `s.procManager`, losing schema/cleanup visibility) and DAG-aware coordinator (Approach C — over-engineered, only 1 dependency edge exists)
- `TopologicalTiers()` uses Kahn's algorithm, placed next to `ResolveDependencies` in `registry.go` — external plugins (not in registry) are tier 0
- `proc.index` is reassigned tier-locally before each tier's handshake — coordinator only cares about tier-local indices, PM-global ordering is irrelevant
- Async handlers start AFTER all tiers complete — they read from the same connections used during the startup handshake
- `TestTieredStartupOrdering` and `TestSinglePMAfterTieredStartup` skipped — existing integration tests (bgp-rs + bgp-adj-rib-in) already exercise tiered startup; mock-heavy startup tests would be brittle

## Patterns

- `s.procManager` overwrite (latent bug): Phase 1 and Phase 2 already overwrote it. Hasn't manifested because Phase 2 NLRI decoder plugins don't dispatch commands to Phase 1 plugins. This spec fixes it structurally via a single PM per phase.
- `net.Pipe` is synchronous — later-tier processes naturally wait on write until the engine reads, providing zero-cost tier sequencing

## Gotchas

- `adj-rib-in` `OnExecuteCommand` at `rib.go:105-106` passed `peer` instead of `args` to `handleCommand`. Delta replay always targeted `"*"` with `from-index=0` — completely broken, silently. Fixed: `strings.Join(args, " ")`.
- bgp-rs 5x100ms retry loop was a workaround for the ordering race, not a real fix — can be simplified after tier ordering is in place

## Files

- `internal/component/plugin/registry/registry.go` — TopologicalTiers() added
- `internal/component/plugin/server_startup.go` — runPluginPhase refactored (single PM, per-tier coordinator, async handlers after all tiers)
- `internal/component/plugin/types.go` and `internal/component/bgp/reactor/reactor.go` — Dependencies field added
- `internal/component/bgp/plugins/adj_rib_in/rib.go` — OnExecuteCommand args passthrough fixed
