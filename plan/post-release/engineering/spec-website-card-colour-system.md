# Spec: Website card color system

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Priority | P3 |
| Release reason | Website presentation and authoring consistency, requested by the owner |
| Depends | - |
| Phase | closed after final review and rendered proof |
| Handoff | - |
| Updated | 2026-10-23 |

## Task

Give website cards distinct, readable colors that consistently identify their subjects.
The owner requested one color guideline and shared generation logic across the website.
The recent homepage override made different categories blue and removed useful distinctions.
Document and enforce the existing category system instead of adding a second taxonomy.

## Required Reading

- `ai/doors/system.md` and `ai/doors/le-tooling.md`
  → Constraint: Keep this change in the website producer and authored website sources.
- `docs/contributing/gh-pages.md` and `website/AI.md`
  → Decision: Build the preview in the session scratch directory through the native site producer.
  → Constraint: Preserve all page content, links, interactions, and Markdown mirrors.
- `website/style-guide/index.html`, palette and category sections
  → Decision: Extend the existing design guide with rules for every card family.
  → Constraint: Separate subject colors from success, warning, failure, and evidence states.
- `ai/META-DECISIONS.md`
  → Constraint: Run focused proof only. No unsolicited full verification, broad lint, or repository-wide check.

## Current Behavior

**Sources read:**
- `website/assets/css/00-tokens.css` defines light and dark palette roles.
- `website/assets/css/10-base.css` maps seven category classes to accent roles and allows decorative tone aliases.
- `website/assets/css/30-components.css` overrides homepage categories with blue and includes theme-specific card overrides.
- `internal/le/site/homebody.go` assigns arbitrary tones to Decision and plugin cards.
- `website/data/audience.json` carries categories and an independent tone override for the MCP card.
- `website/style-guide/index.html` permits positional colors in the proof strip and audience cards.
- `internal/le/site/docpage.go` owns the seven-category vocabulary. Weekly topics add the existing neutral `meta` category.
- `internal/le/site/home.go` and `internal/le/site/blog.go` cycle article colors and permit independent tone overrides.
- `internal/le/site/plugins.go` hashes group names into decorative colors. Its filter buckets are a separate classification and must not change.

**Preserve:** Existing categories, content, links, anchors, filters, card order, theme control, and status meanings.
End-to-end tests stays the second homepage evidence card. Self-contained management keeps the approved logo-blue treatment.
Archived presentation decks retain their authored design.

**Change:** Remove conflicting category overrides and arbitrary color assignment from live website cards.
Use a varied, brighter category palette and shared roles for labels, borders, surfaces, links, and focus states.

## Data Flow

### Entry Point

Authored category metadata, card data, and the website's existing producer registrations.

### Transformation Path

1. Read the existing subject category from the authored record or explicit semantic classification.
2. Generate the shared category class through one validated vocabulary.
3. Resolve the class to theme-aware accent roles in the shared stylesheet.
4. Render the same components in the design guide and published pages.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Authored data to generated HTML | Existing website producers | Verified by producer audit |
| Shared CSS to browser rendering | Native stylesheet expansion and linked site assets | Final 20:50 UTC browser computed-style and filter proof |

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Canonical website sources and native producers remain authoritative |
| No unintended runtime coupling | Yes | No daemon or protocol changes |
| No duplicated category system | Yes | Extend the existing pageCategories validation with the existing neutral case |
| Zero-copy preserved | N-A | Static website generation |
| Registration, outbound | Yes | Existing producer registrations remain unchanged |
| Registration, inbound | Yes | Category consumers share pageCategories; no second category registry |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Existing subject categories cover the cards | Feature, topic, audience and milestone data | Need an explicit neutral rule for cross-category cards | Producer inventory; existing meta category covers mixed subjects | Confirmed |
| A-2 | A shared palette can remain readable in both themes | Existing semantic tokens | Labels or links lose contrast | Browser minimum label contrast 5.18:1 light and 6.54:1 dark; automatic dark matches | Confirmed |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A decorative hue makes a false category claim | Tone or position overrides a subject | Classify by the card's primary subject and use neutral when none applies |
| R-2 | Category changes overwrite status meanings | Status cards or badges change meaning | Keep the status vocabulary independent and check its producers |
| R-3 | A dark-theme selector overrides shared category roles | Computed colors differ across page families | Verify actual CSS cascade and theme rendering |
| R-4 | Concurrent changes enter the commit | Diff contains unrelated files or hunks | Review and commit only the owned paths through the native commit workflow |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Card subject cues, text contrast, or website generation |
| How is it corrected? | A scoped source correction and private regeneration, without destructive Git commands |
| Who else touches this path? | Concurrent repository sessions, whose changes remain outside this work |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Native website generation | → | Category metadata and shared card styles | TestCardCategoriesFollowTheirSubjects |
| Unknown category in authored card data | → | Shared category validation | TestCardCategoryRejectsUnknownValues |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A reader opens the design guide | A visible palette and practical assignment rules cover subject cards, neutral cards, and independent status cards |
| AC-2 | The same subject appears on different pages | Its label, accent border and related card details use the same category roles |
| AC-3 | Cards move or reorder | Their color follows their subject, never their position |
| AC-4 | The homepage's four argument cards appear | They have distinct, readable subject treatments instead of the uniform-blue override |
| AC-5 | A reader switches themes | Category labels and links remain readable, and category identity remains recognizable |
| AC-6 | A generator receives a category | It uses the shared vocabulary, with a documented neutral case and a useful error for an invalid category |
| AC-7 | Existing website content is rendered | Content, controls and destinations remain intact, including the second end-to-end evidence card and separate status colors |

## 🧪 TDD Test Plan

| Test | Location | Validates | Status |
|------|----------|-----------|--------|
| TestCardCategoriesFollowTheirSubjects | Website producer tests | Rendered categories follow semantic inputs across card families and survive reorder | PASS |
| TestCardCategoryRejectsUnknownValues | Website producer tests | Invalid input cannot silently become an arbitrary tone | PASS |
| TestRequiredSubjectCardsRejectNeutralCategory | Website producer tests | Required seven-subject inventories reject neutral before omission | PASS, all three producer entry points |
| Rendered palette and card checks | Private browser preview | Contrast, shared computed colors, labels, focus and theme behavior | PASS |

Numeric boundaries, daemon functional tests and protocol interoperability are not applicable to static card presentation.
Do not add tests that only repeat CSS declarations. Generator contract tests must exercise real producers.

## Files to Modify

- `website/assets/css/00-tokens.css`: varied category palette and readable roles.
- `website/assets/css/10-base.css`: shared category/card styles and removal of arbitrary positional color rules.
- `website/assets/css/30-components.css`: remove conflicting overrides and preserve category roles in both themes.
- `website/style-guide/index.html`: canonical card color guideline and live examples.
- `website/AI.md`: author and generator instructions.
- `docs/contributing/gh-pages.md`: shared category production contract.
- `ai/INDEX.md`: a discovery pointer to the card color guide.
- `internal/le/site/docpage.go`: shared vocabulary, optional neutral category and required-subject validation.
- `internal/le/site/changes.go`: shared topic validation, semantic weekly category and removal of neutral control overrides.
- `internal/le/site/home.go`: semantic audience, news and weekly cards.
- `internal/le/site/homebody.go`: four distinct argument subjects; approved Modular label.
- `internal/le/site/blog.go`: authored optional category; no positional tones.
- `internal/le/site/plugins.go`: semantic area categories independent of filter buckets.
- `internal/le/site/datapages.go`: required-subject feature and milestone validation.
- `internal/le/site/blog_test.go` and `internal/le/site/home_test.go`: replace only the authorized color-policy expectations.
- `website/data/audience.json`, `website/data/topics.json` and `website/data/whats-new.json`: canonical subject assignments, with topic names and existing filter capability preserved.
- `website/faq/faq.md` and `website/contribute/contribute.md`: subject or neutral card classes; existing content preserved.
- `website/blog/posts/how-ze-manages-memory.md`: explicit platform category.
- `ai/META-DECISIONS.md`: the owner's additional standing instruction to clean up task-owned temporary files and servers, check space, preserve useful/unrelated work and explain retention.

The contribution route is produced from `website/contribute/contribute.md`.
Unrelated `website/data/repo-facts.json` and concurrent runtime/RFC changes remain outside this scope.

## Files to Create

- `internal/le/site/cardcategories_test.go`: producer-driven semantic stability, invalid-category and required-subject regressions.
- `test/weakened/e0706495.md`: the three explicitly authorized color-policy replacements, with the ordering mistake recorded honestly.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | No configuration change |
| YANG validation constraints | N-A | No configuration change |
| YANG custom validators | N-A | No configuration change |
| CLI commands/flags | N-A | Existing website build interface |
| CLI grammar | N-A | No command change |
| Editor autocomplete | N-A | No editor change |
| Functional test for RPC/API | N-A | Static website only |
| Pipe completeness | N-A | No command output change |
| Env var registration | N-A | No environment variable |
| Doctor dependency check | N-A | No new runtime dependency |
| Prometheus metrics | N-A | No runtime metrics |
| BGP family surface | N-A | No protocol change |

### Documentation Update Checklist

| # | Question | Applies? | File / reason |
|---|----------|----------|---------------|
| 1 | Product feature level changed? | N-A | Website presentation only |
| 2 | Config syntax changed? | N-A | No config change |
| 3 | CLI changed? | N-A | No CLI change |
| 4 | API/RPC changed? | N-A | No API change |
| 5 | Plugin changed? | N-A | No plugin change |
| 6 | User guide? | Yes | Website design guide palette and card assignment rules |
| 7 | Wire format changed? | N-A | No protocol change |
| 8 | Plugin SDK changed? | N-A | No SDK change |
| 9 | Protocol behavior changed? | N-A | No conformance change |
| 10 | Test infrastructure changed? | N-A | Existing test runner |
| 11 | Daemon comparison affected? | N-A | Comparative claims remain unchanged |
| 12 | Internal architecture changed? | Yes | `website/AI.md` and `docs/contributing/gh-pages.md` describe shared category use |
| 13 | Route metadata changed? | N-A | No daemon route metadata |
| 14 | Metrics changed? | N-A | No metrics change |
| 15 | Plugin inventory changed? | N-A | No plugin inventory change |
| 16 | Existing source anchors affected? | Yes | Check anchors for the final owned source list during implementation |
| 17 | Config/CLI examples affected? | N-A | No examples change |

## Implementation Steps

1. Audit the category paths and establish shared validation through real producer tests.
2. Implement semantic assignments and one theme-aware card palette, with matching design-guide examples and author instructions.
3. Build the private preview once, compare content and card destinations, inspect representative pages in both themes, and run focused tests.
4. Complete independent review and native scoped closure. No push or publication.

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Subject identity | A routing card stays routing-colored on every page |
| Reordering | No card color is chosen by index or positional CSS |
| Neutral case | Cross-category content makes no false subject claim |
| Status distinction | Success, warning, failure and evidence colors retain their meanings |
| Preservation | No card text, destination, control or existing page disappears |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Shared category contract | Focused producer tests and source review |
| Visible color guideline | Native rendered design guide and browser inspection |
| Consistent live pages | Computed category colors and screenshots across representative page families |

## Security Review Checklist

| Check | What to verify |
|-------|----------------|
| Category input | Unknown values cannot become arbitrary CSS classes or markup |
| Existing escaping | Titles, labels and links keep their current escaping |

## Owned journal rows

None identified in the selected card-category scope.

## Key Design Decisions

| Decision | Alternative | Rationale |
|----------|-------------|-----------|
| Reuse subject categories | Assign a different color by card position | Subjects need a stable visual identity across pages |
| One shared palette | Separate page palettes | A local override caused the owner's reported inconsistency |
| Keep status colors separate | Use subject colors for all measures | Status colors communicate an independent fact |

The owner's latest feedback fixes the hue assignments: Operate blue, Routing amber,
Services orange, Automate violet, Observe green, Secure magenta, and Platform teal.
Give accents saturated colors and use deeper text on tinted labels for contrast.
Selection controls and cards must read the same category tokens.
The homepage Decision card represents operator evaluation, Runtime represents Platform,
Routing represents Routing, and the plugin card represents Automate. Label the plugin card Modular.
Mixed editorial or cross-topic evidence cards use neutral; they do not rotate colors by position.

Two existing tests assert positional color assignment. That expectation contradicts this request.
Replace only the positional policy assertions in `TestAnIndexCardTakesTheToneAtItsPosition`
and `TestTheTeasersAreTheThreeNewestWeeks`, preserving inventory, content and chronology assertions.
The replacement proof must demonstrate stable semantic colors after reordering.

The incidental exact class literal in `TestTheBlogIndexRendersItsAuthoredInputs`
also changes from `has-media tone-sky` to `has-media cat-meta`, because the
fixture states no subject. Its body, order, media and link assertions remain.

## Known Limitations

Archived presentation decks are separate authored visual documents. This change governs live website cards and their generation.
No website publication is authorized.

## Implementation Summary

### What Was Implemented

- Shared seven-subject palette plus neutral, theme-aware card/label/control roles, semantic article/audience/news/plugin selection and required-subject validation.
- Canonical topic memberships and authored FAQ/contribution categories; approved Modular and Operate labels. Existing management blue and E2E-second order remain.
- Live guide examples, producer documentation, author instructions and discovery pointer; the owner's cleanup instruction is recorded in META.

### Bugs Found/Fixed

- A neutral feature passed validation but could disappear from the seven-subject homepage grouping. `validateSubjectCategory` restores the catalog contract; `TestRequiredSubjectCardsRejectNeutralCategory` fails before the fix and passes through all three publishing entry points.
- The weekly neutral filter/chip kept obsolete local colors. `changesIndexStyle` now keeps only the dashed chip border; the final browser confirms shared neutral text/surface roles and selected-control behavior.

### Documentation Updates

- `docs/contributing/gh-pages.md` anchors `cardCategory`, `validateCategory`, `validateSubjectCategory`, the weekly category method and `pluginGroupCategory`/`pluginBucketOf`.
- `website/AI.md`, the live style guide and `ai/INDEX.md` describe one shared palette. The final guide places performance engineering under Platform and comparison cards under subject classes.
- Focused source-anchor reads and rendered guide checks passed. Repository-wide `doc check verify`, compilation and full verification were not started under the owner's selective-testing rule.

### Deviations from Plan

- The contribution source is `website/contribute/contribute.md`; no nonexistent project/contribute file was created.
- Required feature/milestone inputs retain seven subject categories; optional cards/pages/topics may use neutral. This prevents silent omission by existing catalog consumers.
- Exactly three authorized color-policy assertions changed. All article/media/link/inventory and teaser chronology checks remain; the weakening ledger records that it was written after those edits.
- Native removal validation requires a currently tracked spec. This spec began untracked, so commit A is prepared/executed first, followed immediately by a fresh native commit-B removal script. Both scripts come from exact printed paths; no direct Git mutation substitutes for them.
- The owner's cleanup follow-up adds the META rule in this same scoped change. Temporary comparison inputs were removed after a clean comparison; compact evidence and the useful current preview remain with explicit retention conditions.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Shared validation initially admitted neutral to seven-subject inventories | Existing grouping/filter consumers require a subject | Independent review traced featureCard.validate to homeFeatureOverview | Added validateSubjectCategory and a producer-driven red/green regression |
| approach | Weekly neutral controls retained a local palette workaround | Shared neutral tokens now provide readable roles | Independent CSS/source review | Removed local colors and checked final rendered controls |
| approach | Accepted-policy ledger was written after authorized assertion replacements | Record the approved replacement before editing its assertion | Author handoff and ledger review | Preserve truthful ordering note; do not backdate or toggle tests |
| approach | Guide wording retained page-specific accents and ambiguous performance placement | Subject classes and canonical Platform performance apply | Browser/documentation review | Corrected three sentences in one record pass |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Distinct readable subject colors | Done | CSS token/category roles and live guide | Browser verifies both themes and automatic dark |
| Shared semantic generation | Done | docpage.go, home.go, blog.go, changes.go, plugins.go, datapages.go | No positional/hash hue selection remains |
| Preserve content and functionality | Done | Final preservation report and browser evidence | Differences classified individually; no missing page/card destination |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Guide has eight paired palette examples, subject/neutral rules and independent status wording. | Visible palette and practical assignment rules |
| AC-2 | Done | Browser recorded seven homepage selector/card matches in both themes; all eight guide selector/label pairs match. | Same subject uses shared roles |
| AC-3 | Done | PASS TestCardCategoriesFollowTheirSubjects, TestAnIndexCardKeepsItsCategoryWhenReordered and TestWeeklyCardCategoriesFollowAllTheirTopics. | Category survives reorder |
| AC-4 | Done | homeTemplate emits operate/platform/routing/automate; final browser shows four subject treatments and Modular. | Distinct homepage arguments |
| AC-5 | Done | Browser label contrast minimum 5.18:1 light, 6.54:1 dark; all eight automatic-dark accents equal explicit dark. | Readable explicit and automatic themes |
| AC-6 | Done | PASS TestCardCategoryRejectsUnknownValues; PASS TestRequiredSubjectCardsRejectNeutralCategory through homepage/features/milestones. Optional categories use meta; required subject catalogs retain seven-subject rejection. | One validated category vocabulary |
| AC-7 | Done | Final comparison retains 1009 published pages, 52 feature cards, 91 plugin cards, 13 FAQ answers and five proof cards with E2E second. Browser Services filter selects 9/52 and reset restores 52. | Preserved content, controls and destinations |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestCardCategoriesFollowTheirSubjects | Done | internal/le/site/cardcategories_test.go | PASS, real article/audience producers reordered |
| TestCardCategoryRejectsUnknownValues | Done | internal/le/site/cardcategories_test.go | PASS, audience/topic/article/news invalid inputs refused |
| TestRequiredSubjectCardsRejectNeutralCategory | Done | internal/le/site/cardcategories_test.go | Named red observed; final full-tags/race PASS for homepage/features/milestones |
| Rendered palette and card checks | Done | scratch/card-colour-browser-evidence.json | Final 20:50 UTC artifact, computed styles, contrast and filter behavior |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `ai/INDEX.md` | Done | Present and hash-pinned in the final native review artifact |
| `docs/contributing/gh-pages.md` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/blog.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/blog_test.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/changes.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/datapages.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/docpage.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/home.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/home_test.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/homebody.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/plugins.go` | Done | Present and hash-pinned in the final native review artifact |
| `internal/le/site/cardcategories_test.go` | Done | Present and hash-pinned in the final native review artifact |
| `website/AI.md` | Done | Present and hash-pinned in the final native review artifact |
| `website/assets/css/00-tokens.css` | Done | Present and hash-pinned in the final native review artifact |
| `website/assets/css/10-base.css` | Done | Present and hash-pinned in the final native review artifact |
| `website/assets/css/30-components.css` | Done | Present and hash-pinned in the final native review artifact |
| `website/blog/posts/how-ze-manages-memory.md` | Done | Present and hash-pinned in the final native review artifact |
| `website/contribute/contribute.md` | Done | Present and hash-pinned in the final native review artifact |
| `website/data/audience.json` | Done | Present and hash-pinned in the final native review artifact |
| `website/data/topics.json` | Done | Present and hash-pinned in the final native review artifact |
| `website/data/whats-new.json` | Done | Present and hash-pinned in the final native review artifact |
| `website/faq/faq.md` | Done | Present and hash-pinned in the final native review artifact |
| `website/style-guide/index.html` | Done | Present and hash-pinned in the final native review artifact |
| `test/weakened/e0706495.md` | Done | Present and hash-pinned in the final native review artifact |
| `ai/META-DECISIONS.md` | Done | Present and hash-pinned in the final native review artifact |

### Audit Summary

- **Total items:** 39 (3 requirements, 7 ACs, 4 checks and 25 owned files).
- **Done:** 39.
- **Partial:** 0.
- **Skipped:** 0.
- **Changed:** Required-subject validation, actual contribution path and the explicit cleanup follow-up are documented above.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Distinct readable subject colors | Rendered browser proof | Seven homepage selector/card matches; eight guide pairs; minimum contrast 5.18:1 light and 6.54:1 dark; automatic dark matches |
| One color guideline and shared generation | Producer tests and source review | PASS TestCardCategoriesFollowTheirSubjects; shared cardCategory/validateCategory/validateSubjectCategory; guide paired examples |
| Preserve existing website information and functions | Artifact comparison and interactions | 1009 published pages retained; unchanged card destinations; Services filter 9/52 and reset 52; all FAQ answers and E2E second retained |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | All agreed work is complete. Push/publication is not authorized by this request. Archived decks retain their separate authored design. | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/website-card-colour-system-f4fd81ed-a95a-446e-b1fc-088457c23b03.md` |
| `./le spec review check` | OK: 11 code files, clean hashes; artifact pins all 25 owned non-spec files |
| Rounds | 2; final source review has 0 BLOCKER and 0 ISSUE |
| Reviewer lenses used | Logic/wiring; security/edge cases/Go style; completeness/cascade/preservation |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Neutral required metadata could be accepted then omitted by seven-category consumers | featureCard.validate, groupMilestones, homeFeatureOverview | validateSubjectCategory and TestRequiredSubjectCardsRejectNeutralCategory |
| 2 | ISSUE | Weekly neutral controls bypassed shared palette roles | changesIndexStyle | Removed obsolete color/ring overrides; final browser neutral ink/surfaces verified |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| internal/le/site/cardcategories_test.go | Yes | Source read contains semantic/reorder, unknown-value and required-subject producer tests; native review hash 2a42208cb9985eb9b047837a496cbae3191259dcc9371d84157c8f01b0628c26 |
| test/weakened/e0706495.md | Yes | Three accepted-policy records read: one native weakening row and two prose maintenance records; complete old non-color assertions retained |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Visible palette and practical assignment rules | Guide has eight paired palette examples, subject/neutral rules and independent status wording. |
| AC-2 | Same subject uses shared roles | Browser recorded seven homepage selector/card matches in both themes; all eight guide selector/label pairs match. |
| AC-3 | Category survives reorder | PASS TestCardCategoriesFollowTheirSubjects, TestAnIndexCardKeepsItsCategoryWhenReordered and TestWeeklyCardCategoriesFollowAllTheirTopics. |
| AC-4 | Distinct homepage arguments | homeTemplate emits operate/platform/routing/automate; final browser shows four subject treatments and Modular. |
| AC-5 | Readable explicit and automatic themes | Browser label contrast minimum 5.18:1 light, 6.54:1 dark; all eight automatic-dark accents equal explicit dark. |
| AC-6 | One validated category vocabulary | PASS TestCardCategoryRejectsUnknownValues; PASS TestRequiredSubjectCardsRejectNeutralCategory through homepage/features/milestones. Optional categories use meta; required subject catalogs retain seven-subject rejection. |
| AC-7 | Preserved content, controls and destinations | Final comparison retains 1009 published pages, 52 feature cards, 91 plugin cards, 13 FAQ answers and five proof cards with E2E second. Browser Services filter selects 9/52 and reset restores 52. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Native website generation | N-A: static website producer tests | 24 focused tests PASS in job-website-card-colours-green-59106c77.log, package 12.207s; final native build 22 producers and 1009 published/written pages |
| Unknown category in authored data | N-A: TestCardCategoryRejectsUnknownValues | Named audience/topic/article/news branches PASS and errors propagate to publishing boundary |
| Neutral required subject | N-A: TestRequiredSubjectCardsRejectNeutralCategory | Homepage/features/milestones PASS in card-subjects-green-exception.log, package 115.816s, full tags/race |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Confirmed | Seven canonical subjects plus neutral cover the audited live card families; required-subject inventories retain their strict contract |
| A-2 | Confirmed | Final browser computed roles and contrast in light/dark; eight automatic-dark accents match explicit dark |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| User guide and palette | Eight guide palette examples use shared cat-* and production legend/card styles | Yes |
| Producer contract and author rules | gh-pages anchors actual cardCategory, validateCategory, validateSubjectCategory, weekly and plugin producers; AI/INDEX pointers resolve | Yes |
| Existing source anchors | Focused reads of gh-pages and website/AI anchors for home, blog, data and plugin producers match unchanged contracts | Yes |
| Product/config/CLI/API/SDK/protocol/metrics/inventory/comparison claims | Owned implementation changes only static site rendering/category data; no daemon handler, schema, wire or runtime inventory changed | N-A with source-aware scope evidence |
| Cleanup standing instruction | Parent verified the exact META hunk against the owner's follow-up | Yes |

Final native artifact: 23 October 2026 20:50 UTC; 22 producers, 1009 published/written pages, 194 source files; source digest `3832c2f3d3da579e6ae05aa6f8f54ab06a06e26172968fda186b7d8570732831`.
Compact evidence lives in this session's scratch: `card-colour-preservation-final.md`, `card-colour-dynamic-evidence.json`, `card-colour-source-review.md`, `card-colour-browser-evidence.json`, `card-colours-after.png` and the named test logs.
The baseline ZIP, expanded cache, raw diff/report and helper were removed after classification (10,479,560 bytes), plus the earlier superseded cache (2,099,048 bytes).
No additional whole-tree check or full gate is owed on this agent's initiative under the current META selective-testing decision; no push was requested.

Native commit preparation classified only the removed positional test as a weakening. The two retained tests' policy-maintenance descriptions remain in the same ledger as prose, because native ledger rows must match actual findings and file-scoped rows apply only to retired files. No assertion was changed to satisfy this bookkeeping check; the ledger hash was refreshed in the native review artifact.

