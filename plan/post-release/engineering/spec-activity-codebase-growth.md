# Spec: Activity codebase growth

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P3 |
| Release reason | Correct public project-size reporting; no shipped routing or lifecycle behavior changes. |
| RFC gaps | - |
| Depends | - |
| Phase | design admitted; WIP authorization pending |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

## Task

Make the public activity page show the size and growth of Ze's own codebase.
Replace its cumulative-additions interpretation with the number of first-party
Go source and test lines present in the selected committed tree on each day.
Deletions reduce this inventory, and a same-size rewrite does not grow it.

### Owner requirement

Thomas stated on 2026-11-11:

> So the report on http://10.12.100.120:8000/project/activity/ is BULLSHIT

> I want to show the growth of the codebase NOT the change over time

The public page must answer this requirement throughout its default view,
headline figures, chart, date readout and readable mirror. Correcting only a
secondary tab while retaining an additions headline does not meet the task.
The homepage's full-word units and Zeledon separator are a separate small
correction already being implemented; this spec does not own those changes.
The earlier homepage WIP exception is not authorization to claim this spec.

### Admission record

Main accepted the design after reading the measurement, page, browser and
immutable-object producers. The ACs cover useful historical inventories,
deletions, preserved source and the distinct legacy consumers. The changed-wire
census includes the Go writers/readers, browser code and readable mirrors.

Native claim admission refused on 2026-11-11: 18 specs were already in progress
against a cap of 12. Admission requires a one-off cap of at least 19 or a free
slot. The owner's exception request is pending; no claim or implementation is
authorized by this record alone.

## Required Reading

Reading is complete; unchecked boxes remain template markers.

- [ ] `ai/doors/system.md` and `ai/doors/le-tooling.md`.
  → Constraint: implement and test in main; preserve concurrent sessions and use existing native tooling.
- [ ] `website/AI.md`, Presentation tooling.
  → Decision: replace the public Growth interpretation and documented default; preserve explicitly labeled activity in deck and standalone outputs.
- [ ] `docs/architecture/site-facts.md`, repository line facts.
  → Constraint: source includes tooling; comments and blank lines count; dependencies do not count toward first-party size. Homepage recorded facts retain their existing writer.
- [ ] `plan/README.md`, `plan/templates/TEMPLATE.md`, and `ai/rules/spec-no-code.md`.
  → Decision: post-release engineering, positive ACs, no code snippets, closure tables added only at closure.
- [ ] `ai/rules/architecture.md`, `evidence.md`, `planning.md`, `repo-maintenance.md`, `commands.md` and `context-economy.md`.
  → Constraint: reuse immutable Git object I/O and the registered page producer; main executes all tests and builds.

**Key insights:** calendar additions describe work, not remaining source.
Historical first-party Go size needs its own measured series. Existing immutable
object readers can count daily trees without a checkout or a second batch reader.

## Current Behavior

**Source files read:**

- [ ] `internal/le/repo/rewrite/activity.go`, `collectActivity`, `collectGoStats`, `addGoFile`: additions across source extensions versus current tracked Go inventory.
- [ ] `internal/le/repo/rewrite/activitymeasure.go`, `MeasureActivity`, `measureActivity`, `measureCells`: one year of daily additions and commits, emitted as calendar attributes.
- [ ] `internal/le/repo/rewrite/activityinventory.go`, `MeasureGoInventory`: the existing collector's public inventory entry point.
- [ ] `internal/le/repo/objects/objects.go`, `Resolve`, `Read`, `takeBlob`: immutable reference resolution and checked, length-framed batch blob reads.
- [ ] `internal/le/site/activity.go`, `renderActivityPage`, `activityBody`, `activityWidgetHTML`, `activitySummaryHTML`: registered public route, additions headline and Calendar-first layout.
- [ ] `internal/le/site/activitygrowth.go`, `activityGrowthHTML`, `activityGrowthScript`: browser sums additions from calendar cells, assumes the final point is the maximum and shows a separate current snapshot including vendor.
- [ ] `internal/le/site/activityscript.go`, `activityPageScript`: shared added-lines/commits controls and tooltips.
- [ ] `internal/le/site/activitygrowthstyle.go`, `activityGrowthStyle`: theme, responsive layout and metric visibility.
- [ ] `internal/le/site/presentations.go`, `renderActivity`: deck calendar uses the existing measurement and slide surface.
- [ ] `internal/le/site/activity_growth_test.go`, `activity_growth_integration_test.go`, `activity_test.go`, `presentations_test.go`, and `internal/le/repo/rewrite/sourcerewrite_test.go`: current public wording, browser interactions, historical bounds and legacy output tests.

**Preserve:** the public route, shared site shell, light/dark support, keyboard
and pointer interaction, date selection, readable mirror, and useful commit
calendar. Deck and standalone activity outputs retain their explicit additions
and commit semantics. Homepage recorded facts and their refresh mechanism remain.

**Change:** the public page opens on Codebase Growth with truthful first-party
size figures. Remove cumulative added-lines figures, controls and tooltip text
from that public page. A retained Calendar measures commits only. Dependencies
may remain a clearly separate secondary inventory, never part of Ze's own total.

No live related activity/growth spec was found in the work-item folders.
`plan/handover/homepage-publication.md` is a supporting publication record rather
than ownership of this correction. The journal's activity timing and theme rows
explain preservation constraints; this spec creates no lesson row by default.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Immutable Git I/O | `repo/objects/objects.go`, `Resolve`, `Read` | Pins commits and validates batch blob framing | Extend use to historical inventory; do not copy the protocol reader. |
| Go population and line semantics | `rewrite/activity.go`, `collectGoStats`, `addGoFile` | Source/tests split, comments/blanks and exclusions | Share population predicates and content-counting semantics with historical blobs. |
| Activity measurement | `activitymeasure.go`, `ActivityWindow`, `MeasureActivity` | Calendar history and current live inventory | Add an explicit historical Go carrier for the public page; do not reinterpret additions as size. |
| Registered public producer | `site/activity.go`, `renderActivityPage` | Publishes page and Markdown mirror | Extend the same route and producer. |
| Growth interactions | `activitygrowth.go`, `activityGrowthScript` | Pointer, keyboard and slider readout | Replace input series and scale calculation; retain useful interactions. |
| Shared activity surfaces | `presentations.go`, `renderActivity`; rewrite `renderActivityPage` | Explicit legacy calendars | Preserve behavior; avoid charging unused historical inventory work to these surfaces. |
| Homepage line facts | `site/facts/sitefacts.go`, `derive` | Records an inventory snapshot | Does not supply history; preserve its writer and provenance. |

## Data Flow

### Entry Point

The existing registered `/project/activity/` producer requests its displayed
window and one historical Go inventory. No new command, route or external API.

### Transformation Path

1. Resolve the requested reference once to a full immutable commit ID with
   `objects.Resolve`. Every ancestry, membership and blob read uses that pin.
2. Enumerate first-parent ancestry with IDs, tree identities and committer
   timestamps. Author dates do not define when a change entered this history.
3. For each inclusive UTC day, select the ancestry-latest eligible commit whose
   committer timestamp is at or before that day's end. An idle day carries its
   prior tree, and a pre-repository day is zero. This selection applies even
   when a committer clock goes backward; do not sort commits into a synthetic
   parent order or omit the pre-window baseline.
4. List each distinct selected tree with NUL-framed `ls-tree`. Count regular
   Go blobs only, using the existing first-party exclusions including vendor
   and temporary files. Count `_test.go` separately; all other included Go is
   source. Symlinks and submodules are not regular source files.
5. Read previously unseen blob IDs with `objects.Read`, bounded batches, and a
   per-measurement cache of line counts by immutable object ID. Count comments,
   blank lines and a final unterminated line. Empty and NUL-containing blobs are
   counted from their bytes rather than Git's text/binary diff classification.
6. Emit dated source/test values and their sum. The public summary and current
   composition use the latest selected committed point, not mutable checkout
   files. A pinned historical end date never reads future source into its cards.
7. Serialize those points for the public chart. Plot source, tests and combined
   first-party size with clear legend/readout labels; choose scales from every
   point, so an earlier peak and a later decline remain visible. The current
   selected date persists across useful representation changes.
8. Publish the page and its readable mirror through the existing producer. The
   no-JavaScript response states the latest first-party size and measurement
   definition. Commit Calendar remains useful without additions controls.

### Historical Point Contract

| Field | Type | Meaning |
|-------|------|---------|
| Date | UTC date | One inclusive day in the requested window. |
| Source lines | Nonnegative integer | Included regular Go blobs except `_test.go`. |
| Test lines | Nonnegative integer | Included regular `_test.go` blobs. |
| Combined first-party lines | Derived integer | Source plus tests; never includes vendor. |
| Source identity | Retained commit/tree ID or explicit pre-repository absence | Identifies the immutable inventory behind the point. |

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Git history to tooling | Pinned ancestry, tree records and checked blob batches | Planned fixture tests below. |
| Rewrite measurement to site | Explicit historical Go values, independent of calendar additions | Planned producer wiring test. |
| Go renderer to browser | Escaped JSON and text values | Planned rendered and browser tests. |
| Public HTML to Markdown | Existing `htmlToMarkdown` path | Existing mirror test updated with judged expectations. |

### Changed-Wire Census

**Search evidence:** searches covered Go under `internal/le/site` and
`internal/le/repo/rewrite`, repository HTML/JS/JSON/shell/Python carriers under
`internal`, `website` and `test`, and activity documentation. Terms were
`ActivityWindow`, `MeasureActivity`, `activityGrowth`, `activity-growth`,
`data-lines`, `Cumulative added` and `Historical category`. Positive controls
were `measureCells` writing `data-lines` and `activityGrowthScript` reading it.
No separate shell/Python reader of the historical growth data was found.

| Wire | Role | Carrier | Source evidence | Disposition |
|------|------|---------|-----------------|-------------|
| Activity measurement | writer | `activitymeasure.go`, `measureActivity` | Constructs `ActivityWindow` and calendar cells | Preserve additions/commits; introduce explicit public history measurement. |
| Activity measurement | reader | `site/activity.go`, `renderActivityPage`, `activityWidgetHTML` | Calls `MeasureActivity` and renders public page | Obtain pinned Go history; use growth/default first-party summary. |
| Activity measurement | reader | `site/presentations.go`, `renderActivity` | Calls `MeasureActivity`, slide surface | Preserve explicit activity semantics and avoid unused history scan. |
| Activity measurement | reader | `rewrite/activity.go`, `renderActivityPage` | Uses window summaries, cells and Go inventory | Preserve standalone report and options. |
| Calendar attributes | writer | `activitymeasure.go`, `measureCells` | Writes dates, additions and commits | Preserve legacy attributes; public Calendar exposes commits without misleading additions. |
| Calendar attributes | reader | `site/activityscript.go`, `activityPageScript` | Reads both metrics and fills tooltips | Preserve deck behavior; public path cannot expose additions through controls or tooltips. |
| Public growth data | writer | `site/activitygrowth.go`, `activityGrowthHTML` | Currently supplies snapshot and chart containers | Supply explicit dated source/test/combined inventory, not cumulative additions. |
| Public growth data | reader | `site/activitygrowth.go`, `activityGrowthScript` | Currently sums `data-lines` | Read inventory points directly and use whole-series scales. |
| Public HTML/mirror | writer | `activity.go`, `renderActivityPage` | `writePublishedPage`, `htmlToMarkdown` | Publish truthful size wording and accessible fallback. |
| Public HTML/mirror | reader | `activity_test.go`, published HTML/Markdown fixtures | Word/layout parity and mirror assertions | Update expected wording through independent test judging. |
| Public growth data | reader | `activity_growth_integration_test.go`, observation script | Drives charts, metric controls, tabs, slider and themes | Replace churn assumptions with shrinking inventory and retain interaction coverage. |
| Activity measurement | reader | `activity_growth_test.go`, `presentations_test.go`, rewrite `sourcerewrite_test.go` | Direct window fixtures and legacy assertions | Add history fixture data where public output needs it; keep legacy assertions. |

`website/AI.md`, Presentation tooling, documents these data flows rather than
speaking a serialized wire. Update its cumulative-additions and current-only
category descriptions with committed-size semantics and the preserved legacy
surface distinction. The census groups fixture/browser readers with their
actual measurement and growth-data writers; it does not invent separate wires
for the test harnesses that inspect those values.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| Intended data path | Yes by design | Public route consumes the new inventory producer, not browser arithmetic over churn. |
| Component isolation | Yes | Development tooling only; no shipped runtime imports or API changes. |
| No duplicated mechanism | Yes | Reuse `objects.Resolve`/`Read`, Go population semantics and registered site producer. |
| Allocation bounds | Yes by design | Tree listing per distinct selected daily tree; byte batches bounded; cache retains counts, not all blob bodies. |
| Registration outbound | Yes | Existing activity producer owns the existing route; no new registration vocabulary. |
| Registration inbound | Yes | Searches of existing callers/control readers require no shared core switch or command/parser list additions. |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | First-parent committed Go inventory is the useful meaning of codebase growth | Owner requirement and existing homepage Go definition | Wrong population or integration date | Main design admission; source/test/total fixture and public wording review | Design admitted; test evidence pending |
| A-2 | One selected tree per UTC day provides useful detail | Existing year/day activity window | Inadequate temporal detail | Date scrubber/browser inspection and daily endpoint tests | Pending tests |
| A-3 | Main has adequate local history and objects | Existing Git-backed producer | Invented partial history | Missing/shallow-history refusal test and canonical build | Pending tests |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Starting each window at zero loses existing source | Mid-history fixture starts at zero | Count the first selected tree, including all pre-window contents. |
| R-2 | Merge or rename double counts source | Endpoint differs from direct tree inventory | Count exact selected tree membership, not accumulated additions or rename patches. |
| R-3 | Dirty work or moving HEAD changes later reads | Same pinned request changes its result | Resolve once; read only immutable IDs; test movement and dirty files. |
| R-4 | Final value is below an earlier peak | Earlier curve leaves viewBox | Scale from all points; browser regression includes a decline. |
| R-5 | Missing history or malformed objects becomes a false zero | Partial chart looks plausible | Fail explicitly with cause/fix; only genuine pre-repository days are zero. |
| R-6 | Historical scan is unnecessarily large | One read per commit or uncached blob | Select daily endpoints, deduplicate trees and blob IDs, bound batches; no unmeasured performance claim. |
| R-7 | Public fix changes legacy slide/report behavior | Existing deck/standalone test fails | Keep surface-specific rendering and request historical data only where consumed. |
| R-8 | Timestamp skew or window cutoffs misorder data | A pinned end date includes later source | Use documented first-parent eligibility/UTC rule and test skew, same-day ties and both bounds. |
| R-9 | Concurrent edits enter this commit | Prepared diff includes foreign homepage/tooling hunks | Main captures baseline and commits only owned paths/hunks. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Public project-size charts and mirror can mislead; no routing/session behavior changes. |
| How is it corrected? | A focused forward fix through native commit tooling; destructive git commands remain forbidden. |
| Who else touches this path? | Other website and tooling sessions; preserve their markup, styles and source changes. |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Registered public activity producer | → | Historical inventory and public page | `TestActivityPagePublishesCommittedCodebaseGrowth` |
| Growth controls in rendered page | → | Historical series/readout | `TestActivityGrowthShowsHistoricalCodebaseSize` |
| Existing deck/standalone actions | → | Original explicit activity surfaces | `TestActivitySlideKeepsCalendarRepresentation`, `TestActivityDashboardFixture` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Daily source additions, rewrites and deletions | Source/test/combined values equal the regular first-party Go inventory in each selected tree; deletion lowers size and same-size rewrite preserves it | Comments, blanks and tooling remain counted because this measures repository size. |
| AC-2 | Window begins after repository creation or contains idle/pre-repository days | First point includes its pre-window inventory; idle dates carry size; genuine pre-repository dates are zero | Existing contents survive the window boundary because they remain in the codebase. |
| AC-3 | Merges, renames, moves across exclusions and varied blob bytes | Exact tree membership determines size and source/test classification; empty, final-unterminated and NUL-containing files count correctly | Included regular Go blobs remain counted regardless of Git's diff classification; vendor, temporary files, symlinks and submodules remain excluded. |
| AC-4 | Selected ref, pinned dates, timestamp skew, dirty files and concurrent HEAD movement | One immutable requested history supplies all points and headline values; later/uncommitted source does not enter that result | Earlier committed source persists until its selected descendant changes it; no fabricated data for missing history. |
| AC-5 | Visitor opens public page with or without JavaScript | Codebase Growth is the default; current first-party source/tests/total and definitions are readable; public added-lines headlines/controls are replaced | A commits-only Calendar and clearly separate dependencies may remain because they cannot inflate the first-party size. |
| AC-6 | Scrubber, pointer, keyboard, themes, mobile width and declining history | Selected date and three inventory values agree; earlier maximum and later decline fit the chart; focus and mobile layout remain usable | Commit activity remains independently inspectable; it is not mislabeled codebase size. |
| AC-7 | Public build/mirror and legacy deck/report render | Canonical page and mirror agree on inventory semantics while existing explicit legacy activity behavior remains | Added-line measurements survive only on the clearly labeled legacy surfaces whose callers still use them. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGoGrowthCountsRemainingSourceAndTests` | New `rewrite/activitygrowth_test.go` | Add, same-size rewrite and delete against exact fixture trees. | Planned |
| `TestGoGrowthCarriesBaselineAndIdleDates` | Same | Pre-window inventory, idle dates and zero before repository creation. | Planned |
| `TestGoGrowthCountsMergeAndRenameTrees` | Same | First-parent merge result, rename and excluded/test path transitions. | Planned |
| `TestGoGrowthCountsBlobLineBoundaries` | Same | Empty, blank, comments, CRLF, missing final newline and NUL bytes. | Planned |
| `TestGoGrowthPinsHistoryAndWindow` | Same | Requested ref, moving HEAD, dirty files, UTC bounds and skewed timestamps. | Planned |
| `TestGoGrowthRefusesIncompleteHistory` | Same | Missing/shallow history and unavailable objects cannot produce a plausible partial curve. | Planned |
| `TestActivityPagePublishesCommittedCodebaseGrowth` | New `site/activity_codebase_growth_test.go` | Registered producer publishes measured points and committed-size default summary. | Planned |
| `TestActivityPageOffersGrowthRepresentation` | Existing `site/activity_growth_test.go` | Public default, useful source/test/total values and commit Calendar. | Update with judged expectations |
| `TestActivitySlideKeepsCalendarRepresentation` | Existing same | Legacy slide retains its explicit calendar. | Preserve |
| Existing public wording/mirror tests | `site/activity_test.go` | HTML and Markdown say the new approved meaning. | Update fixtures with judge |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Window | Inclusive ordered UTC dates | One-day window and commit at 23:59:59 UTC | End before start | Positive day-count overflow rejected if exposed |
| Blob lines | Zero through actual blob content length | One unterminated line; trailing newline adds no extra line | Negative values impossible | Malformed/truncated object framing refuses |
| Point selection | First through final displayed day | End-of-day commit included | Genuine pre-repository day zero | Next-day commit excluded |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestActivityGrowthShowsHistoricalCodebaseSize` | Existing `site/activity_growth_integration_test.go` | Real Chromium drives declining history, three values, date controls, Calendar, themes and responsive chart. | Planned replacement of old cumulative expectation, independently judged |
| Canonical build/check and visual inspection | Existing native site actions | Actual page/mirror and linked homepage remain useful in desktop/mobile light/dark. | Main-thread execution owed |

No RPC, API, `.ci` harness or protocol change: a new `.ci` or interop scenario
would not exercise this website more directly than the existing real browser.

## Files to Modify

- `internal/le/repo/rewrite/activity.go`: share existing Go population/content semantics as needed without changing legacy calendar measurement.
- `internal/le/repo/rewrite/activitymeasure.go`: explicit historical carrier, preserving existing additions and commit consumers.
- `internal/le/site/activity.go`: request history, default public Growth, truthful summary/description, commits-only Calendar and mirror.
- `internal/le/site/activitygrowth.go`: serialize and render historical source/tests/combined values and correct declining scales.
- `internal/le/site/activityscript.go`, `activitygrowthstyle.go`: preserve legacy behavior while removing public churn controls/tooltips and supporting revised chart layout.
- `internal/le/site/activity_growth_test.go`, `activity_growth_integration_test.go`, `activity_test.go`: approved behavior and interaction tests.
- `internal/le/site/testdata/published-activity-main.html`, `published-activity.md`: judged public wording fixtures.
- `website/AI.md`, Presentation tooling: committed Go history definition and public versus legacy surfaces.
- `docs/architecture/site-facts.md`, line inventory paragraph: historical committed activity versus recorded homepage facts; retain provenance distinction.
- `docs/contributing/gh-pages.md`, activity publication description where needed: default project-size view and build source.
- `ai/INDEX.md`: existing website/activity discovery row gains historical codebase-size keywords.

## Files to Create

- `internal/le/repo/rewrite/activitygrowth.go`: immutable daily Go-tree measurement using existing object reader.
- `internal/le/repo/rewrite/activitygrowth_test.go`: precise historical fixture tests.
- `internal/le/site/activity_codebase_growth_test.go`: public producer wiring over a historical fixture repository.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | No RPC or configuration. |
| YANG constraints | N-A | No YANG fields. |
| Custom validators | N-A | No config validator. |
| CLI commands/flags | No | Existing native activity/site actions remain. |
| CLI grammar | N-A | No syntax added. |
| Editor autocomplete | N-A | No editor surface. |
| Functional test for new RPC/API | N-A | No endpoint; existing Chromium test covers public interaction. |
| Pipe completeness | N-A | No command answer format changes. |
| Env registration | N-A | No new environment setting. |
| Doctor runtime dependency | No | Git is already the development producer's dependency; no shipped dependency. |
| Doctor code per problem class | N-A | Tooling Git failures return actionable errors through the existing build path; no daemon log/report state or new runtime refusal class. |
| Prometheus | N-A | Static website measurement. |
| BGP family surface | N-A | No protocol behavior. |

### Documentation Update Checklist

| # | Question | Applies? | File / reason |
|---|----------|----------|---------------|
| 1 | Product feature scope/evidence/level | No | Website project history only, no shipped feature claim. |
| 2 | Config syntax | N-A | No configuration change. |
| 3 | CLI command | No | Existing commands and arguments retained. |
| 4 | API/RPC | N-A | No product endpoint. |
| 5 | Plugin | N-A | No plugin change. |
| 6 | User guide | Yes | `docs/contributing/gh-pages.md`: activity publication/default view where described. |
| 7 | Wire format | Yes | `website/AI.md`: page-local historical JSON replaces inferred additions; no product wire. |
| 8 | Plugin SDK/protocol | N-A | No SDK change. |
| 9 | RFC behavior | N-A | No RFC behavior or proof change. |
| 10 | Test infrastructure | No | Existing Go/Chromium harness reused. |
| 11 | Daemon comparison | N-A | No daemon comparison. |
| 12 | Internal architecture | Yes | `website/AI.md`, `docs/architecture/site-facts.md`: immutable history and inventory boundaries. |
| 13 | Route metadata | N-A | No route metadata change. |
| 14 | Prometheus | N-A | No telemetry. |
| 15 | Registered inventory | No | Existing route/producer; `ai/INDEX.md` discovery keywords updated. |
| 16 | Source anchors | Yes | Changed files' Design headers name `website/AI.md`; existing site-facts paragraph about live activity becomes inaccurate. Native anchor inventory remains owed to main before implementation. `objects.go` is reused without changing its spec-workflow contract. |
| 17 | Config/CLI/API examples | No | Source search found presentation build commands; their syntax remains unchanged. |

### Mechanical Discovery Checklist

| Need | Existing path and action |
|------|--------------------------|
| First lookup | Existing `ai/INDEX.md` website/tooling rows point at owning docs; add growth terminology. |
| Regression contract | `website/AI.md` records exact committed-tree semantics; `ai/rules/evidence.md` rejects inferred unsupported figures. |
| Inventory | Existing activity producer owns its route; explicit historical carrier owns point data. |
| Proof | Historical Git fixtures, registered producer test, real-browser interactions and canonical artifact check. |

## Implementation Steps

1. **Wiring first:** add failing `TestActivityPagePublishesCommittedCodebaseGrowth` through the existing route; define the explicit history contract and request it only from the public producer. No new route or placeholder UI.
2. Implement immutable daily tree inventory and its discriminating fixture tests, then replace public additions summaries/default/chart/readout with source/tests/combined inventory. Preserve legacy activity paths and add the page's usable no-JavaScript answer.
3. Update affected browser/wording fixtures through independent test judging; update docs with their producing source anchors. Main runs scoped race/browser tests, native build/check and four viewport/theme inspections.
4. Independent code/spec review, fixes and re-review precede closure. Claim only after the main-thread design/WIP admission; keep it through the native closure script and release afterward.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Meaning | Every public size figure comes from remaining first-party Go blobs, never additions or vendor-inflated overall totals. |
| Dates | Pre-window baseline, idle dates, UTC cutoff, first-parent integration and timestamp skew follow one explicit selection rule. |
| Pin | History listing, membership, blob data and headline point all descend from one resolved ref. |
| Plot | Earlier peaks and later deletions remain within the same honest scale. |
| Legacy | Added-lines semantics remain only where explicitly preserved, without unused historical scans. |
| Shared workspace | Owned source/docs hunks are isolated; no history checkout or copy is created. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Historical inventory producer | Named `TestGoGrowth...` fixture tests through admitted scoped race run. |
| Public truthful default/report | Producer wiring, public HTML/mirror tests and native canonical site check. |
| Useful interactive chart | `TestActivityGrowthShowsHistoricalCodebaseSize` and desktop/mobile theme inspection. |
| Preserved legacy outputs | Existing deck and standalone fixture tests. |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Git references | Resolve once with option-safe existing helper; pass full immutable IDs afterward. |
| Object framing | Reuse `objects.Read`; malformed/missing objects refuse rather than produce zeros. |
| Browser injection | JSON encoding and existing escaped text paths; DOM values use textContent. |
| Resources | Bound blob batches; deduplicate selected trees/objects; no retained corpus of blob bodies or per-commit checkout. |

### Failure Routing

| Failure | Route |
|---------|-------|
| Inventory mismatch | Fix membership/line-count producer; do not weaken exact fixture inventory. |
| WIP admission refusal | Keep ready and return exact refusal to main; no claim bypass. |
| Missing history/object | Explicit cause and recovery instruction; never substitute partial zero data. |
| Public/legacy mismatch | Correct surface selection and rerun affected scoped tests. |
| Unrelated concurrent compile failure | Main reports and waits for settling source; no alternate checkout. |

## Review Gate

### Code Review Runs

| Run | Scope | Reviewer | Verdict | Evidence |
|-----|-------|----------|---------|----------|

### Spec Review Runs

| Run | Scope | Reviewer | Verdict | Evidence |
|-----|-------|----------|---------|----------|

## Design Insights

The observed totals were different measurements, not inconsistent arithmetic.
The defect is using cumulative additions to answer a codebase-size question.
The public source of truth must therefore change, not merely its explanation.

## Owned journal rows

| Class file | Date | Surface |
|------------|------|---------|

No journal row is created merely to record closure or repeat the corrected contract.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Count exact selected daily trees | Replay first-parent additions minus deletions from the root | Daily trees naturally include baseline, merges, rename membership and binary Go blobs without diff-parser exceptions. |
| Reuse immutable object reader with count cache | New cat-file protocol reader or checkout per date | Existing checked framing and immutable IDs already supply the boundary. |
| Change the public default and headline | Correct only the secondary Growth tab | The visitor must meet the requested size report immediately. |
| Preserve separate legacy activity outputs | Change every additions consumer | Existing deck/report callers still ask for activity; they must not be silently reinterpreted. |

### Design Admission Challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | One daily tree inventory and existing object reader; no diff replay state, history checkouts or second counter vocabulary. |
| Uniformity | Existing native producer, explicit measurement carrier, snapshot-style values and generic site publication. |
| Performance | Work is bounded by selected daily trees and unique regular Go blob IDs, not every commit's full checkout. Batch payloads are bounded and cache stores counts; actual runtime is unmeasured. |
| Strongest concern | History traversal and tree listings add build work. Validate the actual canonical build and reduce repeated tree/blob reads before considering a different correctness model. |

## Known Limitations

The metric is committed first-party Go repository size, including tooling,
comments and blanks, rather than shipped binary source or all-language size.
Daily snapshots describe the selected first-parent history under the documented
committer-date rule; they do not reconstruct uncommitted working directories.
These are the declared measurement boundaries, not deferred acceptance criteria.

## Checklist

- [ ] Main-thread design admission and owner WIP exception recorded before claim.
- [ ] Native source-anchor inventory reconciled before implementation.
- [ ] Each AC demonstrated by its named scoped tests and browser/artifact evidence.
- [ ] Independent fixture/test judgment preserves old useful proof while changing approved semantics.
- [ ] Review Gate clean; closure sections completed and native two-commit closure run.

Template checkboxes remain unchecked by workflow rule. This ready artifact
records design, not executed tests, a successful claim, or completed implementation.
