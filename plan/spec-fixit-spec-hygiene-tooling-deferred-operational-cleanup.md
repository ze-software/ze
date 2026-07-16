# Spec: fixit-spec-hygiene-tooling-deferred-operational-cleanup

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules, spec closure
3. `ai/rules/planning.md` - "Spec Closure" two-commit rule
4. `plan/spec-fixit-spec-hygiene-tooling.md` - the source spec (if still on disk)

## Task

Two operational cleanups that the spec-hygiene TOOLING spec found but deliberately
did not perform, because it was building the checks, not running their output.

**Provenance:** deferred from `plan/spec-fixit-spec-hygiene-tooling.md` (the
"Note (record, do not implement here)" block), recorded 2026-07-17.

**Item 1: close `spec-ipsec-13-rekey-wire`.** The source spec flagged it
HIGH-confidence completed-but-not-closed. Verified 2026-07-17: the file exists and
still declares `| Status | in-progress |`. Closing it means the two-commit rule in
`ai/rules/planning.md` "Spec Closure": commit A saves code + spec + learned summary,
commit B does the `git rm`. Do NOT close it on the strength of the flag alone --
read its Review Gate first and confirm the work actually landed. `/ze-status` flags
in-progress specs with clean Review Gates as "completed but not closed", which is
the signal that produced this item.

**Item 2: prune the un-indexed learned files.** Learned summaries that no index
references. Establish the current set before acting (`make ze-regen` /
`ai/LEARNED-FULL-INDEX.md` are the index side; `plan/learned/` is the source side),
because the set moves as sessions land summaries.

**Why these are not spec-shaped work:** both are chores against repo state, not
design or code. They are filed here so they keep a destination and stay visible to
the commit gate, per `ai/rules/deferral-tracking.md`. Neither needs a design phase;
whoever picks this up can go straight to doing them carefully.

**Caution:** item 1 closes a spec this session did not implement. `ai/rules/planning.md`
Spec Closure is BLOCKING and the learned summary is part of commit A, not a follow-up.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/planning.md` - Spec Closure, the two-commit rule
  → Constraint: commit A = code + spec + learned summary; commit B = `git rm` of the spec. Never one commit.
- [ ] `ai/rules/deferral-tracking.md` - why a chore still needs a home
  → Constraint: a row stays live until the work lands, not until it is filed.

**Key insights:**
- Both items are repo-state chores; the risk is doing them carelessly, not designing them wrong.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/spec-closure-check.py` - the closure check; `--list` prints "every completed-but-not-closed spec (triage view)" (`:20`, flag registered at `:314`; `--json` also available)
  → Constraint: `--list` is the triage view that produced item 1's flag; run it to refresh the candidate set rather than trusting the source spec's snapshot.

**Behavior to preserve:**
- `plan/learned/` numbering: never renumber by hand. `make ze-learned-numbers-check` detects duplicates, `make ze-learned-numbers-fix` resolves them.

**Behavior to change:**
- `spec-ipsec-13-rekey-wire` leaves `in-progress` (closed, or genuinely reopened with a reason).
- Un-indexed learned files are pruned or indexed.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A maintainer (or `/ze-status`) observing an in-progress spec with a clean Review Gate, and index/source drift in `plan/learned/`.

### Transformation Path
1. Read `plan/spec-ipsec-13-rekey-wire.md`'s Review Gate and Implementation Audit
2. If genuinely complete → two-commit closure per `ai/rules/planning.md`
3. Enumerate `plan/learned/*.md` against `ai/LEARNED-FULL-INDEX.md`
4. Prune or index the difference; regenerate with `make ze-regen`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Spec state ↔ /ze-status | spec `Status` field | [ ] |
| `plan/learned/` ↔ generated index | `make ze-regen` | [ ] |

### Integration Points
- `make ze-regen` / `ai/LEARNED-FULL-INDEX.md` - the index side of item 2.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding — N/A, no new feature surface

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `spec-ipsec-13-rekey-wire` is genuinely complete | source spec's HIGH-confidence flag, unverified against its Review Gate | it stays open; only the flag was wrong | read its Review Gate | unvalidated |
| A-2 | The un-indexed learned files are stale rather than merely unindexed | not yet established | pruning would destroy real summaries; index them instead | diff `plan/learned/` against the index | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Closing a spec whose work did not land hides unfinished work | Review Gate not clean, or ACs without evidence | read the gate first; `ai/rules/no-partial-completion.md` |
| R-2 | Pruning a learned file that is referenced, or is the only record of a lesson | `make ze-learned-numbers-check`, grep for references | index instead of prune when in doubt; `ai/rules/never-destroy-work.md` |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| N/A — repo-state chore, no runtime surface | → | N/A | `make ze-regen-check` green after item 2 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `spec-ipsec-13-rekey-wire` reviewed | Either closed via the two-commit rule, or left open with a written reason |
| AC-2 | `plan/learned/` compared against the index | No un-indexed learned file remains; `make ze-regen-check` is green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A — no code change; the gate is `make ze-regen-check` | - | - | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A — repo-state chore | - | - | |

### Future (if deferring any tests)
- None; this spec changes repo state, not behavior. Its evidence is a green `make ze-regen-check` plus the closure commits.

## Files to Modify
- `plan/spec-ipsec-13-rekey-wire.md` - closed (commit B `git rm`s it)
- `plan/learned/` - prune or index the un-indexed files
- `ai/LEARNED-FULL-INDEX.md` - regenerated, never hand-edited

## Implementation Steps

### Implementation Phases
1. **Phase: Verify before acting** — read `spec-ipsec-13-rekey-wire`'s Review Gate; enumerate un-indexed learned files. Resolve A-1 and A-2 before touching anything.
2. **Phase: Close** — two-commit closure if item 1 is confirmed complete.
3. **Phase: Index** — prune or index; `make ze-regen`; confirm `make ze-regen-check` is green.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
