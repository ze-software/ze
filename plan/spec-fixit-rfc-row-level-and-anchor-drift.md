# Spec: fixit-rfc-row-level-and-anchor-drift

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `-` |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Five requirement rows state a level or a section anchor that the RFC text does
not support. Each row is gated and proven, so every gate is green over text that
misquotes its source.**

Found on 2026-08-02 by the RFC 7296 extraction sign-off walk and by the independent
review of `spec-rfcgate-2-deferred-rs-replay-evidence` (closed 2026-08-03 in `15dac5bc4`; written without its `plan/` path because `spec-citation-check.py` reads any such path as a LIVE citation and the file is gone. Its record was retired with the learned corpus), both while closing
the rfcgate-1b RFC 7296 pilot spec. Neither was looking for this. Both found it
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
| No bypassed layers (data flows through the intended path) | N/A | No runtime path changes. `git diff -- internal/` over this spec's files is comment and test prose only; `forward_med.go`'s diff is three comment lines |
| No unintended coupling (components stay isolated) | N/A | No import, signature or call site changes |
| No duplicated functionality (extends existing, does not recreate) | Yes | The two new tests land in the existing `scripts/dev/rfc_requirements_test.py`; no new file, no new helper |
| Zero-copy preserved where applicable (refs, not copies) | N/A | No buffer, pool or wire code touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N/A | Nothing registers; the change is summary text, one comment and two test classes |

## Risks & Assumptions

| ID | Assumption | Validation | Status |
|----|-----------|------------|--------|
| A-1 | Ze's behavior already satisfies all five rows at the RFC's real level | Read the producing function for each row and confirm | confirmed |
| A-2 | No third-party claim depends on the stricter level | Read `docs/features/rfc-status.md` rows for both RFCs | confirmed |
| A-3 | `RFC7296-1.3.3-1` is genuinely the same class as `RFC7296-3.3.6-1` | Compare both against their RFC sections | confirmed |

A-1 evidence: `medPropagationAllowedTo` (internal/component/bgp/reactor/forward_med.go) is
`!isEBGP || rsClient`, and `applyFactsMED` in the same file returns before it records any
strip when that answer is true. Ze's AUTOMATIC RFC 4271 Section 5.1.4 strip therefore never
fires toward a route server client, which meets RFC 7947 Section 2.2.3's SHOULD. It is not
unconditional in the wider sense: an operator's own `med-remove` modify policy on a peer's
import chain rewrites the payload upstream of this predicate, which
test/plugin/med-removal-configured.ci asserts against an `rs-client true` receiver. That is
Section 5.1.4's own required removal mechanism, and a SHOULD is what explicit operator
configuration may override.

`localNonceIsLower` (internal/component/ike/engine/rekey.go) is
`bytes.Compare(local, remote) < 0`, the octet-by-octet comparison RFC 7296 Section 2.8.1
names. Both call sites in `handleCreateChildSAOwned` (internal/component/ike/engine/inbound.go)
abandon our own exchange when ours is the lower nonce, and each is gated: a pending rekey of
the matching kind must exist, and the peer's request must actually carry a nonce. A malformed
request with no Ni deliberately never makes us abandon an in-flight rekey. Within those
conditions the rule is applied with no configuration input, which meets that section's SHOULD.

A-2 evidence: the RFC 7947 row of `docs/features/rfc-status.md` claims "MED preserved
(verbatim wire forwarding)", a statement about what Ze does, not about what the RFC
demands. The RFC 7296 row makes no claim about the rekey collision rule. Neither row
carries a spelled gap count that the two demotions move, and neither demotion adds or
removes a `{gap}`. No ledger edit is owed.

A-3 evidence: both ids anchor to a section next to the one that carries their obligation.
`RFC7296-1.3.3-1` cites §1.3.3 while "The KEi payload MUST be included" is in §1.3.2.
`RFC7296-3.3.6-1` cites §3.3.6 while the mandate is the mandatory Transform Type table of
§3.3.3. The precedent governs, and the two are now recorded under one rule in
`rfc/short/rfc7296.md`.

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
- `rfc/short/rfc7296.md` - three rows plus the Known Limitation and two Correction notes
- `rfc/short/rfc7947.md` - one row plus its Correction note
- `internal/component/bgp/reactor/forward_rs_test.go` - three tag comments
- `internal/component/bgp/reactor/forward_med.go` - one comment, which called `RFC7947-x-3` a gated MUST
- `internal/component/bgp/reactor/forward_med_test.go` - one comment, same defect (added in review round 1)
- `rfc/enrolled.txt` - the RFC 7947 note's MUST count, its Section 2.2.3 quote, and a stale polarity clause
- `rfc/extraction/rfc7296.json` - the `1.7:6` mapping reason, which named §2.13
- `scripts/dev/rfc_requirements_test.py` - `GATED_FLOOR` 222 -> 221, plus the two tests below
- `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/rfc7296.md`, `rfc/requirements/rfc7947.md`,
  `rfc/requirements/rfc4271.md` - generated, via `make ze-rfc-index`. The rfc4271 shard is
  in the set because the `forward_med_test.go` comment fix shifted three of its citations

## Files to Create
- None. Tests land in the existing `scripts/dev/rfc_requirements_test.py`.

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

**D-1 (2026-08-15, owner direction relayed through the main thread): option 1 for both
level rows.** The level is corrected to match the RFC, under the same requirement id, and
the code is not weakened. Ze keeps the stricter behavior and every test that proves it.
Rejected alternative: option 2, keeping `[MUST]` with a note saying Ze exceeds the RFC. It
changes no gate state, but it leaves the summary misquoting its source, and a reader
comparing the two still cannot tell a choice from a transcription error. The note option is
not lost: each corrected row carries a Correction paragraph that states the RFC's own level
AND records that Ze exceeds it, which is what option 2 was for.

**D-2: `RFC7296-3.3.6-1` keeps its id, its `[MUST]` and its `(§3.3.6)` citation.** The
`RFC7296-1.3.3-1` precedent governs (A-3). Renumbering is refused by
`check_retired_requirements`, and re-citing without renumbering is refused by
`_validate_id`. The repair is in the row's TEXT, which now names §3.3.3 as the section the
mandate is in, plus one Known Limitation note covering both ids so the repository holds one
rule and not two. Rejected alternative: re-texting the row to §3.3.6's own
"missing a mandatory Transform Type" sentence. That sentence is already the source of
`RFC7296-3.3.6-4` (site 3.3.6:6 in `rfc/extraction/rfc7296.json`), so the row would
duplicate a sibling.

**D-3: `RFC7296-1.7-2` cites `(§1.7)` alone.** §1.7 is where the MUST sentence is. §2.13
states the same property as an assumption and carries no MUST about variable-sized keys, so
citing it pointed a reader at a section that does not hold the obligation. The id already
anchors to §1.7, so nothing renumbers.

**Measured effect of D-1 on the gated population.** `rfc7296` 222 gated of 227 rows before,
221 of 227 after. `rfc7947` 3 of 6 before, 2 of 6 after. Repo-wide over every summary,
3045 before and 3043 after. No row was deleted and no id moved. None of the seven ratchets
reads a level: `check_coverage_ratchet` and `check_evidence_ratchet` compare tag polarity
and carrier sets per id and every tag stays, `check_retired_requirements` compares ids,
and `check_status_completeness`, `check_status_agreement` and `check_gap_count_agreement`
key on enrolment and on `{gap}` annotations. The extraction sign-off is also unaffected:
both demoted-or-retexted ids are `unsourced-ids` entries rather than mapped sites, and only
a MAPPED site's target must be gated, which `check_extraction_signoff` enforces in
`scripts/dev/rfc_requirements.py`.

## Resolved: two self-test floors pinned the pre-correction figures (owner call, 2026-08-15)

**Thomas authorised both one-line edits on 2026-08-15, and both have landed.** The RFC 7947
note in `rfc/enrolled.txt` now says two MUST-level transparency requirements, quotes Section
2.2.3, and drops the stale `{single-polarity: positive}` clause. `GATED_FLOOR` in
`scripts/dev/rfc_requirements_test.py` is 221, and its comment carries the row that moved and
the RFC sentence that moved it, so a future decrease that cannot name both reads as the
deletion the floor refuses. `make ze-rfc-check` exits 0, selftest included: 2965 gated
MUST-level requirements across 171 enrolled RFCs, 3539 tags resolved.

The observation below stands and was NOT acted on here: it names a gap in a case this spec
did not author, and closing it is separable work.

### The state that made the call (kept for the record)

`python3 scripts/dev/rfc_requirements.py --check` exits 0 over the corrected tree: every
ratchet, every ledger edge and every extraction sign-off passes. `make ze-rfc-check` is
still red, because its FIRST half is `--selftest` and two cases in
`scripts/dev/rfc_requirements_test.py` record the figures that held while the misquotes
were in the tree. Neither is one of the seven ratchets. Each needs a one-line edit that
lowers or rewords a recorded figure, which is reserved (`ai/rules/rfc-compliance.md`), so
this phase stopped rather than making it.

| Case | Why it fires | The one-line repair |
|------|--------------|--------------------|
| `test_every_level_cited_in_an_enrolment_note_matches_its_summary` | The RFC 7947 note in `rfc/enrolled.txt` calls `RFC7947-x-3` a MUST. The test's own message rules the direction: "The summary owns the level" | Correct that note's prose. The same clause is stale for a second reason that predates this spec: it still calls `RFC7947-x-3` `{single-polarity: positive}`, which the 2026-08-14 correction removed when `test/plugin/med-not-propagated-across-as.ci` gave it a real negative. The note also opens "three MUST-level transparency requirements", now two |
| `test_rfc7296_every_gated_row_is_proven_in_both_polarities` | `GATED_FLOOR = 222`, the RFC 7296 pilot's landed figure, against 221 now | The floor's own comment says it exists so that "deleting rows must not be the cheap way to keep this case green". No row was deleted: the file still carries 227 rows and `test_rfc7296_ids_are_neither_retired_nor_demoted` still passes. The floor cannot tell a deletion from a level correction, so lowering it to 221 needs the owner to say that a corrected level is not a lost row |

A related observation for whoever takes that decision:
`test_rfc7296_ids_are_neither_retired_nor_demoted` is named for demotion but cannot see one.
It builds its baseline from `r.gated and r.rid in PRE_PILOT_IDS`, so a demoted id leaves the
baseline instead of failing against it. `RFC7296-2.8-1` is in `PRE_PILOT_IDS`, and that case
passed through this change without noticing it.

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

- the rfcgate-1b RFC 7296 pilot spec, the RFC 7296 extraction sign-off walk
  (2026-08-02): rows one, three and four.
- The independent review of `spec-rfcgate-2-deferred-rs-replay-evidence`
  (2026-08-02): rows two and five.

---

## Implementation Summary

### What Was Implemented

Five rows corrected, each under its existing requirement id, with no row deleted and no id
renumbered. Every RFC sentence below was re-read in `rfc/full/` at closure, not taken from
the spec's own record.

| Row | Change | RFC sentence that governs |
|-----|--------|---------------------------|
| `RFC7296-2.8-1` | `[MUST]` -> `[SHOULD]`, text re-quoted | §2.8.1: "If redundant SAs are created through such a collision, the SA created with the lowest of the four nonces used in the two exchanges SHOULD be closed by the endpoint that created it" |
| `RFC7947-x-3` | `[MUST]` -> `[SHOULD]`, text re-quoted | §2.2.3: "if applied to an NLRI UPDATE sent to a route server, this attribute SHOULD be propagated to other route server clients, and the route server SHOULD NOT modify its value" |
| `RFC7296-3.3.6-1` | id, level and `(§3.3.6)` citation kept; TEXT now names §3.3.3 | §3.3.3 table: IKE mandatory types are "ENCR, PRF, INTEG*, D-H". §3.3.6: "a proposal that is missing a mandatory Transform Type ... MUST consider this proposal unacceptable" |
| `RFC7296-1.7-2` | citation `(§1.7, §2.13)` -> `(§1.7)` | §1.7 carries the sentence. §2.13 states only "It is assumed that PRFs accept keys of any length, but have a preferred key size" and no MUST about variable-sized keys |
| `RFC7947-x-1` tag prose | "MUST NOT" -> "SHOULD NOT" at three sites | §2.2.2.1: "the route server SHOULD NOT prepend its own AS number ... This is a recommendation rather than a requirement" |

Consequent edits, each authorised by the owner on 2026-08-15: the RFC 7947 note in
`rfc/enrolled.txt` (three MUST-level -> two, Section 2.2.3 quoted, a stale
`{single-polarity: positive}` clause dropped), and `GATED_FLOOR` 222 -> 221 in
`scripts/dev/rfc_requirements_test.py` with the row and the RFC sentence recorded in its
comment. One comment in `internal/component/bgp/reactor/forward_med.go` stopped calling
`RFC7947-x-3` a gated MUST. The `1.7:6` mapping reason in `rfc/extraction/rfc7296.json`
stopped naming §2.13.

### Bugs Found/Fixed

- The two tests the Wiring Test table and the TDD plan named did not exist. Written at
  closure as `TestAnchorMismatchIsRefused` and `TestLevelChangeUnderStableIdIsAccepted`
  in `scripts/dev/rfc_requirements_test.py`, seven cases, all green.
- The misquote had survived in three more places the implementation missed: the Key
  Requirements table of `rfc/short/rfc7947.md`, the Section 2.8 row of the summary table
  in `rfc/short/rfc7296.md`, and a comment on `TestForwardKeepsMEDForRouteServerClient`.
  Review round 1 found all three. Correcting a row without sweeping its own file for the
  same sentence is what left them.
- One claim written during the correction was itself false: "no configuration can turn
  that off" about MED toward a route server client. `test/plugin/med-removal-configured.ci`
  disproves it. Corrected in all three places it had been written.
- No defect in shipped behavior was found. Ze's runtime behavior is unchanged by every
  edit in this spec; the whole diff is summary text, comments and tests.

### Documentation Updates

Two owed, both found by review round 4 rather than by the original checklist, and both the
same defect the whole spec is about: a claim stated more strongly than its source supports.

- `docs/features/rfc-status.md`, RFC 7947 row. It claimed RS transparency flatly: "no
  AS_PATH prepend, no NEXT_HOP rewrite, MED preserved (verbatim wire forwarding)". Both
  named attributes are recommendations the operator may override, so the row now says
  transparency holds BY DEFAULT, names Sections 2.2.2.1 and 2.2.3 as recommendations, and
  lists `as-path-prepend` and `med-remove` in Remaining as the policies that change what an
  RS client receives.
- `docs/guide/ipsec.md`, Rekeying. It rendered RFC 7296 Section 2.8.1's SHOULD as fact:
  "The endpoint that created the SA with the lowest of the four nonces closes that SA". It
  now states the recommendation and says Ze follows it. Same class as the summary-table row
  corrected in `rfc/short/rfc7296.md`.
- `make ze-doc-test` PASSED after both edits: 3027 digest anchors across 23 digests resolve.
- The RFC 7296 row of `docs/features/rfc-status.md` needed no edit: it makes no claim about
  the rekey collision rule. Neither row carries a spelled gap count that the demotions move,
  and `check_gap_count_agreement` is green.
- `git grep -n "7947" -- docs/ ai/` returns two level-adjacent hits and neither is stale.
  `docs/architecture/bgp/egress-attribute-rules.md` names "RFC 7947 Section 2.2.3" as the
  MULTI_EXIT_DISC exemption and its gating condition `PeerSettings.RSClient`, with no level
  word. `docs/architecture/meta/filter-community.md` says "RFC 7947 requires route-server
  transparency" as a statement about the document's purpose, not about `RFC7947-x-3`.
- Source anchors on the changed files: `docs/guide/configuration.md` anchors on
  `applyFactsMED` and `docs/architecture/bgp/egress-attribute-rules.md` on
  `medPropagationAllowedTo`, both in `forward_med.go`. That file's diff is three comment
  lines and no claim in either doc goes stale.
- `make ze-doc-test` not required: no `docs/` file changed.

### Deviations from Plan

- The spec left `RFC7296-3.3.6-1` at `[MUST]` (D-2) and this holds up on re-reading, for a
  reason the spec did not state: §3.3.6, the section the row CITES, carries its own MUST
  ("MUST consider this proposal unacceptable"). The row's level is therefore supported by
  its own citation, not only by §3.3.3's table. The corrected text now says both halves.
- Two files outside the spec's Blast Radius were edited: `rfc/enrolled.txt` and
  `rfc/extraction/rfc7296.json`. Both are consequences of the demotions and both are named
  in Files to Modify now.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The Wiring Test and TDD tables named two tests as though they existed | Neither `TestLevelChangeUnderStableIdIsAccepted` nor `TestAnchorMismatchIsRefused` was in the tree; the ratchets were argued in the spec's prose instead | `grep -n` for both names in `scripts/dev/rfc_requirements_test.py` at closure step 1 returned nothing | Both written, six cases, each paired with a discriminating case so "always silent" and "always refuses" cannot pass |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Five misquoting rows corrected under stable ids | Done | `rfc/short/rfc7296.md`, `rfc/short/rfc7947.md` Compliance Checklists | No row deleted, no id renumbered |
| No requirement id retired | Done | `check_retired_requirements`, green in `make ze-rfc-check` | 227 rows still in `rfc/short/rfc7296.md` |
| No proof lost with the demotions | Done | `git grep -n "RFC requirement: RFC7296-2.8-1\|RFC requirement: RFC7947-x-3"` | 6 tags on 2.8-1 (3 positive, 3 negative) and 4 on x-3 (3 positive, 1 negative), identical to HEAD |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | D-1 in Key Design Decisions | Option 1 for both level rows, with option 2 recorded as the rejected alternative |
| AC-2 | Done | The `RFC7296-2.8-1` and `RFC7947-x-3` rows; `make ze-rfc-check` exit 0 | Both rows now quote their RFC sentence |
| AC-3 | Done | The Correction paragraph on each corrected row | Option 2's content kept: each says Ze exceeds the RFC and names the RFC's own level |
| AC-4 | Done | The Known Limitation note above `rfc/short/rfc7296.md`'s Compliance Checklist, covering both ids; the `RFC7296-1.7-2` row | `RFC7296-1.7-2` re-cited with no renumber; `RFC7296-3.3.6-1` recorded |
| AC-5 | Done | `TestReactorForwardRSEBGPPrepend` and `TestReactorForwardRSTransparent` tag comments in `forward_rs_test.go` | Three sites, not the two the spec predicted |
| AC-6 | Done | `make ze-rfc-check` exit 0; `ai/RFC-REQUIREMENTS.md` regenerated | See Pre-Commit Verification for the commit-scoping decision |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLevelChangeUnderStableIdIsAccepted` | Done | `scripts/dev/rfc_requirements_test.py` | Written at closure; 3 cases |
| `TestAnchorMismatchIsRefused` | Done | `scripts/dev/rfc_requirements_test.py` | Written at closure; 3 cases |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `rfc/short/rfc7296.md` | Done | Three rows, one Known Limitation, two Corrections |
| `rfc/short/rfc7947.md` | Done | One row, one Correction |
| `internal/component/bgp/reactor/forward_rs_test.go` | Done | Three tag comments |
| `ai/RFC-REQUIREMENTS.md` | Done | Regenerated |
| `internal/component/bgp/reactor/forward_med.go`, `rfc/enrolled.txt`, `rfc/extraction/rfc7296.json`, `scripts/dev/rfc_requirements_test.py` | Changed | Added to Files to Modify; recorded in Deviations |

### Audit Summary
- **Total items:** 15
- **Done:** 14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the four files added beyond the original Blast Radius, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every gated row's level and anchor is supported by the RFC sentence it cites | Manual comparison against `rfc/full/`, recorded verbatim | The five sentences quoted in Implementation Summary, each re-read at closure from `rfc/full/rfc7296.txt` §2.8.1/§3.3.3/§3.3.6/§1.7/§2.13 and `rfc/full/rfc7947.txt` §2.2.2.1/§2.2.3 |
| The repair costs no proof: a corrected level does not take its tests with it | Unit test, both directions | `TestLevelChangeUnderStableIdIsAccepted`: the demotion keeping both polarities is silent, and the demotion that DROPS the negative still fires. Tag census unchanged from HEAD (6 on `RFC7296-2.8-1`, 4 on `RFC7947-x-3`) |
| The anchor repair could not have been done by moving the citation | Unit test, both directions | `TestAnchorMismatchIsRefused`: re-citing `RFC7296-3.3.6-1` to `(§3.3.3)` raises `ParseError` naming `RFC7296-3.3.3-<n>`, while `(§3.3.6)` and the multi-section `(§2.8, §2.8.1)` both parse |
| Ze's behavior is unchanged and still meets both demoted requirements | Producing function read at closure | `medPropagationAllowedTo` (`forward_med.go`) is `!isEBGP \|\| rsClient`, so Ze's automatic Section 5.1.4 strip never fires toward an RS client. An operator's `med-remove` policy still removes the metric upstream of it, which is Section 5.1.4's own required mechanism (`test/plugin/med-removal-configured.ci`). `localNonceIsLower` (`rekey.go`) is `bytes.Compare(...) < 0`, the octet-by-octet comparison §2.8.1 names, and both call sites in `handleCreateChildSAOwned` (`inbound.go`) abandon our own exchange when ours is lower, given a pending rekey and a nonce in the request |
| The whole gate still passes | Gate run | `make ze-rfc-check` exit 0: selftest 784 tests OK, then 2965 gated MUST-level requirements across 171 enrolled RFCs, 3539 tags resolved |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The spec's metadata names no deferral shard (`Deferral shard: -`), and `plan/deferrals/fixit-rfc-row-level-and-anchor-drift.md` does not exist |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-rfc-row-level-and-anchor-drift-55b89662-ed70-484b-8728-361629e96dbc.md` |
| `review_gate.py check` | OK, clean, 4 code files, hashes match (exit 0) |
| Rounds | 5, and rounds 2 to 4 were each earned by a PRODUCT defect the previous round left in shipped text, always the same shape: a claim corrected at some sites and still standing at another. Round 2 found the Key Requirements table of `rfc/short/rfc7947.md` still calling MED a MUST. Round 3 found the `RFC7947-x-1` tag on `TestReactorForwardRSTransparent` still claiming Ze meets the no-prepend recommendation unconditionally, plus a false unqualified nonce claim in a Correction paragraph this change had itself added. Round 4 found the public `docs/features/rfc-status.md` row still stating transparency with no qualifier. Round 5 reported 0 BLOCKER and 0 ISSUE |
| Reviewer lenses used | Round 1, two parallel independent agents: RFC-text fidelity and evidence (every row re-read against `rfc/full/`), and gate integrity and test discrimination (each producer stubbed to prove the new tests fail when their mechanism is removed). Rounds 2 to 5, one independent agent each, scoped to the previous round's fixes, with a repo-wide convergence sweep in rounds 4 and 5 |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The two tests the Wiring Test table and the TDD plan name did not exist, so the claim that the ratchets permit the repair rested on prose alone | `scripts/dev/rfc_requirements_test.py` | Wrote `TestAnchorMismatchIsRefused` and `TestLevelChangeUnderStableIdIsAccepted`. Found at closure step 1, before round 1 |
| 2 | ISSUE | The misquote survived in the same file: the Key Requirements table still read "MED \| MUST be preserved across the RS", 53 lines above the Correction calling that reading wrong. The AS_PATH row beside it had been given the "recommendation rather than a requirement" gloss; the MED row had not | `rfc/short/rfc7947.md`, Key Requirements table | That row now states Section 2.2.3's SHOULD and its recommendation status, matching the AS_PATH row's treatment |
| 3 | ISSUE | "no configuration can turn that off" was false. `test/plugin/med-removal-configured.ci` configures `med-remove true` on an import-chain modify policy and asserts an `rs-client true` receiver gets attribute 4 removed. The same overstatement sat in `rfc/enrolled.txt` and in the `forward_med.go` comment | `rfc/short/rfc7947.md` Correction, `rfc/enrolled.txt` rfc7947 note, `medPropagationAllowedTo` doc comment | All three now say what is true: Ze's AUTOMATIC Section 5.1.4 strip never fires toward an RS client, while an operator's `med-remove` policy rewrites the payload upstream of the predicate, which is Section 5.1.4's own required removal mechanism |
| 4 | ISSUE | `RFC7296-3.3.6-1` had been given a second clause quoting §3.3.6's "missing a mandatory Transform Type" sentence, which its own tags do not prove: they sit on `TestResponderRequiresKEForDH`, driving a missing KE payload. That sentence is `RFC7296-3.3.6-4`'s, and it is proven there | `rfc/short/rfc7296.md`, row `RFC7296-3.3.6-1` and the Known Limitation note | Clause removed. The row rests on §3.3.3's own "A compliant implementation MUST understand all mandatory and optional types", and the note now points at the sibling that owns the enforcement obligation |
| 5 | ISSUE | `test_demoted_id_is_not_read_as_retired`, written at closure, asserts an ABSENCE and still passed with `check_retired_requirements` stubbed to return nothing. Proven by mutation in round 1 | `scripts/dev/rfc_requirements_test.py`, `TestLevelChangeUnderStableIdIsAccepted` | Added `test_the_same_id_actually_deleted_is_still_caught`: delete the row instead of correcting its level and the ratchet fires, so the silence is a verdict about that input rather than the function's only answer |
| 6 | NOTE | Two further sites carried the level this spec set out to correct: a `forward_med_test.go` comment calling `RFC7947-x-3` a gated MUST, and the Section 2.8 row of the `rfc/short/rfc7296.md` summary table rendering the SHOULD as fact | `TestForwardKeepsMEDForRouteServerClient`; `rfc/short/rfc7296.md` summary table | Both corrected. Same class as the `forward_med.go` comment the implementation had already fixed |
| 7 | NOTE | Three false statements in the closure record: the symbol `handleOwnedCreateChildSA` (real name `handleCreateChildSAOwned`), "word for word" over `RFC7947-x-3` (which moves the sentence's conditional opening into its subject), and an unqualified "the caller always abandons the lower-nonce exchange" | `plan/spec-fixit-rfc-row-level-and-anchor-drift.md` | All three corrected. Record defects, so per `ai/rules/planning.md` they earned no extra round |
| 8 | NOTE | The Architectural Verification table answered "No" to all five checks with no evidence, which reads as five violations rather than "does not apply" | `plan/spec-fixit-rfc-row-level-and-anchor-drift.md` | Answered N/A or Yes per row with the evidence that settles it |
| 9 | NOTE | A docstring said "222 gated rows" against a `GATED_FLOOR` of 221 | `test_rfc7296_every_gated_row_is_proven_in_both_polarities` | Corrected, with the reason for 221 written beside it |
| 10 | ISSUE (round 2) | The same over-claim survived at two sites round 1's fix did not reach: "Ze meets it unconditionally" in the `RFC7947-x-3` tag comment, and "keeps the metric unconditionally" in the spec's A-1 evidence. This is the identical failure round 1 found, one layer out: correcting a claim without sweeping every site that repeats it | `TestReactorForwardRSTransparent`; the spec's A-1 evidence | Both now name the automatic Section 5.1.4 strip and the operator's `med-remove` policy that still removes the metric upstream of it |
| 11 | ISSUE (round 2) | The A-1 nonce claim was corrected in Goal Validation but left unqualified in the A-1 evidence paragraph | The spec's A-1 evidence | Now states both gates on the abandonment, and that a request with no Ni deliberately never abandons an in-flight rekey |
| 12 | NOTE (round 2) | "Section 2.13 states no MUST about variable-sized keys" was over-broad: §2.13 carries four MUSTs, extracted as `RFC7296-2.13-1..4` | `rfc/short/rfc7296.md` Correction; `rfc/extraction/rfc7296.json` id `1.7:6` | Both now name §2.13's own MUSTs and say why none of them is this obligation |
| 13 | ISSUE (round 3) | The unqualified nonce claim survived at a third site, in a paragraph this change had ADDED: the `RFC7296-2.8-1` Correction said "the caller always abandons the exchange that carries the lower nonce", wrong about both the gating and the number of call sites | `rfc/short/rfc7296.md` Correction | The sentence is gone; the paragraph now quotes §2.8.1's own definition of "lowest" |
| 14 | ISSUE (round 3) | The sibling absolute claim one line above the round 2 fix was left standing: the `RFC7947-x-1` tag said Ze meets the no-prepend recommendation "unconditionally". `ExtractASPathPrependOps` emits copies of the local AS from an operator `as-path-prepend` policy, by the same upstream import route `med-remove` takes | `TestReactorForwardRSTransparent` | The tag now names the automatic eBGP prepend and the operator policy route |
| 15 | ISSUE (round 3) | Both new Correction paragraphs carried Ze implementation notes and Ze file paths inside `rfc/short/` summaries. `ai/rules/rfc-compliance.md` forbids it outright: a summary is a protocol-only reference a reader must be able to use with no knowledge of Ze | `rfc/short/rfc7296.md`, `rfc/short/rfc7947.md` | Every Ze path, symbol and behavior statement stripped from both new Corrections. The Ze-side facts stay where they belong: the code comments, `rfc/enrolled.txt`, and this spec |
| 16 | ISSUE (round 4) | The PUBLIC ledger still stated RS transparency unconditionally, the most visible instance of the claim rounds 1 to 3 had been chasing through the summaries and comments | `docs/features/rfc-status.md`, RFC 7947 row | Now says transparency holds by default, names Sections 2.2.2.1 and 2.2.3 as recommendations, and lists `as-path-prepend` and `med-remove` as the operator policies that change what an RS client receives |
| 17 | NOTE (round 4) | The IPsec guide rendered Section 2.8.1's SHOULD as fact, the same class corrected in the summary table | `docs/guide/ipsec.md`, Rekeying | States the recommendation and that Ze follows it. `make ze-doc-test` PASSED |
| 18 | NOTE (round 4) | The mis-anchor paragraph this change added said where the repo's tests sit, a statement about Ze inside a protocol-only summary | `rfc/short/rfc7296.md` | Rewritten to contrast the two obligations instead of the two test sets |
| 19 | NOTE (round 5) | Two more docs stated an RFC 7947 recommendation as a requirement, the same class this spec exists to correct: "RFC 7947 requires route-server transparency", against §2.2.2.1's "This is a recommendation rather than a requirement"; and "requires per-client import and export policy but does not mandate it", self-contradictory against §2.3.2.1's "the most portable method" | `docs/architecture/meta/filter-community.md`, `docs/guide/bgp-policy.md` | Both corrected. NOTEs do not block, but each is one clause and each is this spec's own defect class |
| 20 | NOTE (round 5) | `docs/guide/ipsec.md` said Ze "follows" §2.8.1's close-the-SA rule. Ze resolves the collision one step earlier, abandoning its pending exchange so the redundant SA is never created, which is the condition §2.8.1's clause opens on | `docs/guide/ipsec.md`, Rekeying | Refined to say what Ze actually does |

### Homed, not fixed here
| Severity | Finding | Why it is not fixed in this spec |
|----------|---------|----------------------------------|
| ISSUE | **There is no level ratchet anywhere.** None of the 23 `check_*` functions in `scripts/dev/rfc_requirements.py` reads `Requirement.level` against HEAD, and `GATED_FLOOR` is the only floor in the tree and covers `rfc7296` alone. So `RFC7947-x-3` moved MUST -> SHOULD with no gate firing at all, and any RFC without a floor can be demoted silently today. `test_rfc7296_ids_are_neither_retired_nor_demoted` is named for demotion and cannot see one: it builds its baseline from `r.gated` over the CURRENT rows, so a demoted id leaves the baseline instead of failing against it | Pre-existing, and the goal of this spec does not depend on it. Building the ratchet needs a recorded MUST-level subset per RFC and a place to record an authorised correction, which is an owner decision rather than an edit. Recorded in `plan/journal/reference-checked-claim-unchecked.md`, and the docstring of the blind test now states its own reach |
| NOTE | **`rfc/short/` still carries pre-existing Ze-specific content**, which `ai/rules/rfc-compliance.md` bars: `rfc/short/rfc7947.md` has an "Implementation Notes (Ze context)" heading, the 2026-08-14 correction naming `applyFactsMED` and its path, and an `RFC7947-x-2` annotation naming a file and line; `rfc/short/rfc7296.md` has six more sites | Each belongs to an earlier, closed spec. This change removed the violations it introduced and did not delete another spec's recorded corrections. Whether the rule or the practice is what should change is Thomas's call |
| NOTE | **A comment overstates transparency on the PLUGIN rail.** `internal/component/bgp/reactor/reactor_api_forward.go` says "an RS client's AS_PATH is never modified", but the export policy chain runs earlier in the same loop and can prepend the local AS for an RS client. The twin comment in `forward_rs.go` is accurate, because that rail skips peers carrying `ExportFilters` | Pre-existing, on a rail this change does not touch, and outside the file scope Thomas set for this commit |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/rfc_requirements_test.py` | Yes | Holds both new classes; `python3 -m unittest -v ...TestAnchorMismatchIsRefused ...TestLevelChangeUnderStableIdIsAccepted` ran 6 tests, OK |
| `rfc/short/rfc7296.md`, `rfc/short/rfc7947.md` | Yes | Both parsed by `make ze-rfc-check`, which reports 171 enrolled RFCs |
| `plan/deferrals/fixit-rfc-row-level-and-anchor-drift.md` | No | Correctly absent: the spec declares no deferral shard |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-2 | Both demoted rows render the RFC sentence faithfully | `rfc/full/rfc7296.txt` §2.8.1 and `rfc/full/rfc7947.txt` §2.2.3 read at closure. `RFC7296-2.8-1` is word for word. `RFC7947-x-3` moves the sentence's conditional opening ("if applied to an NLRI UPDATE sent to a route server, this attribute SHOULD ...") into its subject so the row reads standalone; both halves of the obligation are kept and no word of them is changed |
| AC-4 | The anchor could not be repaired by re-citing | `TestAnchorMismatchIsRefused.test_recited_without_renumber_is_refused` passes: `ParseError` says "disagrees with its section" and names `RFC7296-3.3.3-<n>` |
| AC-5 | All `RFC7947-x-1` tag sites state `SHOULD NOT` | `git grep -n "MUST NOT prepend own AS" -- internal/` returns nothing |
| AC-6 | The gate is green after every edit | `make ze-rfc-check` exit 0, log kept under this session's scratch directory |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-rfc-check` over the edited summaries | none (the `ze-rfc-check` target runs `rfc_requirements.py --selftest` then `--check`) | Yes: exit 0, both halves |
| A row whose level is edited under a stable id | none (unit) | Yes: `TestLevelChangeUnderStableIdIsAccepted` drives `check_coverage_ratchet` and `check_retired_requirements` directly, and its second case proves the ratchet still fires when proof is lost |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `medPropagationAllowedTo` (`forward_med.go`) and `localNonceIsLower` (`rekey.go`) read at closure, plus the two call sites in `inbound.go` that abandon the lower-nonce exchange. Both exceed their SHOULD |
| A-2 | confirmed | The RFC 7947 row of `docs/features/rfc-status.md` claims Ze behavior, not an RFC level; the RFC 7296 row says nothing about rekey collisions. Neither carries a spelled gap count that moves, and `check_gap_count_agreement` is green |
| A-3 | confirmed | `RFC7296-1.3.3-1` cites §1.3.3 while its sentence is in §1.3.2; `RFC7296-3.3.6-1` cites §3.3.6 while the D-H mandate is §3.3.3's table. Same shape, and now recorded under one rule above `rfc/short/rfc7296.md`'s Compliance Checklist |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC status page, RFC 7947 row (checklist row 2) | The row claimed RS transparency with no qualifier. `ExtractASPathPrependOps` and `test/plugin/med-removal-configured.ci` both show an operator policy overriding it, so the row now says "by default" and names the two policies. `make ze-doc-test` PASSED | Yes, edited |
| RFC status page, RFC 7296 row | Makes no claim about the rekey collision rule, so the demotion does not reach it. No spelled gap count moves; `check_gap_count_agreement` green | Yes, no edit |
| IPsec guide, Rekeying section | Rendered Section 2.8.1's SHOULD as fact; corrected against `rfc/full/rfc7296.txt` §2.8.1 | Yes, edited |
| Feature list, config syntax, wire format, architecture (rows 1, 3, 4: No) | `git grep -n "7947" -- docs/ ai/` and the source-anchor grep on `forward_med.go` / `forward_rs.go` return no further claim that a level change staled | Yes |

## Core Insight

A gate that reads only what was written down cannot tell a faithful transcription from a
confident one. `make ze-rfc-check` proved for months that `RFC7947-x-3` had a test for each
polarity, which was true, while the sentence it claimed to enforce said something the RFC
never said. Coverage and fidelity are different properties: the first is machine-checkable
from inside the repository, the second only from beside the source text. The floor that
guards the count is the same shape of instrument, which is why lowering it needed the row
and the RFC sentence written into its comment rather than a smaller number.
