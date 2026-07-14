# Spec: payload-predicate-waits

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | 8/8 |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/testing/ci-format.md` - `.ci` directive catalog and engine-step model
4. `internal/test/runner/engine_steps.go` - engine-step parse + executor (Go-side extension target)
5. `test/scripts/ze_api.py` - Python observer SDK (Python-side primitives target)

## Task

Deliver **Layer 2: payload-predicate waits** so `.ci` functional tests can wait until
a *specific observed payload matches a predicate*, instead of `time.sleep` + single-shot
assert. This is the sleep-elimination layer above the already-shipped `request quiesce`
barrier (Layer 1). Two symmetric surfaces:

- **Python observer SDK** (`test/scripts/ze_api.py`): arbitrary Python-callable predicates
  for embedded observers. Add a generic predicate poll (`wait_until`), a query-result
  predicate poll (`dispatch_until`), and give `wait_for_event` the optional predicate the
  docs already promise.
- **Go engine-steps** (`internal/test/runner/engine_steps.go`): a richer *declarative*
  predicate grammar for first-class `.ci` engine steps, extending `expect=output` /
  `expect=stream` from a single `contains=` substring to `contains` + `matches` (regex) +
  `absent` (negation) + `json` (path=value over JSON output).

Prove both with conversions that also lower the `time.sleep` ratchet, then wire discovery
so future agents find the primitives.

Scope confirmed by user (2026-07-13): "primitive + proof + extend go testing framework",
maximal Go grammar (`contains + matches + absent + json`). Full migration of the remaining
~456 sleeps is explicitly NOT part of this spec (it is the separate "migrate .ci sleeps" effort).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/testing/ci-format.md` - authoritative `.ci` directive catalog; engine-step directives documented here
  → Decision: new predicate keys (`matches=`/`absent=`/`json=`) are documented as extensions of the existing `expect=output`/`expect=stream` grammar, not new directives
  → Constraint: `command=`/`stream=` and `expect=output`/`expect=stream` keep their raw remainder (colons preserved); the generic `action:key=value` splitter must NOT be applied to them
- [ ] `ai/rules/testing.md` - Python Observer API table + sleep-ratchet rule
  → Constraint: line 405 documents `wait_for_event(predicate)` which does not exist yet; this is doc-vs-code drift to fix, not a new claim to invent
  → Constraint: the ratchet counts `time.sleep(` only in `test/**/*.ci`; sleeps inside `ze_api.py` are exempt, so polling helpers may sleep internally
- [ ] `ai/rules/functional-test-gate.md` - which test dir is required per change type
  → Constraint: plugin behavior → `.ci` in `test/plugin/`; the proof conversions and the new Go-grammar test satisfy this

### RFC Summaries (MUST for protocol work)
- [ ] N/A - this is test infrastructure, no wire protocol behavior changes

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- The engine-step executor re-dispatches `lastCommand` and matches a fixed substring; the Go extension generalizes the *match*, not the re-dispatch loop.
- The Python side already has `dispatch_until_done` (fixed `status=="done"` predicate) and `wait_for_event` (no predicate) — both are the narrow forms of the general primitives.
- Sleeps live in embedded Python observers doing sleep+show+assert, and in hand-rolled predicate loops (kernel/state polls) that use `wait_for_event` merely as a backoff.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/test/runner/engine_steps.go` (335L) - EngineStep struct + parse + RunEngineSteps executor
  → Constraint: `EngineStepExpectOutput` (`:275-295`) loops `for !strings.Contains(lastOutput, step.Text)` re-dispatching `lastCommand` every 200ms until timeout; predicate is a single substring
  → Constraint: `EngineStepExpectStream` (`:324-331`) matches `strings.Contains(e, needle)` over delivered stream events
  → Constraint: `parseEngineExpectContains` (`:156-176`) strips a trailing `:timeout=` via `strings.LastIndex`, then requires a `contains=` prefix; colons inside the needle are preserved
  → Constraint: `lastOutput` (`:273`) is `status + " " + data`; a `json=` predicate needs the raw `data` alone, so the executor must track `lastData` separately
  → Constraint: `EngineStep` struct fields (LSP-confirmed) are Kind, Text, Namespace, Name, Timeout; JSON is regenerated per test run (runner writes `engine-steps.json`), so adding fields is not a cross-version serialization concern
- [ ] `test/scripts/ze_api.py` (1557L) - Python observer SDK
  → Constraint: `dispatch_until_done` (`:951-973`) polls `dispatch(cmd)` until `result["status"]=="done"`, `time.sleep(delay)` between attempts; 12 callers depend on the name/behavior
  → Constraint: `wait_for_event` (`:975-999`) returns the first event JSON string of ANY subscribed type; no predicate parameter (docstring flags the caveat at `:982-985`)
  → Constraint: `dispatch` (`:938-949`) returns the `{"result": ...}` envelope; helpers extract `(resp or {}).get("result", {}) or {}`
  → Constraint: sleeps inside this module are ratchet-exempt (comment `:960-962`)
- [ ] `internal/test/runner/record_parse.go` - `.ci` line dispatcher
  → Constraint: `expect=output:`/`expect=stream:` are special-cased BEFORE the generic `:` split (so their `contains=` needle may hold colons) and routed to `parseEngineExpectContains`; the new keys are parsed inside that function, so record_parse routing is unchanged
- [ ] `test/plugin/fib-recursive.ci` (180L) - proof-conversion target #1
  → Constraint: 2 `time.sleep` — `:43` in `wait_for_rib` (poll `show bgp rib status` for `status=="done"`), `:71` a fixed 2.0s before `show rib` + assert `172.16.0.0/16` present
- [ ] `test/plugin/forked-route-install-kernel.ci` (164L) - proof-conversion target #2
  → Constraint: 0 `time.sleep`; two hand-rolled 40-iteration loops (`:70-75`, `:88-92`) polling KERNEL state `kernel_has_ze_route()` (shells `ip route show`), using `wait_for_event(timeout=0.25)` only as a backoff. Neither a dispatch nor an event predicate fits — a generic `wait_until(predicate)` does
- [ ] `internal/test/runner/engine_steps_test.go` (13K) - existing Go unit tests; extend for new predicate kinds

**Behavior to preserve:** (unless user explicitly said to change)
- `dispatch_until_done(cmd)` name, signature, and behavior — the 12 callers must not change.
- `wait_for_event(timeout=...)` with no predicate returns the first event of any type (today's behavior). The predicate parameter is optional and defaults to None.
- `expect=output:contains=<substr>` and `expect=stream:contains=<substr>` behavior unchanged (contains is the default predicate kind).
- All existing engine-step `.ci` tests (`expect=output:` in 6 files, `expect=event:`/`expect=stream:`) keep passing.
- The engine-step re-dispatch loop (200ms interval, timeout error format) is preserved; only the *match test* generalizes.

**Behavior to change:** (only if user explicitly requested)
- Add richer predicate grammar to engine-steps (`matches`/`absent`/`json`) — user requested.
- Add `predicate` param to `wait_for_event` — closes documented drift, user-approved.
- Add `wait_until` and `dispatch_until` primitives — user requested "primitive".

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Python surface:** a `.ci` `tmpfs=*.run` observer imports `ze_api` and calls `wait_until` / `dispatch_until` / `wait_for_event(predicate)`.
- **Go surface:** a `.ci` `expect=output:matches=|absent=|json=...` (or `expect=stream:matches=`) line, parsed by the runner into `engine-steps.json`, executed by the spawned `ze-test engine-steps` executor.

### Transformation Path
1. **Go parse:** `record_parse.go` routes `expect=output:`/`expect=stream:` to `parseEngineExpectContains` → determines predicate kind + operand(s) → `EngineStep{Kind, Match, Text, Path, Timeout}`.
2. **Go serialize:** `MarshalEngineSteps` writes `engine-steps.json` in the test tmpfs.
3. **Go execute:** the spawned executor's `RunEngineSteps` re-dispatches `lastCommand`, evaluates the predicate against `lastOutput`/`lastData` until true or timeout.
4. **Python:** observer calls the primitive; the primitive polls `dispatch`/kernel/event and applies the Python predicate; internal `time.sleep` is ratchet-exempt.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` file ↔ runner | new predicate keys parsed in `parseEngineExpectContains` | [ ] |
| runner ↔ engine-steps executor | `engine-steps.json` (new `Match`/`Path` fields) | [ ] |
| observer ↔ engine | `dispatch-command` RPC (unchanged); predicate applied observer-side | [ ] |

### Integration Points
- `EngineStep` struct — add `Match` (predicate kind) and `Path` (json path) fields.
- `RunEngineSteps` `EngineStepExpectOutput`/`EngineStepExpectStream` — branch on `Match`.
- `ze_api.API` — add `wait_until`, `dispatch_until`; extend `wait_for_event`; refactor `dispatch_until_done`.

### Architectural Verification
- [ ] No bypassed layers (predicate evaluated at the same points as today's contains/status checks)
- [ ] No unintended coupling (test infra only; no production `internal/`/`cmd/` behavior change)
- [ ] No duplicated functionality (`dispatch_until_done` becomes a wrapper; contains stays the default)
- [ ] Zero-copy preserved where applicable (N/A — test infra)
- [ ] Registration over hardcoding — N/A (extends an existing directive grammar and SDK, no new registry surface)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `expect=output:`/`expect=stream:` are routed to `parseEngineExpectContains` before the generic `:` split | Explore sweep cited `record_parse.go:241-246`; not personally read | new keys truncated at first `:` | read `record_parse.go` during audit; add a parse unit test with a colon-bearing operand | **confirmed** — `record_parse.go:241-246` `CutPrefix`-intercepts both before the generic `:` split at `:250` |
| A-2 | `show rib` returns a JSON array of objects each with a `prefix` key | `fib-recursive.ci:78-81` uses `result_json_data` + `e.get('prefix')` | dispatch_until predicate for the proof never matches | run converted fib-recursive.ci | **confirmed** — `fib-recursive.ci:78-83` already passes asserting `e.get('prefix')=='172.16.0.0/16'` over `result_json_data(show rib)` |
| A-3 | `EngineStep` is regenerated per run, so new fields need no back-compat default beyond "" → contains | `engine_steps.go` doc `:12-14` (runner serializes per test) | old json files break | grep for any committed `engine-steps.json`; default `Match==""` to contains | **confirmed** — `git ls-files` shows no committed `engine-steps.json`; new fields use `omitempty`, `Match==""`→contains |
| A-4 | No Python unit-test harness exists for `ze_api.py`; primitives are covered by `.ci` functional tests + Go engine-steps unit tests | to confirm in audit | AC test rows wrong | audit: `ls test/scripts` + grep for a pytest/`_test` | **confirmed** — no pytest/unittest in `test/scripts/`; coverage is `.ci` functional + Go unit tests |
| A-5 | `request bgp rib inject` + `show rib` are dispatchable from an engine-steps executor without a live BGP peer | `fib-recursive.ci` uses both via the observer; engine-steps runs from OnAllPluginsReady | new Go-grammar test needs a peer/redesign | prototype the new `.ci` during Phase 3 | **confirmed** — `test/plugin/engine-steps-predicates.ci` dispatches `request bgp rib inject`/`withdraw` + `show rib` from the engine-steps executor with no live peer; PASS on first run, 3x stable |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `absent=` on a never-populated output passes instantly (false green) | a withdrawal test passes even if injection failed | require a preceding `contains=`/inject step so absent proves a transition, not a vacuous truth; document the ordering in ci-format.md |
| R-2 | `json=` path walker mis-handles arrays/missing keys → confusing timeout errors | opaque "path not found" at timeout | precise error naming the path + the actual JSON (truncated), mirror existing `%.200q` error style; boundary unit tests for missing key/out-of-range index/non-JSON |
| R-3 | regex `matches=` compile error surfaces only at runtime (timeout) | test hangs to timeout on a bad regex | compile the regex once at parse time (`parseEngineExpectContains`) and fail the parse, not the run |
| R-4 | Adding `wait_until` grows scope beyond "two primitives" | review pushes back | it is the foundation the forked proof needs; keep it minimal (bool return, internal backoff) and documented; drop if user rejects |
| R-5 | Converting fib-recursive changes an existing passing test's mechanics | flaky/regressed fib-recursive | keep the assertions identical; only replace the wait mechanism; run the test pre/post |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` observer calls `dispatch_until("show rib", pred)` | → | `API.dispatch_until` (`ze_api.py`) | `test/plugin/fib-recursive.ci` (converted) |
| `.ci` observer calls `wait_until(pred)` | → | `API.wait_until` (`ze_api.py`) | `test/plugin/forked-route-install-kernel.ci` (converted) |
| `.ci` observer calls `wait_for_event(predicate=pred)` | → | `API.wait_for_event` predicate branch | `test/plugin/event-predicate-wait.ci` (new; echo-peer reflects an announced UPDATE, predicate matches it by prefix) |
| `.ci` `expect=output:matches=<regex>` | → | `RunEngineSteps` matches branch | `test/plugin/engine-steps-predicates.ci` (new) |
| `.ci` `expect=output:absent=<substr>` | → | `RunEngineSteps` absent branch | `test/plugin/engine-steps-predicates.ci` (new) |
| `.ci` `expect=output:json=<path>=<value>` | → | `RunEngineSteps` json branch + path walker | `test/plugin/engine-steps-predicates.ci` (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `wait_until(pred, attempts, delay)` where `pred()` becomes true on attempt k≤attempts | returns True; stops polling at k; returns False if never true within attempts |
| AC-2 | `dispatch_until(cmd, pred)` where `pred(result)` true on attempt k | returns the winning `result` dict; returns the last `result` after attempts exhausted (caller checks) |
| AC-3 | `dispatch_until_done(cmd)` | unchanged behavior; implemented as `dispatch_until(cmd, lambda r: r.get("status")=="done")`; all 12 existing callers pass |
| AC-4 | `wait_for_event(predicate=pred)` with stream of events | returns the first event whose decoded form satisfies `pred`; `predicate=None` returns the first event of any type (unchanged) |
| AC-5 | `expect=output:matches=<regex>` | re-dispatches until the regex matches `lastOutput` or timeout; bad regex fails at PARSE time with a clear error |
| AC-6 | `expect=output:absent=<substr>` | re-dispatches until `lastOutput` no longer contains `<substr>` or timeout (for withdrawal) |
| AC-7 | `expect=output:json=<path>=<value>` | re-dispatches until the dotted `<path>` into the JSON `data` stringifies to `<value>`; missing path / non-JSON → clear error at timeout |
| AC-8 | `expect=stream:matches=<regex>` | matches a delivered stream event by regex (contains + matches supported on stream; absent/json are output-only, documented) |
| AC-9 | fib-recursive.ci converted | 2 `time.sleep` removed; test still passes with identical assertions. **Baseline correction (user-approved 2026-07-13):** the committed baseline (456) was STALE — the true HEAD `.ci` sleep count is 462. Removing 2 → 460, so the baseline is set to **460** (462−2), not 454. AC-9's original "456→454" arithmetic assumed baseline==actual==456, which was false (pre-existing debt). Ratchet passes: 460 ≤ 460 |
| AC-10 | Discovery updated | `ai/rules/testing.md:405` drift fixed + `dispatch_until`/`wait_until` rows added; `docs/architecture/testing/ci-format.md` documents the new predicate grammar; `docs/functional-tests.md` + `ai/INDEX.md` updated |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | writes an observer that waits for a route to appear in the RIB | `dispatch_until("show rib", pred)` → poll `dispatch-command` → predicate over result JSON | `test/plugin/fib-recursive.ci` |
| 2 | writes an observer that waits for a kernel-FIB state transition | `wait_until(lambda: kernel_has_ze_route(p))` → poll arbitrary predicate | `test/plugin/forked-route-install-kernel.ci` |
| 3 | writes a first-class `.ci` that polls a `show` for a JSON field | `expect=output:json=path=value` → re-dispatch `command=` → json path walk | `test/plugin/engine-steps-predicates.ci` |
| 4 | writes a first-class `.ci` that waits for a withdrawal | `expect=output:absent=prefix` after inject+withdraw `command=` steps | `test/plugin/engine-steps-predicates.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseEngineExpectMatches` | `internal/test/runner/engine_steps_test.go` | `matches=` parses to `Match="matches"`; bad regex fails parse | |
| `TestParseEngineExpectAbsent` | `internal/test/runner/engine_steps_test.go` | `absent=` parses to `Match="absent"`, needle preserved (colons ok) | |
| `TestParseEngineExpectJSON` | `internal/test/runner/engine_steps_test.go` | `json=path=value` parses to `Match="json"`, `Path` + `Text` split at first `=`; IPv6-colon value preserved | |
| `TestRunEngineStepsMatchesRegex` | `internal/test/runner/engine_steps_test.go` | executor polls until regex matches; timeout error names the regex | |
| `TestRunEngineStepsAbsent` | `internal/test/runner/engine_steps_test.go` | executor polls until substring absent; passes only after a present→absent transition in the fake dispatch | |
| `TestRunEngineStepsJSONPath` | `internal/test/runner/engine_steps_test.go` | path walk over object/array; value match; missing key / OOR index / non-JSON → error | |
| `TestRunEngineStepsContainsUnchanged` | `internal/test/runner/engine_steps_test.go` | `Match==""`/`"contains"` behaves exactly as before | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| json array index in path | 0 .. len-1 | len-1 | N/A (negative rejected) | len (out of range → error, not panic) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-recursive` | `test/plugin/fib-recursive.ci` | observer waits for recursive route in RIB via `dispatch_until` (no sleep) | |
| `forked-route-install-kernel` | `test/plugin/forked-route-install-kernel.ci` | observer waits for kernel route appear/disappear via `wait_until` | |
| `engine-steps-predicates` | `test/plugin/engine-steps-predicates.ci` | first-class `.ci` uses `matches=`/`absent=`/`json=` against `show` after inject/withdraw | PASS (native, 3x stable) |
| `event-predicate-wait` | `test/plugin/event-predicate-wait.ci` | observer subscribes to bgp update events, announces a route to an echo-mode ze-peer, and `wait_for_event(predicate)` matches the reflected update by prefix | PASS (native, 3x stable) |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | test infrastructure, no wire protocol change | - |

### Future (if deferring any tests)
- None. The broad sleep migration (~454 remaining) is a separate effort, not deferred test coverage of THIS feature.

## Files to Modify
- `internal/test/runner/engine_steps.go` - `EngineStep` fields (`Match`, `Path`), `parseEngineExpectContains` (parse new keys + compile regex), `RunEngineSteps` (predicate branches + `lastData`), a `matchEngineOutput` helper + json path walker (split to `engine_predicate.go` if the file would exceed ~600L per `rules/file-modularity.md`)
- `internal/test/runner/engine_steps_test.go` - new unit tests above
- `test/scripts/ze_api.py` - add `wait_until`, `dispatch_until`; extend `wait_for_event(predicate=None)`; refactor `dispatch_until_done` to a wrapper; module-level convenience `wait_until`/`dispatch_until`
- `test/plugin/fib-recursive.ci` - convert 2 sleeps → `dispatch_until_done` + `dispatch_until`; drop `import time`
- `test/plugin/forked-route-install-kernel.ci` - convert hand-rolled loops → `wait_until`
- `test/.ci-sleep-baseline` - 456 → 460 (see AC-9: committed baseline was stale vs the true HEAD count of 462; −2 from the fib-recursive conversion → 460; user-approved 2026-07-13)
- `docs/architecture/testing/ci-format.md` - document `matches=`/`absent=`/`json=` predicate grammar (+ absent/json output-only note, R-1 ordering caveat)
- `docs/functional-tests.md` - Python SDK primitives + engine-step predicate grammar
- `ai/rules/testing.md` - fix `:405` drift (`wait_for_event(predicate)`); add `wait_until`/`dispatch_until` rows
- `ai/INDEX.md` - keyword rows for payload-predicate waits / `dispatch_until` / engine-step predicates

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Test infrastructure docs | [ ] yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` |
| Discovery updates (INDEX + rules) | [ ] yes | `ai/INDEX.md`, `ai/rules/testing.md` (per `ai/rules/discovery-updates.md`) |
| Functional test for new behavior | [ ] yes | `test/plugin/engine-steps-predicates.ci` + converted proofs |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (test-infra only) | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes (Python observer SDK) | `docs/functional-tests.md`, `ai/rules/testing.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` |
| 16 | Any changed source file referenced by doc source anchors? | Check | grep `docs/` for `source:` of `engine_steps.go`/`ze_api.py` |
| 17 | Existing docs show examples for this area? | Yes | ci-format.md engine-step examples |

## Files to Create
- `internal/test/runner/engine_predicate.go` - (only if engine_steps.go would exceed ~600L) predicate matcher + json path walker
- `test/plugin/engine-steps-predicates.ci` - functional test for `matches=`/`absent=`/`json=`
- `test/plugin/event-predicate-wait.ci` - functional test for `wait_for_event(predicate)` via an echo-mode ze-peer (AC-4)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, validate A-1..A-5 |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify-changed` scoped to changed tests |
| 6-9. Critical review + fixes | Critical Review Checklist |
| 10. Deliverables | Deliverables Checklist |
| 11. Security | Security Review Checklist |
| 12. Documentation | Documentation Update Checklist |
| 13. /ze-review gate | Review Gate section |
| 14. Summary + close | Executive Summary; two-commit closure |

### Implementation Phases

1. **Phase: Audit + validate assumptions (no code beyond tests)** — read `record_parse.go` (A-1), confirm `show rib` JSON shape (A-2), check for committed `engine-steps.json` (A-3) + Python test harness (A-4). Prototype whether the new Go-grammar `.ci` needs a peer (A-5).
2. **Phase: Go parse (TDD)** — extend `EngineStep`; generalize `parseEngineExpectContains` to `contains`/`matches`/`absent`/`json`; compile regex at parse time.
   - Tests: `TestParseEngineExpectMatches`, `TestParseEngineExpectAbsent`, `TestParseEngineExpectJSON`
3. **Phase: Go execute (TDD)** — `matchEngineOutput` helper + json path walker; track `lastData`; branch in `RunEngineSteps`; preserve contains default + error formats.
   - Tests: `TestRunEngineStepsMatchesRegex`, `TestRunEngineStepsAbsent`, `TestRunEngineStepsJSONPath`, `TestRunEngineStepsContainsUnchanged`
4. **Phase: Python primitives (TDD via functional `.ci`)** — add `wait_until`, `dispatch_until`, `wait_for_event(predicate)`; refactor `dispatch_until_done` to wrapper; module-level convenience functions.
5. **Phase: Proof conversions + Go-grammar functional test** — convert fib-recursive + forked-route-install-kernel; create `engine-steps-predicates.ci`; lower baseline 456→454.
6. **Phase: Discovery + docs** — fix `testing.md:405`; document grammar in ci-format.md + functional-tests.md; `ai/INDEX.md` rows.
7. **Full verification** → `make ze-verify` (or `ze-verify-changed` if known-red baseline).
8. **Complete spec** → audit, learned summary `plan/learned/NNN-payload-predicate-waits.md`, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | absent= is non-vacuous (R-1); json path walker handles arrays/missing/non-JSON (R-2); regex compiled at parse (R-3) |
| Back-compat | `contains=` default unchanged; `dispatch_until_done` + its 12 callers unchanged; `wait_for_event()` no-predicate unchanged |
| Naming | predicate keys `contains`/`matches`/`absent`/`json` consistent with existing `contains=`; Python `wait_until`/`dispatch_until` consistent with `dispatch_until_done` |
| Data flow | predicate evaluated at the same re-dispatch point; no new engine coupling |
| Rule: no-layering | `dispatch_until_done` body fully replaced by the wrapper (no dead duplicate loop) |
| Rule: file-modularity | split `engine_predicate.go` if engine_steps.go > 600L |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Python primitives exist | `grep -n 'def wait_until\|def dispatch_until\b\|predicate' test/scripts/ze_api.py` |
| Go grammar parses | `go test ./internal/test/runner/ -run TestParseEngineExpect` |
| Go grammar executes | `go test ./internal/test/runner/ -run TestRunEngineSteps` |
| Proofs pass | run fib-recursive.ci, forked-route-install-kernel.ci, engine-steps-predicates.ci |
| Ratchet lowered | `cat test/.ci-sleep-baseline` == 454; `make ze-verify-wiring-docs` green |
| Docs synced | `ai/rules/testing.md:405` no longer lists a non-existent API; ci-format.md has the grammar |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | regex from `.ci` compiled with error handling (no panic); json path parsing bounds-checked (no index panic) |
| Resource exhaustion | predicate polls are timeout/attempts-bounded (no unbounded loop); test-only code, not reachable from production |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Parse test fails | Phase 2 |
| Executor test fails | Phase 3 |
| Proof `.ci` fails | check predicate vs actual output; Phase 5 |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The committed `.ci`-sleep baseline (456) equals the actual HEAD count, so removing 2 → 454 (AC-9) | The true HEAD `.ci` sleep count is 462; the baseline (456) was stale by 6 (pre-existing debt — the ratchet is dormant until a `.ci` changes, so drift accumulated uncaught) | Ran `scripts/dev/verify_wiring_docs.py`: ratchet reported "460 time.sleep( calls; baseline is 456" | AC-9's "456→454" is unachievable (460>454 fails ratchet). Set baseline to 460 (462−2) with explicit user approval (2026-07-13) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
- **Go grammar (parse):** `EngineStep` gains `Match string json:"match,omitempty"` (predicate kind: ""→contains, "matches", "absent", "json") and `Path string json:"path,omitempty"` (json dotted path). `omitempty` + `Match==""`→contains keeps existing serialized steps byte-identical (A-3). `parseEngineExpectContains` strips the trailing `:timeout=` (unchanged), then dispatches on the key prefix. `matches=` regex is `regexp.Compile`-validated at parse time (R-3); the source string is stored in `Text` and re-compiled once by the executor. `json=path=value` splits at the FIRST `=` (path has no `=`; value keeps colons/`=` — IPv6 safe since timeout already stripped). `absent=`/`json=` are rejected for `expect=stream` at parse (output-only, AC-8).
- **Go grammar (exec):** track `lastData` (raw `data`) alongside `lastOutput` (`status+" "+data`) so `json=` walks the raw JSON. In `EngineStepExpectOutput`, compile the regex once before the poll loop, then evaluate a `satisfied()` closure each iteration: contains→`strings.Contains(lastOutput)`, matches→`re.MatchString(lastOutput)`, absent→`!strings.Contains(lastOutput)`, json→walk `lastData`+stringify leaf==`Text`. A missing json path / non-JSON during polling is "not satisfied yet" (keep polling); the timeout error names the path + `%.200q` of `lastData` (R-2). `EngineStepExpectStream` supports contains/matches only.
- **json path walker:** `json.Unmarshal(lastData)`→walk `.`-split segments (map key or, if `[]any`, integer index with bounds check)→`stringifyJSON(leaf)` (string as-is, else `json.Marshal`). OOR index / missing key / non-JSON → (not-found, keep polling; named at timeout). No panics; `internal/test/runner` is subject to `c_sprintf_new`/`c_string_concat` hooks so use `fmt.Errorf` (allowed) / `textbuf.Buffer`, never `fmt.Sprintf`.
- **AC-4 functional test (echo peer):** an external observer does NOT receive peer `state` events via `receive [ state ]` or `request subscribe bgp event state` (verified: `events=0`), so state events are not a usable deterministic trigger here. The working pattern (user's exabgp insight) is `ze-test peer --mode echo` (`internal/test/peer/peer.go:476-482` writes received bytes straight back): the observer `request subscribe bgp event update`, `api.send("peer * update ... add <prefix>")`, the echo peer reflects the UPDATE, the daemon delivers a received-update event, and `wait_for_event(predicate=<prefix in json>)` matches it. Deterministic, 3x stable.
- **Python primitives:** module-level `wait_until(predicate, attempts=20, delay=0.25)->bool` (pure poll, no API needed — the forked kernel poll uses it) and `dispatch_until(api, command, predicate, attempts=20, delay=0.25)->dict` (single implementation). `API.wait_until`/`API.dispatch_until` are thin delegates so the wiring-table `API.*` calls resolve; `API.dispatch_until_done` becomes `dispatch_until(self, cmd, lambda r: r.get("status")=="done", ...)` (no dead duplicate loop — no-layering check). `wait_for_event(timeout=5.0, predicate=None)`: `None`→first event (unchanged return type: raw string); else decode each event via `json.loads` (raw string on decode failure) and return the raw event string when `predicate(decoded)` is true.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Three Python primitives (`wait_until` foundation, `dispatch_until`, `wait_for_event(predicate)`) | Two primitives only (dispatch_until + wait_for_event) | forked-route-install-kernel polls KERNEL state — neither a dispatch nor an event — so a generic `wait_until(predicate)` is required; the other two are specializations |
| Predicate kind is one-per-step (`Match` enum), AND-of-conditions via chained steps | multi-needle single step | keeps parse simple and matches today's chained `expect=output` usage (ipsec-sa); documented as a Known Limitation |
| Compile `matches=` regex at PARSE time | compile in executor loop | a bad regex should fail the test immediately, not hang to timeout (R-3) |
| `json=path=value` splits path/value at first `=`, value stringified for compare | full JSON value equality / typed compare | simplest declarative form; string compare covers prefixes/IPs/counts; typed compare deferred |

## Known Limitations
- One predicate kind per engine-step; multiple conditions require chained `expect=output` steps (N re-dispatches).
- `absent=`/`json=` are `expect=output` only (re-dispatched query); `expect=stream` supports `contains`/`matches` (append-only event stream has no "absent", and json-over-one-event is a deliberate non-goal).
- `json=` compares the stringified leaf value (no typed/numeric-range compare).
- Full migration of the ~454 remaining `time.sleep` occurrences is a separate effort.

## RFC Documentation
N/A — no protocol behavior.

## Implementation Summary
### What Was Implemented
- **Go engine-step grammar** (`internal/test/runner/engine_steps.go`): `EngineStep.Match`/`.Path` fields (omitempty, back-compat); `parseEngineExpectContains` generalized to `contains`/`matches`/`absent`/`json` with parse-time regex compilation (R-3) and stream-only-supports-contains/matches enforcement (AC-8); executor gained `engineOutputSatisfied`, `engineOutputTimeoutErr`, `engineJSONPathValue`, `engineStringifyJSON`, and `lastData` tracking. 518L (< 600L; no split needed).
- **Python primitives** (`test/scripts/ze_api.py`): module-level `wait_until` + `dispatch_until` (single implementations); `API.wait_until`/`API.dispatch_until` delegate; `API.dispatch_until_done` refactored to wrap `dispatch_until` (no dead loop); `wait_for_event(timeout, predicate=None)` extended (decode + predicate, raw on decode-fail, `None`=first-event unchanged).
- **Proofs**: fib-recursive.ci (2 sleeps → `dispatch_until_done`+`dispatch_until`), forked-route-install-kernel.ci (hand-rolled loops → `wait_until`), plus new `engine-steps-predicates.ci` (matches/absent/json) and `event-predicate-wait.ci` (wait_for_event predicate via echo peer).
### Bugs Found/Fixed
- ci-sleep baseline was stale (456 vs true 462); corrected to 460 (user-approved).
- 3 pre-existing exported symbols in engine_steps.go had no cross-package non-test caller (ze-validate); unexported/named at use site.
- (No behavioral bugs in the new code — TDD, all tests green.)
### Documentation Updates
- `docs/architecture/testing/ci-format.md`: new "Engine Steps" section (directives + predicate grammar + R-1 absent ordering caveat + source anchor).
- `docs/functional-tests.md`: "Payload-predicate waits" subsection under the quiesce-barrier section.
- `ai/rules/testing.md`: Observer API table rows (wait_until/dispatch_until/dispatch_until_done, clarified wait_for_event) + ratchet-rule mention + source anchor.
- `ai/INDEX.md`: keyword row for payload-predicate waits.
### Deviations from Plan
- **Baseline target changed 454 → 460 (user-approved 2026-07-13).** AC-9 planned `456→454`; the committed baseline was stale (true HEAD count 462), so the correct post-change value is 462−2=460. Ratchet passes at 460 ≤ 460. No extra sleeps were added; the 2 removed are real.
- **Pre-existing unrelated failure observed (not introduced, not fixed):** `check_design_refs` in `verify_wiring_docs.py` flags `internal/component/cli/client/inject.go:1` → broken `plan/spec-command-completion.md` reference. Outside this spec's scope (different, closed spec; file untouched here).

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Python observer SDK: `wait_until`, `dispatch_until`, `wait_for_event(predicate)` | Done | `test/scripts/ze_api.py` | + dispatch_until_done wrapper |
| Go engine-steps: `contains`+`matches`+`absent`+`json` grammar | Done | `internal/test/runner/engine_steps.go` | maximal grammar as scoped |
| Prove both with conversions that lower the sleep ratchet | Done | fib-recursive.ci (−2 sleeps); baseline 460 | |
| Wire discovery so future agents find the primitives | Done | ci-format.md, functional-tests.md, testing.md, ai/INDEX.md | |
| Not part of this spec: full migration of remaining sleeps | Respected | — | only the 2 proof conversions |
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Python smoke test (wait_until stops at k, False on exhaustion) + forked-route-install-kernel.ci (QEMU) | |
| AC-2 | Done | dispatch_until returns winning/last result — fib-recursive.ci + smoke test | |
| AC-3 | Done | dispatch_until_done wraps dispatch_until; 23 call sites unchanged; ipsec/fib suites pass | signature identical |
| AC-4 | Done | event-predicate-wait.ci (echo peer) + smoke test (predicate match/skip/raw/None) | |
| AC-5 | Done | TestParseEngineExpectMatches + TestRunEngineStepsMatchesRegex/TimeoutNamesRegex + engine-steps-predicates.ci | bad regex fails at parse |
| AC-6 | Done | TestParseEngineExpectAbsent + TestRunEngineStepsAbsent/AbsentTimeout + engine-steps-predicates.ci | non-vacuous (present→withdraw) |
| AC-7 | Done | TestParseEngineExpectJSON + TestRunEngineStepsJSONPath/TimeoutNamesPath + TestEngineJSONPathValue + engine-steps-predicates.ci | |
| AC-8 | Done | TestRunEngineStepsStreamMatchesRegex + parse rejects stream absent=/json= | |
| AC-9 | Done (amended) | baseline 460 (462−2, user-approved); ratchet OK | see AC-9 row + Deviations |
| AC-10 | Done | ci-format.md/functional-tests.md/testing.md/ai/INDEX.md updated; ze-doc-test PASS | |
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Parse: Matches/Absent/JSON/ContainsStillDefaults | PASS | engine_steps_test.go | |
| Exec: MatchesRegex/Absent/JSONPath/ContainsUnchanged + timeouts + StreamMatchesRegex + JSONPathValue | PASS | engine_steps_test.go | boundary cases in TestEngineJSONPathValue |
| fib-recursive / forked-route-install-kernel / engine-steps-predicates / event-predicate-wait | PASS | test/plugin/*.ci | forked via QEMU |
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| engine_steps.go / engine_steps_test.go / ze_api.py | Done | |
| record_parse.go | Unchanged | routing already correct (A-1) |
| runner_exec.go / cmd_engine_steps.go | Changed | rename + type-name at use site (ze-validate fix) |
| fib-recursive.ci / forked-route-install-kernel.ci / .ci-sleep-baseline | Done | baseline→460 |
| ci-format.md / functional-tests.md / testing.md / ai/INDEX.md | Done | |
| engine_predicate.go | Not created | engine_steps.go 518L < 600L, no split needed |
| engine-steps-predicates.ci / event-predicate-wait.ci | Created | |
### Audit Summary
- **Total items:** AC-1..AC-10 (10) + TDD tests + files
- **Done:** all 10 ACs
- **Partial:** none
- **Skipped:** none (engine_predicate.go intentionally not created — under size threshold)
- **Changed:** AC-9 baseline target 454→460 (user-approved); event test uses echo peer not state events

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Payload-predicate waits usable from Python observers | functional test | fib-recursive.ci (dispatch_until/dispatch_until_done) + forked-route-install-kernel.ci (wait_until, QEMU PASS) + event-predicate-wait.ci (wait_for_event predicate) all pass, no `time.sleep` |
| Payload-predicate waits usable as first-class `.ci` | functional test | engine-steps-predicates.ci passes using `matches=`/`absent=`/`json=` (3x stable) |
| Sleep count reduced | ratchet | fib-recursive.ci: 2 `time.sleep` removed (true count 462→460); `test/.ci-sleep-baseline` set to 460; ratchet OK (460 ≤ 460) via `verify_wiring_docs.py` |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | AC-4 (`wait_for_event(predicate)`) had no functional `.ci` — only unit/smoke coverage | wiring row 3 | **fixed** — added `test/plugin/event-predicate-wait.ci` (echo-mode ze-peer reflects an announced UPDATE; predicate matches by prefix); 3x stable PASS |
| 2 | ISSUE | ze-validate: 3 pre-existing exported symbols in engine_steps.go have no cross-package non-test caller (surfaced by touching the file) | `engine_steps.go` MarshalEngineSteps/WaitFrom/EngineDispatch | **fixed** — unexported `marshalEngineSteps`/`waitFrom` (runner-internal); named `runner.EngineDispatch` at the executor use site (`cmd_engine_steps.go`) |
| 3 | ISSUE | ci-sleep baseline (456) was stale vs the true HEAD count (462); AC-9's "→454" unachievable | `test/.ci-sleep-baseline` | **fixed** — set to 460 (462−2), user-approved 2026-07-13; ratchet OK (460 ≤ 460) |
| 4 | NOTE | 2 test-relaxations (fib-recursive, forked-route-install-kernel) flagged by audit-test-relaxation.py | those `.ci` files | **acknowledged** — legitimate replaced coverage (sleep/hand-rolled loop → predicate poll; assertions preserved; both tests PASS, forked in QEMU) |
| 5 | NOTE | Pre-existing broken design ref (unrelated, not introduced) | `internal/component/cli/client/inject.go:1` → `plan/spec-command-completion.md` | **acknowledged** — outside this spec's scope; file untouched here |
### Fixes applied
- Added `test/plugin/event-predicate-wait.ci` (AC-4 functional proof via echo peer). 3x stable PASS.
- Unexported `marshalEngineSteps`/`waitFrom`; named `runner.EngineDispatch` at `cmd_engine_steps.go`. ze-validate now passes.
- Set `test/.ci-sleep-baseline` to 460 (user-approved). Ratchet OK.

### Run 2+ (re-runs until clean)
Re-ran the pre-checks + scoped verification after the fixes:
- `make ze-validate` → all checks passed (0 findings).
- `python3 scripts/dev/audit-test-relaxation.py` → 2 documented RELAXED (legitimate replaced coverage), 0 deleted, 0 undocumented-weakened.
- `make ze-lint-changed` → 0 issues. Go unit tests (runner, cli) PASS. `make ze-doc-test` PASS.
- Functional: engine-steps-predicates, event-predicate-wait, fib-recursive PASS (native, 3x stable); forked-route-install-kernel PASS (QEMU); ipsec suite 6/6 PASS (contains= unbroken).

**Known-red (pre-existing, NOT introduced, unrelated to this spec — scoped verify per `ai/rules/git-safety.md`):**
- `make ze-lint` (whole tree) fails: `internal/component/bgp/plugins/route_refresh/handler/*_test.go` mocks miss `DrainPeerSync` (a `ReactorLifecycle` method added by earlier unpushed commits). Present at session start; route_refresh not in this change's diff.
- `verify_wiring_docs.py check_design_refs`: `internal/component/cli/client/inject.go:1` → broken `plan/spec-command-completion.md` ref. Unrelated closed spec; file untouched.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE **for this change's scope** (all Run-1 ISSUEs fixed; remaining reds are pre-existing and unrelated, documented above)
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| engine_steps.go / engine_steps_test.go | yes | git status M |
| ze_api.py | yes | git status M |
| engine-steps-predicates.ci / event-predicate-wait.ci | yes | git status ?? |
| .ci-sleep-baseline == 460 | yes | `cat` == 460 |
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-8 | primitives + grammar work | `go test ./internal/test/runner/` PASS; smoke test PASS |
| AC-9 | ratchet down + baseline correct | `verify_wiring_docs.py` → "ci-sleep ratchet OK (460 ≤ 460)" |
| AC-10 | docs synced | `make ze-doc-test` PASS |
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| dispatch_until / dispatch_until_done | fib-recursive.ci | PASS |
| wait_until | forked-route-install-kernel.ci | PASS (QEMU) |
| wait_for_event(predicate) | event-predicate-wait.ci | PASS (3x) |
| matches=/absent=/json= | engine-steps-predicates.ci | PASS (3x) |
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | record_parse.go:241-246 |
| A-2 | confirmed | fib-recursive.ci passes with prefix predicate |
| A-3 | confirmed | no committed engine-steps.json; omitempty back-compat |
| A-4 | confirmed | no pytest/unittest harness for ze_api |
| A-5 | confirmed | engine-steps-predicates.ci dispatches inject/withdraw/show rib, no live peer |
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| ci-format.md Engine Steps grammar | anchor → engine_steps.go parseEngineExpectContains/engineOutputSatisfied/engineJSONPathValue | ze-doc-test PASS |
| functional-tests.md Payload-predicate waits | anchor → ze_api.py + engine_steps.go | ze-doc-test PASS |
| testing.md Observer API rows | anchor → ze_api.py | ze-doc-test PASS |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/test/runner`, `test/scripts`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for json array index
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-payload-predicate-waits.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-payload-predicate-waits.md`
