# Spec: In-Process Chaos and Route Dynamics

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-02-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `cmd/ze-chaos/inprocess/runner.go` — current in-process runner
3. `cmd/ze-chaos/peer/simulator.go` — SimulatorConfig, executeChaos, executeRoute
4. `cmd/ze-chaos/main.go` — runScheduler, runRouteScheduler, runPeerLoop patterns

## Task

Add chaos scheduling and route dynamics to the in-process runner so that `--in-process --chaos-rate 0.1 --route-rate 0.05` works the same as the external orchestrator, but driven by virtual clock advances instead of wall-clock time.

Currently `--in-process` mode can only test steady-state BGP behavior. The chaos and route dynamics systems exist in the external orchestrator but are not wired to in-process mode. This creates a feature gap — in-process tests cannot verify fault tolerance, reconnection, route churn, or any dynamic behavior.

## Required Reading

### Source Files
- [ ] `cmd/ze-chaos/inprocess/runner.go` — Current Run() function, RunConfig, advance loop
  → Constraint: Simulators are single-shot, no reconnection loop
  → Constraint: No chaos/route channels passed to SimulatorConfig
- [ ] `cmd/ze-chaos/peer/simulator.go` — SimulatorConfig, executeChaos, executeRoute
  → Constraint: `executeReconnectStorm` and `executeConnectionCollision` hardcode `net.Dialer{}`
  → Decision: Chaos/Routes channels are nil-safe (nil = disabled)
- [ ] `cmd/ze-chaos/main.go` — runScheduler, runRouteScheduler, runPeerLoop
  → Decision: Schedulers use `Tick(time.Time, []bool)` — decoupled from clock source
  → Decision: Per-peer channels buffered(1), non-blocking dispatch
- [ ] `cmd/ze-chaos/orchestrator.go` — establishedState, event processing
  → Decision: Established state updated from events, snapshot read by schedulers
- [ ] `cmd/ze-chaos/guard.go` — peerGuard action filtering
  → Decision: Guards filter invalid actions before dispatch
- [ ] `cmd/ze-chaos/inprocess/mocknet.go` — MockDialer, ConnPairManager, MockListener
  → Decision: ConnPairManager.NewPair() creates TCP loopback pairs
  → Decision: MockListener.QueueConn() queues for Accept()

**Key insights:**
- Chaos/route schedulers are already decoupled from clock source — `Tick()` takes `time.Time`, works with virtual time
- Simulator already accepts `Conn` and `Clock` for in-process mode, and chaos/route channels are nil-safe
- Reconnection requires creating new connection pairs and queuing on MockListener (pattern exists in DisconnectAt code)
- Two chaos actions (`ReconnectStorm`, `ConnectionCollision`) need a pluggable dialer

## Current Behavior

**Source files read:**
- [ ] `cmd/ze-chaos/inprocess/runner.go` — Launches single-shot simulators, advances virtual clock, handles hardcoded DisconnectAt/ReconnectDelay/StopKeepalivesAt
- [ ] `cmd/ze-chaos/peer/simulator.go` — SimulatorConfig with Conn/Clock/Chaos/Routes fields, executeChaos with hardcoded net.Dialer
- [ ] `cmd/ze-chaos/main.go` — Passes `--chaos-rate`, `--route-rate` to external orchestrator but not to in-process RunConfig
- [ ] `cmd/ze-chaos/orchestrator.go` — establishedState type, event-driven state tracking
- [ ] `cmd/ze-chaos/guard.go` — peerGuard action filtering, AllowChaos/AllowRoute methods
- [ ] `cmd/ze-chaos/inprocess/mocknet.go` — ConnPairManager, MockDialer, MockListener, ConnWithAddr

**Behavior to preserve:**
- Existing `DisconnectAt`, `ReconnectDelay`, `StopKeepalivesAt` fields and their behavior (tests depend on them)
- Existing `Consumer` and `StepDelay` web dashboard integration
- External orchestrator unchanged

**Behavior to change:**
- Add chaos/route scheduling to in-process mode (user requested)
- Add `Dialer` to SimulatorConfig so storm/collision work with mock connections (required for above)

## Data Flow

### Entry Point
- User sets `--in-process --chaos-rate 0.1` flags → `main.go` passes to `RunConfig`

### Transformation Path
1. `Run()` creates schedulers from config (if rate > 0)
2. Virtual clock advance loop calls `scheduler.Tick(vc.Now(), established)` each step
3. Tick returns `[]ScheduledAction` → dispatched to per-peer chaos/route channels
4. Simulator receives action on channel → calls `executeChaos()` or `executeRoute()`
5. If disconnect: simulator exits → reconnection loop creates new pair → simulator restarts

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner → Simulator | Buffered channels (chaos.ChaosAction, route.Action) | [ ] |
| Simulator → Runner | Event channel (peer.Event) for established tracking | [ ] |
| Runner → MockListener | QueueConn for reconnection | [ ] |

### Integration Points
- `chaos.NewScheduler` / `route.NewScheduler` — create schedulers from config
- `chaos.Scheduler.Tick()` / `route.Scheduler.Tick()` — called with `vc.Now()` each advance step
- `peer.RunSimulator` — receives chaos/route channels via SimulatorConfig
- `ConnPairManager.NewPair()` — creates new mock pairs for reconnection
- `MockListener.QueueConn()` — queues reactor end of reconnection pair
- `peerGuard.AllowChaos()` / `AllowRoute()` — filters invalid actions before dispatch

### Architectural Verification
- [ ] No bypassed layers — uses existing scheduler + simulator architecture
- [ ] No unintended coupling — schedulers remain decoupled from clock source
- [ ] No duplicated functionality — reuses chaos, route, guard packages unchanged

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `--in-process --chaos-rate 0.1 --duration 30s` | Events include `EventChaosExecuted` entries |
| AC-2 | `--in-process --route-rate 0.05 --duration 30s` | Events include `EventRouteAction` entries |
| AC-3 | Chaos disconnect action in-process | Simulator reconnects via new mock pair, session re-establishes |
| AC-4 | `executeReconnectStorm` in-process | Uses `SimulatorConfig.Dialer` to create mock connections |
| AC-5 | `executeConnectionCollision` in-process | Uses `SimulatorConfig.Dialer` to create mock connections |
| AC-6 | No `--chaos-rate` flag with `--in-process` | Behavior unchanged from current (no chaos channels) |
| AC-7 | `--in-process --chaos-rate 0.1 --web :8000` | Dashboard shows chaos events in real-time |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInProcessChaosEvents` | `inprocess/runner_test.go` | AC-1: chaos events appear in result | |
| `TestInProcessRouteEvents` | `inprocess/runner_test.go` | AC-2: route action events appear | |
| `TestInProcessChaosReconnect` | `inprocess/runner_test.go` | AC-3: simulator reconnects after chaos disconnect | |
| `TestInProcessNoChaosDefault` | `inprocess/runner_test.go` | AC-6: no chaos when rate=0 | |
| `TestSimulatorDialerField` | `peer/simulator_test.go` | AC-4/5: Dialer field used by storm/collision | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Manual: `ze-chaos --in-process --chaos-rate 0.1 --duration 30s` | CLI | Events include chaos actions | |

## Files to Modify

- `cmd/ze-chaos/inprocess/runner.go` — Add RunConfig fields, scheduler setup, reconnection loop, established tracking
- `cmd/ze-chaos/peer/simulator.go` — Add `Dialer sim.Dialer` to SimulatorConfig, use in storm/collision
- `cmd/ze-chaos/main.go` — Pass chaos/route flags to RunConfig in `--in-process` branch

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] No | |
| CLI commands/flags | [x] No — flags exist, just wire to RunConfig | `cmd/ze-chaos/main.go` |
| Functional test for new RPC/API | [x] No — manual CLI verification | |

## Files to Create

- `cmd/ze-chaos/inprocess/chaos.go` — Scheduler goroutines, established state, reconnect-dialer factory (~200 lines, keeps runner.go under 600)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

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

### Step 1: Add Dialer to SimulatorConfig

Add optional `Dialer sim.Dialer` field. Modify `executeReconnectStorm` and `executeConnectionCollision` to use `cfg.Dialer` when non-nil, fall back to `net.Dialer{}` when nil.

→ **Review:** Does this change normal mode behavior? (No — Dialer nil by default)

### Step 2: Write tests for in-process chaos/route

Create `TestInProcessChaosEvents` and `TestInProcessRouteEvents` that set ChaosRate/RouteRate and verify events.

→ **Review:** Do tests fail? (Yes — RunConfig fields don't exist yet)

### Step 3: Add RunConfig fields

Add `ChaosRate`, `ChaosInterval`, `RouteRate`, `RouteInterval`, `Warmup`, `BaseRoutes` to RunConfig.

### Step 4: Create chaos.go with scheduling infrastructure

| Component | Description |
|-----------|-------------|
| `establishedState` | Thread-safe `[]bool` (same pattern as `orchestrator.go`) |
| `reconnectDialer` | Creates TCP loopback pair, wraps + queues reactor end on MockListener, returns peer end |
| `runChaosScheduler` | Goroutine: ticks chaos scheduler, dispatches to channels |
| `runRouteScheduler` | Goroutine: ticks route scheduler, dispatches to channels |

### Step 5: Integrate into Run()

1. Create per-peer chaos/route channels (if rate > 0)
2. Pass channels to SimulatorConfig
3. Wrap each simulator in reconnection loop
4. Update established state from events
5. Start scheduler goroutines
6. Tick schedulers in advance loop using `vc.Now()`

### Step 6: Wire main.go flags

Pass `--chaos-rate`, `--chaos-interval`, `--route-rate`, `--route-interval`, `--warmup`, `--routes` to RunConfig.

### Step 7: Run tests and verify

```bash
make chaos-unit-test
make chaos-lint
```

### Failure Routing

| Failure | Route To |
|---------|----------|
| Simulator deadlocks with mock dialer | Step 1 — check ConnPairManager.NewPair() threading |
| Scheduler ticks but no events appear | Step 5 — verify established state updated before first tick |
| Reconnection fails | Step 5 — verify new pair queued on correct MockListener |

## Design Insights

### Why virtual time works with existing schedulers

The chaos/route schedulers' `Tick(now time.Time, established []bool)` takes wall time as a parameter. In normal mode, `now` comes from `time.Now()`. In in-process mode, we pass `vc.Now()` instead. The schedulers don't care about the clock source — they just compare `now` against their internal `lastTick` + interval. Virtual time advances in 1s steps, which matches the default `--chaos-interval 1s`.

### Why reconnection needs a dialer factory, not pre-registered connections

Chaos actions are stochastic — we don't know in advance how many reconnections will happen. Pre-registering N connections on the MockDialer doesn't work because N is unknown. Instead, the reconnect-dialer creates pairs on demand.

### ConfigReload action is no-op in-process

`ActionConfigReload` sends SIGHUP to a PID. In-process mode has no separate Ze process. The action is harmless (ZePID=0 means no-op in existing code) but won't test config reload behavior.

## Checklist

### Goal Gates
- [ ] AC-1: chaos events in in-process mode
- [ ] AC-2: route events in in-process mode
- [ ] AC-3: reconnection after chaos disconnect
- [ ] AC-6: no regression when chaos disabled
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] `make ze-lint` passes

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
