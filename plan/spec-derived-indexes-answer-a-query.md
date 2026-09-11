# Spec: no gate parses a rendered index, and the RFC ledger leaves git

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `spec-derived-artifacts-are-not-committed`, closed 2026-09-11, which built the `internal/le/derived` registry (`internal/le/derived/derived.go`: `Artifact`, `Register`, `All`) this spec registers into |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-09-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two rendered indexes are read back as a DATA FORMAT by production code, and both
readers are wrong about the population they are reading.

`collectRFC` (`internal/le/testhealth/collect_rfc.go`) reads
`ai/RFC-REQUIREMENTS.md` as text and pulls nine pipe-separated columns out of
each row with a pinned regex. Its own comment records what that already cost: a
share summed from the rendered rollup answered 43.2% where `/quality/rfc-compliance/`
answered 58.1% for the same question. On 2026-09-02 that ONE number moved to the
collector, and every other number kept parsing the render. `internal/le/site/facts.go`
and `internal/le/site/health.go` both reach that parse through `testhealth.Render`,
so a generated markdown page is load-bearing input to two published surfaces.

`loadDocumentIndex` (`internal/le/spec/citation/anchors.go`) parses
`ai/CODE-TO-DOCS.md` back into `map[source][]doc` with `indexRowPattern`, which
matches a TABLE row. `renderCodeIndex` (`internal/le/docstocode/codetodocs_report.go`)
renders a package of at most `namedInline` = 3 files as BULLETS instead, and only
the table form is matched. 674 of the 2,537 entries in the current index are
bullets, so a quarter of the tree is invisible to the reader.

An earlier draft of this spec called that a guard failing open, and it was WRONG.
`AuditAnchors` (`internal/le/spec/citation/anchors.go`) builds `report.Owners`,
the only half `hookValidateSpec` refuses on, from `declaredDesignDocument`, which
opens each source file and reads its own `// Design:` header. The index is never
consulted for `Owners`. It feeds `report.Mentions` alone, which prints as an
advisory `note:`. So the blocking guard has always fired for a file in a small
package, and measured across all 321 specs with the widened reader, 0 newly fail.
What was blind for a quarter of the tree is the advisory half. That is worth
fixing, and it is not a guard that fails open. The error was diagnosing a
consumer by reading the producer of its data, which is `ai/rules/evidence.md`
run backwards.

The same renderer is why a grep is ambiguous. The table form prints the BASENAME
and the bullet form prints the full path, so `grep resolve.go ai/CODE-TO-DOCS.md`
answers with five rows from five packages and no path on any of them, and
`docs/contributing/navigating-the-code.md` has to teach the reader to work out
which package heading each row sat under.

Both defects are the same shape: a model is rendered for a reader, then
re-parsed by a machine that reconstructs what the renderer dropped. Once each
reader reads the model, the RFC ledger family has no reader left that needs it in
git, and it can join `ai/PACKAGE-MAP.md` outside. That family is the larger
churn: 17 of the last 200 commits and all nine rows of
`plan/journal/concurrent-rfc-gate-stale.md`.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/navigating-the-code.md` - the grep contract this changes
  → Constraint: the page tells the reader "rows are keyed by BASENAME, so the package heading
    is what stops a bare `grep peer.go` returning three packages". Full-path keying deletes that
    sentence rather than rewording it
- [ ] `docs/contributing/gh-pages.md` - what the site renders live and what it publishes from bytes
  → Decision: `quality/health/` and `quality/rfc-compliance/` already render from
    `testhealth.Render` and `rfc.Collect`; the page is silent on `reference/rfcs`, which
    `(*docsRenderer).render` publishes from the committed bytes of `docs/features/rfc-status.md`
  → Constraint: the silence is a documentation defect repaired in this work, not reported
- [ ] `docs/architecture/core-design.md` - le's composition
  → Constraint: the derived registry arrives with the preceding spec; this one registers into it
- [ ] `ai/rules/principles.md`
  → Constraint: every fact is declared once and every other surface derives from that
    declaration; a number parsed back out of a generated artifact is a second declaration
  → Constraint: a guard that returns a zero-shaped answer must fail closed, which is what the
    25% blind spot in `loadDocumentIndex` violates

### RFC Summaries (Scope: protocol)
N-A: Scope is tooling. No protocol behavior changes, and no `rfc/short/` summary's `## Meta`
row is edited. The RFC artifacts move in git; the conformance verdicts they render do not move.

**Key insights:** (minimal context to resume after compaction)
- `rowsFrom` (`internal/le/rfc/ledger.go`) already derives every ledger row from each summary's
  `## Meta` table, and its own comment says the row is "never parsed back off the page the render
  writes". `check_status.go` and `check_audit.go` already use it. `collectRFC` is the outlier.
- `rfc.NewRenderInput` is assembled once and shared by the writer and the freshness checker so
  both produce identical bytes, so a second consumer of the model costs one call, not a new walk.
- `test/health/history.ndjson`, `quality-baseline.json` and `sensitivity-baseline.json` are
  committed RECORDS, not derivations. They stay tracked. So does `test/health/latest.json`,
  whose comparison in `testhealth.Check` is the sensor-rot detector.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/testhealth/collect_rfc.go` - `collectRFC` reads `rfcLedger` as text, pins
      `rfcTableHeader`, and regexes nine columns per row with `rfcRow`. It refuses a missing
      header, a zero enrolled population and a zero gated count, each with a comment saying why
- [ ] `internal/le/rfc/ledger.go` - `rowsFrom` derives the same rows from the summaries
- [ ] `internal/le/spec/citation/anchors.go` - `loadDocumentIndex` matches `indexSectionPattern`
      and `indexRowPattern`, then joins heading to basename; `AuditAnchors` fails closed on an
      empty map
- [ ] `internal/le/docstocode/codetodocs_report.go` - `renderCodeIndex` prints the full path in
      the bullet form and the basename in the table form
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookValidateSpec` is the guard that consumes it
- [ ] `internal/le/site/docs.go` - `(*docsRenderer).render` reads `page.Source` from disk
- [ ] `internal/le/site/docsmanifest.go` - the `docsManifest` row for `docs/features/rfc-status.md`
- [ ] `internal/le/rfc/check_extraction.go` - the ledger and shard byte comparisons
- [ ] `internal/le/doc/wiring/docverify.go` - `rfcFreshnessStage` and the `LedgerPaths()` loop

**Behavior to preserve:**
- Every published number keeps its current value. This spec changes where a number is read
  from, never what it is, so the health page and the compliance page must answer identically
  before and after
- `collectRFC` keeps every refusal it has: a changed ledger shape, an empty enrolled population
  and a zero gated count each stay loud
- `./le rfc check` keeps every verdict that is not a byte comparison against a committed copy
- `test/health/latest.json` and the three baselines stay tracked

**Behavior to change:**
- `collectRFC` derives from the RFC model, not from the rendered ledger
- `loadDocumentIndex` reads the docs-to-code model, so the anchor guard sees every code path
- `renderCodeIndex` prints full paths in both shapes
- `reference/rfcs` is rendered from the RFC model rather than published from committed bytes
- `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/*.md`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt`
  and `docs/features/rfc-status.md` leave git and register as derived artifacts

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test-health update` and `check`, and the site build calling `testhealth.Render`
- A spec write, through `hookValidateSpec`
- `./le site build`

### Transformation Path
1. `rfc.Collect` and `NewRenderInput` answer the model once
2. `testhealth` reads the coverage rows off that model instead of off the rendered page
3. `docstocode.buildCodeIndex` answers the model; `citation` reads it instead of the markdown
4. `renderCodeIndex` keeps rendering for the human and the grep, with full paths in both shapes
5. The RFC artifacts register in `derived`, so they are rebuilt on read and removed on write

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| testhealth → rfc | a Go call answering typed rows | No |
| citation → docstocode | a Go call answering `map[path][]doc` | No |
| site → rfc | a Go call answering the status page's rows | No |

### Integration Points
- `internal/le/rfc` gains an exported accessor for the coverage rows, if `rowsFrom` is unexported
- `internal/le/docstocode` gains an exported accessor for the code index model
- `internal/le/site/docsmanifest.go` loses the `rfc-status` row, or gains a live renderer beside
  `renderHealth`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | each reader calls the producing package rather than its output file |
| No unintended coupling (components stay isolated) | No | `testhealth` already imports `rfc`; `citation` already imports nothing of `docstocode` and will |
| No duplicated functionality (extends existing, does not recreate) | No | `rowsFrom` and `buildCodeIndex` exist; neither is re-implemented |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A: no wire or buffer path |
| Registration over hardcoding, outbound | No | the five RFC artifacts register in `derived` |
| Registration over hardcoding, inbound | No | no list learns their names; `docsManifest`'s count assertion is the one place that must change, and it changes because a row leaves |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The model answers every column `collectRFC` parses today: gated, both, one-polarity, annotated, no-test, outstanding, nightly-only, state | `RenderIndex` renders those columns from `RenderInput`, so the values exist before the render | the health page loses a metric and the change is a regression wearing a refactor's clothes | a test asserting the collector's output is byte-identical before and after, over the current tree | unvalidated |
| A-2 | `rowsFrom`'s enrolment marker and the ledger's `**enrolled**` State cell mean the same population | `collect_rfc.go` reads enrolment from the ledger ROW deliberately, with a comment saying a second source is how the two diverge | the enrolled population shifts silently and every share moves with it | compare the enrolled count from both routes over the current tree, and assert equality in a test | unvalidated |
| A-3 | Nothing else parses `ai/RFC-REQUIREMENTS.md` | the breakage sweep found `collectRFC` and its two site consumers | the untracking breaks a surface this spec did not name | `gopls references` plus a grep for the path across `internal/`, `.github/` and `ai/skills/` | unvalidated |
| A-4 | `reference/rfcs` can be rendered from the model, as `quality/rfc-compliance/` already is | `renderStatusPage` (`internal/le/rfc/render_ledger.go`) is the producer of the page's bytes | the page must stay published from committed bytes, and `docs/features/rfc-status.md` stays tracked | build the site with the file absent and diff the output against the current build | unvalidated |
| A-5 | The 631 bullet entries are the whole blind spot, and no other render shape exists | `renderCodeIndex` has exactly two shapes, switched on `namedInline` | the guard is still open after the fix | a test asserting the parsed model's path count equals the generator's `len(index.Refs)` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Widening the anchor guard from 1,902 paths to 2,533 makes it fire on specs it never judged, and every spec in flight across the checkout starts failing `hookValidateSpec` | other sessions report spec writes refused with "Design document(s) declared by this spec's own code, never named in it" | this is the guard doing its job, but it lands across sessions at once. Land the widening in its own commit, and say so in the message so a session meeting it knows what changed |
| R-2 | The published health numbers move because the model and the render genuinely disagree, and the render was right | the before/after comparison in A-1 differs | STOP and report which number moved and by how much. A number that moves is a finding about the ledger, not a detail of this refactor |
| R-3 | The site build breaks for a page nobody tests | `./le site build` fails after the `docsManifest` row leaves | the deliverables checklist runs a full site build before the review round |
| R-4 | Untracking `rfc/requirements/*.md` orphans a verification-debt row naming the RFC freshness gate | a debt row that no green verify can clear | grep `plan/verification-debt/` for the gate names before deleting them |
| R-5 | Another session is mid-RFC-work when the family leaves git, and its uncommitted shard edits become untracked file edits | that session's commit no longer carries the shards it expected to | the shards are derived from `rfc/short/` summaries, which stay tracked, so no authored text is lost. Say so in the commit message |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The published quality pages carry a wrong number, or the site build fails. No operator-facing daemon behavior changes |
| How is it reverted? | Single commit revert for the reader moves. The untracking is a second commit and reverts on its own |
| Who else touches this path? | Every session doing RFC work touches `rfc/short/` and the shards; the RFC gate is the most contended surface in the repository |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test-health check` with `ai/RFC-REQUIREMENTS.md` absent | → | `collectRFC` reading the model | `TestTestHealthAnswersWithNoRenderedLedgerPresent` |
| a spec write naming a file in a package of three or fewer files | → | `hookValidateSpec` → `AuditAnchors` → `loadDocumentIndex` | `TestTheAnchorGuardSeesAFileInASmallPackage` |
| `./le site build` with `docs/features/rfc-status.md` absent | → | the live renderer for `reference/rfcs` | `TestTheRFCStatusPageRendersWithoutItsCommittedCopy` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le test-health update` over the current tree, before and after the reader change | every metric answers the identical value; the page bytes are unchanged |
| AC-2 | `ai/RFC-REQUIREMENTS.md` absent | `./le test-health check`, `./le site facts update` and `./le site build` all succeed |
| AC-3 | `grep -c` for the pinned ledger header anywhere under `internal/` | answers zero: no code re-parses the rendered ledger |
| AC-4 | the code index model parsed by `citation` | holds one entry per code path the generator holds: 2,533 today, not 1,902 |
| AC-5 | a spec naming a source file that lives in a package of three or fewer files, whose design doc the spec does not name | the spec write is refused, naming that document |
| AC-6 | `ai/CODE-TO-DOCS.md` after regeneration | every row carries the full path from the checkout root, in both the bullet and the table shape |
| AC-7 | `git ls-files` for the five RFC artifacts | answers nothing, and each is reported ignored by `git check-ignore` |
| AC-8 | a commit that edits an `rfc/short/` summary and carries no regenerated shard | is prepared and runs, with no refusal and no debt row |
| AC-9 | `./le verify current mode full` | carries no stage whose only verdict is a byte comparison of an RFC artifact against its committed copy |
| AC-10 | `./le rfc check` | still reports coverage, evidence strength, public status, audit verdicts and extraction sign-off |
| AC-11 | the site's `reference/rfcs` page | renders with the same content as the current build, from the model |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | adds a tagged test, commits it, and never regenerates an index | write → the RFC artifacts are invalidated → `./le rfc check` rebuilds and judges from the tree | `TestATaggedTestNeedsNoLedgerRegeneration` |
| 2 | reads the published RFC status page on the website | `./le site build` → the live renderer → `rfc.NewRenderInput` | `TestTheRFCStatusPageRendersWithoutItsCommittedCopy` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTheCollectorAnswersTheSameRowsAsTheRenderedLedger` | `internal/le/testhealth/collect_rfc_test.go` | A-1 and A-2: the model and the render agree, column by column, over the real tree | |
| `TestTestHealthAnswersWithNoRenderedLedgerPresent` | `internal/le/testhealth/collect_rfc_test.go` | the reader no longer needs the file | |
| `TestTheCollectorStillRefusesAnEmptyEnrolledPopulation` | `internal/le/testhealth/collect_rfc_test.go` | every refusal survived the move | |
| `TestTheDocumentIndexHoldsEveryCodePath` | `internal/le/spec/citation/anchors_test.go` | AC-4: parsed count equals the generator's count | |
| `TestTheAnchorGuardSeesAFileInASmallPackage` | `internal/le/spec/citation/anchors_test.go` | AC-5: the guard that failed open now fires | |
| `TestTheCodeIndexRendersFullPathsInBothShapes` | `internal/le/docstocode/codetodocs_report_test.go` | AC-6 | |
| `TestTheRFCStatusPageRendersWithoutItsCommittedCopy` | `internal/le/site/docs_test.go` | AC-11 | |
| `TestATaggedTestNeedsNoLedgerRegeneration` | `internal/le/commit/commit_test.go` | AC-8 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `namedInline` (files in a package before the render switches shape) | 0-n | 3 | N-A | N-A: both shapes must now carry the full path, so the boundary stops changing meaning. The test asserts a package of exactly 3 and one of exactly 4 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-rfc-ledger-is-derived` | `test/runner/le-rfc-ledger-is-derived.ci` | a user adds a tagged test and runs the RFC gate without regenerating anything | green, 775ms, in `./le functional runner` (13 of 13) |

### Interop Tests (Scope: protocol)
N-A: Scope is tooling. No wire-visible behavior changes.

## Files to Modify
- `internal/le/testhealth/collect_rfc.go` - read the model
- `internal/le/testhealth/testhealth.go` - the pinned header, `rfcRow`, `rfcStateEnrolled` go with it
- `internal/le/rfc/ledger.go` - export the coverage rows if `rowsFrom` is unexported
- `internal/le/spec/citation/anchors.go` - `loadDocumentIndex` reads the model
- `internal/le/docstocode/codetodocs_report.go` - full paths in the table shape
- `internal/le/docstocode/codetodocs.go` - export the code index model
- `internal/le/site/docs.go`, `internal/le/site/docsmanifest.go` - the `rfc-status` row
- `internal/le/site/redirect.go`, `llmsdata.go`, `docs_test.go` - the manifest count
- `internal/le/rfc/check_extraction.go` - delete the ledger and shard byte comparisons
- `internal/le/doc/wiring/docverify.go` - delete `rfcFreshnessStage` and the `LedgerPaths()` loop
- `internal/le/rfc/register.go` - register the five artifacts in `derived`
- `internal/le/changed/selector.go` - the comment naming `enrolled.txt`
- `.gitignore` - the five paths
- `ai/rules/points/testing/rfc-tagged-tests-blocking/reindex-after-moving-a-tagged-test.md` - the
  rule point requiring both outputs in the same commit, then `./le rules render-update`
- `ai/skills/ze-rfc.md`, `ai/skills/ze-rfc-audit.md` - the regenerate-and-commit instructions,
  then `./le ai skills-sync`
- `docs/contributing/rfc-conformance-gates.md` - the "must commit" framing
- `docs/contributing/gh-pages.md` - say how `reference/rfcs` is produced
- `docs/contributing/navigating-the-code.md` - delete the basename warning
- `test/health/README.md` - which artifacts are committed records and which are derived
- `docs/architecture/testing/test-health.md` - declared by `internal/le/testhealth/collect_rfc.go`
  and `testhealth.go`: the RFC rollup's source changes from the rendered ledger to the model
- `docs/architecture/testing/verify-freshness-scope.md` - declared by `internal/le/changed/selector.go`:
  the RFC corpus prefix no longer selects a committed ledger
- `website/AI.md` - declared by `internal/le/site/docs.go` and `redirect.go`: the manifest loses a
  page that is now rendered live

## Files to Create
- `test/runner/le-rfc-ledger-is-derived.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | developer tooling, no operator config |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | No | no verb is added; two check verbs lose an arm |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | N-A | no YANG leaf |
| Functional test for new RPC/API | Yes | `test/runner/le-rfc-ledger-is-derived.ci` |
| Pipe completeness | N-A | report shapes unchanged |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no daemon runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | Scope is tooling |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | developer tooling |
| 2 | Config syntax changed? | N-A | no config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` if it names the deleted check arms |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | Yes | `docs/contributing/navigating-the-code.md`, `docs/contributing/rfc-conformance-gates.md` |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no `rfc/short/` `## Meta` row changes; the artifacts move in git, the verdicts do not move |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `test/health/README.md` |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/contributing/gh-pages.md` |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, command or inventory changed? | Yes | the `derived` registrations; `docs/guide/status.md` if it enumerates verbs |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-derived-indexes-answer-a-query.md`, run before the review round |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/skills/ze-rfc.md` shows the regenerate-and-commit workflow |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the model is reachable from both readers
   - Tests: `TestTheCollectorAnswersTheSameRowsAsTheRenderedLedger`, `TestTheDocumentIndexHoldsEveryCodePath`
   - Files: `internal/le/rfc/ledger.go`, `internal/le/docstocode/codetodocs.go`
   - Verify: both tests fail first because the accessors do not exist, and the equality test is
     what proves the model and the render agree before anything is switched over
2. **Phase: testhealth reads the model**
   - Tests: `TestTestHealthAnswersWithNoRenderedLedgerPresent`, `TestTheCollectorStillRefusesAnEmptyEnrolledPopulation`
   - Files: `internal/le/testhealth/collect_rfc.go`, `testhealth.go`
   - Verify: AC-1 holds, byte for byte, over the current tree
3. **Phase: the anchor guard sees every path** -- ITS OWN COMMIT (R-1)
   - Tests: `TestTheAnchorGuardSeesAFileInASmallPackage`, `TestTheCodeIndexRendersFullPathsInBothShapes`
   - Files: `internal/le/spec/citation/anchors.go`, `internal/le/docstocode/codetodocs_report.go`
   - Verify: the guard fires on a case it silently passed before; the commit message says so,
     because every session in this checkout meets the widened guard at once
4. **Phase: the site renders the status page**
   - Tests: `TestTheRFCStatusPageRendersWithoutItsCommittedCopy`
   - Files: `internal/le/site/docs.go`, `docsmanifest.go`, and the three count assertions
   - Verify: a full `./le site build` diff against the current build is empty
5. **Phase: the RFC family leaves git, in ONE commit with the ignore entries**
   - Tests: `TestATaggedTestNeedsNoLedgerRegeneration`, `test/runner/le-rfc-ledger-is-derived.ci`
   - Files: `.gitignore`, `internal/le/rfc/register.go`, `check_extraction.go`, `docverify.go`
   - Verify: grep `plan/verification-debt/` for the RFC gate names first; `git check-ignore -v`
     names each line afterwards
6. **Phase: Documentation**
   - Files: every row of the Documentation Update Checklist answered Yes, plus the rule point and
     the two skills, each followed by its generator (`./le rules render-update`, `./le ai skills-sync`)
   - Verify: `./le spec citation anchors spec plan/spec-derived-indexes-answer-a-query.md` is clean

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation at file:line |
| Guard widening | the anchor guard's population goes from 1,902 to the generator's full count, and a test proves a previously invisible case now fires |
| No number moved | the before/after equality test is over the REAL tree, not a fixture, because a fixture cannot show a divergence between two real derivations |
| Fail-closed | every refusal `collectRFC` carried is present after the move, and each still names why a zero is not an answer |
| Data flow | no reader opens a generated markdown file; `grep -rn "RFC-REQUIREMENTS" internal/` finds only the writer and the registration |
| Rule: `ai/rules/rfc-compliance.md` | no requirement's classification, level or evidence changes. This spec moves artifacts in git and must not touch a verdict |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no code parses the rendered ledger | `grep -rn "rfcTableHeader\|rfcRow" internal/` is empty |
| the anchor guard's population | the test asserting parsed count equals `len(index.Refs)` passes |
| the five artifacts untracked | `git ls-files ai/RFC-REQUIREMENTS.md rfc/requirements rfc/enrolled.txt rfc/not-enrolled.txt docs/features/rfc-status.md` is empty |
| the site is unchanged | a full `./le site build` diffed against the pre-change build |
| the published numbers are unchanged | `./le test-health update` produces no diff in the page's metric values |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Guard integrity | widening a guard must not introduce a path where it silently passes: an error from the model must refuse, never answer an empty population |
| Published surface | the RFC status page is Ze's public standards claim; a rendering change must not drop a row or soften a state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| A published number moves | STOP. Report which, by how much, and which route is right. Do not pick |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Readers call the producing package | keep parsing but pin the format with a round-trip test | a round-trip test proves the parse matches the render, and both can be wrong about the model. The 25% bullet blind spot is exactly that: a parser and a renderer that agreed with each other and not with the data |
| No point-query CLI verb is added | `./le docs-to-code for-file <path>`, `./le rfc requirements for <stem>` | once rows carry full paths a grep is exact, and once the in-repo readers call Go the verbs have no caller. An abstraction with no user is what `ai/rules/simplicity.md` cuts |
| Full paths in both render shapes | teach the parser to read bullets too | the parser is being deleted, and the ambiguity the basename creates is paid by every agent that greps. Fixing the render fixes both readers at once |
| The anchor-guard widening is its own commit | fold it into the reader change | it changes the verdict for every spec in a shared checkout at once, so it owes its own message and its own revert |

## Known Limitations
- `website/data/repo-facts.json` stays tracked. It is a snapshot of git index state that a build
  cannot re-derive, and `docs/architecture/site-facts.md` argues for committing it.
- The architecture lists `./le arch-map update` writes into `ai/INSTRUCTIONS.md` stay tracked:
  they are marker-bounded blocks inside authored prose, so the file is not a derived artifact.
- `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md`, `ai/rules/INDEX.md` and `docs/features/test-health.md`
  stay tracked: zero commits each in the last 200, and `CLAUDE.md` resolves `@ai/rules/CORE.md`
  before any hook could rebuild it.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
