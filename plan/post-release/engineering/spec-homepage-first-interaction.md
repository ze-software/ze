# Spec: Homepage first interaction

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Owner-requested website ergonomics; no daemon or release behavior changes. |
| RFC gaps | - |
| Depends | - |
| Phase | Approved presentation changes |
| Handoff | - |
| Updated | 2026-10-28 |

## Task

Improve the existing homepage after the owner's approval of the Lust website comparison. Make the BGP quickstart the primary action, shorten the opening explanation without losing information, and show meaningful recorded terminal content before playback. Keep the blue design, all useful destinations, the full replay, transcripts, and pre-release status. This is a presentation change, not a new browser sandbox or installation workflow.

## Required Reading

- [x] `website/style-guide/index.html`, Design principles.
  → Constraint: Preserve useful information and verify equivalent desktop/mobile content, both themes, keyboard access and no-JavaScript behavior.
- [x] `website/AI.md`, homepage and asset contracts.
  → Decision: Change canonical Go template and CSS/JS sources; publish only through the native site builder.
- [x] `docs/contributing/gh-pages.md`, recording and artifact ownership.
  → Constraint: Reuse genuine recordings; do not re-record, publish video or change provenance. The owner's later instruction authorizes committing and pushing repository changes.
- [x] `ai/doors/system.md` and `ai/doors/le-tooling.md`.
  → Constraint: Keep the existing registered homepage producer and shared media renderer.

## Current Behavior

Source files read: `internal/le/site/homebody.go` (`homeTemplate`), `internal/le/site/home.go` (`homeHeroSlots`), and `website/assets/js/site.js` (`initTerminalDemoPlayers`). The template puts several capability paragraphs above two actions, with demos first and quickstart second. The default terminal player has no poster; chapter players already use a recorded-time poster. The browser inspection showed a play symbol before any terminal content. Product previews already have accessible buttons, genuine screenshots, captions and transcript links.

Preserve all homepage sections, generated figures, links, release notice, selected recording identity, full replay, existing management chapter start, keyboard controls, search and navigation. Change only opening hierarchy and presentation. All touched source files were clean at initial inspection.

## Data Flow

The registered homepage producer fills `homeTemplate` from existing demo and content catalogs. The site builder publishes HTML, its derived Markdown mirror and bundled CSS/JS into `../gh-pages`. The browser mounts the existing asciinema player from recording assets. Keep this path; introduce no server, new registry, dependency or second content source.

| Architectural check | Answer |
|---|---|
| Intended layers | Existing homepage producer, shared shell, source assets and player remain authoritative. |
| Coupling | No daemon or protocol changes. |
| Duplication | Reuse existing media and player poster support. |
| Zero-copy | N-A: static page presentation. |
| Registration, outbound and inbound | No new command, route, feature or handler enumeration. |

## Information Preservation Map

| Existing information | Proposed location |
|---|---|
| Network OS identity and immutable services | Opening headline, with compact typography. |
| Plugin extensibility, appliance option, ExaBGP compatibility | Existing introductory chips and concise opening copy. |
| BGP, ISIS, OSPF, interfaces, IPsec, WireGuard and FIB | Compact capability grouping immediately below the opening/demo row; retain individual destinations. |
| YANG, built-in SSH, web, REST, gRPC, gNMI, CLI and MCP | Matching capability grouping, preserving every interface and link. |
| Configuration/protocol engine, daemon and immutable appliance | Concise introduction or capability grouping with existing links. |
| Demo gallery and quickstart | Both remain; quickstart first and visually primary. |
| Product previews, capture provenance and transcripts | Existing showcase, with useful task captions and authentic first frame. |
| Why Ze, web demo, search, Discord and release notice | Existing supporting row, readable below the main decision. |
| Product map and all following sections | Retained. |

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|---|---|---|---|---|---|
| A-1 | Existing recording contains a meaningful preview frame. | The continuous demo and transcript are already published. | Select another genuine frame from that recording, not fabricated terminal text. | The 43-second poster shows AS 65000 and three named peers; Chromium screenshot and recording inspection agree. | Validated |
| A-2 | Existing player supports a poster independent of playback start. | Chapter mount already uses poster and startAt separately. | Use its documented supported interface. | Chromium observed hero playback at 00:00 and chapter playback at 00:36. A recorded-poster reset defect needed the source repair below. | Validated with producer repair |

| ID | Risk | Early signal | Mitigation |
|---|---|---|---|
| R-1 | Moving copy drops a capability or destination. | Inventory mismatch. | Compare preserved links and meaning against the initial template. |
| R-2 | Preview changes playback start or management chapter behavior. | Opening playback skips the introduction. | Keep preview and start separate; exercise both players. |
| R-3 | Narrow screens overflow or actions become inaccessible. | Browser viewport overflow or clipped controls. | Check 390px and 320px layouts and keyboard operation. |
| R-4 | Global CSS changes affect inner pages. | Shared component appearance changes. | Scope layout styles to homepage; check representative existing page. |
| R-5 | Another session changes shared inputs. | Changed content during implementation or commit. | Preserve foreign work, integrate only safe hunks, never overwrite. |

## Blast Radius

| Question | Answer |
|---|---|
| What can break? | Homepage readability, demonstration playback, responsive layout. |
| Recovery | Correct canonical template/assets and regenerate; no operator state migration. |
| Shared work | The terminal-demo showcase spec owns recording work; do not change its tapes, assets, provenance or spec. |

## Wiring Test

| Entry Point | Feature Code | Test |
|---|---|---|
| Generated homepage in Chromium | `homeTemplate`, `homeHeroSlots`, `initTerminalDemoPlayers` | HomepageFirstInteractionBrowserSmoke |
| Existing chapter recording | `initTerminalDemoPlayers` | ManagementChapterBrowserSmoke |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-1 | Open homepage | BGP quickstart is the first primary action; demo gallery remains reachable. |
| AC-2 | Read opening screen | Concise value explanation precedes actions; detailed capabilities are grouped nearby with no loss of meaning or links. |
| AC-3 | Before clicking Play | Genuine terminal content is visible; no autoplay; full replay and transcript remain available. |
| AC-4 | Start replay or management chapter | Full hero replay begins at its original start; management chapter preserves its selected start. |
| AC-5 | Switch product previews with keyboard | Every panel and caption remains available without clipping or layout jumps. |
| AC-6 | Desktop, mobile, light/dark and no JavaScript | Main content and destinations remain readable and reachable; no page-level horizontal overflow. |
| AC-7 | Complete approved presentation changes | Native site build and site check run against canonical `../gh-pages`. The owner's subsequent all-repository commit and push instruction is tracked separately from this presentation acceptance criterion. |

## Test Plan

Browser smoke is the primary proof for this presentation change, including before/after screenshots, primary-action navigation, poster, replay, chapter start, tabs, keyboard, 320/390px mobile, desktop, light/dark and no JavaScript. Run affected existing homepage/media contract tests once after edits through the registered runner. Do not add tests pinning prose, CSS text or incidental defaults. No protocol, kernel, performance or full-tree test run applies.

## Files to Modify

- `internal/le/site/homebody.go`: copy hierarchy, actions, capability grouping and captions.
- `website/assets/css/30-components.css` and `website/assets/css/60-responsive.css`: minimal homepage-specific layout changes where required.
- `internal/le/site/home.go` and existing demo renderer only if needed: connect selected authentic preview through existing media contract.
- `website/assets/js/site.js`: preserve playback behavior while showing the selected poster.
- `website/AI.md`: update homepage presentation contract with implementation.
- `docs/contributing/gh-pages.md`: update preview contract only if changed by implementation.

## Files to Create

`website/tools/asciinema-player-poster.patch` preserves the small upstream player repair reproducibly. Existing vendor files carry the generated bundle, attribution and generation instructions. Session-private browser evidence is retained for review.

### Integration Checklist

| Integration Point | Applies? | Reason |
|---|---|---|
| YANG schema | N-A | Static website only. |
| YANG constraints | N-A | No configuration change. |
| Custom validators | N-A | No configuration change. |
| CLI commands/flags | N-A | No new commands. |
| CLI grammar | N-A | No command change. |
| Editor autocomplete | N-A | No editor change. |
| RPC/API functional test | N-A | No RPC/API change. |
| Pipe completeness | N-A | No command output change. |
| Environment registration | N-A | No new environment variable. |
| Doctor runtime dependency | N-A | Existing assets and player only. |
| Metrics | N-A | No telemetry change. |
| BGP family surface | N-A | No protocol change. |

### Documentation Update Checklist

| # | Surface | Applies? | File / reason |
|---|---|---|---|
| 1 | Feature declaration | No | No daemon capability added. |
| 2 | Config syntax | No | Unchanged. |
| 3 | CLI commands | No | Unchanged. |
| 4 | API/RPC | No | Unchanged. |
| 5 | Plugins | No | Unchanged. |
| 6 | User guide | Yes | `website/AI.md` homepage presentation contract. |
| 7 | Wire format | No | Unchanged. |
| 8 | SDK/protocol | No | Unchanged. |
| 9 | RFC behavior | No | Unchanged. |
| 10 | Test infrastructure | No | Existing browser and site actions. |
| 11 | Daemon comparison | No | Unchanged. |
| 12 | Internal architecture | No | Existing producer/player path. |
| 13 | Route metadata | No | Unchanged. |
| 14 | Metrics | No | Unchanged. |
| 15 | Inventory | No | No registration change. |
| 16 | Source anchors | Yes | `website/AI.md` and, if preview contract changes, `docs/contributing/gh-pages.md`. |
| 17 | Examples | No | No command/config example changes. |

## Discovery and Verification

Existing discovery starts at `ai/INDEX.md`, Edit the website row. `website/AI.md` remains the implementation contract. The style guide governs preservation and accessibility. Existing manifest assets prevent invented preview content; native build/check and browser observations prove publication and interaction. No new discovery mechanism is needed.

## Implementation Steps

1. Wire a genuine preview into the existing homepage player, preserving replay and chapter start behavior.
2. Tighten the hero, prioritize quickstart and group existing capability text with all links retained. Independent implementation slices own disjoint files.
3. Build canonical output, run focused existing contracts and browser smoke, correct in-scope findings.
4. Independent review, source commit and spec closure through the native commit route. Commit remaining repository changes and push as subsequently ordered by the owner.

## Deliverables Checklist

| Deliverable | Verification |
|---|---|
| Clearer homepage with all information retained | HomepageFirstInteractionBrowserSmoke and before/after link inventory. |
| Authentic idle preview with unchanged playback | HomepageFirstInteractionBrowserSmoke and ManagementChapterBrowserSmoke. |
| Responsive and accessible layout | Desktop/mobile screenshots, theme switch, keyboard panels and no-JavaScript navigation. |
| Canonical generated site | Native site build and site check against `../gh-pages`. |
| Accurate presentation contract | Review `website/AI.md` and affected source anchors against implementation. |

## Security Review Checklist

| Surface | Verification |
|---|---|
| Media URLs | Preserve manifest validation and escaping; no new external asset origin. |
| Playback and navigation | No autoplay or hidden active controls; original user-triggered actions remain. |
| Page content | No inline handlers, injected HTML from user input or new runtime service. |
| Shared checkout | Preserve unrelated edits and recordings. The owner's later instruction includes committing all pending repository changes, separated by logical purpose. |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/homepage-first-interaction-644a4e78-0122-4eeb-8642-eb50638a1641.md` |
| `./le spec review check` | CLEAN; hashes match the nine selected template, asset and contract files |
| Rounds | 4; fourth pass checked only the distributed modification notice |
| Reviewer lenses used | Independent information-preservation/accessibility/provenance review and player lifecycle/source review |
| Final verdict | CLEAN |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Initial desktop copy was vertically centered beside a tall showcase, and the genuine 137-column poster was too small. | Homepage layout | Top-aligned hero, smaller headline and explicitly captioned idle-only enlargement; desktop/mobile browser proof. |
| 2 | BLOCKER | Native recorded-poster clearing reset terminal cells but retained the event cursor, leaving forward seek and frame step with partial output. | Player recording driver `clearPoster` | Reset only text posters. Chromium failed before and retained the full AS 65000 dashboard afterward; generated canonical asset also passed. |
| 3 | ISSUE | Patched distributed JavaScript lacked the Apache-2.0 modification notice. | Vendored bundle | Retained top-level `@license Modified by Ze` notice in source patch and generated bundle; independent narrow re-review clean. |

## Owned Journal Rows

None. Existing recording-generation and unrelated historical gate rows are outside this presentation change.

## Implementation Summary

The homepage now places the BGP quickstart first, shortens its opening copy and keeps all existing capability links in two adjacent groups. Task-oriented preview labels retain every panel. A genuine 43-second dashboard frame is visible without autoplay; idle-only enlargement ends on playback, seeking or frame stepping and never applies in fullscreen.

The existing player API separated poster time from playback start, but its recorded-poster cleanup destroyed reconstructed terminal state. The maintained one-condition repair fixes that producer, not an event-loop workaround. The upstream 3.17.0 readable distribution is patched and minified with its pinned Terser version; the README records byte-identical pristine reproduction, hashes and the retained modification notice. No recording, embedded WebAssembly, runtime dependency or protocol implementation changed.

Documentation changed with its producers: `website/AI.md`, `docs/contributing/gh-pages.md` and the vendor README. No whole-tree documentation gate was run; independent source-anchor review and the native site check are the scoped evidence.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Native recorded-time posters were expected to remain correct under all controls. | `clearPoster` sent RIS while preserving the recording cursor; incremental navigation could not restore unchanged cells. | Real keyboard seek and frame-step smoke showed only changing counters. | Repair the underlying driver and retain a reproducible patch; prove failing-before/passing-after in Chromium. |
| approach | The first layout retained centered alignment and unscaled terminal text. | The opening copy left unused space and the recorded text remained hard to read. | Desktop screenshot. | Top-align copy, tighten typography and enlarge only the clearly labeled idle preview. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Primary quickstart action | Done | `internal/le/site/homebody.go`, `homeTemplate` | Real navigation reached `/guides/quickstart/`. |
| Concise opening without information loss | Done | `homeTemplate`, capability groups | Before/after inventory found no missing links or section headings. |
| Meaningful genuine idle terminal | Done | `data-cast-poster`, `initTerminalDemoPlayers`, vendor repair | AS 65000 dashboard before Play; timer remains 00:00. |
| Preserve design, panels, replay, transcript and release notice | Done | Homepage template and scoped styles | Keyboard, themes, mobile, no-JavaScript and replay smoke passed. |
| Canonical publication | Done | Native site producer and `../gh-pages` | Build and check reported 23 producers and 1,091 published/written pages. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | HomepageFirstInteractionBrowserSmoke | First action navigated to Quick Start at 320px; no-JavaScript link also worked. |
| AC-2 | Done | Saved link/heading inventory and screenshots | No original destination or section heading missing. |
| AC-3 | Done | Idle dashboard screenshot and timer | Genuine frame, no autoplay, transcript retained. |
| AC-4 | Done | Replay, chapter and vendor regression smoke | Hero 00:00; chapter 00:36; seek 00:48 and step 00:43 retain complete dashboard. |
| AC-5 | Done | Keyboard preview smoke | All five buttons select the intended panel; inactive panels inert/hidden, stable heights 611.078125px desktop and 449.75px at 320px. |
| AC-6 | Done | Desktop/mobile themes and no-JavaScript smoke | Page scroll width equals viewport at 1,440, 390 and 320px; every no-JavaScript panel and transcript remains available. |
| AC-7 | Done | Native full/partial build and final retained-binary site check | Final published player hash equals source hash. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| HomepageFirstInteractionBrowserSmoke | Done | Session scratch `homepage-browser-proof.json` | Actual generated page, actions, panels, screenshots, keyboard, themes and mobile. |
| ManagementChapterBrowserSmoke | Done | Same browser evidence | Preserved configured 00:36 start and preceding terminal state. |
| Existing homepage/media contracts | Done | `job-homepage-contracts-046a2113.log` | Selected `internal/le/site` tests passed with race and all feature tags. |
| Native publication checks | Done | `./le --name homepage-ergonomics site build partial`; `bin/le-homepage-ergonomics/le site check` | Final check used the successful isolated build after a concurrent transaction edit prevented launcher recompilation. No unrelated source was changed to unblock it. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/le/site/homebody.go` | Done | Canonical copy, grouping, actions and captions. |
| `website/assets/css/30-components.css`, `60-responsive.css` | Done | Homepage layout, focus, responsive controls and idle preview. |
| `internal/le/site/home.go` and demo renderer, if needed | Changed | No edit needed: existing recording producer and mount contract already carry the required data. |
| `website/assets/js/site.js` | Done | Independent preview selection and permanent enlargement removal after interaction. |
| `website/AI.md` | Done | Presentation and provenance contract. |
| `docs/contributing/gh-pages.md` | Done | Preview, chapter and vendor repair contract. |
| Vendor bundle/README and `website/tools/asciinema-player-poster.patch` | Done | Required source repair discovered by actual browser testing. |

### Audit Summary

23 audited rows: 22 done and one conditional file choice changed because no edit was needed. No partial or skipped requirement.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Make quickstart the primary action | Browser workflow | First primary action reached `/guides/quickstart/`; same navigation worked with JavaScript disabled. |
| Shorten opening without losing information | Browser inventory and visual proof | `homepage-before.json` versus `homepage-browser-proof.json`: no missing destinations or section headings; desktop and mobile screenshots show adjacent capability groups. |
| Show meaningful recorded content before playback | Browser state and regression proof | Idle AS 65000 dashboard, timer 00:00; before-fix seek/step lost cells, repaired canonical player retains named peers at 00:48/00:43. |
| Keep existing design and useful paths | Browser workflow | Light/dark, all keyboard panels, no-JavaScript content/transcripts, 00:00 replay, 00:36 chapter, fullscreen and unchanged inner quickstart page. |
| Publish through the existing native path | Artifact check | 23 producers, 1,091 published/written pages; canonical bundle SHA-256 `a9a460ee15f1b76d821a71cf594aa6e20cbe8299caf99ef7d4c14243d9042cb4` matches source and browser fetch. |

## Work Not Done

No homepage implementation or presentation acceptance item remains. The owner's separate instruction to commit all pending repository work and push is tracked by the parent session. Main-branch push needs a passing repository verification record; existing verification is red and pending `go.sum` broadens that run beyond the agent's permitted Linux-host scope. This is not a claim of a completed push.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `website/tools/asciinema-player-poster.patch` | Yes | Read source patch; independent review traced its guard to the readable upstream driver. Existing `tools/` exclusion keeps it unpublished. |
| Vendor bundle and README | Yes | Source/public SHA-256 match; distributed modification notice observed in browser fetch. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Primary quickstart works | Chromium destination `/guides/quickstart/`, title `Quick Start - Ze`. |
| AC-2 | Content preserved | Browser inventory `missingLinks: []`, `missingHeadings: []`. |
| AC-3 | Useful idle poster, no autoplay | Dashboard visible with marker active and `00:00-32:57` timer before interaction. |
| AC-4 | Playback and navigation correct | Canonical seek/step kept AS 65000 and 127.0.0.2; playback 00:00, chapter 00:36; text poster cleared normally. |
| AC-5 | Keyboard panels stable | Every selected panel visible and active, all others inert; unchanged measured panel height. |
| AC-6 | Responsive and fallback paths work | Scroll widths 1440/390/320 equal viewport; five no-JavaScript panels visible; primary link navigated. |
| AC-7 | Canonical output checks | Final site check passed; source/public/browser bundle hashes agree. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Generated homepage served from canonical `../gh-pages` | N-A: actual Chromium UI smoke, no new wording/wiring test | Existing producer rendered the template, bundled assets mounted the real recording, and primary navigation worked. |
| Existing management chapter | N-A: actual Chromium UI smoke | Real chapter recording started at configured 00:36 without idle-only enlargement. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Validated | Recording inspection and actual 43-second dashboard screenshot. |
| A-2 | Validated with producer repair | Independent poster/start options preserved; driver fix removes destructive npt clearing, verified through keyboard and Play. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Homepage presentation contract | `homeTemplate`, scoped CSS and `initTerminalDemoPlayers` | Independent content review clean; real layouts observed. |
| Poster/chapter and source anchors | JS chapter precedence, native driver `clearPoster`, `isSourceOnly` | Independent lifecycle/provenance reviews clean. |
| Vendor origin and repair | Verified npm readable bundle, retained patch, pinned minifier and distributed notice | Pristine reproduction recorded; final source/public hash match observed. |
| Config, API, SDK, protocol, metrics and feature inventory | No homepage change reaches these surfaces | No page update owed for this presentation change. |
