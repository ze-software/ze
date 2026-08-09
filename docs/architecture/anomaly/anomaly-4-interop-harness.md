# Anomaly Chain: End-to-End Test Harness

The chain `trafficfeature` facts, [`anomaly/detect`](anomaly-1-detect.md)
judgment, [`anomaly/shape`](anomaly-2-shape.md) response was unit-tested per
layer, and nothing drove the whole chain from real feature data. The three anomaly
`.ci` tests prove wiring with empty data and say so.

## The chain cannot be driven from a `.ci`

<!-- source: internal/core/observation/observation.go -- Feed, Global, Publish -->

`observation.Feed` is a PROCESS-LOCAL bus. A publisher and a subscriber must live
in the same process.

The first plan was a test-only `fakeflow` plugin publishing to
`observation.Feed`, driven by a `.ci`. It was built and abandoned. A plugin loaded
by config runs isolated from the engine's in-engine `trafficfeature` in the
functional-test DUT, so its `observation.Global().Publish` never reaches
`trafficfeature`. A `selfcheck` command proved it: the injector received its own
publish in its own process (`self_received=1`) while the engine's
`trafficfeature` stayed `degraded`.

An `internal` plugin is in-process by the code (`process.go`, the `startInternal`
goroutine). The DUT's process topology isolated the config-loaded plugin anyway.
Trust runtime evidence, a selfcheck or a PID probe, over a reading of the start
path.

## The harness composes the production types in one process

<!-- source: internal/plugins/anomaly/shape/testsupport.go -- SubscribeForTest -->

`TestChainFactsToResponse` builds a real
`trafficfeature.NewService(observation.Global())`, a real detector and a real
responder, wires them on an in-memory bus, publishes synthetic `KindFlow`
observations, ticks, and asserts both that an incident is recorded and that the
outlier `10.0.0.9/32` is armed.

`shape.SubscribeForTest` is the cross-package composition seam.
`newDetector` and `newResponder` are package-private, so a test that needs both
packages needs one exported helper. `SubscribeForTest` builds an armed responder,
mocks the firewall backend by replacing the `registerTables` and `applyAll`
package variables with no-ops, subscribes it to the bus, and returns an
armed-prefix accessor and a stop function. It follows the `*ForTest` idiom already
used by `ResetForTest` and `SetUsersForTest`, and it is dead code in production.

## The proof is discrimination, not activity

A balanced normal cohort must stay unarmed while only the outlier arms. The
scenario warms every source with balanced inbound and outbound traffic, then makes
only the outlier go pure-outbound. The test asserts that the cohort does NOT arm
during warmup and that only `10.0.0.9/32` arms under attack.

**A pure-outbound source reads as exfiltration.** Zero inbound bytes gives an
infinite out-to-in ratio, which trips the exfil signal. A cohort injected as
outbound-only is therefore flagged as well. The first cut armed all six sources
for this reason. Normal sources need balanced traffic, and only the outlier goes
pure-outbound.

The test runs for about 10 seconds on real one-second ticks. It is skipped under
`-short` so local iteration stays fast, and the plain `go test` CI gate runs it.
