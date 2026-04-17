# Spec: l2tp-11 -- Web Management UI, CQM Graph, Disconnect Action

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-l2tp-9-observer, spec-l2tp-10-metrics |
| Phase | - |
| Updated | 2026-04-17 |

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

## Design Decisions (agreed with user, 2026-04-17)

| # | Decision |
|---|----------|
| D3 | UI, feeds, and handlers live in `internal/component/web/` (new `handler_l2tp.go`, templates, assets). uPlot vendored at `third_party/web/uplot/` via `scripts/vendor/sync_web.go`. |
| D8 | Disconnect POST routes through the CLI dispatcher as command `l2tp session disconnect <sid>`. Authz check happens once in the CLI layer. Read-only profile gets one new deny entry alongside `restart`/`kill`/`clear`. |
| D9 | Disconnect UX: confirm modal with required free-text reason field. Reason logged to the per-session event ring. |
| D11 | Chart colors distinguish `established` / `negotiating` / `down` bucket states. Tx-limit hits and packet loss render as overlay dots. |
| D12 | Chart default window: 24h (matches retention). |

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
| Disconnect action | POST `/l2tp/<sid>/disconnect` with form fields `reason` (required). Handler builds command `l2tp session disconnect <sid>` and dispatches through CLI with authenticated username |
| Authz profile update | `BuiltinReadOnlyProfile` gains a deny entry for `l2tp session disconnect` |
| HTMX patterns | Same conventions already used in `handler_admin.go` and `handler_config.go` |
| Auth | Existing `internal/component/web/auth.go` session cookie |

### Out of Scope

| Area | Location |
|------|----------|
| Ring buffers, observer, CQM sampler | `spec-l2tp-9-observer` |
| Prometheus metrics | `spec-l2tp-10-metrics` |
| Alice-LG/birdwatcher-style public API | None planned |
| Operator-configurable chart colors | Deferred (Firebrick-style customization, can wait for second request) |
| Server-rendered PNG/SVG graphs | Deferred (no concrete use-case today) |

## Required Reading

### Architecture Docs (filled during DESIGN phase)
- [ ] `docs/architecture/web.md` -- web component structure
- [ ] `plan/spec-l2tp-9-observer.md` -- ring buffer read API
- [ ] `internal/component/web/server.go` -- route registration pattern
- [ ] `internal/component/web/sse.go` -- SSE infrastructure reused for live chart updates
- [ ] `internal/component/authz/authz.go` -- read-only profile contents

### RFC Summaries
- [ ] None directly. HTMX and uPlot are library-level, not RFC protocols.

**Key insights:** (filled during DESIGN phase)

## Current Behavior (MANDATORY)

**Source files to read during DESIGN phase:**
- [ ] `internal/component/web/server.go`, `handler.go`, `handler_admin.go`, `handler_config.go`
- [ ] `internal/component/web/sse.go` -- SSE streaming pattern
- [ ] `internal/component/web/auth.go` -- authenticated username propagation
- [ ] `internal/component/authz/authz.go` -- `Authorize(username, command, isReadOnly)`
- [ ] `internal/component/cli/` -- CLI dispatcher entry point used for disconnect
- [ ] `third_party/web/htmx/`, `scripts/vendor/sync_web.go` -- vendoring pattern
- [ ] `plan/spec-l2tp-9-observer.md` -- ring buffer read API surface

**Behavior to preserve:** all existing web UI behavior (admin, config editor, CLI page). This spec is purely additive.

**Behavior to change:** `BuiltinReadOnlyProfile` gains a deny entry for `l2tp session disconnect`.

## Data Flow (MANDATORY)

### Entry Points
- HTTP GET from operator browser
- HTMX SSE connection
- HTTP POST for disconnect action

### Transformation Path
1. List page: handler reads observer state (tunnels + sessions), renders HTML template
2. Detail page: handler reads per-session state + event ring + initial 24h of buckets, renders HTML with chart container
3. Client-side JS loads JSON via GET, initialises uPlot, opens SSE for append
4. Disconnect: handler receives POST, constructs CLI command string, calls CLI dispatcher with authenticated username; dispatcher invokes authz; on Allow, subsystem tears down session; event ring records `disconnect-requested` with actor + reason

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| Web handler to observer | Ring buffer read API (per `spec-l2tp-9`) |
| Web handler to CLI dispatcher | Command string + authenticated username |
| CLI dispatcher to L2TP subsystem | Existing subsystem teardown API (needs definition; see Open Questions) |
| Browser to server (live) | SSE over existing `web/sse.go` |

### Integration Points
- `spec-l2tp-9-observer` ring buffer read API
- `spec-l2tp-10-metrics` no direct dep, but the UI cross-links to `/metrics` for operator convenience
- CLI dispatcher (existing in `internal/component/cli/`)
- `internal/component/authz/` built-in read-only profile update

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GET `/l2tp` on running daemon with active session | → | List page renders with at least one row | `test/l2tp/web-list.ci` |
| GET `/l2tp/<login>/samples` | → | Columnar JSON matching uPlot shape | `test/l2tp/web-samples-json.ci` |
| POST `/l2tp/<sid>/disconnect` with reason, admin profile | → | Session teardown triggered; `disconnect-requested` event in ring | `test/l2tp/web-disconnect-admin.ci` |
| POST `/l2tp/<sid>/disconnect`, read-only profile | → | Authz denies; 403; no teardown | `test/l2tp/web-disconnect-readonly.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Logged-in user visits `/l2tp` | List page renders with active tunnels and sessions; columns sortable |
| AC-2 | User clicks a session row | Detail page for that session renders with live state, negotiated PPP options, kernel counters, CQM chart container, event timeline |
| AC-3 | Detail page loads | JSON GET returns 24h of buckets for that login in columnar uPlot shape; chart draws correctly |
| AC-4 | New bucket lands during page view | SSE pushes bucket; chart appends in place without full reload |
| AC-5 | Chart shows a window where the session was down | Purple band covers that window; established spans use green/blue bands for min/avg/max |
| AC-6 | CSV download requested | `/l2tp/<login>/samples.csv` returns all retained buckets as CSV |
| AC-7 | User clicks Disconnect with reason | Modal requires reason; submitted POST triggers CLI dispatch; session is torn down; `disconnect-requested{actor,reason}` lands in event ring |
| AC-8 | Read-only profile user clicks Disconnect | Authz denies; UI shows error; session remains up |
| AC-9 | Operator without auth session | Redirected to login (existing web auth behavior preserved) |

## 🧪 TDD Test Plan

### Unit Tests (filled during DESIGN phase)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TBD | `internal/component/web/handler_l2tp_test.go` | List page renders expected template with observer data | |
| TBD | `internal/component/web/handler_l2tp_test.go` | Detail page includes event timeline | |
| TBD | `internal/component/web/handler_l2tp_test.go` | JSON feed shape matches uPlot expected columnar form | |
| TBD | `internal/component/web/handler_l2tp_test.go` | Disconnect POST dispatches correct CLI command string | |
| TBD | `internal/component/authz/authz_test.go` | Read-only profile denies `l2tp session disconnect` | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Disconnect reason length | 1-256 chars | 256 | 0 (empty) | 257 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TBD | `test/l2tp/web-list.ci` | List page renders active sessions | |
| TBD | `test/l2tp/web-samples-json.ci` | JSON feed returns valid shape | |
| TBD | `test/l2tp/web-samples-sse.ci` | SSE pushes new bucket | |
| TBD | `test/l2tp/web-samples-csv.ci` | CSV download | |
| TBD | `test/l2tp/web-disconnect-admin.ci` | Admin disconnect succeeds | |
| TBD | `test/l2tp/web-disconnect-readonly.ci` | Read-only disconnect denied | |

## Files to Modify

- `internal/component/web/server.go` -- register `/l2tp` routes
- `internal/component/web/auth.go` -- propagate authenticated username to CLI dispatcher call path (if not already)
- `internal/component/authz/authz.go` -- `BuiltinReadOnlyProfile` gains deny for `l2tp session disconnect`
- `scripts/vendor/sync_web.go` -- add uPlot sync target

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A (no config changes for web routes) |
| Env vars | [ ] | N/A |
| CLI command `l2tp session disconnect` | [ ] | Registered by L2TP subsystem per `spec-l2tp-7-subsystem`; needs YANG `ze:command` + handler |
| Functional tests | [ ] | `test/l2tp/web-*.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (operator web UI) |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` (`l2tp session disconnect`) |
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
- `internal/component/web/handler_l2tp_test.go`
- `internal/component/web/templates/l2tp/list.html`
- `internal/component/web/templates/l2tp/detail.html`
- `internal/component/web/assets/l2tp/` -- client-side JS glue binding uPlot to the SSE stream
- `third_party/web/uplot/` -- vendored library (pulled by `scripts/vendor/sync_web.go`)
- `test/l2tp/web-*.ci` (multiple)

## Implementation Steps

### /implement Stage Mapping (filled during DESIGN phase)

### Implementation Phases (filled during DESIGN phase)

Outline (rough):
1. Vendor uPlot; update `scripts/vendor/sync_web.go` and `third_party/web/MANIFEST.md`
2. Register `/l2tp` routes; implement list page
3. Detail page HTML scaffold + JSON feed + CSV feed
4. SSE feed wiring reusing `web/sse.go`
5. Client-side JS: uPlot init, SSE append, color bands matching bucket states
6. Event timeline rendering
7. Disconnect: CLI command registration, handler POST path, confirm modal template, read-only profile update
8. Functional tests
9. Docs: user guide page, command reference entry, dashboard migration note (cross-ref `spec-l2tp-10`)

### Critical Review Checklist (filled during DESIGN phase)

### Deliverables Checklist (filled during DESIGN phase)

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation on disconnect | `<sid>` is exactly one current session; reason length bounded |
| CSRF | Existing web pattern (session cookie + form token if already used) applies to POST |
| Authz leak | Read-only user must not see disconnect button rendered as enabled; server-side denial is the authoritative gate |
| SSE auth | SSE endpoints require the same auth session as the owning page |
| Error leakage | Session teardown errors do not leak internal state or usernames of other sessions |

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

(LIVE during DESIGN and IMPLEMENT phases)

## Open Questions (resolve during DESIGN)

- CLI command `l2tp session disconnect <sid>` registration: which file owns it (L2TP subsystem `spec-l2tp-7` or this spec)?
- Exact subsystem teardown API signature called from the CLI handler
- Whether the disconnect modal supports RADIUS Disconnect-Cause forwarding (if yes, extra field on form; if no, simple reason only)
- Chart color palette: reuse ze's existing chart palette (if any) or adopt Firebrick's directly

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
- [ ] Disconnect goes through CLI dispatcher (single authz surface)
- [ ] SSE channel reuses `web/sse.go`, no new SSE infrastructure

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
