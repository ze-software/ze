# Spec: rfc-ledger-single-declaration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/rfc-ledger-single-declaration.md` |
| Handoff | - |
| Updated | 2026-09-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Declare each fact about an RFC once, in the summary that owns it, and derive
every other surface from it.

Three facts about one RFC live in three files that RFC does not own. Its
requirements are authored in `rfc/short/<stem>.md`. Its enrolment is a row in
`rfc/enrolled.txt` or `rfc/not-enrolled.txt`. Its public support claim is a row
in `docs/features/rfc-status.md`. Nothing binds the three together, so every way
they can disagree needed a gate of its own, and every enrolment costs an edit to
a file that every other RFC shares.

Two costs follow, and both are measured rather than argued.

The first is churn. Over the last 400 commits `rfc/enrolled.txt` was touched by
120 and `docs/features/rfc-status.md` by 120. A file every RFC shares is also a
file every session collides on, in a checkout several sessions work at once.

The second is a class of defect only a second copy can have, and the populations
are already apart. Measured 2026-09-01: 191 summaries, 181 enrolled, 9 declared
not enrolled, 1 in neither, and 158 rows on the public page. Seven of those rows
name an RFC with no summary at all: RFC 4762, 5065, 5120, 5925, 6514, 8362 and
8538. Thirty-two enrolled RFCs have no public row and are grandfathered.
`plan/journal/gate-excludes-part-of-its-population.md` holds the same shape as an
open row against `checkSuperseded`, where a Meta cell absent from
`rfc/short/rfc8203.md` removed that stem from a gate's population while the gate
read clean.

After this spec, `rfc/enrolled.txt`, `rfc/not-enrolled.txt` and
`docs/features/rfc-status.md` are GENERATED from the summaries. A stem in both
disposition files, a disposition naming a summary that does not exist, and a
duplicate key on the public page each become unrepresentable rather than
refused.

This spec changes no conformance verdict, moves no requirement, and fixes no
`{gap}`. It changes where three facts are written down.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - what each gate checks and refuses
  → Constraint: the public ledger's three hard guards exist because
    `checkStatusAgreement` reaches for a row only when a `{gap}` exists. The
    generated page must keep the two claims that are NOT about agreement:
    `checkUnprovenSupport`'s support claim over a summary declaring zero gated
    requirements, and `checkGapCountAgreement`'s spelled-number comparison.
  → Decision: "Un-enrolment exempts only the MISSING-ROW branch." The generated
    page must preserve that asymmetry.
- [ ] `ai/rules/principles.md` - the declaration rule this spec is an instance of
  → Constraint: every fact is declared once and every other surface derives from
    that declaration; a new member registers itself rather than being added to a
    central enumeration. `rfc/enrolled.txt` is that central enumeration.
- [ ] `ai/rules/no-layering.md` - what the migration may not be
  → Constraint: X is deleted first and Y implemented after. No period in which
    both the files and the Meta fields are authored, and no fallback that reads
    the file when the Meta field is absent.
- [ ] `docs/architecture/core-design.md` - the design doc every `internal/le/rfc/` file declares
  → Constraint: the rfc area is one command with one shared parse; a second
    derivation of the same fact beside `NewRenderInput` is what this spec removes
    rather than adds.
- [ ] `website/AI.md` - the design doc `internal/le/site/` declares
  → Constraint: the site derives its RFC figures from `rfc.Collect`, so moving
    the declaration changes what the site reads and not how it renders.
- [ ] `plan/journal/gate-excludes-part-of-its-population.md` - 109 rows, the
  largest recorded class, two open rows against these files
  → Constraint: an absent Meta field must be an ERROR naming the summary. The
    2026-08-30 row records the alternative: `parseSuccessorStem` returns an
    empty successor for an absent cell, the stem leaves `checkSuperseded`'s
    population, and nine requirement lines are never asked for a disposition.

### RFC Summaries (Scope: protocol)

Not applicable: this spec changes no protocol behavior. It changes where the
ledger records enrolment and the public support claim.

**Key insights:**
- Every refusal that compares two copies is retired by having one copy. Every
  refusal that states a property of one document survives unchanged.
- The Meta parser is per-field and asymmetric: an absent field is a silent
  default, a misspelled-but-recognizable field is a loud error. A new field
  inherits the silent default unless the spec designs it not to.
- The deleting commit disarms four ratchets, not one. That is the hazard this
  spec must land a fix for in the same commit.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/rfc/check.go` - the driver, and the enrolled-set gate at its ratchet block
- [ ] `internal/le/rfc/check_status.go` - `checkSummaryDisposition`, `checkStatusAgreement`, `checkStatusCompleteness`, `checkUnprovenSupport`, `checkGapCountAgreement`, `checkSupportedSignoff`
- [ ] `internal/le/rfc/check_core.go` - `checkEnrolment`, `checkSuperseded`
- [ ] `internal/le/rfc/check_ratchets.go` - the four ratchets behind the enrolled-set gate
- [ ] `internal/le/rfc/check_baseline.go` - `baselineEnrolled`, `baselineStatusRows`, `baselineDispositions`, `baselineSummaryStems`
- [ ] `internal/le/rfc/summary.go` - `parseEnrolled`, `summaryTitle`
- [ ] `internal/le/rfc/ledger.go` - `parseStatusLedger`, `parseDispositions`, `parseSuccessorStem`
- [ ] `internal/le/rfc/render.go` - `NewRenderInput`, the one shared derivation point
- [ ] `internal/le/site/rfccompliance.go` - `rfcInputsHTML`, `rfcComplianceMirror`
- [ ] `internal/le/site/rfcdetail.go` - the per-stem enrolment strings
- [ ] `internal/le/site/docsmanifest.go` - the page's verbatim publication route
- [ ] `rfc/enrolled.txt` - stem, reason, 181 rows, free text
- [ ] `rfc/not-enrolled.txt` - stem, kind, why, 9 rows, kinds `backlog`, `blocked`, `non-normative`
- [ ] `docs/features/rfc-status.md` - 158 rows, all five columns authored

Three refusals of `checkSummaryDisposition` and two of the status checks exist
only because the fact is stored outside the summary:

| Refusal | Producer | After this spec |
|---------|----------|-----------------|
| a summary in neither disposition file | `checkSummaryDisposition` | becomes a schema check over one Meta cell |
| a stem in BOTH files | `checkSummaryDisposition` | unrepresentable: one field holds one value |
| a disposition naming a summary that does not exist | `checkSummaryDisposition` | unrepresentable: the field dies with the file |
| a `{gap}` whose RFC has no public row | `checkStatusAgreement` | becomes a schema check over one Meta cell |
| newly enrolled with no public row | `checkStatusCompleteness` | becomes a schema check over one Meta cell |
| a `non-normative` reason judging what ZE owes | `checkSummaryDisposition` | unchanged, still checked over the field's text |
| a support claim over a summary declaring zero gated requirements | `checkUnprovenSupport` | unchanged |
| a Remaining count disagreeing with the `{gap}` count | `checkGapCountAgreement` | unchanged |
| a row that existed at HEAD and is gone while the stem stays enrolled | `checkStatusCompleteness` | survives only through a baseline reader over the new source |

**The Meta parser is per-field, and its failure mode is asymmetric.** There is no
generic Meta-table reader. `summaryTitle` (`internal/le/rfc/summary.go`) and
`parseSuccessorStem` (`internal/le/rfc/ledger.go`) each match one labeled row by
regex. An absent field returns an empty value with no error. A field whose name
matches the obsolescence pattern in an unrecognized spelling is a loud error.
Nothing distinguishes a summary with no Meta table from one whose Meta table
omits the field.

**The corpus spells its Meta fields 45 ways.** All 191 summaries carry a `## Meta`
heading. Obsolescence is written four ways, "Updated by" two, the identity field
is `RFC` in 174 summaries, `Number` in 9 and `RFC Number` in 1, and two rows
carry `Obsoletes / Updates` as one label.

**Four ratchets are gated on the enrolled sets meeting.** `check.go` runs
`checkRetiredRequirements`, `checkLevelRatchet`, `checkCoverageRatchet` and
`checkEvidenceRatchet` only inside a test that the current enrolled set
intersects the baseline one, and an unreadable baseline is replaced by an empty
map. At the commit that deletes `rfc/enrolled.txt`, HEAD is that commit, the
baseline read fails, the intersection is empty, and all four stop running for the
one commit whose own changes they exist to judge. `checkStatusCompleteness`
loses its HEAD-row branch the same way.

**The public page is authored prose, not derived data.** Area matches the
summary's Title in 3 of 158 rows. Status carries 13 distinct values. Implemented
coverage totals 85,608 characters with a 14,449-character maximum, and Remaining
totals 73,034 with a 3,620 maximum; both maxima are RFC 7296. Only the numeric
gap count is derived, in the 56 of 158 rows that carry one, and all 56 agree
today because a live gate enforces it.

**Behavior to preserve:**
- Every refusal that states a property of ONE document.
- The enrolment ratchet's monotonicity, and the asymmetry that un-enrolment
  exempts only the missing-row branch.
- The published page's columns and prose, and its route at `/reference/rfcs/`.

**Behavior to change:**
- Enrolment and the public support claim are authored in the summary's Meta
  table. The three files become outputs of `./le rfc index-update`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`./le rfc check`, driven by `check()` (`internal/le/rfc/check.go`), and
`./le rfc index-update`, driven by the writer in `internal/le/rfc/write.go`.

### Transformation Path

`rfc/short/<stem>.md` → the summary parser → the Meta fields → one enrolment map,
one disposition map and one status map, the same three maps the checks consume
today → the checks, unchanged in what they judge. The same maps feed `render.go`,
which emits the three files as generated output.

### Boundaries Crossed

One: the working tree against git HEAD, read by `check_baseline.go` through git.
Nothing else. No network, no plugin boundary, no kernel.

### Integration Points

`NewRenderInput` (`internal/le/rfc/render.go`) is the one shared derivation
point: both `ai/RFC-REQUIREMENTS.md` and the website's RFC pages flow through it,
so re-pointing the three loaders there keeps the two producers from diverging.
`internal/le/site/rfccompliance.go` and `rfcdetail.go` name the three paths as
literal strings in published prose and must be edited in the same work.
`plan/spec-publish-the-rfc-requirement-ledger.md` is in progress over
`/quality/rfc-compliance/<stem>/`, a different route from the mirror this page
feeds.

### Architectural Verification

The generated files carry the same do-not-edit banner as the other generated
ledgers, and `./le doc check verify` reports them stale against their sources the
way it reports `ai/RFC-REQUIREMENTS.md` stale.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A new Meta field can be added uniformly across the corpus | 191 of 191 summaries carry a `## Meta` heading | the field lands in 45 spelling regimes and reads absent in some | measured: BROKEN. 45 distinct field spellings exist, so step 1 is a normalizing pass | broken |
| A-2 | Area and Status move verbatim as short authored values | Area matches Title in 3 of 158 rows; Status carries 13 distinct values | the migration is a rewrite rather than a move | measured: confirmed, both are authored | confirmed |
| A-3 | Implemented coverage and Remaining are editorial prose no gate derives | only the spelled gap count is read, in 56 of 158 rows | generation would have to synthesize prose, which it must not do | measured: confirmed, 158 KB of authored prose | confirmed |
| A-4 | The enrolment baseline can be read from HEAD's summaries | `baselineSummaryStems` already reads summaries at HEAD | four ratchets stay disarmed past the migration commit | AC-6 | CONFIRMED, with one correction the assumption missed: HEAD's summaries answer nothing AT the migration commit, because they predate the field. `baselineMetas` reads them and falls back to HEAD's two retired files only when no summary at HEAD declares an enrolment at all. `TestRatchetsFireWhenEnrolmentMovesToMeta` proves the ratchets fire across that commit, and forcing the fallback to return nothing makes it red |
| A-5 | No consumer outside `internal/le/rfc/` parses the three files | measured: the website names two of the paths as literal strings in published prose | the public page cites deleted paths | AC-7 | CONFIRMED as stated, BROKEN as intended. Nothing outside the package PARSES them, so no reader broke. What names them is published PROSE, in `internal/le/site/rfccompliance.go` and `rfcdetail.go`, and those files belong to another session that was editing them concurrently. The replacement wording was handed over rather than written here; the deferral shard carries the row |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | The deleting commit disarms `checkRetiredRequirements`, `checkLevelRatchet`, `checkCoverageRatchet` and `checkEvidenceRatchet`, because the enrolled baseline is unreadable and the intersection is empty | the check reports fewer violations over a larger change | AC-6: the baseline reader over Meta rows at HEAD lands in the same commit, and a test forces the four ratchets to fire across the migration |
| R-2 | An absent or misspelled Meta field defaults to "not enrolled", removing a stem from the gated population | the enrolled count moves without an author intending it | AC-3: an absent or unrecognized field is an error naming the summary |
| R-3 | Generating the page from summaries deletes the seven public rows whose RFC has no summary | those RFCs vanish from the public ledger | AC-8: the seven summaries are written first; RFC 6514 and RFC 8362 carry support claims and need requirements or a recorded escape |
| R-4 | The migration silently changes an enrolment while no ratchet is watching | the generated file differs from the deleted one | AC-5: byte-identical generation, proven inside the migration commit |
| R-5 | The published `/quality/rfc-compliance/` page keeps citing `rfc/enrolled.txt` after it is generated | a reader follows a path that no longer exists | AC-7: the site's literal citations are edited in the same work |
| R-6 | The Meta field becomes a second place to write a claim while the page still looks authored | an author edits the generated page | the banner plus `./le doc check verify` reporting it stale |
| R-7 | AC-11 silently empties `checkGapCountAgreement`'s population. `gapCountRE` matches a spelled number IMMEDIATELY before MUST or SHALL, so "Fifteen of 219 MUST-level rows" does not match, `spelledGapCount` returns not-held, and the row leaves the check without a word of complaint | the checked-row count falls from 56 | the renderer keeps the spelled number adjacent to the keyword and appends the denominator after it, so the existing reader still matches; AC-12 pins the population at 56 of 158 rows rather than trusting the phrasing |

## Blast Radius

`internal/le/rfc/`, `internal/le/site/`, and the corpus under `rfc/short/`. No
product code and no protocol behavior. Three tracked files change from authored
to generated, and seven summaries are created.

## Wiring Test (MANDATORY -- NOT deferrable)

| What | Test |
|------|------|
| The Meta field reaches `./le rfc check` from a real summary | `TestEnrolmentReadFromSummaryMeta` -> a fixture stem enrolled by its Meta field is gated, and the same stem with the field absent is refused |
| The generated page reaches the tree from `./le rfc index-update` | `TestIndexUpdateWritesTheStatusPage` -> the writer emits `docs/features/rfc-status.md` and `./le doc check verify` reports it fresh |
| The four gated ratchets still fire across the migration | `TestRatchetsFireWhenEnrolmentMovesToMeta` -> a fixture whose HEAD carries the file shape and whose tree carries the Meta shape still reports a coverage regression |

## Acceptance Criteria

| ID | When | Then |
|----|------|------|
| AC-1 | A summary's Meta table carries its enrolment field | `./le rfc check` gates that RFC's MUST-level requirements, with no row in any disposition file |
| AC-2 | A summary's Meta table carries its support field | `./le rfc index-update` renders that RFC's row on the public page with its Area, Status, Implemented coverage and Remaining cells |
| AC-3 | A summary carries no enrolment field, or one whose value is outside the closed set | `./le rfc check` refuses, naming the summary and the accepted values; it does NOT default to un-enrolled |
| AC-4 | Two summaries would render the same status key | unrepresentable by construction; a test asserts one summary renders exactly one row |
| AC-5 | The migration commit is applied | the generated three files are byte-identical to the authored files they replace |
| AC-6 | The migration commit is checked | `checkRetiredRequirements`, `checkLevelRatchet`, `checkCoverageRatchet` and `checkEvidenceRatchet` all run, because the enrolled baseline is read from HEAD's summaries |
| AC-7 | The published `/quality/rfc-compliance/` page and the per-stem pages are rendered after the change | no rendered text names `rfc/enrolled.txt`, `rfc/not-enrolled.txt` or an authored `docs/features/rfc-status.md` as an authored input |
| AC-8 | The seven RFCs with a public row and no summary are migrated | each has a summary carrying its support field, and RFC 6514 and RFC 8362 satisfy `checkUnprovenSupport` with requirements or a recorded escape |
| AC-9 | An author edits the generated public page by hand | `./le doc check verify` reports it stale against its sources |
| AC-10 | `checkSummaryDisposition`, `checkUnprovenSupport` and `checkGapCountAgreement` are run after the change | every refusal that states a property of one document still fires on its fixture |
| AC-11 | Any of the three generated files states a count | it states the absolute number AND the percentage of the population it is drawn from, and names that population; a bare count is refused by the renderer's own test |
| AC-12 | The generated page is checked after AC-11 | `checkGapCountAgreement` still reads a gap count from the same 56 of 158 rows it reads today, and a test asserts that population rather than the phrasing |

## End-to-End User Stories

| Story | Path |
|-------|------|
| An author enrols an RFC | edits one file, `rfc/short/<stem>.md`: sets its Meta enrolment and support fields, runs `./le rfc index-update`, and the three generated files follow |
| An author retires an RFC | deletes `rfc/short/<stem>.md`; no disposition row and no public row survives it, because both lived in it |

## 🧪 TDD Test Plan

### Unit Tests

Three planned names moved to the file that holds the producer they exercise, and
two were absorbed by a wider test that covers the planned assertion and more. The
Status column names what landed.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnrolmentIsReadFromTheSummaryMetaTable` | `internal/le/rfc/summary_test.go` | AC-1, AC-3, R-2 | passing. Planned as two tests, `TestEnrolmentReadFromSummaryMeta` and `TestAbsentEnrolmentFieldIsRefusedNotDefaulted`; written as one because the refusals are subtests of the same parse and splitting them would duplicate the fixture. Four refusal cases: absent, outside the closed set, no reason, and a near-miss label |
| `TestGeneratedStatusRowsMatchTheAuthoredPage` | `internal/le/rfc/render_ledger_test.go` | AC-2, AC-5 | passing. Planned as `TestStatusRowRenderedFromSummaryMeta`; written wider, because rendering a row from Meta and rendering the row the page ALREADY held are the same assertion, and the second one also proves the migration lost nothing |
| `TestOneSummaryRendersExactlyOneStatusRow` | `internal/le/rfc/check_test.go` | AC-4 | passing. In `check_test.go` rather than `render_test.go`: it reads the real corpus, which is where the other corpus-wide tests live |
| `TestRatchetsFireWhenEnrolmentMovesToMeta` | `internal/le/rfc/check_test.go` | AC-6, R-1 | passing, and its red phase was FORCED: with `baselineMetasBeforeMigration` returning nothing, the test fails at the baseline assertion |
| `TestSurvivingDispositionRefusalsStillFire` | `internal/le/rfc/check_test.go` | AC-10 | passing. Eight subtests, one per surviving refusal, each a property of a single document |
| `TestHandEditedStatusPageReportsStale` | `internal/le/doc/wiring/docverify_test.go` | AC-9 | passing over all three generated files, and its red phase was FORCED: with the ledger block removed from `rfcFreshnessStage`, all three subtests fail |
| `TestEveryGeneratedCountCarriesItsDenominator` | `internal/le/rfc/render_ledger_test.go` | AC-11 | passing. In `render_ledger_test.go`, beside the renderer it judges |
| `TestGapCountPopulationSurvivesTheMove` | `internal/le/rfc/render_ledger_test.go` | AC-12, R-7 | passing at 57 of 159 rows, not the 56 the spec predicted. The extra row is RFC 1994, whose Remaining cell the authored page's own parser discarded |
| `TestGeneratedEnrolmentMatchesTheAuthoredFiles` | `internal/le/rfc/render_ledger_test.go` | AC-5, R-4 | passing. Not planned. Compares the generated enrolment against HEAD's two authored files, stem for stem and reason for reason |
| `TestSourceRestrictedExcusesASupportClaimAndDebtDoesNot` | `internal/le/rfc/check_test.go` | the `source-restricted` disposition | passing. Not planned: the disposition did not exist when the spec was written |
| `TestSourceRestrictedReasonMustNameWhatStopsTheCopy` | `internal/le/rfc/check_test.go` | the `source-restricted` reason discipline | passing. Not planned, same reason |
| `TestTheMetaScanStopsAtItsOwnTable` | `internal/le/rfc/summary_test.go` | the scan bound | passing. Not planned: written after `rfc/short/rfc8277.md` was refused for a duplicate that was not one |

### Boundary Tests (numeric inputs)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGapCountAgreementAcrossZeroOneAndMany` | `internal/le/rfc/check_test.go` | AC-10 at 0, 1 and n gaps | passing. Seven cases: no spelled number, zero, one agreeing, one disagreeing, twenty-one agreeing, twenty-one off by one, and a digit count the reader deliberately does not judge |

### Functional Tests

Not applicable: `./le rfc check` and `./le rfc index-update` are development
tooling with no daemon path, and a tag in `internal/le/` cannot prove an RFC
requirement. Their carriers are unit tests, which is correct here: this spec
proves tooling, not conformance.

### Interop Tests (Scope: protocol)

Not applicable: no wire-visible change.

## Files to Modify

- `internal/le/rfc/summary.go` - parse the two Meta fields; refuse an absent or unrecognized value
- `internal/le/rfc/ledger.go` - `parseStatusLedger` and `parseDispositions` become derivations over the summaries
- `internal/le/rfc/check_status.go` - retire the comparison refusals, keep the rest
- `internal/le/rfc/check_core.go` - `checkEnrolment` reads the Meta-derived set
- `internal/le/rfc/check_baseline.go` - a baseline reader for enrolment and status over HEAD's summaries
- `internal/le/rfc/check.go` - the ratchet block's enrolled-set gate reads the new baseline
- `internal/le/rfc/render.go`, `internal/le/rfc/write.go` - emit the three generated files
- `internal/le/rfc/sections.go` - the backlog sections that read the dispositions
- `internal/le/rfc/selftest_core.go` - one fixture per surviving refusal
- `internal/le/doc/wiring/docverify.go` - the three files join the freshness check
- `internal/le/site/rfccompliance.go`, `internal/le/site/rfcdetail.go` - the literal path citations
- `rfc/short/*.md` - the Meta normalization and the two new fields
- `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `docs/features/rfc-status.md` - authored to generated
- `docs/contributing/rfc-conformance-gates.md` - the artifact table, the ratchet table and the public-ledger section
- `ai/skills/ze-rfc.md`, `ai/skills/ze-rfc-audit.md`, `ai/INDEX.md`, `ai/rules/rfc-compliance.md`, `ai/rules/testing.md`, `rfc/extraction/README.md` - where enrolment is declared

## Files to Create

- `plan/deferrals/rfc-ledger-single-declaration.md`
- `rfc/short/rfc4762.md`, `rfc5065.md`, `rfc5120.md`, `rfc5925.md`, `rfc6514.md`, `rfc8362.md`, `rfc8538.md` - the seven RFCs carrying a public row and no summary

### Integration Checklist

| Surface | Answer |
|---------|--------|
| YANG schema + validation | N/A - development tooling, no config surface |
| CLI grammar + completion | N/A - no new verb; `./le rfc check` and `index-update` keep their shape |
| Functional test | N/A - no daemon path; unit tests are the carrier |
| Env var | N/A - none added |
| Doctor check + diagnostic code | N/A - no runtime dependency added |
| Prometheus counters | N/A - development tooling |
| BGP family surface | N/A - no family, capability or attribute |
| Registry / inventory preventing drift | Yes - the summary parser is the one declaration; `./le doc check verify` reports the generated files stale |
| Discovery from `ai/INDEX.md` | Yes - the RFC enrolment and status rows are repointed to the summary Meta field |
| Rule preventing regression | Yes - `ai/rules/rfc-compliance.md` and `ai/skills/ze-rfc.md` name the Meta field as the place enrolment is declared |

### Documentation Update Checklist (BLOCKING)

| Page | Answer |
|------|--------|
| `docs/contributing/rfc-conformance-gates.md` | Yes - the artifact table, the ratchet table and the public-ledger edges |
| `ai/skills/ze-rfc.md` | Yes - the enrolment procedure |
| `ai/skills/ze-rfc-audit.md` | Yes - it names the disposition files |
| `ai/INDEX.md` | Yes - the RFC enrolment and status keyword rows |
| `ai/rules/rfc-compliance.md` | Yes - where enrolment is declared |
| `ai/rules/testing.md` | Yes - it names the enrolment files |
| `rfc/README.md` | Yes - if it names the disposition files |
| `rfc/extraction/README.md` | Yes - it names enrolment as a precondition |
| `docs/features/rfc-status.md` | Yes - gains a generated banner |
| `docs/features.md` | N/A - no user-visible feature changes |
| Command docs | N/A - no command shape changes |
| API docs | N/A |
| Plugin docs | N/A |
| Source anchors in architecture docs | Yes - any `// Design:` annotation naming the three files |

## Implementation Steps

1. Normalize the Meta tables. One spelling per fact across all 191 summaries,
   with the parser refusing an unrecognized spelling of a field it knows, as
   `parseSuccessorStem` already refuses an unrecognized obsolescence spelling.
2. Write the seven missing summaries, so no public row loses its declaration.
3. Add the two Meta fields to the summary parser, with a closed value set for
   enrolment and an error on absent or unrecognized. Refuse rather than default.
4. Derive the enrolment, disposition and status maps from the summaries, and
   point every check and `NewRenderInput` at the derivation. Retire the
   comparison refusals and their fixtures; keep every other refusal and fixture.
5. Add the baseline reader over HEAD's summaries and repoint the ratchet block's
   enrolled-set gate at it, so the four ratchets keep running across the change.
6. Teach `render.go` and `write.go` to emit the three files, and add them to the
   freshness check.
7. Migrate: write the Meta fields on every summary, generate, compare
   byte-for-byte against the authored files, then delete them in the same commit.
8. Edit the site's literal path citations and the documentation pages named
   above, in this same work.

## Goal Validation

One row per goal the Task section states, with evidence rather than an assertion
that the work was done.

| Goal | Evidence |
|------|----------|
| Each fact about an RFC is declared once | `rfc/enrolled.txt`, `rfc/not-enrolled.txt` and `docs/features/rfc-status.md` have no reader. `parseEnrolled`, `loadEnrolled`, `parseDispositions`, `loadDispositions`, `parseStatusLedger` and `loadStatusLedger` are deleted, and `summaryMetas` is the one parse every consumer takes its answer from. `go vet` over the package is the check: a second reader would have to call a function that no longer exists |
| Every other surface derives from that declaration | `LedgerFiles` is the one producer of all three files, `IndexUpdate` writes them, and `rfcFreshnessStage` compares against the same function. `TestHandEditedStatusPageReportsStale` proves a hand edit to any of the three is reported, so a divergence cannot sit unnoticed |
| The migration changes no enrolment and no public claim | Proven AT the migration commit and self-retiring after it, which is what a one-shot proof should do. `TestGeneratedEnrolmentMatchesTheAuthoredFiles` and `TestGeneratedStatusRowsMatchTheAuthoredPage` compare the render against HEAD's AUTHORED files, so both skip once HEAD carries the generated ones -- which is true from `199b684f6` onward. The results they recorded while they could run: `rfc/enrolled.txt` regenerated with a ZERO diff over 181 rows, `rfc/not-enrolled.txt` kept its nine rows and gained nine, and the public page kept all 159 rows with no loss and no gain, four cells differing for reasons named individually in `migrationExempt`. What survives as a LIVE test is the invariant rather than the migration: `TestOneSummaryRendersExactlyOneStatusRow` and `TestAPublicRowCannotBeDeletedWhileItsRFCStaysEnrolled` |
| Three classes of defect become unrepresentable rather than refused | A summary in neither disposition file does not parse (`readEnrolment`); a stem in both cannot be written, because one field holds one value; a disposition naming a summary that does not exist dies with the file that carried it. `TestSurvivingDispositionRefusalsStillFire` proves the eight refusals that are NOT about agreement all still fire |
| The change does not disarm the gate it edits | `TestRatchetsFireWhenEnrolmentMovesToMeta` builds a fixture whose HEAD carries the retired file shape and whose tree carries the Meta shape, and asserts a coverage regression is still reported. Its red phase was forced by breaking `baselineMetasBeforeMigration` |
| The public page is no less honest than before | It is MORE honest, and by a measurable amount. Eight public rows named an RFC with no summary and were therefore outside every check in the package; all eight now have one. RFC 1994's Remaining cell rejoins `checkGapCountAgreement`, whose population is 57 of 159 rather than 56. `./le rfc check` reports zero violations against any of the nine summaries this spec added |

Two goals are NOT validated here, and neither is silently dropped. The site's
published prose is handed to the session that owns those files, with a row in the
deferral shard. `TestSupportedRowsHaveDerivableScope` stays red on a number this
spec did not move.

## Design Insights

- Five of the refusals over these files check that two copies agree. Deleting a
  copy retires a check without weakening anything, which is the only kind of
  simplification that is free.
- The rest state a property of one document, and none of them moves.
- The dangerous half of the change is not the data. It is that the deleting
  commit is the one commit whose baseline cannot be read, and four ratchets are
  gated behind that read.

## Key Design Decisions

- The two facts go into the Meta table rather than into a new per-RFC file,
  because the Meta table is the summary's existing header and a second per-RFC
  file would recreate the problem one directory down.
- The migration is proven by byte-identical generation rather than by a
  transition period, because `ai/rules/no-layering.md` forbids a shape that keeps
  both, and because a ratchet cannot police the commit that changes what it
  reads.
- Every count the generated files state carries its percentage and names its
  denominator (owner directive, 2026-09-01). An absolute number alone reads as
  an achievement: "Gated MUSTs 3,256" is a count of obligations JUDGED and reads
  as a count of obligations MET. The percentage is what makes the population
  visible, and the population is what the reader is actually being told about.
- The seven orphan public rows are answered by writing seven summaries rather
  than by keeping an authored side-file, because a side-file is the second
  declaration this spec exists to remove.

## Known Limitations

- The public page stays 158 KB of authored prose. This spec moves where it is
  authored and does not derive it, because no gate reads more than its numeric
  gap count.
- Thirty-two enrolled RFCs have no public row and stay grandfathered; this spec
  does not write them one.

## RFC Documentation (Scope: tooling)

Not applicable: this spec changes no protocol behavior. It changes where the
ledger records enrolment and the public support claim.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Tests written before the implementation they cover
- [ ] Tests FAIL before the change
- [ ] Tests PASS after it
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
