# Spec: forward-barrier

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - reactor and forward pool design
4. `internal/component/bgp/reactor/forward_pool.go` - forward pool implementation
5. `internal/component/bgp/plugins/cmd/peer/peer.go` - peer command handlers
6. `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` - peer YANG schema
7. `internal/exabgp/bridge/bridge.go` - bridge runtime, goroutine structure
8. `internal/exabgp/bridge/bridge_command.go` - ExaBGP to ze command translation

## Task

Replace fragile `time.sleep()` calls in functional tests with a deterministic `peer <selector> flush` barrier command that blocks until all queued forward pool items have been written to peer sockets. Also make the ExaBGP bridge transparent barrier inject `peer <addr> flush` after every route command.

Functional tests use Python plugin scripts that send BGP UPDATE commands via `send()`. The `send()` RPC returns when the command is "dispatched" to the reactor, but the reactor forwards updates asynchronously via a forward pool (per-peer worker goroutines with buffered channels). There is no feedback to the plugin when bytes actually hit the wire. Tests use `time.sleep(0.2)` to `time.sleep(0.5)` between sends, which is fragile under parallel test load.

**Naming decision:** "flush" means "drain the write buffer to the wire" (like `fflush()` / `fsync()`), NOT "remove all entries" (like `ip route flush`). This is a deliberate choice to match buffer-flush semantics. Documented here to prevent misinterpretation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - forward pool architecture, reactor event loop
  → Decision: Forward pool uses per-destination-peer workers with buffered channels and overflow buffers
  → Constraint: Workers are long-lived goroutines (one per destination peer), exit on idle timeout
- [ ] `docs/architecture/api/commands.md` - existing command syntax patterns
  → Constraint: Target-first syntax: `peer <selector> <verb>`

- [ ] `internal/exabgp/bridge/bridge.go` - bridge runtime: goroutine structure, I/O wiring between ze and ExaBGP plugin
  → Constraint: Two goroutines (pluginToZebgp, zebgpToPluginWithScanner) share no state except Bridge.mu and Bridge.running. Flush coordination needs a new shared channel.
- [ ] `internal/exabgp/bridge/bridge_command.go` - ExaBGP to ze command translation, peer address extraction
  → Decision: Route commands (announce/withdraw) return translated command with peer address. Bridge can detect these and extract the peer address for targeted flush.

### RFC Summaries (MUST for protocol work)
- Not applicable (internal testing infrastructure, no RFC involvement)

**Key insights:**
- Forward pool has three queues to drain: `w.ch` (buffered channel), `w.pending` (atomic counter for in-flight dispatch), `w.overflow` (unbounded overflow buffer)
- The `fwdBatchHandler` executes under `session.writeMu` lock, meaning bytes are on the wire when the handler returns
- Peer command handlers follow a consistent pattern: `pluginserver.RegisterRPCs` in `init()`, handler function, YANG schema in `schema/ze-peer-cmd.yang`
- The `ze_api.py` already has a `flush()` function but it currently just delegates to `send()` -- needs repurposing

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_pool.go` - fwdPool with per-peer workers, Dispatch/TryDispatch/DispatchOverflow, drain batch pattern, idle timeout
  → Constraint: fwdWorker has `ch` (buffered channel), `pending` (atomic int), `overflow` (mutex-protected slice). All three must be empty for barrier to pass.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate dispatches to forward pool via TryDispatch/DispatchOverflow
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - peer command handlers (list, detail, teardown, add, remove, pause, resume, save)
  → Constraint: Uses `pluginserver.RegisterRPCs` with `WireMethod` for registration and `RequiresSelector: true` for commands that operate on specific peers
- [ ] `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` - YANG schema for peer commands using `ze:command` extension
- [ ] `internal/component/plugin/types.go` - ReactorPeerController interface (PausePeer, ResumePeer, etc.)
- [ ] `test/scripts/ze_api.py` - Python API library with `send()`, `wait_for_ack()`, `flush()`, and convenience functions
  → Constraint: `wait_for_ack(expected_count, timeout)` currently sleeps for `0.2 * max(1, expected_count)` seconds. Already present at every call site that needs barrier. Change implementation to send `peer * flush` RPC instead of sleeping.
  → Constraint: `flush()` method currently strips newline and calls `send()`. Unrelated to barrier -- leave as-is or remove if unused.
- [ ] `test/plugin/ipv4.ci` - typical test pattern: send messages with `time.sleep(0.2)` between them
- [ ] `test/plugin/ipv6.ci` - uses `time.sleep(0.5)`, was already bumped from 0.2s due to flakiness

**Behavior to preserve:**
- All existing `.ci` test expectations (hex, JSON output) remain unchanged
- Forward pool Dispatch/TryDispatch/DispatchOverflow semantics unchanged
- Existing `peer * pause`/`resume` command patterns preserved
- Plugin 5-stage registration protocol unchanged
- The `send()` RPC semantics unchanged (returns on dispatch, not on wire delivery)

**Behavior to change:**
- `wait_for_ack()` in `ze_api.py` changed from sleep-based delay to sending `ze-bgp:peer-flush` RPC (blocks until forward pool drained). The `expected_count` parameter becomes unused.
- Inter-message `time.sleep()` calls in listed `.ci` tests removed (FIFO ordering in forward pool guarantees message order; `wait_for_ack()` at end of send loop guarantees delivery)
- New `peer-flush` RPC added to reactor API

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Points
- **YANG RPC path (ze-native plugins):** Plugin script calls `wait_for_ack()`, which sends `ze-bgp:peer-flush` RPC
- **Text protocol path (ExaBGP bridge):** Bridge writes `peer <addr> flush` as text command after translating a route command

### Transformation Path (YANG RPC -- ze-native plugins)
1. Python `wait_for_ack()` calls `_call_engine('ze-bgp:peer-flush', params)` (synchronous RPC)
2. Engine receives RPC on plugin server, dispatches to `handleBgpPeerFlush` handler
3. Handler calls `ctx.Reactor().FlushForwardPool(ctx.Context())` (or filtered variant)
4. `reactorAPIAdapter.FlushForwardPool()` calls `fp.Barrier(ctx, filter)`
5. `fwdPool.Barrier()` dispatches sentinel `fwdItem` (done callback only, no data) to each targeted worker's channel
6. Workers process sentinels in FIFO order -- reaching the sentinel means all prior items are on the wire. Sentinel's `done` callback signals completion.
7. `Barrier()` waits for all sentinel callbacks, returns nil
8. RPC response propagates back to plugin, `wait_for_ack()` returns

### Transformation Path (Text protocol -- ExaBGP bridge)
1. Bridge reads ExaBGP route command from plugin stdout
2. Bridge translates to ze command, writes to ze stdout
3. Bridge writes `peer <addr> flush` to ze stdout
4. Bridge blocks on `flushDone` channel
5. Engine reads flush command from text protocol, dispatches to `handleBgpPeerFlush`
6. Handler runs barrier (same steps 3-6 as YANG path)
7. Handler detects text-protocol plugin, injects `{"type":"flush-done"}` synthetic event on plugin stdin pipe
8. Bridge event goroutine reads synthetic event, signals `flushDone`, suppresses event
9. Bridge command goroutine unblocks, reads next ExaBGP plugin line

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine (YANG RPC) | `ze-bgp:peer-flush` RPC over MuxConn wire protocol | [ ] |
| Plugin ↔ Engine (text protocol) | `peer <addr> flush` text command + `{"type":"flush-done"}` synthetic event response | [ ] |
| Engine ↔ Reactor | `ReactorPeerController.FlushForwardPool()` method call | [ ] |
| Reactor ↔ Forward Pool | `fwdPool.Barrier(ctx, filter)` method call | [ ] |

### Integration Points
- `fwdPool` (forward_pool.go) - new `Barrier()` method on existing type
- `reactorAPIAdapter` (reactor_api.go) - new `FlushForwardPool()` method implementing interface
- `ReactorPeerController` (plugin/types.go) - new method in existing interface
- `handleBgpPeerFlush` (plugins/cmd/peer/peer.go) - new handler following existing pattern
- `ze-peer-cmd.yang` - new `flush` container following `pause`/`resume` pattern
- `ze_api.py` - repurpose `flush()` from "write message" to "barrier RPC"
- Plugin server dispatch (server/) - synthetic `flush-done` event injection for text-protocol plugins
- ExaBGP bridge (exabgp/bridge/) - flush injection after route commands, `flushDone` channel coordination, suppress synthetic event

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (barrier is pure synchronization, no data copying)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Python `flush()` in plugin script | -> | `fwdPool.Barrier()` via `handleBgpPeerFlush` RPC | `test/plugin/ipv4.ci` (sends updates, calls flush(), verifies wire output) |
| Python `flush()` in plugin script | -> | `fwdPool.Barrier()` via `handleBgpPeerFlush` RPC | `test/plugin/ipv6.ci` (sends updates, calls flush(), verifies wire output) |
| ExaBGP bridge transparent flush | -> | `fwdPool.Barrier()` via bridge-injected `peer <addr> flush` | `test/exabgp/` (ExaBGP plugin sends route, bridge injects flush, verifies delivery) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `peer * flush` command when forward pool has queued items | Command blocks until all workers have drained channels, pending counters, and overflow buffers, then returns success |
| AC-2 | `peer <addr> flush` command with specific peer address | Command blocks until only that peer's worker is drained, returns success |
| AC-3 | `peer * flush` when no workers exist or no workers match filter | Command returns immediately with success |
| AC-4 | `peer * flush` with context cancellation / deadline exceeded | Command returns context error before pool is fully drained |
| AC-5 | `test/plugin/ipv6.ci` with `flush()` replacing `time.sleep(0.5)` | Test passes reliably under parallel load |
| AC-6 | All modified `.ci` tests with `flush()` replacing inter-message `time.sleep()` | All tests pass with `make ze-functional-test` |
| AC-7 | ExaBGP bridge translates a route command followed by transparent `peer <addr> flush` | Bridge blocks until flush response before reading next plugin line. ExaBGP plugin is unaware. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFwdPool_Barrier_DrainsAll` | `internal/component/bgp/reactor/forward_pool_barrier_test.go` | Barrier waits until channel and overflow are empty | |
| `TestFwdPool_Barrier_NoWorkers` | `internal/component/bgp/reactor/forward_pool_barrier_test.go` | Barrier returns immediately when no workers exist | |
| `TestFwdPool_Barrier_Filtered` | `internal/component/bgp/reactor/forward_pool_barrier_test.go` | Barrier with filter only waits for matching workers | |
| `TestFwdPool_Barrier_ContextCancel` | `internal/component/bgp/reactor/forward_pool_barrier_test.go` | Barrier returns context error on cancellation | |
| `TestFwdPool_Barrier_StoppedPool` | `internal/component/bgp/reactor/forward_pool_barrier_test.go` | Barrier returns immediately or error on stopped pool | |
| `TestFwdPool_Barrier_WithOverflow` | `internal/component/bgp/reactor/forward_pool_barrier_test.go` | Barrier waits for overflow buffer to drain, not just channel | |
| `TestBridge_FlushAfterRoute` | `internal/exabgp/bridge/bridge_test.go` | Bridge injects `peer <addr> flush` after translated route command and blocks until response | |
| `TestBridge_FlushNotForwarded` | `internal/exabgp/bridge/bridge_test.go` | Flush response from ze is consumed by bridge, not forwarded to ExaBGP plugin | |
| `TestBridge_NonRouteNoFlush` | `internal/exabgp/bridge/bridge_test.go` | Non-route commands (passthrough) do not trigger flush injection | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no numeric inputs | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipv4.ci` | `test/plugin/ipv4.ci` | Plugin sends IPv4 updates, flush() ensures delivery before withdraw | |
| `ipv6.ci` | `test/plugin/ipv6.ci` | Plugin sends IPv6 updates, flush() ensures delivery (was flaky with sleep) | |
| `nexthop.ci` | `test/plugin/nexthop.ci` | Plugin sends next-hop updates, flush() ensures delivery | |
| `mup4.ci` | `test/plugin/mup4.ci` | Plugin sends MUP IPv4 updates, flush() ensures delivery | |
| `mup6.ci` | `test/plugin/mup6.ci` | Plugin sends MUP IPv6 updates, flush() ensures delivery | |
| `eor.ci` | `test/plugin/eor.ci` | Plugin sends EOR markers, flush() ensures delivery | |
| `flowspec.ci` | `test/plugin/flowspec.ci` | Plugin sends flowspec updates, flush() ensures delivery | |
| `add-remove.ci` | `test/plugin/add-remove.ci` | Plugin sends updates and removes peer, flush() ensures delivery | |
| `watchdog.ci` | `test/plugin/watchdog.ci` | Plugin sends watchdog updates, flush() ensures delivery | |
| `custom-flowspec-plugin.ci` | `test/plugin/custom-flowspec-plugin.ci` | Custom flowspec plugin, flush() ensures delivery | |
| `explicit-plugin-config.ci` | `test/plugin/explicit-plugin-config.ci` | Explicit plugin config, flush() ensures delivery | |
| `registration.ci` | `test/plugin/registration.ci` | Registration sequence, flush() replaces inter-message sleeps | |
| `handoff-no-declare.ci` | `test/plugin/handoff-no-declare.ci` | Handoff without declaration, flush() replaces sleeps | |
| `handoff-listen.ci` | `test/plugin/handoff-listen.ci` | Handoff listen, flush() replaces sleeps | |
| `decode-mp-reach.ci` | `test/plugin/decode-mp-reach.ci` | MP_REACH decode, flush() replaces sleeps | |
| `decode-mp-unreach.ci` | `test/plugin/decode-mp-unreach.ci` | MP_UNREACH decode, flush() replaces sleeps | |
| `decode-update.ci` | `test/plugin/decode-update.ci` | UPDATE decode, flush() replaces sleeps | |
| `fast.ci` | `test/plugin/fast.ci` | Fast send pattern, flush() replaces sleep | |
| `peer-selector-name-and-ip.ci` | `test/plugin/peer-selector-name-and-ip.ci` | Peer selector test, flush() replaces sleep | |

### Future (if deferring any tests)
- None -- all tests listed above are existing tests being modified, not new tests

## Files to Modify
- `internal/exabgp/bridge/bridge.go` - add `flushDone` channel to Bridge struct, coordinate between goroutines for transparent flush
- `internal/exabgp/bridge/bridge_command.go` - detect route commands (announce/withdraw) and extract peer address for targeted flush
- `internal/component/bgp/reactor/forward_pool.go` - export any shared types if needed for barrier (fwdKey fields, worker access)
- `internal/component/bgp/reactor/reactor_api.go` - add `FlushForwardPool()` method to `reactorAPIAdapter`
- `internal/component/plugin/types.go` - add `FlushForwardPool(ctx context.Context) error` to `ReactorPeerController` interface
- `internal/component/bgp/plugins/cmd/peer/peer.go` - add `handleBgpPeerFlush` handler and register `ze-bgp:peer-flush` RPC
- `internal/component/plugin/server/dispatch.go` or `server.go` - for text-protocol plugins, inject `{"type":"flush-done"}` synthetic event on plugin event channel after flush handler completes (exact file TBD during implementation, depends on where text-protocol dispatch lives)
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` - add `flush` container with `ze:command`
- `internal/component/bgp/schema/ze-bgp-api.yang` - add `peer-flush` RPC definition
- `test/scripts/ze_api.py` - change `wait_for_ack()` implementation from sleep to `ze-bgp:peer-flush` RPC
- `internal/component/plugin/server/mock_reactor_test.go` - add `FlushForwardPool` stub to mock
- `test/plugin/ipv4.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/ipv6.ci` - remove inter-message `time.sleep(0.5)`
- `test/plugin/nexthop.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/mup4.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/mup6.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/eor.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/flowspec.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/add-remove.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/watchdog.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/custom-flowspec-plugin.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/explicit-plugin-config.ci` - remove inter-message `time.sleep(0.2)`
- `test/plugin/registration.ci` - remove inter-message `time.sleep(0.1)` and `time.sleep(0.2)`
- `test/plugin/handoff-no-declare.ci` - remove inter-message `time.sleep(0.1)` and `time.sleep(0.2)`
- `test/plugin/handoff-listen.ci` - remove inter-message `time.sleep(0.1)` and `time.sleep(0.2)`
- `test/plugin/decode-mp-reach.ci` - remove inter-message `time.sleep(0.1)` and `time.sleep(0.2)`
- `test/plugin/decode-mp-unreach.ci` - remove inter-message `time.sleep(0.1)` and `time.sleep(0.2)`
- `test/plugin/decode-update.ci` - remove inter-message `time.sleep(0.1)` and `time.sleep(0.2)`
- `test/plugin/fast.ci` - remove inter-message `time.sleep(0.1)`
- `test/plugin/peer-selector-name-and-ip.ci` - remove inter-message `time.sleep(0.1)`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [Yes] | `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang`, `internal/component/bgp/schema/ze-bgp-api.yang` |
| CLI commands/flags | [No] | N/A (operational RPC only, not a CLI subcommand) |
| Editor autocomplete | [Yes] | YANG-driven (automatic once YANG updated) |
| Functional test for new RPC/API | [Yes] | All listed `.ci` tests serve as functional tests for the flush command |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [Yes] | `docs/architecture/api/commands.md` - operational RPC available to plugins (covered by item 4) |
| 2 | Config syntax changed? | [No] | - |
| 3 | CLI command added/changed? | [No] | - |
| 4 | API/RPC added/changed? | [Yes] | `docs/architecture/api/commands.md` - add `peer <selector> flush` to Peer commands table |
| 5 | Plugin added/changed? | [No] | - |
| 6 | Has a user guide page? | [No] | - |
| 7 | Wire format changed? | [No] | - |
| 8 | Plugin SDK/protocol changed? | [No] | - |
| 9 | RFC behavior implemented? | [No] | - |
| 10 | Test infrastructure changed? | [Yes] | `docs/functional-tests.md` - document `flush()` as replacement for `time.sleep()` in send loops |
| 11 | Affects daemon comparison? | [No] | - |
| 12 | Internal architecture changed? | [No] | - |

## Files to Create
- `internal/component/bgp/reactor/forward_pool_barrier.go` - `Barrier()` method on `fwdPool` and `BarrierPeer()` variant
- `internal/component/bgp/reactor/forward_pool_barrier_test.go` - unit tests for barrier

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Forward pool barrier** -- implement `Barrier()` on `fwdPool`
   - Tests: `TestFwdPool_Barrier_DrainsAll`, `TestFwdPool_Barrier_NoWorkers`, `TestFwdPool_Barrier_Filtered`, `TestFwdPool_Barrier_ContextCancel`, `TestFwdPool_Barrier_StoppedPool`, `TestFwdPool_Barrier_WithOverflow`
   - Files: `forward_pool_barrier.go`, `forward_pool_barrier_test.go`, `forward_pool.go` (if type exports needed)
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Interface and reactor wiring** -- add `FlushForwardPool` to `ReactorPeerController` interface and implement on `reactorAPIAdapter`
   - Tests: compilation of existing tests (interface satisfaction)
   - Files: `plugin/types.go`, `reactor_api.go`, `mock_reactor_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Command handler and YANG** -- register `peer-flush` RPC handler, add YANG schema entry
   - Tests: `TestYANGCommandTree` (existing test should detect new command)
   - Files: `plugins/cmd/peer/peer.go`, `plugins/cmd/peer/schema/ze-peer-cmd.yang`, `schema/ze-bgp-api.yang`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: ExaBGP bridge transparent barrier** -- BLOCKED on bridge MuxConn fix

   **Finding (implementation research):** The bridge's runtime I/O is non-functional with MuxConn. After the 5-stage protocol, ze wraps the plugin connection in MuxConn (`#<id> method json` wire format). The bridge continues with raw text I/O. Both directions fail silently:
   - Commands: bridge writes raw text to stdout, MuxConn drops lines without `#` prefix
   - Events: ze sends MuxConn-formatted events, bridge fails to parse `#<id>` prefix as JSON

   The bridge needs to speak MuxConn wire format before flush (or any command dispatch) can work. This is a separate spec: `spec-exabgp-bridge-muxconn.md`.

   ~~Tests: `internal/exabgp/bridge/bridge_test.go` (existing tests must still pass), new test for flush injection~~
   ~~Files: `bridge.go` (flushDone channel, goroutine coordination), `bridge_command.go` (detect route commands, extract peer address)~~
   ~~Verify: bridge correctly injects flush after route commands, blocks until response, does not forward flush response to ExaBGP plugin~~

5. **Phase: Python API and test migration** -- change `wait_for_ack()` implementation, remove inter-message sleeps from `.ci` tests
   - Tests: all 20 listed `.ci` functional tests
   - Files: `test/scripts/ze_api.py` (`wait_for_ack` implementation), all `.ci` files listed in Files to Modify (remove `time.sleep` between sends)
   - Verify: functional tests pass with `wait_for_ack()` sending flush RPC and inter-message sleeps removed

6. **Phase: Documentation** -- update API command docs and functional test docs
   - Files: `docs/architecture/api/commands.md`, `docs/functional-tests.md`
   - Verify: docs reflect new command

7. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)

8. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-forward-barrier.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Barrier correctly waits for all three queue types (ch, pending, overflow). Polling interval reasonable (1ms). No busy-wait without sleep. |
| Naming | YANG uses `flush` (matches existing verb pattern). Wire method uses `ze-bgp:peer-flush` (matches `ze-bgp:peer-pause` pattern). |
| Data flow | Plugin wait_for_ack() -> RPC -> handler -> reactor adapter -> fwdPool.Barrier() -> return |
| Rule: no-layering | Old `wait_for_ack()` sleep implementation fully replaced with flush RPC, not layered |
| Rule: goroutine-lifecycle | Barrier uses polling or condition variable, NOT per-call goroutine |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `forward_pool_barrier.go` exists | `ls internal/component/bgp/reactor/forward_pool_barrier.go` |
| `forward_pool_barrier_test.go` exists | `ls internal/component/bgp/reactor/forward_pool_barrier_test.go` |
| `FlushForwardPool` in ReactorPeerController | `grep FlushForwardPool internal/component/plugin/types.go` |
| `handleBgpPeerFlush` registered | `grep peer-flush internal/component/bgp/plugins/cmd/peer/peer.go` |
| YANG `flush` container | `grep flush internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` |
| `ze-bgp-api.yang` RPC | `grep peer-flush internal/component/bgp/schema/ze-bgp-api.yang` |
| `ze_api.py` `wait_for_ack()` sends flush RPC | `grep peer-flush test/scripts/ze_api.py` |
| No inter-message `time.sleep` in modified .ci files | `grep -l 'time.sleep' test/plugin/ipv4.ci` returns empty (or only non-forward sleeps) |
| `docs/architecture/api/commands.md` updated | `grep flush docs/architecture/api/commands.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Peer selector validated by existing `filterPeersBySelector()` -- no new validation needed |
| Resource exhaustion | Barrier uses context with timeout -- cannot block forever. Polling interval bounded. |
| Denial of service | Flush is an operational command requiring plugin authentication -- same as pause/resume |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Details

### ExaBGP Bridge Transparent Barrier

The ExaBGP bridge (`internal/exabgp/bridge/`) translates ExaBGP plugin commands to ze commands. ExaBGP plugins are third-party code that cannot be modified. The bridge must transparently ensure route delivery without the ExaBGP plugin knowing.

**Mechanism:** After the bridge translates a route command (announce or withdraw) and writes it to ze, it also writes `peer <addr> flush` using the same peer address from the translated command. It then waits for the flush response before reading the next line from the ExaBGP plugin's stdout.

**Text protocol has no RPC responses.** The bridge uses the text protocol (raw lines on stdin/stdout), not YANG RPC with MuxConn. After writing a text command to ze, the bridge gets no command ack -- ze only sends JSON events on the plugin's stdin. The flush response must arrive as a synthetic event.

**Solution:** When the engine processes a `peer <addr> flush` command from a text-protocol plugin, the flush handler (after the barrier completes) injects a synthetic JSON event `{"type":"flush-done"}` on the plugin's event channel (stdin pipe). The bridge's event goroutine recognizes this event, signals the command goroutine, and suppresses it (does NOT forward to the ExaBGP plugin). This reuses the existing event delivery path with no protocol changes.

**I/O coordination:** The bridge runs two goroutines: `pluginToZebgp` (reads plugin stdout, writes commands to ze) and `zebgpToPluginWithScanner` (reads events from ze, writes to plugin stdin). A shared `flushDone` channel coordinates:

| Step | Goroutine | Action |
|------|-----------|--------|
| 1 | `pluginToZebgp` | Translates ExaBGP command, writes ze command to stdout |
| 2 | `pluginToZebgp` | Writes `peer <addr> flush` to stdout |
| 3 | `pluginToZebgp` | Blocks on `flushDone` channel |
| 4 | `zebgpToPluginWithScanner` | Reads `{"type":"flush-done"}` synthetic event from ze stdin |
| 5 | `zebgpToPluginWithScanner` | Signals `flushDone` channel (suppresses event from reaching ExaBGP plugin) |
| 6 | `pluginToZebgp` | Unblocks, reads next ExaBGP plugin line |

**Backpressure:** The ExaBGP plugin writes to its stdout pipe. When the bridge slows down (waiting for flush), the OS pipe buffer fills and the plugin's `write()` syscall blocks naturally. The plugin experiences standard Unix backpressure without any protocol-level awareness.

**Per-peer flush:** The bridge uses `peer <addr> flush` (not `peer * flush`) because the translated command already targets a specific peer. This avoids blocking on unrelated peers.

### Barrier Implementation Strategy: Sentinel Pattern

The `Barrier()` method dispatches a sentinel `fwdItem` into each targeted worker's channel. The sentinel carries only a `done` callback (no rawBodies, no updates). Workers process items in FIFO order, so reaching the sentinel guarantees all prior items have been written to the socket. The sentinel's `done` callback signals a per-worker done channel. `Barrier()` waits for all done channels (with context cancellation).

This is better than polling because:
- Deterministic: FIFO ordering is a structural guarantee, not a timing heuristic
- Zero overhead on the hot path: no condition variables, no atomic checks added to worker loop
- No busy-wait: `Barrier()` blocks on channels, not sleep loops
- Handles overflow correctly: if items are in the overflow buffer, the sentinel enters the channel after the worker has drained overflow (workers drain overflow after each batch)

The existing `fwdItem.done` callback mechanism (used for cache release) is reused for sentinel signaling. No new fields or synchronization primitives needed.

### Sentinel Dispatch Rules

| Situation | Sentinel goes via | Why |
|-----------|-------------------|-----|
| Worker channel has space | `TryDispatch` (non-blocking) | Normal path |
| Worker channel full | `DispatchOverflow` (overflow buffer) | Sentinel queues behind existing items, which is correct |
| No worker for peer | Return immediately (AC-3) | No items to drain |
| Pool stopped | Return error or nil | Pool is shutting down |

### Python API: wait_for_ack() becomes the barrier

`wait_for_ack()` already exists at every call site that needs the barrier. Its semantic ("wait until my routes are delivered") is exactly what the barrier provides. Change only the implementation, not the call sites.

| Aspect | Before | After |
|--------|--------|-------|
| Implementation | `time.sleep(0.2 * max(1, expected_count))` | Send `ze-bgp:peer-flush` RPC (blocks until pool drained) |
| `expected_count` param | Scales sleep duration | Unused (flush drains everything). Keep param for backwards compat, ignore value. |
| `timeout` param | Unused (kept for API compat) | Pass as context deadline to flush RPC |
| Return value | Always `True` | `True` on success, may raise on timeout |

The existing `flush()` method (strips newline, calls `send()`) is unrelated to the barrier. Leave it as-is.

### Sleep Replacement Rules

Only remove `time.sleep()` calls that exist to wait for forward delivery between `send()` calls. Do NOT remove sleeps that serve other purposes:
| Sleep purpose | Action |
|---------------|--------|
| Wait between `send()` calls for forward delivery | Remove -- `wait_for_ack()` at end of loop handles delivery |
| `wait_for_ack()` after send loop | Keep call, change implementation to send flush RPC |
| Wait for session establishment | Keep |
| Wait for RPKI cache | Keep |
| Wait for event propagation to plugin | Keep |
| Wait for graceful restart timers | Keep |
| General initialization delay | Keep |

Inter-message sleeps are unnecessary because: (1) the forward pool's per-peer worker processes items in FIFO order, so ordering is guaranteed, and (2) `wait_for_ack()` at the end of the send loop now blocks until everything is on the wire.

### Pipeline Ordering Invariant (BLOCKING constraint)

The barrier's correctness depends on a sequential guarantee: `send()` returns only AFTER `ForwardUpdate()` has dispatched items to the forward pool. This means by the time `flush()` arrives at the barrier, all preceding send items are in the pool.

| Step | Guarantee | What breaks if violated |
|------|-----------|------------------------|
| `send()` blocks until RPC response | RPC response means ForwardUpdate completed | If `send()` returned before ForwardUpdate, flush could see an empty pool and return early |
| Commands processed in FIFO order | Flush handler runs after preceding update handler | If commands reordered, flush could run before the update reaches the pool |

If anyone later changes `send()` to return earlier (e.g., after command dispatch but before ForwardUpdate), the barrier breaks silently. This invariant must be preserved.

### Pre-Session Flush Behavior

If `flush()` is called for a peer whose BGP session is not yet established, the forward pool has no worker for that peer (workers are created on first forward). AC-3 applies: barrier returns immediately with success.

This is technically incorrect -- routes may be cached waiting for the session. But it is acceptable because:
- In tests, sessions are always established before plugins send routes
- In the ExaBGP bridge, the bridge starts after the 5-stage plugin handshake, which happens after ze is running and sessions are establishing
- A cached route will be forwarded when the session comes up; the next flush after session establishment will catch it

This is a documented limitation, not a bug.

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

Not applicable -- this is internal testing infrastructure, not a protocol feature.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- **Phase 4 blocked:** ExaBGP bridge transparent barrier blocked on bridge MuxConn incompatibility. Bridge's runtime I/O is non-functional after stage 5 (both commands and events silently dropped). Created `spec-exabgp-bridge-muxconn.md` as prerequisite. AC-7 not achievable until bridge I/O is fixed.
- **.ci file sleep removal reverted:** Removing inter-message `time.sleep()` broke 5 functional tests. The sleeps ensure ordering between plugin commands and ze-peer API commands (`cmd=api` lines). Only `wait_for_ack()` implementation was changed; inter-message sleeps remain.

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
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
- [ ] Write learned summary to `plan/learned/NNN-forward-barrier.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
