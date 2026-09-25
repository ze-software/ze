# Spec: RFC evidence strength 5 -- black-box conformance with external suites and protocol fuzzers

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-evidence-strength-0-umbrella.md` (children 1 to 4 first, by the owner's priority order) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Owner approval, 2026-09-24, as a later phase.** Every RFC proof Ze holds today is
written by Ze's own authors: 5911 unit links, 105 functional and 42 interop
(`./le rfc check`, `evidence:` line). Interop scenarios (152 directories under
`test/interop/scenarios/`) prove that a named peer accepts Ze. None of them was
written by a third party to test a protocol implementation against its RFC. A
black-box suite does that: it drives Ze only through the wire and judges it by the
RFC, with oracles Ze's authors did not write.

Research questions this spec answers before any design:

| # | Question |
|---|----------|
| Q-1 | Which external conformance suites or protocol fuzzers exist for the protocols Ze enrolls, first BGP, IKEv2 and OSPF, and what licence and maintenance state each has. No candidate is named here as fact: each is verified in RESEARCH |
| Q-2 | How each one's verdict maps to requirement ids, so a pass can count as evidence under `./le rfc check` (a new carrier kind in `carriers.go`) or stays an advisory report |
| Q-3 | Which pipeline runs each: the `./le verify` stages or the nightly evidence workflow (`.github/workflows/evidence-nightly.yml`) |
| Q-4 | How a failing case is triaged against the RFC text, so a tool's own misreading is not recorded as a Ze defect |

Out of scope: the negative-path fuzzing per RFC error-handling clause, which is
`plan/spec-rfc-evidence-strength-6-error-handling-fuzz.md`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - the interop suites and their vacuity traps
  → Constraint: a test that validates a change must be shown RED under a revert first (`ai/rules/interop-and-goal-validation.md`)
- [ ] `docs/contributing/rfc-conformance-gates.md` "The public ledger's edges" and the evidence kinds
  → Constraint: a tag in a carrier nothing executes is refused; a new carrier kind needs an executing pipeline

**Key insights:**
- Ze's evidence is 97% unit-level by link count; black-box evidence is the kind the ledger has least of

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/carriers.go` - the carrier table; a new evidence kind is a new row there
- [ ] `internal/le/interoplab/zebuild.go` - how the lab builds the ze binary an external tool would drive

**Behavior to preserve:** every existing gate and suite.

**Behavior to change:** see Task.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- decided in RESEARCH

### Transformation Path
1. decided in RESEARCH

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze and an external tool | decided in RESEARCH | No |

### Integration Points
- `./le rfc check` evidence kinds (`carriers.go`) if the result is to count as RFC evidence

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | skeleton |
| No unintended coupling (components stay isolated) | No | skeleton |
| No duplicated functionality (extends existing, does not recreate) | No | skeleton |
| Zero-copy preserved where applicable (refs, not copies) | No | skeleton |
| Registration over hardcoding, outbound | No | skeleton |
| Registration over hardcoding, inbound | No | skeleton |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | see Task | owner brief, 2026-09-24 | scope changes | RESEARCH | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a found defect is large | many red cases in the first run | each defect is fixed under `ai/rules/completion.md` or recorded as a journal row; never a weakened oracle |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | test infrastructure only; defects it finds are product work |
| How is it reverted? | a commit revert |
| Who else touches this path? | `plan/spec-improve-4-conformance-fixtures.md`, `./le test fuzz`, the interop lab |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| decided in DESIGN | → | decided in DESIGN | named in DESIGN (skeleton) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | see Task | drafted in DESIGN |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| drafted in DESIGN | - | - | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| drafted in DESIGN | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| drafted in DESIGN | - | - | |

## Files to Modify
- decided in DESIGN

## Files to Create
- decided in DESIGN

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | test infrastructure |
| YANG validation constraints | N-A | test infrastructure |
| YANG custom validators | N-A | test infrastructure |
| CLI commands/flags | No | an `./le` action is likely; decided in DESIGN |
| CLI grammar (keyword before value) | No | decided in DESIGN |
| Editor autocomplete | N-A | test infrastructure |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | le action |
| Env var registration | N-A | none expected |
| Doctor check for runtime dependencies | N-A | no product dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the summaries whose requirements gain evidence |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/`, `docs/contributing/testing.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` dev-tools row for any new action |
| 16 | Any changed source file referenced by existing doc source anchors? | N-A | skeleton; answered in DESIGN |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Research** -- the questions in Task
2. **Phase: Design** -- through `/ze-spec`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | a finding is reported against the RFC text, quoted, never against the tool's opinion alone |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| drafted in DESIGN | - |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Tool provenance | an external tool is pinned by version and checksum |

### Failure Routing

| Failure | Route To |
|---------|----------|
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations

- skeleton

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** edited spec
- [ ] **Commit B:** `remove plan/spec-rfc-evidence-strength-5-black-box-conformance.md` only
