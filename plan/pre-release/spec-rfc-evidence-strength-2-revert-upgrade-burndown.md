# Spec: RFC evidence strength 2 -- upgrade the grandfathered revert proofs, one document at a time

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-evidence-strength-1-targeted-mutant-ratchet.md` (the upgrade mode, the backlog count, and the pilot's cost per record) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Replace every verified `revert` record on a mutatable unit carrier with a `mutant`
record, one RFC at a time, until the upgrade backlog that child 1 publishes is 0 or
holds only the units the owner ruled on under D-4
(`plan/pre-release/spec-rfc-evidence-strength-0-umbrella.md`).

Measured 2026-09-24: 1559 revert records sit on Go unit carriers, over 597 producer
functions in 72 packages. The largest documents are rfc2661 (128), rfc9190 (86),
rfc5798 (82), rfc4301 (73), rfc3748 (60), rfc7296 (59), rfc8907 (56), rfc9552 (55).
The largest producer packages are `internal/core/eap` (179), `internal/component/l2tp`
(128), `internal/component/ike/engine` (107) and `internal/component/bgp/reactor` (92).
The pilot in child 1 upgrades rfc7606 and rfc4271 first.

This spec stays a skeleton until child 1's pilot has measured the cost per record and
the rate of units with no targeted mutant. Those two figures set the schedule and the
size of each work package.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the proof routes and the route ratchet child 1 adds
  → Constraint: an upgrade re-observes the red; it never copies a revert record's fingerprints into a mutant record

**Key insights:**
- One gomu report per producer PACKAGE serves every record in that package, so the natural work package is a package, and the natural progress unit is a document

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `rfc/discrimination/*.json` - 1576 revert records, 1559 on unit carriers

**Behavior to preserve:** every verified proof stays verified until its replacement is recorded.

**Behavior to change:** the route of each upgraded record.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc discriminate stem <stem> report <gomu.json> upgrade`, then `./le rfc discriminate-record ... route mutant`

### Transformation Path
1. one gomu run per producer package
2. candidates per revert-proven unit
3. one recorded mutant record replaces each revert record

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le tool and gomu | report file | No |

### Integration Points
- child 1's backlog count is the progress measure

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | records are written only by `discriminate-record` |
| No unintended coupling (components stay isolated) | Yes | artifacts only; test edits follow D-4 |
| No duplicated functionality (extends existing, does not recreate) | Yes | uses child 1's tooling |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling |
| Registration over hardcoding, outbound | N-A | no code |
| Registration over hardcoding, inbound | N-A | no code |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Most revert-proven units kill at least one targeted mutant | the 73 existing mutant records show units that do | many units need a strengthened test and an owner approval each (D-4) | child 1 pilot rate | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Upgrading exposes weak tests in bulk | pilot rate above 10% | batch the D-4 approvals per document and present them to the owner as one table per document |
| R-2 | A record staled by a producer edit during the burn-down | `discrimination:` stale line for a stem in progress | re-observe; the ratchet already refuses a stale committed record |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Proof records only; no product behavior |
| How is it reverted? | a commit revert per document |
| Who else touches this path? | any session that records proofs for the same stem |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | child 1 route-mix line | `TestCheckPrintsDiscriminationRouteMix` (child 1); this child adds no code |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | each document finished by this spec | `./le rfc discriminate stem <stem>` lists no "upgrade owed" unit outside the D-4 finding list |
| AC-2 | programme end | `./le rfc check` prints an upgrade backlog of 0, or the D-4 list count |
| AC-3 | each unit with no targeted mutant | either a strengthened test with an `RFC-approved:` trailer and a mutant record, or a finding row the owner ruled on; never a silently retained revert record |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N-A | - | no code; each record is its own observed red | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | the gate re-verifies each record | |

## Files to Modify
- `rfc/discrimination/<stem>.json` - one document per commit
- tagged `*_test.go` files - only under D-4 with the owner's approval

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no code |
| YANG validation constraints | N-A | no code |
| YANG custom validators | N-A | no code |
| CLI commands/flags | N-A | no code |
| CLI grammar (keyword before value) | N-A | no code |
| Editor autocomplete | N-A | no code |
| Functional test for new RPC/API | N-A | no code |
| Pipe completeness | N-A | no code |
| Env var registration | N-A | no code |
| Doctor check for runtime dependencies | N-A | no code |
| Prometheus counters/metrics | N-A | no code |
| BGP family surface (new SAFI / capability / attribute) | N-A | no code |

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
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `ai/RFC-REQUIREMENTS.md` regenerated per document |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | N-A | only test files change, under D-4 |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Schedule** -- from child 1's pilot figures, order the documents and size the packages
2. **Phase: Burn-down** -- one document per commit, largest backlog first

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | every upgraded record was observed, not copied |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Backlog at 0 or at the D-4 count | `./le rfc check` upgrade-backlog figure |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Weakened claim | a claim reworded narrower to fit a mutant is a scope change; it needs the owner's ruling |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Unit kills no targeted mutant | D-4 |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One document per commit | one package per commit | the ledger measures documents; a package spans many |

## Known Limitations

- `.ci` and interop revert records (17) are outside this spec (D-3).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] AC-N rows carry testable assertions

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
- [ ] **Commit A:** records + edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-evidence-strength-2-revert-upgrade-burndown.md` only
