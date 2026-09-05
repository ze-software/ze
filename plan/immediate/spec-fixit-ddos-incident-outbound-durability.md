# Spec: one failed incident open discards the whole DDoS incident

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

A single transport blip during the opening seconds of an attack costs the whole
incident report, start to end, and the operator sees one WARN line.

Found by the ddos detect audit on 2026-08-15. The state the row described lived
in `runEngine` closure variables then; a race fix on 2026-09-02 moved it into
`reporter` (`internal/plugins/ddos/flowtriq/reporter.go`), which owns it behind
one mutex. The durability defect survived that move unchanged:

- `(*reporter).onDetected` calls `(*client).openIncident`. On error it logs
  `logger().Warn("ddos-flowtriq: open incident failed")` and returns, leaving
  `r.uuid` empty.
- `(*reporter).onOngoing` returns early when `r.uuid == ""`, and
  `(*reporter).resolveLocked`, which `onCleared` and `swapClient` both call,
  answers `had == false` on the same condition. So the Ongoing updates and the
  Cleared resolve are discarded too.
- `(*client).post` is fire-and-forget. On a transport error or a status of 400
  or above it calls `c.cb.recordFailure()` and returns the error. There is no
  buffer and no re-queue anywhere in the package.

The guard is correct in isolation: sending an Ongoing or a Resolve for an
incident the remote side never opened would be worse than sending nothing. The
defect is upstream of it. There is no path by which a failed open is retried, so
`uuid` cannot become non-empty later in the same attack.

The fix is a durable outbound path, and it is a design decision about ordering
and bounding rather than a line fix. Two shapes were named in the audit: a
bounded outbound queue drained by the existing circuit breaker, which keeps the
open-before-update-before-resolve ordering and takes the bound the breaker
already implies; or deferring the open until the first successful post, which is
cheaper but changes the remote start timestamp the incident record is keyed on.

**Bounding is not optional.** Ze has no outbound buffer at all today, which is
why it has no unbounded-growth risk. Any queue added here inherits that risk
during a long API outage, so the bound belongs in this spec and not in review.

**What is already right and must survive the fix.** `circuitBreaker` counts
genuinely consecutive failures, opens at 5 and resets on any 2xx or 3xx.
`(*client).post` caps error bodies with a 1 KiB `io.LimitReader`, so a hostile
or broken API cannot balloon a log line. The HTTP client carries a 10 second
timeout. `reporter.mu` is held for the WHOLE of each writer rather than around a
copy, because a copy-under-lock shape leaves a test-then-post window open, and
`ApplyBudget` is 20 seconds because an apply now waits for a post in flight.
None of these should be disturbed.

This is not a defect in detection. The detector still detects, mitigates and
emits its own events; this plugin is a reporting sink, and what leaks is state
on the remote API rather than memory on the node.

**Test surface.** `test/plugin/ddos-incident-confidence.ci` and the other twelve
`ddos-*.ci` fixtures cover the detector and its responders, and
`./le functional plugin` is the owning gate. No fixture drives the reporting
client against a failing API, so this spec owes a new one asserting that an
incident survives a failed open, which is the assertion that would fail today.

Two sibling rows from the same audit are separate work and are NOT in scope
here: an open incident never resolved on teardown, and `(*client).heartbeat`
having no non-test caller.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` - the DDoS event contract, declared by both source files
  → Constraint: <to be filled>
- [ ] `ai/patterns/functional-test.md` - the structure of the new fixture
  → Decision: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ddos/flowtriq/reporter.go` - `(*reporter).onDetected` logs and returns on a failed `openIncident`, leaving `uuid` empty; `onOngoing` and `resolveLocked` both no-op while it is empty
- [ ] `internal/plugins/ddos/flowtriq/client.go` - `(*client).post` records a circuit-breaker failure and returns the error, with no buffer and no re-queue; the HTTP client has a 10 second timeout and error bodies are read through a 1 KiB `io.LimitReader`
- [ ] `internal/plugins/ddos/flowtriq/register.go` - `runEngine` subscribes the four reporter callbacks to the ddos event bus and declares `ApplyBudget: 20`

**Behavior to preserve:** (unless the user explicitly said to change it)
- the circuit breaker's consecutive-failure count and its reset on any 2xx or 3xx
- the 1 KiB cap on a logged error body and the 10 second HTTP timeout
- the ordering guarantee: open before update before resolve
- `reporter.mu` held for the whole of each writer

**Behavior to change:** (only what the user asked for)
- a failed open must not discard the rest of the incident

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ddosevent.Detected`, `Characterized`, `Ongoing` and `Cleared`, delivered inline on the detector's publishing goroutine
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| detector ↔ reporting plugin | typed event bus, delivered inline | No |
| reporting plugin ↔ remote API | HTTP POST through `(*client).post` | No |

### Integration Points
- `internal/core/ddosevent/` - the event types the reporter subscribes to
- `internal/plugins/ddos/detect/detector.go` - the publisher

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | <to be filled> | <to be filled> | <to be filled> | <to be filled> | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | an unbounded queue grows through a long API outage | memory growth on a node with no attack traffic | the bound is decided in this spec |
| R-2 | a drain goroutine holds `reporter.mu` across a post and stalls a config apply | the apply budget is exceeded | <to be filled> |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | incident records on the remote API, and node memory during an API outage |
| How is it reverted? | <to be filled> |
| Who else touches this path? | the two sibling audit rows: incident lifecycle on teardown, and heartbeat wiring |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ddosevent.Detected` with the API refusing the open | → | <to be filled> | <to be filled> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the API refuses the open, then accepts later posts | the incident is opened and its updates and resolve reach the API in order |
| AC-2 | the API is unreachable for the whole attack | the queue stays inside its stated bound and the oldest or newest excess is dropped by a stated rule |
| AC-3 | the API accepts every post | the behavior is unchanged from today |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | reads an incident on the API after a transport blip at attack start | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <to be filled> | `internal/plugins/ddos/flowtriq/` | <to be filled> | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| queue bound | <to be filled> | <to be filled> | <to be filled> | <to be filled> |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| <to be filled> | `test/plugin/` | an incident survives a failed open | |

## Files to Modify
- `internal/plugins/ddos/flowtriq/reporter.go` - <to be filled>
- `internal/plugins/ddos/flowtriq/client.go` - <to be filled>

## Files to Create
- `test/plugin/` - a fixture driving the reporting client against a failing API

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | the queue bound, if it is configurable |
| Prometheus counters/metrics | | queue depth and dropped posts |
| Functional test for new RPC/API | | `test/plugin/` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 12 | Internal architecture changed? | | `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- <to be filled>
2. **Phase: <to be filled>**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | the open-before-update-before-resolve order holds across a retry |
| Data flow | the bound is enforced at one place and stated in the docs |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| <to be filled> | <to be filled> |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
- <to be filled>

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
