# Spec: subscriber-reader-loops-retry-a-failing-socket-without-backoff

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/6 |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Four receiver goroutines on the subscriber path answer a socket read error by
immediately reading again. A persistent error spins a core at 100% for as long as
the daemon runs, and two of the four do it silently.**

Each loop distinguishes exactly one error: the socket is closed, or the context is
done, in which case it returns. Every other error takes a bare `continue`. There
is no delay, no error counter, no escalation and no ceiling. When the failing
condition is persistent rather than transient, and a read that fails returns
immediately, the loop becomes a busy loop that does no work.

| Loop | File | On error |
|------|------|----------|
| `(*UDPListener).readLoop` | `internal/component/l2tp/listener.go` | Recycles the buffer slot and continues, with no log line |
| `discoveryReader` | `internal/component/l2tp/pppoe/subsystem.go` | Logs at debug and continues |
| `rsReaderLoop` | `internal/component/l2tp/ppp/ra_linux.go` | Continues, with no log line |
| `dhcpv6ServerLoop` | `internal/component/l2tp/ppp/dhcpv6_linux.go` | Logs at debug and continues |

A crafted packet cannot trigger this: every one of the four bounds-checks its
input and drops a malformed frame on a separate path. What triggers it is a
socket-level failure that does not clear, and on a router terminating subscriber
sessions the CPU that burns is the CPU the remaining sessions need.

Ze's PPPoE client carries a bounded relative: `waitForPADO` and `waitForPADS`
poll with `runtime.Gosched()` inside a `select` default arm. The socket timeout
caps each poll, and the discovery timeout ends the loop, so it burns at most one
core for the discovery window. It is in scope because it is the same shape and it
is cheap to bound properly while the other four are being fixed.

The goal: no receiver goroutine on the subscriber path can consume a core
indefinitely on a failing socket, and every error a loop swallows is visible to an
operator.

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` - the design document `listener.go`, `ra_linux.go` and `dhcpv6_linux.go` declare: the L2TP UDP transport, the RA sender and the DHCPv6-PD listener
  → Decision: `readLoop` owns a fixed slot pool created once at goroutine start and allocates nothing per packet, so any delay added must not allocate per iteration either
  → Constraint: the listener's stop path is a closed channel plus a `closed` flag under a mutex, and the error path must keep checking both so a stopping listener still exits at once
  → Constraint: the page is silent on what any of these loops does with an error that is neither "closed" nor "context done", so it gains that statement in this work
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - the design document `subsystem.go` declares: the PPPoE subsystem lifecycle and its `AF_PACKET` discovery reader
  → Constraint: one discovery reader serves every configured access interface through an ifindex lookup, so a delay applied there stops discovery on all of them at once and must therefore be short and bounded
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` - the design document `dialer.go` declares: the client's discovery poll
  → Constraint: the client's read uses `SO_RCVTIMEO` at roughly 100ms, so the poll already has a natural pace and needs a blocking read rather than a yield, not a longer timer
- [ ] `docs/architecture/core-design.md` - where a leaf helper belongs and what may import what
  → Constraint: `internal/component/l2tp` imports `internal/component/l2tp/ppp`, so `ppp` cannot import its parent. A helper shared by all four call sites cannot live in the `l2tp` package and belongs in a leaf under `internal/core/`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2516.md` - PPPoE discovery, read to confirm no timer in this spec changes a protocol-visible interval
  → Constraint: the delay applies to a failing socket only. No conformant peer exchange reaches the error path, so no RFC timer is affected

**Key insights:**
- The bug is not the retry, it is the absence of a floor on how fast the retry may happen. A transient error should be retried at once; a persistent one must not be retried at full speed forever.
- Two of the four are silent, which is the worse half: a spinning core with no log line is diagnosed by profiling rather than by reading a log.
- The import direction is already fixed and it decides where the helper lives, so this is not an open design question.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/listener.go` - `(*UDPListener).readLoop` takes a slot from `freeCh`, reads with `ReadFromUDPAddrPort`, and on error checks the `closed` flag under the mutex, returns when closed, and otherwise returns the slot to `freeCh` and continues with no log and no delay. The adopted-tunnel-socket loop in the same file returns on any read error and is not affected
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `discoveryReader` reads a discovery frame, returns on `errSocketClosed`, and otherwise logs at debug and continues with no delay
- [ ] `internal/component/l2tp/ppp/ra_linux.go` - `rsReaderLoop` checks `ctx.Err()` at the top and after a failed `ReadFrom`, returns when the context is done, and otherwise continues with no log and no delay
- [ ] `internal/component/l2tp/ppp/dhcpv6_linux.go` - `dhcpv6ServerLoop` checks `ctx.Err()` at the top and after a failed `ReadFrom`, returns when the context is done, and otherwise logs at debug and continues with no delay
- [ ] `internal/component/l2tp/pppoeclient/dialer.go` - `waitForPADO` and `waitForPADS` run a `for` with a `select` whose default arm calls a non-blocking read helper and then `runtime.Gosched()`; the socket carries `SO_RCVTIMEO` at roughly 100ms and the loop ends at `discoveryTimeout`
- [ ] `internal/component/l2tp/ppp/session_run.go` - `readFrames` reads one PPP frame per iteration from the channel file and re-reads on a short read; read for comparison, and it is bounded by the blocking read

**Behavior to preserve:**
- Every loop exits at once when its socket is closed or its context is done. A stopping listener must not wait out a delay.
- `readLoop` allocates nothing per packet: the slot pool, the free channel and the release closures stay created once at goroutine start.
- A transient error is still retried, and the first retry after a success is still immediate.
- The discovery reader keeps serving every access interface from one goroutine.
- No protocol-visible timer changes.

**Behavior to change:**
- A repeated read error introduces a bounded, growing delay before the next attempt, reset by the next successful read.
- The two silent loops log their error.
- Each loop's swallowed errors are counted so an operator can see a socket failing rather than inferring it from CPU use.
- The client's discovery poll blocks on its socket timeout rather than yielding.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Not a packet. The entry point is a failing socket read: `ReadFromUDPAddrPort` on the L2TP UDP listener, `recvfrom` on the PPPoE `AF_PACKET` socket, `ReadFrom` on the ICMPv6 Router Solicitation socket, and `ReadFrom` on the DHCPv6 UDP socket bound to a PPP interface.
- Format at entry: an error value from the read call, plus the loop's own liveness signal, which is the `closed` flag for the listener and `ctx.Err()` for the others.

### Transformation Path
1. The read call returns an error.
2. The loop tests its exit condition: `closed` under the mutex, or `ctx.Err()`.
3. Not exiting, the loop today recycles its state and reads again immediately.
4. After this work, the loop consults a retry pacer, which decides how long to wait and returns at once when the loop's exit signal fires during the wait.
5. A successful read resets the pacer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel → daemon | The four socket read calls and their error returns | No |
| Loop → operator | The log line and the error counter each loop publishes | No |
| Component → core | The four loops import one leaf pacer from `internal/core/` | No |
| Loop → lifecycle | The pacer's wait observes the same stop channel or context the loop already owns | No |

### Integration Points
- `internal/core/` - the new leaf package holding the pacer, importable by `l2tp`, `l2tp/ppp` and `l2tp/pppoe` alike.
- `(*UDPListener).readLoop` (`listener.go`) - the one loop whose wait must also respect `u.stop` rather than a context.
- `discoveryReader` (`subsystem.go`), `rsReaderLoop` (`ra_linux.go`), `dhcpv6ServerLoop` (`dhcpv6_linux.go`) - the three context-driven loops.
- The subscriber telemetry registration - where the per-loop error counter is declared.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A read error that is neither "closed" nor "context done" can persist rather than clear on the next call | The four loops treat every such error as retryable without evidence that it is | The loops are already correct and only the missing log lines and counters are owed | Drive each loop against a socket in a persistently failing state and measure CPU use over ten seconds | unvalidated |
| A-2 | `internal/component/l2tp/ppp` cannot import `internal/component/l2tp` | `internal/component/l2tp/config.go` imports `internal/component/l2tp/ppp` | The helper may live in the `l2tp` package and no new core leaf is needed | `go list` on the import graph, and `./le tier check` | unvalidated |
| A-3 | Delaying `discoveryReader` on error does not delay any healthy interface, because a healthy socket does not reach the error path | One reader serves every interface through one `AF_PACKET` socket and an ifindex lookup | A per-interface failure stalls discovery on every other interface, and the pacer must move per socket rather than per loop | Read `readDiscoveryFrame` and the socket setup in `subsystem.go`, and drive a two-interface test with one interface failing | unvalidated |
| A-4 | No existing helper in the repository already does this | Read at the producer 2026-09-09, and CONFIRMED: none of the three is reusable. `probe.currentHoldDown = min(p.currentHoldDown*2, p.backoffCap)` (`internal/plugins/ddos/flowspec/probe.go`) doubles a hold-down inside a flowspec probe state machine, so the growth is domain policy rather than a retry pace. `conntrackDestroyBackoff` (`internal/plugins/flowexport/conntrack_worker.go`) is a FIXED 100ms `time.Sleep` after an error threshold: constant, not growing, and the bare `time.Sleep` ignores any stop signal, which is the same uninterruptible-wait defect this spec exists to remove. `respawn.go` (`internal/plugins/exabgp/bridgerun/`) says in its own header "There is no backoff" and counts forks in a window | The new leaf is unnecessary and one of the three is extended instead | Read all three producers, not their call sites | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A delay on a transient error drops packets that would have been read | A functional test sees a session set-up delay under induced errors | The first retry after a success stays immediate, and the delay only grows across consecutive failures |
| R-2 | The wait blocks the stop path, so a listener takes up to the ceiling to shut down | A shutdown test exceeds its deadline | The wait selects on the loop's existing stop channel or context and returns at once when it fires; a shutdown test asserts the exit time under a maxed-out pacer |
| R-3 | The pacer allocates per iteration and breaks `readLoop`'s allocation discipline | An allocation benchmark on the listener regresses | The pacer is a value type holding a counter and a deadline, created once per goroutine, with a timer reused rather than created per wait |
| R-4 | A fifth backoff implementation lands beside three existing ones, so the repository now declares the same idea four times | A reviewer finds `ddos/flowspec`, `flowexport` and `bridgerun` doing this already | A-4 forces reading all three first; if one is already a usable leaf, it is reused and this spec adds no package |
| R-5 | The counter is added but nothing surfaces it, so the failure is still invisible in practice | The metric exists and no command or dashboard shows it | The counter is registered with the existing subscriber metrics and named in the documentation row, so it appears where the other subscriber counters do |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A receiver goroutine pauses when it should read, delaying session set-up, or a listener takes longer to stop than its shutdown deadline allows |
| How is it reverted? | Single commit revert. No config migration and no wire-visible change |
| Who else touches this path? | `spec-pppoe-padr-replay-allocates-unbounded-sessions` changed the PPPoE handlers this reader feeds, not the reader itself, and closed on 2026-09-09; `spec-pppoe-discovery-omits-the-mandatory-service-name-tag` did the same before it closed on the same day |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A persistently failing L2TP UDP socket | → | `(*UDPListener).readLoop` error path (`listener.go`) | `TestListenerReadLoopPacesAFailingSocket` |
| A persistently failing PPPoE `AF_PACKET` socket | → | `discoveryReader` error path (`subsystem.go`) | `TestDiscoveryReaderPacesAFailingSocket` |
| A persistently failing ICMPv6 socket | → | `rsReaderLoop` error path (`ra_linux.go`) | `TestRSReaderLoopPacesAFailingSocket` |
| A persistently failing DHCPv6 socket | → | `dhcpv6ServerLoop` error path (`dhcpv6_linux.go`) | `TestDHCPv6ServerLoopPacesAFailingSocket` |
| Stop or context cancellation during a pacer wait | → | the pacer's wait (`internal/core/`) | `TestPacerWaitReturnsOnStop` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A socket returns the same error on every read for one second, in each of the four loops | The loop performs a bounded number of read attempts rather than an unbounded one, and the goroutine's CPU time over that second is a small fraction of one core |
| AC-2 | A single error followed by a successful read | The retry after the error is immediate and the successful read is not delayed |
| AC-3 | A successful read after a run of consecutive errors | The pacer resets, so the next error is retried immediately again |
| AC-4 | The listener is stopped while its pacer is waiting at the ceiling | `readLoop` returns within the listener's existing shutdown deadline, not after the remaining wait |
| AC-5 | The context is cancelled while any of the three context-driven loops is waiting | The loop returns at once |
| AC-6 | Any of the four loops swallows a read error | The error is logged with the socket it came from, including in the two loops that log nothing today |
| AC-7 | A run of read errors on any loop | A counter names that loop and rises once per swallowed error |
| AC-8 | `readLoop` under a packet load with no errors | Allocations per packet are unchanged from before this work |
| AC-9 | Ze's PPPoE client is waiting for a PADO with no frames arriving | The wait blocks on the socket timeout rather than yielding in a spin, and the discovery timeout still ends it at the same moment |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a router whose subscriber socket starts failing and sees the daemon stay responsive | failing read → pacer → bounded retry | `TestListenerReadLoopPacesAFailingSocket` and the QEMU CPU assertion |
| 2 | Diagnoses the failure from the log and the counter rather than from a profile | failing read → log line → counter → show output | `TestReaderLoopsCountSwallowedErrors` |
| 3 | Stops the daemon while a socket is failing and it exits promptly | stop signal → pacer wait interrupted → goroutine returns | `TestPacerWaitReturnsOnStop` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPacerFirstRetryIsImmediate` | the new core leaf's test file | AC-2 | |
| `TestPacerGrowsToCeiling` | the new core leaf's test file | AC-1, the ceiling is reached and not exceeded | |
| `TestPacerResetsAfterSuccess` | the new core leaf's test file | AC-3 | |
| `TestPacerWaitReturnsOnStop` | the new core leaf's test file | AC-4 and AC-5 | |
| `TestListenerReadLoopPacesAFailingSocket` | `internal/component/l2tp/listener_test.go` | AC-1 for the listener, with a fake connection returning a persistent error | |
| `TestListenerReadLoopStopsPromptlyWhilePacing` | `internal/component/l2tp/listener_test.go` | AC-4 | |
| `TestDiscoveryReaderPacesAFailingSocket` | `internal/component/l2tp/pppoe/subsystem_test.go` | AC-1 for the PPPoE reader | |
| `TestRSReaderLoopPacesAFailingSocket` | `internal/component/l2tp/ppp/ra_test.go` | AC-1 and AC-6 for the RA reader, which logs nothing today | |
| `TestDHCPv6ServerLoopPacesAFailingSocket` | `internal/component/l2tp/ppp/dhcpv6_test.go` | AC-1 for the DHCPv6 listener | |
| `TestReaderLoopsCountSwallowedErrors` | `internal/component/l2tp/listener_test.go` | AC-7 | |
| `TestReadLoopAllocationsPerPacketUnchanged` | `internal/component/l2tp/listener_test.go` | AC-8, an allocation assertion over the read path | |
| `TestWaitForPADOBlocksRatherThanSpins` | `internal/component/l2tp/pppoeclient/dialer_test.go` | AC-9 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| consecutive errors before the first delay | 1 | the first error retries immediately | N/A | the second error waits |
| pacer delay | 0 to the ceiling | the ceiling | N/A | the ceiling is never exceeded |
| pacer ceiling | short enough that recovery is not noticeably delayed | to be fixed at design time and stated in the design page | N/A | N/A |
| stop latency while pacing | 0 to the listener's shutdown deadline | the deadline | N/A | exceeding it fails `TestListenerReadLoopStopsPromptlyWhilePacing` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `subscriber-reader-failing-socket` | `test/qemu/` | A subscriber socket is put into a persistently failing state and the daemon's CPU use stays low while the log and the counter show the failure | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-chap-ipv4` | `test/interop-pppoe/scenarios/01-pppoe-chap-ipv4` | accel-ppp | Session set-up timing is unchanged when no error occurs, which is the regression this work could cause | |

## Files to Modify
- `internal/component/l2tp/listener.go` - `readLoop` paces its error path, logs the error and counts it
- `internal/component/l2tp/pppoe/subsystem.go` - `discoveryReader` paces and counts
- `internal/component/l2tp/ppp/ra_linux.go` - `rsReaderLoop` paces, logs and counts
- `internal/component/l2tp/ppp/dhcpv6_linux.go` - `dhcpv6ServerLoop` paces and counts
- `internal/component/l2tp/pppoeclient/dialer.go` - `waitForPADO` and `waitForPADS` block on the socket timeout instead of yielding
- `docs/research/l2tpv2-ze-integration.md` - the design document `listener.go`, `ra_linux.go` and `dhcpv6_linux.go` declare: what a receiver goroutine does with an error it cannot classify
- `docs/architecture/l2tp/bng-5-pppoe.md` - the design document `subsystem.go` declares: the same statement for the discovery reader
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` - the design document `dialer.go` declares: the discovery wait blocks rather than polls
- `docs/architecture/core-design.md` - the new leaf's place in the tier layout

## Files to Create
- `internal/core/` - the retry pacer leaf package: a value type holding a consecutive-failure count and a reusable timer, with a wait that observes a stop channel or a context
- `test/qemu/` - the failing-socket scenario asserting CPU use, the log line and the counter

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The ceiling is an internal constant, not an operator choice: a leaf would be a knob nobody can set correctly |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No new verb; the counter appears through the existing metrics surface |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | N-A | No new RPC; the QEMU scenario covers the behavior |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or sysctl: the sockets are already opened by the current code |
| Prometheus counters/metrics | Yes | One counter per reader loop for swallowed read errors, registered with the existing subscriber metrics |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A resilience fix, not a feature an operator selects |
| 2 | Config syntax changed? | No | No leaf changes |
| 3 | CLI command added/changed? | No | No verb changes |
| 4 | API/RPC added/changed? | No | No RPC change |
| 5 | Plugin added/changed? | No | All four loops are component code |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` and `docs/guide/pppoe.md`, where the new counter is named as a diagnostic |
| 7 | Wire format changed? | No | Nothing wire-visible changes |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement changes; the delay applies only to a failing socket |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new QEMU scenario |
| 11 | Affects daemon comparison? | No | No comparison row changes |
| 12 | Internal architecture changed? | Yes | `docs/research/l2tpv2-ze-integration.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/architecture/l2tp/cpe-1-pppoe-client.md` and `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The four reader-error counters, in the subscriber telemetry section |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff.md`. `listener.go`, `ra_linux.go` and `dhcpv6_linux.go` declare `docs/research/l2tpv2-ze-integration.md`, `subsystem.go` declares `docs/architecture/l2tp/bng-5-pppoe.md`, and `dialer.go` declares `docs/architecture/l2tp/cpe-1-pppoe-client.md`; this spec names and edits all three |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example in these pages shows a reader loop or a timer |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the pacer exists as a leaf and one loop consults it
   - Tests: `TestPacerFirstRetryIsImmediate`, `TestListenerReadLoopPacesAFailingSocket`
   - Files: the new `internal/core/` leaf, `listener.go`
   - Verify: `./le tier check` accepts the placement, the listener calls the pacer, and the wiring test fails because the pacer is still a stub returning zero
2. **Phase: reuse before creation** -- ALREADY DONE, main thread, 2026-09-09. A-4 carries the verdict and the evidence: none of the three is a reusable pacer, so the new leaf stands. This phase remains in the list because a reader who skips A-4 must still be told the question was asked and answered, not skipped
   - Tests: none; the verdict is in A-4 and in Key Design Decisions
   - Files: read only, `internal/plugins/ddos/flowspec/probe.go`, `internal/plugins/flowexport/conntrack_worker.go`, `internal/plugins/exabgp/bridgerun/respawn.go`
   - Verify: no work outstanding. One thing to CARRY FORWARD: `conntrack_worker.go`'s bare `time.Sleep` is the same uninterruptible-wait class this spec removes, and it gets a journal row rather than a fix, since nothing in this spec's goal depends on it
3. **Phase: the pacer's behavior** -- growth, ceiling, reset and interruptible wait
   - Tests: `TestPacerGrowsToCeiling`, `TestPacerResetsAfterSuccess`, `TestPacerWaitReturnsOnStop`
   - Files: the core leaf
   - Verify: the wait returns at once on stop, and the timer is reused rather than created per wait
4. **Phase: the remaining three loops, and the log and counter for ALL FOUR**
   - Tests: `TestDiscoveryReaderPacesAFailingSocket`, `TestRSReaderLoopPacesAFailingSocket`, `TestDHCPv6ServerLoopPacesAFailingSocket`, `TestReaderLoopsCountSwallowedErrors`
   - Files: `subsystem.go`, `ra_linux.go`, `dhcpv6_linux.go`, the telemetry registration, AND `listener.go`. Phase 1 wired the listener to the pacer and stopped there, so the listener still swallows its error with no log line and no counter, which is the silent half this spec exists to remove. It is the easiest one to forget precisely because its pacing already works
   - Verify: the two silent loops now log, and every loop's swallowed error is counted
5. **Phase: the client poll and the allocation guard**
   - Tests: `TestWaitForPADOBlocksRatherThanSpins`, `TestReadLoopAllocationsPerPacketUnchanged`
   - Files: `pppoeclient/dialer.go`, `listener_test.go`
   - Verify: the discovery timeout still ends the wait at the same moment, and the listener's per-packet allocation count is unchanged
6. **Phase: proof under a real failure** -- the QEMU scenario
   - Tests: `subscriber-reader-failing-socket`
   - Files: `test/qemu/`
   - Verify: revert the pacer call in one loop, watch the CPU assertion go red, restore it, confirm green, and record that red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All four loops call the pacer, and all four publish a log line and a counter |
| Feature completeness | The failure is visible without a profiler: log, counter, and a documented diagnostic |
| Correctness | The stop path is never delayed, and the first retry after a success is immediate |
| Naming | The counter names the loop and the condition, not the function that detected it |
| Data flow | One pacer implementation, consulted by four loops. No per-loop variant and no second copy |
| Rule: `ai/rules/simplicity.md` | The pacer is a counter, a ceiling and an interruptible wait. No policy interface, no configurable strategy, no per-error classification table |
| Rule: `ai/rules/principles.md` | The pacer's zero delay means "retry now", which is a real value rather than a failure standing in for one, and a swallowed error is never silent |
| Rule: `ai/rules/goroutine-lifecycle.md` | Every wait observes the goroutine's existing exit signal, so no goroutine outlives its owner |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One pacer, four callers | `gopls references` on the pacer's wait shows exactly the four loops |
| No fifth backoff copy | The Key Design Decisions table records the three existing implementations and the verdict on each |
| Stop is never delayed | `TestListenerReadLoopStopsPromptlyWhilePacing` passes |
| Allocation discipline held | `TestReadLoopAllocationsPerPacketUnchanged` passes |
| The QEMU scenario discriminates | The recorded red from the reverted pacer call |
| The tier placement is legal | `./le tier check` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | The defect is itself a CPU exhaustion; the fix must not replace it with unbounded memory in the pacer, which holds one counter and one timer per goroutine |
| Fail-closed guard | The pacer is not a guard and must not become one: it never suppresses an error, it only paces the retry |
| Denial of service | Confirm no attacker-reachable path can drive a loop into the error branch, which would turn the delay into a way to slow discovery deliberately |
| Error leakage | The new log lines name the socket and the error, not peer-supplied bytes |
| Availability under stop | A daemon must still stop promptly while every loop is pacing at the ceiling |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Four loops written at different times reached the same wrong default, which is what a missing shared helper looks like from the outside. The repository already held three private backoff implementations, none of them reachable from a component.
- The two silent loops are the expensive half. A logged busy loop is found by reading a log; a silent one is found by profiling a production router.
- The import direction between `l2tp` and `l2tp/ppp` decides where the fix can live, and it was already fixed before this spec started. That is a constraint to read, not a decision to take.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One pacer in a core leaf | A private helper in each of the three packages | `ppp` cannot import `l2tp`, so no component-level home reaches all four call sites, and three copies is the shape that produced this defect |
| Pace the retry rather than exit on error | Return from the loop on any unclassified error, as the adopted-tunnel-socket loop already does | Exiting kills the listener for a transient error, which turns a CPU problem into an outage. The tunnel socket may exit because a dead tunnel socket never recovers, and that reasoning does not carry to a listener |
| A fixed internal ceiling | A configurable ceiling as a YANG leaf | An operator has no information with which to choose it, and a wrong value reintroduces the spin. The value is stated in the design page instead |
| Count and log every swallowed error | Log only, and rely on the log | A log line under a persistent failure is itself a flood. A counter is the surface an operator watches, and the log is rate-limited by the pacer that now spaces the retries |

## Known Limitations
- The pacer bounds retries on a failing socket. It does not diagnose the failure, and it does not recover a socket that will never work again: an operator still has to act on the counter.
- `readFrames` in `session_run.go` is left alone. Its read blocks, so it cannot spin, and bringing it into this work would widen the change for no defect.
- The ceiling is one value for all four loops. If one socket later needs a different pace, that is a change to this design, and the reason will be a measurement rather than a preference.

## RFC Documentation (Scope: protocol)

No RFC requirement changes here: the pacer acts only on a failing socket, and no
conformant peer exchange reaches the error path. Where a comment describes a
timer, it states that the interval is an internal retry pace and not a
protocol-visible timer, so a later reader does not mistake it for one.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
