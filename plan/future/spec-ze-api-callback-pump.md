# Spec: ze-api-callback-pump

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> is the helper every plugin fixture drives ze through,
and it has no callback pump. A fixture whose plugin must answer ze's filter
verdict has to write the pump itself, so the same loop is open-coded in every
such fixture.

This is an improvement, not a defect: the open-coded copies work. It is filed
because the copy count is growing. Six fixtures carried it before 2026-08-16 and
seven more gained it that day, when the `redistribution-*.ci` set was repaired to
actually launch its peers. Thirteen copies of one loop is the point at which the
next author copies a copy.

**Why the pump is needed at all.** ze asks for the filter verdict on the plugin's
callback fd, and only `API.read_line` answers it. A plugin parked in a dispatch
RPC leaves the question unanswered until the reactor's IPC deadline expires, and
then `on-error=reject` decides the route. So a fixture that polls for a result
without pumping the callback fd does not merely run slowly: it gets the
`on-error` verdict instead of its filter's, which is a silently wrong answer
rather than a timeout.

**That is what makes it worth centralising rather than leaving to each author.**
The consequence of forgetting the pump is not a red test. It is a green test
measuring the wrong thing, which `ai/rules/testing.md` calls a vacuous pass. A
helper that pumps by construction removes the whole class.

**Related, already recorded.** `plan/journal/helper-bypassed-by-an-open-coded-copy.md`
holds this class. This spec is one member of it, and the class file is what earns
a deliberate pass over the journal rather than a fix by whoever tripped over it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` -- the plugin process protocol,
      including which fd carries the callback and what ze does on no answer
- [ ] `docs/plugin-development/README.md` -- what a plugin author is told the
      helper does for them
- [ ] `ai/patterns/functional-test.md` -- the fixture patterns this changes

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> -- `API.read_line`, `wait_for_config`,
      `wait_for_registry`, `ready`, `_call_engine`, and every polling helper
  → Constraint: `read_line` is the only thing that answers the callback. A pump
    is a loop around it, so the helper already owns the primitive.
- [ ] the six pre-existing fixtures that open-code `serve_until`, and the seven
      `test/plugin/redistribution-*.ci` that gained one on 2026-08-16
  → Decision: the copies are the specification. Read them all before designing
    the signature; the differences between them are the requirements.
- [ ] `internal/component/plugin/` -- the reactor side, to name the IPC deadline
      and the `on-error` fallback
  → Constraint: the timeout the pump must beat is a property of the daemon, not
    of the fixture. Name it rather than guessing a poll interval.

**Behavior to preserve:**
- Every existing fixture keeps working. A helper that requires rewriting thirteen
  fixtures to land is a worse trade than the copies.
- The helper stays a thin, readable script: it is read by plugin authors as
  documentation of the protocol.

**Behavior to change:**
- A fixture can wait for a condition without open-coding the callback loop.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.ci` fixture's `tmpfs=*.run` plugin script imports `ze_api` and waits for
  something.

### Transformation Path
1. The plugin declares itself and reaches `ready()`.
2. ze asks for a filter verdict on the callback fd.
3. The fixture polls for its own condition.
4. If that poll does not call `read_line`, the question is unanswered.
5. The reactor's IPC deadline expires and `on-error` decides the route.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| plugin <-> ze | the callback fd, answered only by `API.read_line` | Yes, established while repairing the redistribution fixtures |
| fixture <-> helper | the open-coded loop each fixture writes | Yes, thirteen copies |
| helper <-> reactor deadline | the timeout the pump must beat | Not read yet |

### Integration Points
- Every `test/plugin/` fixture whose plugin answers a filter.
- `docs/plugin-development/README.md`, which describes the helper.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | each fixture reaches around the helper for a primitive the helper owns |
| No unintended coupling (components stay isolated) | Yes | stays inside `test/scripts/` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> |
| No duplicated functionality (extends existing, does not recreate) | No, today | thirteen copies is the defect; the fix restores this property |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python test helper |
| Registration over hardcoding | N-A | no registration surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | The thirteen copies are close enough that one signature covers them | they solve one problem | the helper needs two entry points, or the copies differ for real reasons worth keeping | diff all thirteen | unvalidated |
| A-2 | Migrating a fixture to the helper is behavior-preserving | the loop is the same loop | a fixture depended on a detail of its own copy | migrate one, prove it still discriminates, then the rest | unvalidated |
| A-3 | The reactor's IPC deadline is knowable from the daemon side | it produces the failure | the pump interval is a guess, which is how this class of helper rots | read the plugin component | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Migrating thirteen fixtures at once turns a helper addition into a corpus rewrite | the diff is thirteen files before the helper has one user | land the helper with ONE migrated fixture; the rest is separable and stays separable |
| R-2 | The helper hides the protocol from plugin authors who read it to learn | an author cannot see why the pump exists | the helper's docstring names the callback fd and the `on-error` consequence |

## Blast Radius

`test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> and whichever fixtures migrate. No daemon code, no wire
behavior.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| a `.ci` plugin script calling the new helper instead of its own loop | -> | the pump in `ze_api.py` | one migrated fixture under `test/plugin/`, proven to still discriminate |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin script waits for a condition using the helper | ze's filter verdict is answered throughout the wait |
| AC-2 | The migrated fixture, with its filter mechanism broken | RED, so the migration did not cost the fixture its discrimination |
| AC-3 | The twelve unmigrated fixtures | Pass unchanged |
| AC-4 | A plugin script that waits WITHOUT pumping | Still possible: the helper adds a facility and forbids nothing |
| AC-5 | The helper's docstring | Names the callback fd and what happens when it goes unanswered |

## End-to-End User Stories

- A plugin author writes a fixture that waits for a route to arrive, uses the
  helper, and gets their filter's verdict rather than the `on-error` fallback.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the pump answers a callback while waiting | `test/scripts/ze_api_test.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> | AC-1 | |
| the pump returns when its condition is met | `test/scripts/ze_api_test.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the one migrated fixture | `test/plugin/` | a plugin waits through the helper and its filter decides the route | |

## Files to Modify

- `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> -- the pump, beside `read_line`
- one `test/plugin/` fixture, migrated as the helper's first user
- `docs/plugin-development/README.md` -- if the helper is documented there

## Files to Create

- `test/scripts/ze_api_test.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) -->, if no sibling suite exists

## Implementation Steps

1. **Phase: Read the thirteen** -- diff the copies; the differences are the
   requirements
   - Verify: A-1 confirmed or broken
2. **Phase: Name the deadline** -- read the reactor side rather than guessing an
   interval
   - Verify: A-3
3. **Phase: Add the pump**
   - Verify: AC-1, AC-4, AC-5
4. **Phase: Migrate exactly one fixture, and prove it still discriminates**
   - Verify: AC-2, and A-2
5. **Phase: `./le functional plugin`, entire**
   - Verify: AC-3

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations

- Migrating the other twelve fixtures is deliberately NOT in this spec. The
  helper with one proven user is the deliverable; a corpus rewrite is separable
  work and each migration owes its own discrimination proof.
