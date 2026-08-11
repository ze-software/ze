# Spec: fixit-bgp-shutdown-cease-notification

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 9/9 |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the Known Limitations are untraced research, not postponed work. Create `plan/deferrals/fixit-bgp-shutdown-cease-notification.md` on the first deferral) |
| Updated | 2026-08-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-29 while implementing `plan/spec-rfcgate-2-evidence.md`. An agent
observed by experiment that a SIGTERM to a running `ze` with an Established peer
produces no BGP NOTIFICATION at all; a second agent then traced the mechanism
against the producing code. Both the observation and the trace are recorded below,
and the trace is what this spec is written from.

## Task

**Ze sends no Cease / Administrative Shutdown NOTIFICATION on SIGTERM, and the code
that would send one is unreachable.** `Session.Close()` builds exactly that message
(`internal/component/bgp/reactor/session_connection.go`), and it has **no
reachable production caller**. Two independent guards each make it dead:

| # | Guard | Why it is always false on the shutdown path |
|---|-------|---------------------------------------------|
| 1 | `if p.session != nil` (`internal/component/bgp/reactor/peer_run.go`) | `runOnce`'s deferred `p.session = nil` (`peer_run.go`) runs before `p.cleanup()`, which is itself `defer p.cleanup()` at `peer_run.go` and therefore can only run after `runOnce` returned. Same goroutine, strict program order: this is a guaranteed ordering, not a race |
| 2 | `if conn != nil` (`session_connection.go`) | The session's cancel goroutine calls `closeConn()` the instant the peer context is done (`session.go`), and `Run` also has `defer s.closeConn()` (`session.go`). `closeConn` sets `s.conn = nil` (`session_connection.go`) |

So even if guard 1 were passed, guard 2 would still skip the send. `logNotifyErr` is
never invoked, so nothing is logged either.

**Timing is not the cause and must not be "fixed".** `r.cleanup()` Phase 2 waits on
every `peer.Wait(waitCtx)` under a 2s deadline (`reactor.go`, `:1626-1630`) and
the hub gives `eng.Stop` a 3s budget (`cmd/ze/hub/main.go`). The peer
goroutine reaches `cleanup()` well before exit. Nothing is issued, so nothing is
left unflushed. A fix that only widens a deadline changes nothing.

**The requirement level is MAY, and that is load-bearing for how this is scoped.**
RFC 4271 §8.2.2's Established-state ManualStop action list says "sends the
NOTIFICATION message with a Cease" in **indicative mood with no RFC 2119 keyword**
(`rfc/full/rfc4271.txt:3940-3943`), despite the document carrying the 2119
boilerplate at `:302-304`. The only keyword governing the act is §6.7's **MAY**
(`rfc/full/rfc4271.txt:1919-1923`). RFC 4486 adds nothing stronger: both its
extracted requirements are MAY (`rfc/short/rfc4486.md`). This is therefore a
conformance-quality and operational defect, **not** a MUST violation, and it does not
engage the owner-escalation gate in `ai/rules/rfc-compliance.md`.

→ Decision (2026-08-10, during extraction): **`RFC4271-8.2.2-18` is written at
MUST, not at MAY. That needs no owner escalation because it RAISES what Ze owes
rather than lowering it** (`ai/rules/rfc-compliance.md`: ask only before doing
LESS). Two readings sit here and only one survives contact with the file. §6.7's
MAY governs a speaker CHOOSING, "at any given time", to close a connection with
Cease of its own accord. Once the operator has issued Event 2, §8.2.2's action
list is what the FSM owes, and §8 binds an implementation to "the same externally
visible behavior" even where the FSM description is conceptual. Every sibling row
in `rfc/short/rfc4271.md` derived from a §8.2.2 action list, `RFC4271-8.2.2-1`
through `-5`, already stands on exactly that ground, so MAY here would have been
the outlier. The requirement is implemented and carries both polarities, so the
level decides the ledger's wording, not Ze's behaviour.

**The operational consequence is why this is worth fixing anyway.** A peer observes a
clean TCP FIN, not an RST and not a NOTIFICATION (`session_connection.go`:
flush, `CloseWrite`, 100ms drain, `Close`). A bare TCP close with no NOTIFICATION is
the signal a graceful-restart-capable peer reads as *"my neighbour restarted, retain
its routes as stale"* rather than *"my neighbour went away, withdraw them"*. Ze
advertises GR. An administrative shutdown may therefore leave peers holding Ze's
routes until Restart-Time expires, which is the opposite of what an operator taking a
box out of service intends.

→ Constraint: the GR consequence above is **unverified**. It follows from what the
peer sees plus the fact that Ze advertises GR; nobody has traced Ze's own GR plugin
or tested a real peer's reaction (`ai/rules/evidence.md`). Validating it is the
first task of this spec, because it decides the priority: if peers do hold stale
routes through an admin shutdown, this is an operator-visible defect and the MAY
level understates it.

## Required Reading

- [ ] `rfc/short/rfc4271.md` and `rfc/full/rfc4271.txt:1919-1923`, `:3940-3943`
  → Constraint: §8.2.2's action list is indicative, §6.7's MAY is the only keyword.
    Do not write this up as a MUST violation; the extraction below is what is missing.
- [ ] `ai/rules/rfc-compliance.md` "Extraction Completeness"
  → Constraint: there is **no `RFC4271-8.2.2-*` requirement extracted at all**
    (`rfc/short/rfc4271.md` jumps from `RFC4271-8.2.1-3` to `8.2.1.4`; confirmed in
    `ai/RFC-REQUIREMENTS.md`). RFC 4271 IS enrolled (`rfc/enrolled.txt`),
    so this is an unextracted obligation inside an enrolled RFC: the gate's silence is
    not conformance.
- [ ] `internal/component/bgp/reactor/session_connection.go` (`Close`, `closeConn`, `Teardown`)
- [ ] `internal/component/bgp/reactor/peer_run.go` (`cleanup`, `runOnce`'s defers)
- [ ] `internal/component/bgp/reactor/reactor.go` (`Stop`, `cleanup` Phase 1 and 2)

## Current Behavior (MANDATORY)

**Source files read** (by the tracing agent, 2026-07-29; every hop cited):

- [ ] `internal/component/bgp/reactor/session_connection.go`
- [ ] `internal/component/bgp/reactor/peer_run.go`
- [ ] `internal/component/bgp/reactor/reactor.go`
- [ ] `internal/component/bgp/reactor/session.go`
- [ ] `internal/component/bgp/reactor/signal.go`
- [ ] `internal/component/bgp/reactor/peer.go`
- [ ] `internal/component/bgp/reactor/session_test.go`
- [ ] `test/reload/signal-stop.ci`


| Hop | What happens | Cite |
|-----|--------------|------|
| 1 | SIGTERM → `cb()` | `signal.go`, `:171` |
| 2 | callback is `r.Stop()` | `reactor.go` |
| 3 | `Stop()` cancels `r.ctx` | `reactor.go` |
| 4 | `monitor()` unblocks → `r.cleanup()` | `reactor.go` |
| 5 | `cleanup()` Phase 1 → `peer.Stop()` → `p.cancel()` | `reactor.go`, `peer.go` |
| 6 | `p.ctx` is a child of `r.ctx`, so step 3 already cancelled it | `peer.go`; started at `reactor.go`, `:1137`, `reactor_peers.go`, `reactor_dynamic.go` |
| 7 | session cancel goroutine → `s.closeConn()` | `session.go`, `:896-897` |
| 8 | `closeConn()` sets `s.conn = nil` | `session_connection.go` |
| 9 | `Session.Run` returns; `defer s.closeConn()` on every path | `session.go` |
| 10 | `runOnce`'s defer sets `p.session = nil` | `peer_run.go` |
| 11 | `runOnce` returns → `defer p.cleanup()` | `peer_run.go`, `:178`, `:517` |
| 12 | `cleanup()` tests `p.session != nil` → false → never sends | `peer_run.go` |

**No other graceful path fires on shutdown.** Every production `Teardown` caller is
BFD-down (`peer_bfd.go`), congestion (`forward_pool_congestion.go`),
prefix-limit (`peer_initial_sync.go`), or the operator command `request peer
teardown` (`plugins/cmd/peer/peer.go`).

**Why this survived.** `TestSessionGracefulClose` (`session_test.go`) calls
`session.Close()` **directly**, without ever running `Session.Run`, so `s.conn` is
still live and the NOTIFICATION *is* built. It asserts only `NoError` and
`StateIdle`, never the wire bytes. The dead path therefore compiles, shows coverage,
and reads as working. `test/reload/signal-stop.ci` does drive a real
`action=sigterm` but asserts only the pre-shutdown route, the EOR and a clean exit
(`:10-11`), so it stays green either way.

**Behavior to change:** an administrative shutdown sends Cease / Administrative
Shutdown to each Established peer, and the send is flushed before the socket closes.

**Behavior to preserve:** the 2s per-peer and 3s engine shutdown budgets. A graceful
send must fit inside them, not extend them; a shutdown that hangs waiting on an
unresponsive peer is worse than one that closes bluntly.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An operator (or an init system, or a container runtime) sends SIGTERM to `ze`. Input
at entry: the signal, plus the set of Established sessions and their sockets.

### Transformation Path
The twelve hops in Current Behavior are the path. In one line: signal → `r.Stop()` →
`r.ctx` cancel → peer context cancel (child of `r.ctx`) → session cancel goroutine
closes the socket and nils it → `Session.Run` returns → `runOnce`'s defer nils
`p.session` → `p.cleanup()` finds nothing to close. The NOTIFICATION build site is
downstream of two nils that the cancel has already set.

**Where a fix inserts.** Before the cancel propagates, not after. The existing
`Session.Teardown(NotifyCeaseAdminShutdown, ...)` path (`session_connection.go`)
already works and is exercised by four other callers, so the fix is a call-site
and ordering problem, not new wire code.

→ Constraint (corrected 2026-08-10, during implementation): **`Reactor.cleanup()`
Phase 1 is NOT that point, and the design section above was wrong to name it.**
`cleanup` is reached only from `monitor()`, which blocks on `<-r.ctx.Done()`
(`reactor.go`). `Reactor.Stop` cancels `r.ctx`, and every session's cancel
goroutine watches a child of it (`session.go`, `Run`), so by the time `monitor`
wakes, `closeConn` is already racing `cleanup` for the socket. The only point
that is ordered rather than racing is **inside `Reactor.Stop`, before
`cancel()`**, and that is where the fix went. Building it in `cleanup` Phase 1
would have produced a shutdown NOTIFICATION that arrived on a fast machine and
vanished on a slow one, which is worse than the defect it replaced.

→ Decision (2026-08-10, round 2): **a RESTART takes a different exit and sends
nothing.** `Reactor.Stop` split into `Stop` and `StopForRestart`; see Key
Design Decisions.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Signal ⇄ reactor | `signal.go` callback → `r.Stop()` | Yes, traced |
| Reactor ⇄ peer | context cancel, parent to child | Yes, traced |
| Peer ⇄ session | `p.session`, nil'd by a defer before cleanup reads it | Yes, traced |
| Session ⇄ socket | `closeConn` flush → `CloseWrite` (FIN) → drain → `Close` | Yes, traced |
| Ze ⇄ peer daemon | peer sees FIN, not NOTIFICATION | Yes, by the tracing agent; NOT yet observed against a real peer |
| Ze ⇄ peer GR state | unverified: whether the peer retains routes as stale | No -- AC-7 |

### Integration Points
- `internal/component/bgp/reactor/reactor.go` (`cleanup` Phase 1 ordering)
- `internal/component/bgp/reactor/session_connection.go` (`Teardown`, `Close`, `closeConn`)
- `internal/component/bgp/reactor/peer_run.go` (the dead guard)
- `test/reload/signal-stop.ci` (the functional assertion that is missing)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Sending Cease before the cancel propagates fits inside the existing 2s per-peer and 3s engine budgets | The send is one small message per peer on an already-open socket; `Teardown` does this today on four other paths | Shutdown slows or hangs, which is worse than the defect | Measure shutdown wall time with N peers before and after | confirmed |
| A-2 | A GR-capable peer currently retains Ze's routes as stale across an admin shutdown | Peer sees a bare FIN, and Ze advertises GR (`docs/features/rfc-status.md`) | The defect is cosmetic rather than operator-visible, and priority drops | AC-7: a real peer (FRR or BIRD) in the interop lab | **broken** |
| A-3 | `Session.Teardown` is safe to call from the reactor's cleanup goroutine | It is called today from BFD, congestion and prefix-limit paths | A data race on session state during shutdown | `make ze-race-reactor` (`ai/rules/testing.md`) | confirmed |

→ Decision: **A-1 confirmed, at the daemon and at the unit level.** With one
Established FRR peer, `ze` exited 2.8s after SIGTERM with the fix and 1.7s
without it (`test/interop/scenarios/shutdown-cease-frr`, two runs each). The
budget is explicit rather than inherited: `shutdownNotifyBudget` is 1s in
`internal/component/bgp/reactor/reactor.go`, which is the room between the hub's
3s engine budget and `cleanup`'s own 2s Phase 2 wait.
`TestReactorStopStaysInBudgetWithUnreadablePeer` drives the black-holed case,
where the write would otherwise block for `controlWriteDeadlineMin` (10s,
`session_write.go`).

→ Decision: **A-2 is BROKEN, measured, and the operational premise in the Task
section goes with it.** FRR 10.3.1, configured `bgp graceful-restart`, withdrew
`10.10.0.0/24` **0.1s** after Ze's bare-FIN shutdown, reported
`Notifications: 0 0`, and gave its operator `Last reset ... Peer closed the
session (n/a)`. Nothing was held stale. The mechanism: Ze advertises the GR
capability with a Restart Time and **no per-AFI/SAFI tuple**
(`capability.GracefulRestart.Families` is empty in the OPEN Ze sends), so FRR
reads `Remote GR Mode: NotApplicable` and never enters helper mode. RFC 4724
Section 3 makes that tuple list the statement of which families a speaker
preserves. **That encoding is conformant, and it is the CONFIG surface that is
defective**: RFC 4724 Section 4 recommends the empty list for a speaker that
preserves nothing (`rfc/full/rfc4724.txt`, lines 288-296), while
`capability { graceful-restart { restart-time 300; } }` lets an operator ask
for something the advertisement does not carry. See Known Limitations; it is
not a compliance question and must not be re-raised as one.

→ Decision: **the fix is still right, and the reason changes.** What the peer
loses is not its routing table, it is the REASON. `Peer closed the session
(n/a)` is what an operator reads on the other side of a planned maintenance,
where `Cease/Administrative Shutdown` is what the RFC 4271 Section 8.2.2 action
list says they should read. That is a diagnosability defect and an RFC
conformance one, not a traffic one. Nothing in the design changed; the priority
argument in the Task section is downgraded, and the stale-route claim there must
not be repeated.

### Risks
| ID | Risk | Early signal | Mitigation / fallback | Status |
|----|------|--------------|----------------------|--------|
| R-1 | A graceful send on shutdown blocks on an unresponsive peer and turns a fast exit into a hang | Shutdown wall time rises with a black-holed peer | Bound the send by the existing budget and treat a failed write as non-fatal (AC-2) | closed -- `shutdownNotifyBudget` bounds the WAIT with `syncutil.WaitGroupWait`, so a send that outlives it is abandoned rather than awaited |
| R-2 | Fixing the ordering introduces a shutdown race the race detector does not catch on a quiet host | Intermittent reds under load | `make ze-race-reactor` (`-race -count=20`), which `ai/rules/testing.md` requires for reactor concurrency changes | closed -- clean, 20 iterations, 0 DATA RACE |
| R-3 | The fix is written as "widen the deadline", which changes nothing | A diff that touches only timeouts | Current Behavior states explicitly that timing is not the cause | closed -- no existing deadline was widened; the diff is a call site and an ordering |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Shutdown. A bad fix either hangs the daemon on exit (R-1) or races session teardown (R-2). Both are worse than today's silent-but-fast close, which is why AC-2 is written about budgets rather than about the message |
| How is it reverted? | Single commit revert; the wire behaviour returns to a bare FIN |
| Who else touches this path? | Anything changing reactor shutdown ordering, and the GR plugin, whose stale-route behaviour is the operational reason this matters |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGTERM to a running `ze` with one Established peer | → | the graceful step in `Reactor.cleanup()` Phase 1 | `test/reload/signal-stop.ci`, extended to assert the NOTIFICATION on the wire (AC-1, AC-5) |
| SIGTERM with a peer whose socket is already broken | → | the same, failing closed | a `.ci` or unit test asserting shutdown still completes inside budget (AC-2) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorStopSendsAdminShutdownBeforeCancel` | `internal/component/bgp/reactor/shutdown_notify_test.go` | AC-1 at the unit level: the peer reads the exact 45 octets. → Constraint (corrected 2026-08-10, round 2): it is NOT the ordering guard and the round-1 claim that it was is withdrawn. It observes the FAR end of the pipe, which races the cancel; measured against the reverted ordering it went red 21 times in 50 under `-race -count=50` | pass |
| `TestReactorStopNotifiesWhileItsContextIsStillLive` | same | AC-1's ordering, deterministically. Reads `r.ctx.Err()` from inside the send, through `onNotifSent` (`session_write.go`), so program order decides. 50/50 green as shipped, 50/50 red on the reverted ordering | pass |
| `TestReactorStopForRestartSendsNoNotification` | same | AC-9. Table over both stops: `Stop` puts the Cease on the wire, `StopForRestart` closes the socket with zero octets on it | pass |
| `TestShutdownNotifySendsCeaseFromEveryConnectedState` | same | AC-8, and the `RFC4271-8.2.2-18 positive` tag. OpenSent, OpenConfirm, Established | pass |
| `TestRFC4271NoCeaseWithoutAManualStop` | same | `RFC4271-8.2.2-18 negative`: zero octets reach a peer of a reactor nobody stopped | pass |
| `TestReactorStopStaysInBudgetWithUnreadablePeer` | same | AC-2, the black-holed peer: Stop returns inside `shutdownNotifyBudget` rather than waiting out a 10s write deadline | pass |
| `TestShutdownNotifyWithoutSessionIsQuiet` | same | AC-2, no session: no panic and no teardown queued for a drain that never comes | pass |
| `TestReactorStopAcceptsNoInboundSessionWhileItNotifies` | `internal/component/bgp/reactor/shutdown_notify_test.go` | AC-10, the INBOUND rail. Round 3's fixture shape: a slow peer holds `Stop` for the whole budget, the elapsed assertion runs FIRST, then a real TCP dial at the reactor's own listen address during that window. Red on the round-3 tree at `Should be zero, but was 1` | pass |
| `TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed` | same | AC-10's residual: the one connection a Listener had ALREADY accepted when `stop` took `r.mu`. Drives `tryCreateDynamicPeer` directly, because the seam is that lock | pass |
| `TestReactorStopStartsNoListenerForAnAddressAddedWhileItNotifies` | `internal/component/bgp/reactor/shutdown_notify_test.go` | AC-11, the LISTENER producer. A netlink address-added event inside the budget re-opened a listener on the still-live `r.ctx`: `Reactor.cleanup` releases the EventBus subscriptions, and `monitor()` reaches `cleanup` only after the cancel. Red on the round-4 tree at `Should be false` | pass |
| `TestPeerStartWithContextDoesNotLiftTheStopsSeal` | same | AC-11, the PEER-START producer. `Peer.StartWithContext` cleared `Peer.stopping`, and `Reactor.StartPeers` reaches it from `coord.OnPostStartup` with `r.mu` released. Red on the round-4 tree at `Condition satisfied` (the peer dialed inside the budget) | pass |
| `TestRunOncePublishesNoSessionAfterTheStopHasMarkedThePeer` | same | AC-11, the DIAL producer, for a cycle already past `run`'s loop top. Red on the round-4 tree at `expected: 1, actual: 2` | pass |
| `TestSessionConnectSendsNoOpenOnceTeardownHasRun` | same | AC-11, a dial already IN FLIGHT. Table: a live session puts its OPEN on a real TCP listener, a torn-down one puts nothing. Red on the round-4 tree at `"37" is not less than or equal to "0"` -- the OPEN, on the wire, from a session the notify had already torn down | pass |
| `TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer` | `internal/component/bgp/reactor/shutdown_notify_test.go` | AC-11 at the frame: the accept rail against an EXISTING CONFIGURED peer, for both stops. `findPeerByAddr` succeeds there, so round 4's `tryCreateDynamicPeer` gate is never consulted and `acceptOrReject` goes straight to `Peer.AcceptConnection`. Red on the round-5 tree at `Expected nil, but got: &net.TCPConn{...}` in the `restart` case | pass |
| `TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn` | `internal/component/bgp/reactor/shutdown_notify_test.go` | AC-11's ownership half, round 7: the refusal hands the connection back rather than closing one it does not own. Table over both accept entry points. The `accept with open` row is RED on the round-6 tree at `write tcp ...: use of closed network connection`; the `accept` row is refused one step earlier, at `Session.Accept`'s entry check, and does not discriminate | pass |
| `TestSealedSessionConnectClosesTheConnItDialed` | same | The other half of the same rule, round 7: `Connect` owns what it dialed and must close it, so moving the close out of `connectionEstablished` cannot leak a socket. Green on the round-6 tree too -- it guards the rail the round-7 fix must not regress | pass |
| `TestSessionGracefulCloseSendsAdminShutdown` | `internal/component/bgp/reactor/session_test.go` | AC-3: the test that used to cover the dead `Session.Close` now asserts the wire bytes of a real `Teardown` | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| per-peer shutdown budget | 0..2s | 2s | N/A | a send that exceeds it must be abandoned, not awaited |
| engine shutdown budget | 0..3s | 3s | N/A | same |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `signal-stop-cease.ci` | `test/reload/`, new file (B-1) | An operator stops Ze and the peer is told why, rather than seeing the socket vanish | pass. → Constraint (corrected 2026-08-10, round 2): it guards the FEATURE, not the ordering, and the round-1 claim above it is withdrawn. The "peer got 2 messages, timed out waiting for the third" signature is what REMOVING the send produces. With `notifyPeersShutdown` moved after `cancel()` the file passes 20/20, because the octets are already in the kernel send buffer when `closeConn` sends FIN rather than RST. The ordering guard is the unit test above |
| `shutdown-cease-frr` | `test/interop/scenarios/` | AC-7 and AC-1 against a real daemon: FRR reports `lastNotificationReason=Cease/Administrative Shutdown` | pass, and red before the fix (`FRR reports no Cease/Administrative Shutdown NOTIFICATION from Ze`) |

## Files to Modify
- `internal/component/bgp/reactor/reactor.go` - `Stop` sends before it cancels, plus `notifyPeersShutdown` and `shutdownNotifyBudget` (NOT `cleanup()` Phase 1: see the corrected constraint in Data Flow). Round 2 split it: `Stop` and `StopForRestart` over one `stop(notify bool)`. Round 4 added the seal: `stop` marks the peers, shuts the listeners and sets `Reactor.stopping` in ONE hold of `r.mu`. Round 5 makes `Reactor.StartWithContext` the ONLY place either half of that seal is lifted, adds the `r.stopping` read inside `startListenerForAddressPort`, and rewrites `stop`'s comment, which asserted both rails were shut while two producers read no seal. Round 6: `stop` seals every peer's live session on BOTH stops, and its comment is rewritten again -- it claimed an accepted conn "needs a published `p.session` (there is none after the seal)", which is false, since the seal stops new publishes and never unpublishes. Round 7: the same comment again, on the lock hierarchy -- it said no Peer lock is ever taken under `r.mu` while five sites take one -- and `startListenerForAddressPort`'s "three call sites", which are four
- `docs/features/rfc-status.md` - round 4, the RFC 4271 row's Implemented coverage cell records the Section 8.2.2 ManualStop Cease. Round 1's checklist claimed this and never wrote it
- `docs/architecture/behavior/fsm-active.md`, `fsm-connect.md`, `fsm-established.md`, `fsm-open-confirm.md`, `fsm-open-sent.md` - round 4, the `EventManualStop` rows credited `Session.Stop` with a Cease it does not send
- `internal/component/config/infra/hook.go` - round 2, `ReactorHandle` gains `StopForRestart` so always-on daemon code can take the silent stop without naming the BGP type
- `cmd/ze/hub/infra_setup.go` - round 2, `stopForRestart` pairs the graceful-restart marker write with the silent stop, in one place; the API reboot callback calls it
- `cmd/ze/hub/service_ssh.go` - round 2, `restart` and `reboot` call `stopForRestart`; `shutdown` keeps `Stop`
- `cmd/ze/hub/ssh_infra.go` - round 2, `sshWireInputs.WriteGRMarker` becomes `StopForRestart` (the seam carries the paired action, not half of it)
- `docs/architecture/behavior/fsm-active.md`, `fsm-connect.md`, `fsm-established.md`, `fsm-open-confirm.md`, `fsm-open-sent.md`, `fsm-idle.md` - round 2, the ManualStop rows and source anchors named the deleted `Session.Close`; they now name `Session.Stop` / `Session.Teardown`, the two real producers of `EventManualStop` (`session.go`, `session_connection.go`). `fsm-idle.md` also claimed `CloseWithNotification` fires ManualStop; it fires Event 23 (`OpenCollisionDump`). `fsm.md` needed no edit: its only match is `Session.CloseWithNotification`, which still exists
- `internal/component/bgp/reactor/reactor_dynamic.go` - round 4, `tryCreateDynamicPeer` refuses once `stop` has sealed. It takes the same `r.mu` the seal is set under, so the answer is ordered rather than raced
- `internal/component/bgp/reactor/reactor_peers.go` - round 4, `AddPeer` does not start a peer a concurrent reload adds after the seal, for the same reason
- `internal/component/bgp/reactor/peer.go` - `ShutdownNotify`, the per-peer send. Round 4 records on `Peer.stopping` that it covers the OUTBOUND rail only. Round 5: `StartWithContext` no longer clears that flag, because it holds no lock that could order the clear against a stop. Round 6: `Peer.sealSession` carries the stop's seal to the one session that can still publish a conn, and the `Peer.stopping` comment stops crediting the listener shutdown with the inbound rail
- `internal/component/bgp/reactor/peer_run.go` - the dead guard in `cleanup`, removed. Round 5: `runOnce` reads `Peer.stopping` inside the `p.mu` hold that publishes `p.session`
- `internal/component/bgp/reactor/reactor_iface.go` - round 5, NOT edited, and that is the point: `handleAddrAddedPayload` needs no change because the seal is read in `startListenerForAddressPort`, which it already calls with `r.mu` held
- `internal/component/bgp/reactor/session_connection.go` - `Close()` deleted. Round 5: `connectionEstablished` refuses a torn-down session inside the `s.mu` hold that publishes `s.conn`, which is the gate `Session.Connect` never had and `Session.Accept` always did. Round 6: `Session.Accept`'s `s.tearingDown.Store(false)` deleted, because it NEUTRALISED that gate, and `Session.seal` added as the field's only writer. Round 7: that refusal no longer closes the connection -- it does not own it on either accept rail -- and `Session.Connect`, which does own its dial, closes it there instead
- `internal/component/bgp/reactor/session.go` - round 6, the `tearingDown` field and the lock-hierarchy note describe a one-way seal rather than "set by Teardown to block Accept races"
- `internal/component/bgp/reactor/reactor_iface.go` - round 6, `handleAddrAddedPayload` logs the designed refusal at Debug. It logged `errReactorStopping` at Error, so every shutdown that met a netlink event put a designed refusal in front of the operator as a failure. Round 5's row said this file needed no change and that remains true of the SEAL; the log level is a separate defect the seal created
- `internal/test/cli/cmd_replay.go` - the replay harness used `Session.Close` for cleanup and now uses `Session.Stop`. Found by the compiler, not by the spec's audit
- `internal/component/bgp/reactor/session_test.go` - `TestSessionGracefulClose` asserts wire bytes, not just `NoError`
- `test/interop/interop.py` - `docker_signal`, so a scenario can send a real SIGTERM to the ze container
- `rfc/audit/rfc7606.json` - five verdicts SHIFTED by the `session_test.go` edit, re-stamped with `make ze-rfc-reseal` (byte-identical units, nothing re-judged)
- `ai/RFC-REQUIREMENTS.md` - generated, `make ze-rfc-index`
- ~~`test/reload/signal-stop.ci` - assert the NOTIFICATION~~ blocked, see B-1
- `test/plugin/rfc7606-relay-one-field.ci` - **the booby trap did NOT spring, and
  the file was not touched.** Measured 2026-08-10 with the fix in place: 6/6 green
  under `ze-test bgp plugin --pattern rfc7606-relay-one-field -c 6`, and the whole
  plugin suite 582/582. The reason is in the runner, not in luck:
  `Runner.runTest` calls `terminateGracefully(clientCmd)` **after**
  `peerCmd.Wait()` (`internal/test/runner/runner_exec.go`), so every ze-peer
  process has already exited when ze gets its SIGTERM. `Reactor.Stop` then finds
  no live session to notify, nothing is written, and no `notification sent` line
  reaches stderr for the reject rule to match. A `.ci` only meets the new
  behaviour when the SIGTERM comes from a peer that is still connected, i.e.
  through `action=sigterm`, which is what `test/reload/signal-stop-cease.ci` does.
  The prediction below is kept because it was the right thing to check.
  That test carries `reject=stderr:pattern=notification`, and it is green today ONLY
  because Ze sends no Cease on shutdown. The moment this spec lands, every shutdown
  emits `notification sent` (`internal/component/bgp/reactor/session_write.go`)
  and that reject rule flips RED on a test that has nothing to do with shutdown.
  Found by an independent reviewer on 2026-07-29, who verified it by establishing a
  real session against `ze-test peer --mode sink`, sending SIGTERM, and observing the
  peer receive no type-03 frame. Narrow the reject rule to the session-reset case it
  actually means before changing the shutdown path, not after the red appears.
  NOTE: that file is RFC-tagged, so the `rfc-tagged-test` hook will block a behaviour
  change to it; only the user may authorise one, via `// rfc-test-change-approved:`
- `rfc/short/rfc4271.md` - extract the §8.2.2 ManualStop action list at its true level (AC-6)

## Files to Create
- `internal/component/bgp/reactor/shutdown_notify_test.go` - the unit tests, both RFC polarities
- `test/reload/signal-stop-cease.ci` - the whole-daemon wire assertion (B-1 explains why it is not in `signal-stop.ci`)
- `test/interop/scenarios/shutdown-cease-frr/` - the GR-peer interop scenario (AC-7): `ze.conf`, `frr.conf`, `announce-shutdown.py`, `check.py`

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No leaf is added or changed. The behaviour is bound to which stop the daemon takes, not to config |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | `daemon shutdown`, `daemon restart` and `daemon reboot` keep their spelling; only the reactor call behind them changed (`cmd/ze/hub/service_ssh.go`) |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | N-A | No RPC added. The daemon behaviour is covered by `test/reload/signal-stop-cease.ci` |
| Pipe completeness | N-A | No command output added |
| Env var registration | No | No env var added; `shutdownNotifyBudget` is a constant, deliberately, so a shutdown cannot be made to hang by configuration |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, certificate or binary |
| Prometheus counters/metrics | No | The existing counter already covers it: `session.onNotifSent = p.IncrNotificationSent` (`peer_run.go`) is wired on every running peer, so a shutdown Cease is counted like any other NOTIFICATION |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute added |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A defect fix on an existing action, not a new one |
| 2 | Config syntax changed? | No | -- |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | No | -- |
| 5 | Plugin added/changed? | No | -- |
| 6 | Has a user guide page? | No | No guide page documents what the peer sees on a daemon stop; grepped `docs/` for `daemon shutdown`, `daemon restart` and `daemon reboot`, and the only hits are unrelated (config-archive, web-interface, traffic qdisc) |
| 7 | Wire format changed? | No | The 45-octet NOTIFICATION is the existing `Session.Teardown` encoding; its diagram is in this spec's RFC Documentation section |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` carries `RFC4271-8.2.2-18`, written in round 1. `docs/features/rfc-status.md`'s RFC 4271 row was NOT: round 1 claimed it in this cell and never wrote it, and round 4's review found the file unmodified in the tree, its Implemented coverage cell recording the Event 10 hold-timer action list alone. Written in round 4, in the Implemented coverage cell, leaving the Remaining cell's spelled gap count untouched (`check_gap_count_agreement`). Round 2 adds no requirement: a restart raises no Event 2 (Key Design Decisions) |
| 10 | Test infrastructure changed? | Yes | `test/interop/interop.py` gained `docker_signal`; `docs/functional-tests.md` describes the runner surface and needs no row for a scenario helper |
| 11 | Affects daemon comparison? | No | -- |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/fsm-active.md`, `fsm-connect.md`, `fsm-established.md`, `fsm-open-confirm.md`, `fsm-open-sent.md`, `fsm-idle.md`: the ManualStop rows named the deleted `Session.Close`. Updated in round 2. Round 4 corrected the wire-side-effect cell in five of the six: it credited `Session.Stop` with a Cease it does not send, and `fsm-active.md` said the opposite two rows apart |
| 13 | Route metadata keys added/changed? | No | -- |
| 14 | Prometheus counters added/changed? | No | -- |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | -- |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grepped `docs/` for the deleted symbol and for the changed files. The six FSM docs above were the hits, through `<!-- source: internal/component/bgp/reactor/session_connection.go — Close, ... -->` anchors. No gate reads those anchors (`validate.py`'s regex captures the path only), which is why they survived round 1 |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The FSM docs carry event tables, not examples; they are corrected above |

## Implementation Steps

1. **Phase: Validate A-2 first.** Answer AC-7 before writing the fix. If peers do not
   retain stale routes, this is a tidy-up; if they do, it is an operator-visible defect
   and the rest of the spec is urgent. Cheapest route is the existing interop lab with
   a GR-capable peer.
2. **Phase: Wiring.** Extend `test/reload/signal-stop.ci` to assert the NOTIFICATION.
   It must FAIL first, against today's bare FIN (`ai/rules/testing.md`).
3. **Phase: The ordering fix.** Send Cease from `Reactor.cleanup()` Phase 1, before the
   cancel propagates, reusing `Session.Teardown`. Bound it by the existing budgets.
4. **Phase: Remove the dead code.** `Session.Close()` and the `peer_run.go` guard are
   reachable or gone, and `TestSessionGracefulClose` stops asserting a path nothing takes.
5. **Phase: Extraction.** Add the §8.2.2 requirement to `rfc/short/rfc4271.md` at its
   true level, with a tagged test. This is the part that stops the ledger being silent.
6. **Phase: Race.** `make ze-race-reactor` per `ai/rules/testing.md`.
7. **Phase (round 2): the restart path.** Split `Reactor.Stop` so a stop this
   speaker comes back from sends nothing, and prove both halves in one test.
8. **Phase (round 4): the inbound rail.** Round 3 closed the OUTBOUND half of
   the flap with `Peer.stopping` and left the INBOUND half open. Shut the
   listeners inside `Reactor.stop`, in the same hold of `r.mu` that marks the
   peers, and refuse a dynamic peer once that seal is set.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-9 each have code and a named test; see Implementation Audit |
| Every route into `Reactor.Stop` classified | Each production caller is either a stop for good or a stop to come back from, and takes the matching method. `gopls references` on `Reactor.Stop`: `cmd/ze/hub/infra_setup.go` (reboot, silent), `cmd/ze/hub/service_ssh.go` x3 (shutdown notifies; restart and reboot silent), `reactor.go` `startSignalHandler` (SIGTERM, notifies), `reactor_api.go` `reactorAPIAdapter.Stop` (`daemon shutdown`, notifies), `internal/component/bgp/plugin/register.go` and `internal/component/bgp/cli/childmode.go` (process exit, notifies) |
| The ordering assertion is the ordering assertion | The test that CLAIMS to guard the ordering must fail on the reverted ordering every time, not most times. Measured `-race -count=50` both ways |
| No test claims evidence it does not carry | Each TDD row states what would make it red, and that statement was MEASURED, not reasoned |
| Budget arithmetic still holds | 1s notify + 2s `cleanup` Phase 2 = the hub's 3s. Any change to one of the three is a change to all three (Known Limitations) |
| Rule: `ai/rules/stale-comments.md` | Every doc and comment naming `Session.Close` is gone, and `gopls symbols` confirms the symbol is gone rather than renamed |
| Rule: `ai/rules/rfc-compliance.md` | Silence on restart lowers no requirement: a restart raises no Event 2. Stated with the RFC section text, not with a summary |
| Registration over hardcoding | N-A: no command, view, family or handler is added, so nothing new needs to register and be core-discovered. The one new cross-component surface is `infra.ReactorHandle.StopForRestart`, which is the existing always-on seam gaining a method, not a plugin spelling in a shared package. `cmd/ze/hub` still names no BGP type (`ai/rules/plugins.md`), which is what keeps `//go:build ze_bgp` compilable out -- verified with `go vet -tags ze_core ./cmd/ze/...` |

## Design Insights

The defect is invisible in three separate ways at once, and that is the transferable
lesson: the code exists and compiles; a unit test covers it and passes; and the
functional test that drives the real signal asserts everything except the thing that
went missing. Coverage of a dead path reads exactly like coverage of a live one.

## Key Design Decisions

- The fix belongs BEFORE the context cancel, not after. Everything downstream of the
  cancel is already nil by construction.
- Reuse `Session.Teardown` rather than reviving `Session.Close`: the former is exercised
  by four production callers, the latter by none.
- **A restart sends nothing at all, so `Reactor.Stop` split in two** (round 2).
  `Stop` notifies and then cancels; `StopForRestart` cancels in silence, and
  `daemon restart` and `daemon reboot` take it (`cmd/ze/hub/infra_setup.go`,
  `cmd/ze/hub/service_ssh.go`). Round 1 had all three of those callbacks on
  `Stop`, one line after `writeGRMarker()`, so a planned reboot told every peer
  to drop exactly the routes the marker had just been written to keep.
  RFC 4724 Section 5, "Changes to BGP Finite State Machine", is why
  (`rfc/full/rfc4724.txt`, lines 569-585): it REPLACES RFC 4271's FSM text, and
  in the replacement a peer that receives a NOTIFICATION (Event 24 or Event 25)
  still "deletes all routes associated with this connection", with no condition
  attached. The retain branch is the conditional one: it needs a
  TcpConnectionFails (Event 18) AND the Graceful Restart Capability received
  "with one or more AFIs/SAFIs" (lines 587-604). **Ze advertises an empty tuple
  list, so no peer takes that branch for a Ze session today** (A-2, Known
  Limitations: FRR reads `Remote GR Mode: NotApplicable` and withdraws in 0.1s).
  So the reason is NOT that a bare FIN preserves the peer's forwarding state
  today. It is that a bare FIN is the only end a peer can EVER retain across,
  and a Cease forecloses it for every peer and every configuration: fixing the
  tuple list makes silence start working, and can never make a Cease work.
  RFC 8538 is the extension that lets a NOTIFICATION coexist with graceful
  restart and it is `Unsupported` (`docs/features/rfc-status.md`), so silence is
  the only option that can work. The subcode would have been wrong in any case: RFC 4486
  subcode 2 is Administrative Shutdown, subcode 4 is Administrative Reset.
- **This lowers nothing `RFC4271-8.2.2-18` asks, because a restart is not Event
  2.** RFC 4271 Section 8.1.2 defines ManualStop as the administrator stopping
  the PEER CONNECTION. `daemon restart` stops the process and brings it back;
  the sessions end because the process ends, which is the Event 18 the peer
  sees and the exact case RFC 4724 was written for. Ze raises no ManualStop on
  that path and owes no Cease on it. `TestRFC4271NoCeaseWithoutAManualStop`
  already says the Cease belongs to the stop rather than to a session being up,
  and `TestReactorStopForRestartSendsNoNotification` is the second instance of
  that same rule.
- **The marker write and the silent stop are ONE decision, and the code says
  so.** `stopForRestart` in `cmd/ze/hub/infra_setup.go` does both, and the
  three callbacks (API reboot, SSH restart, SSH reboot) call it rather than
  repeating the pair. A call site that wrote the marker and then picked the
  wrong stop is the defect round 1 shipped; there is now no site that can pick.

## Known Limitations

- ~~The GR stale-route consequence is reasoned from what the peer observes~~
  **Settled 2026-08-10 by measurement, and it was wrong: FRR withdrew in 0.1s.**
  See A-2 in Risks & Assumptions.
- ~~What FSM event fires on the shutdown path instead of `fsm.EventManualStop`~~
  **Answered by the fix rather than by tracing: the shutdown path now raises
  `fsm.EventManualStop` itself, through `Session.Teardown` (`session_connection.go`,
  `teardown`). Before it, no FSM stop event fired at all on this path -- the
  session left Established through `handleConnectionClose` on the socket the
  cancel goroutine had already closed.**
- **Ze advertises Graceful Restart with no per-AFI/SAFI tuple.** Found while
  answering AC-7, and NOT fixed here: `capability.GracefulRestart.Families` is
  empty in the OPEN Ze sends, which is why FRR reports `Remote GR Mode:
  NotApplicable` and never enters helper mode for a Ze peer. RFC 4724 Section 3
  makes that list the statement of which families the speaker preserves.
  **This is NOT a conformance defect, and the escalation gate in
  `ai/rules/rfc-compliance.md` does not fire on it.** RFC 4724 Section 4
  sanctions exactly this encoding: "even if the speaker does not have the
  ability to preserve its forwarding state for any address family during BGP
  restart, it is still recommended that the speaker advertise the Graceful
  Restart Capability to its peer (as mentioned before this is done by not
  including any <AFI, SAFI> in the advertised capability)"
  (`rfc/full/rfc4724.txt`, lines 288-296). The empty list is the RFC's own
  spelling of "I preserve nothing", and Ze preserves nothing.
  **What IS defective is the config surface.**
  `capability { graceful-restart { restart-time 300; } }` reads as an operator
  asking for their routes to be held for 300 seconds, and produces an
  advertisement that promises nothing of the kind. The leaf accepts a number
  the wire cannot then mean. That is a config-and-documentation defect with its
  own spec, not a branch of this one, and a future reader must not re-raise it
  as a compliance question.
- **An abandoned notify goroutine can block `closeConn` while it holds
  `s.mu`.** `Session.teardown` writes the NOTIFICATION under `s.writeMu` with
  the control-message write deadline, whose minimum is 10s
  (`controlWriteDeadlineMin`, `session_write.go`). `notifyPeersShutdown` stops
  WAITING after `shutdownNotifyBudget` (1s), but it cannot stop the goroutine,
  which keeps `s.writeMu` for up to the remaining 9s. `closeConn`
  (`session_connection.go`) takes `s.mu` and then `s.writeMu` to flush, in that
  order, so it parks holding `s.mu` until the write deadline fires.
  **Exit stays bounded, with zero margin.** `Reactor.cleanup` Phase 2 waits on
  each `peer.Wait(waitCtx)` under its own 2s deadline (`reactor.go`) and
  abandons the peer when it expires, and the hub gives the whole engine 3s
  (`cmd/ze/hub/main.go`): 1s of notify plus 2s of Phase 2 is that whole 3s. A
  peer whose socket black-holes therefore costs the entire budget and leaves
  nothing for the rest of the stop path.
  `TestReactorStopStaysInBudgetWithUnreadablePeer` returns well inside it
  today, and the fix is not a wider budget but a write deadline taken from the
  shutdown budget rather than from the control-message one. Its own spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] `make ze-verify`
- [ ] `make ze-race-reactor` (reactor concurrency change)
- [ ] AC-7 answered with evidence, not reasoning

### TDD
- [ ] Tests written
- [ ] Tests FAIL (the `.ci` assertion must fail against today's bare FIN before the fix)
- [ ] Tests PASS
- [ ] Every new requirement row carries a positive and a negative tagged test

### Closure
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] Spec removed in commit B

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SIGTERM to `ze` with one Established peer | The peer receives a NOTIFICATION with Error Code Cease and subcode Administrative Shutdown BEFORE the TCP close, asserted on the wire, not by a log line |
| AC-2 | The same, with the peer socket already broken | Shutdown still completes inside the existing 2s/3s budgets; a failed send is logged, never fatal, and never hangs. → Constraint (corrected 2026-08-10, round 2): the log line is `logNotifyErr`'s "notification send failed" (`internal/component/bgp/reactor/session.go`), inside the session, where the write error exists. It is NOT in `Peer.ShutdownNotify`: `Session.teardown` returns nil on every path (`session_connection.go`), so the `if err != nil` branch round 1 wrote there could never run. The branch is deleted rather than the AC weakened -- a branch that cannot run reads as coverage, which is the defect this whole spec is about (AC-3) |
| AC-3 | `Session.Close()` after this spec | It has a reachable production caller, or it is deleted. Dead code that appears to implement a protocol action is worse than its absence, because it reads as coverage |
| AC-4 | `peer_run.go` | Either reachable, or removed with its guard |
| AC-5 | `test/reload/signal-stop.ci` | Asserts the NOTIFICATION on the wire, so the behaviour cannot silently regress to a bare FIN again. → Constraint: the assertion exists and passes, in `test/reload/signal-stop-cease.ci` rather than in `signal-stop.ci`. See B-1 |
| AC-6 | `rfc/short/rfc4271.md` | Carries an extracted requirement for the §8.2.2 ManualStop action list at its true level, so the ledger stops being silent about it. **All THREE sites, not just the Established one.** → Constraint (corrected 2026-08-10): the three line numbers in the 2026-07-29 reading were wrong, and one of them is a different EVENT. The ManualStop (Event 2) sites are `rfc/full/rfc4271.txt:3505` (OpenSent, and it words the clause "sends the NOTIFICATION **with** a Cease", which is why a grep for "NOTIFICATION message with a Cease" misses it), `:3718` (OpenConfirm) and `:3943` (Established). `:3733` is OpenConfirm's **AutomaticStop (Event 8)** list, which is a different obligation and belongs to `RFC4271-8.2.2-16`'s neighbourhood, not here. A fix that covers only Established leaves an operator stopping a peer mid-handshake with the same silent close |
| AC-8 | A ManualStop while a session is in OpenSent or OpenConfirm | The peer receives Cease, same as from Established. The FSM lists the action in all three states, so a fix keyed only to Established is incomplete |
| AC-9 | `daemon restart` or `daemon reboot`, which writes the RFC 4724 restarting-speaker marker | The peer receives NO NOTIFICATION at all, so the session ends on TcpConnectionFails, which is the only end RFC 4724 Section 5 lets a peer retain routes across. It does not yet retain them: that branch needs the GR Capability with one or more AFI/SAFI tuples and Ze sends an empty list (A-2). A Cease would foreclose the retention for good; silence leaves it reachable. A shutdown, on the same session, still receives the Cease. Both halves asserted in one test |
| AC-10 | An inbound TCP connection arriving at a stopping daemon, inside the notify budget | No session comes up on it. Round 3 marked every peer `stopping` and closed the OUTBOUND rail; the accept path reads neither that flag nor `p.ctx`, and an address in a dynamic group builds a NEW peer that the stop's own snapshot predates. Both are the same `RFC4271-8.2.2-18` miss on a second session, so the listeners close in the same hold of `r.mu` that sets the flag, and a dynamic peer is refused after that seal |
| AC-11 | ANY connection a sealed daemon still holds or receives: an outbound dial, a dial already in flight, an inbound connection for a CONFIGURED peer, one a `Listener` had already accepted, one arriving on a listener a netlink address-added event started, or one for a peer `StartPeers` started | None of them becomes a BGP session. → Constraint (corrected 2026-08-11, round 6): the round-5 wording said "any NEW SOCKET", and that is the wrong property. It is silent on the population that broke this twice, a connection that ALREADY EXISTED when the seal was taken, and enumerating socket producers is what let rounds 3, 4 and 5 each gate the rail in front of them and leave another open. The property is "no conn becomes a session after the seal". A conn becomes a session at exactly two writes, one writer each: `p.session` in `Peer.runOnce` and `s.conn` in `Session.connectionEstablished`. Both are found by grep, both read the seal inside the lock that publishes, and there is no way round either |
| AC-7 | A GR-capable peer, admin shutdown | The stale-route question in the Task is answered with evidence: either peers correctly withdraw, or the spec records what they do instead. → Decision: answered. FRR 10.3.1 withdraws in 0.1s, with or without the fix, and the reason is that Ze's GR capability carries no per-AFI/SAFI tuple. See A-2 |

## Blockers

| ID | Blocker | Owner | Exact remedy |
|----|---------|-------|--------------|
| B-2 | **The closure commit MUST carry `internal/component/bgp/reactor/shutdown_notify_test.go` AND `test/reload/signal-stop-cease.ci`.** Both are UNTRACKED, and `rfc/requirements/rfc4271.md` is tracked at HEAD with an `RFC4271-8.2.2-18` row citing both, by path and line, for the positive and negative tags. Omit either and `ze-rfc-check` passes in this working tree and fails on a fresh clone: it reads the working tree, and the clone has neither file. Rounds 5 and 6 only APPENDED to the test file, below every tagged test, so the cited lines have not moved. Re-verified 2026-08-11 after round 6: both cited lines of `shutdown_notify_test.go` still open the `RFC requirement: RFC4271-8.2.2-18` positive and negative tags, the cited line of `signal-stop-cease.ci` still opens the positive one, and `git ls-files` still reports both files untracked while `rfc/requirements/rfc4271.md` is tracked. Re-verify once more before committing | the closing round | `--file` both paths in the same `commit_helper.py create` run that carries `rfc/requirements/rfc4271.md`, then run `make ze-tracked-build-check` (the commit carries Go) |
| B-1 | `test/reload/signal-stop.ci` now carries `RFC requirement: RFC4271-8.2.2-18` and does NOT assert it. Writing the tag before the assertion made the file RFC-tagged, after which `.claude/hooks/pretool-writeedit.py` refuses both the assertion and the removal of the tag, and refuses them in one edit too (it reads the file from disk). Only the user may write the `rfc-test-change-approved:` marker that lifts it | the user | EITHER add, immediately after the `action=sigterm:conn=1:seq=2` line, `expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D0306021741646D696E6973747261746976652053687574646F776E` and then delete `test/reload/signal-stop-cease.ci`, OR delete the tag and the NOT-YET-BACKED comment block from `signal-stop.ci` and keep the sibling file. Either edit needs the approval marker in its own replacement text. Recorded in `plan/journal/guard-blocks-its-own-authors-repair.md` |

**Both blockers are RESOLVED at closure (2026-08-11).**

**B-1, cleared by the user.** Thomas wrote the marker himself:
`test/reload/signal-stop.ci` now opens with
`rfc-test-change-approved: Thomas, 2026-08-10`, and the second of the two routes
was taken -- the `RFC requirement:` tag this file never asserted was removed, and
the sibling `test/reload/signal-stop-cease.ci` keeps both the tag and the
`seq=3` wire expectation. One subject per file: this file tests the shutdown
signal, the sibling tests the NOTIFICATION. No test was deleted and no proof was
lost.

**B-2, re-verified immediately before the commit script was prepared.**
`rfc/requirements/rfc4271.md` is tracked and UNMODIFIED, so its
`RFC4271-8.2.2-18` row is already at HEAD. That file is GENERATED and keeps its
own line citations current; the row it holds reads:

```
| `RFC4271-8.2.2-18` | MUST | 8.2.2 | `internal/component/bgp/reactor/shutdown_notify_test.go:174` (unit/verify), `test/reload/signal-stop-cease.ci:3` (functional/verify) | `internal/component/bgp/reactor/shutdown_notify_test.go:207` (unit/verify) |  |
```

Both cited lines of `shutdown_notify_test.go` still open the positive and
negative `RFC requirement: RFC4271-8.2.2-18` tags, the cited line of
`signal-stop-cease.ci` still opens the positive one, and `git ls-files` still
reports both files UNTRACKED. Both are `--file`d in commit A, and
`make ze-tracked-build-check` runs after the script.

## RFC Documentation (Scope: protocol)

RFC 4271 §6.7 (the MAY that governs a speaker choosing to Cease of its own
accord), §8.2.2 (the ManualStop action list, extracted as `RFC4271-8.2.2-18` at
MUST -- see the Decision under Task), RFC 4486 (subcode 2, Administrative
Shutdown), RFC 8203 (the shutdown communication `Session.teardown` attaches to
subcodes 2 and 4).

Wire format of what shutdown now sends, 45 octets:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
+                    Marker, 16 octets of 0xFF                  +   0..15
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Length = 0x002D       |  Type = 3     | Code = 6      |   16..19
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Subcode = 2   | Len = 0x17    |  "Administrative Shutdown"    |   20..44
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

Offsets: 0..15 marker (RFC 4271 §4.1), 16..17 length, 18 type NOTIFICATION,
19 Error Code Cease (§4.5), 20 Error Subcode Administrative Shutdown (RFC 4486
§4), 21 shutdown communication length, 22..44 the UTF-8 text (RFC 8203 §2).

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An administrative shutdown sends Cease / Administrative Shutdown to each Established peer | Done | `Reactor.Stop` → `notifyPeersShutdown` → `Peer.ShutdownNotify` → `Session.Teardown` (`reactor.go`, `peer.go`, `session_connection.go`) | Sent from OpenSent and OpenConfirm too, per §8.2.2's three action lists |
| The send is flushed before the socket closes | Done | `writeMessageWithin` writes and flushes under `s.writeMu` before `closeConn` runs (`session_write.go`, `session_connection.go`) | |
| The 2s per-peer and 3s engine budgets are preserved | Done | `shutdownNotifyBudget` = 1s, waited with `syncutil.WaitGroupWait` (`reactor.go`) | No existing deadline was widened. The zero remaining margin is recorded in Known Limitations |
| The ledger stops being silent about §8.2.2 | Done | `RFC4271-8.2.2-18` in `rfc/short/rfc4271.md`, both polarities tagged | |
| A restart must not foreclose the peer's forwarding state | Done | `Reactor.StopForRestart` and `stopForRestart` (`reactor.go`, `cmd/ze/hub/infra_setup.go`) | Round 2. RFC 4724 Section 5. It does not PRESERVE that state today -- the empty AFI/SAFI tuple list keeps the peer off the retain branch (A-2) -- but a Cease would make retention unreachable for every peer, and silence keeps it one config fix away |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestReactorStopSendsAdminShutdownBeforeCancel` (wire bytes), `TestReactorStopNotifiesWhileItsContextIsStillLive` (ordering), `test/reload/signal-stop-cease.ci` (whole daemon), `test/interop/scenarios/shutdown-cease-frr` (real peer) | |
| AC-2 | Done | `TestReactorStopStaysInBudgetWithUnreadablePeer`, `TestShutdownNotifyWithoutSessionIsQuiet` | The log line is `logNotifyErr`'s, in the session. Round 2 corrected the AC and deleted the unreachable branch in `Peer.ShutdownNotify` |
| AC-3 | Done | `Session.Close` deleted; `gopls symbols internal/component/bgp/reactor/session_connection.go` shows `CloseWithNotification`, `closeConn`, `setCloseReason` and no `Close` | `internal/test/cli/cmd_replay.go` moved to `Session.Stop` |
| AC-4 | Done | `Peer.cleanup` no longer carries the `if p.session != nil` teardown guard (`peer_run.go`) | |
| AC-5 | Done | `test/reload/signal-stop-cease.ci` | In the sibling file, not in `signal-stop.ci`. See B-1 |
| AC-6 | Done | `rfc/short/rfc4271.md`, `RFC4271-8.2.2-18` covering all three ManualStop sites | |
| AC-7 | Done | `test/interop/scenarios/shutdown-cease-frr`, FRR 10.3.1 | Answer recorded in A-2: FRR withdraws in 0.1s either way, and why |
| AC-8 | Done | `TestShutdownNotifySendsCeaseFromEveryConnectedState` (OpenSent, OpenConfirm, Established) | |
| AC-9 | Done | `TestReactorStopForRestartSendsNoNotification`, both halves in one table | Round 2 |
| AC-10 | Done | `TestReactorStopAcceptsNoInboundSessionWhileItNotifies`, `TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed` | Round 4. Discrimination measured: the first goes red on the round-3 tree with `Should be zero, but was 1` |
| AC-11 | Done | `TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer`, `TestReactorStopStartsNoListenerForAnAddressAddedWhileItNotifies`, `TestPeerStartWithContextDoesNotLiftTheStopsSeal`, `TestRunOncePublishesNoSessionAfterTheStopHasMarkedThePeer`, `TestSessionConnectSendsNoOpenOnceTeardownHasRun`, `TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn`, `TestSealedSessionConnectClosesTheConnItDialed` | Round 6 restates the AC as the property and closes it at the two publish sites; the round-5 tests stay green and keep their own rails cheap. Round 7 adds the ownership half: a refusal returns the connection instead of closing one it does not own, so the rail that buffers it for the next cycle is handed a live socket. Each measured red against the tree before its own fix, except the two named in the TDD table as non-discriminators; sources verified byte-identical after restore |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestReactorStopSendsAdminShutdownBeforeCancel` | Done | `shutdown_notify_test.go` | Round 2 corrected its claim: a wire assertion, not the ordering guard |
| `TestReactorStopNotifiesWhileItsContextIsStillLive` | Done | same | Added in round 2. 50/50 green shipped, 50/50 red reverted |
| `TestReactorStopForRestartSendsNoNotification` | Done | same | Added in round 2 |
| `TestShutdownNotifySendsCeaseFromEveryConnectedState` | Done | same | `RFC4271-8.2.2-18 positive` |
| `TestRFC4271NoCeaseWithoutAManualStop` | Done | same | `RFC4271-8.2.2-18 negative` |
| `TestReactorStopStaysInBudgetWithUnreadablePeer` | Done | same | |
| `TestShutdownNotifyWithoutSessionIsQuiet` | Done | same | |
| `TestReactorStopStartsNoListenerForAnAddressAddedWhileItNotifies` | Done | `shutdown_notify_test.go` | Round 5. Red on the round-4 tree at `Should be false` |
| `TestPeerStartWithContextDoesNotLiftTheStopsSeal` | Done | same | Round 5. Red at `Condition satisfied` |
| `TestRunOncePublishesNoSessionAfterTheStopHasMarkedThePeer` | Done | same | Round 5. Red at `expected: 1, actual: 2` |
| `TestSessionConnectSendsNoOpenOnceTeardownHasRun` | Done | same | Round 5. Red at `"37" is not less than or equal to "0"` |
| `TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer` | Done | same | Round 6. Red at `Expected nil, but got: &net.TCPConn{...}` in the `restart` case |
| `TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn` | Done | same | Round 7. Red at `write tcp 127.0.0.1:39615->127.0.0.1:53684: use of closed network connection` in the `accept with open` case |
| `TestSealedSessionConnectClosesTheConnItDialed` | Done | same | Round 7. Not a discriminator: green on the round-6 tree, where `connectionEstablished` closed the dial. It pins that the close stayed after it moved |
| `TestSessionGracefulCloseSendsAdminShutdown` | Done | `session_test.go` | |
| `signal-stop-cease.ci` | Done | `test/reload/` | |
| `shutdown-cease-frr` | Done | `test/interop/scenarios/` | |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify and Files to Create | Done | The round-2 rows (`hook.go`, `infra_setup.go`, `service_ssh.go`, `ssh_infra.go`, the six FSM docs) were added to that list as they were written |
| `test/reload/signal-stop.ci` | Changed | Blocked by B-1; the assertion lives in `signal-stop-cease.ci` |
| `test/plugin/rfc7606-relay-one-field.ci` | Changed | Predicted booby trap did not spring; the reason is recorded in Files to Modify and was measured |

### Audit Summary

- **Total items:** 11 acceptance criteria, 14 tests, 5 task requirements
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** 2 files (both recorded above; neither drops an acceptance criterion)

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-bgp-shutdown-cease-notification-640fa955-f03a-45e8-a58f-4b367f5859e6.md`, `verdict=clean rounds=9`, 33 files hash-pinned |
| `review_gate.py check` | clean -- `review_gate: OK (0 code files, clean, hashes match ...)` |
| Rounds | 9. Round 1 reviewed the whole diff; each round after it reviewed the round before. Round 2 found the restart regression, round 3 the outbound flap, round 4 the inbound rail the round-3 fix left open, round 5 the two producers rounds 3 and 4 both left unread, round 6 that round 5's enumeration was accurate and the argument on it unsound, round 7 a PRODUCT defect (`Session.connectionEstablished`'s seal refusal called `closeConnQuietly` on a connection it does not own, so `acceptOrReject` buffered an already-closed socket for the next `runOnce` cycle and `acceptPendingConnection` closed it a second time). Round 8 found the product clean and could not record clean, because the closure commit's FILE SET was unresolvable: `peer.go` carries two specs. Round 9 re-verified on the current tree after Thomas settled the one-commit plan; no new product defect |
| Reviewer lenses used | round 9: re-verify round 8's five core conclusions on the current tree, re-hash the file list, confirm B-2. Round 8: whether the commit's file set is executable. Round 7: does a comment's safety claim survive a grep for its own counter-example, who OWNS a connection at each refusal, does the new test discriminate. Round 1: insertion point, budget arithmetic, dead-code deletion, FSM state coverage. Round 2: RFC 4724 interaction, test-claim truth, dead branches, doc anchors. Round 3: multi-peer stop ordering, RFC section-number accuracy, claim-versus-measurement, log-level symmetry. Round 4: the OTHER rail into a session (accept, not dial), peer creation during the stop, doc-versus-code agreement, checklist claims against the tree. Round 5: enumerate the producers instead of the rails, gate altitude (producer versus caller), whether a comment's safety claim is true of the diff it sits in, closure-time tracked-file consequences. Round 6: is the PROPERTY the right one, does a gate survive its own neighbours (a flag one caller clears), which population a proof is silent about, log level of a designed refusal |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The fix defeated graceful restart on the reboot path: `writeGRMarker()` then `r.Stop()` sent a Cease that RFC 4724 Section 5 makes the peer act on by deleting the routes the marker asks it to keep | `cmd/ze/hub/infra_setup.go`, `cmd/ze/hub/service_ssh.go` | `Reactor.StopForRestart` plus a single `stopForRestart` that pairs the marker write with the silent stop. `TestReactorStopForRestartSendsNoNotification` |
| 2 | BLOCKER | `signal-stop-cease.ci` was labelled an ordering guard. It is a feature guard: the reverted ordering passes it 20/20 | `test/reload/signal-stop-cease.ci`, the spec's TDD table | The claim corrected in both places, and a real ordering guard added: `TestReactorStopNotifiesWhileItsContextIsStillLive`, deterministic over `-count=50` |
| 3 | ISSUE | AC-2's "a failed send is logged" pointed at a branch that cannot run: `Session.teardown` returns nil on every path | `internal/component/bgp/reactor/peer.go` | The branch deleted; AC-2 corrected to name `logNotifyErr` (`session.go`), which is where the failure is actually reported |
| 4 | ISSUE | Six FSM docs named the deleted `Session.Close` in their ManualStop rows and source anchors | `docs/architecture/behavior/fsm-*.md` | Rewritten to `Session.Stop` / `Session.Teardown`. `fsm-idle.md`'s claim that `CloseWithNotification` fires ManualStop corrected to Event 23 |
| 5 | NOTE | An abandoned notify goroutine holds `s.writeMu` for up to 10s and can block `closeConn` while it holds `s.mu` | `session_connection.go`, `session_write.go` | Recorded in Known Limitations with the three budgets. Exit stays bounded; the margin is zero |
| 6 | NOTE | The GR-families finding was written as a possible conformance defect | A-2, Known Limitations | Corrected: RFC 4724 Section 4 sanctions the empty AFI/SAFI list. The defect is the config surface |
| 7 | BLOCKER | Round 2's own fix opened a shutdown flap. `Reactor.stop` notifies every peer BEFORE it cancels, so for the whole 1s budget each `p.ctx` is still live -- and `ShutdownNotify`'s teardown raises `ErrTeardown`, the one error `Peer.run` answers by resetting the delay and re-dialing with NO wait. A peer already told the engine was leaving re-dialed the still-open listener, could reach Established, and was then killed by the cancel with no NOTIFICATION on it: a second `RFC4271-8.2.2-18` miss on the same stop. No existing test drove `Stop` with more than one peer, which is why it shipped | `internal/component/bgp/reactor/reactor.go`, `peer_run.go`, `peer.go` | `Peer.stopping` (`atomic.Bool`), set by `Reactor.stop` on every peer before the notify and read at the top of `Peer.run`'s loop; cleared by `StartWithContext`. `TestReactorStopDoesNotRedialAPeerItHasAlreadyNotified` (two peers, one slow to write) |
| 8 | ISSUE | The replacement FSM text was cited as RFC 4724 **Section 8** in seven places. Section 8 is Acknowledgments; the FSM text is Section 5, "Changes to BGP Finite State Machine" (`rfc/full/rfc4724.txt`, heading at line 472, replacement text at 567-604) | `reactor.go`, `cmd/ze/hub/infra_setup.go`, `cmd/ze/hub/service_ssh.go`, `cmd/ze/hub/ssh_infra.go`, `internal/component/config/infra/hook.go`, `shutdown_notify_test.go`, this spec | All seven corrected to Section 5 |
| 9 | ISSUE | Six comments and the spec claimed "a bare FIN preserves the peer's forwarding state". It does not today: the retain branch needs the GR Capability received "with one or more AFIs/SAFIs" (lines 587-604) and Ze sends an empty list (A-2, FRR: `Remote GR Mode: NotApplicable`, withdraws in 0.1s). The unconditional half -- any NOTIFICATION deletes the routes (lines 569-585) -- is correct | the same seven sites | Reworded to the real reason, decision unchanged: a bare FIN is the only end a peer can EVER retain across, so a Cease forecloses retention permanently while silence leaves it one config fix away |
| 10 | ISSUE | `logNotifyErr` logged the FAILED send at Debug while the success path logs at Warn, so an operator at the default level saw every NOTIFICATION that went out and none that did not. Its own comment claimed the opposite | `internal/component/bgp/reactor/session.go` | Raised to Warn, comment corrected, and `TestLogNotifyErrLogsCodeName` now asserts `level=WARN` rather than only the rendering |
| 11 | BLOCKER | Round 3 closed one rail of two. `Peer.stopping` is read at the top of `Peer.run`, and NOTHING on the accept path reads it: `Listener.acceptLoop` reaches `Peer.AcceptConnection` through `acceptOrReject` on its own goroutine, and `tryCreateDynamicPeer` builds a peer the stop's snapshot predates. The listeners were shut in `Reactor.cleanup` Phase 1, which `monitor()` reaches only after the cancel, so for the whole 1s budget the daemon was still accepting connections and could put a fresh session in OpenSent for the cancel to kill in silence: the same `RFC4271-8.2.2-18` miss round 3 fixed outbound | `internal/component/bgp/reactor/reactor.go`, `reactor_dynamic.go` | `Reactor.stop` shuts `r.listener` and every `r.listeners` entry in the SAME hold of `r.mu` that marks the peers, so `tryCreateDynamicPeer` (same lock) either predates the seal or is refused by it. Verified at the producers that this cannot disturb a session: `Listener.Stop` and `Listener.cleanup` close `l.listener` only, and a `Listener` holds no reference to a conn it has handed on (`listener.go`). `TestReactorStopAcceptsNoInboundSessionWhileItNotifies`, `TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed` |
| 12 | ISSUE | The Documentation Update Checklist row 9 claimed `docs/features/rfc-status.md`'s RFC 4271 row was written in round 1. It was not: the file was unmodified in the tree, and its Implemented coverage cell recorded the Section 8.2.2 Event 10 hold-timer list with no mention of the ManualStop Cease | `docs/features/rfc-status.md`, this spec | The row written, in the Implemented coverage cell only, so the Remaining cell's spelled gap count is untouched (`check_gap_count_agreement`). The checklist claim corrected to say what round 4 found |
| 14 | BLOCKER | The listener rail was open two ways after round 4. `Reactor.handleAddrAddedPayload` takes `r.mu` and calls `startListenerForAddressPort` with no seal check, and the EventBus subscriptions are released in `Reactor.cleanup`, which `monitor()` reaches only after the cancel -- so a netlink address-added event inside the notify budget started a fresh `Listener` on the still-live `r.ctx` and the daemon accepted again. Companion on the same rail: `Reactor.StartPeers` calls `Peer.StartWithContext`, which CLEARED `Peer.stopping`, reachable through `coord.OnPostStartup` if SIGTERM lands during plugin startup. `Reactor.stop`'s own comment asserted both rails were shut | `internal/component/bgp/reactor/reactor.go`, `reactor_iface.go`, `peer.go` | The seal read inside `startListenerForAddressPort`, which all three callers reach with `r.mu` held. `Peer.StartWithContext` no longer clears the flag; `Reactor.StartWithContext` clears it for every peer under the same `r.mu` that `stop` sets it under. The comment rewritten to the real producer set. `TestReactorStopStartsNoListenerForAnAddressAddedWhileItNotifies`, `TestPeerStartWithContextDoesNotLiftTheStopsSeal` |
| 15 | BLOCKER | The third rail: a `runOnce` already in flight. `Peer.run` reads `p.stopping` at the loop top only. (a) With `p.session` already published, `ShutdownNotify` -> `Session.teardown` sends nothing because `s.conn == nil` and only sets `tearingDown`; `Session.Connect` then dials anyway -- unlike `Session.Accept` it had NO `tearingDown` check -- and `connectionEstablished` reached OpenSent and sent the OPEN. (b) With the snapshot predating the publish, no teardown happened at all and the session could reach Established inside the budget. Both end at `cancel()` with a bare FIN after an OPEN: the RFC 4271 Section 8.2.2 ManualStop miss on the outbound twin of AC-10 | `internal/component/bgp/reactor/peer_run.go`, `session_connection.go` | (b) closed by reading `Peer.stopping` inside the `p.mu` hold that publishes `p.session`, which is the lock `ShutdownNotify` reads that field under. (a) closed by reading `s.tearingDown` inside the `s.mu` hold that publishes `s.conn`, which is the lock `teardown` reads that field under -- so one of the two always sees the other. `TestRunOncePublishesNoSessionAfterTheStopHasMarkedThePeer`, `TestSessionConnectSendsNoOpenOnceTeardownHasRun` |
| 16 | BLOCKER | Round 5's syscall enumeration was accurate and the ARGUMENT built on it was unsound. The property it proves is "no new socket opens", and what must hold is "no conn becomes a session after the seal": the two differ on exactly the population that broke this twice, a conn that ALREADY EXISTED when the seal was taken. Counterexample, an EXISTING CONFIGURED peer on the accept rail: `acceptOrReject` (`reactor_connection.go`) reads no seal at all, `findPeerByAddr` succeeds so round 4's `tryCreateDynamicPeer` gate is never consulted, and `Session.Accept` cleared `s.tearingDown` between its own entry check and the `connectionEstablished` gate. `Accept` loads false, `teardown` stores true and reads a nil `s.conn` so writes nothing, `Accept` stores false, `connectionEstablished` loads false, publishes `s.conn` and sends the OPEN, and the cancel closes it bare: the same RFC 4271 Section 8.2.2 ManualStop miss as findings 7, 11, 14 and 15 | `internal/component/bgp/reactor/session_connection.go`, `peer.go`, `reactor.go` | Fixed at the frame rather than on a fifth rail. `s.tearingDown` becomes ONE-WAY: `Session.seal` is its only writer and only ever writes true, and `Accept`'s clear is deleted. `Reactor.stop` seals every peer's live session (`Peer.sealSession`) on BOTH stops, so the seal no longer arrives as a side effect of the notify and `StopForRestart` seals too. The argument is now checkable by grep over the two publish sites. `TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer` |
| 17 | BLOCKER | A false safety claim in the diff, and it is what hid finding 16. `Reactor.stop`'s comment said an accepted conn becomes a session through `Peer.AcceptConnection`, "which needs a published `p.session` (there is none after the seal)". `p.session` has two non-test writers, the publish in `runOnce` and the defer's nil, and the defer runs only when `runOnce` RETURNS -- for a Connect/Active peer, after `Session.Run` unwinds, well inside the 1s budget. The seal stops new publishes; it does not unpublish | `internal/component/bgp/reactor/reactor.go`, `peer.go`, `peer_run.go` | The comment rewritten to the property that holds and the two writes that carry it, with the listener shutdown and the `r.stopping` reads named as what they are: less work on the way out, not what makes the stop final. The same false credit removed from the `Peer.stopping` field comment and from `Peer.run`'s loop-top comment |
| 18 | ISSUE | `handleAddrAddedPayload` logged round 5's designed refusal at Error, so every shutdown that met a netlink address event told the operator the listener had FAILED | `internal/component/bgp/reactor/reactor_iface.go` | Branch on `errors.Is(startErr, errReactorStopping)` and log at Debug |
| 19 | ISSUE | The THIRD false safety claim in this diff, and the same shape as findings 17 and 11. `Reactor.stop`'s comment said the sessions are sealed outside the `r.mu` hold "so no Peer lock is ever taken under `r.mu`". `r.mu` nests `p.mu` five times: `peer.SettingsSnapshot` and `peer.currentSession` (both `p.mu.RLock`) under `r.mu.RLock` at two sites in `reactor_api.go`, `peer.Stop` (`p.mu.Lock`) under `r.mu.Lock` at two more, and `peerGRCapable`'s `p.mu.Lock` under `r.mu.RLock` in `reactor.go`. The conclusion survives -- the order is uniformly `r.mu` then `p.mu` -- but the stated reason INVERTS the guidance: a reader who believed `r.mu` never nests `p.mu` would take `p.mu` then `r.mu` for free, and that deadlocks against all five | `internal/component/bgp/reactor/reactor.go` | The comment states the real invariant: `r.mu` before `p.mu` everywhere, nothing takes `r.mu` under `p.mu` (a `Peer` method needing the reactor copies `p.reactor` out and releases `p.mu` first), and sealing outside the hold adds no nesting either way |
| 20 | ISSUE | The refusal closed a connection it does not own. `Session.connectionEstablished`'s seal branch called `closeConnQuietly(conn)`, which is right for `Connect` -- `runOnce` never sees that socket -- and wrong for both accept rails. `acceptOrReject` then buffered an already-closed socket on `ErrSessionTearingDown` (`reactor_connection.go`) and the next `runOnce` cycle accepted a dead conn and paid a backoff; `acceptPendingConnection` closed it a second time. `Session.Accept`'s own new comment stated the property the branch broke | `internal/component/bgp/reactor/session_connection.go` | The close moved to `Session.Connect`, the one rail that owns what it hands in, gated on `ErrSessionTearingDown`. `connectionEstablished` returns the error and keeps its hands off the conn, and both comments now say so. `TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn`, `TestSealedSessionConnectClosesTheConnItDialed` |
| 21 | NOTE | "Collision resolution is untouched" was imprecise: the round-5 gate adds a refusal `AcceptWithOpen` never had, because it has no entry check of its own | this spec, round 6's State section | Corrected in place. RFC 4271 Section 6.8 resolution between two live connections is untouched; a session already sealed by a teardown or a stop now refuses a collision accept, which is intended and whose conn the caller closes |
| 22 | NOTE | `startListenerForAddressPort`'s comment said "three call sites". There are three calling FUNCTIONS and four sites: `AddPeer` reaches it twice | `internal/component/bgp/reactor/reactor.go` | Reworded to four sites, naming `AddPeer`'s two branches (a new listener, and the restart of one whose MD5 peer set changed) |
| 13 | ISSUE | Five FSM docs credited `Session.Stop` with a Cease NOTIFICATION. `Session.Stop` is `timers.StopAll()` plus `fsm.Event(EventManualStop)` (`session.go`), it sends nothing, and `gopls references` gives it ONE caller, `internal/test/cli/cmd_replay.go`. `fsm-active.md` said both things: the table row credited the Cease and the prose below it said Stop sends nothing | `docs/architecture/behavior/fsm-active.md`, `fsm-connect.md`, `fsm-established.md`, `fsm-open-confirm.md`, `fsm-open-sent.md` | Every `EventManualStop` row's wire-side-effect cell now reads "Cease NOTIFICATION from `Session.Teardown` when a conn exists; `Session.Stop` sends nothing". `fsm-connect.md`'s prose gained the same clause and the `session.go — Stop` anchor |

## State at 2026-08-11 (round 7 implemented, NOT closed)

Round 7's independent review returned **0 BLOCKER** and judged the frame closed.
Two ISSUEs and two NOTEs were fixed; nothing else was touched.

**Finding 19 is the third false safety claim in this diff, and that is the
pattern rather than the typo.** Round 6 found one (finding 17), round 7 found
this one, and each had a real defect under it. `Reactor.stop`'s comment said "no
Peer lock is ever taken under `r.mu`". Five sites take one, and every one of them
is `r.mu` THEN `p.mu`:

| Site | Nested call | Outer lock |
|------|-------------|-----------|
| `reactor_api.go`, the reload snapshot | `peer.SettingsSnapshot`, `peer.currentSession` (`p.mu.RLock`) | `r.mu.RLock` |
| `reactor_api.go`, the reload cost estimate | the same two | `a.r.mu.RLock` |
| `reactor_api.go`, the remove-peer journal step | `peer.Stop` (`p.mu.Lock`) | `r.mu.Lock` |
| `reactor_api.go`, the add-peer rollback step | `peer.Stop` | `r.mu.Lock` |
| `reactor.go`, `peerGRCapable` | `p.mu.Lock` | `r.mu.RLock` |

Nothing takes `r.mu` under `p.mu`: a `Peer` method that needs the reactor copies
`p.reactor` out and releases `p.mu` before it uses it (`peer.go`,
`getPluginCapabilities`, `getPluginFamilies`, `validateOpen`; `peer_run.go`). So
the deadlock conclusion survives and the reason inverted it: a reader who
believed `r.mu` never nests `p.mu` would treat `p.mu` then `r.mu` as free.

**Finding 20 is one place with two findings in it, which is why it was in scope
rather than a follow-up.** `connectionEstablished`'s refusal closed the
connection. Ownership, verified at each of the three callers: `runOnce` never
sees the socket `Connect` dialed (`peer_run.go`), so `Connect` must close it;
`acceptOrReject` BUFFERS the conn on `ErrSessionTearingDown` for a passive peer
and hands it to the next cycle (`reactor_connection.go`); `acceptPendingConnection`
closes it itself. The close moved to `Connect`, and `Session.Accept`'s comment --
"Refusing is not losing the connection" -- is true again at both of its answers.

**Evidence.** `TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn` is
RED on the round-6 tree at `write tcp 127.0.0.1:39615->127.0.0.1:53684: use of
closed network connection`, in its `accept with open` case: `AcceptWithOpen` has
no entry check, so a sealed session reaches the changed branch on every call and
the interleave `Session.Accept` needs is not required. The `accept` case is
refused one step earlier and does NOT discriminate; it is there because
`acceptOrReject` takes that rail in production. `TestSealedSessionConnectClosesTheConnItDialed`
does not discriminate either, by construction: it guards the rail the fix must
not regress. `make ze-test-pkg PKG=./internal/component/bgp/reactor` is green
under `-race`, and `session_connection.go` was verified byte-identical after the
revert-and-restore.

**Both new tests are appended at the END of `shutdown_notify_test.go` and the
import block is unchanged**, so the lines `rfc/requirements/rfc4271.md` cites for
`RFC4271-8.2.2-18` are still 174 and 207 (B-2). That is why
`TestSealedSessionConnectClosesTheConnItDialed` matches `io.EOF` by its message:
importing `io` would have moved them.

## State at 2026-08-11 (round 6 implemented, NOT closed)

Round 6 stops gating rails and stops enumerating socket producers. It fixes the
PROPERTY.

**What must hold.** Not "no new socket opens", which is what round 5 proved. "No
conn becomes a session after the seal". The two differ on exactly the population
that broke this twice: a connection that ALREADY EXISTED when the seal was
taken. Round 5's own residual, "a dial already in flight", is one member of that
population, and the accept rail against an EXISTING CONFIGURED peer is another
that round 5 missed entirely.

**The set that closes it is two writes, one writer each, and grep finds both.**

| Publish site | File | Gate |
|--------------|------|------|
| `p.session` | `Peer.runOnce` (`peer_run.go`) | `Peer.stopping`, read inside the `p.mu` hold that publishes |
| `s.conn` | `Session.connectionEstablished` (`session_connection.go`) | `s.tearingDown`, read inside the `s.mu` hold that publishes |

```
$ grep -rn '\.session = \|s\.conn = ' internal/component/bgp/reactor/*.go | grep -v _test.go
peer_run.go:275:        p.session = session
peer_run.go:314:                p.session = nil
session_connection.go:376:      s.conn = conn
session_connection.go:554:              s.conn = nil
```

**Two things made the second gate a gate rather than a decoration.** `s.tearingDown`
is now ONE-WAY -- `Session.seal` is its only writer and only ever writes true --
and `Reactor.stop` seals every peer's live session itself (`Peer.sealSession`),
on both stops. Before, the seal arrived only as a side effect of `ShutdownNotify`
on the notify path, so `StopForRestart` sealed nothing at all, and `Session.Accept`
cleared the flag between its own entry check and `connectionEstablished`'s read.

**Why `Accept` cleared it, verified at the producer before it was deleted.** Its
comment said "This allows the session to be reused after a teardown", and that is
not what the store did. `Accept` refuses a sealed session at its ENTRY check, so a
session torn down before the call never reaches the store; and a session torn down
after it is one the store then unseals. The store was therefore a no-op on every
ordering except the one the flag exists to refuse. Reuse runs on a different route
and still does: `acceptOrReject` buffers the connection on `ErrSessionTearingDown`
for a passive peer (`reactor_connection.go`, the only production handler of that
error), and `runOnce` offers it to the NEXT cycle's session, which is a new
`Session` value (`peer_run.go`, `NewSession` per cycle). The Session value itself
is never reused.

→ Constraint (corrected 2026-08-11, round 7): **"Collision resolution is
untouched" was imprecise, and the accurate statement is narrower.** RFC 4271
Section 6.8 resolution between two live connections IS untouched, for the reason
the round-6 wording gave: `CloseWithNotification` sets no seal
(`session_connection.go`), so the session `handlePendingCollision` closes and
then re-accepts through `AcceptWithOpen` carries no seal when the re-accept
arrives, and `AcceptWithOpen` never cleared one either. What the round-5 gate DID
add is a refusal `AcceptWithOpen` never had: it has no entry check of its own, so
a session sealed by `Session.teardown` or by `Reactor.stop` now refuses a
collision accept at `connectionEstablished`. That is benign and intended -- the
session it would land on is one the operator or the stop has already ended -- and
the caller keeps the connection and closes it (`acceptPendingConnection`,
`reactor_connection.go`).

**Evidence.** `TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer` drives the
accept rail against a configured peer for both stops, asserts the rail really
reaches the peer's published session BEFORE anything else, and is RED on the
round-5 tree at `Expected nil, but got: &net.TCPConn{...}` -- a conn published on
a live session after the seal. `make ze-test-pkg PKG=./internal/component/bgp/reactor`
is green under `-race`, all six earlier shutdown tests included. The two edited
sources were verified byte-identical after the revert-and-restore.

**What the new test does NOT discriminate.** Measured: with `Peer.sealSession`
restored and `Accept`'s clear put back, it passes. `Accept`'s entry check refuses
first, so no deterministic ordering reaches the store -- the clear is observable
only in the interleave, which is why it survived five rounds. It is deleted on the
producer argument above, not on a test. A test that drove that interleave would
need a positive observation of a goroutine parked on `s.mu`, and Go offers none.

## State at 2026-08-11 (round 5 implemented, NOT closed)

Round 5 stops gating rails one at a time and closes the producer SET.

**Why the set is closed, not just longer.** A "new socket" in this package comes
from exactly two calls: `s.dialer.DialContext` in `Session.Connect`
(`session_connection.go`, the only dial) and `l.listenerFactory.Listen` in
`Listener.StartWithContext` (`listener.go`, the only listen). Everything else
hands an existing conn along. `gopls references` gives `Session.Connect` ONE
production caller, `Peer.runOnce` (`peer_run.go`); `Listener.StartWithContext`
has two, `Reactor.StartWithContext` and `startListenerForAddressPort`; and an
accepted conn becomes a session only through `Peer.AcceptConnection` (needs a
published `p.session`) or `tryCreateDynamicPeer` (sealed in round 4).

| Producer | Gate added in round 5 | Why here and not at the caller |
|----------|----------------------|--------------------------------|
| `startListenerForAddressPort` (`reactor.go`) | reads `r.stopping`, returns `errReactorStopping` | All three callers hold `r.mu`, and one of them is the netlink event (`handleAddrAddedPayload`, `reactor_iface.go`) whose subscription `Reactor.cleanup` releases only AFTER the cancel. Gating callers is what left it open twice |
| `Peer.StartWithContext` (`peer.go`) | no longer clears `Peer.stopping`; `Reactor.StartWithContext` clears it for every peer under the `r.mu` that `stop` sets it under | The method holds no lock that could order the clear against a stop, and `Reactor.StartPeers` reaches it from `coord.OnPostStartup` with `r.mu` released |
| `Peer.runOnce` (`peer_run.go`) | reads `Peer.stopping` INSIDE the `p.mu` hold that publishes `p.session` | That is the field `ShutdownNotify` reads under the same lock, so "the notify covered this session, or this session was never published" becomes a guarantee. `run`'s loop top can be passed before the mark, and the dial that follows takes a TCP handshake |
| `Session.connectionEstablished` (`session_connection.go`) | reads `s.tearingDown` INSIDE the `s.mu` hold that publishes `s.conn` | `teardown` sets `tearingDown` and then reads `s.conn` under that same lock, so either the Cease goes out on the conn or the conn is refused. `Session.Accept` had this check; `Connect` and `AcceptWithOpen` reach the wire through the same funnel |

**The argument, in one line.** After the seal no new `p.session` is published, so
every session that exists predates the seal, so `ShutdownNotify` covered it and
`s.tearingDown` is set on it; the only residual is a dial already in flight, and
`connectionEstablished` refuses that one under the lock `teardown` reads.

**Evidence.** Each of the four tests in the TDD table was measured RED against
the round-4 tree by reverting its own fix, and the four source files were
verified byte-identical afterwards. `make ze-test-pkg
PKG=./internal/component/bgp/reactor` is green under `-race`, and
`TestReactorStopAcceptsNoInboundSessionWhileItNotifies` and
`TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed` both still pass.

**`Reactor.stop`'s comment was a false safety claim in the diff** and is
rewritten: it asserted both rails were shut while two producers read no seal.

## State at 2026-08-11 (round 4 implemented, NOT closed)

Round 4 closes the INBOUND rail. `Reactor.stop` marks every peer stopping, shuts
`r.listener` and every `r.listeners` entry, and sets `Reactor.stopping`, all in
ONE hold of `r.mu`; `tryCreateDynamicPeer` and `AddPeer` take that same lock, so
a peer either predates the seal and is marked with the rest, or is refused after
it. Stopping a listener cannot disturb a session: `Listener.Stop` and
`Listener.cleanup` close `l.listener`, the LISTENING socket, and a `Listener`
keeps no reference to a conn it has handed to the handler (`listener.go`). That
is why the listeners can go here while the cancel still has to come last.

Evidence: `TestReactorStopAcceptsNoInboundSessionWhileItNotifies` dials the
reactor's own listen address inside the notify budget, asserts the window really
opened before anything else, and is RED on the round-3 tree at
`Should be zero, but was 1` -- a dynamic peer built from that dial.
`TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed` covers the one connection
the listener close cannot reach, the one a `Listener` had already accepted.
`make ze-test-pkg PKG=./internal/component/bgp/reactor` is green under `-race`.

## State at 2026-08-10 (session paused, NOT closed)

Round 2 is implemented and the whole round-1 review is answered. Not reviewed
since, so not closed.

**The fix.** `Reactor.Stop` sends the Cease before it cancels; `StopForRestart`
cancels in silence. Both run over one `stop(notify bool)`. The marker write and
the silent stop are ONE closure, `stopForRestart` in `cmd/ze/hub/infra_setup.go`,
called by all three restart callbacks — so no call site can pick the wrong stop,
which is exactly how round 1 shipped a regression.

**Why silence is right on restart, read from the RFC.** RFC 4724 Section 5,
"Changes to BGP Finite State Machine", replaces RFC 4271's FSM text, and in the
replacement a peer receiving ANY NOTIFICATION deletes the routes, with no
condition on it. Retention is the conditional branch: `TcpConnectionFails` AND
the GR Capability received with one or more AFI/SAFI tuples. Ze sends an empty
tuple list, so a peer does not retain today (A-2). Silence is right anyway, and
the subcode question is moot: no NOTIFICATION of any subcode can be retained
across, so a Cease forecloses retention for every peer permanently while silence
leaves it one config fix away. Round 1 wrote the GR marker and then sent a
NOTIFICATION that told every peer to discard what the marker was asking for.

**Evidence.** `TestReactorStopNotifiesWhileItsContextIsStillLive` reads
`r.ctx.Err()` from inside the send: 50/50 green as shipped, 50/50 RED with the
notify moved after `cancel()`. It replaces a far-end test that detected the same
regression only 21 times in 50. `TestReactorStopForRestartSendsNoNotification`
pins both halves. FRR reports `lastNotificationReason=Cease/Administrative
Shutdown` on shutdown.

**What blocks closure**

| # | Owed |
|---|------|
| 1 | No independent Review Gate pass over round 2, no artifact |
| 2 | AC-2's "a failed send is logged" was resolved by DELETING a branch that could never run (`Session.teardown` returns nil on every path) and pointing the AC at `logNotifyErr`. A reviewer should confirm that is the right call rather than making the failure observable where the AC first said |
| 3 | `test/reload/signal-stop.ci`'s stray RFC tag was removed with Thomas's `rfc-test-change-approved` marker of 2026-08-10; the sibling `signal-stop-cease.ci` carries both the tag and the assertion |

**One item recorded and NOT this spec's:** `audit-test-relaxation.py` flags
`session_rfc7606_diagnostics_test.go` as WEAKENED with no approval token. That
edit belongs to the forward-rail spec — a `bytes.Buffer` to `syncBuffer` race fix
in `captureSessionLog`, no assertion weakened — and it blocks THAT spec, not this
one.

---

## Implementation Summary

### What Was Implemented

- **`Reactor.Stop` sends before it cancels.** `Reactor.stop(notify bool)`
  (`reactor.go`) is the one body; `Stop` passes true and `StopForRestart`
  passes false. `notifyPeersShutdown` fans out to `Peer.ShutdownNotify`
  (`peer.go`), which reaches `Session.Teardown` (`session_connection.go`), and
  the wait is bounded by `shutdownNotifyBudget` (1s) through
  `syncutil.WaitGroupWait`.
- **`Session.Close()` DELETED.** It built exactly the Cease message and had no
  reachable production caller. `internal/test/cli/cmd_replay.go` moved to
  `Session.Stop`. `Peer.cleanup`'s `if p.session != nil` teardown guard went
  with it (`peer_run.go`).
- **A restart sends nothing.** `Reactor.StopForRestart`, reached through
  `infra.ReactorHandle.StopForRestart` (`internal/component/config/infra/hook.go`)
  and the single `stopForRestart` that pairs the RFC 4724 marker write with the
  silent stop (`cmd/ze/hub/infra_setup.go`, `service_ssh.go`, `ssh_infra.go`).
  `daemon shutdown` keeps `Stop`; `daemon restart` and `daemon reboot` take the
  silent one.
- **The seal.** `Reactor.stop` marks every peer (`Peer.stopping`), shuts
  `r.listener` and every `r.listeners` entry, and sets `Reactor.stopping`, all
  in ONE hold of `r.mu`; then it seals every peer's live session
  (`Peer.sealSession` -> `Session.seal`). `Session.tearingDown` is ONE-WAY:
  `Session.seal` is its only writer and only ever writes true.
- **The two publish sites read the seal inside the lock that publishes.**
  `Peer.runOnce` reads `Peer.stopping` inside the `p.mu` hold that publishes
  `p.session`; `Session.connectionEstablished` reads `s.tearingDown` inside the
  `s.mu` hold that publishes `s.conn`. Those are the only two writes by which a
  conn becomes a session, so the property is checkable by grep.
- **`RFC4271-8.2.2-18` extracted at MUST** in `rfc/short/rfc4271.md`, covering
  all three ManualStop action lists (OpenSent, OpenConfirm, Established), with a
  positive and a negative tagged test.

### Bugs Found/Fixed

- The reboot path sent a Cease one line after the graceful-restart marker was
  written, telling every peer to drop exactly the routes the marker asks them to
  keep. `TestReactorStopForRestartSendsNoNotification`.
- The fix's own first version opened a shutdown flap: `ErrTeardown` is the one
  error `Peer.run` answers by re-dialing with no wait, so a peer already told
  the engine was leaving re-dialed the still-open listener.
  `TestReactorStopDoesNotRedialAPeerItHasAlreadyNotified`.
- The inbound rail stayed open for the whole notify budget (listeners shut in
  `Reactor.cleanup`, which `monitor()` reaches only after the cancel).
  `TestReactorStopAcceptsNoInboundSessionWhileItNotifies`,
  `TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed`.
- A netlink address-added event inside the budget started a fresh `Listener` on
  the still-live `r.ctx`, and `Peer.StartWithContext` CLEARED `Peer.stopping`.
  `TestReactorStopStartsNoListenerForAnAddressAddedWhileItNotifies`,
  `TestPeerStartWithContextDoesNotLiftTheStopsSeal`.
- A `runOnce` already in flight dialed anyway and sent its OPEN, because
  `Session.Connect` had no `tearingDown` check.
  `TestRunOncePublishesNoSessionAfterTheStopHasMarkedThePeer`,
  `TestSessionConnectSendsNoOpenOnceTeardownHasRun`.
- `Session.Accept` CLEARED `s.tearingDown` between its own entry check and the
  `connectionEstablished` gate, which neutralised the gate for an existing
  configured peer. `TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer`.
- The seal refusal closed a connection it does not own, so `acceptOrReject`
  buffered an already-closed socket for the next cycle and
  `acceptPendingConnection` closed it a second time.
  `TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn`,
  `TestSealedSessionConnectClosesTheConnItDialed`.
- `logNotifyErr` logged the FAILED send at Debug while the success path logs at
  Warn. Raised to Warn; `TestLogNotifyErrLogsCodeName` asserts `level=WARN`.
- `handleAddrAddedPayload` logged the designed refusal at Error, putting a
  designed refusal in front of the operator as a failure on every shutdown that
  met a netlink event. Now branches on `errors.Is(startErr, errReactorStopping)`
  and logs at Debug.

### Documentation Updates

- `docs/features/rfc-status.md` - the RFC 4271 row's Implemented coverage cell
  records the Section 8.2.2 ManualStop Cease. The Remaining cell's spelled gap
  count is untouched (`check_gap_count_agreement`).
- `docs/architecture/behavior/fsm-active.md`, `fsm-connect.md`,
  `fsm-established.md`, `fsm-open-confirm.md`, `fsm-open-sent.md`,
  `fsm-idle.md` - the `EventManualStop` rows and their source anchors named the
  deleted `Session.Close`, and five of them credited `Session.Stop` with a Cease
  it does not send. `fsm-idle.md`'s claim that `CloseWithNotification` fires
  ManualStop corrected to Event 23 (`OpenCollisionDump`).
- `rfc/short/rfc4271.md` - `RFC4271-8.2.2-18`.
- `rfc/audit/rfc7606.json` - five verdicts SHIFTED by the `session_test.go`
  edit, re-stamped with `make ze-rfc-reseal`. Byte-identical units, nothing
  re-judged.
- `ai/RFC-REQUIREMENTS.md` is GENERATED and is NOT in this commit:
  `make ze-rfc-index` was not run, per the owner's instruction for this phase.
- `make ze-doc-test` NOT run in this phase: a QEMU run holds the tree and the
  owner directed that no suite start.

### Deviations from Plan

- Step 2 planned the wire assertion inside `test/reload/signal-stop.ci`. It
  lives in the new sibling `test/reload/signal-stop-cease.ci` instead (B-1).
- Step 3 planned the send from `Reactor.cleanup()` Phase 1. It is in
  `Reactor.stop`, before the cancel: everything downstream of the cancel is
  already nil by construction.
- The spec did not plan a `Stop` / `StopForRestart` split, a seal, or a
  one-way `tearingDown`. Rounds 2, 4, 5 and 6 each added one, and each was a
  defect the round before had shipped.
- A-2 is BROKEN: no peer retains Ze's routes across a bare FIN today, because
  Ze's GR capability carries no per-AFI/SAFI tuple. The fix stands and its
  reason changed from traffic to diagnosability and conformance.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2: a GR-capable peer was assumed to retain Ze's routes as stale across an admin shutdown, which made this an operator-visible traffic defect | FRR 10.3.1 withdrew `10.10.0.0/24` 0.1s after the bare-FIN shutdown, reported `Notifications: 0 0`, and read `Remote GR Mode: NotApplicable`. Ze advertises the GR capability with a Restart Time and no `<AFI, SAFI>` tuple, so no peer takes the retain branch | AC-7, in the interop lab against a real FRR | The Task section's stale-route claim is downgraded in place and must not be repeated. The fix stands on diagnosability and RFC 4271 Section 8.2.2 conformance. The empty tuple list is RFC 4724 Section 4's own recommendation and is NOT a compliance defect; the CONFIG surface is, and it has its own spec |
| approach | Round 1 put all three restart callbacks on `Reactor.Stop`, one line after `writeGRMarker()` | A planned reboot then told every peer to drop exactly the routes the marker had just been written to keep. RFC 4724 Section 5's replacement FSM text deletes routes on any NOTIFICATION, unconditionally | Round 2 review | `Reactor.StopForRestart`, and a single `stopForRestart` that pairs the marker write with the silent stop so no call site can pick the wrong one |
| approach | Rounds 3, 4 and 5 each gated the rail in front of them: outbound, then inbound-dynamic, then the listener and peer-start producers | Enumerating socket PRODUCERS proves "no new socket opens". What must hold is "no conn becomes a SESSION after the seal", and the two differ on the population that broke this twice: a conn that already existed when the seal was taken | Round 6 review | Fixed at the frame. A conn becomes a session at exactly two writes, one writer each; both read the seal inside the lock that publishes, and `Session.seal` is `tearingDown`'s only writer |
| escalation | Three false safety claims shipped in comments across rounds 3, 6 and 7, and each had a real defect under it | `Reactor.stop`'s comment claimed an accepted conn needs a published `p.session` "(there is none after the seal)" -- the seal stops new publishes and never unpublishes. It then claimed "no Peer lock is ever taken under `r.mu`" -- five sites take one | Each found by grepping the comment for its own counter-example | Every comment rewritten to the property that holds and the writes that carry it. The pattern is the lesson, not the typo: a safety claim in a comment must survive a grep for its own counter-example |
| approach | Round 5's seal refusal called `closeConnQuietly(conn)` in `Session.connectionEstablished` | That is right for `Connect`, which owns what it dialed, and wrong for both accept rails: `acceptOrReject` BUFFERS the conn for the next cycle and `acceptPendingConnection` closes it itself | Round 7 review; `TestSealedSessionRefusesAnAcceptWithoutClosingTheCallersConn` is red at `use of closed network connection` | The close moved to `Session.Connect`, gated on `ErrSessionTearingDown`. `connectionEstablished` returns the error and keeps its hands off the conn |

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An administrative shutdown tells the peer WHY, on the wire, rather than dropping the socket in silence | interop (real peer) | `test/interop/scenarios/shutdown-cease-frr`: FRR 10.3.1 reports `lastNotificationReason=Cease/Administrative Shutdown`. RED before the fix at `FRR reports no Cease/Administrative Shutdown NOTIFICATION from Ze` |
| The same, for the whole daemon under a real SIGTERM | functional | `test/reload/signal-stop-cease.ci` asserts the 45 octets at `seq=3`. Removing the send produces "peer got 2 messages, timed out waiting for the third" |
| The NOTIFICATION is sent while the reactor context is still live, not after the cancel | unit, deterministic | `TestReactorStopNotifiesWhileItsContextIsStillLive` reads `r.ctx.Err()` from inside the send through `onNotifSent`, so program order decides. 50/50 green as shipped, 50/50 red on the reverted ordering. The `.ci` above does NOT discriminate this: it passes 20/20 with `notifyPeersShutdown` moved after `cancel()`, because the octets are already in the kernel send buffer when `closeConn` sends FIN rather than RST |
| The action list is honoured from every connected state, not only Established | unit, RFC-tagged | `TestShutdownNotifySendsCeaseFromEveryConnectedState` (OpenSent, OpenConfirm, Established), carrying `RFC requirement: RFC4271-8.2.2-18 positive` |
| The Cease belongs to the STOP, not to a session being up | unit, RFC-tagged | `TestRFC4271NoCeaseWithoutAManualStop`, carrying the negative tag: zero octets reach a peer of a reactor nobody stopped |
| A restart forecloses nothing | unit | `TestReactorStopForRestartSendsNoNotification`, a table over both stops: `Stop` puts the Cease on the wire, `StopForRestart` closes the socket with zero octets on it |
| Shutdown stays inside its budget with a peer that cannot be written to | unit | `TestReactorStopStaysInBudgetWithUnreadablePeer`: `Stop` returns inside `shutdownNotifyBudget` rather than waiting out the 10s `controlWriteDeadlineMin`. Daemon-level: `ze` exited 2.8s after SIGTERM with the fix and 1.7s without, one Established FRR peer, two runs each |
| No conn becomes a session after the seal | unit, seven tests | `TestSealedStopAcceptsNoInboundSessionOnAConfiguredPeer`, `TestReactorStopAcceptsNoInboundSessionWhileItNotifies`, `TestTryCreateDynamicPeerRefusesAfterTheStopHasSealed`, `TestReactorStopStartsNoListenerForAnAddressAddedWhileItNotifies`, `TestPeerStartWithContextDoesNotLiftTheStopsSeal`, `TestRunOncePublishesNoSessionAfterTheStopHasMarkedThePeer`, `TestSessionConnectSendsNoOpenOnceTeardownHasRun`. Each measured RED on the tree before its own fix, except the two the TDD table names as non-discriminators |
| No data race in the shutdown path | race detector | `make ze-race-reactor` clean, `-race -count=20`, 373.201s, covering `Peer.stopping`, `Reactor.stopping`, `Session.seal` and `Peer.sealSession`. Every reactor file in the tree predates that run |
| The ledger stops being silent about Section 8.2.2 | RFC gate | `RFC4271-8.2.2-18` in `rfc/short/rfc4271.md` at MUST, with both polarities tagged and both cited by `rfc/requirements/rfc4271.md`, which is tracked at HEAD |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| This spec has no deferral shard | n/a | The metadata row says `-`, corrected 2026-08-03: it named a shard that never existed. No shard named for this spec stem exists under `plan/deferrals/`, so there is nothing to `--remove` and `deferral_shard_removal_problems` has nothing to block |
| `plan/deferrals/rfcgate-2-evidence.md`, the row that CREATED this spec (Ze sends no Cease on SIGTERM) | done | Its Destination cell names this spec, and the behaviour it describes is implemented and proven. That shard is a FOREIGN one and is not emptied by this: it is not removed here |
| Known Limitation: Ze advertises Graceful Restart with no per-AFI/SAFI tuple | deferred | NOT a conformance defect. RFC 4724 Section 4 sanctions the empty list for a speaker that preserves nothing. The defect is the CONFIG surface, where `capability { graceful-restart { restart-time 300; } }` reads as a promise the advertisement does not carry. Its own spec; a future reader must not re-raise it as a compliance question |
| Known Limitation: an abandoned notify goroutine holds `s.writeMu` for up to 9s after the budget expires and parks `closeConn` holding `s.mu` | deferred | Exit stays bounded, with zero margin: 1s of notify plus `cleanup` Phase 2's 2s is the hub's whole 3s. The fix is a write deadline taken from the shutdown budget rather than from `controlWriteDeadlineMin`, not a wider budget. Its own spec |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/shutdown_notify_test.go` | yes | `ls -l` 49832 bytes, `grep -c '^func Test'` = 17, UNTRACKED and `--file`d in commit A |
| `test/reload/signal-stop-cease.ci` | yes | `ls -l` 3843 bytes, UNTRACKED and `--file`d in commit A |
| `test/interop/scenarios/shutdown-cease-frr/announce-shutdown.py` | yes | `ls -l` 549 bytes |
| `test/interop/scenarios/shutdown-cease-frr/check.py` | yes | `ls -l` 5195 bytes |
| `test/interop/scenarios/shutdown-cease-frr/frr.conf` | yes | `ls -l` 785 bytes |
| `test/interop/scenarios/shutdown-cease-frr/ze.conf` | yes | `ls -l` 530 bytes |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The peer reads the Cease before the TCP close | `test/reload/signal-stop-cease.ci` carries `expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D0306021741646D696E6973747261746976652053687574646F776E`, read in the file, after the `action=sigterm:conn=1:seq=2` line |
| AC-2 | A black-holed peer costs the budget, not the exit | `TestReactorStopStaysInBudgetWithUnreadablePeer` and `TestShutdownNotifyWithoutSessionIsQuiet` are both in `shutdown_notify_test.go`; `make ze-race-reactor` ran the package clean |
| AC-3 | `Session.Close` is gone, not renamed | `gopls symbols internal/component/bgp/reactor/session_connection.go` returns `Connect`, `Accept`, `AcceptWithOpen`, `processOpen`, `tuneTCPConnection`, `connectionEstablished`, `CloseWithNotification`, `Teardown`, `TeardownAutomatic`, `seal`, `teardown`, `closeConn`, `setCloseReason` -- and no `Close`. Every remaining `.Close()` in the package is on a `net.Conn` |
| AC-4 | The dead `if p.session != nil` guard is gone | `Peer.cleanup` in `peer_run.go` no longer holds it |
| AC-5 | The wire assertion cannot silently regress | it is in `test/reload/signal-stop-cease.ci`, which is `--file`d in commit A; `test/reload/signal-stop.ci` carries Thomas's `rfc-test-change-approved:` marker and no longer carries a tag it does not assert |
| AC-6 | The ledger carries the requirement | `grep -n '8.2.2-18' rfc/requirements/rfc4271.md` returns the row, at MUST, citing the positive and negative tags |
| AC-7 | The GR question is answered with evidence | `test/interop/scenarios/shutdown-cease-frr` against FRR 10.3.1; the answer and its mechanism are recorded in A-2 |
| AC-8 | Cease from OpenSent and OpenConfirm too | `TestShutdownNotifySendsCeaseFromEveryConnectedState` |
| AC-9 | A restart is silent | `TestReactorStopForRestartSendsNoNotification`, both halves in one table |
| AC-10, AC-11 | No conn becomes a session after the seal | the seven tests named in Goal Validation; `Session.seal` is `tearingDown`'s only writer and `grep` over the two publish sites is the whole argument |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| SIGTERM to a running `ze` with one Established peer | `test/reload/signal-stop-cease.ci` | yes, read: it establishes a real session against `ze-peer`, sends `action=sigterm:conn=1:seq=2`, and asserts the 45 Cease octets at `seq=3`. The header states what it guards (the FEATURE) and what it does not (the ORDERING), both measured |
| the shutdown signal itself, without the NOTIFICATION assertion | `test/reload/signal-stop.ci` | yes, read: it drives the same signal on the same session and asserts the route, the EOR and a clean exit. It carries Thomas's approval marker recording that the tag it never asserted was removed |
| a GR-capable peer observing the shutdown | `test/interop/scenarios/shutdown-cease-frr/check.py` | yes, read: it asserts `lastNotificationReason=Cease/Administrative Shutdown` in FRR's own neighbor output, which is the peer's judgement rather than Ze's log |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `ze` exited 2.8s after SIGTERM with the fix and 1.7s without, one Established FRR peer, two runs each. `shutdownNotifyBudget` is 1s, the room between the hub's 3s and `cleanup` Phase 2's 2s. `TestReactorStopStaysInBudgetWithUnreadablePeer` drives the black-holed case |
| A-2 | **broken** | FRR 10.3.1 withdrew in 0.1s, `Notifications: 0 0`, `Remote GR Mode: NotApplicable`. Mistake Log row 1; the Task section's stale-route claim is downgraded in place and the fix's reason restated as diagnosability and conformance |
| A-3 | confirmed | `make ze-race-reactor` clean, `-race -count=20`, 373.201s, over a tree whose reactor files all predate the run |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/rfc-status.md`, RFC 4271 Implemented coverage | the cell records the Section 8.2.2 ManualStop Cease; the Remaining cell's spelled gap count is untouched, so `check_gap_count_agreement` reads the same number | yes |
| The six `docs/architecture/behavior/fsm-*.md` ManualStop rows | `Session.Close` no longer exists (`gopls symbols` on `session_connection.go`); every row now names `Session.Stop` / `Session.Teardown`, and the wire-side-effect cells read "Cease NOTIFICATION from `Session.Teardown` when a conn exists; `Session.Stop` sends nothing" | yes |
| `rfc/short/rfc4271.md` carries `RFC4271-8.2.2-18` at MUST | the level RAISES what Ze owes rather than lowering it, so no owner escalation is due (`ai/rules/rfc-compliance.md`). Every sibling row derived from a Section 8.2.2 action list stands on the same ground | yes |
| `rfc/audit/rfc7606.json` | five verdicts shifted by the `session_test.go` edit and were re-stamped with `make ze-rfc-reseal`; the units are byte-identical and nothing was re-judged. It now pins `session_rfc7606_diagnostics_test.go`, which is why that file must ride in this commit | yes |
| No user guide, CLI, config, YANG, RPC or plugin surface changed | checklist rows 1-8 and 11-17, each answered with its grep in the Documentation Update Checklist above | yes |
| `ai/RFC-REQUIREMENTS.md` | GENERATED; `make ze-rfc-index` was NOT run and the file is NOT in this commit, per the owner's instruction for this phase | yes, and stated rather than claimed clean |

### What was NOT run, and why

- **`make ze-verify` was NOT run.** A QEMU run holds this shared checkout and
  the owner directed that no suite start. This checkout is shared, so a fully
  green `ze-verify` is unreachable by construction (`ai/rules/git-safety.md`).
  The commit is prepared with `--unverified`.
- **No end-to-end `make ze-qemu-needs-linux-test` has completed on this
  machine.** Nothing here claims that target passes.
- **`make ze-doc-test`, `make ze-lint-changed`, `make ze-plugin-test`,
  `make ze-interop-test` and `make ze-rfc-index` were NOT run in this phase.**
  The interop and functional results quoted above are from the implementation
  sessions that produced them, and are recorded as such.
- **What WAS run in this phase:** `make ze-race-reactor` (clean, `-race
  -count=20`, 373.201s), `python3 scripts/dev/review_gate.py check` on both
  specs (OK, clean, hashes match), `python3 scripts/dev/relax-census.py` (748 at
  HEAD, ceiling 752, working tree 752), `python3 scripts/dev/spec-citation-check.py`
  (OK), and `make ze-tracked-build-check` after the commit script.

## Core Insight

**Coverage of a dead path reads exactly like coverage of a live one.** The
defect was invisible in three ways at once: the code existed and compiled, a
unit test covered it and passed, and the functional test that drove the real
signal asserted everything except the thing that was missing. Two guards each
made `Session.Close` unreachable, and each was a guaranteed program order rather
than a race, so nothing ever flaked to expose it.

**The second insight is about the fix, not the defect.** Rounds 3, 4 and 5 each
closed the rail in front of them and left another open, because each proved "no
new socket opens". The property that must hold is "no conn becomes a SESSION
after the seal", and it differs from the first on exactly the population that
broke this twice: a connection that already existed when the seal was taken.
Stated that way, the proof is two writes with one writer each, and a grep
settles it.
