# Spec: rfcgate-1-extraction

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | ~~tooling (phases 1 to 7) + protocol (phase 8, the rfc7296 pilot)~~ (superseded 2026-07-29 by owner ruling 3) **tooling only: phases 1 to 7** |
| Depends | - |
| Phase | ~~7/8 (phases 1-7 implemented and green; phase 8 NOT started)~~ (superseded 2026-07-29 by owner ruling 3) **7/7 -- phases 1 to 7 complete and green; phase 8 re-homed to `plan/spec-rfcgate-1b-rfc7296-pilot.md`** |
| Deferral shard | `plan/deferrals/rfcgate-1-extraction.md` |
| Updated | 2026-07-29 |

Umbrella: `plan/spec-rfcgate-0-umbrella.md`. This is program ONE of the set, the highest
priority child, and the first of four that the umbrella's "Sequencing Constraint" requires
to be merged strictly serially (1, then 2, then 3, then 4). ~~It builds the machinery, plus
ONE pilot sign-off:~~ ~~performing the 166 sign-offs is fleet work owned by the umbrella and
by `plan/spec-followup-rfc-enrollment.md`~~ (superseded 2026-07-29 by owner ruling 2, below)
~~performing the other 165 sign-offs is fleet work owned by the umbrella and by
`plan/spec-followup-rfc-enrollment.md`; rfc7296 is performed HERE and is not grandfathered.~~

**Superseded again 2026-07-29 by owner ruling 3 (below).** This spec builds the MACHINERY
and nothing else. The rfc7296 pilot is not abandoned and not grandfathered: it moved intact
to `plan/spec-rfcgate-1b-rfc7296-pilot.md`, which carries all 214 walked obligations and
this spec's AC-23 to AC-26. Performing the other 165 sign-offs remains fleet work owned by
the umbrella and by `plan/spec-followup-rfc-enrollment.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** `make ze-rfc-check` is green over 2720 gated MUST-level requirements
across 166 enrolled RFCs. Every one of those requirements was audited against the tests
that prove it. Nothing anywhere bounds what the summary MISSED. The gate audits the
requirements a summary LISTS; an obligation nobody wrote down is invisible to it, to the
four ratchets, and to `/ze-rfc-audit`, which re-checks classifications of listed rows.

`ai/rules/rfc-compliance.md:73-78` states this in the rule itself: "It cannot know about
an obligation nobody wrote down. A green gate is bounded by what was extracted, so a
missing extraction is invisible to it and to any audit that only re-checks
classifications." Today that boundedness is enforced by nothing at all.

**The mechanism.** `source_keyword_count` (`scripts/dev/rfc_requirements.py:1323`) is the
only function that reads the RFC's own text. It has exactly three consumers:

| Consumer | Line | What it does with the count |
|----------|------|------------------------------|
| `check_enrolment` | `:676` | presence only (`is None`): fails when the source file is absent |
| `check_new_summaries` | `:1113` | fires only when captured == 0 AND source keywords > 0 |
| `unconverted_summaries` | `:1353` | renders the zero-capture table in the ledger |

Once a summary captures one gated MUST and is enrolled, the RFC's own text is never
consulted again for the rest of the summary's life.

**The size of the hole.** Re-derived 2026-07-29 over the 166 enrolled RFCs by splitting
`rfc/full/<stem>.txt` into normative sentences (containing MUST / MUST NOT / SHALL /
SHALL NOT / REQUIRED, excluding RFC 2119 boilerplate): 3988 distinct normative sentences
against 2720 captured requirements, ratio 0.68. 61 of 166 RFCs have more normative
sentences than captured requirements; raw deficit 1683. The 20 RFCs with ratio below 0.30
hold 1315 of that deficit (78%). Worst offenders: rfc7296 (263 sites vs 18 captured),
rfc7950 (224 vs 9), rfc8907 (101 vs 11), rfc3748 (101 vs 20), rfc2661 (100 vs 18),
rfc4301 (89 vs 15), rfc5176 (78 vs 5), rfc5036 (77 vs 14), rfc7011 (76 vs 19), rfc2865
(70 vs 10). Estimated genuine unextracted obligations after false-positive calibration:
1200-1500.

**Two confirmed individual instances.** RFC 7296 Section 2.2, "Use of Sequence Numbers for
Message ID" (`rfc/full/rfc7296.txt:1392`), carries "Retransmission of a message MUST use the
same Message ID as the original message" (`:1397`) and "In the unlikely event that Message
IDs grow too large to fit in 32 bits, the IKE SA MUST be closed or rekeyed" (`:1439`).
Neither appears in any row of `rfc/short/rfc7296.md`: the summary anchors ids to fourteen
sections (1.2, 1.3.3, 1.4, 2.1, 2.4, 2.6, 2.7, 2.8, 2.9, 2.23, 3.3, 3.3.2, 3.3.6, 3.8) out
of 92 numbered headings in the source, and 2.2 is not among them. Re-verified 2026-07-29:
`grep -c "RFC7296-2\.2-" rfc/short/rfc7296.md` returns 0. RFC 4271 Section 8.2.2, the
Finite State Machine, has zero requirement ids: `rfc/short/rfc4271.md` allocates
`RFC4271-8.2.1-*` and `-8.2.1.4-1` and nothing under 8.2.2.

These are no longer cited only as evidence. Owner ruling 2 (below) took the two RFC 7296
obligations OUT of the grandfather and gave them to this spec, which now performs the
rfc7296 sign-off as its pilot. The RFC 4271 case stays evidence: it names an unanchored
section, not a confirmed unextracted obligation, and the section's recall problem is what
`unsourced-ids` exists to express (R-7).

**The goal.** Bound the gate's blind spot with a per-RFC extraction sign-off that a
machine can re-check, gate a NEW enrolment on it, grandfather the remaining 165 as an
explicitly counted and published backlog, and ratchet the signed-off count so it can only
rise. The umbrella owns the drain POLICY that turns the ratchet into a forcing function
(the start date, the rate, the semantics of the quota, and the acceptance criterion over
it); this spec builds and publishes the per-register numbers that policy is read against,
AND implements the floor COMPARISON that reads them (`check_drain_floor`, AC-27 to AC-30,
resolved 2026-07-29). It
then performs ONE sign-off, rfc7296, which owner ruling 2 removed from the grandfather:
that sign-off is the pilot proving the artifact format against the worst-measured input in
the corpus, and it discharges two obligations the gate is confirmed to be blind to today.

**Non-goals.** Performing any of the other 165 sign-offs. Authoring the drain POLICY: the
start date, the rate, the cadence semantics and the value of N belong to the umbrella and
ultimately to Thomas, and no phase here invents one. ~~Designing the release cadence or
the value of N.~~ (Restated 2026-07-29: unchanged in substance, but the old wording was
being read as excluding the floor COMPARISON as well, which this spec DOES implement. See
AC-27 to AC-30 and "Grandfathering, ratchet, and the drain interface".) Changing the
requirement-to-test coverage rules, the polarity rule, the annotation kinds, or any of the
four existing ratchets.

## Owner Rulings (settled 2026-07-29)

Both questions this spec raised at its design gate were put to Thomas and answered. They
are DECISIONS here, not options: no later phase may reopen either without a fresh ruling.

### Ruling 1: every register earns drain credit, and every register is published apart

| Field | Value |
|-------|-------|
| Question raised | Does a `prose` or `manual-walk` sign-off count toward the umbrella's per-release drain quota, or only an `rfc2119` one? |
| Ruling | It counts, exactly as an `rfc2119` sign-off does |
| Rationale accepted | A large minority of the enrolled corpus is not in the RFC 2119 register, so excluding the weaker registers would leave it permanently undrainable: a backlog with no exit rather than a ratchet. Three measurements of that minority are in play, at three different source-side denominators, and **all three are correct; none corrects another.** `plan/spec-rfcgate-0-umbrella.md` "The three pre-2119 measurements" is the canonical statement and this row references it rather than re-deriving it: **22 of 166** have zero uppercase MUST-level keyword occurrences anywhere in the source while declaring 164 gated MUSTs between them (this spec's A-3, list unchanged, including `rfc2328` OSPFv2, `rfc792`, `rfc905`, `rfc1350` and `sflow-v5`); **33 of 166** declare more gated rows than their source has uppercase occurrences; **53 of 166** declare more gated requirements than the boilerplate-excluded SITE scan finds sites for (this spec's A-2), which is the denominator this spec's register rule operates on. `rfc/short/rfc2181.md` is the sharpest case at either occurrence denominator: 23 gated MUSTs against ONE uppercase occurrence, in a memo whose own Section 3 says it does not use the 2119 expressions. Every figure supports the ruling |
| Mandatory counterweight (hard AC, not prose) | The ledger and the status envelope publish EACH register in its own column. A signed count is never rendered without its register split beside it, so "N signed off" can never be misread as "N keyword-verified" |
| Mechanised by | AC-11 (credit + own column), AC-21 (ledger columns), AC-22 (envelope split), AC-16 (envelope counts) |
| Named tests | `TestExtractionStatus.test_every_register_counts_toward_the_signed_total`, `TestExtractionLedger.test_registers_are_published_in_separate_columns`, `TestExtractionStatus.test_signed_by_register_sums_to_total` |
| Consequence for A-5 | The `manual-walk` register is retained whatever the live count turns out to be. Dropping it would leave an rfc1877-shaped RFC with no route to a sign-off at all, and the ruling forbids an undrainable remainder |

### Ruling 2: rfc7296 is re-authored NOW, outside the grandfather, and this spec owns it

| Field | Value |
|-------|-------|
| Question raised | Two CONFIRMED unextracted MUSTs sit in an RFC enrolled before HEAD. The grandfather clause says pre-HEAD summaries are backlog. Which wins? |
| Ruling | The rules win. A CONFIRMED unextracted MUST escapes the grandfather by definition, and this spec performs the work |
| Rationale accepted | `ai/rules/rfc-compliance.md:28`: "An unextracted obligation is still an obligation. Add the checklist row -- the gate's silence is not conformance." `ai/rules/no-parking.md`: the session that found it is the entry point that must fix it. Grandfathering bounds what a NEW MECHANISM may red on; it was never a licence to leave a named, verified obligation unextracted once someone has read it |
| Scope taken | Re-author `rfc/short/rfc7296.md` against `rfc/full/rfc7296.txt` per `ai/skills/ze-rfc.md`, extract at minimum `RFC7296-2.2-1` and `RFC7296-2.2-2`, produce `rfc/extraction/rfc7296.json` in this spec's format, and implement-and-prove every newly extracted requirement or escalate it |
| Mechanised by | Implementation phase 8, AC-23 through AC-26, the escalation rule in "The rfc7296 pilot" under Design, and the Failure Routing row that forbids any other outcome |
| Deliberately NOT taken | The other 165 stay grandfathered. The ruling turns on the obligations being CONFIRMED, and no equivalent confirmation exists for the rest. RFC 4271 Section 8.2.2 is an unanchored section, not a confirmed unextracted obligation, and stays evidence |

**What ruling 2 costs, stated honestly.** rfc7296 has the worst capture ratio measured in
the corpus: 263 derived sites against 18 captured gated requirements, corroborated
2026-07-29 at 271 normative sentences and 311 uppercase MUST-level keyword occurrences by
an independent split, over 92 numbered sections of which 14 carry ids. The walk will
surface an unknown number of new obligations, and each one is owed working code plus a
positive and a negative tagged test. That tail cannot be sized before the walk runs and
must be scoped with the owner once the real count is known (R-10). Pretending the number
is knowable now would be the fabrication this spec exists to remove.

### Ruling 3: the rfc7296 pilot becomes its own spec, and all 108 unimplemented MUSTs are fixed there

| Field | Value |
|-------|-------|
| Question raised | Phase 8a ran the walk exactly as ruling 2 and the Failure Routing table require ("Phase 8a's count is larger than this spec can carry -> STOP and take the COUNT to Thomas for scoping"). The count came back at **214 distinct obligations** against the 18 gated MUST rows the summary holds today, triaged 63 `implemented-and-testable`, 25 `implemented-untested`, **108 NOT IMPLEMENTED**, 18 `uncertain`. What happens to 108 unimplemented IKEv2 MUSTs, and does the machinery set wait for them? |
| Ruling (Thomas, 2026-07-29, two halves that compose) | **OR-A: fix all 108 inside the spec that owns them.** Not annotated, not deferred, not written off. **OR-B: phase 8 becomes its own spec**, so the rfcgate machinery set is not serialized behind an IKEv2 compliance workstream |
| Rationale accepted | The two halves answer two different questions and neither weakens the other. OR-A keeps `ai/rules/rfc-compliance.md:53` intact: `{gap}` / `{not-applicable}` / `partial` are void as authority, and 108 confirmed unextracted obligations escape the grandfather by definition (umbrella D7, "found means owed"). OR-B is a change of HOME, not of scope: serializing four machinery children behind an IKEv2 workstream of unknown length is what would actually have reduced delivery, and the debt is paid in full in the destination spec |
| Scope taken OUT of this spec | Implementation phase 8 in full (8a to 8e), AC-23 to AC-26, the four `TestRealTree.test_rfc7296_*` rows of the TDD plan, and the rfc7296 row of the Wiring Test table. Nothing under `rfc/short/rfc7296.md`, `rfc/extraction/rfc7296.json`, `rfc/enrolled.txt`, `docs/features/rfc-status.md` or `internal/component/ike/` was touched by this spec |
| Where it went | `plan/spec-rfcgate-1b-rfc7296-pilot.md` (Status `design`, `Depends \| spec-rfcgate-4-ledger`, so the extraction bar is on HEAD when its sign-off artifact is judged). It carries all 214 obligations in its Appendix A and maps this spec's ACs one-for-one: AC-23 -> its AC-1, AC-24 -> its AC-5, AC-25 -> its AC-4, AC-26 -> its AC-3 (`plan/spec-rfcgate-1b-rfc7296-pilot.md:422-427`) |
| Recorded as a deferral | `plan/deferrals/rfcgate-1-extraction.md:8`, Status `deferred`, Destination `plan/spec-rfcgate-1b-rfc7296-pilot.md`. A live row with a real destination, which is what a change of home looks like; a terminal status here would have said the work was done |
| One narrowing the destination spec applied | AC-25's annotation branch does NOT survive the move. Its successor (`plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-4) removes the annotation escape entirely: the only surviving path to a non-proven row is Thomas's recorded answer to the STOP-and-ask escalation. The obligation got stricter in transit, never looser |

**What this spec's delivered scope therefore is: phases 1 to 7, and only those.** The
machinery -- site and section derivation, register derivation, the artifact parser and
skeleton writer, `check_extraction_signoff`, `check_extraction_ratchet`,
`check_drain_floor`, the `check_enrolment` precondition, the ledger table, the status
envelope, and the discovery surfaces. AC-1 to AC-22 and AC-27 to AC-32 are this spec's and
are audited below. AC-23 to AC-26 are struck in the Acceptance Criteria table and re-homed;
they are not dropped, and the strikethrough carries the destination so a reader can follow
them.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md` - "Extraction Completeness" (`:73-98`) and "the four
  ratchets" (`:100-131`) are the rule this spec mechanises and the table it extends.
  → Decision: extraction completeness is already BLOCKING policy at enrolment time; this
    spec adds the machinery, it does not widen the obligation.
  → Constraint (`:114-116`): "Summaries that predate HEAD are the existing backlog and are
    deliberately grandfathered: a rule that reds the gate on unrelated work gets removed
    rather than obeyed." Any design that reds 166 RFCs on day one is rejected on this line.
  → Constraint (`:64-71`): `rfc/short/*.md` are protocol-only reference documents and MUST
    NOT contain Ze-specific information. The sign-off record therefore cannot live there.
  → Constraint (`:94-98`): a misquoted requirement licenses a justification that never
    engages it. The design must make the source-sentence-to-requirement pair explicit so a
    reader can judge it; it must not claim to judge it mechanically.
- [ ] `ai/rules/fail-closed-guards.md` - the guard contract.
  → Constraint: "A guard that genuinely cannot deny MUST log, error, or fail its gate."
    The 22 enrolled RFCs whose source has no capitalised RFC 2119 keyword make any
    keyword-driven arithmetic vacuously green; the design must say so rather than pass.
  → Constraint (the zero-value trap): a zero site count must never read as a legitimate
    "nothing to extract".
- [ ] `ai/rules/derive-not-hardcode.md` - registry-derived display and inventory.
  → Decision: the site inventory is DERIVED from the source text at check time. Only the
    per-site CLASSIFICATION is authored. A hand-typed "sites seen" number is a claim, and
    claims are what this whole programme exists to remove.
- [ ] `plan/deferrals/rfc-gate-regression-ratchets.md` - the 2026-07-20 owner ruling.
  → Constraint: "mass-generating 164 audit files would record a verdict for an audit
    nobody performed, which is precisely the declare-instead-of-prove failure this whole
    programme exists to remove." The sign-off artifact must be structurally impossible to
    mass-generate into a green state.
- [ ] `plan/spec-followup-rfc-enrollment.md` - owns `rfc/enrolled.txt` and the
  Coverage-by-RFC rollup going forward.
  → Constraint: that spec grows enrolment; this one adds a precondition to growing it.
    The two must not both claim ownership of `rfc/enrolled.txt`; see "Interaction with
    spec-followup-rfc-enrollment" below.
- [ ] `ai/skills/ze-rfc.md:43-54` - the "Coverage self-check (BLOCKING)" the author is
  told to run by hand today (two greps, compare counts, judge by order of magnitude).
  → Decision: this spec replaces that honour-system count with a recorded artifact. The
    skill step becomes "run the skeleton writer, classify every site", not "eyeball a ratio".
  → Constraint (`:158-192`, "Requirement IDs"): the rfc7296 re-authoring may never renumber
    or reuse an id. A new obligation takes the next free ordinal IN ITS OWN section, so
    Section 2.2's first two ids are `RFC7296-2.2-1` and `RFC7296-2.2-2`.
  → Constraint (`:194-223`, "Annotations"): writing `{not-applicable}`, `{gap}` or
    `{single-polarity}` is Thomas's call, not the implementer's, and an annotation found
    already in place is VOID as authority. This is the rule the phase-8 escalation
    procedure implements.
- [ ] `ai/rules/no-parking.md` - recording a problem is not addressing it; the finder is
  the entry point.
  → Decision: this is half of the basis for owner ruling 2. A confirmed unextracted MUST
    may not be routed to a deferral row, a follow-up spec, or a known-failure shard.
  → Constraint: "reducing coverage to reach green" is banned, so phase 8 may not reach a
    green gate by narrowing what it extracts.
- [ ] `ai/rules/git-safety.md` - "Before Any Commit", Step 1: `commit_helper.py create`
  refuses to prepare a script over a non-FRESH verify, so any red inside `make ze-verify`
  blocks every commit in the repository.
  → Constraint: this is why the cross-child sequencing below is load-bearing.
    `plan/spec-rfcgate-4-ledger.md` arms a gate that is red by design on four stems, so it
    may arm only in the commit that clears them, or later. This spec is the opposite shape
    and must land green (AC-19, AC-24).
  → ~~Constraint: `ai/rules/git-safety.md:229-242` "Structural Gates Are Never Known-Red"
    applies, so the only escape is the owner-only `--structural-red-ok`.~~ **Corrected
    2026-07-29: it does not apply.** `STRUCTURAL_GATES`
    (`scripts/dev/commit_helper.py:512-523`) holds exactly eight stage names and
    `ze-rfc-check` is not one of them. The conclusion is unchanged; see the next entry for
    the rule that actually binds.
- [ ] `ai/rules/fix-dont-record.md` - "Recording": a `plan/known-failures/` shard is never a
  destination for a failure that reproduces.
  → Constraint: this is what makes the sibling's arming order binding rather than merely
    advisable. An early-armed `check_unproven_support` red IS `--unverified`-bypassable in
    the mechanism, but `--unverified` is legitimate only for a flaky or environmental red or
    one already logged, and a red naming four fixed stems is deterministic and reproducible,
    so logging it is banned. The only remaining route is an explicit owner ruling on every
    commit until the four are cleared.
- [ ] `plan/spec-rfcgate-0-umbrella.md` - "Sequencing Constraint" and "Drain Schedule
  Design (D2)".
  → Constraint: all four children edit `scripts/dev/rfc_requirements.py` and must be merged
    strictly serially, 1 then 2 then 3 then 4. Never two in flight.
  → Constraint (child-4 row, D8): child 4 extracts and enrols rfc1035, rfc3765, rfc4486 and
    rfc5301. From the day THIS spec lands, each of those four carries an extraction sign-off
    as a precondition of enrolment (AC-1), and two of them have no `rfc/full/` source text
    yet, which the umbrella already requires child 4 to fetch first.
  → ~~Open (recorded, not decided): the umbrella's child-1 row asks for
    `rfc/recertified.txt`.~~ **Decision (umbrella, 2026-07-29):** the
    `rfc/extraction/<stem>.json` set IS the record and the quota is derived from it; no
    per-stem ledger file exists. This spec additionally creates `rfc/drain-budget.txt`,
    which carries the drain POLICY (start date, rate) and nothing else. See "Interaction
    with the umbrella's recertification ledger" under Design, and the umbrella's "Where
    the counter lives".

### RFC Summaries (Scope: tooling for the machinery; rfc7296 is worked on directly)
- [ ] `rfc/short/rfc7296.md` - 23 checklist lines: 16 MUST, 2 MUST NOT (18 gated), 5 SHOULD.
  Anchors ids to 14 sections out of 92 numbered headings; Section 2.2 is not one of them.
  → Constraint: a summary can be internally perfect and still miss an entire section.
  → Decision (owner ruling 2): this file is RE-AUTHORED by phase 8, not merely cited. It is
    the only summary this spec edits.
  → Constraint (`check_retired_requirements`, `rfc_requirements.py:1007`): re-authoring may
    not delete any of the 23 existing ids. A misquote is corrected by editing the TEXT under
    the same id.
  → Constraint (`check_coverage_ratchet`): no existing requirement may lose a polarity it
    holds at HEAD. Re-authoring adds rows; it never demotes proof.
  → Constraint (`rfc/enrolled.txt:159`): the enrolment descriptor spells the current
    disposition (15 met, 1 single-polarity positive, 2 gap). Re-authoring changes those
    counts, so the descriptor and `docs/features/rfc-status.md:213` move with the summary.
- [ ] `rfc/full/rfc7296.txt:1392-1441` - Section 2.2, "Use of Sequence Numbers for Message
  ID", read in full for the two obligations.
  → Constraint: quote both verbatim in the new checklist rows. `:1397` "Retransmission of a
    message MUST use the same Message ID as the original message"; `:1439` "In the unlikely
    event that Message IDs grow too large to fit in 32 bits, the IKE SA MUST be closed or
    rekeyed".
  → Decision: the phase walks all 92 sections, not only 2.2. The two named obligations are
    the FLOOR of what it extracts, never the ceiling.
- [ ] `rfc/audit/` - contains exactly one file, `rfc7606.json` (checked 2026-07-29).
  → Decision: there is no `rfc/audit/rfc7296.json`, so re-authoring rfc7296's requirement
    TEXT stales no recorded audit verdict and `check_audit_freshness` cannot fire on it.
    One complication the pilot does not have to pay for.
- [ ] `rfc/short/rfc4271.md` - `:689-690` show Section 6.2 DOES have `-6.2-1` and `-6.2-2`;
  `:710-712` show Section 6.1 has per-subcode rows while `:713` has only an umbrella row.
  → Constraint: state the narrow true claim. Two earlier researcher claims about this file
    (RFC 7606 Section 3 items (e)/(f) missing; RFC 4271 Section 6.2 having no rows) were
    both false. Verify before citing.
- [ ] `rfc/short/rfc2181.md` - declares 23 gated MUST-level requirements from a source
  whose Section 3 says "This memo does not use the oft used expressions MUST, SHOULD, MAY,
  or their negative forms" (`rfc/full/rfc2181.txt`, sole uppercase occurrence).
  → Constraint: a correct summary can legitimately declare far more obligations than a
    capitalised-keyword scan can find sites for. The register model below exists for this.

**Key insights:** (minimal context to resume after compaction)
- The gate proves LISTED requirements are proven. Nothing proves the LIST is complete.
- The only source-text reader is `source_keyword_count`, and all three of its consumers
  are presence-only, zero-capture-only, or rendering.
- A keyword-derived inventory is the wrong oracle for 53 of the 166 enrolled RFCs.
- Grandfathering is not politeness, it is the documented condition under which a ratchet
  survives in this repo. It bounds what a NEW mechanism may red on. It has never been a
  licence to leave a CONFIRMED unextracted obligation unextracted (owner ruling 2).
- Every register earns drain credit; no count is ever published without its register beside
  it (owner ruling 1). Credit and evidence-strength are two different facts, and the design
  keeps them in two different columns rather than choosing between them.
- rfc7296 is the pilot: worst capture ratio in the corpus, so if the artifact format holds
  there it holds anywhere.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/rfc_requirements.py` - the whole gate, read in full. `run_check:1629`
  composes eight checks; `source_keyword_count:1323` is the only source-text reader.
- [ ] `scripts/dev/rfc_requirements.py:655` - `check_enrolment`: enrolment is monotonic,
  every enrolled stem needs a summary, and (`:676`) needs a source text to exist.
- [ ] `scripts/dev/rfc_requirements.py:1239` - `load_audit`: reads `rfc/audit/<rfc>.json`,
  returns `{}` when absent. The artifact-per-RFC shape this spec mirrors.
- [ ] `scripts/dev/rfc_requirements.py:1227` - `verdict_is_fresh`: a verdict is fresh only
  while requirement text AND every tagged test are byte-identical; "biased to over-trigger:
  a false 'stale' costs a re-read, a false 'fresh' ships a test that no longer enforces".
- [ ] `scripts/dev/rfc_requirements.py:714` - `_git_baseline_ids`: the HEAD-baseline read
  pattern (`git ls-tree` then per-blob parse) the new ratchet reuses.
- [ ] `scripts/dev/rfc_requirements.py:899` - `_git_cat_blobs`: batch HEAD blob read, the
  measured reason the baseline costs +0.5s instead of +1.7s.
- [ ] `scripts/dev/rfc_requirements.py:763` - `_git_baseline_summary_stems`: returns None
  (not an empty set) on git failure, because its consumption polarity makes an empty
  baseline accuse every file in the repository. The new ratchet's polarity is the opposite
  and an empty set is safe; the distinction must be restated, not copied blindly.
- [ ] `scripts/dev/rfc_requirements.py:1408` - `_render_rollup`: the Coverage-by-RFC table
  the extraction backlog is published beside.
- [ ] `scripts/dev/rfc_requirements.py:1578` - `check_ledger_fresh`: a stale
  `ai/RFC-REQUIREMENTS.md` fails the build, so anything published there cannot rot.
- [ ] `scripts/dev/rfc_requirements_test.py` - 1829 lines; `_load:21`, `_patched:34`,
  `_run_capturing:51`, `_req:479`, `_tag:492`, `_FakeSubprocess:1293`, and the
  helper-plus-wiring pairing every gate-level class follows.
- [ ] `scripts/status/verify_run.go:237` and `:259` - `stagesForMode` runs `ze-rfc-check`
  in BOTH verify branches; `TestStagesForModeMatchesGolden` pins each against a committed
  golden, so adding a verify stage is a two-branch plus golden change.
- [ ] `Makefile:437-443` - `ze-rfc-check` runs `--selftest` then `--check`; `ze-rfc-index`
  runs `--write`.
- [ ] `internal/component/bgp/message/rfc7606_test.go:9-23` - the authored side of the
  two-way link: `// RFC requirement: <id> <polarity>` tags sit on the test, inline or on
  the doc comment. The MACHINERY half of this spec does not touch the tag side; phase 8
  adds tags for every requirement the rfc7296 walk newly extracts.

**Source files read for the rfc7296 pilot (phase 8):**
- [ ] `rfc/full/rfc7296.txt:1392-1441` - Section 2.2 in full; the two obligations at `:1397`
  and `:1439`, quoted verbatim above.
- [ ] `rfc/short/rfc7296.md` - 23 checklist lines, 18 gated, 14 anchored sections, zero
  `RFC7296-2.2-*` ids.
- [ ] `rfc/enrolled.txt:159` - the rfc7296 descriptor: 15 met, 1 single-polarity positive,
  2 gap.
- [ ] `docs/features/rfc-status.md:213` - the public rfc7296 row: Partial, disclosing
  exactly two MUST gaps (`RFC7296-2.9-1` TS narrowing / TS_UNACCEPTABLE, `RFC7296-1.4-1`
  Delete-payload echo).
- [ ] `internal/component/ike/engine/msgid.go:1-90` - `pendingRekey` (which holds
  `messageID` and `sentMsg` "for retransmit") and `inboundClass` / `classifyInbound`, the
  Section 2.3 window classifier including `inboundRetransmit`.
- [ ] `internal/component/ike/engine/sa.go:83-96` - `NextMsgID uint32`, the cached
  last response, `RetransmitTime` and `RetransmitCount`.
  → Note (`ai/rules/no-fabrication.md`): these are named as WHERE the walk looks, not as a
    conformance verdict. Whether Ze already satisfies `:1397` and `:1439` is determined in
    phase 8 by reading the producing functions, and the answer is cited `file:line` there.
    Nothing in this spec asserts it in advance.

**Behavior to preserve:**
- Every existing check in `run_check:1629` keeps its current semantics, exit codes, and
  message text. This spec ADDS checks; it changes none.
- The four ratchets (`check_enrolment`, `check_coverage_ratchet`,
  `check_retired_requirements`, `check_new_summaries`) keep grandfathering pre-HEAD state.
- `--check` exit codes stay 0 / 2, and `--check-fresh` stays 0 / 1 / 2 (`run_check_fresh:1725`).
- `ai/RFC-REQUIREMENTS.md` stays fully generated and byte-stable across renders
  (`test_render_is_deterministic`, `rfc_requirements_test.py:1743`).
- `rfc/short/*.md` stay protocol-only: no Ze-specific field is added to them.
- `rfc/enrolled.txt` stays one-stem-per-line, parsed by `line.split()[0]`
  (`parse_enrolled:688`).
- `ze-rfc-check` stays a single verify stage in both branches; no golden change.
- The 166 currently enrolled RFCs stay green when the MACHINERY lands (phases 1 to 7, zero
  artifacts present), and stay green after the pilot lands (phase 8, exactly one artifact
  present, 165 unsigned). Grandfathering is scope, so a non-empty artifact set changes
  nothing for the stems that have no artifact.
- The 23 requirement ids already in `rfc/short/rfc7296.md` keep their ids and their proof.
  Phase 8 adds rows; `check_retired_requirements` and `check_coverage_ratchet` both stay
  green through the re-authoring, by construction rather than by luck.

**Behavior to change:**
- `check_enrolment` gains one precondition that applies only to stems newly enrolled since
  HEAD: a valid extraction sign-off must exist.
- `run_check` gains `check_extraction_signoff` and `check_extraction_ratchet`.
- `render_ledger` gains an extraction backlog table with one column per register.
- Two new CLI modes and two new make targets are added.
- `rfc/short/rfc7296.md` gains checklist rows for every obligation the phase-8 walk newly
  extracts, at minimum `RFC7296-2.2-1` and `RFC7296-2.2-2`. The rfc7296 descriptor in
  `rfc/enrolled.txt:159` (still one stem per line, still `line.split()[0]`-parseable) and
  the public row at `docs/features/rfc-status.md:213` move with it.
- The IKE implementation and its tests change wherever a newly extracted requirement is not
  already implemented and proven. The set is not enumerable before the walk (R-10).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A maintainer runs `make ze-rfc-extract STEM=<stem>`. The tool reads the pinned source
  text (`rfc/full/<stem>.txt` or `rfc/drafts/<stem>.txt`) and writes an UNCLASSIFIED
  skeleton to `rfc/extraction/<stem>.json`.
- A maintainer classifies every derived site and section in that file by hand.
- `make ze-rfc-check` (both verify branches) re-derives the inventory and judges the
  classification.
- A maintainer adds a stem to `rfc/enrolled.txt`; the sign-off is now its precondition.
- Phase 8 adds NO entry point: the pilot drives `make ze-rfc-extract STEM=rfc7296` and then the same classify-and-check loop, which is the proof that the format is usable.

### Transformation Path
1. Source text read from `rfc/full/<stem>.txt` or `rfc/drafts/<stem>.txt` (the same two
   locations `source_keyword_count:1329-1331` already searches, in the same order).
2. Register derivation: the tool decides, from the text alone, which keyword register can
   serve as an oracle for this RFC (`rfc2119`, `prose`, or neither).
3. Site derivation: normative sites are enumerated in document order and attributed to the
   enclosing section, producing stable locators of the form `<section>:<n>`.
4. Section derivation: every section heading in the source is enumerated, with its site count.
5. Skeleton write (`--extract-skeleton`): the derived inventory is written with every
   disposition null. Nothing is classified, and an all-null artifact FAILS the check.
6. Human classification: each site becomes `mapped` (to a requirement id in the summary)
   or `excluded` (with a kind from a closed set and a mandatory reason); each section
   becomes `walked` or `skipped` with a reason.
7. Check (`--check`): the inventory is re-derived from the current source text, the
   recorded `source-sha` is compared, and the forward and reverse arithmetic is evaluated.
8. Ratchet: the signed-off stem set and each stem's exclusion count are compared against
   HEAD via the existing batch blob reader.
9. Render (`--write`): the per-RFC extraction status and the grandfathered backlog are
   published into `ai/RFC-REQUIREMENTS.md`, where `check_ledger_fresh:1578` keeps them fresh.
10. Status emission (`--extraction-status --json`): the machine-readable counts the
    umbrella's drain schedule consumes.
11. Floor comparison (`--check`): `rfc/drain-budget.txt` supplies the policy; the floor is
    computed, capped at the backlog size, and judged against the signed count. Inert at rate 0.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RFC source text ⇄ sign-off artifact | `source-sha` over the normalized source, using the existing `_normalize`/`requirement_sha:1219` pair | No |
| Sign-off artifact ⇄ summary | every `mapped-to` names a requirement id parsed from `rfc/short/<stem>.md` | No |
| Sign-off artifact ⇄ enrolment | `check_enrolment` refuses a NEW stem in `rfc/enrolled.txt` with no valid sign-off | No |
| Sign-off artifact ⇄ HEAD | `git ls-tree` plus `_git_cat_blobs:899` over `rfc/extraction/`, same pattern as `_git_baseline_ids:714` | No |
| Sign-off artifact ⇄ published ledger | `render_ledger` extraction table, kept fresh by `check_ledger_fresh:1578` | No |
| Extraction status ⇄ umbrella drain quota | `--extraction-status --json` envelope with `schema-version`, carrying a per-register split so quota credit and evidence strength stay separable (ruling 1) | No |
| Drain policy ⇄ floor comparison | `rfc/drain-budget.txt` (authored: start date, rate) read by `check_drain_floor` and compared against the derived signed count, capped at the backlog size. The policy crosses INTO this spec; the spec never writes it back (resolved 2026-07-29) | No |
| Newly extracted rfc7296 requirement ⇄ Thomas | phase-8 escalation only: requirement id, verbatim RFC text with its `rfc/full/` line, producing code as `file:line`, and the cost of full proof (`ai/rules/rfc-compliance.md:36-51`) | No |

### Integration Points
- `run_check:1629` - the composition point; all three new checks
  (`check_extraction_signoff`, `check_extraction_ratchet`, `check_drain_floor`) are appended
  there, inside the existing `try` so a malformed artifact or an unparseable budget exits 2
  with a clean message through the existing handler (`:1688-1694`), never a traceback.
  Re-read 2026-07-29: `run_check` composes nine checks today and not one of them reads a
  budget file or computes a floor, which is why the comparison needed an implementing owner.
- `check_enrolment:655` - gains the new-enrolment precondition.
- `_render_rollup:1408` / `render_ledger:1465` - gain the extraction table.
- `Makefile:437` - `ze-rfc-check` is unchanged and keeps running `--selftest` then `--check`,
  so the new check rides an existing verify stage and `stagesForMode` is untouched.
- `scripts/dev/rfc_requirements_test.py` - `--selftest` runs it before `--check` judges the
  live tree, so the fixtures gate the gate.
- `_render_rollup:1408` / `render_ledger:1465` - the extraction table's register columns are
  the published half of ruling 1. A rendered signed count with no register split beside it
  is a defect, not a formatting choice (AC-21).
- Phase 8 pilot: `rfc/short/rfc7296.md` (re-authored), `rfc/extraction/rfc7296.json` (the
  first artifact), `rfc/enrolled.txt:159` (descriptor), `docs/features/rfc-status.md:213`
  (public row), `ai/RFC-REQUIREMENTS.md` (regenerated by `make ze-rfc-index`, kept fresh by
  `check_ledger_fresh:1578`), and the IKE tests that carry the new `RFC requirement:` tags.
- Phase 8 does NOT integrate through a new code path. Every new check it exercises is one
  phases 1 to 7 already built, which is exactly what makes it a pilot rather than a feature.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | To verify: the check reads the source text only through the same two-location lookup `source_keyword_count:1329` uses |
| No unintended coupling (components stay isolated) | No | To verify: nothing outside `scripts/dev/rfc_requirements.py` learns the artifact format; `rfc/short/*.md` gains no Ze-specific field |
| No duplicated functionality (extends existing, does not recreate) | No | To verify: reuses `_normalize`, `requirement_sha`, `_git_cat_blobs`, `parse_summary_file`, `check_ledger_fresh`; adds no second git reader and no second freshness mechanism |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A for the machinery: Python developer tooling, no wire path. NOT automatically N-A for phase 8: any IKE encoding or decoding it touches is a wire path, so `ai/rules/buffer-first.md` and `ai/rules/no-sprintf-alloc.md` apply there in full and are checked per change rather than waived by this row |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | To verify: no per-RFC branch anywhere. Register selection, site inventory, section inventory and backlog table are all derived from the source text and the artifact directory. The exclusion-kind set and the register set are closed enums validated at parse time, mirroring `ANNOTATION_KINDS:77` and `_parse_annotation:213`, not per-RFC special cases. The rfc7296 pilot does not weaken this: it adds a DATA file under `rfc/extraction/` and names the stem only in tests and in this spec, never in a branch, an allowlist or a code constant. Grandfathering stays new-since-HEAD scope, so no stem was added to any list when rfc7296 stopped being grandfathered |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The sizing is robust to extractor methodology. | Two independent derivations 2026-07-29: brief said 3940 sites / 0.69 / 60 RFCs / deficit 1646; independent re-run gave 3988 / 0.68 / 61 / 1683 | The scale of the hole is misjudged and the drain quota is set against a wrong denominator | Implementation re-derives and publishes the real numbers in the ledger; compare against both estimates | confirmed (2026-07-29, phase 2) |
| A-2 | A capitalised-keyword inventory is the WRONG oracle for a large minority of enrolled RFCs. | Measured: 53 of 166 enrolled RFCs declare more gated requirements than the capitalised-keyword site scan finds sites for. rfc2181 declares 23 from 1 site; `rfc/full/rfc2181.txt` Section 3 says the memo does not use the 2119 expressions | A single-register design ships and is vacuously green for a third of the tree | Re-derive the register split during implementation; assert the counts in a test over the live tree | confirmed (2026-07-29, phase 2) |
| A-3 | 22 enrolled RFCs have zero capitalised MUST-level keywords in their source. | Measured 2026-07-29 over `rfc/enrolled.txt`: rfc1071, rfc1332, rfc1350, rfc1877, rfc2205, rfc2328, rfc2347, rfc2348, rfc2349, rfc2918, rfc2966, rfc3101, rfc3623, rfc5701, rfc6286, rfc7534, rfc7535, rfc792, rfc8050, rfc8571, rfc905, sflow-v5. They declare 164 gated MUSTs between them | The pre-2119 fail-open is mis-sized | `test_pre2119_register_is_derived_over_live_tree` asserts the live count is non-zero and every such stem derives the prose register | confirmed (2026-07-29, phase 2) |
| A-4 | A case-insensitive modal scan gives those 22 a non-empty inventory. | Measured: 688 lowercase normative sites across the 22 (rfc905 229, rfc2328 159, rfc2205 102) | The prose register is as vacuous as the keyword one and only a manual walk remains | Re-derive during implementation; the prose-register test asserts a non-empty inventory for a fixture built from a keyword-free source | confirmed (2026-07-29, phase 2) |
| A-5 | At least one enrolled RFC has an empty inventory under BOTH registers while declaring gated MUSTs. | Measured: rfc1877 has 0 capitalised and 0 lowercase modal occurrences in 10591 chars, and declares 4 gated MUSTs | Only the live `manual-walk` COUNT is wrong; the register itself stays either way. Owner ruling 1 makes an undrainable remainder unacceptable, so an RFC with no mechanical inventory must still have a route to a sign-off, and `manual-walk` is that route. If the live set turns out empty, the ledger's `manual-walk` column reads 0 and the register remains the terminal escape AC-10 needs | Implementation enumerates the live set and records the real count in the ledger. The register is NOT dropped on a zero count (superseding the earlier "drop it and record the deviation" plan, 2026-07-29 owner ruling 1) | confirmed (2026-07-29, phase 2) |
| A-6 | Site granularity is required; section granularity is not sufficient. | Measured over the 166: 1671 hole-sites sit in wholly-unanchored sections but 558 sit in sections that already carry ids, and 618 of 1502 sections have sites and no ids. rfc4271 is 9 unanchored against 48 partial | A far cheaper section-only design would do, and the site machinery is over-built | Re-derive the unanchored/partial split during implementation and record it in the learned summary | confirmed (2026-07-29, phase 2) |
| A-7 | Published RFC source texts do not change, so `source-sha` staleness fires almost never for `rfc/full/` and occasionally for `rfc/drafts/` (7 files). | RFCs are immutable once published; `_ID_RE`'s section anchor rests on the same immutability (`rfc_requirements.py:110-113`) | Sign-offs stale in bulk and the ratchet becomes noise | Implementation records the sha; any staleness observed in the first release is a signal to revisit | ~~unvalidated -- see Phase Results~~ **deferred with a destination (2026-07-29), NOT left unvalidated.** It is a claim about FUTURE churn in the RFC source texts and no evidence available inside one session can settle it. What IS established: the sha is recorded, is re-derived deterministically from the single two-location lookup, and `_evaluate_extraction` (`scripts/dev/rfc_requirements.py:2498-2508`) fails an artifact the moment its source moves -- so the risk A-7 names is DETECTED rather than silently absorbed. What remains open is whether that detection RATE is tolerable once the fleet drain signs off at volume, and only measured throughput answers it. Homed at `plan/deferrals/rfcgate-1-extraction.md:9`, Status `deferred`, Destination `plan/spec-followup-rfc-enrollment.md` (the spec that owns the fleet drain, i.e. the first activity that produces the volume A-7 is a claim about) |
| A-8 | The umbrella's drain quota consumes a per-register count that this spec publishes, and credits a sign-off in ANY register. | Owner ruling 1 (2026-07-29): a `prose` or `manual-walk` sign-off counts toward the quota exactly as `rfc2119` does, provided each register is published in its own column. `plan/spec-rfcgate-0-umbrella.md` "Drain Schedule Design (D2)" owns the value of N and the cadence | Only the TRANSPORT would be wrong (the umbrella hand-parsing the rendered table instead of the JSON envelope). The counting semantics are settled and do not move | `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` and `TestExtractionLedger.test_registers_are_published_in_separate_columns`; the envelope is cheap either way | confirmed |
| A-10 | The number of obligations the rfc7296 walk newly extracts is NOT estimable before the walk runs. | 263 derived sites against 18 captured is a SITE count, and per-RFC false-positive calibration does not exist. The corpus-wide 1200-1500 estimate is calibrated across 166 RFCs and says nothing reliable about one of them | Nothing breaks: the scoping conversation with the owner simply happens earlier and against a better prior | Phase 8a produces the real count BEFORE phase 8c writes any implementation, and the count is taken to the owner (R-10) | **confirmed (2026-07-29, phase 8a).** The walk ran and returned **214 distinct obligations** (63 `implemented-and-testable`, 25 `implemented-untested`, 108 NOT IMPLEMENTED, 18 `uncertain`), reached by reading 289 keyword lines in context, collapsing 43 restatements and judging 8 non-normative. No pre-walk figure predicted it: this spec's own estimate was 263 derived SITES, a different unit at a different denominator, and the corpus-wide 1200-1500 calibration says nothing about one RFC. The count went to the owner exactly as R-10 required, and it is what produced owner ruling 3. Evidence: `plan/deferrals/rfcgate-1-extraction.md:8`; the 214 rows are enumerated in `plan/spec-rfcgate-1b-rfc7296-pilot.md` Appendix A |
| A-11 | Neither §2.2 obligation is proven today by a test that merely lacks a tag. | `grep -c "RFC7296-2\.2-" rfc/short/rfc7296.md` returns 0, so no tag can name them. An untagged test may still exercise the behavior, which the grep cannot see | The phase-8 work for those two is a summary row plus a tag rather than an implementation, which is cheaper, not different in kind | Phase 8c reads the producing functions in `internal/component/ike/engine/msgid.go` and `sa.go` and cites `file:line` for the verdict, per `ai/rules/no-fabrication.md` | ~~unvalidated~~ **MOVED 2026-07-29 with phase 8 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md`.** Its validation method IS phase 8c, which is that spec's work, so it cannot be settled here without doing the work this spec was ruled out of. Not left dangling: the destination spec already records what the walk established about IMPLEMENTATION status (`:181-182`) -- `RFC7296-2.2-1` is implemented by construction because retransmission resends `sa.LastSentMsg` byte-identically (`internal/component/ike/engine/fsm.go:138-142`), and `RFC7296-2.2-2` is NOT, since `NextMsgID` is a bare `uint32` (`sa.go:83`) whose every mutation is an unchecked `++`. Those are that spec's citations, relayed here with their source rather than re-asserted as this spec's own reading (`ai/rules/no-fabrication.md`). What A-11 actually claims -- that no UNTAGGED test already proves either -- is a proof-side question its phase 8c answers |
| A-9 | Adding these checks inside `--check` keeps `ze-rfc-check` fast enough to stay in both verify branches. | `_git_cat_blobs:899` records the measured budget: 1.7s at HEAD, 2.2s with the batched baseline, and states that a gate which doubles verify time "is a gate people learn to skip" | The extraction check makes verify slow enough to be bypassed | Time `make ze-rfc-check` before and after; the delta budget is stated as a Deliverable | confirmed (2026-07-29, phase 7) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The exclusion list becomes a 1600-slot escape hatch: every unmapped site excluded with a shrug. | An RFC signs off with an exclusion ratio near 1.0 | Exclusions are shrink-only per stem against HEAD (a rise requires a recorded `resign-reason`); the kind set is closed and each kind carries a distinct, checkable obligation; `duplicate-of` must name an id that some OTHER site maps, so a chain of duplicates cannot cover an RFC with nothing mapped; the per-RFC exclusion ratio is published in the ledger. Sunlight rather than a numeric threshold, which would be gamed by rewording |
| R-2 | Sign-offs are mass-generated for an audit nobody performed. Already ruled out once for `rfc/audit/*.json` (`plan/deferrals/rfc-gate-regression-ratchets.md`, 2026-07-20). | A commit adds many `rfc/extraction/*.json` at once | Structural, not policy: the skeleton writer can only emit UNCLASSIFIED dispositions, and an unclassified site FAILS the check. Generating skeletons en masse makes the gate REDDER, never greener. There is no `--sign-off` mode, no default disposition, and no bulk classifier |
| R-3 | A site is mapped to a requirement whose TEXT misquotes the RFC (the RFC 4271 Section 5.1.6 case, `ai/rules/rfc-compliance.md:94-98`). | A reviewer reading a `mapped-to` pair finds the requirement does not render the sentence | Out of mechanical reach and stated as such. The artifact makes the source-sentence-to-requirement pair EXPLICIT, which is what `/ze-rfc-audit` needs to be pointed at. The gate checks the link's endpoints exist; it never claims to judge the rendering |
| R-4 | The pre-2119 vacuous green: a keyword-driven check reports "0 sites, all classified" for 22 enrolled RFCs holding 164 gated MUSTs. | An RFC with a keyword-free source signs off instantly with an empty inventory | The register is DERIVED, never authored; a `rfc2119` claim on a keyword-free source is refused; an inventory empty under both registers with any gated requirement declared is refused as unevaluable, and the only remaining route is a `manual-walk` sign-off, which is credited to the drain quota exactly like any other register (owner ruling 1) but is published in its OWN column and is never folded into a number rendered without its register split |
| R-5 | The check over-fires and reds `ze-rfc-check` in both verify branches, blocking every commit in the repo. | `make ze-verify` red on a tree with no RFC change | Grandfathering is implemented as SCOPE, not as an allowlist: with no `rfc/extraction/*.json` present the new check judges only stems newly enrolled since HEAD, so the 166 stay green when the machinery lands, and the 165 unsigned ones stay green after the pilot. The first sign-off is opt-in, per RFC |
| R-6 | The check under-fires and is quietly satisfied by nothing, reproducing the failure it exists to fix. | `make ze-rfc-check` green with zero sign-offs and no mention of the fact | The success line and the ledger both report signed-off-of-enrolled counts on every run, mirroring `run_check:1707-1711`. "0 of 166 signed off" is loud, not silent. `check_enrolment:661` already sets this precedent by refusing to report clean while enforcing nothing |
| R-7 | The site extractor's recall is poor enough that a full sign-off still misses obligations. | RFC 4271 Section 8.2.2: 35168 chars of normative state machine, ONE capitalised keyword site (a SHALL), the rest carried by indicative prose ("the local system: initializes ... sets ... starts") | Stated as a Known Limitation rather than papered over. The section axis exists precisely for this: a section may record `unsourced-ids` for requirements read from indicative prose with no keyword site behind them, so the §8.2.2 case is expressible without lying about it |
| R-8 | Two specs both grow `rfc/enrolled.txt` and collide. | A merge conflict in `rfc/enrolled.txt`, or two specs claiming ownership | This spec adds a PRECONDITION to enrolment and never adds a row. `plan/spec-followup-rfc-enrollment.md` keeps ownership of the file and the rollup. See the interaction section below |
| R-9 | The artifact's derived fields (`quote`, site count) are hand-edited to make a red go green. | A `quote` that does not match the re-derived site text | The check re-derives every derived field and compares. A mismatch is a violation naming the locator, not a silent overwrite |
| R-10 | **The rfc7296 phase has an unbounded tail.** Its capture ratio is the worst measured in the corpus (263 derived sites against 18 captured gated requirements, 92 sections against 14 anchored), so the walk may surface a large number of new obligations, each owed working code plus a positive and a negative tagged test. The number is not knowable now (A-10) | Phase 8a's real count lands and exceeds what phases 1 to 7 cost combined | Sequencing, not scope reduction. Phase 8a produces the count FIRST and it goes to the owner for scoping before any implementation begins. Nothing in this spec authorises meeting a large count by extracting less, annotating, or deferring: those are exactly what the escalation rule and the Failure Routing row forbid. If the tail is too large for one spec, the OWNER splits it, and the spec stays OPEN until he rules |
| R-11 | Re-authoring adds a gated MUST row that is unproven at the moment it is written, reddening `ze-rfc-check` for an enrolled RFC | `make ze-rfc-check` fails naming a new `RFC7296-*` id with no tag and no annotation | The commit that ADDS a row is the commit that proves it, or that carries Thomas's authorised annotation. Never two commits. This is a sequencing obligation on phase 8, not a reason to write the row later: writing obligations down and gating none of them is the rot `check_new_summaries` exists to stop, and hiding one to stay green is worse than a red |
| R-12 | Phase 8 must change an EXISTING rfc7296-tagged test (a correction to a misquoted requirement re-points what the test must assert) | The `rfc-tagged-test` edit-time guard rejects the edit | Correct outcome, not an obstacle. Only the user may authorise it, via `// rfc-test-change-approved: <date> <what was approved>`; `// test-relax:` explicitly does NOT satisfy that guard. Phase 8 never self-authorises, and an authorisation it did not receive is a STOP |
| R-13 | Cross-child collision with `plan/spec-rfcgate-4-ledger.md`, which arms a gate that is RED by design on four stems (its R-6, superseded there by R-6a and OC-1) | A `make ze-verify` red on rfcgate-1's landing commit that names `unproven support` for rfc1035, rfc3765, rfc4486 or rfc5301 | The umbrella's Sequencing Constraint puts this spec first and forbids two children in flight. Independently: an armed red leaves `make ze-verify` non-green and `commit_helper.py create` refuses a script over that, so rfcgate-4's gate may arm ONLY in the commit that clears its four stems, or later. ~~A deterministic structural gate is never a permitted known-red (`ai/rules/git-safety.md:229`, and `--structural-red-ok` is owner-only)~~ (corrected 2026-07-29: `ze-rfc-check` is not in `STRUCTURAL_GATES`, `scripts/dev/commit_helper.py:512-523`; the binding rule is `ai/rules/fix-dont-record.md`, which forbids logging a deterministic reproducible red, leaving no legal bypass short of an owner ruling per commit). This spec lands green in both phases (AC-19, AC-24) and must never inherit that red |
| ~~R-14~~ | ~~The umbrella's child-1 row names `rfc/recertified.txt`; this spec ships `rfc/extraction/<stem>.json` plus a derived `--extraction-status --json`~~ | ~~The umbrella's drain check looks for a file this spec never creates~~ | **CLOSED 2026-07-29 by the umbrella, which was the right place to decide it.** The resolution: the `rfc/extraction/<stem>.json` set IS the record, and the quota is DERIVED from it through ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Two spec corrections found by implementing it"); no per-stem ledger file is created, by this spec or any other. A second hand-kept list of who has been signed off is the rotting registry `ai/rules/derive-not-hardcode.md` forbids, and the 2026-07-20 ruling in `plan/deferrals/rfc-gate-regression-ratchets.md` already refused that artifact shape. What remains authored is POLICY only: a start date and a rate, in `rfc/drain-budget.txt`, which this spec creates and which may never name an RFC. See `plan/spec-rfcgate-0-umbrella.md` "Where the counter lives" and "What is still authored" |

## Phase Results (phases 1 to 7, implemented 2026-07-29)

Phases 1 to 7 are implemented and green. **Phase 8 (the rfc7296 pilot) is NOT started**:
8a requires taking the walk's real count to the owner for scoping (R-10), which only the
main thread can do. Nothing under `rfc/short/rfc7296.md`, `rfc/extraction/rfc7296.json`,
`rfc/enrolled.txt`, `docs/features/rfc-status.md` or `internal/component/ike/` was touched.

### Measured register split over the live tree (A-2 to A-6)

Derived over the 166 enrolled RFCs by `derive_inventory`, the shipped code, not a probe:

| Figure | Measured | Spec estimate | Verdict |
|--------|----------|---------------|---------|
| Register split | `rfc2119` **101**, `prose` **64**, `manual-walk` **1** | 113 / 52+ / 1 | A-2 confirmed |
| Cannot take the `rfc2119` grade | **65 of 166** (23 keyword-free + 42 undercount) | 53 | A-2 confirmed, minority LARGER than estimated |
| Zero capitalised MUST-level **sites** | **23** | 22 | A-3 confirmed; see the denominator note below |
| Prose sites over those stems | **777** | 688 | A-4 confirmed |
| Empty under BOTH registers | **rfc1877 only**, 4 gated MUSTs | rfc1877 | A-5 confirmed exactly |
| Capitalised sites, all 166 | **4208** | 3988 | A-1 confirmed (+5.5%) |
| RFCs with more sites than captured | **69** | 61 | A-1 confirmed |
| Raw deficit | **1845** | 1683 | A-1 confirmed (+10%) |
| Sites in wholly-unanchored sections | **3048** | 1671 | A-6 confirmed |
| Sites in sections that ALREADY carry ids | **2862** | 558 | A-6 confirmed MORE strongly: section-only granularity would miss **48%** of hole-sites, not 25% |
| rfc7296 | **261** sites, 104 sections, register `rfc2119` | 263 sites, 92 numbered sections | corroborates A-1 within 1% |

**A-3's count differs from the spec's 22 for a reason worth recording, not a defect.** The
spec measured raw uppercase *occurrences*; the implementation measures normative *sites*
(sentences) after excluding the RFC 2119 boilerplate. `rfc5443` is the whole difference:
its 5 uppercase occurrences are all inside its own "Conventions Used in This Document"
boilerplate, which the site scan correctly refuses to count as an obligation. Both figures
are right at their own denominator -- this is a fourth entry for the umbrella's "three
pre-2119 measurements", not a correction of any of them.

**A-3's validation wording is wrong in one particular, and A-5 is why.** It says "every
such stem derives the prose register". `rfc1877` does not: it derives `manual-walk`,
because it is empty under both scans -- exactly what A-5 predicts. The implemented test
(`test_keyword_free_sources_never_derive_rfc2119`) asserts the property that actually
matters: no keyword-free stem may derive `rfc2119`.

### Cost (A-9)

| Measurement | HEAD | With extraction | Delta |
|-------------|------|-----------------|-------|
| `--check` on the live tree (best of 3) | 2.52-2.55s | 2.54-2.65s | **+0.07s (+3%)** |
| `--selftest` | ~6.3s (217 tests) | 8.56s (252 tests) | +2.2s |
| `make ze-rfc-check` end to end | ~8.8s | **11.7s** | +2.9s (+33%) |

`--check` is flat because the derivation is scoped to stems that HAVE an artifact (zero
today). That scoping was **forced by measurement, not chosen for elegance**: deriving the
inventory for all 166 enrolled RFCs costs 1.94s (read 0.02, strip 0.11, sha 0.07, keyword
scan 0.76, prose scan 0.97), and `check_ledger_fresh` re-renders on every `--check`, so a
ledger row carrying a derived register for an UNSIGNED stem would have put that 1.94s on
every verify -- a +76% gate, against `_git_cat_blobs:899`'s recorded budget. The extraction
table therefore leaves the derived columns blank for unsigned stems, which is also the
honest rendering: a register derived for a stem nobody has walked is not a fact this
repository has established.

The `--selftest` growth is the live-tree class (`TestRealTreeExtraction`), which derives
all 166 once per run. It is the only test that would catch a derivation that passes every
fixture and returns nothing on real RFC formatting, so the 2.2s buys the corpus-level
guarantee. 2.9s sits inside a `ze-verify` measured in minutes.

### Two spec corrections found by implementing it

| Where | Correction |
|-------|-----------|
| ~~AC-16, AC-22, the Wiring Test row and the Deliverables row~~ **Eleven sites across two specs** (amended 2026-07-29, see below) all spell the command `make ze-rfc-extraction-status --json` | **Not runnable.** GNU make parses `--json` as a make option and exits 2 with `unrecognized option '--json'` before any recipe runs (verified). The target therefore ALWAYS emits the JSON envelope -- that envelope is the mode's only consumer, so a second human-readable shape would be a mode to keep in step for nothing -- and the runnable command is `make ze-rfc-extraction-status`. The script still accepts `--json` and ignores it, so the documented spelling works at the script level. |
| The artifact's top-level field table (Design, "The artifact") | AC-10 requires a `manual-walk` sign-off to carry "a stated reason why no mechanical inventory exists", and no field in the table could hold it. Added `register-reason`, required only when `register` is `manual-walk` -- the same conditionally-required shape `resign-reason` already has. |

**Amendment 2026-07-29 (review finding): recording a correction is not applying
it.** Both rows above were written and then NEITHER was applied. Row 1's spelling
was left standing at every site it names, so every consumer of this spec still read
the unrunnable command; row 2's field was implemented in code but never added to the
Design field table, which is the contract a future implementer reads.

Row 1 -- eleven sites carried the unrunnable spelling: in THIS spec the R-14
resolution, the Wiring Test row, AC-16, AC-22, user story 4, and **both**
Deliverables Checklist rows ("Every register earns credit", "Status envelope"), the
last two being the ones that instruct this spec's own closer to run a command that
exits 2 and to record its output as verification; and in
`plan/spec-rfcgate-0-umbrella.md` five more (the D2 interface row, the child-1 row,
"Where the counter lives", "Who implements the floor", and the Goal Validation row).
All eleven are now struck and corrected in place. A twelfth sits in
`plan/spec-rfcgate-1b-rfc7296-pilot.md` ("The sign-off validates" row) and is left
for that spec's owner.

Row 2 -- `register-reason` is now in the Design field table, and the omission is
recorded in Deviations.

The general lesson: a correction written into a Phase Results table has changed
nothing until the sites it names are edited (`ai/rules/fix-dont-record.md`).

### Design decisions taken during implementation

| Decision | Why |
|----------|-----|
| `signed-off`, `reviewer` and `register-reason` are required at CHECK time, not PARSE time | A freshly generated skeleton has no date and no reviewer. Requiring them to parse meant either the skeleton did not parse (so the gate reported a missing field instead of the unclassified sites, and a reviewer could not run the check mid-walk) or the writer had to INVENT a date and a reviewer -- fabricating a sign-off record for a walk nobody performed, which is R-2's failure mode exactly. |
| `duplicate-of` names its target in `mapped-to` | AC-8 is only checkable if the duplicate NAMES the id it duplicates. `mapped-to` reads as "the requirement id this site relates to": for a mapping, the id this site proves; for a duplicate, the id already captured elsewhere. No new field, and the closed set stays closed. |
| `check_enrolment` takes `newly_enrolled` from the CALLER rather than computing `current - baseline` itself | Every existing use of `baseline` in that function is `baseline - current`, where `_git_baseline_enrolment:698` returning an empty set on git failure accuses nobody. `current - baseline` against that same empty set would accuse all 166 enrolled RFCs of being new. Same trap `_git_baseline_summary_stems:763` documents, opposite direction. `None` means "could not tell" and the precondition is skipped. |
| `extraction_status` counts `signed` INDEPENDENTLY of the register split | Mutation testing caught this: with `signed` defined as `sum(counts.values())`, AC-22's "the keys sum to the published total" was a tautology that no test could ever fail. Two derivations that must agree is a real cross-check. |
| `source_keyword_count` now reads through `source_text` | It inlined its own two-location lookup, so the spec's Architectural Verification row ("the source text is read only through the same two-location lookup `source_keyword_count:1329` uses") was not actually true of the code. One reader now; behaviour identical (None for absent, None for unreadable). |

### Mutation verification (Critical Review Checklist, "Vacuity")

Every new check was disabled in turn and the tests that exist to prove it were re-run. All
eight went fully RED and all restored green (`tmp/rfcgate-mutation.py`, in-memory only).
Two survivors were found and are recorded above and below: the tautological sum (fixed in
the code) and two tests that passed with their producing code disabled
(`test_keyword_free_sources_never_derive_rfc2119`, which needed an upper bound because a
totally broken scan makes every stem `manual-walk`; and
`test_registers_are_published_in_separate_columns`, which asserted register NAMES while
the table's own explanatory prose already names all three -- now asserts the rendered
counts).

### Not done in this run

- Phase 8 in full (8a to 8e). AC-23 to AC-26 are untouched.
- A-7 (source texts do not change) cannot be validated inside one session: it is a claim
  about future churn. What IS established is that the sha is recorded, re-derived
  deterministically, and read through the single two-location lookup. It needs a closure
  decision rather than a phase-7 verdict.
- `make ze-doc-test` and `make ze-verify-wiring-docs` are RED on this tree from 29 stale
  anchors in `ai/digests/mcp.md`, caused by a CONCURRENT session having deleted
  `internal/component/mcp/{session,elicit,reply_sink}.go` in the working tree without yet
  updating that digest. Not this spec's work, and in a directory this run was told not to
  touch. Every other gate in both targets passed: "Wiring check PASSED", "No documentation
  drift detected", "All commands validated", "all corpus path references resolve",
  "ai/RFC-REQUIREMENTS.md up to date".

## Deviations

Added 2026-07-29 after an independent review of phases 1 to 7. Every row was
reproduced by the reviewer and re-verified against the producing code before being
recorded here (`ai/rules/no-fabrication.md`). DEV-1 to DEV-3 are deviations from the
spec as written; DEV-4 to DEV-7 are defects the review found in what was implemented,
kept here rather than only in a report because a report is not a record
(`ai/rules/fix-dont-record.md`).

| # | Deviation | Why, and what it changes |
|---|-----------|--------------------------|
| DEV-1 | The artifact carries an eleventh top-level field, `register-reason`, which the spec's Design field table did not list | AC-10 requires a `manual-walk` sign-off to carry "a stated reason why no mechanical inventory exists", and none of the ten listed fields could hold it. Enforced in code: `_evaluate_extraction` fails a `manual-walk` artifact with no `register-reason`, and `parse_extraction_artifact` accepts the key and reads it as optional at parse time so a mid-walk skeleton still parses. The FIELD was right from the start; what was missing is that only the Phase Results correction row recorded it, never the Design table a future implementer reads as the contract. That table is now amended in place. **No approval needed** -- the field is required BY an AC, so adding it implements the spec rather than departing from it; only the spec's own table was behind |
| DEV-2 | **AC-21's literal wording is not met.** It requires the extraction table to render "one column per register (`rfc2119`, `prose`, `manual-walk`) with its own signed count". What ships is one `Register` column per RFC ROW, plus a prose summary line above the table reading `Signed off by register: rfc2119 0, prose 0, manual-walk 0.` | The AC's INTENT -- owner ruling 1's mandatory counterweight, that no signed count is ever published without its register split -- is fully met, verified by reading every publishing site: `render_extraction_table`'s summary line (via `_register_phrase`), `run_check`'s success line, and `extraction_status`'s `signed-by-register` key. `register_counts` returns all three registers even at zero, so a register can never go missing and read as "not a thing" rather than as zero. There is verifiably NO bare signed total anywhere. The substitution is a rendering choice: the table already has one row per stem, so a three-column cross-tab would be one populated cell per row and two blanks. **Recorded rather than resolved by editing AC-21**: `ai/rules/planning.md` makes specs append-only, and silently relaxing an AC to match the code is the exact failure this spec set exists to correct. The AC text stands as written; accepting the substitution or rendering the cross-tab is a closure decision, not an implementer's |
| DEV-3 | The command correction in "Two spec corrections found by implementing it" was recorded and then applied at none of the eleven sites it named | See the amendment under that heading. Fixed 2026-07-29: all eleven struck and corrected in place |
| DEV-4 | **Defect: the drain floor double-counts every sign-off, so an armed schedule goes permanently green at HALF the corpus.** `check_drain_floor` computes `backlog = len(enrolled - set(signed))` and passes that as `required_floor`'s cap, while comparing it against the CUMULATIVE `total = len(signed)`. Each sign-off therefore both raises `total` by one and lowers the cap by one | Reproduced 2026-07-29 by driving `required_floor` directly: enrolled 166, rate 100/calendar month, 12 months elapsed (a schedule demanding the whole corpus), and the comparison flips red-to-green at exactly `signed = 83`. Whenever the owed count reaches the remaining backlog the condition collapses to `total >= enrolled - total`, i.e. `total >= enrolled/2`, and no rate however aggressive demands more. AC-28's stated intent ("capped at the backlog size ... so the check goes permanently green once the backlog is DRAINED") wants the cap over the FULL enrolled set: with `min(len(enrolled), owed)` the floor is 166, `total = 83` stays red, and the check still self-retires at `total = 166`. **The fix belongs in `scripts/dev/rfc_requirements.py` and is not made by this documentation pass.** Recorded so the closer cannot reach green on the shipped rate of 0 without seeing it: AC-30 calls a floor that can never fail "the vacuity this spec exists to remove", and at any rate above the backlog this floor cannot fail past halfway. **RESOLVED 2026-07-29, same day**: `required_floor`'s third parameter is now `drainable` (renamed from `backlog`, because its meaning changed and the old name would have preserved the bug in the reader's head), and `check_drain_floor` passes `len(enrolled)` rather than the remainder. Re-measured on the same input: 166 enrolled at rate 100/month over 12 months now flips green at **166**, not 83. Self-retirement is preserved by a different route, stated in the function's own docstring: a drained corpus has `signed == enrolled` and the floor can never exceed `enrolled`, so the comparison is permanently satisfied without a removal commit. AC-28's literal phrase "capped at the backlog size" therefore now reads as the DRAINABLE set rather than the residual one; the AC text is unchanged and this row is the correction, per the append-only rule |
| DEV-5 | **Defect found and fixed during implementation: `_SECTION_HEADING_RE` over-matches, and the naive splitter bricked four enrolled stems' skeletons.** A repeated section id emitted a duplicate `sections` row, which `parse_extraction_artifact` refuses (`duplicate section`), so `make ze-rfc-extract` wrote an artifact that could not be read back. Each duplicate body also restarted `_sites_for`'s per-section counter at 1, so two different sentences both became site `7:1` and one was silently dropped | Reproduced 2026-07-29 by running the heading regex over every enrolled source: exactly four stems match an id more than once -- `rfc1195` (id `7`, 20 times), `rfc2865` (id `1`, twice), `rfc2869` (id `0`, 8 times) and `sflow-v5` (ids `1` through `6`, twice each). Fixed in `_section_bodies`: a repeated id EXTENDS the section it already opened instead of starting a second one, and the matched heading line's title stays in its body so a FALSE heading match never deletes a live obligation. Recorded because the dropped-site half failed in the worse direction (a false green), and because those four stems are the regression fixtures anyone touching that regex needs |
| DEV-6 | **Hazard: a live-corpus test contradicts the grandfathering principle its own sibling cites.** `TestRealTreeExtraction.test_the_machinery_accuses_nobody_with_zero_signoffs` asserts `R.extraction_stems() == set()` against the real tree, so it reds the moment anyone commits the first `rfc/extraction/*.json` -- including this spec's own phase-8 rfc7296 pilot | Reproduced 2026-07-29 by pointing `EXTRACTION_DIR` at a directory holding one artifact and re-running the class: that assertion is the single clean failure. Its sibling `test_preexisting_enrolment_without_signoff_passes` cites `ai/rules/rfc-compliance.md:114-116` for the rule this breaks -- "a rule that reds the gate on unrelated work gets removed rather than obeyed" -- and this assertion reds the gate on precisely the work the machinery exists to enable. `test_the_shipped_budget_parses_and_is_inert` is the same shape pinned to the shipped rate of 0 and reds on the arming commit; the reviewer counted both, and only the first reproduces as a first-sign-off red. Both should assert the PROPERTY (no stem is accused for lacking a sign-off it never had) rather than the corpus's current census. The cited line range has itself already drifted: `114-116` is now the keyword-visible-sites bullet, and the grandfathering paragraph sits below it |
| DEV-7 | **`make ze-verify-changed` did not run one of this diff's own tests, and could not have.** `scripts/dev/changed-pkgs.sh` collects `*.go` paths only, so a diff whose entire test surface is `scripts/dev/rfc_requirements_test.py` (1444 new lines) never puts `./scripts/dev` in the changed-package set, and `TestPythonUnitTests` -- the Go test that discovers and runs every `scripts/**/*_test.py` -- is therefore never selected | Reproduced 2026-07-29: `bash scripts/dev/changed-pkgs.sh` on this tree emits seven packages (`./cmd/ze`, `./cmd/ze/hub`, `./internal/chaos/mcp`, `./internal/chaos/orchestrator`, `./internal/component/mcp`, `./internal/test/cli`, `./internal/test/runner`), every one of them a CONCURRENT session's MCP work, and `./scripts/dev` is absent. The hole is general, not specific to this spec: any change to a Python tool under `scripts/` is invisible to scoped verification, which then reports green having tested none of it. `python3 scripts/dev/rfc_requirements.py --selftest` and a full `make ze-verify` do run these tests; only the scoped path does not. ~~**The fix belongs in `scripts/dev/changed-pkgs.sh` and is not made by this documentation pass**~~ **RESOLVED 2026-07-29, same day.** `PATHSPECS` is now `('*.go' '*.py' 'rfc')` and both `*.py` and `rfc/*` map to `PYTHON_TEST_PKG="./scripts/dev"` -- the one Go package that executes Python, since `python_tests_test.go` globs every `*_test.py` under its roots and a `test/scripts/*.py` has no Go package of its own (`scripts/dev/changed-pkgs.sh:42-90`). The comment at `:25-28` records the defect so the pathspec is not narrowed back. `scripts/dev/changed-pkgs.sh` is consequently in this diff and is hash-pinned by the review artifact. Fixed rather than recorded, per `ai/rules/fix-dont-record.md` -- the same rule DEV-3 exists to illustrate |
| DEV-8 | **Phase 8 and AC-23 to AC-26 were removed from this spec entirely, after phase 8a had run.** The spec was written to deliver machinery AND the rfc7296 pilot (owner ruling 2); it delivers machinery only | Owner ruling 3, 2026-07-29 (see Owner Rulings above). Phase 8a executed exactly as ruling 2 and the Failure Routing table required and returned **214 distinct obligations**, 108 of them NOT IMPLEMENTED. Taken to Thomas for scoping per R-10; he ruled twice: fix all 108 (OR-A), in a spec of its own (OR-B). **Not a scope reduction and not self-authorized**: the destination `plan/spec-rfcgate-1b-rfc7296-pilot.md` exists, carries all 214 rows, maps AC-23..AC-26 onto its own AC-1/5/4/3 (`:422-427`), and NARROWED the inherited AC-25 by removing its annotation escape. The move is a live deferral row with a real destination (`plan/deferrals/rfcgate-1-extraction.md:8`), never a `done` or a `cancelled`. Nothing under `rfc/short/rfc7296.md`, `rfc/extraction/`, `rfc/enrolled.txt`, `docs/features/rfc-status.md` or `internal/component/ike/` was touched, so no half-performed pilot is left behind |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | **Machinery (phases 1 to 7):** nothing user-visible and nothing on a wire path. The failure mode that matters is process, not product. Over-firing reds `ze-rfc-check`, which `scripts/status/verify_run.go:237,259` runs in BOTH verify branches, so it blocks every commit in the repo until fixed. Under-firing leaves the blind spot exactly where it is today while advertising that it is closed, which is worse than not shipping. **Pilot (phase 8):** this half CAN reach the wire. If a newly extracted RFC 7296 requirement is not already implemented, implementing it changes live IKEv2 behavior, so the blast radius is the IPsec data path and its interop surface, not a dev script |
| How is it reverted? | Two different answers now. The machinery is a single commit revert: the artifact directory `rfc/extraction/` is additive and orphaned by a revert, with no data migration, no config change and no state on any device. The pilot is NOT freely revertible: reverting an IKE conformance fix reintroduces a known wire-visible violation, which `ai/rules/rfc-compliance.md:25` forbids. A pilot defect is fixed forward |
| Who else touches this path? | `plan/spec-followup-rfc-enrollment.md` (owns `rfc/enrolled.txt` and the rollup), `plan/spec-rfcgate-0-umbrella.md` (owns the drain quota consuming `--extraction-status`, and the Sequencing Constraint that puts this spec first), `plan/spec-rfcgate-4-ledger.md` (arms a gate red by design on rfc1035, rfc3765, rfc4486 and rfc5301, and from the day this spec lands its D8 enrolment work carries four extraction sign-offs as a precondition; R-13), `/ze-rfc` and `/ze-rfc-audit` (skills whose steps change), the IKE engine and its maintainers (phase 8 only), and the open deferral row in `plan/deferrals/rfc-gate-regression-ratchets.md` (the audit backlog, deliberately NOT touched here) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rfc-check` on a tree where a stem is newly enrolled with no sign-off | → | `check_enrolment` precondition | `TestExtractionSignoffWiring.test_run_check_fails_on_new_enrolment_without_signoff` |
| `make ze-rfc-check` on a tree with an unclassified site in a sign-off | → | `check_extraction_signoff` via `run_check` | `TestExtractionSignoffWiring.test_run_check_fails_on_unclassified_site` |
| `make ze-rfc-check` on a tree whose HEAD sign-off was deleted | → | `check_extraction_ratchet` via `run_check` | `TestExtractionRatchetWiring.test_run_check_fails_when_a_signoff_disappears` |
| `make ze-rfc-extract STEM=<stem>` | → | `run_extract_skeleton` | `TestSkeletonWriter.test_skeleton_writes_every_site_unclassified` |
| ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Two spec corrections found by implementing it") | → | `run_extraction_status` | `TestExtractionStatus.test_json_envelope_carries_counts_and_registers` |
| `make ze-rfc-check` on a tree whose `rfc/drain-budget.txt` names a rate putting the floor above the signed count | → | `check_drain_floor` via `run_check` | `TestDrainFloorWiring.test_run_check_fails_when_signed_count_is_below_the_floor` |
| `make ze-rfc-check` with `rfc/drain-budget.txt` deleted | → | `check_drain_floor`'s fail-closed guard on the POLICY input, mirroring `check_enrolment:660-664` | `TestDrainFloor.test_missing_drain_budget_is_error_not_empty` |
| `make ze-rfc-index` then `make ze-rfc-check` | → | extraction table in `render_ledger`, guarded by `check_ledger_fresh:1578` | `TestExtractionLedger.test_stale_extraction_table_fails_check_fresh` |
| `make ze-rfc-index` on a tree with sign-offs in more than one register | → | per-register columns in `render_ledger` (ruling 1) | `TestExtractionLedger.test_registers_are_published_in_separate_columns` |
| ~~`make ze-rfc-check` on the tree after the rfc7296 pilot lands~~ | ~~→~~ | ~~`check_extraction_signoff` over `rfc/extraction/rfc7296.json`, 165 stems still unsigned~~ | ~~`TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered`~~ **MOVED 2026-07-29 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md` with AC-24. The machinery-side wiring it would have exercised is proved by `TestGrandfatheredBacklog.test_an_unsigned_backlog_is_accused_by_nothing` and `TestExtractionSignoffWiring.test_run_check_clean_on_a_fully_classified_signoff`, both delivered here** |

No `.ci` row for the machinery: it is developer tooling with no daemon surface, so a
functional `.ci` is N/A and the driving surface is the Python suite named in the Functional
Tests table below. Phase 8 is different in kind: its wiring proof is the RFC tag pair on a
real IKE test, and where a newly extracted requirement changes wire-visible behavior the
interop surface (`make ze-ipsec-interop-test`) is required, not a `.ci`.
Every wiring row above drives `run_check` / the CLI entry point through `_patched`
(`rfc_requirements_test.py:34`), never the helper alone, so a check that stops being called
fails here. Each has a discriminating twin listed in the TDD plan, following the file's own
convention (`test_run_check_clean_when_coverage_held`, `:1365`).

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A stem present in `rfc/enrolled.txt` now and absent at HEAD, with no `rfc/extraction/<stem>.json` | `make ze-rfc-check` exits 2 naming the stem and the missing sign-off. A stem enrolled at HEAD with no sign-off does NOT fail |
| AC-2 | `make ze-rfc-extract STEM=<stem>` on an RFC with N derived sites | `rfc/extraction/<stem>.json` lists all N sites with null dispositions, every section of the source, the derived register, `source-path` and `source-sha`. No disposition is invented |
| AC-3 | A sign-off with any site whose disposition is null | Exit 2, naming the site locator and its quote. Skeleton generation alone can never produce a passing sign-off |
| AC-4 | A sign-off whose `source-sha` differs from the current source text | Exit 2 naming the stem, on the same over-trigger bias `verdict_is_fresh:1231-1233` records: a false stale costs a re-read, a false fresh ships an unbounded summary |
| AC-5 | A site classified `mapped` to a requirement id that does not exist in `rfc/short/<stem>.md` | Exit 2 naming the site and the unknown id |
| AC-6 | A gated requirement in the summary that no site maps to and that no section lists in `unsourced-ids` | Exit 2: the summary asserts an obligation no source site backs. This is the reverse arithmetic that catches an invented requirement |
| AC-7 | A site classified `excluded` with no reason, an empty reason, or a kind outside the closed set | Exit 2 with a parse error naming the site, mirroring `_parse_annotation:204-238` |
| AC-8 | A site excluded `duplicate-of` naming an id that no other site maps | Exit 2. A chain of duplicates cannot cover an RFC in which nothing is actually mapped |
| AC-9 | A source with no capitalised MUST-level keyword outside the RFC 2119 boilerplate, and an artifact claiming `register: rfc2119` | Exit 2: the claimed register is stronger than the derived one. The artifact may claim the derived register or a weaker one, never a stronger one |
| AC-10 | An enrolled RFC whose derived inventory is empty under BOTH registers while its summary declares at least one gated requirement | Exit 2 unless the artifact carries a `manual-walk` sign-off with a complete section walk, a reviewer and a stated reason why no mechanical inventory exists. The gate says it cannot evaluate rather than passing |
| AC-11 | A `manual-walk` or `prose` sign-off | It counts toward the signed total, and therefore toward the umbrella's drain quota, exactly as an `rfc2119` sign-off does (owner ruling 1). It is ALSO published in its own register column, so `N signed off` can never be read as `N keyword-verified`. Both halves are required: crediting without the split launders an assertion, and splitting without crediting leaves a third of the corpus undrainable |
| AC-12 | A stem with a valid sign-off at HEAD and none now | Exit 2: extraction sign-off is monotonic, matching `check_enrolment:665-669`'s wording and reasoning |
| AC-13 | A stem whose exclusion count is higher than at HEAD, with no `resign-reason` | Exit 2. With a `resign-reason` plus a bumped `signed-off` date and reviewer, it passes |
| AC-14 | Any tree | `make ze-rfc-check`'s success line reports signed-off count, enrolled count, and the grandfathered backlog count. Zero signed off is stated out loud, never silent |
| AC-15 | `make ze-rfc-index` | `ai/RFC-REQUIREMENTS.md` carries an extraction table: per enrolled RFC the register, site count, mapped, excluded, exclusion ratio, and sign-off date or `UNSIGNED (grandfathered)`. A stale table fails `--check-fresh` through the existing `check_ledger_fresh:1578` |
| AC-16 | ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Two spec corrections found by implementing it") | A JSON envelope with `schema-version`, signed and enrolled counts, per-register counts, and the unsigned backlog list. Lower kebab-case keys per `ai/rules/json-format.md` |
| AC-17 | Git unavailable or `rfc/extraction/` absent at HEAD | The ratchet judges nothing and says so; it never accuses every stem of having lost a sign-off. The empty-versus-None distinction `_git_baseline_summary_stems:763-776` documents is restated for this consumer's polarity |
| AC-18 | A malformed or unreadable `rfc/extraction/<stem>.json` | Exit 2 with a clean `cannot run` message through the existing handler (`run_check:1688-1694`), never an uncaught traceback |
| AC-19 | The tree as it stands when the MACHINERY lands (166 enrolled, zero sign-offs, phases 1 to 7 complete) | `make ze-rfc-check` exits 0. Grandfathering is scope, not an allowlist file |
| AC-20 | An artifact whose derived field (`quote`, per-section site count, register) was hand-edited away from what the source re-derives | Exit 2 naming the field and the locator |
| AC-21 | `make ze-rfc-index` on a tree carrying sign-offs in more than one register | The extraction table renders one column per register (`rfc2119`, `prose`, `manual-walk`) with its own signed count. A signed total rendered WITHOUT the three component columns beside it is a failure of this AC, not a formatting preference (owner ruling 1's mandatory counterweight) |
| AC-22 | ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Two spec corrections found by implementing it") on the same tree | The envelope carries `signed-by-register` with a key per register; the keys sum to the published signed total; and no register is excluded from that total. A consumer can compute quota credit and read evidence strength from the same document without either being inferred |
| ~~AC-23~~ | ~~`rfc/short/rfc7296.md` after phase 8~~ | ~~It carries `RFC7296-2.2-1` and `RFC7296-2.2-2`, each quoting its RFC text verbatim and citing `(§2.2)`, so the id's section matches its citation as `make ze-rfc-check` already requires. At minimum these two; the walk's other findings are additional, never a substitute~~ **MOVED 2026-07-29 (owner ruling 3, OR-B) to `plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-1, carried verbatim in substance (`:424`). NOT dropped: `plan/deferrals/rfcgate-1-extraction.md:8` homes it, Status `deferred`** |
| ~~AC-24~~ | ~~The tree after phase 8 (166 enrolled, exactly one sign-off)~~ | ~~`make ze-rfc-check` exits 0, `rfc/extraction/rfc7296.json` validates with every site and section classified and its register derived, and the other 165 stems remain unsigned and unaccused. A non-empty artifact set changes nothing for a stem that has no artifact~~ **MOVED 2026-07-29 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-5, no substantive change (`:425`). The half of it that belongs to the MACHINERY -- green with zero artifacts -- is AC-19 and is delivered and audited here** |
| ~~AC-25~~ | ~~Any requirement the phase-8 walk newly extracts~~ | ~~It carries a positive AND a negative `RFC requirement:` tagged test, OR an annotation whose authorisation by Thomas is recorded in this spec's Deviations with the date, the requirement id and his answer. An annotation present WITHOUT that record fails this AC. Nothing in this spec pre-authorises an annotation~~ **MOVED 2026-07-29 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-4, and NARROWED there (`:426`): OR-1 removes the annotation branch entirely, so the only surviving path to a non-proven row is Thomas's recorded answer to the escalation. The obligation got stricter in transit** |
| ~~AC-26~~ | ~~`rfc/short/rfc7296.md` before and after phase 8~~ | ~~Every one of the 23 existing ids is still present and still carries at least the polarities it held at HEAD. `check_retired_requirements` and `check_coverage_ratchet` stay green across the re-authoring, and no id is renumbered or reused~~ **MOVED 2026-07-29 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-3, no substantive change (`:427`). Vacuously true here: this spec never edited `rfc/short/rfc7296.md`, which `git status` confirms unmodified** |
| AC-27 | `rfc/drain-budget.txt` carrying the shipped rate of `0`, any signed count | The required floor computes to 0, `check_drain_floor` passes, and `make ze-rfc-check` exits 0 while the backlog is still published. The comparison ships INERT by design (umbrella D5); its first real exercise is the arming commit, which is Thomas's. Satisfies the umbrella's AC-13 |
| AC-28 | A budget whose rate times the calendar months elapsed since `start` exceeds the current backlog size | The floor is CAPPED at the backlog size and never exceeds it, so the check goes permanently green once the backlog is drained and needs no removal commit. Satisfies the umbrella's self-retirement row and its Required-floor boundary |
| AC-29 | `rfc/drain-budget.txt` missing, empty, or unparseable | Exit 2 naming the file. It does NOT compute a floor of 0 and report nothing owed: an absent policy is the zero-value trap `ai/rules/fail-closed-guards.md` names, and the guard is the same shape as `check_enrolment:660-664`, which refuses to report clean while enforcing nothing. The ARTIFACT side needs no such guard because its polarity is already safe: an absent `rfc/extraction/` yields zero signed stems, so the backlog reads at its maximum. Satisfies the umbrella's AC-4 |
| AC-30 | A budget with a non-zero rate whose computed floor exceeds the signed count | Exit 2 naming the floor, the signed count, the start date and the rate. This is the discriminating case: a floor that can never fail is the vacuity this spec exists to remove. Every register counts toward the signed count it compares (AC-11), so no RFC is undrainable because of the register its own authors wrote in |
| AC-31 | An artifact whose `register` is missing, empty, or outside `rfc2119` / `prose` / `manual-walk` | Exit 2 naming the artifact and the offending value, as a `ParseError` in the same shape as `_parse_annotation:213-217`. It does not default to the strong grade and does not silently drop the artifact. Satisfies the umbrella's AC-17, first arm |
| AC-32 | An artifact claiming `register: rfc2119` over a source that HAS capitalised keywords but whose derived site count is below the declared gated-requirement count | Exit 2: the derivation grades that source `prose` under the undercount clause, so the claim is stronger than the derivation supports. AC-9 covers the keyword-free arm of the same rule; this is the undercount arm, and it is the arm the rfc2181 shape lands in. Satisfies the umbrella's AC-17, second arm |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Signs off the extraction of a new RFC before enrolling it | `make ze-rfc-extract STEM=x` → classify every site → `make ze-rfc-check` red until complete → add stem to `rfc/enrolled.txt` → green | `TestExtractionSignoffWiring.test_run_check_fails_on_new_enrolment_without_signoff` plus its clean twin |
| 2 | Reads how much of the standards claim is bounded | `make ze-rfc-index` → extraction table in `ai/RFC-REQUIREMENTS.md` | `TestExtractionLedger.test_table_reports_unsigned_backlog` |
| 3 | Drains one RFC from the grandfathered backlog | classify → `make ze-rfc-check` green → signed count rises and cannot fall again | `TestExtractionRatchet.test_signoff_count_is_monotonic` |
| 4 | Runs the umbrella's per-release drain check | ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; see "Two spec corrections found by implementing it") → counts consumed by the umbrella's quota gate | `TestExtractionStatus.test_json_envelope_carries_counts_and_registers` |
| 5 | Drains a pre-2119 RFC and gets credit for it without overstating the evidence | classify under the derived `prose` or `manual-walk` register → signed total rises by one → the ledger shows which column it landed in | `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` plus `TestExtractionLedger.test_registers_are_published_in_separate_columns` |
| ~~6~~ | ~~Reads what the gate was blind to in rfc7296 and finds it discharged~~ | ~~`rfc/short/rfc7296.md` §2.2 rows → tagged tests via `ai/RFC-REQUIREMENTS.md` → `rfc/extraction/rfc7296.json` showing every §2.2 site mapped~~ | ~~`TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered`~~ **MOVED 2026-07-29 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md` with AC-23 to AC-26. Stories 1 to 5 are this spec's and are audited below** |

## 🧪 TDD Test Plan

All tests live in `scripts/dev/rfc_requirements_test.py`, auto-discovered by
`scripts/dev/python_tests_test.go` and run by `--selftest` before `--check` judges the live
tree (`Makefile:438`). Existing helpers reused rather than duplicated: `_load:21`,
`_patched:34`, `_run_capturing:51`, `_req:479`, `_tag:492`, `_FakeSubprocess:1293`.
Fixtures extended rather than replaced: `TestEnrolment:603` gains the sign-off precondition
rows beside its existing source-text rows; `TestRealTree:1813` gains live-tree assertions;
`TestLedgerStaleness:1696` gains the extraction-table staleness case.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSiteInventory.test_sites_are_attributed_to_enclosing_section` | `scripts/dev/rfc_requirements_test.py` | locator `<section>:<n>` derived in document order | |
| `TestSiteInventory.test_rfc2119_boilerplate_is_not_a_site` | same | the "key words ... interpreted as described" sentence never becomes a site | |
| `TestSiteInventory.test_register_is_derived_not_authored` | same | keyword-free source derives `prose`; keyword-rich source derives `rfc2119` (AC-9) | |
| `TestSiteInventory.test_register_falls_to_prose_when_sites_undercount_declared` | same | the rfc2181 shape: 1 keyword site, 23 gated, derives `prose` (A-2) | |
| `TestSiteInventory.test_prose_register_inventory_is_not_empty` | same | case-insensitive modal scan yields sites for a keyword-free fixture (A-4) | |
| `TestExtractionArtifact.test_missing_reason_on_exclusion_fails` | same | AC-7, mirroring `_parse_annotation:204-238` | |
| `TestExtractionArtifact.test_unknown_exclusion_kind_fails` | same | AC-7, closed set | |
| `TestExtractionArtifact.test_duplicate_of_must_name_a_mapped_id` | same | AC-8 | |
| `TestExtractionArtifact.test_unreadable_artifact_raises_parse_error` | same | AC-18 | |
| `TestExtractionSignoff.test_unclassified_site_fails` | same | AC-3 | |
| `TestExtractionSignoff.test_fully_classified_signoff_passes` | same | discriminating twin: proves the check is not "always fails" | |
| `TestExtractionSignoff.test_source_sha_mismatch_fails` | same | AC-4 | |
| `TestExtractionSignoff.test_mapped_to_unknown_requirement_fails` | same | AC-5 | |
| `TestExtractionSignoff.test_gated_requirement_with_no_site_fails` | same | AC-6, reverse arithmetic | |
| `TestExtractionSignoff.test_unsourced_id_recorded_on_a_section_passes` | same | AC-6 escape for indicative prose (the RFC 4271 §8.2.2 shape, R-7) | |
| `TestExtractionSignoff.test_hand_edited_quote_fails` | same | AC-20 | |
| `TestPre2119FailsClosed.test_keyword_free_source_cannot_claim_rfc2119` | same | AC-9, the fail-open this spec exists to close | |
| `TestPre2119FailsClosed.test_rfc2119_register_below_source_count_is_rejected` | same | AC-32: the undercount arm of the same refusal (the rfc2181 shape). Name carried over from the umbrella's Unit Tests table, which had no counterpart here | |
| `TestExtractionArtifact.test_unknown_register_is_hard_error` | same | AC-31: a missing, empty or unknown `register` is a `ParseError`, never a silent default to the strong grade. Name carried over from the umbrella | |
| `TestPre2119FailsClosed.test_empty_inventory_with_gated_musts_is_refused` | same | AC-10, the guard says it cannot evaluate | |
| `TestPre2119FailsClosed.test_manual_walk_is_published_in_its_own_column` | same | AC-11, the counterweight half of owner ruling 1 | |
| `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` | same | AC-11, the credit half: an artifact in each register raises the signed total by one apiece | |
| `TestExtractionStatus.test_signed_by_register_sums_to_total` | same | AC-22, and its discriminating twin: a register silently dropped from the total makes the sum disagree | |
| `TestExtractionLedger.test_registers_are_published_in_separate_columns` | same | AC-21: a rendered signed total with no register split beside it fails | |
| `TestDrainFloor.test_rate_zero_computes_a_zero_floor` | same | AC-27 (umbrella AC-13): the comparison ships inert, and the backlog is still published beside it | |
| `TestDrainFloor.test_drain_floor_caps_at_backlog_size` | same | AC-28 and self-retirement: the floor never exceeds the backlog. Name carried over from the umbrella's Unit Tests table, which had no counterpart here | |
| `TestDrainFloor.test_missing_drain_budget_is_error_not_empty` | same | AC-29 (umbrella AC-4): the zero-value trap closed on the POLICY input. Name carried over from the umbrella | |
| `TestDrainFloor.test_signed_below_armed_floor_fails` | same | AC-30, and the discriminating twin of the three rows above: with the rate armed, a signed count under the floor reds. Without this the whole floor could ship unable to fail | |
| `TestExtractionRatchet.test_signoff_count_is_monotonic` | same | AC-12 | |
| `TestExtractionRatchet.test_exclusions_are_shrink_only` | same | AC-13, R-1 | |
| `TestExtractionRatchet.test_resign_reason_permits_a_rise` | same | AC-13 twin | |
| `TestExtractionRatchet.test_git_failure_judges_nothing` | same | AC-17, driven through `_FakeSubprocess:1293` with plausible non-empty output | |
| `TestSkeletonWriter.test_skeleton_writes_every_site_unclassified` | same | AC-2, R-2: generation cannot produce a pass | |
| `TestSkeletonWriter.test_skeleton_refresh_preserves_existing_classifications` | same | a re-run does not silently discard a reviewer's work (`ai/rules/never-destroy-work.md`) | |
| `TestExtractionStatus.test_json_envelope_carries_counts_and_registers` | same | AC-16 | |
| `TestExtractionLedger.test_table_reports_unsigned_backlog` | same | AC-15 | |
| `TestExtractionLedger.test_stale_extraction_table_fails_check_fresh` | same | AC-15 through the existing freshness gate | |
| `TestEnrolment.test_new_enrolment_without_signoff_fails` | same (extends `:603`) | AC-1 | |
| `TestEnrolment.test_preexisting_enrolment_without_signoff_passes` | same (extends `:603`) | AC-19, grandfathering | |
| `TestRealTree.test_every_enrolled_rfc_derives_a_register` | same (extends `:1813`) | A-2, A-3 on the live tree | |
| `TestRealTree.test_live_tree_is_green_with_zero_signoffs` | same (extends `:1813`) | AC-19, the machinery landing | |
| ~~`TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered`~~ | ~~same (extends `:1813`)~~ | ~~AC-24: the live tree is green with exactly one artifact, and the 165 unsigned stems are not accused~~ **MOVED 2026-07-29 with AC-24 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md`. Never written here; `grep -c` in `scripts/dev/rfc_requirements_test.py` returns 0** | |
| ~~`TestRealTree.test_rfc7296_summary_carries_the_section_2_2_requirements`~~ | ~~same (extends `:1813`)~~ | ~~AC-23: `RFC7296-2.2-1` and `RFC7296-2.2-2` exist, cite `(§2.2)`, and their text matches the source sentences at `rfc/full/rfc7296.txt:1397` and `:1439`~~ **MOVED 2026-07-29 with AC-23 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md`** | |
| ~~`TestRealTree.test_rfc7296_new_requirements_are_proven_or_authorised`~~ | ~~same (extends `:1813`)~~ | ~~AC-25: every `RFC7296-*` id added since HEAD carries a positive and a negative tag, or an annotation. The test proves the mechanical half; the Deviations record proves the authorisation half, which no test can check~~ **MOVED 2026-07-29 with AC-25 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md`, where OR-1 removed the annotation branch the second clause allowed** | |
| ~~`TestRealTree.test_rfc7296_ids_are_neither_retired_nor_demoted`~~ | ~~same (extends `:1813`)~~ | ~~AC-26, driving `check_retired_requirements` and `check_coverage_ratchet` across the re-authoring~~ **MOVED 2026-07-29 with AC-26 (owner ruling 3) to `plan/spec-rfcgate-1b-rfc7296-pilot.md`** | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| site ordinal in a locator `<section>:<n>` | 1..sites-in-section | sites-in-section | 0 | sites-in-section + 1 |
| `schema-version` | 1..1 | 1 | 0 | 2 |
| exclusion count vs HEAD baseline | 0..baseline | baseline | N-A (any decrease is legal) | baseline + 1 without `resign-reason` |
| signed-off stem count vs HEAD baseline | baseline..enrolled | enrolled | baseline - 1 | N-A (a rise is the point) |
| derived sites for an enrolled RFC declaring gated MUSTs | 1..N | 1 | 0 (refused unless `manual-walk`) | N-A |
| drain rate in `rfc/drain-budget.txt` (entries per calendar month) | 0..166 | 166 | negative | 167, which exceeds the whole enrolled set |
| required floor computed by `check_drain_floor` | 0..backlog size | backlog size | negative | backlog size + 1, which must be capped rather than reached (AC-28) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the whole suite above | `scripts/dev/rfc_requirements_test.py` | a maintainer runs `make ze-rfc-check` and the gate behaves as specified; run by `--selftest` before `--check`, and by `go test` through `scripts/dev/python_tests_test.go` | |
| `make ze-rfc-check` on the live tree | `Makefile:437` | the 166 enrolled RFCs stay green when the machinery lands (AC-19) | |
| `make ze-rfc-check` on the live tree after the pilot | `Makefile:437` | green with exactly one sign-off present and 165 unsigned (AC-24) | |
| `make ze-unit-test` over the IKE packages | `internal/component/ike/...` | the tagged tests proving each newly extracted RFC 7296 requirement (AC-25) | |
| `make ze-ipsec-interop-test` | `test/ipsec-interop/` | required only where a newly extracted requirement changes wire-visible IKEv2 behavior, per `ai/rules/interop-and-goal-validation.md`; whether it does is settled by the phase-8 walk | |

No `.ci` functional test for the machinery half: it touches no daemon Go under `internal/`,
`pkg/` or `cmd/`, so there is no daemon surface for a `.ci` to drive, and the Python suite
is the driving surface, per the same convention `scripts/dev/dep_audit.py` and
`scripts/dev/migrate_module.py` follow. The pilot half is judged by a different standard:
its proof is the `RFC requirement:` tag pair on a real IKE test, and its interop obligation
is the strongSwan lab, neither of which a `.ci` would add anything to.

### Interop Tests (Scope: protocol)
N-A for the machinery: no wire-visible behavior changes. NOT automatically N-A for the
pilot. `ai/rules/interop-and-goal-validation.md` requires an interop test whenever protocol
behavior is implemented or changed, and the IKEv2 lab is `test/ipsec-interop/`
(strongSwan, `make ze-ipsec-interop-test`). Whether phase 8 changes wire behavior at all is
determined by reading the producing code during the walk (A-11), so the obligation is
recorded here as CONDITIONAL and resolved with evidence, never assumed away in advance.

## Files to Modify
- `scripts/dev/rfc_requirements.py` - site and section derivation, register derivation,
  artifact parse, `check_extraction_signoff`, `check_extraction_ratchet`,
  `check_drain_floor` (the floor comparison, resolved 2026-07-29: it reads the umbrella's
  policy and this spec's derived signed count), the `check_enrolment` precondition, the
  ledger extraction table, two new CLI modes, and the success-line counts
- `scripts/dev/rfc_requirements_test.py` - the classes and fixture extensions above
- `Makefile` - `ze-rfc-extract` and `ze-rfc-extraction-status`, declared beside the existing
  `ze-rfc-check` (`Makefile:437`) and `ze-rfc-index` (`:442`); `ze-rfc-check` itself
  unchanged. Verified 2026-07-29: both existing RFC targets live in `Makefile` and neither
  appears anywhere in `mk/`, so `mk/inventory.mk` is NOT the home for these two and needs no
  entry (its "Quick reference" header lists only targets declared in that file). The
  discovery surface is `ai/INDEX.md`, next
- `ai/RFC-REQUIREMENTS.md` - regenerated; gains the extraction table (generated, never
  hand-edited)
- `ai/rules/rfc-compliance.md` - "the four ratchets" becomes five; "Extraction
  Completeness" gains the sign-off procedure and its grandfathering scope
- `ai/skills/ze-rfc.md` - step 6 and the "Coverage self-check (BLOCKING)" block at `:43-54`
  are replaced by the recorded sign-off; canonical source, so `make ze-ai-sync` regenerates
  `.claude/skills/`, `.codex/skills/` and `.agents/skills/` (`ai/rules/canonical-sources.md`)
- `ai/INDEX.md` - Dev Tools rows near `:212` for the two new targets, and the RFC keyword
  row at `:372`
- `docs/contributing/rfc-implementation-guide.md` - the sign-off step in the authoring flow

**Phase 8 (the rfc7296 pilot) additionally modifies:**
- `rfc/short/rfc7296.md` - re-authored against `rfc/full/rfc7296.txt` per
  `ai/skills/ze-rfc.md`; gains `RFC7296-2.2-1`, `RFC7296-2.2-2` and every other row the walk
  produces. Existing ids keep their numbers and their proof (AC-26)
- `rfc/enrolled.txt` - the rfc7296 descriptor on line 159 only. No stem is added or removed
  by this spec, and the file stays `line.split()[0]`-parseable
- `docs/features/rfc-status.md` - the rfc7296 row at `:213`, whose two disclosed MUST gaps
  and Implemented-coverage prose move with the re-authored summary
- `internal/component/ike/**` (engine, wire, crypto) - the tagged tests for every newly
  extracted requirement, plus any implementation a requirement turns out to need. The exact
  set is NOT enumerable before phase 8a produces the real count (A-10, R-10); naming a
  guessed list here would be the fabrication this spec exists to remove

## Files to Create
- `rfc/extraction/README.md` - what the artifact is, what it is not, and the explicit
  statement that a `manual-walk` sign-off is an assertion the gate cannot verify
- `plan/deferrals/rfcgate-1-extraction.md` - the shard named in the metadata table
- `rfc/extraction/rfc7296.json` - the pilot sign-off (phase 8 only)
- `rfc/drain-budget.txt` - the drain POLICY the umbrella's quota reads: a `start` date and
  a `rate` in entries per calendar month, shipping at `0` (umbrella D5). Two fields, no
  per-stem row ever. Added by the 2026-07-29 resolution of R-14

~~The first `rfc/extraction/<stem>.json` is created by whoever performs the first sign-off,
which is out of scope here. No artifact ships with this spec, by design: shipping one
would be the mass-generation failure R-2 exists to prevent, in miniature.~~
(Superseded 2026-07-29 by owner ruling 2.) Exactly ONE artifact ships with this spec,
`rfc/extraction/rfc7296.json`, and it is hand-classified site by site as the pilot of the
format. R-2 is untouched by this: what R-2 forbids is MASS generation of unearned
sign-offs, and its defence is structural rather than numeric (a generated skeleton can only
be UNCLASSIFIED, and an unclassified site fails the check), so one genuinely performed walk
is the opposite of the failure mode, not an exception to it. The remaining 165 are created
by whoever performs them, which stays out of scope here.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | developer tooling; nothing reaches the config tree |
| YANG validation constraints | No | as above |
| YANG custom validators | No | as above |
| CLI commands/flags | No | `ze` binary untouched; the new flags are on a dev script, not on `ze`, so `ai/rules/cli-grammar.md` does not apply |
| CLI grammar (keyword before value) | No | as above |
| Editor autocomplete | No | as above |
| Functional test for new RPC/API | No | no RPC or API added |
| Pipe completeness | No | no `ze` command output |
| Env var registration | No | no new env var |
| Doctor check for runtime dependencies | No | the new file path is repo content read by a dev script, not a daemon runtime dependency; `ze doctor` never reads it |
| Prometheus counters/metrics | No | dev tooling |
| BGP family surface (new SAFI / capability / attribute) | No | no protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | developer tooling; `docs/features.md` describes product features |
| 2 | Config syntax changed? | No | no config surface |
| 3 | CLI command added/changed? | No | no `ze` subcommand; the new entry points are make targets on a dev script |
| 4 | API/RPC added/changed? | No | none added |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | contributor-facing, covered by row 10 |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | ~~No; no protocol behavior changes~~ (superseded 2026-07-29 by owner ruling 2). Phase 8 newly PROVES at least `RFC7296-2.2-1` and `RFC7296-2.2-2`, and may implement them. Update `docs/features/rfc-status.md:213` (the rfc7296 row: Status, Implemented coverage, and the Remaining column that today discloses exactly two MUST gaps) with a source anchor to the producing `file:line`, and reconcile the `rfc/enrolled.txt:159` descriptor with the re-authored summary. The machinery half still changes no protocol behavior; the other 165 sign-offs remain owned by the spec that performs them |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (the RFC Requirement Tags section) and `docs/contributing/rfc-implementation-guide.md` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares product capability |
| 12 | Internal architecture changed? | No | no daemon architecture change |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` and `ai/` for anchors naming `scripts/dev/rfc_requirements.py`, `ze-rfc-check` and `ze-rfc-index`; `ai/INDEX.md:212-213` and `ai/rules/rfc-compliance.md:107-112` are already known to name them |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/skills/ze-rfc.md:43-54` shows the two-grep coverage self-check this spec replaces; update it or it teaches the superseded method |

Rows 1, 7 and 11 are answered No for the machinery and are RE-ANSWERED by phase 8 with
evidence before closure: whether a newly extracted RFC 7296 obligation turns out to be a
user-visible feature (row 1), a wire-format change (row 7), or a shift in daemon comparison
(row 11) is decided by the walk, not predictable now. An unchanged No on any of those three
after phase 8 must be backed by the grep or the read that established it, not by this
paragraph.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- entry points exist and the wiring tests fail
   because the feature is a stub.
   - Tests: every row of the Wiring Test table, written first and failing
   - Files: `scripts/dev/rfc_requirements.py` (stub `check_extraction_signoff`,
     `check_extraction_ratchet`, `run_extract_skeleton`, `run_extraction_status`, all
     returning empty/no-op), `Makefile` (both targets), `scripts/dev/rfc_requirements_test.py`
   - Verify: `run_check` calls both stubs and `--extract-skeleton` / `--extraction-status`
     dispatch from `main:1754`; the wiring tests fail on behavior, not on import errors
2. **Phase: Derivation** -- site inventory, section inventory, register selection.
   - Tests: `TestSiteInventory.*`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: the register split over the live tree is re-derived and recorded; A-2, A-3,
     A-4, A-5 and A-6 move to `confirmed` or `broken` before any check is written on top
3. **Phase: Artifact** -- parse, validate, closed enums, skeleton writer.
   - Tests: `TestExtractionArtifact.*`, `TestSkeletonWriter.*`
   - Files: `scripts/dev/rfc_requirements.py`, `rfc/extraction/README.md`
   - Verify: a generated skeleton FAILS the check (R-2 is structural before any policy
     text is written about it)
4. **Phase: Forward and reverse arithmetic** -- `check_extraction_signoff`.
   - Tests: `TestExtractionSignoff.*`, `TestPre2119FailsClosed.*`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: AC-3 through AC-11, plus AC-31 and AC-32 (the two register-validation arms of
     the umbrella's AC-17); each red test's discriminating twin passes
5. **Phase: Ratchets and enrolment precondition**.
   - Tests: `TestExtractionRatchet.*`, `TestEnrolment.*`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: AC-1, AC-12, AC-13, AC-17, AC-19; `make ze-rfc-check` still green on the live
     tree with zero sign-offs
6. **Phase: Publication, status, and the floor comparison**.
   - Tests: `TestExtractionLedger.*`, `TestExtractionStatus.*`, `TestDrainFloor.*`,
     `TestDrainFloorWiring.*`, `TestRealTree.*`
   - Files: `scripts/dev/rfc_requirements.py`, `ai/RFC-REQUIREMENTS.md` (regenerated),
     `rfc/drain-budget.txt`
   - Verify: AC-14, AC-15, AC-16, AC-21, AC-22, AC-27, AC-28, AC-29, AC-30; the ledger regenerates byte-stably twice in
     a row. Owner ruling 1 is a gate here, not a later polish: a signed total rendered or
     emitted without its per-register split is a failing phase, and the credit half is
     proved by an artifact in each register raising the total by one apiece
7. **Phase: Discovery surfaces** (`ai/rules/discovery-updates.md`).
   - Tests: `make ze-doc-test`, `make ze-doc-links`, `make ze-verify-wiring-docs`
   - Files: `ai/rules/rfc-compliance.md`, `ai/skills/ze-rfc.md`, `ai/INDEX.md`,
     `docs/functional-tests.md`, `docs/contributing/rfc-implementation-guide.md`
   - Verify: `make ze-ai-sync` regenerates the three skill trees; the ratchet table reads
     five, not four; timing delta for `ze-rfc-check` recorded against A-9
8. ~~**Phase: The rfc7296 pilot** (owner ruling 2). This phase is the only one that touches
   protocol content, and it is not optional: two CONFIRMED unextracted MUSTs escape the
   grandfather, and the finder is the entry point (`ai/rules/no-parking.md`).~~

   **MOVED 2026-07-29 in full (8a to 8e) by owner ruling 3 to
   `plan/spec-rfcgate-1b-rfc7296-pilot.md`.** Step 8a RAN and is what produced the ruling:
   the walk read 289 MUST/MUST NOT/SHALL keyword lines in context across all 92 numbered
   sections, collapsed 43 restatement sites, judged 8 non-normative, and returned **214
   distinct obligations** (63 `implemented-and-testable`, 25 `implemented-untested`, 108 NOT
   IMPLEMENTED, 18 `uncertain`) against the 18 gated MUST rows the summary holds today. That
   count went to Thomas exactly as the Failure Routing table requires, and he ruled: fix all
   108, and do it in a spec of its own. Steps 8b to 8e were NOT performed here -- nothing
   under `rfc/short/rfc7296.md`, `rfc/extraction/rfc7296.json`, `rfc/enrolled.txt`,
   `docs/features/rfc-status.md` or `internal/component/ike/` was touched, which `git
   status` confirms. The 214 rows are carried in the destination spec's Appendix A and the
   move is recorded at `plan/deferrals/rfcgate-1-extraction.md:8`, Status `deferred`.
   The original step text stands below UNCHANGED rather than struck, because it is exactly
   what the destination spec inherited and rewriting it here would lose the provenance. Read
   every one of 8a to 8e as `plan/spec-rfcgate-1b-rfc7296-pilot.md`'s work, not this spec's.

   - 8a. **Walk and count.** Re-author `rfc/short/rfc7296.md` against
     `rfc/full/rfc7296.txt` section by section per `ai/skills/ze-rfc.md`, across all 92
     numbered sections rather than the 14 currently anchored. Produce the real count of
     newly extracted obligations and STOP there. Take that count to the owner for scoping
     before writing any implementation (R-10, A-10). No id is renumbered or reused; a new
     obligation takes the next free ordinal in its own section.
   - 8b. **The floor.** Extract at minimum `RFC7296-2.2-1` (`rfc/full/rfc7296.txt:1397`,
     "Retransmission of a message MUST use the same Message ID as the original message")
     and `RFC7296-2.2-2` (`:1439`, "In the unlikely event that Message IDs grow too large
     to fit in 32 bits, the IKE SA MUST be closed or rekeyed"), both quoted verbatim and
     cited `(§2.2)`. These are the floor of the phase, never its ceiling.
   - 8c. **Prove or escalate, per requirement.** For each newly extracted requirement,
     apply the escalation rule in "The rfc7296 pilot" under Design: read the producing
     function, implement what is missing, and land a positive AND a negative
     `RFC requirement:` tagged test. Anything less goes to Thomas with the four items that
     rule names. The commit that adds a row is the commit that proves it (R-11).
   - 8d. **Sign off.** `make ze-rfc-extract STEM=rfc7296`, then classify every derived site
     and section by hand into `rfc/extraction/rfc7296.json`. This is the pilot: the format
     is being proved against the worst-measured input in the corpus, so a field that turns
     out unusable here is a defect in phases 3 and 4 to fix, not a field to work around.
   - 8e. **Reconcile the published claims.** `make ze-rfc-index`, then update
     `docs/features/rfc-status.md:213` and the `rfc/enrolled.txt:159` descriptor to match
     the re-authored summary, with source anchors to producing `file:line`.
   - Tests: `TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered`,
     `TestRealTree.test_rfc7296_summary_carries_the_section_2_2_requirements`,
     `TestRealTree.test_rfc7296_new_requirements_are_proven_or_authorised`,
     `TestRealTree.test_rfc7296_ids_are_neither_retired_nor_demoted`, plus the IKE unit
     tests carrying the new tags
   - Files: `rfc/short/rfc7296.md`, `rfc/extraction/rfc7296.json`, `rfc/enrolled.txt:159`,
     `docs/features/rfc-status.md:213`, `ai/RFC-REQUIREMENTS.md`, `internal/component/ike/**`
   - Verify: AC-23, AC-24, AC-25, AC-26; `make ze-rfc-check` exits 0 with exactly one
     artifact and 165 unsigned stems; `make ze-verify` passes; and
     `make ze-ipsec-interop-test` wherever the walk changed wire-visible behavior

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line` in `scripts/dev/rfc_requirements.py` |
| Feature completeness | Every user story has a working path; the drain-status envelope is actually emitted, not just designed |
| Fail-closed | Each new check denies on a miss, an unmapped input, an empty set and an error. Specifically: an empty site inventory, an absent artifact for a new enrolment, a git failure, and an unreadable artifact are each judged explicitly and none falls through to the permissive branch (`ai/rules/fail-closed-guards.md`) |
| Vacuity | For each new check, disable the producing code and confirm the test flips red. A check that cannot fail is the exact failure being fixed |
| Correctness | Register derivation reads the source text and nothing else; a stronger claimed register is refused and a weaker one is allowed and recorded |
| Naming | JSON keys lower kebab-case with `schema-version` in the envelope (`ai/rules/json-format.md`); closed enums mirror `ANNOTATION_KINDS:77` |
| Data flow | The source text is read only through the two-location lookup `source_keyword_count:1329` uses; no second git reader and no second freshness mechanism is introduced |
| Derive, never hardcode | `sites`, `sections`, `quote`, `register` and every published count are derived at check time; only dispositions and reasons are authored (`ai/rules/derive-not-hardcode.md`) |
| Registration over hardcoding | No per-RFC branch, no per-RFC allowlist file, no stem named in code. Grandfathering is scope (new-since-HEAD), never a list |
| No fabrication | Every claim in the learned summary cites the producing function; the corrected RFC 4271 §8.2.2 figure below is carried forward, not the brief's |
| Rule: `ai/rules/no-parking.md` | Any defect this work surfaces in the existing gate is fixed here, not recorded. The rfc7296 obligations are the worked example: they were found, so they are fixed here |
| Rule: `ai/rules/rfc-compliance.md` | If a sign-off design decision would leave an obligation unextracted or unproven, STOP and ask Thomas rather than choosing the narrower option |
| Drain floor ownership | The floor COMPARISON lives here (`check_drain_floor`); the POLICY it reads (start date, rate, cadence semantics, the value of N) is authored in `rfc/drain-budget.txt` and owned by the umbrella and ultimately by Thomas. A rate, a date or a cadence hardcoded in `scripts/dev/rfc_requirements.py` is a violation, not a default, and a `rfc/drain-budget.txt` that names an RFC is the hand-kept registry the 2026-07-29 resolution rejected |
| Owner ruling 1 held | Every register raises the signed total, AND every published count carries its register split. Check BOTH halves: a reviewer who sees only the columns has not verified the credit, and one who sees only the total has not verified the counterweight |
| Owner ruling 2 held | `rfc/short/rfc7296.md` carries §2.2 rows; `rfc/extraction/rfc7296.json` is fully classified; every newly extracted id is proven or carries a Deviations-recorded Thomas authorisation. A phase-8 that extracted only the two named obligations and skipped the rest of the walk has not held the ruling |
| No pre-authorised annotation | Grep the diff for `{gap}`, `{not-applicable}` and `{single-polarity}` added to `rfc/short/rfc7296.md`. Each one must have a dated authorisation in Deviations naming the requirement id. An annotation justified by this spec's own text is a violation, since the spec explicitly grants no such permission |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `check_extraction_signoff` wired into `run_check` | `grep -n "check_extraction_signoff" scripts/dev/rfc_requirements.py` shows a definition AND a call inside `run_check` |
| `check_extraction_ratchet` wired into `run_check` | same grep shape |
| `check_drain_floor` wired into `run_check` | same grep shape; and with the shipped rate of `0` the floor reads 0 and `make ze-rfc-check` exits 0 (AC-27) |
| The floor can actually fail | set a non-zero rate in a scratch `rfc/drain-budget.txt`, run `make ze-rfc-check`, observe exit 2 naming the floor, the signed count and the start date, then restore the shipped rate of `0` (AC-30). A floor that cannot fail is the vacuity this spec exists to remove |
| `check_enrolment` precondition | `python3 scripts/dev/rfc_requirements.py --selftest` passes `TestEnrolment.test_new_enrolment_without_signoff_fails` |
| Live tree green when the machinery lands | `make ze-rfc-check` exits 0 with zero `*.json` files under `rfc/extraction/` (phases 1 to 7) |
| Live tree green after the pilot | `make ze-rfc-check` exits 0 with exactly one, `rfc/extraction/rfc7296.json`, and `ls rfc/extraction/*.json \| wc -l` reads 1 |
| Registers published apart | `make ze-rfc-index`, then read the extraction table: one column per register, and no signed total rendered without them |
| Every register earns credit | ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; the `--json` spelling exits 2 before the recipe runs, so a closer following it verifies nothing -- see "Two spec corrections found by implementing it"), then confirm `signed-by-register` has a key per register and its values sum to the published total |
| rfc7296 §2.2 extracted | `grep -c "RFC7296-2\.2-" rfc/short/rfc7296.md` is at least 2, having been 0 on 2026-07-29 |
| rfc7296 §2.2 proven | `grep -rn "RFC requirement: RFC7296-2.2-" internal/` shows a positive AND a negative tag for each of the two ids, or Deviations records Thomas's authorisation for what is missing |
| Nothing was retired to reach green | `check_retired_requirements` and `check_coverage_ratchet` green across the re-authoring: all 23 pre-existing rfc7296 ids still present, none demoted |
| Backlog published | `grep -n "UNSIGNED" ai/RFC-REQUIREMENTS.md` after `make ze-rfc-index` |
| Status envelope | ~~`make ze-rfc-extraction-status --json`~~ `make ze-rfc-extraction-status` (corrected 2026-07-29; the `--json` spelling exits 2 before the recipe runs, so a closer following it verifies nothing -- see "Two spec corrections found by implementing it") output parses as JSON (pipe it into `python3 -c` with `json.load`) |
| Skeleton cannot pass | write a skeleton for one stem, run `make ze-rfc-check`, observe exit 2 naming unclassified sites, then delete it |
| Verify cost | `time make ze-rfc-check` before and after; the delta is under the budget `_git_cat_blobs:899` documents (a gate that doubles verify time is one people skip) |
| Five ratchets documented | count the data rows of the ratchet table in `ai/rules/rfc-compliance.md`: five, not four |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `rfc/extraction/*.json` is repo content parsed by a dev script. Malformed JSON, wrong types, and unknown enum values must raise `ParseError` and exit 2, never a traceback and never a silent skip (AC-18) |
| Path handling | `stem` in the artifact must equal the filename stem; the tool must not follow a `source-path` that escapes `rfc/full/` or `rfc/drafts/`, which are the only two locations `source_keyword_count:1329-1331` searches |
| Resource use | The site derivation reads every enrolled RFC's source text on every `--check`. Bound the cost and measure it (A-9); a per-run re-read of 172 text files must not push `ze-rfc-check` past its budget |
| Trust boundary (machinery) | None crossed: no network, no untrusted input, no privilege. The honest risk is social, not technical, and is R-2 |
| Trust boundary (pilot) | Crossed, and this row must not be waived by the one above. IKEv2 parses attacker-controlled network input, and the two named §2.2 obligations sit ON the replay-protection surface: `rfc/full/rfc7296.txt:1437` states that Message IDs "are cryptographically protected and provide protection against message replays", which is why `:1439` requires the SA to be closed or rekeyed rather than allowed to wrap. Any phase-8 change to Message ID acceptance, window handling, or retransmission caching is reviewed as security-relevant: what a peer can force, what wraps, and what a replayed or out-of-window message can reach |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| `ze-rfc-check` red on the live tree with zero sign-offs | The grandfathering scope is wrong. Back to phase 5, not an allowlist |
| The register split does not match A-2/A-3 | Back to phase 2; the derivation is wrong, not the assumption's consequence |
| Audit finds a missing AC | Back to the relevant phase and implement |
| **A newly extracted rfc7296 requirement cannot be implemented and proven inside this spec** | **STOP and escalate to Thomas**, supplying the requirement id as allocated, the RFC section text verbatim with its `rfc/full/rfc7296.txt` line number, the producing code cited as `file:line` (read, not inferred from a caller), and what full implementation plus the tagged pair would cost. Ask which way he wants it fixed. NEVER annotate it, never move it to `plan/deferrals/`, never route it to a follow-up spec, never write it into `plan/known-failures/`, and never present "leave it non-conformant" as an option. The spec stays OPEN until he rules, and his ruling is recorded in Deviations with the date (`ai/rules/rfc-compliance.md:36-51`, `ai/rules/no-parking.md`) |
| Phase 8a's count is larger than this spec can carry | Same route: STOP and take the COUNT to Thomas for scoping (R-10). Scoping is his decision; extracting less is not an available answer, and neither is closing the spec on the part that was easy (`ai/rules/no-partial-completion.md`) |
| An existing rfc7296-tagged test must change and the `rfc-tagged-test` guard rejects the edit | Correct behavior. Only the user authorises it, via `// rfc-test-change-approved: <date> <what was approved>`. `// test-relax:` does not satisfy that guard, and self-authorising is the failure it exists to stop |
| `ze-rfc-check` red after the pilot because a new row has no tag | Not a grandfathering problem and not a scope problem: the row and its proof belong in the same commit (R-11). Finish the proof or carry the ruling; never delete the row |
| `ze-rfc-check` red naming an `unproven support` stem (rfc1035, rfc3765, rfc4486, rfc5301) | That red belongs to `plan/spec-rfcgate-4-ledger.md`, which is out of sequence. Unwind the ordering rather than bypassing (R-13). Do not `--unverified` past it and do not log it: the red is deterministic and reproducible, which `ai/rules/fix-dont-record.md` bans from `plan/known-failures/` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design

### The artifact

One file per RFC at `rfc/extraction/<stem>.json`, a sibling of `rfc/audit/<stem>.json`
(`load_audit:1239`). The audit artifact answers "does this test still enforce this
requirement" and is keyed on requirement text plus test shas; the extraction artifact
answers "does this summary capture this RFC" and is keyed on the source text sha. Two
questions, two lifecycles, two freshness keys, two files.

**Top level.**

| Field | Type | Authored or derived | Meaning |
|-------|------|---------------------|---------|
| `schema-version` | integer | authored | 1 |
| `stem` | string | authored | must equal the filename stem |
| `register` | enum | authored, cross-checked | `rfc2119`, `prose`, or `manual-walk`; may never be stronger than the derived register |
| `register-reason` | string | authored | **Added 2026-07-29 during implementation; see Deviations.** Required only when `register` is `manual-walk`: the stated reason why no mechanical inventory exists. AC-10 demands it and no other field could hold it. Same conditionally-required shape as `resign-reason` below, and like it, required at CHECK time rather than PARSE time so a mid-walk skeleton still parses |
| `source-path` | string | derived | `rfc/full/<stem>.txt` or `rfc/drafts/<stem>.txt` |
| `source-sha` | string | derived | sha256[:16] of the normalized source, using the existing `_normalize`/`requirement_sha:1214-1220` pair |
| `signed-off` | date | authored | when the walk was performed |
| `reviewer` | string | authored | who performed it |
| `resign-reason` | string | authored | required only when the exclusion count rises above HEAD |
| `sections` | list | see below | one entry per derived section |
| `sites` | list | see below | one entry per derived normative site |

**Section entry.**

| Field | Type | Authored or derived | Meaning |
|-------|------|---------------------|---------|
| `id` | string | derived | the section number as the source spells it |
| `sites` | integer | derived | count of normative sites attributed to this section |
| `disposition` | enum | authored | `walked` or `skipped`; null fails |
| `skip-kind` | enum | authored | required for `skipped`: `front-matter`, `references`, `iana`, `acknowledgements`, `appendix-non-normative` |
| `reason` | string | authored | required for `skipped` |
| `unsourced-ids` | list of requirement ids | authored | requirement ids read from this section's indicative prose with no keyword site behind them |

**Site entry.**

| Field | Type | Authored or derived | Meaning |
|-------|------|---------------------|---------|
| `id` | string | derived | locator `<section>:<n>`, the nth normative site of that section in document order |
| `quote` | string, truncated | derived | the normalized site sentence, written by the skeleton and re-verified at check time |
| `disposition` | enum | authored | `mapped` or `excluded`; null fails |
| `mapped-to` | requirement id | authored | required for `mapped`; must exist in `rfc/short/<stem>.md` |
| `excluded-kind` | enum | authored | required for `excluded`, from the closed set below |
| `reason` | string | authored | required for `excluded`, non-empty |

**Exclusion kinds (closed set).**

| Kind | Means | Extra obligation |
|------|-------|------------------|
| `not-a-requirement` | the keyword is in non-normative use: a quotation, a description of another system, RFC 2119 boilerplate the extractor did not strip | reason names which |
| `binds-another-role` | the obligation binds a role Ze does not implement (a CA, a registry, an IANA action, the peer) | reason names the role |
| `duplicate-of` | restates an obligation already captured | must name a requirement id that SOME OTHER site maps (AC-8) |
| `cross-document` | the obligation belongs to another RFC the sentence cites | mirrors the concept `_CROSS_RFC_SEC_RE:135` already encodes |
| `advisory-in-context` | the capitalised keyword sits inside a SHOULD/MAY construction the splitter mis-cut | reason quotes the enclosing construction |

Anything outside these sets is a `ParseError`, exactly as an unknown annotation kind is
today (`_parse_annotation:213-217`).

### Register derivation (this is where the pre-2119 fail-open is closed)

The register is DERIVED from the source text; the artifact only declares which derived
register it is signing under, and a stronger claim than the derivation supports is refused.

| Derived register | Condition on the source text | Inventory used | Sign-off strength |
|------------------|------------------------------|----------------|-------------------|
| `rfc2119` | at least one capitalised MUST-level keyword outside the 2119 boilerplate, AND derived site count is at least the declared gated-requirement count | capitalised-keyword sites | strongest available: full forward and reverse arithmetic |
| `prose` | no capitalised keyword outside boilerplate, OR derived site count below the declared gated-requirement count | case-insensitive modal sites | same arithmetic over a noisier inventory, plus a complete section walk |
| `manual-walk` | the inventory is empty under BOTH registers while the summary declares at least one gated requirement | none exists | assertion only: complete section walk, reviewer, and a stated reason why no inventory exists |

Measured 2026-07-29: 22 of 166 enrolled RFCs have no capitalised MUST-level keyword at all
(A-3) and would be vacuously green under any keyword-only arithmetic while declaring 164
gated MUSTs between them. A further 31 fall to `prose` on the undercount clause, for a
total of 53 (A-2), leaving 113 that can take the `rfc2119` grade. 53 is the SITE-denominator
figure and it is the one this rule operates on; the umbrella also carries a 33 measured at
the keyword-OCCURRENCE denominator, which is a different question and not a correction of
this one (`plan/spec-rfcgate-0-umbrella.md`, "The three pre-2119 measurements").
Under the prose register those 22 yield 688 sites rather than zero (A-4). rfc1877 yields zero under both and is the sole measured `manual-walk` candidate
(A-5): 10591 characters with no capitalised keyword and no lowercase modal, and four gated
MUSTs declared.

The fail-closed shape, stated against `ai/rules/fail-closed-guards.md`: a keyword-driven
check on a keyword-free source cannot deny, so it must SAY so. It says so three ways. It
refuses the stronger claim (AC-9). It refuses an unevaluable sign-off outright unless a
`manual-walk` record exists (AC-10). And it never publishes a sign-off count without the
register that earned it, so the published number can never be read as stronger evidence
than it is (AC-11, AC-21, AC-22). The zero-value trap is closed at the producer: an empty
inventory is a refusal, never a satisfied count of zero.

**Credit and strength are two facts, and owner ruling 1 keeps them apart rather than
choosing between them.** ~~A `manual-walk` is never added to the derived sign-off total.~~
(Superseded 2026-07-29.) Every register raises the signed total by one per stem, so every
sign-off earns drain credit and no RFC is undrainable because of the register its own
authors wrote in. What never happens is a total rendered or emitted ALONE: the ledger's
extraction table carries an `rfc2119`, a `prose` and a `manual-walk` column, and the status
envelope carries `signed-by-register` whose values sum to the total. A reader who wants
"how much is bounded" reads the total; a reader who wants "how strongly" reads the columns;
neither has to infer the other, and neither can be quoted as the other.

Why the earlier design was wrong, recorded so it is not re-derived: excluding the weaker
registers from the total looks conservative and is not. It would have made 53 of 166
enrolled RFCs, including OSPFv2 (`rfc/full/rfc2328.txt`, zero uppercase MUST-level
keywords) and RFC 2181 (23 declared gated MUSTs against ONE uppercase occurrence),
permanently ineligible for the quota that exists to drain them. A backlog with no exit is
not a stricter ratchet, it is a ratchet that will be deleted rather than obeyed, which is
the exact failure `ai/rules/rfc-compliance.md:114-116` records.

### The two arithmetics

**Forward** (catches a MISSED obligation): every derived site is `mapped` or `excluded`.
An unclassified site fails. A site the reviewer never saw, or one that appears because the
source text changed, lands unclassified and reds the gate.

**Reverse** (catches an INVENTED obligation, and the misquote's other face): every gated
requirement of the summary is either the target of some site's `mapped-to`, or listed in
some section's `unsourced-ids`. `rfc/enrolled.txt`'s own header records that a requirement
the RFC does not contain was found once and corrected; today nothing looks for the next one.

The `unsourced-ids` escape is not a loophole, it is the honest name for a real case.
RFC 4271 Section 8.2.2 is 35168 characters of the most load-bearing state machine in the
product and contains exactly ONE capitalised MUST-level keyword (a SHALL, about tracking a
second connection). The rest is indicative prose: "the local system: initializes all BGP
resources for the peer connection, sets ConnectRetryCounter to zero, starts the
ConnectRetryTimer". A requirement extracted from that has no keyword site to map to and
must still be expressible, attributed to the section it was read from.

**Correction to the research brief, recorded so it is not repeated.** The brief stated
RFC 4271 Section 8.2.2 has "~45 MUST/SHALL occurrences". It has one. The 45 comes from
slicing the source from the Section 8.2.2 heading to a `8.3.` marker that does not exist
in RFC 4271, so the slice ran to the end of the document. Bounded properly (offsets 118996
to 154164, the `9.  UPDATE Message Handling` heading), the section holds one capitalised
keyword and two lowercase modals. The claim that Section 8.2.2 has NO requirement id is
correct and independently verified. The corrected figure strengthens the argument rather
than weakening it: a keyword scan is nearly blind to that section.

### The rfc7296 pilot (owner ruling 2)

**Why one RFC escapes the grandfather.** Grandfathering answers "what may a NEW MECHANISM
red on". It has never answered "may a known obligation stay unextracted", and the rules
answer that second question in the other direction: `ai/rules/rfc-compliance.md:28` says an
unextracted obligation is still an obligation and the gate's silence is not conformance,
and `ai/rules/no-parking.md` makes the session that found it the entry point that fixes it.
Two RFC 7296 obligations were read, quoted and confirmed absent during this spec's
research. That confirmation is what removes them from the backlog: they stopped being
"things nobody has looked at" the moment somebody looked.

**Why rfc7296 and not another.** It is the worst input in the corpus, so it is the best
pilot. 263 derived sites against 18 captured gated requirements is the lowest measured
ratio; 92 numbered sections against 14 anchored is the widest section gap; and its source
is dense enough that the site extractor, the section axis, `unsourced-ids` and the
exclusion kinds all get exercised on one file. A format that survives rfc7296 needs no
second opinion. A format that does not is better discovered here than after 165 sign-offs
were authored against it, which is why phase 8 runs inside this spec rather than after it:
a defect the pilot exposes is a defect in phases 3 and 4 to FIX, not a field to work around.

**What the pilot walks.** All 92 numbered sections, not only Section 2.2. The two confirmed
obligations are the FLOOR of what phase 8 extracts, never the ceiling: extracting exactly
the two already-named ones and stopping would reproduce, in miniature, the same
"bounded by what somebody happened to notice" failure this whole spec exists to end.

**The escalation rule for every newly extracted requirement (BLOCKING).**

Each requirement the walk newly extracts is implemented and proven. A new checklist row is
not bookkeeping; it is an obligation Ze owes. For each newly allocated
`RFC7296-<section>-<n>`, phase 8 lands working code plus a positive AND a negative
`RFC requirement:` tagged test, in the SAME commit as the row (R-11).

**This spec pre-authorises nothing weaker.** `{gap}`, `{not-applicable}`,
`{single-polarity}` and "implemented but unproven" are all choices to do LESS than full
compliance, and `ai/rules/rfc-compliance.md` "Ask Thomas Whenever Full Compliance Is On The
Table" reserves every one of them to Thomas. Nothing in this spec, its acceptance criteria,
its phase plan or its risk table may be cited as permission for any of them, and this
sentence exists so that no future reader can mistake the phase's existence for its
approval.

When full implementation plus a tagged pair is not the answer being taken, the implementer
STOPS and asks, in one message carrying all four of:

| # | What the escalation must carry | Why |
|---|-------------------------------|-----|
| 1 | The requirement id exactly as allocated (`RFC7296-2.2-1`, ...) | The id is the permanent contract tests bind to; an escalation about "the retransmission thing" cannot be recorded or re-found |
| 2 | The RFC section text VERBATIM, with its `rfc/full/rfc7296.txt` line number | A paraphrase is where a misquote enters, and a misquoted obligation licenses a justification that never engages it (`ai/rules/rfc-compliance.md:94-98`) |
| 3 | The producing code as `file:line`, READ, not inferred from a caller | `ai/rules/no-fabrication.md`: a coherent story about what the code does is a hypothesis until the producing function has been read |
| 4 | What full implementation plus the tagged pair would actually cost | Thomas is deciding between real options, and a cost he has to guess at is not one of them |

The question is always "which way do you want this fixed", never "may I skip it". Offering
"leave it non-conformant" as an option is banned (`ai/rules/no-parking.md`). His answer is
recorded in this spec's Deviations with the date and the requirement id, so the next reader
inherits a decision rather than a rationalization.

**A pre-existing annotation is not authority.** rfc7296 carries 2 `{gap}` rows and 1
`{single-polarity: positive}` today (`rfc/enrolled.txt:159`;
`docs/features/rfc-status.md:213` discloses the two gaps as `RFC7296-2.9-1` TS narrowing
and `RFC7296-1.4-1` Delete-payload echo). Every one of those is VOID as authority
(`ai/rules/rfc-compliance.md:53`, owner directive 2026-07-27). Any that the walk re-touches
is re-raised with the same four items above, and left alone otherwise: re-raising every
annotation in the file is not this phase's scope, but citing one as settled is forbidden.

**What the pilot must not break.** The re-authoring runs inside two live ratchets. No
existing id may be deleted (`check_retired_requirements`), so a misquote is corrected by
editing the TEXT under the same id. No existing requirement may lose a polarity it holds at
HEAD (`check_coverage_ratchet`), so the walk only ever adds proof. Separately, the
`rfc-tagged-test` edit-time guard blocks a behavior change to any already-tagged rfc7296
test; only the user can authorise one, and `// test-relax:` explicitly does not (R-12).
There is no `rfc/audit/rfc7296.json` (`rfc/audit/` holds only `rfc7606.json`, checked
2026-07-29), so `check_audit_freshness` cannot fire on the re-authored text: one cost the
pilot does not pay.

### Grandfathering, ratchet, and the drain interface

Grandfathering is implemented as SCOPE, never as an allowlist file. The sign-off check
judges a stem when the stem has an artifact, and `check_enrolment` demands an artifact only
for a stem enrolled since HEAD. With zero artifacts present the new machinery judges only
new enrolments, so the 166 stay green (AC-19). With ONE artifact present after the pilot,
the other 165 are still unjudged and still green (AC-24), which is precisely the property
scope-based grandfathering has and an allowlist file would not: nothing had to be added to
a list of exceptions when rfc7296 stopped being an exception. This follows the four existing ratchets,
each of which grandfathers pre-HEAD state for the reason `ai/rules/rfc-compliance.md:114-116`
records: a rule that reds the gate on unrelated work gets removed rather than obeyed.

Two ratchets keep it moving in one direction only: the set of stems with a valid sign-off
may not shrink (AC-12), and a signed-off stem's exclusion count may not rise without a
recorded re-sign (AC-13, R-1). Both read HEAD through `git ls-tree` plus `_git_cat_blobs:899`,
the same batch path `_git_baseline_ids:714` uses. Both consume the baseline as
`baseline - current`, so an empty baseline accuses nobody and a git failure judges nothing
(AC-17). That is the opposite polarity from `_git_baseline_summary_stems:763`, whose
docstring explains why IT must return None instead; the distinction is restated for this
consumer rather than copied.

The umbrella owns the forcing function. The interface between the two specs:

| This spec provides | The umbrella provides |
|--------------------|-----------------------|
| `--extraction-status --json`: signed count, enrolled count, `signed-by-register`, unsigned backlog list, `schema-version` | the value of N, the release cadence, and where the per-release floor is stored |
| the counting rule settled by owner ruling 1: EVERY register credits the signed total, and no total is ever published without its register split | ~~a gate that fails when the floor did not rise by N since the last release, reading the total and free to report the split~~ (Resolved 2026-07-29: the POLICY that gate reads, plus the acceptance criterion over it. The comparison itself is `check_drain_floor` and is implemented HERE: AC-27 to AC-30) |
| the floor COMPARISON: read the policy, compute `ceil(rate x months elapsed since start)`, cap it at the backlog size, judge the derived signed count against it, and fail closed when the policy file is missing or unparseable | the two policy fields in `rfc/drain-budget.txt`, the decision to ship the rate at `0`, and the arming commit that makes it non-zero |
| a monotonic guarantee: the signed count never falls between releases | the release-time reading of that guarantee |
| the published backlog table in `ai/RFC-REQUIREMENTS.md`, kept fresh by `check_ledger_fresh:1578` | the release-time reading of that table |
| refusal of a NEW enrolment without a sign-off | the ordering of which RFCs drain first |
| one worked example, rfc7296, so the quota is not armed against an unproven format | the arming commit itself, which this set's D4 puts outside child scope |

~~This spec must NOT implement the cadence, the quota, or a calendar.~~ **Amended
2026-07-29, by the supervising session, because as written it left the floor comparison
specified by the umbrella (its AC-13) and forbidden here, so no spec implemented it.** What
this spec must not do is INVENT the POLICY: the rate, the start date and the cadence
semantics are the umbrella's and ultimately Thomas's, and nothing here may hardcode or
override them. Implementing the comparison that reads that policy against the derived
signed-set is this spec's, and belongs here for three reasons. The floor is a pure function
of the extraction sign-off set, which this spec already derives, already counts and already
renders per register, so `ai/rules/derive-not-hardcode.md` puts the arithmetic where the
data lives. A fifth child, or the umbrella, would split one derivation across two owners
for no benefit and give the ratchet a second reader of `rfc/extraction/`. And per owner
decision D5 the rate ships at `0`, so the comparison lands INERT and its first real exercise
is the arming commit, which is Thomas's, not this spec's.

If the umbrella lands first and specifies a different status shape, this spec's envelope
adapts; the counts it derives do not change, and neither does the counting RULE, which is
now an owner ruling rather than a child's design choice.

### Interaction with the umbrella's recertification ledger (RESOLVED 2026-07-29)

~~The umbrella's child-1 row asks this spec to "create `rfc/recertified.txt` and the drain
schedule machinery". This spec ships `rfc/extraction/<stem>.json` plus a derived
`--extraction-status --json` and a rendered ledger table, and no file by that name. The
divergence is recorded rather than quietly resolved (R-14)~~ **The umbrella resolved it on
2026-07-29**, which is where the decision belonged. The outcome:

| Resolution | Verdict |
|------------|---------|
| **The extraction artifact set IS the record, read through `--extraction-status --json`** | **CHOSEN.** One artifact, one question, no second source of truth. The quota's signed count, its per-register split and its backlog are all derived from `rfc/extraction/` plus the live summaries |
| `rfc/recertified.txt` as a RENDERED view of `rfc/extraction/` | Rejected as redundant: `ai/RFC-REQUIREMENTS.md` already publishes those numbers and `--extraction-status --json` already emits them, and each extra copy is one more artifact `check_ledger_fresh` must keep fresh |
| `rfc/recertified.txt` hand-authored with typed counts | Refused, as it was before the resolution. A hand-typed count is a claim, and claims are what this programme removes (`ai/rules/derive-not-hardcode.md`). It is also the artifact shape the 2026-07-20 ruling in `plan/deferrals/rfc-gate-regression-ratchets.md` already rejected |

**What this spec creates as a consequence.** One authored file, `rfc/drain-budget.txt`,
holding the drain POLICY and nothing else: a `start` date and a `rate` in entries per
calendar month, shipping at `0` per the umbrella's D5. It is authored because no artifact
can derive it -- when the clock starts and how fast it runs are choices, not measurements.

→ Constraint: `rfc/drain-budget.txt` may never gain a per-stem row, a count, a stem list or
a register column. The moment it names an RFC it has become the hand-kept registry this
resolution rejected. A reviewer treats such a row as a defect, not as an extension.
→ Constraint: ~~this spec still does NOT implement the cadence, the value of N, or the floor
computation. It creates the budget file and emits the counts; the umbrella owns what is
done with them.~~ (Amended 2026-07-29, same resolution as above.) This spec still does NOT
author the cadence, the value of N, or the rate. It creates the budget file, emits the
counts, and implements the floor COMPARISON that reads both (`check_drain_floor`, AC-27 to
AC-30). The umbrella owns the policy those inputs carry and the acceptance criterion over
the result.

### Cross-child sequencing (why the order is load-bearing)

The umbrella's Sequencing Constraint already forbids two children in flight and fixes the
order 1, 2, 3, 4, on the textual ground that all four edit
`scripts/dev/rfc_requirements.py`. A second, sharper reason applies to child 4 in
particular: `plan/spec-rfcgate-4-ledger.md` (its R-6, superseded there by R-6a and OC-1)
arms `check_unproven_support`, which is RED BY DESIGN on four stems (rfc1035, rfc3765,
rfc4486, rfc5301). An armed red leaves `make ze-verify` non-green in both modes, and
`commit_helper.py create` refuses a script over that (`ai/rules/git-safety.md`, Step 1), so
that gate may be armed ONLY in the commit that resolves the four, or in a later one. It may
never be armed "now, cleared next commit".

~~A deterministic structural gate is never an eligible known-red and its only bypass is
owner-only (`ai/rules/git-safety.md:229-242`).~~ **Corrected 2026-07-29.** That sentence was
false and is withdrawn; the ordering it justified is unchanged. `ze-rfc-check` is not one of
the eight names in `STRUCTURAL_GATES` (`scripts/dev/commit_helper.py:512-523`), so the red
is `--unverified`-bypassable in mechanism. It is not bypassable in practice:
`--unverified` covers a flaky or environmental red or one already logged in
`plan/known-failures/`, and `ai/rules/fix-dont-record.md` forbids a shard for anything
deterministic and reproducible. A red naming four fixed stems is the most deterministic red
there is. What remains is an explicit owner ruling on every commit until the four are
cleared, which is a cost, not an escape. The correction matters to a reader who checks the
citation and finds `ze-rfc-check` absent from the frozenset: the ordering does not fall with
the wrong argument.

This spec is the opposite shape and must stay that way: it lands green with zero artifacts
(AC-19) and stays green with one (AC-24). Sequencing them the other way round would make
this spec's landing commit inherit a red it cannot clear and did not cause, which is the
scenario the Failure Routing table now routes explicitly.

One consequence runs the other way and is worth stating, because it lands on child 4 as new
work: from the day THIS spec lands, `check_enrolment` refuses a newly enrolled stem with no
extraction sign-off (AC-1). Child 4's D8 enrols exactly four, so it carries four extraction
sign-offs, and two of those four (rfc3765, rfc4486) have no `rfc/full/` source text yet.
The umbrella already requires child 4 to fetch them first, which the sign-off makes
mandatory rather than advisable: with no source text there is no inventory to derive, and
no register to sign under.

### Interaction with `plan/spec-followup-rfc-enrollment.md`

That spec owns `rfc/enrolled.txt` and the Coverage-by-RFC rollup and grows enrolment RFC by
RFC. This spec adds one precondition to growing it and adds a second table beside the
rollup. Neither adds a row to `rfc/enrolled.txt`. The practical consequence for that spec:
from the day this lands, each of its enrolment increments carries an extraction sign-off
for the RFC it enrolls, which is work it did not previously have and which its own
`ai/skills/ze-rfc.md:43-54` coverage self-check was already asking for by hand.

The open deferral row in `plan/deferrals/rfc-gate-regression-ratchets.md` (arm the SHA
freshness ratchet for the other 164 enrolled RFCs, destination
`plan/spec-followup-rfc-enrollment.md`) is deliberately NOT touched here. It concerns
semantic audit verdicts, a different artifact and a different question, and the 2026-07-20
ruling that closed it stands. This spec cites that ruling as precedent for R-2 rather than
reopening it.

## Design Insights
- The gate's blind spot is not an oversight in `rfc_requirements.py`; it is the boundary of
  what a requirement-to-test gate can see. Closing it needs a different oracle (the source
  text), not a stronger version of the same comparison.
- Measured, not assumed: section granularity would be 75% as good and far cheaper
  (1671 of 2229 hole-sites sit in wholly-unanchored sections), but RFC 4271, the most
  load-bearing RFC in the tree, is 9 unanchored against 48 partial. The cheap design would
  declare it nearly complete. Site granularity is required by evidence, not by preference.
- The pre-2119 problem is bigger than the 22 keyword-free RFCs. RFC 2181 states in its own
  Section 3 that it does not use the 2119 expressions, and its summary correctly declares
  23 gated MUSTs against one keyword occurrence. Any design keyed on capitalised keywords
  alone is wrong for 53 of 166 enrolled RFCs, not for 22.
- A skeleton generator that can only emit UNCLASSIFIED entries inverts the mass-generation
  incentive: generating artifacts makes the gate redder. That is a structural answer to a
  social failure mode, which is what the 2026-07-20 ruling asked for.
- Verifying the brief's own numbers caught one false claim (RFC 4271 §8.2.2's keyword
  count) and confirmed the rest within a few percent. The brief warned that an earlier
  researcher had made two errors on this exact file; a third was waiting.
- The two questions this spec took to the owner had the same shape, and both were answered
  against the cautious-looking option. Excluding weak registers from the quota LOOKED
  strict and would have created a third of a corpus that could never be drained. Leaving
  rfc7296 to the backlog LOOKED consistent with grandfathering and would have left two
  confirmed obligations unextracted. Both times the conservative move was the one that
  quietly reduced what Ze owes, which is the direction the rules never permit.
- Grandfathering and no-parking do not actually conflict; they answer different questions.
  Grandfathering bounds what a NEW MECHANISM may red on. No-parking binds what a PERSON who
  has read an obligation may do next. The apparent tension came from reading a scope rule
  as an entitlement.
- A ratchet has two failure modes, not one. Too weak, and it proves nothing. Too strong,
  and it gets deleted rather than obeyed (`ai/rules/rfc-compliance.md:114-116`). Both
  rulings are the owner steering between those two walls: credit every register so the
  backlog has an exit, publish every register so the credit is not mistaken for evidence.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A per-RFC sign-off artifact under `rfc/extraction/` | (a) a bare monotonic captured-count ratchet; (b) gate full per-site mapping of all ~4265 sites immediately | (a) is strictly dominated by `check_retired_requirements:1007-1056`, which fires on any id present at HEAD and absent now, a superset of "the count fell" that also catches a delete-one-add-one swap; a count ratchet adds a weaker second copy of an existing guarantee and says nothing about what was never extracted. (b) reds 166 RFCs on day one for work nobody has done, which `ai/rules/rfc-compliance.md:114-116` records as the way a ratchet gets removed rather than obeyed |
| Site granularity, with a section axis beside it | section-only classification | Measured: 558 of 2229 hole-sites sit in sections that already carry ids, and RFC 4271 is 48 partial against 9 unanchored. Section-only would pass RFC 4271 as nearly complete. The section axis is kept anyway because sites alone are blind to indicative-prose sections (RFC 4271 §8.2.2: 35168 characters, one keyword site) |
| The site inventory is derived at check time; only dispositions are authored | record `sites: {seen, mapped, excluded}` counts in the artifact | A hand-typed "seen" is a claim, and claims are what this programme removes (`ai/rules/derive-not-hardcode.md`). Deriving turns the check into arithmetic a machine can redo, and makes an unclassified site impossible to hide |
| The register is derived, and a stronger claim is refused | trust an authored `method` field | The 22 keyword-free RFCs are exactly the population that would benefit from claiming the strong register. Deriving it removes the choice |
| `manual-walk` is accepted, credited to the quota, and published in its own column (owner ruling 1, 2026-07-29) | (a) refuse `manual-walk` outright; (b) accept it but exclude it from the signed total | (a) makes the ratchet unsatisfiable for rfc1877-shaped RFCs, and an unsatisfiable ratchet gets removed. (b) was this spec's earlier design and the owner overruled it: excluding the weaker registers from the total leaves 53 of 166 enrolled RFCs, including OSPFv2 and RFC 2181, permanently undrainable, which is a backlog with no exit dressed as rigour. Accepting it silently into an undifferentiated total would launder an assertion into the same number as verified evidence, so the ruling pairs the credit with a mandatory per-register split. Credit and strength are two facts and get two columns |
| The `manual-walk` register is retained even if no live RFC needs it | drop it when the measured live set is empty (A-5's original plan) | Owner ruling 1 makes "no route to a sign-off" an unacceptable outcome for any enrolled RFC. Dropping the register would leave an rfc1877-shaped RFC refused by AC-10 with nothing to satisfy it. A column that reads 0 costs nothing; an unsatisfiable refusal costs the ratchet |
| rfc7296 is re-authored inside this spec (owner ruling 2, 2026-07-29) | (a) leave it grandfathered with the other 165; (b) file the two obligations as a deferral pointing at the fleet spec; (c) re-author it in a separate follow-on spec | (a) treats a scope rule as an entitlement: grandfathering bounds what a new mechanism may red on, and `ai/rules/rfc-compliance.md:28` says an unextracted obligation is still an obligation. (b) is exactly what `ai/rules/no-parking.md` names as recording instead of fixing, and the deferral machinery is explicitly not a compliance decision procedure. (c) is the same deferral with a spec-shaped wrapper. The obligations were confirmed by THIS spec's research, so this spec is the entry point. It also buys a pilot the format badly needs |
| The pilot is rfc7296 rather than an easy RFC | pilot on a small, keyword-rich, already well-captured RFC | An easy pilot proves the format works where it was never in doubt. rfc7296 has the corpus's worst capture ratio (263 sites against 18 captured) and the widest section gap (92 against 14), so it exercises the site inventory, the section axis, `unsourced-ids` and the exclusion kinds at once. A format defect is far cheaper to find before 165 sign-offs are authored against it than after |
| Artifact in `rfc/extraction/`, not in `rfc/short/<stem>.md` | put the sign-off in the summary's front matter | `ai/rules/rfc-compliance.md:64-71`: summaries are protocol-only reference documents and must carry no Ze-specific information. A sign-off is Ze bookkeeping |
| Artifact in `rfc/extraction/`, not in `rfc/enrolled.txt` | extra columns on the enrolment row | Sign-off and enrolment are independent states: a grandfathered RFC is enrolled and unsigned, and a new RFC must be signed BEFORE it is enrolled. One file cannot represent both orders. `parse_enrolled:688-695` also takes `line.split()[0]`, so the format cannot carry structure |
| Separate file from `rfc/audit/<rfc>.json` | one artifact per RFC carrying both | Different questions, different freshness keys (`verdict_is_fresh:1227` hashes requirement text plus test shas; extraction hashes the source text). Merging them would stale one half whenever the other changed and force a re-do of the wrong work |
| The check rides inside `--check`, no new verify stage | add `ze-rfc-extraction-check` to `stagesForMode` | `scripts/status/verify_run.go:215-223` records that the two stage lists are hand-duplicated and pinned by a golden, so a new stage is a three-place change with a known drift history. The extraction check is the same concern as `ze-rfc-check` and belongs inside it |
| Do NOT raise `source_keyword_count` into a ratio gate | fail when captured/keywords falls below a threshold | Vacuously green for the 22 keyword-free RFCs; wrong in the other direction for the 53 whose summaries legitimately out-declare the keyword count; and a ratio is a proxy for the thing rather than the thing, gameable by splitting one requirement into three |
| Exclusions shrink-only per stem, with a recorded re-sign as the only way up | a numeric cap on the exclusion ratio | A cap is gamed by rewording, and picks an arbitrary number nobody can defend. Shrink-only plus a published per-RFC ratio makes the pressure directional and the state visible |

## Known Limitations
- **The bound is over keyword-visible sites, not over obligations.** The forward
  arithmetic proves every site the extractor can SEE is accounted for. It does not prove
  every obligation is captured; those differ by the extractor's recall, and RFC 4271
  Section 8.2.2 shows recall can be near zero for an indicative-prose section. The residual
  is published rather than claimed away, and `unsourced-ids` is how a reviewer records an
  obligation the extractor cannot see. Today the bound is zero, so this is a floor being
  raised, not a ceiling being reached.
- **A `manual-walk` sign-off is an assertion.** The gate checks that the section walk is
  complete against the derived section list and that the source sha is pinned. It cannot
  check that the reviewer read anything. It still earns drain credit (owner ruling 1),
  because an RFC whose own authors wrote no RFC 2119 keywords must have SOME route out of
  the backlog. The honesty is carried by the published register column, not by withholding
  the credit: a reader is told exactly what kind of evidence each signed stem rests on.
- **Misquote detection stays semantic.** The artifact makes the source-sentence-to-
  requirement pair explicit, which is what a reader needs; the gate never judges the
  rendering (R-3).
- ~~**No sign-off ships with this spec.** Performing them is fleet work owned by the
  umbrella and `plan/spec-followup-rfc-enrollment.md`. Shipping one here would be the
  mass-generation failure in miniature.~~ (Superseded 2026-07-29 by owner ruling 2.)
  **Exactly one sign-off ships: rfc7296.** The other 165 stay fleet work owned by the
  umbrella and `plan/spec-followup-rfc-enrollment.md`. One hand-performed walk is not the
  mass-generation failure R-2 names; R-2's defence is structural, not numeric, and it is
  unchanged (a generated skeleton can only be UNCLASSIFIED, and an unclassified site fails
  the check).
- **The rfc7296 tail is unscoped, deliberately and visibly.** Its capture ratio is the worst
  measured in the corpus, so the walk may surface a large number of new obligations, and
  the number is not knowable before phase 8a runs (A-10, R-10). The spec does not pretend
  otherwise and does not carry a guess. What it does carry is the rule that no outcome
  other than "implemented and proven" may be chosen without Thomas, and the sequencing that
  puts the count in front of him before implementation starts.
- **The pilot bounds one RFC, not the corpus.** After phase 8 the extraction bound covers
  rfc7296 and the 165 others remain unbounded and published as backlog. That is a floor
  raised from zero to one, which is the honest description; anyone reading the signed count
  as progress toward completeness should read the backlog column beside it.
- **SHOULD/MAY-level sites are outside the inventory**, matching `GATED_LEVELS:69` and the
  existing scope decision that advisory levels are listed and may be tagged but never gate.

The two below were surfaced by the 2026-07-29 independent review and are ACCEPTED
design limits, not defects. They are recorded so the next reader does not rediscover
them as bugs and spend the diagnosis again. The defects that review found are in
Deviations (DEV-4 to DEV-7) and are a different thing entirely.

- **A FIRST sign-off may exclude 100% of its sites and no ratchet can see it.**
  `check_extraction_ratchet` compares a stem's exclusion count against the same stem's
  count at HEAD, so it can only judge stems present in BOTH the baseline and the
  current tree. A stem signing off for the first time has no baseline row, so its
  exclusion count has nothing to rise above and every site could be excluded with a
  shrug on the way in. The ratchet is shrink-only BY DESIGN -- a cap on the exclusion
  ratio is gamed by rewording and picks a number nobody can defend -- so the mitigation
  is publication rather than a threshold: the per-RFC exclusion ratio is a column of
  the extraction table in `ai/RFC-REQUIREMENTS.md`, which makes an all-excluded
  sign-off loud in review even though it is green in the gate. This is umbrella R-1
  ("the pressure is directional and the state is visible"), and it is the accepted
  trade, not an oversight. The reviewer is the control on the first sign-off; the
  ratchet is the control on every one after it.
- **A zero-site inventory yields a valid sign-off that classifies nothing, and the
  artifact cannot say why.** `rfc1877` is the only live case: it is empty under both
  the keyword and the prose scan, derives `manual-walk`, and signs off on
  `register-reason` plus whatever `unsourced-ids` its sections declare. That is exactly
  AC-10's designed path and the alternative is worse -- refusing the sign-off outright
  would leave a keyword-free RFC permanently undrainable, which is the backlog-with-no-
  exit that owner ruling 1 rejected. The limit is that nothing in the artifact
  distinguishes "the walk genuinely found no obligation in this text" from "the
  derivation found nothing to walk and the reviewer signed the empty result". Both
  render identically: `manual-walk`, zero sites, a reason string. The reason string is
  the only place the difference can live, so it is load-bearing prose that no gate
  reads, and a reviewer of a `manual-walk` sign-off should read it as the whole
  evidence.

## RFC Documentation (Scope: protocol)

**Machinery (phases 1 to 7):** N-A for enforcing code. It adds no protocol behavior, so no
`// RFC NNNN Section X.Y` comment is added or changed. The RFC-facing artifact it produces
is the sign-off record, whose contract is documented in `rfc/extraction/README.md` and in
`ai/rules/rfc-compliance.md` "Extraction Completeness".

**Pilot (phase 8):** NOT N-A, superseding the blanket claim above (2026-07-29, owner ruling
2). Wherever phase 8 implements a newly extracted RFC 7296 MUST, `ai/rules/rfc-compliance.md`
"RFC MUST Comments (BLOCKING)" applies in full: a `// RFC 7296 Section X.Y: "<quoted
requirement>"` comment sits directly above the enforcing code, and where the change touches
wire encoding or decoding the wire-format documentation obligation applies too. The two
named obligations are behavioral rather than format-defining (Message ID reuse across a
retransmission, and closing or rekeying the IKE SA at Message ID exhaustion), so the
comment obligation is certain and the ASCII-diagram obligation is decided per requirement
by what the walk actually finds.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-32 all demonstrated (AC-27 to AC-32 added 2026-07-29: the floor comparison
      and the two register-validation arms)
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none unwritten
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] `make ze-rfc-check` green on the live tree with zero sign-off artifacts present
      (machinery, phases 1 to 7), and green again with exactly one after the pilot
- [ ] Feature code integrated in `scripts/dev/rfc_requirements.py`, called from `run_check`
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence, including
      the re-answer of documentation rows 1, 7 and 11 after phase 8
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] `ai/rules/rfc-compliance.md` ratchet table reads five ratchets, not four
- [ ] Owner ruling 1 demonstrated on both halves: an artifact in each register raises the
      signed total, and no published count appears without its register split
- [ ] Owner ruling 2 demonstrated: `rfc/short/rfc7296.md` walked across all 92 sections,
      `RFC7296-2.2-1` and `RFC7296-2.2-2` present with verbatim text, and
      `rfc/extraction/rfc7296.json` fully classified
- [ ] Every requirement the rfc7296 walk newly extracted is implemented and proven, OR
      Deviations records Thomas's dated authorisation for it by requirement id. No
      annotation rests on this spec's own text
- [ ] No rfc7296 id retired, renumbered or demoted across the re-authoring
- [ ] `make ze-ipsec-interop-test` run, or the read that established no wire-visible change
      cited as `file:line`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional Python suite green through `--selftest` and `go test`
- [ ] Interop tests for protocol features: N-A for the machinery; CONDITIONAL for the pilot.
      Where phase 8 changed wire-visible IKEv2 behavior, `make ze-ipsec-interop-test`
      (strongSwan, `test/ipsec-interop/`) is required and its result pasted. Where it did
      not, cite the producing code read to establish that, not an assumption
- [ ] Every new check mutation-verified: disable the producing code, confirm the test flips
      red, revert immediately (`ai/rules/functional-test-gate.md`)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-rfcgate-1-extraction.md`, carrying the
      corrected RFC 4271 §8.2.2 figure, the measured register split, both owner rulings with
      their dates and rationales, the real count the rfc7296 walk produced, and any
      annotation Thomas authorised with the requirement id and his reason
      (`ai/rules/rfc-compliance.md:19-20`)
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-rfcgate-1-extraction.md` only

---

## Implementation Summary

### What Was Implemented

Phases 1 to 7, the whole machinery, and nothing else (owner ruling 3).

- **Derivation** (`scripts/dev/rfc_requirements.py:1717-1967`): `source_path` /
  `source_text` (one two-location lookup, now also used by `source_keyword_count:1366`),
  `_strip_page_furniture`, `_section_bodies`, `_sentences`, `_sites_for`,
  `derive_register`, `derive_inventory` with a sha-keyed memo.
- **Artifact** (`:1970-2251`): `Extraction` / `ExtractionSite` / `ExtractionSection`,
  `parse_extraction_artifact` with closed enums (`REGISTERS:1624`,
  `EXCLUSION_KINDS:1629`, `SECTION_SKIP_KINDS`), `_reject_unknown_keys`, and the
  eleventh top-level field `register-reason` (DEV-1).
- **Skeleton writer** (`:2253-2450`): `_artifact_document`, `_validated_stem`,
  `_sweep_stale_staging`, `run_extract_skeleton` -- which round-trips its own output
  through the real parser in a staging dir before `os.replace`, so a derivation defect
  can never land a file the gate cannot read.
- **The two arithmetics** (`_evaluate_extraction:2469-2633`, `evaluate_extractions:2636`,
  `check_extraction_signoff:2673`): forward (every derived site classified, every derived
  field re-derived and compared) and reverse (every gated requirement backed by a site or
  declared `unsourced-ids`), plus the site-level-vs-row-level check at `:2566-2587`.
- **Ratchets** (`_git_baseline_extractions:2689`, `check_extraction_ratchet:2754`) and the
  **enrolment precondition** (`check_enrolment:706-716`, driven from `run_check:3233-3238`).
- **Publication** (`credited:2818`, `register_counts:2839`, `_register_phrase:2851`,
  `extraction_status:2855`, `run_extraction_status:2883`, `render_extraction_table:2894`)
  and the **floor comparison** (`parse_drain_budget:2974`, `required_floor:3050`,
  `check_drain_floor:3091`), all wired at `run_check:3293-3295` inside the existing `try`.
- **Entry points**: `Makefile:452` (`ze-rfc-extract`), `Makefile:460`
  (`ze-rfc-extraction-status`), dispatched from `main:3389-3410`. `ze-rfc-check`
  (`Makefile:437`) is unchanged, so `stagesForMode` and its golden were untouched.
- **Created**: `rfc/extraction/README.md`, `rfc/drain-budget.txt` (`start 2026-07-29`,
  `rate 0`), `plan/deferrals/rfcgate-1-extraction.md`.
- **Tests**: `scripts/dev/rfc_requirements_test.py` grew from 217 to **301** tests.
- **Discovery**: `ai/rules/rfc-compliance.md` (Extraction Completeness rewritten; the
  ratchet table now reads **five** rows, `:124-128`), `ai/skills/ze-rfc.md` (the
  two-grep honour-system coverage self-check replaced by the recorded sign-off),
  `ai/INDEX.md:214-215` and `:375`, `docs/functional-tests.md:693-713`,
  `docs/contributing/rfc-implementation-guide.md:563-601`, `ai/RFC-REQUIREMENTS.md`
  (regenerated, gains the Extraction sign-off table at `:182`).

### Bugs Found/Fixed

Six, all found by independent review or by mutation, none by the implementer. Each is a
Deviations row (DEV-4 to DEV-7) or a Phase Results entry, and each now has a test.

| Bug | Fix | Test that now covers it |
|-----|-----|------------------------|
| `_SECTION_HEADING_RE` matched column-0 table rows and TOC lines, so four ENROLLED stems (rfc1195, rfc2865, rfc2869, sflow-v5) derived a duplicate section id; the skeleton their own parser refused, and rfc1195 silently lost 6 sentences to locator collision | `_section_bodies:1783-1820` merges a repeated id and keeps the matched line's title in the body; `derive_inventory:1941-1953` asserts uniqueness at the producer; `run_extract_skeleton:2417-2438` stages and round-trips before writing | `TestRealTreeExtraction.test_every_enrolled_stem_round_trips_through_the_parser`, `test_no_enrolled_stem_derives_a_duplicate_locator`, `TestSkeletonWriter.test_a_skeleton_the_parser_would_refuse_is_never_written` |
| The drain floor capped at the RESIDUAL backlog while comparing a CUMULATIVE total, collapsing the condition to `signed >= enrolled/2` (DEV-4) | `required_floor:3050-3088` takes `drainable` and `check_drain_floor:3120` passes `len(enrolled)` | `TestDrainFloor.test_half_the_corpus_does_not_satisfy_an_armed_schedule`, `test_the_floor_can_demand_the_whole_enrolled_set`, `test_a_fully_drained_corpus_is_permanently_green` |
| An un-enrolled sign-off raised drain credit without lowering the backlog, publishing `signed + backlog > enrolled` | `credited:2818-2836`, read by the floor, the envelope and the ledger | `TestDrainFloor.test_an_unenrolled_signoff_earns_no_drain_credit`, `test_a_signoff_counts_the_moment_its_stem_enrols` |
| `signed` defined as `sum(counts.values())` made AC-22 a tautology no test could fail (found by mutation) | `extraction_status:2876` counts the total independently of the split | `TestExtractionStatus.test_signed_by_register_sums_to_total` |
| Two tests asserted the live corpus's current census (`extraction_stems() == set()`, the shipped rate of 0), so the first sign-off and the arming commit would each have redded one (DEV-6) | Both re-expressed as properties over FIXTURE trees | `TestGrandfatheredBacklog` (whole class, `:4331-4360`), `TestDrainFloor.test_a_rate_of_zero_reads_off_disk_as_an_inert_floor` |
| `scripts/dev/changed-pkgs.sh` filtered every git query on `*.go`, so a Python-only or corpus-only change selected zero packages and `make ze-verify-changed` could not run this diff's own tests (DEV-7) | `PATHSPECS=('*.go' '*.py' 'rfc')` with `*.py` and `rfc/*` mapped to `PYTHON_TEST_PKG="./scripts/dev"` (`changed-pkgs.sh:46-90`) | the mapping is exercised by `go test ./scripts/dev/ -run TestPythonUnitTests`, which now gets selected |
| `errs.extend(parse_errs)` was the only line turning an unparseable ENROLLED summary into a violation, and nothing drove it: deleting it left all 295 tests green while an unparseable summary exited 0 with every MUST of that RFC unchecked | The line is unchanged (`:3273`); the gap was the missing test | `TestSummaryParseErrorWiring` (`:1806-...`), driven through the real parser over a real temporary `SUMMARY_DIR` |

### Documentation Updates

Every row of the Documentation Update Checklist that was answered Yes for the MACHINERY,
each with a source anchor, verified in the Documentation Verified table below.

- `ai/rules/rfc-compliance.md` -- Extraction Completeness rewritten; ratchet table five rows.
- `ai/skills/ze-rfc.md:30`, `:51`, `:65`, `:69` -- canonical source; `make ze-ai-sync`
  regenerates `.claude/`, `.codex/`, `.agents/` skill trees.
- `ai/INDEX.md:214-215` (Dev Tools) and `:375` (keyword row).
- `docs/functional-tests.md:693-713`, anchored to
  `check_extraction_signoff`/`run_extract_skeleton`.
- `docs/contributing/rfc-implementation-guide.md:563-601`, anchored to
  `run_extract_skeleton`/`check_extraction_signoff`, `Makefile -- ze-rfc-extraction-status`
  and `run_extraction_status`.
- `ai/RFC-REQUIREMENTS.md:182-186` -- generated, never hand-edited, kept fresh by
  `check_ledger_fresh`.
- Row 9 (RFC behavior implemented/changed/newly proven) was answered Yes FOR PHASE 8 ONLY.
  It moves with phase 8: `docs/features/rfc-status.md` and `rfc/enrolled.txt` are unmodified
  in this diff, and the obligation is now
  `plan/spec-rfcgate-1b-rfc7296-pilot.md`'s. Rows 1, 7 and 11 were to be RE-ANSWERED after
  phase 8; that re-answer moves with it for the same reason and for the same evidence.
- `make ze-doc-test`: reported exit 0 by the implementation phase. The tree currently also
  carries a CONCURRENT session's uncommitted MCP work (`ai/digests/mcp.md` and the deleted
  `internal/component/mcp/*.go`), which is why the doc gate is re-verified here per-claim
  by grep rather than by a whole-tree re-run this spec does not own.

### Deviations from Plan

See the `## Deviations` section: DEV-1 (`register-reason` absent from the Design field
table), DEV-2 (AC-21's literal column shape), DEV-3 (a correction recorded but not
applied), DEV-4 to DEV-7 (the four review-found defects, all now fixed). Added at closure:
owner ruling 3 removed phase 8 and AC-23 to AC-26 from this spec entirely.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A correction was written into the Phase Results table (`make ze-rfc-extraction-status --json` is not runnable) and then applied at NONE of the eleven sites that carried the wrong spelling -- including the two Deliverables rows instructing this spec's own closer to run a command that exits 2 | Recording a correction changes nothing until the sites it names are edited (`ai/rules/fix-dont-record.md`) | Independent review, 2026-07-29 | All eleven struck and corrected in place; DEV-3 |
| approach | Two tests were written against the LIVE corpus census (`extraction_stems() == set()`, and the shipped rate of 0) inside the very machinery whose purpose is to make the first sign-off possible | A ratchet that reds on the work it demands gets deleted rather than obeyed -- the exact failure `ai/rules/rfc-compliance.md` records, built into the thing meant to prevent it | Independent review, 2026-07-29 | Both re-expressed as properties over fixture trees (`TestGrandfatheredBacklog`, `TestDrainFloor.test_a_rate_of_zero_reads_off_disk_as_an_inert_floor`); DEV-6 |
| approach | The drain floor's credit and its denominator described two different sets (cumulative total against residual backlog), and separately credit counted un-enrolled sign-offs the backlog did not | A quota whose two sides measure different sets can be satisfied without draining anything | Independent review driving `required_floor` directly, 2026-07-29 | `drainable` parameter + `credited()`; DEV-4 and the `credited` bug row above |
| approach | 252 fixture tests were green over a skeleton writer that bricked four enrolled stems, and 295 were green over a `run_check` line whose deletion stopped every MUST of an unparseable summary being checked | Fixture coverage says nothing about real RFC formatting, and a line no test drives is unproven however obvious it looks | Live-corpus round-trip test + mutation testing, 2026-07-29 | `TestRealTreeExtraction` round-trip and locator tests; `TestSummaryParseErrorWiring` |
| assumption | A-3 was stated as "22 enrolled RFCs have zero capitalised MUST-level keywords" and its validation wording said every such stem derives the `prose` register | The live figure is **23** at the SITE denominator (`rfc5443`'s five uppercase occurrences are all inside its own 2119 boilerplate), and `rfc1877` derives `manual-walk`, not `prose` -- exactly what A-5 predicts | Phase 2 re-derivation over the live tree | Recorded in Phase Results; the shipped test asserts the property that matters (`test_keyword_free_sources_never_derive_rfc2119`) |

## Implementation Audit

Scope of this audit: **phases 1 to 7 only**. AC-23 to AC-26 were removed from this spec by
owner ruling 3 and are audited by `plan/spec-rfcgate-1b-rfc7296-pilot.md`.

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Bound the gate's blind spot with a per-RFC extraction sign-off a machine can re-check | Done | `_evaluate_extraction:2469-2633`, `check_extraction_signoff:2673`, wired `run_check:3293` | Both arithmetics implemented; every derived field re-derived and compared |
| Gate a NEW enrolment on that sign-off | Done | `check_enrolment:706-716`, caller `run_check:3233-3238` | `newly_enrolled` computed by the caller so an unavailable baseline accuses nobody |
| Grandfather the remaining corpus as an explicitly counted and PUBLISHED backlog | Done | `render_extraction_table:2894`, `run_check:3329-3333` | Live: 166 unsigned, published in the ledger and on the success line |
| Ratchet the signed-off count so it can only rise | Done | `check_extraction_ratchet:2754-2815` | Plus exclusions shrink-only per stem |
| Publish the per-register numbers the drain policy is read against | Done | `extraction_status:2855-2880`, `run_extraction_status:2883` | `signed-by-register` with every register present at zero |
| Implement the floor COMPARISON that reads that policy | Done | `parse_drain_budget:2974`, `required_floor:3050`, `check_drain_floor:3091` | Ships inert at `rate 0`; no rate, date or cadence is defaulted in code |
| Do NOT author the drain POLICY | Done | `rfc/drain-budget.txt` (authored, two fields), `parse_drain_budget:2982-2984` | The closed key set makes a per-stem row a parse error |
| ~~Perform the rfc7296 pilot sign-off~~ | Changed | -- | **MOVED by owner ruling 3** to `plan/spec-rfcgate-1b-rfc7296-pilot.md`. Phase 8a RAN (214 obligations counted, taken to the owner); 8b to 8e are the destination spec's. `plan/deferrals/rfcgate-1-extraction.md:8` |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `check_enrolment:706-716`; `TestEnrolmentSignoffPrecondition.test_new_enrolment_without_signoff_fails`; grandfather twin `test_preexisting_enrolment_without_signoff_passes`; end-to-end `TestExtractionSignoffWiring.test_run_check_fails_on_new_enrolment_without_signoff` | Both arms proven: a new stem fails, a pre-HEAD stem passes |
| AC-2 | Done | `run_extract_skeleton:2385-2450`, `_artifact_document:2253`; `TestSkeletonWriter.test_skeleton_writes_every_site_unclassified`, `test_a_generated_skeleton_fails_the_check` | Every site and section null; the writer prints the unclassified counts |
| AC-3 | Done | `_evaluate_extraction:2553-2558` (site) and `:2611-2615` (section); `TestExtractionSignoff.test_unclassified_site_fails`, `test_unclassified_section_fails` | Error names the locator and quotes the sentence |
| AC-4 | Done | `_evaluate_extraction:2498-2508`; `TestExtractionSignoff.test_source_sha_mismatch_fails`, `test_source_sha_mismatch_reports_once_not_per_site` | Returns early with one accurate error, on `verdict_is_fresh`'s over-trigger bias |
| AC-5 | Done | `_evaluate_extraction:2559-2564`; `TestExtractionSignoff.test_mapped_to_unknown_requirement_fails` | |
| AC-6 | Done | `_evaluate_extraction:2623-2632`, `unsourced-ids` escape `:2617-2622`; `TestExtractionSignoff.test_gated_requirement_with_no_site_fails`, `test_unsourced_id_recorded_on_a_section_passes`, `test_unsourced_ids_naming_an_unknown_requirement_fails` | The reverse arithmetic; advisory rows exempt (`test_advisory_requirement_needs_no_site`) |
| AC-7 | Done | `parse_extraction_artifact:2184-2188` (kind) and `:2189-2193` (reason); `TestExtractionArtifact.test_unknown_exclusion_kind_fails`, `test_missing_reason_on_exclusion_fails`, `test_empty_reason_on_exclusion_fails` | `ParseError`, same shape as `_parse_annotation` |
| AC-8 | Done | `_evaluate_extraction:2588-2596`; `TestExtractionSignoff.test_duplicate_of_must_name_a_mapped_id`, `test_duplicate_of_needs_the_id_it_duplicates`, twin `test_duplicate_of_passes_when_another_site_maps_the_id` | |
| AC-9 | Done | `derive_register:1853-1864` + refusal `_evaluate_extraction:2515-2523`; `TestPre2119FailsClosed.test_keyword_free_source_cannot_claim_rfc2119`, permissive twin `test_a_weaker_claim_than_the_derivation_is_allowed` | |
| AC-10 | Done | `derive_register:1862-1864`, `_evaluate_extraction:2491-2496`; `TestPre2119FailsClosed.test_empty_inventory_with_gated_musts_is_refused`, `TestExtractionSignoff.test_manual_walk_without_a_register_reason_is_refused`, `TestPre2119FailsClosed.test_manual_walk_passes_when_every_gated_must_is_declared_unsourced` | The `register-reason` field is DEV-1 |
| AC-11 | Done | Credit: `register_counts:2839-2848`, `extraction_status:2876`; counterweight: `_register_phrase:2851`, `render_extraction_table:2925-2930`, `run_check:3329-3333`. `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` + `TestExtractionLedger.test_registers_are_published_in_separate_columns`, `test_no_bare_signed_total_is_rendered` | Both halves of owner ruling 1 proven separately |
| AC-12 | Done | `check_extraction_ratchet:2779-2784`; `TestExtractionRatchet.test_signoff_count_is_monotonic`, twins `test_a_retained_signoff_passes`, `test_a_new_signoff_is_not_a_violation` | |
| AC-13 | Done | `check_extraction_ratchet:2786-2814`; `test_exclusions_are_shrink_only`, `test_resign_reason_permits_a_rise`, `test_a_carried_over_resign_reason_does_not_license_a_second_rise`, `test_resign_reason_without_a_bumped_date_still_fails` | The carried-forward-reason case is a review addition |
| AC-14 | Done | `run_check:3324-3333`; `TestSuccessLine.test_a_clean_run_publishes_the_extraction_bound`, `test_a_clean_run_with_nothing_signed_still_says_so`. Live: `extraction: rfc2119 0, prose 0, manual-walk 0 signed off of 166 enrolled; 166 unsigned (grandfathered backlog).` | Zero is stated out loud |
| AC-15 | Done | `render_extraction_table:2894-2967`, freshness via `check_ledger_fresh:3153`; `TestExtractionLedger.test_table_reports_unsigned_backlog`, `test_signed_rows_carry_their_derived_counts`, `test_stale_extraction_table_fails_check_fresh`, `test_extraction_table_is_in_the_rendered_ledger`. Live: `ai/RFC-REQUIREMENTS.md:182`, 166 `UNSIGNED (grandfathered)` rows | Derived columns blank for unsigned stems: honest, and the measured reason is the 1.94s cost |
| AC-16 | Done | `extraction_status:2855-2880`, `run_extraction_status:2883-2891`, dispatch `main:3389-3395`, `Makefile:460`; `TestExtractionStatus.test_json_envelope_carries_counts_and_registers`, `TestExtractionStatusWiring.test_extraction_status_dispatches_from_main`. Live envelope pasted below | Spelling corrected to `make ze-rfc-extraction-status` (DEV-3) |
| AC-17 | Done | `check_extraction_ratchet:2763-2765`, `_git_baseline_extractions:2689`; `TestExtractionRatchet.test_git_failure_judges_nothing`, `test_no_extraction_dir_at_head_accuses_nobody`; enrolment side `TestExtractionSignoffWiring.test_an_unavailable_enrolment_baseline_accuses_nobody` with discriminating twin `test_an_available_baseline_still_accuses_a_new_enrolment` | The empty-versus-None polarity is restated in `check_enrolment`'s docstring `:668-678` |
| AC-18 | Done | `parse_extraction_artifact:2068` and the other `ParseError` sites, caught at `run_check:3298-3304`; `TestExtractionArtifact.test_unreadable_artifact_raises_parse_error`, `test_missing_file_raises_parse_error` | Clean `cannot run`, exit 2, never a traceback |
| AC-19 | Done | Scope, not an allowlist: `evaluate_extractions:2657` iterates `load_extractions()` only. `TestGrandfatheredBacklog.test_an_unsigned_backlog_is_accused_by_nothing`, `TestEnrolmentSignoffPrecondition.test_preexisting_enrolment_without_signoff_passes`. Live: `make ze-rfc-check` exit 0 with zero `rfc/extraction/*.json` | |
| AC-20 | Done | quote `_evaluate_extraction:2547-2552`, section count `:2606-2610`, register `:2515-2523`, site set `:2527-2537`; `TestExtractionSignoff.test_hand_edited_quote_fails`, `test_hand_edited_section_count_fails`, `test_an_invented_site_fails`, `test_a_site_missing_from_the_artifact_fails` | |
| AC-21 | **Changed** | `render_extraction_table:2925-2930` renders ONE `Register` column per RFC row plus a per-register summary line above the table (`ai/RFC-REQUIREMENTS.md:186`), not a three-column cross-tab. `TestExtractionLedger.test_registers_are_published_in_separate_columns`, `test_no_bare_signed_total_is_rendered` | **DEV-2. The AC's literal wording is not met; its INTENT is, and is mechanically enforced.** `register_counts:2845` seeds every register at zero so none can go missing. Verified by reading all three publishing sites that there is NO bare signed total anywhere. The AC text is unchanged (append-only); accepting this substitution is a CLOSURE decision and is flagged, not self-approved |
| AC-22 | Done | `extraction_status:2869-2880` (total counted independently of the split); `TestExtractionStatus.test_signed_by_register_sums_to_total`, `test_every_register_key_is_present_even_at_zero`, `test_the_envelope_arithmetic_is_self_consistent`. Live envelope below | The tautology this once was is the mutation-found bug above |
| ~~AC-23~~ | Moved | `plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-1 (`:424`) | Owner ruling 3; `plan/deferrals/rfcgate-1-extraction.md:8` |
| ~~AC-24~~ | Moved | `plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-5 (`:425`) | Its machinery half is AC-19, delivered here |
| ~~AC-25~~ | Moved | `plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-4 (`:426`) | NARROWED there: the annotation branch is removed |
| ~~AC-26~~ | Moved | `plan/spec-rfcgate-1b-rfc7296-pilot.md` AC-3 (`:427`) | `rfc/short/rfc7296.md` unmodified in this diff |
| AC-27 | Done | `check_drain_floor:3091-3130`, `required_floor:3088`; shipped `rfc/drain-budget.txt` `rate 0`; `TestDrainFloor.test_rate_zero_computes_a_zero_floor`, `test_a_rate_of_zero_reads_off_disk_as_an_inert_floor`, `TestDrainFloorWiring.test_run_check_clean_at_the_shipped_rate_of_zero`. Live: exit 0 with the backlog still published | |
| AC-28 | **Changed** | `required_floor:3088` is `min(drainable, ceil(rate x months))` with `check_drain_floor:3120` passing `len(enrolled)`; `TestDrainFloor.test_required_floor_never_exceeds_the_drainable_set`, `test_a_fully_drained_corpus_is_permanently_green`, `test_the_floor_can_demand_the_whole_enrolled_set`, `test_half_the_corpus_does_not_satisfy_an_armed_schedule` | **DEV-4.** The AC's phrase "capped at the backlog size" now reads as the DRAINABLE set, not the residual one -- capping at the residual double-counted every sign-off and collapsed the check to `signed >= enrolled/2` (measured: green at 83 of 166). Self-retirement is preserved by a different route and is tested |
| AC-29 | Done | `parse_drain_budget:2986-2995` (missing), `:3004-3046` (malformed, duplicate key, non-finite, negative), surfaced as a violation at `check_drain_floor:3100-3103`; `TestDrainFloor.test_missing_drain_budget_is_error_not_empty`, `test_unparseable_drain_budget_is_an_error`, `test_a_non_finite_rate_is_refused_at_parse_time`, `test_a_duplicate_budget_key_is_refused`, `test_a_budget_naming_an_rfc_is_refused` | The NaN case is a review addition: it would have reached `math.ceil` outside `run_check`'s `try` |
| AC-30 | Done | `check_drain_floor:3121-3130`; `TestDrainFloor.test_signed_below_armed_floor_fails`, `test_the_failure_message_names_the_registers_it_summed`, `test_the_failure_message_names_a_zero_register_too`, end-to-end `TestDrainFloorWiring.test_run_check_fails_when_signed_count_is_below_the_floor` | The floor can actually fail: this is the discriminating case for AC-27 to AC-29 |
| AC-31 | Done | `parse_extraction_artifact:2086-2092`; `TestExtractionArtifact.test_unknown_register_is_hard_error`, `test_a_register_outside_the_closed_set_is_still_a_parse_error` | Does not default to the strong grade |
| AC-32 | Done | `derive_register:1860-1863` (undercount clause) + refusal `_evaluate_extraction:2515-2523`; `TestPre2119FailsClosed.test_rfc2119_register_below_source_count_is_rejected`; corroborated live by `TestRealTreeExtraction.test_a_large_minority_cannot_take_the_rfc2119_grade` | The rfc2181 shape lands here |

### Tests from TDD Plan

Every planned test exists under its planned name except the four phase-8 rows (moved) and
the three below, which are recorded rather than quietly renamed.

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSiteInventory.*` (5 planned) | Done | `rfc_requirements_test.py:2162-2400` | Shipped with 17 more, including the memo-key and determinism cases |
| `TestExtractionArtifact.*` (4 planned) | Done | `:2402-2531` | `test_duplicate_of_must_name_a_mapped_id` lives in `TestExtractionSignoff` instead: it is a CHECK-time cross-site rule, not a parse-time one |
| `TestExtractionSignoff.*` (7 planned) | Done | `:2784-3085` | 25 shipped |
| `TestPre2119FailsClosed.*` | Changed | `:3087-3173` | 4 of the 5 planned names shipped. `test_manual_walk_is_published_in_its_own_column` was NOT written: it is a LEDGER assertion, and the property it names is proved by `TestExtractionLedger.test_registers_are_published_in_separate_columns` and `test_no_bare_signed_total_is_rendered`. Verified absent: `grep -c` returns 0 |
| `TestExtractionStatus.*` (3 planned) | Done | `:3441-3547` | |
| `TestExtractionLedger.*` (3 planned) | Done | `:3549-3674` | Plus `test_no_bare_signed_total_is_rendered` and `test_extraction_table_render_is_independent_of_input_order` |
| `TestDrainFloor.*` | Changed | `:3676-3986` | `test_drain_floor_caps_at_backlog_size` was NOT written under that name: DEV-4 changed the cap's meaning from the residual backlog to the drainable set, and a test asserting the old name would assert the bug. Shipped as `test_required_floor_never_exceeds_the_drainable_set` plus `test_a_fully_drained_corpus_is_permanently_green`. Verified absent: `grep -c` returns 0 |
| `TestExtractionRatchet.*` (4 planned) | Done | `:3175-3328` | 11 shipped |
| `TestSkeletonWriter.*` (2 planned) | Done | `:2533-2782` | 12 shipped, including the staging and round-trip cases |
| `TestEnrolment.test_new_enrolment_without_signoff_fails` / `..._passes` | Changed | `TestEnrolmentSignoffPrecondition:3330-3372` | Class renamed rather than extending `TestEnrolment`; both planned tests exist verbatim |
| `TestRealTree.test_every_enrolled_rfc_derives_a_register` | Changed | `TestRealTreeExtraction.test_every_enrolled_rfc_derives_a_register:4265` | Live-tree extraction class split out from `TestRealTree` |
| `TestRealTree.test_live_tree_is_green_with_zero_signoffs` | Changed | `TestGrandfatheredBacklog:4331-4360` | DEV-6: re-expressed as a property over a fixture tree. The live-census form would have redded on the first sign-off. Verified absent under the old name: `grep -c` returns 0 |
| `TestRealTree.test_rfc7296_*` (4 planned) | Moved | `plan/spec-rfcgate-1b-rfc7296-pilot.md` | Owner ruling 3. All four verified absent: `grep -c` returns 0 |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/rfc_requirements.py` | Done | Modified; all new code `:1617-3130` plus the `run_check` wiring |
| `scripts/dev/rfc_requirements_test.py` | Done | Modified; 217 to 301 tests |
| `Makefile` | Done | `ze-rfc-extract:452`, `ze-rfc-extraction-status:460`; `ze-rfc-check:437` unchanged |
| `ai/RFC-REQUIREMENTS.md` | Done | Regenerated; Extraction sign-off table `:182` |
| `ai/rules/rfc-compliance.md` | Done | Five ratchets `:124-128`; Extraction Completeness rewritten `:82-98` |
| `ai/skills/ze-rfc.md` | Done | `:30`, `:51`, `:65`, `:69` |
| `ai/INDEX.md` | Done | `:214-215`, `:375` |
| `docs/contributing/rfc-implementation-guide.md` | Done | `:563-601` |
| `rfc/extraction/README.md` | Done | Created |
| `plan/deferrals/rfcgate-1-extraction.md` | Done | Created; two rows, both `deferred` with destinations |
| `rfc/drain-budget.txt` | Done | Created; `start 2026-07-29`, `rate 0` |
| `scripts/dev/changed-pkgs.sh` | Changed | NOT in the plan. Added during review: DEV-7's fix, without which scoped verification could not run this diff's tests |
| `docs/functional-tests.md` | Done | `:693-713` (planned via Documentation checklist row 10) |
| `rfc/extraction/rfc7296.json` | Moved | Owner ruling 3; `plan/spec-rfcgate-1b-rfc7296-pilot.md` |
| `rfc/short/rfc7296.md`, `rfc/enrolled.txt`, `docs/features/rfc-status.md`, `internal/component/ike/**` | Moved | Owner ruling 3; all four unmodified in this diff per `git status` |

### Audit Summary
- **Total items:** 28 delivered ACs (AC-1..AC-22, AC-27..AC-32), 8 Task requirements,
  13 test groups, 15 file rows.
- **Done:** 26 of 28 ACs; 7 of 8 Task requirements; 9 of 13 test groups; 13 of 15 file rows.
- **Partial:** none.
- **Skipped:** none.
- **Changed:** AC-21 (DEV-2, rendering shape -- flagged as a closure decision, not
  self-approved), AC-28 (DEV-4, cap semantics corrected), 4 test groups (renames and one
  deliberate non-write), 2 file rows. All recorded in Deviations or in the rows above.
- **Moved by owner ruling 3:** AC-23..AC-26, 4 test rows, 5 file rows, 1 Task requirement,
  1 user story, 1 wiring row. Homed at `plan/deferrals/rfcgate-1-extraction.md:8`.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Bound the gate's blind spot with a sign-off a machine can re-check | functional (the gate itself) | `make ze-rfc-check` exit 0, and the discriminating proof that it is not vacuous: `TestExtractionSignoffWiring.test_run_check_fails_on_unclassified_site` reds `run_check` end-to-end on an unclassified site, with `test_run_check_clean_on_a_fully_classified_signoff` as its twin. Every one of the eight new checks was mutation-verified: disabled in turn, all eight went fully RED and all restored green |
| Make an unearned sign-off structurally impossible rather than discouraged | functional | `TestSkeletonWriter.test_a_generated_skeleton_fails_the_check`: generating skeletons makes the gate REDDER. There is no `--sign-off` mode, no default disposition, and no bulk classifier -- verified by reading `main:3382-3414`, which dispatches only `--selftest`, `--write`, `--check-fresh`, `--extraction-status`, `--extract-skeleton`, `--check` |
| Gate a NEW enrolment on a sign-off without redding the existing 166 | functional, both arms | RED arm: `TestEnrolmentSignoffPrecondition.test_new_enrolment_without_signoff_fails`. GREEN arm: `test_preexisting_enrolment_without_signoff_passes`, plus the live tree -- `make ze-rfc-check` exit 0 over 166 enrolled RFCs with zero artifacts present |
| Publish the backlog rather than claim it away | rendered artifact | `ai/RFC-REQUIREMENTS.md:182` (`## Extraction sign-off`), `:186` (`Signed off by register: rfc2119 0, prose 0, manual-walk 0. Unsigned (grandfathered) backlog: 166 of 166 enrolled.`), and 166 rows reading `UNSIGNED (grandfathered)` (`grep -c` = 166). Kept fresh by `check_ledger_fresh`, proved by `TestExtractionLedger.test_stale_extraction_table_fails_check_fresh` |
| Ratchet the count so it can only rise | functional | `TestExtractionRatchet.test_signoff_count_is_monotonic` + `test_exclusions_are_shrink_only`, each with a passing twin so neither is an "always fails" check; wired through `run_check` by `TestExtractionRatchetWiring.test_run_check_fails_when_a_signoff_disappears` |
| Close the pre-2119 fail-open (the reason a keyword-only design was rejected) | measured over the live corpus | `TestRealTreeExtraction.test_a_large_minority_cannot_take_the_rfc2119_grade` and `test_keyword_free_sources_never_derive_rfc2119` run over all 166 enrolled RFCs. Measured split: `rfc2119` 101, `prose` 64, `manual-walk` 1; **65 of 166** cannot take the strong grade, against a spec estimate of 53 |
| Implement the floor COMPARISON, inert, without authoring policy | functional, with the discriminating case | Inert: `TestDrainFloorWiring.test_run_check_clean_at_the_shipped_rate_of_zero` and the live exit 0. Can actually fail: `TestDrainFloorWiring.test_run_check_fails_when_signed_count_is_below_the_floor` and `TestDrainFloor.test_half_the_corpus_does_not_satisfy_an_armed_schedule`. No policy in code: `parse_drain_budget` defaults nothing (`:2982-2984`) and refuses a file naming an RFC (`test_a_budget_naming_an_rfc_is_refused`) |
| Owner ruling 1 held on BOTH halves | functional, split deliberately | Credit: `TestExtractionStatus.test_every_register_counts_toward_the_signed_total`. Counterweight: `TestExtractionLedger.test_registers_are_published_in_separate_columns` and `test_no_bare_signed_total_is_rendered`. A reviewer who saw only one has verified only one |
| Keep `ze-rfc-check` cheap enough to stay in both verify branches (A-9) | benchmark | `--check` on the live tree 2.52-2.55s to 2.54-2.65s, **+0.07s (+3%)**; `make ze-rfc-check` end to end ~8.8s to 11.7s. Re-measured at closure: 301 tests in 8.685s, whole target exit 0. Inside `_git_cat_blobs:942`'s recorded budget |
| ~~Discharge the two confirmed rfc7296 obligations~~ | -- | **MOVED by owner ruling 3.** Phase 8a's walk DID run and is what produced the ruling: 214 obligations, 108 unimplemented. The goal is not abandoned and not annotated away -- it is `plan/spec-rfcgate-1b-rfc7296-pilot.md`'s, with all 108 to be fixed there (OR-A), and `plan/deferrals/rfcgate-1-extraction.md:8` is the live row that holds it |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The rfc7296 pilot in full: re-author `rfc/short/rfc7296.md` with the 214 walked obligations, prove the 88 already implemented, IMPLEMENT the 108 that are not, resolve the 18 uncertain, sign off `rfc/extraction/rfc7296.json` (`plan/deferrals/rfcgate-1-extraction.md:8`) | **deferred** (live, homed) | `plan/spec-rfcgate-1b-rfc7296-pilot.md` -- exists on disk, Status `design`, `Depends \| spec-rfcgate-4-ledger`, carries all 214 rows in Appendix A (`:875`) and maps AC-23..AC-26 to its AC-1/5/4/3 (`:422-427`). Owner ruling 3 (OR-A + OR-B). Stays LIVE because the work is outstanding: filing work in a spec is not finishing it |
| A closure decision on A-7 (whether derived-site inventories stay stable against future churn in the RFC source texts) (`:9`) | **deferred** (live, homed) | `plan/spec-followup-rfc-enrollment.md` -- owns the fleet drain, the first activity producing the sign-off volume A-7 is a claim about. Not a machinery gap: `_evaluate_extraction:2498-2508` fails an artifact the moment its source moves, so the risk is detected rather than absorbed |

Both rows carry an existing `plan/` destination, so `deferral_unassigned_problems` has
nothing to flag. The shard is deleted by commit B with the spec, per
`ai/rules/deferral-tracking.md`; its rows live on in the two destination specs.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfcgate-1-extraction-6aa27893-1bd2-42e1-9e68-879943aa8740.md` |
| `review_gate.py check` | clean (`verdict=clean`, 5 files hash-pinned: `rfc/drain-budget.txt`, `rfc/extraction/README.md`, `scripts/dev/changed-pkgs.sh`, `scripts/dev/rfc_requirements.py`, `scripts/dev/rfc_requirements_test.py`) |
| Reviewer lenses used | Four independent subagent passes: (1) fail-closed / vacuity, (2) spec conformance with its own 11-mutation battery, (3) test quality with a 30-mutation battery, (4) second-pass adversarial re-review of the 19 fixes with a 14-mutation battery |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The skeleton writer emitted a file its own parser refused for four ENROLLED stems; one such artifact committed makes every later `--check` print `cannot run` and hide every other RFC violation in the repo | `_SECTION_HEADING_RE`, `_section_bodies` | `_section_bodies:1783-1820` merges a repeated id and keeps the heading line's title; `run_extract_skeleton:2417-2438` stages and round-trips before `os.replace`; `TestRealTreeExtraction.test_every_enrolled_stem_round_trips_through_the_parser` |
| 2 | BLOCKER | Locator collision silently dropped obligations (rfc1195 lost 6 sentences) | `derive_inventory`, `_sites_for` | Producer-side uniqueness guard `derive_inventory:1941-1953`; `test_no_enrolled_stem_derives_a_duplicate_locator` |
| 3 | BLOCKER | The drain floor double-counted every sign-off, going permanently green at half the corpus at any armed rate (DEV-4) | `required_floor`, `check_drain_floor` | `drainable` parameter `:3050-3088`; `check_drain_floor:3120` passes `len(enrolled)`; `test_half_the_corpus_does_not_satisfy_an_armed_schedule` |
| 4 | BLOCKER | Credit and denominator described different sets: an un-enrolled sign-off satisfied the floor without draining anything, publishing `signed + backlog > enrolled` | `check_drain_floor`, `extraction_status`, `render_extraction_table` | `credited:2818-2836`, read by all three; `test_an_unenrolled_signoff_earns_no_drain_credit` |
| 5 | ISSUE | Two live-corpus census assertions would red on the first sign-off and on the arming commit -- the machinery reddening the work it exists to enable (DEV-6) | `TestRealTreeExtraction`, `TestDrainFloor` | `TestGrandfatheredBacklog:4331-4360`; `TestDrainFloor.test_a_rate_of_zero_reads_off_disk_as_an_inert_floor` |
| 6 | ISSUE | `signed` defined as `sum(counts.values())` made AC-22 unfalsifiable (mutation-found) | `extraction_status` | Independent count `:2876`; `test_signed_by_register_sums_to_total` |
| 7 | ISSUE | A site quoting a capitalised MUST could be mapped to a SHOULD row and reported as captured, while `evaluate` never gates a SHOULD -- the RFC's own MUST downgraded to advice inside the artifact meant to prevent exactly that | `_evaluate_extraction` | `:2566-2587`; `test_a_must_bearing_site_mapped_to_an_advisory_row_fails` with twins |
| 8 | ISSUE | A carried-forward `resign-reason` let exclusions climb indefinitely behind one sentence written years earlier | `check_extraction_ratchet` | `:2803-2814`; `test_a_carried_over_resign_reason_does_not_license_a_second_rise` |
| 9 | ISSUE | `float("nan")` passed every rate guard and reached `math.ceil` OUTSIDE `run_check`'s `try`, i.e. a traceback where AC-18 requires exit 2 | `parse_drain_budget` | `:3040-3044`; `test_a_non_finite_rate_is_refused_at_parse_time` |
| 10 | ISSUE | `errs.extend(parse_errs)` -- the only line turning an unparseable ENROLLED summary into a violation -- was driven by nothing; deleting it left 295 tests green while every MUST of that RFC stopped being checked | `run_check:3273` | Line unchanged; `TestSummaryParseErrorWiring:1806` drives it through the real parser over a real file |
| 11 | ISSUE | `make ze-verify-changed` could not select this diff's own tests: `changed-pkgs.sh` filtered every git query on `*.go` (DEV-7) | `scripts/dev/changed-pkgs.sh` | `PATHSPECS=('*.go' '*.py' 'rfc')` with `*.py`/`rfc/*` mapped to `./scripts/dev` (`:46-90`) |
| 12 | ISSUE | The inventory memo keyed on the stem alone would hand a second body the first body's inventory, silently and toward green | `derive_inventory` | Four-part key over stem, gated count, RAW bytes and resolved path `:1867-1913`; `test_the_memo_is_keyed_on_the_source_not_the_stem` and three siblings |
| 13 | ISSUE | The `--json` spelling recorded as a correction was applied at none of the eleven sites naming it, including the two Deliverables rows telling this spec's closer to run a command that exits 2 (DEV-3) | this spec and `plan/spec-rfcgate-0-umbrella.md` | All eleven struck and corrected in place |

Run 2 through run 4 each re-reviewed the fixes as new code. The final pass reports
**0 BLOCKER, 0 ISSUE**, 6/6 mutations killed, 301 selftests OK, `make ze-rfc-check` exit 0.

## Pre-Commit Verification

Re-verified at closure, independently of the audit above. Every command below was run in
this session; nothing in this section is copied from the Implementation Audit.

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `rfc/extraction/README.md` | yes | `ls -la` -> `-rw-rw-r-- 1 thomas thomas 9118 Jul 29 13:20 rfc/extraction/README.md` |
| `rfc/drain-budget.txt` | yes | `ls -la` -> `-rw-rw-r-- 1 thomas thomas 1943 Jul 29 14:10 rfc/drain-budget.txt`; `cat` shows `start 2026-07-29` and `rate 0` and no per-stem row |
| `plan/deferrals/rfcgate-1-extraction.md` | yes | `ls -la` -> `-rw-rw-r-- 1 thomas thomas 3952 Jul 29 12:39`; two rows, both `deferred` with `plan/` destinations |
| `plan/spec-rfcgate-1b-rfc7296-pilot.md` (the destination created by owner ruling 3) | yes | `ls -la` -> `-rw-rw-r-- 1 thomas thomas 135256 Jul 29 13:25`; `Status \| design`, `Depends \| spec-rfcgate-4-ledger` |
| `rfc/extraction/*.json` | **none, deliberately** | `ls rfc/extraction/` lists `README.md` only. Phase 8 moved, so this spec ships zero artifacts and the machinery lands on the AC-19 shape |
| `scripts/dev/rfc_requirements.py`, `rfc_requirements_test.py`, `changed-pkgs.sh`, `Makefile` | yes, modified | `git status --porcelain` shows ` M` for each |
| `rfc/short/rfc7296.md`, `rfc/enrolled.txt`, `docs/features/rfc-status.md`, `internal/component/ike/**` | untouched | absent from `git status --porcelain`, which is the proof phase 8 was not partially performed |

### AC Verified (grep/test)

Two `python3 -m unittest -v` runs over named tests, executed now, plus live command output.

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A new enrolment without a sign-off fails; a pre-HEAD one does not | Run A: `TestEnrolmentSignoffPrecondition.test_new_enrolment_without_signoff_fails` ok. Run B: `test_preexisting_enrolment_without_signoff_passes` ok ("AC-19: the 166 stay green when the machinery lands") |
| AC-2, AC-3 | A skeleton is all-null and CANNOT pass | Run A: `TestSkeletonWriter.test_skeleton_writes_every_site_unclassified` ok, `test_a_generated_skeleton_fails_the_check` ok, `TestExtractionSignoff.test_unclassified_site_fails` ok |
| AC-4, AC-5, AC-6 | sha mismatch, unknown id, and the reverse arithmetic all red | Run A: `test_source_sha_mismatch_fails`, `test_mapped_to_unknown_requirement_fails`, `test_gated_requirement_with_no_site_fails` -- all ok |
| AC-7, AC-8 | Closed exclusion set, mandatory reason, duplicate chain refused | Run A: `TestExtractionArtifact.test_missing_reason_on_exclusion_fails`, `test_unknown_exclusion_kind_fails`, `TestExtractionSignoff.test_duplicate_of_must_name_a_mapped_id` -- all ok |
| AC-9, AC-10, AC-32 | The pre-2119 fail-open is closed at both arms | Run A: `TestPre2119FailsClosed.test_keyword_free_source_cannot_claim_rfc2119` ok ("the register is a property of the SOURCE"), `test_empty_inventory_with_gated_musts_is_refused` ok. Run B: `test_rfc2119_register_below_source_count_is_rejected` ok |
| AC-11, AC-21, AC-22 | Every register credits the total, and no bare total is published | Run A: `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` ok, `test_signed_by_register_sums_to_total` ok, `TestExtractionLedger.test_registers_are_published_in_separate_columns` ok, `test_no_bare_signed_total_is_rendered` ok |
| AC-12, AC-13, AC-17 | Monotonic sign-off, shrink-only exclusions, git failure judges nothing | Run A: `TestExtractionRatchet.test_signoff_count_is_monotonic`, `test_exclusions_are_shrink_only`, `test_git_failure_judges_nothing` -- all ok |
| AC-14 | The bound is stated out loud, including at zero | Live `make ze-rfc-check`: `extraction: rfc2119 0, prose 0, manual-walk 0 signed off of 166 enrolled; 166 unsigned (grandfathered backlog).` Run B: `TestSuccessLine.test_a_clean_run_with_nothing_signed_still_says_so` ok |
| AC-15 | The ledger publishes the backlog and a stale table reds | `grep -n '## Extraction sign-off' ai/RFC-REQUIREMENTS.md` -> `182`; `grep -c 'UNSIGNED (grandfathered)'` -> `166`. Run B: `TestExtractionLedger.test_table_reports_unsigned_backlog`, `test_stale_extraction_table_fails_check_fresh` -- ok |
| AC-16 | The status envelope is emitted and parses | Live `make ze-rfc-extraction-status` exit 0, emitting `{"backlog": 166, "enrolled": 166, "schema-version": 1, "signed": 0, "signed-by-register": {"manual-walk": 0, "prose": 0, "rfc2119": 0}, "unsigned": [...166 stems...]}`. Run B: `TestExtractionStatus.test_json_envelope_carries_counts_and_registers` ok |
| AC-18 | A malformed artifact is a clean exit 2 | Run A: `TestExtractionArtifact.test_unreadable_artifact_raises_parse_error` ok ("malformed JSON is a clean exit-2, never a traceback") |
| AC-19 | The live tree is green with zero artifacts | `make ze-rfc-check` -> `EXIT=0`, `rfc-requirements OK: 2720 gated MUST-level requirement(s) across 166 enrolled RFC(s); 2575 test tag(s) resolved.` with `ls rfc/extraction/` showing no `.json`. Run A: `TestGrandfatheredBacklog.test_an_unsigned_backlog_is_accused_by_nothing` ok |
| AC-20 | A hand-edited derived field reds | Run A: `TestExtractionSignoff.test_hand_edited_quote_fails` ok ("every derived field is re-derived and compared") |
| AC-27, AC-28 | Inert at rate 0; the cap is the drainable set, not the residual | Run B: `test_rate_zero_computes_a_zero_floor`, `test_a_rate_of_zero_reads_off_disk_as_an_inert_floor`, `TestDrainFloorWiring.test_run_check_clean_at_the_shipped_rate_of_zero`, `test_required_floor_never_exceeds_the_drainable_set`, `test_a_fully_drained_corpus_is_permanently_green`, `test_the_floor_can_demand_the_whole_enrolled_set`, `test_half_the_corpus_does_not_satisfy_an_armed_schedule` -- all ok |
| AC-29, AC-30 | An absent policy errors; an armed floor can actually fail | Run B: `test_missing_drain_budget_is_error_not_empty`, `test_unparseable_drain_budget_is_an_error`, `test_signed_below_armed_floor_fails`, `TestDrainFloorWiring.test_run_check_fails_when_signed_count_is_below_the_floor` -- all ok |
| AC-31 | An unknown register is a hard parse error | Run B: `TestExtractionArtifact.test_unknown_register_is_hard_error`, `test_a_register_outside_the_closed_set_is_still_a_parse_error` -- ok |
| AC-23..AC-26 | not this spec's | `grep -c test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered scripts/dev/rfc_requirements_test.py` -> `0`; `git status` shows `rfc/short/rfc7296.md` unmodified. Owned by `plan/spec-rfcgate-1b-rfc7296-pilot.md` |

Totals for the two runs, pasted: `Ran 22 tests in 0.029s / OK` and
`Ran 20 tests in 0.671s / OK`. Whole suite at closure: `Ran 301 tests in 8.685s / OK`
(inside `make ze-rfc-check`), and `Ran 301 tests in 8.948s / OK` under
`python3 -m unittest scripts.dev.rfc_requirements_test`.

### Wiring Verified (end-to-end)

No `.ci` row exists or should: this is developer tooling with no daemon surface. The driving
surface is the Python suite, and every row below was re-read in
`scripts/dev/rfc_requirements_test.py` to confirm it drives `run_check` or the CLI entry
point through `_ExtractionDrive` / `_patched`, never the helper alone.

| Entry Point | Test file / class (read, not inferred) | Verified |
|-------------|----------------------------------------|----------|
| `make ze-rfc-check` on a new enrolment with no sign-off | `TestExtractionSignoffWiring(_ExtractionDrive):4015-4082` -- drives `run_check`, has the grandfather twin AND the baseline-unavailable pair | yes; ran ok |
| `make ze-rfc-check` on an unclassified site | `TestExtractionSignoffWiring.test_run_check_fails_on_unclassified_site:4064` with clean twin `:4075` | yes |
| `make ze-rfc-check` on a deleted HEAD sign-off | `TestExtractionRatchetWiring:4112-4136`, twin `test_run_check_clean_when_the_signoff_is_still_there` | yes |
| `make ze-rfc-extract STEM=<stem>` | `TestSkeletonWriterWiring:4157-4177` -- dispatches from `main`, plus `test_extract_skeleton_without_a_stem_fails_closed` | yes |
| `make ze-rfc-extraction-status` | `TestExtractionStatusWiring.test_extraction_status_dispatches_from_main:4180`; live run exit 0 with valid JSON | yes |
| `make ze-rfc-check` with an armed rate above the signed count | `TestDrainFloorWiring.test_run_check_fails_when_signed_count_is_below_the_floor:4139`, twin `:4148` | yes |
| `make ze-rfc-check` with `rfc/drain-budget.txt` deleted | `TestDrainFloor.test_missing_drain_budget_is_error_not_empty:3888` -- asserts the message names `drain-budget.txt` | yes |
| `make ze-rfc-index` then `make ze-rfc-check` | `TestExtractionLedger.test_stale_extraction_table_fails_check_fresh:3631` -- through the EXISTING `check_ledger_fresh` | yes |
| `make ze-rfc-index` with sign-offs in more than one register | `TestExtractionLedger.test_registers_are_published_in_separate_columns:3561` + `test_no_bare_signed_total_is_rendered:3580` | yes |
| The Python suite is actually discovered and run by Go | `go test ./scripts/dev/ -run TestPythonUnitTests -count=1` -> `ok github.com/ze-software/ze/scripts/dev 57.747s`, EXIT=0 | yes |
| ~~`make ze-rfc-check` after the rfc7296 pilot lands~~ | -- | MOVED with AC-24 to `plan/spec-rfcgate-1b-rfc7296-pilot.md` |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Live derivation over 166 RFCs: 4208 capitalised sites (est. 3988), 69 RFCs with more sites than captured (est. 61), raw deficit 1845 (est. 1683). Every figure within ~10% and in the same direction |
| A-2 | confirmed, minority LARGER than estimated | Measured register split `rfc2119` 101 / `prose` 64 / `manual-walk` 1; **65 of 166** cannot take the strong grade against an estimate of 53. Gated live by `TestRealTreeExtraction.test_a_large_minority_cannot_take_the_rfc2119_grade` |
| A-3 | confirmed, at a stated denominator | **23**, not 22, at the SITE denominator; `rfc5443` is the whole difference (its 5 uppercase occurrences sit inside its own 2119 boilerplate). Both figures right at their own denominator. `test_keyword_free_sources_never_derive_rfc2119` |
| A-4 | confirmed | 777 prose sites over those stems (est. 688), so the prose register rescues almost all of them: `test_the_prose_register_rescues_almost_all_of_them` |
| A-5 | confirmed exactly | `rfc1877` is the sole live `manual-walk` stem, 4 gated MUSTs, empty under both scans. The register is retained regardless of the count, per owner ruling 1 |
| A-6 | confirmed MORE strongly | 2862 hole-sites sit in sections that ALREADY carry ids (est. 558), so section-only granularity would miss **48%** of them, not 25%. Site granularity is required by evidence |
| A-7 | **deferred with a destination** | A claim about FUTURE churn; no evidence inside one session settles it. Detected rather than absorbed: `_evaluate_extraction:2498-2508` fails an artifact the moment its source moves. Homed at `plan/deferrals/rfcgate-1-extraction.md:9` -> `plan/spec-followup-rfc-enrollment.md`. NOT left `unvalidated` |
| A-8 | confirmed | The envelope is emitted and consumed as designed (live `make ze-rfc-extraction-status`), and the counting RULE is owner ruling 1, not a child's design choice |
| A-9 | confirmed | `--check` +0.07s (+3%); `make ze-rfc-check` 11.7s end to end, re-measured green at closure. Well inside `_git_cat_blobs:942`'s budget |
| A-10 | confirmed by the walk | The phase-8a walk returned 214 distinct obligations; no pre-walk figure predicted it (this spec's own estimate was 263 SITES, a different unit). `plan/deferrals/rfcgate-1-extraction.md:8` |
| A-11 | **moved with phase 8** | Its validation method IS phase 8c, which owner ruling 3 assigned to `plan/spec-rfcgate-1b-rfc7296-pilot.md`. That spec already records the implementation-side finding at `:181-182`; the proof-side question A-11 asks is its phase 8c work |

No assumption is left `unvalidated`: nine are confirmed, one is deferred with a named
destination spec, one moved with the phase that owns its validation method.

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 10 (test infrastructure): `docs/functional-tests.md` documents the extraction sign-off | `grep -n` -> `:693-711` describes the artifact and the two make targets; `:713` carries `<!-- source: scripts/dev/rfc_requirements.py -- check_extraction_signoff/run_extract_skeleton -->`, and both symbols exist (`:2673`, `:2385`) | yes |
| Row 10: `docs/contributing/rfc-implementation-guide.md` carries the sign-off step | `grep -n` -> `:563-564` (checklist), `:581` (command), `:591` (contract), anchors at `:593`, `:600`, `:601`. `:596-597` warns that `make ze-rfc-extraction-status --json` is unrunnable -- the DEV-3 correction actually applied | yes |
| Row 16 (existing anchors naming changed files): `ai/INDEX.md` Dev Tools and keyword rows | `grep -n` -> `:214`, `:215`, `:375`; both make targets exist (`Makefile:452`, `:460`) and the described behavior matches `run_extract_skeleton` / `run_extraction_status` | yes |
| Row 17 (existing docs showing the superseded method): `ai/skills/ze-rfc.md` two-grep self-check replaced | `grep -n` -> `:30` (the enrolment precondition), `:51` (`make ze-rfc-extract STEM=$ARGUMENTS`), `:65` (contract), `:69` (backlog + status target). Canonical source, so `make ze-ai-sync` regenerates the three skill trees | yes |
| `ai/rules/rfc-compliance.md` ratchet table reads FIVE | `grep -n` -> rows at `:124`, `:125`, `:126`, `:127`, `:128`, the fifth being `check_extraction_ratchet`. Also `:82` (the recorded walk) and `:98` (the first-sign-off limitation) | yes |
| `ai/RFC-REQUIREMENTS.md` extraction table is generated and current | `grep -n '## Extraction sign-off'` -> `:182`; `:186` carries the register split; `grep -c 'UNSIGNED (grandfathered)'` -> `166`. `make ze-rfc-check` exit 0 includes `check_ledger_fresh`, which re-renders and compares, so a stale table would have redded | yes |
| Row 9 (RFC behavior) and the re-answers of rows 1, 7, 11 | `git status --porcelain` shows `docs/features/rfc-status.md`, `rfc/enrolled.txt` and `rfc/short/rfc7296.md` ABSENT, i.e. unmodified. The machinery changes no protocol behavior, so all four are No for this spec; the Yes moved with phase 8 to `plan/spec-rfcgate-1b-rfc7296-pilot.md` | yes |
| Rows 2, 3, 4, 5, 8, 12, 13, 14, 15 (config, CLI, API/RPC, plugin, SDK, architecture, route metadata, metrics, registry inventory) | No `ze` subcommand, RPC, YANG, plugin or metric was added: the new entry points are two make targets on a dev script (`Makefile:452`, `:460`) and two flags on `main:3389-3403`. `git status` shows no `internal/`, `pkg/` or `cmd/` file in this diff except the concurrent session's MCP work | yes |

## Core Insight

A gate that checks whether every listed obligation is proven cannot see an obligation
nobody listed, and no amount of strengthening that comparison ever will: the missing oracle
is the RFC's own text. The whole design follows from taking that seriously -- derive the
inventory from the source at check time so a hand-typed number cannot lie, derive the
REGISTER too so the RFCs that would most benefit from claiming strong evidence are exactly
the ones that cannot, and make the skeleton writer emit only UNCLASSIFIED entries so that
mass-generating artifacts turns the gate redder rather than greener. That last one is the
transferable part: when the failure mode is social (declare instead of prove), the durable
answer is a structure in which the cheap move is also the losing move, not a policy telling
people not to make it.
