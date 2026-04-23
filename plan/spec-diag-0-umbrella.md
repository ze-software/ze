# Spec: diag-0-umbrella -- Production Diagnostics via MCP

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-04-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/mcp/tools.go` -- MCP auto-generation from command registry
4. `internal/component/mcp/handler.go` -- MCP handler, ze_execute
5. `internal/component/l2tp/observer.go` -- event ring, CQM buckets (already exist)
6. Sibling child specs (once written): `spec-diag-1-*` .. `spec-diag-7-*`

## Task

Enable Claude (via MCP) to diagnose and resolve production networking issues on a
running Ze instance without stopping it. The target scenarios are: packet loss
investigation, L2TP tunnel instability, BGP convergence problems, interface errors,
and the general class of failures expected on a busy production network.

### Why an umbrella

Production diagnostics spans multiple components (BGP, L2TP, interfaces, traffic,
bus) and multiple capabilities (state inspection, event history, packet capture,
active probes, metrics query, health). No single spec can cover this without becoming
unmanageable. The umbrella defines scope, priority order, and cross-cutting concerns.
Child specs own their ACs, TDD plans, and wiring.

### Design constraint: MCP auto-generation

Ze's MCP tools are auto-generated from the command registry (`CommandLister` in
`tools.go`). Every YANG RPC registered as a CLI command automatically becomes an
MCP tool. This means the diagnostic work is primarily about **adding CLI commands
and YANG RPCs**, not about modifying the MCP component itself. The MCP surface
grows for free.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- small-core + registration pattern
  → Constraint: new diagnostic commands register via standard YANG RPC + `init()` pattern
  → Constraint: components are independent; diagnostic queries cross component boundaries only via command dispatch
- [ ] `docs/guide/mcp/overview.md` -- MCP auto-generation from command registry
  → Decision: all diagnostic features become MCP tools automatically via YANG RPC registration; no MCP code changes needed
- [ ] `internal/component/mcp/tools.go` -- auto-generation via `CommandLister`
  → Constraint: `CommandLister` returns `[]CommandInfo`; each child spec adds YANG RPCs that appear here

### RFC Summaries (MUST for protocol work)
- [ ] Not protocol work. Diagnostic RPCs are Ze-internal, not standardized.

**Key insights:**
- MCP tools are derived from command registry; adding YANG RPCs is sufficient to expose new diagnostics
- L2TP observer has per-session event rings and CQM buckets but no external callers (confirmed via LSP: `eventRing` referenced only in `observer.go`)
- Event bus is fire-and-forget; no built-in history subscriber exists

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/tools.go` (415 LOC) -- auto-generates MCP tools from `CommandLister`; groups by prefix; typed YANG params surfaced as JSON Schema
- [ ] `internal/component/mcp/handler.go` (307 LOC) -- single HTTP POST; JSON-RPC 2.0; four methods; `ze_execute` handcrafted tool
- [ ] `internal/component/l2tp/observer.go` (214+ LOC) -- `eventRing` circular buffer, `eventRingPool` pre-allocated free list, `sampleRing` for CQM, `ObserverEvent` record type with 7 event types
- [ ] `internal/component/l2tp/cqm.go` (60+ LOC) -- `CQMBucket` with 100s aggregation (min/max/avg RTT, echo count, `BucketState`)
- [ ] `internal/component/l2tp/metrics.go` -- Prometheus counters/gauges/histograms for sessions, tunnels, echo RTT, loss
- [ ] `internal/core/events/` -- typed pub/sub (`Event[T]`, `SignalEvent`), namespace/type registry, compact IDs

**Behavior to preserve:**
- MCP auto-generation from `CommandLister` (all existing tools continue to work)
- L2TP observer event ring and CQM internals (add external query surface, do not change internal behavior)
- Event bus fire-and-forget semantics (add subscriber, do not change dispatch)
- Existing `show l2tp *`, `show bgp *`, `show bfd *` commands unchanged

**Behavior to change:**
- None directly. This umbrella adds new commands and new data collection. No existing behavior is modified.

## Data Flow (MANDATORY)

### Entry Point

Diagnostic queries enter via two paths:
1. **CLI**: operator types `show l2tp observer 42` in SSH session -> CLI dispatch -> command handler -> reads internal ring -> returns JSON
2. **MCP**: Claude sends `tools/call` with `ze_show_l2tp` action `observer` -> MCP dispatch -> same command handler -> same JSON

### Transformation Path

1. Query enters CLI/MCP dispatch layer (existing)
2. Command handler reads from internal data structure (ring buffer, metric counter, VPP API)
3. Handler formats result as JSON (structured data, not pre-formatted strings per `derive-not-hardcode`)
4. JSON returns through dispatch to caller (CLI renders, MCP passes through)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| MCP ↔ Command dispatch | `CommandDispatcher` string command | [ ] (existing, unchanged) |
| Command handler ↔ Internal state | Direct read of ring buffer snapshot or metric value | [ ] (per child spec) |
| L2TP subsystem ↔ Observer | Observer subscribes to typed events; queries call `snapshot()` | [ ] (per spec-diag-1) |
| Core events ↔ Global ring | New subscriber on `EventBus.Subscribe()` with drop-on-full channel | [ ] (per spec-diag-2) |

### Integration Points

- `CommandLister` in `tools.go` -- new YANG RPCs auto-surface as MCP tools
- `EventBus` in `pkg/ze/eventbus.go` -- global ring subscribes to all namespaces
- L2TP `observer.go` -- new exported methods to query event ring and CQM data
- VPP govpp bindings -- platform-specific trace/counter APIs (spec-diag-4, spec-diag-1)

### Architectural Verification

- [ ] No bypassed layers (diagnostic queries go through command dispatch, same as existing commands)
- [ ] No unintended coupling (each diagnostic command lives in its domain component; no cross-component imports)
- [ ] No duplicated functionality (extends existing ring/metric/event infrastructure)
- [ ] Zero-copy preserved where applicable (ring `snapshot()` returns copies, not references to live data)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `show l2tp observer <session-id>` | → | L2TP observer `eventRing.snapshot()` | `test/l2tp/show-observer.ci` (spec-diag-1) |
| CLI `show event recent` | → | Global event ring query | `test/event/show-recent.ci` (spec-diag-2) |
| CLI `show l2tp health` | → | L2TP metrics query | `test/l2tp/show-health.ci` (spec-diag-3) |
| CLI `show l2tp capture` | → | L2TP control packet ring | `test/l2tp/show-capture.ci` (spec-diag-4) |
| CLI `show health` | → | Component health registry | `test/health/show-health.ci` (spec-diag-6) |
| CLI `show log recent` | → | Log ring buffer query | `test/log/show-recent.ci` (spec-diag-7) |

Phase-level ACs decompose into child-spec ACs. Child specs own the per-phase wiring tests.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Owning child |
|-------|-------------------|-------------------|--------------|
| AC-1 | `show l2tp observer <session-id>` for an active session | JSON array of event records (timestamp, type, tunnel-id, session-id, RTT, reason) | diag-1 |
| AC-2 | `show l2tp cqm <session-id>` for an active session | JSON array of CQM buckets (start, state, echo-count, min/max/avg RTT) | diag-1 |
| AC-3 | `show interface counters` | JSON array of interfaces with rx/tx/error counters | diag-1 |
| AC-4 | `show component status` | JSON object with per-component health and uptime | diag-1 |
| AC-5 | `show event recent count 10` | Last 10 events from global ring, newest first | diag-2 |
| AC-6 | `show bgp peer history <peer>` for a peer that flapped | FSM transition records with timestamps and triggers | diag-2 |
| AC-7 | `show l2tp health` | Sessions sorted by echo loss, degraded sessions flagged | diag-3 |
| AC-8 | `show metrics query <name> label=value` | Matching time series values | diag-3 |
| AC-9 | `show l2tp capture tunnel-id <id>` | Decoded L2TP control messages with AVP summary | diag-4 |
| AC-10 | `show bgp capture peer <peer>` | Decoded BGP messages with attribute summary | diag-4 |
| AC-11 | `show health` | Aggregated component health with dependency status | diag-6 |
| AC-12 | `show log recent level error component l2tp` | Filtered log entries from ring buffer | diag-7 |
| AC-13 | All new commands queryable via MCP `tools/call` | MCP auto-generation picks up YANG RPCs without code changes | all |

## 🧪 TDD Test Plan

### Unit Tests (phase-owned; listed here for cross-reference)

| Test | File | Validates | Phase |
|------|------|-----------|-------|
| `TestObserverSnapshot` | `internal/component/l2tp/observer_test.go` | AC-1 (event ring snapshot export) | diag-1 |
| `TestCQMBucketExport` | `internal/component/l2tp/cqm_test.go` | AC-2 (CQM bucket query) | diag-1 |
| `TestGlobalEventRing` | `internal/core/events/ring_test.go` | AC-5 (global ring capture and query) | diag-2 |
| `TestFSMHistoryRing` | `internal/component/bgp/reactor/history_test.go` | AC-6 (per-peer FSM history) | diag-2 |
| `TestMetricQuery` | `internal/core/metrics/query_test.go` | AC-8 (filtered metric query) | diag-3 |
| `TestL2TPCaptureRing` | `internal/component/l2tp/capture_test.go` | AC-9 (control packet ring) | diag-4 |
| `TestHealthRegistry` | `internal/core/health/registry_test.go` | AC-11 (component health aggregation) | diag-6 |
| `TestLogRingQuery` | `internal/core/slogutil/ring_test.go` | AC-12 (log ring filtered query) | diag-7 |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| event ring count param | 1 - 10000 | 10000 | 0 | 10001 |
| session-id (L2TP) | 1 - 65535 | 65535 | 0 | 65536 |
| capture ring depth (config) | 16 - 4096 | 4096 | 15 | 4097 |
| log query count | 1 - 10000 | 10000 | 0 | 10001 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-l2tp-show-observer` | `test/l2tp/show-observer.ci` | Operator queries session event history | -- (diag-1) |
| `test-event-show-recent` | `test/event/show-recent.ci` | Operator queries global event history | -- (diag-2) |
| `test-l2tp-show-health` | `test/l2tp/show-health.ci` | Operator checks session health summary | -- (diag-3) |
| `test-l2tp-show-capture` | `test/l2tp/show-capture.ci` | Operator inspects L2TP control packets | -- (diag-4) |
| `test-health-show` | `test/health/show-health.ci` | Operator checks system health | -- (diag-6) |
| `test-log-show-recent` | `test/log/show-recent.ci` | Operator queries recent errors | -- (diag-7) |

### Future (if deferring any tests)
- Active probe tests (spec-diag-5) require platform-specific setup (VPP or raw socket); deferred until platform backend exists

## Inventory: What Exists Today

### Already queryable via MCP (auto-generated from YANG RPCs)

| Area | Commands | What they expose |
|------|----------|-----------------|
| BGP peers | `peer-list`, `peer-show`, `summary`, `peer-capabilities`, `warnings` | FSM state, peer table, negotiated caps, prefix warnings |
| BGP RIB | `show` (with pipeline filters), `best`, `best-status`, `status` | Route queries by prefix/peer/community/family/AS-path, best-path, counts |
| BGP RIB mutation | `inject`, `withdraw`, `clear-in`, `clear-out` | Canary prefix injection, RIB clearing |
| BGP monitor | `monitor` | Live event streaming with filters |
| BGP metrics | `metrics-show`, `metrics-list` | Prometheus text format, metric names |
| BGP log | `log-show`, `log-set` | Runtime log level per subsystem |
| L2TP show | `summary`, `tunnels`, `tunnel`, `sessions`, `session`, `statistics`, `listeners`, `config` | Tunnel/session state, protocol counters, listener endpoints |
| L2TP teardown | `tunnel-teardown`, `session-teardown`, `*-all` variants | Administrative teardown |
| BFD | `show-sessions`, `show-session`, `show-profile` | BFD session state, resolved profiles |

### Exists in code but NOT queryable via MCP

| Component | What exists | Why not queryable |
|-----------|-------------|-------------------|
| L2TP observer | `eventRing` per session (tunnel-up/down, session-up/down, echo-rtt, disconnect-requested). Confirmed: `eventRing` has 12 references, all in `observer.go` only. No external callers. | No CLI command exposes event ring snapshots |
| L2TP CQM | 100-second aggregated echo RTT buckets (`CQMBucket` in `cqm.go`: min/max/avg/count per bucket, `BucketState` tags) | No CLI command exposes CQM bucket history |
| L2TP metrics | Prometheus counters/gauges/histograms (`metrics.go`: session/tunnel counts, per-login echo RTT/loss, rx/tx bytes) | Only via Prometheus scrape, not via CLI/MCP |
| Event bus | Typed pub/sub (`Event[T]`, `SignalEvent` in `internal/core/events/`). 40+ event types across BGP, L2TP, traffic namespaces. | Fire-and-forget; no history, no query |
| Core metrics | Abstract gauge/counter/histogram registry (`internal/core/metrics/`) | No CLI query endpoint |

### Does not exist

| Capability | Impact on diagnosis |
|------------|-------------------|
| Event history (global) | Cannot answer "what happened in the last 5 minutes?" |
| FSM transition log | Cannot answer "why did this peer/session change state?" |
| Packet capture | Cannot see wire-level L2TP control packets or BGP messages |
| Interface counters | Cannot see link errors, rx/tx rates, MTU, drops |
| Active probes (ping/traceroute) | Cannot validate forwarding path from the router's perspective |
| Health endpoint | No `/health` for load balancers or monitoring |
| Component health | Cannot ask "is VPP responsive?" or "is the event bus backed up?" |
| Structured log query | Cannot search recent logs by severity/component |
| Traffic/policer state | Cannot see per-session rate limits, shaper queue depths |

## Child Specs

| # | Name | Priority | Rationale |
|---|------|----------|-----------|
| 1 | Runtime state inspection | P0 | Table stakes. Without this, Claude is blind. Exposes existing internal state that has no CLI surface. |
| 2 | Event history and FSM log | P0 | Turns "what happened?" from guesswork into a queryable timeline. L2TP observer rings exist but are locked inside the subsystem. |
| 3 | Metrics query via CLI/MCP | P1 | Makes Prometheus data accessible without Grafana. Enables "which sessions have loss > 5%?" |
| 4 | Packet capture | P1 | Wire-level debugging for L2TP control packets, BGP messages. The heavy weapon. |
| 5 | Active probes | P2 | Validates forwarding path: ping, traceroute, L2TP echo, BFD state from the router. |
| 6 | Health and readiness | P2 | Component health aggregation, `/health` endpoint for monitoring. |
| 7 | Structured log query | P3 | Search recent logs by severity, component, correlation ID. Lower priority because `log-show`/`log-set` already exist. |

## Child Spec Details

### spec-diag-1-runtime-state -- Runtime State Inspection

**Problem:** Internal state exists but has no CLI surface. Claude can ask "show l2tp sessions"
but cannot ask "show me the event ring for session 42" or "what are the CQM buckets for
this login?" or "show interface counters".

**New commands to add:**

| Domain | Command | What it exposes |
|--------|---------|----------------|
| L2TP | `show l2tp observer <session-id>` | Event ring snapshot for one session (last N events with timestamps) |
| L2TP | `show l2tp observer all` | All active event rings (summary: session ID, event count, last event) |
| L2TP | `show l2tp cqm <session-id>` | CQM bucket history for one session (100s buckets with min/max/avg RTT, state, echo count) |
| L2TP | `show l2tp cqm summary` | Aggregate CQM across all sessions (sessions with loss > threshold, sessions in degraded state) |
| L2TP | `show l2tp echo <session-id>` | Current echo state: last RTT, loss ratio, consecutive failures |
| Interface | `show interface <name>` | Link state, rx/tx counters, error counters, MTU, speed |
| Interface | `show interface counters` | All interfaces with rx/tx/errors in table format |
| Interface | `show interface errors` | Only interfaces with non-zero error counters |
| Traffic | `show traffic session <session-id>` | Per-session policer state, current rate, drops |
| Traffic | `show traffic summary` | Aggregate traffic stats across all sessions |
| Component | `show component status` | Each component: running/stopped, uptime, restart count |
| Plugin | `show plugin status` | Each plugin: loaded, tier, health, restart count, last crash |

### spec-diag-2-event-history -- Event History and FSM Transition Log

**Problem:** Events fire and are gone. When something goes wrong, the only record is
scattered log lines. The L2TP observer has per-session event rings, but there is no
global event history and no FSM transition log for BGP peers.

**New capabilities:**

| Capability | Design approach |
|------------|----------------|
| Global event ring | Core `internal/core/events/` gets a configurable ring buffer subscriber. Captures last N events across all namespaces. Queryable by time, namespace, event type. |
| L2TP observer exposure | YANG RPCs for existing event ring and CQM data (the data exists, it just needs a command surface). |
| BGP FSM history | Per-peer ring of state transitions (idle/connect/active/opensent/openconfirm/established) with timestamp, trigger, and error. Queryable via `show bgp peer history <peer>`. |
| L2TP FSM history | Per-session/tunnel ring of FSM transitions. Queryable via `show l2tp tunnel history <id>` and `show l2tp session history <id>`. |

**Commands:**

| Command | What it exposes |
|---------|----------------|
| `show event recent [count N] [namespace X] [type Y]` | Last N events from global ring, optionally filtered |
| `show event namespaces` | List registered event namespaces with event counts |
| `show bgp peer history <peer>` | Last N FSM transitions for a BGP peer |
| `show l2tp tunnel history <id>` | Last N FSM transitions for an L2TP tunnel |
| `show l2tp session history <id>` | Last N FSM transitions for an L2TP session |

### spec-diag-3-metrics-query -- Metrics Query via CLI/MCP

**Problem:** `metrics-show` dumps raw Prometheus text format. Claude needs to ask
targeted questions: "which sessions have echo loss > 5%?" or "what is the update
rate for peer X over the last minute?"

**New capabilities:**

| Capability | Design approach |
|------------|----------------|
| Metric query by name and label | `show metrics query <name> [label=value...]` returns matching time series |
| Metric summary | `show metrics summary` returns top-N metrics by value, anomalies |
| L2TP session health | `show l2tp health` returns sessions sorted by echo loss ratio, flagging those above threshold |
| BGP peer health | `show bgp health` returns peers sorted by update rate, flagging silent peers |

### spec-diag-4-packet-capture -- Packet Capture

**Problem:** Cannot see wire-level packets. VPP has trace/pcap APIs (govpp) but
they are not exposed. For L2TP, the control plane is userspace (Ze handles it directly),
so we can capture without VPP.

**New capabilities:**

| Capability | Design approach |
|------------|----------------|
| L2TP control packet log | Ring buffer of decoded L2TP control messages (SCCRQ/SCCRP/SCCCN/StopCCN/ICRQ/ICRP/ICCN/CDN/HELLO/ZLB) with timestamps, tunnel/session IDs, AVP summary. Always running, fixed size. |
| BGP message log | Ring buffer of decoded BGP messages (OPEN/UPDATE/NOTIFICATION/KEEPALIVE) with timestamps, peer, message type, attribute summary. Per-peer, configurable depth. |
| VPP trace integration | Expose VPP `trace add` and `show trace` via CLI commands for dataplane packet debugging. Platform-specific (linux only). |
| Capture export | `capture dump <ring> [pcap\|json\|text]` exports ring contents in selected format |

**Commands:**

| Command | What it exposes |
|---------|----------------|
| `show l2tp capture [tunnel-id N] [count N]` | Last N decoded L2TP control messages |
| `show bgp capture [peer X] [count N]` | Last N decoded BGP messages for a peer |
| `debug capture l2tp start [filter...]` | Start/configure L2TP control capture |
| `debug capture bgp start [peer X]` | Start/configure BGP message capture |
| `debug capture stop` | Stop active captures |
| `debug vpp trace [interface X] [count N]` | VPP dataplane trace (linux only) |

### spec-diag-5-active-probes -- Active Probes

**Problem:** Cannot validate the forwarding path from the router's perspective. Need
to answer "can traffic reach X from interface Y?"

**New capabilities:**

| Capability | Design approach |
|------------|----------------|
| Ping | ICMP echo from a specified source, report RTT/loss. Via VPP punt or raw socket. |
| Traceroute | UDP/ICMP traceroute from a specified source. |
| L2TP echo probe | Send explicit LCP echo to a specific session, report RTT. Uses existing echo mechanism. |
| BFD session query | Already exists (`show bfd session`). May need richer output (negotiated timers, detect multiplier, last up/down transition). |
| Route lookup | `show route lookup <prefix>` -- check what the FIB says for a given destination. |

### spec-diag-6-health -- Health and Readiness

**Problem:** No `/health` endpoint. No way for monitoring or Claude to ask "is
everything working?"

**New capabilities:**

| Capability | Design approach |
|------------|----------------|
| Component health registry | Each component reports healthy/degraded/down with a reason string. Core aggregates. |
| `/health` HTTP endpoint | Returns JSON: overall status + per-component breakdown. For load balancers, monitoring, and MCP queries. |
| `show health` CLI command | Same data as the HTTP endpoint, via CLI/MCP. |
| Dependency checks | VPP API responsive? Kernel netlink socket open? RADIUS reachable? Each subsystem checks its dependencies. |

### spec-diag-7-log-query -- Structured Log Query

**Problem:** Logs go to syslog/stdout. No way to search "show me the last 10 errors
from the L2TP subsystem" without grepping files.

**New capabilities:**

| Capability | Design approach |
|------------|----------------|
| Log ring buffer | slog handler that also writes to an in-memory ring (last N entries). |
| Log query | `show log recent [count N] [level error] [component l2tp]` filters the ring. |
| Correlation ID | slog fields include a request/session correlation ID for tracing a single flow. |

## Design Alternatives

### Approach A: CLI-First (CHOSEN)

Add YANG RPCs and CLI commands for each diagnostic capability. MCP auto-generation
picks them up. Each subsystem owns its own diagnostic commands.

**Gains:** Operators get SSH CLI access too. Follows existing registration pattern.
Zero MCP code changes. Each child spec independent and testable.

**Costs:** More YANG schema work per feature. Cross-component queries need coordinator
dispatching to each component's status command.

### Approach B: MCP-Native Diagnostic Module (REJECTED)

Add handcrafted MCP tools that query internal state directly, bypassing CLI dispatch.
A diagnostic MCP module with direct access to observer rings, metrics, event bus.

**Rejected because:** Breaks component isolation (imports from L2TP, BGP, core/events).
Operators get nothing via SSH CLI. Contradicts `design-context.md`: "no direct
cross-component imports." Creates a god-module.

## Failure Mode Analysis

| What could go wrong | Impact | Mitigation |
|---------------------|--------|------------|
| Ring buffer snapshot during high event rate | Query contention on hot path | Copy under read lock (existing observer pattern) |
| Global event ring subscriber blocks event bus | All event delivery stalls | Channel with drop-on-full; subscriber never blocks |
| Capture ring on busy LNS fills instantly | Only seeing last few ms | Configurable depth; decoded summaries not raw bytes; filter by tunnel/session |
| `show interface counters` with 10k interfaces | Response too large | Pagination via count/offset; summary mode default |
| BGP capture hooks slow read/write path | Per-message overhead on every UPDATE | Capture opt-in (off by default); ring append O(1) |
| Health check queries stuck subsystem | Health query hangs | Timeout per component probe; report "unknown" after timeout |
| Log ring stores sensitive data | Secret exposure via `show log recent` | Redaction at slog handler level (same as `show l2tp config`) |
| Concurrent show queries during ring rotation | Data race | Snapshot returns copy; atomic under lock |

## Triple Challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | Minimum change for diag-1/diag-2: data structures exist, adding query surface only. diag-4/diag-5 add new infra, justified because wire-level visibility is absent. diag-7 most debatable (log-set provides some control already). |
| Uniformity | Every child follows: YANG RPC -> command handler -> reads internal data -> returns JSON. Same pattern as existing `show l2tp sessions`, `show bgp summary`, `show bfd sessions`. Ring buffer reuses observer's `eventRing` design. |
| Performance | Fixed-size pre-allocated rings (`eventRingPool` pattern). Snapshot returns copies. Global event ring uses drop-on-full channel. Capture copies decoded summaries (~1KB). BGP capture opt-in. No per-event allocations on hot path. |

## Cross-Cutting Concerns

### MCP surface growth

All new commands are YANG RPCs registered via the standard pattern. The MCP
auto-generation in `tools.go` picks them up without modification. No changes
to the MCP component are needed for children 1-7.

### Ring buffer sizing

All ring buffers are configurable via YANG config under a new
`diagnostics` container. Defaults must be conservative for memory on
gokrazy appliances (e.g., 1000 events per global ring, 64 events per
session ring, 256 packets per capture ring).

### Security

- Packet capture and debug commands are write/destructive operations.
  MCP auth (bearer token or OAuth) gates access. YANG marks them
  `config false` with appropriate access control.
- Health endpoint needs no auth (monitoring probes) or optional auth
  (configurable).
- Log query must redact secrets (passwords, shared-secrets, tokens)
  the same way `show l2tp config` redacts shared-secret today.

### Performance impact

- Ring buffers are fixed-size, pre-allocated. No allocation on the hot
  path (L2TP observer pattern already demonstrates this with
  `eventRingPool`).
- Packet capture rings copy the decoded summary, not raw bytes, to
  avoid holding large buffers.
- Global event ring subscriber must not block the event bus. Use a
  channel with drop-on-full semantics.
- Metric queries must not block the Prometheus scrape path. Read from
  a snapshot or use atomic reads.

### Platform specifics

- VPP trace/pcap: linux only (build-tagged).
- Ping/traceroute: via VPP punt on gokrazy, via raw socket on
  development machines. Platform backends like existing `iface`
  pattern.
- Kernel interface counters: via netlink on linux, via sysctl on
  darwin (dev only).

## Diagnosis Workflows (What Claude Can Do After This)

### Workflow: L2TP session flapping

1. `show l2tp sessions` -- find the flapping session
2. `show l2tp session history <id>` -- see FSM transitions (when, why)
3. `show l2tp observer <session-id>` -- see event ring (echo-rtt drops, disconnect reasons)
4. `show l2tp cqm <session-id>` -- see RTT degradation over time
5. `show l2tp capture tunnel-id <id>` -- see actual control packets (CDN reason codes, HELLO timing)
6. `show l2tp health` -- check if this is isolated or systemic (many sessions with high loss)

### Workflow: Packet loss investigation

1. `show interface errors` -- find interfaces with error counters
2. `show interface <name>` -- detailed counters (CRC, runts, giants, drops)
3. `show traffic session <id>` -- check if policer is dropping
4. `show l2tp echo <session-id>` -- check if loss is on the L2TP control plane
5. `show l2tp cqm <session-id>` -- RTT history to correlate with loss
6. `show bgp health` -- check if BGP peers are healthy (no silent peers)
7. `debug vpp trace interface <name> count 100` -- capture dataplane packets if needed

### Workflow: BGP convergence problem

1. `show bgp summary` -- peer states and prefix counts
2. `show bgp peer history <peer>` -- FSM history (flaps? stuck in OpenSent?)
3. `show bgp capture peer <peer>` -- see actual OPEN/UPDATE/NOTIFICATION messages
4. `show bgp rib show received peer <peer> count` -- how many prefixes from this peer?
5. `show bgp rib show best prefix <pattern>` -- is the expected route present?
6. `show event recent namespace bgp` -- recent BGP events
7. `show bgp peer-capabilities <peer>` -- negotiation mismatch?
8. `bgp rib inject <peer> <family> <prefix>` -- inject canary, verify propagation

### Workflow: General "something is wrong"

1. `show health` -- overall system health
2. `show component status` -- any components down?
3. `show event recent count 50` -- what happened recently?
4. `show log recent level error count 20` -- any errors?
5. `show metrics summary` -- any anomalies?
6. Drill into the specific subsystem based on findings

## Implementation Order

The specs should be implemented in this order, driven by diagnostic value per effort:

1. **spec-diag-1** (runtime state): Exposes existing internal state. Highest impact
   because it makes invisible data visible. Most of the data structures already exist
   (L2TP observer, metrics, component registry).

2. **spec-diag-2** (event history): The L2TP observer rings exist but have no CLI
   surface. BGP FSM history is new but follows the same ring pattern. The global event
   ring is a new subscriber on the existing event bus.

3. **spec-diag-3** (metrics query): Wraps existing Prometheus metrics with targeted
   query commands. Moderate new code.

4. **spec-diag-4** (packet capture): New ring buffers for decoded control messages.
   L2TP control plane is userspace so capture is straightforward. BGP message capture
   requires hooking the message read/write path. VPP trace is platform-specific.

5. **spec-diag-5** (active probes): Platform-specific networking code.
   Ping/traceroute need raw sockets or VPP punt. Most complex platform integration.

6. **spec-diag-6** (health): Registry pattern, well-understood. Lower priority because
   Claude can already check individual components.

7. **spec-diag-7** (log query): slog handler + ring buffer. Low priority because
   `log-show`/`log-set` provide some log control already.

## Files to Modify

- `internal/component/l2tp/observer.go` -- export snapshot methods for event ring and CQM (diag-1)
- `internal/component/l2tp/cqm.go` -- export bucket query methods (diag-1)
- `internal/component/l2tp/schema/ze-l2tp-api.yang` -- add observer/cqm/echo/capture/history RPCs (diag-1, diag-2, diag-4)
- `internal/component/cmd/l2tp/schema/ze-l2tp-cmd.yang` -- augment show tree with new commands (diag-1, diag-2, diag-4)
- `internal/core/events/` -- add ring buffer subscriber (diag-2)
- `internal/core/metrics/` -- add query interface (diag-3)
- `internal/core/slogutil/` -- add ring buffer slog handler (diag-7)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/l2tp/schema/ze-l2tp-api.yang`, new YANG modules per child |
| CLI commands/flags | Yes | YANG `ze:command` augments (automatic) |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/l2tp/*.ci`, `test/event/*.ci`, `test/health/*.ci`, `test/log/*.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (add diagnostics section) |
| 2 | Config syntax changed? | Yes (diagnostics container) | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (new show/debug commands) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | -- |
| 6 | Has a user guide page? | Yes | `docs/guide/diagnostics.md` (new) |
| 7 | Wire format changed? | No | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented? | No | -- |
| 10 | Test infrastructure changed? | No | -- |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (diagnostics is a differentiator) |
| 12 | Internal architecture changed? | Maybe | `docs/architecture/core-design.md` if global event ring is significant |

## Files to Create

Child specs:

- `plan/spec-diag-1-runtime-state.md`
- `plan/spec-diag-2-event-history.md`
- `plan/spec-diag-3-metrics-query.md`
- `plan/spec-diag-4-packet-capture.md`
- `plan/spec-diag-5-active-probes.md`
- `plan/spec-diag-6-health.md`
- `plan/spec-diag-7-log-query.md`

New source files (anticipated; each child spec confirms):

- `internal/core/events/ring.go` -- global event ring buffer subscriber
- `internal/core/metrics/query.go` -- metric query by name and label filter
- `internal/core/health/registry.go` -- component health aggregation
- `internal/core/slogutil/ring.go` -- log ring buffer slog handler
- `internal/component/l2tp/capture.go` -- L2TP control packet capture ring
- `internal/component/bgp/reactor/history.go` -- per-peer FSM transition history
- `internal/component/cmd/event/` -- event query command handler + YANG
- `internal/component/cmd/health/` -- health query command handler + YANG
- Corresponding `_test.go` files
- `test/l2tp/*.ci`, `test/event/*.ci`, `test/health/*.ci`, `test/log/*.ci` -- functional tests

## Implementation Steps

Each phase below is its own child spec. Umbrella tracks ordering and hand-off.

| Phase | Child spec | Depends on | Delivers |
|-------|-----------|-----------|----------|
| 1 | `spec-diag-1-runtime-state.md` | -- | L2TP observer/CQM/echo exposure, interface counters, component/plugin status |
| 2 | `spec-diag-2-event-history.md` | -- | Global event ring, BGP FSM history, L2TP FSM history |
| 3 | `spec-diag-3-metrics-query.md` | -- | Targeted metric queries, L2TP/BGP health summaries |
| 4 | `spec-diag-4-packet-capture.md` | -- | L2TP control capture ring, BGP message capture ring, VPP trace |
| 5 | `spec-diag-5-active-probes.md` | diag-1 (interface infra) | Ping, traceroute, L2TP echo probe, route lookup |
| 6 | `spec-diag-6-health.md` | diag-1 (component status) | Health registry, `/health` endpoint, dependency checks |
| 7 | `spec-diag-7-log-query.md` | -- | Log ring buffer, structured log query |

### /implement Stage Mapping

Umbrella does NOT go through `/implement` -- child specs do. Umbrella is the
contract that keeps them consistent. When a child spec's `/implement` stage 2
(audit) runs, it cross-references this umbrella to confirm no cross-phase
surface was broken.

### Implementation Phases (umbrella-level)

1. **Phase 1 -- Runtime State** (`spec-diag-1-runtime-state.md`)
   - Export L2TP observer event ring and CQM bucket query methods
   - Add YANG RPCs for observer, cqm, echo queries
   - Add interface counter query (platform-specific backends)
   - Add component/plugin status query
2. **Phase 2 -- Event History** (`spec-diag-2-event-history.md`)
   - Add global event ring subscriber to `internal/core/events/`
   - Add per-peer BGP FSM transition ring to reactor
   - Add L2TP tunnel/session FSM history export
   - Add `show event` and `show *  history` commands
3. **Phase 3 -- Metrics Query** (`spec-diag-3-metrics-query.md`)
   - Add metric query interface to `internal/core/metrics/`
   - Add `show metrics query`, `show l2tp health`, `show bgp health` commands
4. **Phase 4 -- Packet Capture** (`spec-diag-4-packet-capture.md`)
   - Add L2TP control packet capture ring
   - Add BGP message capture ring (hook read/write path)
   - Add VPP trace integration (linux only)
   - Add `show * capture` and `debug capture` commands
5. **Phase 5 -- Active Probes** (`spec-diag-5-active-probes.md`)
   - Add ping/traceroute with platform backends
   - Add L2TP echo probe command
   - Add route lookup command
6. **Phase 6 -- Health** (`spec-diag-6-health.md`)
   - Add component health registry to `internal/core/health/`
   - Add `/health` HTTP endpoint
   - Add `show health` command
   - Add dependency checks per subsystem
7. **Phase 7 -- Log Query** (`spec-diag-7-log-query.md`)
   - Add ring buffer slog handler to `internal/core/slogutil/`
   - Add `show log recent` command with filters
   - Add correlation ID to slog fields

### Critical Review Checklist (umbrella)

| Check | What to verify across phases |
|-------|------------------------------|
| Completeness | Every AC-N has at least one child spec that owns it |
| Correctness | Ring buffer snapshot returns copies, not live references; queries do not block hot paths |
| Naming | YANG RPCs follow existing naming (`show l2tp *` pattern); JSON keys use kebab-case per ze convention |
| Data flow | All queries go through command dispatch; no direct cross-component calls |
| Rule: no-layering | No wrapper layers around existing data structures; export methods directly |
| Rule: derive-not-hardcode | Command lists, event type enumerations derived from registries |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Seven child specs exist | `ls plan/spec-diag-*.md` |
| L2TP observer queryable | `show l2tp observer` returns JSON in functional test |
| Global event ring captures events | `show event recent` returns events after BGP/L2TP activity |
| Metrics queryable by label | `show metrics query` with label filter returns matching series |
| L2TP capture ring active | `show l2tp capture` returns decoded control messages |
| Health endpoint responds | `curl /health` returns JSON with component status |
| Log query works | `show log recent level error` returns filtered entries |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | session-id, tunnel-id, peer address, count parameters validated against ranges |
| Secret redaction | Log ring buffer must redact passwords, tokens, shared-secrets before storage |
| Resource exhaustion | Ring buffer sizes bounded by config; query results paginated or count-limited |
| Access control | `debug capture` and `clear` commands require write access; `show` commands are read-only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
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

- MCP auto-generation means the entire diagnostic surface is a CLI problem, not an MCP problem. Every YANG RPC registered as a command becomes an MCP tool for free. This is the key architectural insight: invest in CLI commands, get MCP diagnostics as a side effect.
- The L2TP observer already demonstrates the correct ring buffer pattern: pre-allocated pool, fixed-size rings, snapshot returns copies. Reuse this pattern for global event ring, BGP FSM history, packet capture, and log ring.
- Cross-component state (like "show component status") must go through the command dispatch layer, not direct function calls. Each component registers its own health/status command; a coordinator command aggregates by dispatching to each.

## RFC Documentation

Not protocol work. No RFC references needed for diagnostic commands.

## Implementation Summary

_To be filled as phases complete; each phase lands its own summary in `plan/learned/NNN-diag-<phase>-<name>.md`. Umbrella summary lands after all phases as `plan/learned/NNN-diag-0-umbrella.md`._

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| L2TP session flap diagnosis | -- | (diag-1, diag-2, diag-4) | observer + FSM history + capture |
| Packet loss investigation | -- | (diag-1, diag-3, diag-4) | interface counters + metrics + capture |
| BGP convergence diagnosis | -- | (diag-2, diag-4) | FSM history + message capture |
| General health check | -- | (diag-6) | component health + dependencies |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 .. AC-13 | -- | (child-spec-owned) | Each child spec fills this row |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (all) | -- | (child-spec-owned) | -- |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| child specs 1-7 | -- | Written when each phase begins |

### Audit Summary
- **Total items:** 13 ACs + 7 phases + documentation rows
- **Done:** 0
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| -- | -- | (umbrella has no code yet; `/ze-review` run after each child spec lands) | -- | -- |

### Fixes applied
- None (pre-implementation)

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (after each phase)
- [ ] All NOTEs recorded in phase specs

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-diag-0-umbrella.md` | [ ] | ls will be run when child specs begin |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| -- | (umbrella has no verifiable ACs of its own; all delegate to child specs) | -- |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (child-spec-owned) | `test/*/*.ci` | [ ] |

## Open Questions

| # | Question | Impact |
|---|----------|--------|
| 1 | Should packet capture rings store raw bytes (for pcap export) or decoded summaries (for readability)? | Memory vs. fidelity tradeoff. Decoded summaries use less memory but lose wire details. Could store both: small raw ring + decoded summary. |
| 2 | Should the global event ring be always-on or opt-in? | Always-on is simpler and means history is available when you need it. Cost is a few MB of memory on gokrazy. |
| 3 | How should `show interface` integrate with VPP? VPP has its own counter infrastructure. | Need to decide: query VPP counters via govpp API, or track counters in Ze independently. VPP counters are authoritative for dataplane. |
| 4 | Should health checks be push (event on state change) or pull (query on demand)? | Pull is simpler. Push enables real-time alerting. Could do both: pull for query, push (via event bus) for alerting. |
| 5 | Capture depth defaults for gokrazy appliances: what memory budget? | Need to profile. 256 decoded packets at ~1KB each = 256KB per ring. Acceptable for a few rings. |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
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
- [ ] Write learned summary to `plan/learned/NNN-diag-0-umbrella.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
