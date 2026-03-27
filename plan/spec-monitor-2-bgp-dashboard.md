# Spec: monitor-2-bgp-dashboard

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-03-27 |

**Spec set:** `plan/learned/396-bgp-monitor.md` (completed sibling), `spec-monitor-2-bgp-dashboard.md` (this).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/cmd/peer/summary.go` - summary RPC response format
4. `internal/component/bgp/plugins/cmd/peer/peer.go:118-177` - peer-detail RPC response format
5. `internal/component/cli/model_monitor.go` - streaming mode pattern (factory + session + poll)
6. `internal/component/cli/model.go` - Model struct fields, Update() dispatch, message types
7. `internal/component/bgp/plugins/cmd/monitor/monitor.go:46-56` - `bgp monitor` redirect to replace

## Task

Implement `bgp monitor` as a live peer status dashboard in the TUI. The dashboard auto-refreshes every 2 seconds, showing a header bar with router identity and a sortable, color-coded peer table with update rates. Users navigate with keyboard shortcuts and can drill into peer details. Esc exits back to the CLI.

**Data transport tradeoff:** the dashboard polls via `commandExecutor("bgp summary")` which creates an SSH exec channel per poll (every 2s). This is pragmatic (no new RPCs, reuses existing infrastructure) but adds per-poll latency vs a persistent streaming connection. Acceptable for a 2s refresh cycle. If latency becomes a problem, a streaming variant can be added later using the streaming handler registry (implemented in `plan/learned/396-bgp-monitor.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - command dispatch and streaming
  → Constraint: SSH exec is the transport, commands return JSON
  → Decision: dashboard polls via existing `ze-bgp:summary` RPC
- [ ] `docs/architecture/api/process-protocol.md` - plugin server architecture
  → Constraint: RPCs return `plugin.Response` with `Data` as `map[string]any`

### Learned Summaries
- [ ] `plan/learned/396-bgp-monitor.md` - original monitor implementation (spec-monitor-1 completed)
  → Decision: TUI streaming uses MonitorSession pattern (factory + channel + poll tick)
  → Decision: `bgp monitor` was renamed to `event monitor`; `bgp monitor` is a redirect placeholder
  → Constraint: Bubble Tea message-driven event loop

**Key insights:**
- `ze-bgp:summary` RPC already returns all needed data: router-id, local-as, uptime, peers-configured, peers-established, per-peer address/remote-as/state/uptime/updates-received/updates-sent/keepalives-received/keepalives-sent/eor-received/eor-sent
- `ze-bgp:peer-detail` RPC returns extended info keyed by peer IP: remote-as, local-as, router-id, timer (sub-object with receive-hold-time/send-hold-time/connect-retry as int seconds), connect (bool), accept (bool), state, uptime, counters, optional name/group/local-ip/prefix-updated/prefix-stale
- Rate computation must be client-side (diff counters between polls)
- Monitor session pattern (factory + channel + poll tick) can be adapted for dashboard
- `bgp monitor` is currently a streaming handler redirect (prints error, tells user to use `event monitor`). Must be replaced with dashboard entry, not layered on top
- `model.go` is 1334 lines -- minimize additions there, put all dashboard logic in new files

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - handleBgpSummary returns JSON with per-peer stats, handleBgpPeerStatistics computes rates from counters/uptime
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - handleBgpPeerList returns brief peer list, handleBgpPeerDetail returns extended info, filterPeersBySelector matches by IP or name
- [ ] `internal/component/cli/model_monitor.go` - MonitorSession pattern: factory creates session, 50ms poll tick drains channel, Esc stops, outputBuf accumulates lines
- [ ] `internal/component/cli/model_render.go` - View() layout: viewport (fills screen minus 3) + message area (2 lines) + prompt (1 line)
- [ ] `internal/component/cli/model_mode.go` - ModeEdit/ModeCommand switching with per-mode state persistence
- [ ] `internal/component/cli/model.go` - Model struct, Bubble Tea Update/View, message types

**Behavior to preserve:**
- `ze-bgp:summary` RPC response format unchanged
- `ze-bgp:peer-detail` RPC response format unchanged
- Existing TUI modes (edit, command) work unchanged
- Monitor session cleanup pattern (context cancel + deferred cleanup)

**Behavior to change:**
- `bgp monitor` currently redirects to `event monitor` with an error message (see `monitor.go:46-56`). Replace this redirect with dashboard mode entry
- New TUI view: custom-rendered dashboard bypasses the single-string viewport for a fixed-layout screen

**Rendering approach:** Bubble Tea's `viewport.Model` renders a single scrollable string. The dashboard needs fixed header (2 lines), scrollable peer table, and fixed footer (1 line). The View() method must render the full screen directly using lipgloss (not through the viewport widget). The header and footer are rendered at fixed positions. The peer table occupies the middle, with manual scroll offset tracking when peers exceed available rows. This is the same pattern used by help overlays -- direct screen composition, not viewport delegation

## Data Flow (MANDATORY)

### Entry Point
- User types `bgp monitor` in CLI (command mode or edit mode via `run bgp monitor`)
- Currently `bgp monitor` is caught by `isMonitorCommand()` at `model.go:803` and `:885` (registered as streaming handler in `monitor.go:47`). The streaming handler prints a redirect error. Dashboard entry requires: (1) remove the `bgp monitor` streaming handler registration from `monitor.go`, (2) add dashboard detection in `model.go` before the `isMonitorCommand` check (or as a separate dashboard-specific check)

### Transformation Path
1. CLI creates DashboardSession via DashboardFactory
2. DashboardFactory starts background polling: calls `commandExecutor("bgp summary")` every N seconds
3. JSON response parsed into peer data structure
4. Client-side rate computation: diff update counters between consecutive polls
5. Render header bar + peer table + footer into viewport
6. On each poll tick, replace viewport content (not append)
7. Key events update sort column, selected peer, or enter detail view
8. Esc cancels context, cleans up session, returns to CLI

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI to daemon | `commandExecutor("bgp summary")` via SSH exec | [ ] |
| CLI to daemon | `commandExecutor("peer <ip> detail")` for detail view via SSH exec | [ ] |
| Dashboard state to viewport | Render formatted table string, call setViewportText | [ ] |

### Integration Points
- `commandExecutor` - existing SSH exec function for RPC calls
- `setViewportText` - existing viewport content setter
- ModeCommand - dashboard entered from command mode
- lipgloss - direct screen composition for color-coded rendering (no existing table formatter to reuse)

### Architectural Verification
- [ ] No bypassed layers (uses existing command executor for data)
- [ ] No unintended coupling (dashboard is a TUI concern, no engine changes)
- [ ] No duplicated functionality (reuses existing summary RPC)
- [ ] Zero-copy preserved where applicable (N/A -- TUI rendering)

## Wiring Test (MANDATORY)

| Entry Point | via | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp monitor` CLI command | DashboardFactory + command executor | Dashboard session + render | `test/plugin/bgp-monitor-dashboard.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bgp monitor` in CLI | Enters dashboard mode: header bar + peer table displayed |
| AC-2 | Dashboard active, 2 seconds pass | Peer table refreshes with updated data |
| AC-3 | Dashboard active, peer state changes | State column updates, color changes accordingly |
| AC-4 | Press `s` in dashboard | Sort column cycles: Address, ASN, State, Uptime, Rx, Tx, Rate |
| AC-5 | Press `S` in dashboard | Sort direction reverses (ascending/descending) |
| AC-6 | Press `j`/Down in dashboard | Selection moves to next peer |
| AC-7 | Press `k`/Up in dashboard | Selection moves to previous peer |
| AC-8 | Press Enter on selected peer | Detail view shows extended peer info |
| AC-9 | Press Esc in detail view | Returns to peer table |
| AC-10 | Press Esc in peer table | Exits dashboard, returns to CLI |
| AC-11 | Dashboard active, peer list changes between polls | Selection preserved by peer address (not index) |
| AC-12 | Dashboard header | Shows: AS number, router-id, uptime, peers established/total |
| AC-13 | Rate column | Displays updates/sec computed from counter diffs between polls |
| AC-14 | Zero peers configured | Dashboard shows header with `peers 0/0` and empty table body with "no peers configured" message |
| AC-15 | Counter decrease (peer restart) | Rate shows `--`, baseline reset. No negative rate displayed |
| AC-16 | Poll failure | Status line shows error, last good data preserved, retry on next interval |
| AC-17 | Terminal < 80 cols | Low-priority columns dropped, display remains readable |

## Dashboard Layout

### Header (2 lines)

| Line | Content |
|------|---------|
| 1 | `AS <N>  rid <IP>  up <duration>  peers <established>/<total>` |
| 2 | Status: green "connected" or red error message if poll fails |

### Peer Table (fills remaining space minus footer)

| Column | Source | Sortable | Format |
|--------|--------|----------|--------|
| Peer | `address` | Yes | IP address |
| ASN | `remote-as` | Yes | Integer |
| State | `state` | Yes | Color-coded: green=established, yellow=connecting/active, red=stopped/idle |
| Uptime | `uptime` | Yes | Duration (e.g., `2h30m`) |
| Rx | `updates-received` | Yes | Integer (with thousands separator for large values) |
| Tx | `updates-sent` | Yes | Integer (with thousands separator) |
| Rate | computed client-side | Yes | `N.N/s` (updates received per second, computed from counter diffs) |

Sort indicator in header: current sort column marked with arrow (up/down).
Selected row highlighted (e.g., cyan background + bold).

### Footer (1 line)

| Left | Right |
|------|-------|
| `q Quit  s Sort  j/k Navigate  Enter Detail  Esc Back` | `Last update: Ns ago` |

### Detail View (replaces table on Enter)

Shows extended peer info from `ze-bgp:peer-detail` RPC. The detail view auto-refreshes on the same 2s poll interval (calls `peer <ip> detail` via commandExecutor). This keeps counters live while viewing.

| Field | Source | Notes |
|-------|--------|-------|
| Neighbor | `address` (map key) | |
| Remote ASN | `remote-as` | |
| Local ASN | `local-as` | |
| State | `state` | Color-coded |
| Uptime | `uptime` | Duration string |
| Router ID | `router-id` | Dotted-quad IP |
| Recv Hold Time | `timer.receive-hold-time` | Seconds (int) |
| Send Hold Time | `timer.send-hold-time` | Seconds (int) |
| Connect Retry | `timer.connect-retry` | Seconds (int) |
| Connect | `connect` | Bool: initiates outbound |
| Accept | `accept` | Bool: accepts inbound |
| Local IP | `local-ip` | Optional, only if valid |
| Updates Rx | `updates-received` | |
| Updates Tx | `updates-sent` | |
| Keepalives Rx | `keepalives-received` | |
| Keepalives Tx | `keepalives-sent` | |
| EOR Rx | `eor-received` | |
| EOR Tx | `eor-sent` | |
| Update Rate | computed | Client-side updates/sec |
| Name | `name` | Optional |
| Group | `group` | Optional |

Esc or Backspace returns to peer table. If the peer disappears while in detail view, auto-return to peer table with status message "peer disconnected".

## Keyboard Shortcuts

| Key | Context | Action |
|-----|---------|--------|
| `q` or `Ctrl-C` | Any | Quit dashboard |
| `Esc` | Detail view | Return to peer table |
| `Esc` | Peer table | Exit dashboard, return to CLI |
| `s` | Peer table | Cycle sort column forward |
| `S` | Peer table | Reverse sort direction |
| `j` or Down | Peer table | Select next peer |
| `k` or Up | Peer table | Select previous peer |
| `Enter` | Peer table | Show detail for selected peer |
| `?` | Any | Toggle help overlay |

## Rate Computation

Rate is computed client-side from consecutive polls:

| Step | Detail |
|------|--------|
| Store | On each poll, save per-peer `updates-received` counter and poll timestamp |
| Compute | On next poll: `rate = (current - previous) / elapsed_seconds` |
| Display | Format as `N.N/s` (one decimal place) |
| First poll | Rate is `--` (no previous data yet) |
| Peer disappeared | Remove from previous counters map |
| Peer reappeared | First poll after reappear shows `--` |
| Counter decrease | If current counter < previous (peer restarted, counter reset), show `--` and reset baseline. Do NOT compute negative rate |
| Short interval | If elapsed < 0.5s (jitter, catch-up), skip rate computation, keep previous rate. Prevents artificial spikes |
| Daemon unreachable | Show "disconnected" in status line (not generic error), keep last good data and rates frozen |

## Terminal Width Handling

| Terminal Width | Behavior |
|----------------|----------|
| >= 100 cols | All columns shown at full width |
| 80-99 cols | Truncate peer description/name column first, then compact Uptime to short form |
| < 80 cols | Drop Rate column, then Tx column. Minimum: Peer + ASN + State + Uptime |

Column priority (last dropped first): Rate, Tx, Rx, Uptime, State, ASN, Peer (never dropped).

Peer addresses are never truncated. ASN is never truncated. State may be abbreviated (e.g., "estab", "conn", "stop") at narrow widths.

## Color Scheme

| State | Color |
|-------|-------|
| `established` | Green |
| `connecting`, `active`, `opensent`, `openconfirm` | Yellow |
| `stopped`, `idle` | Red |
| Selected row | Cyan highlight + bold |
| Header text | White |
| Footer text | Dim gray |
| Error status | Red |

## Polling and Refresh

| Parameter | Value |
|-----------|-------|
| Data refresh interval | 2 seconds |
| Key input poll | 50ms (Bubble Tea default) |
| Poll command | `bgp summary` via commandExecutor |
| Detail poll command | `peer <ip> detail` via commandExecutor (also on 2s interval). Response is `{"peers":{"<ip>":{...}}}` keyed by peer address |
| Poll failure (command error) | Show error in status line, keep last good data, retry on next interval |
| Poll failure (SSH unreachable) | Show "disconnected" in status line (red), freeze display, retry on next interval |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDashboardParseSnapshot` | `internal/component/cli/model_dashboard_test.go` | JSON summary response parsed into dashboard state | |
| `TestDashboardRateComputation` | `internal/component/cli/model_dashboard_test.go` | Counter diff rate calculation, first-poll shows `--` | |
| `TestDashboardRateCounterDecrease` | `internal/component/cli/model_dashboard_test.go` | Counter decrease (peer restart) shows `--`, resets baseline | |
| `TestDashboardRateShortInterval` | `internal/component/cli/model_dashboard_test.go` | Elapsed < 0.5s skips rate computation, keeps previous rate | |
| `TestDashboardSortPeers` | `internal/component/cli/model_dashboard_test.go` | Sort by each column, ascending and descending | |
| `TestDashboardSelectionPersistence` | `internal/component/cli/model_dashboard_test.go` | Selected peer preserved by address across re-sort | |
| `TestDashboardRenderHeader` | `internal/component/cli/model_dashboard_test.go` | Header format: AS, router-id, uptime, peer counts | |
| `TestDashboardRenderPeerTable` | `internal/component/cli/model_dashboard_test.go` | Table columns, color codes, sort indicator | |
| `TestDashboardRenderZeroPeers` | `internal/component/cli/model_dashboard_test.go` | Zero peers shows `peers 0/0` header and "no peers configured" | |
| `TestDashboardRenderNarrowTerminal` | `internal/component/cli/model_dashboard_test.go` | Columns dropped at narrow widths per priority | |
| `TestDashboardKeyHandling` | `internal/component/cli/model_dashboard_test.go` | j/k navigation, s sort cycle, Enter detail, Esc exit | |
| `TestDashboardPollFailure` | `internal/component/cli/model_dashboard_test.go` | Error displayed, last good data retained | |
| `TestDashboardDetailAutoRefresh` | `internal/component/cli/model_dashboard_test.go` | Detail view refreshes on poll tick | |
| `TestDashboardDetailPeerDisappears` | `internal/component/cli/model_dashboard_test.go` | Returns to table with "peer disconnected" message | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Peer count | 0-N | N peers | 0 (empty table, show "no peers configured") | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-monitor-dashboard` | `test/plugin/bgp-monitor-dashboard.ci` | `bgp monitor` RPC via dispatch-command returns valid summary data (same pattern as `monitor-basic.ci`). TUI rendering is covered by unit tests since `.ci` tests cannot simulate interactive key events | |

**Note on .ci test scope:** the `.ci` test verifies the command is reachable via dispatch-command and returns valid data (wiring test). The existing `monitor-basic.ci` calls `dispatch(api, 'bgp monitor')` and checks `monitor-configured` status -- the dashboard test follows the same pattern but verifies summary data is returned. TUI rendering (table layout, colors, key handling, polling) is tested via unit tests with injected commandExecutor (no SSH).

## Files to Modify

- `internal/component/cli/model.go` (1334 lines) - UPDATE: add dashboard session field, handle dashboard messages (dashboardTickMsg, dashboardDataMsg). NOTE: already over 1000-line threshold, dashboard fields add ~10 lines; keep minimal
- `internal/component/cli/model_commands.go` (874 lines) - UPDATE: detect `bgp monitor` as dashboard command, route to startDashboardSession
- `internal/component/bgp/plugins/cmd/monitor/monitor.go` - UPDATE: replace `bgp monitor` redirect handler (lines 46-56) with dashboard-aware response. The streaming handler currently prints "use event monitor instead"; repurpose to return dashboard configuration data
- `cmd/ze/cli/main.go` (393 lines) - UPDATE: wire DashboardFactory (commandExecutor-based poller)
- `docs/architecture/api/commands.md` - UPDATE: document dashboard

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | No new RPCs needed; `ze-bgp:monitor` RPC already exists, `ze-bgp:summary` and `ze-bgp:peer-detail` already exist. Replace `bgp monitor` streaming redirect in `monitor.go` |
| CLI commands/flags | [x] | Dashboard mode entry in model |
| CLI usage/help text | [x] | Help text and `?` overlay |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Functional test for new RPC/API | [x] | `test/plugin/bgp-monitor-dashboard.ci` |

## Files to Create

**Split upfront to stay under 600 lines per file (rules/file-modularity.md):**

- `internal/component/cli/model_dashboard.go` - dashboard session lifecycle, state struct, DashboardFactory, polling, start/stop, data parsing (~200 lines)
- `internal/component/cli/model_dashboard_render.go` - header, peer table, detail view, footer rendering with lipgloss (~250 lines)
- `internal/component/cli/model_dashboard_sort.go` - sort column enum, sort logic, selection persistence (~100 lines)
- `internal/component/cli/model_dashboard_test.go` - unit tests
- `test/plugin/bgp-monitor-dashboard.ci` - functional test

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Dashboard data model and sort** -- define dashboard state struct (peers, counters, sort, selection), JSON snapshot parsing, rate computation (with edge cases: counter decrease, short interval), sort logic, selection persistence
   - Tests: `TestDashboardParseSnapshot`, `TestDashboardRateComputation`, `TestDashboardRateCounterDecrease`, `TestDashboardRateShortInterval`, `TestDashboardSortPeers`, `TestDashboardSelectionPersistence`
   - Files: `model_dashboard.go`, `model_dashboard_sort.go`, `model_dashboard_test.go`
   - Verify: tests fail then pass

2. **Phase: Dashboard rendering** -- direct screen composition with lipgloss (not viewport widget). Header bar, peer table with color-coded states and sort indicator, selected row highlight, footer. Terminal width handling (column priority dropping). Zero-peers display
   - Tests: `TestDashboardRenderHeader`, `TestDashboardRenderPeerTable`, `TestDashboardRenderZeroPeers`, `TestDashboardRenderNarrowTerminal`
   - Files: `model_dashboard_render.go`
   - Verify: tests fail then pass

3. **Phase: Key handling and navigation** -- j/k navigation, s/S sort, Enter detail, Esc exit, ? help
   - Tests: `TestDashboardKeyHandling`
   - Files: `model_dashboard.go`, `model.go`
   - Verify: tests fail then pass

4. **Phase: Polling and session lifecycle** -- DashboardFactory, periodic polling via commandExecutor, poll failure handling (error vs disconnected), session start/stop, daemon unreachable display
   - Tests: `TestDashboardPollFailure`
   - Files: `model_dashboard.go`, `model.go`, `model_commands.go`
   - Verify: tests fail then pass

5. **Phase: Detail view** -- Enter on peer shows extended info from peer-detail RPC with auto-refresh on 2s interval. Esc returns to table. Auto-return to table if peer disappears
   - Tests: `TestDashboardDetailAutoRefresh`, `TestDashboardDetailPeerDisappears`
   - Files: `model_dashboard.go`, `model_dashboard_render.go`
   - Verify: tests fail then pass

6. **Phase: Wire into CLI** -- detect `bgp monitor` in `model_commands.go` (currently handled as streaming command via `isMonitorCommand`; dashboard needs separate detection before the streaming check), DashboardFactory wiring in `cmd/ze/cli/main.go`, replace `bgp monitor` redirect in `monitor.go` with dashboard-aware response
   - Files: `model_commands.go`, `cmd/ze/cli/main.go`, `monitor.go`
   - Verify: end-to-end works

7. **Functional tests** -- create `.ci` test
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- audit, learned summary, delete spec

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-17) has implementation with file:line |
| Correctness | Rate computation handles edge cases (first poll, counter decrease, short interval, disappeared peers) |
| Naming | JSON keys in summary response unchanged, dashboard internal naming follows Go conventions |
| Data flow | Dashboard polls via commandExecutor, no direct reactor access from CLI |
| Rule: no-layering | No dual code paths for dashboard rendering |
| Rule: goroutine-lifecycle | Polling uses Bubble Tea tea.Tick, not a goroutine per poll |
| Rule: file-modularity | Dashboard split into 3 files: state/lifecycle, rendering, sort. Each under 300 lines |
| Rendering | Dashboard uses direct screen composition (lipgloss), not viewport widget. Fixed header/footer verified |
| Terminal width | Column dropping tested at narrow widths |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `bgp monitor` enters dashboard | `test/plugin/bgp-monitor-dashboard.ci` passes |
| Header shows AS/rid/uptime/peers | Unit test `TestDashboardRenderHeader` passes |
| Peer table shows all columns | Unit test `TestDashboardRenderPeerTable` passes |
| Sort works | Unit test `TestDashboardSortPeers` passes |
| Rate computed correctly | Unit test `TestDashboardRateComputation` passes |
| Rate handles counter decrease | Unit test `TestDashboardRateCounterDecrease` passes |
| Selection survives re-sort | Unit test `TestDashboardSelectionPersistence` passes |
| Esc exits dashboard | Unit test `TestDashboardKeyHandling` passes |
| Zero peers handled | Unit test `TestDashboardRenderZeroPeers` passes |
| Narrow terminal handled | Unit test `TestDashboardRenderNarrowTerminal` passes |
| Detail view auto-refreshes | Unit test `TestDashboardDetailAutoRefresh` passes |
| File split correct | `wc -l model_dashboard*.go` -- each under 400 lines |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Dashboard only reads data, no user input beyond key events |
| Resource exhaustion | Poll interval bounded (minimum 1s), no unbounded accumulation of poll results |
| Data exposure | Dashboard shows same data as `bgp summary` (already authorized) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
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

## RFC Documentation

N/A -- no RFC requirements for dashboard UI.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

## Implementation Audit

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

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for zero peers
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/400-bgp-dashboard.md`
- [ ] Summary included in commit
