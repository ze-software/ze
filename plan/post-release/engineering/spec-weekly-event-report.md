# Spec: weekly-event-report

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling for the weekly public update; no shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The weekly update's RFC section and its "what the checking found" stories need
the week's tooling events: requirements proven by a recorded break, test-judge
refusals and approvals, interop scenarios that gained a recorded red, and
defects the RFC checking surfaced. Today the weekly writer rebuilds the
week-on-week movement by hand from `git show` at pinned revisions.

Thomas agreed (2026-11-09 session) that this is a report DERIVED from records
the tooling already keeps (for example the discrimination records under
`rfc/discrimination/`), over a pinned revision range, and not a new append-only
log that every session writes, because every fact is declared once and a shared
log would collide between sessions. A new tracked record is added only where
the tooling keeps none today (test-judge verdicts are untracked approval files,
for instance), and that addition is part of this spec's design.

Companion: `spec-weekly-news-fragments` (agent-written fragments and the
closure requirement). Both change `ai/skills/ze-weekly-update.md` Phase 1.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - to be read at research
  → Constraint: to be recorded at research

**Key insights:**
- To be recorded at research.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `ai/skills/ze-weekly-update.md` - "The RFC MUST programme is a standing item": the movement is rebuilt by hand from distinct `rid` values under `rfc/discrimination/` at pinned revisions (`git show`)

**Behavior to preserve:**
- To be recorded at research

**Behavior to change:**
- To be recorded at design

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [mechanism] | [file + symbol, or page + section] | [what it does today] | [extends / replaces / does not apply: why] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- To be recorded at research

### Transformation Path
1. To be recorded at research

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| to be recorded | | No |

### Changed-Wire Census

**Search evidence:** to be recorded at design

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- To be recorded at design

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
| A-1 | Most weekly events are already recorded in tracked files | `rfc/discrimination/` records exist | A new record is needed for each event kind | Research | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A derived report reads a record whose meaning changed between revisions | Counts that jump without commits | Pin both endpoints and read each at its own revision |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The weekly writer gets wrong counts; nothing user-visible in Ze until published |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-weekly-news-fragments` (weekly skill Phase 1) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| to be designed | → | | to be named at design |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | to be designed | to be designed | to be designed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be designed | | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| to be designed | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be designed | | | |

## Files to Modify
- `ai/skills/ze-weekly-update.md` - the RFC section reads the report

## Files to Create
- to be designed

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling only |
| YANG validation constraints | N-A | tooling only |
| YANG custom validators | N-A | tooling only |
| CLI commands/flags | Yes | a le report command, named at design |
| CLI grammar (keyword before value) | Yes | checked by `./le cli grammar` |
| Editor autocomplete | N-A | le keywords complete from their declaration |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | Yes | the report answers through the shared renderer |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | tooling |
| 2 | Config syntax changed? | N-A | |
| 3 | CLI command added/changed? | N-A | le command, documented at design |
| 4 | API/RPC added/changed? | N-A | |
| 5 | Plugin added/changed? | N-A | |
| 6 | Has a user guide page? | N-A | |
| 7 | Wire format changed? | N-A | |
| 8 | Plugin SDK/protocol changed? | N-A | |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | |
| 10 | Test infrastructure changed? | N-A | |
| 11 | Affects daemon comparison? | N-A | |
| 12 | Internal architecture changed? | Yes | named at design |
| 13 | Route metadata keys added/changed? | N-A | |
| 14 | Prometheus counters added/changed? | N-A | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | N-A | |

## Implementation Steps

1. **Phase: Wiring** -- to be designed

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| to be designed | |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | to be designed |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derived report over existing records | New append-only event log | Every fact declared once; a shared log collides between sessions |

## Known Limitations
- Agent-written fragments are `spec-weekly-news-fragments`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the package tests of the report through `./le job run label unit-pkg command go test <package>`, and `./le cli grammar` (no `./le test unit` group covers `internal/le`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove <the spec's path in its bucket>`
