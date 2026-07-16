# Spec: fixit-reject-fence-observability-deferred-external-plugin-signals

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fixit-plugin-event-subscription |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/process/process.go:791` - `monitorCmd`, the exit-detection site that emits nothing
4. `internal/component/plugin/server/dispatch.go:135` - `registerSubscriptions`, the namespace choke point
5. `internal/component/plugin/resolve.go:116` - `RegisterPluginEventTypes`, the declaration-side producer of the same namespace bug
6. `plan/spec-fixit-plugin-event-subscription.md` - risk R-6, which predicts this spec

## Task

Give `test/plugin/as112-external-refuses.ci` and `test/plugin/cos-external-warns.ci` a
queryable signal to fence on, so they can drop their `time.sleep(4.0)` blind holds
(`as112-external-refuses.ci:79`, `cos-external-warns.ci:80`) without losing their stderr
proofs. Ratchet `test/.ci-sleep-baseline` 132 -> 130.

**Provenance:** deferred from `plan/spec-fixit-reject-fence-observability.md` (its AC-3b,
Phase 2), ruled by Thomas 2026-07-16: spec it separately rather than fold it into
`spec-fixit-plugin-event-subscription`. That spec is at `design` and owns its own gaps;
loading this surface into it couples two things that fail independently. The source spec
closes at Phase 1 and its AC-3b is retired into this file.

### The source spec's name for this work is wrong, and the correction is the design

`spec-fixit-reject-fence-observability` calls the missing thing "the external-plugin-exit
signal" (`:174`, `:185-191`). Re-reading the two tests shows that is one signal too few.
**They do not want the same event:**

| Test | What it waits for | Does the plugin exit? | Evidence |
|------|-------------------|----------------------|----------|
| `as112-external-refuses.ci` | the subprocess starts, logs its ERROR refusal, and **exits** | YES | `:85` `expect=stderr:contains=refusing to start as an external plugin process` |
| `cos-external-warns.ci` | the subprocess starts and **emits a WARN**, then **keeps running** | NO | `:74-76` docstring says cos keeps running after the warning; the test tears the daemon down via the sleep timeout "rather than waiting for cos's own exit" |

So an exit event fences as112 and can never fence cos. cos needs a startup-reached /
warn-emitted signal. Designing one event and discovering this at implementation time is the
failure this table exists to prevent.

### Both tests need a second thing, independent of the signal

Neither test has an observer plugin. `wait.py` runs as a bare `cmd=foreground` process
(`as112-external-refuses.ci:83`, `cos-external-warns.ci:84`) with no `ze_api` import, no
plugin socket, and therefore nothing that could poll or receive a signal even if one
existed (`spec-fixit-reject-fence-observability.md:224-227`). Each test needs an observer
plugin added. This is half the work and is not blocked on anything.

### Why the Phase 1 counter cannot serve either test

Recorded so nobody re-litigates it: the refuse/warn fires during plugin **startup**, and
neither test performs a reload at all, so the reload-generation counter
(`internal/component/plugin/server/reload_generation.go`) can never fence them
(`spec-fixit-reject-fence-observability.md:222-224`, risk A-1).

### The dependency, and the trap inside it

`Depends | spec-fixit-plugin-event-subscription` (`Status | design` as of 2026-07-16, so
this spec waits on something not yet built).

That spec's **risk R-6** (`:454-475`) is the deconfliction this work was blocked on, and it
already did the analysis: an `external-plugin-exited` event declared via
`Registration.EventTypes` gets registered by `RegisterPluginEventTypes`
(`resolve.go:116-135`), which reads the single global default namespace at `resolve.go:121`
-- `bgp`, and only `bgp` (`RegisterDefaultEventNamespace` is called exactly once, at
`internal/component/bgp/plugin/register.go:64`). A plugin-host lifecycle event landing in
the `bgp` namespace is "wrong on its face ... and invisible to anyone subscribing to a sane
namespace" (`:456-462`).

R-6 says the collision "has not materialized" because the concurrent Phase 1 work took the
counter option and declared no event type (`:466-475`). **This spec is the next plugin R-6
predicts will hit that bug silently.** Re-check R-6 at Phase 0 before implementing (`:475`).

> **The dependency does not fully cover us.** `spec-fixit-plugin-event-subscription`'s Gaps
> A/B/C thread a namespace through `registerSubscriptions` (`dispatch.go:148`) -- the
> **subscribe** side. `resolve.go:121` is the **declaration** side, and the spec names these
> as two distinct producers of the same bug (`:461-462`). Landing that spec fixes the half
> we need for the observer to subscribe, but this spec must still either extend
> `resolve.go:121` itself or knowingly ship the event under `bgp` as debt. Decide that in
> design, not in code review.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` - plugin/engine boundary and event surface
  → Constraint: [fill during design]
- [ ] `docs/architecture/api/process-protocol.md` - the wire protocol an observer plugin speaks
  → Constraint: [fill during design]
- [ ] `ai/rules/fail-closed-guards.md` - a fence that cannot advance must fail loudly
  → Constraint: `dispatch_until` returns its LAST result when attempts run out rather than raising (`test/scripts/ze_api.py:1436-1439`), so a caller that does not re-check the predicate and `runtime_fail` turns a never-advancing fence into a silent pass

**Key insights:**
- `process.go:791` `monitorCmd` is the only exit-detection site, and it discards everything: `:793` `_ = cmd.Wait()` throws away the exit code, `:795` sets `running` false, `:798-806` cancels the context and closes conns. No log, no event, no diagnostic.
- The internal-plugin path already logs its exit (`process.go:515`, `logger().Warn("internal plugin exited with non-zero code", ...)`). External has no equivalent. The asymmetry is the gap.
- `manager.go:352` `report.RaiseError(..., reportCodePluginCrash, ..., "plugin process exited unexpectedly: "+name, ...)` reads like exit detection and is NOT: it lives in `ProcessManager.Respawn` (`manager.go:324`), whose only caller is `Server.restartPlugin` (`internal/component/plugin/server/reload_tx.go:398`), a config-reload path. Verified by grepping `\.Respawn(` across `internal/` and `cmd/`: one non-test hit. Do not mistake it for a signal that already exists.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/process/process.go` - `startExternal` (`:538`) runs the plugin via `exec.CommandContext` (`:556`), `Start()` (`:593`), sets `running` (`:598`), spawns `relayStderrFrom` and `monitorCmd` (`:602-603`). `monitorCmd` (`:791`) detects exit and emits nothing.
- [ ] `internal/component/plugin/server/dispatch.go` - `deliverEvent` (`:199`) is the single fan-out point; it rejects unregistered events via `events.IsValidEvent` (`:205-209`). `registerSubscriptions` (`:135`) hardcodes `namespace := plugin.DefaultEventNamespace()` (`:148`) because the RPC carries no namespace (`:143-147`).
- [ ] `internal/component/plugin/resolve.go` - `RegisterDefaultEventNamespace` (`:89`) panics on empty (`:91`) or conflicting (`:96`) registration; `DefaultEventNamespace` (`:103`); `RegisterPluginEventTypes` (`:116`) registers into that single default (`:121`) and errors when none is registered (`:125`).
- [ ] `test/scripts/ze_api.py` - `API.wait_for_event(timeout=5.0, predicate=None)` (`:999`) blocks until a matching `deliver-event` arrives, returns `None` on timeout, and is explicitly documented as the substitute for a fixed `time.sleep` (`:1020-1022`). This is the primitive as112 wants, and it requires the observer to be subscribed.

**Behavior to preserve:**
- Both tests keep their runner-side stderr proofs byte-for-byte: `expect=stderr:contains=refusing to start as an external plugin process` (`as112:85`) and `expect=stderr:contains=dynamic per-interface QoS map updates` (`cos:86`), plus `expect=exit:code=0` on both.
- No change to as112's refuse-and-exit behavior or cos's warn-and-continue behavior. This spec adds observability; it must not alter what either plugin does.
- `deliverEvent`'s existing validation and fan-out semantics for every current event.

**Behavior to change:**
- An external plugin's lifecycle becomes observable by a subscribed plugin. Exact surface is the design question below.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- External plugin subprocess reaches startup / emits its warn / exits (`process.go:556-603`, `:791`)

### Transformation Path
1. [fill during design: exit or startup observed in `process.go`]
2. [fill during design: crossing into the plugin server -- `deliverEvent` (`dispatch.go:199`) or another surface]
3. [fill during design: namespace resolution -- `resolve.go:121` is the trap, see R-6]
4. [fill during design: delivery to a subscribed observer plugin; `ze_api.py:999` `wait_for_event` on the test side]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin subprocess ↔ engine | process exit / startup detection in `process.go` | [ ] |
| Engine ↔ Plugin (observer) | event delivery via `deliverEvent` (`dispatch.go:199`) | [ ] |

### Integration Points
- `internal/component/plugin/process/process.go:791` (`monitorCmd`) - the exit-detection site the signal must hook; today it discards `cmd.Wait()`'s error and emits nothing
- `internal/component/plugin/server/dispatch.go:199` (`deliverEvent`) - the existing fan-out point a new event would ride, including its `events.IsValidEvent` gate (`:205-209`)
- `internal/component/plugin/resolve.go:116` (`RegisterPluginEventTypes`) - where a declared event type resolves its namespace (`:121`), the trap R-1 names
- `test/scripts/ze_api.py:999` (`API.wait_for_event`) - the test-side primitive the observer plugin uses

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | An event is the right shape for this signal at all | `spec-fixit-reject-fence-observability.md:220-221` confirms ONLY that the counter does not help; it explicitly does not establish that a separate exit event is the right remedy | The whole design changes: a queryable `show`-style state (mirroring the Phase 1 counter) may fit better than a pub/sub event | Design review before Phase 1 | unvalidated |
| A-2 | One signal cannot serve both tests | `cos-external-warns.ci:74-76` (cos does not exit) vs `as112-external-refuses.ci:85` (as112 refuses and exits) | Scope shrinks to a single event | Re-read both `.ci` docstrings at Phase 0 | confirmed (2026-07-16, by reading both files) |
| A-3 | `spec-fixit-plugin-event-subscription` landing is enough to declare a non-`bgp` event | Its Gaps A/B/C thread namespace through `registerSubscriptions` (`dispatch.go:148`) | This spec must also extend `RegisterPluginEventTypes` (`resolve.go:121`), or ship under `bgp` as debt | Re-check R-6 at Phase 0 (`spec-fixit-plugin-event-subscription.md:475`) | **likely BROKEN** -- `:461-462` names the declaration side as a distinct producer |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | This spec is the "next plugin" R-6 predicts hits `resolve.go:121` silently: the event registers into `bgp` and the observer subscribing to a sane namespace never matches | The observer's `wait_for_event` times out and returns `None` while the plugin demonstrably ran | Re-check R-6 at Phase 0; fix `resolve.go:121` or accept `bgp` knowingly and record it |
| R-2 | The signal fires before the observer subscribes, and the test hangs on a signal already sent | Flaky pass/fail under load | `startup.go:608-611` registers startup subscriptions before `SignalAPIReady` precisely to avoid a first-send race -- mirror that ordering |
| R-3 | A converted test passes without ever fencing, via the `dispatch_until` last-result trap (`ze_api.py:1436-1439`) | Test passes even when the plugin never starts | Mutation-test each conversion: break the plugin, confirm the test goes RED |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| external plugin (as112) refuses and exits | → | emission at the `monitorCmd` exit site (`process.go:791`) → `deliverEvent` (`dispatch.go:199`) → observer plugin | `test/plugin/as112-external-refuses.ci` (observer fences on the signal; `expect=stderr:contains=refusing to start as an external plugin process` retained) |
| external plugin (cos) warns and keeps running | → | startup-reached emission (NOT the exit site — cos never exits) → `deliverEvent` → observer plugin | `test/plugin/cos-external-warns.ci` (observer fences on the signal; `expect=stderr:contains=dynamic per-interface QoS map updates` retained) |

Both rows are `.ci` functional tests by construction: they are the two tests this spec
exists to convert. Neither is deferrable — a row here without a real fence is the whole
defect being fixed.

### Architectural Verification
- [ ] No bypassed layers (the signal crosses engine → plugin via the existing `deliverEvent` fan-out, not a side channel)
- [ ] No unintended coupling (plugin-host lifecycle does not become a BGP concern — see R-1: landing it in the `bgp` namespace IS that coupling)
- [ ] No duplicated functionality (reuse `deliverEvent`/`wait_for_event`; do not invent a second event path)
- [ ] Registration over hardcoding — the event type registers via `Registration.EventTypes` and the engine discovers it; no new per-feature field, switch case, or factory in a core/shared package (`ai/rules/plugin-self-containment.md`). **This is exactly where R-1 bites**: `RegisterPluginEventTypes` (`resolve.go:121`) resolves every declared type into the single `bgp` default, so honest registration currently yields a wrong namespace

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An external plugin subprocess exits | A subscribed observer plugin can deterministically observe it (as112's case) |
| AC-2 | An external plugin reaches startup and keeps running (cos's case) | A subscribed observer can deterministically observe that it started, without waiting for an exit that never comes |
| AC-3 | `as112-external-refuses.ci` runs | Fences on the signal, no `time.sleep`; `expect=stderr:contains=refusing to start as an external plugin process` and `expect=exit:code=0` still pass |
| AC-4 | `cos-external-warns.ci` runs | Fences on the signal, no `time.sleep`; `expect=stderr:contains=dynamic per-interface QoS map updates` and `expect=exit:code=0` still pass |
| AC-5 | Both conversions land | `test/.ci-sleep-baseline` ratcheted 132 -> 130 |
| AC-6 | Each converted test, with the plugin deliberately broken | Goes RED (mutation proof the fence is real, not a `dispatch_until` silent pass) |
| AC-7 | Every existing event | Unchanged delivery and validation semantics |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill during design] | `internal/component/plugin/process/*_test.go` | exit detection emits the signal | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `as112-external-refuses` | `test/plugin/as112-external-refuses.ci` | external plugin refuses to start; operator sees the refusal | |
| `cos-external-warns` | `test/plugin/cos-external-warns.ci` | external plugin warns and keeps running | |

### Interop Tests
N/A — no wire protocol change. Justify at closure.

## Files to Modify
- `internal/component/plugin/process/process.go` - emit at the `monitorCmd` exit site (`:791`) and/or a startup-reached site
- `internal/component/plugin/server/dispatch.go` - delivery, if the event shape is chosen
- `internal/component/plugin/resolve.go` - `:121`, if the namespace fix lands here (see A-3, R-1)
- `test/plugin/as112-external-refuses.ci` - add an observer plugin, drop `time.sleep(4.0)` at `:79`
- `test/plugin/cos-external-warns.ci` - add an observer plugin, drop `time.sleep(4.0)` at `:80`
- `test/.ci-sleep-baseline` - 132 -> 130

## Implementation Steps

### Implementation Phases

Not planned yet: this spec is `skeleton` and blocked on `spec-fixit-plugin-event-subscription`
landing. Phases are deliberately not written, because the shape of Phase 1 depends on two
things that are not settled (A-1: is an event even the right surface; A-3/R-1: does the
dependency leave `resolve.go:121` still broken). Writing phases now would be inventing them.

Ordering that IS already established:

0. **Phase 0 (do this first, before any design):** re-check R-6 in
   `spec-fixit-plugin-event-subscription.md:454-475`, as `:475` instructs. It predicts this
   spec hits the `resolve.go:121` namespace bug. Confirm whether that spec's landing fixed
   the declaration side or only the subscribe side (A-3), and answer A-1.
1. **Observer plugins are independent work and unblocked.** Adding an observer plugin to each
   test needs no signal and no dependency. It can land first and shrink the blocked surface.
2. Signal design + emission (blocked on Phase 0).
3. Test conversion, one test at a time, each mutation-proven (R-3).
4. Ratchet `test/.ci-sleep-baseline` 132 -> 130 only once both conversions are green.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Each converted `.ci` mutation-proven: break the plugin, test goes RED (R-3)

### Completion (BLOCKING — before ANY commit)
- [ ] Phase 0 answered A-1 and A-3 before any code was written
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations
- Blocked on `spec-fixit-plugin-event-subscription` landing, by Thomas's 2026-07-16 ruling. Until then this stays `skeleton`.
- Whether the signal is an event at all is unvalidated (A-1). The source spec confirmed only the negative: the counter does not help.
- The `.ci`-sleep ratchet counts only `test/**/*.ci`, so `dispatch_until`'s own internal `time.sleep` (`ze_api.py:1438`) is exempt. Converting these two tests is a real determinism win, not a ratchet trick — but the ratchet alone would not tell the difference.
