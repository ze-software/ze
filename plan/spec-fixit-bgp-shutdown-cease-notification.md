# Spec: fixit-bgp-shutdown-cease-notification

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-bgp-shutdown-cease-notification.md` |
| Updated | 2026-07-29 |

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

**Where a fix inserts.** Before the cancel propagates, not after. `Reactor.cleanup()`
Phase 1 (`reactor.go`) is the last point at which sessions are still live
and the sockets still open; the existing `Session.Teardown(NotifyCeaseAdminShutdown, ...)`
path (`session_connection.go`) already works and is exercised by four other
callers, so the fix is a call-site and ordering problem, not new wire code.

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
| A-1 | Sending Cease before the cancel propagates fits inside the existing 2s per-peer and 3s engine budgets | The send is one small message per peer on an already-open socket; `Teardown` does this today on four other paths | Shutdown slows or hangs, which is worse than the defect | Measure shutdown wall time with N peers before and after | unvalidated |
| A-2 | A GR-capable peer currently retains Ze's routes as stale across an admin shutdown | Peer sees a bare FIN, and Ze advertises GR (`docs/features/rfc-status.md`) | The defect is cosmetic rather than operator-visible, and priority drops | AC-7: a real peer (FRR or BIRD) in the interop lab | unvalidated |
| A-3 | `Session.Teardown` is safe to call from the reactor's cleanup goroutine | It is called today from BFD, congestion and prefix-limit paths | A data race on session state during shutdown | `make ze-race-reactor` (`ai/rules/testing.md`) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback | Status |
|----|------|--------------|----------------------|--------|
| R-1 | A graceful send on shutdown blocks on an unresponsive peer and turns a fast exit into a hang | Shutdown wall time rises with a black-holed peer | Bound the send by the existing budget and treat a failed write as non-fatal (AC-2) | open |
| R-2 | Fixing the ordering introduces a shutdown race the race detector does not catch on a quiet host | Intermittent reds under load | `make ze-race-reactor` (`-race -count=20`), which `ai/rules/testing.md` requires for reactor concurrency changes | open |
| R-3 | The fix is written as "widen the deadline", which changes nothing | A diff that touches only timeouts | Current Behavior states explicitly that timing is not the cause | open |

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
| shutdown path calls Teardown while the session is live | `internal/component/bgp/reactor/` | AC-1 at the unit level: the send happens before the cancel nils the socket | |
| a failed write during shutdown is non-fatal and bounded | same | AC-2 | |
| `Session.Close` is reachable, or absent | same | AC-3, AC-4: the dead-code half | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| per-peer shutdown budget | 0..2s | 2s | N/A | a send that exceeds it must be abandoned, not awaited |
| engine shutdown budget | 0..3s | 3s | N/A | same |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/reload/signal-stop.ci` | existing file, new assertion | An operator stops Ze and the peer is told why, rather than seeing the socket vanish | |
| a GR-peer interop scenario | `test/interop/scenarios/` | AC-7: what a real GR-capable peer does with Ze's routes across an admin shutdown | |

## Files to Modify
- `internal/component/bgp/reactor/reactor.go` - the graceful step in `cleanup()` Phase 1
- `internal/component/bgp/reactor/peer_run.go` - the dead guard at `:536-542`
- `internal/component/bgp/reactor/session_connection.go` - `Close()` reachable or deleted
- `internal/component/bgp/reactor/session_test.go` - `TestSessionGracefulClose` asserts wire bytes, not just `NoError`
- `test/reload/signal-stop.ci` - assert the NOTIFICATION
- `test/plugin/rfc7606-relay-one-field.ci` - **a booby trap this spec will spring.**
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
- possibly a GR-peer interop scenario under `test/interop/scenarios/` (AC-7)

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

## Known Limitations

- The GR stale-route consequence is reasoned from what the peer observes plus the
  fact that Ze advertises GR. It is **not** traced through Ze's GR plugin and not
  tested against a real peer. AC-7 exists to settle it.
- What FSM event fires on the shutdown path instead of `fsm.EventManualStop`
  (`session_connection.go`, also dead) was not traced.

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
| AC-2 | The same, with the peer socket already broken | Shutdown still completes inside the existing 2s/3s budgets; a failed send is logged, never fatal, and never hangs |
| AC-3 | `Session.Close()` after this spec | It has a reachable production caller, or it is deleted. Dead code that appears to implement a protocol action is worse than its absence, because it reads as coverage |
| AC-4 | `peer_run.go` | Either reachable, or removed with its guard |
| AC-5 | `test/reload/signal-stop.ci` | Asserts the NOTIFICATION on the wire, so the behaviour cannot silently regress to a bare FIN again |
| AC-6 | `rfc/short/rfc4271.md` | Carries an extracted requirement for the §8.2.2 ManualStop action list at its true level, so the ledger stops being silent about it. **All THREE sites, not just the Established one**: `rfc/full/rfc4271.txt:3718` (OpenSent), `:3733` (OpenConfirm) and `:3943` (Established) each list "sends the NOTIFICATION message with a Cease" for a ManualStop event. Verified 2026-07-29 by grepping the source text; an earlier reading cited only the Established site, and a fix that covers only Established leaves an operator stopping a peer mid-handshake with the same silent close |
| AC-8 | A ManualStop while a session is in OpenSent or OpenConfirm | The peer receives Cease, same as from Established. The FSM lists the action in all three states, so a fix keyed only to Established is incomplete |
| AC-7 | A GR-capable peer, admin shutdown | The stale-route question in the Task is answered with evidence: either peers correctly withdraw, or the spec records what they do instead |

## Known Limitations

- The GR stale-route consequence is reasoned from what the peer observes plus the
  fact that Ze advertises GR. It is **not** traced through Ze's GR plugin and not
  tested against a real peer. AC-7 exists to settle it.
- What FSM event fires on the shutdown path instead of `fsm.EventManualStop`
  (`session_connection.go`, also dead) was not traced.

## RFC Documentation (Scope: protocol)

RFC 4271 §6.7 (MAY, the governing keyword), §8.2.2 (the indicative action list),
RFC 4486 (subcodes; both extracted requirements are MAY).
