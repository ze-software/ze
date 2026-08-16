# Spec: improve-5 -- Panic Containment at Network-Input Task Boundaries

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context
4. `internal/component/bgp/reactor/session.go` -- Run loop (the main gap)
5. `ai/rules/goroutine-lifecycle.md` -- worker goroutine rules

## Task

A panic anywhere in BGP message processing kills the whole daemon: `Session.Run` reads
and processes peer messages inline with no recover boundary, so one malformed-input or
logic bug in a decode/RIB path takes down every peer, the config system, and (on a
gokrazy appliance, where Ze owns the process lifecycle) potentially the device's
control plane. Ze already contains panics at several task boundaries -- the session
cancel goroutine, the reactor monitor, the listener accept loop, engine event dispatch
-- so the pattern is established; the highest-exposure boundary, the per-session
read/process loop that consumes untrusted network input, is the one without it.

Add explicit recover boundaries at network-input task boundaries: recover at the
session/protocol task level (NOT inside low-level decode loops), log structured context
(peer, message type, stack), count it in metrics, close that peer's session, and keep
the process alive. Audit sibling network-input loops (L2TP, IKE, DNS, OSPF/IS-IS
receive paths) for the same boundary and apply the same policy where missing.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/goroutine-lifecycle.md` - long-lived worker rules
  → Constraint: (fill during design)
- [ ] `docs/architecture/core-design.md` - session/reactor layering
  → Constraint: recover must sit where session teardown is already well-defined
- [ ] `ai/rules/cli.md` - panic log content requirements
  → Constraint: log must name peer, state, and next action without reading source

### RFC Summaries (MUST for protocol work)
- `rfc/short/rfc4271.md` - session teardown expectations when processing fails
  → Constraint: (fill during design -- whether a NOTIFICATION can/should be sent from a recover path)

**Key insights:**
- This changes failure blast radius only; message processing semantics are untouched.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/session.go` - `Run` (:706-797) drives `readAndProcessMessage`/`readAndProcessCoalesced` (:773-777) with NO recover; the recover at :713-725 covers only the cancel goroutine
- [ ] `internal/component/bgp/reactor/session_read.go` - buffer-return defer (:64-71) is resource cleanup that RUNS during a panic unwind but does not stop it
- [ ] `internal/component/bgp/reactor/reactor.go` - `monitor` recovers (:1503-1512): precedent for the pattern
- [ ] `internal/component/bgp/reactor/listener.go` - accept-loop recovers (:156, :208): precedent
- [ ] `internal/component/plugin/server/engine_event.go` - event dispatch recovers (:129): precedent
- [ ] Sibling receive loops to audit during design: `internal/component/l2tp/`, `internal/plugins/ospf/`, `internal/core/dnsserver/handler.go` (:51 already recovers)

**Behavior to preserve:** (unless user explicitly said to change)
- Normal error paths (parse errors, NOTIFICATION handling, FSM events) unchanged;
  recover fires only on actual panics.
- Buffer pool discipline: the existing `kept` defer in `session_read.go` must
  still return buffers correctly when unwinding stops at the new boundary.
- Existing recovers (cancel goroutine, monitor, listener, engine events) stay.

**Behavior to change:** (only if user explicitly requested)
- A panic in one session's processing no longer terminates the daemon; it terminates
  that session.

## Data Flow (MANDATORY)

### Entry Point
- Untrusted peer bytes enter at `Session.Run` -> `readAndProcessMessage` (`session.go`);
  a panic anywhere below (decode, attribute parsing, RIB callbacks) unwinds to Run's
  goroutine and today kills the process.

### Transformation Path
1. Recover boundary installed at the top of the session task (Run or its per-iteration processing call, decided at design against defer-cost measurements).
2. On panic: capture stack, log structured context (peer address, FSM state, last message type), increment a per-peer panic metric.
3. Session closes through the EXISTING teardown path (close reason set, connection closed, FSM to Idle), exactly as a fatal read error would.
4. Reactor-level peer restart policy decides whether the peer may re-establish (design decision: treat like any session error vs quarantine after N panics).
5. Design-phase audit table lists every other network-input loop and whether it gained the same boundary.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Panic unwind ↔ session teardown | recover converts panic to the existing close-reason error path | [ ] |
| Session ↔ reactor | teardown/restart signaling unchanged | [ ] |
| Recover ↔ metrics/logs | structured log + counter, no allocation before recover fires | [ ] |

### Integration Points
- `Session.Run` (`session.go`) - boundary location.
- `setCloseReason`/`closeConn` teardown path (`session.go`) - reused for post-panic close.
- Prometheus metrics registry - panic counter.

### Architectural Verification
- [ ] No bypassed layers (post-panic close uses the normal teardown path)
- [ ] No unintended coupling (no recover sprinkled in decode internals)
- [ ] No duplicated functionality (one boundary per task, matching existing precedents)
- [ ] Registration over hardcoding -- N/A for control flow; any new metric registers via the existing registry (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Session state after an arbitrary panic is safe to tear down via the normal path (no locks held across processing calls) | teardown already handles abrupt connection loss | Need lock-audit; recover must also release/abandon state | Design-phase audit of mutexes held during readAndProcess* | unvalidated |
| A-2 | A deferred recover at task level has negligible steady-state cost | Go defer is cheap on modern runtimes; boundary is per-iteration or per-task | Move boundary to per-task only | Benchmark hot path before/after | unvalidated |
| A-3 | Shared state (RIB, pools) touched mid-panic is left consistent enough for OTHER sessions to continue | pools use get/return discipline | Containment gives false safety; process restart is more honest for some panic sites | Design review of processMessage side-effect points; document which panics are NOT safely containable | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Recover masks real bugs (peers flap forever instead of crash reports) | panic counter > 0 in soak/CI | loud ERROR log with stack + metric; optional env knob to re-panic in dev/test builds |
| R-2 | Post-panic state corruption spreads to other peers (A-3 wrong) | weird cross-peer failures after a contained panic | quarantine option: after panic, stop accepting that peer; document non-containable sites |
| R-3 | Boundary placed too low (inside decode) contradicts the design intent | code review | review checklist item: recover only at task boundary |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| injected panic in message processing (test hook) | → | session recover boundary -> teardown -> daemon alive | TestSessionPanicContainment |
| panic during coalesced processing | → | same boundary covers coalesced path | TestSessionPanicContainmentCoalesced |
| two peers, one panics | → | other session unaffected | TestPanicIsolationBetweenPeers |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Panic injected in session processing | Daemon stays up; that session closes via normal teardown; FSM reaches Idle |
| AC-2 | Panic occurs | ERROR log with peer, FSM state, panic value, stack; panic metric incremented |
| AC-3 | Second peer active during first peer's panic | Second session continues; its routes unaffected |
| AC-4 | Read buffers held at panic time | Returned to pool (no leak, pool counters balance) |
| AC-5 | Normal operation (no panic) | No behavior or measurable hot-path performance change |
| AC-6 | Design-phase audit output | Table in this spec listing every network-input loop and its boundary status |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Malicious/buggy peer triggers a processing panic on an appliance | recover -> log/metric -> peer down, device control plane alive | TestSessionPanicContainment |
| 2 | Operator investigates | reads structured panic log + metric, files bug with stack | log content assertion in TestSessionPanicContainment |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestSessionPanicContainment | `internal/component/bgp/reactor/session_panic_test.go` | AC-1, AC-2, AC-4 | |
| TestSessionPanicContainmentCoalesced | same file | coalesced path covered | |
| TestPanicIsolationBetweenPeers | same file | AC-3 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none numeric; N/A -- control-flow change) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A -- panic injection needs a test-build hook; not expressible as a .ci scenario against a release binary. Coverage is Go tests above plus existing .ci suites proving no regression in normal session behavior | - | - | |

### Interop Tests (MANDATORY for protocol features)
- N/A: no wire behavior change; existing interop suites regression-guard normal paths.

## Files to Modify
- `internal/component/bgp/reactor/session.go` - recover boundary in Run/processing call
- `internal/component/bgp/reactor/session_read.go` - verify buffer-return interaction (AC-4)
- metrics registration for the panic counter (owning package)
- Sibling loops from the AC-6 audit (list fixed during design)

## Files to Create
- `internal/component/bgp/reactor/session_panic_test.go` - containment tests (with test-only panic hook)

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - test-only panic injection hook + failing TestSessionPanicContainment
2. **Phase: session boundary** - recover, structured log, metric, teardown reuse
3. **Phase: lock/state audit (A-1, A-3)** - document containable vs non-containable sites in this spec
4. **Phase: sibling-loop audit (AC-6)** - apply boundary where missing
5. `make ze-precommit-verify` including race tests, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-6 with file:line |
| Correctness | recover at task boundary only; teardown path reused; no recover in decode internals |
| Performance | hot path unchanged (A-2 benchmark) |
| Registration over hardcoding | metric registered via existing registry (`ai/rules/plugins.md`) |
| Rule: anti-rationalization | R-1 knob: containment never downgrades the log below ERROR |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | unchanged; this is blast-radius control, not validation |
| DoS surface | repeated-panic peer cannot spin reconnect/panic loops for free (quarantine decision, R-2) |
| Error leakage | panic logs go to local logs/metrics only, never on the wire |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- (fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Contain at session task boundary | crash-fast (status quo); recover per decode call | matches existing Ze precedents (monitor, listener, engine events); appliance target makes process death a device outage |

## Known Limitations
- Panics that corrupt shared state (A-3 audit output) may still require process
  restart; the audit documents which, and containment never pretends otherwise.

## RFC Documentation

Add `// RFC 4271 Section X.Y` comments only if the design decides a NOTIFICATION is
sent from the recover path (see Required Reading constraint).

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-standard-test` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Race detector clean on new tests

### Post-wave corrections (2026-07-10)

The followup wave added new network-input listen paths that the AC-6 audit table must enumerate (all re-verified in current code):

- `bindDoT` (`internal/core/dnsserver/secure.go`) -- DNS-over-TLS (RFC 7858) TCP listener.
- `bindDoH` (`internal/core/dnsserver/secure.go`) -- DNS-over-HTTPS (RFC 8484) listener; its serve goroutine is `serveHTTP` (`secure.go`).
- The UDP/TCP accept-loop goroutine `serve` (`internal/core/dnsserver/manager.go`, launched at `manager.go`); `bindDoT` reuses the same `serve` loop.

Boundary status of these paths: the DoT server is built with the manager's handler (`secure.go`) and the DoH handler dispatches into it (`secure.go`); as112 (`internal/plugins/as112/server.go`) and geodns (`internal/plugins/geodns/server.go`) construct that handler via `dnsserver.Authoritative`, whose per-query recover sits at `internal/core/dnsserver/handler.go`. The new listeners therefore inherit the existing recover boundary, but the AC-6 audit table must list them explicitly with that inheritance noted; the spec's Current Behavior sibling-loop list (which cites only `handler.go`) predates these listeners.
