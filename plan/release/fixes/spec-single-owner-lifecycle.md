# Spec: single-owner-lifecycle

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | A crashed plugin that cannot re-register, a write from a dead BGP session landing on its successor, and an unauthenticated datagram killing an IKE SA are shipped-daemon defects an operator meets in production. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Family S3 of the journal backlog analysis (2026-10-23), part 1 of 3. The other
two parts depend on the ownership check this spec builds:
`plan/release/fixes/spec-single-owner-reload-reconcile.md` and
`plan/release/fixes/spec-typed-identity-keys.md`.

## Task

**Root cause.** A lifecycle in Ze (a plugin process, a BGP session, an LDP or
L2TP session, an IKE SA) has no single owner. Registration, teardown and state
changes are performed by whichever function happens to hold the object, so:

1. two teardown paths clean different subsets of the same registrations;
2. a goroutine started for one session generation keeps writing after that
   generation ended, and its writes land on the successor;
3. a teardown resets state before the code that must read it has read it;
4. two processes or two functions decide the same transition, and the order
   they happen to run in decides the outcome;
5. any handler may write `sa.State`, including on a message that was never
   authenticated.

Each journal row below is one instance. Fixing them one by one leaves the
mechanism in place, and the journal shows it recurring across plugin server,
reactor, LDP, L2TP, BMP, RIB and IKE.

**Goal.** Each lifecycle has one owner object. The owner hands out a
generation, records every registration it makes, is the only writer of the
lifecycle's state, and runs one teardown that releases what it recorded. A
check refuses a state write or a registration made outside the owner, so the
family cannot return.

**Why it matters for shipping.** Every row is a production behavior: a plugin
that crashed once is refused on respawn, a daemon stop always waits out a
deadlock, routes from a dead session reach the new one, an LDP or BGP backoff
never resets, and a forged packet tears down an IKE SA mid-handshake.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and the plugin, BGP and IKE doors in `ai/doors/README.md`
  -> Constraint: any plugin may run as an external process, so the owner must
  release registrations by process identity, never by an in-process call.
- [ ] `docs/architecture/core-design.md` (plugin startup stages, process manager)
- [ ] `docs/architecture/api/process-protocol.md` (plugin process stages and teardown)
- [ ] `docs/architecture/ldp/mpls-ldp.md` (LDP session lifecycle)
- [ ] `docs/architecture/plugin/rib-storage-design.md` (RIB retention and release)
- [ ] `docs/research/l2tpv2-ze-integration.md` (L2TP session events)
- [ ] `ai/rules/goroutine-lifecycle.md`
  -> Constraint: long-lived workers on channels; every goroutine has a named
  owner and a stop path.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7296.txt` Section 2.4 and Section 2.21.1
- [ ] `rfc/full/rfc4724.txt` Section 4.2 (route retention on restart)

**Key insights:**
- Plugin process teardown has two entry points today; the crash path is the
  weaker one.
- BGP already clears per-session flags at teardown, but nothing stops the old
  goroutine writing after the clear.

## Current Behavior (MANDATORY)

Verified at the producer on 2026-10-23.

**Source files read:**
- [ ] `internal/component/plugin/server/dispatch.go` - `cleanupProcess`, the crash and exit teardown
- [ ] `internal/component/plugin/server/restart.go` - `releasePluginRegistrations`, the reload and removal teardown
- [ ] `internal/component/bgp/reactor/peer_run.go` - `runOnce` teardown clear, `run` backoff reset
- [ ] `internal/component/bgp/reactor/update_delay.go` - the two `go p.sendInitialRoutes()` sites
- [ ] `internal/plugins/ldp/register.go` - `runSession` defer order, `onDone` closure
- [ ] `internal/component/ike/engine/fsm.go` - `handleAuthResponse`, `handleEAPResponse`
- [ ] `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP`

| Producer | What it does today | Rows it explains |
|----------|--------------------|------------------|
| `(*Server).cleanupProcess` (`internal/component/plugin/server/dispatch.go`) | The crash and exit path. Unregisters dispatcher commands, pending requests, subscriptions, cache consumer, plugin routes. Never calls `s.registry.Unregister`, `RemovePluginCapabilities` or `removePluginFamilies` | rollback-forgets-partial-apply 2026-08-21 |
| `(*Server).releasePluginRegistrations` (`internal/component/plugin/server/restart.go`) | The reload, removal and restart path. Calls `cleanupProcess` and then the three registries above. Its own comment states that `PluginRegistry.Register` refuses a name it already holds | same row: the two paths disagree on what a plugin owns |
| `(*Peer).sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`), started twice by `go p.sendInitialRoutes()` in `update_delay.go` | Per-session goroutine nobody joins. The teardown clear in `runOnce` (`peer_run.go`) resets `sendingInitialRoutes` and `initialSyncEOROwed`, and its comment says the old goroutine may still be inside its bounded waits. No generation is carried, so a write after the clear reaches the next session | late-write-lands-on-the-successor 2026-08-31 |
| `(*Peer).run` (`internal/component/bgp/reactor/peer_run.go`) | Resets `delay` and `prefixTeardownCount` only in the `err == nil` branch, which `safeRunOnce` returns only on stop. A session ended by NOTIFICATION or hold expiry never resets either | state-read-after-teardown-reset-it 2026-10-11 |
| `runSession` (`internal/plugins/ldp/register.go`) | Defers `onDone` and then `sess.Stop`, so `Stop` runs first and sets `StateNonExistent` (`(*Session).Stop`, `session.go`). The `onDone` closure in `startSessionForAdj` then reads `sess.State()` and always records a setup failure | state-read-after-teardown-reset-it 2026-10-02 |
| `handleAuthResponse` and `handleEAPResponse` (`internal/component/ike/engine/fsm.go`), `handleResponderEAP` (`responder_eap.go`) | Set `sa.State = StateDead` when the SK payload fails to decrypt. `sa.State` is assigned at more than 80 sites across nine non-test files of the engine, with no single transition function | unprotected-message-changes-sa-state 2026-08-14, 2026-09-29 |

RFC 7296 Section 2.4: "If IKE presumes the partner is dead, based on repeated
lack of acknowledgement to an IKE message, then the IKE SA and all Child SAs set
up through that IKE SA are deleted." Death is declared on missing
acknowledgement, not on one undecryptable datagram.

RFC 7296 Section 2.4: "since the first two messages of an SA setup are not
cryptographically protected, an attacker could respond to the initiator's
message before the genuine responder and poison the connection setup attempt."

RFC 7296 Section 2.21.1: "Because all error notifications are completely
unauthenticated, the recipient should continue trying for some time before
giving up."

A datagram whose integrity check fails proves nothing about its sender, so it
must not drive a transition.

**Behavior to preserve:** plugin five-stage startup order; BGP initial sync
semantics (EOR after initial routes); LDP backoff schedule for failed setups;
IKE retransmission and the cached-response replay.

**Behavior to change:** every row in "Owned journal rows".

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Plugin process spawn, exit and crash (process manager); BGP session
  establish and teardown (reactor FSM); LDP and L2TP session up and down;
  IKE datagrams on UDP 500 and 4500.

### Transformation Path
1. The owner is created when the lifecycle starts and issues generation N.
2. Every registration (registry row, capability, family, subscription,
   goroutine, timer) is made through the owner, which records its release.
3. Every asynchronous writer carries generation N; the owner refuses a write
   whose generation is not current.
4. Teardown is one owner method: it bumps the generation, runs the recorded
   releases in reverse order, and only then publishes the terminal state.
5. Readers of terminal facts (was the session operational, did it reach
   Established) read what the owner recorded before teardown, never live state
   teardown has reset.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> plugin process | registrations keyed by process identity | No |
| Reactor <-> per-session goroutines | generation token | No |
| IKE wire <-> SA state | one transition function | No |

### Integration Points
- `process.Process.RunRuntimeCleanupOnce`: becomes the owner's teardown.
- `bgp/reactor` session struct: carries the generation.
- `ike/engine` SA: state field written only by the transition function.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | the two plugin teardown paths become one |
| Zero-copy preserved where applicable | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | the ownership check reads an annotation at the field, not a central list |

## The fix

**One owner per lifecycle.**

| Lifecycle | Owner | Replaces |
|-----------|-------|----------|
| Plugin process | one teardown method on the server, reached from crash, exit, reload, removal and restart alike | `cleanupProcess` and `releasePluginRegistrations` as two paths |
| BGP session | a per-session generation held by the peer; `sendInitialRoutes`, forward overflow and backoff resets check it | the two atomic flags cleared at teardown |
| LDP session | `runSession` hands the terminal facts (reached operational) to `onDone` | `onDone` reading `sess.State()` after `Stop` |
| L2TP session | events for a session are emitted under the order the tunnel lock gives, and a consumer drops events for a generation that ended | emit after unlock |
| Adj-RIB-In early decision | a buffered decision is bound to the generation pending when it was stored | msgID 0 matching any generation |
| BMP monitored peer | peer-up for a collector is followed by a replay of that peer's Adj-RIB, owned by the sender session | live updates as the only Route Monitoring producer |
| IKE SA | one transition function, the only writer of `sa.State`; a message that fails integrity is dropped and counted, never a transition | more than 80 direct assignments |
| GR route retention | `bgp-rib` defers the Adj-RIB-In release until the GR decision for that peer-down is known, so one owner decides | two plugins racing on one event |
| Plugin startup and stop | barriers wait on named dependencies the owner declares; stop releases the plugin API before waiting on the reactor | the cycles in the barrier rows |

**The check that stops it coming back.** A repository check, run on every
commit over the Go files it carries, reads an ownership annotation placed on a
struct field or a struct type (`owner: <function>`). It refuses:

- an assignment to an annotated field outside the named function;
- a composite literal of an annotated type outside the named constructor
  (used by `spec-typed-identity-keys.md`);
- a `go` statement inside a package that declares a lifecycle owner whose
  started function takes neither the owner's generation nor its context.

The annotation sits on the declaration, so a new lifecycle registers itself and
no central list grows. The refusal names the field, the owner, and the command
that prints the owner's location.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| plugin process killed, respawned by the manager | -> | server teardown owner | `TestPluginRespawnAfterCrashRegistersAgain` (new, plugin/server) |
| BGP session torn down and re-established inside the initial-sync window | -> | per-session generation | `TestInitialSyncOfDeadSessionDoesNotReachSuccessor` (new, reactor) |
| IKE_AUTH response with a corrupted SK payload | -> | IKE transition function | `TestUndecryptableAuthResponseLeavesSAAlive` (new, ike/engine) plus a `.ci` over the daemon |
| commit carrying a write to an annotated field outside its owner | -> | ownership check | `TestOwnershipCheckRefusesForeignWrite` (new, internal/le/repo) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A commit adds an assignment to an owner-annotated field outside its owner | `./le commit create` refuses it, naming field, owner and fix |
| AC-2 | A commit adds a composite literal of an owner-annotated type outside its constructor | refused the same way |
| AC-3 | A plugin process is SIGKILLed and the manager respawns it | the respawn completes all five stages and serves its commands; no registry, capability or family row of the dead process remains |
| AC-4 | A daemon stop with plugins running | completes without waiting out a stage or stop timeout; the test logs the measured stop time |
| AC-5 | A BGP session tears down and re-establishes within 2s | no UPDATE or EOR produced by the first session's initial sync reaches the second session's wire |
| AC-6 | A BGP session that reached Established ends by NOTIFICATION or hold expiry | the reconnect delay and the prefix-teardown count are reset |
| AC-7 | An LDP session that reached operational ends | its backoff entry is cleared and SessionDown is emitted once; a session that never reached operational emits no SessionDown |
| AC-8 | An IKE_AUTH or EAP message (initiator and responder) fails SK decryption | the datagram is dropped and counted, the SA keeps its state, a valid retransmission then completes the exchange |
| AC-9 | Every `sa.State` write in `internal/component/ike/engine` | goes through the one transition function; the ownership check enforces it |
| AC-10 | Peer-down of a GR-capable peer | `bgp-rib` retains the Adj-RIB-In (RFC 4724 Section 4.2) in every run of a 20-run loop |
| AC-11 | A BMP collector connects after a monitored peer is up | it receives Peer Up followed by Route Monitoring for that peer's current Adj-RIB-In |
| AC-12 | Each remaining owned row | its reproduction (the test named in the row or in the TDD plan) is red before the fix and green after |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOwnershipCheckRefusesForeignWrite` | `internal/le/repo/ownership_test.go` | AC-1, AC-2 | |
| `TestPluginRespawnAfterCrashRegistersAgain` | `internal/component/plugin/server/restart_test.go` | AC-3 | |
| `TestStopWithPluginsWaitsOutNoTimeout` | `internal/component/plugin/process/manager_test.go` | AC-4 | |
| `TestInitialSyncOfDeadSessionDoesNotReachSuccessor` | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-5, run `-race -count=20` | |
| `TestEstablishedSessionEndResetsBackoff` | `internal/component/bgp/reactor/peer_run_test.go` | AC-6 | |
| `TestOperationalSessionEndClearsBackoff` | `internal/plugins/ldp/register_test.go` | AC-7 | |
| `TestUndecryptableAuthResponseLeavesSAAlive` | `internal/component/ike/engine/fsm_test.go` | AC-8 initiator | |
| `TestUndecryptableEAPRoundLeavesSAAlive` | `internal/component/ike/engine/responder_eap_test.go` | AC-8 responder | |
| `TestRIBRetainsOnGRPeerDown` | `internal/component/bgp/plugins/rib/rib_test.go` | AC-10, `-race -count=20` | |
| `TestCollectorAfterPeerUpGetsAdjRIB` | `internal/component/bgp/plugins/bmp/bmp_test.go` | AC-11 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| session generation | 0 to 2^64-1, wraps | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-crash-respawn` | `test/plugin/` | operator's plugin crashes and comes back serving commands (AC-3) | |
| `reload-dynamic-peer-survives` | `test/reload/` | passes 20 of 20 runs (barrier row 2026-09-08) | |
| `ike-auth-corrupt-response` | `test/ike/` | a corrupted IKE_AUTH response does not end the tunnel (AC-8) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ipsec-auth-corrupt-response-strongswan` | `test/interop-ipsec/scenarios/` | strongSwan | SA survives a corrupted IKE_AUTH response and completes | |
| `bgp-gr-retention-bird` | `test/interop/scenarios/` | BIRD 3 | Adj-RIB-In retained across the peer's restart | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the ownership check and its annotation
   - Tests: `TestOwnershipCheckRefusesForeignWrite`
   - Files: `internal/le/repo/ownership.go`, registered in the repository checks
   - Verify: the check refuses a planted foreign write and runs on commit
2. **Phase: plugin process owner** -- one teardown path, barrier dependencies, stop order
   - Tests: AC-3, AC-4 tests, `reload-dynamic-peer-survives`
   - Files: plugin/server, plugin/process
3. **Phase: session generations** -- BGP, LDP, L2TP, Adj-RIB-In, BMP replay, GR retention
   - Tests: AC-5, AC-6, AC-7, AC-10, AC-11
4. **Phase: IKE transition function** -- annotate `sa.State`, route every write through one function, drop on integrity failure
   - Tests: AC-8, AC-9, interop scenario
5. **Phase: remaining rows** -- each owned row's reproduction red then green (AC-12)

## Files to Modify
- `internal/component/plugin/server/dispatch.go`, `restart.go`, `startup.go`, `engine_events.go`
- `internal/component/plugin/process/manager.go`
- `internal/component/bgp/reactor/peer_initial_sync.go`, `peer_run.go`, `update_delay.go`, `reactor_peers.go`
- `internal/component/bgp/plugins/adj_rib_in/rib_validation.go`, `bgp/plugins/rib/rib.go`, `bgp/plugins/bmp/`
- `internal/plugins/ldp/register.go`
- `internal/component/l2tp/reactor_kernel.go`
- `internal/component/ike/engine/` (`fsm.go`, `responder_eap.go`, `responder.go` and the other state writers)
- `internal/plugins/ddos/flowspec/register.go`

## Files to Create
- `internal/le/repo/ownership.go` and its test

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| rollback-forgets-partial-apply | 2026-08-21 | `cleanupProcess` vs `rollbackStartupProcess`, `PluginRegistry.Register` |
| component-rebuilt-during-reload | 2026-09-20 | ddos flowspec `runEngine` exit leaves FlowSpec NLRI |
| stop-leaves-the-work-running | 2026-10-12 | rib `setupRemoteRIB` Sink on `context.Background()` |
| stop-leaves-the-work-running | 2026-09-17 | firewall-domain guest shutdown restarts plugins |
| plugin-startup-barrier-deadlock | 2026-08-10 | firewall vpp backend waits on vpp at capability barrier |
| plugin-startup-barrier-deadlock | 2026-08-19 | `ProcessManager.Stop` cycle with `Reactor.cleanup` |
| plugin-startup-barrier-deadlock | 2026-09-08 | five-stage handshake against inbound BGP connection |
| late-write-lands-on-the-successor | 2026-08-31 | `sendInitialRoutes` outlives teardown |
| late-write-lands-on-the-successor | 2026-09-09 | `engineEventSubscribers.dispatch` calls a retired handler |
| late-write-lands-on-the-successor | 2026-10-14 | l2tp SessionIPAssigned after SessionDown |
| late-write-lands-on-the-successor | 2026-10-23 | `applyEarlyDecision` msgID 0 matches any generation |
| announced-state-never-replayed | 2026-08-31 | bmp `primeSender`, `bounceMonitoredPeers` never replay Adj-RIB |
| unprotected-message-changes-sa-state | 2026-08-14 | ike responder EAP decrypt failure kills SA |
| unprotected-message-changes-sa-state | 2026-09-29 | ike initiator IKE_AUTH and EAP decrypt failure kills SA |
| state-read-after-teardown-reset-it | 2026-10-02 | ldp `startSessionForAdj` onDone reads state Stop reset |
| state-read-after-teardown-reset-it | 2026-10-11 | `Peer.run` backoff reset only on stop |
| unwired-feature | 2026-10-13 | `addPeerLocked` passive peer gets no listener in standalone start |
| unwired-feature | 2026-09-08 | rib `handleState` vs gr `retain-routes` race on peer-down |

18 rows.

## Rows found already fixed

None in this part. See the two sibling specs.

## Rows considered and not owned

| Class file | Date | Surface (short) | Why not this root cause |
|------------|------|-----------------|-------------------------|
| rollback-forgets-partial-apply | 2026-08-25 | `./le scratch migrate` | le tooling, retired script; tooling family |
| stop-leaves-the-work-running | 2026-10-20 (three rows), 2026-10-11 | dockerkernel install, `runDocker`, unrooted le verbs | le tooling signal handling, not a shipped lifecycle |
| reload-rolls-back-instead-of-applying | 2026-08-25 | traffic reload `context canceled` | mechanism not identified; a flake (T1) until reproduced |
| announced-state-never-replayed | 2026-09-06 | exabgp-compat `api-reload.1.conf` second static route | mechanism not identified by the row |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row before implementation starts
- [ ] RFC 7296 and RFC 4724 sentences re-read in `rfc/full/`
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-12 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass: `./le test unit` over the packages in Files to Modify, `./le test functional plugin`, `./le test functional reload`, and `./le verify current fast`
- [ ] Interop scenarios green, with the red phase recorded

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Owned rows re-pointed to this spec's closure
