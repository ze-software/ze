# 265 — BGP Chaos Self-Test Mode

## Objective

Add a self-testing mode to Ze itself: `--chaos-seed N` wraps Ze's real Clock, Dialer, and ListenerFactory with seed-driven chaos-injecting decorators that introduce random faults during normal operation to expose internal resilience bugs.

## Decisions

- Chaos wrappers delegate to real implementations (not fakes) and add probabilistic faults — the decorator pattern composes cleanly with the existing `sim.Clock`, `sim.Dialer`, and `sim.ListenerFactory` interfaces.
- `math/rand.Rand` with a mutex rather than `crypto/rand` — deterministic chaos (same seed → same fault sequence) is more debuggable than unpredictable chaos.
- Seed=0 means disabled — prevents accidental chaos activation from zero-initialized structs; CLI flag defaults to 0 (off).
- Child processes use env vars (`ze.bgp.chaos.seed`, `ze_bgp_chaos_seed`) — CLI flags are not propagated to forked child processes; env vars are the only reliable channel.
- Timer non-firing and backward clock excluded from ChaosClock — these would cause permanent FSM stalls and break everything; only jitter (0.8-1.2x multiplier) and sleep extension are safe fault types.

## Patterns

- `NewChaosWrappers(cfg)` convenience constructor returns all three wrappers at once — callers inject all three or none; partial injection would be inconsistent.
- Injection point is between `LoadReactorWithPlugins()` and `reactor.Start()` in both `hub/main.go` (in-process mode) and `bgp/childmode.go` (child mode) — SetClock/SetDialer/SetListenerFactory already existed for this purpose.

## Gotchas

- Ze starts via `ze config.conf` → `hub.Run()`, not via `ze bgp server` — the spec originally had wrong CLI syntax throughout; corrected after reading `cmd/ze/main.go`.
- Hook blocks `default:` case in fault-type selection switch — restructured to if/else chain.
- Config file `environment { chaos { } }` block parsing is wired but the config→reactor injection path is not connected; CLI flag and env var paths are complete. Full config-file injection deferred.

## Files

- `internal/sim/chaos.go` — ChaosConfig, chaosRNG, ChaosClock, ChaosDialer, ChaosListenerFactory, chaosConn, chaosListener, NewChaosWrappers
- `cmd/ze/main.go` — Global `--chaos-seed` and `--chaos-rate` flags
- `cmd/ze/hub/main.go` — Chaos wrapper injection in runBGPInProcess
- `cmd/ze/bgp/childmode.go` — Env var injection + injectChaosFromEnv helper
- `internal/component/config/environment.go` — ChaosEnv struct + config section
