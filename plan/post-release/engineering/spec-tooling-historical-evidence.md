# Spec: tooling defects blocked only by missing historical evidence

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Each item reconstructs or attributes a past tooling incident from evidence no longer in the tree; none changes the shipped binary |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas moved these obligations out of `spec-tooling-one-off-defects` on
2026-11-08 so that spec can close: each one's only remaining blocker is
evidence missing from the tree, and he will spend time investigating them
outside agent sessions. Under the rule in `ai/META-DECISIONS.md`, an incident
not investigated in the session that met it is not recorded, so an item stays
here only while someone can still investigate it. The executable integrity
incident (the parent's AC-27) was dropped, not moved, because another session
may have caused it.

On 2026-11-10 two more items moved here under the same rule (AC-6, AC-7).
Each was investigated as far as the tree allows, and what remains is the state
of a run that no longer exists. Also on 2026-11-10, three more 043c682d24
load-red members joined AC-5 beside `update-text-sr-policy-addpath`:
`api-peer-create-delete-rib`, `debug-toggle` and `peer-admin-shutdown`. Only
their failure headers survive (cases 41, 289 and 618 in the "D functional"
excerpt of `tooling-one-off-evidence.md`), with neither a red nor a cause.

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/engineering/spec-tooling-one-off-defects.md` - the parent's proof text for P-12, P-16, P-26, P-34 and P-35
  → Constraint: a recovered member is resolved against its current producer; a fresh green alone does not close it
- [ ] `plan/post-release/engineering/tooling-one-off-evidence.md` - the evidence recovered so far

**Key insights:**
- Every item is evidence-blocked; none is implementable until its evidence is found

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/post-release/engineering/spec-tooling-one-off-defects.md` - the parent spec these items left

**Behavior to preserve:**
- The current gates and tests each item names

**Behavior to change:**
- None until an item's evidence is recovered

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Evidence companion | `plan/post-release/engineering/tooling-one-off-evidence.md` | Holds the partly recovered lint coordinates, the October reconstruction and current hook evidence | Extends: recovered evidence is added there |
| Owned journal rows | `plan/journal/`, the parent's "Owned journal rows" table | Lists the rows the parent fixes | Extends: a row moves here with its obligation |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Recovered historical evidence: a log, an archive, a capture

### Transformation Path
1. Each recovered member is matched to its current producer and test

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | No data path changes | No |

### Changed-Wire Census

**Search evidence:** no wire changes; this spec reconstructs past populations.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| None | - | - | No format is changed | Preserve |

### Integration Points
- The gates and tests each item names

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Decided per item |
| No unintended coupling (components stay isolated) | No | Decided per item |
| No duplicated functionality (extends existing, does not recreate) | No | Decided per item |
| Zero-copy preserved where applicable (refs, not copies) | No | Not applicable to tooling evidence |
| Registration over hardcoding, outbound | No | Decided per item |
| Registration over hardcoding, inbound | No | Decided per item |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each item's evidence may still be recoverable outside the agent sessions | Thomas, 2026-11-08 | The item is dropped under the META-DECISIONS rule | Thomas's investigation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The evidence is never recovered | No new evidence after the investigation | Drop the item under the rule |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: tooling only |
| How is it reverted? | Single commit revert |
| Who else touches this path? | The parent spec's tooling owners |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| The current gate or test a recovered member names | → | its current producer | named per item when its evidence is recovered |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The June technique and fuzz backfill inventories (parent AC-16, P-16) | Recovered, and each lost member is covered by a named current test or gate | A member shown already covered, with its test named |
| AC-2 | The post-OSPF lint population: 93 host, 104 Linux, two capability findings, one uncovered file (parent AC-26, P-26) | Reconstructed from a recovered log, each member resolved against its current producer | A member the current gate already reports, named |
| AC-3 | The authentic October originals and the naming proof archive (parent AC-16, AC-34, P-16, P-34) | Recovered, or the bounded reconstruction shown authentic against a source both sides accept | None |
| AC-4 | The wider GoBGP four-peer configurations, OPENs and captures (parent AC-12, P-12) | Recovered, so the scope beyond the proven stable-session, non-ADD-PATH unicast case is decided on evidence | The proven bounded case, which the parent keeps |
| AC-5 | Each load-red member whose original failure output is lost (parent AC-35, P-35) | Reproduced with its exact wait, readiness event, race or resource bound, then fixed | A member shown not reproducible after a recorded attempt |
| AC-6 | The 2026-10-27 `le-rules-answers` red at `0723d7d26a` (parent AC-34, P-34): `render-update` exited 0 and printed its status page, and the fixture's `ai/rules/aaa.md` still held the OLD rule (`internal/test/fixture/ui_fixture_le_rules_answers.go`, the `requireFile` after the `render-update` run) | The historical cause is named at its producer and a carrier goes red on it. The parent's carrier already goes red when `render-update` writes the old rule back and passes restored, and the one lead, a dotted `ze.repo.root` overriding the stand-in root, was tested and did not reproduce ([evidence](tooling-one-off-evidence.md#p-4-conf-watchdog-identity-ospf-stubnssa-reliability-the-le-rules-answers-lead-2026-11-10)). The failure text survives in `tmp/verify-worktree-logs/20261027T022750.337309046Z-p800009-r1-0723d7d26a13/changed-1676726763/12-test-functional-gating.log` (case 239, 97.9s, eight cases running at once). What is lost is the run itself: its export tree under `tmp/verify-worktree/` and the environment its `le` ran with | None: the carrier's current red and green do not name the old cause |
| AC-7 | The 2026-09-10 post-OSPF gate population (parent AC-26, P-26): `repository check` 24 BGP wiring findings, `commit audit` 16 changed-test findings, `doc check verify` 3759 drift findings and eight summary-rule failures, of which the journal row keeps only the counts | Each member named from a recovered output and resolved against its current producer | A member the current gate already reports, named |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| One red-first test per recovered member | the current producer's package | AC-1 to AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| None | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Named per item | per item | An agent's gate reports the reconstructed member | |

## Files to Modify
- Named per item once its evidence is recovered

## Files to Create
- None yet

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling evidence only |
| YANG validation constraints | N-A | Tooling evidence only |
| YANG custom validators | N-A | Tooling evidence only |
| CLI commands/flags | N-A | Tooling evidence only |
| CLI grammar (keyword before value) | N-A | Tooling evidence only |
| Editor autocomplete | N-A | Tooling evidence only |
| Functional test for new RPC/API | N-A | Tooling evidence only |
| Pipe completeness | N-A | Tooling evidence only |
| Env var registration | N-A | Tooling evidence only |
| Doctor check for runtime dependencies | N-A | Tooling evidence only |
| Prometheus counters/metrics | N-A | Tooling evidence only |
| BGP family surface (new SAFI / capability / attribute) | N-A | Tooling evidence only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Per item | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Per item | derived when files are known |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- name the current gate for each recovered member
   - Tests: the per-item tests above
   - Files: per item
   - Verify: the gate reaches the member
2. **Phase: Reconstruction** -- recover each item's evidence, resolve its members
   - Tests: red-first per member
   - Files: per item
   - Verify: red, fix, green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N is recovered, covered or dropped under the rule |
| Correctness | No member closes on a fresh green alone |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Each recovered member's test | its named test run |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Recovered archives are untrusted data and are read without executing them |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Evidence not recoverable | Drop the item under the `ai/META-DECISIONS.md` rule |
| Test fails for the wrong reason | Fix the test assertion or setup |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-verdict-depends-on-the-machine` | 2026-10-27 | verify changed at 043c682d24 load reds, update-text-sr-policy-addpath, api-peer-create-delete-rib, debug-toggle and peer-admin-shutdown output lost (AC-5) |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-10 | lint after OSPF closure: 93 host, 104 Linux findings (AC-2) |
| `test-type-not-backfilled` | 2026-06-14 | new test type, existing code: June technique inventory (AC-1) |
| `test-type-not-backfilled` | 2026-06-14 | fuzz target backfill: June fuzz inventory (AC-1) |
| `package-red-not-reproduced` | 2026-10-27 | le-rules-answers render-update: historical cause unknown (AC-6) |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-10 | repository check, commit audit and doc check verify red over concurrent work: counts only (AC-7) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Move evidence-only items here | Keep them in the parent; close them as unrecoverable | Thomas, 2026-11-08: the parent closes, and he investigates these outside |

## Known Limitations
- Nothing here is implementable before its evidence is recovered

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated, or dropped under the rule
- [ ] The targeted tests for each recovered member pass in the main checkout, run through `./le test unit <group>` or `./le test functional <suite>` for the packages and suites it reaches

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove` the spec's path only
