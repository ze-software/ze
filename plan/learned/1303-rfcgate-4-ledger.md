# 1303 -- rfcgate-4-ledger

## Context

`docs/features/rfc-status.md` is Ze's public standards claim. It carried 157
hand-written rows and one guard. That guard, `check_status_agreement`, reached
for a row only when a `{gap}` annotation existed. A summary declaring zero
MUST-level requirements has no annotation, so both ledgers agreed by being
empty.

Five defects sat on the edges the guard did not reach.

- Four summaries claimed public support over a checklist with zero gated rows.
- 32 enrolled RFCs had no row at all, and the gate stayed green by coincidence.
- Nine summaries were un-enrolled with no recorded reason.
- A stale comment guarded a branch that suppressed nothing.
- One advisory row bought immunity from the re-author verdict.

The goal was to guard the ledger's edges with machinery instead of discipline.
Owner Ruling OR-1 added a second goal: make the four unproven claims TRUE
rather than merely self-consistent.

## Decisions

- **Row completeness is a git-HEAD ratchet, not a hard gate.** Chosen over
  writing the 32 rows now, and over a checked-in baseline list. Writing them is
  32 product judgements. A baseline file is the hand-maintained second list
  `ai/rules/derive-not-hardcode.md` forbids.
- **The un-enrolled remainder became a sibling file with a KIND.** Chosen over a
  comment in `rfc/enrolled.txt`, which `parse_enrolled` cannot see, and over a
  bare stem list, which cannot separate "the RFC imposes nothing" from "nobody
  extracted it". Three kinds: `non-normative`, `backlog`, `blocked`. Only the
  first claims anything about conformance.
- **The gap count is cross-checked, never generated.** Deriving Remaining text
  from `{gap}` reasons would publish a compliance claim built from
  classifications that `ai/rules/rfc-compliance.md:56` voided as authority. A
  count is a fact about how many annotations exist. It is never a claim that
  they are right.
- **AC-12 reads immediate adjacency only.** A tolerance window reds four honest
  rows on day one, because the page uses a second convention where a number NEAR
  MUST is the not-applicable count. Seven rows carry a number this check does not
  judge, and the gate's own message says so.
- **The unproven-support guard arms in the commit that clears the four, or
  later.** Owner Ruling OR-2 corrected the original plan. Arming first would red
  a stage both verify modes run, and `commit_helper.py` refuses a script over a
  non-FRESH verify. The red would have blocked every commit in the repository,
  including the fixing ones.
- **The four stems were never given an exemption.** Two enrolled. Two are
  declared `backlog`, which renders as DEBT and silences nothing.
- **`rfc3765` enrolled on an evidenced zero** (OR-A). It is Informational,
  invokes RFC 2119 nowhere, and calls its own mechanism advisory. A fabricated
  MUST would put a false claim inside the ledger this work exists to make honest.

## Consequences

- Every enrolled RFC must now disclose. Every summary is enrolled or declared.
  A support claim cannot rest on an empty checklist.
- The 32-row backlog is derived and rendered, so it stays visible and can only
  shrink. Its rows are owned by `plan/spec-followup-rfc-enrollment.md`, sequenced
  behind the annotation re-derivation per OR-3.
- Two public claims remain unproven, and this is the honest cost. `rfc1035` and
  `rfc5301` publish `Partial` while 34 of their gated rows carry no test.
  `check_unproven_support` cannot catch this, because it fires only on a summary
  declaring ZERO gated rows, and both now declare real ones. What changed is that
  the debt is declared, counted and owned by
  `plan/spec-fixit-dns-rfc1035-conformance.md` and
  `plan/spec-fixit-isis-hostname-ascii.md`.
- Closing this spec does not discharge that debt. A future reader who finds
  `rfc1035` unenrolled must read the two fixit specs, and must not conclude the
  extraction was abandoned.
- Enrolment froze the ids of `rfc3765` and `rfc4486`. `check_retired_requirements`
  now refuses to let either lose a requirement, so the re-anchoring window for
  those two is shut for good.

## Gotchas

- **A guard implementing an owner ruling did not check what its own docstring
  claimed.** OR-A's escape had to establish that zero gated requirements is a
  property of the DOCUMENT.

  Three of its four facts described the ARTIFACT instead:

  - the walk performed,
  - the register declared,
  - the reason recorded.

  `manual-walk` is the weakest grade in `_REGISTER_STRENGTH`, and
  `_evaluate_extraction` refuses only a claim STRONGER than derived. Any stem can
  therefore assert it. A source whose own sentences quote capitalised MUST walked
  straight through, via an artifact that excluded every site as
  `not-a-requirement`. The fix reads the DERIVED grade, which is the only fact
  taken off the text.
- **Two Status words contradicted their own Remaining cells by the page's own
  glossary.** The rows advertised support while their Remaining cells listed
  unmet obligations. Nothing caught it, because the disclosure guard fires on a
  `{gap}` annotation and these rows had none.
- **A `.ci` test pinning exact wire bytes was cited as proof but carried no
  tag.** `test/plugin/prefix-maximum-enforce.ci` pins NOTIFICATION error code 06,
  subcode 01, and the AFI/SAFI/count Data field. It was named in the enrolment
  reason as evidence. Because it carried no `RFC requirement:` tag, the gate
  credited unit evidence only. Adding the tag moved `functional/verify` from 6
  to 7. A test that proves an obligation is invisible to the ledger until it
  says which obligation.
- **A stated rejection rule was a six-phrase blacklist.** AC-14 read as a flat
  rejection of reasons that judge what Ze owes. The implementation was
  `_NON_APPLICABILITY_RE`, six named spellings. Seven rephrasings of the
  identical laundering were accepted, including "Ze is not required to do any of
  this" and "This RFC is irrelevant for our implementation". A blacklist accepts
  every wording nobody thought of. The rule is now carried by a POSITIVE
  requirement: the reason must cite an IETF category, RFC 2119 machinery, or
  a keyword scan result. It checks the citation, never its truth.
- **A declared summary had no legal exit from the tree.** Keeping the
  disposition row fired the stale-row branch. Deleting it fired the
  left-without-enrolling branch. Both exits closed at once. A summary that entered
  `rfc/short/` then had no way out. The AC-8 branch is now scoped to stems that
  still exist, which makes deleting the summary and the row together a third
  discharge.
- **Every line number this spec cited into `rfc_requirements.py` was wrong by
  the time phase 6 ran.** The module grew from 1,769 lines to 6,488 across the
  four children. The symbol name is the only durable anchor.
- **The AC-23 correction text said 34 gated rows with 33 owed.** Measured by
  folding tag polarity per requirement, it is 35 extracted, 1 proven, 34 owed.
  The load-bearing half held: zero rows are annotated, which is what
  `ai/rules/rfc-compliance.md` wants.
- **`test_the_declared_remainder_is_debt_not_a_decision` still carries OC-4's
  superseded sentence** in its docstring. That sentence still forbids closing this
  spec while either stem is declared rather than enrolled. OR-B and OR-C are the later rulings and
  they route both stems to fixit specs. The docstring was left alone because the
  review-gate artifact pins that file's hash.

## Files

- `scripts/dev/rfc_requirements.py` -- five guards, wired into `run_check`
- `scripts/dev/rfc_requirements_test.py` -- the wiring drives, and two real-tree
  assertions that no synthetic fixture can replace
- `rfc/not-enrolled.txt` -- new. The declared remainder, seven stems, three kinds
- `rfc/enrolled.txt` -- `rfc3765` and `rfc4486` added
- `rfc/short/rfc1035.md`, `rfc3765.md`, `rfc4486.md`, `rfc5301.md` -- re-authored
- `rfc/extraction/rfc1035.json`, `rfc3765.json`, `rfc4486.json`, `rfc5301.json`
- `rfc/full/rfc3765.txt`, `rfc/full/rfc4486.txt` -- fetched
- `docs/features/rfc-status.md` -- the preamble, plus four corrected rows
- `test/plugin/prefix-maximum-enforce.ci` -- gained the `RFC4486-4-1` tag
- `ai/RFC-REQUIREMENTS.md` -- three new derived tables
