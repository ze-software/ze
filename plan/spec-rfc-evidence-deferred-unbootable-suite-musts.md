# Spec: rfc-evidence-deferred-unbootable-suite-musts

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Status set to `blocked` on 2026-08-05**, from `skeleton`. Reason: blocked: its own Task says the first deliverable is a decision, and the decision is the owner's. It was
reachable from `/ze-status` as actionable until now, which is what a triage of every
`*-deferred-*` spec found.

## Task

242 gated MUST-level requirements cannot be proven at verify tier at all,
because no functional suite boots their subsystem: BFD 98, VRRP 80, dhcpserver
28, geodns 18, dnsserver 18. `mk/test-functional.mk` `all_suites` names no
`bfd`, `vrrp` or `dhcp` suite, so `carrier_for` resolves any `.ci` written there
to `TIER_UNRUN` and the scanner REFUSES the tag. No test fixes this on its own.

**This spec's first deliverable is a decision, and the decision is the owner's.**
It was raised as Q2 of
`plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md`, which is still open
and carries no answer. The row that homes this work lives in
`plan/deferrals/rfcgate-2-deferred-nonunit-evidence-backfill.md`.

### The question to put to Thomas

Three routes, and `ai/rules/rfc-compliance.md` reserves the choice for him
because two of them lower what Ze proves:

| Route | What it costs | What it buys |
|-------|---------------|--------------|
| Add a verify-tier suite per subsystem | new suite infrastructure per subsystem, and the runtime it adds to `make ze-verify` | every one of the 242 becomes provable on every push |
| Accept nightly-only tier for these | a tier that is scheduled and advisory, not merge-gating | reachable today for VRRP, which has `ze-qemu-vrrp-keepalived-test`; the others have no nightly path either |
| Leave them unit-only by decision | the obligation stays proven at the wrong altitude | nothing new to build |

Ask which way, never whether to skip. Do not write `{gap}` for any of the 242:
an annotation that lowers what Ze owes is a compliance decision, not
bookkeeping.

### Constraints

- `functional_suites()` (`scripts/dev/rfc_requirements.py`) reads
  `mk/test-functional.mk` `all_suites` and fails closed when it cannot. A suite
  that is not named there confers no tier, however good its tests are.
- VRRP is the one subsystem with any automated path today
  (`ze-qemu-vrrp-keepalived-test`, nightly). BFD, dhcpserver, geodns and
  dnsserver have none.
- Counts are a 2026-08-02 snapshot. Re-measure by importing
  `scripts/dev/rfc_requirements.py`; do not render `ai/RFC-REQUIREMENTS.md` to
  read a number.

## Required Reading

### Architecture Docs
- [ ] `scripts/dev/rfc_requirements.py` - `CARRIERS`, `carrier_for`, `functional_suites`
  → Constraint: `TIER_UNRUN` is a refusal, and it is the correct interim state rather than a defect.
- [ ] `mk/test-functional.mk` - `all_suites`
  → Constraint: this list is the tier gate.
- [ ] `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md` - the sibling problem for interop trees
  → Decision: that spec gives an existing runner an automated caller; this one asks whether a runner should exist at all.
- [ ] `ai/rules/rfc-compliance.md` - who decides when full proof is not reachable
  → Constraint: ask which way to fix it, never whether to skip it.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5880.md` and the VRRP, DHCP and DNS summaries
  → Constraint: fill at design time, once the route is chosen.

**Key insights:** (minimal context to resume after compaction)
- The blocker is infrastructure, not test-writing skill. A perfect `.ci` in `test/bfd/` earns nothing today.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/dev/rfc_requirements.py` - refuses a tag whose suite `all_suites` does not name

**Behavior to preserve:**
- The `TIER_UNRUN` refusal. It keeps false evidence out and must not be softened to make these 242 look proven.

**Behavior to change:**
- Fill once the route is chosen. Route 1 adds suites; route 2 adds a nightly carrier row; route 3 changes nothing in code.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `# RFC requirement:` tag in a `.ci` under a subsystem directory.

### Transformation Path
1. `scan_ci_tags` reads the tag.
2. `carrier_for` maps the path to a carrier, an evidence kind and an execution tier.
3. An unnamed suite resolves to `TIER_UNRUN` and the tag is refused.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test tree ↔ ledger | `scan_tree` over `.ci` tags | Yes, by the refusal this spec exists to answer |

### Integration Points
- `mk/test-functional.mk` `all_suites` - the list any new suite must join.

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
| A-1 | Each of the five subsystems can be booted by a functional suite at all | they run as daemons or plugins today | route 1 is unavailable for that subsystem and the question narrows | attempt a minimal suite for one subsystem | unvalidated |
| A-2 | The 242 count is stable enough to decide on | measured 2026-08-02 | the decision is taken on a stale size | re-measure before asking | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A new suite is added and `make ze-verify` grows past its budget | verify wall time rises | measure the added runtime per suite before adding the second |
| R-2 | The refusal is softened instead of answered, and 242 requirements gain a tier they do not have | a change to `CARRIERS` or `functional_suites` with no new runner | the tier must follow a runner, never precede it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Route 2 or 3 leaves 242 MUSTs proven at the wrong altitude, which is a public claim outrunning its evidence. Route 1 grows the merge gate |
| How is it reverted? | Route 1 by removing the suite from `all_suites`, which the enrolment ratchet will then flag |
| Who else touches this path? | Any session working `scripts/dev/rfc_requirements.py` or `mk/test-functional.mk` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `.ci` in a newly added suite | → | `functional_suites` then `carrier_for` (`scripts/dev/rfc_requirements.py`) | the tag is accepted rather than refused as `TIER_UNRUN` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The question above is put to Thomas with the counts re-measured | An answer is recorded in this spec, and the routes not taken are recorded with it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Fill once the route is chosen | Fill once the route is chosen | Fill once the route is chosen |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `scripts/dev/rfc_requirements_test.py` | `scripts/dev/` | a newly named suite resolves to a real tier | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Fill at design time | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Fill once the route is chosen | `test/<subsystem>/*.ci` | Fill once the route is chosen | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ze-qemu-vrrp-keepalived-test` | `test/` | keepalived | the one existing nightly path, for VRRP only | |

## Files to Modify
- `mk/test-functional.mk` - `all_suites`, if route 1 is chosen.
- `scripts/dev/rfc_requirements.py` - `CARRIERS`, only after a runner exists.

## Files to Create
- `test/<subsystem>/*.ci` - if route 1 is chosen.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | test infrastructure, no config surface |
| YANG validation constraints | No | no leaf added |
| YANG custom validators | No | no leaf added |
| CLI commands/flags | No | no command added |
| CLI grammar (keyword before value) | No | no command added |
| Editor autocomplete | No | no leaf added |
| Functional test for new RPC/API | Yes | the suites this spec may add |
| Pipe completeness | No | no command output added |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | no observable state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | test infrastructure |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `ai/RFC-REQUIREMENTS.md`, and `docs/features/rfc-status.md` if a support level moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, if a suite is added |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | No | fill at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- re-measure the 242, then put the question to Thomas
   - Tests: `carrier_for` on a draft `.ci` in each of the five subsystems, showing the refusal
   - Files: session scratch only
   - Verify: the counts are current and the refusal is reproduced, not quoted
2. **Phase: Execute the chosen route** -- fill once answered

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | The answer is recorded, and so are the routes not taken |
| Tier honesty | No carrier gains a tier before a runner exists |
| Rule: `ai/rules/rfc-compliance.md` | No `{gap}` written for any of the 242 |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Re-measured counts | import `rfc_requirements` and fold `carrier_for` |
| The recorded answer | this spec's Task section |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a new suite boots daemons on loopback; check no test binds a routable address |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Fill once the route is chosen.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
