# Spec: web-6 -- Live Updates

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-3-config-edit |
| Phase | 1/3 |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-web-0-umbrella.md` -- umbrella spec (D-10 live updates, D-19 partial updates)
4. `internal/chaos/web/sse.go` -- existing SSE broker pattern to reuse
5. `internal/component/web/server.go` -- web server route registration
6. `internal/component/web/handler_config.go` -- config commit handler (broadcast trigger point)

## Task

Add live update capabilities to the Ze web interface. This covers two distinct mechanisms:

1. **SSE config change notifications** -- when any session (web or CLI) commits configuration, all connected web sessions receive a non-intrusive notification banner with a reason string and a "Refresh" button. The notification does not auto-refresh the page or steal focus. Users may click "Refresh" to reload their current view with fresh data, or dismiss the banner.

2. **HTMX auto-poll for monitor elements** -- operational/monitor elements (peer status, counters) use `hx-trigger="every Ns"` to auto-refresh independently at configurable intervals.

The SSE broker pattern is reused from the chaos dashboard (`internal/chaos/web/sse.go`). The SSE client connection uses the HTMX SSE extension already embedded in assets from web-1 (foundation).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `plan/spec-web-0-umbrella.md` -- umbrella spec, design decisions D-10, D-19
  -> Decision: D-10 defines three behaviors: monitor auto-poll, config change notification (banner + refresh), navigation always fresh
  -> Constraint: D-19 specifies SSE config change notification targets `#notification` via OOB swap
- [ ] `docs/architecture/chaos-web-dashboard.md` -- existing HTMX/SSE patterns in chaos dashboard
  -> Decision: reuse SSE broker pattern from chaos dashboard
  -> Constraint: SSEBroker has Subscribe/Unsubscribe/Broadcast/Close/ServeHTTP -- same interface needed
- [ ] `docs/architecture/core-design.md` -- overall architecture
  -> Constraint: web UI is a component, not a plugin

### RFC Summaries (MUST for protocol work)
- Not applicable -- SSE is an HTTP mechanism, not a BGP protocol feature.

**Key insights:**
- Chaos dashboard SSEBroker provides proven Subscribe/Unsubscribe/Broadcast/Close/ServeHTTP pattern
- SSE events are typed (`event: <type>\n`) with data payload (`data: <content>\n\n`)
- SSE data payload is pre-rendered HTML from notification_banner.html template with hx-swap-oob targeting #notification. Not plain text. Server renders via `html/template` (auto-escapes reason text) and broadcasts the pre-rendered HTML
- SSE endpoint `/events` requires valid session cookie. Browser sends cookie automatically with EventSource connections. No custom header needed
- HTMX SSE extension handles automatic reconnection on disconnect
- HTMX OOB swaps allow SSE events to update the notification area without touching the main content
- Config commits from CLI or web both need to trigger SSE broadcast. Editor needs an OnCommit callback (or hook on ReloadNotifier chain). Web component sets this callback when creating Editors. CLI commits through the same Editor trigger the callback, which calls broker.Broadcast() with the rendered notification HTML
- Monitor auto-poll is purely template-driven via `hx-trigger="every Ns"` -- no server-side SSE needed

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/chaos/web/sse.go` -- SSEBroker with Subscribe, Unsubscribe, Broadcast, Close, ServeHTTP. Uses mutex-guarded client map with buffered channels (cap 64). Non-blocking broadcast (drops if buffer full). ServeHTTP sets text/event-stream headers, registers client, streams events until disconnect or close
  -> Constraint: proven pattern -- replicate structure for web component SSE broker
- [ ] `internal/chaos/web/sse_test.go` -- tests for broker registration, broadcast, cleanup, HTTP handler
  -> Constraint: follow same test patterns for web SSE broker tests
- [ ] `internal/component/web/server.go` -- web server setup, route registration, middleware
  -> Constraint: SSE endpoint must be registered alongside existing routes
- [ ] `internal/component/web/handler_config.go` -- config editing handlers including commit
  -> Constraint: commit handler is the trigger point for SSE broadcast
- [ ] `internal/component/web/templates/layout.html` -- page layout frame with notification area
  -> Constraint: layout must include SSE connection attribute for HTMX extension
- [ ] `internal/component/web/templates/notification.html` -- notification area template
  -> Constraint: must support dynamic banner insertion via SSE OOB swap

**Behavior to preserve:**
- Existing chaos dashboard SSE broker unchanged (separate broker instance)
- Web server route registration patterns unchanged
- Config commit handler behavior unchanged (SSE broadcast is additive)
- Notification area template structure unchanged (SSE adds content, does not replace existing)
- HTMX partial update patterns unchanged (SSE notification uses standard OOB swap)

**Behavior to change:**
- None -- all existing behavior preserved. Live updates add new capabilities to the existing web infrastructure

## Data Flow (MANDATORY -- see `rules/data-flow-tracing.md`)

### Entry Point
- **SSE connection:** Browser sends GET `/events` with session cookie to establish long-lived SSE stream. Browser sends cookie automatically with EventSource connections -- no custom header needed. HTMX SSE extension manages the connection lifecycle (open, reconnect on drop)
- **Config commit (trigger):** User A commits via web POST or CLI SSH session. Commit handler calls SSE broker broadcast with reason text
- **Monitor auto-poll:** Browser element with `hx-trigger="every Ns"` sends periodic GET to its own endpoint. No SSE involvement

### Transformation Path

1. **SSE connection establishment:** Browser GET `/events` -> web server routes to SSE broker ServeHTTP -> broker subscribes client -> sets Content-Type: text/event-stream -> enters event loop
2. **Commit triggers notification:** User commits (web or CLI) -> commit handler renders notification_banner.html template with reason text via `html/template` (auto-escapes) -> calls `broker.Broadcast()` with event type "config-change" and the pre-rendered HTML as data payload -> broker iterates all subscribed clients -> writes SSE-formatted event to each client channel
3. **Browser receives SSE event:** HTMX SSE extension receives event -> SSE data payload is pre-rendered HTML from notification_banner.html template (rendered server-side via `html/template` which auto-escapes reason text) with hx-swap-oob targeting #notification -> browser performs OOB swap into `#notification` area. Reference D-24 from umbrella
4. **User clicks Refresh:** HTMX issues GET to current URL path -> server returns fresh content -> content area swaps -> notification banner dismissed
5. **User clicks Dismiss:** Client-side handler removes notification banner from DOM. No server request
6. **Monitor auto-poll:** HTMX timer fires -> GET to element-specific endpoint -> server returns fresh HTML fragment -> element swaps in place

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser <-> SSE endpoint | text/event-stream over HTTPS with session cookie, HTMX SSE extension manages connection | [ ] |
| Commit handler <-> SSE broker | Direct in-process function call (broker.Broadcast) | [ ] |
| CLI commit <-> SSE broker | Commit event hook calls broker.Broadcast (same in-process path) | [ ] |
| SSE event <-> HTMX DOM | HTMX SSE extension parses event, OOB swap into #notification | [ ] |
| Monitor element <-> server | Standard HTMX GET with hx-trigger polling | [ ] |

### Integration Points
- `SSEBroker` -- new broker instance in web component, same pattern as chaos `SSEBroker`
- `handler_config.go` commit handler -- after successful commit, calls broker.Broadcast with pre-rendered notification HTML
- `layout.html` -- adds `hx-ext="sse"` and `sse-connect="/events"` attributes for HTMX SSE extension
- `notification.html` -- supports OOB swap target for SSE-delivered notification banners
- CLI/SSH commit path -- Editor needs an OnCommit callback (or hook on ReloadNotifier chain). Web component sets this callback when creating Editors. CLI commits through the same Editor trigger the callback, which calls broker.Broadcast() with the rendered notification HTML

### Architectural Verification
- [ ] No bypassed layers (SSE events flow through broker, not direct writes)
- [ ] No unintended coupling (SSE broker is self-contained, commit handler has minimal coupling via Broadcast call)
- [ ] No duplicated functionality (reuses chaos SSEBroker pattern, does not reinvent)
- [ ] Zero-copy preserved where applicable (SSE payloads are small text strings, not wire data)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation -- unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| CLI user commits config | -> | SSE broker broadcasts to web sessions | `test/plugin/web-live-notify.ci` |
| Monitor element with hx-trigger polling | -> | Server returns fresh data on timer-driven GET | `test/plugin/web-live-poll.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET `/events` from browser | SSE connection established (Content-Type: text/event-stream, status 200) |
| AC-2 | User A commits via web while User B has SSE connection | User B receives SSE event with type "config-change" and reason text |
| AC-3 | CLI user commits via SSH while web session has SSE connection | Web session receives SSE event with type "config-change" and reason text |
| AC-4 | SSE event received by browser | Notification banner renders in notification area with reason text |
| AC-5 | Notification banner displayed | Banner includes "Refresh" button that reloads current view content |
| AC-6 | Notification banner displayed | Banner includes dismiss/close option to hide without refreshing |
| AC-7 | SSE event received by browser | Page does NOT auto-refresh and focus is NOT stolen from user |
| AC-8 | User clicks "Refresh" on notification banner | Browser fetches fresh content for the current URL path via HTMX GET |
| AC-9 | Monitor element with `hx-trigger="every 5s"` | Element auto-refreshes independently at the specified interval |
| AC-10 | SSE connection drops (network interruption) | HTMX SSE extension automatically reconnects |
| AC-11 | Multiple browsers connected via SSE, one commit occurs | All connected browsers receive the same notification event |
| AC-12 | SSE event sent from broker | Event includes `event: config-change` type and `data:` payload with pre-rendered HTML from notification_banner.html template with hx-swap-oob targeting #notification |
| AC-13 | SSE broadcast timing | SSE broadcast occurs only AFTER CommitSession() returns successfully. Failed commits do not trigger broadcast |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSSEBrokerRegistration` | `internal/component/web/sse_test.go` | Client subscribes to broker and receives a channel for events | |
| `TestSSEBrokerBroadcast` | `internal/component/web/sse_test.go` | Broadcast sends event to all registered clients | |
| `TestSSEBrokerCleanup` | `internal/component/web/sse_test.go` | Disconnected client is removed from broker, no longer receives events | |
| `TestSSEEventFormat` | `internal/component/web/sse_test.go` | SSE event formatted as `event: config-change\ndata: <pre-rendered HTML>\n\n` where data is notification_banner.html output | |
| `TestCommitTriggersSSE` | `internal/component/web/handler_config_test.go` | Commit handler calls broker Broadcast with reason string containing username | |
| `TestNotificationBannerData` | `internal/component/web/sse_test.go` | Notification banner template data includes reason text, refresh URL, and dismiss action | |
| `TestMonitorAutoPolling` | `internal/component/web/handler_config_test.go` | Monitor element template output includes `hx-trigger="every Ns"` attribute | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SSE client buffer capacity | 1-256 | 256 | 0 | N/A (capped) |
| Auto-poll interval seconds | 1-3600 | 3600 | 0 | N/A (capped) |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests -- unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-live-notify` | `test/plugin/web-live-notify.ci` | CLI user commits config, web SSE client receives notification event | |
| `web-live-poll` | `test/plugin/web-live-poll.ci` | Monitor element with auto-poll attribute returns fresh data on timed request | |

### Future (if deferring any tests)
- None -- all tests planned for this spec

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file -- if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `internal/component/web/server.go` -- register `/events` SSE endpoint route, instantiate SSE broker, wire broker to commit event hook
- `internal/component/web/handler_config.go` -- after successful commit, call `broker.Broadcast()` with event type "config-change" and reason including username
- `internal/component/web/templates/layout.html` -- add `hx-ext="sse"` and `sse-connect="/events"` attributes to enable HTMX SSE extension on page load
- `internal/component/web/templates/notification.html` -- add OOB swap target support for SSE-delivered notification banners
- `internal/component/cli/editor.go` -- add OnCommit callback field, called after successful CommitSession()

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A -- SSE is HTTP endpoint, not an RPC |
| CLI commands/flags | No | N/A -- no new CLI commands |
| Editor autocomplete | No | N/A -- no config changes |
| Functional test for new RPC/API | Yes | `test/plugin/web-live-notify.ci`, `test/plugin/web-live-poll.ci` |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add live update notifications and monitor auto-poll |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A -- SSE endpoint is not an RPC |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` -- add section on live notifications and auto-poll behavior |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- add web live updates capability |
| 12 | Internal architecture changed? | No | N/A -- follows existing SSE broker pattern |

## Files to Create
- `internal/component/web/sse.go` -- SSE broker for the web component. Follows chaos dashboard SSEBroker pattern: client registration with mutex-guarded map, buffered channels, non-blocking broadcast, ServeHTTP handler with text/event-stream headers. Broadcasts pre-rendered HTML (from notification_banner.html). Adds commit hook integration so that both web and CLI commits trigger broadcast to all connected clients
- `internal/component/web/sse_test.go` -- unit tests for web SSE broker (registration, broadcast, cleanup, event format)
- `internal/component/web/templates/notification_banner.html` -- template for config change notification banner. Server renders this via `html/template` (auto-escapes reason text) and broadcasts the pre-rendered HTML as SSE data payload with hx-swap-oob targeting #notification. Contains reason text, "Refresh" button (HTMX GET to current path targeting content area), and dismiss/close button (client-side DOM removal, no server request)
- `test/plugin/web-live-notify.ci` -- functional test: start ze with web config, simulate CLI commit, verify SSE event delivered to connected client
- `test/plugin/web-live-poll.ci` -- functional test: start ze with web config, request monitor element with auto-poll, verify periodic refresh returns fresh data

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against -- they MUST be filled with feature-specific items. -->

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: SSE broker** -- implement the SSE broker for the web component
   - Tests: `TestSSEBrokerRegistration`, `TestSSEBrokerBroadcast`, `TestSSEBrokerCleanup`, `TestSSEEventFormat`
   - Files: `internal/component/web/sse.go`, `internal/component/web/sse_test.go`
   - Verify: tests fail -> implement broker with Subscribe, Unsubscribe, Broadcast, Close, ServeHTTP -> tests pass

2. **Phase: Commit hook integration** -- wire commit handler to broadcast SSE events
   - Tests: `TestCommitTriggersSSE`
   - Files: `internal/component/web/handler_config.go`, `internal/component/web/server.go`
   - Verify: tests fail -> add broker instantiation in server, broadcast call in commit handler -> tests pass

3. **Phase: Notification banner template** -- create the notification banner and wire SSE into layout
   - Tests: `TestNotificationBannerData`
   - Files: `internal/component/web/templates/notification_banner.html`, `internal/component/web/templates/layout.html`, `internal/component/web/templates/notification.html`
   - Verify: tests fail -> implement banner template with reason, refresh, dismiss; add SSE attributes to layout -> tests pass

4. **Phase: Monitor auto-poll** -- add hx-trigger polling attributes to monitor elements
   - Tests: `TestMonitorAutoPolling`
   - Files: `internal/component/web/handler_config.go` (or relevant monitor template)
   - Verify: tests fail -> add hx-trigger="every Ns" to monitor element templates -> tests pass

5. **Functional tests** -- create after feature works. Cover user-visible behavior.
   - Tests: `test/plugin/web-live-notify.ci`, `test/plugin/web-live-poll.ci`
   - Verify: functional tests pass end-to-end

6. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

7. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-web-6-live-updates.md`, delete spec from `plan/`. Summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | SSE event format: `event: config-change\ndata: <pre-rendered HTML>\n\n`. Data payload is notification_banner.html rendered via `html/template`, not plain text. Notification does NOT auto-refresh or steal focus. Broadcast only after successful CommitSession() |
| Naming | SSE event type uses kebab-case (`config-change`). Template file names follow existing web template naming |
| Data flow | Commit handler (web and CLI) -> broker.Broadcast -> SSE client channels -> browser. No bypass of broker |
| Rule: no-layering | No duplicate SSE broker (web broker is separate instance, not wrapping chaos broker) |
| Rule: goroutine-lifecycle | SSE broker uses per-connection goroutine (lifecycle-scoped in ServeHTTP), not per-event |
| Rule: buffer-first | Not applicable -- SSE payloads are small text strings |
| Broker cleanup | Disconnected clients removed from broker map. No goroutine leak on disconnect |
| OOB swap correctness | SSE event data is pre-rendered HTML from notification_banner.html containing `hx-swap-oob` attribute targeting `#notification` |
| Dismiss behavior | Dismiss button removes banner client-side only. No server request on dismiss |
| Refresh behavior | Refresh button issues HTMX GET to current path, swaps content area |
| CLI commit path | CLI/SSH commits also trigger SSE broadcast (not only web commits) |

### Deliverables Checklist (/implement stage 9)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| SSE broker file exists | `ls internal/component/web/sse.go` |
| SSE broker tests exist | `ls internal/component/web/sse_test.go` |
| Notification banner template exists | `ls internal/component/web/templates/notification_banner.html` |
| `/events` route registered | grep for `/events` or `events` in `internal/component/web/server.go` |
| Commit handler calls Broadcast | grep for `Broadcast` in `internal/component/web/handler_config.go` |
| Layout has SSE connection attribute | grep for `sse-connect` in `internal/component/web/templates/layout.html` |
| Functional test: notify | `ls test/plugin/web-live-notify.ci` |
| Functional test: poll | `ls test/plugin/web-live-poll.ci` |
| Banner has Refresh button | grep for `Refresh` or `refresh` in notification_banner.html |
| Banner has dismiss option | grep for `dismiss` or `close` in notification_banner.html |
| Monitor element has hx-trigger | grep for `hx-trigger` with `every` in relevant template |

### Security Review Checklist (/implement stage 10)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | SSE endpoint requires valid session cookie (auth middleware applies to `/events` route). Browser sends cookie automatically with EventSource connections. No custom header needed. Unauthenticated clients must not receive config change events |
| Resource exhaustion | SSE broker limits client buffer size (bounded channel). Slow clients have events dropped (non-blocking send), not queued unboundedly |
| Connection limits | Consider maximum concurrent SSE connections. Broker should not allow unlimited client registrations without bounds |
| Event data injection | Reason text is auto-escaped by `html/template` during server-side rendering of notification_banner.html. The SSE data payload is pre-rendered HTML, not raw text that the browser templates |
| Information leakage | SSE events should not include sensitive config data -- only the fact that a commit occurred and by whom |
| Denial of service | SSE endpoint should be protected against connection floods. Leverage existing web server rate limiting if available |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
<!-- LIVE -- write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem -> arch doc, process -> rules, knowledge -> memory.md -->

## RFC Documentation

Not applicable -- SSE is an HTTP mechanism, not a BGP protocol feature. No RFC comments needed.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

### Files Exist (ls)
<!-- For EVERY file in "Files to Create": ls -la <path> -- paste output. -->
<!-- For EVERY .ci file in Wiring Test and Functional Tests: ls -la <path> -- paste output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit -- re-check. -->
<!-- Acceptable evidence: test name + pass output, grep showing function call, ls showing file. -->
<!-- NOT acceptable: "already checked", "should work", reference to audit table above. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the .ci test exist AND does it exercise the full path? -->
<!-- Read the .ci file content. Does it actually test what the wiring table claims? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-web-6-live-updates.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
