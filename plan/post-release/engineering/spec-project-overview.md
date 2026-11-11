# Spec: Project overview

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling, docs |
| Priority | P3 |
| Release reason | Website explanation and navigation improve evaluation without changing release correctness. |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 — all acceptance evidence verified; Round 2 CLEAN and native review check OK; two-commit landing next |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

## Task

Turn `/project/` into an overview that explains Ze's current position, recent development, release work, history and participation. Let readers reach an authentic demonstration of the software from this overview. Preserve every existing destination and distinguish project activity from production readiness.

### Owner request and approval

Thomas requested on 2026-11-11:

> The page for Project Overview http://10.12.100.120:8000/project/ is rubbish - how do you propose we improve it ?

He approved the six-section proposal summarized below with:

> ok

He then added:

> can we make sure the loading page give us a good overview and allow us to see the software in action

Thomas clarified the same day:

> The part of the main page visible when the page loads before scrolling

This spec also improves the opening homepage viewport: readers see a concise product overview and the genuine terminal demonstration before scrolling on ordinary desktop and laptop screens. On a narrow phone screen, show the concise overview, clear watch action and recognizable recording preview immediately; retain the detailed capability links below rather than hiding or removing them.

### Implementation admission

Thomas explicitly authorized a one-off exception on 2026-11-11:

> Allow a one-off WIP exception for this website change

The claim uses a separate session identity and a cap of 20 for this claim only, following the observed 19 active specs. It never borrows the deployment-user-journeys claim. Homepage files overlap with the deployment-user-journeys and homepage-project-metrics work; preserve their current changes and limit edits to the opening composition.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`, `ai/rules/architecture.md`, `ai/rules/planning.md`, `ai/rules/evidence.md`, `ai/rules/spec-no-code.md`, `ai/rules/writing.md`.
  → Constraint: use the existing producer registry; keep development and scoped tests in main; independent review follows implementation.
- [ ] `docs/contributing/gh-pages.md`, `website/AI.md`, especially producer ownership, Markdown rendering, mirrors and homepage demonstrations.
  → Decision: one registered project producer fills page-local content and uses the shared Markdown renderer and shell.
- [ ] `website/style-guide/index.html`, Design principles and category rules.
  → Constraint: preserve useful information, use varied compositions, retain the blue hero, neutral cards and accessible theme tokens.
- [ ] `plan/README.md`, `plan/templates/TEMPLATE.md`, `plan/learned/DESIGN-HISTORY.md`.
  → Decision: this is post-release engineering; extend shared publishing rather than introduce another renderer or navigation system.

**Key insights:** the current page is a flat directory; Milestones records past achievements; update reports and milestones already have canonical inputs; demonstrations already exist on the homepage and in the gallery.

## Current Behavior

**Source files read:** `website/project/index.md`; `internal/le/site/docsmanifest.go` `hubPages`; `docs.go` `docsProducerPages`, `docsRenderer.renderSource`; `producer.go` `registerProducer`; `changes.go` `loadChangeWeeks`; `home.go` `whatsNewBand`, `homeHeroSlots`; `datapages.go` `groupMilestones`, `milestonesBody`; `roadmap.go` `renderRoadmap`; `homebody.go` `homeTemplate`; `website/assets/js/site.js` `initTerminalDemoPlayers`, `initProductShowcases`.

The docs producer publishes the project Markdown as one hero and ten list links. Its Milestones description incorrectly calls historical achievements a planned delivery sequence. The homepage already embeds the genuine continuous terminal recording, four interface previews and a separate SSH configuration walkthrough. The gallery documents each recording and its transcript.

**Preserve:** all ten existing destinations, `/project/` and its mirror, shared navigation/footer/canonical metadata, keyboard access, useful no-JavaScript content, light/dark themes, and the existing homepage and demonstration assets.

**Change:** replace the flat list with substantive sections; accurately label historical milestones; derive fresh update and landmark excerpts from their existing inputs; add a visible demonstration path.

## Existing Mechanisms

| Mechanism | Where | Current behavior | This spec |
|-----------|-------|------------------|-----------|
| Producer registry | `internal/le/site/producer.go`, `registerProducer` | Discovers publishers and records route coverage | Extend with project producer; remove project from docs ownership |
| Shared publishing | `internal/le/site/docs.go`, `docsRenderer.renderSource` | Renders Markdown, shell, links and mirror | Reuse after filling project-local slots |
| Authored page | `website/project/index.md` | Owns overview text and destinations | Extend as the authored layout with two generated slots |
| Latest reports | `internal/le/site/changes.go`, `loadChangeWeeks`; `home.go`, `blogTeaserCards` | Orders reports and renders summaries | Reuse loader and summary semantics; do not duplicate report content |
| Historical landmarks | `website/data/milestones.json`; `datapages.go`, `groupMilestones` | Validates dates/categories and orders history | Reuse data and validated ordering for three latest landmarks |
| Release inventory | `internal/le/site/roadmap.go`, `renderRoadmap` | Publishes committed planning snapshot | Link to it; do not create readiness arithmetic |
| Real demonstrations | `home.go`, `homeHeroSlots`; `homebody.go`, `homeTemplate` | Publishes authentic recordings and previews | Link to existing playback; do not create another recording |
| Shared themes | `website/assets/css/30-components.css` | Provides hero, surfaces and typography | Extend with scoped project styles using current tokens |
| Native daemon compilation | `internal/le/doc/yangcontract/command_surfaces.go`, `BuildDaemon` | Shared site/documentation compiler has an independent five-minute deadline | Extend the finite compile budget to fifteen minutes after observed canonical publication failure; preserve caller cancellation and extraction limits |

## Data Flow

### Entry Point and Transformation Path

`./le site build` enters `internal/le/site/build.go` `Build`, discovers the project producer, reads `website/project/index.md`, loads reports with `loadChangeWeeks` and milestones through their existing validated grouping, fills two explicit page-local slots, and passes the resulting Markdown/HTML to `docsRenderer.renderSource`. That renderer publishes `project/index.html` and `project/index.md`. Existing derived publishers consume the mirror for search and machine-readable text.

Search is a plain-text consumer: `pageSearchRecords` renders each mirror's title and body through the shared Markdown engine, then extracts visible HTML text once before serializing the search index. This decodes prose entities once while retaining literal entity spellings in code. Inline adjacency, image alt text and block/table separation remain searchable; script/style content is omitted. `llms-full.txt` copies Markdown bodies unchanged, `llms.txt` reads structured data and emits page links, and page checks verify mirror existence. Those readers retain their existing formats.

Canonical publication also requires the existing live daemon compile. `site/build.go:refreshNativeSurfaces` and `site/yang.go:runYANGConfigTree` reach `docyangcontract.BuildDaemon` through `liveDaemon`; the documentation drift check reaches it through `loadLiveCommandCatalog` and `catalogDaemon`. The shared compile retains a finite fifteen-minute ceiling and the caller's earlier cancellation. Catalog extraction keeps its separate two-minute limit; owned-process termination and join remain unchanged.

Use the newest report's existing summary, coverage date label and canonical report destination. Select the first three milestone entries in validated newest-first order, retaining source order on equal dates; show all available when fewer than three exist. Display their dates and titles with a link to complete history. Escape inserted text and destinations for their output context. Do not derive release readiness or estimated delivery from these inputs.

### Boundaries Crossed

| Boundary | How | Verification |
|----------|-----|--------------|
| Website inputs to publisher | Existing Markdown/report/JSON readers | Selection, escaping and invalid-input tests |
| Publisher to reader | Shared shell plus HTML and Markdown artifacts | Build publication test and canonical site check |
| Overview to demonstration | Ordinary links to homepage terminal and gallery | Published destination/fragment checks |

### Changed-Wire Census

**Search evidence:** scoped searches of `internal/le/site`, `website/project`, `website/assets/js`, `website/data` and `docs/guide/terminal-demonstrations.md` for project routes, producer registration, report loading, milestone fields and terminal player attributes found the existing readers and writers named above. Positive controls were `hubPages` for `/project/`, `renderChanges` for report publication, and `homeHeroSlots` for demonstration consumption.

No daemon, plugin, CLI, configuration or network-protocol wire changes. Existing report and milestone schemas remain unchanged. The affected public documents are HTML and Markdown at their current URLs.

| Wire | Role | Carrier | Source evidence | Disposition |
|------|------|---------|-----------------|-------------|
| Project HTML/Markdown | writer | `docsRenderer.renderSource`, new project producer | Existing `hubPages` row currently reaches shared renderer | Transfer sole ownership; preserve URLs/shell/mirror |
| Project HTML/Markdown | reader | Browser; existing derived search and machine-reader publishers | `producer.go` derived producer pass reads finished artifacts | Preserve normal documents and links; no reader migration |
| Markdown mirrors containing literal tag/entity text | writer | `docs.go` `docsRenderer.renderSource`, `home.go` `homeMirror`, `authored.go` `publishAuthoredPage`, `activity.go` `renderActivityPage` through `mirror.go` `htmlToMarkdown` | Source search for `htmlToMarkdown(` over non-test `internal/le/site` found these four callers | Preserve readable prose and code; encode literal text so a Markdown renderer cannot reinterpret it as markup |
| Markdown mirrors containing literal tag/entity text | reader | Markdown renderer, search and machine-readable indexes, and external Markdown readers | `markdown.go` `renderMarkdown` allows explicit raw HTML, while the mirror converter currently decodes text entities | Keep intended HTML-to-Markdown structure; retain escaped literal text and existing code content |
| Search title/body records | writer / reader | `search.go` `pageSearchRecords` to published JSON and `website/assets/js/site.js` search result rendering | `newSearchRecord` originally has one page caller and two configuration callers; `searchTitleOf` has one page caller | Render page Markdown to reader text before indexing; preserve the structured configuration constructor and legacy stripping contract |
| Markdown bodies | writer | `mirror.go` `writeMarkdownMirror` | Shared publishers write each Markdown body beside its HTML through this sink | Preserve the existing sibling artifact format |
| Markdown bodies | reader | `llmsfull.go` `writeLLMSFullSection` | Reads each mirror and appends its Markdown body unchanged | Preserve escaped Markdown; no plain-text decoding |
| Structured site data and page links | writer | `build.go` `stageSources`, `publishCommandCatalog`, `publishPluginRegistry`, `publishYANGConfigTree`; `facts.go` `publishSiteFacts` | Stages authored navigation/dependency/feature/equivalent data and writes native catalog, registry, configuration and facts artifacts consumed by `loadLLMSInputs` | Existing writers/formats unchanged; identify the unchanged producer side explicitly |
| Structured site data and page links | reader | `derived.go` `renderLLMS` and its input loaders | Reads structured inputs and writes curated Markdown links | Preserve existing metadata/link format |
| Native build phase deadline | writer / reader | `command_surfaces.go` `daemonBuildTimeout` to `BuildDaemon` | `context.WithTimeout(ctx, daemonBuildTimeout)` owns only the compile; `runCatalogProcess` owns process cleanup | Five to fifteen minutes after observed progressive linking exceeded the old ceiling; retain finite bound and inherited cancellation |
| Shared compiled daemon | writer | `command_surfaces.go` `BuildDaemon` | Fixed `go build` invocation writes `dir/ze` and returns that binary path | Preserve actual current-checkout compilation under the corrected finite budget |
| Shared compiled daemon | reader | `site/build.go` `refreshNativeSurfaces`; `site/yang.go` `runYANGConfigTree`; `command_surfaces.go` `loadLiveCommandCatalog` | Site `liveDaemon` and drift-check `catalogDaemon` both point to `BuildDaemon` | All three callers share the corrected compile budget; command/schema outputs and catalog run deadline unchanged |
| Report/milestone source formats | writer | Authored `website/changes/posts/*.md` and `website/data/milestones.json` | Existing files supply the report front matter/body and milestone date/title/category/blurb fields; this change introduces no producer or schema for them | Preserve canonical authored inputs; do not duplicate them in the overview |
| Report/milestone source formats | reader | `loadChangeWeeks`, `groupMilestones`, project producer | Existing loaders own date/category validation and ordering | Reuse unchanged schema; no new data file |

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| Intended layers | Yes, by design | Project composition enters shared renderer and shell |
| Component isolation | Yes | Website-only inputs and publishing; no runtime changes |
| Reuse rather than duplication | Yes | Existing loaders, validated ordering, shell and mirror |
| Zero-copy | N-A | Offline page composition, outside route processing |
| Outbound registration | Yes | New producer registers from its own initialization |
| Inbound registration | Yes | `Build` discovers it; remove old `hubPages` ownership; no addition to `liveDocSources` or core switch |

## Proposed Page and Information Preservation

| Section | Reader outcome and existing destinations |
|---------|------------------------------------------|
| Where Ze stands | Short ExaBGP-to-Ze explanation, explicit pre-release status, Why Ze |
| Latest development | Prominent dated newest-report summary; Changes |
| Before the first release | Smaller neighboring panel explaining correctness, validation and delivery; Roadmap and quality evidence |
| See Ze running | Explain what the continuous recording shows; prominent Watch Ze in action link to `../#product-terminal`, plus `../demos/terminal/#continuous-live-configuration-showcase`; offer the BGP quickstart |
| How we got here | Compact dated three-landmark timeline; Milestones and Project history |
| How Ze is built | Brief AI process explanation; AI development, Blog, Talks and Activity |
| Get involved | Concrete testing, failure-reporting, code and documentation routes through Contribute |

Retain all ten original destinations. Use a blue hero, wide update/release composition, compact timeline and quieter reading links, with responsive stacking. Preserve details rather than convert every destination into an identical card. Demonstration wording must distinguish real recorded playback from a live hosted router; no new embedded player is required.

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | The added request targets the initial homepage viewport as well as the approved project overview | Thomas's exact clarification above | Opening composition must include useful demo before scrolling | Explicit user clarification on 2026-11-11 | confirmed |
| A-2 | Existing gallery/homepage demonstrations meet the action requirement | Live homepage has showcase playback; manifest describes its workflow | Playback destination may need repair | Open both links and play existing recording during artifact review | confirmed: six real homepage playback cases PASS; canonical gallery/fragment and quickstart HTTP 200; gallery artifact contains its real showcase figure/transcript |

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Two publishers overwrite the overview | Duplicate coverage or old list reappears | Remove only project row from `hubPages`; assert sole owner in Build test |
| R-2 | Hand-maintained freshness or readiness claim drifts | Report differs from canonical source | Derive excerpt/date; no copied counts or release forecast |
| R-3 | Text becomes executable markup or breaks links | Hostile title renders tags/attributes | Escape each generated field; exercise punctuation and hostile values |
| R-4 | Missing reports/data publishes an empty overview | Loader error or empty dynamic slot | Propagate existing validation with input path; allow zero landmarks honestly |
| R-5 | Shared homepage work is overwritten | Diff touches `homebody.go` or existing active-spec changes | Keep implementation scoped; coordinate shared CSS/manifest/doc edits |
| R-6 | Layout or link fragments fail on smaller screens | Overflow or missing demonstration destination | Check both themes, keyboard, zoom, no-JavaScript and mobile layouts |
| R-7 | Shared mirror correction alters code or badge text | Round-trip or existing mirror fixtures differ | Test literal text, actual code spans/fences and badge rendering, and run the existing converter and affected publisher tests |
| R-8 | A healthy native link exceeds the build-phase budget on the shared host | Progressive linker I/O followed by deadline cancellation | Extend only the finite compile ceiling to fifteen minutes; preserve earlier caller cancellation and process-group joining; prove actual canonical publication and existing cancellation behavior |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Project explanation, navigation, publication ownership or mirror; no routing behavior |
| How corrected? | A forward source change through the normal commit flow, followed by site regeneration |
| Who else touches this? | Active deployment-user-journeys and homepage-project-metrics sessions; coordinate shared files |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Build` | → | Registry → project producer → shared renderer → artifact | `TestBuildPublishesProjectOverview` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Build and visit `/project/` | All seven sections explain their purpose and supply useful next actions | Existing chrome, URL and navigation remain useful |
| AC-2 | Unordered valid reports, then a newer report | Newest canonical summary, coverage dates and report link appear in HTML and mirror | Older reports remain accessible through Changes |
| AC-3 | Milestones with equal and distinct dates | Up to three newest dated landmarks appear deterministically; full timeline/history links work | All historical entries remain in Milestones; none are called future delivery |
| AC-4 | Follow See Ze running | Homepage terminal and continuous gallery recording are reachable; quickstart offers a trial path | Existing recordings, previews and full transcripts remain authoritative |
| AC-5 | Compare original destination set | All ten original destinations remain readable and reachable | New quality/demo/quickstart links are useful additions |
| AC-6 | HTML/mirror plus hostile text inputs | Same meaningful text and destinations survive; generated text is safely escaped | Shared shell/scripts remain because they provide normal site behavior |
| AC-7 | Small/large viewport, light/dark, keyboard and no JavaScript | Hierarchy, readable content and working links remain available without overflow | Layout may stack; no section is discarded |
| AC-8 | Initial homepage viewport at 1440x900 and 1366x768, in both themes | Product heading, plain product explanation, watch/try actions and usable genuine recording with playback controls are visible before scrolling | Detailed capabilities, metrics and all original destinations remain available farther down as needed |
| AC-9 | Initial homepage viewport at 390x844 | Concise product explanation, visible watch action and recognizable recording preview appear without horizontal overflow; playback remains usable | Detailed capability descriptions and supplementary screenshots follow the preview because a phone cannot display the full desktop composition at once |
| AC-10 | Search indexes a mirror containing literal prose, entities and code | Published title/display title/body match reader-visible Markdown semantics once, including code entity spellings and image labels | Existing configuration search records and machine-readable Markdown exports retain their formats |
| AC-11 | Native compile progresses beyond the former five-minute ceiling, or the caller cancels | Canonical publication can finish within the finite fifteen-minute compile budget; caller cancellation still joins owned work and prevents advancing stages | Catalog extraction retains its separate two-minute ceiling; no daemon runtime or command-output change |

## 🧪 TDD Test Plan

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildPublishesProjectOverview` | `internal/le/site/project_test.go` | Registered publisher runs through Build; expected sections, newest report, all ten destinations, demo links, mirror and single ownership | Passed under race |
| `TestProjectOverviewSelectsNewestReport` | Same | Existing newest-report ordering and date/summary selection | Passed under race |
| `TestProjectOverviewSelectsLandmarks` | Same | Validated date order, equal-date stability and 0/1/2/3/4-entry inputs | Passed under race |
| `TestProjectOverviewEscapesGeneratedContent` | Same | Literal text and safe destinations survive hostile report/milestone input | Passed under race |
| `TestProjectOverviewRejectsUnfilledSlots` | Same | Missing/unknown generated slot cannot publish plausible incomplete content | Passed under race |
| `TestHomeOpeningShowsOverviewAndRecording` | `internal/le/site/browsertest/home_opening_test.go` | Actual published opening geometry at desktop, laptop and phone sizes; real player, overview and useful actions visible | Passed: all six viewport/theme cases and actual playback advancement |
| `TestLiveCommandCatalogBoundExcludesCompile` | `internal/le/doc/yangcontract/command_catalog_daemon_test.go` | Existing compile and extraction phases retain separate budgets | Passed under race, 5.01s |
| `TestCatalogActionCancellation/build` | `internal/le/doc/yangcontract/catalog_cancel_test.go` | Existing caller cancellation joins owned native build processes, preserves the foreign sentinel and starts no next stage | Passed under race, 6.97s |

Main ran the browser test fresh after the homepage changes. Direct inspection
of `job-home-opening-fixed-a773014c.log` confirms desktop 1440x900, laptop
1366x768 and phone 390x844 each passed in light and dark themes, including
clicking Play and observing the genuine recording's elapsed clock advance.
The same test previously passed both desktop cases and failed the laptop
actions and phone preview/Play geometry before these source changes. Scoped
site regressions and measured focused coverage are recorded separately below.
Canonical build/check and artifact evidence are recorded separately below; this
browser result does not establish those separate results.

Test maintenance is separate from the homepage fix. An independent judge
approved building through the native CLI once instead of importing the entire
site builder into the race-enabled browser package. The initialized-header
selector was corrected against `loadSharedHeader`, which replaces its original
mount. Readiness, geometry, genuine-media and playback assertions remain.

Boundary tests cover the landmark limit above; no numeric public input is introduced. A `.ci` or interop test is N-A: this is static publishing with no new daemon command or protocol. Use an existing synthetic site fixture for the Build test, not a checkout export. Visually check the canonical artifact and follow both demo links; source assertions alone cannot prove readability or playback.

## Files to Modify

- `website/project/index.md`: authored overview and two page-local generated slots.
- `internal/le/site/docsmanifest.go`: transfer project route ownership; retain other hub pages.
- `internal/le/site/docs_test.go`: adjust the docs-owned page census from 150 to 149 under the independent test judge's recorded approval; preserve all source and destination assertions.
- `website/assets/css/30-components.css`: narrowly scoped project layout, coordinated with active sessions.
- `website/AI.md`: document project producer, canonical inputs and preservation behavior.
- `internal/le/site/homebody.go`: concise initial overview and purpose-led genuine demonstration caption; retain all useful content.
- `internal/le/site/home_test.go`: check the same literal phrase/date as reader-visible rendered mirror text; preserve shell/layout assertions under independent test judgment.
- `website/assets/css/60-responsive.css`: responsive opening composition with readable demo and actions at supported viewport sizes.
- `website/style-guide/index.html`: record the homepage opening layout and content preservation treatment.
- `website/assets/css/site.css`: include the project stylesheet through the existing CSS import bundler.
- `internal/le/site/mirror.go`: preserve literal escaped text across the shared HTML-to-Markdown conversion if the focused publication regression confirms the issue; preserve preformatted and inline-code contents.
- `internal/le/site/mirror_test.go`: add focused round-trip coverage for literal tag/entity text and code after the project publication regression reproduces the fault.
- `internal/le/site/search.go`: read escaped Markdown as reader-visible title/body text at the page-search boundary; retain configuration metadata handling.
- `internal/le/site/search_test.go`: publish and inspect literal-text search records plus separator, image-label and script/style preservation cases.
- `internal/le/doc/yangcontract/command_surfaces.go`: extend only the finite daemon compile deadline from five to fifteen minutes after the observed canonical build failure; preserve caller cancellation and the catalog run deadline.
- `docs/contributing/gh-pages.md`: document the shared fifteen-minute compile ceiling at its existing `BuildDaemon` source anchor before changing the constant.

## Files to Create

- `internal/le/site/project.go`: registered project composition using existing loaders and shared rendering.
- `internal/le/site/register_project.go`: project producer registration in its own registration file.
- `website/assets/css/35-project.css`: scoped project layout using existing theme tokens.
- `internal/le/site/project_test.go`: wiring, selection, escaping and incomplete-slot proof.
- `internal/le/site/browsertest/home_opening_test.go`: actual browser viewport and playback verification.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | No daemon configuration or RPC |
| YANG validation constraints | N-A | No new YANG input |
| YANG custom validators | N-A | No new YANG input |
| CLI commands/flags | N-A | Existing build entry point |
| CLI grammar | N-A | No command grammar change |
| Editor autocomplete | N-A | No editor surface |
| Functional test for new RPC/API | N-A | Build wiring test covers static publisher |
| Pipe completeness | N-A | No command output change |
| Env var registration | N-A | No environment input |
| Doctor check for runtime dependencies | N-A | No new runtime dependency; existing website inputs/loaders |
| Doctor code per problem class | N-A | Existing build-time errors; no daemon health class |
| Prometheus counters | N-A | No runtime measurements |
| BGP family surface | N-A | No protocol work |

### Documentation Update Checklist

| # | Question | Applies? | File / reason |
|---|----------|----------|---------------|
| 1 | Product feature scope/evidence | No | Website overview does not alter a product feature |
| 2 | Config syntax | No | No config change |
| 3 | CLI command | No | Existing site build |
| 4 | API/RPC | No | No API change |
| 5 | Plugin | No | No plugin change |
| 6 | User guide | No | Existing demonstration/quickstart guides remain valid |
| 7 | Wire format | No | Existing schemas unchanged |
| 8 | SDK/protocol | No | No runtime boundary |
| 9 | RFC behavior | No | No protocol claim or evidence changes |
| 10 | Test infrastructure | No | Reuse site fixture/testing mechanisms |
| 11 | Daemon comparison | No | No daemon behavior change |
| 12 | Internal architecture | Yes | `website/AI.md`: dedicated project publisher/shared renderer and mirror reader boundaries; `docs/contributing/gh-pages.md`: shared daemon compile budget at its existing source anchor |
| 13 | Route metadata | No | No route processing |
| 14 | Metrics | No | No metrics added |
| 15 | Inventory | No | Existing public route and destinations retained |
| 16 | Source anchors/Design declarations | Yes | Changed docsmanifest source is anchored by `docs/contributing/feature-maturity.md` for destination mapping; mapping remains unchanged. New producer declares `website/AI.md`; verify derived citations before implementation completion |
| 17 | Existing examples | No | Project prose contains no CLI/config/API examples; linked examples unchanged |

## Implementation Steps

1. **Wiring:** write the failing Build publication test; register the project producer and transfer sole route ownership, using shared publishing.
2. **Content/data:** compose authored sections and fill newest-report/landmark slots; add selection, escaping and incomplete-slot proof; maintain mirror parity.
3. **Homepage opening and presentation/docs:** add the real-browser opening test and observe its initial failure; compact/reorder the opening overview and genuine recording, preserve detailed capabilities below the initial composition, and verify supported viewport sizes and actual playback. Apply scoped project layout and update website documentation and style guide. Regenerate canonical `../gh-pages` using `./le site build`, then `./le site check`; review both themes/mobile and play the linked recording. Local regeneration authorizes no push.

### Publication dependency diagnosis

1. **Symptom:** canonical site build session 25941 returned `compile ./cmd/ze: signal: killed` with `context deadline exceeded` (preserved transcript excerpt `scratch/canonical-build-deadline.txt`, not a native job log). Separately, main observed its named `le` bootstrap linker PID 1260241 exceed eight minutes while making file-backed read progress. That establishes host storage delay; it is not a timed measurement of this `cmd/ze` compile.
2. **Root cause:** `command_surfaces.go:BuildDaemon` derives a context with the fixed `daemonBuildTimeout = 5 * time.Minute`. The deadline kills healthy work that this loaded host can take longer to link.
3. **Owning layer:** the shared build-time compiler, reached by site publication, site config-tree extraction and the live documentation catalog. This is a necessary publication dependency, with no routing/runtime protocol change.
4. **[workaround]:** reuse a prior daemon or bypass native catalog/schema generation. That would publish an artifact not proved against the current checkout and lose the existing live-surface contract.
5. **[source]:** change only the compile budget to fifteen minutes and document it. Retain `context.WithTimeout` derivation from the caller, the shared owned-process stop/join path and the catalog's two-minute run bound. Prove the real canonical build and rerun the existing phase-separation and build-cancellation tests; do not add a test that merely repeats the constant.

### Critical Review, Deliverables and Security

| Check | Evidence required |
|-------|-------------------|
| Useful overview | Each AC demonstrated with positive content and working links |
| Preservation | Original ten destinations, preserved homepage content and destinations, and canonical mirror |
| Freshness/accuracy | Canonical newest report, historical dates and explicit pre-release wording; no readiness percentage |
| Input/output safety | Context-appropriate escaping and existing input validation |
| Discovery/regression | `ai/INDEX.md` website row → `website/AI.md`; producer registry plus Build test prevent disconnected publication |
| Artifact | Scoped package tests, `./le site build`, `./le site check`, light/dark/mobile/keyboard/no-JavaScript review |

### Failure Routing

| Failure | Route |
|---------|-------|
| Missing sections, links, mirror or duplicate route | Fix project producer/wiring before proceeding |
| Existing input invalid | Follow canonical loader diagnostic; do not fabricate fallback report |
| Scope expands to homepage | Use Thomas's confirmed initial-viewport clarification and coordinate active owners |
| WIP admission refused | Wait for available slot or explicit owner exception; never borrow another claim |

### Deliverables Checklist

| Deliverable | Verification method | Observed evidence |
|-------------|---------------------|-------------------|
| Overview, canonical update and milestones, original destinations and Markdown mirror | `TestBuildPublishesProjectOverview` and the focused project selection/publication tests | Named tests PASS in `job-project-site-remaining-0fd84a7d.log` |
| Readable homepage explanation and genuine playback in the initial viewport | `TestHomeOpeningShowsOverviewAndRecording` in both themes at desktop, laptop and phone sizes | Six cases with actual playback PASS in `job-home-opening-fixed-a773014c.log` |
| Literal-text preservation through shared mirror publication | New project publication regression and converter round-trip tests, with existing mirror/code cases | New round trips and all affected mirror callers PASS; search publication/configuration and llms-full body/membership regressions PASS |
| Canonical local preview | `./le site build`, `./le site check`, then visual and link review at the original URLs | Main observed canonical Build 67127 and Check 42461 exit 0, 1107/1107 routes; twelve image views and real Tab/no-JavaScript/destination checks PASS |
| Bounded native compilation permits actual publication | Actual canonical build plus existing phase-separation and build-cancellation regressions | Fifteen-minute source correction verified; both existing dependency regressions race PASS; actual canonical retry exit 0 |

### Security Review Checklist

| Check | What to verify | Source/test result |
|-------|----------------|--------------------|
| Generated text remains text | Report and landmark strings retain literal markup/entity characters in HTML and reparsed Markdown; code examples remain unchanged | `html.EscapeString` at every generated field; context-aware mirror literal buffers; hostile-input/publication and code round-trip tests PASS |
| Destinations remain valid | Canonical report dates/slugs and original site-relative links reach the intended pages and recordings | Existing report loader validates route/date inputs; HTML/mirror link sets PASS; canonical site Check passes and action/gallery fragments/quickstart HTTP 200 |
| Existing media boundary | Reuse published recordings and screenshots; the overview adds no service, credential, mutation endpoint or hosted-router session | Authored links and existing `homeHeroSlots` only; actual recording playback PASS; no new runtime endpoint or dependency |
| Generic applicable security checks | Input/path handling, resource bounds, privilege and secret exposure | Fixed authored/data paths use existing loaders and shared renderer; milestone output capped at three; no new socket, shell command, credential, cryptography, privilege or runtime concurrency boundary |
| Native compile remains bounded and cancelable | Finite compile budget, inherited earlier caller cancellation, owned-group stop/join and separate catalog bound | Source census confirms unchanged caller/process ownership and two-minute extraction limit; compile/cancellation phase regressions PASS under race |

## Design Insights and Key Design Decisions

| Decision | Alternative | Reason |
|----------|-------------|--------|
| Dedicated registered composition with shared renderer | Pure static Markdown | Static prose cannot keep latest report/landmarks fresh without duplicate maintenance |
| Two explicit page-local slots | Generic source-transform registry or shared-map exception | Existing homepage substitution pattern is enough for one structured overview |
| Link authentic demonstrations | Embed duplicate replay and duplicate homepage | Makes action reachable while retaining existing recording ownership and avoiding media coupling |

Simplicity: one publisher, two generated sections, existing data and renderer. Uniformity: registry and shared Markdown publishing. Performance: offline composition only; no new network calls or check execution during builds.

## Owned journal rows

| Class file | Date | Surface |
|------------|------|---------|

No existing journal row was identified as an owner of this requested overview redesign.

## Known Limitations

The pages describe a pre-release project; they do not assert production readiness. Phone layouts keep detailed capabilities below the recognizable demonstration preview. No approved overview section or useful existing destination is removed.

## Review Preparation

Independent review is required after implementation. Round 1 covers all changed code, data flow, preserved destinations and generated artifacts; later round scopes must be recorded before they run.

| Run | Scope | Reviewer lenses | Verdict |
|-----|-------|-----------------|---------|

## Scoped Verification

| Evidence | Observed result | Scope and limits |
|----------|-----------------|------------------|
| `job-search-final-6b1045c0.log` | Main's final race run passed `TestSearchIndexesVisibleMirrorText` and all four `TestSearchMirrorTextPreservesReaderBoundaries` cases | Publication/JSON reader text, image/link labels, inline punctuation, block/table separators and script/style skipping on the frozen search source |
| `job-project-site-regressions-bdd6ef8e.log` | Five observed PASS results before external interruption | Activity mirror, authored mirror, Zeledon, Labs and token publication PASS; exit 143 is an interrupted run and the following metadata RUN has no verdict |
| `job-project-site-remaining-0fd84a7d.log` | Remaining affected population PASS under race; package 84.761s | Project/Build/ownership, shared mirror, homepage, search/configuration, llms-full bodies/membership and registry checks; unchanged metadata llms.txt paths excluded by source-based independent judgment |
| `job-project-coverage-ee246922.log` | Focused count-mode coverage run PASS in 0.584s | Profile `scratch/project-overview.cover`; function report `job-project-coverage-summary-50d2fcaf.log`; this selection covers 14.3% of all site-package statements and is not a whole-package completeness claim |
| `job-project-compile-bound-8d5bbc65.log` | Existing dependency regressions PASS under race; package 13.124s | `TestCatalogActionCancellation/build` 6.97s and `TestLiveCommandCatalogBoundExcludesCompile` 5.01s; actual canonical retry recorded separately below |
| Canonical native Build, exec 67127 | Main observed exit 0, publication at 22:32 UTC, 1107/1107 routes written by 24 producers | Source digest `2b7e377051db854ec71d099848b424991e5484de828bfd01153b5f9a44766378`; existing stale-feature warnings and two unstamped talk decks disclosed; no new failure |
| Canonical site Check, exec 42461 | Main observed exit 0, same 1107/1107 route coverage | Actual canonical artifact, separate from the synthetic Build fixture |
| Canonical browser review, exec 98279 | Main observed exit 0; twelve WebP views, Tab navigation, no-JavaScript content/fallback and destination/fragment checks PASS | `scratch/browser-review/main.go` reads the existing preview; no Ze imports or rebuild. One-off documented raw admission applied only to this read-only helper after foreign in-progress sources blocked its native launcher |
| Emulated 200%-equivalent reflow, exec 33107 | Main observed exit 0; 720×450 CSS pixels, DPR 2, scrollWidth 720; retained seven sections, ten destinations and Tab navigation | Zoom-only helper mode and `project-200pct-reflow-light.webp` independently inspected; emulated reflow, not a claim that browser-UI Ctrl+Plus was used |
| `job-project-guard-entry-411eb1c6.log` | Entry-point refusal test and all eight cases PASS; package 1.209s | Unknown and duplicate authored slots now enter `renderProject` with valid companion inputs and retain diagnostic/no-route assertions; independent bounded Round 2 CLEAN |

The independent reviewer read the helper and directly viewed `project-desktop-light.webp`, `project-phone-dark.webp`, `project-demo-dark.webp`, `project-end-light.webp`, `home-laptop-dark.webp` and `home-phone-dark.webp` under `scratch/artifact-review/`. The overview hierarchy, responsive stacking, contrast and initial genuine player/control placement are readable with no visual defect found. Main separately inspected six representative images. Published gallery source has the continuous-showcase heading followed by the genuine `terminal-demo` figure; the helper confirms its HTTP destination and fragment. No gallery playback event is claimed from the screenshot helper; actual playback is the separate six-case browser test.

Canonical Build, Check, browser and emulated reflow results are preserved in `scratch/canonical-artifact-results.md` as excerpts of main's tool transcripts, not native job logs. The independent reviewer read those excerpts, the actual helper assertions and the reflow screenshot; all source and artifact conclusions retain those evidence limits.

### Measured coverage and outcome review

| Producing symbol | Measured statement coverage | Outcome evidence and limits |
|------------------|-----------------------------|-----------------------------|
| `projectOverviewContent`, `projectLatestUpdate`, `projectLandmarkList` | 100% each | Complete/missing/duplicate/unknown slots; absent/blank/authored/fallback summaries; valid/invalid and empty/short/over-limit/equal-date landmark histories all exercised |
| `renderProject` | 95.5% | Canonical source/vocabulary/report/landmark/composition/shared-renderer setup failures and successful publication exercised. The sole unexecuted statement propagates an error from shared `renderSource` output I/O; no output-I/O failure was injected |
| `mirrorConverter.data`, `recordLiteral`, `mirrorCodeComponent` | 100% each | Prose versus preformatted paths, code/badge ancestors, explicit breaks, entity-looking literals and literal greater-than round trips exercised; these figures do not assert every converter function is fully covered |
| `searchMirrorText` | 92.5% | Visible text counted 125 times; hidden text omitted twice; script/style skip guards each executed once; image alt and another image attribute exercised; inline adjacency and both block-boundary outcomes exercised. Unexecuted statements propagate renderer errors, handle a non-EOF tokenizer error from a string reader, and skip a start tag while raw script/style mode is active (those contents tokenize as text) |
| `searchTextBoundary` | 100% | Boundary and transparent-inline outcomes exercised by paragraph/list/table/image/link/code cases |

The interrupted broad selection included unchanged `derived_test.go` llms.txt metadata tests whose fixture reaches real-root RFC derivation. Independent source review confirmed that those structured-data/source-summary paths do not consume changed mirrors, homepage content or `hubPages`. The admitted rerun retained every llms-full body reader, explicit membership/wiki assertion and affected publisher/search contract and passed. No test assertion or product behavior was removed. The native job has no observed deadline or source watcher, and the signal sender was not retained; no cause is assigned to exit 143.

### Bounded pre-commit Go style gate

Main's scoped added-line panic search, including the four new Go files, returned no hits (empty output, `rg` exit 1). The independent closure reviewer read the added lines and found no concern in the three remaining bounded checks:

| Question | Source evidence | Result |
|----------|-----------------|--------|
| What bounds each new loop, queue, retry and cache? | `projectOverviewContent`: two fixed slots. `projectLandmarkList`: finite validated groups, capped at three outputs. `recordLiteral`: finite parsed-source ancestor stack. `searchMirrorText`: EOF on a finite in-memory rendered document. Browser: three sizes/two themes, 45s readiness and 15s playback polls within a 180s context. Tests: finite fixture/case populations. No new queue, retry or cache | No concern |
| Do names describe values rather than Go types? | `projectSource`, `projectLatestSlot`, `landmarks`, `summary`, `literal`, `searchMirrorText`, `searchTextBoundary`, viewport/theme names and compile budget describe their purpose or value | No concern |
| Do new lifecycle or paired APIs state both obligations? | No new paired lifecycle API or worker. Browser/server/context acquisition registers cleanup immediately; existing `BuildDaemon` retains inherited context, deferred cancel and shared process-group stop/join. The const-only budget edit creates no lifecycle | No concern |

## Implementation Summary

### What Was Implemented

- `project.go:renderProject` publishes the authored seven-section overview through the existing registry, report loader, validated milestone ordering and shared renderer. `register_project.go:init` owns registration; the old project row alone leaves `hubPages`.
- `homebody.go:homeTemplate` places concise product copy, watch/try actions and pre-release status beside the genuine recording. Detailed capabilities retain their destinations below the existing metrics, release notice and product map; the responsive stylesheet puts the phone preview before its selector.
- `mirror.go:mirrorConverter.data` preserves literal prose through Markdown rendering while code-producing elements retain decoded literal text. `search.go:pageSearchRecords` consumes Markdown semantics before serializing reader-visible search text.
- The shared `docyangcontract.BuildDaemon` compile ceiling increases from five to fifteen minutes after actual canonical publication hit its previous deadline. Caller cancellation, owned-process cleanup and the separate two-minute catalog run ceiling are unchanged; dependency regressions and the real canonical retry pass.

### Bugs Found/Fixed

- Literal tag/entity text could become markup when a published mirror was rendered again. `TestProjectOverviewPreservesLiteralTextInPublishedMirror` and `TestMirrorPreservesLiteralTextThroughMarkdown` cover HTML, reparsed Markdown, prose, code and badges.
- A code literal buffer initially omitted explicit line breaks; leading literal `>` also became a Markdown quote. The converter round-trip cases now retain both visible text forms; `TestMirrorKeepsPreformattedLiteralText` retains fenced indentation and entity-looking text.
- Search previously indexed escaped Markdown entities as literal search/display spellings. `TestSearchIndexesVisibleMirrorText` inspects the published JSON; `TestSearchMirrorTextPreservesReaderBoundaries` covers punctuation, image/link labels, block/table boundaries and hidden script/style text.
- The original opening clipped laptop actions and hid the recognizable phone preview/Play control. The unchanged browser geometry and playback assertions now pass all six viewport/theme cases.
- Canonical publication hit the existing five-minute native compile deadline. The finite shared compile allowance is now fifteen minutes; existing phase/cancellation regressions and the actual canonical retry pass.

### Documentation Updates

- `website/AI.md` documents the registered project publisher and canonical inputs, mirror/code literal handling, downstream search versus machine-reader boundaries, and preserved homepage content. Its source anchors name `renderProject`, `projectOverviewContent`, `projectLatestUpdate`, `projectLandmarkList`, `pageSearchRecords`, `searchMirrorText`, `configSearchRecords`, `writeLLMSFullSection`, `renderLLMS` and the mirror producers.
- `website/style-guide/index.html` describes the compact opening and capability detail placement while retaining the existing project-metrics layout contract.
- `docs/contributing/gh-pages.md`, Command surfaces, documents the shared fifteen-minute compile budget and earlier caller cancellation at its existing `BuildDaemon`, `loadLiveCommandCatalog` and `runCommandCatalog` source anchor.
- Existing anchored claims in `docs/contributing/gh-pages.md` about `homeTemplate`, `homeHeroSlots` and real terminal playback remain accurate. `docs/contributing/feature-maturity.md` anchors `docsDestinationExact`, whose mapping is unchanged.
- `./le doc check all`, bare `./le repo check` and `./le test weakened check` were not run. Thomas's standing scope decision in `ai/META-DECISIONS.md` requires only evidence for the changed components and says closure never waits on a whole-tree check. Source inspection confirms those actions have no selected-path mode: repository checking includes four tree-wide checks, documentation All judges the checkout, and weakening Check audits the entire changed worktree/range. Owned source/doc/style review, scoped tests, canonical site Check and native commit gates over the selected snapshots/hunks supply the applicable evidence; no automated whole-tree PASS is claimed. RFC checking is N-A because no RFC stem, tagged unit or protocol behavior changes.

### Deviations from Plan

- The observed literal-text publication defect required correcting the shared converter and its actual plain-text search consumer. AC-10 and the reader census record that necessary preservation work; no daemon behavior or data schema changed.
- The browser test builds the actual checkout once through the native `./le --name session site build output <temp>` entry point. This keeps the isolated browser package lightweight and all viewport/media/playback assertions intact. That build is outside the browser binary's race instrumentation; separate affected site-package race runs provide race evidence.
- Independent test judgments permitted the ownership census adjustment (150 to 149), direct reading of the same authored fixture bytes, the ready-header selector correction, and visible-text extraction for mirror assertions. None removes an input, destination or behavior assertion.
- Actual canonical publication exposed a necessary shared build-time dependency: a five-minute daemon compile ceiling. The bounded fifteen-minute correction changes no protocol/runtime behavior and retains the existing compile/extraction/cancellation mechanism; no separate WIP item or test weakening is used.
- Coordinated copy from the concurrent homepage session remains in the reviewed artifact: clearer Go metric labels, release-callout line break/0.9rem text and shorter RFC proof copy. Those changes are now committed in the newer HEAD and are not carried by this session's refreshed two-hunk homepage patch. Existing macro keys, destinations and meaning remain. The bottom quickstart example hunk and AI deployment-card documentation remain excluded because their companion data/examples are not committed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Escaping generated HTML was initially treated as sufficient for the published mirror | The HTML tokenizer decodes entities before the mirror reaches an unsafe-enabled Markdown renderer | Real project publication round-trip regression | Preserve prose encoding and code literals at the shared converter |
| approach | A new literal buffer collected only text tokens | Explicit `br` elements also contribute visible code/badge separation | Independent source review and observed failing cases | Record the same break in literal buffers and retain round-trip cases |
| approach | Mirror safety escaping was initially considered independent of search | Search reads Markdown bodies as plain words and displayed encoded spellings | Downstream consumer census and observed search regression | Render Markdown once at the search boundary; keep configuration metadata semantics |
| approach | Browser readiness looked inside the original header mount | `loadSharedHeader` replaces that mount with the actual header | Production source inspection and baseline readiness failure | Select `header.site-header`; retain all readiness and playback requirements |
| approach | The canonical native compile was expected to complete within its existing five-minute ceiling | Canonical publication hit that deadline; a separate named `le` link demonstrated storage-bound progress beyond eight minutes on this host | Exact canonical failure excerpt and `BuildDaemon` source trace | Extend only the finite shared compile budget; retain cancellation and require a fresh real build |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Explain current position, development, release work, history and participation | Done | `project.go:renderProject`; `website/project/index.md` | Seven authored sections; newest report and three validated landmarks; explicit pre-release wording |
| Reach an authentic demonstration from the overview | Done | `website/project/index.md`, See Ze running | Build retains links; canonical player/gallery fragments and quickstart HTTP 200; actual homepage playback passes |
| Show overview and usable software demonstration before scrolling | Done | `homebody.go:homeTemplate`; responsive CSS; `TestHomeOpeningShowsOverviewAndRecording` | Six viewport/theme cases include real elapsed-clock advancement |
| Preserve useful destinations and distinguish activity from readiness | Done | `projectOverviewPublishedContent`; authored overview; owned homepage hunks | All ten original project destinations and capability/product-map destinations remain; no readiness arithmetic |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestBuildPublishesProjectOverview` PASS | Actual registered Build path; seven sections in HTML and reparsed mirror |
| AC-2 | Done | `TestProjectOverviewSelectsNewestReport` PASS; Build publication PASS | Unordered source files, newer report addition, coverage dates and canonical link |
| AC-3 | Done | `TestProjectOverviewSelectsLandmarks` PASS | 0/1/2/3/4 entries, stable equal dates and within-month ordering |
| AC-4 | Done | Build publication and six browser cases PASS; canonical target checks PASS | Homepage/gallery fragments exist; genuine gallery figure/transcript retained; quickstart HTTP 200 |
| AC-5 | Done | `projectOverviewPublishedContent` in Build publication PASS | All ten old destinations plus purposeful new links in HTML and mirror |
| AC-6 | Done | Escaping, literal mirror, converter and search publication regressions PASS | Hostile tag/entity text remains text; code literals and links survive |
| AC-7 | Done | Canonical browser helper PASS; direct independent screenshot inspection; emulated 200%-equivalent reflow PASS | No horizontal overflow; all sections and destinations available without scripts; 720 CSS pixels/DPR 2 reflow retains readable prose and keyboard actions |
| AC-8 | Done | Browser desktop/laptop cases PASS in both themes | Initial H1, explanation, actions, genuine player/control geometry and actual playback |
| AC-9 | Done | Browser phone cases PASS in both themes | Recognizable preview/Play visible; no horizontal overflow; playback advances |
| AC-10 | Done | `TestSearchIndexesVisibleMirrorText`; `TestSearchMirrorTextPreservesReaderBoundaries` PASS | JSON title/display/body preserve reader text and code entity spellings; existing configuration tests PASS |
| AC-11 | Done | `BuildDaemon` const-only diff; both dependency tests race PASS; canonical Build retry exit 0 | Fifteen-minute finite compile ceiling; caller/process ownership and two-minute catalog extraction unchanged |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBuildPublishesProjectOverview` | Done, PASS | `internal/le/site/project_test.go` | Registered Build, sole project ownership and HTML/mirror destinations |
| `TestProjectOverviewSelectsNewestReport` | Done, PASS | Same | Canonical loader and changed newest report |
| `TestProjectOverviewSelectsLandmarks` | Done, PASS | Same | Limits and stable ordering |
| `TestProjectOverviewEscapesGeneratedContent` | Done, PASS | Same | Generated hostile values remain escaped |
| `TestProjectOverviewRejectsUnfilledSlots` | Done, PASS | Same | Missing, duplicate and unknown slots refused |
| `TestHomeOpeningShowsOverviewAndRecording` | Done, PASS | `internal/le/site/browsertest/home_opening_test.go` | Fresh native build and six real browser playback cases |
| `TestLiveCommandCatalogBoundExcludesCompile` | Done, race PASS 5.01s | `internal/le/doc/yangcontract/command_catalog_daemon_test.go` | Distinct compile and catalog run budgets |
| `TestCatalogActionCancellation/build` | Done, race PASS 6.97s | `internal/le/doc/yangcontract/catalog_cancel_test.go` | Caller cancellation joins actual owned build work without advancing stages |

Additional passing tests cover invalid canonical inputs/composition, project mirror round trips, shared converter/code behavior and search reader semantics. Existing four converter-caller regressions, docs ownership, producer coverage, search/configuration contracts and all llms-full body/membership tests were exercised. The interrupted broad run supplied five observed PASS results; its remaining RUN line supplied no verdict. Unchanged llms.txt metadata tests were excluded after independent source review showed that their structured-data/source-summary path cannot reach this diff.

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `website/project/index.md` | Done | Seven sections and two generated slots |
| `internal/le/site/docsmanifest.go` | Done | Only project hub ownership removed |
| `internal/le/site/docs_test.go` | Done | Independently approved docs-owned census; all source/destination checks retained |
| `internal/le/site/homebody.go` | Done | Owned opening/capability hunks only; foreign deployment edits excluded |
| `internal/le/site/home_test.go` | Done | Visible mirror phrase/date assertion maintenance; all shell assertions retained |
| `internal/le/site/mirror.go` | Done | Context-aware literal preservation |
| `internal/le/site/mirror_test.go` | Done | Prose, marker, code, badge and break round trips |
| `internal/le/site/search.go` | Done | Actual mirror reader boundary |
| `internal/le/site/search_test.go` | Done | Published JSON and reader boundary regressions |
| `website/assets/css/30-components.css` | Done | Compact opening and preserved capability detail |
| `website/assets/css/60-responsive.css` | Done | Laptop/phone composition and preview order |
| `website/assets/css/site.css` | Done | Existing import pipeline includes project CSS |
| `website/AI.md` | Done | Owned project/mirror/search/homepage documentation hunks only |
| `website/style-guide/index.html` | Done | Opening and metrics composition documented |
| `internal/le/site/project.go` | Done | Canonical project composition |
| `internal/le/site/register_project.go` | Done | Registry initialization |
| `website/assets/css/35-project.css` | Done | Scoped token-based project layout |
| `internal/le/site/project_test.go` | Done | Build, selection, safety and refusal proof |
| `internal/le/site/browsertest/home_opening_test.go` | Done | Actual initial viewport and playback proof |
| `internal/le/doc/yangcontract/command_surfaces.go` | Done | Sole constant changes five to fifteen minutes; existing dependency regressions and canonical build PASS |
| `docs/contributing/gh-pages.md` | Done | Actual compile ceiling documented before the source edit |

### Audit Summary

The source/test audit covers four task requirements, eleven acceptance criteria, six original tests plus two existing dependency regressions, and twenty-one implementation/documentation paths: 44 audited rows, all Done, with zero Partial or Skipped rows. Additional regression tests cover invalid canonical inputs, mirror literals and search semantics. No implementation item was reduced. Measured focused coverage and uncovered error outcomes are recorded above; the const-only compile-budget correction adds no branch. Dependency and final guard-entry regressions, canonical Build/Check and artifact/reflow checks pass. Formal Round 2 is CLEAN; native recording/check succeeded, and the two-commit landing is main's final step.

Risks and observations: the site remains explicitly pre-release, and inventory/activity counts do not state production readiness. Shared files are carried through reviewed partial patches; unrelated deployment examples/docs and both foreign journal edits remain outside this session's scope. Theme/viewport/keyboard/no-JavaScript and emulated reflow evidence has the stated browser limits. The fifteen-minute compile ceiling remains finite and inherits earlier cancellation. Existing stale supported-feature evidence and two unstamped talk decks were reported by the successful build; no feature refresh or protocol change is claimed. Retained scratch/browser/timeout-baseline artifacts are evidence, not an active service; their removal waits for owner review.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Replace the flat project directory with a useful overview | Build publication | `TestBuildPublishesProjectOverview` PASS checks the seven sections, distinctive newest report/landmarks and all ten original destinations in HTML and rendered mirror |
| Keep recent development and history tied to their authoritative data | Selection and boundary tests | `TestProjectOverviewSelectsNewestReport` and `TestProjectOverviewSelectsLandmarks` PASS; invalid source/composition tests refuse missing or malformed input |
| Let readers see the software in action from the opening homepage | Real browser functional test | `job-home-opening-fixed-a773014c.log`: six viewport/theme cases PASS with actual Play/clock advancement; canonical gallery destination and genuine figure/transcript verified |
| Preserve meaningful text safely across publication and search | Published artifact regressions | `TestProjectOverviewPreservesLiteralTextInPublishedMirror`, converter round trips and `TestSearchIndexesVisibleMirrorText` PASS; old search/configuration and llms-full contracts PASS |
| Publish the actual current-checkout overview and homepage locally | Canonical native build plus phase/cancellation regression | Actual five-minute failure preserved; bounded source correction and dependency regressions verified; canonical Build 67127 and Check 42461 exit 0 with 1107/1107 routes |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | No in-scope implementation was removed or deferred | Not applicable |

## Review Gate

The independent closure context authored no implementation or test changes. Earlier bounded implementation reviews and test judgments are preparation, not formal Review Gate rounds. Formal Round 1 covered the complete owned diff and canonical artifact with three lenses: logic/wiring, safety/edge cases, and completeness/performance/Go style. It found one guard-entry proof gap. Round 2 reviewed only the two added canonical-input cases and their `renderProject` to `projectOverviewContent` context, directly inspected their passing log, and reports **0 BLOCKER, 0 ISSUE — CLEAN**. The exact refreshed two-hunk homepage and two-hunk AI patches were independently read; the final review scope is nineteen whole paths plus those two partial pairs. Spec alignment, every AC, existing destinations and documentation were reviewed alongside the code.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/project-overview-c43b4620-project-overview.md`; native record succeeded, 21 selected paths, verdict clean |
| `./le spec review check` | Exec 10589 exited 0: review_gate OK, 13 code files, clean, hashes match across nineteen whole paths. Both partial pairs are pinned in the record; native commit preparation checks their selected blobs |
| Rounds | 2; final bounded round CLEAN, 0 BLOCKER and 0 ISSUE |
| Reviewer lenses used | Logic/wiring; safety/edge cases; completeness/performance/Go style; spec requirements and canonical visual/accessibility/destination evidence |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Decoded literal tag/entity text became markup after mirror rendering | `mirror.go:mirrorConverter.data` | Context-aware prose escaping plus publication and converter regressions |
| 2 | ISSUE | New code literal buffer omitted explicit line breaks | `mirror.go:mirrorConverter.start` | Record breaks in literal buffers; code/badge break cases |
| 3 | ISSUE | Literal leading greater-than became a Markdown quote | `mirror.go:mirrorLiteralText` | Encode greater-than; exact visible marker round trip |
| 4 | ISSUE | Search exposed encoded mirror spellings | `search.go:pageSearchRecords` | `searchMirrorText` at the actual reader boundary and JSON publication tests |
| 5 | ISSUE | New script/style skip used a terminating compound guard | `search.go:searchMirrorText` | Separate guards preserve both skip behaviors; final source inspection confirms the split |
| 6 | ISSUE | Unknown/duplicate authored slots were rejected only in helper tests, without entry-point refusal proof | `project.go:projectOverviewContent`; `project_test.go:TestProjectOverviewRefusesInvalidCanonicalInputs` | Two canonical-input table cases enter the registered renderer; all eight cases PASS in `job-project-guard-entry-411eb1c6.log`; bounded Round 2 CLEAN |

### Record notes

Native commit preparation identified census-table records with a `caller` role and unmatched reader-only wire labels. The table now names explicit writer/reader pairs for the shared binary, Markdown bodies, structured site inputs and authored report/milestone inputs. This corrects the source-backed closure record; no code, test, scope or review verdict changed, and it does not open another formal round.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/site/project.go` | Yes | `ls -l`: regular file, 5259 bytes; source independently read |
| `internal/le/site/register_project.go` | Yes | `ls -l`: regular file, 177 bytes; registration independently read |
| `internal/le/site/project_test.go` | Yes | `ls -l`: regular file, 20351 bytes after the two guard-entry cases; source independently read |
| `internal/le/site/browsertest/home_opening_test.go` | Yes | `ls -l`: regular file, 8378 bytes; native-build/browser/playback path independently read |
| `website/assets/css/35-project.css` | Yes | `ls -l`: regular file, 3676 bytes; scoped theme/responsive rules independently read |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3, AC-5, AC-6 | Project publication, newest data, preserved links and literal safety | `job-project-site-remaining-0fd84a7d.log`: named project tests and all converter cases PASS; package PASS in 84.761s |
| AC-8, AC-9 | Initial overview and genuine playable recording | `job-home-opening-fixed-a773014c.log`: all six real viewport/theme/playback cases PASS |
| AC-10 | Search consumes reader-visible Markdown once | Same remaining-race log: both new search tests and every existing search/configuration contract PASS |
| AC-2, AC-3, AC-6, AC-10 | Generated composition, literal contexts and search outcomes are measured | Focused count-mode run PASS; project composition/selection and mirror literal helpers 100%; `renderProject` 95.5%, `searchMirrorText` 92.5%, with uncovered error outcomes stated above |
| AC-4, AC-7 | Canonical demo targets and presentation/accessibility | Build/Check and canonical browser helper exit 0; twelve theme/viewport/section views, Tab, no-JavaScript content and HTTP/fragment targets PASS; explicit 200%-equivalent reflow mode PASS |
| AC-11 | Bounded compilation and inherited cancellation | Const-only fifteen-minute source correction independently read; dependency regressions race PASS; actual canonical retry exit 0 |
| AC-6 | Invalid authored templates refuse publication through the entry point | Final guard-entry log: missing, unknown and duplicate authored-source cases PASS, each requiring the intended diagnostic and no claimed routes |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `Build` registry to project/shared renderer/publication | N-A; `internal/le/site/project_test.go:TestBuildPublishesProjectOverview` | Source follows actual registered Build and inspects sole route ownership, HTML and reparsed mirror; race PASS |
| Native site build to real browser/player | N-A; `internal/le/site/browsertest/home_opening_test.go:TestHomeOpeningShowsOverviewAndRecording` | Builds current main checkout once, serves its fresh output, checks six viewports/themes and actual playback |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Thomas explicitly identified the portion of the main page visible before scrolling |
| A-2 | confirmed | Six real homepage playback cases PASS; canonical gallery target/fragment HTTP 200 and actual showcase figure/transcript present; screenshot helper does not claim a separate gallery Play event |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Project composition and canonical inputs | `renderProject`, existing `loadChangeWeeks`, `groupMilestones` and shared `docsRenderer.renderSource` | Source verified against owned `website/AI.md` subsection |
| Mirror/search/machine-reader boundaries | Four `htmlToMarkdown` callers; `pageSearchRecords`, `searchMirrorText`, `writeLLMSFullSection`, `renderLLMS` | Source verified; new and retained affected reader regressions PASS |
| Homepage and style-guide preservation | Owned `homeTemplate` hunks plus component/responsive CSS | Source verified; capabilities, metrics, release notice, product map and real media remain |
| Existing contributor source anchors | Scoped `docs/` source-anchor search found gh-pages home/player claims and feature-maturity destination mapping | Re-read producing symbols and anchored prose; existing claims remain accurate |
| Shared build-time compile budget | `BuildDaemon` uses `context.WithTimeout(ctx, daemonBuildTimeout)`; all three caller paths retain their contexts and process ownership | Changed Command surfaces prose matches fifteen-minute constant; catalog extraction remains two minutes; runtime behavior unchanged |
| Product/runtime documentation categories marked No | No daemon, config, CLI grammar, API/RPC, plugin, protocol, runtime metric or schema change in owned diff | Source verified; no feature/RFC status update applies |
| Documentation/repository/weakening prechecks | `repo/actions.go:runCheck`, `doc/check/actions.go` action table and `test/weakened/actions.go:checkAnswer` expose checkout/range-wide populations without selected-path scope | Not run under Thomas's standing scoped-evidence rule; manual owned review and canonical site Check complete; selected-blob native commit companion/weakening gates remain mandatory |
| Spec deletion citations | Main's tracked full-path search and Markdown plan search each returned exit 1 with no hits | No tracked citer needs repointing; durable Go Design headers name `website/AI.md` or existing architecture docs |

## Checklist

- [x] Pre-Spec: metadata, source evidence, checklists, positive ACs and the confirmed homepage opening scope verified.
- [x] Admission: Thomas's explicit one-off WIP exception recorded; independent root claim retained.
- [x] TDD: meaningful observed publication, mirror, search and viewport failures followed by named passing scoped regressions.
- [x] Goal: all ACs and destinations demonstrated; actual playback, visual, keyboard, no-JavaScript and emulated reflow checks complete.
- [x] Closure review: template filled, independent Round 2 CLEAN and native record/check OK; main owns the final two-commit landing while retaining the claim.
