# Spec: native-action-job-admission

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-shared-machine-job-admission` built job admission and rolled it out over
106 Makefile targets. The Makefile was then retired and every target became a
native `./le` action, and the rollout did not travel with them. Two actions
admit today, `./le verify lint run` and `./le verify lock run`. Every other
heavy action reaches the machine unadmitted: the unit suites, the functional
runner, the integration gates, the fuzz corpus, the QEMU suites, and the whole
of `./le verify current mode full`, which holds no slot while it runs
twenty-six stages.

The goal is that the admitted population is again a CRITERION rather than a
list: an action that starts a Go test binary, the `ze-test` runner,
`golangci-lint`, `govulncheck`, Docker, or QEMU is admitted, and a check proves
no action reaches those tools around admission.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/running-commands.md` - "One verify at a time": the admitted route, the slot count, and the registry state a reader is told to open
- [ ] `ai/rules/commands.md` - the rule the `bashRawHeavy` hook check enforces, and what an agent is told to type instead
- [ ] `docs/functional-tests.md` - the verify runner's stages and the artifacts a job publishes

**Key insights:** (minimal context to resume after compaction)
- The mechanism is BUILT and tested (`internal/le/job`). This is a wiring problem, not a mechanism problem.
- The hook already refuses the raw tools, so an AGENT is admitted. An `./le` action that shells out to the same tool is not: the hook sees `./le`, and the action reaches `go test` from inside its own process.
- `./le verify current mode full` is the largest job on the machine and takes no slot at all. Its lint stage takes one, which is why a verify appears in the registry and reads as admitted.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/job/job.go` - `Admit` and `Run`, the two entry points an action uses, and `defaultSlots`, which sizes the machine
- [ ] `internal/le/verify/lint/actions.go` - `runHere`, the one action that admits in process: it calls `Admit`, writes its output to the ticket log, and calls `Release` with its own verdict
- [ ] `internal/le/verify/lock/answer.go` - the generic wrapper, `./le verify lock run <label> <argv>`
- [ ] `internal/le/verify/current.go` - `runCurrent`, which takes no ticket before it runs the whole stage population
- [ ] `internal/le/hookruntime/bash.go` - `bashRawHeavy`, the refusal that names `./le job run label <label> command <argv>`

**Behavior to preserve:**
- The nested-job rule: a stage that runs inside a parent's slot runs straight through (`insideParent`).
- Every action's exit-code contract. The commit gate reads it.
- `./le job run` as the generic route for a command with no action of its own.

**Behavior to change:**
- The admitted population grows from two actions to every heavy one.
- A check names an action that reaches a heavy tool without a ticket.

## Data Flow (MANDATORY)

### Entry Point
- An `./le <area> <action>` invocation that starts a Go test binary, `ze-test`, `golangci-lint`, `govulncheck`, Docker, or QEMU.

### Transformation Path
1. The action resolves the checkout root and builds an `Admission` (`job.NewIn`).
2. It calls `Admit` with the argv that identifies its work.
3. Three outcomes, as today: a slot to hold, another job's verdict to answer with, or a parent's slot to run inside.
4. The action writes its output to the ticket's log while it works, so the breaker can see it is alive.
5. It calls `Release` exactly once with the code it answers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Action ↔ registry | files under `tmp/.ze-jobs/`, guarded by one flock per scan | No |
| Parent ↔ stage | `ZE_RUN_JOB` names the parent entry, and the stage runs inside its slot | No |

### Integration Points
- `internal/le/job` - `Admit`, `Ticket`, `Release`.
- `internal/le/verify/engine` - the stage runner, which must not queue a stage behind its own parent.
- `internal/le/leaction` - where a shared "admitted action" helper would live, if the population proves it needs one.

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
| A-1 | Every heavy action can name the argv that identifies its work | `verifylint.jobArgv` does it in four lines | two actions share a verdict they should not, or share none they should | one `jobArgv` per admitted action, reviewed against its parameters | unvalidated |
| A-2 | Admitting `./le verify current mode full` does not deadlock its own stages | `insideParent` and `childEnviron` exist for exactly this | a verify waits for the slot it holds | a full verify under a second holder | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An in-process action forgets `Release` on one return path and leaks a slot for ever | jobs waiting behind a holder whose process is gone | the reaper already answers a dead holder; a leaked slot from a LIVE process is what a `defer` prevents, so the shared helper owns the pairing rather than each action |
| R-2 | The population is a list again, so the next action added misses it | a heavy action with no ticket | a check over the registered actions, which is why the criterion is stated above |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session's ability to run a test, in both directions: a wedged admission point blocks everybody, and no admission point freezes the box |
| How is it reverted? | Per action. Each is an independent wiring change |
| Who else touches this path? | Every session in the checkout, concurrently |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify current mode full` with a slot already held | → | the admission call in `runCurrent` | a test that the run waits rather than starting |
| A heavy action reaching a Go test binary with no ticket | → | the population check | a test that names the unadmitted action |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any registered action that starts a Go test binary, `ze-test`, `golangci-lint`, `govulncheck`, Docker, or QEMU | It takes a ticket before it starts that tool, and releases it with its own verdict |
| AC-2 | A stage of an admitted run | It runs inside the parent's slot and never queues behind it |
| AC-3 | An action added later that reaches a heavy tool with no ticket | A check names it, by action, and fails |
| AC-4 | Two sessions asking for the same heavy action on the same tree | One runs and the other attaches, as `./le verify lint run` already does |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a full verify takes a ticket before its first stage | `internal/le/verify/current_test.go` | AC-1, AC-2 | |
| the population check names an unadmitted heavy action | the check's own package | AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| slot count (`ZE_RUN_SLOTS`) | 1..cores | cores | 0 | cores + 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| two sessions ask for the same heavy action at once | `internal/le/job` | one runs, one shares, neither is killed | |

### Interop Tests (Scope: protocol)
N/A - Scope is tooling; no wire-visible behavior changes.

## Files to Modify
- `internal/le/verify/current.go` - admit the whole run
- every area whose action starts one of the six heavy tools
- `docs/contributing/running-commands.md` - the admitted population, once it is a criterion again

## Files to Create
- the population check, wherever the registered-action inventory already lives

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | N-A | no new command; existing actions gain admission |
| Env var registration | N-A | `ZE_RUN_SLOTS` and `ZE_JOB_STALL_SECONDS` already exist |
| Doctor check for runtime dependencies | N-A | the registry needs no binary beyond what job already requires |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes | `docs/contributing/running-commands.md`, "One verify at a time" |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on the actions this touches |

## Implementation Steps

1. **Phase: the criterion** -- enumerate the registered actions that reach a heavy tool, and write the check that answers the question, so the population is measured before it is changed
2. **Phase: the verify run** -- admit `./le verify current mode full`, the largest job, and prove its stages still run inside its slot
3. **Phase: the rest** -- one action at a time, each with its own `jobArgv`
4. **Phase: the page** -- `docs/contributing/running-commands.md` states the criterion and the derived slot count

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Fail-closed | An action that cannot reach the registry QUEUES, never runs unadmitted |
| Data flow | Admission is decided in one place; no action reaches a heavy tool around it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Every heavy action admits | the population check passes over the registered inventory |
| The verify run holds a slot | a second heavy job waits while a full verify runs |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Each action's label is a path component: `validLabel` refuses anything else |
| Resource exhaustion | The registry stays bounded as the admitted population grows |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The admitted population is the whole value of the mechanism. Two actions out of a hundred is a registry that reports one job and a machine running ten.
- The hook and the actions have to agree. An agent refused a raw `go test` and pointed at `./le test-unit` is admitted only if that action admits itself.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Admit inside each action rather than around the dispatcher | one ticket taken by `leroot.Run` for every command | a dispatcher cannot tell a heavy action from `./le spec session current`, and admitting the cheap ones would queue a session behind a lint to read a status |

## Known Limitations
- Admission covers what an action does in its own process. A tool a test binary starts is attributed to the job that started it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated, not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + spec
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
