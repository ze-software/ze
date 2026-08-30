# Spec: web-testing-gaps

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The templ migration and the htmx 2 to htmx 4 cutover found defects that no test
could see. Each one rendered correct markup and did nothing in a browser. Some
of those blind spots are now closed. The rows below are the ones that are NOT,
recorded on the owner's instruction so they can be fixed later rather than
rediscovered.

This is a deferral holder. It carries the gaps and the evidence for each. The
session that picks it up writes the acceptance criteria.

Ranked by what each one would catch, not by cost.

| # | Gap | Evidence | Verified by |
|---|-----|----------|-------------|
| G-1 | Nothing asserts that an out-of-band target EXISTS on the page meant to receive it. A swap aimed at an absent element does nothing, silently | `#cli-prompt`, `#cli-path-bar` and `#breadcrumb-bar` are swapped into nothing. Each appears only in fixtures that are themselves out-of-band responses, every one carrying `hx-swap-oob`, and in no page fixture. The layout renders `#breadcrumb` instead. Dead before the cutover and dead after | main thread, against the rendered fixtures |
| G-2 | `attributePattern` in `internal/le/webassets/webassets.go` is `\s(hx-[a-z:-]+)=`, whose class holds no digit. Any htmx attribute carrying a digit is invisible to the per-page asset generator and to the check that reads its output | `hx-status:4xx` is the live case: htmx 4 supports it, and the generator cannot see it. The report that found it counted three copies of that regex | main thread read the producer; the three-copy count is the phase's |
| G-3 | `./le repository tracked-build check` compiles what git holds with `go build`, which compiles no `_test.go`. A commit that breaks only tests leaves HEAD red and the check green | It bit twice in one day. Once when this session deleted a directory three test files still embedded, and once when a closure found four uncommitted files leaving HEAD red on a test-only failure | main thread, both incidents |
| G-4 | The htmx upgrade gate keys on file plus category, so a NEW issue of an explained category in an already-explained file passes silently | 16 explained rows exist. The trade was deliberate: no defect-bearing category has an explained row anywhere, so a new event or extension issue still reds. It is still a hole for inheritance | reported by the closure that made the trade |
| G-5 | Two chaos targets cannot discriminate a dead stream. `#stats` and `#events` carry a 500ms poll BESIDE their SSE swap, so they refresh whether or not the stream works. Any future test written against them is vacuous | Found while choosing a target for the streaming proof, which used the toast container instead because no poll fills it | reported by the SSE phase |
| G-6 | The `.wb` runner cannot kill a server mid-test, so no automated browser proof exists for the branch that fires when a request gets no answer at all | Measured by hand on chaos, and covered by a unit test. No acceptance criterion rests on it | reported by the error-surface phase |
| G-7 | `chaos-stream-swaps.wb` is load-sensitive: it failed once at 3.4x machine load and passes standalone and under normal load | Already journalled in `plan/journal/gate-verdict-depends-on-the-machine.md` | reported by the last phase |
| G-8 | `TestChaosCaptureIsDeterministic/viz-panels` is intermittently red on an `event-time` span | Its fix needs a clock seam, which would move fixtures for a reason the port-fidelity test cannot explain. Deliberately left outside the cutover | reported by the fixture phase |

G-1 and G-2 are the two worth doing first. Each is a mechanical invariant over
artifacts that already exist, each would have caught a live defect, and neither
needs a browser.

G-3 is the widest, because it is not specific to the web interface: it makes
every test-only breakage invisible to the one check that reads what git holds.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - what a guard owes
  → Constraint: a check that cannot see part of its population reports a clean result over the part it read. G-2 and G-4 are both that shape.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: G-5 is the "test whose data reaches the assertion by a different path" trap, already named there.
- [ ] `plan/journal/gate-excludes-part-of-its-population.md` - the recurring class
  → Decision: G-2 and G-4 belong to this class and its row count is the argument for fixing them.

**Key insights:**
- Every gap here is a check that reads a real population and silently reads less of it than it appears to.
- G-1 is the only one that needs a new invariant rather than a widened one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/webassets/webassets.go` - `attributePattern` is the G-2 producer, and the generator's whole view of a template's htmx usage passes through it.
- [ ] `internal/component/web/testdata/golden/` - the rendered fixtures that prove G-1: an out-of-band target that appears in no page fixture is swapped into nothing.

**Behavior to preserve:**
- The per-page asset sets and the fixture check that verifies them from the opposite direction. Both are new and both work.
- The htmx upgrade gate's zero count.

**Behavior to change:**
- To be decided by the session that implements this. Nothing here is designed yet.

## Data Flow (MANDATORY)

### Entry Point
- `./le repository generate` and `./le verify current mode full`, for the checks that would carry these invariants.

### Transformation Path
1. To be filled by the implementing session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Source scan ↔ rendered fixture | the two directions that already verify the asset sets | No |

### Integration Points
- `internal/le/webassets/webassets.go` and the markup checks that read its output.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The three targets in G-1 are genuinely dead rather than reached by a path the fixtures do not capture | grep over the rendered fixtures: each appears only inside an element carrying `hx-swap-oob` | G-1 becomes a false positive and its check would refuse working markup | Trace one of the three from its handler to a page that renders it, and fail to find one | unvalidated |
| A-2 | Widening `attributePattern` to accept digits breaks no existing derivation | the pattern is a scan, and a wider scan finds a superset | the generator emits assets a page does not need, which costs bytes and reds the fixture check | Widen it, regenerate, and diff the per-page sets | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The G-1 check refuses an out-of-band target that a page renders conditionally, in a branch no fixture exercises | the check reds on a target an operator can see working | Derive the page set from the source graph, which over-approximates, rather than from fixtures, which under-approximate. That is the split the asset generator already uses |
| R-2 | Fixing G-3 means compiling test files for what git holds, which is slower than `go build` | the check's runtime grows past its current 45 seconds | Measure before choosing. `go vet` compiles tests and may be enough |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A check refuses correct markup, or keeps passing over a population it cannot see. Nothing user-facing: these are all build-time and test-time surfaces. |
| How is it reverted? | Single commit revert per gap. Each is independent of the others. |
| Who else touches this path? | The web, lg and chaos packages are edited by several sessions. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify current mode full` | → | the check that closes G-1 | to be named by the implementing session |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | To be written when this spec is picked up | Each of G-1 to G-8 is either closed by a check or explicitly declined with a reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be named | to be named | G-1 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A until the design exists | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be named | `test/web/*.wb` | an out-of-band swap reaches a target the page actually renders | |

## Files to Modify
- `internal/le/webassets/webassets.go` - the G-2 pattern
- the markup checks that read the fixtures - the G-1 invariant

## Files to Create
- to be decided by the implementing session

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | No command added |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | Yes | `test/web/*.wb` for the G-1 invariant |
| Pipe completeness | N-A | No CLI output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No runtime dependency added |
| Prometheus counters/metrics | N-A | No observable runtime state |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Build and test surfaces only |
| 2 | Config syntax changed? | No | None touched |
| 3 | CLI command added/changed? | No | None touched |
| 4 | API/RPC added/changed? | No | None touched |
| 5 | Plugin added/changed? | No | None touched |
| 6 | Has a user guide page? | No | Nothing an operator sees changes |
| 7 | Wire format changed? | No | None touched |
| 8 | Plugin SDK/protocol changed? | No | None touched |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs this |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | No comparable feature |
| 12 | Internal architecture changed? | No | Checks only |
| 13 | Route metadata keys added/changed? | No | None touched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on each file changed |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example covers these checks |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- to be written when this spec is picked up
   - Tests: the Wiring Test row above, once named
   - Files: to be decided
   - Verify: the check exists and fails on a known-dead target

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | Each new check fails closed. A population it cannot read MUST NOT report clean |
| Correctness | The G-1 check derives its page set from source, which over-approximates, not from fixtures, which under-approximate |
| Rule: `ai/rules/evidence.md` | Every widened check is driven from its entry point in a test, not only from its helper |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| G-1 closed | the check reds on `#cli-prompt` before the dead swap is removed |
| G-2 closed | the pattern matches `hx-status:4xx` and the per-page sets are unchanged otherwise |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| N-A | These are build-time and test-time checks; none reads request data |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| A widened check reds on correct markup | The check is wrong, or its population is wider than its rule. Re-read `ai/rules/evidence.md` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Record the gaps as a skeleton deferral holder rather than fix them now | Fix each as it was found | Each was found mid-cutover, and folding them in would have cost that commit its single focus and restarted its gates |

## Known Limitations
- Nothing here is designed. The Task table is the deliverable; the rest is the shape a later session fills.
- G-7 and G-8 already have journal rows and may be closed there rather than here.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
