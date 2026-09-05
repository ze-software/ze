# Spec: an open DDoS incident is never resolved on teardown

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An open DDoS incident stays open forever when the detector is disabled or the
daemon stops. The incident is reported to the external reporting service, and
nothing ever closes it.

Found by an ad-hoc audit of `ddos detect` on 2026-08-15. Three independent
breaks, any one of them sufficient, in the source spec's own words:

**"An open incident is never resolved when the detector is disabled or the daemon
stops."** Two independent breaks, either sufficient. (a) The detector's teardown
is `(*detector).Stop` (`internal/plugins/ddos/detect/detector.go`): it sets
`stopped`, cancels, waits, and calls `saveBaseline`. It never calls `emitCleared`,
which is assigned only to `d.sm.OnCleared` (`detector.go`) and so fires only on a
rate transition through the state machine. No `AttackCleared` is published, so the
reporting plugin's Cleared handler never runs. (b) The one path that would close
it, the reporting plugin's own `OnConfigApply`, does call `resolveIncident` first,
but it is not invoked for this diff: `(*Server).reloadConfig`
(`internal/component/plugin/server/reload.go`) selects plugins by strict
root-prefix match on declared roots, and the reporting plugin declares a single
`ConfigRoots` entry for its own subtree, so a diff under `ddos/detect` reaches it
with zero sections. Same class, third instance: `runEngine`'s exit path calls
`unsubscribe()` only, so a clean daemon stop with an attack in progress also
leaves the incident open.

The reporting plugin is `internal/plugins/ddos/flowtriq`. Its `configRoot` is
`ddos/flowtriq` and the detector's is `ddos/detect`, so the strict prefix match
in `reload.go` cannot route one plugin's diff to the other. Verified at the
producer on 2026-09-05: the match is `key == root || strings.HasPrefix(key, root +
config.PathSep)`, chosen deliberately so `ddos/local` does not swallow a sibling
key that merely starts with the same letters.

An incident left open at the reporting service is an operator-visible wrong
answer that outlives the process. The service shows an attack in progress against
a router that has been off for a week.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/plugins.md` - the DDoS plugins and the events between them
  → Constraint: [fill at design time]
- [ ] `docs/architecture/api/process-protocol.md` - config apply and the plugin lifecycle
  → Constraint: a plugin is handed only the diff sections under its declared roots

**Key insights:** (minimal context to resume after compaction)
- Three exits leak the incident: detector `Stop`, a `ddos/detect` config diff, and `runEngine`'s clean exit
- The reporting plugin's own `OnConfigApply` already calls `resolveIncident`; it is simply never invoked for a detector-side diff

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ddos/detect/detector.go` - `(*detector).Stop` takes the mutex, sets `stopped`, calls `d.cancel()`, waits on the wait group, and calls `d.saveBaseline()`. It never calls `emitCleared`. `emitCleared` is assigned once, at `d.sm.OnCleared = d.emitCleared`
- [ ] `internal/plugins/ddos/detect/state.go` - `OnCleared` is invoked only from the state machine's rate transition, so a teardown that never moves the rate never fires it
- [ ] `internal/plugins/ddos/detect/register.go` - `configRoot` is `ddos/detect`, declared as the plugin's single `ConfigRoots` entry. The engine's exit path calls `unsubscribe()` and returns
- [ ] `internal/plugins/ddos/flowtriq/register.go` - `configRoot` is `ddos/flowtriq`, a single `ConfigRoots` entry, and `OnConfigApply` posts `resolveIncident` before it re-reads its config
- [ ] `internal/plugins/ddos/flowtriq/reporter.go` - the reporter calls `r.cl.resolveIncident(r.uuid, duration, r.peakPPS, r.peakBPS, r.confidence)`, which is the only close the service ever sees
- [ ] `internal/component/plugin/server/reload.go` - the diff is filed to a plugin only when `key == root` or `strings.HasPrefix(key, root + config.PathSep)`, so a `ddos/detect` key reaches the `ddos/flowtriq` plugin with zero sections

**Behavior to preserve:**
- The strict root-prefix match, which exists so `ddos/local` does not swallow a sibling key
- `resolveIncident` staying the reporting plugin's own call, not the detector's

**Behavior to change:**
- Every exit that ends attack detection must close an open incident

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The detector is torn down: by `(*detector).Stop` on reconfigure or shutdown, by a config diff under `ddos/detect` that disables it, or by the engine's clean exit at daemon stop.
- Format at entry: no message at all, which is the defect. The reporting plugin is waiting for an `AttackCleared` event that is never published.

### Transformation Path
1. Teardown runs in `internal/plugins/ddos/detect`
2. `emitCleared` would advance the attack generation and stage the Cleared event
3. The `AttackCleared` event would reach `internal/plugins/ddos/flowtriq`
4. The reporter would call `resolveIncident` against the external service

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| detect plugin ↔ flowtriq plugin | the `AttackCleared` event over the plugin bus | No |
| Plugin host ↔ plugin | `OnConfigApply` with the sections under the declared roots | No |

### Integration Points
- `(*detector).emitCleared` (`detector.go`) - the publisher every exit must reach
- `(*Server).reloadConfig` (`reload.go`) - the selection that decides which plugin sees a diff

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Cleared event can still be delivered after `d.cancel()` has run | `Stop` cancels before it would publish | The close is staged and dropped, and the fix is silent | Read the publish path against the cancelled context | unvalidated |
| A-2 | A daemon killed rather than stopped cleanly is out of scope for this spec | The three named exits are all orderly | The service still shows stale incidents after a crash | Ask the owner whether a service-side timeout is expected | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Publishing Cleared on every reconfigure closes an incident that is still live, and the next tick opens a second one | Incident churn at the service on each config change | Distinguish a teardown that ends detection from one that resumes it |
| R-2 | Widening the config-root selection so one plugin sees another's diff breaks the isolation the strict match protects | An unrelated plugin reacting to a foreign key | Publish an event rather than widening the selection |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The external reporting service shows a stale open attack, or churns incidents on every config change |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/immediate/spec-fixit-ddos-baseline-restore-staleness.md` touches `(*detector).Stop` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `(*detector).Stop` with an attack in progress | → | `emitCleared` and the reporter's `resolveIncident` | [test name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The detector is stopped with an attack in progress | An `AttackCleared` is published and the reporter resolves the incident |
| AC-2 | A config diff under `ddos/detect` disables the detector during an attack | The incident is resolved |
| AC-3 | The daemon stops cleanly during an attack | The incident is resolved |
| AC-4 | The detector is stopped with no attack in progress | Nothing is published and no incident is created |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Disables DDoS detection while an attack is being reported | config diff → detector teardown → AttackCleared → reporter → service | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill at design time] | `internal/plugins/ddos/detect/detector_test.go` | teardown publishes Cleared exactly when an attack is open | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | N-A | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill at design time] | `test/plugin/*.ci` | An operator disables detection and the incident closes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is plugin, no wire-visible change | |

## Files to Modify
- `internal/plugins/ddos/detect/detector.go` - the teardown path that must publish Cleared
- `internal/plugins/ddos/detect/register.go` - the engine exit path that calls `unsubscribe()` alone
- `internal/plugins/ddos/flowtriq/register.go` - the handler that closes the incident

## Files to Create
- [fill at design time]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | `test/plugin/*.ci` |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | incidents opened and resolved |
| BGP family surface (new SAFI / capability / attribute) | | N-A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/immediate/spec-fixit-ddos-incident-lifecycle-on-teardown.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- reach `emitCleared` from the teardown path, write failing wiring tests
   - Tests: [wiring test names]
   - Files: `detector.go`
   - Verify: the teardown can publish; the wiring test fails because the reporter still sees nothing
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Data flow | The close travels as an event; the config-root selection is not widened |
| Rule: `ai/rules/principles.md` | No exit path silently drops the Cleared event |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| All three exits publish Cleared | one test per exit path |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | The resolve call to the external service must not carry more than the incident it closes |

### Failure Routing
| Failure | Route To |
|---------|----------|
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [fill at closure]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
