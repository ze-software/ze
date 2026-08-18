# Spec: fixit-ike-resource-lifetime-leaks

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | `-` |
| Updated | 2026-08-17 |

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

**Behavior to preserve:** (unless the user explicitly said to change it)
- `RemoveSA` still returns the `XfrmStateDel` error to its caller. Forgetting the ESP form must not swallow the failure.
- `removeIKEBypass` stays idempotent, so running it on a path that already ran it is harmless.
- `SAUp` and `SADown` payload shapes stay as they are. Subscribers outside IKE read them.
- `runResponder` pairing is already correct and must not be disturbed.

**Behavior to change:** (only what the user asked for)
- `RemoveSA` forgets the ESP form whether or not the state delete succeeded.
- Every `p.Run` error exit removes the IKE bypass policies.
- The initiator path emits `SADown` when its established loop returns.

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
| Engine ↔ Dataplane backend | `dataplane.Dataplane` interface calls | No |
| Backend ↔ Kernel | netlink XFRM state and policy messages | No |
| Engine ↔ Event bus | `SAUp` / `SADown` typed events | No |
| Process ↔ Node | bypass policies that outlive the process | No |

### Integration Points
- `internal/component/ike/dataplane` - the backend that owns `espForms`.
- `internal/core/events` - the typed bus carrying `SAUp` and `SADown`.
- Any subscriber counting SA lifecycle events, which is what leak 3 misleads.

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
| A-1 | The double-remove in `delete.go` really does make `XfrmStateDel` fail in ordinary operation | read, `engine/delete.go` | Leak 1 is rare rather than routine, which lowers its priority but does not close it | a QEMU run counting `espForms` entries after a tunnel closes twice | broken 2026-08-17 |
| A-2 | A stranded `espForms` entry holds a real host-wide raw ESP socket open | read, `dataplane/xfrm_linux.go` | Leak 1 is a map leak only, far cheaper than stated | inspect the socket table in QEMU after the repro | confirmed 2026-08-17 |
| A-3 | The four bypass policies are node-wide and survive process exit | read, `engine/register.go` `removeIKEBypass` | Leak 2 is bounded by the process and self-clears | `ip xfrm policy` in QEMU after killing ze on an error path | confirmed 2026-08-17 |
| A-4 | No subscriber compensates for the missing initiator `SADown` elsewhere | read, `engine/fsm.go`, `engine/reconcile.go` | Leak 3 is already handled and only the direct path is unbalanced | grep every `SADown` producer and every subscriber | confirmed 2026-08-17 |

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
| R-2 | Emitting `SADown` on the initiator path double-emits where a caller already emits | subscribers see two downs per up | grep every `emitSADown` caller before adding one, and add the emit at the single return point |
| R-3 | Removing bypass policies on an early error path removes them while another peer still needs them | IKE traffic starts entering IPsec processing | confirm the bypass is process-wide and installed once, not per-peer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An over-eager bypass removal sends IKE control traffic into IPsec processing and stops negotiation node-wide. A double `SADown` misleads every lifecycle subscriber. |
| How is it reverted? | Single commit revert. No config migration, no wire-visible change. |
| Who else touches this path? | `plan/spec-fixit-child-sa-rekey-policy.md` (policy teardown), `spec-fixit-vpp-ipsec-inoperable` (the other backend), `plan/spec-lifecycle-invariants.md` (event pairing in the subscriber namespace) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Second teardown of one Child SA | → | `RemoveSA` forget-on-error path | `TestRemoveSAForgetsESPFormWhenStateDeleteFails` |
| Daemon start that fails after bypass install | → | `p.Run` error exit | `TestRunRemovesIKEBypassOnEveryErrorExit` |
| Initiator tunnel that goes down | → | `runInitiator` teardown | `TestInitiatorEmitsSADownWhenEstablishedLoopReturns` |
| Full teardown on a real kernel | → | XFRM state and policy tables | `test/ipsec/ipsec-teardown-leaves-nothing.ci`, and `fsuite ipsec` in `scripts/evidence/qemu-all-tests.sh` is what runs it |
| A DPD tick on an SA with no send path | → | `sendDPD` early return | `TestDPDNoTransportTakesNoWindow` |
| A DPD tick on a floated SA with a nil fallback | → | `sendDPD` send-path predicate | `TestDPDFloatedSAProbesWithoutTheFallback` |

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

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| reconnect cycles counted for event balance | 1..N | N | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-teardown-leaves-nothing` | `test/ipsec/ipsec-teardown-leaves-nothing.ci` | After the operator tears the tunnel down and both daemons stop, no XFRM state, no XFRM policy and no raw ESP socket remains | PASS 2026-08-17. `fsuite ipsec` added to `scripts/evidence/qemu-all-tests.sh`, so the suite now executes in the QEMU VM where `option=needs-linux:caps=net-admin` is satisfied. Two daemons, one real xfrm backend: the kernel keys a state on (src, dst, spi, proto) and the two ends of one Child SA are the SAME two states, so a second real backend would answer EEXIST (measured). Discriminates: with `RemoveSA`'s state delete removed the clear leaves both states behind and the test reddens, and with the bypass release removed the test reports `RESIDUE: states=0 policies=8`. EXECUTED, twice: natively in an unprivileged user+net namespace (`unshare -rn`, which satisfies caps=net-admin without root), and in the QEMU VM through the new `fsuite ipsec` line, where the whole suite passed 16/16 in 180s |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `05-child-rekey` | `test/interop/scenarios/` | strongSwan | Repeated teardown against a real peer leaves no residue | |

## Files to Modify
- `internal/component/ike/dataplane/xfrm_linux.go` - DONE 2026-08-17, `RemoveSA` defers `espForms.Forget` so every exit releases it, and the delete error still reaches the caller
- `internal/component/ike/engine/register.go` - DONE 2026-08-17, `runEngine` defers `removeIKEBypass` plus `dataplane.CloseBackend` beside the install, and the shutdown tail no longer repeats them
- `internal/component/ike/engine/fsm.go` - DONE 2026-08-17, `runInitiator` calls `emitSADown` and clears `ps.sa` when its established loop returns, as `runResponder` does
- `internal/component/ike/engine/events.go` - DONE 2026-08-17, the `SAUp` / `SADown` godoc states the pairing obligation with MUST on both sides
- `internal/component/ike/engine/dpd.go` - DONE 2026-08-07, `sendDPD` returns before it reserves the request window when the session has no transport
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
| Prometheus counters/metrics | | Decide at design time whether an SA-up minus SA-down gauge is the right early signal for leak 3 |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Defect fix, no new feature |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | | Check `docs/guide/vpn/` at design time |
| 7 | Wire format changed? | No | No wire-visible change |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | | Decide at design time whether any RFC 7296 row changes status |
| 10 | Test infrastructure changed? | | New `.ci` in an existing suite, confirm at design time |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | | Depends on the Integration Checklist row above |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `SAUp` and `SADown` already exist |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for the three changed files at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify at design time |

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

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | All three leaks fixed, not two |
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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
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
