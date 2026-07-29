# Spec: rfcgate-1-extraction

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling (phases 1 to 7) + protocol (phase 8, the rfc7296 pilot) |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/rfcgate-1-extraction.md` |
| Updated | 2026-07-29 |

Umbrella: `plan/spec-rfcgate-0-umbrella.md`. This is program ONE of the set, the highest
priority child, and the first of four that the umbrella's "Sequencing Constraint" requires
to be merged strictly serially (1, then 2, then 3, then 4). It builds the machinery, plus
ONE pilot sign-off: ~~performing the 166 sign-offs is fleet work owned by the umbrella and
by `plan/spec-followup-rfc-enrollment.md`~~ (superseded 2026-07-29 by owner ruling 2, below)
performing the other 165 sign-offs is fleet work owned by the umbrella and by
`plan/spec-followup-rfc-enrollment.md`; rfc7296 is performed HERE and is not grandfathered.

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
| A-1 | The sizing is robust to extractor methodology. | Two independent derivations 2026-07-29: brief said 3940 sites / 0.69 / 60 RFCs / deficit 1646; independent re-run gave 3988 / 0.68 / 61 / 1683 | The scale of the hole is misjudged and the drain quota is set against a wrong denominator | Implementation re-derives and publishes the real numbers in the ledger; compare against both estimates | unvalidated |
| A-2 | A capitalised-keyword inventory is the WRONG oracle for a large minority of enrolled RFCs. | Measured: 53 of 166 enrolled RFCs declare more gated requirements than the capitalised-keyword site scan finds sites for. rfc2181 declares 23 from 1 site; `rfc/full/rfc2181.txt` Section 3 says the memo does not use the 2119 expressions | A single-register design ships and is vacuously green for a third of the tree | Re-derive the register split during implementation; assert the counts in a test over the live tree | unvalidated |
| A-3 | 22 enrolled RFCs have zero capitalised MUST-level keywords in their source. | Measured 2026-07-29 over `rfc/enrolled.txt`: rfc1071, rfc1332, rfc1350, rfc1877, rfc2205, rfc2328, rfc2347, rfc2348, rfc2349, rfc2918, rfc2966, rfc3101, rfc3623, rfc5701, rfc6286, rfc7534, rfc7535, rfc792, rfc8050, rfc8571, rfc905, sflow-v5. They declare 164 gated MUSTs between them | The pre-2119 fail-open is mis-sized | `test_pre2119_register_is_derived_over_live_tree` asserts the live count is non-zero and every such stem derives the prose register | unvalidated |
| A-4 | A case-insensitive modal scan gives those 22 a non-empty inventory. | Measured: 688 lowercase normative sites across the 22 (rfc905 229, rfc2328 159, rfc2205 102) | The prose register is as vacuous as the keyword one and only a manual walk remains | Re-derive during implementation; the prose-register test asserts a non-empty inventory for a fixture built from a keyword-free source | unvalidated |
| A-5 | At least one enrolled RFC has an empty inventory under BOTH registers while declaring gated MUSTs. | Measured: rfc1877 has 0 capitalised and 0 lowercase modal occurrences in 10591 chars, and declares 4 gated MUSTs | Only the live `manual-walk` COUNT is wrong; the register itself stays either way. Owner ruling 1 makes an undrainable remainder unacceptable, so an RFC with no mechanical inventory must still have a route to a sign-off, and `manual-walk` is that route. If the live set turns out empty, the ledger's `manual-walk` column reads 0 and the register remains the terminal escape AC-10 needs | Implementation enumerates the live set and records the real count in the ledger. The register is NOT dropped on a zero count (superseding the earlier "drop it and record the deviation" plan, 2026-07-29 owner ruling 1) | unvalidated |
| A-6 | Site granularity is required; section granularity is not sufficient. | Measured over the 166: 1671 hole-sites sit in wholly-unanchored sections but 558 sit in sections that already carry ids, and 618 of 1502 sections have sites and no ids. rfc4271 is 9 unanchored against 48 partial | A far cheaper section-only design would do, and the site machinery is over-built | Re-derive the unanchored/partial split during implementation and record it in the learned summary | unvalidated |
| A-7 | Published RFC source texts do not change, so `source-sha` staleness fires almost never for `rfc/full/` and occasionally for `rfc/drafts/` (7 files). | RFCs are immutable once published; `_ID_RE`'s section anchor rests on the same immutability (`rfc_requirements.py:110-113`) | Sign-offs stale in bulk and the ratchet becomes noise | Implementation records the sha; any staleness observed in the first release is a signal to revisit | unvalidated |
| A-8 | The umbrella's drain quota consumes a per-register count that this spec publishes, and credits a sign-off in ANY register. | Owner ruling 1 (2026-07-29): a `prose` or `manual-walk` sign-off counts toward the quota exactly as `rfc2119` does, provided each register is published in its own column. `plan/spec-rfcgate-0-umbrella.md` "Drain Schedule Design (D2)" owns the value of N and the cadence | Only the TRANSPORT would be wrong (the umbrella hand-parsing the rendered table instead of the JSON envelope). The counting semantics are settled and do not move | `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` and `TestExtractionLedger.test_registers_are_published_in_separate_columns`; the envelope is cheap either way | confirmed |
| A-10 | The number of obligations the rfc7296 walk newly extracts is NOT estimable before the walk runs. | 263 derived sites against 18 captured is a SITE count, and per-RFC false-positive calibration does not exist. The corpus-wide 1200-1500 estimate is calibrated across 166 RFCs and says nothing reliable about one of them | Nothing breaks: the scoping conversation with the owner simply happens earlier and against a better prior | Phase 8a produces the real count BEFORE phase 8c writes any implementation, and the count is taken to the owner (R-10) | unvalidated |
| A-11 | Neither §2.2 obligation is proven today by a test that merely lacks a tag. | `grep -c "RFC7296-2\.2-" rfc/short/rfc7296.md` returns 0, so no tag can name them. An untagged test may still exercise the behavior, which the grep cannot see | The phase-8 work for those two is a summary row plus a tag rather than an implementation, which is cheaper, not different in kind | Phase 8c reads the producing functions in `internal/component/ike/engine/msgid.go` and `sa.go` and cites `file:line` for the verdict, per `ai/rules/no-fabrication.md` | unvalidated |
| A-9 | Adding these checks inside `--check` keeps `ze-rfc-check` fast enough to stay in both verify branches. | `_git_cat_blobs:899` records the measured budget: 1.7s at HEAD, 2.2s with the batched baseline, and states that a gate which doubles verify time "is a gate people learn to skip" | The extraction check makes verify slow enough to be bypassed | Time `make ze-rfc-check` before and after; the delta budget is stated as a Deliverable | unvalidated |

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
| ~~R-14~~ | ~~The umbrella's child-1 row names `rfc/recertified.txt`; this spec ships `rfc/extraction/<stem>.json` plus a derived `--extraction-status --json`~~ | ~~The umbrella's drain check looks for a file this spec never creates~~ | **CLOSED 2026-07-29 by the umbrella, which was the right place to decide it.** The resolution: the `rfc/extraction/<stem>.json` set IS the record, and the quota is DERIVED from it through `make ze-rfc-extraction-status --json`; no per-stem ledger file is created, by this spec or any other. A second hand-kept list of who has been signed off is the rotting registry `ai/rules/derive-not-hardcode.md` forbids, and the 2026-07-20 ruling in `plan/deferrals/rfc-gate-regression-ratchets.md` already refused that artifact shape. What remains authored is POLICY only: a start date and a rate, in `rfc/drain-budget.txt`, which this spec creates and which may never name an RFC. See `plan/spec-rfcgate-0-umbrella.md` "Where the counter lives" and "What is still authored" |

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
| `make ze-rfc-extraction-status --json` | → | `run_extraction_status` | `TestExtractionStatus.test_json_envelope_carries_counts_and_registers` |
| `make ze-rfc-check` on a tree whose `rfc/drain-budget.txt` names a rate putting the floor above the signed count | → | `check_drain_floor` via `run_check` | `TestDrainFloorWiring.test_run_check_fails_when_signed_count_is_below_the_floor` |
| `make ze-rfc-check` with `rfc/drain-budget.txt` deleted | → | `check_drain_floor`'s fail-closed guard on the POLICY input, mirroring `check_enrolment:660-664` | `TestDrainFloor.test_missing_drain_budget_is_error_not_empty` |
| `make ze-rfc-index` then `make ze-rfc-check` | → | extraction table in `render_ledger`, guarded by `check_ledger_fresh:1578` | `TestExtractionLedger.test_stale_extraction_table_fails_check_fresh` |
| `make ze-rfc-index` on a tree with sign-offs in more than one register | → | per-register columns in `render_ledger` (ruling 1) | `TestExtractionLedger.test_registers_are_published_in_separate_columns` |
| `make ze-rfc-check` on the tree after the rfc7296 pilot lands | → | `check_extraction_signoff` over `rfc/extraction/rfc7296.json`, 165 stems still unsigned | `TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered` |

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
| AC-16 | `make ze-rfc-extraction-status --json` | A JSON envelope with `schema-version`, signed and enrolled counts, per-register counts, and the unsigned backlog list. Lower kebab-case keys per `ai/rules/json-format.md` |
| AC-17 | Git unavailable or `rfc/extraction/` absent at HEAD | The ratchet judges nothing and says so; it never accuses every stem of having lost a sign-off. The empty-versus-None distinction `_git_baseline_summary_stems:763-776` documents is restated for this consumer's polarity |
| AC-18 | A malformed or unreadable `rfc/extraction/<stem>.json` | Exit 2 with a clean `cannot run` message through the existing handler (`run_check:1688-1694`), never an uncaught traceback |
| AC-19 | The tree as it stands when the MACHINERY lands (166 enrolled, zero sign-offs, phases 1 to 7 complete) | `make ze-rfc-check` exits 0. Grandfathering is scope, not an allowlist file |
| AC-20 | An artifact whose derived field (`quote`, per-section site count, register) was hand-edited away from what the source re-derives | Exit 2 naming the field and the locator |
| AC-21 | `make ze-rfc-index` on a tree carrying sign-offs in more than one register | The extraction table renders one column per register (`rfc2119`, `prose`, `manual-walk`) with its own signed count. A signed total rendered WITHOUT the three component columns beside it is a failure of this AC, not a formatting preference (owner ruling 1's mandatory counterweight) |
| AC-22 | `make ze-rfc-extraction-status --json` on the same tree | The envelope carries `signed-by-register` with a key per register; the keys sum to the published signed total; and no register is excluded from that total. A consumer can compute quota credit and read evidence strength from the same document without either being inferred |
| AC-23 | `rfc/short/rfc7296.md` after phase 8 | It carries `RFC7296-2.2-1` and `RFC7296-2.2-2`, each quoting its RFC text verbatim and citing `(§2.2)`, so the id's section matches its citation as `make ze-rfc-check` already requires. At minimum these two; the walk's other findings are additional, never a substitute |
| AC-24 | The tree after phase 8 (166 enrolled, exactly one sign-off) | `make ze-rfc-check` exits 0, `rfc/extraction/rfc7296.json` validates with every site and section classified and its register derived, and the other 165 stems remain unsigned and unaccused. A non-empty artifact set changes nothing for a stem that has no artifact |
| AC-25 | Any requirement the phase-8 walk newly extracts | It carries a positive AND a negative `RFC requirement:` tagged test, OR an annotation whose authorisation by Thomas is recorded in this spec's Deviations with the date, the requirement id and his answer. An annotation present WITHOUT that record fails this AC. Nothing in this spec pre-authorises an annotation |
| AC-26 | `rfc/short/rfc7296.md` before and after phase 8 | Every one of the 23 existing ids is still present and still carries at least the polarities it held at HEAD. `check_retired_requirements` and `check_coverage_ratchet` stay green across the re-authoring, and no id is renumbered or reused |
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
| 4 | Runs the umbrella's per-release drain check | `make ze-rfc-extraction-status --json` → counts consumed by the umbrella's quota gate | `TestExtractionStatus.test_json_envelope_carries_counts_and_registers` |
| 5 | Drains a pre-2119 RFC and gets credit for it without overstating the evidence | classify under the derived `prose` or `manual-walk` register → signed total rises by one → the ledger shows which column it landed in | `TestExtractionStatus.test_every_register_counts_toward_the_signed_total` plus `TestExtractionLedger.test_registers_are_published_in_separate_columns` |
| 6 | Reads what the gate was blind to in rfc7296 and finds it discharged | `rfc/short/rfc7296.md` §2.2 rows → tagged tests via `ai/RFC-REQUIREMENTS.md` → `rfc/extraction/rfc7296.json` showing every §2.2 site mapped | `TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered` |

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
| `TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered` | same (extends `:1813`) | AC-24: the live tree is green with exactly one artifact, and the 165 unsigned stems are not accused | |
| `TestRealTree.test_rfc7296_summary_carries_the_section_2_2_requirements` | same (extends `:1813`) | AC-23: `RFC7296-2.2-1` and `RFC7296-2.2-2` exist, cite `(§2.2)`, and their text matches the source sentences at `rfc/full/rfc7296.txt:1397` and `:1439` | |
| `TestRealTree.test_rfc7296_new_requirements_are_proven_or_authorised` | same (extends `:1813`) | AC-25: every `RFC7296-*` id added since HEAD carries a positive and a negative tag, or an annotation. The test proves the mechanical half; the Deviations record proves the authorisation half, which no test can check | |
| `TestRealTree.test_rfc7296_ids_are_neither_retired_nor_demoted` | same (extends `:1813`) | AC-26, driving `check_retired_requirements` and `check_coverage_ratchet` across the re-authoring | |

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
8. **Phase: The rfc7296 pilot** (owner ruling 2). This phase is the only one that touches
   protocol content, and it is not optional: two CONFIRMED unextracted MUSTs escape the
   grandfather, and the finder is the entry point (`ai/rules/no-parking.md`).
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
| Every register earns credit | `make ze-rfc-extraction-status --json`, then confirm `signed-by-register` has a key per register and its values sum to the published total |
| rfc7296 §2.2 extracted | `grep -c "RFC7296-2\.2-" rfc/short/rfc7296.md` is at least 2, having been 0 on 2026-07-29 |
| rfc7296 §2.2 proven | `grep -rn "RFC requirement: RFC7296-2.2-" internal/` shows a positive AND a negative tag for each of the two ids, or Deviations records Thomas's authorisation for what is missing |
| Nothing was retired to reach green | `check_retired_requirements` and `check_coverage_ratchet` green across the re-authoring: all 23 pre-existing rfc7296 ids still present, none demoted |
| Backlog published | `grep -n "UNSIGNED" ai/RFC-REQUIREMENTS.md` after `make ze-rfc-index` |
| Status envelope | `make ze-rfc-extraction-status --json` output parses as JSON (pipe it into `python3 -c` with `json.load`) |
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
