# Spec: rfcgate-4 -- the public status ledger's edges

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | spec-rfcgate-1-extraction (BLOCKING, see OC-6). The umbrella also orders 2 and 3 ahead of this spec |
| Phase | - |
| Deferral shard | `plan/deferrals/rfcgate-4-ledger.md` |
| Updated | 2026-07-30 |

Part of the `rfcgate` spec set; the umbrella is `plan/spec-rfcgate-0-umbrella.md`,
which fixes the merge order 1, 2, 3, 4 and forbids two children in flight.

~~Siblings are referenced by name only -- this spec is machinery for the ledger's
edges and can land independently of the extraction programs.~~ **Corrected
2026-07-29.** It cannot. Phase 6 ENROLS four RFCs, and once
`plan/spec-rfcgate-1-extraction.md` has landed a new enrolment must carry an
extraction sign-off (its AC-1). Landing this spec first would enrol the four with
no extraction bar in existence, and child 1's grandfathering -- which is scope,
not an allowlist -- would then cover them **permanently**. See OC-6.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`docs/features/rfc-status.md` is Ze's PUBLIC standards claim: 157 hand-written
rows, five columns (RFC, Area, Status, Implemented coverage, Remaining). Nothing
generates it, and no documentation gate validates its rows. The only code that
opens it is `scripts/dev/rfc_requirements.py` (`STATUS_FILE`, `:55`), read once
in `run_check` (`:1683-1685`) to feed a single guard. `make ze-doc-test` runs
only `--check-fresh` (`run_check_fresh:1725-1741`), which never opens the file
at all, so a documentation-focused pass sees nothing about the page.

That single guard is `check_status_agreement` (`:1163-1208`): when a requirement
carries a `{gap}` annotation and its RFC is enrolled, the RFC must have a row,
and that row must not advertise clean support with an empty Remaining. It is
green today.

Five defects sit on the edges it does not reach. All five were verified against
the working tree on 2026-07-29 by driving the gate's own parser
(`parse_summary_file`, `parse_status_ledger`, `load_enrolled`,
`source_keyword_count`) over `rfc/short/`, `rfc/enrolled.txt`,
`rfc/full/` and `docs/features/rfc-status.md`.

**D1 -- a public support claim can stand over an empty checklist.** Four
summaries declare ZERO gated (MUST-level) requirements while the public page
claims support for them:

| Stem | Public Status | Declared requirements | Gated | Uppercase MUST in source | Enrolled |
|------|---------------|-----------------------|-------|--------------------------|----------|
| `rfc1035` (`:234`) | Supported | 0 (no checklist line at all) | 0 | 0 (source present, 23 lowercase `must`) | no |
| `rfc3765` (`:56`) | Supported | 4 (all advisory, ids anchored `-x-`) | 0 | no source text | no |
| `rfc4486` (`:28`) | Supported | 2 (all advisory, ids anchored `-x-`) | 0 | no source text | no |
| `rfc5301` (`:123`) | Experimental | 4 advisory + 2 bracket-tag lines | 0 | 4 | no |

`check_status_agreement` cannot see any of them: its loop (`:1174-1176`) reaches
for a row ONLY when a `{gap}` annotation exists, and a summary with no gated
requirement has no annotation. Both ledgers agree because both are empty. Ze
serves DNS authoritatively (`internal/core/dnsserver/handler.go:48`
`Authoritative`, whose file header names `rfc/short/rfc1035.md`), so the RFC 1035
"Supported" claim is a claim about live behavior resting on an empty checklist.

**Settled (OR-1, see Owner Rulings).** All four are EXTRACTED and ENROLLED by
this spec, so the public claim becomes true rather than merely self-consistent.
The last column of that table reads `no` as of 2026-07-29 and reads `yes` at
closure. The `-x-` id anchor is the skill's deliberate defect marker, not a
resting state (`ai/skills/ze-rfc.md`, Requirement IDs: "conspicuous on purpose"),
and the bracket-tag lines are the re-authoring pattern the parser's own comment
names (`scripts/dev/rfc_requirements.py:93-98`).

**D2 -- 32 enrolled RFCs have no row, and safety is luck.** 175 summaries, 166
enrolled, 157 parseable rows of which 23 key a non-enrolled stem, leaving 32
enrolled RFCs unlisted: rfc1071, rfc2003, rfc2348, rfc2349, rfc2473, rfc2782,
rfc2784, rfc2890, rfc3031, rfc3786, rfc4213, rfc4576, rfc4862, rfc5561, rfc6071,
rfc6138, rfc6397, rfc6482, rfc6549, rfc6996, rfc7012, rfc7427, rfc7440, rfc7611,
rfc792, rfc7950, rfc8097, rfc8571, rfc8707, rfc905, rfc9319, rfc9728. All 32 are
of the form `rfcNNNN`: zero drafts, and no `sflow-v5` or other non-RFC stem, so
the backlog needs no key form `parse_status_ledger` cannot already produce. None
currently carries a `{gap}`. There are 539 `{gap}` annotations across 84 RFCs
today, and all 84 happen to have rows and to be enrolled. Nothing structural
holds that: changing one `{not-applicable}` to `{gap}` in any of the 32 fires the
`row is None` branch (`:1178-1184`) and reds the build with no warning that the
row was owed all along.

**D3 -- the un-enrolled remainder is an absence, not a decision.** Nine summaries
are un-enrolled with no recorded reason: rfc1035, rfc3765, rfc4486, rfc5301,
rfc6987, rfc7999, rfc8195, rfc8326, rfc9129. All nine declare zero gated
requirements, so enrolling them today would add exactly zero obligations
(`evaluate:614-615` skips non-gated requirements). Five (rfc3765, rfc4486,
rfc7999, rfc8326, rfc9129) have no source text under `rfc/full/` or
`rfc/drafts/`, which `check_enrolment:675-684` requires before enrolment. Nothing
distinguishes "not enrolled because the RFC imposes nothing" from "not enrolled
because nobody extracted it" from "not enrolled because we cannot, the text is
missing". A `{gap}` on an un-enrolled summary is additionally exempt from
disclosure entirely (`:1176`), so these nine sit outside `check_status_agreement`.

**Settled (OR-1).** Four of the nine (rfc1035, rfc3765, rfc4486, rfc5301) are
extracted and enrolled here, so the declared remainder ends at FIVE: rfc6987,
rfc7999, rfc8195, rfc8326, rfc9129. The disposition file is still built in this
spec and still seeds nine, because the four are honest DEBT until their
extraction lands and the file is the only place that can say so (see Key Design
Decisions, "the four are declared before they are enrolled").

**D4 -- a stale comment guards a dead branch.** `run_check` (`:1670-1673`) and
`_collect_for_check` (`:1620-1625`) suppress parse errors for un-enrolled
summaries, explaining that the id migration is per-RFC and un-enrolled summaries
have not been converted. Verified: ZERO of the 175 summaries fail to parse. The
migration is complete; the suppression suppresses nothing and now only shields a
future un-enrolled summary from a parse error it should report
(`ai/rules/stale-comments.md`, `ai/rules/fail-closed-guards.md`).

**D5 (found during this design, not in the brief) -- one advisory row buys
immunity from the RE-AUTHOR verdict.** `unconverted_summaries` is called with
`captured = {r.rfc for r in requirements}` (`render_ledger:1531`) -- ANY
requirement at ANY level. A summary that captured four SHOULDs and zero MUSTs is
therefore "captured". It never appears in the "Summaries declaring no MUST-level
requirement" table, even when its source text is full of MUSTs.

`rfc5301` is exactly that: ~~`rfc/full/rfc5301.txt` carries 4 MUST-level
keywords~~ **MISLEADING, corrected 2026-07-29.** All 4 of those keyword hits are
on ONE line. That line is `rfc/full/rfc5301.txt:94`, the key-words boilerplate
paragraph of RFC 2119 ("MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT").
`grep -n "MUST\|SHALL"` returns that single line and nothing else.

Child 1's sharper measurement agrees.
`derive_inventory("rfc5301", 0).keyword_sites == 0` because `_BOILERPLATE_RE`
(`scripts/dev/rfc_requirements.py:2416`) excludes exactly that paragraph. RFC 5301
has ZERO capitalised normative sentences, not four uncaptured ones. It derives
register `prose` with 3 sites over 10 sections, so the extraction is still real
and still owed. But the "four uncaptured uppercase MUSTs" premise behind Q-3
and phase 6 is not a fact.

**The D5 bug it illustrates is unaffected.** rfc5301 is still absent from the
table because it captured 4 advisory rows. Its summary captured 4 advisory rows
and 0 gated. It is absent from the table whose own docstring (`:1340-1348`) says
"an absent summary is indistinguishable from a compliant one, which is how a
standards claim rots".

The table has 2 rows today. 7 summaries are advisory-only, and the count is
confirmed at exactly 7: rfc3765, rfc4486, rfc5301, rfc7999, rfc8195, rfc8326,
rfc9129. Separately, that table's verdict for a zero-source count reads
"consistent: source declares none", which is asserted for `rfc1035`. RFC 1035 is
a 1987 document that predates RFC 2119 and contains 0 uppercase `MUST` but 23
lowercase `must`. The heuristic reads a pre-2119 normative RFC as non-normative.

**Goal.** Close all five so the public ledger's edges are guarded by machinery
rather than by discipline: every enrolled RFC discloses, every summary is a
declared decision rather than an absence, a public support claim cannot rest on
an empty checklist, and the one fact a machine can own about the prose (the gap
count) stops being correct only by luck.

**Second goal, from OR-1.** The four D1 stems are not merely made consistent,
they are made TRUE: their obligations are extracted from the RFC text, all four
are enrolled, and their public rows are corrected. The gate that forbids a
support claim over an empty checklist arms only once that is done, never before
(see Ordering Constraints).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/fail-closed-guards.md` - every check added here is a guard
  → Constraint: a guard must fail closed or say something; a present-but-empty
    value passes `ok` and must still be rejected when empty is also wrong.
  → Decision: drive each new check from `run_check` in a gate-level test, never
    from the helper alone -- "a green unit test on an uncalled guard is worse
    than no test".
- [ ] `ai/rules/derive-not-hardcode.md` - the row-generation question
  → Decision: cross-check, do not generate. Verified that only the gap COUNT is
    machine-ownable; Area is an editorial label, not the summary H1 (`rfc1661`
    H1 is "RFC 1661 - The Point-to-Point Protocol (PPP)", its Area cell is
    "PPP LCP"), Implemented coverage is source-anchored prose, Status is a
    product judgement.
  → Constraint: the 32 missing rows must not become a second hand-maintained
    list. Derive the backlog from `enrolled - rows` and render it.
- [ ] `ai/rules/rfc-compliance.md` - what a public status row means
  → Constraint: `:56` VOIDS every `{gap}` / `{not-applicable}` / `partial` as
    authority. Anything derived from an annotation inherits that voidness, so a
    derived Remaining would launder a void classification into generated text.
  → Constraint: "Ask Thomas Whenever Full Compliance Is On The Table" (`:38-51`)
    -- a classification that lowers what Ze owes is his call, not this spec's.
    This is now the STANDING rule of the four-stem phase: every requirement
    extracted there is implemented and proven with a positive AND a negative
    tagged test, or it is escalated individually. See Owner Rulings.
  → Decision: `:28` ("The RFC requirement is not in `rfc/short/<stem>.md`") is
    the line OR-1 acts on. An unextracted obligation is still an obligation, so
    an empty checklist under a support claim is a defect to fix by extracting,
    not by downgrading the claim.
- [ ] `ai/skills/ze-rfc.md` - how the four summaries are re-authored
  → Constraint: fetch the source FIRST; enrolment requires it
    (`check_enrolment`, `scripts/dev/rfc_requirements.py:675-684`). Walk the RFC
    section by section; the coverage self-check compares captured lines against
    the source keyword count.
  → Constraint: every checklist line carries a section-anchored id. The `-x-`
    anchor carried by all six existing rfc3765 / rfc4486 rows is the skill's
    deliberate defect marker, and re-anchoring it is safe ONLY while no test tags
    the id -- verified zero tags today, a window that closes at enrolment.
  → Constraint: never tick a checkbox, never renumber or reuse an id that a test
    already references, and never annotate `{gap}` / `{not-applicable}` /
    `{single-polarity}` on your own authority.
- [ ] `ai/rules/git-safety.md` - why the arming order is a constraint, not a preference
  → Constraint: `make ze-rfc-check` is a stage in BOTH verify modes
    (`scripts/status/verify_run.go:237`, `:259`), and `commit_helper.py create`
    refuses a script over a non-FRESH verify. Arming a gate that reds the real
    tree therefore blocks every commit in the repository, including the commits
    that would fix it. See Ordering Constraints.
  → Decision: `ze-rfc-check` is NOT in `STRUCTURAL_GATES`
    (`scripts/dev/commit_helper.py:512-523`), so the red is technically
    `--unverified`-bypassable. That is not an escape: the bypass needs either the
    owner override or a `plan/known-failures/` shard, and a deterministic,
    reproducible red may not be recorded instead of fixed
    (`ai/rules/fix-dont-record.md`). The only legal route is not to arm early.
- [ ] `ai/rules/documentation.md` - source anchors
  → Constraint: any factual claim added to `docs/features/rfc-status.md` carries
    a `<!-- source: ... -->` anchor; anchors never go inside fenced blocks.
- [ ] `ai/rules/planning.md`, `ai/rules/spec-no-code.md`
  → Constraint: tables and prose only; no code snippets in this file.
- [ ] `ai/rules/deferral-tracking.md`
  → Decision: the 32 rows are scoped OUT and homed in
    `plan/deferrals/rfcgate-4-ledger.md` with `plan/spec-rfcgate-0-umbrella.md`
    as Destination and Status `deferred` (confirmed by OR-3).
  → Constraint (OR-3): the row's Reason MUST record the ORDERING dependency --
    the 32 rows follow the annotation re-derivation programme
    (`plan/spec-followup-rfc-enrollment.md`, per the umbrella's Out of Scope
    table) rather than preceding it, because the judgement that the 32 are safe
    rests on `{not-applicable}` annotations that `ai/rules/rfc-compliance.md:53`
    voided as authority on 2026-07-27. Without that sentence in the shard, a
    future reader writes the rows first and inherits the void basis.

### RFC Summaries (Scope: protocol)
Not applicable as protocol work: this spec changes no wire behavior. It does
change what the repository is allowed to CLAIM about protocol conformance, which
is why `ai/rules/rfc-compliance.md` is in the architecture list above.

**Key insights:** (minimal context to resume after compaction)
- The census is the spec. 175 summaries / 166 enrolled / 157 rows / 32 enrolled
  without a row / 23 rows keying a non-enrolled stem / 9 un-enrolled summaries,
  all nine zero-gated / 539 `{gap}` across 84 RFCs, every one of the 84 enrolled
  and rowed / 60 rows spell a MUST-gap count and all 60 agree exactly.
- `check_status_agreement` is green only because those two coincidences hold.
- The repo's established shape for "do not red the build on an existing backlog"
  is a git-HEAD baseline plus a rendered backlog table, not a checked-in
  allowlist (`check_new_summaries:1072-1080`, `unconverted_summaries`).
- OR-1 moves the census at closure to 175 summaries / 170 enrolled / 5 declared.
  The four extracted stems are rfc1035, rfc3765, rfc4486, rfc5301; the five
  declared are rfc6987, rfc7999, rfc8195, rfc8326, rfc9129.
- The single hardest-won ordering fact: the unproven-support gate arms in the
  same commit that clears the four, or later. Never earlier. A gate armed over an
  unresolved defect blocks the commit that fixes the defect.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/rfc_requirements.py:55` - `STATUS_FILE` constant; the only
  binding between the gate and the public page.
- [ ] `scripts/dev/rfc_requirements.py:1128` - `parse_status_ledger`: folds
  markdown rows into `{stem: {status, coverage, remaining}}`, keying an
  `RFC <n>` first cell, a `draft-*` stem, or a lowercase hyphenated stem, and
  silently skipping anything else.
- [ ] `scripts/dev/rfc_requirements.py:1163` - `check_status_agreement`: the sole
  cross-ledger guard; `:1176` exempts a non-enrolled stem outright, `:1178`
  fails on a missing row, `:1188-1200` decides whether the row discloses.
- [ ] `scripts/dev/rfc_requirements.py:655` - `check_enrolment`: fails on an
  empty enrolment, on un-enrolment, on an enrolled stem with no summary, and on
  an enrolled stem with no source text. It never looks at `STATUS_FILE`.
- [ ] `scripts/dev/rfc_requirements.py:688` - `parse_enrolled`: skips blanks and
  `#` comments, takes `line.split()[0]`. Reusable verbatim for a sibling file.
- [ ] `scripts/dev/rfc_requirements.py:698` - `_git_baseline_enrolment`: the
  `git show HEAD:<path>` baseline-reader pattern every ratchet uses; returns an
  empty set on failure so a degraded baseline judges nothing.
- [ ] `scripts/dev/rfc_requirements.py:1059` - `check_new_summaries`: the
  precedent for grandfathering a pre-HEAD backlog (`:1083-1084` returns early on
  an absent baseline) and for the "source has MUSTs but summary captured none"
  comparison (`:1113-1121`).
- [ ] `scripts/dev/rfc_requirements.py:93-98` - the `_FIRST_TAG_RE` comment: ad-hoc
  category tags (`[FORMAT]`, `[IPSEC]`, `[TRANSPORT]`, `[LSA]`) list implementation
  TASKS, not RFC 2119 obligations. The comment names them as "why those summaries
  capture zero MUSTs and need re-authoring" -- the exact pattern OR-1 acts on.
- [ ] `scripts/dev/rfc_requirements.py:331-334` - `parse_checklist_line`: a bracket
  tag with no 2119 keyword is "not a requirement, not an error", so it is silently
  prose. `rfc/short/rfc5301.md` has 2 such lines and `rfc/short/rfc6987.md` has 5.
- [ ] `scripts/dev/rfc_requirements.py:675-684` - the enrolment source-text
  requirement: an enrolled stem with no `rfc/full/<stem>.txt` or
  `rfc/drafts/<stem>.txt` is an error, "without it the summary is validated only
  against itself". Verified present for rfc1035, rfc5301, rfc6987 and rfc8195;
  ABSENT for rfc3765, rfc4486, rfc7999, rfc8326 and rfc9129.
- [ ] `scripts/dev/commit_helper.py:512-523` - `STRUCTURAL_GATES`: eight names,
  and `ze-rfc-check` is NOT among them. This is why the arming-order constraint is
  argued from the verify-status gate plus `ai/rules/fix-dont-record.md`, not from
  the structural-red rule.
- [ ] `scripts/dev/rfc_requirements.py:1323` - `source_keyword_count`: counts
  UPPERCASE `MUST|MUST NOT|SHALL|SHALL NOT` in `rfc/full/<stem>.txt` or
  `rfc/drafts/<stem>.txt`; `None` when neither exists.
- [ ] `scripts/dev/rfc_requirements.py:1340` - `unconverted_summaries`: the
  RE-AUTHOR table; takes `captured` as a parameter.
- [ ] `scripts/dev/rfc_requirements.py:1465` - `render_ledger`: emits the rollup,
  the per-RFC requirement tables, and (`:1531`) the unconverted table with
  `captured = {r.rfc for r in requirements}`.
- [ ] `scripts/dev/rfc_requirements.py:1601` - `_collect_for_check`: parses every
  summary tolerantly, returns reported parse errors (`:1624`, enrolled only) plus
  the per-stem map of all failures.
- [ ] `scripts/dev/rfc_requirements.py:1629` - `run_check`: the check ordering.
  `check_enrolment` at `:1637`; `STATUS_FILE` opened at `:1683`, AFTER it.
- [ ] `scripts/dev/rfc_requirements.py:1725` - `run_check_fresh`: what
  `ze-doc-test` calls; touches only `check_ledger_fresh`.
- [ ] `scripts/dev/rfc_requirements_test.py:33` - `_patched`, and `:51`
  `_run_capturing`: the gate-level wiring-test harness every wiring class uses.
- [ ] `scripts/dev/rfc_requirements_test.py:750` - `TestStatusLedgerCrossCheck`,
  `:1754` `TestStatusDisclosureFailsClosed`: the existing status-ledger fixtures
  the new ones sit beside.
- [ ] `internal/core/dnsserver/handler.go:48` - `Authoritative`, the producer
  behind RFC 1035's "Supported" row. Its header cites `rfc/short/rfc1035.md`.
- [ ] `rfc/short/rfc3765.md:70-73`, `rfc/short/rfc4486.md:55-56`,
  `rfc/short/rfc5301.md:124-129` - the existing checklist lines of three of the
  four stems. rfc3765 and rfc4486 carry SIX rows in total, every one with an
  `-x-` id anchor (no section reference); rfc5301 carries four section-anchored
  advisory rows plus two `[FORMAT]` bracket lines. `rfc/short/rfc1035.md` carries
  no checklist line at all.
- [ ] Verified 2026-07-29 by grep over the tree: ZERO `RFC requirement:` tags
  reference an `RFC1035-`, `RFC3765-`, `RFC4486-` or `RFC5301-` id, so the id
  re-anchoring OR-1 requires is safe today and stops being safe at enrolment.
- [ ] `Makefile:437` - `ze-rfc-check` runs `--selftest` then `--check`.
- [ ] `mk/inventory.mk:106` - `ze-doc-test` runs `--check-fresh` only.
- [ ] `scripts/status/verify_run.go:237` - `ze-rfc-check` is a stage in both
  verify modes, so every new check lands in the pre-commit gate.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Exit codes: 0 = a comparison ran and found nothing; 2 = violations, or the
  gate could not run. `--check-fresh` keeps exit 1 for staleness.
- Every existing check keeps its current signature and its current tests. No
  existing fixture is rewritten to accommodate a new check
  (`ai/rules/no-test-deletion.md`, Test Rewrite as Replacement).
- `parse_status_ledger`'s three key forms (RFC number, `draft-*`, lowercase
  hyphenated stem) and its silent skip of a non-matching first cell.
- A degraded baseline (git cannot answer, or an empty baseline) judges NOTHING,
  in every new ratchet exactly as in the existing three.
- `ai/RFC-REQUIREMENTS.md` stays byte-stable for identical inputs; the freshness
  gate (`check_ledger_fresh`) depends on it.
- Enrolment stays monotonic. Nothing here un-enrols an RFC.
- The public page keeps its editorial voice: Status, Area and Implemented
  coverage remain hand-authored.

**Behavior to change:**
- A newly enrolled RFC must bring a status row; a row must not vanish while its
  RFC stays enrolled.
- Every summary must be either enrolled or carry a declared disposition.
- A public support claim over a stem with zero gated requirements fails.
- A `{gap}` on a non-enrolled stem that HAS a public row must disclose.
- Every parse error is reported, enrolled or not.
- The RE-AUTHOR table sees advisory-only summaries, and stops asserting
  non-normativity from a zero uppercase-keyword count.
- A spelled MUST-gap count in a Remaining cell must equal the real count.
- (OR-1) `rfc/short/rfc1035.md`, `rfc/short/rfc3765.md`, `rfc/short/rfc4486.md`
  and `rfc/short/rfc5301.md` declare their real obligations, every line
  section-anchored, and all four are enrolled. `rfc/full/rfc3765.txt` and
  `rfc/full/rfc4486.txt` exist. Their four public rows are corrected.
- (OR-4) `docs/features/rfc-status.md` gains a preamble paragraph stating which
  of the page's properties are machine-checked and which stay editorial.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `make ze-rfc-check` -> `rfc_requirements.py --check` -> `run_check`. Stage in
  both `ze-verify` and `ze-verify-changed` (`scripts/status/verify_run.go:237`,
  `:259`).
- Inputs at entry: `rfc/short/*.md` (markdown checklists), `rfc/enrolled.txt`
  (`<stem>\t<reason>` lines), `rfc/not-enrolled.txt` (NEW, same shape plus a
  kind), `docs/features/rfc-status.md` (markdown table), `rfc/full/*.txt` and
  `rfc/drafts/*.txt` (RFC source text), the working tree's tagged tests, and the
  same four `git show HEAD:` blobs the existing ratchets read.
- Two of those source inputs are CREATED by this spec: `rfc/full/rfc3765.txt` and
  `rfc/full/rfc4486.txt`, fetched from `https://www.rfc-editor.org/rfc/<stem>.txt`
  because `check_enrolment` (`:675-684`) refuses to enrol a stem without one.
  The fetch is a one-time authoring input, not a runtime dependency: the gate
  reads the committed file and never reaches the network.

### Transformation Path
1. `_collect_for_check` parses every summary into `Requirement` values, scans the
   tree for `RFC requirement:` tags, and returns parse errors (change: no longer
   filtered by enrolment) plus the per-stem failure map.
2. `load_enrolled` and a new sibling loader read `rfc/enrolled.txt` and
   `rfc/not-enrolled.txt` into a stem set and a `{stem: (kind, reason)}` map.
3. `run_check` reads `docs/features/rfc-status.md` ONCE, early (moved up from
   `:1683`), into `parse_status_ledger` rows, and reads the HEAD copy of the same
   file through a new `git show` baseline reader.
4. The existing checks run unchanged, then the four new ones fold over
   (requirements x rows x enrolment x dispositions x baselines) producing error
   strings.
5. `render_ledger` gains two derived backlog tables (enrolled-without-row, and
   declared-not-enrolled) and a corrected RE-AUTHOR table.
6. Errors are printed and the process exits 2; an empty error list prints the
   green summary line and exits 0.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Gate <-> public docs | `parse_status_ledger` over `docs/features/rfc-status.md` markdown rows | No |
| Gate <-> enrolment declaration | `parse_enrolled` over `rfc/enrolled.txt` and `rfc/not-enrolled.txt` | No |
| Gate <-> git history | `git show HEAD:<path>` for enrolment, summaries, ids, tags, and (NEW) status rows | No |
| Gate <-> RFC source text | `source_keyword_count` over `rfc/full/` and `rfc/drafts/` | No |
| Gate <-> generated ledger | `render_ledger` -> `ai/RFC-REQUIREMENTS.md`, compared byte-for-byte by `check_ledger_fresh` | No |
| Verify <-> gate | `scripts/status/verify_run.go` stage list, exit code only | No |

### Integration Points
- `run_check` (`:1629`) - the single call site every new check is wired into; a
  check not called there does not exist.
- `render_ledger` (`:1465`) - the only place the backlog becomes visible; adding
  a table changes the ledger bytes, so `make ze-rfc-index` must run in the same
  commit or `check_ledger_fresh` reds.
- `rfc/enrolled.txt` header comments - already document the enrolment contract
  and must point at the new sibling file.
- ~~`ai/rules/rfc-compliance.md` "the four ratchets" table - becomes six.~~
  **FALSE as of 2026-07-29 (freshness re-verification).** The heading is already
  `## What Keeps RFC Testing Valid (the six ratchets)`
  (`ai/rules/rfc-compliance.md:115`): child 1 took it to five
  (`check_extraction_ratchet`) and child 2 to six (`check_evidence_ratchet`).
  This spec adds ONE HEAD-baseline ratchet (status-row completeness), so the
  edit is six -> SEVEN. `check_summary_disposition`, `check_unproven_support`
  and the gap-count cross-check are not HEAD ratchets and do not belong in that
  table.
- ~~`ai/INDEX.md:212` - already stale ("the three ratchets"). Corrected here.~~
  **FALSE as of 2026-07-29.** `ai/INDEX.md:212` reads "the five ratchets" (child 1
  corrected it, and `ai/INDEX.md` is unmodified vs HEAD). It is stale by ONE, not
  by three, and the string "the three ratchets" does not exist in the file.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Every new check is called from `run_check` and proven by a `_patched` + `_run_capturing` wiring test, mirroring `TestCoverageRatchetWiring` |
| No unintended coupling (components stay isolated) | No | The gate keeps its one-way read of `docs/`; nothing writes the public page |
| No duplicated functionality (extends existing, does not recreate) | No | Reuses `parse_enrolled`, the `git show` baseline pattern, and `source_keyword_count` rather than adding parallel readers |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A: Python tooling, no wire path |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | The 32-row backlog is DERIVED (`enrolled - rows`) and rendered, never listed in a checked-in allowlist; the disposition file declares decisions, not exceptions |

## Ordering Constraints (BLOCKING)

The owner corrected this spec's original sequencing on 2026-07-29 (OR-2). The
principle it encoded is retained in full; only the order changed. These are
numbered because they constrain WHICH COMMIT each piece may land in, which no
acceptance criterion about behavior can express.

| # | Constraint | Reason |
|---|-----------|--------|
| OC-1 | `check_unproven_support` may be CALLED from `run_check` with an inert stub body at any time, but may not carry its real body in any commit that lands BEFORE the commit resolving all four D1 stems. Same commit is the earliest legal arming point; later is also legal | An armed gate reds `make ze-rfc-check` on the real tree. That stage runs in both verify modes (`scripts/status/verify_run.go:237`, `:259`), and `commit_helper.py create` refuses to prepare a script over a non-FRESH verify (`ai/rules/git-safety.md`, Step 1). Arming first would block every commit in the repository, INCLUDING the commits that fix the four |
| OC-2 | The bypass that technically exists must not be planned for | `ze-rfc-check` is absent from `STRUCTURAL_GATES` (`scripts/dev/commit_helper.py:512-523`), so the red is `--unverified`-bypassable in principle. In practice the bypass needs the owner override or a `plan/known-failures/` shard, and a deterministic reproducible red may never be recorded instead of fixed (`ai/rules/fix-dont-record.md`). A plan that depends on it is a plan to break a rule |
| OC-3 | The four stems NEVER receive a baseline, allowlist, exemption or grandfather clause. Not a HEAD baseline, not a checked-in list, not a `non-normative` disposition | This is the retained principle. The 32 rowless enrolments are grandfathered because every one has its requirements classified, so no unmet MUST hides behind them. The four are the opposite: a public support claim with nothing underneath. Grandfathering the sharpest defect would ship the gate and keep the bug (`ai/rules/no-parking.md`). They are FIXED, not excused |
| OC-4 | The `backlog` disposition the four carry between phase 2 and phase 6 is not an exemption under OC-3 | A `backlog` kind renders as DEBT in the ledger and asserts nothing about conformance; it is the honest statement that extraction is owed. It also cannot silence the gate, because the gate is not armed yet. The spec MAY NOT close while any of the four is still declared rather than enrolled |
| OC-5 | Every intermediate commit leaves `make ze-rfc-check` green | The observable form of OC-1 and OC-4. If a phase cannot land green, it is not finished, and the answer is never to arm less or to record the red |
| OC-6 | This spec lands AFTER `plan/spec-rfcgate-1-extraction.md`, and each of the four D1 enrolments in phase 6 carries a valid `rfc/extraction/<stem>.json` sign-off. Four enrolments, four sign-offs | Child 1 adds a precondition to `check_enrolment`: a stem enrolled since HEAD must carry an extraction sign-off (`plan/spec-rfcgate-1-extraction.md` AC-1), and its grandfathering is SCOPE rather than an allowlist, so the bar reaches only what enrols after the bar exists. If this spec landed FIRST, the four would enrol with no bar, be enrolled at HEAD when child 1 arrives, and be grandfathered out of the extraction bar **permanently** -- the four stems whose unproven public claims motivated the whole programme escaping its central check. The umbrella names this as a load-bearing ordering reason in its own right ("Why child 4 must follow child 1"), independent of the module-conflict and ratchet-baseline reasons. Two of the four (rfc3765, rfc4486) have no source text, so phase 6's FETCH step is a precondition of the sign-off and not merely of the summary: with no source there is no inventory to derive and no register to sign under |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Exactly 32 enrolled stems lack a status row, and none carries a `{gap}` today | Probe over `load_enrolled` / `parse_status_ledger` / `parse_summary_file`, 2026-07-29 | The completeness ratchet reds on landing instead of grandfathering | Re-run the census probe at implementation start; the count appears in the rendered backlog table | unvalidated |
| A-2 | All 539 `{gap}` annotations belong to enrolled, rowed RFCs | Same probe: 84 gap RFCs, 0 missing a row, 0 un-enrolled | Narrowing the `:1176` exemption is not a no-op and reds on landing | `TestGapDisclosureCoversRowedUnenrolled` plus a full `make ze-rfc-check` | unvalidated |
| A-3 | Exactly 4 stems carry a public support claim with zero gated requirements | Same probe | The OR-1 phase has a fifth stem in it, and its scope grew without a ruling | Re-run the probe at the start of phase 6; the gate's own error list enumerates them, and any stem beyond the named four routes to `plan/spec-rfcgate-1-extraction.md` | unvalidated |
| A-4 | All 60 rows spelling a MUST-gap count agree with the real `{gap}` count | Probe with a spelled-number parser tolerant of `twenty-five` (a naive parser produces a false mismatch on `rfc1661`) | The count cross-check reds on landing and must be scoped to a ratchet | `TestGapCountAgreementRealFile` over the committed page | unvalidated |
| A-5 | Zero of the 175 summaries fail to parse | Probe: `parse_summary_file` over every stem, 0 `ParseError` | Removing the enrolment filter on parse errors reds on landing | `TestUnenrolledParseErrorIsReported` plus a full run | unvalidated |
| A-6 | `parse_enrolled` can read the disposition file unchanged (comments skipped, first token taken) | `scripts/dev/rfc_requirements.py:688-695` | A second parser is needed, and the kind column needs its own grammar | `TestDispositionFileParsesLikeEnrolled` | unvalidated |
| A-7 | `git show HEAD:docs/features/rfc-status.md` is available wherever the other three baselines are | `_git_baseline_enrolment:698-711` uses the same call and tolerates failure | The row-deletion half of the completeness ratchet never fires; it must stay quiet, not guess | `TestStatusBaselineSurvivesGitFailure` | unvalidated |
| A-8 | Adding tables to `render_ledger` does not destabilize `check_ledger_fresh` | `:1510-1515` already sorts for byte-stability | The ledger churns per machine and the freshness gate becomes noise | `TestLedgerRenderStableWithBacklogTables` (render twice, compare) | unvalidated |
| A-9 | `https://www.rfc-editor.org/rfc/rfc3765.txt` and `.../rfc4486.txt` are fetchable and are the authoritative texts | `ai/skills/ze-rfc.md` step 2 and `ai/rules/rfc-compliance.md:83` name that URL form; the other 170 sources under `rfc/full/` came the same way | Neither RFC can be enrolled at all (`check_enrolment:675-684`), and OR-1 cannot be executed as written | The fetch itself, in phase 6, before a line of summary is written | unvalidated |
| A-10 | RFC 1035's obligation tail is not knowable before the section-by-section walk | It is the DNS specification: 55 pages, 0 uppercase MUST, 23 lowercase `must`, and pre-RFC-2119 normative prose has no keyword to count | The phase's size is unknown at planning time, which is recorded rather than guessed | The walk. The count is reported to the owner when known, and the tail is scoped WITH him at that point (OR-1) | unvalidated |
| A-11 | Re-anchoring the six `-x-` ids in rfc3765 / rfc4486 breaks no test tag | Grep over the tree, 2026-07-29: zero `RFC requirement:` tags name an `RFC1035-`, `RFC3765-`, `RFC4486-` or `RFC5301-` id | A re-anchor silently re-points a tag at a different obligation, the exact failure `ai/skills/ze-rfc.md` forbids ("Never renumber. Never reuse.") | Re-run the grep at the START of phase 6; the window closes the moment a tagged test lands | unvalidated |
| A-12 | Enrolling four stems does not red an existing ratchet | `check_coverage_ratchet` compares per-requirement polarity against HEAD and a NEW requirement has no HEAD polarity; `check_retired_requirements` covers ids of ENROLLED RFCs only, and these are un-enrolled at HEAD; `check_new_summaries` fires on summaries NEW since HEAD, and all four exist at HEAD | Phase 6 reds on a ratchet nobody planned for, and the fix is not obvious from the error | A full `make ze-rfc-check` immediately after the first stem is enrolled, before the other three | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A row is written only to satisfy the gate, with a vacuous Remaining ("No tracked gap in current source anchors.") | A new row lands in the same commit as a new enrolment, with Remaining text copied verbatim from another row | The completeness gate checks PRESENCE only; disclosure quality stays with `check_status_agreement`, which already rejects an empty Remaining under `Supported`. Record in the review checklist that a duplicated boilerplate Remaining is a review BLOCKER, not a gate concern -- a gate that judged prose quality would be gamed faster than it was written |
| R-2 | A derived Remaining is only as honest as the annotations, and `ai/rules/rfc-compliance.md:56` VOIDS every annotation as authority | Any proposal to generate Remaining text from `{gap}` reasons | Do not derive Remaining. Cross-check the COUNT only: a count is a fact about how many annotations exist, never a claim that their classifications are right. This is the load-bearing reason D6 is a cross-check and not a generator |
| R-3 | A requirement mis-annotated `{not-applicable}` vanishes from BOTH ledgers at once -- strictly worse than today's two-ledger disagreement surface | A `{gap}` becomes `{not-applicable}` in the same commit that removes gap text from a row | Out of scope to fix here and NOT silently accepted: the disposition and re-audit programs own it. Recorded in Known Limitations, and it was put to Thomas as Q-4. His answer (OR-3) sequences the 32 rows BEHIND the annotation re-derivation for exactly this reason, so the risk is now owned by `plan/spec-followup-rfc-enrollment.md` rather than absorbed here |
| R-4 | The disposition file becomes a laundering surface -- "not enrolled" reading as "decided, settled" for a summary that is really unextracted debt | A disposition reason that reads like a conformance judgement ("not applicable to ze") | The file carries a KIND (`non-normative` / `backlog` / `blocked`), the gate rejects an unknown kind, and `backlog` / `blocked` render as DEBT in the ledger, never as a decision. AC-14 forbids a reason asserting non-applicability |
| R-5 | The 32-row backlog, having stopped reddening anything, is never written | The rendered backlog table's count does not move across releases | The deferral row names `plan/spec-rfcgate-0-umbrella.md` as Destination with Status `deferred`, and the count is rendered on every `make ze-rfc-index` so it is visible in review |
| ~~R-6~~ | ~~The D3 gate reds the build on four stems this spec cannot honestly clear alone~~ | ~~`make ze-rfc-check` fails on landing with four `unproven support` errors~~ | ~~This is the intended pressure, not a defect. The spec stays OPEN until the four are cleared~~ SUPERSEDED by OR-2, 2026-07-29. The risk was real but the mitigation was unusable: an armed gate reds a stage both verify modes run, and `commit_helper.py` refuses a script over a non-FRESH verify, so the pressure would have fallen on every commit in the repository including the fixing ones. Replaced by R-6a and the Ordering Constraints |
| R-6a | The gate is armed early by accident: a phase-1 stub is given its real body while the four are still unresolved | `make ze-rfc-check` reds on the real tree with `unproven support` errors naming the A-3 stems | OC-1 makes the arming point a commit-level constraint, OC-5 makes "green at every intermediate commit" the observable test, and phase 7 is the ONLY phase permitted to give the check a body. A red here is not logged, not `--unverified`-ed, and not baselined: it means a commit was assembled out of order and the body belongs back in phase 7 |
| R-7 | `source_keyword_count` is uppercase-only, so a pre-2119 RFC reads as non-normative and could be used to justify a `non-normative` disposition | A `non-normative` disposition for a pre-1997 RFC | The corrected verdict text names the uncertainty, and a lowercase counter is reported alongside for evidence. AC-13 requires the pre-2119 caveat in the rendered verdict. RFC 1035 is the worked example and OR-1 settles it the other way: 0 uppercase and 23 lowercase `must` means EXTRACT, never `non-normative` |
| R-8 | The four-stem phase is extraction work inside the ledger child, which the umbrella forbids (`plan/spec-rfcgate-0-umbrella.md` D1 "No child may bundle an extraction change with an evidence, audit, or ledger change", D4 "no child may open the backlog", and its Out of Scope row "Changing any `rfc/short/*.md` requirement text or id -- Out of the set entirely") | A reviewer reads phase 6 as a child violating its umbrella, or the phase grows into the general backlog | OR-1 is the later ruling and it supersedes those constraints FOR THESE FOUR STEMS ONLY. The exception is bounded by name: rfc1035, rfc3765, rfc4486, rfc5301. No fifth stem is extracted here, no `{not-applicable}` anywhere in the tree is re-derived here, and the 32 rowless enrolments stay deferred. Any pull toward a fifth stem is scope creep and routes to `plan/spec-rfcgate-1-extraction.md` |
| R-9 | Re-authoring rewrites requirement ids, and an id is a permanent contract | A phase-6 diff renumbers or drops an id | Verified zero test tags on all four stems today (A-11), which is what makes re-anchoring the six `-x-` ids legal AT ALL. Re-run the grep at phase start. After enrolment the ids are frozen by `check_retired_requirements`, so the correction happens now or never |
| R-10 | An extracted MUST turns out to be unimplemented or unprovable, and the phase stalls | A requirement with no producing code, or one where only one polarity can be asserted | This is the EXPECTED outcome for some rows, not a failure. Each one is escalated individually per `ai/rules/rfc-compliance.md:38-51`, quoting the requirement id, the RFC section text verbatim and the producing code `file:line`. The spec stays OPEN pending the answer. Writing `{gap}`, `{not-applicable}` or `{single-polarity}` without that answer is banned (`ai/skills/ze-rfc.md`, "Writing any of the three is Thomas's call, not yours") |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and no wire behavior. A wrong check reds `make ze-verify` for every developer (both verify modes run `ze-rfc-check`), or -- the failure that matters -- passes green while the public compliance page keeps a false claim |
| How is it reverted? | Single commit revert. The new checks are additive functions plus their call sites; `rfc/not-enrolled.txt` is a new file; the ledger is regenerated by `make ze-rfc-index` |
| Who else touches this path? | The sibling `rfcgate` specs under `plan/spec-rfcgate-0-umbrella.md`, and any extraction work editing `rfc/short/*.md`. Concurrent edits collide on `ai/RFC-REQUIREMENTS.md` (generated) and on `rfc/enrolled.txt` |
| What does the OR-1 phase touch that the machinery does not? | Requirement TEXT and IDS in four `rfc/short/*.md` files, four rows of the public page, `rfc/enrolled.txt`, and two new `rfc/full/*.txt` sources. Still no daemon code, no wire behavior. The failure mode is different in kind from the rest of the spec: a wrong extraction publishes an obligation the RFC does not contain, or leaves out one it does, and the gate cannot see either (`ai/rules/rfc-compliance.md`, Extraction Completeness). The mitigation is the section-by-section walk against the fetched source plus the coverage self-check, not a test |
| How is the OR-1 phase reverted? | Not by revert. An enrolled stem may not be un-enrolled (enrolment is monotonic, `check_enrolment`), so a mistake in the extraction is corrected forward by editing the requirement TEXT under the same id |

## Wiring Test (MANDATORY -- NOT deferrable)

Every row drives `run_check` end to end. The first eight go through `_patched` +
`_run_capturing` over synthetic inputs, the harness `TestCoverageRatchetWiring`
(`:1337`) and `TestRetiredRequirementsWiring` (`:1562`) already use. A
helper-only test would prove the helper and nothing about whether the gate calls
it (`ai/rules/fail-closed-guards.md`, Test corollary).

The last two rows are deliberately NOT patched: they assert over the REAL
`rfc/` tree, because what they prove is a fact about this repository (the four
stems are enrolled and sourced, and the declared remainder is five) rather than
about a code path. A synthetic fixture would pass with the four still broken,
which is the whole failure OR-1 exists to end. `TestGapCountAgreementRealFile`
is the existing precedent for a real-file assertion in this suite.

For the MACHINERY, `.ci` functional tests are N/A: it adds no daemon code, so its
driving surface is `scripts/dev/rfc_requirements_test.py`. Phase 6's tagged tests
are a separate matter and prefer a `.ci` binding where one is reachable (see
Functional Tests).

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rfc-check` (`run_check`) | → | status-row completeness ratchet | `TestStatusCompletenessWiring.test_run_check_fails_when_a_new_enrolment_has_no_row` |
| `make ze-rfc-check` (`run_check`) | → | status-row deletion half of the same ratchet | `TestStatusCompletenessWiring.test_run_check_fails_when_a_row_is_deleted_under_enrolment` |
| `make ze-rfc-check` (`run_check`) | → | summary disposition completeness | `TestSummaryDispositionWiring.test_run_check_fails_on_a_summary_that_is_neither_enrolled_nor_declared` |
| `make ze-rfc-check` (`run_check`) | → | unproven-support guard | `TestUnprovenSupportWiring.test_run_check_fails_on_support_claim_over_zero_gated_requirements` |
| `make ze-rfc-check` (`run_check`) | → | gap-count cross-check | `TestGapCountWiring.test_run_check_fails_when_a_spelled_count_disagrees` |
| `make ze-rfc-check` (`run_check`) | → | narrowed `{gap}` disclosure exemption | `TestGapDisclosureWiring.test_run_check_fails_on_unenrolled_gap_with_a_rowed_claim` |
| `make ze-rfc-check` (`run_check`) | → | parse errors no longer filtered by enrolment | `TestParseErrorReportingWiring.test_run_check_reports_an_unenrolled_parse_error` |
| `make ze-rfc-index` (`run_write` → `render_ledger`) | → | derived backlog tables | `TestLedgerBacklogTables.test_ledger_renders_enrolled_without_row_and_dispositions` |
| `make ze-rfc-check` (`run_check` → `check_enrolment`) | → | the four stems enrolled with source text present | `TestFourStemEnrolmentRealTree.test_all_four_are_enrolled_and_sourced` |
| `make ze-rfc-check` (`run_check` → `check_summary_disposition`) | → | the declared remainder shrinks to five by enrolment, not by deletion | `TestFourStemEnrolmentRealTree.test_declared_remainder_is_exactly_five` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A stem is enrolled in the working tree and was not enrolled at HEAD, and `docs/features/rfc-status.md` has no row for it | `make ze-rfc-check` exits 2 naming the stem and saying the public ledger must disclose the newly gated RFC |
| AC-2 | A stem is enrolled at HEAD and now, had a row at HEAD, and the row is absent in the working tree | The gate exits 2 naming the stem and the deleted row |
| AC-3 | The 32 stems enrolled at HEAD without a row at HEAD are unchanged | The gate does NOT fail on them; they are grandfathered exactly as `check_new_summaries` grandfathers pre-HEAD summaries |
| AC-4 | `git show HEAD:docs/features/rfc-status.md` fails or returns nothing | The completeness ratchet reports nothing and the gate does not fail on it; the run is not silently treated as clean for the other checks |
| AC-5 | A summary exists in `rfc/short/` that is in neither `rfc/enrolled.txt` nor `rfc/not-enrolled.txt` | The gate exits 2 naming the stem and both files |
| AC-6 | A stem appears in BOTH `rfc/enrolled.txt` and `rfc/not-enrolled.txt` | The gate exits 2; the contradiction is rejected rather than resolved by precedence |
| AC-7 | `rfc/not-enrolled.txt` names a stem with no `rfc/short/<stem>.md` | The gate exits 2 naming the stale disposition row |
| AC-8 | A stem leaves `rfc/not-enrolled.txt` without entering `rfc/enrolled.txt` | The gate exits 2; a disposition may only be discharged by enrolment |
| AC-9 | A disposition line carries a kind outside `non-normative` / `backlog` / `blocked`, or carries no reason | The gate exits 2 naming the line and the accepted kinds |
| AC-10 | A stem has a `docs/features/rfc-status.md` row whose Status is a support claim (anything other than `Unsupported` or `Future`), declares zero gated requirements, and has no `non-normative` disposition | The gate exits 2 saying the public claim rests on an empty checklist |
| AC-11 | The same stem carries a `non-normative` disposition | The gate passes for that stem |
| AC-12 | ~~A Remaining cell spells a MUST-gap count (`One`..`Nineteen`, `Twenty`..`Twenty-nine`, `Thirty`..) followed by `MUST` or `SHALL`~~ **NARROWED 2026-07-29, see AC-12 correction below: a spelled number IMMEDIATELY followed by `MUST` or `SHALL`** | The spelled number equals the count of `{gap}` annotations in that stem's summary, or the gate exits 2 naming both numbers. `Twenty-five` parses as 25, never as 5 |
| AC-13 | A summary declares zero GATED requirements and declares advisory ones | It appears in the ledger's "Summaries declaring no MUST-level requirement" table. When its source keyword count is zero, the verdict names the pre-RFC-2119 uncertainty rather than asserting "consistent: source declares none". It also reports the lowercase `must` count as evidence |
| AC-14 | A `non-normative` disposition reason asserts non-applicability to Ze rather than a property of the RFC text | ~~A `non-normative` disposition reason asserts non-applicability to Ze rather than a property of the RFC text~~. **REFINED 2026-07-30 to the two-part rule actually implemented.** A `non-normative` reason must (a) avoid the Ze-owes-nothing phrasings AND (b) positively CITE a property of the document. The citation names an IETF category, the key-words machinery of RFC 2119 / RFC 8174 / BCP 14, or a capitalised-keyword scan result | The gate exits 2 on either half. The original wording described the rejection as flat, which an independent review showed was a SIX-PHRASE BLACKLIST: seven evasions were accepted, including "Ze is not required to do any of this" and "This RFC is irrelevant for our implementation". A blacklist accepts every wording nobody thought of, so the positive citation requirement (`non_normative_reason_cites_the_document`, `scripts/dev/rfc_requirements.py:2094`, wired at `:2189`) is the half that guarantees anything. **What the gate does NOT do, stated so the prose does not overclaim again:** it checks the CITATION, never its truth. Nothing opens `rfc/full/<stem>.txt` to confirm the category or re-run the scan. It converts an unfalsifiable assertion into a checkable one and names who checks it 
| AC-15 | A requirement carries `{gap}`, its stem is NOT enrolled, and its stem HAS a status row | The row must disclose (non-`Supported` status, or a non-empty Remaining that is not a no-gap phrase) or the gate exits 2 |
| AC-16 | A requirement carries `{gap}`, its stem is NOT enrolled, and its stem has NO status row | The gate does not fail: an un-rowed, un-enrolled RFC makes no public claim to contradict |
| AC-17 | An UN-ENROLLED summary fails to parse | The parse error is reported and the gate exits 2; enrolment no longer filters parse-error reporting, and the comment claiming otherwise is gone |
| AC-18 | `make ze-rfc-index` runs twice on an unchanged tree | `ai/RFC-REQUIREMENTS.md` is byte-identical, and contains a derived "Enrolled without a public status row" table and a "Declared not enrolled" table |
| AC-19 | `make ze-rfc-check` runs on the committed tree after every Owner Ruling obligation lands | Exit 0 |
| AC-20 | `rfc/full/rfc3765.txt` and `rfc/full/rfc4486.txt` are present in the tree | `check_enrolment` raises no missing-source error for either stem, and both are enrolled |
| AC-21 | Each of `rfc/short/rfc1035.md`, `rfc3765.md`, `rfc4486.md`, `rfc5301.md` is re-authored | Each declares at least one GATED (MUST-level) requirement; every checklist line carries a section-anchored id; no line retains an `-x-` anchor and no obligation remains behind a bare bracket tag; every checkbox is `[ ]` |
| AC-22 | The enrolment ledgers after phase 6 | All four stems appear in `rfc/enrolled.txt` and none in `rfc/not-enrolled.txt`; `rfc/not-enrolled.txt` holds exactly five stems (rfc6987, rfc7999, rfc8195, rfc8326, rfc9129); the gate exits 0 |
| AC-23 | A newly extracted gated requirement of the four has no test, or only one polarity | The requirement carries a positive AND a negative `RFC requirement:` tag, OR an annotation Thomas authorised in this spec's Owner Rulings, recorded there with the requirement id, the RFC section text verbatim and the producing code `file:line`. No annotation is written on the implementer's own authority; absent an answer the spec stays OPEN |
| AC-24 | The four `docs/features/rfc-status.md` rows after phase 6 | Each row's Status, Implemented coverage and Remaining match the post-extraction reality, each factual claim carries a `<!-- source: ... -->` anchor outside any fenced block, and any `{gap}` in the four summaries is disclosed per `check_status_agreement` |
| AC-25 | A reader opens `docs/features/rfc-status.md` without reading `scripts/dev/rfc_requirements.py` | The preamble tells them which properties are machine-checked (row presence for a new enrolment, gap-count agreement, disclosure under a `{gap}`, and that every summary is enrolled or declared) and which stay editorial judgement (Status, Area, Implemented coverage) |
| AC-26 | `make ze-rfc-check` is run at EVERY commit this spec produces, in order | Exit 0 at each one. In particular the commit that gives `check_unproven_support` its real body is the same as, or later than, the commit that enrols the fourth stem (OC-1, OC-5) |
| AC-27 | Each of the four stems at the moment it is enrolled (OC-6) | A valid `rfc/extraction/<stem>.json` sign-off exists for it, with every derived site and section classified and its register derived. `make ze-rfc-check` exits 2 for any of the four enrolled without one, which is child 1's AC-1 firing through this spec's entry point. Four enrolments produce four artifacts, and `ls rfc/extraction/*.json` grows by exactly four across phase 6 |
| AC-28 | This spec's landing position relative to `plan/spec-rfcgate-1-extraction.md` | Child 1 is on HEAD before the first phase-6 enrolment commit. If it is not, the four enrol with no extraction bar in existence and are grandfathered out of it permanently (OC-6). Verified from `git log`, not from the final tree, since a tree that ends green proves nothing about the order that produced it |

## 🧪 TDD Test Plan

### Unit Tests
All in `scripts/dev/rfc_requirements_test.py`, auto-discovered by
`scripts/dev/python_tests_test.go` and run by `--selftest` from `Makefile:437`.
New classes sit beside `TestStatusLedgerCrossCheck` (`:750`) and
`TestStatusDisclosureFailsClosed` (`:1754`).

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStatusCompleteness.test_new_enrolment_without_a_row_fails` | `scripts/dev/rfc_requirements_test.py` | AC-1 | |
| `TestStatusCompleteness.test_deleted_row_under_enrolment_fails` | same | AC-2 | |
| `TestStatusCompleteness.test_preexisting_rowless_enrolment_is_grandfathered` | same | AC-3 | |
| `TestStatusCompleteness.test_absent_baseline_judges_nothing` | same | AC-4 | |
| `TestStatusBaselineReader.test_status_baseline_survives_git_failure` | same | A-7, AC-4 | |
| `TestSummaryDisposition.test_undeclared_summary_fails` | same | AC-5 | |
| `TestSummaryDisposition.test_stem_in_both_files_fails` | same | AC-6 | |
| `TestSummaryDisposition.test_disposition_naming_no_summary_fails` | same | AC-7 | |
| `TestSummaryDisposition.test_leaving_disposition_without_enrolling_fails` | same | AC-8 | |
| `TestSummaryDisposition.test_unknown_kind_or_missing_reason_fails` | same | AC-9 | |
| `TestSummaryDisposition.test_disposition_file_parses_like_enrolled` | same | A-6 |  |
| `TestUnprovenSupport.test_support_claim_over_zero_gated_fails` | same | AC-10 | |
| `TestUnprovenSupport.test_non_normative_disposition_permits_the_claim` | same | AC-11 | |
| `TestUnprovenSupport.test_unsupported_and_future_rows_are_not_claims` | same | AC-10 boundary | |
| `TestUnprovenSupport.test_enrolled_stem_with_gated_requirements_passes` | same | AC-10 negative | |
| `TestUnprovenSupport.test_non_applicability_reason_is_rejected` | same | AC-14 | |
| `TestGapCountAgreement.test_matching_spelled_count_passes` | same | AC-12 | |
| `TestGapCountAgreement.test_mismatched_spelled_count_fails` | same | AC-12 | |
| `TestGapCountAgreement.test_compound_number_is_not_read_as_its_tail` | same | AC-12 (`Twenty-five` is 25, not 5) | |
| `TestGapCountAgreement.test_row_without_a_spelled_count_is_not_judged` | same | AC-12 boundary | |
| `TestGapCountAgreementRealFile.test_committed_page_agrees` | same | A-4 over the real `docs/features/rfc-status.md` | |
| `TestGapDisclosureScope.test_unenrolled_gap_with_a_row_must_disclose` | same | AC-15 | |
| `TestGapDisclosureScope.test_unenrolled_gap_without_a_row_is_clean` | same | AC-16 | |
| `TestGapDisclosureScope.test_enrolled_behaviour_unchanged` | same | regression on `check_status_agreement` | |
| `TestParseErrorReporting.test_unenrolled_parse_error_is_reported` | same | AC-17 | |
| `TestUnconvertedSummaries.test_advisory_only_summary_is_listed` | same | AC-13, D5 | |
| `TestUnconvertedSummaries.test_zero_source_count_verdict_names_pre_2119_doubt` | same | AC-13, R-7 | |
| `TestLedgerBacklogTables.test_enrolled_without_row_table_rendered` | same | AC-18 | |
| `TestLedgerBacklogTables.test_disposition_table_rendered` | same | AC-18 | |
| `TestLedgerBacklogTables.test_render_is_deterministic` | same | A-8, AC-18 | |
| `TestFourStemEnrolmentRealTree.test_all_four_are_enrolled_and_sourced` | same, over the real `rfc/` tree | AC-20, AC-22 | |
| `TestFourStemEnrolmentRealTree.test_each_declares_a_gated_requirement` | same | AC-21 | |
| `TestFourStemEnrolmentRealTree.test_no_x_anchor_and_no_bare_bracket_tag_remains` | same | AC-21 (the `-x-` and `[FORMAT]` forms) | |
| `TestFourStemEnrolmentRealTree.test_declared_remainder_is_exactly_five` | same | AC-22 | |
| `TestFourStemEnrolmentRealTree.test_no_requirement_id_lost_since_head` | same | A-11, A-12, R-9 | |

Wiring classes (`TestStatusCompletenessWiring`, `TestSummaryDispositionWiring`,
`TestUnprovenSupportWiring`, `TestGapCountWiring`, `TestGapDisclosureWiring`,
`TestParseErrorReportingWiring`) are listed in the Wiring Test table and are
mandatory, not optional duplicates of the helper tests above.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Spelled gap count parsed from a Remaining cell | 1..39 (`One`..`Thirty-nine`) | `Thirty-nine` = 39 | `Zero` -- not a gap claim, must not be parsed as a count | `Forty` -- unrecognised, must not silently read as 4 |
| Compound spelled number tail | `Twenty-one`..`Twenty-nine` | `Twenty-five` = 25 | N/A | A bare `Five` inside `Twenty-five` must NOT match (the false-mismatch this probe hit) |
| Gated requirement count backing a support claim | 0..N | 1 (claim permitted) | 0 (claim rejected unless `non-normative`) | N/A |
| Disposition kinds accepted | 3 | `blocked` | empty kind rejected | a fourth kind rejected |

### Functional Tests
The driving surface for the MACHINERY is `scripts/dev/rfc_requirements_test.py`,
executed by `make ze-rfc-check` (`--selftest`, `Makefile:438`) and independently
by `scripts/dev/python_tests_test.go` under `go test`. For the machinery itself
no `.ci` test applies: it adds no daemon code and no user-facing command, so
there is no ze process to drive.

Phase 6 is different. The tagged tests that prove the newly extracted
requirements test DAEMON behavior, and where a requirement can be proven through
a user entry point its binding is a `.ci` functional test in preference to a unit
test, per the umbrella's D3 constraint (`.ci` runs inside `ze-verify` on every
push). The location of each is decided per requirement during the walk and is
not guessable in advance; what is fixed is the preference order and the
both-polarity rule.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-rfc-check` on the committed tree | `Makefile:437` | A developer runs the pre-commit gate and the public ledger's edges are checked, not just its `{gap}` rows | |
| `make ze-rfc-index` then `git diff --exit-code ai/RFC-REQUIREMENTS.md` | `Makefile:442` | The rendered backlog is committed and stays fresh | |
| `python3 scripts/dev/rfc_requirements.py --selftest` | `scripts/dev/rfc_requirements_test.py` | Every new guard is driven from `run_check`, not from its helper | |
| `make ze-rfc-check` after each of the four enrolments, one stem at a time | `Makefile:437` | A developer enrolling an RFC learns immediately whether its extraction holds, instead of at the end of four | |
| The `ai/skills/ze-rfc.md` coverage self-check per stem (source keyword count against captured checklist lines) | `rfc/full/<stem>.txt` vs `rfc/short/<stem>.md` | The extraction is bounded by the RFC text rather than by the author's patience; an order-of-magnitude gap means under-capture | |

### Interop Tests (Scope: protocol)
N-A for the machinery: it changes no wire behavior and no protocol
implementation, only what the repository may claim about protocol conformance.

Conditional for phase 6: extraction proves EXISTING behavior and changes none of
it, so `ai/rules/interop-and-goal-validation.md` is not triggered by the
extraction itself. If a requirement turns out to be unimplemented, the fix is
protocol work, it leaves this spec by OR-1a's escalation, and the interop
obligation travels with it rather than being absorbed here.

## Files to Modify
- `scripts/dev/rfc_requirements.py` - four new checks plus their `run_check`
  wiring, a new `git show` baseline reader for the status page, a disposition
  loader, the narrowed `{gap}` exemption, the un-filtered parse-error reporting
  with its corrected comment, the corrected `captured` set and verdict text for
  `unconverted_summaries`, and two new `render_ledger` tables. The `STATUS_FILE`
  read moves from `:1683` to just after `_collect_for_check` so one parse feeds
  every consumer.
- `scripts/dev/rfc_requirements_test.py` - the fixtures above.
- The test files carrying the new `RFC requirement:` tags for the four stems
  (Go `_test.go` under the package that produces the behavior, or a `test/**.ci`
  where the requirement is reachable from a user entry point, which is the
  preferred binding). Which files these are is decided per requirement during the
  phase-6 walk; they are named in the spec's audit table as they land.
- `rfc/enrolled.txt` - header comments gain the sibling-file contract: every
  summary is enrolled or declared, and the remainder is a decision, not an
  absence. Four entries are ADDED (rfc1035, rfc3765, rfc4486, rfc5301), taking
  the file from 166 to 170.
- `rfc/short/rfc1035.md`, `rfc/short/rfc3765.md`, `rfc/short/rfc4486.md`,
  `rfc/short/rfc5301.md` - re-authored per `ai/skills/ze-rfc.md`, walked section
  by section against the fetched source. rfc1035 has no checklist at all today
  and its normative prose is pre-RFC-2119 lowercase; rfc3765 and rfc4486 carry
  six `-x-`-anchored advisory rows between them that get real section anchors;
  rfc5301 has four section-anchored advisory rows to KEEP (ids unchanged), two
  `[FORMAT]` bracket lines to convert or drop, and 4 uppercase MUST-level
  keywords in its source that nothing has captured.
- `ai/RFC-REQUIREMENTS.md` - regenerated (`make ze-rfc-index`) with the two new
  backlog tables and the corrected RE-AUTHOR table.
- `ai/rules/rfc-compliance.md` - ~~"What Keeps RFC Testing Valid (the four
  ratchets)" becomes six~~ **corrected 2026-07-29: the heading is at `:115` and
  already reads "the six ratchets". This spec takes it to SEVEN** -- and the
  section gains the disclosure guards.
- `ai/INDEX.md` - ~~`:212` corrects "the three ratchets" (already stale)~~
  **corrected 2026-07-29: `:212` reads "the five ratchets" and is stale by one,
  not three** -- and names the disposition file. The keyword row
  ~~at `:372`~~ **at `:374`** gains `rfc/not-enrolled.txt`. Line `:372` is now
  the "extended message" row, and a NEW extraction keyword row sits at `:375`.
- `docs/features/rfc-status.md` - preamble states which properties of the page
  are machine-checked and which remain editorial (OR-4, AC-25), with a source
  anchor to `scripts/dev/rfc_requirements.py`. **Corrected 2026-07-29: child 2
  has ALREADY added a preamble paragraph here ("How strong the proof behind a row
  is", `docs/features/rfc-status.md:7-8`, with its own `<!-- source: ... -->`
  anchor) covering evidence KIND/TIER. AC-25's four properties are still absent,
  so AC-25 is not satisfied. But this spec EXTENDS child 2's paragraph rather
  than writing a fresh preamble, and must not duplicate or displace it.**
  The four D1 rows (~~`:28` rfc4486, `:56` rfc3765, `:123` rfc5301,
  `:234` rfc1035~~ **-> `:32` rfc4486, `:60` rfc3765, `:127` rfc5301, `:238`
  rfc1035 -- every row shifted +4 by child 2's preamble**) are corrected in phase 6
  to match what the re-authored summaries actually declare.
- `docs/contributing/rfc-implementation-guide.md` - the enrol-or-declare step.
- `ai/skills/ze-rfc.md` - the skill that writes summaries must record a
  disposition when it does not enrol (canonical source; `make ze-ai-sync`
  regenerates the `.claude` / `.codex` / `.agents` copies, which are never
  edited directly per `ai/rules/canonical-sources.md`).

## Files to Create
- `rfc/not-enrolled.txt` - the declared remainder. Columns:
  `<stem>` TAB `<kind>` TAB `<reason>`, with `#` comment and blank-line
  tolerance matching `rfc/enrolled.txt`. Seeded with the nine current
  un-enrolled summaries: `rfc1035` and `rfc6987` (declare nothing at all),
  `rfc5301` (`backlog` -- 4 uppercase MUST-level keywords in
  `rfc/full/rfc5301.txt`, zero captured), `rfc8195` (`backlog`), and
  `rfc3765`, `rfc4486`, `rfc7999`, `rfc8326`, `rfc9129` (`blocked` -- no source
  text, which `check_enrolment:675-684` requires).
  → Revised by OR-1: the seed is still NINE rows at phase 2, but the four
    extracted stems are seeded `backlog` (never `non-normative`, OC-3/OC-4) and
    are discharged into `rfc/enrolled.txt` in phase 6, leaving FIVE at closure.
    `rfc1035` is `backlog` for the same reason: 0 uppercase MUST with 23
    lowercase `must` is a pre-2119 extraction owed, not an absence of obligation.
    `rfc6987` is `backlog` too, not `non-normative`: its 5 bracket-tag lines are
    unparsed obligations of exactly the D5 shape, so declaring it non-normative
    would launder them.
- `rfc/full/rfc3765.txt`, `rfc/full/rfc4486.txt` - fetched from
  `https://www.rfc-editor.org/rfc/<stem>.txt` at the start of phase 6. Without
  them neither stem can be enrolled at all (`check_enrolment:675-684`), and a
  summary written without its source is validated only against itself. Under OC-6
  the fetch is doubly binding: with no source text there is no site inventory to
  derive and no register to sign under, so the extraction sign-off is impossible
  too, not merely the enrolment.
- `rfc/extraction/rfc1035.json`, `rfc/extraction/rfc3765.json`,
  `rfc/extraction/rfc4486.json`, `rfc/extraction/rfc5301.json` - the four
  extraction sign-offs OC-6 requires, in the format
  `plan/spec-rfcgate-1-extraction.md` defines and this spec consumes without
  modifying. Each is hand-classified site by site during phase 6, immediately
  before its stem is enrolled. They are NOT optional and NOT deferrable: child 1's
  `check_enrolment` precondition refuses the enrolment without them (AC-27).
- `plan/deferrals/rfcgate-4-ledger.md` - created at implementation start with
  the 32-row deferral (see Known Limitations), whose Reason carries the OR-3
  ordering dependency.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Developer tooling; no daemon config surface |
| YANG validation constraints | N-A | As above |
| YANG custom validators | N-A | As above |
| CLI commands/flags | N-A | The script's flags are unchanged: `--check`, `--check-fresh`, `--write`, `--selftest` |
| CLI grammar (keyword before value) | N-A | No `ze` command added |
| Editor autocomplete | N-A | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC; the driving surface is `scripts/dev/rfc_requirements_test.py` |
| Pipe completeness | N-A | Not a `ze` command; output is a gate report |
| Env var registration | N-A | No env var read |
| Doctor check for runtime dependencies | N-A | No runtime dependency; `rfc/not-enrolled.txt` is a build-time input to a dev gate, never read by the daemon |
| Prometheus counters/metrics | N-A | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer gate only; nothing ships in `ze` |
| 2 | Config syntax changed? | No | No YANG, no config leaf |
| 3 | CLI command added/changed? | No | Script flags unchanged |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Contributor surface, not operator surface |
| 7 | Wire format changed? | No | No wire path touched |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` preamble (what is machine-checked, OR-4/AC-25) plus the four D1 rows (`:28`, `:56`, `:123`, `:234`), each with a source anchor. Under OR-1 this spec also newly PROVES protocol behavior: every gated requirement extracted for rfc1035, rfc3765, rfc4486 and rfc5301 gains tagged tests, so `ai/RFC-REQUIREMENTS.md` is regenerated and the Status / Implemented coverage / Remaining cells of the four rows are rewritten from what the summaries now declare |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (RFC Requirement Tags section) and `docs/contributing/rfc-implementation-guide.md` gain the enrol-or-declare step |
| 11 | Affects daemon comparison? | Yes, conditionally | `docs/comparison.md` and `docs/features.md` are reconciled if and only if the OR-1 extraction changes a support LEVEL for one of the four (`ai/rules/discovery-updates.md` requires the reconciliation when the level moves). The machinery alone changes nothing there; grep both files for the four stems in phase 6 and record the grep when the answer is "no change needed" |
| 12 | Internal architecture changed? | No | No component boundary moves |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry touched |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for `source: scripts/dev/rfc_requirements.py` and for anchors naming `rfc/enrolled.txt`; update each claim about what the gate enforces |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/skills/ze-rfc.md` and `docs/contributing/rfc-implementation-guide.md` show the enrolment step; both must show the disposition alternative |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- create every entry point as a stub
   called from `run_check`, and write the wiring tests so they fail because the
   stub returns no errors.
   - Tests: the six `*Wiring` classes from the Wiring Test table, plus
     `TestLedgerBacklogTables.test_ledger_renders_enrolled_without_row_and_dispositions`
   - Files: `scripts/dev/rfc_requirements.py` (stub checks + call sites; move the
     `STATUS_FILE` read up), `scripts/dev/rfc_requirements_test.py`
   - Verify: each wiring test fails on an assertion about the missing error
     string, never on an import or a signature error
   - **OC-1 applies from here:** the `check_unproven_support` stub is wired now
     and stays INERT (returns no errors) until phase 7. A stub that judges
     nothing cannot red the tree, which is what makes wiring-first compatible
     with arming-last
   - Also in this phase: create `plan/deferrals/rfcgate-4-ledger.md` with the
     32-row deferral, so the scope decision is recorded when it is made rather
     than at commit time. Its Reason carries the OR-3 ordering dependency
2. **Phase: Declared remainder** -- the disposition file, its loader, and
   `check_summary_disposition`.
   - Tests: the `TestSummaryDisposition` class (AC-5..AC-9, AC-14, A-6)
   - Files: `rfc/not-enrolled.txt` (nine seeded rows), `rfc_requirements.py`,
     `rfc/enrolled.txt` header
   - Verify: `make ze-rfc-check` green with all nine declared; removing any one
     row reds
   - Four of the nine (the OR-1 stems) are seeded `backlog` and are discharged
     into `rfc/enrolled.txt` in phase 6, which is also the happy path of AC-8.
     `backlog` here states DEBT and excuses nothing (OC-4)
3. **Phase: Status-row completeness** -- the git baseline reader and the
   two-sided ratchet.
   - Tests: `TestStatusCompleteness`, `TestStatusBaselineReader` (AC-1..AC-4, A-7)
   - Files: `rfc_requirements.py`
   - Verify: the 32 pre-existing stems do not red; a synthetic new enrolment does
4. **Phase: Disclosure scope and parse-error honesty** -- narrow the `:1176`
   exemption, drop the enrolment filter on parse errors, correct the comment.
   - Tests: `TestGapDisclosureScope`, `TestParseErrorReporting` (AC-15..AC-17)
   - Files: `rfc_requirements.py`
   - Verify: green on the real tree (A-2, A-5 make both no-ops today), and red on
     synthetic inputs
5. **Phase: Cross-check and render** -- the gap-count agreement check, the
   corrected `captured` set and verdict text, the two backlog tables.
   - Tests: `TestGapCountAgreement`, `TestGapCountAgreementRealFile`,
     `TestUnconvertedSummaries`, `TestLedgerBacklogTables` (AC-12, AC-13, AC-18)
   - Files: `rfc_requirements.py`, then `make ze-rfc-index` to commit
     `ai/RFC-REQUIREMENTS.md`
   - Verify: 60/60 rows agree on the committed page; the ledger renders
     identically twice. The RE-AUTHOR table grows from 2 rows to 9: the two that
     declare nothing at all (rfc1035, rfc6987) plus the seven advisory-only stems
     the D5 bug hid. Four of those nine are phase 6's worklist; the other five are
     the declared remainder, and the table is where they stay visible
6. **Phase: Extract and enrol the four (OR-1)** -- the phase this spec exists to
   make possible, and the one that must land BEFORE the guard arms.
   Per stem, in this order, one stem per commit:
   - Re-run the tag grep from A-11. A tagged id on any of the four closes the
     re-anchoring window and the id-correction step is dropped for that stem
   - FETCH the source when absent: `rfc/full/rfc3765.txt`, `rfc/full/rfc4486.txt`
     from `https://www.rfc-editor.org/rfc/<stem>.txt`. No source, no enrolment
   - RE-AUTHOR the summary per `ai/skills/ze-rfc.md`, walking the RFC section by
     section. rfc1035 is pre-RFC-2119: extract the LOWERCASE normative prose
     (0 uppercase MUST, 23 lowercase `must`). rfc5301 has 4 uncaptured
     uppercase MUST-level keywords plus 2 `[FORMAT]` lines the parser reads as
     prose. Section-anchor every id; keep every id a test already uses
   - Run the skill's coverage self-check for that stem, then `make ze-rfc-check`
   - Write the tagged tests: a POSITIVE and a NEGATIVE tag per gated requirement.
     Anything less is ESCALATED, never annotated (see the Failure Routing rows)
   - **SIGN OFF the extraction (OC-6, BLOCKING before enrolment):**
     `make ze-rfc-extract STEM=<stem>`, then classify every derived site and
     section by hand into `rfc/extraction/<stem>.json`. Child 1 has landed by
     now, so `check_enrolment` REFUSES the next step without this artifact
     (`plan/spec-rfcgate-1-extraction.md` AC-1). This replaces the skill's
     two-grep coverage self-check with a recorded artifact rather than adding a
     step beside it. Four stems, four artifacts
   - ENROL the stem in `rfc/enrolled.txt`, remove its `rfc/not-enrolled.txt` row
     (the only legal discharge, AC-8), correct its `docs/features/rfc-status.md`
     row with source anchors, and run `make ze-rfc-index`
   - Tests: `TestFourStemEnrolmentRealTree` (AC-20..AC-22, AC-24)
   - Files: the four `rfc/short/*.md`, two new `rfc/full/*.txt`,
     `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `docs/features/rfc-status.md`,
     the tagged test files, `ai/RFC-REQUIREMENTS.md`
   - Verify: `make ze-rfc-check` exit 0 after EACH stem, not only after the fourth
   - **Scoping gate (OR-1, expected):** RFC 1035 is the DNS specification and its
     obligation tail is not knowable before the walk (A-10). When the real count
     is known, report it and scope the tail WITH the owner. That conversation is
     part of this phase, not a surprise inside it, and it is not a licence to
     truncate the extraction unilaterally
7. **Phase: Arm the unproven-support guard** -- the D1 gate, given its real body.
   - Tests: `TestUnprovenSupport` (AC-10, AC-11, AC-14), `TestUnprovenSupportWiring`
   - Files: `rfc_requirements.py`
   - **Ordering (OC-1, BLOCKING):** this phase may share a commit with the last
     stem of phase 6, or land after it. It may NEVER land before. A stub was
     already wired in phase 1 and is inert until now
   - Verify: the gate names exactly the A-3 stems when they are synthetically
     re-broken in a fixture, and names NOTHING on the real tree
8. **Phase: Rules, docs and indexes** -- everything downstream of the machinery.
   - Tests: `make ze-verify` (AC-19), `make ze-doc-test`
   - Files: `ai/rules/rfc-compliance.md` (four ratchets becomes six),
     `ai/INDEX.md` (`:212` "the three ratchets", `:372` keyword row),
     `docs/features/rfc-status.md` preamble (OR-4, AC-25),
     `docs/contributing/rfc-implementation-guide.md`, `docs/functional-tests.md`,
     `ai/skills/ze-rfc.md` (then `make ze-ai-sync`), and the
     `docs/comparison.md` / `docs/features.md` grep from Documentation row 11
   - Verify: `make ze-verify` green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line` and a named test |
| Feature completeness | Each of the six new/changed guards is CALLED from `run_check` (or `render_ledger`); grep the call sites, do not trust the helper tests |
| Correctness | Each new ratchet judges NOTHING on a degraded baseline, and says so in its docstring; a false "clean" is the failure mode that matters |
| Correctness | The unproven-support guard treats a present-but-empty Status cell as a claim, not as a pass (`ai/rules/fail-closed-guards.md`, present-but-empty) |
| Correctness | The spelled-number parser cannot read a compound as its tail (`Twenty-five` is 25) and cannot read an unrecognised word as a match |
| Naming | Disposition kinds are the three declared words; the file's column order matches `rfc/enrolled.txt` so one reader serves both |
| Data flow | `STATUS_FILE` is opened exactly once per run and its rows are shared by every consumer; no second parse |
| Rule: `ai/rules/derive-not-hardcode.md` | No checked-in list of the 32; the backlog is `enrolled - rows`, rendered |
| Rule: `ai/rules/rfc-compliance.md` | No new code derives prose from an annotation's classification; only the COUNT is cross-checked (R-2) |
| Rule: `ai/rules/no-test-deletion.md` | Existing status-ledger fixtures are ADDED to, never rewritten to accommodate a new check |
| Rule: `ai/rules/stale-comments.md` | The migration-amnesty comment is gone, not reworded around a branch that still filters |
| Ordering (OC-1) | Walk the commit sequence, not just the final tree: `git log -p` the phase-7 commit and confirm no earlier commit gave `check_unproven_support` a judging body. A final tree that is green proves nothing about the commits it took to get there |
| Ordering (OC-3) | No baseline, allowlist, `non-normative` disposition, or `{not-applicable}` shortcut anywhere near the four stems. Grep the diff for their names in every gate-input file |
| Ordering (OC-6) | Confirm from `git log` that `plan/spec-rfcgate-1-extraction.md`'s machinery is on HEAD before the first phase-6 enrolment, and that each of the four enrolment commits carries its `rfc/extraction/<stem>.json`. This is the check that cannot be recovered later: an enrolment that lands before the bar exists is grandfathered out of it permanently, and nothing in the final tree shows that it happened |
| Rule: `ai/rules/rfc-compliance.md` (OR-1) | Every gated requirement added to the four summaries has BOTH polarities tagged, or an annotation whose authorisation is recorded verbatim in Owner Rulings with the requirement id, the RFC section text and the producing `file:line`. An annotation with no recorded authorisation is a BLOCKER, not a NOTE |
| Extraction completeness (OR-1) | For each of the four, the reviewer re-runs the skill's coverage self-check independently. A summary that captured far fewer lines than the source's keyword count is under-captured, and for rfc1035 the uppercase count is 0 so the comparison must be made against the lowercase prose instead |
| Requirement ids (R-9) | No id that existed at HEAD was renumbered or dropped; the six `-x-` anchors became real section anchors; rfc5301's four existing section-anchored ids are unchanged |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `rfc/not-enrolled.txt` exists, seeded with nine stems at phase 2 and holding exactly five at closure | `wc -l rfc/not-enrolled.txt` and `make ze-rfc-check` |
| The four D1 stems are enrolled, sourced, and re-authored | the non-comment non-blank line count of `rfc/enrolled.txt` reads 170 (it reads 166 today; count it the way `parse_enrolled` does, skipping `#` and blank lines, since a raw `wc -l` counts the header comments too); `ls rfc/full/rfc3765.txt rfc/full/rfc4486.txt`; `make ze-rfc-check` exit 0 |
| Every gated requirement of the four is proven or individually authorised | `make ze-rfc-index` then read the four blocks of `ai/RFC-REQUIREMENTS.md`: each row shows a positive and a negative test, or an annotation whose authorisation is quoted in Owner Rulings |
| The guard armed no earlier than the fix | `git log -p -- scripts/dev/rfc_requirements.py` across this spec's commits: the judging body of `check_unproven_support` appears in the phase-7 commit or the last phase-6 commit, never before |
| The public page says what it guarantees | ~~`sed -n '1,10p' docs/features/rfc-status.md` shows the machine-checked vs editorial paragraph (AC-25)~~. **FALSE-PASS as of 2026-07-29.** That command ALREADY prints a preamble paragraph (child 2's "How strong the proof behind a row is", `:7-8`), so it passes before this spec writes anything. **Replace it with a check for AC-25's four named properties: `grep -n "row presence\|gap-count\|editorial" docs/features/rfc-status.md`, and confirm Status / Area / Implemented coverage are each named as editorial** |
| Every summary is enrolled or declared | `make ze-rfc-check` (AC-5) |
| Newly enrolled RFCs must disclose | `make ze-rfc-check` after a synthetic enrolment (AC-1) |
| A support claim cannot rest on an empty checklist | `make ze-rfc-check` (AC-10) |
| The 32-row backlog is visible | `grep -A3 "Enrolled without a public status row" ai/RFC-REQUIREMENTS.md` |
| The deferral is homed | `grep -n "Destination" plan/deferrals/rfcgate-4-ledger.md` |
| Every new guard is wired | `python3 scripts/dev/rfc_requirements.py --selftest` |
| Ledger is fresh | `make ze-rfc-index` then `git diff --exit-code ai/RFC-REQUIREMENTS.md` |
| Rules and indexes updated | `make ze-doc-test` and `make ze-rules-condensed` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `rfc/not-enrolled.txt` is developer-authored, never network input; the loader must still reject a malformed line loudly rather than skipping it, so a typo cannot silently un-declare a summary |
| Untrusted subprocess output | `git show HEAD:<path>` output is parsed with the same tolerant reader as the existing baselines; a failure returns an empty result and judges nothing, never an exception that aborts the run |
| Resource exhaustion | Each new check is a single fold over already-parsed data; only one additional `git show` per run |
| Error leakage | Error strings quote stem names and counts, never file contents beyond the offending line |
| Authorization that could fail open | The whole spec is about a guard failing open. Every new branch's default must be "fail", and the degraded-baseline case must be explicitly quiet rather than accidentally permissive |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | N-A (Python); an import or attribute error routes to the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure (`ruff`) | Fix inline; if architectural → DESIGN |
| `make ze-rfc-check` red on the real tree for a reason A-1..A-5 did not predict | Re-run the census probe before changing any check; an assumption is broken and gets a Mistake Log row |
| A newly extracted gated requirement of the four has no producing code, or admits only one polarity | STOP and ESCALATE to Thomas per `ai/rules/rfc-compliance.md:38-51`. Quote the requirement id, the RFC section text VERBATIM, and the producing code (or its absence) as `file:line`; state what full implementation plus a tagged pair would cost; ask WHICH WAY to fix it. Never "may I skip it", never an annotation written on your own authority (`ai/skills/ze-rfc.md`, Annotations). The spec stays OPEN pending the answer, and the answer is recorded in Owner Rulings where the question was |
| The RFC 1035 walk yields a larger obligation tail than the phase assumed (A-10) | Report the real count and scope the tail WITH the owner. This is an expected conversation, named in phase 6. Do NOT truncate the extraction, do not stop at the "interesting" sections, and do not enrol a partial capture to reach green |
| A source fetch fails or returns something that is not the RFC | STOP. That stem cannot be enrolled at all (`check_enrolment:675-684`) and a summary written without its source is validated only against itself. Report it; never enrol on a summary alone |
| `make ze-rfc-check` reds on the four before phase 7 | A commit was assembled out of order (OC-1). Do not log it, do not `--unverified` past it, do not baseline it. Move the judging body back into phase 7 |
| Phase 6 stalls and phases 7-8 are ready | They wait. OC-1 is not negotiable by convenience, and shipping the gate over unresolved stems is the exact failure OR-2 corrected |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The gate already contains every mechanism this spec needs. Three ratchets read
  `git show HEAD:<blob>`, one backlog is rendered rather than listed, and one
  loader tolerates comments. The defects are not missing machinery; they are
  edges nobody pointed the existing machinery at.
- The two ledgers disagree in one direction only today because two independent
  coincidences hold (every `{gap}` RFC is enrolled, and every one has a row).
  A guard whose correctness rests on a coincidence is not a guard, it is a
  coincidence with a test suite.
- The strongest verification finding was not in the brief: one advisory row buys
  a summary immunity from the RE-AUTHOR verdict (D5). It shows the general
  shape -- `captured` meant "captured anything", the check meant "captured the
  obligations", and the mismatch is invisible because the table it feeds is
  supposed to be short.
- Deriving the row was tempting and is wrong. Verified: Area is an editorial
  label, not the summary H1; Remaining is source-anchored prose whose content is
  built from classifications `ai/rules/rfc-compliance.md:56` voided as authority.
  The only fact a machine can own is the gap COUNT -- and 60 rows already carry
  one, all 60 correct, with nothing enforcing it. That is a cross-check waiting
  to be written, not a generator waiting to be written.
- **A gate armed over an unresolved defect blocks the commit that fixes it.**
  This spec's first draft chose to land the D1 guard red, on the sound principle
  that grandfathering the sharpest defect ships the gate and keeps the bug. The
  principle survived; the sequencing did not. `ze-rfc-check` runs in both verify
  modes and `commit_helper.py` refuses a script over a non-FRESH verify, so a red
  there is not a pressure applied to the four stems, it is a pressure applied to
  every commit in the repository -- including the four commits that clear them.
  The generalisation is worth carrying: a ratchet may only be armed at or after
  the moment its subject is clean, and "arm it red to force the fix" is
  self-defeating whenever the gate sits on the path to the fix.
- **The route from "consistent" to "true" ran through extraction, not
  downgrade.** Both ledgers agreeing on nothing is the cheapest way to make D1
  green: write `Unsupported` in four cells and the guard passes forever. The
  owner took the other branch. It is the same asymmetry `ai/rules/rfc-compliance.md`
  encodes at `:40` -- making Ze more conformant never needs permission, and only
  the narrower answer has to be asked for.
- **The id-correction window closes at enrolment, and only luck left it open.**
  All six checklist rows in rfc3765 and rfc4486 carry the `-x-` anchor the skill
  calls a defect marker. Re-anchoring them is safe only while no test tags them,
  and after enrolment `check_retired_requirements` freezes ids for good. Nobody
  planned that ordering; extraction simply has to happen before proof, and proof
  is what creates the tags. Worth naming so the next extraction does the
  correction first rather than discovering the door shut.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Status-row completeness is a git-HEAD ratchet, not a hard requirement | (a) hard gate, writing all 32 rows now; (b) a checked-in baseline file listing the 32 | (a) makes 32 separate product judgements, each a potential Thomas escalation, inside a machinery spec; (b) is a hand-maintained second list, the exact thing `ai/rules/derive-not-hardcode.md` forbids and the thing `check_new_summaries` already declined to build. The git baseline is derived, matches three existing ratchets, and the backlog stays visible because it is rendered |
| A sibling `check_status_completeness`, not an extension of `check_enrolment` | Extending `check_enrolment` (it already folds over enrolled stems) | `check_enrolment` answers "is enrolment coherent with the summaries"; rows are a different question with a different input. Extending it would force the `STATUS_FILE` read above `:1637` for one caller and would rewrite three passing fixtures for no gain (`ai/rules/no-test-deletion.md`). The `STATUS_FILE` read moves anyway, but for sharing, not for coupling |
| The remainder is declared in a sibling file with a KIND | (a) a comment in `rfc/enrolled.txt`; (b) a bare stem list; (c) a `not-enrolled:` prefix line inside `rfc/enrolled.txt` | (a) is invisible to `parse_enrolled` and therefore ungateable. (b) cannot distinguish "the RFC imposes nothing" from "nobody extracted it" from "we have no text", and that distinction is the whole point of D3. (c) would be parsed as a stem named `not-enrolled:` by `line.split()[0]` |
| Three kinds: `non-normative`, `backlog`, `blocked` | A free-text reason only | A free-text reason lets "not enrolled" read as "settled", which is R-4. `backlog` and `blocked` render as DEBT, so declaring the remainder cannot launder unextracted obligations into decisions |
| The gap count is cross-checked, never generated | (a) generate Remaining from `{gap}` ids; (b) leave it hand-written and unchecked | (a) inherits the voidness of every annotation (`rfc-compliance.md:56`) and would publish a generated compliance claim built from classifications nobody re-derived. (b) is the status quo, correct 60/60 times by discipline alone with no gate |
| ~~The D1 guard lands armed, red on four stems~~ | ~~Grandfathering the four behind a baseline, as with the 32 rows~~ | ~~The 32 are a docs-completeness gap ... the four are the opposite~~ SUPERSEDED by OR-2, 2026-07-29. The comparison with the 32 is retained and still governs (see the replacement row); only the arming point was wrong. Landing the guard red would have blocked every commit in the repository, the fixing ones included |
| The guard arms in the same commit that clears the four, or later, and the four are never excused | (a) arming first and clearing after, the superseded row above; (b) a HEAD baseline over the four; (c) a `non-normative` disposition for the four | (a) is unusable: `ze-rfc-check` is a stage in both verify modes (`scripts/status/verify_run.go:237`, `:259`) and `commit_helper.py create` refuses over a non-FRESH verify, so the red lands on every commit including the fix. (b) and (c) are the grandfather clause the retained principle forbids -- the 32 rowless enrolments are excusable because each has its requirements classified, while the four are a public claim with nothing underneath (`ai/rules/no-parking.md`). The distinction is preserved exactly; only the ORDER changed. See Ordering Constraints OC-1..OC-5 |
| The four D1 stems are extracted and enrolled, not downgraded (OR-1) | (a) downgrade the four public rows to `Unsupported` / `Partial`; (b) declare them `non-normative`; (c) leave them to `plan/spec-rfcgate-1-extraction.md` | Owner ruling. All three alternatives make the ledger consistent by lowering what Ze claims, and each is a classification that lowers what Ze owes, which `ai/rules/rfc-compliance.md:38-51` reserves to Thomas -- he chose the fullest option instead. (c) additionally leaves a false public claim standing for the length of another spec |
| The four are DECLARED (`backlog`) in phase 2 before they are ENROLLED in phase 6 | Seeding `rfc/not-enrolled.txt` with only the five that remain, and landing the extraction first | Every intermediate commit must be coherent and green (OC-5), and phase 2's own check demands that every summary be enrolled or declared. Since the rfc1035 tail is unknowable before the walk (A-10), phase 6 has an open-ended length, and the tree cannot sit incoherent for it. `backlog` is the honest word for that interval: it renders as DEBT, asserts nothing about conformance, and its discharge into `rfc/enrolled.txt` is exactly the transition AC-8 exists to police |
| The `-x-` id anchors are corrected during the re-author, not later | Leaving them and adding new rows beside them | An id is a permanent contract, and after enrolment `check_retired_requirements` freezes every one. Verified zero tags on all four stems today (A-11), so the correction is free now and impossible later. `ai/skills/ze-rfc.md` allows a section change "only while no test tags it" |
| The `{gap}` disclosure exemption is narrowed to "no row", not removed | Removing `req.rfc not in enrolled` entirely | An un-enrolled, un-rowed RFC makes no public claim, so there is nothing to contradict and demanding a row would force rows for reference-only summaries. Narrowing to "has a row" closes the hole and is a no-op today (A-2) |

## Owner Rulings (SETTLED 2026-07-29)

Every question this spec raised has been answered by Thomas. Nothing here is
open. The questions are kept verbatim below the rulings because the ruling only
means something beside the thing it decided; the RULINGS are what the
implementation obeys.

| ID | Ruling | Answers | Basis |
|----|--------|---------|-------|
| OR-1 | **EXTRACT AND ENROL ALL FOUR STEMS.** rfc1035, rfc3765, rfc4486 and rfc5301 are each published as Supported or Experimental over a checklist with zero MUST-level rows. Extract their real obligations and enrol all four, so the public claim becomes TRUE rather than merely consistent. Fetch the two missing sources, re-author all four summaries section by section per `ai/skills/ze-rfc.md`, enrol them in `rfc/enrolled.txt`, and correct their `docs/features/rfc-status.md` rows | Q-1, Q-2, Q-3 | Owner ruling, 2026-07-29. The fullest option was chosen over every narrower one. Implemented as phase 6 |
| OR-1a | **Every newly extracted requirement is proven, or escalated individually.** Each needs a positive AND a negative tagged test. Anything less is escalated to Thomas per `ai/rules/rfc-compliance.md` "Ask Thomas Whenever Full Compliance Is On The Table", quoting the requirement id, the RFC section text VERBATIM, and the producing code as `file:line`. This spec does NOT pre-authorise annotating any of them | Q-1, Q-2, Q-3 | Owner ruling, 2026-07-29. Enforced by AC-23 and the Failure Routing rows |
| OR-1b | **The rfc1035 tail is not knowable before the walk, and that is recorded rather than guessed.** RFC 1035 is the DNS specification; its normative prose is pre-RFC-2119 lowercase. Scoping the tail with the owner once the real count is known is PART of the phase, not a surprise inside it | Q-1 | Owner ruling, 2026-07-29. Recorded as A-10 and as phase 6's scoping gate |
| OR-2 | **The sequencing was wrong; the principle was not.** Landing `check_unproven_support` armed and red on the four was correct in principle (grandfathering the sharpest defect ships the gate and keeps the bug) and unusable in practice: a red `ze-rfc-check` blocks every commit in this repository, including the commits that fix the four. The gate arms in the SAME commit that resolves them, or a later one, NEVER before. The four are never given a permanent exemption list: they are FIXED, not excused | (correction to this spec's own design) | Owner ruling, 2026-07-29. Implemented as Ordering Constraints OC-1..OC-5, and as the strikethrough in Key Design Decisions |
| OR-3 | **The 32 rowless enrolled RFCs stay scoped OUT and deferred**, and the deferral row as written is correct. Their ordering FOLLOWS the annotation re-derivation programme rather than preceding it, because the judgement that they are safe rests on `{not-applicable}` annotations that `ai/rules/rfc-compliance.md:53` voided as authority on 2026-07-27. That dependency is recorded explicitly in the deferral's Destination reasoning, so a future reader does not write the rows first and inherit the void basis | Q-4 | Owner ruling, 2026-07-29. The re-derivation programme's home is `plan/spec-followup-rfc-enrollment.md` per the umbrella's Out of Scope table |
| OR-4 | **YES, the public page states its own guarantees.** `docs/features/rfc-status.md` carries a preamble paragraph naming which of its properties are machine-checked and which remain editorial judgement. A reader must be able to tell, WITHOUT reading the gate source, that Status is a human judgement while row presence, gap disclosure and gap counts are enforced | Q-5 | Owner ruling, 2026-07-29. Added as AC-25 |

### The questions these rulings answered

Kept verbatim. Each was a case where "implement the RFC fully and prove it fully"
was on the table, which `ai/rules/rfc-compliance.md` says this spec was not
authorised to answer. None was a "may I skip it" -- each asked which way to fix
it, and each has been answered above.

| # | Question | Evidence |
|---|----------|----------|
| Q-1 | **ANSWERED: OR-1, OR-1a, OR-1b -- extract and enrol.** RFC 1035 is published as `Supported` (`docs/features/rfc-status.md:234`) while `rfc/short/rfc1035.md` declares zero requirements at any level. Its source text contains 0 uppercase `MUST` but 23 lowercase `must` -- it predates RFC 2119. Ze answers DNS authoritatively (`internal/core/dnsserver/handler.go:48`). Extract RFC 1035's obligations and enrol it, downgrade the public claim, or declare it `non-normative` on the strength of the pre-2119 wording? | Probe over `parse_summary_file`, `source_keyword_count`, and `grep -ci "\bmust\b" rfc/full/rfc1035.txt` |
| Q-2 | **ANSWERED: OR-1, OR-1a -- fetch both sources, extract, enrol.** RFC 3765 (`:56`) and RFC 4486 (`:28`) are published as `Supported`, declare only advisory requirements, and have NO source text under `rfc/full/` or `rfc/drafts/`, so `check_enrolment:675-684` forbids enrolling them as they stand. Fetch the source and extract, or downgrade the claim? | Same probe; `rfc/full/` and `rfc/drafts/` listing |
| Q-3 | **ANSWERED: OR-1, OR-1a -- extract the four now; `backlog` is the interval, not the outcome.** RFC 5301 (`:123`) is published as `Experimental` while `rfc/full/rfc5301.txt` carries 4 uppercase MUST-level keywords and the summary captured zero (its four rows are SHOULD/MAY). Extract the four now, or record it `backlog` and let the extraction program take it? | Probe: `source_keyword_count("rfc5301") == 4`, gated count 0 |
| Q-4 | **ANSWERED: OR-3 -- the 32 stay deferred, sequenced BEHIND the re-derivation.** 539 `{gap}` and every `{not-applicable}` in the tree were VOIDED as authority on 2026-07-27 (`ai/rules/rfc-compliance.md:56`). The 32 rowless enrolled RFCs are judged safe here because none carries a `{gap}` -- a judgement that rests on exactly those voided classifications. Should the 32-row work be sequenced behind a re-derivation of their annotations, or does writing the rows come first? | `ai/rules/rfc-compliance.md:53-62`; the annotation census in R-3 |
| Q-5 | **ANSWERED: OR-4 -- yes, and it is AC-25.** Should `docs/features/rfc-status.md` state in its preamble which properties are machine-checked (row presence for new enrolments, gap-count agreement, disclosure under a `{gap}`) and which stay editorial (Status, Area, Implemented coverage)? It makes the page honest about its own guarantees; it also publishes the gate's boundaries. | This spec's Documentation checklist row 9 |

## Known Limitations
- **The 32 missing status rows are NOT written here.** Writing them is 32
  separate product judgements about Status and source-anchored Implemented
  coverage prose, several of which (rfc7611, rfc7440, rfc8097, rfc3786, rfc4576,
  rfc5561, rfc6397, rfc2349) are deliberate non-implementations a user would come
  to the page to look up. That is neither machinery nor cheap, and per OR-3 it
  needs the annotation re-derivation FIRST: the judgement that the 32 are safe
  (none carries a `{gap}`) rests on `{not-applicable}` annotations voided as
  authority on 2026-07-27. Recorded in `plan/deferrals/rfcgate-4-ledger.md`
  with Destination `plan/spec-rfcgate-0-umbrella.md` and Status `deferred`, and
  its Reason carries that ordering dependency naming
  `plan/spec-followup-rfc-enrollment.md`; the umbrella should route it to a child
  spec. The work stays visible through the rendered "Enrolled without a public
  status row" table, and the ratchet ensures the number can only shrink from 32.
- **The rfc1035 obligation tail is unmeasured until phase 6 walks it** (A-10,
  OR-1b). RFC 1035 is the DNS specification, its normative prose predates RFC
  2119, and no keyword count can size it in advance. The phase reports the real
  count to the owner and scopes the tail with him at that point. This is a stated
  unknown, not an estimate presented as one.
- **The five remaining declared stems are still unextracted debt.** rfc6987,
  rfc7999, rfc8195, rfc8326 and rfc9129 end this spec declared, not enrolled.
  rfc6987 is `backlog` rather than `non-normative` on purpose: its five
  bracket-tag lines are unparsed obligations of exactly the D5 shape, and calling
  it non-normative would launder them. Their extraction belongs to
  `plan/spec-rfcgate-1-extraction.md`, and OR-1 extends to the four D1 stems
  only.
- **A mis-annotated `{not-applicable}` still vanishes from both ledgers at
  once** (R-3). Nothing here detects a wrong classification -- only a missing
  disclosure, a missing row, a missing decision, and a wrong count. Detecting a
  wrong classification is the re-audit program's job (`/ze-rfc-audit`, the
  `check_audit_freshness` SHA ratchet), not this one's.
- **`source_keyword_count` remains uppercase-only.** The lowercase count added
  for the RE-AUTHOR verdict is evidence in the rendered table, not a gate input.
  A pre-2119 RFC still needs a human to judge whether its lowercase "must"
  clauses bind.
- **Row PROSE quality is not gated** (R-1). The completeness ratchet checks
  presence; `check_status_agreement` already rejects an empty Remaining under a
  `Supported` claim. A boilerplate Remaining that is technically non-empty
  remains a review concern, deliberately.

## RFC Documentation (Scope: protocol)

No `// RFC NNNN Section X.Y: "<quoted requirement>"` comments are added or
changed: this spec enforces no protocol obligation in code. It changes the
machinery that decides whether the repository's CLAIMS about such obligations
are allowed to stand, which is `ai/rules/rfc-compliance.md`'s "What Keeps RFC
Testing Valid" section rather than its "RFC MUST Comments" section. The one
documentation obligation that follows is the ratchet table in that rule
(~~four ratchets becomes six~~ **six becomes seven, corrected 2026-07-29:
`ai/rules/rfc-compliance.md:115` already reads "the six ratchets"**), listed in
Files to Modify.

**Revised by OR-1.** That remains true of the MACHINERY, but phase 6 is protocol
work and carries the full obligation. Every gated requirement extracted for
rfc1035, rfc3765, rfc4486 and rfc5301 is proven by tagged tests
(`RFC requirement: <id> positive` and `... negative`), and any MUST/MUST NOT the
extraction finds enforced in Ze's code gains the
`// RFC NNNN Section X.Y: "<quoted requirement>"` comment directly above the
enforcing line, per this rule's "RFC MUST Comments" section. The summaries are
authored under `/ze-rfc`. Where a requirement turns out to be unenforced, the
answer is OR-1a's escalation, never a comment describing the gap.

Scope note: `ai/rules/rfc-compliance.md` forbids Ze-specific content inside
`rfc/short/` files. The four re-authored summaries stay pure protocol reference;
everything about what Ze does with those obligations lives in the tests, the
public row, and this spec.

## Freshness Re-verification (appended 2026-07-29, after children 1 and 2)

This spec was authored before its predecessors landed. Child 1 is committed
(`2b1f84827`, `cb9f72609`). Child 2 is complete and STAGED in the working tree
but not yet committed (`git diff --stat HEAD` shows
`scripts/dev/rfc_requirements.py +858`, `rfc_requirements_test.py +1014`,
`docs/features/rfc-status.md +4`, `rfc/enrolled.txt` 1 line, and
`rfc/audit/rfc7606.json`). `scripts/dev/rfc_requirements.py` has grown from
1,769 to **4,192** lines, so **every line number this spec cited into that module
was wrong**. The symbol name is the durable anchor. The numbers below are correct
as of this date and will move again.

`make ze-rfc-check` exits **0** on this tree (166 enrolled, 2,720 gated
requirements, 2,579 tags, **0** extraction sign-offs).

### Citation corrections -- `scripts/dev/rfc_requirements.py`

Every symbol still exists. Nothing this spec depends on was deleted or renamed.

| Spec cite | Symbol | Now at |
|-----------|--------|--------|
| `:55` | `STATUS_FILE` | `:63` |
| `:93-98` | `_FIRST_TAG_RE` + its ad-hoc-category-tag comment | comment `:99-106`, regex `:107` |
| `:331-334` | `parse_checklist_line`, "not a requirement, not an error" | func `:316`, comment `:348` |
| `:614-615` | `evaluate`'s non-gated skip (`if not req.gated`) | func `:960`, skip `:1002` |
| `:655`, `:675-684` | `check_enrolment`, and its source-text requirement | func `:1043`, source-text `:1078-1087` |
| `:688`, `:688-695` | `parse_enrolled` | `:1102-1109` |
| `:698`, `:698-711` | `_git_baseline_enrolment` | `:1112-1136` |
| `:1059`, `:1072-1080`, `:1083-1084`, `:1113-1121` | `check_new_summaries`, grandfather early return, source-has-MUSTs comparison | func `:1709`, early return `:1733`, comparison `:1763` |
| `:1128` | `parse_status_ledger` | `:1778` |
| `:1163-1208` | `check_status_agreement` | `:1813-1858` |
| `:1174-1176` | its loop + the non-enrolled exemption | `:1824-1826` |
| `:1176` | `req.rfc not in enrolled` exemption | `:1826` |
| `:1178`, `:1178-1184` | the `row is None` branch | `:1829-1834` |
| `:1188-1200` | the disclosure decision | `:1838-1850` |
| `:1323` | `source_keyword_count` | `:1973` |
| `:1340`, `:1340-1348` | `unconverted_summaries` + docstring | `:1990-2004` |
| `:1465` | `render_ledger` | `:2251` |
| `:1510-1515` | the byte-stability sorts | `:2291`, `:2298`, `:2303` (churn comment `:2301`) |
| `:1531` | the `captured = {r.rfc for r in requirements}` call site | `:2328` |
| `:1601`, `:1620-1625`, `:1624` | `_collect_for_check`, and the enrolment filter on parse errors | func `:3929`, filter `:3952`, amnesty docstring `:3937-3939` |
| `:1629`, `:1637`, `:1670-1673`, `:1683-1685` | `run_check`, the `check_enrolment` call, the stale amnesty comment, and the `STATUS_FILE` read | func `:3957`, call `:3990`, comment `:4036-4038`, read `:4049-4050` |
| `:1725-1741` | `run_check_fresh` | `:4127` |
| (new, child 1) | `_NO_GAP_RE` | `:170` |

### Citation corrections -- everything else

| Spec cite | What | Verdict |
|-----------|------|---------|
| `docs/features/rfc-status.md:28 / :56 / :123 / :234` | the four D1 rows | **WRONG: `:32` rfc4486, `:60` rfc3765, `:127` rfc5301, `:238` rfc1035** (+4 from child 2's preamble) |
| `rfc/short/rfc3765.md:70-73` | 4 `-x-`-anchored advisory rows | correct, unchanged |
| `rfc/short/rfc4486.md:55-56` | 2 `-x-`-anchored `[MAY]` rows | correct, unchanged |
| `rfc/short/rfc5301.md:124-129` | 4 section-anchored advisory + 2 `[FORMAT]` | correct, unchanged (`[FORMAT]` at `:124`, `:126`) |
| `rfc/short/rfc1035.md` no checklist line | | correct |
| `Makefile:437` / `:438` / `:442` | `ze-rfc-check` / `--selftest` / `ze-rfc-index` | all correct |
| `mk/inventory.mk:106` | `--check-fresh` | correct |
| `scripts/status/verify_run.go:237`, `:259` | `mk("ze-rfc-check")` in both modes | correct |
| `scripts/dev/commit_helper.py:512-523` | `STRUCTURAL_GATES`, eight names, no `ze-rfc-check` | **`:514-525`**. Eight names and the absence both confirmed, so OC-2 stands |
| `internal/core/dnsserver/handler.go:48` | `Authoritative`, and the header cites `rfc/short/rfc1035.md` at `:3` | correct |
| `ai/rules/rfc-compliance.md:28` | "The RFC requirement is not in `rfc/short/<stem>.md`" | correct |
| `ai/rules/rfc-compliance.md:38-51`, `:40` | the Ask-Thomas table, and "making Ze more conformant never needs permission" | heading `:36`, `:40` correct, table `:43-50` |
| `ai/rules/rfc-compliance.md:53` | "Every earlier answer ... is VOID" | correct |
| `ai/rules/rfc-compliance.md:56` | cited as VOIDing every annotation | **WRONG: `:56` is a `plan/learned` table row. Use `:53` for the VOID statement, `:57` for the `{gap}`/`{not-applicable}`/`partial` row** |
| `ai/rules/rfc-compliance.md:83` | the `rfc-editor.org` URL form | **WRONG: `:78` and `:149`** |
| `ai/INDEX.md:212` | ratchet count | reads "the five ratchets". See the struck claim above |
| `ai/INDEX.md:372` | the RFC keyword row | **WRONG: `:374`. `:372` is "extended message". A NEW extraction keyword row exists at `:375`** |
| `scripts/dev/rfc_requirements_test.py:33` `_patched`, `:51` `_run_capturing` | the wiring harness | **`:55` and `:77`** |
| `..._test.py:750` `TestStatusLedgerCrossCheck`, `:1754` `TestStatusDisclosureFailsClosed` | | **`:1176` and `:2644`** |
| `..._test.py:1337` `TestCoverageRatchetWiring`, `:1562` `TestRetiredRequirementsWiring` | | **`:1959` and `:2383`** (file is now 5,393 lines, 60 classes) |

### Re-measured figures (driven through the module's own functions)

Derivation: `load_enrolled`, `summary_stems`, `parse_status_ledger`,
`parse_summary_file`, `source_keyword_count`, `source_path`,
`unconverted_summaries`, `derive_inventory` over the working tree.

| Claim | Re-measured | Verdict |
|-------|-------------|---------|
| 175 summaries / 166 enrolled / 157 rows / 23 rows keying a non-enrolled stem | identical | HOLDS |
| **32 enrolled with no status row (AC-3's grandfather set)** | **32, and the same 32 stems** | **HOLDS.** Child 2 edited `rfc/enrolled.txt` (rfc7947's reason prose only, no stem added or removed) and `docs/features/rfc-status.md` (preamble only, no row added or removed), so neither ledger's membership moved |
| 9 un-enrolled summaries, all zero-gated | 9, same stems, all gated=0 | HOLDS |
| **4 stems with a public support claim over zero gated requirements (D8/A-3)** | **exactly 4: rfc1035 `Supported`, rfc3765 `Supported`, rfc4486 `Supported`, rfc5301 `Experimental`** | HOLDS |
| `rfc/full/rfc3765.txt` and `rfc/full/rfc4486.txt` ABSENT | `source_path` returns `None` for both | HOLDS -- step 4a's fetch is still a precondition |
| 539 `{gap}` across 84 RFCs, all rowed and enrolled (A-2) | 539 / 84 / 0 missing a row / 0 un-enrolled | HOLDS |
| 0 of 175 summaries fail to parse (A-5) | 0 | HOLDS |
| 7 advisory-only summaries, and the RE-AUTHOR table has 2 rows (D5) | 7 (`rfc3765 rfc4486 rfc5301 rfc7999 rfc8195 rfc8326 rfc9129`), and the table has 2 (`rfc1035`, `rfc6987`) | HOLDS -- D5 is still live at `:2328` |
| **60 rows spell a MUST-gap count, all 60 agree (A-4)** | **60 rows, 60 matches, 0 mismatches -- but ONLY under IMMEDIATE adjacency** | HOLDS with a load-bearing qualifier. See AC-12 below |
| `check_status_agreement` reaches for a row only when a `{gap}` exists | `:1826` `continue`s on no-annotation / non-gap / non-enrolled, `:1828` fetches the row after | HOLDS |
| `rfc/not-enrolled.txt` exists | does NOT exist | HOLDS -- AC-5..AC-9 still introduce it |
| `rfc/extraction/` holds artifacts | only `README.md`, and `extraction_stems()` is empty | 0 sign-offs, so AC-27's "grows by exactly four" is measured from 0 |

### Collisions with what landed

| # | Finding | Evidence | Effect on this spec |
|---|---------|----------|---------------------|
| C-1 | **AC-27 is achievable for rfc1035 and rfc5301, and correctly blocked for rfc3765/rfc4486.** `derive_register` (`:2606`) returns `prose` when there are zero capitalised keyword sites but non-zero lowercase ones, and `_SITE_PROSE_RE` (`:2408`) is case-insensitive. Measured: `derive_inventory("rfc1035", 0)` -> register `prose`, **31 sites over 73 sections**, and `("rfc5301", 0)` -> `prose`, 3 sites over 10 sections. For rfc3765/rfc4486 it returns `None` | `:2606-2617`, `:2651-2720`, probe | The pre-2119 worry in A-10 / R-7 does NOT block the sign-off. rfc1035 signs under `prose` (or weaker), and `:3268-3276` forbids claiming `rfc2119` over it. **31 sites is the first real size estimate for the rfc1035 walk** -- A-10 said the tail was unknowable, and it is now bounded from below |
| C-2 | **Step 4a's ordering is CONFIRMED by child 1's shipped code, almost verbatim.** `run_extract_skeleton` (`:3138`) exits 2 with "has no source text ... Fetch it ... before extracting: with no source there is no inventory to derive and no register to sign under". `evaluate_extractions` (`:3410-3416`) additionally errors on any committed artifact whose stem has no source | `:3138-3149`, `:3410-3416` | **OC-6 and the 4a->4d order hold unchanged.** rfc3765/rfc4486 are doubly bound: no skeleton, and no valid artifact |
| C-3 | **A-12 is materially incomplete: it enumerates 3 ratchets and there are now 6, plus a schedule.** Re-verified each against a new enrolment. `check_evidence_ratchet` (`:1599`) skips at `:1632` (`req.rfc not in baseline_enrolled`), and its docstring says "an RFC enrolled in this very commit is not accused". `check_extraction_ratchet` (`:3507`) folds only over `baseline - current` and `baseline & current`, so a NEW artifact cannot fire it. `check_drain_floor` (`:3844`) reads `rfc/drain-budget.txt` which ships `rate 0`, so `required_floor` (`:3803`) returns `min(170, ceil(0 * months)) == 0` | `:1632`, `:3507-3520`, `rfc/drain-budget.txt`, `:3803-3841` | **A-12's CONCLUSION survives: enrolling the four reds no ratchet.** Its basis must be widened from three ratchets to six plus the drain floor. And the drain floor's inertness must be noted as *depending on the owner not having armed a rate* |
| C-4 | **`check_status_agreement` can be made unconditional with no change to child 2.** `check_evidence_ratchet` takes `(requirements, tags, enrolled, baseline_evidence, baseline_enrolled)` and never reads status rows. Child 2's `rfc-status.md` edit is prose plus an HTML comment before the first table, and `parse_status_ledger` still yields 157 rows | `:1599-1605`, probe | No collision. Phase 4 proceeds as written |
| C-5 | **AC-12's spelled-number logic does NOT already exist** -- no spelled-number parsing anywhere in the module (`grep -i "nineteen\|twenty\|spelled"` finds only unrelated prose). It must be built | grep over `:1-4192` | AC-12 builds, does not reuse |
| C-6 | **A newer batch baseline reader exists and is the one to use.** `_git_cat_blobs` (`:1491`) reads many HEAD blobs in ONE `git cat-file --batch`, and its docstring makes the batch interface "a condition of the check being kept rather than an optimization" (per-file `git show` measured at +1.7s vs +0.5s) | `:1491-1502` | A-7's single extra `git show` is still within the Security Review's "only one additional `git show` per run", so it is legal. But `_git_cat_blobs` is the current idiom, and A-7 MUST name it |
| C-7 | Child 1's own in-file self-citations have already rotted the same way this spec's did. `check_enrolment`'s docstring cites `_git_baseline_enrolment:698` (now `:1112`) and `_git_baseline_summary_stems:763`. Meanwhile `_git_baseline_enrolment`'s docstring cites the same symbol as `:791` (now `:1188`), and `:2473` cites `source_keyword_count:1329` (now `:1973`) | `:1055`, `:1059`, `:1115`, `:2473` | Not this spec's to fix, but it confirms the class of rot is structural. Do not trust ANY intra-module line citation in this file |

### Claims that are now FALSE (not merely stale)

Each is struck in place above. They are collected here so an implementer cannot
miss them.

| Claim | Where | Reality |
|-------|-------|---------|
| "`ai/rules/rfc-compliance.md` 'the four ratchets' becomes six" | Integration Points, Files to Modify, RFC Documentation | Already **six** (`:115`). The edit is six -> **seven** |
| "`ai/INDEX.md:212` already stale ('the three ratchets'). Corrected here" | Integration Points, Files to Modify | Reads "the five ratchets". The string "three ratchets" does not exist |
| "the keyword row at `:372`" | Files to Modify | `:372` is the "extended message" row. The RFC row is `:374`, and a new extraction row is `:375` |
| The four `docs/features/rfc-status.md` row numbers | Task/D1 table, Files to Modify, Documentation row 9 | All +4: `:32`, `:60`, `:127`, `:238` |
| "`docs/features/rfc-status.md` gains a preamble paragraph" (OR-4/AC-25 as a NEW addition) | Behavior to change, Files to Modify, Deliverables | A preamble paragraph is **already there** (child 2). AC-25's properties are still missing, so AC-25 stands -- but the work is an EXTENSION, and the Deliverables' `sed -n '1,10p'` check is now a **false pass** |
| "`rfc/full/rfc5301.txt` carries 4 MUST-level keywords ... nothing has captured" | D5, Q-3, Files to Modify, phase 6 | All 4 hits are boilerplate from RFC 2119 on `rfc/full/rfc5301.txt:94`. `derive_inventory` reports `keyword_sites == 0`. There are **zero** uncaptured capitalised MUSTs in RFC 5301 |
| `ai/rules/rfc-compliance.md:56` VOIDs every annotation | Required Reading, R-2, Q-4, Key Design Decisions | `:56` is a table row about `plan/learned`. The VOID statement is `:53`, and the annotation row is `:57` |
| Every `scripts/dev/rfc_requirements.py:NNNN` and `rfc_requirements_test.py:NNNN` in this spec | throughout | See the two correction tables above |

### AC-12: a defect the re-measurement exposed (recorded, NOT changed)

A-4's 60/60 is TRUE, but only under a parser that requires the spelled number to
sit **immediately** before `MUST`/`SHALL` with nothing between. AC-12 as written
says "spells a MUST-gap count ... followed by `MUST` or `SHALL`", which does not
say that, and its Boundary Tests worry only about the compound tail
(`Twenty-five` != 5). Measured with a 40-character tolerance window instead of
strict adjacency, the committed page yields **four false mismatches**. The page
uses a **second, unrelated convention**. A spelled number immediately
before `MUST` is the **gap** count. A spelled number *near* `MUST` is often
the **`{not-applicable}`** count.

| Row | Text | Spelled | Real `{gap}` | Real `{not-applicable}` |
|-----|------|---------|--------------|-------------------------|
| rfc7432 | "Sixty-four further MUSTs bind PE roles ze does not..." | 64 | 15 | **64** |
| rfc8484 | "Six client-role and media-type-definer MUSTs are not-applicable" | 6 | 1 | **6** |
| rfc9012 | "Nine further MUSTs are annotated not-applicable" | 9 | 51 | **9** |
| rfc9830 | "Twelve further MUSTs are annotated not-applicable" | 12 | 20 | **12** |

Two further facts the boundary table does not cover. The primary gap count is
written in **digits** in two rows ("51 MUST-level gaps", "20 MUST-level gaps",
both agreeing with their real counts). And `Sixty-four` occurs on the page, which
is outside AC-12's stated `One`..`Thirty` range.

Consequence, stated rather than fixed: implemented as literally worded, AC-12
makes `TestGapCountAgreementRealFile` (A-4) **RED on landing** on those four rows.
The decision on how to word it is the owner's. There are two candidate readings.
Reading (a) requires strict adjacency, which reproduces 60/60 exactly. Reading (b)
parses both conventions and compares each against its own annotation kind.

### Verdict on the 4a-4d ordering

**It still holds, and child 1's shipped code strengthens it.** Fetch (4a) before
extract/sign (4b-4c) before enrol (4d) is now enforced by two independent
mechanisms rather than argued. `run_extract_skeleton:3138` refuses to emit a
skeleton without source text, and `check_enrolment:1078-1087` refuses the
enrolment itself. OC-1's arming-last constraint is unaffected -- nothing that
landed changes `ze-rfc-check`'s position in either verify mode
(`scripts/status/verify_run.go:237`, `:259`) or its absence from
`STRUCTURAL_GATES` (`scripts/dev/commit_helper.py:514-525`). AC-19 and AC-26 are
achievable. The tree is green today, and all six ratchets plus the drain floor were
individually cleared against a four-stem enrolment (C-3). The only new
per-commit obligation is the one AC-27 already names.

**One risk this spec's Risks table does not carry.** Child 1's sign-off does not
force a gated requirement. `_evaluate_extraction:3332` demands a gated target only
when the DERIVED quote contains a **capitalised** keyword, and rfc1035 and rfc5301
both have `keyword_sites == 0`. So an all-advisory re-authoring of either would
produce a **valid** sign-off and a **passing** `check_enrolment`. It would then fail
this spec's own AC-10 / AC-21, which require at least one gated row. The two gates
can disagree, and the walk is what has to settle it. This is the concrete form of
A-10 / OR-1b's stated unknown and belongs in the phase-6 scoping conversation.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-28 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`scripts/dev/rfc_requirements.py` wired into
      `run_check`), not helper-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every Owner Ruling discharged: OR-1 (four enrolled, sourced, re-authored,
      rows corrected), OR-1a (every new gated requirement proven or individually
      authorised, with the authorisation recorded in Owner Rulings), OR-1b (the
      rfc1035 tail reported and scoped with the owner), OR-2 (OC-1..OC-5 held at
      every commit), OR-3 (the deferral Reason carries the ordering dependency),
      OR-4 (the preamble paragraph is on the page)
- [ ] OC-1 verified against the COMMIT SEQUENCE, not only the final tree
- [ ] OC-6 verified against the COMMIT SEQUENCE: `plan/spec-rfcgate-1-extraction.md`
      is on HEAD before the first phase-6 enrolment commit, and four
      `rfc/extraction/*.json` sign-offs landed with the four enrolments (AC-27, AC-28)
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior (`--selftest` driven from `run_check`)
- [ ] Interop tests for protocol features (N-A: no wire behavior changes)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-rfcgate-4-ledger.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-rfcgate-4-ledger.md` only (commit A preserves the spec in history)

## AC-12 correction and one unrecorded risk (supervisor, 2026-07-29)

Both come from the spec-freshness review appended above. The first is a decision I
took. The second is a scoping question for the phase-6 conversation, recorded here
rather than left in an agent's report.

### AC-12 as written would red on landing. Narrowed, not dropped.

The review reproduced A-4's `60/60` and it is TRUE -- but only under IMMEDIATE
adjacency, which AC-12's wording never said. Given any tolerance window,
`docs/features/rfc-status.md` yields four false mismatches. The page uses a
SECOND convention: a spelled number NEAR the word MUST is often the
`{not-applicable}` count rather than the `{gap}` count. Measured: rfc7432 (64 vs
gap 15), rfc8484 (6 vs 1), rfc9012 (9 vs 51), rfc9830 (12 vs 20). Two further rows
spell the count in DIGITS, and `Sixty-four` sits outside AC-12's stated
`One..Thirty` range.

| Reading | Verdict |
|---------|---------|
| Spelled number immediately followed by `MUST`/`SHALL` | **CHOSEN.** Matches the measured 60/60 exactly, so the check lands green and every row it judges it judges correctly |
| Any spelled number within a tolerance window of `MUST`/`SHALL` | REJECTED. Four false mismatches on day one, each on a row that is honest. `ai/rules/rfc-compliance.md:114-116`: a check that reds on correct work gets deleted rather than obeyed |

→ Constraint: the digit-spelled rows and anything above `Thirty` are OUT of AC-12's
scope by this narrowing. That is a real coverage limit rather than a tidy-up.
State it in the gate's own message so a reader is not misled into thinking every
Remaining cell is checked. Widening it later means first normalising the page's two
conventions, which is editorial work on `docs/features/rfc-status.md`, not gate work.

→ This narrowing changes what AC-12 CHECKS, not what it MEANS: a spelled gap count
that disagrees with the summary still fails. I took it because the AC as written was
unachievable (`TestGapCountAgreementRealFile` red on landing) and its intent is
unambiguous. Flagged to Thomas rather than filed silently.

### Unrecorded risk: child 1's sign-off and child 4's AC-10 can disagree

Child 1's extraction sign-off does NOT force a stem to declare a gated requirement.
`_evaluate_extraction` (`scripts/dev/rfc_requirements.py:3332`) demands a gated
target only for a site carrying a CAPITALISED keyword, and both `rfc1035` and
`rfc5301` derive `keyword_sites == 0` (`_SITE_PROSE_RE` is case-insensitive, so
`derive_register` grades them `prose`: rfc1035 at 31 sites / 73 sections, rfc5301 at
3 / 10).

So an all-advisory re-authoring of either stem yields a VALID sign-off and a PASSING
`check_enrolment` (`:1043`). It then fails THIS spec's AC-10 and AC-21, which
require at least one gated MUST-level row. Two gates that both look green can
disagree about whether the same stem is adequately extracted. Only the actual
section-by-section walk settles which is right.

→ Constraint: this belongs in the phase-6 scoping conversation, before the walk, not
after. If a stem genuinely has no MUST-level obligation in the part Ze implements,
A-9 already provides the honest route (change the ledger claim, or record the walk as
a `manual-walk` register row saying why zero is correct). And `check_unproven_support`
must accept that evidenced form. What must NOT happen is a row invented to satisfy
AC-10 (`ai/rules/rfc-compliance.md`: a fabricated requirement is worse than a
declared absence).

→ Also corrected by the review, and worth carrying: `rfc5301` does NOT have four
uncaptured capitalised MUSTs. All four hits are boilerplate from RFC 2119 on a single
line (`rfc/full/rfc5301.txt:94`), so the premise behind that open question was not a
fact. D5 itself is unaffected.

### Step 4a is DONE (supervisor, 2026-07-29), and it sharpens the AC-10 tension

`rfc/full/rfc3765.txt` (395 lines) and `rfc/full/rfc4486.txt` (339 lines) are fetched
and verified as real RFC text, not error pages. `rfc3765.txt:8-9` reads
`Request for Comments: 3765 / Category: Informational`, and `rfc4486.txt:8`/`:14` read
`Request for Comments: 4486` / `Subcodes for BGP Cease Notification Message`
(Standards Track). Zero HTML markers in either. `make ze-rfc-check` stays exit 0
after the fetch, as expected. Both stems are unenrolled, so `source_keyword_count`
is not gated on them. Adding source text alone changes nothing the gate measures.

Done ahead of the rest of phase 6. It is the one step that is inert with
respect to the module and the index. It therefore proceeded while child 2's commit
was blocked. It also removes a network dependency from the critical path.

**What the fetch reveals, and phase 6 must not paper over:** `rfc3765` is
**Category: Informational**. An Informational RFC can legitimately carry few or no
MUST-level obligations, and `docs/features/rfc-status.md` publishes it as
`Supported`. That is precisely the collision recorded above. Child 1's sign-off
does not force a gated row, while this spec's AC-10 and AC-21 require one. And it
is now concrete rather than hypothetical for at least one of the four stems.

→ Constraint: if the walk finds `rfc3765` has no MUST-level obligation in the part Ze
implements, the answer is A-9's evidenced form (change the ledger claim, or record a
`manual-walk` register row stating why zero is correct, with `check_unproven_support`
accepting it). It is NEVER a MUST row invented to satisfy AC-10. A fabricated
requirement is worse than a declared absence: it puts a false claim inside the very
ledger this spec set exists to make honest.

→ `rfc4486` is Standards Track and titled for the `Cease` subcodes, so it is the
counterpart case and WILL carry real obligations. Note the overlap with
`plan/spec-fixit-bgp-shutdown-cease-notification.md`, which found that ze sends no
`Cease` on shutdown at all. RFC 4486's subcodes are the vocabulary that spec needs,
and its two currently-extracted requirements are both MAY (`rfc/short/rfc4486.md:55-56`).

## Owner Ruling OR-A (Thomas, 2026-07-29): rfc3765 enrols on an evidenced zero

**Decision: enrol `rfc3765` and record a `manual-walk` sign-off whose reason states
why zero gated obligations is the honest answer. Do NOT fabricate a MUST, and do NOT
soften the public claim.** Taken after the phase-6 extraction walk (step 4b) found
zero MUST-level obligations in the document.

**The evidence that makes zero honest** (from the walk, and re-verified here):

| Fact | Source |
|------|--------|
| Category is Informational | `rfc/full/rfc3765.txt:9` |
| No RFC 2119 boilerplate section exists at all | absent from `rfc/full/rfc3765.txt` |
| Zero occurrences of any of the ten RFC 2119 keywords | keyword scan of the full text |
| The RFC calls its own mechanism "an advisory qualification to readvertisement" | §2, repeated verbatim in §4 |
| It defines no wire format | NOPEER rides RFC 1997's COMMUNITIES attribute |

**Mechanically achievable, verified against child 1's SHIPPED derivation
(2026-07-29):** `derive_inventory("rfc3765", 0)` returns `register=prose`, 1 site,
11 sections. `manual-walk` is WEAKER than `prose`. Child 1 permits an artifact to
declare the derived register or a weaker one, and refuses only a STRONGER claim (its
AC-9, AC-31, AC-32). So the sign-off lands legally, and child 1 needs no change. For
comparison, measured the same way: `rfc1035` prose 31/73 with 26 gated, `rfc5301`
prose 3/10 with 7 gated, `rfc4486` **rfc2119** 1/11 with 1 gated.

→ Constraint: AC-21 takes a dated correction. Its "each declares at least one GATED
requirement" becomes "each declares at least one gated requirement, OR carries an
evidenced `manual-walk` sign-off whose reason establishes that zero is a property of
the document". `check_unproven_support` MUST accept that evidenced form, or it reds on
an honest claim, which is the `ai/rules/rfc-compliance.md:114-116` failure mode.

→ Constraint: this is NOT a general escape from AC-21. It is legal only with the
register derived, the walk performed, and the reason recorded -- three committed facts.
A stem that captured nothing does not qualify.

→ Constraint: the same reasoning does NOT transfer to the other three. `rfc1035` (26
gated), `rfc5301` (7) and `rfc4486` (1) all carry real MUST-level obligations. Each
needs a positive AND negative tagged test per gated row, or an escalation. Zero is
honest for exactly one of the four.

→ Note for AC-24: `docs/features/rfc-status.md` currently publishes rfc4486, rfc3765
and rfc1035 as `Supported` with "No tracked gap in current source anchors". That
phrase matches the gate's `_NO_GAP_RE`, so it will contradict any future `{gap}` on
these stems. RFC 5301's `Experimental` is not a defect: that column is a product
support view, not the RFC's IETF category (5301 is Standards Track).

## Owner Rulings OR-B and OR-C (Thomas, 2026-07-30): both stems get full compliance

The phase-6 walk enrolled two of the four stems and was blocked on the other two.
`rfc1035` and `rfc5301` carry real MUST-level obligations that the code does not
meet, and the implementer had no authority to annotate them. Both went to Thomas
with the requirement text, the producing `file:line`, and the cost. He ruled on each.

### OR-B: rfc5301, all three unmet MUSTs are fixed, rejecting at config time

| Element | Decision |
|---------|----------|
| Scope | All three. Not the wire defect alone |
| Where | Reject non-conforming input at config time, never sanitise at emit (`ai/rules/exact-or-reject.md`) |
| Cost he accepted | An operator with a non-ASCII hostname configured today gets a validation error on their next commit. That is the honest failure |
| Home | `plan/spec-fixit-isis-hostname-ascii.md` |

The live defect: `internal/plugins/isis/lsdb/encode.go:60` writes a bare
`[]byte(name)`. The YANG leaf (`internal/plugins/isis/yang/ze-isis-conf.yang:69`)
carries only `length "1..255"` with no pattern, so a UTF-8 hostname reaches a peer as
8-bit octets. Display is sanitised at `internal/plugins/isis/show.go:125`. Emit is not,
which is why this stayed invisible.

→ Correction the fixit spec establishes, recorded here so it is not lost: RFC 2181
Section 11 says implementations "must not place any restrictions on the labels that can
be used". So `RFC5301-3-9` is a rule about label and name LENGTHS, not about the
character set. An LDH pattern would reject strings RFC 5301 explicitly permits. The
7-bit ASCII constraint comes from `RFC5301-3-7` alone.

### OR-C: rfc1035, full compliance including zone transfer

| Element | Decision |
|---------|----------|
| Scope | FULL. Every extracted MUST gets a real code path and both polarities |
| Includes | AXFR and IXFR, which Ze does not have today. This ADDS a capability rather than only repairing one, and he chose it knowingly |
| Rejected | The narrower option that would have scoped out the roles Ze does not play and recorded the positive-only rows under his authorisation |
| Home | `plan/spec-fixit-dns-rfc1035-conformance.md` |

Six obligations had no code path at all:

- the 512-byte UDP limit and truncation with the TC bit (`internal/core/dnsserver/handler.go:62` does no size accounting, and no
  production call to `Msg.Truncate` exists)
- the TTL maximum rule (`internal/plugins/geodns/server.go:106` emits `rec.TTL` verbatim)
- zone refresh over virtual circuits
- the NOTIMP response to an inverse query

### What these rulings do to AC-22 and AC-23

~~AC-22 requires all four stems in `rfc/enrolled.txt` and a declared remainder of
exactly five.~~ **Corrected 2026-07-30.** Two stems enrol now: `rfc4486` with its one
MUST proven in both polarities, and `rfc3765` on the OR-A evidenced zero. Two stems
stay in `rfc/not-enrolled.txt` declared `backlog`, and the declared remainder is
therefore seven rather than five. Their enrolment is an acceptance criterion of the
two fixit specs above, not of this one.

~~AC-23 requires every newly extracted gated requirement to carry a tagged pair or an
authorised annotation.~~ **Corrected 2026-07-30.** Of the 34 gated rows extracted, one
is proven in both polarities and 33 are owed by the two fixit specs. **Zero are
annotated**, which is the outcome the rule wants: `ai/rules/rfc-compliance.md:52` voids
`{gap}` and `{not-applicable}` as authority, and no implementer wrote one.

→ Constraint, and the honest cost of these corrections: `docs/features/rfc-status.md`
still publishes `rfc1035` as `Supported` and `rfc5301` as `Experimental` while their
MUSTs are unproven. `check_unproven_support` does NOT catch this, because it fires only
on a summary declaring ZERO gated requirements, and both now declare real ones. This is
blind spot 5 one level in: the claim is no longer backed by nothing, but it is not
backed by proof either.

→ Constraint: what changed is that the debt is now DECLARED, counted and visible.
`rfc/not-enrolled.txt` renders it as DEBT, the deferral shard names the destination
spec for each, and two owner rulings commit to fixing both fully. That is a weaker
statement than "conformant" and a much stronger one than the silence this spec set
started from. Do not let a green gate imply otherwise.

→ Constraint: closing this spec does NOT discharge the debt. The umbrella is machinery
only (D4). A future reader who finds `rfc1035` unenrolled MUST read the two fixit
specs, not conclude the extraction was abandoned.

→ Clarification (2026-07-30, correcting the supervising session): `{single-polarity}`
is NOT void. `ai/rules/rfc-compliance.md` voids `{gap}`, `{not-applicable}` and
`partial`. The string `single-polarity` does not appear in that rule at all. It is a
first-class gate annotation, defined in `ANNOTATION_KINDS`
(`scripts/dev/rfc_requirements.py:111`), validated at `:273-285`, documented at
`rfc/enrolled.txt:8`, and already carried by about twenty enrolled RFCs. It still
proves less than a tagged pair, so it needs Thomas's authorisation under AC-23 like any
annotation.

Recorded because the escalation above was framed as though no annotation
route existed for the `rfc1035` rows. Their negative polarity is unreachable while
`github.com/miekg/dns` owns the encoding. That route DOES exist, with his sign-off, and
`plan/spec-fixit-dns-rfc1035-conformance.md` states it accurately.

---

## Implementation Summary

### What Was Implemented

Five guards on the public ledger's edges, all wired into `run_check`
(`scripts/dev/rfc_requirements.py:6174`).

- **Status-row completeness ratchet** (`check_status_completeness:2218`, wired
  `:6291`). A newly enrolled stem must bring a row. An existing row must not
  vanish while its RFC stays enrolled. Both halves judge nothing on a degraded
  baseline (`:2254`).
- **Summary disposition partition** (`check_summary_disposition:2118`, wired
  `:6286`), backed by the new file `rfc/not-enrolled.txt`. Every summary is
  enrolled or declared. Four branches: undeclared (`:2152`), in both files
  (`:2160`), stale row (`:2167`), and discharge by anything but enrolment
  (`:2207`).
- **Unproven-support guard** (`check_unproven_support:2269`, wired `:6299`). A
  public support claim cannot rest on a checklist with zero MUST-level rows.
  `status_is_a_support_claim:2038` treats every Status except `Unsupported` and
  `Future` as a claim, so an empty cell fails closed.
- **Gap-count cross-check** (`check_gap_count_agreement:2441`, wired `:6308`).
  A spelled number immediately before MUST or SHALL must equal the real `{gap}`
  count.
- **Narrowed `{gap}` disclosure exemption** (`check_status_agreement:1999`).
  Enrolment now gates only the missing-row branch.

Two derived tables were added to the ledger (`_render_status_backlog:4293`,
called from `render_ledger` at `:4466`). Parse errors are no longer filtered by
enrolment.

Phase 6 executed Owner Ruling OR-1 over the four D1 stems. Two enrolled
(`rfc/enrolled.txt:209-210`). Two are declared `backlog` in
`rfc/not-enrolled.txt` and routed to fixit specs by OR-B and OR-C.

### Bugs Found/Fixed

Five defects surfaced during the independent review. All five are recorded in
the Review Gate table below with the test that now covers each one.

### Documentation Updates

- `docs/features/rfc-status.md:9-19` gained the preamble Owner Ruling OR-4
  requires (AC-25). It names the four machine-checked properties, names Status,
  Area and Implemented coverage as editorial, and states two coverage limits.
  Source anchor at `:20`.
- The four D1 rows were corrected (AC-24). Measured on the committed page:
  `rfc1035` Partial, `rfc3765` Supported, `rfc4486` Supported, `rfc5301`
  Partial.
- `rfc/not-enrolled.txt` carries its own format documentation, including the
  closed kind set and why `non-normative` is judged twice.
- `ai/rules/rfc-compliance.md:115` already read "the six ratchets", so no edit
  was owed there. Recorded at `:1075` of this spec.

### Deviations from Plan

| # | Planned | Delivered | Why |
|---|---------|-----------|-----|
| D-a | AC-22: all four stems enrolled, declared remainder of five | Two enrolled, remainder of seven | Owner Rulings OR-B and OR-C. `rfc1035` and `rfc5301` carry MUSTs the code does not meet, and the implementer had no authority to annotate them |
| D-b | AC-23: 34 gated rows, one proven, 33 owed | 35 gated rows, one proven, 34 owed | Arithmetic error in the correction text. Measured by driving `_collect_for_check`: rfc1035 27, rfc3765 0, rfc4486 1, rfc5301 7. The load-bearing half of the claim holds. Zero rows are annotated |
| D-c | AC-12 over `One..Thirty` within a tolerance window | Spelled number immediately before MUST or SHALL, `One..Ninety-nine` | Recorded as the AC-12 correction at `:1305`. A window reds four honest rows on day one |
| D-d | AC-14: a flat rejection of Ze-owes-nothing reasons | Two-part rule: the blacklist plus a positive citation requirement | The review proved the blacklist accepted seven evasions. Recorded in the refined AC-14 text at `:537` |
| D-e | AC-21: each of the four declares at least one gated requirement | `rfc3765` declares zero, on an evidenced `manual-walk` sign-off | Owner Ruling OR-A at `:1397`. RFC 3765 is Informational and invokes RFC 2119 nowhere |
| D-f | OC-4: the spec cannot close while any of the four is declared | Two close declared, routed to fixit specs | Superseded by OR-B and OR-C, which are the later rulings. `test_the_declared_remainder_is_debt_not_a_decision` still carries the old OC-4 sentence in its docstring. Not corrected here: the review-gate artifact pins that file's hash |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-9 assumed both missing sources were fetchable and authoritative | True for both. `rfc/full/rfc3765.txt` is Informational, `rfc/full/rfc4486.txt` is Standards Track | The fetch itself, step 4a, recorded at `:1364` | A-9 confirmed |
| assumption | A-4 read the page as 60/60 agreement over any tolerance window | The 60/60 holds only under immediate adjacency. A window yields four false mismatches | The spec-freshness review, recorded at `:1311` | AC-12 narrowed. Deviation D-c |
| approach | D5's premise said `rfc5301` had four uncaptured capitalised MUSTs | All four hits are RFC 2119 boilerplate on one line, `rfc/full/rfc5301.txt:94` | Child 1's own derivation returns `keyword_sites == 0` | Corrected in place at `:119`. D5 itself is unaffected |
| approach | OR-A's escape was believed to establish that zero is a property of the document | Three of its four facts describe the artifact. `manual-walk` is the weakest grade, so any stem can assert it | Independent review | The derived grade is now the fourth fact. `check_unproven_support:2347` |
| escalation | The correction text at `:1499` said the two unenrolled stems publish `Supported` and `Experimental` | Both publish `Partial`. The rows were corrected under AC-24 | This closure, driving `parse_status_ledger` over the committed page | Recorded here. The substance holds: `Partial` is still a support claim, so both still publish a claim whose MUSTs are unproven |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| D1: a public support claim cannot stand over an empty checklist | Done | `check_unproven_support:2269`, wired `:6299` | Two evidenced escapes, never an assertion |
| D2: an enrolled RFC with no row is caught | Done | `check_status_completeness:2218`, wired `:6291` | Git-HEAD ratchet. The 32 stay grandfathered and rendered |
| D3: the un-enrolled remainder is a decision | Done | `check_summary_disposition:2118`, wired `:6286`, plus `rfc/not-enrolled.txt` | Seven declared stems, three kinds |
| D4: the stale comment guarding a dead branch is gone | Done | `run_check:6266` extends every parse error | Measured: 0 of 175 summaries fail to parse |
| D5: one advisory row no longer buys immunity | Done | `render_ledger:4350` narrows the captured set to gated | The table now lists six stems, not two |
| D6: the gap count is cross-checked | Done | `check_gap_count_agreement:2441`, wired `:6308` | Never generated. The count is a fact, the classification is not |
| OR-1 second goal: the four claims become true | Changed | `rfc/enrolled.txt:209-210`, `rfc/not-enrolled.txt` | Two made true, two made declared debt. OR-B and OR-C |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `check_status_completeness:2244-2253`, and `TestStatusCompletenessWiring.test_run_check_fails_when_a_new_enrolment_has_no_row` (`rfc_requirements_test.py:8638`) | Drives `run_check`, not the helper |
| AC-2 | Done | `:2256-2265`, and `..._wiring.test_run_check_fails_when_a_row_is_deleted_under_enrolment` (`:8643`) | |
| AC-3 | Done | `:2244` folds over `newly_enrolled` only | Live tree exit 0 with 32 rowless enrolments present |
| AC-4 | Done | `:2254-2255` early return, and `_git_baseline_status_rows:1235` | Reports nothing, and does not mark the run clean |
| AC-5 | Done | `:2152-2159`, and `TestSummaryDispositionWiring.test_run_check_fails_on_a_summary_that_is_neither_enrolled_nor_declared` (`:8409`) | |
| AC-6 | Done | `:2160-2166` | Rejected, never resolved by precedence |
| AC-7 | Done | `:2167-2172` | |
| AC-8 | Done | `:2207-2214` | Scoped to stems still in the tree, so a summary can be retired |
| AC-9 | Done | `parse_dispositions:1153`, `DISPOSITION_KINDS` | Kind and reason both required |
| AC-10 | Done | `check_unproven_support:2269`, `status_is_a_support_claim:2038`, and `TestUnprovenSupportWiring.test_run_check_fails_on_support_claim_over_zero_gated_requirements` (`:8860`) | An empty Status cell is a claim |
| AC-11 | Done | `:2334-2335` | |
| AC-12 | Changed | `_GAP_COUNT_RE:2422`, `spelled_gap_count:2427`, `check_gap_count_agreement:2441`, and `TestGapCountWiring...` (`:9118`) | Narrowed to immediate adjacency. Deviation D-c. `Twenty-five` parses as 25 |
| AC-13 | Done | `ai/RFC-REQUIREMENTS.md:5761-5773` | Six stems listed. Lowercase column present. `rfc6987` and `rfc8195` read UNDECIDED, not "consistent" |
| AC-14 | Changed | `_NON_APPLICABILITY_RE:2064`, `non_normative_reason_cites_the_document:2094`, wired `:2189` | Two-part rule. Deviation D-d |
| AC-15 | Done | `check_status_agreement:1999-2004` | |
| AC-16 | Done | `:1999-2000` continue | |
| AC-17 | Done | `run_check:6266` | Measured 0 parse errors, so the change is a no-op today by design |
| AC-18 | Done | `_render_status_backlog:4293`, called `:4466`. Tables at `ai/RFC-REQUIREMENTS.md:5708`, `:5747`, `:5761` | `TestLedgerBacklogTables` (`:9256`) asserts all three |
| AC-19 | Done | `make ze-rfc-check` exit 0, and `--selftest` 665 tests OK | Selftest re-run in this closure |
| AC-20 | Done | `rfc/full/rfc3765.txt` and `rfc/full/rfc4486.txt` present. Both in `rfc/enrolled.txt:209-210` | |
| AC-21 | Changed | Measured gated rows: rfc1035 27, rfc3765 0, rfc4486 1, rfc5301 7. No `-x-` anchor remains | `rfc3765` carries the OR-A evidenced zero. Deviation D-e |
| AC-22 | Changed | `rfc/enrolled.txt:209-210`. `rfc/not-enrolled.txt` holds seven stems | Two enrolled, not four. Deviation D-a, OR-B and OR-C |
| AC-23 | Changed | 35 gated extracted, 1 proven both polarities (`RFC4486-4-1`), 34 owed, **0 annotated** | Deviation D-b corrects 34/33 to 35/34. Zero annotations is the criterion that held |
| AC-24 | Done | `parse_status_ledger` over the committed page | rfc1035 Partial, rfc3765 Supported, rfc4486 Supported, rfc5301 Partial |
| AC-25 | Done | `docs/features/rfc-status.md:9-19`, anchor at `:20` | Four checked properties named, three editorial columns named |
| AC-26 | Pending | The arming body and the enrolments are in one uncommitted change set | OC-1 is satisfiable in commit A. Verifiable only once the commits exist |
| AC-27 | Done | `rfc/extraction/` holds four artifacts. Registers: rfc4486 `rfc2119`, rfc3765 `manual-walk`, rfc1035 `prose`, rfc5301 `prose` | The two enrolled stems each carry one. Two more are pre-signed for the fixit specs |
| AC-28 | Done | Child 1 is on HEAD (`2b1f84827`, `cb9f72609`, 2026-07-29). Phase 6 is uncommitted | Child 1 precedes every phase-6 enrolment by construction |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestStatusCompletenessWiring` | Done | `rfc_requirements_test.py:8637` | |
| `TestSummaryDispositionWiring` | Done | `:8406` | |
| `TestUnprovenSupportWiring` | Done | `:8859` | |
| `TestGapCountWiring` | Done | `:9117` | |
| `TestGapDisclosureWiring` | Done | `:9184` | |
| `TestParseErrorReportingWiring` | Done | `:9240` | |
| `TestLedgerBacklogTables` | Changed | `:9256` | Split into `test_enrolled_without_row_table_rendered` and `test_disposition_table_rendered`, plus two more |
| `TestFourStemEnrolmentRealTree` | Changed | `:9401` | `test_all_four_are_enrolled_and_sourced` became `test_all_four_are_sourced` plus `test_rfc3765_is_enrolled`, because only two enrol |
| `TestGapCountAgreementRealFile` | Done | `:9054` | Plus `test_the_unjudged_rows_are_the_seven_the_docstring_names` (`:9076`) |
| `TestStatusCompleteness`, `TestUnprovenSupport` | Done | `:8546`, `:8655` | The unit-level twins the umbrella names |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/rfc_requirements.py` | Done | 6,488 lines. Modified |
| `scripts/dev/rfc_requirements_test.py` | Done | 9,583 lines. Modified |
| `rfc/not-enrolled.txt` | Done | New. Seven declared stems |
| `rfc/full/rfc3765.txt`, `rfc/full/rfc4486.txt` | Done | New. Fetched in step 4a |
| `rfc/short/rfc1035.md`, `rfc3765.md`, `rfc4486.md`, `rfc5301.md` | Done | Re-authored |
| `rfc/extraction/rfc1035.json`, `rfc3765.json`, `rfc4486.json`, `rfc5301.json` | Done | New. Four sign-offs |
| `rfc/enrolled.txt` | Done | Two stems added |
| `docs/features/rfc-status.md` | Done | Preamble plus four corrected rows |
| `ai/RFC-REQUIREMENTS.md` | Done | Regenerated. Three new tables |
| `test/plugin/prefix-maximum-enforce.ci` | Done | Gained the `RFC4486-4-1 positive` tag at `:6` |

### Audit Summary

- **Total items:** 28 acceptance criteria
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5 (AC-12, AC-14, AC-21, AC-22, AC-23), each with an owner ruling
  or a recorded supervisor decision behind it
- **Pending:** 1 (AC-26, verifiable only against commits that do not exist yet)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every enrolled RFC discloses | functional (gate drive) | `TestStatusCompletenessWiring` both methods (`:8638`, `:8643`) drive `run_check` to exit 2. Live tree: 168 enrolled, gate exit 0 |
| Every summary is a declared decision | functional (gate drive) | `TestSummaryDispositionWiring.test_run_check_fails_on_a_summary_that_is_neither_enrolled_nor_declared` (`:8409`). `rfc/not-enrolled.txt` declares seven, and 168 + 7 = 175 summaries |
| A support claim cannot rest on an empty checklist | functional (gate drive) | `TestUnprovenSupportWiring` (`:8860`). Armed and green: measured zero stems with a support-claim row over zero gated rows |
| The gap count stops being correct by luck | functional (real file) | `TestGapCountAgreementRealFile` (`:9054`) over the committed page. 60 rows judged, all agree. Seven unjudged rows named in `test_the_unjudged_rows_are_the_seven_the_docstring_names` (`:9076`) |
| The four D1 claims become true, not merely consistent | mixed | Two enrolled with sign-offs. `RFC4486-4-1` proven in both polarities (`session_prefix_test.py` twins at `:89` and `:114`, plus `test/plugin/prefix-maximum-enforce.ci:6`). Two declared DEBT with 34 owed rows and zero annotations |
| Discrimination: the guards are not vacuous | mutation | Review artifact records eight mutations with zero survivors, including both `RFC4486-4-1` polarities against the producer |

Honest limit on the fifth row. `rfc1035` and `rfc5301` still publish `Partial`,
which `status_is_a_support_claim:2038` counts as a claim. Their MUSTs are
unproven. `check_unproven_support` cannot see this, because it fires only on a
summary declaring zero gated rows, and both now declare real ones. The debt is
declared, counted and owned. It is not discharged.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Write the 32 missing `docs/features/rfc-status.md` rows (2026-07-29, OR-3) | deferred | `plan/spec-followup-rfc-enrollment.md`, which exists. Unchanged: the row already names a spec that outlives this one |
| Enrol `rfc1035` (2026-07-30, OR-1, OR-1a, OR-1b) | deferred | **Re-homed** to `plan/spec-fixit-dns-rfc1035-conformance.md`, the spec OR-C names. The old Destination was this spec's umbrella, which closes in the same change set |
| Enrol `rfc5301` (2026-07-30, OR-1, OR-1a) | deferred | **Re-homed** to `plan/spec-fixit-isis-hostname-ascii.md`, the spec OR-B names. Same reason |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfcgate-4-ledger-6aa27893-1bd2-42e1-9e68-879943aa8740.md` |
| `review_gate.py check` | clean (`review_gate: OK`, hashes match, re-run in this closure) |
| Reviewer lenses used | One full adversarial pass plus supervisor verification of every fix. Lenses: sign-off artifact validity against child 1's own check, independent re-derivation of AC-12's 60/60, and mutation verification of both `RFC4486-4-1` polarities |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The OR-A escape did not check that zero is a property of the document. `manual-walk` is universally assertable, so an `rfc2119`-graded source escaped through an artifact that excluded every site | `check_unproven_support` | The derived grade became the fourth fact (`:2347`). A refusal gets its own message, because telling an author to write the sign-off they already wrote is a dead end |
| 2 | ISSUE | Two Status words contradicted their own Remaining cells by the page's own glossary | `docs/features/rfc-status.md` | Both rows corrected to `Partial` |
| 3 | ISSUE | `test/plugin/prefix-maximum-enforce.ci` pins the exact NOTIFICATION wire bytes and was cited as proof, but carried no tag. The gate credited unit evidence only | `test/plugin/prefix-maximum-enforce.ci` | The `RFC requirement: RFC4486-4-1 positive` tag at `:6`. Measured effect: `functional/verify` moved 6 to 7 |
| 4 | ISSUE | AC-14's stated rejection rule was a six-phrase blacklist. Seven rephrasings of the same laundering were accepted, including "Ze is not required to do any of this" | `_NON_APPLICABILITY_RE:2064` | `non_normative_reason_cites_the_document:2094`, a positive citation requirement that fails closed, wired at `:2189` |
| 5 | ISSUE | A declared summary had no legal exit from the tree. Keeping the row fired the stale-disposition branch, deleting it fired the left-without-enrolling branch | `check_summary_disposition` | The AC-8 branch is scoped to stems still in the tree (`:2207`), so deleting the summary and the row together is the third discharge |

## Pre-Commit Verification

Independently re-verified in this closure session. Every command was run now,
not copied from the audit.

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `rfc/not-enrolled.txt` | yes | `git status --porcelain` reports `?? rfc/not-enrolled.txt`. Seven declared stems read back through `load_dispositions` |
| `rfc/extraction/rfc1035.json`, `rfc3765.json`, `rfc4486.json`, `rfc5301.json` | yes | `ls rfc/extraction/` returns all four plus `README.md`. Registers read back as `prose`, `manual-walk`, `rfc2119`, `prose` |
| `rfc/full/rfc3765.txt`, `rfc/full/rfc4486.txt` | yes | Both open through `check_enrolment`, which refuses an enrolment without source text. Both stems are enrolled |
| `scripts/dev/rfc_requirements.py` | yes | `wc -l` reports 6,488 lines |
| `scripts/dev/rfc_requirements_test.py` | yes | `wc -l` reports 9,583 lines |
| `test/plugin/prefix-maximum-enforce.ci` | yes | `grep -n 'RFC requirement'` returns the tag at `:6` |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | Both ratchet halves drive `run_check` | `grep -n '^class TestStatusCompletenessWiring'` returns `:8637`, and the two methods at `:8638` and `:8643`. Both pass inside the 665-test selftest |
| AC-3 | The 32 are grandfathered and the tree is green | `load_enrolled()` returns 168 stems. `parse_status_ledger` leaves enrolled stems without a row. Gate exit 0 |
| AC-5..AC-9 | The four disposition branches and the kind check | `grep -n 'def check_summary_disposition'` returns `:2118`. Branches read at `:2152`, `:2160`, `:2167`, `:2207`. `TestSummaryDispositionWiring` at `:8406` |
| AC-10, AC-11 | The guard is armed, and the non-normative escape works | `grep -n 'def check_unproven_support'` returns `:2269`, wired at `:6299`. Driving `check_unproven_support` over the live tree returns zero errors |
| AC-12 | Immediate adjacency, and `Twenty-five` is 25 | `SPELLED_NUMBERS['twenty-five']` is 25. `_GAP_COUNT_RE` at `:2422` requires `\s+(?:MUST|SHALL)s?`. `TestGapCountAgreementRealFile` passes |
| AC-13 | The pre-2119 caveat and the lowercase column | `ai/RFC-REQUIREMENTS.md:5761-5773`. `rfc6987` reads `0 | 2 | **UNDECIDED**`, never "consistent" |
| AC-17 | Parse errors are unfiltered | `_collect_for_check()` returns 0 parse errors over 175 summaries. `TestParseErrorReportingWiring` at `:9240` |
| AC-18 | The three tables render | `grep -n` on `ai/RFC-REQUIREMENTS.md` returns `:5708` "Enrolled without a public status row", `:5747` "Declared not enrolled", `:5761` "Summaries declaring no MUST-level requirement" |
| AC-19 | The gate and the selftest are green | `python3 scripts/dev/rfc_requirements.py --selftest`: **Ran 665 tests, OK** |
| AC-20, AC-27 | Sources present, four sign-offs | `rfc/enrolled.txt:209-210`. `ls rfc/extraction/*.json` returns four |
| AC-21 | Gated counts and no stale anchors | Driving `parse_summary_file`: rfc1035 27 gated of 33, rfc3765 0 of 2, rfc4486 1 of 11, rfc5301 7 of 15. `TestFourStemEnrolmentRealTree` asserts no `-x-` anchor and no ticked checkbox |
| AC-22 | Two enrolled, seven declared | `load_enrolled()` contains rfc3765 and rfc4486, and not rfc1035 or rfc5301. `load_dispositions()` returns seven stems |
| AC-23 | 35 extracted, 1 proven, 34 owed, 0 annotated | Folding tag polarity per requirement: `RFC4486-4-1` holds positive and negative. All 27 rfc1035 rows and all 7 rfc5301 rows are untagged. Annotated count is 0 for all four stems |
| AC-24 | The four rows match reality | `parse_status_ledger` returns Status `Partial`, `Supported`, `Supported`, `Partial` |
| AC-25 | The preamble is on the page | `docs/features/rfc-status.md:9` opens "What on this page a machine checks, and what stays a human judgement", then four bullets and the editorial paragraph at `:17` |
| AC-28 | Child 1 precedes phase 6 | `git log -1` confirms `2b1f84827` and `cb9f72609` dated 2026-07-29. `git status --porcelain` shows every phase-6 file still uncommitted |

### Wiring Verified (end-to-end)

No `.ci` test drives the machinery: it adds no daemon code, so its user entry
point is `make ze-rfc-check`. The one `.ci` in this change proves a protocol
obligation, not a gate.

| Entry Point | Test file | Verified |
|-------------|-----------|----------|
| `make ze-rfc-check` to the completeness ratchet | `rfc_requirements_test.py:8637` | yes. `_LedgerEdgeDrive` patches inputs and calls `run_check`, so a helper-only pass is impossible |
| `make ze-rfc-check` to disposition completeness | `:8406` | yes, same harness |
| `make ze-rfc-check` to the unproven-support guard | `:8859` | yes, same harness |
| `make ze-rfc-check` to the gap-count cross-check | `:9117` | yes, same harness |
| `make ze-rfc-check` to the narrowed exemption | `:9184` | yes, same harness |
| `make ze-rfc-check` to unfiltered parse errors | `:9240` | yes, same harness |
| `make ze-rfc-index` to the derived tables | `:9256` | yes. Calls `render_ledger` and asserts all three headings |
| Real `rfc/` tree, the two enrolments | `:9401` | yes. Deliberately unpatched, so a synthetic fixture cannot pass it |
| `ze` prefix-limit teardown to a NOTIFICATION with error code 6 | `test/plugin/prefix-maximum-enforce.ci` | yes. Read the file: it pins error code 06, subcode 01, and the AFI/SAFI/count Data field, and now carries the `RFC4486-4-1 positive` tag |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The rowless-enrolment set is derived, never listed. The rendered table reports the live count and the gate is green on it |
| A-2 | confirmed | Narrowing the exemption is a no-op today. Gate exit 0 with the narrowed branch at `:1999` |
| A-3 | confirmed | Exactly four stems carried a support claim over zero gated rows. No fifth stem entered scope |
| A-4 | **broken, then narrowed** | 60/60 holds only under immediate adjacency. Mistake Log row 2, Deviation D-c, AC-12 correction at `:1311` |
| A-5 | confirmed | `_collect_for_check()` returns 0 parse errors over 175 summaries |
| A-6 | confirmed | `parse_dispositions:1153` reads the file. The kind column needed its own grammar, which is why `parse_enrolled` was not reused verbatim |
| A-7 | confirmed | `_git_baseline_status_rows:1235` tolerates failure and returns None, and `:2254` returns early on it |
| A-8 | confirmed | `TestLedgerBacklogTables` renders and compares. `check_ledger_fresh` is green on the committed ledger |
| A-9 | confirmed | Both texts fetched and verified as real RFC text. Recorded at `:1364` |
| A-10 | confirmed as unknowable, then measured | The walk found 27 gated rows across 73 sections, and one obligation outside the summary's declared scope. Reported to the owner, who ruled OR-C |
| A-11 | confirmed | Zero test tags named an id of any of the four before the re-anchor. `test_no_id_a_test_references_was_lost` now guards the invariant that matters |
| A-12 | confirmed | No ratchet fired on the two enrolments. Gate exit 0 |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| The preamble names four machine-checked properties | Compared each bullet against its function: `check_status_completeness:2218`, `check_gap_count_agreement:2441`, `check_status_agreement:1974`, `check_summary_disposition:2118` | yes. Four bullets, four functions |
| The preamble names the coverage limits | The digit-count and separated-count limits match `check_gap_count_agreement`'s docstring at `:2455`, and the no-summary limit matches `check_unproven_support:2319` | yes |
| `rfc3765` row: Informational, advisory, no wire format | `rfc/full/rfc3765.txt:9` reads `Category: Informational`. Both checklist rows are `[MAY]` | yes |
| `rfc4486` row: one MUST, both polarities | `internal/component/bgp/reactor/session_prefix.go:399` decides, `:448` builds with `message.NotifyCease` and `message.NotifyCeaseMaxPrefixes` | yes |
| `rfc1035` and `rfc5301` Remaining cells disclose the unmet obligations | Compared against `internal/core/dnsserver/handler.go:62`, `internal/plugins/geodns/server.go:106`, `internal/plugins/isis/lsdb/encode.go:60`, `internal/plugins/isis/yang/ze-isis-conf.yang:69` | yes |
| `ai/rules/rfc-compliance.md` ratchet count | `:115` already reads "the six ratchets" | yes, no edit owed |
| No `<!-- source: -->` anchor sits inside a fenced block | The preamble anchor is at `:20`, after the paragraph and outside every fence | yes |
