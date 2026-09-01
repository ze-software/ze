# Spec: rfc-requirement-reattribution

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/rfc-requirement-reattribution.md` |
| Handoff | - |
| Updated | 2026-09-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Give the RFC ledger a way to say **"this obligation was attributed to the wrong
document"**, and move its proof with it.

Today it cannot be said, and the cost is live: `rfc/short/rfc9582.md` declares
thirteen ids describing ASPA PDUs and RTR version negotiation, while
`rfc/full/rfc9582.txt` is *A Profile for Route Origin Authorizations*, which
obsoletes RFC 6482, whose sections stop at 4.3.3.2 before Section 5 and whose
Section 7 is IANA Considerations. Every one of those ids cites a section its own
document does not have, so no reader can check a single row against the text it
names. The obligations are real and Ze meets them; only the attribution is
wrong. They belong to `draft-ietf-sidrops-8210bis`, whose Section 5.12 is ASPA
and whose Section 7 is Protocol Version Negotiation, committed at revision 27 in
`0ef2c6aad` together with a summary carrying them under
`DRAFT-IETF-SIDROPS-8210BIS-` ids.

The owner approved the correction on 2026-09-01. It was attempted and reverted
the same day, because it is refused by the gates rather than by any decision.

## Required Reading

- `internal/le/rfc/summary.go` — `validateID`
  → Constraint: the expected id is built as `Prefix(stem)` + the section the line
    cites, so the id string ENCODES its address. Its own comment states the
    intent: "The id is anchored to the section it cites, so the two can never
    drift apart."
- `internal/le/rfc/check_ratchets.go` — `checkCoverageRatchet`,
  `checkRetiredRequirements`, `checkEnrolment`
  → Constraint: all three treat that same string as stable IDENTITY.
    `checkCoverageRatchet` keys on `baseline[req.RID]`, and its refusal says "An
    annotation does not substitute for proof that was already there", so no
    marker relieves it.
- `internal/le/rfc/rfc.go` — `annotationKinds`, `SupersededKind`,
  `successorDispositions`
  → Decision: the corpus already separates two registers on purpose. The comment
    says why they must never share a slot: "had superseded joined this set,
    marking a requirement would have EVICTED its {gap}". Attribution is a THIRD
    register and follows the same grain.
- `internal/le/rfc/check_baseline.go`
  → Constraint: the baseline is read from git HEAD by `exec`, not from a
    committed snapshot. There is no baseline file to correct alongside a change,
    so every ratchet fires on the very commit that makes the correction.
- `internal/le/rfc/check_core.go` — the `{superseded: restated <id>}` path
  → Constraint: the one existing relocation marker is conditioned on the
    document having been OBSOLETED, and nothing obsoleted RFC 9582.

## Current Behavior (MANDATORY)

Source files read:

- [ ] `internal/le/rfc/summary.go`
- [ ] `internal/le/rfc/check_ratchets.go`
- [ ] `internal/le/rfc/check_baseline.go`
- [ ] `internal/le/rfc/check_core.go`
- [ ] `internal/le/rfc/rfc.go`

A requirement id does two incompatible jobs. As an ADDRESS it is derived from
the stem and the section, and `validateID` refuses any disagreement. As an
IDENTITY it is the key three monotonic ratchets compare against HEAD.

Those roles coexist while the attribution is right. They contradict each other
the instant it is wrong, because correcting an address necessarily changes the
string, and a changed string reads to every ratchet as a retirement. The four
refusals a correction meets are not four problems; they are one property seen
from four sides.

Measured 2026-09-01 by attempting the correction:

| Move | Refused by | Its words |
|------|-----------|-----------|
| Rewrite the rows' TEXT under the same ids | `validateID` | "id disagrees with its section (§2); expected RFC9582-2-\<n\>" |
| Delete the summary | `checkEnrolment` | "enrolled but rfc/short/rfc9582.md does not exist" |
| Un-enrol the stem | `checkEnrolment` | "Enrolment is monotonic" |
| Move the 13 tags to the draft | `checkCoverageRatchet` | "is no longer proven — the test(s) that covered it at HEAD are gone" |

The escape `checkRetiredRequirements` names in its own refusal, "edit its TEXT
under the same id if the wording was wrong", is the first row of that table: it
cannot apply when the corrected text lives in a different SECTION, which a
different document guarantees.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`./le rfc check`, which `check()` (`internal/le/rfc/check.go`) drives over the
whole corpus.

### Transformation Path

`rfc/short/<stem>.md` → `parseChecklistLine` → `validateID` → `Requirement.RID`
→ the three ratchets, each comparing the current set against the same field read
out of git HEAD by `check_baseline.go`. Proof enters separately: an
`RFC requirement:` tag in a test carrier → `Tag.RID` → `baselinePolarities` →
`checkCoverageRatchet`. The two meet only on the id string.

### Boundaries Crossed

One: the working tree against git HEAD. `check_baseline.go` shells out to git
for every monotonic comparison, so the ratchets compare a file an author is
editing against the committed revision of the same file. Nothing else is
crossed: no network, no plugin boundary, no kernel.

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Attribution is a third register, not a fourth annotation kind | `rfc.go` keeps `SupersededKind` out of `annotationKinds` so marking one cannot evict a `{gap}`; the same argument applies here | A re-attributed row silently loses its coverage disposition | AC-3 asserts a row carries both | unvalidated |
| A-2 | Coverage can follow a pointer without weakening the ratchet | The ratchet's purpose is that evidence cannot vanish; evidence that MOVED has not vanished | The marker becomes a way to drop proof | AC-4 forces the red | unvalidated |
| A-3 | `rfc9582` is the only stem needing this today | measured 2026-09-01 across the corpus | The migration is wider than one stem | AC-6 | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | The marker becomes a general escape from the ratchets | The destination id MUST exist and MUST carry the transferred proof; a pointer to nothing is refused |
| R-2 | An author re-attributes to dodge a red rather than to correct an error | The reason quotes the sentence the destination document states, and the source document's own text is checked for its absence |

## Blast Radius

`internal/le/rfc/` only: the summary parser, three ratchets, `rfc.go`'s
vocabulary, and their fixtures in `selftest_core.go`. No product code. The
corpus changes for one stem, `rfc9582`, and one draft.

## Wiring Test (MANDATORY -- NOT deferrable)

| What | Test |
|------|------|
| The marker reaches `./le rfc check` from a real summary | a fixture stem carrying `{reattributed: ...}` is accepted, and the same stem without it is refused |

## Acceptance Criteria

| ID | When | Then |
|----|------|------|
| AC-1 | A checklist row carries `{reattributed: <destination id>; why}` | it parses, and the destination id's format is validated |
| AC-2 | The destination id does not exist in any summary | `./le rfc check` refuses the marker, naming it |
| AC-3 | A row carries both `{gap}` and `{reattributed: ...}` | both survive; neither evicts the other |
| AC-4 | A tag moves from a re-attributed id to its destination | `checkCoverageRatchet` follows the pointer and reports no loss |
| AC-5 | A tag moves from a re-attributed id to nowhere | `checkCoverageRatchet` still refuses |
| AC-6 | The correction is applied to `rfc9582` | its 13 ids point at `draft-ietf-sidrops-8210bis`, the draft is enrolled, the tags carry the draft's ids, and `./le rfc check` names no `rfc9582` violation |
| AC-7 | `docs/features/rfc-status.md` is read after AC-6 | the RFC 9582 row states what Ze implements of the ROA profile, which is nothing |

## End-to-End User Stories

| Story | Path |
|-------|------|
| An author finds a summary describing the wrong document | marks each row `{reattributed: ...}`, writes the destination summary, moves the tags, and `./le rfc check` stays green because the proof moved with the obligation |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReattributedRowParsesAndKeepsItsAnnotation` | `internal/le/rfc/check_test.go` | AC-1, AC-3 | |
| `TestReattributedRowRefusesAnUnknownDestination` | `internal/le/rfc/check_test.go` | AC-2 | |
| `TestCoverageRatchetFollowsAReattribution` | `internal/le/rfc/check_test.go` | AC-4 | |
| `TestCoverageRatchetStillRefusesProofThatWentNowhere` | `internal/le/rfc/check_test.go` | AC-5 | |

## Files to Modify

- `internal/le/rfc/rfc.go` — the third register's constant and its closed set
- `internal/le/rfc/summary.go` — parse the marker beside the annotation
- `internal/le/rfc/check_ratchets.go` — `checkCoverageRatchet` follows the pointer
- `internal/le/rfc/check_core.go` — refuse a destination that does not exist
- `internal/le/rfc/selftest_core.go` — one fixture per refusal
- `rfc/short/rfc9582.md`, `rfc/enrolled.txt`, `docs/features/rfc-status.md`
- `internal/component/bgp/plugins/rpki/rtr_pdu_test.go`, `rtr_session_test.go` — 13 tags

## Files to Create

- `plan/deferrals/rfc-requirement-reattribution.md`

## Implementation Steps

1. Add the register and its parser. Refuse a malformed or unknown destination.
2. Teach `checkCoverageRatchet` to look up `baseline[req.RID]` through the
   pointer, and force the red for both AC-4 and AC-5.
3. Apply it to `rfc9582`: mark the 13 rows, enrol the draft, move the 13 tags.
   The tag move needs an owner-approval row in `test/rfc-changed.md`; the owner
   approved the re-attribution itself on 2026-09-01.
4. Correct the public row, then `./le rfc index-update`.

## Design Insights

- The id conflates identity with address. Everything else follows from that.
- `{superseded: restated}` is the same shape for a different cause: a document
  that was obsoleted. This spec adds the shape for a document that was never the
  right one.

## Key Design Decisions

- A third register rather than a fourth annotation kind, for the reason
  `rfc.go` already gives about `superseded`.
- Coverage follows the pointer rather than being waived. Evidence that moved has
  not vanished, and evidence that vanished is still refused.

## Known Limitations

- The correction is not automatic: an author still writes the destination
  summary and moves the tags by hand, under owner approval.

## RFC Documentation (Scope: tooling)

Not applicable: this spec changes no protocol behavior. It changes how the
ledger records which document states an obligation.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
