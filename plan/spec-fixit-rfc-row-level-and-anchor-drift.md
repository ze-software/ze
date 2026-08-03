# Spec: fixit-rfc-row-level-and-anchor-drift

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Five requirement rows state a level or a section anchor that the RFC text does
not support. Each row is gated and proven, so every gate is green over text that
misquotes its source.**

Found on 2026-08-02 by the RFC 7296 extraction sign-off walk and by the independent
review of `plan/spec-rfcgate-2-deferred-rs-replay-evidence.md`, both while closing
`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. Neither was looking for this. Both found it
by reading the RFC beside the row.

**No gate can see any of these.** `make ze-rfc-check` verifies that a row parses,
that its id agrees with its citation, and that a tagged test exists for each
polarity. It never compares a row's TEXT or LEVEL against the RFC. That bound is
stated in `rfc/extraction/README.md`: "It does not judge whether a requirement's
text renders its source sentence faithfully." This is that bound being exercised,
not a gate defect.

### The five rows

| Row | States | The RFC says | Source |
|-----|--------|--------------|--------|
| `RFC7296-2.8-1` | `[MUST]`, "is closed by the endpoint that created it" | "SHOULD be closed by the endpoint that created it" | `rfc/full/rfc7296.txt:2117` |
| `RFC7947-x-3` | `[MUST]`, "MED must be preserved across the route server" | "SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value" | `rfc/full/rfc7947.txt:266` |
| `RFC7296-3.3.6-1` | cites `(§3.3.6)`, "DH group is mandatory for IKE SA negotiation" | §3.3.6 states no such obligation. The mandate is the mandatory Transform Type table in §3.3.3 | `rfc/full/rfc7296.txt`, §3.3.3 |
| `RFC7296-1.7-2` | cites `(§1.7, §2.13)` | The PRF sentence is in §1.7. §2.13 carries no matching sentence | `rfc/full/rfc7296.txt:1201` |
| `RFC7947-x-1` tag prose | test comments read "MUST NOT prepend own AS" | the row itself is correctly `[SHOULD NOT]` | `internal/component/bgp/reactor/forward_rs_test.go`, two tag sites |

The first two hold Ze to MORE than the RFC asks. That is not a conformance loss, and
it is not automatically wrong: an implementation may choose to be stricter. It IS a
misquote, and a reader cannot tell a deliberate choice from a transcription error.

### Why this is not a one line fix

Two ratchets constrain the repair and they pull against each other.

- `check_retired_requirements` refuses to let a requirement id of an enrolled RFC
  disappear from its summary. Deleting a wrong row is not available, deliberately:
  deleting the line is otherwise the cheapest route from red to green. Correcting a
  misquote means editing the TEXT under the SAME id.
- `parse_checklist_line` refuses an id whose section segment disagrees with its
  citation. `RFC7296-3.3.6-1` therefore cannot simply be re-cited to `(§3.3.3)`.
  Fixing the anchor means renumbering, which is what the first ratchet exists to stop.

`RFC7296-1.3.3-1` has the same shape and is already a recorded Known Limitation of
the pilot: its obligation sits in §1.3.2 while its id and citation name §1.3.3. That
row was left alone on purpose. This spec must decide whether that precedent governs
`RFC7296-3.3.6-1`, or whether the two cases differ.

### The owner decision this spec exists to put

`ai/rules/rfc-compliance.md` reserves any change that alters what Ze owes. Lowering
`RFC7296-2.8-1` and `RFC7947-x-3` from MUST to SHOULD lowers it. The question is
which way to fix each row, never whether to leave it:

1. Correct the level to match the RFC, and accept that the coverage ratchet sees a
   requirement change level under a stable id.
2. Keep the stricter level and add a note to the row saying Ze deliberately exceeds
   the RFC here, so the next reader is not misled into thinking the RFC demands it.

Option 2 changes no gate state and keeps the stricter behavior. Option 1 makes the
summary a faithful rendering of its source. They are different claims, and the choice
belongs to the owner.

## Required Reading

| Document | Why |
|----------|-----|
| `ai/rules/rfc-compliance.md` | Governs any change to what Ze owes, and reserves the decision above |
| `rfc/extraction/README.md` | States the bound this finding exercises, and the derived-versus-authored split |
| `rfc/full/rfc7296.txt` §2.8, §3.3.3, §3.3.6, §1.7 | The source text for four of the five rows |
| `rfc/full/rfc7947.txt` §2.2.2.1 and the MED paragraph | The source text for the other two |
| `scripts/dev/rfc_requirements.py` | `check_retired_requirements`, `check_coverage_ratchet`, `parse_checklist_line`: the constraints on the repair |

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `rfc/full/rfc7296.txt:2117` - says the redundant SA "SHOULD be closed by the endpoint that created it"
- [ ] `rfc/full/rfc7947.txt:266` - says MED "SHOULD be propagated" and the RS "SHOULD NOT modify its value"
- [ ] `rfc/short/rfc7296.md` - carries `RFC7296-2.8-1` at `[MUST]`, and the two mis-anchored rows
- [ ] `rfc/short/rfc7947.md` - carries `RFC7947-x-1` at `[SHOULD NOT]` and `RFC7947-x-3` at `[MUST]`
- [ ] `scripts/dev/rfc_requirements.py` - `parse_checklist_line`, `check_retired_requirements`, `check_coverage_ratchet`
- [ ] `internal/component/bgp/reactor/forward_rs_test.go` - two tag comments reading "MUST NOT prepend own AS"

**Behavior to preserve:** (unless the user explicitly said to change it)
- Ze's runtime behavior for all five rows. The code already satisfies each obligation
  at the RFC's real level; only the row text misreports the source.
- Every requirement id. `check_retired_requirements` refuses a disappearing id.

**Behavior to change:** (only what the user asked for)
- The level or the anchor of the rows the owner selects under AC-1, and the two test
  tag comments. Nothing else.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

No runtime data flow changes. The flow that carries the defect is documentary.

### Entry Point
- `rfc/full/<stem>.txt`, the RFC's own sentence, as plain text.
- Read by a human beside the summary, which is the only place the two are compared.

### Transformation Path
1. A person transcribes the sentence into a row in `rfc/short/<stem>.md`, choosing an
   id, a level and a section citation. **The defect is introduced here.**
2. `scripts/dev/rfc_requirements.py` parses the row, gates it, and counts it. It reads
   the summary only; it never opens `rfc/full/<stem>.txt` to compare.
3. `make ze-rfc-index` renders the counts into `ai/RFC-REQUIREMENTS.md`.
4. `docs/features/rfc-status.md` publishes the claim to readers outside the repo.

Stages 2 to 4 propagate stage 1 faithfully, which is why every gate is green.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RFC text ↔ summary row | Human transcription, no machine check | No |
| Summary ↔ published status page | `check_gap_count_agreement` on counts only, never on level | No |

### Integration Points
- `parse_checklist_line` - enforces id-to-citation agreement, which is what blocks a
  bare re-anchor of `RFC7296-3.3.6-1`.
- `check_coverage_ratchet` - keys on polarity; confirm by reading whether a level
  change under a stable id passes it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

| ID | Assumption | Validation | Status |
|----|-----------|------------|--------|
| A-1 | Ze's behavior already satisfies all five rows at the RFC's real level | Read the producing function for each row and confirm | unvalidated |
| A-2 | No third-party claim depends on the stricter level | Read `docs/features/rfc-status.md` rows for both RFCs | unvalidated |
| A-3 | `RFC7296-1.3.3-1` is genuinely the same class as `RFC7296-3.3.6-1` | Compare both against their RFC sections | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | Changing a level trips `check_coverage_ratchet` on an id that keeps its polarities | Read the ratchet before editing; it keys on polarity, not level, so confirm rather than assume |
| R-2 | A renumber to fix an anchor collides with `check_retired_requirements` | Prefer a recorded Known Limitation over a renumber unless the owner rules otherwise |

## Blast Radius

`rfc/short/rfc7296.md`, `rfc/short/rfc7947.md`, the generated `ai/RFC-REQUIREMENTS.md`,
and possibly two tag comments in `internal/component/bgp/reactor/forward_rs_test.go`.
No product code path changes. `docs/features/rfc-status.md` changes only if A-2 breaks.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rfc-check` over the edited summaries | → | `parse_checklist_line`, `check_retired_requirements` | existing `scripts/dev/rfc_requirements_test.py` suite |
| A row whose level is edited under a stable id | → | `check_coverage_ratchet` | `TestLevelChangeUnderStableIdIsAccepted` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The owner question above is put to Thomas | A recorded answer selecting option 1 or 2 per row, captured in Key Design Decisions with the rejected alternative |
| AC-2 | Option 1 is chosen for a row | That row's level and text match the RFC sentence verbatim, and `make ze-rfc-check` exits 0 |
| AC-3 | Option 2 is chosen for a row | That row carries a note saying Ze exceeds the RFC deliberately, naming the RFC's own level |
| AC-4 | `RFC7296-3.3.6-1` and `RFC7296-1.7-2` | Each either cites the section its sentence is in, or is recorded as a Known Limitation with the reason the anchor cannot move |
| AC-5 | The two `RFC7947-x-1` tag comments | They state the row's real level, `SHOULD NOT` |
| AC-6 | `make ze-rfc-check` after every edit | Exit 0, and `ai/RFC-REQUIREMENTS.md` regenerated in the same commit |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads `rfc/short/rfc7296.md` to learn what RFC 7296 demands | summary row → the RFC sentence it quotes | Manual comparison recorded in the audit; no runtime path exists |
| 2 | Runs `make ze-rfc-check` after the edits | summary → `rfc_requirements.py` → verdict | The existing python suite |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLevelChangeUnderStableIdIsAccepted` | `scripts/dev/rfc_requirements_test.py` | AC-2, that the ratchets permit the repair | |
| `TestAnchorMismatchIsRefused` | `scripts/dev/rfc_requirements_test.py` | AC-4, that a re-cited id without a renumber is still refused | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | No numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | none | This spec changes documentation and test prose, not daemon behavior | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | none | none | No wire behavior changes; the rows already match what Ze does | |

## Files to Modify
- `rfc/short/rfc7296.md` - three rows
- `rfc/short/rfc7947.md` - one row
- `internal/component/bgp/reactor/forward_rs_test.go` - two tag comments
- `ai/RFC-REQUIREMENTS.md` - generated, via `make ze-rfc-index`

## Files to Create
- None expected. Tests land in the existing `scripts/dev/rfc_requirements_test.py`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No config surface |
| CLI commands/flags | No | None |
| Functional test for new RPC/API | No | No API changes |
| Doctor check for runtime dependencies | No | No runtime dependency added |
| Prometheus counters/metrics | No | No observable state |
| BGP family surface | No | No family, capability or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | None |
| 2 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` if A-2 breaks; otherwise the published claim is already correct |
| 3 | Wire format changed? | No | None |
| 4 | Internal architecture changed? | No | None |

## Implementation Steps

1. Put the owner question and record the answer per row in Key Design Decisions.
2. Validate A-1 by reading the producing function behind each row.
3. Apply the chosen option to each row, one row per edit, re-running `make ze-rfc-check`.
4. Correct the two `RFC7947-x-1` tag comments.
5. Regenerate `ai/RFC-REQUIREMENTS.md` with `make ze-rfc-index`.
6. Decide `RFC7296-3.3.6-1` and `RFC7296-1.7-2` against the `RFC7296-1.3.3-1` precedent.

## Design Insights

A green RFC gate bounds what was extracted and proven. It does not bound whether the
extraction is a faithful reading. Those are different properties, and only the second
one needs a human beside the source text.

## Key Design Decisions

Not yet taken. AC-1 is the owner's answer and everything else follows from it.

## Known Limitations

`RFC7296-1.3.3-1` already carries an anchor mismatch recorded as a deliberate Known
Limitation of the pilot. Whatever this spec decides should either extend that
treatment or replace it, so the repository holds one rule and not two.

## RFC Documentation (Scope: protocol)

No new RFC behavior. This spec corrects how two existing RFCs are quoted. Both
summaries stay enrolled and no requirement id is retired.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs, or N-A with a reason
- [ ] Functional `.ci` tests for end-to-end behavior, or N-A with a reason
- [ ] Interop tests for protocol features, or N-A with a reason

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Provenance

- `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`, the RFC 7296 extraction sign-off walk
  (2026-08-02): rows one, three and four.
- The independent review of `plan/spec-rfcgate-2-deferred-rs-replay-evidence.md`
  (2026-08-02): rows two and five.
