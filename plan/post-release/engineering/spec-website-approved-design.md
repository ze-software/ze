# Spec: website-approved-design

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling, docs |
| Priority | P3 |
| Release reason | Applies the owner-approved website presentation; no runtime or release correctness change. |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-10-23 |

## Task

Apply the approved blue website design to the generated GitHub Pages website,
preserving every existing page, information item and interaction. Thomas approved
the homepage and four representative inner-page mockups in this conversation and
explicitly requested implementation on 23 October 2026. This is the implementation
record of that approved design, not a request to repeat its approval gates.

The approved artifacts are the revision-2 homepage and pages/ gallery under
the conversation's local ze-review directory. The implementation belongs in the
canonical website sources and native producers, never in generated Pages HTML.

## Scope

- Shared blue identity, navigation, typography, layered surfaces, light and dark
  themes, responsive layouts and accessible interaction states.
- Homepage, guides, reference and catalog pages, articles, search, quality,
  project, demo and other ordinary website routes.
- Preserve authored prose, command examples, generated inventories, mirrors,
  route IDs, data attributes, downloads, source links, recordings and controls.
- Repair the sixteen stale RFC citation destinations exposed by the scoped
  website link test. Limit this repair to the canonical citation paths and the
  wording needed to identify existing per-requirement evidence records.
- Repair the FAQ's existing raw-HTML markup so all thirteen authored answers
  render as native disclosures, preserving their words, links and IDs.
- Frozen historical talk documents retain their authored contents, layouts and
  assets byte-for-byte; their website listing uses the shared design. Preserve
  the existing native refresh of generated activity and standalone bundles.
- Build and check a private complete artifact. Publication is not implied by
  this request and no push is authorized.

## Required Reading

- `ai/doors/system.md` and `ai/doors/le-tooling.md`: the native producer registry
  owns publication. The runtime network OS is outside this change.
- `website/AI.md` and `docs/contributing/gh-pages.md`: extend the shared shell,
  assets and existing interaction contracts; HTML and Markdown use the same data.
- `website/style-guide/index.html#sg-design-principles`: preserve the information
  map, give controls useful destinations and distinguish evidence from claims.
- `docs/contributing/ze-go-style.md`: read in full before the first Go edit.

## Current Behavior

Source audit: `assets.go:siteStylesheet` expands the canonical CSS layers;
`renderJS` publishes the existing script. `shell.go:pageShell.render` wraps
ordinary pages, while `nav.go:sharedHeaderHTML` renders data-owned navigation.
`home.go:homeBody`, `docs.go:docsRenderer.renderSource`,
`blog.go:blogArticleBody`, `commands.go:cliReferenceBody` and
`datapages.go:featuresBody` own the representative page families.

Preserve page routes, IDs, full prose and examples, data order, generated facts,
source links, search, theme storage, recordings/transcripts, filter behavior,
command arguments/pipes, table controls, Markdown siblings and publication stamps.
Change presentation and organization to the approved blue design. Extend existing
controls where the approved design adds search or clearer filtering.

## Data Flow

Entry point: `./le site build output <private-directory>`.
Registered producers read repository Markdown, JSON, command metadata and facts;
they render bodies through the shared shell. The assets producer expands CSS and
publishes JavaScript. The build assembles mirrors, search data and route coverage;
`./le site check output <private-directory>` checks the complete artifact.

| Boundary | Contract | Verification |
|----------|----------|--------------|
| Source to producer | Existing authored/generated data remains authoritative | Before/after content and route inventory |
| Producer to shell | Preserve roots, canonical URLs, sidebar and theme bootstrap | Existing shell/producer tests |
| Markup to JavaScript | Existing classes, IDs and data attributes remain stable | Browser checks of filters, search and recordings |
| Artifact to Pages | No edits to generated published files | Native private build/check |

Architectural verification: no runtime coupling, no new command or route registry,
no new dependency and no duplicated theme/navigation/filter implementation. Zero
copy and protocol registration rules are not applicable to static presentation.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Ordinary routes use the shared shell/assets | website/AI.md | Some routes miss the design | Producer and assets audit | confirmed |
| A-2 | Prototype CSS/JS cannot replace production assets directly | Root theme and navigation contracts differ | Controls stop working | Source audit of shell, nav and site.js | confirmed |
| A-3 | Counts and command/feature inventories must remain generated | Producer/data contracts | Published information becomes stale | Source audit of home, commands, datapages | confirmed |
| A-4 | Historical authored decks, slides and assets are frozen; generated activity and bundles retain their native refresh | website/AI.md, Presentation tooling and website update checklist | Authored history changes, or existing dynamic presentation behavior is lost | `refreshTalks`, `TestRefreshTalksPreservesHistoricalSlides`, authored-byte and embedded-activity comparisons | confirmed |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Decorative restructuring drops information | Missing text, IDs, links or mirrors | Compare the complete before/after artifact and investigate every loss |
| R-2 | New controls conflict with existing initializers | Filter errors, duplicate column selectors | One owner for site.js; preserve its existing state contracts |
| R-3 | Long technical content overflows mobile layouts | Body wider than viewport | Test dense catalogs and code at mobile width; retain local table scrolling |
| R-4 | Shared checkout changes unrelated build inputs | Temporary build failures or changed facts | Scoped writes; preserve unrelated work and classify evidence changes |

## Blast Radius

A bad change affects static website readability/navigation, not Ze runtime.
Recovery is a corrective scoped edit through the repository commit workflow;
destructive Git commands remain forbidden. Other sessions own unrelated runtime,
RFC and fact changes. Do not include `website/data/repo-facts.json` in this scope.

## Wiring Test

Existing entry points are already registered; no new stub or duplicate producer
is needed. Verify these tests and source paths before extending them.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Assets producer | → | siteStylesheet/renderJS | TestAnAssetEditReachesTheArtifact |
| Shared page shell | → | pageShell.render | TestShellCarriesEveryChromeElement |
| Homepage producer | → | renderHome/homeBody | TestTheHomepageCarriesTheShellAndItsMirror |
| Docs producer | → | docsRenderer.renderSource | TestDocsProducerRendersAManifestRoute |
| Blog producer | → | blogArticleBody | TestAnArticlePageCarriesItsHeroIllustrationAndContents |
| Feature producer | → | featuresBody | TestFeaturesProducerDerivesCardState |
| CLI producer | → | cliReferenceBody | TestCommandSurfacesPreserveLiteralHelp |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any ordinary site route | Approved logo-blue identity, coherent navigation, typography and surfaces in light/dark themes |
| AC-2 | Homepage | Rich approved presentation; existing information, generated evidence, recordings, useful destinations and explicit own-SSH explanation retained |
| AC-3 | Guide or article | Full source content, examples, images and attribution; clear hero, readable main column and accessible outline/related links |
| AC-4 | Feature or command catalog | Complete live inventory, stable anchors, arguments/pipes; usable search/filter/reset/column controls with complete no-JS content |
| AC-5 | All existing website functions | Navigation, search, themes, copy, demos, transcripts, downloads, source links and specialist controls remain usable |
| AC-6 | Complete generated artifact | All existing routes, content and mirrors retained; authored historical decks/slides/assets byte-identical, with native generated activity/bundle refresh preserved; native build/check pass |
| AC-7 | Desktop and mobile readers | No page-wide overflow; keyboard focus, contrast, disclosure behavior and reduced-motion handling remain accessible |

## TDD Test Plan

Reuse existing tests for established behavior, adding focused tests only for new
interaction contracts or a demonstrated defect. Do not create tests that merely
repeat CSS declarations. Relevant existing tests are listed under Wiring Test,
plus `TestTheFeaturesPageKeepsTheDataFilesOwnOrder`,
`TestTheHeroReplaysTheRecordingTheManifestNames`,
`TestABlogArticleRendersItsAuthoredInputs`,
`TestTheTalksProducerLeavesTheDecksAlone` and shell/navigation tests.

Each phase runs its scoped site-package tests through `./le job run` with the
documented feature tags/race recipe. Final proof is a complete private build and
check, preservation comparison and browser checks across route families at
desktop/mobile widths, both themes and no-JavaScript content. No runtime
component/functional suite is applicable. Run only checks reached by this work,
consistent with the owner's selective-testing rule; no unsolicited full gate.

## Files to Modify

- `website/assets/css/{00-tokens,10-base,30-components,60-responsive}.css` and
  `website/assets/js/site.js`: shared design and existing interaction extensions.
- `internal/le/site/{shell,nav,home,homebody,docs,doctransform,markdown,blog}.go`: shared and editorial
  presentation. Extend existing producer helpers where needed.
- `internal/le/site/{health,rfccompliance}.go`: only inline presentation tokens
  if the shared stylesheet cannot safely supply the approved design.
- `internal/le/site/{commands,datapages}.go`: reference/catalog presentation.
- Owning `internal/le/site/*_test.go` and native fixtures only where a changed
  contract requires it; retain semantic assertions.
- `website/style-guide/index.html`, `website/AI.md` and
  `docs/contributing/gh-pages.md`: current design/producer contracts.
- `rfc/short/{rfc2516,rfc4090,rfc5216,rfc5561,rfc7705,rfc7950,rfc9012}.md`:
  fourteen uniquely relocated specification citations and two retired aggregate
  evidence citations; preserve protocol requirements and audit judgments.
- `website/faq/faq.md` and its focused existing docs-producer test: restore ten
  escaped answers to native disclosures while retaining all thirteen answers.

## Files to Create

- `website/assets/product-routes.png`, `product-dashboard.png` and
  `product-editor.jpg`: approved product screenshots with accurate captions and
  connected asset publication.
- Focused producer tests for new catalog/showcase interaction contracts, in the
  existing owning test files. No new public routes are planned.

### Integration Checklist

| Integration Point | Applies? | Reason |
|-------------------|----------|--------|
| YANG/schema/validators/editor completion | N-A | No runtime configuration changes |
| CLI commands/grammar/pipes | N-A | Render existing metadata only; preserve complete pipe documentation |
| API/RPC/plugin/env/doctor/metrics/BGP | N-A | Static website presentation only |
| Site registry/shell/assets | Yes | Extend existing producers and shared asset pipeline |
| HTML/Markdown/search parity | Yes | Retain the same source data and stable URLs |

### Documentation Update Checklist

| Surface | Applies? | File / reason |
|---------|----------|---------------|
| Website design principles/tokens/components | Yes | website/style-guide/index.html |
| Site generation and interaction contracts | Yes | website/AI.md; docs/contributing/gh-pages.md |
| Source-anchor references | Yes | Inspect with native spec citation anchors; update changed statements |
| Product scope/evidence levels/config/CLI/API/wire/plugin/RFC/metrics | N-A | No product behavior or catalog facts change |
| Existing technical examples | Preserve | No syntax/prose rewrite is authorized by this design request |

The first complete baseline and candidate each contained the same sixteen stale
RFC citation destinations, once each. `TestEveryLinkedPathExistsInTheTree`
reported them from the authored public coverage/remaining prose through
`rfcProseHTML` and `rfcLinkablePath`. The bounded correction belongs in the seven
RFC summaries above: fourteen specs now have unique paths under the current plan
layout, and the two former aggregate discrimination files now have real
per-requirement records. This repairs website navigation without changing the
meaning of a protocol requirement, manufacturing evidence, or weakening the test.

The baseline FAQ renders ten authored answers as escaped raw HTML in three code
blocks, leaving only three native disclosures. The source markup repair restores
those answers' intended function under AC-5. Preservation review must compare the
decoded question/answer words, links and IDs rather than require the accidental
escaped-HTML code blocks to survive as examples.

## Implementation Phases

The pre-change build precedes all edits. Phases 1–3 own disjoint files and may run
concurrently once each verifies its existing wiring. Each has one implementation
agent and appends its handoff to the session's per-spec state file.

1. **Shared design and interaction wiring.** Own CSS layers, site.js and style
   guide. Port the approved palette, layout and responsive behavior to production
   selectors, preserving specialized page styling. Extend current controls, never
   replace production JS with prototype JS. Verify asset wiring, scoped asset
   tests and interaction syntax. Coordinate markup hooks with phases 2/3.
2. **Shared shell, homepage and editorial pages.** Own shell/nav/home/homebody/
   docs/blog producers and their tests; website/AI.md. Apply approved composition
   while preserving full information and recording manifests. Keep own-SSH
   wording explicit. Verify relevant shell/home/docs/blog tests and mirrors.
3. **Reference and feature catalogs.** Own commands/datapages producers and
   their tests; docs/contributing/gh-pages.md. Implement approved catalog
   organization and semantic hooks for phase 1, preserving every generated item,
   field, link and no-JS access. Verify command/feature tests and data order.
4. **Integration and preservation.** After phases 1–3, a fresh agent audits the
   full artifact/source diff, repairs issues within this design, and records the
   route/content proof. Main thread performs browser verification and coordinates
   corrections. Closure follows in a separate independent review context.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Content preservation | No lost route, prose, example, card, command cell, stable ID or source link |
| Dynamic truth | Counts and status labels still come from live data, never mockup snapshots |
| Interaction preservation | Existing initializers/theme storage/header roots remain authoritative |
| Progressive enhancement | No-JS readers retain navigation and complete technical content |
| Visual fidelity | Approved blue hierarchy, rich homepage, clear guides/catalogs/articles on all viewport sizes |
| Frozen documents | Authored historical decks, slides and assets remain byte-identical; generated activity and its standalone-bundle embed retain their existing native producer |
| Shared checkout | Only scoped files staged/committed; facts and unrelated changes excluded |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Canonical implementation | Read scoped diff and producer tests |
| Complete private site | Native build/check, before/after preservation report |
| Visible working preview | Desktop/mobile/browser checks and saved screenshots |

Presentation preservation evidence: `internal/le/site/talks.go::refreshTalks`
passes `BundleOnly: true` to `updateTalk`, leaving authored slides untouched.
It intentionally regenerates the LINX activity with its historical window pinned
to 11 June 2026, then `bundlePresentation` embeds that activity. This behavior is
documented in `website/AI.md` and covered by
`TestRefreshTalksPreservesHistoricalSlides` in `internal/le/site/site_test.go`.
The first complete candidate comparison found all talk files byte-identical
except `talks/linx-2026-06/activity.html` and its `index-inlined.html` embed. The
bundle is byte-identical outside that one embed, and both baseline and candidate
embeds decode exactly to their respective standalone activity documents. These
are existing generated measurements, not edits to the authored presentation.

### Security Review Checklist

Retain HTML escaping, safe relative roots, external-link attributes and existing
search handling. No new service, dependency, remote script, permission or tracking
is introduced. Filters treat queries as text, never executable HTML.

### Failure Routing

Presentation/interaction failures return to their owning phase; preservation
losses block completion and are restored through corrective source edits.
Unrelated shared-checkout work is not overwritten. Three failed fixes of the
same model require the repository's stop-and-diagnose rule.

## Key Design Decisions

Use the existing native publication pipeline rather than introducing a second
site generator. Port the approved design onto established DOM contracts rather
than shipping the smaller prototype scripts. Preserve historical decks rather
than retroactively redesigning archived presentations.

## Known Limitations

Publication is outside this request. No runtime features, claims or release
readiness changes are implied by the design work.

## Implementation Summary

### What Was Implemented

The existing native producers and shared assets now carry the approved blue
identity, light/dark themes, rich homepage showcase, reading layouts and catalog
controls. All thirteen FAQ answers render as native disclosures. Sixteen stale
canonical RFC citation destinations now resolve to their existing replacements.

### Bugs Found/Fixed

- Homepage manifest links remain data-owned after removing prototype fragments;
  TestHomeHeroDerivesFromTheManifest passed on its targeted rerun.
- The reading outline handles the leading HTML comment and flat hero without
  enclosing the hero in the reading grid; TestDocOutlineHasANativeDisclosure and
  the docs/article producer tests pass.
- Existing FAQ raw HTML stopped at ten blank lines; removing them restores ten
  answers. TestTheFAQPublishesEveryAnswerAsADisclosure preserves all thirteen
  answers' words, links, IDs and mirror content.
- Change-log filtering no longer calls querySelectorAll with an absent optional
  selector. The final browser observes Secure25/44, reset44 and Routing41.
- Browser integration corrected inherited hero foregrounds, feature label
  overlap, and decorative blog overflow; final390px pages fit their viewport.

### Documentation Updates

website/AI.md records homeBody/homeTemplate, insertDocTOC/pageHero and
blogArticleBody contracts. docs/contributing/gh-pages.md records
featuresBody/featureCardHTML/featuresMirror and CLI producer/control contracts.
website/style-guide/index.html records the approved palette and retained design
principles. Existing feature-maturity, site-facts and test-health anchors were
read against their unchanged loaders. Whole-tree doc check verify was not run:
the owner's latest selective-testing rule governs; scoped source/anchor review,
the real linked-path test and final native site check are the evidence.

### Deviations from Plan

Existing shell.go, nav.go and docs.go wiring already supplied the shared shell
and was retained without edits. The exposed FAQ markup and sixteen stale RFC
citations were repaired within AC-5/6. Frozen talks mean authored documents:
the existing refreshTalks activity and bundle refresh remains intact.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Prototype recording fragments were initially copied | homeHeroSlots must derive manifest links | TestHomeHeroDerivesFromTheManifest | Removed fixed fragments; targeted rerun passed |
| approach | First reading-grid split treated a leading comment close as the hero end | Only the complete flat pageHero wrapper may be extracted | Focused outline regression and source trace | Anchored extraction and regression pass |
| none | No new closure defect | Independent review found zero BLOCKER and zero ISSUE | Complete scoped source and artifact review | No closure product edit or lesson row |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Apply approved design throughout ordinary website | Done | website/assets/css/00-tokens.css;10-base.css;30-components.css;60-responsive.css | Existing shared asset producer reaches all ordinary routes |
| Preserve information and interactions | Done | home.go:248;blog.go:292;commands.go:63;datapages.go:352;markdown.go:529 | Complete artifact and browser proof below |
| Repair exposed FAQ and citation defects | Done | website/faq/faq.md; seven named rfc/short summaries | Exact FAQ equality and verified replacement destinations |
| Complete private artifact and retain authored history | Done | talks.go:112::refreshTalks; scratch/website-approved | Native build/check green; no publication |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Existing shared assets; browser light/dark and family checks | Blue hierarchy applies through native shell |
| AC-2 | Done | TestTheHomepageShowcaseKeepsItsImagesAndRecording; browser playback | Explicit own SSH server wording, generated facts and useful links retained |
| AC-3 | Done | Docs/article producer tests; exact preserved examples/IDs; browser outline/copy | Hero, full prose, attribution, images and related links |
| AC-4 | Done | 52 exact feature cards;487 exact commands on each of two surfaces; browser filters | Arguments, pipes, columns and native fallback retained |
| AC-5 | Done | browser-verification.md; FAQ regression; linked-path test | Search, themes, copy, recordings, transcript and specialist controls |
| AC-6 | Done | Native22producers1009/1009coverage;1013HTML/1020mirrors | Authored26/28talk files exact; two generated outputs retain exact embed contract |
| AC-7 | Done | 1440px/390px browser checks, native keyboard disclosures, reduced-motion CSS | Static no-script fixtures are not a JS-disabled browser claim |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestAnAssetEditReachesTheArtifact | Done | internal/le/site/assets_build_test.go | Native scoped race job passed |
| Shell/home and manifest/showcase tests | Done | internal/le/site/home_test.go; shell tests | Manifest corrected rerun passed |
| Docs/outline/FAQ producer tests | Done | internal/le/site/docsrender_test.go;markdown_test.go | FAQ final0.06s, manifest0.08s |
| Blog article/index/feed tests | Done | internal/le/site/blog_test.go | Scoped phase and remaining coverage green |
| Feature state/order/full-content tests | Done | internal/le/site/datapages_test.go | Native catalog/no-JS jobs green |
| Literal command help/cells/pipe guide tests | Done | internal/le/site/commands_test.go | Native catalog30tests green |
| Historical slides/activity/bundle tests | Done | internal/le/site/site_test.go | Final remaining job passed named tests |
| TestBuildRendersEveryPublishedRoute | Done | internal/le/site/site_test.go | PASS497.51s; remaining27package501.018s |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Four shared CSS layers and site.js | Done | Existing producer publishes canonical assets |
| home.go/homebody.go/blog.go/doctransform.go/markdown.go | Done | Existing producers preserve manifest and authored content |
| shell.go/nav.go/docs.go | Changed | Existing wiring retained; no edit needed |
| commands.go/datapages.go and owning tests | Done | Full inventories, disclosures and controls |
| health.go/rfccompliance.go | Done | Presentation tokens only |
| website/AI.md; style guide; gh-pages.md | Done | Current design and producer contracts |
| Three product image assets | Done | Published and reached by showcase |
| FAQ source/test; seven RFC summaries | Done | Bounded preservation/function repairs |

### Audit Summary

- Total items:27 across the four audit tables.
- Done:26.
- Partial:0.
- Skipped:0.
- Changed:1; existing shell/navigation/docs wiring retained without edits.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Approved blue whole-site presentation | Native artifact and browser | website-approved digest95e1c2acacbdbd42c489c145a070ae49e1c1a59e77dcd38518e2f04557eac84f; final homepage/guide screenshots and family checks |
| Preserve every existing information item | Structural/content comparison | preservation-classification-approved.json:1013/1013HTML,1020mirrors,52cards,974command rows,zero lost IDs or unexplained links |
| Preserve functionality | Functional browser and producer tests | Playback reached00:04/00:44; search4results; filters/columns/hash/copy/FAQ keyboard and thirteen exact answers |
| Preserve authored historical decks | Byte comparison and native test | 26/28files exact; two generated activity/bundle files differ only in the existing activity embed; TestRefreshTalksPreservesHistoricalSlides passes |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None in scope | All seven acceptance criteria are demonstrated; push/publication is not authorized by this request | Not applicable |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | tmp/review/website-approved-design-f4fd81ed-a95a-446e-b1fc-088457c23b03.md; all33file hashes independently rechecked unchanged |
| ./le spec review check | OK:15code files,clean,hashes match; additional explicit33file hash comparison found no change |
| Rounds | 1 |
| Reviewer lenses used | Logic/wiring; information/function preservation; security/edge cases/Go style |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | none | Final independent review:0 BLOCKER,0 ISSUE | scratch/closure-review.md over all33owned paths | No closure fix required |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| website/assets/product-routes.png | Yes | ls337554bytes |
| website/assets/product-dashboard.png | Yes | ls26792bytes |
| website/assets/product-editor.jpg | Yes | ls63107bytes |
| Existing site tests | Yes | Source inspection locates FAQ245,outline234,showcase622,command-help423 and feature-state921 |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Shared design/theme | Assets producer and complete1013HTML comparison; parent final dark/light check |
| AC-2 | Home information/showcase | home.go::homeBody/homeFeatureOverview; showcase regression and real browser playback |
| AC-3 | Complete reading content | insertDocTOC/docReadingLayout and blogArticleBody; only missing pre blocks are three escaped FAQ markup blocks |
| AC-4 | Complete catalogs and controls | Final structural report974/974rows,52/52cards exact; browser query/reset/columns/hash |
| AC-5 | Existing functions and FAQ | Final FAQ13decoded answers exact; zero unexplained link losses after17verified linked occurrences |
| AC-6 | Routes/mirrors/history/build | Native check22producers1009/1009; no missing route/mirror; all512roadmap items preserved except revision |
| AC-7 | Responsive accessible behavior | Final390pxFAQ/features/changes and1440pxguide; keyboardFAQ/outline/search; reduced-motion styles read |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Site assets producer | Not applicable to static build; assets_build_test.go | Existing registration reaches siteStylesheet/renderJS; artifact edit test passed |
| Shared shell | Not applicable; shell tests | pageShell.render and navigation contract remain authoritative |
| Homepage producer | Not applicable; home_test.go | renderHome reaches homeBody and showcase; manifest links remain generated |
| Docs producer | Not applicable; docsrender_test.go | renderSource reaches insertDocTOC; final FAQ13native answers |
| Blog producer | Not applicable; blog_test.go | blogArticleBody retains full authored body and builds contents/related links |
| Features producer | Not applicable; datapages_test.go | featuresBody reaches every featureCardHTML; exact52cards |
| CLI producer | Not applicable; commands_test.go | cliReferenceBody reaches group/facts/rows; exact487commands per surface |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Existing pageShell/assets registration and complete ordinary-route native build |
| A-2 | confirmed | Existing site.js initializer/root/theme contracts retained and exercised |
| A-3 | confirmed | loadFeatureData/live command catalog feed exact52cards and487commands |
| A-4 | confirmed | refreshTalks BundleOnly preserves authored slides;26exact files plus exact activity embeds |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Shared design principles/tokens | website/style-guide/index.html and CSS layers | Yes; principles retained and current palette documented |
| Home/read layout and manifest | website/AI.md anchors homeBody/homeTemplate/insertDocTOC/pageHero/blogArticleBody | Yes; producing functions read |
| Catalog contracts | gh-pages.md anchors featuresBody/featureCardHTML/featuresMirror/cliReferenceBody/writeCommandGroup/writeCommandFacts | Yes; exact content proofs agree |
| Existing anchored loader/health claims | docs/contributing/feature-maturity.md;docs/architecture/site-facts.md;docs/architecture/testing/test-health.md | Yes; respective loader/render behavior unchanged |
| Runtime config/CLI/API/wire/plugin/RFC support | Scoped diff has only static site code/assets/prose/citation destinations; no runtime changes or RFC test/tag changes | No update applicable; existing proof freshness warnings preserved |
| Selective checks | Native site build/check; linked-path race test; source-anchor review | Complete; whole-tree repo/doc/RFC/compile/full verification not started under latest META |

Surviving observations: shared source inputs can change after a build (R-4), so
this proof names one completed artifact and its digest. Static fallback fixtures
demonstrate retained content but are not browser JavaScript-disabled execution.
Existing RFC evidence freshness warnings remain truthful. No publish or push
authority is implied. The complete scoped source review found no remaining
information-loss, interaction or responsive defect under R-1/R-2/R-3.
