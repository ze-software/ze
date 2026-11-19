# Spec: fleet-ui-7 -- Recorded Demonstration of the Fleet Interface for Review on the Website

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | web, fleet, tooling |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-ui-1-interactive-prototype.md |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `docs/architecture/fleet-interface.md`,
`internal/le/site/lgreplay/recording.go`, `website/assets/js/lg-replay.js`.

## Task

Let the owner see the fleet interface from the website checkout, with no daemon
running, and without anyone else being able to find it.

The fleet interface (`spec-fleet-ui-1-interactive-prototype.md`) is rendered by
a daemon: every view, table sort, tree branch and chart is an answer from the
server. A copy of its pages saved as files would lose all of that. The website
already solves the same problem for the looking glass: `le` records the answers
of the real pages, and a small player on the site replays them inside a
sandboxed frame, so the product's own markup, styles and scripts run unchanged.

This spec applies that mechanism to the fleet interface. `le` records the fleet
handlers' answers to a fixed tour of requests, against the sample estate, at a
pinned clock. A host page in the website output plays the recording.

The owner's words, 2026-11-19: "Add it to the site so I can see it on the site.
Do not make it live; so people don't know it exists yet. It will be there for
me to review."

### Decisions

| Topic | Decision |
|-------|----------|
| Not live | The demonstration is written under the website output's `tmp/` directory. Git ignores it there, the page registry, the sitemap, the search index and the publication receipt skip it, and a site build carries it across. It is reachable from the owner's local review server and from nowhere else. No page links to it and it carries a no-index marker. Nothing is pushed |
| Own action | It is produced by its own action, `./le site fleet-demo render`, never by `./le site build`. A site build neither creates nor refreshes it |
| Publishing later | This spec never writes outside `tmp/`. The owner's ruling at closure (2026-11-19): "yes close it, keep it on request". The demonstration stays a feature rendered on request by `./le site fleet-demo render`, never automatically and never by the site build, and stays unpublished. Publishing it is a possible later decision of his (Known Limitations), not owed work of this spec |
| Mechanism | The looking glass replay: a recording of request and answer pairs, and a player that answers the page's requests from it inside a sandboxed frame |
| Data | The sample estate of `fleet-ui-1`, at a clock pinned to the commit date of the tree that is recorded, so two renders of one commit write the same bytes |
| Size | The recording is split: an index and the entry page travel with the frame; the answers of each object (the estate, each group node, each device, the administration pages) are a separate file the host page fetches when first needed and hands to the frame |
| Unrecorded requests | A request outside the recording is answered with a notice in the page: "This is not part of the recorded demonstration". Any posted action is answered with one recorded toast saying the demonstration changed nothing |
| Live updates | The event stream is answered as connected and silent. The demonstration says once, in its host bar, that live updates are not recorded |

### What the tour records

| Area | Requests recorded |
|------|-------------------|
| Estate | Every tab; the three chart ranges; the Busiest choices; each dashboard link's target view |
| Devices table | Each view at the default of every other part, no cross product: every sortable column in both directions; each page size at its first page; each page at the default size; each state, each version and each label value as a single filter term; the three saved filters; the side panel of every device the default view shows |
| Group nodes | For every node of the default tree view: every tab, default view of each table |
| Tree | The tree of each built-in view, and every branch that loads on expansion |
| Devices | For every device: every tab, every routing and logs sub-tab, the divergence view where there is one, the default view of each table at the day range of its charts, and the console's suggested commands. No sort and no side panel |
| Featured devices | For one device of each display state: also the hour and week chart ranges, every sort of its tables, and the side panels of its interfaces and peers |
| Tasks and alarms | Every tab and scope, the task panel's four tabs, the side panel of every task and alarm |
| Rollouts, audit, administration | Every page, each rollout's page, the new rollout form and its default preview, each audit entry's panel, each administration page and panel, the states page |

Not recorded, and answered with the notice: typed filter text, the tree filter,
the header search, any sort or page of a filtered view, a page past the first
at a page size other than the default, a sort or a side panel of an ordinary
device, and a reload of any address but the entry (a navigation is recorded as
the htmx partial request its control sends, and the entry alone as a whole
page).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-interface.md` - the fleet package, its routes, fragments and scripts
  → Constraint: `assets/fleet.js` reads the page path from `location.pathname` and navigates with `location.assign` and `location.replace`; inside a sandboxed frame the location is the frame document, so both must change
  → Constraint: the event stream uses `EventSource`, which the frame's content security policy refuses
  → Constraint: the sample source registers itself from `init()`, so importing its package registers a wall-clock source
- [ ] `internal/le/site/lgreplay/` and `internal/le/site/lgreplay.go` - the looking glass recorder and frame writer
  → Decision: reuse `Recording`, `Exchange`, `Asset`, `ExchangeKey`, `Encode` and `Read` as they are; the key is method, path, sorted query, sorted form and whether htmx asked
  → Constraint: the recorder drives a server over HTTP and its crawl is unexported; it caps a recording at 400 exchanges, which the fleet tour exceeds
- [ ] `website/assets/js/lg-replay.js` - the player
  → Constraint: it replaces `window.fetch` only; htmx 4 sends every request through `fetch`, so htmx works unchanged
  → Constraint: five parts are written for the looking glass: the asset path, the notice class, the mount attribute names, the tour controls and the stream; the frame holds the whole recording inline
  → Constraint: the frame refuses to play unless it is framed and sandboxed, so a host page is required
- [ ] `internal/le/site/` (`pages.go`, `seo.go`, `search.go`, `plaintext.go`, `paths.go`, the publication receipt) - what the site build does with files in the output
  → Constraint: a page file anywhere outside `tmp/` becomes a route, enters the sitemap and the search index, and fails the build's coverage; under `tmp/` none of that happens
  → Constraint: the plain-figure check scans every page file in the output, `tmp/` included
- [ ] `ai/doors/le-tooling.md` - le's rules
  → Constraint: a tool is a package at the path its command name predicts, registered with `leroot.Register`; `le` is built from the committed tree, so a new action exists only after commit
  → Constraint: a le file importing the fleet package carries the build line for `ze_fleet_ui`

**Key insights:**
- The mechanism exists and the owner approved it for the looking glass. This spec parameterises it; it does not write a second one.
- `tmp/` in the website output is already the declared unpublished place. Using it needs no change to what is published.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/site/lgreplay/recording.go` - the recording's types, key and encoding
- [ ] `internal/le/site/lgreplay/record.go`, `tour.go`, `actions.go` - the looking glass crawl, its tour and its action
- [ ] `internal/le/site/lgreplay.go` - writes the frame document with the recording and the player inline
- [ ] `website/assets/js/lg-replay.js` - the host mount and the frame player
- [ ] `internal/le/site/pages.go`, `seo.go`, `search.go`, `plaintext.go` - the walks over the output
- [ ] `internal/component/web/fleetui/assets/fleet.js` - `pagePath`, the navigation calls, the event stream client
- [ ] `internal/component/web/fleetui/sample/register.go`, `source.go` - registration from `init()`, the unexported constructor taking a clock
- [ ] `internal/component/web/fleetui/sample/handler_test.go` - mounts the registered fleet routes in process, with no hub

**Behavior to preserve:**
- The looking glass replay: its recording, its frame, its host page and its browser tests, byte for byte where nothing here requires a change
- The fleet interface served by a daemon: every route, page and script behaviour
- What `./le site build` and `./le site check` produce and judge for the published site
- No shipped `ze` links the sample source

**Behavior to change:**
- A new action writes a recorded demonstration of the fleet interface under the website output's `tmp/fleet-demo/`
- The fleet shell script takes the page path from the page, not from the browser location, and navigates through htmx
- The sample source can be constructed without registering it
- The player takes its product-specific parts from the recording, and can fetch answers through its host page
- The plain-figure check leaves unpublished directories alone

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Recording format | `internal/le/site/lgreplay/recording.go`, `Recording`, `Exchange`, `Asset`, `ExchangeKey`, `Encode`, `Read` | One file of request and answer pairs with provenance, keyed by request | Extends: the fleet recording uses the same types and key; the schema gains optional fields for the player's settings, the chunk index and fallback answers |
| Replay player | `website/assets/js/lg-replay.js` | Answers the frame's `fetch` calls from an inline recording; mounts a sandboxed frame from a host page | Extends: settings read from the recording with today's values as defaults; answers fetched through the host for chunked recordings; a silent event stream; fallback answers |
| Frame document writer | `internal/le/site/lgreplay.go`, `lgReplayFrameDocument` | Writes a frame holding the recording and the player inline, under a strict content security policy, marked no-index | Extends: the same writer produces the fleet frame, with a title and notice passed in |
| In-process fleet routes | `internal/component/web/fleetui/sample/handler_test.go`, `mountFleet` | A test mounts every registered fleet route on a mux with no hub | Extends: the same construction, exported from the fleet package for le to use |
| Sample estate at a fixed clock | `internal/component/web/fleetui/sample/source.go`, `newEstate` | Builds the estate from a clock function; tests pass a fixed one | Extends: exported; registration moves to a sub-package only the hub imports |
| Unpublished output directory | `internal/le/site`, `artifactIgnoreContent`, `unpublishedDirectories`, `publicationIgnored`, `pageRegistry` | `tmp/` in the output is ignored by git, the routes, the sitemap, the search index and the receipt | Extends: the demonstration is written there; the plain-figure walk joins the same declaration |
| le action registration | `internal/le/site/terminaldemo`, `internal/le/site/lgreplay` (`register.go`, `leroot.Register`) | Site sub-areas register their own actions | Extends: `site fleet-demo render` registers the same way |
| Site producers | `internal/le/site/build.go`, `registerProducer` | Sections of the published site | Does not apply: a producer runs in every site build and writes published pages. The demonstration is neither |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le site fleet-demo render`, run by a person
- Later, a browser opening the host page in the website output

### Transformation Path
1. The action builds the sample estate at a clock pinned to the commit date and mounts the fleet routes on an in-process server.
2. The recorder walks the tour: for each request it stores the answer under the request's key, grouped by the object the request belongs to. It follows the links and fragment requests the answers name, within the tour's bounds.
3. The action writes, under `tmp/fleet-demo/` in the website output: the host page, the frame document (player, settings, index, entry page, assets), and one chunk file per object.
4. In the browser the host page mounts the frame. The player answers each request from the recording; for a key whose chunk is not loaded it asks the host, which fetches the chunk file and passes it in.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le to the fleet package | Go imports under the `ze_fleet_ui` build line; handlers called over an in-process server | No |
| le to the website output | Files written under the resolved output's `tmp/fleet-demo/` only | No |
| Host page to frame | `postMessage`, as the looking glass replay does, extended with a chunk request and answer | No |

### Changed-Wire Census

One stored format changes: the recording gains optional fields. Its writer and
readers are listed. No network wire changes.

**Search evidence:** a search of the repository for `lg-replay.json`, `lg-replay-recording`, `Recording{`, `lgreplay.Read` and `lgreplay.Encode` finds the writer in `internal/le/site/lgreplay/`, the frame writer in `internal/le/site/lgreplay.go`, the player in `website/assets/js/lg-replay.js` and the tests in `internal/le/site/browsertest/`. Positive control: the committed recording `website/data/lg-replay.json` is found by the same search.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Recording file | writer | `internal/le/site/lgreplay/recording.go`, `Encode`, called by `site lg-replay record` | Read: writes schema 2 with kebab-case keys | Preserved for the looking glass: new fields are optional and absent from its file, so `website/data/lg-replay.json` does not change |
| Recording file | writer | `internal/le/site/fleetdemo` (new), through the same `Encode` | New | Writes the new optional fields |
| Recording file | reader | `internal/le/site/lgreplay/recording.go`, `Read` | Read: refuses another schema and unknown fields | Changed: accepts the new optional fields; the schema number stays, because a file without them means what it meant |
| Recording inside the frame | writer | `internal/le/site/lgreplay.go`, `lgReplayFrameDocument` | Read: writes the encoded recording into a JSON script element of the frame | Changed: takes a title and notice; the looking glass frame's recording element is written as before |
| Recording inside the frame | writer | `internal/le/site/fleetdemo` (new), through the same frame writer | New | Writes the index, the settings and the entry page; the per-object answers go to chunk files |
| Recording inside the frame | reader | `website/assets/js/lg-replay.js`, the frame player | Read: reads the inline JSON element | Changed: reads the optional settings with today's values as defaults, so the looking glass frame plays as before |
| Recording inside the frame | reader | `internal/le/site/browsertest/lgreplay_test.go` | Read: drives the looking glass frame | Preserved: must stay green unchanged |

### Integration Points
- `leroot.Register` and `internal/le/register.go` - the new action
- `internal/le/site` paths resolution - the output directory
- `fleetui` route mounting and `sample` construction - exported for le

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The recorder asks the same handlers a browser asks, over HTTP |
| No unintended coupling (components stay isolated) | Yes | The fleet package learns nothing about le or the website; le imports the fleet package, never the reverse |
| No duplicated functionality (extends existing, does not recreate) | Yes | One recording format, one player, one frame writer |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Not applicable: no wire encoding path |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The action registers; the player takes product settings from data |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | The blank import list in `internal/le/register.go` is le's declared registry of areas; the plain-figure walk reads the existing unpublished list instead of gaining a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The owner's local review server serves the `gh-pages` checkout including its `tmp/` directory | He reviews the site through a local server of that checkout | He opens the host page from the file system, or the server is told to serve `tmp/` | The owner opening the address | confirmed: the render sits in `../gh-pages/tmp/fleet-demo/` (74 files, `index.html` included), git ignores it (`.gitignore:1:tmp/`), and the owner ruled on it at closure ("yes close it, keep it on request") |
| A-2 | `htmx.ajax` goes through the same `fetch` path as attribute-driven requests | The vendored htmx 4 holds no other request path | Calls from the shell script escape the player and fail in the frame | The browser test over the tree and the task panel | confirmed: `TestFleetDemoOpensADeviceFromItsChunk` expands lazy tree branches, which `fleet.js` `load` asks through `htmx.ajax`, and the player answers them |
| A-3 | A sandboxed frame without same-origin can receive chunk data from its host by message at an acceptable speed | The looking glass replay already talks to its host by message | Chunks are inlined for a smaller tour | The browser test opening a device | confirmed: `TestFleetDemoOpensADeviceFromItsChunk` loads a device chunk (about 1 to 4 MB) through the host within its 20 s wait |
| A-4 | The full tour fits a size a static site can carry | Estimated at tens of megabytes across about sixty chunk files. Broken by the first render (2026-11-19): about 9,000 requests in about seven minutes, and the estate chunk alone 54,751,363 bytes, past `chunkBytesMax` (32 MiB). Two causes: the crawler sent `HX-Request` without `HX-Request-Type: partial`, so the shell (`asksPart`, `handler_shell.go`) answered every navigation with the whole shell (header, sidebar, task panel) where the browser receives the content region; and cross products in the tour. Narrowed: navigations recorded as the partial requests their controls send; the devices table at the default of every other part of a view; ordinary devices at the default view of each table, the day range, no sort and no side panel (the tour table above). Targets: under 40 MiB in total, every chunk under 16 MiB, a render under about two and a half minutes. Second render (2026-11-19, after the narrowing): the estate chunk 50.3 MB, of which side panels 23.5 MB (88 answers of about 266 kB: opening a panel re-rendered the whole page) and table views 14.6 MB. Both were inefficiencies of the live interface and were fixed there: the eye control now fetches the side panel fragment alone, and a navigation inside one object no longer resends the tree and administration list (`handler_shell.go`, `keepsSidebar`). The devices table's views moved to a chunk of their own (`estate-devices`), so the estate dashboard opens without them. Measured at that point: every chunk under 16 MiB, a render under two minutes, and 87.6 MB in total, most of it the device tabs (959 partial pages of about 60 kB). The total bound was raised to 128 MiB, because the recording is a local review artifact under the output's ignored `tmp/` and a viewer loads one chunk per object it opens, so the total is a disk figure and not a download. A published demonstration would first need the larger lever: the inline icon markup, about half of every table row | The tour is narrowed further where the report's kinds say it grew; a published demonstration shares icon markup first | The action reporting each chunk's size and kinds and the total; `TestFleetDemoReportsItsSizeByKind`, `TestFleetDemoNarrowsOrdinaryDevices` | broken as worded: holds for each chunk, not for the total; the total bound restated with its reason |
| A-5 | The fleet scripts run under the frame's content security policy once inlined | The looking glass scripts do | A script construct the policy refuses is changed in the fleet script | The browser test | confirmed: `TestFleetDemoPlaysTheEstateDashboard` asserts `data-lg-replay-errors` is empty, and the frame reports every `securitypolicyviolation` there |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The demonstration is published by accident | A file of it appears in `git status` of the output, in the sitemap or in the search index | It is written only under `tmp/`; a test renders into a scratch output and asserts that the page registry, the sitemap, the search index and the publication receipt do not see it and that git ignores every file |
| R-2 | Changing the player breaks the looking glass replay | Its browser tests go red, or its frame's bytes change | New behaviour is off unless the recording asks for it; the looking glass tests run unchanged |
| R-3 | The recording goes stale as the fleet pages change | The demonstration shows an old page | The host bar states the commit and date it was recorded from; re-rendering is one command |
| R-4 | The recording is not deterministic | Two renders of one commit differ | The clock is pinned, the sample history is seeded, nothing subscribes to the script; a test renders twice and compares. It renders the sampled tour (`tourScope.sampled`: the estate, the task list, the administration, one group subtree, one featured and one ordinary device) through the same `render`, so the package renders the whole tour once, in the render its other tests share |
| R-5 | A fleet script change for the frame alters the live interface | A fleet package test or browser test goes red | The page path attribute and htmx navigation are covered by the fleet package's own tests, which run before the recorder is used |
| R-6 | The tour's crawl grows without bound | The exchange count or size passes its stated bound | Stated bounds per object and in total: `tourExchangesMax` 5,000 (measured 2,007), `chunkExchangesMax` 2,000, `chunkBytesMax` 16 MiB (every chunk measured under it), `totalBytesMax` 128 MiB (measured 88,127,981 bytes; raised from 40 MiB with its reason, A-4); `renderWait` 10 minutes. The action fails naming the bound, what exceeded it, and what that chunk or the whole holds by kind of request, so the next narrowing is measured. The bounds are tightened to the measured figures with room to spare once the render has reported them |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The looking glass replay on the published labs page, if the player change is wrong; nothing else published |
| How is it reverted? | Remove the action's package and its import line; restore the player and the sample registration |
| Who else touches this path? | Whoever works on the looking glass replay or the site generator's output walks |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le site fleet-demo render` | → | The recorder writes the host page, the frame and the chunks under `tmp/fleet-demo/` | `TestFleetDemoRenderWritesTheDemonstration` |
| A browser opens the host page | → | The player shows the estate dashboard from the recording | `TestFleetDemoPlaysTheEstateDashboard` |
| A browser opens a device from the tree | → | The host fetches that device's chunk and the player answers from it | `TestFleetDemoOpensADeviceFromItsChunk` |
| `./le site build` over an output holding the demonstration | → | The published site neither lists nor changes it | `TestFleetDemoStaysOutOfThePublishedSite` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le site fleet-demo render` on a committed tree | The action writes the host page, the frame and one chunk per object under the output's `tmp/fleet-demo/`, and reports the exchange count and the size of each chunk and of the whole | It writes nothing outside that directory, because the demonstration is not published |
| AC-2 | The host page opened from the owner's review server | The estate dashboard of the sample estate appears in the frame, in the fleet interface's own styles, with a host bar naming it a recorded demonstration on sample data, the commit and date it was recorded from, and that live updates are not recorded | The website's own navigation is absent, because the page is not part of the site |
| AC-3 | Every request in the tour table | The page answers from the recording exactly as the daemon answered: tabs, tree branches, sorts, page sizes, single-term filters, saved filters, side panels, chart ranges, sub-tabs, console suggestions | Timed refreshes of device sections answer the same recorded content, because the recording is one moment |
| AC-4 | A request outside the tour | The page shows the notice that it is not part of the recorded demonstration, in place of the answer, and stays usable | Typed filter text and searches remain typeable, because the notice explains what happened |
| AC-5 | Any posted action, after its confirm dialog | The page shows an informational toast reading "Recorded demonstration: this action was not sent", and the view it was posted from stays as it was | The confirm dialogs still open and can be cancelled, because they run in the page |
| AC-6 | A site build, a site check and a publication receipt over an output holding the demonstration | Each completes as it does without it: the route list, the sitemap, the search index and the receipt hold no file of the demonstration, and git reports none as untracked | The files stay on disk across the build, because `tmp/` is carried over |
| AC-7 | Two renders of one commit | Every written file is identical | Nothing may differ, because the review must be of a known state |
| AC-8 | The looking glass replay | Its recording file is unchanged, its frame plays, and its browser tests pass unchanged | Its frame document may gain the player's new code, because the player is one file |
| AC-9 | The fleet interface served by a daemon | Every page and interaction works as before the script changes, proven by the fleet package tests and browser tests | None |
| AC-10 | Any shipped `ze` | The sample source is not linked, proven by the existing absence test | le links it, because le is development tooling |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Renders the demonstration and opens it from the review server | Action, recorder, files; host page, frame, player | `TestFleetDemoPlaysTheEstateDashboard` |
| 2 | Clicks through to a device and its tabs | Tree, chunk fetch, recorded answers | `TestFleetDemoOpensADeviceFromItsChunk` |
| 3 | Sorts the devices table and opens a row's panel | Recorded sort and panel answers | `TestFleetDemoSortsAndOpensAPanel` |
| 4 | Types a filter that was not recorded | The notice | `TestFleetDemoAnswersAnUnrecordedRequestWithTheNotice` |
| 5 | Builds the site afterwards | Site build and check ignore the directory | `TestFleetDemoStaysOutOfThePublishedSite` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFleetDemoRenderWritesTheDemonstration` | `internal/le/site/fleetdemo/render_test.go` | The files written, their names, the index naming every chunk | |
| `TestFleetDemoRenderIsDeterministic` | `internal/le/site/fleetdemo/render_test.go` | Two renders of the sampled tour (`tourScope.sampled`) are identical || written, pending run |
| `TestFleetDemoRecordsPagesAsTheirPartialAnswer` | `internal/le/site/fleetdemo/tour_test.go` | A navigation is recorded as the region htmx asks for, with no shell header; only the entry is a whole page || written, pending run |
| `TestFleetDemoReportsItsSizeByKind` | `internal/le/site/fleetdemo/tour_test.go` | The report counts every answer and its bytes once by kind of request, in the whole and in each chunk, and logs the figures || written, pending run |
| `TestFleetDemoNarrowsOrdinaryDevices` | `internal/le/site/fleetdemo/tour_test.go` | A featured device has side panels and sorts, an ordinary one neither; no later page of the devices table at a non-default size || written, pending run |
| `TestCrawlerSendsTheRequestTypeHTMXSends` | `internal/le/site/lgreplay/crawl_test.go` | An htmx step carries `HX-Request` and `HX-Request-Type: partial`, a plain step neither || written, pending run |
| `TestFleetDemoTourCoversEveryRegisteredPage` | `internal/le/site/fleetdemo/tour_test.go` | Every registered fleet page and tab has an exchange; a page added later fails the test until the tour reaches it || written, pending run |
| `TestFleetDemoBoundsAreEnforced` | `internal/le/site/fleetdemo/tour_test.go` | Passing the exchange or size bound fails with the bound named || written, pending run |
| `TestFleetDemoStaysOutOfThePublishedSite` | `internal/le/site/unpublished_test.go` (package `site`: it runs a real `Build`, whose stubs and walks are unexported, and package `sitefleetdemo` imports `site`, so the render's files are laid down by name) | Route list, sitemap, search index, receipt and git ignore over a scratch output, after a full build that carries the files across unchanged | written, pending run |
| `TestRecordingOptionalFieldsRoundTrip` | `internal/le/site/lgreplay/recording_test.go` | A recording with and without the new fields reads back; the looking glass file is unchanged | |
| `TestPlainFiguresSkipUnpublishedDirectories` | `internal/le/site/plaintext_test.go` | A page under `tmp/` is not scanned; one outside still is, and so is the shared header under `assets/` | written, pending run |
| `TestFleetPagePathComesFromThePage` | `internal/component/web/fleetui/handler_shell_test.go` | Every answer that changes the page carries the page path attribute | |
| `TestSampleNewRegistersNothing` | `internal/component/web/fleetui/sample/source_test.go` | Constructing the estate leaves the source slot empty | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Exchanges per chunk | 1 to the stated bound | The bound | N-A | The action fails naming the chunk |
| Total size | Up to the stated bound | The bound | N-A | The action fails naming the total |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestFleetDemoPlaysTheEstateDashboard` | `internal/le/site/browsertest/fleetdemo_test.go` | The host page shows the estate dashboard | written, pending run (renders the whole tour once per run, `sitefleetdemo.RenderInto`) |
| `TestFleetDemoOpensADeviceFromItsChunk` | `internal/le/site/browsertest/fleetdemo_test.go` | A device opened from the tree shows its dashboard and tabs | written, pending run |
| `TestFleetDemoSortsAndOpensAPanel` | `internal/le/site/browsertest/fleetdemo_test.go` | A recorded sort and a side panel answer | written, pending run |
| `TestFleetDemoAnswersAnUnrecordedRequestWithTheNotice` | `internal/le/site/browsertest/fleetdemo_test.go` | A typed filter shows the notice, and a posted action the toast | written, pending run |
| `TestLGReplayAnswersTheTourFromTheRecording` (existing) | `internal/le/site/browsertest/lgreplay_test.go` | The looking glass replay still plays | |

## Files to Modify
- `internal/le/site/lgreplay/recording.go` - optional fields: player settings, chunk index, fallback answers
- `internal/le/site/lgreplay/record.go` - the generic crawl exported for a second recorder, its bound a parameter
- `internal/le/site/lgreplay.go` - the frame document writer takes a title and notice
- `website/assets/js/lg-replay.js` - settings from the recording, chunk requests through the host, the silent event stream, fallback answers, the page path from the page
- `internal/le/site/plaintext.go` - the plain-figure walk skips the unpublished directories
- `internal/le/register.go` - the new area's import
- `internal/component/web/fleetui/assets/fleet.js`, `shell.templ`, `handler_shell.go` - the page path attribute, navigation through htmx
- `internal/component/web/fleetui/register.go` - a constructor mounting the fleet routes over a given source, used by the tests and by le
- `internal/component/web/fleetui/sample/` - an exported constructor; registration moved to `sample/register`
- `cmd/ze/hub/register_fleet_ui_sample.go` - imports `sample/register`
- `docs/architecture/fleet-interface.md`, `website/AI.md` (the replay section), `docs/contributing/` page that documents site actions
- `plan/post-release/features/spec-fleet-ui-0-umbrella.md` - the child table

## Files to Create
- `internal/le/site/fleetdemo/` - `register.go`, `actions.go`, `render.go`, `tour.go`, the host page text, tests
- `internal/component/web/fleetui/sample/register/register.go` - the `init()` registration
- `internal/le/site/browsertest/fleetdemo_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling; no daemon surface |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | `le site fleet-demo render`, in `internal/le/site/fleetdemo/actions.go`; a le command, not a `ze` one |
| CLI grammar (keyword before value) | Yes | `./le cli grammar` judges it |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | Yes | The browser tests above |
| Pipe completeness | Yes | The action answers through the shared renderer, as le actions do |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | le tooling writes files; no daemon dependency |
| Doctor code per problem class | N-A | No daemon refusal or log line |
| Prometheus counters/metrics | N-A | Tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Nothing reaches a user of a shipped build or a visitor of the published site |
| 2 | Config syntax changed? | No | None |
| 3 | CLI command added/changed? | No | No `ze` command; the le action is documented under row 12 |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | No | Not a product feature |
| 7 | Wire format changed? | No | A stored tooling format gains optional fields, documented under row 12 |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | Yes | The page that documents the site browser tests, for the new cases |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-interface.md` (page path, navigation, construction over a source, the sample's registration sub-package); `website/AI.md` where it documents the replay (the player's settings, chunks, the second recording, where it is written and why it is not published) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors` once files change |
| 17 | Existing docs show config/CLI/API examples for this area? | No | None |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the action registered and reachable; the fleet package's constructor over a source; the sample's exported constructor and its registration sub-package; a render that writes a host page and a frame holding only the estate dashboard, played by the player as it is today plus the minimum (the page path)
   - Tests: `TestFleetDemoRenderWritesTheDemonstration`, `TestSampleNewRegistersNothing`, `TestFleetPagePathComesFromThePage`, the existing fleet and sample tests, the existing absence test
   - Verify: the owner can open the estate dashboard from the website output
2. **Phase: Player** -- settings from the recording, chunk requests through the host, the silent event stream, fallback answers; the looking glass tests unchanged
   - Tests: `TestRecordingOptionalFieldsRoundTrip`, the existing looking glass browser tests, `TestFleetDemoPlaysTheEstateDashboard`
3. **Phase: Tour** -- the full tour, the chunks, the bounds, determinism
   - Tests: `TestFleetDemoTourCoversEveryRegisteredPage`, `TestFleetDemoBoundsAreEnforced`, `TestFleetDemoRenderIsDeterministic`, the remaining browser tests
   - Note (phase agent, 2026-11-19): the fleet pages link every view of every table through `hx-get`, so following every link records every combination; `lgreplay.Crawler` gained an optional `Follow` (nil keeps the looking glass walk), and `tour.follow` asks fragments and one-step views only. Tree branches, chart data and console commands are not `hx-get`: `discover` finds them in the answers (`data-fleet-lazy`, `data-chart-url`, `data-terminal-fill`) and `crawlTour` asks them round after round. The routes see the estate through `plainSource` (Source plus the sample-data tag), so no Step/Hold control is recorded. Also `TestFleetDemoChunksMatchTheIndex` and `TestFleetDemoRecordsTheHTMXFlagEachControlSends` in `tour_test.go`. Not narrowed: the estimate is 60 to 70 MB; A-4 stays unvalidated until the render reports its sizes
   - Note (fix agent, 2026-11-19): the first render broke A-4 (estate chunk 54,751,363 bytes, about 9,000 requests in about seven minutes). The crawler now sends `HX-Request-Type: partial` with every htmx request, so a navigation records the content region and not the whole shell; the tour is narrowed (tour table, A-4); the report and a bound failure carry the bytes by kind of request (`sizeByKind`); the package renders the whole tour once, and the determinism test renders the sampled tour twice
4. **Phase: Not published** -- the plain-figure skip and the proof that the published site does not see the directory
   - Tests: `TestPlainFiguresSkipUnpublishedDirectories`, `TestFleetDemoStaysOutOfThePublishedSite`
   - Note (phase agent, 2026-11-19): the plain-figure walk reads `publicationIgnored`, the declaration the publication receipt reads (the artifact's ignore list and the git metadata), not `unpublishedDirectories`. The latter names `assets/`, and `assets/header.html` is the published shared header whose navigation descriptions carry figures, so skipping it would stop judging text every reader sees. `TestFleetDemoStaysOutOfThePublishedSite` moved to package `site` as the file column above states; render's own placement under `tmp/fleet-demo/` stays `TestFleetDemoRenderWritesTheDemonstration`'s. The search index assertion cannot go red by itself: its walk reads only Markdown mirrors and a render writes none
5. **Phase: Render for the owner** -- commit, run the action into the website output, give him the path

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an implementation at file and line |
| Feature completeness | Every row of the tour table is recorded and plays |
| Correctness | A recorded answer is byte for byte what the handler answered; the key the player computes equals the key the recorder stored, including for posted forms |
| Naming | The action is `site fleet-demo render` and its package is at the path that name predicts |
| Data flow | Nothing is written outside the output's `tmp/fleet-demo/` |
| Operator errors and doctor codes | The action's failures name the bound or the file and what to do |
| Rule: owner rulings on the website | No published page changes; the looking glass replay is untouched in behaviour |
| Rule: sample data | No shipped `ze` links the sample; the demonstration says it is sample data |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The action exists and is in le's help | `./le help` lists `site fleet-demo render`; `./le cli grammar` |
| The demonstration is written | The files under the website output's `tmp/fleet-demo/` |
| The published site does not see it | `./le site check`; `git -C ../gh-pages status --short` lists nothing under `tmp/` |
| The looking glass replay still plays | Its browser tests |
| Placement and imports are accepted | `./le arch tier check` reports nothing for the new package |

### Security Review Checklist
| Check | What to look for |
|-------------------|-----------------|
| Publication | Nothing of the demonstration reaches a tracked or published file |
| Frame isolation | The frame stays sandboxed without same-origin, under its strict content security policy; chunk data arrives only by message from its own host page, and the player checks the sender |
| Recorded content | Only sample data; documentation address ranges; no real name, address or secret |
| Chunk requests | The host fetches only file names taken from the recording's index, never a name the frame supplies freely |
| Size | Bounds enforced at record time |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A page that reads where it is from the browser's location cannot be shown anywhere but its own address. Taking the page path from the page itself costs nothing in the live interface and makes the pages portable.
- "Not published" is a property of where a file is written, not of whether something links to it: the site's own walks find every page in the published tree.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Record and replay in a frame | Saving the pages as static files with file-mapped links; compiling the fleet pages to run in the browser | Static files lose every server-answered interaction and need a second form of every address. Running in the browser is unproven and heavy. The replay runs the real pages unchanged and already exists on the site |
| Write under the output's `tmp/` | A new ignored root such as `review/`; a normal route left out of the navigation | `tmp/` is already declared unpublished in every walk but one. A new root means four declarations to change in the publication path. An unlinked route is still in the sitemap and the search index |
| Its own action | A producer in the site build | A producer runs in every build and its output is published. The demonstration must be neither |
| Chunks fetched through the host | One inline recording | The looking glass recording is small. The fleet tour is tens of megabytes; inline, every visit would load all of it |
| The page path comes from the page | A virtual location kept by the player | The player cannot change what the browser's location reports; the page can simply stop asking it |
| A silent event stream | Replaying the sample's scripted events | Events make the page refetch regions, which would need the recording at every step of the script. One moment, honestly labelled, is enough for a look |
| Parameterise the one player | A second player for the fleet pages | Two players would drift. The existing browser tests hold the looking glass behaviour while the player is generalised |

## Known Limitations
- The demonstration is one recorded moment. Nothing in it moves on its own.
- Typed filters, the tree filter and the header search are not recorded.
- It is not published, and it is rendered only on request with `./le site fleet-demo render`, never by the site build. Publishing it is a possible later decision of the owner, who has not asked for it. It would need a real route, a navigation entry, the site's page rules, and first a smaller recording: sharing the inline icon markup, about half of every table row (`docs/contributing/gh-pages.md`, "Render the fleet demonstration").
- A recording made from a dirty tree reads the player from the working tree and says so in its host bar ("with uncommitted changes"), so its bytes are those of that tree, not of the commit.
- A render whose chunk set shrank leaves the earlier chunk files in `tmp/fleet-demo/`. The index names only the current ones, so the player never fetches a stale file.
- Opening a side panel in the live interface now fetches the panel alone, so the table's sort and pager links keep the view they were drawn with, and the next sort closes the panel (`docs/architecture/fleet-interface.md`, the `PanelLink` paragraph). This departs from AC-9's "as before" (Deviations).

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove` this spec's path only, in the same `./le commit create` script

## Implementation Summary

### What Was Implemented
- `./le site fleet-demo render` (`internal/le/site/fleetdemo`, `actions.go` `runRender`, `render.go` `render`, `tour.go` `planTour`): it serves the fleet routes in process (`fleetui.NewRoutes`) over `sample.New` at a clock pinned to the HEAD commit date, crawls the tour, and writes the host page, the frame, `recording.json` and one chunk per object under the output's `tmp/fleet-demo/` only (`demoDirectoryIn` refuses a link or file on the way). It runs on request only, and no site producer calls it.
- The recording format gained optional `player`, `chunks` and `fallbacks` (`lgreplay/recording.go`, `checkParts`, `SplitChunks`, `ChunkFiles`). The crawl is shared and exported (`lgreplay/record.go`, `Crawler`, `Follow`), and it sends `HX-Request-Type: partial` and `HX-Current-URL` as htmx does.
- The player (`website/assets/js/lg-replay.js`): `mountFrame`, `chunkServer` (fetches only index-named files matching `CHUNK_FILE`), settings from the recording, a silent `EventSource`, fallback answers, the kept pushed address, plain forms. The frame checks its sender (`event.source !== window.parent`).
- Fleet package: the page path from the page (`shell.templ` `contentRegion`, `fleet.js` `navigate`), `NewRoutes` over a given source, `sample.New` with registration moved to `sample/register`, the side panel fetched alone (`PanelLink`, `side-panel` fragment) and the sidebar kept within one object (`keepsSidebar`). The gallery's hand-written addresses now come from the address builders (`component_gallery.go`).
- The plain-figure walk skips what `publicationIgnored` names (`plaintext.go` `plainFiguresOfArtifact`).

### Bugs Found/Fixed
- The crawl recorded every navigation as the whole shell (no `HX-Request-Type: partial`): `TestCrawlerSendsTheRequestTypeHTMXSends`, `TestFleetDemoRecordsPagesAsTheirPartialAnswer`.
- The component gallery linked addresses the server refuses (task tab with no page part, unregistered fragments, absent devices): `TestGalleryAddressesAreAnswered`.
- Opening a side panel re-rendered the whole page (about 266 kB an answer): `TestOpeningAPanelAnswersOnlyThePanel`.
- A browser test clicked a control before htmx bound it: `click` in `browsertest/fleetdemo_test.go` waits for `data-htmx-powered`.
- Closure: the guide stated `totalBytesMax` as 40 MiB where `tour.go` declares 128 MiB, and the bound failure's fix line sent the operator to this spec, which closure deletes (Review Gate, finding 1).

### Documentation Updates
- `docs/architecture/fleet-interface.md`: page path, navigation through htmx, `PanelLink` and the side panel fragment, construction over a source, the sample's registration sub-package (anchors `handler_shell.go`, `fleet.js`, `component_action.templ`).
- `docs/contributing/gh-pages.md`, "Render the fleet demonstration": the action, tour, chunks, bounds, notice, fallbacks and browser tests. At closure, the measured figures and the reason for the total bound moved here from A-4.
- `website/AI.md` (replay section), `docs/architecture/site-facts.md` (plain-figure guard), `docs/architecture/plugin/feature-gates.md`.
- `./le doc check all` (2026-11-19, at closure): FAILED with 13 problems, all YANG command summaries of other sessions' work (`request bgp rib source mark-stale` and `purge-stale`, `show errors`, `show interface errors`). None is in this spec's files, and the digest anchors all resolve.

### Deviations from Plan
- The total size bound is 128 MiB, not 40 MiB (A-4, broken as worded): the total is a disk figure for a local review artifact, and every chunk stays under 16 MiB.
- Ordinary devices record no sort and no side panel, where the Devices row once had panels for every device (A-4 narrowing; tour table updated).
- `TestFleetDemoStaysOutOfThePublishedSite` lives in package `site` (`unpublished_test.go`), not in `fleetdemo`, because it runs a real `Build`.
- AC-9: the live interface's side panel now opens as a fragment, so a table's sort and pager links keep the view they were drawn with and the next sort closes the panel. The design page documents it (`docs/architecture/fleet-interface.md`, `PanelLink` paragraph), and the owner approved the prototype as built after this change (`290951e355`).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4: the full tour fits tens of megabytes | The first render was about 9,000 requests and 54.7 MB for the estate chunk alone, because every navigation was recorded as the whole shell and the tour held cross products | The first render failed `chunkBytesMax` | The crawl sends the htmx request type; the tour was narrowed; the size report by kind of request was added (`sizeByKind`) |
| approach | A fix raised `totalBytesMax` in `tour.go` and left the guide at 40 MiB, with the figures only in this spec | A page that disagrees with the code is repaired in the same change (`ai/rules/documentation.md`) | Closure review | Guide and code comments repointed at closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The owner sees the fleet interface from the website checkout with no daemon | Done | `internal/le/site/fleetdemo/render.go` `render`; `../gh-pages/tmp/fleet-demo/index.html` | Rendered from `152ca49be0` |
| Nobody else can find it | Done | `fleetdemo/actions.go` `demoDirectory`; `host.go` `hostPage` (no-index, no navigation) | `TestFleetDemoStaysOutOfThePublishedSite` |
| Not live: rendered on request, never by the site build | Done | `fleetdemo/register.go`; no `registerProducer` call | Owner ruling at closure |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestFleetDemoRenderWritesTheDemonstration`, `TestFleetDemoReportsItsSizeByKind`; the action's report | 2,007 exchanges, 88,127,981 bytes |
| AC-2 | Done | `TestFleetDemoPlaysTheEstateDashboard`; `host.go` `hostBarText` | |
| AC-3 | Done | `TestFleetDemoTourCoversEveryRegisteredPage`, `TestFleetDemoOpensADeviceFromItsChunk`, `TestFleetDemoSortsAndOpensAPanel` | Narrowed tour (A-4) |
| AC-4 | Done | `TestFleetDemoAnswersAnUnrecordedRequestWithTheNotice` | |
| AC-5 | Done | same test, the bulk action's toast after its confirm dialog; `render.go` `demoFallbacks` | |
| AC-6 | Done | `TestFleetDemoStaysOutOfThePublishedSite`, `TestPlainFiguresSkipUnpublishedDirectories` | The search index assertion cannot go red alone: the render writes no Markdown mirror |
| AC-7 | Done | `TestFleetDemoRenderIsDeterministic` | Over the sampled tour; judge-approved in `cf08ace969` |
| AC-8 | Done | `TestRecordingOptionalFieldsRoundTrip`; `TestLGReplay*` browser tests unchanged | |
| AC-9 | Changed | fleetui package tests and the seven fleet browser tests | Panel deviation recorded above |
| AC-10 | Done | `cmd/ze/fleet_sample_absent_test.go` `TestShippedZeLinksNoFleetSample` | Run owed at closure |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The 13 unit tests of the TDD plan | Done | `fleetdemo/render_test.go`, `fleetdemo/tour_test.go`, `lgreplay/crawl_test.go`, `lgreplay/recording_test.go`, `site/unpublished_test.go`, `site/plaintext_test.go`, `fleetui/handler_shell_test.go`, `fleetui/sample/source_test.go` | Green in the main thread's scoped runs |
| The four `TestFleetDemo*` browser tests and `TestLGReplayAnswersTheTourFromTheRecording` | Done | `browsertest/fleetdemo_test.go`, `browsertest/lgreplay_test.go` | Green |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file of Files to Modify and Files to Create | Done | `fleetdemo/publish_test.go` became `site/unpublished_test.go` (Deviations); `sample/register.go` deleted, moved to `sample/register/register.go` |

### Audit Summary
- **Total items:** 30
- **Done:** 29
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-9, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The owner sees the fleet interface with no daemon | functional (browser) | `TestFleetDemoPlaysTheEstateDashboard`, `TestFleetDemoOpensADeviceFromItsChunk`, `TestFleetDemoSortsAndOpensAPanel` green; the render in `../gh-pages/tmp/fleet-demo/` from `152ca49be0` |
| Nobody else can find it | functional (real site build) | `TestFleetDemoStaysOutOfThePublishedSite` green over a real `Build`; `git -C ../gh-pages check-ignore -v tmp/fleet-demo/index.html` answers `.gitignore:1:tmp/` |
| The looking glass replay is unharmed | functional (browser) | `TestLGReplay*` and `TestReplayPlaysAProduct*` green unchanged; `website/data/lg-replay.json` unchanged |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every in-scope item is done. Publishing is out of scope by the owner's ruling (Known Limitations) | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fleet-ui-7-recorded-demonstration-15269d3e-0e9f-48a2-918c-3e6749565424.md` (5 files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | Round 1: wiring, security (host and frame messaging, chunk file names, write paths), removed behavior, documentation drift, style pass over the fleetdemo Go. Round 2: the closure edits only |

Round 1 covered the committed diff `186f9bd437..HEAD` of this spec's files, because the code is already committed. The pre-checks ran first. `./le repo check` found 5 issues, all number-parse allowlist rows outside this spec's files. `./le test weakened check base origin/main` was clean over 73 test files.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The guide stated `totalBytesMax` as 40 MiB and the tour as estimated at 3,000. `tour.go` declares 128 MiB, and the render measured 2,007. `boundFailure`'s fix line and five comments sent the reader to this spec's A-4, which closure deletes | `docs/contributing/gh-pages.md`, "Render the fleet demonstration"; `internal/le/site/fleetdemo/tour.go` `boundFailure`, bound comments, `panels`, `devicesTable`; `tour_test.go` `TestFleetDemoNarrowsOrdinaryDevices` | Guide: bounds and measured figures. Code: the comments and the fix line repointed at the guide. No regression test: the change is a message and its page, and no test asserts the fix line |

Run 2 (the closure edits): 0 BLOCKER, 0 ISSUE. Every remaining finding is a NOTE: a not-yet-bound `hx-get` link can navigate the frame within htmx's 1 ms settle delay (`lg-replay.js` click listener); the host page carries an inline `<style>` (`host.go` `hostPage`); stale chunk files survive a smaller render; and `follow` skips an unparsable page link silently, since `TestGalleryAddressesAreAnswered` holds the links.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/site/fleetdemo/` | Yes | `actions.go host.go register.go render.go render_absent.go render_test.go tour.go tour_test.go` |
| `internal/component/web/fleetui/sample/register/register.go` | Yes | ls: `register.go register_test.go`; `sample/register.go` absent |
| `internal/le/site/browsertest/fleetdemo_test.go` | Yes | ls: 27K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-7 | Writes only under `tmp/fleet-demo/`, deterministically | `./le test unit pkg path ./internal/le/site/fleetdemo/` green (main thread); `render.go` writes only `filepath.Join(directory, file.name)` from `demoDirectoryIn` |
| AC-2 to AC-5 | Plays, answers, notices, toasts | `./le test unit pkg path ./internal/le/site/browsertest/ run 'TestFleetDemo|TestLGReplay|TestReplayPlaysAProduct'` green (main thread) |
| AC-6 | Out of the published site | `./le test unit pkg path ./internal/le/site/ run 'TestPlainFiguresSkipUnpublishedDirectories|TestFleetDemoStaysOutOfThePublishedSite'` green (main thread) |
| AC-8 to AC-10 | Looking glass, live interface, no shipped sample | The LG browser tests green; `./le test unit pkg path ./internal/component/web/fleetui/` and the seven fleet browser tests green; `TestShippedZeLinksNoFleetSample` owed |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le site fleet-demo render` | No `.ci`: a le action, proven by `TestFleetDemoRenderWritesTheDemonstration` and the real run | `./le help site fleet-demo` lists `render [writes]`; `internal/le/register.go:139` imports the area |
| A browser opens the host page or a device | `browsertest/fleetdemo_test.go` (chromedp over the rendered files) | Read: `openFleetDemo` serves the render and opens `tmp/fleet-demo/index.html` |
| `./le site build` over an output holding it | `site/unpublished_test.go` | Read: a real `Build`, then the registry, sitemap, search, receipt and git walks |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The render in `../gh-pages/tmp/fleet-demo/`, ignored by git; the owner's closure ruling |
| A-2 | confirmed | `TestFleetDemoOpensADeviceFromItsChunk` (lazy branches through `htmx.ajax`) |
| A-3 | confirmed | `TestFleetDemoOpensADeviceFromItsChunk` (chunk by message) |
| A-4 | broken | Mistake Log row 1; the total bound restated with its reason |
| A-5 | confirmed | `TestFleetDemoPlaysTheEstateDashboard` asserts no frame error |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Bounds in `gh-pages.md` | `tour.go` `tourExchangesMax` 5,000, `chunkExchangesMax` 2,000, `chunkBytesMax` 16 MiB, `totalBytesMax` 128 MiB | Yes, at closure |
| Not published, on request only | `gh-pages.md`: "`./le site build` neither creates nor refreshes it, and nothing of it is published" | Yes |
| Rows 1-9, 11, 13-15, 17 (No) | No `ze` command, config, YANG, API or RFC change: the diff's product edits are `internal/component/web/fleetui` markup and scripts behind `ze_fleet_ui`, and `web/auth.go` `AccountMiddleware` | Yes |
