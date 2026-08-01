# Spec: followup-rfc-enrollment

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/1172-rfc-requirement-coverage.md` - the pilot that built the gate this spec extends
4. `ai/RFC-REQUIREMENTS.md` - the generated ledger; its Coverage-by-RFC rollup sizes and ranks this work
5. `rfc/enrolled.txt`, `ai/skills/ze-rfc.md`, `ai/skills/ze-rfc-audit.md`

## Task

Extend the RFC-requirement-coverage gate (built and piloted on RFC 7606 by
`plan/learned/1172-rfc-requirement-coverage.md`) outward to the rest of the RFC
summaries. Today only RFC 7606 is in `rfc/enrolled.txt`; every other summary is listed
in `ai/RFC-REQUIREMENTS.md` marked "not enrolled", so nothing enforces its MUST-level
obligations.

Scope, sized from the ledger's derived Coverage-by-RFC rollup at pilot close (re-derive
before starting; the pilot's fixes may have shifted the numbers):
- **~2136 MUST-level requirements owe work across ~146 summaries**, ranked
  nearest-to-enrollable. Enrolling an RFC means every MUST-level requirement in
  `rfc/short/<stem>.md` is either covered by a positive AND a negative tagged test, or
  carries a reasoned `{gap}` / `{not-applicable}` / `{single-polarity}` annotation.
- **9 summaries capture ZERO of their source RFC's MUSTs and must be re-authored via
  `/ze-rfc` before they can be enrolled**: rfc3630, rfc5187, rfc5303, rfc5304, rfc5310,
  rfc5392, rfc6549, rfc7684, rfc7770.

This is a program, not one spec's work: RFC 7606 alone took a re-author, ~130 tags, an
implemented compliance feature (inner MP_REACH/UNREACH NLRI validation), three other
compliance fixes, and ~10 annotated divergences. Enrolment proceeds RFC-by-RFC,
nearest-to-enrollable first, each as its own reviewable unit.

→ Constraint (`ai/rules/testing.md`, "Back-Fill New Test Types"): the un-enrolled remainder
  must be explicit tracked backlog, never implicit. This spec IS that tracking; the ledger's
  "not enrolled" rollup is its derived, un-rottable record.

**Also decide whether a non-IETF specification can enter this machinery at all.**
Received 2026-07-30 from `plan/deferrals/mcp2026-0-umbrella.md`, whose own spec
closed with the MCP 2026-07-28 cutover. The MCP protocol specification carries
MUST-level obligations, and Ze now implements revision `2026-07-28` in full. No
part of it sits under `make ze-rfc-check`, so no ratchet holds that conformance.

The question is for Thomas, and an implementing session must not answer it
(`ai/rules/rfc-compliance.md`, "Implement Full Compliance. Ask Thomas Only
Before Doing LESS"). A refusal to enroll lowers what Ze owes, and that is a
compliance decision.

Two things need an answer. First, does `rfc/short/` accept a summary of a
document that is not an RFC. Second, does `rfc/enrolled.txt` accept a stem that
`rfc/full/<stem>.txt` cannot hold, because the MCP specification is a website
rather than a text file. No enrollment is the status quo, so this blocks
nothing.
→ Constraint (`ai/rules/derive-not-hardcode.md`): only the test-side `RFC requirement:` tag is
  authored; the ledger derives the reverse. Enrolment adds tags, never hand-written back-links.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `plan/learned/1172-rfc-requirement-coverage.md` - how the gate, tags, polarity rule,
  dispositions, ratchets, ledger, audit, and test-protection hook work.
  → Decision: enrolment is a deliberate act taken once the tests exist; writing a summary does
    NOT enroll it (`rfc/enrolled.txt` header).
- [ ] `ai/skills/ze-rfc.md` - id allocation, polarity, annotations for (re-)authoring summaries.
  → Constraint: ids are allocated once, never renumbered; section-anchored, per-section high-water.

### RFC Summaries (MUST for protocol work)
- [ ] The specific `rfc/short/<stem>.md` for whichever RFC a given enrolment increment targets.
  → Constraint: enrolling an RFC can surface real divergences; each is a `{gap}` disclosure or a
    code fix + user decision, never a faked tag.

**Key insights:**
- The gate is total (every MUST accounted every run); the audit is sampled. Enrolment satisfies
  the gate first, then earns an audit.
- Partial adoption is first-class: the ratchet + rollup make the un-enrolled remainder honest.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/rfc_requirements.py` - the gate; re-derives coverage and enforces enrolment.
- [ ] `ai/RFC-REQUIREMENTS.md` - the "not enrolled" rollup is the authoritative, derived work list.
- [ ] `rfc/enrolled.txt` - currently enrolls only rfc7606.

**Behavior to preserve:** the gate, tag scanner, ledger renderer, ratchets, audit, and
test-protection hook are all built and green; this spec only ADDS enrolments and tags.

**Behavior to change:** `rfc/enrolled.txt` grows; `rfc/short/*.md` gain ids/annotations/tags;
tests gain `RFC requirement:` tags; the 9 zero-capture summaries are re-authored first.

## Data Flow

### Entry Point
- A maintainer adds an RFC stem row to `rfc/enrolled.txt` once its summary's MUSTs are all
  covered-both-polarities or reasoned-annotated.

### Transformation Path
1. `rfc_requirements.py --check` re-derives coverage for the newly enrolled RFC and FAILS until
   every MUST-level requirement is covered or annotated.
2. Tagging enforcing tests (`RFC requirement: <id> <polarity>`) makes the derived ledger show the
   requirement→test link; the gate turns green for that RFC.
3. `make ze-rfc-index` regenerates `ai/RFC-REQUIREMENTS.md`; `ze-doc-test` fails if it is stale.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Summary ⇄ Test | requirement id in `rfc/short/<stem>.md` ↔ `RFC requirement:` tag in the test | `make ze-rfc-check` per enrolled RFC |
| Requirement ledger ⇄ Product ledger | `{gap}` disposition ↔ `docs/features/rfc-status.md` Remaining column | gate cross-check (existing) |

### Integration Points
- `rfc/enrolled.txt` (grows only), `ai/RFC-REQUIREMENTS.md` (derived), `docs/features/rfc-status.md`
  (product ledger reconciled per enrolled RFC).

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Add an RFC stem to `rfc/enrolled.txt` | → | `rfc_requirements.py --check` gates its MUSTs | `make ze-rfc-check` exits non-zero until covered/annotated (existing gate; per-increment) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| (per increment) the newly tagged enforcing tests for each MUST | the RFC's existing `*_test.go` / `.ci` | that RFC's coverage, both polarities |

### Functional Tests
N/A at skeleton stage (developer-tooling enrolment). Per-increment: any enrolled RFC whose
enrolment changes wire behavior adds a `.ci` in the matching `test/` directory, mirroring the
RFC 7606 pilot's `test/plugin/rfc7606-reset.ci`.

## Files to Modify
- `rfc/enrolled.txt` - one row per newly enrolled RFC (grows only)
- `rfc/short/<stem>.md` - ids, `{gap}`/`{single-polarity}`/`{not-applicable}` annotations; re-author the 9 zero-capture summaries
- `docs/features/rfc-status.md` - reconcile each enrolled RFC's row with any disclosed `{gap}`

## Implementation Steps

Design not started. When picked up, run `/ze-spec` to:
1. Re-derive the nearest-to-enrollable ranking and the zero-capture list from `ai/RFC-REQUIREMENTS.md`.
2. Choose increment granularity (per-RFC vs cluster) and ordering.
3. Decide the triage for divergences found during enrolment (fix the code vs disclose a `{gap}`).
4. Fill full Acceptance Criteria and the review-gate sections the template requires, then enroll
   RFCs one increment at a time, each green through `make ze-rfc-check` before the next.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An RFC stem added to `rfc/enrolled.txt` | `make ze-rfc-check` passes only when every MUST-level requirement is covered-both-polarities or reasoned-annotated |
| AC-2 | A zero-capture summary (e.g. rfc5303) before enrolment | Re-authored via `/ze-rfc` so its source MUSTs are captured, before its stem may be enrolled |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The rollup's nearest-to-enrollable ranking is still accurate after the pilot's fixes | `ai/RFC-REQUIREMENTS.md` derived rollup | Re-rank before starting | Re-run `make ze-rfc-index` and read the rollup | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The program stalls half-done and rots | Enrolment count flat over time | The ratchet + rollup keep partial adoption honest and visible; each increment is independently valuable |

## RFC Documentation

Per enrolled RFC, enforcing code keeps its `// RFC NNNN Section X.Y: "quoted requirement"` comment
and the enforcing test carries the machine-checked `// RFC requirement: <id> <polarity>` tag.

## Checklist

### Goal Gates (MUST pass)
- [ ] Each enrolled RFC: every MUST-level requirement covered-both-polarities or reasoned-annotated
- [ ] Tests written for each newly tagged requirement
- [ ] Tests FAIL before the enforcing code/tag exists
- [ ] Tests PASS after
- [ ] `make ze-test` passes
- [ ] `make ze-rfc-check` green for every enrolled RFC

### Quality Gates (SHOULD pass)
- [ ] Each enrolled RFC earns a `/ze-rfc-audit` pass

## Notes

Owns `rfc/enrolled.txt` and the Coverage-by-RFC rollup going forward. Inherited from the deferral
row filed by `plan/spec-rfc-requirement-coverage.md` (now closed; knowledge in
`plan/learned/1172-rfc-requirement-coverage.md`).
