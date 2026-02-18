# Chaos Web Dashboard — Design

Live web dashboard for ze-chaos providing real-time visualization and interactive control of chaos test runs. Uses HTMX + SSE for dynamic updates with all assets embedded in the binary via `go:embed`.

## Architecture Overview

The web dashboard is a `report.Consumer` — the same interface used by the terminal dashboard, JSON log, and Prometheus metrics. It plugs into the existing event fan-out mechanism with zero changes to the Reporter multiplexer.

```
                                  ┌─────────────────────┐
                                  │   Browser (HTMX)    │
                                  │                     │
                                  │  GET /    (layout)  │
                                  │  GET /sse (stream)  │
                                  │  GET /peer/{id}     │
                                  │  POST /control/...  │
                                  └─────────┬───────────┘
                                            │ HTTP
                                            ▼
  ┌───────────┐    ProcessEvent()    ┌──────────────┐    control channel    ┌──────────────┐
  │ Reporter  │───────────────────> │ WebDashboard │ ──────────────────> │ Orchestrator │
  │ (fan-out) │                     │  (Consumer)  │                     │ (event loop) │
  └───────────┘                     │              │                     │              │
       │                            │  state.go    │                     │  scheduler   │
       │ also fans to:              │  sse.go      │                     │  peers       │
       ├─ Dashboard (terminal)      │  handlers.go │                     │  model       │
       ├─ JSONLog (NDJSON)          │  templates/  │                     │  tracker     │
       └─ Metrics (Prometheus)      └──────────────┘                     └──────────────┘
```

**Data flows:**
- Events flow left-to-right: peers -> channel -> Reporter -> WebDashboard.ProcessEvent()
- SSE flows upward: WebDashboard state -> SSE broker -> browser
- Control flows right: browser -> POST handler -> control channel -> orchestrator -> scheduler

**Key constraint:** ProcessEvent() runs synchronously on the main event loop. It must be fast — update internal state and push to a broadcast channel, never block on HTTP or template rendering.

## Layout Architecture

Three-panel layout designed for desktop monitors. Peer table shows an active set (default 40 peers) with auto-promotion on events and adaptive decay — not all 200+ peers at once.

```
┌─────────────────────────────────────────────────────────────────────┐
│  HEADER BAR                                                         │
│  [status badge]  seed: 42        ⏱ 01:23    8/10 up   1200/1500   │
├──────────────┬──────────────────────────────────────────────────────┤
│              │                                                      │
│   SIDEBAR    │   MAIN CONTENT                                       │
│   320px      │                                                      │
│              │   ┌──────────────────────────────────────────────┐   │
│  ┌────────┐  │   │  PEER TABLE (sortable, filterable)          │   │
│  │ Peers  │  │   │  #  Status  ASN    Mode  Sent  Recv  Miss   │   │
│  │ 8/10   │  │   │  0  ●       65001  eBGP  150   148   2      │   │
│  └────────┘  │   │  1  ●       65002  iBGP  100   100   0      │   │
│  ┌────────┐  │   │  2  ○       65003  eBGP  150   0     150    │   │
│  │ Routes │  │   │  ...                                         │   │
│  │1200/   │  │   └──────────────────────────────────────────────┘   │
│  │1500    │  │                                                      │
│  └────────┘  │   ┌──────────────────────────────────────────────┐   │
│  ┌────────┐  │   │  PEER DETAIL PANE (click a row to open)     │   │
│  │ Chaos  │  │   │  Peer 2 | ASN 65003 | eBGP | Hold: 90s     │   │
│  │ 5 evts │  │   │  Routes: 150 sent, 0 received, 150 missing │   │
│  └────────┘  │   │  Events: Disconnected 3s ago (TCPDisconnect)│   │
│  ┌────────┐  │   └──────────────────────────────────────────────┘   │
│  │Converge│  │                                                      │
│  │avg:45ms│  │   ┌──────────────────────────────────────────────┐   │
│  └────────┘  │   │  VISUALIZATION TABS                          │   │
│              │   │  [Events] [Peer Timeline] [Convergence]      │   │
│  Properties  │   │  [Chaos Timeline] [Route Matrix]             │   │
│  ● consist.  │   │                                               │   │
│  ● converge  │   │  (active tab content here)                   │   │
│  ● no-dupes  │   │                                               │   │
│  ● hold-tmr  │   └──────────────────────────────────────────────┘   │
│  ● msg-order │                                                      │
│              │                                                      │
│  ┌────────┐  │                                                      │
│  │CONTROLS│  │                                                      │
│  │[Pause] │  │                                                      │
│  │Rate:0.1│  │                                                      │
│  │[Trigger│  │                                                      │
│  │[Stop]  │  │                                                      │
│  └────────┘  │                                                      │
└──────────────┴──────────────────────────────────────────────────────┘
```

## Header Bar

| Element | Description |
|---------|-------------|
| Title | "ze-chaos" with run status badge (RUNNING / COMPLETED / FAILED) |
| Seed | Current seed value, copyable |
| Elapsed | Running clock (mm:ss or hh:mm:ss) |
| Peer gauge | "N/M established" with progress bar |
| Route gauge | "R announced / S received" with delta indicators |

Updated every second via SSE `tick` event.

## Left Sidebar

### Summary Cards

| Card | Metrics shown |
|------|---------------|
| Peers | Established / Total, iBGP count, eBGP count |
| Routes | Announced, Received, Missing (red if >0), Extra (red if >0) |
| Chaos | Events fired, Reconnections, Withdrawn routes |
| Convergence | min / avg / max / p99 latency |

### Property Badges

- One badge per active property (5 properties available)
- Color: green (PASS), red (FAIL), gray (not checked)
- Click badge to expand violation details (inline accordion)

### Control Panel

| Control | Type | Action |
|---------|------|--------|
| Chaos toggle | Button | Pause / Resume chaos scheduler |
| Chaos rate | Slider (0.0 - 1.0) | Adjust chaos probability live |
| Manual trigger | Dropdown (action type) + button | Execute on selected peer(s) from table |
| New seed | Text input + button | Stop current run, restart with new seed |
| Stop | Button | Graceful shutdown |

## Peer Table

Table showing an **active set** of peers — not all 200+. Peers auto-appear when noteworthy events occur and decay away when idle. Users can pin peers to keep them permanently visible.

### Active Set Design

The table shows at most `max-visible` peers (default: 40). Three categories determine visibility:

| Category | Behavior | Removal |
|----------|----------|---------|
| **Pinned** | Always visible, user-controlled via pin icon | Only manual unpin |
| **Auto-promoted** | Appears when a noteworthy event fires | Decays after adaptive TTL expires |
| **Manually added** | User searches and adds from a peer picker dropdown | Manual remove or decay if unpinned |

**Auto-promotion triggers** (events that make a peer appear):

| Event Type | Priority | Why noteworthy |
|------------|----------|----------------|
| Disconnected | High | Session loss |
| Error | High | Protocol error |
| ChaosExecuted | Medium | Chaos activity |
| Reconnecting | Medium | Recovery in progress |
| RouteWithdrawn (bulk) | Medium | Significant withdrawal |
| Missing routes > 0 | Low | Convergence issue |

**Adaptive decay** — TTL adjusts based on available capacity:

| Active set fill | Decay TTL | Rationale |
|-----------------|-----------|-----------|
| < 50% of max | 120s | Plenty of room, keep peers visible longer |
| 50-80% of max | 30s | Moderate pressure, moderate decay |
| > 80% of max | 10s | Space needed, rapid decay |
| At max capacity | 5s | Immediate pressure, oldest non-pinned peer expires first |

**Stable positions** — peers do not change position when others appear or disappear:
- Rows are ordered by peer index (ascending), not insertion time
- When a peer decays, its row disappears but remaining rows stay in place
- New peers appear in their natural index position
- This prevents the table from "jumping" during high churn

### Columns

| Column | Width | Sortable | Description |
|--------|-------|----------|-------------|
| (pin) | 24px | No | Pin/unpin toggle icon (filled = pinned) |
| (checkbox) | 30px | No | Multi-select for manual chaos trigger |
| # | 40px | Yes | Peer index |
| Status | 80px | Yes | Dot indicator: green (up), red (down), yellow (reconnecting), gray (idle) |
| ASN | 80px | Yes | Peer ASN |
| Mode | 60px | Yes | iBGP / eBGP |
| Sent | 70px | Yes | Routes sent to Ze |
| Received | 70px | Yes | Routes received from Ze |
| Missing | 70px | Yes | Expected but not received (red highlight if >0) |
| Latency | 80px | Yes | Average convergence latency for this peer |
| Last Event | 200px | No | Event type + relative timestamp |
| Families | flex | No | Comma-separated family badges |

### Table Interactions

| Interaction | Mechanism | Result |
|-------------|-----------|--------|
| Click pin icon | `hx-post="/peers/{id}/pin"` | Peer pinned/unpinned, icon toggles |
| Click column header | `hx-get="/peers?sort=col&dir=asc"` | Table body re-rendered, sorted |
| Click same column | Toggle direction (asc -> desc) | Re-sorted in reverse |
| Filter by status | `hx-get="/peers?status=up"` | Only matching rows in active set shown |
| Filter by mode | `hx-get="/peers?mode=ibgp"` | Only matching rows in active set shown |
| Text search | `hx-get="/peers?q=65001"` (debounced 300ms) | Rows matching ASN or index |
| Click row | `hx-get="/peer/{id}" hx-target="#detail-pane"` | Detail pane opens below table |
| Check row(s) | Client-side checkbox | Enables "Trigger" button in control panel |
| Add peer | Peer picker dropdown above table | Manually add peer to active set |

### Peer Picker

A compact dropdown above the table for manually adding peers:
- Typeahead search by peer index or ASN
- Shows all peers not currently in the active set
- Adding a peer puts it in "manually added" category (decays unless pinned)
- "Pin all" / "Unpin all" bulk actions available

### SSE Updates for Active Set

Active set changes are pushed via SSE:
- New peer promoted: full row HTML fragment appended
- Peer decayed: row removal via `hx-swap-oob` with empty content
- Peer state changed: individual cell updates via `hx-swap-oob`
- Decay is computed server-side; client doesn't track TTLs

## Peer Detail Pane

Opens below the table when a peer row is clicked. Loaded as an HTML fragment via HTMX GET.

| Section | Content |
|---------|---------|
| Peer Info | Index, ASN, RouterID, Mode (iBGP/eBGP), HoldTime, Port, Families |
| Route Summary | Per-family breakdown: sent / received / missing counts |
| State Timeline | Horizontal bar showing connected (green) / disconnected (red) periods |
| Convergence | This peer's min/avg/max/p99 latency |
| Chaos History | Table of chaos events affecting this peer (time, action, outcome) |
| Recent Events | Scrollable list of last 100 events for this peer, newest first |
| Actions | Manual chaos trigger buttons (disconnect, withdraw, hold-timer, etc.) |

Close button in top-right corner of the pane. Clicking a different peer row replaces the content.

## Visualization Tabs

Tab bar below the peer table + detail pane. Each tab loads its content lazily on first click via HTMX GET.

### Tab 1: Event Stream

Live scrolling event feed.

| Property | Value |
|----------|-------|
| Buffer size | Last 500 events (ring buffer) |
| Row format | Timestamp (relative), peer index, event type, detail (prefix/action/error) |
| Color coding | Green: established/route-received. Red: disconnected/error. Yellow: chaos/reconnecting. Gray: sent/EOR |
| Filtering | Dropdown for peer index, checkboxes for event types |
| Auto-scroll | On by default, pauses when user scrolls up, toggle button to re-enable |
| Update | SSE-driven, new events prepended at top |

### Tab 2: Peer State Timeline

Horizontal bars showing per-peer connection state over time.

| Property | Value |
|----------|-------|
| Axis | Horizontal: elapsed time (0 to now). Vertical: one bar per peer |
| Bar segments | Green (established), red (disconnected), yellow (reconnecting) |
| Markers | Small icons on bars where chaos actions occurred |
| Hover | Tooltip with event details |
| Scale | For 200+ peers: show top 30 (by state-change count), with pagination and filter controls |
| Update | Refreshed every 2s via SSE or on-demand via tab activation |

### Tab 3: Convergence Histogram

CSS bar chart showing route propagation latency distribution.

| Property | Value |
|----------|-------|
| Buckets | 0-5ms, 5-10ms, 10-25ms, 25-50ms, 50-100ms, 100-250ms, 250-500ms, 500ms-1s, >1s |
| Bar height | Proportional to route count in bucket |
| Color | Gradient: green (fast) -> yellow (moderate) -> red (slow) |
| Deadline | Vertical dashed line at convergence deadline position |
| Stats | Below chart: total resolved, pending, slow count |
| Update | Every 2s via SSE `convergence` event |

Implementation: pure CSS using `div` elements with percentage heights and background colors. No JavaScript charting library needed.

### Tab 4: Chaos Timeline

Horizontal timeline showing chaos events over the run duration.

| Property | Value |
|----------|-------|
| Axis | Horizontal: elapsed time (0 to now) |
| Markers | One per chaos event, positioned by time |
| Color | By action type (10 types, each a distinct color) |
| Legend | Action type names with color swatches |
| Warmup | Shaded region at start of timeline showing warmup period |
| Click | Highlight affected peer in table, show event details in tooltip |
| Update | New markers appended via SSE |

### Tab 5: Route Flow Matrix

Peer-to-peer heatmap showing route propagation.

| Property | Value |
|----------|-------|
| Rows | Announcing peers (source) |
| Columns | Receiving peers (destination) |
| Cell value | Route count received by destination from source (via Ze reflection) |
| Color | Intensity proportional to count (darker = more routes) |
| Click cell | Detail popup: which routes, latency stats |
| Scale for 200+ | Show top 20 peers by route count, or filter by family, or aggregate by AS |
| Toggle | Count view (default) vs latency view (cell shows avg latency) |

Implementation: CSS grid with inline `background-color` opacity. For 200+ peers, the full matrix is 200x200 = 40,000 cells — impractical. The default view shows the top 20 most active peers (by total route count). A dropdown allows selecting specific peers or filtering by family.

## Theme and Styling

Dark theme optimized for monitoring use cases.

| Property | Value |
|----------|-------|
| Background | Dark (#0f1117) |
| Surface | Cards and panels (#1a1d23) with subtle border (#2d333b) |
| Text primary | Light (#c9d1d9) |
| Text secondary | Muted (#8b949e) |
| Accent | Blue (#58a6ff) for interactive elements, links, selected states |
| Success | Green (#3fb950) for established, pass, received |
| Warning | Amber (#d29922) for reconnecting, slow |
| Danger | Red (#f85149) for disconnected, fail, missing, error |
| Font data | System monospace (`ui-monospace, SFMono-Regular, monospace`) |
| Font labels | System sans-serif (`system-ui, -apple-system, sans-serif`) |
| Spacing | 8px base grid |
| Border radius | 6px for cards, 4px for badges and buttons |
| Status dots | 10px circles with the state color |

All CSS uses custom properties (variables) defined on `:root`, making future theme changes trivial.

## HTMX Communication

### Request/Response Map

| Trigger | Method | Path | Response | HTMX Target |
|---------|--------|------|----------|-------------|
| Page load | GET | / | Full HTML page (layout shell) | Whole page |
| SSE connect | GET | /sse | text/event-stream (ongoing) | Various (hx-swap-oob) |
| Click peer row | GET | /peer/{id} | HTML fragment: peer detail pane | #detail-pane |
| Sort column | GET | /peers?sort=col&dir=asc | HTML fragment: table body | #peer-tbody |
| Filter change | GET | /peers?status=up&mode=ibgp | HTML fragment: table body | #peer-tbody |
| Pagination | GET | /peers?page=N&size=50 | HTML fragment: table body + page controls | #peer-tbody |
| Pause chaos | POST | /control/chaos/pause | HTML fragment: updated control panel | #control-panel |
| Resume chaos | POST | /control/chaos/resume | HTML fragment: updated control panel | #control-panel |
| Set rate | POST | /control/chaos/rate | HTML fragment: updated rate display | #chaos-rate |
| Trigger action | POST | /control/chaos/trigger | HTML fragment: confirmation toast | #toast |
| New seed | POST | /control/run/restart | Redirect to / | Whole page |
| Stop run | POST | /control/run/stop | HTML fragment: stopped status | #run-status |
| Convergence chart | GET | /viz/convergence | HTML fragment: histogram | #viz-convergence |
| Chaos timeline | GET | /viz/chaos-timeline | HTML fragment: timeline | #viz-chaos |
| Peer timeline | GET | /viz/peer-timeline?page=1 | HTML fragment: bars | #viz-peer-timeline |
| Route matrix | GET | /viz/route-matrix?top=20 | HTML fragment: heatmap | #viz-route-matrix |

### SSE Event Types

The SSE stream sends targeted HTML fragments. Each SSE event includes `hx-swap-oob="true"` attributes so HTMX replaces specific DOM elements without full page re-render.

| SSE Event Name | Frequency | Content | DOM Targets |
|----------------|-----------|---------|-------------|
| tick | Every 1s | Header bar (elapsed, gauges) | #header |
| stats | Per event batch (debounced 200ms) | Summary cards, property badges | #sidebar-stats |
| peer-update | Per peer state change | Single updated table row | #peer-row-{id} |
| event | Per event (throttled to 10/s max) | New event row for feed | #event-feed (prepend) |
| convergence | Every 2s | Histogram bucket data | #convergence-chart |
| property | On property status change | Updated property badge | #prop-{name} |

### Debouncing Strategy

ProcessEvent() is called on every event (potentially 1000+/s during initial route announcement). It must not send an SSE message per event — that would overwhelm the browser.

Strategy:
1. ProcessEvent() updates internal state (fast, no I/O)
2. ProcessEvent() signals a "dirty" flag on changed components (header, specific peers, stats)
3. A background goroutine wakes every 200ms, checks dirty flags, renders only changed fragments, sends one batched SSE message
4. For event feed: buffer events and flush at most 10 rows per SSE message

This gives ~5 SSE updates/second — smooth visual updates without overload.

## Asset Embedding

| Asset | Source | Embedded Path | Approximate Size |
|-------|--------|---------------|------------------|
| htmx.min.js | htmx.org release (pinned version) | web/assets/htmx.min.js | ~14KB gzip |
| htmx SSE extension | htmx-ext-sse | web/assets/sse.js | ~3KB |
| style.css | Custom (this project) | web/assets/style.css | ~5KB |
| HTML templates | Go templates (this project) | web/templates/*.html | ~10KB total |

All files are vendored into `cmd/ze-chaos/web/` and embedded via `go:embed` directives. The binary is self-contained — no CDN, no internet, works in air-gapped lab environments.

Served at `/assets/htmx.min.js`, `/assets/sse.js`, `/assets/style.css` with appropriate `Content-Type` headers and `Cache-Control: immutable` (assets are versioned with the binary).

## WebDashboard Internal State

The WebDashboard consumer maintains all state derived from events. This state is the single source of truth for all HTTP handlers and SSE rendering.

| State | Description | Updated By |
|-------|-------------|------------|
| Run metadata | Seed, start time, peer count, scenario params, chaos config | Constructor |
| Per-peer status | Status enum (idle/established/disconnected/reconnecting), route counts (sent/received/withdrawn/missing), last event, families, per-peer convergence stats | ProcessEvent (all types) |
| Per-peer state history | List of (timestamp, state) transitions | ProcessEvent (Established, Disconnected, Reconnecting) |
| Global event buffer | Ring buffer of last 1000 events | ProcessEvent |
| Per-peer event buffer | Ring buffer of last 100 events per peer | ProcessEvent |
| Convergence histogram | Bucket counts (9 buckets), running min/avg/max/p99 | ProcessEvent (RouteReceived triggers latency calc via internal announce tracking) |
| Chaos history | Ordered list of (timestamp, peerIndex, actionType) | ProcessEvent (ChaosExecuted) |
| Route flow matrix | N×N matrix of route counts (source peer → receiving peer) | ProcessEvent (RouteReceived — source inferred from model's announced routes) |
| Property results | Per-property pass/fail + violation count | Periodic snapshot from PropertyEngine reference |
| SSE client set | Set of connected SSE channels with client IDs | HTTP handler (SSE connect/disconnect) |
| Dirty flags | Bitmask of which components changed since last SSE flush | ProcessEvent sets, SSE goroutine clears |

**Thread safety:** ProcessEvent() is called from the main goroutine. HTTP handlers and the SSE goroutine read state concurrently. All state access is protected by a RWMutex — ProcessEvent takes a write lock, handlers take a read lock.

## Control Architecture

Interactive control adds a reverse data path: browser -> web server -> orchestrator.

### Command Types

| Command | Parameters | Effect |
|---------|------------|--------|
| PauseChaos | (none) | Scheduler stops generating actions on Tick() |
| ResumeChaos | (none) | Scheduler resumes normal operation |
| SetChaosRate | rate (float64, 0.0-1.0) | Scheduler uses new rate on next Tick() |
| TriggerChaos | peerIndex (int), actionType (string) | Targeted action sent to specific peer's chaos channel |
| StopRun | (none) | Cancel the main context, triggering graceful shutdown |
| RestartRun | seed (uint64), optional params | Stop current run, re-initialize orchestrator with new parameters |

### Channel Design

- Buffered Go channel (capacity 16) created in `setupReporting()`, passed to both WebDashboard and orchestratorConfig
- Orchestrator's main event loop uses `select` to read from both the event channel and the control channel
- Non-blocking send from HTTP handlers — if the channel is full (16 commands queued), respond with HTTP 503 "busy"
- RestartRun is the most complex command — it requires stopping all goroutines and re-entering `runOrchestrator()` with new parameters

### Scheduler Extensions

The chaos scheduler needs these new methods (all are backwards-compatible additions):

| Method | Behavior |
|--------|----------|
| Pause() | Sets paused flag; Tick() returns empty slice when paused |
| Resume() | Clears paused flag |
| SetRate(r float64) | Updates the probability used by Tick() |
| IsPaused() bool | Returns current paused state |

These are mutex-protected since the scheduler runs in its own goroutine.

## New Chaos Action Types

Five new action types added for richer interactive control from the dashboard. These complement the existing 10 actions.

### Existing Actions (Reference)

| Action | Weight | Description |
|--------|--------|-------------|
| TCPDisconnect | 25 | Abrupt TCP close, no notification |
| NotificationCease | 15 | Clean close with NOTIFICATION message |
| HoldTimerExpiry | 15 | Stop sending KEEPALIVEs (timeout detection) |
| PartialWithdraw | 15 | Withdraw random 10-50% of routes |
| FullWithdraw | 5 | Withdraw all routes |
| DisconnectDuringBurst | 5 | Disconnect during initial route announcement |
| ReconnectStorm | 5 | Rapid connect/disconnect cycles |
| ConnectionCollision | 5 | Duplicate connection same RouterID |
| MalformedUpdate | 5 | Invalid ORIGIN value |
| ConfigReload | 5 | SIGHUP to Ze |

### New Actions

| Action | Description | Parameters |
|--------|-------------|------------|
| ClockDrift | Skew keepalive timing to test hold-timer tolerance. Instead of sending keepalives at the negotiated interval, send them early or late by a configurable drift amount. Tests whether Ze correctly detects hold-timer expiry vs tolerates minor jitter. | drift: duration (e.g., +5s, -3s). Positive = late, negative = early. Absolute value must be less than hold time. |
| RouteBurst | Announce a configurable number of extra routes in rapid succession. Floods Ze with announcements to test throughput and backpressure handling. Routes use deterministic prefixes from the peer's seed to avoid collisions. | count: int (number of routes to announce, e.g., 500, 5000). family: string (address family, default: ipv4/unicast). |
| WithdrawalBurst | Withdraw a configurable number of routes in rapid succession. Unlike PartialWithdraw (random 10-50%), this allows precise control over the volume. | count: int (number of routes to withdraw). If count exceeds announced routes, withdraws all. |
| RouteFlap | Withdraw and immediately re-announce the same set of routes. Tests Ze's handling of rapid state changes for the same prefixes without dropping the session. Repeats the flap cycle a configurable number of times. | count: int (routes to flap). cycles: int (number of withdraw+announce cycles, default: 3). interval: duration (delay between cycles, default: 100ms). |
| SlowPeer | Artificially delay all outgoing messages (keepalives, route announcements) by a configurable amount. Simulates a congested or overloaded peer. Lasts for a configurable duration then returns to normal. | delay: duration (added to each message send, e.g., 2s). duration: duration (how long to remain slow, e.g., 30s). |
| ZeroWindow | Set the TCP receive window to zero on the peer's socket, simulating a receiver that stops reading. Ze must handle TCP zero-window probes and backpressure without crashing or corrupting state. After a configurable duration, the window is restored and pending data drains. Tests Ze's write-side buffering, keepalive behavior under backpressure, and session timeout handling. | duration: duration (how long to keep window at zero, e.g., 15s). |

### Weight Assignment for New Actions (Scheduler)

When used in the automatic scheduler (not manual trigger), new actions use these weights:

| Action | Weight | Rationale |
|--------|--------|-----------|
| ClockDrift | 5 | Subtle — tests edge cases |
| RouteBurst | 5 | Aggressive — use sparingly |
| WithdrawalBurst | 10 | Common failure mode |
| RouteFlap | 10 | Important stability test |
| SlowPeer | 5 | Duration-based, fewer occurrences |
| ZeroWindow | 5 | TCP-level stress test |

New actions are **disabled by default** in the automatic scheduler. Enabled via `--chaos-actions` flag (comma-separated list) or from the dashboard UI.

## Parameterized Manual Trigger UI

The control panel's manual trigger is not a simple dropdown+button — each action type has its own parameter form that appears when selected.

### Trigger Form Layout

```
┌─── Manual Chaos Trigger ──────────────────────┐
│                                                │
│  Target: [Selected peers from table]     [All] │
│                                                │
│  Action: [▼ TCPDisconnect          ]           │
│                                                │
│  ┌─── Parameters ───────────────────────────┐  │
│  │ (varies by action type — see below)      │  │
│  └──────────────────────────────────────────┘  │
│                                                │
│  [Execute]                          [Cancel]   │
│                                                │
└────────────────────────────────────────────────┘
```

When the action dropdown changes, the parameters section updates via HTMX (`hx-get="/control/trigger-params?action=RouteBurst"` → replaces #trigger-params with the relevant form fields).

### Per-Action Parameter Forms

| Action | Parameters Shown | Defaults |
|--------|------------------|----------|
| TCPDisconnect | (none) | — |
| NotificationCease | (none) | — |
| HoldTimerExpiry | (none — peer stops keepalives) | — |
| PartialWithdraw | percentage: slider 10-90% | 30% |
| FullWithdraw | (none) | — |
| DisconnectDuringBurst | (none) | — |
| ReconnectStorm | cycles: number input 2-20 | 5 |
| ConnectionCollision | (none) | — |
| MalformedUpdate | (none) | — |
| ConfigReload | (none — sends SIGHUP) | — |
| ClockDrift | drift: text input with sign (+5s, -3s) | +2s |
| RouteBurst | count: number input 1-10000; family: dropdown | 500, ipv4/unicast |
| WithdrawalBurst | count: number input 1-10000 | 100 |
| RouteFlap | count: number 1-1000; cycles: number 1-50; interval: text (ms/s) | 50, 3, 100ms |
| SlowPeer | delay: text (ms/s); duration: text (s) | 2s, 30s |
| ZeroWindow | duration: text (s) | 15s |

### Target Selection

- **From table:** Check peer rows using the checkbox column, then open the trigger form. Target shows "Peers: 0, 3, 7" (selected indices).
- **All peers:** Button to target all currently established peers.
- **Single peer:** From the peer detail pane's "Actions" section, buttons trigger actions on that specific peer (pre-filled target).

### Validation

Before sending the trigger command:
- Target must have at least one established peer
- Parameters must be within valid ranges (see boundary tests)
- ClockDrift absolute value must be less than target peer's hold time
- RouteBurst/WithdrawalBurst count clamped to reasonable maximum (10000)
- Server-side validation returns error fragment if invalid

### HTMX Interaction

| Step | Request | Response |
|------|---------|----------|
| Select action type | GET /control/trigger-params?action=RouteBurst | HTML fragment: parameter form |
| Submit trigger | POST /control/chaos/trigger (form data: action, peers, params) | HTML fragment: confirmation toast or error |
| Confirmation | Toast auto-dismisses after 3s | — |

## Replay Constraint

**All actions triggered from the UI MUST flow through the normal event pipeline and appear in the NDJSON event log.** This is critical for reproducibility:

1. Manual trigger from UI → POST handler sends command to control channel
2. Control channel → orchestrator dispatches action to peer's chaos channel
3. Peer executes action → emits `EventChaosExecuted` with action name and parameters
4. EventChaosExecuted → flows through Reporter → all consumers (Dashboard, JSONLog, Metrics, WebDashboard)
5. JSONLog records the event with full details (action type, parameters, peer index, timestamp offset)

This means:
- Manual triggers are **indistinguishable** from scheduler-generated chaos events in the event log
- `--replay` can replay a run that included manual triggers
- `--shrink` can minimize reproductions that include manual triggers
- `--diff` can compare runs with/without manual intervention

### Event Log Format for Parameterized Actions

The NDJSON record for chaos events already includes `"chaos-action"`. For parameterized actions, the parameters are included in a `"chaos-params"` field:

| Field | Type | Description |
|-------|------|-------------|
| chaos-action | string | Action type name (e.g., "RouteBurst") |
| chaos-params | object | Action parameters (e.g., {"count": 500, "family": "ipv4/unicast"}) |

Existing non-parameterized actions emit `"chaos-params": null` (or omit the field).

### Control Actions (non-chaos)

UI actions that are NOT chaos events (pause/resume/rate change/stop) are also logged but as a separate record type:

| Record Type | Fields | Purpose |
|-------------|--------|---------|
| control | action, params, timestamp | Audit trail for UI interactions |

These are informational — `--replay` skips them (they're not peer events). They exist for debugging "what did the operator do during this run?"

## Implementation Phases

| Phase | Scope | Dependencies |
|-------|-------|--------------|
| **Phase 1: Foundation** | HTTP server, layout shell, summary cards, peer table (sortable, filterable, paginated), SSE streaming, peer detail pane, dark theme, embedded assets | None |
| **Phase 2: Visualizations** | Event stream feed, convergence histogram, peer state timeline, chaos event markers timeline | Phase 1 |
| **Phase 3: Interactive Controls** | Pause/resume chaos, manual trigger, rate slider, stop/restart, multi-select peers, control channel | Phase 1, scheduler extensions |
| **Phase 4: Route Flow Matrix** | Peer-to-peer heatmap with top-N filtering, family filter, count/latency toggle | Phase 1 |

Each phase is independently testable and deployable. Phase 1 delivers a complete view-only dashboard. Phases 2-4 can be implemented in any order after Phase 1.
