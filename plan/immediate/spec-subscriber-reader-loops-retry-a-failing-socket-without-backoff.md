# Spec: subscriber-reader-loops-retry-a-failing-socket-without-backoff

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-09-11 |

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
| `subscriber-reader-failing-socket` | `test/draft/l2tp/subscriber-reader-failing-socket.ci`, TRACKED by a `.gitignore` exception and read by NO gate (KNOWN VACUOUS, see the file's header and Known Limitations) | A subscriber socket is put into a persistently failing state and the daemon's CPU use stays low while the log and the counter show the failure | vacuous under ptrace, deliberately not gated; the mechanism that would gate it is named in the file |

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
- `test/qemu/` - never created; does not exist and no reachable scenario needed it. The failing-socket scenario landed at `test/draft/l2tp/subscriber-reader-failing-socket.ci` instead, KNOWN VACUOUS (Phase 6), plus its fixture at `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go` and `internal/test/fixture/register_l2tp_failing_socket.go`
- The three files are TRACKED and no gate reads them. `test/draft/` is the one directory whose property is that every recursive `.ci` reader skips it, and the scenario is tracked there by an explicit `.gitignore` exception, the same route `test/draft/plugin/gr-vacuity-*.ci` already takes. A tracked `.ci` under `test/l2tp/` would instead be an accidental orphan, since `netnsSelections` (`internal/le/qemu/netns_linux.go`) is an explicit name list. Each of the three files states in its own header that no gate reads it, why, and what would have to change for it to be gated

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
6. **Phase: proof under a real failure** -- the QEMU scenario. NOT DONE. `test/qemu/`
   does not exist (confirmed by an earlier phase); the scenario was written at
   `test/draft/l2tp/subscriber-reader-failing-socket.ci` against the L2TP
   listener, actually run under a real Linux kernel via `./le qemu run` on
   this session's own darwin host (qemu-system-aarch64 with HVF is installed
   here), and PROVEN VACUOUS: the only mechanism that can sustain a
   repeating (non-blocking) read error on any of the four loops is
   ptrace-based syscall fault injection (`strace -e inject`), and ptrace's
   own signal-trapping overhead (Go's runtime sends itself a constant SIGURG
   stream for goroutine preemption, and every delivery is a ptrace stop
   regardless of `--seccomp-bpf`) throttles the traced daemon's CPU use and
   iteration rate to the same order of magnitude as the pacer's own 250ms
   ceiling. The pre-fix (pacer-reverted) build and the fixed build were
   indistinguishable under this mechanism in a captured run: 19 read errors
   and 0.0 measurable CPU delta either way, against 36640 errors and 0.99
   CPU-seconds when the SAME reverted build was driven by hand outside the
   .ci harness in under 4 seconds. Moved to `test/draft/` so it cannot pass
   vacuously in a gating run. No CPU/counter threshold fixes this: ptrace's
   overhead dominates the exact range the test needs to discriminate.
   - Tests: `subscriber-reader-failing-socket` (draft, not gated)
   - Files: `test/draft/l2tp/subscriber-reader-failing-socket.ci`,
     `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go`,
     `internal/test/fixture/register_l2tp_failing_socket.go`
   - Outcome (owner decision, 2026-09-11): AC-1 resolves on the unit
     coverage and the spec CLOSES. The scenario, its fixture and its
     registration are COMMITTED, tracked, and read by no gate, at
     `test/draft/l2tp/`, by the same `.gitignore` exception route the
     `gr-vacuity-*.ci` exhibit takes. Each of the three files states in its
     own header that nothing gates it, why, and what would gate it.
   - Verify: the discrimination walk WAS run (this is the finding above),
     but it discriminates against the TEST, not for it: reverting the pacer
     does not turn this scenario red. AC-1's CPU claim for all four loops
     stays proven only by the unit tests from Phases 1-5
     (`TestListenerReadLoopPacesAFailingSocket` and its three siblings),
     which drive the same code through a synthetic socket error rather than
     a real kernel failure, and were not open to the ptrace confound because
     nothing external traces them. A working end-to-end proof needs a
     failure-injection point that does not route through ptrace -- a kernel
     fault-injection framework (docs.kernel.org/fault-injection) against the
     socket receive path is the candidate named in the .ci file's own
     comment; LD_PRELOAD does not apply, since CGO_ENABLED=0 makes Ze's
     binaries call the kernel directly with no libc to interpose on.

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
- **No end-to-end CPU assertion exists for AC-1, and none can be built on ptrace.** AC-1 is
  resolved on the unit coverage (owner decision, 2026-09-11). The four
  `Test*PacesAFailingSocket` tests drive each loop's real error branch through a real
  socket held in a persistently failing state, and each has an observed red, two of them
  on a real Linux kernel in the QEMU guest (335,243 swallowed errors in 200ms with the
  pacing removed, against fewer than 50 with it). What no test in this spec does is assert
  the DAEMON's own CPU use, as an operator's monitoring would read it, while a real kernel
  socket fails.
  The reason is that no mechanism exists to hold a real socket in that state from outside
  the daemon. The only one found, ptrace-based syscall fault injection, cannot provide the
  proof: Go's runtime emits a continuous SIGURG stream for goroutine preemption, ptrace
  traps every delivery regardless of `--seccomp-bpf`, and ptrace-stopped time counts as
  wall clock rather than as the tracee's CPU, so the tracer throttles the traced daemon
  into the same range the pacer's own ceiling produces. The pre-fix build then passes every
  assertion. No threshold fixes it, because the measuring instrument dominates over exactly
  the range the two builds would differ in.
  The scenario, its fixture and the measured numbers are kept at
  `test/draft/l2tp/subscriber-reader-failing-socket.ci`, tracked and read by no gate, with
  the finding in its own header. Phase 6 (Implementation Steps) carries the full writeup
  and the candidate that was not pursued: a kernel fault-injection framework against the
  socket receive path.
- **The client's discovery poll is bounded by the discovery timeout, not by the pacer.**
  `waitForPADS` treats a read error as "no frame yet" (`tryReadPADS` returns `(0, nil)` on
  `rxErr`), so a socket that fails immediately rather than blocking still spins that loop
  for up to `discoveryTimeout`. AC-9 asks only that the wait block on the socket timeout
  rather than yield, and that is what changed. Bounding the client's error path the way the
  four receiver loops are bounded is a separate change to a loop that already cannot run
  forever.

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

---

## Implementation Summary

### What Was Implemented
- `internal/core/pacer` -- `Pacer`, a value type holding one delay and one reused
  timer. `Wait(stop <-chan struct{}) bool` paces one retry and returns true when
  the caller's own exit signal fired; `Succeed()` resets. Growth 0, 10, 20, 40,
  80, 160ms, then pinned at a 250ms ceiling.
- All four receiver loops call it on the error branch they used to `continue`
  from: `(*UDPListener).readLoop` (`listener.go`), `discoveryReader`
  (`pppoe/subsystem.go`), `rsReaderLoop` (`ppp/ra_linux.go`) and
  `dhcpv6ServerLoop` (`ppp/dhcpv6_linux.go`). Each passes the exit signal it
  already owns, so no loop waits out a delay while it is stopping.
- `discoveryReader` had no exit signal beyond its socket closing, so `Subsystem`
  gained a real `stop` channel, made in `Start` and closed in `Stop`.
- All four log their swallowed error, two of them for the first time, and all
  four count it: `ze_l2tp_listener_read_errors_total` (`l2tp/reader_metrics.go`),
  `ze_pppoe_discovery_read_errors_total` (`pppoe/metrics.go`) and
  `ze_ppp_reader_errors_total{loop="ra"|"dhcpv6"}` (`ppp/reader_metrics_linux.go`).
  Three counters rather than one because `ppp` can import neither `pppoe` nor
  `l2tp`. Each binds through `registry.InjectPluginMetrics`.
- `pppoeclient/dialer.go` lost its `runtime.Gosched()` from both discovery waits:
  `SO_RCVTIMEO` already blocks that read for about 100ms.
- Phase 6 (this closure): the failing-socket scenario, its fixture and its
  registration are committed, tracked, and read by no gate. See Known
  Limitations and the Review Gate below.

### Bugs Found/Fixed
- A vacuous first draft of `TestReaderLoopsCountSwallowedErrors`: the collector
  registers the series at 0 as soon as it binds, so a bare non-empty check passed
  with no error ever counted. The test now asserts the literal `... 0` line
  first, then requires it to change. Found by the phase-4 agent in its own draft.
- `loopRA`, `loopDHCPv6` and `countReaderError` were declared in the
  cross-platform `ppp/metrics.go` while both call sites are `//go:build linux`,
  so they were dead on every non-Linux lint flavor. Moved to
  `ppp/reader_metrics_linux.go`, mirroring the package's existing
  `kernel_linux.go`/`socket_other.go` split.
- `tunnel_fixture_l2tp_failing_socket.go` carried a comment claiming a check the
  code does not make ("the counter must already be moving"). The call's real job
  is to order the CPU baseline after the series exists. Comment corrected in this
  closure; found by the Review Gate, round 1.

### Documentation Updates
- `docs/research/l2tpv2-ze-integration.md` section 11.5 -- what all four loops do
  with an error they cannot classify. The page was silent on this before.
- `docs/architecture/l2tp/bng-5-pppoe.md` -- the discovery reader's pacing and its
  counter, citing the page's existing "One AF_PACKET raw socket per namespace".
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` -- the client's discovery wait
  blocks rather than polls.
- `docs/architecture/core-design.md` -- `internal/core/pacer/` in the core leaf table.
- `docs/guide/l2tp.md` and `docs/guide/pppoe.md` -- the three counters as
  operator diagnostics.
- `test/draft/README.md` (this closure) -- the two tracked exceptions, what each
  is, why no gate reads it, and what would gate it.
- `./le doc check verify`: see Pre-Commit Verification.

### Deviations from Plan
- The spec's Files to Create named `test/qemu/`. It does not exist and no
  reachable scenario needed it. The scenario landed under `test/draft/l2tp/`.
- The spec's Phase 4 file list omitted `listener.go`, and the phase-4 brief
  corrected it: the listener's log line and counter were Phase 4's work, since
  Phase 1 wired only the pacer.
- The spec described `discoveryReader` as context-driven. It is not: it took no
  context and its only exit signal was its socket closing. It gained a stop
  channel rather than a context.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spec assumed all three non-listener loops were context-driven | `discoveryReader` had no context and no stop channel; a bare port of the ctx.Done() pattern would have left a stopping subsystem sitting out 250ms | Phase 4 read `subsystem.go` before wiring it | `Subsystem.stop` added, closed by `Stop` alongside the socket |
| approach | Phase 6 built the end-to-end scenario on ptrace fault injection, made it work mechanically, and only then measured that it cannot discriminate | ptrace's own signal-trap overhead throttles the traced daemon into the pacer's range | The discrimination walk: the pre-fix build passed every assertion | The scenario is kept as a finding, not as a test; the instrument is measured BEFORE the test is trusted |
| escalation | Three Linux-only tests were written, cross-compiled and never executed, on a host where QEMU boots | `./le qemu run` runs Linux test binaries here; only the guest-only `pppoe-test` verb refuses on darwin | This closure ran them | All three now pass on kernel 7.2 in the guest, and two carry an observed red there |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| No receiver goroutine on the subscriber path can consume a core indefinitely on a failing socket | Done | `pacer.Wait` called from all four error branches | Four `Test*PacesAFailingSocket` tests, two proven on a real kernel |
| Every error a loop swallows is visible to an operator | Done | three counters plus a log line per loop | `TestReaderLoopsCountSwallowedErrors` scrapes the rendered `/metrics` text |
| The client's bounded relative is bounded properly | Done | `waitForPADO`, `waitForPADS` (`dialer.go`) | `TestWaitForPADOBlocksRatherThanSpins`; the error path stays bounded by the discovery timeout (Known Limitations) |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestListenerReadLoopPacesAFailingSocket`, `TestDiscoveryReaderPacesAFailingSocket`, `TestRSReaderLoopPacesAFailingSocket`, `TestDHCPv6ServerLoopPacesAFailingSocket` | Resolved on the unit coverage by owner decision, 2026-09-11. No end-to-end CPU assertion exists; Known Limitations says why none can be built on ptrace |
| AC-2 | Done | `TestPacerFirstRetryIsImmediate` | The first failure a Pacer meets returns a zero delay |
| AC-3 | Done | `TestPacerResetsAfterSuccess` | `Succeed` puts the next failure back at zero |
| AC-4 | Done | `TestPacerWaitReturnsOnStop`, `TestListenerReadLoopStopsPromptlyWhilePacing` | Stop returns in under 150ms while the pacer waits at the 250ms ceiling |
| AC-5 | Done | `TestPacerWaitReturnsOnStop`, `TestRSReaderLoopStopsPromptlyWhilePacing`, `TestDiscoveryReaderStopsPromptlyWhilePacing` | The RA one runs on a real kernel in the guest |
| AC-6 | Done | `listener.go`, `subsystem.go`, `ra_linux.go`, `dhcpv6_linux.go` error branches | The two formerly silent loops log; the `.ci` asserts the listener's line on stderr |
| AC-7 | Done | `TestReaderLoopsCountSwallowedErrors` | Asserts the literal zero line first, then the change |
| AC-8 | Done | `TestReadLoopAllocationsPerPacketUnchanged` | 3 allocations per packet, the pre-existing count, measured with `Succeed` removed |
| AC-9 | Done | `TestWaitForPADOBlocksRatherThanSpins` | Returns within one blocking read of the stop signal |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPacerFirstRetryIsImmediate` | Done | `internal/core/pacer/pacer_test.go` | |
| `TestPacerGrowsToCeiling` | Done | same | Drives `next()` so no real time is waited |
| `TestPacerResetsAfterSuccess` | Done | same | |
| `TestPacerWaitReturnsOnStop` | Done | same | |
| `TestPacerWaitReusesTheTimer` | Done | same | Extra; proves the allocation constraint AC-8 rests on |
| `TestListenerReadLoopPacesAFailingSocket` | Done | `internal/component/l2tp/listener_test.go` | Real socket, deadline in the past |
| `TestListenerReadLoopStopsPromptlyWhilePacing` | Done | same | |
| `TestDiscoveryReaderPacesAFailingSocket` | Done | `internal/component/l2tp/pppoe/discovery_reader_test.go` | Red was a HANG, the busy loop itself |
| `TestRSReaderLoopPacesAFailingSocket` | Done | `internal/component/l2tp/ppp/ra_linux_test.go` | Green and red both observed in the QEMU guest, this closure |
| `TestDHCPv6ServerLoopPacesAFailingSocket` | Done | `internal/component/l2tp/ppp/dhcpv6_linux_test.go` | Same |
| `TestReaderLoopsCountSwallowedErrors` | Done | `internal/component/l2tp/listener_test.go` | |
| `TestReadLoopAllocationsPerPacketUnchanged` | Done | same | |
| `TestWaitForPADOBlocksRatherThanSpins` | Done | `internal/component/l2tp/pppoeclient/dialer_test.go` | |
| `subscriber-reader-failing-socket` | Changed | `test/draft/l2tp/` | Written, run, and PROVEN VACUOUS. Tracked, gated by nothing, header carries the finding |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/pacer/pacer.go` | Done | The "new core leaf" the spec named without naming |
| `internal/component/l2tp/listener.go` | Done | |
| `internal/component/l2tp/pppoe/subsystem.go` | Done | Plus the `stop` channel the spec did not foresee |
| `internal/component/l2tp/ppp/ra_linux.go` | Done | |
| `internal/component/l2tp/ppp/dhcpv6_linux.go` | Done | |
| `internal/component/l2tp/pppoeclient/dialer.go` | Done | |
| `internal/component/l2tp/reader_metrics.go` | Done | Not in the plan; the listener's counter needed its own hook |
| `internal/component/l2tp/ppp/reader_metrics_linux.go` | Done | Not in the plan; the lint gate forced the linux-only split |
| `test/qemu/` | Changed | Never created. The scenario lives at `test/draft/l2tp/` |
| the four doc pages | Done | Listed under Documentation Updates |

### Audit Summary
- **Total items:** 9 ACs, 14 tests, 10 files
- **Done:** 9 ACs, 13 tests, 8 files
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 test (the `.ci`, vacuous and not gated), 2 files (`test/qemu/` never created, two metrics files added), all recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| No receiver goroutine on the subscriber path can consume a core indefinitely on a failing socket | functional (unit, against a real failing socket) | With the pacing removed, `dhcpv6ServerLoop` recorded `ze_ppp_reader_errors_total{loop="dhcpv6"} 335243` in a 200ms window and `rsReaderLoop` signalled in 4.296ms, both on Linux kernel 7.2 in the QEMU guest. With it restored, both pass their bounds. Logs: `job-ppp-qemu-red-fa4f5994.log` and `job-ppp-qemu-17a92a06.log`. The listener and the discovery reader carry the same pair on darwin |
| Every error a loop swallows is visible to an operator | functional (rendered `/metrics`) | `TestReaderLoopsCountSwallowedErrors` renders the registry through the same promhttp handler an exporter serves, requires `ze_l2tp_listener_read_errors_total 0` before any read, and then requires that line to change |
| A daemon still stops promptly while every loop is pacing at the ceiling | functional | `TestListenerReadLoopStopsPromptlyWhilePacing` (Stop under 150ms against a 250ms ceiling), `TestRSReaderLoopStopsPromptlyWhilePacing` (guest), `TestDiscoveryReaderStopsPromptlyWhilePacing` |
| The fix costs the packet path nothing | benchmark | `TestReadLoopAllocationsPerPacketUnchanged` asserts 3 allocations per packet, the count measured with the pacer's only added line on that path commented out |
| The daemon's own CPU use stays low under a real kernel socket failure | NOT PROVEN | No test asserts this. The only mechanism that sustains such a failure is ptrace, and it destroys the measurement. Owner decision 2026-09-11: AC-1 resolves on the unit coverage above. Known Limitations carries the finding and `test/draft/l2tp/subscriber-reader-failing-socket.ci` carries the numbers |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The end-to-end assertion that the daemon's own CPU stays low under a real failing socket | No mechanism exists that fails a real socket without tracing the daemon, and tracing destroys the measurement. Owner decision 2026-09-11 removed it from this spec's scope | `plan/spec-failing-socket-proof-needs-a-non-ptrace-injection-point.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/subscriber-reader-loops-retry-a-failing-socket-without-backoff-8a072de8-ce8e-47de-bdd2-015f5b44ca51.md` |
| `review check` | `review_gate: OK (3 code files, clean, hashes match)` |
| Rounds | 2 |
| Reviewer lenses used | Round 1: (A) correctness, concurrency, lifecycle and guards, reading each producer; (B) style against `docs/contributing/ze-go-style.md`, simplicity, security and documentation. Round 2: the fixes round 1 made, plus their siblings |

**Round 1 scope (declared before it ran):** the whole diff, meaning commit
`300a7541a` plus every uncommitted phase-6 file, read at the producer.
**Round 2 scope (declared before it ran):** the four files round 1's fixes
touched (`.gitignore`, `test/draft/README.md`, the `.ci` header,
`tunnel_fixture_l2tp_failing_socket.go`, `register_l2tp_failing_socket.go`) and
the checks that read them (`TestDraftDirIsGitignored`,
`TestDraftReadmeNamesEveryCheck`, `TestDraftDirIsInvisibleToRepoChecks`).

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Unwired code: the `.ci` was gitignored and both fixture files untracked, so the registered `l2tp/failing-socket` personality had no caller the tree holds, and nothing said why | `test/draft/l2tp/subscriber-reader-failing-socket.ci`, `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go`, `register_l2tp_failing_socket.go` | The `.ci` is tracked by an explicit `.gitignore` exception, the route `test/draft/plugin/gr-vacuity-*.ci` already takes, and all three headers now state that no gate reads them, why, and what would gate them |
| 2 | ISSUE | A comment claimed a check the code does not make: "the counter must already be moving before the window starts", over a call that only waits for the series to exist | `tunnelL2TPFailingSocket` (`internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go`) | The comment now names the call's real job, which is to order the CPU baseline after the series exists, and points at the delta assertion that answers the movement question |
| 3 | ISSUE | Two Linux-only tests had never been executed, so AC-1 for `rsReaderLoop` and `dhcpv6ServerLoop` rested on tests nobody had run and no red had been observed for | `ppp/ra_linux_test.go`, `ppp/dhcpv6_linux_test.go` | Run in the QEMU guest on kernel 7.2: three pass, and the discrimination walk with the pacing removed produced 335,243 swallowed errors in 200ms. Files restored byte-identical afterwards |
| 4 | ISSUE | `test/draft/README.md` said the gr-vacuity exhibit was "the one tracked exception", which finding 1's fix makes false | `test/draft/README.md` | Rewritten as a table of the two exceptions, each with its reason and what would gate it, plus the bar a third would have to clear |

### Notes recorded, not blocking
- `Pacer.Wait`'s `if !p.timer.Stop() { <-p.timer.C }` drain is dead code under Go
  1.23+ timer semantics, and `go.mod` declares `go 1.27.0`. Probed directly:
  `Stop()` returns true after expiry and discards the pending value, and 20,000
  races of a closing stop channel against timer expiry never blocked. Harmless,
  left alone.
- `UDPListener.pacer` and `Subsystem.pacer` keep their grown delay across a
  Stop/Start cycle, so the first error after a restart can wait up to the ceiling
  rather than retrying at once. No AC covers a restart.
- `"error", err.Error()` in three loops beside `"error", err` in the fourth. No
  behavioral difference.
- `var readDiscoveryFrame = pppoe.ReadDiscoveryFrame` (`pppoeclient/dialer.go`)
  is a mutable package-level function variable that production code calls
  through, and it is the only one of that shape under `internal/component`, where
  every other package-level `var x = pkg.Y` is an error value or a logger. It is
  restored by `t.Cleanup` and no test in the package runs in parallel.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/pacer/pacer.go` | Yes | in `300a7541a`; `go test ./internal/core/pacer/...` reports `ok` |
| `internal/component/l2tp/reader_metrics.go` | Yes | in `300a7541a` |
| `internal/component/l2tp/ppp/reader_metrics_linux.go` | Yes | in `300a7541a` |
| `test/draft/l2tp/subscriber-reader-failing-socket.ci` | Yes | `git check-ignore -v` names the negation line, and `git status` lists it as untracked rather than ignored |
| `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go` | Yes | compiles into `ze-test`; the fixture name list printed by the runner includes `l2tp/failing-socket` |
| `internal/test/fixture/register_l2tp_failing_socket.go` | Yes | same list is how the name reaches the runner |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Each of the four loops paces a persistently failing socket | `go test -race -count=1` over `./internal/core/pacer/... ./internal/component/l2tp/...`: every package `ok` (log `job-unit-pacer-l2tp-89026932.log`). The two Linux-only ones ran in the QEMU guest: `--- PASS: TestDHCPv6ServerLoopPacesAFailingSocket`, `--- PASS: TestRSReaderLoopPacesAFailingSocket` |
| AC-1 | The tests would go red without the fix | With the two `p.Wait(ctx.Done())` blocks removed, the same guest run: `ze_ppp_reader_errors_total{loop="dhcpv6"} 335243 ... want fewer than 50 in 200ms` and `rsCh signaled in 4.296ms: rsReaderLoop is not pacing its retries` |
| AC-4, AC-5 | The stop path never waits out the delay | `--- PASS: TestRSReaderLoopStopsPromptlyWhilePacing` in the guest; the listener and pppoe siblings pass in the darwin run above |
| AC-7 | A counter names the loop and rises | `internal/component/l2tp` is `ok` in the run above, which includes `TestReaderLoopsCountSwallowedErrors` |
| AC-8 | Per-packet allocations unchanged | same run includes `TestReadLoopAllocationsPerPacketUnchanged`, asserting exactly 3 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A persistently failing L2TP UDP socket | `test/draft/l2tp/subscriber-reader-failing-socket.ci` | Read in full. It drives the real daemon and the real counter, and it is VACUOUS: the pre-fix build passes it. Tracked, gated by nothing, header says so. The gating proof is the unit test, named in the Wiring Test table |
| The other three loops | none | Each is wired to `pacer.Wait` at its own error branch, proven by the four unit tests and by the observed reds |
| The `l2tp/failing-socket` personality | the same `.ci`, plus `ze-test fixture l2tp/failing-socket <port>` by hand | The name resolves in the runner's registry: it appears in the runner's own "use one of" list |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | A read error that is neither closed nor context-done does persist: a deadline set into the past reproduces it indefinitely on a real socket, and every `Test*PacesAFailingSocket` drives exactly that |
| A-2 | confirmed | `internal/component/l2tp` imports `l2tp/ppp` and `l2tp/pppoe`, neither imports back, and `./le tier check` reports the core import direction clean. The pacer lives in `internal/core/` for that reason |
| A-3 | confirmed | One shared `AF_PACKET` socket serves every access interface, dispatching by ifindex from `recvfrom` (`pppoe/kernel_linux.go`, `readDiscoveryFrame`). A delay there is a delay for all of them, which is why the ceiling is 250ms rather than seconds |
| A-4 | confirmed | Recorded in the Assumptions table with all three producers read: none of `ddos/flowspec/probe.go`, `flowexport/conntrack_worker.go` or `exabgp/bridgerun/respawn.go` is a reusable interruptible pacer |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/research/l2tpv2-ze-integration.md` 11.5 says what all four loops do with an unclassified error | Matches the four error branches as committed, including the correction that `discoveryReader` is not context-driven | Yes |
| `docs/guide/l2tp.md`, `docs/guide/pppoe.md` name the counters | The three metric names match `reader_metrics.go`, `pppoe/metrics.go` and `ppp/metrics.go` exactly | Yes |
| `docs/architecture/core-design.md` lists `internal/core/pacer/` | The package exists and `./le tier check` accepts its placement | Yes |
| `test/draft/README.md` names both tracked exceptions | The `.gitignore` carries both negation blocks; `TestDraftDirIsGitignored` and `TestDraftReadmeNamesEveryCheck` still pass in the run above | Yes |
| Categories answered No | No CLI verb, no YANG leaf, no RPC, no wire change, no plugin, no RFC row: the diff adds no command, no schema file and no protocol-visible timer | Yes |

## Core Insight

The measurement was never the problem; the stimulus was, and the only instrument
that could provide it was also what destroyed the reading. A test whose
instrument shares an order of magnitude with the effect under test cannot
discriminate, and no threshold rescues it. Measure the instrument against a
deliberately broken build BEFORE trusting a green.
