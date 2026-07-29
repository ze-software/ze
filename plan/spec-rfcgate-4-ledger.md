# Spec: rfcgate-4 -- the public status ledger's edges

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | spec-rfcgate-1-extraction (BLOCKING, see OC-6); the umbrella also orders 2 and 3 ahead of this spec |
| Phase | - |
| Deferral shard | `plan/deferrals/rfcgate-4-ledger.md` |
| Updated | 2026-07-29 |

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
therefore "captured" and never appears in the "Summaries declaring no MUST-level
requirement" table, even when its source text is full of MUSTs. `rfc5301` is
exactly that: `rfc/full/rfc5301.txt` carries 4 MUST-level keywords, its summary
captured 4 advisory rows and 0 gated, and it is absent from the table whose own
docstring (`:1340-1348`) says "an absent summary is indistinguishable from a
compliant one, which is how a standards claim rots". The table has 2 rows today;
7 summaries are advisory-only, and the count is confirmed at exactly 7: rfc3765,
rfc4486, rfc5301, rfc7999, rfc8195, rfc8326, rfc9129. Separately, that table's verdict for a zero-source
count reads "consistent: source declares none", which is asserted for `rfc1035`
-- a 1987 document that predates RFC 2119 and contains 0 uppercase `MUST` but 23
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
  behind the RFC 1035 "Supported" row; its header cites `rfc/short/rfc1035.md`.
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
- `ai/rules/rfc-compliance.md` "the four ratchets" table - becomes six.
- `ai/INDEX.md:212` - already stale ("the three ratchets"); corrected here.

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
| AC-12 | A Remaining cell spells a MUST-gap count (`One`..`Nineteen`, `Twenty`..`Twenty-nine`, `Thirty`..) followed by `MUST` or `SHALL` | The spelled number equals the count of `{gap}` annotations in that stem's summary, or the gate exits 2 naming both numbers; `Twenty-five` parses as 25, never as 5 |
| AC-13 | A summary declares zero GATED requirements while declaring advisory ones | It appears in the ledger's "Summaries declaring no MUST-level requirement" table; when its source keyword count is zero the verdict names the pre-RFC-2119 uncertainty instead of asserting "consistent: source declares none", and reports the lowercase `must` count as evidence |
| AC-14 | A `non-normative` disposition reason asserts non-applicability to Ze rather than a property of the RFC text | The gate exits 2; a disposition records why the DOCUMENT imposes nothing, never a judgement about what Ze owes |
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
- `ai/rules/rfc-compliance.md` - "What Keeps RFC Testing Valid (the four
  ratchets)" becomes six, and the section gains the disclosure guards.
- `ai/INDEX.md` - `:212` corrects "the three ratchets" (already stale) and names
  the disposition file; the keyword row at `:372` gains `rfc/not-enrolled.txt`.
- `docs/features/rfc-status.md` - preamble states which properties of the page
  are machine-checked and which remain editorial (OR-4, AC-25), with a source
  anchor to `scripts/dev/rfc_requirements.py`. The four D1 rows (`:28` rfc4486,
  `:56` rfc3765, `:123` rfc5301, `:234` rfc1035) are corrected in phase 6 to
  match what the re-authored summaries actually declare.
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
| The public page says what it guarantees | `sed -n '1,10p' docs/features/rfc-status.md` shows the machine-checked vs editorial paragraph (AC-25) |
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
documentation obligation that follows is the ratchet table in that rule (four
ratchets becomes six), listed in Files to Modify.

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
