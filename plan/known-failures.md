# Known Failures

Pre-existing test failures tracked here per `ai/rules/git-safety.md` ("Before Any
Commit" -> pre-existing failures >10 min): logged, not blocking unrelated commits.

**Scope: non-deterministic (flaky/environmental) TEST reds only.** Deterministic
structural gates (`ze-lint`, `ze-lint-changed`, `ze-tier-check`, `ze-vet-evidence`,
`ze-plugin-boundary-check`, `ze-iface-resolution-check`, `ze-regen-check-readonly`,
`ze-verify-wiring-docs`) are NEVER logged here -- a red means the tree is
structurally broken; fix it at the source. `scripts/dev/commit_helper.py` enforces
this by refusing `--unverified` while a structural gate is red (see
`ai/rules/git-safety.md` "Structural Gates Are Never Known-Red").

## BEFORE LOGGING ANYTHING HERE: reproduce with the Makefile's build tags

Bare `go test ./...` is **NOT** equivalent to `make ze-unit-test` in this repo.
Features compile out behind build tags (`//go:build ze_isis`, `ze_ospf`, `ze_web`,
`ze_ssh`, ...), and the Makefile always supplies them (`Makefile:51` `ZE_FEATURES`
read from `feature-gates.txt`, `Makefile:65` `GO_TEST_TAGS`). A bare run silently
drops those plugins, so their registrations, validators and listeners never exist
and unrelated tests fail with phantom reds.

```
go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')" ./path/...
```

This is not hypothetical: on 2026-07-15 two of the four entries below (7 tests)
were disproven as pure tags artifacts. Both had been logged with a confident but
wrong root cause (a "macOS socket-stack quirk", a "broken listener-conflict
validator"), and one had been "re-confirmed" six days later by repeating the same
flawed invocation. Reproducing a symptom is not attributing a cause.

**Status 2026-07-15: 1 open entry (`rsvpte-lsp-setup`, genuinely
non-deterministic). Of the other three: 2 disproven as tags artifacts (7 tests,
never broken) and 1 fixed (`TestCmdMethods`, a real stale literal).**

**Status 2026-07-04 (historical): three open entries (below).**
`TestBuildCommandTreeEnsureExists` (config/yang) is now resolved (stale test
retargeted to the typed name selector -- see Resolved). A new open entry, the
`rsvpte-lsp-setup` load-only panic, is added. The OSPF build break and
`plugin/all` golden-snapshot failures from 2026-07-02 are resolved (the
concurrent OSPF session's `multi_instance.go` work landed and snapshots are
current). Every previously tracked entry from 2026-07-01 and earlier remains
resolved (see below).

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

### `rsvpte-lsp-setup` -- load-only `slice bounds` panic in `ze`, pre-existing

Observed 2026-07-04 (~1 of 4 full `ze-verify` functional runs): the `ze` engine
process panics during the `rsvpte-lsp-setup` functional test with
`panic: runtime error: slice bounds out of range [:5448] with capacity 512`
(exit 2 -> `expect exit-code 0` fails). A cap-512 buffer is resliced to hold
5448 bytes -- a "trust a length, do not grow/check the buffer" bug; 5448 bytes
is the size of a large boot-time frame (e.g. the share-registry command dump the
external `rsvpte-setup` plugin receives). The test config boots rsvp-te + a BGP
peer (`accept false`, never establishes) + the external JSON plugin.

Reproduction is environment-specific, NOT raw repetition: 0 panics across 40
serial + ~360 parallel isolated `ze-test rsvpte 3` runs, 0 under a `-race` build
in isolation (no data race detected at that load), 0 under heavy synthetic load
(which only produced 15s timeouts). It only appears in the full-verify
environment (all feature plugins compiled in, GOMAXPROCS=13, real suite load).
The verify aggregator truncates the goroutine stack to 2 lines
(`goroutine N [running]:`); the runner itself keeps up to 10 MB / 200 lines
(`runner_exec_util.go:55`, `report.go:175`), so a full-suite repro captured via
`ze-test rsvpte --all -v` (not the aggregator) will carry the crash site.

Ruled out (producers read, all safe): BGP text/JSON format scratch buffers
(`format/text_human.go:224`, `format/text_json.go:375` -- both guarded by
`if n > cap(raw)`), the RPC frame/batch pools (`pkg/plugin/rpc/framing.go`
4 KB-cap, `batch.go`, `conn.go:writeAppended`, `mux.go` -- all `append`-based),
and the RSVP message builder (`rsvpte/build.go:encodeMessage`, 1500-cap). The
BGP forwarding/update pools do not run (the peer never establishes). The cap-512
buffer is elsewhere; the captured crash stack will pin it. Owner: in-progress
this session (debugging continues).

**UPDATE 2026-07-13 — the cap-512 diagnosis is DISPROVEN; likely already-fixed or
misattributed.** Two independent exhaustive static sweeps (share-registry send
path + repo-wide pooled/fixed-cap reslice) found NO cap-512 buffer resliced to a
data-driven length on any boot-reachable path: the registry send is
`json.Marshal` + `append` (`plugin/server/startup.go:733` -> `ipc/rpc.go:171` ->
`rpc/mux.go:110`/`conn.go:286` -> `framePool` cap **4096**, append-grown); the BGP
`SessionBuffer` is 4096/65535, never 512 (`core/bgp/wire/writer.go:48`); the only
cap-512 format scratches (`format/text_human.go:219`, `text_json.go:370`) are
guarded and hold <=512 raw bytes. Dynamic: **0 reproductions in 160 runs** (40
isolated + 120 under `scripts/dev/stress-repro.py` full-core CPU+GC load,
`GOTRACEBACK=all`). The 2026-07-04 crash also predates the plugin
startup/RPC-dispatch refactors (`1eb89f509`, `3404c4396`, `8f3203ef5`, 07-07/08)
that rewrote this exact area. Conclusion: either already fixed by those refactors,
or the truncated 2-line aggregator stack misattributed another concurrent suite's
`ze` crash to rsvpte. **Do not chase the cap-512 share-registry hypothesis.** If a
`rsvpte-lsp` exit-2 recurs, reproduce with `scripts/dev/stress-repro.py rsvpte`
(keeps the untruncated stack) and grep every concurrent daemon's stderr in the
failing run before attributing. A separate `rsvpte-lsp-teardown` exit-2 (no stack
in the 200-line capture) was seen once on 2026-07-13 and did not reproduce in 160
runs; it is not the same panic and its cause is unverified.

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

### `reload` suite -- 6 iface tunnel/wireguard tests time out without CAP_NET_ADMIN (unprivileged sandbox)

Observed 2026-07-09 (rootless Linux sandbox; `unshare --net` returns
"Operation not permitted", no CAP_NET_ADMIN). Six `.ci` tests in the `reload`
suite time out (20s each), blocking a native `make ze-verify` /
`ze-functional-test` from completing:

- `test-tx-iface-tunnel-modify-key` (id 26)
- `test-tx-iface-tunnel-remove` (id 27)
- `test-tx-iface-wireguard-invalid-bad-public-key` (id 29)
- `test-tx-iface-wireguard-invalid-no-private-key` (id 30)
- `test-tx-iface-wireguard-modify` (id 31)
- `test-tx-iface-wireguard-remove` (id 32)

Root cause is environmental, not a product regression. All six put a real
Linux interface (gre tunnel or wireguard) in the **boot** config. At startup
the iface plugin's `OnConfigure` handler
(`internal/component/iface/register.go:395`, error return at `:416`) calls
`applyConfig`, which invokes the netlink create
(`internal/plugins/iface/netlink/tunnel_linux.go:42-51`,
`internal/plugins/iface/netlink/wireguard_linux.go:33-41`). Without
CAP_NET_ADMIN that returns EPERM ("operation not permitted"). Unlike the
reload path, a startup create failure is FATAL: `OnConfigure` returns the
error, which fails the interface plugin's Config-stage handshake in
`deliverConfigRPC` (`internal/component/plugin/server/startup.go:691,713`),
cascading to "config-path plugin startup failed". The daemon never serves
BGP, so the peer never connects and the test's route+EOR expectation times
out.

The four sibling tests that apply the interface on **reload** rather than at
boot (`test-tx-iface-apply` id 23, `test-tx-iface-bgp-chain` id 24,
`test-tx-iface-tunnel-create` id 25, `test-tx-iface-wireguard-apply` id 28)
PASS natively: the daemon boots with no interface, the BGP peer receives
route+EOR (assertion satisfied) before the failing reload, and a reload
create failure rolls back without killing the already-running daemon.

These are legitimately privilege-dependent (create-then-modify/remove a real
netdev), so they pass under QEMU-root / a privileged host, not in this
sandbox. Per `ai/rules/qemu-testing.md` the six boot-config tests arguably
belong on `option=needs-linux` rather than the current
`option=skip-os:value=darwin`, BUT (a) `needs-linux` only skips on non-Linux
GOOS, so it would NOT change native-Linux behavior here (still run, still time
out), and (b) reclassifying adds them to `ze-qemu-needs-linux-test`, whose
Alpine VM kernel must carry the gre/wireguard modules -- `runtime.config` has
`CONFIG_WIREGUARD`/`CONFIG_NET_UDP_TUNNEL`/`CONFIG_TUN` but no explicit GRE
symbol, so this needs QEMU verification before flipping. Classification change
deferred pending that verification; the native timeout is documented here so
`ze-verify` in an unprivileged sandbox is not treated as a structural red.
Owner: whichever session runs these under QEMU-root and confirms the kernel
modules.

## Harness notes (not failures)

### `.ci` suites -- 108 quick-exit `ze` commands across 50 files are silently unasserted

Found 2026-07-15 while building the vrrp suite. `expect=exit:code=` is
**file-level**: `Record.ExpectExitCode` is a single value (`record_parse.go:486`
-- a later line overwrites an earlier one) and `runOrchestrated` compares it
against `lastQuickZeErr`, the exit status of only the **last** quick-exit `ze`
command (`runner_exec.go:911-915`; the `case quickZe:` branch just stores the
error and continues). A file running several `ze config validate` commands
therefore asserts only the final one. Proven with a probe: a file whose `seq=1`
ran a **valid** config (exit 0) under `expect=exit:code=1` **passed**.

Stdout expectations are file-level in the same way (matched against accumulated
output), so `expect=stdout:contains=` can be satisfied by a different command
than intended -- e.g. two rejection cases both asserting `contains=vrid` are both
satisfied by the first one's message.

Fixed at the source for new tests: `cmd=...:exit=N` asserts a command's own exit
code the moment it finishes (`ci-format.md` "Process Commands"). It is opt-in, so
existing files are unaffected -- and consequently still unasserted.

Worst offenders (`quick-ze` commands / unasserted): `test/ui/format-operators.ci`
15/14, `test/ospf/ospf-config.ci` 7/6, `test/ospf/ospf-bfd-config.ci` 5/4,
`test/ospf/ospf-virtual-link-config.ci` 5/4, `test/ospfv3/ospf-ipsec-config.ci`
5/4, `test/ui/skills-list-get.ci` 5/4, `test/isis/isis-doctor.ci` 4/3.

**Not yet swept:** arming the 108 belongs to the suites' owners -- it may surface
real defects (a validation that never rejected what its test claimed). Re-measure
with the script in the vrrp session, or by counting `cmd=foreground` quick-`ze`
lines without `:exit=` in any file that has more than one.

The full plugin suite shows load-induced flakiness under max parallelism -- e.g.
`257`, `258`, `312` failed in one `--all` run but pass 3/3 in isolation. Running
two full `--all` suites back-to-back melts down (resource exhaustion: ~50
timeouts, ~200 "failures"). Triage individual tests in isolation; treat a
contiguous block of failures or a spike of timeouts in `--all` as a
harness/resource artifact, not real regressions.

### `sync.Pool` capacity/identity unit flakes under full-suite GC pressure

Observed 2026-07-07 in a full `ze-verify` run (stage 07 `ze-unit-test-cached`):
`internal/core/textbuf` `TestPoolPreservesCapacityWithoutString` (`"128" is not
greater than or equal to "300"`) and `internal/core/bufpool`
`TestGetReturnsSameBufferAfterPut`. Both assert a `sync.Pool` preserves a
buffer's capacity/identity across Get/Put, which the GC can invalidate under the
memory pressure of the full parallel suite. textbuf passes 5/5 in isolation
(`go test ./internal/core/textbuf/ -run TestPoolPreservesCapacityWithoutString
-count=5`). Same non-deterministic class as learned 881. Triage in isolation;
not a regression from an unrelated change.

### `internal/component/iface` `TestIntegrationApplyConfigVLANUnitAddressReconcile` -- pre-existing, VLAN unit address change never applied on reload

Confirmed pre-existing 2026-07-15 by `git archive HEAD` into a scratch tree and
running the test there in the QEMU VM: it fails **identically** on committed
`HEAD` (no VRRP work present), so `spec-vrrp-3-macvlan` did not cause it. Also
reproduced with the new owned-device reconcile pass compiled out, and
`config_apply.go`'s diff is purely additive (+143/-0), so the reconcile path is
byte-identical to `HEAD` in that configuration.

Symptom: `unit_integration_linux_test.go:133` -- after
`applyConfig(current, previous, b)` swaps a VLAN unit's address from
`10.60.200.1/24` to `10.60.200.2/24`, the new address is absent from
`parent0.200`. The reload's `applyConfig` returns **no errors**, so the address
loop believes it succeeded; the device itself survives (the `linkExists` check
above passes). The initial apply and its `requireAddress` assertion both pass,
so this is specific to the address-change-on-reload path for VLAN sub-interface
units, not to unit creation.

Fix owner: whichever session next works `internal/component/iface` VLAN unit
reconcile (`config_apply.go` unit/address loops). Not fixed by the VRRP umbrella
(`plan/spec-vrrp-0-umbrella.md`): unrelated code path, and VRRP's virtual
addresses reach the kernel through the owner registry, which is covered by
`registry_integration_linux_test.go` and the new
`device_owner_integration_linux_test.go` and is green.

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

### `l2tp` functional suite `session-stopccn-cascade` -- pre-existing, predates the initiator work

Confirmed pre-existing 2026-07-10 by building `bin/ze` + `bin/ze-test` at commit
`fe6aa242f` (the parent of `b68e7e9c9`, the first `spec-followup-l2tp-call`
commit) via `git archive` and running the test: it fails there **3/3**
deterministically, identically to `HEAD` (5/5). So `spec-followup-l2tp-call`
did not cause it. Symptom: `test/l2tp/session-stopccn-cascade.ci` step 2
(`expect=stderr:contains=StopCCN clearing sessions`) does not match. Root cause
lies in the answering-side reliable-receive path: after the tunnel + session 1
establish, ze's receive window does not advance past the second session's
rapid-fire ICRQ, so the peer's later StopCCN (a higher Ns) is never delivered to
`handleStopCCN` (`tunnel_fsm.go:582`), and the `StopCCN clearing sessions` log at
`tunnel_fsm.go:597` never fires. It belongs to the answering-side
reliable-receive path, not this initiator spec. Fix owner: whichever session
next works the L2TP reliable-transport receive path. `.ci` unchanged since
before this spec.

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
