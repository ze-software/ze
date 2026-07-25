# Known Failures -- Resolved Archive

Verbatim archive of resolved and struck-through known-failure entries,
moved out of `plan/known-failures.md` when it was sharded into
`plan/known-failures/`. Entries here are never edited; they exist as
history so a cleared red is not simply forgotten. `collect_known_failures`
(`scripts/dev/testing_health.py`) counts every `### ` entry in this file as
resolved.

## Struck-through (resolved in place, were not yet under `## Resolved`)

### ~~`ze-test bgp plugin` dest-peer-teardown cluster (85, 97, 222, 398)~~ -- RESOLVED 2026-07-25: all four members addressed, and the shard was already stale for one

222 `forward-congestion-teardown-metrics` and 398 `role-otc-unicast-scope` were
fixed on 2026-07-25 (`52ad2f71b`); see the sibling entries for the observer
readiness gate and the missing `option=linger`. The remaining two:

**85 `bgp-rs-asn4-transcode` -- the shard was already stale for this member.**
Its fail-open observer had been replaced by the event-driven
`ze_api.run_rs_observer` in `4b52d74a6` (2026-07-24), two days AFTER the shard
was written on 2026-07-22, by unrelated work. That helper subscribes and blocks
on each peer's EOR event and fails closed, which is exactly the lever that fixed
222. So this member has a producer-level reason to have stopped reproducing; the
shard simply outlived its own subject. Another stale shard in this sweep -- re-run
a shard's own reproduction before believing it.

**97 `bmp-locrib` -- a genuine fail-open, now fixed.** Unlike 85 this file had
not changed since 2026-07-15, before the shard was written. Its shutdown observer
gated on a `.collector-done` marker written by the BMP collector, but:

- the wait was a 15s deadline that FELL THROUGH to `request shutdown` whether or
  not the marker ever appeared, so a run that never validated the behaviour under
  test shut ze down anyway -- surfacing as the peer's `connection closed before
  completion`, which is this shard's recorded symptom reported as a symptom
  rather than a cause; and
- it bridged to the peer's only expectation (ze's End-of-RIB) with
  `time.sleep(0.3)`, a sleep rather than a barrier, even though the collector's
  marker says nothing about whether the peer has been served its EOR.

Replaced with a single fail-closed `wait_until` + `runtime_fail` on the marker.
Mutation-verified -- with the marker made unreachable the test reports
`ZE-OBSERVER-FAIL: bmp collector never wrote .collector-done: Loc-RIB route
monitoring was never validated` instead of stopping ze and blaming the peer. 3/3
green afterwards, and faster without the sleep.

**An establishment barrier was tried here first and does not work; independent
review caught it and the record is corrected rather than left standing.** The
first attempt copied the established+`quiesce()` fence from
`api-rib-clear-out.ci`, and it was vacuous twice over: the predicate substring
`'established'` is satisfied by the KEY `connections-established` that every peer
row carries (`internal/component/bgp/plugins/cmd/peer/peer.go:238`), so it
returned before any session existed, and `quiesce()` then also returned at once
because `PendingSync` is false for an idle peer. Rewriting it to parse the real
`state` field made the test FAIL, which is what exposed the deeper point: this
test's check peer has no `option=linger`, so it exits the instant it receives
ze's EOR and the whole exchange is over ~4 ms after startup. The state such a
barrier waits for is already gone before an external observer can poll it. The
marker is the only fence actually available here, and it is a sound proxy: it is
written only after a validated Loc-RIB PDU, which requires the peer's UPDATE to
have been installed, which is after ze put the peer's EOR on the wire.

Neither 85 nor 97 reproduced this session, including in a deliberately contended
full-suite run (load 13.47 on 16 cores with two agents building). For a
load-sensitive cluster that is data, not proof, which is why each member was
closed on a producer-level reason rather than on green runs.

### ~~`ze-test bgp reload` -- fixed startup deadlines fail under CPU oversubscription~~ -- RESOLVED 2026-07-25: every finding closed, and finding 3's hang was disproven

The headline issue -- reload tests failing on FIXED startup deadlines under
deliberate CPU starvation (`peer did not start listening within 5s`) -- is fixed
at source. Those inner readiness gates now scale by the same parallel headroom as
the outer per-test budget: `withParallelHeadroom`
(`internal/test/runner/runner_exec_util.go:147`) is applied to both ze-peer bind
barriers (`runner_exec.go:148`, `:904`), both daemon.ready waits, the
`await=stderr` fence (`await_stderr.go:87`), and the HTTP readiness
wait/retry-count/per-request budgets (`runner_validate.go:508`, `:573`, `:722`).
Its doc comment names this shard as the class it closes. In 2026-07-23 this was
blocked only because a concurrent session held those files.

Found while closing it: `peerBindFailure`
(`internal/test/runner/peer_contract.go:160`) still hardcoded "within 5s" in its
message, so a parallel run reported a budget it had never applied. It now takes
the enforced deadline; its generic test asserts the widened value and rejects
"within 5s", mutation-verified against a reverted producer.

Finding 1 (`await=stderr` never `Wait()`s the peer): already fixed by
`drainPeers`. Finding 2 (eight vacuous reload tests): fixed, and it uncovered two
real product defects -- the netlink monitor dropping address events for
pre-existing links, and the same-subnet secondary flush -- both fixed and archived
separately.

**Finding 3 (the daemon hangs when it cannot apply interface config) is
DISPROVEN.** Reproduced in the QEMU Alpine VM as an unprivileged user (uid 1000,
no CAP_NET_ADMIN) with a config carrying only `interface { dummy zdiag0 {} }`:
the daemon EXITS rc=1 in under a second, by both entry points, while the same
binary and config as root started normally and created the device. The guard
already fails CLOSED. What was really wrong is that the error dropped its cause:
`runPluginPhase` synthesized the phase error from `proc.Stage()` alone while the
real reason was logged at Debug and discarded, so an operator got "plugin
interface failed during startup at stage Config" -- no offending object, no
reason, no corrective action. Fixed: `Process.SetStartupError` records the
handshake cause, `startupFailureError`
(`internal/component/plugin/server/startup_failure.go`) wraps it, and
`joinApplyErrors` (`internal/component/iface/register.go`) appends the
CAP_NET_ADMIN remediation.

Two stale details in the original finding, corrected: the failure is at stage 2
(configure), not stage 3 -- the plugin-side stage-3 error is a consequence of the
already-closed pipe -- and `ze <config>` as a bare positional no longer exists
(`cmd/ze/ze_core_dispatch.go:402-404`). `ai/rules/qemu-testing.md` carried the
same stage-3 wording and the same implication that the DAEMON hangs; it now says
the TEST hangs, which is what actually happens (the check peer goes on waiting
for a session the exited daemon will never open).

### ~~`l2tp` functional suite `session-stopccn-cascade`~~ -- RESOLVED 2026-07-25: original diagnosis refuted, test is correct as marked

The shard blamed the answering-side reliable-receive path: "ze's receive window
does not advance past the second session's rapid-fire ICRQ". That has no producer.
`OnReceive` classifies purely on `hdr.Ns == e.nextRecvSeq` and advances on every
in-order delivery (`internal/component/l2tp/reliable.go:487-499`); there is no
window gating on the receive path at all -- `e.win` governs only the send side. The
`.ci` peer sends strictly increasing Ns 0..6, so the reorder queue is never touched.

The real cause is the missing CAP_NET_ADMIN, which the test's own
`option=needs-linux:caps=net-admin` marker already states: the genl tunnel create
returns EPERM, `handleKernelError` tears the sessions down
(`reactor_kernel.go:388-411` -> `session_fsm.go:434-455`), so `clearSessions`
returns nil and the asserted log at `tunnel_fsm.go:596` is suppressed. The marker
is correct and was kept.

The investigation did find a separate REAL bug, now fixed (`baf45d6bd`):
`reorderEntry` carried only (ns, payload), so gap-fill delivery hard-coded
SessionID 0 -- a reserved id that is never allocated -- and every reorder-queued
ICCN/ICRP/OCCN/OCRP/CDN/WEN/SLI was dropped as "unknown SID". Coverage no longer
depends on the privileged `.ci`: `TestSession_StopCCNCascadeThroughEngine` drives
the same sequence over real UDP through the reactor, mutation-verified against a
no-op `clearSessions`.

### ~~`internal/component/iface` `TestIntegrationApplyConfigVLANUnitAddressReconcile`~~ -- RESOLVED 2026-07-25: Linux same-subnet secondary flush

Root cause is the kernel, not the reconcile logic. `10.60.200.1/24` ->
`10.60.200.2/24` is a renumber INSIDE one subnet, and the reconcile is
make-before-break: `config_apply.go:898` adds the new address, which Linux marks
`IFA_F_SECONDARY`, then `:916` removes the old one -- and `__inet_del_ifa` deletes
every same-subnet secondary along with the primary unless
`net.ipv4.conf.<dev>.promote_secondaries` is 1. Both `RemoveAddress` calls return
nil, which is exactly why `applyConfig` reported no errors while the address was
gone.

Verified directly in the QEMU VM, independent of ze: the sibling shows as
`secondary`, deleting the primary leaves `ip -4 addr show` EMPTY, and with
`promote_secondaries=1` the sibling survives and is promoted.

Fixed in `b8fdfeb5b`: `RemoveAddress` enables the knob before a cascading delete
and REJECTS the removal naming the endangered addresses if it cannot. The test now
passes in QEMU (`--- PASS: TestIntegrationApplyConfigVLANUnitAddressReconcile`),
alongside two new integration tests for the sibling-preserved and other-subnet
cases.

### ~~`bgp plugin` `role-otc-export-unknown` -- rare spurious withdraw~~ -- RESOLVED 2026-07-25: the test let its own source peer disconnect

The daemon was right. Without `option=linger` a check peer closes the moment its
script completes, ~1 ms after sending its UPDATE; `adj_rib_in/rib.go:573-576` then
deletes its RIB on session-down, and `bgp-rs` `handleStateDown` ->
`sendBatchedWithdrawals` (`rs/server_handlers.go:90-149`) emits
`del prefix 10.0.0.0/24` to the destination peer -- byte-for-byte the captured
frame `001B:02:0004180A00000000`.

Two earlier revisions of this shard reasoned from false premises and both were
corrected: the RFC 9494 conversion in `forwardUpdateCore` was correctly ruled out,
but the "unexamined candidate: the peer-down withdrawal path" it named was right
and had been wrongly dismissed on the belief that `bgp-rs` was not loaded. It is:
a `bgp {}` config auto-loads ~20 BGP plugins before the `.ci`'s own stanzas, so a
plugin set must be read from the daemon's `startup tiers computed` line, never
from the `.ci`.

The test also asserted nothing: its gate was `if total < 0`, which can never fire,
so `total-routes == 0` passed silently on every run. Fixed in `52ad2f71b`:
`option=linger` on the source peer and the gate tightened to `total < 1`,
mutation-verified by removing linger (both go red). Sibling
`role-otc-unicast-scope` had the same two defects and the same fix. Confirmed by a
full `plugin` suite run at 495/495.

### ~~`rsvpte-lsp-setup` -- load-only `slice bounds` panic~~ -- RESOLVED 2026-07-25: stale, closed on a third independent sweep

The 2026-07-13 UPDATE already disproved the cap-512 diagnosis. A third independent
repo-wide sweep (2026-07-25) reconfirms it: all 15 cap-512 sites enumerated, none
can be resliced past 512 -- the two format scratches keep their `n > cap(raw)`
guards and the rest are append-only, ioctl-fixed, or guarded upstream;
`internal/core/textbuf` is clean. With 0 reproductions in 160 prior runs and the
2026-07-04 crash predating the plugin-startup/RPC-dispatch refactors (`1eb89f509`,
`3404c4396`, `8f3203ef5`) that rewrote that exact area, it is either already fixed
or was a truncated 2-line aggregator stack misattributing another suite's crash.

The sweep did find a real, remotely-triggerable panic elsewhere, now fixed
(`457ec9ee8`): `handlePendingCollision` read `buf[HeaderLen:hdr.Length]` into a
4096-byte buffer using the raw wire Length, with no `ValidateLengthWithMax` --
unlike both sibling read paths. Any host able to open a colliding TCP connection
to a configured peer address could panic the reactor before capability
negotiation.

### ~~`ze-test bgp plugin 458 show-l2tp-tunnel-detail` -- deterministic on darwin~~ -- RESOLVED 2026-07-25: no longer reproduces

Re-run on darwin with the current tree: `ze-test bgp plugin --pattern
show-l2tp-tunnel-detail` -> PASS (2.8s, test id 460 after renumbering), and the
full plugin suite ran 495/495 in the same session. The shard asserted no root
cause ("not traced -- outside this session's area"), so nothing here identifies
which change cleared it; it is closed on the evidence that it passes, not on a
fix. If it returns, re-open with the daemon stderr rather than reusing this entry.

### ~~`ze-test install` kernel tests (1-6, 20-31, 39, 40) -- broken by the isolated-binary layout~~ -- RESOLVED 2026-07-25: fixed by `ebf0dfbad`, shard was stale

The shard (2026-07-22) predates the fix. `ebf0dfbad` (2026-07-24, "fix(test):
install kernel suite green in isolated ze-verify layout") changed the 20 `.ci`
files to resolve the repo as `${ZE_REPO_ROOT:-$(... command -v ze ...)}`, so the
runner-exported `ZE_REPO_ROOT` (`internal/test/runner/runner_exec_util.go:118`)
wins over the binary-neighbour derivation the shard blamed. Verified 2026-07-25
by rebuilding the isolated layout by hand (`tmp/isolab/bin/{ze,ze-test}`) and
running `ZE_BIN=... ZE_TEST_BIN=... ze-test install --all` -> `pass 37/37`
(3 honest skips). The guard is now load-bearing for a second reason: the runner
puts a bare-name shim dir on PATH, so `dirname(command -v ze)/..` is a temp dir
in EVERY layout, not just the isolated one.

### ~~`static` functional suite `004-show` -- obsolete config syntax + darwin daemon boot~~ -- RESOLVED 2026-07-25: rewritten, plus 4 sibling defects found and fixed

Both shard claims confirmed and fixed, and running the suite under QEMU surfaced
three more:

- 004 used the obsolete flat `next-hop <addr> { }` form; the schema models
  `next { hop <addr> { } }` (`internal/plugins/static/yang/ze-static-conf.yang:77-119`).
- 004 and 005 drove `ze cli -c "show static"`, which speaks SSH
  (`internal/component/cli/client/main.go:380`) against configs declaring no SSH
  server -- unreachable on any platform. Both rewritten to dispatch through an
  external plugin, with structured assertions (route count, (prefix, table)
  pairs, named table id, interface-only next-hop) replacing substring matches.
- The suite registered `DefaultParallel=0`, so all 7 daemons wrote the ONE kernel
  routing table concurrently: 5 of 7 failed under QEMU with cross-test symptoms
  ("reload did not remove 172.16.0.0/12" while another test's blackhole was in
  the dump). Now registered serial (`internal/test/cli/register.go`).
- 006 asserted "no interface backend loaded", which is unreachable on Linux:
  omitting the `interface { backend }` stanza defaults to netlink
  (`internal/component/iface/default_linux.go:8`), and on darwin the daemon never
  reaches static. It could pass on no platform. Rewritten to assert the
  reachable behaviour (route skipped, interface named, daemon survives); the
  no-backend message keeps its deterministic unit test
  (`internal/plugins/static/backend_linux_test.go:19`).

Verified: `ze-test static --all` under QEMU -> `pass 7/7`.

### ~~`sync.Pool` capacity/identity unit flakes under full-suite GC pressure~~ -- RESOLVED 2026-07-25: both tests asserted a guarantee sync.Pool does not make

`TestPoolPreservesCapacityWithoutString` (textbuf) and
`TestGetReturnsSameBufferAfterPut` (bufpool) each did one Put/Release then one
Get and asserted the SAME buffer came back. sync.Pool promises no such thing: it
is emptied at every GC, and an item parked in one P's private slot cannot be
stolen by another P -- so under the parallel suite's memory pressure the
assertion legitimately failed. Both now disable GC for the test
(`debug.SetGCPercent`) and retry, asserting the property holds at least once,
which removes the environmental variable without weakening the check. Both were
mutation-verified: making bufpool's `Put` a no-op and making textbuf's `Get`
always reset to the inline array each turn the corresponding test red.

### ~~`.ci` suites -- 108 quick-exit `ze` commands across 50 files are silently unasserted~~ -- RESOLVED 2026-07-25: 149 commands armed across 52 files

Re-measured on the current tree: 149 unasserted quick-exit `ze` commands in 52
files with more than one such command (files with exactly one are already
covered by the file-level `expect=exit:code=`). All 149 now carry
`cmd=...:exit=N`, taking N from the `expect=exit:code=` the test author had
already written directly beneath each command -- so the assertion added is the
intent already recorded in the file, never a guess and never a snapshot of
current behaviour. Six commands had no adjacent declaration and were armed by
hand after checking the real exit status (two "bad config" validations do reject
with exit 1; four `config import` / `connect add` setup steps do succeed).
Mutation-verified: claiming `exit=1` on a seq=1 command that really exits 0
produces `cmd seq=1 (ze config validate -): expected exit code 1, got 0`, a
failure the old file-level assertion could not produce. Arming surfaced no
product defect: ospf 81/81, ospfv3 11/11, isis 12/12, bgp parse 254/254,
ui 156/156, appliance 11/11, install 37/37, firewall, vrrp all green.

### ~~`ze-test bgp plugin 224 forward-overflow-two-tier` -- deterministic dest-peer teardown on darwin~~ -- RESOLVED 2026-07-24: real forwarding bug, fixed under `spec-fixit-peer-verdict-and-forward-rail` (defects 7-10)

The shard's own triage was right ("if it reproduces on Linux it is a real
forwarding/establishment bug and belongs in a spec, not here"). The cause was
not darwin timing but four real defects, all fixed 2026-07-24: (7) a forward-pool
FIFO violation across the overflow/channel boundary; (8) the test asserting
per-message NLRI framing ze does not owe (the forward rail packs same-attr NLRIs);
(9) the source check-peer closing its session the moment its script completed, so
ze correctly withdrew its routes mid-burst; and (10) the observer shutting ze down
after only ONE route reached adj-rib-in, truncating the throttled forwarding. 224
now passes 10/10 in isolation and in the full `bgp plugin --all` suite. See the
`plan/spec-fixit-peer-verdict-and-forward-rail.md` addendum.

Original shard (verbatim):

> `ze-test bgp plugin 224` (forward-overflow-two-tier) fails deterministically --
> 4/4 isolated runs, RC=1, each ~12-13s. The dest peer (127.0.0.2) exchanges OPEN
> then closes: `failed: connection closed before completion`; the source's
> 50-route burst never reaches adj-rib-in; the EOR dispatch then fails with
> `no established peers to send to`. Forces overflow with `ZE_FWD_CHAN_SIZE=2`
> plus a 50-UPDATE burst under a 20s timeout. A HEAD baseline (`dfb8c01ac`) failed
> 3/3 with the identical symptom, so it was not caused by
> `spec-fixit-bgp-concurrency-races`. Root cause was left unasserted at the time.

### ~~`internal/component/doctor` -- 4 listener/schema tests fail on macOS~~ -- RESOLVED 2026-07-15: DISPROVEN, was a missing-build-tags artifact

**The macOS diagnosis was wrong. These tests pass. Nothing to fix.**

Original entry (2026-07-02, "re-confirmed" 2026-07-03) blamed this specific macOS
host's socket stack for allowing a second bind where the test expects exclusivity
(a supposed `SO_REUSEPORT`/dual-stack quirk), covering
`TestCheckListeners_PortInUse`, `TestCheckListeners_API`,
`TestCollectSchemaListeners_SSHDefault`, `TestCollectSchemaListeners_SSHExplicit`.

Disproven 2026-07-15 on the same macOS host, by running the four tests both ways:

| Invocation | Result |
|---|---|
| `go test ./internal/component/doctor/ -run '...'` | FAIL (all 4) |
| `go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt)" ./internal/component/doctor/ -run '...'` | **PASS (all 4)** |

Root cause: bare `go test` omits the feature gates the Makefile always supplies
(`Makefile:51` `ZE_FEATURES` from `feature-gates.txt`; `Makefile:65`
`GO_TEST_TAGS = ze_core $(ZE_FEATURES) $(ZE_TAGS)`). Without `ze_web`/`ze_ssh`/
`ze_rest` the listeners the checks expect never register, so `checkListeners`
correctly reports nothing and the assertions fail. The host was never at fault.

**Lesson: bare `go test` is NOT equivalent to `make ze-unit-test` in this repo.**
Feature-gated plugins compile out (`//go:build ze_isis`, `ze_ospf`, `ze_web`, ...),
so a bare run produces phantom reds. Reproduce a red with the Makefile tags before
logging it here.

### ~~`internal/component/config/schema/cli` `TestCmdMethods`~~ -- FIXED 2026-07-15

This one was REAL (it failed with the Makefile tags too, unlike the two
disproven entries above) and the diagnosis here was correct.

Confirmed pre-existing 2026-07-15 by `git archive HEAD` into a scratch tree and
running the test there: it failed identically on committed `HEAD`
(`main_test.go:470: expected 13 system RPCs, got 14`), so the VRRP work did not
cause it. The 14 `ze-system-api` RPCs are all generic daemon methods; none is
VRRP -- verified by enumerating them, `ze-system:quiesce` included.

Cause: the test hardcodes per-module RPC counts, and `ze-system:quiesce` was
added without updating the literal.

Fixed by bumping the literal to 14 (`main_test.go`). The suggestion to derive the
expectation instead (`ai/rules/derive-not-hardcode.md`) was considered and NOT
taken: a count derived from the same registry the test reads would be
tautological. The shape that actually catches silent removal is a golden snapshot
file, as `plugin/all/testdata/*.snapshot` does. Left as a note in the test rather
than a silent bump; converting these four counts to a snapshot remains open for
whoever next touches that file.

**A SECOND test had the same stale-`quiesce` root cause** and was only found when
`make ze-verify` finally ran (2026-07-15): `internal/core/ipc` `TestExtractRPCs/system-api`
(`yang_test.go:320-326`) asserts `ElementsMatch` against its own hardcoded
`wantRPCs` list, which also predated `ze-system:quiesce`. Fixed by adding the
entry. Two hardcoded copies of the same list drifted from one added RPC, which is
precisely what `ai/rules/derive-not-hardcode.md` exists to prevent -- worth
remembering if a third copy surfaces.

### ~~`config/cli` -- 3 tests fail, pre-existing~~ -- RESOLVED 2026-07-15: DISPROVEN, same missing-build-tags artifact

**The `config-listener-conflict` validator is NOT broken. These tests pass.**

Original entry (2026-07-03, "re-confirmed" 2026-07-09) claimed the validator
produced no diagnostic "in this build", covering
`TestValidateListenerConflictRelated` (cmd_validate_test.go),
`TestConfigFixPlanRepairIDs` and `TestConfigFixPlanRepairIDsFromFix`
(cmd_fix_test.go:106).

Disproven 2026-07-15 by running the three tests both ways:

| Invocation | Result |
|---|---|
| `go test ./internal/component/config/cli/ -run '...'` | FAIL (all 3) |
| `go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt)" ./internal/component/config/cli/ -run '...'` | **PASS (all 3)** |

Root cause: identical to the doctor entry above. All three fixtures configure
`environment { web { server ... } }`, so they need `ze_web`. Bare `go test` omits
it, the web listener validator never registers, no diagnostic is produced, and the
tests fail. "In this build" was the clue -- it was literally a different build.

Note the failure mode this created: the entry named a specific producing
component as broken, and a later session "re-confirmed" it by repeating the same
flawed invocation. Symptom-matching a red does not attribute it. Reproduce with
the Makefile tags first.

### ~~`internal/chaos/inprocess` `TestInProcessChaosReconnect`~~ -- FIXED 2026-07-16: the runner stopped virtual time while the system still needed it

**Fixed in `runner.go` (Run's teardown). Never a flake, and never a BGP defect.**

History: logged 2026-07-08 as a `-race` flake ("passes 3/3 without `-race`"); a
2026-07-16 update correctly bisected the widening to `44ad25d23` (`fix(bgp):
reconnect backoff floor 5s, not 120s connect-retry`) and correctly said DO NOT
revert it. It left one open question -- "the advance loop burns 90s real ... where
that real time goes is UNVERIFIED".

**The premise was wrong: none of it is spent in `vc.Advance`.** A goroutine dump
taken 30s into the freeze pinned it in two stacks:

| Goroutine | Where |
|---|---|
| runner | `simWg.Wait()` (`runner.go:594`) -- the advance loop had ALREADY finished |
| ze session | `VirtualClock.Sleep` (`virtualclock.go:49`) from `session.go:767` |

The advance loop does exactly what `runner.go:427-430` implies: 60 virtual seconds
in ~0.6s real. Then it exits -- and **nothing advances the clock again**.
`session.Run()` polls for its connection with `s.clock.Sleep(10ms)`
(`session.go:762-768`), and `VirtualClock.Sleep` is a bare `<-ch`
(`virtualclock.go:47-50`); `clock.Clock.Sleep` takes no ctx, so `simCancel()`
cannot reach a goroutine parked there -- only `Advance` can. ze's session was
stranded mid-sleep, never finished the handshake, the simulator blocked forever on
the reply that never came (`executeReconnectStorm` -> `readMsg`,
`simulator_actions.go:233`), and `simWg.Wait()` hung until the test's own 90s
context tore the sockets down. Hence 92.00s, and `established==1` because the peer
was asleep, not because reconnect was broken.

`44ad25d23` only changed WHEN ze lands in that sleep: the advance loop finishes in
~0.6s real, so a chaos action firing late in the virtual window is still
mid-handshake when time stops. The 120s floor parked the retry outside the window
and hid it. The latent defect predates it.

Fix: keep advancing the virtual clock during teardown, until both the simulators
and the reactor are down. Real time does not stop while a system shuts down, and
neither may virtual time.

Verified: 3/3 PASS in **3.70s** -- matching the 3.69s measured at `8f5f2ff4b`
(2026-07-08, before the regression), vs 92.00s broken. Full `./internal/chaos/...`
tree green; the target test 2/2 green under `-race`. `make ze-lint-changed`: 0 issues.

Two lessons worth keeping. (1) This was logged as non-deterministic but failed
**3/3 in BOTH modes** -- a deterministic red, which this file's own scope rule says
never belongs here. Re-measure before inheriting a "flaky" label. (2) Three
plausible mechanisms (the new iface chaos weights; the blocking timer send at
`virtualclock.go:168`; "the advance loop is slow") were each disproven by
experiment. The goroutine dump settled in one run what code-reading had got wrong
three times: when a test hangs, dump the stacks before theorising.

### ~~`internal/component/l2tp` `TestPeerTeardownWithdrawsSubscriberRoute`~~ -- FIXED 2026-07-16 (`9af30c440`): the test violated a documented setter contract

The diagnosis here was correct (write at `reactor_setters.go:114` vs read at
`reactor_kernel.go:263`, the test calling `SetRouteObserver` after `Start()`), and
the suggested fix -- set the observer before `Start()` -- was the one taken.

Two corrections for the record. **It was not load-sensitive and not 1-in-3**:
`-race` reports it on the FIRST run, 3/3, both alone and in the full package. The
"1/3 under `-race -count=3`" measurement understated it. And it was never a
product bug: `SetRouteObserver` documents "MUST be called before `Start()`; the
goroutine creation barrier synchronizes the write here with reads in the run
loop" (`reactor_setters.go:106-109`), and the sole production caller honours it --
`subsystem.go:241` installs the observer, `:313` starts the reactor 72 lines
later. The lock-free write is deliberate: the reload-time setters
(`setHelloRetries` and friends, `reactor_setters.go:18-98`) DO take `tunnelsMu`
because `subsystem_reload.go` calls them on a live reactor, while the install-time
setters trade the lock for the `Start()` happens-before edge. Adding a mutex would
have weakened a working design to accommodate a misusing test.

Fixed by giving the builder an observer parameter that installs before `Start()`
(`buildLogReactorWithClockObserver`); the other 11 callers are untouched and the
wrapper passes nil, a documented no-op. `setHelloRetries` stays after `Start()`,
which is safe -- it locks.

Verified: the two affected tests 5/5 under `-race`, and the whole
`./internal/component/l2tp/...` tree green under `-race`.

### ~~`internal/component/l2tp` `TestReactorKernelDisabledReturnsNil`~~ -- FIXED 2026-07-16: stale assertion retired, verified on real Linux

The diagnosis logged here was exactly right, and the fix is the one it prescribed.
Recorded because the evidence is now direct rather than inferred: this is a
`_linux_test.go`, so it does not build on the macOS dev host (`go test` reports
"no tests to run"), which is why it sat open. Run in a `golang:1.26` container --
the test is pure logic (no netlink, no privileges), so a container suffices:

| Tree | Result on linux/amd64 |
|---|---|
| committed HEAD (`require.Nil(t, teardowns)`) | **FAIL** `reactor_kernel_linux_test.go:159: Expected nil, but got: []l2tp.kernelTeardownEvent{{localTID:0x66, localSID:0x9}}` |
| with the fix | **PASS**; full `./internal/component/l2tp/...` tree green |

`0x66`/`0x9` are exactly the event the test seeds, confirming the producer:
`collectKernelEventsLocked` drains `pendingKernelTeardowns` BEFORE the worker
check and returns them (`reactor_kernel.go:23-27`), deliberately, so the route
observer learns of torn sessions with no kernel worker present. That is the
mechanism `TestPeerTeardownWithdrawsSubscriberRoute` depends on -- the same
teardown-withdraw path whose `-race` bug was fixed today in `9af30c440`.

Fixed by retiring the teardowns-nil assertion and asserting the drained event
instead (a stricter `require.Equal` on the exact event, plus a drained-list
check: 4 assertions where there were 3). Renamed to
`TestReactorKernelDisabledSkipsSetupsButStillDrainsTeardowns`, because
"ReturnsNil" now describes only the setups. The surviving assertions (setups nil,
`kernelSetupNeeded` not cleared) were correct and are kept.

Note this was a DETERMINISTIC red, so like the chaos entry above it never
belonged in a file scoped to non-deterministic failures. Two of this file's
entries turned out that way today.

Confirmed pre-existing (git-blame) 2026-07-10: the test asserts
`require.Nil(t, teardowns)` from `collectKernelEventsLocked` when no kernel
worker is present (`reactor_kernel_linux_test.go:159`, last touched 2026-06-12).
But commit `e231fbfdd` (2026-06-26, "withdraw subscriber routes on
peer-initiated teardown") deliberately made `collectKernelEventsLocked`
drain `pendingKernelTeardowns` **unconditionally** so the route observer learns
of torn sessions even with no kernel worker (see the function's leading
comment). The test's teardowns-nil assertion has been failing deterministically
since that commit; the setups-nil and flag-not-cleared assertions remain
correct. Fix: update the test to expect the drained teardown event. Owner:
whichever session next touches `internal/component/l2tp/reactor_kernel_linux_test.go`.

## Resolved

### 2026-07-08 -- `internal/plugins/ospf` `virtual_link.go` `-race` data race -> two bugs fixed at source

**Resolved 2026-07-08.** The reported write was `virtual_link.go:160`
(`rt.reachable = r.Reachable` in `(*engine).onVirtualLinksResolved`); a `-race`
rerun surfaced the missing second stack, which pinned TWO distinct bugs:

1. **Production pointer escape.** `virtualLinkTopology()` guarded the
   `e.virtualLinks` map with `e.mu` but snapshotted `*virtualLinkRuntime` pointers
   and read their mutable fields (`cost`, `localAddr`, `cfg`) AFTER unlocking,
   racing `onVirtualLinksResolved`'s writes -- the retransmit loop
   (`instance.go:730` -> `originateSelfLSAs` -> `virtualLinkTopology`) is a real
   lock-free reader. Fixed by snapshotting VALUES under `e.mu` (`virtualLinkConfig`
   is a pure-value struct); `virtualNeighbors` now takes the name string so no
   runtime pointer escapes the lock.
2. **The reported race (test).** `TestVirtualLinkResolutionDrivesRuntime` drove
   `onVirtualLinksResolved` directly, which re-triggers SPF; the live computer's
   50ms back-off timer re-entered the callback on its own goroutine and wrote
   `rt.reachable`/`rt.cost` (`virtual_link.go:160`) while the test read them
   lock-free (`virtual_link_test.go:119`). In production the callback only ever
   runs on the single SPF goroutine, so the overlap is test-only. Fixed with
   `e.spf.Stop()` up front (callback runs synchronously); also removes a latent
   flake -- with no transit topology the async run resolves the link back down.

Verified: `go test -race ./internal/plugins/ospf` full package PASS; the reported
test x50 and all `Virtual*` x10 PASS under `-race`; `make ze-lint-changed` green.

### 2026-07-07 -- `ze-tier-check` `routeinstall` unclassified non-engine placement -> moved to core

**Resolved 2026-07-07.** Root cause: `internal/plugins/routeinstall` (added by
`f5057cd2a`, learned 1070) is a pure client-side library -- no `sdk.NewWithConn`,
no `init`/`register.go`, imports only `internal/core/*` + `pkg/plugin/rpc` -- so
the tier gate correctly flagged it as an unclassified non-engine placement in the
plugin tier. It was NOT a flaky/environmental failure and should never have been
parked here; a deterministic structural gate red means the tree is broken. Fixed
by moving it to `internal/core/rib/routeinstall` (beside `locrib`, its in-process
twin), which is outside the audited areas, so no manifest row or fake registration
is needed. `ze-tier-check` + `TestEnginePlacement` green. To stop this class from
being waved through again, `commit_helper.py` now refuses to treat a deterministic
structural gate as a bypassable known-red (see `ai/rules/git-safety.md`).

### 2026-07-04 -- `config/yang` `TestBuildCommandTreeEnsureExists` -> stale test retargeted

**Resolved 2026-07-04.** Not a product regression: the ensure-exists handlers
were not "missing from the built tree." Commit `5f7c70f18` (the verb-first
grammar gate) intentionally restructured `ze-iface-cmd.yang` -- `create interface
dummy <name>` became `create interface dummy name <name>` (a typed `name`
selector, cli-grammar.md R6) -- which moved `ze:command`/`ze:ensure-exists`/`unit`/
`address` from the `dummy` grouping onto the nested `name` node, but left this
test navigating the old positions. The rollback behavior is preserved (the
ensure-exists lives on the `name` node now). Fixed by retargeting the test's
navigation through `.Children["name"]`, keeping every assertion. Deterministic
under `-tags ze_ospf`; verified green.

### 2026-07-02 -- `internal/plugins/ospf/multi_instance.go` build break -> concurrent OSPF session completed

**Resolved 2026-07-03.** The concurrent OSPF multi-instance refactor landed:
`e.mInstanceMismatch`, `cfg.instanceIDSet`, `cfg.forInstance` now exist. Full
tagged build with `ze_ospf` succeeds. `plugin/all` golden-snapshot tests
(`TestRegisteredPluginNames`/`TestRegisteredWireMethods`/`TestYANGSchemaProviders`)
also pass.

### 2026-07-01 -- kernel-runtime-deps parallel-execution flake -> per-test isolation

**Resolved 2026-07-01.** `install/26` kernel-runtime-deps TOCTOU race was a
shared-path collision: it read/created `tmp/kernel/build/vmlinuz` while
`ze-kernel-overlay` (which runs `make ze-kernel`) moved/removed that same dir,
so a concurrent `out.stat()` threw `FileNotFoundError`. Fixed by redirecting the
test's build-output artifact to a per-test dir: `make -q -C gokrazy/kernel
OUT="$work/out"` (the Makefile's `OUT :=` at `gokrazy/kernel/Makefile:19` loses
to a command-line override), with the fake vmlinuz created/touched there and the
`out.stat()` hardened with try/except. The mtime dependency graph the test
exercises is unchanged: prerequisites are repo-relative source paths, not `OUT`.
Verified: 4 back-to-back parallel `ze-test install --pattern kernel` runs, all
20/20 PASS with runtime-deps and overlay in the same batch.

### 2026-07-01 -- Plugin cos-vendor (cisco/coexist) -> fixture fixed

**Resolved 2026-07-01.** `cos-vendor-cisco`, `cos-vendor-coexist` (was ids
126/127, now 128/129) nested `radius { server ... }` under `l2tp { authentication
{ ... } }`. The `authentication` container (added by spec 888, l2tp-env-promote)
holds only PPP-phase `timeout`/`reauth-interval`; the RADIUS config path is
`l2tp { auth { radius { ... } } }`, defined by the authradius plugin
(`internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang:11`).
The fixtures named the wrong sibling container. Fixed both `.ci` files to `auth {}`;
`ze config validate` now returns "configuration valid" (was `unknown field in
authentication: radius`). Same class as the paths-limit.ci fixture fix. These
tests are `needs-linux`; the failing part (YANG validation) is host-independent
and was verified on darwin via `ze config validate`.

### 2026-07-01 -- Parse VPP feature gate (bridge/veth/aggregates) -> already passing

**Resolved 2026-07-01.** `iface-vpp-rejects-bridge`, `iface-vpp-rejects-veth`,
`iface-vpp-aggregates-errors` were logged in the 2026-06-17 triage as "VPP backend
feature gate not implemented". Spec 621 (backend-feature-gate) shipped the walker
`config.ValidateBackendFeatures` (`internal/component/config/backend_gate.go`) and
wired it into `ze config validate` (`cmd/ze/config/cmd_validate.go`). Re-run
2026-07-01: all 7 `iface-vpp-*` parse tests PASS. The triage entry was stale.

### 2026-07-01 -- L2TP reauth-interval-clamp -> replaced by YANG range

**Resolved 2026-07-01.** The 2026-06-17 triage flagged the env-var path bypassing
the reauth safety floor. Spec 888 (l2tp-env-promote) removed the L2TP env vars
entirely and moved `reauth-interval` into YANG with `range "0 | 5..86400"`
(`internal/component/l2tp/yang/ze-l2tp-conf.yang:172`), deleting `clampReauthInterval`
and its test. Verified 2026-07-01: `ze config validate` rejects `reauth-interval 3`
(`outside range 0, 5..86400`) and accepts 0/300 -- the floor is now enforced at
commit time, a stronger guarantee than the old runtime clamp.

### 2026-06-18 -- Web suite (was 81/81 FAIL) -> harness fixed, genuine bugs fixed

**Resolved 2026-06-18.** Root cause was the harness, not the product: the
runner launched `ze start --web <port> --insecure-web` against an empty temp
config store, so the daemon refused to start (full `--web` needs a loaded
config) and exited before binding the port -- every test timed out at the
readiness probe (`config "ze.conf" has unknown type`).

Harness fixes (`internal/test/cli/cmd_web.go`,
`internal/component/web/testing/runner.go`):
- Launch `--web-only` (standalone web UI, no daemon/config -- the mode the
  daemon's own error hint recommends). Server now binds; suite 0 -> ~76/81.
- Readiness probe does an HTTPS GET, not a bare TCP connect (TCP accepts the
  instant the listener binds, before routes mount, so a browser could hit an
  empty page).
- `expect` assertions auto-retry up to 5s (standard auto-waiting pattern),
  absorbing HTMX/JS render races a single point-in-time snapshot caught as
  "(empty page)".
- Each test closes its browser session when done (was leaking 80+ live pages
  into the shared agent-browser daemon over a run).
- Seed a zefs local-admin into the temp store so `/show/users/` lists the
  always-on "(system)" power user (verified via curl: page renders `(system)`).

Four genuine pre-existing test bugs the all-failing harness had masked, all
fixed (authorized `.wb` edits): `scenario-interface-setup` and
`interface-configured-display` filled a `field-mac-address` the key-only add
overlay never renders (removed -- mac-address is edited on the detail page);
`logs-live-stream` asserted the transient "Connecting" that SSE replaces with
"Connected" (now asserts "Connected"); `system-users-power` needed the seeded
admin to show the "(system)" marker.

Residual: this is performance-sensitive browser automation driven through one
shared agent-browser daemon. Under heavy host load (this dev box sat at load
avg ~7 from unrelated apps) render races still flake a rotating handful per
full run; every test passes individually. Expected reliable on a quiet CI
host. Verified: each fixed test passes in isolation; `webtesting` unit tests
green; `--web-only` server serves every exercised route.

### 2026-06-18 -- Lint: `internal/analyze/inject.go:64` goconst

`--router-id` had 3 occurrences across inject.go, serve.go, replay.go.
Added `//nolint:goconst` to inject.go and serve.go (replay.go already had it).

### 2026-06-17 -- Plugin observer/RIB visibility (6 tests) -> PASS

**Resolved 2026-06-17.** `40` bestpath-reason, `220` multipath-basic, `224`
nexthop-self, `225` nexthop-unchanged, `308` rib-forward-handle-observed, `350`
rr-basic. Triaged as "routes never appear in RIB within 15-20s timeout (product
bug in forwarding)". Actually the same establishment-time EoR race as the exabgp
suite: these tests have the mock peer wait for ze's End-of-RIB
(`rib-forward-handle-observed.ci:21`) and an observer poll for the prefix; the
bgp-rs duplicate/misordered EoR perturbed establishment so the poll timed out.
Fixed by `99c943404` (`AnnounceEOR` honors `ShouldQueue()`). Now 0 failures
across 5 runs (~2-4s each, not the 15-20s timeout); full plugin suite 422/424
(only 126/127 cos-vendor remain). Causation is circumstantial (not bisected --
reverting a committed fix needs forbidden git ops) but the failing->passing
transition lands in this commit's window and the EoR mechanism is shared.

### 2026-06-17 -- `ze-test bgp encode 38 paths-limit` (broken fixture) -> PASS

**Resolved 2026-06-17.** Not a ze bug. ze emits the route WITH an ADD-PATH
path-id (`00 00 00 00 18 0C 00 02`); the fixture expected it WITHOUT, so the
decoder read the four path-id zero bytes as four `0.0.0.0/0` prefixes. ze is
RFC 7911-correct: the config advertises add-path send/receive, the ze-peer mock
MIRRORS the OPEN so it advertises receive, the family negotiates
(`negotiated.go:279` gates on localSend && remoteReceive), and a path-id is then
mandatory on every ipv4/unicast NLRI. The expected hex in
`test/encode/paths-limit.ci` (added `56f48c85f`) omitted the path-id and was
internally inconsistent. Fixed the fixture (user-authorized per
`ai/rules/testing.md`) to ze's correct output. Encode suite now 53/53.

### 2026-06-17 -- `ze-exabgp-test` (was "10/40 product bugs") -> 40/40 PASS

**Resolved 2026-06-17.** The "10 distinct encoding bugs" was a mis-diagnosis.
Verified non-deterministic: failure set changed every run (e.g. run A
{20,32,35,39,40} vs run B {1,14,18,25,31,33,35,36,40}); conf-addpath passed
5/5 alone yet failed under parallel load. Two real causes:

1. **EoR race (8 tests + watchdog).** Two producers send End-of-RIB on session
   establishment: reactor `sendInitialRoutes` (always, per-family) and the
   bgp-rs plugin's `replayForPeer` goroutine (fast-fails when bgp-adj-rib-in is
   absent, as the exabgp wrapper loads a minimal plugin set). Announce/withdraw
   honor `ShouldQueue()`+opQueue; `AnnounceEOR` wrote directly, so the plugin
   EoR raced ahead of the still-queued route NLRI. Partial fix `af60758d0`
   covered only family-specific routes, not the static-route phase.
   Fix `99c943404`: `AnnounceEOR` skips peers in initial sync (reactor owns the
   EoR). Removes the race and the duplicate EoR.
2. **srv6-mup (1 test) -- the only real encoding bug.** `routeattr_prefixsid.go`
   wrote the SRv6 SID Structure Sub-Sub-TLV header as 4 bytes (`0,1,0,len`)
   instead of RFC 9252 3 bytes (`1,0,len`) -- a spurious leading reserved byte,
   inflating the inner sub-TLV by 1 (0x1F vs 0x1E). Decode side was already
   correct (`srv6sid.go`). Fix: drop the extra byte.

After both: 40/40 pass across repeated full-suite runs; watchdog 12/12 alone.

### 2026-06-17 -- decode suite (37/37 FAIL -> 37/37 PASS)

**Resolved 2026-06-17.** Three root causes:
1. 36 `.ci` files referenced `ze-test decode` (removed); renamed to
   `ze bgp decode`. Added `--family` long flag alias for `-f`.
2. `CombinedOutput()` in `decoding.go` mixed YANG description mismatch
   warnings (stderr) into JSON output. Changed to separate stdout/stderr.
3. Plugin registry maps (`CapabilityMap`, `FamilyMap`, `InProcessDecoders`)
   were captured at package init time before all plugins registered. Changed
   to lazy `sync.OnceValue` so lookups happen after all `init()` complete.

### 2026-06-17 -- plugin JSON-match + cli-show (7 new regressions -> 0)

**Resolved 2026-06-17.** Same YANG-warnings-in-stdout root cause:
`CombinedOutput()` in `runner_validate.go:decodeToEnvelope` mixed stderr
warnings into JSON decode output. Changed to `cmd.Output()`. Also updated
`cli-show.ci` test expectation (`Available commands` -> `Commands`).

### 2026-06-17 -- UI functional tests

**Resolved 2026-06-17.** All 5 UI failures from 2026-06-13 now pass.

### 2026-06-04 -- QEMU baseline triage

**Resolved 2026-06-04.** Clean re-run confirmed host-load artifacts.
Fixed: UI build tags, expected strings, mpls-doctor semicolons, firewall
skip-env. Product bugs fixed: L2TP CDN teardown, StopCCN cascade.
Environment deps (skip-env tagged): show-policy-routes, wireguard-invalid.

### 2026-06-10 -- routewatch QEMU integration tests flaky (netns roulette)

**Resolved 2026-06-10.** Namespace-aware subscribe + event polling.

### 2026-06-11 -- `make ze-verify-wiring-docs` command validation drift

**Resolved 2026-06-11.** Wiring, doc, and inventory gates all green.

### 2026-05-31 -- pppoe-client `no-default-route` + dispatch single-marshal

**Resolved 2026-05-31.** TypeEmpty wired end-to-end. Single-marshal,
stale plugin lists, migration keyword, multi-line descriptions, CLI
grammar catch-up.

### 2026-07-23 -- `ze-test bgp reload` `commit-transactional` never writes `meta/config/rollback`

**Resolved 2026-07-23.** Root cause: a successful SIGHUP reload emitted no
signal at all. `handleSIGHUPReload` (`cmd/ze/hub/main_reload.go`) printed
`received SIGHUP, reloading config...` on entry and, on success, printed
nothing; only failures printed `reload error: %v`. The daemon's last word was
therefore identical whether a reload finished instantly, was still running, or
had wedged.

Two consequences, and the second is what disguised this as flakiness:

1. An operator could not tell a completed reload from a stuck one.
2. No functional test could fence on completion, so every reload `.ci` raced its
   own teardown. The runner tears the daemon down right after `action=sighup`;
   under load it was killed mid-reload, leaving a partial atomic write (a stray
   `.ze-storage-*` under `rollback/`) and no `meta/config/rollback` pointer.
   Reproduced by hand outside the harness: SIGHUP, nothing logged, process still
   alive, temp file orphaned.

Fix: `reloadComplete()` prints the stable phrase `reload complete` after a
successful reload (direct and queued-SIGHUP paths), and `commit-transactional.ci`
fences on it with `await=stderr:contains=reload complete`.
`test-tx-ipsec-eap-tls-requires-ca.ci` got the same treatment for its own
rejection message, replacing a back-to-back SIGHUP/SIGTERM pair no daemon could
win under load.

Verified: `bgp reload` 31/31 green unloaded; the ipsec test survived 40/40
invocations of `stress-repro.py "bgp reload" --test 34` at 64 burners on 32
cores, having previously failed on invocation 1.

### 2026-07-23 -- `ze-test bgp reload` seven iface/apply-ordering tests hang without CAP_NET_ADMIN

**Resolved 2026-07-23.** Root cause: the wrong marker, and a marker that could
not express the requirement.

The seven tests carried `option=skip-os:value=darwin` ("skip on macOS").
`ai/rules/qemu-testing.md` prescribes `option=needs-linux` for a `.ci` that boots
a daemon which APPLIES Linux-only config, which is what these do. And
`needs-linux` gated only on `runtime.GOOS`, so even the correct marker would not
have helped on an unprivileged Linux host.

Unprivileged, the interface plugin dies during its stage-3 handshake
(`iface: set up "order-del0": operation not permitted`), the daemon never reaches
the asserted state, and the test does not fail -- it HANGS to the suite timeout.
That is why these read as load-sensitive timeouts.

Fix: `option=needs-linux` now accepts `caps=net-admin`, gated on the effective
capability read from `/proc/self/status` (`internal/test/runner/caps_linux.go`),
not on uid 0 -- a setcap'd binary holds the capability without being root, and a
restricted container may lack it while being root. The seven tests declare it and
now SKIP with a reason naming `make ze-qemu-needs-linux-test`, where they run for
real as root. An unknown `caps=` value is a parse error, so a typo cannot silently
disable the gate. Guarded by `TestNeedsLinux*` in
`internal/test/runner/caps_option_test.go`, which injects the probe so BOTH
polarities are asserted on any host -- an earlier version asserted only the host's
real answer and was therefore vacuous in QEMU, the one place the suite runs for
real.

Only the seven that actually fail were marked; the four sibling iface tests that
pass unprivileged still run.

**What this does NOT claim.** These seven now execute in no automated pipeline:
CI runs `make ze-verify` unprivileged (so they skip), and no workflow invokes
`ze-qemu-needs-linux-test` -- `ai/rules/qemu-testing.md` already records the QEMU
suites as run by "NOTHING automated". The change converts an opaque hang into an
honest skip; it does not give these tests a gate. Two follow-ups, both recorded in
`reload-transaction-tests-load-sensitive.md`: a workflow that runs the QEMU
needs-linux target, and the daemon-side defect underneath (an unprivileged `ze`
whose config names an interface hangs instead of failing, which is a
`fail-closed-guards.md` violation in the product, not just in the tests).
