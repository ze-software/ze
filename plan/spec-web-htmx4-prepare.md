# Spec: web-htmx4-prepare

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze serves htmx 2.0.4 from three packages. The owner decided on 2026-08-15 to
convert fully to htmx 4, with no compatibility shim, across web, lg and chaos.

A library swap cannot half-land. The moment `htmx.min.js` becomes htmx 4, every
unconverted site breaks together. This spec therefore carries every part of the
migration that ships while Ze still serves htmx 2, so the cutover spec that
follows holds only the work that must land in one commit.

Four goals, each valuable on its own if the cutover never runs:

1. htmx 4 and its `hx-sse` extension are vendored and synced, and no page loads
   them. The vendoring path becomes a gate rather than a habit.
2. Chaos responses gain golden fixtures. Chaos holds 346 of the 665 htmx
   attribute occurrences and has no fixtures, so today nothing would show what a
   rename changed there.
3. Handlers that answer 4xx and 5xx return a fragment that can be swapped into
   the target. htmx 4 swaps every response except 204 and 304, which inverts the
   contract `handleResponseError` relies on. htmx 2 swaps neither, so the
   fragments are inert until the cutover.
4. Each page imports only the assets that page needs, and a generator derives
   the set from the component graph.

The cutover spec (`spec-web-htmx4-cutover`, not yet written) holds the
mechanical renames, the SSE conversion, the out-of-band ordering reversal, the
inheritance carrier and the history change.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the registration pattern the generator must follow
  → Constraint: a new per-page mapping registers or generates. It MUST NOT become a hand-maintained switch in a shared package.
- [ ] `ai/rules/repo-maintenance.md` - what a new generator and a new gate owe
  → Constraint: a new generator needs a `make generate` entry, a staleness gate, and an `ai/INDEX.md` row, or no future agent finds it.
- [ ] `ai/rules/evidence.md` - the fail-closed rule for guards
  → Decision: the existing drift check reports and exits 0, so it is a guard that fails open. The gate MUST exit non-zero.
- [ ] `ai/rules/simplicity.md` - the generator against a hand-written manifest
  → Decision: the generator is justified by drift, not by size. Seven head blocks would be seven manifest lines today, and the failure mode of a stale manifest is a page that renders correctly and does nothing.
- [ ] `third_party/web/MANIFEST.md` - the vendoring contract
  → Constraint: `third_party/web/` is the source of truth and consumer copies are generated. New extension files each need a MANIFEST row and a `sync_web.go` entry.

**Key insights:**
- `//go:embed` cannot reference a path outside its own package, which is why one library is vendored four times. Three are consumers and one is the source.
- The consumer copies MUST stay tracked in git. Compiling without `make` has to work.
- `internal/test/golden` is a shared package and `AssertPortFidelity` takes the root directory as a parameter, so the port-fidelity machinery works for chaos unchanged.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/vendor/sync_web.go` - copies each asset from `third_party/web/<pkg>/` to every consumer. Run by `make ze-sync-vendor-web` only, never by `make generate`.
- [ ] `scripts/vendor/check_web.go` - `driftCheck` compares consumer copies against the source and needs no network. `run` calls it, prints, and returns nil regardless. `main` exits non-zero only when `run` errors, so drift never fails the process.
- [ ] `Makefile` target `generate` - runs `yang_glue.go`, `plugin_imports.go`, `feature_tags.go`, templ, and `fuzz-targets.py`. The vendor sync is absent.
- [ ] `internal/component/web/assets/notification.js` - `handleResponseError` reads `evt.detail.xhr` and returns early when it is absent. Its comment states the contract: htmx does not swap non-2xx, so without this handler a failed action gives no feedback.
- [ ] `internal/component/web/page_layout.templ`, `page_workbench.templ`, `page_login.templ`, `page_snapshot.templ`, `l2tp_list.templ`, `l2tp_detail.templ`, `internal/component/lg/layout.templ` - seven head blocks, each hand-writing its script tags. Corrected 2026-08-15: this spec first said five. `page_login.templ` and `page_snapshot.templ` were missed. A page the generator never enumerates is a page whose import set nothing checks, which is the failure A-5 exists to catch.
- [ ] `internal/chaos/web/render.go` - chaos renders HTML from Go string literals and its head block loads htmx and `sse.js`. No CSP header is set anywhere under `internal/chaos`.
- [ ] `internal/test/golden/portcheck.go` - `AssertPortFidelity` reads pre-port bytes through `git archive` and reports per-file findings. It takes the root directory as a parameter.

**Behavior to preserve:**
- Every page keeps serving htmx 2.0.4 and behaving as it does today. This spec changes no rendered htmx attribute.
- The consumer asset trees stay byte-identical to `third_party/web/`.
- An operator who triggers a failed action still gets feedback in the browser.
- Compiling with no `make` run still works, so generated asset copies stay tracked.

**Behavior to change:**
- `make generate` gains the vendor sync, so consumer copies become generated rather than hand-copied.
- The drift check exits non-zero when a consumer copy differs.
- Handlers answering 4xx and 5xx return a fragment renderable into the request's target.
- Each page's script tags come from generated code rather than a hand-written head block.

## Data Flow (MANDATORY)

### Entry Point
- `make generate`, for the vendor sync and the per-page import generator.
- `make ze-verify`, for the drift gate and the import check.
- An HTTP request to any web, lg or chaos page, for the rendered script tags.
- An HTTP request that a handler answers with 4xx or 5xx, for the error fragment.

### Transformation Path
1. `third_party/web/` holds one copy of each asset. `sync_web.go` copies each into the three consumer `assets/` trees.
2. The import generator walks each page's `.templ`, follows `@component(...)` invocations transitively, and collects the `hx-*` attributes each component names.
3. The generator emits, per page, the set of asset files that page must load. The layout renders that set instead of a hand-written list.
4. A handler that answers an error status renders an error fragment instead of a bare status plus text.
5. The fixture check reads each captured golden page and asserts every `hx-*` attribute in the rendered bytes has its extension imported by that page.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build ↔ source tree | the generator writes tracked Go, `make generate` regenerates it | No |
| Source tree ↔ served bytes | `//go:embed assets` per package | No |
| Handler ↔ browser | error fragment in the response body at a 4xx/5xx status | No |
| Generator ↔ fixture check | one over-approximates from source, the other under-approximates from output | No |

### Integration Points
- `Makefile` target `generate` - the sync and the generator join the existing five steps.
- `scripts/vendor/check_web.go` - `driftCheck` becomes the gate's exit status.
- `internal/test/golden` - the chaos capture reuses `AssertPortFidelity` and `NormalizeHTML`.
- The seven head blocks - each stops hand-writing script tags.

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
| A-1 | htmx 2.0.4 swaps no 4xx or 5xx response, so an error fragment is inert until the cutover | `handleResponseError` comment, `internal/component/web/assets/notification.js` | The fragments change behaviour now, and this spec stops being cutover-free | Serve a 400 with a fragment body against vendored htmx 2.0.4 in a browser and confirm no swap | confirmed. Chrome 151 loaded `internal/component/web/assets/htmx.min.js` under the production CSP and fired four `hx-get`s: the 200 swapped, and 400, 403 and 500 did not. Each carried `htmx:beforeSwap` with `shouldSwap=false`, then `htmx:responseError`. The 200 control is what makes the three absences evidence. Held again against the running daemon by `test/web/web-error-fragment.wb`. It covers the looking glass and the chaos dashboard as well: all three serve the SAME asset, md5 `19a573773be4ca22570ca2f8543120c5`, which `make ze-check-vendor-web` keeps true, and neither of those two interfaces has an `htmx:responseError` handler, so a refusal reaches nobody there today |
| A-2 | Chaos HTML responses are deterministic enough to capture as golden fixtures | The web and lg captures already work with fixed data | Chaos fixtures flap and the gate is worthless | Capture twice from identical state and diff | confirmed: `TestChaosCaptureIsDeterministic` renders all 40 cases twice from independently seeded state, green at `-count=5`. Two spans needed a rewrite, both named in `golden_test.go` |
| A-3 | `AssertPortFidelity` works against a chaos root with no change to `internal/test/golden` | `portcheck.go`, whose signature takes the root as a parameter | The chaos phase needs the helper generalised first | Called from `internal/chaos/web` against `assets` at HEAD: the comparison ran over 5 units and passed, with no edit to the helper | confirmed. The ref is the constraint, not the root: `golden.PrePortRef` predates the chaos fixtures, so the call fails closed there. See R-7 |
| A-4 | templ's `@component(...)` invocation syntax is stable enough to parse for the component graph | `.templ` sources across web and lg | The generator breaks whenever templ changes syntax, which is the maintenance tail this design accepts | Parse every `.templ` in the tree and assert the graph reaches each known page | confirmed, and the tail is instrumented. `TestComponentGraphReachesEveryPage` fails when a source names an htmx attribute and the set derived for its page is empty. Proven to fire: with the attribute pattern changed to match nothing, it named 5 pages across chaos and lg |
| A-5 | Every page that renders an htmx attribute is reachable from a layout the generator can enumerate | Seven head blocks found by grep: six in `internal/component/web`, one in `internal/component/lg`. The first count of five was wrong and phase 1 corrected it | A page renders htmx with no import set and the check cannot see it | Assert the generator's page list equals the set of templates containing a head block | confirmed against ELEVEN head blocks, not eight. `TestComponentGraphReachesEveryPage` walks `internal/` and finds the eight the generator enumerates plus three that render no htmx attribute at all and need none: `internal/perf/report/html.go`, `internal/component/api/rest/server.go` (Swagger UI) and the head marker `internal/test/markupcheck/head.go` declares. The test fails the moment one of the three renders an attribute |
| A-6 | The consumer asset copies must remain tracked in git | `//go:embed` needs the file at compile time; `ze-tracked-build-check` compiles what git holds | Making them ignored breaks compiling without `make` | Confirm each copy is tracked, and that the tracked-build check passes after the sync joins `make generate` | confirmed -- every pre-existing copy is tracked, `.gitignore` covers none of the four `assets/` directories, and the six new htmx 4 copies are in the commit list. `ze-check-vendor-web` reports MISSING for a copy left out of the commit, so the gate itself now enforces it |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The generator over-approximates and ships an extension a page never uses | The import set is larger than the fixture check's derived set | Accept it. Over-approximation costs bytes, under-approximation costs a broken page |
| R-2 | Chaos fixtures capture a timestamp, a counter or a random id and flap | Two captures from identical state differ | Normalise the volatile field in the capture, the way `NormalizeHTML` already normalises whitespace |
| R-3 | Adding the sync to `make generate` fights another session running `make generate` in the shared checkout | A consumer copy changes under a running gate | The sync is idempotent and byte-comparing, so a concurrent run converges. Do not run a gate while another session generates |
| R-4 | The error fragments change what an operator sees under htmx 2 | A functional test that asserted an error message now sees markup | A-1 settles this before the phase starts. The toast path stays until the cutover |
| R-5 | The generator and the fixture check disagree, and the disagreement is the generator being right | The check fails on a page whose branch the fixture never exercised | The check reports the attribute and the page. Add fixture coverage for the branch rather than weakening the check |
| R-6 | Making the drift check exit non-zero turns a silent condition into a red gate for everybody at once | `ze-verify` goes red on first arming | Run the check before wiring it in, and fix any existing drift in the same phase |
| R-7 | The cutover compares the chaos fixtures against `golden.PrePortRef`, which predates them, and reads the failure as a chaos defect | `git archive 80f0b8b57 -- testdata/golden` reports `pathspec did not match any files` | The cutover passes `REF=<the commit that lands internal/chaos/web/testdata/golden>`. `AssertPortFidelity` fails closed on a wrong ref, so the mistake cannot pass silently |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong import set gives a page that renders correctly and does nothing in the browser. A wrong error fragment gives an operator no feedback on a failed action. Neither affects routing, the daemon, or any peer. |
| How is it reverted? | Single commit revert per phase. Nothing here changes a wire format, a config schema, or on-disk state. |
| Who else touches this path? | Several sessions share this checkout. `internal/component/web` is under active edit, and another session is mid-rename of `ctx.ProcessName` in `internal/component/plugin/server/command.go`, which web imports transitively. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make generate` | → | the vendor sync step | `TestGenerateSyncsVendoredAssets` |
| `make ze-verify` | → | the drift gate's exit status | `TestDriftCheckExitsNonZeroOnMismatch` |
| `make generate` | → | the per-page import generator | `TestGeneratedImportsAreCurrent` |
| HTTP GET of any captured chaos page | → | the chaos golden capture | `TestChaosGoldenOutput` |
| HTTP request answered 4xx or 5xx | → | the error fragment renderer | `test/web/web-error-fragment.wb`, with `TestErrorStatusReturnsSwappableFragment` covering the renderer directly |
| The same request to the looking glass or the chaos dashboard | → | the same renderer, through each server's own chain | `TestLGHandlerGoldenOutput/ui-route-detail-invalid-htmx`, `TestChaosGoldenOutput/control-pause-no-control-htmx` |
| HTTP GET of any page | → | the generated script tags in its head | `test/web/web-page-assets.wb`, with `TestPageImportsCoverRenderedAttributes` covering the fixture side. `.ci` cannot serve it: that runner has no HTTPS client |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A consumer asset copy is edited, then `make generate` runs | The copy is restored byte-identical to its `third_party/web/` source |
| AC-2 | A consumer asset copy differs from its source and the drift gate runs | The gate exits non-zero and names the file. Today it exits 0 |
| AC-3 | The drift gate runs with no network access | It completes and reports, because it queries no registry |
| AC-4 | htmx 4 core and `hx-sse` are vendored and synced | Both are present in `third_party/web/htmx/`, carry a MANIFEST row each, and are copied to every consumer. No page loads either, and every page still serves htmx 2.0.4 |
| AC-5 | Any chaos page is rendered and one byte of its markup changes | The chaos golden check fails and names the page |
| AC-6 | A handler in web, lg or chaos answers a request with a 4xx or 5xx status and a body | The body is a fragment that can be swapped into the request's target, not a bare status line |
| AC-6b | The lg and chaos surfaces specifically | Scope correction, 2026-08-15. AC-6 first read as web only, and phase 4 converted web. lg and chaos serve from their own `http.Server`, so 17 sites in `internal/component/lg` and 30 in `internal/chaos` still answer a bare status line. The owner chose all three surfaces, and htmx 4 swaps a bare status line into the target, which would put raw error text into the page |
| AC-7 | An operator triggers a failing action in a browser serving htmx 2.0.4 | Feedback still appears, unchanged from today |
| AC-8 | `make generate` runs | Per-page import sets are generated from the `.templ` component graph, and the generated file is tracked |
| AC-9 | A page's rendered bytes carry an htmx attribute whose extension that page does not import | The fixture check fails, naming the page and the attribute |
| AC-10 | The generated import file is edited by hand and the staleness gate runs | The gate fails, the same way the templ generate check already does |
| AC-11 | A page renders no htmx attribute at all | It imports no htmx asset |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Submits a config change that fails validation | handler answers 4xx with an error fragment, htmx 2 does not swap it, the existing handler surfaces the message | `TestErrorStatusReturnsSwappableFragment` |
| 2 | Loads a page that opens no SSE stream | generated import set omits the SSE extension, so the browser fetches less | `TestPageImportsCoverRenderedAttributes` |
| 3 | Loads any chaos dashboard page | chaos renders exactly the bytes its fixture captured | `TestChaosGoldenOutput` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDriftCheckExitsNonZeroOnMismatch` | `scripts/vendor/check_web_test.go` | AC-2, the guard fails closed | PASS |
| `TestDriftCheckNeedsNoNetwork` | `scripts/vendor/check_web_test.go` | AC-3 | PASS |
| `TestGenerateSyncsVendoredAssets` | `scripts/vendor/sync_web_test.go` | AC-1 | PASS |
| `TestComponentGraphReachesEveryPage` | `scripts/codegen/web_assets_test.go` | A-4, A-5 | PASS |
| `TestGeneratedImportsAreCurrent` | `scripts/codegen/web_assets_test.go` | AC-8, AC-10 | PASS |
| `TestPageWithNoHtmxImportsNothing` | `scripts/codegen/web_assets_test.go` | AC-11 | PASS |
| `TestPageImportsCoverRenderedAttributes` | `internal/component/web/markup_contract_test.go` | AC-9, the fixture side | PASS |
| `TestLGPageImportsCoverRenderedAttributes` | `internal/component/lg/page_assets_test.go` | AC-9 over the 11 captured looking-glass pages | PASS, proven to discriminate |
| `TestChaosPageImportsCoverRenderedAttributes` | `internal/chaos/web/page_assets_test.go` | AC-9 over the chaos dashboard | PASS |
| `TestErrorStatusReturnsSwappableFragment` | `internal/component/web/handler_error_test.go` | AC-6 | PASS, proven to discriminate: both cases go red when `ServerHandler` drops `errorFragments` |
| `TestErrorFragmentMiddlewareConvertsOnlyBareStatusLines` | `internal/component/web/handler_error_test.go` | AC-6 by KIND: the plain-text refusal converts, and the html, JSON, 200 and empty-body answers pass through | PASS |
| `TestErrorFragmentEscapesTheMessage` | `internal/component/web/handler_error_test.go` | the security row: an operator value in a message is escaped by templ | PASS |
| `TestErrorFragmentAndNotificationAgree` | `internal/component/web/handler_error_test.go` | AC-7: `notification.js` reads a class the fragment carries | PASS |
| `TestRefusedSecretValueNeverReachesTheBrowser` | `internal/component/web/handler_error_test.go` | the security row: `config.LeafHoldsSecret` keeps a refused secret out of the body | PASS, driven from `POST /config/set/` |
| `TestMiddlewareConvertsOnlyBareStatusLines` | `internal/core/errorfragment/errorfragment_test.go` | AC-6b by KIND, for the one middleware all three interfaces now wrap their mux with | PASS, 7 kinds |
| `TestRenderEscapesTheMessage`, `TestRenderNamesTheStatus` | `internal/core/errorfragment/errorfragment_test.go` | the security row: an operator value in a message is escaped, and the status is readable | PASS |
| `TestMiddlewareForwardsAFlush` | `internal/core/errorfragment/errorfragment_test.go` | the SSE routes of lg and chaos keep streaming through the middleware | PASS |
| `TestLGHandlerGoldenOutput` (`ui-route-detail-invalid`, `ui-route-detail-invalid-htmx`) | `internal/component/lg/handler_golden_test.go` | AC-6b for the looking glass, captured through the server's own chain | PASS, proven to discriminate: the htmx case goes red when `NewLGServer` drops the middleware, and the bare twin stays green |
| `TestChaosGoldenOutput` | `internal/chaos/web/golden_test.go` | AC-5 | passing over 42 fixtures; proven to discriminate |
| `TestChaosGoldenOutput` (`control-pause-no-control`, `control-pause-no-control-htmx`, `active-set-max-visible-invalid-htmx`) | `internal/chaos/web/golden_test.go` | AC-6b for the chaos dashboard | PASS, proven to discriminate: both htmx cases go red when `fragmentMux` registers the bare handler, and the bare twin stays green |
| `TestChaosCaptureIsDeterministic` | `internal/chaos/web/golden_test.go` | A-2, R-2 | passing at `-count=5` |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A: this spec adds no numeric input | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-error-fragment` | `test/web/web-error-fragment.wb` | a failed config action answers a fragment and the operator sees the message | PASS, proven to discriminate: it goes red when `handleResponseError` reads the raw body again. `.ci` cannot serve it for the reason the row below gives, and a browser is the only place AC-7 can be observed |
| `web-page-assets` | `test/web/web-page-assets.wb` | a page's head carries only the assets it needs | PASS, proven to discriminate. `.ci` cannot serve it: the runner has no HTTPS client (`test/plugin/web-json-response.ci` records the same limit), so the browser suite is where an HTTP GET of a page is asserted |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A: no protocol behavior changes | - | - | - | - |

## Files to Modify
- `Makefile` - `generate` gains the vendor sync and the import generator; a drift gate target joins `ze-verify`
- `scripts/vendor/check_web.go` - `driftCheck`'s result reaches the exit status; the registry query separates from the local invariant
- `scripts/vendor/sync_web.go` - entries for the htmx 4 core and `hx-sse`
- `third_party/web/MANIFEST.md` - a row per new asset
- `internal/component/web/page_layout.templ`, `page_workbench.templ`, `page_login.templ`, `page_snapshot.templ`, `l2tp_list.templ`, `l2tp_detail.templ` - head blocks render the generated import set
- `internal/component/lg/layout.templ` - same
- `internal/chaos/web/render.go` - same, and its head block is a Go string literal
- `internal/component/web/assets/notification.js` - reads the error fragment; the `detail.xhr` rewrite waits for the cutover
- `internal/component/lg/server.go` - `NewLGServer` wraps the mux with the error-fragment middleware; `assets/style.css` styles the fragment
- `internal/chaos/web/handlers.go` - `registerRoutes` registers through `fragmentMux`; `assets/style.css` styles the fragment
- `ai/INDEX.md` - rows for the new generator and the new gate

## Files to Create
- `scripts/codegen/web_assets.go` - the per-page import generator
- `scripts/codegen/web_assets_test.go` - its tests
- `scripts/vendor/check_web_test.go` - the drift gate's tests
- `scripts/vendor/sync_web_test.go` - the sync's tests
- `internal/chaos/web/golden_test.go` - the chaos capture and check
- `internal/chaos/web/testdata/golden/` - the captured fixtures
- `internal/component/web/handler_error_test.go` - the error fragment's tests
- `internal/core/errorfragment/` - the one conversion and the one fragment shape, shared by web, lg and chaos
- `internal/chaos/web/route_mux.go` - `fragmentMux`, which registers every chaos route behind that middleware
- `third_party/web/htmx/htmx4.min.js`, `third_party/web/htmx/hx-sse.min.js` - the vendored htmx 4 assets
- `test/web/*.ci` - the two functional tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config option is added. The import set is derived at build time, never configured |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | No command added |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | Yes | `test/web/*.ci`, two scenarios in the TDD plan |
| Pipe completeness | N-A | No CLI output produced |
| Env var registration | N-A | No env var added |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate at runtime. The new assets are embedded, not read from the host |
| Prometheus counters/metrics | N-A | No observable runtime state added |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Preparation only. Every page behaves as it does today |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No command touched |
| 4 | API/RPC added/changed? | No | No API shape changes. Error responses gain a body, not a new endpoint |
| 5 | Plugin added/changed? | No | No plugin touched |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md`, for the error-feedback behaviour |
| 7 | Wire format changed? | No | No wire format touched |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs this |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for the chaos golden capture |
| 11 | Affects daemon comparison? | No | No comparable feature changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/` for the vendoring and generation path |
| 13 | Route metadata keys added/changed? | No | No route metadata touched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on every file in Files to Modify and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify the web-interface guide's examples against the handlers after the error-fragment phase |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make every entry point exist and fail
   - Tests: all six rows of the Wiring Test table, each failing for the right reason
   - Files: `Makefile`, `scripts/vendor/check_web_test.go`, `scripts/vendor/sync_web_test.go`, `scripts/codegen/web_assets.go` as a stub, `internal/chaos/web/golden_test.go` as a stub
   - Verify: each test names a target or a symbol that exists and does nothing yet
2. **Phase: Vendoring becomes a gate** -- AC-1, AC-2, AC-3, AC-4
   - Tests: `TestDriftCheckExitsNonZeroOnMismatch`, `TestDriftCheckNeedsNoNetwork`, `TestGenerateSyncsVendoredAssets`
   - Files: `scripts/vendor/check_web.go`, `scripts/vendor/sync_web.go`, `third_party/web/MANIFEST.md`, `Makefile`, the vendored htmx 4 assets
   - Verify: existing drift is fixed BEFORE the gate is armed (R-6), then the gate goes red on an edited copy and green on a clean tree
3. **Phase: Chaos gains fixtures** -- AC-5
   - Tests: `TestChaosGoldenOutput`, `TestChaosCaptureIsDeterministic`
   - Files: `internal/chaos/web/golden_test.go`, `internal/chaos/web/testdata/golden/`
   - Verify: capture twice from identical state and diff (A-2), then change one byte of a renderer and confirm the check names the page
4. **Phase: Error responses become fragments** -- AC-6, AC-7
   - Tests: `TestErrorStatusReturnsSwappableFragment`, the `web-error-fragment` functional test
   - Files: the web handlers answering 4xx and 5xx, `internal/component/web/assets/notification.js`
   - Verify: A-1 first, in a browser against vendored htmx 2.0.4. Then confirm an operator still sees feedback, unchanged
5. **Phase: Per-page imports are generated** -- AC-8, AC-9, AC-10, AC-11
   - Tests: `TestComponentGraphReachesEveryPage`, `TestGeneratedImportsAreCurrent`, `TestPageWithNoHtmxImportsNothing`, `TestPageImportsCoverRenderedAttributes`, the `web-page-assets` functional test
   - Files: `scripts/codegen/web_assets.go`, the seven head blocks, `internal/chaos/web/render.go`, `Makefile`, `ai/INDEX.md`
   - Verify: the generated set and the fixture-derived set agree on every captured page. A disagreement is a defect in one of them, never a reason to relax the check
6. **Phase: The looking glass and the chaos dashboard** -- AC-6b
   - Tests: `TestMiddlewareConvertsOnlyBareStatusLines` and its siblings, the lg and chaos golden pairs
   - Files: `internal/core/errorfragment/`, `internal/component/lg/server.go`, `internal/chaos/web/route_mux.go`, both `assets/style.css`, and the web migration onto the shared package
   - Verify: revert the wrap on each surface and confirm the htmx fixture goes red while its bare twin stays green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The drift gate fails closed. A miss, an error, or an empty file list MUST NOT read as "no drift" |
| Correctness | The generator over-approximates and the fixture check under-approximates. Neither is relaxed to make the other pass |
| Naming | The generated file and its target follow the naming of `plugin_imports.go` and `ze-templ-generate-check` |
| Data flow | No page reads `third_party/web/` at runtime. Only the embedded consumer copy is served |
| Rule: `ai/rules/evidence.md` | The drift gate is driven from its entry point in a test, not only from its helper |
| Rule: `ai/rules/no-layering.md` | The generated head block REPLACES the hand-written script tags. Both MUST NOT coexist |
| Rule: `ai/rules/repo-maintenance.md` | The new generator and gate each have an `ai/INDEX.md` row and a `make` entry |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| htmx 4 and `hx-sse` vendored and synced | `md5` of each consumer copy equals its `third_party/web/` source |
| No page loads htmx 4 yet | grep the rendered golden fixtures for the htmx 4 filename and find nothing |
| The drift gate fails closed | edit a consumer copy, run the gate, confirm a non-zero exit |
| Chaos fixtures exist | the golden directory is non-empty and the check names every captured page |
| The import set is generated | `make generate` leaves the tree clean; a hand edit makes the staleness gate fail |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error fragment content | An error fragment MUST NOT echo a secret, a credential, or a bcrypt hash. `config.LeafHoldsSecret` is the single predicate |
| Error fragment escaping | The fragment carries operator-supplied values, so every one is escaped by templ rather than concatenated |
| Asset integrity | A vendored asset is copied byte-for-byte and never rewritten. The drift gate is what proves it |
| Import set | The generated set MUST NOT be influenced by request data. It is a build-time constant |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| Generator and fixture check disagree | Neither is relaxed. Find which one is wrong, and add fixture coverage if the branch was never exercised |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Convert fully to htmx 4, no compatibility shim | `htmx-2-compat` at 538 bytes brotli would let the port land without touching markup | Owner decision, 2026-08-15. Ze adopts htmx 4 idiom at every site |
| Split the migration into preparation and cutover | One spec with nine phases | A library swap cannot half-land, so everything that can ship on htmx 2 should, leaving the smallest possible cutover |
| Derive per-page imports from the `.templ` component graph | A hand-written manifest; runtime collection; deriving from the golden fixtures | Source derivation over-approximates, which costs bytes. Fixture derivation under-approximates, which costs a broken page. Runtime collection needs the whole page buffered because templ streams the head before the body |
| Verify the import set from both directions | The generator alone | The generator over-approximates from source and the check under-approximates from output. Neither is sound alone, and a disagreement is a real defect |
| Do not ship `hx-csp` | Ship it at 1,350 bytes brotli for the `unsafe-eval` half | Its three eval features are `hx-on`, `js:` prefixes and trigger filters. `markupcheck` forbids `hx-on`, `js:` exists only in chaos which sets no CSP, and the web trigger filters became `ze-enter` in 38714d33e. It would buy nothing |
| Keep the consumer asset copies tracked in git | Ignore them and generate at build time | `//go:embed` needs the file at compile time, so compiling without `make` must work |
| One shared package holds both the conversion and the fragment markup (`internal/core/errorfragment`), and web moved onto it | A small renderer per package, with the middleware shared; or the middleware duplicated in lg and chaos | Three interfaces refuse requests the same way, so there is one question and it deserves one answer (`ai/rules/no-layering.md`). The middleware is 120 lines of buffering writer, and two more copies is where a divergence would hide. The markup is a class-name contract a script reads, so a second shape would be a second contract. The package has three real users, which is what keeps it from being an abstraction with one (`ai/rules/simplicity.md`). It is written in Go rather than templ because the whole fragment is one div and two spans: a `.templ` would add a templ dependency to `internal/core` and a full-tree `templ generate` run for 12 lines of markup. `html.EscapeString` produces the bytes templ produced, which is why no web fixture moved |

## Known Limitations
- htmx 4 is vendored but served nowhere. The cutover spec makes it live.
- `handleResponseError` still reads `evt.detail.xhr`. That rewrite belongs to the cutover, because `detail.ctx` does not exist under htmx 2.
- The chaos capture is blind to any branch its fixtures never exercise, which is the under-approximation this design accepts and pairs with the generator.
- The generator parses templ syntax, so a templ syntax change is a maintenance tail. A-4 states it and `TestComponentGraphReachesEveryPage` is its early signal.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
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

- **Vendoring became a gate.** `scripts/vendor/check_web.go` was rewritten: `driftCheck`'s
  result now reaches the exit status, and the npm registry query moved behind `--updates`.
  The pair list is DERIVED from the two trees (`vendorPackages`, `consumerDirs`,
  `subscribes`), so a new asset needs no edit there. `sync_web.go` joined `make generate`
  and `ze-check-vendor-web` joined `ze-verify` in both modes.
- **htmx 4.0.0-beta5 vendored, served nowhere.** `third_party/web/htmx/htmx4.min.js` and
  `hx-sse.min.js`, byte-identical to the `htmx.org-4.0.0-beta5.tgz` registry tarball, plus
  the six consumer copies. No page loads either.
- **Chaos gained fixtures.** 55 golden files under `internal/chaos/web/testdata/golden/`,
  captured through a real mux from `registerRoutes`. Route coverage and renderer coverage
  are machine-derived from `handlers.go`, `dashboard.go` and `render.go`, so neither list is
  hand-maintained.
- **Error responses became swappable fragments, on all three surfaces.**
  `internal/core/errorfragment` holds one middleware and one markup shape. The web UI, the
  looking glass and the chaos dashboard each wrap their mux with it in exactly one place.
- **Per-page imports are generated.** `scripts/codegen/web_assets.go` derives each page's
  asset set from the templ component graph and writes `page_assets.go` in three packages.
  Eleven pages carry a set; the L2TP, sign-in and snapshot pages load no htmx at all.
  `TestPageImportsCoverRenderedAttributes` and its two siblings check the same claim from
  the opposite direction, out of the rendered fixtures.

### Bugs Found/Fixed

- `sortPeers` (`internal/chaos/web/handlers.go`) was not a total order: `slices.SortFunc` is
  not stable and the indices arrive from a map range, so `GET /peers?sort=status` answered a
  different row order on every request. Fixed by breaking a tie on the peer index. Covered by
  `peers-sort-status` and `viz-all-peers-sort-chaos`.
- `maskSecretInMessage` (`internal/component/web/secret.go`) masked only the RAW value, and
  `config.ValidateValue` writes the value with `%q`. A secret holding a quote or a backslash
  therefore reached the browser escaped but fully readable. Found at closure review, fixed
  here, covered by `TestRefusedSecretWithAQuoteNeverReachesTheBrowser` (proven to
  discriminate: the unfixed mask answers `invalid uint16: "hunter2\"4471bc"`).
- `ServerHandler` (`internal/component/web/auth.go`) was exported with no cross-package
  caller, which `make ze-validate` reports as an ISSUE. Found at closure review, unexported.

### Documentation Updates

- `docs/architecture/web-interface.md` -- the error-answer path for web and a new "LG Refused
  Requests" section. Anchored on `internal/core/errorfragment`.
- `docs/architecture/chaos-web-dashboard.md` -- "Refused Requests", and a "Rendered Bytes"
  section for the golden capture plus the generated head block.
- `docs/architecture/web-components.md` -- "Per-page asset imports".
- `docs/architecture/testing/runner-architecture.md` -- the `head` expectation, with the
  reason `expect=html` cannot serve it.
- `ai/digests/web.md`, `ai/patterns/web-endpoint.md` -- both had described the error
  answer as "OOB error writer in fragment.go".
- `ai/INDEX.md` -- rows for `ze-sync-vendor-web`, `ze-check-vendor-web`, `ze-web-assets-check`
  and `ze-chaos-golden-update`, plus a keyword row for the per-page asset set.
- `make ze-doc-test`: red on `rfc/requirements/rfc9568.md` only, which is FOREIGN (another
  session holds `internal/plugins/vrrp/transport/transport_integration_linux_test.go`
  modified and its tag moved two lines). Journalled.

### Deviations from Plan

- The spec named five head blocks, then seven. There are ELEVEN in `internal/`, of which
  eight render an htmx attribute. A-5 was validated against the real eleven.
- The functional tests are `.wb`, not `.ci`. The `.ci` runner has no HTTPS client, so it
  cannot GET a page from a `--web-only` daemon.
- AC-6b was added mid-flight: AC-6 first read as web only, and lg and chaos serve from their
  own `http.Server`.
- The shared fragment renderer is Go, not templ. A `.templ` would put a templ dependency in
  `internal/core` and need a full-tree `templ generate` for 12 lines of markup.
  `html.EscapeString` emits the bytes templ emitted, which is why no web fixture moved.
- Documentation checklist row 10 named `docs/functional-tests.md` for the chaos capture. That
  file describes functional SUITES and the chaos capture is a Go unit test that
  `ze-unit-test` runs, so the update went to `docs/architecture/chaos-web-dashboard.md`
  instead. Row 6 named `docs/guide/web-interface.md`: no update is owed, see Documentation
  Verified below.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The mask replaced the raw value only | Every validator that names what it refused writes it with `%q`, so the message holds the ESCAPED value | closure review, then proven by reverting the fix | `maskSecretInMessage` also replaces `strconv.Quote(value)` minus its outer quotes |
| approach | `ServerHandler` was exported so both the daemon and the capture could reach it | Both are in `package web`; the export bought nothing and cost a `ze-validate` ISSUE | closure review ran `make ze-validate` | unexported to `serverHandler` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| htmx 4 and `hx-sse` vendored and synced, loaded by no page; the vendoring path is a gate | Done | `third_party/web/htmx/`, `scripts/vendor/check_web.go` `run` | `make ze-check-vendor-web` exits 0 over 23 copies and non-zero on one edit |
| Chaos responses gain golden fixtures | Done | `internal/chaos/web/golden_test.go`, `testdata/golden/` (55 files) | route and renderer coverage machine-derived |
| 4xx/5xx answers return a swappable fragment | Done | `internal/core/errorfragment/errorfragment.go` `Middleware`, `Render` | one middleware, three servers |
| Each page imports only the assets it needs, derived by a generator | Done | `scripts/codegen/web_assets.go` `derive`, three `page_assets.go` | 11 pages; 4 load no htmx |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestGenerateSyncsVendoredAssets` | both subtests |
| AC-2 | Done | `TestDriftCheckExitsNonZeroOnMismatch` | red at HEAD, green after the exit-status fix |
| AC-3 | Done | `TestDriftCheckNeedsNoNetwork` | 2 subtests, unreachable-network env |
| AC-4 | Done | `make ze-check-vendor-web`; grep of every fixture and source | no non-vendored file names `htmx4.min.js` or `hx-sse.min.js` |
| AC-5 | Done | `TestChaosGoldenOutput` | proven to discriminate twice, by a one-byte renderer change |
| AC-6 | Done | `TestErrorStatusReturnsSwappableFragment`, `test/web/web-error-fragment.wb` | red when `serverHandler` drops the middleware |
| AC-6b | Done | `TestLGHandlerGoldenOutput/ui-route-detail-invalid-htmx`, `TestChaosGoldenOutput/control-pause-no-control-htmx` | each bare twin stays green, which is the gate |
| AC-7 | Done | `test/web/web-error-fragment.wb` in a real browser | the toast reads `Error 400: invalid uint16: "70000"` |
| AC-8 | Done | `TestGenerateRunsWebAssetsGenerator`, `TestGeneratedImportsAreCurrent` | `make -n generate` names the generator |
| AC-9 | Done | `TestPageImportsCoverRenderedAttributes` + the lg and chaos siblings | red when `sse.js` is deleted from `ui-peers.txt` |
| AC-10 | Done | `make ze-web-assets-check` | exit 2 on a hand edit, naming the file and `make generate` |
| AC-11 | Done | `TestPageWithNoHtmxImportsNothing` | 4 pages carry an empty set |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every unit row of the TDD table | Done | as written in each row | all PASS; see Pre-Commit Verification |
| `TestRefusedSecretWithAQuoteNeverReachesTheBrowser` | Done | `internal/component/web/handler_error_test.go` | ADDED at closure, not in the plan |
| `web-error-fragment`, `web-page-assets` | Done | `test/web/*.wb` | `make ze-web-test` 89/89 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every path in Files to Create | Done | `ls` output in Pre-Commit Verification |
| every path in Files to Modify | Done | plus `internal/component/web/secret.go`, `handler_config_form.go`, `auth.go`, `server.go`, `fragment.go`, which the error phase needed |
| `test/web/*.ci` | Changed | `.wb`; the `.ci` runner has no HTTPS client |

### Audit Summary
- **Total items:** 12 AC + 4 task requirements + 21 tests = 37
- **Done:** 37
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the functional test carrier, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| htmx 4 vendored and synced, no page loads it, and vendoring is a gate rather than a habit | functional (gate) | `make ze-check-vendor-web` exits 0 reporting "all 23 consumer copies match their third_party/web/ source"; `TestDriftCheckExitsNonZeroOnMismatch` proves the non-zero exit; a repository-wide grep over `*.go`, `*.templ`, `*.txt`, `*.html` and `*.wb` finds no reference to `htmx4.min.js` or `hx-sse.min.js` outside the vendored files themselves |
| A rename in chaos markup is visible | functional (golden) | `TestChaosGoldenOutput` over 55 fixtures. Proven to discriminate: `hx-trigger="every 500ms"` -> `501ms` failed `index` and `index-no-control` by name, and `hx-swap-oob="delete"` -> `"Delete"` failed `sse-peer-removal` by name |
| A refused htmx request answers markup the browser can swap, on all three interfaces | functional (golden through each real chain) + browser | `ui-route-detail-invalid-htmx.txt` and two chaos `-htmx` fixtures, each captured through its own server's `Handler`; each bare twin stays green when the wrap is reverted. In the browser, `test/web/web-error-fragment.wb` |
| Under htmx 2 the operator sees exactly what it saw before | browser | A-1: Chrome 151 under the production CSP fired four `hx-get`s; only the 200 swapped. `test/web/web-error-fragment.wb` asserts the toast text and the ABSENCE of `error-fragment` from the page |
| Each page loads only what it needs, derived rather than written | functional (two independent derivations) | `make ze-web-assets-check` exit 0; `TestPageImportsCoverRenderedAttributes` + `TestLGPageImportsCoverRenderedAttributes` + `TestChaosPageImportsCoverRenderedAttributes` read the CAPTURED bytes and never read the generator. `ze-test web -p page-assets` fails when the login assertion is inverted |
| No operator-visible behaviour changed | functional | `make ze-web-test` 89/89; the only fixtures that moved are the 13 whose pages genuinely stopped loading an asset, each explained in `port_check_test.go` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| N-A: the spec metadata declares no deferral shard | done | `plan/deferrals/web-htmx4-prepare.md` does not exist; nothing to account for |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/web-htmx4-prepare-bd3cb1c5-21a8-4146-a390-5190a37971e8.md`, hash-pinned over the 159 files of commit A |
| `review_gate.py check` | clean |
| Rounds | 2 |
| Reviewer lenses used | guards and fail-closed behaviour (`check_web.go` `run`/`driftCheck`, `errorfragment` `Middleware`); security (escaping, the secret mask); wiring and gate population (`ze-validate`, `audit-test-relaxation.py`); commit atomicity against `ze-tracked-build-check` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The secret mask replaces the RAW value, and every validator that names what it refused writes it with `%q`. A secret carrying a quote or a backslash reaches the browser escaped and readable | `internal/component/web/secret.go` `maskSecretInMessage` | also replacing `strconv.Quote(value)` minus its outer quotes, with `TestRefusedSecretWithAQuoteNeverReachesTheBrowser` proving it by reverting |
| 2 | ISSUE | `ServerHandler` is a new exported symbol with no cross-package caller, which `make ze-validate` reports | `internal/component/web/auth.go` | unexported to `serverHandler`; the four call sites and three comments follow |
| 3 | NOTE | `checkUpdates` still names `htmx.min.js` and `sse.js` only, so `--updates` reports nothing about the htmx 4 pair | `scripts/vendor/check_web.go` `checkUpdates` | left. The pin is a deliberate beta and `--updates` runs on no gate path; the cutover spec repoints it |
| 4 | NOTE | The middleware converts only a body whose Content-Type was already `text/plain` at `WriteHeader` time. A handler that writes a 4xx body with no explicit type is left bare | `internal/core/errorfragment/errorfragment.go` `writer.WriteHeader` | left. Every site in the three packages goes through `http.Error`, which sets the type; the conservative branch is pass-through, never a wrong conversion |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `scripts/codegen/web_assets.go` | Yes | `ls -la` 23K |
| `scripts/codegen/web_assets_test.go` | Yes | `ls -la` 8.7K |
| `scripts/vendor/check_web_test.go` | Yes | `ls -la` 9.8K |
| `scripts/vendor/sync_web_test.go` | Yes | `ls -la` 2.5K |
| `internal/chaos/web/golden_test.go` | Yes | `ls -la` 26K |
| `internal/chaos/web/testdata/golden/` | Yes | `find` counts 55 files |
| `internal/chaos/web/route_mux.go` | Yes | `ls -la` 1.7K |
| `internal/component/web/handler_error_test.go` | Yes | `ls -la` 16K |
| `internal/core/errorfragment/` | Yes | `errorfragment.go` 8.0K, `errorfragment_test.go` 8.0K |
| `third_party/web/htmx/htmx4.min.js`, `hx-sse.min.js` | Yes | 36K and 5.4K |
| `test/web/web-error-fragment.wb`, `web-page-assets.wb` | Yes | 1.8K and 1.1K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3 | the sync runs in `generate` and the gate fails closed | `make ze-test-pkg PKG=./scripts/vendor` exit 0 (4 tests) |
| AC-4 | vendored, synced, loaded nowhere | `make ze-check-vendor-web` exit 0; the repository grep for both file names returns nothing outside the vendored copies |
| AC-5 | a chaos byte change is named | `make ze-test-pkg PKG=./internal/chaos/web` exit 0 over 55 fixtures |
| AC-6, AC-6b, AC-7 | the fragment on all three surfaces, feedback unchanged | `make ze-test-pkg PKG=./internal/core/errorfragment` exit 0; `./internal/component/lg` exit 0; `./internal/component/web` exit 0; `make ze-web-test` 89/89 |
| AC-8, AC-10 | generated and gated | `make ze-web-assets-check` exit 0; `make ze-test-pkg PKG=./scripts/codegen` exit 0 |
| AC-9, AC-11 | the fixture side agrees, and an htmx-free page imports nothing | the three `PageImportsCoverRenderedAttributes` tests, inside the three package runs above |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make generate` to the vendor sync | `scripts/vendor/sync_web_test.go` | Yes: `TestGenerateSyncsVendoredAssets` reads the recipe with `make -n` and restores an edited copy |
| `make ze-verify` to the drift gate | `scripts/vendor/check_web_test.go` | Yes: `TestZeVerifyRunsDriftGate` reads `stagesForMode`, which now lists `ze-check-vendor-web` in both branches |
| `make generate` to the import generator | `scripts/codegen/web_assets_test.go` | Yes: `TestGenerateRunsWebAssetsGenerator` and `TestGeneratedImportsAreCurrent` |
| chaos page GET to the golden capture | `internal/chaos/web/golden_test.go` | Yes: served through a real mux from `registerRoutes` |
| 4xx/5xx to the fragment | `test/web/web-error-fragment.wb` | Yes: read the file; it posts `value=70000` to `/config/set/system/dns` and asserts the toast text and the absence of `error-fragment` from the page |
| the same request to lg and chaos | `internal/component/lg/handler_golden_test.go`, `internal/chaos/web/golden_test.go` | Yes: each captured through that server's own `Handler` |
| page GET to the generated script tags | `test/web/web-page-assets.wb` | Yes: read the file; it uses `expect=head`, which `runner.go` `getHeadHTML` serves |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Chrome 151 under the production CSP: the 200 swapped and 400/403/500 did not, each with `shouldSwap=false` |
| A-2 | confirmed | `TestChaosCaptureIsDeterministic` green at `-count=10`; two clock spans normalized, everything else seeded |
| A-3 | confirmed | `AssertPortFidelity` ran over the chaos `assets` tree with no edit to `internal/test/golden`; the REF is the constraint, recorded as R-7 |
| A-4 | confirmed | `TestComponentGraphReachesEveryPage` named 5 pages when the attribute pattern was changed to match nothing |
| A-5 | confirmed | eleven head blocks found; the three the generator does not enumerate render no htmx attribute, and the test fails the moment one does |
| A-6 | confirmed | every consumer copy is tracked, `.gitignore` covers no `assets/` directory, and the gate reports MISSING for a copy left out of a commit |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 6, `docs/guide/web-interface.md` | its error-feedback claim is "Error notifications appear as toasts in the top-right corner", anchored on `component_notification_error.templ`. Both are unchanged: AC-7 requires the operator to see what it saw before, and `test/web/web-error-fragment.wb` proves the toast text | Yes, no update owed |
| Row 10, test infrastructure | `docs/functional-tests.md` describes functional SUITES and enumerates no individual test; the chaos capture is a Go test that `ze-unit-test` runs. Documented in `docs/architecture/chaos-web-dashboard.md` and `ai/INDEX.md` instead | Yes, destination changed |
| Row 12, internal architecture | `docs/architecture/web-components.md`, `web-interface.md`, `chaos-web-dashboard.md`, `testing/runner-architecture.md` all edited with source anchors | Yes |
| Row 16, existing source anchors | `docs/architecture/testing/runner-architecture.md` carried an anchor naming `checkExpectation: element/breadcrumb/html/url/title`, which became stale when `head` was added. Updated, plus a second anchor for `runner.go` | Yes |
| `make ze-doc-test` | red on `rfc/requirements/rfc9568.md` only, FOREIGN and journalled; the `ai/DOCS-TO-CODE.md` staleness is a WARNING, not a failure | Yes, attributed |

## Core Insight

Two derivations of the same fact, taken from opposite ends and never allowed to read each
other, are worth more than one derivation plus a checker. The generator over-approximates a
page's asset set from the templ source; `markupcheck` under-approximates it from the captured
bytes. Neither is sound alone, and merging them, or letting one import the other's table,
would make the pair vacuous while leaving both green.
