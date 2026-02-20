# Feature Integration Completeness

**BLOCKING:** Every new feature MUST be proven to work integrated, not just in isolation.
Rationale: `.claude/rationale/integration-completeness.md`

## Rule

Every new feature needs at least one end-to-end test from its intended usage point.

| Feature Type | Required Integration Test |
|-------------|--------------------------|
| Injectable interface | Inject fake, verify component uses it |
| New CLI flag | Flag changes program behavior |
| New config option | Option affects runtime behavior |
| New API/RPC | Caller reaches handler through real transport |
| New event/hook | Event fires, subscriber receives it |
| New plugin capability | Engine dispatches to plugin correctly |
| New struct field | Field is read and affects a decision |

## Self-Check

> "If I deleted all new code except tests, would any test fail because it tried to USE the feature through the intended path?"

If no → feature is tested in isolation only → rule violated.

## What Can Be Deferred

Deferrable: advanced behavior (deterministic scheduler, fault injection, property testing, benchmarks).
NOT deferrable: basic integration proof (one test proving the wiring works).
