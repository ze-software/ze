# 1054 - Anomaly facts->judgment->response end-to-end integration test

**Spec:** spec-anomaly-4-interop-harness (closed) | **Date:** 2026-07-02 | **Depends:** 1046, 1048, 1049 | **Umbrella:** spec-anomaly-0-umbrella (child 4)

## Context

The anomaly chain (`trafficfeature` facts -> `anomaly/detect` judgment -> `anomaly/shape`
response) was unit-tested per layer, but nothing drove the WHOLE chain from real feature data.
The three existing anomaly `.ci`s prove wiring with empty data and say so. Goal: one test that
injects synthetic traffic, gets a real incident, and arms the responder.

## Key Decisions

- **In-process Go integration test, NOT a `.ci`.** The original plan (a test-only `fakeflow`
  plugin publishing to `observation.Feed`, driven by a `.ci`) was BUILT and then ABANDONED: a
  `.ci` cannot drive this chain. `observation.Feed` is a **process-local** bus
  (`observation.go`); a config-`plugin{internal}` plugin runs isolated from the engine's
  in-engine `trafficfeature` in the functional-test DUT, so its `observation.Global().Publish`
  never reaches `trafficfeature` (proven by a `selfcheck` command: the injector's own publish is
  received in its process, `self_received=1`, while the engine's `trafficfeature` stayed
  `degraded`). The user chose the in-process test, which co-locates the whole chain.
- **Compose the real production types in one process.** `TestChainFactsToResponse`
  (`detect/chain_integration_test.go`) builds a real `trafficfeature.NewService(observation.Global())`,
  a real `newDetector(cfg, bus)`, and a real responder (via `shape.SubscribeForTest`), wires them on
  an in-memory `testBus` (mirrors `ddosevent/event_test.go`), publishes synthetic `KindFlow`
  observations, ticks, and asserts `recentIncidents()>0` AND the outlier `10.0.0.9/32` is armed.
- **`shape.SubscribeForTest` is the cross-package composition seam.** `newDetector`/`newResponder`
  are package-private; a test needing both packages needs one exported helper. `SubscribeForTest`
  (new `shape/testsupport.go`, `*ForTest` idiom, precedent: `ResetForTest`, `SetUsersForTest`)
  constructs an armed responder, mocks the firewall backend (`registerTables`/`applyAll` package
  vars -> no-ops), subscribes it to the bus, and returns an armed-prefix accessor + stop.
- **Discrimination, not just "something fired," is the real proof.** A BALANCED normal cohort must
  stay unarmed while only the pure-outbound / high-fan-out / rare-port outlier arms. The scenario
  warms every source with balanced in+out traffic, then makes only the outlier go pure-outbound;
  the test asserts the cohort does NOT arm during warmup and only `10.0.0.9/32` arms under attack.

## Consequences

- The chain has a real end-to-end regression gate before Phase B widens any layer. Runs ~10s (real
  1s ticks); `-short`-skippable so local iteration is fast, the CI gate (plain `go test`) still runs it.
- No production behavior changed. `shape/testsupport.go` ships an exported test-only helper (dead in
  production, accepted `*ForTest` pattern). `docs/functional-tests.md` documents the pattern (feeds
  that can't cross the plugin boundary are proven by in-process Go integration tests).

## Gotchas

- **`observation.Feed` is process-local (`observation.go`).** Publishers and subscribers must be
  in the SAME process. A config-loaded plugin cannot inject into the engine's feed. If you need to
  drive a feed-based chain from a test, compose the types in one process (a Go test), not via a `.ci`.
- **Pure-outbound sources read as exfil.** Every source with zero inbound bytes has `+Inf` out/in
  ratio and trips the exfil signal, so a "normal" cohort injected outbound-only gets flagged too.
  Normals need BALANCED (in+out) traffic; only the outlier goes pure-outbound. This bit the first cut
  (armed all 6 sources) before the balanced-cohort fix (armed only the outlier).
- **`internal` plugins are in-process by the code (`process.go startInternal` goroutine), but the
  functional-test DUT's process topology isolated the config-loaded plugin anyway.** Trust the
  runtime evidence (a `selfcheck`/PID probe) over a reading of the start path.
- **Discovered, out of scope:** `deviation-threshold` (and any `decimal-2` YANG leaf with a `range`)
  is currently unsettable -- "range validation not supported for type string" (`schema.go`)
  against the in-flight anomaly YANG restructure. Flagged to the user; not fixed here.
- Config restructure (2026-07-02): anomaly config is nested `anomaly { detect {} shape {} }`; show
  commands are `show anomaly detect` / `show anomaly shape` (wire methods `ze-show:anomaly*` unchanged).

## Files

None recorded.
