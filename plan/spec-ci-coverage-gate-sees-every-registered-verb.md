# Spec: ci-coverage-gate-sees-every-registered-verb

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The `.ci` coverage gate reports a CLI command that a changed file registers and no
`.ci` test names. It cannot see a verb that a BGP plugin registers. The owner asked
for this as its own spec (spec triage, 2026-10-09). The defect is recorded in
`plan/journal/gate-excludes-part-of-its-population.md`, row dated 2026-08-30
(`checkCLIHandlerCoverage`).

Two causes, and each one is enough alone:

1. `CLIPaths` (`internal/le/repo/repository.go`) holds `internal/component/cli/`,
   `internal/component/cmd/` and `internal/plugins/`. No file under
   `internal/component/bgp/plugins/` enters the population.
2. `RegisterPattern` matches only `MustRegister\w+("name")`. The announce verbs
   register through `pluginserver.RegisterRPCs` with
   `RPCRegistration{WireMethod: ...}` (`internal/component/bgp/plugins/cmd/announce/announce.go`
   `init`), which the pattern cannot match. 129 files call `pluginserver.RegisterRPCs`.

The goal: the gate reads its population from the registered command tree, not from
path prefixes and one call shape, so a verb registered in any package and in any
shape is either named by a `.ci` or reported. Both halves are owed together: fixing
`CLIPaths` alone still matches nothing while the extractor knows only `MustRegister`.

The journal row records why it was not fixed in place: widening the population shows
coverage debt across every BGP plugin at once. The spec must decide how that debt
lands (an explicit debt list that only shrinks, or the tests themselves) before the
gate widens, so it does not fall as ISSUE on whichever session commits next.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - the `.ci` corpus the gate reads; no page describes this gate yet
- [ ] `docs/architecture/testing/ci-format.md` - what a `.ci` file can name

**Key insights:**
- The gate is `checkCLIHandlerCoverage` in `internal/le/repo/repository.go`; the journal row names the old path `internal/le/repository/`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/repo/repository.go` - `CLIPaths`, `RegisterPattern`, `checkCLIHandlerCoverage`: filter changed files by path prefix, extract names by regex, search the `.ci` corpus as a string
- [ ] `internal/le/repo/repository_test.go` - the existing test drives `internal/plugins/x/cmd.go`
- [ ] `internal/component/bgp/plugins/cmd/announce/announce.go` - seven `RPCRegistration` entries, none named by a `.ci`

**Behavior to preserve:**
- A changed file outside the CLI population owes nothing.
- A command a `.ci` names is not reported.

**Behavior to change:**
- The population and the name extraction, as stated in Task.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The repository checks run by the verify gate, with the changed-file list.

### Transformation Path
1. Changed files filtered by `CLIPaths`.
2. Command names extracted by `RegisterPattern`.
3. Each name searched in the `.ci` corpus from `readCITests`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le gate to the ze command registry | to be designed | No |

### Integration Points
- To be designed.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The registered command tree can be read by `le` without starting a daemon | to be checked | the population needs another source | design research | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The widened gate reds every session at once | first run lists many BGP plugin verbs | land the debt decision before the gate widens |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the gate refuses commits, or still reports nothing; nothing user-visible |
| How is it reverted? | single commit revert |
| Who else touches this path? | sessions adding CLI verbs; `spec-ci-coverage-remaining-surfaces` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a changed BGP plugin file that registers an RPC verb | → | `checkCLIHandlerCoverage` | to be named in design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a changed file under `internal/component/bgp/plugins/` registers a verb no `.ci` names | the gate reports it |
| AC-2 | a verb registered through `pluginserver.RegisterRPCs` and no `.ci` names it | the gate reports it |
| AC-3 | a registration the gate cannot resolve to a name | the gate fails and names the file, it does not skip it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be designed | `internal/le/repo/repository_test.go` | AC-1 to AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the gate takes no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be designed | | | |

## Files to Modify
- `internal/le/repo/repository.go` - population and extraction

## Files to Create
- to be designed

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling only |
| YANG validation constraints | N-A | tooling only |
| YANG custom validators | N-A | tooling only |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | tooling only |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no output change |
| Env var registration | N-A | none added |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md`: describe the gate and its population |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- to be designed
2. **Phase: debt decision** -- how the existing uncovered verbs land, decided with the owner

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- To be written at design.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests: N-A, tooling with no protocol peer

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-ci-coverage-gate-sees-every-registered-verb.md` only, in the same `./le commit create` script
