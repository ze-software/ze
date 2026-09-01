# Spec: publish the RFC requirement ledger

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 7/7 |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-09-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The website publishes RFC conformance only as aggregates. `/quality/rfc-compliance/`
shows five headline cards and rollup counts over 4,852 requirements, and
`/reference/rfcs/` is a 159 KB mirror of the hand-authored `docs/features/rfc-status.md`.
Neither one names a single requirement. A reader is told that 80 RFCs carry gaps and is
given no way to learn which requirement, which test, or which verdict.

The owner's instruction: "./quality/rfc-compliance seems to not have any of the details
we keep in this repo, we should publish details like we do for commands." The commands
family he names is `/reference/command-equivalents/`, 411 generated pages, one per
command.

The goal is a generated detail-page family, one page per summary stem in `rfc/short/`,
carrying the per-requirement ledger this repository already computes: the requirement
text, its level and section, the tests bound to it and the carrier that runs them, its
annotation, its audit verdict and freshness, its discrimination record, and the
extraction sign-off that decided which sentences became requirements at all.

**This spec publishes what the repository already computes.** It changes no conformance
verdict, adds no gate, and fixes no `{gap}`.

### Owner ruling: a ratio leads, and its denominator is what binds Ze (2026-09-01)

The page led with `Gated MUSTs 3,256`. That is a count of obligations JUDGED and it reads
as a count of obligations MET, and 834 of it are `{not-applicable}`: obligations that
never bound Ze. The owner's words: "so we claim RFC 3k .. looks impressive then we look
and see only half covered. This is deceptive."

The rules below bind the index page and every per-RFC page alike. Four amendments
followed the first ruling within the hour, and each SUPERSEDES what it replaces rather
than sitting beside it. Every row is dated 2026-09-01.

| # | Rule | Why | Supersedes |
|---|------|-----|------------|
| 1 | Every ratio's denominator is the obligations that BIND Ze: the gated population less the `{not-applicable}` set | An obligation that never bound Ze is scope, not coverage, and carrying it in the denominator flatters every share above it | - |
| 2 | The proof ratio sits immediately after the shares it corrects | A test pair is not a proof, and the gate says so: a tagged unit is proven when a recorded break has been observed to redden it | - |
| 3 | The ratio cards PARTITION their denominator: every binding bucket is in exactly one card, so the shares add to 100% | Publishing two parts of a four-part split left 3.3% of RFC 4271 unexplained and no way to learn where it went. The report was incomplete, not wrong | - |
| 4 | SCALE leads, then STANDING: `Gated MUSTs` and `Out of scope` open the grid, the three partition shares follow, then the proof ratio | Owner amendment: "GATED MUSTS should also probably be the first number." Safe ONLY while the population is unmistakably labeled as scale, carries the neutral tone and the sentence saying a population is not a result, and the shares sit in the same grid immediately after it | The ratio-first order of the first ruling |
| 5 | A card's color names what its measure MEANS, never how well Ze scores on it. Green is a good outcome at any value, red a bad one above zero, and neither a population nor a scope count is an outcome | Owner amendment: "TESTED BOTH WAYS color MUST be green." The NUMBER under the label already carries the performance; a color that graded as well as labeled made a reader decode two scales at once, and it put an amber card on the measure that IS the good news | The threshold tones of AC-34's first form |
| 6 | Each card carries the percentage AND the arithmetic behind it, the count under the value, in a grid of equal cards | A percentage whose numerator and denominator live in a sentence beside it is a figure the reader has to assemble | - |
| 7 | The proof card states that the break is NOT re-run: a red observed once under a recorded procedure, whose unit, claim and producer still hash to what was recorded. A record whose ground moved is a LAPSED proof and is not counted as one | Owner question: "You do that by editing the code during code creation can you really report on it?" `verifyOneDiscrimination` (`internal/le/rfc/discriminate.go:608`) re-checks every record against the tree on each run, and answers `unit-changed`, `claim-changed`, `producer-changed`, `unit-gone`, `tag-gone` or `citation-gone` when anything moved. Reporting a lapsed record as a proof would publish a red nobody has seen against bytes nobody has checked | - |

This is the disclosure ruling applied to PRESENTATION rather than to content. Nothing was
hidden before these changes; the arrangement flattered the result, which is the same
defect one layer up.

Rule 5 is a reading the implementation states so it can be overturned in one line. Its
hardest case is the proof ratio: it is a good outcome by the rule, so it is green, and
2.7% in green looks odd. The card says the number in its own sentence rather than in its
color, which is what rule 5 asks for. The one card where the rule costs signal is
`Semantic verdicts`: its value is a count of judgements recorded, which is a scale rather
than an outcome, so it takes the neutral tone and the stale and shifted counts sit in its
own sentence and on each RFC's page under the requirement id.

### Owner ruling: disclosure is FULL (2026-09-01)

The owner confirmed the disclosure scope. It is a ruling, not an assumption this spec
took, and it is not open for the implementation to reduce.

The public per-RFC page names, per requirement:

| State published | Never |
|-----------------|-------|
| A gated MUST with no test bound to it | Omitted, or shown as a blank cell |
| An audit verdict of `weak`, `wrong` or `unimplemented` | Softened, relabeled, or folded into an "audited" count |
| A verdict whose freshness is `stale-unit`, `stale-requirement` or `shifted` | Reported as fresh, or reported as merely "audited" |
| A tagged unit with no discrimination record | Counted as proven |
| A `no-break` discrimination record | Counted as a proof |

**A bad state MUST NOT be aggregated into a count where the requirement id is
available.** A page that says "6 requirements carry a weak verdict" and does not name
the six has reproduced the aggregate this work exists to replace. A count may accompany
the list; it never stands in for it. AC-17 and AC-18 carry this, and the argument is the
one that motivates a public ledger at all: the claim becomes checkable from outside, and
a claim whose failing rows are hidden is not checkable.

### Gates not held

`/ze-spec` runs its SCOPE, RESEARCH and DESIGN gates through `AskUserQuestion` in the
main thread. This spec was written in a subagent, which can hold no dialogue, so the
three gates are unrun. The Open Decisions table below carries what each gate would have
asked. Every row has a stated recommendation and a reason, so the spec is implementable
as written and each row is an override rather than a blocker.

## Required Reading

### Architecture Docs
- [ ] `website/AI.md` - the site package's own design doc, declared by every producer file
  → Constraint: every published route is written by exactly ONE named producer; a route
    two producers write, and a route no producer wrote, are both red in `Coverage`.
  → Constraint: the "Website architecture" section describes `ResolvePaths` and
    `IsSourceOnly` as exported. Both are unexported in `internal/le/site/paths.go`, where
    they are spelled `resolvePaths` and `isSourceOnly`. The same section names one of the
    four failure modes `./le site check` has, and the page never mentions the producer
    registry or `Coverage`. This work adds a producer, so the page is edited in the same
    change.
- [ ] `docs/contributing/gh-pages.md` - the operator-facing account of the site build
  → Constraint: it carries a "Quality pages" section describing `/quality/rfc-compliance/`.
    The new family is a child of that route, so its account belongs in that section.
- [ ] `docs/architecture/core-design.md` - declared by the `// Design:` header of
  `internal/le/rfc/coverage.go`, `rfc.go`, `discriminate.go` and `sections.go`, the files
  this work edits
  → Decision: the page describes the rfc area AS ONE COMMAND. This work adds exported
    symbols to that package and changes no verb, no argument, no output and no gate, so
    the page stays correct and is named here as unaffected rather than edited. If the
    export phase ends up moving a command surface, that is the signal the page is owed an
    edit in the same change (`ai/rules/documentation.md`).
- [ ] `docs/contributing/rfc-conformance-gates.md` - the gate this ledger publishes
  → Constraint: the discrimination record is the proof behind a tag's PROSE claim, and
    `no-break` is an ESCAPE counted apart from the two proof routes. A page that folded
    the escape into "proven" would publish the opposite of what the gate holds.
- [ ] `ai/rules/principles.md` - declare once, derive everywhere
  → Constraint: the new pages must not re-parse markdown `internal/le/rfc` already parses,
    and must not recompute a bucket `internal/le/rfc/coverage.go` already computes. Where
    the package holds the fact privately, the package exports it; the site package never
    grows a second copy.
- [ ] `ai/rules/rfc-compliance.md` - what a gap and an exclusion each mean
  → Constraint: a `{gap}` is an ISSUE and an `excluded` site is a DECISION. The page must
    render them apart, and must never let an exclusion read as coverage.
- [ ] `ai/rules/simplicity.md` - the simplest fully correct answer
  → Decision: the route choice below is settled by a mechanical fact rather than by taste.

### RFC Summaries (Scope: protocol)
Not applicable. This spec publishes the conformance ledger and implements no protocol
behavior.

**Key insights:** (minimal context to resume after compaction)
- One producer owns `/quality/rfc-compliance/` today and can own the whole family, so no
  route is doubled and no navigation entry is added.
- 39 of the 181 summary stems have NO row in `docs/features/rfc-status.md`, so that page
  cannot serve as the family index. This is the fact that settles the route.
- `rfc.Collect` plus `rfc.NewRenderInput` is one whole reading of the ledger. `rfc.Check`
  is a second full pass that also type-checks every tagged package, and the detail pages
  must not call it.
- Two exported fields of `rfc.RenderInput` have unexported types, so no other package can
  read them today. Both carry facts the pages need.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/site/producer.go` - `Producer{Name, Render func(Paths) ([]string, error)}`.
  `Render` writes pages and ANSWERS the route of each one, spelled with a leading and a
  trailing slash. `registerProducer` runs from the owning file's `init()`. `checkProducer`
  panics on an empty name, a nil `Render`, or a duplicate name. `Coverage` compares the
  answered routes against the published tree and is red on an unclaimed or a doubled route.
  `writeNamedArtifact` writes a published file that is not a route, and `namedArtifacts`
  is the list `checkNamedArtifacts` holds it against.
- [ ] `internal/le/site/plugins.go`, `internal/le/site/plugindetail.go` - the detail-family
  pattern this work follows. `renderPluginCatalog` reads `data/plugin-registry.json` through
  `readArtifactJSON`, refuses an input that names no plugin, calls `removeRetiredPluginPages`
  to delete the directory of a plugin the input no longer carries, writes the index, loops
  `writePluginDetailPage`, and returns 94 routes. `writePluginDetailPage` builds the route
  and the destination from a per-family prefix constant, and `writePublishedPage`
  (`internal/le/site/catalog.go`) writes `index.html` and `index.md` together.
- [ ] `internal/le/site/rfccompliance.go` - the aggregate page. `renderRFCCompliance`
  derives one `rfcCompliance` snapshot through `liveRFCCompliance`, publishes it as
  `data/rfc-compliance.json` (4.5 KB), and returns one route. `collectRFCCompliance`
  calls `rfc.Collect`, then `rfc.NewRenderInput`, then `rfc.Check`. `rfcDisplayName`
  fakes a printable name from the stem because nothing carries the RFC's title.
  `rfcSatisfaction` and `rfcAnnotationBuckets` spell the annotation-kind vocabulary as
  string literals, because `internal/le/rfc` does not export it.
- [ ] `internal/le/site/build.go` - `refreshNativeSurfaces` runs BEFORE the producer
  registry and republishes each derived input a producer then reads:
  `publishCommandCatalog`, `publishPluginRegistry`, `publishYANGConfigTree`,
  `publishSiteFacts`, the CSS and JS bundles, and the talk decks.
- [ ] `internal/le/site/llmsfull.go` - `assignPages` FAILS THE BUILD on a published page
  no navigation section claims. `sectionClaimants` gives a navigation entry a claim over
  the route it points at AND every route under it, so a child of an existing entry needs
  no navigation edit. `website/data/nav.json` carries an entry whose `href` is
  `quality/rfc-compliance/`.
- [ ] `internal/le/site/docs.go`, `internal/le/site/docsmanifest.go` - the `docs` producer
  writes `/reference/rfcs/` from `docs/features/rfc-status.md` through the
  `docsDestinationExact` map. The HTML body and the Markdown mirror are produced from two
  different strings, `marked` and `plain`, and the body-only transforms
  (`relayoutEvidenceCells`, `colorCodeCells`) therefore reach the HTML alone.
- [ ] `internal/le/rfc/rfc.go` - `Requirement{RFC,RID,Level,Text,Section,Source,Line,Ticked}`
  plus `Gated()`, `Annotation{Kind,Polarity,Reason}`, `Successor{Disposition,Target,Reason}`,
  `Tag{RID,Polarity,File,Line,Claim}`, `Prefix`. The polarity literals and
  `annotationKindNames` are private, while the sibling vocabularies `ExclusionKinds`,
  `SiteDispositions`, `SectionDispositions`, `SectionSkipKinds` and `FreshnessStates` are
  exported.
- [ ] `internal/le/rfc/render.go` - `RenderInput` carries `Requirements`, `Tags`, `Enrolled`,
  `Stems`, `Carriers`, `Rows`, `Dispositions`, `Successors`, `Audits`, `States`, `Covers`,
  `Discrimination`, `Unscanned`. `Covers` and `Discrimination` are exported fields whose
  types, `cover` and `discriminationVerdict`, are unexported, so neither can be read from
  another package. `shardRow` renders the six-column row `rfc/requirements/<stem>.md`
  carries.
- [ ] `internal/le/rfc/coverage.go` - `Carrier.Label` is exported. `carrierFor`,
  `evidenceLabel`, `evidenceTier`, `isNightlyOnly`, `rfcCoverage` and `rfcCoverageRows`
  are private, which is why `rfccompliance.go` recomputes its own buckets.
- [ ] `internal/le/rfc/audit.go` - `Audit.Verdict` answers `map[string]any`. The five
  verdict words and the six verdict keys are private constants.
- [ ] `internal/le/rfc/summary.go` - `parseEnrolled` keeps the first field of the row and
  discards the rest, which is the enrolment reason.
- [ ] `internal/le/rfc/ledger.go` - `parseStatusLedger` keys on the first cell and reads
  the third, fourth and fifth. The second cell, the Area, is discarded. `parseDispositions`
  reads `rfc/not-enrolled.txt` into `{stem: Disposition{Kind, Reason}}`, so the declined
  half IS structured.
- [ ] `internal/le/rfc/artifact.go` - `Extraction`, `ExtractionSection`, `ExtractionSite`,
  `LoadExtractions` and `ExclusionKinds` are exported and usable as they stand.
- [ ] `internal/le/rfc/discriminate.go` - `DiscriminationRecord` is exported.
  `discriminationVerdict`, `cover` and the three proof states are not.

**Repository scale, measured 2026-09-01:**

| Fact | Count |
|------|-------|
| Summaries in `rfc/short/` | 181 |
| Enrolled stems (`rfc/enrolled.txt`) | 173 |
| Declined stems (`rfc/not-enrolled.txt`) | 8 |
| Requirement shards in `rfc/requirements/` | 180 |
| Summaries carrying a Meta `\| Title \|` row | 159 |
| Rows `parseStatusLedger` keys out of `docs/features/rfc-status.md` | 158 |
| Enrolled stems with NO row in that ledger | 32 |
| Declined stems with NO row in that ledger | 7 |
| `rfc/audit/*.json` files | 1 (`rfc7606.json`) |
| `rfc/discrimination/*.json` files | 5 |
| `rfc/extraction/*.json` files | 45 |
| Requirement rows in the largest shard (`rfc7296`) | 228 |
| Bytes in the largest shard | 76,537 |
| Bytes across all shards | 1,323,373 |
| Published `llms-full.txt` today | 4,296,302 |
| Published `data/search-index.json` today | 1,926,916 |
| Published `/reference/rfcs/index.md` today | 159,498 |
| Published `/quality/rfc-compliance/index.md` today | 4,036 |

**Behavior to preserve:**
- `/quality/rfc-compliance/` keeps every section and every number it publishes today, and
  `data/rfc-compliance.json` keeps its schema. The frozen-page tests in
  `internal/le/site/rfccompliance_test.go` stay green.
- `/reference/rfcs/` keeps being written by the `docs` producer from
  `docs/features/rfc-status.md`.
- `rfc/requirements/<stem>.md` and `ai/RFC-REQUIREMENTS.md` keep their exact bytes.
  `./le rfc index-update` and `./le rfc check` are untouched.
- Every existing symbol in `internal/le/rfc` keeps its signature. This work adds exports;
  it renames and removes nothing.

**Behavior to change:**
- `/quality/rfc-compliance/` gains two link tables, one for the enrolled stems and one for
  the declined stems.
- 190 new routes are published, one per summary, and the index answers 191 in total.
- `parseEnrolled` keeps the enrolment reason instead of discarding it.
- A summary's Meta `| Title |` row becomes a parsed fact, and the 22 summaries with no such
  row gain one. 21 of them carried no Meta table at all and gained one carrying the RFC
  number and the title; `rfc7871` had a Meta section written as a bullet list and gained
  the table under it.

## Data Flow (MANDATORY)

### Entry Point
- `./le site build`, which runs `refreshNativeSurfaces` and then the producer registry.
- Inputs at entry: `rfc/short/*.md` (181 markdown summaries), `rfc/enrolled.txt`,
  `rfc/not-enrolled.txt`, `docs/features/rfc-status.md`, `rfc/audit/*.json`,
  `rfc/discrimination/*.json`, `rfc/extraction/*.json`, and the `RFC requirement:` tags
  scanned out of the test tree.

### Transformation Path
1. `refreshNativeSurfaces` calls a new `publishRFCLedger`, which reads the checkout once
   through `rfc.Collect` and then `rfc.NewRenderInput`, and never through `rfc.Check`.
2. `publishRFCLedger` flattens that reading into one snapshot value and writes it with
   `writeNamedArtifact` as `data/rfc-requirements.json`. The path joins `namedArtifacts`.
3. The `rfc-compliance` producer reads that file back with `readArtifactJSON`. It refuses
   an input naming no stem, by name.
4. It deletes the directory of any stem the input no longer carries.
5. It writes the index at `/quality/rfc-compliance/` (the aggregate page it already writes,
   plus the two link tables) and one page per stem at `/quality/rfc-compliance/<stem>/`,
   each through `writePublishedPage`, which writes `index.html` and `index.md` together.
6. It answers 182 routes.
7. The derived producers then run over the finished artifact: `search` and `seo` walk every
   mirror and need nothing, and `llms-full` claims the new routes through the existing
   `/quality/rfc-compliance/` navigation entry.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Checkout to snapshot | `rfc.Collect` + `rfc.NewRenderInput` + `rfc.LoadExtractions` in `collectRequirementLedger`, JSON out | Yes: `TestABuildPublishesTheRequirementLedger`, and `TestARequirementRowMatchesItsGeneratedShard` over the real corpus |
| Snapshot to producer | `readArtifactJSON` over the published ledger path, through `loadRequirementLedger` | Yes: `TestAnUnusableRequirementLedgerIsRefusedByName` |
| Producer to artifact | `writePublishedPage`, HTML and Markdown together | Yes: `TestTheRFCLedgerClaimsOnlyPublishedRoutes` stats both files for every answered route |
| Producer to coverage | the answered route list, checked by `Coverage` | Yes: the real build answers `published 912, written 912` and `./le site check` exits 0 |
| Artifact to llms-full | the `sectionClaimants` prefix claim from `website/data/nav.json` | Yes: `TestEveryRFCDetailRouteBelongsToOneSection`, and the real build, which runs `assignPages` |

### Integration Points
- `internal/le/site/build.go` `refreshNativeSurfaces` - one new call, placed beside
  `publishPluginRegistry`, before `publishSiteFacts`.
- `internal/le/site/producer.go` `namedArtifacts` - one new entry.
- `internal/le/site/rfccompliance.go` `renderRFCCompliance` - reads the new snapshot, writes
  the two link tables, loops the detail pages, answers every route.
- `internal/le/rfc` - the exports named in the Export Surface table below.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `refreshNativeSurfaces` derives the snapshot before the registry runs, and the producer reads it back through `readArtifactJSON`. The producer calls nothing in `internal/le/rfc` that reads the checkout |
| No unintended coupling (components stay isolated) | Yes | The site package holds no RFC vocabulary of its own. `TestTheSiteReadsItsRFCVocabularyFromThePackage` fails on any literal from the six closed sets `internal/le/rfc` declares |
| No duplicated functionality (extends existing, does not recreate) | Yes | `rfc.RequirementRows` and `rfc.CoverageRows` are the one producer of the cells and the buckets, and `RenderShards` was refactored to format what `RequirementRows` answers rather than to assemble the cells a second time |
| Zero-copy preserved where applicable (refs, not copies) | N-A | A build-time page generator writes files. No wire path, no pool, no hot loop |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The pages are written by the existing `rfc-compliance` producer, which is already registered from its own file's `init()`. No producer was added, no central switch was edited, and the one new entry in a shared list is `namedArtifacts`, which `checkNamedArtifacts` reads as the whole population by design |

## Open Decisions (for the owner)

Each row states the recommendation the spec is written under. An unanswered row does not
block implementation. Disclosure scope is NOT here: the owner ruled on it on 2026-09-01
and the Task section carries the ruling.

| ID | Question | Recommendation | Why |
|----|----------|----------------|-----|
| OD-1 | Route family: `/quality/rfc-compliance/<stem>/` or `/reference/rfcs/<stem>/`? | `/quality/rfc-compliance/<stem>/` | 39 of the 181 stems have no row in `docs/features/rfc-status.md`, so `/reference/rfcs/` cannot index the family. See Key Design Decisions |
| OD-2 | Does the extraction sign-off belong on a per-RFC page? | Yes | The exclusions are the decisions that kept sentences out of the requirement set. Without them the requirement count has no account of what was dropped |
| OD-3 | Should `collectRFCCompliance` be refactored to read the new snapshot, removing the second full walk of the tree per build? | Not in this spec | It is a separable simplification. Known Limitations names it |
| OD-4 | Should a summary with no Meta `\| Title \|` row be REFUSED by the parser? | No, answer empty | A refusal is a new gate, which this spec's scope discipline excludes. The 22 summaries are backfilled instead, so the empty answer is unreachable today |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A navigation entry claims every route under it, so 190 children of `/quality/rfc-compliance/` need no `nav.json` edit | `sectionClaimants` in `internal/le/site/llmsfull.go`, read | `assignPages` fails the build on 190 unsectioned pages | `TestEveryRFCDetailRouteBelongsToOneSection`, which runs `assignPages` over the real navigation with all 190 detail routes added to the published set | confirmed |
| A-2 | `rfc.Collect` plus `rfc.NewRenderInput` carries every fact a detail page needs except the title and the enrolment reason | Field-by-field read of `RenderInput` and of `shardRow` | A third reading of the checkout is needed, and the snapshot stops being one derivation | `TestARequirementRowMatchesItsGeneratedShard` | confirmed, with one addition: the extraction sign-off is `rfc.LoadExtractions`, which `NewRenderInput` does not carry. It is a second read of `rfc/extraction/` and no second walk of the summaries or the test tree |
| A-3 | `rfc.Check` is not needed by any detail page | `renderRFCCompliance` uses it only for the gate verdict and the violation list, both of which stay on the aggregate page | The build pays a second package type-check per site build | Review of the new code for any `rfc.Check` call | confirmed: `grep -n "rfc.Check" internal/le/site/rfcledger.go internal/le/site/rfcdetail.go` answers nothing |
| A-4 | The `docs` producer keeps `/reference/rfcs/` and this work adds no producer that writes it, so no route is doubled | `docsDestinationExact` and the `docs` producer registration, read | `Coverage.Doubled` goes red and `./le site check` exits 1 | `./le site check` after a full build | confirmed: the build answers 912 published and 912 written, and `./le site check` exits 0 with an empty unclaimed and doubly-claimed list |
| A-5 | Publishing 191 pages does not break any existing site test | The site tests read frozen published pages per family, and no existing test enumerates the whole route set except the coverage check | Frozen fixtures need regeneration | `go test ./internal/le/site` | broken, in one place and repaired: `TestTheRFCComplianceProducerClaimsItsPublishedRoute` asserted the producer answers exactly one route, which this work deliberately changes. It now asserts the index route is published and every other answered route is a child of it. The frozen page and mirror comparisons were untouched, because the two link tables are appended after the sections those comparisons read |
| A-6 | Every summary stem is already a safe URL segment, so no slug function is needed | 190 stems are lowercase `rfcNNNN` or `draft-...`, checked | Two stems could collide on one directory | `TestASlugIsTheStemItself` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The largest page is huge: `rfc7296` carries 228 requirement rows and its shard is 76 KB of Markdown | The published `index.html` for `rfc7296` exceeds 200 KB | Accept it, and measure. `/reference/rfcs/index.md` is already 159 KB, so the family adds no new class of page. No page-size budget exists in this package |
| R-2 | `llms-full.txt` grows by roughly the 1.3 MB of shard content, on a 4.3 MB file | The published file passes 5.5 MB | Measure after the first build and report. Trimming what llms-full carries is a separate decision |
| R-3 | `data/search-index.json` grows by the same corpus, on a 1.9 MB file | The published file passes 3 MB | Same as R-2 |
| R-4 | The build derives the RFC ledger twice: once in `publishRFCLedger` and once in `collectRFCCompliance` | `./le site build` wall time grows visibly | OD-3. The second walk excludes `rfc.Check`, which is the expensive half and is paid once already |
| R-5 | Requirement text is quoted RFC prose and carries pipes, backticks, angle brackets and ampersands | A cell breaks its row, or raw markup reaches the page | Escape at the render boundary and pin it with a test, patterned on `TestAPipeInAPluginDescriptionStaysInItsCell` |
| R-6 | The page and `rfc/requirements/<stem>.md` drift, so the site and the repository disagree about a requirement | Nothing signals it | A test asserts the page's cells equal the shard's cells for the same requirement, which is what makes drift a red test |
| R-7 | A summary deleted from `rfc/short/` leaves a frozen page with a fresh timestamp | Nothing signals it | `removeRetiredRFCPages`, patterned on `removeRetiredPluginPages`, with its own test |
| R-8 | Exporting `cover` and `discriminationVerdict` changes the JSON shape of `RenderInput` and moves `ai/RFC-REQUIREMENTS.md` or a shard | `./le rfc check` reports a stale generated file | Neither field is marshalled by the render path; confirm by diffing the generated files before and after |
| R-9 | An implementer softens a bad state under review pressure: a `weak` verdict becomes a count, an untested MUST is dropped from the table | A page whose failing rows are fewer than the snapshot's failing rows | The owner ruled on 2026-09-01 that none of it is held back. AC-17 and AC-18 make the softening a red test rather than a judgement call |
| R-10 | The two link tables push `/quality/rfc-compliance/index.md` from 4 KB to roughly 30 KB, moving the frozen fixture | `TestTheRFCComplianceSectionsReadAsThePublishedPage` goes red | Expected. The fixture is regenerated in the same change, and the existing assertions about the aggregate sections stay |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The published website. No daemon, no wire behavior, no configuration. A wrong page publishes a wrong conformance claim, which is a reputational cost rather than an operational one |
| How is it reverted? | Single commit revert. The pages are generated, so the next build removes them |
| Who else touches this path? | Any session working `internal/le/site` (the `plan/spec-site-renderers-in-go.md` coverage work is open and owns the current `Coverage` red) and any session working `internal/le/rfc` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le site build` | → | `publishRFCLedger` writes `data/rfc-requirements.json` | `TestABuildPublishesTheRequirementLedger` |
| the producer registry | → | `renderRFCCompliance` returns one route per stem plus the index | `TestTheRFCLedgerClaimsEachPublishedRouteOnce` |
| a reader on `/quality/rfc-compliance/` | → | the enrolled and declined link tables | `TestTheComplianceIndexLinksEveryPublishedStem` |
| `./le site check` | → | `Coverage` over the answered routes | `TestTheRFCLedgerClaimsOnlyPublishedRoutes` |
| the `llms-full` derived producer | → | `assignPages` over the real navigation | `TestEveryPublishedRouteBelongsToOneSection` (existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le site build` over this checkout | One page is published at `/quality/rfc-compliance/<stem>/` for every summary stem in `rfc/short/`, and the producer answers each of those routes exactly once |
| AC-2 | Any published stem page | It carries a requirement table with one row per requirement of that summary, in the summary's own order, with the six columns `rfc/requirements/<stem>.md` carries: requirement id, level, section, positive tests, negative tests, note |
| AC-3 | Any published stem page | Its at-a-glance facts state the public status from `docs/features/rfc-status.md` (or say the ledger carries no row), whether the stem is enrolled, the enrolment reason or the non-enrolment kind and reason, the gated MUST count, the gap count, the nightly-only count, the tag count, and the paths of the summary, the shard and the RFC text |
| AC-4 | A requirement carrying a `{gap}` annotation | The page shows the kind and the whole reason prose, and the page shows the ledger's Remaining cell for that RFC. A gap is never rendered as coverage |
| AC-5 | A requirement carrying a recorded audit verdict | The row shows the verdict word, its published meaning, and its freshness state. A requirement with no recorded verdict says it has none, rather than showing a blank cell |
| AC-6 | A tag on a published page | The page states whether a discrimination record exists for it. A record shows its route, its re-verified state, and, for a `no-break` record, its escape reason. A tag with no record reads as unproven, and a `no-break` record is never counted as a proof |
| AC-7 | A stem carrying an `rfc/extraction/<stem>.json` sign-off | The page shows the reviewer, the sign-off date, the source path and SHA, one row per section with its disposition, and every excluded site with its `excluded-kind` and its reason |
| AC-8 | `/quality/rfc-compliance/` | It lists every one of the 181 stems as a link, in two tables: the enrolled stems with their public status and their gated MUST count, and the declined stems with their disposition kind and reason. No stem is absent from both tables |
| AC-9 | Any published page of the family | An `index.md` mirror sits beside its `index.html`, and every fact the HTML states the mirror states |
| AC-10 | A summary removed from `rfc/short/`, then a build | The stem's published directory is gone |
| AC-11 | A build | `data/rfc-requirements.json` is published, is listed in `namedArtifacts`, and is the only input the detail producer reads from the checkout |
| AC-12 | A requirement whose text carries a pipe, a backtick, an angle bracket or an ampersand | The character renders inside its own cell, escaped, and breaks neither the row nor the markup |
| AC-13 | `./le site check` after a full build | It reports no unclaimed route and no doubled route arising from this family, and no missing named artifact |
| AC-14 | `go test ./internal/le/site` | It passes, including `TestEveryPublishedRouteBelongsToOneSection`, which runs over the real navigation |
| AC-15 | Any published stem page | Its heading carries the RFC's title from the summary's Meta `\| Title \|` row. Every summary in `rfc/short/` carries such a row after this change, and a summary without one shows the display name alone rather than a wrong title |
| AC-16 | The new site code | It spells no annotation kind, polarity, audit verdict, freshness state or discrimination route as a string literal, and recomputes no per-RFC bucket. Each comes from an exported symbol of `internal/le/rfc` |
| AC-17 | The five states the owner ruling names: a gated MUST with no tag, a `weak`, `wrong` or `unimplemented` verdict, a `stale-unit`, `stale-requirement` or `shifted` freshness, a tagged unit with no discrimination record, and a `no-break` record | Each appears on the page under the requirement id it belongs to, with its own word. None is omitted, relabeled, softened, shown as a blank cell, or counted as proven |
| AC-18 | Any bad state the snapshot holds for which a requirement id exists | The page names the requirement id. A count MAY accompany the list and MUST NOT stand in for it. For every count of a bad state the page prints, the ids that produced it are on the same page |
| AC-19 | Any published stem page | It opens with the card grid the index page carries, rendered by `rfcCardsHTML` and stated by `rfcCardsMirror`, over that RFC's own numbers: the gated MUST count, the both-polarities count, the declared gaps, the gated MUSTs with no test, the tagged units nothing was observed to break, and the recorded verdicts. A number that is not good news carries the `warn` or `bad` tone |
| AC-20 | The proof state of a requirement | One row per tagged unit, with the polarity, the test file, the test function, the `kind/tier` carrier and the proof state each in its own cell. The sentence that explains `unproven` is a legend above the table, stated once, and never repeated on a row |
| AC-21 | A requirement id printed anywhere other than its own row | It links to that row's anchor `#<lowercased rid>`, and carries the requirement's own text where the mention is a row of its own. The row itself carries the anchor |
| AC-22 | The at-a-glance table | It holds countable facts and repository paths only. The enrolment reason and the public ledger's coverage sentence are prose under their own headings, and the public ledger's remainder is stated exactly once on the page |
| AC-23 | A section with nothing to show | It says so in exactly one sentence. No section states its emptiness twice, and no section prints a bare "None." |
| AC-24 | A coverage bucket | Its table cell carries a count and no list of ids. The ids of each weakness bucket are a labeled list under the table, each one linked to its requirement's row |
| AC-25 | A requirement row whose section the extraction sign-off names | The Section cell carries the section's own title beside its number, and the sign-off table carries a Name column |
| AC-26 | Any table this family publishes | It sits inside `div.rfc-table-wrap`, which scrolls, so the page body never scrolls sideways |
| AC-35 | Any card grid of this family, index or per-RFC | SCALE leads and STANDING follows: the grid opens with `Gated MUSTs` and `Out of scope`, both neutral, the first saying in words that a population is not a result, and the three shares that partition the binding population follow in the same grid. Every ratio's denominator is the obligations that BIND Ze, which is the gated population less the `{not-applicable}` set, and no ratio is taken over the gated count |
| AC-36 | The card grid | The proof ratio sits immediately after the last partition share, and says in words that a test pair is not a proof until a break has been observed. A `no-break` escape is not counted as a proof, so a population of nothing but escapes reads 0.0% |
| AC-39 | The ratio cards of any grid | They PARTITION their denominator: every bucket that binds Ze is in exactly one card and no bucket is in two, so the shares add to 100%. Each names its own denominator on the card. The proof ratio is over tagged units and is NOT one of them, and the page says so beneath the grid |
| AC-40 | The proof card | It states that a break is observed once and never re-run, that what is re-verified is the unit, the claim and the producer still hashing to what was recorded, and it counts a LAPSED record apart from a live proof and apart from a unit that never had one. A summary whose records have all lapsed reads 0.0% |
| AC-41 | Any card of this family | It carries the percentage and the arithmetic behind it: the count under the value, on the card, with the sentence beside it saying what the measure MEANS rather than repeating the numbers. Its color names that meaning, never the score: green for a good outcome at any value, red for a bad one above zero, neutral for a population or a scope count |
| AC-37 | The requirement buckets on the index | The shares are over the binding population. The table carries a binding total that states whether every binding obligation falls in one bucket, then the not-applicable row marked as scope, then the gated total stated as the sum of the two. The bar covers the binding buckets only, and a sentence names the count it leaves out |
| AC-38 | The at-a-glance table of a stem page | It carries the binding count, the out-of-scope count and the tagged-unit count beside the figures it already carried, so every denominator the cards use is readable as a number |
| AC-61 | The tests of one requirement | They read in carrier order: KIND first, TIER within a kind, then polarity within a group, positive before negative. The order is `rfc.CarrierRank`, declared beside the vocabulary it orders, never a sequence restated in the site package. The sort is total and stable: alike citations order by the name a reader sees. A carrier the vocabulary does not rank sorts AFTER every ranked one rather than at rank zero, and an absent-polarity row takes the rank of the requirement's best carrier so the gap shows inside the group the eye is reading |
| AC-62 | The subject row | It carries no weight of its own. `.md-content td:first-child` is bold and sticky site-wide and the spanning cell is a first child, so the requirements table opts out by class. Position marks the subject; weight on top of it is emphasis doing a job the layout already does |
| AC-57 | The Requirements section | Each requirement is a SUBJECT row carrying its anchored id and its quoted sentence, always visible and with no disclosure, above a metadata row carrying the level, the section and the tests. The sentence is what the row is about; hiding it while showing its attributes is inside out. It stays a table rather than a block per requirement, because the Proof state section already renders one block per requirement and two lists of the same shape on one page help nobody |
| AC-58 | The retired Note column | Every mark it carried has a home and none was dropped. The `{kind} reason` annotation and the nightly-only mark render beside the tests they explain; the audit verdict and the superseded pointer are already rendered per requirement id under Proof state and Superseded. Verified over the real corpus rather than assumed |
| AC-59 | An annotation reason | It is stated under the rows it explains, where a reader who has just read "no negative test" is asking why, rather than in a column three cells away |
| AC-60 | The tests of one requirement | A GRID of divs, one row per citation, three fields: the polarity, the kind and tier, then the test. The tier LEADS the name it qualifies, so a fixed-width token precedes a variable-width one and the tiers align down the page. Divs rather than a nested table, which would inherit the outer table's width pressure. The grid carries table roles so it reads as data, and stacks per citation on a narrow viewport. A polarity with no test keeps its row and says so |
| AC-55 | The Requirements table | The two polarities share ONE Tests column, a labeled line each, so neither list is squeezed into half the width. A polarity with no test states "no positive test" or "no negative test" rather than leaving an empty half-cell |
| AC-56 | The requirement's own text | It is a SECOND ROW spanning every column of the table, not a disclosure inside the id cell where it expanded within the narrowest column on the page. The colspan is derived from the table's declared columns, never written beside them. A requirement quoting nothing gets no row |
| AC-53 | Anywhere a test is cited: the Requirements table's positive and negative cells, and the proof-state table | The citation names the TEST and not the file it lives in, and the name is the link, to the line the tag is written on. A Go test and an interop checker are named by their function, a `.ci` or `.et` scenario by its own file name. The whole path stays reachable as the link's `title` and as the link target in the mirror. The proof-state table carries no Test file column, and the kind and tier stay |
| AC-54 | Two tagged units on one page whose test names are equal | They render DIFFERENTLY: the package directory is prefixed to both. A name only one unit carries is never qualified |
| AC-51 | The public ledger's Coverage and Remaining cells | They are SPLIT, never rewritten: a Coverage cell renders one item per top-level claim, a Remaining cell renders its lead as prose and each authored "Theme: body" group as its own labeled item. The split is total and lossless -- rejoining the items reproduces the author's bytes -- and every item closes every bracket, quotation and code span it opens. A cell that carries no such structure renders whole |
| AC-52 | Any requirement id or file path inside that prose | An id the RFC declares links to its row on the same page; one it does not declare is left as text. A path addressable from the repository root links to its file through `repositoryBlobURL`; a relative citation is left as text. Every linked path resolves to a file in the tree |
| AC-47 | The exclusion vocabulary | Every kind declares whether it means SCOPE (the obligation never bound Ze) or DEBT (it is real, unbuilt, and a named spec owes it). There is no default, so a seventh kind lands in one group or reddens a test rather than reading as scope by omission |
| AC-48 | Any count of exclusions, on the index and on a stem page | A `relocated-to-spec` site is published as the DEBT it is, apart from the count that says an obligation never bound Ze: its own section with the reserved id, the quoted sentence and the spec that owns it, and its own row in the census. No count sums the two |
| AC-49 | Any published prose naming where a fact is authored | It names the summary's own `## Meta` table, never `rfc/enrolled.txt`, `rfc/not-enrolled.txt` or `docs/features/rfc-status.md`, which `./le rfc index-update` GENERATES from it. The page reads the enrolment reason, the title, the support claim, the coverage and the remainder from `RenderInput.Metas` |
| AC-50 | The enrolment and disposition renderers | They carry no branch for a summary declaring no enrolment or a kind with no reason: `ParseMeta` refuses both, and the artifact loader refuses the state BY NAME rather than rendering a placeholder for it |
| AC-42 | `/quality/rfc-compliance/` | It carries an exclusion disclosure at the same standing as the gap disclosure: the count per `excluded-kind`, the vocabulary and each kind's meaning read from `internal/le/rfc` rather than restated, the summaries that used each kind linked to their own pages, and a total stated as a share of the normative sentences the walks found |
| AC-43 | That section | It states on its own face how much of the corpus it covers, because sign-offs exist for a minority of summaries, and it states that `ai/rules/rfc-compliance.md` treats `binds-another-role` as PRESUMED WRONG until justified, beside that kind's own row and list |
| AC-44 | Any card grid of this family | It holds MEASURES only. How the gate is enforced -- the pre-commit stage count, the reproduce command, the inputs it reads and the artifacts it publishes -- sits in one section of its own, and the fact that the gate runs before a commit is verified is still published there |
| AC-45 | Any card grid of this family | It reads in four movements, headed Overall, Positive, Neutral and Negative in that order. A card's movement is DERIVED from its tone, so a card cannot sit under a heading its color contradicts, and every card lands in exactly one movement |
| AC-46 | The Check results section | It is a table with one row per finding: the RFC, the requirement id linked to its own page and row, the level, what is wrong, and the requirement's own text. The columns come from `rfc.Finding`, the parts the checks had before they formatted the line; nothing parses the message back apart. A finding carrying no requirement states itself in the column it fills. The 25-row bound stands and the page says how many findings it does not show |
| AC-29 | `/quality/rfc-compliance/`, the requirement buckets | The table carries a binding total that states in words whether the buckets account for the binding population or how many fall in none, and a gated total below it. The tape carries no text of its own; a key beneath it names every binding bucket with its count and its share, and every bucket the vocabulary declares has a color rule |
| AC-30 | The Gated MUSTs card, on the index and on a stem page | Its note says the number is what the gate HOLDS, and names the larger population it is a subset of: the requirements the corpus extracted and the summaries they came from. It is not the lead card (AC-35) |
| AC-31 | SUPERSEDED by AC-57 (owner review, 2026-09-01) | The requirement's sentence was put behind a disclosure in the id cell, then in a spanning row. Both hid or narrowed the subject while showing its metadata. It now LEADS its row with no disclosure. AC-2's shard equality is unchanged throughout: the cells are equal as DATA, and only the rendering moved |
| AC-32 | A proof-state row | The TEST is the link, to `blob/main/<file>#L<line>` from the line `rfc.Tag` records; the test-file cell is not a link. One repository-blob helper answers that URL for this page and for the documentation renderer, and no second host literal exists. A tag with no line addresses the file, and a cover with no file is stated unlinked |
| AC-33 | "What the public ledger says" | Three labeled facts: the status, the Coverage cell and the Remaining cell. A cell longer than 240 characters is FOLDED behind a disclosure and nothing is dropped. The Remaining cell is stated there and nowhere else on the page |
| AC-34 | Any card of this family | It declares the rule that chose its tone, and the page publishes those rules beside the grid. A number with no good or bad direction takes the neutral tone rather than a color that reads as a verdict |
| AC-28 | A requirement that is BOTH a declared gap and a gated MUST with no test | It takes ONE row under Gaps and untested MUSTs, naming both states in its State cell. The reason is quoted once |
| AC-27 | `/quality/rfc-compliance/` | The gate verdict is a status block carrying the tone vocabulary, not a `<pre>` block. No `pre` element is published under `<main>`, so no copy button offers a status as a command. The invocation that reproduces the check is the only command on the block |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTheRFCLedgerClaimsEachPublishedRouteOnce` | `internal/le/site/rfcdetail_test.go` | AC-1: one route per stem, answered once | green |
| `TestARequirementRowMatchesItsGeneratedShard` | `internal/le/site/rfcdetail_test.go` | AC-2, A-2, R-6: the page's six cells equal the shard's for the same requirement | green (compares against the shard `RenderShards` renders, not the file on disk: freshness is `./le rfc check`) |
| `TestAStemPageStatesItsEnrolmentAndItsPublicStatus` | `internal/le/site/rfcdetail_test.go` | AC-3 | green |
| `TestAGapIsShownWithItsReasonAndTheLedgerRemainder` | `internal/le/site/rfcdetail_test.go` | AC-4 | green |
| `TestAnAuditVerdictAppearsWithItsMeaningAndFreshness` | `internal/le/site/rfcdetail_test.go` | AC-5 | green |
| `TestARequirementWithNoVerdictSaysSoRatherThanShowingABlank` | `internal/le/site/rfcdetail_test.go` | AC-5, `ai/rules/principles.md` fail-closed | green |
| `TestATagWithNoDiscriminationRecordReadsAsUnproven` | `internal/le/site/rfcdetail_test.go` | AC-6 | green |
| `TestANoBreakRecordIsCountedApartFromAProof` | `internal/le/site/rfcdetail_test.go` | AC-6, the escape is never a proof | green |
| `TestTheExtractionSignoffNamesEveryExcludedSiteAndItsKind` | `internal/le/site/rfcdetail_test.go` | AC-7 | green |
| `TestTheComplianceIndexLinksEveryPublishedStem` | `internal/le/site/rfccompliance_test.go` | AC-8 | green |
| `TestADeclinedStemStatesItsKindAndReason` | `internal/le/site/rfccompliance_test.go` | AC-8 | green |
| `TestAnRFCPageReadsAsThePublishedPage` | `internal/le/site/rfcdetail_test.go` | AC-9, frozen published page, HTML | green |
| `TestAnRFCPageMirrorReadsAsThePublishedMirror` | `internal/le/site/rfcdetail_test.go` | AC-9, frozen mirror, Markdown parity | green |
| `TestARetiredRFCLosesItsPage` | `internal/le/site/rfcdetail_test.go` | AC-10, R-7 | green |
| `TestAnUnusableRequirementLedgerIsRefusedByName` | `internal/le/site/rfcdetail_test.go` | AC-11: a ledger naming no stem, and an entry with an empty stem, are each refused with the file named | green |
| `TestABuildPublishesTheRequirementLedger` | `internal/le/site/rfcdetail_test.go` | AC-11, wiring | green |
| `TestAPipeInRequirementTextStaysInItsCell` | `internal/le/site/rfcdetail_test.go` | AC-12, R-5 | green |
| `TestTheRFCLedgerClaimsOnlyPublishedRoutes` | `internal/le/site/rfcdetail_test.go` | AC-13, A-4 | green |
| `TestAStemPageCarriesTheTitleFromTheSummaryMetaRow` | `internal/le/site/rfcdetail_test.go` | AC-15 | green |
| `TestASlugIsTheStemItself` | `internal/le/site/rfcdetail_test.go` | A-6: every published directory equals its own stem | green |
| `TestTheSiteReadsItsRFCVocabularyFromThePackage` | `internal/le/site/rfcdetail_test.go` | AC-16: the new file holds none of the vocabulary literals | green |
| `TestEveryDisclosedStateAppearsUnderItsRequirementID` | `internal/le/site/rfcdetail_test.go` | AC-17: a fixture carrying all five states, each found on the page beside its id | green |
| `TestNoBadStateIsPublishedOnlyAsACount` | `internal/le/site/rfcdetail_test.go` | AC-18: for every count of a bad state the page prints, the snapshot's ids for that state are on the page too | green |
| `TestAStemPageOpensWithTheCardGridTheIndexCarries` | `internal/le/site/rfcdetail_test.go` | AC-19: the page carries the markup `rfcCardsHTML` answers, and a bad number reads as bad | green |
| `TestATagWithNoDiscriminationRecordReadsAsUnproven` | `internal/le/site/rfcdetail_test.go` | AC-20: one row per tagged unit, the state in its own cell, the legend once | green (rewritten for the table shape) |
| `TestANoBreakRecordIsCountedApartFromAProof` | `internal/le/site/rfcdetail_test.go` | AC-20: the escape reads as the escape on its own row | green (rewritten for the table shape) |
| `TestAMentionedRequirementLinksToItsOwnRow` | `internal/le/site/rfcdetail_test.go` | AC-21 | green |
| `TestTheGlanceTableCarriesNoProse` | `internal/le/site/rfcdetail_test.go` | AC-22: no glance cell exceeds 80 characters, the prose survives under its heading, the remainder is stated once | green |
| `TestAnEmptySectionStatesItsEmptinessOnce` | `internal/le/site/rfcdetail_test.go` | AC-23 | green |
| `TestACoverageBucketCarriesACountAndNotAList` | `internal/le/site/rfcdetail_test.go` | AC-24 | green |
| `TestARequirementRowNamesItsSection` | `internal/le/site/rfcdetail_test.go` | AC-25 | green |
| `TestEveryTableOnAStemPageScrollsInsideItsOwnContainer` | `internal/le/site/rfcdetail_test.go` | AC-26: every `<table>` under `<main>` is wrapped | green |
| `TestTheTestsReadInCarrierOrder` | `internal/le/site/rfcdetail_test.go` | AC-61: a mixed requirement reads unit, functional, interop with polarity inside each, and the applied order is the one `internal/le/rfc` declares | green |
| `TestTheTestOrderIsTotalAndStable` | `internal/le/site/rfcdetail_test.go` | AC-61: alike citations order by name, an unranked carrier sorts last, and the absent polarity keeps its group | green |
| `TestTheSubjectCarriesNoWeightOfItsOwn` | `internal/le/site/rfcdetail_test.go` | AC-62: the class, the override, no weight tag in the subject row, and no bold in the mirror | green |
| `TestEveryCarrierTheVocabularyDeclaresIsRanked` | `internal/le/rfc/carriers_test.go` | AC-61: every one of the 34 carriers in the real table has a rank, over 7 distinct kind/tier labels | green, and RED with `editor` removed from the order: 2 carriers unranked |
| `TestTheReadingOrderIsTotalAndAscending` | `internal/le/rfc/carriers_test.go` | AC-61: the two vocabularies make one strictly increasing sequence, tier inside kind, and a kind outside the vocabulary answers no rank | green |
| `TestTheRequirementTextLeadsItsRow` | `internal/le/site/rfcdetail_test.go` | AC-57: the header carries the metadata only, no disclosure survives, and the id and its sentence lead together | green (replaces AC-31's test, which held the disclosure this supersedes) |
| `TestNothingTheNoteCarriedWasLost` | `internal/le/site/rfcdetail_test.go` | AC-58: over the real corpus, every annotation and nightly-only mark is rendered beside the tests, and every audit and superseded mark has a section to render it | green: 1,731 annotations, 310 superseded, 4 audit, 2 nightly-only |
| `TestAnAnnotationReasonSitsUnderTheTests` | `internal/le/site/rfcdetail_test.go` | AC-59, including that the mark follows the rows rather than preceding them | green |
| `TestBothPolaritiesShareOneColumn` | `internal/le/site/rfcdetail_test.go` | AC-55: one Tests column, the two labels, and an absent polarity stated in both renderings | green |
| `TestTheRequirementTextSpansTheWholeTable` | `internal/le/site/rfcdetail_test.go` | AC-56: the spanning row, the colspan agreeing with the rendered header, the text kept in both renderings, and no row for a requirement quoting nothing | green |
| `TestAPipeInRequirementTextStaysInItsCell` | `internal/le/site/rfcdetail_test.go` | AC-12: the column count is now READ from the mirror's own header rather than written in the test | green (rewritten) |
| `TestATestIsCitedByNameAndNotByItsFile` | `internal/le/site/rfcdetail_test.go` | AC-53: the name links the line, the path is the title, neither table shows the path, and the Test file column is gone | green |
| `TestAScenarioIsCitedByItsFileName` | `internal/le/site/rfcdetail_test.go` | AC-53: a `.ci` unit is named by its base name, with the path as the title | green |
| `TestTwoTestsSharingANameAreToldApart` | `internal/le/site/rfcdetail_test.go` | AC-54: a collision is qualified by package, and an unambiguous name is not | green |
| `TestEveryShardCitationIsATaggedUnit` | `internal/le/site/rfcdetail_test.go` | AC-53: over 10,768 real cells, every citation the shard prints is a tagged unit, which is what lets the page cite from fields rather than re-read the shard's markdown | green |
| `TestEveryLedgerCellSplitsWithoutLoss` | `internal/le/site/rfcdetail_test.go` | AC-51: over all 318 cells of the real corpus, both splits rejoin byte for byte and every item is balanced | green, and RED on a naive cut at every semicolon: 13 unbalanced items |
| `TestEveryLinkedPathExistsInTheTree` | `internal/le/site/rfcdetail_test.go` | AC-52: all 379 linked paths exist, 65 relative citations stay text, and no undeclared id is linked | green |
| `TestTheLedgerProseRendersAsItsOwnStructure` | `internal/le/site/rfcdetail_test.go` | AC-51, AC-52: the claims, the themes, the lead, and a semicolon inside parentheses and inside a code span | green |
| `TestTheExclusionGroupsPartitionTheVocabulary` | `internal/le/site/rfccompliance_test.go` | AC-47: every kind declares a group, no kind declares one outside the two, and `relocated-to-spec` reads as debt | green |
| `TestARelocatedObligationIsPublishedAsDebt` | `internal/le/site/rfccompliance_test.go` | AC-48: the debt count, the section, the reserved id, the quote, the spec, and the total row stating the two apart | green |
| `TestADeclinedStemStatesItsKindAndReason` | `internal/le/site/rfccompliance_test.go` | AC-50: a stem with no disposition is refused by name rather than rendered with a placeholder | green (rewritten: the old fallback text is a state `ParseMeta` now refuses) |
| `TestTheExclusionLedgerIsPublishedWithItsCoverage` | `internal/le/site/rfccompliance_test.go` | AC-42, AC-43: the per-kind counts against the ledger, the vocabulary read from `internal/le/rfc`, the coverage sentence, and the presumed-wrong caution | green |
| `TestTheGateInputRowsReadAsThePublishedRows` | `internal/le/site/rfccompliance_test.go` | AC-44: the mechanism is in one section and no longer a card | green |
| `TestTheCardGridReadsInFourMovements` | `internal/le/site/rfccompliance_test.go` | AC-45: the four headings, tone-derived membership, and every card in exactly one movement | green |
| `TestTheCheckSectionPublishesTheGatesOwnIssues` | `internal/le/site/rfccompliance_test.go` | AC-46: the columns come from the finding's parts, the id links to its row, a partless finding still states itself, and the list is no longer bullets | green |
| `TestARatioLeadsAndAPopulationFollows` | `internal/le/site/rfccompliance_test.go` | AC-35: the grid opens with the two scale cards, both neutral, the first saying it is a population, and the three shares follow; no ratio is taken over the gated count | green, and RED with a ratio denominator changed to the gated count |
| `TestTheRatioCardsPartitionTheirDenominator` | `internal/le/site/rfccompliance_test.go` | AC-39: every binding bucket is in exactly one group, no bucket is in two, no non-binding bucket is in any, the parts name their denominator, and the proof ratio is not one of them | green, and RED with the single-polarity group deleted |
| `TestALapsedRecordIsNotCountedAsAProof` | `internal/le/site/rfccompliance_test.go` | AC-40: the card says the break is not re-run and what is re-verified instead, and four lapsed records read 0.0% | green, and RED with `Proven()` counting a lapsed record |
| `TestTheProofRatioSitsBesideTheTestPairRatio` | `internal/le/site/rfccompliance_test.go` | AC-36, over both card families, plus the escape case | green |
| `TestTheBucketTableAccountsForEveryGatedRequirement` | `internal/le/site/rfccompliance_test.go` | AC-29: the arithmetic over the live snapshot, the total row, the wordless tape, and a color rule for every declared bucket | green, and RED with the total row dropped and `.rfc-tape-missing_unexcused` deleted |
| `TestTheGatedMUSTCardNamesThePopulationItIsASubsetOf` | `internal/le/site/rfccompliance_test.go` | AC-30 | green |
| `TestEveryCardStatesTheRuleBehindItsColor` | `internal/le/site/rfccompliance_test.go` | AC-34: every card of both families carries a rule, its tone is in the vocabulary, the legend states it, and a population reads neutral | green |
| `TestTheRequirementTextSitsBehindADisclosure` | `internal/le/site/rfcdetail_test.go` | AC-31 | green |
| `TestAProofStateRowLinksTheTestToItsOwnLine` | `internal/le/site/rfcdetail_test.go` | AC-32, including the lineless and fileless cases | green |
| `TestThePublicLedgerCellIsLabelledAndFolded` | `internal/le/site/rfcdetail_test.go` | AC-33: a long cell folds and keeps every word, a short one does not fold, and the remainder is stated once | green |
| `TestAGapThatIsAlsoUntestedTakesOneRow` | `internal/le/site/rfcdetail_test.go` | AC-28 | green |
| `TestTheGateVerdictIsAStatusAndNotTerminalOutput` | `internal/le/site/rfccompliance_test.go` | AC-27: no `pre` under `<main>`, no `rfc-command`, and a red gate reads red | green |
| `TestASectionTitleIsTheOpeningSentenceOfItsReason` | `internal/le/rfc/extraction_test.go` | AC-25: the derivation, and prose about the walk answering no title | green |
| `TestEveryDerivedSectionTitleReadsAsATitle` | `internal/le/rfc/extraction_test.go` | AC-25, over the real corpus | green |
| `TestEnrolmentKeepsItsReason` | `internal/le/rfc/summary_test.go` | the reason survives `parseEnrolled` | green |
| `TestASummaryTitleComesFromTheMetaRow` | `internal/le/rfc/summary_test.go` | AC-15, and an absent row answers empty rather than guessing | green |
| `TestEverySummaryCarriesATitleRow` | `internal/le/rfc/summary_test.go` | the 22 backfills landed, over the real corpus | green |
| `TestTheCoverageRowIsReadableFromAnotherPackage` | `internal/le/rfc/coverage_test.go` | the exported per-RFC counters answer what the private walk answers | green |
| `TestTheDiscriminationVerdictIsReadableFromAnotherPackage` | `internal/le/rfc/discriminate_test.go` | the exported type carries the record, the state and the detail | green, plus `TestTheCoverKeyIsReadableFromAnotherPackage` |
| `TestTheAuditVerdictAccessorAnswersTheRecordedFields` | `internal/le/rfc/audit_test.go` | the typed accessor answers what the untyped map holds | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| stems in the published ledger | 0 to 181 | 181 | 0, which is refused by name | N/A, the corpus bounds it |
| requirement rows on one page | 0 to 228 | 228 | N/A, zero rows is a legal summary and the page says so | N/A |
| tags bound to one requirement | 0 to many | N/A | 0, which renders as "no test" rather than as a blank | N/A |
| audit verdicts on one page | 0 to the requirement count | the requirement count | 0, which renders as "not audited" rather than as a blank | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le site build` then `./le site check` | run by hand, recorded here | An operator builds the site and the checker reports no unclaimed route, no doubled route and no missing named artifact | green: `./le site build output <scratch>/site` exits 0 with `published 912, written 912`, and `./le site check output <scratch>/site` exits 0 with the same pair and no other finding |

No `.ci` test applies. The feature is a build-time site generator with no daemon surface,
no RPC and no CLI command of its own: `./le site build` and `./le site check` already
exist and are the entry points. The verification gate runs only `./le site facts check`
(`internal/le/verify/engine/stages.go`, the stage list `StagesForMode` answers) and never
a full site build, so `go test ./internal/le/site` plus the recorded manual build and
check is the whole proof this family can carry.

### Interop Tests (Scope: protocol)
Not applicable. No wire-visible behavior changes.

## Files to Modify
- `internal/le/site/rfccompliance.go` - `renderRFCCompliance` reads the new snapshot,
  writes the two link tables into the index body and mirror, deletes retired stem
  directories, loops the detail writer, and answers every route
- `internal/le/site/build.go` - `refreshNativeSurfaces` calls `publishRFCLedger`
- `internal/le/site/producer.go` - `namedArtifacts` gains `data/rfc-requirements.json`
- `internal/le/site/llmsdata.go` - the new artifact path constant, beside `pluginFile`
- `internal/le/site/doctransform.go` - `codeHostBlob` and `codeHostTree` become the whole
  repository rather than the `docs/` subtree, and resolve through the shared helper
- `internal/le/rfc/coverage.go` - export `CarrierFor`, `NightlyOnly`, `CoverageRow`,
  `CoverageRows`
- `internal/le/rfc/rfc.go` - export `AnnotationKinds`, the two polarity constants and
  `Polarities`
- `internal/le/rfc/audit.go` - export `AuditVerdicts` and the typed verdict accessor
- `internal/le/rfc/discriminate.go` - export `Cover`, `DiscriminationVerdict` and
  `ProofStates`
- `internal/le/rfc/render.go` - `RenderInput.Covers` and `RenderInput.Discrimination` take
  the exported types
- `internal/le/rfc/summary.go` - `parseEnrolled` keeps the reason; a Meta `| Title |`
  reader is added
- `internal/le/rfc/artifact.go` - `ExtractionSection.Title` derives a section's own name
  from the opening sentence of its reason
- `internal/le/rfc/artifact.go` - `exclusionKinds` carries each kind's published meaning AND
  its group, and `ExclusionKindMeaning`, `ExclusionKindGroup` and `ExclusionPresumedWrong`
  read it
- `internal/le/rfc/carriers.go` - `kindEditor` and `kindUnknown` become named constants
  beside the three that already were, and `carrierKindOrder`, `carrierTierOrder`,
  `CarrierKinds`, `CarrierTiers`, `CarrierRank` and `CarrierLabelRank` declare the reading
  order beside the vocabulary it orders
- `internal/le/rfc/check.go` - `Finding` carries the parts a check had before it formatted
  its line; `CheckReport.Findings` is the one list and `Violations` is rendered from it
- `internal/le/rfc/check_core.go` - `evaluate` answers findings rather than strings
- `internal/le/rfc/status.go` - `Collected` gains the enrolment reasons and the titles
- `rfc/short/*.md` - the 22 summaries with no Meta `| Title |` row gain one
- `website/AI.md` - the producer registry, `Coverage`, the four `./le site check` failure
  modes, the corrected `resolvePaths` and `isSourceOnly` spellings, and the new family
- `docs/contributing/gh-pages.md` - the "Quality pages" section gains the detail family
- `docs/contributing/rfc-conformance-gates.md` - the ledger is now published per RFC, and
  the page names where
- `docs/features/rfc-status.md` - one sentence pointing a reader at the per-requirement
  detail
- `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md` - regenerated, never hand-edited

## Files to Create
- `internal/le/site/rfcdetail.go` - the per-stem page: the section list, the hero, the
  overview cards, the at-a-glance facts, the enrolment and public-ledger prose, the
  coverage buckets and the requirement rows, each with its markup and its mirror
- `internal/le/site/rfcmarkup.go` - the primitives both renderings call: the requirement
  anchor, the linked mention, the scrolling table container, the escaped cell, the
  repository-blob URL and the prose fold
- `internal/le/site/rfcprose.go` - the authored ledger prose: the claim split, the theme
  split, and the linker that turns a requirement id into a row link and a repository path
  into a file link without altering a word
- `internal/le/site/rfccitation.go` - how a test is cited: the name a reader sees, the
  package qualifier where two units on one page share a name, and the link to the tagged
  line with the path kept as the link's title
- `internal/le/site/rfcevidence.go` - the evidence half: the gaps, the proof state of
  every tagged unit, the extraction sign-off and the superseded rows
- `internal/le/site/rfcledger.go` - the snapshot type and `publishRFCLedger`
- `internal/le/site/rfcdetail_test.go` - the tests above
- `internal/le/site/testdata/` - the frozen published page and mirror for one stem, and
  the disclosure fixture carrying all five bad states AC-17 names

## Export Surface `internal/le/rfc` Owes

The page must not re-parse markdown the package parses, and must not recompute a bucket
`coverage.go` computes (`ai/rules/principles.md`). Each row names what the site package
needs, what stands in the way today, and what to export.

| Need | Today | Export or add |
|------|-------|---------------|
| The carrier that runs a tag's file | `carrierFor` is private; `Carrier.Label` is exported | `CarrierFor(rel string, carriers []Carrier) (Carrier, bool)` |
| The nightly-only mark the shard prints | `isNightlyOnly` is private, and so are the tier constants | `NightlyOnly(found []Tag, carriers []Carrier) bool` |
| The tagged unit a tag sits in | `RenderInput.Covers` is an exported field whose key type `cover` is unexported, so no other package can index it | export the key type as `Cover{RID, Polarity, Unit}` |
| The re-verified state of a stored proof | `RenderInput.Discrimination` is an exported field whose element type `discriminationVerdict` is unexported | export it as `DiscriminationVerdict{Record, State, Detail}`, plus `ProofStates()` |
| One requirement's recorded verdict | `Audit.Verdict` answers `map[string]any`; the five verdict words and the six keys are private | a typed accessor answering the verdict word, the note, the three fingerprint maps, the upgrade reason and the no-code-path flag, plus `AuditVerdicts()` |
| The annotation-kind vocabulary | `annotationKindNames` is private, while its four siblings are exported | `AnnotationKinds()` |
| The two polarities | private literals | `PolarityPositive`, `PolarityNegative`, `Polarities()` |
| Per-RFC gated, both, one, annotated, missing and nightly-only counts | `rfcCoverage` and `rfcCoverageRows` are private, which is why `rfccompliance.go` recomputes its own buckets | `CoverageRow` and `CoverageRows(RenderInput) []CoverageRow` |
| The enrolment reason | `parseEnrolled` keeps the first field and drops the rest | `Collected` gains the reason map, filled by the same parse |
| The RFC's title | nothing parses the Meta `\| Title \|` row | a reader for it, answered per stem on `Collected` |
| The carrier table | already reachable as `RenderInput.Carriers` | nothing. The commissioning brief listed `carriers` as unusable; the loaded table is already exposed |

Two of these rows are latent defects rather than convenience: `RenderInput.Covers` and
`RenderInput.Discrimination` are exported fields no package outside `internal/le/rfc` can
read. An exported field of an unexported type is a name with no reachable value.

## What One RFC Page Carries

| # | Section | Content | AC |
|---|---------|---------|----|
| 1 | Hero | The display name and the title, with the public status as the eyebrow label | AC-15 |
| 2 | Overview | Eight cards, rendered by the index page's own card renderer, SCALE FIRST: gated MUSTs and out of scope, then the three shares that partition the binding population (tested both ways, one polarity plus reason, one polarity unexcused, no test at all), then the proof ratio over tagged units, then the audit verdicts. Each carries its value, the arithmetic under it, what the measure means, and a color naming that meaning. The rules are published beside the grid | AC-19, AC-30, AC-34, AC-35, AC-36, AC-39, AC-40, AC-41 |
| 3 | At a glance | Public status, enrolment state, requirement count, gated MUST count, binding count, out-of-scope count, gap count, untested count, nightly-only count, tag count, tagged-unit count, audited count, discrimination record count, and the paths of the summary, the shard and the RFC text. Countable facts and paths only | AC-3, AC-22, AC-38 |
| 4 | Enrolment | The enrolment reason, or the non-enrolment kind and reason, as the paragraph it is | AC-3, AC-22 |
| 5 | What the public ledger says | Three labeled facts: the public status, the Coverage cell as its list of claims, and the Remaining cell as its lead and its themed groups. Every requirement id links to its row and every repository path to its file. A cell over 240 characters folds behind a disclosure, and nothing is rewritten, paraphrased or dropped | AC-3, AC-22, AC-33, AC-51, AC-52 |
| 6 | Coverage | The per-RFC counters: gated, both polarities, one polarity, annotated, missing, nightly-only. The table carries counts; the membership of each weakness is a labeled list under it, every id linked to its row | AC-3, AC-18, AC-24 |
| 7 | Requirements | Two rows per requirement: the anchored id and its quoted sentence spanning the table with no weight of their own, then the level, the section's own title and the tests. The tests are a grid, one row per citation in carrier reading order, tier before the name, with a row for an absent polarity and the annotation reason beneath. No Note column | AC-2, AC-12, AC-21, AC-25, AC-53, AC-55 to AC-62 |
| 8 | Gaps and untested MUSTs | Every `{gap}` row with its whole reason prose and every gated MUST with no tag. Each row names its requirement id, links to its row and carries its text. The ledger's Remaining cell is NOT here: it is stated once, in section 5, beside the Coverage cell it belongs with | AC-4, AC-17, AC-18, AC-21 |
| 9 | Proof state | One block per requirement: the linked id, the requirement text, the recorded verdict with its meaning and freshness, and one ROW PER TAGGED UNIT carrying the polarity, the test file, the test cited by NAME and linked to the line its tag is written on, the carrier and the proof state. One legend explains `unproven`. Every weak, wrong, unimplemented, stale, shifted or unproven row names its requirement id | AC-5, AC-6, AC-17, AC-18, AC-20 |
| 10 | Extraction sign-off | The reviewer, the date, the source path and SHA, one row per section with its name, its disposition and skip kind, and every excluded site with its `excluded-kind` and reason | AC-7, AC-25 |
| 11 | Superseded | Where the summary carries a successor, the disposition, the target and the reason | AC-2, AC-21 |

A section with nothing to show says so in ONE sentence and is not omitted. An omitted
section and an empty one read the same to a reader, and only one of them is a fact
(`ai/rules/principles.md`). One sentence and not two: the first pass printed the
Superseded sentence and then a bare "None." under it, which reads as two facts (AC-23).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | A build-time site generator has no daemon configuration |
| YANG validation constraints | N-A | No YANG leaf is added |
| YANG custom validators | N-A | No YANG leaf is added |
| CLI commands/flags | N-A | `./le site build` and `./le site check` already exist and are unchanged |
| CLI grammar (keyword before value) | N-A | No command is added |
| Editor autocomplete | N-A | No configuration leaf is added |
| Functional test for new RPC/API | N-A | No RPC or API is added. `go test ./internal/le/site` plus a recorded `./le site build` and `./le site check` is the proof, as the TDD plan states |
| Pipe completeness | N-A | No command output is added |
| Env var registration | N-A | No environment variable is added |
| Doctor check for runtime dependencies | N-A | No file path, socket, service, module, port, binary or certificate is added at daemon runtime. `data/rfc-requirements.json` is a published build artifact, and its absence is caught by `checkNamedArtifacts` |
| Prometheus counters/metrics | N-A | No daemon state is added |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol behavior changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` gains the published per-RFC ledger under the RFC conformance entry |
| 2 | Config syntax changed? | N-A | No configuration syntax changes |
| 3 | CLI command added/changed? | N-A | `./le site build` gains a surface, not a command or a flag |
| 4 | API/RPC added/changed? | N-A | No RPC changes |
| 5 | Plugin added/changed? | N-A | No plugin changes |
| 6 | Has a user guide page? | Yes | `docs/contributing/gh-pages.md`, "Quality pages" |
| 7 | Wire format changed? | N-A | No wire format changes |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No `rfc/short/` requirement row, level, annotation or claim changes. The 22 Meta `\| Title \|` backfills touch the Meta block only, and `docs/features/rfc-status.md` gains one navigational sentence and no status cell |
| 10 | Test infrastructure changed? | N-A | No suite, runner or carrier changes |
| 11 | Affects daemon comparison? | N-A | `docs/comparison.md` compares daemons |
| 12 | Internal architecture changed? | Yes | `website/AI.md` (the producer registry, `Coverage`, the four check failure modes, the two corrected symbol spellings, the new family) and `docs/contributing/rfc-conformance-gates.md` (the ledger is now published per RFC) |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | No registration inventory changes. The site producer registry is internal to the build and is documented in `website/AI.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, not answered from memory: run `./le spec citation anchors spec plan/spec-publish-the-rfc-requirement-ledger.md` and name every page it lists. `ai/CODE-TO-DOCS.md` already routes `internal/le/site/build.go`, `catalog.go` and `rfccompliance.go` to `docs/contributing/gh-pages.md`, and `internal/le/rfc/audit.go`, `carriers.go`, `coverage.go` and `freshness.go` to `docs/contributing/rfc-implementation-guide.md`. `docs/architecture/core-design.md` is DECLARED by the `// Design:` header of four `internal/le/rfc` files this work edits, and is named as unaffected: it describes the rfc area as one command, and this work adds exported symbols without changing a verb, an argument, an output or a gate. Both generated indexes are regenerated, never hand-edited |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `website/AI.md` names `ResolvePaths` and `IsSourceOnly` as exported; both are unexported. Corrected in this change |

### Documentation defects found while researching, and their disposition

| Defect | In this work's blast radius? | Disposition |
|--------|------------------------------|-------------|
| `website/AI.md` names `ResolvePaths` and `IsSourceOnly` as exported; `internal/le/site/paths.go` spells them `resolvePaths` and `isSourceOnly` | Yes: this work adds a producer, and the page is that producer's declared design doc | Fixed here, row 17 |
| `website/AI.md` describes one of the four failure modes `./le site check` has; `checkReport.exit` refuses on a source-only path, a missing mirror, a missing named artifact and a red `Coverage` | Yes: this work adds a named artifact and 182 routes, so two of the four now apply to it | Fixed here, row 12 |
| `website/AI.md` never mentions the producer registry or `Coverage` | Yes, same reason | Fixed here, row 12 |
| `rfc/requirements/README.md` and `rfc/audit/README.md` do not exist, while `rfc/discrimination/README.md` and `rfc/extraction/README.md` do | No: this work reads those directories and makes no page about them wrong | One journal row, per `ai/rules/completion.md`. Not widened into this spec |
| `rfc/README.md` is a hand-written BGP-only status table that disagrees with `docs/features/rfc-status.md` | No: it was already wrong and this change does not make it wrong | One journal row. Not widened into this spec |
| 39 of the 181 summary stems have no row in `docs/features/rfc-status.md` | No, and it is the fact that settles OD-1 rather than a defect this work introduces | Reported to the owner. Not fixed here |
| 22 of the 181 summaries carry no Meta `\| Title \|` row | Yes: AC-15 depends on that row | Backfilled here |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- publish an empty ledger and claim the routes
   - Tests: `TestABuildPublishesTheRequirementLedger`,
     `TestTheRFCLedgerClaimsEachPublishedRouteOnce`,
     `TestAnUnusableRequirementLedgerIsRefusedByName`, `TestASlugIsTheStemItself`
   - Files: `internal/le/site/rfcledger.go`, `internal/le/site/build.go`,
     `internal/le/site/producer.go`, `internal/le/site/llmsdata.go`,
     `internal/le/site/rfccompliance.go`
   - Verify: the build writes `data/rfc-requirements.json`, the producer reads it and
     answers one route per stem, and the pages are stubs, so the content tests fail
2. **Phase: Exports** -- give `internal/le/rfc` the surface the snapshot needs
   - Tests: `TestEnrolmentKeepsItsReason`, `TestASummaryTitleComesFromTheMetaRow`,
     `TestEverySummaryCarriesATitleRow`, `TestTheCoverageRowIsReadableFromAnotherPackage`,
     `TestTheDiscriminationVerdictIsReadableFromAnotherPackage`,
     `TestTheAuditVerdictAccessorAnswersTheRecordedFields`
   - Files: the `internal/le/rfc` files in Files to Modify, plus the 22 `rfc/short/*.md`
     backfills
   - Verify: `./le rfc check` still passes and `rfc/requirements/*.md` and
     `ai/RFC-REQUIREMENTS.md` are byte-identical, which is R-8's check
3. **Phase: Snapshot** -- fill `publishRFCLedger` from one reading of the checkout
   - Tests: `TestARequirementRowMatchesItsGeneratedShard`
   - Files: `internal/le/site/rfcledger.go`
   - Verify: the snapshot's cells for a requirement equal the shard's cells, and no
     `rfc.Check` call exists in the new code
4. **Phase: Detail page** -- the body, the mirror, the disclosure and the escaping
   - Tests: `TestAStemPageStatesItsEnrolmentAndItsPublicStatus`,
     `TestAGapIsShownWithItsReasonAndTheLedgerRemainder`,
     `TestAnAuditVerdictAppearsWithItsMeaningAndFreshness`,
     `TestARequirementWithNoVerdictSaysSoRatherThanShowingABlank`,
     `TestATagWithNoDiscriminationRecordReadsAsUnproven`,
     `TestANoBreakRecordIsCountedApartFromAProof`,
     `TestTheExtractionSignoffNamesEveryExcludedSiteAndItsKind`,
     `TestAStemPageCarriesTheTitleFromTheSummaryMetaRow`,
     `TestAPipeInRequirementTextStaysInItsCell`,
     `TestEveryDisclosedStateAppearsUnderItsRequirementID`,
     `TestNoBadStateIsPublishedOnlyAsACount`,
     `TestTheSiteReadsItsRFCVocabularyFromThePackage`
   - Files: `internal/le/site/rfcdetail.go`, the disclosure fixture under
     `internal/le/site/testdata/`
   - Verify: every section of the What One RFC Page Carries table renders, each test above
     fails before its section exists, and the two disclosure tests fail against a page that
     prints a count without its ids
5. **Phase: Index and retirement** -- the two link tables and the retired-page delete
   - Tests: `TestTheComplianceIndexLinksEveryPublishedStem`,
     `TestADeclinedStemStatesItsKindAndReason`, `TestARetiredRFCLosesItsPage`,
     `TestTheRFCLedgerClaimsOnlyPublishedRoutes`
   - Files: `internal/le/site/rfccompliance.go`
   - Verify: the index links all 181 stems, and a stem dropped from the input loses its
     directory
6. **Phase: Freeze and measure** -- the frozen fixtures and the size numbers
   - Tests: `TestAnRFCPageReadsAsThePublishedPage`,
     `TestAnRFCPageMirrorReadsAsThePublishedMirror`, and the regenerated
     `TestTheRFCComplianceSectionsReadAsThePublishedPage`
   - Files: `internal/le/site/testdata/`
   - Verify: `./le site build` then `./le site check` exits 0. Record the published sizes of
     `llms-full.txt`, `data/search-index.json`, `data/rfc-requirements.json` and the largest
     stem page, which answers R-1, R-2 and R-3 with measurements
7. **Phase: Documentation** -- the pages the checklist named
   - Files: `website/AI.md`, `docs/contributing/gh-pages.md`,
     `docs/contributing/rfc-conformance-gates.md`, `docs/features.md`,
     `docs/features/rfc-status.md`, and the regenerated `ai/CODE-TO-DOCS.md` and
     `ai/DOCS-TO-CODE.md`
   - Verify: `./le spec citation anchors` names no page left unaddressed

### Critical Review Checklist
| Check | What to verify for this spec | Verdict |
|-------|------------------------------|---------|
| Completeness | Every AC-N has an implementation at file and symbol, and AC-17 is checked against a real `weak` verdict and a real untested MUST from this corpus, not the fixture alone | PASS with one gap NAMED: this corpus holds no `weak`, `wrong` or `unimplemented` verdict and no `no-break` record today, because `rfc/audit/` holds one file whose 52 verdicts are all `enforced`. Those two states are proven by the disclosure fixture alone. The other three ARE proven over the corpus: `TestEveryUntestedMustOfThisCheckoutIsNamedOnItsPage` names 1549 untested gated MUSTs across 142 pages, and `TestNoBadStateOfThisCheckoutIsPublishedOnlyAsACount` names 505 declared gaps, 4050 tagged units with no record, 2 non-enforced verdicts and 2 verdicts that are no longer current |
| Feature completeness | All 181 stems are reachable by a link from `/quality/rfc-compliance/`, including the 39 with no row in `docs/features/rfc-status.md` | PASS: `TestTheComplianceIndexLinksEveryStemOfThisCheckout` renders the index from the live ledger and requires an `href` for all 190 stems |
| Owner ruling | No bad state is omitted, softened, relabeled, or published only as a count. Read the page against the snapshot's failing rows, not against the page's own summary | PASS: both AC-18 tests walk the SNAPSHOT for failing rows and require each id on the page, over the fixture and over the corpus |
| Correctness | The page's requirement cells equal the shard's, so the site and the repository cannot state different things about one requirement | PASS by construction: `rfc.RequirementRows` is the one producer of both, and `TestARequirementRowMatchesItsGeneratedShard` compares 5093 rows cell by cell |
| Correctness | A `no-break` discrimination record is never rendered as a proof, and a `{gap}` is never rendered as coverage | PASS: `TestANoBreakRecordIsCountedApartFromAProof` requires the words "escape" and "which is not a proof" beside the route, and `TestAGapIsShownWithItsReasonAndTheLedgerRemainder` requires the gap row and its whole reason in the Gaps section |
| Naming | JSON keys in `data/rfc-requirements.json` are kebab-case, matching every other published data file | PASS: every tag is kebab-case. Four were renamed away from a bare vocabulary word (`positive-test`, `negative-test`, `mapped-sites`, `excluded-sites`), so `TestTheSiteReadsItsRFCVocabularyFromThePackage` stays a mechanical check |
| Data flow | The detail producer reads `data/rfc-requirements.json` and nothing else from the checkout. No `rfc.Check` call exists in the new code | PASS: the producer reads the ledger artifact and `website/data/page-links.json`, which is the sidebar every producer reads. It calls `rfc.TableCell`, a pure formatter, and nothing else in that package |
| Rule: `ai/rules/principles.md` | No vocabulary literal and no recomputed bucket in the new site code. A missing verdict, a missing title and a missing carrier each SAY so rather than answering an empty string that reads like data | PASS: `TestTheSiteReadsItsRFCVocabularyFromThePackage` covers the literals; the buckets are `rfc.CoverageRows`; and `rfcVerdictText`, `rfcDetailHeading`, `rfcPathOrAbsent`, `rfcOrUnstated` and `rfcIndexDispositionKind` each answer a sentence for the absent case |
| Rule: `ai/rules/documentation.md` | `website/AI.md` and `docs/contributing/gh-pages.md` are edited in this change, not in a follow-up | PASS: both are edited here, with `docs/features.md`, `docs/features/rfc-status.md`, `docs/contributing/rfc-conformance-gates.md`, `docs/architecture/core-design.md` and the regenerated `docs/features/test-health.md` |

### Deliverables Checklist
| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| One page per summary stem | The published `quality/rfc-compliance/` directory holds one subdirectory per stem in `rfc/short/`, counted and compared | MET: 190 stem directories against 190 summaries |
| Every page carries a mirror | `./le site check` reports no missing mirror | MET: the check answers no `missing-mirrors` key at all |
| The snapshot is published and named | `data/rfc-requirements.json` exists with content, and `checkNamedArtifacts` holds it | MET: 7,110,598 bytes published, and the path is in `namedArtifacts`, which `TestABuildPublishesTheRequirementLedger` asserts |
| No route is unclaimed or doubled | `./le site check` reports an empty `unclaimed` and an empty `doubly-claimed` for this family | MET: `published 912, written 912`, and the check exits 0 |
| The generated RFC files did not move | `git diff --stat rfc/requirements/ ai/RFC-REQUIREMENTS.md` is empty | MET for this work: running `./le rfc index-update` after the export refactor re-rendered `ai/RFC-REQUIREMENTS.md` and 189 shards byte-identical. One file moved, `rfc/requirements/rfc4724.md`, and it was stale against a test tag committed before this work rather than changed by it |
| Every summary carries a title | `TestEverySummaryCarriesATitleRow` over the real corpus | MET: 190 of 190 |
| Every bad state is named, not counted | `TestNoBadStateIsPublishedOnlyAsACount` over the real corpus | MET, by that test over the fixture and `TestNoBadStateOfThisCheckoutIsPublishedOnlyAsACount` over the corpus |
| The site tests pass | `go test ./internal/le/site` | PARTLY: every test this work added or changed passes. Seven tests fail in files this work never opened -- four `changes_*` tests whose frozen fixture predates the 2026-08-24 weekly update, `TestACodeSpanHoldingAPipeStaysInOneTableCell` in the docs renderer, and two untracked `zz*_test.go` probes another session left pointing at a deleted scratch directory |
| The RFC gate still passes | `./le rfc check` | NO, and it did not pass before this work either: 147 enrolled gated MUSTs carry no test and no annotation, two `rfc7606` verdicts are stale, one `rfc2865` record no longer verifies, and one `rfc8671` requirement maps to no extraction site. None of the four is this work's, and the published pages state all of them |

### Security Review Checklist
| Check | What to look for | Result |
|-------|-----------------|--------|
| Input validation | The snapshot is repository-authored, not user-supplied, but it is read back from a file. An entry with an empty stem, and a ledger naming no stem, are each refused by name rather than skipped | PASS: `loadRequirementLedger` refuses both and names `data/rfc-requirements.json` in each message. `TestAnUnusableRequirementLedgerIsRefusedByName` covers the pair |
| Injection | Requirement text is quoted RFC prose and reaches HTML. Every field is escaped at the render boundary; no field is written raw | PASS: every cell goes through `html.EscapeString`, including inside `rfcInlineHTML`, which escapes each segment between the markdown markers rather than passing the cell through. `TestAPipeInRequirementTextStaysInItsCell` asserts `&lt;once&gt; &amp; only once.` in the markup and refuses a raw `<once>` |
| Path handling | The published directory is the stem, and a stem that is not a plain lowercase path segment must be refused rather than joined into a path | PARTIAL, and stated: `TestASlugIsTheStemItself` refuses any stem outside `[a-z0-9][a-z0-9.-]*` over the real corpus, and all 190 pass. There is no runtime refusal in the writer, because the stem comes from a directory listing of a tracked tree rather than from input. A stem carrying a path separator would be caught by that test before it could be published |
| Resource exhaustion | 191 pages and roughly 1.3 MB of table content per build. Bounded by the corpus, measured in phase 6 | MEASURED and larger than estimated: the 190 pages total about 5.6 MB of HTML and 4.7 MB of Markdown, the snapshot is 7.1 MB, and `llms-full.txt` doubled to 9.3 MB. Bounded by the corpus, and Known Limitations carries the numbers |
| Information disclosure | This work publishes conformance weaknesses on purpose, by owner ruling of 2026-09-01. It publishes no credential, no host address and no private path: every field comes from a tracked file already public in the repository | PASS: every published field is read from `rfc/short/`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `docs/features/rfc-status.md`, `rfc/audit/`, `rfc/discrimination/` or `rfc/extraction/`, all of which are tracked and public. The repository-relative paths it prints are paths the generated shards already print |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to RESEARCH |
| Lint failure | Fix inline. If architectural, back to DESIGN |
| A disclosure test is red | Fix the PAGE. Weakening AC-17 or AC-18 is refused: the owner ruled on the scope and this spec cannot reduce it |
| `./le site check` reports a doubled route | A-4 is broken: another producer writes this family's route. Back to DESIGN |
| `assignPages` fails on unsectioned pages | A-1 is broken: the navigation prefix claim does not reach these routes. Back to DESIGN |
| `./le rfc check` goes red after the export phase | R-8 is real: a generated file moved. Revert the field type change and add an accessor instead |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- An exported struct field whose type is unexported is a name with no reachable value.
  `RenderInput.Covers` and `RenderInput.Discrimination` are both in that state, and both
  carry facts another package wants. The shape is worth a hunt across the repository.
- The index of a generated family and the mirror of a hand-authored page are different
  artifacts, and using one as the other silently orphans whatever the authored page does
  not list. Here it would have been 39 of 181 pages, with nothing to say so.
- `rfc/audit/` holds one file and `rfc/discrimination/` holds five, against 173 enrolled
  RFCs. Publishing the proof state per requirement makes that ratio visible for the first
  time. That is the intended effect of the owner's disclosure ruling and it will read as a
  weakness.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The family is published at `/quality/rfc-compliance/<stem>/`, and `/quality/rfc-compliance/` is its index | `/reference/rfcs/<stem>/`, children of the `docs`-produced mirror of `docs/features/rfc-status.md`, as the commissioning brief proposed | 39 of the 181 stems have no row in that ledger (32 enrolled, 7 declined), so its table cannot index the family and 39 pages would be reachable only from search. The quality route is also the page the owner named, its producer already owns the route so the index costs no new producer and no navigation entry, and it needs no change to the `docs` renderer. The reference route would additionally need a link transform applied to two representations, because the `docs` producer builds the HTML from `marked` and the mirror from `plain` |
| One producer writes the index and all 182 routes | A second producer for the detail pages | `checkProducer` refuses a duplicate name and `Coverage` refuses a doubled route, so the index would have to move out of the aggregate producer or the two would have to split the route set. One producer over one snapshot is fewer moving parts and matches `renderPluginCatalog`, which writes an index and 93 detail pages |
| `refreshNativeSurfaces` derives the snapshot; the producer reads it | The producer calls `rfc.Collect` and `rfc.NewRenderInput` at render time | It matches `publishPluginRegistry` and `publishCommandCatalog`, it keeps the producer a pure function of a published input so a test can state a ledger instead of walking the tree, and the published JSON is itself the machine-readable ledger the disclosure ruling wants |
| The pages never call `rfc.Check` | Calling it to show the gate verdict per RFC | `rfc.Check` is a second full pass that also type-checks every tagged package under a 15 minute vet timeout. The aggregate page already pays it once and shows the verdict; a detail page adding a second pass would double the most expensive part of the build for a fact already published |
| The Meta `\| Title \|` row is the only title source, and the 22 summaries without one are backfilled | Parse the H1 as a fallback | The H1 separator is inconsistent across the corpus (an em dash, a double hyphen, a colon) and one H1 carries a `(short)` suffix, so a fallback parser would have to guess. One labeled source and a backfill leaves nothing to guess and nothing to drift |
| A summary with no title row answers empty, and the page shows the display name alone | Refuse the summary | A refusal is a new gate, and this spec's scope excludes adding one (OD-4). After the backfill the empty answer is unreachable, and `TestEverySummaryCarriesATitleRow` keeps it that way |
| Every bad state is named under its requirement id, and a count never replaces the list | Publish the counts and let a reader ask for the detail | Owner ruling, 2026-09-01. An aggregate that hides which requirement is unproven is the gap this work exists to close, and a per-RFC aggregate is the same failure at a smaller scale |
| OD-1 answered: the family is `/quality/rfc-compliance/<stem>/`, the quality subtree | `/reference/rfcs/<stem>/` | Owner decision, 2026-09-01, on the reason already on the record: 39 of the 190 summaries have no row in `docs/features/rfc-status.md`, so the mirrored reference index cannot link them |
| OD-2 answered: the extraction sign-off is published on the per-RFC page | Leave it off | Owner decision, 2026-09-01. The exclusions are the decisions that kept sentences out of the requirement set, so without them the requirement count has no account of what was dropped |
| OD-3 answered: `collectRFCCompliance` is NOT refactored onto the new snapshot in this work | Unify the two walks now | Owner decision, 2026-09-01. It is separable follow-up work; the aggregate page keeps its own collection path and its `rfc.Check` call. Known Limitations carries it |
| OD-4 answered: a summary with no Meta `\| Title \|` row answers empty and no gate refuses it | Refuse the summary | Owner decision, 2026-09-01. A refusal is a new gate, which this spec's scope excludes. The 22 summaries without a row were backfilled, and `TestEverySummaryCarriesATitleRow` keeps the empty answer unreachable |
| `./le site check` takes the same `output <directory>` keyword `./le site build` takes | Leave the check pointed at the published tree only | AC-13 asks for a clean check after a full build, and a session must not build over `../gh-pages`, which is the published artifact another checkout holds. A check that could judge only the default artifact could not answer for the artifact the sibling verb had just produced. The default is unchanged |
| `rfc.RequirementRows` is the ONE producer of a requirement's six cells, and `RenderShards` formats what it answers | Assemble the cells again in the site package from `RenderInput` | R-6 asks for a test that makes drift red. Deriving both from one function makes drift impossible instead, which is the stronger answer and costs one refactor of `shardRow` |
| `internal/le/testhealth` reads the ledger's own `Annotated` and `No test` columns instead of deriving the remainder by subtraction | Leave the subtraction and let the site build refuse | BLOCKING and fixed here: `collectRFC` compared the live annotation split against `gated - both`, which is the annotated population only when the RFC gate is green. 133 enrolled gated MUSTs carry no test today, so the producer refused and `./le site build` could not finish. The columns are right there in the row this code already parses (`ai/rules/principles.md`) |
| `ProofStates()` is NOT exported | Export it as the Export Surface table names | Nothing outside a test calls it. `ai/rules/completion.md` requires a non-test caller for every exported symbol, and `./le doc wiring` refuses one without. The page shows a proof state's word beside the `Detail` sentence that explains it, so a legend of nine bare words would add nothing |

## Known Limitations

- The build derives the RFC ledger twice: `publishRFCLedger` walks it for the detail pages,
  and `collectRFCCompliance` walks it again for the aggregate page. Unifying them means
  changing an existing producer's test seam, which is separable work. OD-3.
- `rfc/requirements/README.md` and `rfc/audit/README.md` do not exist, and `rfc/README.md`
  disagrees with `docs/features/rfc-status.md`. Neither is made wrong by this change, so
  each gets a journal row rather than a fix here (`ai/rules/completion.md`).
- 39 summary stems have no row in `docs/features/rfc-status.md`. This work makes the
  omission visible on the published index and does not close it.
- No page-size, link-check or orphan-page budget exists in `internal/le/site`. This work
  adds none; it measures and reports (phase 6). Measured over this checkout on 2026-09-01,
  against the numbers R-1, R-2 and R-3 predicted: the largest stem page is `rfc7296` at
  338 KB of HTML and 283 KB of Markdown (R-1 predicted over 200 KB); `llms-full.txt` grew
  from 4.3 MB to 9.3 MB (R-2 predicted 5.5 MB, so it is 3.8 MB larger than the risk row
  estimated); `data/search-index.json` grew from 1.9 MB to 3.4 MB (R-3 predicted over
  3 MB); `data/rfc-requirements.json` is 7.1 MB, which is the requirement text plus every
  excluded extraction quote; and `/quality/rfc-compliance/index.md` grew from 4 KB to
  43 KB (R-10 predicted about 30 KB).
- `./le rfc check` is RED over this checkout and was red before this work: 147 enrolled
  gated MUSTs carry no test and no annotation, two `rfc7606` audit verdicts are stale
  against a changed interop checker, one `rfc2865` discrimination record no longer
  verifies, and one `rfc8671` requirement maps to no extraction site. The published pages
  state every one of those weaknesses, which is what the disclosure ruling asks for.

## RFC Documentation (Scope: protocol)

Not applicable. This spec implements no protocol behavior and adds no enforcing code, so
no `// RFC NNNN Section X.Y:` comment is owed.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/le/site`, `internal/le/rfc`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior (N-A with the reason in the TDD plan)
- [ ] Interop tests for protocol features (N-A: no wire-visible behavior changes)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
