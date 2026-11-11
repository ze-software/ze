# Spec: Homepage project metrics

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Website presentation and repository statistics do not block release correctness. |
| RFC gaps | - |
| Depends | - |
| Phase | independent closure; final checks and visual evidence pending |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

## Task

Give visitors an immediate sense of Ze's size in the homepage hero, replacing
its four repeated navigation actions with four generated metrics. Preserve the
product demonstration, release notice, existing testing evidence and useful
navigation paths.

### Owner request and approval

On 2026-11-11 Thomas requested:

> The frontpage does not provide a easy way to see the size of the project like the activity page does, can you please provide some of these key metric nicely on the page
> It may be well place in the location of where "why ze exists" is, as we can probably remove join discord (have it in the text and search the site (as it is at the top), watch a demo is now present in the player so it brings little.
>
> Think and tell me how you propose we change the frontpage to make it better.

After the proposal to show Go source lines, Go test lines, plugins and CLI
commands, with compact hero styling and the retained text links, Thomas approved:

> ok do it

On 2026-11-11 Thomas authorized the one-off work-in-progress exception:

> yes authorise

The active count had grown to 15 before the claim succeeded, so this claim used
a cap of 16. This authorizes this spec's admission only; it does not change the
standing project cap or authorize exceptions for other work.

The illustrative values from the 2026-11-11 artifact were 1.57M source lines,
1.84M test lines, 96 plugins and 499 commands. They are examples, never literals
to maintain. Source lines include development tooling; they are not a count of
the shipped binary's code.

## Required Reading

- [ ] `ai/doors/system.md` and `ai/doors/le-tooling.md`.
  → Constraint: work and scoped tests stay in main; no product runtime or plugin API change.
- [ ] `docs/contributing/gh-pages.md` and `website/AI.md`.
  → Decision: edit the native homepage template, generate its mirror, rebuild the canonical artifact without pushing.
- [ ] `docs/architecture/site-facts.md`.
  → Constraint: a repository fact that walks files is derived through the existing committed repository-facts update/check mechanism; the site build reads the committed result.
- [ ] `website/style-guide/index.html`, Design principles and component rules.
  → Decision: compact blue hero band, prominent tabular figures, neutral labels and responsive columns; preserve maturity disclosures.
- [ ] `ai/rules/evidence.md`, `ai/rules/planning.md`, `ai/rules/architecture.md`, `ai/rules/spec-no-code.md` and `ai/rules/repo-maintenance.md`.
  → Constraint: share the existing inventory definition, account for all JSON consumers, and use no code snippets in this spec.

**Key insights:** activity already separates first-party source, tests and vendor;
the homepage already substitutes snapshot tokens; browser replacement is generic.
The repository-facts gate must compare the same precision that the new band shows.

## Current Behavior

**Source files read:**

- [ ] `internal/le/site/homebody.go`, `homeTemplate`: four hero action links beside the mascot/release notice; separate testing proof section below.
- [ ] `internal/le/site/home.go`, `renderHome`, `homeBody`, `homeMirror`: reads the generated facts, substitutes tokens and derives Markdown from HTML.
- [ ] `internal/le/repo/rewrite/activity.go`, `collectGoStats`, `addGoFile`: tracked Go files, first-party source/tests split, vendor separate, total lines include comments and blanks.
- [ ] `internal/le/repo/rewrite/activitymeasure.go`, `MeasureActivity`: activity history plus the checkout's Go inventory.
- [ ] `internal/le/site/activity.go`, `renderActivityPage`, `activityGoPanelHTML`: publishes that inventory without a committed repository-facts intermediary.
- [ ] `internal/le/site/facts/sitefacts.go`, `derive`; `facts/staleness.go`, `compare`, `render`: committed data derivation and comparison at published precision.
- [ ] `internal/le/site/facts.go`, `factsFromRepositoryFile`, `factsFromSiteData`, `factsFromCommandSurfaces`: committed repository facts, plugin registration count and CLI catalog count.
- [ ] `internal/le/site/factmark.go`, `siteFactSnapshot`, `markedFact`; `factfallback.go`, `markedValue`: generic scalar bindings and validation.
- [ ] `website/assets/js/site.js`, `lookupFact`, `loadSiteFacts`, `fillRepoStats`: fetches one snapshot and replaces marked text safely.
- [ ] `internal/le/site/paths.go`, `Paths`; `build.go`, `refreshNativeSurfaces`: shared paths and facts publication before registered producers.
- [ ] `internal/le/site/browsertest/fill_test.go`: real-browser coverage of generated scalar bindings and snapshot reload.

**Preserve:** hero/player and quickstart, mascot/release notice, proof section,
header search, existing snapshot keys, standalone activity semantics, mirrors and
other sessions' quickstart and website documentation changes.

**Change:** replace only the four hero action tiles; retain Why Ze exists as an
introductory text link and Discord as a linked sentence; add the four figures and
their definitions/destinations.

Related-work search covered homepage/activity terms in post-release engineering,
proposals and journals. The existing terminal-demo showcase work owns recordings;
this spec changes neither recordings nor their manifest selections.

## Existing Mechanisms

| Mechanism | Where | Today | Disposition |
|-----------|-------|-------|-------------|
| Go inventory | `rewrite/activity.go`, `collectGoStats` | Separates source, tests, vendor | Extend its callable surface to share this collector with repository-facts derivation; do not copy the scanner. |
| Committed repository facts | `site/facts/sitefacts.go`, `derive` | One writer/checker counter and provenance | Extend with source/test total lines, using the existing dirty-tree warning. |
| Published snapshot | `site/facts.go`, `publishSiteFacts` | One JSON file before all page rendering | Extend repository fields and compact display fields; retain plugin/command keys. |
| Scalar markers | `site/factmark.go`, `markedFact`; `site.js`, `fillRepoStats` | Server fallback and browser refresh | Extend through new keys only; no new name switch or browser formatter. |
| Hero action layout | `homebody.go`, `homeTemplate`; `30-components.css` | Four columns, two on mobile | Replace actions with metric links using the same section surface. |
| Build-local memoization | `paths.go`, `Paths.testHealth` | Shares an expensive test-health render | Does not apply: line counts are committed facts, so build-time caching would bypass their provenance boundary. |

## Data Flow

### Entry Point

Existing `./le site facts update` derives the repository counts; existing
`./le site build` publishes the homepage. No new command or route is introduced.

### Transformation Path

1. Expose the existing default Go inventory collection as a read-only callable
   operation in `internal/le/repo/rewrite`, with no duplicated file classification.
2. Extend repository-facts derivation with `repo.go_source_lines` and
   `repo.go_test_lines`, committed-data category and explicit source sentences.
3. Record them through the existing update action and dirty-tree warning. Extend
   the existing check to compare these keys at their actual compact display precision.
4. Read the committed raw values in `factsFromRepositoryFile`; publish them and
   `repo.go_source_lines_display` / `repo.go_test_lines_display`. Compact millions
   use two decimal digits, nearest hundredth of a million; smaller values remain
   intelligible and zero renders as zero. Share that formatting rule with the
   checker rather than letting it approximate a different displayed precision.
5. `homeTemplate` names the two display keys and existing `plugin_registrations`
   and `cli_commands` tokens. Generated HTML and its derived mirror carry the
   values; the existing browser code refreshes the four marked scalar elements.

### Boundaries Crossed

| Boundary | How | Verification |
|----------|-----|--------------|
| Repository → committed data | Existing facts update/check actions | Fixture source/test/vendor classification and visible-precision tests. |
| Committed data → published JSON | Existing `factsFromRepositoryFile` | Snapshot integration fixture with distinct sentinel values. |
| JSON → HTML/browser/Markdown | Existing token, marker and mirror functions | Homepage rendering, browser reload and mirror tests. |

### Changed-Wire Census

**Search evidence:** searched Go, JS, shell, Python, JSON, HTML, Markdown and
workflow sources under internal, website, demos, test and .github for
site-facts.json, repo-facts.json, siteFactSnapshot, loadSiteFactSnapshot,
parseSiteFactSnapshot, data-ze-stat, plugin_registrations and cli_commands.
Positive controls were `publishSiteFacts` and `fillRepoStats`. The wire is an
additive repository/published JSON extension, not a product protocol.

| Wire | Role | Carrier | Source evidence | Disposition |
|------|------|---------|-----------------|-------------|
| repo-facts.json | writer | `facts/sitefacts.go`, `derive`, `write` | JSON-encodes the Facts map | Add raw source/test keys with category/provenance. |
| repo-facts.json | reader | `facts/staleness.go`, `read`, `compare` | Decodes and compares all derived facts | Extend new-key formatting comparison; preserve old-key precision. |
| repo-facts.json | reader | `site/facts.go`, `factsFromRepositoryFile` | Required-key list and typed assignments | Require and publish both new values; missing keys name existing update action. |
| site-facts.json | writer | `site/facts.go`, `renderSiteFacts`, `publishSiteFacts` | Marshals siteFacts; preserves unchanged timestamps | Extend typed repository section; unchanged snapshot still produces stable bytes. |
| site-facts.json | reader | `factmark.go`, `loadSiteFactSnapshot`, `parseSiteFactSnapshot`; `factfallback.go`, `markedValue` | Generic dotted-key traversal and type validation | Preserve generic behavior; new model fields become valid markers automatically. |
| site-facts.json | reader | `home.go`, `homeBody`, `homeMirror` | Template substitution and HTML-derived mirror | Bind new band to all four scalar keys, retain readable non-JS output. |
| site-facts.json | reader | `site.js`, `lookupFact`, `fillRepoStats` | Fetch and textContent replacement | Preserve generic reader; exercise all four new-band bindings through two snapshot versions. |
| site-facts.json | reader | `nav.go`, `commands.go`, `equivalents.go`, `derived.go`, `llmsdata.go`, `health.go`, `authored.go`, `docs.go`, `blog.go` | Existing readers load generic snapshot or existing keys | Preserve all existing fields and output contracts; no migration of unrelated pages. |
| Both JSON files | writer/reader | `facts/sitefacts_test.go`, `facts_test.go`, `factmark_test.go`, `home_test.go`, `testdata/published-site-facts.json`, `browsertest/fill_test.go` | Fixture producers and assertions supply/consume snapshots | Add required keys and focused assertions without deriving goldens from changed output alone. |

### Integration Points

Expose inventory collection, extend the existing facts schema, and reuse the
homepage's existing registration and token substitution. No change to Paths,
producer registration, activity history or frontend key lookup is needed.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Committed facts → published snapshot → existing homepage renderer. |
| No unintended coupling | Yes | Development tooling only; no daemon changes. |
| No duplicated functionality | Yes | Existing inventory scanner and scalar bindings retained. |
| Zero-copy preserved where applicable | N-A | Static site generation, no packet path. |
| Registration over hardcoding, outbound | Yes | Existing home producer remains the sole homepage route owner. |
| Registration over hardcoding, inbound | Yes | Generic dotted-key readers discover the model's keys; no browser key list added. |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | The four metrics and placement are approved | Owner request and approval above | Product layout needs redesign | Main-thread approval already received | confirmed |
| A-2 | Repo source lines include tooling and comments | `collectGoStats`, `addGoFile` | Label overstates product-only code | Source inspection and mixed-file fixture | confirmed |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Display precision and staleness comparison diverge | Boundary fixture changes visible string without gate difference | Share the new line-count formatter and test the visible rounding transition. |
| R-2 | Regeneration captures another session's in-progress files | Existing update action warns about dirty Go files | Inspect warnings; preserve unrelated data; disclose provenance rather than claim a fresh committed measurement. |
| R-3 | Hero overflows or numbers become illegible | Mobile/zoom or dark/light browser view | Four desktop columns, two mobile columns, tabular numerals and neutral labels. |
| R-4 | Fixture refresh hides an unrelated content regression | Broad golden diff beyond new band | Preserve independent assertions; judge any expected-output change under existing test rules. |
| R-5 | Activity and homepage snapshots differ in time | Live activity inventory differs from recorded facts | Use the same counting definition; describe homepage figures as recorded repository counts, never claim exact synchronization with a later working tree. |
| R-6 | Concurrent quickstart/docs work gets overwritten | Shared-file diff includes unrelated hunks | Edit only band/link/docs hunks and commit only owned hunks. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Website statistics, links or layout; product routing unaffected. |
| Recovery | Correct the owned source hunks and rebuild; never destructive git commands. |
| Shared paths | Other sessions are editing homebody quickstart content and website/AI.md. |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Existing homepage producer | → | `renderHome` → `homeBody` → snapshot substitution | `TestHomeProjectMetricsUseSnapshotValues` |
| Existing facts update derivation | → | `derive` → shared Go inventory | `TestRepositoryFactsIncludeGoLineInventory` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Homepage rendered with distinct snapshot figures | Four metric links show Go source lines, Go test lines, plugins and CLI commands; every numeral is a marked snapshot scalar and changes with fixture data | Existing proof metrics remain because they explain quality rather than footprint. |
| AC-2 | Repository with tracked source, test and vendor Go files | Recorded source/test raw totals match the existing activity collector; comments/blanks count and vendor does not | Tooling source remains counted because this describes the repository. |
| AC-3 | Hero links followed | Line metrics and detail link reach project/activity/, plugins reach reference/plugins/, commands reach reference/cli/ | Header search and demo/player navigation remain available because only duplicate action tiles are replaced. |
| AC-4 | Homepage read without JavaScript | Four fallback figures, definitions and Markdown mirror remain readable; Why Ze exists and inline Discord links remain discoverable | Mascot/release notice, player, quickstart and proof content remain because they retain separate purposes. |
| AC-5 | Desktop/mobile, both themes and keyboard | Compact blue-hero band uses four columns on desktop and 2×2 on mobile; links have visible focus and no horizontal overflow at 320px | Existing responsive layout elsewhere remains unchanged. |
| AC-6 | Site build/check after changed data | Canonical ../gh-pages publishes the new band and mirror; check reports no new missing markers, links or artifact-boundary defects | Existing unrelated failures are named with evidence; no test is weakened. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRepositoryFactsIncludeGoLineInventory` | `internal/le/site/facts/golines_test.go` | Source/test/vendor population through existing derivation. | PASS; initial red also observed |
| `TestGoLineDisplayPrecisionMatchesStaleness` | `internal/le/site/facts/golines_test.go` | Same new-key formatting boundary in display and checker. | PASS; initial red also observed |
| `TestGoLineFormattingBoundaries` | `internal/le/site/facts/golines_test.go` | Compact unit transitions, zero and maximum int. | PASS |
| `TestPublishedGoLineFactsUseRecordedInventory` | `internal/le/site/golinefacts_test.go` | Published raw/display values come from recorded sentinels. | PASS |
| `TestRecordedGoLinesDistinguishMissingNegativeAndZero` | `internal/le/site/golinefacts_test.go` | Missing/null values and negative totals are refused; measured zero remains valid. | PASS |
| `TestHomeProjectMetricsUseSnapshotValues` | `internal/le/site/home_metrics_test.go` | Homepage entry point binds four sentinel values and destinations. | PASS; initial red also observed |
| `TestHomeProjectMetricsPreserveNavigationAndRelease` | `internal/le/site/home_metrics_test.go` | Useful inline links, release notice, proof and demo survive. | PASS; initial red also observed |
| `TestHomeProjectMetricsMirrorIsReadable` | `internal/le/site/home_metrics_test.go` | Markdown carries values, definitions and destination links. | PASS; initial red also observed |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Go line total | Nonnegative int | Zero, just below/at compact-unit and hundredth-million rounding transitions | Negative value refused if presented as new committed fact | Overflow prevented by integer-safe formatting |
| Required committed key | Present with numeric value | Zero remains a measurement | Missing key names existing update action | N-A: no new external numeric input |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestTheBrowserFillsHomeProjectMetrics` | `internal/le/site/browsertest/home_metrics_test.go` | All four band figures refresh from two successive snapshots using real site.js. | PASS |
| Canonical build and check | Existing site commands | Generated homepage/mirror and linked detail pages are reviewable. | Build exit 0; final check pending concurrent builder completion |

No product RPC or protocol changes: .ci and interop scenarios are inapplicable.
Use scoped Go tests and the existing browser harness; do not run any le verify.

## Files to Modify

- `internal/le/repo/rewrite/activityinventory.go`: expose the existing inventory collector without copying its logic; activity.go remains untouched because another session owns changes there.
- `internal/le/site/facts/sitefacts.go`, `staleness.go`: new recorded raw facts and shared display-precision comparison; dedicated new tests are listed under Files to Create.
- `website/data/repo-facts.json`: regenerate through the native action.
- `internal/le/site/facts.go`, `facts_test.go`: required new keys, published raw/display values and fixture integration.
- `internal/le/site/homebody.go`: metric band and inline navigation; `home_test.go` gains fixture input keys only. Dedicated new homepage assertions live in `home_metrics_test.go`.
- `internal/le/site/testdata/published-site-facts.json` and homepage fixtures as required by actual fixture readers: additive values and owner-approved presentation only.
- `internal/le/site/browsertest/home_metrics_test.go`: new scalar bindings exercised through the existing real-browser fixture helpers; `fill_test.go` remains unchanged.
- `website/assets/css/30-components.css`, `60-responsive.css`: scoped band styles.
- `website/AI.md`, `docs/architecture/site-facts.md`, `docs/contributing/gh-pages.md`: definitions, provenance, display precision and replaced navigation description with source anchors.
- `ai/INDEX.md`: extend existing website/site-facts discovery description if needed to find Go line metrics.

## Files to Create

The implementation adds `activityinventory.go` under rewrite, `golines.go` and
`golines_test.go` under site/facts, `golinefacts_test.go` and
`home_metrics_test.go` under site, and `home_metrics_test.go` under browsertest.
These extend existing package owners rather than introducing another subsystem.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | Static website only. |
| YANG validation constraints | N-A | No configuration. |
| YANG custom validators | N-A | No configuration. |
| CLI commands/flags | No | Reuses existing site facts/build/check actions. |
| CLI grammar | N-A | No grammar change. |
| Editor autocomplete | N-A | No editor change. |
| Functional test for new RPC/API | N-A | No product endpoint. |
| Pipe completeness | N-A | No new output surface. |
| Env registration | N-A | No environment setting. |
| Doctor check for runtime dependencies | N-A | No new product dependency; existing repository Git/read paths only. |
| Doctor code per problem class | N-A | No daemon refusal or log/report event. |
| Prometheus counters | N-A | Static site facts, no telemetry. |
| BGP family surface | N-A | No protocol change. |

### Documentation Update Checklist

| # | Question | Applies? | File / reason |
|---|----------|----------|---------------|
| 1 | Product feature scope/evidence/level | No | Presentation of existing repository facts only. |
| 2 | Config syntax | N-A | No configuration change. |
| 3 | CLI command | No | Existing command syntax retained. |
| 4 | API/RPC | N-A | No product endpoint. |
| 5 | Plugin | No | Registration population only displayed. |
| 6 | User guide | Yes | `docs/contributing/gh-pages.md`: homepage generation and navigation. |
| 7 | Wire format | Yes | `docs/architecture/site-facts.md`: additive repository and published JSON fields. |
| 8 | Plugin SDK/protocol | N-A | No plugin transport. |
| 9 | RFC behavior | N-A | Existing RFC proof section preserved. |
| 10 | Test infrastructure | No | Existing Go/browser harness reused. |
| 11 | Daemon comparison | N-A | No daemon changes. |
| 12 | Internal architecture | Yes | `website/AI.md` and `docs/architecture/site-facts.md`: provenance and display contract. |
| 13 | Route metadata | N-A | No metadata schema. |
| 14 | Prometheus | N-A | No telemetry. |
| 15 | Registered inventory | No | Counts existing registrations, adds none. |
| 16 | Source anchors | Yes | Search found homepage anchors in gh-pages.md and website/AI.md, facts anchors in site-facts.md; feature-maturity.md factsFromSiteData anchor remains accurate because feature semantics stay unchanged. The core-design.md Design header on facts/sitefacts.go remains accurate: this extends existing tooling with two facts and changes no command engine or component architecture. Run native citation inventory during implementation. |
| 17 | Config/CLI/API examples | No | Source-anchor search found build/demo instructions; existing commands remain valid. |

### Mechanical Discovery Checklist

| Need | Existing mechanism and action |
|------|-------------------------------|
| First discovery | `ai/INDEX.md` website and site facts rows point to owning docs; add metric keywords to that row if absent. |
| Regression rule | `ai/rules/evidence.md` and `docs/architecture/site-facts.md` require one derivation and marked published values. |
| Inventory | Existing repository-facts map and siteFacts model; browser remains generic. |
| Proof | Focused derivation/homepage/browser tests plus canonical site build/check. |

## Implementation Steps

1. **Wiring first, within one coherent implementation phase:** confirm the
   registered homepage route reaches `renderHome`; add the failing snapshot
   sentinel test before changing data or markup. No new stub route is needed.
2. Extend the existing inventory/facts chain and precision checks, add the metric
   band and retained inline links, then update docs and fixtures in the same
   phase. Preserve other sessions' hunks. Run focused tests, regenerate native
   facts, and build/check the canonical artifact; inspect desktop/mobile themes.
3. Hand the complete owned diff to an independent reviewer for both code and
   spec lenses, fix real findings, then close through the existing commit path.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Definitions | Source is non-test first-party Go including tooling; test is _test.go; both include comments/blanks and exclude vendor. |
| Data | No literal illustrative totals; all four band values are snapshot scalars; new line facts enter through committed data. |
| Preservation | Demo, quickstart, release, proof and useful links remain reachable. |
| Precision | New line display/checker agree without changing rounding of unrelated keys. |
| Shared work | Only owned template/docs hunks are committed. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Recorded/published Go facts | Scoped facts and site package tests. |
| Metric band and mirror | Homepage sentinel and mirror tests; canonical build/check. |
| Browser refresh and layout | Existing browser package tests and artifact inspection at desktop/320px in both themes. |
| Accurate docs | Source-anchor/citation inventory and review against producers. |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Scalar injection | Existing escaped spans and textContent remain the only dynamic rendering. |
| External link | Discord retains existing fixed destination and safe external-link attributes. |
| Resource use | No browser file walks or new network requests; build reads committed line facts. |

### Failure Routing

| Failure | Route |
|---------|-------|
| Wrong classification/rounding | Fix producer or shared formatter; do not weaken expectations. |
| Missing snapshot key | Extend canonical derivation/fixture and regenerate through existing action. |
| Unrelated test red | Identify owner/cause and report scoped evidence; preserve shared work. |
| Work-in-progress cap refusal | Keep spec ready and report exact admission refusal to owner; do not bypass cap. |

## Review Gate

### Code Review Runs

| Run | Scope | Reviewer | Verdict | Evidence |
|-----|-------|----------|---------|----------|
| 1 | Owned source, CSS, documentation and generated repository facts | Independent homepage_review agent | CLEAN: 0 BLOCKER, 0 ISSUE | Inventory/precision, validation/security and presentation/preservation producer reads. |

### Spec Review Runs

| Run | Scope | Reviewer | Verdict | Evidence |
|-----|-------|----------|---------|----------|
| 1 | Task, six ACs, changed-wire census, tests and preserved behavior | Independent homepage_review agent | CLEAN: 0 BLOCKER, 0 ISSUE | Dedicated test locations reconciled; pending execution evidence is recorded separately. |

| Field | Value |
|-------|-------|
| Artifact | Main-thread hash-pinned record pending; no artifact claimed yet. |
| `./le spec review check` | Not run yet. |
| Rounds | 1 |
| Reviewer lenses used | Inventory/formatter correctness; validation/security; spec, presentation, style and preservation. |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

No BLOCKER or ISSUE. The test-location documentation NOTE was corrected without changing code or assertions.

## Design Insights

The committed-facts architecture governs the new homepage line metrics even
though the older activity page still reads a live inventory. Reusing that older
page's scanner does not authorize bypassing the newer publication boundary.

The following paragraphs preserve intermediate execution history. Current
results are recorded in Execution Evidence and Limits and Closure Readiness.

The main thread observed the five initial tests fail for the intended reasons:
both recorded keys absent, the visible rounding boundary unreported, and the
homepage missing the band, definitions and mirror values. Its canonical session
scratch log is `job-homepage-metrics-red-c4fc3ad1.log`. The implementation agent
runs no tests, builds or formatters under `ai/rules/commands.md`; the main thread
owns subsequent scoped evidence.

The first post-implementation scoped-run attempt executed no tests: another
session's report producer had six mismatched arguments and blocked launcher
compilation. The main thread observed the producer corrected before retrying.
Post-implementation verification remains pending; this is not a test result.

The main thread reported an independent review with zero BLOCKERs and zero
ISSUEs. Its documentation NOTE is addressed by the test locations above. Native
checks remain pending: the second launcher attempt failed on another session's
doctor-coverage wrapper/model mismatch while that source was still settling.
Neither failed launcher attempt executed the post-implementation tests. The
spec remains in-progress; main owns the native checks, review record and closure.

## Owned journal rows

| Class file | Date | Surface |
|------------|------|---------|

None; this task adds presentation rather than claiming unrelated journal fixes.

## Key Design Decisions

| Decision | Alternatives considered | Rationale |
|----------|-------------------------|-----------|
| Recorded line facts through existing update/check | Measure inventory directly on every site build; cache ActivityWindow in Paths | Preserves committed-data provenance; no new build state or repeated history scan. |
| Four scalar metric links in current hero row | Add a separate dashboard section or six-plus tiles | Gives scale at the requested location and fits existing responsive hierarchy. |
| Compact source/test totals, existing exact small counts | Duplicate existing unit-test/proof counters | Complements existing evidence and matches approved proposal. |

## Known Limitations

Repository size measures source volume, not production maturity or performance.
Recorded line facts and the live activity inventory can represent different
checkout moments; their counting definition remains shared and explicit.

## Checklist

- [ ] Native spec citation inventory checked during implementation.
- [ ] All six acceptance criteria demonstrated with scoped evidence.
- [ ] Independent code and spec reviews recorded before closure.
- [ ] Canonical artifact generated and checked without push.
- [ ] Closure template appended and closure commits completed through le commit.

---

## Implementation Summary

### What Was Implemented

Four linked metrics replace the repeated hero action tiles. Line totals use the
shared activity scanner, recorded repository facts and a shared compact formatter.
Existing scalar bindings provide HTML fallbacks, browser refresh and Markdown.
Scoped CSS supplies four desktop columns, two mobile columns and visible focus.
Why Ze exists and Discord remain text links; player, search, mascot, release
notice and testing proof retain their separate purposes.

### Bugs Found/Fixed

Missing/null numeric values could otherwise appear as measured zero. Pointer
validation of the new keys rejects those states and negative values while
preserving zero, covered by `TestRecordedGoLinesDistinguishMissingNegativeAndZero`.

### Documentation Updates

| Page | Change verified against source |
|------|--------------------------------|
| `website/AI.md` | Band, links and provenance; anchors name `homeTemplate` and `factsFromRepositoryFile`. |
| `docs/contributing/gh-pages.md` | Homepage generation and reader paths; anchors name the same producers. |
| `docs/architecture/site-facts.md` | Definitions and precision; anchors name `derive`, `FormatGoLines`, `displayedFact` and `factsFromRepositoryFile`. |
| `ai/INDEX.md` | Existing site-facts row now names homepage line metrics. |

Main-thread `doc check all` result is pending. Source review found no stale
contract caused by the owned change; unrelated shared edits remain excluded.

### Deviations from Plan

Dedicated test files preserve existing assertions and goldens. Broad site/browser
runs hit the ten-minute timeout in RFC-ledger rebuilding; focused affected tests
passed, as did the complete facts and rewrite packages. This is scoped evidence,
not a whole-site/browser-package green claim.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A present count key appeared sufficient to identify a measurement | Missing/null numeric values decode as zero through an integer field | Producer inspection before the final run | Added pointer validation over the same bytes and dedicated rejection/zero cases. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Show project size early | Done | `homeTemplate`, `factsFromRepositoryFile` | Four generated figures with detail links. |
| Simplify repeated navigation | Done | `homeTemplate` | Why/Discord become text links; header search and player paths remain. |
| Share activity definitions | Done | `MeasureGoInventory`, `derive` | Same scanner; recorded and live inventories can differ in time. |
| Responsive presentation | Implemented; visual confirmation pending | Owned hero CSS | Main is inspecting the canonical artifact. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Snapshot-value rendering and browser refresh tests PASS | Four independent figures and two browser snapshots. |
| AC-2 | Done | `TestRepositoryFactsIncludeGoLineInventory` PASS | Source, tests, vendor, comments, blanks and untracked work. |
| AC-3 | Done | Snapshot-value rendering test PASS; canonical links read | Activity, plugin and CLI destinations. |
| AC-4 | Done | Preservation and mirror tests PASS | Readable values/definitions and retained paths. |
| AC-5 | Pending main visual evidence | Responsive/focus source reviewed | No viewport/theme pass claimed yet. |
| AC-6 | Build done; final checks pending | Native canonical build exit 0; HTML/Markdown read | Site/doc/mechanical checks remain pending. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRepositoryFactsIncludeGoLineInventory` | PASS | `site/facts/golines_test.go` | Full facts-package race run. |
| `TestGoLineDisplayPrecisionMatchesStaleness` | PASS | `site/facts/golines_test.go` | Visible rounding boundary. |
| `TestGoLineFormattingBoundaries` | PASS | `site/facts/golines_test.go` | Zero, unit transitions and maximum integer. |
| `TestPublishedGoLineFactsUseRecordedInventory` | PASS | `site/golinefacts_test.go` | Recorded sentinels. |
| `TestRecordedGoLinesDistinguishMissingNegativeAndZero` | PASS | `site/golinefacts_test.go` | Both keys and all admission states. |
| Three `TestHomeProjectMetrics` tests | PASS | `site/home_metrics_test.go` | Values, links, preservation and mirror. |
| `TestTheBrowserFillsHomeProjectMetrics` | PASS | `site/browsertest/home_metrics_test.go` | Real site script across two snapshots. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Inventory, facts derivation, formatter and snapshot files | Done | Existing package owners extended. |
| Homepage template, CSS and dedicated tests | Done | Shared quickstart hunks preserved. |
| Recorded data and documentation | Done | Native update; factual prose has producer anchors. |
| Existing fixture files | Changed | Additive input keys only. |
| Original broad test-file locations | Changed | Dedicated files carry the planned checks and extra boundary cases. |

### Audit Summary

All implementation deliverables are present; none is omitted or deferred.
Visual confirmation, final checks and the main-thread review/closure actions
remain pending evidence.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Show size immediately | Rendering/browser | Named rendering and browser tests PASS; canonical HTML has marked 1.57M, 1.84M, 96 and 499. |
| Keep figures consistent and inspectable | Derivation/snapshot | Inventory, precision and recorded-sentinel tests PASS; generated JSON has both raw values and provenance. |
| Simplify navigation while retaining useful paths | Rendering/mirror | Preservation and mirror tests PASS; canonical Markdown retains detail/Why/Discord links. |
| Work across viewports/themes | Visual | Main-thread confirmation pending. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|

No in-scope implementation is omitted. Pending verification and closure remain
part of this spec, not deferred work.

## Pre-Commit Verification

### Files Exist

| File | Exists | Evidence |
|------|--------|----------|
| `rewrite/activityinventory.go`, `site/facts/golines.go`, `golines_test.go` | Yes | Source reads confirm collector, formatter and inventory/precision tests. |
| `site/golinefacts_test.go`, `home_metrics_test.go`, `browsertest/home_metrics_test.go` | Yes | Source reads and named PASS output confirm all three carriers. |

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-3 | Four bound figures and destinations | `TestHomeProjectMetricsUseSnapshotValues` PASS. |
| AC-2 | Shared source/test population | `TestRepositoryFactsIncludeGoLineInventory` PASS. |
| AC-4 | Fallback/mirror and retained paths | Preservation and mirror tests PASS. |
| AC-5, AC-6 | Responsive, checked artifact | Visual and final check results pending; canonical build exit 0. |

### Wiring Verified

| Entry Point | Test carrier | Verified |
|-------------|--------------|----------|
| Existing home producer | `site/home_metrics_test.go` | Actual `renderHome` yields bound values/destinations; named test PASS. |
| Existing facts update | `site/facts/golines_test.go` | Existing update action reads mixed-file fixture; named test PASS. |
| Snapshot reload | `site/browsertest/home_metrics_test.go` | Real browser/script reads two snapshots; named test PASS. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Owner approved proposal, implementation and one-off WIP exception. |
| A-2 | confirmed | Collector source and mixed-file inventory test PASS. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Metrics and retained navigation | `homeTemplate` and generated HTML/Markdown | Yes. |
| Recorded definitions and precision | `derive`, `MeasureGoInventory`, `FormatGoLines`, `factsFromRepositoryFile` | Yes. |
| Discovery | Existing site-facts row | Yes; no new command or registry. |
| Product/config/API/protocol categories | Owned diff and source-anchor census | Unchanged; website presentation and repository facts only. |

## Execution Evidence and Limits

Main-thread admitted, tagged `go test -race` runs in the main checkout produced:

| Scope | Directly observed output |
|-------|--------------------------|
| Complete facts package | `ok github.com/ze-software/ze/internal/le/site/facts 3.248s` |
| Complete activity/rewrite package | `ok github.com/ze-software/ze/internal/le/repo/rewrite 1.144s` |
| Browser refresh | `--- PASS: TestTheBrowserFillsHomeProjectMetrics (3.19s)` |
| Five focused affected site tests | All five named tests PASS; `ok github.com/ze-software/ze/internal/le/site 5.416s` |
| Canonical native build | Exit 0; generated homepage and Markdown contain all four values and detail links. |

The broad site/browser package runs timed out after ten minutes in RFC-ledger
rebuilding. Neither whole package is reported green. A narrow retry was
interrupted by main to capture its rebuild stack; it was neither a pass nor an
assertion failure. Earlier concurrent-source launcher failures ran no tests.
No test was weakened and no whole-tree verification was run.

The first canonical site check reported source-only files while another site's
builder was staging into the same artifact checkout. Main observed that
concurrent builder and will regenerate and check the completed canonical
artifact after it finishes. That first result is not reported as a passing
check or suppressed as irrelevant. Documentation checking is still running.

## Closure Readiness

Independent code/spec review and the generated-data supplement are clean.
Final checks, visual evidence and the main-thread review pin remain pending.
Main retains the claim and will perform closure commits, post-commit compile
check and release. No push is authorized. Closure uses `no-news`: website
presentation only, with no operator/runtime change.
