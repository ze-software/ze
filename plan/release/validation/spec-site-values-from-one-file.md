# Spec: site-values-from-one-file

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P1 |
| Release reason | Public site figures are baked into pages by separate counters, so the site can contradict itself and a changed value needs every page rewritten; accurate public claims are release validation |
| RFC gaps | - |
| Depends | - |
| Phase | 7/7 |
| Handoff | - |
| Updated | 2026-10-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner rule (`ai/META-DECISIONS.md`, Website design): every website value that
changes over time lives in one JSON file, and pages render it with JavaScript,
so updating that file updates every page that shows the value.

Agreed with Thomas on 2026-10-29:

| Decision | Choice |
|----------|--------|
| The one file | `data/site-facts.json`, written by the site build |
| Text before JS runs | the value from the last build, inside the `data-ze-stat` span; JS replaces it at load |
| Scope | every value shown site-wide: homepage, shared header, health, compare, CLI reference, RFC compliance index, release and download data. A per-RFC detail page's own figures stay generated in that page, which is their only copy |
| Release and download data | superseded at the research gate (2026-10-30): no build-time fetch. `./le site release update` fetches GitHub's releases data and commits it under `website/data/`; the build reads only that committed file, and updating the data is one command and one commit |
| Page descriptions (meta, Open Graph, Twitter) | reworded with no figures, so no stale count reaches a link preview and the guard covers `<head>` with no exemption (research gate) |
| Appliance ISO size | "under 150 MB" stays as the marked element's fallback text, replaced by the release ISO asset's size once a release carries one. The dated sentence about the 2026-10-01 local build is a dated measurement, not a changing value, and stays as written (design gate) |
| Health page | every metric the page shows goes through the snapshot: a `health` section carries each metric of the test-health record the page renders, keyed by the metric's own `key`, and the page renders every figure as a marked element (final gate, 2026-10-30) |
| Design | approved 2026-10-30: one marked-value helper, token names derived from the snapshot's JSON keys, tokens in hand-written HTML, committed release data, a plain-text guard in `./le site check`, a Playwright browser test of the fill |
| Regression guard | a site check refuses a page that shows a published value as plain text outside `data-ze-stat` markup |
| Folder | `plan/release/validation/` |
| Boundary with the RFC section | split by surface (owner ruling, `ai/META-DECISIONS.md` commit e6e4f9dfb3): RFC completion data lives per RFC and per profile under `data/rfc/`, owned by `plan/release/validation/spec-rfc-figures-sectioned-presentation.md`. This spec does not touch `internal/le/site/rfc*.go` or `website/assets/js/rfc-figures.js`. RFC aggregates shown OUTSIDE the RFC section (home, health) read the same producer the RFC index uses. `facts.go` and `home.go` are shared with that spec: the later lander rebases |
| Guard covers the RFC section too | agreed with rfc-website 2026-10-29: no exemption. The guard accepts a value only inside a marked element: `data-ze-stat="<key>"` (source `data/site-facts.json`) or `data-ze-rfc="<stem>:<field>"` (source `data/rfc/<stem>.json`). RFC pages still carrying server-written figures sit in a committed ratchet list that may only shrink, emptied by the RFC spec's website phase |

Found before the spec opened:

| Surface | Today |
|---------|-------|
| Homepage proof strip, `compare/nos` | use `data-ze-stat` spans |
| `{{ze:<name>}}` tokens | supported in website-owned Markdown only; one source uses one (`website/compare/nos.md`, `interop-scenarios`) |
| RFC compliance index | aggregate figures baked as plain text; it says "240 summaries" where the snapshot's `rfc.summaries` is 229 |
| `quality/health`, `reference/cli`, shared `assets/header.html` ("490 commands") | figures baked as plain text |
| `labs/appliance-install` | ISO size, build name and byte count typed by hand |
| Release downloads | no data slot exists |

## Required Reading

- [ ] `internal/le/site/facts.go` - the snapshot model and its derivation
  → Constraint: `siteFacts` is the ONE model of `data/site-facts.json`; fields are declared in JSON-key alphabetical order because the file is published key-sorted. A new section (release) is a new field in that order
  → Constraint: `publishSiteFacts` runs in `refreshNativeSurfaces`, before any page producer, after the command catalog and config tree; a producer cannot write an input another producer reads
  → Constraint: a build over an unchanged tree changes no byte of the file (`previousFactsAt` keeps the previous file when only the stamps differ, AC-14 of the site-renderers spec)
  → Constraint: a number the tree cannot answer stops the build by name; only the network fact (`github_stars`) may be carried, and `_sources` then says "carried" or "unknown". The release section follows the same three-state source sentence
  → Constraint: the network reach is one bounded context (5 s), body capped at 1 MiB, variable-injected (`liveGitHubStars`) so tests never reach the network
  → Constraint: `displayCount` floors magnitudes and marks a plus; `_display` keys exist so churn is low. RFC figures are printed exactly because `./le rfc check` compares them
  → Decision: `rfc.summaries` counts the RFCs that hold at least one requirement (229), not the summary files (the RFC index's "240 summaries"). The two pages count different populations under one word; the fix publishes both under distinct keys or one page changes its wording, never two counters for one population

- [ ] `docs/architecture/site-facts.md` - where a published number must come from
  → Constraint: one derivation, one committed file, one reader, one gate, one fix; release data is the Network category, like `github_stars`, and gets a row in that page's category table
  → Constraint: a gate judges what a reader sees (the published string), not the raw count
- [ ] `website/AI.md` - "Prose number tokens", "The facts snapshot"
  → Constraint: `data/site-facts.json` is written before any page producer runs; Markdown mirrors take plain text, HTML takes the span
  → Decision: its sentence that spans let "a build check a published number against the snapshot" has no checker behind it (no producer reads `data-ze-stat` back); this spec supplies the check and corrects the page in the same phase
- [ ] `ai/META-DECISIONS.md` - Website design, last bullet
  → Constraint: exactly one JSON file per value; site-wide values in `data/site-facts.json`, RFC completion data in `data/rfc/`; a page keeps only the last build's value as fallback text inside the marked element

**Key insights:** (minimal context to resume after compaction)
- The snapshot and the JS fill exist; the defect is that most producers write the value as plain text, and three of them re-count populations the snapshot already counts
- The shared header is inserted after `DOMContentLoaded` has run the stat fill, so a span in it is never filled until the fill re-runs after insertion
- Tokens map through a hand-written name table (`numberTokenSpecs`) with three sibling tables; the generic dotted-path resolver `factValue` already exists, so token names can derive from the snapshot's own JSON keys
- No GitHub release exists; the releases list answers empty, so the release section states "none yet" as a real answer, not an error
- No JS test harness exists; the fill is untested

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/site/facts.go` - `deriveSiteFacts` builds the snapshot from committed repo facts, test health, the RFC ledger, site data and the star fetch; `publishSiteFacts` writes it before producers run
- [ ] `internal/le/site/nav.go` - `navCounts` replaces six `%(x)s` placeholders in `website/data/nav.json` descriptions with plain integers; the star badge prints `GitHubStars` plain in its label and count
- [ ] `internal/le/site/commands.go` - `cliReferenceDescription` and `cliReferenceLead` print `len(commands)` and `len(groups)` plain: a second count of `cli_commands`, and a group count with no key
- [ ] `internal/le/site/equivalents.go` - `equivalentIndexBody` → `writeStat` prints `len(rows)` (second count of `cli_commands`) and the equivalent, reviewed, mapping and vendor-only counts, none with a key; the mirror re-counts two of them
- [ ] `internal/le/site/home.go` - `homeProofStats` writes spans for the proof strip; `featureCategoryLinks` prints per-category card counts plain
- [ ] `internal/le/site/docpage.go` - `numberTokenSpecs` hand map name → dotted key; `substitute` writes span or plain; `factValue` resolves any dotted path; `loadNumberTokens` leaves tokens alone when no snapshot exists
- [ ] `internal/le/site/authored.go` - `publishAuthoredPage` reads hand-written HTML, wraps it, extracts main, converts the mirror; no token pass
- [ ] `internal/le/site/derived.go` - `writeLLMSProductSnapshot` prints a plugin-registration count (no key) and raw `Interop.Scenarios` where pages show `scenarios_display`; `liveNavDescription` is a switch on href
- [ ] `internal/le/site/actions.go` - `runCheck` runs source walk, named artifacts, page mirrors, coverage; reports through `checkReport` fields; exit 1 on any finding
- [ ] `website/assets/js/site.js` - `initRepoStats` fetches the snapshot once in `DOMContentLoaded` and fills `[data-ze-stat]`; `loadSharedHeader` inserts the header later
- [ ] `website/labs/appliance-install/index.html` - ISO name, byte count and "under 150 MB" typed by hand
- [ ] `internal/appliance/cmd_iso.go` - `resolveISOOutput` names the ISO from the image stem (a timestamp); the manifest carries `image-sha256`, no size

**Behavior to preserve:**
- A build over an unchanged tree rewrites no byte of `data/site-facts.json`
- A number the tree cannot answer stops the build by name; only network facts may be carried, with `_sources` saying live, carried or unknown
- Markdown mirrors and `llms.txt` carry plain text
- A first build with no snapshot leaves tokens unresolved rather than refusing them
- `displayCount` rounding, and exact RFC figures

**Behavior to change:**
- Every site-wide changing value outside the RFC section reaches HTML only inside `data-ze-stat`, read from the snapshot, never from a second count
- The snapshot gains the keys pages print without one (CLI group count, equivalents counts, per-category feature counts, plugin registrations) and a release section
- Tokens resolve in hand-written HTML too, and token names derive from the snapshot's keys
- The JS fill also runs over the shared header after insertion
- A site check refuses a published value outside its marker

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le site build`: committed `website/data/repo-facts.json`, the test-health record, `rfc/short/*.md`, `website/data/*.json`, `go.mod`, the built binary's command catalog and config tree, and two network reads (GitHub repository record, GitHub releases list)
- Page load: the browser fetches `data/site-facts.json`

### Transformation Path
1. `refreshNativeSurfaces` publishes the command catalog and config tree, then `publishSiteFacts` derives and writes `data/site-facts.json`
2. Each producer (nav, home, docs, authored, commands, equivalents, health) reads the snapshot and emits each value as a `data-ze-stat` span holding the display string; mirrors and `llms.txt` emit the plain string from the same snapshot
3. The new check walks published HTML text nodes and refuses a known published string outside a marker
4. In the browser, `site.js` fetches the snapshot once and fills every marked element, the inserted header included

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build ↔ GitHub API | HTTPS GET, bounded context, body cap, carried on failure | No |
| Build ↔ artifact | `data/site-facts.json`, key-sorted JSON | Yes (`facts.go`) |
| Artifact ↔ browser | `fetch`, dotted key lookup (`lookupFact`) | Yes (`site.js`) |
| Header fragment ↔ page | `assets/header.html` inserted after `DOMContentLoaded` | Yes (`site.js`) |

### Integration Points
- `siteFacts` - gains fields; one model stays one model
- `factValue` - becomes the only token resolver
- `runCheck` / `checkReport` - gains the plain-text finding
- `initRepoStats` - re-runs over the header after `loadSharedHeader`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | design phase |
| No unintended coupling (components stay isolated) | No | design phase |
| No duplicated functionality (extends existing, does not recreate) | No | design phase |
| Zero-copy preserved where applicable (refs, not copies) | N-A | build tooling, no hot path |
| Registration over hardcoding, outbound | No | design phase |
| Registration over hardcoding, inbound: searched `numberTokenSpecs`, `navCounts`, the `homeProofStats` slot map, the `liveNavDescription` switch | No | design phase |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The RFC index's "240 summaries" and the snapshot's 229 count different populations (summary files vs RFCs holding a requirement) | `factsFromRFCLedger` counts RFCs with a requirement | the defect is misreported to rfc-website, which owns that page | rfc-website reads its producer's count | unvalidated |
| A-2 | The releases list (`/releases`) answers an empty array with no release, where `/releases/latest` would 404 | observed empty list via `gh release list` and the API; `/latest` behaviour from API docs, not observed | the update command reports an error instead of committing "none yet" | the command reads the list endpoint; a test states the empty answer | confirmed (2026-10-30): `/releases/latest` answered 404, `/releases` answered `[]` |
| A-3 | Meta and Open Graph descriptions can be worded without the number | crawlers read them without JS | a stale count survives in link previews | owner chose rewording at the research gate | confirmed |
| A-5 | The Playwright runner the demo pipeline uses can load a locally built page with a swapped `data/site-facts.json` | `home_test.go` fixture names a playwright browser engine with `run.cjs`; node is installed | the browser AC needs another harness | phase 1 runs a stub page through the runner before any JS change | broken (2026-10-30): no Playwright runner exists. `Playwright` and `run.cjs` appear only as fixture strings in `home_test.go` and `terminaldemo_test.go`; no `node_modules`, no Playwright browsers, no Go browser driver in `go.mod`. Google Chrome is installed on the Mac |
| A-6 | The ISO asset of a release is identifiable by its `.iso` name suffix | `resolveISOOutput` names it image stem + `.iso` | the lab shows no size after the first release | a unit test over a stated release record with one `.iso` asset and one other asset | unvalidated |
| A-4 | Raw values under 100 and years are too common to match without false positives | matches of 11, 38, 45, 52, 53, 59 across hundreds of pages | the check is noisy or misses small values | the check matches display strings plus unit-labelled values; a test seeds a plain small value next to its label | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A token resolved inside an attribute writes a span into markup | broken `<meta>` or `title` | substitution marks text nodes only; attributes take plain text; a test covers a token in an attribute |
| R-2 | Header spans stay at the last-build value because the fill ran before insertion | header count differs from the JSON after a JSON-only update | the fill re-runs after insertion; covered by a JS test |
| R-3 | Golden files pin plain header text | `TestTheSharedHeaderReadsAsThePublishedHeader` red | golden refresh through `ze-test-judge` |
| R-4 | rfc-website edits `facts.go` and `home.go` concurrently | rebase conflict | the later lander rebases; both specs name the shared files |
| R-5 | The update command fails (rate limit, network) | non-zero exit naming the endpoint and the retry command | the committed file is left untouched; the build never reaches the network for release data |
| R-6 | The guard's matcher misses a figure (a value under 100 with no unit word) | a plain small figure survives on a page | display strings and unit-labelled values cover the site-wide set; a test seeds each published key's display form on a fixture page |
| R-7 | Removing `numberTokenSpecs` breaks the existing `{{ze:interop-scenarios}}` in `website/compare/nos.md` | build refuses an unknown token | the source is rewritten to the dotted key in the same commit; the refusal names the key set |
| R-9 | The health section makes `data/site-facts.json` large and changes on every health refresh | file size grows past tens of KB; every page refetches it | only the fields the page renders are carried, not the record's internals; the size is stated in the phase report, and the owner hears it if it passes 64 KB |
| R-10 | A health metric's value is text with a unit (for example "112 / 39808") rather than one number | marked element holds a composite string the check cannot match | the snapshot carries each component as its own key; the page composes the sentence from marked parts |
| R-8 | A JSON-only update without a rebuild leaves fallback text stale for no-JS readers | `curl` of a page shows the old figure | accepted by the owner (fallback is the last build's value); the next build refreshes it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Published figures on the site (header, home, reference pages, lab) show wrong or empty values; no product binary changes |
| How is it reverted? | Single commit revert, then `./le site build` regenerates the artifact |
| Who else touches this path? | rfc-website (`plan/release/validation/spec-rfc-figures-sectioned-presentation.md`) in `facts.go` and `home.go`; the weekly-update flow regenerates the homepage |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le site build` over a fixture source tree | → | every producer emits values through the one marked-value helper | `TestEverySiteWideValueIsPublishedInsideItsMarker` |
| `./le site build` with `{{ze:<dotted.key>}}` in a hand-written HTML source | → | `publishAuthoredPage` token pass | `TestAHandWrittenPageResolvesATokenToAMarkedValue` |
| `./le site check` over an artifact with one plain figure | → | the plain-text guard in `runCheck` | `TestSiteCheckRefusesAPublishedValueOutsideItsMarker` |
| `./le site release update` against a stated API answer | → | the release action writes `website/data/github-releases.json` | `TestReleaseUpdateCommitsTheReleasesTheForgeStates` |
| browser load of a built page, then a swapped `data/site-facts.json` | → | `site.js` fill, header included | `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A site build over a fixture tree | Every value the snapshot holds that a page outside the RFC section shows (header counts and stars, homepage figures, per-category feature counts, CLI reference command and group counts, command-equivalents counts, health figures, the lab ISO size) appears in the page body only inside `data-ze-stat="<key>"`, whose text equals the snapshot's value for that key |
| AC-2 | The CLI reference and command-equivalents pages | Their command count is the snapshot's `cli_commands`; a fixture whose catalog and snapshot disagree publishes the snapshot's figure (no second count) |
| AC-3 | A page source carrying `{{ze:<dotted.key>}}` for any key the snapshot's JSON names | Resolves with no edit to any name table; a key the snapshot does not name stops the build with an error naming the token and how to list valid keys |
| AC-4 | A token in a text node of a hand-written HTML source | Becomes a marked span; the same token inside an attribute or inside `<head>` becomes the plain value, never a span |
| AC-5 | A build with no snapshot yet | Tokens stay unresolved, as today; no refusal |
| AC-6 | Page descriptions (meta description, Open Graph, Twitter) on every page this spec touches | Carry no snapshot value |
| AC-7 | `./le site check` over an artifact where one page shows a snapshot display string (thousands separator or "+") or a value of 100 or more beside its unit word, outside a marker | Exit 1, naming the page, the figure and the key it matched; the same artifact with the figure marked passes |
| AC-8 | The same check over an RFC page listed in the committed ratchet file | Not refused; a listed page that no longer carries a plain figure is refused as a stale ratchet entry, and a page cannot be added to the list by the check |
| AC-9 | `data-ze-rfc="<stem>:<field>"` elements | Accepted by the check as marked, the same as `data-ze-stat` |
| AC-10 | `./le site release update` when the forge has no release | Writes `website/data/github-releases.json` stating an empty release list and the endpoint it read; the snapshot's release section states "none yet" in `_sources` |
| AC-11 | The same command when the forge states a release with an `.iso` asset and another asset | The committed file keeps tag, name, publication time, prerelease flag, page URL and, per asset, name, size, sha256 digest and download URL; the snapshot exposes the latest release's version and its ISO asset's size and download URL |
| AC-12 | The update command cannot reach the forge | Non-zero exit naming the endpoint, the cause and the retry command; the committed file is unchanged |
| AC-13 | A site build | Never reaches the network for release data; it reads only the committed file |
| AC-14 | The appliance lab with no release | Shows "under 150 MB" as the marked fallback for the ISO size key; with a release carrying an ISO, the snapshot's size replaces it |
| AC-15 | A built page in a browser, then `data/site-facts.json` replaced with different values and the page reloaded | Every marked element, the shared header's included, shows the new values; `data/site-facts.json` is fetched once per page load |
| AC-16 | A build over an unchanged tree | Rewrites no byte of `data/site-facts.json` (preserved) |
| AC-18 | The health page over a fixture test-health record | Every figure the page shows (each metric's value, numerator, denominator, floor, threshold and percentage) is a marked element whose key is under `health.<metric key>`, and the snapshot holds every such key; a metric added to the record reaches the snapshot and the page with no edit to a name table |
| AC-17 | `llms.txt` and Markdown mirrors | Carry plain values read from the snapshot, `scenarios_display` included where pages show it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEverySiteWideValueIsPublishedInsideItsMarker` | `internal/le/site/factmark_test.go` | AC-1, wiring | |
| `TestTheCommandCountIsTheSnapshotsNotARecount` | `internal/le/site/commands_test.go` | AC-2 | |
| `TestATokenResolvesAnyKeyTheSnapshotNames` | `internal/le/site/factmark_test.go` | AC-3 | |
| `TestAnUnknownTokenIsRefusedByName` | `internal/le/site/factmark_test.go` | AC-3 | |
| `TestAHandWrittenPageResolvesATokenToAMarkedValue` | `internal/le/site/authored_test.go` | AC-4, wiring | |
| `TestATokenInAnAttributeOrHeadStaysPlain` | `internal/le/site/factmark_test.go` | AC-4 | |
| `TestTokensWaitForTheFirstSnapshot` | `internal/le/site/factmark_test.go` | AC-5 | |
| `TestPageDescriptionsCarryNoSnapshotValue` | `internal/le/site/factmark_test.go` | AC-6 | |
| `TestSiteCheckRefusesAPublishedValueOutsideItsMarker` | `internal/le/site/plaintext_test.go` | AC-7, wiring | |
| `TestSiteCheckHonoursAndShrinksTheRFCRatchet` | `internal/le/site/plaintext_test.go` | AC-8 | |
| `TestSiteCheckAcceptsTheRFCMarker` | `internal/le/site/plaintext_test.go` | AC-9 | |
| `TestReleaseUpdateCommitsTheReleasesTheForgeStates` | `internal/le/site/release/release_test.go` | AC-10, AC-11, wiring | |
| `TestReleaseUpdateLeavesTheFileWhenTheForgeIsUnreachable` | `internal/le/site/release/release_test.go` | AC-12 | |
| `TestTheBuildReadsReleaseDataOnlyFromTheCommittedFile` | `internal/le/site/facts_test.go` | AC-13 | |
| `TestTheLabISOSizeFallsBackUntilAReleaseCarriesOne` | `internal/le/site/facts_test.go` | AC-14, A-6 | |
| `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot` | `internal/le/site/browsertest/fill_test.go` | AC-15, wiring | |
| `TestEveryHealthFigureIsAMarkedSnapshotValue` | `internal/le/site/health_test.go` | AC-18 | |
| `TestANewHealthMetricReachesTheSnapshotUnnamed` | `internal/le/site/facts_test.go` | AC-18, derived not enumerated | |
| `TestLLMSAndMirrorsCarryPlainSnapshotValues` | `internal/le/site/derived_test.go` | AC-17 | |
| existing `TestASecondBuildChangesNothing` | `internal/le/site/site_test.go` | AC-16 (preserved) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| guard's raw-value threshold | values of 100 and over beside a unit word | 100 | 99 (not matched as raw; matched only as a display string) | N/A |
| release asset size | 0 to int64 max | 0 | negative (refused by the update command) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot` | `internal/le/site/browsertest/fill_test.go`, chromedp driving headless Chrome | a reader loads the home and CLI reference pages; after the JSON changes, both the header and the body show the new figures | |

### Golden files
`testdata/published-header.html` pins plain header text; moving the header to marked values changes it. The refresh goes through `ze-test-judge` (`ai/META-DECISIONS.md`, Test changes): the new golden must prove at least what the old one did.

## Files to Modify
- `internal/le/site/facts.go` - snapshot gains CLI group count, equivalents counts, per-category feature counts, plugin registrations, release section read from the committed file
- `internal/le/site/docpage.go` - `numberTokenSpecs` deleted; tokens resolve through `factValue` against the snapshot's JSON keys
- `internal/le/site/nav.go` - `navCounts` replacer deleted; nav descriptions use tokens and emit marked values; star badge count marked, its aria label worded without the number
- `internal/le/site/commands.go` - counts read from the snapshot, marked; descriptions reworded without figures
- `internal/le/site/equivalents.go` - counts read from the snapshot, marked; mirror reads the same values
- `internal/le/site/home.go` - proof strip slot map replaced by the helper; per-category counts marked
- `internal/le/site/authored.go` - token pass between read and shell
- `internal/le/site/derived.go` - `llms.txt` reads display values from the snapshot; `liveNavDescription` switch replaced by the nav data's tokens
- `internal/le/site/actions.go` - `runCheck` gains the plain-text guard
- `internal/le/site/health.go` - every figure rendered as a marked element under `health.<metric key>`, read from the snapshot
- `internal/le/site/facts.go` (again) - a `health` section derived from the same `readTestHealth` record the page reads, one entry per metric, keyed by the metric's `key`; the health section is a map, not a struct field per metric, so a new metric needs no edit
- `website/assets/js/site.js` - fill becomes callable on a subtree, re-run after header insertion, one fetch per load
- `website/data/nav.json` - `%(x)s` placeholders become `{{ze:<key>}}`
- `website/compare/nos.md` - `{{ze:interop-scenarios}}` becomes `{{ze:interop.scenarios_display}}`
- `website/labs/appliance-install/index.html` - ISO size sentence uses the release ISO size token with "under 150 MB" fallback
- `website/AI.md` - token syntax, the check, the release file
- `docs/architecture/site-facts.md` - release data category row, the plain-text guard, the committed release file
- `docs/contributing/gh-pages.md` - how to refresh release data

## Files to Create
- `internal/le/site/factmark.go` - the one marked-value helper and the key set derived from the snapshot's JSON tags
- `internal/le/site/plaintext.go` - the plain-text guard
- `internal/le/site/release/` (`register.go`, `actions.go`, `release.go`) - `./le site release update`, registered like `internal/le/site/facts`
- `website/data/github-releases.json` - committed release data, empty list until the first release
- `website/data/plain-figure-ratchet.json` - committed list of RFC pages allowed to carry server-written figures until rfc-website's website phase empties it
- `internal/le/site/browsertest/fill_test.go` - chromedp test serving a built fixture page; `go.mod` gains `github.com/chromedp/chromedp` (test-only use)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | site tooling, no YANG |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le site release update`, `internal/le/site/release/register.go` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`; the verb takes no value |
| Editor autocomplete | N-A | `le` completion derives from the action registry |
| Functional test for new RPC/API | N-A | no RPC; the browser test covers the user path |
| Pipe completeness | Yes | the action's report goes through le's `\| json` envelope like `site facts` |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | `le` tooling; the only new dependency is the GitHub API reached by an explicit command, not by the product |
| Prometheus counters/metrics | N-A | no runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | site tooling, no `features/` entry |
| 2 | Config syntax changed? | No | no config |
| 3 | CLI command added/changed? | Yes | `le help` derives from the registry; `docs/contributing/gh-pages.md` names the refresh command |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/contributing/gh-pages.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | the browser test's harness is described in `website/AI.md` |
| 11 | Affects daemon comparison? | No | `compare/nos` changes only its token name |
| 12 | Internal architecture changed? | Yes | `docs/architecture/site-facts.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `le` actions only |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/release/validation/spec-site-values-from-one-file.md` in phase 1; known: `docs/architecture/site-facts.md`, `website/AI.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `website/AI.md` token examples move to dotted keys |

### Discovery
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` row "Edit the website or presentations" → `website/AI.md`, and the `./le site facts` row; add a `./le site release` row |
| Rule that prevents regression | `ai/META-DECISIONS.md`, Website design, last bullet |
| Registry that prevents drift | the snapshot's JSON keys are the token set; no name table |
| Verification that proves it | `./le site check` plain-text guard |

## Implementation Steps

1. **Phase: Wiring** -- the marked-value helper, the derived key set, and failing wiring tests
   - Tests: `TestEverySiteWideValueIsPublishedInsideItsMarker`, `TestATokenResolvesAnyKeyTheSnapshotNames`, `TestSiteCheckRefusesAPublishedValueOutsideItsMarker`, plus `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot` failing against today's `site.js` (header span never filled)
   - Files: `factmark.go`, `plaintext.go`, `docpage.go`
   - Verify: tests fail because producers still print plain text
2. **Phase: Producers** -- nav, home, commands, equivalents, authored, derived through the helper; second counts removed; descriptions reworded; `nav.json` and `compare/nos.md` tokens rewritten
   - Tests: AC-1 to AC-6, AC-17 rows; golden refresh through `ze-test-judge`
   - Verify: wiring tests 1 and 2 pass
3. **Phase: Health** -- snapshot `health` section derived from the record, every health figure marked
   - Tests: AC-18 rows
4. **Phase: Release data** -- `./le site release update`, committed file, snapshot release section, lab ISO token
   - Tests: AC-10 to AC-14 rows
5. **Phase: Browser fill** -- `site.js` subtree fill after header insertion, one fetch
   - Tests: `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot`, run red against today's `site.js` first
6. **Phase: Guard and ratchet** -- the check over the whole artifact, the RFC ratchet file seeded with today's RFC pages, `data-ze-rfc` accepted
   - Tests: AC-7 to AC-9 rows; `./le site build` and `./le site check` on `../gh-pages` green
7. **Phase: Docs** -- each phase edits its page as it lands (`ai/rules/documentation.md`); this phase only confirms `./le doc check` and the citation anchors

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No producer outside the RFC section calls `len(...)` on a population the snapshot counts |
| Naming | Token names are the snapshot's dotted JSON keys, verbatim |
| Data flow | Only the update command reaches the network for release data |
| Rule: principles (central enumeration) | `numberTokenSpecs`, `navCounts`, the proof-strip slot map and the `liveNavDescription` switch are gone, not wrapped |
| Rule: no-layering | The old token names are deleted, not aliased |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No plain site-wide figure in the artifact | `./le site check` on `../gh-pages` |
| Release data committed | `website/data/github-releases.json` exists and the snapshot's `_sources` names it |
| Name tables gone | grep for `numberTokenSpecs` and `navCounts` finds nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the forge's answer is bounded (time, body size), parsed into a typed record, URLs kept only when https on github.com |
| Injection | token values and release fields are HTML-escaped in spans; JS sets `textContent`, never `innerHTML` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One helper, tokens named by the snapshot's keys | rewrite plain numbers after the build by value matching | matching by value is ambiguous (45 is both weekly updates and CLI groups) |
| Committed release file refreshed by a command | fetch at every build like the star count | owner choice at the research gate; a build stays offline for release data and the change is one commit |
| Browser test through chromedp driving headless Chrome, in its own package `internal/le/site/browsertest` | Playwright via npm (brings a Node toolchain); Node test with a DOM stub (no real page) | A-5 broke: no Playwright runner exists. Owner chose chromedp on 2026-10-30. Only a real page proves the header ordering defect is fixed. The test refuses (fails, never skips) when no Chrome is found, naming the install; its own package keeps `internal/le/site` tests runnable on a host without Chrome |

## Known Limitations
- The RFC section's figures are owned by `plan/release/validation/spec-rfc-figures-sectioned-presentation.md`; this spec only accepts its marker and seeds its ratchet
- No-JS readers see the last build's value until the next build (owner decision)

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- Commits a12a86c3df, 0a5f2f29c4, 9c266c60c1, 0bd25245a9, 7d3be5ac1c (chromedp vendor alone), 03ef9b1db2, 750348ea2c, 6c3cf2ac39, c9c74a5ff3, 82cfc35015, then this closure's commit A.
- One marked-value path: `markedFact`, `factLookup`, `substitute`/`substituteHTML`/`replaceTokens` (`internal/le/site/factmark.go`); hand-marked fallback elements (`markedValue`, `readMarkedText`, `factfallback.go`).
- Producers (nav, home, commands, equivalents, authored, derived, docs, blog, health) read every figure from the snapshot inside its marker; `numberTokenSpecs`, `navCounts` and `liveNavDescription` deleted.
- Health section of the snapshot derived from the record (`healthFacts`, `healthfacts.go`); every health figure marked (`health.go`).
- `./le site release update` (`internal/le/site/release/`), committed `website/data/github-releases.json`, snapshot `release` section (`factsFromReleases`, `releasefacts.go`), lab ISO size marker.
- `site.js` fills body and inserted header from one fetch (`loadSiteFacts`, `fillRepoStats`), proven in headless Chrome (`browsertest/fill_test.go`).
- Plain-figure guard in `./le site check` (`checkPlainFigures`, `plaintext.go`) with the RFC ratchet `website/data/plain-figure-ratchet.json`.
- Closure fix: the health meter's bar names its percentage key (`data-ze-stat-width`, `healthMeter`) and `fillRepoStats` sets its width from the snapshot.

### Bugs Found/Fixed
- The health meter's bar width was a plain copy of the percentage that no snapshot update moved, so after a JSON-only update the bar contradicted the marked figure beside it. Fixed in `healthMeter` (`internal/le/site/health.go`) and `fillRepoStats` (`website/assets/js/site.js`); covered by `TestTheMeterWidthIsTheMarkedPercentage` and the width assertion of `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot`, both red against HEAD (scratch `close-red/unit.txt`, `close-red/browser.txt`: `MeterWidth:1%` where `63.4%` was wanted).

### Documentation Updates
- `docs/architecture/site-facts.md`: categories, hand-marked element, release data, the RFC ratchet, and (closure) the meter bar row under the health keys table (anchor `healthMeter` via the row text).
- `website/AI.md`: token syntax, the facts snapshot, the browser fill (closure: `[data-ze-stat-width]`), the guard.
- `docs/contributing/gh-pages.md`: refreshing release data, the ratchet. `docs/architecture/testing/test-health.md`, `ai/INDEX.md` (`./le site release` row).
- `./le doc check all` (closure): rc=1, every finding from other sessions' work (DESIGN.md `policy` plugin, dns resolver catalog, three stale anchors to `filter_registry.go` and two flow-export drafts); "checked 3037 anchors across 23 digests, all resolve".

### Deviations from Plan
- A-5 broke: no Playwright runner existed; the browser test uses chromedp driving headless Chrome in its own package (owner choice, Key Design Decisions).
- The ratchet exempts a listed page as a whole, at AC-8's page granularity.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5: a Playwright runner was assumed available | only fixture strings named it | phase 1 stub run | chromedp test in `internal/le/site/browsertest` |
| approach | the meter bar's width was left as "the one plain copy" of a percentage | `site.js` can set a style from the snapshot as it sets text | closure review | `data-ze-stat-width`, fixed in commit A |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One file, `data/site-facts.json`, written by the build | Done | `internal/le/site/facts.go` `publishSiteFacts` | |
| Fallback text is the last build's value inside the marker | Done | `factmark.go` `markedFact`; `factfallback.go` `markedValue` | |
| Scope: header, home, health, compare, CLI reference, RFC index aggregates outside RFC section, release data | Done | producers listed above | RFC section owned by spec-rfc-figures-sectioned-presentation |
| Release data committed by a command, never fetched at build | Done | `release/actions.go` `update`; `releasefacts.go` `factsFromReleases` | |
| Page descriptions carry no figure | Done | `commands.go` `cliReferenceDescription` and peers | `TestPageDescriptionsCarryNoSnapshotValue` |
| ISO size fallback "under 150 MB" | Done | `website/labs/appliance-install/index.html` | |
| Health page through the snapshot | Done | `healthfacts.go` `healthFacts`; `health.go` | meter width included at closure |
| Regression guard in `./le site check` | Done | `plaintext.go` `checkPlainFigures`; `actions.go` `runCheck` | |
| Guard covers the RFC section through a shrinking ratchet, `data-ze-rfc` accepted | Done | `plaintext.go` `loadPlainFigureRatchet`, `figureMarked` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestEverySiteWideValueIsPublishedInsideItsMarker`, `TestTheLabISOSizeFallsBackUntilAReleaseCarriesOne` | |
| AC-2 | Done | `TestTheCommandCountIsTheSnapshotsNotARecount` | |
| AC-3 | Done | `TestATokenResolvesAnyKeyTheSnapshotNames`, `TestAnUnknownTokenIsRefusedByName` | |
| AC-4 | Done | `TestAHandWrittenPageResolvesATokenToAMarkedValue`, `TestATokenInAnAttributeOrHeadStaysPlain` | |
| AC-5 | Done | `TestTokensWaitForTheFirstSnapshot` | |
| AC-6 | Done | `TestPageDescriptionsCarryNoSnapshotValue` | |
| AC-7 | Done | `TestSiteCheckRefusesAPublishedValueOutsideItsMarker` | bare percentages are outside AC-7's matcher (Known Limitations) |
| AC-8 | Done | `TestSiteCheckHonoursAndShrinksTheRFCRatchet` | |
| AC-9 | Done | `TestSiteCheckAcceptsTheRFCMarker` | |
| AC-10 | Done | `TestReleaseUpdateCommitsTheReleasesTheForgeStates` | |
| AC-11 | Done | `TestReleaseUpdateCommitsTheReleasesTheForgeStates`, `TestTheBuildReadsReleaseDataOnlyFromTheCommittedFile` | |
| AC-12 | Done | `TestReleaseUpdateLeavesTheFileWhenTheForgeIsUnreachable` | |
| AC-13 | Done | `TestTheBuildReadsReleaseDataOnlyFromTheCommittedFile` | |
| AC-14 | Done | `TestTheLabISOSizeFallsBackUntilAReleaseCarriesOne` | |
| AC-15 | Done | `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot` | now also a bar width |
| AC-16 | Done | `TestASecondBuildChangesNothing` | |
| AC-17 | Done | `TestLLMSAndMirrorsCarryPlainSnapshotValues` | |
| AC-18 | Done | `TestEveryHealthFigureIsAMarkedSnapshotValue`, `TestANewHealthMetricReachesTheSnapshotUnnamed`, `TestTheMeterWidthIsTheMarkedPercentage` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every test in the TDD table | Done | files as the table names (each `func Test...` found by `git grep` at 82cfc35015) | |
| `TestTheMeterWidthIsTheMarkedPercentage` | Done | `internal/le/site/health_test.go` | added at closure |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file in Files to Modify and Files to Create | Done | `git show --stat` over the commits listed above; `ls` below |

### Audit Summary
- **Total items:** 9 requirements, 18 ACs, 21 tests
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (A-5, chromedp instead of Playwright, owner choice)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Updating `data/site-facts.json` updates every page that shows the value | functional (real browser) | `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot`: swapped snapshot, reload, header, body and bar show the new values, one fetch per load; red against HEAD's `site.js` for the bar |
| No value is printed outside its marker | unit over built pages, plus the check | `TestEverySiteWideValueIsPublishedInsideItsMarker`, `TestEveryHealthFigureIsAMarkedSnapshotValue`; phase 6 real check over a scratch build: exit 0, no plain figure outside the ratchet |
| Release data committed by one command, read only from the file | unit with refusing transport | `TestTheBuildReadsReleaseDataOnlyFromTheCommittedFile`, `TestReleaseUpdateCommitsTheReleasesTheForgeStates` |
| A regression is refused | negative test | `TestSiteCheckRefusesAPublishedValueOutsideItsMarker` exit 1 naming page, figure and key |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Emptying the plain-figure ratchet (RFC compliance index figures into `data-ze-rfc` markers) | the RFC section belongs to the RFC spec by owner ruling | `plan/release/validation/spec-rfc-figures-sectioned-presentation.md` (AC-19) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/site-values-from-one-file-682dd437-589f-4317-855c-acaec0f0a700.md` (8 files, verdict clean) |
| `./le spec review check` | clean |
| Population | every file of the commits carrying `Spec-work: site-values-from-one-file` plus commit A. The `vendor/` tree of 7d3be5ac1c (chromedp, cdproto, go-json-experiment) was judged as a vendored upstream module that only `internal/le/site/browsertest` imports, which the owner chose; its files were not read line by line |
| Rounds | 2 |
| Reviewer lenses used | wiring, removed behaviour, security (forge answer bounds, escaping), guard false negatives, test weakening (`./le test weakened check base a12a86c3df~1`: no finding in `internal/le/site`), style pass over changed Go |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The meter bar's width was a plain copy of the percentage, so a snapshot update left the bar contradicting the marked note beside it | `internal/le/site/health.go` `healthMeter`; `website/assets/js/site.js` `fillRepoStats` | `data-ze-stat-width` on the fill, width set from the snapshot; tests red then green |

### Notes (not blocking)
- The guard does not match a bare percentage ("63.4 %") or a figure under 100 with no thousands separator; AC-7 and A-4 bound the matcher to display strings and unit-labelled values of 100 or more. The page-level tests (AC-1, AC-18) cover those figures.
- The ratchet exempts a listed page wholesale and is bounded to the RFC section and to pages that still show a figure; growth by a hand edit is not refused. The RFC spec empties it.
- `File.Latest` counts a prerelease as the latest release, where the forge's `/releases/latest` would not.
- `publishedFigures` discards the found flag of `snapshot.value` for keys it took from `snapshot.keys()`, which always resolve.
- The 82cfc35015 commit carries no `Weakening-approved:` trailer and owed none: `testweakened.Check` (`internal/le/test/weakened/testweakened.go`) writes one only for a structural finding, and the range shows none in `internal/le/site`.

### Run 1
- 1 ISSUE (meter width), the NOTEs above.

### Run 2
- 0 BLOCKER, 0 ISSUE over the fix (`healthMeter`, `fillRepoStats`, the two tests, the two page edits).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/site/factmark.go`, `plaintext.go`, `release/{register,actions,release}.go`, `browsertest/fill_test.go`, `website/data/github-releases.json`, `website/data/plain-figure-ratchet.json` | Yes | `ls` at closure listed each |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-15, AC-18 | fill and meter | `go test -race ./internal/le/site/ -run 'Health\|Meter\|Series\|LongHistory'`: ok 7.965s; `go test ./internal/le/site/browsertest/`: ok 9.778s |
| AC-1 to AC-17 | as the audit | `go test -race ./internal/le/site/...` after 82cfc35015: site, browsertest, facts, release, wiki ok (implementation handoff); `./le verify changed` over 750348ea2c: every site package ok |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le site build`, `./le site check`, `./le site release update`, browser load | no `.ci`: Go tests drive each entry (`renderHealth`, `runCheck`, `update`, chromedp over the real `site.js`) | Yes, read at closure |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `factsRFC` in `facts.go` counts RFCs holding a requirement (`facts.RFC.Summaries = len(summaries)`); the index wording is the RFC spec's (its line on "240 summaries") |
| A-2 | confirmed | observed 2026-10-30 |
| A-3 | confirmed | owner choice |
| A-4 | confirmed | `plaintext_test.go` seeds 99 sections (not refused), 100 stars (refused), 1490 commands (not a 490 match) |
| A-5 | broken | Mistake Log, Deviations |
| A-6 | confirmed | `TestTheLabISOSizeFallsBackUntilAReleaseCarriesOne` (one `.iso` and another asset; two `.iso` refused) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| site-facts.md health keys row, AI.md browser fill | `healthMeter` writes `data-ze-stat-width`; `fillRepoStats` reads it | Yes |
| Citers of this spec's path | `git grep` found one, `factmark_test.go` comment, repointed to `docs/architecture/site-facts.md` | Yes |
