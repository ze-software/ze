# Spec: the fix pass for restated registries

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**RESUME HERE (session stopped 2026-09-14 at the weekly usage cap).** The state,
the resume kit, the gate's judgement rules, the wave-2 scope split and the traps
that cost agents real time are in
`plan/handover/restated-registry-fix-pass-2026-09-14.md`.
Read it BEFORE touching anything: eight agents were stopped mid-edit, the tree
does not compile, and `./le` does not rebuild itself so every measurement needs
`./le --update` first. Derive the uncommitted file list from `git status`; do not
trust any list written down, which is the defect this spec exists to remove.

## Task

`./le enumeration report` measures the Go literals that restate a live registry.
On 2026-09-14 it answered 230 rows. The gate that produced that number refuses
the NEXT one (spec-a-literal-restates-a-registry, closed 2026-09-14); it does not
end the ones already there, and the owner decided on 2026-09-14 that the backlog
stays visible in `report` rather than behind exemption markers.

This spec ends them. Each row is a second declaration of a set some registry
already holds, and two declarations of one fact drift. At least two have already
drifted into an answer an operator reads:

- The RIB spells the FlowSpec SAFI `flowspec` where the family registry spells it
  `flow`, so the two names for one family disagree.
- `readOnlyVerbs` omits `resolve`, which `command.Verbs` classifies as a read
  verb, so an agent is told `resolve` is daemon-mode when it is not.

That is why this sits in `plan/immediate/`: a CLI surface that answers wrongly is
the bucket's own test (`plan/README.md`).

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/registration.md` - every registry that exists and what each holds
- [ ] `ai/rules/principles.md` - the two always-on directives a restated registry breaks
- [ ] `internal/le/enumeration/enumeration.go` - what the gate calls a copy, and the
  three narrowings that keep a declaration and another namespace out of the count

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/enumeration/report.go` - the row a reader acts on: the file, the
  symbol, the corpus, and how many of the unit's strings are registry keys

**Behavior to preserve:**
- `./le enumeration check` keeps blocking on what a change set introduces.

**Behavior to change:**
- Each fixed row derives its set from the registry, so `./le enumeration report`
  answers fewer rows and no row is silenced by a marker instead.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le enumeration report`, whose rows are the work list.

### Transformation Path
1. Read the current rows, grouped by corpus.
2. For each row, replace the literal with a call to the registry that owns the set.
3. Re-run the report and confirm the row is gone rather than suppressed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| literal → registry | the fixed call site reads the registry at runtime | No |

### Integration Points
- `internal/le/enumeration` - the measurement this spec drives down.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every one of the 230 rows has a registry call that can replace it | the gate reports only a set a live registry holds | a row needs a registry that does not exist yet, and that row is its own spec | fix one row per corpus first and count what the call costs | unvalidated |
| A-2 | A row can be fixed without changing what the surface answers | the literal and the registry are meant to hold one set | the two disagree, and the drift is a defect of its own | assert the surface's answer before and after each fix | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixing a drifted copy changes a published answer | a functional test goes red on the corrected spelling | the drift IS the defect: correct the answer and the test that pinned the wrong one |
| R-2 | 230 rows is more than one session | the report count does not move | cut the work by corpus, one spec phase per corpus, and land each |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A surface answers from the registry instead of a stale copy, so a drifted name changes. An operator meets the corrected name |
| How is it reverted? | Each row is its own edit and its own commit |
| Who else touches this path? | Every component named in the report |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le enumeration report` | → | the fixed call sites | the row count falls, and no new exemption marker appears |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le enumeration report` after the pass | every remaining row is a set no registry holds, named in this spec |
| AC-2 | The FlowSpec SAFI name | the RIB and the family registry answer one spelling, proven by a test that reads both |
| AC-3 | `readOnlyVerbs` | derived from `command.Verbs`, so `resolve` is classified as the registry classifies it |
| AC-4 | Any fixed row | no exemption marker was added for it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per-row test at each fixed call site | with the fixed code | the surface answers from the registry | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the FlowSpec family name an operator types | `test/` | one spelling reaches the RIB and the family registry | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling; the FlowSpec row is a name, not a wire change | |

## Files to Modify
- Named by `./le enumeration report` at the time this spec runs, one file per row.

## Files to Create
- None expected; a row needing a registry that does not exist becomes its own spec.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no operator config changes |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | no command is added |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no new answer |
| Env var registration | N-A | no new environment leaf |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | no runtime metric |
| BGP family surface (new SAFI / capability / attribute) | Yes | the FlowSpec SAFI name is one of the rows |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | answered when the FlowSpec row is fixed |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `ai/patterns/registration.md` if a registry gains a reader |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | `./le spec citation anchors` during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: one corpus at a time** -- read the report, take the rows of one corpus
   - Tests: a test at each fixed call site
   - Files: the report's own list
   - Verify: the report's tally for that corpus falls to zero
2. **Phase: the drifted rows first** -- FlowSpec and `readOnlyVerbs` answer wrongly today
   - Tests: AC-2 and AC-3
   - Verify: the two surfaces agree with the registry

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every row the report named is gone or is named here as needing a registry first |
| Correctness | No row was closed by adding an exemption marker |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The report count falls | `./le enumeration report` |
| No new marker | `git diff` holds no added `enumeration: exempt` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open | A surface that read a literal and now reads a registry must refuse an empty registry rather than answering nothing |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A row needs a registry that does not exist | Its own spec, named in Work Not Done |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Known Limitations
- A key set no registry holds cannot be fixed here. spec-a-literal-restates-a-registry
  named six such sets: functional test suite names, kernel RTPROTO names, IANA
  protocol names, ExaBGP API control words, iproute2 subcommands, and RFC 7752
  BGP-LS protocol-ID names. Each needs a registry to exist first, and each is its
  own spec.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

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
