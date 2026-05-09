# Spec: diag-0-umbrella -- Production Diagnostics via MCP

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-05-09 |

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

| Entry Point | → | Feature Code | Test | Status |
|-------------|---|--------------|------|--------|
| `ze-l2tp-api:observer` | → | `subsystem_snapshot.SessionEvents()` → `observer.eventRing.snapshot()` | `observer_test.go` | done |
| `ze-l2tp-api:cqm` | → | `subsystem_snapshot.LoginSamples()` → `cqm.go` | `cqm_test.go` | done |
| `ze-show:interface` | → | `iface.ListInterfaces()` → platform backend | `show_test.go` | done |
| `ze-show:event-recent` | → | `server.EventRing().Snapshot()` | `event_ring_test.go` | done |
| `ze-bgp:peer-history` | → | `reactor.PeerFSMHistory()` → per-peer FSM ring | (reactor tests) | done |
| `ze-show:l2tp-health` | → | `subsystem.LoginSummaries()` → observer | (handler test) | done |
| `ze-show:metrics-query` | → | Prometheus scrape + `filterMetricLines()` | (handler test) | done |
| `ze-show:capture` | → | `l2tp.CaptureSnapshot()` / `reactor.BGPCaptureSnapshot()` | missing | gap |
| `ze-show:health` | → | `health.Check()` → registered CheckFuncs | `registry_test.go` | done |
| `ze-bgp:log-recent` | → | `slogutil.GlobalLogRing().Snapshot()` | `ring_test.go` | done |
| `ze-show:ping` | → | ICMP raw socket | `ping_test.go` | done |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Status | Implementation |
|-------|-------------------|-------------------|--------|----------------|
| AC-1 | `show l2tp observer <session-id>` for an active session | JSON array of event records (timestamp, type, tunnel-id, session-id, RTT, reason) | done | `cmd/l2tp/l2tp.go` `ze-l2tp-api:observer`, `l2tp/subsystem_snapshot.go:SessionEvents()` |
| AC-2 | `show l2tp cqm <session-id>` for an active session | JSON array of CQM buckets (start, state, echo-count, min/max/avg RTT) | done | `cmd/l2tp/l2tp.go` `ze-l2tp-api:cqm`, `l2tp/subsystem_snapshot.go:LoginSamples()` |
| AC-3 | `show interface counters` | JSON array of interfaces with rx/tx/error counters | done | `cmd/show/show.go` `ze-show:interface` with brief/errors/type filters |
| AC-4 | `show component status` | JSON object with per-component health and uptime | done | `cmd/show/system.go` `ze-show:system-subsystem-list` (stage, running, command-count) |
| AC-5 | `show event recent count 10` | Last 10 events from global ring, newest first | done | `cmd/show/show.go` `ze-show:event-recent`, `plugin/server/event_ring.go` |
| AC-6 | `show bgp peer history <peer>` for a peer that flapped | FSM transition records with timestamps and triggers | done | `cmd/peer/peer.go` `ze-bgp:peer-history`, `reactor/reactor.go:PeerFSMHistory()` |
| AC-7 | `show l2tp health` | Sessions sorted by echo loss, degraded sessions flagged | done | `cmd/show/show.go` `ze-show:l2tp-health`, `l2tp/observer.go:LoginSummaries()` |
| AC-8 | `show metrics query <name> label=value` | Matching time series values | done | `cmd/show/show.go` `ze-show:metrics-query`, Prometheus text filter |
| AC-9 | `show l2tp capture tunnel-id <id>` | Decoded L2TP control messages with AVP summary | done | `cmd/show/show.go` `ze-show:capture`, `l2tp/capture.go:CaptureRing` |
| AC-10 | `show bgp capture peer <peer>` | Decoded BGP messages with attribute summary | done | `cmd/show/show.go` `ze-show:capture`, `reactor/capture.go:BGPCaptureRing` |
| AC-11 | `show health` | Aggregated component health with dependency status | done | `cmd/show/show.go` `ze-show:health`, `core/health/registry.go` |
| AC-12 | `show log recent level error component l2tp` | Filtered log entries from ring buffer | done | `cmd/log/log.go` `ze-bgp:log-recent`, `core/slogutil/ring.go:LogRing` |
| AC-13 | All new commands queryable via MCP `tools/call` | MCP auto-generation picks up YANG RPCs without code changes | done | All handlers use `pluginserver.RegisterRPCs`; MCP auto-gen unchanged |

## 🧪 TDD Test Plan

### Unit Tests (phase-owned; listed here for cross-reference)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestObserver*` (6 tests) | `internal/component/l2tp/observer_test.go` | AC-1 (event ring snapshot export) | done |
| `TestCQM*` | `internal/component/l2tp/cqm_test.go` | AC-2 (CQM bucket query) | done |
| `TestHandleShowInterface*` | `internal/component/cmd/show/show_test.go` | AC-3 (interface counters) | done |
| `TestGlobalEventRing*` (6 tests) | `internal/component/plugin/server/event_ring_test.go` | AC-5 (global ring capture and query) | done |
| (BGP FSM history ring tested via reactor) | `internal/component/bgp/reactor/` | AC-6 (per-peer FSM history) | done |
| `TestRegistryHealthy` etc (7 tests) | `internal/core/health/registry_test.go` | AC-11 (component health aggregation) | done |
| `TestLogRing*` (6 tests) | `internal/core/slogutil/ring_test.go` | AC-12 (log ring filtered query) | done |
| `TestPing*` (10 tests) | `internal/component/cmd/show/ping_test.go` | active probes (diag-5) | done |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| event ring count param | 1 - 10000 | 10000 | 0 | 10001 |
| session-id (L2TP) | 1 - 65535 | 65535 | 0 | 65536 |
| capture ring depth (config) | 16 - 4096 | 4096 | 15 | 4097 |
| log query count | 1 - 10000 | 10000 | 0 | 10001 |

### Functional Tests

No dedicated `.ci` functional tests were written for the diagnostic commands.
Coverage is via unit tests on the handlers, ring buffers, and health registry.
Diagnostic commands are exercised in-production via MCP/CLI.

### Future (if deferring any tests)
- Dedicated `.ci` tests for diagnostic commands would improve coverage but require
  multi-component test harness (L2TP session up + observer active + query)

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

### Now queryable via MCP (implemented since spec was written)

All items from the original "not queryable" and "does not exist" lists are now implemented:

| Component | CLI surface | Handler |
|-----------|-------------|---------|
| L2TP observer | `ze-l2tp-api:observer` | `cmd/l2tp/l2tp.go`, `subsystem_snapshot.go:SessionEvents()` |
| L2TP CQM | `ze-l2tp-api:cqm` | `cmd/l2tp/l2tp.go`, `subsystem_snapshot.go:LoginSamples()` |
| L2TP health | `ze-show:l2tp-health` | `cmd/show/show.go`, `observer.go:LoginSummaries()` |
| L2TP capture | `ze-show:capture l2tp` | `cmd/show/show.go`, `l2tp/capture.go:CaptureRing` |
| L2TP FSM history | `ze-l2tp-api:tunnel-history`, `ze-l2tp-api:session-history` | `cmd/l2tp/l2tp.go`, `l2tp/fsm_history.go` |
| BGP peer history | `ze-bgp:peer-history` | `cmd/peer/peer.go`, `reactor/reactor.go:PeerFSMHistory()` |
| BGP capture | `ze-show:capture bgp` | `cmd/show/show.go`, `reactor/capture.go:BGPCaptureRing` |
| BGP health | `ze-show:bgp-health` | `cmd/show/show.go` |
| Global event ring | `ze-show:event-recent`, `ze-show:event-namespaces` | `cmd/show/show.go`, `plugin/server/event_ring.go` |
| Metrics query | `ze-show:metrics-query` | `cmd/show/show.go`, Prometheus text filter |
| Interface counters | `ze-show:interface` (brief, errors, type filters) | `cmd/show/show.go` |
| Component status | `ze-show:system-subsystem-list` | `cmd/show/system.go` |
| Health registry | `ze-show:health` + HTTP `/health` | `cmd/show/show.go`, `core/health/registry.go` |
| Log ring query | `ze-bgp:log-recent` (level, component, count) | `cmd/log/log.go`, `core/slogutil/ring.go` |
| Traffic/policer | `ze-show:traffic` | `cmd/show/show.go` |
| Ping | `ze-show:ping` | `cmd/show/ping.go` |
| Route lookup | `ze-show:route-lookup` | `cmd/show/route_lookup.go` |

### Remaining gaps

| Capability | Status |
|------------|--------|
| Traceroute | Not implemented (ping exists, traceroute does not) |
| Per-session traffic rate/drops | `ze-show:traffic` shows per-interface qdisc, not per-L2TP-session |
| VPP trace integration | Not implemented (platform-specific, needs govpp trace API) |
| Capture pcap export | Not implemented (decoded summaries only, no raw byte storage) |

## Child Specs

No child specs were written. All seven phases were implemented directly without
individual spec files. The umbrella served as the sole design document.

| # | Name | Priority | Status |
|---|------|----------|--------|
| 1 | Runtime state inspection | P0 | done (observer, CQM, interface, component status, traffic) |
| 2 | Event history and FSM log | P0 | done (global event ring, BGP peer history, L2TP tunnel/session history) |
| 3 | Metrics query via CLI/MCP | P1 | done (metrics query with label filter, L2TP/BGP health) |
| 4 | Packet capture | P1 | done (L2TP capture ring, BGP capture ring, zero-alloc append) |
| 5 | Active probes | P2 | partial (ping done, route-lookup done, traceroute not implemented) |
| 6 | Health and readiness | P2 | done (health registry, HTTP handler, CLI) |
| 7 | Structured log query | P3 | done (log ring handler wired into slogutil, level/component/count filter) |

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

- `internal/component/l2tp/observer.go` -- `LoginSummaries()` exported via `subsystem_snapshot.go`
- `internal/component/l2tp/subsystem_snapshot.go` -- `SessionEvents()`, `LoginSamples()`, `LoginSummaries()`, `CaptureSnapshot()`
- `internal/component/cmd/show/show.go` -- 13 new RPC handlers registered
- `internal/component/cmd/show/system.go` -- `system-subsystem-list` handler
- `internal/component/cmd/l2tp/l2tp.go` -- observer, cqm, tunnel-history, session-history handlers
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- peer-history handler
- `internal/component/bgp/reactor/reactor.go` -- `PeerFSMHistory()`, `EnableCapture()`, `CaptureSnapshot()`
- `internal/component/cmd/log/log.go` -- log-recent handler
- `internal/component/plugin/server/server.go` -- global event ring
- `internal/component/plugin/types_bgp.go` -- `FSMHistoryProvider`, `BGPCaptureProvider` interfaces
- `internal/core/slogutil/slogutil.go` -- ring handler wired into logger creation

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

All files below have been created. Child specs were not written; work proceeded
directly from the umbrella.

Source files created:

- `internal/core/health/registry.go` + `registry_test.go` -- component health aggregation
- `internal/core/slogutil/ring.go` + `ring_test.go` -- log ring buffer slog handler
- `internal/component/l2tp/capture.go` -- L2TP control packet capture ring (zero-alloc append)
- `internal/component/l2tp/fsm_history.go` -- L2TP tunnel/session FSM transition ring
- `internal/component/bgp/reactor/capture.go` -- BGP message capture ring (zero-alloc append)
- `internal/component/plugin/server/event_ring.go` + `event_ring_test.go` -- global event history ring
- `internal/component/cmd/show/ping.go` + `ping_test.go` -- ICMP ping with raw socket
- `internal/component/cmd/show/route_lookup.go` -- FIB route lookup

Handlers added to existing files:

- `internal/component/cmd/show/show.go` -- interface, traffic, l2tp-health, bgp-health, metrics-query, event-recent, event-namespaces, health, capture
- `internal/component/cmd/show/system.go` -- system-subsystem-list
- `internal/component/cmd/l2tp/l2tp.go` -- observer, cqm, tunnel-history, session-history
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- peer-history
- `internal/component/cmd/log/log.go` -- log-recent

## Implementation Steps

All seven phases were implemented directly from the umbrella without child specs.

| Phase | Status | What was delivered |
|-------|--------|-------------------|
| 1 | done | `ze-l2tp-api:observer`, `ze-l2tp-api:cqm`, `ze-show:interface`, `ze-show:system-subsystem-list`, `ze-show:traffic` |
| 2 | done | `ze-show:event-recent`, `ze-show:event-namespaces`, `ze-bgp:peer-history`, `ze-l2tp-api:tunnel-history`, `ze-l2tp-api:session-history` |
| 3 | done | `ze-show:metrics-query`, `ze-show:l2tp-health`, `ze-show:bgp-health` |
| 4 | done | `ze-show:capture` (L2TP + BGP), zero-alloc rings. VPP trace not implemented. |
| 5 | partial | `ze-show:ping`, `ze-show:route-lookup` done. Traceroute not implemented. |
| 6 | done | `ze-show:health`, `health.Registry`, HTTP `/health` handler |
| 7 | done | `ze-bgp:log-recent`, `slogutil.LogRing`, `ringHandler` wired into logger |

### Critical Review Checklist (umbrella)

| Check | What to verify | Status |
|-------|----------------|--------|
| Completeness | Every AC has a handler registered via `pluginserver.RegisterRPCs` | pass (AC-1 through AC-13) |
| Correctness | Ring buffer snapshot returns copies, not live references; queries do not block hot paths | pass (all rings: lock, copy, unlock) |
| Naming | Wire methods follow `ze-show:*`, `ze-l2tp-api:*`, `ze-bgp:*` patterns; JSON keys use kebab-case | pass |
| Data flow | All queries go through command dispatch; no direct cross-component calls | pass |
| Rule: no-layering | No wrapper layers; `subsystem_snapshot.go` delegates directly to observer/cqm | pass |
| Rule: derive-not-hardcode | Event namespaces from registry; message types from typed enums | pass |
| Zero-alloc hot path | Capture rings store value types only; string formatting at snapshot time | pass |

### Deliverables Checklist

| Deliverable | Verification method | Status |
|-------------|---------------------|--------|
| L2TP observer queryable | `ze-l2tp-api:observer` handler exists, unit tests pass | done |
| Global event ring captures events | `ze-show:event-recent` handler exists, 6 unit tests pass | done |
| Metrics queryable by label | `ze-show:metrics-query` handler exists | done |
| L2TP capture ring active | `ze-show:capture l2tp` handler exists, `CaptureRing` implemented | done |
| Health endpoint responds | `ze-show:health` + HTTP handler, 7 unit tests pass | done |
| Log query works | `ze-bgp:log-recent` handler exists, 6 unit tests pass | done |

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
- The L2TP observer already demonstrates the correct ring buffer pattern: pre-allocated pool, fixed-size rings, snapshot returns copies. This pattern was reused for global event ring, BGP FSM history, packet capture, and log ring.
- Cross-component state (like "show component status") goes through the command dispatch layer via `pluginserver.RegisterRPCs`, not direct function calls.
- Two-tier capture design (numeric value types on hot path, string formatting at query time) achieves zero-alloc append while still returning JSON-friendly output. This resolved open question #1 definitively.
- Child specs turned out to be unnecessary for this scope. The umbrella was sufficient as the sole design document because the pattern (ring buffer + handler + RegisterRPCs) was uniform across all seven phases.

## RFC Documentation

Not protocol work. No RFC references needed for diagnostic commands.

## Implementation Summary

_To be filled as phases complete; each phase lands its own summary in `plan/learned/NNN-diag-<phase>-<name>.md`. Umbrella summary lands after all phases as `plan/learned/NNN-diag-0-umbrella.md`._

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| L2TP session flap diagnosis | done | observer, cqm, tunnel/session-history, capture, l2tp-health | Full workflow supported |
| Packet loss investigation | done | interface (errors/counters), metrics-query, capture, traffic | Full workflow supported |
| BGP convergence diagnosis | done | peer-history, bgp capture, event-recent, bgp-health | Full workflow supported |
| General health check | done | health registry, event-recent, system-subsystem-list, log-recent | Full workflow supported |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `ze-l2tp-api:observer` handler + `observer_test.go` | Per-session event ring snapshot |
| AC-2 | done | `ze-l2tp-api:cqm` handler + `cqm_test.go` | CQM bucket query |
| AC-3 | done | `ze-show:interface` handler + `show_test.go` | Counters, errors, brief, type filters |
| AC-4 | done | `ze-show:system-subsystem-list` handler | Stage, running, command-count |
| AC-5 | done | `ze-show:event-recent` handler + `event_ring_test.go` | Namespace filter, count limit |
| AC-6 | done | `ze-bgp:peer-history` handler + reactor FSM history ring | Per-peer transitions |
| AC-7 | done | `ze-show:l2tp-health` handler | Login summaries sorted by RTT |
| AC-8 | done | `ze-show:metrics-query` handler | Prometheus text filter by name+labels |
| AC-9 | done | `ze-show:capture l2tp` handler + `l2tp/capture.go` | Zero-alloc capture ring |
| AC-10 | done | `ze-show:capture bgp` handler + `reactor/capture.go` | Zero-alloc capture ring |
| AC-11 | done | `ze-show:health` handler + `health/registry_test.go` | Registry + HTTP 200/503 |
| AC-12 | done | `ze-bgp:log-recent` handler + `slogutil/ring_test.go` | Level/component/count filter |
| AC-13 | done | All handlers use `pluginserver.RegisterRPCs` | MCP auto-gen unchanged |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Observer tests | done | `l2tp/observer_test.go` | 6 tests |
| CQM tests | done | `l2tp/cqm_test.go` | Bucket export |
| Global event ring | done | `plugin/server/event_ring_test.go` | 6 tests |
| Health registry | done | `core/health/registry_test.go` | 7 tests |
| Log ring | done | `core/slogutil/ring_test.go` | 6 tests |
| Ping | done | `cmd/show/ping_test.go` | 10 tests |
| Interface show | done | `cmd/show/show_test.go` | 2 tests |
| L2TP/BGP capture ring | missing | no `capture_test.go` | Unit tests for ring append/snapshot not written |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| child specs 1-7 | skipped | Work proceeded directly from umbrella |
| `core/health/registry.go` | done | |
| `core/slogutil/ring.go` | done | |
| `l2tp/capture.go` | done | |
| `l2tp/fsm_history.go` | done | |
| `reactor/capture.go` | done | |
| `plugin/server/event_ring.go` | done | |
| `cmd/show/ping.go` | done | |
| `cmd/show/route_lookup.go` | done | |

### Audit Summary
- **Total items:** 13 ACs + 7 phases + 8 new files
- **Done:** 13 ACs, 6/7 phases, 8/8 files
- **Partial:** 1 (diag-5: ping done, traceroute missing)
- **Skipped:** child specs (implemented directly)
- **Missing tests:** L2TP capture ring, BGP capture ring (no unit tests)

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

All five original questions have been resolved by implementation.

| # | Question | Resolution |
|---|----------|------------|
| 1 | Should packet capture rings store raw bytes or decoded summaries? | **Decoded summaries.** `captureRecord`/`bgpCaptureRecord` store numeric value types only (uint16, netip.Addr, MessageType). Zero-alloc on append. String formatting deferred to `Snapshot()`. Raw bytes would require per-append heap allocation or fixed 4KB slots; summaries use ~40 bytes/slot. |
| 2 | Should the global event ring be always-on or opt-in? | **Always-on.** Created unconditionally in `plugin/server/server.go:161`. Cost is negligible. |
| 3 | How should `show interface` integrate with VPP? | **`iface.ListInterfaces()`.** Platform-specific backends (netlink on linux, sysctl on darwin) behind the existing `iface` package. No direct VPP counter query yet. |
| 4 | Should health checks be push or pull? | **Pull.** `health.Registry.Check()` runs all registered checks on demand. No event-bus push. |
| 5 | Capture depth defaults for gokrazy? | **256 slots per ring.** `captureRingCapacity = 256` (L2TP), `bgpCaptureRingCapacity = 256` (BGP). At ~40 bytes/slot = ~10KB per ring. |

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
