# Spec: rfcgate-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/rfcgate-0-umbrella.md` |
| Updated | 2026-07-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Spec set `rfcgate`. Children, in merge order: `plan/spec-rfcgate-1-extraction.md`,
`plan/spec-rfcgate-2-evidence.md`, `plan/spec-rfcgate-3-audit-teeth.md`,
`plan/spec-rfcgate-4-ledger.md`. This umbrella owns the sequencing, the drain
schedule, and the scope boundary; it writes no gate logic of its own. "Owns the
drain schedule" means the POLICY (the rate, the start date, the semantics of the
quota) and the acceptance criterion over it; the floor COMPARISON that reads them
is implemented by `plan/spec-rfcgate-1-extraction.md` as `check_drain_floor`
(resolved 2026-07-29, see "Where the counter lives").

## Task

**The problem.** `make ze-rfc-check` is green, and its green is narrower than it
reads. The gate reports 2720 gated MUST-level requirements across 166 enrolled
RFCs, 2575 resolved test tags, and 0 outstanding
(`ai/RFC-REQUIREMENTS.md:5`, `:11`). It runs in both branches of `stagesForMode`
(`scripts/status/verify_run.go:237`, `:259`) and reaches CI through
`.github/workflows/verify.yml`, so it fires on every push. Nothing about that is
broken. What is broken is what the number is a number *of*.

**The symptom.** Six independent blind spots, each verified against the producing
code, let the gate report full coverage over a requirement set that is
self-selected, evidence that never leaves the Go test binary, verdicts nothing
reads, and public support claims nothing checks. The measured decomposition of
the green:

| Disposition of the 2720 gated MUST-level requirements | Count | Share |
|-------------------------------------------------------|-------|-------|
| Both polarities tested (positive and negative tags) | 974 | 35.8% |
| One polarity tested, annotated `{single-polarity}` | 370 | 13.6% |
| No test, annotated `{not-applicable}` | 841 | 30.9% |
| No test, annotated `{gap}` | 535 | 19.7% |
| **Satisfied by a written excuse rather than a test** | **1376** | **50.6%** |
| **Not proven in both polarities** | **1746** | **64.2%** |

Derived by parsing every enrolled summary with the gate's own
`parse_summary_file` and joining against `scan_tree` tag polarities; the four
rows sum to 2720 exactly.

**The multiplier.** Thomas voided every prior answer that pointed away from full
compliance or full proof on 2026-07-27 (`ai/rules/rfc-compliance.md:53`), and
named `{gap}` / `{not-applicable}` / `partial` in `rfc/short/*.md` and
`docs/features/rfc-status.md` as exactly where a void answer hides (`:58`). 1376
requirements currently ride on classifications that directive voids. The gate
counts every one of them as satisfied.

**The goal.** Build the machinery that makes the gate's green mean what a reader
takes it to mean: bounded extraction, evidence that runs, verdicts with teeth,
and a public ledger checked unconditionally. Machinery only. This set does not
drain the backlog it exposes (D4).

## Owner Decisions (Constraints)

Settled by Thomas: D1-D4 in the design session that produced this spec, D5-D8 in
the review of 2026-07-29. Children inherit them and may not relitigate them.

| ID | Decision |
|----|----------|
| D1 | SEPARATE PROGRAMS, EXTRACTION FIRST. Do not fuse extraction, annotation re-derivation and semantic audit into one per-RFC pass. Extraction is program one. |
| D2 | Gate strength = block NEW enrolments, ratchet the existing 166 as grandfathered visible backlog, publish the backlog count in `ai/RFC-REQUIREMENTS.md`, AND add a DRAIN SCHEDULE: the ratchet requires N RFCs recertified per release, so the backlog has a forcing function, not just a floor. |
| D3 | Interop = make the runner fail closed, add interop to nightly CI as advisory, extend the tag scanner, ratchet wire-evidence count, and PREFER `.ci` bindings over interop bindings because `.ci` actually runs inside `ze-verify` on every push. |
| D4 | Machinery now, then a large parallel fleet for the drain. This spec set is MACHINERY ONLY. The fleet drain is named as follow-on work, not scoped here. |
| D5 | DRAIN ANCHOR CONFIRMED AS DESIGNED. The repository has no releases, so the quota is calendar-anchored, ships at rate 0 so the machinery lands inert, and is armed by a one-line commit once the first fleet batch has measured real throughput. Thomas arms it. |
| D6 | REGISTER GRADING COUNTS TOWARD THE QUOTA. Grade each per-RFC sign-off by the register its source text is written in: `rfc2119`, `prose`, or `manual-walk`. A weaker register COUNTS toward the drain quota, because a third of the corpus is not written in RFC 2119 register at all and excluding it would make that third permanently undrainable. But publish each register in its OWN column, so a reader can never read "N signed off" as "N keyword-verified". |
| D7 | FOUND MEANS OWED. `rfc7296` is re-authored now, outside the grandfather. A CONFIRMED unextracted MUST escapes the grandfather by definition. |
| D8 | THE FOUR UNPROVEN-SUPPORT STEMS ARE EXTRACTED AND ENROLLED. `rfc1035`, `rfc3765`, `rfc4486` and `rfc5301` are published as Supported or Experimental over checklists with zero MUST-level rows. Extract and enrol all four, including fetching the two missing source texts and extracting `rfc1035`'s lowercase pre-2119 prose. |

→ Constraint (D1): child 1 lands alone and first. No child may bundle an
extraction change with an evidence, audit, or ledger change.
→ Constraint (D2): the drain forcing function is part of this umbrella's
design obligation, not a child's discretion. See "Drain Schedule Design".
→ Constraint (D3): when a requirement can be bound to either a `.ci` test or an
interop scenario, the `.ci` binding wins. Interop is additive evidence, never a
substitute, because `.ci` runs in `ze-verify` and interop does not.
→ Constraint (D4): no child may open the backlog. A child that finds itself
re-deriving an annotation has left its scope. D7 and D8 are the two named
exceptions, and both ADD obligations rather than lowering any.
→ Constraint (D5): the calendar anchor is settled, not a proposal. No child may
substitute a release hook, a tag count, or a commit count for it, and no child
ships a non-zero rate. A-7 is confirmed and R-2 is an accepted risk.
→ Constraint (D6): the per-RFC sign-off artifact carries a `register` field, and the
published ledger reports the three registers separately. A child that sums the
registers into one "signed off" total has broken the second half of D6, which is
the half that keeps the first half honest.
→ Constraint (D7): the general principle, which governs every future find and
not only `rfc7296`: an unextracted obligation is still an obligation
(`ai/rules/rfc-compliance.md:28`), and the session that confirms one is the entry
point that owes it (`ai/rules/no-parking.md`). Grandfathering covers what nobody
has looked at; it never covers what somebody has read and confirmed. Recording a
confirmed unextracted MUST as backlog is banned.
→ Constraint (D8): the four stems are extracted BEFORE they are enrolled, in that
order. Enrolling a zero-capture summary would change nothing the gate measures,
which is the reading blind spot 5 corrects.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md` - the owner directive, the four ratchets, and
  the Extraction Completeness clause this set mechanises.
  → Decision: conformance is not negotiable and only Thomas may authorise a
    deviation, in answer to the question the "Ask Thomas" clause requires (`:17`, `:38`).
  → Constraint: every prior answer pointing away from full compliance is VOID (`:53`);
    a `{gap}` or `{not-applicable}` is not authority and may not be cited as one (`:58`).
  → Constraint: summaries predating HEAD are deliberately grandfathered because
    "a rule that reds the gate on unrelated work gets removed rather than obeyed" (`:114-116`).
    The drain schedule must respect this or it will be deleted rather than drained.
- [ ] `ai/rules/fail-closed-guards.md` - every new check in this set is a guard.
  → Constraint: a guard must fail closed or say something; a zero value must never
    be a valid-looking answer. The gate already has the precedent at
    `scripts/dev/rfc_requirements.py:660-664` (an empty `rfc/enrolled.txt` is an
    error, not an empty backlog). Every child copies that shape.
- [ ] `ai/rules/derive-not-hardcode.md` - the backlog and its ranking are published,
  never authored.
  → Constraint: only the test-side `RFC requirement:` tag is hand-written; the
    ledger derives the reverse index. The extraction backlog is derived the
    same way, from the committed `rfc/extraction/` artifacts plus the live summaries.
    Only the drain POLICY (start date, rate) is authored (see "Where the counter lives").
- [ ] `ai/rules/testing.md` "Back-Fill New Test Types" - this set introduces new
  gate categories.
  → Constraint: the applicable set must be named and either back-filled or recorded
    as explicit tracked backlog, never left implicit. D2's published backlog IS that record.
- [ ] `ai/skills/ze-rfc-audit.md` - the audit verdict vocabulary child 3 makes load-bearing.
  → Constraint: the vocabulary is exactly `enforced`, `weak`, `wrong`, `unimplemented` (`:67-72`).
    Only `enforced` means proven.

### RFC Summaries (Scope: protocol)
Mostly not applicable at umbrella level: this set changes no protocol behaviour, and
children 2 and 3 read summaries as data only. ~~This set touches no `rfc/short/*.md`
content.~~ Superseded by D7 and D8, which put five summaries in scope for ADDITIONS:

- [ ] `rfc/short/rfc7296.md` (child 1, D7) - IKEv2. Captures 18 MUST-level rows
  against a source keyword count of 310.
  → Constraint: two obligations are CONFIRMED missing and must be captured:
    `rfc/full/rfc7296.txt:1397` (a retransmission MUST reuse the original Message ID)
    and `:1439` (at Message ID exhaustion the IKE SA MUST be closed or rekeyed).
  → Decision: additions only. No existing row's text or id changes, so the id
    contract and `check_retired_requirements` are untouched.
- [ ] `rfc/short/rfc1035.md`, `rfc/short/rfc3765.md`, `rfc/short/rfc4486.md`,
  `rfc/short/rfc5301.md` (child 4, D8) - each declares zero MUST-level rows while
  the product ledger publishes a support claim over it.
  → Constraint: extraction precedes enrolment. `rfc3765` and `rfc4486` have no source
    text at all and must be fetched first; `rfc1035` is lowercase pre-2119 prose and
    is extracted under the `prose` register (D6).
  → Constraint: a newly captured MUST is proven or escalated, never absorbed into a
    fresh `{gap}` (`ai/rules/rfc-compliance.md:53`, R-9).

**Key insights:** (minimal context to resume after compaction)
- The gate's green is bounded by what was extracted, and nothing bounds extraction.
- 1376 of 2720 requirements are satisfied by prose the owner has voided as authority.
- The audit file's `verdict` field has no reader anywhere in the repo.
- Interop exists (104 scenarios) and runs nowhere.
- D5-D8 are settled owner rulings, not options: calendar anchor confirmed and R-2
  accepted; weak registers count but publish separately; `rfc7296` is re-authored
  now because found means owed; the four unproven-support stems are extracted then
  enrolled by child 4.
- `check_unproven_support` must never arm before the four stems are resolved: the
  red blocks every commit in the repository, including its own revert, because
  `commit_helper.py create` refuses a script over a non-FRESH verify. ~~A
  structural red is unbypassable.~~ (Corrected 2026-07-29: `ze-rfc-check` is not
  a structural gate. See "Why 4c can never precede 4b" for the real mechanism,
  which reaches the same conclusion by a different route.)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/rfc_requirements.py` - the whole gate: summary parser
  (`parse_summary_file`), tag scanner (`scan_tree:540`), the four ratchets
  (`check_enrolment:655`, `check_coverage_ratchet:955`,
  `check_retired_requirements:1007`, `check_new_summaries:1059`), the public-ledger
  cross-check (`check_status_agreement:1163`), the audit freshness ratchet
  (`verdict_is_fresh:1227`, `load_audit:1239`), the source-keyword probe
  (`source_keyword_count:1323`), and the ledger renderer.
- [ ] `scripts/status/verify_run.go` - `ze-rfc-check` is a stage in both branches of
  `stagesForMode` (`:237`, `:259`); no interop target appears in either.
- [ ] `test/interop/run.py` - exits 0 with "Docker unavailable, skipping interop tests"
  when Docker is absent (`:133-134`).
- [ ] `test/ipsec-interop/run.py` - exits 1 in the same situation (`:119-120`), so the
  two runners disagree about whether a missing lab is a pass.
- [ ] `internal/component/bgp/message/header.go` - a representative producer of the
  behaviour these requirements govern: RFC 4271 Section 4.1 length and marker
  validation, with the MUST quoted inline above the enforcing branch (`:63-89`,
  `:103-148`). Read to confirm the gate is measuring real enforcing code, not a
  paper exercise.
- [ ] `rfc/enrolled.txt` - 166 entries, tab-separated stem plus rationale.
- [ ] `ai/RFC-REQUIREMENTS.md` - the generated ledger; headline counts at `:5` and `:11`.
- [ ] `docs/features/rfc-status.md` - the public product ledger; the rows at `:28`,
  `:56`, `:234` are three of the four verified zero-requirement-under-a-public-claim
  cases; the fourth is the RFC 5301 row, which carries `Experimental` rather than
  `Supported`. All four are D8's named set.
- [ ] `rfc/audit/rfc7606.json` - the only audit file: 52 entries, 49 `enforced`,
  2 `unimplemented`, 1 `implemented`, 3 with an empty `tests` map.

**Behavior to preserve:**
- The gate stays green on a tree that is genuinely compliant. No child may red the
  tree for pre-existing backlog: D2's grandfathering is what keeps the rule obeyed
  rather than deleted (`ai/rules/rfc-compliance.md:114-116`).
- All four existing ratchets keep their current semantics. Extraction, evidence,
  audit, and ledger checks are added beside them, never folded into them.
- The requirement id contract is unchanged: ids are allocated once and never
  renumbered; `check_retired_requirements` keeps blocking their disappearance. D7
  and D8 ADD rows, which allocates new ids and touches no existing one, so the
  contract holds. Once added, those ids are protected by the same ratchet.
- Only the test-side tag is authored. No child introduces a hand-written back-link
  from a requirement to a test.
- The three enrolled draft entries resolve their source text from `rfc/drafts/`, not
  `rfc/full/`. This is correct behaviour, not a gap: `check_enrolment:676` accepts
  either directory. Recorded because a naive `rfc/full/` scan reports them as
  missing and invites a fix for a defect that does not exist.

**Behavior to change:**
- Extraction gains an upper bound: captured MUST-level rows are compared against the
  source text, not only against themselves (child 1).
- Evidence gains file kinds beyond `_test.go` and `.ci`, and the interop runner stops
  reporting a skipped lab as a pass (child 2).
- The audit `verdict` field gains a reader, a schema, and a consequence (child 3).
- The public-ledger cross-check runs unconditionally rather than only when a `{gap}`
  exists, and a summary declaring zero requirements stops satisfying a public support
  claim at any level, `Supported` or `Experimental` (child 4, `check_unproven_support`).
- Four stems that publish a support claim over an empty checklist gain real
  checklists and enrol (child 4, D8), and `rfc/short/rfc7296.md` gains the two
  confirmed MUSTs it was missing (child 1, D7).

## The Six Blind Spots

Each row verified in this session against the producing code. Sizes are measured,
not estimated, except where the row says otherwise.

| # | Blind spot | Size (measured) | Producing `file:line` |
|---|-----------|-----------------|----------------------|
| 1 | **Extraction is unbounded.** The gate audits the requirements a summary LISTS, never what the RFC CONTAINS. `source_keyword_count` has exactly three consumers and none of them compares a non-zero captured count against the source count. | Raw deficit 2636 MUST-level keyword occurrences over captured rows; top 20 entries carry 66.5% of it. `rfc7296` captures 18 against a source count of 310; `rfc7950` captures 9 against 245. Estimated 1200-1500 distinct unextracted obligations after deflating restatements (A-1). **Two clarifications to an earlier reading of this row.** The denominator is all 166 enrolled entries, not 163: every enrolled entry HAS source text, 163 under `rfc/full/` and the 3 drafts under `rfc/drafts/`. 163 was a naive `rfc/full/`-only count, which is precisely the trap this spec warns about under "Behavior to preserve". And 2636 is the sum of per-entry POSITIVE deficits, not the difference of the totals, which is 2404; the two differ because only 127 of the 166 carry a positive deficit at all: 33 capture MORE rows than their source has keyword occurrences (blind spot 1a) and 6 capture exactly as many | `scripts/dev/rfc_requirements.py:1323` (definition); `:676` (presence-only, inside `check_enrolment`); `:1113` (fires only when captured == 0, inside `check_new_summaries`); `:1353` (rendering only) |
| 1a | **Pre-2119 sources make any keyword check vacuously green.** 22 enrolled entries have zero capitalised MUST / SHALL / REQUIRED anywhere in their source text, including `rfc2328` (OSPFv2) and `rfc792` (ICMP). A source-vs-captured comparison reads "no deficit" for every one of them. Widening the test from "zero keywords" to "fewer keywords than the summary captured" catches 11 more, giving **33 at the keyword-occurrence denominator**; widening it again to normative SITES after boilerplate exclusion gives **53**, which is the figure child 1's register rule operates on. All three are correct at their own denominator and none corrects another: see "The three pre-2119 measurements". | Two nested measurements, both reproduced by driving `parse_summary_file` and `source_keyword_count` over the enrolled set. Inner: 22 entries at exactly zero capitalised keywords (rfc1071, rfc1332, rfc1350, rfc1877, rfc2205, rfc2328, rfc2347, rfc2348, rfc2349, rfc2918, rfc2966, rfc3101, rfc3623, rfc5701, rfc6286, rfc7534, rfc7535, rfc792, rfc8050, rfc8571, rfc905, sflow-v5). Outer: 33 entries whose captured MUST-level rows EXCEED their source keyword count, the 22 plus 11 token users, of which `rfc2181` is the sharpest (23 captured MUST rows against exactly 1 uppercase keyword occurrence). `rfc2328` sits at 25 captured against 0 | `scripts/dev/rfc_requirements.py:1323` returns a count that is legitimately 0; the fail-open is in any consumer that treats 0 as "nothing owed" |
| 2 | **Evidence is in-process.** The tag scanner recognises exactly two file extensions, so a requirement can only ever be proven by something that runs inside a Go test binary or the `.ci` runner. The git-baseline scanner repeats the same filter independently, so the two must be changed together or the ratchet baseline desynchronises from the live scan. | 2575 tag references: 2571 on `*_test.go`, 4 on `.ci` (in exactly two files, `test/plugin/rfc7606-reset.ci` and `test/plugin/rfc7606-withdraw.ci`), 0 on any interop artefact | `scripts/dev/rfc_requirements.py:558-563` (`scan_tree`); `:835` (mirrored filter inside `_git_baseline_tag_polarities:797`) |
| 3 | **Interop runs nowhere.** 104 scenarios exist and no automation executes them. The BGP runner treats a missing Docker as success; the IPsec runner treats the same condition as failure. Neither is reachable from `ze-verify` or any workflow. | 104 scenario directories under `test/interop/scenarios/`, plus the ipsec, l2tp and pppoe trees; 0 of 6 files in `.github/workflows/` mention interop | `scripts/status/verify_run.go:232-289` (no interop stage in either `stagesForMode` branch); `test/interop/run.py:133-134` (prints "Docker unavailable, skipping interop tests" then exits 0); `test/ipsec-interop/run.py:119-120` (exits 1) |
| 4 | **Audit is 4.5% and toothless.** One audit file exists. Freshness compares only the requirement text sha and the per-test sha map, so a verdict recorded `weak` or `wrong` is treated exactly like `enforced`. No code anywhere reads the `verdict` field. `load_audit` validates no schema, and the vocabulary has already drifted. 3 entries carry an empty `tests` map, which makes them permanently fresh and therefore unfalsifiable. | 52 entries in 1 file (`rfc/audit/rfc7606.json`); 44 of the 974 both-polarity-proven requirements carry a verdict (4.5%); vocabulary observed: 49 `enforced`, 2 `unimplemented`, 1 `implemented` (not one of the four defined values); 3 entries with empty `tests` | `scripts/dev/rfc_requirements.py:1234-1236` (`verdict_is_fresh` compares `requirement_sha` and `tests` only); `:1239` (`load_audit`, no schema validation); a repo-wide search for a read of `verdict["verdict"]` returns nothing; vocabulary defined at `ai/skills/ze-rfc-audit.md:67-72` |
| 5 | **Empty summaries sit under public support claims.** A summary declaring zero requirements at any level satisfies the gate trivially while the public ledger publishes a support claim with no tracked gap. `check_status_agreement` reaches for a product-ledger row only when a `{gap}` annotation exists, so a summary with no requirements is never compared to anything. | 9 summaries declare zero MUST-level rows: rfc1035, rfc3765, rfc4486, rfc5301, rfc6987, rfc7999, rfc8195, rfc8326, rfc9129. **Four of the nine carry a public support claim** and are D8's named set: rfc1035, rfc3765 and rfc4486 are `Supported` with "No tracked gap in current source anchors", and rfc5301 is `Experimental` with "Same IS-IS experimental status.". The other five have no ledger row at all, so they publish no claim. All nine are currently unenrolled | `scripts/dev/rfc_requirements.py:1163` (`check_status_agreement` iterates requirements and `continue`s unless `ann.kind == "gap"`); `docs/features/rfc-status.md:234` (RFC 1035), `:28` (RFC 4486), `:56` (RFC 3765), plus the RFC 5301 row |
| 6 | **Ledger edges are safe by luck.** 32 enrolled entries have no row in the public ledger. The `row is None` branch turns the gate red the moment any one of them acquires a `{gap}`. Nothing today warns that this cliff exists; the tree is green only because none of those 32 has yet needed a gap disclosure. | 32 enrolled entries with no `docs/features/rfc-status.md` row, **all of them `rfcNNNN` stems**: rfc1071, rfc2003, rfc2348, rfc2349, rfc2473, rfc2782, rfc2784, rfc2890, rfc3031, rfc3786, rfc4213, rfc4576, rfc4862, rfc5561, rfc6071, rfc6138, rfc6397, rfc6482, rfc6996, rfc7012, rfc7427, rfc7440, rfc7611, rfc792, rfc7950, rfc8097, rfc8571, rfc8707, rfc905, rfc9319, rfc9728, rfc6549. Separately, the 9 unenrolled summaries of blind spot 5 carry no declared reason for being unenrolled | `scripts/dev/rfc_requirements.py:1178-1184` (`row is None` appends a hard error) |

**Correction to an earlier reading (blind spot 5).** The 9 unenrolled summaries are
not an enrolment problem *first*. Every one of them declares zero MUST-level
requirements, so enrolling them today would change nothing the gate measures.
Their real defect is under-capture, which is blind spot 1. ~~Child 4 owns the
disclosure edge; child 1 owns the capture.~~ Superseded by D8: extraction and
enrolment of the four claim-carrying stems are one indivisible act and both belong
to child 4, because a stem must clear child 1's extraction bar at the moment it
enrols, and child 1's bar does not exist until child 1 has landed. The ordering
inside child 4 is extract, then enrol, never the reverse. The remaining five stems
(rfc6987, rfc7999, rfc8195, rfc8326, rfc9129) publish no claim and stay out of
scope, owing only the declared reason for being unenrolled.

**Correction to an earlier reading (blind spot 6).** An earlier count reported 36
enrolled entries with no product-ledger row, decomposed as 32 `rfcNNNN` stems plus
3 IETF drafts and `sflow-v5`. That decomposition was wrong and the figure is 32.
Re-measured by driving `load_enrolled` and `parse_status_ledger`: 166 enrolled
against 157 ledger rows, 32 enrolled stems unmatched, every one of the form
`rfcNNNN`. The 3 drafts and `sflow-v5` each DO have a row, because
`parse_status_ledger` keys a draft row by its full draft stem and a non-RFC row by
its lowercase hyphenated stem (`scripts/dev/rfc_requirements.py:1140-1152`), both
branches added precisely so a `{gap}` on such a summary can find its disclosure
row. A naive scan for `^RFC \d+` misses those two branches and manufactures four
phantom cliff entries. Recorded because the phantom set is the one a future reader
would otherwise try to fix.

### The three pre-2119 measurements (canonical; stated once, here)

Three figures for "how much of the corpus is not written in RFC 2119 register"
circulate across this spec set. **All three are correct. None corrects another.**
They differ because each counts a different thing on the source side, and a reader
who meets one of them alone will mis-size the problem. This table is the single
statement; every other mention in this set and in the children references it
rather than re-deriving it.

| Figure | Denominator on the source side | What it means | Where it binds |
|--------|-------------------------------|---------------|----------------|
| **22 of 166** | uppercase MUST / SHALL / REQUIRED keyword OCCURRENCES, anywhere in the source | Zero such keywords exist in the source text at all, yet these 22 declare **164 gated MUSTs** between them. Includes `rfc2328` (OSPFv2), `rfc792` (ICMP), `rfc905`, `rfc1350` (TFTP) and `sflow-v5` | The vacuous-green case. Any keyword-driven arithmetic reports "no deficit" for every one of them. This is what makes the register model necessary rather than merely tidy |
| **33 of 166** | the same uppercase keyword OCCURRENCE count, compared against captured rows | These declare MORE gated MUST rows than their source has uppercase MUST-level keyword occurrences. The 22 above plus 11 token users. `rfc2181` is the sharpest: 23 captured gated rows against exactly ONE occurrence, in a memo whose own Section 3 says it does not use the 2119 expressions | The "a correct summary can legitimately out-declare its keyword count" case. It is why a captured-over-keywords RATIO can never be a gate |
| **53 of 166** | normative keyword SITES, after RFC 2119 boilerplate exclusion | These declare more gated requirements than the site scan finds sites for. This is `plan/spec-rfcgate-1-extraction.md`'s measure (its A-2), and it is the one child 1's register-derivation rule actually applies | The operative figure for grading. 53 derive `prose` or `manual-walk`; the other 113 can take `rfc2119` |

**The point all three support is the same, and it is D6, already decided.** A
weaker register COUNTS toward the drain quota, and is published in its own column.
Whichever figure a reader arrives with, excluding the weak registers strands a
large minority of the corpus with no route out of the backlog, and merging the
columns publishes a number that means less than it reads.

→ Constraint: a child that needs one of these figures cites this table and names
the denominator it is using in the same sentence. Quoting "33" or "53" bare, with
no denominator, is what made the three look like a disagreement in the first place.
→ Constraint: the OPERATIVE figure for anything about grading, register
derivation, or which entries can take the `rfc2119` bound is **53**, because that
is the denominator child 1's rule uses. Use 22 for the vacuous-green argument and
33 for the ratio-is-not-a-gate argument, and never interchangeably.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A developer runs `make ze-rfc-check`, or pushes and CI runs `make ze-verify`,
  which invokes the same target from `stagesForMode`
  (`scripts/status/verify_run.go:237`, `:259`).
- Input at entry: the working tree. Four artefact families are read as data:
  `rfc/short/*.md` (requirement checklists), `rfc/full/*.txt` and `rfc/drafts/*.txt`
  (source text), `rfc/enrolled.txt` (the gated set), and every `*_test.go` and `*.ci`
  file in the tree (the `RFC requirement:` tags). `docs/features/rfc-status.md` and
  `rfc/audit/*.json` are read as cross-check inputs.

### Transformation Path
1. `parse_summary_file` turns each `rfc/short/<stem>.md` checklist table into
   requirement records carrying id, level, section, and any annotation.
2. `scan_tree` walks the tree and collects `RFC requirement: <id> <polarity>` tags,
   accepting only `*_test.go` and `*.ci` files.
3. The two are joined by requirement id to produce, per requirement, a polarity set
   and an optional annotation. Both polarities present means proven; one means
   `{single-polarity}`; none means the annotation is the answer.
4. The four ratchets compare the joined result against a git-HEAD baseline, so a
   downgrade reds even when the working tree alone looks acceptable.
5. `check_status_agreement` cross-checks `{gap}` annotations against
   `docs/features/rfc-status.md`, and `verdict_is_fresh` cross-checks recorded audit
   verdicts against the current requirement and test shas.
6. `make ze-rfc-index` renders the joined result into `ai/RFC-REQUIREMENTS.md`, whose
   staleness is itself gated by `ze-doc-test`.

**Where this set inserts.** Child 1 adds a step between 1 and 3 that bounds step 1's
output against the source text. Child 2 widens step 2's admissible inputs and makes
the interop runner a real producer rather than a silent skip. Child 3 gives step 5's
verdict a consequence. Child 4 makes step 5's ledger cross-check unconditional.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Summary ⇄ RFC source text | captured MUST-level rows vs `source_keyword_count` over `rfc/full/` or `rfc/drafts/` | No (this is the boundary child 1 creates; today nothing compares them) |
| Summary ⇄ Test | requirement id in `rfc/short/<stem>.md` ↔ `RFC requirement:` tag | Yes, `make ze-rfc-check` today |
| Test corpus ⇄ Runner | tag file extension decides whether the evidence is admissible | Yes, and the answer today is `_test.go` and `.ci` only (`:558-563`) |
| Interop lab ⇄ CI | no path exists; the runner exits 0 on a missing lab | Yes, verified absent from every workflow and from `stagesForMode` |
| Requirement ledger ⇄ Product ledger | `{gap}` annotation ↔ `docs/features/rfc-status.md` row | Partly: only `{gap}` triggers the reach, so 9 zero-requirement summaries are never compared |
| Audit verdict ⇄ Gate result | `verdict_is_fresh` reads shas; the verdict word itself has no reader | Yes, verified: no consumer of the `verdict` field exists |
| Sign-off artifact set ⇄ Backlog | committed `rfc/extraction/<stem>.json` files subtract from the grandfathered set; the quota reads the derived count through ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Command correction" under "Where the counter lives") | No (created by child 1, per D2) |
| Sign-off artifact ⇄ Source register | the artifact's `register`, DERIVED from the source text and refused when it claims a stronger grade than the derivation supports, grades what that text can be checked against | No (created by child 1, per D6). Measured today: 53 of 166 enrolled entries cannot take the `rfc2119` grade under child 1's derivation rule. That is the site-denominator figure; see "The three pre-2119 measurements" for why 22, 33 and 53 are all correct and all different |
| Public support claim ⇄ Captured requirement count | a `Supported` or `Experimental` row must be backed by at least one captured MUST-level row | No (created by child 4 as `check_unproven_support`). Red on exactly four stems today, which is why its arming order is constrained |

### Integration Points
- `scripts/dev/rfc_requirements.py` - every child modifies this one module. See
  "Sequencing Constraint".
- `scripts/status/verify_run.go` - `stagesForMode` gains nothing from this set; the
  existing `ze-rfc-check` stage carries the new checks.
- `.github/workflows/` - child 2 adds interop to a nightly workflow as advisory,
  alongside the existing `evidence-nightly.yml` and `qemu-nightly.yml` precedents.
- `ai/RFC-REQUIREMENTS.md` - gains the derived extraction backlog section, with
  the three register columns rendered separately (D6).
- `rfc/extraction/<stem>.json` - the per-RFC sign-off artifacts created by child 1.
  This set IS the record the drain quota is derived from (see "Where the counter
  lives"); nothing about the quota is hand-maintained.
- `rfc/drain-budget.txt` - the drain POLICY only, two fields (start date, rate),
  created by child 1.
- `rfc/short/` - read as data by every child, and WRITTEN by exactly two, both adding
  rows only: child 1 for `rfc7296` (D7) and child 4 for the four D8 stems.
- `rfc/enrolled.txt` - read by every child; child 4 appends the four D8 stems after
  their summaries are extracted.
- `rfc/full/` - read by `source_keyword_count`; child 4 adds the two missing D8 source
  texts so that probe stops returning None for them.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every new check enters through the existing `ze-rfc-check` target and the existing `_collect_for_check` dispatch; no child adds a second entry point to the gate |
| No unintended coupling (components stay isolated) | Yes, with one bounded exception | The set touches gate tooling under `scripts/`, `rfc/`, `docs/`, `ai/`, and CI config. No daemon BEHAVIOUR moves. The exception is D7: proving the two `rfc7296` MUSTs requires tagged `_test.go` files, which live under `internal/`. That is test code against existing behaviour and changes no production path. If a MUST turns out to be unimplemented, that is the escalation in Failure Routing, and the behaviour change belongs to a separate compliance spec, not to a child of this set |
| No duplicated functionality (extends existing, does not recreate) | Yes | Child 1 reuses `source_keyword_count` rather than writing a second source probe; child 2 widens `scan_tree` rather than adding a parallel scanner; child 3 extends `load_audit`/`verdict_is_fresh`; child 4 extends `check_status_agreement`. Nothing is reimplemented |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Developer tooling in Python; no wire path, no pooled buffer, no hot path |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | Yes | The backlog, its ranking, and the recertification set are all derived from committed data files and rendered, never enumerated in source. No child hardcodes an RFC stem list anywhere |

## Child Specs

| # | File | Scope | What it gates | Depends on |
|---|------|-------|---------------|-----------|
| 1 | `plan/spec-rfcgate-1-extraction.md` | Bound extraction against source text. Compare captured MUST-level rows to `source_keyword_count` for every enrolled entry. Block a NEW enrolment whose capture ratio fails the bar. Grandfather the existing 166 into a visible, published backlog. Create the per-RFC sign-off artifact `rfc/extraction/<stem>.json` carrying the DERIVED `register` (D6), the derived status envelope ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Command correction" under "Where the counter lives") the quota consumes, and `rfc/drain-budget.txt` holding the drain policy's two fields. **Plus the drain floor COMPARISON itself, `check_drain_floor`, which reads this umbrella's policy against child 1's derived signed count and caps the floor at the backlog size (resolved 2026-07-29: this umbrella keeps the policy and AC-13 and writes no code; the child implements the comparison, because the floor is a pure function of the sign-off set the child already derives).** ~~Create `rfc/recertified.txt` with the `register` column.~~ (Resolved 2026-07-29; see "Where the counter lives".) **Plus, per D7: re-author `rfc/short/rfc7296.md` to capture the two confirmed unextracted MUSTs, and prove them or escalate.** | A new enrolment cannot enter `rfc/enrolled.txt` without a valid sign-off artifact. An existing entry's captured count cannot fall. The published backlog count cannot silently shrink without a sign-off. The published sign-off count is broken out by register and never summed into one figure. `rfc7296` carries both confirmed MUSTs. | - |
| 2 | `plan/spec-rfcgate-2-evidence.md` | Widen what counts as proof. Extend `scan_tree` and `_git_baseline_tag_polarities` together to admit interop scenario evidence. Make `test/interop/run.py` fail closed on a missing lab, matching `test/ipsec-interop/run.py`. Add interop to a nightly workflow as advisory. Ratchet the wire-evidence tag count so it cannot fall. | A requirement bound only to an interop scenario is admissible but marked as non-`ze-verify` evidence. The wire-evidence count is monotonic. A missing Docker lab is a failure, never a pass. | 1 |
| 3 | `plan/spec-rfcgate-3-audit-teeth.md` | Give the audit verdict a reader and a consequence. Schema-validate `rfc/audit/*.json` on load. Reject an entry with an empty `tests` map. Constrain the verdict to the four defined values. Make a `weak`, `wrong`, or `unimplemented` verdict stop counting the requirement as proven. | A drifted verdict word is a parse error, not a silent pass. An unfalsifiable entry is rejected. A requirement whose only audit verdict is `weak` or `wrong` is no longer reported as proven. | 1, 2 |
| 4 | `plan/spec-rfcgate-4-ledger.md` | Close the public-claim edges. Run the product-ledger cross-check unconditionally rather than only on `{gap}`. Introduce `check_unproven_support`, which rejects a public support claim backed by a summary declaring zero requirements. Resolve the 32 enrolled entries with no ledger row before the `row is None` cliff fires. Require a declared reason for an unenrolled summary. **Plus, per D8: extract and enrol the four unproven-support stems, fetching `rfc/full/rfc3765.txt` and `rfc/full/rfc4486.txt` first.** | A public support claim must be backed by at least one captured requirement. Every enrolled entry has a ledger row. An unenrolled summary carries a written reason. The four stems are extracted, enrolled, and no longer the set `check_unproven_support` would red on. | 1, 2, 3 |

## Sequencing Constraint

**All four children modify `scripts/dev/rfc_requirements.py`. They must be
implemented and merged strictly serially, in the order 1 → 2 → 3 → 4. Two
children must never be in flight at the same time.**

Two independent reasons, and the second is the one that would not be caught by a
merge tool:

1. **Textual.** Every child adds a check into the same module and the same
   `_collect_for_check` / main dispatch. Concurrent agents produce overlapping
   edits in the same functions of a single roughly 1700-line file. This is the
   ordinary conflict, and it is the less dangerous one.
2. **Semantic, and silent.** Every ratchet in this gate compares the working tree
   against git HEAD (`_git_baseline_tag_polarities:797`, `check_enrolment`'s
   baseline, `check_coverage_ratchet:955`, `check_retired_requirements:1007`,
   `check_new_summaries:1059`). A half-merged sibling changes the *baseline* the
   other child's ratchet reads. Two children landing in overlapping windows would
   each calibrate a floor against a tree the other is mutating, and the resulting
   floor would be wrong in a way that still shows green. A conflict marker is
   loud; a mis-calibrated ratchet is not.

**Why this order, and not another:**

| Position | Child | Why it must come here |
|----------|-------|----------------------|
| 1st | 1-extraction | It is the only child that changes what the gate COUNTS. Every other child's thresholds, floors, and percentages are denominated in the requirement set. If extraction lands last, children 2, 3 and 4 commit ratchet baselines against a requirement set that then grows by an estimated 1200-1500, and every one of those floors becomes silently too low. D1 independently mandates extraction as program one. |
| 2nd | 2-evidence | It changes what COUNTS AS PROOF. It must follow extraction because its wire-evidence floor is a count over the requirement set. It must precede audit because an audit verdict is a judgement *about a test*, and this child changes which tests are admissible; auditing first would audit a corpus that then changes shape. |
| 3rd | 3-audit-teeth | It makes the verdict load-bearing. It needs child 1's requirement ids to key verdicts against, and child 2's final scanner to define the set of tests a verdict may legitimately cite. Landing it earlier would harden a verdict schema around an evidence model that then widens. |
| 4th | 4-ledger | It reconciles what the other three produce. The disclosure obligation is a function of the final requirement set (child 1), the final evidence rules (child 2), and the final verdict semantics (child 3). Reconciling the public ledger first would reconcile it against numbers that then change three times, and each change would re-open the `row is None` cliff this child exists to close. **And, load-bearing on its own: child 4 ENROLS four RFCs, and child 1 is what makes an enrolment carry an extraction bar. See "Why child 4 must follow child 1" below.** |

**Why child 4 must follow child 1: the four D8 enrolments would otherwise be
grandfathered out of the extraction bar permanently.**

Stated separately and named, because the two reasons above (module conflict and
ratchet-baseline calibration) would both be satisfied by several orderings that
get this one wrong. This reason admits exactly one:

| Step | What happens |
|------|--------------|
| Child 1 lands | `check_enrolment` gains a precondition: a stem enrolled since HEAD must carry a valid `rfc/extraction/<stem>.json` sign-off (`plan/spec-rfcgate-1-extraction.md` AC-1). Grandfathering is SCOPE, not an allowlist -- the bar applies to what enrols AFTER the bar exists |
| Child 4 lands after it | Its four D8 enrolments (rfc1035, rfc3765, rfc4486, rfc5301) are new-since-HEAD, so each must carry a sign-off. Four extractions, judged by the machinery, at the moment they enrol |
| Child 4 landing BEFORE it | The four enrol while no bar exists. They are then enrolled at HEAD when child 1 arrives, so child 1's grandfathering covers them and **they never face the extraction bar at all.** Not "later": never. The bar has no retroactive branch, by design |

The four stems whose unproven public support claims MOTIVATED this whole
programme would be the four that permanently escaped its central check. That is
not a small loss of rigour; it is the programme's own subject slipping out of its
own net, silently and with a green gate.

→ Constraint: the order 1 before 4 is BINDING for this reason alone, independently
of module conflicts and ratchet baselines. A future reader who reshuffles the
children for an otherwise good reason (a merge window, a blocked child, a smaller
diff) must preserve 1-before-4 or explicitly re-plan how the four D8 stems clear
child 1's extraction bar. There is no third option in which they simply enrol.
→ Constraint: this is not a licence to enrol them early "and sign off after".
`plan/spec-rfcgate-1-extraction.md` AC-1 gates enrolment on the sign-off existing,
and D8's own 4a-to-4b ordering already puts extraction before enrolment for the
independent reason that enrolling a zero-capture summary changes nothing the gate
measures.

**Ordered constraint inside child 4: resolve the four stems BEFORE arming
`check_unproven_support`.**

`spec-rfcgate-4-ledger` introduces `check_unproven_support`, and on today's tree
that check is red on exactly four stems: `rfc1035`, `rfc3765`, `rfc4486` and
`rfc5301` (D8). The constraint is:

| Order | Step | Must not be reordered because |
|-------|------|-------------------------------|
| 4a | Fetch `rfc/full/rfc3765.txt` and `rfc/full/rfc4486.txt` | Two of the four stems have no source text in `rfc/full/` or `rfc/drafts/`, so `source_keyword_count` returns None for them and extraction cannot be bounded until the text is present |
| 4b | Extract the four summaries, produce a sign-off artifact for each, and enrol them | Enrolment of a zero-capture summary changes nothing the gate measures. And because child 1 has landed, each of the four is a new-since-HEAD enrolment and `check_enrolment` refuses it without a valid `rfc/extraction/<stem>.json` (`plan/spec-rfcgate-1-extraction.md` AC-1). Four enrolments therefore means four extraction sign-offs, not three or none. See "Why child 4 must follow child 1" |
| 4c | Arm `check_unproven_support` | See the blocking reason below |
| 4d | Resolve the 32 missing ledger rows, then make `check_status_agreement` unconditional | R-6: the `row is None` cliff must be scheduled, not discovered |

**Why 4c can never precede 4b, stated because it is nowhere else in this spec
set.** `check_unproven_support` runs inside `ze-rfc-check`, which is a stage in
both branches of `stagesForMode` (`scripts/status/verify_run.go:237`, `:259`) and
therefore part of `make ze-verify`. An armed-but-red gate there leaves every
`make ze-verify` in the repository non-green, and `scripts/dev/commit_helper.py
create` refuses to prepare a commit script over a non-FRESH verify
(`ai/rules/git-safety.md`, Step 1). So arming this check while the four stems are
still unresolved would not merely red this spec set: it would block **every
commit in the repository**, including the commit that fixes it, for every
concurrent session. The check MUST arm in the same commit that resolves the four
stems, or in a later one. Never before.

~~`ai/rules/git-safety.md` makes a structural gate red **unbypassable**: it is
explicitly not eligible for the `plan/known-failures/` path, and the only escape
is an owner-only `--structural-red-ok` override.~~ **Corrected 2026-07-29.** That
argument was wrong on its facts and is withdrawn; the constraint it supported is
unchanged and now rests on the paragraph above plus this one. `STRUCTURAL_GATES`
(`scripts/dev/commit_helper.py:512-523`) is a frozenset of exactly eight stage
names -- `ze-lint`, `ze-lint-changed`, `ze-tier-check`,
`ze-iface-resolution-check`, `ze-plugin-boundary-check`,
`ze-regen-check-readonly`, `ze-verify-wiring-docs`, `ze-vet-evidence` -- and
`ze-rfc-check` is NOT among them. The red is therefore technically
`--unverified`-bypassable. That is not an escape, for two reasons that survive
the correction:

| The bypass | Why it is not a route |
|------------|----------------------|
| `--unverified "<reason>"` | Legitimate only for a flaky or environmental red, or one already logged in `plan/known-failures/`. A gate red on four NAMED stems is maximally deterministic and reproduces on every run |
| A `plan/known-failures/` shard | `ai/rules/fix-dont-record.md` bans a shard for anything deterministic and reproducible. Recording this red instead of fixing it is the move that rule exists to stop |
| The owner override | Then every commit in the repository, by every concurrent session, needs an explicit Thomas ruling until the four are cleared. That is the cost the arming order exists to avoid, not a mitigation of it |

The only honest route past an early-armed red is an explicit owner ruling on
every commit until the four stems are cleared. Arming last costs nothing and
removes the question entirely.

The same reasoning is why 4d follows 4c rather than leading: both tighten the same
file, and a child that arms two reds in one commit has no green intermediate
state to bisect from.

**Operational rule for the implementing sessions.** Each child lands as its own
reviewed unit, green through `make ze-verify`, and is committed before the next
child's implementation session begins. The umbrella is the only spec allowed to be
open across the whole set.

## Drain Schedule Design (D2)

D2 requires a forcing function, not just a floor. Designing it is this umbrella's
job. Three questions must be answered mechanically: where the counter lives, what
"per release" means in this repository, and how it fails closed.

### Where the counter lives

**Resolved 2026-07-29. This settles the conflict `plan/spec-rfcgate-1-extraction.md`
raised as its R-14 and correctly left to this umbrella to decide.**

~~`rfc/recertified.txt`, a committed, append-only ledger created by child 1. One row
per entry that has been walked against its source text section by section, per the
Extraction Completeness clause (`ai/rules/rfc-compliance.md:73-98`).~~

~~| Column | Content | Why it is needed |~~
~~| stem | The `rfc/enrolled.txt` stem | Joins the row to the summary and the source |~~
~~| date | `YYYY-MM-DD` of the recertification | Feeds the budget computation |~~
~~| captured | MUST-level rows captured at recertification time | Detects a later deletion |~~
~~| source | `source_keyword_count` at recertification time | Records the denominator |~~
~~| register | The register the SOURCE TEXT is written in | D6 |~~

**The record is the per-RFC sign-off artifact set, `rfc/extraction/<stem>.json`,
which child 1 already builds. The drain quota is DERIVED from that set and is
never hand-maintained.** A second hand-kept list of who has been signed off is
precisely the rotting registry `ai/rules/derive-not-hardcode.md` forbids, and
mass-generating entries into it is the declare-instead-of-prove failure the
2026-07-20 owner ruling in `plan/deferrals/rfc-gate-regression-ratchets.md`
already refused. Child 1's artifact is also strictly stronger than the struck row
above: it is per-site classified, its skeleton writer can only emit UNCLASSIFIED
entries, and an unclassified site fails the check, so generating artifacts en
masse makes the gate redder rather than greener.

Every column the struck ledger row would have carried already exists in the
artifact, derived rather than typed:

| What the ledger row would have held | Where it lives now | Authored or derived |
|-------------------------------------|--------------------|---------------------|
| stem | the artifact filename and its `stem` field, cross-checked against it | derived, cross-checked |
| date | `signed-off` (plus `reviewer`, which the row never had) | authored |
| captured / source (the staleness pair) | `source-sha` over the normalized source text, which fires on ANY source change rather than only on a count change | derived |
| register | `register`, DERIVED from the source text and refused when the artifact claims a stronger grade than the derivation supports (D6) | derived, cross-checked |

The backlog is derived, never authored: it is the enrolled set minus the set of
stems carrying a valid sign-off artifact. `ai/RFC-REQUIREMENTS.md` publishes the
backlog count, the signed count broken out per register, and the per-entry deficit
ranking (`ai/rules/derive-not-hardcode.md`). Child 1 emits the same counts
machine-readably through ~~`make ze-rfc-extraction-status --json`~~
`make ze-rfc-extraction-status`, and that envelope
is the interface the quota's floor comparison consumes. The comparison itself is
child 1's (`check_drain_floor`); this umbrella owns the policy it reads and the
acceptance criterion over its result. See "Who implements the floor" below.

**Command correction, 2026-07-29 (found by implementing child 1).** Every
occurrence of `make ze-rfc-extraction-status --json` in this umbrella is struck
and replaced by `make ze-rfc-extraction-status`. GNU make parses `--json` as one
of its OWN options and exits 2 with `unrecognized option '--json'` before any
recipe runs, so the documented spelling was unrunnable at every site (verified
2026-07-29). The target always emits the JSON envelope -- that envelope is the
mode's only consumer, so a second human-readable shape would be a mode to keep in
step for nothing -- and `scripts/dev/rfc_requirements.py` still accepts a
`--json` flag and ignores it, so the spelling works at the SCRIPT level and only
the `make` wrapper rejects it. Recorded in full at
`plan/spec-rfcgate-1-extraction.md`, "Two spec corrections found by implementing
it".

### What is still authored: the policy input, and only that

Deriving the record leaves exactly two facts that no artifact can supply, because
they are POLICY rather than data: **when the drain clock starts, and how fast it
runs.** They live in `rfc/drain-budget.txt`, a committed file with those two
fields and nothing else.

| Field | Content | Why it cannot be derived |
|-------|---------|--------------------------|
| `start` | `YYYY-MM-DD` the budget begins counting from | A choice about when the obligation attaches. No artifact records it |
| `rate` | Entries per calendar month. Ships at `0` per D5 | A choice about how fast the backlog must drain. D5 reserves the non-zero value to Thomas |

→ Constraint: `rfc/drain-budget.txt` carries POLICY ONLY. It may never gain a
per-stem row, a count, a stem list, or a register column. The moment it names an
RFC it has become the hand-kept registry this resolution rejected, and a reviewer
should treat any such row as a defect rather than an extension.
→ Constraint: the file name changed with its contents. Calling a two-field policy
file `rfc/recertified.txt` when it records no recertification is the same defect
this spec set exists to correct, a name whose meaning is narrower than it reads,
so the file is `rfc/drain-budget.txt`.

**Rejected alternatives, recorded so the question is not reopened.**

| Alternative | Why rejected |
|-------------|--------------|
| `rfc/recertified.txt` hand-authored with per-stem `stem`/`date`/`captured`/`source`/`register` rows (this umbrella's original design) | A hand-typed count is a claim, and claims are what this programme removes. It duplicates the extraction artifact's data with weaker staleness detection (a count comparison rather than a source sha) and no per-site classification, and it is the artifact shape the 2026-07-20 ruling already refused |
| `rfc/recertified.txt` as a RENDERED view of `rfc/extraction/` | Harmless but redundant: it is a third copy of what `ai/RFC-REQUIREMENTS.md` already publishes and `--extraction-status --json` already emits, and every copy is one more thing `check_ledger_fresh` must keep fresh |
| The two policy fields as a header inside an existing file (`rfc/enrolled.txt`, or the extraction artifacts) | `rfc/enrolled.txt` is parsed by `line.split()[0]` (`parse_enrolled:688`), so it cannot carry structure without changing a parser three specs depend on. Putting policy inside a per-RFC artifact would scatter one global setting across 166 files |

### Register grading (D6)

The register is what the source text offers a mechanical check, so it decides what
a sign-off can be graded against:

| Register | When it applies | What the sign-off is graded against |
|----------|-----------------|-------------------------------------|
| `rfc2119` | The source uses capitalised MUST / MUST NOT / SHALL / REQUIRED, and the captured count is comparable to the source keyword count | The mechanical source-vs-captured bound. This is the strong grade |
| `prose` | The source states obligations in lowercase indicative prose, so the keyword probe is vacuous. Mechanically: the captured MUST-level rows exceed the source keyword count | A committed declaration that the obligations were extracted by prose reading, recorded on the row itself. This is A-3's fail-closed route, now settled |
| `manual-walk` | The source is present but neither grade is honest for it, and the signer walked it section by section under the Extraction Completeness clause (`ai/rules/rfc-compliance.md:73-98`) | The walk, declared and dated. The weakest grade and the one that must never hide inside a total |

**Both halves of D6, and neither without the other.**

| Half | Rule | What it prevents |
|------|------|------------------|
| Counting | A `prose` or `manual-walk` sign-off COUNTS toward the drain floor exactly as an `rfc2119` one does | 53 of 166 enrolled entries cannot take the `rfc2119` grade under child 1's derivation rule (the site denominator; see "The three pre-2119 measurements"). Excluding the weaker registers would make roughly a third of the corpus permanently undrainable, and a quota nobody can satisfy is a quota that gets deleted, which is the `ai/rules/rfc-compliance.md:114-116` failure mode again |
| Publishing | `ai/RFC-REQUIREMENTS.md` reports the three registers in three separate columns and never renders a single summed "signed off" figure | Reading "N signed off" as "N keyword-verified" is the same category error this whole spec set exists to correct: a number whose meaning is narrower than it reads. Merging the columns would rebuild the defect one layer up |

→ Constraint: a child that needs one number for a threshold uses the per-register
counts and states which registers it summed, in the message it prints. There is no
bare total anywhere, in the ledger or in a gate message.

### What "per release" means mechanically

**It cannot mean a release, because this repository has none.** Verified: `git tag
--list` is empty, there is no `VERSION` file, and there is no version constant. The
closest existing artefact is `ze-release-evidence` (`mk/test-release.mk:83`), which
is a target with no schedule, so it can force nothing. `ai/rules/compatibility.md`
states plainly that Ze has never been released.

So the unit is **calendar time**, which is the only counter in this repository that
advances on its own:

| Element | Definition |
|---------|-----------|
| Budget | `rfc/drain-budget.txt`, carrying a start date and a rate expressed as entries per calendar month, and nothing else |
| Required floor | `ceil(rate x months elapsed since start)`, capped at ~~the current backlog size~~ **the DRAINABLE set size, which is the whole enrolled set** (corrected 2026-07-29 during child 1's implementation, and recorded there as DEV-4). Capping at the RESIDUAL backlog double-counts every sign-off, once by raising the cumulative signed total and once by lowering the cap it is compared against, so the condition collapses to `signed >= enrolled / 2`. Measured on the implemented code before the fix: 166 enrolled at rate 100/month over 12 months flipped red-to-green at exactly 83 sign-offs, so no rate Thomas could arm would ever have demanded more than half the corpus |
| Result | The gate fails when the signed-off stem count, read from ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Command correction" under "Where the counter lives") and derived from `rfc/extraction/`, is below the required floor. Implemented by child 1 as `check_drain_floor` inside the existing `ze-rfc-check` (resolved 2026-07-29; child 1 AC-27 to AC-30) |
| Self-retirement | The check goes permanently green once the backlog is drained, and needs no removal commit. ~~Because the floor is capped at backlog size.~~ The mechanism is the corrected one above (2026-07-29): a drained corpus has `signed == enrolled`, and the floor can never exceed `enrolled`, so the comparison is satisfied from then on. Self-retirement therefore survives the cap correction intact, by a different route than the original wording described |

**The rate ships at zero. Settled by D5, not proposed.** The machinery lands inert,
publishing the backlog and computing a floor of 0. It is armed by a separate
one-line commit that sets a non-zero rate, taken **after** the first fleet batch has
measured real throughput, **and Thomas takes it**. Guessing a rate now would either
be so low it forces nothing or so high it reds the tree on unrelated work, and
`ai/rules/rfc-compliance.md:114-116` records exactly what happens to a rule in the
second category: it gets removed rather than obeyed.

D5 confirms the calendar anchor as designed, on the verified ground that this
repository has no releases: `git tag --list` is empty, there is no `VERSION` file,
and `ze-release-evidence` (`mk/test-release.mk:83`) is a target with no schedule.
A-7 is therefore `confirmed` rather than awaiting validation, and R-2 (the drain
never arms) is an **accepted** risk carrying an owner ruling, not an open one. The
distinction matters to a future reader: R-2 is not a hazard someone forgot to
close, it is a cost the owner priced and took, and the arming commit has a named
owner rather than an empty slot.

### Who implements the floor (resolved 2026-07-29)

**This umbrella specifies the floor and writes none of it.
`plan/spec-rfcgate-1-extraction.md` implements the comparison, as `check_drain_floor`.**
The split had been left implicit and the two specs contradicted each other on it: this
umbrella required the floor to compute and the check to be green (AC-13) while the child
forbade itself the cadence, the quota and a calendar, so the comparison was specified by one
spec, forbidden in the other, and implemented by neither.

| Half | Owner | What it covers |
|------|-------|----------------|
| POLICY | this umbrella, and ultimately Thomas | The rate, the start date, the semantics of the quota, and the acceptance criteria over the result (AC-4, AC-13). Authored into `rfc/drain-budget.txt`, never into code |
| COMPARISON | `plan/spec-rfcgate-1-extraction.md` (its AC-27 to AC-30) | Read the policy, compute the required floor, cap it at the backlog size, judge the derived signed count against it, and fail closed on a missing or unparseable budget |

Three reasons the comparison belongs to child 1 rather than to a fifth child or to this
umbrella. The floor is a pure function of the extraction sign-off set, which child 1 already
derives, already counts and already renders per register, so
`ai/rules/derive-not-hardcode.md` wants the count computed where the data lives. Putting a
ten-line comparison anywhere else would split one derivation across two owners for no
benefit and give `rfc/extraction/` a second reader to keep in step. And per D5 the rate
ships at `0`, so the comparison lands INERT and its first real exercise is the arming
commit, which is Thomas's: carrying an arithmetic that computes 0 on every tree it will see
before he arms it enlarges nothing about the child's scope.

→ Constraint: this umbrella still writes no gate logic, and naming the implementer is the
opposite of taking the work. AC-4 and AC-13 stay here as the assertions; child 1 carries the
code and the tests that satisfy them.

### How it fails closed

Three ways, each mirroring the precedent already in this module at
`scripts/dev/rfc_requirements.py:660-664`, where an empty `rfc/enrolled.txt` is an
error rather than an empty backlog:

| Condition | Behaviour | Why not the permissive reading |
|-----------|-----------|-------------------------------|
| `rfc/drain-budget.txt` missing or unparseable | Hard error naming the file. Child 1 owns this as its AC-29, inside `check_drain_floor` | An absent budget would otherwise compute a floor of 0 and read as "nothing owed", which is the zero-value trap `ai/rules/fail-closed-guards.md` names: a zero value must never be a valid-looking answer. Note the ARTIFACT side has the opposite polarity and needs no guard: an absent `rfc/extraction/` yields zero SIGNED stems, so the backlog reads as its maximum, not as drained |
| A sign-off artifact whose `source-sha` no longer matches the live source text | The artifact is STALE and does not count toward the floor | Otherwise signing once and then editing keeps the credit while the basis moves. Child 1 owns this as its AC-4. It is strictly stronger than the struck ledger's count comparison, which only noticed a change in HOW MANY rows the summary declared |
| A signed stem whose summary later loses a captured requirement | `check_retired_requirements` fires on the vanished id independently of the drain floor | The two guards compose: the extraction artifact bounds what was captured, and the existing ratchet stops a capture disappearing |
| The clock | Read from the committed start date and the current date only | A rate or date overridable by flag or environment variable is a forcing function the caller can silence, which is not a forcing function |
| A sign-off artifact whose `register` is missing, empty, or outside the three defined values | Hard error naming the artifact and the offending value; it does not count | D6 makes the register load-bearing for what the published columns mean. An unreadable register defaulting to the strong grade would let the weakest sign-off be published as the strongest, which inverts the decision. Same shape as child 3's verdict vocabulary check (AC-8), and the same reason |
| An artifact claiming `register: rfc2119` over a source the derivation grades weaker | Hard error: the claimed grade is stronger than the derived one | The register is a property of the source, not a claim the signer may assert. Child 1 derives it and refuses the stronger claim (its AC-9); the artifact may declare the derived register or a weaker one, never a stronger one |

## Relationship to `plan/spec-followup-rfc-enrollment.md`

That spec is a `skeleton` dated 2026-07-17. It was read in full for this design.
**Its numbers are stale and its framing is superseded; its remaining real work is
exactly what D4 defers.**

| Its claim | Reality on 2026-07-29 | Consequence |
|-----------|----------------------|-------------|
| "Today only RFC 7606 is in `rfc/enrolled.txt`" (line 24) | 166 entries are enrolled | The enrolment program it describes has already been executed |
| "~2136 MUST-level requirements owe work across ~146 summaries" (line 30) | 0 outstanding across 166 (`ai/RFC-REQUIREMENTS.md:11`); 2720 gated | Its sizing is inverted: the work did not remain outstanding, it was absorbed into 1376 annotations |
| "9 summaries capture ZERO of their source RFC's MUSTs and must be re-authored" (lines 33-35), naming rfc3630, rfc5187, rfc5303, rfc5304, rfc5310, rfc5392, rfc6549, rfc7684, rfc7770 | All 9 of those are now enrolled. A different set of 9 summaries now declares zero MUST-level rows: rfc1035, rfc3765, rfc4486, rfc5301, rfc6987, rfc7999, rfc8195, rfc8326, rfc9129 | Its zero-capture list is stale in both directions, but the *shape* it identified is real and is blind spot 5 |
| "Owns `rfc/enrolled.txt` and the Coverage-by-RFC rollup going forward" (line 176) | Still true as a home for the drain | It remains the correct destination holder for the fleet work |

**Decision.** This set does not delete or absorb it. `plan/spec-rfcgate-0-umbrella.md`
supersedes its stale framing and takes ownership of the *machinery*;
`plan/spec-followup-rfc-enrollment.md` remains the named destination for the *fleet
drain* (D4). It must be rewritten, not deleted, once child 1 lands and the published
backlog gives it real numbers to be sized against. Rewriting it before then would
re-derive a ranking that child 1 is about to make derivable, which is the
hardcoding this repository's rules exist to prevent.

→ Constraint: no child in this set edits `plan/spec-followup-rfc-enrollment.md`.
The rewrite is follow-on work and belongs to the session that opens the drain.

## Out of Scope (explicit)

Named so that a later reader does not mistake absence for oversight. Each item is
excluded by an owner decision, not by convenience.

| Not in scope | Why | Where it goes |
|--------------|-----|---------------|
| The fleet drain: recertifying the 166 grandfathered entries against their source text | D4: machinery now, fleet after. A fleet run before the machinery exists would recertify against a bar that does not yet exist | `plan/spec-followup-rfc-enrollment.md`, rewritten after child 1 |
| Re-deriving the 1376 `{gap}` / `{not-applicable}` annotations that `ai/rules/rfc-compliance.md:53` voids | Same reason. Re-deriving an annotation is a compliance judgement per requirement and, where it would lower what Ze owes, it is a question for Thomas (`ai/rules/rfc-compliance.md:38-51`), never a batch operation | Same destination; each re-derivation that lowers an obligation is escalated individually |
| Writing the missing tests for the 1746 requirements that are not both-polarity proven | This set builds the instrument that measures them; it does not close the measurement. The two `rfc7296` MUSTs of D7 and the four stems of D8 are the named exceptions below, and they are exceptions because they are CONFIRMED, not because they are convenient | Same destination |
| ~~Changing any `rfc/short/*.md` requirement text or id~~ **Amended by D7 and D8.** Still out of scope: changing the TEXT or the ID of an EXISTING requirement row. Now in scope: ADDING rows for a confirmed unextracted MUST | `check_retired_requirements` protects existing ids and nothing in this amendment touches one; adding a row allocates a new id, which the id contract permits and never renumbers. A text correction remains a compliance act and stays out | Additions: children 1 and 4, per the two rows below. Edits to existing rows: out of the set entirely |
| Changing daemon behaviour to close a `{gap}` | No child touches `internal/`, `pkg/`, or `cmd/`. If proving a D7 or D8 requirement turns out to need a behaviour change, that is the escalation in Failure Routing, not a quiet scope expansion | Individual compliance specs, per RFC |

**Two amendments, and the principle behind them (D7).**

The grandfather in D2 exists because a rule that reds the tree on unrelated work
gets removed rather than obeyed (`ai/rules/rfc-compliance.md:114-116`). It covers
the entries **nobody has looked at**. It has never covered an obligation somebody
has read and confirmed, and reading one is what changes its status:

| Formerly out of scope | Now in scope, by which decision | Confirmed evidence | Owner |
|-----------------------|--------------------------------|--------------------|-------|
| Re-authoring `rfc/short/rfc7296.md` | D7 | Two live unextracted MUSTs read in the source: `rfc/full/rfc7296.txt:1397` ("Retransmission of a message MUST use the same Message ID as the original message") and `:1439` ("the IKE SA MUST be closed or rekeyed" at Message ID exhaustion). The summary captures 18 MUST-level rows against a source keyword count of 310 | `plan/spec-rfcgate-1-extraction.md` |
| Extracting and enrolling `rfc1035`, `rfc3765`, `rfc4486`, `rfc5301` | D8 | All four publish a support claim (three `Supported`, one `Experimental`) over a checklist declaring zero MUST-level rows. `rfc3765` and `rfc4486` have no source text in `rfc/full/` or `rfc/drafts/` at all | `plan/spec-rfcgate-4-ledger.md` |

**The general principle, stated so it governs the next find and not only these
two.** A confirmed unextracted MUST escapes the grandfather by definition: found
means owed. An unextracted obligation is still an obligation
(`ai/rules/rfc-compliance.md:28`), and the session that confirms one is the entry
point that owes it (`ai/rules/no-parking.md`). Recording it as backlog is the
banned move, not the safe one.

→ Constraint: a child that CONFIRMS an unextracted MUST while doing something else
extracts it in that child, and does not file it. What stays out of scope is
*going looking*: no child opens a hunt across the 166, because that hunt is the
fleet drain and D4 puts it after the machinery. The line is between what you have
read and what you have not.
→ Constraint: extracting a MUST is not the end of the obligation. A newly captured
MUST is either proven with tagged tests in both polarities, or it is escalated to
Thomas under `ai/rules/rfc-compliance.md:38-51`. It may NOT be parked behind a
fresh `{gap}` or `{not-applicable}`: those are exactly the classifications the
2026-07-27 directive voids (`:53`), and minting a new one to absorb a newly found
obligation would convert a compliance win into a compliance loss.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The raw deficit of 2636 MUST-level keyword occurrences deflates to roughly 1200-1500 distinct unextracted obligations, because an RFC restates an obligation across sections | Measured `source_keyword_count` minus captured rows per enrolled entry; deflation factor is an estimate, not a measurement | Child 1's capture-ratio bar is calibrated against the wrong denominator and either passes everything or blocks every new enrolment | Child 1 hand-walks the top 3 entries (rfc7296, rfc7950, rfc2661) section by section and reports distinct obligations against keyword occurrences | unvalidated |
| A-2 | A source-vs-captured comparison is a meaningful bound only for the enrolled entries whose source genuinely uses RFC 2119 capitalisation, and under child 1's derivation rule that set is **113**, the other **53** falling to `prose` or `manual-walk` | The three measurements are canonicalised once under "The three pre-2119 measurements": 22 at the keyword-occurrence denominator with zero keywords, 33 at the same denominator declaring more rows than occurrences, 53 at the normative-SITE denominator after boilerplate exclusion. 53 is the operative one because it is the denominator `plan/spec-rfcgate-1-extraction.md`'s register rule uses (its A-2). ~~That set is 133, not 144; 33 in total cannot take the `rfc2119` grade and 133 can.~~ **Corrected 2026-07-29:** 133 was derived from the occurrence denominator while the implementing rule uses the site denominator, so the gradeable set is 113. This is a denominator correction, NOT one measurement correcting another; all three figures stand | The bound is weaker than assumed and blind spot 1 stays partly open after child 1 | Child 1 re-derives the split during its phase 2 and reports the live numbers: 113 gradeable `rfc2119`, 53 requiring `prose` or `manual-walk` | unvalidated |
| A-3 | The pre-2119 entries can be routed to a distinct, non-vacuous check rather than exempted | **Owner ruling D6 (2026-07-29)**, which settles the routing: the distinct check is the `register` column, graded `rfc2119` / `prose` / `manual-walk`, and a weaker register counts toward the quota while being published in its own column. Originally reasoned from `ai/rules/fail-closed-guards.md`: a guard that cannot deny must say something. The set is larger than first measured, and is measured three different ways at three different denominators: see "The three pre-2119 measurements" | They would be exempted, and the entries concerned (OSPFv2, ICMP and DNS Clarifications among them) would keep an unbounded extraction forever, which is blind spot 1 surviving its own fix | Settled by D6. Child 1 implements it: a zero or below-captured source-keyword count fails closed by DERIVING a `prose` or `manual-walk` register for the sign-off artifact, never by defaulting to `rfc2119` and never by exemption | **confirmed** |
| A-4 | Serialising the four children costs less than the ratchet mis-calibration that concurrency would cause | All four modify `scripts/dev/rfc_requirements.py`; all ratchets read git HEAD (`:797`, `:955`, `:1007`, `:1059`) | The set takes four sequential cycles where it could have taken two | Observed at merge: any child that lands with no conflict against its predecessor and no baseline movement disproves the coupling for that pair | unvalidated |
| A-5 | Extending `scan_tree` to admit interop evidence requires the identical change in `_git_baseline_tag_polarities`, or the ratchet baseline desynchronises | The two filters are independent implementations of the same rule (`:558-563` and `:835`); the second carries a comment stating it mirrors the first | Child 2 lands a one-sided widening and the ratchet reports phantom polarity losses on every subsequent run | Child 2 adds a test asserting the two filters accept identical extension sets, driven from both entry points | unvalidated |
| A-6 | No consumer of the audit `verdict` field exists, so child 3 can define its semantics freely | Repo-wide search for a read of the `verdict` key across `scripts/dev/rfc_requirements.py` and `scripts/checks/` returns nothing | Child 3 changes behaviour some other tool depends on | Child 3 re-runs the search across the whole tree, not only the two directories checked here, before defining the semantics | unvalidated |
| A-8 | The two confirmed `rfc7296` MUSTs (D7) can be proven by tagged tests against existing daemon behaviour, without a behaviour change | Both are IKEv2 session-layer obligations over Message ID handling, and `internal/component/ike/` implements the exchange; neither has been traced to its producing function yet, so this is explicitly a hypothesis and not a finding (`ai/rules/no-fabrication.md`) | Child 1 cannot prove what it just extracted. It may NOT mint a `{gap}` to absorb it: that is the classification the 2026-07-27 directive voids | Child 1 reads the producing function for each MUST and cites it `file:line`. If the behaviour is absent, the finding goes to Thomas under `ai/rules/rfc-compliance.md:38-51` as a "which way do I fix it" question, and the spec stays open | unvalidated |
| A-9 | Extracting the four D8 stems yields at least one MUST-level row each, so their public support claims become backed rather than merely unmeasured | Three publish `Supported` and one `Experimental` over zero captured rows; `rfc1035` (DNS) and `rfc4486` (Cease subcodes) plainly carry obligations Ze implements | A stem genuinely has no MUST-level obligation in the part Ze implements. Then `check_unproven_support` would red on a claim that is honest, and the check would be wrong rather than the claim | Child 4 extracts first and counts. A stem that truly captures zero after a real walk is not forced to fabricate a row: it changes its ledger claim or records the walk as a `manual-walk` register row saying why zero is the right answer, and `check_unproven_support` must accept that evidenced form | unvalidated |
| A-7 | Calendar time is an acceptable substitute for "per release" in D2, given no release mechanism exists | **Owner ruling D5 (2026-07-29): confirmed as designed.** Underlying evidence unchanged and re-verified: `git tag --list` empty; no `VERSION` file; no version constant; `ze-release-evidence` (`mk/test-release.mk:83`) unscheduled; `ai/rules/compatibility.md` states Ze has never been released | It would have been mis-shaped and Thomas would have bound it to something else. He did not | Settled. No further validation is owed, and no child may re-open it. The one remaining action is the arming commit, which is Thomas's and is tracked as R-2 | **confirmed** |

### Risks

`Status` is `open` (live, mitigated but unresolved) or `accepted` (the owner has
priced the risk and taken it, so it is a recorded cost rather than an outstanding
hazard). An `accepted` row is not a row anybody forgot to close.

| ID | Risk | Early signal | Mitigation / fallback | Status |
|----|------|--------------|----------------------|--------|
| R-1 | **The sign-off artifact becomes a 1746-slot escape hatch.** A new sign-off artefact is a new place to write an excuse. 1746 requirements are not both-polarity proven; if an artifact can be filed without a genuine section-by-section walk, the set will have replaced one unaudited annotation with another, at greater cost | Artifacts appearing faster than any plausible reading rate, or arriving in batches with identical `signed-off` dates and no accompanying summary edits | Structural rather than procedural, which is what the resolution of 2026-07-29 bought: the artifact is per-SITE classified, its skeleton writer can emit only UNCLASSIFIED entries, and an unclassified site FAILS the check, so a batch of generated artifacts makes the gate redder rather than greener. It pins `source-sha`, so it goes stale on any change to the source text. Exclusions are shrink-only per stem against HEAD and a rise needs a recorded `resign-reason`, and the per-RFC exclusion ratio is published. D6's derived `register` narrows the hatch further: `manual-walk` is visibly the weakest grade and is published as such, so filing one to dodge a bound advertises the dodge | open |
| R-2 | **The drain never arms.** The rate ships at 0 by design; if nobody sets it, D2's forcing function is a floor with no force, and the set has built a backlog counter rather than a drain | The published backlog count flat across calendar quarters while the rate stays 0 | **ACCEPTED by Thomas under D5 (2026-07-29).** The ruling: ship inert, arm once the first fleet batch has measured real throughput, and Thomas takes the arming commit himself. The risk is not mitigated away, it is priced: a guessed rate is either inert anyway or reds the tree on unrelated work and gets deleted (`ai/rules/rfc-compliance.md:114-116`), so shipping at 0 dominates both alternatives. Fallback if it never arms: publish-only, which is strictly better than today's invisible backlog. Accountability, which is what changed on 2026-07-29: the arming commit has a named owner rather than an empty slot, and `plan/spec-followup-rfc-enrollment.md` carries it as a deliverable | **accepted** |
| R-3 | **Binding requirements to tests nothing runs makes the ledger LESS honest.** Admitting interop evidence lets 104 scenarios that no automation executes start satisfying MUST-level requirements. A requirement proven only by an unrun scenario reads identically to one proven by a `ze-verify` test, and the ledger's total would rise while real proof did not | An interop-bound requirement count that grows while the nightly interop workflow is red, absent, or skipped | D3's ordering is the primary mitigation: `.ci` bindings are PREFERRED because `.ci` runs in `ze-verify` on every push. Child 2 must additionally mark interop-bound requirements as a distinct evidence class in `ai/RFC-REQUIREMENTS.md`, never merged into the `ze-verify`-proven total, and must make the runner fail closed first so an unrun lab cannot masquerade as a pass. This is the same separate-columns discipline D6 imposes on registers, applied to evidence classes | open |
| R-4 | **A date-based red gets deleted rather than obeyed.** `ai/rules/rfc-compliance.md:114-116` records this outcome explicitly for rules that red the gate on unrelated work | Any developer proposing to disable, skip, or raise the budget rather than recertify | Rate ships at 0; arming happens only after measured throughput; the floor caps at backlog size and self-retires. If it still proves intolerable, the fallback is publish-only, which preserves visibility and loses only the forcing | open |
| R-5 | **Child 1's new bound reds the tree for the existing 166.** A source-vs-captured comparison applied without grandfathering would fail 127 of the 166 entries on the first run, those carrying a positive deficit; the entries that cannot take the `rfc2119` grade would fail the register check instead (53 at the site denominator, 33 at the keyword-occurrence one; see "The three pre-2119 measurements"). Either way the red is the overwhelming majority of the enrolled set | `make ze-rfc-check` red on a tree nobody changed | D2 mandates grandfathering: the bound blocks NEW enrolments and ratchets the existing set. Child 1's first acceptance criterion is that the check is green on an unmodified tree | open |
| R-6 | **`check_status_agreement` made unconditional (child 4) fires the `row is None` cliff for 32 enrolled entries at once** | Child 4's first local run reds with 32 identical errors | Child 4 resolves the 32 rows as its own step 4d, before tightening the check. The cliff is known and enumerated in blind spot 6, so this is scheduled work rather than a discovery | open |
| R-7 | **The 3 entries with an empty `tests` map are permanently fresh today; rejecting them (child 3) may red an artefact nobody can fix** | Child 3's schema validation reds `rfc/audit/rfc7606.json` with no obvious correct value to supply | Those 3 entries are unfalsifiable by construction, so rejecting them is correct. If a genuine no-test verdict is meaningful (an `unimplemented` verdict on a `{gap}` requirement legitimately cites no test), child 3 must model that case explicitly rather than by an empty map | open |
| R-8 | **`check_unproven_support` arms before the four stems are resolved and blocks every commit in the repository.** `ze-rfc-check` runs inside `make ze-verify` in both modes, and `commit_helper.py create` refuses to prepare ANY commit script over a non-FRESH verify (`ai/rules/git-safety.md`, Step 1), including the one that would fix it. The blast radius is every concurrent session, not just this spec set. ~~`ze-rfc-check` is a structural gate and `ai/rules/git-safety.md` makes a structural red unbypassable~~ (corrected 2026-07-29: it is NOT in `STRUCTURAL_GATES`, `scripts/dev/commit_helper.py:512-523`; see "Why 4c can never precede 4b") | A child-4 working tree where `check_unproven_support` exists and `rfc/enrolled.txt` does not yet name all four stems | The ordered constraint 4a to 4d in "Sequencing Constraint", which exists specifically for this. Child 4 must run the check locally BEFORE committing it and confirm it is green on its own tree. Recovery if it lands anyway is NOT a bypass: `--unverified` is legitimate only for a flaky or environmental red, and `ai/rules/fix-dont-record.md` bans a `plan/known-failures/` shard for a deterministic reproducible one. The only honest route is an explicit owner ruling on every commit until the four are cleared, which is precisely why this must not be reached by accident | open |
| R-9 | **A newly extracted MUST is absorbed by a fresh `{gap}` and the compliance position gets worse, not better.** D7 and D8 add requirement rows. The cheapest way to keep the gate green over a new row is to annotate it, and `{gap}` / `{not-applicable}` are exactly what the 2026-07-27 directive voids as authority | Child 1 or child 4 committing new checklist rows whose annotations outnumber their tagged tests | Stated as a constraint in "Out of Scope": a newly captured MUST is proven in both polarities or escalated to Thomas, never annotated. The gate cannot catch this on its own (a `{gap}` is a legal annotation), so it is a review obligation: the Critical Review Checklist carries the row | open |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and no daemon behaviour: this set touches gate tooling, data files, and CI config only. The failure mode is developer-facing and comes in two shapes. Too strict: `make ze-verify` reds on unrelated work and the rule gets deleted rather than obeyed (R-4, R-5). Too loose: the gate keeps reporting a green that is narrower than it reads, which is today's state, and the set will have cost four cycles for nothing |
| How is it reverted? | Single commit revert per child, and the children are serialised precisely so that each is independently revertible. Two wrinkles. The first is `rfc/drain-budget.txt`: reverting the code leaves the policy file, which is harmless (it is inert without a reader) but should be removed in the same revert to avoid a budget nobody consumes. The extraction ARTIFACTS under `rfc/extraction/` are different and are NOT removed by a revert of the machinery: each records a walk somebody actually performed, and destroying that is `ai/rules/never-destroy-work.md`. They simply go unread until the machinery returns. The second wrinkle is sharper and is why R-8 exists: a red `ze-rfc-check` leaves `make ze-verify` non-green, and `commit_helper.py` then refuses to prepare ANY commit script, **including the revert of the commit that caused it**. ~~Escaping that state needs the owner-only `--structural-red-ok` override.~~ (Corrected 2026-07-29: `ze-rfc-check` is not in `STRUCTURAL_GATES`, so escaping needs `--unverified`, which is legitimate only for a flaky or environmental red -- see "Why 4c can never precede 4b".) A new check must therefore be green in the commit that introduces it, which is exactly what the 4a-to-4d ordering guarantees |
| Who else touches this path? | `plan/spec-followup-rfc-enrollment.md` (skeleton, stale, named as the fleet-drain destination). Any session running `/ze-rfc`, `/ze-rfc-audit`, or enrolling an RFC touches `rfc/short/`, `rfc/enrolled.txt`, and the tags this gate reads. Because the ratchets compare against git HEAD, a concurrent session adding tags shifts the baseline mid-implementation; this is the same coupling that forces the children to serialise, and it applies to unrelated sessions too |

## Wiring Test (MANDATORY -- NOT deferrable)

Every row names the gate entry point, the function that runs, and the concrete
test proving the chain. `.ci` functional tests are N/A for this set: it changes no
daemon behaviour and adds no user-facing command, so the driving surface is the
gate's own Python test file, which `TestPythonUnitTests` executes (see Functional
Tests below).

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rfc-check` on a tree with a new stem in `rfc/enrolled.txt` and no sign-off artifact | → | child 1's extraction bound inside `check_enrolment` | `TestEnrolment.test_new_enrolment_without_signoff_fails` in `scripts/dev/rfc_requirements_test.py` (child 1 owns the name; this row follows it) |
| `make ze-rfc-check` on an unmodified tree carrying the grandfathered 166 | → | child 1's grandfathering split between backlog and gated set | `TestRealTree.test_live_tree_is_green_with_zero_signoffs` in `scripts/dev/rfc_requirements_test.py` |
| `make ze-rfc-check` with a missing `rfc/drain-budget.txt` | → | child 1's fail-closed guard on the POLICY input, inside `check_drain_floor`, mirroring `check_enrolment:660-664` | `TestDrainFloor.test_missing_drain_budget_is_error_not_empty` in `scripts/dev/rfc_requirements_test.py` |
| `make ze-rfc-check` with `rfc/extraction/` absent entirely | → | child 1's backlog derivation | `TestEnrolment.test_preexisting_enrolment_without_signoff_passes`: zero artifacts reads as zero SIGNED and a full backlog, never as a drained one. The artifact side needs no fail-closed guard because its polarity is already safe |
| An `RFC requirement:` tag placed in an interop scenario file | → | child 2's widened `scan_tree` and the mirrored `_git_baseline_tag_polarities` filter | `test_tree_and_baseline_filters_agree` in `scripts/dev/rfc_requirements_test.py` (~~`test_interop_tag_admitted_by_both_scanners`~~, renamed 2026-07-29 to the name child 2 carries for its AC-8, which is the both-filters-agree assertion this row describes) |
| `python3 test/interop/run.py` with Docker absent | → | child 2's fail-closed exit, matching `test/ipsec-interop/run.py:119-120` | `TestInteropRunnerFailsClosedWithoutDocker` in `test/interop/run_test.go` (~~`test_interop_runner_fails_closed_without_docker` in `test/interop/run_test.py`~~, renamed 2026-07-29 to the name child 2 carries for its AC-1; child 2 shells to the runner from a Go test, so the carrier is `.go`) |
| An `rfc/audit/*.json` entry whose verdict word is outside the four defined values | → | child 3's schema validation in `load_audit` | `TestAuditSchema.test_unknown_verdict_value_fails` in `scripts/dev/rfc_requirements_test.py`, driven from `run_check` by child 3's `TestAuditSchemaWiring` (~~`test_audit_vocabulary_drift_is_parse_error`~~, renamed 2026-07-29 to the name child 3 carries for its AC-1, whose fixture is the live `implemented` drift) |
| An `rfc/audit/*.json` entry recorded `weak` over a both-polarity-tagged requirement | → | child 3's verdict reader | `TestAuditLedger.test_weak_verdict_removes_proven_status` in `scripts/dev/rfc_requirements_test.py` (added to child 3's TDD plan 2026-07-29 behind its new AC-24, since no existing child-3 test covered the loss of proven status; ~~unprefixed `test_weak_verdict_removes_proven_status`~~, class prefix added so both specs carry one string) |
| A `docs/features/rfc-status.md` row claiming public support for a summary declaring zero requirements | → | child 4's `check_unproven_support` | `TestUnprovenSupportWiring.test_run_check_fails_on_support_claim_over_zero_gated_requirements` in `scripts/dev/rfc_requirements_test.py` (~~`test_supported_claim_requires_captured_requirement`~~, renamed 2026-07-29 to the name child 4 carries for its AC-10) |
| An enrolled stem with no `docs/features/rfc-status.md` row | → | child 4's unconditional row presence check | `TestStatusCompletenessWiring.test_run_check_fails_when_a_new_enrolment_has_no_row` in `scripts/dev/rfc_requirements_test.py` (~~`test_enrolled_entry_requires_ledger_row`~~, renamed 2026-07-29 to the name child 4 carries for its AC-1. Note the narrower scope child 4 settled: its ratchet fires on a NEW enrolment or a DELETED row, and its AC-3 grandfathers the 32 pre-existing rowless enrolments, which AC-12 below still reads as resolved) |
| A sign-off artifact graded `prose`, counted against the drain floor | → | child 1's register-aware floor computation (D6 counting half) | `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` in `scripts/dev/rfc_requirements_test.py` |
| The rendered extraction section of `ai/RFC-REQUIREMENTS.md` | → | child 1's register-split renderer (D6 publishing half) | `TestExtractionLedger.test_registers_are_published_in_separate_columns` in `scripts/dev/rfc_requirements_test.py` |
| A sign-off artifact whose `register` word is outside the three defined values, or claims a grade stronger than the derivation supports | → | child 1's register derivation and validation, same shape as child 3's verdict vocabulary check | `TestSiteInventory.test_register_is_derived_not_authored` and `TestPre2119FailsClosed.test_keyword_free_source_cannot_claim_rfc2119` in `scripts/dev/rfc_requirements_test.py` |
| `make ze-rfc-check` on a tree where `rfc/short/rfc7296.md` has lost either confirmed MUST | → | child 1's `check_retired_requirements`, which already blocks a requirement id disappearing | `TestRealTree.test_rfc7296_ids_are_neither_retired_nor_demoted` in `scripts/dev/rfc_requirements_test.py` |
| `make ze-rfc-check` with a drain budget whose floor would exceed the backlog size, and with one whose armed floor exceeds the signed count | → | child 1's `check_drain_floor`; this umbrella owns the policy it reads, not the comparison | `TestDrainFloor.test_drain_floor_caps_at_backlog_size` and `TestDrainFloor.test_signed_below_armed_floor_fails` in `scripts/dev/rfc_requirements_test.py` |
| `make ze-rfc-check` on a tree where a D8 stem is enrolled but its summary captures zero rows | → | child 4's `check_unproven_support` driven from the gate entry point | `TestUnprovenSupportWiring.test_run_check_fails_on_support_claim_over_zero_gated_requirements` in `scripts/dev/rfc_requirements_test.py`, with `TestFourStemEnrolmentRealTree.test_each_declares_a_gated_requirement` as its real-tree half over the four D8 stems (~~`test_enrolled_zero_capture_claim_fails`~~, renamed 2026-07-29 to the names child 4 carries. This row and the public-support row above now cite the same wiring test, because they state one obligation twice) |

## Acceptance Criteria

Umbrella-level. Each child carries its own AC table; these are the assertions that
only the set as a whole can satisfy.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An unmodified tree, after all four children have landed | `make ze-verify` is green. No child reds the tree for pre-existing backlog |
| AC-2 | A new stem added to `rfc/enrolled.txt` with a summary capturing far fewer MUST-level rows than its source contains | `make ze-rfc-check` fails, naming the stem, the captured count, the source count, and the sign-off artifact that is missing |
| AC-3 | The 166 grandfathered entries | `ai/RFC-REQUIREMENTS.md` publishes the backlog count, the signed count, and the per-entry deficit ranking, all derived from `rfc/extraction/` and the live summaries, never hand-written |
| AC-4 | `rfc/drain-budget.txt` deleted from the working tree | `make ze-rfc-check` reports a hard error naming the file. It does not compute a floor of 0 and report nothing owed. Implemented by child 1's `check_drain_floor` (its AC-29); this umbrella owns the assertion, not the code |
| AC-4a | `rfc/extraction/` absent or empty | The backlog reads as the full enrolled set and the signed count reads 0. An absent record must never read as a drained backlog, and the derivation's polarity gives that for free rather than needing a guard |
| AC-5 | A sign-off artifact whose `source-sha` no longer matches the live source text | The artifact does not count toward the drain floor, and the gate says which stem went stale and why |
| AC-6 | Docker absent, `python3 test/interop/run.py` invoked | The runner exits non-zero. The BGP and IPsec runners agree on this |
| AC-7 | A requirement whose only evidence is an interop scenario tag | It is reported in `ai/RFC-REQUIREMENTS.md` as a distinct evidence class, never merged into the count of requirements proven by `ze-verify`-resident tests |
| AC-8 | An `rfc/audit/*.json` entry with a verdict word outside `enforced`, `weak`, `wrong`, `unimplemented` | Loading the audit file is a parse error naming the entry and the offending word |
| AC-9 | An `rfc/audit/*.json` entry with an empty `tests` map | The entry is rejected. An entry that cannot go stale cannot be counted |
| AC-10 | A requirement carrying a `weak` or `wrong` verdict and both polarity tags | It is no longer reported as proven; the ledger shows the verdict as the reason |
| AC-11 | A `docs/features/rfc-status.md` row claiming public support backed by a summary declaring zero MUST-level requirements | `check_unproven_support` fails, naming the row and the summary. It covers `Experimental` as well as `Supported`, or `rfc5301` escapes it. All four known cases (RFC 1035, RFC 3765, RFC 4486, RFC 5301) are resolved before the check is armed |
| AC-12 | Any of the 32 enrolled entries with no `docs/features/rfc-status.md` row | Each has a row before child 4 tightens the check, so the `row is None` cliff cannot fire as a surprise |
| AC-13 | The drain budget, before the arming commit | The rate is 0, the floor computes to 0, and the check is green while still publishing the backlog. The floor is computed and capped by child 1's `check_drain_floor` (its AC-27, AC-28, AC-30), which is where the comparison is implemented; this umbrella owns the POLICY it reads and this criterion over its result, and writes no code (resolved 2026-07-29, see "Who implements the floor") |
| AC-14 | The four children, at merge time | Each landed alone and green, in the order 1, 2, 3, 4, with no two in flight simultaneously |
| AC-15 | A sign-off artifact graded `prose` or `manual-walk` (D6) | It counts toward the drain floor exactly as an `rfc2119` artifact does. A quota that excluded the weaker registers would be unsatisfiable for the 53 of 166 entries that cannot take the `rfc2119` grade (the site-denominator figure; see "The three pre-2119 measurements") |
| AC-16 | The published extraction section of `ai/RFC-REQUIREMENTS.md` (D6) | The three registers appear in three separate columns. No summed "signed off" figure is rendered anywhere in the ledger or in any gate message. A reader cannot mistake a `manual-walk` sign-off for a keyword-verified one |
| AC-17 | A sign-off artifact whose `register` is missing, empty, or outside `rfc2119` / `prose` / `manual-walk`, or one claiming `rfc2119` over a source the derivation grades weaker | The gate reports a hard error naming the artifact and the offending value. It does not default to the strong grade and does not silently drop the artifact |
| AC-18 | `rfc/short/rfc7296.md` after child 1 (D7) | It captures the retransmission Message ID reuse MUST (`rfc/full/rfc7296.txt:1397`) and the Message ID exhaustion MUST (`:1439`). Each carries tagged tests in both polarities, or an escalation to Thomas recorded in the spec. Neither is absorbed by a new `{gap}` or `{not-applicable}` |
| AC-19 | `rfc1035`, `rfc3765`, `rfc4486`, `rfc5301` after child 4 (D8) | All four have source text present (`rfc/full/rfc3765.txt` and `rfc/full/rfc4486.txt` newly fetched), a summary capturing their MUST-level obligations, a row in `rfc/enrolled.txt`, and a sign-off artifact whose DERIVED register reflects their source. `rfc1035`'s obligations are extracted from lowercase pre-2119 prose under the `prose` register |
| AC-20 | The commit that arms `check_unproven_support` | It is the same commit that resolves the four stems, or a later one. Running `make ze-rfc-check` on the parent commit of the arming commit, with the check present, must not be a state that ever existed in history |
| AC-21 | The commit order around child 4's four D8 enrolments | Child 1's extraction bar is on HEAD before the first of them, and each of the four carries a valid `rfc/extraction/<stem>.json` sign-off. A history in which any of the four enrolled before child 1 landed fails this AC even if the final tree is green, because that stem is then grandfathered out of the extraction bar permanently and no later state records it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnrolment.test_new_enrolment_without_signoff_fails` | `scripts/dev/rfc_requirements_test.py` | AC-2: a new enrolment is blocked without a sign-off artifact | |
| `TestRealTree.test_live_tree_is_green_with_zero_signoffs` | `scripts/dev/rfc_requirements_test.py` | AC-1: the existing 166 do not red the gate. ~~`test_grandfathered_backlog_is_green`~~ (renamed 2026-07-29 to the name child 1 actually carries; the unit-level twin is `TestEnrolment.test_preexisting_enrolment_without_signoff_passes`) | |
| `TestDrainFloor.test_missing_drain_budget_is_error_not_empty` | `scripts/dev/rfc_requirements_test.py` | AC-4: the zero-value trap is closed on the policy input (child 1 AC-29; name adopted there 2026-07-29, class prefix added so both specs carry one string) | |
| `TestExtractionSignoff.test_source_sha_mismatch_fails` | `scripts/dev/rfc_requirements_test.py` | AC-5: credit is falsified by a later change to the source text (child 1 owns the name) | |
| `TestPre2119FailsClosed.test_keyword_free_source_cannot_claim_rfc2119` | `scripts/dev/rfc_requirements_test.py` | A-3 (confirmed via D6): a zero or below-captured source-keyword count fails closed by requiring a `prose` or `manual-walk` register, never by exemption or by defaulting to `rfc2119`. Its derivation twin is `TestSiteInventory.test_register_is_derived_not_authored`. ~~`test_pre_2119_source_requires_explicit_declaration`~~ (renamed 2026-07-29 to the name child 1 carries) | |
| `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` | `scripts/dev/rfc_requirements_test.py` | AC-15 and D6 counting half: a `prose` or `manual-walk` row counts exactly as an `rfc2119` row does. ~~`test_weak_register_counts_toward_floor`~~ (renamed 2026-07-29 to the name child 1 carries, which the Wiring Test table above already used) | |
| `TestExtractionLedger.test_registers_are_published_in_separate_columns` | `scripts/dev/rfc_requirements_test.py` | AC-16 and D6 publishing half: three columns, and no summed "signed off" figure anywhere. ~~`test_registers_render_in_separate_columns`~~ (renamed 2026-07-29 to the name child 1 carries) | |
| `TestExtractionArtifact.test_unknown_register_is_hard_error` | `scripts/dev/rfc_requirements_test.py` | AC-17, first arm: an unreadable register is an error, never a silent default to the strong grade. Added to child 1's TDD plan 2026-07-29 under this name, with child 1 AC-31 behind it | |
| `TestPre2119FailsClosed.test_rfc2119_register_below_source_count_is_rejected` | `scripts/dev/rfc_requirements_test.py` | AC-17, second arm: a row cannot claim the strong grade and fail the bound that grade is defined by. Added to child 1's TDD plan 2026-07-29 under this name, with child 1 AC-32 behind it | |
| `TestRealTree.test_rfc7296_ids_are_neither_retired_nor_demoted` | `scripts/dev/rfc_requirements_test.py` | AC-18 and D7: once extracted, the two confirmed MUSTs are protected by the existing id ratchet, which child 1 drives across the whole re-authoring. ~~`test_rfc7296_confirmed_musts_cannot_be_retired`~~ (renamed 2026-07-29 to the name child 1 carries) | |
| `TestUnprovenSupport.test_unsupported_and_future_rows_are_not_claims` | `scripts/dev/rfc_requirements_test.py` | AC-11 and AC-19: `check_unproven_support` covers `Experimental` as well as `Supported`, so `rfc5301` cannot escape it. Child 4 states the rule by its complement (its AC-10: a support claim is any Status other than `Unsupported` or `Future`), which is what makes `Experimental` non-escaping. ~~`test_enrolled_zero_capture_claim_fails`~~ (renamed 2026-07-29 to the name child 4 carries) | |
| `TestDrainFloor.test_drain_floor_caps_at_backlog_size` | `scripts/dev/rfc_requirements_test.py` | AC-13 and self-retirement: the floor never exceeds the backlog (child 1 AC-28; the name is adopted verbatim there, with `TestDrainFloor.test_signed_below_armed_floor_fails` as its discriminating twin) | |
| `test_tree_and_baseline_filters_agree` | `scripts/dev/rfc_requirements_test.py` | A-5: the live filter and the git-baseline filter accept identical extension sets. ~~`test_interop_tag_admitted_by_both_scanners`~~ (renamed 2026-07-29 to the name child 2 carries for its AC-8) | |
| `test_nightly_only_marker_rendered` | `scripts/dev/rfc_requirements_test.py` | AC-7: interop-bound requirements are not merged into the `ze-verify`-proven total. Child 2 generalises the class from "interop" to "nightly tier" (its AC-11: its own ledger marker and its own rollup column), which covers interop and any later nightly carrier. ~~`test_interop_evidence_is_a_distinct_class`~~ (renamed 2026-07-29 to the name child 2 carries) | |
| `TestInteropRunnerFailsClosedWithoutDocker` | `test/interop/run_test.go` | AC-6: a missing lab is a failure, not a pass. ~~`test_interop_runner_fails_closed_without_docker` in `test/interop/run_test.py`~~ (renamed 2026-07-29 to the name child 2 carries for its AC-1; child 2 shells to the runner from a Go test, so the carrier is `.go`) | |
| `TestAuditSchema.test_unknown_verdict_value_fails` | `scripts/dev/rfc_requirements_test.py` | AC-8: the observed `implemented` drift is caught. ~~`test_audit_vocabulary_drift_is_parse_error`~~ (renamed 2026-07-29 to the name child 3 carries for its AC-1) | |
| `TestAuditSchema.test_enforced_with_empty_tests_fails` | `scripts/dev/rfc_requirements_test.py` | AC-9: the 3 unfalsifiable entries are rejected. Child 3 decomposes this rather than banning every empty `tests` map, which would make `unimplemented` unrecordable: an `enforced` verdict with no test is rejected outright (its AC-5), and an `unimplemented` one must carry a `code` map (its AC-7) that stales when the cited producer is edited (its AC-8), which is what closes the permanently-fresh hole AC-9 names. ~~`test_audit_entry_without_tests_is_rejected`~~ (renamed 2026-07-29 to the name child 3 carries) | |
| `TestAuditLedger.test_weak_verdict_removes_proven_status` | `scripts/dev/rfc_requirements_test.py` | AC-10: the verdict field gains a consequence. Added to child 3's TDD plan 2026-07-29 behind its new AC-24: child 3 had a test for a `weak` verdict NOT failing the gate (its AC-10) and a worklist naming every finding (its AC-18), but none for the requirement losing proven status, which is the obligation the Child Specs table gives it. ~~unprefixed `test_weak_verdict_removes_proven_status`~~ (class prefix added so both specs carry one string) | |
| `TestUnprovenSupport.test_support_claim_over_zero_gated_fails` | `scripts/dev/rfc_requirements_test.py` | AC-11: an empty summary cannot back a public claim. ~~`test_supported_claim_requires_captured_requirement`~~ (renamed 2026-07-29 to the name child 4 carries for its AC-10) | |
| `TestStatusCompleteness.test_new_enrolment_without_a_row_fails` | `scripts/dev/rfc_requirements_test.py` | AC-12: the `row is None` cliff is closed deliberately. Child 4 closes it as a git-HEAD ratchet on new enrolments and deleted rows, not by writing the 32 rows: its OR-3 defers those to `plan/spec-followup-rfc-enrollment.md`, behind the annotation re-derivation, so AC-12's "each has a row before child 4 tightens the check" overstates what child 4 now delivers. ~~`test_enrolled_entry_requires_ledger_row`~~ (renamed 2026-07-29 to the name child 4 carries) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Drain rate (entries per calendar month) | 0..166 | 166 | negative | 167 (exceeds the whole enrolled set) |
| Required floor | 0..backlog size | backlog size | negative | backlog size + 1 (must be capped, never exceeded) |
| `source_keyword_count` for a pre-2119 entry | 0 | 0 | N/A | N/A (0 is valid input and must fail closed, not read as no deficit) |
| Captured MUST-level rows per summary | 0..source count | source count | N/A | source count + 1 (a summary capturing more than its source contains is a misquote signal) |
| Audit `tests` map size | 1..N | 1 | 0 (rejected, AC-9) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `scripts/dev/rfc_requirements_test.py` | beside the tool, run by `TestPythonUnitTests` | A developer runs `make ze-rfc-check` and gets a verdict that reflects bounded extraction, admissible evidence, real verdicts, and a checked public ledger | |
| `test/interop/run_test.py` | beside the runner | A developer or a nightly job runs the interop suite without Docker and gets a failure rather than a silent pass | |

`.ci` functional tests are N/A for this set: no daemon code, no user-facing command,
no wire behaviour changes. The `.py` surfaces above are the driving entry points, per
`ai/rules/testing.md` "Testing Python Tooling".

## Files to Modify
- `scripts/dev/rfc_requirements.py` - all four children; the sequencing constraint above governs the order
- `scripts/dev/rfc_requirements_test.py` - the unit tests for every new check
- `test/interop/run.py` - child 2: fail closed on a missing lab
- `docs/features/rfc-status.md` - child 4: resolve the 32 missing rows and the four unproven-support claims (D8)
- `rfc/short/rfc7296.md` - child 1: add the two confirmed unextracted MUSTs (D7). Additions only; no existing row's text or id changes
- `rfc/short/rfc1035.md`, `rfc/short/rfc3765.md`, `rfc/short/rfc4486.md`, `rfc/short/rfc5301.md` - child 4: extract the MUST-level obligations of the four D8 stems
- `rfc/enrolled.txt` - child 4: enrol the four D8 stems, after extraction, never before
- `ai/RFC-REQUIREMENTS.md` - regenerated; gains the extraction backlog and the interop evidence class
- `ai/rules/rfc-compliance.md` - record the extraction sign-off contract and the drain schedule beside the four existing ratchets
- `ai/rules/hook-mapping.md` - register the new gate checks so a future agent can find what rejected them
- `ai/skills/ze-rfc-audit.md` - child 3: the verdict vocabulary becomes machine-enforced rather than documentary
- `Makefile` - child 1: the two new targets `ze-rfc-extract` and `ze-rfc-extraction-status`, declared beside the existing `ze-rfc-check` (`Makefile:437`) and `ze-rfc-index` (`:442`). Any later child that introduces a make target adds it in the same place
- `ai/INDEX.md` - child 1: a Dev Tools row per new target beside the existing `ze-rfc-check` / `ze-rfc-index` rows (`:212-213`), and the RFC keyword row (`:372`). This is the discovery surface for the new targets (`ai/rules/discovery-updates.md`)
- ~~`mk/inventory.mk` - if a new make target is introduced by any child~~ (Resolved 2026-07-29, and deliberately no longer conditional: `mk/inventory.mk` needs NO entry. Verified on the tree: `ze-rfc-check` and `ze-rfc-index` are declared in `Makefile:437` and `:442`, neither appears anywhere under `mk/`, and `mk/inventory.mk`'s "Quick reference" header lists only the inventory, doc, spec-status and command targets that file itself declares. Its one RFC touch is a call to `rfc_requirements.py --check-fresh` inside the `ze-doc-test` recipe, which no child changes. A conditional row invited a future reader to register a `Makefile` target in the wrong file)

## Files to Create
- `rfc/drain-budget.txt` - the drain POLICY only: a start date and a rate, and nothing else (child 1, D5). ~~`rfc/recertified.txt` - the recertification ledger and drain budget, with the `register` column.~~ (Resolved 2026-07-29: the record is the `rfc/extraction/<stem>.json` set child 1 already creates, and the quota is derived from it. See "Where the counter lives".)
- `test/interop/run_test.py` - the fail-closed test for the interop runner (child 2)
- `rfc/full/rfc3765.txt`, `rfc/full/rfc4486.txt` - the two missing D8 source texts, fetched before extraction so `source_keyword_count` can return a real number rather than None (child 4, step 4a)
- `plan/spec-rfcgate-1-extraction.md` .. `plan/spec-rfcgate-4-ledger.md` - the four children, authored concurrently with this umbrella by sibling sessions

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Developer gate tooling; no daemon config surface |
| YANG validation constraints | N-A | No YANG leaf added |
| YANG custom validators | N-A | No YANG leaf added |
| CLI commands/flags | No | Children may add flags to `rfc_requirements.py`, which is a script, not a `ze` subcommand. If a child adds a `ze` subcommand it must revisit this row |
| CLI grammar (keyword before value) | N-A | No `ze` command added |
| Editor autocomplete | N-A | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | N-A | The drain clock is deliberately not overridable by environment variable; that is the point of the fail-closed design |
| Doctor check for runtime dependencies | No | The gate reads committed files only. Child 2's interop runner depends on Docker, but that is a developer lab, not a daemon runtime dependency, and it now fails closed rather than degrading |
| Prometheus counters/metrics | N-A | Developer tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Gate tooling; no operator-visible feature |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No `ze` command added |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | No | Developer tooling, not operator-facing |
| 7 | Wire format changed? | No | No protocol code touched |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` (child 4 resolves 32 missing rows and the four unproven-support claims, D8). Plus `rfc/short/rfc7296.md` (D7) and the four D8 summaries: these ADD requirement rows for confirmed obligations. No EXISTING requirement's text or id changes, so `check_retired_requirements` is untouched and this set still measures rather than re-derives |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/` for the interop runner's new fail-closed behaviour and its nightly workflow |
| 11 | Affects daemon comparison? | No | No capability changes |
| 12 | Internal architecture changed? | No | No daemon architecture touched |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `scripts/dev/rfc_requirements.py`, `test/interop/run.py`, and `ai/RFC-REQUIREMENTS.md`, and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/rfc-compliance.md` "What Keeps RFC Testing Valid" describes four ratchets; it must describe the new checks too, or it becomes the stale guidance this set exists to prevent |

## Implementation Steps

This umbrella writes no gate logic. Its implementation is the sequencing, the
scope boundary, and the drain design that the children execute.

1. **Phase: Wiring (MANDATORY FIRST)** -- confirm the set's entry point and scope boundary before any child begins
   - Tests: the Wiring Test table above is the contract each child must satisfy; no test is written by this umbrella
   - Files: this spec, plus the four child specs authored by sibling sessions
   - Verify: every child spec exists, names its position in the merge order, cites this umbrella, and carries the owner decisions D1-D4 as inherited constraints
2. **Phase: Child 1 (extraction)** -- lands alone, first, per D1
   - Tests: the child-1 rows in the Unit Tests table, including the five register rows (D6)
   - Files: `scripts/dev/rfc_requirements.py`, `scripts/dev/rfc_requirements_test.py`, `rfc/extraction/` (the artifact format plus the rfc7296 pilot), `rfc/drain-budget.txt`, `ai/RFC-REQUIREMENTS.md`, `rfc/short/rfc7296.md`
   - Verify: `make ze-verify` green on an unmodified tree (AC-1); a synthetic new enrolment reds (AC-2); a deleted ledger reds with a named error (AC-4); a weak register counts toward the floor while rendering in its own column (AC-15, AC-16); an unknown register is a hard error (AC-17); and the drain floor computes to 0 at the shipped rate while the backlog is still published, with `check_drain_floor` wired into `run_check` (AC-13, child 1 AC-27 to AC-30)
   - D7 sub-phase, ordered within child 1: extract the two confirmed `rfc7296` MUSTs, then prove each with tagged tests in both polarities. If the producing daemon behaviour turns out to be absent, STOP and escalate to Thomas under `ai/rules/rfc-compliance.md:38-51`; do not mint a `{gap}` and do not close the child (AC-18, A-8, R-9)
3. **Phase: Child 2 (evidence)** -- begins only after child 1 is committed
   - Tests: the three child-2 rows in the Unit Tests table
   - Files: `scripts/dev/rfc_requirements.py`, `test/interop/run.py`, `test/interop/run_test.py`, a nightly workflow
   - Verify: both scanners accept identical extension sets (A-5); the interop runner exits non-zero without Docker (AC-6); interop evidence is reported as a distinct class (AC-7)
4. **Phase: Child 3 (audit teeth)** -- begins only after child 2 is committed
   - Tests: the three child-3 rows in the Unit Tests table
   - Files: `scripts/dev/rfc_requirements.py`, `ai/skills/ze-rfc-audit.md`
   - Verify: a drifted verdict word is a parse error (AC-8); an empty `tests` map is rejected (AC-9); a `weak` verdict removes proven status (AC-10)
5. **Phase: Child 4 (ledger)** -- begins only after child 3 is committed; follows the ordered steps 4a to 4d in "Sequencing Constraint" and never reorders them
   - Tests: the child-4 rows in the Unit Tests table
   - Files: `scripts/dev/rfc_requirements.py`, `docs/features/rfc-status.md`, `rfc/full/rfc3765.txt`, `rfc/full/rfc4486.txt`, the four `rfc/short/` summaries, `rfc/enrolled.txt`
   - Verify (in this order): the two source texts are present (4a); the four summaries capture their obligations and are enrolled (4b, AC-19); `check_unproven_support` is armed in the same commit as 4b or later and is green on the tree it lands in (4c, AC-20, R-8); the 32 rows exist before `check_status_agreement` becomes unconditional (4d, AC-12); a zero-capture public support claim reds, `Experimental` included (AC-11)
6. **Phase: Umbrella closure** -- after child 4 is committed
   - Tests: `make ze-verify` on an unmodified tree
   - Files: this spec, `plan/spec-followup-rfc-enrollment.md` handed forward for rewrite
   - Verify: AC-1 and AC-14 hold; the published backlog is non-zero and ranked; the drain rate is still 0 and the arming commit is named as fleet work

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every blind spot in the six-row table is owned by exactly one child, and no child owns a blind spot outside its stated scope |
| Feature completeness | The published backlog is derived from committed data end to end; no step in the chain is hand-authored (`ai/rules/derive-not-hardcode.md`) |
| Correctness | Every new check fails closed. Specifically: a missing ledger, a zero source-keyword count, an empty audit `tests` map, and a missing product-ledger row each produce an error rather than a permissive default |
| Naming | The sign-off artifact's fields match the vocabulary already in `rfc_requirements.py` (`captured`, `source`, requirement `level`, `register`), so a reader does not learn a second vocabulary for one concept. Separately: `rfc/drain-budget.txt` is named for what it holds. If it ever acquires a per-stem row it has become the hand-kept registry the 2026-07-29 resolution rejected, and the name is then the least of the problems |
| Data flow | No child adds a second entry point to the gate. Everything enters through `ze-rfc-check` and the existing dispatch |
| Rule: `ai/rules/rfc-compliance.md` | No child re-derives an annotation, and no child classifies a requirement downward. Any finding that would lower what Ze owes is escalated to Thomas, never resolved in the child (`:38-51`). D7 and D8 move in the opposite direction and are therefore permitted without asking: they ADD requirement rows. Check the direction, not the fact of the edit |
| Rule: `ai/rules/no-parking.md` | A blind spot discovered during a child's implementation is fixed or escalated, never recorded and passed on. The one thing this set may legitimately record is the backlog, and only because publishing it is the deliverable. A CONFIRMED unextracted MUST is never part of that backlog (D7) |
| D6 both halves | The register counts toward the floor AND renders in its own column. Grep the child's diff for any rendered total that sums registers; one bare "signed off" number defeats the publishing half while the counting half still passes its tests |
| R-9: no new annotation absorbs a new obligation | For every requirement row child 1 or child 4 ADDS, confirm it carries tagged tests or a recorded escalation. A fresh `{gap}` over a newly extracted MUST is a compliance loss dressed as a green gate, and no gate in this set can detect it |
| R-8: arming order | For child 4, confirm from `git log` that the commit introducing `check_unproven_support` is at or after the commit resolving the four stems. A red between them leaves `make ze-verify` non-green and so blocks every commit in the repository through the verify-status gate. Do NOT accept "`--unverified` was available" as an answer: the bypass exists (`ze-rfc-check` is not a structural gate) but is illegal for a deterministic reproducible red (`ai/rules/fix-dont-record.md`) |
| Rule: `ai/rules/fail-closed-guards.md` | Each new guard is driven from its entry point (`make ze-rfc-check`) in at least one test, not only from the helper it lives in |
| Sequencing | Each child's commit lands with the previous child already on HEAD; check `git log` order before claiming AC-14 |
| Sequencing: child 1 before child 4's enrolments | Confirm from `git log` that child 1's extraction bar is on HEAD before the first of child 4's four D8 enrolment commits, and that each of the four carries an `rfc/extraction/<stem>.json`. This is the one ordering error the final tree cannot reveal: an enrolment landing before the bar exists is grandfathered out of it permanently, silently, with a green gate. See "Why child 4 must follow child 1" |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The four child specs exist and cite this umbrella | `ls plan/spec-rfcgate-*.md` returns five files; grep each for `spec-rfcgate-0-umbrella` |
| Extraction bound blocks a new enrolment | Add a synthetic stem to `rfc/enrolled.txt`, run `make ze-rfc-check`, observe the named failure, revert |
| Backlog is published and ranked | grep `ai/RFC-REQUIREMENTS.md` for the extraction section and confirm the count matches enrolled minus the stems carrying a valid, non-stale sign-off artifact |
| The quota reads a derived record, not a hand-kept one | `ls rfc/` shows no per-stem ledger file; `rfc/drain-budget.txt` contains exactly two fields and names no RFC; the signed count in ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Command correction" under "Where the counter lives") equals `ls rfc/extraction/*.json \| wc -l` minus any stale ones |
| Registers are published separately (D6) | grep `ai/RFC-REQUIREMENTS.md` for the three register columns, and confirm no summed "signed off" figure appears in the ledger or in any gate message |
| `rfc7296`'s two confirmed MUSTs are captured and proven (D7) | grep `rfc/short/rfc7296.md` for both obligations, then grep the tree for their `RFC requirement:` tags in both polarities; captured MUST-level rows rise from 18 |
| The four D8 stems are extracted and enrolled | `rfc/full/rfc3765.txt` and `rfc/full/rfc4486.txt` exist; each of the four summaries captures a non-zero MUST-level count; all four appear in `rfc/enrolled.txt` |
| `check_unproven_support` armed in the right order | `git log` shows the arming commit at or after the commit that resolves the four stems, and `make ze-rfc-check` is green on the arming commit itself |
| Drain floor self-retires | `TestDrainFloor.test_drain_floor_caps_at_backlog_size` passes, and `grep -n "check_drain_floor" scripts/dev/rfc_requirements.py` shows a definition AND a call inside `run_check` |
| Interop runner fails closed | Run `python3 test/interop/run.py` with Docker unavailable, observe non-zero exit |
| Audit verdict has a reader | grep `scripts/dev/rfc_requirements.py` for a read of the verdict field; it must return a hit after child 3 |
| Public ledger has no unbacked support claim | `make ze-rfc-check` green after child 4, with all four known rows resolved (RFC 1035, 3765, 4486 at `Supported`, RFC 5301 at `Experimental`) |
| Whole set green | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `rfc/drain-budget.txt`, `rfc/extraction/*.json` and `rfc/audit/*.json` are parsed from the repository. Malformed content must produce a named parse error, never a silent empty result that reads as "nothing owed" |
| Trust boundary | The gate reads committed files only and executes nothing from them. No child may add code that executes content read out of `rfc/` |
| Denial of the guard | The drain clock must not be overridable by flag or environment variable, or the forcing function is silenceable by the party it constrains |
| Resource exhaustion | `scan_tree` walks the whole repository. Child 2 widens its accepted extensions; confirm the walk does not begin reading large binary artefacts under the interop trees |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation or import error | Fix in the child that introduced it |
| Test fails for the wrong reason | Fix the test assertion or fixture |
| Test fails on behavior mismatch | Re-read the producing function named in the blind-spot table; if the cited behaviour was misread, back to RESEARCH and correct this spec |
| A child's check reds the tree for pre-existing backlog | R-5: grandfathering was not applied. Back to DESIGN for that child; do not weaken the check |
| A child needs to re-derive an EXISTING annotation to proceed | It has left its scope (D4). Stop, record the finding, and escalate; do not open the backlog |
| A child CONFIRMS an unextracted MUST while doing something else | D7: found means owed. Extract it in that child. Do NOT file it, and do not treat "pre-existing" or "grandfathered" as cover. Going looking across the 166 is still out of scope; acting on what you have already read is not |
| A newly extracted MUST cannot be proven without a daemon behaviour change | STOP and ask Thomas which way to fix it (`ai/rules/rfc-compliance.md:38-51`), keep the child open, and never absorb it into a `{gap}` (R-9) |
| A finding would lower what Ze owes on an RFC | STOP and ask Thomas (`ai/rules/rfc-compliance.md:38-51`). Never resolve it inside a child |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **The gate is honest about everything except its own denominator.** All four
  ratchets, the tag scanner, the ledger cross-check and the audit freshness
  comparison are well built and fail in the right direction. Every one of the six
  blind spots is a boundary the gate never claimed to police, which is exactly why
  they survived: nothing in the code is wrong, and reading the code carefully
  produces confidence rather than suspicion. The defect is in the boundary between
  what the gate measures and what a reader assumes it measures.
- **`source_keyword_count` already exists and is already correct.** It was written,
  wired into `check_enrolment` and `check_new_summaries`, and rendered into the
  ledger, and in all three places it is used only for presence or for the zero case.
  The comparison that would bound extraction is one line that was never written.
  This is the cheapest of the six fixes and the highest leverage.
- **The `verdict` field is the sharpest edge.** `/ze-rfc-audit` exists specifically
  to detect a test that is tagged and green but cannot fail on non-compliance, and
  it records that judgement as `weak`. Nothing reads it. The audit is doing the
  hardest part of the work correctly and then writing the answer to a field with no
  consumer.
- **Two runners disagreeing about the same condition is the tell.** `test/interop/run.py`
  exits 0 without Docker and `test/ipsec-interop/run.py` exits 1. One of them is
  wrong, and the disagreement is what makes it obvious which.
- **"Per release" was a design question in disguise.** D2 asked for N recertified per
  release; this repository has no releases at all. Discovering that changed the
  design from a release hook to a calendar budget, and forced the rate-ships-at-zero
  decision that keeps the forcing function from becoming the kind of rule that gets
  deleted rather than obeyed. D5 confirmed the whole shape unchanged.
- **The quota's hardest problem was not the rate, it was the unit of account.** A
  sign-off over `rfc2328` and a sign-off over `rfc7296` cost different things and
  prove different things, because OSPFv2 contains no capitalised keyword to check
  against and IKEv2 contains 310. D6 refuses the two obvious answers, which are to
  exclude the weak ones (leaving a third of the corpus undrainable) and to merge
  them (making the total mean less than it reads). Counting them while publishing
  them apart is the only answer that is honest in both directions, and it is the
  same discipline D3 already applies to interop evidence: admit it, never merge it.
- **The grandfather has a natural boundary, and reading is what crosses it.** D2's
  grandfathering exists because a rule that reds on unrelated work gets deleted. But
  once somebody has READ an obligation and confirmed it, it is no longer unrelated
  work: `ai/rules/rfc-compliance.md:28` and `ai/rules/no-parking.md` together make
  the finder the entry point. D7 names that boundary, which means the grandfather
  now shrinks by itself every time anyone looks closely, rather than needing a
  separate campaign to dismantle it.
- **A verify-resident gate is the one kind of check that can trap its own revert.**
  The arming-order constraint is not bureaucratic caution: `ze-rfc-check` sits
  inside `make ze-verify`, so its red leaves the tree non-green, and
  `commit_helper.py` then refuses every commit script in the repository, the fix
  and the revert included. A new check must be green in the commit that introduces
  it. That was true of the other checks in this set too; child 4 is simply the
  first one whose red set was known in advance and non-empty.
  ~~The trap holds because a structural red is unbypassable.~~ **Corrected
  2026-07-29, and the correction is the more interesting half.** `ze-rfc-check` is
  NOT one of the eight names in `STRUCTURAL_GATES`
  (`scripts/dev/commit_helper.py:512-523`), so a bypass technically exists. The
  trap holds anyway, which is the lesson worth carrying: a gate does not need to be
  unbypassable to be untouchable. `--unverified` is legitimate only for a flaky or
  environmental red or one already logged, `ai/rules/fix-dont-record.md` forbids
  logging a deterministic reproducible failure, and a red on four named stems is as
  deterministic as a red gets. Reaching for the bypass therefore means breaking a
  rule or asking Thomas on every commit until the four are cleared. Designing an
  ordering around "what is enforced" rather than "what is legal" would have got the
  right answer here by luck; it is the legality that binds.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Four serialised children over one large spec | One spec covering all six blind spots; two specs split by file | D1 mandates extraction as a separate first program. Beyond that, all four change one Python module and every ratchet reads git HEAD, so concurrency produces silently mis-calibrated floors, not just merge conflicts. Serialising costs cycles and buys correctness that a merge tool cannot supply |
| Extraction first over evidence first | Evidence first, because interop is the most visible gap | Every other child's thresholds are denominated in the requirement set. Landing an evidence floor before extraction bakes a pre-extraction denominator into a ratchet, and the resulting floor is too low in a way that still shows green |
| Calendar-time drain budget over a release hook | Tie the drain to `ze-release-evidence` (`mk/test-release.mk:83`); tie it to git tags; tie it to a commit count | The repository has no releases: no tags, no VERSION file, no version constant. `ze-release-evidence` is unscheduled, so it can force nothing. Calendar time is the only counter that advances without anyone choosing to advance it |
| Rate ships at 0, armed later | Ship a guessed non-zero rate; ship publish-only with no rate at all | A guessed rate is either inert or reds the tree on unrelated work, and `ai/rules/rfc-compliance.md:114-116` records that the second kind gets deleted rather than obeyed. Shipping the machinery inert preserves the forcing function's existence while deferring only its calibration, which is the one part that genuinely needs measured data |
| The quota's record IS the `rfc/extraction/<stem>.json` set; only the policy (start date, rate) is authored, in `rfc/drain-budget.txt` (resolved 2026-07-29) | (a) a hand-authored `rfc/recertified.txt` with `stem`/`date`/`captured`/`source`/`register` rows, this umbrella's original design; (b) `rfc/recertified.txt` as a rendered view of `rfc/extraction/`; (c) the two policy fields inside an existing file | (a) is a second hand-kept registry of who has been signed off, which `ai/rules/derive-not-hardcode.md` forbids and which the 2026-07-20 ruling in `plan/deferrals/rfc-gate-regression-ratchets.md` already refused as declare-instead-of-prove. It is also strictly weaker than child 1's artifact: a count comparison for staleness where the artifact pins a source sha, and no per-site classification at all. (b) is redundant with two things that already publish the same numbers. (c) fails on `rfc/enrolled.txt`, whose `line.split()[0]` parser cannot carry structure, and scatters one global setting across 166 files if put in the artifacts. A bare stem-and-date row was also considered and is R-1 exactly: a signature with nothing to falsify it |
| `.ci` bindings preferred over interop bindings | Treat all evidence as equal once admitted | D3. `.ci` runs inside `ze-verify` on every push; interop runs nowhere today and nightly-advisory at best after child 2. Merging them into one total would raise the reported proof without raising the real proof, which is R-3 |
| Weak registers count toward the quota but publish in their own column (D6) | Exclude `prose` and `manual-walk` from the quota so only keyword-verified sign-offs count; count them and merge all three into one total | Excluding them makes 53 of 166 entries permanently undrainable (the site denominator, which is the one the register rule uses; see "The three pre-2119 measurements"), and an unsatisfiable quota is deleted rather than obeyed (`ai/rules/rfc-compliance.md:114-116`). Merging them rebuilds this spec set's own defect one layer up: a number whose meaning is narrower than it reads. Taking both halves is the only combination that is neither unsatisfiable nor misleading |
| A confirmed unextracted MUST escapes the grandfather (D7) | Leave `rfc7296` to the fleet drain like every other entry; file the two MUSTs as backlog rows | The grandfather covers what nobody has looked at. Both MUSTs were READ and confirmed in this design session, and `ai/rules/rfc-compliance.md:28` says an unextracted obligation is still an obligation while `ai/rules/no-parking.md` makes the finder the entry point. Filing them would be the recording-instead-of-fixing move both rules ban |
| The four D8 stems are extracted THEN enrolled, both inside child 4 | Enrol them now and extract later; split extraction into child 1 and enrolment into child 4 | Enrolling a zero-capture summary changes nothing the gate measures, so enrolment-first buys a bigger enrolled set and no more proof. Splitting across children breaks the serialisation: the stems must clear child 1's extraction bar at the moment they enrol, and that bar does not exist until child 1 has landed |
| Supersede `plan/spec-followup-rfc-enrollment.md` rather than delete or absorb it | Delete it as stale; fold its content into this umbrella; rewrite it now | Its numbers are stale but its ownership claim over the drain is still correct, and D4 puts the drain out of this set's scope. Rewriting it now would re-derive a ranking that child 1 is about to make derivable, which is the hardcoding the rules forbid |

## Known Limitations

- **The set builds the instrument and does not take the measurement.** After all
  four children land, the backlog is bounded, published and ranked, and it is still
  a backlog. 1746 requirements remain not-both-polarity-proven and 1376 still rest
  on annotations `ai/rules/rfc-compliance.md:53` voids as authority. That is D4 and
  it is deliberate, but a reader of the green gate after this set must not read it
  as compliance achieved. The D7 and D8 carve-outs are narrow on purpose: they
  close the obligations this design session actually CONFIRMED by reading, and they
  are not a licence to start the drain early.
- **The drain has no force until the arming commit.** R-2, now an accepted risk
  under D5 rather than an open one. Publish-only is the landing state, and Thomas
  owns the arming commit.
- **A third of the corpus gets a weaker bound than the rest, and says so.** 53 of
  166 enrolled entries cannot take the `rfc2119` grade under child 1's derivation
  rule, so their sign-off is graded `prose` or `manual-walk`: a human assertion
  where the others get a mechanical comparison. (53 is the site-denominator
  figure and the operative one here; 22 and 33 measure the same problem at the
  keyword-occurrence denominator and are equally correct. See "The three pre-2119
  measurements".) D6 settles the trade deliberately in both directions. The
  weaker grades COUNT, because a quota that excluded a third of the corpus would be
  undrainable and would be deleted rather than obeyed. And they are PUBLISHED
  separately, because a merged total would mean something narrower than it reads,
  which is the exact defect this whole set exists to correct. The limitation is
  real; what changed is that it is now visible in the ledger instead of buried.
- **The extraction bound counts keyword occurrences, not distinct obligations.**
  A-1. It is an upper bound on the deficit, not a measurement of it, and a summary
  can satisfy the ratio while still missing a specific obligation. Extraction
  Completeness remains a human walk (`ai/rules/rfc-compliance.md:73-98`); this set
  makes skipping it visible, not impossible.

Anything above that is genuinely outstanding work has its row in
`plan/deferrals/rfcgate-0-umbrella.md`, with `plan/spec-followup-rfc-enrollment.md`
as the destination for the drain.

## RFC Documentation (Scope: protocol)

Largely not applicable as protocol documentation: this set changes no protocol
behaviour and adds no enforcing code, so there is no MUST to quote above a new
branch. It changes how the repository PROVES its RFC obligations, which is
documented in `ai/rules/rfc-compliance.md` beside the four existing ratchets, and in
`ai/rules/hook-mapping.md` so a future agent can find what rejected an edit.

Two exceptions, both from D7 and D8, and both DOCUMENTARY rather than behavioural:

| What | Where it is documented |
|------|------------------------|
| The two confirmed `rfc7296` MUSTs (D7) | `rfc/short/rfc7296.md` gains a checklist row each, and the enforcing branch each is proven against gains the RFC section quoted inline above it, in the form `internal/component/bgp/message/header.go` already uses (`:63-89`). If no enforcing branch exists, the finding is escalated, not documented away |
| The four D8 stems | `rfc/short/<stem>.md` gains a real checklist, and `docs/features/rfc-status.md` gains a coverage claim that now has captured requirements behind it. The support level itself does not rise: the claim was already published, and D8 makes it backed rather than merely unmeasured |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-21 (plus AC-4a) all demonstrated
- [ ] Child 1 on HEAD before child 4's first D8 enrolment, and four extraction
      sign-offs landed with the four enrolments (AC-21, and the ledger spec's OC-6)
- [ ] D6 both halves hold: weak registers counted, three registers published separately, no summed total anywhere
- [ ] D7 discharged: both confirmed `rfc7296` MUSTs captured and proven, or escalated to Thomas with the spec left open
- [ ] D8 discharged: the four stems extracted then enrolled, with both missing source texts fetched
- [ ] `check_unproven_support` armed no earlier than the commit resolving the four stems
- [ ] Wiring Test table complete: every row a concrete test name, none outstanding
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] All four children landed, each alone and green, in the order 1, 2, 3, 4
- [ ] Backlog published in `ai/RFC-REQUIREMENTS.md`, derived and non-zero
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] `plan/spec-followup-rfc-enrollment.md` handed forward for rewrite, not deleted

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior (`.py` driving surfaces; `.ci` is N-A for gate tooling)
- [ ] Interop tests for protocol features (N-A: this set changes no protocol behaviour)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-rfcgate-0-umbrella.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-rfcgate-0-umbrella.md` only (commit A preserves the spec in history)
