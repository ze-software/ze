# Spec: agent-economy-2-scoped-test

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it adds a short `le` action for scoped tests and changes no shipped binary. |
| RFC gaps | - |
| Depends | plan/post-release/engineering/spec-agent-economy-1-call-shape.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 2 of `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
which holds the baseline.

Testing one package is the most frequent heavy action an agent takes, and it
has no short form. The agent assembles the build-tag list by hand and wraps
the command in job admission. Over three weeks that produced 1,857 raw
`go vet`, `go test` and `go build` attempts of which 707 failed, about 350
`./le help test` lookups, and false reds from missing tags. Queue wait is
measured and never saved, so run time cannot be told from waiting. A mistyped
`le` command answers with the whole usage, and `./le commit create` refuses a
body for one over-long word without naming it.

The goal: one short command tests, vets or compiles named packages under the
right tags, admitted, answering in one line on green; every admitted run
records its wait; a mistyped command answers with the nearest real one; a
commit body with one over-long word is accepted.

## Research Findings (2026-11-11, read at the producer by a research agent; re-read at design)

| Finding | Where |
|---------|-------|
| `./le job run` passes its argv unchanged: no tags, no race, no timeout. The agent supplies the tags from `feature-gates.txt` | `internal/le/job`, `Admission.Run`; `docs/contributing/running-commands.md` |
| The hook's suggested replacement for a raw `go test` adds no tags and no `quiet` | `internal/le/hookruntime/bash.go`, `admittedCommand` |
| `./le test unit` has the groups bgp, core, plugins, config, cli, installer and all. None covers `internal/le`, and there is no per-package action | `internal/le/test/unit/groups.go` |
| Tags, timeout, race and the environment are already computed | `internal/le/go/toolchain`, `Toolchain.GoTest`, `GoVet`, `GoList`, `Environment` |
| Admission with attach-to-identical-job exists, four slots on this machine | `internal/le/job/job.go`, `defaultSlots`, `Admission.queue` |
| Quiet output exists: one line on green, up to 20 failure lines on red | `internal/le/job/quiet.go`, `runQuiet`, `quietSummary` |
| Scope keywords `scope <packages>` and `changed` exist on lint | `internal/le/go/lint/actions.go`, `changedScope` |
| The reached-package selector exists | `internal/le/repo/changed/selector.go`, `Scope.Resolve`; `./le repo changed scope print both` |
| Wait is measured and dropped: the duration log holds label, seconds and time; seconds start at the claim | `internal/le/job`, `Ticket.Waited`, `appendDuration`; `tmp/.ze-verify-duration.txt` |
| Run time since 2026-10-21: `unit-pkg` 2,164 runs and 26.7h; `lint` 488 and 12.2h; `test-bgp` 983 and 7.6h | the same file |
| An unknown command prints `unknown command` and the full usage. A retired name already answers in one line | `internal/le/le/root/dispatch.go`, `refuseRetired` |
| The commit body is wrapped at 72; "unwrappable" fires for one word longer than the line and does not name it | `internal/le/commit/input.go`, `wrapBody` |
| `docs/contributing/testing.md` shows a one-package test with no tags, and a command with a literal `bin/le` | that page, ladder step 1 and the cheat sheet |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - the door for `le`
  → Constraint: a tool's package sits at the path its command name predicts, and `le help`, completion and `le help ai` list every new word.
  → Constraint: an action answers through the shared renderer and stays callable as Go.
- [ ] `docs/contributing/running-commands.md` - launcher, tags, admission
  → Constraint: a session runs one CPU-intensive job at a time, and heavy work is admitted.

**Key insights:**
- Every part of the short action exists. The work is joining them behind one registered action and pointing the hook messages and pages at it.

## Current Behavior (MANDATORY)

**Source files read:** the main thread read one producer; the rest of the Research Findings table is owed a read at design.
- [ ] `internal/le/hookruntime/bash.go` - `admittedCommand` builds the replacement the raw-test refusal prints, with no tags and no `quiet`; `rawHeavy` matches `go test` and `golangci-lint` only, so a raw `go vet` is refused by nothing

**Behavior to preserve:**
- The long `./le job run label <label> command <argv>` form keeps working.
- Admission, the slot count and the attach rule are unchanged.

**Behavior to change:**
- Only what the acceptance criteria name.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Unit groups | `internal/le/test/unit/groups.go`, `Group` | Run a fixed package pattern with tags and race | Extends: a package-list action beside the groups, same toolchain calls |
| Job admission and quiet | `internal/le/job` | Admit, run, summarize | Extends: the new action admits under the `unit-pkg` label and is quiet by default |
| Reached-package selector | `internal/le/repo/changed/selector.go` | Packages and tags a change reaches | Extends: the `changed` keyword consumes it |
| One-line refusal of a retired name | `internal/le/le/root/dispatch.go`, `refuseRetired` | Answers a retired command in one line | Extends: an unknown command answers the same way with the nearest registered name |

## Data Flow (MANDATORY)

### Entry Point
- `./le test unit pkg <path>...` typed by an agent or the owner.

### Transformation Path
1. The action resolves the packages (named, or reached by the change).
2. The toolchain supplies tags, race and environment.
3. Admission queues and runs the job, records wait and run time.
4. The quiet summary prints one line, or the failure lines.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ shared machine | job admission | No |

### Integration Points
- The toolchain's test and vet argv builders, job admission and its quiet summary, the reached-package selector, the `le` dispatcher's unknown-command answer.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test unit pkg <path>` | → | the package action | `TestPackageActionBuildsTheToolchainArgv` |
| an admitted run's release | → | the duration record | `TestDurationRecordCarriesTheWait` |
| `./le <mistyped>` | → | the dispatcher's unknown-command answer | `TestUnknownCommandNamesTheNearestRegisteredOne` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le test unit pkg <path>...` on green packages | Runs them under the tags the toolchain derives, admitted, and prints one line: exit code, pass count, waited seconds, log path | The log holds the full output: nothing is discarded |
| AC-2 | The same on a red package | Prints the failing test names and their failure lines, then the same summary line | Up to the quiet mode's failure-line cap; the log holds the rest |
| AC-3 | `run <regex>` | Runs only matching tests | None |
| AC-4 | `vet` in place of a test run | Vets, and so compiles, the same packages under the same tags | None |
| AC-5 | `changed` in place of paths | Runs the packages `./le repo changed scope` reaches | A change reaching no Go package answers that nothing is reached and exits 0 |
| AC-6 | Any admitted run | Its record holds the wait beside the run time, and the summary shows the wait when above zero | Older three-column rows stay readable |
| AC-7 | A raw `go test` or raw `go vet` typed by an agent | The refusal names the short action with the caller's packages | `ZE_ADMIT_RAW` with a reason still passes |
| AC-8 | An unknown `le` command | One line naming the nearest registered command, then how to list the area | An ambiguous word lists up to three candidates |
| AC-9 | A commit body holding one word longer than the line | Accepted, the word on a line of its own | A subject over the limit is still refused |
| AC-10 | `docs/contributing/testing.md` and the running-commands page | Show the short action; no example omits tags or names `bin/le` literally | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPackageActionBuildsTheToolchainArgv` | `internal/le/test/unit/` | AC-1, AC-3, AC-4 | |
| `TestDurationRecordCarriesTheWait` | `internal/le/job/` | AC-6 | |
| `TestUnknownCommandNamesTheNearestRegisteredOne` | `internal/le/le/root/` | AC-8 | |
| others | named at design | AC-2, AC-5, AC-7, AC-9 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| named at design | `test/runner/` | A red package prints its failing test through the short action | |

## Files to Modify
- Named at design, after the producers in the Research Findings table are read. The areas are the unit-test action, job admission's record and summary, the dispatcher, the commit input, the raw-command refusal messages, and the testing and running-commands pages.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the package action and its failing wiring test
2. **Phase: the wait record** -- persist and print the wait
3. **Phase: answers that carry the fix** -- the unknown-command answer, the commit body, the refusal messages
4. **Phase: pages** -- the testing and running-commands pages, in the phase whose code makes them wrong

## Open Questions for Design
- Whether the action lives at `le test unit pkg` or needs its own area for the command-path rule.
- Whether `vet` is a keyword of the same action or a sibling action.
- The nearest-name measure for AC-8, drawn from the registry.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] The targeted tests pass, run by the main thread: the packages under `internal/le/test/unit`, `internal/le/job`, `internal/le/le/root` and `internal/le/commit`, and `./le test functional runner`. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
