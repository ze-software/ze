# Feature Integration Completeness

**BLOCKING:** Every new feature MUST be proven to work integrated, not just in isolation.
Rationale: `ai/rationale/integration-completeness.md`

Every feature needs at least one end-to-end test from its intended usage point.

| Feature Type | Required Test |
|-------------|---------------|
| Injectable interface | Inject fake, verify component uses it |
| CLI flag | Flag changes program behavior |
| Config option | Option affects runtime behavior |
| YANG config leaf | Env var registered (`env.MustRegister`), appears in `ze env registered` |
| API/RPC | Caller reaches handler through real transport |
| Event/hook | Event fires, subscriber receives |
| Plugin capability | Engine dispatches to plugin correctly |
| Struct field | Field is read and affects a decision |

**Self-check:** "If I deleted all new code except tests, would any test fail because it tried to USE the feature through the intended path?" No → isolation only → rule violated.

## Functional `.ci` Test (BLOCKING)

**Every user-facing feature MUST have a `.ci` functional test** in `test/` that exercises the feature from the user's perspective: config file → ze launch → command/event → expected output. A Go unit test proves the algorithm; a `.ci` test proves a user can reach and use the feature.

| Feature Type | `.ci` Location | What the test does |
|-------------|----------------|-------------------|
| Config option | `test/parse/` | Config with option → ze parses without error |
| API/RPC command | `test/plugin/` | Config + peer → send command → verify wire/JSON output |
| Plugin behavior | `test/plugin/` | Config + plugin → trigger behavior → verify effect |
| CLI subcommand | `test/parse/` or `test/ui/` | Run subcommand → verify stdout/stderr/exit code |
| Wire encoding | `test/encode/` | Config with route → verify hex output |
| Wire decoding | `test/decode/` | Hex input → verify JSON output |

**A unit test is NOT a substitute for a `.ci` test.** Unit tests validate logic in isolation. `.ci` tests validate the feature is wired, reachable, and usable. Both are required.

**Deferrable:** advanced behavior (deterministic scheduler, fault injection, property testing, benchmarks).
**NOT deferrable:** one `.ci` test proving the feature works from the user's entry point.

## Wiring Tests (BLOCKING — NEVER deferrable)

A wiring test proves the feature is reachable from its intended entry point (config, CLI, event dispatch, plugin launch). It is the minimum proof that the feature is integrated, not just isolated. **For user-facing features, the wiring test MUST be a `.ci` functional test**, not a Go unit test.

| Banned | Why |
|--------|-----|
| "Deferred to next spec" | Next spec won't pick it up. Feature ships unwired. |
| "Requires infrastructure not yet built" | Then the feature is blocked, not done. |
| "Unit tests cover the logic" | Unit tests prove the algorithm, not the wiring. |
| "make ze-verify passes" | Passing tests that don't exercise the entry point prove nothing. |
| "Go test exercises the handler" | A Go test with mocked entry points is not a `.ci` test. |

**If the wiring test cannot be written, the feature is not done — it is blocked.**

Every spec MUST have a `## Wiring Test` table (see `plan/TEMPLATE.md`). Every row for a user-facing feature must name a `.ci` test file.

## Production Path Verification (BLOCKING)

Before modifying any handler, dispatcher, or protocol step: **grep for ALL implementations** of that function/protocol step in the codebase. Ze has multiple code paths for the same protocol (e.g., `subsystem.go` and `plugin/server/startup.go` both implement stage-1). Modifying one is not enough.

| Step | Action |
|------|--------|
| 1 | Grep for the protocol method/handler name across all `.go` files |
| 2 | List every implementation found |
| 3 | For each consumer of the feature: trace which implementation it actually calls |
| 4 | Modify (and test) the implementation the consumer uses, not just any implementation |

**One implementation found is not proof there's only one.** Finding *a* handler is not the same as finding *the* handler the feature's consumer calls.
