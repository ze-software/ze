# Feature Integration Completeness

**BLOCKING:** Every new feature MUST be proven to work integrated, not just in isolation.
Rationale: `.claude/rationale/integration-completeness.md`

Every feature needs at least one end-to-end test from its intended usage point.

| Feature Type | Required Test |
|-------------|---------------|
| Injectable interface | Inject fake, verify component uses it |
| CLI flag | Flag changes program behavior |
| Config option | Option affects runtime behavior |
| API/RPC | Caller reaches handler through real transport |
| Event/hook | Event fires, subscriber receives |
| Plugin capability | Engine dispatches to plugin correctly |
| Struct field | Field is read and affects a decision |

**Self-check:** "If I deleted all new code except tests, would any test fail because it tried to USE the feature through the intended path?" No → isolation only → rule violated.

**Deferrable:** advanced behavior (deterministic scheduler, fault injection, property testing, benchmarks).
**NOT deferrable:** one test proving the wiring works.
