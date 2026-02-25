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

## Wiring Tests (BLOCKING — NEVER deferrable)

A wiring test proves the feature is reachable from its intended entry point (config, CLI, event dispatch, plugin launch). It is the minimum proof that the feature is integrated, not just isolated.

| Banned | Why |
|--------|-----|
| "Deferred to next spec" | Next spec won't pick it up. Feature ships unwired. |
| "Requires infrastructure not yet built" | Then the feature is blocked, not done. |
| "Unit tests cover the logic" | Unit tests prove the algorithm, not the wiring. |
| "make ze-verify passes" | Passing tests that don't exercise the entry point prove nothing. |

**If the wiring test cannot be written, the feature is not done — it is blocked.**

Every spec MUST have a `## Wiring Test` table (see `docs/plan/TEMPLATE.md`). Every row must have a concrete test name. The `validate-spec.sh` hook enforces this mechanically (exit 2).
