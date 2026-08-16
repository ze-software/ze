# Spec: web-htmx4-cutover

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Depends | spec-web-htmx4-prepare (closed 39e395afc) |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze serves htmx 2.0.4 to three interfaces. This spec makes htmx 4 the served
library and converts every site htmx 4 changed, with no compatibility shim.

The preparation spec landed everything that could ship while htmx 2 was still
served: htmx 4 vendored and served nowhere, the vendor drift check armed, 55
chaos golden fixtures, error responses converted to swappable fragments across
all three interfaces, and a generator deriving each page's asset set.

What remains cannot be split. The moment a page loads htmx 4, every unconverted
site breaks together, so this spec lands in ONE commit. Its phases are work
units, not separate landings.

Two of its changes are silent when wrong, and that shapes the whole test plan. A
named SSE event stops swapping and simply does nothing. A handler reading
`detail.xhr` returns early, and every error toast disappears with no console
error. Neither shows in a golden fixture, because a fixture captures markup and
these are runtime behaviours. Every phase therefore owes a browser proof, not
only a fixture diff.

## Required Reading

### Architecture Docs
- [ ] `tmp/session/2026-08-14-bd3cb1c5-21a8-4146-a390-5190a37971e8/scratch/htmx4-surface.md` - the site-level breaking-change map
  → Constraint: eight rows are mechanical and diff-verifiable. Thirteen need a decision per site. The judgement rows are the phase plan.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: a test asserting an ABSENCE passes when the mechanism is deleted. Every proof here needs a positive control, the way the preparation spec proved a 200 swapped before treating three non-swaps as evidence.
- [ ] `ai/rules/evidence.md` - read the producer
  → Decision: htmx 4 behaviour is settled from a fetched doc or the beta bytes. This session already got one mechanism backwards by reading minified source without checking behaviour.
- [ ] `ai/rules/no-layering.md` - replacing X with Y
  → Constraint: htmx 2's `htmx.min.js` and `sse.js` are DELETED in this spec, in every consumer and in `third_party/web/`. Both versions MUST NOT coexist after it lands.

**Key insights:**
- The server side is nearly free: Ze uses only `HX-Request`, `HX-Redirect` and `HX-Current-URL`, and all three survive htmx 4.
- No removed htmx JS API is used anywhere in Ze.
- Every `hx-swap` value Ze uses still exists in htmx 4.
- `HX-Request` is now load-bearing beyond htmx itself: `internal/core/errorfragment` gates the error-fragment conversion for all three interfaces on it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/lg/handler_ui.go` - `handleUIEvents` writes TWO named events on ONE stream: `peer-update` carrying the rendered table body, and `peer-error` carrying `engine unavailable`. Named events no longer swap in htmx 4, and an unnamed stream carries one kind of message, so the second event needs its own home.
- [ ] `internal/component/lg/peers.templ` - consumes that stream with `hx-ext`, `sse-connect` and `sse-swap`, the only three such attributes in lg.
- [ ] `internal/chaos/web/render.go`, `dashboard.go`, `viz.go`, `viz_convergence_trend.go`, `handlers.go` - 16 `sse-swap` sites, HTML built from Go string literals.
- [ ] `internal/component/web/assets/notification.js` - `handleResponseError` reads `evt.detail.xhr` and returns early when absent. htmx 4 uses fetch and provides no `xhr`.
- [ ] `internal/component/web/assets/cli.js` - listens for `htmx:oobAfterSwap` and guards on `e.detail.target.id`. htmx 4 folds that event into `htmx:after:swap`, so the listener starts firing for every swap.
- [ ] `internal/component/web/assets/log-live.js`, `internal/component/lg/assets/theme.js` - listen for `htmx:beforeSwap` and `htmx:afterSettle`, both renamed.
- [ ] `internal/component/web/page_assets.go`, `internal/component/lg/page_assets.go`, `internal/chaos/web/page_assets.go` - generated per-page asset sets naming the asset files, so the cutover changes what they resolve to.
- [ ] `scripts/codegen/web_assets.go` - the generator that writes them, and the attribute-to-asset mapping it applies.

**Behavior to preserve:**
- Every interface behaves as it does today from an operator's seat. The library changes; what the operator sees does not.
- The lg peer table still updates live. The chaos dashboard still updates live.
- A failed action still gives feedback in all three interfaces.
- Back and forward navigation still works on every URL that pushes one.

**Behavior to change:**
- htmx 4 is the served library. htmx 2 is deleted from the tree.
- Named SSE events become unnamed messages, so `hx-sse` swaps them.
- Out-of-band content swaps after the main content rather than before.
- The JS listeners use htmx 4's event names and its `ctx` object.

## Data Flow (MANDATORY)

### Entry Point
- An HTTP request to any page in web, lg or chaos, which now loads htmx 4.
- An SSE stream from lg's `handleUIEvents` or the chaos events route.
- A browser event: a swap, a settle, a request error.

### Transformation Path
1. A page's generated asset set resolves to the htmx 4 core and, where the page streams, `hx-sse`.
2. htmx 4 issues a request carrying `HX-Request`, which `internal/core/errorfragment` still reads.
3. A response swaps: main content first, then out-of-band elements in document order.
4. An SSE stream pushes an UNNAMED message, which `hx-sse` swaps into the subscribing element.
5. A failed request fires htmx 4's error event, whose `ctx.response` carries the status the handler wrote.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server ↔ browser | `HX-Request` on the way in, the error fragment on the way out | No |
| Server ↔ browser | SSE stream, now unnamed messages | No |
| Generated asset set ↔ served page | `page_assets.go` names the htmx 4 files | No |
| Browser event ↔ Ze JS | renamed events, `ctx` instead of `xhr` | No |

### Integration Points
- `internal/core/errorfragment` - gated on `HX-Request`, which A-1 must confirm htmx 4 still sends.
- `scripts/codegen/web_assets.go` - its attribute-to-asset mapping changes with the attribute names.
- `internal/test/golden` - `AssertPortFidelity` proves the mechanical rows, with the pre-cutover ref.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | htmx 4 still sends `HX-Request` with that exact spelling | `#createCoreHeaders` in the beta bytes writes it unconditionally, reached by every request through `#createRequestContext`, `htmx.ajax` included | would have reverted every error fragment in all three interfaces to a bare status line with no test failing | beta5 and beta6 bytes, both betas identical | **confirmed** |
| A-2 | `detail.target` still exists on the merged swap event | assumed from the htmx 2 shape | REAL AND LANDED: `htmx:after:swap` fires once per REQUEST on `ctx.sourceElement` with detail `{ctx}` only. `initErrorPanel` (`internal/component/web/assets/cli.js`) guards with `e.detail && e.detail.target`, so it does not throw. It silently never calls `syncErrorPanel` and the error panel stops syncing | jsdom probe against both betas | **BROKEN, and CLOSED in phase 4**. `initErrorPanel` now listens for `htmx:after:settle` and reads `e.target.id`, and its comment is rewritten. Measured in a browser: `htmx:after:settle` fires once per swap task with the target as the event's own target (`toast-container`, `stats`, `events` in one chaos capture), while `htmx:after:swap` fires once per request on the element that issued it |
| A-3 | `htmx.ajax` keeps its target, swap and values options object | the migration guide lists the function but not its options | three call sites would break | `ajax()` does `Object.assign(ctx, context)`; probe confirmed `values` reach a GET as query params and an element-valued `target` resolves | **confirmed**, no change needed |
| A-4 | `hx-swap-oob="delete"` is accepted | `delete` is a listed swap style | one chaos site would stop removing its element | probed on both a `div` and a `tr` | **confirmed**, with a trap beside it: a bare `tr` following any non-table element is dropped by the HTML parser before htmx sees it. `renderPeerRemoval` (`internal/chaos/web/dashboard.go`) emits its `tr` alone as the whole payload, so it survives, and the port MUST keep it first in its response |
| A-5 | An `hx-sse` stream is not subject to the new `defaultTimeout` of 60000 | htmx 4 uses fetch and the config default changed from 0 | both dashboards would go stale after a minute of an idle stream | `hx-sse` returns false from its before-response hook for `text/event-stream`, so the request path returns before the timer and the `finally` clears it. Probed at a 250ms timeout: the stream ran 1400ms over 34 reads and never aborted | **confirmed**, no action |
| A-6 | No `hx-get` in Ze relies on implicit enclosing-form inclusion | htmx 4 stops including enclosing form inputs for GET and DELETE | a request would silently lose its parameters | parsed all 223 golden HTML files: 238 request-issuing elements, 2 with a form ancestor, both `hx-post` carrying `hx-include`. No `hx-get` sits in a form, chaos renders no form at all, and `hx-delete` exists nowhere | **confirmed**, no action |
| A-7 | The beta the cutover ships on matches the migration guide the surface map was built from | the map cites the htmx 4 migration guide; beta5 was vendored | a breaking change would be missed because nothing in the map describes it | diffed beta5 against beta6 | **BROKEN in our favour, and CLOSED in phase 2**. The published docs serve BETA6 and are inaccurate for beta5 on the event name and on `hx-method`. The map was therefore built from beta6 docs against beta5 bytes. The tree now ships BETA6: none of its five changes touches a map row with a Ze site, and it costs 99 brotli bytes. `third_party/web/htmx/{htmx.min.js,hx-sse.min.js}` are `htmx.org@4.0.0-beta6`, fetched from unpkg and compared byte for byte against jsdelivr |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A named SSE event silently stops swapping and a dashboard freezes with no error | A live stream test sees no DOM change after an event | Every stream gets a browser proof asserting a POSITIVE change, never the absence of an error |
| R-2 | The out-of-band ordering reversal changes what lands in a target, and a fixture cannot see it because both orders produce the same markup | The `#error-list` sync in `cli.js` fires at a different time | **JUDGED phase 4, no site depended on the old order.** An order dependence needs an out-of-band target INSIDE the main target of the same response. In web every existing target (`#breadcrumb`, `#notification-bar`, `#notification-area`, `#commit-bar`, `#error-list`, `#tool-overlays`) is a sibling of `#content-area` or of `#workbench-workspace`; `#finder` sits inside the main split but the response carrying `finderOOB` targets `#detail` beside it (`hx-target="#detail"` on the add form). In chaos every out-of-band element arrives on the stream, one element per message, so there is no main content to order against. lg has none. The error panel was the one live dependence and it was the event, not the order |
| R-3 | The library swap lands but one interface was never opened in a browser | A fixture-only proof | **CLOSED phase 5**: all three interfaces have a browser proof of the error path. lg needed no handler, chaos needed one: a refusal there replaced the peer table's body, or the control that asked, depending on which view was open |
| R-4 | Deleting htmx 2 leaves a page pointing at a file that no longer exists | The asset-resolution check | A missing asset must fail a test, never 404 in a browser |
| R-5 | The pushed URLs must now answer a full-page GET, and some answer only a fragment | Back navigation renders a fragment with no layout | **MEASURED FALSE, phase 1.** The finder renders 61 pushed URLs (the workbench renders none), and every one answers a complete page on a direct GET. `test/web/history-full-page.wb` therefore PASSES before the cutover and is a regression guard for phase 6, not a red to clear. `TestPushedURLsAnswerAFullPage` still owes the whole population |
| R-6 | The cutover is one commit and it is large, so a review pass loses scope | The Review Gate reports on the diff rather than on the change | Phase agents each report what they changed; the Review Gate reads the per-spec state file first |
| R-7 | `hx-sse` defaults `reconnect` and `pauseOnBackground` to true, so both dashboards drop and re-open their stream on every tab switch | a stream reconnect logged on focus change | **DECIDED phase 3: keep both defaults, override neither.** Confirmed in the beta6 extension bytes: both default to true when the element carries `hx-sse:connect`. A background tab releases its connection, which is what `maxSSEClients` (100, `internal/component/lg/server.go`) exists to protect, and a dropped reader takes the pause branch rather than the error branch, so no "Server disconnected" banner appears on a tab switch. The cost is bounded: on return lg is stale for at most one 5s tick, and chaos keeps `#stats` and `#events` fresh through their 500ms polls |
| R-8 | An out-of-band-only response blanks the main target unless `swapEmpty` is false | a target empties when a response carried only out-of-band content | `hx-sse` sets it automatically; a non-SSE out-of-band-only response does not, so each of the 21 sites must be checked for whether it ever answers out-of-band alone |
| R-9 | beta6 replaces `popstate` with the Navigation API, and Ze pushes 12 URLs | back or forward behaves differently in a real browser | **CLOSED phase 6, and the count was wrong: the finder pushes 336 URLs in the default tree.** Both traversals were driven in a real browser under beta6: back from a pushed `/show/bgp/` landed on `/show/` as a whole page (the head loads the library, 9577 bytes of body), and forward returned to `/show/bgp/` as a whole page carrying the bgp detail. `test/web/history-full-page.wb` holds both, with `action=forward` added to the runner for the direction `back` cannot reach |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Any or all of the three web interfaces stop updating, stop reporting errors, or stop navigating. Nothing touches the daemon, the routing table, or any peer: this is presentation only. |
| How is it reverted? | Single commit revert. htmx 2 is deleted in the same commit that serves htmx 4, so the revert restores both. |
| Who else touches this path? | Several sessions share this checkout. `internal/component/web` is edited frequently, and the vendored assets are now gated by `ze-check-vendor-web`. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| HTTP GET of any page | → | the generated asset set resolving to htmx 4 | `TestPagesServeHtmx4` |
| lg SSE stream emits an event | → | `hx-sse` swapping an unnamed message | `test/web/lg-stream-swaps.wb` |
| chaos SSE stream emits an event | → | `hx-sse` swapping an unnamed message | `test/web/chaos-stream-swaps.wb` |
| A request fails with 4xx | → | htmx 4's error event reaching `notification.js` | `test/web/web-error-toast.wb` |
| A response carries out-of-band content | → | main content swapping first, then out-of-band | `test/web/oob-order.wb` |
| Browser back after a pushed URL | → | the URL answering a full-page GET | `test/web/history-full-page.wb` |

All six exist after phase 1. Five are RED for the reason this spec names, and each
one's red is preceded by a passing positive control, so a red says "the cutover has
not happened" rather than "nothing ran".

| Test | State after phase 1 | The step that is red |
|------|--------------------|----------------------|
| `TestPagesServeHtmx4` | red | every derived asset set resolves to bytes carrying `version:"2.0.4"`, and lg and chaos name `sse.js` |
| `lg-stream-swaps.wb` | red | the peers page head loads `sse.js`. The controls above it pass: the session is `established` and the table holds its row |
| `chaos-stream-swaps.wb` | red | the dashboard head loads `sse.js`. `#toast-container` is found first |
| `web-error-toast.wb` | red | `error-fragment-message` never enters the DOM, because htmx 2 swaps no 4xx. The toast assertions above it PASS, which is the behaviour phase 5 must not lose |
| `oob-order.wb` | red | `class="error-item"` never enters `#error-list`, same cause. The 200 main-plus-out-of-band control passes |
| `history-full-page.wb` | GREEN | nothing. See R-5: the behaviour already holds, so this file guards it through phases 2 and 6 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any page in web, lg or chaos is served | It loads the htmx 4 core, and `hx-sse` only if it streams. No page loads htmx 2 |
| AC-2 | The tree is searched for htmx 2 | Its core and its SSE extension are absent from `third_party/web/` and from every consumer. Both versions MUST NOT coexist |
| AC-3 | lg's peer stream emits an update | The peer table changes in a real browser. The assertion is a positive DOM change, never the absence of an error. **GREEN phase 3**: `test/web/lg-stream-swaps.wb`, 83.8s, uptime crossing a minute |
| AC-4 | The chaos dashboard streams an update | The corresponding panel changes in a real browser. **GREEN phase 3**: `test/web/chaos-stream-swaps.wb` on `#toast-container`, the one target no poll fills |
| AC-5 | A request is answered 4xx with an error fragment | The operator sees the message in all three interfaces, proven in a browser. **GREEN phase 5**: web `test/web/web-error-toast.wb` (toast and fragment), lg `test/web/lg-error-fragment.wb` (the fragment lands in `#results`), chaos `test/web/chaos-error-fragment.wb` (the fragment lands in the new `#action-error`, and the peer table and the Add control survive it) |
| AC-6 | A response carries main content and out-of-band content | Main content lands first, out-of-band after, and the error panel still syncs. **GREEN phase 4**: `test/web/oob-order.wb`, red at its last step until `initErrorPanel` (`internal/component/web/assets/cli.js`) moved to `htmx:after:settle` and `e.target.id` |
| AC-7 | Every URL the finder pushes is fetched directly as a GET | Each answers a complete page, not a fragment, and both browser traversals land on one. **GREEN phase 6**: `TestPushedURLsAnswerAFullPage` (`internal/component/web/handler_history_test.go`) crawls from `/show/` and holds the DERIVED population, 336 pushed URLs over 337 pages of the default tree, not the 12 this row first claimed nor the 61 phase 1 measured. `test/web/history-full-page.wb` drives back AND forward in a real browser, which R-9 owed |
| AC-8 | An SSE stream idles for more than 60 seconds | It stays open and the next event still swaps. **GREEN phase 3**: the same 70-second wait in `lg-stream-swaps.wb` proves a stream older than htmx 4's 60000ms `defaultTimeout` still swaps |
| AC-9 | A tagged htmx event fires | Ze's listeners use htmx 4's names, and none listens for a htmx 2 name. **GREEN phase 5**: `TestNoListenerUsesAHtmx2EventName`, and the vendored scanner reports 0 `[old-event]` rows, down from 8 |
| AC-10 | The mechanical rows are compared against the pre-cutover ref | `AssertPortFidelity` reports only intended changes |
| AC-11 | A page's asset set names a file that does not exist | A test fails, rather than the browser 404ing |
| AC-12 | lg's engine becomes unavailable while a browser watches | The operator is still told, even though `peer-error` can no longer share one unnamed stream with `peer-update`. **GREEN phase 3**: the reason travels IN the table body as its only row (`peersStreamError`), from `engineError`, the producer the page banner already read. `test/web/lg-stream-error.wb` against `ze-test lg` |
| AC-13 | htmx's own `upgrade-check` scanner is run over the tree | It reports zero issues, or every remaining issue is listed with the reason it does not apply. On the pre-cutover tree it reports 206 issues across 39 of 610 files, and it finds inheritance carriers our own scan missed. Phase 1 vendored the scanner at `third_party/web/htmx-upgrade-check.py` and armed `make ze-htmx-upgrade-check`; it reproduced 206/39/610 and is red. Phase 2 UNWIRED it from both `ze-verify` modes: a gate that is red by construction for the length of this spec must not sit in the gate every session of this shared checkout runs. It stays runnable standalone, with its tests and its explained list. **GREEN phase 6**: the scanner reports 0 unexplained issues over 607 files, its last 35 rows are all `[inheritance]` and sit in `scripts/dev/htmx-upgrade-explained.txt` as 16 rows (the gate keys on file and category), and the gate is back in `stagesForMode` and in both golden lists |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Watches the looking glass peer table update | `handleUIEvents` emits an unnamed message, `hx-sse` swaps it into the table body | `test/web/lg-stream-swaps.wb` |
| 2 | Watches a chaos dashboard panel update | chaos stream emits an unnamed message, `hx-sse` swaps it | `test/web/chaos-stream-swaps.wb` |
| 3 | Submits a config change that fails validation | handler answers 4xx with a fragment, htmx 4 swaps it, the toast reports it | `test/web/web-error-toast.wb` |
| 4 | Navigates back after following a config path | the pushed URL answers a full page | `test/web/history-full-page.wb` |
| 5 | Watches the looking glass while the engine is down | the error reaches the browser by whatever route replaces the second named event | `test/web/lg-stream-error.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPagesServeHtmx4` | `internal/component/web/markup_contract_test.go` | AC-1 | written, red |
| `TestHtmx2IsGone` | `scripts/vendor/check_web_test.go` | AC-2 | |
| `TestNoListenerUsesAHtmx2EventName` | `internal/component/web/markup_contract_test.go` | AC-9 | |
| `TestEveryPageAssetResolves` | `internal/component/web/markup_contract_test.go` | AC-11 | |
| `TestPushedURLsAnswerAFullPage` | `internal/component/web/handler_history_test.go` | AC-7 | written, GREEN (phase 6) |
| `TestFreezeStopsThePollsItOwns` | `internal/chaos/web/render_test.go` | the Freeze control, which htmx 4's loss of trigger filters killed | written, GREEN (phase 6) |
| `TestFreezeListenerReadsTheMarker` | `internal/chaos/web/render_test.go` | the same control's two halves | written, GREEN (phase 6) |
| `TestChaosPortFidelity` | `internal/chaos/web/golden_test.go` | AC-10 | |
| `TestEngineUnavailableReachesTheBrowser` | `internal/component/lg/handler_ui_test.go` | AC-12 | written, GREEN (phase 3) |
| `TestMergedErrorEventIsToldApart` | `internal/component/web/handler_error_test.go` | AC-5 | written, GREEN (phase 5) |
| `TestErrorDrawerWiringHoldsTogether` | `internal/component/web/markup_contract_test.go` | AC-6 | updated to the htmx 4 event, GREEN (phase 4) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SSE idle seconds before the next event | 0 to unbounded | 60 and beyond must survive | N-A | N-A: A-5 decides whether a bound exists at all |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lg-stream-swaps` | `test/web/*.wb` | the peer table updates from a live stream | written, red |
| `lg-stream-error` | `test/web/*.wb` | the engine going down still reaches the operator | written, GREEN (phase 3) |
| `chaos-stream-swaps` | `test/web/*.wb` | a chaos panel updates from a live stream | written, red |
| `web-error-toast` | `test/web/*.wb` | a failed action reports its message | written, GREEN (phase 2) |
| `oob-order` | `test/web/*.wb` | main content lands, then out-of-band | written, GREEN (phase 4) |
| `lg-error-fragment` | `test/web/*.wb` | a refused search reports its message in the looking glass | written, GREEN (phase 5) |
| `chaos-error-fragment` | `test/web/*.wb` | a refused action reports its message on the chaos dashboard, and destroys nothing | written, GREEN (phase 5) |
| `history-full-page` | `test/web/*.wb` | back AND forward navigation render a whole page | written, GREEN (phase 6, extended to forward) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A: no protocol behavior changes | - | - | - | - |

## Files to Modify
- `internal/component/lg/handler_ui.go` - `handleUIEvents` emits unnamed messages; the engine-unavailable case needs its own route
- `internal/component/lg/peers.templ` - drop `hx-ext`, rename `sse-connect`, drop `sse-swap`
- `internal/chaos/web/render.go`, `dashboard.go`, `viz.go`, `viz_convergence_trend.go`, `handlers.go` - the 16 chaos SSE sites
- `internal/component/web/assets/notification.js` - the error event's `ctx.response` instead of `detail.xhr`
- `internal/component/web/assets/cli.js` - the folded swap event and its error-list guard
- `internal/component/web/assets/log-live.js`, `internal/component/lg/assets/theme.js` - renamed events
- the 21 out-of-band sites - judged for ordering (phase 4: none depended on the old order, see R-2)
- `internal/chaos/web/render.go`, `internal/chaos/web/assets/style.css` - `#action-error`, the region a refusal lands in, and the listeners that tell a refusal from a lost server
- the pushed-URL sites and their handlers - each must answer a full-page GET (phase 6 measured 336, and every one already did)
- `internal/chaos/web/viz.go`, `viz_matrix.go`, `viz_timeline.go`, `viz_panels.go` - the eight polls the Freeze control owns, which htmx 4 stopped freezing
- `internal/chaos/web/render.go` - `freezePoll` and the listener that cancels a marked poll
- `internal/component/web/testing/runner.go` - `Forward()` and `action=forward`, the traversal `back` cannot make
- `scripts/dev/htmx-upgrade-explained.txt`, `scripts/status/verify_run.go` - the 16 explained rows and the re-wired gate
- `scripts/codegen/web_assets.go` - the attribute-to-asset mapping follows the renamed attributes
- `third_party/web/MANIFEST.md`, `scripts/vendor/sync_web.go` - htmx 2 removed, htmx 4 takes the served name

## Files to Create
- `test/web/lg-stream-swaps.wb`, `lg-stream-error.wb`, `chaos-stream-swaps.wb`, `web-error-toast.wb`, `oob-order.wb`, `history-full-page.wb`
- `test/web/lg-error-fragment.wb`, `test/web/chaos-error-fragment.wb` - AC-5 in the two interfaces that had never been opened on their error path
- `internal/component/web/handler_history_test.go`
- `third_party/web/htmx-upgrade-check.py` - htmx's own scanner, vendored (AC-13)
- `scripts/dev/htmx_upgrade_check.py` and its explained list - the AC-13 gate

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config option added |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | No command added |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | Yes | The six browser tests above |
| Pipe completeness | N-A | No CLI output produced |
| Env var registration | N-A | No env var added |
| Doctor check for runtime dependencies | N-A | The assets are embedded, not read from the host |
| Prometheus counters/metrics | N-A | No observable runtime state added |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The interfaces behave as before; the library beneath changes |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No command touched |
| 4 | API/RPC added/changed? | Yes | The lg SSE stream's shape changes, so any doc describing its events |
| 5 | Plugin added/changed? | No | No plugin touched |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` |
| 7 | Wire format changed? | No | No wire format touched |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs this |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for the browser-proof tests |
| 11 | Affects daemon comparison? | No | No comparable feature changes |
| 12 | Internal architecture changed? | Yes | The architecture docs the preparation spec added, for the asset and streaming path |
| 13 | Route metadata keys added/changed? | No | No route metadata touched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on every file in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify the web-interface guide against the converted interfaces |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- every entry point exists and fails
   - Tests: all six Wiring Test rows, each failing for the right reason
   - Files: the six test files
   - Verify: each names a real symbol or route and fails because the cutover has not happened
2. **Phase: Cut over and rename** -- AC-1, AC-2, AC-9, AC-10, AC-11
   - Tests: `TestPagesServeHtmx4`, `TestHtmx2IsGone`, `TestNoListenerUsesAHtmx2EventName`, `TestEveryPageAssetResolves`, `TestChaosPortFidelity`
   - Files: the vendoring, the generator's mapping, `hx-ext`, `sse-connect`, the eight event strings
   - Verify: fixtures move only where intended. Every later phase runs against htmx 4 from here
3. **Phase: SSE emits unnamed messages** -- AC-3, AC-4, AC-8, AC-12
   - Tests: `lg-stream-swaps.wb`, `lg-stream-error.wb`, `chaos-stream-swaps.wb`, `TestEngineUnavailableReachesTheBrowser`
   - Files: `handler_ui.go`, `peers.templ`, the 16 chaos sites
   - Verify: a positive DOM change in a real browser, a stream surviving 60 seconds idle, and the engine-down path still reaching the operator
4. **Phase: Out-of-band ordering** -- AC-6
   - Tests: `oob-order.wb`
   - Files: the 21 out-of-band sites, the error-list guard in `cli.js`
   - Verify: judge each site for order dependence; prove the error panel still syncs
5. **Phase: Error and event surface** -- AC-5
   - Tests: `web-error-toast.wb`
   - Files: `notification.js`, `cli.js`, `log-live.js`, `theme.js`
   - Verify: all three interfaces show a failed action's message in a browser. lg and chaos have never been exercised here
6. **Phase: History and inheritance** -- AC-7, AC-13
   - Tests: `TestPushedURLsAnswerAFullPage`, `history-full-page.wb`, `TestFreezeStopsThePollsItOwns`, `TestFreezeListenerReadsTheMarker`
   - Files: `internal/component/web/handler_history_test.go`, the runner's `forward` action, the explained list, `scripts/status/verify_run.go`, and the eight chaos polls the Freeze control owns
   - Verify: each pushed URL answers a whole page as a direct GET, both browser traversals land on one, and the scanner reports no unexplained issue
   - The 35 `[inheritance]` rows are 6 distinct carrier-to-descendant relationships over 271 rendered fixtures. NONE was load-bearing: every relying descendant either carries its own copy of the attribute, or asks a handler that never read what the ancestor included. Each was measured in a browser rather than reasoned about, and each holds a row in `scripts/dev/htmx-upgrade-explained.txt`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Every stream proof asserts a POSITIVE change. An assertion that no error appeared passes when the mechanism is deleted |
| Correctness | Every browser proof has a control that would fail if the mechanism were removed |
| Naming | No Ze source names a htmx 2 event, attribute, or asset file |
| Data flow | `HX-Request` still reaches `internal/core/errorfragment` in all three interfaces |
| Rule: `ai/rules/no-layering.md` | htmx 2 is DELETED, not left beside htmx 4 |
| Rule: `ai/rules/evidence.md` | Every htmx 4 behavioural claim cites a fetched doc or the beta bytes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| htmx 4 is served everywhere | grep the rendered fixtures for the htmx 4 filename on every page that needs it |
| htmx 2 is gone | search the tree for its core and its SSE extension and find nothing outside history |
| Every interface opened in a browser | a recorded pass for web, lg and chaos |
| Streams survive an idle minute | a stream test that waits, then asserts a swap |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error fragment content | The conversion is gated on `HX-Request`. Confirm A-1, or every fragment reverts to a bare status line silently |
| Escaping | The renamed attributes carry the same operator-supplied values; escaping MUST NOT regress |
| CSP | htmx 4 MUST NOT require `unsafe-eval`. Ze removed its last eval-dependent trigger this session and MUST NOT reintroduce one |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A browser proof passes but the feature is dead | The proof lacked a control. Add one before trusting any result from that file |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

**htmx 4 has no trigger filter, and no row of the surface map said so.** The
chaos dashboard's Freeze control was written as `hx-trigger="every 500ms
[!window._frozen]"`. htmx 4 parses the interval and ignores the condition, so
after the cutover the box changed nothing: 15 poll requests in 3 seconds with it
ticked, measured in a browser. Nothing was red, because a golden fixture holds
markup and this is a runtime behaviour, which is the failure shape the whole spec
was written around. It was found by trying to hold a panel still long enough to
read one, not by a check. The fix is a marker attribute (`freezePoll`) and one
listener canceling `htmx:before:request`, and `TestFreezeStopsThePollsItOwns`
makes a new poll a decision rather than a default.

**The scanner's inheritance rows are a reading list, not a defect list.**
`check_inheritance` flags an ancestor whenever ANY descendant issues a request,
whether or not that descendant carries its own copy of the attribute. Over 271
rendered fixtures the 35 rows reduce to 6 carrier-to-descendant relationships,
and every one of them survives the loss: the descendant carries its own copy, or
its handler never read what the ancestor included (`handlePeerDetail` reads a
path value alone; `handleVizRouteMatrixCell` reads src and dst alone), or the
inherited value equals htmx 4's default (`defaultSwap` is `innerHTML`). One
carrier is better off: a field POST inside a monitored panel no longer inherits
`hx-select=".main-split"`, which its `.ze-field` answer never contained.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One commit for the whole cutover | Phase-by-phase commits | The library swap cannot half-land. Once a page loads htmx 4, every unconverted site breaks together |
| Cut the library over EARLY, in phase 2 | Convert everything first, swap last | Later phases must measure the real target. Converting against htmx 2 and swapping last would leave every judgement row unproven |
| Named SSE events become unnamed messages | Hand-written JS per stream; a re-fetch triggered by the named event | Owner decision. The alternatives cost hand-written JS in every streaming package, or turn a push of ready HTML into push-then-pull |
| Every phase owes a browser proof | Golden fixtures alone | Two of the changes are silent when wrong. A fixture captures markup, and a stream that stopped swapping produces correct markup that never updates |
| htmx 2 is deleted in the same commit | Keep it until the cutover is proven | `ai/rules/no-layering.md`. Two versions in the tree is the state where a page silently loads the wrong one |

## Known Limitations
- The seven A-N rows are unvalidated at the time of writing and are being settled against the beta bytes. Any that break may change a phase.
- lg and chaos have no error-event handler today, so their error fragments have never been exercised in a browser. Phase 5 is their first exposure.
- The chaos fixtures gain port fidelity only in this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
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
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

- htmx 4.0.0-beta6 replaces htmx 2.0.4 in all three interfaces, under the name
  every page already loaded (`third_party/web/htmx/htmx.min.js`). htmx 2's core
  and its separately published `sse.js` are deleted from `third_party/web/` and
  from all three `assets/` directories. No shim.
- The SSE extension is htmx 4's `hx-sse.min.js`, and `hx-sse:connect` replaces
  `hx-ext` plus `sse-connect`. `scripts/codegen/web_assets.go` and
  `internal/test/markupcheck/head.go` follow the renamed attributes, and
  `loadOrder` now puts the core first as a rule rather than as a property of two
  names.
- Both streams send UNNAMED messages: `handleUIEvents` and `peerStreamBody`
  (`internal/component/lg/handler_ui.go`), and every chaos `Broadcast`
  (`internal/chaos/web/dashboard.go`, `sse.go`). lg's engine-unavailable signal
  travels IN the table body as its only row (`peersStreamError`), because one
  unnamed stream carries one kind of message. `sse-swap` is gone from the tree.
- Five event names renamed and two folded events rewritten against `ctx`:
  `handleResponseError` and the new `handleRequestError`
  (`internal/component/web/assets/notification.js`), the `.wb-form-post` reload
  (`assets/cli.js`), and the chaos connection banner (`render.go`).
- `initErrorPanel` (`assets/cli.js`) moved to `htmx:after:settle` and
  `e.target.id`, which is AC-6's whole fix.
- chaos gained `#action-error`: `htmx:response:error` retargets the refusal
  there instead of letting it replace the peer table or the control that asked.
- Inheritance judged over 271 rendered fixtures. No `:inherited` was added
  anywhere; the 35 scanner rows hold 16 explained rows in
  `scripts/dev/htmx-upgrade-explained.txt`.
- AC-13's gate: htmx's own scanner vendored at
  `third_party/web/htmx-upgrade-check.py`, run by
  `scripts/dev/htmx_upgrade_check.py`, wired as `make ze-htmx-upgrade-check`
  into both `stagesForMode` lists (`scripts/status/verify_run.go`).
- Eight `.wb` browser proofs, `TestPushedURLsAnswerAFullPage`, and the runner
  gained `action=back`, `action=forward` and `option=server:kind=`.

### Bugs Found/Fixed

- **The chaos Freeze control was dead.** htmx 4 has no trigger filter, so
  `hx-trigger="every 500ms [!window._frozen]"` parsed the interval and ignored
  the condition: measured at 15 polls in 3 seconds with the box ticked. This
  spec introduced it at the phase 2 cutover and repaired it in phase 6 with
  `freezePoll` plus one `htmx:before:request` listener
  (`internal/chaos/web/render.go`). Covered by `TestFreezeStopsThePollsItOwns`
  and `TestFreezeListenerReadsTheMarker` (`internal/chaos/web/render_test.go`).
- **`expect=url:contains=` could only ever fail.** `checkURL`
  (`internal/component/web/testing/expect.go`) read the accessibility snapshot,
  which never carries the address. No `.wb` used it, so nothing was red. It now
  reads the address bar through `Browser.getURL`.
- **A long `action=wait:ms=` destroyed the page.** `waitMs` was a bare sleep and
  the agent-browser daemon reaps itself after its idle window, so every
  assertion after a 70-second wait read an empty page. `waitMs` now touches the
  daemon every half-window, and `browserIdleWindow` is the single source both it
  and `agentEnv` read.
- **lg reported only a nil engine response, as a named event nothing consumed.**
  `peerStreamBody` now reads `engineError`, so a dispatch error reaches a
  watching operator too.
- **`test/web/web-error-fragment.wb` asserted htmx 2's contract.** Its last two
  lines were `not-contains`; htmx 4 swaps a 4xx, so both are `contains` now.

### Documentation Updates

- `docs/guide/looking-glass.md` -- the embedded library and the extension the
  peers page loads.
- `docs/guide/web-interface.md` -- the two toast kinds, anchored on
  `notification.js -- handleResponseError and handleRequestError`.
- `docs/architecture/chaos-web-dashboard.md` -- the SSE section rewritten
  against the code (its old table named events that no longer existed), the two
  error regions, and a Freeze Control section.
- `docs/architecture/testing/runner-architecture.md`, `docs/functional-tests.md`
  -- `option=server:kind=`, `option=env:`, `action=back`, `action=forward`, and
  the corrected `url` row.
- `ai/INDEX.md` -- the gate row in Dev Tools. `ai/digests/web.md`,
  `ai/patterns/web-endpoint.md` -- the stream shape and the vendored asset names.
- `make ze-doc-test` and `make ze-verify-wiring-docs`: both exit 0 (2026-08-16).

### Deviations from Plan

- The gate was UNWIRED from both verify modes in phase 2 and re-wired in phase
  6. A gate red by construction for the length of a spec must not sit in the
  gate every session of a shared checkout runs.
- Phase 6 gained work no phase named: the Freeze repair. It is a regression this
  spec introduced, so it was fixed here rather than homed elsewhere.
- AC-7's population is 336 pushed URLs, not the 12 the AC first claimed nor the
  61 phase 1 measured. `TestPushedURLsAnswerAFullPage` derives it by crawling.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2: `detail.target` still exists on the merged swap event | `htmx:after:swap` fires once per REQUEST on `ctx.sourceElement` with `{ctx}` only. `htmx:after:settle` is the per-task event, dispatched ON the target | jsdom probe against both betas, then measured in a browser | `initErrorPanel` moved to `htmx:after:settle` and `e.target.id` (phase 4) |
| assumption | A-7: the beta the cutover ships matches the migration guide the surface map was built from | The published docs serve beta6 and are inaccurate for beta5. The map was built from beta6 docs against beta5 bytes | diffed beta5 against beta6 | The tree ships beta6. None of its five changes touches a map row with a Ze site (phase 2) |
| approach | The surface map's fixture-parsing scan sized the inheritance work at what phase 6's line suggested | The scanner sees chaos, which that scan could not: 35 rows over 8 more files | `make ze-htmx-upgrade-report` after the cutover | Phase 6 judged all 35 over 271 fixtures rather than the map's subset |
| escalation | A library upgrade removed a feature and every gate stayed green | htmx 4 has no trigger filter, and a golden fixture holds markup while this is a runtime behaviour | trying to hold a viz panel still long enough to read it | Fixed, plus a row in `plan/journal/refactor-removes-feature.md` |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| htmx 4 is the served library in all three interfaces | Done | `third_party/web/htmx/htmx.min.js`, `scripts/vendor/sync_web.go` | `TestPagesServeHtmx4` reads the served BYTES, not the name |
| Every site htmx 4 changed is converted, with no compatibility shim | Done | the 113-file diff | `make ze-htmx-upgrade-check` exits 0 |
| htmx 2 is deleted in the same commit | Done | `scripts/vendor/check_web.go` -- `TestHtmx2IsGone` | 8 deletions, `third_party/` and three consumers |
| The spec lands in ONE commit | Done | commit A | The library swap cannot half-land |
| Every phase owes a browser proof, not a fixture diff | Done | 8 `.wb` files | Two of the changes are silent when wrong |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPagesServeHtmx4` (`internal/component/web/markup_contract_test.go`) | Reads the bytes of each derived asset set; only the two streaming pages name the extension |
| AC-2 | Done | `TestHtmx2IsGone` (`scripts/vendor/check_web_test.go`) | Probed: a `version:"2.0.4"` file named `sse.js` in a consumer reds it |
| AC-3 | Done | `test/web/lg-stream-swaps.wb` | 83.8s; uptime crosses a minute in a real browser and nothing polls that table |
| AC-4 | Done | `test/web/chaos-stream-swaps.wb` | `#toast-container` is the one SSE-only target |
| AC-5 | Done | `web-error-toast.wb`, `lg-error-fragment.wb`, `chaos-error-fragment.wb` | All three assert the NESTING of the fragment, not only its words |
| AC-6 | Done | `test/web/oob-order.wb` | Red at its last step before the `cli.js` change, green after, nothing else touched |
| AC-7 | Done | `TestPushedURLsAnswerAFullPage`, `test/web/history-full-page.wb` | 336 URLs over 337 pages; an `HX-Request` header reds it 674 times |
| AC-8 | Done | the 70-second wait in `lg-stream-swaps.wb` | A stream older than htmx 4's 60000ms `defaultTimeout` still swaps |
| AC-9 | Done | `TestNoListenerUsesAHtmx2EventName` | 0 `[old-event]` scanner rows, down from 8 |
| AC-10 | Done | `TestChaosPortFidelity` (`internal/chaos/web/golden_test.go`) | Against `ca8df922c`, plus the web and lg port-fidelity tests |
| AC-11 | Done | `TestEveryPageAssetResolves` | Over all three surfaces, plus the per-surface served-filesystem checks |
| AC-12 | Done | `test/web/lg-stream-error.wb`, `TestEngineUnavailableReachesTheBrowser` | `<tr class="stream-error">` enters a tbody that had none |
| AC-13 | Done | `make ze-htmx-upgrade-check` | 0 unexplained over 607 files, 16 explained rows, a stage of both verify modes |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPagesServeHtmx4` | Done | `internal/component/web/markup_contract_test.go` | |
| `TestHtmx2IsGone` | Done | `scripts/vendor/check_web_test.go` | |
| `TestNoListenerUsesAHtmx2EventName` | Done | `internal/component/web/markup_contract_test.go` | Reads the vendored bytes for the events htmx 4 dispatches |
| `TestEveryPageAssetResolves` | Done | `internal/component/web/markup_contract_test.go` | |
| `TestPushedURLsAnswerAFullPage` | Done | `internal/component/web/handler_history_test.go` | `pushedURLFloor` refuses a vacuous pass |
| `TestFreezeStopsThePollsItOwns` | Done | `internal/chaos/web/render_test.go` | |
| `TestFreezeListenerReadsTheMarker` | Done | `internal/chaos/web/render_test.go` | Probed at closure: deleting the listener reds it 4 ways |
| `TestChaosPortFidelity` | Done | `internal/chaos/web/golden_test.go` | |
| `TestEngineUnavailableReachesTheBrowser` | Done | `internal/component/lg/handler_ui_test.go` | Fails on an `event:` line |
| `TestMergedErrorEventIsToldApart` | Done | `internal/component/web/handler_error_test.go` | Source-text assertions; names the browser proof it rests on |
| `TestErrorDrawerWiringHoldsTogether` | Done | `internal/component/web/markup_contract_test.go` | |
| `TestAgentEnvEntriesCarryOneSettingEach` | Changed | `internal/component/web/testing/runner_test.go` | Added at closure, not in the plan: `agentEnv` now builds three env entries from one shared `textbuf.Buffer` |
| the eight `.wb` files | Done | `test/web/` | `make ze-web-test` 97/97 |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/lg/handler_ui.go`, `peers.templ` | Done | Unnamed messages, `peersStreamError`, `swapEmpty:true` |
| the 16 chaos SSE sites | Done | `render.go`, `dashboard.go`, `viz.go`, `viz_convergence_trend.go`, `handlers.go`, `sse.go` |
| `assets/notification.js`, `cli.js`, `log-live.js`, `theme.js` | Done | |
| the 21 out-of-band sites | Done | Judged; none depended on the old order (R-2) |
| the pushed-URL sites and their handlers | Changed | Every one already answered a full page; no handler needed a change |
| the eight Freeze polls | Done | `viz.go`, `viz_matrix.go`, `viz_timeline.go`, `viz_panels.go` |
| `scripts/codegen/web_assets.go`, `internal/test/markupcheck/head.go` | Done | Both independent checkers follow the renamed attributes |
| `third_party/web/MANIFEST.md`, `scripts/vendor/sync_web.go` | Done | Vendored tools section; the asset list is core plus extension |
| the six plus two `.wb` files, `handler_history_test.go`, the scanner, the gate | Done | |
| `internal/test/cli/cmd_lg.go` | Changed | Not in the plan: `ze-test lg --listen` serves the real looking glass with an engine that always fails, so AC-12 has a server |

### Audit Summary

- **Total items:** 43 (5 requirements, 13 AC, 13 tests, 12 files)
- **Done:** 40
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations and in the rows above)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| htmx 4 is the served library, with no compatibility shim | functional + unit | `TestPagesServeHtmx4` reads the served bytes for `4.0.0-beta6`; `TestHtmx2IsGone` finds neither htmx 2 file anywhere; `make ze-htmx-upgrade-check` exits 0 over 607 files |
| Every operator-visible behaviour survives the swap | functional (browser) | `make ze-web-test` 97/97, including the eight `.wb` this spec added across web, lg and chaos |
| A named SSE event silently not swapping is caught | functional (browser) | `lg-stream-swaps.wb` asserts a POSITIVE uptime change over 70s; `chaos-stream-swaps.wb` asserts a toast count rising on the one SSE-only target |
| A handler reading `detail.xhr` silently reporting nothing is caught | functional (browser) | `web-error-toast.wb` asserts the toast text AND the swapped fragment; `chaos-error-fragment.wb` asserts the fragment lands in `#action-error` and the peer table survives |
| Back and forward still work on every URL that pushes one | unit + functional | `TestPushedURLsAnswerAFullPage` over 336 derived URLs; `history-full-page.wb` drives both traversals in a real browser under the Navigation API |
| The cutover is provable after it lands | gate | `make ze-htmx-upgrade-check` is a stage of BOTH `stagesForMode` lists, and its explained list fails both ways |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none: the spec declares no deferral shard | done | `plan/deferrals/web-htmx4-cutover.md` does not exist and was never created |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/web-htmx4-cutover-bd3cb1c5-21a8-4146-a390-5190a37971e8.md` |
| `review_gate.py check` | `OK (clean, hashes match)` over 125 files |
| Rounds | 2 |
| Reviewer lenses used | guard-and-ratchet (the new gate, the explained list, the Freeze repair), generated-artifact-with-its-generator (`web_assets.go` and the two checkers), logic-and-removed-behaviour over the JS and the two stream producers |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Three explained rows named `renderPeerRow` as the inheritance carrier. The carrier is `renderPeerRowInsert` (`internal/chaos/web/dashboard.go:688`, its tag at :700); `renderPeerRow` (:658) carries `hx-swap-oob="outerHTML"` alone and is not a carrier. A gate's justification naming the wrong producer is a false claim (`ai/rules/evidence.md`) | `scripts/dev/htmx-upgrade-explained.txt` rows for `dashboard.go`, `render.go`, `sse-peer-row-insert.html` | Corrected to `renderPeerRowInsert` and `writePeerRows`; `make ze-htmx-upgrade-check` re-run and green |
| 2 | ISSUE | `agentEnv` builds three env entries from ONE `textbuf.Buffer` and resets before the third alone. This diff turned the first from a plain constant into a Buffer expression, so the second now depends on `String()` detaching. Nothing read the entries back | `internal/component/web/testing/runner.go` -- `agentEnv` | `TestAgentEnvEntriesCarryOneSettingEach` (`runner_test.go`). Measured: `String()` does detach, each entry carries one setting, and the test now pins it |

NOTEs recorded and not fixed:

- The gate keys on `(path, category)`, so a NEW inheritance issue in one of the 16 explained files passes silently. Every defect-bearing category (`old-event`, `removed-attr`, `ext`, `swap-syntax`) has no explained row anywhere, so a new one of those is red. A count column would red every new chaos poll panel for the one category the Design Insight calls a reading list rather than a defect list (`ai/rules/simplicity.md`).
- `assetImplementing` (`internal/test/markupcheck/head.go`) returns the extension alone for `hx-sse:`, while `assetsFor` (`scripts/codegen/web_assets.go`) returns core plus extension. The two independent checkers differ only for a page whose ONLY htmx attribute is `hx-sse:connect`; no such page exists, and the comment states the choice.
- `handleRequestError`'s no-answer branch has no automated browser proof. The `.wb` runner cannot kill a server mid-test; the same discrimination was measured by hand on chaos. No AC depends on it.
- `make ze-validate` reports 6 unwired-export findings over `runner.go` and `dashboard.go`. All six have in-package callers, none is new in this diff, and the check scopes itself to changed files, so touching those files surfaced dormant findings. `ze-verify` runs `ze-validate-tree`, which declares an empty changed set.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `test/web/lg-stream-swaps.wb` | Yes | `ls -1`, 1.8K, 2026-08-15 20:02 |
| `test/web/lg-stream-error.wb` | Yes | `ls -1`, 1.5K |
| `test/web/chaos-stream-swaps.wb` | Yes | `ls -1`, 1.6K |
| `test/web/web-error-toast.wb` | Yes | `ls -1`, 1.8K |
| `test/web/oob-order.wb` | Yes | `ls -1`, 2.2K |
| `test/web/history-full-page.wb` | Yes | `ls -1`, 2.4K |
| `test/web/lg-error-fragment.wb` | Yes | `ls -1`, 6.4K |
| `test/web/chaos-error-fragment.wb` | Yes | `ls -1`, 2.2K |
| `internal/component/web/handler_history_test.go` | Yes | `ls -1`, 4.0K |
| `internal/chaos/web/render_test.go` | Yes | `ls -1`, 3.9K |
| `internal/test/cli/cmd_lg.go` | Yes | `ls -1`, 2.3K |
| `third_party/web/htmx-upgrade-check.py` | Yes | `ls -1`, 22K |
| `scripts/dev/htmx_upgrade_check.py` | Yes | `ls -1`, 11K |
| `scripts/dev/htmx_upgrade_check_test.py` | Yes | `ls -1`, 8.7K |
| `scripts/dev/htmx-upgrade-explained.txt` | Yes | `ls -1`, 6.0K |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-9, AC-11 | The served bytes are htmx 4 and no listener names a htmx 2 event | `make ze-test-pkg PKG=./internal/component/web`: ok (phase 6, 303s with -race) |
| AC-2 | htmx 2 is absent from `third_party/` and every consumer | `git status`: 8 deletions covering `htmx4.min.js` and `sse.js` in `third_party/web/htmx/` and all three `assets/` |
| AC-3, AC-4, AC-5, AC-6, AC-7, AC-8, AC-12 | Every browser proof passes | `make ze-web-test`: 97/97, 155.1s |
| AC-10 | Only intended fixture changes | `make ze-test-pkg PKG=./internal/chaos/web`: ok, 3.030s (2026-08-16, this session) |
| AC-13 | Zero unexplained issues, and the gate is wired | `make ze-htmx-upgrade-check`: `607 file(s) ... carry no unexplained htmx 4 upgrade issue (16 explained)`, exit 0 (2026-08-16, after the review fix). `python3 scripts/dev/htmx_upgrade_check_test.py`: 13/13 |
| the Freeze repair | Its test would fail if the listener were removed | Probed 2026-08-16: deleting the listener line from `writeLayout` reds `TestFreezeListenerReadsTheMarker` on 4 needles and reds both index goldens. `render.go` restored, sha256 compared |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| HTTP GET of any page | `TestPagesServeHtmx4` | Yes: reads `derivedPageAssets`, then the BYTES of each named asset. A name test would pass over htmx 2 wearing htmx 4's name |
| lg SSE stream emits an event | `test/web/lg-stream-swaps.wb` | Yes: `option=server:kind=lg` starts a daemon plus a `ze-test peer` sink, waits 70s and asserts the uptime crossed a minute |
| chaos SSE stream emits an event | `test/web/chaos-stream-swaps.wb` | Yes: `option=server:kind=chaos` starts `ze-chaos`; asserts on `#toast-container`, the one target no poll fills |
| A request fails with 4xx | `test/web/web-error-toast.wb` | Yes: asserts the toast text and the swapped fragment; `chaos-error-fragment.wb` and `lg-error-fragment.wb` cover the other two interfaces |
| A response carries out-of-band content | `test/web/oob-order.wb` | Yes: its last step reads the drawer sync, which only the listener produces |
| Browser back after a pushed URL | `test/web/history-full-page.wb` | Yes: direct GET, then `action=back` asserting the address does NOT contain `/show/bgp`, then `action=forward` |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `#G(e)` in the beta6 core returns `{"HX-Request":"true", "HX-Source":..., "HX-Current-URL":..., Accept:"text/html"}`. Re-read at closure |
| A-2 | broken, closed | `htmx:after:swap` fires once per request on `ctx.sourceElement` with `{ctx}` only. `initErrorPanel` moved to `htmx:after:settle` |
| A-3 | confirmed | `ajax()` does `Object.assign(ctx, context)`; probe confirmed values and an element-valued target |
| A-4 | confirmed | Probed on a `div` and a `tr`, with the trap recorded: a bare `tr` must stay first in its payload |
| A-5 | confirmed | `hx-sse` returns false from its before-response hook for `text/event-stream`; probed at a 250ms timeout over 34 reads |
| A-6 | confirmed | 223 golden HTML files parsed: 238 request-issuing elements, 2 with a form ancestor, both `hx-post` carrying `hx-include` |
| A-7 | broken, closed | The tree ships beta6. None of its five changes touches a map row with a Ze site |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #4 API/RPC: the lg SSE stream's shape | `ai/digests/web.md` names `peerStreamBody` and `peersStreamError`, read against `internal/component/lg/handler_ui.go` | Yes |
| #6 user guide: `docs/guide/web-interface.md` | The two toast kinds, anchored on `notification.js -- handleResponseError and handleRequestError`; both functions read at closure | Yes |
| #10 test infrastructure | `docs/functional-tests.md` and `docs/architecture/testing/runner-architecture.md` name `option=server:kind=`, `option=env:`, `action=back`, `action=forward`, each read against `parser.go` and `runner.go` | Yes |
| #12 internal architecture | `docs/architecture/chaos-web-dashboard.md`'s SSE section rewritten against `sse.go` and `dashboard.go`; its old table named events that no longer existed | Yes |
| #16 source anchors on changed files | `make ze-doc-test` exit 0 and `make ze-verify-wiring-docs` exit 0 (2026-08-16). Both stale-anchor checks read the tree | Yes |
| #17 existing examples for this area | `grep -rn "sse.js\|hx-ext\|sse-swap\|sse-connect\|2\.0\.4\|peer-update\|htmx:afterSettle\|htmx:beforeSwap\|htmx:oobAfterSwap\|htmx:responseError\|htmx:sendError\|htmx:afterRequest\|htmx:sseError\|htmx:sseOpen\|htmx:afterSwap" docs/ ai/ --include="*.md"` returns one hit, an unrelated BGP wire method in `ai/digests/api-ipc.md` | Yes |
| New gate discoverable | `ai/INDEX.md` carries the `make ze-htmx-upgrade-check` row in Dev Tools | Yes |

## Core Insight

**A dependency upgrade removes features, and a golden fixture cannot see it.**
Every gate in this repository reads markup or bytes. htmx 4 dropped the trigger
filter, so `hx-trigger="every 500ms [!window._frozen]"` kept rendering, kept
parsing, and stopped meaning anything: the Freeze control changed nothing and no
check was red. The same shape governs the whole spec, which is why every phase
owed a browser proof rather than a fixture diff. What a fixture proves is that
the bytes did not move. What an upgrade changes is what those bytes DO.
