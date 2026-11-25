# Spec: pppoe-vpp-dataplane -- terminate PPPoE subscriber sessions in the VPP data plane

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P2 |
| Release reason | Owner decision: Ze terminates PPPoE subscriber sessions in VPP, after the first release (OD-5, decided 2026-10-28). The shipped-behaviour fix that was Phase 1 (validate accepts PPPoE under the VPP backend and every session then fails) moved to `spec-pppoe-refuse-vpp-backend` |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-vpp-failure-lifecycle.md` (OD-2 and AC-12 consume its form-1 continuation inside the timer and its declared-loss event) |
| Phase | 2/7 (shared PPP seam; full product acceptance remains owed) |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Checkpoint for dependent work — 2026-11-19

This is an implementation checkpoint, not feature closure. The shared kernel
foundation is committed in `668311841c`, and compiled capability recognition
in `d589eead60`. Implementation checkpoint `83b7c9c3c9` commits the prepared-access
seam, composed VPP provider, guarded server/CPE consumers, VPP process ownership
and their current tests across 288 changed files. Other work can use these source contracts while preserving the
validation limits below. The VPP provider is composed; it is not fully accepted.

| Contract or remaining work | Resume instruction |
|----------------------------|--------------------|
| Prepared access | Session construction uses the retained `Access.Open` contract. Include the provider registrations and generated composition root when changing this seam. |
| Native PPP writes | Bind the same created unit and backend before mutation or native birth/seal assignment. Do not reconstruct authority from a name, index or descriptor number. |
| Cleanup | Take final counters from the original unit and disconnect before closing. Release the genuine remote seal/duplicate before local writer settlement. `NativeReleasePending` includes uncertain original-file, borrow and duplicate retirement independently of writer-socket closure. |
| Current scoped evidence | All eight consumer package/race runs passed before the final diagnostic/projection additions. Their final focused tests, CPE SDK diagnostics and eight override-fidelity roots passed together in `job-unit-pkg-66932b38.log`. These are host results, not genuine Linux fault acceptance. |
| Native validation still owed | The CPE fault, CPE registry and native access consumers now run and pass on the host Ze Linux 7.3 kernel under race (2026-11-22, `bc776b7a2f`: five judged fixture defects corrected). Retirement controls done 2026-11-22. These pass under host race and in the Ze guest: the five logical roots L1-L5 `TestSessionNativeReleaseProjection{RetainsOriginalCloseFailure,JoinsOriginalBorrow,RetainsDuplicateCloseFailure,IgnoresNamespaceHistory,PendingSealDefersSettlement}`, the three genuine roots `...{FailedOriginalClose,ConfirmedOriginalClose,PendingSealDefersLocalSettlement}`, the corrected original-duplicate fixture `TestCreatedUnitNativeRetirementJoinsRetainedOriginalDuplicate` and the genuine borrow root. The eight causal cuts are in "Retirement causal cuts" below. |
| Provider history defect | Repaired in `0c93a81fd2` (2026-11-22). `Settle` keeps a delayed settlement's causes and matched NACK; physical close joins instead of overwriting; a response the write itself received is still returned once and not retained. The model root and both genuine ACK/NACK roots went red to green. |
| IPv6 | The arm64 kernel `cddff82e255fc9a7010a79d5237a733463569f495ae483b60962455788fb3884` and image `f92054020556fb26` passed four constructor models and thirteen genuine API roots. Carrier adopted 2026-11-22 (resume step 3): the kernel session carrier opens its ICMPv6 and UDPv6 sockets from the original unit, and the name-based path is deleted. On amd64 runtime tree `2b23af13` all nine `TestNativeIPv6Carrier*` roots pass, and so does the whole `pppoe/dataplane/kernel` package. The adoption also repaired kernel patch `0013b` for UDP6 receive. Fragment ownership is accepted (resume step 4, `169eccfa2c`): the five fragment roots pass on `2b23af13`, the package passes 47/0/0, and the name-based carrier cut turns the ownership roots red. Complete subscriber acceptance remains open. |
| PCI override prerequisite | The Go reader and doctor reject ambiguous legacy override state and preserve exact raw bytes/errors. The `driver_override_raw` kernel patch `0006` is not enrolled. Do not claim managed NIC restoration is accepted on the current kernel from the Go model results. |
| Final gates | Current defect: VPP subscribers forward unpoliced, because the shaper's ingress policer is refused by traffic-vpp and only logged, and a VPP session's CoA rate change is NAKed (`unwired-feature.md`, 2026-11-22). The fix is the policer wiring design below. The real-VPP IPv4 product lab passed 4 of 4 on 2026-11-22, including 100 subscribers, and `product-framed` passed 1 of 1 (AC-7) ("Real-VPP product lab" below); the discrimination of both is owed. The OD-2 and policer designs are in "Implementation designs — 2026-11-22". D-1 is decided (2026-11-23, "Owner decisions of 2026-11-23" there): OD-2 now waits on the design of `spec-vpp-failure-lifecycle`, and the policer wiring waits on C-1. Server PADS and PADT order is proven (`b6388ee211`). AC-11 still lacks a PPPoE operator clear (owned here) and CoA Disconnect for PPPoE (`spec-lifecycle-invariants` I-4). L2TP accounting counts the original unit (`38267f5e0c`) and AC-24 route receipt is proven (`0a92e78161`); resume step 5 lists what remains of accounting and who owns it. Owed implementation, each needing a design: OD-2 VPP loss handling for AC-12 (no PPPoE code consumes `vpp/disconnected` or `vpp/reconnected`, and `Reconcile` in `pppoe/dataplane/vpp/reconcile.go` has no production caller), and policer wiring for AC-9, AC-10 and AC-15 (`newPolicerRatePolicy` in `pppoe/dataplane/vpp/rate_policy.go` has no production caller). Owed: the accounting admission design and the name-based shaper (Owned journal rows), scoped lint, documentation checks, independent review, discrimination, and real VPP forwarding, restart, interop and scale. AC-8 waits on `spec-l2tp-ipv6-subscriber`. Keep this spec in phase 2/7 until those requirements are actually met. |

### Retirement causal cuts — 2026-11-22

Each cut ran once and was restored to HEAD. L1-L5 are the five logical roots
in the order the checkpoint table names them. Logs: `job-cut-C*-678f4411.log`
and `job-gcut-*-fa991f8b.log` under `tmp/session/2026-11-22-183517/scratch/`.

| Cut | Red roots |
|-----|-----------|
| A failed original close is read as retirement proof | L1, L3, genuine `FailedOriginalClose` |
| The borrow check is removed from publication | L2 |
| An unsettled duplicate is ignored | L3 |
| The namespace close error is fed into retirement | L4 |
| The seal wait is removed from `closeNativeOperations` | L5 |
| The session ignores unit retirement in `NativeReleasePending` | L1, L2, L3 |
| Retention is cleared before the seal confirms | genuine `PendingSealDefersLocalSettlement`, the duplicate fixture |
| Retirement never completes | genuine `ConfirmedOriginalClose` and `PendingSealDefersLocalSettlement`, the duplicate fixture |

The genuine borrow root stays green under the borrow cut. A real
`CreatedUnit.Close` does not close the file until its borrows end, so only L2
discriminates the publication check. This is defence in depth, not a gap.

The final diagnostics and retirement fixes are implemented. The whole
`ppp/kernel` package passes on 2026-11-22 (the "Current broader
creation/guard population" row below); the other consumer packages' native
reruns remain owed. The earlier full VPP run's eight override
failures are preserved as baseline evidence; their focused source repairs now
pass. No worktree, source copy, local server or VM from this checkpoint is left
running. Current proof inputs remain under the existing session's
`scratch/vpp-current-open-successor/counter-creation/`; use its manifests and
small handoffs rather than reapplying the consumed assembly publisher.

### Real-VPP product lab — 2026-11-22

`./le --name session test qemu docker-lab lab "test deployment vpp-pppoe product"`
passed on runtime tree `2b23af13`, with image `ze-vpp-pppoe-lab` from the
content-keyed cache (key `88588b41...`): `pppoe-vpp-ze-ac-pppd-client` (33 s),
`-namespace` (25 s), `-vlan` (24 s) and `-hundred` (7 min 42 s), 4 passed and 0
failed (`tmp/session/2026-11-22-203554/scratch/job-vpp-product-fc5ac019.log`).
It is the first recorded pass of the hundred-subscriber scenario since the
capture batching fix `2467c8c5f0`. It covers AC-3, AC-4 (state), AC-5, AC-16,
the pppd SIGTERM close of AC-11, and 100 concurrent subscribers as a functional
result, not a performance figure. Discrimination of these scenarios (revert,
rebuild, red) is owed.

AC-11 teardown gaps and their owners: no PPPoE operator clear command exists
(`guard-added-to-one-half-of-a-pair.md`, 2026-11-22, owned here); a RADIUS
Disconnect-Request for a PPPoE session answers Disconnect-NAK
(`authradius/coa.go` `handleDisconnect`, "not yet wired to PPPoE teardown"),
owed by `spec-lifecycle-invariants` I-4 and its AC-15.

`./le test deployment vpp-pppoe product-framed` passed the same day: scenario
`pppoe-vpp-ze-ac-radius-framed-route` (26 s), image from cache key
`88588b41...` (`tmp/session/2026-11-22-203554/scratch/job-vpp-framed-52302217.log`).
It covers AC-7 for a RADIUS Framed-Route. Its discrimination is owed too.

## Implementation designs — 2026-11-22 (D-1 decided 2026-11-23; C-1 open)

Two strongest-tier design agents produced these read-only, from facts read at
the producers. Each designs within an agreed decision: OD-2 decides the loss
behavior, and OD-4 with AC-15 decides the policer behavior. Paths are under
`internal/component/l2tp/` unless named otherwise. Nothing here is built.

### OD-2 VPP loss handling (AC-12)

Corrections to earlier records:

- The loss seam exists, unwired: `pppoe/dataplane/lifecycle.go`
  (`SubscribeLifecycleFunc`, `LifecycleSignals` with Lost and Ready,
  `LifecycleWatch` with Snapshot, Admit, Admission and Current, and a sticky
  `LossSequence`). `RegisterSessionDataplane` takes it as an optional third
  argument; only tests call `WatchLifecycle`, and `plugins/pppoe-vpp/register.go`
  passes none.
- VPP access uses `NewAccess`, not `NewRecoverableAccess`. `SettleAccess` runs
  only when an access Close failed (caller `RefuseAccess`).
- PPPoE sends no accounting Stop today: `authradius` subscribes to
  `l2tpevents.SessionDown` only, `subevents.SessionDownPayload` has no cause,
  and `withdrawSubscriber` drops `ppp.EventSessionDown.Cause`.
- `vpp/disconnected` and the process-exit proof are unordered: the proof comes
  from a 1 s pidfd probe (`vpp/conn_health.go`, `process_peer.go`
  `observeExit`), and disconnected is the deferred `retireReadiness` in
  `vpp.go`.

Mechanisms extended:

| Mechanism | Use |
|-----------|-----|
| VPP readiness subscription, as `iface/register.go` `subscribeReconcileOnReady` and `fib/vpp/register.go` `onVPPReady` do | In the pppoe-vpp wrapper, mapped onto `LifecycleSignals` |
| `LifecycleWatch` | Wired as written; the PPPoE subsystem owns its one worker |
| Reload's removal half (`quiesceRemovedServers`, `closeInterface`, `detachDrainedServers`, `closeAccessConfiguration`) | Becomes the loss drain |
| Reload's addition half (`prepareAccessConfiguration`, `prepareAddedServers`, `swapServers`) | Becomes the recovery |
| `Reconcile` (`pppoe/dataplane/vpp/reconcile.go`) | Called at the head of VPP `Prepare`; its error does not fail Prepare, because `controlMapOwners.refuseOverlap` stays the gate |
| `AccessRecovery` | Unchanged |
| `ppp.EventSessionDown.Cause`, `TerminateCauseLostService` (3) | Exist; `subevents.SessionDownPayload` gains a `Cause` field |

Subscription and order:

1. A new `plugins/pppoe-vpp/lifecycle.go` (tags `ze_vpp` and `ze_l2tp`) maps
   `vpp/disconnected` to Lost, and `vpp/connected` and `vpp/reconnected` to
   Ready, with no I/O. `register.go` passes it as the third argument. The
   `pppoe` package imports nothing from VPP.
2. `Subsystem.startLocked` calls `WatchLifecycle` on the bus before
   preparation, takes the snapshot, prepares and builds, then admits that
   snapshot. A stable provider (kernel) answers supported=false, and one
   helper names that case "always admitted".
3. Admission fence: `dispatchFrame` drops frames when there is no admission.
   `handlePADR` takes the permit and rechecks Current before it sends PADS and
   again before it hands the start to `SessionsIn()`. A stale permit closes
   what was opened and sends no PADS.
4. On loss: (1) Lost fences admission; (2) under `lifecycleMu`, quiesce every
   server (`dispatchMu` waits out an in-flight PADR); (3) for each admission,
   mark lost-service on the row (`SessionTable.markTeardownCause`) and run
   `handleSessionDown` with PADT suppressed (claim the publication, send
   nothing); (4) wait for the admission; the PPP terminal event reaches
   `withdrawSubscriber`, which publishes the subscriber SessionDown with cause
   3, and the pool releases on that event; (5) detach the servers and close
   the accesses, the provider retaining any failed native cleanup; (6) raise
   the loss report once.
5. On ready (a retry is requested, nothing is admitted, and every loss is
   retired): run `prepareAccessConfiguration` (VPP Prepare runs Reconcile
   first), refuse a changed discovery namespace, build the servers, run
   `swapServers`, then Admit the exact observation (a failed Admit is a new
   loss), and clear the report.
6. While prepare fails, the worker retries on its own `pacer.Pacer`; a second
   Ready during the candidate phase does not wake it.
7. Lock order: the worker needs `lifecycleMu`, and `stopLocked` holds it while
   it joins workers. So `stopLocked` closes the watch and returns the worker's
   done channel, Stop and Reload join after unlocking, and a worker that finds
   its watch closed returns.

Report once: a new code `doctor-pppoe-backend-lost`
(`pppoe/register_doctor_codes.go`), with a runtime check and a health probe
on the pattern of `pppoe/dataplane/vpp/reconcile_doctor.go`, named from the
provider (`s.backend.Name()`), not from VPP. One Warn per retired loss
sequence and one warning raised under `pppoe`, `backend/vpp`, cleared on
Admit; retry failures log at Debug. Text sample:

| Part | Text |
|------|------|
| Warning | PPPoE lost its "vpp" session backend and ended 37 subscriber sessions on 2 access interfaces |
| Why | No subscriber can connect until the backend is ready and its access interfaces are prepared again. Ended subscribers redial by themselves. |
| Fix | `ze doctor` |
| Recovery (Info) | pppoe: session backend "vpp" is ready; discovery resumed on 2 access interfaces |

Races:

- A disconnect during PADR or open: the open fails as not sent or uncertain
  (retained with its MAC reservation), and the Current recheck stops PADS.
- A reconnect before the drain finishes: the loss count is sticky, the worker
  retires every loss before it prepares, and Admit compares the exact
  observation.
- A second disconnect: a no-op while unavailable; during re-preparation Admit
  fails, and the worker closes the new accesses and loops.
- AC-23: a drain before the exit proof makes each session Close fail,
  `macReservation.retain` raises one `doctor-pppoe-vpp-reconcile` per session,
  and `retireCoveredSessions` clears it on proof; `refuseOverlap` blocks
  re-preparation until then.
- Final counters: the stats segment is gone, so the Stop carries no counters
  (the accounting spec owns the record).

Tests, each with its red:

| Test | Where | Proves | Red before the change |
|------|-------|--------|-----------------------|
| `TestVPPEventsDriveLifecycle` | `plugins/pppoe-vpp/lifecycle_test.go` | VPP events drive Lost and Ready | `WatchLifecycle` answers supported=false |
| `TestPPPoEVPPReconcileOnReconnect` | `pppoe/subsystem_lifecycle_test.go` | Fake changing provider, two sessions. After Lost: no PADT, two SessionDown with cause 3, empty tables, accesses closed, exactly one warning, PADI unanswered. After Ready: Prepare runs again, PADO answered, warning cleared | Sessions stay and PADO is answered |
| `TestLossDuringPADRPublishesNoPADS`, `TestReadyBeforeDrainFinishesWaits`, `TestSecondLossDuringRecoveryRefusesAdmission` | same | The three races above | - |
| `TestPrepareReconcilesRetainedFirst` | `pppoe/dataplane/vpp/` | Prepare reconciles retained state first | - |
| Lab verb `product-restart` | `internal/le/test/deployment/vpppppoe/` (action in `register.go`, `productSuiteRestart` in `product_plan.go`, new `product_restart.go`; split `prepareProduct`; add `show warnings` to the `observeProduct` allowlist) | Dial, kill VPP with SIGKILL, wait for zero `show pppoe sessions`, exactly one `doctor-pppoe-backend-lost`, restart VPP and its infrastructure, the warning clears, no `doctor-pppoe-vpp-reconcile`, redial, `checkProduct` passes | Revert the wrapper's lifecycle argument: sessions never reach zero |

The new verb appears in `le help`, and `docs/labs/pppoe-interop.md` and this
spec's `pppoe-vpp-restart` row are renamed to it.

Files. Create: `plugins/pppoe-vpp/lifecycle.go` and its test,
`pppoe/subsystem_lifecycle.go` and its test, `vpppppoe/product_restart.go`.
Change: `plugins/pppoe-vpp/register.go`, `pppoe/subsystem.go`,
`pppoe/subsystem_reload.go`, `pppoe/server.go`, `pppoe/session.go`,
`pppoe/register_doctor_codes.go`, `subscriber/events/events.go`,
`pppoe/dataplane/vpp/prepare.go`, the lab's `register.go`, `product_plan.go`,
`product_check.go` and `product_observe.go`,
`docs/architecture/l2tp/bng-5-pppoe.md`, and this spec's rows.

Open and recorded choices:

- **D-1 (owner decision, decided 2026-11-23: see "Owner decisions of
  2026-11-23" below).** With `vpp external true`, the API is lost
  while the VPP process stays alive. Managed mode kills the child, so only
  external mode is exposed. No exit proof arrives: old sessions keep
  forwarding in VPP, retained MACs stay blocked, and `refuseOverlap` refuses
  re-preparation until that VPP exits. Goal G-5 ("no stale state in VPP,
  service resumes without operator action") conflicts with the rule in
  `bng-5-pppoe.md`, "An API reconnection alone cannot release that ownership".
  - (A) OD-2 literally: service stays fenced until VPP restarts, and the
    warning names it. Released pool addresses can be issued again while stale
    VPP routes exist.
  - (B) Hold each retained session's pool address until Reconcile releases it
    (a second release event).
  - (C) Let Reconcile act over a new API generation of the same pidfd-proven
    process, by exact identity readback. It meets G-5 and changes the
    documented invariant.
- **D-2 (ordering, recorded).** RADIUS Stop on the wire for PPPoE is owed by
  `spec-pppoe-subscribers-produce-no-accounting-or-telemetry` (design). Until
  then AC-12's accounting clause is provable only at the event (cause 3).
- **D-3 (recorded).** VPP not up at boot: Start fails as it does today; AC-12
  does not cover it.

### Owner decisions of 2026-11-23: D-1 resolved

Thomas set three rules for every VPP Ze runs. They are recorded in
`docs/research/vpp-deployment-reference.md`, "Design decisions", and owned by
`plan/post-release/features/spec-vpp-failure-lifecycle.md`:

1. Ze refuses to run VPP unmanaged; with an external VPP the operator accepts
   that Ze may terminate and restart it.
2. Ze knows how to manage and restart an external VPP: a systemd unit, or a
   direct start of the binary as with the embedded runtime.
3. After a failure: process alive, forwarding continued, reconnect and state
   readable: continue from where it is. Software possibly crashed or stopped
   forwarding: reflect it in Ze and update IGP and BGP peers.

**Owner refinements, same day** (lifecycle spec R-4 to R-7 and R-9; the "Design
decisions" rows of both pages, each naming the earlier row it narrows):

- One timer serves both the alive test and the loss declaration. A probe
  missed inside it is not a loss.
- "Recovered" has exactly two forms. Form 1, inside the timer: Ze reconnects
  to the same pidfd-proven process and a full readback of everything it
  configured (interfaces, LCP pairs, owned routes, PPPoE sessions) matches.
  Ze continues, with no kill and no withdrawal, and the PPPoE sessions are
  kept. Form 2, on a declared loss: Ze kills the process through the retained
  pidfd, waits for the exit proof, restarts VPP and reconfigures it exactly as
  at boot; PPPoE drains per OD-2 and subscribers redial.
- Option C is scoped to form 1 only. A declared loss is never adopted, and no
  undecided window follows the declaration.
- Blackholing during the restart is accepted. Routes are withdrawn
  (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when
  neither form succeeds. The recovery deadline and the crash-loop limit that
  bound "cannot recover" are the main session's proposals (P-1, P-2), not his
  words, until he adopts them.
- Corrected the same day: "Ze is the only restarter, a systemd unit it drives
  has Restart=no" was the main session's proposal P-3, not his rule. His
  systemd words reverse it: "we can not be sure that an OS update will not
  change the settings so we should detect the settings and adapt". Ze requires
  no unit policy; it reads the unit's effective settings each time it acts and
  adapts (lifecycle R-9, step 2a).
- Answered by Thomas, still 2026-11-23 (lifecycle A-1 to A-7, R-10 to R-12;
  VPP page rows 10 to 15, `bng-5-pppoe.md` row 4). Each narrows the bullets
  above where they differ:
  - Two limits: a failed alive test is the loss declaration, and a second
    limit bounds the reconnect-and-verify attempt (form 1) before Ze kills and
    restarts. Form 1 now runs after the declaration, inside that second
    limit; "inside the timer" and "a declared loss is never adopted" above
    read as "inside the reconnect-and-verify limit" and "never adopted once
    that limit expires".
  - Form 1 keeps the PPPoE sessions on the presence of data, his words: "We
    should take the presence of data as a sign that the software is still
    running and reconciliation may be then happening in the background." The
    same pidfd-proven process answering with its configured data is adopted;
    the option C readback below no longer gates the adoption and runs as
    PPPoE's part of the background reconciliation. A mismatch it finds is
    repaired, never a loss. Recorder: whether the readback table's "refuse,
    retain and report" is that repair for a field mismatch, and whether it
    settles E-1, is for this spec's OD-2 design.
  - The recovery deadline and the crash-loop limit are adopted; past either,
    Ze withdraws. After a withdrawal Ze keeps retrying with a growing backoff,
    and the PPPoE service comes back when VPP recovers (by form 2, so
    subscribers redial). Ze's own protocol timers keep running.
  - A foreign process on VPP's API socket (its pidfd does not resolve to the
    configured unit) is reported by `ze doctor` and neither refused nor
    killed; Ze keeps working with it. For PPPoE, Recorder: the retained
    owners of a foreign peer follow E-4 (a different process fails form 1);
    how they retire when Ze may not kill that process is for the lifecycle
    design.
- Open: the values of the two limits, the deadline and the crash-loop limit,
  and lifecycle choices 4, 7 and 8, all proposed at that spec's design review.
- F-1 stays a defect to fix; F-2 closes with it; F-3 is fixed for form 1 by
  change 2. E-1 stays open for form 1; E-2, E-3 and E-4 close on form 2. No
  graceful-restart claim on re-open: the F bit stays 0.

He also chose option C for D-1, with osvbng and VyOS as references. Under the
rules and their refinements D-1 resolves on two paths, recorded in
`docs/architecture/l2tp/bng-5-pppoe.md`, "Design decisions":

| Path | What PPPoE does |
|------|-----------------|
| Form 2: a forced restart (the reconnect-and-verify limit expired, or form 1 failed: a different or unprovable process, or no configured data inside the limit) | Ze kills VPP through the retained pidfd and restarts it, so its exit proof always arrives and retires the retained owners. OD-2's drain runs here, on every forced restart, and subscribers redial. E-2, E-3 and E-4 close on this path |
| Form 1: inside the reconnect-and-verify limit (same pidfd-proven process answering with its configured data) | Sessions survive. Option C runs in the background reconciliation: retained owners read back their exact native identity over the new connection of the same process and are cleaned there; a mismatch is repaired, never a loss. E-1 stays open |
| After the failed alive test, before form 1 succeeds or the limit expires | Admission is fenced and sessions are held (lifecycle choice 6) |

OD-2 (A) still decides what a forced restart does; it never runs inside the
reconnect-and-verify limit. The subscription in step 1 of "Subscription and order" maps the
lifecycle spec's events once that spec names them (its choice 8). This work
depends on that spec's design.

Findings (read at the producers, 2026-11-23):

| # | Finding |
|---|---------|
| F-1 | Defect: the VPP provider holds no process lease. `captureAPI` (`pppoe/dataplane/vpp/api.go`) keeps only the connector and the generation. `Connector.RetainProcessForGeneration` and `ProcessLease.SameProcess` (`internal/component/vpp/process_peer.go`, `process_lease_equality.go`) have test callers only, and the first one's comment says it MUST precede the first native mutation |
| F-2 | Defect: without a lease the old generation's exit proof is lost. `processAPIConnection.Disconnect` (`process_transport.go`) releases the witness; the last release closes `stop`; `observeExit` then returns without `notifyProcessExit` (its only caller). `exitedProcess.Covers` needs the exact generation. So in external mode after an API drop, a later VPP exit never retires the old owners, and `refuseOverlap` refuses for good, not "until VPP exits". Managed mode may race the same way (1 s tick against `retireManagedConnection`): unconfirmed |
| F-3 | An old owner cannot use the new connection: `binaryAPI.open` is `NewStreamForGeneration(generation)` and answers `errVppGenerationChanged` after a reconnect, so `retrySession`, `readSessionKeys` and `controlMapOwners.remove` all fail |
| F-4 | The accel-ppp-ng VPP-loss row in "Reference design" was stale; corrected there |
| F-5 | The product lab already runs `vpp external true` (`vpppppoe/product_plan.go` `productConfigWith`); it converts under the lifecycle spec's restart method |

References: osvbng (753e934) keeps sessions in its OpDB and, after an API loss,
reconnects and decides "same or new VPP" from an interface-count heuristic
(`ifCount <= 1` means state lost, then replay; key-idempotent: retval -116 is
success, -500 is delete and recreate). accel-ppp-ng at 7e4ec88 only
disconnects and reconnects lazily on an API error. vyos-1x (db29bdc89) holds
no long-lived API and remaps every cp mapping on each commit. None proves
process identity; option C is stricter than all three, and ng's HEAD shows that
a live VPP after an API drop is real.

Changes (not built):

1. Fixes F-1 and F-2: `captureAPI` takes
   `connector.RetainProcessForGeneration(generation)`; `binaryAPI` holds the
   lease until its last owner releases, and `Retire` runs on proof. Leases are
   not constructible outside the `vpp` component, so a test seam is owed. Each
   retained generation of a live process holds one of the 1024
   `processWitnessMax` slots.
2. Fixes F-3: `Reconcile` gets the current `binaryAPI` from `Prepare`. A
   retained owner on the same connector with an older generation: when
   `old.lease.SameProcess(current.lease)` is true, readback and cleanup run on
   the current stream; when it is false or errors, the owner is retained and
   reported. Equality never retires an owner; only a confirmed deletion or a
   completed absence read does.

Readback before delete:

| Owner | Readback | Then |
|-------|----------|------|
| Session, confirmed creation (pending session set, closed) | `readSessionIdentity`: sw_if_index, SID, client MAC, EncapIfIndex, local MAC, DecapVrfID, a single row, plus the interface name captured at create (`binaryAPI.interfaceName`) | `closeNative`, unchanged |
| Session, unknown creation | Full dump `readSessionKeys` on the current stream | Absent: release. Present: retained (E-1) |
| Owned cp map (`controlMapLeaseState`, owned, no references) | Reciprocal pair read (`controlPairPresent`) | `remove` on the current stream; Prepare adds it again |
| `controlMapUnconfirmed` | Targeted read of both indices | Absent: release. Present: retained (E-1) |
| Unknown first policer binding | None is safe | Retained (E-2) |

Refuse, retain and report on: `SameProcess` false or an error; an old owner
with no lease; any field mismatch; a duplicate or incomplete dump; a session
not closed; a session absent while its routes remain. `refuseOverlap` is
unchanged. With the OD-2 drain: Close fails as not sent,
`macReservation.retain` raises one `doctor-pppoe-vpp-reconcile` per session,
and recovery (`prepareAccessConfiguration`, then VPP Prepare, then
`Reconcile(current)` first) clears them while the worker pacer retries. About
five API calls per session, serially; not sized against the retry interval.

Option B's address hold is not needed on the common path: no VPP session is
admitted before Reconcile. It leaves two holes: a session left retained while
admission resumes keeps its /32 in VPP, so a new session given that address
adds a second ze-pppoe path (traffic shared between stale and new); and the
pool is shared with L2TP and pppoe-kernel, which keep admitting in the window.

Open owner choices left by C, and where each stands under the rules:

| # | Choice | Status under the rules |
|---|--------|------------------------|
| E-1 | Unknown-outcome creations (a PADR in flight at the loss): (a) keep refusing, the MAC blocked until VPP exits; (b) adopt on a complete identity match in a same-process VPP where Ze holds the owned cp map for that access. VPP sessions carry no owner tag, so Ze cannot tell its own from an identical one another API client made | Open on form 1, inside the timer (lifecycle choice 6) |
| E-2 | Unknown first policer binding stays quarantined until VPP exits (policer design, C-1) | Closes on form 2: the kill delivers the exit |
| E-3 | A retained session's address: (a) recovery refuses admission while Reconcile reports a pending confirmed session; (b) hold for retained sessions only (misses RADIUS addresses and framed routes); (c) a publication guard: AddAddressP2P or Connect refuses a prefix that already has a ze-pppoe path through another index (covers pool, RADIUS and framed routes; exact or refuse). None alone closes the window in which L2TP and pppoe-kernel share the pool | Closes on form 2: the exit removes the stale state before admission resumes |
| E-4 | A different process on reconnect: after change 1 the old lease delivers the exit proof and retirement works; if the old process lives on behind another socket, owners stay retained and overlapping accesses fenced | Closes on form 2: an unprovable process fails form 1 and is killed |

Tests owed (`pppoe/dataplane/vpp` with a fake connector and the lease seam,
unless named):

| Test | Red before the change |
|------|-----------------------|
| `TestReconcileSameProcessReconnectCleansRetainedSession` | PendingSessions stays 1 with `errVppGenerationChanged` |
| `TestReconcileSameProcessRemovesOwnedControlMap` | The map stays and Prepare is refused by `refuseOverlap` |
| `TestReconcileDifferentProcessRefusesCleanup` | Cut: skip `SameProcess` |
| `TestReconcileComparisonErrorRetains` | Same cut |
| `TestReconcileSameProcessIdentityMismatchRetains` (table: index, SID, MAC, access, local MAC, table, name) | Cut: skip the readback |
| `TestReconcileUnknownCreationVisibleStaysRetained` | Cut: delete on presence |
| `TestCaptureAPIRetainsProcessLease` | No lease taken |
| `TestLeasedWitnessPublishesExitAfterDisconnect` (`internal/component/vpp`; check for an existing one first) | Pins F-2 |
| Lab `product-restart`, API-drop case | Three cases (lifecycle step 3.1 and step 5). A SIGSTOP (or a container pause) shorter than the single timer does not break the socket: it exercises the late-probe state, asserts that nothing changes, and cannot prove form 1, because no reconnect happens and the `SameProcess` cut below would stay green on it. Form 1 needs a real API-only drop with the process alive (mechanism at the lifecycle design): it asserts same process, readback matches, sessions survive. A freeze longer than the timer asserts form 2: a pidfd kill, the exit proof, a restart and reconfiguration, the OD-2 drain and a redial. The timer value is the lifecycle spec's P-4. Red: cut the `SameProcess` adoption; reconcile warnings persist and the redial is refused ("client MAC already belongs to a live or constructing session"). A socket proxy cannot drop the API, because SO_PEERPIDFD would name the proxy. Not read: whether the lab VPP shares a container with Ze (signal or pause) |

Text that changes when this is built: `bng-5-pppoe.md` "on its captured
connection" ("VPP session ownership") and "block overlapping admissions across
API reconnections"; the `Reconcile` comment in `reconcile.go` ("on the original
immutable connection. A failed or changed connection cannot authorize an old
owner to use a new one"); the `SameProcess` comment, which names this caller.
The page's invariant already carries a note that the decision replaces it.

### Per-session VPP policer wiring (AC-9, AC-10, AC-15)

Facts at the producer:

- `newPolicerRatePolicy` (`pppoe/dataplane/vpp/rate_policy.go`) has no caller.
  `session.closeNative` (`pppoe/dataplane/vpp/session.go`) never touches a
  policer, though its comment says every native child owner, policers
  included, MUST finish before session deletion.
- `subscriber.RatePolicy` has no consumer: `dataplane.OpenedSession`,
  `ppp.SessionHandle` and `subscriber.Session` carry no policy and no owner
  command (`ppp` cannot: `subscriber` imports `ppp`).
- **Current defect.** The initial rate today is `Subsystem.onSessionUp`
  (`pppoe/subsystem.go`) handing every session, VPP included, to the shaper
  handler by `sess.PppInterface`. For a VPP session the shaper calls the
  traffic backend by the VPP interface name; traffic-vpp refuses the ingress
  policer (`applyInterface`, `internal/plugins/traffic/vpp/backend_linux.go`);
  the shaper logs a warning, and the VPP subscriber forwards unpoliced.
  (`unwired-feature.md`, 2026-11-22.)
- CoA today: `confirmCoA` (`plugins/authradius/coa_transaction.go`) names the
  shaper's rate request as a literal (prepare, apply, commit, rollback with a
  token through `cfg.Dispatch`). The shaper's `prepareRate` needs a confirmed
  interface, so a VPP session's CoA is NAKed.
- The kernel rate owner today is the shaper plugin, by `pppN` name
  (`identifier-reused-after-its-owner-is-gone.md`, 2026-11-22, AC-26). Its
  `ShaperHandler` is an in-process function registered from `init()`, so it
  already does nothing when the shaper runs external.
- The library CoA is `PolicerUpdate` in place on name-verified indices
  (`policerRates.apply`): no unbind, no delete, no feature change. Its cleanup
  (`policerRates.cleanup`) turns the features off (four input arcs, device
  input, two output arcs), unbinds each, then deletes each.
- AC-9's verify refusal (TBF or HTB under VPP) does not exist.

Mechanisms extended:

| Mechanism | Use |
|-----------|-----|
| Rate policy seam (`subscriber/rate_policy.go`) | The one route: a VPP session gets a local rate policy; a kernel session gets a remote policy whose owner is the shaper's command |
| Shaper handler registry | `RegisterShaperHandler` also registers the shaper's rate owner command; the kernel provider reads that registration and never names the command string |
| `NewOpenedSession` (`pppoe/dataplane/session.go`) | `OpenedSession` gains a rate policy accessor, validated by `ValidateRatePolicy` at construction |
| CoA owner-command transaction (`authradius` `confirmCoA`, the shaper's rate operation, CoS `session_transaction.go`) | Reused; only the command name stops being a literal |
| VPP session owner and retention (`session.Close`, its retain, the reservation retain, `Reconcile`) | The policer owner becomes a child of the session |

Build point, initial rate, CoA, shaper exclusion:

1. In the factory's `openWithOps`, after `verifySession` and the interface
   name and before the transfer is marked: build the policer policy from the
   owner command, the created index, a prefix and the access API; store the
   native owner on the session; hand the policy to `NewOpenedSession`. No
   native write at construction; PADS timing is unchanged.
2. Initial install happens in `onSessionIPAssigned`, before the first
   acknowledge (before Connect opens forwarding); the Filter-Id read moves
   there. Local policy: new rates, then prepare, apply, commit, and cancel the
   assignment on failure, so a session never forwards unpoliced. Remote
   policy: today's shaper call. No rate: a named no-op. Install once per
   session.
3. Shaper exclusion by variant: `onSessionUp` calls the shaper handler only
   for a remote policy.
4. CoA: `subscriber.Session` gains a `RateOwner` field; `authradius` rebuilds
   the owner command from it and dispatches there; an empty owner answers
   CoA-NAK. The rate prepare request gains `subscriber-id`, which the shaper's
   rate request must accept (it refuses unknown fields today).
5. VPP owner command: a new PPPoE request RPC (`pppoe/cmd` and
   `ze-pppoe-api.yang`) with the same JSON operations and token. It finds the
   session by subscriber ID through `pppoe.LookupService()`, requires a local
   policy, allows one pending rate transaction per session, and updates the
   registry rates on commit. `authradius` imports nothing from VPP.

Teardown (AC-15):

1. `closeNative`: admin down, IP gates closed, then the rates' Close (features
   off, unbind, delete), then routes, then the VPP session remove only when
   rates, routes and gates are confirmed. PADT stays after Close.
2. On failure or an unknown outcome, the rates' Close error joins the cleanup
   error, the session remove is skipped, and the session's retain keeps the
   owner and the MAC reservation (AC-23). A retry re-enters `closeNative`;
   `policerRates.Close` clears only confirmed slots.
3. Gap: the process-exited branch must release the policer slots, and
   `policerRates` has no retirement method yet.
4. The CoA path deletes nothing.

Verify refusal (AC-9): beside `validateBackendGate` (`pppoe/backend_gate.go`),
reading the selected provider's registered rate capability, never a test of
the backend name. The refusal stays, and its text names the VPP egress shaper
spec as what lifts it ("VPP subscriber shaping research" below; that spec is
not written yet).

Tests, each with its red:

| Test | Where | Proves | Red before the change |
|------|-------|--------|-----------------------|
| Close order | `pppoe/dataplane/vpp/` unit, fake API | Feature off, unbind, policer delete, then the session delete; an unknown unbind leaves the session retained; process exit releases; the factory exposes a local policy | Drop the rates' Close |
| CoA in place | same | Two `PolicerUpdate` plus readback; zero bind, feature or delete | - |
| Local before acknowledge | `pppoe/` unit | A local policy is installed before Acknowledge and the shaper is not called; a remote policy calls it | Today the shaper is called for both |
| Owner dispatch | `plugins/authradius/` unit | Dispatch goes to `Session.RateOwner`; an empty owner NAKs | - |
| Lab verb `product-rate` | `vpppppoe/`, modelled on `product_framed.go` | Filter-Id gives two policers at the rate on the session interface and drops above it; CoA ACK with the same indices, the new rate and bounded loss; 100-session teardown under traffic leaves no policer and the same VPP PID | - |

Files. Change: `pppoe/dataplane/vpp/factory.go`, `session.go`,
`rate_owner.go`, `pppoe/dataplane/session.go`,
`pppoe/dataplane/kernel/factory.go`, `pppoe/subsystem.go`, `pppoe/session.go`,
`pppoe/cmd/pppoe.go`, the PPPoE API YANG, `subscriber/session.go`,
`subscriber/handler_registry.go`, `plugins/shaper/register.go`,
`rate_transaction.go`, `plugins/authradius/coa_transaction.go`, `coa.go`,
`docs/architecture/l2tp/bng-5-pppoe.md`,
`docs/architecture/l2tp/subscriber-session-model.md`, and this spec's rows.
Create: the PPPoE rate-transaction handler, `vpppppoe/product_rate.go`, and
the tests.

Choices:

- **C-1 (owner decision, open): where a configured VPP rate lives.** The only
  configured rate is `l2tp/shaper/default-rate`, which also serves kernel L2TP
  sessions, and AC-26 requires kernel shaping while VPP is the backend.
  - (a) Add `policer` to `l2tp/shaper/qdisc-type` (R-4's wording): one value,
    but TBF and HTB cannot be refused under VPP without also refusing AC-26's
    L2TP case, and PPPoE reads the shaper's config.
  - (b) A PPPoE-owned rate leaf: conflicts with OD-3 ("no leaf added").
  - The designer recommends (b), subject to OD-3. RADIUS Filter-Id rates need
    no config and are unaffected.
- **C-2 (answered).** AC-10's "replaced" is answered by `bng-5-pppoe.md`:
  update in place, so there is no gap at all.
- **C-3 (answered).** A `.ci` stub cannot supply AF_PPPOX and a linux-cp TAP
  (it would need many VPP messages in `internal/test/cli/cmd_vpp_stub.go`);
  this spec already moved session proof to the product lab, so the lab
  carries AC-10.

Not verified by the designer: `authradius` dispatch reaching an engine-side
RPC with a source peer; whether the FreeRADIUS lab image ships `radclient`;
the policer name prefix rules; whether editing `confirmCoA` stales its three
RFC 5176 records.

### VPP subscriber shaping research — 2026-11-23

Thomas said yes to a VPP shaping spec and asked how osvbng and VyOS do it.
Read-only research by a strongest-tier agent; nothing is built.

| Source | What it does |
|--------|--------------|
| osvbng's plugin (`~/Code/github.com/veesix-networks/osvbng-vpp`, `plugins/osvbng_qos_sched/`, about 4.8k lines of C) | Egress (download) only: `ip4-cake-enqueue` and `ip6-cake-enqueue` features on `ip4-output` and `ip6-output`, keyed by TX sw_if_index. One owner thread per scheduler, the others hand off (`vlib_buffer_enqueue_to_thread`, drop on congestion, counted). A polling input node `cake-dequeue` per owning worker sends at most one frame per pass and restarts buffers on the output arc. Per-scheduler byte limit (1.5 x 100 ms x rate, floor 64 KiB) with tail drop, a 128-entry ring per flow, COBALT drop and ECN. Disable under the worker barrier (features off, drain, free) and on interface delete, because sw_if_index is recycled; `tests/churn-rig.sh` reproduces a SIGSEGV from buffers queued to a deleted interface (the AC-15 class). API: enable and disable, dump, reset stats, aggregates; no in-place rate update. GPL-3.0-or-later, early development, LLM-authored, targets VPP 26.06 (Ze builds 25.10: unverified). Its figures are from containerlab with no pinning: enqueue 282 clocks per packet for FIFO and token bucket, 347 for full CAKE, 271 baseline; a 100 Mbit/s shaper measured 123 Mbit/s in phase 1 |
| VyOS (`~/Code/github.com/vyos/vyos-1x`, db29bdc89) and accel-ppp-ng (7e4ec88) | VyOS does not shape in VPP. Subscriber rate under VPP is police-only in both directions: ng's `install_limiter` hook makes two RFC 2698 2R3C policers (cir 0.8 x rate, eir = rate, violate drop). VyOS's RADIUS shaper template hard-codes `down-limiter=tbf`, which under the VPP hook silently becomes a policer: Ze's exact-or-reject forbids that. ng's `vpppolicer_remove_limiter` still unbinds and deletes before it disables the input features, which AC-15 forbids: do not copy its order |

The smallest correct Ze shaper matches the kernel TBF that `translateQdisc`
(`internal/plugins/traffic/netlink/translate_linux.go`) programs: rate; burst
of rate/8 bytes (125 ms, floor 2048); limit of 1 s of bytes (floor 2048); one
FIFO. In VPP: a feature on `ip4-output` and `ip6-output` keyed by TX
sw_if_index, one FIFO of buffer indices with a byte limit and tail drop, a
virtual-time token bucket with a burst cap, an owner thread with handoff, a
polling input node per owning worker, and a barrier-protected disable plus an
interface-delete hook. No flow hash, DRR, COBALT, tins or aggregates: a few
hundred lines. Ze also needs an in-place rate update (AC-10, C-2), which
osvbng lacks. Upload stays policed (the patch 0018 path).

Mechanisms it extends: the rate policy seam (`subscriber/rate_policy.go`: the
shaper becomes a second controller beside `policerRates`, download on the
shaper, upload on the policer, owner command and CoA unchanged); the
registered rate capability (police becomes police plus tbf; htb stays refused);
the `qdisc-type` enum (`shaper/yang/ze-l2tp-shaper-conf.yang`), which bears on
C-1; the VPP build (the `third_party/vpp-patches/` Dockerfile's plugin list,
the plugin directory as a build-time diff); the bindings (`./le build vpp
update`, a fourth package in `Dockerfile.api`); ownership tokens.

Tension to put to Thomas with the spec: the global traffic-vpp backend already
accepts htb and tbf and programs them as policers
(`internal/plugins/traffic/vpp/verify.go` `verifyQdiscType`: "A SINGLE class
binds one policer to interface output"), while OD-4 and AC-9 refuse the same
substitution for subscribers. A VPP shaper would make traffic-vpp's
single-class tbf exact too.

Licensing, facts only: Ze is AGPL-3.0; VPP and Ze's carried patches are
Apache-2.0 (`third_party/vpp-patches/LICENSE`); osvbng's plugin is
GPL-3.0-or-later. `ai/META-DECISIONS.md` keeps changes to carried third-party
code under that code's licence and code Ze writes under Ze's. Route A, Ze
writes its own plugin: as a diff in the VPP tree it is Apache-2.0 and can be
offered upstream (OD-1); whether a wholly new plugin directory is a
third-party patch (Apache-2.0) or Ze-written code (AGPL-3.0) is an owner call.
Route B, carry osvbng's plugin: it stays GPL-3.0+, VPP plus the plugin as
shipped is a GPLv3 combined work, bindings generated from its `.api` compile
into the AGPL binary, `spec-vpp-embedded-runtime` ships VPP inside Ze's
artifact, and it brings about 4k lines TBF does not need, no rate update, and
a 26.06 target.

Recommendation: a new post-release feature spec (a search of `plan/` for cake,
hqos, osvbng and VPP shaping found none; `spec-traffic-ingress-shaping` is the
kernel IFB path; OD-4 here already frames shaping as a later mechanism). Its
first deliverable is a TBF-only egress shaper plugin in Ze's VPP build on the
session interface, with enable, update in place, disable and dump messages; the
rate capability accepts tbf for download, upload stays policed, htb stays
refused; teardown under churn is proven in the real-VPP lab (the AC-15
analogue, same VPP PID). CAKE, AQM and aggregates come later. AC-9's refusal
stays and names that spec as what lifts it. Licence route: owner call, above.

Proof only a known host gives: pacing accuracy, burst, enqueue cost, idle-poll
cost at thousands of sessions, latency and jitter, handoff drops under RSS
skew, DPDK against af_packet. Proof anywhere: rate bounded, queue rather than
drop, in-place CoA, teardown. Not verified: that the output arcs run for
pppoe_session TX in Ze's 25.10 (the policer design assumes it too); that
osvbng's plugin compiles on 25.10; how osvbng applies a CoA to a scheduler.

Doors: `ai/doors/system.md`, `ai/doors/forwarding.md` (vpp, iface, traffic), `ai/doors/access-security.md` (l2tp, PPPoE, PPP).

## Implementation progress — 2026-11-19

This spec remains in phase 2/7. Implemented foundations land in separate commits
while their remaining verification and dependent implementation stay open.
An implementation commit does not close an acceptance criterion by itself.

| Work | Current state | Evidence and remaining work |
|------|---------------|-----------------------------|
| Original kernel PPP unit creation and counters | Committed in `1bd37283ed`; typed callers integrated | The reached kernel package passed with race detection; all 15 creation roots passed on Ze Linux 7.3, with no skips. The original file and creation-bound counter reader survive rename and refuse foreign or reused resources. Typed constructors and callers are now integrated; their native consumer acceptance remains open. |
| Counter guard discrimination | Verified for the creation library | C01–C06 each produced its intended semantic failure and a restored pass. C02's two restored native roots passed. Suppressing the original reader error in C06 failed the backend-closure assertion; after removing the overlay, all 15 creation roots passed again on Ze Linux 7.3 with no skips. Canonical files stayed unchanged and the overlays were removed. |
| Native creation rejection | Committed in `db1ba06e7b`; scoped Linux population passed | The original serialized request header and complete validated reply batch distinguish a definitive negative acknowledgment from an uncertain creation outcome. All 58 selected counter and missing-link roots passed on Ze Linux 7.3, with no failures or skips (`job-counter-create-rejection-native-population-691e138c.log`); check, command and lab exits were zero and the guest stopped. This includes ten new rejection roots, the real exclusive-name collision with its original reader still productive, and all three missing-link producer regressions. C05–C10 discrimination, scoped lint and the typed setup retry integration remain owed. |
| Shared kernel CoS cleanup and diagnostics | Committed in `a8101579b4`, with the native error repair in `1c2050e529` | All 94 CoS package roots and the registered cleanup YANG tests passed with race detection. Four RFC fixture judgments are recorded. Native replacement cleanup exposed an invalid missing-link error that panicked during rendering; preserving the original ENODEV fixed the unchanged test, then all six reached native CoS controls passed without skips. All three new missing-link producer regressions subsequently passed in the 58-root Linux run. Wider native VLAN, functional, RFC proof, discrimination and lint obligations remain open. |
| VPP process, socket and statistics ownership | Committed in `83b7c9c3c9`; focused lifecycle and override repairs passed | Four lifecycle roots passed after reproducing a second manager during pending cleanup. The full package then exposed eight original-override byte failures; their unchanged focused roots passed in `job-unit-pkg-66932b38.log` after the reader/writer repair. Readiness reporting is implemented. Complete package/Linux reruns, lint and subscriber restart acceptance remain owed. |
| Guarded server and CPE writer consumers | Committed in `83b7c9c3c9`; eight package/race runs and final focused additions passed | The original-unit consumers passed all eight packages in `job-unit-pkg-53fa613e.log`. Final SDK cleanup diagnostics, physical-retirement projection and override fidelity controls passed across three packages in `job-unit-pkg-66932b38.log`. New warning sites now reach declared doctor reports; introduced-change commit gates passed. Genuine Linux fault, causal, lint, documentation and complete subscriber acceptance remain owed. |
| Captured channel and session ownership | Committed in `668311841c`; all six reached package and race runs passed | The 194-file dependency closure carries typed session, worker, factory, client, relay and original-file query ownership. The latest `job-unit-pkg-6ba0b2be.log` records all six reached packages passing; kernel and outer L2TP integration packages also compiled for Linux arm64. The run includes the factory signature repair and independently approved realm fixture corrections. An unchanged PADS-before-PADT test exposed a missing pending-End wake in the exact-owner transition; that producer is repaired. Wider Linux consumer proof, native writer binding, scoped lint and complete subscriber acceptance remain owed. |
| Passive original-unit retirement | Integrated; reached package and race run and both genuine Linux roots passed | The same created unit now joins its actual file close, short descriptor borrows and retained native-release duplicate. It keeps namespace cleanup and physical retirement outcomes separate. Alias refusal preserves both original error causes without closing either original. Ten logical controls are included in the passing kernel package run. Both genuine native roots passed in `job-ppp-original-unit-retirement-native-6f2512e9.log` on the matched Ze Linux 7.3 kernel and image, without skips, with all exits zero and the VM stopped. Scoped lint and retirement discrimination remain owed. |
| Current iface/netlink checks | Linux compilation and both repaired native roots passed | The broader native run (`job-native-access-lint-fixed-native-population-2f2483cd.log`) exposed an overlong fixture name and a retained-sibling progress failure under persistent timeout, followed by a secondary synctest deadlock. Cleanup now ends its spent turn without advancing past another sibling; the fixture uses a valid name and retains every lifecycle assertion. Both unchanged questions passed on Ze Linux 7.3 in `job-native-access-two-failure-rerun-58cfa4eb.log`, without skips, with all exits zero and the guest stopped. Independent source reviews cleared the product repair and the fixture correction; the native fixture approval is recorded. 2026-11-22: the whole iface/netlink integration package (host Ze Linux 7.3, race, `-ppp-provider-required=true`, `job-netlink-integ-f0a47066.log`) passes 293 roots; only `TestVLANQoSEgressPCPOnWire` and `TestVLANQoSDSCPFullChain` fail, because this kernel lacks `CONFIG_NF_TABLES_NETDEV` (added at Thomas's next kernel rebuild; `plan/journal/unwired-feature.md`, 2026-10-28). The morning run had 17 failures. Fixed on the way: the settlement history (`0c93a81fd2`); the PPP unit liveness probe, which returned EINTR and dropped a renegotiating CPE session under load (`1a92721b86`, `TestOriginalPairLiveSurvivesSignals` red then green); five fixture defects (`bc776b7a2f`). Scoped lint of the package is clean. Causal proof remains owed. |
| Atomic attachment and channel capture | Matching kernel and image built; native primitive population passed | Strict GNU application through `0011c` and the full arm64 kernel/module build passed in `job-ppp-original-guard-kernel-context-fixed-8ad4ac45.log`. Kernel SHA-256 is `b56187bb8c65f3987e4a4d4b48a6c15f951dcb6938ee0fabb74a21913a17fba7`; its matching guest image is `411acf54456d0db2`. The 84-root native population below passed on that pair. Original-file Go consumers, discrimination and complete IPv6 socket/packet guards remain owed. |
| Native original-file guard tests | All 84 native creation/guard/capture/support/pair roots passed | `job-ppp-native-complete-original-guard-population-22c9e783.log` records 15 creation, 28 original-file guard, 16 channel, 11 capture, four global-support and ten exact-pair roots passing, including all 162 prefix cases. No failures or skips; matched kernel `b56187bb...7fba7`, image `411acf54456d0db2`, all exits zero and VM stopped. The earlier old-kernel `ENOTTY` remains ABI-absence evidence only. Go session consumers and complete IPv6 authority remain separate. |
| Native channel and capture tests | Original-file capture and lifecycle population passed on the matching kernel | All sixteen channel and eleven atomic capture roots passed in the same 84-root native run, including queued handoff and concurrent cleanup joins. This establishes the carried kernel boundary on arm64; it does not establish shared Go caller integration or 32-bit execution. |
| Guarded Go PPP writer | Provider and ACK/history repairs implemented; four native provider roots and ten model roots passed | `job-ppp-genuine-replacement-writer-native-067735d2.log` records the actual same-index genuine PPP replacement, renamed original, foreign creator and IPv4/IPv6 address/route controls passing, with the reached models, no skips and all exits zero. The guest stopped. The replacement fixture received independent approval and retains its strict ESTALE and survivor checks. Terminal retirement, consumer wiring, race and scoped lint remain owed. |
| Writer terminal custody and physical close | Implemented; five native provider roots and fifteen model roots passed | `job-ppp-writer-terminal-reached-population-067735d2.log` records all twenty roots passing without skips on the matching kernel/image. Five terminal model bodies first failed in `cb95ea42`. Real exact-socket close refusal then exposed twelve custody/teardown failures in `54996aa5`; the unchanged native question passed after retaining the physical-close error separately from historical write errors. Independent judgments cover the file-close oracles and both conservative custody-model corrections. Five further genuine unit/duplicate-retirement native tests still depend on the shared consumer integration; race, lint and full subscriber acceptance remain owed. |
| Constructor cleanup custody | Implemented and independently reviewed; all 23 reached provider/model roots passed | `job-ppp-constructor-custody-native-red-2247b440.log` reproduces loss of a still-live original socket when Bind, name lookup or creator probe fails and physical Close is refused. All three unchanged native questions passed after the repair, followed by all 23 reached roots in `job-ppp-constructor-custody-reached-population-067735d2.log`, without skips, with all exits zero and the guest stopped. The refused partial writer retains the exact socket and both causes with zero mutation features. The wider genuine shared-consumer matrix, race and lint remain owed. |
| IPv6 original-unit control sockets | Complete kernel/image built; real Go API passed four models and thirteen native roots | Kernel chain `acc1e3798d` plus routing repair `73108145a8` passed strict qualification and the full arm64 build. Kernel SHA-256 is `cddff82e255fc9a7010a79d5237a733463569f495ae483b60962455788fb3884`, cache `77b841ae-5d7f6651`; matching image is `f92054020556fb26`. The useful API test first failed at the refusal scaffold in `99fcf86c`. The actual producer then passed all seventeen selected roots in `job-ppp-control-socket-real-api-native-population-2b2b0e6c.log`, with no skips and all exits zero. The scaffold is gone. Packet/carrier, multicast, fragment, race and lint acceptance remain owed. |
| Current broader creation/guard population | Whole `ppp/kernel` package passes (2026-11-22) | The 2026-11-19 failure of `TestCreatedUnitNativeRetirementJoinsRetainedOriginalDuplicate` (`job-ppp-complete-current-core-guard-native-population-c1e8b184.log`, `ppp: original native mutation authority is unavailable`) is resolved by the fixture correction in `83b7c9c3c9`. The package now passes whole: host race 333/0/0 (`job-ppp-kernel-host2-abdd45ee.log`), and Ze guest on runtime tree `2b23af13` 334/0/0 (`job-ppp-kernel-guest-fixed-1b124a49.log`). Eight further red tests were found and fixed on the way. All were test defects that had kept roots from ever running. In `ceca4252b0`: three tests opened `/dev/ppp` outside a user namespace (`ppp_open` needs `CAP_NET_ADMIN` in the opener's namespace), and three supplier-capture tests used a 16-byte dummy link name that exceeded `IFNAMSIZ`. In `ad00b94b4e`: two bridge owner-guard tests needed `CONFIG_BRIDGE_VLAN_FILTERING`, which Ze's kernel lacks; the fixture was reworked with judge approval, and `spec-iface-bridge-vlan-filtering` AC-6 restores the VLAN assertion. Separately, and not a red test, integration-only helpers moved out of the production files for lint (`f1fee74f36`). Also `./le setup ppp-access` was corrected (`865f67540e`), and the PPPoE counter fixture now forwards `BindNativePPPUnit` (`2f045d3c0a`), so five counter roots pass in the guest. Logs are under `tmp/session/2026-11-22-183517/scratch/`. |
| Terminal settlement history | Repaired and validated (`0c93a81fd2`, 2026-11-22) | The model root and both genuine ACK/NACK roots failed before the repair and pass after it, with all 79 model and 10 provider roots under race. The CPE consumer expectations that encoded the dropped history were corrected under independent judgment (`bc776b7a2f`). The PPPoE client logs a released original's non-pending cleanup result as released with errors, not as a failure. |
| Compiled IPv6 control-socket declaration | Known bit9 recognized; runtime checks pending | The global support validator recognizes the enrolled control-socket ABI bit while retaining older valid declarations and unchanged session/bridge requirements. Two independently judged test updates retain unknown-bit11, malformed-reply and cleanup refusals. This recognition grants no original-unit or socket authority and does not activate the constructor. Current Linux execution remains owed. |
| Full VPP subscriber service | Open | Forwarding, IPv6, policing, restart recovery, final accounting samples, release ordering, interoperability and scale acceptance are not complete. |

## Resume checkpoint — 2026-11-19

The owner paused this session for token availability and requested committed
source and an explicit continuation record. This remains phase2/7, not closed.
Other agents may continue their separately owned work. Recheck the current tree
and this record before editing; their changes can supersede this snapshot.

1. **Root provider history:** done in `0c93a81fd2` (2026-11-22), red then
   green on the model and both genuine ACK/NACK roots. The five genuine
   retirement questions pass on 2026-11-22 (checkpoint table, "Native
   validation still owed").
2. **Shared-session owner:** done 2026-11-22. No session held a live claim on
   any L2TP or PPP spec. The false-release projection was already in the tree:
   `CreatedUnit.nativeRetirementPending` treats a failed original close as no
   retirement proof, so `NativeReleasePending` stays true after the writer and
   the tracked duplicate close. The retirement controls and causal cuts in the
   checkpoint prove it, and the retained-duplicate failure is resolved.
   Server wiring, proven in `2c9fbd7770`: `TestServerNativeIPv6CarrierFollowsOriginalUnit`
   and `TestServerNativeIPv6TeardownReleasesOriginalUnit` run a real PPPoE
   server session (production discovery dispatch, `newInterfaceServer` over
   the prepared kernel access, `ppp.NewProductionDriver`, a scripted
   subscriber). Carrier: after the original `pppN` is renamed and a dummy
   takes its name, the RA and a DHCPv6 Reply still arrive on the session, and
   no RA or DHCPv6 server frame reaches the dummy. Teardown: PADT, released
   cleanup, then the original index is gone. Both pass in the Ze guest on
   runtime tree `2b23af13` (`job-pppoe-v6-guest-e504a046.log`). Cuts: a
   name-based carrier (the carrier files from before `c63d11d484` plus
   `session_ops.go`, as a build overlay) turns the carrier root red and leaves
   teardown green (`job-pppoe-v6-cut-guest-0a433bba.log`); a retirement that
   never completes turns teardown red and leaves the carrier green
   (`job-pppoe-v6-cutrel-guest-e7aadaf7.log`). Logs are under
   `tmp/session/2026-11-22-203554/scratch/`. The release seal and its retained
   duplicate have no production caller, correctly: only a transport with
   remote rate operations owes the seal, and production has none (the comment
   that said otherwise is corrected). The PPPoE client has no IPv6 carrier:
   the kernel's `accept_ra` takes the RA, and DHCPv6 is the iface DHCP client,
   by name. Its native release runs its own path, proven by
   `TestNativeCPEConsumerKeepsOriginalAcrossNameReplacement`. L2TP shares the
   session and driver layers, not the construction.
3. **Packet acceptance:** done 2026-11-22. The old carrier's useful
   renamed-original failure was observed first: on runtime tree `6db1b4aa`
   `TestNativeIPv6CarrierOriginalTraffic` passed and the other eight
   `native_ipv6_carrier_owner` roots failed
   (`job-pppoe-dp-kernel-guest-04070357.log`). That is the causal red for the
   adoption. The kernel session carrier now opens its ICMPv6 and UDPv6 sockets
   from the original unit (`CreatedUnit.OpenIPv6ControlSocket` inside
   `WithOriginalNamespace`), joins multicast on interface 0 and names no zone.
   The name-based path and `SO_BINDTODEVICE` are deleted with no fallback:
   sessions already require Ze's patched kernel through the native PPP binding
   (`c63d11d484`). The adoption exposed a defect in kernel patch `0013b`:
   Linux 7.3 `udp_try_make_stateless` reset the skb extensions at UDP enqueue
   and erased the PPP RX receipt, so every datagram to an owned UDP6 socket was
   dropped. `0013b` now keeps such a packet stateful while the receipt remains
   (`ppp_control_scrub`, `5ab4769de0`). The new root
   `TestCreatedUnitControlSocketNativeUDP6ReceivesPeerDatagram` is red on
   `6db1b4aa` (`job-udp6-red-before2-140821db.log`) and green on the rebuilt
   tree `2b23af13`. On `2b23af13` all nine `TestNativeIPv6Carrier*` roots pass,
   and the whole `pppoe/dataplane/kernel` package passes 42/0/0
   (`job-pppoe-dp-guest-fixed-b7ce7151.log`). These logs are under
   `tmp/session/2026-11-22-183517/scratch/`.
4. **Fragment draft:** done 2026-11-22 in `169eccfa2c`. The five roots
   `TestNativeIPv6Fragment{Useful,DistinctOwners,RetiredOwner,Rename,OrdinaryEthernet}`
   and the R2 helper were promoted unchanged from the reviewed draft, with the
   graft applied and only lint edits. The draft directory
   `internal/component/l2tp/pppoe/dataplane/kernel/testdata/ppp-ipv6-fragments-draft/`
   was removed. In the Ze guest on runtime tree `2b23af13` all five pass, and
   the whole `pppoe/dataplane/kernel` package passes 47/0/0
   (`job-pppoe-dp-frag-guest-b7ce7151.log`). Discrimination: for one build,
   the carrier's four product files went back to their name-based versions
   from before `c63d11d484`. `DistinctOwners` and `RetiredOwner` then received
   another owner's datagram, `Rename` never admitted the renamed original, and
   all eight carrier ownership roots went red. `Useful`, `OrdinaryEthernet` and
   `TestNativeIPv6CarrierOriginalTraffic` stayed green
   (`job-frag-cut-guest-ae0a386b.log`). This run is also the carrier
   adoption's causal cut on the fixed kernel. Logs are under
   `tmp/session/2026-11-22-183517/scratch/`.
5. **Accounting and finishing checks:** counter R6 is committed in
   `1a6b966648`. Accounting done 2026-11-22 in `38267f5e0c`: RADIUS
   accounting (`authradius`) reads counters only from the session's original
   PPP unit, through the L2TP network-qualified capture. The Start baseline
   comes from the IP-assigned payload, each Interim from the live reading, and
   the Stop from the terminal measurement, awaited off the publication
   callback (on a peer CDN, an admin clear or a timer the Down is published
   before PPP measures). A missing measurement omits the counters, never sends
   zero, and raises `doctor-radius-accounting-counters`. Evidence: the
   package passes; the guest `.ci` tests `radius-acct-wire`,
   `slow-accounting-control`, `session-stopccn-cascade` and
   `session-clear-leaves-no-subscriber` pass; three cuts are red; four RFC
   records are re-taken; every test change is judged. Route receipt (AC-24)
   done in `0a92e78161`: `TestServerNativeIPv6FramedRouteOwnedByOriginalUnit`
   installs a PAP Framed-IPv6-Route on the original unit, keeps it across the
   rename, and withdraws it exactly at IPv6CP close while an operator route to
   the same prefix stays. It passes in the guest, and the cut "no intent
   recorded before the native add" is red
   (`job-pppoe-cutroute-guest-d52c06a3.log`). Remaining, each with its owner:
   - (a) PPPoE subscribers send no RADIUS accounting at all:
     `plan/release/fixes/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md`,
     amended 2026-11-22 to count the original unit, never a name.
   - (b) `ClaimAccountingAssignment` and `AccountingAdmission`
     (`ppp/auth_admission.go`) have no production caller, so no accounting
     capacity is reserved before Accept. It needs a design; this spec owns it
     (`invariant-enforced-by-an-absent-call-site.md`, 2026-11-22, Owned
     journal rows).
   - (c) The L2TP idle timeout reads sysfs by name and reads a failure as
     idle: `zero-value-as-valid-answer.md`, 2026-10-28, owned by
     `spec-lifecycle-invariants`.
   - (d) PPPoE has no Idle-Timeout or Session-Timeout. The VPP part of AC-17
     stays in this spec.
   - (e) AC-8 on the kernel data plane: production starts the IPv6 service
     with no prefix allocator (`afterLCPOpenIPv6Service` passes nil to
     `startIPv6Service`, and `GetPrefixHandler` has no production caller), so
     a DHCPv6 Request gets no answer and a Solicit gets NoPrefixAvail; no
     PPPoE or L2TP subscriber can obtain IA_PD. The server tests above
     exposed it. `spec-l2tp-ipv6-subscriber` owes the wiring, and
     `unwired-feature.md` (2026-08-30) records it. This spec's AC-8 waits on
     it.
   - (f) The kernel subscriber shaper applies traffic control by interface name, against
     "Native PPP writes" above
     (`identifier-reused-after-its-owner-is-gone.md`, 2026-11-22, Owned
     journal rows).

   Root still owes scoped race/lint, docs and final independent review of
   the socket/carrier/provider changes. No whole-tree verification or push is
   authorized by this checkpoint.

CONTROL6 tests now run on the amd64 runtime tree `2b23af13`, built by QEMU
(cache `~/.cache/ze/runtime-kernel/7.3-runtime-amd64-runtime-qemu-2b23af13-5d7f6651`).
The arm64 pair `cddff82e...`/`f92054020556fb26` remains the earlier arm64
evidence. The earlier `b56187bb...7fba7` kernel and `411acf54456d0db2` image remain retained for other
sessions; do not mix the pairs or evict them without checking their users.

Decisive logs and the pending history proposal/judgment are under
`tmp/session/2026-11-15-125434/scratch/`. The existing detailed owning record is
`tmp/session/2026-11-15-204844/state/session-state-l2tp-session-identity-and-publish-stall-0636cd44-05d3-4d15-a39a-ca76b35afc15.md`,
section “ROOT PPP native guard”. Retain the small current history judgment and
source qualification manifest until the unresolved regressions have RED/GREEN
and canonical evidence; then remove superseded temporary inputs. Kernel layout
evidence currently measures selected compiled types only. Old/new native slab
geometry remains unmeasured; no allocation-rate or complete layout claim follows.

Main must rebuild the Linux integration ELF from the current main checkout,
with `CGO_ENABLED=0` and the SSOT feature tags plus `integration`, before running
native questions. Check `le job report`, run one CPU-intensive job at a time,
and use `le test qemu docker-lab` with the explicit matched kernel/image.
The provider history selector is
`^TestNativePPP(ModelSettlementHistorySurvivesPhysicalClose|ProviderSettled(ACK|NACK)HistorySurvivesClose)$`;
the socket API selector is `^TestCreatedUnitControlSocket` with
`-ppp-control-socket-required=true`; provider native runs require
`-ppp-provider-required=true`. Existing binaries are snapshot evidence, not
proof of code another agent changes after this checkpoint.

The creation-library results do not prove its pending session-constructor or
subscriber integration. Commits record existing evidence and outstanding
checks without claiming full support. The native CoS fixture retains its
prior expected-device assertion at the original-target capture boundary;
its survivor, initial-conflict and CoA-conflict roots passed in the six-control
native rerun.

## Task

Ze's PPPoE access concentrator terminates every subscriber session in the Linux
kernel: an AF_PPPOX socket per session, a `/dev/ppp` channel and unit, and a
`pppN` interface that netlink addresses and traffic control shapes. The owner decided that Ze
will also terminate PPPoE subscriber sessions in the VPP data plane, so that
subscriber traffic is encapsulated, decapsulated, routed and policed by VPP at
DPDK speed while Ze keeps the whole control plane (discovery, LCP, auth, NCP,
DHCPv6, RA, RADIUS, accounting).

The shared kernel handle also retains exact route ownership before attempting
a native add. A failed add or withdrawal keeps its cleanup intent; an invalid
or unattempted tuple cannot remove another owner's route. This repair belongs
to the shared seam introduced by this feature.

The seam must preserve the full PPPoE access-interface identity through
admission and every PPP event. Numeric protocol labels must not select a
worker: an opaque admission binding must identify its exact owner, including
after identifier reuse. Kernel subscriber shaping must own an independent kernel
backend and durably acknowledge its original-state snapshots, whether its
plugin runs internally or in another process.
The VPP connection must also prove the physical identity of its API, statistics
peer and supervised child before publishing readiness or retiring their state.

Operator recovery must reach every registered subsystem during reload. A
failure from an earlier subsystem must not prevent a later transport from
retrying its retained cleanup, and the result must preserve every failure cause.

Daemon shutdown must keep a provider usable until its declared dependents
finish engine cleanup. Canceling every plugin together closes VPP while
interface cleanup still needs its API. Shutdown must respect the selected
dependency graph and retain its distinct contract: it does not invoke the
removal-only `OnBye` callback.

This lifecycle work also owns the defects its scoped server tests exposed.
A startup that observes its terminal stop skips remaining readiness steps and
cannot turn that stopped outcome into success or clear a successor's stopped
record. A callback admitted while the lifetime was live may finish after Stop;
completion must not leave reentrant startup callbacks deadlocked. A genuinely
completed startup keeps its historical result when the server stops later. An intentional
restart must retire the old fatal plugin's logical ownership before stopping
it, while retaining its physical join; a genuinely unexpected fatal exit must
still stop the daemon.

The same server checks require operational command recognition to follow the
actual quote, pipe and keyword-boundary grammar. Its catalogue must include
predeclared subsystem commands and replace that declaration at the real
registration stage. Usage must render the command registry's current route-kind
values. These are prerequisite repairs, with their original questions and
independent test judgments preserved, rather than exclusions from the tests.

Shared kernel CoS also needs a permanent recovery command:
`ze request class-of-service cleanup`. It makes one bounded pass over failed
original owners and preserves healthy members, pending work and compensation
custody. Recovery uses the captured original native grant and dispatcher,
joins an uncertain operation before compensation, and confirms readback before
clearing custody. The first subscriber's departure must not retire that grant
while shared survivors still need it. A current device name or global backend
lookup cannot replace this authority.

The recovery warning must retain its original cause, fix and resource owner
after the recipient process ends. Process cleanup currently clears that
warning after joining admitted RPCs. Recovery must preserve resource-owned
diagnostic custody and clear it only on proof for that same original resource
and failure episode; a replacement process cannot clear it by name.

The real SDK test helper must also wait for runtime initialization before
returning its engine. It must join the existing runtime-ready receipt on both
wire and bridge transports, while serving initializer RPCs, so the first
command reaches the initialized handler. An earlier Stage 5 answer, fabricated
readiness or command retry cannot replace that receipt.

The migration also keeps interface-owned-device cleanup registered through
the current process-generation callback API, with its original process-pointer
guard intact. The callback arity repair keeps the shared tree buildable.

Offline backend validation also preserves typed causes and their diagnostic
codes. This work repairs the interface gate's lost causes and gives the parse
reader shared Linux admission and Linux-only selection. The default-kernel
acceptance fixture declares its access interface and omits the backend.
Its Linux guest execution and 80-run stress passed. Current parser checks,
lint and documentation checks, and the remaining feature acceptance are still
owed.

AC-1 also needs a real DUT with `ze_l2tp` present and `ze_vpp` absent.
The generic and parse runners admit that fixture from the selected artifact's
embedded Go build tags. Unknown metadata fails; a known mismatch skips before
resource acquisition. Declaration errors still fail before host admission.
The unchanged no-VPP refusal and the full-VPP prerequisite questions must each
execute on their matching build; modeled metadata tests alone do not prove
the CLI contract.

The adopted VyOS receive parser is also hardened in this work: its
`pppoe_input_node` and control-dispatch trace read fields beyond a truncated
logical payload without a preceding length check. This is a source-confirmed
logical bounds defect; no crash has been demonstrated. A separately retained
Ze diff and malformed-frame proof are part of Phase 0, before the image is
used as mechanism evidence.

Phase 0 must also fix `pppoe_cp.c` `pppoe_add_del_cp` in the adopted patch.
Its hardware lookup uses a software interface index as a hardware pool index,
and its delete path writes the map without checking that the pair exists.
The unchanged C regression demonstrated the absent-delete failure in the
native lab; VPP remained responsive. A separate Ze diff must prove correct mapping when the index
values differ, safe refusal of invalid deletes, and removal of a valid pair.

The native ARM lab build also exposed an upstream CRC32C portability defect:
the session worker hash calls scalar helpers absent from the software CRC
branch. Phase 0 supplies those seeded helpers without assuming hardware CRC
instructions, and runs an independent 29-case C regression during the build.

The shared ownership migration also covers the existing Service-Name and realm
relay lifetimes. An accepted relay must end in both directions, shutdown must
release admissions still waiting to bridge, and an old call's delayed cleanup
must preserve a replacement using the same numeric identifiers. A full control
queue must not prevent the PPP reader from joining when its session ends.

The goal has five parts:

| # | Goal |
|---|------|
| G-1 | A PPPoE client reaching an access interface owned by VPP completes discovery, LCP, authentication and IPCP/IPv6CP against Ze, and its IP traffic is forwarded by VPP, never by the Linux kernel |
| G-2 | The kernel and VPP session data planes are two registered backends behind one seam. The configuration selects exactly one; neither falls back to the other (`ai/rules/no-layering.md`) |
| G-3 | Every per-session data-plane operation Ze performs today on `pppN` (MTU, admin state, peer address, framed and delegated-prefix routes, VRF, rate limit, teardown) has a VPP equivalent or is refused at verify |
| G-4 | Subscribers get IPv6 (RA, DHCPv6 IA_NA and IA_PD) on the VPP backend without kernel sockets bound to `pppN` |
| G-5 | A VPP restart or a lost GoVPP connection leaves no stale session state in Ze or in VPP, and service resumes without operator action |

### Reference design: accel-ppp-ng (VyOS)

`/home/thomas/Code/github.com/accel-ppp/accel-ppp-ng` already does this for VyOS.
Read at the source, not from its README:

| Seam | What ng does | Proof |
|------|--------------|-------|
| Hook table | A session-hooks interface registered by name; the PPPoE server looks up the `"vpp"` hooks when an interface is configured `vpp-cp=true` | `accel-pppd/include/ap_session_hooks.h`, `ap_session_hooks.c`; `ctrl/pppoe/pppoe.c` (`parse_server`, `ap_session_hooks_find("vpp")`); `session_hooks/vyos_vpp/vpphooks.c` (`vpp_hooks`, `DEFINE_INIT`) |
| Control-plane interface | PPPoE runs on the Linux side of the access interface's LCP TAP. VyOS maps the VPP access interface (dp) to its LCP TAP (cp) with `pppoe_add_del_cp(dp_sw_if_index, cp_sw_if_index)` | `vyos-1x python/vyos/vpp/control_vpp.py` (`map_pppoe_interface` reads the LCP pair: `vpp_index_hw` is dp, `vpp_index_kernel` is cp); `src/conf_mode/service_pppoe-server.py` (`apply`) |
| Session without `/dev/ppp` | The kernel AF_PPPOX socket is connected on the cp TAP but never attached to a PPP channel or unit; it is read and written directly as the PPP frame channel ("non-dev-ppp"). No `pppN` is created | `accel-pppd/ppp/ppp.c` (`establish_ppp`: `chan_fd = fd` when `is_non_dev_ppp`; `connect_ppp_channel`: `unit_fd = fd`, `ifindex = -1`); `ctrl/pppoe/pppoe.c` (`connect_channel`) |
| Session interface | `pppoe_add_del_session(client_mac, session_id, disable_fib=1)` creates the VPP `pppoe_sessionN` interface; ng then disables the `ip4-not-enabled`/`ip6-not-enabled` features on it and names the session after the VPP interface | `session_hooks/vyos_vpp/vpppoe.c` (`vpppoe_sync_add_pppoe_interface`, `vpppoe_set_feature`); `vpphooks.c` (`vpphook_pppoe_create_vpp_session_interface`) |
| Addresses and routes | Every address operation becomes a VPP route to the session interface: peer /32, framed routes, IPv6 /128 and delegated prefixes, all with `ip_route_add_del` | `vpphooks.c` (`vpphook_ipaddr_add_peer`, `vpphook_iproute_add`, `vpphook_ip6route_add`); `vppiputils.c` |
| Rate limit | Two VPP policers per session (`policer_add_del`), bound with `policer_input` and `policer_output`, the input one enabled as a feature on the `ip4-unicast`, `ip6-unicast`, `ip4-multicast`, `ip6-multicast` arcs | `session_hooks/vyos_vpp/vpppolicer.c` (`vpppolicer_install_limiter`) |
| IPv6 control | The adopted patch punts RS, RA and DHCPv6 inside PPP frames to the cp TAP; ng parses IPv6 out of the PPP channel and answers in PPP frames ("non-socket DHCPv6/ND") | Patched VPP `pppoe_decap.c` (`is_control_plane_packet`); `accel-pppd/ipv6/ppp_ipv6layer.c`, `ipv6/dhcpv6.c`, `ipv6/nd.c` (`is_non_socket_dhcpv6_nd`); commit `07f158d` |
| Teardown | Flush routes, remove policers, delete the VPP session | `vpphooks.c` (`vpphook_pppoe_terminate`) |
| VPP loss | At `90c6d35` a lost VPP connection set `is_vpppoe_lost` (`pppoe_on_vpp_connection_lost`) and stopped the VPP PPPoE services. At HEAD `7e4ec88` both are gone: `vpp_check_error` only disconnects, `vpp_get_vapi` reconnects lazily, and services stop and start only on cp TAP netlink events (`pppoe_on_del_link` with TERM_NAS_ERROR, `pppoe_on_add_link`). An API drop with VPP alive changes nothing: no resync, no identity check (corrected 2026-11-23) | `session_hooks/vyos_vpp/vpputils.c` (`vpp_check_error`), `ctrl/pppoe/pppoe.c`; touched by `2c00fde`, `d65aeee` |

ng drives the **upstream VPP `pppoe` plugin through VAPI**, not a custom plugin,
but the API it calls is the **VyOS-patched** one: upstream VPP 25.10 has no
`disable_fib` field and no per-interface cp map.

### VPP patches ng requires

VyOS builds VPP `stable/2510` with every patch in `vyos/vyos-vpp-patches`
(`vyos-build scripts/package-build/vpp/package.toml`). The PPPoE path depends on:

| Patch | What it changes | Why ng needs it |
|-------|-----------------|-----------------|
| `0013-pppoe.-Automated-session-management.-12.patch` | `pppoe_add_del_cp` takes `dp_sw_if_index` + `cp_sw_if_index` (a map per access interface instead of one global cp); `pppoe_add_del_session` gains `disable_fib`; LCP, IPCP and PADT sniffing for optional auto sessions | Several access interfaces; a session created before IPCP assigns an address |
| `0014-pppoe-Added-option-enable-pass-nd-and-dhcpv6.patch` | startup.conf `pppoe { enable-pass-nd-and-dhcpv6 }` punts ICMPv6 RS/RA and UDP DHCPv6 inside the session to the cp interface | IPv6 RA/DHCPv6 served from the PPP channel; the patch does not punt NS |
| `0018-policer-Added-features-for-unicast-multicast-arc.-29.patch` | Registers `policer-input` on the `ip4/ip6-unicast` and `ip4/ip6-multicast` arcs | Upstream `policer-input` exists only on `device-input`, which a PPPoE session interface never traverses |
| `0025-T8231-Removed-PPPoE-input-feature-setup-41.patch` | Stops enabling `pppoe-input` as a `device-input` feature on the cp interface | Non-PPPoE traffic was being steered into PPPoE nodes |
| `0031-PPPoE-Added-cp_binding_dump-API-call-49.patch` | `pppoe_cp_binding_dump` | Reconciling the dp/cp map after a restart or reload |

`0024-T7860-IPv6-ICMP-RA-punt-shortcut-36.patch` touches `pppoe_cp.c` but serves
DHCPv6 on LCP TAPs, not subscribers.

Ze additionally retains `third_party/vpp-patches/0099-pppoe-bound-frame-parsing.patch`
after these five originals. It bounds all three decapsulation lanes and the
control dispatcher by the actual buffer chain and declared payload length,
keeps discovery separate from the session protocol field, and provides an
optional lab-only runtime parser test. Its 31-case runtime proof and the
four receive-path checks passed in the Ze-kernel lab; the evidence is recorded
in the Phase 0 test table below.

The separate `ze/0100-pppoe-cp-binding-tests.patch` adds the
optional `test pppoe cp-binding` regression command under the same lab test
option. Its absent-map deletion must fail against the adopted implementation
before a separate Ze source fix is applied. The native lab observed
`unmapped deletion was accepted` before `ze/0101-pppoe-validate-cp-bindings.patch`.
The unchanged ten-case command passed after that repair in the same
Ze-kernel guest. The full packet mechanism proof remains separate.

### What upstream VPP 25.10 does without patches

Read in the upstream `v25.10` source (`src/plugins/pppoe/`, `src/vnet/policer/node_funcs.c`,
`src/vnet/classify/policer_classify.c`, fetched for this spec):

| Fact | Where | Consequence for Ze |
|------|-------|--------------------|
| Non-IP PPP protocols (LCP, PAP, CHAP, IPCP, IPv6CP) and discovery go to the CP node before any session lookup | `pppoe_decap.c` (`PPPOE_INPUT_NEXT_CP_INPUT`) | Control frames reach Ze before the VPP session exists |
| One global `cp_if_index`, set by `pppoe_add_del_cp(sw_if_index)` | `pppoe_cp.c`, `pppoe.api` | All access interfaces share one cp TAP; Ze cannot tell the ingress access interface |
| The CP node learns `client MAC -> ingress interface` in `link_table`, keyed by MAC alone; frames Ze sends out the cp are switched by destination MAC and get the access interface's MAC as source | `pppoe_cp_node.c` | The same client MAC on two access interfaces collides |
| `pppoe_add_del_session` takes `client_ip` and always installs `client_ip/32` (or `/128`) to the session in `decap_vrf_id`; it fails with `INVALID_SW_IF_INDEX` until `link_table` knows the MAC | `pppoe.c` (`pppoe_add_del_session`, `fib_table_entry_path_add`) | The session can be created only once an address is known; an IPv6-only session has no unique address at that time |
| IPv6 inside the session is decapsulated to `ip6-input` on the session interface; nothing reaches the cp | `pppoe_decap.c` | RS and DHCPv6 need VPP's own RA (`ip6_nd` API) and a punt socket for UDP 547, or patch 0014 |
| `policer-input` is a `device-input` feature; `policer-output` is on `ip4-output` and `ip6-output`; `ip4/ip6-policer-classify` run on `ip4/ip6-unicast` | `node_funcs.c`, `policer_classify.c` | Download policing works upstream; upload policing needs a classify table per session, or patch 0018 |

### What Ze vendors

`go.fd.io/govpp v0.13.0` (`go.mod`) ships `binapi/pppoe` generated from VPP
`25.10-release`, in the upstream shape (no `DisableFib`, single `SwIfIndex` cp).
It is not in `vendor/` because nothing imports it; neither are `punt`, `ip6_nd`
or `feature`. A patched VPP changes the message CRCs, so the govpp module's
binapi would no longer match and Ze would generate its own `pppoe` binapi from
the patched `.api`.

That inventory records the design baseline. The implementation now retains its
generated PPPoE and feature bindings under
`internal/component/l2tp/pppoe/dataplane/vpp/binapi/`; generation alone does not
prove the product backend's runtime behavior.

## Earlier implementation evidence

The feature remains in progress at Phase 2/7. The results below establish
bounded parts of the work; they do not demonstrate all AC-1 through AC-37.
Later-phase implementation drafts exist, but their full product acceptance
remains open. The original design, phase order and acceptance criteria stay
in force.

### Landed work and observed proof

| Work | Landed source and evidence | Limit |
|------|----------------------------|-------|
| Phase 0 mechanism | The Phase 0 test table records the real VPP plain, namespace and VLAN runs, parser and receiver bounds, binding regression, MTU boundary, and strict 29-case CRC32C build proof | This is mechanism evidence, not the complete Ze subscriber path. The native 55-group ownership suite passed after its route-withdrawal repair; its independent countercuts remain owed |
| Shared PPP foundation | `98b534756dec35376aeada38b97d0919a1b8b370` landed the admission and native cleanup ownership work across 238 paths. `internal/component/l2tp/ppp/start_session.go` `StartSession` now carries `Frames` and `Dataplane`; `internal/component/l2tp/pppoe/dataplane/registry.go` owns provider selection. Scoped PPP/Linux and RFC observations are recorded with that commit | The commit does not establish full kernel parity, shared native CoS custody or VPP integration acceptance |
| Cleanup callback compatibility | `28d3db6139e71f55f1caab87970ee141626a2ec0` repaired `internal/component/iface/cmd/owned_devices.go`; `e7e590078670a41bbfa7f917fafb021d8d72652f` adapted `internal/component/firewall/register_contribution.go`. `TestProcessGenerationCleanupIdentity`, the iface command package, and the three firewall contribution race tests passed; scoped host/Linux lint passed | The original process-pointer guards remain. The Linux-only namespace-mismatch test was not executed on macOS; callback compatibility does not prove physical recipient retirement |
| Proof and parser tooling | `9bf6af486404c844abe675731f50efffb728e347` retains explicit RFC guest images; `837213e1fa2c634711ce4570a1f59b459b258bdd` preserves concurrent approvals. `a3362aaad47f8fb3ed1c4251eb1e4264b7ab01d4` landed shared Linux admission and actual-DUT build-tag admission. The current runner race package recorded 545 PASS, zero FAIL and five known SKIP; scoped runner lint passed | Package results and modeled metadata checks do not prove matching-artifact CLI acceptance. The five skips are existing opt-in, host-capability and dedicated-entry requirements |
| Linux default backend | `test/parse/pppoe-netlink-backend-accepted.ci` executed in the Linux guest and passed its 80-run stress with zero skips | This establishes the omitted-backend fixture, not every PPPoE/L2TP kernel or VPP scenario |
| GoVPP transport and stub readback | `30e4099b2d` landed bounded native writes and shutdown, with its three transport race questions passing after observed blocked-write failures. The subsequent uncommitted stub inventory repair passed 14 unit questions and all seven original stub-backed functional tests. Cuts in `TestVPPStubZeroIndexDumpsAll`, `TestVPPStubNameFilterCaseOverridesIndex` and `TestVPPStubLoopbackFlagsFollowAdminState` failed semantically, then passed after restoration. The later eight-test native Linux suite includes the new shutdown test below and passed without skips | The stub results exercise native clients against modeled VPP. They do not prove physical VPP forwarding or subscriber acceptance |
| SDK test-engine readiness and peer context | `d0afd17842bd2102172aba876a741b582f13b5da` landed the runtime-ready join in `internal/test/pluginengine/pluginengine.go` `Start`. The held-initializer and first-command questions passed on both transports after their original failures; reached BGP transport regressions and three CoS controls also passed. `4700e17eb4` then added `ExecutePeer`, preserving `Execute`'s empty-peer default. Its actual SDK peer echo passed for IPv4, IPv6 and escaped arguments on wire and bridge; deliberately dropping the peer failed both transports, followed by a canonical pass. All six fixture package roots passed with race detection and scoped host/Linux lint reported zero issues | These are harness prerequisites. Supplied peer context does not authenticate a caller, and these results do not establish native CoS authority, warning recovery or lifecycle correctness |
| Command-result projection | `cdb4156509f137e6bfc2f89e023edab1f3d1ca48` landed the coherent SDK/RPC/daemon outcome repair across 21 Go files and three pages. The earlier 17 semantic failures and four controls became 21 passes. Eight compiler-only breaks failed at their intended assertions before restored passes. The later four-package race run recorded 1,775 PASS and two FAIL. The malformed-escape expectation received independent approval and three focused passes. The startup terminal-error repair then passed 14 focused race roots and the later complete server package | The two failures are accounted for, not an all-green verdict for the original run. Later adapter edits still need their own acceptance. The latest lint plan refused changed foreign inputs and produced no clean lint verdict. Documentation checks failed on catalog, artifact-prefix and seven foreign-anchor findings, with no selected core-page anchor failure |
| Real-daemon command outcomes | All 22 external-SDK, REST, TLS gRPC, MCP, CLI and web scenarios passed their draft baseline. They then passed 80 invocations each under 32 CPU burners and parallelism two: 1,760 named executions and 9,600 verification steps, all passing. All 22 were promoted unchanged and passed from the live plugin suite. Seven compiler-only breaks produced 11 intended semantic failures; all 11 cases passed after restoration. Both description-only test corrections also passed their draft baselines, 80 repetitions each and promoted live checks; all 800 verification steps passed. `5308422017` published the coherent 80-path adapter change | Broad adapter lint stopped before analysis because shared inputs changed during planning. Client-only lint found import grouping in the unselected `hidden_projection_test.go`; it did not clear broad adapter lint. The historical full plugin result remains 945 PASS, 108 FAIL, nine timeouts and 177 SKIP. The live outcome passes do not clear those failures |
| Resource-warning foundation | `6158b3211de8bb0fcbce4451b9c344379059f482` landed the independently reviewed generic report store and ordinary report-RPC changes. All nine new resource-warning questions, the complete report-store race package, report-RPC regressions and unchanged process-warning cleanup passed. Compiler-overlay breaks produced semantic failures for source cleanup, overwrite, ordinary clear, eviction, reservation capacity, copied ownership, old-episode clearance, and false RPC clear/raise acknowledgement; canonical tests then passed. Scoped host/Linux lint passed | This is the generic foundation, not native CoS warning recovery. Native scope binding, live compensation clearance and actual fork recovery remain open |

### Publication checkpoint — 2026-11-18

The owner requested coherent commits before all testing is complete. These
commits publish implemented pieces; the spec remains in progress. Later
validation must update this record rather than imply that publication cleared
the remaining acceptance criteria.

| Piece | Published implementation | Evidence and work still owed |
|-------|--------------------------|------------------------------|
| Original PPP unit owner | `1bd37283ed` retains the original creation echo, unit file and counter reader | The portable kernel package and all fifteen new roots in the matching Ze7.3 guest passed. Channel/session construction and transport consumers remain a separate migration; the library alone does not protect every native mutation |
| Original-file kernel guards and atomic capture | `945c562595` and `38cdd738b3` publish the reviewed link/address, route, dispatch/alias, channel/unit, atomic PPPoX capture, global support and exact-pair guards | Strict application, full arm64 kernel/modules and matching image builds passed. All 84 native primitive roots and 162 prefix cases passed without failures or skips on kernel `b56187bb...7fba7` and image `411acf54456d0db2`. The earlier `ENOTTY` was old-kernel ABI absence. Shared Go consumers, discrimination and complete IPv6 authority remain open |
| Original resource-warning details | `fde1cbe42d` validates and copies optional structured context before retaining an episode | Both new questions failed semantically before implementation and now pass with race detection in `job-terminal-cleanup-scoped-green-044c4183.log`. The implementation also passed independent source review |
| CoS cleanup recovery | `a8101579b4` publishes retained native-access recovery | Nine focused original recovery roots passed with race detection. Wider native, causal and functional acceptance remains owed; this publication does not complete subscriber recovery |
| CQM producer and shared documentation | `8f9c0fad72` publishes the producer, portable coverage and shared pages | The parallel CQM work recorded root, command and web race passes. Remaining fixture, Linux and accounting work belongs to their current integration records |
| Generic plugin terminal cleanup | `792c1469b6` retains an original process's reserved warning, terminal outcome and final cause, and closes its delivery queue on exit | All three server declaration roots pass with race detection in `job-terminal-cleanup-scoped-green-044c4183.log`. All ten actual Linux external-process roots pass in `job-terminal-cleanup-external-postpub-path-corrected-4dae34ca.log`, including the original join timeout, killed child, retained cause, full reservation and incomplete stderr cases. No skips; guest and harness stopped normally. Process tests could not compile because `recovery_delivery_integration_test.go:341` still compares a `uint64` peer identity with `netip.Addr`; its owner has the exact failure. Process execution, reached functional checks, lint and VPP opt-in remain owed |

The prepared-access change and its required VPP suppliers still need coherent
publication. Its earlier 408 scoped passes and five native passes remain
valid evidence for that boundary. Constructor-only socket diagnostics are
source-reviewed; post-handoff socket custody remains unfinished. The kernel
getter remains private. Original-file mutation guards are published; their
kernel, modules and matching guest image built successfully. All 84 native
primitive roots passed; Go consumer integration and full IPv6 authority remain owed.
Numeric channel connect/disconnect and IPv6 carrier authority also need their
own repair and native proof. None of these open items is marked complete.

### Observed defects and source-only work

| Area | Actual result or current source state | What remains unproven |
|------|---------------------------------------|----------------------|
| Canceled VPP control-socket cleanup | Six deterministic Linux tests ran through the existing worker in Ze7.3. Four failed semantically: failed Close had no retained warning, source clearing and healthy successors had no protected episode, and full diagnostic capacity still reached the opener. Both ownership controls passed. The guest stopped normally | Reserve the original warning owner before native work, preserve its cause and request identity, and keep healthy successors from clearing the failed episode. Also retain cleanup or namespace-restoration errors returned before socket publication. Confirm cleanup ordering and the operator repair path; the fake endpoints do not establish native socket retirement |
| Plugin terminal cleanup | The daemon-owned terminal outcome implementation is published. Three server declaration race roots and all ten actual Linux external-process roots now pass after the recorded semantic failures and join timeout. The first post-change guest invocation copied from the wrong path and ran no tests; the corrected invocation supplied the actual passes above. The VPP manager and exact-PCI driver restoration fixes are integrated: all nineteen focused race roots passed. Three real Linux VPP child/Wait questions still await the post-fix run | Finish process coverage after the shared fixture migration, reached consumers and lint. The VPP adapter remains disabled: its lifecycle can consume a failed stop result and attempt rollback restart after unresolved cleanup; that producer needs its own tests and repair before truthful opt-in. Finish the separate socket-diagnostic implementation and post-handoff custody. Portable NIC models do not prove physical PCI restoration |
| Kernel native mutation ownership | Two real Ze7.3 tests completed useful original operations, renamed the original PPP device, and verified live original descriptors and foreign-namespace controls. The MTU call changed the old-name replacement from 1600 to 1300; address withdrawal removed its matching address. Both calls reported success. These are concrete handle tests; the fixture does not bind a ProductionDriver release seal | Protect configuration and teardown operations with the original native authority, alongside counters. Keep replacement devices unchanged, preserve uncertain original effects and their cleanup custody, and prove the complete admitted-driver path. Source review found the later backend operations still resolve the old name; neither an initial alias stamp nor counter-only protection fixes them |
| Guarded PPP writer and IPv6 integration | The actual provider and reviewed ACK/history repairs are integrated. Regression bodies reproduced malformed ACK acceptance and terminal-history cleanup retention before repair. In `job-ppp-native-writer-review-fixes-green-46c11acb.log`, the models and three useful native provider roots pass; the fourth fails because its Dummy replacement returns `EOPNOTSUPP` before PPP creator validation. A genuine same-index PPP fixture is authored with strict assertions retained and awaits independent judgment and execution. Corrected IPv6 setup compiled in `ae019ca1`; actual original traffic in `7436a54b` now reaches address readiness and finds the source still tentative | Join actual unit-file retirement, wire session consumers, and complete provider validation/publication. Wait for actual IPv6 source and multicast-route readiness before judging packet ownership; original traffic has not passed. Constructor review also found a missing foreign-current-namespace refusal in private ioctl43; repair and native negative proof are required. Kernel socket/fragment enforcement and matching-kernel native execution remain owed |
| Prepared access session ownership | The original dispatch regression failed with owner0/shared1, then passed with race detection after opening through the captured Access. Two independent source reviews found no new defect. The six-package race run recorded 408 top-level passes and no failures; its one macOS platform skip then passed in Ze7.3. All eight new admission/cleanup contract roots ran. Four further real Ze7.3 namespace tests passed with no skips: admission after active-kind replacement, sibling survival, retained failed native release and final parent socket closure. The guest stopped with all exits zero. Five mapped ownership pages are updated | Finish final test judgments, scoped lint and dependency publication. Prove creation-bound child counters and the complete parent Subsystem Stop/Reload retention path. The four native handle tests do not establish complete subscriber acceptance. Native PPP creation binding and mutation targeting remain unfinished |
| Shared legacy CoS | The original-target release/report repairs are integrated. All 94 CoS package roots passed with race detection, including the nine focused original recovery questions. Reached iface, iface/netlink and authradius package race runs also passed | Native VLAN and original-target survivor proofs, functional stress, discrimination, RFC proof records, scoped lint and final publication remain owed. Portable package passes do not establish physical kernel/VPP recovery or full subscriber acceptance |
| CoS reports and compensation | The earlier nine-question baseline recorded three PASS and six FAIL: five missing-report failures and unsafe compensation preparation. The integrated successor now passes the complete 94-root CoS race package, including report-only retries, original readback, compensation and registered restart | Keep the earlier semantic failures as baseline evidence. Native recovery and causal cuts must still prove the current guards; the complete race pass does not supply that evidence |
| CoS cleanup command | The earlier seven-question baseline and real-daemon draft exposed an absent command/schema and lost failed-Down custody. The command and recovery implementation are now integrated. Both registered YANG cleanup tests pass with race detection after the operational containers explicitly declare state data | Rerun the actual daemon draft, then complete functional stress and promoted proof. The earlier independent-progress failure on lost custody is not discrimination of cleanup counts |
| Native authority and coherent integration | Identity, counters, terminal cleanup and accounting integration remain in progress over the landed shared PPP foundation. Private source reviews accepted bounded actor and counter refinements, but no complete native union has runtime acceptance. The direct Birth/ID migration preserves the current authenticated native calls, token, peer and rollback behavior. A separate strict SDK/grant receiver is conditional on its later integration, not an AC-2 prerequisite | Finish the actual PPPoE rate-lifetime, counter, terminal and accounting bindings and their independent test judgments. Then prove original-owner capture, survivor ownership, compensation, physical retirement, fresh admission and diagnostics through real entry points. Source clearance of a private input does not establish complete producer or subscriber acceptance |
| Kernel PPP counters | `66ffd4c583` preserves all eight supplied native totals in both kernel adapters, including measured zero; all 12 focused host race tests passed. The real Ze7.3 namespace and creation-echo primitives passed. `dc46dde6a7` adds the shared creation-bound reader: its initial 27 new tests and ten existing controls passed. Review exposed two typed-nil payload panics; both failed before repair and passed afterward. All 29 affected creation/reader roots then passed, including useful original traffic, rename, caller namespace changes, deletion, movement and permanent history-loss refusal. Three payload-bound cuts reached their intended failing assertions. Four additional refusal roots and six affected reader controls passed; measured counts confirm all seven additional rejection outcomes ran. The redundant helper parameter is removed; scoped lint now reports only three findings in unchanged monitor replay tests | Carry the creation receipt through the original worker slot and session constructor, retaining a genuinely owned backend lifetime. The backend currently has test callers only. Four original PPP session tests still expose wrong-device reads through the legacy name-based provider; diagnostic conversion and backend-only passes do not fix those session bindings. Complete native PPP/session and subscriber acceptance remain open |
| CoS command and diagnostics successor | The initial eight-report baseline had seven semantic failures and one SDK readiness setup failure. After the harness repair, the apply-compensation question failed with zero active warnings. The integrated counts, retry wording, profile explanations and apply/rollback warnings now pass the complete 94-root CoS race package | The baseline establishes the original defects. Native publication, causal failures and restored passes remain owed for the current successor |
| Warning custody after process loss | The generic `ResourceWarning` foundation now protects reserved subjects from ordinary source cleanup and false ordinary RPC acknowledgements. Its runtime tests passed. The native report and receiver proposals remain private; independent review found that repeating completed old recovery could clear a replacement episode or block its new baseline | Fix daemon recovery idempotence and exact episode binding, then integrate the native producers. A confirmed live compensation must clear its own resolved failure episode while preserving the resource reservation. No CoS-name exception or change to ordinary process-warning cleanup is accepted |
| Provider shutdown | `bc532d7502` published ordered shutdown, terminal startup handling, retained diagnostics and the normal-hub fixture. Both original subcases of `internal/component/plugin/server/shutdown_dependency_cleanup_test.go` `TestShutdownKeepsDependenciesLiveThroughEngineCleanup` failed semantically, then passed. The remaining startup and retirement cuts and six shutdown guards produced their intended failures, followed by restored passes. The hub/SIGTERM path passed 80 native repetitions, its promoted live run and all eight VPP functional tests. Startup and server/BGP checks passed 14 and ten focused roots; 13 later server controls passed with race detection. Process and manager race packages passed. Final scoped server lint reported zero issues on the host and Linux; the two lifecycle pages passed their prose check | The process/manager run ended with a dependency vet failure during concurrent command edits; later server runs passed with that supplier present. Its package passes are not an all-pass verdict for that earlier job. The hub results use the VPP stub and do not establish physical VPP or subscriber acceptance. Independent command, BGP readiness and storage edits remain outside this shutdown commit |
| Outcome adapters and visible warnings | Seven of nine reached adapter packages passed, including the complete server package. Web had five failing roots and the fixture package had one observer failure. Later repairs for SSH `first 1` late outcomes and MCP/web absent-document warnings passed 35 focused race roots. The web warning repair passed 12 unit roots, 27 verdict/privacy combinations and both previously failing real-daemon warning drafts. CLI and MCP warning repairs passed 13 focused race roots, including their native-oracle controls. The complete command package passed, including decoded path, quoted argument and declared-shape regressions. The SDK metadata guard passed after its deliberate break produced two semantic failures. Ten further compiler-only breaks produced intended failures; all 12 targeted roots passed after restoration. The additional CLI output-failure question passed real broken-pipe and healthy-output cases. Restoring the old diagnostic order failed its final-fix and output-evidence assertions; the complete canonical client race package then passed 57 roots. Native protobuf generation completed; inspected changes contain comments, tool versions and import whitespace | Independent source review cleared the selected adapter dependency closure, published in `5308422017`. The unit inverse orchestration ended with exit 1 after its passes because six unrelated server test files changed concurrently. The tested producer and relevant test pins stayed unchanged. Broad adapter lint stopped before analysis after shared IPC inputs changed; the unchanged server scope reported zero issues on the host and Linux. No complete adapter-lint verdict is claimed. Current scoped passes do not clear the earlier reached-package or full plugin failures. The existing session state retains the exact commands, source pins and receipts |
| Selected DUT variants | `b7748c4aa6` landed the absent-provider, enabled-required and LCP-required parse cases. Each passed 80 stress invocations and a promoted live run on its matching DUT, with zero skips. Opposite compositions produced the expected applicability skips; the full DUT passed the existing no-access control | These are Darwin CLI and build-metadata results. The provider remains part of the uncommitted feature implementation. Separate orphan, interface-flag and Linux sysctl cases, broader metadata cuts and native dataplane acceptance remain open |

### Remaining work, in dependency order

| Order | Work and required result |
|-------|--------------------------|
| 1 | Complete the original native-authority and CoS ownership integration. Capture the original prepared access and native target before Up or first CoA. Retain one original baseline for shared survivors and settle uncertain operations before compensation. Repair the new kernel retry/transfer path so a current backend or reused device cannot replace the original target. Preserve the direct Birth/ID migration's current native, token, peer and rollback behavior. Do not make the separate strict SDK/grant receiver an AC-2 prerequisite. Full native ownership, physical retirement and fresh recipient admission remain required for the complete feature |
| 1a | Complete the original retained-table and captured-creator recovery caller, authenticated event receipt bridge and genuine released/forget handshake. Deduplicate physical-owner counts and carry every independent failure cause without exceeding a transport frame. A never-admitted participant needs its genuine joined unused-grant proof and independent rate End before writer release. Finalize compensation custody only after the entire initial admission or multi-owner CoA commits; an individual owner's commit cannot discard rollback prematurely. Prove two subscribers can use the same configured profile without an old committed beforeimage blocking the second |
| 2 | Retain the landed runtime-ready, outcome and resource-warning foundations and AC-32 evidence. Finish CoS command/diagnostic recovery and AC-31 against the preserved semantic failures. Prove failed-pass counts, report-only retry, publication failure and same-resource, same-episode compensation clearance. Retain the 22 outcome scenarios' baseline, 80-run stress, promoted live passes and causal proof, the completed protobuf generation, both description-only test corrections and the CLI sink-failure proof published in `5308422017`. Resolve the reached checks and obtain a valid scoped lint verdict. Require semantic causal failures and restored passes for original-baseline, survivor, settlement, transfer, diagnostics and recovery guards |
| 3 | Retain the published dependency-ordered shutdown and full-stop contract without `OnBye`, including the completed guard failures, restored passes and scoped checks. Prove physical VPP cleanup completes before its provider stops. The native stub passes do not discharge that proof |
| 4 | Retain the matching-DUT refusal and enabled/LCP prerequisite passes and their 80-run stress results. Finish the remaining parser cases, metadata causal proofs and native backend acceptance for AC-1, AC-2, AC-29 and AC-30. Preserve declaration failures, exact metadata refusal, valid mismatch skips and the existing Linux default-backend proof |
| 5 | Accept Phase 2 kernel parity and the complete Phase 3 IPv4, Phase 4 IPv6 and Phase 5 policer/CoA paths. Use the actual Ze entry points, native forwarding/counter and TAP captures, VRF/framed/delegated routes, teardown and concurrent policer ordering. Finish the named functional and `pppoe-vpp-ze-ac-pppd-client` interop proofs, including their rebuilt causal failures; draft code and unit-only passes do not discharge these paths |
| 6 | Accept Phase 6 VPP loss/restart through `pppoe-vpp-restart`, then Phase 7 guides, digests, feature declarations, RFC performer annotations and discrimination records. Complete scoped checks and both independent review gates, demonstrate all 37 acceptance criteria and every goal, then close. Phase 1 remains owned by `spec-pppoe-refuse-vpp-backend` |

## Required Reading

### Architecture Docs
- [ ] `ai/digests/subscriber.md` - PPPoE discovery to PPP handoff, shared PPP driver, subscriber registry
  → Constraint: PADS is sent only after the session data plane exists (`handlePADR`); the VPP backend keeps that order
  → Constraint: one reader per channel (`pppSession.run` starts exactly one `readFrames`)
- [ ] `ai/digests/vpp-dataplane.md` - VPP supervision, connector, readiness events, iface-vpp and fib-vpp
  → Constraint: readiness is only the `vpp/connected` and `vpp/reconnected` EventBus events (`internal/core/vpp/events/events.go`); `vpp/disconnected` exists
  → Constraint: `startup.conf` is written once; a plugin load or section change needs a VPP restart
- [ ] `docs/guide/vpp.md` - operator view, LCP TAP namespace limitation
  → Constraint: LCP TAPs land in `vpp.lcp.netns` (default `dataplane`); Ze's discovery socket must open in the namespace the cp TAP lives in
- [ ] `docs/architecture/core-design.md` sections 14, 14a, 14b, 19 - backend split, `ze:backend` commit gate, allowed imports
  → Decision: a backend that cannot implement config exactly refuses it at verify
- [ ] `docs/architecture/traffic/fw-7-traffic-vpp.md` - "Exact or reject"
  → Constraint: a VPP policer drops; a traffic control TBF/HTB shapes. Mapping one to the other silently is refused
- [ ] `plan/post-release/engineering/spec-dataplane-seams-4-control-packet-rx.md` - shared control-packet receive path (decision pending)
  → Constraint: this spec does not build a shared receive path; it adds one PPPoE-local receive path over the cp TAP
- [ ] `plan/post-release/engineering/spec-dataplane-seams-3-route-egress-interface.md`
  → Constraint: `BestChangeEntry` carries no egress interface, so a route "to session interface" cannot travel through sysrib to fib-vpp today
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - PPPoE AC design the changed files declare
  → Constraint: its kernel-session sections gain the backend split and the VPP path in Phase 7
- [ ] `docs/research/l2tpv2-ze-integration.md` - PPP driver and transport boundary the PPP files declare
  → Constraint: the transport/PPP contract it describes (fds in `StartSession`) is replaced in Phase 2, and the page is edited in that phase
- [ ] `docs/research/vpp-deployment-reference.md` - startup.conf reference the VPP component declares
  → Constraint: the `plugins` section and any `pppoe` startup section are documented there
- [ ] `docs/labs/pppoe-interop.md` - PPPoE Docker lab, pppd + rp-pppoe client image
- [ ] `third_party/vpp-linux-cp/README.md` - LCP pair semantics, v25.10

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2516.md` - PPPoE; the session-stage obligations that move into VPP (table below)
- [ ] `rfc/short/rfc1661.md` - LCP stays in Ze; MRU vs VPP session MTU
- [ ] `rfc/short/rfc1332.md`, `rfc/short/rfc5072.md` - IPCP and IPv6CP stay in Ze
- [ ] `rfc/short/rfc4861.md`, `rfc/short/rfc4862.md` - RA/RS for subscribers
- [ ] `rfc/short/rfc4638.md` - PPP-Max-Payload; the VPP session MTU must follow it (`plan/release/fixes/spec-pppoe-rfc4638-max-payload.md`)
- [ ] `rfc/short/rfc2866.md`, `rfc/short/rfc5176.md` - accounting octets and CoA rate change must read VPP counters and re-program VPP policers

#### RFC 2516 obligations whose performer changes

| Id | Requirement (short) | Kernel backend performer | VPP backend performer | Test owed (asserts what Ze installs) |
|----|---------------------|--------------------------|----------------------|--------------------------------------|
| RFC2516-3-1 | AC allocates a PPP virtual interface once the session is established | `ppp.DevPPPSetup` creates `pppN` | `pppoe_add_del_session` creates `pppoe_sessionN` | `pppoe_session_dump` after PADS shows the session; absent before PADI |
| RFC2516-6-1 | Session-stage CODE is 0x00 | Linux `pppoe` module encap | VPP `pppoe` encap (`pppoe.c`, `pppoe_fixup`) | Captured downstream data frame from VPP carries CODE 0 |
| RFC2516-7-1 | SESSION_ID does not change and is the discovery value | AF_PPPOX connect address | `session_id` in `pppoe_add_del_session` | Session dump SID equals the PADS SID |
| RFC2516-x-6 | Session traffic DESTINATION_ADDR is the peer's unicast MAC | AF_PPPOX remote | `client_mac` in the session | Captured frame destination equals PADR source |
| RFC2516-x-7 | SOURCE_ADDR is the sender's MAC | kernel device MAC | encap interface hardware address (`link_table`) | Captured frame source equals the access interface MAC |
| RFC2516-x-1, x-2 | VER and TYPE are 1 | kernel encap | VPP encap | Same capture |
| RFC2516-5.5-4 | No PPP frames after PADT | Ze closes the PPPoX fd | Ze deletes the VPP session before sending PADT; control path stops in Ze | After PADT, VPP session absent and no session frame leaves for that SID |
| RFC2516-7-3 | Stop using the session when LCP terminates | Ze closes fds | Ze deletes the VPP session | Same as above, LCP-initiated |
| RFC2516-x-9 | MRU not negotiated above 1492 | Ze LCP | Ze LCP; VPP session MTU set to the negotiated MRU | Session interface MTU readback |

Each row keeps its `lower-layer:` annotation and names both performers, one per backend (`ai/rules/rfc-compliance.md`, whole-stack directive).

**Key insights:**
- ng does not bypass the kernel for control: control frames travel VPP -> LCP TAP -> Linux -> Ze. Only data never touches Linux.
- The upstream VPP pppoe plugin is usable for one access interface and dual-stack sessions; multi-interface, IPv6-only, upload policing and IPv6 control need either the VyOS patches or the upstream workarounds named above.

## Current Behavior (MANDATORY)

The source inventory below records the pre-implementation design baseline.
The shared `StartSession` seam now carries `Frames` and `Dataplane`, and the
current provider locations are recorded after Files to Create. The original
paths remain here as the research record; acceptance of the complete backend
remains open.

**Source files read:**
- [ ] `internal/component/l2tp/pppoe/server.go` - `handlePADR`: cookie check, SID alloc, `pppoeCreate` then `ppp.DevPPPSetup`, then PADS, then `ppp.StartSession` on `SessionsIn()`
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `Start`: one raw discovery socket for all interfaces, servers keyed by kernel ifindex; PPP driver built with `iface.GetBackend()` as its `ppp.IfaceBackend`; `onSessionUp` calls `subscriber.GetShaperHandler()` with `sess.PppInterface`
- [ ] `internal/component/l2tp/pppoe/kernel_linux.go` - discovery socket, `pppoeCreate` (AF_PPPOX)
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - `pppoe` container: globals, `list interface` keyed by kernel interface name; no `ze:backend`
- [ ] `internal/component/l2tp/ppp/start_session.go` - `StartSession` carries `ChanFD`, `UnitFD`, `UnitNum` (iface name is `"ppp"+UnitNum`)
- [ ] `internal/component/l2tp/ppp/session.go` - `IfaceBackend`: `SetMTU`, `SetAdminUp`, `AddAddressP2P`, `AddRoute`, `RemoveAddress`, `RemoveRoute`
- [ ] `internal/component/l2tp/ppp/session_run.go` - `PPPIOCCONNECT` of chan to unit, `pppN` MRU/MTU/admin-up
- [ ] `internal/component/l2tp/ppp/devppp_linux.go` - `DevPPPSetup` ioctl sequence
- [ ] `internal/component/l2tp/ppp/ra_linux.go`, `dhcpv6_linux.go` - RA and DHCPv6 sockets bound with `SO_BINDTODEVICE` to `pppN`
- [ ] `internal/component/l2tp/subscriber/handler_registry.go` - `ShaperHandler(iface, down, up)` singleton
- [ ] `internal/component/l2tp/plugins/shaper/shaper.go`, `config.go` - shaping through `traffic.GetBackend()`, qdisc TBF or HTB only
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - `AddAddressP2P` and `AddRoute` answer `errNotSupported`
- [ ] `internal/plugins/iface/vpp/lcp.go` - `SetupLCPPair`, `RemoveLCPPair`
- [ ] `internal/component/vpp/startupconf.go` - `plugin default disable`; dpdk, urpf, linux_cp, linux_nl, wireguard enabled explicitly
- [ ] `internal/core/vpp/events/events.go` - `connected`, `disconnected`, `reconnected`
- [ ] `internal/le/test/deployment/vppiface.go` - real-VPP harness image `ligato/vpp-base:26.06-release`

**Behavior to preserve:**
- The kernel backend's behaviour, wire output, `show pppoe` answers and every existing PPPoE `.ci` and interop scenario, unchanged.
- L2TP keeps the kernel data plane; it consumes the same PPP package and must not regress.
- PADS only after the session data plane exists.

**Behavior to change:**
- PPPoE under `interface backend vpp` validates and then fails every session at IPCP Opened (journal row below). It becomes either a working VPP session data plane or a verify-time refusal.
- `ppp.StartSession` stops carrying kernel file descriptors as its data-plane contract (replaced, not layered: see Key Design Decisions).

## Existing Mechanisms

| Mechanism | Location | Current role | Use in this spec |
|-----------|----------|--------------|------------------|
| Registered interface and traffic backends | `internal/component/iface/backend.go`, `internal/component/traffic/backend.go` | Select one configured backend and reject unsupported semantics | Extend the registration pattern with the PPPoE session seam |
| Subscriber handler registries | `internal/component/l2tp/subscriber/handler_registry.go` | Share authentication, pool and rate handling across PPPoE and L2TP | Preserve these entry points while replacing the session data-plane contract |
| VPP connector and readiness events | `internal/component/vpp/`, `internal/core/vpp/events/` | Supervise VPP and notify consumers of connection changes | Use for session programming and loss handling |
| Shared PPP driver | `internal/component/l2tp/ppp/` | Own PPP negotiation for both access transports | Replace its kernel descriptor contract with one frame channel and session handle |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- PPPoE discovery and session frames from a subscriber arrive on a VPP-owned access interface (DPDK or af_packet).
- Config: `interface { backend vpp; }`, `vpp { lcp ...; plugins ... }`, `pppoe { interface <name> ...; }`.

### Transformation Path
1. VPP `pppoe-input` (registered for both PPPoE ethertypes) sends discovery and non-IP session frames to `pppoe-cp-dispatch`, which learns the client MAC and transmits on the cp interface (the access interface's LCP TAP).
2. Ze's PPPoE subsystem reads discovery and session control frames on the Linux side of that TAP, in the TAP's namespace.
3. On PADR, the VPP session backend creates the VPP session interface (`pppoe_add_del_session`), then Ze sends PADS and starts the PPP session with a frame channel that is not `/dev/ppp`.
4. LCP, auth and NCP run in Ze exactly as today. On NCP Opened, the session's data-plane handle installs the peer route(s) to the session interface in the subscriber's VRF, enables IP on it, sets MTU, and applies the rate.
5. Subscriber IP traffic is decapsulated by VPP into the session interface and routed by the VPP FIB; downstream traffic routed to the session interface is encapsulated with the session ID and client MAC.
6. Teardown (PADT either way, LCP terminate, CoA Disconnect, operator clear) removes routes, policers and the VPP session before PADT is sent.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VPP -> Linux (control frames) | LCP TAP, cp map | Phase 0 mechanism passed on 2026-11-16; complete product backend acceptance remains owed |
| Ze -> VPP (session, routes, policers) | GoVPP binary API through `vppcomp.GetActiveConnector()` | No |
| VPP lifecycle -> PPPoE | EventBus `vpp/connected`, `vpp/disconnected`, `vpp/reconnected` | Events exist; consumer No |
| PPPoE -> subscriber plugins | unchanged: `subscriber` handler registries and `subscriber` event namespace | Yes (digest) |

### Integration Points
- `pppoe.Subsystem` selects the registered session data plane by the configured interface backend name.
- `ppp.Driver` receives a per-session data-plane handle through `StartSession` instead of fds.
- `iface` VPP backend creates the LCP pair for each PPPoE access interface (existing `SetupLCPPair`).
- `vpp` component loads `pppoe_plugin.so` and, if patched, writes the `pppoe` startup section.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | Control plane stays in Ze; only the per-session data-plane calls change |
| No unintended coupling | Yes (design) | VPP backend lives in its own `ze_vpp`+`ze_l2tp` gated package and registers; `pppoe` never imports `vpp` |
| No duplicated functionality | Yes (design) | Reuses PPP driver, discovery codec, subscriber registries, GoVPP connector |
| Zero-copy preserved | Yes (design) | Control frames only; data never reaches Ze |
| Registration over hardcoding, outbound | Yes (design) | Session data planes register by name from `init()` |
| Registration over hardcoding, inbound | No, to verify in Phase 1 | Lists to search: `startupconf.go` plugin section (wireguard is a toggle there today), `feature-gates.txt` dependent-gate notes, doctor codes |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A Linux AF_PPPOX socket connected on the cp TAP but never attached to a PPP channel delivers and sends PPP frames as a datagram socket | ng `establish_ppp` non-dev-ppp path; Linux `pppoe_rcv_core` queues to the socket when not bound | Use an AF_PACKET 0x8864 socket on the TAP with Ze-side demux by (MAC, SID) | Complete native plain, namespace and VLAN mechanism runs passed reciprocal LCP/IPCP and control-socket assertions, 2026-11-16 | confirmed |
| A-2 | The source-built Phase 0 artifact contains and loads `pppoe_plugin.so` | `third_party/vpp-patches/Dockerfile` pins VPP 25.10, applies the retained patches and includes `pppoe` in the CMake plugin selection (OD-1 B) | Repair the source-build or startup recipe before taking mechanism evidence | Complete proof required running `show plugins`, 31 parser cases, four receive paths and ten binding cases, and passed on 2026-11-16 | confirmed |
| A-3 | VLAN-tagged access (subinterfaces) reaches the cp TAP with the tag intact | `pppoe_cp_node.c` resets the buffer to the Ethernet header | VLAN access needs an LCP pair per subinterface and per-subinterface cp map | Native capture showed untagged frames at the Linux VLAN socket; explicit child LCP creation and child-to-child mapping made the complete VLAN mechanism pass | broken; stated fallback proven |
| A-4 | VPP's pppoe encap honours the session interface MTU | upstream `pppoe.c` | MRU/MTU enforcement must be a VPP feature or a refusal | Complete proof passed all three native topologies at 1492/1493 and the independent VPP-only 1400/1401 boundary with client MTU above 1400, 2026-11-16 | confirmed |
| A-5 | Discovery and session sockets opened in the LCP netns work while Ze runs in another namespace | `vpp.lcp.netns` default `dataplane`; Ze has netns helpers for other features | Restrict to empty netns and say so in docs | Complete native namespace topology passed socket restoration, reciprocal control and forwarded traffic, 2026-11-16 | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Control-frame path through LCP TAP is slow under PADI storm | PADI rate test drops LCP | Existing PADI limiter; VPP-side policer on cp (copp work, seams-4) |
| R-2 | Patched VPP CRCs diverge from govpp binapi | `CheckCompatiblity` fails at connect | Ze-generated binapi from the same patched `.api` (OD-1 B) |
| R-3 | VPP restart loses every session silently | Sessions in Ze, none in `pppoe_session_dump` | Reconcile on `vpp/reconnected` (OD-2) |
| R-4 | Policer semantics differ from traffic control shaping | Operator configured TBF on VPP | Refuse at verify; separate `policer` rate mode |
| R-5 | Scale: one VPP interface per session | VPP interface table growth, stats segment size | Measure on Thomas's Mac (perf rule); size limits documented |
| R-6 | Native-only compilation selects no CRC ISA while VPP core calls scalar CRC32C helpers | Observed native ARM compiler errors in `session/application.c` before the PPPoE runtime tests | Retained portable helper patch, unchanged native flags, and exact seeded-vector regression before accepting the image |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Kernel-backend PPPoE and L2TP subscribers if the PPP channel seam regresses; VPP subscribers lose service |
| How is it reverted? | Per-phase commits; Phase 2 (PPP channel seam) is the only one touching L2TP |
| Who else touches this path? | `spec-pppoe-rfc4638-max-payload`, `spec-pppoe-subscribers-produce-no-accounting-or-telemetry`, `spec-pppoe-relay-agent`, `spec-pppoe-virtual-interface-proof`, `spec-bsd-l2tp-pppoe` (a third session data plane: netgraph/pipex, must fit the same seam), `spec-dataplane-seams-4-control-packet-rx`, `spec-l2tp-ipv6-subscriber` |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le build vpp run` with the optional proof variant | → | Registered source build and immutable image report | `TestVPPBuildActionRegisters`, `TestVPPProofVariantGrammarPassesStartupChecks` |
| `./le test deployment vpp-pppoe run` or `proof` | → | Isolated mechanism and same-guest parser proof | `TestMechanismActionRegisters`, `TestMechanismProofActionRegisters`; native results are recorded in the Phase 0 test table |
| Config `interface backend vpp` + `pppoe interface X` | → | PPPoE verify selects the `vpp` session data plane or refuses | `test/parse/pppoe-vpp-backend-refused.ci`, `pppoe-vpp-enabled-required.ci`, `pppoe-vpp-lcp-required.ci`, `pppoe-vpp-no-access-accepted.ci` |
| PADR on a VPP access interface | → | VPP provider `Open` creates the session; returning from `Open` permits PADS | `TestVPPFactoryPublishesOnlyCreatedInterface`, `TestVPPFactoryRollbackClosesSocketAndConfirmedSession` (`pppoe/dataplane/vpp/factory_test.go`); server order, `TestPPPoESessionCreatedBeforePADS` and `TestPPPoESessionOpenFailureSendsNoPADS` (`pppoe/vpp_session_order_test.go`, `b6388ee211`); end to end, `./le test deployment vpp-pppoe product` (passed 2026-11-22) |
| IPCP Opened | → | session handle installs the peer /32 route in VPP | `TestVPPSessionPeerRouteUsesCreatedIndexAndTable`, `TestVPPPeerRouteWaitsForPostACKConnect`; the lab `./le test deployment vpp-pppoe product` (scenario `pppoe-vpp-ze-ac-pppd-client`, named in `vpppppoe/product_plan.go`) |
| `vpp/disconnected` then `vpp/reconnected` | → | session reconciliation per OD-2 | Owed, not written: OD-2 is not built. No PPPoE code consumes the `vpp` events, and `Reconcile` (`pppoe/dataplane/vpp/reconcile.go`) has no production caller (Final gates) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `interface backend vpp`, a `pppoe` block, and no VPP session data plane in the build (`ze_vpp` absent) | `ze config validate` refuses, naming the `pppoe` path, the backend and the build tag that provides it |
| AC-2 | Same tree on a build with the VPP session data plane, `vpp lcp` disabled | Validate refuses: the PPPoE access interface needs an LCP pair, with the leaf that enables it |
| AC-3 | pppd client sends PADI on a VPP access interface | PADO comes back with Ze's AC-Name; the PADO source MAC is the access interface MAC |
| AC-4 | Client sends PADR | A VPP session with the PADS SESSION_ID and the client MAC exists before PADS leaves; no `pppN` exists on the host |
| AC-5 | LCP, CHAP, IPCP complete | The peer address is reachable from a host behind VPP; VPP's session interface counters move; the host kernel sees no 0x8864 data frame for that session |
| AC-6 | Session in a configured VRF | Peer route sits in that VRF's VPP table only |
| AC-7 | RADIUS Framed-Route or DHCPv6-PD prefix for the session | Route to the session interface installed in VPP; removed on teardown |
| AC-8 | IPv6CP opens | Client receives an RA with the configured prefix and obtains IA_NA/IA_PD by DHCPv6, all without a kernel socket bound to a `pppN` |
| AC-9 | Rate attributes (RADIUS Filter-Id rate or config) on a VPP session | Download and upload are policed at the rate on the session interface; a TBF/HTB shaper config under the VPP data plane is refused at verify |
| AC-10 | CoA rate change | Policers replaced on the session; no traffic gap beyond one policer swap |
| AC-11 | PADT from client, LCP Terminate, CoA Disconnect, operator clear | VPP session, routes and policers are gone; PADT sent after the VPP session is deleted; pool address released; accounting Stop sent |
| AC-12 | VPP process killed while sessions are up | Ze reports the loss once, frees every session's state per OD-2, and serves new PADI once `vpp/reconnected` fires and the LCP pair is back |
| AC-13 | Kernel backend (default kernel) | Every existing PPPoE and L2TP test and interop scenario passes unchanged |
| AC-14 | `show pppoe session` on VPP | Names the VPP session interface and its sw_if_index; `ze doctor` reports the VPP pppoe plugin state |
| AC-15 | A policed session ends (PADT, terminate, CoA Disconnect, idle) or a CoA swaps its rate, with many such sessions at once under traffic | The `policer-input` features leave the ip4/ip6 unicast and multicast arcs before the policer is unbound and deleted; VPP keeps its PID; no policer remains for a session that is gone. Source: accel-ppp-ng PR #46 (https://github.com/accel-ppp/accel-ppp-ng/pull/46), where the reverse order let `policer_input_node_fn` run on a deleted policer and VPP died with SIGBUS (VyOS T9502) |
| AC-16 | A VPP session ends while others are up (PADT, operator clear, idle or session timeout) | The session's PPPoE socket, which is its PPP frame channel with no `/dev/ppp` unit, is closed exactly once by its one owner, after it is deregistered from any poller; the daemon and every other session keep running. Source: accel-ppp-ng PR #44 (https://github.com/accel-ppp/accel-ppp-ng/pull/44), where the shared fd was closed early and the later `EPOLL_CTL_DEL` on it failed with EBADF and aborted the daemon, dropping every session (VyOS T9499) |
| AC-17 | A VPP session carries a RADIUS Idle-Timeout or Session-Timeout, and sends traffic | Its byte counters and idle detection come from the session's data-plane handle (the VPP session interface's counters, by the sw_if_index known only after the session interface is created); accounting octets move; a counter read that fails is reported and never read as zero or as idle, and never ends the session as a NAS error. Source: accel-ppp-ng PR #42 (https://github.com/accel-ppp/accel-ppp-ng/pull/42), where the ifindex was resolved before the hook created the interface, stats read nothing, and every session ended with NAS-Error every 60 s (VyOS T9211) |
| AC-18 | Truncated Ethernet/VLAN/PPPoE frames, payload declarations beyond the actual buffer chain, and truncated direct IPv6/ICMPv6/UDP headers reach the patched VPP receiver under Ze's manual session management (auto-discovery disabled) | Every receive lane and the control dispatcher reject and count malformed lengths before classification or unsafe tracing; valid empty discovery, VLAN and split-header frames still pass. The retained Ze diff's real-buffer runtime command passes all 31 cases, including explicit accepted-frame metadata assertions, and malformed wire frames produce no control-plane forwarding or session allocation |
| AC-19 | A valid data/control interface pair has different software and hardware index values; callers also request deletion with an invalid interface, an absent binding or a mismatched pair | `pppoe_add_del_cp` resolves the hardware interface from the supplied software index and installs the intended pair. Invalid, absent and mismatched deletions are refused before any map write, preserving every existing binding. Deleting the exact valid pair removes both map entries and leaves other bindings intact; VPP remains responsive throughout |
| AC-20 | The native lab compiler selects the software CRC32C branch | Patched VPP builds without forcing CRC instructions. Scalar helpers preserve the input seed and low-byte-first order; the standalone C executable passes 27 independently specified word vectors and two standard chaining cases, returns nonzero on a mismatch, and emits the exact 29-case success answer required by the recipe |
| AC-21 | A shared PPP or relayed admission ends during authentication, pending bridge setup, a full control queue, remote teardown or local shutdown; an older call's cleanup arrives after numeric identifiers are reused | Every owned reader and resource is released once, both relay endpoints learn termination, and cleanup verifies the exact current owner before changing a replacement. An admitted session survives malformed or duplicate requeue of its owner |
| AC-22 | One NCP is disabled, declined or closed while the other family is usable | VPP enables only the opened family's receive path. A later negotiated address pair can enable its family; withdrawing its last pair disables that family without interrupting the other |
| AC-23 | A route or protocol-enable mutation has an uncertain outcome, or forwarding cleanup fails before VPP would reuse a session's native interface index | The exact mutation and ownership remain retained for reconciliation. The interface is not recycled while old paths or enable references can reach another subscriber; confirmed mutation followed by stream-close failure is still owned |
| AC-24 | The kernel handle adds or withdraws a route, including a failed add, failed removal, invalid or unattempted tuple, and exhaustion of its ownership bound | It records the canonical destination, gateway, metric and protocol intent before native add; only that exact owned intent may be withdrawn. Failed operations retain the intent for retry, absent ownership performs no native deletion, and at most 1024 intents are retained with released slots reusable |
| AC-25 | PPPoE access indices differ above bit 15, or a queued response, lifecycle event or cleanup arrives after a subscriber ID or authentication/address-request round is reused | Full access identities remain distinct. Every response and mutation verifies the exact admitted worker binding; provider answers also identify the exact pending request. An old admission or request cannot acknowledge, authorize, cancel, release or change its replacement. The interface-owned-device cleanup callback remains compatible with the server's process-generation API and preserves its original process-pointer guard |
| AC-26 | A kernel subscriber needs shaping while the global traffic backend is VPP, with the shaper running internally or as an external plugin | The shaper owns an independent kernel backend and durably acknowledges its own original-state snapshots before replacement. Its cleanup restores only its own state; the global backend remains independently owned. Missing or corrupt persistence refuses the destructive operation |
| AC-27 | A managed VPP API socket belongs to a different process than the spawned child, or the statistics socket belongs to a different physical process than the API, including across PID namespaces | Readiness and counter publication are refused. Process retirement requires the exact retained physical process proof; a claimed PID, logical generation or another child's completed wait cannot retire a live foreign peer |
| AC-28 | An operator reload encounters an earlier subsystem failure while a later transport retains uncertain subscriber cleanup | Reload notifies every registered subsystem in registration order, including the later transport's exact retained cleanup owner, and returns all failure causes. An earlier error cannot make the operator recovery path unreachable |
| AC-29 | The interface backend gate aggregates a typed host refusal or ordinary configuration errors | Every original cause remains reachable and the operator text stays unchanged. Ordinary errors remain configuration failures. On a host with no default backend, the offline JSON CLI rejects an omitted interface backend with exit 1, `valid=false` and `config-host-backend`, never `config-plugin-verify`. A cause-only break fails that diagnostic assertion |
| AC-30 | A parse fixture declares `needs-linux`, or `ZE_QEMU_LINUX_ONLY=1` selects parse fixtures | Shared admission validates complete option names, keys and capabilities using the actual file path. Commands, stdin, assertions, existing skip reasons and the exact guest route remain intact. The population filter excludes only valid unmarked files. Malformed files fail rather than skip. The omitted-backend kernel acceptance fixture requires an executed Linux PASS and 80 stress runs with zero skips |
| AC-31 | Failed shared kernel CoS owners remain while other members are healthy, pending or still compensatable; the operator runs `ze request class-of-service cleanup` | The registered no-argument command makes one bounded pass over eligible failed original owners using their captured native authority. It reports attempted and recovered counts, retains partial progress and every independent failure cause, and preserves healthy, pending and compensatable owners. An empty set is a successful no-op; malformed arguments fail before any native write. Uncertain operations settle before compensation and only confirmed same-owner readback clears cleanup custody |
| AC-32 | `internal/test/pluginengine` `Start` runs a real SDK plugin over either wire or bridge, with a held initializer or an initializer that makes an engine RPC | `Start` remains pending until the existing runtime-ready callback receipt confirms initialization. Initializer RPCs are served while waiting; the first command then reaches the initialized handler once with its original arguments. Stage 5 or bridge activation alone cannot release `Start`; no fabricated readiness or command retry substitutes for the receipt |
| AC-33 | The daemon stops with dependent internal engines or external child processes still cleaning up | The selected dependency graph keeps providers available through actual engine completion and external reaping/stderr joins. Whole-daemon shutdown sends no removal-only `OnBye`. Expired cleanup retains and reports unfinished ownership rather than claiming a join or discarding remaining processes |
| AC-34 | Stop races startup readiness, callback completion or a replacement generation; an empty startup callback reenters Stop | Once terminal Stop is observed before a remaining readiness step, that step is skipped. A callback admitted while live may finish after Stop, but cannot turn a terminal outcome into success or clear a successor's stopped record. Final Stop admission and completion publication are serialized; no lifecycle lock is held across callbacks. Parent cancellation is observed before the final completion check. A genuinely completed startup keeps its historical success after a later Stop or cancellation |
| AC-35 | An operator intentionally restarts a running plugin whose unexpected-exit policy is fatal | The old generation loses only its logical restart ownership before it is stopped; its actual cleanup and process join remain retained. The intentional retirement does not stop the daemon. An unexpected fatal exit of the current generation still invokes its configured policy |
| AC-36 | Operational command recognition receives incomplete quotes, pipe syntax, a keyword prefix, or commands predeclared by a subsystem before its full registration | Recognition follows the actual grammar and complete keyword boundaries, never accepts an unfinished quoted command, and sees the original subsystem declaration. Full registration replaces only that subsystem's declaration without stale or duplicate catalogue entries |
| AC-37 | Usage renders command arguments after the command registry publishes its current route-kind values | It renders the declared values from the actual registration. Existing command and route-kind behavior remains available; fixture changes require independent judgment against that contract rather than dropping or weakening the usage question |
| AC-38 | A guarded PPP write has an uncertain outcome, or its retained socket's native Close fails, including during construction | Pending writes retain their exact socket and causes without resubmission. Only successful passive retirement of the same original unit, borrows and tracked duplicate may end uncertain transport custody; the unknown outcome remains reported. A failed socket Close retains its writer, backend resources and original cause without retrying a logically retired file or numeric descriptor. Bind, name-query and creator-probe failures retain a refused partial writer with zero mutation features when Close fails. The independent counter reader survives that failure. Successful physical Close permits final backend cleanup despite historical write errors; unrelated devices remain unchanged because no replacement authority is acquired |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Subscriber dials PPPoE through a VPP access port and browses | client -> VPP pppoe-cp -> TAP -> Ze discovery/PPP -> VPP session + route -> VPP FIB | interop `pppoe-vpp-ze-ac-pppd-client` |
| 2 | Operator restarts VPP | vpp component -> `vpp/disconnected` -> PPPoE backend reconcile -> `vpp/reconnected` | Owed, not written: a restart scenario, after OD-2 loss handling is built |
| 3 | RADIUS lowers a subscriber's rate by CoA | authradius CoA -> `SessionRateChange` -> VPP policer replace | Owed, not written: a CoA rate test, after the policer is wired |

## 🧪 TDD Test Plan

### Phase 0 Lab Tooling Tests

The main thread observed all 12 build tests and all 31 mechanism tests plus
discovery fuzz seeds pass in `job-unit-pkg-50e95a94.log`, after the final
mechanical repairs. Scoped host and Linux lint reported zero issues in
`job-pppoe-mechanism-lint-5e4d4c3e.log`. The owning session state records those
runs. These tests cover the lab tooling, not A-1 through A-5 or the
product session backend. The root registry package also passed in
`job-unit-pkg-136c4de5.log`.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPPBuildActionRegisters` | `internal/le/build/vpp/build_test.go` | Native build entry point and unknown-action refusal | Passed |
| `TestVPPBuildUsesRegisteredImage` | same | Registered recipe, narrow context, immutable image identity and retention release | Passed |
| `TestVPPBuildFailureIsReported` | same | Build failure produces no artifact or unowned cleanup | Passed |
| `TestVPPBuildRetentionFailureIsReported` | same | Retention-release failure reaches the caller | Passed |
| `TestVPPBuildCancellationReachesBuilder` | same | Caller cancellation reaches Docker | Passed |
| `TestVPPBuildRetentionCleanupSurvivesCancellation` | same | Export-race cleanup survives caller cancellation | Passed |
| `TestVPPBuildRunHandlesSignals` | `internal/le/build/vpp/signal_test.go` | Real registered action handles SIGINT/SIGTERM and releases its retained image | Passed; signal-root counterfactual failed before the live source passed |
| `TestVPPProofVariantSelectsSeparateImage` | `internal/le/build/vpp/variant_test.go` | Missing-metadata image uses a separate tag and reports its variant | Passed |
| `TestVPPProofVariantPreservesBaselineRecipe` | same | Both variants share source and recipe; baseline is unchanged | Passed |
| `TestVPPProofVariantRefusesUnknownName` | same | Undeclared proof variants are refused | Passed |
| `TestVPPProofVariantOmissionSelectsBaseline` | same | Omitted or explicit baseline selects the normal image | Passed |
| `TestVPPProofVariantGrammarPassesStartupChecks` | `internal/le/build/vpp/startup_test.go` | Production `internal/le` composition passes the real registry startup checks | Passed; unchanged assertion moved to the complete production fixture after independent judgment |
| `TestMechanismActionRegisters` | `internal/le/test/deployment/vpppppoe/plan_test.go` | Native mechanism entry point and unknown-action refusal | Passed |
| `TestMechanismFindsNegotiatedClientInterface` | same | Checker uses the assigned PPP interface and refuses absent or ambiguous state | Passed |
| `TestMechanismRejectsEthernetReturnRoute` | same | Reply route must use the negotiated PPP interface | Passed |
| `TestMechanismRequiresCounterIncreaseInBothDirections` | same | Controlled traffic must advance both actual VPP counters | Passed |
| `TestMechanismPlanRetainsFailureEvidence` | same | Bounded peer logs are declared before cleanup | Passed |
| `TestMechanismControlStatePreservesProbeFailures` | same | Transport and permission errors cannot become healthy state | Passed |
| `TestMechanismControlStateAcceptsMissingMarker` | same | Only a completed missing-file answer permits the state read | Passed |
| `TestMechanismDiscoveryChecksDeclaredLength` | same | Discovery parser bounds the declared payload | Passed |
| `TestMechanismPlanHasIndependentTrafficPeer` | same | Traffic sender is separate from the VPP and subscriber peers | Passed |
| `TestMechanismPlanMountsOnlyOwnedInputs` | same | Peers receive only task-owned helper and evidence mounts | Passed |
| `TestMechanismLCPAckEchoesRequest` | same | Bounded LCP Configure-Ack echoes the received request | Passed |
| `TestMechanismLCPAckRejectsTruncatedRequest` | same | Truncated control packets produce no acknowledgment | Passed |
| `TestMechanismReadsBothVPPPacketCounters` | same | Both explicit packet counter rows are required | Passed |
| `TestMechanismResolvesControlPair` | same | LCP readback identifies the intended control interface | Passed |
| `TestMechanismImageRequiresCompleteDigest` | same | Counterfactual input requires a complete immutable image ID | Passed |
| `TestMechanismProofActionRegisters` | `internal/le/test/deployment/vpppppoe/proof_test.go` | Combined proof has its own native action | Passed |
| `TestMechanismMetadataProofRequiresItsOracle` | same | Only the complete exact metadata refusal on successful CLI transport with empty stderr proves the mutation | Passed; ten attribution cases. Added mixed assertion, embedded assertion and stderr negatives first failed against substring matching, then passed after the checker repair |
| `TestMechanismBindingsActionRegisters` | `internal/le/test/deployment/vpppppoe/binding_proof_test.go` | Binding regression has its own admitted native action | Passed |
| `TestMechanismBindingProofRequiresCompletedExactSuccess` | same | Only the completed exact ten-case success marker proves the binding cases | Passed; eleven attribution cases |
| `TestMechanismBindingPlanUsesBaseline` | same | The isolated binding peer reads back the normal baseline variant | Passed |
| `TestMechanismProofOrdersBindingStageAndPreservesIdentity` | same | Full baseline precedes same-image bindings and metadata; failures stop later stages | Passed |
| `TestMechanismProofRetainsBaselineAcrossStages` | `internal/le/test/deployment/vpppppoe/lease_proof_test.go` | A shared-tag replacement cannot remove the baseline before dependent peers finish; cancellation still releases once | Passed after observed behavioral failure |
| `TestMechanismProofReportsBaselineLeaseFailures` | same | Build, identity and release failures remain failures with their evidence | Passed after observed behavioral failure |
| `TestMechanismProofPreflightsBeforeBaselineBuild` | `internal/le/test/deployment/vpppppoe/preflight_proof_test.go` | Docker, staged-helper and kernel refusals stop before source image builds | Passed after observed behavioral failure |
| `TestMechanismImageReleasePreservesPrimaryFailure` | same | Client image release preserves primary setup and earlier cleanup failures under cancellation | Passed after observed behavioral failure |
| `TestMechanismMTUProbeRequiresIndependentClientMTU` | `internal/le/test/deployment/vpppppoe/mtu_test.go` | Actual client interface MTU must independently exceed 1400 | Passed after observed behavioral failure |
| `TestMechanismMTUProbeControlsVPPBoundary` | same | Lower only VPP to 1400, require 1400 success and 1401 refusal, restore 1492 | Passed after observed behavioral failure |
| `TestMechanismMTUProbeRequiresExactVPPReadback` | same | Read exactly one session row and its IPv4 MTU element | Passed after observed behavioral failure |
| `TestMechanismMTUProbeRequiresTypedPingRefusal` | same | Transport failure, unrelated packet loss and a different MTU cannot prove enforcement | Passed after observed behavioral failure |
| `TestMechanismMTUProbeRestoresAfterPositiveFailure` | same | Positive-control failure still restores VPP and preserves both error causes | Passed after observed behavioral failure |
| `TestMechanismMTUProbeRestoresAfterCancellation` | same | Cancellation after an attempted change still causes bounded restoration and readback | Passed after observed behavioral failure |
| `FuzzMechanismDiscovery` | `internal/le/test/deployment/vpppppoe/plan_test.go` | Bounded discovery parsing over malformed inputs | Seed cases passed; no sustained fuzz run claimed |
| `test pppoe frame-bounds` | `third_party/vpp-patches/0099-pppoe-bound-frame-parsing.patch` (optional runtime C test) | AC-18: real buffers cover truncated declarations, poisoned padding, discovery, VLAN, split chains and IPv6/ICMPv6/UDP bounds; accepted metadata is explicit | Complete Ze-kernel proof passed on 2026-11-16: all31 cases and malformed-wire checks in plain/namespace/VLAN topologies on baseline `sha256:a0aac6ad351b37311384caa5fab112630d3a733875777faffc5689a61a6085bc`; missing-metadata image `sha256:92da1e02c179f5731d3a6fbe9010333f50fe17bafc419252b589ab298f1d04d0` produced only the exact expected metadata assertion |
| `test pppoe cp-binding` | `third_party/vpp-patches/ze/0100-pppoe-cp-binding-tests.patch` (optional runtime C test) | AC-19: software/hardware index skew, absent-map refusal with `VNET_API_ERROR_NO_SUCH_ENTRY`, mismatched-delete preservation and valid create/delete | Native runtime RED: `unmapped deletion was accepted`, VPP running without OOM, before separate 0101 repair; unchanged ten-case regression passed afterwards in the same guest, baseline image `sha256:fcfbf59eaa472d77898e1b1f60a8ca965f4f03d7e7f3ba04099b729712f24d2f` |
| `test pppoe receive-bounds` | `third_party/vpp-patches/ze/0102-pppoe-receiver-bounds-tests.patch` (optional runtime C test) | AC-18: actual scalar, paired-left, paired-right and control-dispatch graph paths, drops, counters, safe traces and unchanged session allocation | Complete Ze-kernel proof required and passed the exact four-path answer before dialing in all three baseline topologies, 2026-11-16 |
| `test pppoe ownership` | `third_party/vpp-patches/ze/0103-pppoe-owned-reconciliation.patch` (native C test) | Exact session, feature, policer and route ownership; wrong MAC/SID absence; all feature selectors; native policer configuration; independent shared-prefix paths and route withdrawal | Ze-kernel runtime first failed the exact IPv4 withdrawal assertion; retaining the original requested path before native fixup repaired it. The unchanged assertion and strengthened 55-group suite passed on 2026-11-16, guest image `sha256:a017bfa233b228b5e486a2ec74fc7a2c5d52c7cf078af66dead01e36139d4d06`. Independent countercut runs remain owed |
| `test_crc32c_scalar` | `third_party/vpp-patches/0098-crc32-portable-scalar-helpers.patch` (build-time C executable) | AC-20: every scalar width, nonzero seeds, low-byte order and standard CRC32C chaining, with a strict process/output oracle | Native ARM compiler RED preceded the repair; source build completed all 1045 targets and the actual executable printed `VPP CRC32C scalar helpers: 29 cases passed` in the owned proof-cycle log recorded in the session state |

### Unit Tests

Rewritten 2026-11-22 to the tests that exist in the tree. "Written" means the
test exists; it records no run result. A planned test with no counterpart is
marked "Owed, not written". `vpp/` below is
`internal/component/l2tp/pppoe/dataplane/vpp/`.

| Planned question | Test that exists | Validates | Status |
|------------------|------------------|-----------|--------|
| Session data-plane registry | `TestSessionRegistrySelectsFactory`, `TestSessionRegistryRefusesRegistration` (`pppoe/dataplane/registry_test.go`); `TestLifecycleRegistry*` (`lifecycle_test.go`) | Selection of the registered provider; refused registrations | Written |
| Kernel parity | `TestKernelDataplaneParity` (`internal/component/l2tp/ppp/kernel/session_test.go`) | Kernel backend reproduces the AF_PPPOX + `/dev/ppp` sequence | Written |
| PPP over a non-fd frame channel, one reader (planned `TestPPPChannelSeam`) | None found | - | Owed, not written |
| Session created before PADS | `TestVPPFactoryPublishesOnlyCreatedInterface`, `TestVPPFactoryRollbackClosesSocketAndConfirmedSession` (`vpp/factory_test.go`) | Publication needs the created name and index, table and MTU acknowledgement; returning from `Open` permits PADS; rollback after a confirmed create | Written (provider). Server half written in `b6388ee211`: `TestPPPoESessionCreatedBeforePADS`, `TestPPPoESessionOpenFailureSendsNoPADS` (`internal/component/l2tp/pppoe/vpp_session_order_test.go`, a fake data plane built like the VPP provider's); the cut "PADS sent early" turns both red |
| Peer route on IPCP Opened | `TestVPPSessionPeerRouteUsesCreatedIndexAndTable`, `TestVPPPeerRouteWaitsForPostACKConnect` (`vpp/`) | Route add carries the session's created index and table | Written |
| Teardown order | `TestVPPSessionCloseOwnsSocketAndTeardown` (`vpp/session_test.go`) | Close removes every owned path before the VPP session and refuses later operations | Written. PADT after the session delete, written in `b6388ee211`: `TestPPPoEPADTAfterSessionDeletedOn{ServerTeardown,LCPTerminate,InterfaceRemoval}` (`pppoe/vpp_session_order_test.go`); the cut "close the transport after PADT" turns all three red, and "PADS sent early" also turns `ServerTeardown` red on its exact sequence |
| OD-2 on disconnect and reconnect (planned `TestPPPoEVPPReconcileOnReconnect`) | None. `TestVPPReconcile*` test the library `Reconcile`, which has no production caller | - | Owed, not written: OD-2 loss handling is not built (Final gates) |
| AC-15 policer features before policer | `TestVPPPolicerFeatureFenceCoversAllNativeArcs` (`vpp/rate_features_test.go`), `TestVPPPolicerOwnerCloseOrdersFeaturesBindingsAndObjects` (`vpp/rate_owner_test.go`) | Cleanup confirms every feature arc off, then each binding, then the owned policer | Written (provider). Owed: the production path, because `newPolicerRatePolicy` has no production caller (Final gates) |
| AC-16 frame channel closed once | `TestSessionFrameChannelClosedOnceByOwner` (`internal/component/l2tp/ppp/kernel/session_test.go`); for the VPP socket, `TestVPPSessionCloseOwnsSocketAndTeardown` | One owner closes the channel once | Written |
| AC-17 counters from the session handle | `TestVPPSessionCountersRequireExactCreatedIndex`, `TestVPPSessionCounterFreshnessBounds` (`vpp/`) | Counters read by the created index; failed and stale reads refused | Written. Owed: idle detection and accounting from these counters (resume step 5 (d)) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Session ID | 1-65534 | 65534 | 0 | 65535 (RFC2516-4-1) |
| Session MTU on VPP | 576-1492 (1500+ only with RFC 4638) | 1492 | 575 | 1493 without PPP-Max-Payload |
| Policer rate | existing `traffic.ValidateRate` bounds | per validator | per validator | per validator |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-vpp-backend-refused` | `test/parse/` | Offline validation refuses an unsupported PPPoE access backend, naming the path, the active and supported backends and the repair (toward AC-1) | Written |
| `pppoe-vpp-enabled-required` | `test/parse/` | Configured PPPoE access refuses a disabled VPP provider | Written |
| `pppoe-vpp-lcp-required` | `test/parse/` | AC-2: configured PPPoE access refuses VPP without control-plane pairs | Written |
| `pppoe-vpp-no-access-accepted` | `test/parse/` | Global PPPoE settings with no access interface are accepted on VPP | Written |
| Session add, route and teardown (planned `test/vpp/pppoe-vpp-session.ci`) | - | Covered against real VPP by `./le test deployment vpp-pppoe product` instead | Owed, not written as a `.ci` |
| CoA rate (planned `test/vpp/pppoe-vpp-coa-rate.ci`) | - | AC-10 | Owed, not written: the policer is not wired |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-vpp-ze-ac-pppd-client` (`-<mode>` for the other modes, `vpppppoe/product_plan.go`) | Not under `test/interop-pppoe/scenarios/`: the real-VPP lab `./le test deployment vpp-pppoe product`, `product-framed`, `product-ipv6` (`internal/le/test/deployment/vpppppoe/`) | pppd client, real VPP container, Ze | AC-3..AC-8, AC-11: discovery, authentication, IPCP/IPv6CP, data forwarded by VPP, clean teardown | `product` (IPv4) passed 2026-11-22, 4 of 4 ("Real-VPP product lab" under the checkpoint); its discrimination (revert, rebuild, red) is owed. `product-framed` passed 2026-11-22, 1 of 1 (`pppoe-vpp-ze-ac-radius-framed-route`, AC-7 Framed-Route); its discrimination is owed. `product-ipv6`: cannot pass, because its config writes a `pool { ipv6-na }` leaf the pool YANG lacks (`test-against-broken-path.md`, 2026-11-20) |
| `pppoe-vpp-restart` | - | same | AC-12 | Owed, not written: OD-2 loss handling is not built |

Goal validation: G-1 by the first scenario's VPP-counter and TAP-capture assertions; G-2 by AC-1/AC-13; G-3 by AC-6, AC-7, AC-9; G-4 by AC-8; G-5 by the restart scenario. Each scenario is reverted-and-rebuilt once to record its red (`ai/rules/interop-and-goal-validation.md`). The lab Docker host runs Ze's kernel (`ai/META-DECISIONS.md`).

## Files to Modify
- `internal/le/register.go` - compose the Phase 0 VPP build and mechanism actions
- `internal/component/l2tp/pppoe/server.go` - `handlePADR` calls the selected session data plane instead of `pppoeCreate`/`DevPPPSetup`
- `internal/component/l2tp/pppoe/subsystem.go` - select the data plane by backend name; discovery socket opened per data plane (cp TAP, netns); VPP readiness events
- `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - no leaf added (OD-3); the refusal `spec-pppoe-refuse-vpp-backend` lands is replaced by the data-plane selection
- `internal/component/l2tp/ppp/start_session.go`, `manager.go`, `session.go`, `session_run.go`, `ncp.go`, `ipv6_service*.go`, `ra*.go`, `dhcpv6*.go` - frame channel and per-session data-plane handle replace fds and `IfaceBackend`
- `internal/component/l2tp/` L2TP reactor kernel path - supplies the kernel handle (no behaviour change)
- `internal/component/l2tp/plugins/shaper/` - rate application through the session handle; VPP policer mode
- `internal/component/vpp/startupconf.go`, `config.go`, `yang/ze-vpp-conf.yang` - load `pppoe_plugin.so`; the patched `pppoe` startup section (OD-1 B)
- `rfc/short/rfc2516.md` - performer per backend on the rows above
- `feature-gates.txt` - dependent gate entry for the VPP session data plane

## Files to Create
- `internal/le/build/vpp/build.go`, `register.go`, `variant.go` - registered source build, retained image ownership and closed proof variants
- `internal/le/build/vpp/build_test.go`, `signal_test.go`, `variant_test.go`, `startup_test.go` - Phase 0 build and actual startup coverage
- `internal/le/test/deployment/vpppppoe/register.go`, `plan.go`, `check.go`, `wire.go` - native mechanism action, isolated peers and bounded state/wire assertions
- `internal/le/test/deployment/vpppppoe/control_linux.go`, `control_other.go`, `inject_linux.go` - unattached PPPoX control channel, namespace ownership and independent packet inputs
- `internal/le/test/deployment/vpppppoe/proof.go`, `plan_test.go`, `proof_test.go` - same-guest baseline/counterfactual orchestration and tooling tests
- `internal/le/test/deployment/vpppppoe/binding_proof_test.go`, `lease_proof_test.go` - binding attribution, composition and baseline image ownership tests
- `internal/le/test/deployment/vpppppoe/preflight_proof_test.go` - preflight ordering and cleanup diagnostic regressions
- `internal/le/test/deployment/vpppppoe/mtu.go`, `mtu_test.go` - independently measured VPP MTU boundary and cancellation-safe restoration
- `third_party/vpp-patches/Dockerfile`, `README.md`, `LICENSE` - source pin, provenance, build recipe and license
- `third_party/vpp-patches/0013-pppoe.-Automated-session-management.-12.patch`, `0014-pppoe-Added-option-enable-pass-nd-and-dhcpv6.patch`, `0018-policer-Added-features-for-unicast-multicast-arc.-29.patch`, `0025-T8231-Removed-PPPoE-input-feature-setup-41.patch`, `0031-PPPoE-Added-cp_binding_dump-API-call-49.patch` - unchanged adopted VyOS diffs
- `third_party/vpp-patches/0099-pppoe-bound-frame-parsing.patch`, `third_party/vpp-patches/proof/missing-frame-metadata.diff` - Ze receive hardening, optional real-buffer tests and the separate proof-only mutation
- `third_party/vpp-patches/0098-crc32-portable-scalar-helpers.patch` - portable seeded scalar helpers and the standalone build-time C regression
- `third_party/vpp-patches/ze/0100-pppoe-cp-binding-tests.patch` - optional C binding regression command; observed native failure precedes the separate `ze/0101-pppoe-validate-cp-bindings.patch` source repair
- `third_party/vpp-patches/ze/0102-pppoe-receiver-bounds-tests.patch` - optional real-graph regression of all receiver lanes, control dispatch, malformed counters and initialized traces
- `internal/component/l2tp/pppoe/dataplane.go` - the seam: interface and name registry
- `internal/plugins/pppoe/kernel/` - kernel session data plane (moved, not copied)
- `internal/plugins/pppoe/vpp/` - VPP session data plane (`ze_vpp` + `ze_l2tp`)
- `test/interop-pppoe/scenarios/pppoe-vpp-ze-ac-pppd-client/`, `pppoe-vpp-restart/`
- `test/parse/pppoe-vpp-*.ci`, `test/vpp/pppoe-vpp-*.ci`
- OD-1 B: `third_party/vpp-patches/` (diffs against the VPP release) and the build step that applies them

The session backend implementation uses the following locations. The original
planned paths above remain as history rather than instructions to create
parallel packages.

| Role | Current implementation |
|------|------------------------|
| Session provider interface and registry | `internal/component/l2tp/pppoe/dataplane/registry.go`, `session.go`; registry tests in `dataplane/registry_test.go` |
| Kernel session provider | `internal/component/l2tp/pppoe/dataplane/kernel/`; registered by `internal/component/l2tp/plugins/pppoe-kernel/register.go` |
| VPP session provider | `internal/component/l2tp/pppoe/dataplane/vpp/`; registered by `internal/component/l2tp/plugins/pppoe-vpp/register.go` |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | OD-3: no leaf added |
| YANG validation constraints | Yes | Backend verifier for PPPoE on VPP |
| YANG custom validators | Yes | LCP pair present for each PPPoE access interface |
| CLI commands/flags | Yes | Native VPP build and mechanism/proof actions; `show pppoe session` gains the data-plane interface |
| CLI grammar | Yes | Native lab/build actions use action-before-keyword grammar; the product `show` form is unchanged |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | `test/vpp/pppoe-vpp-*.ci` |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | Yes | `doctor-vpp-pppoe-plugin` (running VPP lists `pppoe_plugin.so`), following `doctor-vpp-lcp-plugin` |
| Prometheus counters/metrics | Yes | Per-session octets from VPP interface stats feed accounting (shared with `spec-pppoe-subscribers-produce-no-accounting-or-telemetry`) |
| BGP family surface | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | Yes | `features/` PPPoE entry, VPP data plane level |
| 2 | Config syntax changed | No (OD-3) | `docs/guide/configuration.md` states PPPoE follows `interface backend` |
| 6 | User guide page | Yes | `docs/guide/vpp.md` (status table, PPPoE section), PPPoE guide |
| 9 | RFC behavior proven | Yes | `rfc/short/rfc2516.md` performer annotations |
| 10 | Test infrastructure | Yes | `docs/labs/pppoe-interop.md` (VPP scenarios and proof), `docs/architecture/testing/qemu-integration.md` (manual QEMU carrier) |
| 12 | Internal architecture | Yes | `ai/digests/subscriber.md`, `ai/digests/vpp-dataplane.md`, `ai/doors/access-security.md` |
| 16 | Source anchors | Yes | Run `./le spec citation anchors spec plan/post-release/features/spec-pppoe-vpp-dataplane.md` at implementation |

## Implementation Steps

1. **Phase 0: Lab proof of mechanism (no product code).** Real VPP container (af_packet access interface, LCP pair), pppd client, Ze unmodified except a scratch harness. Prove A-1..A-5 and the upstream facts above by capture and `vppctl show pppoe session`. Apply the separately retained Ze parser hardening diff, run its real-buffer runtime test, and inject malformed frames to prove AC-18 through the receive entry points. Run the baseline and exact missing-metadata counterfactual on the same Ze-kernel Docker guest. Also prove AC-19 with divergent software/hardware indices, refused invalid/absent/mismatched map deletions, preserved unrelated bindings and successful exact-pair removal. Repair native CRC32C portability and require the strict 29-case build-time proof for AC-20. Output: the evidence recorded in this spec, run against the VPP Ze builds with the patches (OD-1 B).
2. **Phase 1: moved out.** The verify-time refusal of PPPoE under `interface backend vpp` is `spec-pppoe-refuse-vpp-backend` (OD-5), which owns the journal row and lands before this spec. This spec's AC-1 holds while no VPP session data plane is registered, and Phase 3's registration replaces that refusal with AC-2.
3. **Phase 2: The seam, kernel backend moved behind it.** `pppoe` declares the session data-plane interface and registry; the kernel implementation moves to its own package and registers; `ppp.StartSession` carries a frame channel and a per-session data-plane handle in place of `ChanFD`/`UnitFD`/`UnitNum` and `IfaceBackend`; L2TP supplies the kernel handle. The handle's contract names one owner for each descriptor it holds, which closes it exactly once (AC-16). No behaviour change. Tests: AC-13, AC-16, `TestKernelDataplaneParity`, `TestPPPChannelSeam`.
4. **Phase 3: VPP backend, IPv4.** cp map, discovery and session control on the cp TAP, session create before PADS, MTU, peer route in VRF, framed routes, teardown order, session counters and idle detection read through the session handle, `show pppoe` and doctor. Tests: AC-2..AC-7, AC-11, AC-14, AC-16, AC-17, interop `pppoe-vpp-ze-ac-pppd-client` IPv4.
5. **Phase 4: VPP backend, IPv6.** IPv6CP, RA, DHCPv6 IA_NA/IA_PD, delegated-prefix routes, through patch 0014's punt of RS and DHCPv6 to the cp (OD-1 B). Tests: AC-8.
6. **Phase 5: Rate limiting.** VPP policers per session, CoA replace, verify refuses tc qdisc modes under VPP. Removal order on teardown and on a CoA swap: the `policer-input` features leave the four arcs before the policer is unbound and deleted (AC-15). Tests: AC-9, AC-10, AC-15.
7. **Phase 6: VPP loss and restart.** OD-2 policy on `vpp/disconnected`/`vpp/reconnected`, cp map and LCP pair re-established, discovery resumes. Tests: AC-12, interop `pppoe-vpp-restart`.
8. **Phase 7: Docs, RFC annotations, discrimination records.** `rfc/short/rfc2516.md` performers, `./le rfc record` for each tagged unit, guides and digests.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | PADS never precedes a successful VPP session add; PADT never precedes VPP session delete |
| Control binding | Hardware lookup follows the supplied software index; invalid or mismatched map deletion preserves every existing binding, while exact-pair deletion works |
| Data flow | No subscriber IP packet traverses the Linux kernel on the VPP backend (capture) |
| Rule: no-layering | No path tries kernel then VPP; fds removed from `StartSession`, not kept beside the handle |
| Rule: exact-or-reject | traffic control shaping config refused under VPP, never mapped to a policer silently |
| Rule: process boundary | The VPP backend reaches the connector the same way `fib-vpp` does; nothing calls into `pppoe` from another component |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Seam with two registered backends | `rg RegisterSessionDataplane internal/` shows kernel and vpp registrations |
| VPP session data plane | interop `pppoe-vpp-ze-ac-pppd-client` passes with recorded red |
| Restart handling | interop `pppoe-vpp-restart` passes with recorded red |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Frames read off the cp TAP are parsed with the existing bounded discovery and PPP codecs; a frame whose (MAC, SID) has no session is dropped and counted |
| Spoofing | Session add only after cookie-verified PADR; a client MAC already bound on another access interface is refused (upstream `link_table` collision) |
| Resource exhaustion | `max-sessions` bounds VPP interfaces; PADI limiter applies on the TAP |

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

- The seam ng calls "session hooks" is a per-session data-plane handle, not a global function table. In Ze it is a registered backend that returns, per session, an object owning that session's data-plane state; the PPP package programs addresses, routes and rate through that object and never names a backend.
- Subscriber routes cannot go through sysrib and fib-vpp until `BestChangeEntry` carries an egress interface (seams-3). The VPP session backend installs them directly with its own route source, exactly as `internal/plugins/static/vpp` does for interface routes, and owns their removal.
- The cp TAP is a Linux interface in the LCP namespace, so Ze's existing raw-socket and PPP codecs carry over; only their binding target changes. That is why this is a backend and not a rewrite.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `test-against-broken-path.md` | 2026-11-20 | The product-ipv6 lab config uses a pool field (`ipv6-na`) Ze has never had; owed under G-4 and AC-8 |
| `identifier-reused-after-its-owner-is-gone.md` | 2026-11-22 | The kernel subscriber shaper applies and restores tc by `pppN` name (PPPoE handler registry, L2TP session events); owed under AC-26 and "Native PPP writes" |
| `unwired-feature.md` | 2026-11-22 | VPP subscribers forward unpoliced; a VPP session's CoA rate change is NAKed (policer wiring, AC-9, AC-10, AC-15) |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-22 | PPPoE has no operator clear-session command (L2TP has `clear l2tp session`); owed under AC-11 |
| `invariant-enforced-by-an-absent-call-site.md` | 2026-11-22 | `ClaimAccountingAssignment` and `AccountingAdmission` have no production caller: no accounting capacity is reserved before Accept |
| `invariant-enforced-by-an-absent-call-site.md` | 2026-11-23 | The VPP provider takes no process lease (F-1, F-2): after an API drop a later VPP exit's proof is lost and retained owners stay quarantined for good |

The earlier row this spec found (`validate-accepts-what-start-refuses.md`, 2026-10-28) is owned by `spec-pppoe-refuse-vpp-backend`.

## Work Not Done

### Wrong assumptions found during implementation

| Assumption | Observed result | Resolution |
|------------|-----------------|------------|
| A-3: rely on a preserved raw VLAN header and a parent-only CP map | The scratch helper received and emitted untagged discovery on `cp0.100`; capture showed PADO leaving Linux but no access egress. VPP's Ethernet classifier had no matching CP VLAN child | Use the fallback already named in A-3: create the data subinterface's complete LCP pair, read back its actual CP child and map child to child. The unchanged native VLAN mechanism then passed. This preserves the approved backend/control-plane design |

### Scope moved to another spec

| Item | Where it went | Why |
|------|---------------|-----|
| Phase 1: refuse PPPoE under `interface backend vpp` at verify | `spec-pppoe-refuse-vpp-backend` | OD-5: a shipped-behaviour fix belongs to the release; the rest of this spec is post-release |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1 Session data plane is a backend registered by name in the `pppoe` component, with kernel and vpp implementations in their own packages | A hook table like ng; a switch on backend name in `subsystem.go` | Registration pattern (`ai/doors/system.md`); a third backend (BSD netgraph/pipex, `spec-bsd-l2tp-pppoe`) plugs in the same way |
| D-2 One backend per configuration, no fallback; selection follows the interface backend | Kernel fallback when VPP is down | `ai/rules/no-layering.md`; a VPP-owned access NIC has no kernel data path to fall back to |
| D-3 `ppp.StartSession` carries a frame channel and a data-plane handle; fds and `IfaceBackend` are removed | Keep fds and add an optional handle | No-layering: two contracts for one job is the hybrid the rule bans |
| D-4 Ze keeps all control: discovery, PPP, auth, NCP, DHCPv6, RA, RADIUS | VPP auto session management (patch 0013 sniffing) | Ze is the RFC 2516 AC; VPP sniffing would make a second owner of session state |
| D-5 PADS after VPP session add; VPP session delete before PADT | Optimistic PADS | Preserves today's invariant (`handlePADR`) and RFC2516-5.5-4 |
| D-6 Subscriber routes installed by the VPP session backend directly | Through sysrib and fib-vpp | `BestChangeEntry` has no egress interface (seams-3) |
| D-7 Rate on VPP is policing, declared as such; traffic control shaping modes refused under VPP | Map TBF to a policer | Exact-or-reject (`fw-7-traffic-vpp.md`) |

## Owner decisions (decided 2026-10-28)

Thomas approved the recommendation for each of the five decisions. None is open.

| ID | Decision | Chosen |
|----|----------|--------|
| OD-1 | Which VPP | **B now, C in parallel.** Ze builds VPP itself with the VyOS patches (0013, 0014, 0018, 0025, 0031) kept as diffs against the VPP release and applied at build time, and generates its own `pppoe` binapi from the patched `.api`; in parallel the needed changes are offered upstream to FD.io, and a patch is dropped once its change lands in a VPP release Ze builds |
| OD-2 | VPP loss policy | **A.** On `vpp/disconnected` tear down every VPP session in Ze (no PADT is possible), Stop accounting with a lost-service cause, release pool addresses; clients redial when LCP echo fails |
| OD-3 | Config surface | **A.** No new leaf: PPPoE's data plane follows `interface backend`; `pppoe_plugin.so` loads whenever `pppoe` is configured with the VPP backend |
| OD-4 | Subscriber rate on VPP | **A.** Police both directions; TBF and HTB shaping under VPP are refused at verify until a VPP shaping mechanism exists |
| OD-5 | Release bucket | **Post-release**, except Phase 1 (refuse PPPoE on the VPP backend at verify), which moved to `spec-pppoe-refuse-vpp-backend` |

The options as they were put to the owner:

| ID | Decision | Option A | Option B | Option C | Evidence |
|----|----------|----------|----------|----------|----------|
| OD-1 | Which VPP | Upstream VPP only. Limits: one access interface per VPP (global cp, MAC-keyed `link_table`); VPP session created at first NCP Opened with the peer address (IPv6-only sessions need a workaround: unique peer link-local per session); upload policing via a classify table per session; IPv6 via VPP-native RA (`ip6_nd` API) and a punt socket for UDP 547 (new DHCPv6 transport in Ze) | Ze builds VPP itself with patches 0013, 0014, 0018, 0025, 0031 kept as diffs against the VPP release and applied at build time (the rule Ze already applies to Go modules); Ze generates the `pppoe` binapi from the patched `.api` | Upstream the needed changes to FD.io first and use A until they land | Upstream facts table above (`pppoe_cp.c`, `pppoe.c`, `pppoe_decap.c`, `node_funcs.c`); patches in `vyos/vyos-vpp-patches`; Ze today execs the host's `/usr/bin/vpp` (`internal/component/vpp/register.go`) and builds no VPP; VyOS ships B |
| OD-2 | VPP loss policy | Tear down every VPP session in Ze on `vpp/disconnected` (no PADT is possible: the control path is VPP), Stop accounting with a lost-service cause, release pools; clients redial when LCP echo fails | Keep Ze's sessions and replay VPP state on `vpp/reconnected`; a session whose client MAC VPP has not relearned (`INVALID_SW_IF_INDEX`) waits for the client's next control frame | - | ng does A (commit `90c6d35`); B depends on `link_table` relearning (`pppoe.c`) and on clients surviving the outage within their LCP echo budget |
| OD-3 | Config surface | No new leaf: PPPoE's data plane follows `interface backend`; `pppoe_plugin.so` loads whenever `pppoe` is configured with the VPP backend | Explicit `pppoe dataplane kernel\|vpp` leaf plus a `vpp plugins pppoe` toggle like `vpp plugins wireguard` | - | "every fact declared once" (`ai/rules/principles.md`); wireguard precedent in `startupconf.go`; ng uses a per-interface `vpp-cp=true` option |
| OD-4 | Subscriber rate on VPP | Police both directions (ng behaviour) and refuse TBF/HTB under VPP | Also implement shaping by another VPP mechanism | - | `vpppolicer.c`; `fw-7-traffic-vpp.md` exact-or-reject |
| OD-5 | Release bucket | Post-release feature (this file's bucket) | Release scope | - | `plan/README.md` buckets; Phase 1 is a shipped-behaviour fix and may move to `plan/release/fixes/` on its own |

## Known Limitations
- No VPP-side PADI or LCP policing of the cp path in this spec; that is copp work (`spec-dataplane-seams-0-umbrella` F-5).
- L2TP LNS sessions stay on the kernel data plane; VPP has an L2TPv3 plugin, not an L2TPv2 LNS.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The VPP backend's session add, PADS ordering, teardown ordering and MTU setting
each carry the RFC 2516 quote of the row they discharge; callers carry the
section reference (`ai/rules/rfc-compliance.md`, functions added from 2026-09-24).

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
- [ ] AC-1..AC-37 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped tests for the reached packages and behaviors pass in the main checkout; GitHub CI verifies the pushed commits, and agents run no `./le verify` command
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-pppoe-vpp-dataplane.md` only, in the same `./le commit create` script
