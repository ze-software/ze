# Spec: l2tp-11 -- Web Management UI, CQM Graph, Disconnect Action

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-l2tp-9-observer, spec-l2tp-10-metrics |
| Phase | 7/7 |
| Updated | 2026-04-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-l2tp-9-observer.md` -- state source for list/detail/chart/timeline
4. `plan/spec-l2tp-10-metrics.md` -- metric shapes referenced in UI
5. `internal/component/web/server.go`, `handler_admin.go`, `handler_config.go`, `sse.go`, `auth.go` -- existing web patterns
6. `internal/component/authz/authz.go` -- authz API and built-in profiles
7. `third_party/web/htmx/` and `scripts/vendor/sync_web.go` -- vendoring precedent

## Task

Provide a web UI for L2TP session management and debugging, inspired by MPD's
status page and Firebrick's CQM graphs. Pages live under the existing ze web
component:

- **List page:** active tunnels and sessions, sortable columns, click through
  to detail.
- **Detail page:** per-session live state, LCP/IPCP/IPv6CP negotiated options,
  kernel traffic counters, an in-page CQM graph rendered client-side with
  uPlot, a timeline of events from the per-session event ring, and a
  disconnect button.
- **Disconnect action:** POST routed through the CLI dispatcher so authz gates
  at a single surface; modal requires a free-text reason; reason is logged to
  the per-session event ring.
- **Data feeds:** JSON for chart initial load, SSE for append-on-new-bucket,
  CSV for download (Firebrick parity).

## Design Decisions

| # | Decision | Date |
|---|----------|------|
| D3 | UI, feeds, and handlers live in `internal/component/web/` (new `handler_l2tp.go`, templates, assets). uPlot vendored at `third_party/web/uplot/` via `scripts/vendor/sync_web.go`. | 2026-04-17 |
| D8 | Disconnect POST routes through the CLI dispatcher as command `clear l2tp session <sid> [reason <text...>] [cause <code>]`. Authz check happens once in the CLI layer via existing `clear` verb deny in `BuiltinReadOnlyProfile`. No new authz entries needed. | 2026-04-22 |
| D9 | Disconnect UX: confirm modal with required free-text reason field and optional Disconnect-Cause dropdown. Reason + cause logged to the per-session event ring. | 2026-04-22 |
| D11 | Chart colors distinguish `established` / `negotiating` / `down` bucket states via CSS custom properties (`--color-l2tp-established`, `--color-l2tp-negotiating`, `--color-l2tp-down`). JS reads via `getComputedStyle`. Green/amber/purple defaults. Tx-limit hits and packet loss render as overlay dots. | 2026-04-22 |
| D12 | Chart default window: 24h (matches retention). | 2026-04-17 |
| D13 | Observer access: extend `l2tp.Service` interface with `SessionEvents(uint16) []ObserverEvent` and `LoginSamples(string) []CQMBucket`. Single locator, single nil check. Web handlers call `l2tp.LookupService()`. | 2026-04-22 |
| D14 | SSE for CQM chart: ticker-based per-connection handler (no `EventBroker`). Handler ticks every `BucketInterval` (100s), calls `LoginSamples`, diffs against last sent count, writes new buckets as SSE events. Same heartbeat/flusher/context pattern as `sse.go:ServeHTTP`. | 2026-04-22 |
| D15 | URL scheme: `/l2tp` as new top-level prefix. Direct mux registration in `hub/main.go`, not through `ParseURL`/`knownPrefixes`. Matches existing pattern for `/cli`, `/events`, `/gokrazy/`. | 2026-04-22 |
| D16 | Command grammar: move L2TP destructive commands from top-level `l2tp` container to augmenting `clear` verb. `clear l2tp session <sid>`, `clear l2tp tunnel <tid>`, etc. Read tree (`show l2tp ...`) unchanged. Fixes grammar violation (noun at verb level). | 2026-04-22 |
| D17 | Keyword-prefixed optional args: `reason <text...>` and `cause <code>` are parsed by keyword, not position. Unknown keywords rejected. Extensible for future keywords. | 2026-04-22 |
| D18 | ObserverEvent extended with `Actor string`, `Reason string`, `Cause uint32` fields for disconnect events. Zero-valued for all other event types. Ring is pre-allocated; no allocation cost. | 2026-04-22 |

## Scope

### In Scope

| Area | Description |
|------|-------------|
| List page | Under `/l2tp`. Active tunnels and sessions. Columns: login, sid, ifname, peer IP, uptime, state, quality (current-bucket summary). Sortable. Row click to detail |
| Detail page | Under `/l2tp/<sid>`. Live session state, LCP/IPCP/IPv6CP options, kernel byte/packet counters, embedded uPlot chart, event timeline, disconnect button |
| uPlot integration | Vendored at `third_party/web/uplot/`. Rendered client-side from JSON+SSE feeds |
| JSON feed | `/l2tp/<login>/samples?from=&to=` columnar arrays matching uPlot's native shape |
| SSE feed | `/l2tp/<login>/samples/stream` pushes one aggregated bucket per 100s |
| CSV feed | `/l2tp/<login>/samples.csv` download endpoint, same data |
| Event timeline | Rendered server-side in the detail page from the per-session event ring |
| Disconnect action | POST `/l2tp/<sid>/disconnect` with form fields `reason` (required) and optional `cause` (uint32). Handler builds command `clear l2tp session <sid> reason <text> [cause <code>]` and dispatches through CLI with authenticated username |
| YANG grammar fix | Move destructive L2TP commands from top-level `l2tp` to augmenting `clear`. `clear l2tp session/tunnel teardown/teardown-all`. Read tree unchanged |
| Observer Service extension | Add `SessionEvents` and `LoginSamples` to `l2tp.Service` interface so web handlers access observer data via `l2tp.LookupService()` |
| ObserverEvent extension | Add `Actor`, `Reason`, `Cause` fields to `ObserverEvent` for disconnect audit trail |
| HTMX patterns | Same conventions already used in `handler_admin.go` and `handler_config.go` |
| Auth | Existing `internal/component/web/auth.go` session cookie |

### Out of Scope

| Area | Reason |
|------|--------|
| Ring buffers, observer, CQM sampler | `spec-l2tp-9-observer` (done) |
| Prometheus metrics | `spec-l2tp-10-metrics` (done) |
| Alice-LG/birdwatcher-style public API | None planned |
| Operator-configurable chart colors | Deferred (Firebrick-style customization, can wait for second request) |
| Server-rendered PNG/SVG graphs | Deferred (no concrete use-case today) |
| New authz profile entries | `clear` is already denied in `BuiltinReadOnlyProfile` (entry 30) |

## Required Reading

### Architecture Docs
- [ ] `internal/component/web/server.go` -- `WebServer.HandleFunc` route registration
  -> Constraint: routes registered on `*http.ServeMux` directly, not through `ParseURL`
- [ ] `internal/component/web/sse.go` -- `EventBroker` Subscribe/Broadcast/ServeHTTP pattern
  -> Constraint: CQM SSE does NOT use EventBroker (per-login, not broadcast); reuse heartbeat/flusher/context pattern only
- [ ] `internal/component/web/handler.go` -- `ParseURL`, `knownPrefixes`, `ValidatePathSegments`
  -> Constraint: `/l2tp` routes bypass this; registered directly on mux like `/cli`, `/events`
- [ ] `internal/component/web/handler_admin.go` -- `CommandDispatcher`, `HandleAdminExecute`
  -> Constraint: web disconnect calls `dispatch("clear l2tp session <sid> reason ...")` same signature
- [ ] `internal/component/web/auth.go` -- `GetUsernameFromRequest`, session cookie, `AuthMiddleware`
  -> Constraint: actor name for disconnect audit comes from `GetUsernameFromRequest(r)`
- [ ] `internal/component/web/render.go` -- `go:embed templates`, `go:embed assets`, `Renderer`, `RenderFragment`
  -> Constraint: L2TP templates go in `templates/l2tp/`, assets in `assets/`; all embedded
- [ ] `internal/component/authz/authz.go` -- `BuiltinReadOnlyProfile` denies `restart`, `kill`, `clear`
  -> Decision: `clear l2tp session` is already denied by entry 30 (`clear` prefix match)
- [ ] `internal/component/l2tp/observer.go` -- `Observer.SessionEvents`, `Observer.LoginSamples`, `ObserverEvent`
  -> Constraint: observer methods return nil when CQM disabled; web handlers must handle gracefully
- [ ] `internal/component/l2tp/cqm.go` -- `CQMBucket`, `BucketState`, `BucketInterval` (100s)
  -> Constraint: SSE ticker interval = `BucketInterval`; JSON shape = columnar arrays matching uPlot
- [ ] `internal/component/l2tp/service_locator.go` -- `Service` interface, `LookupService()`
  -> Decision: extend interface with `SessionEvents` + `LoginSamples`
- [ ] `internal/component/l2tp/subsystem_snapshot.go` -- `Snapshot`, `TunnelSnapshot`, `SessionSnapshot`
  -> Constraint: list page data comes from `Snapshot()`; detail page from `LookupSession()`
- [ ] `internal/component/cmd/l2tp/l2tp.go` -- `handleSessionTeardown`, `parseIDArg`
  -> Decision: extend to parse `reason <text...>` and `cause <code>` keyword args
- [ ] `internal/component/cmd/l2tp/schema/ze-l2tp-cmd.yang` -- current command tree
  -> Decision: move destructive containers from top-level `l2tp` to augmenting `clear`
- [ ] `internal/component/cmd/clear/schema/ze-cli-clear-cmd.yang` -- `clear` verb container
  -> Decision: augment with `l2tp` sub-tree
- [ ] `cmd/ze/hub/main.go:925-1115` -- `startWebServer`, route registration, `authWrap`
  -> Constraint: L2TP routes registered in same block as `/admin/`, `/cli`, etc.
- [ ] `third_party/web/MANIFEST.md` -- vendored asset table
  -> Constraint: uPlot added here; `scripts/vendor/sync_web.go` copies to consumers
- [ ] `scripts/vendor/sync_web.go` -- asset sync script
  -> Constraint: add uPlot source dir + filename to `assets` slice

### RFC Summaries
- [ ] None directly. HTMX and uPlot are library-level, not RFC protocols.

**Key insights:**
- `CommandDispatcher func(command string) (string, error)` is the only interface between web and CLI; disconnect routes through it
- `BuiltinReadOnlyProfile` prefix-matches with word boundary; `clear` matches `clear l2tp session teardown` but not `show l2tp`
- Observer returns nil/empty when CQM disabled; all web handlers must render gracefully (empty chart, "no data" message)
- `BucketInterval` is 100s; SSE ticker aligns to this; JSON initial load returns up to 864 buckets (24h / 100s)
- The `Subsystem.observer` field is private; extending `Service` interface means adding delegate methods on `*Subsystem`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/server.go` (463L): TLS server, HandleFunc/Handle on mux, ListenAndServe. Key: NewWebServer, HandleFunc.
- [ ] `internal/component/web/handler.go` (231L): URL routing, ParseURL, knownPrefixes, RegisterRoutes. Key: ParseURL not used by L2TP routes.
- [ ] `internal/component/web/handler_admin.go` (293L): CommandDispatcher type, HandleAdminView/Execute, BuildAdminCommandTree. Key: dispatch signature.
- [ ] `internal/component/web/sse.go` (261L): EventBroker, Subscribe/Broadcast/ServeHTTP, heartbeat ticker. Key: ServeHTTP pattern reused for CQM SSE.
- [ ] `internal/component/web/auth.go` (310L): SessionStore, AuthMiddleware, GetUsernameFromRequest, loginHandler. Key: username from context.
- [ ] `internal/component/web/render.go` (200L): go:embed templates+assets, Renderer, NewRenderer, RenderFragment. Key: template structure.
- [ ] `internal/component/authz/authz.go` (384L): Profile, Section, Entry, BuiltinReadOnlyProfile (denies restart/kill/clear). Key: clear already denied.
- [ ] `internal/component/l2tp/observer.go` (469L): Observer, eventRing, ObserverEvent, SessionEvents, LoginSamples, RecordEvent. Key: read API for web.
- [ ] `internal/component/l2tp/cqm.go` (110L): CQMBucket, BucketState, BucketInterval=100s, sampleRing. Key: data shape for chart.
- [ ] `internal/component/l2tp/service_locator.go` (63L): Service interface, LookupService, PublishService. Key: needs SessionEvents+LoginSamples.
- [ ] `internal/component/l2tp/subsystem_snapshot.go` (207L): Snapshot, LookupTunnel/Session, TeardownSession/Tunnel. Key: list/detail data source.
- [ ] `internal/component/l2tp/snapshot.go` (100L): Snapshot, TunnelSnapshot, SessionSnapshot, ConfigSnapshot structs. Key: field shapes for templates.
- [ ] `internal/component/cmd/l2tp/l2tp.go` (384L): RPC handlers, handleSessionTeardown, parseIDArg. Key: extend for reason/cause.
- [ ] `internal/component/cmd/l2tp/schema/ze-l2tp-cmd.yang`: CLI tree with top-level `l2tp` destructive container. Key: must move to `clear`.
- [ ] `internal/component/cmd/clear/schema/ze-cli-clear-cmd.yang`: `clear` verb with `interface` sub-tree. Key: augment target.
- [ ] `cmd/ze/hub/main.go` (1200L): startWebServer, route registration, authWrap, dispatch wiring. Key: registration site for L2TP routes.
- [ ] `third_party/web/MANIFEST.md`: htmx 2.0.4, sse.js, ze.svg, swagger-ui. Key: uPlot goes here.
- [ ] `scripts/vendor/sync_web.go` (60L): asset sync from third_party/web/ to consumers. Key: add uPlot entry.

**Behavior to preserve:** all existing web UI behavior (admin, config editor, CLI page, SSE config notifications). This spec is purely additive.

**Behavior to change:**
- `ze-l2tp-cmd.yang`: destructive commands move from top-level `l2tp` container to augmenting `clear` verb
- `ze-cli-clear-cmd.yang`: gains `l2tp` sub-tree via augment from `ze-l2tp-cmd.yang`
- `l2tp.Service` interface: gains `SessionEvents` and `LoginSamples` methods
- `ObserverEvent` struct: gains `Actor`, `Reason`, `Cause` fields
- `handleSessionTeardown`: parses `reason` and `cause` keyword args, records disconnect event on Observer
- `hub/main.go startWebServer`: registers `/l2tp` routes

## Data Flow (MANDATORY)

### Entry Points
- HTTP GET `/l2tp` -- list page (operator browser)
- HTTP GET `/l2tp/<sid>` -- detail page
- HTTP GET `/l2tp/<login>/samples?from=&to=` -- JSON chart data
- HTTP GET `/l2tp/<login>/samples/stream` -- SSE chart updates
- HTTP GET `/l2tp/<login>/samples.csv` -- CSV download
- HTTP POST `/l2tp/<sid>/disconnect` -- disconnect action

### Transformation Path

- **List:** `LookupService().Snapshot()` -> render HTML table with sortable rows
- **Detail:** `LookupSession(sid)` + `SessionEvents(sid)` -> HTML with chart div + event timeline + disconnect button; client JS fetches JSON, inits uPlot, opens SSE
- **JSON:** `LoginSamples(login)` -> filter by `from`/`to` -> columnar arrays `[timestamps[], minRTT[], avgRTT[], maxRTT[], states[]]`
- **SSE:** `time.Ticker` at `BucketInterval` (100s), diff `LoginSamples` count, write new buckets as `event: bucket`; heartbeat 30s; exit on `ctx.Done()`
- **CSV:** same as JSON but `text/csv` content type
- **Disconnect:** extract `reason` + `cause` from POST -> `dispatch("clear l2tp session <sid> reason <text> [cause <code>]")` -> authz checks `clear` prefix -> RPC handler records `DisconnectRequested` event on Observer -> `TeardownSession(sid)`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web -> observer | `LookupService().SessionEvents(sid)` / `.LoginSamples(login)` | [ ] |
| Web -> snapshot | `LookupService().Snapshot()` / `.LookupSession(sid)` | [ ] |
| Web -> CLI dispatch | `dispatch("clear l2tp session ...")` -> authz -> RPC handler | [ ] |
| RPC -> Observer + Subsystem | `RecordEvent(DisconnectRequested)` then `TeardownSession(sid)` | [ ] |
| Browser -> server | SSE ticker (no EventBroker) | [ ] |

### Integration Points
- `l2tp.Service` interface -- extended with `SessionEvents` + `LoginSamples`
- `l2tp.LookupService()` -- single entry point for all web handler data access
- `CommandDispatcher` (`handler_admin.go`) -- same type used for disconnect dispatch
- `ze-l2tp-cmd.yang` augments `ze-cli-clear-cmd.yang` -- destructive commands under `clear`
- `third_party/web/uplot/` + `scripts/vendor/sync_web.go` -- vendored chart library

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GET `/l2tp` on running daemon with active session | → | List page renders with at least one row | `test/l2tp/web-list.ci` |
| GET `/l2tp/<login>/samples` | → | Columnar JSON matching uPlot shape | `test/l2tp/web-samples-json.ci` |
| POST `/l2tp/<sid>/disconnect` with reason+cause, admin profile | → | `clear l2tp session <sid> reason ... cause ...` dispatched; session torn down; `DisconnectRequested` event in ring | `test/l2tp/web-disconnect-admin.ci` |
| POST `/l2tp/<sid>/disconnect`, read-only profile | → | Authz denies (`clear` prefix); 403; no teardown | `test/l2tp/web-disconnect-readonly.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Logged-in user visits `/l2tp` | List page renders with active tunnels and sessions; columns sortable |
| AC-2 | User clicks a session row | Detail page for that session renders with live state, negotiated PPP options, kernel counters, CQM chart container, event timeline |
| AC-3 | Detail page loads | JSON GET returns 24h of buckets for that login in columnar uPlot shape; chart draws correctly |
| AC-4 | New bucket lands during page view | SSE pushes bucket; chart appends in place without full reload |
| AC-5 | Chart shows a window where the session was down | Purple band covers that window; established spans use green/blue bands for min/avg/max |
| AC-6 | CSV download requested | `/l2tp/<login>/samples.csv` returns all retained buckets as CSV |
| AC-7 | User clicks Disconnect with reason | Modal requires reason (1-256 chars), optional cause code; POST triggers `clear l2tp session <sid> reason <text> [cause <code>]` dispatch; session torn down; `DisconnectRequested{actor,reason,cause}` in event ring |
| AC-8 | Read-only profile user clicks Disconnect | Authz denies (`clear` prefix match in BuiltinReadOnlyProfile entry 30); UI shows error; session remains up |
| AC-9 | Operator without auth session | Redirected to login (existing web auth behavior preserved) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestHandleL2TPList_RendersSessions | `internal/component/web/handler_l2tp_test.go` | List page renders tunnel+session rows from mock Service | |
| TestHandleL2TPDetail_RendersTimeline | `internal/component/web/handler_l2tp_test.go` | Detail page includes event timeline from SessionEvents | |
| TestHandleL2TPSamplesJSON_ColumnarShape | `internal/component/web/handler_l2tp_test.go` | JSON response has `[timestamps[], minRTT[], avgRTT[], maxRTT[], states[]]` | |
| TestHandleL2TPSamplesJSON_FromToFilter | `internal/component/web/handler_l2tp_test.go` | `from`/`to` query params filter buckets by Start time | |
| TestHandleL2TPSamplesCSV_Format | `internal/component/web/handler_l2tp_test.go` | CSV response has header row + data rows matching bucket count | |
| TestHandleL2TPDisconnect_DispatchesCommand | `internal/component/web/handler_l2tp_test.go` | POST with reason+cause dispatches `clear l2tp session <sid> reason <text> cause <code>` | |
| TestHandleL2TPDisconnect_ReasonRequired | `internal/component/web/handler_l2tp_test.go` | POST with empty reason returns 400 | |
| TestHandleL2TPDisconnect_ReasonTooLong | `internal/component/web/handler_l2tp_test.go` | POST with 257-char reason returns 400 | |
| TestHandleSessionTeardown_ParsesReasonCause | `internal/component/cmd/l2tp/l2tp_test.go` | RPC handler parses `reason maintenance cause 6` from args | |
| TestHandleSessionTeardown_RecordsDisconnectEvent | `internal/component/cmd/l2tp/l2tp_test.go` | RPC handler records ObserverEventDisconnectRequested with actor+reason+cause | |
| TestServiceInterface_SessionEvents | `internal/component/l2tp/service_locator_test.go` | Service.SessionEvents returns snapshot from observer | |
| TestServiceInterface_LoginSamples | `internal/component/l2tp/service_locator_test.go` | Service.LoginSamples returns CQM buckets | |
| TestReadOnlyProfile_DeniesClearL2TP | `internal/component/authz/authz_test.go` | `BuiltinReadOnlyProfile` denies `clear l2tp session teardown 42` | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Disconnect reason length | 1-256 chars | 256 | 0 (empty) | 257 |
| Disconnect cause code | 0-65535 | 65535 | N/A (0 = not set) | 65536 |
| Session ID in URL | 1-65535 | 65535 | 0 | 65536 |
| from/to query param | Unix timestamp | max int64 | negative | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| web-list | `test/l2tp/web-list.ci` | GET `/l2tp` with active session returns HTML with session row | |
| web-detail | `test/l2tp/web-detail.ci` | GET `/l2tp/<sid>` returns HTML with session state + event timeline | |
| web-samples-json | `test/l2tp/web-samples-json.ci` | GET `/l2tp/<login>/samples` returns columnar JSON | |
| web-samples-csv | `test/l2tp/web-samples-csv.ci` | GET `/l2tp/<login>/samples.csv` returns CSV with headers | |
| web-disconnect-admin | `test/l2tp/web-disconnect-admin.ci` | POST `/l2tp/<sid>/disconnect` with reason dispatches teardown; event ring has disconnect entry | |
| web-disconnect-readonly | `test/l2tp/web-disconnect-readonly.ci` | POST `/l2tp/<sid>/disconnect` with read-only profile returns 403 | |
| clear-l2tp-session-reason | `test/l2tp/clear-session-reason.ci` | CLI `clear l2tp session <sid> reason test` tears down + records event | |

## Files to Modify

- `internal/component/l2tp/service_locator.go` -- extend `Service` interface with `SessionEvents` + `LoginSamples`
- `internal/component/l2tp/subsystem_snapshot.go` -- add delegate methods `SessionEvents` + `LoginSamples` on `*Subsystem`
- `internal/component/l2tp/observer.go` -- add `Actor string`, `Reason string`, `Cause uint32` fields to `ObserverEvent`; add `ObserverEventDisconnectRequested` constant
- `internal/component/cmd/l2tp/l2tp.go` -- extend `handleSessionTeardown` to parse `reason`/`cause` keywords and record disconnect event
- `internal/component/cmd/l2tp/schema/ze-l2tp-cmd.yang` -- move destructive containers from top-level `l2tp` to augmenting `clear`
- `internal/component/cmd/clear/schema/ze-cli-clear-cmd.yang` -- no change needed (augment target already exists)
- `internal/component/web/assets/style.css` -- add `--color-l2tp-established/negotiating/down` CSS custom properties
- `cmd/ze/hub/main.go` -- register `/l2tp` routes in `startWebServer`
- `scripts/vendor/sync_web.go` -- add uPlot to `assets` slice and `consumers` list
- `third_party/web/MANIFEST.md` -- add uPlot entry

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema change | [x] | `ze-l2tp-cmd.yang` -- move destructive tree to augment `clear` |
| Service interface change | [x] | `service_locator.go` -- add observer methods |
| ObserverEvent extension | [x] | `observer.go` -- add disconnect fields |
| RPC handler extension | [x] | `cmd/l2tp/l2tp.go` -- parse reason/cause, record event |
| Web route registration | [x] | `hub/main.go` -- `/l2tp` routes with authWrap |
| Vendored asset | [x] | `third_party/web/uplot/`, `sync_web.go`, `MANIFEST.md` |
| Functional tests | [x] | `test/l2tp/web-*.ci` |
| Env vars | [ ] | N/A |
| Authz profile update | [ ] | N/A -- `clear` already denied in read-only profile |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (operator web UI) |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` (`clear l2tp session/tunnel` replaces `l2tp session/tunnel teardown`) |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `docs/guide/l2tp-ui.md` (new) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/web.md` |

## Files to Create

- `internal/component/web/handler_l2tp.go` -- list, detail, JSON, SSE, CSV, disconnect handlers
- `internal/component/web/handler_l2tp_test.go` -- unit tests for all handlers
- `internal/component/web/templates/l2tp/list.html` -- list page template
- `internal/component/web/templates/l2tp/detail.html` -- detail page template with chart container, event timeline, disconnect modal
- `internal/component/web/assets/l2tp-chart.js` -- client-side JS: uPlot init, SSE append, CSS color reading, state band rendering
- `third_party/web/uplot/uPlot.min.js` -- vendored uPlot library
- `third_party/web/uplot/uPlot.min.css` -- vendored uPlot styles
- `internal/component/l2tp/service_locator_test.go` -- tests for extended Service interface (if not already exists)
- `test/l2tp/web-list.ci`
- `test/l2tp/web-detail.ci`
- `test/l2tp/web-samples-json.ci`
- `test/l2tp/web-samples-csv.ci`
- `test/l2tp/web-disconnect-admin.ci`
- `test/l2tp/web-disconnect-readonly.ci`
- `test/l2tp/clear-session-reason.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This spec + Required Reading |
| 2. Audit | Files to Modify/Create |
| 3. Implement (TDD) | Implementation Phases below |
| 4. Full verification | `make ze-verify` |
| 5-12 | Per phase below |

### Implementation Phases

**Phase 1: Foundation (Service interface + Observer extension + YANG grammar fix)**
1. Extend `Service` interface with `SessionEvents` + `LoginSamples`
2. Add delegate methods on `*Subsystem`
3. Add `ObserverEventDisconnectRequested`, `Actor`, `Reason`, `Cause` to `ObserverEvent`
4. Move YANG destructive tree from top-level `l2tp` to augmenting `clear`
5. Extend `handleSessionTeardown` to parse `reason`/`cause` keywords and record disconnect event
6. Unit tests for all above

**Phase 2: Vendor uPlot**
1. Download uPlot.min.js + uPlot.min.css to `third_party/web/uplot/`
2. Update `third_party/web/MANIFEST.md`
3. Add uPlot to `scripts/vendor/sync_web.go` assets + consumers
4. Run sync, verify assets appear in `internal/component/web/assets/`

**Phase 3: List page**
1. Create `handler_l2tp.go` with `HandleL2TPList` handler
2. Create `templates/l2tp/list.html` -- sortable table of tunnels+sessions
3. Register `GET /l2tp` route in `hub/main.go`
4. Unit test: renders session rows from mock Service

**Phase 4: Detail page + JSON/CSV feeds**
1. Add `HandleL2TPDetail` handler -- session state, event timeline, chart container
2. Create `templates/l2tp/detail.html` with chart div, timeline, disconnect button
3. Add `HandleL2TPSamplesJSON` -- columnar arrays from `LoginSamples`
4. Add `HandleL2TPSamplesCSV` -- same data as CSV
5. Register routes: `GET /l2tp/{sid}`, `GET /l2tp/{login}/samples`, `GET /l2tp/{login}/samples.csv`
6. Unit tests: JSON shape, CSV format, from/to filtering

**Phase 5: SSE feed + client JS**
1. Add `HandleL2TPSamplesSSE` -- ticker at BucketInterval, diff + write new buckets
2. Register `GET /l2tp/{login}/samples/stream`
3. Create `assets/l2tp-chart.js` -- uPlot init from JSON, SSE append, CSS color bands
4. Add `--color-l2tp-*` CSS custom properties to `style.css`

**Phase 6: Disconnect action**
1. Add `HandleL2TPDisconnect` -- validate reason (1-256), optional cause, build command, dispatch
2. Add disconnect confirm modal to `detail.html`
3. Register `POST /l2tp/{sid}/disconnect`
4. Unit tests: dispatches correct command, reason required, reason too long, cause optional

**Phase 7: Functional tests + docs**
1. Write `.ci` functional tests
2. Update `docs/features.md`, `docs/guide/command-reference.md`, `docs/guide/l2tp-ui.md`

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Completeness | AC-1 through AC-9 demonstrated |
| Correctness | JSON shape matches uPlot columnar format exactly |
| Naming | URL paths kebab-case, CSS vars kebab-case, Go handlers CamelCase |
| Data flow | Disconnect goes through CLI dispatch, not direct subsystem call |
| Rule: buffer-first | N/A (no wire encoding in this spec) |
| Rule: goroutine-lifecycle | SSE handler exits cleanly on context cancel |
| Security | CSRF on disconnect POST, reason length validation, cause range validation, SSE auth |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| List page renders sessions | `test/l2tp/web-list.ci` passes |
| Detail page with chart + timeline | `test/l2tp/web-detail.ci` passes |
| JSON feed columnar shape | `test/l2tp/web-samples-json.ci` passes |
| CSV download | `test/l2tp/web-samples-csv.ci` passes |
| Admin disconnect works | `test/l2tp/web-disconnect-admin.ci` passes |
| Read-only disconnect denied | `test/l2tp/web-disconnect-readonly.ci` passes |
| YANG grammar correct | `clear l2tp session` dispatches correctly |
| uPlot vendored and embedded | `go build` succeeds, assets served |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation on disconnect | `<sid>` uint16 1-65535; reason 1-256 chars; cause 0-65535 |
| CSRF | Disconnect POST goes through `authWrap` (session cookie); same pattern as `/admin/` POST |
| Authz | Read-only user: disconnect button hidden or disabled client-side; server-side `clear` prefix denial is authoritative |
| SSE auth | `/l2tp/<login>/samples/stream` wrapped with `authWrap`; unauthenticated = 401 |
| Error leakage | Teardown errors must not leak internal state, other usernames, or stack traces |
| Login in URL | `/l2tp/<login>/samples` -- login is user-supplied; handler must validate it matches an existing observer entry, not path-traverse |
| Reason injection | Reason text is passed as command arg; `parseIDArg` skips it; reason must not be interpreted as additional commands |

### Failure Routing

| Failure | Route To |
|---------|----------|
| HTMX SSE disconnects | Reconnect logic on client side; server idempotent |
| Chart draws wrong bucket count | DESIGN phase re-examine JSON shape vs uPlot expected form |
| Disconnect POST dispatches wrong command | IMPLEMENT phase fix; add explicit assertion test |
| Read-only user reaches disconnect | SECURITY review, authz profile audit |
| 3 fix attempts fail | STOP. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The existing `clear` verb in `BuiltinReadOnlyProfile` covers L2TP teardowns with zero authz changes. The original spec's plan to add a dedicated deny entry was unnecessary -- the grammar fix (moving to `clear` verb) solved it as a side effect.
- SSE per-login feeds are better served by a simple ticker than by the EventBroker. The broker is designed for broadcast (config change notifications to all clients). Per-login CQM data is inherently scoped; broadcasting would leak data.
- The `CommandDispatcher` signature (`func(string) (string, error)`) is the narrowest bottleneck for passing structured data. Using keyword args (`reason <text> cause <code>`) in the command string is the pragmatic choice -- extending the dispatcher signature would ripple through web, LG, MCP, and REST API consumers.
- Startup ordering prevents passing Observer directly to web handlers: web server starts before L2TP subsystem `Start()` creates the Observer. The service locator pattern (extend `Service` interface) solves this cleanly.

## Open Questions (all resolved)

| Question | Resolution | Decision # |
|----------|------------|-----------|
| CLI command registration | Reuse existing `session-teardown` RPC; dispatch key changes to `clear l2tp session` | Q1, Q8 |
| Subsystem teardown API | Existing `TeardownSession(sid)` unchanged; reason/cause handled in RPC layer | Q5 |
| RADIUS Disconnect-Cause | Included as optional `cause <code>` keyword arg | Q6 |
| Chart color palette | CSS custom properties, green/amber/purple defaults | Q7 |
| Observer access from web | Extend `Service` interface | Q2 |
| SSE model | Ticker-based per-connection, no EventBroker | Q3 |
| URL scheme | `/l2tp` top-level, direct mux registration | Q4 |
| ObserverEvent shape | Add Actor/Reason/Cause fields | Q9 |
| Authz | No changes needed; `clear` already denied in read-only profile | Q8 |

## RFC Documentation

N/A.

## Implementation Summary (filled during IMPLEMENT)

## Implementation Audit (filled during IMPLEMENT)

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate (filled during IMPLEMENT)

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Pre-Commit Verification (filled during IMPLEMENT)

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table filled with concrete test names
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] uPlot vendored and built into the binary via embed
- [ ] Read-only profile deny entry added and test-covered

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] One concern per file (handler, templates, client JS)
- [ ] No runtime fetch of third-party assets (all vendored, per `third_party/web/` pattern)
- [ ] Disconnect goes through CLI dispatcher via `clear l2tp session` (single authz surface)
- [ ] SSE uses ticker-per-connection pattern (not EventBroker)
- [ ] YANG destructive commands under `clear` verb (not top-level noun)
- [ ] Observer access via extended `Service` interface (not direct struct access)

### TDD
- [ ] Tests written first
- [ ] Tests FAIL initially
- [ ] Tests PASS after implementation
- [ ] Boundary test on reason length
- [ ] Authz denial test covers read-only profile

### Completion (BLOCKING before commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary at `plan/learned/NNN-l2tp-11-web.md`
- [ ] Summary in same commit as code
