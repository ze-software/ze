# Spec: fixit-ddos-incident-heartbeat-wiring

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** `(*client).heartbeat` has no non-test caller. It is defined in
the ddos incident-reporting plugin's client and called only from that client's
`_test.go` (two calls, both in `client_test.go`). Ze therefore never reports
liveness, baseline readiness, or circuit-breaker state to the incident API, so a
node whose detector has silently stopped is indistinguishable from a node with
no attacks. The remote side cannot tell the two apart, and neither can the
operator reading the remote side.

**What exists and works.** `(*client).heartbeat` posts to `/agent/heartbeat`
with `version`, `baseline_ready`, `baseline_avg_pps` and `baseline_p99_pps`, and
goes out through `(*client).post` like every other call, so it inherits the
circuit breaker, the 10 second HTTP timeout and the 1 KiB error-body cap. The
`circuitBreaker` type itself works and counts genuinely consecutive failures:
`recordFailure` trips at `tripAt` of 5 and sets `trippedUntil` to 60 seconds
ahead, `recordSuccess` resets the count on any 2xx or 3xx, and `tripped` clears
the count once the wait has passed. There is no half-open probe. Nothing
publishes the breaker's state anywhere.

**Why it was recorded rather than fixed (2026-08-15).** It is dead code rather
than wrong code, so nothing observable is currently incorrect. Wiring it needs a
tick source in `runEngine` and a decision on cadence and payload, which is a
small feature rather than a line fix.

**What the work is.** Give the heartbeat a caller, which means answering four
questions the code does not answer today:

1. **The tick source.** `runEngine`
   (`internal/plugins/ddos/flowtriq/register.go`) subscribes to four detector
   events and then blocks in `p.Run`. It holds no ticker, so the heartbeat needs
   one whose lifetime ends with the plugin's.
2. **The cadence.** Fast enough that a stopped detector is noticed, slow enough
   that a long API outage does not spend the circuit breaker on heartbeats.
3. **The payload.** `heartbeat` takes baseline readiness and two baseline rates
   today. The row's point is liveness AND breaker state, and the breaker state
   has no accessor, so the payload decision reaches back into the client.
4. **Ownership of the state it reads.** The reporting state now lives behind one
   mutex in `reporter` (`internal/plugins/ddos/flowtriq/reporter.go`), taken for
   the whole of each writer after a live data race was fixed on 2026-09-02. A
   heartbeat that reads the client or the incident state is a fifth writer and
   must take the same lock, for the whole of its own work.

**Naming note.** The plugin directory carries a product name the owner asked to
keep out of these records. The symbols named here are unique in the tree, so
`gopls` or a plain grep resolves each one; the paths are given because the
validation hook requires a source path in Current Behavior.

## Required Reading

### Architecture Docs
- [ ] `docs/integrations/flowtriq-api.md` - what the remote API expects on the heartbeat endpoint
  → Decision: [fill during research]
  → Constraint: [fill during research]
- [ ] `ai/rules/goroutine-lifecycle.md` - what a ticker goroutine in a plugin owes
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- The reporting state has one owner and one mutex since 2026-09-02; the heartbeat is a fifth writer, not a reader outside the lock.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ddos/flowtriq/client.go` - `(*client).heartbeat` builds the four-field body and posts it through `(*client).post`; `circuitBreaker` trips at 5 consecutive failures, waits 60 seconds, resets on success, and exposes its state to nobody.
- [ ] `internal/plugins/ddos/flowtriq/register.go` - `runEngine` builds a `reporter`, subscribes to the four detector events, and blocks in `p.Run`; it starts no ticker and calls no heartbeat.
- [ ] `internal/plugins/ddos/flowtriq/reporter.go` - `reporter` holds the incident state behind `mu`, taken for the whole of each of the five writers.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The circuit breaker's consecutive-failure counting, its 5 and 60 second constants, and its reset on success.
- The incident open, update and resolve ordering the reporter holds.
- The 10 second HTTP timeout and the 1 KiB error-body cap.

**Behavior to change:** (only what the user asked for)
- Ze sends a heartbeat while the plugin runs, carrying liveness, baseline readiness and circuit-breaker state.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A timer tick inside the reporting plugin's process, once the plugin has a configured client.
- Detector state arrives on the event bus as `ddosevent` values; baseline readiness is read from the detector's published state.

### Transformation Path
1. The tick fires inside the plugin.
2. The reporter reads liveness, baseline readiness and breaker state under its own lock.
3. `(*client).heartbeat` posts the body to `/agent/heartbeat` through `(*client).post`.
4. The breaker records success or failure.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ remote API | HTTP POST with a bearer token and a node UUID header | No |
| Detector ↔ reporting plugin | event bus values, delivered on the publisher's goroutine | No |

### Integration Points
- `reporter` (`internal/plugins/ddos/flowtriq/reporter.go`) - the state owner the heartbeat has to go through.
- `(*client).post` (`internal/plugins/ddos/flowtriq/client.go`) - the one outbound path, with the breaker on it.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The remote API accepts the body `heartbeat` already builds | the endpoint and the four fields are written in `client.go` | the payload has to change and the doc page is wrong | reading `docs/integrations/flowtriq-api.md` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A heartbeat during an API outage spends the circuit breaker that incident posts need | incidents stop being reported while heartbeats fail | decide the cadence against the breaker's 5-failure trip and its 60 second wait |
| R-2 | The ticker goroutine outlives the plugin's config apply or its shutdown | a goroutine leak under repeated apply | tie the ticker's lifetime to the same context the subscriptions use |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the incident API is flooded, or the breaker opens and real incidents are lost |
| How is it reverted? | single commit revert |
| Who else touches this path? | the two other rows of the same 2026-08-15 audit: outbound durability, and incident lifecycle on teardown |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the plugin runs with a configured client | → | the tick that calls `(*client).heartbeat` | `TestRunningPluginSendsAHeartbeat` |
| the circuit breaker is open | → | the heartbeat payload | `TestHeartbeatReportsTheBreakerState` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the plugin is configured and running | a heartbeat reaches the API at the configured cadence |
| AC-2 | the detector's baseline is not ready | the heartbeat says so |
| AC-3 | the circuit breaker is open | the heartbeat carries that state once the breaker allows a post |
| AC-4 | the plugin stops or is reconfigured | the ticker goroutine ends with it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunningPluginSendsAHeartbeat` | `internal/plugins/ddos/flowtriq/reporter_test.go` | AC-1 | |
| `TestHeartbeatReportsTheBreakerState` | `internal/plugins/ddos/flowtriq/client_test.go` | AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-incident-heartbeat` | `test/plugin/ddos-incident-heartbeat.ci` | a running node reports liveness to the incident API | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->

## Files to Modify
- `internal/plugins/ddos/flowtriq/register.go` - the tick source and its lifetime
- `internal/plugins/ddos/flowtriq/reporter.go` - the heartbeat's read of the shared state, under the existing lock
- `internal/plugins/ddos/flowtriq/client.go` - the breaker state accessor the payload needs

## Files to Create
- [named at design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | the cadence, if it is operator-settable |
| Functional test for new RPC/API | | `test/plugin/ddos-incident-heartbeat.ci` |
| Doctor check for runtime dependencies | | [answered at design] |
| Prometheus counters/metrics | | [answered at design] |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/integrations/flowtriq-api.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- start the ticker and write the failing test that a running plugin heartbeats
   - Tests: `TestRunningPluginSendsAHeartbeat`
   - Files: `internal/plugins/ddos/flowtriq/register.go`
   - Verify: the test fails because nothing calls the heartbeat
2. **Phase: [named at design]**

## Known Limitations
- [filled at design]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
