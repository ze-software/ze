# Spec: fixit-ike-resource-lifetime-leaks

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `-` |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Three IKE resources outlive the thing that owns them, because a cleanup step sits
below an early return or has no paired teardown at all.**

Found on 2026-08-02 while closing the rfcgate-1b RFC 7296 pilot spec. Each
defect was verified by reading the producing function. None was measured at run
time, and phase 1 of this spec is that measurement.

The shared shape is one rule, broken three ways: a resource acquired on the way in
must be released on EVERY way out, including the error ways. Two of the three leak
a host-wide or node-wide object, so the blast radius is larger than the tunnel that
created it.

### Leak 1: a failed state delete strands the raw ESP socket

`RemoveSA` (`internal/component/ike/dataplane/xfrm_linux.go`) calls
`netlink.XfrmStateDel` at `:169` and returns its error immediately. The
`b.espForms.Forget(spi)` call sits below it at `:175`, so a delete that fails skips
the forget and the SPI stays in the map forever.

The reachable trigger is the documented double-remove in
`internal/component/ike/engine/delete.go`: the second remove finds no state and the
delete fails, which is the ordinary case rather than an exotic one. The forgotten
entry keeps a host-wide raw ESP socket alive after the last tunnel is gone.

### Leak 2: an early error return strands four node-wide bypass policies

`p.Run` (`internal/component/ike/engine/register.go`) returns 1 at `:451`. The
`removeIKEBypass(dataplane.Get(), log)` call is at `:483`, below it. So every error
path above `:483` leaves the four IKE control-plane bypass policies installed after
the process is gone.

Those policies are node-wide, not per-peer. A bypass that survives its installer
keeps IKE traffic out of IPsec processing for a daemon that is no longer running.

### Leak 3: an SAUp event with no paired SADown

`runInitiator` emits `SAUp` at `internal/component/ike/engine/fsm.go`, then
returns `ps.runEstablished(...)` at `:249`. Nothing on that path emits `SADown`.

`runResponder` is the control that shows this is a defect rather than a design
choice: it emits `SAUp` at `:305`, and when `runEstablished` returns it calls
`emitSADown(bus, sa, log)` at `:324`. The initiator path has the emit and not the
pair.

The reconnect-on-peer-Delete path re-enters `runInitiator`, so the imbalance
repeats once per reconnect cycle rather than once per process. Any subscriber that
counts SAs up against SAs down drifts without bound.

### Leak 4: a liveness probe that was never built holds the request window (CLOSED 2026-08-07)

Inherited from the closure review of `spec-fixit-ike-dpd-cleartext`, row 2 of
`plan/deferrals/fixit-ike-dpd-cleartext.md`. It is the same rule as the three above.

`sendDPD` (`internal/component/ike/engine/dpd.go`) reserved the one request window of
RFC 7296 Section 2.3, then wrapped the build and the send in `if tr != nil`, then set
`awaitReply` whether or not a datagram existed. With a nil transport the window was held
by a probe nobody could answer. `serviceRequestWindow` (`established.go`) returns early
while a probe awaits its reply, and `shouldRetransmit` finds no datagram to repeat, so
the one exit left was the dead-peer verdict, which RFC 7296 Section 2.4 allows only after
repeated attempts go unanswered.

Fixed: `sendDPD` returns before `reserveRequestWindow` when the SA has no send path, and
the build moved out of the conditional. The awaiting state is now entered only for a
datagram `sendRaw` was given, so an awaited probe always has bytes behind it.

The predicate is `sa.sendPath(tr)`, never the `tr` argument. `tr` is a FALLBACK:
`sendPath` (`sa.go`) answers with `sa.nattSocket` whenever the SA has floated to port
4500, so a working NAT-traversing tunnel sends while `tr` is nil. A guard on `tr` would
stop every probe for the life of that tunnel, which is the black hole RFC 7296 Section
2.4 asks liveness checks to prevent.

### Leak 5: an unconfirmed IKE swap SA outlives its session

`runEstablished` (`internal/component/ike/engine/established.go`) tears down `ownedSA`
and `pendingRekey` when it returns, and its own comment called `pendingRekey` "the one
holder forgetKeys cannot reach". There are TWO.

`ps.pendingIKESwap` (`internal/component/ike/engine/reconcile.go`) is written only by
`setPendingIKESwap`, from the IKE-rekey responder branch of `handleOwnedInbound`
(`internal/component/ike/engine/inbound.go`), and cleared only where the peer's
INFORMATIONAL Delete of the old IKE SA promotes it (`inbound.go`). Nothing cleared it at
session end, and `ps.run` loops `runOnce` on the SAME `PeerSession`
(`internal/component/ike/engine/reconcile.go`), so a swap the peer never confirmed
survived into the next reconnect cycle holding its `SK_*` material, against RFC 7296
Section 2.12. The next cycle's owner loop then promotes it: the swap branch keys on the
slot being occupied, not on which cycle filled it, so the peer's first IKE Delete moves
the live tunnel onto keys negotiated for a connection that is over.

It is the same rule as the four above, in the same function the spec already edits: a
resource acquired on the way in is released on EVERY way out. Found by an independent
review of this spec on 2026-08-19, and recorded before that at
`plan/journal/stale-artifact-reused.md` (2026-08-12 row).

### What is read and what is measured

Every claim above is READ from the producing function, and the file plus symbol is
named so a reader can check it. Nothing here is measured at run time yet. A run
that shows the ESP socket count falling to zero after the last tunnel closes would
overturn leak 1, and phase 1 exists to settle each of the three before any fix
lands.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - the early-return shape is the failure mode it names
  → Constraint: a cleanup that a miss or an error skips must fail loudly, never silently leave the resource held
- [ ] `ai/rules/go-standards.md` - paired-operation documentation
  → Constraint: a paired operation (acquire/release, Emit-up/Emit-down) states the obligation in the godoc using MUST

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - IKE SA teardown and Delete processing
  → Constraint: Section 1.4 Delete semantics are what makes the double-remove in `delete.go` ordinary rather than exceptional

**Key insights:** (minimal context to resume after compaction)
- Three leaks, one rule: release on every exit, including error exits.
- `runResponder` is the in-tree control for leak 3. Copy its pairing, do not invent one.
- Leaks 1 and 2 escape the tunnel and the process respectively, so neither is bounded by peer count.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `RemoveSA` at `:163`, `XfrmStateDel` at `:169`, `espForms.Forget` at `:175`
- [ ] `internal/component/ike/engine/register.go` - `Run` returns 1 at `:451`, `removeIKEBypass` at `:483`
- [ ] `internal/component/ike/engine/fsm.go` - `SAUp.Emit` at `:238` (initiator) and `:305` (responder), `emitSADown` at `:324`
- [ ] `internal/component/ike/engine/delete.go` - the documented double-remove that reaches leak 1
- [ ] `internal/component/ike/engine/dpd.go` - `sendDPD` reserved the window, skipped the build on a nil transport, and set `awaitReply` with a nil `probeMsg`. It is the only non-test writer of `awaitReply = true`, which is what makes the fixed guard sufficient
  → Decision: return before `reserveRequestWindow`, never reserve and release. Nothing is then taken that a later path has to give back
- [ ] `internal/component/ike/engine/established.go` - `serviceRequestWindow` and `serviceRequestRetransmit` both return early on `dpd.awaitingReply()`, and `maintainSA` reads `dpd.timedOut(now)` BEFORE `shouldRetransmit` in the same tick
  → Constraint: `shouldRetransmit` refuses once `timedOut`, so a liveness budget shorter than one tick plus one backoff step reaches the verdict with no repeat. See Known Limitations
- [ ] `internal/component/ike/engine/reconcile.go` - `pendingIKESwap` is declared beside `pendingRekey` with the same ownership (the `maintainSA` loop, no lock), `setPendingIKESwap` is its only writer, and `ps.run` loops `runOnce` on one `PeerSession` per peer
  → Constraint: the session outlives the SA, so anything left on the session is carried into the next reconnect cycle
- [ ] `internal/component/ike/engine/inbound.go` - the IKE-rekey responder branch fills `pendingIKESwap`, and the peer's INFORMATIONAL Delete of the old IKE SA is the only path that empties it
  → Constraint: the promotion branch keys on the slot being occupied, never on which cycle filled it

**Behavior to preserve:** (unless the user explicitly said to change it)
- `RemoveSA` still returns the `XfrmStateDel` error to its caller. Forgetting the ESP form must not swallow the failure.
- `removeIKEBypass` stays idempotent, so running it on a path that already ran it is harmless.
- `SAUp` and `SADown` payload shapes stay as they are. Subscribers outside IKE read them.
- `runResponder` pairing is already correct and must not be disturbed.

**Behavior to change:** (only what the user asked for)
- `RemoveSA` forgets the ESP form whether or not the state delete succeeded.
- Every `p.Run` error exit removes the IKE bypass policies.
- The initiator path emits `SADown` when its established loop returns.
- `runEstablished` releases `ps.pendingIKESwap` on every exit, as it already releases
  `ps.pendingRekey`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator teardown, peer Delete, DPD timeout, or daemon stop reaches the IKE engine.
- Format at entry: an IKE Delete payload, a config change, or a process signal.

### Transformation Path
1. Teardown decision in `engine/delete.go` or the owner loop in `engine/fsm.go`.
2. Child SA teardown calls into `dataplane.Dataplane` (`RemoveSA`, `RemovePolicy`).
3. The XFRM backend issues netlink calls and updates its own `espForms` map.
4. Process exit runs `p.Run`'s tail, which is where `removeIKEBypass` lives.
5. The event bus carries `SAUp` and `SADown` to subscribers outside IKE.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Dataplane backend | `dataplane.Dataplane` interface calls | Yes -- `runEngine` (`engine/register.go`) reaches the backend only through `dataplane.Get()`, and the deferred cleanup keeps that order: `removeIKEBypass` runs before `dataplane.CloseBackend`, because Close clears the active backend and a removal ordered after it would talk to nothing. `TestRunRemovesIKEBypassOnEveryErrorExit` drives the boundary with a registered recording backend rather than the helper |
| Backend ↔ Kernel | netlink XFRM state and policy messages | Yes -- `RemoveSA` (`dataplane/xfrm_linux.go`) still returns the `XfrmStateDel` error unwrapped in meaning, so a kernel refusal reaches the caller; only the ESP-form release moved above it. `TestXFRMDoubleTeardownLeavesNothing` drives a real netlink delete the kernel refuses, and `ipsec-teardown-leaves-nothing.ci` reads the real state and policy tables after both daemons stop |
| Engine ↔ Event bus | `SAUp` / `SADown` typed events | Yes -- both are `events.Register` types in `engine/events.go` and cross the bus by that registration only. The added emit calls the existing `emitSADown` (`engine/reconcile.go`), so no new payload shape and no new producer type. `TestSAUpAndSADownBalanceAcrossReconnects` counts the pair over N cycles from a subscriber |
| Process ↔ Node | bypass policies that outlive the process | Yes -- this is the boundary leak 2 crosses, and the fix closes it: the release is deferred at the install site, so it runs on the error exits too. A-3 confirmed the policies carry no peer identity and are not reference-counted, so nothing else on the node removes them |
| Session ↔ SA | `ps.pendingRekey` and `ps.pendingIKESwap` held on the session while the SA is owned by `maintainSA` | Yes -- both fields are owned without a lock by the `maintainSA` goroutine (`engine/reconcile.go`), and the teardown defer runs after that goroutine has returned, which is why it is the one safe place to touch them. `TestRunEstablishedClearsPendingRekeyOnExit` and `TestRunEstablishedClearsPendingIKESwapOnExit` drive `runEstablished` itself, so the race detector covers the hand-off (`make ze-unit-pkg-test` runs `-race`) |

### Integration Points
- `internal/component/ike/dataplane` - the backend that owns `espForms`.
- `internal/core/events` - the typed bus carrying `SAUp` and `SADown`.
- Any subscriber counting SA lifecycle events, which is what leak 3 misleads.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every fix moves an EXISTING call, and none adds a new route. `RemoveSA` still reaches the kernel through `netlink` and the ESP form through `b.espForms`; `runEngine` still reaches the policy table through `dataplane.Get()`; `runInitiator` still reaches the bus through `emitSADown`; `runEstablished` still releases the swap SA through `setPendingIKESwap`, the same writer the fill path uses. No test reaches past an interface: `TestRunRemovesIKEBypassOnEveryErrorExit` registers a backend under `dataplane.Register` rather than assigning one |
| No unintended coupling (components stay isolated) | Yes | The diff is confined to `internal/component/ike/` plus `internal/component/plugin/process/manager.go`. The plugin-manager change is a generic grace-period fix with no IKE spelling in it (`pluginStopGrace` applies to every plugin), which is the direction `ai/rules/plugins.md` requires: no plugin name in a shared package. Nothing outside IKE imports `engine`'s new symbols, because there are none -- no exported symbol was added |
| No duplicated functionality (extends existing, does not recreate) | Yes | Four of the five fixes are one line each moving an existing call to a `defer`. Leak 3 calls the existing `emitSADown`, copied from `runResponder`, rather than a second emitter. Leak 5 calls the existing `setPendingIKESwap` with nil rather than open-coding `forgetKeys` plus a nil assignment, so the release and the close stay one call and cannot drift apart |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No encoding path is touched. `RemoveSA` takes a `uint32` SPI and a `net.IP` the caller owns and copies neither; the SA erasures release references and overwrite in place (`forgetKeys`, `sa.go`), which is the opposite of copying. `SAEvent` is built once per emit, as it already was |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Nothing is added to any registry and nothing is hardcoded into one. `SAUp` / `SADown` were already `events.Register` types and keep their names and payloads; no new event, command, YANG leaf, or dataplane backend is introduced; no switch case or per-feature field is added to a shared package. The test-only backend registers through `dataplane.Register` like every other backend, and is removed by `t.Cleanup` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The double-remove in `delete.go` really does make `XfrmStateDel` fail in ordinary operation | read, `engine/delete.go` | Leak 1 is rare rather than routine, which lowers its priority but does not close it | a QEMU run counting `espForms` entries after a tunnel closes twice | broken 2026-08-17 |
| A-2 | A stranded `espForms` entry holds a real host-wide raw ESP socket open | read, `dataplane/xfrm_linux.go` | Leak 1 is a map leak only, far cheaper than stated | inspect the socket table in QEMU after the repro | confirmed 2026-08-17 |
| A-3 | The four bypass policies are node-wide and survive process exit | read, `engine/register.go` `removeIKEBypass` | Leak 2 is bounded by the process and self-clears | `ip xfrm policy` in QEMU after killing ze on an error path | confirmed 2026-08-17 |
| A-4 | No subscriber compensates for the missing initiator `SADown` elsewhere | read, `engine/fsm.go`, `engine/reconcile.go` | Leak 3 is already handled and only the direct path is unbalanced | grep every `SADown` producer and every subscriber | confirmed 2026-08-17 |
| A-5 | Nothing but the peer's IKE Delete empties `ps.pendingIKESwap`, so a session that ends before that Delete carries the swap SA into the next cycle | read, `engine/reconcile.go`, `engine/inbound.go` | Leak 5 is already handled somewhere else and the release is a duplicate | `gopls references` on `setPendingIKESwap` and on the field, over non-test code | confirmed 2026-08-19 |
| A-6 | The teardown defer is a safe place to touch `pendingIKESwap`, on the same argument the `pendingRekey` clear already rests on | read, `engine/established.go`, `engine/reconcile.go` | The release needs a lock or a different site, and the one-line fix is wrong | both fields carry the same "owned by the maintainSA loop, no lock" declaration, and the defer runs after `maintainSA` returned; the race detector runs it (`-race` is on in `make ze-unit-pkg-test`) | confirmed 2026-08-19 |

A-1 is BROKEN in the half that matters, and the correction widens leak 1 rather than
closing it. The second `RemoveSA` does fail, as stated. It is not what strands the ESP
form: the FIRST remove succeeded and already ran `Forget`, so the second finds nothing
left to forget. The stranding case is a FIRST remove that fails while the SPI is still
watched, which the kernel produces on its own -- a hard lifetime expiry, an operator
flush, or any state the kernel dropped behind the backend. That is why the fix releases
on every exit rather than on the second call, and why both new tests drive a delete the
kernel refuses.

A-2 is confirmed by reading the producer: `espFormReceiver.Forget`
(`dataplane/espform_linux.go`) releases the raw sockets only when the LAST watched SPI
goes, so one stranded entry holds them open for the life of the process.
`TestXFRMDoubleTeardownLeavesNothing` measures that half on a real kernel.

A-3 is confirmed by reading `installIKEBypass` / `removeIKEBypass` (`engine/bypass.go`):
the four policies carry no peer identity and go straight to the kernel policy table, and
`runEngine` runs as an in-process plugin whose non-zero exit is logged rather than
turned into a process exit (`internal/component/plugin/process/process.go`), so nothing
removes them once the engine returns.

A-4 is confirmed by enumerating every `emitSADown` producer: `reconcile.go` (the
reconcile stop path), `register.go` (TerminateAllSAs and TerminatePeerSA),
`established.go` (a PENDING SA, never the owned one) and `fsm.go` (the responder owner
loop). None of them runs when an established initiator tunnel goes down of its own
accord, which is the reconnect path.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Forgetting the ESP form on a failed delete hides a real kernel failure | a delete error that no longer changes behavior | keep returning the error, and log the forget separately |
| R-2 | Emitting `SADown` on the initiator path double-emits where a caller already emits | subscribers see two downs per up | CLOSED 2026-08-19. Every `emitSADown` caller was enumerated before the emit was added, and the emit sits at the single return point with `ps.setSA(nil)` beside it so the operator paths find no SA to emit a second down for. `TestSAUpAndSADownBalanceAcrossReconnects` counts the pair, and `10-clear-reestablish` proves a real peer still re-establishes across that clear |
| R-3 | Removing bypass policies on an early error path removes them while another peer still needs them | IKE traffic starts entering IPsec processing | confirm the bypass is process-wide and installed once, not per-peer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An over-eager bypass removal sends IKE control traffic into IPsec processing and stops negotiation node-wide. A double `SADown` misleads every lifecycle subscriber. |
| How is it reverted? | Single commit revert. No config migration, no wire-visible change. |
| Who else touches this path? | `spec-fixit-child-sa-rekey-policy` (policy teardown; closed 2026-08-22, so the durable record is `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` and `docs/architecture/ike/ipsec-13-rekey-wire.md`), `spec-fixit-vpp-ipsec-inoperable` (the other backend), `plan/spec-lifecycle-invariants.md` (event pairing in the subscriber namespace) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Second teardown of one Child SA | → | `RemoveSA` forget-on-error path | `TestRemoveSAForgetsESPFormWhenStateDeleteFails` |
| Daemon start that fails after bypass install | → | `p.Run` error exit | `TestRunRemovesIKEBypassOnEveryErrorExit` |
| Initiator tunnel that goes down | → | `runInitiator` teardown | `TestInitiatorEmitsSADownWhenEstablishedLoopReturns` |
| Full teardown on a real kernel | → | XFRM state and policy tables | `test/ipsec/ipsec-teardown-leaves-nothing.ci`, and `fsuite ipsec` in `scripts/evidence/qemu-all-tests.sh` is what runs it |
| A DPD tick on an SA with no send path | → | `sendDPD` early return | `TestDPDNoTransportTakesNoWindow` |
| A DPD tick on a floated SA with a nil fallback | → | `sendDPD` send-path predicate | `TestDPDFloatedSAProbesWithoutTheFallback` |
| A session that ends holding an unconfirmed IKE swap SA | → | `runEstablished` teardown defer | `TestRunEstablishedClearsPendingIKESwapOnExit` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `XfrmStateDel` returns an error inside `RemoveSA` | The SPI is absent from `espForms`, and the error still reaches the caller unchanged |
| AC-2 | `p.Run` takes any error exit after the bypass is installed | No IKE bypass policy remains in the kernel policy table |
| AC-3 | An initiator tunnel establishes and then goes down | Exactly one `SADown` follows the one `SAUp`, on the initiator path |
| AC-4 | An initiator tunnel reconnects N times after a peer Delete | The count of `SAUp` equals the count of `SADown` for every N |
| AC-5 | A tunnel is torn down twice on a real kernel | The XFRM state table, the policy table, and `espForms` are all empty. Proven in two halves: `TestXFRMDoubleTeardownLeavesNothing` drives the SECOND `XfrmStateDel` for one SPI, which the kernel refuses, and asserts the SPI is unwatched and the ESP sockets closed; `ipsec-teardown-leaves-nothing.ci` tears one tunnel down through the operator clear, twice over the same peer, then stops both daemons and asserts no state, no policy and no raw ESP socket is left |
| AC-6 | `sendDPD` runs on an SA with no send path | No request window is held, no `awaitReply` is set, no probe is stored, and no Message ID is spent |
| AC-7 | `sendDPD` runs on a floated SA whose fallback transport is nil | The probe leaves from the NAT-T socket, and the SA awaits its reply |
| AC-8 | A session ends while an IKE-SA rekey it answered is still unconfirmed | `ps.pendingIKESwap` is empty when `runEstablished` returns, and the SA it held has forgotten its `SK_*`, its EAP MSK and its nonces, so the next reconnect cycle has nothing to promote |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Tears a tunnel down and brings it back many times | fsm → dataplane → kernel → event bus | `test/ipsec/ipsec-teardown-leaves-nothing.ci` |
| 2 | Stops ze after a startup error | `p.Run` error exit → `removeIKEBypass` → kernel | `TestRunRemovesIKEBypassOnEveryErrorExit` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRemoveSAForgetsESPFormWhenStateDeleteFails` | `internal/component/ike/dataplane/xfrm_state_del_failure_linux_test.go` | AC-1 | PASS 2026-08-17, red without the fix |
| `TestRunRemovesIKEBypassOnEveryErrorExit` | `internal/component/ike/engine/register_test.go` | AC-2 | PASS 2026-08-17, red without the fix |
| `TestInitiatorEmitsSADownWhenEstablishedLoopReturns` | `internal/component/ike/engine/sa_event_pairing_test.go` | AC-3 | PASS 2026-08-17, red without the fix |
| `TestSAUpAndSADownBalanceAcrossReconnects` | `internal/component/ike/engine/sa_event_pairing_test.go` | AC-4 | PASS 2026-08-17, red without the fix |
| `TestXFRMDoubleTeardownLeavesNothing` | `internal/component/ike/dataplane/xfrm_teardown_integration_linux_test.go` | AC-5, kernel half | written, runs on the QEMU integration rail |
| `TestDPDNoTransportTakesNoWindow` | `internal/component/ike/engine/dpd_test.go` | AC-6 | PASS 2026-08-07 |
| `TestDPDFloatedSAProbesWithoutTheFallback` | `internal/component/ike/engine/dpd_test.go` | AC-7 | PASS 2026-08-07 |
| `TestRunEstablishedClearsPendingIKESwapOnExit` | `internal/component/ike/engine/established_test.go` | AC-8 | PASS 2026-08-19, tagged `RFC7296-2.12-1` positive. Red without the fix, on all four assertions (measured: the slot stays occupied and SK_d, the EAP MSK and both nonces survive) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| reconnect cycles counted for event balance | 1..N | N | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-teardown-leaves-nothing` | `test/ipsec/ipsec-teardown-leaves-nothing.ci` | After the operator tears the tunnel down and both daemons stop, no XFRM state, no XFRM policy and no raw ESP socket remains | PASS 2026-08-17. `fsuite ipsec` added to `scripts/evidence/qemu-all-tests.sh`, so the suite now executes in the QEMU VM where `option=needs-linux:caps=net-admin` is satisfied. Two daemons, one real xfrm backend: the kernel keys a state on (src, dst, spi, proto) and the two ends of one Child SA are the SAME two states, so a second real backend would answer EEXIST (measured). Discriminates, and the two halves are not equally strong. With `RemoveSA`'s state delete removed the clear leaves both states behind and the test reddens. With the bypass release removed ALTOGETHER -- the deferred call and the shutdown tail both -- the test reports `RESIDUE: states=0 policies=8`. That second half does NOT discriminate AC-2's fix: the test stops both daemons with `cmd=stop:signal=term`, which is the clean exit, and HEAD's shutdown tail already removed the bypass on that path. So what the `.ci` proves is that the release still runs on the clean path after it moved into a defer, which is the regression this refactor could have caused. AC-2 names the ERROR exit, and `TestRunRemovesIKEBypassOnEveryErrorExit` is its only proof: it drives `runEngine` to `return 1` through a broken plugin pipe and asserts every installed policy came back. EXECUTED, twice: natively in an unprivileged user+net namespace (`unshare -rn`, which satisfies caps=net-admin without root), and in the QEMU VM through the new `fsuite ipsec` line, where the whole suite passed 16/16 in 180s |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `09-responder-ike-rekey` | `test/interop-ipsec/scenarios/` | strongSwan | The make-before-break path this spec now releases at session end still completes normally: strongSwan initiates the IKE SA rekey, Ze answers it and holds the new SA in `ps.pendingIKESwap`, and strongSwan's Delete of the old IKE SA promotes it. A release ordered too early would leave nothing to promote, and the tunnel would drop instead of swapping | PASS 2026-08-19, executed: `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=09-responder-ike-rekey`, strongSwan reports `completed a peer-initiated IKE-SA rekey against Ze the responder` and the tunnel survives. It bounds the fix rather than proving it: a confirmed swap empties the slot before `runEstablished` returns, so the release runs on an empty slot here. The proof that the release happens is the unit test |
| `10-clear-reestablish` | `test/interop-ipsec/scenarios/` | strongSwan | Repeated teardown against a real peer: an operator `clear vpn ipsec sa` sends strongSwan an authenticated Delete and Ze re-initiates, with a new ESP SPI inside 30 seconds. This is the reconnect cycle leak 3 and leak 5 both live on, driven by a real peer rather than a fixture | PASS 2026-08-19, executed: `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=10-clear-reestablish`, ESP SPIs move from `['0x10db820f', '0xc0623eca']` to `['0x7773ae74', '0xc5fb5a22']` and strongSwan reports the SA established again inside 30s. It is the guard R-2 asked for: `runInitiator` now clears `ps.sa` beside its `emitSADown`, and a real peer re-establishing across that clear is what shows the teardown did not break the reconnect |
| `05-child-rekey` | `test/interop-ipsec/scenarios/` | strongSwan | N-A for this spec. The spec cited it at `test/interop/scenarios/`, which is the BGP lab; the scenario is IPsec and lives under `test/interop-ipsec/scenarios/`. It proves Child SA rekey make-before-break against strongSwan, which no fix here changes | N-A |

## Files to Modify
- `internal/component/ike/dataplane/xfrm_linux.go` - DONE 2026-08-17, `RemoveSA` defers `espForms.Forget` so every exit releases it, and the delete error still reaches the caller
- `internal/component/ike/engine/register.go` - DONE 2026-08-17, `runEngine` defers `removeIKEBypass` plus `dataplane.CloseBackend` beside the install, and the shutdown tail no longer repeats them
- `internal/component/ike/engine/fsm.go` - DONE 2026-08-17, `runInitiator` calls `emitSADown` and clears `ps.sa` when its established loop returns, as `runResponder` does
- `internal/component/ike/engine/events.go` - DONE 2026-08-17, the `SAUp` / `SADown` godoc states the pairing obligation with MUST on both sides
- `internal/component/ike/engine/dpd.go` - DONE 2026-08-07, `sendDPD` returns before it reserves the request window when the session has no transport
- `internal/component/ike/engine/established.go` - DONE 2026-08-19, `runEstablished`'s teardown defer releases `ps.pendingIKESwap` beside `ps.pendingRekey`, and the comment that called `pendingRekey` "the one holder forgetKeys cannot reach" now names both
- `internal/component/ike/engine/reconcile.go` - DONE 2026-08-19, the `pendingIKESwap` field states the emptiness obligation and `setPendingIKESwap`'s godoc names its nil argument as the release half of the pair
- `docs/architecture/ike/ipsec-14-responder.md` - DONE 2026-08-19, the make-before-break paragraph states where the pending slot is released and why
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - DONE 2026-08-22, the design doc `events.go` and `fsm.go` both declare. It said the two SA lifecycle events are registered and said nothing about their pairing, which is the obligation leak 3 broke and this spec now states in the `events.go` godoc. A paragraph names both owner loops as the producers of both events, with a source anchor for `runInitiator` and `runResponder`
- `docs/architecture/api/process-protocol.md` - N-A 2026-08-22, the design doc `manager.go` declares. Its daemon-shutdown row already names `pluginStopGrace` and the group wait that follows it, so the page describes the code this spec left behind rather than the code it replaced. No edit is owed
- `docs/features/rfc-status.md` - DONE 2026-08-19, the RFC 7296 Section 2.12 sentence names the two session-held holders
- `rfc/requirements/rfc7296.md` - DONE 2026-08-19, GENERATED, regenerated by `make ze-rfc-index-update` after the new tag
- `plan/journal/stale-artifact-reused.md` - DONE 2026-08-19, the 2026-08-12 row's Fix cell records the fix instead of "Not fixed"
- `internal/component/ike/engine/bypass.go` - DONE 2026-08-17, `removeIKEBypass`'s godoc no longer says it runs once at shutdown: it runs on every exit, from the deferred cleanup
- `internal/component/plugin/process/manager.go` - DONE 2026-08-17, `ProcessManager.Stop` waits `pluginStopGrace` (the daemon's own 3s shutdown budget) for each plugin goroutine and NAMES any plugin that misses it. It waited 500ms and discarded the timeout, so a release slower than that was lost with nothing logged. This is what made AC-5 fail in QEMU: the IKE engine's eight XFRM bypass policy deletes run after its read loop ends, and the daemon exited first (MEASURED: a release delayed 700ms leaves `RESIDUE: policies=8`)

## Files to Create
- `test/ipsec/ipsec-teardown-leaves-nothing.ci` - DONE 2026-08-17, functional proof that teardown leaves no residue, plus the `fsuite ipsec` line in `scripts/evidence/qemu-all-tests.sh` that makes it execute

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface changes |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-teardown-leaves-nothing.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency, the XFRM dependency already has one |
| Prometheus counters/metrics | No | No counter is added. The SA-up-minus-SA-down gauge considered for leak 3 would measure the symptom rather than the state: `internal/component/ike/engine/metrics.go` already exports `ze_ipsec_sa_count` and `ze_ipsec_tunnel_up` from the SA table, which is the state itself, so a gauge derived from event arithmetic can only ever disagree with it or repeat it. The drift the gauge would have shown is what `TestSAUpAndSADownBalanceAcrossReconnects` asserts directly |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Defect fix, no new feature |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | The guide page is `docs/guide/ipsec.md`, not `docs/guide/vpn/`, which does not exist. It documents the `vpn { ipsec {} }` configuration and the `show` / `clear` surface. No leaf, command, output field, or default changed, so nothing on that page becomes wrong |
| 7 | Wire format changed? | No | No wire-visible change |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `RFC7296-2.12-1` gains a second positive test, `TestRunEstablishedClearsPendingIKESwapOnExit`. `docs/features/rfc-status.md` updated: its Section 2.12 sentence now names the two session-held holders released on the same exit. `rfc/requirements/rfc7296.md` regenerated by `make ze-rfc-index-update`. No row changes level, polarity, or enrolment, so no ratchet fires; `make ze-rfc-check` is green (2966 gated MUST-level requirements, 3559 tags resolved) |
| 10 | Test infrastructure changed? | No | The `.ci` joins the EXISTING `test/ipsec/` suite, and the `fsuite ipsec` line added to `scripts/evidence/qemu-all-tests.sh` makes that suite execute in the QEMU phase. `docs/functional-tests.md` already documents the suite list (`ipsec` is named at `:76`) and the `scripts/evidence/qemu-all-tests.sh` mechanism, so no page changes. No runner, harness, directive, or make target was added |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | No counter added, per the Integration Checklist row above. `internal/component/ike/engine/metrics.go` reads the live SA table in `Update`, so nothing it publishes is derived from `SAUp` / `SADown` arithmetic and the event pairing fix changes no metric value |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `SAUp` and `SADown` already exist |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grepped `docs/` for every changed file. `docs/architecture/ike/ipsec-14-responder.md` anchors `reconcile.go -- setPendingIKESwap`, and its "IKE rekey responder makes before breaking" paragraph described the fill and the swap without the release. Updated, with an anchor added for `established.go`'s teardown defer. The remaining anchors are directory-level (`internal/component/ike/engine/` in `docs/features.md`) or name symbols this spec does not touch (`fsm.go -- runResponder, reapStaleHandshake`; `reconcile.go -- setSA, getSA`), so no other page describes behavior that changed |
| 17 | Existing docs show config/CLI/API examples for this area? | No | `docs/guide/ipsec.md` carries the `vpn { ipsec {} }` examples and `docs/features.md` lists the `show vpn ipsec` / `clear vpn ipsec` surface. Every example stays correct: no YANG leaf, command, flag, JSON key, or output field is added, removed, or renamed by any of the five fixes |

## Implementation Steps

1. **Phase: Evidence (MANDATORY FIRST)** - measure all three leaks before fixing any
   - Tests: the four unit tests above, written to FAIL against current code
   - Files: the three test files named in the TDD plan
   - Verify: each test reproduces its leak, so A-1 through A-4 move off `unvalidated`
2. **Phase: Leak 1** - forget the ESP form on every exit of `RemoveSA`
   - Tests: `TestRemoveSAForgetsESPFormWhenStateDeleteFails`
   - Files: `internal/component/ike/dataplane/xfrm_linux.go`
   - Verify: the delete error still reaches the caller, and the map entry is gone
3. **Phase: Leak 2** - remove the bypass on every error exit of `Run`
   - Tests: `TestRunRemovesIKEBypassOnEveryErrorExit`
   - Files: `internal/component/ike/engine/register.go`
   - Verify: R-3 is settled first, so removal cannot strip a bypass another peer needs
4. **Phase: Leak 3** - pair the initiator emit
   - Tests: `TestInitiatorEmitsSADownWhenEstablishedLoopReturns`, `TestSAUpAndSADownBalanceAcrossReconnects`
   - Files: `internal/component/ike/engine/fsm.go`
   - Verify: no double emit, checked against every existing `emitSADown` caller
5. **Phase: Kernel proof** - the functional test on a real kernel
   - Tests: `ipsec-teardown-leaves-nothing`
   - Files: `test/ipsec/ipsec-teardown-leaves-nothing.ci`
   - Verify: runs in QEMU per `ai/rules/platform-linux.md`, and reddens when any fix is reverted
6. **Phase: Leak 5** - release the unconfirmed IKE swap SA on every exit
   - Tests: `TestRunEstablishedClearsPendingIKESwapOnExit`
   - Files: `internal/component/ike/engine/established.go`, `internal/component/ike/engine/reconcile.go`
   - Verify: the release goes in the EXISTING teardown defer rather than a new one, so the defer chain's order is unchanged and the lock argument the `pendingRekey` comment already makes carries over unaltered; the normal swap still promotes, which `TestSetPendingIKESwapClearsSuperseded` (`responder_test.go`), the IKE-rekey rows of `rfc7296_negotiation_test.go` and the `09-responder-ike-rekey` interop scenario all check

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | All five leaks fixed, not four. Leak 5 is in scope because this spec's goal is that no IKE resource outlives its session, and it is in the exact function the spec already edits |
| Holder enumeration | Every holder `forgetKeys` cannot reach is named. `gopls references` on `setPendingIKESwap` and on `pendingRekey`, not grep: a comment claiming "the one holder" is what missed the second one |
| Correctness | The `RemoveSA` error still propagates unchanged after the forget moves |
| Naming | The new `.ci` name states the invariant, not the mechanism |
| Data flow | The bypass removal runs once per process, never per peer |
| Rule: `ai/rules/evidence.md` | No cleanup left below an early return anywhere in the three functions |
| Rule: `ai/rules/go-standards.md` | The `SAUp` godoc states the `SADown` pairing obligation |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| ESP form forgotten on delete failure | `go test -run TestRemoveSAForgetsESPForm ./internal/component/ike/dataplane/` |
| Bypass removed on every error exit | `go test -run TestRunRemovesIKEBypass ./internal/component/ike/engine/` |
| Initiator events balanced | `go test -run TestSAUpAndSADownBalance ./internal/component/ike/engine/` |
| No residue after teardown | `make ze-qemu-needs-linux-test` with the new `.ci` |
| Unconfirmed IKE swap SA released | `make ze-unit-pkg-test PKG=./internal/component/ike/engine RUN=TestRunEstablishedClearsPendingIKESwapOnExit` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | None of the three fixes takes new external input |
| Resource exhaustion | Leak 1 and leak 2 ARE the resource exhaustion this spec closes. Confirm the fix bounds them rather than slowing them |
| Fail-open authorization | A bypass policy that outlives its process is a fail-open state. Confirm removal cannot itself fail silently |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Leak 4: `sendDPD` returns before `reserveRequestWindow` | Reserve first and release on the no-send-path branch | A release that a later edit forgets recreates the leak. A path that takes nothing needs no release, and it also stops the Message ID being spent on a message nobody sends |
| Leak 4: the guard reads `sa.sendPath(tr)`, not `tr` | Guard on the `tr` argument | `tr` is a fallback. `sendPath` prefers the SA's own socket, so a floated SA sends while `tr` is nil. A guard on the argument silences liveness for the life of a working NAT-T tunnel |

## Known Limitations
- Phase 1 is evidence, so no fix lands until each leak is measured rather than read.
- Leak 4's remaining edge is FIXED (2026-08-18), not deferred. The 2026-08-07 ask was
  itself the error: `ai/rules/rfc-compliance.md` reserves the question for a choice that
  does LESS than the RFC, and full compliance was reachable here, so it was owed
  implementation rather than an owner decision.

  `maintainSA` reads `dpd.timedOut(now)` before `shouldRetransmit` in the same tick, and
  `shouldRetransmit` refuses once `timedOut`. A `dead-peer-detection timeout 1`, the
  smallest value `parseDPD` accepts, therefore reached the dead-peer verdict after ONE
  attempt and no repeat. RFC 7296 Section 2.4 MUST: an endpoint concludes the other one
  has failed "only when repeated attempts to contact it have gone unanswered for a
  timeout period". The same section's "The number of retries and length of timeouts are
  not covered in this specification" chooses how MANY repeats an implementation makes; it
  never licenses zero of them.

  `timedOut` (`internal/component/ike/engine/dpd.go`) now requires the elapsed budget AND
  `retries > 0`. The second candidate, flooring `timeout` at 2 seconds in `parseDPD`, was
  rejected: it rewrites an operator's configured value, and it makes the violation
  unlikely by timing rather than impossible by construction. The one case that still ends
  on the budget alone is a probe with no stored datagram, which can never be repeated;
  waiting for a repeat there would hold the SA and its request window open for ever.

  Proven by `TestDPDVerdictNeedsARepeatedAttempt` and
  `TestDPDVerdictEndsAProbeThatCannotBeRepeated` (`dpd_test.go`, both tagged
  `RFC7296-2.4-11`). `TestSesPeerFailedOnlyAfterRepeatedSilence` asserted the verdict on
  the elapsed budget alone, which is the shape the requirement refuses; that assertion is
  corrected to establish a repeat first, and it is strengthened rather than weakened.
  Reverting `timedOut` reddens both.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ike-resource-lifetime-leaks-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring and entry-point reachability, removed-behaviour audit, logic and guard correctness, security and resource exhaustion, RFC 7296 conformance and tag polarity, interop and goal validation, test discrimination by source mutation |

Round 1 found 2 BLOCKER and 5 ISSUE and recorded them as fixed. It could not record a
clean verdict, because the session that made those fixes is not independent of them.

Round 2 ran in `/ze-close`, independent of the session that wrote the code and of the
session that recorded round 1. It re-measured every claim the verdict rests on rather
than reading round 1's record. Round 1's artifact no longer exists: it lived under
`tmp/`, which is untracked, so nothing in round 2 cites it.

All seven round-1 findings are confirmed fixed IN THE TREE. The implementation is
committed and pushed, so B-1 is moot rather than pending. `ps.pendingIKESwap` is
released in `runEstablished`'s teardown defer and `TestRunEstablishedClearsPendingIKESwapOnExit`
reddens on all four assertions when that call is deleted, which settles B-2 by
measurement. The interop scenarios live under `test/interop-ipsec/scenarios/` and both
`09-responder-ike-rekey` and `10-clear-reestablish` were executed again and PASS, which
settles I-1. The Architectural Verification and Boundaries Crossed tables carry evidence
in every cell (I-2, I-3), no "decide at design time" placeholder survives anywhere in
the spec (I-4), and the Functional Tests row states plainly that the `.ci` does NOT
discriminate AC-2's fix and names the unit test that does (I-5).

Every discrimination measurement mutated PACKAGE SOURCE, which changes the `go test`
cache key, so none could be served a cached verdict and none needed `-count=1`. The
interop runs are Docker executions with fresh SPIs in their output, which no cache can
serve.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | Round 2 found 0 BLOCKER and 0 ISSUE | - | - |

Round 2 recorded three NOTEs, which do not block. First, the Files to Modify entry for
`internal/component/plugin/process/manager.go` reads DONE 2026-08-17 while the change
landed on 2026-08-18. Second, the Review Gate table named a round-1 artifact under
`tmp/`, which no longer exists; round 2 records a fresh one. Third, no interop scenario
can redden for leaks 1, 2, 3 or 5, and that is a property of the defects rather than a
gap in the evidence: leaks 1 and 2 are kernel-local, leak 3 is on the internal event
bus, and leak 5's window between Ze answering the peer's rekey and the peer's Delete is
milliseconds against a real strongSwan and cannot be opened without adding a
fault-injection seam to the daemon. The spec states this correctly and names the unit
test as the proof, so it makes no vacuous claim and the two scenarios stand as the
regression bound they are described to be.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- Five releases, each moved onto the exit path that always runs. `RemoveSA`
  (`internal/component/ike/dataplane/xfrm_linux.go`) defers `espForms.Forget`, so a
  kernel delete that fails still releases the SPI and the delete error still reaches
  the caller. `runEngine` (`internal/component/ike/engine/register.go`) defers
  `removeIKEBypass` plus `dataplane.CloseBackend` beside `installIKEBypass`.
  `runInitiator` (`internal/component/ike/engine/fsm.go`) calls `emitSADown` and clears
  `ps.sa` when its established loop returns, which is what `runResponder` already did.
  `sendDPD` (`internal/component/ike/engine/dpd.go`) returns before
  `reserveRequestWindow` when `sa.sendPath(tr)` answers nil. `runEstablished`
  (`internal/component/ike/engine/established.go`) releases `ps.pendingIKESwap` in the
  defer that already released `ps.pendingRekey`.
- One supporting fix outside IKE. `ProcessManager.Stop`
  (`internal/component/plugin/process/manager.go`) waits `pluginStopGrace` for each
  plugin ENGINE and names any plugin that misses it. The old code waited 500ms on the
  goroutine group and discarded the timeout, so the eight XFRM bypass deletes that now
  run on the way out of `runEngine` were lost with nothing logged. It carries no IKE
  spelling: `pluginStopGrace` applies to every plugin.

### Bugs Found/Fixed
- Leak 5 was not in the spec when it was written. An independent review on 2026-08-19
  found it, and `plan/journal/stale-artifact-reused.md` had carried it since 2026-08-12
  marked "Not fixed". It is the same rule in the same function, so it was fixed here
  rather than counted again. Covered by `TestRunEstablishedClearsPendingIKESwapOnExit`.
- The deferred release in `runEngine` moved the bypass deletes AFTER the plugin read
  loop ends, which the 500ms wait in `ProcessManager.Stop` then discarded. Covered by
  `TestStopWaitsForAPluginReleaseThatOutlastsItsReadLoop` and
  `TestStopNamesThePluginThatMissesItsCleanupGrace`.

### Documentation Updates
- `docs/architecture/ike/ipsec-14-responder.md`: the make-before-break paragraph now
  states where the pending slot is released. Source anchors for
  `reconcile.go -- setPendingIKESwap` and the `established.go` teardown defer.
- `docs/features/rfc-status.md`: the RFC 7296 Section 2.12 sentence names both
  session-held holders.
- `rfc/requirements/rfc7296.md`: GENERATED, refreshed by `make ze-rfc-index-update`.
- `make ze-doc-verify` on 2026-08-22 exits 2 on one line, `ai/DOCS-TO-CODE.md is stale`.
  That file is generated from `// Design:` headers in `.go` files and is gitignored,
  and the working tree holds three other sessions' uncommitted `.go` edits. No
  documentation claim of this spec is implicated, and nothing this closure commits
  feeds that index.

### Deviations from Plan
- Phase 1 was written as "measure all three leaks before fixing any". A-1 was measured
  and came back BROKEN in the half that matters, which widened leak 1 rather than
  closing it. The fix therefore releases on EVERY exit rather than on the second call.
- Leak 5 was added after the spec was approved, so the spec carries five leaks while
  the Task section still opens by naming three.
- `plan/learned/NNN-<name>.md` no longer exists as a destination. The lesson is a row in
  `plan/journal/green-that-could-not-have-been-red.md`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 said the double remove in `delete.go` is what strands the ESP form | The FIRST remove succeeds and already runs Forget, so the second finds nothing left. The stranding case is a first remove the kernel refuses | measured 2026-08-17 | The fix releases on every exit, and both new tests drive a delete the kernel refuses |
| approach | The closure review mutated `pluginStopGrace` to its historical 500ms to test discrimination, read green, and nearly recorded a vacuity BLOCKER | The pre-fix code had NO engine wait at all, so its effective budget was 500ms and not 1000ms. The true revert is grace 0, and the test reddens there at 0.50s | measured 2026-08-22 during this closure | Row in `plan/journal/green-that-could-not-have-been-red.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A resource acquired on the way in is released on EVERY way out | Done | the five call sites named in Implementation Summary | Four of the five are a call moved onto a `defer` |
| The delete error still reaches `RemoveSA`'s caller | Done | `internal/component/ike/dataplane/xfrm_linux.go` `RemoveSA` | The `defer` runs after the return value is set, so the error is unchanged |
| `removeIKEBypass` stays idempotent | Done | `internal/component/ike/engine/bypass.go` `removeIKEBypass` | A removal that finds nothing logs at Debug and continues |
| `runResponder` pairing is not disturbed | Done | `internal/component/ike/engine/fsm.go` `runResponder` | Unchanged; `runInitiator` copies it |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRemoveSAForgetsESPFormWhenStateDeleteFails`, `TestXFRMDoubleTeardownLeavesNothing` | Both are Linux-gated and execute on the QEMU rail |
| AC-2 | Done | `TestRunRemovesIKEBypassOnEveryErrorExit` | Reverting the defer reddens 9 assertions |
| AC-3 | Done | `TestInitiatorEmitsSADownWhenEstablishedLoopReturns` | Asserts the ORDER, one up then one down |
| AC-4 | Done | `TestSAUpAndSADownBalanceAcrossReconnects` | 4 cycles, counts equal |
| AC-5 | Done | `TestXFRMDoubleTeardownLeavesNothing`, `test/ipsec/ipsec-teardown-leaves-nothing.ci` | Kernel half and daemon half |
| AC-6 | Done | `TestDPDNoTransportTakesNoWindow` | No window, no awaitReply, no probe, no Message ID |
| AC-7 | Done | `TestDPDFloatedSAProbesWithoutTheFallback` | The predicate is `sendPath`, never `tr` |
| AC-8 | Done | `TestRunEstablishedClearsPendingIKESwapOnExit` | Tagged `RFC7296-2.12-1` positive |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRemoveSAForgetsESPFormWhenStateDeleteFails` | Done | `internal/component/ike/dataplane/xfrm_state_del_failure_linux_test.go` | `//go:build linux`, needs no capability |
| `TestRunRemovesIKEBypassOnEveryErrorExit` | Done | `internal/component/ike/engine/register_test.go` | Drives `runEngine` through a broken plugin pipe |
| `TestInitiatorEmitsSADownWhenEstablishedLoopReturns` | Done | `internal/component/ike/engine/sa_event_pairing_test.go` | |
| `TestSAUpAndSADownBalanceAcrossReconnects` | Done | `internal/component/ike/engine/sa_event_pairing_test.go` | |
| `TestXFRMDoubleTeardownLeavesNothing` | Done | `internal/component/ike/dataplane/xfrm_teardown_integration_linux_test.go` | `//go:build integration && linux` |
| `TestDPDNoTransportTakesNoWindow` | Done | `internal/component/ike/engine/dpd_test.go` | |
| `TestDPDFloatedSAProbesWithoutTheFallback` | Done | `internal/component/ike/engine/dpd_test.go` | |
| `TestRunEstablishedClearsPendingIKESwapOnExit` | Done | `internal/component/ike/engine/established_test.go` | |
| `ipsec-teardown-leaves-nothing` | Done | `test/ipsec/ipsec-teardown-leaves-nothing.ci` | `option=needs-linux:caps=net-admin` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 13 files in Files to Modify | Done | Each verified present at HEAD by reading the named symbol on 2026-08-22 |
| `test/ipsec/ipsec-teardown-leaves-nothing.ci` | Done | Plus the `fsuite ipsec` line in `scripts/evidence/qemu-all-tests.sh` |

### Audit Summary
- **Total items:** 8 acceptance criteria, 9 tests, 14 files
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** A-1 broken, and leak 5 added after approval. Both are in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| No IKE resource outlives the thing that owns it | unit, driven from each entry point | Every release reddens its test when reverted. Measured 2026-08-22: AC-2 nine assertions, AC-3 and AC-4 both tests, AC-6 six assertions, AC-7 one, AC-8 four, plugin grace one |
| A stranded ESP form no longer holds a host-wide raw ESP socket | functional, on a real kernel | `TestXFRMDoubleTeardownLeavesNothing` asserts `espForms.running()` is false after the last watched SA goes, and `ipsec-teardown-leaves-nothing.ci` reads `/proc/net/raw` for IPPROTO_ESP after both daemons stop |
| Node-wide bypass policies do not survive the daemon | functional, on a real kernel | `ipsec-teardown-leaves-nothing.ci` asserts `RESIDUE: none` after `cmd=stop:signal=term` on both daemons, and the `fsuite ipsec` line executes that suite in the QEMU VM |
| Ze still interoperates across the paths these releases touch | interop, strongSwan | `09-responder-ike-rekey` and `10-clear-reestablish` both PASS, executed 2026-08-22 on this checkout. Scenario 10 moved the ESP SPIs from `['0xaf996ef1', '0xcb282102']` to `['0xc00dde2b', '0xe07ef29f']` inside 30 seconds, which is R-2's guard: the new `ps.setSA(nil)` did not break the reconnect |
| RFC 7296 Section 2.12 is proven, not asserted | RFC-tagged test, both polarities | `RFC7296-2.12-1` carries a positive and a negative tag in `rfc7296_wp2_test.go`, and this spec adds a second positive in `established_test.go`. `make ze-rfc-check` green on 2026-08-22: 2966 gated MUST-level requirements, 3589 tags resolved, no ratchet fired |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| This spec declares no shard of its own (`Deferral shard: -`) | n/a | `ls plan/deferrals/fixit-ike-resource-lifetime-leaks.md` reports no such file on 2026-08-22 |
| `plan/deferrals/fixit-ike-dpd-cleartext.md` row 2, the `sendDPD` request window, homed at this spec | done | Fixed 2026-08-07 as leak 4, proven by `TestDPDNoTransportTakesNoWindow` and `TestDPDFloatedSAProbesWithoutTheFallback` |
| `plan/deferrals/fixit-ike-dpd-cleartext.md` row 3, `RFC7296-2.4-2` demand-driven liveness | deferred | Homed at `plan/spec-ike-dpd-demand-driven.md`, which exists. That shard still holds this live row, so it is NOT removed here |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ipsec/ipsec-teardown-leaves-nothing.ci` | Yes | `ls -1` on 2026-08-22, 11K |
| `internal/component/ike/dataplane/xfrm_state_del_failure_linux_test.go` | Yes | `ls -1` on 2026-08-22, 2.2K |
| `internal/component/ike/dataplane/xfrm_teardown_integration_linux_test.go` | Yes | `ls -1` on 2026-08-22, 5.7K |
| `internal/component/ike/engine/sa_event_pairing_test.go` | Yes | `ls -1` on 2026-08-22, 7.3K |
| `test/interop-ipsec/scenarios/09-responder-ike-rekey/check.py` | Yes | `ls -1` on 2026-08-22, 2.9K |
| `test/interop-ipsec/scenarios/10-clear-reestablish/check.py` | Yes | `ls -1` on 2026-08-22, 3.6K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The SPI leaves `espForms` on a failed delete, and the error is unchanged | `defer b.espForms.Forget(spi)` sits above the delete in `RemoveSA`, read at HEAD on 2026-08-22. Both tests are Linux-gated, so neither runs on darwin. `GOOS=linux go vet`, with and without the `integration` tag, exits 0 on the package, and the QEMU rail executes them through its unit phase and its derived integration package list |
| AC-2 | No bypass policy survives an error exit | `TestRunRemovesIKEBypassOnEveryErrorExit` PASS. Deleting the deferred release makes it report all 8 policies left installed, plus `released 0 distinct policies, start installed 8` |
| AC-3 | Exactly one SADown follows the one SAUp | `TestInitiatorEmitsSADownWhenEstablishedLoopReturns` PASS. Deleting `emitSADown` plus `setSA(nil)` makes it report `[vpn-ipsec/sa-up]` where it wants an up then a down |
| AC-4 | The counts stay equal across reconnects | `TestSAUpAndSADownBalanceAcrossReconnects` PASS. The same mutation gives `4 sa-up and 0 sa-down`, a drift of 4 |
| AC-5 | Nothing of the tunnel is left on a real kernel | `TestXFRMDoubleTeardownLeavesNothing` asserts the state, the policy, the watched SPI and the raw sockets, and it deletes the state behind the backend so the FIRST teardown fails. `ipsec-teardown-leaves-nothing.ci` asserts `RESIDUE: none` after both daemons stop |
| AC-6 | A probe with no send path takes nothing | `TestDPDNoTransportTakesNoWindow` PASS. Deleting the guard reddens 6 assertions, `Message ID moved to 3, want 2` among them |
| AC-7 | A floated SA probes with a nil fallback | `TestDPDFloatedSAProbesWithoutTheFallback` PASS. Rewriting the guard as `if tr == nil` reddens it with `no datagram arrived at the peer`, which is the black hole the comment names |
| AC-8 | The unconfirmed swap SA is released and erased | `TestRunEstablishedClearsPendingIKESwapOnExit` PASS. Deleting `ps.setPendingIKESwap(nil)` reddens all four assertions: the slot is occupied, SK_d is intact, the EAP MSK is intact, and both nonces are still referenced |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Second teardown of one Child SA | `test/ipsec/ipsec-teardown-leaves-nothing.ci` | Yes. Read the file: it dispatches `clear vpn ipsec sa` twice over the same peer, then `cmd=stop:signal=term` on both daemons, then `residue.py` |
| Daemon start that fails after bypass install | none, unit only | Yes. `TestRunRemovesIKEBypassOnEveryErrorExit` drives `runEngine` itself through a closed plugin pipe and asserts code 1, so the error exit IS the entry point |
| Initiator tunnel that goes down | none, unit only | Yes. `TestInitiatorEmitsSADownWhenEstablishedLoopReturns` drives `runInitiator` and delivers a real peer IKE Delete to it |
| Full teardown on a real kernel | `test/ipsec/ipsec-teardown-leaves-nothing.ci` | Yes. The `fsuite ipsec` line in `scripts/evidence/qemu-all-tests.sh` executes the suite in the VM, serially, and says why |
| A session ending on an unconfirmed IKE swap | none, unit only | Yes. `TestRunEstablishedClearsPendingIKESwapOnExit` calls `ps.runEstablished` with `stopCh` already closed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | The second remove does fail, but the FIRST already ran Forget. The stranding case is a first remove the kernel refuses, which `TestXFRMDoubleTeardownLeavesNothing` constructs by deleting the state behind the backend |
| A-2 | confirmed | `espFormReceiver.Forget` (`dataplane/espform_linux.go`) releases the raw sockets only when the last watched SPI goes. The kernel test asserts `espForms.running()` is false afterwards |
| A-3 | confirmed | `removeIKEBypass` (`engine/bypass.go`) iterates `ikeBypassFamilies` and `ikeBypassPolicies(family)`, and neither carries a peer identity. Read at HEAD on 2026-08-22 |
| A-4 | confirmed | `grep -rn "SAUp.Emit"` over non-test `internal/` returns exactly two producers, both in `fsm.go`, and each is paired at the same level. An IKE rekey swap emits no new SAUp, so a rekey cannot unbalance the count |
| A-5 | confirmed | `grep -rn "pendingIKESwap"` over non-test `internal/` returns the promotion in `inbound.go` and `setPendingIKESwap` in `reconcile.go`. Nothing else empties the slot |
| A-6 | confirmed | Both fields carry the same no-lock ownership by `maintainSA`, and the release sits in the defer that already released `pendingRekey`. `make ze-unit-pkg-test PKG=./internal/component/ike/...` is race-instrumented and green, with the engine package taking 30.9s |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 9, RFC behaviour newly proven | `RFC7296-2.12-1` carries a positive and a negative tag, and `make ze-rfc-check` is green with 3589 tags resolved and no ratchet fired | Yes |
| Row 16, changed files behind doc source anchors | `docs/architecture/ike/ipsec-14-responder.md` anchors `reconcile.go -- setPendingIKESwap` and was updated. `docs/architecture/ike/ipsec-7-ikev2-engine.md` is the design doc `events.go` and `fsm.go` declare, and it now states the pairing obligation | Yes |
| `docs/architecture/api/process-protocol.md`, the design doc `manager.go` declares | Its daemon-shutdown row already names `pluginStopGrace` and the group wait after it, so the page describes the code this spec left behind | Yes, no edit owed |
| Rows 1 to 8, 10 to 15, 17, all No | `make ze-repository-check` green on 2026-08-22, which runs the stale-source-anchor and spec-AC checks | Yes |
| `make ze-doc-verify` | Exit 2 on `ai/DOCS-TO-CODE.md is stale`. That file is gitignored and generated from `// Design:` headers in `.go` files, and three other sessions hold uncommitted `.go` edits in this checkout | Yes, and not charged to this spec |

## Core Insight

The rule this spec enforces is one line: a resource acquired on the way in is released
on every way out. What made it break five times is that the release was written where
the author was LOOKING, which is the success path, and not where the resource was
TAKEN. Four of the five fixes are the same edit, moving the call onto a `defer`
registered beside the acquire, and after that edit no future early return can skip it.
The fifth, `sendDPD`, is stronger: it returns BEFORE it reserves, so the branch takes
nothing and has nothing to give back. A path that acquires nothing cannot leak it.
