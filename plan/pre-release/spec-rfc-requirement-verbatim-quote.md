# Spec: rfc-requirement-verbatim-quote

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-09-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every requirement row in `rfc/short/<stem>.md` carries the RFC sentence it
states, copied verbatim, and `./le rfc check` refuses a row whose quote is not
in `rfc/full/<stem>.txt` (whitespace-normalised).

Origin: ExaBGP's RFC ledger (`qa/rfc/<rfc>.toml`, checked by `check_quote` in
`qa/bin/check_rfc_compliance`). There the requirement text IS the quote, so a
requirement the RFC does not contain cannot be committed, and an inverted one
(RFC 5561's F-bit "MUST be 1" where the RFC says 0) cannot be written at all.
Ze's rows are paraphrases, and the extraction walk ties a row to a site by an
authored mapping, which a real sentence with a wrong paraphrase passes.

Scope decided 2026-09-26 by the owner:

- This spec covers EVERY requirement row, not only unsourced ones. It absorbs
  item 1 of `plan/pre-release/spec-rfc-evidence-strength-4-prose-second-walk.md`
  (quote for unsourced prose rows) and the anchor half of
  `plan/spec-rfc-anchor-resolution-check.md`; strength-4 keeps the blind second
  walk.
- The second ExaBGP idea, a runnable gap (a test tagged against a `{gap}` that
  must fail today and flips when the behaviour lands), is a SEPARATE spec, to be
  written in a later session.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the artifacts and every check `./le rfc check` runs
- [ ] `ai/rules/rfc-compliance.md` - the fabrication measurements and the two arithmetics
- [ ] `plan/pre-release/spec-rfc-evidence-strength-4-prose-second-walk.md` - the overlapping quote design (A-1, A-3, R-1)
- [ ] `plan/pre-release/spec-rfc-evidence-strength-0-umbrella.md` - decisions D-5 and D-6
- [ ] `plan/spec-rfc-anchor-resolution-check.md` - the anchor defect class

### Source
- [ ] `internal/le/rfc/summary.go` - `parseChecklistLine`, `stripMarkers`, `parseFeatureDeclined`
  → Constraint: a row is `- [ ] [<ID>] [<LEVEL>] <text> (§N) {annotation}`; `Text` is the free prose left after trailing `{...}` markers are stripped, and `Section` is derived from the trailing parenthetical
  → Constraint: `{feature-declined}` already carries a double-quoted RFC sentence, read by its own quotation marks because RFC sentences carry semicolons; minimum length `minCorrectionQuote`
  → Constraint: `annotationRE` peels trailing `{...}` groups and `trailingParenRE` takes the LAST parenthetical as the section, so a quote placed inline in the prose can move the section or break the peel; 1394 of 6784 lines already carry a `"`
- [ ] `internal/le/rfc/check_ratchets.go` - `squashWhitespace`, `correctionAuthorizes`, `minCorrectionQuote` (24)
  → Constraint: the one normaliser is `strings.Fields` collapse; matching is case-sensitive, whole-document, over RAW text
- [ ] `internal/le/rfc/inventory.go` - `SourceText`, `SourcePath`, `stripPageFurniture`, `sitesFor`
  → Constraint: site quotes are derived from `stripPageFurniture(raw)`; 163 of 6243 keyword sentences (2.6%) cross a page break and are NOT in `squashWhitespace(raw)`
  → Decision: the row matcher's haystack is `squashWhitespace(stripPageFurniture(source))`, shared with `featureDeclinedQuote` and `correctionAuthorizes` so all three quote paths agree
- [ ] `internal/le/rfc/artifact.go`, `internal/le/rfc/signoff.go` - `ExtractionSite.Quote`, `MappedTo`, `UnsourcedIDs`, `evaluateExtraction`
  → Constraint: `ExtractionSite.Quote` is DERIVED and re-checked against the inventory; each site maps to ONE rid
- [ ] `internal/le/rfc/freshness.go` - `auditFreshness`, `RequirementSHA(req.Text)`
  → Constraint: any change to `Requirement.Text` re-stales that row's audit verdict (79 verdicts in `rfc/audit/rfc7606.json` and `draft-abraitis-idr-addpath-paths-limit.json`); a quote MUST be parsed out of `Text` into its own field or every quoted row owes a re-audit
- [ ] `internal/le/rfc/check_baseline.go`, `internal/le/rfc/check.go` - `baselineLevels`, `checkIDAllocation`, `check()`
  → Constraint: `check()` is a hand-written append sequence; change-scoped rules compare against HEAD and judge nothing where git cannot answer
- [ ] `internal/le/site/rfcledger.go` - `rfcLedgerRequirementOf` publishes `Text` verbatim on the public ledger
- [ ] `.claude/skills/ze-rfc/SKILL.md` - rows are authored by hand; the skill says "quoted or tightly paraphrased" (line 239), "paraphrase only when too long" (258) and "quote verbatim" (581)
  → Constraint: the skill's three wordings collapse into one when this lands

### Reference
- [ ] `~/Code/github.com/exa-networks/exabgp/main/qa/rfc/README.md` - ExaBGP ledger contract
  → Decision: ExaBGP bans a quote across a page break; Ze strips page furniture instead, because 98 machine-quotable site sentences cross one
- [ ] `~/Code/github.com/exa-networks/exabgp/main/qa/bin/check_rfc_compliance` - `normalise`, `check_quote`
  → Constraint: ExaBGP's `normalise` is the same whitespace collapse; no page handling

## Current Behavior (MANDATORY)

Source files read:
- [ ] `internal/le/rfc/summary.go`
- [ ] `internal/le/rfc/check_ratchets.go`
- [ ] `internal/le/rfc/inventory.go`
- [ ] `internal/le/rfc/freshness.go`

A requirement row states its obligation in authored prose. Nothing compares that
prose with the RFC. The extraction walk maps each keyword sentence (site) to one
row id, and the mapping is authored, so a real sentence can carry a wrong row.

Measured 2026-09-26 over 6354 rows in 201 summaries (scratch `measure.out`,
using the real parsers):

| Bucket | Rows | Quote source |
|--------|------|--------------|
| Mapped to exactly one site, site not a lead-in | ~3483 | derivable from the site sentence |
| Mapped to one site that is a lead-in ending in ":" | 89 | human |
| Mapped to several sites | 159 | human choice among derived sentences |
| `unsourced-ids` | 1666 | human |
| Stem has no extraction artifact | 368 | human (10 have no RFC text at all) |
| Artifact exists, row neither mapped nor unsourced (all SHOULD/MAY) | 589 | human, or a SHOULD/MAY sentence locator |

Already quoted in prose: 920 rows carry a span found verbatim in the RFC, 780
of them 24 characters or longer.

Defects the feature exists to catch, confirmed against `rfc/full/`: RFC7432-10-1
allows a length of 0 where §10 says "MUST be set to 32 for an IPv4 address or
128 for an IPv6 address"; RFC4456-x-2 states a MUST NOT the RFC does not
contain. The measurement agent ranked 243 single-site rows with content-word
overlap below 0.34 as worth review (estimate).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`./le rfc check` (and `./le verify` stages that run it) reads `rfc/short/*.md`.

### Transformation Path
1. `parseChecklistLine` splits a line into id, level, text, section and markers
2. `SourceText` loads `rfc/full/<stem>.txt` or `rfc/drafts/<stem>.txt`
3. `check()` runs each check over the parsed requirements and the extraction artifacts
4. findings print; exit 2 on violation

### Boundaries Crossed
Summary markdown to `Requirement` struct; RFC text file to normalised haystack;
HEAD blob (git) to baseline for change scoping.

### Integration Points
`stripMarkers` (new marker kind), `check()` (new check line), `baselineLevels`
(change scope), `rfcLedgerRequirementOf` (public page shows the quote),
`/ze-rfc` skill (authoring).

## Risks & Assumptions

### Assumptions

| # | Assumption | Basis | If wrong | Validation | State |
|---|------------|-------|----------|------------|-------|
| A-1 | `squashWhitespace(stripPageFurniture(source))` finds every derived site quote | 0 of 3572 single-site quotes matched neither raw nor stripped text | valid quotes refused | unit test over a quote crossing a page break | validated by measurement |
| A-2 | A derived site sentence is a correct quote for a single-site row | site quotes are re-derived by `evaluateExtraction` | the row mapping itself is wrong (sample rows 1-6, 9) and the auto-quote exposes it rather than hides it | backfill prints rows whose paraphrase shares little with the quote | confirmed for the sentence, broken for the paraphrase: a derived site sentence is always in the RFC (every applied row passes `rowQuoteRefusal`), but about 1 in 15 applied rows had a paraphrase that meant something else (A-6); owner accepted 2026-09-26, the hand spec re-reads their tests |
| A-3 | Case-sensitive match is right | ExaBGP and both Ze matchers are case-sensitive | a quote with changed case of MUST passes or fails wrongly | test | confirmed: `quoteSource.inSection` matches with `strings.Contains`; `TestCheckQuoteMatchIsCaseSensitive` refuses a row that lowers MUST |

### Risks

| # | Risk | Early signal | Mitigation |
|---|------|--------------|------------|
| R-1 | Arming the rule reds the whole corpus | `./le rfc check` fails on 6354 rows | change-scoped (row added or edited by the commit under test, HEAD^ against HEAD) plus a published backlog count |
| R-2 | Replacing row text re-stales audit verdicts | `checkAuditFreshness` violations after backfill | accepted by the owner (text IS the quote, 2026-09-26): the rfc7606 and draft verdicts are re-run in phase 5 (AC-14) |
| R-3 | Auto-backfill writes a correct quote on a wrongly mapped row, making the wrong mapping look verified | a quote whose words the paraphrase does not share | backfill emits a low-overlap review list; the quote proves the SENTENCE exists, never that the paraphrase matches it |
| R-4 | A quote with `(`, `)` or `{` breaks section or marker parsing | parse errors or moved sections in tests | the section is read only from the trailing `(§…)` parenthetical and markers only from trailing `{…}` groups; the quote is everything before them (`TestQuoteWithParenthesesKeepsTrailingSection`) |
| R-5 | A short generic sentence ("MUST be set to 0.") matches in the wrong place | a quote found in a section the row does not cite | match is scoped to the cited section and its subsections; minimum 24 characters |
| R-6 | Under `./le verify worktree` the tree equals HEAD, so the change-scoped rule judges nothing | verify green over a commit that added an unquoted row | A-5 broke, so the scope is HEAD^ against HEAD (owner, 2026-09-26), the model `checkDiscriminationRatchet` uses, which fires inside the detached verify worktree. A row still uncommitted is judged once committed. Phase 3 also reads `checkIDAllocation` and `checkLevelRatchet`, which share the HEAD-scoped `baselineLevels`, and fixes them if they are blind the same way (journal: `check-cannot-see-the-change-it-looks-for`) |
| R-7 | Replacing a low-overlap row's text changes the obligation its tagged tests claim to prove | tests tagged to a row whose new quote they do not exercise | low-overlap rows are never auto-applied; they go to the review list and to the hand-quote spec |

Additional assumptions:

| # | Assumption | Basis | If wrong | Validation | State |
|---|------------|-------|----------|------------|-------|
| A-4 | A row whose quote carries a lowercase "must" can keep level `[MUST]` through the existing level-correction record in `rfc/corrections/` | `correctionAuthorizes` authorizes a level against a verbatim quote | policy rows need a new marker | read `checkLevelRatchet` and `correctionAuthorizes` producers in phase 1 | validated 2026-09-26, by another route: `correctionAuthorizes` is reached only from `checkLevelRatchet`, which fires only when a HEAD-gated level becomes non-gated; no check compares a row's level with its text, so a [MUST] row quoting a lowercase "must" passes with no record |
| A-5 | The pre-commit route runs `./le rfc check` in the live tree, where HEAD differs from the tree | `baselineLevels` is HEAD-scoped and `checkIDAllocation` is enforced in practice | the ratchet never fires | read the commit route's check invocation in phase 1 | BROKEN 2026-09-26: `./le verify worktree` runs the `rfc check` stage in a detached worktree at the commit under test (`internal/le/verify/lifecycle.go`, `worktree add --detach`), where tree equals HEAD; `./le commit create` and the git hooks run no `rfc check`; `ai/rules/precommit-verify.md` forbids a working-tree gate run |
| A-6 | An auto-apply threshold (content-word overlap at least 0.5, every number in the row present in the sentence, NOT/MUST NOT polarity equal) keeps wrongly mapped rows out | measurement sample rows 1-10 all fall below it | wrong rows get auto-quoted | run the backfill in dry mode and hand-check the 10 sample rows land in the review list | VALID for mapping errors, with two rules added (2026-09-26): a dry run over 6354 rows puts all 10 sample rows in review (unresolved-anchor 2, number-absent 2, partial 2, polarity 1, low-overlap 1, outside-section 1, level 1 by first failing test). The hand-read of 15 quoted rows (evenly spaced over the sorted quote list) found two that covered half of a two-MUST sentence, so `qualified` and `partial` were added and pinned by TestQuoteBackfillRealCorpusReviewsSampleRows. Overlap cannot see a paraphrase whose words match and whose meaning differs: RFC7432-6.3-3, RFC9552-5.2-6, RFC5880-6.7.3-4 are still quoted (backfillKnownMisses), so each rewritten row owes a read of its tagged tests (R-7) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible in the binary. A wrong matcher refuses valid rows (gate red) or passes invented ones (public ledger overstates) |
| How is it reverted? | Single commit revert of the check; backfilled row text stays correct RFC text |
| Who else touches this path? | `spec-rfc-evidence-strength-4-prose-second-walk` (quote for unsourced rows, now owned here), `spec-rfc-anchor-resolution-check` (anchor half, now owned here), every session editing `rfc/short/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a fixture repo whose tip commit adds an unquoted row against HEAD^ | → | row quote check called from `check()` | `TestCheckRefusesNewRowNotVerbatimInSection` in `internal/le/rfc/check_quote_test.go` |
| `./le rfc check` over a fixture whose per-stem unquoted count rises | → | unquoted-count ratchet in `check()` | `TestCheckRefusesUnquotedCountRise` in `internal/le/rfc/check_quote_test.go` |
| `./le rfc quote-backfill stem <stem>` | → | backfill action registered in `actions.go` | `TestQuoteBackfillAppliesSingleSiteRowAndListsLowOverlap` in `internal/le/rfc/quote_backfill_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A row the commit under test adds or edits (HEAD^ against HEAD) whose text (before the trailing section parenthetical and markers) is not one contiguous span of its cited section after whitespace collapse and page-furniture strip | `./le rfc check` exits 2 naming the stem, the row id, the cited section and the first 70 characters of the text |
| AC-2 | Same row, text is a verbatim span of the cited section or one of its subsections, including a span crossing a page break | passes |
| AC-3 | Row text is verbatim but only in a DIFFERENT section than the one cited | refused, naming the section where it was found |
| AC-4 | Cited section is not a heading (`3.b`) | resolved to the nearest heading ancestor (`3`) and matched there |
| AC-5 | Cited section resolves to no heading of the RFC | refused as an unresolved anchor |
| AC-6 | Row text shorter than 24 characters | refused |
| AC-7 | A row the commit under test leaves unchanged, unquoted | not refused; counted in the stem's unquoted figure printed by `./le rfc check` |
| AC-8 | The commit under test raises a stem's unquoted count over HEAD^ | refused, naming the stem and both counts |
| AC-9 | `featureDeclinedQuote` and `correctionAuthorizes` | use the same page-stripped haystack as the row check (one matcher), and a feature-declined quote crossing a page break now passes |
| AC-10 | `./le rfc quote-backfill stem <stem>` dry run | prints rows it would quote and a review list; writes nothing |
| AC-11 | `./le rfc quote-backfill stem <stem> apply` | rewrites only rows mapped to exactly one non-lead-in site that meet the A-6 threshold, keeping id, level, section and markers; every rewritten row passes AC-2 |
| AC-12 | The ten sample rows in Current Behavior (RFC7432-10-1, RFC4456-x-2 and the others) | appear in the review list, never rewritten |
| AC-13 | Corpus after backfill | `./le rfc check` clean on the quote rules; unquoted total printed and at most the review plus human buckets of the phase 4 dry run (3706: 1271 review, 2435 human; the earlier 2870 estimate did not count the review kinds). Owner, 2026-09-26: apply all 1821 rows the tool quotes, knowing about 1 in 15 had a paraphrase whose meaning differed from its sentence; the hand-quote spec re-reads the tagged tests of applied rows |
| AC-14 | `rfc/short/rfc7606.md` | every row quoted (hand-quoting the rows backfill cannot), and the 79 audit verdicts in `rfc/audit/` re-run fresh |
| AC-15 | Public ledger page for a quoted row | shows the RFC sentence as the requirement text |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckRefusesNewRowNotVerbatimInSection` | `internal/le/rfc/check_quote_test.go` | AC-1 | |
| `TestCheckAcceptsQuoteAcrossPageBreak` | same | AC-2 | |
| `TestCheckAcceptsQuoteInSubsection` | same | AC-2 | |
| `TestCheckRefusesQuoteFoundInOtherSection` | same | AC-3 | |
| `TestQuoteSectionResolvesToHeadingAncestor` | same | AC-4 | |
| `TestCheckRefusesUnresolvedSectionAnchor` | same | AC-5 | |
| `TestCheckRefusesQuoteUnderMinimum` | same | AC-6 | |
| `TestCheckCountsUnchangedUnquotedRow` | same | AC-7 | |
| `TestCheckRefusesUnquotedCountRise` | same | AC-8 | |
| `TestQuoteWithParenthesesKeepsTrailingSection` | same | R-4 | |
| `TestFeatureDeclinedQuoteAcrossPageBreak` | `internal/le/rfc/check_core_test.go` | AC-9 | |
| `TestQuoteBackfillDryRunWritesNothing` | `internal/le/rfc/quote_backfill_test.go` | AC-10 | |
| `TestQuoteBackfillAppliesSingleSiteRowAndListsLowOverlap` | same | AC-11 | |
| `TestQuoteBackfillSkipsLeadInAndMultiSite` | same | AC-11 | |
| `TestQuoteBackfillReviewsInvertedNumber` | same | AC-12 (RFC7432-10-1 shape) | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| quote length | 24 and up | 24 | 23 | N/A |
| overlap threshold | 0.0-1.0 | 0.5 auto-applied | 0.49 reviewed | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le rfc check` selftest fixture with a fabricated row | `internal/le/rfc/selftest_core.go` case | an author commits a MUST the RFC does not contain and the gate refuses it | |

## Files to Modify

- `internal/le/rfc/summary.go` - expose the row's quote span (text before the trailing section parenthetical)
- `internal/le/rfc/check_ratchets.go` - `squashWhitespace` haystack becomes page-stripped for `correctionAuthorizes`
- `internal/le/rfc/check_core.go` - `featureDeclinedQuote` shares the matcher
- `internal/le/rfc/inventory.go` - section lookup by id with heading-ancestor resolution over `sectionBodies`
- `internal/le/rfc/check_baseline.go` - baseline reader returns row text as well as level
- `internal/le/rfc/check.go` - call the quote check and the ratchet; print unquoted counts
- `internal/le/rfc/actions.go` - register `quote-backfill`
- `internal/le/rfc/selftest_core.go` - fabricated-row case
- `rfc/short/*.md` - backfilled rows; `rfc/short/rfc7606.md` fully quoted
- `rfc/audit/rfc7606.json`, `rfc/audit/draft-abraitis-idr-addpath-paths-limit.json` - re-run verdicts
- `docs/contributing/rfc-conformance-gates.md` - the quote rule, page-furniture haystack, section anchors, unquoted ratchet
- `ai/rules/points/rfc-compliance/` directive point (then `./le rules condensed-update`) - row text is a verbatim RFC span
- `.claude/skills/ze-rfc/SKILL.md` - one wording: quote verbatim, never paraphrase
- `plan/pre-release/spec-rfc-evidence-strength-4-prose-second-walk.md`, `plan/spec-rfc-anchor-resolution-check.md` - note the absorbed items and point here

## Files to Create

- `internal/le/rfc/check_quote.go` - row quote check and unquoted ratchet
- `internal/le/rfc/check_quote_test.go`
- `internal/le/rfc/quote_backfill.go` - backfill action
- `internal/le/rfc/quote_backfill_test.go`
- `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` - the rows backfill cannot quote, until the count is zero and the rule covers every row

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling, no product config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le rfc quote-backfill` in `internal/le/rfc/actions.go` |
| CLI grammar (keyword before value) | Yes | `stem <stem>`, `apply` keyword, per `ai/rules/cli.md` |
| Editor autocomplete | N-A | le action, not the ze CLI |
| Functional test for new RPC/API | Yes | selftest case in `selftest_core.go` |
| Pipe completeness | N-A | le action output, not a ze command |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | reads files already read by `./le rfc check` |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | not a family |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | tooling only |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | le action documented in `docs/contributing/rfc-conformance-gates.md` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/*.md` row text; public ledger text changes through `rfcledger.go` |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` is declared by `summary.go`'s `// Design:` header; it describes the registry of RFC obligations, which keeps its shape (a row's text becomes the RFC sentence), so the page is unaffected |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/pre-release/spec-rfc-requirement-verbatim-quote.md` in phase 1 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | row-format examples in `docs/contributing/rfc-conformance-gates.md` and `.claude/skills/ze-rfc/SKILL.md` |

## Implementation Steps

1. **Phase: Wiring** -- quote check and ratchet called from `check()`, backfill action registered; validate A-4 and A-5 by reading their producers
   - Tests: the three Wiring Test rows
   - Files: `check.go`, `check_quote.go`, `actions.go`, `quote_backfill.go`
   - Verify: tests fail on the stub
2. **Phase: Matcher and sections** -- one page-stripped haystack shared by three callers; section lookup with heading-ancestor resolution
   - Tests: AC-2 to AC-6, AC-9, R-4
   - Files: `inventory.go`, `check_ratchets.go`, `check_core.go`, `summary.go`
3. **Phase: Change scope and ratchet** -- baseline row text from HEAD; unquoted counts per stem
   - Tests: AC-1, AC-7, AC-8
   - Files: `check_baseline.go`, `check_quote.go`
4. **Phase: Backfill** -- dry run, threshold (A-6), apply
   - Tests: AC-10 to AC-12; dry-run over the corpus confirms the ten sample rows are reviewed
5. **Phase: Corpus** -- apply backfill over all stems; hand-quote rfc7606; re-run its audits; write the hand-backfill spec with the review list
   - Verify: AC-13, AC-14, AC-15
6. **Phase: Docs and rules** -- page, rule point, skill, the two absorbing spec notes, in the same work as the code they describe

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the section lookup never falls back to the whole document silently; an unresolved anchor is a refusal |
| Correctness | a backfilled row keeps its id, level, section parenthetical and every marker |
| Data flow | one normaliser, one haystack builder, three callers |
| Rule: principles (no silent zero) | a stem whose RFC text is missing is reported as unjudged, never as zero unquoted |
| Rule: no-layering | no paraphrase kept beside the quote; the text IS the quote |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Quote check armed | `./le rfc check` refuses the selftest fabricated row |
| Backfill applied | unquoted total printed by `./le rfc check` at or under the human bucket |
| rfc7606 fully quoted | its unquoted count is 0 and `rfc/audit/rfc7606.json` fresh |
| Hand-backfill spec exists | `ls plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | quote text containing `(`, `)`, `{`, `}` or `"` cannot move the section or truncate markers |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Row text IS the quote (owner, 2026-09-26) | quote marker beside the paraphrase | a quote beside a wrong paraphrase makes the wrong row look verified |
| Match over page-stripped text | ExaBGP's ban on quoting across a page | 2.6% of RFC sentences cross a page; the inventory already strips furniture |
| Match scoped to the cited section | whole-document match | verifies the section anchor too (absorbs `spec-rfc-anchor-resolution-check`) and stops generic sentences matching elsewhere |
| Scope is the commit under test against HEAD^ (owner, 2026-09-26, after A-5 broke) | tree against HEAD (blind in the detached verify worktree); both scopes (two comparisons for one rule) | the only scope that fires at the gate, and the one the discrimination ratchet already uses |
| Change-scoped rule plus per-stem ratchet | arm over all rows at once | 2870 rows need a human; the ratchet stops the backlog growing while the hand spec drains it |
| Keep the markdown ledger | ExaBGP-style TOML | 201 files and every parser rewritten for no gain the containment check does not already give |
| Low-overlap rows never auto-applied | apply every single-site row | a wrong mapping would change the obligation under the tagged tests |

## Known Limitations

- Rows backfill cannot quote are owned by `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md`, which ends with the rule covering every row.
- The runnable-gap idea from ExaBGP (a gap test that flips when behaviour lands) is a separate spec, to be written in its own session (owner, 2026-09-26).

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests: N-A, tooling with no protocol peer

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** remove the spec, in the same `./le commit create` script

## Implementation Summary

### What Was Implemented
- Row quote rule: `Requirement.Quote` (`summary.go`), `quoteHaystack` and `quoteSource` with heading-ancestor `resolve` (`inventory.go`), `checkRowQuotes`, `rowQuoteRefusal`, `readQuoteRevisions`, `checkUnquotedRatchet`, `unquotedFigures` (`check_quote.go`), wired in `check()` with the `unquoted` figures on `CheckReport` (`check.go`).
- One haystack for three quote paths: `featureDeclinedQuote` (`check_core.go`) and `correctionAuthorizes` (`check_ratchets.go`) call `quoteHaystack`.
- Scope HEAD^ against HEAD: `baselineLevels`, `baselineMetas`, `baselineMetasBeforeMigration` and `baselineSummaryStems` read `HEAD^` (`check_baseline.go`), so the id, level, retirement, enrolment, public-row and new-summary ratchets see the tip commit in the detached verify worktree.
- `./le rfc quote-backfill stem <stem> [apply]` (`quote_backfill.go`, registered in `actions.go`), and the selftest stage `quote` (`selftest_core.go`).
- Corpus: 1821 rows quoted by the backfill over 153 summaries, 34 rfc7606 rows quoted by hand, 53 audit verdicts judged again (`rfc/audit/rfc7606.json`, `rfc/audit/draft-abraitis-idr-addpath-paths-limit.json`).

### Bugs Found/Fixed
- The id, level and retirement ratchets read HEAD and were blind in the detached verify worktree. Fixed by the HEAD^ move: `TestCheckBaselineRatchetsSeeTipCommit`, `TestCheckMetaRatchetsSeeTipCommit` (journal `check-cannot-see-the-change-it-looks-for`).
- Closure review: the ratchet's RFC-text path had no test (`TestCheckRefusesUnquotedCountRiseFromRFCTextChange`), and quote-backfill used an unchecked stem as a path (`TestQuoteBackfillRefusesStemOutsideSummaries`).

### Documentation Updates
- `docs/contributing/rfc-conformance-gates.md`: new "The row quote" section with `<!-- source: internal/le/rfc/quote_backfill.go -- quoteBackfill -->`; the baseline rows now say `HEAD^`.
- Rule point `ai/rules/points/rfc-compliance/directives/quote-each-requirement-row-verbatim.md`, generated `ai/rules/rfc-compliance.md` and `ai/rules/CORE.md`. `ai/skills/ze-rfc.md` has one wording: quote verbatim.
- `./le doc check verify`: every stage passes except validate-commands, which names `ze-data:backup` and `ze-data:restore` with no handler. That is another session's uncommitted `cmd/ze/hub` work, not this spec.

### Deviations from Plan
- The scope is HEAD^ against HEAD, not the tree against HEAD (A-5 broke; owner decision 2026-09-26).
- The AC-13 bound is 3706 (review plus human), not the earlier 2870 estimate. The tree is at 3662.
- `.claude/skills/ze-rfc/SKILL.md` is a generated mirror. The canonical edit is `ai/skills/ze-rfc.md`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5: the commit route runs `./le rfc check` in the live tree | verify runs it detached at the commit, where the tree equals HEAD | phase 1 read `internal/le/verify/lifecycle.go` | scope moved to HEAD^ (owner), siblings fixed, journal row |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every row carries its RFC sentence | Partial | `rfc/short/*.md` | 3662 rows remain, owned by `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` (the owner-agreed design: ratchet plus hand spec) |
| `./le rfc check` refuses a row whose quote is not in the RFC | Done | `check_quote.go` `checkRowQuotes` | change-scoped, plus the ratchet |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCheckRefusesNewRowNotVerbatimInSection` | the message names stem, id, section and 70 characters |
| AC-2 | Done | `TestCheckAcceptsQuoteAcrossPageBreak`, `TestCheckAcceptsQuoteInSubsection` | |
| AC-3 | Done | `TestCheckRefusesQuoteFoundInOtherSection` | |
| AC-4 | Done | `TestQuoteSectionResolvesToHeadingAncestor` | |
| AC-5 | Done | `TestCheckRefusesUnresolvedSectionAnchor` | |
| AC-6 | Done | `TestCheckRefusesQuoteUnderMinimum` | 23 refused, 24 passes |
| AC-7 | Done | `TestCheckCountsUnchangedUnquotedRow` | |
| AC-8 | Done | `TestCheckRefusesUnquotedCountRise`, `TestCheckRefusesUnquotedCountRiseFromRFCTextChange` | |
| AC-9 | Done | `TestFeatureDeclinedQuoteAcrossPageBreak` | |
| AC-10 | Done | `TestQuoteBackfillDryRunWritesNothing` | |
| AC-11 | Done | `TestQuoteBackfillAppliesSingleSiteRowAndListsLowOverlap`, `TestQuoteBackfillSkipsLeadInAndMultiSite` | |
| AC-12 | Done | `TestQuoteBackfillRealCorpusReviewsSampleRows`, `TestQuoteBackfillReviewsInvertedNumber` | |
| AC-13 | Done | phase 5B `unquotedFigures`: 3662 over 187 stems, under 3706; rfc8326 and rfc9129 unjudged | 1821 applied rows by owner decision |
| AC-14 | Done | rfc7606 is absent from the unquoted figures; the live `./le rfc check` names no stale verdict | 4 verdicts moved from enforced to weak, routed to the hand spec |
| AC-15 | Done | `rfcLedgerRequirementOf` publishes `requirement.Text` (`internal/le/site/rfcledger.go`), which is now the quote | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the 15 planned unit tests | Done | `check_quote_test.go`, `check_core_test.go`, `quote_backfill_test.go` | pass under `-race` |
| selftest fabricated row | Done | `TestRFCSelftestQuoteStageRefusesFabricatedRow` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file in Files to Modify and Files to Create | Done | the skill through its canonical `ai/skills/ze-rfc.md` |

### Audit Summary
- **Total items:** 15 AC, 2 task requirements, 16 tests, 19 files
- **Done:** all but one
- **Partial:** "every row" (3662 rows): the owner-approved design drains them in the hand spec
- **Skipped:** none
- **Changed:** scope HEAD^ (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A requirement the RFC does not contain cannot be committed | functional (selftest through the public check) | `TestRFCSelftestQuoteStageRefusesFabricatedRow`: a committed fabricated MUST is refused, and the verbatim row passes |
| Rows carry the RFC's own sentence | corpus measurement | 1855 rows rewritten; the unquoted total went from 3696 to 3662 after rfc7606; for every stem the figure equals review plus human, so every applied row passes `rowQuoteRefusal` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Quote the 3662 rows the backfill could not, then arm the rule over every row | each needs a human | `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` |
| Read the tagged tests of the 1821 applied rows again (R-7), and tag the four rfc7606 rows now weak | the owner accepted the apply with this follow-up | `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-requirement-verbatim-quote-ba98bd2b-fe38-40c5-80a7-24aae0c73001.md` (186 files, verdict clean) |
| `./le spec review check` | clean over the same 186 files |
| Rounds | 2 |
| Reviewer lenses used | wiring, logic, edge cases, security (path input), style pass, removed behavior (HEAD to HEAD^), docs, citers |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The ratchet's own path (RFC text changes under an untouched row) had no test; the rise test also tripped the row check | `check_quote.go` `quoteChangedStems` | `TestCheckRefusesUnquotedCountRiseFromRFCTextChange`, red with the RFC-text case disabled |
| 2 | ISSUE | `QuoteBackfillReport` and `QuoteBackfillRow` exported with no cross-package caller (`./le repo check`) | `quote_backfill.go` | unexported |
| 3 | ISSUE | `quote-backfill` rewrote the file its stem names without checking that the stem is a stem | `quote_backfill.go` `quoteBackfill` | `stemRE` guard, `TestQuoteBackfillRefusesStemOutsideSummaries`, red without the guard |

NOTEs (recorded, not blocking): figures read zero on a failing `./le rfc check`, the existing pattern for every figure; `backfillRewrite` has one compound condition; a two-commit script is verified only at its tip, so the gate judges commit A's rows only when A itself is verified; `./le repo check` also names five older exports in `inventory.go` and `rfc.go`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/rfc/check_quote.go`, `check_quote_test.go`, `quote_backfill.go`, `quote_backfill_test.go` | yes | `wc -l` at closure |
| `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` | yes | 171 lines |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-12 | tests pass | closure run `go test -race -run 'Quote|FeatureDeclinedQuote|SeeTipCommit|RatchetsFireWhenEnrolmentMoves|PublicRowDeleted|Extraction|Selftest[^R]' ./internal/le/rfc`: exit 0 |
| AC-13, AC-14 | figure and freshness | `phase5b/figures.json` total 3662, rfc7606 absent; the live `./le rfc check` has one violation, the `cmd/ze/hub` type check (another session) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc check` | none: le tooling, `Check(root)` over a committed fixture repository | `TestCheckRefusesNewRowNotVerbatimInSection` and `TestCheckRefusesUnquotedCountRise` read |
| `./le rfc quote-backfill` | none | registered in `actions.go`; `TestQuoteBackfillAppliesSingleSiteRowAndListsLowOverlap` read |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestCheckAcceptsQuoteAcrossPageBreak` |
| A-2 | confirmed for the sentence, broken for the paraphrase | the A-6 row; owner accepted |
| A-3 | confirmed | `TestCheckQuoteMatchIsCaseSensitive` |
| A-4 | confirmed by another route | `correctionAuthorizes` is reached only from `checkLevelRatchet` |
| A-5 | broken | Mistake Log, Deviations |
| A-6 | confirmed with two added kinds | `TestQuoteBackfillRealCorpusReviewsSampleRows` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "The row quote" refusals and scope | `rowQuoteRefusal`, `readQuoteRevisions`, `checkUnquotedRatchet` | yes |
| baseline at `HEAD^` | `baselineLevels`, `baselineMetas`, `baselineSummaryStems` | yes |
| RFC status page | `./le rfc index-update` rewrote no tracked file | yes |
